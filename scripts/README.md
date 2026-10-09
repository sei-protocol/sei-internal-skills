# scripts/

Utility scripts for sei-internal-skills repo maintenance. Make targets at the repo root wrap most of them (`make help`); run a script directly when you need finer-grained control.

| Script | Purpose | Runs from |
|--------|---------|-----------|
| `install.sh` | Install the whole toolkit (no arguments), or take one piece — an output style, a skill, an agent — without cloning | over the wire, or `bash scripts/install.sh` |
| `sync-agents.sh` | Copy every agent into a `.claude/agents/` directory | `make update`, manually |
| `sync-skills.sh` | Copy every skill into a `.claude/skills/` directory | `make update`, manually |
| `sync-output-styles.sh` | Copy output styles to other `.claude/output-styles/` directories. Ships the file; **never** activates it — activation is opt-in per user | `make update` / `make sync-output-styles`, manually |
| `update-agent-permissions.sh` | Install canonical read-only allow-list into `./.claude/settings.json` | `make update-agent-permissions` |
| `verify-agent-permissions.sh` | Fail if `.claude/settings.json` contains mutating patterns or has drifted | `make verify-agent-permissions`, CI |
| `verify-action-pins.sh` | Fail if a `uses:` ref in any `.yml`/`.yaml` under `.github/` names a tag or branch instead of a 40-hex commit sha. A local `./` action is exempt; a `docker://` image needs an `@sha256:` digest | `make verify-action-pins`, CI |
| `tests/install.test.sh` | Regression suite for `install.sh`'s targeted mode, including the piped invocation | `make test-install`, CI |
| `prune-retired.sh` | Remove retired resources from a synced `.claude/`. **The only script here that deletes** — dry-run by default, `--apply` to act. Never touches a core or unrecognized resource | `make prune-retired` / `make prune-retired-apply`, manually |
| `verify-references.sh` | Fail if a shipped artifact cites a resource an engineer cannot reach. Three error classes (ABSENT, STALE-MARKER, MISSING-SCRIPT) and no warning class. `--installed` reports against `~/.claude` and never gates. | CI + `make verify-references` |
| `tests/prune-retired.test.sh` | Regression suite for `prune-retired.sh` — asserts what it must NOT remove | `make test-prune`, CI |
| `skill-package-checks.sh` + `skill-package-rubric.md` | The skill-package checker and the rules it cites | `make test-skill-package-checks`, CI |
| `tests/skill-package-checks.test.sh` | Sweeps `scripts/skill-package-checks.sh` over every skill; asserts it completes and emits parseable JSON, and diffs block failures against `tests/block-baseline.txt` | `make test-skill-package-checks`, CI |
| `agent-permissions.json` | Canonical read-only permission set (source of truth) | Read by both agent-permissions scripts |
| `tests/sync-output-styles.test.sh` | Regression suite for `sync-output-styles.sh` — most importantly, that sync never activates a style | `make test-output-styles`, CI |

---

## Get current in one command

**Never cloned sei-internal-skills?** One line, straight over the wire — uses your `gh` auth (sei-internal-skills is an internal repo, so a bare `curl` will not authenticate):

```bash
gh api repos/sei-protocol/sei-internal-skills/contents/scripts/install.sh -H 'Accept: application/vnd.github.raw' | bash
```

It clones sei-internal-skills to `~/.sei-internal-skills` (override with `SEI_INTERNAL_SKILLS_HOME`), then syncs every skill and agent into `~/.claude` and verifies the catalog. It is idempotent: re-run it any time.

> **Trust note.** This executes whatever is on `sei-protocol/sei-internal-skills@main` against your `~/.claude` — the same trust as cloning sei-internal-skills and running `make`. `gh` gates *who* can fetch (org members only); GitHub is the integrity anchor. It intentionally tracks `main` (no pinned ref) so you always get current. Prefer to read before you run? `gh repo clone sei-protocol/sei-internal-skills ~/.sei-internal-skills && make -C ~/.sei-internal-skills update`.

**Already have the repo?** From your checkout:

```bash
make update     # fast-forward this checkout + sync ALL skills/agents/output-styles into ~/.claude + verify the catalog
```

`make verify-catalog` (CI) fails if a skill's `name:` does not match its directory, or an agent's `name:` does not match its file.

## `sync-skills.sh` and `sync-agents.sh`

Both scripts take the same flags. `sync-skills.sh` copies every skill (a directory under `.claude/skills/` that holds a `SKILL.md`). `sync-agents.sh` copies every `.claude/agents/*.md`.

```bash
# Copy every skill and agent into user scope
./scripts/sync-skills.sh --target ~/
./scripts/sync-agents.sh --target ~/

# Copy into a sibling repo, overwriting changed files
./scripts/sync-skills.sh --target ~/work/platform --force

# Preview without copying
./scripts/sync-skills.sh --target ~/ --dry-run

# Run only the catalog guard (CI)
./scripts/sync-skills.sh --verify
./scripts/sync-agents.sh --verify

# Also write the operating-doctrine block into the repo's AGENTS.md
./scripts/sync-skills.sh --target <repo> --inject-doctrine
```

Without `--force`, a script reports a changed target file as a conflict and skips it. A sync never deletes a file that exists only in the target.

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

**Read-only invariant.** The verify script enforces a deny-list across allow patterns. No `gh issue create / close / delete / edit`, no `gh pr create / merge / close / edit`, and no `gh api -X POST/PUT/DELETE/PATCH` (or `--method` equivalents). No `aws ... <write-verb>-...` (`delete-`, `put-`, `create-`, `update-`, `terminate-`, and the rest). No `kubectl <subcmd>` other than `get/describe/logs/top/explain/version/api-resources/api-versions`, and no `flux <subcmd>` other than `get/describe/version/check`. The shared `.claude/settings.json` stays read-only forever; user-specific mutating patterns belong in `.claude/settings.local.json` (gitignored).

Drift is also a fail condition: `permissions.allow` in `.claude/settings.json` must equal the canonical set exactly. Local additions go in `settings.local.json`; CI fails the PR otherwise.
