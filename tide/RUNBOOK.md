# Tide Agent Council — Team Runbook

## Setup (one-time, ~5 minutes)

### 1. Install Claude Code

```bash
npm install -g @anthropic-ai/claude-code
```

### 2. Clone this repo

```bash
git clone https://github.com/sei-protocol/tide.git
cd tide
```

The repo ships with:
- `.claude/agents/` — the specialist roster (12 agents — see `AGENTS.md` for the full list and per-agent Tide context)
- `.claude/skills/` — Tide-specific procedural skills (`chaos-suite` scaffold today) + `SKILL-TEMPLATE.md` for authoring more
- `tide/interface-registry.yaml` — single source of truth for cross-component interfaces
- `CLAUDE.md` and `AGENTS.md` — governing docs read by agents and skills on every session
- `design/` — constitution + high-level design + per-component LLDs + cross-reviews
- `scripts/verify_registry.py` — registry-code consistency checker (invoked by `/verify` and by CI)

### 3. (Optional) Make the portable agents available from any directory

Nine of the twelve agents are portable (general-purpose) and useful outside Tide. Sync them to user-level so the `/council` and `/coral` skills can find them from any repo:

```bash
./scripts/sync-agents.sh --target ~/
```

To push updates later, re-run with `--force`.

### 4. (Optional) Install the interface-registry pre-commit hook

```bash
cp scripts/pre-commit-hook.sh .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

This runs `verify_registry.py` automatically before commits that touch interface-relevant paths.

### 5. Verify it works

```bash
claude
```

Then type:
```
/verify
```

You should see the verify command run `scripts/verify_registry.py`, augment it with the manual checks, and report on interface consistency.

---

## Daily Workflows

Two skills cover most of what you'll do:

- **`/council`** — full-ceremony workflow for multi-component design, cross-review, and new-subsystem work. Runs scope-tier selection (Product / System / Component / Feature), dispatches specialists in provider-first order, gates one-way doors, manages `.council/workstream.yaml` checkpoints across sessions.
- **`/coral`** — lightweight expert iteration when you have a defined slice of work. No tier selection, no mandatory cross-review, no workstream file. Picks the right specialist(s), iterates, and flags to hand off to `/council` when the work outgrows it.

Reach for `/coral` first for most feature- and component-level work; use `/council` when scope is broader or one-way doors are in play.

### "I need to design a new component"

```bash
> /council
> I want to design a new chaos-engineering subsystem that plugs into the Operator.
```

Council picks the tier (likely Product or System), drafts the high-level design, decomposes into components, dispatches specialists to draft each LLD, runs cross-review, resolves mismatches, and updates the interface registry. You'll be prompted to approve any one-way door decisions before finalizing.

### "I need to review an existing design"

```bash
> /council
> Cross-review design/milestones/m1-platform/lld-tide-operator.md against the runtime LLDs.
```

Or for a narrower review:

```bash
> /coral
> Have the reviewer check the event indexer spec against the TideCouncil event signatures.
```

### "I need to implement something from a spec"

When the LLD exists and the scope is clear:

```bash
> /coral
> Implement the Operator's ProposalRejected event handler per lld-tide-operator.md.
```

When the scope is broader or spans components:

```bash
> /council
> Implement the full end-to-end ProposalRejected path, across Operator + review runtime.
```

### "I need a small team to handle a focused effort"

```bash
> /coral
> Add ProposalRejected event indexing to the Operator.
```

Coral picks the right specialist(s) for you. If the work grows (≥3 components, an interface change, a one-way door, or a cross-session commitment), coral flags and offers to hand off to `/council`.

### "I just want to talk to one specialist"

Name the agent directly:

```bash
> As the kubernetes-specialist, how should the reconciliation loop handle a missed ProposalApproved event?
```

Or let coral route for you:

```bash
> /coral
> How should the reconciliation loop handle a missed ProposalApproved event?
```

### "I want to check everything is consistent"

```bash
> /verify
```

Run after any design or implementation session. Wraps `scripts/verify_registry.py` (mechanical checks: env vars, function names, ServiceAccount patterns, file paths) with Claude-augmented manual checks (event signature hashes, exit-code handling, K8s resource naming). The same underlying script runs in CI on PRs that touch interface-relevant paths.

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

Both `/council` and `/coral` flag these and require explicit approval. Don't skip this.

### Provider Owns the Interface

When two components disagree about an interface, the provider's definition wins:
- Solidity contracts provide event signatures and function signatures
- The Operator provides env vars and volume mounts to runtimes
- Runtimes provide exit codes and completion signaling to the Operator

The consumer adapts. This avoids circular dependencies during resolution.

### Council vs Coral — when to use which

- **Coral first** for most feature- and component-level work. It's the lowest-friction entry point.
- **Council** when the scope is multi-component, introduces new interfaces, involves a one-way door, or will span multiple sessions.
- If you start in coral and the work grows, coral flags and offers a clean handoff.

---

## Tips

**Start with `/coral`.** Even for ambiguous asks, coral can pick the right specialist and either iterate or tell you this needs `/council`. The judgment is built into the skill, so you don't have to pick upfront.

**Read the cross-review output.** When `/council` runs cross-review, the mismatch table is the most valuable artifact — those are bugs that would otherwise surface in integration testing (or production).

**Keep the registry current.** If you make a change outside the skills (manual code edit, quick fix), run `/verify` afterward to catch drift. CI also runs `scripts/verify_registry.py` on PRs that touch interface-relevant paths.

**Parallel specialists.** Both `/council` and `/coral` dispatch specialists in parallel when work doesn't share interface boundaries. Provider-first ordering only applies when there's a real dependency.

**Pulling portable agents into another repo.** Use `scripts/sync-agents.sh --target <path> --categories portable,sei` to copy the general agents into a sibling repo's `.claude/agents/`. Re-run with `--force` to push updates after editing an agent in Tide.

---

## Troubleshooting

**Agent gives inconsistent output:** Make sure `tide/interface-registry.yaml` is up to date. Agents read it at the start of every task. Stale registry = stale outputs.

**Cross-review finds too many issues:** The registry has drifted from the specs. Run `/verify`, fix the registry, then re-run the cross-review.

**Agent doesn't know about a recent change:** Claude Code agents don't have memory between sessions — they read project files fresh each time. Make sure your changes are saved to disk before invoking an agent.

**One-way door approval loop:** If the workflow keeps asking you to approve the same one-way door, the registry entry is missing a `topic0` hash. Compute it (keccak256 of the canonical signature) and fill it in.

**`/council` can't find the agent roster:** You're probably in a non-Tide repo with no `.claude/agents/`. Either `cd` into the Tide repo, or sync portable agents to user-level via `./scripts/sync-agents.sh --target ~/` (run once from Tide).
