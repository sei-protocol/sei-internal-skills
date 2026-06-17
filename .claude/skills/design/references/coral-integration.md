# Coral / Council Integration

How `/design` plugs into the orchestration skills. This document is the contract between coral/council (the caller) and `/design` (the callee). Read alongside `format-spec.md` and the parallel `/issue` skill's `coral-integration.md` for symmetry.

## Why this exists

Coral and council sessions that produce *designs* (vs. code patches or quick answers) generate a lot of high-signal context that often dies with the session: which specialists weighed in, what alternatives were considered, what got cut for YAGNI, what's still open. Without `/design`, that context survives only in chat or git history. The integration makes "capture the design as a tracked doc" a one-step move at handoff.

## When the orchestrator should offer `/design`

Coral / council should *proactively* surface `/design` when **the deliverable is a design**, not a code patch or a quick answer. Specifically:

| Moment | Why |
|---|---|
| Synthesis pass produced an LLD or architecture sketch | This IS the artifact; capture it before context decays |
| Council closed a workstream with a final design | The workstream's whole point was to produce this; record it |
| User says "let's write this up" / "we should document this" | They're explicitly asking for a doc |
| One-way door surfaced and a decision was made | The decision is worth a permanent record |
| Multiple alternatives were weighed before picking one | The reasoning is what matters six months later — capture it |

The orchestrator phrases it as an offer: *"Want me to capture this as a design doc under `docs/designs/` (or `designs/<arc>/` in the DRI designs repo)?"* — user opts in.

## `/design` vs `/issue` at the handoff

Both skills can fire from the same session. They're complementary, not competing:

| Skill | Captures | When |
|---|---|---|
| **`/design`** | The design itself — Background, Goals, Decisions, Diagrams, Alternatives, Trade-offs | Always, when the deliverable IS a design |
| **`/issue`** | Deferred follow-up work — phase 2, scope cuts, sibling workstreams | When the synthesis surfaces a "deferred — when X" or a phase 2 |

Common pattern: a coral session produces a design AND surfaces a deferred slice. The orchestrator offers BOTH:

> Two captures available:
> - `/design` — write up what we just designed under `docs/designs/`
> - `/issue` — file the mid-life patch follow-up as a tracked issue
>
> Want me to do both?

User opts in to either, neither, or both.

When both fire, threading the lineage matters:
- The design doc references the source issue (if applicable).
- The follow-up issue's References section gets a link to the new design doc.
- See `issue-integration.md` for the full bidirectional flow.

## What the orchestrator passes to `/design`

The orchestrator has the session in head. Pre-fill, don't re-prompt. The skill's procedure step 3 is short-circuited when these are provided:

| Design field | Pulled from |
|---|---|
| **Title** | The slice's name as the orchestrator referred to it. Make it descriptive — file readers will see it in directory listings. |
| **Issue ref** | If the session referenced an issue (filed via `/issue` or pre-existing), capture the ref — a GitHub number (`#14`) or a Linear identifier + URL (`ENG-123 — <url>`). Otherwise omit. The reverse link at step 8 then threads to whichever sink the ref names (GitHub via `gh`, Linear via the `save_comment` MCP tool) — see `issue-integration.md` ("Two sources"). |
| **Authors** | git user.name plus any specialists the orchestrator dispatched (the specialist personas, not human names — e.g., `kubernetes-specialist, platform-engineer`). |
| **Background** | Original ask + system context surfaced during specialist dispatch. *Not* the proposed fix — just the why. |
| **Goals** | What the session set out to achieve. Often the orchestrator's framing of the slice. |
| **Non-goals** | Explicit scope cuts during synthesis. If the issue had Out-of-scope, those carry through too. |
| **Design** | The synthesized output — the meat of what the specialists produced and the orchestrator picked. Include code/file references the specialists pointed at. Generate plausible mermaid diagrams from interaction patterns the specialists described; mark them for verification. |
| **Alternatives** | Specialist suggestions that were weighed and not taken — this is high-value content, capture it with the *why-not*. |
| **Trade-offs** | Costs/risks the YAGNI pass acknowledged. "Specialists give max scope; orchestrator picks min" — the cuts ARE trade-offs worth recording. |
| **Open questions** | Anything the session left unresolved. Useful for review. |
| **References** | Source issue (if any), branch/PR being designed against, related prior PRs, runbooks. |

