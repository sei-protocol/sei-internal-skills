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
| `seictl report` | local | Analyze the comparison data of a shadow chain |
| `seictl network` | cluster | Manage `SeiNetwork` CRs via the `genesis-chain` preset |
| `seictl node` | cluster | Manage `SeiNode` CRs via the `rpc` preset |
| `seictl workflow` | cluster | Re-bootstrap or migrate an **existing** SeiNode via a `SeiNodeTaskWorkflow` |
| `seictl task` | cluster | Drive **one** node's sidecar task API directly (`/v0/tasks` through its in-pod kube-rbac-proxy) |

The skill invokes the `network` and `node` subtrees above. The `workflow` and `task` subtrees are **imperative**: they operate on an existing node rather than declaring a new one. They therefore sit outside the GitOps-PR bring-up flow (see `seictl workflow state-sync` and `seictl task` below). The two differ in who executes: `workflow` creates a CR the controller executes; `task` posts straight to one pod's sidecar, controller uninvolved. The `local` commands are out of scope for engineer-facing intents.

The pre-#133 cluster verbs (`context`, `onboard`, `bench up/down/list`) no longer exist; the preset-driven `network`/`node` trees below replace them. The single `nodedeployment` (alias `nd`) tree that preceded these is also gone: `SeiNodeDeployment` was a fleet Kind that split into `SeiNetwork` (the genesis validator pool) + standalone `SeiNode` CRs (each follower). If a reference to any of those older names surfaces in older docs, it is stale.

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
                     [--cpu <cores>] [--memory <quantity>] [--storage <quantity>]
                     [--iops <count> --throughput <MiB/s>]
                     [--node-isolation Shared|Dedicated]
                     [--genesis-account <addr>:<balance>] [--genesis-account ...]
                     [--genesis-override <module.field>=<value>] [--genesis-override ...]
                     [--config-value <file>.toml:<dotted.key>=<value>] [--config-value ...]
                     [--set <dotted.path>=<value>] [--set ...]
                     [--dry-run]
                     [-n <ns>] [--kubeconfig <path>]
```

Loads the `genesis-chain` preset, applies discrete-flag and `--set` overrides, and **server-side-applies** the result. With `--dry-run`, the apiserver validates and returns the would-be-applied CR without persisting.

**Layering, lowest precedence first:**

1. Preset YAML (embedded in the seictl binary).
2. Discrete flags (`--chain-id`, `--image`, `--replicas`, `--cpu`, `--memory`, `--storage`, `--iops`, `--throughput`, `--node-isolation`).
3. `--set <dotted.path>=<value>`. Strategic-merge: maps merge per-key, lists replace wholesale. Wins on collision with discrete flags. SeiNetwork config overrides live under `spec.configOverrides` (reach them via `--set`); there is **no `--override` flag** on `network apply`.

   Overrides take effect only on an **init path**, so set them at create time. An edit to a Running network's overrides never reaches its nodes' on-disk config. See `troubleshooting-seinode.md` → *configOverrides edits never reach a Running node*. For a value that must reach a running pool, use `--config-value` instead (layer 4).

4. `--config-value <file>.toml:<dotted.key>=<value>` (repeatable). Merges into the `spec.configValues` list **by `(fileName, key)`** after `--set`, so it can add to or replace entries the preset or a `--set` put there instead of replacing the list wholesale. See *Typed config values* below.

**Immutability (apply-time, load-bearing):** `spec.genesis`, `spec.replicas`, `spec.resources`, and `spec.dataVolume.storage` are all admission-immutable. The apiserver **rejects** a re-apply of `network apply <same-name>` that changes `--chain-id`, `--replicas`, `--cpu`, `--memory`, `--storage`, `--iops`, or `--throughput`, with `metav1.Status.reason=Invalid`. It is not a silent no-op. To change any of them, `delete` + re-create. This is the new-CRD analogue of the old `updateStrategy` trap.

The two resource one-way doors are create-only for different reasons, both load-bearing. The child StatefulSet uses `OnDelete` and pod-template drift detection observes only the seid image, sidecar image and node isolation, so a changed footprint never rolls onto a running pod. A Get-then-Create task creates the data PVC once and never updates it, so a changed size could never reach the volume. A resize is therefore a new chain: `delete`, pick a fresh chain-id, and re-create.

**Required:** `<name>` (positional) and `--preset genesis-chain`. `--chain-id` and `--image` must resolve after layering — if either is missing in the rendered CR, the apiserver rejects with `metav1.Status.reason=Invalid`.

**Output (success):** the post-apply `SeiNetwork` CR on stdout as JSON. Same shape as `kubectl get seinetwork <name> -o json`.

**Output (failure):** `metav1.Status` on stderr. Non-zero exit. Discriminate with `jq -r .reason` (e.g., `Invalid`, `Forbidden`, `NotFound`, `AlreadyExists`).

**Stderr provenance line (always):** `seictl: applying SeiNetwork <ns>/<name> to <api-server>` (or `applying (dry-run)`).

## `seictl node apply`

```
seictl node apply <name>
                  --preset rpc
                  [--chain-id <id>] [--image <ref>] --network <X>
                  [--cpu <cores>] [--memory <quantity>] [--storage <quantity>]
                  [--iops <count> --throughput <MiB/s>]
                  [--node-isolation Shared|Dedicated]
                  [--external-address <host>:<port>]
                  [--override <toml.key>=<value>] [--override ...]
                  [--config-value <file>.toml:<dotted.key>=<value>] [--config-value ...]
                  [--set <dotted.path>=<value>] [--set ...]
                  [--dry-run]
                  [-n <ns>] [--kubeconfig <path>]
