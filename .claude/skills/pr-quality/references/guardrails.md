# pr-quality — Extended Safety Model

Detailed version of the Guardrails stanza in SKILL.md. Reference for what the workflow will and won't do, and why.

## Suggestive-only contract

The skill posts comments. It does not:
- Set commit status / required checks
- Push commits or edit files
- Lock conversations or apply labels
- Request reviewers

Branch protection rules handle gating. The skill is one signal among many; humans decide what's blocking.

## v1 rule registry is closed at runtime

The 5 rules + brevity dispatch are the only judges that fire in v1. The dispatch runner refuses to load any rule not present in `references/rule-registry.md`.

Why this matters: a procedural skill running in CI has no human in the loop. If a new rule is added inline ("I'll just add a check for X"), there's no review of the rule's precision, no eval coverage, no scope filter. Closed at runtime = the registry is the contract.

To add a rule: PR against `rule-registry.md` + `judges/<rule>.md` + `evals/evals.json`. The author-skill methodology applies (RED a generalist on the new rule; GREEN with the rule's few-shot prompts; require 2/3 self-consistency).

## Cost ceiling

LLM judges cost money. The per-PR cap is $1.00 (rough; tune in `scripts/dispatch-judges.sh`). When the cap is approached:
- Stop dispatching new judges
- Return whatever findings completed
- Log the truncation

Pathological PR (e.g., 5000-line monorepo refactor) hits the size guardrail FIRST and exits before cost guardrail engages. Cost guardrail is the secondary safety net.

## Idempotency contract

The marker shape `<!-- tide-pr-quality | sha=<HEAD_SHA> | findings-hash=<sha256> -->` carries enough information to dedupe AND detect content change:

- Same SHA + same findings-hash → bot was already invoked for this exact state; no-op.
- Same SHA + different findings-hash → not possible (judges are deterministic for fixed input); if observed, log as anomaly.
- Different SHA → re-run; if findings-hash matches → skip PATCH (no churn).
- Different SHA + different findings-hash → PATCH in place.

The workflow also uses GH Actions `concurrency` group with `cancel-in-progress: true` for the per-PR group, killing superseded runs.

## Comment ownership

The bot owns its anchored comment. Manual human edits to the bot comment ARE overwritten on the next run. The marker signals bot ownership; humans should reply in new comments, not edit the bot's body.

If this becomes an actual annoyance, the v2 PR can add manual-edit detection. v1 explicitly does not.

## Closed/merged PR behavior

The workflow only fires on `pull_request: opened, synchronize, reopened`. It does not fire on `closed` or `merged`. Comments left on a merged PR stay (they're historical record); the bot does not chase them.

## Opt-out contract

The label `skip-pr-quality` is the emergency escape valve. v1 treats it as a hard halt:
- Workflow exits 0 with log line
- No diff fetched, no judges dispatched
- Existing bot comment (if any) is left alone — the label says "skip", not "delete history"

To un-skip a PR, remove the label and the next push (or `synchronize` event) reactivates the bot.

## Cost / time profile

Approximate per-PR cost from community data on `claude-code-action`:
- Time: 30-120 seconds (3-5 judges in parallel + comment post)
- Tokens: 5k-20k input, 1k-5k output → ~$0.10-$0.50

Larger PRs (toward 5000-line cap) skew higher; smaller PRs (single-file fixes) closer to floor.

## What this skill is NOT a replacement for

- **Human review.** The bot surfaces patterns; humans review the change.
- **CI tests.** The bot does not run tests. Existing test workflows continue.
- **Branch protection.** Required checks are configured independently.
- **Security scanning.** Trivy, gitleaks, snyk, etc. live in separate workflows.
