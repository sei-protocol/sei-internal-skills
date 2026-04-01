# Tide Agent Council — Team Runbook

## Setup (one-time, ~5 minutes)

### 1. Install Claude Code
```bash
npm install -g @anthropic-ai/claude-code
```

### 2. Copy the workflow kit into your Tide repo
```bash
# From your Tide repo root:
cp -r <path-to-kit>/.claude .claude
cp -r <path-to-kit>/tide/interface-registry.yaml tide/interface-registry.yaml
cp <path-to-kit>/CLAUDE.md CLAUDE.md
```

You should now have:
```
Tide/
  .claude/
    agents/
      coordinator.md
      kubernetes-specialist.md
      platform-engineer.md
      blockchain-developer.md
      reviewer.md
    commands/
      design.md
      review.md
      implement.md
      verify.md
      workstream.md
  tide/
    interface-registry.yaml
  CLAUDE.md
  design/
    constitution/
    milestones/
  pkg/
  runtimes/
  manifests/
  contracts/
```

### 3. Verify it works
```bash
cd Tide/
claude
# Then type: /verify
```

You should see the verify command run and report on interface consistency.

---

## Daily Workflows

### "I need to design a new component"
```bash
claude
> /design TideCouncil
```
This runs the full design cycle: the owning specialist drafts the LLD, other specialists cross-review the interfaces, mismatches are resolved, and the interface registry is updated. You'll be prompted to approve any one-way door decisions.

### "I need to review an existing design"
```bash
claude
> /review design/milestones/m1-platform/lld-tide-operator.md
```
This checks the spec against all interface boundaries it participates in and reports mismatches.

### "I need to implement something from a spec"
```bash
claude
> /implement design/milestones/m1-platform/lld-tide-operator.md
```
This dispatches the right specialist to write code matching the LLD, then verifies the implementation against the interface registry.

### "I need a small team to handle a focused effort"
```bash
claude
> /workstream "Add ProposalRejected event indexing to the Operator and update the CRD state machine"
```
This scopes the work, identifies which specialists and interfaces are involved, dispatches them (in parallel where safe), and cross-checks the result.

### "I just want to talk to one specialist"
Just describe what you need and reference the specialist's domain. Claude Code reads the agent definitions in `.claude/agents/` and will use the right specialist's context:
```bash
claude
> Acting as the kubernetes-specialist, how should the reconciliation loop handle a missed ProposalApproved event?
```
Or for the coordinator to manage a multi-agent effort:
```bash
claude
> Using the coordinator, plan out adding ProposalRejected event indexing to the Operator.
```
The slash commands (`/design`, `/workstream`, etc.) are the preferred way to invoke agents — they handle the orchestration automatically.

### "I want to check everything is consistent"
```bash
claude
> /verify
```
Run this after any design or implementation session. It checks all specs and code against the interface registry and reports any drift.

---

## Key Concepts

### The Interface Registry
`tide/interface-registry.yaml` is the single source of truth. When there's a conflict between a spec and the registry, the registry wins. When you need to change an interface:
1. Update the registry first
2. Then update specs/code to match
3. Run `/verify` to confirm

### One-Way Doors
Some interfaces can't be changed after deployment without significant pain:
- **Event signatures** — changing these after the Operator indexes them breaks all event matching
- **Storage layout** — changing slot positions in upgradeable contracts corrupts state
- **CRD spec field names** — changing these after controllers depend on them requires migration
- **EIP-712 type hashes** — changing these after wallets have signed invalidates existing signatures

The workflow will flag these and ask for your explicit approval. Don't skip this.

### Provider Owns the Interface
When two components disagree about an interface, the provider's definition wins:
- Solidity contracts provide event signatures and function signatures
- The Operator provides env vars and volume mounts to runtimes
- Runtimes provide exit codes and completion signaling to the Operator

The consumer adapts. This avoids circular dependencies in resolution.

---

## Tips

**Start small.** Use `/workstream` for a focused task before running the full `/design` cycle. Get a feel for how the agents interact.

**Read the cross-review output.** The most valuable part of the workflow is the mismatch table from cross-review. These are bugs that would otherwise surface in integration testing (or production).

**Keep the registry up to date.** If you make a change outside the workflow (manual code edit, quick fix), run `/verify` afterward to catch any drift.

**Use the coordinator for ambiguous work.** If you're not sure which specialist should handle something, just describe the effort and ask for coordination:
```bash
claude
> I need to add a new event to TideCouncil that the Operator indexes. Use the coordinator to walk me through this.
```

**Parallel specialists.** The `/workstream` command will run specialists in parallel when their work doesn't share interface boundaries. This is faster than sequential rounds for independent changes.

---

## Troubleshooting

**Agent gives inconsistent output:** Make sure `tide/interface-registry.yaml` is up to date. Agents read it at the start of every task. Stale registry = stale outputs.

**Cross-review finds too many issues:** This usually means the registry drifted from the specs. Run `/verify`, fix the registry, then re-run.

**Agent doesn't know about a recent change:** Claude Code agents don't have memory between sessions. They read project files fresh each time. Make sure your changes are saved to disk before invoking an agent.

**One-way door approval loop:** If the workflow keeps asking you to approve the same one-way door, it means the registry entry doesn't have a `topic0` hash computed yet. Compute it (keccak256 of the canonical signature) and fill it in.
