# Graph & decoration mechanics

The data model, the `betGraph` read contract, slug derivation, idempotency, and reconcile mechanics behind the four operations. Spec source: the `technical-program-manager` design (bdchatham-designs).

## Data model — one identity, derived plan

- **Identity = the bet's Notion page ID.** Immutable. Everything joins on it.
- **`impact:<slug>` Linear label = a re-derivable display alias** of the page ID, via the cache `{pageId → {slug, labelId}}`. The label is the join key on the Linear side; the page ID is the identity behind it.
- **Execution plan = a derived set, not an object:** `plan(bet) = { issues carrying impact:<slug> AND linking the bet's design URL }`. No plan row/project/ID — querying *is* the plan.
- **Decoration = exactly two added things on an issue:** the `impact:<slug>` label + the design-URL link. Everything else (status, assignee, completedAt, parent, PR links) is **native Linear state**, read as-is. There is no invented "progress %" — progress is the status distribution of the derived set.

### Reversible vs irreversible
- **Irreversible (guard hard):** keying identity/grouping on a Linear container (project/initiative/issue ID); adopting a second identity label. Never do these.
- **Sticky (confirm once):** first creation of an `impact:<slug>` label in the shared workspace — others' saved filters key off the name.
- **Reversible (build simple, change later):** the label value (relabel), the design-URL link, the cache, the derived plan, every read.

## Slug derivation

`slug = kebab-case(bet Name)` — lowercase, spaces/punctuation → single hyphens, trimmed. Derived, never stored as identity. **Drift** = an existing `impact:<slug>` label whose slug ≠ kebab(current Name) (the bet was renamed). Drift is **always surfaced for human resolution** (relabel the issues to the new slug, or keep the old label as an alias) — never auto-rejoined, because a wrong rejoin mis-attributes every downstream rollup.

## `betGraph(scope)` — the shared read contract

Inputs: `scope = { persons[], betPageIds[] | all, window }`.

1. **Resolve bets (Notion):** for each in scope, read `{ pageId, name, slug=kebab(name), confidence, owner }` from the Impact Tracker. Read-only.
2. **Read decoration (Linear):** for each bet's `impact:<slug>` label → `list_issues(label, updatedAt ≥ window.start)` → for each issue pull **native** fields: `status, assignee, completedAt, parent, prs[]` (linked PR attachments). `designLinked` = does the issue link the bet's design URL.
3. **Compute (never store):**
```
BetGraph {
  bet:    { pageId, name, slug, confidence, owner }
  plan:   { issues: [ { id, title, status, assignee, completedAt, prs[], designLinked } ] }   // derived set
  rollup: { byStatus, byAssignee, completedInWindow, untaggedNearby }
}
```

The three consumers are projections over the same call:
- `/impact-weekly` → `betGraph({ persons:[me], window:7d })`, grouped by bet.
- `/impact-portfolio` → `betGraph({ all, window:7d })`, per-bet section.
- manager "what did my team do this week" → `betGraph({ persons:myTeam, window:7d })`, grouped by person + bet.

`untaggedNearby` = in-scope work by the persons that carries no `impact:<slug>` label — the **coverage signal** (missing-label work), surfaced, never silently dropped. Its inverse, **scope-creep candidates** = labeled issues with `designLinked=false` (work tied to the bet but not tracing to its design) — the TPM agent derives this directly from `plan.issues[].designLinked`; there is no separate field.

**Degradation:** when Linear's GitHub integration isn't wired, `prs[]` is empty — the graph is **issue-only**; state it, don't infer PRs from branch names.

## Idempotency

- `stamp`: label already present → no-op; design-URL link already present → no-op. Re-running `/issue`/`/design` never double-stamps.
- `ensurePlan`: label exists → reuse (no create, no confirm); only first create confirms.
- `reconcile`: present label/link → no-op; building cache for an already-cached bet → no-op. Live and backfill are safe to interleave and re-run.

## reconcile() — one compute, two triggers

| Trigger | Scope | Cadence | What does the work |
|---|---|---|---|
| **LIVE** | the single issue/design just touched | inline tail of `/design`, `/issue` | mostly a no-op safety net (the step already decorated); makes re-runs idempotent |
| **BACKFILL** | all bets this quarter | manual, one-off bootstrap | bulk-stamp historical issues, build the cache from history; the bulk decoration is shown + confirmed as a batch |

Both: ensure cache + label → detect slug drift (→ human) → decorate gaps (labeled-but-unlinked, or design-linked-but-unlabeled) → report coverage (untagged-rate). Label *creation* is gated; gap-decoration is auto in LIVE, confirm-batched in BACKFILL; drift is always human.

## Deferred (with un-defer triggers)

- **`betPageId` Linear custom field** — un-defer when slug-drift reconciliation gets too noisy to resolve by hand at scale; then it becomes the canonical join and the label demotes to pure display.
- **Materialized graph cache / index** — un-defer when on-demand `betGraph` latency at portfolio scale is a measured bottleneck. Until then, stateless on-demand (can't drift).
- **reconcile daemon / webhooks** — un-defer when decoration-lag between work and graph is a real complaint. Inline + manual covers it.
