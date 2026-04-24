# chaos-suite scripts

These scripts are **scaffold placeholders**. Each represents one deterministic step of the chaos suite procedure. To operationalize the skill, author these scripts against the actual cluster operations from the release-6-5 session and runbook sei-protocol/platform#169.

## Contract for every script

- Takes a single required arg: `--test-id <id>`. Other args as needed.
- Writes YAML state updates to `../state/run-<TS>/<test-id>.yaml`, merging with existing content (don't overwrite).
- Appends to `../state/run-<TS>/audit.log`: timestamp, command invoked, exit code, stdout snapshot, stderr snapshot.
- Exits 0 on success, non-zero on failure. The SKILL.md procedure treats non-zero exits as signal to route to halt conditions.
- No embedded secrets. No cluster identifiers. Both come from env vars or `kubectl config`.
- Debuggable standalone — `./scripts/baseline.sh --test-id CH-TS-01` should work outside the skill for troubleshooting.

## Scripts to author

| Script | Purpose | Source material |
|--------|---------|-----------------|
| `context-check.sh` | Verify kubectl context is in the allowlist (`references/guardrails.md`). Exit non-zero on prod or ambiguous context. | `references/guardrails.md` patterns |
| `baseline.sh` | Capture pre-chaos signals: height, sec/block, mempool, per-pod spread. Exit non-zero if cluster unhealthy. | Baseline section of release-6-5 session |
| `apply-chaos.sh` | Apply the chaos CR for a given test. Triggers scope-confirmation prompt on first run. | Chaos CR templates from runbook #169 |
| `sample-mid.sh` | Capture mid-chaos signals for the specific test. Signals vary by chaos type. | Mid-chaos sampling from release-6-5 |
| `verify-injection.sh` | Verify the chaos actually applied. Per-chaos-type: `tc -s qdisc` for packet-loss, log-timestamp delta for time-skew, tproxy logs for HTTPChaos, etc. | Per-chaos-type verification from runbook #169 |
| `sample-post.sh` | Capture post-chaos recovery signals. | Post-chaos sampling from release-6-5 |
| `verify-cleanup.sh` | Verify clean recovery after chaos expiry. | Recovery verification from release-6-5 |
| `leftover-check.sh` | Pod*Chaos leftover gate from runbook #169. Exits non-zero on any residue. Never remediates. | Runbook #169 Pod*Chaos leftover section |
| `collate-summary.sh` | Read all `../state/run-<TS>/*.yaml` and produce the summary artifact per `../references/summary-template.md`. Write to the target path in the platform repo. | `references/summary-template.md` |

## Authoring tips

- **Idempotence where reasonable.** Re-running a script mid-run should update the state file, not duplicate entries.
- **Structured output.** `kubectl get -o json` + `jq` is more reliable than `kubectl get` + grep. Emit YAML updates, not free text.
- **Fail loud.** On unexpected cluster state, exit non-zero with a specific message. Don't guess.
- **Per-chaos-type branches.** `verify-injection.sh` and similar need to switch on chaos type. Keep the dispatch table in a readable form (case statement, or a YAML config the script reads).
- **No silent kubectl contexts.** Every cluster-mutating command echoes the current context and namespace at the start of its run.
