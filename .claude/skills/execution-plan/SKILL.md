---
name: execution-plan
category: project-management
model: claude-opus-4-8
description: "Use when an agentic-PM step needs to decorate or read the bet↔design↔issue↔PR graph — map a bet to its Linear project, read the execution-plan graph (betGraph), stamp a design-URL link onto an issue, or reconcile/backfill lineage. The shared mechanism /issue, /design, /impact-weekly, /impact-portfolio and the technical-program-manager agent call so identity/mapping/cache/read logic lives in one place. Triggers: 'map this bet to its project', 'read the bet graph', 'stamp the design link', 'reconcile the execution plan', 'backfill the bet lineage', '/execution-plan'. Anti-triggers: NOT for drafting the weekly exec entry (use /impact-weekly); NOT for the cross-project report (use /impact-portfolio); NOT for filing issues (use /issue) or capturing designs (use /design); NOT for deciding scope (product-manager) or judging on-course/at-risk (the technical-program-manager agent). It decorates and reads the graph; it does not author summaries or make judgments."
---

# Execution Plan

The **mechanism substrate** of the agentic-PM loop. It maps bets to their Linear projects and reads the lineage graph `bet ↔ design ↔ issues ↔ PRs` so that `/issue`, `/design`, `/impact-weekly`, `/impact-portfolio`, and the `technical-program-manager` agent all share one implementation of identity, the bet↔project mapping, the cache, and the read contract — instead of each re-implementing it (and re-introducing a second identity). *(Wiring the existing callers onto this substrate — migrating each consumer's read onto `betGraph`'s native-project rollup — is the follow-up burn-down; until each is migrated it keeps its current read path. The contract here is the single home they migrate to.)*

Genre rule, load-bearing: **the Impact doc is the index; Linear + the PRs are the record.** This skill never becomes a third source of truth — it maps a bet to the project the rest of the org already runs work in, *computes* views, and stores nothing but a resolve cache.

The execution plan is a **derived set, not an object**: `plan(bet) = { issues in the bet's Linear project }`. There is no plan row or separate plan ID — the bet's project *is* the container, and querying its issues is the plan.

## Guardrails

This skill's writes are scoped: a Linear **link/relation** (the design URL) on the team's own issues — AUTO; and, behind a confirm gate, the **first mapping/creation** of a bet's project. It never silently moves an issue between projects and never writes Notion. Before any action:

1. **Identity is the bet's Notion page ID — the project is its alias, never the identity.** The bet↔project mapping lives in the cache `{pageId → projectId}`; an issue is attributed to a bet by **membership in the bet's project** (native), not by a label. **Refuse** any operation that would (a) treat the project ID *as* the identity (resolve/group by container without the page-ID cache), (b) introduce a label that carries bet identity or bet attribution, or (c) silently move an issue between projects to re-attribute it. The project is a re-derivable alias of the page ID — exactly as the `impact:` label was — so renaming the project never corrupts identity; the page ID is the join key behind it.
2. **AUTO vs CONFIRM — the human-gate boundary.** Writes are to **Linear** on the team's **own** issues/projects; **read-only on Notion**. *Silent/automatic*: reading anything; adding the bet's design-URL link to an issue (idempotent decoration); reusing a cached bet↔project mapping. *Confirm:* the **first mapping of a bet to a project** (or creating a project for a bet) — the one-way-door act, since others' views and rollups key off it; and **setting an issue's project** (it changes the rollups others see — never silent, see Guardrail #4). The skill **never writes Notion** — exec-facing Notion writes belong to `/impact-weekly` / `/impact-portfolio`; refuse a Notion-write request here and redirect.
3. **Project drift is always human-resolved.** If a cached `projectId` no longer resolves (the project was deleted/archived), or the mapped project's Name diverged from the bet's Name beyond recognition, **halt that bet and surface remap-vs-keep**. **Never auto-rejoin** to a same-named project; a wrong rejoin silently corrupts every downstream rollup. (A Linear project keeps its ID across renames, so drift is rarer than slug-drift was — but deletion/archival and bet-rename still need the human gate.)
4. **Single-bet-per-issue — never silently move an issue between projects.** An issue belongs to at most one project, so it advances exactly one bet. `stamp` may set the project of an issue that has **none** (confirm — it affects rollups); an issue already in a **different** project is a **single-bet conflict** → **surface, never silently move** (the work belongs to another bet, or it spans bets and must be split into separate issues or joined by a native relation). Multi-bet attribution is **not** re-introduced via a label.
5. **No lifecycle writes, no judgments, no exec prose.** **Never** create / close / reassign / re-prioritize Linear issues; never flip Overall Confidence or edit bet definition fields; never author a weekly/exec summary. This skill *maps, decorates, and reads*. Deciding what work should exist is `product-manager`; judging on-course/at-risk and writing the narrative is the `technical-program-manager` agent; the exec entry is `/impact-weekly` / `/impact-portfolio`.
6. **Degrade, don't fabricate.** PR→issue linkage is config-contingent (Linear's GitHub integration); when it's absent, the graph degrades to **issue-only — say so, don't silently drop PRs or invent links**. If Linear or Notion MCP is unavailable, **halt and report** — never fabricate a bet, an issue, a project, a mapping, or an identifier.

