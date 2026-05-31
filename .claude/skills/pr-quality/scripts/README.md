# Scripts

Deterministic steps used by the pr-quality skill, each debuggable standalone. The workflow (`.github/workflows/pr-quality.yml`) invokes `claude-code-action@v1` which loads the skill and follows the procedure in SKILL.md by calling these scripts in order.

| Script | Reads | Writes |
|---|---|---|
| `check-optout.sh` | `gh pr view --json labels` | exit 0 (skip) or continue |
| `check-pr-size.sh` | `gh pr diff` | exit 0 (skip) or continue |
| `fetch-context.sh` | `gh pr diff/view/commits` + memory | `state/run-<PR>-<SHA>/context.json` |
| `dispatch-judges.sh` | `context.json`, rule-registry, judges/ | `state/run-<PR>-<SHA>/judges/*.json` |
| `aggregate.sh` | `judges/*.json` | `state/run-<PR>-<SHA>/aggregated.json` |
| `render-comment.sh` | `aggregated.json`, format-spec | `state/run-<PR>-<SHA>/comment.md` |
| `post-or-update.sh` | `comment.md`, `gh pr view --json comments` | PATCH or create or DELETE bot comment |

All scripts log timestamped entries to `state/run-<PR>-<SHA>/audit.log`. None of them exit non-zero on expected halt conditions (opt-out, empty diff, oversized PR) — those are logged and exited 0.
