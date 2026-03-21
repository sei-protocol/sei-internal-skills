# Component: Tide Operator

**Date:** 2026-03-21
**Status:** Draft

---

## Owner

Kubernetes Specialist (platform team)

## Phase

0.7 – 2. The operator binary ships incrementally:

| Phase | Capability |
|-------|-----------|
| 0.7 | Binary skeleton, CRD installation, leader election, health probes |
| 1 | Event indexer (TideCouncil events), TideProposal controller (creates review Jobs) |
| 2 | Event indexer (TideJobHook + ACP events), TideJob controller (sandbox provisioning, agent execution Jobs, deliverable submission, cleanup) |

## Purpose

The Tide Operator is a single Go binary running as a Kubernetes Deployment (2 replicas, leader election) in the `tide-system` namespace. It bridges on-chain state on Sei EVM to Kubernetes-native resources, enabling the full proposal review and funded job execution lifecycle without manual intervention.

It serves business needs 1–5: design review consensus (via TideProposal CRDs and review agent Jobs), funded job execution (via TideJob CRDs, sandbox provisioning, and agent execution Jobs), and deliverable attestation (via KMS-signed on-chain submission).

## Dependencies

### External Systems Consumed

| System | Interface | Notes |
|--------|-----------|-------|
| Sei EVM RPC | JSON-RPC 2.0 over WebSocket + HTTP | `eth_subscribe` for live events, `eth_getLogs` for backfill/fallback |
| TideCouncil contract | Solidity ABI (event signatures defined below) | Source of truth for proposal lifecycle events |
| TideJobHook contract | Solidity ABI (event signatures defined below) | Emits `SandboxProvisionRequested` on `afterAction(fund)` |
| ERC-8183 ACP contract | Solidity ABI (event signatures defined below) | Emits terminal job events (`JobCompleted`, `JobRejected`) |
| GitHub API v3 | REST, authenticated via GitHub App JWTs | Repository CRUD, installation token generation |
| AWS KMS | `kms:Sign`, `kms:GetPublicKey` via AWS SDK | Agent transaction signing (secp256k1) |
| AWS Secrets Manager | Via CSI SecretProviderClass | GitHub App private keys mounted into agent Jobs |

### Internal Tide Components Consumed

| Component | Interface | Reference |
|-----------|-----------|-----------|
| Agent Review Runtime | Container image, consumes env vars + volumes defined in §Agent Runtime Interface | `lld-agent-review-runtime.md` |
| Agent Execution Runtime | Container image, consumes env vars + volumes defined in §Agent Runtime Interface | `lld-agent-execution-runtime.md` |
| K8s Platform Manifests | Namespaces, RBAC, NetworkPolicies, ResourceQuotas | `lld-k8s-manifests.md` |

### Explicit Exclusions

- Does NOT depend on any dashboard, CLI, or UI component.
- Does NOT depend on ERC-8004 Reputation Registry reads (reputation gating is in TideJobHook, not the operator).
- Does NOT depend on any LLM provider directly — LLM calls happen inside agent runtime containers, not the operator.

---

## Interface Specification

### Go Module

```
module github.com/sei-tide/tide-operator
```

API group: `tide.sei.io`
API version: `v1alpha1`

---

### CRD Types

#### Group/Version Registration

```go
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
)

var (
    GroupVersion = schema.GroupVersion{Group: "tide.sei.io", Version: "v1alpha1"}
)
```

#### TideProposal

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=tp
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Proposal",type=string,JSONPath=`.spec.proposalID`
// +kubebuilder:printcolumn:name="Approvals",type=integer,JSONPath=`.status.approvalCount`
// +kubebuilder:printcolumn:name="Quorum",type=integer,JSONPath=`.spec.quorum`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type TideProposal struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   TideProposalSpec   `json:"spec"`
    Status TideProposalStatus `json:"status,omitempty"`
}

// TideProposalSpec is immutable after creation. All fields are set by the
// event indexer from the on-chain ProposalCreated event.
type TideProposalSpec struct {
    // ProposalID is the on-chain proposal identifier from TideCouncil
    // (uint256 encoded as a decimal string to avoid JSON integer overflow).
    // +kubebuilder:validation:Pattern=`^\d+$`
    ProposalID string `json:"proposalID"`

    // DesignHash is the keccak256 of the design document.
    // +kubebuilder:validation:Pattern=`^0x[0-9a-fA-F]{64}$`
    DesignHash string `json:"designHash"`

    // ParentProposalID links to a previous revision. Empty string if this
    // is the first submission in a review cycle.
    // +kubebuilder:validation:Pattern=`^(0x[0-9a-fA-F]{64})?$`
    // +optional
    ParentProposalID string `json:"parentProposalID,omitempty"`

    // Principal is the Sei address of the proposer.
    // +kubebuilder:validation:Pattern=`^0x[0-9a-fA-F]{40}$`
    Principal string `json:"principal"`

    // Participants lists the ERC-8004 token IDs of agents eligible to review.
    // +kubebuilder:validation:MinItems=1
    Participants []uint64 `json:"participants"`

    // Quorum is the number of "approve" verdicts required for consensus.
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:validation:Maximum=255
    Quorum int32 `json:"quorum"`

    // ExpiresAt is the on-chain proposal TTL deadline.
    ExpiresAt metav1.Time `json:"expiresAt"`

    // CouncilContract is the TideCouncil contract address on Sei.
    // +kubebuilder:validation:Pattern=`^0x[0-9a-fA-F]{40}$`
    CouncilContract string `json:"councilContract"`

    // SourceBlock is the block number containing the ProposalCreated event.
    SourceBlock uint64 `json:"sourceBlock"`

    // SourceTxHash is the transaction hash of the ProposalCreated event.
    // +kubebuilder:validation:Pattern=`^0x[0-9a-fA-F]{64}$`
    SourceTxHash string `json:"sourceTxHash"`
}

type TideProposalStatus struct {
    // Phase is the high-level lifecycle state.
    // +kubebuilder:validation:Enum=Pending;Active;Approved;Rejected;Expired
    Phase ProposalPhase `json:"phase,omitempty"`

    // Reviews holds the latest review status per participant agent.
    // +optional
    Reviews []ReviewStatus `json:"reviews,omitempty"`

    // ApprovalCount is the current number of "approve" verdicts.
    ApprovalCount int32 `json:"approvalCount,omitempty"`

    // ReviewJobRefs tracks the K8s Jobs launched for each reviewer agent.
    // +optional
    ReviewJobRefs []JobReference `json:"reviewJobRefs,omitempty"`

    // Conditions provide granular status signals.
    // +optional
    Conditions []metav1.Condition `json:"conditions,omitempty"`

    // ObservedGeneration is the .metadata.generation the controller last acted on.
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
type TideProposalList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []TideProposal `json:"items"`
}
```

**ProposalPhase values:**

```go
type ProposalPhase string

const (
    // ProposalPhasePending — CR created by indexer, review Jobs not yet launched.
    ProposalPhasePending ProposalPhase = "Pending"

    // ProposalPhaseActive — review Jobs launched, waiting for agent reviews.
    ProposalPhaseActive ProposalPhase = "Active"

    // ProposalPhaseApproved — quorum approvals reached and finalized on-chain.
    ProposalPhaseApproved ProposalPhase = "Approved"

    // ProposalPhaseRejected — proposal explicitly rejected (not enough approvals
    // before expiry, or revoked by admin).
    ProposalPhaseRejected ProposalPhase = "Rejected"

    // ProposalPhaseExpired — on-chain TTL elapsed without finalization.
    ProposalPhaseExpired ProposalPhase = "Expired"
)
```

**Condition types for TideProposal:**

| Condition Type | Meaning when True |
|---------------|-------------------|
| `ReviewJobsLaunched` | All participant review K8s Jobs have been created |
| `QuorumReached` | `approvalCount >= spec.quorum` |
| `OnChainFinalized` | On-chain proposal status is Approved or Expired |

#### TideJob

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=tj
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Job",type=string,JSONPath=`.spec.jobID`
// +kubebuilder:printcolumn:name="Agent",type=integer,JSONPath=`.spec.agentTokenID`
// +kubebuilder:printcolumn:name="Budget",type=string,JSONPath=`.spec.budget`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type TideJob struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   TideJobSpec   `json:"spec"`
    Status TideJobStatus `json:"status,omitempty"`
}

// TideJobSpec is immutable after creation. All fields are set by the event
// indexer from the on-chain JobFunded / SandboxProvisionRequested events.
type TideJobSpec struct {
    // JobID is the on-chain ERC-8183 job identifier
    // (uint256 encoded as decimal string).
    // +kubebuilder:validation:Pattern=`^\d+$`
    JobID string `json:"jobID"`

    // ProposalRef is the metadata.name of the associated TideProposal CR.
    // Empty if the job was created without a linked proposal.
    // +optional
    ProposalRef string `json:"proposalRef,omitempty"`

    // DesignHash is the approved design hash this job implements.
    // +kubebuilder:validation:Pattern=`^0x[0-9a-fA-F]{64}$`
    DesignHash string `json:"designHash"`

    // AgentTokenID is the ERC-8004 identity token of the assigned agent.
    AgentTokenID uint64 `json:"agentTokenID"`

    // ProviderAddress is the agent's Sei wallet address.
    // +kubebuilder:validation:Pattern=`^0x[0-9a-fA-F]{40}$`
    ProviderAddress string `json:"providerAddress"`

    // ClientAddress is the principal/treasury wallet address.
    // +kubebuilder:validation:Pattern=`^0x[0-9a-fA-F]{40}$`
    ClientAddress string `json:"clientAddress"`

    // EvaluatorAddress is the evaluator address for this job.
    // +kubebuilder:validation:Pattern=`^0x[0-9a-fA-F]{40}$`
    EvaluatorAddress string `json:"evaluatorAddress"`

    // Budget is the USDC escrow amount in raw units (6 decimals).
    // Stored as decimal string to prevent integer overflow in JSON.
    // +kubebuilder:validation:Pattern=`^\d+$`
    Budget string `json:"budget"`

    // ExpiresAt is the on-chain job expiry timestamp.
    ExpiresAt metav1.Time `json:"expiresAt"`

    // ACPContract is the ERC-8183 AgenticCommerce contract address.
    // +kubebuilder:validation:Pattern=`^0x[0-9a-fA-F]{40}$`
    ACPContract string `json:"acpContract"`

    // SourceBlock is the block number containing the JobFunded event.
    SourceBlock uint64 `json:"sourceBlock"`

    // SourceTxHash is the transaction hash of the JobFunded event.
    // +kubebuilder:validation:Pattern=`^0x[0-9a-fA-F]{64}$`
    SourceTxHash string `json:"sourceTxHash"`
}

type TideJobStatus struct {
    // Phase is the high-level lifecycle state.
    // +kubebuilder:validation:Enum=Pending;Provisioning;Running;Submitting;Submitted;Completed;Rejected;Failed;Expired
    Phase JobPhase `json:"phase,omitempty"`

    // Sandbox tracks GitHub workspace provisioning state.
    // +optional
    Sandbox *SandboxStatus `json:"sandbox,omitempty"`

    // K8sJob tracks the batch/v1 Job created to run the agent.
    // +optional
    K8sJob *K8sJobReference `json:"k8sJob,omitempty"`

    // DeliverableHash is the deliverable identifier reported by the agent
    // (typically keccak256 of the commit SHA, 0x-prefixed).
    // +optional
    DeliverableHash string `json:"deliverableHash,omitempty"`

    // PRUrl is the GitHub pull request URL opened by the agent.
    // +optional
    PRUrl string `json:"prUrl,omitempty"`

    // SubmissionTxHash is the Sei transaction hash where the deliverable
    // was submitted on-chain via ERC-8183 submit().
    // +optional
    SubmissionTxHash string `json:"submissionTxHash,omitempty"`

    // TokenRefreshedAt is the last time the GitHub App installation
    // token was refreshed for this job's agent.
    // +optional
    TokenRefreshedAt *metav1.Time `json:"tokenRefreshedAt,omitempty"`

    // FailureReason provides a human-readable explanation when Phase=Failed.
    // +optional
    FailureReason string `json:"failureReason,omitempty"`

    // Conditions provide granular status signals.
    // +optional
    Conditions []metav1.Condition `json:"conditions,omitempty"`

    // ObservedGeneration is the .metadata.generation the controller last acted on.
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
type TideJobList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []TideJob `json:"items"`
}
```

**JobPhase values:**

```go
type JobPhase string

const (
    JobPhasePending      JobPhase = "Pending"      // CR created by indexer
    JobPhaseProvisioning JobPhase = "Provisioning"  // GitHub sandbox being set up
    JobPhaseRunning      JobPhase = "Running"       // K8s Job launched and active
    JobPhaseSubmitting   JobPhase = "Submitting"    // Agent succeeded, submitting deliverable on-chain
    JobPhaseSubmitted    JobPhase = "Submitted"     // On-chain submit() confirmed, awaiting evaluator
    JobPhaseCompleted    JobPhase = "Completed"     // Evaluator approved, USDC released (terminal)
    JobPhaseRejected     JobPhase = "Rejected"      // Evaluator rejected (terminal)
    JobPhaseFailed       JobPhase = "Failed"        // Infrastructure or agent failure (terminal)
    JobPhaseExpired      JobPhase = "Expired"       // On-chain expiry elapsed (terminal)
)

func (p JobPhase) IsTerminal() bool {
    switch p {
    case JobPhaseCompleted, JobPhaseRejected, JobPhaseFailed, JobPhaseExpired:
        return true
    }
    return false
}
```

**Condition types for TideJob:**

| Condition Type | Meaning when True |
|---------------|-------------------|
| `SandboxReady` | GitHub workspace repo exists and installation token is valid |
| `K8sJobCreated` | The batch/v1 Job has been created in `tide-agents` |
| `K8sJobSucceeded` | The batch/v1 Job completed with exit code 0 |
| `DeliverableSubmitted` | The deliverable hash has been submitted on-chain |
| `OnChainTerminal` | The on-chain job has reached a terminal state (completed/rejected/expired) |

#### Shared Types

```go
type ReviewStatus struct {
    // AgentTokenID is the ERC-8004 token of the reviewing agent.
    AgentTokenID uint64 `json:"agentTokenID"`

    // Verdict is the agent's review outcome.
    // +kubebuilder:validation:Enum=approve;request_changes;""
    Verdict string `json:"verdict,omitempty"`

    // FeedbackHash is the keccak256 of the structured review JSON stored on GitHub.
    // +optional
    FeedbackHash string `json:"feedbackHash,omitempty"`

    // SubmittedAt is when the on-chain attestation was recorded.
    // +optional
    SubmittedAt *metav1.Time `json:"submittedAt,omitempty"`
}

type JobReference struct {
    // AgentTokenID identifies which agent this Job runs for.
    AgentTokenID uint64 `json:"agentTokenID"`

    // Name is the batch/v1 Job metadata.name.
    Name string `json:"name"`

    // Namespace is the batch/v1 Job namespace (always "tide-agents").
    Namespace string `json:"namespace"`
}

type SandboxStatus struct {
    // RepositoryURL is the GitHub HTTPS URL of the workspace repo.
    // +optional
    RepositoryURL string `json:"repositoryURL,omitempty"`

    // Ready indicates the workspace repo exists and credentials are valid.
    Ready bool `json:"ready"`

    // ProvisionedAt is when the sandbox became ready.
    // +optional
    ProvisionedAt *metav1.Time `json:"provisionedAt,omitempty"`
}

type K8sJobReference struct {
    // Name is the batch/v1 Job metadata.name.
    Name string `json:"name"`

    // Namespace is the batch/v1 Job namespace.
    Namespace string `json:"namespace"`

    // Active is the number of actively running pods.
    Active int32 `json:"active,omitempty"`

    // Succeeded is the number of pods that completed successfully.
    Succeeded int32 `json:"succeeded,omitempty"`

    // Failed is the number of pods that failed.
    Failed int32 `json:"failed,omitempty"`

    // StartTime is when the Job was created.
    // +optional
    StartTime *metav1.Time `json:"startTime,omitempty"`
}
```

---

### Adapter Interfaces

All external system interactions are behind Go interfaces in `pkg/interfaces/`. Implementations live in `internal/adapter/`. Every method documents its error cases and retry guidance.

#### ChainClient

