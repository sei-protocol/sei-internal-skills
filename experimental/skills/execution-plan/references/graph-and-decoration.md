# Graph & decoration mechanics

The data model, the `betGraph` read contract, the bet↔project mapping, idempotency, and reconcile mechanics behind the four operations. Spec source: the `technical-program-manager` design + Design 10 (bdchatham-designs).

## Data model — one identity, project-as-container, derived plan

- **Identity = the bet's Notion page ID.** Immutable. Everything joins on it.
- **A Linear project = the bet's container and re-derivable alias** of the page ID, via the cache `{pageId → projectId}`. The project is the join key on the Linear side; the page ID is the identity behind it. A project keeps its ID across renames, so the mapping survives a rename.
- **Execution plan = a derived set, not an object:** `plan(bet) = { issues in the bet's project }`. No separate plan row/ID — the project *is* the container; querying its issues is the plan. The design-URL link on an issue is corroborating lineage (issue → design doc), not the attribution mechanism.
- **Attribution = native project membership.** An issue advances a bet by being in the bet's project. **No label carries bet identity or attribution.** Labels are reserved for cross-cutting tags / exceptions.
- **Decoration = one added thing on an issue:** the design-URL link. Everything else (project membership, status, assignee, completedAt, parent, PR links) is **native Linear state**, read as-is. Project membership is set only via the gated/surfaced path (never silently). There is no invented "progress %" — progress is the native project rollup (and the status distribution of the derived set).

### Reversible vs irreversible
- **Irreversible (guard hard):** treating the project ID *as* the identity (resolving/grouping by container without the page-ID cache); adopting a label that carries bet identity/attribution; silently moving an issue between projects to re-attribute it. Never do these.
- **Sticky (confirm once):** the first mapping of a bet to a project (or creating a project for a bet) — others' views/rollups key off it. Setting the project of an issue that has none (it changes rollups others see).
- **Reversible (build simple, change later):** the design-URL link, the cache, the derived plan, every read.

## The bet↔project mapping

A bet maps to one Linear project. The common case is **mapping to an existing project** the team already runs the work in; creating a project for a bet is the rarer case. Both first-time acts are confirm-gated (sticky). The mapping is cached `{pageId → projectId}`.

**Drift** = a cached `projectId` that no longer resolves (project archived/deleted) or a bet-rename that makes the mapping ambiguous. Drift is **always surfaced for human resolution** (remap to the right project, or keep the mapping) — never auto-rejoined to a same-named project, because a wrong rejoin mis-attributes every downstream rollup. (Because a project keeps its ID across renames, a plain rename does *not* drift the mapping — only deletion/archival or an ambiguous re-creation does.)

## `betGraph(scope)` — the shared read contract

Inputs: `scope = { persons[], betPageIds[] | all, window }`.

1. **Resolve bets (Notion):** for each in scope, read `{ pageId, name, confidence, owner }` from the Impact Tracker, then the mapped `projectId` from the cache. Read-only.
2. **Read membership + native state (Linear):** for each bet's project → `list_issues(project=projectId, window [start,end])` — **bounded on both ends** (the union of `updatedAt`- and `completedAt`-in-window, so a ticket finished in-window but last touched earlier isn't missed, and a long-running ticket updated after the window doesn't leak into an earlier slice) → for each issue pull **native** fields: `createdAt, status, assignee, completedAt, parent, prs[]` (linked PR attachments). `designLinked` = does the issue link the bet's design URL. Also read the **native project rollup**: `projectProgress, projectStatus, projectTargetDate, projectHealth`.
3. **Compute (never store):**
```
BetGraph {
  bet:    { pageId, name, projectId, confidence, owner }
  plan:   { issues: [ { id, createdAt, title, status, assignee, completedAt, prs[], designLinked } ] }   // derived set = project's issues
  rollup: { byStatus, byAssignee, completedInWindow,
            projectProgress, projectStatus, projectTargetDate, projectHealth,   // native, gained from the project model
            designLinkedNotInProject }                                          // coverage signal
}
```

