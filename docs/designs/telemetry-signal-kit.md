# Design: telemetry signal-kit — metric-driven guards for agentic workstreams

**Status:** Draft
**Date:** 2026-06-14
**Authors:** bdchatham, Claude (Coral)

*Design phase of a `/workstream`; cross-reviewed; design-approval checkpoint pending. No Linear issue filed yet — splits into two issues at approval, see "Issues & sequencing."*

## Background

The `workstream` skill (PLT-500) gives an agentic workstream **human** checkpoints. The gap this design closes: a **machine gate driven by a live metric signal** — the agent as the tireless second watcher staring at the dashboards during a high-risk live procedure (a migration cutover, a deploy), while the human still owns the irreversible go/no-go. The pattern is Amazon's "bake a change against a known aggregate alarm for N minutes before the pipeline proceeds, with immediate rollback if it trips," generalized for agents and made provenance-bearing (every reading is a cited, re-runnable query).

The value the MVP proves is **detection**, not rollback: *an agent can watch a cited live metric, evaluate it fail-closed, and gate/surface before a risky step.* Amazon's auto-rollback was the action; catching the bad change was the point. The MVP ships the catch.

Build-vs-reuse was sourced first (via the `/research` method, 4 blind sweeps + adversarial verify) — see `design/research/telemetry-signal-kit-tooling.md`. Headline: **reuse on every layer** (official `grafana/mcp-grafana`; existing Thanos Query Frontend + recording rules; SRE multi-burn-rate + Flagger/Argo decision schema). The cross-review surfaced a production precedent the research missed: `validate-release/scripts/query-grafana.py` **already authenticates to Grafana's datasource-proxy with a bearer token exactly as this design proposes** — the federation bet is already running, not speculative.

This is the **first (exemplar) kit** of a signal-kit *spine* aligned with Brandon. The generalization the spine must demonstrably support — and the reason this is a primitive, not a one-off gate — is **two orthogonal axes**:

- **Kit (the data source):** telemetry (PromQL/Thanos, this kit), later blockchain-indexed-events, later perf/eBPF.
- **Mode (what a reading *does* — the "act" verb):** **gate** (defensive, per-step: rollout/CICD/ops — *MVP*), **measure** (generative, iterative: optimize/benchmark toward a target value under controlled conditions — *proven-admitted below, not built*), **coordinate** (an event barrier — later).

Mode ⊥ kit: telemetry can feed *both* gate and measure (benchmark via PromQL); perf/eBPF feeds measure. The spine is stated once and **proven** to admit gate *and* measure (see "Modes"); MVP **builds** only telemetry/gate. We do **not** commit to building the matrix on one cell of evidence — but the abstraction is required to hold for the cell we'll want next.

## Goals

1. A **`guard`** primitive — the **gate-mode instance** of the general **signal binding** (a declared, cited, fail-closed, provenance-bearing metric condition). The guard extends the `/workstream` checkpoint ledger; it surfaces/halts before a risky step proceeds if the signal isn't healthy. (The measure-mode instance — an *objective* a workstream optimizes toward — is designed-for, not built; see "Modes.")
2. The **telemetry signal kit** (gate mode, MVP): the read adapter (`grafana/mcp-grafana`, read-only), the citable query vocabulary (existing `chaos_suite:*` / `slo_*` recording rules + SRE multi-burn-rate), and the gate-decision semantics (Flagger-style `interval`/`threshold`/`iterations`, schema prior art only + the telemetry-kit correctness contracts below).
3. The **signal-kit spine** stated once, kit- *and* mode-invariant, with its contracts cleanly separated from telemetry-specific ones, and **proven** to admit measure mode (the iterative-optimization control structure), not just gate.
4. A **decided auth layer** (grafana-mcp interface + a Grafana service-account Viewer token) — standard, not one-off; see "Auth."
5. Encode the correctness contracts the research + SRE cross-review surfaced, **tiered** into spine-level (every kit/mode) and telemetry-kit-level (this kit) — see "Correctness contracts."

