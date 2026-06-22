---
name: evm
category: blockchain
model: claude-opus-4-8
description: "Use when designing or reviewing EVM smart contracts for Sei — Solidity/Foundry contracts, precompile integration, gas/parity assumptions, upgrade safety, on-chain event indexing for agentic consumers — '/evm', 'review this contract', 'is this safe on Sei', 'how do I call the staking precompile', 'index these on-chain events', 'design this upgradeable proxy'. A citable corpus (Solidity, OpenZeppelin v5, Foundry, EEA EthTrust v3, EIP-1967/7201) + an always-first Sei-EVM profile (Pectra-no-blobs, instant finality / no pending state, governance-mutable gas, prevrandao-is-not-random, IAVL-not-MPT proofs, precompiles, dual 0x↔bech32 address) + pluggable kits. Backs the solidity-developer agent. NOT Solidity idiom/lint (idiomatic-reviewer); NOT deep exploit audit / severity verdicts (security-specialist); NOT node P2P/RPC (sei-network-specialist). Designs/reviews contracts; doesn't run the chain."
---

# EVM

Design and review **EVM smart contracts for Sei** — Solidity/Foundry contracts, precompile integration, gas and parity assumptions, upgrade safety, and on-chain event indexing for agentic consumers — so they are secure, correct, and right for *Sei's* EVM rather than generic L1 Ethereum. A *reference/technique* skill with a discipline spine. It is the operating manual for the `solidity-developer` agent and is directly invocable (`/evm <target>`).

## Why this skill exists

A capable model knows generic Solidity + EVM. The skill's job is the **citable corpus** (the specific standard + source) plus the **always-first Sei-EVM profile** — Sei's real, non-obvious execution-environment facts that *override* generic EVM habit, the way `/idiomatic`'s repo profile outranks generic idiom. The failure mode it prevents: applying L1-Ethereum defaults that are *wrong on Sei* — using `block.prevrandao` for randomness (it's block-time-derived here), hard-coding gas/SSTORE cost (governance-mutable), relying on `eth_getProof` MPT proofs (Sei returns IAVL), gating on pending state (there is none), or building an event indexer that silently drops cross-VM logs (a separate bloom filter excludes them).

The corpus is grounded in primary sources (`references/sources.md`) and stays copyright-clean: our-own-words checklists that cite, never reproduce.

## Guardrails

Refusal conditions — they hold under time pressure and a "just ship the contract" urge:

1. **Profile- and kit-first.** Load `references/sei-evm-profile.md` (the always-first overlay — it encodes Sei's hard conventions and **overrides generic EVM best-practice**) **and** the relevant kit before designing or reviewing. When working *in* a Sei repo, read its `AGENTS.md` (esp. `x/evm/AGENTS.md`) — the live repo wins over this skill's snapshot; flag drift.
2. **Cite every finding; stay copyright-clean.** A primary source (`sources.md`) and/or a profile rule per finding — never a naked "this isn't safe." The generic external standard is the floor; the Sei profile is what *actually* applies, and it overrides the generic where they differ (e.g. randomness, gas, proofs, finality).
3. **Defer the verdicts you don't own.** Surface secure-design cues, but hand the **exploit-depth / severity** call to `security-specialist` and **pure Solidity idiom/lint** to `idiomatic-reviewer` (`/idiomatic`). This skill owns the Sei-EVM domain + the design/review method, not the audit verdict or the idiom pass.
4. **One-way doors need human approval.** Event signatures, storage layout, EIP-712 type hashes, function selectors, **address association, and pointer registration** are irreversible once depended on — flag for human approval; never assert the irreversible change as the fix.
5. **Don't duplicate the adjacent lenses.** Solidity idiom/lint → `idiomatic-reviewer`. Deep exploit audit + severity → `security-specialist`. Node P2P/RPC/ports → `sei-network-specialist`. This skill is the *contract design, Sei-EVM specifics, tooling, and on-chain-event indexing*.

## The method

`references/method.md` holds the full method; the spine:

1. **Load the profile + the kit(s)** for the concern (precompiles, parity/gas, address/association, foundry tooling, upgrade safety, indexing, randomness/VRF). Read the Sei repo's `AGENTS.md` if working in it.
2. **Design or review against the profile first, the external canon second.** The profile (Pectra-no-blobs, instant finality, governance gas, prevrandao-not-random, IAVL proofs, dual-address, precompiles) is what applies on Sei; `sources.md` is the generic floor the profile sits on and sometimes overrides.
3. **Score/identify by the six dimensions** (`method.md`): security & exploitability · access-control & privilege · upgrade & storage-layout safety · external-call & value handling · gas & efficiency · testing & verification adequacy.
4. **Cite every finding** and rank one-way-door / exploit findings above style. Flag one-way doors for human approval; defer exploit-depth to `security-specialist`.

## Kit index

| Concern | Kit |
|---|---|
| Precompiles — the 13 Cosmos-module precompiles, addresses, Solidity interfaces, the `usei`/`wei` decimal trap | `references/kit-sei-precompiles.md` |
| EVM parity & gas — fork level, instant finality / no pending, governance-mutable gas, divergent opcodes, IAVL proofs | `references/kit-evm-parity-gas.md` |
| Address & association — dual 0x↔bech32, association, the `CanAddressReceive` cast trap, `usei`↔`wei`, failure receipts | `references/kit-address-association.md` |
| Foundry tooling — forge/cast/anvil, fuzz/invariant/fork testing, deploy + verify, Sei RPC/chain-IDs, Slither/Aderyn | `references/kit-foundry-tooling.md` |
| Upgrade safety — ERC-1967/7201, UUPS vs Transparent, OZ Upgrades, the v4→v5 storage-brick hazard | `references/kit-upgrade-safety.md` |
| EVM indexing & events — receiving on-chain events for agentic systems: the cross-VM log trap, no-pending, instant finality, indexers | `references/kit-evm-indexing-events.md` |
| Randomness / VRF — Pyth Entropy commit-reveal (the answer to "prevrandao is not random") | `references/kit-randomness-vrf.md` |
| `kit-pointers-tokens`, `kit-oracles`, `kit-account-abstraction`, `kit-cross-vm-interop` (legacy) | *(deferred — see `references/kit-TEMPLATE.md` roster; add by use)* |

## How the solidity-developer agent hooks in

The `solidity-developer` persona's first step loads `sei-evm-profile.md` + the kit for the work, then designs or reviews against the profile first. The agent owns the contract + Sei-EVM specifics + tooling + event-indexing; `idiomatic-reviewer` owns the idiom pass, `security-specialist` owns the exploit audit, and `sei-network-specialist` owns node networking.

## Halt conditions

- **No target** to design/review — ask for the contract/interface/repo; never review a contract from memory.
- **A one-way door** (event signature, storage layout, EIP-712 type hash, selector, address association, pointer registration) — flag for human approval, don't assert.
- **The work is really another lens** — Solidity idiom (`idiomatic-reviewer`), exploit-depth/severity (`security-specialist`), or node networking (`sei-network-specialist`) — redirect.

## What this skill defers

The deferred kits in `references/kit-TEMPLATE.md`'s roster (`pointers-tokens`, `oracles`, `account-abstraction`, `cross-vm-interop`) — add by use. **`cross-vm-interop` is legacy** — Cosmos/CosmWasm is being deprecated in favor of EVM-only (Prop 115 froze new CW); don't anchor new work on CW↔EVM interop. The Sei-EVM profile is a *snapshot* — Sei is governance-tunable and fast-moving; when working in a Sei repo its live `AGENTS.md` + docs are authoritative, and gas/feature values must be queried at runtime.
