---
name: coral
description: "Lightweight expert iteration workflow. Use when the user has a specific system, feature, or problem and wants to iterate on it with the right expert(s) from the current repo. No scope-tier ceremony, no cross-review rounds, no workstream file — just smart expert dispatch and iteration. Trigger on 'huddle', 'coral', 'pull in the [specialist]', 'iterate on this with [expert]', 'let's work on X with [expert]', 'use the experts', 'work with the team on this', or when the user wants quick expert input on a defined slice of work. For sustained multi-component design work with formal process, use /council instead — coral will flag when the work grows beyond its scope."
---

# Coral

Lightweight expert iteration. When the user has a defined slice of work and wants the right expert(s) on it, coral picks the smallest set of specialists that match and iterates with them. No tier selection, no cross-review rounds, no workstream file.

This is the right tool when work is **scoped and iterative**: one or two components, no irreversible design decisions in play, a single session. When it isn't, hand off to `council`.

## Locating the Target Repo and Its Roster

1. CWD is the target repo unless the user says otherwise.
2. Read `CLAUDE.md` if present — repo conventions and any roster guidance.
3. Read `.claude/agents/*.md` — the specialist agents available for dispatch. If absent, ask which experts to use or offer to proceed without a formal roster.

## Core Loop

1. **Read the ask.** What's the specific slice?
2. **Pick experts.**
   - Exactly one match → single dispatch.
   - Two cleanly separable concerns → parallel dispatch.
   - Three+ or tangled concerns → this is probably council work. See Handoff.
   - **Design briefs always include a scope-cutter.** If the deliverable is a spec, LLD, or design doc (vs. an implementation patch or a code review), include `product-manager` (or the closest available scope-discipline persona) as either a parallel specialist or as a final scope-review pass before synthesis. Depth specialists alone tend to maximize their domain; the scope-cutter holds the floor on YAGNI.
3. **Dispatch with a focused brief.** Each dispatch includes:
   - The specific slice
   - Which files to read first
   - What output you want (answer, spec, code, review)
   - **Framing:** specialist outputs are *expansion suggestions* — the maximum each expert would argue for in their domain. The orchestrator picks the minimum that delivers. Brief experts to give you the "what's the most you'd argue for," not "what's required."
4. **Synthesize with a YAGNI pass.** Before producing the deliverable:
   - Identify the smallest subset that ships value.
   - Everything else gets an explicit "deferred — when X" line in the deliverable. Not silent omission.
   - The orchestrator's job is to pick the **least** that delivers, not the union of what specialists offered.

   Then present to user, take redirection, follow up.
5. **Offer to bootstrap the next workstream as an issue.** When a deferred slice surfaces ("deferred — when X"), the user explicitly cuts scope ("not now, but file it"), or the session closes with an obvious phase 2, offer the **`/issue`** skill to capture the synthesized context as a tracked GitHub issue. Don't auto-file — the user opts in. When invoked, pre-fill from session context (Problem, Impact, Relevant experts, Proposed approach, Out of scope) rather than re-prompting fields the session already answered. See `.claude/skills/issue/references/coral-integration.md` (in repos where `/issue` is installed) for the full handoff contract.
6. **Summarize at end.** What was done, what files changed, what's next. If an issue was filed via `/issue`, the URL is the "next workstream" pointer.

## Handoff to Council

Coral is deliberately narrow. Flag and ask (never auto-escalate) when any of these appear:

- Work is touching ≥3 components, or ≥2 interface boundaries
- A one-way door comes up (event signatures, storage layout, CRD field names, EIP-712 types, or anything the repo's CLAUDE.md flags as irreversible)
- Work is clearly going to span multiple sessions
- User says "this is bigger than I thought" / "let's do this properly" / "we should design this"
- Cross-review across components becomes necessary, not just single-expert consult

Format: "This feels like it's grown past coral — [reason]. Hand off to /council, or keep iterating here?"

See `references/handoff-to-council.md` for detection criteria in detail.

## What Coral Doesn't Do

- No scope-tier process
- No mandatory cross-review
- No workstream checkpoint file (coral sessions complete in one sitting)
- No formal escalation files — if an expert says "the design is wrong," relay it to the user immediately
- No interface registry updates

## Output Expectations

1. What the expert(s) said or did
2. Files changed, if any
3. What the user should follow up on

Keep it terse. Coral is the fast path.

## Dispatching Tips

- Minimum context to experts. Over-briefing dilutes their specialty.
- Sequentialize only when one expert's output feeds another's task.
- If an expert asks for info you don't have, go to the user rather than guessing.
- Trust domain judgment. If the expert says "this pattern is wrong for the stack," relay it instead of overriding.
- **Specialists give max scope; you pick min scope.** When a brief produces three deep, opinionated outputs, the synthesis temptation is to include all of it. Resist. Cut to MVP, mark the rest deferred with the trigger condition that would un-defer it.