(Building modes/kits beyond telemetry/gate is **deferred — measure when a perf/benchmark workstream needs it; coordinate when an indexed-events barrier does.** We *prove* generality at the first kit and *build* it at the second.)

## Non-goals

- **Not** *building* the measure or coordinate modes, nor the perf/eBPF or blockchain-indexed-events kits — the spine is *proven* to admit them ("Modes"), built later. Measure ships when a perf/benchmark workstream needs it; coordinate when an indexed-events barrier does.
- **Not** the federation *automation* in the guard issue — the guard primitive builds and tests against a **manually-provisioned Viewer token**; the operator-CR/Secrets-Manager automation is a sibling issue (see "Issues & sequencing"). *Operator-CR automation deferred — when more than one agent/guard needs a token, or rotation becomes manual toil.*
- **Not** an in-cluster direct-to-Thanos read path + `prom-label-proxy` tenancy (*deferred — when an agent runs untrusted/model-authored queries needing per-call audit, or must be denied a tenant like pacific-1*).
- **Not** a generic "arbitrary PromQL" guard, and **not** a generic escape hatch in MVP — domain guards only. *Generic escape hatch deferred — when a real guard need can't be expressed as a domain guard.* (Same RBAC-boundary reasoning as the gov-ops content-specific decision.)
- **Not** an Alertmanager grouped/silenced-alert read path (*deferred — when a guard must watch an Alertmanager-grouped aggregate alarm*; per-rule firing state via grafana-mcp covers MVP).
- **Not** auto-abort in MVP — surface-only (see "Surface-vs-act").
- **Not** building anything before design-approval.

## Design

### The `guard` primitive — the gate-mode signal binding (extends the workstream ledger)

The `guard` is the **gate-mode instance** of a signal binding (the measure-mode instance, an `objective`, is in "Modes"). The `/workstream` ledger today holds `checkpoint`s (human gates). Add a second entry kind, `guard` (a signal gate):

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

### The signal-kit spine (kit- and mode-invariant)

Every signal binding, regardless of source or mode, implements the same five verbs: **declare → fetch (fail-closed) → evaluate (via a cited query) → act → capture provenance.** The pluggable **kit** supplies a *read adapter* + *citable query/event vocabulary*; the **mode** supplies the *act semantics* (what a reading does); the verbs are constant. Four of the five verbs are fully invariant — only **act** varies by mode.

The verbs survive the coordinate-mode stress test (a blockchain indexed event): declare ("await proposal X reaching quorum"), fetch fail-closed (indexer unreachable ⇒ cannot confirm ⇒ never PASS), evaluate a cited query (an indexed-events filter, the analog of a recording rule), act (unblock the barrier), capture provenance (query + block range + result). What does *not* translate is telemetry vocabulary — which is why the contracts are tiered below rather than listed as one spine-level block.

### Modes (the `act` layer — proving the spine admits more than gate)

A **mode** is the decision semantics layered on a reading. The spine produces a fail-closed, cited reading; the mode decides what it *does* and how it composes with the workstream:

| Mode | Control structure | `act` produces | Workstream relationship | Status |
|------|-------------------|----------------|-------------------------|--------|
| **gate** | defensive, one-shot per step, binary | `healthy → proceed` / `trip → halt+rollback` | extends the **checkpoint ledger** (a `guard` gates a step) | **MVP** |
| **measure** | generative, *iterative loop*, convergent | `distance-to-objective → {converged, keep-going, budget-exhausted}` | *is* the workstream's **objective / loop terminator** ("done = a measured value under benchmark conditions X") | designed-for, not built |
| **coordinate** | reactive, event-driven | `event-reached → unblock` | a **synchronization barrier** between steps | later |

