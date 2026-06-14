# Signal kit: telemetry (gate mode)

The first signal kit. A **guard** (the gate-mode signal binding declared in the ledger — see SKILL.md "The guard primitive") uses this kit to watch a Prometheus/Thanos metric and gate a high-risk step on it staying healthy. This file is the kit: the read adapter, the citable query vocabulary, the decision semantics, and the **non-negotiable correctness contracts**. The spine (declare → fetch fail-closed → evaluate via cited query → act → capture provenance) is the workstream's; this kit supplies the telemetry realization of it.

Design of record: `docs/designs/telemetry-signal-kit.md`.

## When this kit applies

A workstream step is high-risk and live (a migration cutover, a deploy, a node-shape change) and a metric signal tells you whether it's healthy. The guard is the tireless second watcher; the human still owns the irreversible go/no-go (the `design-approval` / one-way-door checkpoints). Gate mode is binary and defensive: `healthy → proceed`, `trip → halt + route to rollback`.

## Read adapter — the Grafana datasource-proxy call (NOT grafana-mcp)

- **Path:** `GET {GRAFANA_URL}/api/datasources/proxy/uid/prometheus-prod/api/v1/query[_range]` with `Authorization: Bearer {token}`. This is the established `validate-release/scripts/query-grafana.py` pattern — reuse its `query_range()` (proxy URL construction, bearer auth, `status: success` check, empty/NaN handling) and `check-grafana.sh` (token preflight) directly.
- **Why not `grafana/mcp-grafana`:** the MCP server **discards the Thanos `warnings` array** and exposes no `partial_response` param (verified in its `main`), so it cannot satisfy the partial-response-safe contract below. The raw proxy returns the full Prometheus/Thanos envelope — `warnings` intact, `partial_response=false` honorable. (An agent-native, warnings-preserving MCP layer is the future path once the upstream fix lands — see the design's Auth section. Until then: the proxy call.)
- **Auth:** a Grafana **service account, `Viewer` role, SA token** (Grafana's standard programmatic mechanism; read-only is structural — Viewer + Thanos query API write-free + admin/receive/ruler disabled). For development, a manually-minted token; production provisioning/rotation is the platform sibling (Issue B).

## Citable query vocabulary — cite rules, do not re-derive

The guard **names an existing recording rule or alert expr**; it does not re-derive histograms or burn rates inline (in-context arithmetic is the #1 failure mode — all math lives in PromQL/recording rules; the agent reads scalars).

- `chaos_suite:block_time_p95:rate2m`, `chaos_suite:tps:rate1m`, `chaos_suite:block_height_delta:rate2m` — chain-health rates.
- `slo_*_{5m..30d}` ladders — the SLO "fraction-of-window-healthy" series (`sum_over_time((expr > bool THRESH)[w:]) / count` already exists).
- SRE **multi-window multi-burn-rate (MWMBR)** alert exprs — the "is it degrading vs SLO" math (continuous mode only; see contracts).

**Window/eval caveats (cite the expr, not the name):**
- `slo_*_3h` rules actually use a `[2h:]` window despite the `3h` name — cite the expression window.
- The `_3h` tier and recording rules are evaluated by **Prometheus at ~30s**; only the `≥3d -longwindow` series evaluate in **ThanosRuler at 5m**. This sets the per-source freshness budget below.

## Decision semantics — mirror Argo Rollouts / Flagger

Declare the gate with the `AnalysisTemplate`/Flagger vocabulary: `interval` (poll cadence), `count` (windows observed), `failureLimit` / `successCondition` / `thresholdRange`. Two modes of *when* a guard watches (declared in the ledger entry):

- **`soak`** — poll for N minutes after the action, vs the gate-start baseline. The operative math is **N-consecutive-breach** (`count`/`failureLimit`) against the baseline. **MWMBR does NOT apply** — its slow window can't accumulate inside a 10–30 min soak; a soak guard must not claim slow-burn coverage.
- **`continuous`** — watch across a whole phase. MWMBR is the continuous-mode degradation math.
- **`pre-step`** — a single healthy check before the action.

## Non-negotiable correctness contracts

These are what separate a real guard from one that PASSes on the two most common cutover failure modes. The first two are load-bearing — **without them the guard is worse than no guard** (it manufactures false confidence).

1. **Partial-response-safe — inspect `warnings` as the PRIMARY mechanism.** Thanos has partial-response ON by deliberate design; the cited rules are fleet-aggregated (`sum by (chain_id)`, `topk`), so on a cross-cluster blip a `sum` silently returns a partial total with HTTP 200 + a non-empty `warnings` array — and **freshness does not catch this** (the data that returned is current). **Non-empty `warnings` ⇒ inconclusive ⇒ abort.** Also force `?partial_response=false`. (Both are why the adapter is the proxy call, not grafana-mcp.)
2. **No-traffic ≠ healthy — volume floor + liveness co-condition.** A ratio SLI (`rate(ok)/rate(total)`) reads a clean `1.0` on a near-zero denominator — exactly the cutover failure where traffic isn't draining to the new shape. Every ratio guard co-asserts a **minimum-event-volume denominator** (e.g. `sum(rate(tx_count[1m])) >= <min>`) AND, for any consensus-touching cutover, a **liveness floor** (`chaos_suite:block_height_delta:rate2m > 0`) — separate cited queries that must independently pass. Below the floor: `inconclusive`, never PASS.
3. **Target-coverage check.** Co-assert `up{…} == 1` (or expected-series cardinality) for the guard's targets — a pod that crashed mid-soak stops emitting and the last value carries forward inside Prometheus's 5m staleness window, reading "fresh enough." Coverage ≠ "the number is good."
4. **Empty rule result = inconclusive.** `topk` / `histogram_quantile` / join-pattern rules return *no series* when inputs are absent. Empty ⇒ inconclusive ⇒ abort, identical to stale. Never read absence as "not breached = healthy."
5. **Per-source freshness.** Budget by who evaluates the series: `~2×30s` for recording-rule + `_3h`-tier series (Prometheus); `~2×5m` for `≥3d -longwindow` series (ThanosRuler). Provenance records which store answered.
6. **Two-window / N-consecutive-breach, never a single threshold** (see modes above).
7. **Baseline captured at gate-start for cutovers.** `offset 1w` is **not permitted for a cutover** (topology changes; last week is a different fleet — and risks the prod/pacific-1 co-tenancy trap). Gate-start snapshot is mandatory for topology changes; `offset 1w` only for steady-state deploys. For soaks > ~15 min, compare rate/ratio-normalized (organic load ramps else read as degradation) or re-baseline.
8. **Effective detection latency = input-window + rule-interval.** A guard citing `…:rate2m` is ~2.5 min blind to a sharp drop; provenance states this; `continuous` fast-failure guards cite shorter-window rules.

**Fail-closed is the spine rule these all serve:** stale data, unreachable endpoint, auth/query error, empty read, or non-empty `warnings` ⇒ "cannot confirm healthy" = abort, **never** PASS. Same discipline as the workstream checkpoint and gov-ops.

## Provenance (every reading)

Record, per reading: the exact query (recording-rule name + the expr it expands to), the window, the store that answered, the `partial_response` setting, the `warnings` (empty or not), and the verdict (`pass` / `trip` / `inconclusive`). This is the re-runnable audit trail — and in measure mode (later) it *is* the deliverable.

## Surface vs act (gate mode, MVP)

`on_trip` is **surface-and-wait** — halt before the next step, surface the trip + the cited evidence, route to a pre-declared rollback checkpoint; the human owns the call. Auto-abort is deferred (design OQ5): only ever a pre-declared *reversible, idempotent* rollback, at a higher confidence bar than surface, never on a one-way-door step.

## What this kit does not do

- It is the measurement instrument, not the optimizer (relevant when measure mode lands).
- No generic arbitrary-PromQL guard — domain guards cite named rules.
- No Alertmanager grouped-alert read path (per-rule firing state covers MVP).
- No in-cluster direct-to-Thanos / tenancy proxy (deferred).
