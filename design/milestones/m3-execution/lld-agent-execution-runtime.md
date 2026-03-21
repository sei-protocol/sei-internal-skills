# Component: Agent Execution Runtime

**Date:** 2026-03-21
**Status:** Draft

---

## Owner

Platform Engineer

## Phase

Phase 2

## Purpose

A container image that executes a funded coding task as a Kubernetes Job. Given a job specification (task description, upstream source, target repo), it clones the upstream source into the agent's workspace repo, runs a coding agent framework to implement the task, iterates on test failures, pushes the result, opens a pull request to the deliverables repo, and signals completion to the Tide Operator with the deliverable hash (commit SHA).

**Business needs served:**
- #3 — Execute jobs in isolated GitHub sandboxes with scoped credentials.
- #5 — Produce deliverables that can be evaluated for USDC release.

---

## Dependencies

### External Systems Consumed

| System | Interface | Notes |
|--------|-----------|-------|
| GitHub API (REST v3 + Git) | HTTPS on `api.github.com` / `github.com` | Clone upstream into workspace repo. Push commits. Open PR to deliverables repo. Authenticated via GitHub App installation token. |
| Anthropic API | HTTPS on `api.anthropic.com` | LLM backend for the coding agent framework. API key mounted via CSI. |
| AWS KMS | HTTPS on `kms.{region}.amazonaws.com` | Sign the `submit(jobId, deliverableHash)` transaction on Sei. Access via IRSA. |
| Sei EVM RPC | HTTPS on Sei JSON-RPC endpoint | Submit deliverable hash to ERC-8183 AgenticCommerce contract. |

### Internal Tide Components Consumed

| Component | Interface | Reference |
|-----------|-----------|-----------|
| Tide Operator | Env vars, volume mounts, K8s Job labels | The Operator creates the Job with all env vars listed in this spec. |
| ERC-8183 AgenticCommerce Contract | `submit(uint256 jobId, bytes32 deliverableHash)` | See `lld-tide-job-hook.md`. ABI consumed from mounted config. |
| K8s Platform Manifests | Namespace `tide-agents`, RBAC, NetworkPolicy, SecretProviderClass | See `lld-k8s-manifests.md`. |
| Agent Review Runtime | Shares the same completion signaling protocol (exit codes + status file) | See `lld-agent-review-runtime.md` §Completion Signaling Protocol. |

### Explicit Exclusions