```

Loads the `rpc` preset and server-side-applies a single `SeiNode`. An RPC fleet of N is N of these calls — `<name>` per follower (convention `<id>-rpc-<k>` for `k` in `0..N-1`).

**No `--replicas`.** SeiNode is a single node; there is no fan-out flag. The skill owns the N-CR loop (see `ephemeral-chain-flow.md`). Reaching for `--replicas` here is the most common migration mistake from the `nd` era.

**`--network <X>`** is the peer rail. It auto-wires `spec.peers[0].label.selector.sei.io/seinetwork=<X>` (who to peer with) AND stamps `metadata.labels{sei.io/seinetwork=<X>, sei.io/role=node}` on the CR — the producer side of the fleet list query (see `cluster-inspection-recipes.md` recipe #1). `--network` is independent of `--chain-id`: `--chain-id` sets the node's own `spec.chainId`; `--network` sets who it peers with. For an in-namespace ephemeral chain, pass the same value for both. A `node apply` with neither errors `Invalid`.

**`--external-address`** advertises a reachable host:port for external p2p. Leave unset for in-cluster ephemeral chains (followers peer over headless DNS).

**`--override <toml.key>=<value>`** targets `spec.overrides` (per-node `config.toml`/`app.toml`, applied at config-apply — an **init-path** task). Set overrides at create time: a re-apply against a Running node updates only the spec. The on-disk config never changes until the node next traverses an init path. See `troubleshooting-seinode.md` → *configOverrides edits never reach a Running node*. `--set` does strategic-merge on the whole spec and wins on collision.

**`--config-value <file>.toml:<dotted.key>=<value>`** targets `spec.configValues` — the typed, day-2 config surface. Same syntax and rules on both trees; see *Typed config values* below.

**`--cpu` / `--memory` / `--storage`** set the seid container footprint and the data-volume size. See *Resource footprint* below; the rules are identical on both `network apply` and `node apply`.

**`--iops` / `--throughput`** select the data volume's storage performance. See *Storage performance* below. The rules are identical on both `network apply` and `node apply`.

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

Issues a Delete against the named CR. Default propagation is `foreground` (waits for child resources before removing the CR itself). `background` returns immediately; `orphan` leaves children behind.

`--cascade` (the **client** propagation policy) is orthogonal to the CRD's own `spec.deletionPolicy`. A `SeiNetwork`'s `deletionPolicy` defaults to `Delete` (`Retain` makes the controller orphan its generated children; see `seinetwork-crd.md` → *Deletion*); `--cascade` governs how the client waits. Both apply.

**Output (success):** `seinetwork.sei.io/<name> deleted` / `seinode.sei.io/<name> deleted` on stdout. Exit 0.

**Failure:** `metav1.Status` on stderr (`reason=NotFound` if already gone).

## `seictl {network,node} watch`

```
seictl network watch <name> --until <phase> [--timeout <duration>] [-n <ns>]
seictl node    watch <name> --until <phase> [--timeout <duration>] [-n <ns>]
```

Streams every event for the named CR as one NDJSON line on stdout. **Exits 0** when `.status.phase == --until` (exact match). **Exits 1** on `--timeout` exceeded (`metav1.Status.reason=Timeout`), terminal `Failed` phase (stderr lifts `.status.plan.failedTaskDetail.error`), or a transient API failure.

**Two phase vocabularies (the terminal phases differ), plus one non-phase sentinel:**

- `seictl network watch --until=Ready` — `SeiNetworkPhase` reaches `Ready`. Common values: `Pending`, `Initializing`, `Ready`, `Degraded`, `Failed`, `Terminating`.
- `seictl node watch --until=Running` — `SeiNodePhase` reaches `Running`. Common values: `Pending`, `Initializing`, `Running`, `Failed`, `Terminating`. **A node has no `Ready`** — the parser rejects `node watch --until=Ready` with `Invalid`. An operator who waits for a node to reach `Ready` waits forever.
- `seictl node watch --until=caught-up` — the **one legal non-phase sentinel, nodes only**: waits for `Running`, then gates on the SDK serve-readiness check. That check: committed height>1 with `catching_up=false`, plus EVM serving when the node publishes an EVM endpoint. This is the post-state-sync and pre-load verification watch.

**Always pass `--until`.** Matching is exact against the phase set (plus the node-only `caught-up` sentinel); any other value errors `Invalid` at parse rather than timing out silently.

**Idiom for the agent:** `network apply` then `network watch --until=Ready` is the genesis 2-step. For an RPC fleet, loop `node apply` then `node watch --until=Running` per follower. Then assemble endpoints via `node list … -o json | jq`; the fleet is N CRs, so no single object holds the endpoints to read.

## `seictl workflow state-sync`

**This is the destructive paved road — the recipe-gated way to destroy a node's data.** It re-bootstraps an *existing* SeiNode by wiping its local chain state and re-syncing, with no undo — only "resync again." Read the decision rule and the destructive-op gate below before running it. Treat every invocation (standard resync or migration) as an `rm -rf` on that node's chain data. (It is not the *only* way to reach a wipe: a raw `seictl task submit reset-data` does it through one pod's sidecar with none of these protections — see `seictl task` below. Guardrail #9 gates both.)

`seictl workflow` is a third tree alongside `network`/`node`. It shares the CRUD verbs — `apply`, `get`, `list`, `delete` — and adds `state-sync`, the task-generating verb documented here. Unlike `network`/`node`, it is **imperative**: it renders a `SeiNodeTaskWorkflow` CR, server-side-applies it, and watches it to a terminal phase. `seictl workflow --help` is the source of truth for the tree, and the CLI wins on any disagreement; the shipped seictl `workflow/README.md` documents the fuller contract.

**Provenance for the controller-side claims in this section.** The claims cover recipe order, adoption exclusivity, target eligibility, and the force-delete gate. Verification ran on 2026-08-05 against `sei-k8s-controller` main @ `2d670ad` and `seictl` main @ `821f2f8` (v0.0.70). The prod cells and harbor track controller main closely, so treat this as describing the deployed behavior and not just the tip of the tree.

The asymmetry to keep in mind runs the other way. The **engineer's local seictl binary** is the thing that lags (hence gate 1's capability probes in `SKILL.md`). The controller-side semantics here are current.

`state-sync` is the convenience form of the one recipe this tree carries (`state-sync` is the only preset today). `seictl workflow apply --preset state-sync <same flags>` renders and applies the identical spec through the generic verb — it is not a second destructive path, and the gate below applies to it identically.

```
seictl workflow state-sync <node>
                 [--migration GigaStore --backend <pebbledb|rocksdb>]
                 [--rpc-servers <host:port>] [--rpc-servers ...]
                 [--name <workflow-name>]
                 [--dry-run] [--no-watch] [--timeout <duration>]
                 [-n <ns>] [--kubeconfig <path>]
