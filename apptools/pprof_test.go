package apptools

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPprofMux_NamedProfilesAndIndex(t *testing.T) {
	mux := pprofMux()

	cases := []struct {
		path       string
		wantStatus int
		wantBody   bool // 期望非空 body
	}{
		{"/debug/pprof/", http.StatusOK, true},
		{"/debug/pprof/heap", http.StatusOK, true},
		{"/debug/pprof/goroutine", http.StatusOK, true},
		{"/debug/pprof/cmdline", http.StatusOK, true},
		{"/debug/pprof/definitely-not-a-profile", http.StatusNotFound, false},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
		if rec.Code != c.wantStatus {
			t.Fatalf("GET %s: got %d, want %d", c.path, rec.Code, c.wantStatus)
		}
		if c.wantBody && rec.Body.Len() == 0 {
			t.Fatalf("GET %s: empty body, want non-empty", c.path)
		}
	}
}

func TestPprofIndex_ListsProfiles(t *testing.T) {
	rec := httptest.NewRecorder()
	pprofMux().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil))
	body := rec.Body.String()
	for _, want := range []string{"profile", "trace", "heap", "goroutine"} {
		if !strings.Contains(body, want) {
			t.Fatalf("index missing %q; body=%q", want, body)
		}
	}
}
