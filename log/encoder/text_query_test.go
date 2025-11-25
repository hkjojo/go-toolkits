package encoder

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func strptr(s string) *string { return &s }

func makeLine(ts, level, msg string, fields []string) string {
	b := ts + SPLIT + level + SPLIT
	for _, v := range fields {
		b += v + SPLIT
	}
	b += msg
	return b + "\n"
}

func writeLog(dir, prefix string, day time.Time, lines []string) (string, error) {
	name := prefix + "." + day.Format("20060102")
	p := filepath.Join(dir, name)
	fmt.Println("CREATE", p)
	fmt.Print(linesToString(lines))
	return p, os.WriteFile(p, []byte(fmt.Sprintf("%s", linesToString(lines))), 0o644)
}

func linesToString(lines []string) string {
	s := ""
	for _, l := range lines {
		s += l
	}
	return s
}

func TestQueryLogs_AllCases(t *testing.T) {
	dir := t.TempDir()
	prefix := "query.log"

	d1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	l1 := makeLine(d1.Add(1*time.Hour).Format(TextTimeLayout), "INFO", "alpha message", []string{"System", "1"})
	l2 := makeLine(d1.Add(12*time.Hour).Format(TextTimeLayout), "WARN", "beta message", []string{"Other", "2"})
	if _, err := writeLog(dir, prefix, d1, []string{l1, l2}); err != nil {
		t.Fatalf("write day1: %v", err)
	}

	l3 := makeLine(d2.Add(8*time.Hour).Format(TextTimeLayout), "INFO", "beta important", []string{"System", "3"})
	l4 := makeLine(d2.Add(20*time.Hour).Format(TextTimeLayout), "ERROR", "gamma message", []string{"System", "4"})
	if _, err := writeLog(dir, prefix, d2, []string{l3, l4}); err != nil {
		t.Fatalf("write day2: %v", err)
	}

	from := time.Date(d1.Year(), d1.Month(), d1.Day(), 0, 0, 0, 0, time.UTC).Format(TextTimeLayout)
	to := time.Date(d2.Year(), d2.Month(), d2.Day(), 23, 59, 59, 0, time.UTC).Format(TextTimeLayout)

	req := &ListLogReq{From: from, To: to, FieldNames: []string{"time", "level", "module", "id", "message"}}
	rep, err := QueryLogs(req, filepath.Join(dir, prefix))
	if err != nil {
		t.Fatalf("query all: %v", err)
	}
	fmt.Println("RESULT all")
	for _, lg := range rep.Logs {
		fmt.Println(lg.Fields["time"], lg.Fields["level"], lg.Fields["module"], lg.Fields["id"], lg.Fields["message"])
	}
	if len(rep.Logs) != 4 {
		t.Fatalf("expect 4 logs, got %d", len(rep.Logs))
	}

	st := "INFO"
	req2 := &ListLogReq{From: from, To: to, Level: &st, FieldNames: []string{"time", "level", "module", "id", "message"}}
	rep2, err := QueryLogs(req2, filepath.Join(dir, prefix))
	if err != nil {
		t.Fatalf("query level: %v", err)
	}
	fmt.Println("RESULT level=INFO")
	for _, lg := range rep2.Logs {
		fmt.Println(lg.Fields["time"], lg.Fields["level"], lg.Fields["module"], lg.Fields["id"], lg.Fields["message"])
	}
	if len(rep2.Logs) != 2 {
		t.Fatalf("expect 2 info logs, got %d", len(rep2.Logs))
	}

	msg := "beta"
	req3 := &ListLogReq{From: from, To: to, Message: &msg, FieldNames: []string{"time", "level", "module", "id", "message"}}
	rep3, err := QueryLogs(req3, filepath.Join(dir, prefix))
	if err != nil {
		t.Fatalf("query message: %v", err)
	}
	fmt.Println("RESULT message contains beta")
	for _, lg := range rep3.Logs {
		fmt.Println(lg.Fields["time"], lg.Fields["level"], lg.Fields["module"], lg.Fields["id"], lg.Fields["message"])
	}
	if len(rep3.Logs) != 2 {
		t.Fatalf("expect 2 beta logs, got %d", len(rep3.Logs))
	}

	mod := "System"
	req4 := &ListLogReq{From: from, To: to, Filters: map[string]*string{"module": &mod}, FieldNames: []string{"time", "level", "module", "id", "message"}}
	rep4, err := QueryLogs(req4, filepath.Join(dir, prefix))
	if err != nil {
		t.Fatalf("query field module: %v", err)
	}
	fmt.Println("RESULT module=System")
	for _, lg := range rep4.Logs {
		fmt.Println(lg.Fields["time"], lg.Fields["level"], lg.Fields["module"], lg.Fields["id"], lg.Fields["message"])
	}
	if len(rep4.Logs) != 3 {
		t.Fatalf("expect 3 module:System logs, got %d", len(rep4.Logs))
	}

	eqTime := d2.Add(8 * time.Hour).Format(TextTimeLayout)
	req5 := &ListLogReq{From: from, To: to, Filters: map[string]*string{"time": strptr(eqTime)}, FieldNames: []string{"time", "level", "module", "id", "message"}}
	rep5, err := QueryLogs(req5, filepath.Join(dir, prefix))
	if err != nil {
		t.Fatalf("query filter time: %v", err)
	}
	fmt.Println("RESULT time=", eqTime)
	for _, lg := range rep5.Logs {
		fmt.Println(lg.Fields["time"], lg.Fields["level"], lg.Fields["module"], lg.Fields["id"], lg.Fields["message"])
	}
	if len(rep5.Logs) != 1 {
		t.Fatalf("expect 1 log at exact time, got %d", len(rep5.Logs))
	}

	_ = from
	_ = to

	badReq := &ListLogReq{From: "not-a-ms", To: to}
	if _, err := QueryLogs(badReq, filepath.Join(dir, prefix)); err == nil {
		t.Fatalf("expect error for invalid time range")
	}

	unsorted := []*Log{
		{Fields: map[string]string{"time": d2.Add(20 * time.Hour).Format(TextTimeLayout)}},
		{Fields: map[string]string{"time": d1.Add(1 * time.Hour).Format(TextTimeLayout)}},
		{Fields: map[string]string{"time": d2.Add(8 * time.Hour).Format(TextTimeLayout)}},
	}
	mgr := newManager(req, filepath.Join(dir, prefix), 0)
	sorted := mgr.sortResults(unsorted)
	if sorted.Logs[0].Fields["time"] != d1.Add(1*time.Hour).Format(TextTimeLayout) || sorted.Logs[2].Fields["time"] != d2.Add(20*time.Hour).Format(TextTimeLayout) {
		t.Fatalf("sort ascending failed")
	}
}

