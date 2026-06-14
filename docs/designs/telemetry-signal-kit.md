# Design: telemetry signal-kit — metric-driven guards for agentic workstreams

**Status:** Draft (design phase of a `/workstream`; design-approval checkpoint pending)
**Date:** 2026-06-14
**Issue:** (telemetry signal-kit workstream — sibling to the PLT-500 framework family)
**Authors:** bdchatham, Claude (Coral)

## Background

The `workstream` skill (PLT-500) gives an agentic workstream **human** checkpoints. The gap this design closes: a **machine gate driven by a live metric signal** — the agent as the tireless second watcher staring at the dashboards during a high-risk live procedure (a migration cutover, a deploy), while the human still owns the irreversible go/no-go. The pattern is Amazon's "bake a change against a known aggregate alarm for N minutes before the pipeline proceeds, with immediate rollback if it trips," generalized for agents and made provenance-bearing (every reading is a cited, re-runnable query).

Build-vs-reuse was sourced first (via the `/research` method, 4 blind sweeps + adversarial verify) — see `design/research/telemetry-signal-kit-tooling.md`. Headline: **reuse on every layer** (official `grafana/mcp-grafana`; existing Thanos Query Frontend + recording rules; SRE multi-burn-rate + Argo-Rollouts/Flagger decision schema). This design turns those decisions into the guard primitive, its kit, and the federation slice.

This is the **first (exemplar) kit** of a broader architecture aligned with Brandon: an agnostic **signal-kit spine** + pluggable **signal kits** (sources) × **modes** (gate / measure / coordinate) — the same shape that made the `/idiomatic` language kits work. Telemetry/gate is the "Go pack" that proves the spine before the indexed-events (coordinate) and perf/eBPF (measure) kits.

## Goals

1. A **`guard`** primitive: a non-human, signal-driven gate that extends the `/workstream` checkpoint ledger — declared up front, fail-closed, provenance-bearing, and it surfaces/halts before a risky step proceeds if the signal isn't healthy.
2. The **telemetry signal kit** (gate mode, MVP): the read adapter (`grafana/mcp-grafana`, read-only), the citable query vocabulary (existing `chaos_suite:*` / `slo_*` recording rules + SRE multi-burn-rate), and the gate-decision semantics (Argo-Rollouts/Flagger-style `interval`/`count`/`failureLimit` + two-window).
3. The **metric-federation slice**: a Grafana service-account Viewer token (the non-interactive answer to the headless-auth crux) provisioned via the existing Grafana Operator — agent read access, structurally read-only.
4. The **signal-kit spine** stated once, kit-agnostic, so indexed-events (coordinate) and perf (measure) kits drop in later.
5. Encode the non-negotiable correctness contracts the research's verify pass surfaced (partial-response-safe, fail-closed, freshness, two-window, baseline-at-gate-start, cite-the-query).

## Non-goals

- **Not** the measure or coordinate modes, nor the perf/eBPF or blockchain-indexed-events kits (later kits; spine is designed to admit them).
- **Not** an in-cluster direct-to-Thanos read path + `prom-label-proxy` tenancy (*deferred — when an agent runs untrusted/model-authored queries needing per-call audit, or must be denied a tenant like pacific-1*).
- **Not** a generic "arbitrary PromQL" guard as the primary surface (domain guards over a generic hole — same RBAC-boundary reasoning as the gov-ops content-specific decision; generic is an explicit escape hatch).
- **Not** an Alertmanager grouped/silenced-alert read path (*deferred — when a guard must watch an Alertmanager-grouped aggregate alarm specifically*; per-rule firing state via grafana-mcp covers MVP).
- **Not** building anything before design-approval.

## Design

### The `guard` primitive (extends the workstream ledger)

The `/workstream` ledger today holds `checkpoint`s (human gates). Add a second entry kind, `guard` (a signal gate):

```
- guard:    <name>
  signal:   <a CITED live query — a recording-rule name / PromQL expr / a firing-alert rule>
  healthy:  <condition — baseline-relative (preferred) or absolute threshold>
  when:     pre-step  (gate before the risky action)
          | soak      (poll for N minutes after the action, vs the gate-start baseline)
          | continuous(watch across the whole phase)
  on_trip:  surface + route to a declared rollback checkpoint
          | (auto-abort ONLY if the step is reversible — see the surface-vs-act decision)
```

A high-risk step (e.g. a validator node-shape cutover) declares **both** a `guard` (the metric watch) and a human `checkpoint` (the go/no-go). The agent: capture baseline → human go-ahead → execute → soak-watch the guard → trip ⇒ halt before the next step + surface + route to rollback.

### The signal-kit spine (kit-agnostic)

Every signal kit, regardless of source, implements: **declare → fetch (fail-closed) → evaluate (via a cited query) → act (gate/surface) → capture provenance.** The pluggable kit supplies a **read adapter**, a **citable query/event vocabulary**, and the **domain semantics**; the spine is constant. (Telemetry is the first kit; indexed-events and perf are the same spine with different adapters/vocabularies.)

### The telemetry kit (MVP)

- **Read adapter:** `grafana/mcp-grafana` (official, Apache-2.0) — PromQL instant/range + per-rule firing-alert state; Thanos-transparent via the Grafana datasource. No custom Thanos MCP.
- **Citable query vocabulary:** cite the *existing* recording rules (`chaos_suite:block_time_p95:rate2m`, `:tps:rate1m`, `:block_height_delta:rate2m`, the `slo_*_{5m..30d}` ladders) and SRE **multi-window multi-burn-rate** alert exprs. The guard names a rule; it does **not** re-derive histograms inline. The `chaos_suite` validation agent's poll loop is the implementation template (~60–70% exists).
- **Decision semantics:** mirror **Argo Rollouts `AnalysisTemplate` / Flagger** (`interval` / `count` / `failureLimit` / `successCondition` / `thresholdRange`) + the MWMBR two-window AND-gate. Threshold/rate/quantile/window-fraction are PromQL-native (the `sum_over_time((expr > bool THRESH)[w:])/count` "fraction-of-window-healthy" primitive already exists); baseline-relative deltas across the cutover boundary are the one genuinely agent-side computation — pushed into a pinned two-window query, not in-context arithmetic.

