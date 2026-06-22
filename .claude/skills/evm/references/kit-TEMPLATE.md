# EVM-concern kit (TEMPLATE)

A kit is **data** the method loads for one EVM concern (precompiles, parity/gas, address/association, foundry tooling, upgrade safety, indexing, …). It teaches the pattern as Sei actually does it, cites the external canon beneath it, and gives review cues + the failure modes to catch. Adding a concern = drop one file conforming to this template at `references/kit-<concern>.md`.

Each kit provides the five sections below, in order, so the method stays concern-agnostic. Copy the skeleton; see `kit-sei-precompiles.md` for a worked kit.

This section schema is a **soft one-way door** — changing it churns every kit. Revise deliberately.

---

```markdown
# <Concern> kit

## 1. What this concern is
One paragraph: the pattern as Sei's EVM actually does it, and what generic
Ethereum/EVM mental model gets it wrong here (the override).

## 2. The pattern (how Sei does it)
The concrete shape — the precompile/interface/config/tooling — cited to the repo
(file:line in sei-chain) and/or the Sei docs, and to the external canon
(`sources.md` §anchor). "Do it this way."

## 3. Anti-patterns / failure modes
Named smells with a detection cue and the correct rewrite — the generic habits
that are wrong on Sei (e.g. block.prevrandao for randomness; hard-coded gas;
MPT state proofs; pending-tx subscriptions; wrong precompile decimals).

## 4. Review cues
What a reviewer looks for, mapped to the method's six dimensions. Cite the profile
rule / `sources.md` anchor each cue rests on. Always write `Dimension N (name)`.

## 5. One-way doors in this concern
The irreversible / blast-radius-wide decisions (event signatures, storage layout,
EIP-712 type hashes, selectors, address association, pointer registration) that
must be flagged for human approval, not asserted.
```

---

**Authoring rules:**
- **Cite both layers:** the Sei pattern (a file:line in `sei-chain`, or a Sei docs path) AND the external canon (`sources.md`) it specializes or overrides. A claim with neither is not a kit entry.
- The **profile** (`sei-evm-profile.md`) holds the cross-cutting hard conventions — kits reference it, don't restate it.
- Where Sei **overrides** a generic EVM assumption (prevrandao, gas, proofs, finality), say so explicitly and cite the generic as the floor (`sources.md`).
- Keep review cues mapped to the six method dimensions so findings stay rankable. **Always write the dimension as `Dimension N (name)`** — keep the parenthetical name, never a bare `Dimension N`. The number→name map lives only in `method.md`, so a kit pulled into a windowed context must carry the name with it.
- **Hand exploit-depth/severity to `security-specialist` and pure idiom to `idiomatic-reviewer`** — surface the cue + detector, defer the verdict (the boundary the profile names).

## Kit roster (shipped + deferred)

Shipped:
- `kit-sei-precompiles.md` — the 13 precompiles, fixed addresses, Solidity interfaces, the `usei`/`wei` decimal trap, the deprecated oracle.
- `kit-evm-parity-gas.md` — fork level, finality/no-pending, governance-mutable gas/SSTORE, divergent opcodes, IAVL proofs, SELFDESTRUCT.
- `kit-address-association.md` — dual 0x↔bech32, association, the `CanAddressReceive` cast trap, HD coin-type, `usei`↔`wei` value handling, failure receipts.
- `kit-foundry-tooling.md` — forge/cast/anvil/chisel, fuzz/invariant/fork testing, deploy + `verify-contract`, Sei RPC/chain-IDs, Slither/Aderyn.
- `kit-upgrade-safety.md` — ERC-1967/7201, UUPS vs Transparent, OZ Upgrades storage-layout checks, the v4→v5 storage-brick hazard.
- `kit-evm-indexing-events.md` — receiving on-chain events for agentic consumers: the cross-VM synthetic-log/bloom trap, no-pending, instant-finality (no reorg), failure receipts, indexer options, event design.

Deferred (add as a conforming kit when first encountered — the corpus grows by use):
- `kit-pointers-tokens` — native/IBC-denom→ERC20 pointers (first-class), CW20/CW721 pointers (legacy, already-deployed CW only), pointerview/pointer precompiles, versioned/no-pointer-to-pointer, USDC/LayerZero bridging.
- `kit-randomness-vrf` — Pyth Entropy V2 commit-reveal (the answer to "prevrandao isn't random").
- `kit-oracles` — Pyth / Chainlink Data Streams / API3 / RedStone (the native oracle precompile is deprecated).
- `kit-account-abstraction` — ERC-4337 (bundler/paymaster) + EIP-7702 SetCode; Thirdweb/Particle/Pimlico smart wallets.
- `kit-cross-vm-interop` — the `wasmd` precompile + `wasmbinding` (CW↔EVM), the 1-hop write rule. **Legacy — Cosmos/CosmWasm is being deprecated; do not anchor new work here.**
