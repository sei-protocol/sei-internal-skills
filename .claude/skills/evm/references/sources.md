# Sources — the EVM / Solidity canon

The external best-practice this skill cites. **Cite-and-link**; never reproduce reserved text. Verified against the primary source during authoring (2026-06) unless flagged. These are the **generic floor**; the Sei-EVM profile (`sei-evm-profile.md`) is what *actually applies* on Sei and **overrides** some of these (gas, randomness, proofs, finality) — each override is flagged in the profile.

## Language, libraries, tooling

- **§solidity — the Solidity compiler.** Target `^0.8.20` minimum (the OZ v5 floor), `0.8.27+` for recent ergonomics: transient storage (EIP-1153 `tstore`/`tload`), `require(cond, CustomError())` (0.8.26 via-IR / 0.8.27 legacy), the `erc7201` builtin (0.8.35). Custom errors since 0.8.4. Pin a specific recent compiler per project; don't float. — https://docs.soliditylang.org/ + https://github.com/ethereum/solidity/releases *(verified via GitHub releases; soliditylang.org 403'd the research fetcher — the version line is corroborated by two sources. The repo org shows `argotorg/solidity` mirrors alongside `ethereum/solidity`; confirm the canonical repo before hardcoding a URL.)*

- **§oz — OpenZeppelin Contracts v5.x.** Library-wide **custom errors**; **`Ownable(initialOwner)`** explicit constructor (rejects `address(0)`); **`Counters`/`SafeMath` removed**; **`AccessManager`** (centralized role system); **ERC-7201 namespaced storage** for all upgradeables; min Solidity 0.8.20. **Hazard:** upgrading a live v4 (sequential-storage) proxy to v5 (namespaced) is storage-incompatible and can brick the proxy. — https://docs.openzeppelin.com/contracts/5.x/changelog + https://github.com/OpenZeppelin/openzeppelin-contracts/releases/tag/v5.0.0 *(verified; treat the 5.6.x **line** as current, don't hardcode the exact patch.)*

- **§foundry — the Foundry toolchain.** Forge (build/test/deploy/verify), Cast (chain interaction), Anvil (local node + forking), Chisel (REPL). Testing canon: fuzz + **invariant** testing, `forge coverage`, cheatcodes, **fork testing**. Deploy/verify: `forge script` + `forge verify-contract`. — https://getfoundry.sh/ *(verified; docs domain is now `getfoundry.sh`, older `book.getfoundry.sh` redirects.)*

## Security canon

- **§ethtrust — EEA EthTrust Security Levels (the live, reviewer-oriented standard).** Three levels: **[S]** automated static checks, **[M]** manual audit, **[Q]** business-logic/intent review. The strongest Solidity-specific, reviewer-oriented citable standard. **Latest = v3 (Mar 2025)**; v4 expected H2 2026. — https://entethalliance.org/specs/ethtrust-sl/v3/ + checklist https://entethalliance.org/specs/ethtrust-sl/v3/checklist.html *(verified; re-check for v4 before relying on a version-pinned clause.)*

- **§trailofbits — Trail of Bits secure-contracts canon.** `building-secure-contracts` (dev guidelines), `not-so-smart-contracts` (curated vuln examples), **Slither** (the canonical static analyzer). Actively maintained. — https://github.com/crytic/building-secure-contracts + https://secure-contracts.com/ *(verified.)*

- **§swc — SWC Registry (HISTORICAL ONLY).** **Unmaintained since 2020**; the README itself redirects to EthTrust + SCSVS. Cite an `SWC-###` only as a historical identifier, **never as the live taxonomy** — use §ethtrust. — https://github.com/SmartContractSecurity/SWC-registry *(verified stale at the repo README.)*

## Proxy / upgrade safety

- **§erc1967 — Proxy Storage Slots (Final).** Standard implementation/admin/beacon slots (`bytes32(uint256(keccak256('eip1967.proxy.<x>')) - 1)`). Both UUPS and Transparent use these. — https://eips.ethereum.org/EIPS/eip-1967 *(verified.)*

- **§erc7201 — Namespaced Storage Layout (Final).** `keccak256(abi.encode(uint256(keccak256(bytes(id))) - 1)) & ~bytes32(uint256(0xff))`, `@custom:storage-location erc7201:<id>` NatSpec. The OZ v5 upgradeable storage model; pairs with the Solidity `erc7201` builtin. Use the **OZ Upgrades plugin** for automated storage-layout compatibility checks; `initializer`/`reinitializer(v)` replace constructors. — https://eips.ethereum.org/EIPS/eip-7201 + https://docs.openzeppelin.com/contracts/5.x/api/proxy *(EIPs verified; the OZ Upgrades plugin docs URL was not fetched — verify before embedding.)*

- **§eip1153 — Transient storage.** `tstore`/`tload`, transient value-type state vars (Solidity ≥0.8.28); cleared at end of transaction — useful for reentrancy locks and transient accounting. — https://eips.ethereum.org/EIPS/eip-1153 *(verified via Solidity release notes.)*

## Sei (the primary Sei source — the profile's grounding)

- **§sei — Sei EVM, primary sources.** `sei-protocol/sei-chain`: `x/evm/AGENTS.md` (the authoritative EVM-module design doc — dual-address, StateDB `usei`↔`wei` bridge, supported tx types, ante/failure-receipt semantics, pointer contracts, precompiles, synthetic receipts/bloom), `precompiles/<name>/*.{go,sol}` (the precompile addresses + published Solidity interfaces), `x/evm/types/config.go` + `x/evm/config/config.go` (fork level, chain ids, gas params). The published Sei docs (`docs.sei.io` / the `sei-docs` repo `evm/`) for the developer-facing canon (precompile addresses, EVM-parity, networks). — https://github.com/sei-protocol/sei-chain + https://docs.sei.io/ *(the profile cites file:line into sei-chain; treat the repo + live docs as authoritative over this snapshot — flag drift.)*

