---
name: systems
category: code-quality
model: claude-opus-4-8
description: "Use when reviewing or designing code/architecture for systems-level quality — reliability, performance, observability, safety, API durability — 'systems review', '/systems', 'will this hold under load', 'is this resilient/observable', 'is this API backward-compatible'. A citable standards corpus (Google SRE, AWS Builder's Library, OTel semconv, TigerBeetle TIGER STYLE, Google AIP, …) the systems-engineer agent hooks into; findings ranked by consequence-under-load, each cited. Anti-triggers: NOT for language-idiom conformance (use /idiomatic); NOT for line-level correctness bugs (use /code-review); NOT for cross-component interface consistency (use /cross-review); NOT for operating running services — SLOs/alerts/runbooks/incidents (sre-engineer), manifests/runtimes (platform-engineer), telemetry SDK/backend (the observability agents). Reviews how code behaves on the machine and over time; does not run the platform."
---

# Systems

Review and design code so it **behaves well on the machine and over time** — reliable under failure, performant under load, observable when it breaks, safe by construction, and durable at its interfaces. A *reference/technique* skill with a discipline spine. It is the operating manual for the `systems-engineer` agent and is also directly invocable (`/systems-review <target>`).

## Why this skill exists

A capable model already knows most systems principles. The skill's job is **not** a textbook — it's the **citable corpus** (the specific authority + the specific cue the model can't reliably reproduce from memory) plus the **discipline to rank findings by consequence under load** and not cry wolf. The standards corpus is grounded in the open canon (see `references/sources.md`) and stays copyright-clean: our-own-words checklists that point at sources, never reproduce them.

## Guardrails

Refusal conditions — they hold under time pressure and a checklist-completion urge:

1. **No consequence → no finding** (Rule 1). Never flag a rule that doesn't bite for this system's load/criticality.
2. **Cite every finding; stay copyright-clean.** An authority and/or repo rule per finding; never reproduce or closely-reword reserved source text.
3. **Suggest-only.** Never rewrite the author's files — produce findings the human/calling agent applies.
4. **Don't duplicate the idiom or ops lens** (Rule 3). Idiom → `/idiomatic`; operating the live system → the ops agents.
5. **One-way doors are flagged, not asserted.** A change to a published API/wire format goes to human approval.

## When to use / when not

| Use `/systems` for… | Use instead… |
|---|---|
| Will this hold under load / failure? (reliability, perf, observability, safety, API durability) | — |
| A finding that names a consequence under load + cites an authority | — |
| Does the code read native to the language + package patterns? | `/idiomatic` |
| Line-level correctness bugs (races, nil derefs, logic) | `/code-review` |
| Cross-component interface/boundary consistency | `/cross-review` |
| Operating the running system — SLOs, alerts, runbooks, incidents | `sre-engineer` |
| Manifests, runtimes, cloud auth, GitOps | `platform-engineer` |
| Telemetry SDK wiring / backend | `opentelemetry-expert` / `observability-platform-engineer` |

Idiom ⊂ systems quality: `/idiomatic` answers "does it read native"; `/systems` answers "does it behave well on the machine and over time." Run the idiom pass first, then this on top.

## The method

1. **Identify the work's systems surface** and load the relevant reference(s): `reliability` (remote calls, retries, queues, failure handling), `observability` (does it expose how it's doing), `performance` (hot paths, concurrency, latency), `safety-quality` (invariants, bounds, untrusted input), `api-design` (a published/wire interface).
2. **Apply on top of the `/idiomatic` pass** — don't re-flag idiom (that's the other lens).
3. **Rank every finding by consequence under load** (severity model in each reference): correctness/safety > consequence-under-load > advisory.
4. **Cite every finding** (an authority from `sources.md` and/or a repo rule) and suggest the fix. Suggest-only — never rewrite the author's files.

## The discipline spine

Three non-negotiable rules.

### Rule 1 — Rank by consequence-under-load; don't flag rules that don't bite here
Every finding names the failure mode it prevents *for this system's actual load and criticality* (a retry storm, a cardinality explosion, an irreversible API break). A rule with no plausible consequence for *this* target is trivia, not a finding. On a system that's already sound, the answer is *"behaves well — no findings."* Demanding circuit breakers on a one-shot batch job, p99 hedging on a cron, or static allocation on a control-plane CRD is crying wolf — and a reviewer that cries wolf gets muted.

