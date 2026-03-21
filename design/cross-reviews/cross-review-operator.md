# Cross-Review: Tide Operator ↔ Contract Specs ↔ Runtime Specs

**Reviewer:** Kubernetes Specialist (Operator owner)
**Date:** 2026-03-21

---

## 1. Completion Signaling Protocol

**Status: MISMATCH (CRITICAL)**

| Aspect | Operator Spec | Platform Engineer Runtimes |
|--------|--------------|---------------------------|
| Mechanism | `/dev/termination-log` (K8s termination message, 4KB max) | `/workspace/.tide/status.json` (emptyDir, 64KB) |
| Read method | `pod.status.containerStatuses[].state.terminated.message` via Pod API | Direct file read from shared volume |
| Persistence | Survives pod termination (in etcd via Pod status) | **Lost on pod termination** (emptyDir is garbage-collected) |

**Root cause:** After a pod terminates, Kubernetes garbage-collects `emptyDir` volumes. The Operator cannot read `/workspace/.tide/status.json` after the container exits. The termination message approach is the only correct mechanism because it is persisted in the Pod API object.

**Resolution:** Operator's termination message approach is the primary signaling mechanism. The Platform Engineer's `status.json` remains useful for _live debugging_ while the pod is running (e.g., `kubectl exec` or sidecar reads), but the Operator MUST NOT depend on it post-termination.

The runtimes must write critical completion data (status, deliverable hash, error info, token usage, timing) to `/dev/termination-log` as JSON. The rich `status.json` can still be written for debugging but is not authoritative.

**Action items:**
- Operator spec updated: keep termination message as primary, expand `AgentResult` struct with `token_usage` and `timing` fields
- Platform Engineer runtimes: MUST write `AgentResult` JSON to `TIDE_RESULT_PATH` (`/dev/termination-log`) before exiting
- Platform Engineer runtimes: MAY continue writing `status.json` for live debugging, but it is advisory only

---

## 2. Exit Codes

**Status: MISMATCH — Operator adopts Platform Engineer's granular codes**

| Exit Code | Operator (before) | Platform Engineer | Resolution |
|-----------|-------------------|-------------------|------------|
| 0 | Success | Success | COMPATIBLE |
| 1 | Agent failure | Unrecoverable internal error (bug, panic) | COMPATIBLE |
| 2 | Soft timeout | _(not defined)_ | REMOVED — covered by 143 + SIGTERM handler |
| 10 | _(not defined)_ | Missing/invalid env var | ADOPT |
| 11 | _(not defined)_ | Secret mount failure | ADOPT |
| 20 | _(not defined)_ | Git clone failure | ADOPT |
| 21 | _(not defined)_ | Design doc / upstream commit not found | ADOPT |
| 22 | _(not defined)_ | Design hash mismatch / task file not found | ADOPT |
| 30 | _(not defined)_ | LLM API failure | ADOPT |
| 31 | _(not defined)_ | Token budget exceeded | ADOPT |
| 32 | _(not defined)_ | Max iterations exceeded (execution only) | ADOPT |
| 40 | _(not defined)_ | Git push failure | ADOPT |
| 41 | _(not defined)_ | PR creation failure (execution only) | ADOPT |
| 50 | _(not defined)_ | KMS signing failure | ADOPT |
| 51 | _(not defined)_ | Sei RPC failure | ADOPT |
| 52 | _(not defined)_ | Sei transaction reverted | ADOPT |
| 137 | OOMKilled | OOMKilled | COMPATIBLE |
| 143 | SIGTERM / deadline | SIGTERM / deadline | COMPATIBLE |

**Resolution:** Operator adopts the Platform Engineer's full exit code table (0, 1, 10-52, 137, 143). Removes exit code 2 (soft timeout — the SIGTERM handler in the runtimes handles graceful shutdown, and the Operator should treat 143 as the timeout signal). The granular codes let the Operator make smarter retry/fail decisions without parsing the result JSON.

---

## 3. Event Signatures (Operator ↔ TideCouncil)

### 3a. ProposalCreated

**Status: MISMATCH (5 issues)**