func TestQueryLogs_NoFields(t *testing.T) {
	dir := t.TempDir()
	prefix := "nofields.log"
	d := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)

	l1 := makeLine(d.Add(1*time.Hour).Format(TextTimeLayout), "INFO", "hello", []string{})
	l2 := makeLine(d.Add(2*time.Hour).Format(TextTimeLayout), "WARN", "", []string{})
	if _, err := writeLog(dir, prefix, d, []string{l1, l2}); err != nil {
		t.Fatalf("write nofields: %v", err)
	}

	from := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC).Format(TextTimeLayout)
	to := time.Date(d.Year(), d.Month(), d.Day(), 23, 59, 59, 0, time.UTC).Format(TextTimeLayout)

	req := &ListLogReq{From: from, To: to, FieldNames: []string{"time", "level", "message"}}
	rep, err := QueryLogs(req, filepath.Join(dir, prefix))
	if err != nil {
		t.Fatalf("query nofields all: %v", err)
	}
	fmt.Println("RESULT nofields all")
	for _, lg := range rep.Logs {
		fmt.Println(lg.Fields["time"], lg.Fields["level"], lg.Fields["message"])
	}
	if len(rep.Logs) != 2 {
		t.Fatalf("expect 2 logs, got %d", len(rep.Logs))
	}
	emptyMsgCount := 0
	for _, lg := range rep.Logs {
		if lg.Fields["message"] == "" {
			emptyMsgCount++
		}
	}
	if emptyMsgCount != 1 {
		t.Fatalf("expect 1 empty message, got %d", emptyMsgCount)
	}

	st := "WARN"
	req2 := &ListLogReq{From: from, To: to, Level: &st, FieldNames: []string{"time", "level", "message"}}
	rep2, err := QueryLogs(req2, filepath.Join(dir, prefix))
	if err != nil {
		t.Fatalf("query nofields level: %v", err)
	}
	fmt.Println("RESULT nofields level=WARN")
	for _, lg := range rep2.Logs {
		fmt.Println(lg.Fields["time"], lg.Fields["level"], lg.Fields["message"])
	}
	if len(rep2.Logs) != 1 {
		t.Fatalf("expect 1 warn, got %d", len(rep2.Logs))
	}

	msg := "hello"
	req3 := &ListLogReq{From: from, To: to, Message: &msg, FieldNames: []string{"time", "level", "message"}}
	rep3, err := QueryLogs(req3, filepath.Join(dir, prefix))
	if err != nil {
		t.Fatalf("query nofields message: %v", err)
	}
	fmt.Println("RESULT nofields message contains hello")
	for _, lg := range rep3.Logs {
		fmt.Println(lg.Fields["time"], lg.Fields["level"], lg.Fields["message"])
	}
	if len(rep3.Logs) != 1 {
		t.Fatalf("expect 1 hello, got %d", len(rep3.Logs))
	}
}

func TestQueryLogs_InvalidFirstField(t *testing.T) {
	dir := t.TempDir()
	prefix := "invalidfirst.log"
	d := time.Date(2025, 1, 4, 0, 0, 0, 0, time.UTC)
	_, _ = writeLog(dir, prefix, d, []string{})

	from := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC).Format(TextTimeLayout)
	to := time.Date(d.Year(), d.Month(), d.Day(), 23, 59, 59, 0, time.UTC).Format(TextTimeLayout)

	req := &ListLogReq{From: from, To: to, FieldNames: []string{"level", "time", "message"}}
	if _, err := QueryLogs(req, filepath.Join(dir, prefix)); err == nil {
		t.Fatalf("expect error when first field is not time")
	}
}
