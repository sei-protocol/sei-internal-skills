---
name: coral
category: workflow
model: claude-opus-4-8
description: "Use when the user has a specific system, feature, or problem and wants quick expert iteration with the right specialist(s) on a defined slice — 'huddle', 'coral', 'pull in the [specialist]', 'iterate on this with [expert]', 'let's work on X with [expert]', 'use the experts', 'work with the team on this', '/coral'. Anti-triggers: NOT for sustained multi-component design work with formal process (use /council); NOT for adversarial review of running systems (use /bugbash); NOT for capturing a session's design (use /design); NOT for one-off in-conversation TODOs (use TaskCreate). Coral flags handoff to /council when work crosses ≥3 components, ≥2 interface boundaries, hits one-way doors, or spans multiple sessions."
---

# Coral

Lightweight specialist iteration. When the user has a defined slice of work and wants the right specialist(s) on it, coral picks the smallest set of specialists that match and iterates with them. No scope-tier ceremony, no cross-review rounds, no workstream file.

This is the right tool when work is **scoped and iterative**: one or two components, no irreversible design decisions in play, a single session. When it isn't, hand off to `council`.

## Locating the Target Repo and Its Roster

1. CWD is the target repo unless the user says otherwise.
2. Read `CLAUDE.md` if present — repo conventions and any roster guidance.
3. Read `.claude/agents/*.md` — the specialist agents available for dispatch. If absent, ask which specialists to use or offer to proceed without a formal roster.

## Core Loop

