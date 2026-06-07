---
name: pr-quality
category: output-quality
model: claude-opus-4-8
description: "Use when about to open a PR or reviewing one — 'opening a PR', 'run pr-quality on this', 'check my PR', '/pr-quality', '/pr-quality 94'. Fires before `gh pr create` (pre-PR mode — agent surfaces findings inline against the staged diff so the author can revise) and on demand against an existing PR (post-PR mode — posts a fresh PR comment with findings). Suggestive only — never gates merge. Anti-triggers: NOT for blocking PRs (use branch protection if you need gating); NOT for style auto-fix (use gofmt / prettier); NOT for license / IP / security scanning (separate tooling). For verbosity-only on agent output, use /brevity directly. For multi-component design / cross-review, use /council."
---

# pr-quality

A two-mode coordinator that runs a fixed v1 judge set against a PR's diff + body and surfaces findings — either inline (pre-PR) or as a single fresh PR comment (post-PR). Suggestive only. Silence on zero findings.

This skill is **agent-invoked and user-invocable**. No CI workflow, no GitHub App install, no secret management. Proactive trigger lives in `CLAUDE.md` / `AGENTS.md` working-agreement references.

## Guardrails

Before any side-effecting action:

1. **Mode check.** Pre-PR mode reads the staged diff (`git diff --cached` or the planned diff if PR isn't created yet) + the agent's planned body. Post-PR mode reads an existing PR via `gh pr view --json body` + `gh pr diff <PR>`. If neither is determinable, halt.

2. **Refusal conditions.** This skill will refuse to:
   - **Block merge.** No exit code on findings; the skill is suggestive by contract.
   - **Run rules outside the locked v1 set** documented in [`references/rule-registry.md`](references/rule-registry.md). Adding a rule is a PR against that file, not a runtime override.
   - **Edit code or push commits.** Pre-PR surfaces findings to the agent for revision; agent decides what to apply. Post-PR posts a comment only.
   - **Comment on closed or merged PRs.** Post-PR mode silently skips.
   - **Comment on someone else's PR without explicit user invocation.** Pre-PR is the agent's own pre-flight; post-PR requires the user to name the PR (`/pr-quality <PR>`).

3. **Halt conditions** (exit cleanly, no comment):
   - PR diff is empty.
   - PR exceeds the size threshold in `references/rule-registry.md` (default 5000 lines).
   - Any LLM judge subagent returns malformed output twice — log and abort that judge; continue with others.

## Procedure

The skill runs from a Claude Code session (interactive or agent-driven). Steps below are executed by Claude using the `Bash` and `Agent` tools; mechanical scans are deterministic scripts, LLM judges are subagent dispatches loading the per-rule prompt files.

### Pre-PR mode

Triggered when an agent is about to invoke `gh pr create` (per the working agreement in `CLAUDE.md` / `AGENTS.md`).

1. **Read the staged diff.** `git diff --cached` (or `git diff HEAD origin/main` if the agent has already pushed but not created the PR). Read the agent's planned PR body.
2. **For each rule** in [`references/rule-registry.md`](references/rule-registry.md) whose file scope matches the staged changes:
   - Mechanical rules: run the predicate script (`scripts/scan-yaml-cpu.sh`, `scripts/scan-harbor-ghcr.sh`).
   - LLM-judged rules: dispatch a subagent with the corresponding `references/judges/<rule>.md` as the prompt + the diff slice as input.
   - Brevity dispatch: dispatch a subagent that loads `.claude/skills/brevity/SKILL.md` and applies it to the planned PR body. (This judge is a *trigger detector*, not a re-implementation of brevity's rules — the standard lives in `/brevity` itself.)
3. **Surface findings inline.** Group by severity (`warn` first), sort by file path within tier. No comment posted — the agent uses findings to revise before `gh pr create`.
4. **The agent then revises** the body / code / both and re-runs the skill, or proceeds to `gh pr create` if findings count is acceptable.

### Post-PR mode

Triggered by `/pr-quality` (current PR by branch) or `/pr-quality <PR>` (explicit PR number).

1. **Read the PR.** `gh pr view <PR> --json body,headRefOid` + `gh pr diff <PR>`.
2. **For each rule in scope** — same dispatch as pre-PR mode.
3. **Aggregate findings.** Group by severity, sort by file path.
4. **Post a fresh comment** if findings count > 0:
   ```
   gh pr comment <PR> --body-file <(...)
   ```
   The comment body follows [`references/format-spec.md`](references/format-spec.md). No comment when zero findings — silence is the success state.

## Halt Conditions

Halt cleanly (no comment) if:

- PR diff is empty.
- PR exceeds the configured size threshold (see `rule-registry.md`).
- LLM judge subagent fails (returns malformed JSON) twice — log; continue with remaining judges; emit a finding-set without the failed judge.
- (Post-PR mode) PR is closed or merged at read time.

## What this skill doesn't do

- **Block merge.** Branch protection rules are the gating mechanism, not this skill.
- **Auto-fix.** Suggestions only.
- **Run unlocked rules.** The v1 set is in `rule-registry.md`. New rules are a PR.
- **Self-consistency / multi-sample judging.** v1 single-shot per LLM judge. Un-defer trigger: a real PR produces a false-positive that consistency would have caught.
- **Cap findings.** v1 uncapped. Un-defer trigger: a real PR's output reads as wallpaper (>~7 findings); add severity-rank-and-cap then.
- **Anchored comment / hash dedupe / PATCH-in-place.** v1 posts fresh comment each invocation. Un-defer trigger: comment-spam complaint on repeated invocations of the same PR.
- **Skip-pr-quality label / opt-out.** Local invocation; user just doesn't invoke. No label needed.

## References

- [`references/rule-registry.md`](references/rule-registry.md) — locked v1 rule set, scope per rule, un-defer triggers for deferred dimensions, size threshold knob
- [`references/judges/`](references/judges/) — one file per LLM-judged rule with prompt + scope + few-shot examples
- [`references/format-spec.md`](references/format-spec.md) — post-PR comment body format

## Output

**Pre-PR mode**: structured finding list surfaced to the agent (file:line, rule_id, one-sentence explanation). No external side effect.

**Post-PR mode**: one fresh PR comment per [`references/format-spec.md`](references/format-spec.md) when findings > 0. No comment on zero findings.
