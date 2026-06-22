# Address & association kit

## 1. What this concern is

On Sei every account has **two addresses for one key** — an EVM `0x…` (keccak of the pubkey) and a Cosmos `sei1…` (bech32) — and they must be **associated** to behave as one account. The generic EVM habit — "an address is an address, balances are unified" — is wrong here: **before association the two are separate accounts with separate balances**, and value handling carries Sei-specific traps (`CanAddressReceive`, `usei`↔`wei` dust, HD coin-type 60 vs 118, failure receipts). *Cited:* `x/evm/keeper/address.go`, `x/evm/AGENTS.md`; sei-docs `learn/accounts.mdx`; `sources.md` §sei.

## 2. The pattern (how Sei does it)

- **One key → two addresses.** EVM `0x…` = last 20 bytes of keccak(uncompressed pubkey); Cosmos `sei1…` = bech32(RIPEMD160(SHA256(compressed pubkey))). A bidirectional mapping is stored on association (`SetAddressMapping`, `x/evm/keeper/address.go:10-13`). *Cited:* sei-docs `learn/accounts.mdx`.
- **Association is permanent + one-to-one.** Via the **addr precompile `0x…1004`** (`associate(v,r,s,customMessage)`, `associatePubKey(hex)`) or **implicitly by signing any tx** (the EVM address is recovered from the secp256k1 pubkey in the ante handler). Emits `EventTypeAddressAssociated`. For wallets, the signed-message method is the recommended path. *Cited:* `precompiles/addr/Addr.sol`, `x/evm/keeper/address.go:17-21`, `x/evm/AGENTS.md`; sei-docs `learn/accounts.mdx`.
- **Resolve with the addr precompile (view).** `getSeiAddr(address)` / `getEvmAddr(string)`; `getSeiAddr` **reverts if unassociated** (selector `0x0c3c20ed`) — surface that as "not linked", not a generic failure. *Cited:* `precompiles/addr/Addr.sol`, sei-docs `learn/accounts.mdx`.
- **Unassociated "cast" fallback.** With no association the module uses a deterministic 20-byte cast (`GetSeiAddressOrDefault`/`GetEVMAddressOrDefault`, `x/evm/keeper/address.go:41-64`) → **two views of the same account until association**. *Cited:* `x/evm/keeper/address.go:41-64`, `x/evm/AGENTS.md`.
- **`usei`(6) ↔ `wei`(18) value bridge.** EVM balances are 18-dec; native is 6-dec; the StateDB bridge tracks a sub-`usei` `wei` remainder (dust). Precompile value args mix decimals (see `kit-sei-precompiles`). *Cited:* `x/evm/AGENTS.md`.

## 3. Anti-patterns / failure modes

- **Assuming a 0x address can receive native/CW funds before association.** Cue: a flow that sends bank/native funds to an EVM address (or ERC tokens to a bech32) assuming they unify. Rewrite: ensure association first (or use the pointer/bank precompile path); surface "not linked" from `getSeiAddr` reverts. *Cited:* profile §5.
- **Ignoring the `CanAddressReceive` cast trap.** Cue: a native send to a direct-cast Sei address whose cast EVM origin was *already associated* with a true pubkey-derived address — the send can be **rejected** (`x/evm/keeper/address.go:78-86`). Rewrite: resolve the true associated address; don't send to a stale cast address. *Cited:* `x/evm/keeper/address.go:78-86`.
- **Same-mnemonic, wrong account.** Cue: deriving with BIP-44 coin type 118 (Cosmos) and expecting the EVM account, or vice versa — EVM uses **coin type 60**, Cosmos **118**; the same mnemonic yields different accounts. Rewrite: derive with the correct coin type for the VM. *Cited:* sei-docs `learn/accounts.mdx`.
- **`usei`/`wei` decimal mismatch / dust.** Cue: treating a precompile value arg as 18-dec when it's 6-dec (or vice versa); ignoring sub-`usei` dust in accounting. Rewrite: match the method's decimal contract; account for the dust remainder. *Cited:* profile §6.
- **Using the retired `sei_associate` RPC.** Cue: a gasless `sei_associate` JSON-RPC call. Rewrite: it's in the deprecated `sei_*` namespace (returns `legacy_sei_deprecated` on public RPCs) — use the signed-message association method. *Cited:* sei-docs `learn/accounts.mdx`.

## 4. Review cues

- **Dimension 4 (external-call & value handling):** flows account for association state before cross-VM value transfer; `getSeiAddr` reverts handled as "not linked"; the `CanAddressReceive` cast trap avoided; decimal contract correct. *Basis:* profile §5, §6; `sources.md` §sei.
- **Dimension 1 (security & exploitability):** no privilege/identity logic assumes the cast and associated addresses are interchangeable; association is treated as a trust event. *Basis:* profile §5.

## 5. One-way doors in this concern

- **Address association is permanent and one-to-one** — a contract or off-chain flow that triggers/relies on an association is making an irreversible identity binding; flag for human approval, surface that it cannot be undone.
