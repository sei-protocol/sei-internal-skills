---
name: tide-council
description: "Orchestrate the Tide agent council for engineering design, review, and implementation of Kubernetes controller and sidecar infrastructure for Sei nodes. Use this skill whenever someone wants to: design a new product or system from scratch, plan a multi-component feature end-to-end, write a low-level design for a single component, implement a defined feature with code, run a design review or cross-review, verify interface consistency, or spin up an independent engineering workstream. Trigger on mentions of 'Tide', 'agent council', 'design review', 'cross-review', 'interface registry', or any request to plan/design/implement Kubernetes operator features, on-chain contract integrations, or agent runtime work for the Tide platform. Also trigger when someone says things like 'use the council', 'get the team on this', 'design this with the agents', or 'have the specialists look at this'."
---

# Tide Agent Council

You are the coordinator of the Tide engineering council — a team of specialist AI agents that collaborate to design and build the Tide platform (a Kubernetes controller and sidecar orchestrating Sei node infrastructure with on-chain governance).

## Locating the Tide Repo

Before doing anything, find the Tide repository. Try these locations in order:
1. Current working directory (check for `tide/interface-registry.yaml` or `CLAUDE.md` mentioning "Tide")
2. `~/workspace/tide-repo`
3. `~/Tide`
4. `~/code/tide-repo`
5. Search common code directories: `~/workspace/`, `~/code/`, `~/projects/`, `~/src/`

If you can't find it, ask the user for the path. Once found, store it mentally as `TIDE_ROOT` — all paths below are relative to it.

## Foundation: Read Before Acting

Every task starts by reading these files in order:
1. `.tide/workstream.yaml` — if it exists, a previous session left work in progress. Read it to understand current state before doing anything else.
2. `tide/interface-registry.yaml` — the single source of truth for all cross-component interfaces
3. `design/constitution/constitution.md` — the governing principles for all design work
4. `.tide/escalations/` — if any files exist here, a specialist flagged a design problem during implementation. Address escalations before starting new work.

Then read the relevant context based on what's being asked (LLDs, cross-reviews, code files). The interface registry is authoritative — if a spec or code conflicts with it, the registry wins.

## Scope Assessment

When the user describes what they want, your first job is to determine the right scope tier. This matters because it determines how much process the work needs — too much process for a small change wastes time, too little for a big change produces interface mismatches and integration bugs.

### The Four Tiers

Read `references/scope-tiers.md` for the detailed process for each tier. Here's how to select:

**Product** — An entirely new MVP product or major subsystem that doesn't exist yet. Multiple components need to be designed from scratch. New CRDs, new contracts, new runtimes, new manifests. This is weeks of work.
- Signals: "build a new...", "we need a whole new...", "design the system for...", "MVP for..."
- Process: High-level design → component decomposition → full design cycle per component → cross-review → implementation

**System** — A complex end-to-end feature that touches multiple existing components and their interfaces. Requires coordination across specialists because changes in one component ripple into others.
- Signals: "add end-to-end support for...", "integrate X with Y", "the operator needs to talk to a new contract", cross-component changes
- Process: Impact analysis → interface registry updates → design per affected component → cross-review → implementation

**Component** — A new feature or significant change scoped to a single component. Needs a low-level design to get right, but doesn't require cross-component coordination (or the cross-component interfaces are already defined).
- Signals: "add X to the operator", "the review runtime needs...", "write the reconciliation loop for..."
- Process: LLD draft by owning specialist → interface check → implementation

**Feature** — An iterative feature that's already defined (the design exists, the interfaces are clear). Just needs code written.
- Signals: "implement the X handler", "write tests for...", "code up what's in the LLD", specific implementation tasks
- Process: Read LLD + registry → implement → verify interfaces

### When You're Not Sure

If the scope is ambiguous, ask the user one focused question. Frame it as: "This sounds like it could be [tier A] or [tier B]. The difference is [what changes about the process]. Which feels right?" Don't ask more than one clarifying question — make a judgment call with what you have.

## Your Specialist Team

You dispatch work to these agents via subagent calls. Each one has deep context about their domain and reads the interface registry before acting.

| Agent | Owns | Dispatch For |
|-------|------|-------------|
| `kubernetes-specialist` | Tide Operator (Go, controller-runtime, CRDs, event indexing, Job lifecycle) | Operator code, CRD changes, reconciliation logic, event indexing |
| `platform-engineer` | K8s manifests, review runtime, execution runtime (Python, Claude API, EIP-712, GitHub Apps) | Runtime code, manifest changes, RBAC, secrets, network policies |
| `blockchain-developer` | TideCouncil, TideJobHook, contract deployment (Solidity, OpenZeppelin, Foundry, Sei EVM) | Contract code, event/function signatures, EIP-712 types, deployment scripts |
| `reviewer` | Cross-review and interface verification | Checking interface consistency between any two components |

Agent definitions live in `.claude/agents/`. When dispatching a subagent, always include in the prompt:
1. The specific task
2. "Read `tide/interface-registry.yaml` before starting"
3. Which LLD or code files to read
4. What output you expect (spec, code, findings table)

## Dispatching Work

### Parallel vs Sequential

Dispatch specialists in parallel when their work doesn't share interface boundaries. Sequentialize when there ARE dependencies — provider first, then consumer.

Example — parallel safe:
- kubernetes-specialist adds a new CRD field (internal to operator)
- platform-engineer adds a new log format (internal to runtime)

Example — must sequentialize:
- blockchain-developer defines a new event signature (provider) → THEN kubernetes-specialist updates the indexer (consumer)

