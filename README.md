<p align="center">
  <img src="assets/sei-internal-skills-logo.png" alt="sei-internal-skills" width="100%">
</p>

# sei-internal-skills

sei-internal-skills is Sei's library of **portable Claude Code skills and specialist agents** for engineering work. It is the centralized, version-controlled home for those workflows and personas. They help us review code, investigate failures, operate releases, run ephemeral chains, and collaborate with specialist agents.

You author skills and agents once here and sync them out to your user-scope (`~/.claude/`) and sibling repos. The same `/harbor-dev`, `/kubernetes`, or `kubernetes-specialist` then works the same way everywhere.

## Setup

One-liner

```sh
gh api repos/sei-protocol/sei-internal-skills/contents/scripts/install.sh -H 'Accept: application/vnd.github.raw' | bash
```

Or clone into `~/.sei-internal-skills`, then run `make update`.

The canonical permission set is **strictly read-only by design**. `make verify-agent-permissions` rejects mutating patterns (`gh issue create`, `gh pr merge`, `aws delete-*`, `kubectl apply`, etc.), and CI runs it on every PR that touches the permission files. Local additions for your own workflow go in `.claude/settings.local.json` (gitignored).

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
| **One skill** | `… \| bash -s -- skill harbor-dev` |
| **One agent** | `… \| bash -s -- agent sre-engineer` |

Written out in full, for the output style:

```sh
gh api repos/sei-protocol/sei-internal-skills/contents/scripts/install.sh \
  -H 'Accept: application/vnd.github.raw' | bash -s -- output-style
```

That lands `~/.claude/output-styles/asd-ste100.md` **and turns it on**, because naming
a style is the request to use it. It takes effect in your next session, or after
`/clear` in the current one.

It will not overwrite a style you already chose. If `outputStyle` is already set to
something else, that gets reported and left alone. It keeps every other key in
`settings.json` and backs the file up before the write. It refuses a `settings.json`
that does not parse rather than guessing at it. It edits through a symlinked
`settings.json` rather than replacing it, so a dotfiles setup stays linked.
`--no-activate output-style` installs the file and stops.

The fetch needs `gh` rather than `curl` because sei-internal-skills is internal, so it
needs `gh` auth. Same trust model as the installer one-liner above.

**What it will not do:** delete anything, or install a second resource you did not
name. Only `output-style` writes `settings.json`; a skill or agent never does. Ask for a skill and you get that skill — not its
agent, not the skills it references. If an agent names a skill it expects, take that
too.

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

- **`/harbor-dev`**: spin up an ephemeral chain, attach an RPC fleet, run a bench, tear it down.
- **`/giga-dev`**: how the shared giga testnet works, where its logs and metrics live, and how to diagnose it.
- **`/kubernetes`**: design or review sei-k8s-controller code.

## The catalog

| | What it is | Who gets it |
|---|---|---|
| **`.claude/`** — the catalog | 3 skills, 3 agents. | Everyone, via `make update` |

### Retiring something

Syncing never deletes — a target-only file is often your own work, and a sync that
pruned by difference would eat it. The cost is that *retiring* a resource does not
un-install it. After the slim-down, every environment that had ever synced still
carried the removed skills, and Claude Code kept discovering them.

`make prune-retired` closes that gap. It reports; it does not act:

```sh
make prune-retired          # report what is stale. Deletes nothing.
make prune-retired-apply    # actually remove them
```

It will never remove a resource in the current core, or one it does not recognize.
It reports a skill you authored yourself and leaves it alone. `make update` runs the
check and prints a one-line hint when something is stale, but never deletes.

To move an existing install to the three-skill catalog:

1. `make -C ~/.sei-internal-skills update`. It pulls and installs the three skills and three agents.
2. `make -C ~/.sei-internal-skills prune-retired`. It lists what step 4 deletes, and deletes nothing.
3. Read the RETIRED list. Prune matches by name only. If a listed name is your own skill, or a retired skill you edited, copy it to a new name first. If a listed skill says `has state/ files` (for example a gov-ops audit log), copy that `state/` out first.
4. `make -C ~/.sei-internal-skills prune-retired-apply`. It deletes the RETIRED list from `~/.claude`.
5. For each repository you synced skills into: `~/.sei-internal-skills/scripts/prune-retired.sh --target <repo>`, then the same command with `--apply`.
6. Optional: `git -C ~/.sei-internal-skills clean -ndX -- .claude/skills experimental` lists ignored files that removed skills left in your checkout. They are harmless; delete the ones you do not need.
7. Restart any open Claude Code session.

