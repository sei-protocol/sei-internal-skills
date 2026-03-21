# Cross-Review: Blockchain Developer → Operator + Runtimes

**Reviewer:** Blockchain Developer
**Date:** 2026-03-21
**Status:** Complete

**Specs under review:**
- `lld-tide-operator.md` (K8s Specialist)
- `lld-agent-review-runtime.md` (Platform Engineer)
- `lld-agent-execution-runtime.md` (Platform Engineer)

**Reference specs (mine):**
- `lld-tide-council.md`
- `lld-tide-job-hook.md`
- `lld-contract-deployment.md`

---

## 1. Event Signatures

### 1.1 ProposalCreated

**Status: MISMATCH — 5 sub-issues**

| Aspect | TideCouncil (authoritative) | Operator (consumed) |
|---|---|---|
| Canonical signature | `ProposalCreated(uint256,address,bytes32,uint256,uint256[],uint8,uint40)` | `ProposalCreated(uint256,bytes32,bytes32,address,uint8,uint48)` |
| Parameter count | 7 | 6 |
| Indexed fields | `proposalId`, `principal`, `designHash` (3 topics) | `proposalId`, `principal` (2 topics) |
| `parentProposalId` type | `uint256` | `bytes32` |
| `participantTokenIds` | Present (`uint256[]`) | **Missing entirely** |
| `expiresAt` type | `uint40` | `uint48` |

**Consequences:**
1. `keccak256` of the two signature strings produce different topic[0] hashes. The operator's `eth_subscribe` filter will never match my events.
2. The operator's `decodeProposalCreated` will fail to ABI-decode the data section because the layout is completely different.
3. The operator's Go struct has no field for `participantTokenIds`. The operator needs the participant list to create one review K8s Job per agent — this data is in the event but the operator doesn't decode it. The operator may be relying on a separate `getParticipants()` RPC call, but the event signature must still be fixed for the indexer to fire.

**Resolution:**
- **Operator updates `pkg/constants/events.go`** to:
  ```go
  TopicProposalCreated = crypto.Keccak256Hash([]byte(
      "ProposalCreated(uint256,address,bytes32,uint256,uint256[],uint8,uint40)"))
  ```
- **Operator updates `ProposalCreatedEvent` struct** to:
  ```go
  type ProposalCreatedEvent struct {
      ProposalID          *big.Int         // indexed, in topics[1]
      Principal           common.Address   // indexed, in topics[2]
      DesignHash          [32]byte         // indexed, in topics[3]
      ParentProposalID    *big.Int         // in data
      ParticipantTokenIDs []*big.Int       // in data (dynamic array)
      Quorum              uint8            // in data
      ExpiresAt           uint64           // in data (uint40 on-chain, fits uint64)
  }
  ```
- **No changes to TideCouncil.** My event is the source of truth.

---

### 1.2 ReviewSubmitted

**Status: MISMATCH — agentTokenId type**

| Aspect | TideCouncil (authoritative) | Operator (consumed) |
|---|---|---|
| Canonical signature | `ReviewSubmitted(uint256,uint256,uint8,bytes32)` | `ReviewSubmitted(uint256,uint64,uint8,bytes32)` |
| `agentTokenId` Solidity type | `uint256` | `uint64` |
| Indexed fields | `proposalId`, `agentTokenId` | `proposalId`, `agentTokenId` |

**Consequences:**
- Different canonical strings → different topic[0] hashes → operator filter misses all ReviewSubmitted events.
- Even if hashes matched, `uint64` vs `uint256` affects ABI decoding of topics[2] (the operator would truncate the upper 192 bits).

**Resolution:**
- **Operator updates** signature to `"ReviewSubmitted(uint256,uint256,uint8,bytes32)"`.
- **Operator updates** `AgentTokenID` field type from `uint64` to `*big.Int`.
- **No changes to TideCouncil.**

---

### 1.3 ProposalApproved

**Status: MISMATCH — indexed field placement**

