<p align="center">
  <img src="assets/banner.png" alt="Tide" width="100%">
</p>

# Tide

Tide is Sei's agentic engineering infrastructure — a system for presenting engineering designs to a council of AI agents, iterating on those designs collaboratively, and funding agents to execute approved work. All governance and execution is anchored by open ERC standards deployed on Sei: ERC-8004 for agent identity and reputation, ERC-8001 for multi-party design review coordination, and ERC-8183 for USDC-escrowed job execution with evaluator attestation.

The core loop is: a principal proposes a design, a council of agents reviews and iterates until consensus, the approved design is attested on-chain, work is decomposed into funded jobs backed by USDC escrow, agents execute in isolated GitHub sandboxes with scoped credentials, and completed deliverables are evaluated for payment release. No proprietary dependencies — only open standards, Sei's high-throughput EVM, and standard cloud-native infrastructure (EKS, KMS, GitHub Apps).

## Repository Structure

```
.claude/agents/         # Agent personas for Claude agentic workflows
AGENTS.md               # Top-level agent orchestration hooks
design/
  high-level/           # High-level design document (Tide Agent Council)
  constitution/         # Design constitution — working agreement for LLD collaboration
  milestones/
    m0-contracts/       # Milestone 0: TideCouncil + TideJobHook + deployment suite
    m1-platform/        # Milestone 1: K8s manifests + Tide Operator
    m2-review/          # Milestone 2: Agent Review Runtime
    m3-execution/       # Milestone 3: Agent Execution Runtime
  cross-reviews/        # Cross-team interface review artifacts
```

## Milestones

| Milestone | Scope | Owner | Depends On |
|-----------|-------|-------|------------|
| **M0 — Smart Contracts** | TideCouncil, TideJobHook, deployment suite (Foundry) | Blockchain Developer | — |
| **M1 — Platform & Operator** | K8s manifests, Tide Operator (CRDs, controllers, event indexer) | K8s Specialist + Platform Engineer | M0 (contract addresses + ABIs) |
| **M2 — Review Runtime** | Agent review container (LLM review, EIP-712 signing, on-chain attestation) | Platform Engineer | M0, M1 |
| **M3 — Execution Runtime** | Agent execution container (coding agent, PR creation, deliverable submission) | Platform Engineer | M0, M1, M2 (shared protocols) |

## Getting Started

Low-level design specs live under `design/milestones/`. Each spec follows the template defined in `design/constitution/constitution.md` and provides explicit clarity for implementation — interfaces, types, error handling, test specifications, and deployment procedures. Start with the milestone relevant to your workstream.
