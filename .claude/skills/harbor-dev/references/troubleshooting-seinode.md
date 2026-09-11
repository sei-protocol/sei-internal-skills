# Troubleshooting SeiNode (manual)

`seictl` does not ship a diagnose verb — this file documents the manual `kubectl`-driven flow.

## Decision tree by phase

Read `.status.phase` first: `kubectl get seinode <name> -o jsonpath='{.status.phase}'`.

### Phase: Pending (controller has not picked it up)

Common causes:

- **State-sync gate blocking (no pod and no StatefulSet ever appear)** — a snapshot-bootstrap node (`spec.fullNode.snapshot` set) whose witnesses the controller cannot resolve. Check first:

  ```sh
  kubectl get seinode <name> -o jsonpath='{range .status.conditions[?(@.type=="StateSyncReady")]}{.reason}: {.message}{end}'
  ```

  `NoSyncersConfigured` means no `spec.fullNode.snapshot.rpcServers` and no registry entry for the chain (eng chains never have one). The controller deliberately holds StatefulSet creation until the gate opens — the fix is in the condition message: declare ≥2 `rpcServers` or drop `stateSync` and genesis-replay. See `state-sync-bootstrap.md`. (On pre-`254375d` controllers this same cause presented as a pod stuck Pending on `persistentvolumeclaim not found` — if you see that shape, check this condition before chasing storage.)
- Controller leader lease unhealthy → `kubectl get lease -n sei-k8s-controller-system` and `kubectl get pods -n sei-k8s-controller-system`
- Controller pod missing or crashlooping → `kubectl describe pod -n sei-k8s-controller-system -l app.kubernetes.io/name=sei-k8s-controller`

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
| `configure-state-sync` ("no reachable RPC witness") | `rpcServers` endpoints wrong/unreachable, or wrong-chain (shape passes admission; chain membership is never checked — PLT-793) | `kubectl logs <name>-0 -c seictl`; re-read witness endpoints verbatim from the members' status; see `state-sync-bootstrap.md` failure table |

### Phase: Running (steady-state issues)

Symptoms after Ready:

- **Block production stalls** — `kubectl get seinode <name> -o jsonpath='{.status.conditions[?(@.type=="Ready")].lastTransitionTime}'`, then `kubectl logs <name>-0 -c seid` for consensus errors.
- **HTTPRoute hostname returns 503** — `kubectl get httproute -n <ns>`, verify `parentRefs` points at the shared Gateway, hostname matches `*.harbor.platform.sei.io`. Run `istioctl analyze -n <ns>`.
- **Pod restart loops** — `kubectl describe pod <name>-0 -n <ns>`. Common: OOMKill, image pull error, init container failure.

### Phase: Failed (terminal)

Once `.status.phase == Failed`, the controller stops reconciling — Failed is terminal, including across controller image upgrades. Recovery is delete-and-recreate:

- **A standalone follower SeiNode** (`role=node`, applied via `seictl node apply`): delete it and re-apply the manifest. Flux re-applies it from the workspace repo on next reconcile, or escape-hatch re-apply directly.
- **A validator SeiNode** the SeiNetwork generated (`role=validator`): the SeiNetwork controller recreates it within seconds after deletion — do not hand-roll a replacement.

```sh
kubectl get seinode <name> -o jsonpath='{.status.conditions[?(@.type=="Ready")].message}'
# read the message; if structural, recreate:
kubectl delete seinode <name> -n eng-<alias>
```

**PVC behavior** — verify before deleting on stateful nodes:
- For **imported** PVCs (`spec.dataVolume.import` set on the SeiNode): the controller preserves the PVC, and the recreated SeiNode reuses existing data.
- For **controller-managed** PVCs (no `spec.dataVolume.import`): the controller's `handleNodeDeletion` path deletes the PVC during teardown. Delete-and-recreate **wipes data**. Safe for an ephemeral chain you recreate from genesis; not safe for archive nodes or any chain with state worth keeping.

## SeiNetwork genesis plan stuck

Symptom: the SeiNetwork sits in `Initializing`, and its validator SeiNodes are missing or in an unexpected state. Controller logs show a genesis-plan task (e.g. `assemble-and-upload-genesis`) that cannot advance. The typical reason: someone deleted validator SeiNodes mid-plan, and the assembler cannot find a quorum.

Inspect the network's plan directly:

```sh
seictl network get <id> -n eng-<alias> -o jsonpath='{.status.plan}' | jq
```

