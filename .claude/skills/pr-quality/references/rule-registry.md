# pr-quality — Rule Registry (v1)

The fixed set of rules the coordinator dispatches. **The registry is per-release**: adding, removing, or modifying a rule requires a PR to this file + the corresponding `judges/<rule>.md`. Runtime expansion is refused per SKILL.md guardrails.

## v1 active rules (5 + brevity dispatch)

| Rule ID | Mechanism | File scope | Severity | Cites |
|---|---|---|---|---|
| `no_cpu_limits` | YAML-AST | `*.yaml`, `*.yml` | `warn` | `feedback_no_cpu_limits` |
| `harbor_ecr_convention` | path-scoped grep | `clusters/harbor/**` | `warn` | `feedback_harbor_ecr_convention` |
| `narration_comments` | LLM-judge (n=3) | `*.go`, `*.py`, `*.ts` — function-doc above declarations | `warn` (3/3) or `nudge` (2/3) | `feedback_narration_comments` |
| `temporary_migration_notes` | LLM-judge (n=3) | `CLAUDE.md`, `AGENTS.md`, `README.md`, `docs/**` | `warn` (3/3) or `nudge` (2/3) | `feedback_temporary_migration_notes` |
| `authoritative_voice` | LLM-judge (n=3) | `.claude/skills/**/*.md` | `warn` (3/3) or `nudge` (2/3) | `feedback_authoritative_voice` |
| (dispatch) `/brevity` | skill-loaded subagent | PR body + comment-line additions | `nudge` | `feedback_concise_in_code_comments`, `feedback_authoritative_voice` |

## v1 deferred dimensions + un-defer triggers

| Dimension | Un-defer trigger |
|---|---|
| Documentation completeness | First PR that ships without a required package/file doc and a reviewer flags it manually. |
| Reference drift (broken links, stale wikilinks) | First stale `[[wikilink]]` or dead PR reference that causes measurable confusion (someone has to ask "what is X"). |
| Commit message hygiene | First non-Conventional-Commits commit that lands on `main`. Conventional Commits is already in CLAUDE.md and rarely violated; automating it would be solving a non-problem today. |

When a trigger fires, the un-defer PR adds: the rule entry to this registry, the `judges/<rule>.md`, any `references/judges/` prompt assets, and one or more eval cases.

## Memory entries explicitly NOT in v1

These were considered during the scope cut and excluded:

| Memory entry | Why excluded |
|---|---|
| `feedback_concise_in_code_comments` | Overlaps with `narration_comments`; lower precision; folded into the brevity dispatch on PR body. |
| `feedback_boring_clear_code` | High false-positive rate on legitimate defensive code; LLM judge precision insufficient. |
| `feedback_iam_scoping` | Fires on architecture decisions, not PR diffs; not PR-shaped. |
| `feedback_isolated_repo_clones` | Claude-actor convention, not a code/doc convention; irrelevant to PR diffs. |

## Adding a rule

1. PR to this file with the new row.
2. New `judges/<rule>.md` with prompt + scope + few-shot examples.
3. New eval case in `evals/evals.json`.
4. Cross-review by `reviewer` (convention fit) + `product-engineer` (mechanism coherence).
5. Ship.
