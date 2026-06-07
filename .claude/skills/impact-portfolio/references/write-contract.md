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
6. Can the MCP read **`last_edited_by`** (and a settable custom property) on a page? The destructive-action summary needs the last-editor; if it can't be read, a replace must **halt** rather than overwrite blind.

Record the spike outcome; the build issue (Tide #119 / PLT-437) tracks it as task 1.

## Idempotency

The **ISO `report_week` property** is the idempotency key — the Monday of the reported calendar week, the **same** Monday-anchored week `impact-weekly` keys its toggles on. Before writing:

1. `notion-fetch` the "Weekly Reports" section; enumerate child pages; match on `report_week == thisMonday` (not the display title). Re-scan **immediately before** the create (TOCTOU narrowing — single-runner convention bounds, but does not eliminate, the race).
2. **Absent →** create (write `report_week` + `generated_by`). **Exactly 1 + marker asserts →** update in place (pulls in toggles added since the last run). **>1, or marker mismatch →** halt and surface.

Re-running the same week is safe and converges. A local `state/report-<weekISO>.json` records `{pageId, weekKey}` as a **diagnostic-only** backstop for mid-failure re-runs (gitignored). **The backstop never resolves a write target.** Every write path — including a mid-failure re-run, and the *convergent* and *empty-live-lookup* cases (the live `report_week` query agreeing with, or returning nothing despite, the backstop) — re-fetches the live page by `report_week` and asserts `generated_by == impact-portfolio` immediately before writing. If the live page can't be confirmed (empty/flaky lookup), **halt** — never write a `pageId` carried over from the backstop, a prior URL echo, or runner input. Identity is the live property + marker, never a cached id.

**Residual (stated, not hidden):** matching on the `report_week` property means a human-created page with a *colliding title but no `report_week`* is invisible to the match — the create path sees "absent → create" and a title-twin coexists rather than being detected. This is the accepted cost of property-over-title matching (it prevents the clobber); surface it the way the untagged-rate residual is surfaced, and a human reconciles the twin.

## Confirm the destructive action — not just the prose

`impact-weekly`'s confirm gate reviews the draft body. This skill's gate must additionally surface the **destructive action**, because an update *replaces* an existing page:

The summary is **branch-specific** (the required fields differ — so a create can't use "id/last-editor don't exist" as cover to skip it):

- **Replace** (exactly 1, marker asserts): target page id + URL, its **last-editor** + last-edited time, and "replace".
- **Create** (absent): the **parent** + the **TOCTOU re-scan result** + "no existing target (create)". A wrong-parent or TOCTOU-duplicate create is destructive too — "it's only a create" is not a reason to fast-confirm.
- **Halt**: >1 match / absent-or-mismatched marker / `last_edited_by` unreadable on a replace.

Three rules make the gate real:

1. **Fresh per run.** Confirmation is given against *this run's* rendered summary; it is not transferable. A standing or blanket "always confirm" does **not** satisfy the gate — it cannot have seen this run's target or last-editor, so it is not informed consent to this clobber.
2. **Full-body-clobber semantics.** "Render the page" (Procedure step 7) re-renders the *whole* page, so a write replaces everything not in the new render. Treat it as a full-body clobber for the confirm **regardless** of the underlying MCP call (full replace *or* `update_content` section-replace) — reason about the whole-page blast radius, not the call. (Until spike item 5 resolves the mechanism, this is the safe default.)
3. If `last_edited_by` can't be resolved on a replace, **halt** — don't replace without the human-vs-machine signal.

A gate that confirms wording but hides "this replaces page X last edited by a human on Tuesday" is confirming the wrong thing.

## Partial-fetch manifest

Track every source (bet toggle fetch, Linear scan) that failed. Render a footer: `Read N of M sources; could not load: <bet>, <bet>.` The `could not load:` list must name **every** failed source — a count (`N of M`) without the full enumeration is itself a soft partial, because the reader can't tell *which* projects are missing. A source whose display name couldn't be fetched is listed by its candidate id / page id, **never omitted**. A partial run is **never** presented as complete — half a portfolio silently omitted is indistinguishable from "those projects were quiet." Removing, softening, or truncating the manifest (including dropping names from the `could not load:` list) is a **halt** — the runner cannot opt out of it.

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
