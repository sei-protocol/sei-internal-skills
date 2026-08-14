<p align="center">
  <img src="assets/sei-internal-skills-logo.png" alt="sei-internal-skills" width="100%">
</p>

# sei-internal-skills

sei-internal-skills is Sei's library of **portable Claude Code skills and specialist agents** for engineering work. It's the centralized, version-controlled home for the workflows and personas that help us review code, investigate failures, operate releases, run ephemeral chains, and collaborate with specialist agents.

Skills and agents are authored once here and synced out to your user-scope (`~/.claude/`) and sibling repos, so the same `/xreview`, `/root-cause`, or `kubernetes-specialist` works the same way everywhere.

## Setup

One-liner

```sh
gh api repos/sei-protocol/sei-internal-skills/contents/scripts/install.sh -H 'Accept: application/vnd.github.raw' | bash
```

Or clone, then

```sh
make bootstrap
```

This runs:
- `make sync-agents` — installs sei-internal-skills's portable agents into `~/.claude/agents/` so they're reachable from any cwd
- `make sync-skills` — installs sei-internal-skills's portable skills into `~/.claude/skills/`
- `make update-agent-permissions` — installs the canonical read-only allow-list (`gh` reads, GitHub WebFetch) into `./.claude/settings.json`

The canonical permission set is **strictly read-only by design**. Mutating patterns (`gh issue create`, `gh pr merge`, `aws delete-*`, `kubectl apply`, etc.) are rejected by `make verify-agent-permissions`, which CI runs on every PR that touches the permission files. Local additions for your own workflow go in `.claude/settings.local.json` (gitignored).

Run `make` with no args to list all targets.

## Just one piece

The setup above installs the whole core. Give the same command a target and it
installs one thing instead — no clone, no `make`, nothing else installed. If you
already have the checkout, it reads that rather than downloading again.

```sh
# see everything available, by kind
gh api repos/sei-protocol/sei-internal-skills/contents/scripts/install.sh \
  -H 'Accept: application/vnd.github.raw' | bash -s -- list
```

Then name what you want:

| Want | Command |
|---|---|
| **The output style** | `… \| bash -s -- output-style` |
| **One skill** | `… \| bash -s -- skill xreview` |
| **One agent** | `… \| bash -s -- agent idiomatic-reviewer` |

Written out in full, for the output style:

```sh
gh api repos/sei-protocol/sei-internal-skills/contents/scripts/install.sh \
  -H 'Accept: application/vnd.github.raw' | bash -s -- output-style
```

