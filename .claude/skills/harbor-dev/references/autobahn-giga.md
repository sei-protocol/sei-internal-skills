# Autobahn / Giga / EVM-only on harbor

Read this when an engineer asks for a "giga node", "giga executor", "flatkv chain", "autobahn network", "evm-only chain", or wants to reproduce `sei-chain/integration_test/autobahn/README.md` on the harbor cluster. The README describes a self-contained Docker/AWS harness (`autobahn-e2e`); this file maps each of its knobs onto the CRD field that carries it, and names the ones the controller owns so the agent never double-sets them.

Last verified 2026-09-11 against sei-chain `main` (`sei-tendermint/node/public.go`, `giga/evmonly/rpc/server.go`, `docker/localnode/scripts/step4_config_override.sh`, `sei-db/config/sc_config.go`) and sei-k8s-controller `main` after #553 (`api/v1alpha1/common_types.go:ConsensusSpec`, `internal/planner/consensus_overlay.go`, `internal/noderesource/noderesource.go:UpCheckForNode`, `sidecar/tasks/autobahn.go`, `sidecar/tasks/assemble_genesis.go`). Spec: `specs/008-consensus-engine`.

**Measured on harbor 2026-09-15** against `eng-brandon/brandon-autobahn-02` and `-03`: the four settings an Autobahn bench chain must carry, the working recipe, and the field-choice table. The tier to bench on comes from the same run. Those are measurements, not defaults.

## Three tiers, three answers

| Tier | What it is | Deployable on harbor today? |
|---|---|---|
| **Giga** | `[giga_executor]` (parallel EVM executor + OCC) and/or Giga storage (`sc-write-mode`, `evm-ss-split`, `[receipt-store]`) on an otherwise normal Cosmos+CometBFT chain | **Yes** — every knob is an `app.toml` key, so it is `spec.configValues` (below). Benchable with the regular `sei-load-bench.md` flow: CometBFT RPC and the full `eth_*` surface stay up |
| **Autobahn** | Autobahn consensus (`autobahn-config-file` in `config.toml` → `autobahn.json`) driving the normal Cosmos application | **Yes, on a controller at or after sei-k8s-controller#553** — `spec.consensus: {engine: Autobahn}` on the SeiNetwork. The genesis ceremony generates `autobahn.json` from every validator's identity and publishes it beside `genesis.json`. CometBFT RPC stays up. **This is the tier to bench Giga on today** — the EVM-only row says why. It needs four settings that nothing defaults, and it has **no working RPC followers**, so the bench targets the validators. Read *Four settings an Autobahn bench chain must carry* before rendering. Tier 2 below |
| **EVM-only** | `evm-only = true` in `config.toml`: seid swaps the Cosmos app for the disk-backed EVM-only executor, chain ID `713715`, `2^200` wei implicit balances | **It applies, and it is the wrong tier for a Giga benchmark (measured 2026-09-15)** — `spec.consensus: {engine: Autobahn, evmOnly: true}`. The controller closes the CometBFT RPC/REST/gRPC listeners itself, probes readiness on `GET / :8545`, attaches no `cosmos-exporter`, and `restart-seid`/`stop-seid` wait on the same up-check. Two open sei-chain defects rule it out for a bench — *Bench Giga on Tier 2, not Tier 3*. Everything in *What EVM-only changes for the rest of this skill* applies. Tier 3 below |

**Gate first.** `spec.consensus` exists only on a controller that carries #553. Probe before rendering; a CRD without the field silently prunes it and the pool boots CometBFT:

```sh
kubectl explain seinetwork.spec.consensus.engine --context=harbor && kubectl explain seinetwork.spec.genesis.consensusParams --context=harbor
# both exit 0 on a controller at or after #553; a non-zero exit means the cell predates it — offer Tier 1 only and link PLT-1259 (same probe shape as preflight.md gate 5; the served version is what `explain` reads)
seictl network apply --help | grep -q -- --consensus-engine || echo "seictl predates seictl#255: render the YAML by hand (below), or --set spec.consensus.engine=Autobahn [--set spec.consensus.evmOnly=true]"
```

