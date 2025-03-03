package kratos

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	pbc "git.gonit.codes/dealer/actshub/protocol/go/common/v1"

	"golang.org/x/exp/mmap"
)

const (
	logTimeLayout = "2006-01-02T15:04:05.000Z"
)

type Manager struct {
	timeParser *timeParser
	logDir     string
	filePrefix string
}

type timeParser struct {
	buf [24]byte
}

func newManager(path string) *Manager {
	index := strings.LastIndex(path, "/")
	return &Manager{
		timeParser: &timeParser{},
		logDir:     path[:index],
		filePrefix: path[index+1:] + ".",
	}
}

type chunkRange struct {
	Start int // include the start
	End   int // exclude the end
}

func QueryLogs(req *pbc.ListLogReq, path string) (*pbc.ListLogRep, error) {
	mgr := newManager(path)
	fromTime, toTime, err := parseTimeRange(req.From, req.To)
	if err != nil {
		return nil, fmt.Errorf("invalid time range: %v", err)
	}

	filePaths := mgr.generateLogFilePaths(fromTime, toTime)

	results := mgr.processFilesConcurrently(filePaths, req)

	return mgr.sortResults(results), nil
}

func parseTimeRange(fromStr, toStr string) (from, to time.Time, err error) {
	fromStr = fromStr[:10]
	toStr = toStr[:10]
	loc, _ := time.LoadLocation("UTC")

	if from, err = time.ParseInLocation("2006-01-02", fromStr, loc); err != nil {
		return
	}
	if to, err = time.ParseInLocation("2006-01-02", toStr, loc); err != nil {
		return
	}

	to = to.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
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

func (m *Manager) processFilesConcurrently(paths []string, req *pbc.ListLogReq) []*pbc.ListLogRep_Log {
	var wg sync.WaitGroup
	results := make(chan []*pbc.ListLogRep_Log)

	for _, path := range paths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			if res, err := m.processLogFile(p, req); err == nil {
				results <- res
			}
		}(path)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var finalResults []*pbc.ListLogRep_Log
	for res := range results {
		finalResults = append(finalResults, res...)
	}
	return finalResults
}

// process single log file
func (m *Manager) processLogFile(path string, req *pbc.ListLogReq) ([]*pbc.ListLogRep_Log, error) {
	readerAt, err := mmap.Open(path)
	if err != nil {
		return nil, err
	}
	defer readerAt.Close()

	data := make([]byte, readerAt.Len())
	_, err = readerAt.ReadAt(data, 0)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	chunks := splitDataToChunks(data, runtime.NumCPU()*2)

	var wg sync.WaitGroup
	resultChan := make(chan []*pbc.ListLogRep_Log, len(chunks))

	for _, chunk := range chunks {
		wg.Add(1)
		go func(c chunkRange) {
			defer wg.Done()
			results := m.processChunk(data, c, req)
			resultChan <- results
		}(chunk)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	finalResults := make([]*pbc.ListLogRep_Log, 0)
	for res := range resultChan {
		finalResults = append(finalResults, res...)
	}

	return finalResults, nil
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
	return time.Parse(logTimeLayout, string(p.buf[:]))
}

func (m *Manager) getChunkTimeRange(data []byte, cr chunkRange) (from, to time.Time, valid bool) {
	if lineStart := cr.Start; lineStart+24 < cr.End {
		if t, err := m.timeParser.Parse(data[lineStart:]); err == nil {
			from = t
			valid = true
		}
	}

	for i := cr.End - 1; i >= cr.Start; i-- {
		if string(data[i-3:i]) == "Z->" && i-26 >= cr.Start {
			if t, err := m.timeParser.Parse(data[i-26 : i-2]); err == nil {
				to = t
				return from, to, valid
			}
			break

		}
	}
	return from, to, false
}

func (m *Manager) processChunk(data []byte, cr chunkRange, req *pbc.ListLogReq) []*pbc.ListLogRep_Log {
	defer func() {
		if err := recover(); err != nil {
		}
	}()

	reqFrom, _ := time.Parse(logTimeLayout, req.From)
	reqTo, _ := time.Parse(logTimeLayout, req.To)
	chunkFrom, chunkTo, ok := m.getChunkTimeRange(data, cr)
	if !ok || chunkFrom.After(reqTo) || chunkTo.Before(reqFrom) {
		return nil
	}

	var results []*pbc.ListLogRep_Log

	start := cr.Start
	if cr.Start > 0 {
		for start < cr.End && data[start-1] != '\n' {
			start++
		}
	}

	end := cr.End
	for i := start; i < end; i++ {
		if data[i] == '\n' {
			line := string(data[start:i])
			start = i + 1

			logEntry, err := parseLogLine(line)
			if err == nil && matchFilters(logEntry, req) {
				results = append(results, logEntry)
			}
		}
	}

	// handle the last line
	if start < end {
		line := string(data[start:end])
		logEntry, err := parseLogLine(line)
		if err == nil && matchFilters(logEntry, req) {
			results = append(results, logEntry)
		}
	}

	return results
}

func parseLogLine(line string) (*pbc.ListLogRep_Log, error) {
	parts := strings.Split(line, "->")
	if len(parts) < 5 {
		return nil, errors.New("invalid log format")
	}

	for i := range parts {
		parts[i] = strings.TrimRight(parts[i], "-")
	}

	return &pbc.ListLogRep_Log{
		Time:    parts[0],
		Status:  parts[1],
		Module:  parts[2],
		Source:  parts[3],
		Message: parts[4],
	}, nil
}

func matchFilters(entry *pbc.ListLogRep_Log, req *pbc.ListLogReq) bool {
	logTime, err := time.Parse(logTimeLayout, entry.Time)
	if err != nil {
		return false
	}

	reqFrom, _ := time.Parse(logTimeLayout, req.From)
	reqTo, _ := time.Parse(logTimeLayout, req.To)

	// filter time
	if logTime.Before(reqFrom) || logTime.After(reqTo) {
		return false
	}
	// filter status
	if req.Status != nil && *req.Status != entry.Status {
		return false
	}
	// filter module
	if req.Module != nil && *req.Module != entry.Module {
		return false
	}
	// filter source
	if req.Source != nil && *req.Source != entry.Source {
		return false
	}
	// filter message
	if req.Message != nil && !strings.Contains(entry.Message, *req.Message) {
		return false
	}

	return true
}

func (m *Manager) sortResults(results []*pbc.ListLogRep_Log) *pbc.ListLogRep {
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

			if t, err := m.timeParser.Parse([]byte(results[idx].Time)); err == nil {
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
	sorted := make([]*pbc.ListLogRep_Log, len(results))
	for i, item := range items {
		sorted[i] = results[item.index]
	}
	return &pbc.ListLogRep{Logs: sorted}
}