| Aspect | Operator Spec | TideCouncil Contract |
|--------|--------------|---------------------|
| Signature | `ProposalCreated(uint256,bytes32,bytes32,address,uint8,uint48)` | `ProposalCreated(uint256,address,bytes32,uint256,uint256[],uint8,uint40)` |
| Param count | 6 | 7 |
| Indexed fields | `proposalId`, `principal` | `proposalId`, `principal`, `designHash` |
| `parentProposalId` type | `bytes32` | `uint256` |
| `expiresAt` type | `uint48` | `uint40` |
| `participantTokenIds` | **MISSING** | `uint256[]` (in data) |
| `designHash` position | 2nd param (non-indexed) | 3rd param (**indexed** — in topics[2]) |

**Impact:** Topic[0] hash mismatch means the Operator will never match `ProposalCreated` events. Even with a correct topic hash, the data decoding would fail due to wrong parameter types, ordering, and missing `participantTokenIds`.

**Resolution:** Operator spec updated to match the contract exactly:
```
ProposalCreated(uint256,address,bytes32,uint256,uint256[],uint8,uint40)
```
- `proposalId` indexed (topics[1])
- `principal` indexed (topics[2])
- `designHash` indexed (topics[3])
- `parentProposalId` as `uint256` (data)
- `participantTokenIds` as `uint256[]` (data)
- `quorum` as `uint8` (data)
- `expiresAt` as `uint40` → decoded into `uint64` in Go (data)

### 3b. ReviewSubmitted

**Status: MISMATCH (1 issue)**

| Aspect | Operator Spec | TideCouncil Contract |
|--------|--------------|---------------------|
| Signature | `ReviewSubmitted(uint256,uint64,uint8,bytes32)` | `ReviewSubmitted(uint256,uint256,uint8,bytes32)` |
| `agentTokenId` type | `uint64` | `uint256` |

**Impact:** Topic[0] hash mismatch (`uint64` ≠ `uint256` in the canonical signature). The Operator will never match `ReviewSubmitted` events.

**Resolution:** Operator spec updated to use `uint256` for `agentTokenId`. The Go struct stores it as `*big.Int`, but we validate it fits in uint64 before populating the CRD.

### 3c. ProposalApproved

**Status: MISMATCH (1 issue — indexing)**

| Aspect | Operator Spec | TideCouncil Contract |
|--------|--------------|---------------------|
| Signature | `ProposalApproved(uint256,bytes32)` | `ProposalApproved(uint256,bytes32)` |
| Topic[0] hash | Matches | Matches |
| `designHash` indexing | Non-indexed (in data) | **Indexed** (in topics[2]) |

**Impact:** Topic[0] hashes match, so the Operator WILL receive these events. However, the Go decoder tries to read `designHash` from the log `data` section, but it's actually in `topics[2]` (indexed fields go to topics). Data section is empty. Decoding fails silently or produces zero bytes.

**Resolution:** Operator spec updated to read `designHash` from `topics[2]` instead of from log data.

### 3d. ProposalRejected

**Status: MISMATCH (missing event)**

The Operator spec does NOT index `ProposalRejected` events. The TideCouncil contract emits:
```
ProposalRejected(uint256 indexed proposalId, bytes32 indexed designHash)
```

**Impact:** If a principal rejects a proposal, the Operator never learns about it. The TideProposal CR stays in `Active` phase indefinitely until the reconciliation safety net catches it via `GetProposalState` (at 5-minute intervals). This is a degraded experience but not data loss.

**Resolution:** Operator spec updated to index `ProposalRejected` events for faster state transitions.

### 3e. ProposalExpired

**Status: COMPATIBLE**

Both specs agree: `ProposalExpired(uint256 indexed proposalId)`.

---

## 4. Event Signatures (Operator ↔ TideJobHook)

### 4a. SandboxProvisionRequested

**Status: MISMATCH (4 issues)**

| Aspect | Operator Spec | TideJobHook Contract |
|--------|--------------|---------------------|
| Signature | `SandboxProvisionRequested(uint256,uint64,address,uint256,uint256)` | `SandboxProvisionRequested(uint256,address,address,uint256,uint256,uint256)` |
| Param count | 5 | 6 |
| Indexed fields | `jobId`, `agentTokenId` | `jobId`, `provider`, `client` |
| `agentTokenId` type | `uint64` (indexed) | `uint256` (non-indexed) |
| `client` field | **MISSING** | `address indexed client` |
| `provider` field | Non-indexed | **Indexed** (topics[2]) |

**Impact:** Topic[0] hash mismatch. Even if hashes matched, field layout is completely different.

