# Ephemeral chain flow (engineer-facing)

The skill's daily-driver flow for engineers spinning up an ephemeral Sei chain on harbor — typically for testing a build, reproducing a bug, validating a release candidate, or driving a load run. Engineer says one English sentence; the agent renders the CRs (one `SeiNetwork` + N `SeiNode` followers) via `seictl network apply --dry-run` / `seictl node apply --dry-run`, opens a PR against `sei-protocol/harbor-engineering-workspace`, and watches the network to Ready (and each follower to Running) after Flux applies on merge.

## The architectural model in three lines

1. **`seictl network apply --dry-run` / `seictl node apply --dry-run` are the rendering verbs.** `network apply` renders a `SeiNetwork` CR (genesis validator pool) from the `genesis-chain` preset; `node apply` renders one `SeiNode` follower from the `rpc` preset — an RPC fleet of N is N of these (no `--replicas` on a node). JSON on stdout — agent pipes through `yq -P 'del(.metadata.creationTimestamp, .metadata.generation, .metadata.managedFields, .metadata.resourceVersion, .metadata.uid, .status)' -o yaml` to strip server-side fields, writes to `engineers/<alias>/<task>/seinetwork-<id>.yaml` and `seinode-<id>-rpc-<k>.yaml` in the workspace repo, opens the PR. No cluster mutation at render time.
2. **The engineer reviews + merges; Flux applies.** Same audit-trail model the team uses across the platform. `harbor-engineering-workspace`'s per-engineer Flux Kustomization watches `engineers/<alias>/`; once the PR merges, the CRs land in `eng-<alias>` within ~60s. `kubectl get seinetwork,seinode -n eng-<alias> -o yaml` is the live view; git is the change history.
3. **The watch step is the post-merge agentic value-add — but the vocab splits.** `seictl network watch --until=Ready` (a SeiNetwork's terminal phase is `Ready`); `seictl node watch --until=Running` per follower (a SeiNode's terminal is `Running` — it has **no `Ready`**; `--until=Ready` on a node errors `Invalid` at parse). Watch streams NDJSON. Endpoints are NOT extracted from the network's last event — the fleet is N CRs; assemble it via `node list … -o json | jq` over each follower's `.status.endpoint` (see the networking section + recipe #1).

```mermaid
flowchart LR
  E[Engineer: natural language] --> A[harbor-dev skill]
  A -->|seictl network apply --dry-run<br/>seictl node apply --dry-run × N| Y[rendered SeiNetwork +<br/>N SeiNode YAMLs]
  Y -->|git commit + push| B[(engineers/&lt;alias&gt;/&lt;task&gt;/<br/>in harbor-engineering-workspace)]
  B -->|PR| PR[reviewable PR]
  PR -->|engineer merges| F[Flux Kustomization]
  F -->|kubectl apply| K[Kubernetes API]
  K --> C1[SeiNetwork CR]
  K --> C2[SeiNode CRs<br/>N followers]
  C1 --> R[sei-k8s-controller reconciles]
  C2 --> R
  R --> V[validator SeiNodes<br/>generated from genesis]
  R --> P[seid pods + per-node<br/>headless Service +<br/>.status.endpoint]
  A -->|network watch --until=Ready<br/>node watch --until=Running| W[NDJSON stream]
  W -.poll.-> C1
  W -.poll.-> C2
  W -->|exit 0 on phase| A
  P -->|PodMonitor| M[Prometheus]
  M --> G[Grafana dashboard]
  G --> E
```

## Preset taxonomy (v1: 2 presets)

Both presets ship embedded in the seictl binary at `presets/<name>.yaml` — the binary version IS the preset version. No remote distribution, no preset versioning lockfile.

The invocation shapes below show the bare apply form. **For engineer-facing flows, the agent always passes `--dry-run`** to capture the rendered CR as JSON, pipes through `yq -P`, and writes the YAML (`seinetwork-<id>.yaml` / `seinode-<id>-rpc-<k>.yaml`) for the workspace-repo PR. Direct (no `--dry-run`) apply is the escape-hatch path covered below.

### `genesis-chain` (→ `seictl network`, renders a `SeiNetwork`)

Validators that run a fresh genesis ceremony on apply. Default 4 replicas (immutable once created).

```sh
seictl network apply <name> \
  --preset genesis-chain \
  --chain-id <chain-id> \
  --image <seid-ref> \
  [--replicas N] \
  -n eng-<alias>
```

Auto-wired on the rendered CR:
- `metadata.annotations.seictl.sei.io/preset: genesis-chain`
- `metadata.labels.sei.io/seinetwork: <chain-id>`
- The controller generates the validator SeiNodes and stamps `sei.io/seinetwork=<chain-id>`, `sei.io/role=validator` on each.

**Immutability:** `spec.genesis` and `spec.replicas` are admission-immutable. To change the chain-id or replica count, `delete` + re-create — a re-apply is rejected `Invalid`.

### `rpc` (→ `seictl node`, renders one `SeiNode` per follower)

A full-node follower that **peers to an existing network by label selector**. One CR per follower; an RPC fleet of N is N `node apply` calls (`<id>-rpc-0 .. <id>-rpc-(N-1)`). No `--replicas`.

```sh
seictl node apply <name> \
  --preset rpc \
  --chain-id <chain-id> \
  --image <seid-ref> \
  --network <chain-id> \
  -n eng-<alias>
```

Auto-wired on the rendered CR (driven by `--network`):
- `metadata.labels.sei.io/seinetwork: <chain-id>`, `metadata.labels.sei.io/role: node`
- **`spec.peers[0].label.selector.sei.io/seinetwork: <chain-id>`** — the follower peers with every node in the namespace tagged with that network. So pointing the fleet at the genesis chain is "pass `--network <chain-id>`."

`--chain-id` (the node's own chain) and `--network` (who to peer with) are independent flags; for an in-namespace ephemeral chain, pass the same value for both. This is the load-bearing piece: `--network <id>` is enough to wire each follower to the genesis validators — no hand-rolled `--set spec.peers...` plumbing.

If an engineer asks for anything other than `genesis-chain` or `rpc` (archive node, single validator, fork-test), `apply` can't serve it — surface that and offer the hand-rolled-CR alternative.

## Override and composition

Atomic preset + `--set` overrides on the CR spec (the spec is flat — no `spec.template`):

| Override path | Mechanism | Example |
|---|---|---|
| Validator-pool replicas (network only) | `--replicas N` (create-time only — immutable) | `--replicas 21` |
| Image ref | `--image <ref>` | `--image ghcr.io/sei-protocol/seid:v6.4.0` |
| Chain ID | `--chain-id <id>` | `--chain-id sei-test-1` |
| Peer target (node only) | `--network <id>` | `--network sei-test-1` |
| Anything else | `--set <dotted.path>=<value>` (repeatable) | `--set spec.resources.requests.memory=8Gi` |

Layering, lowest precedence first: preset YAML → discrete flags → `--set`. Maps merge per-key; lists replace wholesale. Server-side-apply dry-run validates the merged CR against the apiserver's schema (preset isn't enough — the cluster's CRD is the schema oracle).

### Snapshot bootstrap (RPC followers)

For followers attaching to long-lived chains (`pacific-1`, `atlantic-2`), state-sync from a published snapshot is much faster than fresh sync. Discover available heights and wire the spec via `--set spec.fullNode.snapshot.s3.targetHeight=<height>` (flat on SeiNode) — see `references/aws-dependencies.md#snapshot-discovery-harbor-sei-snapshots` for the recipe + sidecar selection mechanics.

Do not wire snapshots for `genesis-chain` (fresh ceremony — nothing to restore from) or short-lived ephemeral chains where fresh sync is fast enough.

### Anti-pattern: do not set `spec.sidecar` overrides

seictl does not populate this field (flat on SeiNode). If it appears in a rendered follower — typically from a stale `--set`, a hand edit, or copy-paste from a debug session — strip it before writing to the workspace repo. The sidecar image is wired by sei-k8s-controller from cluster config; overriding it pins a specific seictl/sidecar version, hides the platform's chosen default, and confuses the failure mode when reproducing a seid bug. (SeiNetwork validators take their sidecar from controller config — there is no per-follower override to make on them.)

The single legitimate use is testing a **platform / seictl / sidecar** change — never a sei-chain change. When the engineer's intent is sei-chain testing (the common case), `spec.sidecar` must be absent.

Echo the absence explicitly in the plan echo: `sidecar: cluster default (no override)`. If the engineer asked for a platform-test custom sidecar, echo `sidecar: <ref> (engineer-supplied; platform/seictl test mode)`.

### Anti-pattern: do not enable `spec.fullNode.snapshotGeneration`

Eng-workspace chains are ephemeral consumers of `harbor-sei-snapshots`, not producers. Enabling generation on a follower publishes non-canonical state and disables seid pruning. If the rendered SeiNode carries it (stale `--set`, hand edit), strip it (flat on SeiNode). Snapshot publishing is the snapshot-publisher workload's job — see `references/aws-dependencies.md#snapshot-discovery-harbor-sei-snapshots`.

## Networking: discoverability is the controller's job, exposure is yours

The controller publishes each node's reachability as `.status.endpoint.*` (in-cluster `svc` URLs) and stamps a per-node **headless Service** for peer connectivity. A SeiNetwork additionally gets a `<network>-internal` ClusterIP Service fronting its validator pool (published as `.status.internalService`, with composed `.status.endpoints` — aggregate URLs for Tendermint RPC/REST only; EVM is advertised per-pod because filters/subscriptions/finalized-tag reads pin to a node; the Service also carries an evm-http port, inert while the backend is validators-only since ModeValidator disables EVM). Beyond that the controller stops: it creates **no** load balancer, no ingress, no HTTPRoute — those were the old SND `spec.networking` concerns and are gone — and standalone followers get no aggregate of any kind. **All exposure topology is engineer-owned Flux YAML** in the task dir, alongside the CRs. This is deliberate — discoverability is the controller's job; exposure is a per-use-case choice the engineer makes explicit in git, reviewable and varying without a controller change.

**The discoverability rule (load-bearing): use the published endpoint verbatim — never reconstruct the URL.** The controller owns the DNS form: the node's per-node **headless** Service at `<node-name>.<namespace>.svc` (e.g. `http://chaos-rpc-0.eng-<alias>.svc:8545`). A reconstructed URL desyncs the moment the controller changes its naming. Read `.status.endpoint` (recipe #1); don't synthesize it.

### The decision rule, by use-case

**Internal ClusterIP/headless over HTTP is the default for in-namespace work. HTTPRoute/ingress only when the engineer explicitly needs external access. p2p is always TCP over the controller's headless Service — never an L7 concern.**

| Use-case | p2p (26656) | REST/RPC/EVM-HTTP (1317/26657/8545), EVM-WS (8546) | gRPC (9090) | Opinionated manifest |
|---|---|---|---|---|
| **In-namespace** (the default — load from a seiload Job in `eng-<alias>`, ad-hoc curl from a debug pod) | headless Service over TCP — already provided by the controller (per-node headless DNS). Nothing to add. p2p is raw TCP and **cannot** route through L7 — it stays pod-to-pod via headless DNS. | point load at the published `.status.endpoint.evmJsonRpc` **verbatim**. No ingress. This is the default the skill renders. | reach the per-node headless DNS directly (h2c is native in-cluster — no proxy, no annotation). gRPC is **not** in `.status.endpoint`, so dial the headless service by name. | none beyond the CRs — or a thin engineer-owned ClusterIP `Service` selecting `sei.io/seinetwork=<id>,sei.io/role=node` if a round-robin VIP across followers is wanted (the aggregate the SND used to auto-create; now explicit). |
| **External access** (laptop / external dApp must reach the chain) | external p2p needs `--external-address` + a NodePort/LB the engineer renders — rare for ephemeral testing; flag as expansion. | **HTTPRoute / ingress over HTTP** — only when external reach is genuinely required. Render a Gateway-API `HTTPRoute` (EVM HTTP + WS via `Upgrade` header match, REST, RPC) into the Flux dir. | gRPC `HTTPRoute` **must** set `appProtocol: kubernetes.io/h2c` on the backend Service port — without it, Istio/Gateway-API mis-detects the protocol and the route breaks silently (no error, just failed h2c framing). Rendered only on explicit external-gRPC intent. | engineer-owned `HTTPRoute` per protocol in the task dir; the gRPC route carries the `h2c` appProtocol. Rendered **only on explicit external-access intent**, never by default. |

The skill renders the opinionated minimum: bare CRs for the common case; an engineer-owned aggregate `Service` or `HTTPRoute` only when the use-case demands it, always as Flux-managed YAML in the task dir, never as a CRD field. For the seid port topology and the Istio/h2c constraints behind this table, the `sei-network-specialist` agent is the authority.

## Output

### `seictl network|node apply` — success

Native `SeiNetwork` / `SeiNode` CR on stdout as JSON. Same shape as `kubectl get seinetwork|seinode <name> -o json`. `.status` is whatever the controller has had time to write — usually empty or `phase=Pending` immediately post-apply. The `watch` step provides the eventual terminal-phase snapshot.

### `seictl network|node apply` — failure

`metav1.Status` on stderr; non-zero exit. Common reasons:

- `Invalid` — the rendered CR fails apiserver schema validation (typo'd `--set` path), OR a re-apply changes an immutable field (`spec.genesis`/`spec.replicas` on a SeiNetwork) — delete + re-create instead.
- `Forbidden` — RBAC denies the apply. Likely the engineer's access entry is read-only; pre-flight gate 5 normally catches this earlier.
- `AlreadyExists` — name collision with an existing CR. If the existing CR is Flux-owned (rendered from another workspace-repo manifest), `git rm` that manifest first; calling `seictl network|node delete` on a Flux-owned CR races the next reconcile. If hand-rolled (no Flux owner), `delete` it or pick a new name.

### `seictl network|node watch` — success

NDJSON stream of CR events on stdout, one per line. Exits 0 when `.status.phase == --until` (exact match). The vocab splits: `network watch --until=Ready`; `node watch --until=Running` (no `Ready` on a node — `--until=Ready` errors `Invalid` at parse). Endpoints are NOT read from the network's last line — assemble the fleet across followers:

```sh
# Per follower: block to Running
seictl node watch foo-rpc-0 --until=Running -n eng-x
# Then read the fleet's published URLs (verbatim — never reconstruct):
seictl node list -n eng-<alias> -l sei.io/seinetwork=foo,sei.io/role=node -o json \
  | jq -r '[.items[].status.endpoint.evmJsonRpc | select(.)]'
```

### `seictl network|node watch` — failure

`metav1.Status` on stderr; non-zero exit. Reasons:

- `Timeout` — `--timeout` exceeded (default 15m). Inspect the last NDJSON line for partial status.
- Terminal `Failed` phase — stderr lifts `.status.plan.failedTaskDetail.error` and the failing task name. Don't auto-retry; surface to the engineer.
- `Invalid` at parse — an illegal `--until` for the tree (e.g. `Ready` on a node). Fix the phase name.
- Transient API failure — transport error from the watch connection; the `metav1.Status.message` carries the detail.

## The headline procedure (PR-based)

Engineer says: "spin up a chain of 4 validators with seid sha=abc, then add an RPC fleet."

1. **Pre-flight** — five gates (see `preflight.md`). Halt on first failure.
2. **Resolve naming** — derive a chain-id from caller context (Linear ticket / PR slug / commit substring / `--tag` / ask). Lowercase, k8s-namespace-safe (`^[a-z]([a-z0-9-]{0,28}[a-z0-9])?$`). For "chain X with RPC," the genesis network is `<id>` and the followers are `<id>-rpc-0 .. <id>-rpc-(N-1)`.
3. **Resolve image** — sei-chain (`seid`) image. **Required input** (PR / commit / branch / explicit `--image`); never silently default. Resolve to a full SHA + verify in registry per `references/image-resolution.md`. Surface the resolved digest in the plan echo.
4. **Render the SeiNetwork CR** — `seictl network apply <id> --preset genesis-chain --chain-id <id> --image <ref> [--replicas N] -n eng-<alias> --dry-run` emits the would-be-applied CR as JSON on stdout. Pipe through `yq -P 'del(.metadata.creationTimestamp, .metadata.generation, .metadata.managedFields, .metadata.resourceVersion, .metadata.uid, .status)' -o yaml` to strip server-side fields (the workspace-repo file should be source-of-truth-shaped, not server-shaped).
5. **Render the follower CRs (if requested)** — **loop** N times: `seictl node apply <id>-rpc-<k> --preset rpc --chain-id <id> --network <id> -n eng-<alias> --dry-run` for `k` in `0..N-1`, same `yq` strip per file. `--network <id>` auto-wires each follower's peer selector at the genesis network; there is no `--replicas` on a node — the skill owns this loop.
6. **Plan echo & confirm** (first side-effecting call only) — show: cluster (harbor), namespace (`eng-<alias>`), preset(s), network name + follower names, chain-id, image digest, validator replica count, target path under workspace repo (`engineers/<alias>/<task>/`), what's about to be committed and pushed. Wait for confirmation.
7. **Write to workspace repo** — fresh clone of `sei-protocol/harbor-engineering-workspace` (or session-scoped clone). Write rendered YAML to `engineers/<alias>/<task>/seinetwork-<id>.yaml` (and `seinode-<id>-rpc-<k>.yaml` per follower, or one multi-doc file). Update `engineers/<alias>/<task>/kustomization.yaml` listing all of them as resources. Append `<task>` to `engineers/<alias>/kustomization.yaml`'s `resources:` list if not already present.
8. **Commit + push** — branch `feat/eng-<alias>-<task>`. Commit message: `feat(eng/<alias>): spin up <task> — chain-id=<id>, image=<digest-prefix>`. Push.
9. **Open the PR** — title: `feat(eng/<alias>): spin up <task>`; body lists chain-id, image digest, preset(s), expected endpoints. `gh pr create --repo sei-protocol/harbor-engineering-workspace --base main`.
10. **Surface and halt** — surface PR URL with: "after merge, Flux reconciles in ~60s; ping me to watch the network to Ready and report endpoints."
11. **After merge — watch genesis to Ready** — `seictl network watch <id> --until=Ready --timeout=15m -n eng-<alias>`. NDJSON stream; exits 0 when `.status.phase=Ready`. Halt on non-zero with the `metav1.Status.reason` surfaced.
12. **Watch each follower to Running** (if applicable) — per `k`: `seictl node watch <id>-rpc-<k> --until=Running --timeout=15m -n eng-<alias>` (terminal is `Running` — `--until=Ready` errors `Invalid` on a node).
13. **Report** — assemble the fleet's endpoints across followers (the fleet is N CRs — there is no single object to read from). Use recipe #1 / `seictl node list -n eng-<alias> -l sei.io/seinetwork=<id>,sei.io/role=node -o json | jq` and read each follower's published URL **verbatim** — never reconstruct it (the controller owns the per-node headless DNS form):
    - `.status.endpoint.evmJsonRpc` — EVM HTTP JSON-RPC URL (per follower)
    - `.status.endpoint.evmWs` — EVM WebSocket URL
    - `.status.endpoint.tendermintRpc` — Tendermint RPC URL
    - `.status.endpoint.tendermintRest` — Tendermint REST URL
    - For pod-targeted connectivity (seiload's WebSocket block collector, etc.), pick one follower — its `.status.endpoint` is already its stable per-node URL.
14. **Report teardown** — `git rm -r engineers/<alias>/<task>/` **and** remove the `<task>` entry from `engineers/<alias>/kustomization.yaml`'s `resources:` list (Kustomize fails to render with an orphan reference). Commit → push → merge. Flux prunes the SeiNetwork + SeiNodes on next reconcile, cascading to pods/PVCs per k8s deletion propagation.

## Halt conditions specific to this flow

Stop and report (don't auto-remediate):

- **Render rejected by `--dry-run` with `metav1.Status.reason=Invalid`.** The would-be-applied CR fails schema validation. Surface the message; ask the engineer to inspect the `--set` paths or preset overrides. Don't push a broken CR to the workspace repo.
- **Re-apply rejected `Invalid` on an immutable field** — `network apply <same-name>` with a changed `--chain-id`/`--replicas` is rejected (`spec.genesis`/`spec.replicas` are admission-immutable). Delete + re-create; don't retry.
- **`AlreadyExists` on cluster post-merge** — the engineer's PR proposed a name that already has a live CR (escape-hatch direct-apply, or stale workspace state). Surface the existing object's metadata; ask whether to pick a different name or `git rm` the old manifest first.
- **Workspace-repo task path collision** — `engineers/<alias>/<task>/` already exists in the workspace repo. Don't silently overwrite. Halt and ask whether to reuse the dir (add new files alongside) or pick a different `<task>` name.
- **Push rejected (non-fast-forward)** — engineer or another agent pushed to the same branch. Don't force-push. Halt; surface `git pull --rebase` and let the engineer resolve.
- **Watch (post-merge) exits with `metav1.Status.reason=Timeout`.** Don't loop. Surface the last NDJSON line's `.status.plan.tasks[]` for the engineer to inspect.
- **Watch exits on terminal `Failed` phase.** Surface `.status.plan.failedTaskDetail.error` and the failing task name. Do not auto-retry — Failed means the controller gave up; the cause is structural.
- **Image digest resolution fails** — image not in registry or auth missing. Stop and surface the recovery command per `references/image-resolution.md`.
- **PR merge stuck** — engineer hasn't merged; agent isn't waiting indefinitely. Surface the PR URL and end the turn; the engineer pings back when ready.

## Escape hatch: direct `seictl network|node apply` (rare)

If the engineer specifically asks to bypass the PR loop for a one-shot debug session and confirms they understand the result won't be in git history, fall through to direct apply: `seictl network apply <id> --preset genesis-chain --chain-id <id> --image <ref> -n eng-<alias>` (no `--dry-run`; server-side applies), then per follower `seictl node apply <id>-rpc-<k> --preset rpc --chain-id <id> --network <id> -n eng-<alias>`. Then `seictl network watch <id> --until=Ready -n eng-<alias>` and `seictl node watch <id>-rpc-<k> --until=Running -n eng-<alias>`.

**Steer first.** The agent does not volunteer this path. Before running it, ask: "I can do this through the GitOps PR flow (audit trail, Flux reconciles, `git rm` to tear down) — do you want that, or do you specifically need a direct-apply run with no git history?" Only proceed on explicit confirmation.

## When the agent should NOT drive any of this

Surface to the engineer if the request maps to:

- **Long-lived shared resources** that other engineers should depend on. Those go through `harbor-engineering-workspace` PRs the engineer authors directly, not through agent-driven task dirs.
- **Cross-namespace work.** The agent operates only in `eng-<alias>`.
- **CRD changes, sei-k8s-controller config changes, cluster-wide Flux updates.** Those go through `sei-protocol/platform` PRs; not in this skill's scope.
