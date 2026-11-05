package hook

import (
	"math"
	"sync/atomic"
	"time"

	"go.uber.org/zap/zapcore"
)

// levelTier groups zap levels into the three independently budgeted classes of
// the Redis stream hook. Debug shares the info budget; dpanic/panic/fatal share
// the error budget.
type levelTier int

const (
	tierInfo levelTier = iota
	tierWarn
	tierError
	tierCount
)

func tierOf(lvl zapcore.Level) levelTier {
	switch {
	case lvl >= zapcore.ErrorLevel:
		return tierError
	case lvl == zapcore.WarnLevel:
		return tierWarn
	default:
		return tierInfo
	}
}

// dropReason is why the Redis stream hook did not deliver an entry.
type dropReason int

const (
	dropRate      dropReason = iota // over the level's token-bucket budget
	dropQueue                       // async queue full
	dropXAddError                   // XADD returned an error
	dropReasonCount
)

// LevelCounts is a per-level breakdown of dropped entries.
type LevelCounts struct {
	Info  uint64
	Warn  uint64
	Error uint64
}

// Total sums all levels.
func (c LevelCounts) Total() uint64 { return c.Info + c.Warn + c.Error }

// DropCounts is a snapshot of entries the Redis stream hook dropped, by reason
// and level. Counters are cumulative since process start.
type DropCounts struct {
	Rate      LevelCounts // over the level's budget (qps / warn_qps / error_qps)
	Queue     LevelCounts // async queue full
	XAddError LevelCounts // XADD failed (MOVED, NOAUTH, OOM, connection errors, ...)
}

// Total sums all reasons and levels.
func (c DropCounts) Total() uint64 { return c.Rate.Total() + c.Queue.Total() + c.XAddError.Total() }

type dropCounters [dropReasonCount][tierCount]atomic.Uint64

func (d *dropCounters) add(r dropReason, lvl zapcore.Level) {
	d[r][tierOf(lvl)].Add(1)
}

func (d *dropCounters) snapshot() DropCounts {
	row := func(r dropReason) LevelCounts {
		return LevelCounts{
			Info:  d[r][tierInfo].Load(),
			Warn:  d[r][tierWarn].Load(),
			Error: d[r][tierError].Load(),
		}
	}
	return DropCounts{Rate: row(dropRate), Queue: row(dropQueue), XAddError: row(dropXAddError)}
}

// processDrops aggregates every RedisStreamCore in the process. A process
// normally runs one core; the aggregate lets a service export the metric
// without holding a handle to the core buried inside its logger.
var processDrops dropCounters

// DropStats returns the process-wide drop counts of all Redis stream hooks.
// Backs the system_log_dropped_total metric.
func DropStats() DropCounts { return processDrops.snapshot() }

// tokenBucket is a lock-free token bucket refilled lazily from elapsed time.
// capacity <= 0 means unlimited.
type tokenBucket struct {
	capacity   int64
	tokens     atomic.Int64
	lastRefill atomic.Int64 // unix nanoseconds
}

func newTokenBucket(qps int) *tokenBucket {
	b := &tokenBucket{capacity: int64(qps)}
	if qps > 0 {
		b.tokens.Store(int64(qps))
		b.lastRefill.Store(time.Now().UnixNano())
	}
	return b
}

// allow consumes one token. Returns true if granted, false if over budget.
func (b *tokenBucket) allow() bool {
	if b.capacity <= 0 {
		return true
	}

	now := time.Now().UnixNano()
	last := b.lastRefill.Load()
	elapsed := now - last

	// One second = full budget.
	if elapsed > 0 {
		refill := int64(math.Floor(float64(elapsed) * float64(b.capacity) / float64(time.Second)))
		if refill > 0 && b.lastRefill.CompareAndSwap(last, now) {
			// Cap at capacity so a long idle window doesn't allow a burst > qps.
			if tokens := b.tokens.Add(refill); tokens > b.capacity {
				b.tokens.Store(b.capacity)
			}
		}
	}

	for {
		current := b.tokens.Load()
		if current <= 0 {
			return false
		}
		if b.tokens.CompareAndSwap(current, current-1) {
			return true
		}
	}
}
