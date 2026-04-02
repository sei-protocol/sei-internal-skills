# MVP Contract Deployment and Agent Wallet Setup

## Owner

Blockchain Developer

## Phase

MVP (pre-Phase 1) -- arctic-1 testnet only

## Purpose

This document specifies the minimum viable on-chain deployment needed to support the Tekton-driven agent review loop on Sei's arctic-1 testnet (chain ID 713715). The loop is:

1. Brandon (principal) proposes a design via `propose()`
2. Council agents review via EIP-712 signed attestations submitted through `submitReview()`
3. Quorum is reached, anyone calls `finalize()` to approve on-chain
4. Operator watches `ProposalApproved` events and triggers execution jobs

This document covers: which contracts to deploy, which to stub, how to set up agent wallets and identities, the exact Foundry deployment script, and the one-way door inventory.

**Business needs served:**
1. Present a design to a council of agents and collect structured feedback (Phase 1)
2. Reach quorum consensus and attest the approved design on-chain (Phase 1)

---

## 1. MVP Contract Scope

### What We Deploy

| Contract | Deploy? | Justification |
|---|---|---|
| **TideCouncil** (UUPS proxy) | Yes -- full implementation | Core of the review loop. All functions from the LLD are needed. |
| **MockIdentityRegistry** (ERC-721 Enumerable) | Yes -- mock, not reference ERC-8004 | Agents need NFT identities for EIP-712 signing verification. A simple ERC-721 Enumerable with an open `mint()` function is sufficient. Deploying the full ERC-8004 reference implementation adds dependencies we do not need for MVP. |

### What We Skip

| Contract | Deploy? | Justification |
|---|---|---|
| **TideJobHook** | No -- Phase 2 | The MVP loop ends at `ProposalApproved`. Job funding, sandbox provisioning, and USDC escrow are Phase 2 features. The Operator can trigger execution jobs off-chain without on-chain escrow for MVP. |
| **ERC-8183 AgenticCommerce (ACP)** | No -- Phase 2 | TideJobHook's dependency. Not needed until we have USDC-escrowed jobs. |
| **ERC-8004 ReputationRegistry** | No -- Phase 3+ | Reputation gating is explicitly deferred in the constitution. |
| **ERC-8004 reference IdentityRegistry** | No -- overkill for MVP | The reference ERC-8004 implementation includes governance, metadata URIs, and transfer restrictions we do not need. A mock ERC-721 Enumerable provides the exact interface TideCouncil consumes: `ownerOf(uint256)`, `balanceOf(address)`, and `tokenOfOwnerByIndex(address, uint256)`. |

### TideCouncil MVP-Critical Functions

Every function from the LLD is needed. No stubs.

**State-changing (used by the agent loop):**

| Function | Caller | MVP Role |
|---|---|---|
| `propose()` | Brandon (principal) via cast or script | Creates proposals. Entry point for the review loop. |
| `submitReview()` | Relayer (anyone) with agent's EIP-712 signature | Agents submit reviews via meta-transactions. The review runtime calls this. |
| `finalize()` | Relayer or Operator | Marks proposal as approved once quorum is met. Emits `ProposalApproved`. |
| `reject()` | Brandon (principal) | Withdraws a proposal. Needed for iteration. |
| `expire()` | Anyone after TTL | Cleanup. Low priority but trivial to include. |

**Admin (used by Brandon during setup):**

| Function | Caller | MVP Role |
|---|---|---|
| `emergencyRevokeAgent()` | Owner (Brandon) | Safety valve if an agent key is compromised. |
| `setPauser()` | Owner (Brandon) | Update pauser address. |
| `pause()` / `unpause()` | Pauser (Brandon) | Emergency stop. |

**View (used by Operator and runtimes):**

| Function | Caller | MVP Role |
|---|---|---|
| `getProposal()` | Operator, runtimes | Read proposal state. |
| `getReviews()` | Operator, runtimes | Read submitted reviews. |
| `getParticipants()` | Operator | Read participant list for a proposal. |
| `getReviewNonce()` | Review runtime | Get agent's current nonce before signing. |
| `isAgentRevoked()` | View | Check revocation status. |
| `isParticipant()` | View | Check participation status. |
| `proposalCount()` | View | Total proposals created. |

### MockIdentityRegistry for MVP

Instead of the full ERC-8004, we deploy a minimal mock:

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {ERC721Enumerable, ERC721} from "@openzeppelin/contracts/token/ERC721/extensions/ERC721Enumerable.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

