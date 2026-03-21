# Component: TideJobHook

## Owner

Blockchain Developer

## Phase

0

## Purpose

TideJobHook is a custom hook contract implementing ERC-8183's `IACPHook` interface. It bridges the AgenticCommerce job lifecycle to Tide-specific behavior: verifying agent identity and reputation before jobs are funded, emitting events that the Tide Operator watches to provision GitHub sandboxes, and posting reputation feedback to the ERC-8004 ReputationRegistry when jobs complete or are rejected.

The hook composes ERC-8183 with ERC-8004 exactly as both standards recommend, without modifying either.

**Business needs served:**
1. Decompose into funded USDC jobs — the hook gates funding on agent identity (Phase 2)
2. Execute in isolated GitHub sandboxes — the hook emits the provisioning trigger event (Phase 2)
3. Evaluate + release USDC — the hook records reputation after evaluation (Phase 2)

---

## Dependencies

### External

| Dependency | Interface | Notes |
|---|---|---|
| **ERC-8183 AgenticCommerce (ACP)** | `IACP` — see interface below | The ACP contract calls this hook during job lifecycle transitions. The hook reads job details via `IACP.getJob(jobId)`. The ACP address is set as `immutable` in the constructor. |
| **ERC-8004 IdentityRegistry** | `IERC721` + `IERC721Enumerable` | Used in `beforeAction(fund)` to verify the provider has a registered on-chain identity. Requires `balanceOf(address)` and `tokenOfOwnerByIndex(address, uint256)`. |
| **ERC-8004 ReputationRegistry** | `IReputationRegistry` — see interface below | Used in `afterAction(complete)` and `afterAction(reject)` to post feedback signals. |
| **OpenZeppelin Contracts v5.x** | `Ownable2Step`, `Pausable`, `ReentrancyGuard`, `IERC165` | Access control, emergency pause, reentrancy protection, interface detection. |

### Internal

| Dependency | Interface | Notes |
|---|---|---|
| **TideConstants** | `TideConstants.sol` | Shared action selectors and constants. |

### Explicit Exclusions

- **Does NOT depend on** TideCouncil. The hook operates on ERC-8183 jobs independently of the design review lifecycle.
- **Does NOT depend on** USDC directly. USDC transfers are handled by the ACP contract. The hook only reads job metadata.
- **NOT upgradeable.** No UUPS proxy. If the hook needs changes, deploy a new version and set it on new jobs. Old jobs retain the old hook.

---

## Interface Specification

### Assumed External Interfaces

These interfaces are dependencies. The actual deployed contracts must match these signatures.

#### IACP (ERC-8183 AgenticCommerce)

```solidity
/// @notice Minimal interface for reading job details from the ERC-8183 ACP contract.
/// @dev Dependency: the deployed ERC-8183 reference implementation must expose getJob().
///      If it does not, TideJobHook must be updated to decode the `data` parameter instead.
///      This is documented as a known dependency in the deployment checklist.
interface IACP {
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

    /// @notice Returns the full job struct for a given job ID.
    function getJob(uint256 jobId) external view returns (Job memory);
}
```

**Dependency note:** If the ERC-8183 reference implementation uses a different struct layout or function name, this interface must be updated before deployment. Verify against the deployed ACP ABI during the deployment checklist (see `lld-contract-deployment.md`).

#### IIdentityRegistry (ERC-8004)

```solidity
/// @notice Minimal ERC-8004 IdentityRegistry interface.
/// @dev Extends ERC-721. Requires Enumerable extension for address → tokenId resolution.
interface IIdentityRegistry {
    /// @notice Returns the owner of a given token ID. Reverts if token doesn't exist.
    function ownerOf(uint256 tokenId) external view returns (address);

    /// @notice Returns the number of tokens owned by an address.
    function balanceOf(address owner) external view returns (uint256);

    /// @notice Returns the token ID at a given index for an owner.
    /// @dev Requires ERC-721 Enumerable. Index 0 returns the agent's identity token.
    function tokenOfOwnerByIndex(address owner, uint256 index) external view returns (uint256);
}
```

**Dependency note:** ERC-8004 uses ERC-721 for identity. This hook assumes the Enumerable extension is supported. If the deployed ERC-8004 IdentityRegistry does not support `tokenOfOwnerByIndex`, an adapter contract or a reverse-lookup mapping must be added to TideJobHook.

#### IReputationRegistry (ERC-8004)

```solidity
/// @notice Minimal ERC-8004 ReputationRegistry interface for posting feedback.
interface IReputationRegistry {
    /// @notice Posts a feedback signal for an agent.
    /// @param tokenId The agent's ERC-8004 identity token ID.
    /// @param score A 0-100 quality score.
    /// @param tags Categorization tags for the feedback (e.g., "job-complete").
    function submitFeedback(
        uint256 tokenId,
        uint8 score,
        bytes32[] calldata tags
    ) external;
}
```

**Dependency note:** The exact function signature for `submitFeedback` must match the deployed ERC-8004 ReputationRegistry. If the deployed contract uses different parameter types (e.g., `string[]` instead of `bytes32[]`), the hook's internal calls must be adjusted. Verify during deployment.

