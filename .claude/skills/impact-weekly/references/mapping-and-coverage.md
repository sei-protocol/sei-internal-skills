# Mapping & Coverage

How work is attributed to a bet (the anti-mis-tracking machinery, failure mode #1). Mis-attribution silently corrupts a shared exec board, so this is the most safety-critical step.

## Identity: the bet's Notion page ID

The join key is the bet's **immutable Notion page ID** — never the slug, never the Name (both mutable). The `impact:<slug>` label is a human-readable display alias (kebab of the Name) layered on top for filterability. The cache is:

```
state/project-map-<user>.json   (gitignored)
{ "impact:<slug>": { "pageId": "<notion-page-id>", "name": "<current Name>" } }
```

## Primary: label-first

1. The week's Linear issues are grouped by their `impact:<slug>` label.
2. Each label resolves to a bet page ID via the cache.
3. Grouped issues map deterministically to that bet. No guessing.

This is the reliable path. It only works for work that was decorated upstream (a future `/issue` enhancement stamps the label; until adoption rises, most work hits the fallback below — that's expected at cold start).

## Fallback: name-match, human-confirmed

For untagged issues:

1. Fetch the engineer's `Person`-scoped Impact Tracker rows (a small candidate set).
2. Propose a mapping by fuzzy-matching the issue's Linear project/title against the bet Names — **with the confidence shown**.
3. **Present all proposed mappings in one batched confirmation** (most-uncertain first), not a prompt per issue — the user confirms or corrects the batch in a single interaction. An unconfirmed name-match is never written. At cold start (most work untagged) this batch is the bulk of the interaction, so keep it to one screen; aggressive caching makes subsequent weeks near-zero-touch.
4. Cache confirmed mappings by page ID so the cost decays toward zero.

Linear *projects* do **not** correspond to Impact bets (they're coarser and durable — Calm Velocity, Incidents, …), so never treat a Linear project as a bet. The match is against bet Names, confirmed by a human.

## Slug drift (rename / split / merge)

Because identity is the page ID, a renamed bet doesn't orphan past work, and **a write to that same page ID still proceeds** — the page is the canonical target; the only open item is relabeling the alias. On detecting `label slug ≠ current Name`, **surface it for human resolution** (relabel, or keep the alias) but don't block the write to the correct page. (Halt only when the cached page ID resolves to something that is *not* the engineer's Impact Tracker bet row — see the write contract's pre-write verification.) Split/merge are surfaced the same way. Never silently re-join or drop.

## Coverage gate

Before any write, reconcile and surface:

- **Worked bets with no row** — work mapped to something that isn't an owned Impact bet → assign or skip (human call).
- **Owned bets with work but no entry** — don't silently omit; either draft an entry or note "no qualifying work."
- **Unmapped issues** — listed explicitly as "assign or skip."

**If a gap exists, surface it and get the user's call before writing — never write a *silent* subset, and never resolve a gap by attributing work to a convenient bet.** This is not "never write if any gap exists": once the user has assigned or explicitly skipped the gapped items (the coverage sign-off in procedure step 4), the **confirmed** bets are written, and the skipped/unmapped items are reported as still-open (step 8). The gate prevents *silent* omission and mis-attribution, not acknowledged partial progress. The gate's **untagged-rate** (work that fell to the name-match fallback) is the adoption signal for the decoration convention.

## PR substantiation caveat

PRs attach to issues via Linear's GitHub integration, which is **config-contingent** (magic words / branch naming). When a PR link is present, cite it as secondary evidence; when absent, substantiate from the issue alone and say PR linkage was unavailable — don't drop the work, and don't fabricate a PR link.
