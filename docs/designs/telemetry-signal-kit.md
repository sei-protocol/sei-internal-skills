# Design: telemetry signal-kit — metric-driven guards for agentic workstreams

**Status:** Draft
**Date:** 2026-06-14
**Authors:** bdchatham, Claude (Coral)

*Design phase of a `/workstream`; cross-reviewed; design-approval checkpoint pending. No Linear issue filed yet — splits into two issues at approval, see "Issues & sequencing."*

## Background

The `workstream` skill (PLT-500) gives an agentic workstream **human** checkpoints. The gap this design closes: a **machine gate driven by a live metric signal** — the agent as the tireless second watcher staring at the dashboards during a high-risk live procedure (a migration cutover, a deploy), while the human still owns the irreversible go/no-go. The pattern is Amazon's "bake a change against a known aggregate alarm for N minutes before the pipeline proceeds, with immediate rollback if it trips," generalized for agents and made provenance-bearing (every reading is a cited, re-runnable query).

The value the MVP proves is **detection**, not rollback: *an agent can watch a cited live metric, evaluate it fail-closed, and gate/surface before a risky step.* Amazon's auto-rollback was the action; catching the bad change was the point. The MVP ships the catch.

Build-vs-reuse was sourced first (via the `/research` method, 4 blind sweeps + adversarial verify) — see `design/research/telemetry-signal-kit-tooling.md`. Headline: **reuse on every layer** (official `grafana/mcp-grafana`; existing Thanos Query Frontend + recording rules; SRE multi-burn-rate + Argo-Rollouts/Flagger decision schema). The cross-review surfaced a production precedent the research missed: `validate-release/scripts/query-grafana.py` **already authenticates to Grafana's datasource-proxy with a bearer token exactly as this design proposes** — the federation bet is already running, not speculative.

This is the **first (exemplar) kit** of a signal-kit *spine* aligned with Brandon. The spine is stated once and proven by this kit; two later kits (indexed-events, perf/eBPF) are named only to pressure-test that the spine generalizes — their modes are out of scope here and will be specified when those kits are designed. We do **not** commit a kit×mode roadmap on the evidence of one cell.

## Goals

1. A **`guard`** primitive: a non-human, signal-driven gate that extends the `/workstream` checkpoint ledger — declared up front, fail-closed, provenance-bearing; it surfaces/halts before a risky step proceeds if the signal isn't healthy.
2. The **telemetry signal kit** (gate mode, MVP): the read adapter (`grafana/mcp-grafana`, read-only), the citable query vocabulary (existing `chaos_suite:*` / `slo_*` recording rules + SRE multi-burn-rate), and the gate-decision semantics (Argo-Rollouts/Flagger-style `interval`/`count`/`failureLimit` + the telemetry-kit correctness contracts below).
3. The **signal-kit spine** stated once, kit-invariant (so indexed-events and perf kits drop in later), with its contracts cleanly separated from telemetry-specific ones.
4. Encode the correctness contracts the research + SRE cross-review surfaced, **tiered** into spine-level (every kit) and telemetry-kit-level (this kit) — see "Correctness contracts."

(Spine generalization beyond statement is **deferred — when the second kit (indexed-events/coordinate) is actually built**. We generalize at the second kit, not the first.)

## Non-goals

- **Not** the measure or coordinate modes, nor the perf/eBPF or blockchain-indexed-events kits (later kits; the spine is designed to admit them, proven only when built).
- **Not** the federation *automation* in the guard issue — the guard primitive builds and tests against a **manually-provisioned Viewer token**; the operator-CR/Secrets-Manager automation is a sibling issue (see "Issues & sequencing"). *Operator-CR automation deferred — when more than one agent/guard needs a token, or rotation becomes manual toil.*
- **Not** an in-cluster direct-to-Thanos read path + `prom-label-proxy` tenancy (*deferred — when an agent runs untrusted/model-authored queries needing per-call audit, or must be denied a tenant like pacific-1*).
- **Not** a generic "arbitrary PromQL" guard, and **not** a generic escape hatch in MVP — domain guards only. *Generic escape hatch deferred — when a real guard need can't be expressed as a domain guard.* (Same RBAC-boundary reasoning as the gov-ops content-specific decision.)
- **Not** an Alertmanager grouped/silenced-alert read path (*deferred — when a guard must watch an Alertmanager-grouped aggregate alarm*; per-rule firing state via grafana-mcp covers MVP).
- **Not** auto-abort in MVP — surface-only (see "Surface-vs-act").
- **Not** building anything before design-approval.

