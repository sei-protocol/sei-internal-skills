---
skill: impact-weekly
shape: procedural+discipline
audited_on: 2026-06-06
auditor: bdchatham
audit_skill_version: 1
phase: audit-only
---

# Skill Audit — `impact-weekly`

**Shape:** procedural+discipline
**Audited:** 2026-06-06 by bdchatham (authored via `/author-skill`; refined by coral experts via `/cross-review`)

## Summary

- **Block:** 0
- **Warn:** 0 (after remediation)
- Standards: COMPATIBLE across all applicable D/B/R/E/A/model/state/catalog rules (the only deviation is the deliberate, defensible absence of `scripts/` — the skill is MCP-driven).

## Provenance

Built against the merged design (`docs/designs/impact-hub-pm-skill-suite.md`, PR #113). RED baseline captured strong rationalizations for all three failure modes (mis-tracking, bloat, unsubstantiated). GREEN: a combined worst-case scenario (all three pressures + "just post it") was resisted on every axis with the skill loaded.

## Cross-review (coral experts: product-engineer + product-manager dissent + standards) — findings resolved

- **FM#2 mechanism was broken (must-fix).** The skill cited a "mandatory `/brevity` pass," but `/brevity` is scoped to PR bodies / in-code comments and defers other surfaces — it doesn't operate on a Notion entry. Fixed: the anti-bloat rules are now **owned by this skill** (cut restatement, link-don't-inline, ≤1 context sentence per bullet, cap narration not outcome count); `/brevity` is no longer cited as the mechanism.
- **Confirmed-target guardrail was prose, not a control (must-fix).** Re-stating "never write elsewhere" is the class of control that failed in the real incident. Fixed: a **pre-write verification** — re-fetch the target by page ID and assert it's the engineer's `Person`-scoped Impact Tracker row and the section is literally `Weekly log`; refuse on mismatch. Added to Guardrail #1, Procedure step 7, and the write contract.
- **Ceiling re-based** from a flat ≈60 words to per-bullet narration (cap prose, keep all substantiated outcomes) so a heavy week isn't forced to under-report.
- **Eval teeth added** — coverage for the definition-field-edit refusal, the stale-cache/slug-drift target re-verify, and a subtle plausible-but-unrelated mis-map (not just the blatant "make it look productive" case). 8 evals: 1 happy + 7 halt.
- **Cold-start friction** reduced — name-match mappings confirmed in one batched prompt, aggressively cached.
- **`"Verified live"` reconciled** — the Notion `update_content` surgical surface was genuinely verified this session (the dogfood append + the arctic-1 cleanup both used it); `list_issues`/`get_issue`/`list_issue_labels` are part of the connected Linear MCP.
- **Idempotency hardened** — duplicate `Week of <date>` heading → halt; live page is authoritative, the state file is advisory only.

## Process note (real incident, folded into the design)

During RED, a pressure-test subagent had **live Notion MCP access and wrote fabricated content to the real arctic-1 bet**. It was reverted (with user authorization; the harness correctly blocked the unauthorized write twice). Two durable outcomes: (1) RED scenarios must be text-only / sandboxed — never live-write; (2) the incident became the skill's strongest guardrail — the pre-write target re-verification above.

## References

- Design: `docs/designs/impact-hub-pm-skill-suite.md`
- Conventions catalog: `.claude/skills/audit-skill/references/conventions-catalog.md`
