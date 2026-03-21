# Component: Agent Review Runtime

**Date:** 2026-03-21
**Status:** Draft

---

## Owner

Platform Engineer

## Phase

Phase 1

## Purpose

A container image that executes a single design review cycle as a Kubernetes Job. Given a proposal ID and design document location, it reads the document from the `sei-tide/proposals` GitHub repo, produces a structured JSON review using Claude Sonnet via the Anthropic API, pushes the review to the proposals repo, signs an EIP-712 attestation via AWS KMS, submits the attestation to the TideCouncil contract on Sei, and signals completion to the Tide Operator.

**Business need:** #1 — Present a design to a council of agents and collect structured feedback.

---

## Dependencies

### External Systems Consumed

| System | Interface | Notes |
|--------|-----------|-------|
| GitHub API (REST v3 + Git) | HTTPS on `api.github.com` / `github.com` | Clone proposals repo, push review files. Authenticated via GitHub App installation token generated in init container. |
| Anthropic API | HTTPS on `api.anthropic.com` | Claude Sonnet for structured review generation. API key mounted via CSI. |
| AWS KMS | HTTPS on `kms.{region}.amazonaws.com` | EIP-712 signature generation. Agent's secp256k1 key is non-exportable. Access via IRSA on the Pod's ServiceAccount. |
| Sei EVM RPC | HTTPS on Sei JSON-RPC endpoint | Submit `submitReview()` transaction to TideCouncil contract. Endpoint URL provided via env var. |

### Internal Tide Components Consumed

