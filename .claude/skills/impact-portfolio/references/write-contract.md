# Notion Write Contract

How `impact-portfolio` writes — **one row in the Weekly Reports database**, and only that. It is strictly **read-only on every bet** and on the Impact Tracker. Read alongside `selection-and-coverage.md` (what goes in the report) and the SKILL.md procedure.

## Where it writes

The **Weekly Reports** database — a data source under the Impact Hub page: `collection://af6a7313-890d-4a8c-a936-59b7e94ef8f6` (database page `a12973581bb640aeaf3dcfa75977b6a5`). Each weekly report is **one row** (itself a page); the report body lives in that page. Schema:

| Property | Type | Role |
|---|---|---|
| `Name` | title | display — `Impact Report - Week of <Month Xth>` |
| `report_week` | date | **identity / idempotency key** — the ISO Monday of the reported week |
| `generated_by` | select (`impact-portfolio` / `human`) | **provenance / clobber-guard** — skill-written rows carry `impact-portfolio` |

Nothing on any bet page or Impact Tracker row is ever touched. (The spike that chose a database over plain sub-pages — and why — is recorded on sei-internal-skills #119 / PLT-437: Notion allows custom properties only on database rows, not on plain sub-pages, so the database is what makes the identity + provenance below *native* rather than a fragile workaround.)

## Identity — the `report_week` date property (never the title)

- **Display title:** `Impact Report - Week of <Month Xth>` — human-facing only, never the match key (ordinal / locale / format drift).
- **Join key:** the row's **`report_week` date property** = the week's **ISO Monday** (`YYYY-MM-DD`), the same Monday-anchored week `impact-weekly` keys its toggles on.
- **Timezone:** the ISO Monday is computed against a **single declared TZ** (runner-confirmed or a documented default — never the runner's local clock; **halt** if it can't be established). A wrong TZ silently forks the identity.
- **Date-only comparison:** `report_week` is set and matched as a **date with no time component**, normalized to the declared TZ. The match is `report_week == thisMonday` as a *calendar date* — never a datetime/instant compare. Rows carrying a time component or a date *range* (a human picking a time in the UI, or another tool) are **out of scope** and may mis-match; that mis-match surfaces as a residual (below), never a silent wrong-row write.

## Provenance — `generated_by`, asserted before any overwrite

On **create**, set `generated_by = impact-portfolio`. Before **any** update to an existing row, re-read it and assert **both** `generated_by == impact-portfolio` **and** `report_week == thisMonday`. If `generated_by` is `human` (or absent), or the week mismatches, **halt** — never overwrite a human-created row. This native property *is* the ownership signal: it replaces the old `last_edited_by` inference (which the MCP does not reliably expose) with an explicit, readable field the skill itself sets.

## Mechanism

`notion-create-pages` parenting under a `data_source_id`, and database rows carrying custom properties, are the **proven** Notion paths (the Impact Tracker itself demonstrates them) — this is why the persistence is a database, not a sub-page.

- **Resolve:** query the Weekly Reports data source for rows where `report_week == thisMonday` (`query_data_sources`, or `notion-fetch` the data source and filter the date column). Re-query **immediately before** create (TOCTOU narrowing; the single-runner convention bounds, but does not eliminate, the race).
- **Create:** `notion-create-pages` with `parent = { data_source_id: af6a7313-890d-4a8c-a936-59b7e94ef8f6 }`, `properties = { Name, "date:report_week:start": <ISO Monday>, generated_by: "impact-portfolio" }`, `content =` the rendered report body.
- **Update (in place):** for the matched skill-owned row, rewrite its page body. Because the body is fully re-rendered each run, treat the update as **full-body-clobber semantics** regardless of the underlying call (`replace_content` on the row, or `update_content` section-replace). The row is skill-owned (its `generated_by` proves it), so a body replace is safe here — unlike a bet page, which is never written.

## Idempotency

`report_week` is the key. Before writing, query the rows for `thisMonday`:

1. **Absent →** create the row (`Name` + `report_week` + `generated_by: impact-portfolio`).
2. **Exactly 1, `generated_by == impact-portfolio` →** update its body in place (pulls in toggles added since the last run).
3. **>1, or the single match is `generated_by == human` / has no marker / the week mismatches →** **halt and surface** — never guess, never clobber a human row.

A local `state/report-<weekISO>.json` records `{rowPageId, weekKey}` as a **diagnostic-only** backstop (gitignored). **The backstop never resolves a write target.** Every write path — including a mid-failure re-run, and the *convergent* and *empty-live-query* cases — re-queries by `report_week` and asserts `generated_by` live immediately before writing. If the live query is empty/flaky, **halt** — never write a `rowPageId` carried over from the backstop, a prior URL echo, or runner input. Identity is the live property + provenance, never a cached id.

The old title-twin concern is now largely moot: rows are matched by `report_week`, not title. A human row for the same week is **detected** by the query and **halted on** via `generated_by == human`. The remaining residuals are enumerated below.

## Residuals (stated, not hidden)

The clobber-guard is best-effort, not cryptographic. Four accepted residuals — each surfaced or mitigated, none a silent failure:

1. **Unkeyed human row.** A human row for the week with `report_week` unset is invisible to the match → the skill creates its own row and a duplicate coexists. **Surfacing:** the create-branch confirm lists the database's existing same-week-candidate rows (by `Name`/title, and any `report_week` near the week) so the runner catches the twin and reconciles **before** creating; never clobber.
2. **Hand-edited skill row.** The report row is **machine-managed**: a re-run **regenerates the whole body** (to pull in newly-added toggles), discarding any manual edits a human made to the row in between. The skill cannot detect this — Notion does not reliably expose `last_edited_by` — so the mitigation is twofold: (a) the confirm's replace branch states the body is fully regenerated, and (b) humans edit the **source weeklies**, not the report row. Don't hand-edit the report; it is rebuilt every run.
3. **`generated_by` is advisory.** It is a normal select any board member can set; a human who sets their row's `generated_by = impact-portfolio` would pass the assert. The guard is trust-on-read, not proof of authorship. Accepted on a shared internal board; revisit if this report ever becomes a trust boundary.
4. **Date representation.** Only date-only, declared-TZ `report_week` values are in scope (see Identity). A same-week row carrying a time component or range that the date-only query misses produces the **same coexisting-duplicate state as Residual #1** — and is **surfaced the same way** (the create-branch confirm lists same-week-candidate rows for the runner to catch), never a silent duplicate. The create path itself can't detect it; the `>1`-match halt catches it only on a later convergent run, so the confirm listing is the primary surfacing.

All four are MVP-accepted on a shared internal board; each is surfaced or mitigated, not silent.

## Confirm the destructive action — not just the prose

Render the report, name the **target database**, and show a **branch-specific destructive-action summary**, then require **fresh per-run** confirmation (a standing "always confirm" never satisfies the gate — it can't have seen this run's target):

- **Create** (no row for the week): the target database + the **TOCTOU re-query result** + "no existing row (create)" + a **list of existing same-week-candidate rows** (by `Name`/title or a near-week `report_week`) so the runner catches an unkeyed/odd-keyed twin (Residuals #1/#4) before creating. A wrong-target or TOCTOU-duplicate create is destructive too — "it's only a create" is not a reason to fast-confirm.
- **Replace** (exactly 1, `generated_by == impact-portfolio`): the target **row id + URL + its `generated_by` value** + "replace (**regenerates the full body** — any manual edits to the row are discarded)".
- **Halt**: >1 row / `generated_by == human` or absent / `report_week` mismatch.

Treat every write as full-body-clobber semantics (the whole body is re-rendered). `generated_by` — not `last_edited_by` — is the human-vs-machine signal, and it is always readable because the skill sets it.

## Partial-fetch manifest

Track every source (bet Weekly-log fetch, Linear scan) that failed. Render a footer: `Read N of M sources; could not load: <bet>, <bet>.` The `could not load:` list must name **every** failed source — a count (`N of M`) without the full enumeration is itself a soft partial, because the reader can't tell *which* projects are missing. A source whose display name couldn't be fetched is listed by its candidate id / page id, **never omitted**. A bet whose Linear half failed is **unknown, not quiet** → it goes in the manifest, never silently classified inactive. A partial run is **never** presented as complete; removing, softening, or truncating the manifest is a **halt** — the runner cannot opt out of it.

## Draft → confirm → write

1. Render the full report + the target database + the destructive-action summary.
2. Require explicit, **fresh per-run** confirmation (the clobber, if any, is visible before it happens).
3. Create or update per the resolved decision; echo the row URL.
4. **Verify the render after write.** Re-fetch the row and read the body: if a code span renders bold, backticks are literal, a bare domain auto-linked, or a stray `*` appears, an authoring rule was violated — fix the source and re-write. A write isn't done until its render is clean.

Author every line of the report body to the Notion-flavored-Markdown rules in `../../impact-weekly/references/notion-flavored-markdown.md` (at most one inline `code` span per emphasis run; tokens bearing `_`/`__`/`*`/`**` and bare domains/paths/filenames in code spans; no literal `*` inside an emphasis run). The **render-correctness** half (Rules 1–4 + the after-write verify) is the binding part here — it prevents the silent on-render corruption of a shared exec surface. Its **re-matchability** half (Rule 5: a tangled line can't be surgically re-edited) is `impact-weekly`-specific to the `update_content` surgical path and does **not** gate this skill's write: a portfolio row is full-body-clobbered, so a corrupt line is overwritten wholesale on the next run rather than stranded. Verify the render regardless.

Never auto-write. The confirm gate is the safety that keeps a duplicate or a clobber off the shared exec board.

## Degradation

If `notion-create-pages` / the update call fails, the MCP is absent, or sources fetched only partially and the runner rejects a flagged partial: report what was attempted and stop. Never claim a write that didn't happen; never fabricate a row URL, a section, an owner, a confidence, or a link.

## One-way doors (persisted to the shared board — explicit human approval)

- The **Weekly Reports database** location + schema (`report_week`, `generated_by`) — other consumers come to depend on it.
- The **title convention** and the **`report_week`-as-identity** scheme (changing either forks all history).