**Authorization requirement:** The TideJobHook contract address must be authorized to call `submitFeedback()` on the ReputationRegistry. This is a post-deployment setup step — see deployment checklist.

### Action Selectors

```solidity
/// @notice ERC-8183 function selectors passed to hooks.
/// @dev These must match the actual function signatures on the deployed ACP contract.
///      Computed as bytes4(keccak256("<function signature>")).
///
///      Assumed ACP function signatures:
///        fund(uint256 jobId)
///        submit(uint256 jobId, bytes32 deliverableHash)
///        complete(uint256 jobId)
///        reject(uint256 jobId)
///
///      If the ACP uses different signatures (e.g., complete(uint256,string)),
///      these constants must be updated before deployment.

bytes4 public constant FUND_SELECTOR = bytes4(keccak256("fund(uint256)"));
bytes4 public constant SUBMIT_SELECTOR = bytes4(keccak256("submit(uint256,bytes32)"));
bytes4 public constant COMPLETE_SELECTOR = bytes4(keccak256("complete(uint256)"));
bytes4 public constant REJECT_SELECTOR = bytes4(keccak256("reject(uint256)"));
```

### Reputation Constants

```solidity
/// @notice Tag posted to ReputationRegistry on successful job completion.
bytes32 public constant TAG_JOB_COMPLETE = keccak256("job-complete");

/// @notice Tag posted to ReputationRegistry on job rejection.
bytes32 public constant TAG_JOB_REJECTED = keccak256("job-rejected");
```

### Events

All events below are **cross-component interfaces** consumed by the Tide Operator. Event signatures, indexed fields, and parameter order are one-way doors.

```solidity
/// @notice Emitted after a job is funded. The Tide Operator watches this event to
///         provision the agent's GitHub sandbox (create workspace repo, generate
///         App installation token, clone upstream source).
/// @dev Topic[0]: keccak256("SandboxProvisionRequested(uint256,address,address,uint256,uint256,uint256)")
/// @param jobId The ERC-8183 job ID that was just funded.
/// @param provider The agent's wallet address (the job's provider).
/// @param client The principal's wallet address (the job's client / treasury).
/// @param agentTokenId The agent's ERC-8004 identity token ID.
/// @param budget The USDC amount escrowed for this job (in raw 6-decimal units).
/// @param expiry Unix timestamp when the job expires.
event SandboxProvisionRequested(
    uint256 indexed jobId,
    address indexed provider,
    address indexed client,
    uint256 agentTokenId,
    uint256 budget,
    uint256 expiry
);

/// @notice Emitted after a positive reputation signal is posted for a completed job.
/// @dev Topic[0]: keccak256("ReputationUpdated(uint256,uint256,uint8,bool)")
/// @param jobId The completed job.
/// @param agentTokenId The agent that completed the job.
/// @param score The reputation score posted (0-100).
/// @param positive True for completion, false for rejection.
event ReputationUpdated(
    uint256 indexed jobId,
    uint256 indexed agentTokenId,
    uint8 score,
    bool positive
);

/// @notice Emitted when the minimum reputation score threshold is updated.
event MinReputationScoreUpdated(
    uint256 oldScore,
    uint256 newScore
);

/// @notice Emitted when the completion or rejection score is updated.
event ReputationScoreConfigUpdated(
    uint8 oldCompletionScore,
    uint8 newCompletionScore,
    uint8 oldRejectionScore,
    uint8 newRejectionScore
);
```

#### Event Signature Strings (for operator topic hash derivation)

| Event | Canonical Signature |
|---|---|
| `SandboxProvisionRequested` | `SandboxProvisionRequested(uint256,address,address,uint256,uint256,uint256)` |
| `ReputationUpdated` | `ReputationUpdated(uint256,uint256,uint8,bool)` |
| `MinReputationScoreUpdated` | `MinReputationScoreUpdated(uint256,uint256)` |
| `ReputationScoreConfigUpdated` | `ReputationScoreConfigUpdated(uint8,uint8,uint8,uint8)` |

Topic[0] for each event = `keccak256(bytes(canonical_signature))`.

### Custom Errors

```solidity
/// @notice Caller is not the ACP contract.
/// @param caller The unauthorized caller address.
error UnauthorizedCaller(address caller);

/// @notice The job's provider does not have an ERC-8004 identity (balanceOf == 0).
/// @param provider The provider address without an identity.
error ProviderNotRegistered(address provider);

/// @notice The job's provider has a reputation score below the minimum threshold.
/// @param provider The provider address.
/// @param agentTokenId The agent's identity token ID.
/// @param score The agent's current reputation score.
/// @param required The minimum required score.
error ReputationBelowThreshold(
    address provider,
    uint256 agentTokenId,
    uint256 score,
    uint256 required
);

/// @notice The ACP getJob() call returned a provider of address(0), indicating the job
///         does not exist or is malformed.
/// @param jobId The invalid job ID.
error InvalidJob(uint256 jobId);

/// @notice Score value exceeds 100.
/// @param score The invalid score.
error InvalidScore(uint8 score);

/// @notice pause() or unpause() called by non-pauser.
/// @param caller The unauthorized caller.
error NotPauser(address caller);
```

