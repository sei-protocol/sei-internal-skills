# Genre, rendering, and the discussion layer

This reference backs `SKILL.md`. It holds the record-vs-agenda doctrine (with sources), the render template, theme derivation, the discussion-layer mining-and-citation rules, and honest-emptiness handling. The skill body is the contract; this is the how.

## The doctrine: the tracker is the record, the sync is the agenda

A standup's value is **not** reciting completed work. The four real functions are sharing goals, coordinating effort, surfacing problems, and team identity (Fowler, *It's Not Just Standing Up*) — none of which need a status recitation. The named failure modes this skill is built to avoid:

- **Runners, not the baton** (Fowler) — celebrating individual activity instead of the flow of work. Corrective: **"walk the board"** — step through work items and their dependencies, not people.
- **Status report instead of risk detector** (ThinkLouder, *Daily Scrum patterns*) — the round-robin "yesterday / today / no blockers" script drifts into performance and stops the team thinking about what could go wrong.
- **Reporting to the leader** (Geekbot; dev.to, *Detangling the Standup*) — members address the manager, not each other; the real audience becomes management, and the "real standup" happens in the hallway after.

So the skill renders two layers and never collapses them:

- **Spine = the record.** What's live, lifted from Linear/PRs, each item **linked**. Referenced tersely, **never re-narrated** — the tracker already holds the detail.
- **Discussion layer = the agenda.** Risks · open decisions · blockers · asks · forward. This is the value-add — the conversation-worthy items the sync should "walk."

Async-standup norms set the form (Range; Teaminal): **terse** (a few one-line bullets), **link to the artifact rather than restate it**, and **flag what needs discussion** rather than resolving it inline.

Sources: Fowler, *It's Not Just Standing Up* (martinfowler.com); ThinkLouder, *Daily Scrum Standup Patterns That Surface Risk*; Geekbot, *Daily Standup Anti-Patterns*; dev.to, *Detangling the Standup*; Range, *Asynchronous Daily Standups*; Teaminal, *Async Standups*.

## Render template (Slack-paste-ready)

Markdown that pastes cleanly into the Slack composer (validated): `[text](url)` links convert; `*bold*` / `_italics_` convert; **avoid tables and `#` headings** (the composer renders them raw). Use bold lines as section headers and `-` bullets.

```
*🗣️ <Team> — Standup / Sync agenda (<window>)*
_AI-generated draft for the sync · spine = linked record · ⟨prompt⟩ = fill before the meeting_

*<Name> — <theme>, <theme>, <theme>*
- <in-flight / in-review / just-shipped, one line> — [PLT-123](url)
- ⚠️ blocker: <mined blocker, one line> — [source](url)        ← mined + cited
- ⟨decision needed: …?⟩                                          ← unfilled prompt
- ⟨ask / risk / forward: …⟩
```

- **Theme header** (`Name — theme, theme, theme`): 2–4 short domain tags, **derived** from the person's in-window work — their issues' projects, labels, and teams (e.g. a person with Observability-Plane + release issues → "observability, release"). Derive, don't editorialize.
- **Spine bullets**: in-progress · in-review · just-shipped, each with its `betGraph` link. One clause each.
- **Discussion bullets**: mined items carry a `[source](url)`; unfilled items render as a **visible prompt** in `⟨…⟩` (or your chosen marker) so the person/EM fills them before the sync. An unanswered prompt **ships as a prompt** — it is never silently dropped or silently filled.

## Discussion-layer sources (mine-and-cite, else prompt)

Light mining only — surface **obvious** signals, each cited:

- **PR review state**: `review-requested`, `changes-requested`, or stale-open PRs → an "in review / blocked on review" item, citing the PR.
- **PR / ticket comments**: an explicit "blocked on X", "need a decision on Y", "waiting for Z" → a blocker/decision item, citing the comment.
- **Recent Slack** (if connected): an explicit blocker/decision/ask the person posted → cite the message.

Rules:
- **Absence of a signal is not a blocker.** If mining finds nothing for a category, emit a **prompt**, not an inferred item.
- **Never infer** a risk/decision/blocker/status from the release calendar, the roadmap, or "what they're probably doing." That is the honest-emptiness violation in disguise.
- If a mining source is unavailable, the discussion layer is **prompts-only** and the brief says so.

## Honest-emptiness handling

For a person with no live work in the window:

- Say it plainly: `*<Name> — quiet window*` / "No live tracked items since <last sync>."
- If there is genuine off-window or off-Linear context, surface it **clearly labeled as carry-over**, e.g. `_carry-over (not this window):_ owns the v6.6 release (feature freeze Mon); tracked off-Linear`. Label is mandatory — it must not read as this-window progress.
- Optionally add a single prompt: `⟨check-in: blocked? context-switched? off the board?⟩`.
- **Never** pull older completed work and present it as current; **never** infer current work from a calendar; **never** invent bullets to fill the section. A quiet week, stated honestly, is useful signal for the sync.

## Window

Default **"since last sync"** (the last standup, or the last working day) — **configurable, never the ISO calendar week**. The window must include **in-progress / in-review / blocked** items regardless of `updatedAt`, because the standup cares about what's *live*, not what *completed*. (This is exactly why an engineer with no ISO-week completions still has real standup content.)
