# Judge Prompt Template

All LLM-judged rules share this prompt shape. Per-rule prompts live in `judges/<rule>.md`; this file defines the contract every judge call follows.

## Required structure

```
You are judging whether a diff hunk violates the [rule_id] rule from Tide's convention set.

**Rule**: [one-sentence statement of the rule, verbatim from the memory entry]

**Scope**: [file globs the rule applies to]

**You will receive**:
- diff_hunk: the changed lines with file path
- context_before / context_after: ±10 lines around the hunk
- changed_file_path

**You must output structured JSON**:
{
  "verdict": "violation" | "no_violation",
  "span": "<file>:<line>" or null,
  "citation": "[rule_id]",  // verbatim from the closed enum below
  "confidence": "low" | "medium" | "high",
  "explanation": "one sentence, max 30 words"
}

**citation enum** (closed; emit verbatim or auto-fail):
- "no_cpu_limits"
- "harbor_ecr_convention"
- "narration_comments"
- "temporary_migration_notes"
- "authoritative_voice"

If you cannot decide between violation and no_violation, emit verdict=no_violation. Better to miss than to fabricate.

**Examples** (2 negatives + 3 positives minimum per rule; per-rule lists in judges/<rule>.md):
[5-shot block, per rule]

**Now judge**:
[diff_hunk with context]
```

## Self-consistency contract

Each LLM-judged rule runs n=3 samples at temp=0.3.

- 3/3 `violation` → severity `warn`
- 2/3 `violation` → severity `nudge`
- ≤1/3 `violation` → no finding emitted

The dispatch runner in `scripts/dispatch-judges.sh` enforces this; individual judges return a single verdict per sample.

## Closed-enum citation requirement

The `citation` field must be one of the enum values verbatim. If a judge emits a citation outside the enum, the dispatch runner treats the entire sample as `no_violation`. This is the Greptile / CodeRabbit pattern — bind the model to the known rulebook; refuse free-form complaints.

## Why this shape

- **Structured output** → deterministic aggregation; no regex on free-form prose.
- **One rule per call** → precision per Zheng et al. 2023; task-stacking craters precision.
- **±10 lines of context** → judge can distinguish a comment that narrates its own line vs. one documenting package intent.
- **Confidence field** → tie-breaker for severity ranking + 5-cap drop strategy.
- **Closed-enum citation** → eliminates the "model fabricates a violation" failure mode.
