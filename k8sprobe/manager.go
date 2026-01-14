package k8sprobe

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sync"
)

// Cause is a type alias for string, often used to provide reasons or explanations within validation contexts.
type Cause = string

// EmptyCause represents an empty string for Cause, signifying the absence of a specific reason or explanation.
const EmptyCause Cause = ""

// ValidityChecker represents an interface for objects capable of validating their state and returning a boolean result.
// It is used for health checks or determining the validity of various entities within the system.
type ValidityChecker interface {

	// IsValid validates the current state of an object, returning a boolean indicating validity and a string explaining the reason.
	IsValid() (bool, Cause)
}

// Manager provides functionality to manage and monitor health probes for applications through a registry.
type Manager struct {
	mu     sync.RWMutex
	probes map[ProbeType][]ValidityChecker

	// Server configuration
	addr       string
	pathPrefix string
	httpServer *http.Server
}

// NewManager creates and returns a new instance of Manager with an initialized probe registry.
// It also applies any provided Options to configure the Manager.
func NewManager(opts ...Option) *Manager {
	m := &Manager{
		pathPrefix: "/healthz", // Default path prefix
		addr:       ":18080",
		probes:     make(map[ProbeType][]ValidityChecker),
	}

	for _, opt := range opts {
		opt(m)
	}

	m.initServer()
	return m
}

// initServer initializes the HTTP server or registers handlers based on configuration.
func (m *Manager) initServer() {
	if m.addr == "" {
		return
	}

	handler := NewHttpHandler(m)
	// Ensure pattern handles the dynamic probe type correctly
	// The UrlPathValue is "{probeType}", so the full pattern is "/healthz/{probeType}"
	pattern := m.pathPrefix + "/" + UrlPathValue

	// Start a new Server
	mux := http.NewServeMux()
	mux.Handle(pattern, handler)

	m.httpServer = &http.Server{
		Addr:    m.addr,
		Handler: mux,
	}

	m.startServer()
}

func (m *Manager) startServer() {
	go func() {
		// We ignore ErrServerClosed as it indicates a graceful shutdown
		if err := m.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("http probe listen err: %v\n", err)
		}
	}()
}

// Stop stops the internal HTTP server if it was started by the Manager.
func (m *Manager) Stop(ctx context.Context) error {
	if m.httpServer != nil {
		return m.httpServer.Shutdown(ctx)
	}
	return nil
}

// RegisterProbe registers a health probe of the specified type with the provided state retriever in the registry.
func (m *Manager) RegisterProbe(probeType ProbeType, probe ValidityChecker) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.probes[probeType] = append(m.probes[probeType], probe)
}

// checkProbe checks the status of the specified health probe type and returns true if the probe passes, otherwise false.
func (m *Manager) checkProbe(probeType ProbeType) (bool, Cause) {
	var probes []ValidityChecker
	m.mu.RLock()
	probes = slices.Clone(m.probes[probeType])
	m.mu.RUnlock()

	for _, probe := range probes {
		if isValid, cause := probe.IsValid(); !isValid {
			return false, cause
		}
	}

	return true, EmptyCause
}