1. **Read the ask.** Produce a one-line slice statement that names the work, its boundaries, and what "done" looks like. Use this slice statement throughout dispatch and synthesis as the anchor — if a specialist drifts off-slice, the statement is what you bring them back to.
2. **Pick specialists** — route off the shared table (`.claude/skills/cross-review/references/slate-routing.md`), the **one** slate mechanism (cited, not duplicated). `/coral` owns the *production*-phase application; `/cross-review` owns the *review*-phase one; both read the same change-type × tier table, so a `shared-stack` or `skill-package` slice pulls the standards-stewards in production too. If this prose and the table ever diverge, the table wins.
   - Exactly one match → single dispatch.
   - Two cleanly separable concerns → parallel dispatch.
   - Three+ or tangled concerns → this is probably council work. See Handoff.
   - **Design briefs always include a scope-cutter** (the table's T2 doc-decision / design-brief application). If the deliverable is a spec, LLD, or design doc (vs. an implementation patch or a code review), include `product-manager` (or the closest available scope-discipline persona) as either a parallel specialist or as a final scope-review pass before synthesis. Depth specialists alone tend to maximize their domain; the scope-cutter holds the floor on YAGNI.
   - **Code review/refinement slices include the idiom lens** (the table's `idiomatic-reviewer`-on-code-presence wiring). If the slice is reviewing or refining existing code (vs. producing a design), include `idiomatic-reviewer` alongside the domain specialist. It checks a distinct axis — does the code read native to its language, framework, and the package's documented patterns — that the builder doesn't self-cover. Pair them: the language specialist (e.g. `kubernetes-specialist`) owns the build; `idiomatic-reviewer` owns the idiom pass.
   - **Skill-package and shared-stack slices pull the standards-stewards — but on different rules** (the table's §4 wiring; if this prose and the table diverge, the table wins). A `skill-package` slice (edits a canonical `.claude/` skill or agent) pins **`audit-skill` + `author-skill` + `prose-steward` unconditionally** in production — the same trio `/cross-review` pins — because a skill *is* prose + discipline + an authored artifact regardless of which files the diff touches. A `shared-stack` slice pulls its stewards **by file-type-present** instead: `prose-steward` if prose, `idiomatic-reviewer` if code, and `audit-skill`+`author-skill` *only if* a `.claude/` skill body is in the diff — not the unconditional trio. Either way the dogfood gap (the change that skips its stewards under momentum) doesn't reopen at the build phase.
3. **Dispatch with a focused brief.** Each dispatch includes:
   - The specific slice
   - Which files to read first
   - What output you want (answer, spec, code, review)
   - **Framing:** specialist outputs are *expansion suggestions* — the maximum each specialist would argue for in their domain. The orchestrator picks the minimum that delivers. Brief specialists to give you the "what's the most you'd argue for," not "what's required."
4. **Synthesize with a YAGNI pass.** Before producing the deliverable:
   - Identify the smallest subset that ships value.
   - Everything else gets an explicit "deferred — when X" line in the deliverable. Not silent omission.
   - The orchestrator's job is to pick the **least** that delivers, not the union of what specialists offered.

   Then present to user, take redirection, follow up.

   **Offer `/cross-review` when the outputs need a consistency check.** When two or more specialists produced outputs that touch a shared boundary (one's output is the other's input, or both define part of the same contract), offer `/cross-review` before shipping: it has the relevant specialists independently review the combined work and synthesizes a COMPATIBLE / MISMATCH / MISSING table. This is the cross-review pass between you (the orchestrator) and the coral experts — distinct from the per-specialist dispatches that produced the work. Don't auto-fire; the user opts in.
5. **Offer artifact capture.** Two natural artifacts; either (or both) may apply:
   - **`/design`** — when the deliverable IS a design (LLD, architecture sketch, system-tier decision). Captures the synthesized design under `docs/designs/` (or repo-specific path like Tide's `design/milestones/`) with mermaid diagrams. Pre-fill from session: Background, Goals, Non-goals, Design, Alternatives, Trade-offs, Open questions, References.
   - **`/issue`** — when a deferred slice surfaces ("deferred — when X"), the user cuts scope ("not now, but file it"), or the session closes with an obvious phase 2. Captures synthesized context as a tracked issue (GitHub or Linear). Pre-fill from session: Problem, Impact, Relevant experts, Proposed approach, Out of scope.

   Don't auto-fire either — the user opts in. Both can fire from the same session: `/design` captures **this** work, `/issue` captures **next** work. When both fire, thread the lineage (design body links the issue if applicable; new issue's References gets a `Design: <path>` line). See `.claude/skills/design/references/coral-integration.md` and `.claude/skills/issue/references/coral-integration.md` for the handoff contracts.
6. **Summarize at end.** What was done, what files changed, what's next. If a design was captured, its path is the "this work" pointer. If an issue was filed, its pointer (GitHub URL, or Linear identifier + URL) is the "next workstream" pointer.

## Handoff to Council

Coral is deliberately narrow. Flag and ask (never auto-hand-off) when any of these appear:

- Work is touching ≥3 components, or ≥2 interface boundaries
- A one-way door comes up (persisted schema / field names, public API contracts, on-disk or wire formats, signed or indexed identifiers, or anything the repo's CLAUDE.md flags as irreversible)
- Work is clearly going to span multiple sessions
- User says "this is bigger than I thought" / "let's do this properly" / "we should design this"
- Cross-review across components becomes necessary, not just single-specialist consult

Format: "This feels like it's grown past coral — [reason]. Hand off to /council, or keep iterating here?"

See `references/handoff-to-council.md` for detection criteria in detail.

## What Coral Doesn't Do

- No scope-tier **ceremony** — coral does not run `/council`'s scope-tier process (the formal tiering, phase gates, and review rounds). It **does** read the change-class tier off the shared routing table (`references/slate-routing.md` §3) to size its specialist slate — reading the tier is not running the ceremony.
- No mandatory cross-review
- No workstream checkpoint file (coral sessions complete in one sitting)
- No formal escalation files — if a specialist says "the design is wrong," relay it to the user immediately
- No interface registry updates

## Output Expectations

1. What the specialist(s) said or did
2. Files changed, if any
3. What the user should follow up on

Keep it terse. Coral is the fast path.

## Dispatching Tips

- Minimum context to specialists. Over-briefing dilutes their specialty.
- Sequentialize only when one specialist's output feeds another's task.
- If a specialist asks for info you don't have, go to the user rather than guessing.
- Trust domain judgment. If the specialist says "this pattern is wrong for the stack," relay it instead of overriding.
- **Specialists give max scope; you pick min scope.** When a brief produces three deep, opinionated outputs, the synthesis temptation is to include all of it. Resist. Cut to MVP, mark the rest deferred with the trigger condition that would un-defer it.

## Multi-cycle iteration

Coral often runs multiple times against the same target — PR revisions, post-review fixes, validation rounds. That's expected and valuable:

- **Each cycle sees moved code.** Later rounds catch what earlier rounds couldn't because the diff has stabilized enough to read carefully. The "shipping CRITICAL bug" is more often a round-2 find than round-1.
- **When specialists disagree, reason from first principles.** Two deep outputs pulling in opposite directions is a real signal — don't pick by seniority or recency. State the tradeoff plainly and choose.
- **When a finding looks like a misread, verify before applying.** Specialists work from excerpts; sometimes they call out code that doesn't exist anymore. Read the file, check the test, and push back if the finding is wrong. Apply only what the code actually needs.
