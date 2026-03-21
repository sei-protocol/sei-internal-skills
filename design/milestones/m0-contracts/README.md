# Milestone 0 — Smart Contracts

**Owner:** Blockchain Developer
**Phase:** 0
**Dependencies:** None (first milestone)

## Scope

Deploy the on-chain governance and execution primitives on Sei EVM:

1. **TideCouncil** — Design proposal lifecycle, EIP-712 agent reviews, quorum-based approval, UUPS upgradeability
2. **TideJobHook** — ERC-8183 `IACPHook` implementation for job escrow, sandbox provisioning events, reputation updates
3. **Contract Deployment Suite** — Foundry scripts for deploy, verify, and upgrade flows with mock contracts for testing

## Deliverables

| Spec | Output |
|------|--------|
| `lld-tide-council.md` | `TideCouncil.sol`, `ITideCouncil.sol`, Foundry test suite (49 tests) |
| `lld-tide-job-hook.md` | `TideJobHook.sol`, Foundry test suite |
| `lld-contract-deployment.md` | `Deploy.s.sol`, `Verify.s.sol`, `Upgrade.s.sol`, mock contracts |

## Done Criteria

- Contracts deployed to Sei arctic-1 testnet
- All Foundry tests passing
- Event signatures verified against Operator's topic hash constants
- ABI JSON exported for Operator and runtime consumption
