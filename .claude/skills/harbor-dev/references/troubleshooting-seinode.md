# Troubleshooting SeiNode (manual)

`seictl` doesn't ship a diagnose verb — this file documents the manual `kubectl`-driven flow.

## Decision tree by phase

Read `.status.phase` first: `kubectl get seinode <name> -o jsonpath='{.status.phase}'`.

### Phase: Pending (controller hasn't picked it up)

Common causes:

- Controller leader lease unhealthy → `kubectl get lease -n sei-system` and `kubectl get pods -n sei-system`
- Controller pod missing or crashlooping → `kubectl describe pod -n sei-system -l app.kubernetes.io/name=sei-k8s-controller`

If controller looks healthy but the SeiNode is still Pending after a minute, something deeper is wrong; capture controller logs and escalate.

### Phase: Initializing (task plan running)

Most common: failing PlannedTask. Inspect:

```sh
kubectl get seinode <name> -o jsonpath='{.status.plan}' | jq
```

Look for the task with `state=Failed` and read `.lastError`.

| Failed task | Usual cause | Where to look |
|---|---|---|
| `snapshot-restore` | S3 403 (Pod Identity wrong) | `kubectl logs <name>-0 -c seictl` (init container); `kubectl exec <name>-0 -- aws sts get-caller-identity` |
| `configure-genesis` (retried 180×) | Genesis URL missing or ConfigMap not mounted | `.status.plan[?name==configure-genesis].lastError` |
| `discover-peers` (returns 0) | EC2 tag query empty or peer label selector mismatch | `aws ec2 describe-instances --filters Name=tag:<key>,Values=<value>` from your laptop with the same filter |
| `mark-ready` | seid health check timing out | `kubectl logs <name>-0 -c seid` |

### Phase: Running (steady-state issues)

Symptoms after Ready:

- **Block production stalls** — `kubectl get seinode <name> -o jsonpath='{.status.conditions[?(@.type=="Ready")].lastTransitionTime}'`, then `kubectl logs <name>-0 -c seid` for consensus errors.
- **HTTPRoute hostname returns 503** — `kubectl get httproute -n <ns>`, verify `parentRefs` points at the shared Gateway, hostname matches `*.harbor.platform.sei.io`. Run `istioctl analyze -n <ns>`.
- **Pod restart loops** — `kubectl describe pod <name>-0 -n <ns>`. Common: OOMKill, image pull error, init container failure.

### Phase: Failed (terminal)

Once `.status.phase == Failed`, the controller stops reconciling — Failed is terminal, including across controller image upgrades. Recovery is delete-and-recreate so the parent SND rebuilds the SeiNode with the current pod template:

```sh
kubectl get seinode <name> -o jsonpath='{.status.conditions[?(@.type=="Ready")].message}'
# read the message; if structural, recreate:
kubectl delete seinode <name> -n eng-<alias>
```

The SND owner watcher recreates the SeiNode within seconds.

**PVC behavior** — verify before deleting on stateful nodes:
- For **imported** PVCs (`spec.import` set on the SeiNode): the PVC is preserved; the recreated SeiNode reuses existing data.
- For **controller-managed** PVCs (no `spec.import`): the controller's `handleNodeDeletion` path deletes the PVC during teardown. Delete-and-recreate **wipes data**. Safe for ephemeral chains being recreated from genesis; not safe for archive nodes or any chain with state worth preserving.

## SND plan stuck — "plan in progress, skipping SeiNode mutations"

Symptom: child SeiNodes are missing or in an unexpected state, controller logs show `plan in progress, skipping SeiNode mutations` for the SND. Typically caused by an SND-level plan that can't advance (e.g., `assemble-and-upload-genesis` can't find an assembler because child SeiNodes were deleted mid-plan).

Recovery — delete the SND so Flux re-applies it from the workspace repo on next reconcile:

```sh
kubectl delete snd <name> -n eng-<alias>
```

Flux reconciles in ~60s and the controller rebuilds the plan from scratch.

**Only safe with `spec.deletionPolicy: Delete`** (the default). If the SND has `deletionPolicy: Retain`, deleting orphans the child SeiNodes and their networking — the Flux re-apply creates a fresh SND that may collide with the orphaned resources. Check first with `kubectl get snd <name> -o jsonpath='{.spec.deletionPolicy}'`.

## apply-statefulset fails: rollingUpdate not allowed for OnDelete

Symptom: controller logs show

```
applying statefulset: StatefulSet.apps "<name>" is invalid:
  spec.updateStrategy.rollingUpdate: Invalid value: {"Partition":0,"MaxUnavailable":null}:
  only allowed for updateStrategy 'RollingUpdate'
```

Cause: a legacy StatefulSet has a stale `rollingUpdate` field from an earlier controller version that used `type: RollingUpdate`. The current controller sets `type: OnDelete` but doesn't claim ownership of `rollingUpdate`, so SSA leaves the stale field and the merged result is invalid.

