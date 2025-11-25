// Package encoder Text log query utilities
// Usage:
// - Build a ListLogReq with From/To in TextTimeLayout (UTC) and FieldNames defining the log column order.
// - FieldNames must include "time"; its index determines which column is parsed as time.
// - Optional: set Separator (default tab), Level, Message and Filters for matching.
// - Logs are read from daily files named <pathPrefix>.YYYYMMDD.
// - Call QueryLogs(req, pathPrefix) to get ListLogRep with each record in Fields keyed by FieldNames.
package encoder

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const logLimit = 10000
const dataSize = 1024 * 1024 * 200

var total int32

type Manager struct {
	timeParser   *timeParser
	logDir       string
	filePrefix   string
	needMsgMatch bool
	msgPattern   []byte
	limit        int32
	separator    string
	fieldNames   []string
}

type timeParser struct {
	buf [24]byte
}

type ListLogReq struct {
	From       string             // inclusive start time, format TextTimeLayout (UTC)
	To         string             // inclusive end time, format TextTimeLayout (UTC)
	Level      *string            // optional exact level filter (e.g., "INFO"), nil = no filter
	Message    *string            // optional substring match on message field, nil = no filter
	Separator  *string            // optional field separator (default: tab "\t")
	FieldNames []string           // ordered field names for each log part; must include "time"
	Filters    map[string]*string // optional exact-match filters by field name; "message" uses contains
}

type Log struct {
	Fields map[string]string
}

type ListLogRep struct {
	Logs []*Log
}

func newManager(req *ListLogReq, path string, limit int32) *Manager {
	index := strings.LastIndex(path, "/")
	mgr := &Manager{
		timeParser: &timeParser{},
		logDir:     path[:index],
		filePrefix: path[index+1:] + ".",
		limit:      logLimit,
	}

	if limit > 0 {
		mgr.limit = limit
	}

	if req.Message != nil {
		mgr.needMsgMatch = true
		mgr.msgPattern = []byte(*req.Message)
	}
	mgr.separator = SPLIT
	if req.Separator != nil {
		mgr.separator = *req.Separator
	}
	mgr.fieldNames = req.FieldNames
	total = 0

	return mgr
}

type chunkRange struct {
	Start int // include the start
	End   int // exclude the end
}

func QueryLogs(req *ListLogReq, path string) (*ListLogRep, error) {
	mgr := newManager(req, path, logLimit)
	if len(req.FieldNames) == 0 || req.FieldNames[0] != "time" {
		return nil, fmt.Errorf("first field must be time")
	}
	fromTime, toTime, err := parseTimeRange(req.From, req.To)
	if err != nil {
		return nil, fmt.Errorf("invalid time range: %v", err)
	}

	filePaths := mgr.generateLogFilePaths(fromTime, toTime)
	results, err := mgr.processFiles(filePaths, req)
	if err != nil {
		return nil, err
	}

	return &ListLogRep{Logs: results}, nil
}

func parseTimeRange(fromStr, toStr string) (from, to time.Time, err error) {
	loc, _ := time.LoadLocation("UTC")
	f, err := time.Parse(TextTimeLayout, strings.TrimSpace(fromStr))
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	t, err := time.Parse(TextTimeLayout, strings.TrimSpace(toStr))
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	f = f.In(loc)
	t = t.In(loc)
	from = time.Date(f.Year(), f.Month(), f.Day(), 0, 0, 0, 0, loc)
	to = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, loc)
	return
}

func (m *Manager) generateLogFilePaths(from, to time.Time) []string {
	var paths []string

	for d := from; d.Before(to) || d.Equal(to); d = d.AddDate(0, 0, 1) {
		filename := m.filePrefix + d.Format("20060102")
		path := filepath.Join(m.logDir, filename)

		if _, err := os.Stat(path); err == nil {
			paths = append(paths, path)
		}
	}
	return paths
}

func (m *Manager) processFiles(paths []string, req *ListLogReq) ([]*Log, error) {
	var finalResults []*Log

	for _, path := range paths {
		if total >= m.limit {
			break
		}

		results, err := m.processLogFile(path, req)
		if err != nil {
			return nil, err
		}

		finalResults = append(finalResults, results...)
	}

	return finalResults, nil
}

// process single log file
func (m *Manager) processLogFile(path string, req *ListLogReq) ([]*Log, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var chunkNum = 1
	if len(data) > dataSize {
		chunkNum = len(data) / dataSize
	}

	chunks := splitDataToChunks(data, chunkNum)

	var results []*Log

	for _, chunk := range chunks {
		if total >= m.limit {
			break
		}
		results = append(results, m.processChunk(data, chunk, req)...)
	}

	return results, nil
}

// split single log data to chunks
func splitDataToChunks(data []byte, chunkNum int) []chunkRange {
	totalSize := len(data)
	chunkSize := totalSize / chunkNum
	chunks := make([]chunkRange, 0, chunkNum)

	start := 0
	for i := 0; i < chunkNum; i++ {
		end := start + chunkSize
		if i == chunkNum-1 {
			end = totalSize
		} else {
			// find the next '\n'
			for end < totalSize && data[end] != '\n' {
				end++
			}
			if end < totalSize {
				// include the '\n'
				end++
			}
		}
		chunks = append(chunks, chunkRange{Start: start, End: end})
		start = end
	}

	return chunks
}

