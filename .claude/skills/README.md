# Tide Project Skills

Project-scoped skills for team processes. Each subdirectory is a self-contained skill (SKILL.md + scripts + references + evals).

**Authoring standard:** read [`SKILL-TEMPLATE.md`](./SKILL-TEMPLATE.md) before creating a new skill.

Claude Code discovers skills as direct subdirectories — nested folders are NOT discovered. Logical grouping happens in this catalog, not in directory structure.

## Catalog

### Workflow (user-level, not here)
- `/council` and `/coral` live at `~/.claude/skills/`. They orchestrate the specialist roster in `.claude/agents/` — council for full-ceremony multi-component design, coral for lightweight expert iteration. See the skills directly for details.

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

A skill in this repo can only be invoked when Claude Code is running with this repo as the working directory. If a procedural skill needs to operate against a different repo (e.g., `chaos-suite` operates on clusters referenced by the platform repo), either:

- Invoke it from this repo and have it write output into the other repo via absolute path, OR
- Duplicate the skill into the target repo.

User-level skills (`~/.claude/skills/`) are always discoverable regardless of CWD — use them for truly repo-agnostic skills, not for skills tied to a specific team process.
