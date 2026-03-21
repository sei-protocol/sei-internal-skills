# Component: TideCouncil

## Owner

Blockchain Developer

## Phase

0

## Purpose

TideCouncil is a lightweight on-chain governance contract for AI agent design reviews on Sei. It manages the lifecycle of design proposals: creation, structured agent review with EIP-712 attestations, quorum-based approval, expiry, and principal-initiated rejection. It is **not** a DAO — the principal retains authority, and the council provides structured, auditable consensus among a set of registered AI agents.

**Business needs served:**
1. Present a design to a council of agents and collect structured feedback (Phase 1)
2. Reach quorum consensus and attest the approved design on-chain (Phase 1)

---

## Dependencies

### External

| Dependency | Interface | Notes |
|---|---|---|
| **ERC-8004 IdentityRegistry** | `IERC721` + `IERC721Enumerable` | Agent identity as ERC-721 NFTs. Used to verify agent registration and resolve `address → tokenId`. Must support `ownerOf(uint256)`, `balanceOf(address)`, and `tokenOfOwnerByIndex(address, uint256)`. |
| **OpenZeppelin Contracts Upgradeable v5.x** | `Initializable`, `OwnableUpgradeable`, `PausableUpgradeable`, `EIP712Upgradeable`, `UUPSUpgradeable` | Proxy upgradeability, access control, EIP-712 domain separator management. |
| **Sei EVM** | EVM-compatible chain | Chain ID 1329 (pacific-1 mainnet), 713715 (arctic-1 testnet). EIP-712 domain separator binds to chain ID and verifying contract address. |

### Internal

| Dependency | Interface | Notes |
|---|---|---|
| None | — | TideCouncil has no internal Tide dependencies. It is a standalone contract consumed by the Tide Operator and TideJobHook. |

### Explicit Exclusions

- **Does NOT depend on** ERC-8183 (AgenticCommerce). Proposal approval and job creation are decoupled — the operator bridges these off-chain.
- **Does NOT depend on** ERC-8004 ReputationRegistry. Reputation gating on council membership is deferred to Phase 3+.
- **Does NOT depend on** USDC or any token. TideCouncil is gas-only — no token transfers.

---

## Interface Specification

### Enums

```solidity
/// @notice Verdict an agent can submit for a design review.
enum Verdict {
    Approve,          // 0 — Agent approves the design as-is
    RequestChanges    // 1 — Agent requests changes before approval
}

/// @notice Lifecycle status of a design proposal.
enum ProposalStatus {
    Proposed,   // 0 — Active, accepting reviews
    Approved,   // 1 — Quorum approvals reached, finalized
    Rejected,   // 2 — Principal explicitly withdrew the proposal
    Expired     // 3 — TTL elapsed without finalization
}
```

**Design decision — `Rejected` state:** There is no negative quorum. `Rejected` is reached **only** when the principal explicitly calls `reject(proposalId)` to withdraw their own proposal. This is the simplest option: the principal reads agent feedback, then either (a) creates a revised proposal with `parentProposalId` linking to the original, or (b) explicitly rejects. Automatic rejection via majority `RequestChanges` is deferred — it adds complexity without clear Phase 0-2 value.

### Structs

```solidity
/// @notice On-chain representation of a design proposal.
/// @dev Storage-packed: 3 slots per proposal.
///   Slot 0: bytes32 designHash              (32 bytes)
///   Slot 1: uint256 parentProposalId        (32 bytes)
///   Slot 2: address principal (20) + uint40 createdAt (5) + uint40 expiresAt (5)
///           + uint8 quorum (1) + ProposalStatus status (1) = 32 bytes
struct Proposal {
    bytes32 designHash;
    uint256 parentProposalId;
    address principal;
    uint40 createdAt;
    uint40 expiresAt;
    uint8 quorum;
    ProposalStatus status;
}

/// @notice An agent's on-chain review attestation for a proposal.
/// @dev Storage-packed: 3 slots per review.
///   Slot 0: bytes32 feedbackHash            (32 bytes)
///   Slot 1: uint256 agentTokenId            (32 bytes)
///   Slot 2: Verdict verdict (1) + uint40 timestamp (5) = 6 bytes
struct Review {
    bytes32 feedbackHash;
    uint256 agentTokenId;
    Verdict verdict;
    uint40 timestamp;
}
```

### Events

All events below are **cross-component interfaces** consumed by the Tide Operator. Event signatures, indexed fields, and parameter order are one-way doors.

```solidity
/// @notice Emitted when a principal creates a new design proposal.
/// @dev Topic[0]: keccak256("ProposalCreated(uint256,address,bytes32,uint256,uint256[],uint8,uint40)")
/// @param proposalId Auto-incremented proposal identifier (starts at 1)
/// @param principal Address that created the proposal
/// @param designHash keccak256 of the design document content
/// @param parentProposalId 0 for initial submissions, previous proposalId for revisions
/// @param participantTokenIds ERC-8004 token IDs of agents invited to review
/// @param quorum Minimum number of Approve verdicts required for finalization
/// @param expiresAt Unix timestamp after which the proposal can be expired
event ProposalCreated(
    uint256 indexed proposalId,
    address indexed principal,
    bytes32 indexed designHash,
    uint256 parentProposalId,
    uint256[] participantTokenIds,
    uint8 quorum,
    uint40 expiresAt
);

/// @notice Emitted when an agent submits a review for a proposal.
/// @dev Topic[0]: keccak256("ReviewSubmitted(uint256,uint256,uint8,bytes32)")
/// @param proposalId The proposal being reviewed
/// @param agentTokenId ERC-8004 token ID of the reviewing agent
/// @param verdict Approve (0) or RequestChanges (1)
/// @param feedbackHash keccak256 of the off-chain structured review JSON
event ReviewSubmitted(
    uint256 indexed proposalId,
    uint256 indexed agentTokenId,
    Verdict verdict,
    bytes32 feedbackHash
);

/// @notice Emitted when a proposal reaches quorum and is finalized as approved.
/// @dev Topic[0]: keccak256("ProposalApproved(uint256,bytes32)")
///      This is the primary event the Tide Operator watches to trigger job decomposition.
/// @param proposalId The approved proposal
/// @param designHash The design hash that was approved (redundant with Proposal storage, included
///        for operator convenience — avoids a getProposal() RPC call)
event ProposalApproved(
    uint256 indexed proposalId,
    bytes32 indexed designHash
);

/// @notice Emitted when the principal explicitly rejects (withdraws) their proposal.
/// @dev Topic[0]: keccak256("ProposalRejected(uint256,bytes32)")
/// @param proposalId The rejected proposal
/// @param designHash The design hash of the rejected proposal
event ProposalRejected(
    uint256 indexed proposalId,
    bytes32 indexed designHash
);

/// @notice Emitted when a proposal's TTL elapses and it is marked expired.
/// @dev Topic[0]: keccak256("ProposalExpired(uint256)")
/// @param proposalId The expired proposal
event ProposalExpired(
    uint256 indexed proposalId
);

/// @notice Emitted when an agent is emergency-revoked by the owner.
/// @dev Topic[0]: keccak256("AgentRevoked(uint256,address)")
/// @param agentTokenId ERC-8004 token ID of the revoked agent
/// @param revokedBy Address that performed the revocation (always the owner)
event AgentRevoked(
    uint256 indexed agentTokenId,
    address indexed revokedBy
);

/// @notice Emitted when the pauser address is updated.
/// @dev Topic[0]: keccak256("PauserUpdated(address,address)")
event PauserUpdated(
    address indexed oldPauser,
    address indexed newPauser
);
```

