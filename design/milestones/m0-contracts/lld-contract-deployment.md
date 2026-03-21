# Component: Contract Deployment Suite

## Owner

Blockchain Developer

## Phase

0

## Purpose

This document specifies the Foundry project structure, deployment scripts, and verification procedures for deploying all Tide smart contracts on Sei. The suite handles deploying ERC-8004 registries, ERC-8183 AgenticCommerce, TideCouncil (UUPS proxy), and TideJobHook — then wiring them together with correct initialization parameters and post-deployment authorization grants.

**Business needs served:** All five Phase 0-2 business needs depend on deployed, verified, and correctly initialized contracts. This component is the foundation for everything else.

---

## Dependencies

### External

| Dependency | Version | Notes |
|---|---|---|
| **Foundry (forge, cast, anvil)** | Latest stable | Build, test, deploy, verify. |
| **OpenZeppelin Contracts** | v5.x | Installed via `forge install`. Provides UUPS, Ownable, Pausable, ERC1967Proxy. |
| **OpenZeppelin Contracts Upgradeable** | v5.x | Installed via `forge install`. Provides upgradeable base contracts for TideCouncil. |
| **forge-std** | Latest | Test utilities, `vm` cheatcodes, `Script` base. |
| **Sei EVM RPC** | — | `https://evm-rpc-arctic-1.sei-apis.com` (testnet), `https://evm-rpc.sei-apis.com` (mainnet). |
| **SeiTrace (SeiScan)** | — | Block explorer with contract verification API. |

### Internal

| Dependency | Interface | Notes |
|---|---|---|
| **TideCouncil** | `lld-tide-council.md` | Contract source deployed by this suite. |
| **TideJobHook** | `lld-tide-job-hook.md` | Contract source deployed by this suite. |

### Explicit Exclusions

- **Does NOT deploy** the Tide Operator or any K8s resources.
- **Does NOT provision** GitHub Apps, KMS keys, or AWS resources.
- **Does NOT handle** ERC-8004 agent identity minting (that's Phase 1 task 1.1).

---

## Interface Specification

The deployment suite's "interface" is the set of deployment scripts, their input parameters, and their deterministic outputs.

### Deployment Script: `Deploy.s.sol`

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Script, console2} from "forge-std/Script.sol";
import {ERC1967Proxy} from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import {TideCouncil} from "../src/TideCouncil.sol";
import {TideJobHook} from "../src/TideJobHook.sol";

