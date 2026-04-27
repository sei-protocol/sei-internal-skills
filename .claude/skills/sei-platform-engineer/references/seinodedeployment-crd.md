# SeiNodeDeployment CRD reference (operations view)

> If this disagrees with the in-cluster CRD, **the CRD wins**.
> Run `kubectl explain seinodedeployment.spec --recursive` for the source of truth.
> File an issue at sei-protocol/sei-k8s-controller/issues if you spot drift.

Last verified: 2026-04-26 against sei-k8s-controller `<version-pending>`.

## What it is

A `SeiNodeDeployment` (`sei.io/v1alpha1`) manages a **fleet** of `SeiNode`s — N replicas of a `SeiNodeTemplate` plus fleet-level concerns: shared networking (HTTPRoute, Services), genesis ceremonies (fresh or fork-based), monitoring (ServiceMonitor), and deployment strategies (in-place, blue-green, hard-fork).

Used by `seictl bench up` to spin the validators and RPC fleet for a benchmark.

## Spec fields you'll touch

[outline]

The 5 fields engineers actually edit (or seictl renders):

- `spec.replicas` — number of nodes in the fleet
- `spec.template` — `SeiNodeTemplate` (stamped N times, same shape as `SeiNode.spec`)
- `spec.genesis` — optional genesis ceremony config (chainId, validators, fork support)
- `spec.networking` — HTTPRoute, ClusterIP Service, AuthorizationPolicy
- `spec.updateStrategy` — `InPlace` / `BlueGreen` / `HardFork`

## Status fields you'll read when debugging

[outline]

The 5 conditions and fields engineers actually read:

- `.status.phase` — `Pending` → `Initializing` → `Ready` → `Upgrading` / `Degraded` / `Failed` / `Terminating`
- `.status.conditions[type=NodesReady]` — child SeiNodes status.readyReplicas == replicas
- `.status.conditions[type=RouteReady]` — HTTPRoute hostname resolves in DNS
- `.status.conditions[type=GenesisCeremonyComplete]` — genesis.json assembled
- `.status.plan[*]` — group-level plan state for genesis assembly + rollout

## Everything else

`kubectl explain seinodedeployment.spec --recursive` and the source at `sei-protocol/sei-k8s-controller/api/v1alpha1/seinodedeployment_types.go`.
