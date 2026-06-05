---
skill: cross-review
shape: procedural+discipline
audited_on: 2026-06-05
auditor: bdchatham
audit_skill_version: 1
phase: audit-only
---

# Skill Audit — `cross-review`

**Shape:** procedural+discipline
**Audited:** 2026-06-05 by bdchatham
**Phase:** audit-only (ran alongside authoring, via `/author-skill` + `/audit-skill` static + semantic checks)

## Summary

- **Block:**   0
- **Warn:**    1 (remediated)
- **Info:**    0

Static checks: all pass (after trimming the description from 1054 → 957 chars to satisfy D2). Semantic checks: one warn (D6), remediated in-pass. Two independent reviews (semantic conventions audit + `platform-engineer` cross-review) both returned "safe to commit."

## Warn findings

### D6 — Description summarized workflow (Obra CSO trap)

- **Source:** semantic
- **Evidence:** description tail read "...then synthesize a findings table" and "The review counterpart to /coral's produce; /coral and /council both call into it" — describing what the skill *does* rather than purely *when* to invoke it. The "both call into it" also flattened the real coral/council split.
- **Remediation (applied):** removed the "then synthesize a findings table" workflow tail; rephrased the relationship to "The review counterpart to producing work with /coral; /coral offers it at synthesis and /council invokes it as its review phase" — accurate (coral offers opt-in; council invokes as a mandatory phase) and WHEN-routing.

## Nits (addressed)

- **Guardrail #2 roster framing** — reworded "cross-review is multi-expert by design" → "cross-review selects its reviewer slate from a roster" so the permitted single-reviewer pass no longer reads as a contradiction.

## Nits (accepted, not changed)

- Two Halt Conditions (bare approval; reviewers not blinded) are corrective *re-dispatch* actions rather than stop-and-report halts. Meaning is clear from context and the evals encode it correctly; left as-is.
- No eval isolates the single-reviewer-labeling case or the reviewers-split / provider-tie-break-fails halt. Both are real disciplines in the skill; candidates for a future eval. Current bar (1 happy + 3 halt) exceeds the minimum.

## Provenance

Authored via `/author-skill`: RED baseline (3 pressure scenarios — rubber-stamp under time+authority, dispatch≠cross-review conflation, consensus theater) surfaced the rationalizations the skill encodes; grounded in a research pass (Fagan inspection independent-prep, Asch conformity, code-review rubber-stamping, multi-agent LLM sycophancy/consensus-theater, API provider/consumer compatibility). The surviving scenarios became the halt-condition evals.

## References

- Conventions catalog: `.claude/skills/audit-skill/references/conventions-catalog.md`
- Authoring methodology: `.claude/skills/author-skill/SKILL.md`
