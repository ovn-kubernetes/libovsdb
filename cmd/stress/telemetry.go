package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func setupTelemetry(ctx context.Context, endpoint, metricsAddress string) (func() error, error) {
	if endpoint == "" {
		return func() error { return nil }, nil
	}
	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	provider := trace.NewTracerProvider(trace.WithBatcher(exporter),
		trace.WithResource(resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName("libovsdb-stress"))))
	registry := prometheus.NewRegistry()
	reader, err := otelprom.New(otelprom.WithRegisterer(registry))
	if err != nil {
		_ = provider.Shutdown(ctx)
		return nil, err
	}
	meter := metric.NewMeterProvider(metric.WithReader(reader))
	listener, err := net.Listen("tcp", metricsAddress)
	if err != nil {
		_ = provider.Shutdown(ctx)
		_ = meter.Shutdown(ctx)
		return nil, err
	}
	otel.SetTracerProvider(provider)
	otel.SetMeterProvider(meter)
	server := &http.Server{Handler: promhttp.HandlerFor(registry, promhttp.HandlerOpts{}), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(listener) }()
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return errors.Join(server.Shutdown(ctx), meter.Shutdown(ctx), provider.Shutdown(ctx))
	}, nil
}
