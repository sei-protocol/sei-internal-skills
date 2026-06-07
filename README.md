<p align="center">
  <img src="assets/banner.png" alt="Tide" width="100%">
</p>

# Tide

Tide is Sei's library of **portable Claude Code skills and specialist agents** for engineering work. It's the centralized, version-controlled home for the workflows and personas that help us research problems, groom work, document progress in git and tickets, author and iterate on designs and 1-pagers, automate operational processes like releases and root-cause analysis, and collaborate with specialist agents.

Skills and agents are authored once here and synced out to your user-scope (`~/.claude/`) and sibling repos, so the same `/coral`, `/cross-review`, `/root-cause`, or `kubernetes-specialist` works the same way everywhere.

## Setup

One-line install: clone, then

```sh
make bootstrap
```

This runs:
- `make sync-agents` — installs Tide's portable agents into `~/.claude/agents/` so they're reachable from any cwd
- `make sync-skills` — installs Tide's portable skills into `~/.claude/skills/`
- `make update-agent-permissions` — installs the canonical read-only allow-list (`gh` reads, GitHub WebFetch) into `./.claude/settings.json`

The canonical permission set is **strictly read-only by design**. Mutating patterns (`gh issue create`, `gh pr merge`, `aws delete-*`, `kubectl apply`, etc.) are rejected by `make verify-agent-permissions`, which CI runs on every PR that touches the permission files. Local additions for your own workflow go in `.claude/settings.local.json` (gitignored).

Run `make` with no args to list all targets.

## Daily use

Most work starts with one of the collaboration skills:

- **`/coral`** — lightweight expert iteration on a defined slice. Picks the right specialist(s) and iterates. The fast path; reach for it first.
- **`/cross-review`** — have the relevant specialists independently review a design, plan, diff, or set of expert outputs, then synthesize a findings table. The review counterpart to coral's "produce."
- **`/council`** — full-ceremony engineering for multi-component design, scope-tier selection, and multi-session workstreams. The heavier sibling of coral.
- **`/bugbash`** — long-running adversarial review of an existing system before launch.
- **`/root-cause`** — disciplined, multi-expert investigation of a complex problem.

Coral and council offer **`/design`** (capture this work as a durable doc) and **`/issue`** (file the next workstream) at natural handoff moments.

## What's in here

- **Skills** (`.claude/skills/`) — self-contained Claude Code skills, grouped by domain:
  - **Workflow** — `/coral`, `/council`, `/cross-review`
  - **Workstream bootstrap** — `/design`, `/issue`
  - **Hardening & investigation** — `/bugbash`, `/root-cause`
  - **Skill authoring** — `/author-skill`, `/audit-skill`
  - **Output quality** (Tide-local) — `/brevity`, `/pr-quality`
  - **Product management** — `/prfaq`
  - **Project management** — `/impact-weekly`, `/impact-portfolio`
  - **Release operations** — `/chaos-suite`, `/validate-release`
  - **Engineer self-service** — `/harbor-dev`
- **Agents** (`.claude/agents/`) — specialist personas dispatched by the skills (or directly via the Agent tool), grouped by domain:
  - **Platform infra** — `kubernetes-specialist`, `platform-engineer`, `network-specialist`, `k8s-capacity-management`, `sei-network-specialist`
  - **Observability** — `opentelemetry-expert`, `observability-platform-engineer`, `sre-engineer`
  - **Security** — `security-specialist`, `tee-specialist`
  - **Blockchain** — `solidity-developer`
  - **Product management** — `product-engineer`, `product-manager`, `go-to-market-specialist`
  - **Release operations** — `platform-release-manager`
- **Sync machinery** (`scripts/`, `Makefile`):
  - `sync-skills.sh` / `sync-agents.sh` — copy skills/agents into user-scope (`~/.claude/`) or sibling repos, by domain or alias
  - `Makefile` — `make bootstrap` (one-shot install), plus `make sync-skills` / `make sync-agents`
  - `update-agent-permissions.sh` — installs the canonical read-only permission set

## Organization & selective sync

Skills and agents are grouped into **domains** for navigation and selective install — e.g. `project-management` (`/impact-weekly`, `/impact-portfolio`), `product-management` (`/prfaq`, `go-to-market-specialist`), `workflow`, `platform-infra`, and so on. The domain is **metadata, not directory structure**: each skill/agent carries a `category:` in its frontmatter, the catalogs ([`.claude/skills/README.md`](.claude/skills/README.md), [`AGENTS.md`](AGENTS.md)) group by it, and the sync scripts let you install one domain at a time:

```sh
make sync-skills                                            # the `portable` set (default)
./scripts/sync-skills.sh --categories project-management    # just one domain
./scripts/sync-skills.sh --categories all                   # everything syncable
```

Claude Code discovers skills/agents **flat** (`~/.claude/skills/<name>/`, `~/.claude/agents/<name>.md`) in both user and project scope — nested folders and custom roots like `~/.claude/tide/` are **not** discovered. So the install is always flat; domains never become on-disk folders. The aliases `portable`, `sei`, and `all` cross-cut the domains. (`output-quality` — `/brevity`, `/pr-quality` — is Tide-local and intentionally not synced.)

## Repository structure

```
.claude/agents/             # Specialist personas dispatched by the skills
.claude/skills/             # Skill definitions (SKILL.md + references + evals)
.claude/skills/README.md    # Skill catalog — start here
.claude/skills/SKILL-TEMPLATE.md  # Authoring standard for new skills
.github/workflows/          # CI (read-only permission enforcement)
AGENTS.md                   # Agent roster + how the skills dispatch them
CLAUDE.md                   # Project context auto-loaded into every session
assets/                     # Banner image
design/                     # Durable design reference (TEE research, coral-exercise outputs)
scripts/                    # sync-agents.sh, sync-skills.sh, permission tooling
```

## Where to start

| If you're... | Start here |
|---|---|
| **Using the skills day to day** | `.claude/skills/README.md` (the catalog) |
| **Authoring a new skill** | `.claude/skills/SKILL-TEMPLATE.md`, then `/author-skill` |
| **Auditing an existing skill** | `/audit-skill <name>` → report under `docs/skill-audits/` |
| **Adding or editing an agent persona** | `.claude/agents/` + update the roster in `AGENTS.md` |
| **Wiring a sibling repo to use these** | `scripts/sync-agents.sh --target <path>` and `scripts/sync-skills.sh --target <path>` |

## Contributing & conventions

- **Conventional commits.** `feat:`, `fix:`, `docs:`, `refactor:` — reference the skill or component in scope (e.g. `feat(cross-review): ...`, `docs(readme): ...`).
- **Brevity discipline.** Apply `/brevity` before writing PR bodies or WHY-style in-code comments.
- **PR-quality discipline.** Before `gh pr create`, apply `/pr-quality` to the staged diff + planned body.
- **Edit skills here, not in `~/.claude/`.** User-scope copies are overwritten on the next sync. Change a skill in Tide and PR it.

## Documentation map

| Doc | What it covers |
|-----|----------------|
| `README.md` (this file) | Orientation, install, daily use, structure |
| `CLAUDE.md` | Project context auto-loaded into every Claude Code session |
| `AGENTS.md` | Agent roster + how the skills dispatch them |
| `.claude/skills/README.md` | Skill catalog and cross-repo sync guidance |
| `.claude/skills/SKILL-TEMPLATE.md` | Authoring standard for new skills |
| `scripts/README.md` | What each script does and when CI vs. humans run them |
| `design/README.md` | Durable design reference kept in the repo |
