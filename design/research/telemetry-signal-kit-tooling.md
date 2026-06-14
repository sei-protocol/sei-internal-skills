# Research: telemetry signal-kit — build-vs-reuse tooling + metric federation

**Status:** Draft
**Date:** 2026-06-14
**Issue:** (telemetry signal-kit workstream — sibling to the PLT-500 framework family; no Linear issue filed yet)
**Authors:** bdchatham, Claude (Coral) — sourced via the `/research` method (4 blind sweeps + adversarial verify)

## Question

Build-vs-reuse for the **telemetry (PromQL/Thanos) signal-kit, gate mode** — the "agent watches a metric for N minutes and gates a risky procedure on it staying healthy, reading provenance-bearing (re-runnable) queries" capability (the Amazon bake-against-alarm pattern, generalized for agents).

- **Decision it informs:** what OSS we stand on for (1) agent→metric querying, (2) the metric math/significance layer, (3) the federation/auth for agent read access — so we reuse, not reinvent.
- **Falsifiable claims sought (verdicts below):** "a maintained MCP server exists an agent can use read-only"; "PromQL+Thanos cover most gate math, so the shared math layer is thin"; "Thanos Query gives the fleet-global read view"; "headless/cron agent runs can auth to it."
- **Boundary:** in = telemetry gate-kit tooling + math + federation. out = perf/eBPF kit, coordinate/indexed-events kit, the full signal-kit design, building anything.

## Sweep coverage

Four angles, blind to each other: (A) our Prometheus/Thanos/Grafana stack [observability-platform-engineer, against live `clusters/prod/monitoring/` config]; (B) OSS MCP servers + clients [web, by-source/entity]; (C) metric-data federation/auth [platform-engineer, against live infra]; (D) math/significance OSS + failure modes [web, by-counter-thesis]. Maintenance/recency adversarially verified via repo + GitHub API.

## Findings (tagged)

**Query/MCP layer**
- **[verified]** `grafana/mcp-grafana` — **official Grafana Labs**, Apache-2.0, **v0.15.2 (2026-06-04)**, ~3.1k★. One-stop: PromQL instant+range, metric/label discovery, **alerting with per-rule firing `state`**. Thanos behind a Grafana datasource is transparent. *This is the read adapter to build on.* Source: github.com/grafana/mcp-grafana.
- **[verified]** Fallback (no Grafana): `pab1it0/prometheus-mcp-server` — MIT, v1.6.1 (2026-05-01), queries only, **no alert reads**. Library route: prometheus `client_golang` / `prometheus-api-client-python`.
- **[verified]** No maintained **Thanos-specific** MCP exists (reached via the Prometheus-compatible API through Grafana/Prometheus MCP). `DazWilkin/prometheus-mcp-server` is **archived** — reference only.
- **[unverified]** Whether grafana-mcp returns a grouped *active-alerts list* vs only per-rule `state`; and no tool exposes a dedicated **Alertmanager** grouped/silenced read — if the gate watches an Alertmanager-grouped aggregate alarm, that read path is a gap to add.

**Stack-native math (claim: "math layer is thin" — verdict: TRUE for threshold/rate/quantile/window-fraction; FALSE for boundary-delta/significance)**
- **[verified]** PromQL/Thanos natively cover `rate`/`irate`, `histogram_quantile` (p99), `increase`/`delta`, `predict_linear`, `deriv`, `*_over_time`. The **`sum_over_time((expr > bool THRESH)[w:]) / count` "fraction-of-window-healthy" primitive already exists in our SLO recording rules** — exactly the "stayed healthy for N minutes" gate shape. Source: `recording-validator-slos.yaml`, `recording-chain-slos.yaml`.
- **[verified]** NOT native, genuinely agent-side: **baseline-relative delta across a deploy boundary** (pin the cutover time → two range queries → diff; PromQL `offset` only helps for a fixed clock offset), significance testing, cross-metric correlation, contiguous-streak length.
- **[verified]** Reuse template already exists: `chaos_suite:block_time_p95:rate2m`, `:tps:rate1m`, `:block_height_delta:rate2m`, the `slo_*_{5m..30d}` ladders, and the **chaos-suite validation agent + `compute-stats.py`** — ~60–70% of the guard's poll loop. A guard should **cite these rules** (cheap instant queries), not re-derive histograms inline.

**Math/decision prior art (claim C: build on established OSS — verdict: TRUE)**
- **[verified]** **SRE multi-window multi-burn-rate (MWMBR)** is the "is it degrading vs SLO" math, expressed as **PromQL recording rules + alert exprs** — config, not code. Source: SRE Workbook ch.5.
- **[verified]** **Argo Rollouts `AnalysisTemplate`** (Apache-2.0, **v1.9.0 2026-03-20**, strong cadence) and **Flagger** (Apache-2.0, v1.43.0 2026-04-21, CNCF, lower cadence) are "bake-against-alarm as a CRD" — `interval`/`count`/`failureLimit`/`successCondition` / `thresholdRange`/`maxFailedChecks`. **Mirror this schema for the guard.**
- **[verified — do NOT depend]** Kayenta/Spinnaker statistical ACA (baseline-vs-canary distribution test) — **standalone repo archived**; borrow the distribution-comparison *idea* only. AICoE PAD (ML anomaly) — **dead since 2023; reject.**