## Delegated-authority canon (for `kit-delegated-authority`)

Verified against primary sources (the EIP text + the implementing repos) at research time. Several are **Draft / pre-1.0** — the kit depends on the *model*, not the exact RPC API.

- **§erc7710 — Smart Contract Delegation (Draft).** A Delegation Manager validates a delegation chain + its caveat enforcers, then executes; redemption entry point is **`redeemDelegations(bytes[] permissionContexts, bytes32[] modes, bytes[] executionCallDatas)`** (plural, batch). The concrete caveat-enforcer set lives in the MetaMask **`delegation-framework`** repo (`src/enforcers/`), CREATE2-self-deployable; audited by Consensys Diligence (Aug 2024 core, Apr 2025 `MultiTokenPeriodEnforcer`). — https://eips.ethereum.org/EIPS/eip-7710 + https://github.com/MetaMask/delegation-framework
- **§erc7715 — Request Execution Permissions (Draft).** Wallet RPC to request a scoped, time-bounded caveat set. **Canonical spec name (post-2026-01) = `wallet_requestExecutionPermissions`** (+ `wallet_revokeExecutionPermission`); **MetaMask's shipping impl still exposes the legacy `wallet_grantPermissions`**. The toolkit is the MetaMask **Smart Accounts Kit** (formerly "Delegation Toolkit"). — https://eips.ethereum.org/EIPS/eip-7715 + https://docs.metamask.io/ *(RPC name churned recently — pin to the wallet you target; see Verify-before-print.)*
- **§erc4337 — Account Abstraction.** EntryPoint **v0.6 / v0.7 / v0.8** (v0.8 current: native EIP-7702 + EIP-712 UserOp hashing). Session-key time-bounding via `validateUserOp`'s packed `validationData` (`validAfter`/`validUntil`). Live on Sei via Pimlico. — https://eips.ethereum.org/EIPS/eip-4337
- **§eip7702 — Set EOA Code (Final, shipped Pectra 2025-05).** A type-4 tx sets an EOA's code to a delegation designator, enabling batching, sponsorship, and scoped session keys directly on EOAs. — https://eips.ethereum.org/EIPS/eip-7702
- **§verifier-chain — smart-account signature verification.** **ERC-1271** (Final — `isValidSignature` → `0x1626ba7e`, deployed accounts) · **ERC-6492** (Final — counterfactual/undeployed accounts) · **ERC-7739** (Draft — nested-712 binds the account address, anti cross-account replay) · **ERC-7730** (Draft — clear-signing metadata). Verify in the order 6492 → 1271 → `ecrecover`. — https://eips.ethereum.org/EIPS/eip-1271 · /eip-6492 · /eip-7739 · /eip-7730
- **§safe-zodiac — m-of-n + timelock.** Safe (Gnosis) m-of-n smart account, CREATE2-self-deployable via `safe-singleton-factory` (for chains lacking a hosted deployment); Zodiac **Delay** modifier adds an enqueue-then-execute cooldown/veto window. — https://github.com/safe-global + https://github.com/gnosisguild/zodiac-modifier-delay
- **§erc8004 — Trustless Agents (Draft).** Agent **identity + discovery + reputation only** (Identity/Reputation/Validation registries); explicitly does **not** grant execution authority — keep authz in the 7710/7715/4337/7702 stack. — https://eips.ethereum.org/EIPS/eip-8004
- **§eas — Ethereum Attestation Service.** Off-chain EIP-712-signed attestations are a first-class EAS path (`signTypedData`, zero-gas), with optional on-chain UID timestamping as the anchor — legitimizes "off-chain-signed + Merkle anchor" as the EAS-blessed default. EAS is not native on Sei. — https://docs.attest.org/

## Verify-before-print (do not assert as fact until checked at primary source)

- **Block gas limit:** the Sei docs cite a "12.5M block gas limit" but `x/evm/AGENTS.md` says the EVM module enforces **no** block-level limit (per-tx only). Reconcile against the live chain before printing a number.
- **Current mainnet gas params** (min gas price ~50 gwei, SSTORE ~72k): governance-mutable point-in-time values — present as "query at runtime," never a fixed constant.
- **Consensys "Smart Contract Security Field Guide"** (successor to `smart-contract-best-practices`): existence indicated but the canonical URL was **not** confirmed — verify before citing.
- **Solodit (Cyfrin) finding aggregator + Aderyn** (Rust static analyzer): useful, but confirmed only second-hand — verify at the primary sites before listing as canon.
- **SCSVS** (Smart Contract Security Verification Standard): recommended companion to EthTrust, but its current version/maintainer was not fetched.
- **ERC-7715 RPC method name:** canonical spec = `wallet_requestExecutionPermissions` (post-2026-01-16 rename), but MetaMask's shipping impl still exposes `wallet_grantPermissions` — both are "current" depending on spec-vs-implementation. It is a Draft EIP whose RPC name changed recently; pin to the exact wallet you target and re-verify. (No method named `wallet_revokePermissions` exists — the spec revoke is `wallet_revokeExecutionPermission`.)
- **EntryPoint addresses on Sei chainId 1328 (testnet):** not published here; the PoC explicitly lists these as an open item — verify the live Pimlico/EntryPoint deployment before wiring, never hard-code.
- **EntryPoint v0.8 numeric diffs** (unused-gas penalty thresholds, exact AUTH-validation details): confirmed via secondary sources only (eip-4337.com was unreachable) — verify against the `eth-infinitism/account-abstraction` release notes before asserting specific numbers.
