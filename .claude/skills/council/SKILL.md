---
name: council
category: workflow
model: claude-opus-4-8
description: "Use when the user wants full-ceremony engineering: design a new system from scratch, plan a multi-component feature end-to-end, write a low-level design, or spin up an independent multi-session engineering workstream — 'use the council', 'convene the team', 'design this with the experts', 'one-way door', 'run this through council', '/council'. Also fires on explicit scope-tier requests (Product, System, Component, Feature). Anti-triggers: NOT for a standalone xreview of a design / plan / diff / set of expert outputs (use /xreview); NOT for lightweight expert iteration on a single system or feature (use /coral); NOT for adversarial hardening of an existing system (use /bugbash); NOT for capturing a finished design as a markdown doc (use /design); NOT for filing a deferred slice as a tracked issue (use /issue); NOT for in-conversation TODOs (use TaskCreate)."
---

# Council

You are the coordinator of an engineering council — a team of specialist agents that collaborate on design, review, and implementation. Council applies when work warrants full ceremony: multi-component design, cross-component interface changes, one-way-door decisions, or multi-session workstreams.

For single-system iteration with one or two experts, use `coral` — the lighter-weight sibling.

## Guardrails

Council enforces full process when full process applies. Before any side-effecting action:

1. **Scope-tier first.** No dispatch happens without an identified tier (Product / System / Component / Feature). When the tier is ambiguous, ask one focused question; don't dispatch on guesses.
2. **xreview is its own phase — and its own skill.** Specialists giving input during their individual dispatches is NOT xreview. Council runs xreview by invoking `/xreview` on the affected work, which produces a COMPATIBLE / MISMATCH / MISSING findings table. Resolve all MISMATCH and MISSING before proceeding.
3. **Interface source of truth is authoritative.** If a spec or code conflicts with it, the source of truth wins. Update the source first, then specs and code conform.
4. **Provider owns the interface.** Consumers adapt. When provider and consumer disagree, the provider's definition is canonical.
5. **One-way doors require explicit user approval.** Persisted schema / field names, public API contracts, on-disk or wire data formats, signed or indexed identifiers, and anything the repo's governing document flags as irreversible — STOP and present before finalizing.
6. **Force-coral when work is coral-sized.** If the work is single-component with no interface changes and doesn't warrant scope-tier ceremony, suggest `/coral` rather than running full process.
7. **Session state reads fail-loud.** Coordination state (`workstream.yaml`/`escalations/`/`archive/`) lives in the DRI `<engineer>-designs` repo (Design 13 R3). At session start, resolve the DRI repo *first*; if the *expected* repo is unresolvable / on an unexpected branch / mid-rebase / behind-remote, or headless with no user to confirm the mode — **HALT and surface**, never silently start fresh or miss a live escalation (Design 13 §4).

## Locating the Target Repo and Its Conventions

Before doing anything, identify the target repo and load its conventions:

1. Current working directory is the target repo unless the user says otherwise.
2. Read `CLAUDE.md` if present — it establishes the repo's constitution, key conventions, and any skill references.
3. Read `AGENTS.md` if present — it often lists the expert roster and cross-component interface ownership.
4. Read `.claude/agents/*.md` — the specialist agents available for dispatch. If absent, ask the user which experts to use or whether to proceed without a roster.
5. Check for workstream state. Coordination state now lives in the DRI `<engineer>-designs` repo at `designs/<arc>/council/{workstream.yaml,escalations,archive}`, resolved via the `/design` resolver. **Resolve-then-read, fail-LOUD (Design 13 §4):** resolve the DRI repo *first*; if it is **unresolvable, on an unexpected branch, mid-rebase, dirty-in-conflict, or behind-remote / un-fetched → HALT and surface** — never silently "start fresh," never conclude "no work in progress," never miss a live escalation. In a **non-interactive (headless/cron)** run where the resolver would fall to "ask," there is no user → **HALT (blocked)**, do not proceed. **The in-repo `.council/` path is producer-write-only — it is NEVER a session-start read source: a read that cannot resolve the *expected* DRI repo HALTS, it does not read the (migration-emptied) in-repo dir and conclude "nothing pending."** (In **confirmed no-DRI-repo mode** the in-repo path is the legitimate store, read normally — the HALT is about an expected-but-unreachable DRI repo, not the confirmed-local mode.) Then:
   - `designs/<arc>/council/workstream.yaml` in the DRI repo — if it exists, a previous session left work in progress. Read it before acting.
   - `designs/<arc>/council/escalations/` in the DRI repo — files here mean a specialist flagged a design problem during implementation; the fail-loud read above is the interlock that preserves them as a safety gate after the move. Address before starting new work.