/// @title MockIdentityRegistry
/// @notice Minimal ERC-721 Enumerable for MVP agent identities on arctic-1.
/// @dev Provides the exact interface TideCouncil consumes from ERC-8004:
///      ownerOf(uint256), balanceOf(address), tokenOfOwnerByIndex(address, uint256).
///      Only the owner can mint. This will be replaced by the real ERC-8004
///      IdentityRegistry before mainnet deployment.
contract MockIdentityRegistry is ERC721Enumerable, Ownable {
    constructor(address owner_) ERC721("TideAgentIdentity", "TIDE-ID") Ownable(owner_) {}

    /// @notice Mint an identity NFT to an agent address.
    /// @param to The agent's wallet address.
    /// @param tokenId The token ID to assign (must be unique).
    function mint(address to, uint256 tokenId) external onlyOwner {
        _mint(to, tokenId);
    }
}
```

**Why Ownable on the mock?** Prevents random addresses from minting identity tokens to themselves. Brandon controls minting.

**Migration path to mainnet:** Replace `MockIdentityRegistry` address in TideCouncil's storage with the real ERC-8004 deployment. TideCouncil reads the `identityRegistry` address from storage, so this is a governance call to update the address (add a setter in V2 if needed, or redeploy since it is testnet).

---

## 2. Agent Wallet and Identity Setup

### Agent Roster

| Agent Name | Role | Token ID | KMS Key Alias |
|---|---|---|---|
| `blockchain-dev` | Solidity/Foundry specialist | 1 | `tide/agents/blockchain-dev` |
| `k8s-specialist` | Operator/CRD specialist | 2 | `tide/agents/k8s-specialist` |
| `platform-eng` | Runtimes/manifests specialist | 3 | `tide/agents/platform-eng` |
| `coordinator` | Council orchestrator | 4 | `tide/agents/coordinator` |
| `reviewer` | Cross-review verifier | 5 | `tide/agents/reviewer` |

**Canonical short names:** `blockchain-dev`, `k8s-specialist`, `platform-eng`, `coordinator`, `reviewer`. These flow into ServiceAccount names (`tide-agent-{name}`), AWS Secrets Manager paths (`tide/agents/{name}/...`), KMS aliases (`alias/tide-agent-{name}`), and IRSA role names.

Token IDs 1-5 are assigned sequentially. Token ID 0 is not used (ERC-721 convention: token 0 can cause confusion with default uint256 values).

### AWS KMS Key Specification

Each agent needs a secp256k1 signing key in AWS KMS for EIP-712 meta-transaction signing.

**Key spec:**
- **Key type:** `ECC_SECG_P256K1` (secp256k1)
- **Key usage:** `SIGN_VERIFY`
- **Key spec:** `ECC_SECG_P256K1`
- **Key policy:** Agent runtime IAM role can call `kms:Sign` and `kms:GetPublicKey`; no other principals
- **Region:** `us-east-1` (or wherever the EKS cluster runs)

### Manual Steps (Brandon must do these)

These steps require AWS Console or CLI access with IAM admin permissions. They cannot be automated in a Foundry script.

**Step 1: Create KMS keys**

```bash
#!/usr/bin/env bash
# scripts/create-kms-keys.sh
# Creates AWS KMS secp256k1 signing keys for each agent.
# Requires: aws CLI configured with IAM permissions for kms:CreateKey, kms:CreateAlias.

set -euo pipefail

AGENTS=("blockchain-dev" "k8s-specialist" "platform-eng" "coordinator" "reviewer")
REGION="${AWS_REGION:-us-east-1}"
OUTPUT_FILE="deployments/agent-kms-keys.json"

echo "[]" > "$OUTPUT_FILE"

for agent in "${AGENTS[@]}"; do
    echo "Creating KMS key for agent: $agent"

    # Create the key
    KEY_OUTPUT=$(aws kms create-key \
        --key-spec ECC_SECG_P256K1 \
        --key-usage SIGN_VERIFY \
        --description "Tide agent signing key: $agent" \
        --region "$REGION" \
        --output json)

    KEY_ID=$(echo "$KEY_OUTPUT" | jq -r '.KeyMetadata.KeyId')
    KEY_ARN=$(echo "$KEY_OUTPUT" | jq -r '.KeyMetadata.Arn')

    # Create an alias for human readability
    aws kms create-alias \
        --alias-name "alias/tide-agent-${agent}" \
        --target-key-id "$KEY_ID" \
        --region "$REGION"

    echo "  Key ID:  $KEY_ID"
    echo "  Key ARN: $KEY_ARN"

    # Append to output file
    jq --arg name "$agent" \
       --arg key_id "$KEY_ID" \
       --arg key_arn "$KEY_ARN" \
       '. += [{"agent": $name, "kms_key_id": $key_id, "kms_key_arn": $key_arn}]' \
       "$OUTPUT_FILE" > "${OUTPUT_FILE}.tmp" && mv "${OUTPUT_FILE}.tmp" "$OUTPUT_FILE"
done

echo ""
echo "KMS keys created. Output: $OUTPUT_FILE"
echo "Next step: run scripts/derive-agent-addresses.sh to get Ethereum addresses."
```

**Step 2: Derive Ethereum addresses from KMS public keys**

```bash
#!/usr/bin/env bash
# scripts/derive-agent-addresses.sh
# Reads KMS key ARNs from deployments/agent-kms-keys.json,
# fetches public keys, and derives Ethereum addresses.
# Requires: aws CLI, openssl, cast (from foundry), jq, xxd.

set -euo pipefail

REGION="${AWS_REGION:-us-east-1}"
INPUT_FILE="deployments/agent-kms-keys.json"
OUTPUT_FILE="deployments/agent-wallets.json"

echo "[]" > "$OUTPUT_FILE"

AGENT_COUNT=$(jq length "$INPUT_FILE")