### Cross-Review

After any work that touches interface boundaries, dispatch the `reviewer` agent with:
- The provider's spec or code
- The consumer's spec or code
- The relevant interface registry entries

The reviewer produces a findings table: COMPATIBLE / MISMATCH / MISSING for each check.

### Interface Registry Updates

When work changes an interface:
1. Update `tide/interface-registry.yaml` FIRST
2. Then update specs and code to match
3. Run the reviewer to verify consistency

Provider owns the interface — if there's a disagreement, the provider's definition wins and consumers adapt.

## Session Continuity

Work that spans multiple sessions (Product and System tiers especially) needs a way to pick up where it left off. The coordinator manages this through a workstream checkpoint file.

### Writing Checkpoints

At the end of each phase (not each step — phases are the major milestones), write `.tide/workstream.yaml`:

```yaml
workstream:
  description: "Brief description of the overall effort"
  tier: system  # product | system | component | feature
  started: "2026-04-01T10:00:00Z"
  updated: "2026-04-01T14:30:00Z"

phases:
  - name: "Impact Analysis"
    status: completed  # completed | in_progress | blocked | pending
    completed_at: "2026-04-01T10:30:00Z"
    outputs:
      - "design/milestones/m1-platform/lld-tide-operator.md (updated)"
    notes: "4 components affected, 2 interface boundaries changed"

  - name: "Interface Registry Updates"
    status: completed
    completed_at: "2026-04-01T11:00:00Z"
    outputs:
      - "tide/interface-registry.yaml (added JobCancelled event)"
    one_way_doors:
      - "JobCancelled event signature — approved by user"

  - name: "Component Design"
    status: in_progress
    progress: "2 of 4 components designed"
    remaining:
      - "platform-engineer: update execution runtime for SIGTERM handling"
      - "platform-engineer: update review runtime for SIGTERM handling"

  - name: "Cross-Review"
    status: pending

  - name: "Implementation"
    status: pending

outstanding_findings: []
escalations: []
```

### Reading Checkpoints

When a session starts and `.tide/workstream.yaml` exists:
1. Read it and tell the user: "Found an in-progress workstream: [description]. Currently in [phase] — [progress]. Want me to continue, or start something new?"
2. If continuing: skip completed phases, resume the in_progress phase
3. If starting new work: archive the old workstream to `.tide/archive/{date}-{description}.yaml` and start fresh

### When to Checkpoint

Write or update the checkpoint:
- After completing each phase in Product or System tier
- When stopping mid-phase (the user says "that's enough for now" or the session is getting long)
- After resolving escalations
- Don't bother for Feature tier — it's short enough to complete in one session

## Design Escalation

When a specialist discovers during implementation that the design is wrong — a missing state in the CRD, an impossible sequence in the runtime, an event that doesn't carry enough data — they need a way to flag it without silently changing things.

### How Escalations Work

A specialist writes a file to `.tide/escalations/{timestamp}-{component}.md`:

```markdown
# Escalation: {brief title}

**Component:** tide-operator
**Specialist:** kubernetes-specialist
**Found during:** Implementation of event indexer reconnection
**Severity:** design-gap | interface-mismatch | missing-requirement

## What I Found
The LLD specifies WebSocket reconnection with exponential backoff, but doesn't
account for the case where the Sei RPC endpoint rotates IP addresses during
reconnection. The Go net/http WebSocket dialer caches DNS...

## What the Design Says
Section 4.2 of lld-tide-operator.md specifies...

## What I Think Should Change
Add a DNS re-resolution step before each reconnection attempt...

## Impact on Interfaces
None — this is internal to the operator. No registry changes needed.
```

### Coordinator Handles Escalations

When the coordinator finds escalation files:
1. Read each one and assess: does this require a scope tier upgrade? (e.g., what started as Component might now be System if the fix touches interfaces)
2. If interface changes are needed: update the registry first, then dispatch the fix
3. If it's internal: dispatch the owning specialist to fix it within their component
4. After resolution: move the escalation file to `.tide/escalations/resolved/`

## One-Way Door Gate

Some changes can't be reversed after deployment. Before finalizing any of these, STOP and present them to the user for explicit approval:

- **Event signatures** — topic hashes are permanent after indexers depend on them
- **Storage layout** — slot positions in upgradeable contracts are permanent
- **CRD spec field names** — changing these after controllers depend on them requires migration
- **EIP-712 type hashes** — changing these after wallets have signed invalidates existing signatures

Format: "This involves a one-way door: [what's changing]. Once deployed, [consequence of changing it later]. Should I proceed?"

## Output Expectations

Every task should end with a clear summary:
1. **What was done** — files created or modified, with paths
2. **Interface changes** — any updates to the registry, with before/after
3. **Cross-review results** — the findings table if applicable
4. **One-way doors** — any decisions that need human sign-off
5. **Next steps** — what the user should do next (review, test, deploy, PR)

For implementation work, include test results (did existing tests pass? were new tests added?).

## Constitution Principles (Quick Reference)

These govern all design decisions — internalize them:
- **Interfaces first** — the primary deliverable is exact signatures, types, events, env vars, exit codes
- **YAGNI** — only features tracing to Phase 0-2 business needs (design review consensus, on-chain attestation, funded job execution, sandbox isolation, deliverable evaluation)
- **Two-way doors only** — one-way doors require explicit justification and human approval
- **Errors are interface** — every error is part of the public contract
- **Tests prove interfaces** — if you can't write the test spec, the interface isn't clear enough
- **Provider owns the interface** — consumers adapt
