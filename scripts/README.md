# scripts/

Utility scripts for Tide repo maintenance. Most are wrapped by Make targets at the repo root (`make help`); the scripts can also be run directly when you need finer-grained control.

| Script | Purpose | Runs from |
|--------|---------|-----------|
| `sync-agents.sh` | Copy portable agents to other `.claude/agents/` directories | `make sync-agents`, manually |
| `sync-skills.sh` | Copy portable skills to other `.claude/skills/` directories | `make sync-skills`, manually |
| `update-agent-permissions.sh` | Install canonical read-only allow-list into `./.claude/settings.json` | `make update-agent-permissions` |
| `verify-agent-permissions.sh` | Fail if `.claude/settings.json` contains mutating patterns or has drifted | `make verify-agent-permissions`, CI |
| `agent-permissions.json` | Canonical read-only permission set (source of truth) | Read by both agent-permissions scripts |

---

## `sync-agents.sh`

Copies agent personas from `.claude/agents/` to a target `.claude/agents/` directory — typically user-level (`~/`) or a sibling repo. The category lists at the top of the script are the source of truth for which agents are portable.

```bash
# Sync portable agents to user-level (default category)
./scripts/sync-agents.sh --target ~/

# Portable + sei to a sibling repo
./scripts/sync-agents.sh --target ~/work/platform --categories portable,sei

# Preview without copying
./scripts/sync-agents.sh --target ~/ --dry-run
```

Categories: `portable` (default), `sei`, `all`. Non-destructive by default — pass `--force` to overwrite changed files.

## `sync-skills.sh`

Sibling of `sync-agents.sh` — same shape, same flags. Copies skills from `.claude/skills/` to a target `.claude/skills/` directory.

```bash
# Sync portable skills to user-level (default category)
./scripts/sync-skills.sh --target ~/

# Also sync the Sei skills (chaos-suite, harbor-dev, validate-release)
./scripts/sync-skills.sh --target ~/ --categories all

# Preview without copying
./scripts/sync-skills.sh --target ~/ --dry-run
```

Categories: `portable` (default), `sei`, `all`. Update the lists at the top of the script when a skill is added, renamed, or re-categorized.

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