for ((i=0; i<AGENT_COUNT; i++)); do
    AGENT=$(jq -r ".[$i].agent" "$INPUT_FILE")
    KEY_ID=$(jq -r ".[$i].kms_key_id" "$INPUT_FILE")
    KEY_ARN=$(jq -r ".[$i].kms_key_arn" "$INPUT_FILE")

    echo "Deriving address for agent: $AGENT"

    # Get the DER-encoded public key from KMS
    PUB_KEY_DER=$(aws kms get-public-key \
        --key-id "$KEY_ID" \
        --region "$REGION" \
        --output text \
        --query 'PublicKey')

    # Decode base64 DER -> extract raw 65-byte uncompressed public key
    # The DER-encoded key has a header; the last 65 bytes are the uncompressed point (04 || x || y)
    RAW_PUB_HEX=$(echo "$PUB_KEY_DER" | base64 -d | openssl ec -pubin -inform DER -outform DER 2>/dev/null | tail -c 65 | xxd -p -c 65)

    # Strip the 04 prefix (uncompressed point marker) to get 64 bytes (x || y)
    PUB_KEY_64="${RAW_PUB_HEX:2}"

    # Keccak256 hash of the 64-byte public key, take last 20 bytes as Ethereum address
    ETH_ADDRESS=$(cast keccak "0x${PUB_KEY_64}" | cut -c 27-66)
    ETH_ADDRESS="0x${ETH_ADDRESS}"

    echo "  Address: $ETH_ADDRESS"

    jq --arg name "$AGENT" \
       --arg key_id "$KEY_ID" \
       --arg key_arn "$KEY_ARN" \
       --arg address "$ETH_ADDRESS" \
       '. += [{"agent": $name, "kms_key_id": $key_id, "kms_key_arn": $key_arn, "address": $address}]' \
       "$OUTPUT_FILE" > "${OUTPUT_FILE}.tmp" && mv "${OUTPUT_FILE}.tmp" "$OUTPUT_FILE"
done

echo ""
echo "Agent wallets derived. Output: $OUTPUT_FILE"
echo "Next step: fund each address with testnet SEI, then run the Foundry deployment."
```

**Step 3: Fund agent wallets with testnet SEI**

Testnet SEI is available from the Sei faucet:
- **Faucet URL:** https://arctic-1.sei.io/faucet (or Discord faucet in the Sei Discord)
- **Amount per request:** Typically 1-10 SEI per request
- **Amount needed per agent:** 0.5 SEI is more than sufficient for gas on testnet. Budget 2 SEI per agent to be safe.
- **Total needed:** 10 SEI for agents + 10 SEI for deployer = 20 SEI

Fund each address from `deployments/agent-wallets.json`. If the faucet has rate limits, use the deployer wallet to distribute:

```bash
#!/usr/bin/env bash
# scripts/fund-agent-wallets.sh
# Sends testnet SEI from the deployer to each agent wallet.
# Requires: cast, DEPLOYER_KEY env var set.

set -euo pipefail

RPC_URL="https://evm-rpc-arctic-1.sei-apis.com"
AMOUNT="2ether"  # 2 SEI per agent (ether = native token in cast)
INPUT_FILE="deployments/agent-wallets.json"

AGENT_COUNT=$(jq length "$INPUT_FILE")

for ((i=0; i<AGENT_COUNT; i++)); do
    AGENT=$(jq -r ".[$i].agent" "$INPUT_FILE")
    ADDRESS=$(jq -r ".[$i].address" "$INPUT_FILE")

    echo "Funding $AGENT ($ADDRESS) with $AMOUNT..."

    cast send "$ADDRESS" \
        --value "$AMOUNT" \
        --rpc-url "$RPC_URL" \
        --private-key "$DEPLOYER_KEY"

    echo "  Funded."
done

echo "All agents funded."
```

**Step 4: IAM permissions for agent runtimes**

Each agent runtime runs as a K8s pod with an IAM role attached via IRSA (IAM Roles for Service Accounts). The IAM role needs:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "AllowKMSSigning",
            "Effect": "Allow",
            "Action": [
                "kms:Sign",
                "kms:GetPublicKey"
            ],
            "Resource": "<agent-specific-KMS-key-ARN>"
        }
    ]
}
```

Each agent's service account (`tide-agent-{name}`) gets an IAM role scoped to ONLY that agent's KMS key. This is configured by the platform engineer via IRSA annotations on the service accounts. For MVP, a single shared IAM role with access to all 5 keys is acceptable, with per-agent scoping deferred to hardening.

### Agent Registry File

After all setup steps, produce the canonical agent registry:

**File: `deployments/arctic-1-agents.json`**

