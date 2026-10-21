package cache

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// RegisterMetrics observes the number of rows in each cached table. Callers must
// unregister the returned registration when the cache is no longer needed.
// Configure the global MeterProvider before calling RegisterMetrics. Attributes
// can distinguish multiple caches reporting tables with the same names.
func (t *TableCache) RegisterMetrics(attributes ...attribute.KeyValue) (metric.Registration, error) {
	attributes = append([]attribute.KeyValue(nil), attributes...)
	meter := otel.Meter("github.com/ovn-kubernetes/libovsdb/cache")
	gauge, err := meter.Int64ObservableGauge("libovsdb.cache.rows",
		metric.WithDescription("Number of rows in a cached table"), metric.WithUnit("{row}"))
	if err != nil {
		return nil, err
	}
	return meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		for _, name := range t.Tables() {
			if table := t.Table(name); table != nil {
				labels := append(append([]attribute.KeyValue(nil), attributes...), attribute.String("table", name))
				observer.ObserveInt64(gauge, int64(table.Len()), metric.WithAttributes(labels...))
			}
		}
		return nil
	}, gauge)
}
