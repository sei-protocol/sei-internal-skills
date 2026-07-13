# seictl CLI surface

Canonical command reference for the engineer-facing surface. **`seictl network --help` / `seictl node --help` / `seictl workflow --help` are the source of truth** — when this file disagrees, the CLI wins.

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
| `seictl workflow` | cluster | Re-bootstrap or migrate an **existing** SeiNode via a `SeiNodeTaskWorkflow` |

The skill invokes the `network` and `node` subtrees above. The `workflow` subtree is a third, **imperative** tree — it operates on an existing node rather than declaring a new one, so it sits outside the GitOps-PR bring-up flow (see `seictl workflow state-sync` below). The `local` commands are out of scope for engineer-facing intents.

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

## `seictl workflow state-sync`

**This is the one command in the skill that destroys data.** It re-bootstraps an *existing* SeiNode by wiping its local chain state and re-syncing, with no undo — only "resync again." Read the decision rule and the destructive-op gate below before running it, and treat every invocation (plain resync or migration) as an `rm -rf` on that node's chain data.

`seictl workflow` is a third tree alongside `network`/`node`. It shares the CRUD verbs — `apply`, `get`, `list`, `delete` — and adds `state-sync`, the task-generating verb documented here. Unlike `network`/`node`, it is **imperative**: it renders a `SeiNodeTaskWorkflow` CR, server-side-applies it, and watches it to a terminal phase. `seictl workflow --help` is the source of truth for the tree, and the CLI wins on any disagreement; the shipped seictl `workflow/README.md` documents the fuller contract.

```
seictl workflow state-sync <node>
                 [--migration GigaStore --backend <pebbledb|rocksdb>]
                 [--rpc-servers <host:port>] [--rpc-servers ...]
                 [--name <workflow-name>]
                 [--dry-run] [--no-watch] [--timeout <duration>]
                 [-n <ns>] [--kubeconfig <path>]
```

