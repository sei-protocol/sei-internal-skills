# scripts/

This directory holds the install tooling, the contributor gates, and the inputs of the seidroid runner image. Most scripts have a make target at the repository root, and `make help` lists them. Run a script directly when you need a flag that its target does not pass.

## Engineer tooling

| Script | What it does | Run it with |
|---|---|---|
| `install.sh` | Installs the whole catalog when you give no arguments. With a target, it installs one output style, skill or agent and clones nothing. See [Install forms](#install-forms). | a one-liner, or `bash scripts/install.sh` |
| `sync-skills.sh` | Copies every skill into a `.claude/skills/` directory. | `make update`, or by hand |
| `sync-agents.sh` | Copies every agent into a `.claude/agents/` directory. | `make update`, or by hand |
| `sync-output-styles.sh` | Copies the output styles into a `.claude/output-styles/` directory. It never activates a style. | `make update`, `make sync-output-styles` |
| `prune-retired.sh` | Removes retired skills and agents from a synced `.claude/`. It is the only script here that deletes. Without `--apply` it only reports. It never removes a catalog resource or a resource it does not recognize. | `make prune-retired`, `make prune-retired-apply` |

## Contributor gates

| File | What it checks or holds | Run it with |
|---|---|---|
| `verify-references.sh` | Fails if a shipped artifact cites a skill that `.claude/skills/` does not hold. It has three error classes: ABSENT, STALE-MARKER and MISSING-SCRIPT. `--installed` reports against `~/.claude` and always exits 0. | `make verify-references`, CI |
| `skill-package-checks.sh` | Checks one skill (`--skill-dir <path>`) against the `[static]` rules in `skill-package-rubric.md`. | `make test-skill-package-checks`, CI |
| `skill-package-rubric.md` | The rules a skill holds. A reviewer cites them by rule ID. | read it |
| `verify-agent-permissions.sh` | Fails if `.claude/settings.json` holds a mutating pattern, or if its allow list differs from `agent-permissions.json`. | `make verify-agent-permissions`, CI |
| `update-agent-permissions.sh` | Writes the allow list from `agent-permissions.json` into `.claude/settings.json`. | `make update-agent-permissions` |
| `agent-permissions.json` | The canonical read-only permission set. Both permission scripts read it. | edit it |
| `verify-action-pins.sh` | Fails if a `uses:` ref under `.github/` names a tag or branch instead of a 40-hex commit sha. A local `./` action is exempt. A `docker://` image needs an `@sha256:` digest. | `make verify-action-pins`, CI |
| `sei-internal-skills-doctrine.md` | The operating doctrine. The sync writes it into a consuming repository's `AGENTS.md` as the managed block. | edit it, then `make sync-doctrine-self` |
| `lib/inject-doctrine.sh` | Writes or checks the managed doctrine block between its BEGIN and END markers. | `make sync-doctrine-self`, `make sync-doctrine-self-check` |
| `tests/catalog-coverage.test.sh` | The catalog guard fails closed, every skill and agent syncs, and the README counts match the tree. | `make test-catalog`, CI |
| `tests/verify-references.test.sh` | Regression suite for `verify-references.sh`. | `make test-references`, CI |
| `tests/skill-package-checks.test.sh` | Sweeps the checker over every skill. It diffs `block` failures against `tests/block-baseline.txt`. | `make test-skill-package-checks`, CI |
| `tests/install.test.sh` | Regression suite for `install.sh`. It runs offline. | `make test-install`, CI |
| `tests/prune-retired.test.sh` | Asserts what `prune-retired.sh` must not remove. | `make test-prune`, CI |
| `tests/sync-output-styles.test.sh` | Asserts that a sync never activates a style. | `make test-output-styles`, CI |
| `tests/inject-doctrine.test.sh` | Regression suite for `lib/inject-doctrine.sh`. | `make test-doctrine`, CI |

`make check` runs `make verify-catalog` and every check and suite in this table. It does not run `update-agent-permissions.sh` or `make sync-doctrine-self`, because they write files.

## seidroid runner inputs: do not move, rename or change

The runner image for seidroid bakes three of these files. The runner workflows mount the fourth, `check-runner-credential-bridge.sh`, by path. A move or a rename breaks the image build or its gate.

| File | What it does in the runner |
|---|---|
| `managed-settings.json` | The runner's admin permission policy. The image installs it read-only at `/etc/claude-code/managed-settings.json`. |
| `sei-runner-credential-check` | The readiness check for the GitHub credential bridge. A `profile.d` hook runs it at the start of each login shell. |
| `check-runner-credential-bridge.sh` | The CI proof of the credential bridge. CI runs it inside the image with a fake token. |
| `sei-writing-lint` | Lints prose against the writing contract inside a sandbox. The image installs it as `/usr/local/bin/sei-writing-lint`. |

`managed-settings.json` is wide on purpose. It allows `Bash`, `Read`, `Grep`, `Glob`, `Agent` and `Task`, and `WebFetch` on three domains. A headless session has no person to answer a permission prompt. The runner gate checks three things only:

- The file exists.
- The agent uid cannot write it.
- It parses as JSON, and its allow list is not empty.

No gate reviews what the list allows.

## Install forms

Each form installs the same catalog. Pick one.

```sh
# curl: no auth
curl -fsSL https://raw.githubusercontent.com/sei-protocol/sei-internal-skills/main/scripts/install.sh | bash

# gh
gh api repos/sei-protocol/sei-internal-skills/contents/scripts/install.sh \
  -H 'Accept: application/vnd.github.raw' | bash

# clone, read, then install
git clone https://github.com/sei-protocol/sei-internal-skills ~/.sei-internal-skills
make -C ~/.sei-internal-skills update
```

A one-liner clones the repository into `~/.sei-internal-skills` and runs `make sync-all`. To use a different path, set the variable on `bash`, not on `curl`: `curl -fsSL <url> | SEI_INTERNAL_SKILLS_HOME=<path> bash`. You can run it again at any time. To install one piece, append `-s -- <target> [name]` to a one-liner. `bash scripts/install.sh -h` lists the targets.

**Trust.** A one-liner runs `install.sh` from `main` against your `~/.claude`, the same trust as a clone followed by `make`. GitHub is the integrity anchor: the script comes over HTTPS from this public repository, which anyone can read. The one-liner tracks `main` and pins no ref, so you always get the current catalog. To read the script before it runs, use the clone form.

## sync-skills.sh and sync-agents.sh

Both scripts take the same flags. `sync-skills.sh` copies every skill, which is a directory under `.claude/skills/` that holds a `SKILL.md`. `sync-agents.sh` copies every `.claude/agents/*.md`.

```sh
# Copy every skill and agent into user scope, as make sync-all does
./scripts/sync-skills.sh --target ~/ --force
./scripts/sync-agents.sh --target ~/ --force

# Copy into a sibling repository, and overwrite changed files
./scripts/sync-skills.sh --target ~/work/platform --force

# Preview without a copy
./scripts/sync-skills.sh --target ~/ --dry-run

# Run only the catalog guard (CI)
./scripts/sync-skills.sh --verify
./scripts/sync-agents.sh --verify

# Also write the operating-doctrine block into the repository's AGENTS.md
./scripts/sync-skills.sh --target <repo> --inject-doctrine
```

Without `--force`, a script skips each target copy that differs from the source, reports it as a conflict, and then exits 1. For a skill, a file that the target copy lacks also counts as a difference, and the script skips the whole skill. With `--force`, the script overwrites the target copy. In both modes, a file that exists only in the target stays.

## Agent permissions

`agent-permissions.json` holds the canonical read-only allow list for subagent Bash and WebFetch calls. Edit that file to add or remove a pattern. Both permission scripts read it.

```sh
# Write the canonical set into .claude/settings.json. A re-run changes nothing.
make update-agent-permissions

# Preview without a write
DRY_RUN=1 ./scripts/update-agent-permissions.sh

# Fail on a mutating pattern or on drift from the canonical set
make verify-agent-permissions
```

The verify script applies a deny list to every allow pattern:

- No `gh issue create`, `close`, `delete` or `edit`, and no `gh pr create`, `merge`, `close` or `edit`.
- No `gh api -X POST`, `PUT`, `DELETE` or `PATCH`, or the `--method` form.
- No `aws` write verb, such as `delete-`, `put-`, `create-`, `update-` or `terminate-`. The script holds the full list.
- No `kubectl` subcommand other than `get`, `describe`, `logs`, `top`, `explain`, `version`, `api-resources` and `api-versions`.
- No `flux` subcommand other than `get`, `describe`, `version` and `check`.

Drift also fails: `permissions.allow` in `.claude/settings.json` must equal the canonical set. A personal or mutating pattern goes outside that file. [Permission pre-approval](../.claude/skills/SKILL-TEMPLATE.md#permission-pre-approval) in the skill template says where. CI runs the verify script on a pull request that touches `.claude/settings.json` or one of these files.
