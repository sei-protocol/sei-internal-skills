# Tide Design Constitution

**Date:** 2026-03-21
**Status:** Active

---

## Purpose

This document governs how the Tide engineering council collaborates to produce low-level component designs. Each design must give a team of mid-level engineers explicit clarity on what to build — no ambiguity in interfaces, no unstated assumptions, no features without a business need.

---

## Principles

### 1. Two-Way Doors Only

Every design decision must be reversible without significant rework. One-way doors require explicit callout and justification.

**One-way doors in this system** (require extra scrutiny):
- On-chain storage layout in upgradeable contracts (slot positions are permanent)
- Solidity event signatures after indexers depend on them
- CRD `spec` field names after controllers depend on them
- EIP-712 type hashes after wallets have signed with them

**Everything else is a two-way door** — function bodies, thresholds, config values, container images, K8s labels, CLI flags. Design for change.

### 2. YAGNI — You Aren't Gonna Need It

If a feature is not required by the Phase 0–2 business needs below, exclude it. Document it in the "Deferred" section with one line explaining why.

**Phase 0–2 business needs (the ONLY valid justifications for features):**
1. Present a design to a council of agents and collect structured feedback (Phase 1)
2. Reach quorum consensus and attest the approved design on-chain (Phase 1)
3. Decompose an approved design into funded USDC-escrowed jobs (Phase 2)
4. Execute jobs in isolated GitHub sandboxes with scoped credentials (Phase 2)
5. Evaluate deliverables and release USDC to agent wallets (Phase 2)

**Explicitly deferred (Phase 3+):**
- Reputation gating on council membership
- Automated evaluator contracts (human evaluation first)
- Agent onboarding CLI (manual provisioning is fine for 3 agents)
- Dashboard / read-only UI
- Marketplace / bidding features
- Multi-chain support
- Token-weighted voting
- Autonomous governance (principal retains control)

### 3. Interfaces First

The primary deliverable of each design is its **interfaces** — the exact signatures, types, events, env vars, exit codes, and error conditions that other components depend on. Implementation guidance is secondary.

**Test: if two engineers could read the interface spec and independently build compatible components, the design is good enough.**

### 4. Errors Are Interface

Every error a component can produce is part of its public interface. Callers must know: what can fail, how failures are surfaced (revert reason, exit code, error type), and what the caller should do about it.

### 5. Tests Prove Interfaces

Each design includes concrete test cases that verify the interface works. If you can't write the test spec, the interface isn't clear enough.

### 6. One Business Need → One Feature

Every feature must trace to one of the five Phase 0–2 business needs listed above. If it doesn't trace, it's deferred. No "nice to have" in the LLDs.

---

## Component Inventory

| # | Component | Owner | Phase | Output Files |
|---|-----------|-------|-------|-------------|
| 1 | TideCouncil Contract | Blockchain Developer | 0 | `lld-tide-council.md` |
| 2 | TideJobHook Contract | Blockchain Developer | 0 | `lld-tide-job-hook.md` |
| 3 | Contract Deployment Suite | Blockchain Developer | 0 | `lld-contract-deployment.md` |
| 4 | Tide Operator | Kubernetes Specialist | 0.7–2 | `lld-tide-operator.md` |
| 5 | Agent Review Runtime | Platform Engineer | 1 | `lld-agent-review-runtime.md` |
| 6 | Agent Execution Runtime | Platform Engineer | 2 | `lld-agent-execution-runtime.md` |
| 7 | K8s Platform Manifests | Platform Engineer | 0.7 | `lld-k8s-manifests.md` |

---

## Design Template

Every low-level design follows this structure. No sections may be omitted — if a section doesn't apply, write "N/A" with a one-line explanation.

```
# Component: [Name]

## Owner
Role/team that builds this.

## Phase
Which implementation phase this belongs to.

## Purpose
One paragraph: what this component does, why it exists, and which business need(s) it serves.

## Dependencies
- External systems consumed (with exact interface reference)
- Internal Tide components consumed (with exact interface reference)
- Explicit exclusions: what this component does NOT depend on

## Interface Specification
Every function, event, type, env var, or API this component exposes.
Every error condition. Every configuration parameter.
Use exact types — no pseudotypes, no "TBD."

## State Model
What state exists, where it lives (on-chain, CRD, ConfigMap, filesystem),
how it transitions, and what the source of truth is.
Use Mermaid `stateDiagram-v2` for state machines. No ASCII art.

## Internal Design
How the component works internally. Enough for a mid-level engineer to implement.
Pseudocode or Mermaid diagrams (sequence, flowchart, state) for non-trivial logic.
No ASCII art — always use Mermaid for visual representations.

## Error Handling
Every error case: what causes it, how it's detected, how it's surfaced,
what the caller/operator should do.

## Test Specification
Concrete test cases: name, setup, action, expected result.
Interface boundary tests first. Integration tests for cross-component flows.

## Deployment
How this component is built, packaged, and deployed.
Testnet vs mainnet differences.

## Deferred (Do Not Build)
Features explicitly excluded with one-line rationale tracing to YAGNI.
```

