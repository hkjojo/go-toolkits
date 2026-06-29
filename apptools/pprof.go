package apptools

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"strconv"
	"strings"
	"time"
)

// NewPprofServer 起一个独立 debug 端口的 pprof server,供 `go tool pprof` 按需拉取
// (与 NewPyroscope 的持续推送互补:一个拉、一个推)。PPROF_ADDR 未配 → no-op。
//
// 刻意不 import net/http/pprof:它的 init() 会把 handler 注册进 http.DefaultServeMux,
// 而不少服务用 DefaultServeMux 作 NotFoundHandler,会让主业务端口被动暴露 pprof(且无法关)。
// 这里用 runtime/pprof + runtime/trace 自实现 handler,挂自有 mux + 独立端口,对调用方零副作用。
func NewPprofServer() (func(), error) {
	addr := os.Getenv("PPROF_ADDR")
	if addr == "" {
		return func() {}, nil
	}

	// 不设 WriteTimeout:避免长时 CPU/trace profile 被 server 写超时截断
	// (时长由请求的 ?seconds= 自行控制)。ReadHeaderTimeout 防 slowloris。
	srv := &http.Server{
		Addr:              addr,
		Handler:           pprofMux(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		_ = srv.ListenAndServe()
	}()

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}, nil
}

func pprofMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/profile", pprofCPU)
	mux.HandleFunc("/debug/pprof/trace", pprofTrace)
	mux.HandleFunc("/debug/pprof/cmdline", pprofCmdline)
	// 兜底子树:索引页 + 命名 profile(heap/goroutine/allocs/block/mutex/threadcreate)。
	mux.HandleFunc("/debug/pprof/", pprofNamed)
	return mux
}

// pprofCPU 复刻 net/http/pprof.Profile(改用 runtime/pprof,规避 net/http/pprof 的 init 副作用)。
func pprofCPU(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	sec, err := strconv.ParseInt(r.FormValue("seconds"), 10, 64)
	if sec <= 0 || err != nil {
		sec = 30
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="profile"`)
	if err := pprof.StartCPUProfile(w); err != nil {
		http.Error(w, fmt.Sprintf("Could not enable CPU profiling: %s", err), http.StatusInternalServerError)
		return
	}
	sleep(r, time.Duration(sec)*time.Second)
	pprof.StopCPUProfile()
}

// pprofTrace 复刻 net/http/pprof.Trace。
func pprofTrace(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	sec, err := strconv.ParseFloat(r.FormValue("seconds"), 64)
	if sec <= 0 || err != nil {
		sec = 1
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="trace"`)
	if err := trace.Start(w); err != nil {
		http.Error(w, fmt.Sprintf("Could not enable tracing: %s", err), http.StatusInternalServerError)
		return
	}
	sleep(r, time.Duration(sec*float64(time.Second)))
	trace.Stop()
}

func pprofCmdline(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, strings.Join(os.Args, "\x00"))
}

// pprofNamed 服务命名 profile(heap/goroutine/allocs/block/mutex/threadcreate)与索引页。
// 复刻 net/http/pprof 的 named-handler 行为(不含 delta/seconds 增量采样)。
func pprofNamed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	name := strings.TrimPrefix(r.URL.Path, "/debug/pprof/")
	if name == "" {
		writePprofIndex(w)
		return
	}
	p := pprof.Lookup(name)
	if p == nil {
		http.Error(w, "Unknown profile", http.StatusNotFound)
		return
	}
	if gc, _ := strconv.Atoi(r.FormValue("gc")); name == "heap" && gc > 0 {
		runtime.GC()
	}
	debug, _ := strconv.Atoi(r.FormValue("debug"))
	if debug != 0 {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, name))
	}
	_ = p.WriteTo(w, debug)
}

func writePprofIndex(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	bw := bufio.NewWriter(w)
	defer bw.Flush()
	fmt.Fprint(bw, "/debug/pprof/\n\nprofile  (CPU, ?seconds=N)\ntrace    (?seconds=N)\ncmdline\n")
	for _, p := range pprof.Profiles() {
		fmt.Fprintf(bw, "%-13s (%d)\n", p.Name(), p.Count())
	}
}

// sleep 在 d 到期或请求被取消(client 断开)时返回,使长 profile 可被 client 主动中止。
func sleep(r *http.Request, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-r.Context().Done():
	}
}