```

**Required:** `<node>` (the target SeiNode's `metadata.name`). The workflow takes the name `<node>-state-sync` unless you pass `--name`. Streams plan progress as NDJSON on stdout until a terminal phase.

**`fullNode` targets only.** A SeiNode is exactly one mode (`fullNode` | `archive` | `replayer` | `validator` | `seed`, enforced by CRD CEL), and only `fullNode` is an eligible target. The workflow's own CEL cannot see the target's mode at admission, so the refusal lands at adoption. The workflow fails **terminally** with a message naming the mode (`ReasonWorkflowTargetRejected`). That is why `kubectl wait --for=condition=Failed` resolves instead of parking Pending forever.

A terminal refusal here never held the node and never wiped anything. The adoption check refuses seed nodes because they store no chain state to re-bootstrap.

**One workflow per node.** The node carries a single `status.adoptedWorkflow` pointer, and the controller consults it only when nil. While it holds a value, the controller considers no other workflow for that node. The controller seeds a workflow queued behind an actively-executing one as Pending (`ReasonWorkflowQueued`). One queued behind a **parked-Failed** workflow gets no status at all, because the controller never reaches the adoption path that would seed it.

A paused node or one mid-drift-plan (an image roll) defers adoption (`ReasonWorkflowTargetNotReady`) rather than racing it. This exclusivity is what makes the double-wipe scenario structurally impossible — see *Re-run and recovery*.

The recipe: hold the node's readiness gate, stop seid, **`reset-data` (wipes the data directory)**, configure state-sync, then release the readiness gate. The release is the terminal step, so **the workflow completes at release**. `Complete` means every mutation landed and the workflow released the node to re-bootstrap. seid restarts and catches up *after* Complete. Verify catch-up node-side with `seictl node watch <node> --until=caught-up` (the CLI prints this handoff on success). A migration inserts one extra step (config-patch) after `reset-data`; otherwise a standard resync and a migration run the same steps, both including the `reset-data` wipe.

### When to reach for it

**Key on node identity, not the word "state-sync."** Both this and the S3-snapshot create-time path get you a synced node without a full fresh sync. They are therefore easy to conflate, but they are different mechanisms at different lifecycle stages. The S3 path is a sidecar tarball restore baked into CR creation; this is CometBFT p2p state-sync on a live node:

- **New follower, nothing to preserve → the S3-snapshot create-time path** (`--set spec.fullNode.snapshot.s3.targetHeight=<h>` on `node apply`; see `ephemeral-chain-flow.md` → *Snapshot bootstrap*). The default for standing up a follower.
- **Existing node to re-bootstrap in place → `seictl workflow state-sync`.** Reach for it only when the node already exists and needs its state rebuilt (a store migration, or a follower whose local state is unrecoverable). **If a healthy node is serving, or a plain restart would fix it, you almost never want this command.** For an ephemeral follower that has merely fallen behind, `delete` + re-apply the SeiNode through the GitOps PR flow. That gets a fresh snapshot bootstrap that is non-destructive to shared state, in the audit trail, and human-reviewed. Prefer that over the imperative wipe whenever the node is disposable.

### Before you run it — the destructive-op gate

This tree has no PR-review gate. Give it back by hand, every time, on **both** the standard and the migration path (both wipe):

1. **Get explicit engineer sign-off before the side-effecting apply.** The agent does not volunteer this command and does not self-authorize the wipe. The same rule governs the direct-apply escape hatch in `SKILL.md` ("Escape hatch: direct `seictl network|node apply`"). State plainly what the wipe will destroy: node, namespace, standard-resync vs migration, and (for a migration) the backend.
2. **Verify the target is the intended node with a concrete read, not a self-confirm:** `seictl node get <node> -n eng-<alias> -o jsonpath='{.spec.chainId}{"  phase="}{.status.phase}{"\n"}'` — match chain-id, phase, and identity against intent. Pointing this at the wrong follower deletes its state.
3. **Never wipe a follower anyone else depends on** (a shared RPC endpoint, another engineer's load job, a dApp). Dependency is not observable from the cluster, so this is an escalation, not a check. On a long-lived `pacific-1` / `atlantic-2` follower or any shared node, escalate to the node's owner; do not wipe on agent initiative. Your own follower in your own `eng-<alias>` namespace is fair game; a shared one is not.
4. **`--dry-run` first** to render and inspect the CR without mutating anything, then apply:

```sh
# 1. render + inspect — no cluster mutation
seictl workflow state-sync <node> --dry-run -n eng-<alias>
# 2. after sign-off + target verification, apply and watch
seictl workflow state-sync <node> -n eng-<alias>
```

### Store migration (`--migration`) — irreversible, extra gate

`--migration GigaStore --backend <pebbledb|rocksdb>` sets the giga store flags on the way through the resync: state-store (`ss-enable`, `evm-ss-split`, `ss-backend`) and state-commit (`sc-enable`). Beyond the wipe every resync does, a migration **changes the storage engine and is not reversible without another resync**. Extra rules:

- **A migration needs both tokens** — a single flag cannot trigger one. **Omit both unless the task is explicitly a store migration**; `--migration` silently escalates a recoverable standard resync into an irreversible engine change.
- **`--backend rocksdb` needs a seid image built with `-tags rocksdbBackend`.** `reset-data` wipes *before* seid restarts on the new backend, and `--dry-run` does not check the running image's build tags. A wrong image therefore wipes first, then fails to boot, leaving a Failed workflow holding the node (an outage with the data already gone). Verify the target's image supports the backend before a rocksdb migration; prefer `pebbledb` (no build tag) unless rocksdb is specifically required.

```sh
seictl workflow state-sync <node> --migration GigaStore --backend pebbledb --dry-run -n eng-<alias>
```

### Witnesses (`--rpc-servers`)

Optional. Sets the CometBFT light-client servers (a primary plus witnesses) for trust-point verification; bare `host:port`, repeatable, at least two or the plan refuses to compile. When omitted, the plan uses the node's resolved state-syncers — the harbor norm. Snapshot chunks arrive separately over p2p from snapshot-serving peers, so a witness is not a snapshot provider.

### Never commit a workflow CR to the Flux workspace repo

Unlike `network`/`node`, a `SeiNodeTaskWorkflow` is a one-shot, spec-immutable request object. Under Flux ownership its recovery path breaks. Flux re-creates a force-deleted Failed workflow from git on the next reconcile. The spec CEL rejects any edit to a committed workflow YAML, and that wedges the whole Kustomization. Run it imperatively — the audit trail is the Complete CR left in-cluster plus the logged invocation.

### Re-run and recovery

- **The CLI refuses to re-run a terminal workflow.** `seictl workflow state-sync` pre-flights the target on the watch path. If a **same-named** workflow is already `Complete` or `Failed`, it refuses with an actionable error rather than a silent no-op. The pre-flight is name-scoped — a `--name` run skips it entirely, so it is not a backstop against a wrong-target re-run. (A *changed* spec is separately rejected by the CRD's CEL — params are immutable.)
- **A Failed workflow always holds the node not-ready until you remove it.** Release is the terminal step, so failure can only happen while the workflow holds the node. A mid-operation failure is therefore a node outage, not just an unfinished task.
- **Recovery is force-delete first, always.** Remove the Failed workflow — annotate `sei.io/force-delete-workflow=<reason>`, then `seictl workflow delete <name>` — which releases the node; only then re-run (same name, or `--name` for a fresh one).

  **The annotation is not optional today.** The controller's data-state verification is a stub that always reports unavailable (fail-closed by design). An un-annotated delete therefore parks the workflow `Terminating` with the node still held, and emits a `WorkflowDeleteHeld` warning event that names the annotation. Order does not matter. Annotating a workflow already stuck `Terminating` releases it on the next poll (≤30s), so a delete-first mistake is recoverable without touching finalizers by hand. (A `Complete` workflow needs no annotation — the next reconcile of the target reaps its finalizer.)
- **A `--name` run is not a recovery for a Failed workflow** — but it is not a second wipe either. Adoption is exclusive (one `status.adoptedWorkflow` pointer per node), so while the Failed workflow holds the node, the controller never adopts the fresh workflow. It compiles no plan, no `reset-data` runs, and the watch ends at `--timeout` (`reason=Timeout`) having changed nothing while the node stays held. The wasted 15m is the cost, not a stacked wipe. Remove the Failed workflow first, every time — that removal is what releases the node.

### Output and timeout

- **Success:** plan progress as NDJSON on stdout, one line per workflow event (the full CR); exit 0 when `.status.phase` reaches `Complete`. **`Complete` means the recipe's mutations landed and the workflow released the node — not that it caught up.** The resync runs after Complete. The CLI prints the verification handoff (`seictl node watch <node> --until=caught-up`) on success, and reporting a state-sync as done without that watch passing is premature. `--dry-run` / `--no-watch` stop after render / apply and emit the CR instead of watching. **Avoid `--no-watch` on this command** — with it the agent never observes a Failed phase, and a Failed workflow silently holds the node not-ready.
- **Failure:** `metav1.Status` on stderr, non-zero exit. Human diagnostic lines on stderr are prefixed `seictl:`; strip them before parsing:

```sh
seictl workflow state-sync <node> -n eng-<alias> 2>err.log; grep -v '^seictl:' err.log | jq -r .reason
```

  `.reason` reads `Timeout` on `--timeout`, `InternalError` on a terminal `Failed` phase.
- **Timeout:** `--timeout` defaults to 15m (60m on older binaries — the bound is the binary's; the watch semantics below are the controller's, regardless of binary). The watch ends when the workflow releases the node; catch-up happens after Complete and is not part of the watch. A timeout therefore **means a wedged recipe step in all but one case, not a slow sync**. On a timeout, **do not kill-and-retry**. Read the plan first (`seictl workflow list -n eng-<alias>`, then `.status.plan.tasks` on the one in-flight workflow) and map the verb to what you see.

  An archive-scale `reset-data` still clearing is the one legitimately slow case — raise `--timeout` and wait. Any other step parked past its budget has wedged — force-delete it. Launching a second workflow alongside it does not help and is not a shortcut. Adoption is exclusive, so the new one parks unadopted behind the held node and times out too.

  One version tell remains, and on the current fleet it should never fire. A plan whose **last** task is `await-condition` comes from a controller predating the release-terminal recipe. On that controller a long watch would be a slow catch-up rather than a wedge. harbor and the prod cells track controller main closely, so seeing that shape means you are on an unexpectedly stale cell. Establish which controller image the cell runs before acting on it, rather than settling in to wait.

## `seictl task`

The operator-facing surface over **one** node's sidecar task API (`/v0/tasks`). It reaches the sidecar directly through that pod's in-pod kube-rbac-proxy on **:8443** (not the sidecar's own :7777). Sibling of `seictl workflow`, and the distinction is load-bearing. `workflow` creates a CR the **controller** executes with its hold/ordering machinery; `task` posts to **one pod's sidecar** with the controller uninvolved.

Added by seictl #229 and **first shipped in `v0.0.66`**, above the skill's `v0.0.59` preflight floor. Confirm with `seictl task --help` before the first invocation in a session rather than assuming the binary has it. `seictl task --help` is the source of truth.

Every verb addresses a single pod. Target with `--node <name>` (the SeiNode / headless-service name; the sidecar resolves at `<node>-0.<node>.<ns>`) — required on every verb except `snapshot-upload`'s discovery path. Shared flags: `-n/--namespace`, `--port` (default `8443`), `--kubeconfig`. The verbs dial the pod, so run them from somewhere with cluster network reachability.

### `seictl task snapshot-upload` — the paved road

```
seictl task snapshot-upload [--node <name> | --chain <chain-id>]
                            [--timeout 2h15m] [--poll-interval 20s]
                            [-n <ns>] [--port 8443] [--kubeconfig <path>]
