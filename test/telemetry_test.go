package test_test

import (
	"context"
	"testing"

	"github.com/ovn-kubernetes/libovsdb/cache"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/ovn-kubernetes/libovsdb/test"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestSpanRecorderLifecycle(t *testing.T) {
	dbModel, err := test.GetModel()
	require.NoError(t, err)
	tableCache, err := cache.NewTableCache(dbModel, nil, nil)
	require.NoError(t, err)

	var previous *tracetest.SpanRecorder
	for _, name := range []string{"first", "second"} {
		t.Run(name, func(t *testing.T) {
			recorder := test.NewSpanRecorder(t)
			require.Empty(t, recorder.Ended())
			ctx, parent := otel.Tracer("test").Start(context.Background(), "parent")
			defer parent.End()
			require.NoError(t, tableCache.Populate(ctx, ovsdb.TableUpdates{}))
			parent.End()

			spans := recorder.Ended()
			require.Len(t, spans, 2)
			require.Equal(t, "cache_Populate", spans[0].Name())
			require.Equal(t, parent.SpanContext().TraceID(), spans[0].SpanContext().TraceID())
			require.Equal(t, parent.SpanContext().SpanID(), spans[0].Parent().SpanID())
			if previous != nil {
				// Cleanup must stop the previous test from collecting new spans.
				require.Len(t, previous.Ended(), 2)
			}
			previous = recorder
		})
	}
}
