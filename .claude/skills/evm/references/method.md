# The method — designing & reviewing EVM contracts on Sei

Two modes, one spine: **design** (authoring Solidity/contracts for Sei) and **review** (a lens over existing contracts/deploys). Both load the profile + kit first, judge against the Sei-EVM profile's hard conventions before the generic external canon, and rank exploit/one-way-door findings above style.

## The four stages

1. **Load.** `sei-evm-profile.md` (always first — its conventions override generic EVM habit) + the kit(s) for the concern (precompiles, parity/gas, address/association, foundry tooling, upgrade safety, indexing, randomness/VRF). If working *in* a Sei repo, read its `AGENTS.md`/`CLAUDE.md` (esp. `x/evm/AGENTS.md`); the live repo wins over this snapshot — flag drift.
2. **Read the whole target.** For review: the contract(s) + their storage layout, the proxy/init wiring, any precompile calls + their decimal contracts, the events emitted, the deploy/verify scripts, the tests. For design: the existing contracts + interfaces in scope. Never review an EVM contract for Sei from a generic mental model — Sei diverges (no-pending, IAVL proofs, governance-gas, dual-address, cross-VM logs) in ways a default reading gets wrong.
3. **Apply the six dimensions** (below), profile-first. Each finding cites a `sources.md` anchor and/or a profile rule. Flag a genuinely-uncertain call rather than forcing it. Hand exploit-depth/severity verdicts to `security-specialist`; hand pure idiom to `idiomatic-reviewer`.
4. **Rank + surface.** One-way doors (event signatures, storage layout, EIP-712 type hashes, function selectors, **address association, pointer registration**) and exploitability lead; gas/style is bundled. Flag one-way doors for human approval — never assert the irreversible change as the fix.

## The six dimensions (the scorecard)

Grounded in the external canon (`sources.md`) and specialized by the Sei-EVM profile.

1. **Security & exploitability.** Reentrancy (CEI ordering present, not just a guard), oracle/price-manipulation, arithmetic/overflow intent, untrusted-`delegatecall`, **signature & replay safety** (the EIP-712 domain separator binds name+version+**chainId**+verifyingContract; nonce/deadline on signed messages; no `ecrecover` malleability — this is design-time and owned here; the *exploit verdict* stays `security-specialist`'s), **DoS/griefing** (unbounded loops, gas-griefing on external calls), and known-weakness classes. **Sei-specific:** never `block.prevrandao` for entropy (use a VRF — `kit-randomness-vrf`); `SELFDESTRUCT` is out; cross-VM state can move balances outside your call; **chainId binding matters for cross-network replay between `pacific-1`/`atlantic-2` (1329/1328)**. Flag the class + detector; the **exploit-depth/severity verdict is `security-specialist`'s**. *Basis:* `sources.md` §ethtrust, §trailofbits; profile §3.

2. **Access control & privilege.** Owner/role design, missing/incorrect modifiers, privilege-escalation, admin-key blast radius, `msg.sender` (never `tx.origin`) auth, zero-address checks. Prefer OZ `AccessControl`/`AccessManager` (v5). **Sei-specific:** privileged ops that call precompiles (staking/gov/distribution) widen blast radius. *Basis:* `sources.md` §oz, §ethtrust.

3. **Upgrade & storage-layout safety.** Proxy correctness (ERC-1967 slots), storage-collision avoidance, **ERC-7201 namespaced storage** for upgradeables, `initializer`/`reinitializer` discipline, OZ Upgrades storage-layout checks. **Red flag:** a **v4→v5 OZ proxy upgrade** (sequential→namespaced storage) is storage-incompatible and can brick the proxy. *Basis:* `sources.md` §erc1967, §erc7201, §oz.

4. **External-call & value handling.** Checks-Effects-Interactions, checked return on every low-level `call`/`send`, pull-over-push payouts. **Sei-specific:** **precompile value args mix `usei`(6)/`wei`(18) decimals** (e.g. `delegate` wants a `1e12`-multiple) — get the decimal contract right; payable precompiles suppress transfer events; native sends can hit the `CanAddressReceive` cast-address trap. *Basis:* profile §6, §5; `sources.md` §trailofbits.

5. **Gas & efficiency.** Storage packing, transient storage (EIP-1153) where it fits, calldata/loop patterns, custom-error vs require-string reverts. **Sei-specific:** **gas params are governance-mutable — never hard-code SSTORE cost or min gas price; `eth_estimateGas` at runtime**; no block-level gas limit (per-tx only); OCC means hot-shared-state contention serializes — spread writes. *Basis:* profile §2, §10; `sources.md` §solidity, §foundry.

6. **Testing & verification adequacy.** Fuzz + invariant coverage, fork tests against real chain state, `forge coverage`, Slither/Aderyn in CI, deploy-script + `forge verify-contract`. **Sei-specific:** fork-test against a Sei RPC (chain id 1329/1328), not mainnet-Ethereum assumptions; test precompile interactions on a local chain; assert event shape (events are one-way doors). *Basis:* `sources.md` §foundry, §trailofbits; profile §1.

## Design discipline (when authoring, not just reviewing)

- Argue the **maximum scope you'd defend** in the contract domain, then name what you'd **cut first** for an MVP and the condition that un-defers it — the orchestrator/human picks the minimum (the established agent output-discipline).
- A change touching an **event signature, storage layout, EIP-712 type hash, function selector, an address association, or a pointer registration** is a **one-way door** — design it as if you can't take it back, and flag it for human approval before finalizing.
- Reach for the profile's Sei reality over a generic one: entropy is a **VRF** (not `prevrandao`); gas is **estimated at runtime** (not a hard-coded constant); randomness/proof/finality/pending assumptions follow **Sei**, not L1 Ethereum.
