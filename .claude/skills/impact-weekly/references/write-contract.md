# Notion Write Contract

How `impact-weekly` writes to a bet — the only place it mutates Notion. The `update_content` surgical mechanism was **verified live against the Impact Tracker** (an append to one bet's Weekly log and a search-and-replace removal on another both succeeded); the idempotent in-place update on re-run uses the same `update_content` call. The Linear work-source tools (`list_issues`, `get_issue`, `list_issue_labels`) are part of the connected Linear MCP surface.

## What it writes — and only this

A single dated entry appended under the bet's **`Weekly log`** body section. Nothing else on the page is touched:

- **Never** the definition fields (*Why it matters* / *Success looks like*) — those are the owner's.
- **Never** the *End-of-quarter retrospective* (that's `impact-eoq`'s surface, phase 2).
- **Never** the *Overall Confidence* property — the draft may *suggest* a change ("2 weeks no movement on SEI-123 → consider At Risk"); the human sets it.

## Entry shape

Each week is a **collapsible toggle** — the leading `>` on the title line folds the entry under its week, so a bet's Weekly log stays scannable as entries accumulate:

```markdown
> **Week of <YYYY-MM-DD>**   ← the `>` makes it a toggle; the date is the Monday of the work window (deterministic; see Idempotency)
    <one-sentence executive outcome for this bet this week.>
    - <outcome bullet> — [SEI-123](linear-url) ([PR #456](pr-url))
    - <outcome bullet> — [SEI-789](linear-url)
```

- The outcome sentence and bullets are **nested inside the toggle** (indented beneath the `>` title), so they collapse with it. The title carries only the week — the substance is one fold away.
- One outcome line + one bullet per outcome; each bullet ≥1 evidence link and ≤1 context sentence. Cap the *narration*, not the number of outcomes — a heavy week keeps all its outcomes (each still one link), it does not pad prose or inline bodies.
- Links are the substantiation; never inline PR/issue bodies, diffs, or logs.

## Mechanism

- **Append** under the `Weekly log` heading via `notion-update-page` with `command: update_content` (search-and-replace on the heading), inserting the dated entry as a collapsible toggle (`>` title + nested body). This is surgical — it does not rewrite the page body. **Do not use `replace_content`** (it rewrites the whole page; the harness blocks it on shared exec pages, and rightly so).
- **Resolve the target page** by the engineer's `Person` + the bet identity (Notion **page ID** from the mapping cache), not by Name (Names are mutable).
- **Verify before write — a check, not a vow.** Immediately before the `update_content` call, re-fetch the resolved page by its page ID and assert (a) it is a row in the engineer's `Person`-scoped Impact Tracker and (b) the section being appended to is literally `Weekly log`. Refuse on any mismatch. This is the enforced form of the confirmed-target guardrail — added after a test agent once wrote to the wrong bet; re-stating "never" was not enough, so the target is re-verified at write time.

## Idempotency

The `Week of <YYYY-MM-DD>` **toggle title** is the idempotency key — the **Monday of the calendar week** being reported (ISO `YYYY-MM-DD`). The window is a **fixed calendar week, not a rolling 7 days** (see Procedure step 1): a re-run on any later day recomputes the *same* week's full set, so replacing the block in place only corrects or grows the entry and **never drops** work that a shifted rolling window would have excluded. Before writing:

1. `notion-fetch` the page; scan the Weekly log for an existing `Week of <date>` toggle (match the week in the title, regardless of whether a prior entry was a toggle or a plain heading — a pre-existing heading-style entry is replaced with the toggle form).
2. **Present** → replace that week's toggle in place (update, not duplicate). **Absent** → insert a new dated toggle under the heading.

Re-running the same week is therefore safe and converges. If **more than one** `Week of <date>` entry is found (e.g. a prior manual edit), **halt and surface** rather than guess which to update.

The **live page is authoritative.** A local `state/posted-<user>-<weekISO>.json` records `{pageId, weekKey}` as an advisory backstop for mid-failure re-runs (gitignored), reconciled against the page every run — if the file says "posted" but the page lacks the toggle, the page wins and the entry is (re)written.

## Draft → confirm → write

1. Render every entry + the **exact target page URL** it will append to.
2. Require explicit confirmation (echo the targets so a wrong bet is visible before any write).
3. Write per confirmed page; echo each resulting URL.

Never auto-write, even from a rich handoff. The confirm gate is the safety that keeps a mis-mapped entry off a shared exec page.

## Degradation

If `notion-update-page` fails or the MCP is absent: report what was attempted and stop. Never claim a write that didn't happen; never fabricate a page URL.