/// @title Deploy
/// @notice Deploys all Tide contracts in a single broadcast session.
///         Reads configuration from environment variables.
/// @dev Usage:
///   forge script script/Deploy.s.sol:Deploy \
///     --rpc-url $RPC_URL \
///     --broadcast \
///     --verify \
///     --verifier blockscout \
///     --verifier-url $VERIFIER_URL \
///     --private-key $DEPLOYER_KEY
contract Deploy is Script {

    // ── Environment Variables (required) ───────────────────

    // Pre-deployed dependency addresses
    // address IDENTITY_REGISTRY  — ERC-8004 IdentityRegistry
    // address REPUTATION_REGISTRY — ERC-8004 ReputationRegistry
    // address ACP_CONTRACT        — ERC-8183 AgenticCommerce

    // TideCouncil initialization parameters
    // address COUNCIL_OWNER       — Owner (upgrades + emergencyRevokeAgent)
    // address COUNCIL_PAUSER      — Pauser (pause/unpause)
    // uint40  DEFAULT_TTL         — Default proposal TTL in seconds
    // uint8   DEFAULT_QUORUM      — Default quorum

    // TideJobHook constructor parameters
    // address HOOK_OWNER          — Hook owner
    // address HOOK_PAUSER         — Hook pauser

    function run() external {
        // ── Read Configuration ─────────────────────────────

        address identityRegistry = vm.envAddress("IDENTITY_REGISTRY");
        address reputationRegistry = vm.envAddress("REPUTATION_REGISTRY");
        address acpContract = vm.envAddress("ACP_CONTRACT");

        address councilOwner = vm.envAddress("COUNCIL_OWNER");
        address councilPauser = vm.envAddress("COUNCIL_PAUSER");
        uint40 defaultTTL = uint40(vm.envUint("DEFAULT_TTL"));
        uint8 defaultQuorum = uint8(vm.envUint("DEFAULT_QUORUM"));

        address hookOwner = vm.envAddress("HOOK_OWNER");
        address hookPauser = vm.envAddress("HOOK_PAUSER");

        // ── Deploy ─────────────────────────────────────────

        vm.startBroadcast();

        // Step 1: Deploy TideCouncil implementation
        TideCouncil councilImpl = new TideCouncil();
        console2.log("TideCouncil implementation:", address(councilImpl));

        // Step 2: Deploy TideCouncil proxy with initialize calldata
        bytes memory councilInitData = abi.encodeCall(
            TideCouncil.initialize,
            (councilOwner, identityRegistry, councilPauser, defaultTTL, defaultQuorum)
        );
        ERC1967Proxy councilProxy = new ERC1967Proxy(
            address(councilImpl),
            councilInitData
        );
        console2.log("TideCouncil proxy:", address(councilProxy));

        // Step 3: Deploy TideJobHook
        TideJobHook hook = new TideJobHook(
            acpContract,
            identityRegistry,
            reputationRegistry,
            hookOwner,
            hookPauser
        );
        console2.log("TideJobHook:", address(hook));

        vm.stopBroadcast();

        // ── Output Summary ─────────────────────────────────

        console2.log("\n=== Deployment Summary ===");
        console2.log("TideCouncil implementation:", address(councilImpl));
        console2.log("TideCouncil proxy:         ", address(councilProxy));
        console2.log("TideJobHook:               ", address(hook));
        console2.log("\n=== Post-Deployment Steps ===");
        console2.log("1. Verify all contracts on SeiTrace");
        console2.log("2. Register TideJobHook with ACP (whitelist or per-job)");
        console2.log("3. Authorize TideJobHook on ReputationRegistry");
        console2.log("4. Run post-deployment verification script");
    }
}
```

### Verification Script: `Verify.s.sol`

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Script, console2} from "forge-std/Script.sol";
import {ITideCouncil} from "../src/interfaces/ITideCouncil.sol";
import {ITideJobHook} from "../src/interfaces/ITideJobHook.sol";

/// @title Verify
/// @notice Post-deployment verification script. Reads deployed addresses from env
///         and runs a battery of view calls to confirm correct initialization.
/// @dev Usage:
///   forge script script/Verify.s.sol:Verify \
///     --rpc-url $RPC_URL
contract Verify is Script {

    function run() external view {
        address councilProxy = vm.envAddress("COUNCIL_PROXY");
        address hookAddr = vm.envAddress("HOOK_ADDRESS");
        address expectedOwner = vm.envAddress("COUNCIL_OWNER");
        address expectedPauser = vm.envAddress("COUNCIL_PAUSER");
        address expectedIdentityRegistry = vm.envAddress("IDENTITY_REGISTRY");
        address expectedACP = vm.envAddress("ACP_CONTRACT");

        ITideCouncil council = ITideCouncil(councilProxy);
        ITideJobHook hook = ITideJobHook(hookAddr);

        // ── TideCouncil Checks ─────────────────────────────

        console2.log("=== TideCouncil Verification ===");

        // Owner
        address actualOwner = council.owner();
        require(actualOwner == expectedOwner, "Council owner mismatch");
        console2.log("Owner:              OK", actualOwner);

        // Proposal count (should be 0)
        uint256 count = council.proposalCount();
        require(count == 0, "Proposal count should be 0");
        console2.log("Proposal count:     OK (0)");

        // Not paused
        bool paused = council.paused();
        require(!paused, "Council should not be paused");
        console2.log("Paused:             OK (false)");

        // ── TideJobHook Checks ─────────────────────────────

        console2.log("\n=== TideJobHook Verification ===");

        // ACP address
        address actualACP = hook.acpContract();
        require(actualACP == expectedACP, "ACP address mismatch");
        console2.log("ACP:                OK", actualACP);

        // Identity registry
        address actualIR = hook.identityRegistry();
        require(actualIR == expectedIdentityRegistry, "Identity registry mismatch");
        console2.log("IdentityRegistry:   OK", actualIR);

        // Min reputation score (should be 0)
        uint256 minRep = hook.minReputationScore();
        require(minRep == 0, "Min reputation should be 0");
        console2.log("MinReputation:      OK (0)");

        // Completion score (should be 80)
        uint8 compScore = hook.completionScore();
        require(compScore == 80, "Completion score should be 80");
        console2.log("CompletionScore:    OK (80)");

        // Rejection score (should be 20)
        uint8 rejScore = hook.rejectionScore();
        require(rejScore == 20, "Rejection score should be 20");
        console2.log("RejectionScore:     OK (20)");

        // Not paused
        bool hookPaused = hook.paused();
        require(!hookPaused, "Hook should not be paused");
        console2.log("Paused:             OK (false)");

        console2.log("\n=== All Checks Passed ===");
    }
}
```

