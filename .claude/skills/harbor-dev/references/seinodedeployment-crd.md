# SeiNodeDeployment CRD reference (operations view)

> If this disagrees with the in-cluster CRD, **the CRD wins**.
> Run `kubectl explain seinodedeployment.spec --recursive` for the source of truth.
> File an issue at sei-protocol/sei-k8s-controller/issues if you spot drift.

## What it is

A `SeiNodeDeployment` (`sei.io/v1alpha1`) manages a **fleet** of `SeiNode`s — N replicas of a `SeiNodeTemplate` plus fleet-level concerns: shared networking (HTTPRoute, Services), genesis ceremonies (fresh or fork-based), monitoring (ServiceMonitor), and deployment strategies (in-place, blue-green, hard-fork).

Used by `seictl nd apply` to materialize the validators or RPC fleet from a preset.

## Spec fields you'll touch

The 5 fields engineers actually edit (or seictl renders):

- `spec.replicas` — number of nodes in the fleet
- `spec.template` — `SeiNodeTemplate` (stamped N times, same shape as `SeiNode.spec`)
- `spec.template.spec.overrides` — flat map of dotted TOML keys (`config.toml`/`app.toml`/`client.toml`) merged onto mode defaults at config-apply.
- `spec.genesis` — optional genesis ceremony config (chainId, validators, fork support)
- `spec.genesis.overrides` — flat map with dotted cosmos-module-path KEYS, merged into `app_state` before gentx. First key segment must be a module (`staking`, `bank`, `gov`, ...) — `app_state.` prefix is wrong, and `consensus_params.*` isn't reachable. Example: `"staking.params.unbonding_time": "600s"`. Render with `seictl nd apply --genesis-override <key>=<value>` (since v0.0.51).
- `spec.networking` — HTTPRoute, ClusterIP Service, AuthorizationPolicy
- `spec.updateStrategy` — `InPlace` / `BlueGreen` / `HardFork`

## Status fields you'll read when debugging

The fields the agent reads (most via `seictl nd get -o jsonpath` or `seictl nd watch` NDJSON):

- `.status.phase` — `Pending` → `Initializing` → `Ready` → `Upgrading` / `Degraded` / `Failed` / `Terminating`. The `--until` argument to `seictl nd watch` matches this exactly.
- `.status.conditions[type=NodesReady]` — child SeiNodes `status.readyReplicas == replicas`.
- `.status.conditions[type=RouteReady]` — HTTPRoute hostname resolves in DNS.
- `.status.conditions[type=GenesisCeremonyComplete]` — genesis.json assembled.
- `.status.plan[*]` — group-level plan state for genesis assembly + rollout. On terminal `Failed`, `.status.plan.failedTaskDetail.error` carries the cause and is lifted to stderr by `seictl nd watch`.
- `.status.endpoints.*` — controller-published service URLs: `tendermintRpc[]`, `tendermintRest[]`, `evmJsonRpc[]`, `evmWs[]`. Each is an array even if there's only one entry.
- `.status.perPodServices[]` — per-pod headless Service handles for callers that need pod-targeted connectivity (seiload's WebSocket block collector, gRPC streaming, etc.). Each entry has `name`, `namespace`, `ports.{evmHttp, evmWs, ...}`.
- `.status.internalService` — the aggregate ClusterIP Service handle backing `.status.endpoints.*`.

## Peer auto-wiring (`rpc` preset)

`seictl nd apply --preset rpc --chain-id <id>` writes a single label-based peer source:

```yaml
spec.template.spec.peers:
- label:
    selector:
      sei.io/chain: <chain-id>
```

The SND controller stamps `sei.io/chain=<chainID>` on every child SeiNode as a **controller-reserved label** — `sei.io/chain`, `sei.io/nodedeployment`, `sei.io/nodedeployment-ordinal`, and `sei.io/revision` are written after template labels and cannot be suppressed by `spec.template.metadata.labels` (source: `sei-protocol/sei-k8s-controller/internal/controller/nodedeployment/labels.go:48-58`). The SeiNode controller resolves selector matches to headless DNS, then the sidecar's `discover-peers` task queries `:26657 /status` on each match to harvest CometBFT node IDs.

**Failure mode**: chain-id mismatch (typo, fork rename) yields empty `status.resolvedPeers`; the node sits in initial sync forever with no persistent peers. Confirm by checking `kubectl get seinode -l sei.io/chain=<id>` returns both the genesis validators and the new RPC.

**Anti-pattern**: do not hand-wire a `static` `PeerSource` for in-cluster peers — static encodes ephemeral pod IPs and breaks under StatefulSet rescheduling. Let `label.selector` resolve to headless DNS.

## Snapshot bootstrap semantics

`spec.template.spec.fullNode.snapshot.s3.targetHeight` is a **ceiling, not an exact pin**. The sidecar lists `s3://harbor-sei-snapshots/<chainID>/state-sync/*.tar.gz`, parses heights from filenames, and selects `max(height ≤ targetHeight)` (source: `sei-protocol/seictl/sidecar/tasks/snapshot_restore.go:162-210`). `targetHeight=0` means "use the newest available." If no snapshot ≤ `targetHeight` exists, the `snapshot-restore` task fails with `no snapshot found at or below height <H>`. `latest.txt` in the bucket is publisher bookkeeping — the sidecar does not read it on restore. Discovery recipe: `references/aws-dependencies.md#snapshot-discovery-harbor-sei-snapshots`.

**Anti-pattern**: pinning `targetHeight` to a specific block expecting an exact match. Use `0` for newest, or first run `aws s3 ls` and pick a value ≥ a published height.

**Eng-workspace rule**: `spec.template.spec.fullNode.snapshotGeneration` stays unset on every eng-workspace SND (Guardrail #6 in `SKILL.md`).

## Everything else

`kubectl explain seinodedeployment.spec --recursive` and the source at `sei-protocol/sei-k8s-controller/api/v1alpha1/seinodedeployment_types.go`.
