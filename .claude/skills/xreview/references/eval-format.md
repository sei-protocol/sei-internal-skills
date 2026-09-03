# Eval Format

> Moved here from `author-skill` when that skill was cut. It survives because the rubric's
> **E4** and **E5** rules cite this vocabulary — `compliance_signals`, `forbidden_signals`,
> `source` — and a rule whose terms are defined nowhere cannot be applied. The authoring
> workflow around it did not survive; this is the schema alone.

Every skill ships with at least two evals: one happy-path and one halt-condition. The pressure scenarios from RED-GREEN-REFACTOR convert directly — they're already the test cases.

## File location

`<skill>/evals/evals.json`

## Schema

```json
{
  "skill": "<skill-name>",
  "version": "1",
  "evals": [
    {
      "id": "happy-path-1",
      "type": "happy-path",
      "scenario": "<verbatim text of the scenario the agent encounters>",
      "skill_loaded": true,
      "expected": {
        "choice": "A",
        "compliance_signals": [
          "agent cites <skill-name> explicitly",
          "agent identifies the rationalization and rejects it",
          "agent picks option A"
        ],
        "forbidden_signals": [
          "agent picks option B or C",
          "agent applies any of the rationalizations from the table"
        ]
      },
      "source": "RED scenario 1 — survived REFACTOR cycle 2"
    },
    {
      "id": "halt-condition-1",
      "type": "halt-condition",
      "scenario": "<a scenario that should trigger one of the skill's halt conditions>",
      "skill_loaded": true,
      "expected": {
        "halt": true,
        "halt_reason": "<which halt condition fires>",
        "compliance_signals": [
          "agent surfaces the halt condition to the user",
          "agent does not auto-remediate"
        ]
      },
      "source": "constructed from guardrails halt-conditions list"
    }
  ]
}
```

## Eval types

### `happy-path`

The agent encounters a realistic scenario, the skill is loaded, the agent should comply with the skill's procedure.

Compliance signals (what to look for in the response):

- Cites the skill explicitly.
- Walks through the procedure in order.
- Picks the correct option on A/B/C choices.
- Identifies rationalizations and rejects them per the table.

### `halt-condition`

A scenario that should trip one of the skill's documented halt conditions. The agent should *stop* and report, not auto-remediate.

Compliance signals:

- Surfaces the halt condition.
- Reports what state is dirty / what was captured / what's incomplete.
- Asks the user for remediation rather than proceeding.

### `adversarial` (optional, the Obra 3-eval ideal)

A scenario specifically designed to *break* the skill. Combines maximum pressure with edge-case ambiguity. Used to find loopholes that survived REFACTOR.

Compliance signals:

- Same as happy-path, but under harder conditions.
- Bonus: agent self-corrects mid-response when they detect they're rationalizing.

### `discipline`

A scenario that exercises a *standing rule the skill must hold every time it acts* — an authoring/formatting contract, an attribution invariant, a per-case substantiation rule — rather than a single procedure run or a stop-and-report halt. Scored with **happy-path semantics**: pass only if *all* `compliance_signals` match and *no* `forbidden_signals` match. Use it when the rule is always-on (e.g. "every written line renders clean", "Status carries exactly one of three literals") and a plain happy-path eval wouldn't pin the specific defect the rule exists to prevent. A `discipline` scenario whose correct behavior is to *stop* (a `>1 match → halt`) is better written as a `halt-condition`; reserve `discipline` for the must-always-hold rules.

Compliance signals:

- Applies the rule under the named pressure without being reminded of it.
- Rejects the specific defect the rule prevents (the forbidden_signals are the historical failure modes).

## Running evals

Evals do not run automatically. They ship *with* the skill so a later invocation, a CI job, or a rubric lens checking E1-E5 can run them.

**Manual run:**

```bash
# Inside Claude Code, with the skill present:
# 1. Open the skill's evals/evals.json
# 2. For each eval, dispatch a subagent with the scenario.
# 3. Compare the response against expected.compliance_signals.
# 4. Pass if all compliance_signals match and no forbidden_signals match.
```

Nothing automates this today.

## When to add more evals

After shipping the skill:

- New rationalization observed in production → add an eval for it.
- New halt condition added → add a halt-condition eval.
- Model upgrade → re-run all evals; if any newly fail, REFACTOR.

Each new eval is the test for the next REFACTOR cycle. The skill grows by the evals it accumulates over time.

## Minimum bar (sei-internal-skills)

- 1 happy-path eval.
- 1 halt-condition eval.

## Target (Obra)

- 3 evals minimum: happy-path, edge-case, adversarial.

## Format anti-patterns

- ❌ Eval scenarios that are abstract ("test the agent's understanding of X"). Scenarios must be concrete — real file paths, real consequences, A/B/C choice.
- ❌ Compliance signals that are subjective ("agent is helpful"). Signals must be observable — "agent cites skill X" or "agent picks option A" or "agent quotes the rationalization table".
- ❌ Evals without a `source` field. Every eval traces to a RED scenario, a halt condition in guardrails, or a real production incident.
- ❌ Evals that test the model, not the skill. If an eval passes without the skill loaded (`skill_loaded: false`), the eval is testing baseline model capability, not the skill's contribution.
