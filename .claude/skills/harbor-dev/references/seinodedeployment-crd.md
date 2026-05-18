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
- `spec.genesis.overrides` — flat map with dotted cosmos-module-path KEYS, merged into `app_state` before gentx. First key segment must be a module (`staking`, `bank`, `gov`, ...) — `app_state.` prefix is wrong, and `consensus_params.*` isn't reachable. Example: `"staking.params.unbonding_time": "600s"`. Cannot be emitted via seictl `--set`; render with seictl, then patch the YAML with `yq` (see `seictl-cli.md`).
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

## Everything else

`kubectl explain seinodedeployment.spec --recursive` and the source at `sei-protocol/sei-k8s-controller/api/v1alpha1/seinodedeployment_types.go`.