Recovery — patch the StatefulSet to drop the stale field:

```sh
kubectl patch sts -n eng-<alias> <name> --type=merge \
  -p='{"spec":{"updateStrategy":{"type":"OnDelete","rollingUpdate":null}}}'
```

Metadata-only patch; pods are not restarted. The controller's next reconcile applies the new template successfully, then `replace-pod` rolls pods normally.

## Cross-cutting issues

### PVC stuck after delete

`sei.io/seinode-finalizer` blocks SeiNode deletion until the controller releases the PVC. If the controller is unhealthy or EBS CSI flaked, the SeiNode sits `Terminating` forever.

Inspection: `kubectl get seinode <name> -o jsonpath='{.metadata.finalizers}'` and controller logs (`kubectl logs -n sei-system -l app.kubernetes.io/name=sei-k8s-controller --tail=100`).

Manual override (only after confirming PVC orphan is acceptable):

```sh
kubectl patch seinode <name> -p '{"metadata":{"finalizers":[]}}' --type=merge
```

### HTTPRoute hostname unreachable

1. `kubectl get httproute <name> -n <ns> -o yaml` — verify `parentRefs` points at the shared Gateway.
2. Hostname must match `*.harbor.platform.sei.io` (the Gateway's listener pattern).
3. Check `AuthorizationPolicy` isn't denying: `kubectl get authorizationpolicy -n <ns>`.
4. Run `istioctl analyze -n <ns>` for cross-cutting Istio issues.

### Pod can't reach S3 / 0 peers discovered

Pod Identity check from inside the pod:

```sh
kubectl exec <name>-0 -c seid -- aws sts get-caller-identity
```

If this fails, the Pod Identity association is missing or wrong. Check via AWS:

```sh
aws eks list-pod-identity-associations --cluster-name harbor --namespace eng-<alias> --profile <chosen>
```

For peer discovery: `kubectl exec <name>-0 -c seid -- aws ec2 describe-instances --filters Name=tag:<key>,Values=<value> --region eu-central-1`. If empty, the tag query is wrong (verify `spec.peers.ec2Tags` in the SeiNode spec) or no instances are tagged.

If a `CiliumNetworkPolicy` is active in the namespace, check `kubectl get ciliumnetworkpolicy -n eng-<alias>` for egress denies.

## Diagnosing wedged nodes (app vs blockstore height)

A common wedge: the blockstore advances but the app falls behind. Compare the two — `lag = blockstore - app`. `lag = 0` is healthy; `lag = 1` sustained means the app-side commit handler is hung; `lag > 1` and growing means the app is structurally stuck.

### With HTTPRoute (Istio Gateway hostname exposed)

When the SND has `spec.networking.httproute` set, `.status.networking.routes[]` carries a public hostname per protocol. Query directly from your laptop — no `kubectl exec` needed.

```sh
hostname=$(kubectl get snd <name> -n eng-<alias> \
  -o jsonpath='{.status.networking.routes[?(@.protocol=="rpc")].hostname}')

app=$(curl -s https://$hostname/abci_info | jq -r .result.response.last_block_height)
store=$(curl -s https://$hostname/status   | jq -r .result.sync_info.latest_block_height)
echo "app=$app blockstore=$store lag=$((store - app))"
```

The aggregate hostname round-robins across replicas via the Istio Gateway (Envoy LB), so successive curls may hit different pods. For per-replica view (one pod wedged, others fine), use the exec recipe below.

### Without HTTPRoute (in-cluster only)

If `.status.networking.routes` is empty or has no `rpc` protocol entry, the chain isn't externally reachable — go through the pod's loopback via `kubectl exec`.

```sh
# Per-pod
kubectl exec -n eng-<alias> <pod> -c seid -- sh -c '
  app=$(curl -s localhost:26657/abci_info | jq -r .result.response.last_block_height)
  store=$(curl -s localhost:26657/status   | jq -r .result.sync_info.latest_block_height)
  echo "app=$app blockstore=$store lag=$((store - app))"
'

# Fleet view — every pod on a chain
for pod in $(kubectl get pods -n eng-<alias> \
    -l sei.io/chain=<chain-id> -o jsonpath='{.items[*].metadata.name}'); do
  echo "=== $pod ==="
  kubectl exec -n eng-<alias> $pod -c seid -- sh -c '
    app=$(curl -s localhost:26657/abci_info | jq -r .result.response.last_block_height)
    store=$(curl -s localhost:26657/status   | jq -r .result.sync_info.latest_block_height)
    echo "app=$app blockstore=$store lag=$((store - app))"
  '
done
```

If validators and RPC fullnodes share a chain-id, filter further: `-l sei.io/chain=<chain-id>,sei.io/role=validator` or `,sei.io/role=node`.

### Reading the result

| Pattern | What it means |
|---|---|
| Both heights equal, advancing | Healthy |
| Both heights equal, frozen | Consensus halted — check peer connectivity, validator quorum |
| `lag = 1` sustained across multiple polls, blockstore advancing | App commit handler hung — `kubectl logs <pod> -c seid` for app-side panic/deadlock. A single-shot `lag = 1` is normal sampling-race noise at 200ms blocks; poll 3–5× to confirm. |
| `lag > 1` and growing, outside of restart/state-sync | App falling behind structurally; usually won't catch up without intervention |
| `lag` shrinks over time | Catch-up after restart or state-sync; healthy |

## Profiling (pprof)

Dev SNDs applied via the seictl `genesis-chain` and `rpc` presets carry `network.rpc.pprof_listen_address: "0.0.0.0:6060"` in `spec.template.spec.overrides` (see sei-protocol/seictl#194). seid exposes Go pprof at port 6060 inside the pod.

Access from the engineer's laptop — port-forward tunnels through the API server; no LB / HTTPRoute / external network involved:

```sh
kubectl port-forward -n eng-<alias> <pod-name> 6060:6060
```

Then in a separate shell:

```sh
# CPU profile, 30s capture window
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Heap snapshot
go tool pprof http://localhost:6060/debug/pprof/heap

# Goroutine snapshot (cheap, instant — good first look at a wedge)
curl -s 'http://localhost:6060/debug/pprof/goroutine?debug=1' | less

# Full index
curl http://localhost:6060/debug/pprof/
```

Cost: idle pprof is essentially free (port listener + a few KB metadata). CPU profiling adds ~5% during the explicit capture window only; heap and goroutine snapshots are cheap.

If the override didn't take (older seictl version, hand-rolled SND), confirm and recover:

```sh
# Did the override land in config.toml?
kubectl exec -n eng-<alias> <pod> -c seid -- grep pprof_listen_address /.sei/config/config.toml

# Apply via --set on the SND (works regardless of preset)
seictl nd apply <id> --preset rpc --chain-id <id> --image <ref> \
  --set spec.template.spec.overrides."network.rpc.pprof_listen_address"="0.0.0.0:6060" \
  -n eng-<alias>
```

**Production caveat**: seictl ships one set of presets (`genesis-chain`, `rpc`) used in both dev and prod; there is no separate prod preset. When promoting a chain to prod, strip the pprof override explicitly: `--set spec.template.spec.overrides."network.rpc.pprof_listen_address"=""`. Pprof must never be reachable in prod — it exposes profile dumps and memory state to anyone with HTTP access to port 6060.

## Preserving the data dir for debugging

When a node is in a bad state and you want a point-in-time copy of its data dir for offline inspection — without doubling storage or waiting minutes-to-hours for `cp -r` — use `cp -al` **inside the PVC**:

```sh
# Hardlink the data dir at the current point-in-time.
# Near-instant, near-zero extra space.
kubectl exec -n eng-<alias> <pod> -c seid -- \
  cp -al /.sei/data /.sei/keep-$(date -u +%Y%m%dT%H%M%SZ)
```

**Do NOT hardlink to `/tmp`.** `/tmp` is a separate filesystem in containers (tmpfs or a separate emptyDir mount). Hardlinks cannot span filesystems, so `cp -al /.sei/data /tmp/keep-...` fails with `EXDEV` ("Invalid cross-device link"). Stay within the PVC mount.

### What the hardlink trick actually preserves

Hardlink ≠ symlink. A hardlink is a second directory entry pointing at the same inode; the data lives at the inode, not at either name. When seid's compaction later unlinks the original SST file, the inode survives because the keep dir still references it.

| File class | Behavior under hardlink trick |
|---|---|
| SeiDB / blockstore / `evidence.db` SST files | **Frozen.** Compaction unlinks `/.sei/data/...` but the inode survives via `/.sei/keep-.../...` |
| Tendermint WAL (`data/cs.wal/wal`) + app-level WALs | **Not frozen.** seid writes to the same inode the keep dir references — both names see the new bytes |
| Pebble `CURRENT` / `MANIFEST-*` files | **Frozen.** Rotated via atomic rename → new inode; old generation preserved in keep dir |
| `data/snapshots/*.tar.gz` | **Frozen.** Snapshot tarballs are immutable post-write; safe to hardlink |
| `addrbook.json`, `priv_validator_state.json` | **Frozen.** Updated via atomic rename; old version preserved |

For most "what did the state look like at height H" debugging, the SST files and validator state are what matters.

### Cleanup

```sh
kubectl exec -n eng-<alias> <pod> -c seid -- rm -rf /.sei/keep-<timestamp>
```

PVC space won't fully release until the original files are also unlinked (compaction takes care of this naturally as seid runs).

### vs. `deletionPolicy: retain`

`SeiNodeDeployment.spec.deletionPolicy: retain` preserves the PVC across SND deletion — for when you're tearing down the SND but want the disk to survive for forensics. The hardlink trick above is for **live debugging** while the node continues running. They're complementary, not redundant.
