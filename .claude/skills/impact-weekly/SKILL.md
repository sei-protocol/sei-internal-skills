---
name: impact-weekly
model: claude-opus-4-8
description: "Use when rolling up an engineer's week of work into the matching Impact Hub bet as an executive progress update — 'impact weekly', 'weekly impact update', 'roll up my week into Impact Hub', 'update my impact bet', 'Friday impact update', '/impact-weekly'. Queries the engineer's Linear week (+ linked PRs), maps each item to its bet, drafts a substantiated exec entry, and on confirmation appends it to that bet's Weekly log. Anti-triggers: NOT for filing issues/tasks (use /issue); NOT for capturing a design (use /design); NOT for the across-team portfolio scan or the end-of-quarter per-engineer rollup (deferred phase-2 impact-portfolio / impact-eoq); NOT for editing a bet's definition fields (Why it matters / Success looks like — the owner writes those); NOT for setting Overall Confidence (it only suggests)."
---

# Impact Weekly

Turns an engineer's week of real work into a substantiated, executive-summary progress entry on the right Impact Hub bet — fast enough to actually happen every Friday. It reads the work from Linear (the source of truth), maps it to the bet, drafts a tight entry where every claim links to its evidence, and writes only after you confirm.

The genre rule, and the whole point: **the Impact doc is the index; Linear and the PRs are the record.** An entry that tries to *be* the record is the failure this skill exists to prevent.

This is the synthesis tail of the work loop in `docs/designs/impact-hub-pm-skill-suite.md`. It is the one skill that writes to a shared, exec-facing Notion page — so it is deliberately narrow and confirmation-gated.

## Guardrails

This skill writes exactly one thing: a dated entry under the **Weekly log** of a bet you have explicitly confirmed. Before any write:

1. **Confirmed target only — and verify it as a check, not a vow.** Append solely to the Weekly log of the bet the user confirmed this run. **Never** write to a different bet, **never** edit the definition fields (*Why it matters* / *Success looks like*) or the *End-of-quarter retrospective*, and **never** set/flip *Overall Confidence* (suggest it, the human sets it). A write to any page that isn't the confirmed target is the top refusal — it is how an agent silently corrupts a shared exec board. Re-stating "never" is what failed once; so **immediately before any write, re-fetch the target by its cached page ID and assert (a) it is a row in the engineer's `Person`-scoped Impact Tracker and (b) the section being appended to is literally `Weekly log` — refuse the write if either assertion fails.**
2. **Coverage before write (anti-mis-tracking).** Reconcile the week's work against the engineer's owned bets. If a worked-on item maps to no bet, or an owned bet got work but no entry, **report the gap — never silently write a subset, and never attribute work to a bet it doesn't belong to.** Mapping is label-first (`impact:<slug>`); untagged work falls to a name-match that **requires human confirmation**. The join identity is the bet's immutable Notion **page ID**, never the slug.
3. **Substantiate or refuse (anti-unsubstantiated).** Every claim carries ≥1 link to its Linear issue / PR, resolving to *this* engineer's work in *this* period. A claim with no evidence is **refused, not softened** — surfaced as "unsubstantiated: cite or cut." Confidence is read from actual issue state, never from the engineer's say-so, and is only ever *suggested*.
4. **Index, don't inline (anti-bloat).** The ceiling is on *narration*, not on the number of substantiated outcomes: one outcome line + **one bullet per outcome, ≤1 context sentence each**, no inlined PR/issue bodies, diffs, or logs. A heavy week keeps all its outcomes (each still one link); it does not pad prose. Apply these rules directly as this skill's own check — **`/brevity` is scoped to PR bodies and in-code comments and does not operate on a Notion entry**, so don't defer to it: cut anything the linked title already says, write one sentence that informs rather than narrates, never restate the record. "Leadership wants to see everything" and "more detail = more credit" are bloat rationalizations; the links carry the depth.
5. **Draft → confirm → write; degrade, don't fabricate.** Always render the entries and the exact target page(s), get explicit confirmation, then write. Never auto-write. If Linear or the Notion write surface is unavailable, **say so and stop** — never invent an entry, a link, or a Notion/Linear identifier.

See `references/write-contract.md` for the append/idempotency mechanics and `references/mapping-and-coverage.md` for the label-first mapping, name-match fallback, and coverage gate.

## Preconditions

- **Linear MCP** connected (`list_issues`, `get_issue`, `list_issue_labels`) — the work source. Interactively authenticated; may be absent in headless runs (then halt, don't fabricate).
- **Notion MCP** connected with the block-append/update surface (`notion-fetch`, `notion-update-page` with `update_content`) — verified against the Impact Tracker.
- The engineer's Linear identity (assignee) and the week window (default: the current ISO calendar week, Mon→now; not a rolling 7 days).

## Procedure

1. **Resolve engineer + period.** Linear assignee (`me` or named) and the window — **default the current ISO calendar week (Monday 00:00 → now), not a rolling "last 7 days."** A fixed calendar week keeps the `Week of <Monday>` idempotency key stable: a re-run on any later day recomputes the *same* week's full set, so the in-place replace only corrects/grows the entry and never drops early-week work. Accept an explicit week override. Output: a concrete `(assignee, since=that Monday, until)`.

2. **Gather the week's work.** Query Linear for the engineer's work in the `[since, until]` window — the **union** of issues *updated* in-window (`updatedAt` within `[since, until]`) and issues *completed* in-window (`completedAt` within `[since, until]`), so a ticket finished this week but last touched earlier isn't missed. Bound both ends — never roll up work outside the chosen week. For each issue, read its linked PRs. **PR→issue linkage is config-contingent** (Linear's GitHub integration) — when absent, substantiation degrades to issue-only; say so, don't silently drop PRs. Output: the week's issues with evidence links.

