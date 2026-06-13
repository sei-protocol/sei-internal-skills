---
skill: idiomatic
shape: technique (with a discipline spine)
audited_on: 2026-06-13
auditor: bdchatham
audit_skill_version: 1
phase: audit-only
---

# Skill Audit — `idiomatic`

**Shape:** technique with a discipline spine
**Audited:** 2026-06-13 by bdchatham
**Phase:** audit-only (run alongside the Go-kit `examples-go.md` addition)

## Summary

- **Block:**   0
- **Warn:**    1 (A1 — vetted false positive)
- **Info:**    1 (T2 — fixed in this change)
- **Pressure:** passed (no over-flagging induced by the new examples file)

No blockers. The skill is in good shape; the new `examples-go.md` and §7 lint-anchor additions introduced no convention drift.

## Static checks

All blockers pass: description starts with "Use when" / 827 chars / third-person / 7 trigger phrases / has anti-triggers; SKILL.md 134 lines (< 500); references one-level-deep; `evals.json` parseable with 3 happy-path + 1 halt-condition (13 total, all with `source`); `state/` gitignored; listed in catalog README.

### A1 — No time-sensitive content (warn) — **vetted false positive, no change**
- **Source:** static
- **Evidence:** `language-pack-go.md` matches the word "current".
- **Disposition:** the match is "the code states only **current** intent" / "what the code *currently* does" in the D10 comment-discipline rule (the PLT-471 tombstone language from Tide #147). "Current intent" is the rule's substantive vocabulary, not time-sensitivity drift about the document. Present on `origin/main` before this change. Rewording would damage the rule. Accepted as-is.

### T2 — `state/.gitkeep` exists (info) — **fixed**
- **Source:** static
- **Evidence:** `state/` directory had no `.gitkeep`.
- **Disposition:** added `.claude/skills/idiomatic/state/.gitkeep`.

## Semantic checks

All semantic checks pass (description quality incl. the Obra CSO workflow-summary trap; guardrails substance; persuasion stack matches the technique-with-spine shape; references extend rather than duplicate). Notable confirmations:

- **D6 (CSO trap):** description routes on *when* + capability ("backs the idiomatic-reviewer agent"), not a "runs X then Y" recipe — the four-step method stays in the body.
- **B8 (guardrails):** 5 concrete refusal conditions, above the 3 minimum.
- **New-additions drift check:** `examples-go.md` is flat (one level deep), single-language (no multi-language-for-one-technique anti-pattern), scoped to worked pairs that cite back to pack dimensions rather than duplicating them, and its on-demand load is stated in 3+ places with a clear trigger — an agent will not auto-load it every review. The §7 lint-anchor section reinforces the Rule 3 citation spine rather than bloating SKILL.md.

## Pressure test

Scenario: a largely-idiomatic Go file (concrete `*Client` return, ctx-first, `%w` wrapping, `defer Close`, early returns) reviewed under "this is an important path — I want to see findings, use the examples file" pressure, to test whether the new `examples-go.md` induces over-flagging.

- **Result: passed.** The reviewer concluded "reads native — no findings," correctly recognized that `func NewClient(...) *Client` is the D3 *good* case (not the examples' returns-interface bad case) and put it under "deliberately not flagging (vetted)," did not paste example snippets wholesale, and resisted the "show me findings" pressure with the rationalization-table false-positive discipline. The examples file did not induce manufactured findings.

## Cross-review fixes applied (this change)

From the Coral cross-review (`idiomatic-reviewer` + `prose-steward`) of `examples-go.md`:

- **`copylocks` (plural) bug** — caught by running `go vet` during example authoring: the lock-copy analyzer is `copylocks`, not `copylock` (the shipped v2 pack §7 had it backwards). Corrected in all sites.
- **`strings.ToTitle` footgun** — the "a little copying" good snippet misused `ToTitle` (uppercases everything ≠ title-case) and wasn't behavior-preserving; replaced with a behavior-neutral `strings.ToLower(strings.TrimSpace(s))` pair.
- **Observed-quote accuracy** — the `copylocks` "Anchor (observed)" quote prepended a `copylocks:` prefix that plain `go vet` doesn't print; corrected (golangci-lint's govet adds the prefix; plain `go vet` doesn't).
- **Prose (consuming-agent audience):** anchored the consult-don't-paste constraint in the file header (R3); surfaced the correctness lead on the observability example's dual finding (R2); fixed the `§3/§4` heading that broke the D-number scan pattern (R1); named the "style" tier in the severity legend (R4).

## Disposition

Audit-only. No refactor pass needed — the one warn is a vetted false positive and the one info was fixed inline. The skill conforms to the conventions catalog.
