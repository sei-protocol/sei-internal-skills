# Sei precompiles kit

## 1. What this concern is

Sei exposes Cosmos-SDK modules to Solidity through **stateful precompiles at fixed addresses** (`0x…1001`–`0x…100C`, `0x…1011`). The generic EVM mental model — "precompiles are the 9 cheap crypto builtins at `0x01`–`0x0a`" — is wrong here: Sei's precompiles are **module gateways** (bank, staking, gov, distribution, oracle, ibc, address, json, wasmd, pointer/pointerview, solo, p256), each with a **published Solidity interface** you import and call like a contract. The non-obvious traps are the **non-contiguous address map**, **mixed `usei`(6)/`wei`(18) decimals on value args**, and a **deprecated oracle**. *Cited:* `precompiles/setup.go:46-60` (the address registry); `sources.md` §sei.

## 2. The pattern (how Sei does it)

- **Import the published interface, call the fixed address.** Each precompile ships an interface `.sol` in `precompiles/<name>/` (mirrored under `contracts/src/precompiles/` and exported from `@sei-js/precompiles`). Bind the interface to the address and call. *Cited:* per-precompile `*.sol`; addresses in `precompiles/setup.go:46-60`.
- **The address map (memorize the gap):** `0x…1001` **bank** · `1002` **wasmd** · `1003` **json** · `1004` **addr** · `1005` **staking** · `1006` **gov** · `1007` **distribution** · `1008` **oracle** · `1009` **ibc** · `100A` **pointerview** · `100b` **pointer** · `100C` **solo** · `1011` **p256**. The jump from `100C` to `1011` is real — don't assume a tidy sequence. *Cited:* `precompiles/setup.go:46-60`, each precompile's address const (e.g. `precompiles/bank/bank.go:33`, `precompiles/p256/p256.go:24`).
- **Key surfaces a contract author uses:**
  - **bank** (`0x…1001`): `balance`, `all_balances`, `name/symbol/decimals/supply`, `send` (gated to a registered native ERC20 pointer), `sendNative` (payable, to a bech32 recipient). Reads native/factory denoms `eth_getBalance` can't see. *Cited:* `precompiles/bank/Bank.sol`.
  - **staking** (`0x…1005`): `delegate`(payable)/`undelegate`/`redelegate`/`createValidator`/`editValidator` + queries + events. *Cited:* `precompiles/staking/Staking.sol`.
  - **gov** (`0x…1006`): `vote`/`voteWeighted`/`deposit`/`submitProposal` (JSON string). *Cited:* `precompiles/gov/Gov.sol`.
  - **distribution** (`0x…1007`): withdraw rewards / set withdraw addr / validator commission. *Cited:* `precompiles/distribution/Distribution.sol`.
  - **json** (`0x…1003`): `extractAsBytes`/`extractAsUint256`/… — parse JSON (oracle/NFT-metadata/CW responses) inside a contract. *Cited:* `precompiles/json/Json.sol`.
  - **p256** (`0x…1011`): `verify(bytes)` — **RIP-7212** secp256r1 verification (passkeys/WebAuthn/Secure Enclave). *Cited:* `precompiles/p256/p256.go:74`.
  - **addr** (`0x…1004`): association + `getSeiAddr`/`getEvmAddr` — see `kit-address-association`.
  - **pointer/pointerview** (`0x…100b`/`0x…100A`): create/look-up token pointers — see deferred `kit-pointers-tokens`.
- **The decimal contract is per-method.** `staking.delegate()` takes `msg.value` in **18-dec wei** but truncates to **6-dec `uSEI`** (pass a multiple of `1e12`); `undelegate`/`redelegate` take **6-dec `uSEI`**; bank reads format to 6 decimals, `sendNative` parses 18. Read each method's decimal contract — don't assume 18 everywhere. *Cited:* sei-docs `evm/precompiles/staking.mdx`; profile §6.

## 3. Anti-patterns / failure modes

- **Hand-rolling a precompile address or ABI from memory.** Cue: a literal `0x…1008` with no imported interface, or an assumed method set. Rewrite: import the published `.sol` (or `@sei-js/precompiles`) and use the address from the registry — the map is non-contiguous and methods are exact.
- **Wrong value decimals on a payable precompile.** Cue: `staking.delegate{value: 1}()` or a non-`1e12`-multiple; assuming 18 decimals on a 6-dec method. Rewrite: match the method's documented decimal contract; for `delegate`, send a `1e12` multiple.
- **Building on the oracle precompile (`0x…1008`).** It is **deprecated — "will be shut off soon"**; the docs route devs to Pyth/Chainlink/API3/RedStone. Cue: a contract reading `getExchangeRates`/`getOracleTwaps` from `0x…1008`. Rewrite: a third-party oracle (deferred `kit-oracles`). *Cited:* sei-docs `evm/precompiles/oracle.mdx`.
- **Assuming `wasmd.instantiate` works.** Prop 115 froze new CosmWasm — `instantiate` **reverts** at runtime even though it's in the ABI. Cue: a contract instantiating a CW contract via `0x…1002`. Rewrite: deploy native EVM; CW interop is legacy. *Cited:* profile (direction); sei-docs `evm/precompiles/cosmwasm-precompiles/cosmwasm.mdx`.
- **Treating a precompile call as free/cheap.** They execute module logic and can revert with module errors; gas is real and governance-tunable. Cue: an unchecked precompile call in a tight loop. Rewrite: handle reverts, estimate gas at runtime.

## 4. Review cues

- **Dimension 4 (external-call & value handling):** every precompile call uses the published interface + registry address; value args match the method's `usei`/`wei` decimal contract; return/revert handled. *Basis:* profile §6, §9; `sources.md` §sei.
- **Dimension 1 (security & exploitability):** privileged precompile calls (staking/gov/distribution) are access-gated; no build on the deprecated oracle; entropy is not sourced from a precompile/opcode. *Basis:* profile §3; `sources.md` §ethtrust.
- **Dimension 5 (gas & efficiency):** precompile gas is estimated at runtime, not assumed; no precompile call in an unbounded loop. *Basis:* profile §2.

## 5. One-way doors in this concern

- **A contract whose external interface hard-depends on a precompile address/ABI** is coupled to a chain feature — if a precompile is deprecated (as the oracle is) the contract breaks; flag a precompile dependency in a non-upgradeable contract for human review.
- **`pointer` precompile registration** (creating a token pointer) is a registered, versioned, one-to-one mapping — see `kit-pointers-tokens`; treat registration as a one-way door.
