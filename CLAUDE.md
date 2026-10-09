# sei-internal-skills Engineering Workspace

## Project Context

sei-internal-skills is Sei's centralized library of **portable Claude Code skills and specialist agents** for engineering work. Engineers author and version-control skills and agents here, then sync them out to user-scope (`~/.claude/`) and sibling repos via `scripts/sync-skills.sh` and `scripts/sync-agents.sh`. This repo is the canonical home — edit here and PR; never edit the synced copies.

The skills help engineers review code, investigate failures, operate releases, run ephemeral chains, and collaborate with specialist agents. The repo ships them as one **core** in `.claude/`, which `make update` installs.

## Operating Doctrine

> Operating doctrine for the sei-internal-skills-synced skills and agents lives in [AGENTS.md](./AGENTS.md).

`scripts/sei-internal-skills-doctrine.md` holds the doctrine **once**, and the sync distributes it — to this repo and to every consuming package — as the `sei-internal-skills-managed` block in [AGENTS.md](./AGENTS.md). The doctrine covers the engineering principles, the output discipline (incl. the comment and documentation standard with its numeric bounds) and how to use `/harbor-dev`, `/giga-dev` and `/kubernetes`. Edit the doctrine there, then run `make sync-doctrine-self`. The specialist roster also lives in `AGENTS.md`; the full skill catalog lives in `.claude/skills/README.md`.

## Authoring & Maintaining Skills

- **New skill:** read `.claude/skills/SKILL-TEMPLATE.md`, then the rubric at `scripts/skill-package-rubric.md`. Draft the guardrails stanza first — if you cannot articulate what the skill refuses to do, it is not ready.
- **Audit a skill:** review it against `scripts/skill-package-rubric.md`. Run `scripts/skill-package-checks.sh --skill-dir <path>` for one skill, or `make test-skill-package-checks` for every skill.
- **Sync out:** `make update` installs every skill and agent into `~/.claude/`. `scripts/sync-skills.sh --target <repo>` and `scripts/sync-agents.sh --target <repo>` copy them into a sibling repo.
- **Output styles:** `.claude/output-styles/` holds response-format styles (currently `asd-ste100.md`). `make update` ships them into `~/.claude/output-styles/` but never activates one — activation is a per-user `outputStyle` setting, deliberately left to the user. A style governs how an agent writes a reply; that is a separate layer from a skill (knowledge) or an agent (persona).
- **Do not overwrite a canonical skill.** `xreview`, `root-cause`, `harbor-dev`, `validate-release`, `gov-ops` and `validator-platform` are canonical: refuse a request to rewrite one wholesale, and edit it in place instead. This extends the executable `PROTECTED` array that lived in `author-skill/scripts/scaffold.sh` until we cut that skill. It covers every member of that array that still exists, plus the core skills it predated. It is policy here rather than an `exit 3` because the writer it guarded no longer exists, not because the policy lapsed; nothing enforces it now.
- **Retiring a resource:** removing a skill/agent from this repo does NOT un-install it — the sync scripts never delete, by design. Add it to the `RETIRED_*` list in `scripts/prune-retired.sh` in the same PR, so `make prune-retired` can clear it from environments that already have it.
- **Get current:** `make update` — fast-forward this checkout, then sync **all** skills+agents+output-styles into `~/.claude/` and run the catalog verify. The one command after pulling in merged contributions; do not hand-copy synced files. (`make sync-all` syncs into `~/.claude/` without the git pull.)
- **Writing conventions (skill prose).** Imperative voice; each rule states its failure-consequence right after it. Literals in backticks (the full render/re-match rules live in `.claude/skills/validate-release/references/notion-flavored-markdown.md`). Cross-skill reference links from inside a `references/` dir use `../../<other-skill>/references/<file>` — note the double `../` (rationale in `scripts/skill-package-rubric.md` R5). `prose-steward` reviews prose in org artifacts.