**Measure-mode walkthrough (proving it fits the spine).** A perf-optimization workstream — "drive p99 commit latency below X under benchmark workload W on node-shape S" — runs the *same five verbs*: **declare** the objective + benchmark circumstances (workload, node shape, the cited metric); **fetch** fail-closed (benchmark harness or Thanos unreachable ⇒ cannot claim progress, never false-converge); **evaluate** the objective metric via a cited, re-runnable query; **act** = report *distance-to-objective* into the optimization loop (converged / keep-going / exhausted) rather than gate a step; **capture provenance** — and here provenance is *more* central than in gate mode: the re-runnable query, the benchmark conditions, and the historical series *are the deliverable* (your "technical documentation on the data, historical queries to retrieve it again"). The spine holds; only `act` and the workstream relationship change.

**The boundary that keeps measure mode in scope-bounds: the signal-kit is the measurement instrument, not the optimizer.** It answers, provably and with citation, *"where am I relative to the objective?"* The agent/workstream decides *what change to try next*. We are not building a perf auto-tuner; we are building the fail-closed, provenance-bearing measurement + objective-evaluation that an optimization workstream consumes. (This is also why telemetry serves measure today and perf/eBPF is a *richer measure-mode kit* later — kit ⊥ mode.)

MVP **builds** gate; this section exists to **prove** the spine and the contract-tiering admit measure without rework — the test Brandon set. Measure mode ships when a perf/benchmark workstream needs it.

### The telemetry kit (MVP)

- **Read adapter:** the Grafana **datasource-proxy** HTTP call (`…/api/datasources/proxy/uid/prometheus-prod/api/v1/query[_range]`), extending the production `validate-release/scripts/query-grafana.py` (~50–60% of the mechanical loop reuses directly — proxy call, token preflight, window-stats reduction; the correctness contracts below are net-new). PromQL instant/range, Thanos-transparent, **warnings-preserving** (the reason this beats `grafana/mcp-grafana`, which drops them — see "Auth"). Per-rule firing-alert state via the Grafana alerting API or the `ALERTS` series.
- **Citable query vocabulary:** cite the *existing* recording rules (`chaos_suite:block_time_p95:rate2m`, `:tps:rate1m`, `:block_height_delta:rate2m`, the `slo_*_{5m..30d}` ladders) and SRE **multi-window multi-burn-rate** alert exprs. The guard names a rule; it does **not** re-derive histograms inline. The `chaos_suite` validation agent's poll loop is the implementation template (OQ3+OQ4: estimated 60–70% reusable, pending the source pull).
- **Decision semantics:** mirror **Flagger's `Canary.analysis` / `MetricTemplate`** (`interval` / `threshold` / `iterations` / `metrics[].thresholdRange`) — the Flux-ecosystem-native reference (Argo Rollouts' `AnalysisTemplate` is the one-to-one equivalent). **Schema prior art only — neither controller is deployed** (this is a Flux shop; the guard is an agent poll-loop, not a `Canary`/`Rollout`). If the gate is ever wanted as *infrastructure* (a CRD) rather than an agent skill, Flagger is the Flux-native home — a deferred alternative, not MVP. Threshold/rate/quantile/window-fraction are PromQL-native (the `sum_over_time((expr > bool THRESH)[w:])/count` "fraction-of-window-healthy" primitive already exists in `recording-validator-slos.yaml`); baseline-relative deltas across the cutover boundary are the one genuinely agent-side computation — pushed into a pinned two-window query, not in-context arithmetic.

### Auth (decided — standard convention, not one-off)

The interface and the credential are distinct layers:

- **Credential — a Grafana service account, `Viewer` role, service-account token.** Service accounts are Grafana's standard programmatic-access mechanism (GA since Grafana 9.1, the explicit replacement for API keys) — *this is the convention*, not a one-off. Viewer makes read-only structural. Precedent in-repo: `validate-release/scripts/query-grafana.py` consumes exactly this token shape.
- **Interface — the Grafana datasource-proxy HTTP call** (`/api/datasources/proxy/uid/prometheus-prod/api/v1/query[_range]`), the established `query-grafana.py` pattern. **Not `grafana/mcp-grafana`** for gated reads — see the decision below. The proxy returns the raw Prometheus/Thanos envelope, so it **preserves the `warnings` array and honors `partial_response=false`** — both load-bearing for the partial-response contract.
- **Scope — distinct service account per consumer.** The guard gets its own SA (not the chaos-suite one), so its calls are independently attributable and revocable, with a finite token expiry.

