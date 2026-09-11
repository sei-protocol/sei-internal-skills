# Randomness / VRF kit

## 1. What this concern is

On-chain randomness on Sei must come from a **verifiable random function (VRF)**, not a block opcode. The generic EVM habit — "use `block.prevrandao` (post-Merge) or `blockhash`" — is **wrong on Sei**: `block.prevrandao` **derives from block time** (profile §3), so it is predictable and griefable. The profile names this trap repeatedly; this kit is the **answer**: Pyth Entropy V2, a commit-reveal VRF with an on-chain callback. *Cited:* sei-docs `evm/vrf/pyth-network-vrf.mdx`; profile §3; `sources.md` §sei.

## 2. The pattern (how Sei does it)

- **Pyth Entropy V2 — commit-reveal.** The contract requests randomness and receives it in a callback, so the value is not known at request time (no front-run). Addresses (from the docs — verify at integration time): **mainnet `0x98046bd286715d3b0bc227dd7a956b83d8978603`**, **testnet `0x36825bf3fbdf5a29e2d5148bfe7dcf7b5639e320`**. *Cited:* sei-docs `evm/vrf/pyth-network-vrf.mdx`. *(addresses are environment-specific — verify-before-print.)*
- **The flow:** quote the fee with `getFeeV2()`, call **`requestV2{value: fee}()`** → returns a `sequenceNumber`, then implement the callback **`entropyCallback(uint64 sequenceNumber, address provider, bytes32 randomNumber)`** to consume the random value. Never use the value before the callback. *Cited:* sei-docs `evm/vrf/pyth-network-vrf.mdx`.
- **Pay the fee.** `requestV2` is payable — fund it with the `getFeeV2()` quote; handle fee changes. *Cited:* sei-docs `evm/vrf/pyth-network-vrf.mdx`.

## 3. Anti-patterns / failure modes

- **Opcode/header "randomness".** Cue: entropy from `block.prevrandao`, `block.difficulty`, `blockhash`, `block.timestamp`, or a hash of these. Rewrite: Pyth Entropy `requestV2` + `entropyCallback`. On Sei `prevrandao` is block-time-derived — predictable. *Cited:* profile §3.
- **Acting on the request before the callback.** Cue: using a value at `requestV2` time, or deriving "randomness" from the `sequenceNumber`. Rewrite: only consume the `randomNumber` delivered to `entropyCallback` (that is the commit-reveal guarantee). *Cited:* sei-docs `evm/vrf/pyth-network-vrf.mdx`.
- **Unfunded / fee-stale request.** Cue: `requestV2()` with no `value` or a hard-coded fee. Rewrite: quote `getFeeV2()` each call and forward it. *Cited:* sei-docs `evm/vrf/pyth-network-vrf.mdx`.
- **Unprotected callback.** Cue: an `entropyCallback` callable by anyone / not validating the provider+sequenceNumber. Rewrite: restrict the callback to the Entropy contract and match the outstanding request. *Cited:* `sources.md` §ethtrust (access control).

## 4. Review cues

- **Dimension 1 (security & exploitability):** no opcode/header randomness; randomness comes from the VRF callback, not the request. The callback is access-restricted and request-matched; nobody can predict or grief the outcome at request time. *Basis:* profile §3; `sources.md` §ethtrust.
- **Dimension 4 (external-call & value handling):** a runtime `getFeeV2()` quote funds the request; callback re-entrancy considered (CEI). *Basis:* `sources.md` §trailofbits.

## 5. One-way doors in this concern

- **A deployed contract's randomness source** (the Entropy provider/address it binds to) is a published dependency. Switching providers post-deploy on a non-upgradeable contract is not possible. Flag a hard-coded VRF dependency in a non-upgradeable contract for human review.
