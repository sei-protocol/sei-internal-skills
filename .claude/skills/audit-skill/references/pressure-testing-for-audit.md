# Pressure Testing — Audit Reframing

Audit-skill reuses the RED-GREEN-REFACTOR methodology from `../../author-skill/references/testing-with-subagents.md`. The differences in audit context are listed here.

## Read the source first

**REQUIRED BACKGROUND:** Read `.claude/skills/author-skill/references/testing-with-subagents.md` for the foundational methodology. This file only documents how audit-mode differs.

## Differences from author-mode

### No baseline-without-skill pass

In `/author-skill`, RED runs the scenarios *without* the skill loaded — that's the baseline that proves the skill addresses real failures. In `/audit-skill`, there is no baseline. The skill *exists*; the audit measures the *current* skill's resistance to pressure scenarios.

So audit-mode runs:

1. Pressure scenarios **with the skill loaded** (always).
2. Captures rationalizations the skill *failed* to prevent.
3. Reports them as `P7` findings (severity: `block`).

There is no GREEN phase in the audit-only Phase 1 — the audit just reports what bypasses the current skill. GREEN happens in Phase 2 (refactor), after edits are applied.

### Scenario selection

Use the templates from `../../author-skill/references/pressure-scenario-templates.md`. Pick scenarios that match the audited skill's *shape*:

- **Discipline** — Scenarios A/B/C combining time + sunk cost + authority + exhaustion + social pressure.
- **Technique** — Application, edge-case, missing-information scenarios.
- **Pattern** — Recognition, application, counter-example scenarios.
- **Reference** — Retrieval, application, gap-test scenarios.
- **Procedural** — Procedural skills usually have a discipline core (the guardrails); pressure-test those.

For first-pass audit, **3 scenarios** is the budget. Refactor cycles in Phase 2 may iterate the same scenarios.

### What counts as a "fail"

A skill *fails* a pressure scenario if the subagent (with the skill loaded):

- Picks the wrong option (A/B/C) per the scenario's documented correct answer.
- Picks the correct option but in their reasoning hints at a loophole ("I'll comply this time, but if X were true I'd consider Y") — this is a *partial fail*, severity `warn`.
- Cites the skill but applies it incorrectly — flag as `warn` and add a note about which section confused the subagent.
- Doesn't mention the skill at all (the description didn't route invocation) — this is a *D-series fail* (description didn't fire), not a P-series fail.

### Mapping to findings

| Pressure outcome | Finding |
|------------------|---------|
| Subagent picked correct option, cited skill, no rationalization | No finding — skill passed |
| Subagent picked correct option but rationalized a loophole | `P7.<scenario-id>` severity `warn`, with the rationalization quoted verbatim |
| Subagent picked wrong option | `P7.<scenario-id>` severity `block`, with the wrong choice and the reasoning |
| Subagent didn't mention skill | `D-series` finding (description didn't route) |

### What to write in the findings file

Each pressure-test finding includes:

```json
{
  "id": "P7.scenario-A",
  "severity": "block",
  "title": "Skill bypassed under time+authority+sunk-cost pressure",
  "result": "fail",
  "evidence": "Subagent chose option B with reasoning: '<verbatim quote>'",
  "catalog_ref": "P7",
  "scenario_id": "discipline-A",
  "phase": "audit"
}
```

Verbatim subagent quotes are the load-bearing artifact — they're what makes the refactor pass concrete. Don't paraphrase.

## Refactor phase — the GREEN side

When the user opts into Phase 2 and edits land, re-run the **same** scenarios. The audit-skill's REFACTOR cycle (capped at 3) is:

1. Apply tightening (from per-finding diffs).
2. Re-dispatch the same scenarios — same prompts, fresh subagent.
3. Capture new outcomes.
4. If a previously-failing scenario now passes: that finding is `resolved`.
5. If a previously-failing scenario still fails: that finding survives — propose further tightening.
6. If a new scenario fails (the tightening introduced a regression or surfaced a new loophole): that's a new finding, cycle again.

Cap at 3 cycles. If REFACTOR isn't converging by cycle 3, the skill design has a deeper problem than the conventions check — halt and surface.

## Scenario reuse across audits

When an audit completes, the surviving (converged-on-pass) scenarios are *appended* to the target skill's `evals/evals.json`. They become the test set for future audits.

This means: each audit makes the next audit stronger. The conventions catalog doesn't change every audit, but the per-skill eval set grows with every refactor pass.

## When pressure testing is overkill

For pure **reference** skills (API docs, syntax tables), pressure testing is low-value — you can't pressure-bypass a reference. Run scenarios anyway, but tag them as `severity: info` in the report. The audit's value for reference skills comes from the static + semantic checks (TOC, one-level-deep, no persuasion language).

For pure **pattern** skills (mental models), pressure testing is medium-value — the test is recognition under unusual context, not compliance under pressure. Use the recognition / counter-example scenarios from the pressure-scenario-templates, not the discipline scenarios.

For **discipline** and **procedural** skills, pressure testing is the centerpiece of the audit. Don't skip.