| Component | Interface | Reference |
|-----------|-----------|-----------|
| Tide Operator | Env vars, volume mounts, K8s Job labels | The Operator creates the Job with all env vars listed in this spec. See [Interface Specification > Environment Variables](#environment-variables). |
| TideCouncil Contract | `submitReview(proposalId, verdict, feedbackHash, agentTokenId, nonce, signature)` | See `lld-tide-council.md`. ABI consumed from mounted config. |
| K8s Platform Manifests | Namespace `tide-agents`, RBAC, NetworkPolicy, SecretProviderClass | See `lld-k8s-manifests.md`. |

### Explicit Exclusions

- Does NOT depend on the Agent Execution Runtime (Phase 2).
- Does NOT depend on ERC-8183 / TideJobHook (those are Phase 2 job escrow).
- Does NOT depend on any dashboard or CLI tool.
- Does NOT call ERC-8004 Reputation Registry (reputation updates happen via TideJobHook in Phase 2+).

---

## Interface Specification

### Environment Variables

Set by the Tide Operator when creating the K8s Job. The container MUST fail with exit code 10 if any required variable is missing or empty.

| Variable | Required | Type | Example | Description |
|----------|----------|------|---------|-------------|
| `TIDE_PROPOSAL_ID` | Yes | `uint256` as decimal string | `"42"` | On-chain proposal ID from TideCouncil. |
| `TIDE_DESIGN_HASH` | Yes | `bytes32` hex string | `"0xabcdef..."` | keccak256 of the design document. Used to verify document integrity after clone. |
| `TIDE_DESIGN_PATH` | Yes | Repo-relative path | `"proposals/2026-03/design-v2.md"` | Path to the design document within the proposals repo. |
| `TIDE_PROPOSALS_REPO` | Yes | GitHub `owner/repo` | `"sei-tide/proposals"` | Proposals repository. Init container clones this. |
| `TIDE_PROPOSALS_REPO_BRANCH` | No | Git branch name | `"main"` | Branch to clone. Default: `main`. |
| `TIDE_AGENT_NAME` | Yes | Lowercase alphanumeric + hyphen | `"alpha"` | Agent identifier. Used in review file naming and git author. |
| `TIDE_AGENT_TOKEN_ID` | Yes | `uint64` as decimal string | `"1"` | Agent's ERC-8004 identity token ID. Included in the EIP-712 attestation. |
| `TIDE_COUNCIL_CONTRACT` | Yes | Checksummed Ethereum address | `"0x1234...abcd"` | TideCouncil contract address on Sei. |
| `TIDE_COUNCIL_ABI_PATH` | No | Filesystem path | `"/secrets/tide-council-abi.json"` | Path to TideCouncil ABI JSON. Default: `/secrets/tide-council-abi.json`. |
| `TIDE_SEI_RPC_URL` | Yes | HTTPS URL | `"https://evm-rpc.sei-apis.com"` | Sei EVM JSON-RPC endpoint. |
| `TIDE_SEI_CHAIN_ID` | Yes | `uint256` as decimal string | `"1329"` | Sei chain ID. `1329` for mainnet, `713715` for arctic-1 testnet. |
| `TIDE_KMS_KEY_ARN` | Yes | AWS KMS key ARN | `"arn:aws:kms:us-west-2:123456:key/abc-123"` | KMS key for EIP-712 signing. |
| `TIDE_AWS_REGION` | Yes | AWS region | `"us-west-2"` | AWS region for KMS API calls. |
| `TIDE_LLM_MODEL` | No | Anthropic model ID | `"claude-sonnet-4-20250514"` | Model to use for review. Default: `claude-sonnet-4-20250514`. |
| `TIDE_LLM_MAX_INPUT_TOKENS` | No | Positive integer string | `"100000"` | Max input tokens per API call. Default: `100000`. |
| `TIDE_LLM_MAX_OUTPUT_TOKENS` | No | Positive integer string | `"16384"` | Max output tokens per API call. Default: `16384`. |
| `TIDE_LLM_TOKEN_BUDGET` | No | Positive integer string | `"200000"` | Total token budget (input + output) for the entire review. Default: `200000`. Exceeding this budget causes the review to finalize with what it has. |
| `TIDE_LLM_TEMPERATURE` | No | Float string `[0.0, 1.0]` | `"0.3"` | LLM temperature. Default: `0.3`. |
| `TIDE_REVIEW_TIMEOUT_SECONDS` | No | Positive integer string | `"1800"` | Soft timeout for the review workflow (not the K8s Job deadline). Default: `1800` (30 min). |
| `TIDE_GITHUB_APP_ID` | Yes | Integer string | `"123456"` | GitHub App ID for token generation. |
| `TIDE_GITHUB_INSTALLATION_ID` | Yes | Integer string | `"78901234"` | GitHub App installation ID for the `sei-tide` org. |
| `TIDE_RESULT_PATH` | No | Filesystem path | `"/dev/termination-log"` | Path to write the compact `AgentResult` JSON on exit. Default: `/dev/termination-log`. Kubernetes exposes this as the pod termination message (4KB max). |
| `TIDE_RUNTIME_MODE` | No | `review` / `execution` | `"review"` | Runtime mode indicator set by the Operator. Default: `review`. |
| `TIDE_PROVIDER_ADDRESS` | No | Checksummed Ethereum address | `"0xdead..."` | Agent's Sei wallet address, provided by Operator. If set, skips KMS public key derivation at startup. |
| `TIDE_EXPIRES_AT` | No | RFC 3339 timestamp | `"2026-03-28T12:00:00Z"` | On-chain proposal expiry. Useful for deadline-aware timeout logic. |
| `TIDE_LOG_LEVEL` | No | `debug` / `info` / `warn` / `error` | `"info"` | Structured log level. Default: `info`. |

### Volume Mounts

| Mount Path | Volume Type | Access | Size Limit | Contents |
|------------|-------------|--------|------------|----------|
| `/workspace` | `emptyDir` | Read-write | 10Gi | Init container clones proposals repo here. Main container reads design doc, writes review, writes status file. |
| `/secrets` | CSI (`secrets-store.csi.k8s.io`) | Read-only | N/A | Contains: `github-app-key.pem` (GitHub App private key), `anthropic-api-key` (plaintext API key), `tide-council-abi.json` (contract ABI), `agent-system-prompt.txt` (agent persona/instructions). |
| `/tmp` | `emptyDir` | Read-write | 1Gi | Scratch space for git operations, temporary files. |

#### `/secrets` File Layout

Files are mounted by the SecretProviderClass from AWS Secrets Manager:

| File | Secrets Manager Object | Format |
|------|----------------------|--------|
| `/secrets/github-app-key.pem` | `tide/agents/{agent-name}/github-app-key` | PEM-encoded RSA private key |
| `/secrets/anthropic-api-key` | `tide/agents/{agent-name}/anthropic-api-key` | Plaintext API key string (no newline) |
| `/secrets/tide-council-abi.json` | `tide/config/tide-council-abi` | Standard Solidity ABI JSON array |
| `/secrets/agent-system-prompt.txt` | `tide/agents/{agent-name}/system-prompt` | UTF-8 text, the agent's review persona and instructions |

### Exit Codes

The container MUST exit with one of these codes. The Tide Operator uses the exit code for quick status determination. The compact `AgentResult` JSON in the termination message provides structured error details.

The Operator treats exit code 0 as success and all non-zero codes as failure, but the granular codes below enable richer failure categorization in CRD status and alerting.

| Exit Code | Meaning | Operator Action |
|-----------|---------|-----------------|
| 0 | Success — review pushed, attestation submitted on-chain | Parse `AgentResult` from termination message. Transition to next phase. |
| 1 | Unrecoverable internal error (bug, panic) | Log error. Alert platform team. Do not retry automatically. |
| 2 | Soft timeout — review exceeded 80% of `activeDeadlineSeconds` | Transition to `Failed`, reason `AgentSoftTimeout`. Partial work may exist. |
| 10 | Missing or invalid environment variable | Log which variable. Fix Operator Job template. Do not retry. |
| 11 | Secret mount failure — required file missing at `/secrets/*` | Check SecretProviderClass and Secrets Manager. Do not retry. |
| 20 | Git clone failure (network, auth, repo not found) | Retry with backoff (Operator decides). Check GitHub App credentials. |
| 21 | Design document not found at `TIDE_DESIGN_PATH` | Do not retry. Notify principal that the document path is invalid. |
| 22 | Design document hash mismatch — `keccak256(file) != TIDE_DESIGN_HASH` | Do not retry. The document was modified after the proposal was created. |
| 30 | Anthropic API failure after retries exhausted | Retry with backoff. Check API key validity, rate limits, model availability. |
| 31 | Token budget exceeded before review completion | Non-fatal if partial review was produced. Check termination message for details. Operator may retry with higher budget. |
| 40 | Git push failure (review file could not be pushed) | Retry with backoff. Check GitHub App permissions on proposals repo. |
| 50 | KMS signing failure | Retry with backoff. Check IRSA permissions, KMS key status. |
| 51 | Sei RPC failure — transaction submission failed | Retry with backoff. Check RPC endpoint health. |
| 52 | Sei transaction reverted — on-chain error | Do not retry blindly. Read revert reason from termination message. Common causes: nonce already used (replay), proposal expired, agent not in participant list. |

### Completion Signaling Protocol

**Decision: Kubernetes termination messages (primary) + debug status file (secondary).**

The container uses two complementary signaling mechanisms:

1. **Primary — Termination message (`TIDE_RESULT_PATH`):** A compact JSON object written to the path specified by `TIDE_RESULT_PATH` (default `/dev/termination-log`). Kubernetes persists this in `pod.status.containerStatuses[].state.terminated.message` (max 4096 bytes). The Tide Operator reads this from the Pod API after the Job completes. This is the **only** mechanism the Operator relies on.

2. **Secondary — Debug status file (`/workspace/.tide/status.json`):** A rich JSON file written to the emptyDir volume while the pod is running. Useful for live inspection via `kubectl exec` or log collection. The Operator does NOT read this file — emptyDir volumes are cleaned up after pod termination.

The exit code provides a quick pass/fail signal. The termination message provides structured metadata the Operator uses for CRD status updates.

#### Termination Message Schema (Primary — AgentResult)

Written to: `TIDE_RESULT_PATH` (default `/dev/termination-log`). Must fit in 4KB.

```json
{
  "status": "success",
  "feedbackHash": "0x123456...",
  "verdict": "approve",
  "tokensUsed": {
    "input": 45000,
    "output": 8200
  },
  "error": ""
}
```

This matches the Operator's `AgentResult` Go struct:

```go
type AgentResult struct {
    Status          string      `json:"status"`
    DeliverableHash string      `json:"deliverableHash,omitempty"`
    PRUrl           string      `json:"prUrl,omitempty"`
    PRNumber        int         `json:"prNumber,omitempty"`
    CommitSHA       string      `json:"commitSha,omitempty"`
    FeedbackHash    string      `json:"feedbackHash,omitempty"`
    Verdict         string      `json:"verdict,omitempty"`
    TokensUsed      *TokenUsage `json:"tokensUsed,omitempty"`
    Error           string      `json:"error,omitempty"`
}
```

For review jobs, populate `feedbackHash` and `verdict`. For failures, populate `error`.

**Rules:**
- `status` MUST be `"success"` or `"failure"`.
- On success: `feedbackHash` and `verdict` are required.
- On failure: `error` should contain a human-readable message (truncated to fit 4KB total).
- The termination message MUST be written even on failure. If the container cannot write the file (e.g., filesystem error), it exits with code 1 and the Operator relies on exit code alone.

#### Debug Status File (Secondary)

Written to: `/workspace/.tide/status.json` (while pod is running, for live inspection only)

```json
{
  "version": "1",
  "component": "review-runtime",
  "status": "success | failure | partial",
  "exit_code": 0,
  "proposal_id": "42",
  "agent_name": "alpha",
  "agent_token_id": "1",
  "design_hash": "0xabcdef...",
  "review": {
    "file_path": "reviews/2026-03/review-alpha-42-v2.json",
    "feedback_hash": "0x123456...",
    "verdict": "approve | request_changes",
    "pushed": true,
    "commit_sha": "abc123def456..."
  },
  "attestation": {
    "signed": true,
    "tx_hash": "0x789...",
    "nonce_used": 5,
    "block_number": 12345678
  },
  "token_usage": {
    "input_tokens": 45000,
    "output_tokens": 8200,
    "total_tokens": 53200,
    "budget_remaining": 146800
  },
  "timing": {
    "started_at": "2026-03-21T10:00:00Z",
    "review_completed_at": "2026-03-21T10:05:23Z",
    "push_completed_at": "2026-03-21T10:05:28Z",
    "attestation_completed_at": "2026-03-21T10:05:35Z",
    "finished_at": "2026-03-21T10:05:35Z",
    "total_seconds": 335
  },
  "error": {
    "stage": "llm_review | git_push | kms_sign | sei_submit",
    "message": "Human-readable error description",
    "details": "Stack trace or API error body (truncated to 4KB)"
  }
}
```

**Rules:**
- This file is for debugging and live monitoring only. The Operator does NOT read it.
- The `review` and `attestation` objects are null/absent if the corresponding stage was not reached.
- `status: "partial"` means the review was generated but attestation failed.
- Maximum file size: 64KB (truncate `error.details` if needed).
- Updated progressively during execution so `kubectl exec` can inspect mid-run.

### Review Output File Format

The review is written to the proposals repo at:

```
reviews/{year}-{month}/review-{agent-name}-{proposal-id}-v{revision}.json
```

Example: `reviews/2026-03/review-alpha-42-v2.json`

The revision number `v{N}` increments if the agent reviews the same proposal multiple times (successive design iterations with the same proposal ID). The init container checks for existing reviews to determine the next revision number.

The file follows the structured review schema from the parent design doc:

```json
{
  "schema_version": "1",
  "design_hash": "0xabcdef...",
  "proposal_id": 42,
  "agent_id": 1,
  "agent_name": "alpha",
  "verdict": "request_changes",
  "summary": "The proposed caching layer introduces a consistency risk...",
  "sections": [
    {
      "section": "Architecture",
      "assessment": "needs_work",
      "comments": "The write-through cache assumes single-writer...",
      "suggestion": "Consider a lease-based approach per the Chubby paper..."
    },
    {
      "section": "Cost Model",
      "assessment": "acceptable",
      "comments": "Estimates are reasonable given current Bedrock pricing."
    }
  ],
  "blocking_concerns": [
    "Consistency model under concurrent writes is undefined"
  ],
  "non_blocking_suggestions": [
    "Add a capacity planning section",
    "Reference the existing caching in sei-chain/giga"
  ],
  "token_usage": {
    "input_tokens": 45000,
    "output_tokens": 8200
  },
  "model": "claude-sonnet-4-20250514",
  "reviewed_at": "2026-03-21T10:05:23Z"
}
```

**Validation rules:**
- `verdict` MUST be one of: `"approve"`, `"request_changes"`.
- `sections[].assessment` MUST be one of: `"acceptable"`, `"needs_work"`, `"critical"`.
- `blocking_concerns` MUST be a non-empty array if `verdict == "request_changes"`.
- `blocking_concerns` MUST be an empty array if `verdict == "approve"`.
- The `feedback_hash` in the EIP-712 attestation is `keccak256(canonical_json(review_file))` where canonical JSON is sorted-key, no-whitespace serialization.

---

## State Model

### Runtime Lifecycle

```
┌──────────────┐     ┌──────────────┐     ┌───────────────┐
│  Pod Created │────▶│ Init: Setup  │────▶│  Main: Review │
│  by Operator │     │  workspace   │     │   workflow    │
└──────────────┘     └──────┬───────┘     └───────┬───────┘
                            │ fail                 │
                            ▼                      ▼
                     ┌──────────────┐     ┌───────────────┐
                     │ Exit 20-22   │     │ Exit 0 or     │
                     │ (git/hash)   │     │ 1,30-52       │
                     └──────────────┘     └───────────────┘
```

### Filesystem State Transitions

**After init container completes (`/workspace`):**

```
/workspace/
├── proposals/              # git clone of sei-tide/proposals
│   ├── proposals/          # design documents
│   │   └── 2026-03/
│   │       └── design-v2.md
│   └── reviews/            # existing reviews (if any)
│       └── 2026-03/
│           └── review-beta-42-v1.json
└── .tide/
    ├── github-token          # GitHub App installation token (plaintext, 1h TTL)
    └── repo-metadata.json   # {"clone_sha": "abc...", "cloned_at": "..."}
```

**After main container completes (success):**

```
/workspace/
├── proposals/              # now has the review committed and pushed
│   └── reviews/
│       └── 2026-03/
│           ├── review-beta-42-v1.json   # pre-existing
│           └── review-alpha-42-v1.json  # newly created
└── .tide/
    ├── github-token
    ├── repo-metadata.json
    └── status.json          # debug status file (live inspection only)
```

---

## Internal Design

### Init Container: `workspace-setup`

The init container prepares the workspace. It runs as the same non-root user (UID 1000) as the main container.

**Steps (sequential, fail-fast):**

```
1. VALIDATE environment variables
   - Check all required env vars are set and non-empty
   - Parse and validate types (uint256 strings, addresses, URLs)
   - On failure: exit 10

2. VALIDATE secret mounts
   - Verify /secrets/github-app-key.pem exists and is valid PEM
   - Verify /secrets/anthropic-api-key exists and is non-empty
   - Verify /secrets/tide-council-abi.json exists and is valid JSON array
   - Verify /secrets/agent-system-prompt.txt exists and is non-empty
   - On failure: exit 11

3. GENERATE GitHub App installation token
   - Read /secrets/github-app-key.pem
   - Create JWT: iss=TIDE_GITHUB_APP_ID, iat=now-60s, exp=now+600s
   - POST https://api.github.com/app/installations/{TIDE_GITHUB_INSTALLATION_ID}/access_tokens
     Headers: Authorization: Bearer {jwt}, Accept: application/vnd.github+json
   - Extract token from response
   - Write token to /workspace/.tide/github-token (mode 0600)
   - On failure: exit 20

4. CLONE proposals repo
   - mkdir -p /workspace/proposals
   - git clone --depth 1 --branch {TIDE_PROPOSALS_REPO_BRANCH}
       https://x-access-token:{token}@github.com/{TIDE_PROPOSALS_REPO}.git
       /workspace/proposals
   - Write clone metadata to /workspace/.tide/repo-metadata.json
   - On failure: exit 20

5. VERIFY design document exists
   - Check /workspace/proposals/{TIDE_DESIGN_PATH} exists
   - On failure: exit 21

6. VERIFY design document hash
   - Compute keccak256 of the file contents (raw bytes, no normalization)
   - Compare to TIDE_DESIGN_HASH
   - On failure: exit 22

7. DETERMINE review revision number
   - List existing reviews matching pattern: review-{TIDE_AGENT_NAME}-{TIDE_PROPOSAL_ID}-v*.json
   - Next revision = max(existing) + 1, or 1 if none
   - Write revision number to /workspace/.tide/next-revision

8. VALIDATE Sei RPC connectivity
   - eth_chainId call to TIDE_SEI_RPC_URL
   - Verify returned chain ID matches TIDE_SEI_CHAIN_ID
   - On failure: log warning (non-fatal in init; main container will retry)
```

**Pseudocode for GitHub App token generation:**

```python
import jwt, time, requests

def generate_installation_token(app_id, installation_id, private_key_pem):
    now = int(time.time())
    payload = {
        "iat": now - 60,
        "exp": now + (10 * 60),
        "iss": app_id,
    }
    encoded_jwt = jwt.encode(payload, private_key_pem, algorithm="RS256")

    resp = requests.post(
        f"https://api.github.com/app/installations/{installation_id}/access_tokens",
        headers={
            "Authorization": f"Bearer {encoded_jwt}",
            "Accept": "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
        },
    )
    resp.raise_for_status()
    return resp.json()["token"]
```

### Main Container: Review Workflow

**Steps (sequential with retry logic):**

```
1. READ design document
   - Load /workspace/proposals/{TIDE_DESIGN_PATH} into memory
   - Extract title, section headers for LLM context

2. LOAD agent system prompt
   - Read /secrets/agent-system-prompt.txt
   - This contains the agent's persona, review focus areas, and output format instructions

3. GENERATE structured review via Anthropic API
   - See §LLM Integration below
   - Result: parsed JSON conforming to the review schema

4. VALIDATE review output
   - Verify JSON schema compliance (verdict, sections, blocking_concerns rules)
   - If validation fails: retry LLM call (up to 2 additional attempts with feedback)
   - If still invalid after 3 attempts: exit 30

5. WRITE review file
   - Determine path: reviews/{year}-{month}/review-{agent}-{proposal_id}-v{revision}.json
   - Write canonical JSON to /workspace/proposals/{review_path}

6. COMPUTE feedback hash
   - Serialize review to canonical JSON (sorted keys, no whitespace)
   - feedback_hash = keccak256(canonical_json_bytes)

7. GIT COMMIT and PUSH review
   - cd /workspace/proposals
   - git config user.name "tide-agent-{TIDE_AGENT_NAME}[bot]"
   - git config user.email "{TIDE_GITHUB_APP_ID}+tide-agent-{TIDE_AGENT_NAME}[bot]@users.noreply.github.com"
   - git add reviews/
   - git commit -m "review: {TIDE_AGENT_NAME} reviews proposal {TIDE_PROPOSAL_ID}"
   - git push origin {branch}
   - On conflict: git pull --rebase, retry push (up to 3 attempts)
   - On failure after retries: exit 40

8. RETRIEVE review nonce from TideCouncil
   - Call getReviewNonce(TIDE_AGENT_TOKEN_ID) on TideCouncil contract
   - This returns the next expected nonce for this agent

9. CONSTRUCT EIP-712 typed data
   - Domain: {name: "TideCouncil", version: "1", chainId: TIDE_SEI_CHAIN_ID, verifyingContract: TIDE_COUNCIL_CONTRACT}
   - Message: {proposalId: TIDE_PROPOSAL_ID, verdict: verdict_enum, feedbackHash: feedback_hash, agentTokenId: TIDE_AGENT_TOKEN_ID, nonce: nonce}
   - Compute EIP-712 hash (domain separator + struct hash)

10. SIGN via AWS KMS
    - Call kms:Sign with:
        KeyId: TIDE_KMS_KEY_ARN
        Message: eip712_hash (32 bytes)
        MessageType: DIGEST
        SigningAlgorithm: ECDSA_SHA_256
    - Parse DER-encoded signature into (r, s, v)
    - On failure after 3 retries: exit 50

11. SUBMIT review transaction to TideCouncil
    - Encode calldata: submitReview(proposalId, verdict, feedbackHash, agentTokenId, nonce, signature)
      Note: 6 parameters — includes agentTokenId per TideCouncil interface
    - Estimate gas, set gas price from Sei RPC
    - Send raw transaction (signed via KMS — the same key signs both EIP-712 attestation and tx)
    - Wait for receipt (timeout: 60 seconds, poll every 2 seconds)
    - On receipt status=0 (revert): parse revert reason, exit 52
    - On timeout or RPC error after 3 retries: exit 51

12. WRITE termination message and debug status file
    - Write compact AgentResult JSON to TIDE_RESULT_PATH (default /dev/termination-log)
    - Write rich status JSON to /workspace/.tide/status.json (debug only)
    - Exit 0
```

### LLM Integration

**API call structure:**

```python
import anthropic

client = anthropic.Anthropic(api_key=read_file("/secrets/anthropic-api-key"))

system_prompt = read_file("/secrets/agent-system-prompt.txt")
design_content = read_file(f"/workspace/proposals/{design_path}")

REVIEW_SCHEMA = {
    "name": "design_review",
    "description": "Structured review of an engineering design document",
    "input_schema": {
        "type": "object",
        "properties": {
            "verdict": {"type": "string", "enum": ["approve", "request_changes"]},
            "summary": {"type": "string", "maxLength": 2000},
            "sections": {
                "type": "array",
                "items": {
                    "type": "object",
                    "properties": {
                        "section": {"type": "string"},
                        "assessment": {"type": "string", "enum": ["acceptable", "needs_work", "critical"]},
                        "comments": {"type": "string", "maxLength": 2000},
                        "suggestion": {"type": "string", "maxLength": 2000}
                    },
                    "required": ["section", "assessment", "comments"]
                }
            },
            "blocking_concerns": {"type": "array", "items": {"type": "string"}},
            "non_blocking_suggestions": {"type": "array", "items": {"type": "string"}}
        },
        "required": ["verdict", "summary", "sections", "blocking_concerns", "non_blocking_suggestions"]
    }
}

response = client.messages.create(
    model=llm_model,
    max_tokens=int(max_output_tokens),
    temperature=float(temperature),
    system=system_prompt,
    messages=[
        {
            "role": "user",
            "content": f"Review the following engineering design document. "
                       f"Proposal ID: {proposal_id}. Design hash: {design_hash}.\n\n"
                       f"---\n\n{design_content}"
        }
    ],
    tools=[REVIEW_SCHEMA],
    tool_choice={"type": "tool", "name": "design_review"}
)
```

**Retry logic:**

| Failure | Strategy | Max Retries |
|---------|----------|-------------|
| HTTP 429 (rate limit) | Wait for `Retry-After` header value, or 60 seconds if absent | 5 |
| HTTP 500/502/503 (server error) | Exponential backoff: 2s, 4s, 8s | 3 |
| HTTP 529 (overloaded) | Exponential backoff: 10s, 20s, 40s | 3 |
| Network timeout (30s per request) | Exponential backoff: 2s, 4s, 8s | 3 |
| Malformed JSON in tool_use response | Re-prompt with error feedback appended | 2 |
| All retries exhausted | Exit 30 | — |

**Token budget enforcement:**

```
tokens_used = 0

for each API call:
    tokens_used += response.usage.input_tokens + response.usage.output_tokens
    if tokens_used > int(TIDE_LLM_TOKEN_BUDGET):
        if review_is_complete:
            # finalize with current review
            break
        else:
            # write partial review, set status="partial"
            exit 31
```

**Structured output parsing:**

1. Use Anthropic's `tool_use` feature (forced tool choice) to get JSON-schema-validated output.
2. Extract the `input` field from the tool_use content block.
3. Apply post-validation:
   - If `verdict == "request_changes"` and `blocking_concerns` is empty → re-prompt once asking for blocking concerns.
   - If `verdict == "approve"` and `blocking_concerns` is non-empty → re-prompt once to resolve inconsistency.
4. Enrich the review with metadata fields (`schema_version`, `design_hash`, `proposal_id`, `agent_id`, `agent_name`, `token_usage`, `model`, `reviewed_at`).

### EIP-712 Signing via KMS

The agent's Sei wallet key lives in AWS KMS as a non-exportable `ECC_SECG_P256K1` key. Signing flow:

```
1. Construct EIP-712 domain separator:
   domainSeparator = keccak256(abi.encode(
       keccak256("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"),
       keccak256("TideCouncil"),
       keccak256("1"),
       chainId,
       councilAddress
   ))

2. Construct struct hash:
   REVIEW_TYPEHASH = keccak256("Review(uint256 proposalId,uint8 verdict,bytes32 feedbackHash,uint256 agentTokenId,uint256 nonce)")
   structHash = keccak256(abi.encode(
       REVIEW_TYPEHASH,
       proposalId,
       verdict,      # 0 = Approve, 1 = RequestChanges
       feedbackHash,
       agentTokenId,
       nonce
   ))

3. Compute digest:
   digest = keccak256("\x19\x01" || domainSeparator || structHash)

4. Sign digest via KMS:
   POST kms:Sign
     KeyId: TIDE_KMS_KEY_ARN
     Message: digest (32 bytes, raw)
     MessageType: DIGEST
     SigningAlgorithm: ECDSA_SHA_256

5. Parse DER signature → (r, s):
   - KMS returns ASN.1 DER-encoded signature
   - Parse into (r, s) integers
   - Normalize s to low-s form (if s > secp256k1.n/2, s = secp256k1.n - s)

6. Recover v:
   - Try v=27: ecrecover(digest, v, r, s) == agent_address → use v=27
   - Try v=28: ecrecover(digest, v, r, s) == agent_address → use v=28
   - Agent's Ethereum address is derived from the KMS public key

7. Submit (v, r, s) as the signature in the submitReview() transaction
```

### Transaction Submission

The agent needs to submit a transaction to call `TideCouncil.submitReview()`. The transaction itself is signed by the same KMS key:

```
1. Get agent's nonce: eth_getTransactionCount(agent_address, "pending")
2. Encode submitReview calldata:
   submitReview(proposalId, verdict, feedbackHash, agentTokenId, nonce, signature)
   Selector: bytes4(keccak256("submitReview(uint256,uint8,bytes32,uint256,uint256,bytes)"))
3. Estimate gas: eth_estimateGas(submitReview_calldata)
4. Get gas price: eth_gasPrice() (Sei uses a simple gas price model)
5. Build raw transaction:
   {
     nonce: tx_nonce,
     gasPrice: gas_price,
     gasLimit: estimated_gas * 1.2,  // 20% buffer
     to: TIDE_COUNCIL_CONTRACT,
     value: 0,
     data: submitReview_calldata,
     chainId: TIDE_SEI_CHAIN_ID
   }
6. RLP-encode and sign via KMS (same key, same signing flow as EIP-712 but over tx hash)
7. eth_sendRawTransaction(signed_tx)
8. Poll eth_getTransactionReceipt every 2s, timeout at 60s
```

---

## Error Handling

| Stage | Error Condition | Detection | Exit Code | Recovery |
|-------|----------------|-----------|-----------|----------|
| Startup | Required env var missing | Check at process start | 10 | Fix Operator Job template. |
| Startup | Secret file missing/invalid | Check file existence and format | 11 | Check SecretProviderClass, Secrets Manager. |
| Init: Token Gen | GitHub App JWT rejected (expired key, wrong app ID) | HTTP 401 from GitHub | 20 | Verify `TIDE_GITHUB_APP_ID`, check key hasn't been revoked. |
| Init: Clone | Repo not found or access denied | git clone exit code != 0 | 20 | Verify `TIDE_PROPOSALS_REPO`, check App installation. |
| Init: Clone | Network timeout | git clone hangs > 120s | 20 | Retry; check egress NetworkPolicy. |
| Init: Verify | Design document not at expected path | File not found | 21 | Principal must fix `TIDE_DESIGN_PATH` or push the document. |
| Init: Verify | Hash mismatch | keccak256 comparison fails | 22 | Document was modified after proposal creation. Principal must create a new proposal. |
| Review: LLM | API key invalid | HTTP 401 from Anthropic | 30 | Rotate key in Secrets Manager. |
| Review: LLM | Rate limited | HTTP 429 | 30 (after retries) | Operator retries later. Consider raising Anthropic tier. |
| Review: LLM | Model unavailable | HTTP 529 or 503 | 30 (after retries) | Wait and retry. |
| Review: LLM | Token budget exceeded | Running total exceeds `TIDE_LLM_TOKEN_BUDGET` | 31 | Review may be partial. Operator retries with higher budget, or accepts partial. |
| Review: Parse | LLM output doesn't match JSON schema | Schema validation failure after 3 attempts | 30 | Likely a prompt issue. Check system prompt clarity. |
| Push: Git | Push rejected (conflict) | git push exit code != 0 | 40 (after rebase retries) | Another agent pushed concurrently. The rebase retry handles most cases. |
| Push: Git | Auth expired (token > 1h old) | HTTP 401 from GitHub | 40 | Init container token expired. The review process shouldn't take > 1h; if it does, the workflow needs refactoring. |
| Sign: KMS | Access denied | KMS returns AccessDeniedException | 50 | Check IRSA policy on ServiceAccount, verify KMS key ARN. |
| Sign: KMS | Key disabled or pending deletion | KMS returns DisabledException | 50 | Re-enable key or create new one. Critical: agent is inoperable. |
| Submit: Sei | Nonce already used (replay) | Revert with "nonce already used" | 52 | Agent already reviewed this proposal. Do not retry. |
| Submit: Sei | Proposal expired | Revert with "proposal expired" | 52 | TTL elapsed. Principal must create new proposal. |
| Submit: Sei | Agent not in participant list | Revert with "not a participant" | 52 | Agent was not included in the proposal. Do not retry. |
| Submit: Sei | RPC unreachable | Connection error or timeout | 51 | Retry. Check egress to Sei RPC endpoint. |
| Submit: Sei | Insufficient gas (agent wallet empty) | Revert or estimation failure | 51 | Fund agent wallet with SEI for gas. |
| General | Soft timeout exceeded (80% of `activeDeadlineSeconds`) | Timer check between stages | 2 | Write termination message with partial results. Exit 2. |
| General | K8s `activeDeadlineSeconds` reached | SIGTERM from kubelet | 143 (128+15) | Operator interprets 143 as timeout. Termination message may not exist. |
| General | OOMKilled | Container exceeds memory limit | 137 (128+9) | Operator interprets 137 as OOM. Increase memory limit. |

---

## Test Specification

### Unit Tests

| Test | Setup | Action | Expected Result |
|------|-------|--------|-----------------|
| `test_env_validation_success` | All required env vars set with valid values | Run env validation | No error, all values parsed correctly |
| `test_env_validation_missing_required` | Omit `TIDE_PROPOSAL_ID` | Run env validation | Error with exit code 10, message names the missing variable |
| `test_env_validation_invalid_address` | Set `TIDE_COUNCIL_CONTRACT` to "not-an-address" | Run env validation | Error with exit code 10 |
| `test_github_token_generation` | Mock GitHub API, valid PEM key | Generate installation token | Valid token returned, written to `/workspace/.tide/github-token` |
| `test_github_token_expired_key` | Mock GitHub API returns 401 | Generate installation token | Error with exit code 20 |
| `test_design_hash_verification_match` | File with known content, matching hash | Verify hash | Success |
| `test_design_hash_verification_mismatch` | File with content that doesn't match hash | Verify hash | Error with exit code 22 |
| `test_review_schema_validation_approve` | Review JSON with `verdict: "approve"`, empty `blocking_concerns` | Validate review schema | Success |
| `test_review_schema_validation_approve_with_blockers` | Review JSON with `verdict: "approve"`, non-empty `blocking_concerns` | Validate review schema | Validation failure (inconsistent) |
| `test_review_schema_validation_request_changes_no_blockers` | Review JSON with `verdict: "request_changes"`, empty `blocking_concerns` | Validate review schema | Validation failure (missing blockers) |
| `test_canonical_json_deterministic` | Same review object serialized twice | Canonical JSON serialization | Identical byte output |
| `test_feedback_hash_computation` | Known review JSON | Compute keccak256 of canonical JSON | Known expected hash |
| `test_eip712_digest_computation` | Known domain, known message values | Compute EIP-712 digest | Known expected 32-byte digest |
| `test_kms_signature_parsing` | DER-encoded secp256k1 signature from KMS mock | Parse to (r, s, v) | Correct r, s values; s is low-s normalized |
| `test_kms_signature_v_recovery` | Parsed (r, s) and known address | Recover v | Correct v value (27 or 28), ecrecover matches agent address |
| `test_token_budget_tracking` | Budget of 100000, multiple API responses | Track cumulative tokens | Exceeds budget → returns partial flag |
| `test_retry_backoff_429` | Mock returning 429 with Retry-After: 5 | Execute with retry | Waits ~5s, retries, eventual success |
| `test_retry_exhaustion` | Mock returning 500 forever | Execute with retry | 3 retries, then exit 30 |
| `test_termination_message_success` | All stages succeed | Write termination message | Valid `AgentResult` JSON at `TIDE_RESULT_PATH`, status="success", feedbackHash and verdict populated |
| `test_termination_message_failure` | KMS fails | Write termination message | status="failure", error message populated, fits in 4KB |
| `test_debug_status_file_success` | All stages succeed | Write debug status file | Valid JSON at `/workspace/.tide/status.json`, status="success" |
| `test_debug_status_file_partial` | LLM succeeds, push succeeds, KMS fails | Write debug status file | status="partial", review present, attestation absent, error present |
| `test_review_revision_numbering` | Existing review files v1, v2 | Determine next revision | Returns 3 |

### Integration Tests

| Test | Setup | Action | Expected Result |
|------|-------|--------|-----------------|
| `test_e2e_review_happy_path` | Sei testnet with TideCouncil. Proposal created. Anthropic API key valid. KMS key provisioned. | Run full container against testnet | Exit 0. Review pushed to proposals repo. Attestation visible on-chain. Status file complete. |
| `test_e2e_review_request_changes` | Same as above but design has obvious flaws | Run container | Exit 0. Verdict is `request_changes`. `blocking_concerns` non-empty. |
| `test_e2e_expired_proposal` | Proposal with TTL in the past | Run container | Exit 52. Status file shows revert reason "proposal expired". |
| `test_e2e_duplicate_review` | Agent already reviewed this proposal (nonce used) | Run container | Exit 52. Status file shows "nonce already used". |
| `test_init_clone_private_repo` | Proposals repo is private, App has access | Run init container | Successful clone. |
| `test_init_clone_no_access` | Proposals repo is private, App lacks access | Run init container | Exit 20. |
| `test_concurrent_push` | Two agents pushing reviews simultaneously | Run two containers in parallel | Both succeed (one rebases). No lost reviews. |

### E2E Smoke Test

Run as a post-deploy check after the review runtime image is published:

```bash
#!/bin/bash
# Smoke test: submit a test proposal, run agent review, verify on-chain attestation

TESTNET_RPC="https://evm-rpc-testnet.sei-apis.com"
PROPOSAL_ID=$(cast call $COUNCIL_ADDRESS "proposalCount()" --rpc-url $TESTNET_RPC)

# 1. Create a test proposal (requires principal wallet)
cast send $COUNCIL_ADDRESS \
    "propose(bytes32,bytes32,uint256[],uint8,uint48)" \
    $(keccak256 < test-design.md) \
    0x0000000000000000000000000000000000000000000000000000000000000000 \
    "[1]" 1 72 \
    --rpc-url $TESTNET_RPC --private-key $PRINCIPAL_KEY

# 2. Run the review container
kubectl apply -f - <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: smoke-test-review
  namespace: tide-agents
spec:
  # ... (full job spec with test env vars)
EOF

# 3. Wait for completion
kubectl wait --for=condition=complete job/smoke-test-review -n tide-agents --timeout=600s

# 4. Verify exit code
EXIT_CODE=$(kubectl get pod -l job-name=smoke-test-review -n tide-agents \
    -o jsonpath='{.items[0].status.containerStatuses[0].state.terminated.exitCode}')
[ "$EXIT_CODE" -eq 0 ] || { echo "FAIL: exit code $EXIT_CODE"; exit 1; }

# 5. Verify on-chain attestation
REVIEW_COUNT=$(cast call $COUNCIL_ADDRESS \
    "getReviewCount(uint256)" $PROPOSAL_ID --rpc-url $TESTNET_RPC)
[ "$REVIEW_COUNT" -gt 0 ] || { echo "FAIL: no reviews on-chain"; exit 1; }

echo "PASS: review runtime smoke test"
```

---

## Deployment

### Dockerfile

```dockerfile
FROM python:3.12-slim AS base

RUN groupadd -g 1000 tide && useradd -u 1000 -g tide -m tide

RUN pip install --no-cache-dir \
    anthropic==0.49.* \
    PyJWT==2.9.* \
    cryptography==44.* \
    requests==2.32.* \
    web3==7.6.* \
    boto3==1.36.* \
    pydantic==2.10.*

COPY --chown=tide:tide src/ /app/
WORKDIR /app

USER 1000:1000

# Init container entrypoint
FROM base AS init
ENTRYPOINT ["python", "init_workspace.py"]

# Main container entrypoint
FROM base AS main
ENTRYPOINT ["python", "run_review.py"]
```

The image is a multi-stage build producing two targets:
- `tide-review-runtime:init-{tag}` — init container
- `tide-review-runtime:main-{tag}` — main container

Alternatively, a single image with different entrypoints selected via the Job spec `command` field:
- Init: `["python", "/app/init_workspace.py"]`
- Main: `["python", "/app/run_review.py"]`

**Recommended:** Single image with different commands. Simpler to build and version.

### Image Tagging

```
{ecr-repo}/tide-review-runtime:{git-sha-short}
{ecr-repo}/tide-review-runtime:{semver}
{ecr-repo}/tide-review-runtime:latest  # testnet only, never used in mainnet
```

- Testnet deployments use `latest` or `git-sha`.
- Mainnet deployments use semver tags only.
- Tags are immutable — never overwrite a semver tag.

### Build Pipeline

```yaml
# .github/workflows/build-review-runtime.yml
on:
  push:
    paths: ["agent-review-runtime/**"]
    branches: [main]
  pull_request:
    paths: ["agent-review-runtime/**"]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run unit tests
        run: cd agent-review-runtime && python -m pytest tests/ -v
      - name: Lint
        run: cd agent-review-runtime && ruff check src/
      - name: Build image
        run: docker build -t tide-review-runtime:${{ github.sha }} agent-review-runtime/
      - name: Push to ECR
        if: github.ref == 'refs/heads/main'
        run: |
          aws ecr get-login-password | docker login --username AWS --password-stdin $ECR_REPO
          docker tag tide-review-runtime:${{ github.sha }} $ECR_REPO/tide-review-runtime:${{ github.sha }}
          docker push $ECR_REPO/tide-review-runtime:${{ github.sha }}
```

### Testnet vs Mainnet Differences

| Aspect | Testnet (arctic-1) | Mainnet |
|--------|-------------------|---------|
| `TIDE_SEI_CHAIN_ID` | `713715` | `1329` |
| `TIDE_SEI_RPC_URL` | `https://evm-rpc-testnet.sei-apis.com` | `https://evm-rpc.sei-apis.com` |
| `TIDE_COUNCIL_CONTRACT` | Testnet deployment address | Mainnet deployment address |
| `TIDE_LLM_TOKEN_BUDGET` | `50000` (lower for cost control) | `200000` |
| Image tag | `latest` or git SHA | Semver only |
| `TIDE_LOG_LEVEL` | `debug` | `info` |

---

## Deferred (Do Not Build)

| Feature | Rationale |
|---------|-----------|
| Multi-model review (GPT-4, Gemini) | YAGNI — Claude Sonnet is sufficient for Phase 1. Model is configurable via env var for future extension. |
| Review caching / deduplication | YAGNI — 3 agents reviewing ~10 designs/month does not warrant caching. |
| Streaming LLM output | YAGNI — reviews are batch operations, not interactive. |
| Auto-retry by the container itself on exit 51/52 | Two-way door concern: retry logic belongs in the Operator, not the container. The container is fail-fast. |
| Review aggregation / consensus logic | Belongs in the Tide Operator or TideCouncil contract, not in the individual agent runtime. |
| Custom GitHub webhook receiver | YAGNI — the container pushes to GitHub; it doesn't need to react to GitHub events. |
| Dashboard integration / webhook notifications | Phase 3+. No dashboard or notification system in Phase 0-2. |
| Reputation-based prompt augmentation | Phase 3+. The system prompt is static for now. |
