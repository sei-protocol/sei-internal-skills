# Tide Engineering Workspace

## Project Context
This repository contains the Tide platform — a Kubernetes controller and sidecar that orchestrates Sei node infrastructure, governed by on-chain coordination contracts (ERC-8001, ERC-8183, ERC-8004) deployed on Sei EVM.

## Architecture Overview
Tide has three layers:
1. **On-chain contracts** — TideCouncil (design governance), TideJobHook (job escrow hooks), deployed on Sei EVM (chain ID 1329)
2. **Tide Operator** — Go binary (controller-runtime) that bridges on-chain events to Kubernetes workloads
3. **Agent Runtimes** — Python containers (review + execution) that run as K8s Jobs in the `tide-agents` namespace

## Agent Council
This workspace uses a council of specialist agents for design and implementation. The council follows a constitution (see `design/constitution/constitution.md`) with these core principles:
- **Interfaces first** — the primary deliverable is exact signatures, types, events, env vars, exit codes
- **YAGNI** — only features tracing to Phase 0-2 business needs
- **Two-way doors only** — one-way doors (storage layout, event sigs, CRD field names, EIP-712 types) require explicit justification
- **Errors are interface** — every error is part of the public contract

## Interface Registry
The single source of truth for all cross-component interfaces is `tide/interface-registry.yaml`. All agents read from it. All specs must be consistent with it. When an interface changes, the registry is updated first, then specs are updated to match.

## Working Agreements
- **Provider owns the interface.** Consumers adapt. See the ownership table in the interface registry.
- **Env var naming:** Runtimes are consumers, so runtime convention wins. The Operator adapts.
- **Completion signaling:** `/dev/termination-log` is primary (survives pod termination). `status.json` is advisory only.
- **Exit codes:** Granular codes (10-52) from runtimes. Operator groups them for retry/fail decisions.
- **Commit style:** Conventional commits (`feat:`, `fix:`, `docs:`, `refactor:`). Reference the component in scope.

## Key File Locations
- `design/constitution/constitution.md` — governing document for all design work
- `design/milestones/` — low-level design documents per component
- `tide/interface-registry.yaml` — machine-readable interface contracts
- `pkg/` — Go source for the Tide Operator
- `runtimes/review/` — Python review runtime
- `runtimes/execution/` — Python execution runtime
- `manifests/` — Kustomize base/overlays for K8s platform

## Code Conventions
- **Go:** controller-runtime patterns (kubebuilder). Packages match directories. Types are PascalCase.
- **Python:** Runtimes use structured logging (JSON to stdout). Exit codes are granular (see registry).
- **Solidity:** OpenZeppelin base contracts. Custom errors over require strings. Events for all state transitions.
- **K8s labels:** `tide.sei.io/{key}` prefix. Standard `app.kubernetes.io/component` for pod selection.
- **Secrets:** AWS Secrets Store CSI Driver. Never baked into images. Never in etcd as K8s Secrets.

## How to Use the Agent Council

**IMPORTANT: This project has a specialist agent council. When asked to design, implement, review, or verify any Tide component, you MUST use the agent workflow described below. Do not attempt to do design or implementation work directly — dispatch it to the appropriate specialist agents.**

### Workflow: Design, Review, Implement, Verify

When the user asks you to work on any Tide component, follow this process:

1. **Read foundation files first:**
   - `tide/interface-registry.yaml` — the single source of truth for all cross-component interfaces
   - `design/constitution/constitution.md` — the governing principles
   - `.tide/workstream.yaml` — if it exists, there's work in progress from a previous session
   - `.tide/escalations/` — if files exist here, a specialist flagged a design problem

2. **Determine scope tier** (read `.claude/skills/tide-council/references/scope-tiers.md` for the full process per tier):
   - **Product** — entirely new MVP/subsystem, multiple new components (days of work)
   - **System** — feature touching multiple existing components and their interfaces (hours)
   - **Component** — feature scoped to a single component (30min–few hours)
   - **Feature** — design exists, interfaces defined, just write code (minutes–hour)
   If the scope is ambiguous, ask the user one focused question to clarify.

3. **Dispatch specialist agents** using the Agent tool. Agent definitions are in `.claude/agents/`:
   - `kubernetes-specialist` — Tide Operator (Go, controller-runtime, CRDs, event indexing, Job lifecycle)
   - `platform-engineer` — K8s manifests, agent runtimes (Python, Claude API, EIP-712, GitHub Apps)
   - `blockchain-developer` — TideCouncil, TideJobHook (Solidity, OpenZeppelin, Foundry, Sei EVM)
   - `reviewer` — cross-review and interface verification between any two components

   Always include in every specialist dispatch: "Read `tide/interface-registry.yaml` before starting."

   Dispatch in parallel when work doesn't share interface boundaries. Sequentialize when there ARE dependencies — provider first, then consumer.

4. **Cross-review** after any work touching interface boundaries: dispatch the `reviewer` agent with provider spec, consumer spec, and registry entries.

5. **One-way door gate** — STOP and present to user for approval before finalizing: event signatures, storage layout, CRD spec field names, EIP-712 type hashes.

6. **Session continuity** — for Product/System tier work, write `.tide/workstream.yaml` checkpoints after each phase (see `.claude/skills/tide-council/SKILL.md` for the YAML format).

### Key Rules
- **Never skip the interface registry.** It is the single source of truth.
- **Provider owns the interface.** Consumers adapt.
- **YAGNI.** Only features tracing to Phase 0-2 business needs.
- **Errors are interface.** Every error is part of the public contract.
- **Conventional commits.** `feat:`, `fix:`, `docs:`, `refactor:` — reference the component in scope.
