---
name: opentelemetry
description: >-
  OpenTelemetry instrumentation expert. Use when instrumenting code with
  traces/metrics/logs, reviewing OTel SDK usage, designing telemetry pipelines,
  evaluating metric dimensionality, or setting up Prometheus/OTLP exporters.
user-invocable: false
---

# OpenTelemetry

## Meter & Metric Design

### One Meter Per Package

Each Go package gets a single `Meter`. Name it after the package, not the subsystem:

```go
var meter = otel.Meter("evmrpc")  // not "evmrpc_rpc" or "evmrpc_filter"
```

Keep all metric instrument declarations in a single `metrics.go` file per package. Initialize as package-level vars — they are safe for concurrent use.

### Naming

- Name the var after what it measures with a type suffix: `requestLatency` (histogram), `wsConnectionCount` (counter)
- Do not repeat the package name in the variable: `requestLatency`, not `rpcRequestLatency`
- Prometheus metric names use `snake_case` with unit suffix: `evmrpc_request_latency_seconds`, `evmrpc_ws_connection_count`
- Remove redundant words: `evmrpc_requests_total` not `evmrpc_rpc_requests_total`

### Histograms Are Your Most Powerful Instrument

**A histogram gives you count, sum, and distribution from a single instrument.** This is the key insight for getting the most out of OTel metrics.

When you record a latency histogram, the Prometheus exporter automatically produces:
- `_bucket` — the distribution (p50, p95, p99 via `histogram_quantile`)
- `_sum` — total accumulated duration (useful for calculating average: sum/count)
- `_count` — total number of observations (equivalent to a separate counter)

**This means a latency histogram replaces a separate request counter.** If you always record latency for every request, `_count` gives you the request count for free. Do not create both a counter and a histogram for the same operation — the histogram's `_count` is your counter.

```go
// GOOD: one histogram gives you latency AND count
requestLatency = must(meter.Float64Histogram(
    "evmrpc_request_latency_seconds",
    metric.WithDescription("RPC request latency"),
    metric.WithUnit("s"),
))

// BAD: redundant counter — the histogram _count already tracks this
requestCount = must(meter.Int64Counter(
    "evmrpc_requests_total",  // DELETE THIS — use requestLatency_count instead
))
```

When to still use a standalone counter:
- Events that have no meaningful duration (WebSocket connects, errors emitted)
- When you need a counter without the overhead of histogram buckets

### Units

**Always use `seconds` for duration/latency.** Go's `time.Duration.Seconds()` returns `float64` without precision loss. Never use milliseconds — Prometheus and Grafana expect seconds for duration metrics, and `histogram_quantile` works on the raw bucket values.

```go
// GOOD
metric.WithUnit("s")
requestLatency.Record(ctx, time.Since(start).Seconds(), ...)

// BAD
metric.WithUnit("ms")
requestLatency.Record(ctx, float64(time.Since(start).Nanoseconds()) / float64(time.Millisecond), ...)
```

### Attribute Keys

Define repeated attribute keys as package-level vars to avoid string duplication and typos:

```go
var (
    attrEndpoint   = attribute.Key("endpoint")
    attrConnection = attribute.Key("connection")
    attrErrorType  = attribute.Key("error.type")
)
```

For errors, tag by error type for a fixed set of well-known errors rather than a boolean success/fail. This gives you error rate by category:

```go
// GOOD: tells you WHAT failed
attrErrorType.String("timeout")
attrErrorType.String("invalid_params")

// LESS USEFUL: only tells you something failed
attribute.Bool("success", false)
```

### Dimensionality

Labels/attributes must be low cardinality:
- **Good**: endpoint name (`eth_getLogs`), connection type (`http`, `ws`), error type (`timeout`)
- **Bad**: user ID, request ID, full URL path, transaction hash

Every unique label combination creates a new time series. 10 endpoints × 3 connection types × 5 error types = 150 series. Unbounded labels cause cardinality explosion.

### Abstraction Boundaries

Instrument at the correct layer. Different layers need different metrics:

- **Network boundary** (RPC handler): request count, latency, error rate. One measurement per external request. This is where histograms with endpoint + connection labels belong.
- **Internal processing** (worker pools, batch jobs): only when diagnosing specific bottlenecks. Use separate, clearly-named metrics. Never overload an RPC-level metric with internal batch counts.

Ask: "who calls this code?" If multiple external APIs call the same internal function, the internal function's metrics should not carry API-level labels — they should describe what the function does, not who called it.

## Context Propagation

### Always Thread Request Context

