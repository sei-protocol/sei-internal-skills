# Chaos Suite Guardrails

The full safety model for the chaos-suite skill. The SKILL.md guardrails stanza is the summary; this document is authoritative.

## Scope

This skill operates on **dev and staging Sei clusters ONLY**. It refuses to run against production.

## Allowed kubectl Contexts

The skill reads `kubectl config current-context` and matches it against an allowlist:

- `dev-*`
- `staging-*`
- `release-*-dev`
- `release-*-staging`

Anything else — including ambiguous contexts like `default`, or prod patterns like `prod-*`, `mainnet-*`, `*-production` — causes the skill to refuse immediately.

**TODO (before operationalizing):** verify these patterns match the actual context naming at Sei. Update the list here and in `scripts/context-check.sh` (when authored). If the convention differs, update both.

## Pre-Flight Checks

Before the first `kubectl apply` of any run:

1. **Context allowed.** `kubectl config current-context` matches an allowlist pattern.
2. **Env gate.** `CHAOS_SUITE_ALLOW=1` is set.
3. **Namespace exists.** The target namespace contains the expected Sei workload pods.
4. **Baseline healthy.** Height progressing, no pod crashloops, mempool under threshold.
5. **No unresolved prior run.** `state/` has no incomplete `run-<ts>/` directory (or the user has explicitly chosen to archive it and start fresh).

Any failure → refuse to start.

## Scope Confirmation Ritual

On the first side-effecting call of every run, the skill echoes:

```
About to start chaos suite:
  Cluster context:  <context>
  Namespace:        <ns>
  Release version:  <version>
  Test count:       <N>
  Expected runtime: ~<X> minutes
  Results will be written to: <path>

Type 'confirm' to proceed:
```

The user must type `confirm` verbatim. Any other response aborts — no partial apply, no "probably yes" parsing.

## Destructive Actions Requiring Extra Confirmation

Even inside the skill's happy path, these actions re-prompt the user:

- **Force pod-restart after leak detection.** If the leftover check finds residue, the skill NEVER auto-restarts. It surfaces the leaked state and asks the user to approve force-restart or request manual inspection. The response is captured in `state/run-<ts>/audit.log`.

## Anti-Corruption

- The skill never modifies the final summary artifact during a run. All intermediate writes go to `state/`. The artifact is written exactly once, at the end, by `collate-summary.sh`.
- Interrupted runs: next invocation detects the incomplete `state/run-<ts>/` directory and offers resume / archive / start-fresh. Silent overwrite is never an option.
- Every command that modifies cluster state appends to `state/run-<ts>/audit.log` with timestamp, command, exit code, stdout/stderr snapshot.

## Unsafe Patterns (Never Pre-Approved)

These are always interactive, even if the user tries to allowlist them:

- `kubectl delete` against any resource NOT owned by a chaos CR.
- `kubectl exec` against a Sei node pod.
- `kubectl apply` against resources outside the chaos CR template set.
- Any operation in a namespace that isn't the designated chaos test namespace — refuse, don't prompt.

## Secrets Handling

The skill never reads or writes secrets (no kubeconfig edits, no env dump to state). If any step seems to need a secret, that's a signal the skill's design is wrong — raise it with the user, don't work around it.