```

Submits one `snapshot-upload-once` with a fresh unique task ID and polls it to a terminal state. This is the procedure the per-(network, cluster) CronJob invokes daily. Reach for it when an engineer needs an on-demand snapshot publish, not as part of chain bring-up.

- **Target:** `--node` names one explicitly; `--chain` discovers a random pod labelled `sei.io/snapshot-publish=true,sei.io/chain=<chain>` (exact match). Mutually exclusive. When no pod carries the labels, discovery says so — fall back to `--node`.
- **Exit codes are kubectl-wait-compatible:** 0 when the task ends `uploaded` **or** `noop`. A `noop` is healthy (the chain has not advanced a snapshot interval; the verb prints which outcome it was). Nonzero on a failed task, or on `--timeout`, where the task **may still be running server-side** — `seictl task delete <id>` cancels it.
- **Defaults:** `--timeout` 2h15m, deliberately above the sidecar's own 2h upload deadline so the CLI bound never fires before the server's. `--poll-interval` 20s (sidecar-local, cheap).
- **Output:** the terminal `TaskResult` as JSON on stdout; progress and the verdict on stderr.
- **The fresh task ID is load-bearing.** The engine coalesces a reused ID onto an existing Completed row and never re-runs it — which is why the verb mints its own. Do not hand-craft a repeated ID via `submit` and expect a re-run.

### Raw verbs — thin wrappers over the sidecar client

| Verb | What it does |
|---|---|
| `seictl task get <id> --node <name>` | Read one task result |
| `seictl task list --node <name>` | List recent task results (the node's task history) |
| `seictl task submit <type> --node <name> [--params '<json>']` | POST an arbitrary task; params validated server-side |
| `seictl task delete <id> --node <name>` | Delete a task result, or **cancel** it if still running |

`list` and `get` are the read side and are safe. They are a useful diagnosis path when a workflow step parks: read the node's own task history rather than inferring from `.status.plan.tasks` alone.

**`submit` is a genuine escape hatch — treat it like the direct-apply escape hatch, not like a read.** It POSTs any task type the sidecar's wire protocol accepts, and that set includes destructive ones (`reset-data` among them). Submitted this way a task runs **without** the workflow recipe around it. That means no `mark-not-ready` hold, no `stop-seid` first, no ordering guarantee, and no adoption pointer telling the controller a node is busy. Prefer `seictl workflow state-sync` for anything the recipe already covers. Get explicit engineer sign-off (naming node, namespace, and task type) before submitting a mutating task by hand.

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

## Typed config values (`--config-value`)

`spec.configValues` is a list of `{fileName, key, value}` entries the controller overlays on a node's generated `config.toml` / `app.toml` (SeiNetwork and SeiNode alike). It is the **day-2** config surface: an edit reaches a Running node. `spec.overrides` / `spec.configOverrides` are init-path only (see *configOverrides edits never reach a Running node* in `troubleshooting-seinode.md`); prefer `--config-value` for anything that may need to change after first boot.

**The two surfaces take keys in different vocabularies — translate, never copy.** A `spec.overrides` key is a *unified sei-config schema* path (`storage.state_commit.write_mode`, `network.rpc.pprof_listen_address`); config-apply silently rejects anything else. A `--config-value` key is the *raw TOML path inside the named file* (`app.toml:state-commit.sc-write-mode`, `config.toml:rpc.pprof_laddr`), exactly as it appears in `/sei/config/<file>`. Carrying a unified key into `--config-value` writes a key seid does not know, and the failure surfaces late — at `config-validate`, or as a silently ignored table. Worked pair for pprof: override `spec.overrides."network.rpc.pprof_listen_address"="0.0.0.0:6060"` ⇔ config value `--config-value config.toml:rpc.pprof_laddr=0.0.0.0:6060`. Confirm a raw key by reading the rendered file on a running pod before you write it.

**Minimum version.** `--config-value` arrives with seictl#253, which post-dates the v0.0.72 floor the rest of this skill assumes; no tag carried it at the time of writing. Pre-flight Gate 1 check 5 probes `seictl node apply --help` for `--config-value`; an older binary fails loud at parse (`flag provided but not defined: -config-value`). Upgrade (`go install ...@latest` once a tag ships, else `@main`); do not fall back to `--set spec.configOverrides`, which lands at first boot only.

```
--config-value <file>.toml:<dotted.key>=<value>      # repeatable
```

**Typing.** The value is parsed as JSON first, so `true`, `400`, `1.5`, `["a","b"]`, `{"x":1}` and `"quoted"` keep their type; a value that is not valid JSON (`async`, `0.0.0.0:8545`, `100ms`) is stored as a string. Integral numbers stay exact `int64` — large chain IDs and gas limits do not lose precision. The CR carries the typed value (`value: 400`, not `value: "400"`), and seid receives a typed TOML key. Quote a numeric string that must stay a string (`--config-value app.toml:evm.some_id='"713715"'`).

**Rejected at render** — fix these before the PR, they never reach the cluster: empty value; `null` (top-level or nested in an array/object — the controller refuses both); `fileName` not matching `^[A-Za-z0-9_-]+\.toml$` (≤64 chars — so `autobahn.json` is not expressible here); `key` not matching dotted `^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$` (≤256 chars); more than **100** entries; a spec whose *pre-existing* list (preset or `--set spec.configValues=[...]`) already names one `(fileName, key)` twice — seictl refuses to build on a list the controller would reject. Repeating a `--config-value` for an identity that is already in the list is **not** a rejection; it replaces the entry (next paragraph).

**Merge by identity.** `--config-value` merges into the existing list by `(fileName, key)`: an existing entry with the same identity is replaced in place, new ones append in flag order. This runs **after** `--set`, so a preset's or `--set`'s entries survive; `--set spec.configValues=[...]` alone replaces the list wholesale.

**Precedence on disk.** base config generated by the controller → controller-owned `[p2p]` peer keys → `configValues` overlay → seid validation. `configValues` wins over the base and over legacy overrides for the same key; the controller re-patches only peer keys, so it never overwrites a configValue.

**Network vs node.** On a SeiNetwork the set is **authoritative for every validator child** — the controller pre-validates it (`ConfigValuesValid` condition on the network; `InvalidConfigValues` event on failure, children keep their last good set) and then copies it into each child `SeiNode.spec.configValues`. Editing a child directly is overwritten on the next reconcile; edit the network. A standalone SeiNode (RPC/follower) owns its own list.

**Restart semantics — read before editing a live bench.** A configValues change on a Running node builds a `config-update` plan: regenerate base → patch peers → overlay → validate → restart seid. On a SeiNetwork every validator restarts **at the same time** (no rolling order), so block production stops until >2/3 are back. Never change network configValues during a measurement window; provision the bench with the values from the start, or re-apply between runs and wait for `Ready`. The plan does not touch the StatefulSet or image; a `--config-value` re-apply alone never rolls pods.

**Base-regeneration side effect.** Because the plan regenerates the base file, keys that were set out-of-band — `[statesync]` trust height/hash from a state-sync workflow, giga migration keys written by a migration task — revert to defaults unless they are also expressed as configValues. If a follower was state-synced or migrated, add those keys as `--config-value` entries before any day-2 config edit.

**Verify on the cluster, not in the spec.**

```bash
kubectl get seinetwork <id> -o jsonpath='{.status.conditions[?(@.type=="ConfigValuesValid")]}'
kubectl get seinode <name> -o jsonpath='{.status.currentConfigValuesHash}{"\n"}{.status.plan.tasks[*].type}'
kubectl exec <pod> -c seid -- grep -A2 '^\[evm\]' /sei/config/app.toml
```

An empty `currentConfigValuesHash` on a Running node means it predates the feature: edits wait for the next image roll (`NodeUpdateInProgress` reason `ConfigBaselineUnobserved`). Triage in `troubleshooting-seinode.md` → *ConfigValuesValid=False / configValues edit produced no restart*.

## Node isolation (`--node-isolation`)

Present on both `network apply` and `node apply`, identical semantics. Writes `spec.scheduling.nodeIsolation`; on a SeiNetwork the controller copies the block into every validator child.

```
--node-isolation Shared      # may share a worker node with other Sei pods
--node-isolation Dedicated   # one Sei pod per worker node
```

Input is case-insensitive (`dedicated`, `DEDICATED`) and renders canonical (`Dedicated`). Any other token fails at render before the apiserver is reached. `--set spec.scheduling.nodeIsolation=...` wins over the flag on collision.

**Omitted means unset, not `Shared`.** The field has no schema default. An unset field resolves in the controller as: legacy `sei.io/dedicated-node: "true"` annotation on the SeiNode → `Dedicated`; otherwise `Shared`. The field supersedes the annotation — write the field, never the annotation, on a new CR.

**What `Dedicated` does.** The pod gets a required pod anti-affinity against every Sei-managed pod (`sei.io/node` exists) across all namespaces, and every Sei pod carries a matching term that repels dedicated pods. Two consequences: a Dedicated pod schedules only onto a worker node hosting no Sei pod at all, and no later Sei pod lands next to it. When the cell's controller config names a single-tenant Karpenter NodePool for the mode, the pod also selects that pool; harbor's controller config names none today (PLT-1227), so on harbor `Dedicated` is anti-affinity only and consumes a whole node from the shared pool.

**Capacity, not correctness, is the failure mode.** Admission accepts `Dedicated` regardless of cluster capacity. The pod then sits `Pending` until a worker node with no Sei pod exists, and the SeiNetwork reports the child as `placement: Pending` with an empty `workerNode`. That is a capacity ask to the platform team, not a retry, and not a reason to flip to `Shared` silently. Check with `cluster-inspection-recipes.md` recipe #9.

**Mutable, and a change rolls the pod.** `nodeIsolation` is not admission-immutable. Once the controller has observed a node's isolation (`status.currentNodeIsolation` set), a change between the effective desired value and the rolled value builds a node-update plan that replaces the pod (StatefulSets use `OnDelete`; nothing rolls on its own). On a SeiNetwork that means every validator pod restarts — same blast radius as a `configValues` edit. Set it at create time for a bench; never flip it inside a measurement window. An empty `status.currentNodeIsolation` means not yet observed and never triggers a roll.

**Server-side apply drops the field on a re-apply that omits the flag.** `apply` runs with force-ownership; a `network apply <same-name>` without `--node-isolation` renders the field unset, the apply removes it, and the controller resolves `Shared` and rolls the pool back to shared placement. Repeat `--node-isolation Dedicated` on every re-apply of a Dedicated CR. In the PR flow this is moot — the committed YAML carries the field — but the escape hatch and any `--dry-run` re-render must carry it.

**Verify:**

```sh
# The rendered CR carries it
seictl network apply <id> ... --node-isolation Dedicated --dry-run -n eng-<alias> | jq -r .spec.scheduling.nodeIsolation