## Design

### The `guard` primitive (extends the workstream ledger)

The `/workstream` ledger today holds `checkpoint`s (human gates). Add a second entry kind, `guard` (a signal gate):

```
- guard:    <name>
  signal:   <a CITED live query — a recording-rule name / PromQL expr / a firing-alert rule>
  healthy:  <condition — baseline-relative at gate-start (cutover) or absolute SLO threshold>
  when:     pre-step  (gate before the risky action)
          | soak      (poll for N minutes after the action, vs the gate-start baseline)
          | continuous(watch across the whole phase)
  on_trip:  surface + route to a PRE-DECLARED rollback checkpoint   (MVP default)
          | auto-abort a PRE-DECLARED reversible rollback           (deferred — OQ5)
```

The three `when:` modes are the contract: `pre-step` gates before the action, `soak` polls for N minutes after it, `continuous` watches the whole phase. `on_trip` is **fail-closed and surface-by-default** — a trip always halts before the next step and routes to a pre-declared rollback checkpoint; auto-abort is the deferred exception (OQ5). A high-risk step (e.g. a validator node-shape cutover) declares **both** a `guard` (the metric watch) and a human `checkpoint` (the go/no-go). The agent: capture baseline → human go-ahead → execute → soak-watch the guard → trip ⇒ halt + surface + route to the declared rollback checkpoint.

