package apptools

import (
	"testing"

	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

func TestResolveEndpoint_OptionOverridesEnv(t *testing.T) {
	const key = "TEST_OTLP_ENDPOINT"

	t.Setenv(key, "from-env")
	if got := ResolveEndpoint("from-option", key); got != "from-option" {
		t.Fatalf("ResolveEndpoint(option, env) = %q, want from-option", got)
	}
	if got := ResolveEndpoint("", key); got != "from-env" {
		t.Fatalf("ResolveEndpoint(empty, env) = %q, want from-env", got)
	}

	t.Setenv(key, "")
	if got := ResolveEndpoint("", key); got != "" {
		t.Fatalf("ResolveEndpoint(empty, empty) = %q, want empty", got)
	}
}

func TestResolveEndpoint_TrimsWhitespace(t *testing.T) {
	const key = "TEST_OTLP_ENDPOINT_WS"

	// Whitespace-only option is treated as unset, falls through to env.
	t.Setenv(key, "env-val")
	if got := ResolveEndpoint("   ", key); got != "env-val" {
		t.Fatalf("ResolveEndpoint(whitespace, env) = %q, want env-val", got)
	}
	// Surrounding whitespace on a real option value is trimmed.
	if got := ResolveEndpoint("  host:4317  ", key); got != "host:4317" {
		t.Fatalf("ResolveEndpoint trimmed option = %q, want host:4317", got)
	}
	// Whitespace-only env is treated as empty too.
	t.Setenv(key, "   ")
	if got := ResolveEndpoint("", key); got != "" {
		t.Fatalf("ResolveEndpoint(empty, whitespace-env) = %q, want empty", got)
	}
}

func TestResolveEndpoint_FirstNonEmptyEnvKeyWins(t *testing.T) {
	const specific = "TEST_OTLP_SPECIFIC"
	const generic = "TEST_OTLP_GENERIC"

	// Earlier key wins when set.
	t.Setenv(specific, "specific-val")
	t.Setenv(generic, "generic-val")
	if got := ResolveEndpoint("", specific, generic); got != "specific-val" {
		t.Fatalf("ResolveEndpoint = %q, want specific-val (first key wins)", got)
	}
	// Falls through to the next key when the earlier one is empty.
	t.Setenv(specific, "")
	if got := ResolveEndpoint("", specific, generic); got != "generic-val" {
		t.Fatalf("ResolveEndpoint = %q, want generic-val (fallthrough)", got)
	}
}

func TestNewTracerProvider_NoEndpoint_ReturnsNoop(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	tp, cleanup, err := NewTracerProvider()
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if _, ok := tp.(tracenoop.TracerProvider); !ok {
		t.Fatalf("provider = %T, want noop.TracerProvider (no exporter created)", tp)
	}
	if cleanup == nil {
		t.Fatal("cleanup must be non-nil")
	}
	cleanup() // must be a safe no-op
}

func TestNewTracerProvider_WithEndpoint_ReturnsRealProvider(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	tp, cleanup, err := NewTracerProvider(WithEndpoint("localhost:4317"))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if _, ok := tp.(tracenoop.TracerProvider); ok {
		t.Fatal("expected a real TracerProvider when endpoint is set, got noop")
	}
	if cleanup != nil {
		cleanup()
	}
}

// Regression guard for #1: a deployment configured only via the generic
// OTEL_EXPORTER_OTLP_ENDPOINT (no signal-specific var, no Option) must still
// export — the no-op guard must not short-circuit it.
func TestNewTracerProvider_GenericEnvEndpoint_ReturnsRealProvider(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")

	tp, cleanup, err := NewTracerProvider()
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if _, ok := tp.(tracenoop.TracerProvider); ok {
		t.Fatal("expected a real TracerProvider when only the generic env endpoint is set, got noop")
	}
	if cleanup != nil {
		cleanup()
	}
}

func TestNewPyroscope_NoAddress_ReturnsNil(t *testing.T) {
	t.Setenv("PYROSCOPE_ADHOC_SERVER_ADDRESS", "")

	p, err := NewPyroscope()
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if p != nil {
		t.Fatalf("profiler = %v, want nil (no profiler started)", p)
	}
}
