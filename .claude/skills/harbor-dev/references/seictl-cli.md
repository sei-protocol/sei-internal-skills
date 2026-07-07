# seictl CLI surface

Canonical command reference for the engineer-facing surface. **`seictl network --help` / `seictl node --help` are the source of truth** — when this file disagrees, the CLI wins.

## Top-level commands

| Command | Domain | What it does |
|---|---|---|
| `seictl config patch` | local | Patch app.toml/client.toml/config.toml |
| `seictl genesis patch` | local | Patch genesis JSON |
| `seictl patch` | local | Generic TOML/JSON merge-patch |
| `seictl serve` | local | Run the in-pod sidecar HTTP server |
| `seictl await` | local | Wait condition |
| `seictl report` | local | Analyze shadow chain comparison data |
| `seictl network` | cluster | Manage `SeiNetwork` CRs via the `genesis-chain` preset |
| `seictl node` | cluster | Manage `SeiNode` CRs via the `rpc` preset |

The skill invokes the `network` and `node` subtrees above. The `local` commands are out of scope for engineer-facing intents.

The pre-#133 cluster verbs (`context`, `onboard`, `bench up/down/list`) are gone — replaced by the preset-driven `network`/`node` trees below. The single `nodedeployment` (alias `nd`) tree that preceded these is also gone: `SeiNodeDeployment` was a fleet Kind that split into `SeiNetwork` (the genesis validator pool) + standalone `SeiNode` CRs (each follower). If a reference to any of those older names surfaces in older docs, it's stale.

## The two trees

`seictl network` and `seictl node` share the same five verbs — `apply`, `get`, `list`, `delete`, `watch` — and the same kubectl-shaped flag conventions. They differ only in the Kind they target and the preset they carry:

- `seictl network` → `seinetworks.sei.io/v1alpha1`, preset `genesis-chain`. One CR per chain; the controller mints N validator SeiNodes from a genesis ceremony.
- `seictl node` → `seinodes.sei.io/v1alpha1`, preset `rpc`. One CR per follower; an RPC fleet of N is **N standalone `node apply` calls** (SeiNode has no `spec.replicas` — see `seinode-crd.md`).

**Common flags on every verb (both trees):**

- `--kubeconfig <path>` (also `$KUBECONFIG`) — colon-merge honored. Defaults to `$HOME/.kube/config` or in-cluster auth.
- `--namespace <ns>` / `-n <ns>` — target namespace. Falls back to the kubeconfig context's default namespace, then the in-cluster ServiceAccount's namespace.

The skill always passes `-n eng-<alias>` explicitly.

## `seictl network apply`

```
seictl network apply <name>
                     --preset genesis-chain
                     [--chain-id <id>] [--image <ref>] [--replicas N]
                     [--genesis-account <addr>:<balance>] [--genesis-account ...]
                     [--genesis-override <module.field>=<value>] [--genesis-override ...]
                     [--set <dotted.path>=<value>] [--set ...]
                     [--dry-run]
                     [-n <ns>] [--kubeconfig <path>]
```

Loads the `genesis-chain` preset, applies discrete-flag and `--set` overrides, and **server-side-applies** the result. With `--dry-run`, the apiserver validates and returns the would-be-applied CR without persisting.

**Layering, lowest precedence first:**

1. Preset YAML (embedded in the seictl binary).
2. Discrete flags (`--chain-id`, `--image`, `--replicas`).
3. `--set <dotted.path>=<value>`. Strategic-merge: maps merge per-key, lists replace wholesale. Wins on collision with discrete flags. SeiNetwork config overrides live under `spec.configOverrides` (reach them via `--set`); there is **no `--override` flag** on `network apply`. Overrides take effect only on an **init path** — set them at create time; an edit to a Running network's overrides never reaches its nodes' on-disk config (see `troubleshooting-seinode.md` → *configOverrides edits never reach a Running node*).

**Immutability (apply-time, load-bearing):** `spec.genesis` and `spec.replicas` are admission-immutable. Re-applying `network apply <same-name>` with a changed `--chain-id` or `--replicas` is **rejected** with `metav1.Status.reason=Invalid` — it is not a silent no-op. To change either, `delete` + re-create. This is the new-CRD analogue of the old `updateStrategy` trap.