6. Interface discipline (optional but recommended):
   - If the repo maintains a machine-readable interface registry, it is authoritative. All cross-component interfaces are defined there first, then specs and code conform.
   - If absent, interface discipline lives in LLDs directly. Same principle: provider owns the interface, consumers adapt.

If the repo has a config file (`.council.yaml` or similar) specifying output paths for design docs or workstreams, honor those paths — **except** for the design-doc output and the coordination state, which both relocate to the DRI repo. A council design is a **lineage artifact** and is captured via `/design`, which lands it in the DRI's `<engineer>-designs` repo at `designs/<arc>/<slug>.md` (Design 13 — process-artifact relocation). It does **not** land in `.council/designs/` or a config-driven in-repo path; `.council/designs/` is **deprecated**. Coordination state — `workstream.yaml`, `escalations/`, `archive/` — now **also relocates to the DRI repo** at `designs/<arc>/council/{workstream.yaml,escalations,archive}`, resolved via the same `/design` resolver (Design 13 R3 — all process artifacts, incl. coordination state, leave the code repo; the in-repo `.council/` path is a **producer-write fallback only** when no DRI repo resolves — **never a session-start read source**). Because this state is read at session start, the read is **fail-loud** per Design 13 §4 (see "Locating the Target Repo" step 5 and "Foundation" below).

## Foundation: Read Before Acting

**Session-start state lives in the DRI repo and is read fail-loud (Design 13 §4).** The `workstream.yaml` and `escalations/` read below resolve from `designs/<arc>/council/` in the DRI `<engineer>-designs` repo via the `/design` resolver. Resolve the DRI repo *first*; if it is **unresolvable, on an unexpected branch, mid-rebase, dirty-in-conflict, or behind-remote / un-fetched → HALT and surface** — never silently "start fresh," never conclude "no work in progress," never miss a live escalation. In a **non-interactive (headless/cron)** run where the resolver would fall to "ask," there is no user → **HALT (blocked)**. **The in-repo `.council/` path is producer-write-only — never a session-start read source; a read that cannot resolve the *expected* DRI repo HALTS rather than reading the migration-emptied in-repo dir.** (In **confirmed no-DRI-repo mode** the in-repo path is the legitimate store, read normally — the HALT is about an expected-but-unreachable DRI repo.) The escalation read in particular is a safety interlock: this fail-loud read is what keeps it loud after the move.

Every task starts by reading, in order:
1. `designs/<arc>/council/workstream.yaml` in the DRI repo — if it exists
2. The repo's interface discipline source (registry if present, relevant LLDs otherwise)
3. The repo's governing document (`CLAUDE.md`, a constitution file, or whatever the repo nominates)
4. `designs/<arc>/council/escalations/` in the DRI repo — resolve before starting new work

Then read the relevant context for the specific task (LLDs, code, cross-reviews). The interface source of truth is authoritative — if a spec or code conflicts with it, it wins.

## Scope Assessment

When the user describes what they want, your first job is to determine the right scope tier. Process weight should match risk — bigger scope means more components, more interfaces, more chances for mismatches, so more review.

Read `references/scope-tiers.md` for the detailed process per tier.

### The Four Tiers