**Never** close a gap with `spec.configOverrides`, an init container, or a hand-edited pod. `configOverrides` is a Running-node no-op, and anything outside the CRD is invisible to the planner, so the next reconcile either ignores it or fights it.

## Tier 1 — Giga via `spec.configValues`

`spec.configValues[]` is the SeiNetwork's typed `{fileName, key, value}` overlay, applied to every validator's generated TOML (controller spec 002/003, `api/v1alpha1/seinetwork_types.go:ConfigValue`; `fileName` must match `^[A-Za-z0-9_-]+\.toml$`, `key` is dotted, `value` is JSON — no `null`). It is the **only** config surface that works on a Running network: `spec.configOverrides` is applied on the init path and is a no-op afterwards. `seictl network apply --config-value <file>.toml:<dotted.key>=<value>` renders one entry per flag; the flag ships in seictl#253 and is absent from `v0.0.72`, so probe `seictl network apply --help | grep -q -- --config-value` first — a binary without it fails at render with `flag provided but not defined`. Full field/flag semantics land with sei-internal-skills#434 (`seinetwork-crd.md`, `seictl-cli.md`); until it merges this paragraph is the definition.

The README's validators run "the production-shaped Giga storage manager: FlatKV for EVM state, littidx for receipts, littblock for blocks." On a Cosmos+CometBFT chain `step4_config_override.sh` offers two fresh-boot storage recipes. `sc-write-mode` and `evm-ss-split` are coupled — the script never sets one without the other — so take a recipe **whole**; ask the engineer which one before rendering (the rules below say what each measures).

Recipe A — dual-write (`GIGA_STORAGE=true`): FlatKV written alongside memiavl, execution still reads memiavl.

```yaml
spec:
  configValues:
    # executor
    - {fileName: app.toml, key: giga_executor.enabled,     value: true}
    - {fileName: app.toml, key: giga_executor.occ_enabled, value: true}
    # SC layer, table [state-commit] (sei-db/config/toml.go). Pin the mode, or
    # `sc-write-mode-enable-auto` (default true) forces `auto` and silently ignores the explicit value.
    - {fileName: app.toml, key: state-commit.sc-write-mode,             value: "test_only_dual_write"}
    - {fileName: app.toml, key: state-commit.sc-write-mode-enable-auto, value: false}
    # SS layer, table [state-store] — true only with dual-write
    - {fileName: app.toml, key: state-store.evm-ss-split, value: true}
    # receipts
    - {fileName: app.toml, key: receipt-store.rs-backend, value: "pebble"}
```

Recipe B — FlatKV-only (`GIGA_FLATKV_ONLY=true`): the post-migration terminal state, booted directly; this is the FlatKV read path.

```yaml
spec:
  configValues:
    - {fileName: app.toml, key: giga_executor.enabled,     value: true}
    - {fileName: app.toml, key: giga_executor.occ_enabled, value: true}
    - {fileName: app.toml, key: state-commit.sc-write-mode,             value: "flatkv_only"}
    - {fileName: app.toml, key: state-commit.sc-write-mode-enable-auto, value: false}
    # flatkv_only allocates no split store; step4 forces this false
    - {fileName: app.toml, key: state-store.evm-ss-split, value: false}
    - {fileName: app.toml, key: receipt-store.rs-backend, value: "pebble"}
```

`seictl` form: one `--config-value app.toml:giga_executor.enabled=true` per entry. The table prefix is part of the key: `state-store.sc-write-mode` (wrong table) is accepted by the CRD, ignored by seid, and the node silently runs `auto`.

Rules, each with its consequence:

- **`sc-write-mode` values are a closed enum** — `memiavl_only`, `migrate_evm`, `evm_migrated`, `migrate_all_but_bank`, `all_migrated_but_bank`, `migrate_bank`, `flatkv_only`, `test_only_dual_write`, `auto` (`sei-db/config/sc_config.go`). A typo is accepted by the CRD (the value is opaque JSON) and rejected by seid at boot, so the validator pool crash-loops after genesis. Copy the literal.
- **Pick one recipe per chain and say which.** Recipe A measures write amplification, not the FlatKV read path ("test clusters only, never testnet/mainnet"); Recipe B measures the FlatKV read path. `GIGA_MIGRATE_FROM_MEMIAVL` (`memiavl_only` + `evm-ss-split = false` → `migrate_evm` mid-run) is the runner-driven migration path, not a bench topology. Mixing keys across recipes (`flatkv_only` + `evm-ss-split = true`) is a pair the script never emits and the CRD will not refuse. Ask which path the bench is about before rendering; a copied default silently benches the wrong one.
- **Giga storage is an on-disk format decision; it is create-time for a SeiNetwork.** A day-2 `configValues` edit restarts every validator in place with the new `app.toml` — all of them together, so block production stops until >2/3 of voting power is back — but a store opened as memiavl does not become FlatKV on restart. Changing the validator pool's storage mode means a new chain. The one sanctioned per-node exception is a **follower**: `seictl workflow state-sync <node> --migration GigaStore --backend <pebbledb|rocksdb>` (`seictl-cli.md`, *Store migration*) resyncs the node onto the giga store with its own gates — engineer sign-off, `--dry-run` first, escalate on a shared follower. It is destructive and per-SeiNode; it does not apply to a SeiNetwork's validators.
- **Defaults drift.** `step4` writes `[giga_executor]` explicitly "because the Go config defaults it on, so a node that never writes the section silently runs giga regardless." Whatever the engineer wants — on **or off** — write it; never rely on omission, or an A/B bench compares two identical executors.
- **`seictl network apply --set` cannot write a dotted config key.** `--set` splits the path on every dot, so `--set spec.configValues...` and `--set spec.configOverrides...` cannot express `state-store.ss-enable`. Escaping does not help. Use the typed flag (`--config-value app.toml:state-store.ss-enable=true`), or write the YAML in the GitOps repo.
- **Pin the storage knobs on both sides of a comparative bench.** `comparative-bench.md` substitutes image per side; storage/executor knobs must be identical unless they *are* the variable. State which one it is in the experiment dir README.

Verification after `Running` — check the storage keys, not just the executor section, because a misplaced `sc-write-mode` is a healthy node in the wrong mode:

```sh
want=$(kubectl get seinetwork <chain-id> -n eng-<alias> -o jsonpath='{.status.replicas}')
pods=$(kubectl get pods -n eng-<alias> -l sei.io/chain=<chain-id>,sei.io/role=validator -o name)
got=$(printf '%s\n' $pods | grep -c .)
[ -n "$want" ] && [ "$got" -eq "$want" ] || { echo "FAIL: $got validator pods matched sei.io/chain=<chain-id>; seinetwork/<chain-id> .status.replicas='$want' (empty = network lookup missed)"; exit 1; }
for p in $pods; do
  echo "== $p"
  kubectl exec -n eng-<alias> "$p" -c seid -- \
    awk '/^\[/{s=$0} (s ~ /^\[giga_executor\]/ && /^(enabled|occ_enabled) *=/) || /^(sc-write-mode|sc-write-mode-enable-auto|evm-ss-split|rs-backend) *=/{print s, $0}' \
    /home/nonroot/.sei/config/app.toml
done
# The count gate is the point: an empty selector match (wrong chain-id, wrong namespace, pods not up) must fail, not print nothing and pass. Any count other than replicas fails — fewer is a short pool, more is a Retain-orphaned pool on the same chain-id (SKILL.md teardown hazard). The gate assumes the SeiNetwork name equals spec.genesis.chainId (what sei.io/chain carries) — the seictl preset guarantees it; if you named them differently, substitute the network name on the `want=` line.
# awk is deliberately asymmetric: enabled/occ_enabled are anchored to [giga_executor] (they are common key names); the four storage keys are NOT anchored so a key under the wrong table still prints, with its table, and is caught by the expect line.
# /home/nonroot is platform.HomeDir; the data PVC is mounted at /home/nonroot/.sei. Pods carry sei.io/chain + sei.io/role (noderesource.ResourceLabels).
# expect, Recipe A, identical for every validator: [giga_executor] enabled = true · [giga_executor] occ_enabled = true · [state-commit] sc-write-mode = "test_only_dual_write" · [state-commit] sc-write-mode-enable-auto = false · [state-store] evm-ss-split = true · [receipt-store] rs-backend = "pebble"
# expect, Recipe B, identical for every validator: [giga_executor] enabled = true · [giga_executor] occ_enabled = true · [state-commit] sc-write-mode = "flatkv_only" · [state-commit] sc-write-mode-enable-auto = false · [state-store] evm-ss-split = false · [receipt-store] rs-backend = "pebble"
```

