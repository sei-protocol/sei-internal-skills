# sei-internal-skills Engineering Workspace

## Project Context

sei-internal-skills is Sei's centralized library of **portable Claude Code skills and specialist agents** for engineering work. Engineers author and version-control skills and agents here, then sync them out to user-scope (`~/.claude/`) and sibling repos via `scripts/sync-skills.sh` and `scripts/sync-agents.sh`. This repo is the canonical home — edit here and PR; never edit the synced copies.

The repo ships three skills and three agents in `.claude/`.

## Operating Doctrine

> Operating doctrine for the sei-internal-skills-synced skills and agents lives in [AGENTS.md](./AGENTS.md).

`scripts/sei-internal-skills-doctrine.md` holds the doctrine **once**, and the sync distributes it — to this repo and to every consuming package — as the `sei-internal-skills-managed` block in [AGENTS.md](./AGENTS.md). The doctrine covers the engineering principles, the output discipline (incl. the comment and documentation standard with its numeric bounds) and the three skills. Edit the doctrine there, then run `make sync-doctrine-self`. The specialist roster also lives in `AGENTS.md`; the full skill catalog lives in `.claude/skills/README.md`.

## Authoring & Maintaining Skills

- **New skill:** read `.claude/skills/SKILL-TEMPLATE.md`, then the rubric at `scripts/skill-package-rubric.md`. Draft the guardrails stanza first — if you cannot articulate what the skill refuses to do, it is not ready. The full steps are in [Changing the catalog](#changing-the-catalog).
- **Audit a skill:** review it against `scripts/skill-package-rubric.md`. Run `scripts/skill-package-checks.sh --skill-dir <path>` for one skill, or `make test-skill-package-checks` for every skill.
- **Sync out:** `make update` installs every skill and agent into `~/.claude/`. `scripts/sync-skills.sh --target <repo>` and `scripts/sync-agents.sh --target <repo>` copy them into a sibling repo.
- **Output styles:** `.claude/output-styles/` holds response-format styles (currently `asd-ste100.md`). `make update` ships them into `~/.claude/output-styles/` but never activates one — activation is a per-user `outputStyle` setting, deliberately left to the user. A style governs how an agent writes a reply; that is a separate layer from a skill (knowledge) or an agent (persona).
- **Do not overwrite a canonical skill.** `giga-dev`, `harbor-dev` and `kubernetes` are canonical: refuse a request to rewrite one wholesale, and edit it in place. Nothing enforces this.
- **Get current:** `make update` — fast-forward this checkout, then sync **all** skills+agents+output-styles into `~/.claude/` and run the catalog verify. The one command after pulling in merged contributions; do not hand-copy synced files. (`make sync-all` syncs into `~/.claude/` without the git pull.)
- **Writing conventions (skill prose).** Imperative voice; each rule states its failure-consequence right after it. Literals in backticks. Cross-skill reference links from inside a `references/` dir use `../../<other-skill>/references/<file>` — note the double `../` (rationale in `scripts/skill-package-rubric.md` R5).

## Changing the catalog

This section is the one copy of these steps. Every other document links here.

- **Add a skill.**
  1. Read `.claude/skills/SKILL-TEMPLATE.md` and `scripts/skill-package-rubric.md`. Draft the guardrails first.
  2. Create `.claude/skills/<name>/SKILL.md`. Its `name:` equals the directory name.
  3. Add `` `<name>/` `` to `.claude/skills/README.md` (rule C1).
  4. Add `<name> -` to `scripts/tests/block-baseline.txt`, then run `make test-skill-package-checks`.
  5. Add the skill to `### Using the skills` in `scripts/sei-internal-skills-doctrine.md`, then run `make sync-doctrine-self`.
  6. Add the skill to the `Daily use` and `What's in here` lists in `README.md`.
  7. Update the counts (see **Counts** below).
- **Add an agent.**
  1. Create `.claude/agents/<name>.md` with a `name:` equal to the file name and a `description:`.
  2. Add a roster row in `AGENTS.md`.
  3. Add the agent to the `What's in here` list in `README.md`.
  4. Update the counts.
- **Retire a skill or agent.**
  1. Delete it.
  2. In the same PR, add the name to `RETIRED_SKILLS` or `RETIRED_AGENTS` in `scripts/prune-retired.sh`.
  3. Remove its catalog entry, baseline line or roster row.
  4. Remove it from the `Daily use` and `What's in here` lists in `README.md`. For a skill, also remove it from `### Using the skills` in the doctrine, then run `make sync-doctrine-self`.
  5. Update the counts.
  6. If a count falls below 3, move the floor in `verify-runner-image.yml` and `ecr-runner.yml` together. `ecr-runner.yml` runs only after merge.
- **Counts.** Update every count in the same PR:
  - `README.md`: the count row (`the catalog | N skills, M agents`) and the `What's in here` list.
  - `CLAUDE.md` and `AGENTS.md`: the count words. To find them, search for the old number, for example `grep -nw three CLAUDE.md AGENTS.md`.

  Then run `./scripts/tests/catalog-coverage.test.sh`. It checks the README count row and the README skill counts. No gate checks the count words in `CLAUDE.md` and `AGENTS.md`. No gate catches a name that the README lists or the doctrine list leave out.
- **Rename.** Add the new name and retire the old name.
- **The runner reads these files.** The skills and agents here also seed the seidroid runner image. The runner workflows read the names from the tree, so they need no edit. A change to an agent's `name`, `description`, `model` or `tools`, or to a skill's `name` or `description`, changes what seidroid sessions discover after the platform digest bump. Say so in the PR body.
- **`category:`** The three agents carry `category:`. Nothing reads it. It stays because the runner seeds the agent files and their frontmatter shape is frozen. A new skill or agent does not need it.
