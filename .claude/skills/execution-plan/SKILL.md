---
name: execution-plan
category: project-management
model: claude-opus-4-8
description: "Use when an agentic-PM step needs to decorate or read the bet↔design↔issue↔PR graph — stamp a Linear issue to its Impact bet, ensure a bet's impact:<slug> label, read the execution-plan graph (betGraph), or reconcile/backfill lineage. The shared mechanism /issue, /design, /impact-weekly, /impact-portfolio and the technical-program-manager agent call so identity/label/cache logic lives in one place. Triggers: 'stamp this issue to the bet', 'ensure the impact label', 'read the bet graph', 'reconcile the execution plan', 'backfill the impact lineage', '/execution-plan'. Anti-triggers: NOT for drafting the weekly exec entry (use /impact-weekly); NOT for the cross-project report (use /impact-portfolio); NOT for filing issues (use /issue) or capturing designs (use /design); NOT for deciding scope (product-manager) or judging on-course/at-risk (the technical-program-manager agent). It decorates and reads the graph; it does not author summaries or make judgments."
---

# Execution Plan

The **mechanism substrate** of the agentic-PM loop. It decorates and reads the lineage graph `bet ↔ design ↔ issues ↔ PRs` so that `/issue`, `/design`, `/impact-weekly`, `/impact-portfolio`, and the `technical-program-manager` agent all share one implementation of identity, labels, the cache, and the read contract — instead of each re-implementing it (and re-introducing a second identity).

Genre rule, load-bearing: **the Impact doc is the index; Linear + the PRs are the record.** This skill never becomes a third source of truth — it tightens links on the records that already exist and *computes* views; it stores nothing but a resolve cache.

The execution plan is a **derived set, not an object**: `plan(bet) = { Linear issues carrying impact:<slug> AND linking the bet's design URL }`. There is no plan row, project, or ID.

## Guardrails

This skill writes only two kinds of thing — a Linear **label** and a Linear **link/relation** on the team's own issues — plus, behind a confirm gate, nothing else. Before any action:

1. **Identity is the bet's Notion page ID — never a container.** `impact:<slug>` is a *re-derivable alias* of the page ID via the cache `{pageId → {slug, labelId}}`. **Refuse** any operation that would key identity or grouping on a Linear container (project / initiative / issue ID), or that introduces a second `impact`-style label or a label hierarchy. One flat `impact:<slug>` label. Keying identity on a container is the one irreversible corruption this skill exists to prevent.
2. **AUTO vs CONFIRM — the human-gate boundary.** *Silent/automatic* (no confirm): reading anything; applying the `impact:<slug>` label + design-URL link to the team's **own** Linear issues (idempotent decoration). *Confirm (draft→confirm→write):* **(a)** the **first creation** of an `impact:<slug>` label in the shared Linear workspace (the one-way-door act — others' saved filters will key off it), and **(b)** **any** write to Notion. There are no other write classes.
3. **Slug drift is always human-resolved.** If a label's slug ≠ kebab(the bet's current Name), **halt that bet and surface it** — present relabel-vs-keep-alias to the user. **Never auto-rejoin**; a wrong rejoin silently corrupts identity across every downstream rollup.
4. **No lifecycle writes, no judgments, no exec prose.** **Never** create / close / reassign / re-prioritize Linear issues; never flip Overall Confidence or edit bet definition fields; never author a weekly/exec summary. This skill *decorates and reads*. Deciding what work should exist is `product-manager`; judging on-course/at-risk and writing the narrative is the `technical-program-manager` agent; the exec entry is `/impact-weekly` / `/impact-portfolio`.
5. **Degrade, don't fabricate.** PR→issue linkage is config-contingent (Linear's GitHub integration); when it's absent, the graph degrades to **issue-only — say so, don't silently drop PRs or invent links**. If Linear or Notion MCP is unavailable, **halt and report** — never fabricate a bet, an issue, a label, or an identifier.

See `references/graph-and-decoration.md` for the data model, the `betGraph` read contract, slug derivation, idempotency, and the reconcile mechanics.

## Preconditions

- **Linear MCP** connected (`list_issues`, `get_issue`, `list_issue_labels`, `create_issue_label`, `save_issue` for label/link application) — interactively authenticated; may be absent headless (then halt).
- **Notion MCP** connected (`notion-fetch`, search) for resolving bets in the Impact Tracker by **page ID**. This skill only *reads* Notion; it never writes it.
- A bet reference (Notion page ID, or a slug/name resolvable to one) for `ensurePlan`/`stamp`; a `scope` for `betGraph`/`reconcile`.

## The operations (procedure)