# Placement per validator once Ready (also recipe #9)
kubectl get seinetwork <id> -n eng-<alias> \
  -o jsonpath='{range .status.nodes[*]}{.name}{"\t"}{.placement}{"\t"}{.workerNode}{"\n"}{end}'

# What the controller has rolled, per node
kubectl get seinode -n eng-<alias> -l sei.io/seinetwork=<id> \
  -o custom-columns='NAME:.metadata.name,WANT:.spec.scheduling.nodeIsolation,ROLLED:.status.currentNodeIsolation'
```

## Resource footprint (`--cpu` / `--memory` / `--storage`)

Present on both `network apply` and `node apply`, with identical semantics. Each flag overrides exactly one dimension of the preset footprint, resolved independently: `--cpu 8` alone leaves memory and storage on their preset values.

| Flag | CR path | Preset default | Accepts |
|---|---|---|---|
| `--cpu` | `spec.resources.requests.cpu` | `4` | bare cores (`4`) or millicores (`500m`) |
| `--memory` | `spec.resources.requests.memory` | `32Gi` | any Kubernetes quantity (`32Gi`) |
| `--storage` | `spec.dataVolume.storage.resources.requests.storage` | `500Gi` | any Kubernetes quantity (`500Gi`) |

The default footprint is roughly a quarter of the mainnet validator shape (16 CPU / 128Gi / 2000Gi). That mainnet shape is the controller's per-mode default, and far too large for a dev chain.

**seictl validates the quantities locally, before it renders.** A bad spelling (`32GB`) or a non-positive value (`0`, `-1`) exits non-zero naming the flag. This matters because the render output goes into a git PR. Without the local check, an invalid quantity reaches the apiserver only after merge. The engineer then reads the failure out of a Flux reconcile instead of the terminal.

**Neither the presets nor the three flags emit `resources.limits`.** The CRD's CEL accepts only `memory` under `limits` — seid deliberately carries no CPU limit — and requires `limits.memory` to equal `requests.memory`, which the controller derives from the request. `seictl` refuses to render a CR carrying `spec.resources.limits.cpu` from any source, including `--set`. seictl passes a `--set` memory limit through, and the apiserver enforces the equality rule.

**Storage size lives only at `spec.dataVolume.storage`, never under `spec.resources`.** DR-001 separated the two: `spec.resources.requests` accepts only `cpu` and `memory`, and `spec.dataVolume.storage.resources.requests` accepts only `storage`. Note the nested volume-claim shape of the storage path — `spec.dataVolume.storage` is an object, not a quantity. It carries `resources` for the size and `volumeAttributesClassName` for the performance selection, and CEL requires `resources.requests.storage` whenever a CR populates `resources`. A bare quantity at `spec.dataVolume.storage` fails schema validation.

**Minimum version: v0.0.72.** That tag is the first to carry seictl#248, so `go install ...@latest` clears the gate. Older binaries reject the three flags at parse, and their presets carry no resource block at all. Gate 1 in `preflight.md` probes `node apply --help` for `--cpu`.

`--iops` and `--throughput` merged after the resource flags, but both landed in the same v0.0.72 tag. Only a build taken from `main` between those two merges carries one set without the other. Gate 1 probes for them separately anyway. See *Storage performance* below.

**Fleet cost is per node, and the size is create-only.** Both presets carry the same 500Gi. A 4-validator chain with a 4-follower fleet therefore provisions about 4Ti of EBS that no later edit can shrink. An EVM-serving follower also inherits the consensus-validator shape, which may not suit it. The default is still the default — pass `--storage 500Gi` on the follower loop unless the engineer asks for something else. Raise the fleet total with them when N is large, since correcting an oversized volume means deleting the node and losing its data.

**Provenance for the controller-side claims in this section.** Those claims are:

- the CEL limits and immutability rules
- the `OnDelete` StatefulSets and their drift set (seid image, sidecar image, node isolation)
- the Get-then-Create ensure-data-pvc task
- the per-mode 16 CPU / 128Gi default

Verified against `sei-k8s-controller` main @ `7da9946` on 2026-09-10. The sources read were `api/v1alpha1/seinode_types.go`, `api/v1alpha1/seinetwork_types.go`, and the generated CRDs under `config/crd/`. A reader cannot check these from the CLI alone. If one ever looks wrong, re-verify against the controller rather than against `seictl --help`.

## Storage performance (`--iops` / `--throughput`)

Present on both `network apply` and `node apply`, with identical semantics. The two flags are one selection, so pass both or neither.

**You supply the pair; seictl resolves the name.** The platform owns a catalog of VolumeAttributesClass objects, and each one encodes a supported (IOPS, throughput) pair. You give the parameters you want. seictl finds the class that carries them and writes its name to `spec.dataVolume.storage.volumeAttributesClassName`. No flag accepts a class name, and the direction never runs the other way.

The supported set is two entries:

| Selection | Resolves to | Data volume |
|---|---|---|
| omit both flags | *standard* — no `volumeAttributesClassName`, so the gp3 StorageClass defaults apply | any size |
| `--iops 10000 --throughput 750` | `sei-gp3-performance-v1` | at least 20Gi |

Omitting both flags is a real selection, not a gap. The PVC then carries no `volumeAttributesClassName` at all.

The catalog holds no archive tier, and this skill needs none. seictl ships two presets, `genesis-chain` and `rpc`, so every node it renders is a validator or a fullNode on a dev chain. Both default to the standard tier, and a performance selection is an explicit opt-in for a storage-bound bench. Archive nodes fall outside harbor-dev's dev-only scope.

**The `-v1` suffix is load-bearing.** VolumeAttributesClass `parameters` are immutable, so the platform retunes a tier by creating a new object (`sei-gp3-performance-v2`), never by editing this one. Do not strip the suffix or treat it as noise.

**An unsupported pair fails at the flag, and the message names the whole supported set.** Each offering appears with the class name it resolves to, next to the standard tier. seictl refuses `--iops` without `--throughput` for the same reason: half a pair would make seictl invent the other half.

**The 10000-IOPS tier needs a data volume of at least 20Gi.** EBS gp3 caps IOPS at 500 times the volume size in GiB. seictl enforces that floor locally, against the size the render actually produced. The procedure passes `--storage` on every render, so that is the value the engineer chose rather than the 500Gi preset default.

That local check matters more than it looks. CEL cannot see this rule. The size and the class name are two independent fields, and the ratio belongs to AWS rather than to the schema. The apiserver therefore accepts `--storage 10Gi` next to the performance tier. It then creates the PVC, provisioning fails, and the pod sits `Pending` on `ProvisioningFailed`. Both fields are create-only, so the remedy at that point is a new chain.

**`--set` cannot reach around either guard.** seictl re-reads `spec.dataVolume.storage` after every layer. A `--set` of the class name or of the size therefore meets the same two checks the flags meet. This mirrors the `spec.resources.limits.cpu` guard.

**Create-only on both Kinds.** The name binds when the controller provisions the data PVC. On any re-apply, admission rejects a first-time set, a change, and an unset alike — the value is settable only at creation. Comparing two performance tiers therefore means two chains, not one chain edited between runs — see `comparative-bench.md`.

**Provenance, and what is not merged yet.** The catalog is the one VolumeAttributesClass the platform ships: `sei-gp3-performance-v1`, `driverName: ebs.csi.aws.com`, `iops: "10000"`, `throughput: "750"` (platform `clusters/base/default/volume-attributes-class.yaml`, PR 1661, merged). The CRD field `spec.dataVolume.storage.volumeAttributesClassName` and its create-only CEL come from `sei-k8s-controller` PR 533. Reviewers approved that PR, but it is **not merged**. Requirement 3 of Spec 001 and DR-001 fix the direction the pair resolves in.

DR-001 lines 101-106 split the ownership: the platform owns the catalog, the harness owns the menu. Re-verify against the controller if the field path ever looks wrong.

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
  resources:
    requests:
      cpu: "4"
      memory: 32Gi
  dataVolume:
    storage:
      resources:
        requests:
          storage: 500Gi
  configOverrides:
    network.rpc.pprof_listen_address: "0.0.0.0:6060"
```

