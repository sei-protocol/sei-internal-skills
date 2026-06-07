# Notion Write Contract

How `impact-portfolio` writes — the only place it mutates Notion, and it mutates **only its own report page**. It is strictly **read-only on every bet page**. Read alongside `selection-and-coverage.md` (what goes in the report) and the SKILL.md procedure.

## What it writes — and only this

One page per week, under the **"Weekly Reports"** section (a plain toggle heading, confirmed) on the Impact Hub page `35edb6ff605780b6b023d95456209168`. Nothing on any bet page is ever touched.

## Identity — the ISO Monday, as a page property (never the title)

- **Display title:** `Impact Report - Week of <Month Xth>` — human-facing only.
- **Join key:** the week's **ISO Monday** (`YYYY-MM-DD`), written as a page **property** (`report_week`). The display title is **never** the match key: ordinal suffixes, locale, and date formatting drift across runners and would produce duplicate pages. This mirrors `impact-weekly`'s ISO-key discipline ("the alias is never the join key").
- **Timezone:** "the week's Monday" is computed against a **single declared TZ** (document it in the run; do not use the runner's local TZ implicitly) — otherwise a Sunday-night/Monday-morning run resolves to different Mondays for different runners.

## Provenance — prove ownership before any overwrite

On **create**, write a provenance marker the skill can later assert: a `generated_by: impact-portfolio` page property (alongside `report_week`). Before **any** update/replace, re-fetch the target and assert **both** `generated_by == impact-portfolio` **and** `report_week == thisMonday`. If either is absent or mismatched, **halt** — do not write. This defends against:

- a human creating a page with a colliding title under "Weekly Reports" (title match would resolve it; the marker won't),
- clobbering a human's mid-week edit to the report page (surface it; don't silently replace).

"We own the page" is only true if the marker says so — a naming coincidence is not an ownership boundary.

## Mechanism (spike before first live run)

The only Notion write **verified live in this repo** is `notion-update-page` `update_content` (surgical search-and-replace; `validate-release` additionally creates pages under a *database*). Creating a sub-page **under a toggle block** with a **custom property** is unproven. **Spike these before relying on them:**

1. Can `notion-create-pages` parent a page under the "Weekly Reports" **toggle/heading block** (vs. only a page or database)? **Fallback:** create under the Impact Hub **page** and append a link-to-page block inside the toggle via `update_content`.
2. Does `notion-create-pages` accept **nested block content** (headings + linked bullets) and a **custom page property** in one call?
3. Does the Notion MCP resolve `Person` user-ID → **display name**? (Else render the raw ID, flagged.)
4. Do **in-page anchor links** render (exec-summary bullet → its section)? (Else name the section in text.)
5. Is a **full-body replace** allowed on this skill-owned page, or must updates use `update_content` section-replace? Lead with the verified `update_content` path if create/replace can't be confirmed.

Record the spike outcome; the build issue (Tide #119 / PLT-437) tracks it as task 1.

## Idempotency

The **ISO `report_week` property** is the idempotency key — the Monday of the reported calendar week, the **same** Monday-anchored week `impact-weekly` keys its toggles on. Before writing:

1. `notion-fetch` the "Weekly Reports" section; enumerate child pages; match on `report_week == thisMonday` (not the display title). Re-scan **immediately before** the create (TOCTOU narrowing — single-runner convention bounds, but does not eliminate, the race).
2. **Absent →** create (write `report_week` + `generated_by`). **Exactly 1 + marker asserts →** update in place (pulls in toggles added since the last run). **>1, or marker mismatch →** halt and surface.

Re-running the same week is safe and converges. A local `state/report-<weekISO>.json` records `{pageId, weekKey}` as an advisory backstop for mid-failure re-runs (gitignored); on divergence between the backstop and the live lookup, **halt** rather than letting the (compromised) lookup override — the live page is authoritative for *content*, but identity is the property + marker.

## Confirm the destructive action — not just the prose

`impact-weekly`'s confirm gate reviews the draft body. This skill's gate must additionally surface the **destructive action**, because an update *replaces* an existing page:

- the resolved target page id and URL,
- its **last-editor** and last-edited time,
- the decision: **create** (absent) vs **replace** (exactly-1-marker-asserts) vs **halt** (>1 / mismatch).

A gate that confirms wording but hides "this replaces page X last edited by a human on Tuesday" is confirming the wrong thing.

## Partial-fetch manifest

Track every source (bet toggle fetch, Linear scan) that failed. Render a footer: `Read N of M sources; could not load: <bet>, <bet>.` A partial run is **never** presented as complete — half a portfolio silently omitted is indistinguishable from "those projects were quiet." If the runner won't accept a manifest-flagged partial, halt.

## Draft → confirm → write

1. Render the full page + the exact parent + the destructive-action summary.
2. Require explicit confirmation (the clobber is visible before it happens).
3. Write per the resolved decision; echo the resulting URL.

Never auto-write. The confirm gate is the safety that keeps a duplicate or a clobber off the shared exec board.

## Degradation

If `notion-create-pages`/`update_content` fails, the MCP is absent, or sources fetched only partially and the runner rejects a flagged partial: report what was attempted and stop. Never claim a write that didn't happen; never fabricate a page URL, a section, an owner, a confidence, or a link.

## One-way doors (persisted to the shared board — explicit human approval)

- The **title convention** and the **ISO-`report_week` identity scheme** (changing either forks all history).
- **Where pages live** (children of "Weekly Reports").
- The **full-body-replace contract** + the **provenance model** (consumers come to rely on the page being machine-managed).
