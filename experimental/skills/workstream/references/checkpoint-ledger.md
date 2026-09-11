# The checkpoint ledger

A workstream declares its checkpoints up front as a **ledger**: a typed list, surfaced to the human before the work begins. That makes the human-in-the-loop contract explicit rather than improvised. This file defines the format, the canonical types, and the custom-checkpoint contract.

## Format

Each ledger entry is a `name` / `trigger` / `gate` triple:

```
- name:    <short identifier, kebab-case>
  trigger: <the workstream state that reaches this gate — when the agent must stop>
  gate:    <exactly what the human must confirm before the agent proceeds past it>
```

The ledger is **conversational** — surfaced in the agent's message, not a machine-parsed file. (A parsed, resumable manifest waits until real use validates the primitive.)

**A second entry kind — `guard`.** Besides human `checkpoint`s, a ledger may declare signal-driven **guards** (fail-closed metric gates that watch a live signal during a high-risk step). A guard carries a `guard`/`signal`/`healthy`/`when`/`on_trip` shape, not the human triple. The guard spine enforces it (fail-closed; surface-on-trip; static-after-declaration). See SKILL.md "The guard primitive" and `signal-kit-telemetry.md`. Guards and checkpoints co-exist in one ledger: a high-risk step typically declares both (the guard watches; the human checkpoint owns the irreversible call).

**A third entry kind — `review-gate`.** A ledger may also declare a **review-gate**: a fail-closed *consensus* gate. It counts as satisfied when the `/xreview` review-ledger reads a passing terminal (unanimous RATIFY, zero open findings) AND a declared check set is green. It carries a `review-gate`/`slate`/`checks`/`ledger`/`satisfied_when`/`on_fail` shape, not the human triple.

The review-gate spine enforces it, reading the committed ledger fail-closed against `/xreview`'s gate-read contract, never the transcript. That contract is a passing terminal with `Convergence: unanimous` and zero open findings. The spine never self-relaxes and never gates a one-way door.

A review-gate **composes** `/xreview` (PLT-535) — it never reimplements the slate or the review-ledger. See SKILL.md "The review-gate primitive" and `review-gate.md`. It is the declarable form of delegating `pr-sign-off` to expert-consensus + checks. The operator declares it up front, and that declaration is the merge authorization. The human still owns every one-way door.

## Surfacing the ledger

At workstream start, after scoping the work, the agent writes and shows the ledger:

```
Workstream: <one-line objective>
Checkpoints I will honor (you confirm at each before I proceed):
- design-approval — after the design is captured, cross-reviewed, and given a `prose-steward` review, before I implement.
- pr-sign-off — after CI is green, before I merge or start dependent work.
```

This is the contract. The agent does not silently add, drop, or relax an entry later.

## The two canonical types

| name | trigger | gate |
|---|---|---|
| `design-approval` | a design capture via `/design` is complete, cross-reviewed, **and given a `prose-steward` review** | the human signs off on the captured, reviewed, prose-reviewed-passed design before implementation |
| `pr-sign-off` | a PR is ready (CI green); merging it or starting dependent work is next | the human confirms the merge / the go-ahead for dependent work |

These cover the two most common human gates (sign off on *what we will build*; sign off on *shipping it*). They are the default ledger.

**One-way-door approval is intentionally not a third canonical type.** Persisted schema/field names, wire/on-disk formats, signed or indexed identifiers, and prod-touching steps already have a gate in `council`. That is its one-way-door category. A workstream reuses that gate by reference — it does not re-canonize it. When council surfaces a one-way door, treat it as a checkpoint even if it was not pre-declared.

## Custom checkpoints

A workstream may need a gate the canonical set does not name. Operators declare these inline. The only requirement: a custom checkpoint **carries the same `name`/`trigger`/`gate` triple** — no freeform "stop and check with me sometime" gates, which nobody can enforce.

Common custom example:

```
- name:    outcome-alignment
  trigger: a phase's result is synthesized, before the next phase begins
  gate:    the human confirms the result matches intent before I build on it
```

`outcome-alignment` is an *example* of the escape hatch, not a blessed canonical name — declare it when a workstream wants it.

## Worked example

A workstream to refactor a controller, touching a persisted CRD field:

```
Workstream: refactor the reconcile loop; introduce a new status field
Checkpoints:
- design-approval — after the LLD is captured, cross-reviewed, and given a `prose-steward` review, before I edit controller code.
- one-way-door — (council-owned; surfaced here for visibility, NOT re-declared as a workstream
  checkpoint) the new status field is persisted CRD surface, so council's existing one-way-door
  gate applies before the schema change is finalized.
- pr-sign-off — after CI is green, before I merge.
```

At each trigger the agent STOPs, surfaces the gate + the evidence, and waits for explicit confirmation. Reversible work (tests, docs, an un-applied migration) continues meanwhile. A `/goal` Stop hook pushing toward completion does not waive any of these.