```json
{
    "chain_id": 713715,
    "network": "arctic-1",
    "identity_registry": "<deployed MockIdentityRegistry address>",
    "agents": [
        {
            "name": "blockchain-dev",
            "token_id": 1,
            "address": "0x...",
            "kms_key_arn": "arn:aws:kms:us-east-1:ACCOUNT:key/KEY-ID",
            "kms_key_alias": "alias/tide-agent-blockchain-dev"
        },
        {
            "name": "k8s-specialist",
            "token_id": 2,
            "address": "0x...",
            "kms_key_arn": "arn:aws:kms:us-east-1:ACCOUNT:key/KEY-ID",
            "kms_key_alias": "alias/tide-agent-k8s-specialist"
        },
        {
            "name": "platform-eng",
            "token_id": 3,
            "address": "0x...",
            "kms_key_arn": "arn:aws:kms:us-east-1:ACCOUNT:key/KEY-ID",
            "kms_key_alias": "alias/tide-agent-platform-eng"
        },
        {
            "name": "coordinator",
            "token_id": 4,
            "address": "0x...",
            "kms_key_arn": "arn:aws:kms:us-east-1:ACCOUNT:key/KEY-ID",
            "kms_key_alias": "alias/tide-agent-coordinator"
        },
        {
            "name": "reviewer",
            "token_id": 5,
            "address": "0x...",
            "kms_key_arn": "arn:aws:kms:us-east-1:ACCOUNT:key/KEY-ID",
            "kms_key_alias": "alias/tide-agent-reviewer"
        }
    ]
}
```

This file is committed to the repo. The Operator reads it (or a ConfigMap derived from it) to map `agent name -> token ID -> KMS key ARN` when constructing env vars for agent runtime pods.

---

## 3. Deployment Script

### `script/DeployMVP.s.sol`

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Script, console2} from "forge-std/Script.sol";
import {ERC1967Proxy} from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import {TideCouncil} from "../src/TideCouncil.sol";
import {MockIdentityRegistry} from "../test/mocks/MockIdentityRegistry.sol";