```go
package interfaces

import (
    "context"
    "math/big"

    "github.com/ethereum/go-ethereum"
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/core/types"
)

// ChainClient abstracts all Sei EVM interactions.
// Implementations must be safe for concurrent use.
type ChainClient interface {
    // LatestBlockNumber returns the current chain head block number.
    //
    // Errors:
    //   - ErrRPCUnavailable: RPC endpoint unreachable (retry with backoff)
    //   - context.DeadlineExceeded: call timed out (retry)
    LatestBlockNumber(ctx context.Context) (uint64, error)

    // GetLogs fetches historical event logs matching the filter.
    // Used for backfill and polling fallback.
    //
    // The caller is responsible for paginating large block ranges
    // (max 2000 blocks per call, enforced by most Sei RPC endpoints).
    //
    // Errors:
    //   - ErrRPCUnavailable: RPC endpoint unreachable (retry with backoff)
    //   - ErrBlockRangeExceeded: requested range too large (reduce range, retry)
    //   - context.DeadlineExceeded: call timed out (retry)
    GetLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error)

    // SubscribeFilterLogs opens a WebSocket subscription for live event logs.
    // Returns a subscription that delivers logs to ch. The subscription is
    // closed on error; the caller must resubscribe.
    //
    // Errors:
    //   - ErrWebSocketUnavailable: WS endpoint unreachable (fall back to polling)
    //   - context.Canceled: caller canceled
    SubscribeFilterLogs(ctx context.Context, query ethereum.FilterQuery, ch chan<- types.Log) (ethereum.Subscription, error)

    // GetProposalState reads the current on-chain state of a TideCouncil proposal.
    // Used as a reconciliation safety net to verify CRD state matches chain state.
    //
    // Errors:
    //   - ErrContractCall: ABI encoding/decoding failure (do not retry, likely bug)
    //   - ErrRPCUnavailable: RPC endpoint unreachable (retry with backoff)
    //   - ErrProposalNotFound: proposal ID does not exist on-chain (do not retry)
    GetProposalState(ctx context.Context, councilAddr common.Address, proposalID *big.Int) (*OnChainProposal, error)

    // GetJobState reads the current on-chain state of an ERC-8183 job.
    //
    // Errors:
    //   - ErrContractCall: ABI encoding/decoding failure (do not retry)
    //   - ErrRPCUnavailable: RPC endpoint unreachable (retry with backoff)
    //   - ErrJobNotFound: job ID does not exist on-chain (do not retry)
    GetJobState(ctx context.Context, acpAddr common.Address, jobID *big.Int) (*OnChainJob, error)

    // SubmitDeliverable signs (via KMS) and sends a submit(jobId, deliverableHash)
    // transaction to the ERC-8183 contract.
    //
    // The implementation must:
    //   1. Build the transaction calldata (ABI-encode submit(uint256,bytes32))
    //   2. Estimate gas
    //   3. Sign via KMSClient using the agent's key ARN
    //   4. Send the raw transaction
    //   5. Wait for 1 confirmation
    //
    // Errors:
    //   - ErrKMSSign: KMS signing failed (retry once, then fail)
    //   - ErrNonceConflict: nonce already used (re-fetch nonce, retry once)
    //   - ErrInsufficientGas: agent wallet lacks SEI for gas (fail, surface to operator)
    //   - ErrTxReverted: contract reverted (read revert reason, fail with reason)
    //   - ErrRPCUnavailable: RPC endpoint unreachable (retry with backoff)
    SubmitDeliverable(ctx context.Context, acpAddr common.Address, jobID *big.Int, deliverableHash [32]byte, agentKMSKeyARN string) (txHash common.Hash, err error)
}

// OnChainProposal represents the on-chain view of a TideCouncil proposal,
// read via eth_call.
type OnChainProposal struct {
    DesignHash       [32]byte
    ParentProposalID [32]byte
    Principal        common.Address
    CreatedAt        uint64
    ExpiresAt        uint64
    Quorum           uint8
    Status           uint8 // 0=Proposed, 1=Approved, 2=Rejected, 3=Expired
    Reviews          []OnChainReview
}

type OnChainReview struct {
    AgentTokenID uint64
    Verdict      uint8 // 0=Approve, 1=RequestChanges
    FeedbackHash [32]byte
    Timestamp    uint64
}

// OnChainJob represents the on-chain view of an ERC-8183 job.
type OnChainJob struct {
    Client    common.Address
    Provider  common.Address
    Evaluator common.Address
    Amount    *big.Int
    Status    uint8 // ERC-8183 job status enum
    ExpiresAt uint64
}
```

**ChainClient error sentinel values:**

```go
package interfaces

import "errors"

var (
    ErrRPCUnavailable     = errors.New("chain: RPC endpoint unavailable")
    ErrWebSocketUnavailable = errors.New("chain: WebSocket endpoint unavailable")
    ErrBlockRangeExceeded = errors.New("chain: block range too large")
    ErrContractCall       = errors.New("chain: contract call failed")
    ErrProposalNotFound   = errors.New("chain: proposal not found")
    ErrJobNotFound        = errors.New("chain: job not found")
    ErrKMSSign            = errors.New("chain: KMS signing failed")
    ErrNonceConflict      = errors.New("chain: nonce conflict")
    ErrInsufficientGas    = errors.New("chain: insufficient gas balance")
    ErrTxReverted         = errors.New("chain: transaction reverted")
)
```

#### GitHubClient

```go
package interfaces

import (
    "context"
    "time"
)

// GitHubClient abstracts GitHub API interactions for sandbox management.
// Implementations must be safe for concurrent use.
type GitHubClient interface {
    // EnsureRepository creates the workspace repo if it does not exist.
    // If it already exists, this is a no-op (idempotent).
    // The repo is created from the configured template repo.
    //
    // Errors:
    //   - ErrGitHubRateLimit: API rate limit exceeded (retry after Reset-At header)
    //   - ErrGitHubAuth: authentication failure (do not retry, check App key)
    //   - ErrGitHubAPI: other API error (retry with backoff, max 3 attempts)
    EnsureRepository(ctx context.Context, opts CreateRepoOpts) (*Repository, error)

    // GenerateInstallationToken creates a short-lived (1h) installation
    // access token scoped to the specified repositories.
    //
    // Errors:
    //   - ErrGitHubRateLimit: rate limit exceeded (retry after Reset-At)
    //   - ErrGitHubAuth: App private key invalid or expired (do not retry)
    //   - ErrInstallationSuspended: App installation suspended by org admin (do not retry)
    //   - ErrGitHubAPI: other API error (retry with backoff, max 3 attempts)
    GenerateInstallationToken(ctx context.Context, appID int64, installationID int64, repos []string) (*InstallationToken, error)

    // ArchiveRepository makes a repository read-only. Used for cleanup
    // when a job reaches a terminal state.
    //
    // Errors:
    //   - ErrGitHubRateLimit: rate limit exceeded (retry after Reset-At)
    //   - ErrGitHubAPI: other API error (retry with backoff, max 3 attempts)
    ArchiveRepository(ctx context.Context, owner string, repo string) error
}

type CreateRepoOpts struct {
    Org          string // GitHub organization (e.g., "sei-tide")
    Name         string // Repository name (e.g., "agent-alpha-job-42")
    TemplateRepo string // Template repo for scaffolding (e.g., "sei-tide/workspace-template")
    Private      bool
    Description  string
}

type Repository struct {
    FullName  string // "sei-tide/agent-alpha-job-42"
    CloneURL  string // "https://github.com/sei-tide/agent-alpha-job-42.git"
    CreatedAt time.Time
}

type InstallationToken struct {
    Token     string
    ExpiresAt time.Time
}

var (
    ErrGitHubRateLimit         = errors.New("github: rate limit exceeded")
    ErrGitHubAuth              = errors.New("github: authentication failed")
    ErrGitHubAPI               = errors.New("github: API error")
    ErrInstallationSuspended   = errors.New("github: installation suspended")
)
```

#### KMSClient

```go
package interfaces

import "context"

// KMSClient abstracts AWS KMS signing operations.
// Each agent has a dedicated secp256k1 key in KMS.
type KMSClient interface {
    // SignDigest signs a 32-byte digest using the specified KMS key.
    // Returns the DER-encoded ECDSA signature.
    //
    // The implementation must convert the DER signature to the
    // [R || S || V] format required by Ethereum before the caller
    // uses it for transaction signing.
    //
    // Errors:
    //   - ErrKMSKeyNotFound: key ARN does not exist (do not retry)
    //   - ErrKMSKeyDisabled: key is disabled (do not retry, agent revoked)
    //   - ErrKMSThrottled: API throttled (retry with backoff)
    //   - ErrKMSInternal: transient KMS error (retry once)
    SignDigest(ctx context.Context, keyARN string, digest [32]byte) (signature []byte, err error)

    // GetPublicKey retrieves the public key for the specified KMS key.
    // Used to derive the agent's Ethereum address.
    //
    // Errors:
    //   - ErrKMSKeyNotFound: key ARN does not exist (do not retry)
    //   - ErrKMSKeyDisabled: key is disabled (do not retry)
    //   - ErrKMSThrottled: API throttled (retry with backoff)
    GetPublicKey(ctx context.Context, keyARN string) (publicKeyDER []byte, err error)
}

var (
    ErrKMSKeyNotFound = errors.New("kms: key not found")
    ErrKMSKeyDisabled = errors.New("kms: key disabled")
    ErrKMSThrottled   = errors.New("kms: throttled")
    ErrKMSInternal    = errors.New("kms: internal error")
)
```

---

### Agent Runtime Interface (Provided by Operator)

This is the interface the operator **provides** to agent containers. The agent runtime teams (review and execution) implement against this contract. The operator owns these definitions.

#### Environment Variables

Every agent K8s Job (both review and execution) receives these environment variables. All values are strings.

Variable names are harmonized with the Platform Engineer's runtime specs (`lld-agent-review-runtime.md`, `lld-agent-execution-runtime.md`). Where naming differed, the runtime's convention wins because both runtimes already code against those names.

**Core variables (set for all Jobs):**

| Variable | Description | Example | Set For |
|----------|-------------|---------|---------|
| `TIDE_PROPOSAL_ID` | On-chain proposal ID (decimal string) | `"7"` | Both |
| `TIDE_DESIGN_HASH` | Design document hash (0x-prefixed) | `"0xabc1..."` | Both |
| `TIDE_AGENT_TOKEN_ID` | Agent's ERC-8004 token ID (decimal) | `"1"` | Both |
| `TIDE_AGENT_NAME` | Human-readable agent name | `"alpha"` | Both |
| `TIDE_PROPOSALS_REPO` | Proposals repo (`org/name`) | `"sei-tide/proposals"` | Both |
| `TIDE_GITHUB_APP_ID` | GitHub App numeric ID | `"123456"` | Both |
| `TIDE_GITHUB_INSTALLATION_ID` | GitHub App installation ID | `"78901234"` | Both |
| `TIDE_PROVIDER_ADDRESS` | Agent Sei wallet (0x-prefixed) | `"0xdead..."` | Both |
| `TIDE_EXPIRES_AT` | Job/proposal expiry (RFC 3339) | `"2026-03-28T12:00:00Z"` | Both |
| `TIDE_SEI_RPC_URL` | Sei EVM HTTP RPC endpoint | `"https://evm-rpc.sei.io"` | Both |
| `TIDE_SEI_CHAIN_ID` | Sei chain ID | `"1329"` | Both |
| `TIDE_KMS_KEY_ARN` | Agent's AWS KMS key ARN | `"arn:aws:kms:..."` | Both |
| `TIDE_AWS_REGION` | AWS region for KMS API calls | `"us-west-2"` | Both |
| `TIDE_RESULT_PATH` | Path to write result JSON | `"/dev/termination-log"` | Both |
| `TIDE_RUNTIME_MODE` | `"review"` or `"execution"` | `"execution"` | Both |
| `TIDE_LOG_LEVEL` | Structured log level | `"info"` | Both |

**Review-only variables:**

| Variable | Description | Example |
|----------|-------------|---------|
| `TIDE_COUNCIL_CONTRACT` | TideCouncil contract address | `"0x1234...abcd"` |
| `TIDE_DESIGN_PATH` | Path to design doc in proposals repo | `"proposals/2026-03/design-v2.md"` |
| `TIDE_LLM_MODEL` | Anthropic model ID (default from platform ConfigMap) | `"claude-sonnet-4-20250514"` |
| `TIDE_LLM_TOKEN_BUDGET` | Total token budget for the review | `"200000"` |
| `TIDE_LLM_MAX_OUTPUT_TOKENS` | Max output tokens per API call | `"16384"` |
| `TIDE_LLM_TEMPERATURE` | LLM temperature | `"0.3"` |
| `TIDE_REVIEW_TIMEOUT_SECONDS` | Soft timeout for review workflow | `"1800"` |

**Execution-only variables:**

| Variable | Description | Example |
|----------|-------------|---------|
| `TIDE_JOB_ID` | On-chain ERC-8183 job ID (decimal) | `"42"` |
| `TIDE_ACP_CONTRACT` | ERC-8183 AgenticCommerce contract address | `"0x5678...efgh"` |
| `TIDE_WORKSPACE_REPO` | Agent workspace repo (`org/name`) | `"sei-tide/agent-alpha-job-42"` |
| `TIDE_DELIVERABLES_REPO` | PR target repo (`org/name`) | `"sei-tide/deliverables"` |
| `TIDE_UPSTREAM_REPO` | Upstream source repo to clone | `"sei-protocol/sei-chain"` |
| `TIDE_UPSTREAM_BRANCH` | Branch to clone from upstream (default: `main`) | `"main"` |
| `TIDE_UPSTREAM_COMMIT` | Pin clone to specific commit (optional) | `"abc123..."` |
| `TIDE_WORKSPACE_BRANCH` | Branch in workspace repo (default: `job-{TIDE_JOB_ID}`) | `"job-7-caching-layer"` |
| `TIDE_TASK_DESCRIPTION` | Human-readable task description for the coding agent | `"Implement caching..."` |
| `TIDE_TASK_FILE_PATH` | Optional path to detailed task spec in proposals repo | `"tasks/job-7-spec.md"` |
| `TIDE_CLIENT_ADDRESS` | Principal wallet (0x-prefixed) | `"0xbeef..."` |
| `TIDE_BUDGET_RAW` | USDC budget in raw units (decimal) | `"36000000"` |
| `TIDE_LLM_MODEL` | Anthropic model ID | `"claude-sonnet-4-20250514"` |
| `TIDE_LLM_TOKEN_BUDGET` | Total token budget for execution | `"2000000"` |
| `TIDE_LLM_MAX_OUTPUT_TOKENS` | Max output tokens per LLM call | `"16384"` |
| `TIDE_MAX_ITERATIONS` | Max edit-test-fix iterations | `"25"` |
| `TIDE_EXECUTION_TIMEOUT_SECONDS` | Soft timeout for coding workflow | `"3000"` |
| `TIDE_CODING_FRAMEWORK` | Coding agent framework (`openhands`/`swe-agent`/`custom`) | `"openhands"` |

**Canonical naming (runtime convention wins — runtimes are the consumers):**
- `TIDE_KMS_KEY_ARN` — runtimes expect ARN, not key ID
- `TIDE_COUNCIL_CONTRACT` — runtimes use CONTRACT, not ADDRESS
- `TIDE_ACP_CONTRACT` — runtimes use CONTRACT, not ADDRESS
- `TIDE_GITHUB_INSTALLATION_ID` — runtimes use this shorter form

Note: The operator's OWN config vars (`TIDE_COUNCIL_ADDRESS`, `TIDE_ACP_ADDRESS` in §Configuration) retain `_ADDRESS` naming since those are operator-internal. The agent Job env vars above use the runtime naming convention.

#### Volume Mounts

| Mount Path | Volume Type | Access | Purpose |
|------------|-------------|--------|---------|
| `/workspace` | `emptyDir` (sizeLimit: 10Gi) | read-write | Git working directory. Init container clones repos here; main container works here. |
| `/tmp` | `emptyDir` (sizeLimit: 1Gi) | read-write | Temporary files for the agent process. |
| `/secrets` | CSI `SecretProviderClass` | read-only | Secrets from AWS Secrets Manager via per-agent SecretProviderClass. |

**`/secrets` directory layout (per SecretProviderClass in `lld-k8s-manifests.md`):**

```
/secrets/
├── github-app-key.pem              # GitHub App RSA private key (PKCS#1 PEM)
├── anthropic-api-key               # Plaintext Anthropic API key (no newline)
├── agent-system-prompt.txt         # Review persona/instructions
├── agent-execution-system-prompt.txt  # Execution persona/instructions
├── tide-council-abi.json           # TideCouncil contract ABI JSON array
└── acp-abi.json                    # ERC-8183 ACP contract ABI JSON array
```

#### Init Container Contract

The operator creates an init container named `workspace-setup` using a platform-provided image. The init container:

1. Reads `TIDE_GITHUB_APP_ID` and `/secrets/github-app-key.pem`
2. Generates a GitHub App installation token
3. Clones `TIDE_WORKSPACE_REPO` into `/workspace/src`
4. Clones `TIDE_PROPOSALS_REPO` into `/workspace/proposals` (read-only reference)
5. Writes the installation token to `/workspace/.tide/github-token` (agent reads this for git push)
6. Writes git configuration to `/workspace/.gitconfig`
7. Exits 0 on success, 1 on any failure

