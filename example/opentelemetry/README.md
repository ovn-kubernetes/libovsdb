# OpenTelemetry

libovsdb uses the global OpenTelemetry tracer and meter providers. Without an
application-configured provider, instrumentation is a no-op. The library does
not install exporters or start an HTTP server.

Tracing covers client connection, RPC, cache reads and operation builders;
server RPC handlers; database access; all transaction operations (including
wait, commit, abort, comment and assert); reference processing; and cache
population and application of model updates.

Context-aware methods propagate their span context to nested operations. RPC
notifications, server handlers and operation builders without a context start
new traces. OVSDB JSON-RPC does not carry W3C trace context, so client and server
traces are not automatically joined across the wire.

## API changes

Database and transaction interfaces now take `context.Context` as the first
argument. Database implementations and reference providers must accept it too.
`updates.ProcessReferences` also takes a context. Pass the current request
context (or `context.Background()` where no request context exists).

Cache methods `Update`, `Update2`, `Populate`, `Populate2` and `ApplyCacheUpdate`
also require `context.Context` as the first argument. `ovsdb.NotificationHandler`
implementations must accept a context and return an error from `Update` and
`Update2`. Pass the current context to preserve parent spans; there are no
context-free compatibility wrappers.

Call `cache.RegisterMetrics()` after configuring the global meter provider. It
returns a registration and an error. Unregister it when disposing of the cache:

```go
registration, err := ovs.Cache().RegisterMetrics()
if err != nil {
    return err
}
defer registration.Unregister()
```

`libovsdb.cache.rows` is an observable gauge with a `table` attribute. It reports
current row counts, including empty tables, without per-table instrument names.
Applications with multiple caches should pass distinguishing attributes, e.g.
`RegisterMetrics(attribute.String("client", clientID))`, to avoid reporting
multiple gauge values for the same table attribute set.

## Local demo

The demo pulls the published OVS image and builds the stress tool from the root
`Containerfile`. Run from this directory:

```sh
docker compose up --build
```

The stress tool exports OTLP traces to the collector and serves Prometheus
metrics on port 2222. View traces at http://localhost:16686 (Jaeger) or
http://localhost:9411 (Zipkin), and metrics at http://localhost:9090 (Prometheus).
The `-otlp` flag enables telemetry; `-metrics-address` overrides the metrics
listener address. The demo uses plaintext OTLP and is for local development only.
Shutdown flushes telemetry with a bounded timeout.
