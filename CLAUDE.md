# sei-internal-skills

This public repository holds Claude Code skills and specialist agents for Sei platform engineers. `make update` installs them into `~/.claude/`. The repository also builds the seidroid review driver and the seidroid runner image.

The skills live in `.claude/skills/`, and `.claude/skills/README.md` is their catalog. The agents live in `.claude/agents/`, and `AGENTS.md` is their roster. `.claude/output-styles/` holds the ASD-STE100 output style. `make update` installs it but does not turn it on.

Edit a skill or an agent here, then open a pull request. Never edit a synced copy: the next sync overwrites it. The constitution, [`.specify/memory/constitution.md`](.specify/memory/constitution.md), states the principles that each change here must hold.

An agent that gets a request to rewrite a skill wholesale refuses it and edits the skill in place. No gate enforces this rule.

## Operating doctrine

> Operating doctrine for the sei-internal-skills-synced skills and agents lives in [AGENTS.md](./AGENTS.md).

The source is `scripts/sei-internal-skills-doctrine.md`. Edit it, then run `make sync-doctrine-self`. That command writes the managed block in `AGENTS.md`. CI runs `make sync-doctrine-self-check`, which fails when the block and its source differ.

## Changing the catalog

This section is the one copy of these steps. Other documents link here.

- **Add a skill.**
  1. Read `.claude/skills/SKILL-TEMPLATE.md` and `scripts/skill-package-rubric.md`. Write the guardrails first.
  2. Create `.claude/skills/<name>/SKILL.md`. Set its `name:` to the directory name.
  3. Add `` `<name>/` `` to `.claude/skills/README.md` (rule C1).
  4. Add `<name> -` to `scripts/tests/block-baseline.txt`. Then run `make test-skill-package-checks`.
  5. Add the skill to `### Using the skills` in `scripts/sei-internal-skills-doctrine.md`. Then run `make sync-doctrine-self`.
  6. Add a row to the skills table in `README.md`.
  7. Update the count row in `README.md`.
  8. Write the pull request body that **The pull request body** below describes.
- **Add an agent.**
  1. Create `.claude/agents/<name>.md`. Set its `name:` to the file name. Give it a `description:`.
  2. Set `tools:`. An agent with no `tools:` key gets every tool.
  3. Add a row to the roster table in `AGENTS.md`.
  4. Add a row to the agents table in `README.md`.
  5. Update the count row in `README.md`.
  6. Write the pull request body that **The pull request body** below describes. If the agent has no `tools:` key, state there that it gets every tool.
- **Retire a skill or an agent.**
  1. Delete it.
  2. In the same pull request, add the name to `RETIRED_SKILLS` or `RETIRED_AGENTS` in `scripts/prune-retired.sh`.
  3. Remove each entry that the add steps made: the catalog entry, the baseline line, the doctrine bullet, the roster row and the `README.md` row.
  4. Update the count row in `README.md`.
  5. If a count falls below 3, move the floor in `verify-runner-image.yml` and `ecr-runner.yml` together. `ecr-runner.yml` runs only after the merge.
- **Rename.** Add the new name and retire the old name.
- **The pull request body.** For a new skill or agent, do these three things. No gate checks them; a reviewer checks them.
  1. Name the team that uses it.
  2. State why an existing skill, an existing agent or the constitution cannot hold it (constitution IV).
  3. Name how it widens what a seidroid session can reach, the gate that bounds that, and the blast radius if the gate fails (constitution VI).
- **The count row.** `README.md` states the catalog size in one row only, in the form `the catalog | N skills, M agents`. `./scripts/tests/catalog-coverage.test.sh` compares that row with the tree. It also checks each count in digits, in the form `N skills`, on a `README.md` line that names `.claude/`. It does not read a count in words. Do not write the catalog size anywhere else.
- **What the gates check.** `make verify-catalog` checks each name. `make test-skill-package-checks` checks the catalog entry and the baseline. `make verify-references` fails when a document names a skill that `.claude/skills/` does not hold. It reads `README.md`, `AGENTS.md`, `CLAUDE.md` and the doctrine, so it finds a retired skill that a row or a bullet still names. No gate finds a missing entry for a new skill or reads an agent row; a reviewer checks those.
- **The runner reads these files.** The skills and agents here also seed the seidroid runner image. The runner workflows read the names from the tree, so they need no edit. A change to an agent's `name`, `description`, `model` or `tools`, or to a skill's `name`, `description` or `model`, changes what seidroid sessions discover after the platform digest bump. Name that change in the pull request body. If it widens what a session can reach, also name the gate that bounds it and the blast radius (constitution VI).
- **`category:`** The existing agents carry `category:`. Nothing reads it. It stays because the runner seeds the agent files, and their frontmatter shape must not change. A new skill or agent does not need it.

