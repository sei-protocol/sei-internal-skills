# scripts/

Utility scripts for Tide repo maintenance. Three scripts, three jobs:

| Script | Purpose | Runs from |
|--------|---------|-----------|
| `verify_registry.py` | Code ↔ registry consistency check | CI, pre-commit hook, `/verify` skill |
| `sync-agents.sh` | Copy portable agents to other `.claude/agents/` directories | Operator, manually |
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

## `pre-commit-hook.sh`

Local git hook that runs `verify_registry.py` before commits that touch the interface boundary (`pkg/`, `runtimes/`, the registry itself, `manifests/`). Skips silently for unrelated commits.

Install:

```bash
cp scripts/pre-commit-hook.sh .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

Same script CI runs — local enforcement matches PR enforcement.