#### Event Signature Strings (for operator topic hash derivation)

| Event | Canonical Signature |
|---|---|
| `ProposalCreated` | `ProposalCreated(uint256,address,bytes32,uint256,uint256[],uint8,uint40)` |
| `ReviewSubmitted` | `ReviewSubmitted(uint256,uint256,uint8,bytes32)` |
| `ProposalApproved` | `ProposalApproved(uint256,bytes32)` |
| `ProposalRejected` | `ProposalRejected(uint256,bytes32)` |
| `ProposalExpired` | `ProposalExpired(uint256)` |
| `AgentRevoked` | `AgentRevoked(uint256,address)` |
| `PauserUpdated` | `PauserUpdated(address,address)` |

Topic[0] for each event = `keccak256(bytes(canonical_signature))`.

**Note on `Verdict` enum in event signatures:** Solidity ABI encoding uses the underlying type (`uint8`) for enum parameters in event topic hash computation. The canonical string `ReviewSubmitted(uint256,uint256,uint8,bytes32)` is correct even though the Solidity source declares `Verdict verdict`. All consumers (Operator, runtimes) must use `uint8` in the canonical string.

### Custom Errors

```solidity
/// @notice designHash parameter is bytes32(0).
error ZeroDesignHash();

/// @notice quorum is 0 or exceeds the number of participants.
/// @param quorum The invalid quorum value
/// @param participantCount The number of participants provided
error InvalidQuorum(uint8 quorum, uint256 participantCount);

/// @notice A participant's ERC-8004 token does not exist (ownerOf reverts).
/// @param agentTokenId The unregistered token ID
error ParticipantNotRegistered(uint256 agentTokenId);

/// @notice A participant agent has been revoked via emergencyRevokeAgent.
/// @param agentTokenId The revoked token ID
error ParticipantRevoked(uint256 agentTokenId);

/// @notice parentProposalId is non-zero but does not reference an existing proposal.
/// @param parentProposalId The invalid parent reference
error ParentProposalNotFound(uint256 parentProposalId);

/// @notice The referenced proposalId does not exist.
/// @param proposalId The invalid proposal ID
error ProposalNotFound(uint256 proposalId);

/// @notice A state-changing operation requires Proposed status but found something else.
/// @param proposalId The proposal in the wrong state
/// @param current The actual status
error ProposalNotInProposedStatus(uint256 proposalId, ProposalStatus current);

/// @notice The proposal's TTL has elapsed. Reviews and finalization are blocked.
/// @param proposalId The expired proposal
/// @param expiresAt When the proposal expired
error ProposalHasExpired(uint256 proposalId, uint40 expiresAt);

/// @notice expire() was called but the TTL has not elapsed yet.
/// @param proposalId The proposal
/// @param expiresAt When it expires
/// @param currentTime Current block.timestamp
error ProposalNotYetExpired(uint256 proposalId, uint40 expiresAt, uint40 currentTime);

/// @notice The agent is not in the proposal's participant list.
/// @param proposalId The proposal
/// @param agentTokenId The non-participant agent
error AgentNotParticipant(uint256 proposalId, uint256 agentTokenId);

/// @notice The agent has already submitted a review for this proposal.
/// @param proposalId The proposal
/// @param agentTokenId The agent that already reviewed
error AgentAlreadyReviewed(uint256 proposalId, uint256 agentTokenId);

/// @notice The agent has been revoked and cannot submit reviews.
/// @param agentTokenId The revoked agent
error AgentIsRevoked(uint256 agentTokenId);

/// @notice The nonce in the EIP-712 signature does not match the expected nonce.
/// @param agentTokenId The agent
/// @param expected The next valid nonce
/// @param provided The nonce in the signature
error InvalidNonce(uint256 agentTokenId, uint256 expected, uint256 provided);

/// @notice EIP-712 signature verification failed (ecrecover returned wrong address or zero).
error InvalidSignature();

/// @notice finalize() was called but not enough non-revoked Approve verdicts.
/// @param proposalId The proposal
/// @param approvals Number of valid approvals counted
/// @param quorum Required quorum
error QuorumNotReached(uint256 proposalId, uint256 approvals, uint8 quorum);

/// @notice reject() was called by someone other than the proposal's principal.
/// @param proposalId The proposal
/// @param caller Who called reject
/// @param principal The actual principal
error NotPrincipal(uint256 proposalId, address caller, address principal);

/// @notice pause() or unpause() was called by someone other than the pauser.
/// @param caller Who called the function
error NotPauser(address caller);

/// @notice TTL is below the minimum (1 hour).
/// @param ttl The provided TTL
/// @param minimum The minimum allowed (3600)
error TTLTooShort(uint40 ttl, uint40 minimum);

/// @notice TTL exceeds the maximum (30 days).
/// @param ttl The provided TTL
/// @param maximum The maximum allowed (2592000)
error TTLTooLong(uint40 ttl, uint40 maximum);

/// @notice The agent has already been revoked.
/// @param agentTokenId The already-revoked agent
error AgentAlreadyRevoked(uint256 agentTokenId);
```

### Constants

```solidity
/// @notice Minimum TTL for a proposal (1 hour).
uint40 public constant MIN_TTL = 3600;

/// @notice Maximum TTL for a proposal (30 days).
uint40 public constant MAX_TTL = 2_592_000;

/// @notice EIP-712 type hash for the Review struct.
/// keccak256("Review(uint256 proposalId,uint8 verdict,bytes32 feedbackHash,uint256 agentTokenId,uint256 nonce)")
bytes32 public constant REVIEW_TYPEHASH = keccak256(
    "Review(uint256 proposalId,uint8 verdict,bytes32 feedbackHash,uint256 agentTokenId,uint256 nonce)"
);
```

### Functions