## What's in here

- **Skills** (`.claude/skills/`) — 3 self-contained Claude Code skills: `/harbor-dev`, `/giga-dev`, `/kubernetes`.
- **Agents** (`.claude/agents/`) — 3 specialist personas: `kubernetes-specialist`, `sei-network-specialist`, `sre-engineer`.
- **Sync machinery** (`scripts/`, `Makefile`):
  - `sync-skills.sh` / `sync-agents.sh` — copy every skill and agent into user scope (`~/.claude/`) or a sibling repo
  - `sync-output-styles.sh` — copy output styles into `~/.claude/output-styles/`; ships them, never activates them
  - `install.sh` — the whole toolkit, or [one piece](#just-one-piece) without cloning
  - `Makefile` — `make update`, `make sync-output-styles`, `make prune-retired`
  - `update-agent-permissions.sh` — installs the canonical read-only permission set

### Output styles

An **output style** governs how Claude writes its replies for a whole session. It is a
different layer from a skill (knowledge, invoked on demand) and an agent (a persona
dispatched for a task).

`make update` installs the styles into `~/.claude/output-styles/` but leaves every one of
them **off**. Activating a style rewrites assistant behavior in every session and every
repo, so that choice belongs to you, not to the installer. Writing it automatically
would overwrite anyone who already picked a different style.

Shipped: **ASD-STE100** — Simplified Technical English. Short sentences, active voice, one
meaning per word, outcome first. Turn it on with `/config` → Output Style → ASD-STE100, or
put `"outputStyle": "ASD-STE100"` in `~/.claude/settings.json`.

## Repository structure

```
.claude/skills/             # THE CORE — skill definitions (SKILL.md + references + evals)
.claude/skills/README.md    #   Skill catalog — start here
.claude/skills/SKILL-TEMPLATE.md  #   Authoring standard for new skills
.claude/agents/             # THE CORE — specialist personas dispatched by the skills
.claude/output-styles/      # Response-format styles (shipped, opt-in — see Output styles)
agents/                     # Omni agent bundles baked into the omnigent server image
sei-agent-driver/           # Go module — the headless review driver
scripts/                    # sync-*.sh, permission tooling, regression suites
.github/workflows/          # CI — catalog, doctrine, permissions, runner image
AGENTS.md                   # Agent roster + the distributed operating-doctrine block
CLAUDE.md                   # Project context auto-loaded into every session
assets/                     # Repo logo used by this README
```

`agents/` is unrelated despite the name — those are omnigent server bundles, not agent personas for Claude Code.

## Where to start

| If you are... | Start here |
|---|---|
| **Using the skills day to day** | `.claude/skills/README.md` (the catalog) |
| **Authoring a new skill** | [`CLAUDE.md` § Changing the catalog](CLAUDE.md#changing-the-catalog) |
| **Auditing a skill** | `scripts/skill-package-checks.sh --skill-dir <path>`, then `scripts/skill-package-rubric.md` |
| **Adding or editing an agent persona** | `.claude/agents/` + update the roster in `AGENTS.md` |
| **Wanting exactly one thing** | [Just one piece](#just-one-piece) — the same installer, with a target |
| **Wiring a sibling repo to use these** | `scripts/sync-agents.sh --target <path>` and `scripts/sync-skills.sh --target <path>` |

## Contributing & conventions

- **Conventional commits.** `feat:`, `fix:`, `docs:`, `refactor:` — reference the skill or component in scope (e.g. `feat(harbor-dev): ...`, `docs(readme): ...`).
- **Output discipline.** Follow the Output discipline in `AGENTS.md` for PR bodies and in-code comments. Conclusion first, an in-body comment at 4 lines or fewer, a header at 20 or fewer.
- **Pre-PR discipline.** Before `gh pr create`, re-read the diff and the planned body against the Output discipline in `AGENTS.md`.
- **Edit skills here, not in `~/.claude/`.** The next sync overwrites user-scope copies. Change a skill in sei-internal-skills and PR it.

## Documentation map

| Doc | What it covers |
|-----|----------------|
| `README.md` (this file) | Orientation, install, daily use, structure |
| `CLAUDE.md` | Project context auto-loaded into every Claude Code session |
| `AGENTS.md` | Agent roster + how the skills dispatch them |
| `.claude/skills/README.md` | Skill catalog and cross-repo sync guidance |
| `.claude/skills/SKILL-TEMPLATE.md` | Authoring standard for new skills |
| `scripts/README.md` | What each script does and when CI vs. humans run them |
