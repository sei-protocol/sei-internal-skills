# Cross-Review: Platform Engineer

**Date:** 2026-03-21
**Reviewer:** Platform Engineer
**Specs reviewed:**
- `lld-tide-operator.md` (K8s Specialist)
- `lld-tide-council.md` (Blockchain Developer)
- `lld-tide-job-hook.md` (Blockchain Developer)
- `lld-agent-review-runtime.md` (own)
- `lld-agent-execution-runtime.md` (own)
- `lld-k8s-manifests.md` (own)

---

## 1. Completion Signaling Protocol

**Status: MISMATCH (critical)**

| Aspect | My Specs (review + execution runtimes) | Operator Spec |
|--------|---------------------------------------|---------------|
| Primary output | `/workspace/.tide/status.json` (emptyDir) | `/dev/termination-log` (K8s termination message) |
| Max size | 64KB | 4KB (K8s limit on termination messages) |
| Schema | Rich nested JSON (review, attestation, timing, token_usage, error) | Compact `AgentResult` struct (~500 bytes) |
| Read mechanism | Operator reads file from emptyDir after pod terminates | Operator reads `pod.status.containerStatuses[].state.terminated.message` via K8s API |

**Why the Operator is correct:** After a pod terminates, emptyDir volumes are garbage-collected. The Operator **cannot** read `/workspace/.tide/status.json` from a terminated pod. Kubernetes termination messages are persisted in the Pod API object and survive pod cleanup. This is the standard K8s pattern.

**Resolution (applied to my specs):**
1. **PRIMARY**: Write compact `AgentResult` JSON to the path in `TIDE_RESULT_PATH` env var (default `/dev/termination-log`). Must fit in 4KB.
2. **SECONDARY**: Continue writing rich `status.json` to `/workspace/.tide/status.json` for live debugging while the pod is running (kubectl exec, log inspection). The Operator does NOT rely on this file.
3. Updated both runtime specs with the Operator's `AgentResult` schema.

---

## 2. Exit Codes

**Status: MISMATCH (negotiable)**

| Exit Code | Operator Expects | My Review Runtime | My Execution Runtime |
|-----------|-----------------|-------------------|---------------------|
| 0 | Success | Success | Success |
| 1 | Agent-level failure (catch-all) | Unrecoverable internal error | Unrecoverable internal error |
| 2 | Soft timeout (80% of activeDeadlineSeconds) | *(not defined)* | *(not defined)* |
| 10 | *(not handled)* | Missing env var | Missing env var |
| 11 | *(not handled)* | Secret mount failure | Secret mount failure |
| 20-22 | *(not handled)* | Git/hash errors | Git/source errors |
| 30-32 | *(not handled)* | LLM failures | LLM failures |
| 40-41 | *(not handled)* | Git push failure | Git push / PR failure |
| 50-52 | *(not handled)* | KMS/Sei failures | KMS/Sei failures |
| 137 | OOMKilled | OOMKilled | OOMKilled |
| 143 | SIGTERM | SIGTERM | SIGTERM |

**Resolution (proposal to Operator team):**

My granular exit codes (10-52) are strictly more informative than the Operator's coarse 0/1/2 scheme. The Operator already treats all non-zero codes as failure — my codes don't break that. I propose:

1. **My runtimes add exit code 2** for soft timeout (currently missing). This aligns with the Operator's expectation.
2. **The Operator should adopt my granular exit codes** for richer failure categorization in `TideJob.status.failureReason`. The Operator can group them: `10-11` = config error (don't retry), `20-22` = git error (maybe retry), `30-32` = LLM error (retry with backoff), `40-41` = push error (retry), `50-52` = chain error (depends).
3. **Regardless of what the Operator does**, my codes already work. `0` = success, everything else = failure. The termination message provides structured error details.

Applied: Added exit code 2 to both runtimes.

---

## 3. Environment Variable Naming

### 3a. `TIDE_KMS_KEY_ARN` vs `TIDE_KMS_KEY_ID`

**Status: MISMATCH**

| Spec | Variable Name |
|------|--------------|
| Operator | `TIDE_KMS_KEY_ARN` |
| Review Runtime | `TIDE_KMS_KEY_ID` |
| Execution Runtime | `TIDE_KMS_KEY_ID` |

**Resolution:** Adopt `TIDE_KMS_KEY_ARN` (Operator owns env var naming). Applied to both runtimes.

### 3b. `TIDE_COUNCIL_CONTRACT` vs `TIDE_COUNCIL_ADDRESS`

**Status: MISMATCH**

| Spec | Variable Name |
|------|--------------|
| Operator | `TIDE_COUNCIL_CONTRACT` |
| Review Runtime | `TIDE_COUNCIL_ADDRESS` |

**Resolution:** Adopt `TIDE_COUNCIL_CONTRACT`. Applied.

### 3c. `TIDE_ACP_CONTRACT` vs `TIDE_ACP_ADDRESS`

**Status: MISMATCH**

| Spec | Variable Name |
|------|--------------|
| Operator | `TIDE_ACP_CONTRACT` |
| Execution Runtime | `TIDE_ACP_ADDRESS` |

**Resolution:** Adopt `TIDE_ACP_CONTRACT`. Applied.

### 3d. `TIDE_RESULT_PATH`

**Status: MISMATCH**

| Spec | Behavior |
|------|----------|
| Operator | Provides `TIDE_RESULT_PATH` (default `/dev/termination-log`) |
| My Runtimes | Hardcode `/workspace/.tide/status.json` |

**Resolution:** Runtimes now read `TIDE_RESULT_PATH` and write `AgentResult` JSON to that path. Applied.

### 3e. `TIDE_RUNTIME_MODE`

**Status: MISMATCH (minor)**

| Spec | Behavior |
|------|----------|
| Operator | Provides `TIDE_RUNTIME_MODE` (`"review"` or `"execution"`) |
| My Runtimes | Don't reference this variable |

**Resolution:** Not critical since each runtime is a separate image, but useful for shared library code and logging. Added as optional env var to both runtimes.

### 3f. `TIDE_GITHUB_INSTALLATION_ID` vs `TIDE_GITHUB_APP_INSTALLATION_ID`

**Status: MISMATCH**

| Spec | Variable Name |
|------|--------------|
| Operator | `TIDE_GITHUB_INSTALLATION_ID` |
| Review Runtime | `TIDE_GITHUB_APP_INSTALLATION_ID` |
| Execution Runtime | `TIDE_GITHUB_APP_INSTALLATION_ID` |

**Resolution:** Adopt `TIDE_GITHUB_INSTALLATION_ID`. Applied to both runtimes.

### 3g. Additional env vars Operator provides but my specs don't consume

| Operator Variable | Consumed By My Runtimes? | Notes |
|-------------------|-------------------------|-------|
| `TIDE_PROVIDER_ADDRESS` | No | Agent's Sei wallet. My runtimes derive this from KMS public key. Could consume directly to skip a KMS call at startup. Added as optional. |
| `TIDE_CLIENT_ADDRESS` | No (execution only) | Principal wallet. Not needed by agent logic. Noted. |
| `TIDE_BUDGET_RAW` | No (execution only) | USDC budget. Not needed by agent logic. Noted. |
| `TIDE_EXPIRES_AT` | No | RFC 3339 expiry. Useful for deadline awareness. Added as optional to both runtimes. |

### 3h. Env vars my runtimes need but the Operator doesn't set

**Status: MISMATCH (critical for Operator)**

The Operator's `buildExecutionJob()` code does NOT include these env vars that my runtimes require:

| Variable | Runtime | Required? | Notes |
|----------|---------|-----------|-------|
| `TIDE_LLM_MODEL` | Both | No (has default) | Operator should set from ConfigMap `LLM_MODEL` |
| `TIDE_LLM_TOKEN_BUDGET` | Both | No (has default) | Operator should set from ConfigMap `LLM_TOKEN_BUDGET_*` |
| `TIDE_LLM_MAX_OUTPUT_TOKENS` | Both | No (has default) | Operator should set or leave default |
| `TIDE_LLM_MAX_INPUT_TOKENS` | Review | No (has default) | Operator should set or leave default |
| `TIDE_LLM_TEMPERATURE` | Review | No (has default) | Operator should set or leave default |
| `TIDE_MAX_ITERATIONS` | Execution | No (has default) | Operator should set from ConfigMap `MAX_ITERATIONS_EXECUTION` |
| `TIDE_TEST_COMMAND` | Execution | No | Task-specific; Operator may not know |
| `TIDE_CODING_FRAMEWORK` | Execution | No (has default) | Operator should set from agent config |
| `TIDE_REVIEW_TIMEOUT_SECONDS` | Review | No (has default) | Operator should set or leave default |
| `TIDE_EXECUTION_TIMEOUT_SECONDS` | Execution | No (has default) | Operator should set or leave default |
| `TIDE_LOG_LEVEL` | Both | No (has default) | Operator should set from ConfigMap `LOG_LEVEL` |
| `TIDE_AWS_REGION` | Both | Yes | Required for KMS calls. Operator MUST add this. |
| `TIDE_DESIGN_PATH` | Review | Yes | Path to design doc in proposals repo. Operator must derive. |
| `TIDE_UPSTREAM_REPO` | Execution | Yes | Upstream source repo. Operator must set from job decomposition. |
| `TIDE_UPSTREAM_BRANCH` | Execution | No | Default `main` |
| `TIDE_UPSTREAM_COMMIT` | Execution | No | Pin to commit if needed |
| `TIDE_TASK_DESCRIPTION` | Execution | Yes | Task spec. Operator must set from job decomposition. |

**Action required by Operator team:** Add the required env vars (especially `TIDE_AWS_REGION`, `TIDE_DESIGN_PATH`, `TIDE_UPSTREAM_REPO`, `TIDE_TASK_DESCRIPTION`) to `buildExecutionJob()` and `buildReviewJob()`. Optional vars should be set from the `tide-platform-config` ConfigMap or agent ConfigMap.

---

## 4. TideCouncil Function Signatures

### 4a. Review Nonce Getter

**Status: MISMATCH**

| Spec | Function Name |
|------|--------------|
| My Review Runtime (step 8) | `reviewNonces(TIDE_AGENT_TOKEN_ID)` |
| TideCouncil Contract | `getReviewNonce(uint256 agentTokenId)` |

**Resolution:** Rename to `getReviewNonce` in my spec. Applied.

### 4b. Submit Review Function

**Status: MISMATCH (parameter count)**

| Spec | Signature |
|------|-----------|
| My Review Runtime (step 11) | `review(proposalId, verdict, feedbackHash, nonce, signature)` — 5 params |
| TideCouncil Contract | `submitReview(proposalId, verdict, feedbackHash, agentTokenId, nonce, signature)` — 6 params |

**Resolution:** Rename to `submitReview` and add `agentTokenId` parameter. Applied.

### 4c. EIP-712 Type Hash

**Status: COMPATIBLE**

Both my runtime and TideCouncil use:
```
Review(uint256 proposalId,uint8 verdict,bytes32 feedbackHash,uint256 agentTokenId,uint256 nonce)
```

---

## 5. Volume Mount Paths

**Status: COMPATIBLE (with one minor discrepancy)**

| Mount Path | Operator | Review Runtime | Execution Runtime |
|------------|----------|---------------|-------------------|
| `/workspace` | emptyDir 10Gi | emptyDir 10Gi | emptyDir 10Gi |
| `/secrets` | CSI (read-only) | CSI (read-only) | CSI (read-only) |
| `/tmp` | emptyDir 1Gi | emptyDir 1Gi | emptyDir **2Gi** |

**Resolution:** The Operator's `buildExecutionJob()` creates `/tmp` with 1Gi sizeLimit, but the execution runtime spec says 2Gi. Since the Operator creates the Job, its 1Gi wins. Updated execution runtime spec to note the Operator controls this (1Gi). If 2Gi is needed, negotiate with Operator.

---

## 6. K8s Labels (NetworkPolicy Selector)

**Status: COMPATIBLE**

| Spec | Label | Value |
|------|-------|-------|
| K8s Manifests NetworkPolicy `agent-egress-allow` | `app.kubernetes.io/component` | `agent` |
| Operator's generated Job template | `constants.LabelComponent` = `app.kubernetes.io/component` | `constants.ComponentAgent` = `agent` |

NetworkPolicy will correctly select Operator-generated pods.

---

## 7. SecretProviderClass → Runtime Secrets File Layout

**Status: MISMATCH**

| File | My SPC Mounts | Operator Expects in `/secrets` |
|------|--------------|-------------------------------|
| `github-app-key.pem` | Yes | Yes |
| `anthropic-api-key` | Yes | **No** (not listed) |
| `agent-system-prompt.txt` | Yes | **No** (expects `config.json` instead) |
| `agent-execution-system-prompt.txt` | Yes | **No** |
| `tide-council-abi.json` | Yes | **No** |
| `acp-abi.json` | Yes | **No** |
| `config.json` | **No** | Yes |

The Operator's `/secrets` layout expects `github-app-key.pem` and `config.json`. My SecretProviderClass mounts 6 individual files.

**Resolution:** Since the Operator uses my SecretProviderClass name (from `agent.SecretProviderClass` in the agent config), the CSI driver mounts what MY SPC defines. The Operator's documentation of `/secrets` contents is incomplete — it should match my SPC file layout. No change needed in my specs. The Operator spec should be updated to reflect the actual mounted files.

---

## 8. ServiceAccount Name

**Status: MISMATCH**

| Spec | ServiceAccountName |
|------|-------------------|
| Operator's `buildExecutionJob()` | `"tide-agent"` (single shared SA) |
| K8s Manifests (my spec) | Per-agent SAs: `tide-agent-alpha`, `tide-agent-beta`, `tide-agent-gamma` |

**Resolution:** My per-agent SA design is correct for KMS blast-radius isolation (each agent's IRSA role scopes to its own KMS key). The Operator should use `fmt.Sprintf("tide-agent-%s", agent.Name)` instead of `"tide-agent"`. This is a bug in the Operator spec — flagged for the Operator team.

---

## 9. Operator Event Signature Mismatches with Contracts

**Status: MISMATCH (Operator team must fix, not my problem)**

These are documented for completeness since they affect the end-to-end flow:

### 9a. ProposalCreated event

| Source | Signature |
|--------|-----------|
| TideCouncil (canonical) | `ProposalCreated(uint256,address,bytes32,uint256,uint256[],uint8,uint40)` |
| Operator topic hash | `ProposalCreated(uint256,bytes32,bytes32,address,uint8,uint48)` |

Mismatch in parameter order, types (`uint40` vs `uint48`), and the presence of `uint256[]` (participants array) and `uint256` (parentProposalId) in the actual contract vs `bytes32` placeholders in the Operator.

### 9b. SandboxProvisionRequested event

| Source | Signature |
|--------|-----------|
| TideJobHook (canonical) | `SandboxProvisionRequested(uint256,address,address,uint256,uint256,uint256)` |
| Operator topic hash | `SandboxProvisionRequested(uint256,uint64,address,uint256,uint256)` |

Parameter count and types differ. Hook indexes `provider` and `client` as `address`; Operator expects `uint64 agentTokenId`.

**Action:** Operator team must update `pkg/constants/events.go` to match deployed contract ABIs. The Operator's spec acknowledges these are "placeholders" pending the blockchain team's final ABI.

---

## 10. Init Container: Token File Path

**Status: MISMATCH (minor)**

| Spec | Token file path |
|------|----------------|
| Operator's init container contract | `/workspace/.tide/github-token` |
| My review runtime | `/workspace/.tide/git-token` |
| My execution runtime | `/workspace/.tide/git-token` |

**Resolution:** Adopt the Operator's naming: `/workspace/.tide/github-token`. Applied to both runtimes.

---

## Summary of Changes Applied

### lld-agent-review-runtime.md
1. Completion signaling: primary to `TIDE_RESULT_PATH` (termination log), secondary to status.json
2. Added `AgentResult` compact JSON schema for termination message
3. Renamed `TIDE_KMS_KEY_ID` → `TIDE_KMS_KEY_ARN`
4. Renamed `TIDE_COUNCIL_ADDRESS` → `TIDE_COUNCIL_CONTRACT`
5. Renamed `TIDE_GITHUB_APP_INSTALLATION_ID` → `TIDE_GITHUB_INSTALLATION_ID`
6. Added `TIDE_RESULT_PATH`, `TIDE_RUNTIME_MODE`, `TIDE_PROVIDER_ADDRESS`, `TIDE_EXPIRES_AT`
7. Added exit code 2 (soft timeout)
8. Fixed `reviewNonces()` → `getReviewNonce()`
9. Fixed `review()` → `submitReview()` with `agentTokenId` parameter
10. Fixed git token path: `git-token` → `github-token`

### lld-agent-execution-runtime.md
1. Completion signaling: primary to `TIDE_RESULT_PATH` (termination log), secondary to status.json
2. Added `AgentResult` compact JSON schema for termination message
3. Renamed `TIDE_KMS_KEY_ID` → `TIDE_KMS_KEY_ARN`
4. Renamed `TIDE_ACP_ADDRESS` → `TIDE_ACP_CONTRACT`
5. Renamed `TIDE_GITHUB_APP_INSTALLATION_ID` → `TIDE_GITHUB_INSTALLATION_ID`
6. Added `TIDE_RESULT_PATH`, `TIDE_RUNTIME_MODE`, `TIDE_PROVIDER_ADDRESS`, `TIDE_EXPIRES_AT`
7. Added exit code 2 (soft timeout)
8. Fixed git token path: `git-token` → `github-token`
9. Noted Operator controls `/tmp` volume size (1Gi, not 2Gi)

### lld-k8s-manifests.md
1. Added note about ServiceAccount mismatch with Operator (per-agent SAs vs shared SA)
2. Added note about Operator needing to use `tide-agent-{name}` for ServiceAccountName

### Items for Operator team to resolve
1. Use per-agent ServiceAccountNames in generated Jobs
2. Add missing env vars to `buildExecutionJob()` / `buildReviewJob()`
3. Update event signature topic hashes to match deployed contracts
4. Update `/secrets` directory layout docs to match SPC file layout
5. Consider adopting granular exit codes (10-52) for richer failure categorization
