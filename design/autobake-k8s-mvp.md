# Autobake K8s Lift-and-Shift — MVP

**Date:** 2026-04-17
**Status:** Draft
**Owner:** Platform Team
**Scope tier:** System (single feature, multiple components touched, no new CRDs)

---

## Purpose

Replace the EC2-based `sei-autobake` performance pipeline with a Kubernetes-native equivalent driven by a GitHub Actions workflow that creates an ephemeral `SeiNodeDeployment` (via the existing `sei-k8s-controller`) and runs `seiload` against it as a `Job`.

**This is a lift-and-shift.** Same trigger shape, same profile, same report, same Slack post. We are swapping the orchestration substrate (Terraform + Ansible + SSM-on-EC2 → `kubectl` + K8s resources) and the compute substrate (EC2 AMIs building from source → container images consumed from whatever machinery already exists). Nothing else.

The workflow, templates, and scripts live **in this repo (Tide)**, alongside other platform designs. They reuse Tide's existing in-cluster identity / credential plumbing to manage resources reproducibly — every per-run resource is declarative (YAML templates rendered with `envsubst`), named by run ID, owned by the parent `SeiNodeDeployment` so teardown cascades deterministically, and created with `kubectl apply` rather than imperative shell state.

Regression detection, rolling baselines, dashboards, CRDs, PR status checks, bisect tooling, dedicated container image release pipelines, and the existing-alerts label fix are **explicitly out of scope** for MVP and listed in the Post-MVP Improvements section. We will layer them on once the lift-and-shift is wired and running.

---

## Dependencies

### External Systems Consumed

| System | Interface | Notes |
|--------|-----------|-------|
| `sei-k8s-controller` | `SeiNodeDeployment` CRD (v1alpha1) | Must be running in the target cluster with genesis ceremony support enabled. |
| AWS EKS | Kubernetes API v1.28+ | Same cluster the Tide platform targets. |
| seid container image | Whatever image source the platform team already maintains (dev-tagged is fine) | Reuse existing machinery. A dedicated release pipeline is post-MVP. |
| seiload container image | Same as above | Same. |
| AWS S3 | `s3://sei-autobake-results/` (or existing Tide results bucket) | Stores the JSON report per run. IRSA on the Job's ServiceAccount. |
| Slack | Incoming webhook to `#autobake-runs` (existing channel `C09D2P5GM7B`) | Same channel today's EC2 workflow posts to. Parity. |
| Cluster credentials | Tide platform's existing in-cluster identity / OIDC plumbing | Reuse whatever mechanism the platform already grants to CI/automation workloads. Do not invent new auth. |

### Internal Components Consumed

| Component | Interface | Notes |
|-----------|-----------|-------|
| `sei-k8s-controller` planner (Init plan, genesis plan) | Watches `SeiNodeDeployment` → creates child `SeiNode`s, runs genesis ceremony, stamps `.status.phase=Ready` | No API changes required from this MVP. |
| `sei-k8s-controller` status conventions | `.status.phase`, `.status.conditions[RolloutInProgress]` with `observedGeneration` | Already present; MVP polls these. |

### Prerequisite (the sole blocker on MVP delivery)

- **P1 — Controller change: stable in-cluster RPC Service per `SeiNodeDeployment`.** See §Critical Controller Gap below. This must land in the `sei-k8s-controller` before MVP ships. Everything else the MVP needs (container images, credentials, alerting) can be satisfied by existing machinery and improved incrementally after MVP is running.

### Explicit Exclusions

- Not replacing `snapshotter`, `state-syncer`, `webapp`, `heatseeker`, or `setup-loadtest-testnet/`. They remain on EC2.
- Not introducing any new CRDs. Run state lives in the GitHub Actions workflow + labels on the `SeiNodeDeployment`.
- Not generating `PrometheusRule` / `ServiceMonitor` resources from the workflow (existing chain-health metrics surface via the controller's existing `ServiceMonitor` reconciler if `Spec.Monitoring` is set; we set it; that's it).
- Not running both systems in parallel. Smoke-test via `workflow_dispatch`, then cut over.

---

## Critical Controller Gap — cluster-internal RPC Service

The single structural change the controller must ship to support MVP cleanly.

**Problem.** Today a `SeiNodeDeployment` exposes RPC endpoints only via:
1. **Per-node headless Services** at `{node-name}-0.{node-name}.{ns}.svc.cluster.local` — requires the workflow to pick a specific node ordinal and implement retry/failover if that node is unhealthy or being rolled.
2. **External Service + HTTPRoute** when `.spec.networking` is set — triggers external DNS resolution and blocks reconciliation on DNS propagation. Inappropriate for a purely in-cluster client.

