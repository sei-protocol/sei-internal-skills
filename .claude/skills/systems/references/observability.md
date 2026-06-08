# Observability-by-design

Checklist for building code you can debug from the outside at 3am. Our-own-words rules that point at the open canon — cite the authority in findings (see `sources.md`); never reproduce source text. Decides *what to instrument and where*; the SDK wiring is the opentelemetry-expert's lane, the backend is the observability-platform-engineer's.

Sections: `## checklist` · `## severity model`. Anchors: Google SRE, RED (Wilkie), USE (Gregg), Honeycomb/Majors, OpenTelemetry semconv, W3C Trace Context.

## checklist

| id | principle | rule | cue | authority |
|----|-----------|------|-----|-----------|
| OBS1 | Four Golden Signals | Emit latency, traffic, errors, and saturation for every user-facing service — they answer "is it healthy?" before you drill into why. | A service with a request counter but no latency or saturation signal. | Google SRE: Monitoring |
| OBS2 | Split success/failure latency | Measure successful and failed request latency separately; a flood of fast errors otherwise drags the average down and hides a real slowdown. | One latency metric over all outcomes; fast 500s masking slow 200s. | Google SRE: Monitoring |
| OBS3 | Histograms, not means | Record latency as a distribution (histogram/quantiles), never a mean — the average hides the tail, and the tail is where users suffer. | A gauge emitting `avg_latency_ms`; SLOs computed from a mean. | Google SRE: Monitoring; Gregg |
| OBS4 | RED per request-service | Track Rate, Errors, Duration per request-driven service, consistently, so dashboards compose and on-call is legible across unfamiliar services. | Ad-hoc per-team metric shapes; no consistent rate/error/duration triplet. | RED (Wilkie) |
| OBS5 | USE per resource | For each finite resource (CPU, memory, pool, queue) track Utilization, Saturation, Errors — catches exhaustion before it becomes user-facing. | A worker/connection pool with no saturation/queue-depth metric. | USE (Gregg) |
| OBS6 | One wide event per request | Emit a single wide, structured event per request with all context (ids, params, timings, outcome, downstream calls). Reassembling one request from scattered narrow logs is what you can't do under pressure. | Many thin log lines per request that must be grepped and stitched. | Honeycomb/Majors |
| OBS7 | Low cardinality on metric labels | Keep metric label values bounded — no user/request ids or raw URLs as labels. Unbounded label sets explode the time-series count and can take down the metrics backend. | `user_id`/`request_id`/raw-URL as a metric label. | OTel semconv; RED (Wilkie) |
| OBS8 | High cardinality on spans/events | Put per-request detail (ids, payloads, fields) on spans and wide events, where arbitrary cardinality is cheap and where you actually slice while debugging. | High-detail context dropped because "too high-cardinality for metrics," with no span/event home. | Honeycomb/Majors |
| OBS9 | Structured K-V logs | Log structured key-value records, not interpolated prose, so logs are queryable and machine-parseable. | `log.Printf("user %s failed: %v", u, err)`; free-text logs with no fields. | Honeycomb/Majors; 12-Factor: Logs |
| OBS10 | OTel semantic conventions | Name attributes/metrics with OpenTelemetry semantic conventions, not bespoke keys, so signals correlate across services and tools out of the box. | `httpStatus`/`statuscode`/`resp_code` instead of the semconv attribute; per-service spellings. | OTel semconv |
| OBS11 | Trace-context propagation | Propagate W3C `traceparent`/`tracestate` across every boundary — HTTP, gRPC, queues, jobs. A boundary that drops context breaks the trace where you need it most. | A producer/consumer or RPC hop that doesn't forward trace headers; traces ending at a service edge. | W3C Trace Context |
| OBS12 | Tag errors with a type | Classify errors with a bounded `error.type` (timeout, refused, validation…) so error metrics are diagnosable, not just countable. | A single error counter with no type/cause dimension. | OTel semconv |
| OBS13 | Instrument decisions & error paths | Instrument boundaries, branch points, and the error/retry/fallback paths — "what will I wish I'd logged at 3am?" The unhappy path is where instrumentation is thinnest and need is highest. | Rich logging on the happy path, silence on the `catch`/retry/degrade branch. | Honeycomb/Majors; Google SRE |
| OBS14 | Honest health endpoints | Health endpoints must reflect real state (see reliability REL16): readiness checks the dependencies needed to serve; an endpoint that returns 200 unconditionally lies to the orchestrator. | `/health` returning 200 without checking anything; readiness OK while a required dependency is down. | Google SRE; 12-Factor |
| OBS15 | Exemplars bridge metrics→traces | Attach exemplars (trace ids) to metrics so a latency-histogram spike links straight to an example trace. *(Advisory / cut-first — un-defer once metrics + tracing both exist and on-call pivots between them by hand.)* | A latency dashboard with no path to a representative trace. | OTel; Google SRE |
| OBS16 | Sampling as an explicit tradeoff | Treat trace sampling as a deliberate decision that keeps the signal that matters (errors, slow, rare paths). *(Advisory / cut-first — un-defer once trace volume/cost is material; until then sample everything.)* | A flat random sample dropping the rare error traces you most need. | Google SRE; OTel |

## severity model

- **correctness/safety** — OBS7 (unbounded metric cardinality can take down the backend), OBS14 (a lying health endpoint routes to broken pods or kills healthy ones). Cause incidents directly; lead with them.
- **consequence-under-load** — OBS2–OBS3 (averaged latency hides the tail you're paged on), OBS11 (a dropped trace boundary blinds you mid-incident), OBS13 (no instrumentation on the error path = undebuggable failure), OBS12 (untyped errors slow diagnosis).
- **advisory** — OBS1, OBS4–OBS6, OBS8–OBS10 (signal coverage, RED/USE, wide events, hygiene), OBS15–OBS16 (exemplars, sampling). Raise them; don't gate on them.
