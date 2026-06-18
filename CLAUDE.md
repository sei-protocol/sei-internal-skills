# Tide Engineering Workspace

## Project Context

Tide is Sei's centralized library of **portable Claude Code skills and specialist agents** for engineering work. Skills and agents are authored and version-controlled here, then synced out to user-scope (`~/.claude/`) and sibling repos via `scripts/sync-skills.sh` and `scripts/sync-agents.sh`. This repo is the canonical home — edit here and PR; never edit the synced copies.

The skills help engineers research, groom work, document progress in git and tickets, author and iterate on designs and 1-pagers, automate operational processes (releases, root-cause analysis), and collaborate with specialist agents.

## Operating Doctrine

> Operating doctrine for the Tide-synced skills and agents lives in [AGENTS.md](./AGENTS.md).

The engineering principles, the output discipline (incl. the comment/documentation standard and its champions), the workflow skills and when each applies (`/coral`, `/xreview`, `/council`, `/bugbash`, `/root-cause`, plus the `/design` + `/issue` handoffs), the xreview discipline, and the key rules are maintained **once** in `scripts/tide-doctrine.md` and distributed — to this repo and to every consuming package — as the `tide-managed` block in [AGENTS.md](./AGENTS.md). Edit the doctrine there, then run `make sync-doctrine-self`. The specialist roster also lives in `AGENTS.md`; the full skill catalog lives in `.claude/skills/README.md`.

## Authoring & Maintaining Skills

- **New skill:** read `.claude/skills/SKILL-TEMPLATE.md`, then use `/author-skill`. Draft the guardrails stanza first — if you can't articulate what the skill refuses to do, it isn't ready.
- **Audit a skill:** use `/audit-skill <name>` (audit-only by default; `--apply` to refactor). Reports land under `docs/skill-audits/`.
- **Sync out:** `make sync-skills` / `make sync-agents` push portable updates to user-scope; pass `--target <repo>` for sibling repos.
- **Writing conventions (skill prose).** Imperative voice; each rule states its failure-consequence right after it. Literals in backticks (the full render/re-match rules live in `.claude/skills/impact-weekly/references/notion-flavored-markdown.md`). Cross-skill reference links from inside a `references/` dir use `../../<other-skill>/references/<file>` — note the double `../` (rationale in `.claude/skills/audit-skill/references/conventions-catalog.md` R5). Author to `/lingua`.