### The federation slice (the infra callout — a one-way door)

- **Read path:** Thanos **Query Frontend** (`thanos-query-frontend.monitoring.svc:9090`) — the fleet-global view Grafana already trusts.
- **Auth:** a **Grafana service-account Viewer token** → datasource-proxy. This is the non-interactive answer to the headless crux (Grafana's Google OAuth is interactive-only). Provisioned via a `GrafanaServiceAccount` CR through the already-deployed Grafana Operator; token → AWS Secrets Manager → CSI. Read-only is structural (Viewer role; Thanos query API is write-free; receive/ruler disabled).
- **One-way door:** external agents have no private Thanos path today; routing them through Grafana is the MVP. **Publicly exposing Thanos's query API is flagged for human approval and recommended against** (contradicts the deliberate "no public Thanos gRPC" stance).

### Non-negotiable correctness contracts (from the research verify pass)

- **Partial-response-safe:** force `partial_response=false` / inspect the `warnings` array — the deployed Thanos has partial-response ON, so a naive guard could **gate-PASS on a half-fleet read.** Highest-priority.
- **Fail-closed:** stale data, unreachable endpoint, auth/query error ⇒ "cannot confirm healthy" = degraded/abort, **never** PASS (same spine as the workstream checkpoint + gov-ops).
- **Freshness + min-samples** precondition before any verdict; **two-window / N-consecutive-breach**, never a single threshold; **baseline captured at gate-start** (snapshot or `offset 1w`), not an absolute; **all math in PromQL/recording rules**; **cite the re-runnable query** with every reading (provenance = the research/lineage discipline).

### Surface-vs-act decision

`on_trip` is **surface-and-wait by default**; **auto-abort is allowed only for a step the workstream declares reversible** (Amazon's immediate-rollback is correct for reversible deploys). A one-way-door step always surface-and-waits — the guard detects, the human owns the irreversible call (the checkpoint spine reused).

### Open architectural choice: guard-as-`/workstream`-extension vs its own skill

Recommend the `guard` primitive lives **in the `/workstream` skill** (a second ledger entry kind), with the telemetry kit as a reference (`references/signal-kit-telemetry.md`) — the spine is workstream's, the kit is data, mirroring idiomatic's pack model. A standalone `/guard` skill is deferred unless guards are needed outside a workstream.

## Alternatives considered

- **Generic arbitrary-PromQL guard** as the primary surface — rejected as the default (wide, hard-to-reason capability; same boundary reasoning as gov-ops content-specific tasks); kept as an explicit escape hatch behind domain guards.
- **Custom Thanos MCP / hand-rolled HTTP client** — rejected; `grafana/mcp-grafana` is official, current, and Thanos-transparent.
- **In-agent metric math / a "math MCP"** — rejected; in-context arithmetic is the #1 failure mode. Math lives in PromQL/recording rules; the agent reads scalars.
- **Kayenta-style statistical ACA** — rejected as a dependency (archived); borrow the distribution-comparison idea only if simple thresholds prove flappy.
- **In-cluster direct-to-Thanos + prom-label-proxy** — deferred (see Non-goals).

## Trade-offs

- **Grafana-proxy read path** adds a hop vs direct Thanos — accepted for MVP (it's the only authenticated, externally-reachable, non-interactive surface; in-cluster direct is the later optimization).
- **Domain guards over generic** — less flexible day one, but safer and self-describing; the escape hatch covers the long tail.
- **Surface-default (not auto-abort)** — slightly less hands-off than Amazon's auto-rollback, but correct for one-way doors; reversible-step auto-abort recovers most of the benefit.
- **Dogfooding the workstream to design this** — slower than ad-hoc, but validates the checkpoint primitive and is the standard.

## Open questions (to resolve at / before design-approval)

- **OQ1 (Grafana edition)** — per-datasource SA scoping is an Enterprise/RBAC feature; if OSS, the floor is Viewer-across-all-datasources (still read-only). Owner: platform-engineer. *Verify before the federation slice is built.*
- **OQ2 (`GrafanaServiceAccount` CR)** — confirm the deployed Grafana Operator version supports the CR. Owner: platform-engineer.
- **OQ3 (where headless/cron agents run)** — in-cluster vs external decides the read path (direct Thanos vs Grafana proxy). Owner: bdchatham. *The single highest-leverage unknown.*
- **OQ4 (chaos-suite agent reuse)** — pull the `platform-release-manager`/chaos-suite agent source; it likely implements 60–70% of the poll loop. Owner: observability-platform-engineer. *Could collapse the implementation scope.*
- **OQ5 (auto-abort scope)** — does MVP ship surface-only, or surface + auto-abort-reversible? Recommend surface-only for the first kit, add reversible auto-abort once the gate is trusted. Owner: bdchatham.

## References

- `design/research/telemetry-signal-kit-tooling.md` (the build-vs-reuse research, this workstream).
- The `/workstream` + checkpoint primitive (PLT-500); the `/research` method (PLT-501).
- `grafana/mcp-grafana`; SRE Workbook ch.5 (MWMBR); Argo Rollouts AnalysisTemplate; Flagger.
- Live stack: `clusters/prod/monitoring/{thanos-query,grafana,grafana-operator,recording-rules-chaos-suite}.yaml`; `recording-{validator,chain}-slos.yaml`.
