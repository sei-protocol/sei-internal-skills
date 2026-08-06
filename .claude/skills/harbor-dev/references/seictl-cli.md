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
| `seictl task` | cluster | Drive **one** node's sidecar task API directly (`/v0/tasks` through its in-pod kube-rbac-proxy) |

The skill invokes the `network` and `node` subtrees above. The `workflow` and `task` subtrees are **imperative** — they operate on an existing node rather than declaring a new one, so they sit outside the GitOps-PR bring-up flow (see `seictl workflow state-sync` and `seictl task` below). The two differ in who executes: `workflow` creates a CR the controller executes; `task` posts straight to one pod's sidecar, controller uninvolved. The `local` commands are out of scope for engineer-facing intents.

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

**Two phase vocabularies (the terminal phases differ), plus one non-phase sentinel:**

- `seictl network watch --until=Ready` — `SeiNetworkPhase` reaches `Ready`. Common values: `Pending`, `Initializing`, `Ready`, `Degraded`, `Failed`, `Terminating`.
- `seictl node watch --until=Running` — `SeiNodePhase` reaches `Running`. Common values: `Pending`, `Initializing`, `Running`, `Failed`, `Terminating`. **There is no `Ready` on a node** — `node watch --until=Ready` is rejected at parse with `Invalid`. An operator who waits for a node to reach `Ready` waits forever.
- `seictl node watch --until=caught-up` — the **one legal non-phase sentinel, nodes only**: waits for `Running`, then gates on the SDK serve-readiness check (committed height>1 with `catching_up=false`, plus EVM serving when the node publishes an EVM endpoint). This is the post-state-sync and pre-load verification watch.

The `--until` flag is **required**. Matching is exact against the phase set (plus the node-only `caught-up` sentinel); any other value errors `Invalid` at parse rather than timing out silently.

**Idiom for the agent:** `network apply` then `network watch --until=Ready` is the genesis 2-step; for an RPC fleet, loop `node apply` then `node watch --until=Running` per follower, then assemble endpoints via `node list … -o json | jq` (the fleet is N CRs — there is no single object to read endpoints from).

## `seictl workflow state-sync`