### Functions

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Ownable2Step, Ownable} from "@openzeppelin/contracts/access/Ownable2Step.sol";
import {Pausable} from "@openzeppelin/contracts/security/Pausable.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/security/ReentrancyGuard.sol";
import {IERC165} from "@openzeppelin/contracts/utils/introspection/IERC165.sol";

/// @title TideJobHook
/// @notice ERC-8183 hook that bridges job lifecycle events to Tide infrastructure.
/// @dev Not upgradeable. Deploy a new version and set it on new jobs if changes are needed.
contract TideJobHook is IERC165, Ownable2Step, Pausable, ReentrancyGuard {

    // ── Immutables ─────────────────────────────────────────

    /// @notice The ERC-8183 ACP contract address. Only this address can call hook functions.
    address public immutable acpContract;

    /// @notice The ERC-8004 IdentityRegistry for agent identity verification.
    IIdentityRegistry public immutable identityRegistry;

    /// @notice The ERC-8004 ReputationRegistry for posting feedback signals.
    IReputationRegistry public immutable reputationRegistry;

    // ── Mutable State ──────────────────────────────────────

    /// @notice Minimum reputation score required to fund a job for an agent.
    ///         Set to 0 initially (any registered agent can participate).
    uint256 public minReputationScore;

    /// @notice Address authorized to pause/unpause the hook.
    address public pauser;

    /// @notice Reputation score posted on successful job completion (0-100).
    uint8 public completionScore;

    /// @notice Reputation score posted on job rejection (0-100).
    uint8 public rejectionScore;

    // ── Constructor ────────────────────────────────────────

    /// @notice Deploys the TideJobHook with immutable dependencies.
    /// @param acpContract_ The deployed ERC-8183 ACP contract address.
    /// @param identityRegistry_ The deployed ERC-8004 IdentityRegistry address.
    /// @param reputationRegistry_ The deployed ERC-8004 ReputationRegistry address.
    /// @param owner_ The initial owner (can update config and transfer ownership).
    /// @param pauser_ The initial pauser address (2-of-3 multisig recommended).
    constructor(
        address acpContract_,
        address identityRegistry_,
        address reputationRegistry_,
        address owner_,
        address pauser_
    ) Ownable(owner_) {
        acpContract = acpContract_;
        identityRegistry = IIdentityRegistry(identityRegistry_);
        reputationRegistry = IReputationRegistry(reputationRegistry_);
        pauser = pauser_;
        completionScore = 80;
        rejectionScore = 20;
    }

    // ── Modifiers ──────────────────────────────────────────

    /// @notice Restricts calls to the ACP contract only.
    /// @dev Critical access control. Without this, anyone could call afterAction
    ///      with COMPLETE_SELECTOR to post fraudulent positive reputation signals.
    modifier onlyACP() {
        if (msg.sender != acpContract) revert UnauthorizedCaller(msg.sender);
        _;
    }

    // ── IACPHook Implementation ────────────────────────────

    /// @notice Called by the ACP before a job action executes.
    /// @dev Hook behavior by selector:
    ///      - FUND_SELECTOR: Verifies provider has ERC-8004 identity and meets
    ///        minimum reputation threshold. Reverts to block funding if checks fail.
    ///      - All other selectors: No-op (returns without reverting).
    /// @param jobId The ERC-8183 job ID.
    /// @param selector The function selector of the ACP action being executed.
    /// @param data ABI-encoded action context from the ACP. Not decoded in v1 —
    ///        job details are read via IACP.getJob(jobId) callback instead.
    ///        See "Data Parameter Encoding" section for documented assumptions.
    function beforeAction(
        uint256 jobId,
        bytes4 selector,
        bytes calldata data
    ) external onlyACP nonReentrant whenNotPaused {
        if (selector == FUND_SELECTOR) {
            _validateProviderIdentity(jobId);
        }
        // All other selectors: no-op (beforeAction(submit), etc.)
    }

    /// @notice Called by the ACP after a job action completes.
    /// @dev Hook behavior by selector:
    ///      - FUND_SELECTOR: Emits SandboxProvisionRequested for the Tide Operator.
    ///      - COMPLETE_SELECTOR: Posts positive reputation to ERC-8004 ReputationRegistry.
    ///      - REJECT_SELECTOR: Posts negative reputation to ERC-8004 ReputationRegistry.
    ///      - All other selectors: No-op.
    /// @param jobId The ERC-8183 job ID.
    /// @param selector The function selector of the ACP action that just completed.
    /// @param data ABI-encoded action context from the ACP. Not decoded in v1.
    function afterAction(
        uint256 jobId,
        bytes4 selector,
        bytes calldata data
    ) external onlyACP nonReentrant whenNotPaused {
        if (selector == FUND_SELECTOR) {
            _emitSandboxProvisionRequest(jobId);
        } else if (selector == COMPLETE_SELECTOR) {
            _postReputation(jobId, completionScore, TAG_JOB_COMPLETE, true);
        } else if (selector == REJECT_SELECTOR) {
            _postReputation(jobId, rejectionScore, TAG_JOB_REJECTED, false);
        }
        // All other selectors: no-op
    }

    // ── Admin Functions ────────────────────────────────────

    /// @notice Updates the minimum reputation score for job funding. Owner-only.
    /// @param newMinScore New minimum score (0-100). Set to 0 to allow any registered agent.
    function setMinReputationScore(uint256 newMinScore) external onlyOwner {
        uint256 oldScore = minReputationScore;
        minReputationScore = newMinScore;
        emit MinReputationScoreUpdated(oldScore, newMinScore);
    }

    /// @notice Updates the reputation scores posted on completion/rejection. Owner-only.
    /// @param newCompletionScore Score for completed jobs (0-100).
    /// @param newRejectionScore Score for rejected jobs (0-100).
    function setReputationScores(
        uint8 newCompletionScore,
        uint8 newRejectionScore
    ) external onlyOwner {
        if (newCompletionScore > 100) revert InvalidScore(newCompletionScore);
        if (newRejectionScore > 100) revert InvalidScore(newRejectionScore);
        emit ReputationScoreConfigUpdated(
            completionScore, newCompletionScore,
            rejectionScore, newRejectionScore
        );
        completionScore = newCompletionScore;
        rejectionScore = newRejectionScore;
    }

    /// @notice Updates the pauser address. Owner-only.
    function setPauser(address newPauser) external onlyOwner {
        pauser = newPauser;
    }

    /// @notice Pauses all hook functions. Pauser-only.
    function pause() external {
        if (msg.sender != pauser) revert NotPauser(msg.sender);
        _pause();
    }

    /// @notice Unpauses the hook. Pauser-only.
    function unpause() external {
        if (msg.sender != pauser) revert NotPauser(msg.sender);
        _unpause();
    }

    // ── ERC-165 ────────────────────────────────────────────

    /// @notice Returns true for IACPHook and IERC165 interface IDs.
    function supportsInterface(bytes4 interfaceId) external pure returns (bool) {
        return interfaceId == type(IACPHook).interfaceId
            || interfaceId == type(IERC165).interfaceId;
    }

    // ── Internal Functions ─────────────────────────────────
    // (see Internal Design section for implementation details)

    function _validateProviderIdentity(uint256 jobId) internal view { /* ... */ }
    function _emitSandboxProvisionRequest(uint256 jobId) internal { /* ... */ }
    function _postReputation(uint256 jobId, uint8 score, bytes32 tag, bool positive) internal { /* ... */ }
    function _resolveAgentTokenId(address provider) internal view returns (uint256) { /* ... */ }
}
```

### IACPHook Interface

```solidity
/// @notice ERC-8183 hook interface. Implemented by TideJobHook.
interface IACPHook {
    function beforeAction(uint256 jobId, bytes4 selector, bytes calldata data) external;
    function afterAction(uint256 jobId, bytes4 selector, bytes calldata data) external;
}
```

### Complete Interface (ITideJobHook.sol)

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

interface ITideJobHook {
    // ── Events ─────────────────────────────────────────────
    event SandboxProvisionRequested(
        uint256 indexed jobId,
        address indexed provider,
        address indexed client,
        uint256 agentTokenId,
        uint256 budget,
        uint256 expiry
    );

    event ReputationUpdated(
        uint256 indexed jobId,
        uint256 indexed agentTokenId,
        uint8 score,
        bool positive
    );

    event MinReputationScoreUpdated(uint256 oldScore, uint256 newScore);

    event ReputationScoreConfigUpdated(
        uint8 oldCompletionScore,
        uint8 newCompletionScore,
        uint8 oldRejectionScore,
        uint8 newRejectionScore
    );

    // ── Errors ─────────────────────────────────────────────
    error UnauthorizedCaller(address caller);
    error ProviderNotRegistered(address provider);
    error ReputationBelowThreshold(
        address provider,
        uint256 agentTokenId,
        uint256 score,
        uint256 required
    );
    error InvalidJob(uint256 jobId);
    error InvalidScore(uint8 score);
    error NotPauser(address caller);

    // ── Hook Functions (called by ACP only) ────────────────
    function beforeAction(uint256 jobId, bytes4 selector, bytes calldata data) external;
    function afterAction(uint256 jobId, bytes4 selector, bytes calldata data) external;

    // ── Admin Functions ────────────────────────────────────
    function setMinReputationScore(uint256 newMinScore) external;
    function setReputationScores(uint8 newCompletionScore, uint8 newRejectionScore) external;
    function setPauser(address newPauser) external;
    function pause() external;
    function unpause() external;

    // ── View Functions ─────────────────────────────────────
    function acpContract() external view returns (address);
    function identityRegistry() external view returns (address);
    function reputationRegistry() external view returns (address);
    function minReputationScore() external view returns (uint256);
    function pauser() external view returns (address);
    function completionScore() external view returns (uint8);
    function rejectionScore() external view returns (uint8);
    function FUND_SELECTOR() external view returns (bytes4);
    function SUBMIT_SELECTOR() external view returns (bytes4);
    function COMPLETE_SELECTOR() external view returns (bytes4);
    function REJECT_SELECTOR() external view returns (bytes4);
    function TAG_JOB_COMPLETE() external view returns (bytes32);
    function TAG_JOB_REJECTED() external view returns (bytes32);
}
```

