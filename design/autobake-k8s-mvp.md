# Autobake K8s Lift-and-Shift — MVP

**Date:** 2026-04-17
**Status:** Draft
**Owner:** Platform Team
**Scope tier:** System (single feature, multiple components touched, no new CRDs)

---

## Purpose

Replace the EC2-based `sei-autobake` performance pipeline with a Kubernetes-native equivalent driven by a GitHub Actions workflow that creates an ephemeral `SeiNodeDeployment` (via the existing `sei-k8s-controller`) and runs `seiload` against it as a `Job`.

**This is a lift-and-shift.** Same trigger shape, same profile, same report, same Slack post. We are swapping the orchestration substrate (Terraform + Ansible + SSM-on-EC2 → `kubectl` + K8s resources) and the compute substrate (EC2 AMIs building from source → pre-built container images). Nothing else.

Regression detection, rolling baselines, dashboards, CRDs, PR status checks, and bisect tooling are **explicitly out of scope** for this MVP and tracked in the Deferred section. We will layer them on once the lift-and-shift is wired and running.

---

## Dependencies

### External Systems Consumed

| System | Interface | Notes |
|--------|-----------|-------|
| `sei-k8s-controller` | `SeiNodeDeployment` CRD (v1alpha1) | Must be running in the target cluster with genesis ceremony support enabled. |
| AWS EKS | Kubernetes API v1.28+ | Same cluster the Tide platform targets. |
| `ghcr.io/sei-protocol/sei-chain` | Container image tagged by commit SHA + `main-latest` | **New** — see prerequisite P1 below. |
| `ghcr.io/sei-protocol/sei-load` | Container image tagged by commit SHA + `main-latest` | **New** — see prerequisite P2 below. |
| AWS S3 | `s3://sei-autobake-results/` | Stores the JSON report per run. IRSA on the Job's ServiceAccount. |
| Slack | Incoming webhook to `#autobake-runs` (existing channel `C09D2P5GM7B`) | Same channel today's EC2 workflow posts to. Parity. |
| GitHub Actions OIDC | `sts.amazonaws.com` audience | Federates runner identity into a scoped in-cluster ServiceAccount. |

### Internal Components Consumed

| Component | Interface | Notes |
|-----------|-----------|-------|
| `sei-k8s-controller` planner (Init plan, genesis plan) | Watches `SeiNodeDeployment` → creates child `SeiNode`s, runs genesis ceremony, stamps `.status.phase=Ready` | No API changes required from this MVP. |
| `sei-k8s-controller` status conventions | `.status.phase`, `.status.conditions[RolloutInProgress]` with `observedGeneration` | Already present; MVP polls these. |

### Prerequisites (gate MVP delivery)

- **P1 — Publish a `sei-chain` container image.** New workflow in `sei-protocol/sei-chain`: on merge to main, build and push `ghcr.io/sei-protocol/sei-chain:${sha}` + `:main-latest`. Separate tag variant for `mock_balances` build (`sei-chain-mockbal:${sha}`). This is the first workflow in the repo to produce a container artifact; full image policy (signing, SBOMs, promotion gates) is **deferred** — the MVP only needs an unsigned, SHA-tagged dev image.
- **P2 — Publish a `sei-load` container image.** Same pattern in `sei-protocol/sei-load`. Profile JSON baked into the image (or mounted via ConfigMap — see §Interfaces below).
- **P3 — Controller change: stable in-cluster RPC Service per `SeiNodeDeployment`.** See §Critical Controller Gap below. **This is the one structural requirement that must land in the controller before MVP ships.**

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
- **Tertiary**: standard chain metrics flow to Prometheus via the existing controller `ServiceMonitor` path — no new alerting rules in MVP; the existing alert fix (separate PR — see §Adjacent Work) restores what's already there.

---

## Deferred (Phase 2)

Explicit list with one-line reasons:

