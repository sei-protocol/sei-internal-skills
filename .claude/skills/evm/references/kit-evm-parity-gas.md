# EVM parity & gas kit

## 1. What this concern is

Sei runs a **patched go-ethereum fork** (`v1.15.7-sei-16`, not stock geth) at fork level **Cancun + Prague (Pectra), blobs excluded**, inside a Cosmos chain. "100% EVM tooling compatibility" is true at the *library surface* but **not behavioral parity** — block/finality semantics, gas, several opcodes, and state proofs diverge from L1 Ethereum. The generic habit this kit corrects: assuming L1 behavior for randomness, gas, finality, pending state, and proofs. *Cited:* `go.mod` (go-ethereum fork), `x/evm/types/config.go:11-12`; sei-docs `evm/differences-with-ethereum.mdx`, `evm/evm-parity/evm-compatibility.mdx`; `sources.md` §sei.

## 2. The pattern (how Sei does it)

- **Fork & tx types.** Cancun+Prague at genesis (`CancunTime=0`/`PragueTime=0`, `x/evm/types/config.go:11-12`). Tx types **0/1/2/4** (incl. **EIP-7702 SetCode**) + the Sei `Associate` tx; **type 3 (EIP-4844 blob) is rejected**. **EIP-1153 transient storage is supported**. *Cited:* `x/evm/AGENTS.md`; sei-docs `evm/evm-parity/transaction-types.mdx`.
- **Finality & blocks.** ~400 ms blocks, **instant finality** — `latest`/`safe`/`finalized` collapse to one block; **`tx.wait(1)` is final**; **no pending state** (proposer broadcasts before executing) → pending filters/subscriptions unreliable. *Cited:* sei-docs `evm/evm-parity/finality.mdx`.
- **Gas.** No base-fee burn (fees → validators); EIP-1559 with Sei min/max bounds recomputed each block; legacy txns clear a **governance-set min gas price**; **SSTORE set-gas is a Sei chain param** (`SeiSstoreSetGasEIP2200`, code default `20000` — `x/evm/types/params.go:43`); **no block-level gas limit in the EVM module** (per-tx only). Always `eth_gasPrice`/`eth_estimateGas`. *Cited:* `x/evm/types/params.go:43`, `x/evm/config/config.go:26`, `x/evm/AGENTS.md`; sei-docs `evm/evm-parity/gas-and-fees.mdx`.
- **Divergent opcodes.** `block.prevrandao`/`PREVRANDAO` = **block-time-derived, NOT randomness**; `COINBASE` = global fee-collector (not proposer); `BLOCKHASH` = keccak of the **Tendermint** header; `BASEFEE` no burn; **`SELFDESTRUCT` → use a soft-close pattern**. *Cited:* sei-docs `evm/differences-with-ethereum.mdx`.
- **State proofs.** Single global **AVL tree**, no per-account state root; **`eth_getProof` returns IAVL proofs**, not MPT. *Cited:* sei-docs `evm/differences-with-ethereum.mdx`.
- **Networks.** mainnet `pacific-1` = chain id **1329** (`0x531`); testnet `atlantic-2` = **1328** (`0x530`); devnet `arctic-1` = **713715**; default **713714**. *Cited:* `x/evm/config/config.go:9-26`.

## 3. Anti-patterns / failure modes

- **`block.prevrandao` (or `blockhash`) as randomness.** Cue: entropy derived from `block.prevrandao`/`block.difficulty`/`blockhash`. Rewrite: a VRF (Pyth Entropy — deferred `kit-randomness-vrf`); these are time/header-derived and griefable. *Cited:* profile §3.
- **Hard-coded gas.** Cue: a literal SSTORE cost (20k), a hard-coded min gas price, or a fixed `gasLimit` assuming L1 values; reliance on a "block gas limit." Rewrite: `eth_estimateGas`/`eth_gasPrice` at runtime — Sei's gas is governance-mutable and per-tx. *Cited:* profile §2.
- **Reorg / confirmation-depth logic.** Cue: "wait N confirmations", reorg-rollback handling. Rewrite: finality is instant — `tx.wait(1)` is final; that logic is dead code (and *omitting* it is correct, not risky). *Cited:* profile §1.
- **Gating on pending state.** Cue: `eth_getBlockByNumber("pending")`, pending-nonce arithmetic, pending-tx subscriptions driving logic. Rewrite: there is no pending state — use confirmed `latest`. *Cited:* profile §1.
- **MPT state-proof verification.** Cue: a verifier consuming `eth_getProof` as a Merkle-Patricia proof (cross-chain light client, storage-proof check). Rewrite: Sei returns IAVL proofs — standard MPT verifiers fail; design for IAVL or avoid on-chain state proofs. *Cited:* profile §4.
- **`SELFDESTRUCT`.** Cue: `selfdestruct(...)`. Rewrite: a soft-close (disable + sweep) pattern. *Cited:* profile §3.

## 4. Review cues

- **Dimension 1 (security & exploitability):** no opcode-derived randomness (PREVRANDAO/BLOCKHASH); no SELFDESTRUCT; no reliance on L1-only finality assumptions. *Basis:* profile §3; `sources.md` §ethtrust.
- **Dimension 5 (gas & efficiency):** gas/SSTORE/min-price estimated at runtime, not hard-coded; transient storage (EIP-1153) used where it fits; no dependence on a block gas limit. *Basis:* profile §2; `sources.md` §solidity, §eip1153.
- **Dimension 6 (testing & verification adequacy):** fork tests target a Sei RPC + chain id (1329/1328), not L1 assumptions; finality/pending behavior asserted against Sei semantics. *Basis:* profile §1; `sources.md` §foundry.

## 5. One-way doors in this concern

- **A contract design that bakes in an L1 assumption** (MPT proofs, a fixed gas constant, reorg handling, pending-state gating) is wrong-on-Sei in a way that's expensive to unwind post-deploy — flag the assumption before finalizing, don't silently ship it.
- **Chain-id / network coupling** (a contract that hard-codes `1329` vs `1328` for behavior) is a deploy-target one-way door — parameterize, flag a hard-coded chain id.