The init container image is referenced from agent configuration (see §Configuration). The operator does not own this image.

#### Completion Protocol

The agent runtime signals completion to the operator via **Kubernetes termination messages** combined with **Job exit codes**.

**Exit codes (aligned with Platform Engineer runtime specs — `lld-agent-review-runtime.md` §Exit Codes, `lld-agent-execution-runtime.md` §Exit Codes):**

| Exit Code | Meaning | Controller Action |
|-----------|---------|-------------------|
| `0` | Success — deliverable ready | Parse result from termination message, transition to `Submitting` |
| `1` | Unrecoverable internal error (bug, panic) | Transition to `Failed`, reason `InternalError`. Alert platform team. Do not retry. |
| `2` | Soft timeout — agent exceeded 80% of `activeDeadlineSeconds` | Transition to `Failed`, reason `AgentSoftTimeout`. Partial work may exist. Check termination message for `AgentResult`. |
| `10` | Missing or invalid environment variable | Transition to `Failed`, reason `EnvValidationFailed`. Fix Operator Job template. Do not retry. |
| `11` | Secret mount failure — required file missing at `/secrets/*` | Transition to `Failed`, reason `SecretMountFailed`. Check SecretProviderClass. Do not retry. |
| `20` | Git clone failure (network, auth, repo not found) | Requeue with backoff. Check GitHub App credentials. |
| `21` | Source not found (design doc path or upstream commit) | Transition to `Failed`, reason `SourceNotFound`. Do not retry. |
| `22` | Integrity failure (design hash mismatch or task file missing) | Transition to `Failed`, reason `IntegrityCheckFailed`. Do not retry. |
| `30` | LLM API failure after retries exhausted | Requeue with backoff. Check API key, rate limits, model availability. |
| `31` | Token budget exceeded | Check termination message for `partial` status. Operator may retry with higher budget. |
| `32` | Max iterations exceeded — tests still failing (execution only) | Check termination message for progress. Operator decides retry or fail. |
| `40` | Git push failure | Requeue with backoff. Check GitHub App permissions. |
| `41` | PR creation failure (execution only) | Requeue with backoff. Check permissions on deliverables repo. |
| `50` | KMS signing failure | Requeue with backoff. Check IRSA permissions, KMS key status. |
| `51` | Sei RPC failure — transaction submission failed | Requeue with backoff. Check RPC endpoint health. |
| `52` | Sei transaction reverted — on-chain error | Parse revert reason from termination message. Do not retry blindly. |
| `137` | OOMKilled (SIGKILL by kernel) | Transition to `Failed`, reason `OOMKilled`. Increase memory limit. |
| `143` | SIGTERM from K8s (activeDeadlineSeconds hard timeout) | Transition to `Failed`, reason `DeadlineExceeded`. Termination message may be absent. |

The Operator groups exit codes into retry categories:
- **Do not retry:** 1, 10, 11, 21, 22, 52
- **Retry with backoff:** 20, 30, 40, 41, 50, 51
- **Operator decides:** 31, 32 (partial work may exist — check termination message)
- **Infrastructure failure:** 137, 143

**Termination message:** The agent writes a JSON object to the path specified by `TIDE_RESULT_PATH` (default `/dev/termination-log`). Kubernetes exposes this in `pod.status.containerStatuses[].state.terminated.message` (max 4096 bytes). This is the **primary** completion signaling mechanism.

**Why not `/workspace/.tide/status.json`?** The Platform Engineer's runtime specs define a richer status file at `/workspace/.tide/status.json` on an `emptyDir` volume. However, after a pod terminates, Kubernetes garbage-collects `emptyDir` volumes — the Operator **cannot** read this file post-termination. The termination message is the only reliable post-termination channel because it is persisted in the Pod API object (etcd). The runtimes MAY still write `status.json` for live debugging (e.g., `kubectl exec` into a running pod), but it is advisory, not authoritative.

#### Result Schema

```go
// AgentResult is the JSON written to TIDE_RESULT_PATH by the agent container.
// The operator parses this from the Pod termination message after Job completion.
//
// Size budget: Kubernetes termination messages are capped at 4096 bytes.
// A fully populated AgentResult serializes to ~800-1500 bytes, well within budget.
// The Error field is truncated to 1KB if necessary to stay within limits.
type AgentResult struct {
    // Status is "success", "failure", or "partial".
    // "partial" means the primary task completed but a secondary step failed
    // (e.g., review pushed to GitHub but on-chain attestation failed).
    Status string `json:"status"`

    // ExitCode is the container's exit code, included for redundancy
    // (the Operator also reads it from the Pod status).
    ExitCode int `json:"exitCode"`

    // DeliverableHash is the keccak256 of the deliverable artifact.
    // Required when Status="success" for execution jobs.
    // For review jobs, this is the keccak256 of the review JSON.
    // 0x-prefixed, 66 characters.
    DeliverableHash string `json:"deliverableHash,omitempty"`

    // PRUrl is the GitHub pull request URL (execution jobs only).
    PRUrl string `json:"prUrl,omitempty"`

    // PRNumber is the GitHub PR number (execution jobs only).
    PRNumber int `json:"prNumber,omitempty"`

    // CommitSHA is the head commit of the deliverable branch.
    CommitSHA string `json:"commitSha,omitempty"`

    // FeedbackHash is the keccak256 of the review JSON (review jobs only).
    FeedbackHash string `json:"feedbackHash,omitempty"`

    // Verdict is the review verdict: "approve" or "request_changes" (review jobs only).
    Verdict string `json:"verdict,omitempty"`

    // TxHash is the Sei transaction hash for on-chain submission (if applicable).
    TxHash string `json:"txHash,omitempty"`

    // TokenUsage reports LLM token consumption for cost tracking.
    TokenUsage *TokenUsage `json:"tokenUsage,omitempty"`

    // Timing reports wall-clock durations for each stage.
    Timing *TimingInfo `json:"timing,omitempty"`

    // Error is a human-readable error message when Status="failure" or "partial".
    // Truncated to 1KB to stay within the 4KB termination message limit.
    Error string `json:"error,omitempty"`

    // ErrorStage identifies which stage failed (e.g., "llm_review", "git_push",
    // "kms_sign", "sei_submit", "clone", "coding", "test").
    ErrorStage string `json:"errorStage,omitempty"`
}

type TokenUsage struct {
    Input         int64 `json:"input"`
    Output        int64 `json:"output"`
    Total         int64 `json:"total"`
    BudgetTotal   int64 `json:"budgetTotal,omitempty"`
}

type TimingInfo struct {
    StartedAt  string `json:"startedAt"`            // RFC 3339
    FinishedAt string `json:"finishedAt"`            // RFC 3339
    TotalSec   int    `json:"totalSec"`              // wall-clock seconds
    StageSec   map[string]int `json:"stageSec,omitempty"` // per-stage seconds (e.g., "clone":5, "llm":180, "push":8)
}
```

---

### Event Signatures (Consumed)

The operator indexes events from three contract addresses. These event signatures are **aligned with the Blockchain Developer's deployed contract specs** as of the cross-review on 2026-03-21 (see `cross-review-operator.md`). If the blockchain team changes event signatures post-deployment, the topic hashes in `pkg/constants/events.go` must be updated to match.

**Topic hash computation:** `keccak256(event_signature_string)`

#### TideCouncil Events

These signatures MUST match the Blockchain Developer's TideCouncil contract exactly.
See `lld-tide-council.md` §Event Signature Strings for the authoritative canonical forms.

```go
package constants

import "github.com/ethereum/go-ethereum/crypto"

var (
    // ProposalCreated(uint256 indexed proposalId, address indexed principal,
    //   bytes32 indexed designHash, uint256 parentProposalId,
    //   uint256[] participantTokenIds, uint8 quorum, uint40 expiresAt)
    //
    // Cross-ref: lld-tide-council.md §Events — 3 indexed fields, 4 data fields.
    TopicProposalCreated = crypto.Keccak256Hash([]byte(
        "ProposalCreated(uint256,address,bytes32,uint256,uint256[],uint8,uint40)"))

    // ReviewSubmitted(uint256 indexed proposalId, uint256 indexed agentTokenId,
    //   uint8 verdict, bytes32 feedbackHash)
    //
    // Cross-ref: lld-tide-council.md §Events — agentTokenId is uint256 on-chain.
    TopicReviewSubmitted = crypto.Keccak256Hash([]byte(
        "ReviewSubmitted(uint256,uint256,uint8,bytes32)"))

    // ProposalApproved(uint256 indexed proposalId, bytes32 indexed designHash)
    //
    // Cross-ref: lld-tide-council.md §Events — both fields indexed.
    TopicProposalApproved = crypto.Keccak256Hash([]byte(
        "ProposalApproved(uint256,bytes32)"))

    // ProposalRejected(uint256 indexed proposalId, bytes32 indexed designHash)
    //
    // Cross-ref: lld-tide-council.md §Events — principal withdrew the proposal.
    TopicProposalRejected = crypto.Keccak256Hash([]byte(
        "ProposalRejected(uint256,bytes32)"))

    // ProposalExpired(uint256 indexed proposalId)
    TopicProposalExpired = crypto.Keccak256Hash([]byte(
        "ProposalExpired(uint256)"))
)
```

**Decoded event structs (Go):**

```go
package indexer

import (
    "math/big"
    "github.com/ethereum/go-ethereum/common"
)

type ProposalCreatedEvent struct {
    ProposalID          *big.Int       // indexed, in topics[1]
    Principal           common.Address // indexed, in topics[2]
    DesignHash          [32]byte       // indexed, in topics[3]
    ParentProposalID    *big.Int       // in data (uint256; 0 for initial proposals)
    ParticipantTokenIDs []*big.Int     // in data (uint256[] — agent ERC-8004 token IDs)
    Quorum              uint8          // in data
    ExpiresAt           uint64         // in data (uint40 on-chain, fits in uint64)
}

type ReviewSubmittedEvent struct {
    ProposalID   *big.Int // indexed, in topics[1]
    AgentTokenID *big.Int // indexed, in topics[2] (uint256 on-chain; validated to fit uint64)
    Verdict      uint8    // in data (0=Approve, 1=RequestChanges)
    FeedbackHash [32]byte // in data
}

type ProposalApprovedEvent struct {
    ProposalID *big.Int // indexed, in topics[1]
    DesignHash [32]byte // indexed, in topics[2] — NOT in data
}

type ProposalRejectedEvent struct {
    ProposalID *big.Int // indexed, in topics[1]
    DesignHash [32]byte // indexed, in topics[2] — NOT in data
}

type ProposalExpiredEvent struct {
    ProposalID *big.Int // indexed, in topics[1]
}
```

#### TideJobHook Events

These signatures MUST match the Blockchain Developer's TideJobHook contract exactly.
See `lld-tide-job-hook.md` §Event Signature Strings for the authoritative canonical forms.

```go
var (
    // SandboxProvisionRequested(uint256 indexed jobId, address indexed provider,
    //   address indexed client, uint256 agentTokenId, uint256 budget, uint256 expiry)
    //
    // Cross-ref: lld-tide-job-hook.md §Events — 3 indexed fields, 3 data fields.
    // Emitted by TideJobHook.afterAction() when selector == FUND_SELECTOR.
    TopicSandboxProvisionRequested = crypto.Keccak256Hash([]byte(
        "SandboxProvisionRequested(uint256,address,address,uint256,uint256,uint256)"))
)
```

```go
type SandboxProvisionRequestedEvent struct {
    JobID        *big.Int       // indexed, in topics[1]
    Provider     common.Address // indexed, in topics[2]
    Client       common.Address // indexed, in topics[3]
    AgentTokenID *big.Int       // in data (uint256; validated to fit uint64)
    Budget       *big.Int       // in data (raw USDC 6-decimal units)
    Expiry       *big.Int       // in data (unix timestamp)
}
```

#### ERC-8183 ACP Events

```go
var (
    // JobCompleted(uint256 indexed jobId)
    TopicJobCompleted = crypto.Keccak256Hash([]byte(
        "JobCompleted(uint256)"))

    // JobRejected(uint256 indexed jobId)
    TopicJobRejected = crypto.Keccak256Hash([]byte(
        "JobRejected(uint256)"))

    // RefundClaimed(uint256 indexed jobId)
    TopicRefundClaimed = crypto.Keccak256Hash([]byte(
        "RefundClaimed(uint256)"))
)
```

```go
type JobCompletedEvent struct {
    JobID *big.Int // indexed, in topics[1]
}

type JobRejectedEvent struct {
    JobID *big.Int // indexed, in topics[1]
}

type RefundClaimedEvent struct {
    JobID *big.Int // indexed, in topics[1]
}
```

---

### Constants

```go
package constants

// K8s label keys. All under the tide.sei.io/ prefix.
const (
    LabelManagedBy  = "app.kubernetes.io/managed-by"
    LabelComponent  = "app.kubernetes.io/component"
    LabelAgentID    = "tide.sei.io/agent-id"
    LabelAgentToken = "tide.sei.io/agent-token-id"
    LabelJobID      = "tide.sei.io/job-id"
    LabelProposalID = "tide.sei.io/proposal-id"
    LabelPrincipal  = "tide.sei.io/principal"
    LabelRuntimeMode = "tide.sei.io/runtime-mode" // "review" or "execution"

    ManagedByValue = "tide-operator"
    ComponentAgent = "agent"
)

// K8s annotation keys.
const (
    AnnotationDesignHash = "tide.sei.io/design-hash"
    AnnotationSourceTx   = "tide.sei.io/source-tx"
)

// Namespace constants.
const (
    NamespaceSystem = "tide-system"
    NamespaceAgents = "tide-agents"
)

// ConfigMap names.
const (
    ConfigMapEventCursor = "tide-event-cursor"
    ConfigMapAgentConfig = "tide-agent-config"
)

// Event cursor keys (stored in ConfigMap data).
const (
    CursorKeyBlock    = "lastProcessedBlock"
    CursorKeyTxIndex  = "lastProcessedTxIndex"
    CursorKeyLogIndex = "lastProcessedLogIndex"
)
```

```go
package constants

// Sei chain configuration.
const (
    SeiMainnetChainID = 1329
    SeiTestnetChainID = 713715 // arctic-1
)
```

---

### Configuration

The operator is configured entirely via environment variables and one ConfigMap.

#### Operator Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TIDE_SEI_WS_URL` | Yes | — | Sei EVM WebSocket RPC (e.g., `wss://ws.evm-rpc.sei.io`) |
| `TIDE_SEI_HTTP_URL` | Yes | — | Sei EVM HTTP RPC (e.g., `https://evm-rpc.sei.io`) |
| `TIDE_SEI_CHAIN_ID` | Yes | — | Chain ID (`1329` or `713715`) |
| `TIDE_COUNCIL_ADDRESS` | Yes | — | TideCouncil contract address |
| `TIDE_JOB_HOOK_ADDRESS` | Yes | — | TideJobHook contract address |
| `TIDE_ACP_ADDRESS` | Yes | — | ERC-8183 ACP contract address |
| `TIDE_INDEXER_START_BLOCK` | No | `0` | Block to begin indexing from (set to contract deployment block) |
| `TIDE_INDEXER_BATCH_SIZE` | No | `1000` | Max blocks per `eth_getLogs` call |
| `TIDE_INDEXER_POLL_INTERVAL` | No | `2s` | Polling interval when WebSocket is unavailable |
| `TIDE_GITHUB_ORG` | Yes | — | GitHub organization name (e.g., `sei-tide`) |
| `TIDE_WORKSPACE_TEMPLATE_REPO` | No | `""` | Template repo for workspace creation |
| `TIDE_PROPOSALS_REPO` | Yes | — | Proposals repo name (e.g., `proposals`) |
| `TIDE_DELIVERABLES_REPO` | Yes | — | Deliverables repo name (e.g., `deliverables`) |
| `TIDE_TOKEN_REFRESH_INTERVAL` | No | `30m` | How often to refresh GitHub App tokens for active jobs |
| `TIDE_RECONCILE_INTERVAL` | No | `5m` | Periodic reconciliation interval for non-terminal CRs |
| `TIDE_LEADER_ELECTION_ID` | No | `tide-operator-leader` | Lease name for leader election |
| `TIDE_METRICS_BIND_ADDR` | No | `:8080` | Prometheus metrics endpoint |
| `TIDE_HEALTH_PROBE_BIND_ADDR` | No | `:8081` | Health probe endpoint |
| `AWS_REGION` | Yes | — | AWS region for KMS and Secrets Manager |

#### Agent Configuration (ConfigMap)

`tide-system/tide-agent-config` ConfigMap, key `agents.yaml`:

