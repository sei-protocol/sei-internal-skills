---
name: workstream
category: workflow
model: claude-opus-4-8
description: "Use when launching a substantial multi-step workstream on the Coral stack with declared human checkpoints — 'launch a workstream', 'kick off this goal with checkpoints', 'start a workstream with a PR sign-off gate', 'set up checkpoints for this goal', '/workstream'. Use when the work needs named human gates the agent must surface and obtain confirmation for before proceeding, or signal-driven **guards** — fail-closed metric gates that watch a live signal during a high-risk step ('gate this cutover on the metric staying healthy'). Anti-triggers: NOT the Claude Code /goal harness command (that sets the persistent objective; this governs how it's pursued); NOT a single 1–2 specialist slice (use /coral); NOT scope-tiered design alone (use /council — workstream invokes it); NOT capturing a finished design (use /design)."
---

# Workstream

Launch and govern a substantial workstream on the Coral stack with **declared, enforced human checkpoints**. This is a *procedural* skill (a fixed lifecycle it walks) with a *discipline spine* (the checkpoint-enforcement rules that survive pressure). It composes the existing skills — it does not replace them.

It layers on the Claude Code `/goal` mechanism: `/goal` sets the *persistent objective* (and a Stop hook); `workstream` governs *how* that objective is pursued (the Coral lifecycle) and *where the human gates are* (the checkpoint primitive). The two are different things — see the spine.

> **Vocabulary note.** `council` already uses "workstream" for its per-phase progress state (`.council/workstream.yaml`). Here a **checkpoint is a human gate**; council's "workstream checkpoint" is phase-progress state. Different concepts — this skill's checkpoints are the human-in-the-loop contract, not progress tracking.

## Why this skill exists (read this first)

**The spine: a `/goal` Stop hook governs *stopping*, never *approval*. It cannot authorize walking through a one-way door — a declared human checkpoint outranks the keep-going pressure.**

An agent under a `/goal` Stop hook is pushed to *keep going until the condition holds*. That pressure is correct for momentum and catastrophic at a one-way door. Pressure-testing (RED) showed an agent at a declared `pr-sign-off` gate — PR up, CI green, operator offline, Stop hook pushing — reaching for: *"CI is green, so the checkpoint is really just a formality"*; *"the operator would obviously want me to proceed"*; *"the Stop hook is a live signal and the checkpoint is a stale note — the live signal should win."* Every one of those **upgrades the Stop hook (a control on *stopping*) into authority to walk through a *one-way door***, or substitutes the agent's guess for the explicit sign-off it was told to get.