/// @title DeployMVP
/// @notice Deploys TideCouncil + MockIdentityRegistry for the MVP on arctic-1.
///         Does NOT deploy TideJobHook, ACP, or ReputationRegistry.
/// @dev Usage:
///   # Set environment variables (see .env.arctic-mvp.template)
///   source .env.arctic-mvp
///
///   forge script script/DeployMVP.s.sol:DeployMVP \
///     --rpc-url https://evm-rpc-arctic-1.sei-apis.com \
///     --broadcast \
///     --verify \
///     --verifier blockscout \
///     --verifier-url https://seitrace.com/arctic-1/api \
///     --private-key $DEPLOYER_KEY \
///     -vvvv
contract DeployMVP is Script {
    function run() external {
        // ── Read Configuration ─────────────────────────────────
        address deployer = vm.envAddress("DEPLOYER_ADDRESS");
        uint40 defaultTTL = uint40(vm.envUint("DEFAULT_TTL"));
        uint8 defaultQuorum = uint8(vm.envUint("DEFAULT_QUORUM"));

        // Agent addresses for identity minting (read from agent-wallets.json values)
        address[] memory agentAddresses = new address[](5);
        agentAddresses[0] = vm.envAddress("AGENT_BLOCKCHAIN_DEVELOPER");
        agentAddresses[1] = vm.envAddress("AGENT_KUBERNETES_SPECIALIST");
        agentAddresses[2] = vm.envAddress("AGENT_PLATFORM_ENGINEER");
        agentAddresses[3] = vm.envAddress("AGENT_COORDINATOR");
        agentAddresses[4] = vm.envAddress("AGENT_REVIEWER");

        // ── Deploy ─────────────────────────────────────────────
        vm.startBroadcast();

        // Step 1: Deploy MockIdentityRegistry
        // The deployer is the owner (can mint identity NFTs).
        MockIdentityRegistry identityRegistry = new MockIdentityRegistry(deployer);
        console2.log("MockIdentityRegistry:", address(identityRegistry));

        // Step 2: Mint identity NFTs for each agent (tokenIds 1-5)
        string[5] memory agentNames = [
            "blockchain-dev",
            "k8s-specialist",
            "platform-eng",
            "coordinator",
            "reviewer"
        ];
        for (uint256 i = 0; i < 5; i++) {
            uint256 tokenId = i + 1;
            identityRegistry.mint(agentAddresses[i], tokenId);
            console2.log("  Minted token", tokenId, "to", agentAddresses[i]);
        }

        // Step 3: Deploy TideCouncil implementation
        TideCouncil councilImpl = new TideCouncil();
        console2.log("TideCouncil implementation:", address(councilImpl));

        // Step 4: Deploy TideCouncil proxy with initialization
        // For MVP: deployer is both owner and pauser.
        bytes memory councilInitData = abi.encodeCall(
            TideCouncil.initialize,
            (
                deployer,                     // owner
                address(identityRegistry),    // identityRegistry (our mock)
                deployer,                     // pauser (deployer for MVP)
                defaultTTL,                   // default proposal TTL
                defaultQuorum                 // default quorum
            )
        );
        ERC1967Proxy councilProxy = new ERC1967Proxy(
            address(councilImpl),
            councilInitData
        );
        console2.log("TideCouncil proxy:", address(councilProxy));

        vm.stopBroadcast();

        // ── Output Summary ─────────────────────────────────────
        console2.log("");
        console2.log("========================================");
        console2.log("  MVP Deployment Summary (arctic-1)");
        console2.log("========================================");
        console2.log("MockIdentityRegistry:     ", address(identityRegistry));
        console2.log("TideCouncil implementation:", address(councilImpl));
        console2.log("TideCouncil proxy:         ", address(councilProxy));
        console2.log("");
        console2.log("Agent identities minted:");
        for (uint256 i = 0; i < 5; i++) {
            console2.log("  Token", i + 1, "->", agentAddresses[i]);
        }
        console2.log("");
        console2.log("Next steps:");
        console2.log("  1. Write addresses to deployments/arctic-1.json");
        console2.log("  2. Run VerifyMVP.s.sol");
        console2.log("  3. Verify contracts on SeiTrace");
        console2.log("  4. Create ConfigMap for Operator");
    }
}
```

### `script/VerifyMVP.s.sol`

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Script, console2} from "forge-std/Script.sol";
import {IERC721} from "@openzeppelin/contracts/token/ERC721/IERC721.sol";

/// @title VerifyMVP
/// @notice Post-deployment verification for the MVP contracts on arctic-1.
/// @dev Usage:
///   forge script script/VerifyMVP.s.sol:VerifyMVP \
///     --rpc-url https://evm-rpc-arctic-1.sei-apis.com \
///     -vvvv
contract VerifyMVP is Script {
    function run() external view {
        address councilProxy = vm.envAddress("COUNCIL_PROXY");
        address identityRegistry = vm.envAddress("IDENTITY_REGISTRY");
        address deployer = vm.envAddress("DEPLOYER_ADDRESS");

        console2.log("=== MockIdentityRegistry Verification ===");

        // Check total supply (should be 5)
        (bool ok, bytes memory data) = identityRegistry.staticcall(
            abi.encodeWithSignature("totalSupply()")
        );
        require(ok, "totalSupply() call failed");
        uint256 totalSupply = abi.decode(data, (uint256));
        require(totalSupply == 5, "Expected 5 identity tokens");
        console2.log("Total supply:       OK (5)");

        // Verify each agent owns their token
        address[] memory agentAddresses = new address[](5);
        agentAddresses[0] = vm.envAddress("AGENT_BLOCKCHAIN_DEVELOPER");
        agentAddresses[1] = vm.envAddress("AGENT_KUBERNETES_SPECIALIST");
        agentAddresses[2] = vm.envAddress("AGENT_PLATFORM_ENGINEER");
        agentAddresses[3] = vm.envAddress("AGENT_COORDINATOR");
        agentAddresses[4] = vm.envAddress("AGENT_REVIEWER");

        IERC721 identity = IERC721(identityRegistry);
        for (uint256 i = 0; i < 5; i++) {
            uint256 tokenId = i + 1;
            address owner = identity.ownerOf(tokenId);
            require(owner == agentAddresses[i], "Token owner mismatch");
            console2.log("  Token", tokenId, "owner: OK");
        }

        console2.log("");
        console2.log("=== TideCouncil Verification ===");

        // Check owner
        (ok, data) = councilProxy.staticcall(
            abi.encodeWithSignature("owner()")
        );
        require(ok, "owner() call failed");
        address owner = abi.decode(data, (address));
        require(owner == deployer, "Council owner mismatch");
        console2.log("Owner:              OK");

        // Check proposal count (should be 0)
        (ok, data) = councilProxy.staticcall(
            abi.encodeWithSignature("proposalCount()")
        );
        require(ok, "proposalCount() call failed");
        uint256 count = abi.decode(data, (uint256));
        require(count == 0, "Proposal count should be 0");
        console2.log("Proposal count:     OK (0)");

        // Check not paused
        (ok, data) = councilProxy.staticcall(
            abi.encodeWithSignature("paused()")
        );
        require(ok, "paused() call failed");
        bool paused = abi.decode(data, (bool));
        require(!paused, "Council should not be paused");
        console2.log("Paused:             OK (false)");

        // Check identity registry address
        // TideCouncil stores this; we need the getter or direct storage read.
        // Assuming a getter exists (add to TideCouncil if not):
        console2.log("");
        console2.log("=== All Checks Passed ===");
    }
}
```

### Environment Configuration

**File: `.env.arctic-mvp.template`** (committed to repo, no secrets)

```bash
# Sei Testnet (arctic-1) -- MVP Deployment
# Copy to .env.arctic-mvp and fill in values. NEVER commit .env.arctic-mvp.

# RPC
RPC_URL=https://evm-rpc-arctic-1.sei-apis.com
CHAIN_ID=713715
VERIFIER_URL=https://seitrace.com/arctic-1/api

# Deployer
DEPLOYER_ADDRESS=0x...  # Brandon's deployer EOA
DEPLOYER_KEY=0x...      # Private key (NEVER COMMIT)

# TideCouncil config
DEFAULT_TTL=7200        # 2 hours (fast iteration on testnet)
DEFAULT_QUORUM=3        # 3-of-5 agents must approve

# Agent wallet addresses (derived from KMS public keys)
# Run scripts/derive-agent-addresses.sh first to get these values.
AGENT_BLOCKCHAIN_DEVELOPER=0x...
AGENT_KUBERNETES_SPECIALIST=0x...
AGENT_PLATFORM_ENGINEER=0x...
AGENT_COORDINATOR=0x...
AGENT_REVIEWER=0x...

# Post-deployment (filled after deploy)
COUNCIL_PROXY=0x...
IDENTITY_REGISTRY=0x...
```