```go
// AgentConfigFile is the top-level structure of agents.yaml.
type AgentConfigFile struct {
    Agents []AgentConfig `yaml:"agents"`
}

// AgentConfig defines per-agent operational configuration.
// Operator resolves AgentTokenID from on-chain events to this config
// to obtain GitHub credentials, KMS key, and container images.
type AgentConfig struct {
    // TokenID is the ERC-8004 identity token (matches on-chain events).
    TokenID uint64 `yaml:"tokenID"`

    // Name is the human-readable agent name, used in K8s Job names and labels.
    Name string `yaml:"name"`

    // WalletAddress is the agent's Sei wallet (for verification only).
    WalletAddress string `yaml:"walletAddress"`

    // GitHubAppID is the numeric ID of this agent's GitHub App.
    GitHubAppID int64 `yaml:"githubAppID"`

    // GitHubInstallationID is the App installation ID on the sei-tide org.
    GitHubInstallationID int64 `yaml:"githubInstallationID"`

    // KMSKeyARN is the AWS KMS key ARN for this agent's Sei wallet.
    KMSKeyARN string `yaml:"kmsKeyARN"`

    // ReviewImage is the container image for review Jobs.
    ReviewImage string `yaml:"reviewImage"`

    // ExecutionImage is the container image for execution Jobs.
    ExecutionImage string `yaml:"executionImage"`

    // InitImage is the container image for the workspace-setup init container.
    InitImage string `yaml:"initImage"`

    // SecretProviderClass is the name of the SecretProviderClass CR for this agent.
    SecretProviderClass string `yaml:"secretProviderClass"`
}
```

Example:

```yaml
agents:
  - tokenID: 1
    name: alpha
    walletAddress: "0x1234567890abcdef1234567890abcdef12345678"
    githubAppID: 100001
    githubInstallationID: 200001
    kmsKeyARN: "arn:aws:kms:us-east-1:123456789012:key/alpha-key-id"
    reviewImage: "123456789012.dkr.ecr.us-east-1.amazonaws.com/tide-agent-review:v0.1.0"
    executionImage: "123456789012.dkr.ecr.us-east-1.amazonaws.com/tide-agent-exec:v0.1.0"
    initImage: "123456789012.dkr.ecr.us-east-1.amazonaws.com/tide-workspace-init:v0.1.0"
    secretProviderClass: "tide-agent-alpha-secrets"
  - tokenID: 2
    name: beta
    walletAddress: "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"
    githubAppID: 100002
    githubInstallationID: 200002
    kmsKeyARN: "arn:aws:kms:us-east-1:123456789012:key/beta-key-id"
    reviewImage: "123456789012.dkr.ecr.us-east-1.amazonaws.com/tide-agent-review:v0.1.0"
    executionImage: "123456789012.dkr.ecr.us-east-1.amazonaws.com/tide-agent-exec:v0.1.0"
    initImage: "123456789012.dkr.ecr.us-east-1.amazonaws.com/tide-workspace-init:v0.1.0"
    secretProviderClass: "tide-agent-beta-secrets"
  - tokenID: 3
    name: gamma
    walletAddress: "0x9876543210fedcba9876543210fedcba98765432"
    githubAppID: 100003
    githubInstallationID: 200003
    kmsKeyARN: "arn:aws:kms:us-east-1:123456789012:key/gamma-key-id"
    reviewImage: "123456789012.dkr.ecr.us-east-1.amazonaws.com/tide-agent-review:v0.1.0"
    executionImage: "123456789012.dkr.ecr.us-east-1.amazonaws.com/tide-agent-exec:v0.1.0"
    initImage: "123456789012.dkr.ecr.us-east-1.amazonaws.com/tide-workspace-init:v0.1.0"
    secretProviderClass: "tide-agent-gamma-secrets"
```

---

### Prometheus Metrics

All metrics are exposed on `TIDE_METRICS_BIND_ADDR` (default `:8080/metrics`).

```go
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
    EventsProcessed = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "tide_events_processed_total",
        Help: "On-chain events processed by the indexer.",
    }, []string{"event_type"}) // event_type: proposal_created, review_submitted, job_funded, etc.

    EventProcessingLag = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "tide_event_processing_lag_seconds",
        Help: "Seconds between chain HEAD and the last processed block.",
    })

    IndexerErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "tide_indexer_errors_total",
        Help: "Errors encountered by the event indexer.",
    }, []string{"error_type"}) // error_type: rpc_unavailable, parse_failure, cursor_update

    ProposalsActive = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "tide_proposals_active",
        Help: "Number of TideProposal CRs in non-terminal phases.",
    })

    JobsCreated = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "tide_jobs_created_total",
        Help: "K8s Jobs created by the operator.",
    }, []string{"agent_name", "runtime_mode"}) // runtime_mode: review, execution

    JobsTerminal = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "tide_jobs_terminal_total",
        Help: "TideJob CRs that reached a terminal phase.",
    }, []string{"phase"}) // phase: Completed, Rejected, Failed, Expired

    ActiveK8sJobs = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "tide_active_k8s_jobs",
        Help: "Currently running agent K8s Jobs in tide-agents namespace.",
    })

    TokenRefreshErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "tide_token_refresh_errors_total",
        Help: "GitHub App token refresh failures.",
    }, []string{"agent_name"})

    GitHubAPIRemaining = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Name: "tide_github_api_remaining",
        Help: "GitHub API rate limit remaining requests.",
    }, []string{"agent_name"})

    SandboxProvisioningDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "tide_sandbox_provisioning_seconds",
        Help:    "Time to provision a GitHub sandbox.",
        Buckets: prometheus.ExponentialBuckets(1, 2, 8), // 1s, 2s, 4s, ... 128s
    })

    KMSSignErrors = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "tide_kms_sign_errors_total",
        Help: "AWS KMS signing request failures.",
    })

    OnChainSubmitDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "tide_onchain_submit_seconds",
        Help:    "Time from submit transaction send to 1-confirmation receipt.",
        Buckets: prometheus.ExponentialBuckets(0.5, 2, 8), // 0.5s to 64s
    })

    ReconcileErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "tide_reconcile_errors_total",
        Help: "Controller reconciliation errors.",
    }, []string{"controller"}) // controller: tideproposal, tidejob
)
```

---

## State Model

### CRD Lifecycle: TideProposal

```
                     ┌─────────────────────────────┐
                     │                             │
                     ▼                             │
  [ProposalCreated] ──► Pending ──► Active ────────┤
   (indexer creates       │          │              │
    CR)                   │          │  [ReviewSubmitted events
                          │          │   update status.reviews]
                          │          │
                          │          ├──► Approved  (terminal)
                          │          │    [ProposalApproved event or
                          │          │     quorum detected + finalized]
                          │          │
                          │          └──► Expired   (terminal)
                          │               [ExpiresAt elapsed or
                          │                ProposalExpired event]
                          │
                          └──► Expired   (terminal)
                               [ExpiresAt elapsed before
                                review Jobs launched]
```

**Source of truth:** On-chain TideCouncil contract state.

The CRD is a **cached projection** of on-chain state. If CRD and chain diverge (detected by periodic reconciliation), the chain wins and the CRD is updated.

**State transitions:**

| From | To | Trigger | Controller Action |
|------|----|---------|-------------------|
| — | Pending | Indexer processes `ProposalCreated` event | Create `TideProposal` CR with spec from event data |
| Pending | Active | Controller reconcile loop | Resolve agent configs, create one review K8s Job per participant, set condition `ReviewJobsLaunched=True` |
| Active | Active | Indexer processes `ReviewSubmitted` event | Update `status.reviews[]` entry, increment `approvalCount` if verdict=approve |
| Active | Approved | Indexer processes `ProposalApproved` event | Set phase=Approved, set condition `OnChainFinalized=True` |
| Active | Expired | `time.Now() > spec.expiresAt` or `ProposalExpired` event | Set phase=Expired, set condition `OnChainFinalized=True` |
| Pending | Expired | `time.Now() > spec.expiresAt` | Set phase=Expired |

### CRD Lifecycle: TideJob

```
  [SandboxProvisionRequested] ──► Pending
   (indexer creates CR)             │
                                    ▼
                               Provisioning ──────► Failed (terminal)
                                    │               [GitHub API error,
                                    │                agent config missing]
                                    ▼
                                 Running ─────────► Failed (terminal)
                                    │               [K8s Job failed: OOM,
                                    │                timeout, exit code != 0]
                                    ▼
                                Submitting ───────► Failed (terminal)
                                    │               [KMS sign error,
                                    │                tx reverted,
                                    │                insufficient gas]
                                    ▼
                                 Submitted ───────► Completed (terminal)
                                    │               [JobCompleted event]
                                    │
                                    ├─────────────► Rejected (terminal)
                                    │               [JobRejected event]
                                    │
                                    └─────────────► Expired (terminal)
                                                    [RefundClaimed event
                                                     or expiresAt elapsed]

  Any non-terminal phase ─────────► Expired (terminal)
                                    [spec.expiresAt elapsed]
```

**Source of truth:** On-chain ERC-8183 job state for terminal outcomes (Completed, Rejected, Expired). K8s Job state for Running/Failed. Operator-internal state for Provisioning/Submitting.

**State transitions:**

| From | To | Trigger | Controller Action |
|------|----|---------|-------------------|
| — | Pending | Indexer processes `SandboxProvisionRequested` | Create `TideJob` CR with spec from event data |
| Pending | Provisioning | Controller reconcile | Look up agent config, call `GitHubClient.EnsureRepository()`, call `GitHubClient.GenerateInstallationToken()` |
| Provisioning | Running | Sandbox ready | Build K8s Job spec (§Generated K8s Job Spec), create batch/v1 Job, set condition `K8sJobCreated=True` |
| Provisioning | Failed | Sandbox provisioning error | Set `failureReason`, emit Warning event |
| Running | Submitting | K8s Job pod exits 0 | Read termination message, parse `AgentResult`, extract `deliverableHash`, set condition `K8sJobSucceeded=True` |
| Running | Running | K8s Job pod exits with retryable code (20, 30, 40, 41, 50, 51) | Requeue with backoff. Operator may re-create Job (backoff limit is 0, so the Job itself won't retry). |
| Running | Failed | K8s Job pod exits with non-retryable code (1, 10, 11, 21, 22, 52, 137, 143) | Read termination message, set `failureReason` from result or pod status |
| Submitting | Submitted | `SubmitDeliverable()` succeeds | Record `submissionTxHash`, set condition `DeliverableSubmitted=True` |
| Submitting | Failed | `SubmitDeliverable()` fails permanently | Set `failureReason` |
| Submitted | Completed | Indexer processes `JobCompleted` event | Set condition `OnChainTerminal=True` |
| Submitted | Rejected | Indexer processes `JobRejected` event | Set condition `OnChainTerminal=True` |
| Submitted | Expired | Indexer processes `RefundClaimed` event or `expiresAt` elapsed | Set condition `OnChainTerminal=True` |
| Any non-terminal | Expired | `time.Now() > spec.expiresAt` | Set phase=Expired, clean up K8s Job if running |

### Event Cursor (ConfigMap)

The `tide-system/tide-event-cursor` ConfigMap persists the indexer's position:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tide-event-cursor
  namespace: tide-system
data:
  lastProcessedBlock: "5000000"
  lastProcessedTxIndex: "12"
  lastProcessedLogIndex: "3"
```

The cursor uses a **three-part position** (block, tx index within block, log index within tx) to enable deterministic replay from the exact point of last processing. On startup, if the ConfigMap does not exist or has empty values, the indexer starts from `TIDE_INDEXER_START_BLOCK`.

The cursor is updated **after** the CRD create/update for each event succeeds. This provides at-least-once delivery: a crash between CRD write and cursor update causes re-processing of the same event on restart, which is safe because CRD creation is idempotent (keyed by on-chain ID).

### On-Chain ↔ Off-Chain State Mapping

| On-Chain State | CRD Resource | CRD Phase | Notes |
|---------------|-------------|-----------|-------|
| TideCouncil: Proposal `status=Proposed` | TideProposal | Pending or Active | Active once review Jobs are launched |
| TideCouncil: Proposal `status=Approved` | TideProposal | Approved | Terminal |
| TideCouncil: Proposal `status=Expired` | TideProposal | Expired | Terminal |
| ERC-8183: Job funded, not submitted | TideJob | Pending → Provisioning → Running | Operator manages sandbox + K8s Job |
| ERC-8183: Job submitted | TideJob | Submitted | Awaiting evaluator decision |
| ERC-8183: Job completed | TideJob | Completed | Terminal, USDC released |
| ERC-8183: Job rejected | TideJob | Rejected | Terminal, USDC returned |
| ERC-8183: Job refund claimed | TideJob | Expired | Terminal, USDC returned |

---

## Internal Design

### Binary Architecture

```
┌──────────────────────────────────────────────────────┐
│                   tide-operator binary                │
│                                                      │
│  ┌─────────────────────────────────────────────────┐ │
│  │            controller-runtime Manager            │ │
│  │                                                 │ │
│  │  ┌──────────────┐  ┌────────────────────────┐  │ │
│  │  │ TideProposal │  │      TideJob           │  │ │
│  │  │ Controller   │  │      Controller        │  │ │
│  │  │              │  │                        │  │ │
│  │  │ Watches:     │  │ Watches:               │  │ │
│  │  │ - TideProposal│ │ - TideJob              │  │ │
│  │  │ - batch/v1 Job│ │ - batch/v1 Job         │  │ │
│  │  └──────────────┘  └────────────────────────┘  │ │
│  │                                                 │ │
│  │  ┌──────────────────────────────────────────┐  │ │
│  │  │         Event Indexer (Runnable)          │  │ │
│  │  │                                          │  │ │
│  │  │ Subscribes to Sei EVM events             │  │ │
│  │  │ Creates/updates TideProposal, TideJob    │  │ │
│  │  │ Persists cursor in ConfigMap             │  │ │
│  │  └──────────────────────────────────────────┘  │ │
│  │                                                 │ │
│  │  ┌──────────────────────────────────────────┐  │ │
│  │  │      Token Refresh Manager (Runnable)    │  │ │
│  │  │                                          │  │ │
│  │  │ Periodic goroutine (every 30m)           │  │ │
│  │  │ Refreshes GitHub tokens for Running jobs │  │ │
│  │  └──────────────────────────────────────────┘  │ │
│  │                                                 │ │
│  │  Leader Election (via coordination.k8s.io/v1   │ │
│  │   Lease API)                                    │ │
│  │                                                 │ │
│  │  Health: /healthz (liveness), /readyz (readiness)│ │
│  └─────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────┘
```

All components run within a single `ctrl.Manager`. The Event Indexer and Token Refresh Manager are registered as `manager.Runnable` instances — they start when the manager starts (i.e., when the replica wins leader election) and stop when the manager stops.

### Event Indexer

The indexer runs as a `manager.LeaderElectionRunnable` (only on the leader). It maintains a single goroutine that processes blocks sequentially.

**Startup sequence:**

```
1. Read cursor from ConfigMap tide-system/tide-event-cursor
   - If not found or empty: start from TIDE_INDEXER_START_BLOCK
   - If found: start from lastProcessedBlock + 1

2. Build ethereum.FilterQuery:
   Addresses: [TIDE_COUNCIL_ADDRESS, TIDE_JOB_HOOK_ADDRESS, TIDE_ACP_ADDRESS]
   Topics[0]: [TopicProposalCreated, TopicReviewSubmitted,
               TopicProposalApproved, TopicProposalRejected, TopicProposalExpired,
               TopicSandboxProvisionRequested,
               TopicJobCompleted, TopicJobRejected, TopicRefundClaimed]

3. Attempt WebSocket subscription via SubscribeFilterLogs()
   - On success: enter live streaming mode
   - On failure: enter polling mode
```

**Live streaming mode (WebSocket):**

```
for {
    select {
    case log := <-logCh:
        processLog(log)
        updateCursor(log.BlockNumber, log.TxIndex, log.Index)
    case err := <-sub.Err():
        log.Error("subscription error, falling back to polling")
        metrics.IndexerErrors.WithLabelValues("ws_disconnect").Inc()
        enter polling mode
    case <-ctx.Done():
        return
    }
}
```

**Polling mode (HTTP fallback):**

```
ticker := time.NewTicker(TIDE_INDEXER_POLL_INTERVAL)
for {
    select {
    case <-ticker.C:
        latestBlock := chainClient.LatestBlockNumber()
        if latestBlock <= cursor.Block:
            continue // no new blocks

        // Fetch in batches of TIDE_INDEXER_BATCH_SIZE
        for fromBlock := cursor.Block + 1; fromBlock <= latestBlock; fromBlock += batchSize {
            toBlock := min(fromBlock + batchSize - 1, latestBlock)
            logs := chainClient.GetLogs(filter with fromBlock..toBlock)
            for _, log := range logs {
                processLog(log)
            }
            updateCursor(toBlock, ...)
        }

        // Periodically attempt to re-establish WebSocket
        if time.Since(lastWSAttempt) > 30s {
            tryResubscribe()
        }
    case <-ctx.Done():
        return
    }
}
```

**`processLog` dispatch:**

```go
func (idx *Indexer) processLog(ctx context.Context, log types.Log) error {
    topic0 := log.Topics[0]

    switch topic0 {
    case constants.TopicProposalCreated:
        event, err := decodeProposalCreated(log)
        if err != nil {
            metrics.IndexerErrors.WithLabelValues("parse_failure").Inc()
            return fmt.Errorf("decode ProposalCreated: %w", err)
        }
        return idx.ensureTideProposal(ctx, event, log)

    case constants.TopicReviewSubmitted:
        event, err := decodeReviewSubmitted(log)
        if err != nil {
            return fmt.Errorf("decode ReviewSubmitted: %w", err)
        }
        return idx.updateProposalReview(ctx, event, log)

    case constants.TopicProposalApproved:
        event, err := decodeProposalApproved(log)
        if err != nil {
            return fmt.Errorf("decode ProposalApproved: %w", err)
        }
        return idx.finalizeProposal(ctx, event, ProposalPhaseApproved)

    case constants.TopicProposalRejected:
        event, err := decodeProposalRejected(log)
        if err != nil {
            return fmt.Errorf("decode ProposalRejected: %w", err)
        }
        return idx.finalizeProposal(ctx, event, ProposalPhaseRejected)

    case constants.TopicProposalExpired:
        event, err := decodeProposalExpired(log)
        if err != nil {
            return fmt.Errorf("decode ProposalExpired: %w", err)
        }
        return idx.finalizeProposal(ctx, event, ProposalPhaseExpired)

    case constants.TopicSandboxProvisionRequested:
        event, err := decodeSandboxProvisionRequested(log)
        if err != nil {
            return fmt.Errorf("decode SandboxProvisionRequested: %w", err)
        }
        return idx.ensureTideJob(ctx, event, log)

    case constants.TopicJobCompleted:
        event, err := decodeJobCompleted(log)
        if err != nil {
            return fmt.Errorf("decode JobCompleted: %w", err)
        }
        return idx.updateJobTerminal(ctx, event.JobID, JobPhaseCompleted)

    case constants.TopicJobRejected:
        event, err := decodeJobRejected(log)
        if err != nil {
            return fmt.Errorf("decode JobRejected: %w", err)
        }
        return idx.updateJobTerminal(ctx, event.JobID, JobPhaseRejected)

    case constants.TopicRefundClaimed:
        event, err := decodeRefundClaimed(log)
        if err != nil {
            return fmt.Errorf("decode RefundClaimed: %w", err)
        }
        return idx.updateJobTerminal(ctx, event.JobID, JobPhaseExpired)

    default:
        // Unknown topic — ignore (future contract upgrades may add events)
        return nil
    }
}
```

**Idempotency:** CRD names are derived deterministically from on-chain identifiers:

- `TideProposal`: `proposal-{proposalID}` (e.g., `proposal-7`)
- `TideJob`: `job-{jobID}` (e.g., `job-42`)

`ensureTideProposal` and `ensureTideJob` use `client.Create()` and check for `apierrors.IsAlreadyExists`. If the CR already exists, no update to spec is performed (spec is immutable). Status updates from events (e.g., `ReviewSubmitted`, `JobCompleted`) use `client.Status().Update()` with optimistic concurrency (resource version check).

**Cursor persistence:**

```go
func (idx *Indexer) updateCursor(ctx context.Context, block uint64, txIdx, logIdx uint) error {
    cm := &corev1.ConfigMap{}
    key := client.ObjectKey{Namespace: constants.NamespaceSystem, Name: constants.ConfigMapEventCursor}

    if err := idx.client.Get(ctx, key, cm); err != nil {
        if apierrors.IsNotFound(err) {
            cm = &corev1.ConfigMap{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      constants.ConfigMapEventCursor,
                    Namespace: constants.NamespaceSystem,
                },
                Data: map[string]string{},
            }
            if err := idx.client.Create(ctx, cm); err != nil {
                return err
            }
        } else {
            return err
        }
    }

    cm.Data[constants.CursorKeyBlock] = strconv.FormatUint(block, 10)
    cm.Data[constants.CursorKeyTxIndex] = strconv.FormatUint(uint64(txIdx), 10)
    cm.Data[constants.CursorKeyLogIndex] = strconv.FormatUint(uint64(logIdx), 10)
    return idx.client.Update(ctx, cm)
}
```

### TideProposal Controller

Registered with the manager watching `TideProposal` and `batch/v1 Job` (with a field index on `tide.sei.io/proposal-id` label).

**Reconciliation pseudocode:**

```go
func (r *TideProposalReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    proposal := &v1alpha1.TideProposal{}
    if err := r.Get(ctx, req.NamespacedName, proposal); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // Terminal phases — no further work.
    if proposal.Status.Phase == v1alpha1.ProposalPhaseApproved ||
        proposal.Status.Phase == v1alpha1.ProposalPhaseRejected ||
        proposal.Status.Phase == v1alpha1.ProposalPhaseExpired {
        return ctrl.Result{}, nil
    }

    // Check expiry before any other processing.
    if time.Now().After(proposal.Spec.ExpiresAt.Time) {
        proposal.Status.Phase = v1alpha1.ProposalPhaseExpired
        meta.SetStatusCondition(&proposal.Status.Conditions, metav1.Condition{
            Type:    "OnChainFinalized",
            Status:  metav1.ConditionTrue,
            Reason:  "Expired",
            Message: "Proposal TTL elapsed",
        })
        return ctrl.Result{}, r.Status().Update(ctx, proposal)
    }

    switch proposal.Status.Phase {
    case "", v1alpha1.ProposalPhasePending:
        return r.reconcilePending(ctx, proposal)
    case v1alpha1.ProposalPhaseActive:
        return r.reconcileActive(ctx, proposal)
    }

    return ctrl.Result{}, nil
}