**Required:** `<name>` (positional) and `--preset genesis-chain`. `--chain-id` and `--image` must resolve after layering — if either is missing in the rendered CR, the apiserver rejects with `metav1.Status.reason=Invalid`.

**Output (success):** the post-apply `SeiNetwork` CR on stdout as JSON. Same shape as `kubectl get seinetwork <name> -o json`.

**Output (failure):** `metav1.Status` on stderr. Non-zero exit. Discriminate with `jq -r .reason` (e.g., `Invalid`, `Forbidden`, `NotFound`, `AlreadyExists`).

**Stderr provenance line (always):** `seictl: applying SeiNetwork <ns>/<name> to <api-server>` (or `applying (dry-run)`).

## `seictl node apply`

```
seictl node apply <name>
                  --preset rpc
                  [--chain-id <id>] [--image <ref>] --network <X>
                  [--external-address <host>:<port>]
                  [--override <toml.key>=<value>] [--override ...]
                  [--set <dotted.path>=<value>] [--set ...]
                  [--dry-run]
                  [-n <ns>] [--kubeconfig <path>]
```

Loads the `rpc` preset and server-side-applies a single `SeiNode`. An RPC fleet of N is N of these calls — `<name>` per follower (convention `<id>-rpc-<k>` for `k` in `0..N-1`).

**No `--replicas`.** SeiNode is a single node; there is no fan-out flag. The skill owns the N-CR loop (see `ephemeral-chain-flow.md`). Reaching for `--replicas` here is the most common migration mistake from the `nd` era.

