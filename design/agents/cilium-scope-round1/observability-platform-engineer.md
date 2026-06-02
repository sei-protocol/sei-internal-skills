# Round 1 input — observability-platform-engineer

I own the storage, query, and dashboard layer that Cilium telemetry lands in. I can write PromQL/LogQL all day, vendor mixins, tune Thanos sidecars and Loki ingesters. What I *can't* do credibly is tell you which Cilium-exposed signal is load-bearing for which failure mode, what counts as a healthy baseline for `cilium_bpf_map_pressure`, or whether a Hubble flow rate spike means "scrape harder" or "you have a real problem." That's the gap.

## Scenarios I'd dispatch to a Cilium expert

### Scenario 1: Which Cilium agent metrics are alert-worthy, and at what thresholds
The `cilium_*` namespace exposes ~200 metrics. I can build any alert expression, but I don't know which ones are *leading indicators* (predict failure) vs *trailing* (confirm failure already happened) vs *operational noise that mixin authors included for completeness*. Specifically: relationship between `cilium_bpf_map_pressure{map_name=...}` per-map saturation, `cilium_endpoint_state` transitions, `cilium_endpoint_regenerations_count` failure rate, `cilium_policy_import_errors_total`, and `cilium_identity` count vs the IPCache max. Which of these page, which ticket, which silently graph? Threshold guidance: is `bpf_map_pressure > 0.8` a real number or vendor folklore?

### Scenario 2: Hubble flow log volume — retention, cardinality, sampling strategy
Hubble flows ship to our Loki pipeline. Flow logs are *high-cardinality* (src/dst pod, src/dst identity, verdict, L4 port, L7 metadata) and we will swamp Loki ingesters if we naively label everything. I need an expert to tell me: (a) which Hubble flow fields are stream labels vs structured metadata vs log body content, (b) what sampling rates are safe to drop without losing security-relevant signal (defer to security on the policy, but expert frames the mechanics), (c) what Hubble's own export tuning knobs are (`hubble.eventQueueSize`, `hubble.flowBufferSize`, ringbuffer sizing). My lane is the Loki side; theirs is the producer side.

### Scenario 3: Hubble metrics aggregation — recording rules vs raw export
Hubble exposes its own Prometheus metrics (`hubble_flows_processed_total`, `hubble_drop_total`, `hubble_tcp_flags_total`, HTTP/DNS L7 metrics if enabled). Some of these are per-(src,dst,verdict) and will blow cardinality. Expert call: which Hubble metrics should be enabled cluster-wide vs scoped to specific namespaces via `hubble.metrics.enabled`, and which need pre-aggregation at the Cilium agent (via `hubble.metrics.enableOpenMetrics` + context options) vs handled by my recording rules downstream. I'll write the rules; I need to know the right shape to ask for.

### Scenario 4: Tetragon TracingPolicy event volume — capacity scaling
Tetragon is the wild card. A `TracingPolicy` that hooks a hot syscall (`sys_execve`, `tcp_connect`) can generate millions of events/sec per node. Expert call: which policies are safe to enable cluster-wide vs node-scoped, what's the relationship between policy selectivity (in-kernel filters) and exported event volume, and how does Tetragon backpressure when the export pipeline can't keep up (drops? blocks? OOMs the agent?). I size the Alloy WAL / Loki ingest, but only if I know the worst-case event rate per policy.

### Scenario 5: ConnTrack and NAT table pressure signals
`cilium_bpf_map_pressure{map_name="cilium_ct_*"}` and the NAT map pressures are *the* signals for "node about to drop connections." Expert call: at what pressure does Cilium start evicting, what's the eviction policy (LRU? GC interval?), and what's the operational response (raise `bpf-ct-global-tcp-max`, restart agent, scale node)? SRE owns the SLO threshold; I write the alert expression; the expert tells us the physics of the map so we pick a number that means something.

### Scenario 6: PR-772 dashboard vs upstream mixin reconciliation
PR sei-protocol/platform#772 added a custom Cilium dashboard. The upstream `cilium/cilium` repo ships a mixin (`install/kubernetes/cilium/files/cilium-agent/dashboards/`) plus standalone `cilium-dashboard.json`, `hubble-dashboard.json`, `tetragon-dashboard.json`. Expert call: is PR-772's dashboard answering questions the upstream mixin doesn't, or is it a parallel reinvention that should be retired in favor of vendoring upstream? I can do the vendoring mechanics; I need someone with Cilium domain context to tell me which panels are load-bearing for the harbor on-call story.