func (r *TideProposalReconciler) reconcilePending(ctx context.Context, proposal *v1alpha1.TideProposal) (ctrl.Result, error) {
    // For each participant, resolve agent config and create a review K8s Job.
    agentConfigs := r.resolveAgentConfigs(proposal.Spec.Participants)
    if len(agentConfigs) == 0 {
        r.Recorder.Eventf(proposal, corev1.EventTypeWarning, "AgentConfigMissing",
            "No agent configurations found for participants %v", proposal.Spec.Participants)
        return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
    }

    var jobRefs []v1alpha1.JobReference
    for _, agent := range agentConfigs {
        jobName := fmt.Sprintf("tide-review-%s-%s", proposal.Spec.ProposalID, agent.Name)

        // Check if Job already exists (idempotency).
        existingJob := &batchv1.Job{}
        err := r.Get(ctx, client.ObjectKey{Namespace: constants.NamespaceAgents, Name: jobName}, existingJob)
        if err == nil {
            // Job exists, record ref and continue.
            jobRefs = append(jobRefs, v1alpha1.JobReference{
                AgentTokenID: agent.TokenID,
                Name:         jobName,
                Namespace:    constants.NamespaceAgents,
            })
            continue
        }
        if !apierrors.IsNotFound(err) {
            return ctrl.Result{}, err
        }

        // Build and create the review Job.
        job := r.buildReviewJob(proposal, agent, jobName)
        if err := ctrl.SetControllerReference(proposal, job, r.Scheme); err != nil {
            return ctrl.Result{}, err
        }
        if err := r.Create(ctx, job); err != nil {
            if apierrors.IsAlreadyExists(err) {
                continue // race condition, safe to ignore
            }
            r.Recorder.Eventf(proposal, corev1.EventTypeWarning, "JobCreateFailed",
                "Failed to create review Job for agent %s: %v", agent.Name, err)
            metrics.ReconcileErrors.WithLabelValues("tideproposal").Inc()
            return ctrl.Result{RequeueAfter: 30 * time.Second}, err
        }

        metrics.JobsCreated.WithLabelValues(agent.Name, "review").Inc()
        jobRefs = append(jobRefs, v1alpha1.JobReference{
            AgentTokenID: agent.TokenID,
            Name:         jobName,
            Namespace:    constants.NamespaceAgents,
        })
    }

    proposal.Status.Phase = v1alpha1.ProposalPhaseActive
    proposal.Status.ReviewJobRefs = jobRefs
    meta.SetStatusCondition(&proposal.Status.Conditions, metav1.Condition{
        Type:   "ReviewJobsLaunched",
        Status: metav1.ConditionTrue,
        Reason: "Created",
        Message: fmt.Sprintf("Created %d review Jobs", len(jobRefs)),
    })
    proposal.Status.ObservedGeneration = proposal.Generation
    return ctrl.Result{RequeueAfter: r.reconcileInterval}, r.Status().Update(ctx, proposal)
}

func (r *TideProposalReconciler) reconcileActive(ctx context.Context, proposal *v1alpha1.TideProposal) (ctrl.Result, error) {
    // Check if quorum is reached.
    if proposal.Status.ApprovalCount >= proposal.Spec.Quorum {
        meta.SetStatusCondition(&proposal.Status.Conditions, metav1.Condition{
            Type:    "QuorumReached",
            Status:  metav1.ConditionTrue,
            Reason:  "QuorumMet",
            Message: fmt.Sprintf("%d/%d approvals", proposal.Status.ApprovalCount, proposal.Spec.Quorum),
        })
        // Note: Phase transitions to Approved only when ProposalApproved event is
        // indexed (on-chain finalization). The controller does not call finalize()
        // itself — that is the principal's action.
    }

    // Periodic reconciliation: verify on-chain state matches CRD state.
    onChain, err := r.chainClient.GetProposalState(ctx,
        common.HexToAddress(proposal.Spec.CouncilContract),
        new(big.Int).SetString(proposal.Spec.ProposalID, 10))
    if err != nil {
        if errors.Is(err, interfaces.ErrProposalNotFound) {
            r.Recorder.Eventf(proposal, corev1.EventTypeWarning, "OnChainMissing",
                "Proposal %s not found on-chain", proposal.Spec.ProposalID)
        }
        // Non-fatal — retry at next reconciliation.
        return ctrl.Result{RequeueAfter: r.reconcileInterval}, nil
    }

    // Reconcile on-chain state → CRD state.
    switch onChain.Status {
    case 1: // Approved
        proposal.Status.Phase = v1alpha1.ProposalPhaseApproved
        meta.SetStatusCondition(&proposal.Status.Conditions, metav1.Condition{
            Type:   "OnChainFinalized",
            Status: metav1.ConditionTrue,
            Reason: "Approved",
        })
        return ctrl.Result{}, r.Status().Update(ctx, proposal)
    case 3: // Expired
        proposal.Status.Phase = v1alpha1.ProposalPhaseExpired
        meta.SetStatusCondition(&proposal.Status.Conditions, metav1.Condition{
            Type:   "OnChainFinalized",
            Status: metav1.ConditionTrue,
            Reason: "Expired",
        })
        return ctrl.Result{}, r.Status().Update(ctx, proposal)
    }

    // Sync review count from on-chain as safety net.
    approvalCount := int32(0)
    for _, review := range onChain.Reviews {
        if review.Verdict == 0 { // Approve
            approvalCount++
        }
    }
    if approvalCount != proposal.Status.ApprovalCount {
        proposal.Status.ApprovalCount = approvalCount
        if err := r.Status().Update(ctx, proposal); err != nil {
            return ctrl.Result{}, err
        }
    }

    proposal.Status.ObservedGeneration = proposal.Generation
    return ctrl.Result{RequeueAfter: r.reconcileInterval}, nil
}
```

### TideJob Controller

Registered with the manager watching `TideJob` and `batch/v1 Job` (with owner reference filtering).

**Reconciliation pseudocode:**

```go
func (r *TideJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    tideJob := &v1alpha1.TideJob{}
    if err := r.Get(ctx, req.NamespacedName, tideJob); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // Terminal phases — ensure cleanup, then done.
    if tideJob.Status.Phase.IsTerminal() {
        return r.ensureCleanup(ctx, tideJob)
    }

    // Check expiry before any processing.
    if time.Now().After(tideJob.Spec.ExpiresAt.Time) {
        return r.transitionToExpired(ctx, tideJob)
    }

    switch tideJob.Status.Phase {
    case "", v1alpha1.JobPhasePending:
        return r.reconcilePending(ctx, tideJob)
    case v1alpha1.JobPhaseProvisioning:
        return r.reconcileProvisioning(ctx, tideJob)
    case v1alpha1.JobPhaseRunning:
        return r.reconcileRunning(ctx, tideJob)
    case v1alpha1.JobPhaseSubmitting:
        return r.reconcileSubmitting(ctx, tideJob)
    case v1alpha1.JobPhaseSubmitted:
        return r.reconcileSubmitted(ctx, tideJob)
    }

    return ctrl.Result{}, nil
}
```

#### Pending → Provisioning

```go
func (r *TideJobReconciler) reconcilePending(ctx context.Context, tj *v1alpha1.TideJob) (ctrl.Result, error) {
    agent, err := r.resolveAgentConfig(tj.Spec.AgentTokenID)
    if err != nil {
        r.Recorder.Eventf(tj, corev1.EventTypeWarning, "AgentConfigMissing",
            "No config for agent token %d: %v", tj.Spec.AgentTokenID, err)
        return r.transitionToFailed(ctx, tj, "AgentConfigMissing",
            fmt.Sprintf("agent token %d not found in config", tj.Spec.AgentTokenID))
    }

    tj.Status.Phase = v1alpha1.JobPhaseProvisioning
    tj.Status.ObservedGeneration = tj.Generation
    return ctrl.Result{Requeue: true}, r.Status().Update(ctx, tj)
}
```

#### Provisioning → Running (or Failed)

```go
func (r *TideJobReconciler) reconcileProvisioning(ctx context.Context, tj *v1alpha1.TideJob) (ctrl.Result, error) {
    agent, _ := r.resolveAgentConfig(tj.Spec.AgentTokenID) // already validated in Pending

    // Step 1: Ensure GitHub workspace repo exists.
    repoName := fmt.Sprintf("agent-%s-job-%s", agent.Name, tj.Spec.JobID)
    repo, err := r.githubClient.EnsureRepository(ctx, interfaces.CreateRepoOpts{
        Org:          r.githubOrg,
        Name:         repoName,
        TemplateRepo: r.workspaceTemplateRepo,
        Private:      true,
        Description:  fmt.Sprintf("Workspace for agent %s, job %s", agent.Name, tj.Spec.JobID),
    })
    if err != nil {
        if errors.Is(err, interfaces.ErrGitHubRateLimit) {
            r.Recorder.Eventf(tj, corev1.EventTypeWarning, "GitHubRateLimit",
                "Rate limited during repo creation")
            metrics.TokenRefreshErrors.WithLabelValues(agent.Name).Inc()
            return ctrl.Result{RequeueAfter: 60 * time.Second}, nil // retry later
        }
        return r.transitionToFailed(ctx, tj, "SandboxProvisionFailed", err.Error())
    }

    // Step 2: Generate installation token (validates App credentials).
    repos := []string{repoName, r.proposalsRepo, r.deliverablesRepo}
    token, err := r.githubClient.GenerateInstallationToken(ctx,
        agent.GitHubAppID, agent.GitHubInstallationID, repos)
    if err != nil {
        if errors.Is(err, interfaces.ErrInstallationSuspended) {
            return r.transitionToFailed(ctx, tj, "InstallationSuspended",
                "GitHub App installation suspended for agent "+agent.Name)
        }
        if errors.Is(err, interfaces.ErrGitHubRateLimit) {
            return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
        }
        return r.transitionToFailed(ctx, tj, "TokenGenerationFailed", err.Error())
    }
    _ = token // token is short-lived; init container will generate its own

    now := metav1.Now()
    tj.Status.Sandbox = &v1alpha1.SandboxStatus{
        RepositoryURL: repo.CloneURL,
        Ready:         true,
        ProvisionedAt: &now,
    }
    meta.SetStatusCondition(&tj.Status.Conditions, metav1.Condition{
        Type:   "SandboxReady",
        Status: metav1.ConditionTrue,
        Reason: "Provisioned",
    })

    metrics.SandboxProvisioningDuration.Observe(time.Since(tj.CreationTimestamp.Time).Seconds())

    // Step 3: Create K8s Job.
    jobName := fmt.Sprintf("tide-exec-%s-%s", tj.Spec.JobID, agent.Name)
    k8sJob := r.buildExecutionJob(tj, agent, jobName, repoName)
    if err := ctrl.SetControllerReference(tj, k8sJob, r.Scheme); err != nil {
        return ctrl.Result{}, err
    }
    if err := r.Create(ctx, k8sJob); err != nil {
        if apierrors.IsAlreadyExists(err) {
            // Job already exists — record and move on.
        } else {
            r.Recorder.Eventf(tj, corev1.EventTypeWarning, "K8sJobCreateFailed",
                "Failed to create execution Job: %v", err)
            metrics.ReconcileErrors.WithLabelValues("tidejob").Inc()
            return ctrl.Result{RequeueAfter: 30 * time.Second}, err
        }
    }

    metrics.JobsCreated.WithLabelValues(agent.Name, "execution").Inc()

    startTime := metav1.Now()
    tj.Status.K8sJob = &v1alpha1.K8sJobReference{
        Name:      jobName,
        Namespace: constants.NamespaceAgents,
        Active:    1,
        StartTime: &startTime,
    }
    meta.SetStatusCondition(&tj.Status.Conditions, metav1.Condition{
        Type:   "K8sJobCreated",
        Status: metav1.ConditionTrue,
        Reason: "Created",
    })
    tj.Status.Phase = v1alpha1.JobPhaseRunning
    return ctrl.Result{RequeueAfter: 30 * time.Second}, r.Status().Update(ctx, tj)
}
```

#### Running → Submitting (or Failed)

```go
func (r *TideJobReconciler) reconcileRunning(ctx context.Context, tj *v1alpha1.TideJob) (ctrl.Result, error) {
    if tj.Status.K8sJob == nil {
        return r.transitionToFailed(ctx, tj, "InternalError", "K8sJob reference missing")
    }

    k8sJob := &batchv1.Job{}
    jobKey := client.ObjectKey{Namespace: tj.Status.K8sJob.Namespace, Name: tj.Status.K8sJob.Name}
    if err := r.Get(ctx, jobKey, k8sJob); err != nil {
        if apierrors.IsNotFound(err) {
            return r.transitionToFailed(ctx, tj, "K8sJobDeleted",
                "K8s Job was deleted externally")
        }
        return ctrl.Result{}, err
    }

    // Update K8sJob status snapshot.
    tj.Status.K8sJob.Active = k8sJob.Status.Active
    tj.Status.K8sJob.Succeeded = k8sJob.Status.Succeeded
    tj.Status.K8sJob.Failed = k8sJob.Status.Failed

    // Check for completion.
    if k8sJob.Status.Succeeded > 0 {
        result, err := r.parseAgentResult(ctx, k8sJob)
        if err != nil {
            return r.transitionToFailed(ctx, tj, "ResultParseFailed",
                fmt.Sprintf("failed to parse agent result: %v", err))
        }

        if result.Status != "success" {
            return r.transitionToFailed(ctx, tj, "AgentReportedFailure", result.Error)
        }

        tj.Status.DeliverableHash = result.DeliverableHash
        tj.Status.PRUrl = result.PRUrl
        meta.SetStatusCondition(&tj.Status.Conditions, metav1.Condition{
            Type:   "K8sJobSucceeded",
            Status: metav1.ConditionTrue,
            Reason: "AgentSuccess",
        })
        tj.Status.Phase = v1alpha1.JobPhaseSubmitting
        return ctrl.Result{Requeue: true}, r.Status().Update(ctx, tj)
    }

    // Check for failure.
    if k8sJob.Status.Failed > 0 {
        reason := r.extractFailureReason(ctx, k8sJob)
        return r.transitionToFailed(ctx, tj, reason.code, reason.message)
    }

    // Still running — check token refresh.
    if r.needsTokenRefresh(tj) {
        if err := r.refreshToken(ctx, tj); err != nil {
            metrics.TokenRefreshErrors.WithLabelValues(
                r.agentName(tj.Spec.AgentTokenID)).Inc()
            r.Recorder.Eventf(tj, corev1.EventTypeWarning, "TokenRefreshFailed",
                "GitHub token refresh failed: %v", err)
            // Non-fatal: agent may still have a valid token.
        }
    }

    return ctrl.Result{RequeueAfter: 30 * time.Second}, r.Status().Update(ctx, tj)
}