### Environment Configuration Files

#### `.env.arctic` (testnet)

```bash
# Sei Testnet (arctic-1)
RPC_URL=https://evm-rpc-arctic-1.sei-apis.com
CHAIN_ID=713715
VERIFIER_URL=https://seitrace.com/arctic-1/api

# Pre-deployed dependencies (fill after ERC-8004/8183 deployment)
IDENTITY_REGISTRY=0x...
REPUTATION_REGISTRY=0x...
ACP_CONTRACT=0x...

# TideCouncil initialization
COUNCIL_OWNER=0x...    # deployer EOA for testnet
COUNCIL_PAUSER=0x...   # deployer EOA for testnet
DEFAULT_TTL=7200       # 2 hours (fast iteration on testnet)
DEFAULT_QUORUM=2

# TideJobHook initialization
HOOK_OWNER=0x...       # deployer EOA for testnet
HOOK_PAUSER=0x...      # deployer EOA for testnet

# Deployer
DEPLOYER_KEY=0x...     # private key (NEVER commit to git)
```

#### `.env.pacific` (mainnet)

```bash
# Sei Mainnet (pacific-1)
RPC_URL=https://evm-rpc.sei-apis.com
CHAIN_ID=1329
VERIFIER_URL=https://seitrace.com/pacific-1/api

# Pre-deployed dependencies
IDENTITY_REGISTRY=0x...
REPUTATION_REGISTRY=0x...
ACP_CONTRACT=0x...

# TideCouncil initialization
COUNCIL_OWNER=0x...    # 2-of-3 multisig
COUNCIL_PAUSER=0x...   # 2-of-3 multisig (can be same as owner)
DEFAULT_TTL=259200     # 72 hours
DEFAULT_QUORUM=2

# TideJobHook initialization
HOOK_OWNER=0x...       # multisig
HOOK_PAUSER=0x...      # 2-of-3 multisig

# Deployer
DEPLOYER_KEY=0x...     # private key (NEVER commit to git)
```

---

## State Model

The deployment suite itself is stateless. It produces on-chain state (deployed contracts) and local artifacts (broadcast logs).

### Deployment Artifacts

Foundry stores deployment artifacts in `broadcast/`:

```
broadcast/
└── Deploy.s.sol/
    ├── 713715/          # arctic-1 testnet
    │   └── run-latest.json
    └── 1329/            # pacific-1 mainnet
        └── run-latest.json
```

Each `run-latest.json` contains:
- Transaction hashes for each deployment step
- Deployed contract addresses
- Constructor arguments (for verification)
- Gas used

These artifacts are the source of truth for deployed addresses and must be committed to the repository.

### Deployed Contract Registry

