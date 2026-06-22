# Upgrade safety kit

## 1. What this concern is

Upgradeable contracts on Sei use the **standard EVM proxy stack** — ERC-1967 storage slots, UUPS or Transparent proxies, **ERC-7201 namespaced storage** (the OZ v5 model), `initializer`/`reinitializer`. The generic habit this kit corrects is twofold: (1) stale OZ-v4 patterns (sequential storage, `Initializable` without namespaces) and (2) the dangerous **v4→v5 migration** that silently breaks storage. Storage layout is a one-way door; this kit is about not bricking a live proxy. *Cited:* `sources.md` §erc1967, §erc7201, §oz.

## 2. The pattern (how Sei does it)

- **Proxy slots are ERC-1967.** Implementation/admin/beacon at the standard `keccak('eip1967.proxy.<x>') - 1` slots — both UUPS and Transparent use them. *Cited:* `sources.md` §erc1967.
- **UUPS vs Transparent.** Transparent has an immutable admin/upgrade interface in the proxy; UUPS puts the upgrade function in the implementation (cheaper, but the implementation must keep a valid `_authorizeUpgrade`). Pick deliberately. *Cited:* `sources.md` §oz.
- **ERC-7201 namespaced storage (OZ v5).** Upgradeable contracts store state in a namespaced struct at a computed base slot (`@custom:storage-location erc7201:<id>`), avoiding sequential-slot collisions across upgrades. Solidity 0.8.35 has an `erc7201` builtin to compute the slot. *Cited:* `sources.md` §erc7201, §solidity.
- **Init discipline.** Constructors don't run for proxies — use `initializer` (once) / `reinitializer(version)` (per upgrade); lock the implementation with `_disableInitializers()` in its constructor. *Cited:* `sources.md` §oz.
- **Automated layout checks.** Use the **OZ Upgrades plugin** (Hardhat/Foundry) to diff storage layout across versions before upgrading. *Cited:* `sources.md` §erc7201 *(plugin docs URL flagged verify-before-citing)*.

## 3. Anti-patterns / failure modes

- **v4→v5 OZ proxy upgrade without a storage-layout check.** Cue: upgrading a live proxy from OZ v4 (sequential storage at slots `0,1,2,…`) to v5 (ERC-7201 namespaced storage at a computed base slot) — the layouts are incompatible. The proxy keeps *running*, but the v5 code **reads from the namespaced slot while the existing data still sits at the sequential slots** — prior state is silently abandoned (reads zero/garbage). The hazard is **silent state divergence**, not a clean revert, which makes it worse. Rewrite: run the OZ Upgrades storage-layout check; plan an explicit state migration, never a drop-in v5. *Cited:* `sources.md` §oz.
- **Storage collision on upgrade.** Cue: reordered/inserted state vars, or a new var appended without namespacing on a sequential-storage contract. Rewrite: ERC-7201 namespaced storage; never reorder existing slots. *Cited:* `sources.md` §erc7201.
- **Missing/incorrect init.** Cue: a constructor setting state in an upgradeable contract; no `_disableInitializers()` on the implementation; an un-gated `initialize`. Rewrite: `initializer`/`reinitializer`, lock the implementation, gate init. *Cited:* `sources.md` §oz.
- **UUPS without `_authorizeUpgrade`.** Cue: a UUPS implementation that loses its upgrade-auth (or an upgrade to a non-UUPS impl) — can permanently freeze upgradeability. Rewrite: keep a valid, access-gated `_authorizeUpgrade`. *Cited:* `sources.md` §oz, §erc1967.
- **EIP-712 domain assumptions across upgrades.** Cue: a type hash / domain separator that shifts on upgrade after wallets have signed. Rewrite: treat the type hash as a one-way door (below). *Cited:* `sources.md` §solidity.

## 4. Review cues

- **Dimension 3 (upgrade & storage-layout safety):** ERC-1967 slots; ERC-7201 namespaced storage for upgradeables; no reordered/colliding slots; `initializer`/`reinitializer` + `_disableInitializers`; OZ Upgrades layout check run; **explicit v4→v5 migration plan if applicable**. *Basis:* `sources.md` §erc1967, §erc7201, §oz.
- **Dimension 2 (access-control & privilege):** upgrade authority (`_authorizeUpgrade`/proxy admin) is access-gated, single-purpose, with a sane admin-key blast radius. *Basis:* `sources.md` §oz.

## 5. One-way doors in this concern

- **Storage layout** of a deployed upgradeable contract — slot positions are permanent once data lives in them; any change is a one-way door, flag for human approval.
- **EIP-712 type hashes** — permanent after wallets have signed with them; a change invalidates outstanding signatures. Flag.
- **A v4→v5 (or any sequential→namespaced) storage migration** on a live proxy — irreversible if it bricks; require an explicit reviewed migration plan + human approval, never a drop-in.