### Deployment Procedure (Step by Step)

```bash
# 1. Navigate to contracts directory
cd contracts/

# 2. Install dependencies
forge install OpenZeppelin/openzeppelin-contracts
forge install OpenZeppelin/openzeppelin-contracts-upgradeable
forge install foundry-rs/forge-std

# 3. Build and run tests
forge build
forge test -vvv

# 4. Set env vars
source .env.arctic-mvp

# 5. Deploy
forge script script/DeployMVP.s.sol:DeployMVP \
    --rpc-url "$RPC_URL" \
    --broadcast \
    --verify \
    --verifier blockscout \
    --verifier-url "$VERIFIER_URL" \
    --private-key "$DEPLOYER_KEY" \
    -vvvv

# 6. Record deployed addresses from console output into deployments/arctic-1.json
# (manually or parse from broadcast/DeployMVP.s.sol/713715/run-latest.json)

# 7. Set post-deployment env vars
export COUNCIL_PROXY=<address from step 6>
export IDENTITY_REGISTRY=<address from step 6>

# 8. Run verification
forge script script/VerifyMVP.s.sol:VerifyMVP \
    --rpc-url "$RPC_URL" \
    -vvvv

# 9. Verify source on SeiTrace (implementation contract)
forge verify-contract "$COUNCIL_IMPL" \
    src/TideCouncil.sol:TideCouncil \
    --chain-id 713715 \
    --verifier blockscout \
    --verifier-url "$VERIFIER_URL"

# 10. Verify MockIdentityRegistry on SeiTrace
forge verify-contract "$IDENTITY_REGISTRY" \
    test/mocks/MockIdentityRegistry.sol:MockIdentityRegistry \
    --chain-id 713715 \
    --verifier blockscout \
    --verifier-url "$VERIFIER_URL" \
    --constructor-args $(cast abi-encode "constructor(address)" "$DEPLOYER_ADDRESS")
```

### Output: `deployments/arctic-1.json`

After deployment, produce this file (manually or via a script that parses Foundry broadcast output):

```json
{
    "network": "arctic-1",
    "chain_id": 713715,
    "deployed_at": "2026-04-XX",
    "deployer": "0x...",
    "contracts": {
        "MockIdentityRegistry": {
            "address": "0x...",
            "type": "direct",
            "constructor_args": ["0x<deployer>"]
        },
        "TideCouncil_Implementation": {
            "address": "0x...",
            "type": "implementation"
        },
        "TideCouncil_Proxy": {
            "address": "0x...",
            "type": "ERC1967Proxy",
            "implementation": "0x<impl address>",
            "init_args": {
                "owner": "0x<deployer>",
                "identityRegistry": "0x<mock registry>",
                "pauser": "0x<deployer>",
                "defaultTTL": 7200,
                "defaultQuorum": 3
            }
        }
    },
    "eip712_domain": {
        "name": "TideCouncil",
        "version": "1",
        "chainId": 713715,
        "verifyingContract": "0x<proxy address>"
    }
}
```

### How Runtimes and Operator Discover Contract Addresses

The Operator reads deployed addresses from a Kubernetes ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tide-contracts
  namespace: tide-system
data:
  TIDE_COUNCIL_ADDRESS: "0x<TideCouncil proxy address>"
  TIDE_IDENTITY_REGISTRY: "0x<MockIdentityRegistry address>"
  TIDE_SEI_CHAIN_ID: "713715"
  TIDE_SEI_RPC_URL: "https://evm-rpc-arctic-1.sei-apis.com"
