# Cluster inspection recipes

Canonical invocations for extracting structured data from harbor-cluster resources. Use these directly instead of inferring `-o yaml` and parsing free-form output. Each recipe is the exact command + the field it returns + the failure mode if the resource isn't in the expected shape.

When in doubt about a field, check the live shape:

```sh
kubectl explain seinetwork.status              # tree view of the network's status fields
kubectl explain seinode.status                 # a single node's status fields
kubectl explain seinode.status.endpoint        # the per-node published endpoint (scalar leaf)
```

If a recipe below disagrees with `kubectl explain` on a live cluster, **`kubectl explain` wins** and this doc is stale — file an issue.

## Resource-level conventions

- **Two Kinds**: `seinetwork` (the genesis validator pool, one per chain) and `seinode` (a single node — each follower is a standalone SeiNode; the controller also generates the network's validators as SeiNodes).
- **Namespace**: every recipe assumes `-n eng-<alias>`. Drop the `-n` flag at your peril; `-A` is correct only for cross-namespace platform queries (rare for an engineer's session).
- **Network-identity labels** (the producer↔consumer contract):
  - `sei.io/seinetwork=<id>` — present on every SeiNode (validator or follower) belonging to the network.
  - `sei.io/role=validator` — controller-generated validators of a SeiNetwork.
  - `sei.io/role=node` — followers minted by `seictl node apply --network <id>`.
  - Selector pattern: `-l sei.io/seinetwork=<id>,sei.io/role=node` returns the network's follower SeiNodes only (excludes validators, which serve no EVM).

## Recipes

### 1. RPC endpoints for a chain — point load tools here, not at validators

Returns the fleet of per-follower EVM JSON-RPC URLs for the network. These are the URLs `sei-load`, ad-hoc curl tests, and Foundry should target. **Validators serve no EVM (`ModeValidator` disables EVM HTTP/WS) — never point load traffic at them.** Each follower is one SeiNode publishing its own `.status.endpoint` scalar; the fleet is assembled *across* CRs via `node list`, not from a single object. There is no controller-created aggregate — a round-robin VIP is an engineer-owned Flux Service if wanted (see the networking section in `ephemeral-chain-flow.md`).

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

`select(.)` drops a matched follower whose `.status.endpoint` is unset (not yet `Running`); the **selector** `sei.io/seinetwork=<id>,sei.io/role=node` does the real fleet-scoping at the apiserver (validators are `role=validator` and excluded). **Use the published URLs verbatim — never reconstruct them** (the controller owns the per-node headless DNS form, e.g. `http://<chain-id>-rpc-0.sei.svc:8545`).

If `.items` is empty, no follower nodes exist for the network yet — `seictl node apply <id>-rpc-0 --preset rpc --chain-id <id> --network <id>` first.

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

```sh
# The validator network: chain-id, phase, validator-pool readiness
seictl network list -n eng-<alias> \
  -o custom-columns='NAME:.metadata.name,PHASE:.status.phase,READY:.status.readyReplicas,DESIRED:.status.replicas'

# The network's follower SeiNodes (a node is single, so no READY/DESIRED columns)
seictl node list -n eng-<alias> -l sei.io/seinetwork=<chain-id> \
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

A SeiNode is a single node with its own headless Service, so its `.status.endpoint.evmJsonRpc` **is** the stable per-follower URL — there is no per-pod array to index. For WS subscription affinity or gRPC streaming where you want one fixed target, list the fleet (recipe #1) and pick a follower; its published URL stays put.

```sh
# Every follower's name + its stable EVM HTTP URL
seictl node list -n eng-<alias> -l sei.io/seinetwork=<chain-id>,sei.io/role=node -o json \
  | jq -r '.items[] | "\(.metadata.name)\t\(.status.endpoint.evmJsonRpc // "<pending>")"'
```

### 7. A network's nodes — validators vs followers

There is no parent fleet object to drop down from: validators are SeiNodes the SeiNetwork controller generates; followers are standalone SeiNodes you applied. Both carry `sei.io/seinetwork=<id>`; the role label distinguishes them.

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

### 8. Flux Kustomization Ready state — "is the engineer fully wired?"

The per-engineer Flux Kustomization lives at `<alias>` in the `eng-<alias>` namespace (not `flux-system`). Used during onboarding verification (post-merge of the platform-repo PR) and during incident triage when a workspace push isn't reconciling.

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

If `kubectl get kustomization <alias> -n eng-<alias>` returns `NotFound`, the onboarding PR hasn't merged or the per-engineer Flux wiring wasn't included. Don't try to create the Kustomization yourself — surface to the engineer + platform team.

## Bench observation recipes (named)

Three recipes used by the bench Procedure (single + comparative). Referenced by name from `SKILL.md` step 11 and from `references/sei-load-bench.md`. Each is the exact command, what it shows, and the failure mode.

### `bench:live-tail` — stream seiload output while the Job runs

```sh
kubectl logs -n eng-<alias> -l sei.io/bench-name=<RUN_ID> -c seiload -f
```

Returns: streaming stdout of the seiload container. Terminates when the Job pod terminates. Use during the active `<DURATION>` window when the engineer wants to watch generation rate, error rate, or RPC latency drift in real time.

If `kubectl logs` returns `No resources found`, the bench Job hasn't been scheduled yet — Flux may not have reconciled the merged PR, or the parent kustomization is broken. Cross-check with recipe #8 (Flux Kustomization Ready state).

### `bench:terminal-check` — has the Job hit a terminal state?

```sh
kubectl get job -n eng-<alias> seiload-<RUN_ID> \
  -o jsonpath='{range .status.conditions[*]}{.type}={.status}{"\n"}{end}' \
  | grep -E '^(Complete|Failed)=True$'
```

Returns: `Complete=True` on success, `Failed=True` on `activeDeadlineSeconds` or `backoffLimit` exhaustion, empty if still running. **The host-side `grep` is necessary because kubectl jsonpath filter expressions don't support `||`** — iterate `.status.conditions[*]` and filter on the host. Use after the expected `<DURATION>` window to confirm terminal state before fetching results.

### `bench:teardown` — remove a bench from the engineer's workspace

```sh
git rm -r engineers/<alias>/bench-<RUN_ID>/
# Then edit engineers/<alias>/kustomization.yaml to remove the `bench-<RUN_ID>` entry
# from `resources:` — Kustomize fails to render with a missing-resource entry.
git commit + push
```

After the PR merges, Flux prunes the Job + ConfigMap on next reconcile. PVCs / Pods cascade per k8s deletion propagation. The `<RUN_ID>` task dir is removed from the engineer's workspace tree.

## When a recipe doesn't match observed output

The SeiNetwork/SeiNode status surface is a public contract (per the type comments in `sei-protocol/sei-k8s-controller`'s `api/v1alpha1/`), but it does evolve. If a recipe's jsonpath returns nothing on a live CR that's clearly populated, in priority order:

1. `kubectl explain seinetwork.status.<field-path>` / `kubectl explain seinode.status.<field-path>` to confirm the field still exists with the assumed name.
2. `kubectl get seinetwork|seinode <name> -n eng-<alias> -o yaml` and grep for the field — sometimes the optionals collapse and the path needs a `?(@...)` filter.
3. Check `sei-protocol/sei-k8s-controller` `api/v1alpha1/seinetwork_types.go` / `seinode_types.go` for renames since this doc's last-verified date.
4. File an issue against this skill with the live YAML excerpt + the recipe that broke.

## Out of scope

- **Free-form troubleshooting flows** beyond field extraction — those live in `troubleshooting-seinode.md`.
- **Recipes for resources outside `eng-<alias>`** — cluster-wide platform queries are platform-team work, not engineer-facing.
- **Modifying resources** — these are pure read recipes. `seictl network|node apply` / `delete` is where mutation lives.