```solidity
// ──────────────────────────────────────────────────────────
// Initializer (UUPS proxy)
// ──────────────────────────────────────────────────────────

/// @notice Initializes the TideCouncil proxy. Called once after deployment.
/// @param owner_ Address that can upgrade the contract and call emergencyRevokeAgent.
///        Expected: a multisig or the deployer initially.
/// @param identityRegistry_ Address of the deployed ERC-8004 IdentityRegistry (ERC-721).
/// @param pauser_ Address authorized to call pause() and unpause().
///        Expected: a 2-of-3 multisig.
/// @param defaultTTL_ Default time-to-live for proposals in seconds.
///        Must satisfy MIN_TTL <= defaultTTL_ <= MAX_TTL.
/// @param defaultQuorum_ Default quorum for proposals (can be overridden per-proposal).
///        Must be >= 1.
function initialize(
    address owner_,
    address identityRegistry_,
    address pauser_,
    uint40 defaultTTL_,
    uint8 defaultQuorum_
) external initializer;

// ──────────────────────────────────────────────────────────
// Proposal Lifecycle
// ──────────────────────────────────────────────────────────

/// @notice Creates a new design proposal.
/// @dev Anyone can be a principal. The contract validates:
///      - designHash is non-zero
///      - Each participant tokenId exists in the IdentityRegistry and is not revoked
///      - quorum is >= 1 and <= participantTokenIds.length
///      - ttl is within [MIN_TTL, MAX_TTL]
///      - If parentProposalId != 0, the parent proposal must exist
///      Emits ProposalCreated.
/// @param designHash keccak256 of the design document content (stored on GitHub).
/// @param parentProposalId 0 for initial proposals, or the ID of the proposal being revised.
/// @param participantTokenIds ERC-8004 token IDs of agents invited to review.
/// @param quorum Minimum Approve verdicts required. Pass 0 to use defaultQuorum.
/// @param ttl Time-to-live in seconds. Pass 0 to use defaultTTL.
/// @return proposalId The auto-incremented ID of the new proposal (starts at 1).
function propose(
    bytes32 designHash,
    uint256 parentProposalId,
    uint256[] calldata participantTokenIds,
    uint8 quorum,
    uint40 ttl
) external whenNotPaused returns (uint256 proposalId);

/// @notice Submits an agent's review for a proposal using an EIP-712 signed attestation.
/// @dev Meta-transaction pattern: anyone can relay this call as long as they provide a valid
///      EIP-712 signature from the agent's wallet (the wallet that owns agentTokenId in the
///      IdentityRegistry). The contract validates:
///      - Proposal exists and is in Proposed status
///      - Proposal has not expired (block.timestamp <= expiresAt)
///      - agentTokenId is in the proposal's participant list
///      - agentTokenId has not already reviewed this proposal
///      - agentTokenId is not revoked
///      - nonce matches the agent's current reviewNonce
///      - EIP-712 signature recovers to identityRegistry.ownerOf(agentTokenId)
///      Increments the agent's nonce. Emits ReviewSubmitted.
/// @param proposalId The proposal to review.
/// @param verdict Approve (0) or RequestChanges (1).
/// @param feedbackHash keccak256 of the off-chain structured review JSON.
/// @param agentTokenId The reviewing agent's ERC-8004 token ID.
/// @param nonce The agent's current review nonce (replay protection).
/// @param signature 65-byte ECDSA signature (r, s, v) over the EIP-712 digest.
function submitReview(
    uint256 proposalId,
    Verdict verdict,
    bytes32 feedbackHash,
    uint256 agentTokenId,
    uint256 nonce,
    bytes calldata signature
) external whenNotPaused;

/// @notice Finalizes a proposal as Approved if quorum is met.
/// @dev Permissionless — anyone can call. The contract:
///      1. Verifies proposal is in Proposed status
///      2. Verifies proposal has not expired
///      3. Counts Approve verdicts from non-revoked agents
///      4. Requires approvals >= quorum
///      5. Sets status to Approved
///      Emits ProposalApproved. Reverts with QuorumNotReached if insufficient approvals.
/// @param proposalId The proposal to finalize.
function finalize(uint256 proposalId) external whenNotPaused;

/// @notice Principal explicitly rejects (withdraws) their own proposal.
/// @dev Only callable by the proposal's principal. Proposal must be in Proposed status.
///      Emits ProposalRejected.
/// @param proposalId The proposal to reject.
function reject(uint256 proposalId) external whenNotPaused;

/// @notice Marks a proposal as Expired after its TTL has elapsed.
/// @dev Permissionless — anyone can call after block.timestamp > expiresAt.
///      Proposal must be in Proposed status. Emits ProposalExpired.
/// @param proposalId The proposal to expire.
function expire(uint256 proposalId) external whenNotPaused;

// ──────────────────────────────────────────────────────────
// Admin
// ──────────────────────────────────────────────────────────

/// @notice Revokes an agent from the council. Owner-only.
/// @dev Sets the agent as revoked. Effects:
///      - The agent cannot be added as a participant to new proposals
///      - The agent cannot submit new reviews
///      - Existing reviews from this agent are excluded from quorum calculation
///        in finalize() (checked at finalize-time, not retroactively modified)
///      Does NOT modify any existing proposal or review storage — revocation is
///      lazy-evaluated during finalize(). This is gas-efficient and avoids
///      iterating over all active proposals.
///      Emits AgentRevoked.
/// @param agentTokenId ERC-8004 token ID to revoke.
function emergencyRevokeAgent(uint256 agentTokenId) external onlyOwner;

/// @notice Updates the pauser address. Owner-only.
/// @param newPauser The new pauser address.
function setPauser(address newPauser) external onlyOwner;

// ──────────────────────────────────────────────────────────
// Pause
// ──────────────────────────────────────────────────────────

/// @notice Pauses all state-changing functions. Pauser-only.
function pause() external;

/// @notice Unpauses the contract. Pauser-only.
function unpause() external;

// ──────────────────────────────────────────────────────────
// View Functions
// ──────────────────────────────────────────────────────────

/// @notice Returns the full Proposal struct for a given ID.
function getProposal(uint256 proposalId) external view returns (Proposal memory);

/// @notice Returns all reviews submitted for a proposal.
function getReviews(uint256 proposalId) external view returns (Review[] memory);

/// @notice Returns the participant token IDs for a proposal.
function getParticipants(uint256 proposalId) external view returns (uint256[] memory);

/// @notice Returns the current review nonce for an agent.
function getReviewNonce(uint256 agentTokenId) external view returns (uint256);

/// @notice Returns true if the agent has been revoked.
function isAgentRevoked(uint256 agentTokenId) external view returns (bool);

/// @notice Returns true if agentTokenId is a participant of the given proposal.
function isParticipant(uint256 proposalId, uint256 agentTokenId) external view returns (bool);

/// @notice Returns the total number of proposals created.
function proposalCount() external view returns (uint256);

// ──────────────────────────────────────────────────────────
// UUPS
// ──────────────────────────────────────────────────────────

/// @notice Authorization hook for UUPS upgrades. Owner-only.
function _authorizeUpgrade(address newImplementation) internal override onlyOwner;
```

### EIP-712 Domain & Type Hashes