## Example handoff

**Coral session:** A teammate iterated with `kubernetes-specialist` and `platform-engineer` on adding mid-life SigningKey drift detection (issue #14). Synthesis converged on reusing `buildNodeUpdatePlan` with a new drift check.

**Coral's offer at end of session:**

> The synthesized design is ready to capture. Two natural artifacts:
> - `/design` — write the LLD as `designs/<arc>/seinode-mid-life-signing-key-drift-lld.md`, threading back to issue #14.
> - `/issue` — none right now; the follow-ups (mode switch, demoting to non-signing) are already in #14's Out of scope.
>
> Capture the design?

**User:** yes.

**`/design` is invoked with pre-fill:**

```
title: SeiNode mid-life SigningKey drift detection
status: Draft
issue: 14
authors: kubernetes-specialist, platform-engineer, bdchatham
output_dir: designs/<arc>/  # DRI designs-repo convention (Design 05)
suffix: -lld.md  # detected Tide LLD convention

background: |
  PR #136 shipped single-shot SigningKey deployment. buildRunningPlan
  (internal/planner/planner.go:621-628) only detects image drift...
goals:
  - Mid-life SigningKey patches trigger a controlled pod re-roll
  - Existing single-shot flow continues to work unchanged
  - Drift detection is reusable for future spec-mutation cases
non_goals:
  - Mode switch (full-node → validator). Same mechanic, larger surface.
  - Demoting a validator to non-signing.
design: |
  Reuse the NodeUpdate plan shape (buildNodeUpdatePlan at
  internal/planner/planner.go:656)...

  ### Drift detection

  ```mermaid
  sequenceDiagram
    User->>K8s: kubectl patch seinode --signingKey
    K8s->>Controller: reconcile event
    ...
  ```
  <!-- verify this matches your intent -->
alternatives:
  - "Alt A: dedicated reconciler — clean separation but duplicates rollout watcher"
  - "Alt B: polling — simpler but races with image-drift logic"
trade_offs:
  - Re-apply triggers a full StatefulSet rolling update (~30s unavailability)
  - Drift detection is per-field, not generic
open_questions:
  - Should validation block apply, or surface in status? (kubernetes-specialist, decide in PR)
references:
  - Issue #14 (this design's source)
  - PR #135 (merged LLD for SigningKey)
  - PR #136 (single-shot deployment)
```

User reviews the rendered body, adjusts diagrams, confirms. File written to `designs/<arc>/seinode-mid-life-signing-key-drift-lld.md`. Skill offers to comment on issue #14: `Design captured: designs/<arc>/seinode-mid-life-signing-key-drift-lld.md`.

**Result:** the design is durable, the issue is now linked to its design, and a future engineer reading either the issue OR the design can navigate the lineage.

## What the orchestrator should NOT do

- **Don't auto-write without offering.** Always ask. The user owns when context becomes a tracked doc.
- **Don't fabricate sections.** Open questions especially — only include what was actually unresolved.
- **Don't expand the design during the handoff.** The doc captures what was decided, not a re-imagining of it. If the user wants to grow the design, that's a new coral session and probably council-tier.
- **Don't fire `/design` for code patches.** If the deliverable is "I wrote the code," there's nothing to design-doc; the PR description is the artifact.
- **Don't fabricate diagrams.** If session context didn't surface an interaction pattern, don't invent one to fill the Design section. Diagrams are optional; missing diagrams beat misleading diagrams.

## Status lifecycle

`/design` writes `Status: Draft` initially. The orchestrator should not bump status during the handoff — that's a downstream review-pass concern. The skill's job is to capture the draft; humans (and possibly a future `/design-review` skill) move it through the lifecycle.
