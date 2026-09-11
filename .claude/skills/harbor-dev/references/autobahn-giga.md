# Autobahn / Giga / EVM-only on harbor

Read this when an engineer asks for a "giga node", "giga executor", "flatkv chain", "autobahn network", "evm-only chain", or wants to reproduce `sei-chain/integration_test/autobahn/README.md` on the harbor cluster. The README describes a self-contained Docker/AWS harness (`autobahn-e2e`); this file maps each of its knobs onto the CRD field that carries it, and names the ones the controller owns so the agent never double-sets them.

Last verified 2026-09-11 against sei-chain `main` (`sei-tendermint/node/public.go`, `giga/evmonly/rpc/server.go`, `docker/localnode/scripts/step4_config_override.sh`, `sei-db/config/sc_config.go`) and sei-k8s-controller `main` after #553 (`api/v1alpha1/common_types.go:ConsensusSpec`, `internal/planner/consensus_overlay.go`, `internal/noderesource/noderesource.go:UpCheckForNode`, `sidecar/tasks/autobahn.go`, `sidecar/tasks/assemble_genesis.go`). Spec: `specs/008-consensus-engine`.

## Three tiers, three answers

| Tier | What it is | Deployable on harbor today? |
|---|---|---|
| **Giga** | `[giga_executor]` (parallel EVM executor + OCC) and/or Giga storage (`sc-write-mode`, `evm-ss-split`, `[receipt-store]`) on an otherwise normal Cosmos+CometBFT chain | **Yes** — every knob is an `app.toml` key, so it is `spec.configValues` (below). Benchable with the regular `sei-load-bench.md` flow: CometBFT RPC and the full `eth_*` surface stay up |
| **Autobahn** | Autobahn consensus (`autobahn-config-file` in `config.toml` → `autobahn.json`) driving the normal Cosmos application | **Yes, on a controller at or after sei-k8s-controller#553** — `spec.consensus: {engine: Autobahn}` on the SeiNetwork. The genesis ceremony generates `autobahn.json` from every validator's identity, publishes it beside `genesis.json`, and every validator (and any follower that also sets `engine: Autobahn`) fetches it. CometBFT RPC stays up, so benching is the regular `sei-load-bench.md` flow. Tier 2 below |
| **EVM-only** | `evm-only = true` in `config.toml`: seid swaps the Cosmos app for the disk-backed EVM-only executor, chain ID `713715`, `2^200` wei implicit balances | **Yes, same gate** — `spec.consensus: {engine: Autobahn, evmOnly: true}`. The controller closes the CometBFT RPC/REST/gRPC listeners itself, probes readiness on `GET / :8545`, attaches no `cosmos-exporter`, and `restart-seid`/`stop-seid` wait on the same up-check. Everything in *What EVM-only changes for the rest of this skill* applies. Tier 3 below |

**Gate first.** `spec.consensus` exists only on a controller that carries #553. Probe before rendering; a CRD without the field silently prunes it and the pool boots CometBFT:

```sh
kubectl get crd seinetworks.sei.io -o jsonpath='{.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.consensus.properties.engine.enum}'
# expect: ["Tendermint","Autobahn"]; empty → the cell's controller predates #553, offer Tier 1 only and link PLT-1249
seictl network apply --help | grep -q -- --consensus-engine || echo "seictl predates seictl#255: render the YAML by hand (below), or --set spec.consensus.engine=Autobahn"
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
  genesis:
    chainId: <chain-id>
    consensusParams: {block: {max_gas: "35000000"}}   # top-level genesis.consensus_params; overrides cannot reach it
  configValues:
    # Giga recipe A or B from Tier 1 goes here unchanged.
    # Do NOT set: config.toml autobahn-config-file, evm-only, rpc.laddr; app.toml api.enable, grpc.enable, grpc-web.enable
```