**Resolution:** Operator spec updated to match the contract exactly:
```
SandboxProvisionRequested(uint256,address,address,uint256,uint256,uint256)
```
- `jobId` indexed (topics[1])
- `provider` indexed (topics[2])
- `client` indexed (topics[3])
- `agentTokenId` as `uint256` (data)
- `budget` as `uint256` (data)
- `expiry` as `uint256` (data)

### 4b. ReputationUpdated

**Status: NOT APPLICABLE**

The Operator does not need to index `ReputationUpdated`. It is consumed by off-chain analytics, not the Operator control loop. No action needed.

---

## 5. Environment Variable Naming (Operator → Runtimes)

**Status: MISMATCH (multiple issues)**

The Operator creates K8s Jobs and sets env vars. The runtimes consume them. Since the Operator owns Job creation, it is the source of truth for naming.

### 5a. Name Mismatches

| Operator Name | Runtime Name | Resolution |
|--------------|-------------|------------|
| `TIDE_KMS_KEY_ARN` | `TIDE_KMS_KEY_ID` | **Use `TIDE_KMS_KEY_ID`** — both runtimes already use this name, and AWS KMS accepts ARNs as key IDs |
| `TIDE_COUNCIL_CONTRACT` | `TIDE_COUNCIL_ADDRESS` | **Use `TIDE_COUNCIL_ADDRESS`** — consistent with web3 convention (`_ADDRESS` suffix for contract addresses) |
| `TIDE_ACP_CONTRACT` | `TIDE_ACP_ADDRESS` | **Use `TIDE_ACP_ADDRESS`** — same reasoning |
| `TIDE_GITHUB_INSTALLATION_ID` | `TIDE_GITHUB_APP_INSTALLATION_ID` | **Use `TIDE_GITHUB_APP_INSTALLATION_ID`** — runtimes already use this, and it's clearer |

### 5b. Operator-Only Variables (to remove or keep)

