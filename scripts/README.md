# scripts/

Utility scripts for Tide repo maintenance. Most are wrapped by Make targets at the repo root (`make help`); the scripts can also be run directly when you need finer-grained control.

| Script | Purpose | Runs from |
|--------|---------|-----------|
| `sync-agents.sh` | Copy agents to other `.claude/agents/` directories (membership derived from each agent's `category:`) | `make update` / `make sync-agents`, manually |
| `sync-skills.sh` | Copy skills to other `.claude/skills/` directories (membership derived from each skill's `category:`) | `make update` / `make sync-skills`, manually |
| `update-agent-permissions.sh` | Install canonical read-only allow-list into `./.claude/settings.json` | `make update-agent-permissions` |
| `verify-agent-permissions.sh` | Fail if `.claude/settings.json` contains mutating patterns or has drifted | `make verify-agent-permissions`, CI |
| `agent-permissions.json` | Canonical read-only permission set (source of truth) | Read by both agent-permissions scripts |

---

## Get current in one command

**Never cloned Tide?** One line, straight over the wire — uses your `gh` auth (Tide is an internal repo, so a bare `curl` won't authenticate):

```bash
gh api repos/sei-protocol/Tide/contents/scripts/install.sh -H 'Accept: application/vnd.github.raw' | bash
```

It clones Tide to `~/.tide` (override with `TIDE_HOME`), then syncs ALL skills/agents into `~/.claude` and verifies the catalog. Idempotent — re-run any time.

**Already have the repo?** From your checkout:

```bash
make update     # fast-forward this checkout + sync ALL skills/agents into ~/.claude + verify the catalog
```

These are the only commands you need to keep your environment current. `make bootstrap` installs only the **portable** set into a *consumer* repo (external use); for your own `~/.claude` use `make update` (or the over-the-wire one-liner above).

**Single source of truth:** which alias (`portable` / `sei` / Tide-local) a skill or agent belongs to is **derived from its own `category:` frontmatter** via the small domain→alias map at the top of each sync script — there is no hand-maintained per-item list to drift. Add a skill/agent with a mapped `category:` and it syncs automatically. `make verify-catalog` (run in CI) fails closed if any item's category maps to no alias, so a miscategorized resource is caught, never silently dropped.

## `sync-agents.sh`

Copies agent personas from `.claude/agents/` to a target `.claude/agents/` directory — typically user-level (`~/`) or a sibling repo. Membership is derived from each agent's `category:` frontmatter; the domain→alias map at the top of the script is the only hand-maintained categorization.

```bash
# Sync portable agents to user-level (default category)
./scripts/sync-agents.sh --target ~/

# Portable + sei to a sibling repo
./scripts/sync-agents.sh --target ~/work/platform --categories portable,sei

# Preview without copying
./scripts/sync-agents.sh --target ~/ --dry-run
```

Categories: agent **domains** (`platform-infra`, `observability`, `security`, `blockchain`, `code-quality`, `writing-quality`, `product-management`, `release-operations`, `project-management`) or **aliases** `portable` (default, all non-Sei agents), `sei`, `all`. `--verify` runs only the coverage guard (CI). Non-destructive by default — pass `--force` to overwrite changed files.

## `sync-skills.sh`

Sibling of `sync-agents.sh` — same shape, same flags. Copies skills from `.claude/skills/` to a target `.claude/skills/` directory.

```bash
# Sync portable skills to user-level (default category)
./scripts/sync-skills.sh --target ~/

# Also sync the sei-team skills (impact-weekly, impact-portfolio, chaos-suite, validate-release, harbor-dev)
./scripts/sync-skills.sh --target ~/ --categories all

# Or install a single domain
./scripts/sync-skills.sh --target ~/ --categories project-management

# Preview without copying
./scripts/sync-skills.sh --target ~/ --dry-run
```

Categories: skill **domains** (`workflow`, `workstream-bootstrap`, `hardening`, `investigation`, `skill-authoring`, `code-quality`, `performance`, `writing-quality`, `product-management`, `project-management`, `release-operations`, `engineer-self-service`) or **aliases** `portable` (default), `sei`, `all`. `output-quality` (brevity, pr-quality) and `security` (tee) are Tide-local and not synced. `--verify` runs only the coverage guard (CI). To re-categorize a skill, edit its `category:` frontmatter — not this script; only a new/renamed **domain** (or a change to which alias it belongs to) needs a script edit.

## `update-agent-permissions.sh` + `verify-agent-permissions.sh` + `agent-permissions.json`

Together these three files manage the canonical read-only allow-list for subagent Bash and WebFetch calls.

```bash
# Install the canonical set into .claude/settings.json (idempotent)
./scripts/update-agent-permissions.sh
# or via make:
make update-agent-permissions

# Preview without writing
DRY_RUN=1 ./scripts/update-agent-permissions.sh

# Fail if settings.json has mutating patterns or has drifted from canonical
./scripts/verify-agent-permissions.sh
# or via make:
make verify-agent-permissions
```

`agent-permissions.json` is the source of truth — edit it to add or remove canonical patterns. Both scripts read it. The verify script also runs in CI on PRs that touch `.claude/settings.json` or any of these files (`.github/workflows/verify-agent-permissions.yml`).

**Read-only invariant.** The verify script enforces a deny-list across allow patterns: no `gh issue create / close / delete / edit`, no `gh pr create / merge / close / edit`, no `gh api -X POST/PUT/DELETE/PATCH` (or `--method` equivalents), no `aws ... <write-verb>-...` (delete-, put-, create-, update-, terminate-, etc.), no `kubectl <subcmd>` other than `get/describe/logs/top/explain/version/api-resources/api-versions`, no `flux <subcmd>` other than `get/describe/version/check`. The shared `.claude/settings.json` stays read-only forever; user-specific mutating patterns belong in `.claude/settings.local.json` (gitignored).

Drift is also a fail condition: `permissions.allow` in `.claude/settings.json` must equal the canonical set exactly. Local additions go in `settings.local.json`; CI fails the PR otherwise.
