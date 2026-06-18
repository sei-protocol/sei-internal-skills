# pr-quality — Rule Registry (v1)

The fixed set of rules the coordinator dispatches. **The registry is per-release**: adding, removing, or modifying a rule requires a PR to this file. Runtime expansion is refused per SKILL.md guardrails.

## Knobs

| Knob | Value | What it does |
|---|---|---|
| `size_threshold_lines` | 5000 | Skill halts cleanly on PRs larger than this (judge precision degrades; defer to human review) |

## v1 active rules (5 + brevity dispatch)

### Mechanical rules

| Rule | File scope | Predicate | Cites |
|---|---|---|---|
| `no_cpu_limits` | `*.yaml`, `*.yml` (excluding `kind: Kustomization`) | `scripts/scan-yaml-cpu.sh` walks parsed YAML for `cpu:` set inside any `limits:` block at deeper indent; emits `<file>:<line>` per match | `feedback_no_cpu_limits` — CPU limits cause throttling; set requests only |
| `harbor_ecr_convention` | added/modified diff lines from `clusters/harbor/**` | `scripts/scan-harbor-ghcr.sh` walks the unified diff, tracking `+++ b/<path>` + `@@` hunk headers; matches `+` lines containing `ghcr.io` | `feedback_harbor_ecr_convention` — Harbor workload images go to AWS ECR, not ghcr.io |

Both scripts produce stdout-only output (one `<file>:<line>` per finding), stderr-only logs. Severity = `warn` (high-precision mechanical).

### LLM-judged rules

| Rule | File scope | Cites | Prompt + few-shot |
|---|---|---|---|
| `narration_comments` | `*.go`, `*.py`, `*.ts` — function-doc style only | `feedback_narration_comments` | [`judges/narration_comments.md`](judges/narration_comments.md) |
| `temporary_migration_notes` | `CLAUDE.md`, `AGENTS.md`, `README.md`, `docs/**` | `feedback_temporary_migration_notes` | [`judges/temporary_migration_notes.md`](judges/temporary_migration_notes.md) |
| `authoritative_voice` | `.claude/skills/**/*.md` | `feedback_authoritative_voice` | [`judges/authoritative_voice.md`](judges/authoritative_voice.md) |

Per-judge output schema (every LLM-judge subagent returns):

```json
{
  "verdict": "violation" | "no_violation",
  "span": "<file>:<line>" | null,
  "citation": "<rule_id>",
  "confidence": "low" | "medium" | "high",
  "explanation": "one sentence, max 30 words"
}
```

`citation` MUST be the verbatim `rule_id` (closed enum: `narration_comments`, `temporary_migration_notes`, `authoritative_voice`). A citation outside the enum auto-coerces to `no_violation` — the judge is bound to the known rulebook, not free-form complaint.

Severity = `warn` for confidence=high; `nudge` for confidence=medium; `no finding` for confidence=low.

### Skill-dispatch

| Dispatch | Target | Applies to |
|---|---|---|
| Verbosity | `.claude/skills/brevity/SKILL.md` | PR body + comment-line additions |

The judge subagent loads brevity's SKILL.md (subagent-loads-target-skill — same pattern as `/coral`, `/council`) and returns findings against the input slice. **The judge file in `references/judges/` does NOT re-implement brevity's rules** — the standard lives in `/brevity` itself; the judge merely detects that verbosity matters here.

## v1 deferred dimensions + un-defer triggers

| Dimension | Un-defer trigger |
|---|---|
| Documentation completeness | First PR that ships without a required package/file doc and a reviewer flags it manually |
| Reference drift (broken links, stale wikilinks) | First stale `[[wikilink]]` or dead PR reference that causes measurable confusion |
| Commit message hygiene | First non-Conventional-Commits commit that lands on `main` |

## v1 deferred mechanisms + un-defer triggers

| Mechanism | Un-defer trigger |
|---|---|
| Self-consistency (n=3 sampling, 2/3 agreement) | First false-positive in real-PR output that 3-sample voting would have prevented |
| Severity-rank + 5-cap | Real PR produces >7 findings and reviewer reports the output reads as wallpaper |
| Anchored marker + hash dedupe | Comment spam complaint on repeated invocations against the same PR |
| Cost ceiling per PR | Single invocation budget overrun in real use |

## Memory entries explicitly NOT in v1

| Memory entry | Why excluded |
|---|---|
| `feedback_concise_in_code_comments` | Overlaps with `narration_comments`; lower precision; folded into brevity dispatch |
| `feedback_boring_clear_code` | High false-positive rate on legitimate defensive code |
| `feedback_iam_scoping` | Fires on architecture decisions, not PR diffs |
| `feedback_isolated_repo_clones` | Claude-actor convention, not a code/doc convention; irrelevant to PR diffs |

## Adding a rule

1. PR to this file with the new row.
2. If LLM-judged: new `judges/<rule>.md` with prompt + scope + few-shot examples.
3. If mechanical: new `scripts/scan-<rule>.sh` predicate (single-purpose; stdout = findings, stderr = logs).
4. New eval case in `evals/evals.json`.
5. xreview (via `/xreview`) for convention fit + `product-engineer` for mechanism coherence.
6. Ship.