**This is the destructive paved road — the recipe-gated way to destroy a node's data.** It re-bootstraps an *existing* SeiNode by wiping its local chain state and re-syncing, with no undo — only "resync again." Read the decision rule and the destructive-op gate below before running it, and treat every invocation (standard resync or migration) as an `rm -rf` on that node's chain data. (It is not the *only* way to reach a wipe: a raw `seictl task submit reset-data` does it through one pod's sidecar with none of these protections — see `seictl task` below. Guardrail #9 gates both.)

`seictl workflow` is a third tree alongside `network`/`node`. It shares the CRUD verbs — `apply`, `get`, `list`, `delete` — and adds `state-sync`, the task-generating verb documented here. Unlike `network`/`node`, it is **imperative**: it renders a `SeiNodeTaskWorkflow` CR, server-side-applies it, and watches it to a terminal phase. `seictl workflow --help` is the source of truth for the tree, and the CLI wins on any disagreement; the shipped seictl `workflow/README.md` documents the fuller contract.

**Provenance for the controller-side claims in this section** (recipe order, adoption exclusivity, target eligibility, the force-delete gate): verified against `sei-k8s-controller` main @ `2d670ad` and `seictl` main @ `821f2f8` (v0.0.70) on 2026-08-05. harbor and the prod cells track controller main closely, so treat this as describing the deployed behavior and not just the tip of the tree. The asymmetry to keep in mind runs the other way: the **engineer's local seictl binary** is the thing that lags (hence the version floors in `SKILL.md` gate 1), while the controller-side semantics here are current.

`state-sync` is the convenience form of the one recipe this tree carries (`state-sync` is the only preset today). `seictl workflow apply --preset state-sync <same flags>` renders and applies the identical spec through the generic verb — it is not a second destructive path, and the gate below applies to it identically.

```
seictl workflow state-sync <node>
                 [--migration GigaStore --backend <pebbledb|rocksdb>]
                 [--rpc-servers <host:port>] [--rpc-servers ...]
                 [--name <workflow-name>]
                 [--dry-run] [--no-watch] [--timeout <duration>]
                 [-n <ns>] [--kubeconfig <path>]
```

**Required:** `<node>` (the target SeiNode's `metadata.name`). The workflow is named `<node>-state-sync` unless `--name` is given. Streams plan progress as NDJSON on stdout until a terminal phase.

**`fullNode` targets only.** A SeiNode is exactly one mode (`fullNode` | `archive` | `replayer` | `validator` | `seed`, enforced by CRD CEL), and only `fullNode` is an eligible target. The workflow's own CEL can't see the target's mode at admission, so the refusal lands at adoption: the workflow fails **terminally** with a message naming the mode (`ReasonWorkflowTargetRejected`), which is why `kubectl wait --for=condition=Failed` resolves instead of parking Pending forever. A terminal refusal here never held the node and never wiped anything. Seed nodes are refused because they store no chain state to re-bootstrap.

**One workflow per node.** The node carries a single `status.adoptedWorkflow` pointer, and it is only ever consulted when nil — so while it is set, no other workflow for that node is even considered. A workflow queued behind an actively-executing one is seeded Pending (`ReasonWorkflowQueued`); one queued behind a **parked-Failed** workflow gets no status at all, because the adoption path it would be seeded from is never reached. A paused node or one mid-drift-plan (an image roll) defers adoption (`ReasonWorkflowTargetNotReady`) rather than racing it. This exclusivity is what makes the double-wipe scenario structurally impossible — see *Re-run and recovery*.

The recipe: hold the node's readiness gate, stop seid, **`reset-data` (wipes the data directory)**, configure state-sync, then release the readiness gate — the release is the terminal step, so **the workflow completes at release**. `Complete` means every mutation landed and the node was released to re-bootstrap; seid restarts and catches up *after* Complete, and catch-up is verified node-side with `seictl node watch <node> --until=caught-up` (the CLI prints this handoff on success). A migration inserts one extra step (config-patch) after `reset-data`; otherwise a standard resync and a migration run the same steps, both including the `reset-data` wipe.

### When to reach for it

**Key on node identity, not the word "state-sync."** Both this and the S3-snapshot create-time path get you a synced node without a full fresh sync, so they are easy to conflate — but they are different mechanisms at different lifecycle stages (the S3 path is a sidecar tarball restore baked into CR creation; this is CometBFT p2p state-sync on a live node):

- **New follower, nothing to preserve → the S3-snapshot create-time path** (`--set spec.fullNode.snapshot.s3.targetHeight=<h>` on `node apply`; see `ephemeral-chain-flow.md` → *Snapshot bootstrap*). The default for standing up a follower.
- **Existing node to re-bootstrap in place → `seictl workflow state-sync`.** Reach for it only when the node already exists and needs its state rebuilt (a store migration, or a follower whose local state is unrecoverable). **If a healthy node is serving, or a plain restart would fix it, you almost never want this command.** For an ephemeral follower that has merely fallen behind, `delete` + re-apply the SeiNode through the GitOps PR flow gets a fresh snapshot bootstrap that is non-destructive to shared state, in the audit trail, and human-reviewed — prefer that over the imperative wipe whenever the node is disposable.

### Before you run it — the destructive-op gate

This tree has no PR-review gate. Give it back by hand, every time, on **both** the standard and the migration path (both wipe):

1. **Get explicit engineer sign-off before the side-effecting apply.** The agent does not volunteer this command and does not self-authorize the wipe — the same rule as the direct-apply escape hatch in `SKILL.md` ("Escape hatch: direct `seictl network|node apply`"). State plainly what will be wiped: node, namespace, standard-resync vs migration, and (for a migration) the backend.
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

- **Both tokens are required** — a migration cannot be triggered by a single flag. **Omit both unless the task is explicitly a store migration**; `--migration` silently escalates a recoverable standard resync into an irreversible engine change.
- **`--backend rocksdb` needs a seid image built with `-tags rocksdbBackend`.** `reset-data` wipes *before* seid restarts on the new backend, and `--dry-run` does not check the running image's build tags — so a wrong image wipes first, then fails to boot, leaving a Failed workflow holding the node (an outage with the data already gone). Verify the target's image supports the backend before a rocksdb migration; prefer `pebbledb` (no build tag) unless rocksdb is specifically required.

```sh
seictl workflow state-sync <node> --migration GigaStore --backend pebbledb --dry-run -n eng-<alias>
```

### Witnesses (`--rpc-servers`)

Optional. Sets the CometBFT light-client servers (a primary plus witnesses) for trust-point verification; bare `host:port`, repeatable, at least two or the plan refuses to compile. When omitted, the node's resolved state-syncers are used — the harbor norm. Snapshot chunks arrive separately over p2p from snapshot-serving peers, so a witness is not a snapshot provider.

### Never commit a workflow CR to the Flux workspace repo

Unlike `network`/`node`, a `SeiNodeTaskWorkflow` is a one-shot, spec-immutable request object. Under Flux ownership its recovery path breaks: a force-deleted Failed workflow is re-created from git on the next reconcile, and any edit to a committed workflow YAML is rejected by the spec CEL and wedges the whole Kustomization. Run it imperatively — the audit trail is the Complete CR left in-cluster plus the logged invocation.

### Re-run and recovery

- **Re-running a terminal workflow is refused by the CLI.** `seictl workflow state-sync` pre-flights the target on the watch path: if a **same-named** workflow is already `Complete` or `Failed`, it refuses with an actionable error rather than a silent no-op. The pre-flight is name-scoped — a `--name` run skips it entirely, so it is not a backstop against a wrong-target re-run. (A *changed* spec is separately rejected by the CRD's CEL — params are immutable.)
- **A Failed workflow always holds the node not-ready until it is removed** (release is the terminal step, so failure can only happen while the node is held), so a mid-operation failure is a node outage, not just an unfinished task.
- **Recovery is force-delete first, always.** Remove the Failed workflow — annotate `sei.io/force-delete-workflow=<reason>`, then `seictl workflow delete <name>` — which releases the node; only then re-run (same name, or `--name` for a fresh one). **The annotation is not optional today:** the controller's data-state verification is a stub that always reports unavailable (fail-closed by design), so an un-annotated delete parks the workflow `Terminating` with the node still held, emitting a `WorkflowDeleteHeld` warning event that names the annotation. Order doesn't matter — annotating a workflow already stuck `Terminating` releases it on the next poll (≤30s), so a delete-first mistake is recoverable without touching finalizers by hand. (A `Complete` workflow needs no annotation — its finalizer is reaped on the next reconcile of the target.)
- **A `--name` run is not a recovery for a Failed workflow** — but it is not a second wipe either. Adoption is exclusive (one `status.adoptedWorkflow` pointer per node), so while the Failed workflow holds the node the fresh workflow is never adopted: no plan is compiled, no `reset-data` runs, and the watch ends at `--timeout` (`reason=Timeout`) having changed nothing while the node stays held. The wasted 15m is the cost, not a stacked wipe. Remove the Failed workflow first, every time — that removal is what releases the node.

### Output and timeout

- **Success:** plan progress as NDJSON on stdout, one line per workflow event (the full CR); exit 0 when `.status.phase` reaches `Complete`. **`Complete` means the recipe's mutations landed and the node was released — not that it caught up.** The resync runs after Complete; the CLI prints the verification handoff (`seictl node watch <node> --until=caught-up`) on success, and reporting a state-sync as done without that watch passing is premature. `--dry-run` / `--no-watch` stop after render / apply and emit the CR instead of watching. **Avoid `--no-watch` on this command** — with it the agent never observes a Failed phase, and a Failed workflow silently holds the node not-ready.
- **Failure:** `metav1.Status` on stderr, non-zero exit. Human diagnostic lines on stderr are prefixed `seictl:`; strip them before parsing:

```sh
seictl workflow state-sync <node> -n eng-<alias> 2>err.log; grep -v '^seictl:' err.log | jq -r .reason
```

  `.reason` is `Timeout` on `--timeout`, `InternalError` on a terminal `Failed` phase.
- **Timeout:** `--timeout` defaults to 15m (60m on older binaries — the bound is the binary's; the watch semantics below are the controller's, regardless of binary). The watch ends when the workflow releases the node — catch-up happens after Complete and is not part of the watch — so a timeout **usually means a wedged recipe step, not a slow sync**. On a timeout, **do not kill-and-retry**: read the plan first (`seictl workflow list -n eng-<alias>`, then `.status.plan.tasks` on the one in-flight workflow) and map the verb to what you see. An archive-scale `reset-data` still clearing is the one legitimately slow case — raise `--timeout` and wait. Any other step parked past its budget is wedged — force-delete it. Launching a second workflow alongside it does not help and is not a shortcut: adoption is exclusive, so the new one parks unadopted behind the held node and times out too. One version tell, and on the current fleet it should never fire: a plan whose **last** task is `await-condition` comes from a controller predating the release-terminal recipe, where a long watch would be a slow catch-up rather than a wedge. harbor and the prod cells track controller main closely, so seeing that shape means you are on an unexpectedly stale cell — establish which controller image the cell runs before acting on it, rather than settling in to wait.

## `seictl task`

The operator-facing surface over **one** node's sidecar task API (`/v0/tasks`), reached directly through that pod's in-pod kube-rbac-proxy on **:8443** (not the sidecar's own :7777). Sibling of `seictl workflow`, and the distinction is load-bearing: `workflow` creates a CR the **controller** executes with its hold/ordering machinery; `task` posts to **one pod's sidecar** with the controller uninvolved. Added by seictl #229 and **first shipped in `v0.0.66`** — above the skill's `v0.0.59` preflight floor, so confirm with `seictl task --help` before the first invocation in a session rather than assuming the binary has it. `seictl task --help` is the source of truth.

Every verb addresses a single pod. Target with `--node <name>` (the SeiNode / headless-service name; the sidecar resolves at `<node>-0.<node>.<ns>`) — required on every verb except `snapshot-upload`'s discovery path. Shared flags: `-n/--namespace`, `--port` (default `8443`), `--kubeconfig`. The verbs dial the pod, so run them from somewhere with cluster network reachability.

### `seictl task snapshot-upload` — the paved road

```
seictl task snapshot-upload [--node <name> | --chain <chain-id>]
                            [--timeout 2h15m] [--poll-interval 20s]
                            [-n <ns>] [--port 8443] [--kubeconfig <path>]
```

Submits one `snapshot-upload-once` with a fresh unique task ID and polls it to a terminal state. This is the procedure the per-(network, cluster) CronJob invokes daily — reach for it when an engineer needs an on-demand snapshot publish, not as part of chain bring-up.

- **Target:** `--node` names one explicitly; `--chain` discovers a random pod labelled `sei.io/snapshot-publish=true,sei.io/chain=<chain>` (exact match). Mutually exclusive. When no pod carries the labels, discovery says so — fall back to `--node`.
- **Exit codes are kubectl-wait-compatible:** 0 when the task ends `uploaded` **or** `noop` — a `noop` is healthy (the chain hasn't advanced a snapshot interval; the verb prints which outcome it was). Nonzero on a failed task, or on `--timeout`, where the task **may still be running server-side** — `seictl task delete <id>` cancels it.
- **Defaults:** `--timeout` 2h15m, deliberately above the sidecar's own 2h upload deadline so the CLI bound never fires before the server's. `--poll-interval` 20s (sidecar-local, cheap).
- **Output:** the terminal `TaskResult` as JSON on stdout; progress and the verdict on stderr.
- **The fresh task ID is load-bearing.** The engine coalesces a reused ID onto an existing Completed row and never re-runs it — which is why the verb mints its own. Don't hand-craft a repeated ID via `submit` and expect a re-run.

### Raw verbs — thin wrappers over the sidecar client

| Verb | What it does |
|---|---|
| `seictl task get <id> --node <name>` | Read one task result |
| `seictl task list --node <name>` | List recent task results (the node's task history) |
| `seictl task submit <type> --node <name> [--params '<json>']` | POST an arbitrary task; params validated server-side |
| `seictl task delete <id> --node <name>` | Delete a task result, or **cancel** it if still running |

`list` and `get` are the read side and are safe — they're a useful diagnosis path when a workflow step is parked (read the node's own task history rather than inferring from `.status.plan.tasks` alone).

**`submit` is a genuine escape hatch — treat it like the direct-apply escape hatch, not like a read.** It POSTs any task type the sidecar's wire protocol accepts, and that set includes destructive ones (`reset-data` among them). Submitted this way a task runs **without** the workflow recipe around it — no `mark-not-ready` hold, no `stop-seid` first, no ordering guarantee, and no adoption pointer telling the controller a node is occupied. Prefer `seictl workflow state-sync` for anything the recipe already covers, and require explicit engineer sign-off (naming node, namespace, and task type) before submitting a mutating task by hand.

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
--genesis-override gov.voting_params.voting_period=120s
--genesis-override gov.voting_params.expedited_voting_period=60s
--genesis-override gov.deposit_params.min_deposit='[{"denom":"usei","amount":"100"}]'
```

Each entry writes a flat dotted-key into `spec.genesis.overrides`. The first segment is a cosmos module that exists in `app_state` (`staking`, `bank`, `gov`, `mint`, `slashing`, etc.). Values parse as JSON when they parse (numbers, bools, objects, arrays); otherwise as raw strings — durations like `120s` fail JSON parse and land as the string `"120s"`, which is exactly the proto-JSON duration encoding genesis expects. To force a numeric-looking value to render as string, wrap in JSON quotes: `--genesis-override foo.bar='"42"'`.

Single-segment keys (`--genesis-override staking=...`) and empty values are rejected at apply time.

**The sharp edge — only the module segment is validated; the rest of the key is not.** The genesis assembler checks that the first segment names an existing `app_state` module, then creates any missing deeper path segments and writes the value verbatim, unvalidated. A wrong field name therefore passes `--dry-run`, passes apply, passes genesis assembly — and then **every node crash-loops at InitChain** with an "unknown field" panic, and the chain never starts. The intended change never applies. Two upstream-Cosmos-shaped keys that do NOT exist on sei and have caused exactly this: `gov.params.voting_period_seconds` (sei's gov genesis nests under `voting_params`/`deposit_params`/`tally_params`; there is no `params` object) and `mint.params.inflation` (sei's mint module is custom — `mint_denom` + `token_release_schedule` only; inflation-rate semantics are not expressible in any key).

**Key provenance rule:** never guess a key from upstream Cosmos docs — sei's forks diverge. Take keys from a real genesis: `app_state.<module>` in `curl <any-node>:26657/genesis` on a running chain, or the embedded chains in sei-config. Value shapes matter too: durations and decimals are JSON strings (`"60s"`, `"0.4"`). Production precedent for fast governance: the sei-k8s-controller nightly upgrade suite pins `gov.voting_params.voting_period: "60s"` (`test/integration/upgrade_test.go`).

**Verify after Ready** (cheap, do it): from a validator pod, `seid q gov params voting` (or read `/genesis` on 26657) confirms the override landed; a throwaway proposal submitted with the full min-deposit proves the voting window behaviorally.

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
