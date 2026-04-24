# Tide Agent Council

This repository hosts specialist agent personas in `.claude/agents/`. Portable agents are general-purpose and sync to other repos and user-level via `scripts/sync-agents.sh`; Tide-specific agents stay here.

## Council Roster

| Agent | Scope | Notes |
|-------|-------|-------|
| `blockchain-developer` | Tide-only | Owns TideCouncil + TideJobHook. Full Tide-specific persona (Sei EVM, chain ID 1329, specific ERC-8004/8001/8183 choices, contract file paths). |
| `reviewer` | Tide-only | Cross-review with Tide's specific interface checklist (event sigs, env vars, exit codes, SecretProviderClass names). |
| `kubernetes-specialist` | Portable | General Go + controller-runtime. Tide-specific responsibilities live in this file under "Tide-Specific Agent Context". |
| `platform-engineer` | Portable | General platform + runtime. Same pattern. |
| `solidity-developer` | Portable | General Solidity / Foundry / OpenZeppelin / ERC standards — counterpart to the Tide-specific `blockchain-developer`. |
| `network-specialist` | Portable | General K8s and cloud networking. |
| `sei-network-specialist` | Sei-ecosystem | Sei node networking (seid ports, CometBFT P2P, Waterway, Istio quirks). Valuable to any Sei-adjacent work. |
| `security-specialist` | Portable | General security + adversarial design. |
| `tee-specialist` | Portable | General TEE + attestation. |
| `product-engineer` | Portable | General product-engineering. |
| `product-manager` | Portable | General PM. |
| `opentelemetry-expert` | Portable | General OpenTelemetry. |

## Working Agreement

All design work follows the constitution at `design/constitution/constitution.md`. Key principles:

1. **Two-Way Doors Only** — every decision must be reversible without significant rework
2. **YAGNI** — if it's not required by Phase 0–2 business needs, exclude it
3. **Interfaces First** — exact signatures, types, events, env vars, exit codes
4. **Errors Are Interface** — every error condition is documented
5. **Tests Prove Interfaces** — concrete test specs for every interface

## Cross-Component Interfaces

The most critical contracts between components. The **provider owns** the interface; consumers adapt.

| Interface | Provider | Consumer(s) |
|-----------|----------|-------------|
| On-chain events (topic hashes, indexed fields) | Solidity contracts | Tide Operator |
| K8s Job env vars, volume mounts, exit codes | Tide Operator | Agent runtimes |
| Completion signaling (termination messages) | Agent runtimes | Tide Operator |
| EIP-712 type hashes, domain separator | TideCouncil contract | Review runtime |
| `/secrets` file layout | K8s SecretProviderClass | Agent runtimes |

## Canonical Env Var Names

Runtime convention wins (runtimes are the consumers):

- `TIDE_KMS_KEY_ARN` — not KEY_ID
- `TIDE_COUNCIL_CONTRACT` — not ADDRESS
- `TIDE_ACP_CONTRACT` — not ADDRESS
- `TIDE_GITHUB_INSTALLATION_ID` — not APP_INSTALLATION_ID

## Tide-Specific Agent Context

When portable agents are dispatched in this repo, they apply the following Tide-specific context on top of their generic persona.

### kubernetes-specialist → Tide Operator

**Component owned:** Tide Operator — Go binary bridging on-chain events to Kubernetes workloads.

**Tide-specific responsibilities:**
1. Index on-chain events from TideCouncil and TideJobHook using `blockchain-developer`'s exact topic hashes (from the interface registry)
2. Reconcile CRD state machines (Proposed → Reviewing → Approved, Provisioning → Running → Submitting → Completed)
3. Generate K8s Jobs with correct env vars, volume mounts, labels, and per-agent ServiceAccounts
4. Parse agent completion from Kubernetes termination messages (`/dev/termination-log`)
5. Handle Tide's granular exit codes (0, 1, 2, 10–52, 137, 143) per the Operator's retry/fail logic

**Interface contracts:**
- Consumes from `blockchain-developer`: Event signatures, indexed fields, ABI types
- Provides to runtimes: Env vars (runtime naming convention wins), volume mounts, labels
- Consumes from runtimes: Exit codes, `AgentResult` JSON in termination messages
- Consumes from K8s manifests: Namespace names, ServiceAccount names (`tide-agent-{name}`), NetworkPolicy selectors

**Key specs:** `design/milestones/m1-platform/lld-tide-operator.md`

**Code location:** `pkg/controller/`, `pkg/indexer/`, `pkg/constants/`, `api/v1alpha1/`