**`--network <X>`** is the peer rail. It auto-wires `spec.peers[0].label.selector.sei.io/seinetwork=<X>` (who to peer with) AND stamps `metadata.labels{sei.io/seinetwork=<X>, sei.io/role=node}` on the CR — the producer side of the fleet list query (see `cluster-inspection-recipes.md` recipe #1). `--network` is independent of `--chain-id`: `--chain-id` sets the node's own `spec.chainId`; `--network` sets who it peers with. For an in-namespace ephemeral chain, pass the same value for both. A `node apply` with neither errors `Invalid`.

**`--external-address`** advertises a reachable host:port for external p2p. Leave unset for in-cluster ephemeral chains (followers peer over headless DNS).

**`--override <toml.key>=<value>`** targets `spec.overrides` (per-node `config.toml`/`app.toml`, applied at config-apply — an **init-path** task). Set overrides at create time: a re-apply against a Running node updates only the spec; the on-disk config never changes until the node next traverses an init path (see `troubleshooting-seinode.md` → *configOverrides edits never reach a Running node*). `--set` does strategic-merge on the whole spec and wins on collision.

**Required:** `<name>`, `--preset rpc`. `--chain-id`, `--image`, and `--network` must resolve after layering, else `Invalid`.

**Output (success):** the post-apply `SeiNode` CR on stdout as JSON.

**Output (failure):** `metav1.Status` on stderr; non-zero exit; `jq -r .reason` to discriminate.

## `seictl {network,node} get`

```
seictl network get <name> [-o yaml | json | name | jsonpath=<template>] [-n <ns>] [--kubeconfig <path>]
seictl node    get <name> [-o yaml | json | name | jsonpath=<template>] [-n <ns>] [--kubeconfig <path>]
```

Read-only. Returns the CR as `kubectl get seinetwork|seinode <name> -o <format>` would — yaml is the default.

**Output formats:**
- `yaml` (default) / `json`: full CR.
- `name`: `seinetwork.sei.io/<name>` or `seinode.sei.io/<name>`.
- `jsonpath=<template>`: kubectl-style JSONPath. A follower's published endpoint is a scalar leaf: `seictl node get <name> -o jsonpath='{.status.endpoint.evmJsonRpc}'`.

**Failure:** `metav1.Status` on stderr (`reason=NotFound` if absent; `Forbidden` if RBAC denies).

## `seictl {network,node} list`

```
seictl network list [-A] [-l <label-selector>] [-o yaml | json | name | jsonpath=<template>] [-n <ns>]
seictl node    list [-A] [-l <label-selector>] [-o yaml | json | name | jsonpath=<template>] [-n <ns>]
```

Returns a `SeiNetworkList` / `SeiNodeList`. `-A` overrides `-n` and lists across all namespaces. `--selector` (`-l`) accepts standard label selectors (e.g. `-l sei.io/seinetwork=foo,sei.io/role=node`).

**`node list -o json` is the PRIMARY fleet read** — `.items[].status.endpoint` carries each follower's published URLs; assemble the fleet of URLs via `jq` over `.items[]` (see `cluster-inspection-recipes.md` recipe #1).

**Failure:** `metav1.Status` on stderr.

## `seictl {network,node} delete`

```
seictl network delete <name> [--cascade foreground | background | orphan] [-n <ns>]
seictl node    delete <name> [--cascade foreground | background | orphan] [-n <ns>]
```

Issues a Delete against the named CR. Default propagation is `foreground` (waits for child resources before the CR itself is removed). `background` returns immediately; `orphan` leaves children behind.

`--cascade` (the **client** propagation policy) is orthogonal to the CRD's own `spec.deletionPolicy`. A `SeiNetwork`'s `deletionPolicy` defaults to `Retain` (it governs whether the controller orphans its generated children); `--cascade` governs how the client waits. Both apply.

**Output (success):** `seinetwork.sei.io/<name> deleted` / `seinode.sei.io/<name> deleted` on stdout. Exit 0.

**Failure:** `metav1.Status` on stderr (`reason=NotFound` if already gone).

## `seictl {network,node} watch`

```
seictl network watch <name> --until <phase> [--timeout <duration>] [-n <ns>]
seictl node    watch <name> --until <phase> [--timeout <duration>] [-n <ns>]
```

Streams every event for the named CR as one NDJSON line on stdout. **Exits 0** when `.status.phase == --until` (exact match). **Exits 1** on `--timeout` exceeded (`metav1.Status.reason=Timeout`), terminal `Failed` phase (stderr lifts `.status.plan.failedTaskDetail.error`), or a transient API failure.

**Two phase vocabularies — the terminal phases differ:**

- `seictl network watch --until=Ready` — `SeiNetworkPhase` reaches `Ready`. Common values: `Pending`, `Initializing`, `Ready`, `Degraded`, `Failed`, `Terminating`.
- `seictl node watch --until=Running` — `SeiNodePhase` reaches `Running`. Common values: `Pending`, `Initializing`, `Running`, `Failed`, `Terminating`. **There is no `Ready` on a node** — `node watch --until=Ready` is rejected at parse with `Invalid`. An operator who waits for a node to reach `Ready` waits forever.

The `--until` flag is **required**. Matching is exact; an illegal value for the tree errors `Invalid` at parse rather than timing out silently.

**Idiom for the agent:** `network apply` then `network watch --until=Ready` is the genesis 2-step; for an RPC fleet, loop `node apply` then `node watch --until=Running` per follower, then assemble endpoints via `node list … -o json | jq` (the fleet is N CRs — there is no single object to read endpoints from).

## Conventions across the surface

### Output shape

Native `SeiNetwork` / `SeiNode` (or `…List`) shape on stdout. **No envelope.** Same as `kubectl get seinetwork|seinode -o <format>`. Consumers can pipe directly into `jq` or `yq`.

### Errors

Errors on stderr as `metav1.Status` (kind: Status, apiVersion: v1, status: Failure). Discrimination via `.reason` (`Invalid`, `Forbidden`, `NotFound`, `AlreadyExists`, `Timeout`, `InternalError`). Non-zero exit on every failure.

```sh
seictl network apply foo --preset genesis-chain --chain-id bar -n eng-x 2>err.json
jq -r .reason err.json   # → "Invalid" / "Forbidden" / etc.
jq -r .message err.json  # → human-readable
```

### Exit codes

`0` on success, `1` on every failure. Discrimination is via `metav1.Status.reason` on stderr, not the exit code.

### Provenance

When `seictl network|node apply` succeeds, the post-apply CR carries:

- `metadata.annotations.seictl.sei.io/preset: <preset-name>` — which preset shaped it.
- `metadata.annotations.seictl.sei.io/version: v0.0.<n>` — which seictl shipped it.
- `metadata.labels.sei.io/seinetwork: <id>` — the network a CR belongs to. Validators (controller-generated) also carry `sei.io/role=validator`; followers (from `node apply --network`) carry `sei.io/role=node`.

`kubectl get seinetwork|seinode -o yaml` surfaces these naturally — useful for `git log`-style provenance.

### No ambient state

Commands never `cd`, never modify `~/.kube/config`, never set env vars in the calling shell. Every kubectl call is explicit about context and namespace.

## Presets

Two presets, embedded in the seictl binary at `presets/*.yaml`:

### `genesis-chain` (→ `seictl network`)

Chain validators that run a fresh genesis ceremony.

```yaml
apiVersion: sei.io/v1alpha1
kind: SeiNetwork
spec:
  replicas: 4
  genesis: {}
  configOverrides:
    network.rpc.pprof_listen_address: "0.0.0.0:6060"
```

Layered with `--chain-id` and `--image`, the `genesis` block populates and `spec.image` / `spec.genesis.chainId` get set. The controller stamps `sei.io/seinetwork=<id>` and `sei.io/role=validator` on the generated validator SeiNodes.

**Default replicas: 4.** Override with `--replicas` or `--set spec.replicas=N` **at create time only** — `replicas` is immutable once the network exists (a network minted at 4 cannot be re-applied at 1; `delete` + re-create).

**Genesis params** (`--genesis-override <module.field[.field...]>=<value>`, repeatable):

```
--genesis-override staking.params.unbonding_time=600s
--genesis-override bank.params.default_send_enabled=true
--genesis-override gov.params.voting_period_seconds=120
--genesis-override mint.params.inflation='{"min":0.05,"max":0.2}'
```

Each entry writes a flat dotted-key into `spec.genesis.overrides`. The first segment is a cosmos module that exists in `app_state` (`staking`, `bank`, `gov`, `mint`, `slashing`, etc.). Values parse as JSON when they parse (numbers, bools, objects, arrays); otherwise as raw strings. To force a numeric-looking value to render as string, wrap in JSON quotes: `--genesis-override foo.bar='"42"'`.

Single-segment keys (`--genesis-override staking=...`) and empty values are rejected at apply time.

**Not reachable via this flag:** `consensus_params.*` (CometBFT consensus params, sibling to `app_state` in `genesis.json`, not under any cosmos module). `block.max_gas`, `validator.pub_key_types`, etc. are not currently reachable through `spec.genesis.overrides`.

Distinct from `--set spec.configOverrides...`, which targets per-node `config.toml`/`app.toml` applied at config-apply time.

**Funded accounts at genesis** (`--genesis-account <address>:<balance>`, repeatable):

```
--genesis-account sei1abc...:1000000000000usei
--genesis-account 0xDEAD...:1000000000000000000000usei,500uatom
```

Address can be bech32 (`sei1...`) or 0x-hex. Balance accepts the standard Cosmos coin format — one or more `<int><denom>` entries, comma-separated. Appends entries to `spec.genesis.accounts`.

### `rpc` (→ `seictl node`)

A full-node follower that peers to an existing network by label selector. One CR per follower; an RPC fleet of N is N `node apply` calls.

```yaml
apiVersion: sei.io/v1alpha1
kind: SeiNode
spec:
  fullNode: {}
  overrides:
    network.rpc.pprof_listen_address: "0.0.0.0:6060"
```

When `--network <X>` is set, the renderer auto-wires:

- Object labels: `sei.io/seinetwork=<X>`, `sei.io/role=node` (what `node list -l …` matches).
- `spec.peers[0].label.selector.sei.io/seinetwork=<X>` — points the follower at every node in the namespace tagged with that network.

**The auto-wire is what makes "chain + RPC fleet on the same network" a one-shot.** Pass `--network <id>` (the genesis network's id) to each follower; no hand-rolled `--set spec.peers...` payload.

`seictl network|node apply --preset` accepts only `genesis-chain` (network) or `rpc` (node). If an engineer asks for any other preset (archive, single validator, fork-test), it can't be served by `apply` — surface that and ask whether they want to hand-roll the CR YAML instead.