Layered with `--chain-id` and `--image`, the `genesis` block populates and `spec.image` / `spec.genesis.chainId` get set. The controller stamps `sei.io/seinetwork=<id>` and `sei.io/role=validator` on the generated validator SeiNodes.

**Default replicas: 4.** Override with `--replicas` or `--set spec.replicas=N` **at create time only**. `replicas` is immutable once the network exists (a network minted at 4 cannot be re-applied at 1; `delete` + re-create).

**Genesis params** (`--genesis-override <module.field[.field...]>=<value>`, repeatable):

```
--genesis-override staking.params.unbonding_time=600s
--genesis-override bank.params.default_send_enabled=true
--genesis-override gov.voting_params.voting_period=120s
--genesis-override gov.voting_params.expedited_voting_period=60s
--genesis-override gov.deposit_params.min_deposit='[{"denom":"usei","amount":"100"}]'
```

Each entry writes a flat dotted-key into `spec.genesis.overrides`. The first segment is a cosmos module that exists in `app_state` (`staking`, `bank`, `gov`, `mint`, `slashing`, etc.). Values parse as JSON when they parse (numbers, bools, objects, arrays); otherwise as raw strings. Durations like `120s` fail JSON parse and land as the string `"120s"`, which is exactly the proto-JSON duration encoding genesis expects. To force a numeric-looking value to render as string, wrap in JSON quotes: `--genesis-override foo.bar='"42"'`.

