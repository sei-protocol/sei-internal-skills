# Tide Project Skills

Project-scoped skills for team processes. Each subdirectory is a self-contained skill (SKILL.md + scripts + references + evals).

**Authoring standard:** read [`SKILL-TEMPLATE.md`](./SKILL-TEMPLATE.md) before creating a new skill.

Claude Code discovers skills as direct subdirectories — nested folders are NOT discovered. Logical grouping happens in this catalog, not in directory structure.

## Catalog

### Workflow
Tide is the source-of-truth for these skills. `scripts/sync-skills.sh --target ~ --categories portable` pushes them out to user-scope (`~/.claude/skills/`) so they're discoverable everywhere, not only inside Tide. Run after pulling main, or whenever a teammate updates these skills upstream.

- **`coral/`** — Lightweight expert iteration. Knows about the `/issue` handoff (offers to bootstrap deferred slices and end-of-session phase 2 as a tracked issue).
- **`council/`** — Full-ceremony multi-component design, cross-review, scope-tier selection. The heavier sibling of coral; teammates will mostly use coral, but council is here for when work outgrows it.

### Workstream Bootstrap
- **`issue/`** — Synthesize the current `/coral` or `/council` session into a standard-format GitHub issue that bootstraps the next pickup. Required body sections: Problem, Impact, Relevant experts. Coral / council should offer this skill at handoff moments (deferred slice, scope cut, end-of-session phase 2). Standalone use is supported for ad-hoc filings. **Status: ready** — invokable via `/issue` once Tide is the CWD or the skill directory is on Claude Code's discovery path.

### Release Operations
- **`chaos-suite/`** — Execute the full chaos test suite (runbook: sei-protocol/platform#169) against a dev or staging Sei cluster and collate results into a release summary. **Status: scaffold** — follows the template; scripts are placeholders pending authoring against the live runbook. Tracking issue: sei-protocol/platform#170.

### Engineer Self-Service
- **`sei-platform-engineer/`** — Engineer-facing interface to Sei platform infrastructure on the harbor EKS cluster. Translates natural-language intent (run a benchmark, onboard me, diagnose seinode) into `seictl` invocations. **Status: design draft** — SKILL.md is the contract; depends on seictl's cluster-facing commands (`bench`, `onboard`, `status`, `seinode`, `controller`, `context`) which are not yet implemented. Tracking issue: TBD on sei-protocol/seictl.

### Future Slots
- _(planned)_ `release-verify/` — deploy-smoke + sanity checks after a release cut.
- _(planned)_ Add skills here as the team codifies more processes.

## Adding a New Skill

1. Read [`SKILL-TEMPLATE.md`](./SKILL-TEMPLATE.md).
2. Draft the guardrails stanza FIRST. If you can't articulate what the skill refuses to do, it isn't ready to author.
3. Scaffold the directory structure from the template.
4. Add an entry to the catalog above under the appropriate section.
5. Make sure `state/` is gitignored (the repo-level `.gitignore` already covers `.claude/skills/*/state/`).
6. Pre-approve the skill's happy-path permissions in `.claude/settings.json` or `.claude/settings.local.json`.

## Cross-Repo Skills

A project-scope skill in this repo is only discoverable when Claude Code is running with this repo as CWD. To make a skill discoverable elsewhere, sync it out: `scripts/sync-skills.sh --target <path>` copies skills from Tide into the target's `.claude/skills/` directory. Sibling of `sync-agents.sh`; same conflict-and-`--force` semantics. Tide is the source-of-truth — downstream copies are derivable.

For procedural skills like `chaos-suite` that operate on remote infrastructure, you can also just run them from Tide and pass `--repo` / target paths to direct work elsewhere — no sync needed.

The categories in `sync-skills.sh` (`portable`, `sei`, `tide-only`) decide where each skill goes when syncing. Update the lists in the script when a skill is added, renamed, or re-categorized.