// parseAgentResult reads the termination message from the completed Pod.
func (r *TideJobReconciler) parseAgentResult(ctx context.Context, k8sJob *batchv1.Job) (*AgentResult, error) {
    // List pods owned by the Job.
    podList := &corev1.PodList{}
    if err := r.List(ctx, podList,
        client.InNamespace(k8sJob.Namespace),
        client.MatchingLabels{"job-name": k8sJob.Name},
    ); err != nil {
        return nil, fmt.Errorf("list pods: %w", err)
    }

    for _, pod := range podList.Items {
        for _, cs := range pod.Status.ContainerStatuses {
            if cs.Name != "agent" {
                continue
            }
            if cs.State.Terminated == nil {
                continue
            }
            msg := cs.State.Terminated.Message
            if msg == "" {
                return nil, fmt.Errorf("empty termination message")
            }
            var result AgentResult
            if err := json.Unmarshal([]byte(msg), &result); err != nil {
                return nil, fmt.Errorf("unmarshal result: %w (raw: %s)", err, msg)
            }
            return &result, nil
        }
    }
    return nil, fmt.Errorf("no terminated agent container found")
}
```

#### Submitting → Submitted (or Failed)

```go
func (r *TideJobReconciler) reconcileSubmitting(ctx context.Context, tj *v1alpha1.TideJob) (ctrl.Result, error) {
    if tj.Status.DeliverableHash == "" {
        return r.transitionToFailed(ctx, tj, "InternalError", "deliverable hash missing")
    }

    agent, _ := r.resolveAgentConfig(tj.Spec.AgentTokenID)

    var hash [32]byte
    deliverableBytes := common.FromHex(tj.Status.DeliverableHash)
    copy(hash[:], deliverableBytes)

    jobID, ok := new(big.Int).SetString(tj.Spec.JobID, 10)
    if !ok {
        return r.transitionToFailed(ctx, tj, "InternalError", "invalid jobID format")
    }

    start := time.Now()
    txHash, err := r.chainClient.SubmitDeliverable(ctx,
        common.HexToAddress(tj.Spec.ACPContract),
        jobID, hash, agent.KMSKeyARN)
    if err != nil {
        if errors.Is(err, interfaces.ErrNonceConflict) || errors.Is(err, interfaces.ErrRPCUnavailable) {
            // Transient — retry.
            return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
        }
        if errors.Is(err, interfaces.ErrKMSSign) {
            metrics.KMSSignErrors.Inc()
        }
        return r.transitionToFailed(ctx, tj, "SubmitFailed", err.Error())
    }
    metrics.OnChainSubmitDuration.Observe(time.Since(start).Seconds())

    tj.Status.SubmissionTxHash = txHash.Hex()
    meta.SetStatusCondition(&tj.Status.Conditions, metav1.Condition{
        Type:    "DeliverableSubmitted",
        Status:  metav1.ConditionTrue,
        Reason:  "Submitted",
        Message: "tx: " + txHash.Hex(),
    })
    tj.Status.Phase = v1alpha1.JobPhaseSubmitted
    return ctrl.Result{RequeueAfter: 60 * time.Second}, r.Status().Update(ctx, tj)
}
```

#### Submitted → Completed / Rejected / Expired

```go
func (r *TideJobReconciler) reconcileSubmitted(ctx context.Context, tj *v1alpha1.TideJob) (ctrl.Result, error) {
    // The indexer processes JobCompleted/JobRejected/RefundClaimed events and
    // updates the TideJob status directly. This reconciliation is a safety net
    // that polls on-chain state periodically in case the indexer missed an event.

    jobID, _ := new(big.Int).SetString(tj.Spec.JobID, 10)
    onChain, err := r.chainClient.GetJobState(ctx,
        common.HexToAddress(tj.Spec.ACPContract), jobID)
    if err != nil {
        if !errors.Is(err, interfaces.ErrJobNotFound) {
            // Transient error — retry later.
            return ctrl.Result{RequeueAfter: r.reconcileInterval}, nil
        }
        // Job not found on-chain — unusual, but not fatal. Wait for event.
        return ctrl.Result{RequeueAfter: r.reconcileInterval}, nil
    }

    // Map on-chain status to CRD phase.
    // ERC-8183 status enum (placeholder values — adapt to actual ABI):
    //   0=Open, 1=Funded, 2=Submitted, 3=Completed, 4=Rejected, 5=Refunded
    switch onChain.Status {
    case 3: // Completed
        tj.Status.Phase = v1alpha1.JobPhaseCompleted
        meta.SetStatusCondition(&tj.Status.Conditions, metav1.Condition{
            Type:   "OnChainTerminal",
            Status: metav1.ConditionTrue,
            Reason: "Completed",
        })
        metrics.JobsTerminal.WithLabelValues("Completed").Inc()
        return ctrl.Result{}, r.Status().Update(ctx, tj)
    case 4: // Rejected
        tj.Status.Phase = v1alpha1.JobPhaseRejected
        meta.SetStatusCondition(&tj.Status.Conditions, metav1.Condition{
            Type:   "OnChainTerminal",
            Status: metav1.ConditionTrue,
            Reason: "Rejected",
        })
        metrics.JobsTerminal.WithLabelValues("Rejected").Inc()
        return ctrl.Result{}, r.Status().Update(ctx, tj)
    case 5: // Refunded
        tj.Status.Phase = v1alpha1.JobPhaseExpired
        meta.SetStatusCondition(&tj.Status.Conditions, metav1.Condition{
            Type:   "OnChainTerminal",
            Status: metav1.ConditionTrue,
            Reason: "Refunded",
        })
        metrics.JobsTerminal.WithLabelValues("Expired").Inc()
        return ctrl.Result{}, r.Status().Update(ctx, tj)
    }

    // Still in Submitted state on-chain — waiting for evaluator.
    return ctrl.Result{RequeueAfter: r.reconcileInterval}, nil
}
```

#### Cleanup

```go
func (r *TideJobReconciler) ensureCleanup(ctx context.Context, tj *v1alpha1.TideJob) (ctrl.Result, error) {
    // Idempotent cleanup: archive repo, delete K8s Job.

    // Archive GitHub repo (makes it read-only).
    if tj.Status.Sandbox != nil && tj.Status.Sandbox.RepositoryURL != "" {
        agent, _ := r.resolveAgentConfig(tj.Spec.AgentTokenID)
        repoName := fmt.Sprintf("agent-%s-job-%s", agent.Name, tj.Spec.JobID)
        if err := r.githubClient.ArchiveRepository(ctx, r.githubOrg, repoName); err != nil {
            if !errors.Is(err, interfaces.ErrGitHubRateLimit) {
                // Log but don't fail — cleanup is best-effort.
                r.Recorder.Eventf(tj, corev1.EventTypeWarning, "ArchiveFailed",
                    "Failed to archive repo: %v", err)
            }
        }
    }

    // Delete K8s Job (with propagation to pods).
    if tj.Status.K8sJob != nil {
        k8sJob := &batchv1.Job{
            ObjectMeta: metav1.ObjectMeta{
                Name:      tj.Status.K8sJob.Name,
                Namespace: tj.Status.K8sJob.Namespace,
            },
        }
        propagation := metav1.DeletePropagationBackground
        if err := r.Delete(ctx, k8sJob, &client.DeleteOptions{
            PropagationPolicy: &propagation,
        }); err != nil && !apierrors.IsNotFound(err) {
            return ctrl.Result{RequeueAfter: 30 * time.Second}, err
        }
    }

    return ctrl.Result{}, nil
}
```

#### Generated K8s Job Spec

The `buildExecutionJob` method constructs the exact batch/v1 Job that runs in `tide-agents`:

```go
func (r *TideJobReconciler) buildExecutionJob(
    tj *v1alpha1.TideJob,
    agent *AgentConfig,
    jobName string,
    repoName string,
) *batchv1.Job {
    backoffLimit := int32(0)
    ttl := int32(300)
    activeDeadline := r.computeActiveDeadline(tj.Spec.ExpiresAt.Time)

    labels := map[string]string{
        constants.LabelManagedBy:   constants.ManagedByValue,
        constants.LabelComponent:   constants.ComponentAgent,
        constants.LabelAgentID:     agent.Name,
        constants.LabelAgentToken:  strconv.FormatUint(agent.TokenID, 10),
        constants.LabelJobID:       tj.Spec.JobID,
        constants.LabelRuntimeMode: "execution",
        constants.LabelPrincipal:   tj.Spec.ClientAddress,
    }

    annotations := map[string]string{
        constants.AnnotationDesignHash: tj.Spec.DesignHash,
        constants.AnnotationSourceTx:   tj.Spec.SourceTxHash,
    }

    workspaceRepo := fmt.Sprintf("%s/%s", r.githubOrg, repoName)

    envVars := []corev1.EnvVar{
        // Core vars (set for all Jobs)
        {Name: "TIDE_PROPOSAL_ID", Value: tj.Spec.ProposalRef},
        {Name: "TIDE_DESIGN_HASH", Value: tj.Spec.DesignHash},
        {Name: "TIDE_AGENT_TOKEN_ID", Value: strconv.FormatUint(tj.Spec.AgentTokenID, 10)},
        {Name: "TIDE_AGENT_NAME", Value: agent.Name},
        {Name: "TIDE_PROPOSALS_REPO", Value: fmt.Sprintf("%s/%s", r.githubOrg, r.proposalsRepo)},
        {Name: "TIDE_GITHUB_APP_ID", Value: strconv.FormatInt(agent.GitHubAppID, 10)},
        {Name: "TIDE_GITHUB_INSTALLATION_ID", Value: strconv.FormatInt(agent.GitHubInstallationID, 10)},
        {Name: "TIDE_PROVIDER_ADDRESS", Value: tj.Spec.ProviderAddress},
        {Name: "TIDE_EXPIRES_AT", Value: tj.Spec.ExpiresAt.Format(time.RFC3339)},
        {Name: "TIDE_SEI_RPC_URL", Value: r.seiHTTPURL},
        {Name: "TIDE_SEI_CHAIN_ID", Value: strconv.Itoa(r.seiChainID)},
        {Name: "TIDE_KMS_KEY_ARN", Value: agent.KMSKeyARN},
        {Name: "TIDE_AWS_REGION", Value: r.awsRegion},
        {Name: "TIDE_RESULT_PATH", Value: "/dev/termination-log"},
        {Name: "TIDE_RUNTIME_MODE", Value: "execution"},
        {Name: "TIDE_LOG_LEVEL", Value: r.logLevel},
        // Execution-only vars
        {Name: "TIDE_JOB_ID", Value: tj.Spec.JobID},
        {Name: "TIDE_ACP_CONTRACT", Value: tj.Spec.ACPContract},
        {Name: "TIDE_WORKSPACE_REPO", Value: workspaceRepo},
        {Name: "TIDE_WORKSPACE_BRANCH", Value: fmt.Sprintf("job-%s", tj.Spec.JobID)},
        {Name: "TIDE_DELIVERABLES_REPO", Value: fmt.Sprintf("%s/%s", r.githubOrg, r.deliverablesRepo)},
        {Name: "TIDE_DELIVERABLES_BASE_BRANCH", Value: "main"},
        {Name: "TIDE_UPSTREAM_REPO", Value: r.resolveUpstreamRepo(tj)},
        {Name: "TIDE_UPSTREAM_BRANCH", Value: r.resolveUpstreamBranch(tj)},
        {Name: "TIDE_TASK_DESCRIPTION", Value: tj.Spec.TaskDescription},
        {Name: "TIDE_CLIENT_ADDRESS", Value: tj.Spec.ClientAddress},
        {Name: "TIDE_BUDGET_RAW", Value: tj.Spec.Budget},
        {Name: "TIDE_LLM_MODEL", Value: r.llmModel},
        {Name: "TIDE_LLM_TOKEN_BUDGET", Value: r.llmTokenBudgetExecution},
        {Name: "TIDE_LLM_MAX_OUTPUT_TOKENS", Value: r.llmMaxOutputTokens},
        {Name: "TIDE_MAX_ITERATIONS", Value: r.maxIterationsExecution},
        {Name: "TIDE_EXECUTION_TIMEOUT_SECONDS", Value: r.executionTimeoutSeconds},
        {Name: "TIDE_CODING_FRAMEWORK", Value: r.codingFramework},
    }

    falseVal := false
    nonRoot := true
    user := int64(1000)
    fsGroup := int64(1000)

    return &batchv1.Job{
        ObjectMeta: metav1.ObjectMeta{
            Name:        jobName,
            Namespace:   constants.NamespaceAgents,
            Labels:      labels,
            Annotations: annotations,
        },
        Spec: batchv1.JobSpec{
            ActiveDeadlineSeconds:   &activeDeadline,
            BackoffLimit:            &backoffLimit,
            TTLSecondsAfterFinished: &ttl,
            Template: corev1.PodTemplateSpec{
                ObjectMeta: metav1.ObjectMeta{
                    Labels: labels,
                },
                Spec: corev1.PodSpec{
                    RestartPolicy:      corev1.RestartPolicyNever,
                    ServiceAccountName: fmt.Sprintf("tide-agent-%s", agent.Name),
                    AutomountServiceAccountToken: &falseVal,
                    SecurityContext: &corev1.PodSecurityContext{
                        RunAsNonRoot: &nonRoot,
                        RunAsUser:    &user,
                        FSGroup:      &fsGroup,
                        SeccompProfile: &corev1.SeccompProfile{
                            Type: corev1.SeccompProfileTypeRuntimeDefault,
                        },
                    },
                    InitContainers: []corev1.Container{{
                        Name:  "workspace-setup",
                        Image: agent.InitImage,
                        Env:   envVars,
                        Resources: corev1.ResourceRequirements{
                            Requests: corev1.ResourceList{
                                corev1.ResourceCPU:    resource.MustParse("250m"),
                                corev1.ResourceMemory: resource.MustParse("512Mi"),
                            },
                            Limits: corev1.ResourceList{
                                corev1.ResourceCPU:    resource.MustParse("1"),
                                corev1.ResourceMemory: resource.MustParse("2Gi"),
                            },
                        },
                        SecurityContext: &corev1.SecurityContext{
                            AllowPrivilegeEscalation: &falseVal,
                            Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
                            ReadOnlyRootFilesystem:   &nonRoot,
                        },
                        VolumeMounts: []corev1.VolumeMount{
                            {Name: "workspace", MountPath: "/workspace"},
                            {Name: "secrets", MountPath: "/secrets", ReadOnly: true},
                            {Name: "tmp", MountPath: "/tmp"},
                        },
                    }},
                    Containers: []corev1.Container{{
                        Name:  "agent",
                        Image: agent.ExecutionImage,
                        Env:   envVars,
                        Resources: corev1.ResourceRequirements{
                            Requests: corev1.ResourceList{
                                corev1.ResourceCPU:    resource.MustParse("500m"),
                                corev1.ResourceMemory: resource.MustParse("1Gi"),
                            },
                            Limits: corev1.ResourceList{
                                corev1.ResourceCPU:    resource.MustParse("2"),
                                corev1.ResourceMemory: resource.MustParse("4Gi"),
                            },
                        },
                        SecurityContext: &corev1.SecurityContext{
                            AllowPrivilegeEscalation: &falseVal,
                            Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
                            ReadOnlyRootFilesystem:   &nonRoot,
                        },
                        VolumeMounts: []corev1.VolumeMount{
                            {Name: "workspace", MountPath: "/workspace"},
                            {Name: "tmp", MountPath: "/tmp"},
                            {Name: "secrets", MountPath: "/secrets", ReadOnly: true},
                        },
                    }},
                    Volumes: []corev1.Volume{
                        {
                            Name: "workspace",
                            VolumeSource: corev1.VolumeSource{
                                EmptyDir: &corev1.EmptyDirVolumeSource{
                                    SizeLimit: resource.NewQuantity(10*1024*1024*1024, resource.BinarySI),
                                },
                            },
                        },
                        {
                            Name: "tmp",
                            VolumeSource: corev1.VolumeSource{
                                EmptyDir: &corev1.EmptyDirVolumeSource{
                                    SizeLimit: resource.NewQuantity(1*1024*1024*1024, resource.BinarySI),
                                },
                            },
                        },
                        {
                            Name: "secrets",
                            VolumeSource: corev1.VolumeSource{
                                CSI: &corev1.CSIVolumeSource{
                                    Driver:   "secrets-store.csi.k8s.io",
                                    ReadOnly: &nonRoot,
                                    VolumeAttributes: map[string]string{
                                        "secretProviderClass": agent.SecretProviderClass,
                                    },
                                },
                            },
                        },
                    },
                },
            },
        },
    }
}