Apply rejects single-segment keys (`--genesis-override staking=...`) and empty values.

**The sharp edge — validation covers only the module segment, not the rest of the key.** The genesis assembler checks that the first segment names an existing `app_state` module, then creates any missing deeper path segments and writes the value verbatim, unvalidated. A wrong field name therefore passes `--dry-run`, passes apply, passes genesis assembly. Then **every node crash-loops at InitChain** with an "unknown field" panic, and the chain never starts. The intended change never applies.

Two upstream-Cosmos-shaped keys that do NOT exist on sei have caused exactly this. `gov.params.voting_period_seconds`: sei's gov genesis nests under `voting_params`/`deposit_params`/`tally_params`, with no `params` object. `mint.params.inflation`: sei's mint module is custom — `mint_denom` + `token_release_schedule` only; no key expresses inflation-rate semantics.

**Key provenance rule:** never guess a key from upstream Cosmos docs — sei's forks diverge. Take keys from a real genesis: `app_state.<module>` in `curl <any-node>:26657/genesis` on a running chain, or the embedded chains in sei-config. Value shapes matter too: durations and decimals are JSON strings (`"60s"`, `"0.4"`). Production precedent for fast governance: the sei-k8s-controller nightly upgrade suite pins `gov.voting_params.voting_period: "60s"` (`test/integration/upgrade_test.go`).

