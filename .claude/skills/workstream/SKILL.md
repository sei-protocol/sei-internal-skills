---
name: workstream
category: workflow
model: claude-opus-4-8
description: "Use when launching a substantial multi-step workstream on the Coral stack with declared gates — 'launch a workstream', 'kick off this goal with checkpoints', 'start a workstream with a PR sign-off gate', '/workstream'. Scaffolds the Coral lifecycle (council → xreview → /design → /issue → /execution-plan) and declares three gate kinds: human **checkpoints** (surface + confirm before proceeding), signal **guards** (fail-closed metric gates watching a live signal during a cutover), and **review-gates** (merge-on-consensus — satisfied when the /xreview slate is unanimous and declared checks pass, e.g. 'merge once no agents have concerns and bugbot is clean'). Anti-triggers: NOT the Claude Code /goal harness command (that sets the objective; this governs how it's pursued); NOT a single 1–2 specialist slice (use /coral); NOT scope-tiered design alone (use /council); NOT capturing a finished design (use /design). Composes council/xreview/design/issue/execution-plan; never edits them."
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
3. **Compose, never edit — and never reimplement.** Invoke `council`/`xreview`/`design`/`issue`/`execution-plan`; never modify them. Delegate all lineage decoration to `/execution-plan` (never re-implement label/identity/stamp logic — that mints a second identity). If a composed skill is **unavailable**, surface the gap and run the phases you can — do **not** hand-roll the missing skill's logic inline (e.g. don't reimplement council's scope tiers).
4. **Declare the ledger up front; confirm scope before side effects.** At workstream start, echo the **scope** (objective + the council scope tier) and the **checkpoint ledger**, surface both, and get the operator's go-ahead before invoking the lifecycle. A gate added silently mid-stream, or honored only when convenient, is not a checkpoint.
5. **Don't manufacture ceremony.** A genuinely small slice does not need a workstream — redirect to `/coral`. Checkpoints are gates a human actually wants, not decoration.
6. **A guard fails closed — it never PASSes on data it cannot confirm.** Stale, unreachable, empty, or incomplete (partial-response `warnings`) reads are `inconclusive` ⇒ abort, never "looks fine." A guard does not replace a one-way-door checkpoint and may not launch a workstream or create ledger entries at trip time (the ledger is static after declaration). See "The guard primitive."
7. **A review-gate fails closed — it reads the `/xreview` review-ledger, never the transcript.** A `review-gate` is satisfied **only** when the review-ledger reads a passing terminal per `/xreview`'s gate-read contract (`RESOLVED` or `RESOLVED-WITH-ACCEPTED-RISK`, `OpenFindings: 0`, `Dissenter` held, cross-field consistent) **with `Convergence: unanimous`** (the consensus refinement — a recorded `split` latest round is not merge-ready) **and** every declared check passes. An absent / malformed / self-contradictory ledger, a `split` latest round, any open finding, a pending/neutral/failed check, or a chat assertion of approval ⇒ **FAIL closed** — never merge. Unanimous RATIFY means *zero open concerns*, not "the open ones look minor"; the gate cannot be self-relaxed, and it **never** gates a one-way door (council's human gate stands). The up-front declaration *is* the operator's merge authorization — it is not self-waivable or widenable mid-stream. See "The review-gate primitive." It **composes** `/xreview` (never reimplements the slate or ledger).

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

