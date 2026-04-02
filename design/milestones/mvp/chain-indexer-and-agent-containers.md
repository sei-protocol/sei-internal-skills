# Component: MVP Chain Indexer and Agent Container Spec

**Date:** 2026-04-02
**Status:** Draft

---

## Owner

Kubernetes Specialist (platform team)

## Phase

MVP -- replaces the full Go controller-runtime operator with a lightweight Tekton-driven architecture. The chain indexer bridges on-chain events to Tekton EventListener via HTTP POST. Agent containers run as Tekton TaskRuns rather than operator-managed batch/v1 Jobs.

## Purpose

Two deliverables in one spec:

1. **Chain Indexer** -- a small Python CronJob that polls `eth_getLogs` on Sei arctic-1 testnet, decodes TideCouncil/TideJobHook events, and POSTs structured JSON to a Tekton EventListener. This replaces the full Go event indexer designed in `lld-tide-operator.md`.

2. **Agent Container Spec** -- the base container image, entrypoint pattern, env var mapping, volume mounts, init container, RBAC, and NetworkPolicy that Tekton TaskRuns use to run review and execution agents. This reuses the interface contracts from the interface registry without changes.

**Business needs served:** #1 (design review), #2 (quorum consensus), #3 (funded jobs), #4 (isolated execution).

---

## Dependencies

### External Systems Consumed

| System | Interface | Notes |
|--------|-----------|-------|
| Sei EVM RPC (arctic-1) | JSON-RPC 2.0 over HTTPS | `eth_getLogs` polling. URL: `https://evm-rpc-arctic-1.sei-apis.com`. Chain ID: `713715`. |
| TideCouncil contract | Solidity ABI (event signatures from interface registry) | Source of proposal lifecycle events. |
| TideJobHook contract | Solidity ABI (event signatures from interface registry) | Emits `SandboxProvisionRequested`. |
| Tekton Pipelines | EventListener HTTP endpoint | Receives structured JSON POSTs, stamps out TaskRuns. |
| GitHub API v3 | REST, authenticated via GitHub App | Init container generates installation tokens. |
| AWS KMS | `kms:Sign`, `kms:GetPublicKey` via IRSA | Agent signing operations. |
| AWS Secrets Manager | Via CSI SecretProviderClass | Secrets mounted into agent TaskRun pods. |

### Explicit Exclusions

- Does NOT use WebSocket subscriptions (`eth_subscribe`). Polling is sufficient for MVP cadence.
- Does NOT use controller-runtime, CRDs, or leader election. Tekton owns the workload lifecycle.
- Does NOT include the full TideProposal/TideJob state machines. The Tekton pipeline is simpler: event arrives, TaskRun runs, agent writes result.

---

## 1. On-Chain Event Indexer

### Recommendation: Go Deployment Extracted from Hourglass EVM Chain Poller

