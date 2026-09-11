# Autobahn / Giga / EVM-only on harbor

Read this when an engineer asks for a "giga node", "giga executor", "flatkv chain", "autobahn network", "evm-only chain", or wants to reproduce `sei-chain/integration_test/autobahn/README.md` on the harbor cluster. The README describes a self-contained Docker/AWS harness (`autobahn-e2e`); this file maps each of its knobs onto what the harbor controller can and cannot express today, so the agent never promises a topology the CRD cannot render.

Last verified 2026-09-11 against sei-chain `main` (`sei-tendermint/node/public.go`, `giga/evmonly/rpc/server.go`, `docker/localnode/scripts/step4_config_override.sh`, `sei-db/config/sc_config.go`) and sei-k8s-controller `main` (`api/v1alpha1/seinetwork_types.go`, `internal/noderesource/noderesource.go`, `sidecar/tasks/assemble_genesis.go`).

## Three tiers, three answers

| Tier | What it is | Deployable on harbor today? |
|---|---|---|
| **Giga** | `[giga_executor]` (parallel EVM executor + OCC) and/or Giga storage (`sc-write-mode`, `evm-ss-split`, `[receipt-store]`) on an otherwise normal Cosmos+CometBFT chain | **Yes** — every knob is an `app.toml` key, so it is `spec.configValues` (below). Benchable with the regular `sei-load-bench.md` flow: CometBFT RPC and the full `eth_*` surface stay up |
| **Autobahn** | Autobahn consensus (`autobahn-config-file` in `config.toml` → `autobahn.json`) driving the normal Cosmos application | **No** — `autobahn.json` is a genesis-ceremony artefact (`seid tendermint gen-autobahn-config <every node dir>`), and `ConfigValue.fileName` admits only `*.toml`. Tracked in PLT-1249 |
| **EVM-only** | `evm-only = true` in `config.toml`: seid swaps the Cosmos app for the disk-backed EVM-only executor, chain ID `713715`, `2^200` wei implicit balances | **No** — needs Autobahn (`evm-only requires autobahn-config-file`, `node/public.go:validateNodeSetupConfig`) **and** it stops the CometBFT RPC entirely (`node/node.go`: `if n.config.EVMOnly { evmonlyrpc.Start … } else if RPC.ListenAddress != ""`). The controller's validator readiness probe is `GET /lag_status` on `:26657` (`noderesource.go:readinessProbeForNode`) and `restart-seid` waits on `/status`, so an EVM-only pod never reads Ready and every day-2 plan wedges. Also PLT-1249 |

**Never** try to close the Autobahn/EVM-only gap with `spec.configOverrides`, an init container, or a hand-edited pod. `configOverrides` is a Running-node no-op, and anything outside the CRD is invisible to the planner, so the next reconcile either ignores it or fights it. Say plainly that the topology is not deployable yet and point at PLT-1249.

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
kubectl exec <validator-0-pod> -n eng-<alias> -c seid -- sh -c \
  'awk "/^\\[/{s=\$0} /^(enabled|occ_enabled|sc-write-mode|sc-write-mode-enable-auto|evm-ss-split|rs-backend) *=/{print s, \$0}" $HOME/.sei/config/app.toml'
