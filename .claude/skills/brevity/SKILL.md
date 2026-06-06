---
name: brevity
model: claude-opus-4-8
description: "Use when authoring or revising agent-produced PR descriptions or in-code comments — 'tighten this', 'compress this PR body', 'this is too verbose', 'cut the filler', '/brevity'. Also fires when an agent is about to write a PR body or any in-code comment containing WHY-style narration. Anti-triggers: NOT for compressing memory files / CLAUDE.md (one-shot migration concern); NOT for chat output mid-conversation (soft-guidance via authoritative-voice patterns); NOT for commit messages (Conventional Commits convention covers it); NOT for source code itself or docstring formatting (use gofmt, prettier, golint). For multi-dimensional PR-time critique beyond brevity (convention adherence, defensive code, missing tests), use /pr-quality."
---

# Brevity

Agent-authored natural-language output drifts toward verbose by default. RLHF rewards thoroughness; instruction-following promotes preamble; uncertainty triggers hedging; a corpus dominated by tutorial prose biases comments toward "what?" narration over "why?" insight. This skill exists because brevity discipline cannot survive on memory soft-guidance alone — it gets bypassed the moment an agent is under pressure.

This skill enforces brevity-with-signal on two surfaces: **PR descriptions** and **in-code comments**. Other surfaces (doc/design, runbooks, memory writes, mid-conversation chat) are deferred — call back when one of them produces a real drift incident.

## Guardrails

This skill operates on **agent-authored natural-language output**. Before any side-effecting suggestion or rewrite:

1. **Surface check.** Confirm the output is one of the MVP surfaces (PR body, in-code comment). If the user is asking about a deferred surface, redirect (`feedback_authoritative_voice` for chat output; Conventional Commits for commit messages; gofmt/prettier for code formatting).

2. **Authorship check.** This skill enforces brevity on *agent-authored* content. If compressing human-authored content, surface that and ask whether the user wants editorial suggestions (allowed) or rewrite (requires explicit consent — humans own their words).

3. **Refuse-or-halt conditions.** This skill stops and surfaces when:
   - **Input is below the floor** — PR body <50 words OR comment <1 line. Already brief; further compression is bikeshedding.
   - **Load-bearing claim is unclear** after one careful read — the input is too garbled to compress; suggest a rewrite from scratch.
   - **Section is safety-critical** — `## Safety`, `## Rollback`, `## Blast radius`, `## Migration`, `## Breaking change`. Compression within these sections requires explicit user opt-in.
   - **Compression would lose signal** — if applying the rules removes information a reviewer needs to act, stop.
   - **Discipline is being litigated** — user pushes back on three or more rule applications in one session. Stop and surface the disagreement; let the user redirect.
   - **Request is a mechanical lint** — "ban these words" / "regex replace X with Y" → route to lint tools (write-good, proselint, CI step), refuse to act as a banned-word filter.
   - **Content was authored by someone other than the user** — cross-author PR edits require all three: (a) the user explicitly names the target PR by URL or `owner/repo#N`, (b) the user states the relationship (reviewer suggestion vs. author handoff), (c) the output routes as a suggestion (not a direct edit applied to the PR body).
   - **Content is already committed/merged** — suggestions only; the user owns the diff.

## The Eight Rules — YOU MUST

These rules are non-negotiable. They are not stylistic preferences; they are the failure modes the skill exists to close.

**Rule 1 — Cut every sentence whose removal does not change what a reviewer would do next.**

If a sentence describes the change rather than informing a decision, delete it. The diff describes the change. The PR body informs the reviewer.

❌ "This PR introduces a small but important refactor to the validator initialization logic."
✅ (delete entirely — the diff title + linked issue carry this)

**Rule 2 — Open a PR body with the *what* and *why*, not a wind-up.**

No "In order to...", "The motivation for...", "This PR / This change...", "We are...". Start on the load-bearing noun.

❌ "In order to address the issue in #289, this PR updates the reconciler..."
✅ "Reconciler now waits for StatefulSet pods Ready before stamping `status.allocatable`. Fixes #289."

**Rule 3 — Keep in-code comments to 4 lines or fewer.**

Long WHY narration belongs in the PR description, recoverable via `git blame → PR`. If a comment is longer than the code it annotates, it's too long.

❌ Twelve-line block explaining the original bug, why DeepEqual fails, alternatives considered, rationale, future-engineer warning.
✅ "Both checks required: reflect.DeepEqual gives false-positives on equal-but-reordered maps (#241). Hash short-circuits update storm. Don't drop either."