---

## Cross-Component Interfaces

These interfaces are the most critical — they must be byte-identical across consuming and providing components. Each interface is **owned by its provider**. Consumers adapt.

| Interface | Provider | Consumer(s) | What Must Match |
|-----------|----------|-------------|-----------------|
| On-chain events → Event indexer | Solidity contracts | Tide Operator | Event signatures, indexed fields, topic hashes |
| CRD spec → Job template | CRD types (Operator) | Operator controller | CRD field names/types ↔ Job env vars |
| Controller → Agent runtime | Operator controller | Agent container | Env vars, volume mounts, exit codes |
| Agent runtime → Controller | Agent container | Operator controller | Completion signaling mechanism (file, exit code, or API) |
| TideCouncil events → Operator | TideCouncil contract | Tide Operator | `ProposalCreated`, `ProposalApproved` event ABIs |
| TideJobHook events → Operator | TideJobHook contract | Tide Operator | `SandboxProvisionRequested`, `JobCompleted` event ABIs |
| ERC-8183 hooks → TideJobHook | ERC-8183 ACP contract | TideJobHook | `IACPHook` interface (`beforeAction`, `afterAction` signatures) |

---

## Shared Constants

Contract addresses, chain IDs, event topic hashes, label keys, and secret paths are defined once per language. No hardcoding across components.

| Constant Category | Solidity Location | Go Location |
|-------------------|-------------------|-------------|
| Contract addresses | `TideConstants.sol` | `pkg/constants/addresses.go` |
| Event topic hashes | Derived from ABI | `pkg/constants/events.go` |
| Chain configuration | Constructor args | `pkg/constants/chain.go` |
| K8s labels/annotations | N/A | `pkg/constants/labels.go` |
| Secret paths | N/A | `pkg/constants/secrets.go` |

---

## Configuration Rules

All configuration flows through one of:
1. **Solidity constructor/initializer args** — immutable on-chain config
2. **K8s ConfigMap** — orchestrator/controller config, reconciled by Flux
3. **K8s SecretProviderClass** — secrets, mounted via AWS CSI driver
4. **Environment variables** — agent runtime config, set by the controller when creating Jobs

No config files baked into container images. No environment-specific logic in application code.

---

## Diagram Conventions

All visual representations in design documents must use **Mermaid** syntax. No ASCII art.

| Diagram Type | Mermaid Syntax | Use For |
|---|---|---|
| State machines | `stateDiagram-v2` | Lifecycle phases, upgrade flows, CRD status transitions |
| Sequences | `sequenceDiagram` | Cross-component interactions, request/response flows |
| Flowcharts | `graph TD` or `graph LR` | Decision trees, architecture overviews |
| Entity relationships | `erDiagram` | CRD field relationships, data models |

Mermaid renders natively in GitHub, GitLab, and most documentation tooling. Every diagram must be in a fenced code block with the `mermaid` language tag.

---

## Naming Conventions

| Domain | Convention | Example |
|--------|-----------|---------|
| Solidity contracts | PascalCase | `TideCouncil` |
| Solidity functions | camelCase | `submitReview` |
| Solidity constants | UPPER_SNAKE | `FUND_SELECTOR` |
| Solidity custom errors | PascalCase | `AgentNotRegistered` |
| Go packages | lowercase, match directory | `pkg/controller/tidejob` |
| Go types | PascalCase | `TideJobSpec` |
| K8s labels | `tide.sei.io/{key}` | `tide.sei.io/agent-id` |
| K8s annotations | `tide.sei.io/{key}` | `tide.sei.io/proposal-hash` |
| Secrets Manager paths | `tide/{category}/{name}` | `tide/agents/alpha/github-app-key` |
| GitHub repos | kebab-case under `sei-tide/` | `sei-tide/agent-alpha` |

---

## Decision Log

One-way door decisions made during design. Each must have explicit rationale.

| # | Decision | Rationale | Reversibility |
|---|----------|-----------|---------------|
| | *(filled during design process)* | | |

---

## Collaboration Protocol

### Round 1: Independent Drafting
Each owner drafts their component LLDs following this template. Focus on interface specification.

### Round 2: Cross-Review
Each owner reviews the other owners' interface definitions that touch their component. Flag any incompatibility, ambiguity, or unstated assumption.

### Round 3: Resolution
Resolve all cross-review findings. Update LLDs. Interface owners (providers) have final say on their interfaces; consumers adapt.

### Done Criteria
A component LLD is "done" when:
1. Every interface is specified with exact types (no TBD)
2. Every error case is documented
3. Every test case has setup + action + expected result
4. Every feature traces to a Phase 0–2 business need
5. The owner is confident a team of mid-level engineers could build it without asking clarifying questions
