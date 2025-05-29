package kratos

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/hkjojo/go-toolkits/log/v2"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	pbc "git.gonit.codes/dealer/actshub/protocol/go/common/v1"
	mp "github.com/edsrzf/mmap-go"
)

const logLimit = 10000
const dataSize = 1024 * 1024 * 200
const logTimeLayout = "2006-01-02T15:04:05.000Z"
const splitForm = "\t"

var total int32

type Manager struct {
	timeParser   *timeParser
	logDir       string
	filePrefix   string
	needMsgMatch bool
	msgPattern   []byte
	limit        int32
}

type timeParser struct {
	buf [24]byte
}

func newManager(req *pbc.ListLogReq, path string, limit int32) *Manager {
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

	return mgr
}

type chunkRange struct {
	Start int // include the start
	End   int // exclude the end
}

func QueryLogs(req *pbc.ListLogReq, path string) (*pbc.ListLogRep, error) {
	log.Infow("req info", "path", path, "req", req)
	mgr := newManager(req, path, logLimit)
	fromTime, toTime, err := parseTimeRange(req.From, req.To)
	if err != nil {
		return nil, fmt.Errorf("invalid time range: %v", err)
	}

	filePaths := mgr.generateLogFilePaths(fromTime, toTime)
	log.Infow("file paths", "filePaths", filePaths)
	results, err := mgr.processFiles(filePaths, req)
	if err != nil {
		return nil, err
	}

	return &pbc.ListLogRep{Logs: results}, nil
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

func (m *Manager) processFiles(paths []string, req *pbc.ListLogReq) ([]*pbc.ListLogRep_Log, error) {
	var finalResults []*pbc.ListLogRep_Log

	for _, path := range paths {
		if total >= m.limit {
			log.Infow("log limit reached", "path", path)
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
func (m *Manager) processLogFile(path string, req *pbc.ListLogReq) ([]*pbc.ListLogRep_Log, error) {
	log.Infow("process file info", "path", path, "req", req)
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := mp.Map(f, mp.RDWR, 0)
	if err != nil {
		return nil, err
	}
	defer data.Unmap()

	var chunkNum = 1
	if len(data) > dataSize {
		chunkNum = len(data) / dataSize
	}

	chunks := splitDataToChunks(data, chunkNum)

	var results []*pbc.ListLogRep_Log

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

func (m *Manager) processChunk(data []byte, cr chunkRange, req *pbc.ListLogReq) []*pbc.ListLogRep_Log {
	defer func() {
		if err := recover(); err != nil {
		}
	}()

	reqFrom, _ := time.Parse(logTimeLayout, req.From)
	reqTo, _ := time.Parse(logTimeLayout, req.To)
	chunkFrom, chunkTo, ok := m.getChunkTimeRange(data, cr)
	if ok && (chunkFrom.After(reqTo) || chunkTo.Before(reqFrom)) {
		return nil
	}

	var results []*pbc.ListLogRep_Log

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

		logEntry, err := parseLogLine(string(lineData))
		if err == nil && matchFilters(logEntry, req) {
			results = append(results, logEntry)
			total++
		}
	}

	return results
}

func parseLogLine(line string) (*pbc.ListLogRep_Log, error) {
	parts := strings.Split(line, splitForm)
	if len(parts) < 5 {
		return nil, errors.New("invalid log format")
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
	if entry.Time < req.From || entry.Time > req.To {
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