### Scenario 7: Cilium agent health probes — readiness vs metric signal lag
`cilium-agent` exposes `/healthz`, `/readyz`, and metrics. There's typically a lag between "agent is unhealthy" and "metrics reflect it" because metrics scrape interval (30s) and agent internal state aren't synchronous. Expert call: which conditions does the agent surface via metric *vs* status subresource *vs* log line only, and how do we wire alerts that catch the "unhealthy but still scraping fine" case. Relates to: `cilium_unreachable_nodes`, `cilium_unreachable_health_endpoints`, `cilium_controllers_failing`.

## Required depth per scenario

### For Scenario 1 (agent metrics taxonomy)
- **Knowledge area**: Cilium agent internals — datapath, endpoint lifecycle, identity allocation, policy compilation.
- **Depth bar**: Has operated Cilium at >100 nodes through at least one outage where these metrics were the diagnostic path. Can cite the metric name *and* explain what internal subsystem populates it.
- **Authoritative sources**: `cilium/cilium` repo `Documentation/observability/metrics.rst`, source files under `pkg/metrics/`, agent troubleshooting docs.

### For Scenario 2 (Hubble flow shape)
- **Knowledge area**: Hubble relay/server architecture, flow protobuf schema, export pipeline.
- **Depth bar**: Has tuned Hubble in a production cluster with non-trivial flow rates (>10k flows/sec). Knows the difference between Hubble Relay aggregation and per-node ringbuffers.
- **Authoritative sources**: `cilium/hubble` repo, `flow.proto`, Hubble UI architecture docs.

### For Scenario 3 (Hubble metrics aggregation)
- **Knowledge area**: Hubble metrics exporter config, OpenMetrics context options.
- **Depth bar**: Has configured `hubble.metrics.enabled` for a real workload, knows which contexts (`sourceContext`, `destinationContext`) explode cardinality.
- **Authoritative sources**: Cilium Helm chart `values.yaml` Hubble section; Hubble metrics docs.

### For Scenario 4 (Tetragon)
- **Knowledge area**: eBPF kprobe/tracepoint mechanics, Tetragon event pipeline, in-kernel filtering.
- **Depth bar**: Has written or reviewed a non-trivial `TracingPolicy` and measured its impact. Can reason about kernel-side overhead vs export-side overhead.
- **Authoritative sources**: `cilium/tetragon` repo, TracingPolicy CRD docs, Tetragon performance guidance.

### For Scenario 5 (ConnTrack)
- **Knowledge area**: Linux conntrack, Cilium's BPF replacement, GC/LRU policy.
- **Depth bar**: Has hit conntrack exhaustion in production and tuned out of it.
- **Authoritative sources**: Cilium tuning guide, `bpf-ct-global-*` config docs.

### For Scenario 6 (dashboard reconciliation)
- **Knowledge area**: Awareness of upstream Cilium mixin, Hubble dashboard, Tetragon dashboard.
- **Depth bar**: Has used or audited the upstream dashboards and can speak to gaps for harbor's specific topology.
- **Authoritative sources**: `cilium/cilium` `install/kubernetes/cilium/files/cilium-agent/dashboards/`, isovalent grafana-dashboards repo.

### For Scenario 7 (agent health)
- **Knowledge area**: Cilium agent lifecycle, controller subsystem, health subsystem.
- **Depth bar**: Has triaged a "metrics fine, agent broken" incident.
- **Authoritative sources**: agent health docs, `cilium-health` subcommand, source under `pkg/controller/` and `pkg/health/`.

## What I would NOT want this expert to be

- Not a PromQL/LogQL author — that's my lane. They tell me the metric semantics; I write the expression.
- Not a Grafana dashboarding expert — I own panel wiring, variables, layout, mixin vendoring mechanics.
- Not a Loki/Thanos operator — sizing ingesters, store-gateway, sidecars for Hubble flow volume is mine.
- Not the SLO/alert-tier owner — that's `sre-engineer`. Expert provides the *physics* (what does pressure 0.8 mean), SRE picks the page/ticket threshold, I write the rule.
- Not a NetworkPolicy author or kube-proxy-replacement decision-maker — that's `network-specialist` territory. Adjacent but distinct.
- Not a security-policy decision-maker — security defines what must be detected via Tetragon; expert tells us the mechanics of detecting it.
- Not a chart-values plumber for non-observability bits — `platform-engineer` owns HelmRelease structure; expert tells us *what* values to set, not *how* the manifest is rendered.

The seam I want: expert hands me a list of "these N metrics, these are the alert candidates, this is what each one means, these are realistic thresholds based on cluster size X." I turn that into recording rules, alert expressions, dashboard panels, and capacity numbers for the Loki/Thanos side. They don't write PromQL; I don't pretend to know what `cilium_endpoint_regenerations_count` rate-of-change implies.