Each operation states its write class (AUTO or CONFIRM per Guardrail #2).

1. **`ensurePlan(betPageId)` — [CONFIRM on first label create].** Resolve the bet (Notion, by page ID) → derive `slug = kebab(Name)` → look up / create the `impact:<slug>` Linear label. **Creating** the label the first time is confirm-gated; reusing an existing one is silent. Cache `{pageId → {slug, labelId}}`. Returns `labelId`. On slug drift vs an existing label, halt (Guardrail #3).
2. **`stamp(issueId, betPageId)` — [AUTO].** Apply the `impact:<slug>` label + add the bet's design-URL link to the issue. **Idempotent**: label present / link present → no-op. The team's own issues only; never a lifecycle change.
3. **`betGraph(scope)` — [AUTO, read-only].** The shared **read contract**. `scope = {persons[], betPageIds[]|all, window}`. Resolve bets (Notion) → for each `impact:<slug>` label, `list_issues(label, updatedAt≥window)` → pull **native** fields (status, assignee, completedAt, parent, linked PRs). Return `BetGraph{ bet{pageId,name,slug,confidence,owner}, plan{issues:[{id,title,status,assignee,completedAt,prs[],designLinked}]}, rollup{byStatus,byAssignee,completedInWindow,untaggedNearby} }` — **all computed, never stored**. The plan is the derived set (label ∧ design-linked). `untaggedNearby` carries the coverage signal.
3. **`reconcile(scope)` — [LIVE: AUTO decoration · BACKFILL: CONFIRM-batched].** One idempotent op, two triggers. **LIVE** = the inline tail of `/design` and `/issue` (decorate *this* artifact; mostly a no-op safety net that makes re-runs idempotent). **BACKFILL** = manual, all-bets-this-quarter (bulk-stamp historical issues, build the cache from history — the bulk decoration is shown and confirmed as a batch). Both: ensure cache + label, detect slug drift (→ human, Guardrail #3), decorate gaps (issues linking the design URL but missing the label, or labeled but missing the link), report coverage (untagged-rate).

## Halt Conditions

Stop and report rather than proceeding when:

- **Linear or Notion MCP is unavailable** — never fabricate a bet, issue, label, or identifier.
- **Slug drift** detected for a bet — halt that bet's `ensurePlan`/`reconcile` and surface relabel-vs-keep for human resolution; never auto-rejoin.
- A requested operation would **key identity on a container** or add a second identity label — refuse and explain.
- A requested operation is a **lifecycle write** (create/close/reassign an issue), a **confidence flip**, a **definition-field edit**, or an **exec-summary write** — refuse and redirect to the owning skill/agent/human.
- The Linear↔GitHub integration is absent — proceed **issue-only** and state the degradation; don't invent PR links.

## State

`state/run-<ISO-timestamp>/cache.yaml` holds only `{pageId → {slug, labelId}}` (the resolve cache) + `audit.log`. `state/` is gitignored. `betGraph` is **stateless / on-demand** — no materialized graph, so it can't drift from Linear/Notion. (A `betPageId` Linear custom field and a materialized cache are deferred — see references.)

## Rationalization Table

When your reasoning matches the left column, **stop**.

| Excuse | Reality |
|---|---|
| "Grouping by a Linear project is cleaner than resolving labels." | A container holding identity is the one irreversible corruption here. Identity is the bet page ID; the label is its alias. Refuse the container key. |
| "The slug obviously still matches the renamed bet — just rejoin it." | A wrong rejoin silently mis-attributes everything downstream. Drift is always human-resolved; never auto-rejoin. |
| "The issue is clearly stalled — close it / reassign it while I'm here." | No lifecycle writes. This skill decorates and reads. Surface the stall; a human (or the TPM agent's flag) acts. |
| "I'm already here, just append the weekly note to the bet too." | Any Notion write is confirm-gated and is `/impact-weekly`'s job. This skill never writes exec surfaces. |
| "PR linkage isn't wired, but I'll infer the PR from the branch name." | Degrade to issue-only and say so. Inferring a link is fabricating the record. |
| "Just stamp every recent issue to this bet to raise coverage." | Stamp only issues that genuinely belong (label ∧ design-linked). Mass-stamping to game coverage is mis-attribution. |
| "Cache the whole graph so reads are fast." | A materialized graph is a drift-prone second source of truth (deferred behind a real latency trigger). `betGraph` stays on-demand. |

## Output

End-of-session summary: which operation ran, what was decorated (label/link applied, idempotent no-ops noted), the `betGraph` result or coverage/untagged report, any slug-drift or lifecycle/exec requests **refused** with the redirect, and any degradation (issue-only) stated. If MCP was unavailable or a confirm was declined, say so plainly — a clean stop beats a fabricated decoration.
