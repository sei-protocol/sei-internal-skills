# Signal kit: telemetry (gate mode)

The first signal kit. A **guard** (the gate-mode signal binding declared in the ledger — see SKILL.md "The guard primitive") uses this kit to watch a Prometheus/Thanos metric and gate a high-risk step on it staying healthy. This file is the kit: the read adapter, the citable query vocabulary, the decision semantics, and the **non-negotiable correctness contracts**. The spine (declare → fetch fail-closed → evaluate via cited query → act → capture provenance) is the workstream's; this kit supplies the telemetry realization of it.

Design of record: `docs/designs/telemetry-signal-kit.md`.

## When this kit applies

A workstream step is high-risk and live (a migration cutover, a deploy, a node-shape change) and a metric signal tells you whether it's healthy. The guard is the tireless second watcher; the human still owns the irreversible go/no-go (the `design-approval` / one-way-door checkpoints). Gate mode is binary and defensive: `healthy → proceed`, `trip → halt + route to rollback`.

## Read adapter — direct Thanos via operator cluster access (near-term MVP)

- **Path (MVP):** query **Thanos Query Frontend directly**, in-cluster, against the raw Prometheus API `GET /api/v1/query[_range]` at `thanos-query-frontend.monitoring.svc:9090`. **Auth is the operator's cluster access:** the guard runs **operator-dispatched**, so the operator's `sei` AWS profile already authorizes the EKS API (EKS trusts AWS IAM → kubectl). **No Grafana, no bearer token, no Secrets Manager, no JWT, no infra.** (Precedent caveat: validate-release's `query-grafana.py` establishes the *operator-dispatched* pattern but reads *public Grafana*, not the cluster — so the in-cluster read path here is new; verify the RBAC predicate below, don't assume it.)
- **Use the API-server proxy, one request per poll — NOT a long-lived `port-forward`.** A held `port-forward` is a single SPDY stream that drops across a 10–30 min soak (API-server rollout, LB idle-timeout, QF pod reschedule). Issue each poll as an independent request via the API-server proxy: `kubectl get --raw "/api/v1/namespaces/monitoring/services/thanos-query-frontend:9090/proxy/api/v1/query?<…>" --context <ctx>`. (A per-poll *re-established* `port-forward` is an acceptable fallback; a single tunnel held across the soak is not.) **Any tunnel/proxy/connection failure ⇒ `inconclusive` ⇒ abort — never a skipped poll, never assume the unobserved window was healthy.**
- **Pin `--context <declared-target-fleet>` on EVERY read, and re-verify it each poll.** The ambient `kubectl` context is NOT trusted — querying the wrong cluster's fleet-aggregated `sum by (chain_id)` reads plausibly healthy and PASSes falsely, and on the `prod` context can sweep in **pacific-1 mainnet** series (the co-tenancy trap, contract 7). Derive the context from the gate's declared target fleet; **`inconclusive` if the context is unset, ambiguous, or ≠ the declared target**; re-check it each poll and halt on drift.
- **RBAC is a fail-closed precondition, not an assumption.** Reading `monitoring` needs `create pods/portforward` (port-forward) and/or `get/create services/proxy` (API-proxy) **in the `monitoring` namespace** — a sensitive, separately-governed namespace an operator may not hold those verbs in. **Probe at gate-start** (`kubectl auth can-i get services/proxy -n monitoring --context <ctx>`); if absent ⇒ `inconclusive`, and **escalate to the federation path — do NOT widen RBAC** to force it (a sticky `monitoring` grant is closer to a one-way door than the tunnel is).
- **Warnings + `partial_response=false` are YOUR steps on the raw path, not a client-library freebie.** "Warnings-native" means the API *returns* them — it does not *extract* them for you. On a hand-built API call you MUST (a) append `&partial_response=false` to the query URL yourself, and (b) parse the top-level `warnings` array out of the JSON body yourself; **non-empty `warnings` ⇒ inconclusive ⇒ abort**, and a read whose body you did not parse for `warnings` is an *unconfirmed* read ⇒ inconclusive. (This is *why* direct-Thanos over `grafana/mcp-grafana`, which drops `warnings` entirely — but the obligation moves to you.) Note for range queries: QF has an in-memory range-response cache, so `partial_response=false` — not warnings-inspection alone — is what prevents a *cached* partial slipping through (instant-query soak polls aren't range-cached, so unaffected).
- **Query only operator-declared, cited rules — never an attacker-influenceable string (security control).** Because the guard reads with the operator's broad cluster privileges, "domain guards cite named rules, no arbitrary PromQL" is a *security* control, not just a quality one: it keeps the confused-deputy surface closed. Build every query from the gate's declared ledger / a fixed cited-rule allowlist, never from a fetched/injected name.
- **Accepted trade:** direct-Thanos-via-kubectl reads with the operator's **broad cluster privileges**, not a scoped credential — acceptable **only because** the guard runs **synchronously as the dispatching operator**, who already holds that access (no privilege is created, only borrowed). The moment any part runs **unattended/detached**, the trade is void and the scoped-principal federation work (PLT-527) is required.
- **Not a one-way door:** the API-server proxy / `port-forward` is an authenticated, RBAC-gated in-cluster tunnel — **not** public Thanos exposure.
- **Future (DEFERRED — when the guard runs *unattended* as its own scoped principal):** a federated, scoped read path. *Candidate mechanisms within that deferred design* (none built now): a Grafana datasource-proxy path, or an agent-native *warnings-preserving* MCP layer — authenticated by OIDC/JWT (or a scoped Viewer token in a metrics-only org). Tracked in **PLT-527** / `bdchatham-designs designs/sei-agentic-mesh/07-telemetry-guard-federation.md`. The read adapter is a **thin swappable seam**, so adopting it later is a localized change, not a rewrite.

## Citable query vocabulary — cite rules, do not re-derive

The guard **names an existing recording rule or alert expr**; it does not re-derive histograms or burn rates inline (in-context arithmetic is the #1 failure mode — all math lives in PromQL/recording rules; the agent reads scalars).

- `chaos_suite:block_time_p95:rate2m`, `chaos_suite:tps:rate1m`, `chaos_suite:block_height_delta:rate2m` — chain-health rates.
- `slo_*_{5m..30d}` ladders — the SLO "fraction-of-window-healthy" series (`sum_over_time((expr > bool THRESH)[w:]) / count` already exists).
- SRE **multi-window multi-burn-rate (MWMBR)** alert exprs — the "is it degrading vs SLO" math (continuous mode only; see contracts).

**Window/eval caveats (cite the expr, not the name):**
- `slo_*_3h` rules actually use a `[2h:]` window despite the `3h` name — cite the expression window.
- The `_3h` tier and recording rules are evaluated by **Prometheus at ~30s**; only the `≥3d -longwindow` series evaluate in **ThanosRuler at 5m**. This sets the per-source freshness budget below.

## Decision semantics — mirror Flagger (Argo Rollouts is the Argo-world equivalent)

Borrow the schema vocabulary as **prior art only — neither controller is deployed** (this is a Flux shop; the guard is an agent poll-loop, not a canary controller). Flagger is the Flux-ecosystem-native reference: declare the gate with `Canary.analysis` terms — `interval` (poll cadence), `threshold` (max failed checks before trip), `iterations` (windows observed), `metrics[].thresholdRange{min,max}`. (Argo Rollouts' `AnalysisTemplate` `interval`/`count`/`failureLimit`/`successCondition` is the one-to-one equivalent if that vocabulary is more familiar.) Two modes of *when* a guard watches (declared in the ledger entry):

- **`soak`** — poll for N minutes after the action, vs the gate-start baseline. The operative math is **N-consecutive-breach** (`count`/`failureLimit`) against the baseline. **MWMBR does NOT apply** — its slow window can't accumulate inside a 10–30 min soak; a soak guard must not claim slow-burn coverage.
- **`continuous`** — watch across a whole phase. MWMBR is the continuous-mode degradation math.
- **`pre-step`** — a single healthy check before the action.

## Non-negotiable correctness contracts

Eight contracts in two tiers. **Contracts 1–4 each gate the verdict** — a miss makes the reading unsound and the guard PASSes on a failure it exists to catch (it manufactures false confidence — worse than no guard). Contracts 5–8 are budget/tuning — set them wrong and the guard goes blind or noisy. Below any verdict-gating floor the verdict is `inconclusive`, never PASS.

### Verdict-gating (a miss makes the verdict unsound)

1. **Partial-response-safe — inspect `warnings` as the PRIMARY mechanism.** Thanos has partial-response ON by deliberate design; the cited rules are fleet-aggregated (`sum by (chain_id)`, `topk`), so on a cross-cluster blip a `sum` silently returns a partial total with HTTP 200 + a non-empty `warnings` array — and **freshness does not catch this** (the data that returned is current). **Non-empty `warnings` ⇒ inconclusive ⇒ abort.** Also force `?partial_response=false`. (The raw Thanos API *exposes* both — unlike `grafana/mcp-grafana`, which drops `warnings` — but on a hand-built API call **you** append the param and **you** parse the array; they're surfaced, not automatic. See the read adapter.)
2. **No-traffic ≠ healthy — volume floor + liveness co-condition.** A ratio SLI (`rate(ok)/rate(total)`) reads a clean `1.0` on a near-zero denominator — exactly the cutover failure where traffic isn't draining to the new shape. Every ratio guard co-asserts a **minimum-event-volume denominator** (e.g. `sum(rate(tx_count[1m])) >= <min>`) AND, for any consensus-touching cutover, a **liveness floor** (`chaos_suite:block_height_delta:rate2m > 0`) — separate cited queries that must independently pass. The `<min>` threshold is **declared per-guard at gate-start** (a function of the cutover's expected traffic floor — there is no global default; a guard with `<min>` unset is *mis-declared*, not permissive). Below the floor: `inconclusive`, never PASS.
3. **Target-coverage check.** Co-assert `up{…} == 1` (or expected-series cardinality) for the guard's targets — a pod that crashed mid-soak stops emitting and the last value carries forward inside Prometheus's 5m staleness window, reading "fresh enough." Coverage ≠ "the number is good."
4. **Empty rule result = inconclusive.** `topk` / `histogram_quantile` / join-pattern rules return *no series* when inputs are absent. Empty ⇒ inconclusive ⇒ abort, identical to stale. Never read absence as "not breached = healthy."

### Budget / tuning (set right or the guard is blind/noisy)

5. **Per-source freshness.** Budget by who evaluates the series: `~2×30s` for recording-rule + `_3h`-tier series (Prometheus); `~2×5m` for `≥3d -longwindow` series (ThanosRuler). Provenance records which store answered.
6. **Two-window / N-consecutive-breach, never a single threshold.** For **`soak`**: the verdict is **N-consecutive-breach of the cited query against the gate-start baseline** (Flagger `threshold`/`iterations`; Argo `count`/`failureLimit`). For **`continuous`**: MWMBR. **MWMBR never applies to `soak`** (its slow window can't accumulate in a 10–30 min soak) — a soak guard must not claim slow-burn coverage.
7. **Baseline captured at gate-start for cutovers.** `offset 1w` is **forbidden for a cutover** — a topology change means last week is a different fleet, and on the `prod` context a stale-baseline query risks reading the co-tenant **pacific-1 mainnet** (the prod/pacific-1 co-tenancy trap). Gate-start snapshot is mandatory for any topology change; `offset 1w` is permitted only for steady-state deploys. For soaks > ~15 min, compare rate/ratio-normalized (organic load ramps else read as degradation) or re-baseline.
8. **Effective detection latency = input-window + rule-interval.** A guard citing `…:rate2m` is ~2.5 min blind to a sharp drop; provenance states this; `continuous` fast-failure guards cite shorter-window rules.

**Fail-closed is the spine rule these all serve:** stale data, unreachable endpoint, auth/query error, empty read, or non-empty `warnings` ⇒ "cannot confirm healthy" = abort, **never** PASS. Same discipline as the workstream checkpoint and gov-ops.

## Provenance (every reading)

Record, per reading: the exact query (recording-rule name + the expr it expands to), the window, the store that answered, the `partial_response` setting, the `warnings` (empty or not), and the verdict (`pass` / `trip` / `inconclusive`). This is the re-runnable audit trail — and in measure mode (later) it *is* the deliverable.

## Surface vs act (gate mode, MVP)

`on_trip` is **surface-and-wait** — halt before the next step, surface the trip + the cited evidence, route to a pre-declared rollback checkpoint; the human owns the call. Auto-abort is **deferred (design OQ5), not an MVP build target**: if ever enabled it executes only a pre-declared *reversible, idempotent* rollback, never on a one-way-door step, and must clear a *higher, quantified* confidence bar than surface (design worked example: surface at N=3 consecutive breaches, auto-abort at N=5 — **the exact threshold is undecided pending OQ5, owner bdchatham**).

## A worked guard (ledger entry)

```
- guard:    arctic1-cutover-fleet-health
  signal:   chaos_suite:tps:rate1m            # fleet-aggregated; cite the expr, force partial_response=false
  healthy:  within 10% of the gate-start baseline snapshot
  coassert: sum(rate(sei_chain_app_tx_count_total[1m])) >= <min>   # volume floor, <min> set per-guard
            chaos_suite:block_height_delta:rate2m > 0               # liveness floor (consensus cutover)
            up{job="sei-validator",region="prod-euw1"} == 1         # target coverage
  when:     soak 15m                           # verdict = N-consecutive-breach vs baseline; MWMBR N/A
  on_trip:  surface + route to the rollback-region checkpoint
```

Every poll: non-empty `warnings` ⇒ inconclusive ⇒ abort; any co-assert unmet ⇒ inconclusive ⇒ abort; only all-clear + within-baseline for the full soak ⇒ PASS. Record the cited queries, windows, store, warnings, and verdict as provenance.

## What this kit does not do

- It is the measurement instrument, not the optimizer (relevant when measure mode lands).
- No generic arbitrary-PromQL guard — domain guards cite named rules.
- No Alertmanager grouped-alert read path (per-rule firing state covers MVP).
- No federated scoped-credential / tenancy-isolated read proxy (deferred — PLT-527). The MVP **does** read in-cluster direct-to-Thanos with operator access (see the read adapter); what's deferred is the *scoped/federated* path, not direct-to-Thanos itself.
- It is **not** a Flagger `Canary` / Argo `Rollout`. Those gate a k8s Deployment traffic-shift; this guard watches an *agent-driven* high-risk step (a validator cutover) inside a workstream. If the gate is ever wanted as *infrastructure* (a CRD) rather than an agent skill, **Flagger is the Flux-native home** for it — a deferred alternative, not this MVP.