After deployment, update `src/TideConstants.sol` with the actual addresses:

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @notice Shared constants for all Tide contracts.
/// @dev Addresses are set after deployment. Testnet and mainnet values differ.
library TideConstants {
    // ── Sei USDC ──────────────────────────────────────────
    address constant USDC = 0xe15fC38F6D8c56aF07bbCBe3BAf5708A2Bf42392;

    // ── ERC-8183 Action Selectors ─────────────────────────
    bytes4 constant FUND_SELECTOR = bytes4(keccak256("fund(uint256)"));
    bytes4 constant SUBMIT_SELECTOR = bytes4(keccak256("submit(uint256,bytes32)"));
    bytes4 constant COMPLETE_SELECTOR = bytes4(keccak256("complete(uint256)"));
    bytes4 constant REJECT_SELECTOR = bytes4(keccak256("reject(uint256)"));

    // ── Reputation Tags ───────────────────────────────────
    bytes32 constant TAG_JOB_COMPLETE = keccak256("job-complete");
    bytes32 constant TAG_JOB_REJECTED = keccak256("job-rejected");

    // ── TideCouncil TTL Bounds ────────────────────────────
    uint40 constant MIN_TTL = 3600;       // 1 hour
    uint40 constant MAX_TTL = 2_592_000;  // 30 days
}
```

---

## Internal Design

### Project Structure

```
tide-contracts/
├── foundry.toml
├── remappings.txt
├── .gitignore
├── .env.arctic              # testnet config (gitignored, template committed)
├── .env.pacific             # mainnet config (gitignored, template committed)
├── .env.arctic.template     # committed template without secrets
├── .env.pacific.template    # committed template without secrets
│
├── src/
│   ├── TideCouncil.sol      # UUPS upgradeable council contract
│   ├── TideJobHook.sol      # ERC-8183 hook contract
│   ├── TideConstants.sol    # Shared constants library
│   └── interfaces/
│       ├── ITideCouncil.sol
│       ├── ITideJobHook.sol
│       ├── IACPHook.sol     # ERC-8183 hook interface
│       ├── IACP.sol         # ERC-8183 ACP read interface
│       ├── IIdentityRegistry.sol   # ERC-8004 identity
│       └── IReputationRegistry.sol # ERC-8004 reputation
│
├── test/
│   ├── TideCouncil.t.sol
│   ├── TideJobHook.t.sol
│   ├── Deploy.t.sol         # Deployment script tests
│   └── mocks/
│       ├── MockIdentityRegistry.sol
│       ├── MockReputationRegistry.sol
│       └── MockACP.sol
│
├── script/
│   ├── Deploy.s.sol         # Main deployment script
│   ├── Verify.s.sol         # Post-deployment verification
│   └── Upgrade.s.sol        # TideCouncil UUPS upgrade script
│
├── lib/                     # Managed by forge install
│   ├── forge-std/
│   ├── openzeppelin-contracts/
│   └── openzeppelin-contracts-upgradeable/
│
└── broadcast/               # Deployment artifacts (auto-generated)
    └── Deploy.s.sol/
        ├── 713715/
        └── 1329/
```

### foundry.toml

```toml
[profile.default]
src = "src"
out = "out"
libs = ["lib"]
solc = "0.8.24"
evm_version = "shanghai"
optimizer = true
optimizer_runs = 200
via_ir = false
ffi = false
fs_permissions = [{ access = "read", path = "./"}]

[profile.default.fmt]
line_length = 120
tab_width = 4
bracket_spacing = false

# Gas reporting for deployment cost estimation
[profile.default.fuzz]
runs = 256

[rpc_endpoints]
arctic = "https://evm-rpc-arctic-1.sei-apis.com"
pacific = "https://evm-rpc.sei-apis.com"
localhost = "http://127.0.0.1:8545"

[etherscan]
arctic = { key = "", chain = 713715, url = "https://seitrace.com/arctic-1/api" }
pacific = { key = "", chain = 1329, url = "https://seitrace.com/pacific-1/api" }
```

### remappings.txt

```
@openzeppelin/contracts/=lib/openzeppelin-contracts/contracts/
@openzeppelin/contracts-upgradeable/=lib/openzeppelin-contracts-upgradeable/contracts/
forge-std/=lib/forge-std/src/
```

### .gitignore

```
# Secrets
.env.arctic
.env.pacific
.env

# Build artifacts
out/
cache/

# Broadcast logs contain transaction hashes — commit selectively
# broadcast/

# Dependencies (managed by forge)
lib/
```

### Deployment Order

The deployment script executes in this exact order within a single `vm.startBroadcast()` / `vm.stopBroadcast()` block:

```
Step 1: Deploy TideCouncil implementation
        └─ No constructor args (logic contract)
        └─ Output: councilImpl address

Step 2: Deploy ERC1967Proxy for TideCouncil
        └─ Constructor args: (councilImpl, initializeCalldata)
        └─ initialize() is called atomically in the proxy constructor
        └─ Output: councilProxy address (this is the canonical TideCouncil address)

Step 3: Deploy TideJobHook
        └─ Constructor args: (acpContract, identityRegistry, reputationRegistry, owner, pauser)
        └─ Output: hookAddress
```

**Pre-requisite:** ERC-8004 and ERC-8183 contracts must be deployed first, with their addresses set in environment variables. If they are not yet deployed, use the reference implementations:

```bash
# Clone and deploy ERC-8004 (approximate — adapt to actual repo structure)
git clone https://github.com/erc-8004/erc-8004-contracts
cd erc-8004-contracts
forge script script/Deploy.s.sol --rpc-url $RPC_URL --broadcast