### platform-engineer → Tide platform + runtimes

**Components owned:** K8s platform manifests, agent review runtime, agent execution runtime.

**Tide-specific responsibilities:**
1. Define and maintain the K8s platform: namespaces, RBAC, quotas, network policies, secret management
2. Build the review runtime: LLM review generation, git operations, EIP-712 attestation, on-chain submission
3. Build the execution runtime: coding agent orchestration, test iteration, PR creation, deliverable submission
4. Define the completion signaling protocol: termination messages (primary), `status.json` (advisory only)
5. Define Tide's granular exit codes (0, 1, 2, 10–52, 137, 143) consumed by the Operator

**Interface contracts:**
- Provides to Operator: Exit codes, `AgentResult` JSON schema (written to `TIDE_RESULT_PATH`)
- Consumes from Operator: Env vars (must match canonical names in the registry), volume mounts, labels
- Consumes from `blockchain-developer`: `submitReview()` (6 params), `getReviewNonce()`, `submit()` function signatures
- Provides to K8s platform: SecretProviderClass file layout (`/secrets/*`)

**Secret file paths:**
- Review runtime: `/secrets/agent-system-prompt.txt`
- Execution runtime: `/secrets/agent-execution-system-prompt.txt`

**Key specs:**
- `design/milestones/m1-platform/lld-k8s-manifests.md`
- `design/milestones/m2-review/lld-agent-review-runtime.md`
- `design/milestones/m3-execution/lld-agent-execution-runtime.md`

**Code location:** `runtimes/review/`, `runtimes/execution/`, `manifests/base/`, `manifests/overlays/testnet/`, `manifests/overlays/mainnet/`

### network-specialist → Tide network boundaries

**Tide-specific namespaces:** `tide-system`, `tide-agents`, `tide-runners`.

**Key security boundaries:**
- **tide-runners namespace**: Default deny-all, HTTPS-only egress (port 443), DNS restricted to kube-system, IMDS (169.254.169.254) blocked, private ranges (10/8, 172.16/12, 192.168/16) blocked, ARC controller ingress allowed from gha-system
- **tide-system namespace**: Chain indexer needs egress to Sei RPC (HTTPS) and GitHub API (HTTPS)
- **Cross-namespace**: Runner pods MUST NOT communicate with pods in other namespaces. The ARC controller in gha-system is the only exception.

**Key files:**
- `manifests/base/runners/network-policy.yaml` — tide-runners NetworkPolicies
- `manifests/base/network-policies.yaml` — tide-agents NetworkPolicies
- Platform repo: `clusters/dev/tide-runners/` — Flux-managed runner infrastructure

**Platform context:**
- EKS cluster in us-east-2 with VPC-CNI
- ingress-nginx with cert-manager and external-dns (`*.dev.platform.sei.io`)
- Istio service mesh available (`istio-system` namespace)
- GHA runner scale sets managed by Actions Runner Controller
- Flux GitOps — all manifests in `sei-protocol/platform` under `clusters/dev/`

For Sei node networking (seid ports, CometBFT P2P, Waterway, Istio quirks), dispatch `sei-network-specialist` instead.

### blockchain-developer → TideCouncil + TideJobHook

Full persona is Tide-specific. See `.claude/agents/blockchain-developer.md`. Does NOT sync to other repos — consumers of general Solidity expertise should use `solidity-developer` instead.

### reviewer → Tide interface cross-review

Checklist items are Tide-specific. See `.claude/agents/reviewer.md`. Does NOT sync to other repos. The `/council` skill's own cross-review prompt is the fallback for repos without a dedicated reviewer.

## Sync Script

`scripts/sync-agents.sh` copies portable agents to other `.claude/agents/` directories.

```bash
# Mirror portable agents to user-level (available in any CWD)
./scripts/sync-agents.sh --target ~/

# Copy portable + sei agents to a sibling repo
./scripts/sync-agents.sh --target ~/tide-workspace/platform --categories portable,sei

# See what would be copied
./scripts/sync-agents.sh --target ~/ --dry-run
```

Categories: `portable` (default), `sei`, `tide-only`, `all`.

The script is non-destructive by default — it refuses to overwrite existing files in the target unless `--force` is passed. Use `--force` when rolling out updates after editing an agent in Tide.

## How to Use These Agents

Agent personas are dispatched by the `/council` and `/coral` skills (see `CLAUDE.md`). For direct single-expert consultations, use the Agent tool with the agent name as `subagent_type`.