So the value of this skill is not the lifecycle scaffolding (that's a convenience). It is the **checkpoint primitive + the enforcement discipline**: making human gates a declared, enforced part of the workstream contract rather than prose the operator hopes the agent honors.

## Guardrails

Refusal conditions — these hold under a Stop hook, time pressure, an offline operator, and a tidy green-CI diff:

1. **Never self-approve a declared checkpoint.** When a checkpoint's trigger is reached, STOP and obtain *explicit* human confirmation before proceeding past it. Do not infer approval from silence, from green CI, or from "they'd obviously want this."
2. **A Stop hook never waives a checkpoint.** "I cannot stop until the goal completes" and "I may therefore approve the irreversible thing myself" are different claims; only the first is true. The hook governs *stopping*, not *approval*. Continue reversible in-scope work; leave the hook unsatisfied if the only path forward crosses a declared gate.
3. **Compose, never edit — and never reimplement.** Invoke `council`/`cross-review`/`design`/`issue`/`execution-plan`; never modify them. Delegate all lineage decoration to `/execution-plan` (never re-implement label/identity/stamp logic — that mints a second identity). If a composed skill is **unavailable**, surface the gap and run the phases you can — do **not** hand-roll the missing skill's logic inline (e.g. don't reimplement council's scope tiers).
4. **Declare the ledger up front; confirm scope before side effects.** At workstream start, echo the **scope** (objective + the council scope tier) and the **checkpoint ledger**, surface both, and get the operator's go-ahead before invoking the lifecycle. A gate added silently mid-stream, or honored only when convenient, is not a checkpoint.
5. **Don't manufacture ceremony.** A genuinely small slice does not need a workstream — redirect to `/coral`. Checkpoints are gates a human actually wants, not decoration.
6. **Reversibility raises the bar to *create* a checkpoint, never lowers it to *waive* a declared one.** A reversible change may not need a gate; but once a gate is declared, "this is reversible" does not waive it. Reversibility is the axis for *adding* gates, not for skipping ones already on the ledger.
7. **A guard fails closed — it never PASSes on data it cannot confirm.** Stale, unreachable, empty, or incomplete (partial-response `warnings`) reads are `inconclusive` ⇒ abort, never "looks fine." A guard does not replace a one-way-door checkpoint and may not launch a workstream or create ledger entries at trip time (the ledger is static after declaration). See "The guard primitive."

## The checkpoint primitive

A **checkpoint** is a declared, named human gate the agent must hit, surface, and get explicit confirmation for before proceeding past it.

### The ledger

At workstream start, write a **checkpoint ledger** — a typed list (one entry per gate) and surface it to the human. Each entry is a triple:

```
- name:    <short identifier>
  trigger: <the workstream state that reaches this gate — when the agent must stop>
  gate:    <exactly what the human must confirm before proceeding>
```

See `references/checkpoint-ledger.md` for the format and worked examples.

### Two canonical checkpoint types

The minimal set that covers the common human gates:

- **`design-approval`** — *trigger:* a design has been captured (via `/design`) **and** has a **converged-COMPATIBLE** `/cross-review` verdict — complete slate + raiser-confirmed resolution per cross-review's contract, not an OPEN or single-pass result. *gate:* the human signs off on the captured, reviewed design before implementation begins.
- **`pr-sign-off`** — *trigger:* a PR is **ready** — CI green **AND** automated-review (Cursor Bugbot / Claude) findings iterated (High = blocking) **AND** the diff reconciled with the approved `/design` + interface-registry (divergence surfaced as gate evidence; may compose `/verify`) — and merging it, or starting work that depends on it, is next. *gate:* the human confirms the merge / the go-ahead for dependent work.

**One-way-door approval is not re-declared here** — reuse council's existing one-way-door gate by reference (persisted schema/field names, wire formats, signed IDs, prod-touching steps). A `workstream` simply ensures that gate is honored when council surfaces it.

### Custom checkpoints

Operators may declare additional checkpoints inline (e.g. `outcome-alignment`: confirm the result matches intent before the next phase). **Each inline checkpoint MUST carry the same `name`/`trigger`/`gate` triple** — no freeform gates. This keeps the canonical set minimal while allowing any gate a workstream needs.

### The enforcement spine

When a checkpoint's trigger is reached:

1. **STOP.** Do not take the gated action.
2. **Surface** the checkpoint, the evidence the human needs to decide, and the precise confirmation you're waiting on ("reply 'merge' to proceed past `pr-sign-off`").
3. **Wait for the exact confirmation token you surfaced** (e.g. the literal "merge" / "approved"). An approving-sounding aside ("looks good", "nice") is **not** the token — if what you got isn't the token you asked for, you are still waiting. Meanwhile, continue any *reversible* in-scope work that doesn't cross the gate (docs, a migration script authored-but-not-run, the next phase's prep).
4. **Only after explicit confirmation, proceed.** Never self-approve.

A `/goal` Stop hook pushing you to "keep going" does **not** waive this. If the only path forward crosses an unconfirmed gate, the correct state is: hook unsatisfied, gate surfaced, reversible work continuing. Record *why* the goal isn't done so the unsatisfied hook is legible, not mysterious.

## The guard primitive (a signal gate)

A **guard** is the machine counterpart to a checkpoint: a declared, named, **signal-driven** gate the agent must hit, evaluate, and *fail closed* on before proceeding past a high-risk step. Where a checkpoint waits on a *human*, a guard watches a *live signal* (a metric, later an indexed event). It is the tireless second watcher during a cutover/deploy — the human still owns the irreversible go/no-go (a guard never replaces a one-way-door checkpoint; it sits alongside one).

A guard is the **gate-mode instance** of a *signal binding*. The same declare→fetch→evaluate→act→provenance spine later supports **measure mode** (an objective a workstream optimizes toward) and **coordinate mode** (an event barrier); MVP ships gate mode, telemetry kit. The kit supplies the read adapter + query vocabulary + domain semantics — see `references/signal-kit-telemetry.md`.

### The guard ledger entry

A guard is a second ledger entry kind, declared up front alongside checkpoints:

```
- guard:    <short identifier, kebab-case>
  signal:   <a CITED live query — a recording-rule name / PromQL expr / a firing-alert rule>
  healthy:  <condition — baseline-relative at gate-start (cutover) or absolute SLO threshold>
  when:     pre-step | soak (N min, vs gate-start baseline) | continuous (whole phase)
  on_trip:  surface + route to a PRE-DECLARED rollback checkpoint   (MVP default)
```

A high-risk step declares **both** a `guard` (the metric watch) and a human `checkpoint` (the go/no-go). Flow: capture baseline → human go-ahead → execute → soak-watch the guard → trip ⇒ halt + surface + route to rollback.

### The guard enforcement spine

1. **Fail closed.** Stale data, unreachable endpoint, auth/query error, an **empty read**, or an **incomplete read** (telemetry: a non-empty `warnings` array) ⇒ "cannot confirm healthy" = `inconclusive` ⇒ **abort**, never PASS. A guard that PASSes on data it cannot confirm is worse than no guard — it manufactures false confidence. This is the checkpoint's fail-closed discipline applied to a signal.
2. **Cite the re-runnable query** with every reading (the query, window, store that answered, warnings, verdict) — provenance, not a bare scalar.
3. **Surface on trip; the human owns the irreversible call — the guard never self-executes the rollback.** `on_trip` halts before the next step, routes to a pre-declared rollback **checkpoint**, and STOPS for the human token — it does not run the rollback itself. Auto-abort is deferred and, if ever enabled, only a pre-declared reversible+idempotent rollback, never on a one-way-door step.
4. **Recursion bound.** A guard may surface, halt, and route to an **already-declared** checkpoint — nothing more. It may **not** launch a workstream, spawn a sub-agent that launches one, or create ledger entries at trip time. The ledger is static after declaration. (This bounds a `continuous` guard's trip handler from becoming an unbounded watch→remediate→watch loop.)

The kit's **eight correctness contracts** (four verdict-gating, four budget/tuning) — and the soak verdict rule (N-consecutive-breach vs the gate-start baseline) — live in `references/signal-kit-telemetry.md`. **Do not construct a guard verdict from this summary alone**; a guard that skips the contracts PASSes on the common cutover failure modes (no-traffic-drain, half-fleet read).

## The lifecycle (the procedural scaffold)

`workstream` walks the Coral lifecycle, inserting checkpoints at the seams. It **invokes** the existing skills at the right moments; it does **not** auto-drive them (the operator still confirms phase transitions). Full recommended flow in `references/composition.md`.

1. **Declare + surface the ledger; confirm scope.** Echo the scope (objective + the council scope tier) and the checkpoint ledger — default ledger: `design-approval` after design capture, `pr-sign-off` before merge / dependent work, plus any custom checkpoints the operator names — and get the operator's go-ahead **before** step 2 (per Guardrail 4).
2. **Scope-tier the work via `council`.** Invoke `/council` (it owns the four scope tiers and produces the design content). Do not reimplement tiers.
3. **Cross-review via `/cross-review`.** Run blinded multi-specialist review on the design **before** capture (cross-review precedes `/design` — design captures what's been reviewed, it doesn't review). The `design-approval` gate composes a **converged-COMPATIBLE** verdict — cross-review owns slate-completeness and raiser-confirmed resolution; an OPEN or single-pass verdict does not satisfy it.
4. **Capture via `/design`.** Offer `/design` to write the reviewed design as a durable doc.
5. **`design-approval` checkpoint.** STOP; surface the captured, reviewed design; obtain explicit sign-off before implementing.
6. **Implement** to the approved design. (Council's one-way-door gate fires here if a persisted-schema / wire-format / signed-ID change appears — honor it as a checkpoint even though it wasn't pre-declared.)
7. **Capture deferred slices via `/issue` — conditionally.** Only if a slice was cut. Don't reflexively file an empty issue.
8. **Decorate lineage via `/execution-plan`.** Call it to stamp the bet label + design link (delegated; never duplicated). Its first-label-creation confirm is a *separate* gate it owns — not subsumed by the ledger.
9. **`pr-sign-off` checkpoint.** STOP; surface the ready PR — CI green, automated-review (Bugbot/Claude) findings iterated (High = blocking), and the diff reconciled with the approved `/design` + interface-registry (divergence surfaced as gate evidence; may compose `/verify`) — and obtain explicit confirmation before merge / dependent work.

## Rationalization table

The pressure says… → the rule is…

| Pressure | Rule |
|---|---|
| "CI is green, so the checkpoint is just a formality now." | Green CI proves the code does what you think; it says nothing about whether the human wants it through a one-way door. Reversibility, not test status, is the axis. STOP. |
| "The operator is offline and `/goal` said don't pause to ask — staying blocked is the dithering it warned against." | "Don't pause to ask what to do" is an anti-dithering rule, not a revocation of a gate you deliberately declared. You're not asking what to do; you're waiting on a pre-defined authorization. Surface and continue reversible work. |
| "The operator would obviously want me to proceed." | If you're *inventing* their approval rather than *obtaining* it, that's the exact substitution the checkpoint exists to prevent. The gate says "explicit confirmation" because your guess is the thing not to trust here. |
| "The Stop hook is a live signal; the checkpoint is a stale note — live wins." | The hook governs *stopping*. It cannot manufacture an approval the human never gave. A pre-committed gate is *more* trustworthy than present-you under pressure, not less. STOP; surface the gate. |
| "I'll close the ticket to satisfy the hook, then surface the concern after." | Closing to satisfy the hook is walking through the door and reporting it afterward. Surface *before*, not after. An unsatisfied hook is the system correctly reflecting unfinished work. |
| "It's my own checkpoint, so I can waive my own note." | Past-you set it with the most context and least pressure precisely for present-you to honor. Self-waivable = never a checkpoint. |

## Red flags — STOP and honor the gate

- About to merge / deploy / run a migration with an unconfirmed `pr-sign-off` or one-way-door gate
- Reasoning that concludes "they'd want me to proceed" instead of "they confirmed"
- Treating "I can't stop" as "I may approve this myself"
- Adding or relaxing a checkpoint silently mid-workstream
- Closing the ticket / satisfying the hook as the *way* to get past a gate

All of these mean: stop, surface the gate, wait for explicit confirmation. The Stop hook never converts to approval — that conversion is the failure every flag above names.

## Halt Conditions

Stop and surface rather than proceeding when:

- **A declared checkpoint's trigger is reached** — surface it and wait for explicit human confirmation (the core behavior, not an exception).
- **A one-way door appears that no ledger entry covers** — reuse council's one-way-door gate; treat it as a checkpoint even though it wasn't pre-declared.
- **The work is too small for a workstream** — redirect to `/coral`; don't manufacture ceremony.
- **A composed skill is unavailable** (council/cross-review/design/issue/execution-plan absent) — surface the gap; run the lifecycle phases you can and name what's missing, rather than reimplementing the skill.

## State

Per-run scratch (the in-progress ledger, phase notes) lives in `state/` (gitignored). The ledger is conversational/surfaced, not a machine-parsed manifest — a parsed, resumable on-disk manifest is deferred until the primitive is validated.

## References

- `references/checkpoint-ledger.md` — the ledger format, the two canonical types, the custom-checkpoint contract, worked examples.
- `references/composition.md` — the full lifecycle, which skill is invoked at each seam, and where the checkpoints sit.
- `references/signal-kit-telemetry.md` — the first signal kit (gate mode): the guard's read adapter, citable query vocabulary, decision semantics, and non-negotiable correctness contracts.

## What this skill defers

The `/goal` Stop-hook *mechanism* wiring (this ships the discipline as prose — defer the hook integration until the prose proves insufficient under goal-pressure in an eval; **acceptance criterion for wiring it:** an eval of the operator-offline + Stop-hook + green-CI scenario at a declared gate shows the prose alone failing to hold the gate); auto-driving phase transitions without operator confirmation (defer until an operator runs the same sequence ≥3 times by hand); a machine-parsed, resumable checkpoint manifest (defer until the primitive is validated).
