# Scripts — author-skill

Deterministic steps used by the procedure. Each script is debuggable standalone — run with `-h` or invalid args to see usage.

## `scaffold.sh`

Creates the directory tree for a new skill at the resolved scope (project or user). Refuses to overwrite an existing non-empty directory. Refuses if the name collides with a protected canonical skill (coral, council, design, issue, bugbash, author-skill, chaos-suite, harbor-dev).

```bash
./scaffold.sh --name terraform-review \
              --scope project \
              --shape technique \
              --draft-dir /path/to/state/run-<ts>/draft
```

Exit codes:
- `0` — success; resolved path printed to stdout.
- `1` — bad args / missing flags.
- `2` — target exists and is non-empty.
- `3` — name collision with protected canonical skill.

## `add-catalog-entry.sh`

Appends a bullet to the appropriate section of `.claude/skills/README.md`. Idempotent — refuses to add a duplicate.

```bash
./add-catalog-entry.sh --name terraform-review \
                      --section "Hardening" \
                      --tagline "Pressure-tests terraform module changes against the team's review checklist." \
                      --dry-run
```

Exit codes:
- `0` — success; entry inserted (or dry-run preview printed).
- `1` — bad args.
- `2` — section heading not found.
- `3` — skill already in catalog.

## `sync-check.sh`

If the skill is portable (general-purpose) or Sei-ecosystem, adds it to `PORTABLE=( ... )` or `SEI=( ... )` in `scripts/sync-skills.sh`. Pass `--category none` to skip.

```bash
./sync-check.sh --name terraform-review --category portable --dry-run
```

Exit codes:
- `0` — success; entry added (or skipped if category=none, or dry-run preview printed).
- `1` — bad args.
- `3` — skill already in the chosen array.

## Authoring guidelines

When adding a new script to this skill:

- Flag-based args. No positional args.
- `set -euo pipefail` at the top.
- A `die` helper for consistent error formatting.
- A `--dry-run` flag for any side-effecting script — so the procedure can preview before applying.
- Exit codes documented in this README.
- Idempotent when possible — running twice should either succeed or refuse cleanly, not corrupt state.
- Portable across macOS (BSD tools) and Linux (GNU tools). Prefer `awk` over `sed -i`; prefer POSIX over GNU extensions.
