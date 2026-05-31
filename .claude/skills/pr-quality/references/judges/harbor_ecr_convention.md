# Judge: harbor_ecr_convention (mechanical)

Path-scoped grep. Not an LLM judge.

## Mechanism

1. Identify changed files under `clusters/harbor/**`.
2. For each, grep changed lines (from `gh pr diff`) for `ghcr.io`.
3. If matched, emit finding at the matched line.

## Output shape per finding

```json
{
  "verdict": "violation",
  "span": "<file>:<line>",
  "citation": "harbor_ecr_convention",
  "confidence": "high",
  "explanation": "Harbor workload images go to AWS ECR by convention; ghcr.io reference detected."
}
```

## Scope filter

- File path starts with `clusters/harbor/`
- Match restricted to added/modified lines in the diff (skip removed)

## False-positive cases (intentional non-issues)

- Comments referencing ghcr.io for historical context: in practice these are rare in `clusters/harbor/` and reviewable when surfaced; v1 doesn't try to distinguish comment from value (raw grep). Tolerable until proven noisy.

## Cites

Memory: `feedback_harbor_ecr_convention` — "default to AWS ECR for harbor workloads, not ghcr.io"