Neither is acceptable for the `seiload` Job, which needs a single stable in-cluster hostname and will be consumed by an unbounded set of future workloads (any future validation Job, debug pod, or ad-hoc `kubectl exec` from an engineer). Building node-selection logic into every consumer's template is a structural tax we should pay once in the controller, not per-consumer.

**Change.** The `SeiNodeDeployment` controller provisions an additional **ClusterIP Service** per deployment (no ExternalName, no HTTPRoute, no external DNS) that:
- Selects all child `SeiNode` pods via a deployment-scoped label (`sei.io/nodedeployment=<name>`).
- Publishes the standard seid ports (RPC 26657, EVM HTTP 8545, EVM-WS 8546, REST 1317, gRPC 9090).
- Is named predictably: `{deployment-name}-rpc.{namespace}.svc.cluster.local`.
- Is owned by the `SeiNodeDeployment` (OwnerReference → cascade delete on teardown).
- Is reconciled regardless of `.spec.networking` (which remains the opt-in for external exposure).

**Why this is the right scope.** This is plumbing, not a product feature. It's a two-way door — the Service name is a label the controller owns; consumers resolve it from `.status.rpcService` on the deployment. Any future load-test, debug, or validation workflow depends on it; punting makes the consumer side structurally worse forever.

Interface contract (interfaces-first, per constitution):

```yaml
# SeiNodeDeployment.status (additive)
status:
  rpcService:
    name: autobake-abc123-rpc
    namespace: autobake
    ports:
      rpc: 26657
      evmHttp: 8545
      evmWs: 8546
      rest: 1317
      grpc: 9090
```

Consumer contract: any Job in the same cluster can dial `http://{status.rpcService.name}.{status.rpcService.namespace}.svc:{ports.evmHttp}` once the deployment's `.status.phase == Ready`.

---

## Interfaces

### MVP Workflow inputs

| Input | Source | Notes |
|-------|--------|-------|
| `cron` trigger | GitHub Actions `schedule` | Initial cadence: same as today's EC2 pipeline (configurable in the workflow; does not block MVP). |
| `workflow_dispatch` with `image_sha`, `duration_minutes` | Manual trigger | For ad-hoc runs and (future) bisect. |
| `concurrency: {group: k8s-autobake, cancel-in-progress: true}` | Workflow-level | Dedupes overlapping runs; one line of YAML. |
| Cluster credentials | GitHub Actions OIDC → AWS IAM role → `aws eks get-token` | No long-lived kubeconfig. |

### Rendered Kubernetes objects per run

| Object | Kind | Lifecycle |
|--------|------|-----------|
| `autobake-${run-id}` | `SeiNodeDeployment` (v1alpha1) | Created at step 3 of the workflow; deleted at teardown. |
| `autobake-${run-id}-rpc` | `Service` (ClusterIP) | **Controller-owned** per the §Critical Controller Gap change. |
| `autobake-${run-id}-{0..N}` | `SeiNode` (v1alpha1) | Controller-owned children. |
| `seiload-${run-id}` | `batch/v1 Job` | Created at step 5; deleted at teardown. `ttlSecondsAfterFinished: 3600` as backstop. |

### RBAC (narrow, namespace-scoped to `autobake`)

| Resource | Verbs |
|----------|-------|
| `seinodedeployments.sei.io` | get, list, watch, create, update, patch, delete |
| `seinodes.sei.io` | get, list, watch (read-only — controller owns) |
| `services`, `pods`, `pods/log`, `events`, `configmaps`, `secrets` | get, list, watch |
| `batch/jobs` | get, list, watch, create, delete |

No cluster-scoped permissions. No ability to touch anything outside the `autobake` namespace.

### Workflow steps (shape, not final bash)