---

## State Model

TideJobHook is **not upgradeable** — no proxy, no ERC-7201 namespacing needed.

### Storage Layout

```
┌──────┬─────────────────────────────────────────────────────────────────┐
│ Slot │ Field                                                           │
├──────┼─────────────────────────────────────────────────────────────────┤
│  —   │ address acpContract          (immutable, stored in bytecode)    │
│  —   │ IIdentityRegistry identityRegistry  (immutable, in bytecode)   │
│  —   │ IReputationRegistry reputationRegistry (immutable, in bytecode)│
├──────┼─────────────────────────────────────────────────────────────────┤
│  0   │ address _owner               (from Ownable, 20 bytes)          │
├──────┼─────────────────────────────────────────────────────────────────┤
│  1   │ address _pendingOwner        (from Ownable2Step, 20 bytes)     │
├──────┼─────────────────────────────────────────────────────────────────┤
│  2   │ bool _paused                 (from Pausable, 1 byte)           │
├──────┼─────────────────────────────────────────────────────────────────┤
│  3   │ uint256 _status              (from ReentrancyGuard)            │
├──────┼─────────────────────────────────────────────────────────────────┤
│  4   │ uint256 minReputationScore   (custom, 32 bytes)                │
├──────┼─────────────────────────────────────────────────────────────────┤
│  5   │ address pauser               (custom, 20 bytes)                │
├──────┼─────────────────────────────────────────────────────────────────┤
│  6   │ uint8 completionScore | uint8 rejectionScore                   │
│      │ (custom, packed, 2 bytes used)                                 │
└──────┴─────────────────────────────────────────────────────────────────┘
```