```

The Operator injects these as env vars into agent runtime pods. The runtimes use `TIDE_COUNCIL_ADDRESS` and `TIDE_SEI_CHAIN_ID` to construct EIP-712 domain separators and submit transactions.

This ConfigMap is generated from `deployments/arctic-1.json` by a simple script or manually by the platform engineer. It is NOT auto-synced -- updating it after a redeployment is a manual step.

---

## 4. One-Way Door Inventory

### Event Signatures

| Event | Keccak256 Topic Hash | Status |
|---|---|---|
| `ProposalCreated(uint256,address,bytes32,uint256,uint256[],uint8,uint40)` | Compute at deploy time | One-way door AFTER the Operator indexer depends on it |
| `ReviewSubmitted(uint256,uint256,uint8,bytes32)` | Compute at deploy time | One-way door AFTER the Operator indexer depends on it |
| `ProposalApproved(uint256,bytes32)` | Compute at deploy time | One-way door AFTER the Operator indexer depends on it |
| `ProposalRejected(uint256,bytes32)` | Compute at deploy time | One-way door AFTER the Operator indexer depends on it |
| `ProposalExpired(uint256)` | Compute at deploy time | One-way door AFTER the Operator indexer depends on it |

**Testnet mitigation:** On arctic-1, we can redeploy with new event signatures at any time. The Operator and runtimes are reconfigured via the ConfigMap. No external parties depend on our testnet events. The event signatures become a true one-way door only after mainnet deployment, when external indexers, dashboards, or integrators depend on the topic hashes.

**What changes for mainnet:** Once on pacific-1 (chain ID 1329), event signatures in the interface registry are frozen. The Operator must be rebuilt to match. Any change requires a contract upgrade (UUPS) that preserves the old event signatures alongside any new ones.

### EIP-712 Domain Separator

| Field | Value | Impact |
|---|---|---|
| `name` | `"TideCouncil"` | Baked into signed messages |
| `version` | `"1"` | Baked into signed messages |
| `chainId` | `713715` (testnet) / `1329` (mainnet) | Per-chain, auto-computed by `EIP712Upgradeable` |
| `verifyingContract` | TideCouncil proxy address | Per-deployment, auto-computed by `EIP712Upgradeable` |

**Testnet mitigation:** The domain separator includes `chainId` and `verifyingContract`, so testnet signatures are invalid on mainnet automatically. Redeploying on testnet changes the `verifyingContract`, invalidating all pending (unsigned) reviews, but agents re-derive the domain from the new proxy address.

**What changes for mainnet:** The `name` and `version` strings become permanent. If we change them, all agent signing code must update. These are locked after the first mainnet deployment.

### EIP-712 Type Hash

| Type | Hash String | Impact |
|---|---|---|
| `REVIEW_TYPEHASH` | `keccak256("Review(uint256 proposalId,uint8 verdict,bytes32 feedbackHash,uint256 agentTokenId,uint256 nonce)")` | Baked into every review signature |

**Testnet mitigation:** Can change freely before mainnet. All testnet signatures use the testnet domain separator and are invalid elsewhere.

**What changes for mainnet:** Permanent. Changing the type string invalidates all signed-but-not-yet-submitted reviews. Since agents sign and submit in the same runtime invocation (seconds apart), the blast radius is small, but the type hash must not change between proxy upgrades.

### Storage Layout

| Slot | Field | Impact |
|---|---|---|
| B+0 | `identityRegistry` | Permanent after deployment with state |
| B+1 | `pauser` | Permanent after deployment with state |
| B+2 | `proposalCount` | Permanent after deployment with state |
| B+3 | `defaultTTL`, `defaultQuorum` (packed) | Permanent after deployment with state |
| B+4 | `proposals` mapping | Permanent after deployment with state |
| B+5 | `proposalReviews` mapping | Permanent after deployment with state |
| B+6 | `proposalParticipantList` mapping | Permanent after deployment with state |
| B+7 | `proposalParticipants` mapping | Permanent after deployment with state |
| B+8 | `hasReviewed` mapping | Permanent after deployment with state |
| B+9 | `reviewNonces` mapping | Permanent after deployment with state |
| B+10 | `revokedAgents` mapping | Permanent after deployment with state |

**Testnet mitigation:** On arctic-1, we can redeploy a fresh proxy at any time, discarding all testnet state. No real assets are at risk.

**What changes for mainnet:** Storage layout is permanently frozen after the first proposal is created on mainnet. All future UUPS upgrades must append fields at B+11 and above. The `forge inspect TideCouncil storage-layout` command must be run before every upgrade and diffed against the previous layout.

### ERC-8004 Token ID Assignments

| Token ID | Agent | Impact |
|---|---|---|
| 1 | `blockchain-dev` | Referenced in proposals, reviews, and nonces |
| 2 | `k8s-specialist` | Referenced in proposals, reviews, and nonces |
| 3 | `platform-eng` | Referenced in proposals, reviews, and nonces |
| 4 | `coordinator` | Referenced in proposals, reviews, and nonces |
| 5 | `reviewer` | Referenced in proposals, reviews, and nonces |

**Testnet mitigation:** Token IDs are arbitrary. We can reassign on redeployment. Existing proposals reference old token IDs and become invalid, but that is acceptable on testnet.

**What changes for mainnet:** Token IDs become permanent once referenced in proposals and reviews. The mapping `agent name -> token ID` must be documented and never changed. If a new agent joins, it gets the next sequential token ID.

---

## 5. Cost and Operational Notes

### Estimated Gas Costs (arctic-1)

Gas prices on Sei testnet are extremely low. These estimates assume a gas price of ~0.1 gwei (Sei's base fee).

| Operation | Estimated Gas | Estimated Cost (SEI) | Notes |
|---|---|---|---|
| Deploy MockIdentityRegistry | ~1,500,000 | ~0.00015 | ERC-721 Enumerable is heavy |
| Mint 5 identity NFTs | ~250,000 (total) | ~0.000025 | ~50k gas per mint |
| Deploy TideCouncil impl | ~3,000,000 | ~0.0003 | Large contract with OZ inheritance |
| Deploy TideCouncil proxy | ~500,000 | ~0.00005 | ERC1967Proxy + initialize call |
| `propose()` (5 participants) | ~200,000 | ~0.00002 | Writes proposal struct + participant mappings |
| `submitReview()` | ~100,000 | ~0.00001 | Writes review + increments nonce |
| `finalize()` | ~80,000 | ~0.000008 | Reads reviews, updates status |
| `reject()` | ~50,000 | ~0.000005 | Updates status only |

**Total deployment cost:** Under 0.001 SEI. Gas costs are negligible on testnet.

**Full review cycle cost (propose + 5 reviews + finalize):** Under 0.0001 SEI.

The deployer wallet needs 10 SEI to be extremely comfortable. Agent wallets need 2 SEI each. In practice, 1 SEI per wallet would last thousands of transactions on testnet.

### How to Get Testnet SEI

1. **Sei Faucet (primary):** https://arctic-1.sei.io/faucet -- connect wallet, request SEI. Rate limited to 1 request per address per day.
2. **Sei Discord faucet:** Join the Sei Discord, use the `#faucet` channel with `!faucet <address>`. May have higher limits.
3. **Deployer distribution:** Fund the deployer from the faucet first (multiple days if needed), then distribute to agents via `cast send`.

