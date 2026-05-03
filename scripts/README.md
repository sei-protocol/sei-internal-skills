# scripts/

Utility scripts for Tide repo maintenance. Most are wrapped by Make targets at the repo root (`make help`); the scripts can also be run directly when you need finer-grained control.

| Script | Purpose | Runs from |
|--------|---------|-----------|
| `verify_registry.py` | Code ↔ registry consistency check | CI, pre-commit hook, `/verify` skill |
| `sync-agents.sh` | Copy portable agents to other `.claude/agents/` directories | `make sync-agents`, operator, manually |
| `update-agent-permissions.sh` | Install canonical read-only allow-list into `./.claude/settings.json` | `make update-agent-permissions` |
| `verify-agent-permissions.sh` | Fail if `.claude/settings.json` contains mutating patterns or has drifted | `make verify-agent-permissions`, CI |
| `agent-permissions.json` | Canonical read-only permission set (source of truth) | Read by both agent-permissions scripts |
| `pre-commit-hook.sh` | Local git hook that gates interface-touching commits on `verify_registry.py` | Local git, after install |

---

## `verify_registry.py`

Checks that code in `pkg/`, `runtimes/`, `contracts/`, and `manifests/` is consistent with `tide/interface-registry.yaml`. Specifically:

- Env var names referenced from `runtimes/**/*.py` exist in the registry
- ServiceAccount usage in `pkg/**/*.go` matches the per-agent pattern (`tide-agent-{name}`)
- Contract function names called from runtimes match the registry's canonical names
- The `git_token_path` is consistent across components

```bash
python scripts/verify_registry.py [--repo-root /path/to/tide-repo]
```

Exit codes: `0` pass, `1` mismatches found (printed), `2` registry missing or unparseable.

Requires PyYAML.

Invoked from:
- **CI** — `.github/workflows/verify-interfaces.yml` runs it on every PR that touches interface-relevant paths
- **Pre-commit** — `pre-commit-hook.sh` (below)
- **`/verify` skill** — augments mechanical checks with manual ones (event topic hashes, exit-code handling, K8s resource naming)

## `sync-agents.sh`

Copies agent personas from `.claude/agents/` to a target `.claude/agents/` directory — typically user-level (`~/`) or a sibling repo. The portable / sei / tide-only category lists at the top of the script are the source of truth for which agents are portable.

```bash
# Sync portable agents to user-level (default category)
./scripts/sync-agents.sh --target ~/

# Portable + sei to a sibling repo
./scripts/sync-agents.sh --target ~/tide-workspace/platform --categories portable,sei

# Preview without copying
./scripts/sync-agents.sh --target ~/ --dry-run
```

Categories: `portable` (default), `sei`, `tide-only`, `all`. Non-destructive by default — pass `--force` to overwrite changed files.

Tide-only agents (`blockchain-developer`, `reviewer`) deliberately do **not** sync — they encode Tide-specific context that wouldn't translate.

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

## `pre-commit-hook.sh`

Local git hook that runs `verify_registry.py` before commits that touch the interface boundary (`pkg/`, `runtimes/`, the registry itself, `manifests/`). Skips silently for unrelated commits.

Install:

```bash
cp scripts/pre-commit-hook.sh .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

Same script CI runs — local enforcement matches PR enforcement.