| Aspect | TideCouncil (authoritative) | Operator (consumed) |
|---|---|---|
| Canonical signature | `ProposalApproved(uint256,bytes32)` | `ProposalApproved(uint256,bytes32)` |
| topic[0] hash | **Same** (canonical strings match) | **Same** |
| `proposalId` | indexed → topics[1] | indexed → topics[1] ✓ |
| `designHash` | **indexed → topics[2]** | **in data section** |

**Consequences:**
- topic[0] will match, so the operator's filter WILL receive ProposalApproved logs.
- However, the operator's `decodeProposalApproved` tries to read `designHash` from the data section. Since both fields are indexed, the data section is **empty**. The decode will either fail or read zero bytes.

**Resolution:**
- **Operator updates** `ProposalApprovedEvent` struct to read `DesignHash` from `topics[2]` instead of data:
  ```go
  type ProposalApprovedEvent struct {
      ProposalID *big.Int // indexed, in topics[1]
      DesignHash [32]byte // indexed, in topics[2] — NOT in data
  }
  ```
- **No changes to TideCouncil.**

---

### 1.4 ProposalRejected

**Status: MISMATCH — Operator does not handle this event**

| Aspect | TideCouncil (authoritative) | Operator (consumed) |
|---|---|---|
| Canonical signature | `ProposalRejected(uint256,bytes32)` | **Not defined** |
| Indexed fields | `proposalId`, `designHash` | — |

**Consequences:**
- When a principal calls `reject()`, the operator will not detect the state change. TideProposal CRDs for rejected proposals will remain stuck in `Active` or `Pending` phase indefinitely.
- The operator does have a `ProposalPhaseRejected` phase constant defined in the CRD, but no indexer path to reach it.

**Resolution:**
- **Operator adds** `TopicProposalRejected` to `pkg/constants/events.go`:
  ```go
  TopicProposalRejected = crypto.Keccak256Hash([]byte(
      "ProposalRejected(uint256,bytes32)"))
  ```
- **Operator adds** a `ProposalRejectedEvent` struct and a `case constants.TopicProposalRejected` branch in the indexer's `processLog` switch.
- Note: both fields are indexed (topics[1] and topics[2]), so the data section is empty.
- **No changes to TideCouncil.**

---

### 1.5 ProposalExpired

**Status: COMPATIBLE**

| Aspect | TideCouncil (authoritative) | Operator (consumed) |
|---|---|---|
| Canonical signature | `ProposalExpired(uint256)` | `ProposalExpired(uint256)` |
| Indexed fields | `proposalId` in topics[1] | `proposalId` in topics[1] |

Exact match. No action needed.

---

### 1.6 SandboxProvisionRequested

**Status: MISMATCH — 4 sub-issues**

| Aspect | TideJobHook (authoritative) | Operator (consumed) |
|---|---|---|
| Canonical signature | `SandboxProvisionRequested(uint256,address,address,uint256,uint256,uint256)` | `SandboxProvisionRequested(uint256,uint64,address,uint256,uint256)` |
| Parameter count | 6 | 5 |
| Indexed fields | `jobId`, `provider`, `client` (3 topics) | `jobId`, `agentTokenId` (2 topics) |
| `agentTokenId` position | 4th param (non-indexed, in data) | 2nd param (indexed, in topics[2]) |
| `client` field | Present (indexed) | **Missing entirely** |

**Consequences:**
- Different canonical strings → different topic[0] hashes → operator never sees the event.
- The operator indexes `agentTokenId` as topics[2] using `uint64`, but my event puts `provider` (address) in topics[2] and `client` (address) in topics[3].

**Resolution:**
- **Operator updates** signature to `"SandboxProvisionRequested(uint256,address,address,uint256,uint256,uint256)"`.
- **Operator updates** `SandboxProvisionRequestedEvent` struct:
  ```go
  type SandboxProvisionRequestedEvent struct {
      JobID        *big.Int       // indexed, in topics[1]
      Provider     common.Address // indexed, in topics[2]
      Client       common.Address // indexed, in topics[3]
      AgentTokenID *big.Int       // in data
      Budget       *big.Int       // in data (raw USDC units)
      Expiry       *big.Int       // in data (unix timestamp)
  }
  ```
- **No changes to TideJobHook.**

---

### 1.7 ReputationUpdated

**Status: COMPATIBLE (not consumed by Operator)**

