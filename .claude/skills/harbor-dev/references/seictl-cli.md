# seictl CLI surface

Canonical command reference for the engineer-facing surface. **`seictl nd --help` is the source of truth** — when this file disagrees, the CLI wins.

## Top-level commands

| Command | Domain | What it does |
|---|---|---|
| `seictl config patch` | local | Patch app.toml/client.toml/config.toml |
| `seictl genesis patch` | local | Patch genesis JSON |
| `seictl patch` | local | Generic TOML/JSON merge-patch |
| `seictl serve` | local | Run the in-pod sidecar HTTP server |
| `seictl await` | local | Wait condition |
| `seictl report` | local | Analyze shadow chain comparison data |
| `seictl nodedeployment` (alias `nd`) | cluster | Manage `SeiNodeDeployment` CRs via embedded presets |

The skill only invokes the `nodedeployment` subtree above. The `local` commands are out of scope for engineer-facing intents.

The pre-#133 cluster verbs (`context`, `onboard`, `bench up/down/list`) are gone — replaced by the preset-driven `nd` tree below. If a reference to those names surfaces in older docs, it's stale.

## `seictl nodedeployment` (alias `nd`)

Five verbs: `apply`, `get`, `list`, `delete`, `watch`. Every verb operates against `seinodedeployments.sei.io/v1alpha1` and follows kubectl-shaped flag conventions.

**Common flags on every verb:**

- `--kubeconfig <path>` (also `$KUBECONFIG`) — colon-merge honored. Defaults to `$HOME/.kube/config` or in-cluster auth.
- `--namespace <ns>` / `-n <ns>` — target namespace. Falls back to the kubeconfig context's default namespace, then the in-cluster ServiceAccount's namespace.

The skill always passes `-n eng-<alias>` explicitly.

### `seictl nd apply`

```
seictl nd apply <name>
                --preset <genesis-chain | rpc>
                [--chain-id <id>] [--image <ref>] [--replicas N]
                [--set <dotted.path>=<value>] [--set ...]
                [--dry-run]
                [-n <ns>] [--kubeconfig <path>]
```

Loads the named preset, applies discrete-flag and `--set` overrides, and **server-side-applies** the result to the cluster. With `--dry-run`, the apiserver validates and returns the would-be-applied CR without persisting.

**Layering, lowest precedence first:**

1. Preset YAML (embedded in the seictl binary).
2. Discrete flags (`--chain-id`, `--image`, `--replicas`).
3. `--set <dotted.path>=<value>`. Strategic-merge: maps merge per-key, lists replace wholesale. Wins on collision with discrete flags.

**Required:** `<name>` (positional) and `--preset`. Both v1 presets (`genesis-chain` and `rpc`) require `--chain-id` and `--image` after layering — if either is missing in the rendered CR, the apiserver rejects the apply with `metav1.Status.reason=Invalid`.

**Output (success):** the post-apply `SeiNodeDeployment` CR on stdout as JSON. Same shape as `kubectl get snd <name> -o json`.

**Output (failure):** `metav1.Status` on stderr. Non-zero exit. Discriminate with `jq -r .reason` (e.g., `Invalid`, `Forbidden`, `NotFound`, `AlreadyExists`).

**Stderr provenance line (always):** `seictl: applying SeiNodeDeployment <ns>/<name> to <api-server>` (or `applying (dry-run)`). Useful for orchestrator scripts that want to log the resolved target.

### `seictl nd get`

```
seictl nd get <name> [-o yaml | json | name | jsonpath=<template>]
              [-n <ns>] [--kubeconfig <path>]
```

Read-only. Returns the CR as `kubectl get snd <name> -o <format>` would — yaml is the default.

**Output formats:**
- `yaml` (default): full CR as YAML.
- `json`: full CR as JSON.
- `name`: `seinodedeployment.sei.io/<name>` only.
- `jsonpath=<template>`: kubectl-style JSONPath. Example: `-o jsonpath='{.status.endpoints.evmJsonRpc[0]}'`.