// computeActiveDeadline returns the number of seconds between now and the
// job's on-chain expiry, capped at 3600 (1 hour). Leaves a 5-minute buffer
// before on-chain expiry to allow graceful shutdown.
func (r *TideJobReconciler) computeActiveDeadline(expiresAt time.Time) int64 {
    remaining := time.Until(expiresAt) - 5*time.Minute
    if remaining < 0 {
        remaining = 60 * time.Second // minimum 60s to allow startup
    }
    seconds := int64(remaining.Seconds())
    if seconds > 3600 {
        seconds = 3600
    }
    return seconds
}
```

The `buildReviewJob` method is structurally identical but:
- Uses `agent.ReviewImage` instead of `agent.ExecutionImage`
- Sets `TIDE_RUNTIME_MODE=review`
- Sets review-only env vars: `TIDE_COUNCIL_CONTRACT`, `TIDE_DESIGN_PATH`, `TIDE_LLM_TOKEN_BUDGET` (review budget), `TIDE_LLM_MAX_OUTPUT_TOKENS`, `TIDE_LLM_TEMPERATURE`, `TIDE_REVIEW_TIMEOUT_SECONDS`
- Omits execution-only env vars: `TIDE_JOB_ID`, `TIDE_ACP_CONTRACT`, `TIDE_WORKSPACE_REPO`, `TIDE_WORKSPACE_BRANCH`, `TIDE_DELIVERABLES_REPO`, `TIDE_DELIVERABLES_BASE_BRANCH`, `TIDE_UPSTREAM_REPO`, `TIDE_UPSTREAM_BRANCH`, `TIDE_TASK_DESCRIPTION`, `TIDE_CLIENT_ADDRESS`, `TIDE_BUDGET_RAW`, `TIDE_MAX_ITERATIONS`, `TIDE_EXECUTION_TIMEOUT_SECONDS`, `TIDE_CODING_FRAMEWORK`
- Uses `fmt.Sprintf("tide-agent-%s", agent.Name)` for `ServiceAccountName` (same per-agent SA pattern)

### Token Refresh Manager

Runs as a `manager.LeaderElectionRunnable`. Periodically refreshes GitHub App installation tokens for all TideJobs in `Running` phase.

```go
func (m *TokenRefreshManager) Start(ctx context.Context) error {
    ticker := time.NewTicker(m.refreshInterval) // default 30m
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            m.refreshAll(ctx)
        case <-ctx.Done():
            return nil
        }
    }
}