| Variable | Status | Resolution |
|----------|--------|------------|
| `TIDE_RESULT_PATH` | Operator-only (runtimes don't reference) | **KEEP** — runtimes must adopt this. Default `/dev/termination-log`. |
| `TIDE_RUNTIME_MODE` | Operator-only | **KEEP** — useful for runtimes to know their mode |
| `TIDE_PROVIDER_ADDRESS` | Operator-only | **KEEP** — runtimes may need for tx signing verification |
| `TIDE_CLIENT_ADDRESS` | Operator-only (execution) | **KEEP** — runtimes may need |
| `TIDE_BUDGET_RAW` | Operator-only (execution) | **KEEP** — useful for cost tracking in status file |

### 5c. Missing Variables (runtimes need, Operator doesn't set)

**Review runtime missing env vars:**

| Variable | Required | Resolution |
|----------|----------|------------|
| `TIDE_DESIGN_PATH` | Yes | Operator MUST set — path to design doc in proposals repo |
| `TIDE_LLM_MODEL` | No | Operator SHOULD set from platform ConfigMap, default `claude-sonnet-4-20250514` |
| `TIDE_LLM_MAX_INPUT_TOKENS` | No | Operator MAY set, runtime uses default `100000` |
| `TIDE_LLM_MAX_OUTPUT_TOKENS` | No | Operator MAY set, runtime uses default `16384` |
| `TIDE_LLM_TOKEN_BUDGET` | No | Operator SHOULD set from platform ConfigMap |
| `TIDE_LLM_TEMPERATURE` | No | Operator MAY set, runtime uses default `0.3` |
| `TIDE_REVIEW_TIMEOUT_SECONDS` | No | Operator MAY set, runtime uses default `1800` |
| `TIDE_LOG_LEVEL` | No | Operator SHOULD set from platform ConfigMap |
| `TIDE_AWS_REGION` | Yes | Operator MUST set from its own `AWS_REGION` |
| `TIDE_SEI_CHAIN_ID` | Yes | Already set (COMPATIBLE) |
| `TIDE_PROPOSALS_REPO_BRANCH` | No | Operator MAY set, runtime defaults to `main` |

**Execution runtime missing env vars:**

| Variable | Required | Resolution |
|----------|----------|------------|
| `TIDE_TASK_DESCRIPTION` | Yes | Operator MUST set — the task description for the coding agent |
| `TIDE_TASK_FILE_PATH` | No | Operator MAY set |
| `TIDE_UPSTREAM_REPO` | Yes | Operator MUST set — the source repo to clone |
| `TIDE_UPSTREAM_BRANCH` | No | Operator MAY set, default `main` |
| `TIDE_UPSTREAM_COMMIT` | No | Operator MAY set to pin to a specific commit |
| `TIDE_WORKSPACE_BRANCH` | No | Operator MAY set, default `job-{TIDE_JOB_ID}` |
| `TIDE_DELIVERABLES_BASE_BRANCH` | No | Operator MAY set, default `main` |
| `TIDE_LLM_MODEL` | No | Operator SHOULD set from platform ConfigMap |
| `TIDE_LLM_TOKEN_BUDGET` | No | Operator SHOULD set from platform ConfigMap |
| `TIDE_LLM_MAX_OUTPUT_TOKENS` | No | Operator MAY set |
| `TIDE_MAX_ITERATIONS` | No | Operator SHOULD set from platform ConfigMap |
| `TIDE_TEST_COMMAND` | No | Operator MAY set |
| `TIDE_EXECUTION_TIMEOUT_SECONDS` | No | Operator MAY set, default `3000` |
| `TIDE_CODING_FRAMEWORK` | No | Operator MAY set, default `openhands` |
| `TIDE_LOG_LEVEL` | No | Operator SHOULD set from platform ConfigMap |
| `TIDE_AWS_REGION` | Yes | Operator MUST set |
| `TIDE_ACP_ABI_PATH` | No | Operator MAY set, default `/secrets/acp-abi.json` |

**Resolution:** Operator spec updated with the complete env var table covering ALL variables both runtimes need. Required vars are always set. Optional vars are set from platform ConfigMap defaults when available.

---

## 6. ServiceAccount Names (Operator → K8s Manifests)

**Status: MISMATCH**

| Aspect | Operator Spec | K8s Manifests |
|--------|--------------|---------------|
| ServiceAccount | `"tide-agent"` (hardcoded) | Per-agent: `tide-agent-alpha`, `tide-agent-beta`, `tide-agent-gamma` |

**Impact:** The `tide-agent` ServiceAccount does not exist in the `tide-agents` namespace. Job creation will fail with a ServiceAccount not found error, or (worse) the pod will use the `default` ServiceAccount which has no IRSA annotations — meaning KMS signing and Secrets Manager access will fail.

**Resolution:** Operator spec updated to use `fmt.Sprintf("tide-agent-%s", agent.Name)` to match the per-agent ServiceAccounts defined in the K8s manifests. This is critical for IRSA to work — each agent's SA has its own IAM role scoped to that agent's KMS key.

---

## 7. K8s Labels (Operator → K8s Manifests)

**Status: COMPATIBLE**

| Aspect | Operator Spec | K8s Manifests |
|--------|--------------|---------------|
| Label key | `app.kubernetes.io/component` | `app.kubernetes.io/component` |
| Label value | `"agent"` (via `constants.ComponentAgent`) | `"agent"` |

The NetworkPolicy `agent-egress-allow` selects on `app.kubernetes.io/component: agent`. The Operator sets this label on all agent Job pods. **No action needed.**

---

## Summary of Changes Required

| # | Interface | Status | Owner to Fix | Severity |
|---|-----------|--------|-------------|----------|
| 1 | Completion signaling | MISMATCH | Operator (keep termination-log) + Runtimes (must write to it) | CRITICAL |
| 2 | Exit codes | MISMATCH | Operator (adopt granular codes) | HIGH |
| 3a | ProposalCreated event | MISMATCH | Operator (fix signature, types, indexed fields) | CRITICAL |
| 3b | ReviewSubmitted event | MISMATCH | Operator (fix agentTokenId type uint256) | CRITICAL |
| 3c | ProposalApproved event | MISMATCH | Operator (fix designHash indexing decode) | HIGH |
| 3d | ProposalRejected event | MISMATCH | Operator (add missing event) | MEDIUM |
| 4a | SandboxProvisionRequested event | MISMATCH | Operator (fix signature completely) | CRITICAL |
| 5a | Env var naming | MISMATCH | Operator (rename 4 vars) | HIGH |
| 5c | Missing env vars | MISMATCH | Operator (add all required + optional vars) | HIGH |
| 6 | ServiceAccount names | MISMATCH | Operator (use per-agent SA names) | CRITICAL |
| 7 | K8s labels | COMPATIBLE | None | — |

**Operator LLD updated in place (`lld-tide-operator.md`) with all corrections.**