The Operator does not subscribe to `ReputationUpdated` events. This is correct — reputation changes are informational and don't trigger operator actions. No issue.

---

## 2. EIP-712 Signing

### 2.1 Domain Separator

**Status: COMPATIBLE**

| Aspect | TideCouncil | Review Runtime |
|---|---|---|
| Domain name | `"TideCouncil"` | `"TideCouncil"` |
| Domain version | `"1"` | `"1"` |
| Chain ID source | Deployment-time | `TIDE_SEI_CHAIN_ID` env var |
| Verifying contract | Proxy address | `TIDE_COUNCIL_ADDRESS` env var |
| Type hash string | `EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)` | Same |

Both specs construct identical domain separators. The review runtime correctly derives the domain separator from env vars matching the deployment.

---

### 2.2 REVIEW_TYPEHASH

**Status: COMPATIBLE**

| Aspect | TideCouncil | Review Runtime |
|---|---|---|
| Type string | `Review(uint256 proposalId,uint8 verdict,bytes32 feedbackHash,uint256 agentTokenId,uint256 nonce)` | `Review(uint256 proposalId,uint8 verdict,bytes32 feedbackHash,uint256 agentTokenId,uint256 nonce)` |

Byte-for-byte identical. keccak256 will produce the same hash.

---

### 2.3 Struct Hash Encoding Order

**Status: COMPATIBLE**

| Position | TideCouncil `abi.encode` | Review Runtime `abi.encode` |
|---|---|---|
| 0 | `REVIEW_TYPEHASH` | `REVIEW_TYPEHASH` |
| 1 | `proposalId` | `proposalId` |
| 2 | `uint8(verdict)` | `verdict` (0=Approve, 1=RequestChanges) |
| 3 | `feedbackHash` | `feedbackHash` |
| 4 | `agentTokenId` | `agentTokenId` |
| 5 | `nonce` | `nonce` |

Same order. The verdict enum encoding (0=Approve, 1=RequestChanges) is consistent.

---

### 2.4 Verdict Encoding

**Status: COMPATIBLE**

| Value | TideCouncil | Review Runtime |
|---|---|---|
| 0 | `Verdict.Approve` | Approve |
| 1 | `Verdict.RequestChanges` | RequestChanges |

---

## 3. Function Signatures

### 3.1 Review Submission Function Name

**Status: MISMATCH — function name and parameter list**

| Aspect | TideCouncil (authoritative) | Review Runtime (caller) |
|---|---|---|
| Function name | `submitReview` | `review` |
| Selector | `bytes4(keccak256("submitReview(uint256,uint8,bytes32,uint256,uint256,bytes)"))` | `bytes4(keccak256("review(uint256,uint8,bytes32,uint256,bytes)"))` |
| Parameters | `(proposalId, verdict, feedbackHash, agentTokenId, nonce, signature)` — 6 params | `(proposalId, verdict, feedbackHash, nonce, signature)` — 5 params |

