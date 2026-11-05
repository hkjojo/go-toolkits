# Log

## Telegrame

- Telegram search BotFather,type cmd(/newbot) to create token and robot
- Add robot by robot_name to group
- Send a dummy message to the bot /my_id @my_bot
- https://api.telegram.org/botXXXXX/getUpdates, XXX as token
- get the chat_id
- use https://api.telegram.org/botXXXXX/sendMessage as URL
- char_id as CharID

## DingDing

- Create DingDing group
- Add Group Assistant(Custom robot)
- Get Webhook url as URL

## Redis Stream Hook

Producer-side core that XADDs each log entry into a Redis stream. Designed
for cross-service runtime event aggregation (e.g., a system_log table where
N services produce and a single consumer persists). The hook builds its own
`redis.UniversalClient` from `RedisStreamConfig` — no external client
required.

```go
import (
    tlog "github.com/hkjojo/go-toolkits/log/v2"
    "github.com/hkjojo/go-toolkits/log/v2/hook"
)

cfg := &tlog.Config{
    Level:  "info",
    Format: "json",
    Caller: true,
    Fields: map[string]string{"service": "maker-adapter"},
    Redis: &hook.RedisStreamConfig{
        Addr:      "redis:6379",
        StreamKey: "system_logs",
        MaxLen:    200_000,
        QPS:       50, // info/debug per second
        WarnQPS:   50, // warn per second
        ErrorQPS:  50, // error and above per second
        // Filter is the BaseCore drop list — listed field keys are removed
        // before serialization (e.g., to keep secrets out of logs).
        CoreConfig: hook.CoreConfig{
            Filter: []string{"password", "api_key", "secret"},
        },
    },
}
logger, _ := tlog.New(cfg)
```

### Buffered stdout

`Config.StdoutBufferSize > 0` (json `stdout_buffer_size`, bytes) buffers stdout
and flushes it at least every second; error-and-above entries are flushed
immediately. Call `tlog.FlushStdout()` before the process exits, including
`os.Exit` paths — it flushes every buffer `New` created, however the logger was
wrapped afterwards. Default 0 writes each entry straight to stdout.

`Mode` selects the server topology: `""` / `"single"` (default) or `"cluster"`.
Cluster mode builds a cluster client even from a single seed (e.g. an ElastiCache
configuration endpoint) and accepts a comma-separated seed list in `Addr`; `DB` must
be 0. A single-node client pointed at a cluster gets `MOVED` whenever the stream's
slot lives on another node, so set the mode explicitly. The consumer shares the
same `RedisStreamConfig`, so producer and consumer always use the same topology.

```go
Redis: &hook.RedisStreamConfig{
    Mode:      "cluster",
    Addr:      "my-cluster.xxxxxx.clustercfg.apse1.cache.amazonaws.com:6379",
    StreamKey: "system_logs",
},
```

Each XADD writes the following fields:

| Field     | Source                                             |
|-----------|----------------------------------------------------|
| `ts`      | unix milliseconds (string)                         |
| `level`   | zap raw level (`debug`/`info`/`warn`/`error`/...)  |
| `service` | `RedisStreamConfig.Fields["service"]`              |
| `source`  | sub-logger field via `log.With("source", "...")`   |
| `msg`     | zap entry message                                  |
| `payload` | remaining structured fields, JSON-encoded          |
| `host`    | `os.Hostname()` (auto-injected)                    |

Consumer side (parses the same wire format and feeds a user-provided
`SaverFunc` for batch persistence):

```go
consumer, _ := hook.NewRedisStreamConsumer(cfg.Redis, hook.ConsumerOpts{
    GroupName:  "system-log-persister",
    ConsumerID: "apiserver-" + os.Getenv("HOSTNAME"),
}, func(ctx context.Context, batch []hook.Entry) error {
    return repo.BulkInsertSystemLog(ctx, batch)
})
_ = consumer.Start(ctx)
defer consumer.Stop()
```

The consumer runs three internal goroutines:

1. main loop — `XReadGroup` + saver + `XAck`
2. autoclaim loop — periodic `XAutoClaim` of stale PEL entries
3. cleanup loop — periodic `XGroupDelConsumer` for idle consumers

Failure modes:

- invalid topology config (unknown `Mode`, node list without `Mode: cluster`, non-zero `DB` in cluster mode) → constructor returns an error
- redis ping fail at startup → stderr warning, core continues, sink degrades silently
- saver returns error → batch left in PEL, reclaimed on next autoclaim cycle
- producer storm → one token bucket per level class (`QPS` info/debug, `WarnQPS` warn, `ErrorQPS` error and above; default 50 each, negative = unlimited), checked on the caller goroutine before any field work, so an info flood never uses up the warn or error budget
- async queue full → only the incoming entry is dropped (entries already queued are kept)
- XADD error → entry dropped; stderr reports the first failure and then every 10,000th
- every drop is counted by reason (`Rate` / `Queue` / `XAddError`) and level: `hook.DropStats()` (process-wide) or `(*RedisStreamCore).DropStats()`
- oversized payload → replaced with `{"_truncated":true}` (cap via `MaxPayloadBytes`)