# Clone and deploy ERC-8183 (approximate)
git clone https://github.com/erc-8183/base-contracts
cd base-contracts
forge script script/Deploy.s.sol --rpc-url $RPC_URL --broadcast
```

The exact deployment commands for ERC-8004 and ERC-8183 depend on their repository structure. Document the actual commands after cloning.

### Post-Deployment Steps (manual or scripted)

After `Deploy.s.sol` succeeds:

| Step | Command | Purpose |
|---|---|---|
| 1 | Run `Verify.s.sol` | Confirm all contracts initialized correctly |
| 2 | Verify on SeiTrace | Source code verification for public auditability |
| 3 | Register hook with ACP | Call ACP's hook registration function (if applicable) |
| 4 | Authorize hook on ReputationRegistry | Grant `submitFeedback` permission to hook address |
| 5 | Commit broadcast artifacts | Store deployment addresses and tx hashes in git |
| 6 | Update TideConstants.sol | Fill in deployed addresses for downstream consumers |

### Contract Verification on SeiTrace

SeiTrace (Sei's block explorer) supports Blockscout-compatible verification.

```bash
# Verify TideCouncil implementation
forge verify-contract $COUNCIL_IMPL_ADDRESS \
  src/TideCouncil.sol:TideCouncil \
  --chain-id $CHAIN_ID \
  --verifier blockscout \
  --verifier-url $VERIFIER_URL

# Verify TideCouncil proxy
# Note: ERC1967Proxy verification requires constructor args
forge verify-contract $COUNCIL_PROXY_ADDRESS \
  lib/openzeppelin-contracts/contracts/proxy/ERC1967/ERC1967Proxy.sol:ERC1967Proxy \
  --chain-id $CHAIN_ID \
  --verifier blockscout \
  --verifier-url $VERIFIER_URL \
  --constructor-args $(cast abi-encode "constructor(address,bytes)" $COUNCIL_IMPL_ADDRESS $COUNCIL_INIT_DATA)

# Verify TideJobHook
forge verify-contract $HOOK_ADDRESS \
  src/TideJobHook.sol:TideJobHook \
  --chain-id $CHAIN_ID \
  --verifier blockscout \
  --verifier-url $VERIFIER_URL \
  --constructor-args $(cast abi-encode "constructor(address,address,address,address,address)" \
    $ACP_CONTRACT $IDENTITY_REGISTRY $REPUTATION_REGISTRY $HOOK_OWNER $HOOK_PAUSER)
```

**SeiTrace verification quirks:**
- SeiTrace may require the `--compiler-version` flag if auto-detection fails.
- For UUPS proxies, verify both the implementation and the proxy separately.
- The proxy contract verification requires constructor args that include the initialize calldata.

### Upgrade Script: `Upgrade.s.sol`

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Script, console2} from "forge-std/Script.sol";
import {TideCouncil} from "../src/TideCouncil.sol";

/// @title Upgrade
/// @notice Deploys a new TideCouncil implementation and upgrades the proxy.
/// @dev Must be run by the TideCouncil owner.
///      Usage:
///        forge script script/Upgrade.s.sol:Upgrade \
///          --rpc-url $RPC_URL \
///          --broadcast \
///          --private-key $OWNER_KEY
contract Upgrade is Script {

    function run() external {
        address councilProxy = vm.envAddress("COUNCIL_PROXY");

        vm.startBroadcast();

        // Deploy new implementation
        TideCouncil newImpl = new TideCouncil();
        console2.log("New implementation:", address(newImpl));

        // Upgrade proxy (owner-only)
        TideCouncil(councilProxy).upgradeToAndCall(address(newImpl), "");
        console2.log("Proxy upgraded to:", address(newImpl));

        vm.stopBroadcast();

        // Verify state preserved
        TideCouncil council = TideCouncil(councilProxy);
        console2.log("Proposal count after upgrade:", council.proposalCount());
    }
}
```

### Mock Contracts for Testing

#### MockIdentityRegistry

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {ERC721Enumerable, ERC721} from "@openzeppelin/contracts/token/ERC721/extensions/ERC721Enumerable.sol";