**Recursion bound (reuses the workstream's).** A guard may **surface, halt, and route to an already-declared checkpoint** — nothing more. It may **not** launch a workstream, spawn a sub-agent that launches one, or create new ledger entries at trip time. All ledger entries (checkpoints *and* guards) are declared up front; the ledger is **static after declaration**. Auto-abort (when enabled, OQ5) executes a **pre-declared reversible rollback action**, never open-ended remediation. This is the bound that keeps a continuous guard's trip handler from becoming an unbounded watch→remediate→watch loop.

### The signal-kit spine (kit-invariant)

Every signal kit, regardless of source, implements the same five verbs: **declare → fetch (fail-closed) → evaluate (via a cited query) → act (gate/surface) → capture provenance.** The pluggable kit supplies a **read adapter**, a **citable query/event vocabulary**, and the **domain semantics**; the verbs are constant.

The verbs survive the coordinate-mode stress test (a blockchain indexed event): declare ("await proposal X reaching quorum"), fetch fail-closed (indexer unreachable ⇒ cannot confirm ⇒ never PASS), evaluate a cited query (an indexed-events filter, the analog of a recording rule), act (gate/surface), capture provenance (query + block range + result). What does *not* translate is telemetry vocabulary — which is why the contracts are tiered below rather than listed as one spine-level block.

### The telemetry kit (MVP)

- **Read adapter:** `grafana/mcp-grafana` (official, Apache-2.0) — PromQL instant/range + per-rule firing-alert state; Thanos-transparent via the Grafana datasource. No custom Thanos MCP. **Precedent:** `validate-release/scripts/query-grafana.py` already calls `…/api/datasources/proxy/uid/<uid>/api/v1/query_range` with a bearer token in production — same auth surface, working today.
- **Citable query vocabulary:** cite the *existing* recording rules (`chaos_suite:block_time_p95:rate2m`, `:tps:rate1m`, `:block_height_delta:rate2m`, the `slo_*_{5m..30d}` ladders) and SRE **multi-window multi-burn-rate** alert exprs. The guard names a rule; it does **not** re-derive histograms inline. The `chaos_suite` validation agent's poll loop is the implementation template (OQ3+OQ4: estimated 60–70% reusable, pending the source pull).
- **Decision semantics:** mirror **Argo Rollouts `AnalysisTemplate` / Flagger** (`interval` / `count` / `failureLimit` / `successCondition` / `thresholdRange`). Threshold/rate/quantile/window-fraction are PromQL-native (the `sum_over_time((expr > bool THRESH)[w:])/count` "fraction-of-window-healthy" primitive already exists in `recording-validator-slos.yaml`); baseline-relative deltas across the cutover boundary are the one genuinely agent-side computation — pushed into a pinned two-window query, not in-context arithmetic.

### Correctness contracts (tiered)

**Spine-level — every kit MUST honor these (kit-invariant):**

- **Fail-closed.** Stale data, unreachable endpoint, auth/query error, *or an empty/incomplete read* ⇒ "cannot confirm healthy" = degraded/abort, **never** PASS (same spine as the workstream checkpoint + gov-ops).
- **Signal-is-current-enough-to-trust.** A freshness precondition before any verdict (the kit defines the mechanism and threshold).
- **Incomplete-read-is-inconclusive.** Any partial/degraded read (telemetry: a non-empty `warnings` array; coordinate: indexer behind chain head) ⇒ inconclusive ⇒ abort. Never PASS on a partial view.
- **Cite-the-re-runnable-query / provenance.** Every reading records the exact query, the window, the store that answered, and the result. The provenance contract — **not** the upstream's own logs — is the primary query audit trail.
- **Act = surface-by-default.** Auto-act only on a step the workstream declares reversible (see "Surface-vs-act").

**Telemetry-kit-level — this kit's realization of "current enough / complete / sound" (does NOT generalize):**

- **Partial-response-safe — inspect the `warnings` array as the PRIMARY mechanism.** The deployed Thanos Query has partial-response ON *by deliberate design* (`thanos-query.yaml` — partial data beats a 503 on a cross-cluster blip). The cited rules are **fleet-aggregated** (`sum by (chain_id)`, `topk`), so on a partial read a `sum` silently returns a partial total with HTTP 200 + a `warnings` array — and **freshness does not catch this** (the data that returned is current). The guard MUST treat a non-empty `warnings` as inconclusive→abort; `?partial_response=false` is belt-and-suspenders only (and may not survive the Grafana-proxy hop — see Issue B / OQ-warnings).
- **No-traffic ≠ healthy (volume floor + liveness co-condition).** A ratio SLI (`tx_success_rate = rate(ok)/rate(total)`) reads a clean `1.0` on near-zero denominator — exactly the cutover failure mode (traffic not draining to the new shape). Every ratio guard requires a **co-asserted minimum-event-volume denominator** (e.g. `sum(rate(tx_count[1m])) >= <min>`) AND, for any consensus-touching cutover, a **liveness floor** (`chaos_suite:block_height_delta:rate2m > 0`), both as separate cited queries that must independently pass. Below the floor the verdict is `inconclusive`, never PASS. (The chaos-suite authors already added `block_height_delta:rate2m` precisely because sample-count can't distinguish a halt from stale carry-forward.)
- **Target-coverage check.** Co-assert `up{…} == 1` (or expected-series cardinality) for the targets the guard depends on — a pod that crashed mid-soak stops emitting, and the last value carries forward inside Prometheus's 5m staleness window, reading "fresh enough." Coverage is distinct from "the number is good."
- **Empty rule result = inconclusive.** `topk` / `histogram_quantile` / join-pattern rules return **no series** when inputs are absent. Empty ⇒ inconclusive ⇒ abort, identical to stale. Never read absence as "not breached = healthy."
- **Per-source freshness.** `2*scrape_interval` (30s) for raw/recording-rule series; `2*rule_eval_interval` for **ThanosRuler-served** series (the ≥3d SLO rules evaluate in the Ruler, a separate freshness domain). Provenance records which store answered.
- **Two-window / N-consecutive-breach, never a single threshold.** For **`soak`** mode this is the operative math (Argo/Flagger `count`/`failureLimit` against the gate-start baseline). **MWMBR is `continuous`-mode math only** — its slow window cannot accumulate inside a 10–30 min soak, so a soak guard MUST NOT claim MWMBR slow-burn coverage in its provenance.
- **Baseline captured at gate-start for cutovers.** `offset 1w` is **not permitted for a cutover** (topology changes; last week is a different node count — and risks the prod/pacific-1 co-tenancy trap). Gate-start snapshot is mandatory for topology changes; `offset 1w` is allowed only for steady-state *deploys*. For soaks > ~15 min, comparisons must be rate/ratio-normalized (organic load ramps else read as degradation), or re-baseline.
- **Effective detection latency = input-window + rule-interval.** A guard citing `…:rate2m` is ~2.5 min blind to a sharp drop; provenance must state this, and `continuous`-mode fast-failure guards should cite shorter-window rules.
- **All math in PromQL/recording rules; the agent reads scalars** (in-context arithmetic is the #1 failure mode).

### Surface-vs-act decision

`on_trip` is **surface-and-wait by default**; **auto-abort is deferred to OQ5** and, when enabled, allowed only when **both** (a) the step is declared reversible **and** (b) the rollback action is itself idempotent (re-running it when no rollback is needed is a no-op). A one-way-door step always surface-and-waits — the guard detects, the human owns the irreversible call.

Auto-abort must clear a **higher confidence bar** than surface (e.g. surface at N=3 consecutive breaches, auto-abort at N=5) so an irreversible-ish "roll back a live system" isn't triggered by the same thin signal that merely surfaces. The cost of a false trip is asymmetric: a wrong surface costs a human 30 seconds of "proceed"; a wrong auto-abort is an unplanned rollback during a live migration. **MVP ships surface-only.**

### Decision: the `guard` lives in the `/workstream` skill (revisit when guards are needed outside a workstream)

The `guard` primitive lives **in the `/workstream` skill** (a second ledger entry kind), with the telemetry kit as a reference pack (`references/signal-kit-telemetry.md`) — the spine is workstream's, the kit is data, mirroring idiomatic's pack model. This is *decided*, not open: a guard's whole reason to exist is to gate a step in a procedure (the workstream); `on_trip` already routes to a workstream `checkpoint`; fail-closed is "the same spine as the checkpoint." A standalone `/guard` skill would re-import the checkpoint concept to do its one job. Revisit only if guards are ever needed outside a workstream.

## Issues & sequencing

This workstream splits into **two issues** so the guard primitive isn't held hostage to a prod-auth infra decision:

- **Issue A — guard primitive + telemetry kit** (the skill work: the ledger entry, the spine verbs, the tiered contracts, the decision semantics, the poll loop). Dependency: *a read-only Grafana token*. Built and tested against a **manually-issued Viewer token**. This is the workstream's deliverable and what proves the hypothesis.
- **Issue B — metric-federation slice** (sibling, owned by platform-engineer): SA-token automation via the `GrafanaServiceAccount` CR → Secrets Manager → CSI, gated on OQ1/OQ2/OQ3. Can proceed in parallel; if it slips on OQ3 it does not block Issue A.

### The federation slice (Issue B — a one-way door)

- **Read path:** Thanos **Query Frontend** (`thanos-query-frontend.monitoring.svc:9090`) — the fleet-global view Grafana already trusts.
- **Auth:** a **Grafana service-account Viewer token** → datasource-proxy (the non-interactive answer to the headless crux; Grafana's Google OAuth is interactive-only). Provisioned via a `GrafanaServiceAccount` CR through the already-deployed Grafana Operator; token → AWS Secrets Manager → CSI. **Use a distinct SA per consumer** — the guard must not share the chaos-suite token, so its calls are independently attributable and revocable. Set a **finite token expiry** + rotation (MVP cut-first: short manual expiry + a calendar reminder; un-defer rotation when a second consumer shares the path).
- **Read-only is structural — confirm, don't assume:** Viewer role is enforced at Grafana; the write-free guarantee additionally rests on Thanos Query exposing **no admin path** (confirm `--query.enable-admin` is off; receive/ruler not routed through this datasource). On OSS Grafana the floor is Viewer-across-**all**-datasources — acceptable **only if** that Grafana fronts metrics-only sources; if it fronts Loki/Tempo/SQL with sensitive content, a leaked token reads those too (escalates blast radius — confirm the datasource inventory).
- **Coupling to flag:** routing through Grafana makes **guard availability bounded by Grafana availability** — a Grafana outage fails all guards closed (correct direction, but a new coupling the in-cluster-direct path would avoid).
- **One-way door:** **publicly exposing Thanos's query API is out of scope and requires explicit human approval to even propose** — it contradicts the deliberate "no public Thanos gRPC" stance. Route external agents through Grafana for MVP.

## Alternatives considered

- **Generic arbitrary-PromQL guard** as the primary surface — rejected as the default (wide, hard-to-reason; same boundary reasoning as gov-ops); deferred entirely from MVP (not even an escape hatch).
- **Custom Thanos MCP / hand-rolled HTTP client** — rejected; `grafana/mcp-grafana` is official, current, Thanos-transparent.
- **In-agent metric math / a "math MCP"** — rejected; math lives in PromQL/recording rules, the agent reads scalars.
- **Kayenta-style statistical ACA** — rejected as a dependency (archived); borrow the distribution-comparison idea only if simple thresholds prove flappy.
- **In-cluster direct-to-Thanos + prom-label-proxy** — deferred (see Non-goals).
- **Guard as a standalone `/guard` skill** — rejected (see Decision); guard's home is structurally the workstream ledger.

## Trade-offs

- **Grafana-proxy read path** adds a hop *and* couples guard availability to Grafana — accepted for MVP (only authenticated, externally-reachable, non-interactive surface, and already proven by `query-grafana.py`); in-cluster direct is the later optimization.
- **Domain guards over generic** — less flexible day one, safer and self-describing.
- **Surface-only (not auto-abort)** — less hands-off than Amazon's auto-rollback, but detection is the value; reversible-step auto-abort (OQ5) recovers the rest once the gate earns trust.
- **Two issues, manual token first** — slightly more coordination, but unblocks the primitive from the highest-leverage infra unknown (OQ3).
- **Dogfooding the workstream to design this** — slower than ad-hoc, but validates the checkpoint primitive and is the standard.

## Open questions (to resolve at / before design-approval)

- **OQ1 (Grafana edition)** — per-datasource SA scoping is an Enterprise/RBAC feature; if OSS, the floor is Viewer-across-all-datasources. Acceptable only if Grafana fronts metrics-only sources (confirm inventory). Owner: platform-engineer. *Verify before Issue B is built.*
- **OQ2 (`GrafanaServiceAccount` CR)** — confirm the deployed Grafana Operator version supports the CR. **Not a blocker** — worst case the token is minted out-of-band into Secrets Manager (the runtime consumes a token from env regardless of how it was minted). Owner: platform-engineer.
- **OQ3+OQ4 (collapsed — pull the chaos-suite agent's runner spec)** — *one* investigation resolves both: where headless/cron agents run (decides Grafana-proxy vs in-cluster-direct — the only unknown that changes the architecture) **and** how much of the poll loop is reusable. The `query-grafana.py` evidence (public Grafana hostname + `AWS_PROFILE`) strongly implies **external/CI → Grafana-proxy**, which would confirm the MVP path. Owner: observability-platform-engineer + bdchatham. *Highest leverage; do first.*
- **OQ5 (auto-abort scope)** — MVP ships surface-only. Auto-abort un-defer trigger is **measurable, not "once trusted"**: after the surface-only gate has run on N real procedures with a false-trip rate below tolerance AND the step declares a tested idempotent reversible rollback. Owner: bdchatham.
- **OQ-warnings (blocks the partial-response contract) — `/issue` to observability-platform-engineer:** confirm `grafana/mcp-grafana` surfaces the Thanos `warnings` array to the caller through the datasource-proxy hop, and whether `partial_response=false` survives it. If grafana-mcp swallows warnings, the guard cannot be partial-response-safe through this path and Issue B needs reconsidering.
- **OQ-rules (provenance correctness) — `/issue` to observability-platform-engineer:** `slo_*_3h` rules use a `[2h:]` window (mislabeled); until fixed, guards cite the expr window, not the rule name. Also provide the ThanosRuler eval interval for per-source freshness thresholds.

## References

- `design/research/telemetry-signal-kit-tooling.md` (the build-vs-reuse research, this workstream).
- The `/workstream` + checkpoint primitive (PLT-500); the `/research` method (PLT-501).
- `grafana/mcp-grafana`; SRE Workbook ch.5 (MWMBR); Argo Rollouts AnalysisTemplate; Flagger.
- Production precedent: `.claude/skills/validate-release/scripts/query-grafana.py` (datasource-proxy + bearer-token auth, today).
- Live stack: `clusters/prod/monitoring/{thanos-query,grafana,grafana-operator,recording-rules-chaos-suite}.yaml`; `alerts/protocol/recording-{validator,chain}-slos.yaml`.