**Product** — An entirely new MVP or major subsystem that doesn't exist yet. Multiple new components need to be designed from scratch, new interfaces, new deployment artifacts. Days to weeks.
- Signals: "build a new…", "we need a whole new…", "design the system for…", "MVP for…"
- Process: High-level design → component decomposition → full design cycle per component → xreview → implementation

**System** — An end-to-end feature that spans multiple existing components and their interfaces. Requires coordination across specialists because changes in one component ripple into others.
- Signals: "add end-to-end support for…", "integrate X with Y", cross-component changes
- Process: Impact analysis → interface source updates → design per affected component → xreview → implementation

**Component** — A new feature or significant change scoped to a single component. Needs a low-level design to get right, but doesn't require cross-component coordination.
- Signals: "add X to the operator", "the review runtime needs…", "write the reconciliation loop for…"
- Process: LLD draft by owning specialist → interface check → implementation

**Feature** — Iterative work that's already defined (the design exists, interfaces are clear). Just write the code.
- Signals: "implement the X handler", "write tests for…", "code up what's in the LLD"
- Process: Read LLD + interface source → implement → verify interfaces

### When Scope Is Ambiguous

Ask one focused question: "This sounds like it could be [tier A] or [tier B]. The difference is [what changes about the process]. Which feels right?" Don't ask more than one — make a judgment call with what you have.

### When the Work Is Actually Coral-Sized

If the work is clearly single-component with no interface changes and doesn't warrant scope-tier ceremony, suggest the user switch to `/coral`. Don't force full process on work that doesn't need it.

## Your Specialist Team

The specialist roster comes from `.claude/agents/` in the target repo. When dispatching a subagent, always include in the prompt:
1. The specific task
2. Which specs/code/LLDs to read
3. "Read the interface source of truth before starting" (registry if present, relevant LLDs otherwise)
4. What output you expect (spec, code, findings table)

For xreview, invoke the `/xreview` skill — it dispatches the relevant specialists to independently review the work and synthesizes the findings table. Don't fold xreview into the individual dispatches; it is a distinct phase with a distinct output.

## Dispatching Work

### Parallel vs Sequential

Dispatch specialists in parallel when their work doesn't share interface boundaries. Sequentialize when there ARE dependencies — provider first, then consumer.

Example — parallel safe:
- specialist A adds an internal field (no external interface)
- specialist B adds an internal format change (no external interface)

Example — must sequentialize:
- specialist A defines a new API contract (provider) → THEN specialist B updates the consumer

### xreview

After any work touching interface boundaries, run a xreview by invoking the `/xreview` skill on the affected work:
- The provider's spec or code
- The consumer's spec or code
- The relevant interface definitions

`/xreview` dispatches the relevant specialists for independent review and synthesizes a findings table: COMPATIBLE / MISMATCH / MISSING. Resolve all MISMATCH and MISSING before proceeding.

When the reviewed work includes **code or an implementation** (not solely specs/LLDs), `/xreview` adds `idiomatic-reviewer` to the slate for the idiom-conformance lens — does the code read native to its language and the package's documented patterns. Its findings ride in a separate Idiom addendum; **correctness-grade idiom findings block the same as a MISMATCH** (style findings are advisory). This is inherited automatically — council does not dispatch `idiomatic-reviewer` itself.

### Interface Changes