**Note:** Slots 0-3 are owned by OpenZeppelin base contracts. Custom state starts at slot 4. Since this contract is not upgradeable, storage layout is not a one-way door — it can be changed by deploying a new version.

Immutables (`acpContract`, `identityRegistry`, `reputationRegistry`) are stored in the contract bytecode, not in storage slots. They cost zero gas to read (no SLOAD).

---

## Internal Design

### _validateProviderIdentity(jobId)

Called during `beforeAction(fund)`. Reverts to prevent funding if the provider is unregistered or below reputation threshold.

```
1. Call IACP(acpContract).getJob(jobId) → job
2. Validate job.provider != address(0)                    → revert InvalidJob(jobId)
3. Check identityRegistry.balanceOf(job.provider) > 0     → revert ProviderNotRegistered(provider)
4. agentTokenId = identityRegistry.tokenOfOwnerByIndex(job.provider, 0)
5. If minReputationScore > 0:
   a. Call reputationRegistry.getAverageScore(agentTokenId) → score
      (If the registry returns 0 for agents with no history, new agents
       will be blocked when minReputationScore > 0.)
   b. Validate score >= minReputationScore
      → revert ReputationBelowThreshold(provider, agentTokenId, score, minReputationScore)
```

**Gas profile:** 2-3 external calls (getJob ~2,600 gas warm, balanceOf ~2,600 gas, tokenOfOwnerByIndex ~2,600 gas, optional getAverageScore ~2,600 gas). Total approximately 50,000-70,000 gas.

**Note on `minReputationScore = 0` (default):** When set to 0, the reputation check (step 5) is skipped entirely. Any agent with a valid ERC-8004 identity can be funded. This is the correct default for Phase 0-2 where agents have no reputation history yet.

### _emitSandboxProvisionRequest(jobId)

Called during `afterAction(fund)`. Emits the event that triggers the Tide Operator's sandbox provisioning flow.

```
1. Call IACP(acpContract).getJob(jobId) → job
2. agentTokenId = _resolveAgentTokenId(job.provider)
3. Emit SandboxProvisionRequested(
       jobId,
       job.provider,    // indexed
       job.client,      // indexed
       agentTokenId,
       job.budget,
       job.expiry
   )
```

**Note:** The `getJob()` call here reads from the ACP after funding has completed (afterAction), so `job.status` will be `Funded`. All job fields (provider, client, budget, expiry) are available.

### _postReputation(jobId, score, tag, positive)

Called during `afterAction(complete)` and `afterAction(reject)`.

```
1. Call IACP(acpContract).getJob(jobId) → job
2. agentTokenId = _resolveAgentTokenId(job.provider)
3. Construct tags array:
   bytes32[] memory tags = new bytes32[](1);
   tags[0] = tag;
4. Call reputationRegistry.submitFeedback(agentTokenId, score, tags)
5. Emit ReputationUpdated(jobId, agentTokenId, score, positive)
```

**Reputation scoring policy:**