### Rule 2 — Cite every finding; stay copyright-clean
Every finding names a canonical authority (Google SRE, AWS Builder's Library, OTel semconv, TIGER STYLE, Google AIP…) and/or a repo rule. No naked "this won't scale." The citation is a link, **never reproduced or closely-reworded reserved text** — half the sources are reserved (cite-only); copyright discipline is a spine rule here, not a footnote. An irreversible change (API/wire format) is **flagged for human approval**, not asserted.

### Rule 3 — Don't duplicate the idiom or the ops lens
"Reads native" → `/idiomatic`. "Operate/run the live system" (postmortems, on-call, runtime cluster tuning) → the ops agents. `/systems` reviews how the *code and architecture* behave on the machine and over time — nothing else.

### Rationalization table

| The pressure says… | The rule is… |
|---|---|
| "Add a circuit breaker / hedge / breaker here too." (on a batch job / cron) | Rule 1. Name the consequence under *this* load. No consequence → not a finding. |
| "This is clean code, but let me list some systems nits to be thorough." | Rule 1. On a sound system, say "behaves well — no findings." Padding gets the reviewer muted. |
| "I'll quote the SRE Book / the article to make the point." | Rule 2. Cite-and-link; never reproduce reserved text. Summarize the idea in your own words. |
| "The function's a bit long / the name's off." | Rule 3. That's `/idiomatic`. Don't duplicate the idiom lens. |
| "Let me also tune the alert / write the runbook." | Rule 3. That's `sre-engineer`. This skill reviews the code, not the running system. |
| "I'll just rename the API field to fix it." | Rule 2. A wire-format change is a one-way door — flag for human approval, don't assert. |

## Output format

```
## Systems review: <target>
Surface: <which references loaded> · Idiom pass: <ran /idiomatic? yes/no>

### Findings (ranked by consequence under load)
- [correctness | consequence-under-load | advisory] <finding>. Consequence: <what breaks under load>. Basis: <authority / repo rule>. Fix: <suggestion>.

### Deliberately not flagging (vetted)
- <thing> — <why it's sound / why the rule doesn't bite here>
```

## Theme index

| Theme | Scope | Reference |
|---|---|---|
| Reliability & resilience | timeouts, bounded retries+jitter, idempotency, load shedding, breakers/bulkheads, degrade, liveness/readiness, blast radius | `references/reliability.md` |
| Observability-by-design | golden signals / RED / USE, wide events, cardinality, semconv, trace propagation, honest health | `references/observability.md` |
| Performance & Linux | measure-first, latency hierarchy, tail/p99, bounded concurrency, batching, tooling | `references/performance.md` |
| Safety & quality | assert-in-pairs, bound everything, static allocation, validate input (systems-safety slice only) | `references/safety-quality.md` |
| API & interface design | backward-compat, idempotency, cursor pagination, structured errors, versioning | `references/api-design.md` |
| Citations & licensing | the source corpus + openness | `references/sources.md` |

## How the systems-engineer agent hooks in

The `systems-engineer` persona's first step loads the relevant `/systems` reference(s) for the work in hand and applies the systems lens on top of the `/idiomatic` pass. The two skills compose: idiom = reads native; systems = behaves well on the machine and over time.

## Halt Conditions

Stop and ask / escalate rather than proceeding when:

- **No target artifact** to review (the code/design can't be read) — halt and ask; never review from memory.
- **The work is really another lens** — idiom (`/idiomatic`), line-level correctness (`/code-review`), or operating the live system (the ops agents) — redirect rather than stretch this skill over it.
- **A finding would set a one-way door** (API/wire-format change) — stop and escalate to a human instead of asserting the fix.

## What this skill defers

Culture/process corpus (DORA, postmortems, incident command) — not a reviewable code artifact; stays with `sre-engineer`. A pluggable `_TEMPLATE` — the theme set is closed (not an open set like languages); un-defer if a 6th theme is genuinely needed by a second author. `/coral` + `/council` dispatch wiring — un-defer when standalone is validated. Per-theme specialist-agent dispatch for deep calls — un-defer when a finding needs a reasoning persona the checklist can't carry.