See `references/graph-and-decoration.md` for the data model, the `betGraph` read contract, the bet↔project mapping, idempotency, and the reconcile mechanics.

## Preconditions

- **Linear MCP** connected (`list_issues`, `get_issue`, `list_projects`, `get_project`, `save_project`, `save_issue` for project/link application) — interactively authenticated; may be absent headless (then halt).
- **Notion MCP** connected (`notion-fetch`, search) for resolving bets in the Impact Tracker by **page ID**. This skill only *reads* Notion; it never writes it.
- A bet reference (Notion page ID, or a slug/name resolvable to one) for `ensurePlan`/`stamp`; a `scope` for `betGraph`/`reconcile`.

## The operations (procedure)

Each operation states its write class (AUTO or CONFIRM per Guardrail #2).

1. **`ensurePlan(betPageId)` — [CONFIRM on first map/create].** Resolve the bet (Notion, by page ID) → look up the mapped project via the cache, or **map to an existing project** / **create** the bet's project. Reusing a cached mapping is silent; **the first mapping of a bet to a project** (or creating a project for it) is confirm-gated (the one-way-door act — others' views/rollups key off it). Cache `{pageId → projectId}`. Returns `projectId`. On project drift vs the cache, halt (Guardrail #3).
2. **`stamp(issueId, betPageId)` — [AUTO design-link · CONFIRM/SURFACE project-set].** Attribution comes from **project membership**, so stamp splits: (a) **AUTO** — add the bet's design-URL link to the issue (idempotent design lineage); (b) **project membership is not silently written** — if the issue has **no** project, surface "assign to the bet's project?" (confirm); if it is already in a **different** project, that is a single-bet conflict → **surface, never silently move** (Guardrail #4). Idempotent when the issue is already in the bet's project. Never a lifecycle change.
3. **`betGraph(scope)` — [AUTO, read-only].** The shared **read contract**. `scope = {persons[], betPageIds[]|all, window}`. Resolve bets (Notion) → for each bet's project (cache), `list_issues(project=projectId, updatedAt≥window)` → pull **native** issue fields (status, assignee, completedAt, parent, linked PRs) **plus the native project rollup** (progress, status, target date, health). Return `BetGraph{ bet{pageId,name,projectId,confidence,owner}, plan{issues:[…]}, rollup{byStatus,byAssignee,completedInWindow, projectProgress, projectHealth, projectTargetDate, designLinkedNotInProject} }` — **all computed, never stored**. The plan is the bet's project's issues; the design-URL link is corroborating lineage. `designLinkedNotInProject` (issues linking the bet's design URL but not in its project) is the coverage signal.
4. **`reconcile(scope)` — [LIVE: AUTO · BACKFILL: CONFIRM-batched].** One idempotent op, two triggers. **LIVE** = the inline tail of `/design` and `/issue` (decorate *this* artifact; mostly a no-op safety net). **BACKFILL** = manual, all-bets-this-quarter (map bets→projects, build the cache from current project membership — confirmed as a batch). Both: ensure cache + project mapping, detect project drift (→ human, Guardrail #3), decorate gaps (issues linking the design URL but **not in** the bet's project → **surface** for assignment, never auto-move; issues in the project missing the design link → add link, AUTO), report coverage (design-linked-not-in-project rate).

## Halt Conditions

Stop and report rather than proceeding when:

- **Linear or Notion MCP is unavailable** — never fabricate a bet, issue, project, mapping, or identifier.
- **Project drift** detected for a bet — halt that bet's `ensurePlan`/`reconcile` and surface remap-vs-keep for human resolution; never auto-rejoin.
- A requested operation would **treat the project ID as the identity** (resolve/group by container without the page-ID cache), or **add a label carrying bet identity/attribution** — refuse and explain.
- A `stamp`/`reconcile` would **move an issue already in a different project** to re-attribute it — surface the single-bet conflict (split into per-project issues, or join via a native relation); never silently move.
- A requested operation is a **lifecycle write** (create/close/reassign an issue), a **confidence flip**, a **definition-field edit**, or an **exec-summary write** — refuse and redirect to the owning skill/agent/human.
- The Linear↔GitHub integration is absent — proceed **issue-only** and state the degradation; don't invent PR links.

## State

`state/run-<ISO-timestamp>/cache.yaml` holds only `{pageId → projectId}` (the resolve cache) + `audit.log`. `state/` is gitignored. `betGraph` is **stateless / on-demand** — no materialized graph, so it can't drift from Linear/Notion. (A `betPageId` Linear custom field and a materialized cache are deferred — see references.)

## Rationalization Table

When your reasoning matches the left column, **stop**.

| Excuse | Reality |
|---|---|
| "Just resolve/group bets by the project ID directly — skip the page-ID cache." | The project is the *alias*, the page ID is the *identity*. Resolving by container without the cache makes a renameable/deletable container the identity — the corruption to avoid. Always resolve through `{pageId → projectId}`. |
| "Multi-bet: just add an `impact:<slug>` label (or a second-bet label) so it counts toward both." | Labels never carry bet identity or attribution. An issue advances one bet — its project. Multi-bet work is split into per-project issues or joined by a native relation; never a label, never double-counted. |
| "The issue clearly belongs to this bet — move it into the project while I'm here." | Moving an issue between projects re-attributes it and changes others' rollups. If it has no project, confirm the assignment; if it's in another project, surface the conflict. Never silently move. |
| "The project was renamed; the names obviously match — just rejoin it." | A wrong rejoin silently mis-attributes everything downstream. Drift is always human-resolved; never auto-rejoin. |
| "The issue is clearly stalled — close it / reassign it while I'm here." | No lifecycle writes. This skill maps, decorates, and reads. Surface the stall; a human (or the TPM agent's flag) acts. |
| "I'm already here, just append the weekly note to the bet too." | Any Notion write is confirm-gated and is `/impact-weekly`'s job. This skill never writes exec surfaces. |
| "PR linkage isn't wired, but I'll infer the PR from the branch name." | Degrade to issue-only and say so. Inferring a link is fabricating the record. |
| "Cache the whole graph so reads are fast." | A materialized graph is a drift-prone second source of truth (deferred behind a real latency trigger). `betGraph` stays on-demand. |

## Output

End-of-session summary: which operation ran, what was mapped/decorated (bet↔project mapping, design-URL link applied, idempotent no-ops noted), the `betGraph` result or coverage report (design-linked-not-in-project), any project-drift or single-bet conflict or lifecycle/exec requests **surfaced/refused** with the redirect, and any degradation (issue-only) stated. If MCP was unavailable or a confirm was declined, say so plainly — a clean stop beats a fabricated mapping.