**Rule 4 — Make every verb do work.**

Sentences using "serves to", "aims to", "helps to", "is responsible for", "allows us to", "exists to" must be rewritten with the real verb as the main verb.

❌ "This function serves to validate the input."
✅ "Validates the input."

**Rule 5 — Do not restate names.**

A comment paraphrasing an identifier (`// Slug is the engineer's identifier` on field `Slug string`) gets deleted, not shortened. The name *is* the documentation.

❌ `// Hash returns a hash of the spec.`
✅ (delete — the function signature says this)

**Rule 6 — Prefer one example over one paragraph of explanation.**

If you're describing behavior abstractly, replace the paragraph with the smallest concrete input/output pair.

❌ "The function applies the join by matching the pod and namespace labels, then propagating the workload label via group_left semantics."
✅ "`pod_alerts * on (pod, namespace) group_left(workload) kube_pod_info` → series gains `workload` label."

**Rule 7 — Collapse hedges.**

"Generally", "typically", "in most cases", "it should be noted that", "essentially", "basically" carry no information for engineering readers. Cut or commit.

❌ "It should be noted that the timeout is essentially a safety net."
✅ "Timeout is a safety net."

**Rule 8 — Treat headers and bullets as a budget, not a structure tax.**

A PR body with three single-bullet sections is three sentences pretending to be a document. Flatten it. Use a section only when it groups ≥2 related items.

❌
```
## Summary
- One sentence.

## Context
- One sentence.

## Files
- One file.
```
✅ "One sentence summary. One sentence context. One file."

## Target shapes — concrete

**PR body word count:**
- Fix / small refactor: **30-80 words**
- Feature / behavior change: **100-250 words**
- One-way door (interface, storage, event sig): **+50-100 words** for explicit justification + alternatives considered

**PR body section pattern:**
1. Summary — 1-3 sentences leading with what + why
2. Test plan — checkbox list, 2-5 items
3. Follow-ups — optional bullets
4. Refs — `Fixes #N`, design link

**Does NOT belong in a PR body:**
- Motivation paragraphs that restate the linked issue
- "What each file does" walkthroughs (the diff shows it)
- Background sections (link the design doc)
- "Things I considered but didn't do" (out-of-scope already documented)
- Marketing language
- Re-derived shared context

**In-code comment line count:** 2-4 lines (1 line OK for trivial WHYs).

**Comment earns its place:**
- Non-obvious WHY — invariant the code preserves but doesn't express
- Hidden constraint — upstream bug, ordering requirement, ABI quirk
- Workaround context — pointer to issue/PR
- Magic number provenance — "5s = p99 + 2× jitter, see #1234"