**Required:** `<node>` (the target SeiNode's `metadata.name`). The workflow is named `<node>-state-sync` unless `--name` is given. Streams plan progress as NDJSON on stdout until a terminal phase.

The recipe: hold the node's readiness gate, stop seid, **`reset-data` (wipes the data directory)**, configure state-sync, release the readiness gate, then let seid restart on the reconfigured data dir and begin catching up. A migration inserts one extra step (config-patch) after `reset-data`; otherwise a plain resync and a migration run the same steps, both including the `reset-data` wipe. Catch-up itself can be long — see the timeout note below.

### When to reach for it

**Key on node identity, not the word "state-sync."** Both this and the S3-snapshot create-time path get you a synced node without a full fresh sync, so they are easy to conflate — but they are different mechanisms at different lifecycle stages (the S3 path is a sidecar tarball restore baked into CR creation; this is CometBFT p2p state-sync on a live node):

- **New follower, nothing to preserve → the S3-snapshot create-time path** (`--set spec.fullNode.snapshot.s3.targetHeight=<h>` on `node apply`; see `ephemeral-chain-flow.md` → *Snapshot bootstrap*). The default for standing up a follower.
- **Existing node to re-bootstrap in place → `seictl workflow state-sync`.** Reach for it only when the node already exists and needs its state rebuilt (a store migration, or a follower whose local state is unrecoverable). **If a healthy node is serving, or a plain restart would fix it, you almost never want this command.** For an ephemeral follower that has merely fallen behind, `delete` + re-apply the SeiNode through the GitOps PR flow gets a fresh snapshot bootstrap that is non-destructive to shared state, in the audit trail, and human-reviewed — prefer that over the imperative wipe whenever the node is disposable.

### Before you run it — the destructive-op gate

This tree has no PR-review gate. Give it back by hand, every time, on **both** the plain and the migration path (both wipe):

1. **Get explicit engineer sign-off before the side-effecting apply.** The agent does not volunteer this command and does not self-authorize the wipe — the same rule as the direct-apply escape hatch in `ephemeral-chain-flow.md`. State plainly what will be wiped: node, namespace, plain-resync vs migration, and (for a migration) the backend.
2. **Verify the target is the intended node with a concrete read, not a self-confirm:** `seictl node get <node> -n eng-<alias> -o jsonpath='{.spec.chainId}{"  phase="}{.status.phase}{"\n"}'` — match chain-id, phase, and identity against intent. Pointing this at the wrong follower deletes its state.
3. **Never wipe a follower anyone else depends on** (a shared RPC endpoint, another engineer's load job, a dApp). Dependency is not observable from the cluster, so this is an escalation, not a check: on a long-lived `pacific-1` / `atlantic-2` follower or any shared node, escalate to the node's owner — do not wipe on agent initiative. Your own follower in your own `eng-<alias>` namespace is fair game; a shared one is not.
4. **`--dry-run` first** to render and inspect the CR without mutating anything, then apply:

```sh
# 1. render + inspect — no cluster mutation
seictl workflow state-sync <node> --dry-run -n eng-<alias>
# 2. after sign-off + target verification, apply and watch
seictl workflow state-sync <node> -n eng-<alias>
```

### Store migration (`--migration`) — irreversible, extra gate

`--migration GigaStore --backend <pebbledb|rocksdb>` sets the giga store flags on the way through the resync: state-store (`ss-enable`, `evm-ss-split`, `ss-backend`) and state-commit (`sc-enable`). Beyond the wipe every resync does, a migration **changes the storage engine and is not reversible without another resync**. Extra rules:

- **Both tokens are required** — a migration cannot be triggered by a single flag. **Omit both unless the task is explicitly a store migration**; `--migration` silently escalates a recoverable plain resync into an irreversible engine change.
- **`--backend rocksdb` needs a seid image built with `-tags rocksdbBackend`.** `reset-data` wipes *before* seid restarts on the new backend, and `--dry-run` does not check the running image's build tags — so a wrong image wipes first, then fails to boot, leaving a Failed workflow holding the node (an outage with the data already gone). Verify the target's image supports the backend before a rocksdb migration; prefer `pebbledb` (no build tag) unless rocksdb is specifically required.

```sh
seictl workflow state-sync <node> --migration GigaStore --backend pebbledb --dry-run -n eng-<alias>
```

### Witnesses (`--rpc-servers`)

Optional. Sets the CometBFT light-client servers (a primary plus witnesses) for trust-point verification; bare `host:port`, repeatable, at least two or the plan refuses to compile. When omitted, the node's resolved state-syncers are used — the harbor norm. Snapshot chunks arrive separately over p2p from snapshot-serving peers, so a witness is not a snapshot provider.

### Never commit a workflow CR to the Flux workspace repo

Unlike `network`/`node`, a `SeiNodeTaskWorkflow` is a one-shot, spec-immutable request object. Under Flux ownership its recovery path breaks: a force-deleted Failed workflow is re-created from git on the next reconcile, and any edit to a committed workflow YAML is rejected by the spec CEL and wedges the whole Kustomization. Run it imperatively — the audit trail is the Complete CR left in-cluster plus the logged invocation.

### Re-run and recovery

- **Re-running a terminal workflow is refused by the CLI.** `seictl workflow state-sync` pre-flights the target on the watch path: if a same-named workflow is already `Complete` or `Failed`, it refuses with an actionable error rather than a silent no-op. (A *changed* spec is separately rejected by the CRD's CEL — params are immutable.)
- **A Failed workflow holds the node not-ready until it is removed**, so a mid-operation failure is a node outage, not just an unfinished task.
- **Recovery is force-delete first, always.** Remove the Failed workflow — annotate `sei.io/force-delete-workflow=<reason>`, then `seictl workflow delete <name>` — which releases the node; only then re-run (same name, or `--name` for a fresh one). A `--name` run *without* first removing the Failed workflow leaves the node still held by the first workflow and starts a second `reset-data` on it — a stacked double-wipe, not a recovery.

### Output and timeout

- **Success:** plan progress as NDJSON on stdout, one line per workflow event (the full CR); exit 0 when `.status.phase` reaches `Complete`. `--dry-run` / `--no-watch` stop after render / apply and emit the CR instead of watching. **Avoid `--no-watch` on this command** — with it the agent never observes a Failed phase, and a Failed workflow silently holds the node not-ready.
- **Failure:** `metav1.Status` on stderr, non-zero exit. Human diagnostic lines on stderr are prefixed `seictl:`; strip them before parsing:

```sh
seictl workflow state-sync <node> -n eng-<alias> 2>err.log; grep -v '^seictl:' err.log | jq -r .reason
```

  `.reason` is `Timeout` on `--timeout`, `InternalError` on a terminal `Failed` phase.
- **Timeout:** `--timeout` defaults to 60m. A full state-sync at real chain height (`pacific-1` / `atlantic-2`) can exceed that — set `--timeout` explicitly for the target's dataset. If the watch times out, **do not kill-and-retry**: the workflow is likely still legitimately Running. Check for it before creating another (only one should be in flight) with `seictl workflow list -n eng-<alias>` (or `kubectl get seinodetaskworkflows -n eng-<alias>`), then wait or force-delete — never stack a second wipe.

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
--genesis-override gov.voting_params.voting_period=60s
--genesis-override gov.deposit_params.min_deposit='[{"denom":"usei","amount":"100"}]'
```

Each entry writes a flat dotted-key into `spec.genesis.overrides`. The first segment is a cosmos module that exists in `app_state` (`staking`, `bank`, `gov`, `mint`, `slashing`, etc.). Values parse as JSON when they parse (numbers, bools, objects, arrays); otherwise as raw strings. To force a numeric-looking value to render as string, wrap in JSON quotes: `--genesis-override foo.bar='"42"'`.

Single-segment keys (`--genesis-override staking=...`) and empty values are rejected at apply time.

**Keys below the module are NOT schema-checked.** Only the module segment is validated at assemble time; a misspelled or nonexistent field under a real module (e.g. `gov.params.voting_period_seconds` — there is no `gov.params`) is silently written into the uploaded genesis. The ceremony reports success, then **every validator panics at InitChain** (`unknown field`) and the chain crash-loops at first start. Verify key path and value shape against a real genesis (`app_state.<module>...`) before overriding — decimals are JSON strings (`"0.4"`), and Sei's `mint` module has no inflation params (`params` = `mint_denom` + `token_release_schedule` only).

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
