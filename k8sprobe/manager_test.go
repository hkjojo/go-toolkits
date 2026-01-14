package k8sprobe

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestServe_WithAddress(t *testing.T) {
	// Use a likely free port for testing
	targetAddr := "127.0.0.1:54321"

	// Initialize Manager with Option
	manager := NewManager(WithAddress(targetAddr))
	manager.RegisterProbe(LivenessProbe, &defaultProbe{})

	defer func() {
		if err := manager.Stop(context.Background()); err != nil {
			t.Logf("Stop failed: %v", err)
		}
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://" + targetAddr + "/healthz/live")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

type defaultProbe struct{}

func (d defaultProbe) IsValid() (bool, Cause) {
	return true, EmptyCause
}
