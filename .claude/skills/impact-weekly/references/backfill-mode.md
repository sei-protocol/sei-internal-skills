# Historical-backfill mode

**Backfill mode reconstructs many past weeks of already-shipped work for a window that predated Linear
tracking — there was no contemporaneous Linear state to read, so status and lineage are PR-substantiated,
not Linear-tracked.** It is the same skill applied retroactively — same genre (*the doc is the index; the PRs
are the record*) and same guardrails as the live skill (`write-contract.md` / SKILL.md Guardrails, including
idempotency, the confirm gate, and verify-render-after-write; never auto-write a backfill). Only the
work-source differs: a historical PR corpus instead of this week's Linear query. Invoke it explicitly
("reconstruct the last N months on bet X").

## The 4-part toggle (the backfill entry shape)

A live weekly entry is `headline + bullets` and reads status from native Linear state. A **backfill** entry
carries an explicit **Status** and **Lineage** line so the exec reader knows it is PR-substantiated:

```markdown
> **Week of <YYYY-MM-DD>** — <outcome-verb headline: what shipped, not a volume/maintenance phrase>
    *Status:* <Linear histogram `X done / Y in-progress / Z blocked` (fully-contemporaneous weeks only) | `active — PR-only (no Linear state)` | `flat`>
    *Lineage:* <PR-only attribution + identity note>
    - <one bullet per delivered outcome> — [repo#N](pr-url) [repo#M](pr-url)
```

Backfill entries are dense with repo-qualified refs — exactly the content that trips the autolink / emphasis
hazards `notion-flavored-markdown.md` guards, so the post-write render check matters most here.

## The status-line rule (pinned — this is the recurring defect)

The single defect that recurred across every backfill cycle was the **Status line** drifting into a
PR/commit count, an identifier, or routing prose. Status carries **EXACTLY ONE** of:

1. a **real Linear histogram** `X done / Y in-progress / Z blocked` — **derived from `/execution-plan`'s
   `betGraph({betPageIds:[this bet], window:[that ISO week]}).rollup.byStatus`** over the bet's mapped
   project (compose, don't reimplement — **never** a hand-rolled `list_issues` count and never a label-scoped
   count), and **only for a fully-contemporaneous week**: emit (1) ONLY if *every* issue in that week's
   `betGraph` slice has `issue.createdAt` in or before the reported week (not created same-day-as-the-write
   as backfill). If **any** issue in the slice is after-the-fact backfill, the week is (2).
2. the literal **`active — PR-only (no Linear state)`** — a week with shipped PRs but no fully-contemporaneous
   Linear state (the common backfill case).
3. the literal **`flat`** — a genuine no-movement week.

**Never** put a count, an identifier (PLT-###, PR #), or routing / exclusion / confirmation prose in Status —
those belong in **Lineage**, as do routed-out PRs and net-zero revert pairs (never a Status). No
maintenance/volume headline ("12 PRs merged"). A week with even one backfilled-today issue is (2), not a
synthesized histogram.

## Methodology

- **Index, not record.** One entry per **engineer × bet × ISO-week**, sized at the *outcome*, not the PR count.
  High-level granularity is fine for a historical backfill (one bullet per theme/work-unit) — the operator's call.
- **Cite or cut — by case.** A claim with no evidence is refused, not softened.
  - **PR-only weeks (case 2):** every bullet carries repo-qualified PR links (`repo#N`, never a bare `#N` —
    `qa-testing#83` ≠ `Tide#83`; no ranges or slashes — `platform#5 platform#4`, never `#4-5`). Every PR in
    the window appears in exactly one bullet OR is noted in Lineage as routed-out / revert-evidence.
  - **Contemporaneous weeks (case 1):** substantiate on the **Linear issue ref**; the PR link is secondary and
    config-contingent (per `mapping-and-coverage.md` — PR↔issue linkage isn't guaranteed). "No link" means no
    Linear *and* no PR evidence.
- **Single-bet-per-issue (PLT-530).** A PR/issue advances exactly one bet — its project's. Work that spans
  bets is split or noted; never double-counted. Identity is the bet's Notion **page ID**; attribution is
  project membership.
- **Net-zero / revert pairs → Lineage only.** An enable-then-revert or add-then-drop that nets to zero is
  recorded in Lineage as evidence, **never** counted as a delivered outcome.
  - **Exception — a deliberate force-rollout** (e.g. a digest-pin-then-tag-revert that forces a rollout, then
    settles): counted as the single enablement it is — but only if it (a) cites the enabling PR ref(s),
    repo-qualified (cite-or-cut still applies), (b) names the pairing in Lineage, and (c) maps to exactly one
    bet's project. An unsubstantiated "force-rollout" claim is refused like any other — the carve-out is not a
    laundering path for an uncited net-zero pair.
- **Timeline keys off merge dates — with a contemporaneous override.** Place each PR-only outcome (case 2)
  under the ISO week of its **merge** date (Linear can't backdate `createdAt`). In a contemporaneous week
  (case 1) the **Linear completion week governs** placement and the PR is cited there even if it merged a few
  days into the next ISO week — a single work-unit never spans two weeks.
- **Titles-only corpus → cluster by theme, gut-check first.** When the PR corpus carries only titles (no file
  paths), cluster by title/theme into per-bet weekly buckets and **surface the clusters for a human gut-check
  before writing** — especially when one repo's PRs split across several bets.

## Lineage line

PR-only attribution plus the identity note — and the routed-out / net-zero evidence the status rule defers
here. Typical: *"PR-only — no Linear identifiers this early. Identity is the bet's Notion page ID per
PLT-530; the bet's mapped Linear project is the go-forward container (project membership = attribution); the
`impact:<slug>` label is historical, no longer attribution."* Per PLT-530 the common case maps a bet to an
**existing** project, so don't assert "(created `<date>`)" unless the project was in fact newly created for
this bet.

## Idempotency boundary (backfill ↔ live)

The `Week of <Monday>` toggle key, the >1-match halt, the confirm gate, and verify-render-after-write are
unchanged from `write-contract.md` — backfill does not relax them. The one backfill-specific hazard: a later
**live** `/impact-weekly` run covering the same ISO week targets the **same** toggle title. A live re-run must
**not** silently overwrite a backfilled PR-only toggle — mark backfilled toggles so a live run that finds one
**halts and surfaces** (merge its PR refs into Lineage) rather than blind-replacing PR-substantiated history
with a partial live read.

## Orchestration (drafts via workflow → main loop writes)

A months-long backfill is a fan-out: a Coral workflow drafts the per-bet weekly toggles + the high-level Linear
tickets from the corpus (product-engineer / TPM authors; cross-review verifies). **Sub-agents lack the
interactively-authenticated Notion/Linear MCP**, so they **draft only** — the **main loop does every Notion
`update_content` and Linear write** after the confirm gate, following `notion-flavored-markdown.md` +
verify-render-after-write. Reviewers in that workflow are suggest-only — never the terminal authoring stage
(see `coral` Dispatching Tips). Each bet is its own review-gate / confirm before its writes.