**Verify after Ready** (cheap, do it): from a validator pod, `seid q gov params voting` (or read `/genesis` on 26657) confirms the override landed. A throwaway proposal submitted with the full min-deposit proves the voting window behaviorally.

**Not reachable via this flag:** `consensus_params.*` (CometBFT consensus params, sibling to `app_state` in `genesis.json`, not under any cosmos module). `block.max_gas`, `validator.pub_key_types`, etc. are not currently reachable through `spec.genesis.overrides`.

Distinct from `--set spec.configOverrides...`, which targets per-node `config.toml`/`app.toml` applied at config-apply time.

**Funded accounts at genesis** (`--genesis-account <address>:<balance>`, repeatable):

```
--genesis-account sei1abc...:1000000000000usei
--genesis-account 0xDEAD...:1000000000000000000000usei,500uatom
```

Address accepts bech32 (`sei1...`) or 0x-hex. Balance accepts the standard Cosmos coin format — one or more `<int><denom>` entries, comma-separated. Appends entries to `spec.genesis.accounts`.

### `rpc` (→ `seictl node`)

A full-node follower that peers to an existing network by label selector. One CR per follower; an RPC fleet of N is N `node apply` calls.

```yaml
apiVersion: sei.io/v1alpha1
kind: SeiNode
spec:
  fullNode: {}
  resources:
    requests:
      cpu: "4"
      memory: 32Gi
  dataVolume:
    storage:
      resources:
        requests:
          storage: 500Gi
  overrides:
    network.rpc.pprof_listen_address: "0.0.0.0:6060"
```

When you pass `--network <X>`, the renderer auto-wires:

- Object labels: `sei.io/seinetwork=<X>`, `sei.io/role=node` (what `node list -l …` matches).
- `spec.peers[0].label.selector.sei.io/seinetwork=<X>` — points the follower at every node in the namespace tagged with that network.

**The auto-wire is what makes "chain + RPC fleet on the same network" a one-shot.** Pass `--network <id>` (the genesis network's id) to each follower; no hand-rolled `--set spec.peers...` payload.

`seictl network|node apply --preset` accepts only `genesis-chain` (network) or `rpc` (node). If an engineer asks for any other preset (archive, single validator, fork-test), `apply` cannot serve it. Surface that and ask whether they want to hand-roll the CR YAML instead.