The three consumers are projections over the same call:
- `/impact-weekly` → `betGraph({ persons:[me], window:7d })`, grouped by bet.
- `/impact-portfolio` → `betGraph({ all, window:7d })`, per-bet section.
- manager "what did my team do this week" → `betGraph({ persons:myTeam, window:7d })`, grouped by person + bet.

`designLinkedNotInProject` = in-scope issues that link the bet's design URL but are **in no project, or in a non-bet project** — the **coverage signal** (work tracing to the bet's design but not yet attributed to a bet), surfaced for human assignment, never auto-moved. **It excludes issues already in *another* bet's project**: those are that bet's settled, single-bet-attributed work — not this bet's coverage gap — and surfacing them here would invite the silent re-attribution Guardrail #4 forbids. This exclusion is what keeps single-bet-per-issue honest end-to-end: an issue counted in bet B's `plan.issues` is never *also* a coverage row of bet A, even when it carries A's design link (the AUTO design-link on a conflict is true lineage, but it does not create a cross-bet coverage row). Its inverse, **scope-creep candidates** = issues in the bet's project with `designLinked=false` (work in the project not tracing to its design) — the TPM agent derives this from `plan.issues[].designLinked`; there is no separate field.

**Degradation:** when Linear's GitHub integration isn't wired, `prs[]` is empty — the graph is **issue-only**; state it, don't infer PRs from branch names.

## Idempotency

- `stamp`: design-URL link already present → no-op; issue already in the bet's project → project no-op (the design link is the only AUTO write). Re-running `/issue`/`/design` never double-decorates.
- `ensurePlan`: mapping cached → reuse (no map, no confirm); only the first map/create confirms.
- `reconcile`: present link → no-op; mapping for an already-cached bet → no-op. Live and backfill are safe to interleave and re-run.

## reconcile() — one compute, two triggers

| Trigger | Scope | Cadence | What does the work |
|---|---|---|---|
| **LIVE** | the single issue/design just touched | inline tail of `/design`, `/issue` | mostly a no-op safety net (the step already decorated); makes re-runs idempotent |
| **BACKFILL** | all bets this quarter | manual, one-off bootstrap | map bets→projects, build the cache from current project membership; the batch is shown + confirmed |

Both: ensure cache + project mapping → detect project drift (→ human) → decorate gaps (design-linked-but-not-in-project → **surface** for assignment, never auto-move; in-project-but-unlinked → add design link, AUTO) → report coverage (design-linked-not-in-project rate). Mapping *creation* is gated; design-link decoration is auto in LIVE, confirm-batched in BACKFILL; drift and project-moves are always human.

## Multi-bet attribution — single-bet-per-issue (the resolved crux)

An issue belongs to at most one project, so it advances exactly one bet. Work that genuinely spans two bets is **split into separate issues** (one in each bet's project) or joined by a native Linear **issue relation** (`relates to`) — never a label, never double-counted. This is the operator-chosen tradeoff: native rollups + rollup integrity over the label model's many-to-many attribution. **Un-defer the many-to-many capability** if the split/relate workaround becomes a *measured* recurring burden (e.g. it recurs on more than a handful of issues per quarter, or rollup-integrity complaints arrive) — at which point revisit a native mechanism; until then the workaround is the minimal honest answer.

## Deferred (with un-defer triggers)

- **`betPageId` Linear custom field** — un-defer when project-drift reconciliation gets too noisy to resolve by hand at scale; then it becomes the canonical join and the project demotes to pure container.
- **Materialized graph cache / index** — un-defer when on-demand `betGraph` latency at portfolio scale is a measured bottleneck. Until then, stateless on-demand (can't drift).
- **reconcile daemon / webhooks** — un-defer when decoration-lag between work and graph is a real complaint. Inline + manual covers it.
- **Consumer migration onto the native-project rollup** — `/impact-weekly` / `/impact-portfolio` / `/issue` / the TPM agent migrate their reads onto `betGraph`'s project rollup incrementally; until migrated each keeps its current read path. (The substrate here is the single home they migrate to.)
- **Historical `impact:<slug>` label migration** — left in place; new attribution is project-based. A backfill that strips legacy labels is a separate, confirmed batch once the model has settled.