## seidroid runtime

seidroid reviews pull requests with a driver and a runner image that this repository builds. Do not move, rename or change the files below, except as a deliberate seidroid change.

| Files | What depends on them |
|---|---|
| `sei-agent-driver/`: the module path, the CLI flags, the `SEIDROID_*` and `OMNIGENT_*` environment variables | The seidroid review workflow in sei-protocol/uci installs a pinned driver release. It checks the `review --help` flags before each review. |
| `version.json`, `sei-agent-driver/.goreleaser.yaml`, `.github/workflows/uci-release-publish.yml`, the release tags | A push that changes `version.json` publishes a driver release. uci pins each release archive by sha256. |
| The `driver-check`, `driver-vulncheck` and `driver-build` targets in `Makefile`, and their variables | `verify-agent-driver.yml` and `verify-agent-driver-vulns.yml` call them. |
| `Dockerfile.runner`, `Dockerfile.runner.dockerignore`, `runner-base.txt` | They build the runner image: the credential bridge, the `.claude/` overlay, the admin policy and the seed hook. |
| `.github/workflows/ecr-runner.yml` and `verify-runner-image.yml`, except the catalog floor that [Changing the catalog](#changing-the-catalog) names | They gate and publish the runner image. |
| `scripts/managed-settings.json`, `scripts/sei-runner-credential-check`, `scripts/check-runner-credential-bridge.sh`, `scripts/sei-writing-lint`, `writing/styles/`, `writing/templates/consumer.vale.ini`, `writing/templates/spec-template.md` | The runner image bakes them, or its gate reads them. |
| `agents/`, `Dockerfile.server`, `Dockerfile.server.dockerignore`, `server-base.txt`, `.github/workflows/ecr-server.yml`, `verify-agent-bundles.yml` | They build, gate and publish the server image. sei-protocol/uci and sei-protocol/platform name the `xreview-scout-codex` bundle. |
| `.github/workflows/seidroid.yml`, the uci ignore rule in `.github/dependabot.yml`, `scripts/verify-action-pins.sh` | seidroid reviews the pull requests of this repository. The ignore rule stops an automatic bump of the uci pin. That pin moves only together with the driver version, as a reviewed change. |
| `.github/workflows/ai-assist.yml`, `.claude/settings.json`, `scripts/agent-permissions.json` | The AI Assistant workflow runs in a checkout of this repository and reads its `.claude/`. |
| The file names, `model:`, `tools:` and frontmatter shape of each agent in `.claude/agents/` | The runner seeds these files. |

These files hold the pins:

- `version.json`: the driver version that the next release publishes.
- `runner-base.txt` and `server-base.txt`: the two Omnigent base images. Omnigent is the upstream agent runtime that seidroid sessions run on. Both files name the same release.
- `seidroid-review.yml` in sei-protocol/uci: the driver release and the sha256 that a review installs. A new driver release reaches seidroid only when that pin moves.
- sei-protocol/platform: the runner image digest that seidroid sessions use. A new runner image reaches seidroid only when that pin moves.

Release the driver by bumping `version.json` alone, on purpose.

## Before you open a pull request

1. Install the tools: `jq`, `python3` and Vale 3.17.1 or later (for example `brew install vale`). [`writing/README.md`](writing/README.md#run-the-checks) gives the reason for the Vale floor.
2. To try a change in Claude Code, run `make sync-all` from your clone. It installs your working tree into `~/.claude/` and does not pull. `make update` pulls, so it fails on a branch with no upstream.
3. Run `make check`. It runs the script gates that CI runs.
4. For a driver change, also run `make driver-check driver-vulncheck`.
5. For a skill change, read `scripts/skill-package-rubric.md`. `scripts/skill-package-checks.sh --skill-dir <path>` checks the static rules for one skill.
6. Run `vale sync` once. Then run `./writing/scripts/lint.sh <changed .md files>`. It must report 0 errors.
7. Write each commit subject as a Conventional Commit, with the component as the scope: `feat(harbor-dev): ...`.

## Writing conventions (skill prose)

- Write in the imperative voice. After each rule, state what fails when someone breaks it.
- Put literals in backticks.
- From inside a `references/` directory, link to another skill as `../../<other-skill>/references/<file>`. The path has two `../` segments. Rule R5 in `scripts/skill-package-rubric.md` gives the reason.