**Consequences:**
- Different function names produce different 4-byte selectors. The transaction will revert with a function-not-found error (or route to the fallback function, which doesn't exist).
- The review runtime is missing `agentTokenId` in the calldata. Even if the name were correct, the ABI encoding wouldn't match.

**Evidence from review runtime spec:**
- Dependencies table (line 40): `review(proposalId, verdict, feedbackHash, nonce, signature)`
- Step 11: `Encode calldata: review(proposalId, verdict, feedbackHash, nonce, signature)`

**Resolution:**
- **Review runtime (Platform Engineer) updates** all references from `review()` to `submitReview()`.
- **Review runtime updates** calldata encoding to include `agentTokenId`:
  ```python
  calldata = encode_function_call(
      "submitReview(uint256,uint8,bytes32,uint256,uint256,bytes)",
      [proposalId, verdict, feedbackHash, agentTokenId, nonce, signature]
  )
  ```
- **No changes to TideCouncil.** The function name `submitReview` is a one-way door (ABI published).

---

### 3.2 Nonce View Function Name

**Status: MISMATCH — function name**

| Aspect | TideCouncil (authoritative) | Review Runtime (caller) |
|---|---|---|
| Function name | `getReviewNonce(uint256 agentTokenId)` | `reviewNonces(uint256)` |
| Selector | `bytes4(keccak256("getReviewNonce(uint256)"))` | `bytes4(keccak256("reviewNonces(uint256)"))` |

**Consequences:**
- The review runtime (step 8) calls `reviewNonces(TIDE_AGENT_TOKEN_ID)`. This function does not exist on TideCouncil. The call will revert.
- TideCouncil uses ERC-7201 namespaced storage, so the `reviewNonces` mapping is inside a struct and does NOT have an auto-generated public getter. Only the explicit `getReviewNonce()` view function is available.

**Evidence from review runtime spec:**
- Step 8: "Call reviewNonces(TIDE_AGENT_TOKEN_ID) on TideCouncil contract"

**Resolution:**
- **Review runtime (Platform Engineer) updates** step 8 to call `getReviewNonce(TIDE_AGENT_TOKEN_ID)`.
- **No changes to TideCouncil.**

---

### 3.3 getReviewCount (Smoke Test)

**Status: MISMATCH — function does not exist**

The review runtime's E2E smoke test calls:
```bash
cast call $COUNCIL_ADDRESS "getReviewCount(uint256)" $PROPOSAL_ID
```

TideCouncil does not expose `getReviewCount()`. The available view function is `getReviews(uint256)` which returns the full `Review[]` array. The caller can check `.length`.

**Resolution:**
- **Review runtime (Platform Engineer) updates** the smoke test to use `getReviews(uint256)` and check array length.

---

### 3.4 propose() Smoke Test Signature

**Status: MISMATCH — parameter types in smoke test**

The review runtime's smoke test calls:
```bash
cast send $COUNCIL_ADDRESS \
    "propose(bytes32,bytes32,uint256[],uint8,uint48)" ...
```

TideCouncil's actual signature:
```solidity
function propose(
    bytes32 designHash,
    uint256 parentProposalId,  // uint256, not bytes32
    uint256[] calldata participantTokenIds,
    uint8 quorum,
    uint40 ttl                 // uint40, not uint48
) external returns (uint256);
```

**Mismatches:** `parentProposalId` is `uint256` not `bytes32`, and `ttl` is `uint40` not `uint48`.

**Resolution:**
- **Review runtime (Platform Engineer) updates** the smoke test cast call:
  ```bash
  cast send $COUNCIL_ADDRESS \
      "propose(bytes32,uint256,uint256[],uint8,uint40)" ...
  ```

---

## 4. ACP Interaction (Execution Runtime ↔ TideJobHook)

### 4.1 submit() Selector

**Status: COMPATIBLE**

| Aspect | TideJobHook | Execution Runtime |
|---|---|---|
| Assumed signature | `submit(uint256,bytes32)` | `submit(uint256 jobId, bytes32 deliverableHash)` |
| `SUBMIT_SELECTOR` | `bytes4(keccak256("submit(uint256,bytes32)"))` | Encodes calldata for `submit(uint256,bytes32)` |

The function selector matches. The execution runtime's calldata encoding is compatible with the selector that TideJobHook checks in `beforeAction` / `afterAction`.

---

### 4.2 deliverableHash Encoding

**Status: COMPATIBLE (with documentation discrepancy in Operator)**

| Aspect | Execution Runtime | TideJobHook | Operator |
|---|---|---|---|
| Computation | `bytes32(commit_sha)` — 20-byte SHA left-padded to 32 bytes | Receives as opaque `bytes32` | Reads from agent status file, passes through |
| Operator comment | — | — | "typically keccak256 of the commit SHA" |

The execution runtime computes `deliverableHash` as the raw commit SHA left-padded to bytes32. TideJobHook does not validate or interpret this value (SUBMIT_SELECTOR is a no-op in both `beforeAction` and `afterAction`). The Operator reads the hash from the agent's status file and submits it verbatim.

The Operator's CRD comment (`"typically keccak256 of the commit SHA"`) is **incorrect** — it's left-padded raw bytes, not a keccak256 hash. This is a documentation-only issue since the Operator doesn't compute the hash itself.

**Resolution:**
- **Operator (K8s Specialist) updates** the CRD comment from "typically keccak256 of the commit SHA" to "commit SHA (20 bytes) left-padded to bytes32".

---

## 5. Summary Matrix

| # | Interface | Status | Who Fixes |
|---|---|---|---|
| 1.1 | `ProposalCreated` event signature | **MISMATCH** | K8s Specialist (Operator) |
| 1.2 | `ReviewSubmitted` event — agentTokenId type | **MISMATCH** | K8s Specialist (Operator) |
| 1.3 | `ProposalApproved` event — indexed fields | **MISMATCH** | K8s Specialist (Operator) |
| 1.4 | `ProposalRejected` event — not handled | **MISMATCH** | K8s Specialist (Operator) |
| 1.5 | `ProposalExpired` event | **COMPATIBLE** | — |
| 1.6 | `SandboxProvisionRequested` event signature | **MISMATCH** | K8s Specialist (Operator) |
| 1.7 | `ReputationUpdated` event | **COMPATIBLE** | — |
| 2.1 | EIP-712 domain separator | **COMPATIBLE** | — |
| 2.2 | REVIEW_TYPEHASH | **COMPATIBLE** | — |
| 2.3 | Struct hash encoding order | **COMPATIBLE** | — |
| 2.4 | Verdict encoding | **COMPATIBLE** | — |
| 3.1 | `submitReview` vs `review` function name + params | **MISMATCH** | Platform Engineer (Review Runtime) |
| 3.2 | `getReviewNonce` vs `reviewNonces` | **MISMATCH** | Platform Engineer (Review Runtime) |
| 3.3 | `getReviewCount` smoke test | **MISMATCH** | Platform Engineer (Review Runtime) |
| 3.4 | `propose()` smoke test signature | **MISMATCH** | Platform Engineer (Review Runtime) |
| 4.1 | `submit()` selector | **COMPATIBLE** | — |
| 4.2 | `deliverableHash` encoding | **COMPATIBLE** | K8s Specialist (comment fix only) |

**Totals:** 8 MISMATCH, 8 COMPATIBLE, 0 UNCLEAR

---

## 6. Corrections to My Specs

### TideCouncil (`lld-tide-council.md`)

**No corrections needed.** All event signatures, function signatures, EIP-712 constructs, and type definitions in TideCouncil are authoritative and internally consistent. The mismatches originate from the consumer specs (Operator and Review Runtime).

### TideJobHook (`lld-tide-job-hook.md`)

**No corrections needed.** The `SandboxProvisionRequested` event signature and `SUBMIT_SELECTOR` are authoritative. The Operator must update its event indexer to match.

### Contract Deployment (`lld-contract-deployment.md`)

**No corrections needed.** The deployment scripts reference the correct contract interfaces.

---

## 7. Root Cause Analysis

The event signature mismatches are systematic — they all point to the Operator spec being written against an earlier draft of the contract interfaces:

1. **`uint48` vs `uint40` for timestamps** — The Operator uses `uint48` for `expiresAt`, suggesting it was based on a draft where we considered `uint48`. The final TideCouncil uses `uint40` (sufficient until year 36,812).
2. **`bytes32` vs `uint256` for `parentProposalId`** — The Operator treats this as a hash (bytes32), but TideCouncil uses a numeric ID (uint256) since proposals are auto-incremented.
3. **`uint64` vs `uint256` for token IDs** — The Operator optimistically uses `uint64` for ERC-8004 token IDs. While practically sufficient, the Solidity events use `uint256` per ERC-721 convention. The canonical signature must match the Solidity source exactly.
4. **Missing `participantTokenIds`** — The Operator's event struct omits the participant array, suggesting it was designed to fetch participants via `getParticipants()` RPC call instead. This is fine operationally, but the event signature must still match for the topic[0] filter to work.
5. **`review()` vs `submitReview()`** — The Review Runtime was written against an earlier function name. The `agentTokenId` parameter was added later to support the meta-transaction pattern (anyone can relay the call), and the runtime wasn't updated.

The Operator spec explicitly acknowledges this risk at line 819: *"These event signatures are placeholders — the blockchain team's deployed ABI is the source of truth."* This cross-review fulfills that contract.
