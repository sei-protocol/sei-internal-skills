# Example Output

This is an illustrative example of the bugbash findings log (`designs/<arc>/bugbash/<target>.md` in the DRI repo; in-repo `docs/bugbash/<target>.md` fallback) mid-run, with five findings logged. It uses the user-provided "Item 5: incomplete validation on SeiNode networkconfig with replicas" example, surrounded by representative Items 1–4 to show the shape of a real artifact.

The findings here are illustrative, not real findings against any production system. Some items reference an on-chain attestation flow and an interface registry purely as sample subject matter to show the finding shape — they do not describe sei-internal-skills, which is a skills/agents library, not an on-chain system.

---

# Bugbash: SeiNode controller

**Target path:** `pkg/controllers/seinode`
**Started:** 2026-04-29
**Last updated:** 2026-04-29
**Experts:** kubernetes-specialist, sei-network-specialist, security-specialist, platform-engineer, product-manager

## Summary

| Severity | Count | Launch-blocking |
|----------|-------|-----------------|
| Critical | 1     | yes             |
| High     | 2     | yes             |
| Medium   | 2     | tracked         |
| Low      | 0     | tracked         |

## Findings

## Item 1: Reconcile loop swallows Sei RPC connection errors

### Overview

#### Experts involved

- **Finder:** sei-network-specialist
- **Challenger:** kubernetes-specialist — verdict: confirm
- **Severity:** Critical

### Scenario

When the controller reconciles a SeiNode, it polls the node's gRPC endpoint to verify health. If the gRPC dial fails (network partition, node restarting, port not yet bound), the error is logged at debug level and reconciliation marks the node Healthy=Unknown but continues. Subsequent reconciles within the requeue interval see the cached Unknown state and do not re-poll.

### Impact / Risk / Priority

A node that has crashed and is unreachable can sit in Unknown state for the full requeue interval (up to 10 minutes) without any alert firing or any retry attempt. Operators have no visibility into the unreachable state because the only log line is at debug level. Critical because a "silent broken-window" on the core health invariant the controller exists to maintain — operators trust Healthy reporting and act on it.

### Issue

In `pkg/controllers/seinode/reconcile.go:184`, the gRPC dial result is checked but the error is logged with `klog.V(4)` and reconciliation proceeds with `Healthy: Unknown`. The requeue interval at line 312 is 10 minutes regardless of whether the prior status was Unknown vs. Healthy. The interface registry lists `seinode.health.unknown` as a transient state that requires re-polling on the next requeue, but the cache check at line 198 short-circuits the re-poll.

**Fix sketch:**

- Promote the dial error to `klog.V(2)` with a structured field for the node name.
- When status transitions to Unknown, set requeue interval to 30s (configurable) for a bounded number of fast retries before falling back to the slow interval.
- Clear the cached Unknown state on each requeue so re-poll always runs.

**Test coverage:**

Integration test that crashes a SeiNode mid-reconcile and verifies (a) the controller observes Unknown within 60s, (b) the controller re-polls at the fast interval, and (c) the operator-visible status updates within 90s. The existing test in `controllers/seinode/reconcile_test.go:TestHealthCheck` only covers the dial-success path.

**Metric:**

`seinode_health_unknown_duration_seconds` (histogram, buckets up to 600s). Alert when p95 exceeds 120s — that means nodes are sitting Unknown longer than the fast-retry path should allow, which signals the fast-retry didn't activate. Operationally critical because the failure mode is silent.

## Item 2: Job ownership leak when SeiNode is deleted mid-job

### Overview

#### Experts involved

- **Finder:** kubernetes-specialist
- **Challenger:** platform-engineer — verdict: downgrade from Critical
- **Severity:** High

### Scenario

A SeiNode resource is deleted while a child K8s Job (e.g., a snapshot operation) is still running. The controller's finalizer removes the SeiNode but does not delete or mark the Job for cleanup. The Job continues to run, attempting to write to a volume that may have been released, and consuming cluster resources for the full job timeout.

### Impact / Risk / Priority

The orphaned Job consumes CPU and memory until its activeDeadlineSeconds elapses (default 1 hour). On a multi-tenant cluster, this is observable resource waste. Recovery is automatic (Job eventually times out and the GC sweeps it) but slow and noisy. Downgraded from Critical to High because the failure recovers without manual intervention; the original Critical framing assumed the Job could write to a reused volume, but the volume's reclaim policy prevents reuse before the Job exits.

### Issue

`pkg/controllers/seinode/finalizer.go:67` removes the SeiNode object's finalizer once the on-chain release event is observed. It does not enumerate child Jobs (via owner reference or the `tide.sei.io/seinode` label) before doing so. The interface registry lists the Job-cleanup contract under SeiNode finalizer responsibilities.

**Fix sketch:**

- Before removing the finalizer, list Jobs with `app.kubernetes.io/instance=<seinode>` and either delete them with foreground propagation, or set `activeDeadlineSeconds=30` to force cleanup.
- Add a wait state in the finalizer that requeues until child Jobs are gone, with a max-wait of 5 minutes before forcing.

**Test coverage:**

