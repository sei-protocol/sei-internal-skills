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

## Using the Council and Coral Skills

This repo's specialist roster lives in `.claude/agents/`. For design, review, or implementation work, invoke one of:

- **`/council`** — full-ceremony workflow for multi-component design, cross-review, or new-subsystem work. Runs scope-tier selection (Product / System / Component / Feature), dispatches specialists, gates one-way doors, manages `.council/workstream.yaml` checkpoints across sessions. Use when multiple components or interfaces are in scope.
- **`/coral`** — lightweight expert iteration for a defined slice of work. Picks the right specialist(s) and iterates without scope-tier ceremony. Hands off to `/council` when the work outgrows it (≥3 components, interface changes, one-way doors, multi-session).
- **`/bugbash`** — long-running, read-only adversarial review of an existing component before launch. Loops discovery + challenger passes by the expert slate against a named target (`/bugbash SeiNode controller`) until experts converge on a launch verdict; appends findings to `docs/bugbash/<target>.md`. Inspired by the RALPHY loop, reframed for hardening. Use when a system works on the happy path and needs to be pressure-tested for logical errors, validation gaps, race conditions, and operational risk before shipping.

Both skills will read `tide/interface-registry.yaml` as the authoritative interface source when working in this repo, and resolve any `.council/workstream.yaml` and `.council/escalations/` state before acting.

Tide is the canonical home for these workflow skills (and `/issue`); they sync out to user-scope and other repos via `scripts/sync-skills.sh`. See [`.claude/skills/README.md`](./.claude/skills/README.md) for the daily-flow command and full catalog.

### Handoff: Bootstrapping the Next Workstream as an Issue

When a coral or council session produces a deferred slice, a scope cut, or an obvious "phase 2," the orchestrator should offer **`/issue`** (`.claude/skills/issue/`) to file a standard-format GitHub issue capturing the synthesized context. The skill pre-fills Problem / Impact / Relevant experts / Proposed approach / Out of scope from the session, so the next pickup walks into the same picture instead of re-deriving it. See `.claude/skills/issue/references/coral-integration.md` for the handoff contract — when to offer, what to pass, what not to fabricate.

### Specialists (at `.claude/agents/`)

**Tide-specific** (not portable, stay in this repo):
- `blockchain-developer` — TideCouncil + TideJobHook (Solidity, ERC-8001/8183/8004, Sei EVM)
- `reviewer` — Tide interface cross-review

**Portable** (general — synced to other repos and user-level via `scripts/sync-agents.sh`):
- `kubernetes-specialist` — Go, controller-runtime, CRDs, event indexing, Job lifecycle
- `platform-engineer` — K8s manifests, Python container runtimes, cloud auth
- `solidity-developer` — general Solidity + Foundry + ERC standards
- `network-specialist` — K8s + cloud networking, service mesh
- `security-specialist`, `tee-specialist`, `product-engineer`, `product-manager`, `opentelemetry-expert`, `observability-platform-engineer`, `k8s-capacity-management`, `sre-engineer`

**Sei-ecosystem**:
- `sei-network-specialist` — Sei node networking (seid ports, CometBFT P2P, Waterway, Istio quirks)

See `AGENTS.md` for the full roster table and the Tide-specific agent context that augments portable agents when dispatched in this repo.

### Key Rules (enforced by both skills)
- **Never skip the interface registry.** It is the single source of truth.
- **Provider owns the interface.** Consumers adapt.
- **YAGNI.** Only features tracing to Phase 0-2 business needs.
- **Errors are interface.** Every error is part of the public contract.
- **One-way door gate.** Event signatures, storage layout, CRD spec field names, EIP-712 type hashes require explicit user approval before finalizing.
- **Conventional commits.** `feat:`, `fix:`, `docs:`, `refactor:` — reference the component in scope.