**Comment does NOT earn its place:**
- Restating function/variable name
- Describing what the next lines plainly do
- History ("used to be X, now Y" — that's `git log`)
- TODOs without owner or issue link
- Defensive narration ("we now check the err")

**Reference PR**: [rust-lang/rust#157179](https://github.com/rust-lang/rust/pull/157179) — 28 words, optimizes impl sorting. Says *what*, *why*, *where to look for more*. Tide adds a 2-bullet test plan to this floor.

## Procedure

1. **Surface check.** Confirm the input is a PR body or in-code comment. If neither, halt with the redirect from Guardrails.

2. **Announce the rules being applied.** State: "Applying brevity rules N-M to surface S." This is the Commitment principle — naming the rules forces explicit application rather than silent partial-skip.

3. **Pass 1 — cut filler and hedges (rules 1, 7).** Walk every sentence. Delete those that describe without informing. Cut every hedge ("generally", "typically", "it should be noted", "essentially").

4. **Pass 2 — collapse abstractions (rules 4, 6).** Rewrite weak-verb sentences ("serves to", "aims to", "is responsible for") with the real verb as main verb. Replace abstract-description paragraphs with concrete examples.

5. **Pass 3 — trim structure (rules 5, 8).** Delete name-restating comments. Flatten sections that contain only 1 item. Collapse a sub-bullet that's the only child of its parent.

6. **Pass 4 — verify openers and length (rules 2, 3).** Check the PR body opens on the load-bearing noun (not "In order to" / "The motivation"). Check comments are ≤4 lines.

7. **Show the diff.** Before/after with word counts. The user verifies the compression preserved the load-bearing claim.

8. **Report guardrail catches.** If any section was refused (safety-tagged, comprehensibility loss, etc.), name it explicitly so the user can override if they disagree.

## Rationalization Table

The skill must hold under pressure. The five rows below are the rationalizations that fired in the RED-phase pressure test against a Tide-flavored verbose-PR scenario (time + authority + sunk-cost + cold-reviewer). Each is paired with the counter. The full table with sources and 5 additional secondary rows lives in [`references/rationalization-table.md`](references/rationalization-table.md).

| Rationalization (the agent's excuse) | Reality (the skill's counter) |
|---|---|
| "The lead said 'be thorough.'" | "Thorough" means complete on the load-bearing claim, not long. A 200-word PR body that says what changed and why is more thorough than a 700-word PR body that says it twice with filler. Authority pressure does not override Rule 1. |
| "The reviewer will be cold on Monday — they need context." | The linked issue, the design doc, and the commit messages carry context. The PR body is the index, not the encyclopedia. If a reviewer cannot orient from the index, the index is broken — fix the structure, not the length. |
| "This change is complex, so the explanation must match." | False symmetry. Complexity of the *change* is independent of complexity of the *explanation*. The Linux kernel ships 10,000-line patches with 50-word commit messages. Compression is a separate skill from comprehension. |
| "Adding context can't hurt; omitting might." | Asymmetric loss perception. Adding 400 words of filler hurts every future reviewer who has to skim it. The cost is real, repeated, and uncompensated. RLHF never penalized verbosity in training; the skill does. |
| "The PR body is the durable record — be complete." | The PR body is the index. The commit history, the design doc, the linked issue, and the diff are the record. Treating the PR body as the design doc is genre confusion — it creates the redundancy the team complains about three months later. |

## Red Flags — STOP and rewrite

If any of these phrases appear in your own draft, **stop** and apply the rule:

- **"In order to..."** — Rule 2. Cut. Start on the load-bearing noun.
- **"The motivation for this change is..."** — Rule 2. Cut. The motivation is in `Fixes #N`.
- **"This PR / This change..."** — Rule 2. Cut. The diff is the PR.
- **"It should be noted that..."** — Rule 7. Cut. Just say the thing.
- **"Essentially / basically / generally..."** — Rule 7. Cut or commit.
- **"X serves to / aims to / helps to / is responsible for..."** — Rule 4. Rewrite with X as the subject performing the real verb.
- **"Things I considered but didn't do..."** — Rule 1. Out-of-scope already lives in the issue. If a specific alternative deserves justification, fold it into the Summary in one sentence.
- **"For context / for background / for those unfamiliar..."** — Rule 1. Cut. Link the design doc.
- **Three sections with one bullet each.** — Rule 8. Flatten to prose.
- **A comment that paraphrases the identifier above it.** — Rule 5. Delete.

**All of these mean: Stop. Apply the rule. Don't rationalize the exception.**

## What this skill doesn't do

- **Compress source code, docstrings, or variable names.** That's gofmt/prettier/golint territory.
- **Compress memory files or CLAUDE.md.** One-shot migration concern; deferred.
- **Compress chat output mid-conversation.** `feedback_authoritative_voice` covers this softly today.
- **Compress commit messages.** Conventional Commits convention covers this.
- **Compress human-authored content without explicit consent.** Surface and ask.
- **Replace `/pr-quality` (issue #88).** For multi-dimensional PR critique (convention adherence, defensive code, missing tests), use /pr-quality. This skill owns the brevity dimension only.

## References

- [`references/rules.md`](references/rules.md) — the 8 rules with multiple before/after examples per rule
- [`references/rationalization-table.md`](references/rationalization-table.md) — full table with sources for each underlying bias
- [`references/target-shapes.md`](references/target-shapes.md) — PR body word-count distributions, comment exemplars, the rust-lang/rust#157179 reference
- [`references/guardrails.md`](references/guardrails.md) — detailed safety model

## Output

After a compression pass, the skill shows:

1. **Before** word/line count
2. **After** word/line count
3. **Diff** (the actual rewrite)
4. **Rules applied** (which numbered rules fired)
5. **Guardrail catches** (anything refused, with reason)

Example:
> Before: 150 words. After: 58 words. Rules applied: 1, 2, 7. No guardrail catches. (Body now opens on the load-bearing noun; motivation paragraph cut; "small but important" / "In order to" / "The motivation for" filler removed.)