**Failure:** `metav1.Status` on stderr (`reason=NotFound` if the CR doesn't exist; `Forbidden` if RBAC denies).

### `seictl nd list`

```
seictl nd list [--all-namespaces|-A] [--selector <label-selector>]
               [-o yaml | json | name | jsonpath=<template>]
               [-n <ns>] [--kubeconfig <path>]
```

Returns a `SeiNodeDeploymentList`. `-A` overrides `-n` and lists across all namespaces. `--selector` (`-l`) accepts standard label selectors (e.g., `-l sei.io/chain=foo,sei.io/role=validator`).

**Failure:** `metav1.Status` on stderr.

### `seictl nd delete`

```
seictl nd delete <name>
                 [--cascade foreground | background | orphan]
                 [-n <ns>] [--kubeconfig <path>]
```

Issues a Delete against the named CR. Default propagation is `foreground` (waits for child Pods/Services/HTTPRoutes to be deleted before the SND itself is removed). `background` returns immediately and lets the controller clean up async. `orphan` leaves children behind.

**Output (success):** `seinodedeployment.sei.io/<name> deleted` on stdout. Exit 0.

**Failure:** `metav1.Status` on stderr (`reason=NotFound` if the CR is already gone).

### `seictl nd watch`

```
seictl nd watch <name>
                --until <phase>
                [--timeout <duration>]
                [-n <ns>] [--kubeconfig <path>]
```

Streams every event for the named SND as one NDJSON line on stdout. **Exits 0** when `.status.phase == --until` (exact match). **Exits 1** on:

- `--timeout` exceeded (default `15m`) → `metav1.Status.reason=Timeout` on stderr.
- Terminal `Failed` phase → stderr lifts `.status.plan.failedTaskDetail.error` for the failing task.
- Transient API failure → `metav1.Status` on stderr with the transport error.

The `--until` flag is **required**. Common values: `Ready`, `Initializing`, `Running`. Matching is exact; a misspelling never matches and the watch will time out.

**Subsumes `kubectl wait --for=jsonpath='{.status.phase}'=Ready`** with the additional benefit that NDJSON gives the orchestrator the full event log instead of a binary "ready / timed out."

**Idiom for the agent:** `apply` then `watch --until=Ready` is the headline 2-step. The watch returns the final NDJSON line as the canonical "post-Ready CR" — extract endpoints from it without a follow-up `get`.

## Conventions across the surface

### Output shape

Native `SeiNodeDeployment` (or `SeiNodeDeploymentList`) shape on stdout. **No envelope.** Same as `kubectl get snd -o <format>`. The skill's consumers can pipe directly into `jq` or `yq` without unwrapping a `data:` field.

### Errors

Errors on stderr as `metav1.Status` (kind: Status, apiVersion: v1, status: Failure). Discrimination via `.reason` (e.g., `Invalid`, `Forbidden`, `NotFound`, `AlreadyExists`, `Timeout`, `InternalError`). Non-zero exit code on every failure.

```sh
seictl nd apply foo --preset genesis-chain --chain-id bar -n eng-x 2>err.json
jq -r .reason err.json   # → "Invalid" / "Forbidden" / etc.
jq -r .message err.json  # → human-readable
```

### Exit codes

`0` on success, `1` on every failure. Discrimination is via `metav1.Status.reason` on stderr, not the exit code.

### Provenance

When `seictl nd apply` succeeds, the post-apply CR carries:

- `metadata.annotations.seictl.sei.io/preset: <preset-name>` — which preset shaped it.
- `metadata.annotations.seictl.sei.io/version: v0.0.<n>` — which seictl shipped it.
- `metadata.labels.sei.io/chain: <chain-id>` (also stamped on `spec.template.metadata.labels` for the resulting Pods).
- `metadata.labels.sei.io/role: validator|node` (per preset).

`kubectl get snd -o yaml` surfaces these naturally — useful for `git log`-style provenance even though there's no git in the loop.

### No ambient state

Commands never `cd`, never modify `~/.kube/config`, never set env vars in the calling shell. Every kubectl call is explicit about context and namespace.

## Presets

Two presets, embedded in the seictl binary at `nodedeployment/presets/*.yaml`:

### `genesis-chain`

Chain validators that run a fresh genesis ceremony.

```yaml
apiVersion: sei.io/v1alpha1
kind: SeiNodeDeployment
spec:
  replicas: 4
  template:
    spec:
      validator: {}
  genesis: {}
  updateStrategy:
    type: InPlace
```

Layered with `--chain-id` and `--image`, the `genesis` block populates and `spec.template.spec.chainId` + `image` get set. Auto-wired template labels: `sei.io/chain=<chain-id>`, `sei.io/role=validator`.

**Default replicas: 4.** Override with `--replicas` or `--set spec.replicas=N`.

**Genesis params** (`--genesis-override <module.field[.field...]>=<value>`, repeatable, requires `--preset genesis-chain`, available since v0.0.51):

```
--genesis-override staking.params.unbonding_time=600s
--genesis-override bank.params.default_send_enabled=true
--genesis-override gov.params.voting_period_seconds=120
--genesis-override mint.params.inflation='{"min":0.05,"max":0.2}'
```

Each entry writes a flat dotted-key into `spec.genesis.overrides`. The first segment is a cosmos module that exists in `app_state` (`staking`, `bank`, `gov`, `mint`, `slashing`, etc.). Values parse as JSON when they parse (numbers, bools, objects, arrays); otherwise as raw strings. To force a numeric-looking value to render as string, wrap in JSON quotes: `--genesis-override foo.bar='"42"'`.

Local key-shape validation runs at apply time — single-segment keys (`--genesis-override staking=...`) and empty values are rejected up front rather than after the SND has stalled retrying the assemble-genesis task.

**Not reachable via this flag:** `consensus_params.*` (CometBFT consensus params, sibling to `app_state` in `genesis.json`, not under any cosmos module). If you need to change `block.max_gas`, `validator.pub_key_types`, etc., it's not currently reachable through `spec.genesis.overrides`.

Distinct from `--override`, which targets `spec.template.spec.overrides` (per-node `config.toml`/`app.toml`, applied at config-apply time).

**Funded accounts at genesis** (`--genesis-account <address>:<balance>`, repeatable, requires `--preset genesis-chain`):

```
--genesis-account sei1abc...:1000000000000usei
--genesis-account 0xDEAD...:1000000000000000000000usei,500uatom
```

Address can be bech32 (`sei1...`) or 0x-hex. Balance accepts the standard Cosmos coin format — one or more `<int><denom>` entries, comma-separated. Appends entries to `spec.genesis.accounts`. `--set spec.genesis.accounts[N]...` overrides on collision.

### `rpc`

Full-node fleet that peers to an existing chain by label selector.

```yaml
apiVersion: sei.io/v1alpha1
kind: SeiNodeDeployment
spec:
  replicas: 2
  template:
    spec:
      fullNode: {}
  updateStrategy:
    type: InPlace
```

When `--chain-id <id>` is set, the renderer auto-wires:

- Template labels: `sei.io/chain=<id>`, `sei.io/role=node`.
- `spec.template.spec.peers[0].label.selector.sei.io/chain=<id>` — points the fleet at any SND in the namespace tagged with the same chain ID.

**Default replicas: 2.** Override with `--replicas` or `--set spec.replicas=N`.

**The auto-wire is what makes "chain + RPC fleet on the same `--chain-id`" a one-shot.** Without it the agent would have to hand-craft a `--set spec.template.spec.peers...` payload per call.

`seictl nd apply --preset` accepts only `genesis-chain` or `rpc`. If an engineer asks for any other preset, the request can't be served by `nd apply` — surface that and ask whether they want to hand-roll the SND YAML instead.
