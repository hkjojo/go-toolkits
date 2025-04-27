package kratos

import (
	"fmt"
	pbc "git.gonit.codes/dealer/actshub/protocol/go/common/v1"
	"github.com/hkjojo/go-toolkits/apptools"
	tklog "github.com/hkjojo/go-toolkits/log/v2"
	"os"
	"testing"
	"time"
)

func TestTextLog(t *testing.T) {
	kfg := &tklog.Config{
		Path:   "./demo",
		Level:  "debug",
		Format: "text",
	}

	zLog, err := NewZapLog(kfg)
	if err != nil {
		t.Fatal(err)
	}
	finalLogger := apptools.WithMetaKeys(zLog)
	testLogger := NewActsHelper(finalLogger)
	testLogger.Infow("System", "DB", "this is a test")
}

func TestQueryLog(t *testing.T) {
	//status := "INFO"
	msg := "1633261"
	startTime := time.Now()
	resp, err := QueryLogs(&pbc.ListLogReq{
		From: "2025-04-24T00:00:00.000Z",
		To:   "2025-04-24T23:00:00.000Z",
		//Status: &status,
		Message: &msg,
	}, "./history.log")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("elapsed:", time.Since(startTime).String())

	t.Logf("count: %d", len(resp.Logs))
}

//func TestQueryMore(t *testing.T) {
//	//status := "INFO"
//	msg := "1633261"
//	startTime := time.Now()
//	resp, err := QueryLogsConcurrently(&pbc.ListLogReq{
//		From: "2025-04-24T00:00:00.000Z",
//		To:   "2025-04-24T23:00:00.000Z",
//		//Status:  &status,
//		Message: &msg,
//	}, "./history.log")
//	if err != nil {
//		t.Fatal(err)
//	}
//	t.Log("elapsed:", time.Since(startTime).String())
//
//	t.Logf("count: %d", len(resp.Logs))
//}

func TestRsQueryLog(t *testing.T) {
	err := os.Setenv("LD_LIBRARY_PATH", "./libs")
	if err != nil {
		return
	}
	for _, env := range os.Environ() {
		fmt.Println(env)
	}
	path := "/home/shawn/work/whhl/project/go/hkjojo/go-toolkits/log/kratos/"
	results, err := RsQueryLogs(path, &QueryParams{
		From:    "2025-04-15T05:13:01.454Z",
		To:      "2025-04-15T23:13:01.454Z",
		Service: "",
		Status:  "",
		Module:  "",
		Source:  "",
		Message: "1633261",
	})

	if err != nil {
		panic(err)
	}

	fmt.Printf("Found %d logs:\n", len(results))
	for i, log := range results {
		fmt.Printf("[%d] %s %s %s %s %s\n",
			i+1,
			log.Time,
			log.Status,
			log.Module,
			log.Source,
			log.Message)
	}
}