| Outcome | Score | Tag | Rationale |
|---|---|---|---|
| Job completed (`afterAction(complete)`) | `completionScore` (default: 80) | `TAG_JOB_COMPLETE` | Baseline positive signal. Evaluator approved the deliverable. |
| Job rejected (`afterAction(reject)`) | `rejectionScore` (default: 20) | `TAG_JOB_REJECTED` | Negative signal. Deliverable did not meet acceptance criteria. |

Both scores are owner-configurable via `setReputationScores()`. The 80/20 defaults provide meaningful differentiation without being extreme. A score of 0 (catastrophic failure) or 100 (exceptional work) can be set for special cases via governance.

### _resolveAgentTokenId(provider)

Resolves a provider wallet address to its ERC-8004 token ID.

```
1. Validate identityRegistry.balanceOf(provider) > 0     → revert ProviderNotRegistered(provider)
2. Return identityRegistry.tokenOfOwnerByIndex(provider, 0)
```

**Assumption:** Each agent has exactly one ERC-8004 identity token. `tokenOfOwnerByIndex(provider, 0)` returns the first (and only) token. If an agent transfers their identity token, they lose their link to this hook — all reputation is associated with the token, not the wallet address.

### Data Parameter Encoding (ERC-8183 Dependency)

The `data` parameter in `beforeAction`/`afterAction` is passed through from the ERC-8183 ACP contract. TideJobHook v1 **does not decode** this parameter. Instead, it reads all necessary job details via the `IACP.getJob(jobId)` callback.

**Documented assumption:** The `data` parameter is ABI-encoded context that may contain:

| Action | Expected `data` encoding (assumed, not consumed) |
|---|---|
| `fund` | `abi.encode(address client, uint256 amount)` |
| `submit` | `abi.encode(address provider, bytes32 deliverableHash)` |
| `complete` | `abi.encode(address evaluator)` |
| `reject` | `abi.encode(address evaluator)` |

**If the ERC-8183 reference implementation does not expose `IACP.getJob()`**, the hook must be rewritten to decode `data` instead. This is a blocking dependency — verify during deployment (see `lld-contract-deployment.md` deployment checklist, step 3).

### Hook Behavior Matrix

| `selector` | `beforeAction` | `afterAction` |
|---|---|---|
| `FUND_SELECTOR` | Validate identity + reputation. Revert to block funding if invalid. | Emit `SandboxProvisionRequested`. |
| `SUBMIT_SELECTOR` | No-op. | No-op. |
| `COMPLETE_SELECTOR` | No-op. | Post positive reputation. Emit `ReputationUpdated`. |
| `REJECT_SELECTOR` | No-op. | Post negative reputation. Emit `ReputationUpdated`. |
| Any other | No-op. | No-op. |

---

## Error Handling