E2E test that deletes a SeiNode while a long-running Job is in flight, then asserts the Job is cleaned up within the max-wait window. Existing finalizer tests only cover the no-Jobs-running path.

## Item 3: Missing rate limit on attestation submission

### Overview

#### Experts involved

- **Finder:** security-specialist
- **Challenger:** kubernetes-specialist — verdict: confirm
- **Severity:** High

### Scenario

The runtime submits attestations to the on-chain TideJobHook on every reconcile loop tick. There is no rate limiter or backoff on the submission path. If the controller enters a reconcile-loop fast cycle (e.g., due to Item 1's fast-retry behavior or a CRD spec change loop), it can submit attestations at up to 1 Hz per node.

### Impact / Risk / Priority

At cluster scale (100+ SeiNodes), this can saturate the EVM RPC endpoint with redundant calls and incur unnecessary gas spend. No current security bypass — the attestations are still valid — but the design assumes a much lower submission rate, and a bug elsewhere in the controller could weaponize this into a self-DoS or unintended cost spike.

### Issue

`runtimes/review/attestation.py:142` calls `submit_attestation` directly from the reconcile callback. No `RateLimiter`, no `min_interval` check against the previous submission for the same node.

**Fix sketch:**

- Introduce a per-node submission rate limit (default: 1 submission per 60s).
- Coalesce redundant submissions: if the attestation payload is identical to the last submitted one for the same node, skip.

**Test coverage:**

Unit test that fires 100 reconcile callbacks in 1 second for the same node and asserts only one submission goes through. Add to `runtimes/review/test_attestation.py`.

## Item 4: Reconciler logs include raw EIP-712 signatures

### Overview

#### Experts involved

- **Finder:** security-specialist
- **Challenger:** product-manager — verdict: downgrade from High
- **Severity:** Medium

### Scenario

When the controller processes a job submission, it logs the full request payload at info level for debugging. The payload includes the EIP-712 signature from the requester. Cluster log aggregation (Datadog, Loki, etc.) collects these.

### Impact / Risk / Priority

EIP-712 signatures are not secrets — they're verifiable on-chain — but logging them makes signature replay attacks easier if a log store is later compromised, and the signed payload may include addresses or domain separators that should not be aggregated to third-party log stores by policy. Downgraded from High to Medium because the signatures alone don't grant new authority (the on-chain contract enforces nonce / replay protection), but the logging hygiene gap is real.

### Issue

`pkg/controllers/seinode/jobs.go:201` does `klog.Infof("processing job: %+v", req)` which dumps the full struct. The signature is in `req.Signature`.

**Fix sketch:**

- Implement a `Redacted` wrapper for the signature field.
- Replace the `%+v` log with structured fields, omitting the signature.
- Audit other call sites for similar payload dumps.

**Test coverage:**

Add a log-content assertion to the existing job-submission unit test verifying the signature does not appear in any captured log output.

## Item 5: Incomplete validation on SeiNode networkconfig with replicas

### Overview

#### Experts involved

- **Finder:** kubernetes-specialist
- **Challenger:** sei-network-specialist — verdict: confirm
- **Severity:** High

### Scenario

A SeiNode created with `spec.replicas > 1` and a `spec.networkConfig` block specifying a single static peer endpoint passes validation and reconciles. The controller spins up multiple replica pods, each attempting to bind to the same peer endpoint. The first replica connects; subsequent replicas fail their CometBFT P2P handshake and crash-loop.

### Impact / Risk / Priority

Operators creating a multi-replica SeiNode with a static-peer network config see a partial-success state: one replica running, the others crash-looping. Status reporting calls the SeiNode Unhealthy but does not surface the actual cause. Operators waste time debugging the crash-loop before discovering the configuration mismatch. Recoverable (operator updates the spec) but a foreseeable footgun on a primary user surface.

### Issue

In `pkg/apis/seinode/v1/validation.go:88`, the validation webhook checks that `spec.replicas >= 1` and that `spec.networkConfig.peers` is non-empty when present, but does not check the *combination* of `replicas > 1` with a `networkConfig` that names static peers without per-replica scoping. The interface registry's SeiNode CRD spec at `sei-internal-skills/interface-registry.yaml#seinode-v1` documents that static peer configs are scoped per-replica only when `networkConfig.replicaScope: true`, but this field defaults to false and the webhook doesn't enforce the consequence.

**Fix sketch:**

- Add a webhook validation rule: if `spec.replicas > 1` AND `spec.networkConfig.peers` is set AND `spec.networkConfig.replicaScope != true`, reject with a clear error message naming the conflict.
- Update the CRD's `kubebuilder:validation` annotations to express the constraint where possible.
- Document the interaction in the SeiNode reference doc with a concrete example of the failing config and the fix.

**Test coverage:**

Webhook unit test in `pkg/apis/seinode/v1/validation_test.go` covering the four-cell matrix: (replicas == 1 vs. > 1) × (replicaScope true vs. false) with a static-peer networkConfig. Existing tests cover only `replicas == 1`.

---

*This artifact is mid-run; the convergence counter is currently 1 (last pass produced no new ≥ Medium findings, but the prior pass produced this Item 5). The Launch Verdict section will be appended once convergence_counter reaches 2.*
