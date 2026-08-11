---
name: chaos-suite
category: release-operations
model: claude-opus-5
description: "Release-testing skill that executes the full chaos test suite (runbook sei-protocol/platform#169) against a dev or staging Sei cluster and collates results into a release-summary document. Trigger on 'chaos suite', 'run release chaos tests', 'execute chaos testing for the release cut', 'run the chaos runbook'. NEVER triggers on production — this skill refuses to run if kubectl context matches a prod pattern. NOT for single-test debugging (use ad-hoc kubectl for that). NOT for runbook authoring (this skill executes a runbook, it does not write one)."
---

# Chaos Suite — Release Testing

Execute the full chaos test suite from runbook sei-protocol/platform#169 against a dev or staging Sei cluster, capture baseline / mid-chaos / post-chaos signals per test, verify clean recovery, run the Pod*Chaos leftover gate, and collate the results into a release summary document.

One invocation = one release cut's worth of chaos testing.

## Guardrails

This skill operates on **dev and staging Sei clusters ONLY**. Before any side-effecting action:

1. **Context check** — verify `kubectl config current-context` matches an allowed pattern. Refuse immediately on prod or ambiguous context. See `references/guardrails.md` for the allowlist.
2. **Env gate** — require `CHAOS_SUITE_ALLOW=1` in the invocation environment. Second gate against accidental invocation.
3. **Scope confirmation** — on the first `kubectl apply` of the run, echo target cluster, namespace, release version, test count, expected runtime, and results path. Require the user to type `confirm` verbatim. Any other response aborts.
4. **Refusal conditions** — refuse to start if:
   - Prod cluster context detected
   - `CHAOS_SUITE_ALLOW` not set
   - Baseline health check fails (cluster is already broken; don't layer chaos on top)
   - A previous unresolved run exists in `state/` with an unfinished test

See `references/guardrails.md` for the full safety model.

## Preconditions

- `kubectl` available and authenticated against a dev or staging Sei cluster
- Runbook sei-protocol/platform#169 loaded (human-readable — the skill references test IDs from it)
- Release version known (user provides as arg, or skill prompts)
- Write access to the results directory in the platform repo (`clusters/<env>/release-<version>/results/`)
- `CHAOS_SUITE_ALLOW=1` set

## Procedure

### Outer loop: for each test in the runbook

1. **Baseline capture** — `scripts/baseline.sh --test-id <id>` records height, sec/block, mempool depth, per-pod spread. Non-zero exit if cluster is unhealthy → HALT.
2. **Apply chaos CR** — `scripts/apply-chaos.sh --test-id <id>`. First invocation of the run triggers scope confirmation (per Guardrail 3).
3. **Schedule mid-chaos sampling** — use `ScheduleWakeup` to wake up at `chaos_duration * 0.5` into the window. When it fires:
   - `scripts/sample-mid.sh --test-id <id>` captures mid-chaos signals.
   - `scripts/verify-injection.sh --test-id <id>` confirms the chaos actually applied (log-timestamp delta for time-skew, `tc -s qdisc` for packet-loss, tproxy logs for HTTPChaos, etc.).
   - If injection verification fails → HALT.
4. **Schedule post-chaos sampling** — wake up at `chaos_expiry + buffer`. When it fires:
   - `scripts/sample-post.sh --test-id <id>` captures recovery signals.
   - `scripts/verify-cleanup.sh --test-id <id>` confirms clean recovery.
5. **Leftover check gate** — `scripts/leftover-check.sh --test-id <id>` runs the Pod*Chaos leftover check from runbook #169. If anything is leaked → HALT (do NOT auto-remediate).

### After all tests

6. **Collate** — `scripts/collate-summary.sh --release-version <v>` reads every per-test state file from `state/run-<ts>/` and produces the summary document per `references/summary-template.md`.
7. **Write artifact** — summary lands at `<platform-repo>/clusters/<env>/release-<version>/results/<YYYY-MM-DD>-summary.md`.
8. **Session-end summary** — in-chat: pass/fail counts, FAIL tests with the specific signal that failed, any leaked state that required human intervention, Platform action items, Protocol action items.

## Halt Conditions

Stop and report to the user if:

- **Injection verification fails** — chaos CR applied but the target behavior didn't change. State captured. User decides: retry, skip, or abort.
- **Leftover check finds residue** — Pod*Chaos left pods in a bad state. Surface exactly what's leaked. NEVER auto-restart.
- **Baseline is unhealthy** — cluster has pre-existing issues. Report the failing signal. Don't layer chaos.
- **kubectl context drift** — current context changed mid-run (e.g., user switched contexts in another terminal). Halt and re-confirm before continuing.
- **Unknown test ID in the runbook** — runbook and skill have diverged. Halt rather than skip.
- **Scripts exit non-zero for an unexpected reason** — surface stdout/stderr + audit log. Don't retry silently.

## State Management

- Per-run state: `state/run-<ISO-timestamp>/<test-id>.yaml`. Each test aggregates baseline / mid-chaos / post-chaos / leftover-check results into its file across the run's timeline.
- Audit log: `state/run-<ts>/audit.log` records every command, exit code, output snapshot.
- Resumability: on startup, if `state/run-<ts>/` exists with unfinished tests, offer resume / archive / start-fresh.
- `state/` is gitignored (the repo's `.gitignore` covers `.claude/skills/*/state/`).

## Summary Output

Artifact format: `references/summary-template.md`. Mirrors the reference format the team converged on in the release-6-5 session (per sei-protocol/platform#170).

## Permission Pre-Approval

Pre-approve in `.claude/settings.local.json` (user-specific, not committed):

- `kubectl config current-context`
- `kubectl get` patterns against the expected namespace
- `kubectl apply -f` against the chaos CR templates directory
- `kubectl describe` patterns
- Read-only cluster inspection commands

Leave interactive (never pre-approve):

- `kubectl delete`
- `kubectl exec` against Sei node pods
- `kubectl apply` against anything not a chaos CR
- Any command targeting a namespace outside the designated chaos test namespace

Use the `fewer-permission-prompts` skill against a real run's transcript to generate the allowlist.

---

## Status: Scaffold

This skill follows `../SKILL-TEMPLATE.md` but the `scripts/` directory contains placeholders only. To operationalize:

1. Port the mechanical steps from the release-6-5 chaos session into each script under `scripts/`. See `scripts/README.md` for the per-script authoring plan.
2. Fill in the runbook test IDs and parameters from sei-protocol/platform#169.
3. Confirm the cluster context allowlist patterns in `references/guardrails.md` match Sei's actual context naming.
4. Decide whether to keep the skill in the sei-internal-skills repo or move it to the platform repo. It can only be invoked when Claude Code's CWD is inside that repo — if chaos runs happen from the platform repo, move it there.
5. Write happy-path and halt-path evals in `evals/evals.json`.