That lands `~/.claude/output-styles/asd-ste100.md` and prints how to turn it on. It
does **not** turn it on — see [Output styles](#output-styles).

`gh` is required rather than `curl` because sei-internal-skills is internal, so the fetch
needs its auth. Same trust model as the installer one-liner above.

**What it will not do:** delete anything, touch your `settings.json`, or install a
second resource you did not name. Ask for a skill and you get that skill — not its
agent, not the skills it references. If an agent names a skill it expects, take that
too.

Skills and agents come from either tier, and it tells you which:

```
✓ ~/.claude/skills/project-brief  (experimental)
  Experimental: parked in the repo, not part of the shipped core, and may change.
```

Three environment variables, if you need them:

| | |
|---|---|
| `SEI_SKILLS_REF` | Fetch from a branch or tag instead of `main` |
| `SEI_SKILLS_TARGET` | Install somewhere other than `$HOME` — a sibling repo, say |
| `SEI_INTERNAL_SKILLS_HOME` | Where the checkout lives. If one is there, a targeted install reads it instead of downloading |

```sh
# put one skill into another repo, from the checkout you already have
SEI_SKILLS_TARGET=~/work/platform bash ~/.sei-internal-skills/scripts/install.sh skill harbor-dev
```

## Daily use

Most work starts with one of these:

- **`/xreview`** — have the relevant specialists independently review a design, plan, diff, or set of expert outputs, then synthesize a findings table. Blinded, with an assigned dissenter.
- **`/root-cause`** — disciplined, multi-expert investigation of a complex problem. Signals before hypotheses; falsification before conclusion.
- **`/idiomatic`** then **`/systems`** — review code for language and package idiom, then for systems-level quality on top.
- **`/harbor-dev`** — spin up an ephemeral chain, attach an RPC fleet, run a bench, tear it down.
- **`/pr-quality`** — the locked pre-PR gate. **`/brevity`** — tighten the PR body.

Heavier orchestration — `/coral`, `/council`, `/bugbash`, `/design`, `/issue`, `/research`, `/workstream` — is [experimental](./experimental/README.md) and installs only on opt-in.

## The two tiers

The boundary between them is the point.

| | What it is | Who gets it |
|---|---|---|
| **`.claude/`** — the core | 17 skills, 17 agents. Focused on what an engineering team reaches for on ordinary work. | Everyone, via `make update` |
| **[`experimental/`](./experimental/README.md)** | 12 skills, 2 agents. Still forming, narrow audience, or exploratory. | Only on `make sync-experimental` |

The core is what every teammate installs, so anything added there costs everyone the
effort of filtering past it. That is the whole reason for the split: **a new skill starts
in `experimental/`** unless it clears the bar of serving an engineering team beyond its
author on ordinary work.

The exclusion is structural, not a setting. `sync-skills.sh` and `sync-agents.sh` read
`.claude/skills/` and `.claude/agents/` and nothing else, so a resource is excluded *by
living in* `experimental/`. Parking and promoting are the same one-line operation:

```sh
git mv experimental/skills/<name> .claude/skills/<name>   # promote (then: make verify-catalog)
git mv .claude/skills/<name> experimental/skills/<name>   # park
```

There is no third list to keep in step, which is what makes the boundary hold.

### Retiring something

Syncing never deletes — a target-only file is usually your own work, and a sync that
pruned by difference would eat it. The cost is that *retiring* a resource does not
un-install it: after the slim-down, every environment that had ever synced still
carried the removed skills, and Claude Code kept discovering them.

`make prune-retired` closes that gap. It reports; it does not act:

```sh
make prune-retired          # report what is stale. Deletes nothing.
make prune-retired-apply    # actually remove them
```

It distinguishes two kinds. **Retired** resources are gone from the repo entirely
(recoverable only from the archive) and are listed by hand in the script, so retiring
something is reviewed in a diff. **Parked** ones still live in `experimental/`, so
removing them is reversible with `make sync-experimental` — that list is derived at
runtime and cannot drift.

It will never remove a resource in the current core, or one it does not recognize —
a skill you authored yourself is reported and left alone. `make update` runs the
check and prints a one-line hint when something is stale, but never deletes.

A prior generation of this repo carried 33 skills and 22 agents, including several
product explorations. Those were cut in 2026-08 and preserved with full history in a
private snapshot rather than deleted outright.

## What's in here

- **Skills** (`.claude/skills/`) — 17 self-contained Claude Code skills, grouped by domain:
  - **Workflow** — `/xreview`
  - **Investigation** — `/root-cause`
  - **Code quality** — `/idiomatic`, `/systems`
  - **Writing quality** — `/language`
  - **Skill authoring** — `/author-skill`, `/audit-skill`
  - **Output quality** (sei-internal-skills-local) — `/brevity`, `/pr-quality`
  - **Platform infra** — `/platform`, `/kubernetes`
  - **Blockchain** — `/evm`
  - **Release operations** — `/chaos-suite`, `/validate-release`, `/gov-ops`, `/validator-platform`
  - **Engineer self-service** — `/harbor-dev`
- **Agents** (`.claude/agents/`) — 17 specialist personas dispatched by the skills (or directly via the Agent tool), grouped by domain:
  - **Platform infra** — `kubernetes-specialist`, `platform-engineer`, `network-specialist`, `k8s-capacity-management`, `sei-network-specialist`
  - **Observability** — `opentelemetry-expert`, `observability-platform-engineer`, `sre-engineer`
  - **Security** — `security-specialist`
  - **Blockchain** — `solidity-developer`
  - **Code quality** — `idiomatic-reviewer`, `systems-engineer`
  - **Writing quality** — `prose-steward`
  - **Product management** — `product-engineer`, `product-manager`, `go-to-market-specialist`
  - **Release operations** — `platform-release-manager`
- **[`experimental/`](./experimental/README.md)** — 12 skills and 2 agents that do **not** ship by default: workflow orchestration, exec reporting, `ebpf`/`bugbash`, and `interview`+`sei-interview-expert`. Opt in with `make sync-experimental`.
- **Sync machinery** (`scripts/`, `Makefile`):
  - `sync-skills.sh` / `sync-agents.sh` — copy skills/agents into user-scope (`~/.claude/`) or sibling repos, by domain or alias
  - `sync-output-styles.sh` — copy output styles into `~/.claude/output-styles/`; ships them, never activates them
  - `sync-experimental.sh` — opt-in installer for `experimental/`; never runs as part of update/sync-all/bootstrap
  - `install.sh` — the whole toolkit, or [one piece](#just-one-piece) without cloning
  - `Makefile` — `make bootstrap` (one-shot install), plus `make sync-skills` / `make sync-agents` / `make sync-output-styles` / `make sync-experimental`
  - `update-agent-permissions.sh` — installs the canonical read-only permission set

### Output styles

An **output style** governs how Claude writes its replies for a whole session. It is a
different layer from a skill (knowledge, invoked on demand) and an agent (a persona
dispatched for a task).

`make update` installs the styles into `~/.claude/output-styles/` but leaves every one of
them **off**. Activating a style rewrites assistant behavior in every session and every
repo, so that choice belongs to you, not to the installer — and writing it automatically
would overwrite anyone who already picked a different style.

Shipped: **ASD-STE100** — Simplified Technical English. Short sentences, active voice, one
meaning per word, outcome first. Turn it on with `/config` → Output Style → ASD-STE100, or
put `"outputStyle": "ASD-STE100"` in `~/.claude/settings.json`.

## Organization & selective sync

Skills and agents are grouped into **domains** for navigation and selective install — e.g. `code-quality` (`/idiomatic`, `/systems`), `release-operations` (`/gov-ops`, `/validate-release`), `platform-infra`, `investigation`, and so on. The domain is **metadata, not directory structure**: each skill/agent carries a `category:` in its frontmatter, the catalogs ([`.claude/skills/README.md`](.claude/skills/README.md), [`AGENTS.md`](AGENTS.md)) group by it, and the sync scripts let you install one domain at a time:

```sh
make sync-skills                                            # the `portable` set (default)
./scripts/sync-skills.sh --categories code-quality          # just one domain
./scripts/sync-skills.sh --categories all                   # everything syncable
```

Claude Code discovers skills/agents **flat** (`~/.claude/skills/<name>/`, `~/.claude/agents/<name>.md`) in both user and project scope — nested folders and custom roots like `~/.claude/sei-internal-skills/` are **not** discovered. So the install is always flat; domains never become on-disk folders. The aliases `portable`, `sei`, and `all` cross-cut the domains. (`output-quality` — `/brevity`, `/pr-quality` — is sei-internal-skills-local and intentionally not synced.)

## Repository structure

```
.claude/skills/             # THE CORE — skill definitions (SKILL.md + references + evals)
.claude/skills/README.md    #   Skill catalog — start here
.claude/skills/SKILL-TEMPLATE.md  #   Authoring standard for new skills
.claude/agents/             # THE CORE — specialist personas dispatched by the skills
.claude/output-styles/      # Response-format styles (shipped, opt-in — see Output styles)
experimental/               # Parked skills + agents; never installed by default
experimental/README.md      #   What's parked, and the promote/park mechanism
agents/                     # Omni agent bundles baked into the omnigent server image
sei-agent-driver/           # Go module — the headless review driver
scripts/                    # sync-*.sh, permission tooling, regression suites
.github/workflows/          # CI — catalog, doctrine, permissions, experimental isolation
AGENTS.md                   # Agent roster + the distributed operating-doctrine block
CLAUDE.md                   # Project context auto-loaded into every session
assets/                     # Repo logo used by this README
```

The two `.claude/` trees and `experimental/` are the tier split. `agents/` is unrelated
despite the name — those are omnigent server bundles, not Claude Code agent personas.

## Where to start

| If you're... | Start here |
|---|---|
| **Using the skills day to day** | `.claude/skills/README.md` (the catalog) |
| **Authoring a new skill** | Pick the tier first ([`experimental/README.md`](experimental/README.md) — it is the default), then `.claude/skills/SKILL-TEMPLATE.md`, then `/author-skill` |
| **Looking for a skill that isn't installed** | [`experimental/README.md`](experimental/README.md), then `make sync-experimental` |
| **Auditing an existing skill** | `/audit-skill <name>` → report in the DRI's `<engineer>-designs` repo under `designs/<arc>/audits/` (Design 13) |
| **Adding or editing an agent persona** | `.claude/agents/` + update the roster in `AGENTS.md` |
| **Wanting exactly one thing** | [Just one piece](#just-one-piece) — the same installer, with a target |
| **Wiring a sibling repo to use these** | `scripts/sync-agents.sh --target <path>` and `scripts/sync-skills.sh --target <path>` |

## Contributing & conventions

- **Conventional commits.** `feat:`, `fix:`, `docs:`, `refactor:` — reference the skill or component in scope (e.g. `feat(xreview): ...`, `docs(readme): ...`).
- **Brevity discipline.** Apply `/brevity` before writing PR bodies or WHY-style in-code comments.
- **PR-quality discipline.** Before `gh pr create`, apply `/pr-quality` to the staged diff + planned body.
- **Edit skills here, not in `~/.claude/`.** User-scope copies are overwritten on the next sync. Change a skill in sei-internal-skills and PR it.

## Documentation map

| Doc | What it covers |
|-----|----------------|
| `README.md` (this file) | Orientation, install, daily use, structure |
| `CLAUDE.md` | Project context auto-loaded into every Claude Code session |
| `AGENTS.md` | Agent roster + how the skills dispatch them |
| `.claude/skills/README.md` | Skill catalog and cross-repo sync guidance |
| `.claude/skills/SKILL-TEMPLATE.md` | Authoring standard for new skills |
| `experimental/README.md` | What is parked, why, and how to promote or park a resource |
| `scripts/README.md` | What each script does and when CI vs. humans run them |
