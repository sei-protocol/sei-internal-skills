# Coral / Council Integration

How `/issue` plugs into the orchestration skills. This document is the contract between coral/council (the caller) and `/issue` (the callee). Read alongside `format-spec.md`.

## Why this exists

Coral and council sessions produce a lot of high-signal context: the original ask, what specialists argued for, what the orchestrator cut, what got deferred and why. Without `/issue`, that context dies with the session — the next pickup either re-derives it or never starts. The integration makes "bootstrap the next workstream as a tracked issue" a one-step move at handoff.

## When the orchestrator should offer `/issue`

Coral / council should *proactively* surface `/issue` at these moments:

| Moment | Why |
|---|---|
| Synthesis turns up a "deferred — when X" line | The trigger condition (`X`) and the deferred slice are exactly what the issue's Out of scope + Problem fields capture. Don't lose them. |
| User explicitly cuts scope ("not now, but file it") | The user just *asked* for a file action. Don't make them re-state the slice. |
| End of a coral session with an obvious phase 2 | Phase 2 will get picked up by someone — could be the same user next week, could be a teammate. The issue is the handoff. |
| One-way door surfaces but isn't being walked through | The design discussion that produced "we'd want to think harder before doing X" needs to land somewhere durable. |
| Council closes a workstream with a sibling identified | The sibling workstream gets a tracking issue so the council session that opens it isn't starting cold. |

The orchestrator phrases it as an offer, not a default action: *"Want me to file this as an issue so it's ready for next pickup?"* — user opts in.

## What the orchestrator passes to `/issue`

The orchestrator has the session in head. Pre-fill, don't re-prompt. The skill's procedure step 2 is short-circuited when these are provided:

| Issue field | Pulled from |
|---|---|
| **Title** | The deferred slice's name as the orchestrator referred to it. Make it descriptive, not a label. |
| **Problem** | Observable behavior or gap that motivates the slice. From the original ask + what surfaced during specialist dispatch — *not* the proposed fix. |
| **Impact** | Why this was cut rather than dropped. The case for "this matters, just not now." If the answer is "we don't know yet," that's a sign the issue shouldn't be filed — it's not actually a deferred workstream. |
| **Relevant experts** | The specialists already on the workstream + any adjacent personas the next pickup will need. Cap at 3; if more, the work is council-sized. |
| **Proposed approach** | Whatever sketch the specialists produced that informed the cut. Often a few sentences and a code-pointer or two. If there isn't one, omit the section — don't fabricate. |
| **Acceptance criteria** | If the synthesis produced an "if we did this, we'd want X to be true" list, include it. Otherwise omit; the next pickup defines it. |
| **Out of scope** | Critical for deferred-slice issues. What this issue *won't* touch — usually the things the *current* slice already covered, or the things deferred even further out. |
| **References** | The current PR/branch, the design doc the slice was discussed in, prior issues, runbooks. |

## Example handoff

**Coral session:** Brandon was iterating with `kubernetes-specialist` and `platform-engineer` on adding a SigningKey field to `SeiNode`. The synthesis pass cut "mid-life patch detection" — single-shot deployment ships now, drift detection deferred.

**Coral's offer at end of session:**

> The single-shot deployment is captured in the PR. We deferred mid-life SigningKey patch detection — the trigger to un-defer is "pacific-1 needs zero-downtime cutover."
>
> Want me to file that as an issue to bootstrap the follow-up workstream? I can pre-fill from the design pass we just did.

**User:** yes.

**`/issue` is invoked with pre-fill:**

```
title: Detect spec drift on Running nodes for mid-life SigningKey patch
problem: |
  PR #136 ships single-shot SigningKey deployment; buildRunningPlan only detects
  image drift, so patching SigningKey onto a Running validator is a silent no-op.
  ...
impact: |
  Primary use case is zero-downtime migration cutover for pacific-1, where the
  single-shot downtime envelope may not be acceptable.
  ...
experts: [kubernetes-specialist, platform-engineer, sei-network-specialist]
proposed_approach: |
  Reuse the NodeUpdate plan shape ... [from the specialist sketches]
acceptance_criteria:
  - Status.SigningKeyMountedSecret stamped on rollout success
  - buildRunningPlan detects drift and triggers re-apply
  - ...
out_of_scope:
  - Mode switch (full-node → validator). File as a follow-up.
  - Demoting a validator to non-signing.
references:
  - PR #135 (merged LLD)
  - PR #136 (single-shot deployment)
  - LLD §11 (deferred entry)
```

The user reviews the rendered body, adjusts one sentence, confirms. Issue is filed. Coral's session summary records the URL as the next workstream pointer.

**Result:** when Brandon (or a teammate) picks the issue up next quarter, they walk into the same context the original session had — without re-deriving it, and without Brandon having to remember he had it.

## What the orchestrator should NOT do

- **Don't auto-file without offering.** Always ask. The user owns when context becomes a tracked artifact.
- **Don't fabricate fields.** If a field doesn't have session signal, omit it rather than write a placeholder. `_TBD_` lines are anti-signal.
- **Don't expand the slice during the handoff.** The issue captures what was deferred, not a re-imagining of it. If the user wants to grow the slice, that's a new coral session.
- **Don't file multiple issues from one session reflexively.** If a session genuinely produced multiple deferred slices, ask the user which to file individually — most of the time only one or two are worth tracking.

## Coral / council attribution

Issues filed via this handoff are otherwise indistinguishable from standalone-filed issues. There's no automatic "filed via /coral" label or footer — the body speaks for itself. If a session produced an issue, that's a normal artifact; the workflow trail is in git history (the branch the session was working on) and PRs, not in issue metadata.

## Sibling skill: `/design`

`/issue` and `/design` are sibling artifact-capture skills. From a single coral/council session:

- **`/issue`** captures *next* work — a deferred slice, a phase 2.
- **`/design`** captures *this* work — the synthesized design pass.

Both can fire from the same handoff. When they do, thread the lineage:

- The new issue's References section gets a `Design: <path>` line if the session also produced a design.
- The design's frontmatter gets `Issue: #<n>` if the session also filed an issue (atypical — usually the issue exists *first* and the design is captured during a later pickup; but if both fire from one session, the design references the *new* issue).

Most common pattern: an issue is filed in session 1, picked up in session 2 where a design pass runs and `/design` captures the LLD threading back to the issue. The two-skill flow at the same handoff (filing a new issue AND capturing its design simultaneously) is rarer but supported.
