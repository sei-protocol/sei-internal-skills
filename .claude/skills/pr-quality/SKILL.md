---
name: pr-quality
description: "Used by the .github/workflows/pr-quality.yml GitHub Action on pull_request: opened/synchronize/reopened. Coordinates a parallel-dispatch quality review against a fixed v1 rule registry (verbosity via /brevity; convention adherence — 5 rules: no_cpu_limits, harbor_ecr_convention, narration_comments, temporary_migration_notes, authoritative_voice). Posts a single anchored PR comment with findings, capped at 5, severity-ranked. Suggestive only — never gates merge. Opt-out via PR label skip-pr-quality. Anti-triggers: NOT for blocking PRs (use branch protection); NOT for style auto-fix (use gofmt/prettier); NOT for license/IP/security scanning (separate tooling); NOT for inline-rule expansion at runtime (rule registry is fixed per release; new rules require a PR). For multi-component design / cross-review, use /council. For brevity-only on agent output, use /brevity directly."
---

# pr-quality

A PR-time coordinator that runs as a GitHub Action. Receives a PR diff + body + commits; dispatches a fixed set of judges in parallel; posts a single anchored comment with up to 5 findings ranked by severity. **Suggestive only — never gates merge.** Silence is the success state.

This is a **procedural skill encoding a runtime contract**. The contract — what rules fire, how findings are ranked, when to comment, when not to comment — is fixed per release. The skill is what an unattended CI script reads to enforce that contract; it has no human in the loop to negotiate exceptions.

## Guardrails

This skill posts **suggestive PR comments**, never gates merges. Before any side-effecting action:

1. **Surface check.** Confirm the workflow is running on a `pull_request` event (`opened`, `synchronize`, or `reopened`). If invoked outside that context, halt with a clear log message — there's no PR to comment against.

2. **Opt-out check.** If the PR has the label `skip-pr-quality`, halt before fetching diff. The label is the contract; respect it.

3. **Refusal conditions.** This skill will refuse to:
   - **Block merge.** No `failure` exit code on findings. Only infrastructure failures (missing API key, GitHub auth) produce failed runs.
   - **Post more than 5 findings.** Hard cap. When over cap, drop by (severity ascending, then mechanism LLM > mechanical), surface truncation explicitly in the comment footer.
   - **Spam comments on force-push.** Idempotency via the anchored marker `<!-- tide-pr-quality | sha=<HEAD_SHA> | findings-hash=<sha256> -->`. If findings-hash matches the previous run, skip the PATCH entirely.
   - **Post a comment when zero findings.** No thumbs-up, no "looks good" confirmation. Silence is the success state.
   - **Run rules outside the locked v1 list.** The 5 rules + the brevity-dispatch are the only judges that fire. Adding new rules requires a PR that updates `references/rule-registry.md`. No inline rule expansion.
   - **Add interactive features at runtime.** No slash-command replies (`/show-all`, `/expand`, etc.), no `issue_comment.created` triggers, no thread reply parsing. v1 is one-shot render-and-post.
   - **Future-proof beyond v1.** No marker versioning, no bot-rotation handling, no manual-edit detection in the comment lookup. These are deferred concerns; if they become real, a PR addresses them. Anticipation is feature creep.
   - **Edit code or push commits.** Read + PR-comment-write scope only.
   - **Comment on closed or merged PRs.** No drive-by suggestions after the fact.

4. **Halt-and-surface conditions** (workflow-run-fail with log, no PR comment posted):
   - PR diff is empty or only touches `.github/workflows/pr-quality.yml` itself (the bot does not review its own changes).
   - PR exceeds 5000 changed lines (judge precision degrades; defer to human review).
   - Any dispatched judge subagent fails twice in a row (transient errors retried once; persistent failure → no partial findings).
   - Workflow detects concurrent runs from `cancel-in-progress` → exit cleanly without partial state.

5. **Cost guardrail.** Per-PR budget cap: $1.00 worth of LLM tokens. If the dimension judges exceed budget mid-run, halt with truncation log and post whatever findings completed.

## Preconditions

- `anthropics/claude-code-action@v1` available in the workflow (canonical Claude Code GH Action).
- Repo secret `ANTHROPIC_API_KEY` set.
- Workflow permissions: `contents:read`, `pull-requests:write`, `id-token:write`.
- Repo label `skip-pr-quality` exists (for opt-out).
- `.claude/skills/brevity/` present (the verbosity dimension dispatches to it).
- `references/rule-registry.md` defines the locked v1 rule set; each rule has a dedicated `references/judges/<rule>.md` with prompt + scope + few-shot examples.

## Procedure

The CI workflow invokes this skill via `claude-code-action`. The skill executes steps below in order via `scripts/`. Each script is debuggable standalone via the `dispatch-judges.sh` orchestrator.

1. **Check opt-out label** (`scripts/check-optout.sh`). Read `gh pr view --json labels`. If `skip-pr-quality` present, exit 0 with log "opt-out label present; skipping". No comment, no state.

2. **Check PR-size guardrails** (`scripts/check-pr-size.sh`). Read `gh pr diff --name-only` + `gh pr diff | wc -l`. If empty diff OR only touches `.github/workflows/pr-quality.yml` OR over 5000 changed lines, exit 0 with log.

3. **Fetch shared context** (`scripts/fetch-context.sh`). Output written to `state/run-<PR>-<SHA>/context.json`:
   - PR diff (`gh pr diff`)
   - PR body
   - PR commits (`gh pr view --json commits`)
   - Changed-files list with path scope tags (yaml / go / py / ts / md / skill-md / durable-doc / harbor)
   - Memory snapshot (read `feedback_*.md` entries for the 5 locked rules, scope-filtered to entries actually applicable based on changed files)

