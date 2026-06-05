# Tide Engineering Workspace

## Project Context

Tide is Sei's centralized library of **portable Claude Code skills and specialist agents** for engineering work. Skills and agents are authored and version-controlled here, then synced out to user-scope (`~/.claude/`) and sibling repos via `scripts/sync-skills.sh` and `scripts/sync-agents.sh`. This repo is the canonical home — edit here and PR; never edit the synced copies.

The skills help engineers research, groom work, document progress in git and tickets, author and iterate on designs and 1-pagers, automate operational processes (releases, root-cause analysis), and collaborate with specialist agents.

## Engineering Principles

These principles govern design and review work across the skills (notably `/council`, `/coral`, `/cross-review`):

- **Interfaces first** — the primary deliverable of a design is exact signatures, types, errors, and contracts. Implementation guidance is secondary.
- **YAGNI** — only build what traces to a current-phase need. Everything else is explicitly deferred, not silently omitted.
- **Two-way doors only** — prefer reversible decisions. One-way doors (irreversible choices: persisted schema/field names, public API contracts, on-disk or wire data formats, anything other systems come to depend on) require explicit human approval before finalizing.
- **Errors are interface** — every error condition is part of the public contract.
- **Provider owns the interface** — when a provider and consumer disagree, the provider's definition is canonical and consumers adapt. This breaks circular dependencies during interface resolution.

## Output Discipline

- **Brevity.** Apply `/brevity` (`.claude/skills/brevity/`) before writing PR bodies or in-code comments. The skill self-determines floor; agents do not pre-skip.
- **PR-quality.** Before invoking `gh pr create`, apply `/pr-quality` (`.claude/skills/pr-quality/`) to the staged diff + planned body. Findings surface inline for revision. Post-PR: invoke `/pr-quality <PR>` to post a comment with findings. (Brevity runs during authoring; pr-quality runs on the final diff — they don't chain.)
- **Conventional commits.** `feat:`, `fix:`, `docs:`, `refactor:` — reference the skill or component in scope.

## Using the Skills

This repo's specialist roster lives in `.claude/agents/`; the skill catalog lives in `.claude/skills/README.md`. For design, review, or implementation work, reach for:

- **`/coral`** — lightweight expert iteration on a defined slice. Picks the right specialist(s) and iterates without ceremony. The fast path. Hands off to `/council` when work outgrows it (≥3 components, interface changes, one-way doors, multi-session).
- **`/cross-review`** — have the relevant specialists independently review a design, plan, diff, or set of expert outputs, then synthesize a findings table (COMPATIBLE / MISMATCH / MISSING). The review counterpart to coral's "produce"; coral and council both call into it.
- **`/council`** — full-ceremony workflow for multi-component design and multi-session work. Runs scope-tier selection (Product / System / Component / Feature), dispatches specialists, gates one-way doors, manages `.council/workstream.yaml` checkpoints across sessions.
- **`/bugbash`** — long-running, read-only adversarial review of an existing component before launch. Loops discovery + challenger passes until experts converge on a launch verdict; appends findings to `docs/bugbash/<target>.md`.
- **`/root-cause`** — disciplined, data-driven, multi-expert investigation of complex problems.

### Handoff: Bootstrapping the Next Workstream

When a coral or council session produces a deferred slice, a scope cut, or an obvious "phase 2," the orchestrator offers:
- **`/design`** (`.claude/skills/design/`) — capture *this* work as a durable design doc.
- **`/issue`** (`.claude/skills/issue/`) — file *next* work as a standard-format GitHub issue that bootstraps the next pickup.

See each skill's `references/coral-integration.md` for the handoff contract — when to offer, what to pass, what not to fabricate.

### Specialists (at `.claude/agents/`)

**Portable** (synced to other repos and user-level via `scripts/sync-agents.sh`):
- `kubernetes-specialist` — Go, controller-runtime, CRDs, event indexing, Job lifecycle
- `platform-engineer` — K8s manifests, Python container runtimes, cloud auth
- `solidity-developer` — Solidity + Foundry + ERC standards
- `network-specialist` — K8s + cloud networking, service mesh
- `security-specialist`, `tee-specialist`, `product-engineer`, `product-manager`, `opentelemetry-expert`, `observability-platform-engineer`, `k8s-capacity-management`, `sre-engineer`, `go-to-market-specialist`, `platform-release-manager`

**Sei-ecosystem**:
- `sei-network-specialist` — Sei node networking (seid ports, CometBFT P2P, Waterway, Istio quirks)

See `AGENTS.md` for the full roster table.

### Key Rules (enforced by the design/review skills)
- **Provider owns the interface.** Consumers adapt.
- **YAGNI.** Only features tracing to current-phase needs.
- **Errors are interface.** Every error is part of the public contract.
- **One-way door gate.** Irreversible decisions require explicit user approval before finalizing.
- **Conventional commits.** Reference the skill or component in scope.

## Authoring & Maintaining Skills

- **New skill:** read `.claude/skills/SKILL-TEMPLATE.md`, then use `/author-skill`. Draft the guardrails stanza first — if you can't articulate what the skill refuses to do, it isn't ready.
- **Audit a skill:** use `/audit-skill <name>` (audit-only by default; `--apply` to refactor). Reports land under `docs/skill-audits/`.
- **Sync out:** `make sync-skills` / `make sync-agents` push portable updates to user-scope; pass `--target <repo>` for sibling repos.
