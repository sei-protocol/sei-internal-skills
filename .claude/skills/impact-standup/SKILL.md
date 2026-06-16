---
name: impact-standup
category: project-management
model: claude-opus-4-8
description: >-
  Use when assembling a per-person talking-points agenda for a live team sync / standup
  from the Impact Hub pipeline — 'standup brief', 'team sync agenda', 'talking points for
  the team', 'what should we cover in Protocol sync', 'prep my standup', '/impact-standup'.
  Renders a terse, Slack-paste-ready brief per person: a substantiated SPINE (in-flight /
  in-review / just-shipped from Linear + PRs, each linked) plus a DISCUSSION LAYER (risks ·
  open decisions · blockers · asks · forward) that is clearly typed, may be link-light, and
  is mined-and-cited or PROMPTED from the human — never fabricated. Window is 'since last
  sync', not the ISO week, so live in-progress/blocked work still shows. ZERO-WRITE: renders
  to the conversation / markdown only; never writes a bet, the Impact Tracker, a Weekly
  Reports row, or a Linear label/status/comment — even an 'additive' one a manager asks for.
  Anti-triggers: NOT the retrospective per-bet exec entry that writes a bet's Weekly log
  (use /impact-weekly); NOT the cross-project exec roll-up that writes a report row (use
  /impact-portfolio); NOT a raw Linear status dump; NOT filing issues (/issue) or capturing a
  design (/design). Shares the /execution-plan betGraph read; identity is the bet's Notion
  page ID, never the slug.
---

# Impact Standup

The **agenda for a live sync**, not the record of a week. `impact-weekly` and `impact-portfolio` are the *retrospective, substantiate-or-refuse, write-the-board* tail of the Impact Hub loop. `impact-standup` is the **prospective, conversation-first, write-nothing** sibling: it turns the same `/execution-plan` `betGraph` read into a per-person talking-points brief for a standup or team sync, then gets out of the way.

The load-bearing genre rule comes straight from how good standups work: **the tracker is the record; the sync is for the agenda.** A standup is a *risk detector, not a status report* (ThinkLouder), and its job is to **"walk the board, not the people"** — surface the dependencies, decisions, and blockers that need a conversation, not round-robin completed work at a manager (Fowler, *It's Not Just Standing Up*). So this skill renders two layers and keeps them honest: a **substantiated spine** (what's live, lifted from Linear/PRs, each item linked — the *record*, referenced not re-narrated) and a **discussion layer** (risks · open decisions · blockers · asks · forward — the *agenda*, the actual value-add).

This is the suite's first non-writing projection authored as its own skill (the design doc — `docs/designs/impact-hub-pm-skill-suite.md` — names only weekly/portfolio/eoq; the only prior zero-write surface is `impact-portfolio --read-only`). It exists because the retrospective skills *refuse* exactly the lines a standup needs — an in-progress item, an open question, a blocker, a "behind on reviews" — and refuse to run on a "since last sync" window. This skill keeps those lines; it just never lets them be invented, and never writes them anywhere.

## Guardrails

This skill writes **nothing**. It renders a brief to the conversation (or a markdown/Slack artifact the user pastes). Before producing or acting on a brief:

