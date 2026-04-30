# Severity Rubric

Severity is a launch-readiness call, not a code-quality call. The question every severity asks is: *if this fires in production, what happens, and how badly does it block shipping?*

Findings escalate up the rubric. Pick the highest tier where ANY clause is true.

## Critical

A Critical finding means **launch is unsafe until this is fixed**. Either the failure is unrecoverable, or it compromises trust in the system at a fundamental level.

Any of:

- **Data loss or corruption.** State that can't be reconstructed from inputs is silently lost or corrupted on a path the system actually exercises.
- **Security breach.** An attacker can exfiltrate secrets, escalate privileges, or impersonate a legitimate principal. Includes broken auth, broken authz, leaked credentials, signature bypass.
- **Funds at risk.** For on-chain components: an attacker can drain funds, lock funds permanently, or mint without authorization.
- **Cluster-wide outage path.** A single bad input or state can take down a shared component (controller, ingress, API server) for all tenants.
- **Silent broken-window invariant.** A core invariant the system relies on (job-to-tx-hash uniqueness, replica-set membership, attestation validity) can be violated without anyone noticing.
- **Irreversible bad state.** Once entered, the system cannot recover automatically and there is no documented manual remediation.

Critical findings ALWAYS block launch. A Critical finding being "named in a conditional ship-it verdict" means the team is committing to fix it before the launch event.

## High

A High finding means **launch is risky until this is fixed**. The failure is recoverable or partial, but the team would not want to ship without addressing it.

Any of:

- **Single-tenant outage path.** A single bad input or state can break one tenant's workflow without affecting others, but recovery requires manual intervention.
- **Recoverable data integrity issue.** State can be corrupted but is reconstructible from inputs (e.g., a cache that can be rebuilt). Still requires action.
- **Auth/authz weakness short of breach.** Permission checks have gaps that don't currently grant unauthorized access but would on a near-term feature addition (foreseeable).
- **Race condition on the happy path.** A concurrency bug that fires under realistic load (not just adversarial), even if recoverable.
- **Operational footgun.** A common operator action (rolling restart, scale event, credential rotation) leaves the system in a degraded state without warning.
- **Silent retry exhaustion.** A retry budget can quietly drain without alerting; eventual failure is loud, but the lead-up isn't visible.
- **Interface contract violation that breaks consumers.** Provider/consumer mismatch against the registry that produces wrong results, not just incompatible types.

High findings block launch unless explicitly accepted as a known risk by the user (and even then, only when documented).

## Medium

A Medium finding means **the team should fix this, but launch can proceed**. The failure mode is either narrow, loud, or not blocking.

Any of:

- **Narrow failure mode.** A bug that fires only under uncommon conditions (specific config combinations, edge-case inputs that never appear in normal use).
- **Loud failure with clean recovery.** A panic, error log, or failed reconcile that is observable, alerts somewhere already, and recovers cleanly on retry. The bug is real but the system handles it.
- **Resource inefficiency.** A bottleneck or O(n²) that's not currently load-bearing but would become one at the next scale jump.
- **Documentation/contract drift.** The code does the right thing, but the docs/registry/comments say otherwise. Not a runtime issue but a maintenance hazard.
- **Defensive gap.** A validation or invariant check missing on an input the system trusts from a controlled source. No current attack but the trust assumption is fragile.

Medium findings appear in the findings log, in the launch summary table, and are tracked outside this artifact (typically as `/issue` filings) — but they don't block launch.

## Low

A Low finding means **noted, but no action required for launch**. The artifact captures it so the team has a complete picture; nothing more is expected.

Any of:

- **Minor inefficiency** with no scaling implication.
- **Code smell or readability issue** that doesn't affect correctness.
- **Test gap** for a path that is correctly handled by other tests indirectly.
- **Comment/doc nitpicks** that aren't materially misleading.
- **Convention drift** (e.g., one file uses snake_case where the rest uses camelCase) without functional impact.

Lows are recorded but excluded from the convergence test — they keep surfacing forever and would prevent the loop from terminating. They also do not appear in launch verdicts.

## How to apply the rubric

1. **Identify the failure mode.** What goes wrong, on what path, under what conditions?
2. **Walk Critical first.** If any Critical clause is met, stop — it's Critical.
3. **Walk High next.** If any High clause is met, it's High.
4. **Walk Medium.** If any Medium clause is met, it's Medium.
5. **Otherwise Low.**

When in doubt between two tiers: **lean blocker**. A finding tagged High that should have been Medium costs one extra fix; a finding tagged Medium that should have been High costs a shipped bug.

## Disagreements

When finder and challenger disagree on severity:

- Disagreement of **one tier** (e.g., finder says High, challenger says Medium) → orchestrator picks the higher tier and notes the downgrade reason in the entry.
- Disagreement of **two or more tiers** (e.g., Critical vs. Medium) → both experts likely have different mental models of the failure mode. Orchestrator re-reads the candidate, asks one targeted clarifying question, and assigns the tier the rubric supports. If neither expert's framing matches the rubric, surface to the user.

Severity calls are part of the audit trail. Disagreements are not bugs in the rubric; they are signal.

## Why no "Informational" tier

Informational-tier findings (FYI, observation, "I noticed X") would dilute the convergence signal — every pass would produce some — without adding launch-readiness value. If a finding is below Low, it does not belong in the bugbash artifact at all. Specialists who want to share context outside findings can do so in the conversation, not the log.