func (m *TokenRefreshManager) refreshAll(ctx context.Context) {
    jobList := &v1alpha1.TideJobList{}
    if err := m.client.List(ctx, jobList,
        client.InNamespace(constants.NamespaceSystem),
        client.MatchingFields{"status.phase": string(v1alpha1.JobPhaseRunning)},
    ); err != nil {
        return
    }

    for i := range jobList.Items {
        tj := &jobList.Items[i]
        agent, err := m.resolveAgentConfig(tj.Spec.AgentTokenID)
        if err != nil {
            continue
        }

        repoName := fmt.Sprintf("agent-%s-job-%s", agent.Name, tj.Spec.JobID)
        repos := []string{repoName, m.proposalsRepo, m.deliverablesRepo}

        _, err = m.githubClient.GenerateInstallationToken(ctx,
            agent.GitHubAppID, agent.GitHubInstallationID, repos)
        if err != nil {
            metrics.TokenRefreshErrors.WithLabelValues(agent.Name).Inc()
            continue
        }

        now := metav1.Now()
        tj.Status.TokenRefreshedAt = &now
        _ = m.client.Status().Update(ctx, tj)

        metrics.GitHubAPIRemaining.WithLabelValues(agent.Name).Set(
            float64(m.githubClient.RateLimitRemaining(agent.GitHubAppID)))
    }
}
```

Note: The token refresh generates a new installation token that the init container or agent process can fetch. For the Phase 0-2 implementation, the init container generates its own token at startup, and the `activeDeadlineSeconds` (max 1 hour) means a single token (also 1 hour lifetime) is sufficient. Token refresh becomes relevant if `activeDeadlineSeconds` exceeds 1 hour in future phases. The manager is included now because the architecture requires it and the cost is minimal.

### Reconciliation Safety Net

Both controllers use `RequeueAfter` (configurable via `TIDE_RECONCILE_INTERVAL`, default 5 minutes) for non-terminal CRs. Each reconciliation cycle:

1. Reads current CRD state from the informer cache
2. Reads on-chain state via `ChainClient.GetProposalState` / `GetJobState`
3. If states diverge, on-chain wins — CRD status is updated

This catches events missed by the indexer (e.g., due to a chain reorg that reorders logs, or a WebSocket disconnect during cursor persistence).

---

## Error Handling

Every error the operator can produce, how it is surfaced, and what to do about it.

### Event Indexer Errors

| Error | Cause | Detection | Surfacing | Remediation |
|-------|-------|-----------|-----------|-------------|
| RPC connection lost | Sei RPC node down or network partition | `SubscribeFilterLogs` returns error, or `GetLogs` fails | `tide_indexer_errors_total{error_type="rpc_unavailable"}` metric, operator log | Automatic: fall back to polling, then retry WebSocket every 30s. Manual: check Sei RPC endpoint health. |
| WebSocket disconnect | Transient network issue | Subscription error channel fires | `tide_indexer_errors_total{error_type="ws_disconnect"}` metric | Automatic: fall back to polling, retry WebSocket reconnection. |
| Event parse failure | ABI mismatch (contract upgraded with new event signature) | `abi.Unpack` returns error | `tide_indexer_errors_total{error_type="parse_failure"}` metric, operator log with raw log hex | Manual: update event topic hashes in `pkg/constants/events.go` to match new ABI. **This is a one-way door concern**: if the blockchain team changes event signatures after the indexer depends on them, the indexer breaks until updated. |
| Cursor persistence failure | K8s API server unavailable or ConfigMap conflict | `client.Update` returns error | `tide_indexer_errors_total{error_type="cursor_update"}` metric | Automatic: retry on next event. At-least-once semantics ensure no events are lost. |
| Block range exceeded | `eth_getLogs` range too large for RPC endpoint | RPC returns specific error | `tide_indexer_errors_total{error_type="block_range_exceeded"}` metric | Automatic: halve `TIDE_INDEXER_BATCH_SIZE` and retry. |
| CRD creation conflict | Concurrent write from another reconciliation | `apierrors.IsAlreadyExists` | None (expected during normal operation) | Automatic: skip creation, CR already exists. |

### TideProposal Controller Errors

| Error | Cause | Detection | Surfacing | Remediation |
|-------|-------|-----------|-----------|-------------|
| Agent config missing | Agent token ID not in `tide-agent-config` ConfigMap | ConfigMap lookup returns no match | K8s Warning Event `AgentConfigMissing` on TideProposal, requeue 30s | Manual: add agent config to ConfigMap. |
| Review Job creation failed | ResourceQuota exceeded in `tide-agents`, or PSS violation | `client.Create` returns error | K8s Warning Event `JobCreateFailed`, `tide_reconcile_errors_total{controller="tideproposal"}` metric | Manual: check namespace quota and PSS compliance. Adjust quota or fix Job spec. |
| On-chain state read failure | RPC unavailable during reconciliation safety net | `GetProposalState` returns error | Operator log (non-fatal) | Automatic: retried at next reconciliation interval. |
| Status update conflict | Concurrent CRD update (indexer + controller race) | `apierrors.IsConflict` | None (expected) | Automatic: controller-runtime requeues on conflict. |

### TideJob Controller Errors

| Error | Cause | Detection | Surfacing | Remediation |
|-------|-------|-----------|-----------|-------------|
| Agent config missing | Token ID not in ConfigMap | Lookup failure | K8s Warning Event, phase → `Failed`, `failureReason="AgentConfigMissing"` | Manual: add config. |
| GitHub rate limit | Too many API calls to GitHub | `ErrGitHubRateLimit` from GitHubClient | K8s Warning Event `GitHubRateLimit`, `tide_token_refresh_errors_total` metric, requeue 60s | Automatic: retry after rate limit reset. Manual: check for runaway token refresh loops. |
| GitHub auth failure | App private key expired or revoked | `ErrGitHubAuth` from GitHubClient | K8s Warning Event, phase → `Failed`, `failureReason="GitHubAuthFailed"` | Manual: rotate GitHub App key in Secrets Manager. |
| Installation suspended | GitHub org admin suspended the App | `ErrInstallationSuspended` from GitHubClient | Phase → `Failed`, `failureReason="InstallationSuspended"` | Manual: reinstate App installation in GitHub org settings. |
| Repo creation failed | GitHub API error (network, permissions) | `ErrGitHubAPI` from GitHubClient | Phase → `Failed`, `failureReason="SandboxProvisionFailed"` | Manual: check GitHub org permissions and API status. |
| K8s Job creation failed | ResourceQuota exceeded, PSS violation, image pull error | `client.Create` returns error | K8s Warning Event `K8sJobCreateFailed`, `tide_reconcile_errors_total` metric | Manual: check quota, PSS, and image availability. |
| K8s Job OOMKilled | Agent container exceeded memory limit | Pod status shows OOMKilled (exit code 137) | Phase → `Failed`, `failureReason="OOMKilled"` | Manual: increase memory limit in agent config, or optimize agent memory usage. |
| K8s Job deadline exceeded | `activeDeadlineSeconds` elapsed | Pod status shows DeadlineExceeded (exit code 143) | Phase → `Failed`, `failureReason="DeadlineExceeded"` | Review: was the deadline too short? Was the agent stuck? Check agent logs. |
| Agent env validation failure | Missing or invalid env var | Agent exits with code 10 | Phase → `Failed`, `failureReason="EnvValidationFailed"` | Fix Operator Job template — a required env var is missing. |
| Agent secret mount failure | Secret file missing at `/secrets/*` | Agent exits with code 11 | Phase → `Failed`, `failureReason="SecretMountFailed"` | Check SecretProviderClass and Secrets Manager. |
| Agent LLM failure | LLM API errors after retries | Agent exits with code 30 | Requeue with backoff. Check API key, rate limits. |
| Agent Sei tx reverted | On-chain submission reverted | Agent exits with code 52 | Phase → `Failed`, `failureReason` from `result.error`. Parse revert reason. |
| Agent reported failure | Agent hit unrecoverable error | Agent exits with code 1, result.status="failure" | Phase → `Failed`, `failureReason` from `result.error` | Review agent logs. Bug in agent code. |
| Result parse failure | Termination message empty, malformed, or too large (>4KB) | JSON unmarshal fails | Phase → `Failed`, `failureReason="ResultParseFailed"` | Bug in agent runtime — fix result serialization. |
| K8s Job externally deleted | Someone deleted the Job/Pod | `apierrors.IsNotFound` on Job GET | Phase → `Failed`, `failureReason="K8sJobDeleted"` | Investigate who deleted it. Check for rogue automation. |
| KMS signing failure | KMS key disabled, IAM permission denied, KMS throttled | `ErrKMSSign` from ChainClient | Phase → `Failed` (after 1 retry), `tide_kms_sign_errors_total` metric | Manual: check KMS key status and IAM policy. |
| Nonce conflict | Another transaction used the same nonce | `ErrNonceConflict` from ChainClient | Automatic retry (re-fetch nonce) | Automatic: retry once with fresh nonce. |
| Insufficient gas | Agent wallet has no SEI for gas | `ErrInsufficientGas` from ChainClient | Phase → `Failed`, `failureReason="InsufficientGas"` | Manual: fund agent wallet with SEI. |
| Transaction reverted | Contract rejected the submit() call (e.g., job already submitted, expired) | `ErrTxReverted` from ChainClient | Phase → `Failed`, `failureReason` includes revert reason | Review revert reason. May indicate an on-chain state race. |
| Archive failure (cleanup) | GitHub API error during repo archival | `ArchiveRepository` returns error | K8s Warning Event `ArchiveFailed` (non-fatal) | Manual: archive repo manually via GitHub UI. |

---

## Test Specification

### Unit Tests — Event Indexer

| Test | Setup | Action | Expected |
|------|-------|--------|----------|
| `TestProcessLog_ProposalCreated` | Mock ChainClient returns a log with `TopicProposalCreated`, fake K8s client | Call `processLog` | TideProposal CR created with correct spec fields. Phase=Pending. |
| `TestProcessLog_ProposalCreated_AlreadyExists` | Same as above but TideProposal CR already exists | Call `processLog` | No error. CR unchanged. |
| `TestProcessLog_ReviewSubmitted` | Existing TideProposal in Active phase | Call `processLog` with `TopicReviewSubmitted` (verdict=Approve) | `status.reviews` updated. `approvalCount` incremented. |
| `TestProcessLog_ProposalApproved` | Existing TideProposal in Active phase | Call `processLog` with `TopicProposalApproved` | Phase → Approved. Condition `OnChainFinalized=True`. |
| `TestProcessLog_ProposalRejected` | Existing TideProposal in Active phase | Call `processLog` with `TopicProposalRejected` | Phase → Rejected. Condition `OnChainFinalized=True`. |
| `TestProcessLog_ProposalExpired` | Existing TideProposal in Active phase | Call `processLog` with `TopicProposalExpired` | Phase → Expired. Condition `OnChainFinalized=True`. |
| `TestProcessLog_SandboxProvisionRequested` | Mock K8s client, no existing TideJob | Call `processLog` with `TopicSandboxProvisionRequested` | TideJob CR created with correct spec. Phase=Pending. |
| `TestProcessLog_JobCompleted` | Existing TideJob in Submitted phase | Call `processLog` with `TopicJobCompleted` | Phase → Completed. Condition `OnChainTerminal=True`. |
| `TestProcessLog_UnknownTopic` | Any state | Call `processLog` with unrecognized topic | No error. No state change. |
| `TestCursorPersistence` | ConfigMap exists with block=100 | Process log at block 105 | ConfigMap updated to block=105. |
| `TestCursorPersistence_ConfigMapMissing` | No ConfigMap exists | Process log at block 50 | ConfigMap created with block=50. |
| `TestPollingFallback` | WebSocket subscription fails | Start indexer | Indexer enters polling mode. `tide_indexer_errors_total{error_type="ws_disconnect"}` incremented. |

### Unit Tests — TideProposal Controller

| Test | Setup | Action | Expected |
|------|-------|--------|----------|
| `TestReconcile_Pending_LaunchesReviewJobs` | TideProposal in Pending, 3 participants in agent config | Reconcile | 3 batch/v1 Jobs created in `tide-agents`. Phase → Active. Condition `ReviewJobsLaunched=True`. |
| `TestReconcile_Pending_AgentConfigMissing` | TideProposal in Pending, 1 of 3 agents missing from config | Reconcile | Warning Event emitted. Requeue after 30s. Phase stays Pending. |
| `TestReconcile_Pending_JobAlreadyExists` | TideProposal in Pending, 1 Job already exists | Reconcile | Only 2 new Jobs created. No error on existing Job. |
| `TestReconcile_Active_QuorumReached` | TideProposal in Active, `approvalCount >= quorum` | Reconcile | Condition `QuorumReached=True`. Phase stays Active (waits for on-chain finalize). |
| `TestReconcile_Active_OnChainApproved` | TideProposal in Active, mock ChainClient returns status=Approved | Reconcile | Phase → Approved. Condition `OnChainFinalized=True`. |
| `TestReconcile_Active_OnChainExpired` | TideProposal in Active, mock ChainClient returns status=Expired | Reconcile | Phase → Expired. |
| `TestReconcile_Expired_BeforeLaunch` | TideProposal in Pending, `expiresAt` in the past | Reconcile | Phase → Expired. No Jobs created. |
| `TestReconcile_Terminal_NoOp` | TideProposal in Approved | Reconcile | No state change. Result has no requeue. |

### Unit Tests — TideJob Controller

| Test | Setup | Action | Expected |
|------|-------|--------|----------|
| `TestReconcile_Pending_ToProvisioning` | TideJob in Pending, agent config exists | Reconcile | Phase → Provisioning. |
| `TestReconcile_Pending_AgentMissing` | TideJob in Pending, no agent config | Reconcile | Phase → Failed. `failureReason="AgentConfigMissing"`. |
| `TestReconcile_Provisioning_Success` | TideJob in Provisioning, mock GitHub returns repo + token | Reconcile | Sandbox status set. K8s Job created. Phase → Running. |
| `TestReconcile_Provisioning_RateLimit` | Mock GitHub returns `ErrGitHubRateLimit` | Reconcile | Phase stays Provisioning. Requeue after 60s. |
| `TestReconcile_Provisioning_AuthFailed` | Mock GitHub returns `ErrGitHubAuth` | Reconcile | Phase → Failed. |
| `TestReconcile_Running_JobSucceeded` | K8s Job has `succeeded=1`, pod has termination message with valid result | Reconcile | DeliverableHash set. Phase → Submitting. |
| `TestReconcile_Running_JobFailed_OOM` | K8s Job has `failed=1`, pod exit code 137 | Reconcile | Phase → Failed. `failureReason="OOMKilled"`. |
| `TestReconcile_Running_JobFailed_AgentError` | K8s Job `failed=1`, exit 1, result.status="failure", result.error="LLM error" | Reconcile | Phase → Failed. `failureReason="LLM error"`. |
| `TestReconcile_Running_JobDeleted` | K8s Job not found | Reconcile | Phase → Failed. `failureReason="K8sJobDeleted"`. |
| `TestReconcile_Running_ResultParseFailure` | K8s Job succeeded, termination message is not valid JSON | Reconcile | Phase → Failed. `failureReason="ResultParseFailed"`. |
| `TestReconcile_Submitting_Success` | Mock ChainClient.SubmitDeliverable returns txHash | Reconcile | `submissionTxHash` set. Phase → Submitted. |
| `TestReconcile_Submitting_NonceConflict` | Mock returns `ErrNonceConflict` | Reconcile | Requeue after 10s. Phase stays Submitting. |
| `TestReconcile_Submitting_KMSFailure` | Mock returns `ErrKMSSign` | Reconcile | Phase → Failed. `tide_kms_sign_errors_total` incremented. |
| `TestReconcile_Submitted_Completed` | Mock ChainClient.GetJobState returns status=Completed | Reconcile | Phase → Completed. `tide_jobs_terminal_total{phase="Completed"}` incremented. |
| `TestReconcile_Submitted_Rejected` | Mock returns status=Rejected | Reconcile | Phase → Rejected. |
| `TestReconcile_Submitted_StillWaiting` | Mock returns status=Submitted (not terminal) | Reconcile | Phase stays Submitted. Requeue after interval. |
| `TestReconcile_AnyPhase_Expired` | TideJob in Running, `expiresAt` in the past | Reconcile | Phase → Expired. K8s Job deleted (cleanup). |
| `TestCleanup_ArchivesRepo` | TideJob in Completed, sandbox has repoURL | `ensureCleanup` | `ArchiveRepository` called. K8s Job deleted. |
| `TestCleanup_AlreadyClean` | TideJob in Completed, no K8s Job exists | `ensureCleanup` | No error. |

### Integration Tests — CRD Lifecycle

Run against envtest (in-memory K8s API server with CRDs installed).

| Test | Scenario |
|------|----------|
| `TestTideProposalFullLifecycle` | Create TideProposal → reconcile to Active → simulate ReviewSubmitted events → quorum reached → simulate ProposalApproved → verify terminal state |
| `TestTideJobFullLifecycle` | Create TideJob → reconcile through Pending → Provisioning → Running → simulate Job success → Submitting → Submitted → simulate JobCompleted → verify Completed phase and cleanup |
| `TestTideJobExpiry` | Create TideJob with past expiresAt → verify it transitions directly to Expired |
| `TestIndexerCreatesProposalCR` | Feed raw log to indexer → verify TideProposal CR created correctly |
| `TestIndexerCreatesTideJobCR` | Feed SandboxProvisionRequested log to indexer → verify TideJob CR created correctly |
| `TestIdempotentCRCreation` | Feed same log twice → verify no duplicate CRs, no error |
| `TestConcurrentReconciliation` | Trigger reconcile + indexer update concurrently → verify no data corruption (optimistic concurrency) |

---

## Deployment

### Repository Layout

```
tide-operator/
├── api/
│   └── v1alpha1/
│       ├── groupversion_info.go
│       ├── tideproposal_types.go
│       ├── tidejob_types.go
│       └── zz_generated.deepcopy.go    # generated
├── cmd/
│   └── main.go
├── internal/
│   ├── adapter/
│   │   ├── chain/
│   │   │   ├── client.go               # ChainClient implementation (go-ethereum)
│   │   │   ├── events.go               # ABI decoding for each event type
│   │   │   └── signer.go               # KMS-based transaction signing
│   │   ├── github/
│   │   │   └── client.go               # GitHubClient implementation (go-github)
│   │   └── kms/
│   │       └── client.go               # KMSClient implementation (aws-sdk-go-v2)
│   ├── controller/
│   │   ├── tideproposal/
│   │   │   └── controller.go
│   │   └── tidejob/
│   │       └── controller.go
│   └── indexer/
│       └── indexer.go
├── pkg/
│   ├── constants/
│   │   ├── addresses.go
│   │   ├── chain.go
│   │   ├── events.go
│   │   ├── labels.go
│   │   └── secrets.go
│   └── interfaces/
│       ├── chain.go
│       ├── github.go
│       └── kms.go
├── config/
│   ├── crd/
│   │   └── bases/                       # generated CRD YAML
│   ├── rbac/
│   │   ├── role.yaml                    # generated
│   │   ├── role_binding.yaml
│   │   └── service_account.yaml
│   ├── manager/
│   │   ├── manager.yaml                 # Deployment spec
│   │   └── configmap.yaml               # tide-agent-config
│   └── default/
│       └── kustomization.yaml
├── Dockerfile
├── Makefile
└── go.mod
```

### Dockerfile

```dockerfile
FROM golang:1.22 AS builder
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o tide-operator ./cmd/main.go

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/tide-operator .
USER 65532:65532
ENTRYPOINT ["/tide-operator"]
```

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make generate` | Run controller-gen to generate deepcopy methods |
| `make manifests` | Generate CRD YAML, RBAC YAML from kubebuilder markers |
| `make test` | Run unit tests with envtest (downloads API server binaries) |
| `make build` | Build the Go binary |
| `make docker-build IMG=<tag>` | Build Docker image |
| `make docker-push IMG=<tag>` | Push Docker image to ECR |
| `make install` | Install CRDs into current cluster (`kubectl apply -f config/crd/bases/`) |
| `make deploy IMG=<tag>` | Deploy operator via Kustomize |
| `make undeploy` | Remove operator from cluster |

### Kustomize Deployment

`config/manager/manager.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tide-operator
  namespace: tide-system
  labels:
    app.kubernetes.io/name: tide-operator
    app.kubernetes.io/managed-by: kustomize
spec:
  replicas: 2
  selector:
    matchLabels:
      app.kubernetes.io/name: tide-operator
  template:
    metadata:
      labels:
        app.kubernetes.io/name: tide-operator
    spec:
      serviceAccountName: tide-operator
      securityContext:
        runAsNonRoot: true
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: manager
          image: OPERATOR_IMAGE
          args:
            - --leader-elect
          ports:
            - containerPort: 8080
              name: metrics
            - containerPort: 8081
              name: health
          livenessProbe:
            httpGet:
              path: /healthz
              port: health
            initialDelaySeconds: 15
            periodSeconds: 20
          readinessProbe:
            httpGet:
              path: /readyz
              port: health
            initialDelaySeconds: 5
            periodSeconds: 10
          resources:
            requests:
              cpu: 250m
              memory: 256Mi
            limits:
              cpu: "1"
              memory: 512Mi
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
            readOnlyRootFilesystem: true
          envFrom:
            - configMapRef:
                name: tide-operator-config
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                topologyKey: kubernetes.io/hostname
                labelSelector:
                  matchLabels:
                    app.kubernetes.io/name: tide-operator
```

### RBAC

The operator's ServiceAccount requires these permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: tide-operator-role
rules:
  # CRD management
  - apiGroups: ["tide.sei.io"]
    resources: ["tideproposals", "tidejobs"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["tide.sei.io"]
    resources: ["tideproposals/status", "tidejobs/status"]
    verbs: ["get", "update", "patch"]
  - apiGroups: ["tide.sei.io"]
    resources: ["tideproposals/finalizers", "tidejobs/finalizers"]
    verbs: ["update"]

  # K8s Job management in tide-agents
  - apiGroups: ["batch"]
    resources: ["jobs"]
    verbs: ["get", "list", "watch", "create", "delete"]

  # Pod reading (for termination messages)
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list"]

  # ConfigMap management (event cursor + agent config)
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "list", "watch", "create", "update"]

  # Events (status reporting)
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create", "patch"]

  # Leader election
  - apiGroups: ["coordination.k8s.io"]
    resources: ["leases"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: tide-operator-rolebinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: tide-operator-role
subjects:
  - kind: ServiceAccount
    name: tide-operator
    namespace: tide-system
```

**Namespace scoping note:** The ClusterRole is necessary because the operator manages CRDs in `tide-system` and Jobs in `tide-agents` (two namespaces). However, CRD verbs and Job verbs could be split into two namespace-scoped Roles with corresponding RoleBindings for tighter scoping. This is a two-way door — start with ClusterRole for simplicity, split later if required by security review.

### Testnet vs Mainnet Differences

| Parameter | Testnet (arctic-1) | Mainnet |
|-----------|-------------------|---------|
| `TIDE_SEI_CHAIN_ID` | `713715` | `1329` |
| `TIDE_SEI_WS_URL` | `wss://ws.arctic-1.seinetwork.io` | `wss://ws.evm-rpc.sei.io` |
| `TIDE_SEI_HTTP_URL` | `https://evm-rpc-arctic-1.sei-apis.com` | `https://evm-rpc.sei.io` |
| Contract addresses | Testnet deployments | Mainnet deployments |
| `TIDE_INDEXER_START_BLOCK` | Testnet deployment block | Mainnet deployment block |
| Operator replicas | 1 (cost savings) | 2 (HA) |

All differences are configuration-only. No code changes between testnet and mainnet.

---

## Decision Log

| # | Decision | Rationale | Reversibility |
|---|----------|-----------|---------------|
| 1 | CRD API group `tide.sei.io/v1alpha1` | Matches label prefix convention. `v1alpha1` signals instability appropriate for Phase 0-2. | Semi-one-way: field names in `spec` are hard to change after controllers depend on them. `v1alpha1` allows breaking changes before `v1beta1`. |
| 2 | Termination message for completion signaling | Kubernetes-native (no extra infrastructure), 4KB is sufficient for result JSON, controller reads from Pod status API (no PVC or sidecar needed). | Two-way: can switch to PVC or ConfigMap-based signaling by changing `TIDE_RESULT_PATH` and controller parsing logic. |
| 3 | CRD names derived from on-chain IDs (`proposal-{id}`, `job-{id}`) | Provides natural idempotency key for the indexer. Prevents duplicate CRs on replay. | Two-way: naming convention can change without affecting stored data (old CRs remain). |
| 4 | Agent config in ConfigMap (not a CRD) | 3 agents in Phase 0-2 — a ConfigMap is simpler. Flux reconciles it from Git. | Two-way: can migrate to a `TideAgent` CRD later without affecting existing TideJob/TideProposal CRDs. |
| 5 | Single binary with controller-runtime Manager | Simplifies deployment (one Deployment, one image). Leader election and health probes come for free. Indexer and token refresh run as Manager Runnables. | Two-way: can split into separate binaries later if resource or scaling needs diverge. |
| 6 | WebSocket primary, HTTP polling fallback | WebSocket gives near-real-time event delivery. HTTP polling is a reliable fallback for when WS is unavailable (load balancer timeouts, Sei node restarts). | Two-way: transport is an implementation detail of ChainClient. |
| 7 | `activeDeadlineSeconds` capped at 3600s (1 hour) | Matches GitHub App token lifetime (1 hour). Prevents runaway agent Jobs. Agents should complete coding tasks well within this window. | Two-way: configurable per-agent or per-job in future phases. |

---

## Deferred (Do Not Build)

| Feature | Rationale |
|---------|-----------|
| `TideAgent` CRD | 3 agents in Phase 0-2; ConfigMap is sufficient. Migrate when agent count exceeds ~10. |
| Automated evaluator integration | Phase 0-2 uses human evaluation. Evaluator contracts are a Phase 3+ concern. |
| Dashboard / read-only UI | No business need in Phase 0-2. On-chain state is queryable via SeiScan; CRD state via kubectl. |
| Agent onboarding CLI | Manual provisioning is acceptable for 3 agents. Phase 3 concern. |
| Multi-chain event indexing | Only Sei is supported. Multi-chain is a Phase 3+ concern. |
| Webhook admission controller for CRDs | Spec immutability is enforced by convention (indexer is the only writer). Add a validating webhook if untrusted actors gain CRD write access. |
| CRD finalizers for guaranteed cleanup | Phase 0-2 cleanup is best-effort (archive repo, delete Job). Finalizers add complexity; add them if cleanup SLA becomes critical. |
| Horizontal scaling of controllers | 2 replicas with leader election is sufficient for Phase 0-2 throughput (< 20 concurrent jobs). Shard by CRD namespace if scaling is needed. |
| OpenTelemetry tracing | Prometheus metrics and structured logs are sufficient for Phase 0-2 observability. Add OTel spans when end-to-end trace visibility is needed. |
| Reputation-gated job assignment | TideJobHook enforces reputation on-chain during `beforeAction(fund)`. The operator does not need to duplicate this check. |
| Token-weighted voting | Not a Phase 0-2 business need. Council uses simple quorum. |