Recovery — `spec.genesis` is immutable, so the clean path is `delete` + re-create the SeiNetwork: `git rm` the manifest and let Flux re-apply a fresh one. Note its `spec.deletionPolicy` first (defaults `Delete`; a SeiNetwork admitted before that default carries `Retain`). With `Retain`, the delete orphans the generated validator SeiNodes — clean them up before they collide with the rebuilt chain. **Re-create with a *new* chain-id** (or purge the chain-id's S3 genesis artifacts first). Reuse leaves stale identities that wedge the rebuilt chain at height 0 — see *Chain wedged at height 0 after delete-and-recreate*.

```sh
kubectl get seinetwork <id> -o jsonpath='{.spec.deletionPolicy}' -n eng-<alias>
```

## apply-statefulset fails: rollingUpdate not allowed for OnDelete

Symptom: controller logs show

```
applying statefulset: StatefulSet.apps "<name>" is invalid:
  spec.updateStrategy.rollingUpdate: Invalid value: {"Partition":0,"MaxUnavailable":null}:
  only allowed for updateStrategy 'RollingUpdate'
```

Cause: a legacy StatefulSet has a stale `rollingUpdate` field from an earlier controller version that used `type: RollingUpdate`. The current controller sets `type: OnDelete` but does not claim ownership of `rollingUpdate`, so SSA leaves the stale field and the merged result is invalid.

Recovery — patch the StatefulSet to drop the stale field:

```sh
kubectl patch sts -n eng-<alias> <name> --type=merge \
  -p='{"spec":{"updateStrategy":{"type":"OnDelete","rollingUpdate":null}}}'
```

Metadata-only patch; pods are not restarted. The controller's next reconcile applies the new template successfully, then `replace-pod` rolls pods normally.

## Cross-cutting issues

### PVC stuck after delete

`sei.io/seinode-finalizer` blocks SeiNode deletion until the controller releases the PVC. If the controller is unhealthy or EBS CSI flaked, the SeiNode sits `Terminating` forever.

Inspection: `kubectl get seinode <name> -o jsonpath='{.metadata.finalizers}'` and controller logs (`kubectl logs -n sei-k8s-controller-system -l app.kubernetes.io/name=sei-k8s-controller --tail=100`).

Manual override (only after confirming PVC orphan is acceptable):

```sh
kubectl patch seinode <name> -p '{"metadata":{"finalizers":[]}}' --type=merge
```

### HTTPRoute hostname unreachable

1. `kubectl get httproute <name> -n <ns> -o yaml` — verify `parentRefs` points at the shared Gateway.
2. Hostname must match `*.harbor.platform.sei.io` (the Gateway's listener pattern).
3. Check `AuthorizationPolicy` is not denying: `kubectl get authorizationpolicy -n <ns>`.
4. Run `istioctl analyze -n <ns>` for cross-cutting Istio issues.

### Pod cannot reach S3 / 0 peers discovered

Pod Identity check from inside the pod:

```sh
kubectl exec <name>-0 -c seid -- aws sts get-caller-identity
```

If this fails, the Pod Identity association is missing or wrong. Check via AWS:

```sh
aws eks list-pod-identity-associations --cluster-name harbor --namespace eng-<alias> --profile <chosen>
```

For peer discovery: `kubectl exec <name>-0 -c seid -- aws ec2 describe-instances --filters Name=tag:<key>,Values=<value> --region eu-central-1`. If empty, the tag query is wrong (verify `spec.peers.ec2Tags` in the SeiNode spec) or no instances carry the tag.

If a `CiliumNetworkPolicy` is active in the namespace, check `kubectl get ciliumnetworkpolicy -n eng-<alias>` for egress denies.

## Diagnosing wedged nodes (app vs blockstore height)

A common wedge: the blockstore advances but the app falls behind. Compare the two — `lag = blockstore - app`. `lag = 0` is healthy; `lag = 1` sustained means the app-side commit handler hangs; `lag > 1` and growing means the app is structurally stuck.

### With an engineer-rendered HTTPRoute (Istio Gateway hostname exposed)

The controller exposes no HTTPRoute. If the chain is externally reachable, the engineer rendered a Gateway-API `HTTPRoute` into their Flux dir (see the networking section in `ephemeral-chain-flow.md`). Read its hostname and query from your laptop — no `kubectl exec` needed.

```sh
hostname=$(kubectl get httproute <route-name> -n eng-<alias> \
  -o jsonpath='{.spec.hostnames[0]}')

app=$(curl -s https://$hostname/abci_info | jq -r .result.response.last_block_height)
store=$(curl -s https://$hostname/status   | jq -r .result.sync_info.latest_block_height)
echo "app=$app blockstore=$store lag=$((store - app))"
```

If the engineer also rendered an aggregate ClusterIP `Service` selecting `sei.io/seinetwork=<id>,sei.io/role=node`, that hostname round-robins across followers, so successive curls may hit different nodes. For a fixed node, use that follower's published `.status.endpoint` (one node = one stable URL) or the exec recipe below.

### Without an HTTPRoute (in-cluster only — the default)

With no engineer-rendered route, the chain is in-cluster only. Query a follower's published `.status.endpoint` from an in-namespace pod, or go through the pod's loopback via `kubectl exec`.

```sh
# Per-pod
kubectl exec -n eng-<alias> <pod> -c seid -- sh -c '
  app=$(curl -s localhost:26657/abci_info | jq -r .result.response.last_block_height)
  store=$(curl -s localhost:26657/status   | jq -r .result.sync_info.latest_block_height)
  echo "app=$app blockstore=$store lag=$((store - app))"
'

# Fleet view — every pod on a network
for pod in $(kubectl get pods -n eng-<alias> \
    -l sei.io/seinetwork=<chain-id> -o jsonpath='{.items[*].metadata.name}'); do
  echo "=== $pod ==="
  kubectl exec -n eng-<alias> $pod -c seid -- sh -c '
    app=$(curl -s localhost:26657/abci_info | jq -r .result.response.last_block_height)
    store=$(curl -s localhost:26657/status   | jq -r .result.sync_info.latest_block_height)
    echo "app=$app blockstore=$store lag=$((store - app))"
  '
done
```

To split validators from followers, filter further: `-l sei.io/seinetwork=<chain-id>,sei.io/role=validator` or `,sei.io/role=node`.

### Reading the result

| Pattern | What it means |
|---|---|
| Both heights equal, advancing | Healthy |
| Both heights equal, frozen | Consensus halted — check peer connectivity, validator quorum |
| `lag = 1` sustained across multiple polls, blockstore advancing | The app's commit handler hangs — `kubectl logs <pod> -c seid` for app-side panic/deadlock. A single-shot `lag = 1` is normal sampling-race noise at 200ms blocks; poll 3–5× to confirm. |
| `lag > 1` and growing, outside of restart/state-sync | App falling behind structurally; expect no catch-up without intervention |
| `lag` shrinks over time | Catch-up after restart or state-sync; healthy |

## seid CrashLoopBackOff: `invalid state-commit.sc-write-mode "cosmos_only"`

**Symptom.** Every seid container CrashLoopBackOffs; `kubectl logs <pod> -c seid --previous` shows `Error: invalid state-commit.sc-write-mode "cosmos_only"` followed by the `seid start` usage dump. SeiNetwork may still report `Ready` (the controller does not gate on seid actually serving).

**Cause.** config-apply renders the SeiDB write/read-mode keys **omitted**, so each seid binary applies its own native default. No rendered default can mismatch the image. This symptom therefore has exactly two causes.

(a) The node runs a **stale sidecar** via a `spec.sidecar` image pin. Stale sidecar builds rendered a concrete `cosmos_only` default that main/nightly seid images reject; strip the pin (Guardrail 8 in `SKILL.md`). (b) An **explicit write-mode override** carries a value the pinned image rejects (the accepted value sets differ across seid generations).

**Fix.** Strip any `spec.sidecar` image pin. When you do need an explicit mode, set a value the image accepts — `memiavl_only` (normal), or `migrate_evm` for a SeiDB-migration chain — using the **unified override key** `storage.state_commit.write_mode`:

```bash
# SeiNetwork (validators): spec.configOverrides
seictl network apply <id> ... --set spec.configOverrides."storage.state_commit.write_mode"=memiavl_only
# follower SeiNode: spec.overrides
seictl node apply <id> ... --set spec.overrides."storage.state_commit.write_mode"=memiavl_only
```

**Footgun — the key, not just the value.** The override **key** must be the unified-schema path `storage.state_commit.write_mode`. config-apply **silently rejects** the raw app.toml path `state-commit.sc-write-mode` (`unknown config field`). The broken `cosmos_only` default then stands, and the symptom persists even though you "set the override." (`config-apply` validation lives in sei-config; check the controller log for `unknown config field` if a write-mode override seems ignored.)

> The structural endgame — config knowledge living in the binary itself (ConfigManager, `SEI_CONFIG_MANAGER=v2`, PLT-775) — is in flight; render omission already removes the default-mismatch class.

## configOverrides edits never reach a Running node

**Symptom.** You edit `spec.configOverrides` on a SeiNetwork (or `spec.overrides` on a SeiNode). The change propagates to the child SeiNode's **spec**, but the node's on-disk `config.toml`/`app.toml` — and its behavior — never change. No event, no condition, no error.

**Cause.** The config-apply task consumes overrides **only on init paths** (bootstrap / snapshot-restore / state-sync / genesis). A Running node has **no day-2 apply path**. Update plans fired by image drift carry only the controller-owned `[p2p]` keys (external-address, persistent-peers). The reconciler has no override-drift detection (the spec edit enqueues a reconcile that produces a nil plan).

A `RestartSeid` task re-reads the **unchanged on-disk config** — a restart is not an apply. The spec silently diverges from live state.

**Fix.** Set overrides **before first boot** whenever possible. For a Running node, the change takes effect only when the node next traverses an init path. That means a re-provision: delete + recreate with a **fresh chain-id** (see the chain-id-reuse entry below). Or a snapshot-restore / state-sync task. Verify what a node is *actually* running by reading its rendered files, never by trusting the spec: `kubectl exec <pod> -c seid -- cat /sei/config/app.toml`.

**The day-2 path is `spec.configValues`.** Express the key as a typed config value (`seictl ... --config-value app.toml:<key>=<value>`) instead of an override. The controller detects configValues drift on a Running node and runs a `config-update` plan (`config-patch → config-validate → mark-ready → restart-seid`; regenerate base, patch peers, overlay configValues, validate, restart seid in place — no pod replacement; source: controller `internal/planner/config_update.go`, `planner.go:configValuesDrifted`). On a SeiNetwork that restarts every validator at once, so block production stops until >2/3 are back — never do it mid-bench. See `seictl-cli.md` → *Typed config values*.

## ConfigValuesValid=False / configValues edit produced no restart

**Symptom A.** You edit `spec.configValues` on a SeiNetwork and nothing happens on the children. `kubectl get seinetwork <id> -o jsonpath='{.status.conditions[?(@.type=="ConfigValuesValid")]}'` shows `status: False, reason: InvalidConfigValues`, and `kubectl get events -n eng-<alias> --field-selector reason=InvalidConfigValues` carries the parse error.

**Cause.** The CRD admits the set, but the TOML overlay cannot be built from it: a `null` nested in an array or table, a number outside int64/float64, or two entries under one `fileName` whose dotted paths overlap (`evm` and `evm.http_enabled`). The controller validates on the SeiNetwork before propagating, so every child keeps its **last good set** and keeps running — nothing is wedged, but the edit never lands.

**Fix.** Correct the entry named in the condition message. `seictl --config-value` rejects nulls, bad names, and duplicate `(fileName,key)` pairs at render, so a False condition for those causes means the CR was hand-edited; re-render through seictl. Overlapping dotted paths under one `fileName` (`evm` and `evm.http_enabled`) pass seictl and fail only here: re-rendering reproduces the condition, so drop the shorter path (or fold its table into individual leaf keys) and re-apply.

**Symptom B.** The same edit on a standalone SeiNode (or a child after the network's set propagated) leaves the node with `NodeUpdateInProgress=True` and a plan stuck at `config-validate` or `restart-seid`, and later image rolls also stall.

**Cause.** On a SeiNode there is no pre-validation: an entry the overlay can build but seid refuses (unknown key, wrong type for the key, invalid value) fails at `config-validate` or at seid start, and the failed plan blocks every subsequent update plan for that node until the entry is corrected.

**Fix.** Read the failing task from `kubectl get seinode <name> -o jsonpath='{.status.plan}'` and the seid log (`kubectl logs <pod> -c seid`), fix or remove the entry, and let the next reconcile rebuild the plan. Check the rendered file afterwards (`kubectl exec <pod> -c seid -- cat /sei/config/app.toml`) — the same edit also regenerates the base, so `[statesync]` trust-point keys and out-of-band giga migration keys are gone unless they were also expressed as configValues.

**Symptom C.** The edit is valid, but the node stays Running with no plan and `NodeUpdateInProgress=False, reason: ConfigBaselineUnobserved`.

**Cause.** `status.currentConfigValuesHash` is empty: the node was materialized by a controller build that predates configValues, so there is no observed baseline to diff against. Drift detection starts after the next image update regenerates its config. Nodes provisioned by the current controller never show this.

**Fix.** Roll the image (any `spec.image` change) or re-provision; the configValues land as part of that update.

## Chain wedged at height 0 after delete-and-recreate (chain-id reuse)

**Symptom.** Pods all `Ready` (0 restarts) and the SeiNetwork `Ready`, but the chain never produces a block: `kubectl exec <pod> -c seid -- seid status` shows `latest_block_height: 0`, `catching_up: true`, `latest_block_time: 1970-01-01`, and seid spams `level=ERROR msg="no progress since last advance" logger=tendermint/internal/blocksync` (last_advance frozen at startup). Validators never form consensus.

**Cause.** The **chain-id** keys the genesis ceremony's S3 artifacts — `genesis.json`, `peers.json`, per-node gentxs/identities. The seictl assembler writes and reads them under the prefix `<chain-id>/` in the genesis-artifacts bucket (see `seictl/sidecar/tasks/assemble_genesis.go`, `prefix := a.chainID + "/"`). Deleting and re-creating a chain with the **same chain-id** regenerates fresh node/validator keys. It leaves the prior incarnation's artifacts under that prefix. The fresh keys then mismatch the stale validator set / peer identities, so no validator is a proper member and consensus cannot form.

(Confirmed empirically: an identical 4-validator network — same image, same `migrate_evm` — reached height 740 under a **new** chain-id. The reused-chain-id one stayed at height 0.)

**Fix.** Recreate with a **fresh chain-id** (cleanest — guarantees no stale artifacts). Or purge the artifacts under the old chain-id's `<chain-id>/` prefix in the genesis bucket before re-creating. A fresh chain-id is the safe default for a throwaway dev chain.

> Not a chain bug — a ceremony-lifecycle footgun. Reuse a chain-id only after clearing its S3 artifacts.

## Profiling (pprof)

Dev chains applied via the seictl `genesis-chain` and `rpc` presets carry `network.rpc.pprof_listen_address: "0.0.0.0:6060"` — in `spec.configOverrides` on a SeiNetwork, `spec.overrides` on a follower SeiNode. seid serves the Go pprof endpoint at port 6060 inside the pod.

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

If the override did not take (older seictl version, hand-rolled CR), confirm and recover:

```sh
# Did the override land in config.toml?
kubectl exec -n eng-<alias> <pod> -c seid -- grep pprof_listen_address /.sei/config/config.toml

# Apply via --set on a follower SeiNode (overrides is flat — no spec.template)
seictl node apply <id>-rpc-<k> --preset rpc --chain-id <id> --network <id> --image <ref> \
  --cpu <cpu> --memory <mem> --storage <size> [--iops <iops> --throughput <tput>] \
  --set spec.overrides."network.rpc.pprof_listen_address"="0.0.0.0:6060" \
  -n eng-<alias>
```

Carry the footprint and any storage-performance selection on this re-apply, matching the values the node already holds. Both are create-only. Omitting them on an existing node re-applies a different shape, which the apiserver rejects as `Invalid`. On a fresh node it installs the preset shape in silence instead.

**Running-node caveat**: an override applied to a Running node never reaches its on-disk config. It takes effect only on the node's next init path (see *configOverrides edits never reach a Running node* above). To profile an existing node, express the key as a config value instead (`--config-value config.toml:rpc.pprof_laddr=...` — the raw config.toml key, not the unified `network.rpc.pprof_listen_address` the override takes), which restarts that node only, or re-provision the follower with the override set from first boot.

**Production caveat**: seictl ships one set of presets (`genesis-chain`, `rpc`) used in both dev and prod; there is no separate prod preset. When promoting a follower to prod, strip the pprof override explicitly: `--set spec.overrides."network.rpc.pprof_listen_address"=""`. Pprof must never be reachable in prod — it exposes profile dumps and memory state to anyone with HTTP access to port 6060.

## Preserving the data dir for debugging

A node is in a bad state, and you want a point-in-time copy of its data dir for offline inspection. Use `cp -al` **inside the PVC**. It avoids doubling storage or waiting minutes-to-hours for `cp -r`:

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

PVC space will not fully release until the original files are also unlinked (compaction takes care of this naturally as seid runs).

### vs. retained data on delete

For a SeiNode, `spec.dataVolume.import` governs whether its PVC survives deletion: an imported PVC survives, a controller-managed one does not — see **Phase: Failed** above. A `SeiNetwork`'s `spec.deletionPolicy` (defaults `Delete`) governs something different. `Retain` makes the controller orphan its generated validator SeiNodes, and thus their PVCs, when you delete the network; the controller annotates each orphan with `sei.io/retained-from-seinetwork` and `sei.io/retain-reason`. That is useful when you tear down a network but keep a validator's disk for forensics. The hardlink trick above is for **live debugging** while the node continues running. The three are complementary, not redundant.
