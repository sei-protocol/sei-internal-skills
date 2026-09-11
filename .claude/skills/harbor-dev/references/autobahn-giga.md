# Autobahn / Giga / EVM-only on harbor

Read this when an engineer asks for a "giga node", "giga executor", "flatkv chain", "autobahn network", "evm-only chain", or wants to reproduce `sei-chain/integration_test/autobahn/README.md` on the harbor cluster. The README describes a self-contained Docker/AWS harness (`autobahn-e2e`); this file maps each of its knobs onto what the harbor controller can and cannot express today, so the agent never promises a topology the CRD cannot render.

Last verified 2026-09-11 against sei-chain `main` (`sei-tendermint/node/public.go`, `giga/evmonly/rpc/server.go`, `docker/localnode/scripts/step4_config_override.sh`, `sei-db/config/sc_config.go`) and sei-k8s-controller `main` (`api/v1alpha1/seinetwork_types.go`, `internal/noderesource/noderesource.go`, `sidecar/tasks/assemble_genesis.go`).

## Three tiers, three answers

| Tier | What it is | Deployable on harbor today? |
|---|---|---|
| **Giga** | `[giga_executor]` (parallel EVM executor + OCC) and/or Giga storage (`sc-write-mode`, `evm-ss-split`, `[receipt-store]`) on an otherwise normal Cosmos+CometBFT chain | **Yes** — every knob is an `app.toml` key, so it is `spec.configValues` (see `seinetwork-crd.md`, `--config-value` in `seictl-cli.md`). Benchable with the regular `sei-load-bench.md` flow: CometBFT RPC and the full `eth_*` surface stay up |
| **Autobahn** | Autobahn consensus (`autobahn-config-file` in `config.toml` → `autobahn.json`) driving the normal Cosmos application | **No** — `autobahn.json` is a genesis-ceremony artefact (`seid tendermint gen-autobahn-config <every node dir>`), and `ConfigValue.fileName` admits only `*.toml`. Tracked in PLT-1249 |
| **EVM-only** | `evm-only = true` in `config.toml`: seid swaps the Cosmos app for the disk-backed EVM-only executor, chain ID `713715`, `2^200` wei implicit balances | **No** — needs Autobahn (`evm-only requires autobahn-config-file`, `node/public.go:validateNodeSetupConfig`) **and** it stops the CometBFT RPC entirely (`node/node.go`: `if n.config.EVMOnly { evmonlyrpc.Start … } else if RPC.ListenAddress != ""`). The controller's validator readiness probe is `GET /lag_status` on `:26657` (`noderesource.go:readinessProbeForNode`) and `restart-seid` waits on `/status`, so an EVM-only pod never reads Ready and every day-2 plan wedges. Also PLT-1249 |

**Never** try to close the Autobahn/EVM-only gap with `spec.configOverrides`, an init container, or a hand-edited pod. `configOverrides` is a Running-node no-op, and anything outside the CRD is invisible to the planner, so the next reconcile either ignores it or fights it. Say plainly that the topology is not deployable yet and point at PLT-1249.

## Tier 1 — Giga via `spec.configValues`

The README's validators run "the production-shaped Giga storage manager: FlatKV for EVM state, littidx for receipts, littblock for blocks." On a Cosmos+CometBFT chain the equivalent, taken from `step4_config_override.sh`, is:

```yaml
spec:
  configValues:
    # executor
    - {fileName: app.toml, key: giga_executor.enabled,     value: true}
    - {fileName: app.toml, key: giga_executor.occ_enabled, value: true}
    # storage — SC layer. Pin the mode, or `sc-write-mode-enable-auto` (default true)
    # forces `auto` and silently ignores the explicit value.
    - {fileName: app.toml, key: state-store.sc-write-mode,             value: "test_only_dual_write"}
    - {fileName: app.toml, key: state-store.sc-write-mode-enable-auto, value: false}
    # storage — SS layer
    - {fileName: app.toml, key: state-store.evm-ss-split, value: true}
    # receipts
    - {fileName: app.toml, key: receipt-store.rs-backend, value: "pebble"}
```

`seictl` form: one `--config-value app.toml:giga_executor.enabled=true` per entry (`seictl-cli.md`).

Rules, each with its consequence:

- **`sc-write-mode` values are a closed enum** — `memiavl_only`, `migrate_evm`, `evm_migrated`, `migrate_all_but_bank`, `all_migrated_but_bank`, `migrate_bank`, `flatkv_only`, `test_only_dual_write`, `auto` (`sei-db/config/sc_config.go`). A typo is accepted by the CRD (the value is opaque JSON) and rejected by seid at boot, so the validator pool crash-loops after genesis. Copy the literal.
- **Pick one storage mode per chain and say which.** `step4` exposes two fresh-boot modes and refuses to combine them: `GIGA_STORAGE=true` → `test_only_dual_write` + `evm-ss-split` (execution reads EVM state from memiavl while FlatKV is populated alongside — "test clusters only, never testnet/mainnet"), and `GIGA_FLATKV_ONLY=true` → `flatkv_only` (the post-migration terminal state, booted directly). `GIGA_MIGRATE_FROM_MEMIAVL` (`memiavl_only` → `migrate_evm` mid-run) is the runner-driven migration path, not a bench topology. Rendering dual-write **and** flatkv_only in one `configValues` list is the combination the script rejects; the CRD will not.
- **Giga storage is an on-disk format decision; it is create-time.** A day-2 `configValues` edit restarts every validator in place with the new `app.toml` (the day-2 `configValues` guardrail in `SKILL.md`), but a store opened as memiavl does not become FlatKV on restart. Changing storage mode on a running chain means a new chain (or the `state-sync-bootstrap.md` migration path for a single follower — "migrate a node to giga store" in the triage table).
- **Defaults drift.** `step4` writes `[giga_executor]` explicitly "because the Go config defaults it on, so a node that never writes the section silently runs giga regardless." Whatever the engineer wants — on **or off** — write it; never rely on omission, or an A/B bench compares two identical executors.
- **Pin the storage knobs on both sides of a comparative bench.** `comparative-bench.md` substitutes image per side; storage/executor knobs must be identical unless they *are* the variable. State which one it is in the experiment dir README.

Verification after `Running`: `kubectl exec <validator-0-pod> -c seid -- grep -A2 '^\[giga_executor\]' /root/.sei/config/app.toml`; the value must match the CR, else the `config-patch` task failed — read `.status.plan` per `troubleshooting-seinode.md`. `ConfigValuesValid=False` on the SeiNode means the entry never reached the file (see `seinetwork-crd.md`).

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

- **RPC surface is two methods.** `giga/evmonly/rpc` serves `eth_sendRawTransaction` and `eth_getTransactionReceipt` on `:8545`; every other `eth_*` returns method-not-found. That kills `eth_blockNumber`/`eth_chainId` probes in `cluster-inspection-recipes.md`, the `rpc-up` `StatusCheck` and the "follower height +3 under fault" liveness gate in `chaos-faults.md`, and `seiload` `trackReceipts` (the inclusion tracker is `SubscribeNewHead` + `eth_getBlockByNumber`, `sei-load/stats/inclusion_tracker.go`), `trackBlocks`, `trackUserLatency`. The nightly `autobahn_evm_only.json` profile keeps all three off for that reason; inclusion is confirmed out-of-band with `cast receipt <hash>`.
- **Validators serve EVM.** The `sei-load-bench.md` rule "target RPC followers, never validators" is a Cosmos-chain rule; in EVM-only mode the validator *is* the RPC (`:8545` on the validator pod). A follower SeiNode has no role.
- **Height 0 until load.** `allow_empty_blocks: false` means a healthy idle chain sits at height 0 — any "Ready = producing" gate (spec 007 / PLT-1251) must carve this out.
- **No funding block.** Absent addresses read as `2^200` wei, so the `funding.rootKeyFile` requirement in `sei-load-bench.md` does not apply — but only in EVM-only. A Tier-1 Giga chain on a vanilla image still needs funding or a mock-balances image.
- **Throughput arithmetic.** `offered TPS ≈ desired tx/block × 2.5` at 400 ms; per-block cap `min(2000, floor(35_000_000 / tx gas))` → 1,666 for 21k-gas transfers. Useful for sizing `settings.tps` on any 400 ms-block chain, not just EVM-only.

## Halt conditions specific to this reference

- Engineer asks for Autobahn or EVM-only on harbor → state the tier table verdict, offer Tier 1 (Giga on CometBFT) as the deployable subset, link PLT-1249. Do not render a SeiNetwork with `evm-only = true`.
- `sc-write-mode` value not in the enum above → refuse to render; a wrong literal is a post-genesis crash-loop of the whole pool.
- Request to flip storage mode on a `Running` chain → refuse the in-place edit; it is a new chain.
- `[giga_executor]` omitted on one side of a comparative bench → add it explicitly to both sides before rendering.
