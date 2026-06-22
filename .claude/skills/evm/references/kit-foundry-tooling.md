# Foundry tooling kit

## 1. What this concern is

The authoritative EVM-contract toolchain for Sei work is **Foundry** (forge/cast/anvil/chisel), with Slither/Aderyn for static analysis. Sei itself builds its in-repo contracts with Foundry (`foundry.toml`, `contracts/`) alongside Hardhat. The generic habit this kit corrects: testing/deploying against L1-Ethereum assumptions instead of a **Sei RPC + chain id**, and skipping the test classes (fuzz/invariant/fork) + static analysis that the canon expects. *Cited:* `foundry.toml`, `contracts/README.md`, `contracts/hardhat.config.js`; `sources.md` §foundry, §trailofbits.

## 2. The pattern (how Sei does it)

- **The toolchain.** **Forge** (build/test/deploy/verify), **Cast** (chain interaction), **Anvil** (local node + forking), **Chisel** (REPL). *Cited:* `sources.md` §foundry.
- **Test classes the canon expects:** **fuzz** + **invariant** testing, **`forge coverage`**, **fork testing** (run against real chain state), cheatcodes. Don't ship a contract whose only tests are happy-path unit tests. *Cited:* `sources.md` §foundry.
- **Static analysis in CI:** **Slither** (Trail of Bits — the canonical analyzer, weight findings by Impact+Confidence) and **Aderyn** (Cyfrin, Rust). Surface findings; defer exploit-depth to `security-specialist`. *Cited:* `sources.md` §trailofbits *(Aderyn flagged verify-before-citing in `sources.md`)*.
- **Deploy + verify:** `forge script` for deploys, **`forge verify-contract`** for explorer verification (`seiscan.io`). Sei's explorer is **Blockscout-style** — pass the Sei verifier (`--verifier blockscout --verifier-url <sei-explorer-api>`), not the stock Etherscan default; confirm the current verifier URL from the Sei docs at verify time. *Cited:* `sources.md` §foundry; sei-docs `evm/networks.mdx`. *(verifier URL is environment-specific — verify-before-print.)*
- **Sei in-repo layout (reference):** root `foundry.toml` sets `src=contracts/src`, `out=contracts/out`, `test=contracts/test`; build is `forge install && forge build`; Sei uses Foundry **and** Hardhat (the Hardhat config defines `seilocal`→`127.0.0.1:8545` and an `arctic-1` network). *Cited:* `foundry.toml`, `contracts/README.md`, `contracts/hardhat.config.js:30-35`.
- **Sei network targets (supply these — they're NOT in the in-repo tooling config):** mainnet `pacific-1` chain id **1329**, RPC `https://evm-rpc.sei-apis.com`; testnet `atlantic-2` chain id **1328**, RPC `https://evm-rpc-testnet.sei-apis.com`. *Cited:* `x/evm/config/config.go`; sei-docs `learn/dev-chains.mdx`. *(The repo's own Foundry/Hardhat config only carries `seilocal`/`arctic-1` — mainnet/testnet endpoints are author-supplied.)*

## 3. Anti-patterns / failure modes

- **Happy-path-only tests.** Cue: a test suite with no fuzz/invariant tests and no `forge coverage` gate. Rewrite: add fuzz + invariant tests for state machines and money flows; gate coverage in CI. *Cited:* `sources.md` §foundry.
- **No fork test against Sei (precompiles make this non-optional).** Cue: tests that mock precompiles/oracles instead of forking a Sei RPC. **The precompiles only exist on a real Sei node — local `anvil` has none** — so a fork test against a Sei RPC is the *only* way to exercise precompile-calling code; mocking it tests a fiction. Rewrite: `forge test --fork-url <sei-rpc>` against the real chain id (1329/1328). *Cited:* `sources.md` §foundry; profile §1, §9.
- **No static analysis in CI.** Cue: no Slither/Aderyn step. Rewrite: run Slither (and Aderyn) in CI; triage by Impact+Confidence; escalate real exploitability to `security-specialist`. *Cited:* `sources.md` §trailofbits.
- **Deploy without verification.** Cue: a `forge script` deploy with no `forge verify-contract`. Rewrite: verify on `seiscan.io` so the source is public and indexers/agents can decode it. *Cited:* `sources.md` §foundry.
- **Hard-coding RPC/chain id from L1 habit.** Cue: an Ethereum mainnet RPC / chain id 1 in a Sei deploy script. Rewrite: use the Sei RPC + chain id (1329/1328) as a parameter, not a literal. *Cited:* profile §2 (one-way door on chain-id coupling).

## 4. Review cues

- **Dimension 6 (testing & verification adequacy):** fuzz + invariant + fork tests present; `forge coverage` gated; Slither/Aderyn in CI; deploy script pairs with `forge verify-contract`; fork target is a Sei RPC + chain id. *Basis:* `sources.md` §foundry, §trailofbits; profile §1.
- **Dimension 5 (gas & efficiency):** gas snapshots (`forge snapshot`) inform changes; no hard-coded gas constants (estimate at runtime). *Basis:* profile §2.

## 5. One-way doors in this concern

- **Deployed + verified contract address** is a published interface consumers/indexers depend on — re-deploying to a new address breaks them; treat a redeploy (vs an in-place upgrade) as a one-way door, flag it.
- **The deploy network/chain id** baked into a script targets mainnet vs testnet — a prod deploy is irreversible; flag before running a `pacific-1` deploy.
