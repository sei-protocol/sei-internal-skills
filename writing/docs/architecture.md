# Architecture

This document follows arc42 and omits an empty section.

## 1. Introduction and goals

Give any AI model the smallest context that makes its written output better, and make the
improvement explainable and measurable by a third party.

Three quality goals, in priority order:

1. **Explainability.** Every constraint traces to a published standard. No constraint
   exists only inside a prompt.
2. **Verifiability.** A reviewer can check a finished artifact, and needs no knowledge
   of its origin.
3. **Portability.** The same contract applies in a chat interface, in an agent harness,
   and in CI.

## 2. Constraints

- ASD owns the copyright in ASD-STE100. The repository MUST NOT contain the
  specification or its dictionary. See `writing/NOTICE.md`.
- Vale checks patterns. It has no model of meaning, so a rule that needs judgement
  cannot exist.
- Model behaviour changes between versions. Anchor recognition is therefore a test
  result with a date, not a property.

## 3. Context and scope

```
        anchors/registry.yaml
                 |
     +-----------+-----------+
     |                       |
 generation               verification
     |                       |
  AGENTS.md            styles/*.yml
  system prompts             |
     |                       |
     +----------+------------+
                |
             artifact
                |
             evals/
```

The registry is the only place a human edits an anchor. Both sides derive from it, so
the generation hint and the verification rule cannot drift apart.

## 4. Solution strategy

| Concern | Decision |
|---|---|
| Compress convention into context | Name a public standard (a semantic anchor) |
| Prove the artifact meets the convention | Vale rules that cite the standard |
| Prove the anchor still resolves | Recognition tests, per model version |
| Prove the rules still work | Fixture corpus with expected findings |
| Prevent drift between the two sides | Generate the prompt from the registry |
| Publish an artifact without drift | Build it with a script, from git and the canonical style |

### Plans and ensures

An anchor behaves like a plan: it holds intent, and the intent disappears if the context
does not carry it. A Vale run behaves like an ensure. It recomputes the verdict from the
artifact alone, every time, with no memory of the session that produced it.

Correctness MUST rest on the ensure. The plan is an optimisation that makes the first draft
closer to correct, which is worth having, and which you MUST NOT depend on.

## 5. Building blocks

| Block | Responsibility |
|---|---|
| `writing/anchors/registry.yaml` | Declares anchors, coverage, and recognition tests |
| `writing/anchors/<id>.md` | Human-readable normative summary and citation |
| `writing/styles/AgenticWriting/` | One Vale rule per checkable constraint |
| `writing/styles/config/vocabularies/AgenticWriting/` | Project Technical Names, reviewable in Git |
| `writing/scripts/` | The generators, the gates CI runs, and `lint.sh` |
| `writing/evals/` | Recognition tests and rule regression fixtures |

Two blocks in the plan are not in the tree yet, and PR #364 brings both.
`writing/scripts/render-context.py` turns the registry into the generated context
files. `writing/scripts/sync-openste.sh` turns the MIT wordset into the approved-word
rule.

## 9. Decisions

See `writing/docs/adr/`.

## 10. Quality requirements

- WHEN the registry changes, CI SHALL fail if a generated file is stale.
- WHEN a rule enters the repository, the author SHALL add a fixture line that the rule
  catches.
- WHILE a model version is current, the recognition matrix SHALL hold a result for it.
- IF a rule needs judgement, THEN the registry SHALL list it under `not_checkable`
  instead.

## 11. Risks and technical debt

| Risk | Mitigation |
|---|---|
| False positives train writers to silence rules | Default new rules to `suggestion`; promote only after fixtures pass |
| Partial coverage read as full compliance | Every anchor carries a `coverage` field, and the README states the gap |
| Anchor recognition regresses silently on upgrade | Recognition matrix, re-run per model version |
| Flat STE prose applied where voice matters | `applies_to` field; STE is not anchored for narrative text |

## 12. Glossary

**Anchor** — a well-defined public term used as a reference point in model context.
**Coverage** — how much of a standard the verifier can check: full, partial, or none.
**Recognition test** — the open question that tests whether a model resolves an
anchor, and the verdict it produces.
**Ensure** — a check that recomputes its verdict from the artifact, with no memory.