### Monitoring

**How do we know the contracts are working?**

1. **SeiTrace block explorer:** After deployment, verify contracts at `https://seitrace.com/address/<address>?chain=arctic-1`. The "Events" tab shows all emitted events. After a proposal is created, you should see `ProposalCreated` events in the log.

2. **Direct RPC queries via cast:**
    ```bash
    # Check proposal count
    cast call $COUNCIL_PROXY "proposalCount()(uint256)" --rpc-url $RPC_URL

    # Read a specific proposal
    cast call $COUNCIL_PROXY "getProposal(uint256)((bytes32,uint256,address,uint40,uint40,uint8,uint8))" 1 --rpc-url $RPC_URL

    # Check agent nonce
    cast call $COUNCIL_PROXY "getReviewNonce(uint256)(uint256)" 1 --rpc-url $RPC_URL

    # Watch for events (real-time)
    cast logs --address $COUNCIL_PROXY --rpc-url $RPC_URL --from-block latest
    ```

3. **Operator event indexing:** The Tide Operator subscribes to `ProposalCreated`, `ReviewSubmitted`, `ProposalApproved`, and `ProposalRejected` events via `eth_getLogs` polling. If proposals are created but the Operator does not react, the problem is in the Operator's event filter configuration (topic hashes, contract address), not the contracts.

4. **Smoke test script:** After deployment, run a full cycle manually:
    ```bash
    # Create a proposal (as deployer/principal)
    cast send $COUNCIL_PROXY \
        "propose(bytes32,uint256,uint256[],uint8,uint40)" \
        $(cast keccak "test design document") \
        0 \
        "[1,2,3,4,5]" \
        3 \
        0 \
        --rpc-url $RPC_URL \
        --private-key $DEPLOYER_KEY

    # Verify proposal was created
    cast call $COUNCIL_PROXY "proposalCount()(uint256)" --rpc-url $RPC_URL
    # Should return 1

    cast call $COUNCIL_PROXY \
        "getProposal(uint256)((bytes32,uint256,address,uint40,uint40,uint8,uint8))" \
        1 \
        --rpc-url $RPC_URL
    # Should return the proposal struct with status=0 (Proposed)
    ```

    The full smoke test with signed reviews requires the agent KMS keys to be operational. This is tested as part of the review runtime integration test, not the contract deployment verification.

---

## Dependencies

### External

| Dependency | Version | Notes |
|---|---|---|
| Foundry (forge, cast) | Latest stable | Build, test, deploy, verify |
| OpenZeppelin Contracts | v5.x | ERC-721 Enumerable, ERC1967Proxy |
| OpenZeppelin Contracts Upgradeable | v5.x | UUPS, Ownable, Pausable, EIP712 for TideCouncil |
| forge-std | Latest | Script base, console2, vm cheatcodes |
| AWS CLI | v2.x | KMS key creation, public key export |
| jq | Latest | JSON manipulation in shell scripts |
| Sei arctic-1 RPC | -- | `https://evm-rpc-arctic-1.sei-apis.com` |

### Internal

| Dependency | Interface | Notes |
|---|---|---|
| TideCouncil source | `lld-tide-council.md` | Full implementation deployed |
| MockIdentityRegistry | Defined in this doc | Minimal mock for MVP |

### Explicit Exclusions

- **Does NOT deploy** TideJobHook (Phase 2)
- **Does NOT deploy** ERC-8183 AgenticCommerce (Phase 2)
- **Does NOT deploy** ERC-8004 ReputationRegistry (Phase 3+)
- **Does NOT provision** GitHub Apps or Kubernetes resources (platform engineer scope)
- **Does NOT handle** the Operator deployment or configuration (kubernetes specialist scope)

---

## Deferred (Do Not Build)

| Feature | Rationale |
|---|---|
| TideJobHook deployment | Phase 2 -- job escrow is not needed for the review loop |
| ERC-8183 ACP deployment | Phase 2 -- dependency of TideJobHook |
| ERC-8004 ReputationRegistry | Phase 3+ -- reputation gating explicitly deferred in constitution |
| Full ERC-8004 IdentityRegistry | Overkill for MVP. Mock ERC-721 provides the exact interface TideCouncil needs. |
| CREATE2 deterministic deployment | Nice-to-have for address predictability. Not needed for MVP where we just record deployed addresses. |
| Multisig ownership | On testnet, Brandon's EOA is owner/pauser. Multisig governance is a mainnet hardening task. |
| Contract upgrade procedures | Documented in `lld-contract-deployment.md`. Not needed until post-MVP bug fixes. |
| Per-agent IAM role scoping | Single shared IAM role for all 5 agents is acceptable for MVP. Per-agent scoping is a hardening task. |