/// @notice Minimal ERC-721 Enumerable mock for testing TideCouncil and TideJobHook.
contract MockIdentityRegistry is ERC721Enumerable {
    constructor() ERC721("MockIdentity", "MID") {}

    function mint(address to, uint256 tokenId) external {
        _mint(to, tokenId);
    }

    function burn(uint256 tokenId) external {
        _burn(tokenId);
    }
}
```

#### MockReputationRegistry

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @notice Mock ReputationRegistry that records submitFeedback calls for test assertions.
contract MockReputationRegistry {
    struct FeedbackRecord {
        uint256 tokenId;
        uint8 score;
        bytes32[] tags;
    }

    FeedbackRecord[] public feedbackHistory;
    mapping(uint256 => uint8) public averageScores;

    function submitFeedback(
        uint256 tokenId,
        uint8 score,
        bytes32[] calldata tags
    ) external {
        feedbackHistory.push();
        FeedbackRecord storage record = feedbackHistory[feedbackHistory.length - 1];
        record.tokenId = tokenId;
        record.score = score;
        for (uint256 i = 0; i < tags.length; i++) {
            record.tags.push(tags[i]);
        }
    }

    function setAverageScore(uint256 tokenId, uint8 score) external {
        averageScores[tokenId] = score;
    }

    function getAverageScore(uint256 tokenId) external view returns (uint8) {
        return averageScores[tokenId];
    }

    function feedbackCount() external view returns (uint256) {
        return feedbackHistory.length;
    }

    function getFeedback(uint256 index) external view returns (
        uint256 tokenId, uint8 score, bytes32[] memory tags
    ) {
        FeedbackRecord storage record = feedbackHistory[index];
        return (record.tokenId, record.score, record.tags);
    }
}
```

#### MockACP

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {IACPHook} from "../interfaces/IACPHook.sol";