1. **Zero-write — render only, persist nothing (anti-quiet-write).** This is a meeting artifact. It **never** writes a bet's Weekly log, the Impact Tracker, a Weekly Reports row, or a Linear label / status / comment — **not even an "additive," "reversible," or "low-risk" one, and not because a manager with write access asked**. Persisting the brief as a report row is `/impact-portfolio`'s job (behind its confirm gate); persisting per-bet progress is `/impact-weekly`'s (behind its). When asked to "just save / persist / drop this on the board," **refuse and redirect to the owning skill** — handing the rendered brief to the user to paste is the correct fulfillment, not writing it. Reversibility is **not** the bar; "this skill does not write" is.
2. **Spine substantiated; discussion layer typed, never fabricated (anti-fabrication).** Every **spine** bullet (in-flight / in-review / just-shipped) carries ≥1 evidence link from `betGraph`; a spine bullet with no link is **cut**. Every **discussion-layer** item is one of: (a) **mined** from a real source — a PR review state, a PR/ticket comment, a Slack message — and **cited**, or (b) a **prompt** for the human to fill, **clearly marked as a prompt**. It is never a risk/decision/blocker/status that you **inferred**. "She's probably blocked on review" is a *prompt*, not a bullet.
3. **Honest emptiness — no padding, no laundering carry-over as current (anti-status-theater).** If a person has no live work in the window, **say so**, and surface off-window / in-flight carry-over **clearly labeled as carry-over** — never present older or inferred work as this-window progress, and never invent talking points to fill a section. "Make them look productive / leadership is in the room" is the tell that you are serving manager-performance, not the team (the *reporting-to-the-leader* anti-pattern). A thin true brief beats a padded one — a fabricated bullet dies live under the first follow-up question, in front of the room, on the engineer.
4. **Agenda, not record — walk the board, don't re-narrate (anti-bloat).** The spine is the record; the tracker and PRs already hold it — **reference it tersely, don't restate it**. Terse by genre: per person a short theme header (`Name — theme, theme, theme`) + roughly ≤6 one-line bullets, ≤1 clause each, link-don't-inline. No PR/issue bodies, diffs, or logs. The discussion layer is where the words go, because that is what the sync is *for*.
5. **Attribution + ownership (anti-impersonation).** Mark the brief as an AI-generated draft for a sync. Discussion-layer items are talking points **about** a person's work for the meeting — they are **not** words authored in that person's name on a shared surface (that's both the zero-write line and a content-integrity one). Attribute items by `betGraph` assignee; when ownership is ambiguous, **prompt** — never guess whose item it is.

See `references/genre-and-rendering.md` for the record-vs-agenda doctrine (with sources), the render template, theme derivation, the discussion-layer mining-and-citation rules, and the honest-emptiness handling.

## Preconditions

- **`/execution-plan` `betGraph`** reachable (the shared Linear read) — interactively authenticated; may be absent headless. If absent, **say so and render only what's readable** — never fabricate the window.
- **Optional discussion-layer sources** for *light* mining: a PR host (e.g. GitHub) and/or Slack read. If absent, the discussion layer degrades to **prompts only** and the brief states that.
- A **roster** (persons or team) and a **window** — default **"since last sync"** (the last standup / last working day), **configurable; never the ISO calendar week**. The window is what makes in-progress / in-review / blocked work show up even when nothing *completed*.
- **No write surface is required or used** — the skill never writes Notion, the Tracker, or Linear.

## Procedure

1. **Resolve roster + window.** Persons or team, and the "since last sync" window (accept an explicit override; default to the last working day / last standup). State the resolved window. Do **not** silently use the ISO week — that drops live in-flight work (an engineer mid-task with no completion still has a standup).

2. **Read via `betGraph` — the one shared read.** Call `/execution-plan` `betGraph({persons | team, window})`; don't run a bespoke `list_issues`. Pull each person's **in-progress**, **in-review**, **just-shipped**, and **blocked** items with their linked PRs. Derive **theme tags** from their work's projects / labels / teams (see `references/genre-and-rendering.md`).

3. **Build the spine (per person).** In-progress · in-review · just-shipped, each as one terse bullet with its link. This is the record, referenced — not re-narrated.

4. **Light discussion-layer mining (optional, cited).** Scan linked PRs (review-requested / changes-requested / stale), PR + ticket comments, and recent Slack for **obvious** blocker / decision / risk signals. Include a mined item **only with a citation to its source**. Do **not** infer — absence of a signal is not a blocker.

5. **Prompt the human layer.** For each person, emit **clearly-marked prompts** for the discussion items not found by mining — risks · open decisions · blockers · asks · forward — for the person or EM to fill. Never fill a prompt with invented content; an unanswered prompt ships as a prompt.

6. **Render inline (zero-write).** Per-person sections: `Name — theme, theme, theme`, then spine bullets, then discussion bullets (mined = cited; unfilled = visible prompt). Apply honest-emptiness handling for anyone with no live work. Output **Slack-paste-ready markdown** (`[text](url)` links, `*bold*`/`_italics_`, no tables or `#` headings — see the rendering reference). Add a one-line coverage / read-degradation note.

7. **Hand off — no write, no confirm gate.** There is no destructive action, so there is no confirm step. Offer to give the user the markdown to paste, or — if they want it *persisted* — redirect to `/impact-weekly` (per-bet) or `/impact-portfolio` (report row), each of which carries its own draft→confirm→write gate. This skill stops at render.

## Rationalization Table

When your reasoning matches the left column, **stop**. The right column is the reframe.

| Excuse | Reality |
|--------|---------|
| "The manager has write access and asked me to persist the brief — just drop it in the Weekly Reports DB; it's additive and reversible." | Zero-write is the contract. Reversibility isn't the bar — this skill renders, it never persists. The report row is `/impact-portfolio`'s, behind its gate. Hand the brief over; don't write it. |
| "A new DB row is low-risk and self-owned; only the bet-page appends are the real risk." | Both are writes this skill doesn't make. Low-risk ≠ in-scope. Render only; redirect persistence. |
| "Masih has nothing this window — pull his recent work or infer from the release calendar so his section isn't empty." | Laundering carry-over or inference as current is status-theater and misattribution. Say it's quiet, surface labeled carry-over, never present old/inferred work as this-window progress. |
| "Leadership is in the room — give everyone a few solid talking points so it doesn't look slow." | The brief serves team coordination, not manager-performance (the *reporting-to-the-leader* anti-pattern). A thin true brief beats a padded one; a fabricated bullet fails live under a follow-up. |
| "I found no blocker for her, but she's probably blocked on review — add 'blocked on review'." | A discussion item is mined-and-cited or a prompt — never inferred. Emit the prompt; don't invent the blocker. |
| "Restate what shipped so the section looks complete." | The spine is the record; walk the board, don't re-narrate. Terse reference + link; the value is the agenda layer. |
| "It's faster to write the talking points straight into each engineer's Weekly log under their name." | That authors content in others' names on a shared surface, and it's a write. Not this skill — redirect to `/impact-weekly` (owner-confirmed). |
| "By ISO week the person has nothing, so they did nothing." | Wrong window. Standup is 'since last sync' and includes in-progress / in-review / blocked regardless of last-updated. Re-scope before declaring empty. |
| "Slack/PR mining is unavailable, so I'll just write plausible blockers." | No source → no mined item. Degrade to prompts and say mining was unavailable. Plausible ≠ true. |

## Red Flags — STOP

- "Just persist it / save a record / drop it on the board" → zero-write; hand it to `/impact-portfolio`.
- "Make everyone look productive" / "leadership is watching" → no padding.
- "Pull Masih's old work and call it this week" → no laundering carry-over as current.
- "Add the probable blocker / decision" → mined-and-cited or a prompt, never inferred.
- "Append it under each person's name" → that's a write, in others' names.
- "Use this week (ISO)" when prepping a sync → wrong window; it's 'since last sync'.

All of these mean: read via `betGraph` on the 'since last sync' window, build a linked spine, mine-and-cite or prompt the discussion layer, render terse and Slack-ready, and **write nothing**.

## Halt Conditions

Stop and surface to the user when:

- **`betGraph` / Linear is unavailable** — render only what's readable and say so; never fabricate the window's work.
- **Asked to write or persist anything** (a bet, the Tracker, a Weekly Reports row, a Linear label/status/comment) — refuse and redirect to `/impact-weekly` or `/impact-portfolio`; this skill never writes.
- **A discussion-layer item can't be cited and isn't marked a prompt** — cut it or mark it; never ship an inferred fact as a bullet.
- **The window can't be established** — ask; never silently fall back to the ISO week.

**Never pad a section to be helpful.** A brief that names a quiet week and a real blocker is the valuable output; a full-looking one built on inference is the failure this skill exists to prevent.

## What this skill doesn't do

- **Write anything.** Zero-write on Notion, the Tracker, report rows, and Linear. Persistence is `/impact-weekly` (per-bet exec entry) or `/impact-portfolio` (report row), each behind its own gate.
- **Author content in engineers' names** on a shared surface.
- **Fabricate or infer the discussion layer** — it mines-and-cites or prompts.
- **Replace the retrospective roll-ups** — it's the prospective agenda, not the week's record.
- **Decide scope or judge on-course / at-risk** — that's `product-manager` / the `technical-program-manager` agent. This skill assembles the agenda; humans run the sync.

## Output

The rendered per-person brief inline (Slack-paste-ready), the honest-emptiness notes for anyone quiet, the **unfilled discussion-layer prompts** the humans still need to answer before the sync, and any read degradation (e.g. "mining unavailable — discussion layer is prompts-only"). Never a write confirmation — there is no write.
