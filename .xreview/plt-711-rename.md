# xreview ledger — PLT-711 rename /cross-review → /xreview (PR Tide#200)

Artifact: the skill/command rename diff (identifier-only; activity noun/verb preserved).
Small /workstream. Slate (blinded, one assigned dissenter): prose-steward, idiomatic-reviewer,
skill-authoring/audit, assigned dissenter. Checks: Cursor Bugbot + `verify` CI.

## Round 1 — findings (all four converged on the SAME two)

| Reviewer | Verdict | Findings |
|---|---|---|
| prose-steward | REVISE | (1) review-ledger.md:17 over-renamed an example slug → non-existent external artifact; (2) CLAUDE.md:13 stale `/cross-review`. |
| idiomatic-reviewer | REVISE→RATIFY-on-fix | Four-token identity holds, activity-nouns correctly preserved, ledger/gate paths consistent; one leak: CLAUDE.md:13 `/cross-review` + "cross-review discipline". |
| skill-authoring/audit | CONFORMS (0 blocking) | All 3 test suites pass (catalog 13/0, sync --verify, inject-doctrine 12/0, AGENTS.md byte-exact); 1 non-blocking gap: CLAUDE.md:13. C3 static-check fail = known false-positive (category-derived sync). |
| assigned dissenter | STILL-REFUTED | Exactly two: CLAUDE.md:13 dangling `/cross-review`; review-ledger.md:17 over-renamed Design-08 slug. All other vectors cleared (ledger-path consistency, activity-verbs, guards, no eval over-rename). |

## Resolution (commit 99a1267) — both findings fixed + verified

- **CLAUDE.md:13** — `/cross-review` → `/xreview` + "cross-review discipline" → "xreview discipline" (matches the renamed AGENTS.md/tide-doctrine block). Root cause: CLAUDE.md was excluded from the rename file set (orchestrator scope miss). Verified: `grep` for `/cross-review` / `` `cross-review` `` / `name: cross-review` across all live files (.claude, CLAUDE.md, AGENTS.md, tide-doctrine.md, README.md) = **0 hits**.
- **review-ledger.md:17** — example slug reverted `08-xreview-slate-and-ledger` → `08-cross-review-slate-and-ledger` (the real external bdchatham Design-08 filename, not changing under this PR); only the inserted `xreview/` ledger dir was correct. Matches the correct derivation pattern at evals.json:166.

Checks on 99a1267: `verify` CI = SUCCESS; Cursor Bugbot = **SUCCESS (clean, no new findings)**.

State: **RESOLVED.**
Convergence: **unanimous** — all four reviewers identified the identical two findings; both fixed + grep-verified + guards green + Bugbot clean. idiomatic/skill-auth explicitly flip to RATIFY/CONFORMS on these fixes; dissenter's two findings objectively closed (verified).
Dissenter: held (assigned; R1 STILL-REFUTED on the two findings → both resolved + verified).
OpenFindings: 0. Activity-noun preservation (cross-reviewed/-reviews/-reviewing) + history (docs/skill-audits, .xreview ledger prose, sei_omnigent) deliberately left per scope.