**Basis:** The `EVMChainPoller` from [`Layr-Labs/hourglass-monorepo`](https://github.com/Layr-Labs/hourglass-monorepo/blob/master/ponos/pkg/chainPoller/EVMChainPoller/evmChainPoller.go) provides a production-grade EVM polling loop (~480 lines) with features we'd otherwise need to build: block-by-block cursor persistence, reorg detection (parentHash chain validation with configurable `MaxReorgDepth`), crash recovery, and exponential-backoff retry on the HTTP JSON-RPC client (7 retries, 1s→60s). Sei's team already has operational familiarity with this codebase.

**What we keep from Hourglass (~250 lines):**
- `pollForBlocks()` → `processNextBlock()` → `processBlockLogs()` — core polling loop with `time.Ticker`
- `reconcileReorg()` / `findOrphanedBlocks()` — reorg safety (CometBFT rarely reorgs, but free insurance)
- `ethereum.Client` HTTP JSON-RPC implementation with built-in retry
- Block cursor persistence pattern (adapted from their `AggregatorStore` to a simpler ConfigMap store)

**What we replace (~200 lines):**
| Hourglass component | Tide replacement |
|---------------------|------------------|
| `handleLog()` filters for `TaskCreated`, pushes to channel | Filter for Tide events, HTTP POST to Tekton EventListener |
| `AggregatorStore` (12-method interface, Badger/memory) | `BlockCursorStore` with 3-4 methods (ConfigMap read/write) |
| `IContractStore` + dynamic ABI lookup | Compiled TideCouncil ABI via `abigen` (static, embedded) |
| `IBlockContextManager` (task deadlines) | Remove entirely |
| `GetLogsRequest` (no topic filtering) | Add `topics` field — filter by Tide event signature hashes at the RPC level |
| `types.Task` (EigenLayer-specific) | `DecodedEvent` struct that serializes to JSON for Tekton POST |
| `config.ChainId` (Ethereum mainnet/testnet only) | Add `ChainId_SeiArctic1 = 713715`, `ChainId_SeiMainnet = 1329` |

**Result:** ~450 lines of Go, single binary, single container as a Deployment (not CronJob — the Hourglass pattern uses a long-lived `time.Ticker` loop, which is simpler than CronJob scheduling for sub-minute polling).

**Why Go over Python (revised):**
- We already have a battle-tested Go poller to extract from, including retry logic and reorg detection
- `go-ethereum/accounts/abi` handles ABI decoding natively via `abigen`
- Single static binary — no Python runtime, pip dependencies, or web3.py version management
- Operational familiarity within the Sei team

**Architecture:**

```mermaid
graph LR
    A[Sei arctic-1 RPC] -->|eth_getLogs with topics filter| B[Go Deployment: tide-chain-poller]
    B -->|HTTP POST| C[Tekton EventListener]
    B -->|read/write cursor| D[ConfigMap: tide-event-cursor]
    C -->|TriggerBinding + TriggerTemplate| E[TaskRun]
```

### 1.1 Event Filtering

Topic hashes are computed from the interface registry event signatures using `keccak256`. The indexer watches these topic0 values across two contract addresses.

**TideCouncil events:**

| Event | Signature (from registry) | Topic0 |
|-------|---------------------------|--------|
| ProposalCreated | `ProposalCreated(uint256,address,bytes32,uint256,uint256[],uint8,uint40)` | `keccak256(signature)` |
| ReviewSubmitted | `ReviewSubmitted(uint256,uint256,uint8,bytes32)` | `keccak256(signature)` |
| ProposalApproved | `ProposalApproved(uint256,bytes32)` | `keccak256(signature)` |
| ProposalRejected | `ProposalRejected(uint256,bytes32)` | `keccak256(signature)` |
| ProposalExpired | `ProposalExpired(uint256)` | `keccak256(signature)` |

**TideJobHook events:**

| Event | Signature (from registry) | Topic0 |
|-------|---------------------------|--------|
| SandboxProvisionRequested | `SandboxProvisionRequested(uint256,address,address,uint256,uint256,uint256)` | `keccak256(signature)` |

Note: Topic0 hex values are computed at runtime by `Web3.keccak(text=signature)`. They are NOT hardcoded as strings in the script to avoid transcription errors. The signatures above are byte-identical with the interface registry `events` section.

### 1.2 Cursor Persistence

The indexer persists its position in a ConfigMap `tide-event-cursor` in the `tide-system` namespace. This is the same cursor design from `lld-tide-operator.md` section "Event Cursor (ConfigMap)", reused without changes.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tide-event-cursor
  namespace: tide-system
data:
  lastProcessedBlock: "0"
  lastProcessedTxIndex: "0"
  lastProcessedLogIndex: "0"
```

**Cursor semantics:**
- On startup, read cursor from ConfigMap. If not found or empty, start from `INDEXER_START_BLOCK` env var (set to the contract deployment block).
- Process logs in order: block number, then tx index, then log index.
- Update cursor **after** each successful POST to Tekton EventListener.
- At-least-once delivery: a crash between POST and cursor update re-delivers the event. Tekton TriggerTemplates must be idempotent (TaskRun names derived from on-chain IDs).

### 1.3 ABI Decoding

The Go indexer uses compiled TideCouncil ABI via `abigen` (from `go-ethereum/accounts/abi/bind`). The ABI is embedded in the binary at compile time — no external ABI JSON file needed at runtime.

```go
// Generated by: abigen --abi tide-council.abi.json --pkg contracts --out tide_council.go
import "github.com/tide/indexer/pkg/contracts"

// Decode uses the generated contract bindings
event, err := contracts.ParseProposalCreated(log)
// event.ProposalId, event.Principal, event.DesignHash, event.ParticipantTokenIds, etc.
```

This handles all ABI types including `uint256[]` (participantTokenIds) and `uint40` (expiresAt) correctly. The `go-ethereum/accounts/abi` package provides type-safe decoding with compile-time validation.

### 1.4 Tekton Integration

The indexer POSTs to the Tekton EventListener at a configurable URL. The JSON schema is defined in section 4 below.

```go
func (p *TidePoller) postToTekton(ctx context.Context, payload DecodedEvent) error {
    body, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("marshal event: %w", err)
    }
    req, _ := http.NewRequestWithContext(ctx, "POST", p.config.WebhookURL, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")

    resp, err := p.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("POST to tekton: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 200 && resp.StatusCode < 300 {
        p.logger.Info("posted event", zap.String("event_type", payload.EventType),
            zap.Uint64("block", payload.BlockNumber))
        return nil
    }
    return fmt.Errorf("tekton returned %d", resp.StatusCode)
}
```

### 1.5 Failure Handling

| Failure Mode | Detection | Action |
|-------------|-----------|--------|
| RPC endpoint down | HTTP connection error or 5xx from RPC | Inherited from Hourglass `EthereumClient.Call()`: exponential backoff with 7 retries (1s→60s). If still failing after retries, log error and continue polling on next tick. Cursor is unchanged, so no events are skipped. |
| RPC rate limited | HTTP 429 from RPC | Same retry logic from Hourglass — exponential backoff handles 429s transparently. |
| Missed blocks (Deployment restart) | `latestBlock - cursorBlock > BATCH_SIZE` | Paginate `eth_getLogs` in batches of `BATCH_SIZE` (default 1000). Process all missed blocks before posting new events. The long-lived Deployment resumes from cursor on restart. |
| Tekton EventListener down | HTTP 5xx or connection refused on POST | Retry 3 times with 2s backoff. If still failing, skip and retry on next poll tick. Cursor is NOT updated, so the event will be re-processed. |
| Duplicate delivery | Deployment restarts and re-processes same block range | Tekton TriggerTemplate generates TaskRun names deterministically from on-chain IDs (e.g., `review-proposal-7-agent-1`). Kubernetes rejects duplicate TaskRun creation with `AlreadyExists`. This is safe. |
| Reorg (chain reorganization) | `parentHash` chain validation (from Hourglass) | Inherited from the Hourglass poller: before processing each block, verify `newBlock.parentHash == storedPreviousBlock.hash`. On mismatch, walk backwards up to `MaxReorgDepth` (default 10) to find orphaned blocks, delete them, and re-process. CometBFT rarely reorgs, but the safety is free. |

### 1.6 Configuration

All configuration via environment variables on the Deployment pod. These are indexer-internal vars (no `TIDE_` prefix) since the indexer is not an agent runtime:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SEI_RPC_URL` | Yes | -- | Sei EVM HTTP RPC endpoint |
| `COUNCIL_ADDRESS` | Yes | -- | TideCouncil contract address (0x-prefixed) |
| `JOB_HOOK_ADDRESS` | Yes | -- | TideJobHook contract address (0x-prefixed). Set to `0x0000000000000000000000000000000000000000` for MVP (TideJobHook not deployed). |
| `TEKTON_LISTENER_URL` | Yes | -- | Tekton EventListener URL (e.g., `http://el-tide-github.tekton-tide:8080`) |
| `INDEXER_START_BLOCK` | No | `0` | Block to start from if no cursor exists |
| `BATCH_SIZE` | No | `1000` | Max blocks per `eth_getLogs` call |
| `POLL_INTERVAL` | No | `15s` | Polling interval (Go duration string) |
| `NAMESPACE` | No | `tide-system` | Namespace for cursor ConfigMap |

### 1.7 Go Indexer Architecture (Extracted from Hourglass)

The indexer is a single Go binary extracted from the [Hourglass EVMChainPoller](https://github.com/Layr-Labs/hourglass-monorepo/blob/master/ponos/pkg/chainPoller/EVMChainPoller/evmChainPoller.go). The source lives at `pkg/indexer/` in this repo.

**Package structure:**

```
pkg/indexer/
  main.go              -- entry point, config loading, signal handling
  poller.go            -- core polling loop (extracted from Hourglass pollForBlocks/processNextBlock)
  decoder.go           -- ABI decoding using abigen-generated bindings
  poster.go            -- HTTP POST to Tekton EventListener with retry
  cursor.go            -- ConfigMap-based cursor persistence (simplified from Hourglass AggregatorStore)
  reorg.go             -- parentHash chain validation (extracted from Hourglass reconcileReorg)
  types.go             -- DecodedEvent struct, config types
pkg/contracts/
  tide_council.go      -- abigen-generated TideCouncil bindings
  tide_job_hook.go     -- abigen-generated TideJobHook bindings (stubbed for MVP)
```

**Core loop (from Hourglass `pollForBlocks`):**

```go
func (p *TidePoller) Run(ctx context.Context) error {
    ticker := time.NewTicker(p.config.PollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            if err := p.processNextBlocks(ctx); err != nil {
                p.logger.Error("poll cycle failed", zap.Error(err))
                // Continue polling -- cursor unchanged, will retry
            }
        }
    }
}

func (p *TidePoller) processNextBlocks(ctx context.Context) error {
    cursor, err := p.cursorStore.Read(ctx)
    if err != nil {
        return fmt.Errorf("read cursor: %w", err)
    }

    latest, err := p.ethClient.BlockNumber(ctx)
    if err != nil {
        return fmt.Errorf("get latest block: %w", err)
    }

    fromBlock := cursor.BlockNumber + 1
    if fromBlock > latest {
        return nil // no new blocks
    }

    // Process in batches
    for from := fromBlock; from <= latest; from += p.config.BatchSize {
        to := min(from+p.config.BatchSize-1, latest)

        // Reorg detection (from Hourglass reconcileReorg)
        if err := p.checkReorg(ctx, from); err != nil {
            return fmt.Errorf("reorg detected at block %d: %w", from, err)
        }

        // Fetch logs with topic filter
        logs, err := p.ethClient.FilterLogs(ctx, ethereum.FilterQuery{
            FromBlock: new(big.Int).SetUint64(from),
            ToBlock:   new(big.Int).SetUint64(to),
            Addresses: p.config.ContractAddresses,
            Topics:    [][]common.Hash{p.config.EventTopics},
        })
        if err != nil {
            return fmt.Errorf("filter logs %d-%d: %w", from, to, err)
        }

        for _, log := range logs {
            event, err := p.decoder.Decode(log)
            if err != nil {
                p.logger.Warn("decode failed", zap.Error(err))
                continue
            }
            if err := p.postToTekton(ctx, event); err != nil {
                return fmt.Errorf("post event: %w", err)
            }
        }

        // Update cursor after successful POST
        p.cursorStore.Write(ctx, Cursor{BlockNumber: to, TxIndex: 0, LogIndex: 0})
    }
    return nil
}
```

**Key improvement over Hourglass:** The `FilterLogs` call includes `Topics` (event signature hashes) so the RPC does the filtering, reducing bandwidth. Hourglass fetches all logs and filters in application code.

**Estimated size:** ~450 lines of Go total (250 from Hourglass core + 200 for Tide-specific handler/config).

The previous Python implementation has been superseded by this Go extraction.

### 1.8 Deployment Manifest

The indexer runs as a Deployment with a single replica (not a CronJob). The Hourglass `time.Ticker` pattern provides sub-minute polling without CronJob scheduling limitations.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tide-chain-indexer
  namespace: tide-system
  labels:
    app.kubernetes.io/name: tide-chain-indexer
    app.kubernetes.io/part-of: tide
    app.kubernetes.io/component: indexer
spec:
  replicas: 1
  strategy:
    type: Recreate  # Only one indexer at a time
  selector:
    matchLabels:
      app.kubernetes.io/name: tide-chain-indexer
  template:
    metadata:
      labels:
        app.kubernetes.io/name: tide-chain-indexer
        app.kubernetes.io/component: indexer
    spec:
      serviceAccountName: tide-indexer
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: indexer
          image: "ghcr.io/sei-protocol/tide-chain-indexer:MVP_VERSION"
          imagePullPolicy: IfNotPresent
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop: ["ALL"]
          env:
            - name: SEI_RPC_URL
              valueFrom:
                configMapKeyRef:
                  name: tide-platform-config
                  key: SEI_RPC_URL
            - name: COUNCIL_ADDRESS
              valueFrom:
                configMapKeyRef:
                  name: tide-platform-config
                  key: COUNCIL_ADDRESS
            - name: JOB_HOOK_ADDRESS
              valueFrom:
                configMapKeyRef:
                  name: tide-platform-config
                  key: JOB_HOOK_ADDRESS
            - name: TEKTON_LISTENER_URL
              value: "http://el-tide-github.tekton-tide.svc.cluster.local:8080"
            - name: INDEXER_START_BLOCK
              valueFrom:
                configMapKeyRef:
                  name: tide-platform-config
                  key: INDEXER_START_BLOCK
            - name: POLL_INTERVAL
              value: "15s"
            - name: BATCH_SIZE
              value: "1000"
            - name: NAMESPACE
              value: "tide-system"
          volumeMounts:
            - name: tmp
              mountPath: /tmp
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 128Mi
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8081
            initialDelaySeconds: 10
            periodSeconds: 60
          readinessProbe:
            httpGet:
              path: /readyz
              port: 8081
            initialDelaySeconds: 5
            periodSeconds: 30
      volumes:
        - name: tmp
          emptyDir:
            sizeLimit: 64Mi
```

Note: No ABI ConfigMap volume needed — ABIs are compiled into the Go binary via `abigen`.

---

## 2. Agent Container Spec

### 2.1 Base Image

The agent base image contains the runtime dependencies for both review and execution tasks. A single Dockerfile produces the base, with the entrypoint determining the mode.

**Contents:**

| Component | Version | Purpose |
|-----------|---------|---------|
| Python 3.12 | `python:3.12-slim` base | Runtime for agent logic |
| `anthropic` SDK | latest | Claude API calls |
| `web3.py` | latest | Sei RPC interaction, ABI encoding, EIP-712 signing |
| `boto3` | latest | AWS KMS signing via IRSA |
| `PyGithub` + `PyJWT` | latest | GitHub App token generation, API calls |
| `git` | system package | Repo clone, branch, push |
| `gh` CLI | latest | PR creation, issue management |
| `jq` | system package | JSON manipulation in shell scripts |
| `curl` | system package | Health checks, debugging |

**Dockerfile sketch:**

```dockerfile
FROM python:3.12-slim AS base

# System dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    jq \
    openssh-client \
    && rm -rf /var/lib/apt/lists/*

# Install GitHub CLI
RUN curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
    | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
    | tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
    && apt-get update && apt-get install -y gh \
    && rm -rf /var/lib/apt/lists/*

# Python dependencies
COPY requirements.txt /app/requirements.txt
RUN pip install --no-cache-dir -r /app/requirements.txt

# Application code
COPY src/ /app/src/

# Non-root user
RUN groupadd -r tide && useradd -r -g tide -d /workspace -s /bin/bash tide
USER tide

WORKDIR /workspace

ENTRYPOINT ["python3", "/app/src/entrypoint.py"]
```

**`requirements.txt`:**

```
anthropic>=0.40.0
web3>=7.0.0
boto3>=1.35.0
PyGithub>=2.0.0
PyJWT[crypto]>=2.8.0
cryptography>=42.0.0
requests>=2.31.0
```

### 2.2 Entrypoint Pattern

The container uses a single Python entrypoint that dispatches to the correct runtime based on the `TIDE_RUNTIME_MODE` env var:

```python
#!/usr/bin/env python3
"""Agent entrypoint. Dispatches to review or execution runtime based on TIDE_RUNTIME_MODE."""

import os
import sys

def main() -> int:
    mode = os.environ.get("TIDE_RUNTIME_MODE", "")
    if mode == "review":
        from tide_agent.review import run_review
        return run_review()
    elif mode == "execution":
        from tide_agent.execution import run_execution
        return run_execution()
    else:
        sys.stderr.write(f"FATAL: unknown TIDE_RUNTIME_MODE: {mode!r}\n")
        return 10  # Exit code 10: missing/invalid env var (per interface registry)

if __name__ == "__main__":
    sys.exit(main())
```

### 2.3 Environment Variables

All env vars from the interface registry, mapped to Tekton TaskRun params. The Tekton TriggerBinding extracts values from the indexer JSON payload and passes them as TaskRun params. The TriggerTemplate maps params to container env vars.

**Required env vars (set by Tekton TriggerTemplate from params):**

| Variable | Source | Review | Execution |
|----------|--------|--------|-----------|
| `TIDE_PROPOSAL_ID` | Indexer payload `data.proposalId` | Yes | Yes |
| `TIDE_DESIGN_HASH` | Indexer payload `data.designHash` | Yes | Yes |
| `TIDE_AGENT_TOKEN_ID` | Per-agent from ConfigMap or TriggerTemplate | Yes | Yes |
| `TIDE_AGENT_NAME` | Per-agent from ConfigMap | Yes | Yes |
| `TIDE_KMS_KEY_ID` | Per-agent from ConfigMap (KMS ARN) | Yes | Yes |
| `TIDE_COUNCIL_ADDRESS` | Platform ConfigMap | Yes | No |
| `TIDE_ACP_ADDRESS` | Platform ConfigMap | No | Yes |
| `TIDE_SEI_RPC_URL` | Platform ConfigMap | Yes | Yes |
| `TIDE_SEI_CHAIN_ID` | Platform ConfigMap (`713715` for testnet) | Yes | Yes |
| `TIDE_GITHUB_APP_INSTALLATION_ID` | Per-agent from ConfigMap | Yes | Yes |
| `TIDE_PROPOSALS_REPO` | Platform ConfigMap | Yes | Yes |
| `TIDE_RESULT_PATH` | Hardcoded `/dev/termination-log` | Yes | Yes |
| `TIDE_AWS_REGION` | Platform ConfigMap | Yes | Yes |
| `TIDE_DESIGN_PATH` | Indexer payload or static config | Yes | No |
| `TIDE_RUNTIME_MODE` | TriggerTemplate (`review` or `execution`) | Yes | Yes |

**Optional env vars (with defaults per interface registry):**

| Variable | Default | Review | Execution |
|----------|---------|--------|-----------|
| `TIDE_LLM_MODEL` | `claude-sonnet-4-20250514` | Yes | Yes |
| `TIDE_LLM_TOKEN_BUDGET` | (from ConfigMap) | Yes | Yes |
| `TIDE_LLM_MAX_OUTPUT_TOKENS` | `16384` | Yes | Yes |
| `TIDE_LLM_MAX_INPUT_TOKENS` | `100000` | Yes | No |
| `TIDE_LLM_TEMPERATURE` | `0.3` | Yes | No |
| `TIDE_REVIEW_TIMEOUT_SECONDS` | `1800` | Yes | No |
| `TIDE_EXECUTION_TIMEOUT_SECONDS` | `3000` | No | Yes |
| `TIDE_MAX_ITERATIONS` | (from ConfigMap) | No | Yes |
| `TIDE_CODING_FRAMEWORK` | `openhands` | No | Yes |
| `TIDE_LOG_LEVEL` | `info` | Yes | Yes |
| `TIDE_JOB_ID` | Indexer payload `data.jobId` | No | Yes |
| `TIDE_UPSTREAM_REPO` | Platform ConfigMap | No | Yes |
| `TIDE_TASK_DESCRIPTION` | Indexer payload or static config | No | Yes |
| `TIDE_PROVIDER_ADDRESS` | Per-agent from ConfigMap | Yes | Yes |
| `TIDE_EXPIRES_AT` | Indexer payload `data.expiresAt` (converted to RFC 3339) | Yes | Yes |
| `TIDE_UPSTREAM_BRANCH` | `main` | No | Yes |
| `TIDE_UPSTREAM_COMMIT` | (optional) | No | Yes |
| `TIDE_PROPOSALS_REPO_BRANCH` | `main` | Yes | No |
| `TIDE_TEST_COMMAND` | (optional) | No | Yes |

### 2.4 Volume Mounts

Consistent with the interface registry `volume_mounts` section:

| Mount Path | Volume Type | Access | Size Limit | Purpose |
|------------|-------------|--------|------------|---------|
| `/workspace` | `emptyDir` | read-write | 10Gi | Agent working directory. Init container clones repos here. Main container works here. |
| `/secrets` | CSI `SecretProviderClass` | read-only | -- | Secrets from AWS Secrets Manager (GitHub App key, Anthropic API key, system prompts, contract ABIs). |
| `/tmp` | `emptyDir` | read-write | 1Gi | Temp directory for agent runtime. |

**`/secrets` directory layout (per interface registry):**

```
/secrets/
  github-app-key.pem              # GitHub App RSA private key (PKCS#1 PEM)
  anthropic-api-key               # Plaintext Anthropic API key
  agent-system-prompt.txt         # Review persona/instructions
  agent-execution-system-prompt.txt  # Execution persona/instructions
  tide-council-abi.json           # TideCouncil contract ABI
  acp-abi.json                    # ERC-8183 ACP contract ABI
```

**Tekton TaskRun workspace mapping:**

```yaml
# In the TriggerTemplate's TaskRun spec:
workspaces:
  - name: workspace
    emptyDir:
      sizeLimit: 10Gi
    # Mounted at /workspace by the Task step
  - name: secrets
    csi:
      driver: secrets-store.csi.k8s.io
      readOnly: true
      volumeAttributes:
        secretProviderClass: "tide-agent-$(params.agent_name)-secrets"
  - name: tmp
    emptyDir:
      sizeLimit: 1Gi
```

### 2.5 Init Container

The init container runs before the main agent step. It generates a GitHub App installation token, clones the required repositories, and verifies integrity.

```yaml
# Init step in Tekton Task
- name: workspace-setup
  image: "REGISTRY/tide-workspace-init:MVP_VERSION"
  script: |
    #!/usr/bin/env bash
    set -euo pipefail

    echo "=== Generating GitHub App installation token ==="
    # Generate JWT from App private key
    APP_ID="${TIDE_GITHUB_APP_ID}"
    KEY_PATH="/secrets/github-app-key.pem"
    INSTALLATION_ID="${TIDE_GITHUB_APP_INSTALLATION_ID}"

    # Create JWT (RS256)
    NOW=$(date +%s)
    IAT=$((NOW - 60))
    EXP=$((NOW + 600))

    HEADER=$(echo -n '{"alg":"RS256","typ":"JWT"}' | base64 -w0 | tr '+/' '-_' | tr -d '=')
    PAYLOAD=$(echo -n "{\"iat\":${IAT},\"exp\":${EXP},\"iss\":\"${APP_ID}\"}" | base64 -w0 | tr '+/' '-_' | tr -d '=')
    SIGNATURE=$(echo -n "${HEADER}.${PAYLOAD}" | openssl dgst -sha256 -sign "${KEY_PATH}" | base64 -w0 | tr '+/' '-_' | tr -d '=')
    JWT="${HEADER}.${PAYLOAD}.${SIGNATURE}"

    # Exchange JWT for installation token
    TOKEN=$(curl -s -X POST \
      -H "Authorization: Bearer ${JWT}" \
      -H "Accept: application/vnd.github+json" \
      "https://api.github.com/app/installations/${INSTALLATION_ID}/access_tokens" \
      | jq -r '.token')

    if [ -z "${TOKEN}" ] || [ "${TOKEN}" = "null" ]; then
      echo "FATAL: failed to generate installation token"
      exit 1
    fi

    # Write token for main container
    mkdir -p /workspace/.tide
    echo "${TOKEN}" > /workspace/.tide/github-token
    chmod 600 /workspace/.tide/github-token

    echo "=== Configuring git ==="
    git config --global credential.helper '!f() { echo "password=${TOKEN}"; }; f'
    git config --global user.name "Tide ${TIDE_AGENT_NAME}"
    git config --global user.email "tide-${TIDE_AGENT_NAME}@sei.io"

    echo "=== Cloning proposals repo ==="
    git clone --depth 1 \
      "https://x-access-token:${TOKEN}@github.com/${TIDE_PROPOSALS_REPO}.git" \
      /workspace/proposals

    if [ "${TIDE_RUNTIME_MODE}" = "review" ]; then
      echo "=== Verifying design hash ==="
      DESIGN_FILE="/workspace/proposals/${TIDE_DESIGN_PATH}"
      if [ ! -f "${DESIGN_FILE}" ]; then
        echo "FATAL: design doc not found at ${DESIGN_FILE}"
        exit 1  # Maps to exit code 21 in main container
      fi
    fi

    if [ "${TIDE_RUNTIME_MODE}" = "execution" ] && [ -n "${TIDE_UPSTREAM_REPO:-}" ]; then
      echo "=== Cloning upstream repo ==="
      BRANCH="${TIDE_UPSTREAM_BRANCH:-main}"
      git clone --depth 1 --branch "${BRANCH}" \
        "https://x-access-token:${TOKEN}@github.com/${TIDE_UPSTREAM_REPO}.git" \
        /workspace/src

      if [ -n "${TIDE_UPSTREAM_COMMIT:-}" ]; then
        cd /workspace/src
        git fetch --depth 1 origin "${TIDE_UPSTREAM_COMMIT}"
        git checkout "${TIDE_UPSTREAM_COMMIT}"
        cd /workspace
      fi
    fi

    echo "=== Workspace setup complete ==="
  env:
    - name: TIDE_GITHUB_APP_ID
      value: "$(params.github_app_id)"
    - name: TIDE_GITHUB_APP_INSTALLATION_ID
      value: "$(params.github_installation_id)"
    - name: TIDE_AGENT_NAME
      value: "$(params.agent_name)"
    - name: TIDE_PROPOSALS_REPO
      value: "$(params.proposals_repo)"
    - name: TIDE_DESIGN_PATH
      value: "$(params.design_path)"
    - name: TIDE_RUNTIME_MODE
      value: "$(params.runtime_mode)"
    - name: TIDE_UPSTREAM_REPO
      value: "$(params.upstream_repo)"
    - name: TIDE_UPSTREAM_BRANCH
      value: "$(params.upstream_branch)"
    - name: TIDE_UPSTREAM_COMMIT
      value: "$(params.upstream_commit)"
  volumeMounts:
    - name: workspace
      mountPath: /workspace
    - name: secrets
      mountPath: /secrets
      readOnly: true
    - name: tmp
      mountPath: /tmp
  securityContext:
    allowPrivilegeEscalation: false
    readOnlyRootFilesystem: true
    runAsNonRoot: true
    runAsUser: 1000
    capabilities:
      drop: ["ALL"]
    seccompProfile:
      type: RuntimeDefault
```

### 2.6 Completion Signaling

Exactly as defined in the interface registry `completion_signaling` and `exit_codes` sections.

**Primary mechanism:** Agent writes `AgentResult` JSON to `/dev/termination-log` (controlled by `TIDE_RESULT_PATH` env var). Kubernetes persists this in `pod.status.containerStatuses[].state.terminated.message` (max 4096 bytes).

**Exit codes (full table from interface registry):**

| Code | Meaning | Category |
|------|---------|----------|
| 0 | Success | success |
| 1 | Unrecoverable internal error | permanent_failure |
| 2 | Soft timeout (80% of deadline) | timeout |
| 10 | Missing/invalid env var | config_error |
| 11 | Secret mount failure | config_error |
| 20 | Git clone failure | transient_error |
| 21 | Design doc/upstream commit not found | permanent_failure |
| 22 | Design hash mismatch | permanent_failure |
| 30 | LLM API failure | transient_error |
| 31 | Token budget exceeded | resource_exhaustion |
| 32 | Max iterations exceeded | resource_exhaustion |
| 40 | Git push failure | transient_error |
| 41 | PR creation failure | transient_error |
| 50 | KMS signing failure | transient_error |
| 51 | Sei RPC failure | transient_error |
| 52 | Sei transaction reverted | permanent_failure |
| 137 | OOMKilled | resource_exhaustion |
| 143 | SIGTERM (deadline exceeded) | timeout |

**AgentResult JSON schema (per interface registry `agent_result_schema`):**

```json
{
  "status": "success | failure | timeout",
  "exit_code": 0,
  "deliverable_hash": "0x...",
  "error": "",
  "error_code": "",
  "token_usage": {
    "input_tokens": 50000,
    "output_tokens": 8000,
    "total_tokens": 58000
  },
  "timing": {
    "started_at": "2026-04-02T12:00:00Z",
    "completed_at": "2026-04-02T12:15:00Z",
    "duration_seconds": 900
  }
}
```

Note: The operator LLD's `AgentResult` Go struct includes additional fields (`prUrl`, `prNumber`, `commitSha`, `feedbackHash`, `verdict`, `txHash`, `errorStage`) beyond the interface registry's `agent_result_schema`. For MVP, the Tekton pipeline only needs to read the core fields (`status`, `exit_code`, `deliverable_hash`, `error`). The extended fields are optional and forward-compatible.

---

## 3. Agent Container RBAC

### 3.1 ServiceAccount Per Agent

One ServiceAccount per agent in the `tide-agents` namespace. Consistent with the interface registry `kubernetes.service_accounts` section and the K8s manifests LLD.

```yaml
# One ServiceAccount per canonical agent name.
# Canonical short names: blockchain-dev, k8s-specialist, platform-eng, coordinator, reviewer
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tide-agent-blockchain-dev
  namespace: tide-agents
  labels:
    app.kubernetes.io/name: tide-agent
    app.kubernetes.io/part-of: tide
    tide.sei.io/agent-id: blockchain-dev
  annotations:
    eks.amazonaws.com/role-arn: "arn:aws:iam::ACCOUNT_ID:role/tide-agent-blockchain-dev-irsa"
automountServiceAccountToken: false
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tide-agent-k8s-specialist
  namespace: tide-agents
  labels:
    app.kubernetes.io/name: tide-agent
    app.kubernetes.io/part-of: tide
    tide.sei.io/agent-id: k8s-specialist
  annotations:
    eks.amazonaws.com/role-arn: "arn:aws:iam::ACCOUNT_ID:role/tide-agent-k8s-specialist-irsa"
automountServiceAccountToken: false
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tide-agent-platform-eng
  namespace: tide-agents
  labels:
    app.kubernetes.io/name: tide-agent
    app.kubernetes.io/part-of: tide
    tide.sei.io/agent-id: platform-eng
  annotations:
    eks.amazonaws.com/role-arn: "arn:aws:iam::ACCOUNT_ID:role/tide-agent-platform-eng-irsa"
automountServiceAccountToken: false
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tide-agent-coordinator
  namespace: tide-agents
  labels:
    app.kubernetes.io/name: tide-agent
    app.kubernetes.io/part-of: tide
    tide.sei.io/agent-id: coordinator
  annotations:
    eks.amazonaws.com/role-arn: "arn:aws:iam::ACCOUNT_ID:role/tide-agent-coordinator-irsa"
automountServiceAccountToken: false
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tide-agent-reviewer
  namespace: tide-agents
  labels:
    app.kubernetes.io/name: tide-agent
    app.kubernetes.io/part-of: tide
    tide.sei.io/agent-id: reviewer
  annotations:
    eks.amazonaws.com/role-arn: "arn:aws:iam::ACCOUNT_ID:role/tide-agent-reviewer-irsa"
automountServiceAccountToken: false
```

**Key properties:**
- `automountServiceAccountToken: false` -- agent pods have zero K8s API access. They cannot list, watch, create, or delete any cluster resource.
- IRSA annotation gives each SA its own IAM role, scoped to that agent's KMS key and secrets path.

### 3.2 ServiceAccount for Indexer

The chain indexer needs ConfigMap read/write access for the cursor.

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tide-indexer
  namespace: tide-system
  labels:
    app.kubernetes.io/name: tide-chain-indexer
    app.kubernetes.io/part-of: tide
automountServiceAccountToken: true
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: tide-indexer-cursor
  namespace: tide-system
  labels:
    app.kubernetes.io/part-of: tide
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "create", "update"]
    resourceNames: ["tide-event-cursor"]
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: tide-indexer-cursor
  namespace: tide-system
  labels:
    app.kubernetes.io/part-of: tide
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: tide-indexer-cursor
subjects:
  - kind: ServiceAccount
    name: tide-indexer
    namespace: tide-system
```

### 3.3 IRSA Policy

Each agent's IAM role is scoped to its own KMS key and its own secrets path. No cross-agent access.

**IAM Policy for `tide-agent-{name}-irsa` (template -- substitute canonical agent name and key ARN):**

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "KMSSigning",
      "Effect": "Allow",
      "Action": [
        "kms:Sign",
        "kms:GetPublicKey"
      ],
      "Resource": "arn:aws:kms:us-east-1:ACCOUNT_ID:key/{AGENT_KEY_ID}"
    },
    {
      "Sid": "SecretsManagerRead",
      "Effect": "Allow",
      "Action": [
        "secretsmanager:GetSecretValue",
        "secretsmanager:DescribeSecret"
      ],
      "Resource": [
        "arn:aws:secretsmanager:us-east-1:ACCOUNT_ID:secret:tide/agents/{name}/*",
        "arn:aws:secretsmanager:us-east-1:ACCOUNT_ID:secret:tide/config/*"
      ]
    },
    {
      "Sid": "DenyIMDS",
      "Effect": "Deny",
      "Action": "*",
      "Resource": "*",
      "Condition": {
        "StringEquals": {
          "aws:SourceVpc": ""
        },
        "IpAddress": {
          "aws:SourceIp": "169.254.169.254/32"
        }
      }
    }
  ]
}
```

**IAM Trust Policy (IRSA):**

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::ACCOUNT_ID:oidc-provider/oidc.eks.us-east-1.amazonaws.com/id/CLUSTER_OIDC_ID"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "oidc.eks.us-east-1.amazonaws.com/id/CLUSTER_OIDC_ID:sub": "system:serviceaccount:tide-agents:tide-agent-{name}",
          "oidc.eks.us-east-1.amazonaws.com/id/CLUSTER_OIDC_ID:aud": "sts.amazonaws.com"
        }
      }
    }
  ]
}
```

**Per-agent scoping:**

| Agent | KMS Key Resource | Secrets Path |
|-------|-----------------|--------------|
| blockchain-dev | `arn:aws:kms:us-east-1:ACCOUNT_ID:key/{KEY_ID}` | `tide/agents/blockchain-dev/*` |
| k8s-specialist | `arn:aws:kms:us-east-1:ACCOUNT_ID:key/{KEY_ID}` | `tide/agents/k8s-specialist/*` |
| platform-eng | `arn:aws:kms:us-east-1:ACCOUNT_ID:key/{KEY_ID}` | `tide/agents/platform-eng/*` |
| coordinator | `arn:aws:kms:us-east-1:ACCOUNT_ID:key/{KEY_ID}` | `tide/agents/coordinator/*` |
| reviewer | `arn:aws:kms:us-east-1:ACCOUNT_ID:key/{KEY_ID}` | `tide/agents/reviewer/*` |

All agents also have read access to `tide/config/*` for shared ABIs and configuration.

### 3.4 SecretProviderClass (Finding #6 — previously missing)

Each agent needs a `SecretProviderClass` that maps AWS Secrets Manager paths to mounted files at `/secrets`. The file names must match the interface registry `volume_mounts.secrets.files`.

```yaml
# Template -- one per agent. Replace {name} with canonical agent name.
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: tide-agent-{name}-secrets
  namespace: tide-agents
spec:
  provider: aws
  parameters:
    objects: |
      - objectName: "tide/agents/{name}/github-app-key"
        objectType: "secretsmanager"
        objectAlias: "github-app-key.pem"
      - objectName: "tide/config/anthropic-api-key"
        objectType: "secretsmanager"
        objectAlias: "anthropic-api-key"
      - objectName: "tide/config/agent-system-prompt"
        objectType: "secretsmanager"
        objectAlias: "agent-system-prompt.txt"
      - objectName: "tide/config/agent-execution-system-prompt"
        objectType: "secretsmanager"
        objectAlias: "agent-execution-system-prompt.txt"
      - objectName: "tide/config/tide-council-abi"
        objectType: "secretsmanager"
        objectAlias: "tide-council-abi.json"
      - objectName: "tide/config/acp-abi"
        objectType: "secretsmanager"
        objectAlias: "acp-abi.json"
```

The `objectAlias` fields produce file names matching the interface registry: `github-app-key.pem`, `anthropic-api-key`, `agent-system-prompt.txt`, etc.

Create one SecretProviderClass per agent:
- `tide-agent-blockchain-dev-secrets`
- `tide-agent-k8s-specialist-secrets`
- `tide-agent-platform-eng-secrets`
- `tide-agent-coordinator-secrets`
- `tide-agent-reviewer-secrets`

### 3.5 Per-Agent ConfigMap (Finding #8 — previously missing)

Each agent needs a ConfigMap with its identity-specific values. These are referenced by `configMapKeyRef` in Tekton TaskRun specs.

```yaml
# Template -- one per agent. Populated from deployments/arctic-1-agents.json after contract deployment.
apiVersion: v1
kind: ConfigMap
metadata:
  name: tide-agent-{name}-config
  namespace: tide-agents
data:
  agent-token-id: "{token_id}"           # ERC-8004 NFT token ID (e.g., "1")
  agent-name: "{name}"                    # Canonical short name (e.g., "blockchain-dev")
  kms-key-id: "{kms_key_arn}"            # AWS KMS key ARN for EIP-712 signing
  provider-address: "{eth_address}"       # Agent's Sei wallet address (0x-prefixed)
```

Example for blockchain-dev:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tide-agent-blockchain-dev-config
  namespace: tide-agents
data:
  agent-token-id: "1"
  agent-name: "blockchain-dev"
  kms-key-id: "arn:aws:kms:us-east-1:ACCOUNT:key/KEY-ID-HERE"
  provider-address: "0x1234...abcd"
```

### 3.6 NetworkPolicy

Reuses the design from `lld-k8s-manifests.md` without changes. Agent pods can only reach external HTTPS endpoints. All private ranges and IMDS are blocked.

```yaml
# Default deny all traffic in tide-agents namespace
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: tide-agents
  labels:
    app.kubernetes.io/part-of: tide
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
---
# Allow agent egress to external HTTPS services only
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: agent-egress-allow
  namespace: tide-agents
  labels:
    app.kubernetes.io/part-of: tide
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/component: agent
  policyTypes:
    - Egress
  egress:
    # DNS resolution via kube-dns
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
      ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53
    # HTTPS egress to external services only
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
            except:
              - 169.254.169.254/32   # EC2 IMDS -- blocked
              - 10.0.0.0/8           # VPC internal -- blocked
              - 172.16.0.0/12        # Private ranges -- blocked
              - 192.168.0.0/16       # Private ranges -- blocked
      ports:
        - protocol: TCP
          port: 443
```

**What agents CAN reach (port 443 only):**

| Destination | Purpose |
|-------------|---------|
| `api.github.com` | Git clone, push, PR creation |
| `github.com` | Git clone over HTTPS |
| `api.anthropic.com` | Claude API calls |
| `evm-rpc-arctic-1.sei-apis.com` | Sei RPC for on-chain transactions |
| `kms.us-east-1.amazonaws.com` | AWS KMS for signing |
| `sts.us-east-1.amazonaws.com` | AWS STS for IRSA token exchange |
| `secretsmanager.us-east-1.amazonaws.com` | AWS Secrets Manager (via CSI driver, not direct) |

**What agents CANNOT reach:**

| Blocked | Reason |
|---------|--------|
| `169.254.169.254` (IMDS) | Prevents credential theft of node IAM role |
| `10.0.0.0/8` (VPC internal) | Prevents lateral movement to other pods/services |
| Any non-443 port | No SSH, HTTP, or other protocols |

---

## 4. Tekton to Indexer Event Contract

### 4.1 Common Envelope

Every event POST to the Tekton EventListener follows this JSON schema:

```json
{
  "event_type": "string",
  "block_number": 12345,
  "tx_hash": "0x...",
  "log_index": 3,
  "tx_index": 7,
  "contract": "0x...",
  "timestamp": "2026-04-02T12:00:00Z",
  "data": {}
}
```

| Field | Type | Description |
|-------|------|-------------|
| `event_type` | `string` | One of: `proposal_created`, `review_submitted`, `proposal_approved`, `proposal_rejected`, `proposal_expired`, `sandbox_provision_requested` |
| `block_number` | `integer` | Block number containing the event |
| `tx_hash` | `string` | Transaction hash (0x-prefixed, 66 chars) |
| `log_index` | `integer` | Log index within the transaction |
| `tx_index` | `integer` | Transaction index within the block |
| `contract` | `string` | Contract address that emitted the event (0x-prefixed, 42 chars) |
| `timestamp` | `string` | ISO 8601 timestamp of when the indexer processed this event |
| `data` | `object` | Decoded event fields (schema varies by event type) |

### 4.2 Per-Event Data Schemas

#### `proposal_created`

```json
{
  "event_type": "proposal_created",
  "block_number": 5000100,
  "tx_hash": "0xabc123...def456",
  "log_index": 0,
  "tx_index": 3,
  "contract": "0x1234...abcd",
  "timestamp": "2026-04-02T12:00:00Z",
  "data": {
    "proposalId": "7",
    "principal": "0xdead...beef",
    "designHash": "0x9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
    "parentProposalId": "0",
    "participantTokenIds": ["1", "2", "3"],
    "quorum": "2",
    "expiresAt": "1743580800"
  }
}
```

Field mapping from interface registry `events.proposal_created.params`:

| Data Field | ABI Type | Indexed | Notes |
|-----------|----------|---------|-------|
| `proposalId` | `uint256` | Yes (topics[1]) | Decimal string |
| `principal` | `address` | Yes (topics[2]) | 0x-prefixed checksum address |
| `designHash` | `bytes32` | Yes (topics[3]) | 0x-prefixed hex |
| `parentProposalId` | `uint256` | No (data) | Decimal string. `"0"` for initial proposals. |
| `participantTokenIds` | `uint256[]` | No (data) | Array of decimal strings |
| `quorum` | `uint8` | No (data) | Decimal string |
| `expiresAt` | `uint40` | No (data) | Unix timestamp as decimal string |

#### `review_submitted`

```json
{
  "event_type": "review_submitted",
  "block_number": 5000200,
  "tx_hash": "0x789...012",
  "log_index": 1,
  "tx_index": 5,
  "contract": "0x1234...abcd",
  "timestamp": "2026-04-02T12:30:00Z",
  "data": {
    "proposalId": "7",
    "agentTokenId": "1",
    "verdict": "0",
    "feedbackHash": "0xfeedback..."
  }
}
```

| Data Field | ABI Type | Indexed | Notes |
|-----------|----------|---------|-------|
| `proposalId` | `uint256` | Yes (topics[1]) | Decimal string |
| `agentTokenId` | `uint256` | Yes (topics[2]) | Decimal string |
| `verdict` | `uint8` | No (data) | `"0"` = Approve, `"1"` = RequestChanges |
| `feedbackHash` | `bytes32` | No (data) | 0x-prefixed hex |

#### `proposal_approved`

```json
{
  "event_type": "proposal_approved",
  "block_number": 5000300,
  "tx_hash": "0xdef...456",
  "log_index": 0,
  "tx_index": 2,
  "contract": "0x1234...abcd",
  "timestamp": "2026-04-02T13:00:00Z",
  "data": {
    "proposalId": "7",
    "designHash": "0x9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
  }
}
```

| Data Field | ABI Type | Indexed | Notes |
|-----------|----------|---------|-------|
| `proposalId` | `uint256` | Yes (topics[1]) | Decimal string |
| `designHash` | `bytes32` | Yes (topics[2]) | 0x-prefixed hex. Both fields are indexed (NOT in data section). |

#### `proposal_rejected`

```json
{
  "event_type": "proposal_rejected",
  "block_number": 5000400,
  "tx_hash": "0x111...222",
  "log_index": 0,
  "tx_index": 1,
  "contract": "0x1234...abcd",
  "timestamp": "2026-04-02T14:00:00Z",
  "data": {
    "proposalId": "7",
    "designHash": "0x9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
  }
}
```

| Data Field | ABI Type | Indexed | Notes |
|-----------|----------|---------|-------|
| `proposalId` | `uint256` | Yes (topics[1]) | Decimal string |
| `designHash` | `bytes32` | Yes (topics[2]) | 0x-prefixed hex |

#### `proposal_expired`

```json
{
  "event_type": "proposal_expired",
  "block_number": 5000500,
  "tx_hash": "0x333...444",
  "log_index": 0,
  "tx_index": 0,
  "contract": "0x1234...abcd",
  "timestamp": "2026-04-02T15:00:00Z",
  "data": {
    "proposalId": "7"
  }
}
```

| Data Field | ABI Type | Indexed | Notes |
|-----------|----------|---------|-------|
| `proposalId` | `uint256` | Yes (topics[1]) | Decimal string |

#### `sandbox_provision_requested`

```json
{
  "event_type": "sandbox_provision_requested",
  "block_number": 5000600,
  "tx_hash": "0x555...666",
  "log_index": 0,
  "tx_index": 4,
  "contract": "0x5678...efgh",
  "timestamp": "2026-04-02T16:00:00Z",
  "data": {
    "jobId": "42",
    "provider": "0xaaaa...bbbb",
    "client": "0xcccc...dddd",
    "agentTokenId": "1",
    "budget": "36000000",
    "expiry": "1743667200"
  }
}
```

| Data Field | ABI Type | Indexed | Notes |
|-----------|----------|---------|-------|
| `jobId` | `uint256` | Yes (topics[1]) | Decimal string |
| `provider` | `address` | Yes (topics[2]) | 0x-prefixed checksum address |
| `client` | `address` | Yes (topics[3]) | 0x-prefixed checksum address |
| `agentTokenId` | `uint256` | No (data) | Decimal string |
| `budget` | `uint256` | No (data) | Raw USDC amount (6 decimals) as decimal string |
| `expiry` | `uint256` | No (data) | Unix timestamp as decimal string |

### 4.3 Tekton TriggerBinding Example

The platform engineer's TriggerBinding extracts fields from this JSON. Here is the expected binding pattern:

```yaml
apiVersion: triggers.tekton.dev/v1beta1
kind: TriggerBinding
metadata:
  name: tide-proposal-created
  namespace: tide-system
spec:
  params:
    - name: event_type
      value: $(body.event_type)
    - name: proposal_id
      value: $(body.data.proposalId)
    - name: design_hash
      value: $(body.data.designHash)
    - name: principal
      value: $(body.data.principal)
    - name: participant_token_ids
      value: $(body.data.participantTokenIds)
    - name: quorum
      value: $(body.data.quorum)
    - name: expires_at
      value: $(body.data.expiresAt)
    - name: block_number
      value: $(body.block_number)
    - name: tx_hash
      value: $(body.tx_hash)
    - name: contract
      value: $(body.contract)
```

### 4.4 Idempotent TaskRun Naming

The TriggerTemplate generates deterministic TaskRun names from on-chain identifiers:

| Event Type | TaskRun Name Pattern | Example |
|-----------|---------------------|---------|
| `proposal_created` | `review-p{proposalId}-agent-{tokenId}` | `review-p7-agent-1` |
| `review_submitted` | (no TaskRun -- this is informational, pipeline may post status update) | -- |
| `proposal_approved` | (no TaskRun -- terminal event, pipeline may archive) | -- |
| `proposal_rejected` | (no TaskRun -- terminal event) | -- |
| `proposal_expired` | (no TaskRun -- terminal event) | -- |
| `sandbox_provision_requested` | `exec-j{jobId}-agent-{tokenId}` | `exec-j42-agent-1` |

For `proposal_created`, the TriggerTemplate fans out one TaskRun per participant agent. The TriggerTemplate iterates over `participantTokenIds` using a Tekton `custom` interceptor or a simple CEL filter per agent.

---

## 5. Operational Concerns

### 5.1 Deployment

**Recommended: Kustomize overlay, consistent with existing manifests.**

Directory structure:

```
manifests/mvp/
  base/
    kustomization.yaml
    indexer-deployment.yaml    # Section 1.9 Deployment
    indexer-rbac.yaml          # Section 3.2 ServiceAccount + Role + RoleBinding
    indexer-configmap.yaml     # Contract ABIs
    platform-config.yaml       # RPC URL, contract addresses, start block
  overlays/
    testnet/
      kustomization.yaml
      platform-config-patch.yaml  # arctic-1 specific values
```

**`base/kustomization.yaml`:**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: tide-system
resources:
  - indexer-deployment.yaml
  - indexer-rbac.yaml
  - indexer-configmap.yaml
  - platform-config.yaml
commonLabels:
  app.kubernetes.io/part-of: tide
```

**`overlays/testnet/platform-config-patch.yaml`:**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tide-platform-config
  namespace: tide-system
data:
  SEI_RPC_URL: "https://evm-rpc-arctic-1.sei-apis.com"
  SEI_CHAIN_ID: "713715"
  COUNCIL_ADDRESS: "0xTODO_AFTER_DEPLOYMENT"
  JOB_HOOK_ADDRESS: "0xTODO_AFTER_DEPLOYMENT"
  INDEXER_START_BLOCK: "0"
```

### 5.2 Monitoring

**Structured logging (JSON to stdout):**

The indexer uses structured JSON logging. Key log lines:

| Log Message | Level | Fields | Meaning |
|-------------|-------|--------|---------|
| `"processing blocks"` | INFO | `from_block`, `to_block`, `latest_block` | Batch processing start |
| `"posted {event_type}"` | INFO | `event_type`, `block_number`, `tx_hash` | Successful delivery to Tekton |
| `"tekton_post_failed"` | ERROR | `status`, `event_type` | Failed delivery |
| `"eth_getLogs failed"` | ERROR | `from_block`, `to_block`, `error` | RPC failure |
| `"done"` | INFO | `events_processed`, `cursor_block` | Run complete |

**Prometheus metrics (if running as Deployment):**

In daemon mode, the indexer can expose a `/metrics` endpoint on port 8080:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `tide_indexer_events_total` | Counter | `event_type` | Total events processed |
| `tide_indexer_errors_total` | Counter | `error_type` | Processing errors (rpc_error, decode_error, tekton_post_error) |
| `tide_indexer_lag_blocks` | Gauge | -- | Blocks behind chain HEAD |
| `tide_indexer_last_processed_block` | Gauge | -- | Last successfully processed block |
| `tide_indexer_poll_duration_seconds` | Histogram | -- | Time per poll cycle |

**Alerts:**

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: tide-indexer-alerts
  namespace: tide-system
spec:
  groups:
    - name: tide-indexer
      rules:
        - alert: TideIndexerDown
          expr: absent(up{job="tide-chain-indexer", namespace="tide-system"} == 1)
          for: 5m
          labels:
            severity: critical
          annotations:
            summary: "Tide chain indexer is not running"

        - alert: TideIndexerLagHigh
          expr: tide_indexer_lag_blocks > 100
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "Tide indexer is {{ $value }} blocks behind chain HEAD"

        - alert: TideIndexerPostFailures
          expr: increase(tide_indexer_errors_total{error_type="tekton_post_error"}[15m]) > 3
          labels:
            severity: critical
          annotations:
            summary: "Tide indexer failing to deliver events to Tekton"
```

### 5.3 Cursor Reset Procedure

On testnet, contracts are redeployed frequently. When contracts are redeployed to new addresses:

1. Update `tide-platform-config` ConfigMap with new contract addresses and the deployment block number for `INDEXER_START_BLOCK`.
2. Delete the cursor ConfigMap to force a fresh start:
   ```bash
   kubectl delete configmap tide-event-cursor -n tide-system
   ```
3. Restart the indexer Deployment:
   ```bash
   kubectl rollout restart deployment/tide-chain-indexer -n tide-system
   ```
4. The indexer starts from `INDEXER_START_BLOCK` and reprocesses all events. Since Tekton TaskRun names are deterministic, any already-completed TaskRuns are skipped (AlreadyExists).

**Partial reset (same contract, replay from specific block):**

```bash
kubectl patch configmap tide-event-cursor -n tide-system --type merge \
  -p '{"data":{"lastProcessedBlock":"5000000","lastProcessedTxIndex":"0","lastProcessedLogIndex":"0"}}'
kubectl rollout restart deployment/tide-chain-indexer -n tide-system
```

### 5.4 Indexer Container Image

```dockerfile
FROM python:3.12-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    && rm -rf /var/lib/apt/lists/*

COPY requirements-indexer.txt /app/requirements.txt
RUN pip install --no-cache-dir -r /app/requirements.txt

COPY indexer.py /app/indexer.py

RUN groupadd -r tide && useradd -r -g tide tide
USER tide

ENTRYPOINT ["python3", "/app/indexer.py"]
```

**`requirements-indexer.txt`:**

```
web3>=7.0.0
requests>=2.31.0
kubernetes>=28.0.0
```

---

## Error Handling

### Indexer Errors

| Error | Detection | Surfaced As | Operator Action |
|-------|-----------|-------------|-----------------|
| RPC connection failure | `requests.ConnectionError` from web3 | Exit code 1, error log | Kubernetes restarts pod (Deployment). Next poll cycle retries from cursor position. |
| RPC rate limiting | HTTP 429 from RPC | Retry with backoff, then exit 1 | Increase `POLL_INTERVAL` or switch RPC provider. |
| ABI decode failure | `web3.exceptions.LogTopicError` | Error log, skip event | Investigate ABI mismatch. Likely contract upgrade. |
| Tekton EventListener unreachable | HTTP connection refused or 5xx | Retry 3x, then exit 1 | Check Tekton EventListener health. Cursor is not advanced. |
| ConfigMap write failure | K8s API error | Exit code 1, error log | Check RBAC. Event was already delivered to Tekton (at-least-once). |
| Unknown event topic | Topic not in `TOPIC_TO_EVENT` map | Skip silently | Expected for events from future contract versions. |

### Agent Container Errors

All exit codes and their handling are defined in the interface registry `exit_codes` section. The Tekton pipeline's `finally` task reads the TaskRun status and termination message to determine the outcome.

---

## Test Specification

### Indexer Tests

| Test | Setup | Action | Expected |
|------|-------|--------|----------|
| `test_cursor_cold_start` | No cursor ConfigMap exists | Run indexer | Starts from `INDEXER_START_BLOCK`, creates cursor ConfigMap |
| `test_cursor_resume` | Cursor at block 100 | Run indexer with chain at block 200 | Processes blocks 101-200 only |
| `test_proposal_created_decode` | Mock RPC returns a `ProposalCreated` log | Decode event | All fields match registry schema |
| `test_sandbox_provision_decode` | Mock RPC returns `SandboxProvisionRequested` log | Decode event | `jobId`, `provider`, `client`, `agentTokenId`, `budget`, `expiry` correct |
| `test_tekton_post_success` | Mock Tekton listener returns 202 | Post event | Returns True, cursor advanced |
| `test_tekton_post_retry` | Mock listener returns 500 twice, then 200 | Post event | Retries with backoff, succeeds on 3rd attempt |
| `test_tekton_post_failure` | Mock listener returns 500 3 times | Post event | Returns False, cursor NOT advanced |
| `test_duplicate_handling` | Process same block range twice | Run indexer | Second run skips already-processed logs (cursor check) |
| `test_batch_pagination` | 2500 blocks to process, batch size 1000 | Run indexer | Makes 3 `eth_getLogs` calls (1000, 1000, 500) |
| `test_all_event_types` | Mock RPC with one of each event type | Decode all | All 6 event types decoded correctly, POSTed with correct `event_type` field |

### Agent Container Tests

| Test | Setup | Action | Expected |
|------|-------|--------|----------|
| `test_review_mode_dispatch` | `TIDE_RUNTIME_MODE=review` | Run entrypoint | Imports and calls `run_review()` |
| `test_execution_mode_dispatch` | `TIDE_RUNTIME_MODE=execution` | Run entrypoint | Imports and calls `run_execution()` |
| `test_invalid_mode` | `TIDE_RUNTIME_MODE=invalid` | Run entrypoint | Exits with code 10 |
| `test_missing_mode` | `TIDE_RUNTIME_MODE` unset | Run entrypoint | Exits with code 10 |
| `test_init_container_token` | Valid GitHub App key at `/secrets/github-app-key.pem` | Run init script | Token written to `/workspace/.tide/github-token` |
| `test_init_container_clone` | Valid token, proposals repo accessible | Run init script | Repo cloned to `/workspace/proposals` |
| `test_termination_message` | Agent writes result JSON | Read `/dev/termination-log` | Valid `AgentResult` JSON under 4096 bytes |

---

## Deployment

### Testnet (arctic-1) -- MVP Target

| Parameter | Value |
|-----------|-------|
| Sei RPC URL | `https://evm-rpc-arctic-1.sei-apis.com` |
| Chain ID | `713715` |
| TideCouncil address | Set after deployment |
| TideJobHook address | Set after deployment |
| Indexer poll interval | 15 seconds |
| Indexer batch size | 1000 blocks |
| Indexer namespace | `tide-system` |
| Agent namespace | `tide-agents` |
| Agent images | ECR registry, tagged per release |

### Build Pipeline

1. **Indexer image:** `docker build -f Dockerfile.indexer -t REGISTRY/tide-chain-indexer:TAG .`
2. **Agent base image:** `docker build -f Dockerfile.agent -t REGISTRY/tide-agent:TAG .`
3. **Init image:** `docker build -f Dockerfile.init -t REGISTRY/tide-workspace-init:TAG .`
4. Push to ECR, deploy via `kustomize build manifests/mvp/overlays/testnet | kubectl apply -f -`.

---

## Deferred (Do Not Build)

| Feature | Rationale |
|---------|-----------|
| WebSocket subscription (`eth_subscribe`) | Polling at 15s is sufficient for MVP. WebSocket adds reconnection complexity. |
| Full Go controller-runtime operator | MVP uses Tekton for workload lifecycle. The Go operator is Phase 1+ scope. |
| CRD state machines (TideProposal, TideJob) | Tekton TaskRuns replace CRD-based lifecycle management for MVP. |
| Leader election | Single-replica Deployment with `Recreate` strategy is sufficient. |
| Chain reorg detection | arctic-1 testnet has negligible reorg risk. Add if needed for mainnet. |
| Prometheus metrics endpoint | MVP uses structured logs. Metrics can be added when Prometheus Operator is deployed. |
| Multi-contract ABI routing | MVP has exactly 2 contracts. Dynamic ABI loading is unnecessary. |
| ERC-8183 ACP terminal events (`JobCompleted`, `JobRejected`, `RefundClaimed`) | MVP Phase 1 focuses on proposal review. Execution pipeline events deferred to Phase 2. |

---

## Decision Log

| # | Decision | Rationale | Reversibility |
|---|----------|-----------|---------------|
| 1 | Python CronJob/Deployment over Go binary for indexer | Faster to build, `web3.py` handles ABI decoding natively, no controller-runtime overhead | Two-way door. Can replace with Go binary when moving to full operator. |
| 2 | Deployment with poll loop over CronJob | Sub-minute polling not supported by CronJob. Deployment is simpler. | Two-way door. |
| 3 | ConfigMap for cursor persistence (not PVC) | Consistent with `lld-tide-operator.md` cursor design. ConfigMap is simpler than PVC for a single key-value pair. | Two-way door. |
| 4 | HTTP polling only, no WebSocket | arctic-1 testnet latency tolerance is high. WebSocket adds reconnection complexity not justified for MVP. | Two-way door. Add WebSocket later. |
| 5 | Event signatures computed at runtime from string constants | Avoids hardcoding hex topic hashes that could become stale if signatures change. Signatures are copied verbatim from interface registry. | Two-way door. |
| 6 | Per-agent ServiceAccounts (not shared) | Consistent with `lld-k8s-manifests.md` decision. Limits blast radius of compromised agent. | Two-way door, but changing SA names is operationally disruptive. |