3. **Map work → bets.** Label-first: group issues by their `impact:<slug>` label and resolve label → bet **page ID** (cached). Untagged issues → name-match against the engineer's `Person`-scoped Impact Tracker rows and **ask the user to confirm the proposed mappings — batched in one prompt** (see `references/mapping-and-coverage.md`) — before they count. Unmapped items surface as "assign or skip" — never dropped. Output: `{bet page ID → [issues]}` + an unmapped list.

4. **Coverage check.** Reconcile mapped work ↔ the engineer's owned bets. Surface: bets that got work, bets that got none, and any work that mapped to no bet. **If there's a gap, surface it and get the user's call before writing** — never write a *silent* subset. Once the user has assigned or explicitly skipped the gapped items, the confirmed bets proceed and the skipped/unmapped items are reported as still-open (step 8). Output: a coverage statement the user signs off on.

5. **Draft per-bet entries.** For each bet with work, write one exec-summary entry: a one-line outcome + one bullet per outcome, every bullet ≥1 evidence link and ≤1 context sentence. Apply the anti-bloat check (Guardrail #4) — cut restatement, no inlined bodies, cap narration not outcome count. If issue state warrants, *suggest* a confidence (e.g., "2 weeks no movement → consider At Risk") — never set it. Output: drafted entries, tight and fully substantiated.

6. **Confirm.** Show each entry and the **exact target page** it will append to; require explicit confirmation. On edits, loop back to step 5. Output: user's go/no-go per bet.

7. **Write.** For each confirmed bet, **first re-verify the target** (re-fetch by page ID; assert it's the engineer's Impact Tracker row and the section is `Weekly log` — refuse on mismatch). Then append a dated entry under its **Weekly log** as a **collapsible toggle** (`>` on the `Week of <YYYY-MM-DD>` title so it folds — keeps the log scannable as entries grow), idempotent on that toggle title (the **Monday of the work window**, deterministic — see `references/write-contract.md`; update in place on re-run; if more than one matching entry exists, halt and surface rather than guess). Touch nothing else on the page. Echo each page URL written.

8. **Report.** One line per bet written (with URL), the unmapped/uncovered items still open, and any suggested confidence changes for the human to apply.

## Halt Conditions

Stop and report rather than proceeding when:

- The Linear or Notion MCP is unavailable — never fabricate the week's work or a Notion write.
- Work maps to no bet, or an owned bet's coverage is ambiguous — surface for human assignment; don't guess.
- A name-match mapping isn't confirmed by the user — an unconfirmed bet attribution is never written.
- A drafted bullet can't be substantiated with a real link — refuse the bullet; don't soften it.
- The user declines the confirm prompt — that's a valid stop. Don't write.
- The only target you can resolve is a bet other than the engineer's confirmed one — never write to it.

## Rationalization Table

Pressure patterns captured from baseline testing, and the counter. When your reasoning matches the left column, **stop**.

| Excuse | Reality |
|---|---|
| "They're in a hurry and said to roll it all under one bet — respect their autonomy." | The bet's readers rely on accuracy; the engineer can't consent on their behalf. Map each item to the bet it actually belongs to, or surface it unmapped. |
| "A migration touches adjacent infra, so the cleanup and the incident are 'related' enough." | That's inventing a mapping to avoid checking. Unrelated work goes to its own bet or stays unmapped — not laundered under a convenient one. |
| "Post it all under one bet and quietly add the others too — split the difference." | Half-laundering is still laundering. Each item is attributed to its real bet or none. |
| "It's just an internal status field, low stakes." | Confidence + progress drive staffing, priority, and credibility decisions. "Make it look productive" is the tell that it's a misreport. |
| "Leadership wants to see everything, so inline all the PR descriptions." | The entry is the index, not the record. Inlining bodies is bloat; the links carry the depth — one sentence per outcome, the rest is a click away. |
| "Restating what the PR/issue says shows depth." | It's redundant filler. If the linked title already says it, cut it (brevity rule 1). |
| "A longer entry signals a productive week." | Length is not effort. An exec scanning many bets is served by the tightest true entry, not the longest. |
| "The engineer says it's on track — that's enough to set On Track." | Their memory of progress isn't verified remaining scope, and you'd be publishing the claim. Read issue state; suggest, don't set. |
| "Leadership doesn't read the linked tickets anyway, so skip the links." | The links are the audit trail that makes a claim verifiable, not reading material. Skip them and "major progress" is a vibe. |
| "They said keep it high-level, so I don't need to cite specific issues." | High-level *tone* ≠ unsubstantiated. Tight prose, hard links. |

## Red Flags — STOP and Reset

- "Just roll it all under [one bet]" / "put it all there to save time"
- "Make it look productive" / "show the depth of everything I did"
- "Set confidence to On Track" (from say-so, not issue state)
- "Leadership doesn't read the tickets" / "skip the links"
- "Be thorough and comprehensive" → inlining bodies
- "I'm heading out, just post it" → skipping the coverage check or the confirm gate

All of these mean: map to the real bet, substantiate every claim with a link, keep it to the index, and write only the confirmed target after confirmation.

## Output

End-of-session summary: one line per bet written (title + page URL), the unmapped or uncovered items still needing a human call, and any confidence changes suggested for the owner to apply. If nothing could be written (MCP unavailable, coverage gap, declined confirm), say so plainly — a clean stop beats a fabricated update.
