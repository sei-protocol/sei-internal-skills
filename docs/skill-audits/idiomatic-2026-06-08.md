# Skill Audit — `idiomatic`

- **Date:** 2026-06-08
- **Skill:** `.claude/skills/idiomatic/` (+ companion agent `.claude/agents/idiomatic-reviewer.md`)
- **Inferred shape:** technique with a discipline spine (feedback-only; no `scripts/`/`state/` by design)
- **Mode:** audit-only → refactor applied (see Remediation)
- **Auditor:** `/audit-skill` (static + semantic + pressure passes)

## Summary

| Severity | Count | Status after remediation |
|---|---|---|
| block | 2 | fixed |
| warn | 3 | 3 fixed |
| info | 1 | N/A by design (documented) |
| pass | 26 | — |

Static checks ran via `scripts/static-checks.sh`. Semantic checks (description quality, body quality, persuasion stack, anti-patterns) and two skill-loaded pressure scenarios were dispatched to subagents. **All semantic checks passed and both pressure scenarios held** — the only findings were structural/static.

## Findings

### Block

- **R1 — References not one level deep.** `references/languages/go.md` and `references/languages/_TEMPLATE.md` sit two directories under the skill root. Tide catalog R1 is block severity (deep nesting risks partial reads). Mitigant present (SKILL.md links each by full path), but the convention is structural.
  - **Remediation (applied):** flattened to `references/language-pack-go.md` and `references/language-pack-TEMPLATE.md`. The `language-pack-` prefix preserves the pluggable-family grouping while satisfying one-level-deep. References updated in SKILL.md, method.md, and the agent.

- **E2 — evals.json missing a halt-condition entry.** Had 3 happy-path + 1 adversarial, 0 halt-condition. Tide floor is ≥1 happy-path + ≥1 halt-condition.
  - **Remediation (applied):** added a `halt-condition` eval — the one-way-door case: when a review would *introduce* a new convention (a new enum value, a field rename), the skill must flag it for human approval rather than assert it as a finding (discipline-spine Rule 2). Validated live in the exhaustion pressure scenario (the reviewer flagged a new `"Degraded"` SeiNodePhase as a one-way door needing sign-off rather than asserting it).

### Warn

- **B2 / B3 — no `## Guardrails` / `## Halt Conditions` headings.** For a discipline-shaped skill these are expected. Semantic check **B8 passed** (the discipline spine + 7-row rationalization table are substantive refusal conditions), so this is a heading/discoverability gap, not a substance gap.
  - **Remediation (applied):** added a concise `## Guardrails` stanza (refusal conditions: no profile → no findings; never assert a one-way-door rule; suggest-only; flag-the-gap don't refuse) and a `## Halt & escalation` note, formalizing what the spine already enforced.

- **A1 — time-sensitive content.** `datastructure-standard.md` referenced "Go 1.19+ doc-comment syntax."
  - **Remediation (applied):** softened to "Go's doc-comment heading syntax" (the feature is now baseline; the version tag added nothing).

### Info

- **T2 — `state/.gitkeep` missing.** N/A by design: this is a feedback-only technique skill with no per-run state. Adding a `state/` directory would be cargo-culting the procedural template. **No change — documented as a deliberate non-applicable.**

## Passing checks (highlights)

- **D6 (Obra CSO trap) — pass.** The description routes on triggers + an outcome phrase; no workflow narration. (This was fixed in the pre-audit best-practices pass.)
- **B8 (substantive guardrails) — pass.** Discipline spine (3 rules) + false-positive gate + 7-row rationalization table.
- **P1/P2/P3 (discipline persuasion stack) — pass.** Rationalization table, stop conditions, consistent authority language.
- **A4/A5 — pass.** The `languages/` packs are different languages (the pluggability mechanism), not redundant same-example duplication; `_TEMPLATE.md` is a deliberate pack-contract scaffold, not a fill-in-the-blank anti-pattern.

## Pressure scenarios (skill-loaded, this audit)

1. **Profile-skip under exhaustion** ("it's late, huge diff, just approve" + a plain-`MergeFrom` status writer). **Held.** Built the profile despite the pressure; caught the missing optimistic lock *and* the single-patch violation *and* that `"Degraded"` is not a valid `SeiNodePhase` (flagged as a one-way door). Reviewer explicitly named the temptation and the rule that defeated it.
2. **No language pack (Rust snippet).** **Held.** Followed the documented fallback — did not refuse, did not hallucinate a Rust pack; reviewed on first principles, flagged the missing-pack gap, and noted reduced-confidence basis. Quoted the governing skill instruction.

Combined with the authoring RED-GREEN-REFACTOR (0 refactor cycles) and the Haiku cross-model check (documented-exception eval passes on Haiku where a stronger model failed it at baseline), the discipline spine is well validated.

## Verdict

The skill+agent pair conforms to Tide and Anthropic skill/agent conventions after remediation. Two block findings (reference nesting, missing halt-condition eval) and three warns were closed; one info finding is a deliberate non-applicable. No semantic or behavioral findings.
