# Cluster inspection recipes

Canonical invocations for extracting structured data from harbor-cluster resources. Use these directly instead of inferring `-o yaml` and parsing free-form output. Each recipe is the exact command + the field it returns + the failure mode if the resource is not in the expected shape.

When in doubt about a field, check the live shape:

```sh
kubectl explain seinetwork.status              # tree view of the network's status fields
kubectl explain seinode.status                 # a single node's status fields
kubectl explain seinode.status.endpoint        # the per-node published endpoint (object of URL leaves)
```

If a recipe below disagrees with `kubectl explain` on a live cluster, **`kubectl explain` wins** and this doc is stale — file an issue.

## Resource-level conventions

- **Two Kinds**: `seinetwork` (the genesis validator pool, one per chain) and `seinode` (a single node — each follower is a standalone SeiNode). The controller also generates the network's validators as SeiNodes.
- **Namespace**: every recipe assumes `-n eng-<alias>`. Drop the `-n` flag at your peril; `-A` is correct only for cross-namespace platform queries (rare for an engineer's session).
- **Network-identity labels** (the producer↔consumer contract):
  - `sei.io/seinetwork=<id>` — present on every SeiNode (validator or follower) belonging to the network.
  - `sei.io/role=validator` — controller-generated validators of a SeiNetwork.
  - `sei.io/role=node` — followers minted by `seictl node apply --network <id>`.
  - Selector pattern: `-l sei.io/seinetwork=<id>,sei.io/role=node` returns the network's follower SeiNodes only (excludes validators, which serve no EVM).

## Recipes

### 1. RPC endpoints for a chain — point load tools here, not at validators

Returns the fleet of per-follower EVM JSON-RPC URLs for the network. These are the URLs `sei-load`, ad-hoc curl tests, and Foundry should target. **Validators serve no EVM (`ModeValidator` disables EVM HTTP/WS) — never point load traffic at them.** Each follower is one SeiNode publishing its own `.status.endpoint` URLs; `node list` assembles the fleet *across* CRs, not from a single object. The network's controller-created aggregate (`<network>-internal` ClusterIP, published as `.status.internalService`) fronts only its validator children — no EVM there. The network's composed `.status.endpoints` surfaces EVM per-pod only, because stateful EVM protocols (filters, subscriptions, finalized-tag reads) do not load-balance behind kube-proxy.

The follower fleet therefore always comes from SeiNode CRs. A round-robin VIP over followers would be an engineer-owned Flux Service (see the networking section in `ephemeral-chain-flow.md`), and load tools should not want one.

```sh
# Fleet of per-follower EVM JSON-RPC URLs
seictl node list -n eng-<alias> -l sei.io/seinetwork=<chain-id>,sei.io/role=node -o json \
  | jq -r '[.items[].status.endpoint.evmJsonRpc | select(.)]'

# Fleet of per-follower Tendermint RPC URLs
seictl node list -n eng-<alias> -l sei.io/seinetwork=<chain-id>,sei.io/role=node -o json \
  | jq -r '[.items[].status.endpoint.tendermintRpc | select(.)]'

# Per-follower EVM JSON-RPC, one URL per line (each follower has its own stable URL)
seictl node list -n eng-<alias> -l sei.io/seinetwork=<chain-id>,sei.io/role=node -o json \
  | jq -r '.items[].status.endpoint.evmJsonRpc | select(.)'

# Per-follower EVM WebSocket, one URL per line (WS subscription affinity = pick one follower)
seictl node list -n eng-<alias> -l sei.io/seinetwork=<chain-id>,sei.io/role=node -o json \
  | jq -r '.items[].status.endpoint.evmWs | select(.)'
```

`select(.)` drops a matched follower with no `.status.endpoint` yet (not yet `Running`); the **selector** `sei.io/seinetwork=<id>,sei.io/role=node` does the real fleet-scoping at the apiserver (validators are `role=validator` and excluded). **Use the published URLs verbatim — never reconstruct them** (the controller owns the per-node headless DNS form, e.g. `http://<chain-id>-rpc-0.eng-<alias>.svc:8545`).

If `.items` is empty, no follower nodes exist for the network yet. Render one first, per `ephemeral-chain-flow.md` step 6. That recipe carries the create-only footprint and storage-performance flags.

If `.items[].status.endpoint` is unset, the followers exist but none reached `Running` yet. Use recipe #2 to confirm phase, then `seictl node watch <id>-rpc-<k> --until=Running` to block.

### 2. Phase + readiness for a network or a single node

```sh
# Network phase: Pending | Initializing | Ready | Paused | Degraded | Failed | Terminating
seictl network get <id> -n eng-<alias> -o jsonpath='{.status.phase}'

# Follower (node) phase: Pending | Initializing | Running | Failed | Terminating  (terminal is Running — no Ready)
seictl node get <id>-rpc-<k> -n eng-<alias> -o jsonpath='{.status.phase}'

# Validator-pool readiness math on the NETWORK: <ready>/<total>
seictl network get <id> -n eng-<alias> -o jsonpath='{.status.readyReplicas}/{.status.replicas}'
# A single follower SeiNode is 1/1 when Running.

# Reconcile freshness — observedGeneration matches metadata.generation when status reflects the latest spec (both trees)
seictl network get <id> -n eng-<alias> \
  -o jsonpath='{.metadata.generation}/{.status.observedGeneration}'
# Mismatch means the controller hasn't caught up to your last apply yet.
```

### 3. Failed task on a stuck Initializing/Failed network or node

When `.status.phase` is `Failed` or stuck `Initializing`, the actionable error is in `.status.plan.failedTaskDetail.error` (also lifted to stderr by `seictl network|node watch` on terminal Failed). The path is identical on both trees — use `network get` for genesis failures, `node get` for follower failures.

```sh
# The failed task name + error message (network)
seictl network get <id> -n eng-<alias> \
  -o jsonpath='{.status.plan.failedTaskDetail.taskName}: {.status.plan.failedTaskDetail.error}'

# Same, for a follower
seictl node get <id>-rpc-<k> -n eng-<alias> \
  -o jsonpath='{.status.plan.failedTaskDetail.taskName}: {.status.plan.failedTaskDetail.error}'

# Full task plan (every task's status; useful when failedTaskDetail is empty but phase is wrong)
seictl network get <id> -n eng-<alias> -o jsonpath='{.status.plan}' | jq
```

Common failed-task → root-cause map: `snapshot-restore` → S3 / Pod Identity, `configure-genesis` → genesis URL, `discover-peers` → label selector mismatch, `mark-ready` → seid health (check pod logs).

### 4. List a network + its follower SeiNodes in one shot

`seictl network|node list` emits `yaml | json | name | jsonpath` only — tabular `custom-columns` views go through `kubectl get`, which reads the same CRs.

```sh
# The validator network: chain-id, phase, validator-pool readiness
kubectl get seinetwork -n eng-<alias> \
  -o custom-columns='NAME:.metadata.name,PHASE:.status.phase,READY:.status.readyReplicas,DESIRED:.status.replicas'

# All SeiNodes in the network — the ROLE column distinguishes validators from followers.
# Inventory view (no READY/DESIRED — a SeiNode is a single node). For RPC-load endpoints
# use recipe #1, which filters to role=node and never includes validators.
kubectl get seinode -n eng-<alias> -l sei.io/seinetwork=<chain-id> \
  -o custom-columns='NAME:.metadata.name,ROLE:.metadata.labels.sei\.io/role,PHASE:.status.phase'
```

### 5. Container image actually running (vs. requested)

The spec image and the running pods can drift mid-rollout. The pod-side image is authoritative for "what's actually executing right now." `spec.image` is flat on both Kinds (no `spec.template`).

```sh
# Spec (what was requested by the latest apply) — a follower
seictl node get <id>-rpc-<k> -n eng-<alias> -o jsonpath='{.spec.image}'

# Pod-side (what the kubelet actually pulled and ran — by network label selector)
kubectl get pods -n eng-<alias> -l sei.io/seinetwork=<chain-id> \
  -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[?(@.name=="seid")].image}{"\n"}{end}'
```

A spec image set 30s ago but pods still on the old image is a normal mid-rollout state.

### 6. A follower's stable URL (no round-robin needed)

A SeiNode is a single node with its own headless Service, so its `.status.endpoint.evmJsonRpc` **is** the stable per-follower URL. No per-pod array exists to index. For WS subscription affinity or gRPC streaming where you want one fixed target, list the fleet (recipe #1) and pick a follower. Its published URL stays put.

```sh
# Every follower's name + its stable EVM HTTP URL
seictl node list -n eng-<alias> -l sei.io/seinetwork=<chain-id>,sei.io/role=node -o json \
  | jq -r '.items[] | "\(.metadata.name)\t\(.status.endpoint.evmJsonRpc // "<pending>")"'
```

### 7. A network's nodes — validators vs followers

No parent fleet object exists to drop down from: validators are SeiNodes the SeiNetwork controller generates; followers are standalone SeiNodes you applied. Both carry `sei.io/seinetwork=<id>`; the role label distinguishes them.

```sh
# A network's validator SeiNodes
kubectl get seinode -n eng-<alias> -l sei.io/seinetwork=<chain-id>,sei.io/role=validator

# Phase per node + age (whole network)
kubectl get seinode -n eng-<alias> -l sei.io/seinetwork=<chain-id> \
  -o custom-columns='NAME:.metadata.name,ROLE:.metadata.labels.sei\.io/role,PHASE:.status.phase,AGE:.metadata.creationTimestamp'

# A specific SeiNode's last condition (most recent first)
kubectl get seinode <name> -n eng-<alias> \
  -o jsonpath='{.status.conditions[-1:].type}: {.status.conditions[-1:].message}'
```

### 8. Flux `Kustomization` Ready state — "is the engineer fully wired?"

The per-engineer Flux Kustomization lives at `<alias>` in the `eng-<alias>` namespace (not `flux-system`). Used during onboarding verification (post-merge of the platform-repo PR) and during incident triage when a workspace push is not reconciling.

```sh
# Is the engineer's Flux Kustomization Ready?
kubectl get kustomization <alias> -n eng-<alias> \
  -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
# → True | False | (empty if NotFound — the onboarding PR hasn't merged or didn't ship the Kustomization)

# What's the last applied revision? Compare to the engineer's HEAD on the workspace branch.
kubectl get kustomization <alias> -n eng-<alias> \
  -o jsonpath='{.status.lastAppliedRevision}'

# Why is it not Ready? (message field is populated on Ready=False)
kubectl get kustomization <alias> -n eng-<alias> \
  -o jsonpath='{.status.conditions[?(@.type=="Ready")].message}'
```

If `kubectl get kustomization <alias> -n eng-<alias>` returns `NotFound`, the onboarding PR has not merged or the per-engineer Flux wiring was not included. Do not try to create the Kustomization yourself — surface to the engineer + platform team.

### 9. Validator placement — "is every validator alone on its EC2?"

Run after `network watch --until=Ready` on any chain that will carry a bench, and always on a chain rendered with `--node-isolation Dedicated`. `Ready` says the chain produces blocks; it says nothing about where the pods landed.

```sh
# One row per validator: name, placement, worker node
kubectl get seinetwork <chain-id> -n eng-<alias> \
  -o jsonpath='{range .status.nodes[*]}{.name}{"\t"}{.placement}{"\t"}{.workerNode}{"\n"}{end}'
# → <chain-id>-0    Scheduled    ip-10-0-12-34.ec2.internal
#   <chain-id>-1    Scheduled    ip-10-0-45-67.ec2.internal
#   <chain-id>-2    Pending

# Repeated worker nodes (any output = two validators share a box)
kubectl get seinetwork <chain-id> -n eng-<alias> \
  -o jsonpath='{range .status.nodes[*]}{.workerNode}{"\n"}{end}' | grep -v '^$' | sort | uniq -d

# Requested vs rolled isolation, per validator
kubectl get seinode -n eng-<alias> -l sei.io/seinetwork=<chain-id>,sei.io/role=validator \
  -o custom-columns='NAME:.metadata.name,WANT:.spec.scheduling.nodeIsolation,ROLLED:.status.currentNodeIsolation'

# A standalone follower has no placement row on any parent; read its pod
kubectl get pod -n eng-<alias> -l sei.io/node=<chain-id>-rpc-<k> \
  -o custom-columns='POD:.metadata.name,NODE:.spec.nodeName,PHASE:.status.phase'
```

Reading the table:

- **Every row `Scheduled`, every `workerNode` distinct** — clean; the measurement window may open.
- **A `Pending` row** — the pod is unbound. On a `Dedicated` pool that is a capacity shortfall: no worker node without a Sei pod exists. `kubectl describe pod <name>-0 -n eng-<alias>` shows the scheduler's `FailedScheduling` reason (`didn't match pod anti-affinity rules`). Surface it as a capacity ask to platform; do not delete the pod, do not re-apply, and do not flip the CR to `Shared` unless the engineer accepts a shared run.
- **A repeated `workerNode`** — two validators share bandwidth and CPU. Expected on `Shared`; impossible on `Dedicated` once both are `Scheduled` (if you see it, `ROLLED` is still `Shared` on one of them and a roll is pending or the controller is behind). Any bench number taken in this state measures the neighbour.
- **`WANT` ≠ `ROLLED`** — an isolation change is in flight; the controller replaces the pod on its next node-update plan. Wait for `ROLLED` to match before benching.

## Bench observation recipes (named)

Three recipes used by the bench Procedure (single + comparative). Referenced by name from `SKILL.md` step 11 and from `references/sei-load-bench.md`. Each is the exact command, what it shows, and the failure mode.

### `bench:live-tail` — stream seiload output while the Job runs

```sh
kubectl logs -n eng-<alias> -l sei.io/bench-name=<RUN_ID> -c seiload -f
```

Returns: streaming stdout of the seiload container. Terminates when the Job pod terminates. Use during the active `<DURATION>` window when the engineer wants to watch generation rate, error rate, or RPC latency drift in real time.

If `kubectl logs` returns `No resources found`, the bench Job has not landed on the cluster yet. Flux may not have reconciled the merged PR, or the parent kustomization has failed. Cross-check with recipe #8 (Flux `Kustomization` Ready state).

### `bench:terminal-check` — has the Job hit a terminal state?

```sh
kubectl get job -n eng-<alias> seiload-<RUN_ID> \
  -o jsonpath='{range .status.conditions[*]}{.type}={.status}{"\n"}{end}' \
  | grep -E '^(Complete|Failed)=True$'
```

Returns: `Complete=True` on success, `Failed=True` on `activeDeadlineSeconds` or `backoffLimit` exhaustion, empty if still running. **The host-side `grep` is necessary because kubectl jsonpath filter expressions do not support `||`** — iterate `.status.conditions[*]` and filter on the host. Use after the expected `<DURATION>` window to confirm terminal state before fetching results.

### `bench:teardown` — remove a bench from the engineer's workspace

```sh
git rm -r engineers/<alias>/bench-<RUN_ID>/
# Then edit engineers/<alias>/kustomization.yaml to remove the `bench-<RUN_ID>` entry
# from `resources:` — Kustomize fails to render with a missing-resource entry.
git commit + push
```

After the PR merges, Flux prunes the Job + ConfigMap on next reconcile. PVCs / Pods cascade per k8s deletion propagation. The teardown PR removes the `<RUN_ID>` task dir from the engineer's workspace tree.

## When a recipe does not match observed output

The SeiNetwork/SeiNode status surface is a public contract (per the type comments in `sei-protocol/sei-k8s-controller`'s `api/v1alpha1/`), but it does evolve. If a recipe's jsonpath returns nothing on a live CR that has that field populated, in priority order:

1. `kubectl explain seinetwork.status.<field-path>` / `kubectl explain seinode.status.<field-path>` to confirm the field still exists with the assumed name.
2. `kubectl get seinetwork|seinode <name> -n eng-<alias> -o yaml` and grep for the field — sometimes the optionals collapse and the path needs a `?(@...)` filter.
3. Check `sei-protocol/sei-k8s-controller` `api/v1alpha1/seinetwork_types.go` / `seinode_types.go` for renames since this doc's last-verified date.
4. File an issue against this skill with the live YAML excerpt + the recipe that broke.

## Out of scope

- **Free-form troubleshooting flows** beyond field extraction — those live in `troubleshooting-seinode.md`.
- **Recipes for resources outside `eng-<alias>`** — cluster-wide platform queries are platform-team work, not engineer-facing.
- **Modifying resources** — these are pure read recipes. `seictl network|node apply` / `delete` is where mutation lives.
