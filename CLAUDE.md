# sei-internal-skills Engineering Workspace

## Project Context

sei-internal-skills is Sei's centralized library of **portable Claude Code skills and specialist agents** for engineering work. Skills and agents are authored and version-controlled here, then synced out to user-scope (`~/.claude/`) and sibling repos via `scripts/sync-skills.sh` and `scripts/sync-agents.sh`. This repo is the canonical home — edit here and PR; never edit the synced copies.

The skills help engineers review code, investigate failures, operate releases, run ephemeral chains, and collaborate with specialist agents. The repo ships a focused **core**; broader and still-forming work is parked in [`experimental/`](./experimental/README.md) and installs only on opt-in.

## Operating Doctrine

> Operating doctrine for the sei-internal-skills-synced skills and agents lives in [AGENTS.md](./AGENTS.md).

The engineering principles, the output discipline (incl. the comment/documentation standard and its champions), the workflow skills and when each applies (`/xreview`, `/root-cause`, `/idiomatic`, `/systems`, `/pr-quality`), the xreview discipline, and the key rules are maintained **once** in `scripts/sei-internal-skills-doctrine.md` and distributed — to this repo and to every consuming package — as the `sei-internal-skills-managed` block in [AGENTS.md](./AGENTS.md). Edit the doctrine there, then run `make sync-doctrine-self`. The specialist roster also lives in `AGENTS.md`; the full skill catalog lives in `.claude/skills/README.md`.

## Authoring & Maintaining Skills

- **New skill:** read `.claude/skills/SKILL-TEMPLATE.md`, then use `/author-skill`. Draft the guardrails stanza first — if you can't articulate what the skill refuses to do, it isn't ready.
- **Audit a skill:** use `/audit-skill <name>` (audit-only by default; `--apply` to refactor). Reports land in the DRI's `<engineer>-designs` repo under `designs/<arc>/audits/` (Design 13; in-repo `docs/skill-audits/` only when no DRI repo).
- **Sync out:** `make sync-skills` / `make sync-agents` push portable updates to user-scope; pass `--target <repo>` for sibling repos.
- **Output styles:** `.claude/output-styles/` holds response-format styles (currently `asd-ste100.md`). `make update` ships them into `~/.claude/output-styles/` but never activates one — activation is a per-user `outputStyle` setting, deliberately left to the user. A style governs how replies are written; that is a separate layer from a skill (knowledge) or an agent (persona).
- **Core vs experimental:** `.claude/skills/` and `.claude/agents/` are the **shipped core** — what `make update` installs everywhere. `experimental/` is the parking lot: still forming, narrow audience, or exploratory. The exclusion is structural (the sync scripts read `.claude/` only), so parking or promoting a resource is just a `git mv` — see [`experimental/README.md`](./experimental/README.md). Promote only when an engineering team outside the author would reach for it on ordinary work.
- **Retiring a resource:** removing a skill/agent from this repo does NOT un-install it — the sync scripts never delete, by design. Add it to the `RETIRED_*` list in `scripts/prune-retired.sh` in the same PR, so `make prune-retired` can clear it from environments that already have it. Parked resources need no list entry; that side is derived from `experimental/`.
- **Get current:** `make update` — fast-forward this checkout, then sync **all** skills+agents+output-styles into `~/.claude/` and run the catalog verify. The one command after pulling in merged contributions; don't hand-copy synced files. (`make sync-all` syncs into `~/.claude/` without the git pull.)
- **Writing conventions (skill prose).** Imperative voice; each rule states its failure-consequence right after it. Literals in backticks (the full render/re-match rules live in `experimental/skills/impact-weekly/references/notion-flavored-markdown.md`). Cross-skill reference links from inside a `references/` dir use `../../<other-skill>/references/<file>` — note the double `../` (rationale in `.claude/skills/audit-skill/references/conventions-catalog.md` R5). Author to `/language`.