func (p *timeParser) Parse(b []byte) (time.Time, error) {
	if len(b) < 24 {
		return time.Time{}, fmt.Errorf("invalid time length")
	}
	copy(p.buf[:], b[:24])
	return time.Parse(TextTimeLayout, string(p.buf[:]))
}

func (m *Manager) getChunkTimeRange(data []byte, cr chunkRange) (from, to time.Time, valid bool) {
	if lineStart := cr.Start; lineStart+24 < cr.End {
		if t, err := m.timeParser.Parse(data[lineStart:]); err == nil {
			from = t
			valid = true
		}
	}

	for i := cr.End - 1; i >= cr.Start; i-- {
		if string(data[i-2:i]) == "Z\t" && i-25 >= cr.Start {
			if t, err := m.timeParser.Parse(data[i-25 : i-1]); err == nil {
				to = t
				return from, to, valid
			}
			break

		}
	}
	return from, to, false
}

func (m *Manager) processChunk(data []byte, cr chunkRange, req *ListLogReq) []*Log {
	defer func() {
		if err := recover(); err != nil {
		}
	}()

	reqFrom, _ := m.timeParser.Parse([]byte(strings.TrimSpace(req.From)))
	reqTo, _ := m.timeParser.Parse([]byte(strings.TrimSpace(req.To)))
	chunkFrom, chunkTo, ok := m.getChunkTimeRange(data, cr)
	if ok && (chunkFrom.After(reqTo) || chunkTo.Before(reqFrom)) {
		return nil
	}

	var results []*Log

	start := cr.Start
	if cr.Start > 0 {
		for start < cr.End && data[start-1] != '\n' {
			start++
		}
	}

	searchStart := start
	for total < m.limit {
		var lineData []byte
		var lineStart, lineEnd int

		if m.needMsgMatch {
			patternPos := bytes.Index(data[searchStart:cr.End], m.msgPattern)
			if patternPos == -1 {
				break
			}

			lineStart = searchStart + patternPos
			for lineStart > searchStart && data[lineStart-1] != '\n' {
				lineStart--
			}
			lineEnd = searchStart + patternPos
			for lineEnd < cr.End && data[lineEnd] != '\n' {
				lineEnd++
			}

			lineData = data[lineStart:lineEnd]
			searchStart = lineEnd + 1
		} else {
			if start >= cr.End {
				break
			}

			lineEnd = bytes.IndexByte(data[start:cr.End], '\n')
			if lineEnd == -1 {
				lineEnd = cr.End
			} else {
				lineEnd += start
			}

			lineData = data[start:lineEnd]
			start = lineEnd + 1
		}

		sep := m.separator
		logEntry, err := parseLogLine(string(lineData), sep, m.fieldNames)
		if err == nil && m.matchFilters(logEntry, req, reqFrom, reqTo) {
			results = append(results, logEntry)
			total++
		}
	}

	return results
}

func parseLogLine(line string, sep string, fieldNames []string) (*Log, error) {
	parts := strings.Split(line, sep)
	if len(fieldNames) == 0 || len(parts) < len(fieldNames) {
		return nil, errors.New("invalid log format")
	}
	entry := &Log{Fields: make(map[string]string)}
	for i := 0; i < len(fieldNames); i++ {
		name := fieldNames[i]
		val := parts[i]
		if name != "" {
			entry.Fields[name] = val
		}
	}
	return entry, nil
}

func (m *Manager) matchFilters(entry *Log, req *ListLogReq, reqFrom, reqTo time.Time) bool {
	if tf := entry.Fields["time"]; tf != "" {
		if t, err := m.timeParser.Parse([]byte(tf)); err != nil {
			return false
		} else {
			if t.Before(reqFrom) || t.After(reqTo) {
				return false
			}
		}
	}
	if req.Level != nil && *req.Level != entry.Fields["level"] {
		return false
	}
	if req.Message != nil && !strings.Contains(entry.Fields["message"], *req.Message) {
		return false
	}
	if req.Filters != nil {
		for k, v := range req.Filters {
			if v == nil {
				continue
			}
			if entry.Fields[k] != *v {
				return false
			}
		}
	}
	return true
}

func (m *Manager) sortResults(results []*Log) *ListLogRep {
	type sortItem struct {
		index      int
		parsedTime time.Time
	}

	sem := make(chan struct{}, runtime.NumCPU()*2)
	items := make([]sortItem, len(results))
	var wg sync.WaitGroup

	for i := range results {
		wg.Add(1)
		go func(idx int) {
			sem <- struct{}{}
			defer func() {
				<-sem
				wg.Done()
			}()

			if t, err := m.timeParser.Parse([]byte(results[idx].Fields["time"])); err == nil {
				items[idx] = sortItem{idx, t}
			}
		}(i)
	}
	wg.Wait()

	// sort index
	sort.Slice(items, func(i, j int) bool {
		return items[i].parsedTime.Before(items[j].parsedTime)
	})

	// combine results
	sorted := make([]*Log, len(results))
	for i, item := range items {
		sorted[i] = results[item.index]
	}
	return &ListLogRep{Logs: sorted}
}