- **`design-approval`** — *trigger:* a design has been captured (via `/design`) **and** cross-reviewed. *gate:* the human signs off on the captured, reviewed design before implementation begins.
- **`pr-sign-off`** — *trigger:* a PR is ready (CI green) and merging it — or starting work that depends on it — is the next step. *gate:* the human confirms the merge / the go-ahead for dependent work.

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
3. **Surface on trip; the human owns the irreversible call.** `on_trip` halts before the next step and routes to a pre-declared rollback checkpoint. Auto-abort is deferred and, if ever enabled, only a pre-declared reversible+idempotent rollback, never on a one-way-door step.
4. **Recursion bound.** A guard may surface, halt, and route to an **already-declared** checkpoint — nothing more. It may **not** launch a workstream, spawn a sub-agent that launches one, or create ledger entries at trip time. The ledger is static after declaration. (This bounds a `continuous` guard's trip handler from becoming an unbounded watch→remediate→watch loop.)

The kit's **eight correctness contracts** (four verdict-gating, four budget/tuning) — and the soak verdict rule (N-consecutive-breach vs the gate-start baseline) — live in `references/signal-kit-telemetry.md`. **Do not construct a guard verdict from this summary alone**; a guard that skips the contracts PASSes on the common cutover failure modes (no-traffic-drain, half-fleet read).

## The review-gate primitive (a consensus gate)

A **review-gate** is the third gate kind: where a `checkpoint` waits on a *human* and a `guard` watches a *live signal*, a review-gate reads a **`/xreview` review-ledger + a declared check set**. It makes *"merge once the reviewer slate is unanimously RATIFY (zero open concerns) and the declared automated checks pass"* — the strongest gate operators actually use — a **declarable, enforceable** primitive instead of a conversational hand-off. It is the machine counterpart of `pr-sign-off`: the operator pre-delegates the *routine* merge decision; the human still owns every one-way door.

**It composes `/xreview` (PLT-535), it does not reimplement it.** `/xreview` owns the slate, the routing, the blinded dispatch, the assigned dissent, and the **review-ledger** schema + gate-read contract. The review-gate is the ledger's **consumer**: it *reads* the ledger fail-closed and *invokes* the slate (via the verify-to-convergence loop). There is one contract — `/xreview`'s gate-read contract — and this gate follows it; it never re-derives review state from the transcript.

### The review-gate ledger entry

A third ledger entry kind, declared up front alongside checkpoints and guards:

```
- review-gate: <short identifier, kebab-case>
  slate:    <the declared reviewer slate — or "routed by /xreview per change-class">
  checks:   <the declared automated checks that must be green — e.g. cursor-bugbot, named CI workflows>
  ledger:   <the /xreview review-ledger path — target-derivable per PLT-535; the gate computes it from the target, no registry>
  satisfied_when: the review-ledger's latest round reads a PASSING TERMINAL (per /xreview's gate-read contract) AND Convergence is unanimous (the consensus refinement — see "The review-gate enforcement spine") AND every declared check has passed
  on_fail:  surface + route to a PRE-DECLARED human checkpoint (e.g. pr-sign-off) — never self-merge on a fail
```

### The review-gate enforcement spine

1. **Read the ledger fail-closed — never re-derive review state.** Compute the review-ledger path from the target (PLT-535's target-derivable rule), read the **latest round's** header block, and apply `/xreview`'s passing-terminal gate-read **verbatim** (the provider's ledger-validity check: passing-terminal `State` + `OpenFindings: 0` + parseable in-enum `Convergence` + non-empty `Dissenter` + cross-field consistency), **then the review-gate's consensus refinement: `Convergence: unanimous`** (a recorded `split` latest round is not merge-ready). An **absent, malformed, out-of-enum, self-contradictory, or `split`** ledger ⇒ **FAIL**, identical to `State: OPEN`. Never read the transcript, an author's frontmatter assertion, or your own memory of the reviews — only the committed ledger. **Never error-into-pass:** no clean passing terminal found ⇒ FAIL, never a skipped check that proceeds.
2. **Unanimous RATIFY = zero open concerns — not "the open ones look minor."** An open per-lens DISSENT, an open MISMATCH/MISSING, or `OpenFindings ≥ 1` fails the gate. Resolving it, or the operator explicitly accepting a *named* risk (a new ledger round), is the only path to pass. The gate cannot be self-relaxed.
3. **All declared checks must actually pass.** A check still running, errored, neutral, or never reported is **not** green — fail closed on it, the same way a guard fails closed on an inconclusive read. Pending is not pass.
4. **The up-front declaration is the authorization; it is not self-waivable.** Declaring a review-gate is the operator pre-authorizing the merge *conditioned on* the bar — it replaces the per-PR token, it does not let you invent or lower the bar. A review-gate added, widened, or relaxed silently mid-workstream is not a gate.
5. **A review-gate never gates a one-way door.** For a persisted-schema / wire-format / signed-ID / prod-touching step, council's one-way-door **human** checkpoint stands; the review-gate may sit alongside (the slate still reviews) but does not discharge the irreversible call. Same rule as the guard primitive.

**On a fail, `on_fail` surfaces the specific reason + the ledger evidence and routes to a pre-declared human checkpoint** — it does not loop unboundedly and does not self-merge. When the failure is *open review findings*, the verify-to-convergence loop (lifecycle step 3) is what drives them to resolution; the gate is the *reader* of the result, not the fixer. See `references/review-gate.md` for the full contract and the verify-to-convergence loop.

## The lifecycle (the procedural scaffold)

`workstream` walks the Coral lifecycle, inserting checkpoints at the seams. It **invokes** the existing skills at the right moments; it does **not** auto-drive them (the operator still confirms phase transitions). Full recommended flow in `references/composition.md`.

1. **Declare + surface the ledger; confirm scope.** Echo the scope (objective + the council scope tier) and the checkpoint ledger — default ledger: `design-approval` after design capture, `pr-sign-off` before merge / dependent work, plus any custom checkpoints the operator names — and get the operator's go-ahead **before** step 2 (per Guardrail 4).
2. **Scope-tier the work via `council`.** Invoke `/council` (it owns the four scope tiers and produces the design content). Do not reimplement tiers.
3. **Verify via `/xreview` — iterate to convergence.** Run blinded multi-specialist review on the integrated artifact **before** capture (xreview precedes `/design` — design captures what's been reviewed, it doesn't review). This is a **loop, not a single step**: invoke `/xreview` (it classifies, routes the slate, dispatches blinded, synthesizes the review-ledger); if the ledger's latest round is not a passing terminal (open findings, a DISSENT, a split), **apply the fixes and re-invoke `/xreview`, which appends a new round** — repeat until the latest round is a passing terminal or the `OPEN-BLOCKED` fail-to-human exit. The loop's record **is** the review-ledger's appended rounds — don't keep a second convergence log. **Compose, never reimplement:** the workstream sequences the rounds and applies the fixes; `/xreview` owns how each round is dispatched and recorded. See `references/review-gate.md`.
4. **Capture via `/design`.** Offer `/design` to write the reviewed design as a durable doc.
5. **`design-approval` checkpoint.** STOP; surface the captured, reviewed design; obtain explicit sign-off before implementing.
6. **Implement** to the approved design. (Council's one-way-door gate fires here if a persisted-schema / wire-format / signed-ID change appears — honor it as a checkpoint even though it wasn't pre-declared.)
7. **Capture deferred slices via `/issue` — conditionally.** Only if a slice was cut. Don't reflexively file an empty issue.
8. **Decorate lineage via `/execution-plan`.** Call it to stamp the bet label + design link (delegated; never duplicated). Its first-label-creation confirm is a *separate* gate it owns — not subsumed by the ledger.
9. **Ship — `pr-sign-off` *or* `review-gate`.** Either a human `pr-sign-off` checkpoint (STOP; surface the ready PR; obtain explicit confirmation before merge) **or**, if the operator declared a `review-gate` up front, the review-gate is evaluated: read the review-ledger fail-closed + confirm every declared check is green ⇒ on satisfied, merge per the pre-authorized declaration (no per-PR token). On fail, surface the reason and route to the pre-declared human checkpoint. The two are alternatives the operator chooses at declaration time — the review-gate is additive, never an automatic replacement. **It never gates a one-way door:** a persisted-schema / wire-format / signed-ID / prod-touching merge keeps council's one-way-door human checkpoint even when the slate is unanimous.

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
| "The one open DISSENT is minor / stale — the review-gate is effectively satisfied, merge." | Unanimous RATIFY means *zero* open concerns. The gate reads the committed ledger fail-closed; "effectively satisfied" is the self-relaxed rubber-stamp the gate exists to forbid. Resolve it or have the operator accept the *named* risk (a new ledger round) — then read pass. |
| "Three reviewers told me in chat they ratified — that's the consensus, merge." | The review-gate reads the *committed review-ledger*, never the transcript or an author's assertion. No ledger / no passing terminal ⇒ FAIL closed, identical to a human gate with no confirmation. |
| "Bugbot is still running but the reviews are clean — close enough, merge." | A declared check that is pending/neutral/absent is **not** green. Fail closed on it; pending is not pass. |
| "The slate is unanimous, so I can merge this persisted-schema change myself." | A review-gate never gates a one-way door. The slate reviewing it does not discharge the irreversible call — council's one-way-door human checkpoint stands. |

## Red flags — STOP and honor the gate

- About to merge / deploy / run a migration with an unconfirmed `pr-sign-off` or one-way-door gate
- Reasoning that concludes "they'd want me to proceed" instead of "they confirmed"
- Treating "I can't stop" as "I may approve this myself"
- Adding or relaxing a checkpoint silently mid-workstream
- Closing the ticket / satisfying the hook as the *way* to get past a gate
- Calling a `review-gate` satisfied while a DISSENT / MISMATCH / `OpenFindings ≥ 1` is open, or while a declared check is pending — or from a chat assertion instead of the committed review-ledger
- About to discharge a one-way-door merge with a `review-gate` instead of council's human gate

All of these mean: stop, surface the gate, wait for explicit confirmation. The Stop hook never converts to approval — that conversion is the failure every flag above names.

## Halt Conditions

Stop and surface rather than proceeding when:

- **A declared checkpoint's trigger is reached** — surface it and wait for explicit human confirmation (the core behavior, not an exception).
- **A one-way door appears that no ledger entry covers** — reuse council's one-way-door gate; treat it as a checkpoint even though it wasn't pre-declared.
- **The work is too small for a workstream** — redirect to `/coral`; don't manufacture ceremony.
- **A composed skill is unavailable** (council/xreview/design/issue/execution-plan absent) — surface the gap; run the lifecycle phases you can and name what's missing, rather than reimplementing the skill.
- **A declared `review-gate` cannot be satisfied from the committed ledger** — the review-ledger is absent / malformed / self-contradictory, a finding is open, or a declared check is not green ⇒ FAIL closed; surface the reason + ledger evidence and route to the pre-declared human checkpoint. Never merge, never accept a chat assertion as the evidence.
- **A `review-gate` would discharge a one-way-door merge** — it doesn't; surface that council's one-way-door human checkpoint owns the irreversible call even when the slate is unanimous.

## State

Per-run scratch (the in-progress ledger, phase notes) lives in `state/` (gitignored). The ledger is conversational/surfaced, not a machine-parsed manifest — a parsed, resumable on-disk manifest is deferred until the primitive is validated.

## References

- `references/checkpoint-ledger.md` — the ledger format, the two canonical types, the custom-checkpoint contract, worked examples.
- `references/composition.md` — the full lifecycle, which skill is invoked at each seam, and where the checkpoints sit.
- `references/signal-kit-telemetry.md` — the first signal kit (gate mode): the guard's read adapter, citable query vocabulary, decision semantics, and non-negotiable correctness contracts.
- `references/review-gate.md` — the review-gate contract: the ledger entry shape, the fail-closed gate-read (composing `/xreview`'s contract), and the verify-to-convergence loop.

## What this skill defers

The `/goal` Stop-hook *mechanism* wiring (this ships the discipline as prose — defer the hook integration until the prose proves insufficient under goal-pressure in an eval); auto-driving phase transitions without operator confirmation (defer until an operator runs the same sequence ≥3 times by hand); a machine-parsed, resumable checkpoint manifest (defer until the primitive is validated); programmatic check-status polling and a fully-unattended multi-PR auto-merge pipeline for the review-gate (MVP reads the declared checks' status at gate-evaluation time and merges the single pre-authorized PR — defer the pipeline until the single-gate primitive is validated).
