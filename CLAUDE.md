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

### Quick design review
```
/review <path-to-design-doc>
```

### Full design cycle (draft → cross-review → resolve)
```
/design <component-name>
```

### Generate implementation from spec
```
/implement <path-to-lld>
```

### Verify interface consistency across all specs
```
/verify
```

### Spawn an independent workstream
Use the coordinator agent directly and describe the effort. It will assemble the right team and manage the workflow.