Every validator must print the same six lines under the expected tables. A key under a different table, or absent, is the silent-`auto` failure; one validator differing from the rest is the same failure on that node — `config-patch` and `ConfigValuesValid` are per-SeiNode, so a heterogeneous pool keeps producing blocks while the bench attributes its numbers to one storage mode. A mismatch with the CR means `config-patch` did not land — read that SeiNode's `.status.plan` per `troubleshooting-seinode.md`. `ConfigValuesValid=False` on the SeiNode means the entry failed CRD-side validation and never reached the file.

## Tier 2/3 — Autobahn and EVM-only via `spec.consensus`

The README's four-validator EVM-only network, as one SeiNetwork:

```yaml
spec:
  replicas: 4
  consensus:
    engine: Autobahn      # Tendermint when omitted
    evmOnly: true         # drop this line for Tier 2 (Autobahn + Cosmos app)
    autobahn:             # optional; omitted keeps gen-autobahn-config defaults (400ms / false / 2000)
      blockInterval: 400ms
      allowEmptyBlocks: true    # a chain that starts idle never reaches height 1 without it
      maxTxsPerBlock: 2000  # 1..2000; 2000 is the protocol ceiling, only lowering it has an effect
  genesis:
    chainId: <chain-id>
    consensusParams: {block: {max_gas: "35000000"}}   # top-level genesis.consensus_params; overrides cannot reach it
  configValues:
    # Giga recipe A or B from Tier 1, plus evm.http_enabled and state-store.ss-enable.
    # The measured set is below, under `The measured Giga + Autobahn recipe`.
    # Do NOT set: config.toml autobahn-config-file, evm-only, rpc.laddr; app.toml api.enable, grpc.enable, grpc-web.enable
```