| Error | Cause | Detection | Caller/Operator Action |
|---|---|---|---|
| `UnauthorizedCaller` | `beforeAction` or `afterAction` called by address other than `acpContract` | `msg.sender` check | This is a security violation. Only the ACP should call hooks. Investigate. |
| `ProviderNotRegistered` | Job provider has no ERC-8004 identity | `balanceOf == 0` | Register the agent on ERC-8004 before funding the job. |
| `ReputationBelowThreshold` | Provider's reputation score is below `minReputationScore` | Score comparison | Agent needs more positive job completions, or owner lowers the threshold. |
| `InvalidJob` | `getJob()` returned a zero-address provider | Provider check | Job ID is invalid or the ACP contract is misconfigured. |
| `InvalidScore` | `setReputationScores` called with score > 100 | Input validation | Scores must be in [0, 100]. |
| `NotPauser` | `pause()`/`unpause()` called by non-pauser | `msg.sender` check | Only the pauser address can pause/unpause. |
| OZ `EnforcedPause` | Hook function called while paused | Pausable check | Unpause the contract before proceeding. |
| OZ `ReentrancyGuardReentrantCall` | Reentrant call to hook functions | Reentrancy guard | Investigate the calling contract for reentrancy patterns. |
| External call revert in `submitFeedback` | ReputationRegistry rejects the feedback | External call | Check authorization — TideJobHook must be whitelisted to submit feedback. |
| External call revert in `getJob` | ACP contract reverts (e.g., job doesn't exist) | External call | Verify job exists before the ACP calls the hook (this should not happen in normal flow). |

---

## Test Specification

All tests use Foundry (`forge test`). A `MockACP` contract simulates the ERC-8183 ACP calling the hook.

### Setup (shared)

```solidity
contract TideJobHookTest is Test {
    TideJobHook public hook;
    MockACP public acp;
    MockIdentityRegistry public identityRegistry;
    MockReputationRegistry public reputationRegistry;

    address public owner = makeAddr("owner");
    address public pauser = makeAddr("pauser");
    address public agentWallet = makeAddr("agent");
    address public clientWallet = makeAddr("client");
    uint256 public agentTokenId = 1;

    function setUp() public {
        identityRegistry = new MockIdentityRegistry();
        identityRegistry.mint(agentWallet, agentTokenId);

        reputationRegistry = new MockReputationRegistry();

        acp = new MockACP();
        // Configure mock job: provider=agentWallet, client=clientWallet,
        // budget=36_000_000 (36 USDC), expiry=block.timestamp+7days
        acp.setJob(1, IACP.Job({
            client: clientWallet,
            provider: agentWallet,
            evaluator: makeAddr("evaluator"),
            hook: address(0), // set after hook deploy
            budget: 36_000_000,
            expiry: uint256(block.timestamp + 7 days),
            description: "Implement feature X",
            status: IACP.JobStatus.Open
        }));

        hook = new TideJobHook(
            address(acp),
            address(identityRegistry),
            address(reputationRegistry),
            owner,
            pauser
        );
    }
}
```

### beforeAction Tests

| # | Test | Setup | Action | Assertion |
|---|---|---|---|---|
| 1 | `test_beforeAction_fund_success` | Standard setUp, agent registered | `acp` calls `hook.beforeAction(1, FUND_SELECTOR, "")` | Does not revert. |
| 2 | `test_beforeAction_fund_revertsProviderNotRegistered` | Remove agent from identity registry | `acp` calls `hook.beforeAction(1, FUND_SELECTOR, "")` | Reverts `ProviderNotRegistered(agentWallet)`. |
| 3 | `test_beforeAction_fund_revertsReputationBelowThreshold` | Owner sets `minReputationScore = 50`. Mock returns score 30 for agent. | `acp` calls `hook.beforeAction(1, FUND_SELECTOR, "")` | Reverts `ReputationBelowThreshold(agentWallet, agentTokenId, 30, 50)`. |
| 4 | `test_beforeAction_fund_skipsReputationCheckWhenZero` | `minReputationScore = 0` (default). Mock returns score 0. | `acp` calls `hook.beforeAction(1, FUND_SELECTOR, "")` | Does not revert — score check is skipped. |
| 5 | `test_beforeAction_fund_revertsInvalidJob` | ACP returns provider=address(0) for job 999 | `acp` calls `hook.beforeAction(999, FUND_SELECTOR, "")` | Reverts `InvalidJob(999)`. |
| 6 | `test_beforeAction_submit_isNoOp` | Standard setUp | `acp` calls `hook.beforeAction(1, SUBMIT_SELECTOR, "")` | Does not revert. No state changes. |
| 7 | `test_beforeAction_unknownSelector_isNoOp` | Standard setUp | `acp` calls `hook.beforeAction(1, bytes4(0xdeadbeef), "")` | Does not revert. |

### afterAction Tests

| # | Test | Setup | Action | Assertion |
|---|---|---|---|---|
| 8 | `test_afterAction_fund_emitsSandboxProvisionRequested` | Standard setUp | `acp` calls `hook.afterAction(1, FUND_SELECTOR, "")` | Emits `SandboxProvisionRequested(1, agentWallet, clientWallet, agentTokenId, 36_000_000, expiry)`. All indexed fields correct. |
| 9 | `test_afterAction_complete_postsPositiveReputation` | Standard setUp | `acp` calls `hook.afterAction(1, COMPLETE_SELECTOR, "")` | `reputationRegistry.submitFeedback` called with `(agentTokenId, 80, [TAG_JOB_COMPLETE])`. `ReputationUpdated(1, agentTokenId, 80, true)` emitted. |
| 10 | `test_afterAction_reject_postsNegativeReputation` | Standard setUp | `acp` calls `hook.afterAction(1, REJECT_SELECTOR, "")` | `reputationRegistry.submitFeedback` called with `(agentTokenId, 20, [TAG_JOB_REJECTED])`. `ReputationUpdated(1, agentTokenId, 20, false)` emitted. |
| 11 | `test_afterAction_submit_isNoOp` | Standard setUp | `acp` calls `hook.afterAction(1, SUBMIT_SELECTOR, "")` | No events emitted. No state changes. |
| 12 | `test_afterAction_unknownSelector_isNoOp` | Standard setUp | `acp` calls `hook.afterAction(1, bytes4(0xdeadbeef), "")` | No events emitted. |

### Access Control Tests

| # | Test | Setup | Action | Assertion |
|---|---|---|---|---|
| 13 | `test_beforeAction_revertsUnauthorizedCaller` | Standard setUp | Non-ACP address calls `hook.beforeAction(...)` | Reverts `UnauthorizedCaller(caller)`. |
| 14 | `test_afterAction_revertsUnauthorizedCaller` | Standard setUp | Non-ACP address calls `hook.afterAction(...)` | Reverts `UnauthorizedCaller(caller)`. |
| 15 | `test_setMinReputationScore_onlyOwner` | Standard setUp | Non-owner calls `setMinReputationScore(50)` | Reverts with OZ `OwnableUnauthorizedAccount`. |
| 16 | `test_setReputationScores_onlyOwner` | Standard setUp | Owner calls `setReputationScores(90, 10)` | Succeeds. `completionScore == 90`, `rejectionScore == 10`. |
| 17 | `test_setReputationScores_revertsInvalidScore` | Standard setUp | Owner calls `setReputationScores(101, 10)` | Reverts `InvalidScore(101)`. |

### Pause Tests

| # | Test | Setup | Action | Assertion |
|---|---|---|---|---|
| 18 | `test_pause_blocksHookCalls` | Pauser pauses | ACP calls `hook.beforeAction(...)` | Reverts `EnforcedPause()`. |
| 19 | `test_pause_revertsNotPauser` | Standard setUp | Non-pauser calls `pause()` | Reverts `NotPauser(caller)`. |
| 20 | `test_unpause_restoresHookCalls` | Pauser pauses, then unpauses | ACP calls `hook.beforeAction(1, FUND_SELECTOR, "")` | Does not revert. |

### ERC-165 Tests

| # | Test | Setup | Action | Assertion |
|---|---|---|---|---|
| 21 | `test_supportsInterface_IACPHook` | Standard setUp | `hook.supportsInterface(type(IACPHook).interfaceId)` | Returns true. |
| 22 | `test_supportsInterface_IERC165` | Standard setUp | `hook.supportsInterface(type(IERC165).interfaceId)` | Returns true. |
| 23 | `test_supportsInterface_unknown` | Standard setUp | `hook.supportsInterface(0xdeadbeef)` | Returns false. |

### Integration Tests

| # | Test | Setup | Action | Assertion |
|---|---|---|---|---|
| 24 | `test_fullJobLifecycle_fund_complete` | Standard setUp | ACP calls: `beforeAction(fund)` → `afterAction(fund)` → `afterAction(complete)` | All three succeed. `SandboxProvisionRequested` emitted on fund. `ReputationUpdated` with score=80 emitted on complete. |
| 25 | `test_fullJobLifecycle_fund_reject` | Standard setUp | ACP calls: `beforeAction(fund)` → `afterAction(fund)` → `afterAction(reject)` | `SandboxProvisionRequested` emitted on fund. `ReputationUpdated` with score=20 emitted on reject. |
| 26 | `test_beforeAction_fund_blocksUnregisteredAgent` | Provider has no identity | ACP calls `beforeAction(fund)` | Reverts `ProviderNotRegistered`. Job is not funded. |

---

## Deployment

### Contract Deployment

1. Ensure the following are already deployed: ERC-8183 ACP, ERC-8004 IdentityRegistry, ERC-8004 ReputationRegistry.
2. Deploy `TideJobHook` with constructor arguments:
   - `acpContract_`: deployed ACP address
   - `identityRegistry_`: deployed IdentityRegistry address
   - `reputationRegistry_`: deployed ReputationRegistry address
   - `owner_`: deployer EOA (testnet) or multisig (mainnet)
   - `pauser_`: deployer EOA (testnet) or 2-of-3 multisig (mainnet)
3. Verify the contract on SeiScan.

### Post-Deployment Setup

1. **Register TideJobHook with the ACP:** The ACP must recognize this hook address. Depending on the ERC-8183 implementation, this may require calling a `whitelistHook(address)` function on the ACP, or hooks may be set per-job at creation time. Verify the ACP's hook registration mechanism.
2. **Authorize TideJobHook on ReputationRegistry:** The ReputationRegistry must allow `TideJobHook` to call `submitFeedback()`. This may require calling an access control function on the ReputationRegistry (e.g., `grantRole(FEEDBACK_POSTER_ROLE, hookAddress)`).
3. **Verify `IACP.getJob()` availability:** Call `IACP(acpContract).getJob(0)` from the hook's address to confirm the view function exists and returns the expected struct.

### Post-Deployment Verification

1. Call `hook.acpContract()` — should return the ACP address.
2. Call `hook.identityRegistry()` — should return the IdentityRegistry address.
3. Call `hook.reputationRegistry()` — should return the ReputationRegistry address.
4. Call `hook.minReputationScore()` — should return 0.
5. Call `hook.completionScore()` — should return 80.
6. Call `hook.rejectionScore()` — should return 20.
7. Call `hook.paused()` — should return false.
8. On testnet: create a test job on the ACP with the hook set to this address, fund it, and verify `SandboxProvisionRequested` is emitted.

---

## Deferred (Do Not Build)

| Feature | Rationale |
|---|---|
| **On-chain GitHub PR verification (`beforeAction(submit)`)** | YAGNI. The orchestrator (which created the PR) is trusted to submit valid deliverable hashes. Trustless verification via oracle is a Phase 3+ extension. |
| **Multi-tag reputation feedback** | V1 posts a single tag per outcome. Richer tagging (e.g., `timeliness`, `code-quality`) requires evaluator input not available in Phase 0-2. |
| **Timeliness-weighted scoring** | Score could factor in how quickly the agent completed vs. the expiry. Adds complexity without clear Phase 0-2 need. |
| **Hook upgradeability (UUPS proxy)** | Not upgradeable. New hook versions are deployed separately and set on new jobs. Old jobs retain old hooks. This avoids proxy overhead and simplifies deployment. |
| **Validation Registry attestation on complete** | The HLD mentions writing to ValidationRegistry on job completion. Deferred — ReputationRegistry feedback is sufficient for Phase 0-2. |
| **Custom per-job reputation scores** | All completed jobs get the same `completionScore`. Per-job scoring based on evaluator input requires additional hook parameters not present in v1. |