/// @notice Mock ERC-8183 ACP that stores jobs and can call hook functions.
contract MockACP {
    enum JobStatus { Open, Funded, Submitted, Completed, Rejected, Expired }

    struct Job {
        address client;
        address provider;
        address evaluator;
        address hook;
        uint256 budget;
        uint256 expiry;
        string description;
        JobStatus status;
    }

    mapping(uint256 => Job) public jobs;

    function setJob(uint256 jobId, Job memory job) external {
        jobs[jobId] = job;
    }

    function getJob(uint256 jobId) external view returns (Job memory) {
        return jobs[jobId];
    }

    function callBeforeAction(
        address hook,
        uint256 jobId,
        bytes4 selector,
        bytes calldata data
    ) external {
        IACPHook(hook).beforeAction(jobId, selector, data);
    }

    function callAfterAction(
        address hook,
        uint256 jobId,
        bytes4 selector,
        bytes calldata data
    ) external {
        IACPHook(hook).afterAction(jobId, selector, data);
    }
}
```

---

## Error Handling

| Error | Cause | Detection | Action |
|---|---|---|---|
| Deployment script reverts | Insufficient gas, wrong constructor args, or dependency not deployed | Forge error output | Check env vars, verify dependency addresses exist on-chain via `cast code <addr>` |
| `initialize()` reverts | Invalid parameters (e.g., zero address for identityRegistry) | Transaction receipt | Check initialization parameters in .env file |
| Verification fails on SeiTrace | Wrong compiler version, optimizer settings mismatch, or constructor args mismatch | Verification API response | Match `foundry.toml` settings exactly; use `--compiler-version` flag |
| Proxy points to wrong implementation | Wrong address passed to ERC1967Proxy constructor | `cast storage <proxy> 0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc` | Redeploy or upgrade |
| Post-deployment verify script fails | Contract initialized with wrong values | `Verify.s.sol` output | Check .env and redeploy if on testnet; for mainnet, use admin functions to correct |
| Hook not authorized on ReputationRegistry | Missing post-deployment step | `afterAction(complete)` reverts | Run authorization step (grant role on ReputationRegistry) |
| Nonce collision during deployment | Deployer sent a transaction outside of the script | Forge nonce error | Use `--resume` flag or manually sync nonce with `cast nonce` |

---

## Test Specification

### Deployment Script Tests

These tests run the deployment script against a local Anvil fork and verify the result.

```solidity
contract DeployTest is Test {
    function setUp() public {
        // Fork arctic-1 testnet (or use anvil with mock dependencies)
    }
}
```

| # | Test | Setup | Action | Assertion |
|---|---|---|---|---|
| 1 | `test_deploy_allContractsDeployed` | Set env vars with mock dependency addresses | Run `Deploy.s.sol` on Anvil | All 3 addresses are non-zero. Code exists at each address. |
| 2 | `test_deploy_councilInitializedCorrectly` | Run deploy | Read TideCouncil proxy state | `owner()` matches env. `proposalCount() == 0`. `paused() == false`. |
| 3 | `test_deploy_hookInitializedCorrectly` | Run deploy | Read TideJobHook state | `acpContract()` matches env. `completionScore() == 80`. `rejectionScore() == 20`. |
| 4 | `test_deploy_councilProxyPointsToImpl` | Run deploy | Read ERC-1967 implementation slot | Slot value matches deployed implementation address. |
| 5 | `test_deploy_councilCanBeUpgraded` | Run deploy | Deploy new impl, call `upgradeToAndCall` as owner | Implementation slot updated. State preserved. |
| 6 | `test_deploy_hookImmutablesCorrect` | Run deploy | Read hook immutables | `acpContract`, `identityRegistry`, `reputationRegistry` match constructor args. |

### Testnet vs Mainnet Configuration Tests

| # | Test | Setup | Action | Assertion |
|---|---|---|---|---|
| 7 | `test_config_testnetTTL` | Load `.env.arctic` | Check `DEFAULT_TTL` | 7200 (2 hours). |
| 8 | `test_config_mainnetTTL` | Load `.env.pacific` | Check `DEFAULT_TTL` | 259200 (72 hours). |
| 9 | `test_config_noSecretsInTemplates` | Read `.env.arctic.template` | Grep for private key patterns | No `0x` prefixed 64-char hex strings found. |

### Mock Contract Tests

| # | Test | Setup | Action | Assertion |
|---|---|---|---|---|
| 10 | `test_mockIdentityRegistry_mintAndQuery` | Deploy MockIdentityRegistry | Mint token 1 to addr A | `ownerOf(1) == A`. `balanceOf(A) == 1`. `tokenOfOwnerByIndex(A, 0) == 1`. |
| 11 | `test_mockReputationRegistry_recordsFeedback` | Deploy MockReputationRegistry | Call `submitFeedback(1, 80, [tag])` | `feedbackCount() == 1`. `getFeedback(0)` returns correct values. |
| 12 | `test_mockACP_storesAndReturnsJob` | Deploy MockACP | Set job 1, call `getJob(1)` | Returns correct struct. |

### End-to-End Integration Test

| # | Test | Setup | Action | Assertion |
|---|---|---|---|---|
| 13 | `test_e2e_proposalLifecycle` | Deploy all contracts on Anvil. Mint 3 agent identities. | Create proposal → 3 agents submit reviews (2 approve, 1 request changes) → finalize | `ProposalApproved` emitted. Proposal status == Approved. |
| 14 | `test_e2e_hookFundingGate` | Deploy all contracts. Register agent with identity. | Fund a job via MockACP (calls hook) | `beforeAction(fund)` succeeds. `SandboxProvisionRequested` emitted from `afterAction(fund)`. |
| 15 | `test_e2e_hookReputationFlow` | Deploy all contracts. Fund job. | Complete job via MockACP | `afterAction(complete)` posts feedback to MockReputationRegistry. Score=80, tag=TAG_JOB_COMPLETE. |

---

## Deployment

### Deployment Runbook

#### Testnet (arctic-1)

```bash
# 1. Install dependencies
forge install OpenZeppelin/openzeppelin-contracts@v5.1.0
forge install OpenZeppelin/openzeppelin-contracts-upgradeable@v5.1.0
forge install foundry-rs/forge-std

# 2. Build and test
forge build
forge test -vvv

# 3. Configure environment
cp .env.arctic.template .env.arctic
# Edit .env.arctic: fill in dependency addresses and deployer key

# 4. Deploy ERC-8004 + ERC-8183 (if not already deployed)
# (follow ERC-8004 and ERC-8183 repo instructions)

# 5. Deploy Tide contracts
source .env.arctic
forge script script/Deploy.s.sol:Deploy \
  --rpc-url $RPC_URL \
  --broadcast \
  --verify \
  --verifier blockscout \
  --verifier-url $VERIFIER_URL \
  --private-key $DEPLOYER_KEY

