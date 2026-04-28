# Tide Project Skills

Project-scoped skills for team processes. Each subdirectory is a self-contained skill (SKILL.md + scripts + references + evals).

## First time here?

1. **Inside Tide, no setup needed.** Claude Code auto-discovers everything in this directory.
2. **To use these skills outside Tide** (e.g. `/coral` or `/issue` from another repo), run the sync once:
   ```sh
   ./scripts/sync-skills.sh
   ```
   This copies `coral`, `council`, `design`, and `issue` into `~/.claude/skills/` so they're available everywhere.
3. **Re-run after `git pull`** if these skills changed upstream. It's idempotent — safe to run any time.

**Edit skills in Tide, never in `~/.claude/skills/`.** Local edits at user-scope get overwritten on next sync. To change a skill, edit it here and PR.

---

**Authoring standard:** read [`SKILL-TEMPLATE.md`](./SKILL-TEMPLATE.md) before creating a new skill.

Claude Code discovers skills as direct subdirectories — nested folders are NOT discovered. Logical grouping happens in this catalog, not in directory structure.

## Catalog

### Workflow

Edit these in Tide, never in `~/.claude/skills/` — your edits will be overwritten on next sync. To use them outside Tide, run:

```sh
./scripts/sync-skills.sh
```

- **`coral/`** — Lightweight expert iteration. Knows about the `/issue` handoff (offers to bootstrap deferred slices and end-of-session phase 2 as a tracked issue).
- **`council/`** — Full-ceremony multi-component design, cross-review, scope-tier selection. The heavier sibling of coral; teammates will mostly use coral, but council ships alongside so the coral → council handoff works from anywhere.

### Workstream Bootstrap
Two complementary artifact-capture skills. Coral / council should offer them at handoff moments — `/issue` for **next** work, `/design` for **this** work's design pass. Both pre-fill from session context; user reviews and confirms.

- **`issue/`** — Synthesize the current session into a standard-format GitHub issue that bootstraps the next pickup. Required body sections: Problem, Impact, Relevant experts. Fires when a deferred slice surfaces, the user cuts scope, or the session closes with an obvious phase 2.
- **`design/`** — Capture the current session's design as a markdown doc under `docs/designs/` (or repo-specific path; Tide → `design/milestones/` or `design/high-level/`). ADR-flavored body with mermaid diagrams encouraged. Threads bidirectional lineage to the source issue (frontmatter `Issue: #n` forward; offers to update issue's References reverse). Fires when the deliverable IS a design (LLD, architecture sketch, system-tier decision).

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

A project-scope skill in this repo is only discoverable when Claude Code is running with this repo as CWD. To make a skill discoverable elsewhere, sync it out:

```sh
./scripts/sync-skills.sh                    # daily: portable skills → ~/.claude/skills/
./scripts/sync-skills.sh --categories all   # also sync sei skills (chaos-suite, sei-platform-engineer)
./scripts/sync-skills.sh --target ~/work/sei-k8s-controller --force  # to another repo
```

If a tracked file in the target differs from Tide's version, the skill is reported as a conflict and skipped — re-run with `--force` to overwrite. Target-only files (user customizations, runtime artifacts) are preserved.

Sibling of `scripts/sync-agents.sh` — same shape, same flags. Categories: `portable` (`coral`, `council`, `design`, `issue`), `sei` (`chaos-suite`, `sei-platform-engineer`), `all`. Update the lists in the script when a skill is added, renamed, or re-categorized.

For procedural skills like `chaos-suite` that operate on remote infrastructure, you can also just run them from Tide and pass `--repo` / target paths to direct work elsewhere — no sync needed.