`seictl` form: `seictl network apply <chain-id> --replicas 4 --consensus-engine Autobahn --evm-only --config-value app.toml:giga_executor.enabled=true …` (seictl#255). `--evm-only` without `--consensus-engine Autobahn` is refused locally; the CRD refuses the same pair as `evmOnly requires engine Autobahn`.

### Four settings an Autobahn bench chain must carry

Measured on harbor 2026-09-15 against `eng-brandon/brandon-autobahn-02` and `-03`. Each one is total failure, not degradation, and nothing defaults any of them.

1. **No RPC followers.** Under Autobahn, dissemination runs validator-to-validator, so a follower never syncs. The four validators reached height 4062 while both RPC followers sat at height 0 for the whole run. Provision no follower SeiNode on an Autobahn benchmark chain, and point the load profile and every RPC query at the **validators**. This holds on Tier 2 as well as Tier 3, so `sei-load-bench.md`'s "target the followers, never the validators" rule is off here. Its follower pre-flight gate is off too — that gate never passes on an Autobahn chain.
2. **`app.toml` `evm.http_enabled: true`.** Autobahn routes through each validator's `evmrpc` listener. Without the key the chain deadlocks at height 0.
3. **`spec.consensus.autobahn.allowEmptyBlocks: true`.** A chain that starts with no traffic needs it. Without it the chain never reaches height 1, so nothing can submit the first transaction. The field is create-only, so a chain that missed it is a new chain.
4. **`app.toml` `state-store.ss-enable: true`.** Absence of this one key killed a whole benchmark chain. With `evm.http_enabled: true` and `state-store.ss-enable: false`, every historical state read panics:

   ```
   unable to load historical state with SS disabled
   ```

   `eth_sendRawTransaction` reaches that path through gas estimation, so the chain accepts no transactions at all. Set the key explicitly; do not wait for a sei-config default. sei-config's validator profile (`applyValidatorOverrides`) sets `StateStore.Enable` and `EVM.HTTPEnabled` both to `false`. That pair is coherent: a validator that serves no RPC needs no state store. Autobahn creates a role sei-config does not model — a validator that serves RPC.

### The measured Giga + Autobahn recipe

This `spec.configValues` set produced **928,295 transactions at 3,089 TPS** on a four-validator Autobahn chain (harbor, 2026-09-15). Copy it whole.

```yaml
configValues:
  - {fileName: app.toml, key: evm.http_enabled, value: true}
  - {fileName: app.toml, key: evm.ws_enabled, value: true}
  - {fileName: app.toml, key: state-store.ss-enable, value: true}
  - {fileName: app.toml, key: state-store.evm-ss-split, value: true}
  - {fileName: app.toml, key: giga_executor.enabled, value: true}
  - {fileName: app.toml, key: giga_executor.occ_enabled, value: true}
  - {fileName: app.toml, key: state-commit.sc-write-mode, value: test_only_dual_write}
  - {fileName: app.toml, key: state-commit.sc-write-mode-enable-auto, value: false}
  - {fileName: app.toml, key: receipt-store.rs-backend, value: pebble}
```

with `spec.consensus` and `spec.genesis`:

```yaml
consensus:
  engine: Autobahn
  autobahn: {blockInterval: 400ms, allowEmptyBlocks: true, maxTxsPerBlock: 2000}
genesis:
  consensusParams: {block: {max_gas: "35000000"}}
```

The storage half is Tier 1's Recipe A, unchanged. `evm.ws_enabled: true` is in the set so sei-load receipt tracking works without a rebuild of the chain.

### Bench Giga on Tier 2, not Tier 3

EVM-only plus Giga does not work in sei-chain (measured 2026-09-15). Two open issues:

- [`sei-chain#4168`](https://github.com/sei-protocol/sei-chain/issues/4168) — all four validators panic on one transaction whose tip cap sits below the minimum gas price. Admission read `tx.GasPrice()`, which on a dynamic-fee transaction is the fee cap. Admission therefore accepted transactions the executor then refused, and an executor refusal panics the node. Merged PR #4170 fixes it.
- [`sei-chain#4169`](https://github.com/sei-protocol/sei-chain/issues/4169) — EVM-only keeps no durable committed height, so any restart after block 1 is fatal. Still open; it needs an ADR.

Run a Giga benchmark on Tier 2 until both land. When the engineer asks for EVM-only, say this first and offer Tier 2. Do not render EVM-only and let the bench die on a panic.

### Which CRD field carries a change

| Field | Reaches a Running node? | Use it for |
|---|---|---|
| `spec.configValues` | **Yes — drift-aware.** The controller sees the changed hash and applies the config to a running node, and it propagates to the validator pool | anything you may need to change later |
| `spec.overrides` / `spec.configOverrides` | **No — init path only.** The init container writes a config file only when the file is absent. Measured: `app.toml` kept its 2026-09-01 mtime across 7 pod restarts | first-boot-only keys on a node nobody will edit |
| `spec.consensus` | **No — create-only.** You cannot move a chain to Autobahn after creation | the engine choice at create time; otherwise delete and recreate |

An edit to `spec.overrides` on a live chain changes nothing, and reports nothing. Full semantics: `seictl-cli.md` → *Typed config values*, and `troubleshooting-seinode.md` → *configOverrides edits never reach a Running node*.

How each README step lands, and the rule it carries:

| README step | Where it lives now | Rule |
|---|---|---|
| `seid tendermint gen-autobahn-config node_0 … node_3` | `assemble-genesis` builds `autobahn.json` from every validator's uploaded identity (`validator_pubkey`, `node_pubkey`, in-cluster `autobahn_address`/`evmrpc_url`) with the generator's defaults — `max_txs_per_block 2000`, `block_interval 400ms`, `view_timeout 1.5s`, `allow_empty_blocks false`, `block_db` retention `30s` — and uploads it **before** `genesis.json` | `block_interval`, `allow_empty_blocks` and `max_txs_per_block` are tunable through `spec.consensus.autobahn` (create-only; example above). `view_timeout` and `block_db` are not. Never a `configValues` entry — `autobahn.json` is not TOML |
| `autobahn-config-file`, `evm-only`, `[rpc] laddr = ""`, `[api]`/`[grpc]`/`[grpc-web] enable = false` | Controller-owned overlay (`internal/planner/consensus_overlay.go`), merged after `configValues` | A `configValues` entry on any of these keys fails plan build (`this key is set by spec.consensus and cannot be overridden`) and the node never leaves `Initializing`; `spec.overrides` on the listener keys is refused by CEL at apply. Remove the entry, do not fight it |
| `consensus_params.block.max_gas = 35000000` | `spec.genesis.consensusParams` (nested JSON, deep-merged over `seid init`'s `consensus_params`; `null` anywhere is refused) | Quote the number: genesis stores `max_gas` as a string |
| `seid start --inv-check-period 0 --freeze-height 0` | Nothing: both values are `seid start` defaults (`inv-check-period` is discarded by `app.New`), and a SeiNode's `fullNode.freeze`/`archive.freeze` is the typed freeze-height surface (the seed/frozen refusals below) | No field, by design (controller #557) |
| Chain ID `713715` | Compile-time `config.AutobahnEVMOnlyChainID`; the EVM-only app ignores `app_state` | Do not add `evm.params.chain_id` "to match" — no effect in EVM-only, and on any Cosmos-app chain it moves the EVM chain ID off the default `713714` |

Rules, each with its consequence:

- **Read the chain ID off the chain; never copy it.** A Tier-2 Autobahn genesis chain runs EVM chain ID **713714**. `713715` is EVM-only's compile-time ID and belongs to Tier 3 alone. Confirm with `eth_chainId` against a validator before you write a load profile. A wrong chain ID invalidates every signature, and the run then looks like a dead RPC.
- **`spec.consensus` is create-only on its effective value.** Omitted, `{}` and `{engine: Tendermint}` are interchangeable on edit; any real engine or `evmOnly` change is refused (`spec.consensus.engine is create-only`). Changing it is a new chain — same as the Giga storage-mode rule above.
- **An Autobahn chain has no working followers, on either tier.** Measured 2026-09-15: four validators at height 4062, both RPC followers flat at height 0. Dissemination runs validator-to-validator, so a follower never syncs. Do not add one to make the `sei-load-bench.md` topology look familiar. A follower an engineer adds anyway still needs `spec.consensus.engine: Autobahn` (`seictl node apply <name> --network <chain-id> --consensus-engine Autobahn`), or `configure-genesis` fetches only `genesis.json` and seid refuses to start.
- **A seed cannot be `evmOnly`**, and **a frozen node (`fullNode.freeze`/`archive.freeze`) cannot run Autobahn** — both refused at apply, both because seid refuses the combination at start.
- **EVM-only Ready means "the 8545 listener answers", not "synced".** The `/lag_status` probe is gone with the CometBFT RPC. Treat `Running` as "process up" and prove progress with the receipt of a transaction you sent (`cast receipt <hash>`; `eth_blockNumber` is method-not-found here, see below). Idle at height 0 is `allow_empty_blocks: false`. That is healthy as infrastructure, and a dead end for a bench: the chain takes no first transaction (the four settings above).
- **Autobahn without `evmOnly` keeps the full Cosmos surface.** Readiness, `restart-seid`, `seiload` receipt tracking and the rest of the `harbor-dev` recipes behave as on CometBFT. The `autobahn.json` ceremony differs, and the followers do not work (above). Prefer this tier when the engineer says "autobahn" — and when the engineer says "evm-only" too, per *Bench Giga on Tier 2, not Tier 3*.

Verification after `Running` — on every validator:

```sh
want=$(kubectl get seinetwork <chain-id> -n eng-<alias> -o jsonpath='{.status.replicas}')
pods=$(kubectl get pods -n eng-<alias> -l sei.io/chain=<chain-id>,sei.io/role=validator -o name)
got=$(printf '%s\n' $pods | grep -c .)
[ -n "$want" ] && [ "$got" -eq "$want" ] || { echo "FAIL: $got validator pods matched sei.io/chain=<chain-id>; seinetwork/<chain-id> .status.replicas='$want'"; exit 1; }
for p in $pods; do
  echo "== $p"
  kubectl exec -n eng-<alias> "$p" -c seid -- sh -c \
    'grep -E "^(autobahn-config-file|evm-only) *=" /home/nonroot/.sei/config/config.toml; test -s /home/nonroot/.sei/config/autobahn.json && echo autobahn.json:present'
done
# same count gate as Tier 1: an empty selector match must fail, not pass silently. Run the loop body against any Autobahn follower too (-l sei.io/chain=<chain-id>,sei.io/role=node): a follower without autobahn.json is the engine-omitted case from the rules above.
# expect per validator: autobahn-config-file = "/home/nonroot/.sei/config/autobahn.json" · evm-only = true (Tier 3 only) · autobahn.json:present
# missing autobahn.json with genesis present is a terminal configure-genesis failure: read that SeiNode's .status.plan per troubleshooting-seinode.md
kubectl get seinetwork <chain-id> -n eng-<alias> -o jsonpath='{.spec.consensus}'   # what the apiserver kept; empty means the CRD pruned it (gate above)
```

## What EVM-only changes for the rest of this skill

Engineers read the README and phrase requests in its terms. These assumptions elsewhere in `harbor-dev` do not hold under Tier 3:

- **RPC surface is two methods.** `giga/evmonly/rpc` serves `eth_sendRawTransaction` and `eth_getTransactionReceipt` on `:8545`; every other `eth_*` returns method-not-found. That kills the `eth_blockNumber` follower probe, the `rpc-up` `StatusCheck` and the "follower height +3 under fault" liveness gate in `chaos-faults.md`, and `seiload` `trackReceipts` (the inclusion tracker is `SubscribeNewHead` + `eth_getBlockByNumber`, `sei-load/stats/inclusion_tracker.go`), `trackBlocks`, `trackUserLatency`. The nightly `autobahn_evm_only.json` profile keeps all three off for that reason; inclusion is confirmed out-of-band with `cast receipt <hash>`.
- **Validators serve EVM.** The `sei-load-bench.md` rule "target RPC followers, never validators" is a Cosmos-chain rule; in EVM-only mode the validator *is* the RPC (`:8545` on the validator pod). A follower SeiNode has no role. The per-validator URLs are published — `seictl network get <chain-id> -n eng-<alias> -o json | jq -r '[.status.endpoints.nodes[].evmJsonRpc | select(.)]'` (`internal/controller/seinetwork/endpoints.go`; the same leaf exists on a CometBFT network, where nothing answers on it) — use them verbatim as the profile's `endpoints` once the count equals `replicas` (`select(.)` drops a validator that has not published yet), and skip `sei-load-bench.md`'s "at least one Running follower" gate, which never passes here.
- **Profile `chainId` is `713715`.** The EVM-only app runs the compile-time `AutobahnEVMOnlyChainID`, not a genesis value, and `seiload` signs with the envelope `chainId` (`sei-load-bench.md`, *Profile shape* — only the fee cap resolves from the chain). A Tier-1-shaped profile carrying `713714` has every transaction rejected for wrong chain ID, which looks like a dead RPC. The nightly `autobahn_evm_only.json` carries `713715`; copy it.
- **Height 0 until load.** `allow_empty_blocks: false` (the default) means an idle chain sits at height 0. The controller reports this as `Ready` with `Producing=False/Idle` (`seinetwork-crd.md`). That chain also never reaches height 1, so load does not rescue it. A bench chain sets `allowEmptyBlocks: true` at create time (the four settings above), reads `HeightAdvancing` before load, and treats `HeightStalled` as its wedge signal. On an `allowEmptyBlocks`-off chain, gate the bench start on `Ready`. `HeightStalled` is unreachable there: the controller reports every non-advancing height as `Idle` while `allowEmptyBlocks` is off, so a chain that wedges mid-bench also reads `Idle`. Tell the two apart with `.status.observedHeight` (`height`, `time`): a height that advanced and then stopped is the chain. Height 0 under load is ambiguous — misaddressed load generator or a chain wedged before its first block — and seiload's receipt counters cannot settle it here (`trackReceipts` is off, above); read the seiload log for `eth_sendRawTransaction` errors (wrong chain ID rejects every send) and confirm one hash with `cast receipt <hash>` against the validator's `evmJsonRpc` endpoint. `HeightStalled` appears only with `allowEmptyBlocks: true`. Applies to Tier 2 as well as Tier 3.
- **No funding block.** Absent addresses read as `2^200` wei, so sender accounts need no funding — but only in EVM-only. On every other tier **a genesis chain funds the validator accounts and nothing else.** sei-load's sender accounts hold no balance, so every transaction fails. Fund them (`seiload` `funding.rootKeyFile`), or run a mock-balances image: the `mock-nightly-*` tags report a synthetic balance for any address.
- **Throughput arithmetic.** `offered TPS ≈ desired tx/block × 2.5` at 400 ms; per-block cap `min(2000, floor(35_000_000 / tx gas))` → 1,666 for 21k-gas transfers. Useful for sizing `settings.tps` on any 400 ms-block chain, not just EVM-only.

## Halt conditions specific to this reference

- Engineer asks for Autobahn or EVM-only on a cell whose CRD gate (above) fails → offer Tier 1 (Giga on CometBFT) as the deployable subset, ask the platform team to advance the controller pin. Never render `evm-only = true` or `autobahn-config-file` through `configValues` — on a new controller it is refused at plan build, on an old one it is a pod that never reads Ready.
- Engineer asks for a non-default `autobahn.json` field → `spec.consensus.autobahn.{blockInterval, allowEmptyBlocks, maxTxsPerBlock}` in the manifest (controller #555; `kubectl explain seinetwork.spec.consensus.autobahn` is the gate, and there is no seictl flag). Create-only: a change is a new chain. Any other `autobahn.json` field is not expressible; say so rather than editing the file in the pod. `maxTxsPerBlock` above 2000 is refused by the CRD; the protocol clamps there anyway.
- Follower requested on an Autobahn chain, either tier → refuse it and say why: under Autobahn a follower never syncs, and the validators are the RPC.
- Autobahn chain sitting at height 0 with `allowEmptyBlocks` off → the chain cannot take a first transaction. Recreate it with `allowEmptyBlocks: true`; the field is create-only. Do not send load and wait.
- `eth_sendRawTransaction` fails, or a validator logs `unable to load historical state with SS disabled` → `state-store.ss-enable` is false and the chain accepts no transactions. Recreate with the key set. Do not promise that a day-2 `configValues` edit repairs it. Storage mode is a create-time decision for a SeiNetwork (Tier 1 rules above), and the edit restarts every validator at once.
- EVM-only requested for a Giga benchmark → offer Tier 2 and name sei-chain#4168 and sei-chain#4169.
- Load-profile `chainId` copied from another chain or another tier → read `eth_chainId` off a validator first. A wrong chain ID invalidates every signature.
- Engineer asks for a dotted config key through `seictl ... --set` → `--set` cannot write it. Use `--config-value`, or write the YAML in the GitOps repo.
- `sc-write-mode` value not in the enum above → refuse to render; a wrong literal is a post-genesis crash-loop of the whole pool.
- Request to flip storage mode on a `Running` SeiNetwork → refuse the in-place `configValues` edit; it is a new chain. A single follower is the exception, via the gated `seictl workflow state-sync --migration GigaStore` path only.
- Storage recipe copied without asking, or keys mixed across recipes (`flatkv_only` with `evm-ss-split = true`) → stop and ask Recipe A vs B; they bench different code paths.
- `[giga_executor]` omitted on one side of a comparative bench → add it explicitly to both sides before rendering.