# 6. Record deployed addresses from console output

# 7. Run verification
COUNCIL_PROXY=0x... HOOK_ADDRESS=0x... \
forge script script/Verify.s.sol:Verify --rpc-url $RPC_URL

# 8. Post-deployment authorization steps
# (manual — depends on ERC-8004/8183 admin interfaces)
```

#### Mainnet (pacific-1)

Same steps as testnet with these differences:

| Aspect | Testnet | Mainnet |
|---|---|---|
| Config file | `.env.arctic` | `.env.pacific` |
| Owner/Pauser | Deployer EOA | 2-of-3 multisig address |
| DEFAULT_TTL | 7200 (2h) | 259200 (72h) |
| Verification URL | `seitrace.com/arctic-1/api` | `seitrace.com/pacific-1/api` |
| Chain ID | 713715 | 1329 |
| USDC | Mock ERC-20 or testnet faucet | `0xe15fC38F6D8c56aF07bbCBe3BAf5708A2Bf42392` |
| Pre-deploy review | Optional | **Required** — review bytecode, check constructor args, dry-run with `--dry-run` flag |

**Mainnet deployment requires:**
1. All testnet E2E tests pass on arctic-1
2. Security review checkpoint (Phase 0.7 in the implementation plan)
3. Dry-run with `forge script ... --dry-run` to estimate gas and verify calldata
4. Deployer has sufficient SEI for gas (~0.5 SEI should be ample)
5. Multisig addresses are created and signers confirmed

### Deployment Checklist

```
Pre-Deployment:
[ ] All forge tests pass (forge test -vvv)
[ ] ERC-8004 IdentityRegistry deployed and address recorded
[ ] ERC-8004 ReputationRegistry deployed and address recorded
[ ] ERC-8183 ACP deployed and address recorded
[ ] Verify IACP.getJob() is available: cast call $ACP "getJob(uint256)(tuple)" 0 --rpc-url $RPC_URL
[ ] Verify action selectors match ACP ABI:
    cast sig "fund(uint256)" should match FUND_SELECTOR
    cast sig "complete(uint256)" should match COMPLETE_SELECTOR
    cast sig "reject(uint256)" should match REJECT_SELECTOR
[ ] .env file populated (no placeholder values)
[ ] Deployer wallet has sufficient SEI for gas

Deployment:
[ ] forge script Deploy.s.sol --broadcast succeeds
[ ] Console output shows 3 non-zero addresses
[ ] Transaction hashes recorded from broadcast/

Post-Deployment:
[ ] Verify.s.sol passes all checks
[ ] All 3 contracts verified on SeiTrace
[ ] TideJobHook authorized on ReputationRegistry
[ ] TideJobHook registered with ACP (if required)
[ ] Deployed addresses committed to TideConstants.sol
[ ] broadcast/ artifacts committed to git
[ ] Smoke test: create and finalize a proposal on testnet
[ ] Smoke test: fund a job and verify SandboxProvisionRequested event
```

---

## Deferred (Do Not Build)

| Feature | Rationale |
|---|---|
| **CREATE2 deterministic deployment** | Deterministic addresses are useful for cross-chain deployments. Sei-only for Phase 0-2 — standard CREATE is sufficient. |
| **Deployment via multisig (Gnosis Safe)** | Testnet uses EOA deployer. Mainnet multisig ownership is transferred post-deploy. Deploying *through* a multisig adds tooling complexity without Phase 0-2 value. |
| **Automated CI/CD deployment pipeline** | Manual deployment with the runbook is sufficient for 2 environments (testnet + mainnet). GitHub Actions pipeline is deferred. |
| **Gas optimization report** | `forge test --gas-report` provides basic data. Formal gas optimization (assembly, storage packing beyond current design) is deferred. |
| **Multi-chain deployment** | Sei-only for Phase 0-2. Multi-chain support is explicitly deferred in the constitution. |
| **Deployment to Sei devnet** | Two environments (arctic-1 + pacific-1) are sufficient. A third devnet environment adds maintenance overhead without value. |
| **Foundry coverage enforcement** | `forge coverage` can be run manually. CI enforcement of coverage thresholds is deferred. |
| **Upgrade governance (timelock)** | Owner directly calls `upgradeToAndCall`. A timelock adds delay for upgrades, useful for production but not required for Phase 0-2 with a small trusted team. |