**Why not the MCP server (OQ-warnings, resolved — a blocking finding).** `grafana/mcp-grafana` **discards the Thanos `warnings` array** (verified in `main`: its query tool binds the Prometheus client's `v1.Warnings` to `_`, its result struct has no warnings field, and it exposes no `partial_response` param). The community fallback `pab1it0/prometheus-mcp-server` drops warnings too. Routing gated reads through either is **not partial-response-safe** — on a cross-cluster blip Thanos returns HTTP 200 + partial data + non-empty `warnings`, and the MCP server would hand the agent a clean-looking partial total, defeating the highest-priority correctness contract. The raw datasource-proxy call (already proven by `query-grafana.py`) is therefore both *correct* and *more* reuse, not less.

The **runtime contract is decided for MVP**: a Viewer SA token + the warnings-preserving datasource-proxy call. Only the *minting/rotation automation* is deferred (Issue B) — the operator's `GrafanaServiceAccount` CR is the GitOps path (confirmed supported, operator v5.22.2); a manually-minted token is the identical-contract fallback for Issue-A development.

### Correctness contracts (tiered)

**Spine-level — every kit MUST honor these (kit-invariant):**

- **Fail-closed.** Stale data, unreachable endpoint, auth/query error, *or an empty/incomplete read* ⇒ "cannot confirm healthy" = degraded/abort, **never** PASS (same spine as the workstream checkpoint + gov-ops).
- **Signal-is-current-enough-to-trust.** A freshness precondition before any verdict (the kit defines the mechanism and threshold).
- **Incomplete-read-is-inconclusive.** Any partial/degraded read (telemetry: a non-empty `warnings` array; coordinate: indexer behind chain head) ⇒ inconclusive ⇒ abort. Never PASS on a partial view.
- **Cite-the-re-runnable-query / provenance.** Every reading records the exact query, the window, the store that answered, and the result. The provenance contract — **not** the upstream's own logs — is the primary query audit trail.
- **Act = surface-by-default.** Auto-act only on a step the workstream declares reversible (see "Surface-vs-act").

**Telemetry-kit-level — this kit's realization of "current enough / complete / sound" (does NOT generalize):**

- **Partial-response-safe — inspect the `warnings` array as the PRIMARY mechanism.** The deployed Thanos Query has partial-response ON *by deliberate design* (`thanos-query.yaml` — partial data beats a 503 on a cross-cluster blip). The cited rules are **fleet-aggregated** (`sum by (chain_id)`, `topk`), so on a partial read a `sum` silently returns a partial total with HTTP 200 + a `warnings` array — and **freshness does not catch this** (the data that returned is current). The guard MUST treat a non-empty `warnings` as inconclusive→abort, AND force `?partial_response=false`. **Both are available only on the raw datasource-proxy path** (the chosen read adapter); `grafana/mcp-grafana` drops warnings and exposes no partial-response param (OQ-warnings) — this is *why* the adapter is the proxy call, not the MCP server.
- **No-traffic ≠ healthy (volume floor + liveness co-condition).** A ratio SLI (`tx_success_rate = rate(ok)/rate(total)`) reads a clean `1.0` on near-zero denominator — exactly the cutover failure mode (traffic not draining to the new shape). Every ratio guard requires a **co-asserted minimum-event-volume denominator** (e.g. `sum(rate(tx_count[1m])) >= <min>`) AND, for any consensus-touching cutover, a **liveness floor** (`chaos_suite:block_height_delta:rate2m > 0`), both as separate cited queries that must independently pass. Below the floor the verdict is `inconclusive`, never PASS. (The chaos-suite authors already added `block_height_delta:rate2m` precisely because sample-count can't distinguish a halt from stale carry-forward.)
- **Target-coverage check.** Co-assert `up{…} == 1` (or expected-series cardinality) for the targets the guard depends on — a pod that crashed mid-soak stops emitting, and the last value carries forward inside Prometheus's 5m staleness window, reading "fresh enough." Coverage is distinct from "the number is good."
- **Empty rule result = inconclusive.** `topk` / `histogram_quantile` / join-pattern rules return **no series** when inputs are absent. Empty ⇒ inconclusive ⇒ abort, identical to stale. Never read absence as "not breached = healthy."
- **Per-source freshness.** Budget by *who evaluates the series* (OQ-rules): recording-rule + SLO `_3h`-tier series are evaluated by **Prometheus at ~30s** (use `~2*30s`); only the **`≥3d -longwindow` series** evaluate in **ThanosRuler at 5m** (use `~2*5m`). The `_3h` rules also use a `[2h:]` window despite the `3h` name — cite the **expr window, not the rule name**. Provenance records which store answered.
- **Two-window / N-consecutive-breach, never a single threshold.** For **`soak`** mode this is the operative math (Flagger `threshold`/`iterations`; Argo `count`/`failureLimit`, against the gate-start baseline). **MWMBR is `continuous`-mode math only** — its slow window cannot accumulate inside a 10–30 min soak, so a soak guard MUST NOT claim MWMBR slow-burn coverage in its provenance.
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

- **Issue A — guard primitive + telemetry kit** (the skill work: the ledger entry, the spine verbs, the tiered contracts, the decision semantics, the poll loop). Auth contract decided ("Auth"); built and tested against a **manually-issued Viewer SA token**. This is the workstream's deliverable and what proves the hypothesis.
- **Issue B — SA-token provisioning automation** (sibling, owned by platform-engineer): automate the decided credential via the `GrafanaServiceAccount` CR → Secrets Manager → CSI + rotation, gated on OQ1/OQ2/OQ3. Can proceed in parallel; if it slips on OQ3 it does not block Issue A (which runs on the manual token).

### The federation read path (Issue B — a one-way door)

The auth *mechanism* is decided ("Auth"); this slice is the *read path it points at* plus the provisioning automation.

- **Read path:** Thanos **Query Frontend** (`thanos-query-frontend.monitoring.svc:9090`) — the fleet-global view Grafana already trusts — reached via the Grafana datasource-proxy.
- **Provisioning + rotation:** the Viewer SA token via the `GrafanaServiceAccount` CR (operator/GitOps) → AWS Secrets Manager → CSI, with a finite expiry (MVP cut-first: short manual expiry + a calendar reminder; un-defer automated rotation when a second consumer shares the path).
- **Read-only is structural — confirmed.** Viewer role at Grafana; Thanos Query exposes **no admin path** (`--query.enable-admin` absent → default off; receive/ruler disabled in the chart). Three independent legs hold.
- **Datasource blast-radius — a real Issue-B decision (OQ1, resolved).** Grafana is **OSS** (no per-datasource token scoping) and fronts **not just metrics** — four Loki tenants (full log content, incl. the pacific-1 mainnet co-tenant), CloudWatch, and Pyroscope. An OSS Viewer token reads *all* of them. **Recommendation: mint the guard's SA in a separate Grafana org that fronts only `prometheus-prod`** (structurally metrics-only; OSS supports multi-org, the operator's `instanceSelector` targets it). Cut-first fallback: accept the all-datasources floor with a short-expiry per-consumer SA as the compensating control — but written as an *explicit accepted risk* in Issue B, un-deferred on a security objection or a second token consumer. The operator is `namespaceScope: true` (`monitoring`), so the SA + token Secret live there and Issue B plumbs them out (Secrets Manager / cross-ns CSI).
- **Coupling to flag:** routing through Grafana makes **guard availability bounded by Grafana availability** — a Grafana outage fails all guards closed (correct direction, but a new coupling the in-cluster-direct path would avoid).
- **One-way door:** **publicly exposing Thanos's query API is out of scope and requires explicit human approval to even propose** — it contradicts the deliberate "no public Thanos gRPC" stance. Route external agents through Grafana for MVP.

## Alternatives considered

- **Generic arbitrary-PromQL guard** as the primary surface — rejected as the default (wide, hard-to-reason; same boundary reasoning as gov-ops); deferred entirely from MVP (not even an escape hatch).
- **`grafana/mcp-grafana` as the read adapter** — rejected for gated reads (OQ-warnings): it discards the Thanos `warnings` array and exposes no `partial_response` param, so it cannot satisfy the partial-response-safe contract. The datasource-proxy HTTP call (the `query-grafana.py` pattern) is chosen instead — warnings-preserving and already proven. (A custom Thanos MCP remains rejected; the proxy call is the standard in-repo pattern, not a new client.)
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

## Open questions — RESOLVED (investigation, post-approval)

All build-blockers resolved before Issue A starts; Issue-B items resolved or scoped for the platform sibling.

- **OQ-warnings (was the blocker) — RESOLVED, changed the design.** `grafana/mcp-grafana` discards the Thanos `warnings` array and exposes no `partial_response` param (verified in `main`); `pab1it0` does too. ⇒ **the read adapter is the raw datasource-proxy call, not the MCP server** (see "Auth"). Warnings + `partial_response=false` are both available on the proxy path.
- **OQ3+OQ4 — RESOLVED.** No deployed CronJob/Actions runner exists; the validate-release precedent runs as a **Claude Code background agent from an operator session**, external, via the Grafana datasource-proxy (`query-grafana.py:56-84`, `SKILL.md:59`). ⇒ **external-via-proxy is the proven MVP path** (same as the guard's agent context); in-cluster-direct stays deferred. Reuse: ~50–60% of the mechanical loop (proxy call, preflight, stats); the six correctness contracts are net-new.
- **OQ1 (datasource blast radius) — RESOLVED, Issue-B decision.** OSS Grafana fronts Loki/CloudWatch/Pyroscope too → separate-metrics-only-org recommendation (see "federation read path"). Does not block Issue A.
- **OQ2 (`GrafanaServiceAccount` CR) — RESOLVED, green.** Operator **v5.22.2** supports the CR (introduced v5.12). GitOps minting viable; out-of-band token is the dev fallback.
- **OQ-rules — RESOLVED.** `slo_*_3h` use a `[2h:]` window (cite the expr, not the name); the `_3h` tier is evaluated by **Prometheus ~30s**, not ThanosRuler — the 5m budget applies only to `≥3d -longwindow` series (folded into the per-source-freshness contract). *Still worth a courtesy `/issue` to observability-platform-engineer to fix the `_3h` mislabel at the source.*
- **OQ5 (auto-abort scope) — DECIDED.** MVP ships surface-only; auto-abort un-defer trigger is **measurable**: a false-trip rate below tolerance over N real procedures AND a tested idempotent reversible rollback. Owner: bdchatham.

## References

- `design/research/telemetry-signal-kit-tooling.md` (the build-vs-reuse research, this workstream).
- The `/workstream` + checkpoint primitive (PLT-500); the `/research` method (PLT-501).
- `grafana/mcp-grafana`; SRE Workbook ch.5 (MWMBR); Flagger `Canary`/`MetricTemplate` (Flux-native; schema prior art); Argo Rollouts `AnalysisTemplate` (equivalent).
- Production precedent: `.claude/skills/validate-release/scripts/query-grafana.py` (datasource-proxy + bearer-token auth, today).
- Live stack: `clusters/prod/monitoring/{thanos-query,grafana,grafana-operator,recording-rules-chaos-suite}.yaml`; `alerts/protocol/recording-{validator,chain}-slos.yaml`.