- Does NOT depend on TideCouncil contract (that's the review workflow).
- Does NOT depend on TideJobHook directly (the hook is called by the ACP contract, not the agent).
- Does NOT call ERC-8004 Reputation Registry (reputation updates happen via TideJobHook's `afterAction`).
- Does NOT depend on any evaluator logic — it submits deliverables; evaluation is a separate flow.

---

## Interface Specification

### Environment Variables

Set by the Tide Operator when creating the K8s Job. The container MUST fail with exit code 10 if any required variable is missing or empty.

| Variable | Required | Type | Example | Description |
|----------|----------|------|---------|-------------|
| `TIDE_JOB_ID` | Yes | `uint256` as decimal string | `"7"` | On-chain ERC-8183 job ID. |
| `TIDE_TASK_DESCRIPTION` | Yes | UTF-8 string (≤ 32KB) | `"Implement the caching layer per..."` | Human-readable task description. Passed to the coding agent as the primary instruction. May reference the approved design hash. |
| `TIDE_TASK_FILE_PATH` | No | Repo-relative path | `"tasks/job-7-spec.md"` | Optional path to a detailed task specification file in the proposals repo. If set, this file is loaded and appended to `TIDE_TASK_DESCRIPTION` as additional context for the coding agent. |
| `TIDE_UPSTREAM_REPO` | Yes | GitHub `owner/repo` | `"sei-protocol/sei-chain"` | Upstream source repo to clone. |
| `TIDE_UPSTREAM_BRANCH` | No | Git branch name | `"main"` | Branch to clone from upstream. Default: `main`. |
| `TIDE_UPSTREAM_COMMIT` | No | Git commit SHA (40 hex chars) | `"abc123..."` | Pin clone to a specific commit. If set, overrides branch HEAD. |
| `TIDE_WORKSPACE_REPO` | Yes | GitHub `owner/repo` | `"sei-tide/agent-alpha"` | Agent's workspace repo. Code is pushed here. |
| `TIDE_WORKSPACE_BRANCH` | No | Git branch name | `"job-7-caching-layer"` | Branch name in workspace repo for this task. Default: `job-{TIDE_JOB_ID}`. |
| `TIDE_DELIVERABLES_REPO` | Yes | GitHub `owner/repo` | `"sei-tide/deliverables"` | Target repo for the pull request. |
| `TIDE_DELIVERABLES_BASE_BRANCH` | No | Git branch name | `"main"` | Base branch for the PR. Default: `main`. |
| `TIDE_PROPOSALS_REPO` | No | GitHub `owner/repo` | `"sei-tide/proposals"` | Proposals repo for reading task spec files. Only needed if `TIDE_TASK_FILE_PATH` is set. |
| `TIDE_AGENT_NAME` | Yes | Lowercase alphanumeric + hyphen | `"alpha"` | Agent identifier. Used in branch names, PR author, commit messages. |
| `TIDE_AGENT_TOKEN_ID` | Yes | `uint64` as decimal string | `"1"` | Agent's ERC-8004 identity token ID. |
| `TIDE_ACP_CONTRACT` | Yes | Checksummed Ethereum address | `"0x5678...efgh"` | ERC-8183 AgenticCommerce contract address on Sei. |
| `TIDE_ACP_ABI_PATH` | No | Filesystem path | `"/secrets/acp-abi.json"` | Path to ACP contract ABI JSON. Default: `/secrets/acp-abi.json`. |
| `TIDE_SEI_RPC_URL` | Yes | HTTPS URL | `"https://evm-rpc.sei-apis.com"` | Sei EVM JSON-RPC endpoint. |
| `TIDE_SEI_CHAIN_ID` | Yes | `uint256` as decimal string | `"1329"` | Sei chain ID. |
| `TIDE_KMS_KEY_ARN` | Yes | AWS KMS key ARN | `"arn:aws:kms:us-west-2:123456:key/abc-123"` | KMS key for transaction signing. |
| `TIDE_AWS_REGION` | Yes | AWS region | `"us-west-2"` | AWS region for KMS API calls. |
| `TIDE_GITHUB_APP_ID` | Yes | Integer string | `"123456"` | GitHub App ID for token generation. |
| `TIDE_GITHUB_INSTALLATION_ID` | Yes | Integer string | `"78901234"` | GitHub App installation ID for the `sei-tide` org. |
| `TIDE_RESULT_PATH` | No | Filesystem path | `"/dev/termination-log"` | Path to write the compact `AgentResult` JSON on exit. Default: `/dev/termination-log`. Kubernetes exposes this as the pod termination message (4KB max). |
| `TIDE_RUNTIME_MODE` | No | `review` / `execution` | `"execution"` | Runtime mode indicator set by the Operator. Default: `execution`. |
| `TIDE_PROVIDER_ADDRESS` | No | Checksummed Ethereum address | `"0xdead..."` | Agent's Sei wallet address, provided by Operator. If set, skips KMS public key derivation at startup. |
| `TIDE_EXPIRES_AT` | No | RFC 3339 timestamp | `"2026-03-28T12:00:00Z"` | On-chain job expiry. Useful for deadline-aware timeout logic. |
| `TIDE_LLM_MODEL` | No | Anthropic model ID | `"claude-sonnet-4-20250514"` | Model for coding agent. Default: `claude-sonnet-4-20250514`. |
| `TIDE_LLM_TOKEN_BUDGET` | No | Positive integer string | `"2000000"` | Total token budget for the entire execution. Default: `2000000`. |
| `TIDE_LLM_MAX_OUTPUT_TOKENS` | No | Positive integer string | `"16384"` | Max output tokens per LLM call. Default: `16384`. |
| `TIDE_MAX_ITERATIONS` | No | Positive integer string | `"25"` | Maximum edit-test-fix iterations the coding agent can perform. Default: `25`. |
| `TIDE_TEST_COMMAND` | No | Shell command string | `"make test"` | Command to run tests. If unset, the coding agent infers from the project structure. |
| `TIDE_EXECUTION_TIMEOUT_SECONDS` | No | Positive integer string | `"3000"` | Soft timeout for the coding workflow. Default: `3000` (50 min). |
| `TIDE_CODING_FRAMEWORK` | No | `openhands` / `swe-agent` / `custom` | `"openhands"` | Coding agent framework to use. Default: `openhands`. |
| `TIDE_LOG_LEVEL` | No | `debug` / `info` / `warn` / `error` | `"info"` | Structured log level. Default: `info`. |

### Volume Mounts

| Mount Path | Volume Type | Access | Size Limit | Contents |
|------------|-------------|--------|------------|----------|
| `/workspace` | `emptyDir` | Read-write | 10Gi | Init container clones upstream source here. Main container writes code, runs tests, pushes results. |
| `/secrets` | CSI (`secrets-store.csi.k8s.io`) | Read-only | N/A | Contains: `github-app-key.pem`, `anthropic-api-key`, `acp-abi.json`, `agent-execution-system-prompt.txt`. |
| `/tmp` | `emptyDir` | Read-write | 1Gi (Operator-controlled) | Scratch space for build artifacts, test output, framework temp files. Note: the Operator's generated Job spec controls the sizeLimit (currently 1Gi). If more is needed, negotiate with Operator team. |

#### `/secrets` File Layout

| File | Secrets Manager Object | Format |
|------|----------------------|--------|
| `/secrets/github-app-key.pem` | `tide/agents/{agent-name}/github-app-key` | PEM-encoded RSA private key |
| `/secrets/anthropic-api-key` | `tide/agents/{agent-name}/anthropic-api-key` | Plaintext API key string (no newline) |
| `/secrets/acp-abi.json` | `tide/config/acp-abi` | Standard Solidity ABI JSON array |
| `/secrets/agent-execution-system-prompt.txt` | `tide/agents/{agent-name}/execution-system-prompt` | UTF-8 text, the agent's coding persona and guidelines |

### Exit Codes

The container MUST exit with one of these codes. The Tide Operator uses the exit code for quick status determination. The compact `AgentResult` JSON in the termination message provides structured error details.

The Operator treats exit code 0 as success and all non-zero codes as failure, but the granular codes below enable richer failure categorization in CRD status and alerting.

| Exit Code | Meaning | Operator Action |
|-----------|---------|-----------------|
| 0 | Success — code pushed, PR opened, deliverable submitted on-chain | Parse `AgentResult` from termination message. Transition to `Submitting`. |
| 1 | Unrecoverable internal error (bug, panic) | Log error. Alert platform team. Do not retry. |
| 2 | Soft timeout — execution exceeded 80% of `activeDeadlineSeconds` | Transition to `Failed`, reason `AgentSoftTimeout`. Partial work may exist. |
| 10 | Missing or invalid environment variable | Fix Operator Job template. Do not retry. |
| 11 | Secret mount failure | Check SecretProviderClass and Secrets Manager. Do not retry. |
| 20 | Git clone failure (network, auth, repo not found) | Retry with backoff. Check GitHub App credentials. |
| 21 | Upstream commit not found (`TIDE_UPSTREAM_COMMIT` doesn't exist) | Do not retry. Notify principal. |
| 22 | Task spec file not found (`TIDE_TASK_FILE_PATH` doesn't exist in proposals repo) | Do not retry. Fix task spec path. |
| 30 | LLM API failure after retries exhausted | Retry with backoff. Check API key, rate limits. |
| 31 | Token budget exceeded | Check termination message for progress. Operator may retry with higher budget. |
| 32 | Max iterations exceeded — tests still failing | Non-fatal if code was produced. Check termination message for test results. Operator decides whether to submit partial work or retry. |
| 40 | Git push failure | Retry with backoff. Check GitHub App permissions. |
| 41 | PR creation failure | Retry with backoff. Check permissions on deliverables repo. |
| 50 | KMS signing failure | Retry with backoff. Check IRSA, KMS key. |
| 51 | Sei RPC failure | Retry with backoff. Check RPC endpoint. |
| 52 | Sei transaction reverted | Read revert reason from termination message. Common: job already submitted, job expired, not the provider. |

### Completion Signaling Protocol

Same dual-mechanism protocol as the Agent Review Runtime: Kubernetes termination messages (primary) + debug status file (secondary).

1. **Primary — Termination message (`TIDE_RESULT_PATH`):** A compact JSON object written to the path specified by `TIDE_RESULT_PATH` (default `/dev/termination-log`). Kubernetes persists this in `pod.status.containerStatuses[].state.terminated.message` (max 4096 bytes). The Tide Operator reads this from the Pod API after the Job completes.

2. **Secondary — Debug status file (`/workspace/.tide/status.json`):** Rich JSON for live inspection. The Operator does NOT read this file.

#### Termination Message Schema (Primary — AgentResult)

Written to: `TIDE_RESULT_PATH` (default `/dev/termination-log`). Must fit in 4KB.

```json
{
  "status": "success",
  "deliverableHash": "0xdef456...",
  "prUrl": "https://github.com/sei-tide/deliverables/pull/42",
  "prNumber": 42,
  "commitSha": "def456...",
  "tokensUsed": {
    "input": 1200000,
    "output": 350000
  },
  "error": ""
}
```

This matches the Operator's `AgentResult` Go struct (see review runtime spec for full type definition).

For execution jobs, populate `deliverableHash`, `prUrl`, `prNumber`, `commitSha`. For failures, populate `error`.

**Rules:**
- `status` MUST be `"success"` or `"failure"`.
- On success: `deliverableHash` and `commitSha` are required. `prUrl` and `prNumber` are required if a PR was opened.
- On failure: `error` should contain a human-readable message (truncated to fit 4KB total).
- The termination message MUST be written even on failure.

#### Debug Status File (Secondary)

Written to: `/workspace/.tide/status.json` (while pod is running, for live inspection only)

```json
{
  "version": "1",
  "component": "execution-runtime",
  "status": "success | failure | partial",
  "exit_code": 0,
  "job_id": "7",
  "agent_name": "alpha",
  "agent_token_id": "1",
  "task": {
    "description_preview": "First 200 chars of task description...",
    "upstream_repo": "sei-protocol/sei-chain",
    "upstream_commit": "abc123..."
  },
  "execution": {
    "framework": "openhands",
    "iterations_completed": 12,
    "iterations_max": 25,
    "tests_passing": true,
    "test_summary": "42 passed, 0 failed, 3 skipped",
    "files_changed": ["pkg/cache/cache.go", "pkg/cache/cache_test.go", "..."],
    "lines_added": 347,
    "lines_removed": 12
  },
  "deliverable": {
    "workspace_repo": "sei-tide/agent-alpha",
    "branch": "job-7-caching-layer",
    "commit_sha": "def456...",
    "pr_url": "https://github.com/sei-tide/deliverables/pull/42",
    "pr_number": 42,
    "deliverable_hash": "0xdef456...",
    "submitted_on_chain": true,
    "tx_hash": "0x789...",
    "block_number": 12345678
  },
  "token_usage": {
    "input_tokens": 1200000,
    "output_tokens": 350000,
    "total_tokens": 1550000,
    "budget_remaining": 450000
  },
  "timing": {
    "started_at": "2026-03-21T10:00:00Z",
    "clone_completed_at": "2026-03-21T10:01:15Z",
    "coding_completed_at": "2026-03-21T10:35:42Z",
    "push_completed_at": "2026-03-21T10:35:50Z",
    "pr_created_at": "2026-03-21T10:35:55Z",
    "submission_completed_at": "2026-03-21T10:36:10Z",
    "finished_at": "2026-03-21T10:36:10Z",
    "total_seconds": 2170
  },
  "error": {
    "stage": "clone | coding | test | push | pr | kms_sign | sei_submit",
    "message": "Human-readable error description",
    "details": "Truncated to 4KB"
  }
}

**Rules:**
- This file is for debugging and live monitoring only. The Operator does NOT read it.
- Updated progressively during execution so `kubectl exec` can inspect mid-run.
```

### Pull Request Format

The PR opened to the deliverables repo follows a structured template:

**Title:** `[tide-job-{TIDE_JOB_ID}] {first line of TIDE_TASK_DESCRIPTION}`

**Body:**

```markdown
## Tide Job #{job_id}

**Agent:** {agent_name} (ERC-8004 #{agent_token_id})
**Upstream:** {upstream_repo}@{upstream_commit_short}
**Workspace:** {workspace_repo}/{branch}

## Task Description

{TIDE_TASK_DESCRIPTION}

## Changes

{auto-generated summary of files changed}

## Test Results

```
{test output summary — last 100 lines}
```

## Metadata

- **Iterations:** {N}/{max}
- **Tokens used:** {total} / {budget}
- **Deliverable hash:** `{commit_sha}`
- **On-chain submission:** `{tx_hash}`

---
*This PR was created by Tide Agent `{agent_name}`. Review the changes and use the evaluator flow to complete or reject the job.*
```

**Labels:** `tide-job`, `agent-{TIDE_AGENT_NAME}`, `job-{TIDE_JOB_ID}`

**Head branch:** `{TIDE_AGENT_NAME}/job-{TIDE_JOB_ID}` (pushed to deliverables repo as a branch from the workspace repo)

**Base branch:** `{TIDE_DELIVERABLES_BASE_BRANCH}` (default: `main`)

---

## State Model

### Runtime Lifecycle

```
┌──────────────┐     ┌──────────────┐     ┌────────────────┐
│  Pod Created │────▶│ Init: Setup  │────▶│  Main: Coding  │
│  by Operator │     │  workspace   │     │   workflow     │
└──────────────┘     └──────┬───────┘     └───────┬────────┘
                            │ fail                 │
                            ▼                      ▼
                     ┌──────────────┐     ┌────────────────┐
                     │ Exit 20-22   │     │ Exit 0 or      │
                     │ (git/source) │     │ 1,30-52        │
                     └──────────────┘     └────────────────┘
```

### Filesystem State Transitions

**After init container completes:**

```
/workspace/
├── source/                 # git clone of upstream repo
│   ├── .git/
│   ├── pkg/
│   └── ...
└── .tide/
    ├── github-token          # GitHub App installation token
    ├── repo-metadata.json   # {"upstream_sha": "...", "cloned_at": "..."}
    └── task-spec.md         # merged task description (env var + optional file)
```

**After main container completes (success):**

```
/workspace/
├── source/                 # modified source with agent's changes
│   ├── .git/               # commits from agent
│   ├── pkg/
│   │   └── cache/
│   │       ├── cache.go     # new/modified files
│   │       └── cache_test.go
│   └── ...
└── .tide/
    ├── github-token
    ├── repo-metadata.json
    ├── task-spec.md
    ├── test-output.log      # last test run output (truncated to 1MB)
    ├── iteration-log.jsonl  # one JSON line per edit-test cycle
    └── status.json          # debug status file (live inspection only)
```

### Iteration Log Format

Each line in `/workspace/.tide/iteration-log.jsonl`:

```json
{"iteration": 1, "action": "edit", "files": ["pkg/cache/cache.go"], "tokens": {"input": 50000, "output": 3000}, "timestamp": "..."}
{"iteration": 1, "action": "test", "command": "go test ./pkg/cache/...", "exit_code": 1, "failing_tests": ["TestCacheEviction"], "timestamp": "..."}
{"iteration": 2, "action": "edit", "files": ["pkg/cache/cache.go"], "tokens": {"input": 55000, "output": 2500}, "timestamp": "..."}
{"iteration": 2, "action": "test", "command": "go test ./pkg/cache/...", "exit_code": 0, "failing_tests": [], "timestamp": "..."}
```

---

## Internal Design

### Design Decision: Coding Agent Framework

**Decision: OpenHands as the default framework.**

Rationale:
- OpenHands provides a well-containerized runtime with built-in sandboxing.
- Supports multiple LLM backends including Anthropic Claude.
- Has a clear programmatic API for headless (non-interactive) execution.
- Handles file editing, terminal commands, and git operations natively.
- SWE-bench validated with strong results.
- The framework is swappable via `TIDE_CODING_FRAMEWORK` env var — this is a two-way door.

Alternative frameworks (configured via `TIDE_CODING_FRAMEWORK`):
- `swe-agent`: Lighter weight, simpler to embed, but less mature multi-step planning.
- `custom`: Bypass framework entirely; the main container runs a custom script at `/workspace/.tide/custom-agent.sh`. For simple tasks or specialized workflows.

### Init Container: `workspace-setup`

**Steps (sequential, fail-fast):**

```
1. VALIDATE environment variables
   - Check all required env vars are set and non-empty
   - Parse and validate types
   - On failure: exit 10

2. VALIDATE secret mounts
   - Verify all required files exist at /secrets/*
   - On failure: exit 11

3. GENERATE GitHub App installation token
   - Same mechanism as review runtime (JWT → installation access token)
   - Write to /workspace/.tide/github-token (mode 0600)
   - On failure: exit 20

4. CLONE upstream source
   - mkdir -p /workspace/source
   - git clone --depth 50 --branch {TIDE_UPSTREAM_BRANCH}
       https://x-access-token:{token}@github.com/{TIDE_UPSTREAM_REPO}.git
       /workspace/source
   - Depth 50 (not 1) to allow meaningful git log context for the coding agent
   - On failure: exit 20

5. PIN to specific commit (if TIDE_UPSTREAM_COMMIT is set)
   - cd /workspace/source && git checkout {TIDE_UPSTREAM_COMMIT}
   - On failure (commit not in shallow clone): git fetch --depth 100 origin {commit} && retry
   - On failure after retry: exit 21

6. CONFIGURE workspace repo as push target
   - git remote add workspace https://x-access-token:{token}@github.com/{TIDE_WORKSPACE_REPO}.git
   - git remote add deliverables https://x-access-token:{token}@github.com/{TIDE_DELIVERABLES_REPO}.git
   - git config user.name "tide-agent-{TIDE_AGENT_NAME}[bot]"
   - git config user.email "{TIDE_GITHUB_APP_ID}+tide-agent-{TIDE_AGENT_NAME}[bot]@users.noreply.github.com"

7. CREATE task branch
   - git checkout -b {TIDE_WORKSPACE_BRANCH}  (default: "job-{TIDE_JOB_ID}")

8. LOAD task specification
   - Start with TIDE_TASK_DESCRIPTION
   - If TIDE_TASK_FILE_PATH is set:
       - Clone proposals repo (shallow, sparse checkout of just the task file)
       - Read the file, append to task description
       - On failure: exit 22
   - Write merged spec to /workspace/.tide/task-spec.md

9. VALIDATE Sei RPC connectivity
   - eth_chainId call, verify chain ID
   - On failure: log warning (non-fatal)

10. WRITE repo metadata
    - /workspace/.tide/repo-metadata.json: upstream SHA, clone time, branch name
```

### Main Container: Coding Workflow

**Step-by-step with pseudocode for non-trivial logic:**

```
1. LOAD configuration
   - Read task spec from /workspace/.tide/task-spec.md
   - Read system prompt from /secrets/agent-execution-system-prompt.txt
   - Read API key from /secrets/anthropic-api-key
   - Initialize token budget tracker

2. INITIALIZE coding agent framework
   Based on TIDE_CODING_FRAMEWORK:

   if framework == "openhands":
       configure_openhands()
   elif framework == "swe-agent":
       configure_swe_agent()
   elif framework == "custom":
       verify /workspace/.tide/custom-agent.sh exists

3. EXECUTE coding task
   - See §OpenHands Integration below
   - The framework produces code changes in /workspace/source/

4. RUN final test suite
   - If TIDE_TEST_COMMAND is set: run it
   - Else: let the framework's final test pass be authoritative
   - Capture output to /workspace/.tide/test-output.log (truncate to 1MB)
   - Record pass/fail in iteration log

5. COMMIT changes
   - cd /workspace/source
   - git add -A
   - git commit -m "tide-job-{TIDE_JOB_ID}: {first_line_of_task}"
   - Record commit SHA

6. PUSH to workspace repo
   - git push workspace {TIDE_WORKSPACE_BRANCH}
   - On conflict: force push (agent owns this branch)
   - On failure after 3 retries: exit 40

7. PUSH branch to deliverables repo for PR
   - git push deliverables {TIDE_WORKSPACE_BRANCH}:{TIDE_AGENT_NAME}/job-{TIDE_JOB_ID}
   - On failure after 3 retries: exit 40

8. CREATE pull request
   - POST https://api.github.com/repos/{TIDE_DELIVERABLES_REPO}/pulls
   - Title, body, labels as specified in §Pull Request Format
   - On failure after 3 retries: exit 41

9. COMPUTE deliverable hash
   - deliverable_hash = bytes32(commit_sha) — left-padded with zeros
   - The commit SHA is the canonical identifier of the deliverable

10. SUBMIT deliverable on-chain
    - Encode calldata: submit(TIDE_JOB_ID, deliverable_hash)
    - Sign transaction via KMS (same flow as review runtime)
    - Submit to Sei RPC
    - Wait for receipt
    - On revert: exit 52
    - On RPC failure after 3 retries: exit 51

11. WRITE termination message and debug status file
    - Write compact AgentResult JSON to TIDE_RESULT_PATH (default /dev/termination-log)
    - Write rich status JSON to /workspace/.tide/status.json (debug only)
    - Exit 0
```

### OpenHands Integration

OpenHands is invoked programmatically in headless mode:

```python
from openhands.core.config import AppConfig, SandboxConfig, LLMConfig
from openhands.core.main import create_runtime, run_controller
from openhands.events.action import MessageAction

config = AppConfig(
    llm=LLMConfig(
        model=os.environ.get("TIDE_LLM_MODEL", "claude-sonnet-4-20250514"),
        api_key=read_file("/secrets/anthropic-api-key"),
        max_output_tokens=int(os.environ.get("TIDE_LLM_MAX_OUTPUT_TOKENS", "16384")),
    ),
    sandbox=SandboxConfig(
        # OpenHands runs inside our container; disable its own sandboxing
        # since we already have K8s-level isolation
        use_host_network=True,
        browsergym_eval_env=None,
    ),
    max_iterations=int(os.environ.get("TIDE_MAX_ITERATIONS", "25")),
)

task_spec = read_file("/workspace/.tide/task-spec.md")
system_prompt = read_file("/secrets/agent-execution-system-prompt.txt")

full_prompt = f"""{system_prompt}

## Task

{task_spec}

## Working Directory

The source code is at /workspace/source/. Make your changes there.
Run tests with: {os.environ.get('TIDE_TEST_COMMAND', 'auto-detect from project')}

## Constraints

- Do not modify files outside /workspace/source/
- Do not install system packages
- Do not access the network except for running tests that need it
- Commit your changes when done
"""

runtime = create_runtime(config)
state = run_controller(
    config=config,
    initial_user_action=MessageAction(content=full_prompt),
    runtime=runtime,
)
```

**Token budget enforcement wrapper:**

The execution runtime wraps OpenHands' LLM calls to track token consumption:

```python
class BudgetEnforcingLLM:
    def __init__(self, inner_llm, budget):
        self.inner = inner_llm
        self.budget = budget
        self.used = 0

    def completion(self, messages, **kwargs):
        if self.used >= self.budget:
            raise TokenBudgetExceeded(self.used, self.budget)
        response = self.inner.completion(messages, **kwargs)
        self.used += response.usage.input_tokens + response.usage.output_tokens
        return response
```

When the budget is exceeded mid-iteration:
1. Allow the current iteration to complete (don't cut off mid-edit).
2. Skip further iterations.
3. Run a final test pass.
4. If tests pass → proceed to push/PR/submit (exit 0).
5. If tests fail → write status with `status: "partial"`, exit 31.

### SWE-agent Integration (Alternative)

When `TIDE_CODING_FRAMEWORK=swe-agent`:

```python
from sweagent import Agent, AgentConfig
from sweagent.environment import SWEEnvConfig

env_config = SWEEnvConfig(
    repo_path="/workspace/source",
    environment_setup=os.environ.get("TIDE_TEST_COMMAND", ""),
)

agent_config = AgentConfig(
    model_name=os.environ.get("TIDE_LLM_MODEL"),
    api_key=read_file("/secrets/anthropic-api-key"),
    max_iterations=int(os.environ.get("TIDE_MAX_ITERATIONS", "25")),
)

agent = Agent(agent_config)
result = agent.run(
    task=read_file("/workspace/.tide/task-spec.md"),
    env_config=env_config,
)
```

### Custom Agent Integration

When `TIDE_CODING_FRAMEWORK=custom`:

```bash
#!/bin/bash
# The Operator (or principal) provides a custom agent script at task creation time.
# The script receives the task spec on stdin and works in /workspace/source/.
exec /workspace/.tide/custom-agent.sh < /workspace/.tide/task-spec.md
```

The custom script must:
- Make changes in `/workspace/source/`
- Exit 0 on success, non-zero on failure
- Write test results to `/workspace/.tide/test-output.log`

### Transaction Submission (Deliverable)

Identical to the review runtime's KMS signing and transaction flow, but calling a different contract function:

```
1. Encode calldata:
   submit(uint256 jobId, bytes32 deliverableHash)
   - jobId = TIDE_JOB_ID
   - deliverableHash = bytes32(commit_sha)  (commit SHA is 20 bytes; left-pad to 32)

2. Sign and submit via KMS (same as review runtime §Transaction Submission)

3. Parse receipt:
   - success: deliverable recorded on-chain
   - revert "not the provider": agent address doesn't match job's provider
   - revert "job not funded": job is in wrong state
   - revert "job expired": past deadline
```

---

## Error Handling

| Stage | Error Condition | Detection | Exit Code | Recovery |
|-------|----------------|-----------|-----------|----------|
| Startup | Required env var missing | Check at process start | 10 | Fix Operator Job template. |
| Startup | Secret file missing | File existence check | 11 | Check SecretProviderClass. |
| Init: Clone | Upstream repo not found | git clone fails | 20 | Verify `TIDE_UPSTREAM_REPO`. |
| Init: Clone | Network timeout | git hangs > 120s | 20 | Retry; check NetworkPolicy. |
| Init: Commit | Upstream commit not in history | git checkout fails | 21 | Verify `TIDE_UPSTREAM_COMMIT`. |
| Init: Task | Task file not found | File not at `TIDE_TASK_FILE_PATH` | 22 | Fix path or push file to proposals repo. |
| Coding: LLM | API key invalid | HTTP 401 | 30 | Rotate key in Secrets Manager. |
| Coding: LLM | Rate limited | HTTP 429 | 30 (after retries) | Retry later. |
| Coding: LLM | Token budget exceeded | Running total check | 31 | Partial work may be usable. Retry with higher budget. |
| Coding: Agent | Max iterations with failing tests | Iteration counter | 32 | Code was produced but tests fail. Evaluator decides. Status file has test output. |
| Coding: Agent | Framework crash | Unhandled exception | 1 | Bug in framework integration. Platform team investigates. |
| Coding: Agent | Workspace filesystem full | ENOSPC | 1 | Increase emptyDir sizeLimit. |
| Push: Git | Auth expired (> 1h) | HTTP 401 | 40 | Long execution. Init container should be enhanced to refresh tokens (see Deferred). |
| Push: Git | Permissions denied | HTTP 403 | 40 | Check App's repo access permissions. |
| PR: GitHub | Rate limited | HTTP 403 or 429 | 41 | Wait and retry. |
| PR: GitHub | Branch already exists on deliverables | HTTP 422 | 41 | Force-push the branch, then create/update PR. |
| Sign: KMS | Access denied | AccessDeniedException | 50 | Check IRSA policy. |
| Submit: Sei | Not the provider | Revert | 52 | Agent address doesn't match job. Operator bug. |
| Submit: Sei | Job expired | Revert | 52 | Past deadline. USDC returns to treasury. |
| Submit: Sei | Already submitted | Revert | 52 | Idempotent from Operator perspective — treat as success. |
| General | Soft timeout (80% of `activeDeadlineSeconds`) | Timer check | 2 | Write termination message with partial results. Push whatever work exists, attempt PR. |
| General | K8s activeDeadlineSeconds | SIGTERM | 143 | Termination message may not exist. Operator treats as timeout. |
| General | OOMKilled | Container limit | 137 | Increase memory limit. |

### SIGTERM Handling

The main container installs a SIGTERM handler:

```python
import signal

shutdown_requested = False

def handle_sigterm(signum, frame):
    global shutdown_requested
    shutdown_requested = True

signal.signal(signal.SIGTERM, handle_sigterm)
```

On SIGTERM (K8s `activeDeadlineSeconds` or pod eviction):
1. Signal the coding agent to stop after the current iteration.
2. Commit and push whatever changes exist.
3. Create PR if possible (skip on-chain submission to save time).
4. Write termination message (`AgentResult` with `status: "failure"`) and debug status file with `status: "partial"`.
5. Exit with code 2 (soft timeout).

The K8s Job should have `terminationGracePeriodSeconds: 60` to allow this cleanup.

---

## Test Specification

### Unit Tests

| Test | Setup | Action | Expected Result |
|------|-------|--------|-----------------|
| `test_env_validation_all_present` | All required env vars set | Validate | Success |
| `test_env_validation_missing_job_id` | Omit `TIDE_JOB_ID` | Validate | Exit 10 |
| `test_workspace_branch_default` | `TIDE_WORKSPACE_BRANCH` unset, `TIDE_JOB_ID=7` | Compute branch name | `"job-7"` |
| `test_workspace_branch_override` | `TIDE_WORKSPACE_BRANCH=custom-branch` | Compute branch name | `"custom-branch"` |
| `test_task_spec_merge_env_only` | `TIDE_TASK_DESCRIPTION` set, `TIDE_TASK_FILE_PATH` unset | Merge task spec | Output equals `TIDE_TASK_DESCRIPTION` |
| `test_task_spec_merge_with_file` | Both env var and file path set | Merge task spec | Output contains both, file content appended |
| `test_deliverable_hash_computation` | Known commit SHA `abc123...` (40 hex chars) | Compute `bytes32` | `0x000000000000000000000000abc123...` (left-padded) |
| `test_pr_title_generation` | Job ID 7, task "Implement caching layer for..." | Generate title | `"[tide-job-7] Implement caching layer for..."` |
| `test_pr_body_template` | All metadata fields populated | Generate body | Valid markdown with all sections |
| `test_token_budget_enforcement` | Budget 100000, usage accumulating | Budget check after each call | Raises `TokenBudgetExceeded` at threshold |
| `test_iteration_log_append` | Multiple iterations | Write iteration log | Valid JSONL file, one line per action |
| `test_termination_message_success` | All stages succeed | Write termination message | Valid `AgentResult` JSON at `TIDE_RESULT_PATH`, status="success", deliverableHash and commitSha populated |
| `test_termination_message_failure` | KMS fails | Write termination message | status="failure", error message populated, fits in 4KB |
| `test_debug_status_file_success` | All stages succeed | Write debug status file | Valid JSON at `/workspace/.tide/status.json`, `status="success"`, all fields populated |
| `test_debug_status_file_partial_budget` | Budget exceeded mid-coding | Write debug status file | `status="partial"`, `exit_code=31` |
| `test_debug_status_file_partial_iterations` | Max iterations reached, tests failing | Write debug status file | `status="partial"`, `exit_code=32` |
| `test_sigterm_handler` | Send SIGTERM during coding | Check flag | `shutdown_requested == True` |

### Integration Tests

| Test | Setup | Action | Expected Result |
|------|-------|--------|-----------------|
| `test_e2e_execution_happy_path` | Sei testnet with funded ERC-8183 job. Simple task (e.g., "add a README"). GitHub repos provisioned. | Run full container | Exit 0. Code pushed. PR opened. Deliverable submitted on-chain. Status file complete. |
| `test_e2e_test_iteration` | Task requires fixing a failing test | Run container | Agent iterates, tests pass, exit 0. Iteration log shows multiple cycles. |
| `test_e2e_budget_exceeded` | Low token budget (50K), complex task | Run container | Exit 31. Partial code pushed. Status shows budget exhaustion. |
| `test_e2e_max_iterations` | Very hard task, low iteration limit (3) | Run container | Exit 32. Code pushed but tests may fail. Status shows iteration count. |
| `test_e2e_expired_job` | Job with past expiry on-chain | Run container through to submission | Exit 52. Code was pushed and PR opened, but on-chain submission reverted. |
| `test_init_upstream_pin` | `TIDE_UPSTREAM_COMMIT` set to valid SHA | Run init | Clone at exact commit. `repo-metadata.json` shows correct SHA. |
| `test_init_upstream_pin_missing` | `TIDE_UPSTREAM_COMMIT` set to nonexistent SHA | Run init | Exit 21. |
| `test_framework_openhands` | `TIDE_CODING_FRAMEWORK=openhands`, simple task | Run coding stage | OpenHands produces valid changes. |
| `test_framework_custom` | `TIDE_CODING_FRAMEWORK=custom`, script at expected path | Run coding stage | Custom script executed, exit code propagated. |

### E2E Smoke Test

```bash
#!/bin/bash
# Smoke test: fund a test job, run agent execution, verify deliverable

TESTNET_RPC="https://evm-rpc-testnet.sei-apis.com"

# 1. Create and fund a test job on ERC-8183
#    (assumes test USDC and treasury are pre-provisioned)
JOB_ID=$(cast send $ACP_ADDRESS \
    "createJob(address,address,uint256,uint48,string)" \
    $AGENT_WALLET $EVALUATOR_WALLET 1000000 86400 "Add README.md" \
    --rpc-url $TESTNET_RPC --private-key $TREASURY_KEY \
    | jq -r '.logs[0].topics[1]')

cast send $USDC_ADDRESS "approve(address,uint256)" $ACP_ADDRESS 1000000 \
    --rpc-url $TESTNET_RPC --private-key $TREASURY_KEY
cast send $ACP_ADDRESS "fund(uint256)" $JOB_ID \
    --rpc-url $TESTNET_RPC --private-key $TREASURY_KEY

# 2. Run the execution container
kubectl apply -f - <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: smoke-test-execution
  namespace: tide-agents
spec:
  # ... (full job spec with test env vars, simple README task)
EOF

# 3. Wait for completion (longer timeout — coding takes time)
kubectl wait --for=condition=complete job/smoke-test-execution \
    -n tide-agents --timeout=900s

# 4. Verify exit code
EXIT_CODE=$(kubectl get pod -l job-name=smoke-test-execution -n tide-agents \
    -o jsonpath='{.items[0].status.containerStatuses[0].state.terminated.exitCode}')
[ "$EXIT_CODE" -eq 0 ] || { echo "FAIL: exit code $EXIT_CODE"; exit 1; }

# 5. Verify PR exists
PR_COUNT=$(gh pr list --repo sei-tide/deliverables --label "smoke-test" --json number | jq length)
[ "$PR_COUNT" -gt 0 ] || { echo "FAIL: no PR created"; exit 1; }

# 6. Verify on-chain submission
DELIVERABLE=$(cast call $ACP_ADDRESS "getDeliverable(uint256)" $JOB_ID --rpc-url $TESTNET_RPC)
[ "$DELIVERABLE" != "0x0000000000000000000000000000000000000000000000000000000000000000" ] \
    || { echo "FAIL: no deliverable on-chain"; exit 1; }

echo "PASS: execution runtime smoke test"
```

---

## Deployment

### Dockerfile

```dockerfile
FROM python:3.12-slim AS base

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd -g 1000 tide && useradd -u 1000 -g tide -m tide

RUN pip install --no-cache-dir \
    openhands-ai==0.30.* \
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
ENTRYPOINT ["python", "run_execution.py"]
```

**Single image, dual entrypoint** (same pattern as review runtime):
- Init: `command: ["python", "/app/init_workspace.py"]`
- Main: `command: ["python", "/app/run_execution.py"]`

### Image Tagging

```
{ecr-repo}/tide-execution-runtime:{git-sha-short}
{ecr-repo}/tide-execution-runtime:{semver}
{ecr-repo}/tide-execution-runtime:latest  # testnet only
```

### Resource Requirements

The execution runtime needs more resources than the review runtime because the coding agent framework is memory-intensive and may compile/test code:

| Container | CPU Request | CPU Limit | Memory Request | Memory Limit |
|-----------|-------------|-----------|----------------|--------------|
| `workspace-setup` (init) | 250m | 1 | 512Mi | 2Gi |
| `agent` (main) | 1 | 4 | 2Gi | 8Gi |

These are defaults. The Operator may override based on the `LimitRange` and task complexity.

### Build Pipeline

```yaml
# .github/workflows/build-execution-runtime.yml
on:
  push:
    paths: ["agent-execution-runtime/**"]
    branches: [main]
  pull_request:
    paths: ["agent-execution-runtime/**"]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run unit tests
        run: cd agent-execution-runtime && python -m pytest tests/ -v
      - name: Lint
        run: cd agent-execution-runtime && ruff check src/
      - name: Build image
        run: docker build -t tide-execution-runtime:${{ github.sha }} agent-execution-runtime/
      - name: Push to ECR
        if: github.ref == 'refs/heads/main'
        run: |
          aws ecr get-login-password | docker login --username AWS --password-stdin $ECR_REPO
          docker tag tide-execution-runtime:${{ github.sha }} $ECR_REPO/tide-execution-runtime:${{ github.sha }}
          docker push $ECR_REPO/tide-execution-runtime:${{ github.sha }}
```

### Testnet vs Mainnet Differences

| Aspect | Testnet (arctic-1) | Mainnet |
|--------|-------------------|---------|
| `TIDE_SEI_CHAIN_ID` | `713715` | `1329` |
| `TIDE_SEI_RPC_URL` | Testnet RPC | Mainnet RPC |
| `TIDE_ACP_CONTRACT` | Testnet deployment | Mainnet deployment |
| `TIDE_LLM_TOKEN_BUDGET` | `500000` (lower) | `2000000` |
| `TIDE_MAX_ITERATIONS` | `10` (lower) | `25` |
| Container resource limits | Lower (2 CPU, 4Gi) | Higher (4 CPU, 8Gi) |
| Image tag | `latest` or git SHA | Semver only |
| `TIDE_LOG_LEVEL` | `debug` | `info` |

---

## Deferred (Do Not Build)

| Feature | Rationale |
|---------|-----------|
| Token refresh during long execution | YAGNI for Phase 2 — GitHub App tokens last 1h, and `activeDeadlineSeconds` caps execution. If runs regularly exceed 1h, add a sidecar token-refresh container in Phase 3. |
| Parallel framework execution (run OpenHands + SWE-agent, pick best) | YAGNI — expensive (2x LLM cost). Single framework is sufficient. |
| Automatic test discovery and framework detection | YAGNI — `TIDE_TEST_COMMAND` and `TIDE_CODING_FRAMEWORK` are explicit. Auto-detection is fragile. |
| Container-in-container sandbox for the coding agent | YAGNI — K8s PSS restricted profile provides sufficient isolation. If agents need to run untrusted code, add gVisor in Phase 3. |
| Multi-repo tasks (changes spanning multiple repos) | YAGNI — Phase 2 tasks target a single upstream repo. Multi-repo orchestration is Phase 3. |
| Incremental submission (submit partial work for partial payment) | YAGNI — ERC-8183 is all-or-nothing in Phase 2. Milestone-based escrow is a future ERC-8183 extension. |
| PR review automation (auto-request reviews, respond to comments) | YAGNI — the PR is created; human evaluation handles the rest. |
| Cost tracking / USDC-per-token accounting | YAGNI — `token_usage` in the status file is sufficient. Cost dashboards are Phase 3+. |