**Federation / auth (claims: fleet-global view + headless auth — verdicts: TRUE / TRUE-via-Grafana-SA)**
- **[verified]** **Thanos Query Frontend** (`thanos-query-frontend.monitoring.svc:9090`) is the single fleet-global, Prometheus-compatible read endpoint (prod regions + harbor over VPC peering), auto-selecting resolution + dedup — the same datasource Grafana trusts. Not per-cluster, not remote-read.
- **[verified — highest-priority correctness gotcha]** **Partial-response is ENABLED on the deployed Thanos Query** (contradicts the design doc + alert text). On a cross-cluster blip it returns HTTP 200 + partial data + a `warnings` array. A naive guard could **gate-PASS on a half-fleet read.** The guard MUST inspect `warnings` (treat non-empty = inconclusive) or force `?partial_response=false` per query.
- **[verified]** **Headless auth crux:** Grafana's gate is Google **OAuth = interactive-only**, so cron/headless agents can't "log in." The non-interactive path is a **Grafana service-account Viewer token** → datasource-proxy, provisioned via a `GrafanaServiceAccount` CR through the **already-deployed Grafana Operator** (token → AWS Secrets Manager → CSI). Read-only is structural (Viewer role; Thanos query API is write-free; receive/ruler disabled).
- **[verified]** **External agents have no private path to Thanos today** (ClusterIP only); they must go through Grafana. **One-way door flagged:** publicly exposing Thanos's query API contradicts the deliberate "no public Thanos gRPC" stance — route external through Grafana for MVP.
- **[unverified — confirm before build]** Grafana **edition** (per-datasource SA scoping is an Enterprise/RBAC feature; OSS floor = Viewer across all datasources); `GrafanaServiceAccount` CR support in the deployed operator version; **where headless/cron agents actually run** (in-cluster vs external — the read-path branches on it); release-manager's actual query mechanics (source not in local worktrees); Thanos `--query.timeout`/`--query.max-concurrent` (chart defaults, confirm via `/flags`).

## Completeness assessment

Covered: query/MCP, native math, decision prior art, federation, auth, failure modes. **Gaps before a build decision is final** (all logged above as `[unverified]`): the Grafana edition + operator CR support + where cron agents run (these gate the federation design); the Alertmanager grouped-alert read path (gate-on-aggregate-alarm); pulling the release-manager/chaos-suite agent *source* (it likely already implements 60–70% of the poll loop). None changes the headline reuse picture; all are confirm-before-build items, not blockers.

## Synthesis & recommendation (grounded only in verified findings)

**Reuse, don't build, on every layer:**
1. **Read adapter:** `grafana/mcp-grafana` (official, current, query + firing-alert state; Thanos-transparent). No custom Thanos MCP.
2. **Read path + federation:** Thanos Query **Frontend** via **Grafana datasource-proxy**, authed by a **Grafana service-account Viewer token** (the non-interactive answer to the headless crux), provisioned through the existing Grafana Operator. This *is* the metric-federation infra callout, answered. In-cluster direct-to-Thanos + `prom-label-proxy` tenancy is a deferred optimization.
3. **Math layer is thin — cite, don't compute:** PromQL + the existing `chaos_suite:*`/`slo_*` recording rules + SRE MWMBR cover the gate math. The agent reads pre-computed scalars/booleans; the only agent-side math (deploy-boundary baseline diff) is pushed into a pinned two-window query where possible. The hypothesized "math MCP" is largely **unnecessary**.
4. **Gate decision schema:** mirror **Argo Rollouts AnalysisTemplate / Flagger** (`interval`/`count`/`failureLimit`/`successCondition`/`thresholdRange`) + **MWMBR** two-window logic. Reuse the **chaos-suite validation agent's** poll loop as the implementation template.

**Non-negotiable correctness contracts the guard kit MUST encode (from the verified gaps):**
- **Partial-response-safe:** inspect `warnings` / force `partial_response=false` — never PASS on a half-fleet read.
- **Fail-closed:** stale data, unreachable endpoint, auth/query error → "cannot confirm healthy" = degraded/abort, **never** PASS (the same spine as the workstream checkpoint + gov-ops).
- **Freshness + min-samples precondition** before any verdict; **two-window / N-consecutive-breach**, never a single threshold; **baseline captured at gate-start** (snapshot or `offset 1w`), not an absolute; **all math in PromQL/recording rules**, agent reads scalars; **cite the re-runnable query** with every reading (provenance = the research/lineage discipline).

This makes the telemetry kit the "Go-pack" exemplar that proves the signal-kit spine (declare → fail-closed fetch → evaluate-via-cited-query → gate/surface → provenance) before the coordinate (indexed-events) and measure (perf) kits.

## References

- Sweeps (this session): stack [observability-platform-engineer], MCP/clients [web], federation/auth [platform-engineer], math/gaps [web].
- grafana/mcp-grafana; pab1it0/prometheus-mcp-server; prometheus client_golang; SRE Workbook ch.5 (MWMBR); Argo Rollouts AnalysisTemplate; Flagger.
- Live config: `clusters/prod/monitoring/{thanos-query,prometheus-operator,recording-rules-chaos-suite,grafana,grafana-operator}.yaml`, `recording-{validator,chain}-slos.yaml`; `docs/designs/harbor-prod-thanos-sidecar.md`.
- Related: the PLT-500 `workstream`/checkpoint primitive (a guard is a non-human gate); the `/research` method (this doc's discipline).