Every incoming request gets a context at the API layer. Thread it down the entire call stack. Never create `context.Background()` deep in request-handling code — it breaks trace propagation.

```go
// GOOD: context flows from handler to metric recording
func (a *FilterAPI) GetLogs(ctx context.Context, crit FilterCriteria) ([]*Log, error) {
    defer recordLatency(ctx, "eth_getLogs", time.Now())
    return a.logFetcher.GetLogsByFilters(ctx, crit)
}

// BAD: detached context loses the trace
func recordSomething() {
    ctx := context.Background()  // SMELL: lost the request context
    meter.Record(ctx, 1)
}
```

Once the OTel SDK and framework integration are wired up, spans and trace data attach to the request context automatically. The two prerequisites:
1. OTel SDK + framework integration configured at the boundary (middleware)
2. Every handler passes its request context down the stack

### Testing Contexts

In tests, use `t.Context()` (Go 1.21+) instead of `context.Background()`. This ties the context lifetime to the test and avoids leaked goroutines.

## Span Naming

Use short, low-cardinality names from semantic conventions:

- HTTP: `GET /users/{id}` (method + route template, never full URL path)
- DB: `SELECT users` (operation + table)
- RPC: `grpc.health.v1.Health/Check`
- Messaging: `orders process` (destination + operation)

## Attributes

Use [semantic convention](https://opentelemetry.io/docs/specs/semconv/) attribute names — never invent your own:

- HTTP: `http.request.method`, `url.full`, `http.response.status_code`, `server.address`
- DB: `db.system`, `db.statement`, `db.namespace`
- General: `error.type`, `service.name`

Import from the versioned semconv package (e.g., `go.opentelemetry.io/otel/semconv/v1.x`).

## Span Status

Only set status on errors: `span.SetStatus(codes.Error, err.Error())`. Never set `Ok` — unset is the success state. Call `RecordError` alongside `SetStatus` (it only adds a span event, doesn't change status).

For HTTP: 4xx is `Error` on client spans, `Unset` on server spans. 5xx is always `Error`.

## OTel Pipeline Architecture

```
Application code → OTel SDK (Meter/Tracer API)
                       ↓
                  MeterProvider / TracerProvider (runtime binding)
                       ↓
                  Exporter (Prometheus, OTLP, Jaeger, etc.)
                       ↓
                  Backend (Prometheus, Grafana, Datadog, etc.)
```

Application code is vendor-neutral. The runtime binding decides where signals go. This separation means switching from Prometheus to OTLP push requires zero application code changes.

### Prometheus Exporter

OTel metrics map directly to Prometheus types:
- `Int64Counter` / `Float64Counter` → Prometheus `counter`
- `Float64Histogram` → Prometheus `histogram` (with `_bucket`, `_sum`, `_count`)
- `Int64UpDownCounter` / `Float64Gauge` → Prometheus `gauge`

The exporter exposes a `/metrics` endpoint. Metric names follow Prometheus conventions automatically.

### Zero-Code Instrumentation (eBPF/OBI)

OBI uses eBPF probes to capture network-level telemetry without code changes:
- Automatically generates RED metrics (Rate, Errors, Duration) for HTTP/gRPC/DB
- Cannot capture application-level semantics (business logic, internal state)
- Complementary to SDK instrumentation: OBI for broad baseline, SDK for depth
- Requires Linux kernel 5.8+, supports Go 1.17+

## Testing

Use in-memory exporters with `SimpleSpanProcessor` (not batch — batch introduces timing). Assert on exported spans' names, attributes, and status. No mocking needed.

## Instrumentation Boundaries

Instrument network calls, I/O, and queue operations — not every function. Use span events (`AddEvent`) instead of separate log statements for events within a traced operation.

## Review Checklist

When reviewing OTel instrumentation:
- [ ] One `Meter` per package, instruments in a single `metrics.go` file
- [ ] Histogram used for request latency (not counter + separate timing)
- [ ] No redundant counters alongside histograms measuring the same thing
- [ ] Duration in seconds (`time.Since(start).Seconds()`), never milliseconds
- [ ] Attribute keys use semconv names; repeated keys defined as vars
- [ ] Error tagging by type, not just boolean success/fail
- [ ] No high-cardinality attributes on metrics
- [ ] Instrumentation at the correct abstraction boundary
- [ ] Request context threaded through (no `context.Background()` in request paths)
- [ ] Tests use `t.Context()`, in-memory exporters, `SimpleSpanProcessor`
- [ ] Metric naming: `package_signal_unit` pattern, no redundant prefixes