4. **Dispatch judges in parallel** (`scripts/dispatch-judges.sh`). Cap parallelism at 5. Each judge is one of:

   **Mechanical** (high-precision, single-shot):
   - `no_cpu_limits` — YAML-AST parse of changed `*.yaml` / `*.yml`; walk `resources.limits.cpu`. Pre-render Helm/Kustomize where applicable to avoid templating false-positives.
   - `harbor_ecr_convention` — grep for `ghcr.io` in diff lines from `clusters/harbor/**`.

   **LLM-judged** (self-consistency n=3 at temp 0.3, require 2/3 agreement):
   - `narration_comments` — function-doc style only (comment immediately above `func/def/function` declaration in `*.go` / `*.py` / `*.ts`).
   - `temporary_migration_notes` — durable docs only (`CLAUDE.md`, `AGENTS.md`, `README.md`, `docs/**`).
   - `authoritative_voice` — skill content only (`.claude/skills/**/*.md`).

   **Skill-dispatch** (compose with /brevity):
   - Verbosity dimension: dispatch a subagent that loads `.claude/skills/brevity/SKILL.md` and applies it to PR body + comment-line additions. Returns brevity's verdict + suggested rewrite.

5. **Aggregate findings** (`scripts/aggregate.sh`). Read each judge's verdict JSON. Build a unified findings list:
   - Severity = `warn` (mechanical OR LLM 3/3 consensus) or `nudge` (LLM 2/3 consensus). LLM <2/3 → drop.
   - Dedup hash: `(file, line, rule_id)`. Mechanical wins on tie.
   - Sort: severity ascending → mechanism ascending (warn-mechanical, warn-LLM, nudge). Within tier, sort by file path.
   - Apply 5-cap: drop from tail. Track `suppressed_count` + `suppressed_rules`.

6. **Render comment** (`scripts/render-comment.sh`). Output `state/run-<PR>-<SHA>/comment.md`. Format is owned entirely by [`references/format-spec.md`](references/format-spec.md) — the rendering contract (marker shape, title, finding bullet shape, suppressed-findings disclosure block, disclaimer footer) lives there. This step writes the file; the spec defines the bytes.
   - Disclaimer: `Suggestive only; humans decide. Opt out via label \`skip-pr-quality\`.`

7. **Post or update anchored comment** (`scripts/post-or-update.sh`):
   - Find existing bot comment by marker prefix `<!-- tide-pr-quality |`
   - Extract previous findings-hash from the marker
   - If previous hash == current hash → exit 0 (no-op, no churn)
   - If no existing comment AND findings count == 0 → exit 0 (silence on clean PR)
   - If no existing comment AND findings count > 0 → `gh pr comment` (create new)
   - If existing comment AND findings count == 0 → DELETE the previous comment (clean PR after fixes deserves a fresh slate; do not leave stale findings)
   - If existing comment AND findings count > 0 → PATCH in place via `gh api`

8. **Exit** with code 0 in all happy + halt-with-log paths. Exit non-zero ONLY on infrastructure failure (auth, missing API key, judge double-failure).

## Halt Conditions (workflow-level)

Exit non-zero with a clear log message (no PR comment) if:

- `ANTHROPIC_API_KEY` secret missing.
- `gh` CLI auth fails.
- Required workflow permissions missing (e.g., `pull-requests:write` not granted).
- A dispatched judge fails twice in a row.
- Cost guardrail tripped before any judge completes.

For all soft-halt conditions (opt-out label, empty diff, oversized PR, concurrent-run cancellation), exit 0 with log — these are expected scenarios, not failures.

## State Management

Per-run state at `state/run-<PR>-<SHA>/`: `context.json` (fetched PR context), `judges/<rule_id>.json` per judge, `aggregated.json` (post-dedup, post-cap), `comment.md` (rendered body), `audit.log` (timestamped log). `state/` is gitignored at repo level. CI uploads the whole directory via `actions/upload-artifact` for debugging.

## What this skill doesn't do

- **Block merge.** Use branch protection rules if you need gating.
- **Auto-fix.** Suggestions only; the author + reviewers apply changes. Use `gofmt`, `prettier`, `forge fmt` for style auto-fix.
- **Run unlocked rules.** v1 rule registry is in `references/rule-registry.md`. Expansion is a PR, not runtime.
- **Provide interactive features.** No slash-commands in PR comments, no reply parsing. v1 is one-shot render-and-post.
- **Handle bot identity rotation.** v1 assumes a single bot account. If the bot identity changes, the marker matcher is rebuilt in a PR.
- **Comment on closed/merged PRs.** v1 only runs on open PRs.
- **Cover deferred dimensions.** Documentation completeness, reference drift, commit message hygiene are out of v1 scope. Each has un-defer triggers in `references/rule-registry.md`.

## References

- [`references/rule-registry.md`](references/rule-registry.md) — locked v1 rule set + un-defer triggers for deferred rules
- [`references/judge-prompt-template.md`](references/judge-prompt-template.md) — structured-output prompt shape + few-shot pattern
- [`references/format-spec.md`](references/format-spec.md) — finding rendering format
- [`references/judges/`](references/judges/) — one file per rule with prompt + scope + few-shot
- [`references/guardrails.md`](references/guardrails.md) — extended safety model

## Output

User-visible: one anchored PR comment (when findings > 0), per [`references/format-spec.md`](references/format-spec.md). Silence on zero findings. Prior comment deleted when a re-run yields zero findings. Operator-visible: `audit.log` + CI artifact `pr-quality-artifacts/` with full state for debugging.