# $HOME is /home/nonroot in controller-rendered pods (platform.HomeDir); the data PVC is mounted at $HOME/.sei.
# expect, Recipe A: [giga_executor] enabled = true · [state-commit] sc-write-mode = "test_only_dual_write" · [state-commit] sc-write-mode-enable-auto = false · [state-store] evm-ss-split = true · [receipt-store] rs-backend = "pebble"
# expect, Recipe B: same, with sc-write-mode = "flatkv_only" and evm-ss-split = false
```

Every line must match the CR under the expected table. A key printed under a different table, or absent, is the silent-`auto` failure; a mismatch with the CR means `config-patch` did not land — read `.status.plan` per `troubleshooting-seinode.md`. `ConfigValuesValid=False` on the SeiNode means the entry failed CRD-side validation and never reached the file.

## Tier 2/3 — what the README does that the CRD cannot say

For the engineer who asks anyway, the exact delta. Each row is a design input to PLT-1249, not a workaround list.

| README step | Mechanism | CRD status |
|---|---|---|
| `seid tendermint gen-autobahn-config node_0 … node_3 --output autobahn.json` | Ceremony over **all** validator homes (needs every node's key) → per-node file `config/autobahn.json` | No substrate: `ConfigValue.fileName` is `^[A-Za-z0-9_-]+\.toml$`; `assemble-genesis` knows nothing of it |
| `config.toml`: `autobahn-config-file = "/root/.sei/config/autobahn.json"` | Plain TOML key | Expressible via `configValues` — but pointless until the file exists |
| `config.toml`: `evm-only = true` | Plain TOML key | Expressible — but fatal (readiness probe + `restart-seid` on a port seid no longer opens) |
| `config.toml` `[rpc] laddr = ""`; `app.toml` `[api]`/`[grpc]`/`[grpc-web] enable = false` | README turns Cosmos surfaces off in EVM-only mode | Expressible; the controller's Service still publishes `rpc`/`rest` ports that nothing answers |
| `consensus_params.block.max_gas = 35000000` | Top-level genesis field | **Not reachable**: `spec.genesis.overrides` patches `app_state` only (`assemble_genesis.go:Overrides`) |
| `max_txs_per_block: 2000`, `block_interval: 400ms`, `allow_empty_blocks: false`, `block_db.min_retention_age: 30s` | Fields of `autobahn.json` | Same substrate gap |
| `seid start --chain-id sei --inv-check-period 0 --freeze-height 0` | Start flags | StatefulSet `Command` is fixed by the controller; PLT-1250 |
| Chain ID `713715` | Compile-time `config.AutobahnEVMOnlyChainID`; the EVM-only app ignores `app_state` | Not a genesis override — do not add `evm.params.chain_id` "to match", it has no effect in EVM-only and changes a *Tier-1* chain's EVM chain ID away from the default `713714` |

## What EVM-only changes for the rest of this skill

Even though Tier 3 is not deployable today, engineers read the README and will phrase requests in its terms. Know which assumptions elsewhere in `harbor-dev` break under it, so a future PLT-1249 landing does not silently invalidate them:

- **RPC surface is two methods.** `giga/evmonly/rpc` serves `eth_sendRawTransaction` and `eth_getTransactionReceipt` on `:8545`; every other `eth_*` returns method-not-found. That kills the `eth_blockNumber` follower probe, the `rpc-up` `StatusCheck` and the "follower height +3 under fault" liveness gate in `chaos-faults.md`, and `seiload` `trackReceipts` (the inclusion tracker is `SubscribeNewHead` + `eth_getBlockByNumber`, `sei-load/stats/inclusion_tracker.go`), `trackBlocks`, `trackUserLatency`. The nightly `autobahn_evm_only.json` profile keeps all three off for that reason; inclusion is confirmed out-of-band with `cast receipt <hash>`.
- **Validators serve EVM.** The `sei-load-bench.md` rule "target RPC followers, never validators" is a Cosmos-chain rule; in EVM-only mode the validator *is* the RPC (`:8545` on the validator pod). A follower SeiNode has no role.
- **Height 0 until load.** `allow_empty_blocks: false` means a healthy idle chain sits at height 0 — any "Ready = producing" gate (spec 007 / PLT-1251) must carve this out.
- **No funding block.** Absent addresses read as `2^200` wei, so sender accounts need no funding — but only in EVM-only. A Tier-1 Giga chain on a vanilla image still needs funded senders (`seiload` `funding.rootKeyFile`) or a mock-balances image.
- **Throughput arithmetic.** `offered TPS ≈ desired tx/block × 2.5` at 400 ms; per-block cap `min(2000, floor(35_000_000 / tx gas))` → 1,666 for 21k-gas transfers. Useful for sizing `settings.tps` on any 400 ms-block chain, not just EVM-only.

## Halt conditions specific to this reference

- Engineer asks for Autobahn or EVM-only on harbor → state the tier table verdict, offer Tier 1 (Giga on CometBFT) as the deployable subset, link PLT-1249. Do not render a SeiNetwork with `evm-only = true`.
- `sc-write-mode` value not in the enum above → refuse to render; a wrong literal is a post-genesis crash-loop of the whole pool.
- Request to flip storage mode on a `Running` SeiNetwork → refuse the in-place `configValues` edit; it is a new chain. A single follower is the exception, via the gated `seictl workflow state-sync --migration GigaStore` path only.
- Storage recipe copied without asking, or keys mixed across recipes (`flatkv_only` with `evm-ss-split = true`) → stop and ask Recipe A vs B; they bench different code paths.
- `[giga_executor]` omitted on one side of a comparative bench → add it explicitly to both sides before rendering.