1. Checkout `sei-infra`. Resolve `CHAIN_ID=autobake-${run-id}-${run-attempt}`, `IMAGE_SHA`.
2. OIDC → AWS → kubeconfig.
3. `envsubst < templates/seinodedeployment.yaml | kubectl apply -f -`
4. `kubectl wait --for=jsonpath='{.status.phase}'=Ready seinodedeployment/$CHAIN_ID --timeout=20m`
5. `kubectl wait --for=jsonpath='{.status.rpcService.name}' seinodedeployment/$CHAIN_ID --timeout=1m` (the Service name becomes the Job's RPC target)
6. `envsubst < templates/seiload-job.yaml | kubectl apply -f -`
7. `kubectl wait --for=condition=Complete job/seiload-$RUN_ID --timeout=40m`
8. `kubectl cp` the report out of the pod; upload to S3 `s3://sei-autobake-results/{chain}/{image-sha}/{run-id}/report.json`.
9. Post a Slack message to `#autobake-runs` with report attached (parity with today's output).
10. `if: always()` → `kubectl delete seinodedeployment/$CHAIN_ID` (cascades SeiNodes, Services, PVCs, Job via ownership).

### Outputs

- **Primary**: Slack message to `#autobake-runs` with the same text/attachment shape today's workflow posts. Downstream consumers (humans) see no diff.
- **Secondary**: S3 report at a stable path, GitHub Actions artifact (30-day retention).
- **Tertiary**: standard chain metrics flow to Prometheus via the existing controller `ServiceMonitor` path — no new alerting rules in MVP; the existing-alert label fix lands as a post-MVP improvement (see below).

---

## Ship Order

| Step | Owner | Blocks on |
|------|-------|-----------|
| 1. Ship cluster-internal RPC Service on `SeiNodeDeployment` (P1) | sei-k8s-controller maintainer | — |
| 2. Port the workflow + templates + scripts from sei-infra into this (Tide) repo; refactor for K8s resources | platform team | 1 |
| 3. Smoke-test via `workflow_dispatch`; inspect Slack output + cleanup | platform team | 2 |
| 4. Cut over: disable `continuous_deploy.yml` + `run_autobake_loadtest.yaml` in sei-infra; enable the Tide-hosted workflow | platform team | 3 |
| 5. Keep EC2 terraform deployable 2 weeks as rollback | platform team | 4 |
| 6. Delete EC2 autobake terraform + cron | platform team | 5 + 2 clean weeks |

Step 1 is the only blocker on the rest. Everything downstream is consumer-side configuration.

---

## Post-MVP Improvements (each independent; land after MVP is running)

Ordered by likely impact, not dependency.

- **Fix the three dead regression alerts on today's (and future) autobake monitoring.** `sei-infra/monitoring/deploy/configs/alerts/sei-autobake/autobake_alerts.yaml:48,58,68` reference `chain_id="sei-autobake-v2"` which does not match the metrics emitted (`chain_id="sei-autobake"`). Three one-character edits. Independent; revive currently-silent alerts.
- **Rolling baselines + regression detection.** Prometheus recording rules over 14d windows; delta-vs-baseline alerts with commit-compare URLs. Turns the lift-and-shifted output into a feedback loop that drives regressions down.
- **Grafana dashboard + Slack digest bot.** Single pane of glass for "last N runs, is main green."
- **Dedicated `sei-chain` container image release pipeline.** Replace whatever ad-hoc machinery the MVP consumes with SHA-tagged, policy-governed images (signing, SBOMs, promotion gates). Separate RFC.
- **Dedicated `sei-load` container image release pipeline.** Same.
- **PR status checks on `sei-chain` PRs.** Pre-merge validation; separate investment with its own budget/latency tradeoffs.
- **Bisect tooling beyond manual `workflow_dispatch` invocations.** Once we need to bisect regularly.
- **`SeiLoadTestRun` / `SeiChainValidation` / `PerfBaseline` CRD extraction.** Only after we've run the shell-scripted version for a month and know what the abstractions need to cover. Premature otherwise.
- **Heatseeker, snapshotter, state-syncer K8s migrations.** Independent work, independent teams.
- **Removing `setup-loadtest-testnet/`.** After MVP operates cleanly for 2 weeks.

---

## Open Questions

1. **Which existing image source does the workflow consume?** Confirm with the platform/protocol team which seid and seiload container images exist today (dev-tagged is fine) and where they live. This determines the registry URL + pull-secret plumbing in the templates. Blocker on step 2.
2. **`mock_balances` variant.** Today's EC2 autobake builds seid with `BUILD_TAGS=mock_balances` specifically for the `sei-autobake` chain. Does the existing image source already ship a mock-balances variant, or does MVP need to fund accounts via `GenesisCeremonyConfig.Accounts` instead? Resolving (1) probably answers this.
3. **Cadence post-cutover.** Today's EC2 is weekly. K8s makes 4-per-day or per-commit feasible. Pick cadence before step 3 — affects baseline density when post-MVP regression detection lands.
4. **Namespace.** Single `autobake` ns with runs isolated by deployment name, or namespace-per-run? MVP recommends single; revisit if contention surfaces.

---

## One-way doors

Per constitution: call out deliberately.

- **`status.rpcService.name` / `.ports.*` field names on `SeiNodeDeployment`.** Consumers will reference these in templates. Freeze at the controller change (P1); renames later require migration.
- **Container image tagging convention** (`:${sha}` + `:main-latest`). Downstream consumers will grow to depend on these.
- **S3 report path shape** (`{chain}/{image-sha}/{run-id}/report.json`). Becomes the index for any future bisect tooling.

Everything else in this design (workflow script contents, thresholds, namespace names, Slack channel, recording rule names when we add them in Phase 2) is a two-way door.
