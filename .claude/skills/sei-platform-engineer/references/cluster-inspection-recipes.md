# Cluster inspection recipes

Canonical invocations for extracting structured data from harbor-cluster resources. Use these directly instead of inferring `-o yaml` and parsing free-form output. Each recipe is the exact command + the field it returns + the failure mode if the resource isn't in the expected shape.

Last verified: 2026-05-08 against `sei-protocol/sei-k8s-controller` `api/v1alpha1/seinodedeployment_types.go` (`SeiNodeDeploymentStatus`, `Endpoints`, `PerPodServiceStatus`, `InternalServiceStatus`, `TaskPlan`).

When in doubt about a field, check the live shape:

```sh
kubectl explain seinodedeployment.status        # tree view of every field
kubectl explain seinodedeployment.status.endpoints
```

If a recipe below disagrees with `kubectl explain` on a live cluster, **`kubectl explain` wins** and this doc is stale — file an issue.

## Resource-level conventions

- **Short name**: `snd` for `SeiNodeDeployment`. `kubectl get snd ...` works the same as `kubectl get seinodedeployment ...`.
- **Namespace**: every recipe assumes `-n eng-<alias>`. Drop the `-n` flag at your peril; `-A` is correct only for cross-namespace platform queries (rare for an engineer's session).
- **Chain-identity labels** (stamped by the `genesis-chain` and `rpc` presets):
  - `sei.io/chain=<chain-id>` — present on every SND in the chain.
  - `sei.io/role=validator` — set by `genesis-chain` preset.
  - `sei.io/role=node` — set by `rpc` preset.
  - Selector pattern: `-l sei.io/chain=<chain-id>,sei.io/role=node` returns the chain's RPC SND only.

## Recipes

### 1. RPC endpoints for a chain — point load tools here, not at validators

Returns the aggregate EVM JSON-RPC URL of the chain's RPC SND (the one applied with `--preset rpc`). This is the URL `sei-load`, ad-hoc curl tests, and Foundry should target. **Validators are reachable but reject foreign tx submission and throttle differently — never point load traffic at them.**

```sh
# Aggregate EVM JSON-RPC (single URL behind ClusterIP, kube-proxy round-robin)
kubectl get snd -n eng-<alias> -l sei.io/chain=<chain-id>,sei.io/role=node \
  -o jsonpath='{.items[0].status.endpoints.evmJsonRpc[0]}'
# → http://<rpc-snd-name>-internal.eng-<alias>.svc:8545

# Aggregate Tendermint RPC
kubectl get snd -n eng-<alias> -l sei.io/chain=<chain-id>,sei.io/role=node \
  -o jsonpath='{.items[0].status.endpoints.tendermintRpc[0]}'
# → http://<rpc-snd-name>-internal.eng-<alias>.svc:26657

# Per-pod EVM JSON-RPC (indices 1..N — useful for stable single-pod targeting)
kubectl get snd -n eng-<alias> -l sei.io/chain=<chain-id>,sei.io/role=node \
  -o jsonpath='{range .items[0].status.endpoints.evmJsonRpc[1:]}{@}{"\n"}{end}'

# Per-pod EVM WebSocket (no aggregate — kube-proxy round-robin breaks WS subscription affinity)
kubectl get snd -n eng-<alias> -l sei.io/chain=<chain-id>,sei.io/role=node \
  -o jsonpath='{range .items[0].status.endpoints.evmWs[*]}{@}{"\n"}{end}'
```

If `.items` is empty, no RPC SND has been applied for the chain yet — the engineer needs to `seictl nd apply <id>-rpc --preset rpc --chain-id <id>` first.

If `.items[0].status.endpoints` is empty, the RPC SND exists but hasn't reached `Ready` yet. Use recipe #2 to confirm phase, then `seictl nd watch <id>-rpc --until=Ready` to block.

### 2. Phase + readiness for a single SND

```sh
# Phase: Pending | Initializing | Running | Ready | Failed | Terminating
seictl nd get <name> -n eng-<alias> -o jsonpath='{.status.phase}'

# Readiness math: <ready>/<total>
seictl nd get <name> -n eng-<alias> -o jsonpath='{.status.readyReplicas}/{.status.replicas}'

# Reconcile freshness — observedGeneration matches metadata.generation when status reflects the latest spec
seictl nd get <name> -n eng-<alias> \
  -o jsonpath='{.metadata.generation}/{.status.observedGeneration}'
# Mismatch means the controller hasn't caught up to your last apply yet.
```

### 3. Failed task on a stuck Initializing/Failed SND

When `.status.phase` is `Failed` or stuck `Initializing`, the actionable error is in `.status.plan.failedTaskDetail.error` (also lifted to stderr by `seictl nd watch` on terminal Failed).

```sh
# The failed task name + error message
seictl nd get <name> -n eng-<alias> \
  -o jsonpath='{.status.plan.failedTaskDetail.taskName}: {.status.plan.failedTaskDetail.error}'

# Full task plan (every task's status; useful when failedTaskDetail is empty but phase is wrong)
seictl nd get <name> -n eng-<alias> -o jsonpath='{.status.plan}' | jq
```

Common failed-task → root-cause map: `snapshot-restore` → S3 / Pod Identity, `configure-genesis` → genesis URL, `discover-peers` → label selector mismatch, `mark-ready` → seid health (check pod logs).

### 4. List chains + their phase, role, and ready replicas in one shot

```sh
# All SNDs in the namespace with chain-id, role, phase, readiness
kubectl get snd -n eng-<alias> \
  -o custom-columns='NAME:.metadata.name,CHAIN:.metadata.labels.sei\.io/chain,ROLE:.metadata.labels.sei\.io/role,PHASE:.status.phase,READY:.status.readyReplicas,DESIRED:.status.replicas'

# Same shape but only one chain (validator + rpc together)
kubectl get snd -n eng-<alias> -l sei.io/chain=<chain-id> \
  -o custom-columns='NAME:.metadata.name,ROLE:.metadata.labels.sei\.io/role,PHASE:.status.phase,READY:.status.readyReplicas'
```

### 5. Container image actually running (vs. requested)

The spec image and the running pods can drift mid-rollout. The pod-side image is authoritative for "what's actually executing right now."

```sh
# Spec (what was requested by the latest apply)
seictl nd get <name> -n eng-<alias> -o jsonpath='{.spec.template.spec.image}'

# Pod-side (what the kubelet actually pulled and ran — by SND label selector)
kubectl get pods -n eng-<alias> -l sei.io/chain=<chain-id> \
  -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[?(@.name=="seid")].image}{"\n"}{end}'
```

A spec image set 30s ago but pods still on the old image is a normal mid-rollout state; check `.status.rollout` for progress.

### 6. Per-pod services for granular targeting (stable per-pod URLs)

Per-pod headless services give you a stable URL per replica — needed for WS, gRPC streaming, or any case where round-robin breaks correctness.

```sh
# Names + namespaces of every per-pod service for a given SND
seictl nd get <name> -n eng-<alias> \
  -o jsonpath='{range .status.perPodServices[*]}{.name}.{.namespace}.svc{"\n"}{end}'

# A specific pod's EVM HTTP URL (index 0 = pod 0, index 1 = pod 1, …)
seictl nd get <name> -n eng-<alias> \
  -o jsonpath='{.status.perPodServices[0].name}.{.status.perPodServices[0].namespace}.svc:{.status.perPodServices[0].ports.evmHttp}'
```

### 7. SeiNode (child) details — pod-level health behind a SND

A `SeiNodeDeployment` orchestrates `SeiNode` children. When a SND is unhealthy and the deployment-level recipes don't show why, drop down a level.

```sh
# Children of a deployment
kubectl get sn -n eng-<alias> -l sei.io/chain=<chain-id>

# Phase per child + age
kubectl get sn -n eng-<alias> -l sei.io/chain=<chain-id> \
  -o custom-columns='NAME:.metadata.name,PHASE:.status.phase,AGE:.metadata.creationTimestamp'

# A specific SeiNode's last condition (most recent first)
kubectl get sn <name> -n eng-<alias> \
  -o jsonpath='{.status.conditions[-1:].type}: {.status.conditions[-1:].message}'
```

### 8. Flux Kustomization Ready state — "is the engineer fully wired?"

Used during onboarding verification (post-merge of the platform-repo PR) and during incident triage when a workspace push isn't reconciling.

```sh
# Is the engineer's Flux Kustomization Ready?
kubectl get kustomization eng-<alias> -n flux-system \
  -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
# → True | False | (empty if NotFound — the onboarding PR hasn't merged or didn't ship the Kustomization)

# What's the last applied revision? Compare to the engineer's HEAD on the workspace branch.
kubectl get kustomization eng-<alias> -n flux-system \
  -o jsonpath='{.status.lastAppliedRevision}'

# Why is it not Ready? (message field is populated on Ready=False)
kubectl get kustomization eng-<alias> -n flux-system \
  -o jsonpath='{.status.conditions[?(@.type=="Ready")].message}'
```

If `kubectl get kustomization eng-<alias>` returns `NotFound`, the onboarding PR hasn't merged or the per-engineer Flux wiring wasn't included. Don't try to create the Kustomization yourself — surface to the engineer + platform team.

## When a recipe doesn't match observed output

The SND status surface is a public contract (per the type comments in `sei-protocol/sei-k8s-controller`'s `api/v1alpha1/`), but it does evolve. If a recipe's jsonpath returns nothing on a live SND that's clearly populated, in priority order:

1. `kubectl explain seinodedeployment.status.<field-path>` to confirm the field still exists with the assumed name.
2. `kubectl get snd <name> -n eng-<alias> -o yaml` and grep for the field — sometimes the optionals collapse and the path needs a `?(@...)` filter.
3. Check `sei-protocol/sei-k8s-controller` `api/v1alpha1/seinodedeployment_types.go` for renames since this doc's last-verified date.
4. File an issue against this skill with the live YAML excerpt + the recipe that broke.

## Out of scope

- **Free-form troubleshooting flows** beyond field extraction — those live in `troubleshooting-seinode.md`.
- **Recipes for resources outside `eng-<alias>`** — cluster-wide platform queries are platform-team work, not engineer-facing.
- **Modifying resources** — these are pure read recipes. `seictl nd apply` / `delete` / `patch` is where mutation lives.