- **Rolling baselines + regression detection.** Needs ~14 days of run history to be meaningful; wire after MVP ships and produces runs.
- **Grafana dashboard + Slack digest bot.** Value-add, not parity. Layer after MVP.
- **PR status checks on `sei-chain` PRs.** Pre-merge pipeline is a separate investment with its own budget/latency tradeoffs.
- **`SeiLoadTestRun` / `SeiChainValidation` / `PerfBaseline` CRDs.** Premature abstraction; extract only once we've run this for a month and know what the abstractions need to cover.
- **Bisect tooling beyond manual `workflow_dispatch` invocations.** Two dispatches + two dashboards are enough for MVP.
- **Full container-image policy RFC** (signing, SBOMs, promotion). MVP uses unsigned SHA-tagged images.
- **Heatseeker, snapshotter, state-syncer** K8s migrations. Independent work, independent teams.
- **Removing `setup-loadtest-testnet/`.** Deleted after MVP operates cleanly for 2 weeks.
- **Parallel-operation of EC2 and K8s autobake.** Explicitly rejected; see §Ship Order.

---

## Adjacent Work (ship this week, independent of MVP)

**Fix the three dead regression alerts on today's EC2 autobake.** File `sei-infra/monitoring/deploy/configs/alerts/sei-autobake/autobake_alerts.yaml` lines 48, 58, 68 reference `chain_id="sei-autobake-v2"` which does not match the metrics emitted (`chain_id="sei-autobake"`). Three one-character edits (`s/sei-autobake-v2/sei-autobake/`). Zero dependency on this MVP. Revives currently-silent alerts and validates the alerting path before we wire K8s metrics through it.

---

## Ship Order

| Step | Owner | Blocks on |
|------|-------|-----------|
| 0. Fix `sei-autobake-v2` label bug in existing alerts | sei-infra maintainer | — (ship this week) |
| 1. Publish `sei-chain` container image workflow (P1) | sei-chain maintainer | — |
| 2. Publish `sei-load` container image workflow (P2) | sei-load maintainer | — |
| 3. Ship cluster-internal RPC Service on `SeiNodeDeployment` (P3) | sei-k8s-controller maintainer | — |
| 4. MVP workflow in sei-infra (`.github/workflows/k8s_autobake.yml` + templates) | platform team | P1, P2, P3 |
| 5. Smoke-test via `workflow_dispatch`; inspect Slack output + cleanup | platform team | 4 |
| 6. Flip cron: disable `continuous_deploy.yml` + `run_autobake_loadtest.yaml`; enable `k8s_autobake.yml` | platform team | 5 |
| 7. Keep EC2 terraform deployable 2 weeks as rollback | platform team | 6 |
| 8. Delete EC2 autobake terraform + cron | platform team | 7 + 2 clean weeks |

Steps 0–3 can run in parallel. Step 4 is the convergence.

---

## Open Questions

1. **`mock_balances` variant.** Keep as a second container tag (P1) or fold into the genesis ceremony via `GenesisCeremonyConfig.Accounts`? MVP assumes second tag; cleaner path is a follow-up.
2. **Cadence post-cutover.** Today's EC2 is weekly. K8s makes 4-per-day or per-commit feasible. Pick cadence before step 5 — it affects baseline density when Phase 2 work starts.
3. **Namespace.** Single `autobake` ns with runs isolated by deployment name, or namespace-per-run? MVP recommends single; revisit if contention surfaces.

---

## One-way doors

Per constitution: call out deliberately.

- **`status.rpcService.name` / `.ports.*` field names on `SeiNodeDeployment`.** Consumers will reference these in templates. Freeze at the controller change (P3); renames later require migration.
- **Container image tagging convention** (`:${sha}` + `:main-latest`). Downstream consumers will grow to depend on these.
- **S3 report path shape** (`{chain}/{image-sha}/{run-id}/report.json`). Becomes the index for any future bisect tooling.

Everything else in this design (workflow script contents, thresholds, namespace names, Slack channel, recording rule names when we add them in Phase 2) is a two-way door.
