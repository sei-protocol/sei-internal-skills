# State-sync bootstrap onto a self-owned chain (`rpcServers` witnesses)

> Last verified 2026-07-08 against sei-k8s-controller `254375d` (the commit that
> shipped `SnapshotSource.rpcServers`, live on harbor). If this disagrees with
> `kubectl explain seinode.spec.fullNode.snapshot`, the CRD wins.

Sections: [mental model](#the-mental-model--witnesses-vs-snapshot-providers-two-different-jobs) ·
[preconditions](#preconditions-check-before-rendering) · [spec shape](#the-spec-shape) ·
[after Flux applies](#what-happens-after-flux-applies) · [failure modes](#failure-modes-and-where-they-surface)

**When to use:** the engineer has a running chain in their namespace and wants
to attach a follower **without replaying from genesis** — "add an RPC node with
state sync", "bootstrap from my own chain", "don't replay 400k blocks". For a
young chain (minutes–hours old), genesis replay is usually simpler and needs
none of this; offer that first. State sync earns its setup on chains with real
accumulated height or when the engineer is explicitly testing the state-sync
path itself.

## The mental model — witnesses vs snapshot providers (two different jobs)

- **`rpcServers` = light-client witnesses.** CometBFT RPC endpoints (`:26657`)
  used to acquire and cross-verify the trust point (height + hash). They must
  serve **this chain's** RPC. On harbor, every chain member — validators
  included — serves RPC on 26657, so any two chain members work.
- **Snapshot chunks come over p2p (`:26656`) from snapshot-serving peers** —
  the `spec.peers` label selector rail, not `rpcServers`. At least one peer
  must have snapshot creation enabled AND have already produced a snapshot
  (snapshots appear only when the chain crosses the snapshot interval —
  "creation enabled" on a chain below its first interval means zero snapshots
  and the bootstrap finds nothing).

Declaring `rpcServers` **replaces** the platform's canonical-syncer registry
for that node — the registry (which only carries long-lived chains like
pacific-1) is not consulted. This is what makes self-owned-chain state sync
self-service: no platform PR.

## Preconditions (check before rendering)

1. **The chain has ≥2 members serving RPC** — read their published endpoints
   verbatim (never reconstruct DNS): recipe #1 in
   `cluster-inspection-recipes.md` for followers, or for validators the
   headless-service form visible in `kubectl get svc -n eng-<alias>`.
2. **≥1 peer has produced a snapshot** — snapshot creation on (the chain's
   `storage.snapshot_interval` override or equivalent) and height has crossed
   the interval at least once.
3. **Same image as the chain** — `kubectl get seinetwork <id> -n eng-<alias> -o jsonpath='{.spec.image}'`.
4. **The harbor CRD carries the field** — `kubectl explain seinode.spec.fullNode.snapshot.rpcServers`
   exits 0. If it doesn't, the controller predates the feature: halt and
   surface (the fallback is genesis replay, not improvisation).

## The spec shape

```yaml
spec:
  chainId: <chain-id>
  image: <same seid image as the chain, verbatim>
  peers:
    - label:
        selector:
          sei.io/seinetwork: <chain-id>     # p2p rail — snapshot chunks flow here
  fullNode:
    snapshot:
      stateSync: {}                          # exactly one of s3 | stateSync
      trustPeriod: "9999h0m0s"               # must exceed the snapshot's age; generous is fine on dev
      rpcServers:                            # light-client witnesses
        - <member-0>.<svc>.eng-<alias>.svc.cluster.local:26657
        - <member-1>.<svc>.eng-<alias>.svc.cluster.local:26657
```

`rpcServers` contract (admission-enforced): bare `host:port` — no scheme, no
IPv6 literals, no commas inside an item; **minimum 2 entries**; duplicates
rejected. A single-witness request is rejected at `kubectl apply` time, not at
runtime — CometBFT light-client verification needs two independent servers.

Render via `seictl node apply <id>-rpc-<k> --preset rpc --chain-id <id>
--network <id> --image <ref> -n eng-<alias> --dry-run` and add the
`spec.fullNode.snapshot` block to the emitted YAML (check `seictl node apply
--help` for current `--set` list-literal support before trying to express the
list inline; hand-editing the rendered YAML and re-validating with
`kubectl apply --dry-run=server -f <file>` is the reliable path). **Inspect the
dry-run/server response and confirm `rpcServers` survived** — a cluster whose
CRD predates the field prunes it silently.

## What happens after Flux applies

The controller's `StateSyncReady` gate resolves the witness set from the spec
(condition message says "N rpc-servers declared on spec"), builds the init
plan, creates the data PVC, and the pod schedules. The sidecar queries the
witnesses for a trust point, writes `[statesync]` config, and seid pulls
snapshot chunks over p2p from the label-selector peers, restores, and
block-syncs the tail. Watch with
`seictl node watch <id>-rpc-<k> --until=Running -n eng-<alias>`.

## Failure modes (and where they surface)

| Symptom | Cause | Fix |
|---|---|---|
| Rejected at `kubectl apply` — minItems / pattern / duplicate | <2 witnesses, scheme prefix, missing port, comma in item, IPv6 | Fix the list; the admission message names the rule |
| SeiNode condition `StateSyncReady=False / NoSyncersConfigured`, **no pod and no StatefulSet appear** | `stateSync: {}` set but no `rpcServers` and no registry entry for this chain (eng chains never have one). The controller deliberately holds StatefulSet creation while the gate blocks — this is the fixed version of the old stranded-Pending-pod failure | Add ≥2 `rpcServers`, or drop `stateSync` and genesis-replay. The condition message names both remediations |
| Plan runs but the sidecar's state-sync configure task fails: "no reachable RPC witness" | Witness endpoints wrong/unreachable, or the chain members aren't up | Fix endpoints (read them verbatim from status); confirm members Running |
| State sync starts, finds no snapshots, seid retries/aborts | No peer has actually produced a snapshot yet (young chain, interval not crossed) | Wait for the first snapshot interval, or genesis-replay instead |
| Node syncs then halts on app-hash mismatch / wrong height | **Wrong-chain witness** — an endpoint on a different chain passed shape validation and supplied a foreign trust point. Shape is the ONLY admission validation; chain membership is not checked (sidecar-side assertion tracked as PLT-793) | Every `rpcServers` entry must be a member of `spec.chainId`'s chain. Diagnose via the sidecar container logs |
| seid crash-loops on every boot: `LoadStateFromDBOrGenesisDocProvider(): fromProto: validatorSet proposer error: nil validator` | **Poisoned first boot** — the first seid start under state sync persisted a genesis-derived empty validator set to the state DB, then died; every later boot fails re-loading that state before it can re-attempt state sync. Restarts can never recover it | Wipe the data dir (via the sidecar container) or re-provision via git so the PVC is recreated — and verify the PVC actually went away, or the replacement remounts the poisoned volume. Watch the fresh first boot live to catch the original crash cause |

The controller reports `StateSyncReady=True` on config alone — it does not
probe witness liveness or chain membership. "Condition True + runtime failure
in the sidecar logs" is the designed failure surface for bad endpoints; it is
strictly more debuggable than the pre-fix behavior (a Pending pod pointing at
a PVC that would never exist), but it means the sidecar log, not the CR
status, is where endpoint mistakes show up.

Never enable `spec.fullNode.snapshotGeneration` on the new follower to "help"
— the snapshot-generation guardrail applies unchanged (eng-workspace followers
are consumers, never publishers); the chain's existing members are the
snapshot source.