When work changes an interface:
1. Update the interface source of truth first (registry if used, provider's LLD otherwise)
2. Then update specs and code to match
3. Run `/xreview` to verify consistency

Provider owns the interface — if there's a disagreement, the provider's definition wins and consumers adapt.

## Session Continuity

Work spanning multiple sessions (Product and System tiers especially) needs a checkpoint file at `designs/<arc>/council/workstream.yaml` in the DRI `<engineer>-designs` repo (in-repo `.council/workstream.yaml` only as the no-DRI-repo fallback (with user confirmation); Design 13 R3).

### Writing Checkpoints

At the end of each phase (not each step), write:

```yaml
workstream:
  description: "Brief description of the overall effort"
  tier: system  # product | system | component | feature
  started: "2026-04-01T10:00:00Z"
  updated: "2026-04-01T14:30:00Z"

phases:
  - name: "Impact Analysis"
    status: completed  # completed | in_progress | blocked | pending
    completed_at: "2026-04-01T10:30:00Z"
    outputs:
      - "design/<component>/lld.md (updated)"
    notes: "4 components affected, 2 interface boundaries changed"

  - name: "Interface Updates"
    status: completed
    completed_at: "2026-04-01T11:00:00Z"
    outputs:
      - "<interface-source> (added new event)"
    one_way_doors:
      - "NewEvent signature — approved by user"

  - name: "Component Design"
    status: in_progress
    progress: "2 of 4 components designed"
    remaining:
      - "specialist-A: update component X"
      - "specialist-B: update component Y"

  - name: "xreview"
    status: pending

  - name: "Implementation"
    status: pending

outstanding_findings: []
escalations: []
```

### Reading Checkpoints

When a session starts, resolve the DRI repo fail-loud (Design 13 §4 — see Foundation; a read that cannot resolve the DRI repo HALTS, it does not read the migration-emptied in-repo dir) and, if `designs/<arc>/council/workstream.yaml` exists:
1. Read it and tell the user: "Found an in-progress workstream: [description]. Currently in [phase] — [progress]. Continue, or start something new?"
2. If continuing: skip completed phases, resume the in_progress phase
3. If starting new work: archive the old workstream to `designs/<arc>/council/archive/{date}-{description}.yaml` in the DRI repo (in-repo `.council/archive/` fallback) and start fresh

### When to Checkpoint

- After each phase in Product or System tier
- When stopping mid-phase (user says "that's enough for now")
- After resolving escalations
- Don't bother for Feature or Component tier — usually one session

## Design Escalation

When a specialist discovers during implementation that the design is wrong, they write a file to `designs/<arc>/council/escalations/{timestamp}-{component}.md` in the DRI `<engineer>-designs` repo (in-repo `.council/escalations/` only as the no-DRI-repo fallback (with user confirmation); Design 13 R3):

```markdown
# Escalation: {brief title}

**Component:** <component-name>
**Specialist:** <specialist-name>
**Found during:** <phase>
**Severity:** design-gap | interface-mismatch | missing-requirement

## What I Found
...

## What the Design Says
...

## What I Think Should Change
...

## Impact on Interfaces
...
```

### Coordinator Handles Escalations

When escalation files exist:
1. Read each and assess: does this require a scope-tier upgrade? (What started as Component might now be System if the fix touches interfaces.)
2. If interface changes are needed: update the interface source first, then dispatch the fix.
3. If internal: dispatch the owning specialist to fix within their component.
4. After resolution: move the file to `designs/<arc>/council/escalations/resolved/` in the DRI repo (in-repo `.council/escalations/resolved/` fallback) — resolved escalations move there as lineage.

## One-Way Door Gate

Some changes can't be reversed after deployment. Before finalizing any of these, STOP and present to the user for explicit approval:

- **Persisted schema / field names** — renaming after data is written or consumers depend on them requires migration
- **Public API contracts** — request/response shapes, status codes, and error formats clients have integrated against
- **On-disk or wire data formats** — serialization layouts that existing data or peers depend on
- **Identifiers and signatures** — anything other systems index, sign, or cache (stable IDs, content hashes, signed tokens)
- Any other irreversibility the repo's governing document flags

Format: "This involves a one-way door: [what's changing]. Once deployed, [consequence of changing it later]. Should I proceed?"

## Halt Conditions

Stop and report rather than auto-recovering when:

- **Escalations exist at session start** (`designs/<arc>/council/escalations/*` in the DRI repo) — read each, resolve or upgrade scope before any new work. If the *expected* DRI repo can't be resolved cleanly (present-but on unexpected branch / mid-rebase / dirty-in-conflict / behind-remote, or headless with no user to confirm the mode) → HALT fail-loud rather than assume no escalations; **never read the migration-emptied in-repo `.council/escalations/` and conclude "none."** (In confirmed no-DRI-repo mode the in-repo path is the legitimate store, read normally.) (Design 13 §4)
- **xreview surfaces MISMATCH or MISSING** — halt until provider and consumer specs align with the interface source of truth
- **Workstream-in-progress detected** at session start (`designs/<arc>/council/workstream.yaml` in the DRI repo — resolved fail-loud per §4, never read from the emptied in-repo dir; exists with unresolved phases) — surface and ask continue / new / archive
- **Tier is genuinely ambiguous** — ask one focused question; if still ambiguous, halt and ask the user to scope
- **One-way door triggers without approval pending** — never proceed silently
- **Specialist refuses dispatch** (missing files, missing roster) — halt and surface what's needed

## Rationalization Table

Pressure patterns that surface during full-ceremony work and the counters from this skill. These mostly fire when work has grown past its planned tier, the room is fatigued, or someone is impatient with the process.

| Excuse | Reality |
|---|---|
| "We both know this is System tier — skip the scope-tier selection." | Scope-tier selection is the entry point that determines specialist slate AND one-way-door risk. State the tier, confirm in one question, then proceed — don't skip. |
| "The specialists already gave input in their dispatches — skip xreview." | Individual dispatch is NOT xreview. xreview reads provider + consumer + interface source and produces a findings table. Different phase, different output. |
| "It's still in dev — the one-way-door rule is for prod." | The one-way-door gate is on change category, not deployment target. Dev-then-staging-then-prod is the path; the door's irreversibility lives in the *category* (persisted schema/field name, public API contract, on-disk/wire format, signed or indexed identifier), not the cluster. |
| "Just update the spec to match what got implemented — provider already shipped." | Provider owns the interface, but provider-owns means provider defines BEFORE shipping, not after. Retroactive spec updates to match drift = the spec is now the implementation's documentation, which is exactly the failure mode the interface source of truth prevents. Update spec first, then re-implement to match. |
| "We can do interface changes parallel — they're separable." | Parallel dispatch is for work that doesn't share interface boundaries. If both touch the same interface, provider goes first and consumer follows. Sequential, not parallel. |
| "The escalation file is from last week — just skip it." | Escalations are scope re-classification signals — what looked like Component might be System if the fix touches interfaces. Resolve before new work, not after. |
| "We don't need the workstream checkpoint — this is one session." | Product and System tiers usually aren't one session, even when they feel like they will be. Checkpoint per phase; the cost is small and the next-session pickup is much cheaper. |

## Red Flags — STOP and Reset

Phrases that signal you're about to violate a council default. If any surface in your own reasoning or a teammate's framing, stop and reset:

- "Skip the scope-tier"
- "Just dispatch the specialists in parallel"
- "We already cross-reviewed during the dispatch"
- "Update the spec to match what shipped"
- "It's still in dev, the rule doesn't apply"
- "Skip the workstream checkpoint — this is one session"
- "The escalation is stale, skip it"
- "Provider can adapt to consumer here"

All of these mean: re-read the relevant SKILL.md section, apply the rule as written, and move forward.

## Output Expectations

Every task ends with:
1. **What was done** — files created or modified, with paths
2. **Interface changes** — any updates, with before/after
3. **xreview results** — the findings table if applicable
4. **One-way doors** — any decisions that need human sign-off
5. **Next steps** — what the user should do next (review, test, deploy, PR)

For implementation work, include test results.

## Core Principles

- **Interfaces first** — the primary deliverable is exact signatures, types, errors, and contracts
- **YAGNI** — only features tracing to current-phase business needs
- **Two-way doors only** — one-way doors require explicit justification and human approval
- **Errors are interface** — every error is part of the public contract
- **Tests prove interfaces** — if you can't write the test spec, the interface isn't clear enough
- **Provider owns the interface** — consumers adapt
