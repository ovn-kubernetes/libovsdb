package cache

import (
	"context"
	"testing"

	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/ovn-kubernetes/libovsdb/test"
	"github.com/ovn-kubernetes/libovsdb/updates"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestCacheMetrics(t *testing.T) {
	ctx := context.Background()
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() { otel.SetMeterProvider(previous); _ = provider.Shutdown(ctx) })
	dbModel, err := test.GetModel()
	require.NoError(t, err)
	cache, err := NewTableCache(dbModel, nil, nil)
	require.NoError(t, err)
	registration, err := cache.RegisterMetrics()
	require.NoError(t, err)
	defer func() { _ = registration.Unregister() }()
	for _, count := range []int64{0, 1, 0} {
		if count == 1 {
			require.NoError(t, cache.Table("Bridge").Create("bridge", &test.BridgeType{Name: "test"}, false))
		} else if cache.Table("Bridge").HasRow("bridge") {
			require.NoError(t, cache.Table("Bridge").Delete("bridge"))
		}
		var data metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(ctx, &data))
		found := false
		for _, scope := range data.ScopeMetrics {
			for _, measurement := range scope.Metrics {
				if measurement.Name != "libovsdb.cache.rows" {
					continue
				}
				gauge, ok := measurement.Data.(metricdata.Gauge[int64])
				require.True(t, ok)
				for _, point := range gauge.DataPoints {
					name, _ := point.Attributes.Value(attribute.Key("table"))
					if name.AsString() == "Bridge" {
						found = true
						require.Equal(t, count, point.Value)
					}
				}
			}
		}
		require.True(t, found)
	}
	require.NoError(t, registration.Unregister())
	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &data))
	for _, scope := range data.ScopeMetrics {
		require.Empty(t, scope.Metrics)
	}
}

func TestCacheTracePropagation(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() { otel.SetTracerProvider(previous); _ = provider.Shutdown(context.Background()) })
	ctx, parent := provider.Tracer("test").Start(context.Background(), "parent")
	defer parent.End()
	dbModel, err := test.GetModel()
	require.NoError(t, err)
	for _, tt := range []struct {
		name  string
		call  func(context.Context, *TableCache) error
		spans []string
	}{
		{"Update", func(ctx context.Context, cache *TableCache) error {
			return cache.Update(ctx, nil, ovsdb.TableUpdates{"Bridge": {}})
		}, []string{"cache_Populate", "cache_Update"}},
		{"Update2", func(ctx context.Context, cache *TableCache) error {
			return cache.Update2(ctx, nil, ovsdb.TableUpdates2{"Bridge": {}})
		}, []string{"cache_Populate2", "cache_Update2"}},
		{"Populate", func(ctx context.Context, cache *TableCache) error {
			return cache.Populate(ctx, ovsdb.TableUpdates{})
		}, []string{"cache_Populate"}},
		{"Populate2", func(ctx context.Context, cache *TableCache) error {
			return cache.Populate2(ctx, ovsdb.TableUpdates2{})
		}, []string{"cache_Populate2"}},
		{"ApplyCacheUpdate", func(ctx context.Context, cache *TableCache) error {
			return cache.ApplyCacheUpdate(ctx, updates.ModelUpdates{})
		}, []string{"cache_ApplyCacheUpdate"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := NewTableCache(dbModel, nil, nil)
			require.NoError(t, err)
			before := len(recorder.Ended())
			require.NoError(t, tt.call(ctx, cache))
			spans := recorder.Ended()[before:]
			require.Len(t, spans, len(tt.spans))
			for i, span := range spans {
				require.Equal(t, tt.spans[i], span.Name())
				require.Equal(t, parent.SpanContext().TraceID(), span.SpanContext().TraceID())
				expectedParent := parent.SpanContext().SpanID()
				if i+1 < len(spans) {
					expectedParent = spans[i+1].SpanContext().SpanID()
				}
				require.Equal(t, expectedParent, span.Parent().SpanID())
			}
		})
	}
}