**Domain separator** (managed by OpenZeppelin's `EIP712Upgradeable`):

```
EIP712Domain(
    string name     = "TideCouncil",
    string version  = "1",
    uint256 chainId = <chain ID at deployment: 1329 mainnet, 713715 testnet>,
    address verifyingContract = <TideCouncil proxy address>
)
```

**Review type hash:**

```
Review(uint256 proposalId,uint8 verdict,bytes32 feedbackHash,uint256 agentTokenId,uint256 nonce)
```

**Digest construction** (used in `submitReview` signature verification):

```solidity
bytes32 structHash = keccak256(abi.encode(
    REVIEW_TYPEHASH,
    proposalId,
    uint8(verdict),
    feedbackHash,
    agentTokenId,
    nonce
));
bytes32 digest = _hashTypedDataV4(structHash);
address signer = ECDSA.recover(digest, signature);
```

The `_hashTypedDataV4` function (from `EIP712Upgradeable`) prepends `"\x19\x01"` and the domain separator automatically.

**One-way door:** The REVIEW_TYPEHASH string and the EIP-712 domain name/version are permanent once agents begin signing reviews. Changing them invalidates all pending signatures. These values must not change across proxy upgrades.

### Complete Interface (ITideCouncil.sol)

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

interface ITideCouncil {
    // ── Enums ──────────────────────────────────────────────
    enum Verdict { Approve, RequestChanges }
    enum ProposalStatus { Proposed, Approved, Rejected, Expired }

    // ── Structs ────────────────────────────────────────────
    struct Proposal {
        bytes32 designHash;
        uint256 parentProposalId;
        address principal;
        uint40 createdAt;
        uint40 expiresAt;
        uint8 quorum;
        ProposalStatus status;
    }

    struct Review {
        bytes32 feedbackHash;
        uint256 agentTokenId;
        Verdict verdict;
        uint40 timestamp;
    }

    // ── Events ─────────────────────────────────────────────
    event ProposalCreated(
        uint256 indexed proposalId,
        address indexed principal,
        bytes32 indexed designHash,
        uint256 parentProposalId,
        uint256[] participantTokenIds,
        uint8 quorum,
        uint40 expiresAt
    );

    event ReviewSubmitted(
        uint256 indexed proposalId,
        uint256 indexed agentTokenId,
        Verdict verdict,
        bytes32 feedbackHash
    );

    event ProposalApproved(
        uint256 indexed proposalId,
        bytes32 indexed designHash
    );

    event ProposalRejected(
        uint256 indexed proposalId,
        bytes32 indexed designHash
    );

    event ProposalExpired(uint256 indexed proposalId);

    event AgentRevoked(
        uint256 indexed agentTokenId,
        address indexed revokedBy
    );

    event PauserUpdated(
        address indexed oldPauser,
        address indexed newPauser
    );

    // ── Errors ─────────────────────────────────────────────
    error ZeroDesignHash();
    error InvalidQuorum(uint8 quorum, uint256 participantCount);
    error ParticipantNotRegistered(uint256 agentTokenId);
    error ParticipantRevoked(uint256 agentTokenId);
    error ParentProposalNotFound(uint256 parentProposalId);
    error ProposalNotFound(uint256 proposalId);
    error ProposalNotInProposedStatus(uint256 proposalId, ProposalStatus current);
    error ProposalHasExpired(uint256 proposalId, uint40 expiresAt);
    error ProposalNotYetExpired(uint256 proposalId, uint40 expiresAt, uint40 currentTime);
    error AgentNotParticipant(uint256 proposalId, uint256 agentTokenId);
    error AgentAlreadyReviewed(uint256 proposalId, uint256 agentTokenId);
    error AgentIsRevoked(uint256 agentTokenId);
    error InvalidNonce(uint256 agentTokenId, uint256 expected, uint256 provided);
    error InvalidSignature();
    error QuorumNotReached(uint256 proposalId, uint256 approvals, uint8 quorum);
    error NotPrincipal(uint256 proposalId, address caller, address principal);
    error NotPauser(address caller);
    error TTLTooShort(uint40 ttl, uint40 minimum);
    error TTLTooLong(uint40 ttl, uint40 maximum);
    error AgentAlreadyRevoked(uint256 agentTokenId);

    // ── State-Changing Functions ───────────────────────────
    function initialize(
        address owner_,
        address identityRegistry_,
        address pauser_,
        uint40 defaultTTL_,
        uint8 defaultQuorum_
    ) external;

    function propose(
        bytes32 designHash,
        uint256 parentProposalId,
        uint256[] calldata participantTokenIds,
        uint8 quorum,
        uint40 ttl
    ) external returns (uint256 proposalId);

    function submitReview(
        uint256 proposalId,
        Verdict verdict,
        bytes32 feedbackHash,
        uint256 agentTokenId,
        uint256 nonce,
        bytes calldata signature
    ) external;

    function finalize(uint256 proposalId) external;
    function reject(uint256 proposalId) external;
    function expire(uint256 proposalId) external;

    function emergencyRevokeAgent(uint256 agentTokenId) external;
    function setPauser(address newPauser) external;
    function pause() external;
    function unpause() external;

    // ── View Functions ─────────────────────────────────────
    function getProposal(uint256 proposalId) external view returns (Proposal memory);
    function getReviews(uint256 proposalId) external view returns (Review[] memory);
    function getParticipants(uint256 proposalId) external view returns (uint256[] memory);
    function getReviewNonce(uint256 agentTokenId) external view returns (uint256);
    function isAgentRevoked(uint256 agentTokenId) external view returns (bool);
    function isParticipant(uint256 proposalId, uint256 agentTokenId) external view returns (bool);
    function proposalCount() external view returns (uint256);

    // ── Constants ──────────────────────────────────────────
    function MIN_TTL() external pure returns (uint40);
    function MAX_TTL() external pure returns (uint40);
    function REVIEW_TYPEHASH() external pure returns (bytes32);
}
```

---

## State Model

### Storage Namespace (ERC-7201)

TideCouncil uses ERC-7201 (namespaced storage) to avoid slot collisions between the implementation's storage and OpenZeppelin's upgradeable contract storage.

**Namespace ID:** `"tide.council.storage.v1"`

**Base slot computation:**

```solidity
bytes32 constant STORAGE_SLOT = keccak256(
    abi.encode(
        uint256(keccak256("tide.council.storage.v1")) - 1
    )
) & ~bytes32(uint256(0xff));
```

### Storage Layout

All fields below are offsets from the ERC-7201 base slot `B`. OpenZeppelin's `OwnableUpgradeable`, `PausableUpgradeable`, `EIP712Upgradeable`, and `UUPSUpgradeable` each have their own independent ERC-7201 namespaces managed by OpenZeppelin — they do not collide with the layout below.

```
┌────────┬──────────────────────────────────────────────────────────────────────┐
│ Offset │ Field                                                                │
├────────┼──────────────────────────────────────────────────────────────────────┤
│ B+0    │ address identityRegistry                                             │
│        │ (20 bytes, left-padded to 32)                                        │
├────────┼──────────────────────────────────────────────────────────────────────┤
│ B+1    │ address pauser                                                       │
│        │ (20 bytes, left-padded to 32)                                        │
├────────┼──────────────────────────────────────────────────────────────────────┤
│ B+2    │ uint256 proposalCount                                                │
│        │ (32 bytes) — auto-increment counter, first proposal is ID 1          │
├────────┼──────────────────────────────────────────────────────────────────────┤
│ B+3    │ uint40 defaultTTL | uint8 defaultQuorum                              │
│        │ (6 bytes packed, right-aligned in 32-byte slot)                      │
│        │ Layout: [0x00...00][defaultQuorum:1][defaultTTL:5]                   │
├────────┼──────────────────────────────────────────────────────────────────────┤
│ B+4    │ mapping(uint256 proposalId => Proposal) proposals                    │
│        │ Proposal at keccak256(abi.encode(proposalId, B+4)):                  │
│        │   +0: bytes32 designHash                                             │
│        │   +1: uint256 parentProposalId                                       │
│        │   +2: address principal (20) | uint40 createdAt (5) |                │
│        │       uint40 expiresAt (5) | uint8 quorum (1) | uint8 status (1)    │
├────────┼──────────────────────────────────────────────────────────────────────┤
│ B+5    │ mapping(uint256 proposalId => Review[]) proposalReviews              │
│        │ Array length at keccak256(abi.encode(proposalId, B+5))               │
│        │ Element i base: keccak256(keccak256(abi.encode(proposalId, B+5)))    │
│        │   + (i * 3)                                                          │
│        │ Each Review: 3 slots (feedbackHash, agentTokenId, verdict+timestamp) │
├────────┼──────────────────────────────────────────────────────────────────────┤
│ B+6    │ mapping(uint256 proposalId => uint256[]) proposalParticipantList     │
│        │ Array of ERC-8004 token IDs for each proposal. Written once during   │
│        │ propose(). Used by getParticipants() view function.                  │
├────────┼──────────────────────────────────────────────────────────────────────┤
│ B+7    │ mapping(uint256 proposalId => mapping(uint256 agentTokenId => bool)) │
│        │ proposalParticipants                                                 │
│        │ O(1) lookup: "is this agent a participant of this proposal?"         │
├────────┼──────────────────────────────────────────────────────────────────────┤
│ B+8    │ mapping(uint256 proposalId => mapping(uint256 agentTokenId => bool)) │
│        │ hasReviewed                                                          │
│        │ O(1) lookup: "has this agent already reviewed this proposal?"        │
├────────┼──────────────────────────────────────────────────────────────────────┤
│ B+9    │ mapping(uint256 agentTokenId => uint256) reviewNonces                │
│        │ Per-agent sequential nonce for EIP-712 replay protection.            │
│        │ Incremented after each successful submitReview() call.               │
├────────┼──────────────────────────────────────────────────────────────────────┤
│ B+10   │ mapping(uint256 agentTokenId => bool) revokedAgents                  │
│        │ True if the agent has been emergency-revoked. Checked during:        │
│        │ propose() (participant validation), submitReview(), finalize().       │
└────────┴──────────────────────────────────────────────────────────────────────┘
```

**One-way door:** Slot offsets B+0 through B+10 are permanent. Adding new fields in future upgrades must use offsets B+11 and above. Re-ordering or re-typing any existing slot will corrupt state.

### Solidity Storage Struct

```solidity
/// @custom:storage-location erc7201:tide.council.storage.v1
struct TideCouncilStorage {
    address identityRegistry;
    address pauser;
    uint256 proposalCount;
    uint40 defaultTTL;
    uint8 defaultQuorum;
    mapping(uint256 => Proposal) proposals;
    mapping(uint256 => Review[]) proposalReviews;
    mapping(uint256 => uint256[]) proposalParticipantList;
    mapping(uint256 => mapping(uint256 => bool)) proposalParticipants;
    mapping(uint256 => mapping(uint256 => bool)) hasReviewed;
    mapping(uint256 => uint256) reviewNonces;
    mapping(uint256 => bool) revokedAgents;
}
```

### State Transitions

```
                    ┌──────────┐
    propose() ───►  │ Proposed │
                    └────┬─────┘
                         │
            ┌────────────┼───────────────┐
            │            │               │
            ▼            ▼               ▼
      ┌──────────┐ ┌──────────┐   ┌──────────┐
      │ Approved │ │ Rejected │   │ Expired  │
      └──────────┘ └──────────┘   └──────────┘
       finalize()   reject()       expire()
       (quorum      (principal     (TTL elapsed)
        met)         withdraws)
```

All three terminal states are **final** — no transitions out. A revised design creates a new proposal with `parentProposalId` linking to the original.

---

## Internal Design

### propose()

```
1. Validate designHash != bytes32(0)                          → revert ZeroDesignHash
2. Resolve quorum: if quorum == 0, use defaultQuorum
3. Resolve ttl: if ttl == 0, use defaultTTL
4. Validate MIN_TTL <= ttl <= MAX_TTL                         → revert TTLTooShort/TTLTooLong
5. Validate participantTokenIds.length >= quorum >= 1         → revert InvalidQuorum
6. If parentProposalId != 0:
   validate proposals[parentProposalId].principal != address(0) → revert ParentProposalNotFound
7. For each tokenId in participantTokenIds:
   a. Call identityRegistry.ownerOf(tokenId)
      If reverts (token doesn't exist)                        → revert ParticipantNotRegistered
   b. Check !revokedAgents[tokenId]                           → revert ParticipantRevoked
8. proposalCount++; proposalId = proposalCount
9. Store Proposal struct in proposals[proposalId]
10. Store participant list in proposalParticipantList[proposalId]
11. Set proposalParticipants[proposalId][tokenId] = true for each participant
12. Emit ProposalCreated(...)
13. Return proposalId
```

**Gas profile:** O(n) where n = participant count. Each participant requires one external call to `identityRegistry.ownerOf()` (~2,600 gas for warm SLOAD) plus storage writes. For n=3 (initial council), total gas is approximately 200,000-250,000.

### submitReview()

```
1. Load proposal = proposals[proposalId]
   Validate proposal.principal != address(0)                  → revert ProposalNotFound
2. Validate proposal.status == Proposed                       → revert ProposalNotInProposedStatus
3. Validate block.timestamp <= proposal.expiresAt             → revert ProposalHasExpired
4. Validate proposalParticipants[proposalId][agentTokenId]    → revert AgentNotParticipant
5. Validate !hasReviewed[proposalId][agentTokenId]            → revert AgentAlreadyReviewed
6. Validate !revokedAgents[agentTokenId]                      → revert AgentIsRevoked
7. Validate nonce == reviewNonces[agentTokenId]               → revert InvalidNonce
8. Construct EIP-712 digest:
   structHash = keccak256(abi.encode(
       REVIEW_TYPEHASH, proposalId, uint8(verdict), feedbackHash, agentTokenId, nonce
   ))
   digest = _hashTypedDataV4(structHash)
9. Recover signer = ECDSA.recover(digest, signature)
   Validate signer != address(0) && signer == identityRegistry.ownerOf(agentTokenId)
                                                              → revert InvalidSignature
10. reviewNonces[agentTokenId]++
11. hasReviewed[proposalId][agentTokenId] = true
12. Push Review{feedbackHash, agentTokenId, verdict, uint40(block.timestamp)}
    to proposalReviews[proposalId]
13. Emit ReviewSubmitted(proposalId, agentTokenId, verdict, feedbackHash)
```

**Gas profile:** One external call to `identityRegistry.ownerOf()` (~2,600 gas), one `ecrecover` precompile call (~3,000 gas), plus storage writes (~20,000 gas for cold slots). Total approximately 120,000-150,000 gas per review.

### finalize()

```
1. Load proposal = proposals[proposalId]
   Validate proposal.principal != address(0)                  → revert ProposalNotFound
2. Validate proposal.status == Proposed                       → revert ProposalNotInProposedStatus
3. Validate block.timestamp <= proposal.expiresAt             → revert ProposalHasExpired
4. Load reviews = proposalReviews[proposalId]
5. approvalCount = 0
6. For each review in reviews:
   if !revokedAgents[review.agentTokenId] && review.verdict == Approve:
       approvalCount++
7. Validate approvalCount >= proposal.quorum                  → revert QuorumNotReached
8. proposals[proposalId].status = Approved
9. Emit ProposalApproved(proposalId, proposal.designHash)
```

**Design decision — revoked agent reviews:** When `finalize()` is called, reviews from revoked agents are **skipped entirely** — they do not count as approvals and do not count toward any total. This means revoking an agent can cause a previously quorum-meeting proposal to no longer meet quorum. This is intentional: if an agent is compromised, its attestations should not influence outcomes.

**Gas profile:** O(n) where n = number of reviews (bounded by participant count, max 255). Reads from storage only (no writes except the status update). Approximately 50,000-80,000 gas for 3 reviews.

### reject()

```
1. Load proposal = proposals[proposalId]
   Validate proposal.principal != address(0)                  → revert ProposalNotFound
2. Validate proposal.status == Proposed                       → revert ProposalNotInProposedStatus
3. Validate msg.sender == proposal.principal                  → revert NotPrincipal
4. proposals[proposalId].status = Rejected
5. Emit ProposalRejected(proposalId, proposal.designHash)
```

### expire()

```
1. Load proposal = proposals[proposalId]
   Validate proposal.principal != address(0)                  → revert ProposalNotFound
2. Validate proposal.status == Proposed                       → revert ProposalNotInProposedStatus
3. Validate block.timestamp > proposal.expiresAt              → revert ProposalNotYetExpired
4. proposals[proposalId].status = Expired
5. Emit ProposalExpired(proposalId)
```

### emergencyRevokeAgent()

```
1. Validate msg.sender == owner()                             (via onlyOwner modifier)
2. Validate !revokedAgents[agentTokenId]                      → revert AgentAlreadyRevoked
3. revokedAgents[agentTokenId] = true
4. Emit AgentRevoked(agentTokenId, msg.sender)
```

No iteration over proposals. Revocation is lazy-evaluated during `finalize()`.

### pause() / unpause()

```
1. Validate msg.sender == pauser                              → revert NotPauser
2. Call _pause() / _unpause()    (OpenZeppelin Pausable internals)
```

---

## Error Handling

| Error | Cause | Detection | Caller Action |
|---|---|---|---|
| `ZeroDesignHash` | `propose()` called with `designHash = bytes32(0)` | Input validation | Compute keccak256 of the design document before calling |
| `InvalidQuorum` | quorum is 0 or exceeds participant count | Input validation | Ensure 1 <= quorum <= participants.length |
| `ParticipantNotRegistered` | Token ID does not exist in IdentityRegistry | `ownerOf()` reverts | Register agent on ERC-8004 before including in proposal |
| `ParticipantRevoked` | Token ID has been emergency-revoked | Storage check | Do not include revoked agents in proposals |
| `ParentProposalNotFound` | `parentProposalId != 0` but no proposal exists with that ID | Storage check | Verify parent ID before calling |
| `ProposalNotFound` | Referenced proposalId has never been created | Storage check (principal == address(0)) | Use valid proposal IDs from ProposalCreated events |
| `ProposalNotInProposedStatus` | Operation requires `Proposed` status but proposal is in a terminal state | Status check | Proposal has already been finalized, rejected, or expired — no further action possible |
| `ProposalHasExpired` | TTL elapsed for review/finalize operations | Timestamp check | Call `expire()` instead, or create a new proposal |
| `ProposalNotYetExpired` | `expire()` called before TTL elapses | Timestamp check | Wait until `block.timestamp > expiresAt` |
| `AgentNotParticipant` | Agent was not included in the proposal's participant list | Mapping check | Only invited agents can review |
| `AgentAlreadyReviewed` | Agent already submitted a review for this proposal | Mapping check | One review per agent per proposal. Create a new proposal for revised designs. |
| `AgentIsRevoked` | Revoked agent attempts to submit a review | Storage check | Agent has been removed from the council |
| `InvalidNonce` | EIP-712 nonce doesn't match expected value | Storage check | Use `getReviewNonce(agentTokenId)` to get the current nonce |
| `InvalidSignature` | ecrecover returns wrong address or zero | Signature verification | Check EIP-712 digest construction and signing key |
| `QuorumNotReached` | `finalize()` called but insufficient approvals | Loop count | Wait for more approvals or check if revoked agents reduced the count |
| `NotPrincipal` | Non-principal calls `reject()` | msg.sender check | Only the proposal's creator can reject it |
| `NotPauser` | Non-pauser calls `pause()`/`unpause()` | msg.sender check | Only the designated pauser address can pause/unpause |
| `TTLTooShort` | TTL < 1 hour | Input validation | Use a TTL >= 3600 seconds |
| `TTLTooLong` | TTL > 30 days | Input validation | Use a TTL <= 2592000 seconds |
| `AgentAlreadyRevoked` | `emergencyRevokeAgent()` called on already-revoked agent | Storage check | No action needed — agent is already revoked |

---

## Test Specification

All tests use Foundry (`forge test`). Tests are organized by function under test. Each test specifies: name, setup, action, and assertion.

### Setup (shared)

```solidity
contract TideCouncilTest is Test {
    TideCouncil public council;
    MockIdentityRegistry public identityRegistry;

    address public owner = makeAddr("owner");
    address public pauser = makeAddr("pauser");
    address public principal = makeAddr("principal");

    // Agents with known private keys for EIP-712 signing
    uint256 internal agentAlphaKey;
    address internal agentAlphaAddr;
    uint256 internal agentAlphaTokenId = 1;

    uint256 internal agentBetaKey;
    address internal agentBetaAddr;
    uint256 internal agentBetaTokenId = 2;

    uint256 internal agentGammaKey;
    address internal agentGammaAddr;
    uint256 internal agentGammaTokenId = 3;

    bytes32 internal designHash = keccak256("design-v1.md");
    bytes32 internal feedbackHash = keccak256("review-alpha.json");

    function setUp() public {
        // Generate deterministic keys
        (agentAlphaAddr, agentAlphaKey) = makeAddrAndKey("alpha");
        (agentBetaAddr, agentBetaKey) = makeAddrAndKey("beta");
        (agentGammaAddr, agentGammaKey) = makeAddrAndKey("gamma");

        // Deploy mock IdentityRegistry that maps tokenId → owner
        identityRegistry = new MockIdentityRegistry();
        identityRegistry.mint(agentAlphaAddr, agentAlphaTokenId);
        identityRegistry.mint(agentBetaAddr, agentBetaTokenId);
        identityRegistry.mint(agentGammaAddr, agentGammaTokenId);

        // Deploy implementation + proxy
        TideCouncil impl = new TideCouncil();
        bytes memory initData = abi.encodeCall(
            TideCouncil.initialize,
            (owner, address(identityRegistry), pauser, 72 hours, 2)
        );
        ERC1967Proxy proxy = new ERC1967Proxy(address(impl), initData);
        council = TideCouncil(address(proxy));
    }

    function _signReview(
        uint256 privateKey,
        uint256 proposalId,
        ITideCouncil.Verdict verdict,
        bytes32 _feedbackHash,
        uint256 agentTokenId,
        uint256 nonce
    ) internal view returns (bytes memory) {
        bytes32 structHash = keccak256(abi.encode(
            council.REVIEW_TYPEHASH(),
            proposalId,
            uint8(verdict),
            _feedbackHash,
            agentTokenId,
            nonce
        ));
        bytes32 digest = council.hashTypedDataV4(structHash);
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(privateKey, digest);
        return abi.encodePacked(r, s, v);
    }
}
```

### Proposal Tests

| # | Test | Setup | Action | Assertion |
|---|---|---|---|---|
| 1 | `test_propose_success` | Standard setUp | `principal` calls `propose(designHash, 0, [alpha, beta, gamma], 2, 72h)` | Returns proposalId=1. `getProposal(1)` returns correct fields. `ProposalCreated` event emitted with all params. |
| 2 | `test_propose_usesDefaults` | Standard setUp | `propose(designHash, 0, [alpha, beta], 0, 0)` with quorum=0 and ttl=0 | Uses defaultQuorum=2 and defaultTTL=72h. Stored proposal has quorum=2, expiresAt=block.timestamp+72h. |
| 3 | `test_propose_withParent` | Create proposal 1 first | `propose(hash2, 1, [alpha, beta, gamma], 2, 72h)` | Returns proposalId=2. `getProposal(2).parentProposalId == 1`. |
| 4 | `test_propose_revertsZeroDesignHash` | Standard setUp | `propose(bytes32(0), ...)` | Reverts `ZeroDesignHash()`. |
| 5 | `test_propose_revertsInvalidQuorum` | Standard setUp | `propose(hash, 0, [alpha], 2, 72h)` — quorum(2) > participants(1) | Reverts `InvalidQuorum(2, 1)`. |
| 6 | `test_propose_revertsParticipantNotRegistered` | Standard setUp | `propose(hash, 0, [alpha, 999], 2, 72h)` — tokenId 999 not minted | Reverts `ParticipantNotRegistered(999)`. |
| 7 | `test_propose_revertsParticipantRevoked` | Owner revokes alpha | `propose(hash, 0, [alpha, beta], 2, 72h)` | Reverts `ParticipantRevoked(alphaTokenId)`. |
| 8 | `test_propose_revertsParentNotFound` | Standard setUp | `propose(hash, 999, [alpha, beta], 2, 72h)` | Reverts `ParentProposalNotFound(999)`. |
| 9 | `test_propose_revertsTTLTooShort` | Standard setUp | `propose(hash, 0, [alpha, beta], 2, 60)` — 60 seconds | Reverts `TTLTooShort(60, 3600)`. |
| 10 | `test_propose_revertsTTLTooLong` | Standard setUp | `propose(hash, 0, [alpha, beta], 2, 31 days)` | Reverts `TTLTooLong(2678400, 2592000)`. |
| 11 | `test_propose_revertsWhenPaused` | Pauser pauses | `propose(...)` | Reverts with OpenZeppelin `EnforcedPause()`. |

### Review Tests

| # | Test | Setup | Action | Assertion |
|---|---|---|---|---|
| 12 | `test_submitReview_success` | Create proposal 1 | Sign and submit alpha's review (Approve, nonce=0) | Review stored. `getReviews(1)` returns 1 review. `ReviewSubmitted` emitted. Nonce incremented to 1. |
| 13 | `test_submitReview_requestChanges` | Create proposal 1 | Alpha submits RequestChanges verdict | Review stored with verdict=RequestChanges. |
| 14 | `test_submitReview_multipleAgents` | Create proposal 1 | Alpha approves (nonce=0), Beta approves (nonce=0), Gamma requests changes (nonce=0) | `getReviews(1)` returns 3 reviews. Each agent's nonce is 1. |
| 15 | `test_submitReview_revertsNotParticipant` | Create proposal 1 with [alpha, beta] only | Sign review with gamma's key for tokenId=gammaTokenId | Reverts `AgentNotParticipant(1, gammaTokenId)`. |
| 16 | `test_submitReview_revertsAlreadyReviewed` | Alpha submits review for proposal 1 | Alpha submits again (nonce=1, fresh signature) | Reverts `AgentAlreadyReviewed(1, alphaTokenId)`. |
| 17 | `test_submitReview_revertsInvalidNonce` | Create proposal 1 | Sign with nonce=5, but agent's nonce is 0 | Reverts `InvalidNonce(alphaTokenId, 0, 5)`. |
| 18 | `test_submitReview_revertsInvalidSignature` | Create proposal 1 | Sign with wrong private key (beta's key for alpha's tokenId) | Reverts `InvalidSignature()`. |
| 19 | `test_submitReview_revertsProposalExpired` | Create proposal, `vm.warp` past expiresAt | Submit review | Reverts `ProposalHasExpired(...)`. |
| 20 | `test_submitReview_revertsAgentRevoked` | Create proposal, then owner revokes alpha | Submit alpha's review | Reverts `AgentIsRevoked(alphaTokenId)`. |
| 21 | `test_submitReview_revertsProposalNotFound` | Standard setUp | Submit review for proposalId=999 | Reverts `ProposalNotFound(999)`. |
| 22 | `test_submitReview_revertsProposalNotProposed` | Finalize proposal 1 as Approved | Submit beta's review | Reverts `ProposalNotInProposedStatus(1, Approved)`. |

### Finalize Tests

| # | Test | Setup | Action | Assertion |
|---|---|---|---|---|
| 23 | `test_finalize_approvesWithExactQuorum` | Create proposal (quorum=2). Alpha + Beta approve. | `finalize(1)` | Status → Approved. `ProposalApproved` emitted. |
| 24 | `test_finalize_approvesWithExcessApprovals` | Create proposal (quorum=2). All 3 approve. | `finalize(1)` | Status → Approved. |
| 25 | `test_finalize_revertsQuorumNotReached` | Create proposal (quorum=2). Only Alpha approves. | `finalize(1)` | Reverts `QuorumNotReached(1, 1, 2)`. |
| 26 | `test_finalize_skipsRevokedAgentReviews` | Create proposal (quorum=2). Alpha + Beta approve. Owner revokes Alpha. | `finalize(1)` | Reverts `QuorumNotReached(1, 1, 2)` — only Beta's approval counts. |
| 27 | `test_finalize_revertsIfExpired` | Create proposal, `vm.warp` past expiresAt | `finalize(1)` | Reverts `ProposalHasExpired(...)`. |
| 28 | `test_finalize_revertsIfAlreadyApproved` | Finalize proposal 1 | `finalize(1)` again | Reverts `ProposalNotInProposedStatus(1, Approved)`. |
| 29 | `test_finalize_permissionless` | Alpha + Beta approve. | Random address calls `finalize(1)` | Succeeds — anyone can finalize. |
| 30 | `test_finalize_mixedVerdicts` | Alpha approves, Beta requests changes, Gamma approves. Quorum=2. | `finalize(1)` | Succeeds — 2 approvals >= quorum(2). RequestChanges verdicts are ignored for quorum count. |

### Reject Tests

| # | Test | Setup | Action | Assertion |
|---|---|---|---|---|
| 31 | `test_reject_principalCanReject` | Create proposal 1 | `principal` calls `reject(1)` | Status → Rejected. `ProposalRejected` emitted. |
| 32 | `test_reject_revertsNotPrincipal` | Create proposal 1 | Random address calls `reject(1)` | Reverts `NotPrincipal(1, random, principal)`. |
| 33 | `test_reject_revertsIfNotProposed` | Expire proposal 1 | `principal` calls `reject(1)` | Reverts `ProposalNotInProposedStatus(1, Expired)`. |

### Expire Tests

| # | Test | Setup | Action | Assertion |
|---|---|---|---|---|
| 34 | `test_expire_afterTTL` | Create proposal 1, `vm.warp` past expiresAt | `expire(1)` | Status → Expired. `ProposalExpired` emitted. |
| 35 | `test_expire_revertsIfNotExpired` | Create proposal 1 | `expire(1)` immediately | Reverts `ProposalNotYetExpired(1, expiresAt, block.timestamp)`. |
| 36 | `test_expire_permissionless` | Warp past TTL | Random address calls `expire(1)` | Succeeds — anyone can expire. |

### Admin Tests

| # | Test | Setup | Action | Assertion |
|---|---|---|---|---|
| 37 | `test_emergencyRevokeAgent` | Standard setUp | Owner calls `emergencyRevokeAgent(alphaTokenId)` | `isAgentRevoked(alphaTokenId)` returns true. `AgentRevoked` emitted. |
| 38 | `test_emergencyRevokeAgent_revertsNotOwner` | Standard setUp | Non-owner calls `emergencyRevokeAgent` | Reverts with OZ `OwnableUnauthorizedAccount`. |
| 39 | `test_emergencyRevokeAgent_revertsAlreadyRevoked` | Revoke alpha | Revoke alpha again | Reverts `AgentAlreadyRevoked(alphaTokenId)`. |
| 40 | `test_setPauser` | Standard setUp | Owner calls `setPauser(newPauser)` | `PauserUpdated` emitted. New pauser can pause. |
| 41 | `test_pause_haltsStateChanges` | Pauser pauses | `propose(...)`, `submitReview(...)`, `finalize(...)`, `reject(...)`, `expire(...)` | All revert with `EnforcedPause()`. |
| 42 | `test_pause_revertsNotPauser` | Standard setUp | Non-pauser calls `pause()` | Reverts `NotPauser(caller)`. |
| 43 | `test_unpause_restoresOperations` | Pauser pauses, then unpauses | `propose(...)` | Succeeds. |

### Upgrade Tests

| # | Test | Setup | Action | Assertion |
|---|---|---|---|---|
| 44 | `test_upgrade_ownerCanUpgrade` | Deploy v2 implementation | Owner calls `upgradeToAndCall(v2, "")` | Implementation slot updated. State preserved. |
| 45 | `test_upgrade_revertsNonOwner` | Standard setUp | Non-owner calls `upgradeToAndCall` | Reverts with OZ `OwnableUnauthorizedAccount`. |
| 46 | `test_upgrade_preservesState` | Create proposal, submit reviews | Upgrade to v2 | `getProposal(1)` returns same data. `getReviews(1)` returns same reviews. Nonces unchanged. |

### View Function Tests

| # | Test | Setup | Action | Assertion |
|---|---|---|---|---|
| 47 | `test_getParticipants` | Create proposal with [alpha, beta, gamma] | `getParticipants(1)` | Returns `[alphaTokenId, betaTokenId, gammaTokenId]` in insertion order. |
| 48 | `test_isParticipant` | Create proposal with [alpha, beta] | `isParticipant(1, gammaTokenId)` | Returns false. |
| 49 | `test_getReviewNonce` | Alpha submits 2 reviews (different proposals) | `getReviewNonce(alphaTokenId)` | Returns 2. |

---

## Deployment

### Contract Deployment

1. Deploy `TideCouncil` implementation contract (no constructor arguments).
2. Compute `initialize()` calldata with deployment parameters.
3. Deploy `ERC1967Proxy` with the implementation address and calldata.
4. Verify both contracts on SeiScan (SeiTrace).

### Deployment Parameters

| Parameter | Testnet (arctic-1) | Mainnet (pacific-1) |
|---|---|---|
| `owner_` | Deployer EOA | 2-of-3 multisig |
| `identityRegistry_` | Deployed MockIdentityRegistry or ERC-8004 address | ERC-8004 IdentityRegistry address |
| `pauser_` | Deployer EOA | 2-of-3 multisig |
| `defaultTTL_` | 7200 (2 hours, for fast testing) | 259200 (72 hours) |
| `defaultQuorum_` | 2 | 2 |

### Post-Deployment Verification

1. Call `getProposal(1)` — should revert with `ProposalNotFound` (no proposals yet).
2. Call `proposalCount()` — should return 0.
3. Call `owner()` — should return expected owner.
4. Call `paused()` — should return false.
5. Verify proxy's implementation slot points to correct implementation address.
6. Submit a test proposal on testnet and exercise the full lifecycle.

---

## Deferred (Do Not Build)

| Feature | Rationale |
|---|---|
| **Negative quorum / automatic rejection** | YAGNI. The principal reads feedback and decides to revise or reject. No business need for automated rejection in Phase 0-2. |
| **Reputation gating on council membership** | Phase 3+. TideCouncil does not check agent reputation — only identity registration. |
| **Token-weighted voting** | Explicitly deferred per constitution. |
| **On-chain design document storage** | Only hashes stored on-chain. Full content lives on GitHub. |
| **Un-revoking agents** | YAGNI. If needed, deploy new implementation via UUPS upgrade. |
| **Batch review submission** | One review per call is sufficient for 3 agents. Gas optimization for large councils is deferred. |
| **Proposal amendment (edit after creation)** | Create a new proposal with `parentProposalId`. Editing in place adds complexity without value. |
| **Event replay / historical query helpers** | Off-chain indexer (operator) handles this. No on-chain log history needed. |
| **Role-based access control (AccessControl)** | Two roles (owner + pauser) are sufficient. Full RBAC adds unnecessary complexity. |
