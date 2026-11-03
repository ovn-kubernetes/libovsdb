package test

import (
	"sync"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// Package-level tracers bind to the first global provider, so keep it alive for
// the test process rather than replacing or shutting it down between tests.
// With no exporters or batch processors, this provider has no background work.
var tracerProvider = sync.OnceValue(func() *sdktrace.TracerProvider {
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	return provider
})

// NewSpanRecorder captures spans for a test and unregisters the recorder during
// cleanup. Each test gets fresh recordings, including repeated runs with -count.
// Tests using it must not run in parallel or replace the global tracer provider.
func NewSpanRecorder(t testing.TB) *tracetest.SpanRecorder {
	t.Helper()
	provider := tracerProvider()
	recorder := tracetest.NewSpanRecorder()
	provider.RegisterSpanProcessor(recorder)
	t.Cleanup(func() { provider.UnregisterSpanProcessor(recorder) })
	return recorder
}
