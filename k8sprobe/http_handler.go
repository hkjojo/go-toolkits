package k8sprobe

import (
	"net/http"
	"strings"
)

// HttpHandler serves as an HTTP handler for processing health probe requests based on the Manager's registered probes.
type HttpHandler struct {
	manager *Manager
}

// NewHttpHandler creates and returns a new instance of HttpHandler with the provided Manager.
func NewHttpHandler(manager *Manager) *HttpHandler {
	return &HttpHandler{
		manager: manager,
	}
}

// UrlPathValue is the constant key used to extract the probe type from a request's URL path parameters.
const UrlPathValue = "{probeType}"

// ServeHTTP handles incoming HTTP requests and returns the status of the specified health probe type.
func (h *HttpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var probeType ProbeType
	// remove braces from the path value
	pathValue := strings.Trim(UrlPathValue, "{}")
	switch r.PathValue(pathValue) {
	case LivenessProbe.String():
		probeType = LivenessProbe
	case ReadinessProbe.String():
		probeType = ReadinessProbe
	case StartupProbe.String():
		probeType = StartupProbe
	default:
		http.Error(w, "Invalid probe type", http.StatusBadRequest)
		return
	}

	if isValid, cause := h.manager.checkProbe(probeType); isValid {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(cause))
	}
}