`seictl` form: `seictl network apply <chain-id> --replicas 4 --consensus-engine Autobahn --evm-only --config-value app.toml:giga_executor.enabled=true …` (seictl#255). `--evm-only` without `--consensus-engine Autobahn` is refused locally; the CRD refuses the same pair as `evmOnly requires engine Autobahn`.

How each README step lands, and the rule it carries:

| README step | Where it lives now | Rule |
|---|---|---|
| `seid tendermint gen-autobahn-config node_0 … node_3` | `assemble-genesis` builds `autobahn.json` from every validator's uploaded identity (`validator_pubkey`, `node_pubkey`, in-cluster `autobahn_address`/`evmrpc_url`) with the generator's defaults — `max_txs_per_block 2000`, `block_interval 400ms`, `view_timeout 1.5s`, `allow_empty_blocks false`, `block_db` retention `30s` — and uploads it **before** `genesis.json` | These defaults are not tunable from the CRD yet. An engineer who needs a different `block_interval` or `allow_empty_blocks: true` gets a "not yet" and a PLT-1249 follow-up, not a `configValues` entry — `autobahn.json` is not TOML |
| `autobahn-config-file`, `evm-only`, `[rpc] laddr = ""`, `[api]`/`[grpc]`/`[grpc-web] enable = false` | Controller-owned overlay (`internal/planner/consensus_overlay.go`), merged after `configValues` | A `configValues` entry on any of these keys fails plan build (`this key is set by spec.consensus and cannot be overridden`) and the node never leaves `Initializing`; `spec.overrides` on the listener keys is refused by CEL at apply. Remove the entry, do not fight it |
| `consensus_params.block.max_gas = 35000000` | `spec.genesis.consensusParams` (nested JSON, deep-merged over `seid init`'s `consensus_params`; `null` anywhere is refused) | Quote the number: genesis stores `max_gas` as a string |
| `seid start --inv-check-period 0 --freeze-height 0` | Still fixed by the controller's StatefulSet `Command` | PLT-1250 |
| Chain ID `713715` | Compile-time `config.AutobahnEVMOnlyChainID`; the EVM-only app ignores `app_state` | Do not add `evm.params.chain_id` "to match" — no effect in EVM-only, and on a Tier-1 chain it moves the EVM chain ID off the default `713714` |

Rules, each with its consequence:

- **`spec.consensus` is create-only on its effective value.** Omitted, `{}` and `{engine: Tendermint}` are interchangeable on edit; any real engine or `evmOnly` change is refused (`spec.consensus.engine is create-only`). Changing it is a new chain — same as the Giga storage-mode rule above.
- **A follower of an Autobahn chain must set `spec.consensus.engine: Autobahn` too**, or `configure-genesis` fetches only `genesis.json` and seid refuses to start without the autobahn file. `seictl node apply <name> --network <chain-id> --consensus-engine Autobahn`. Under EVM-only a follower has no role (validators are the RPC) — do not add one to make the `sei-load-bench.md` topology look familiar.
- **A seed cannot be `evmOnly`**, and **a frozen node (`fullNode.freeze`/`archive.freeze`) cannot run Autobahn** — both refused at apply, both because seid refuses the combination at start.
- **EVM-only Ready means "the 8545 listener answers", not "synced".** The `/lag_status` probe is gone with the CometBFT RPC. Treat `Running` as "process up" and prove progress with `cast block-number` under load. Idle is height 0 (`allow_empty_blocks: false`); that is healthy, not stuck.
- **Autobahn without `evmOnly` keeps the full Cosmos surface.** Readiness, `restart-seid`, followers, `seiload` receipt tracking and every other `harbor-dev` recipe behave as on CometBFT. Only the `autobahn.json` ceremony differs. Prefer this tier when the engineer says "autobahn" and does not say "evm-only".

Verification after `Running` — on every validator:

```sh
for p in $(kubectl get pods -n eng-<alias> -l sei.io/chain=<chain-id>,sei.io/role=validator -o name); do
  echo "== $p"
  kubectl exec -n eng-<alias> "$p" -c seid -- sh -c \
    'grep -E "^(autobahn-config-file|evm-only) *=" /home/nonroot/.sei/config/config.toml; test -s /home/nonroot/.sei/config/autobahn.json && echo autobahn.json:present'
done
# expect per validator: autobahn-config-file = "/home/nonroot/.sei/config/autobahn.json" · evm-only = true (Tier 3 only) · autobahn.json:present
# missing autobahn.json with genesis present is a terminal configure-genesis failure: read that SeiNode's .status.plan per troubleshooting-seinode.md
kubectl get seinetwork <chain-id> -n eng-<alias> -o jsonpath='{.spec.consensus}'   # what the apiserver kept; empty means the CRD pruned it (gate above)
```

## What EVM-only changes for the rest of this skill

Engineers read the README and phrase requests in its terms. These assumptions elsewhere in `harbor-dev` do not hold under Tier 3:

- **RPC surface is two methods.** `giga/evmonly/rpc` serves `eth_sendRawTransaction` and `eth_getTransactionReceipt` on `:8545`; every other `eth_*` returns method-not-found. That kills the `eth_blockNumber` follower probe, the `rpc-up` `StatusCheck` and the "follower height +3 under fault" liveness gate in `chaos-faults.md`, and `seiload` `trackReceipts` (the inclusion tracker is `SubscribeNewHead` + `eth_getBlockByNumber`, `sei-load/stats/inclusion_tracker.go`), `trackBlocks`, `trackUserLatency`. The nightly `autobahn_evm_only.json` profile keeps all three off for that reason; inclusion is confirmed out-of-band with `cast receipt <hash>`.
- **Validators serve EVM.** The `sei-load-bench.md` rule "target RPC followers, never validators" is a Cosmos-chain rule; in EVM-only mode the validator *is* the RPC (`:8545` on the validator pod). A follower SeiNode has no role.
- **Height 0 until load.** `allow_empty_blocks: false` means a healthy idle chain sits at height 0 — any "Ready = producing" gate (spec 007 / PLT-1251) must carve this out.
- **No funding block.** Absent addresses read as `2^200` wei, so sender accounts need no funding — but only in EVM-only. A Tier-1 Giga chain on a vanilla image still needs funded senders (`seiload` `funding.rootKeyFile`) or a mock-balances image.
- **Throughput arithmetic.** `offered TPS ≈ desired tx/block × 2.5` at 400 ms; per-block cap `min(2000, floor(35_000_000 / tx gas))` → 1,666 for 21k-gas transfers. Useful for sizing `settings.tps` on any 400 ms-block chain, not just EVM-only.

## Halt conditions specific to this reference

- Engineer asks for Autobahn or EVM-only on a cell whose CRD gate (above) fails → offer Tier 1 (Giga on CometBFT) as the deployable subset, link PLT-1249. Never render `evm-only = true` or `autobahn-config-file` through `configValues` — on a new controller it is refused at plan build, on an old one it is a pod that never reads Ready.
- Engineer asks for a non-default `autobahn.json` field (`block_interval`, `allow_empty_blocks`, `max_txs_per_block`) → not expressible; say so and file it against PLT-1249 rather than editing the file in the pod.
- Follower requested on an EVM-only chain → ask what it is for; the validators serve `:8545` and a follower serves nothing the bench reads.
- `sc-write-mode` value not in the enum above → refuse to render; a wrong literal is a post-genesis crash-loop of the whole pool.
- Request to flip storage mode on a `Running` SeiNetwork → refuse the in-place `configValues` edit; it is a new chain. A single follower is the exception, via the gated `seictl workflow state-sync --migration GigaStore` path only.
- Storage recipe copied without asking, or keys mixed across recipes (`flatkv_only` with `evm-ss-split = true`) → stop and ask Recipe A vs B; they bench different code paths.
- `[giga_executor]` omitted on one side of a comparative bench → add it explicitly to both sides before rendering.
