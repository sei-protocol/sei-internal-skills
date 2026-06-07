# Tide Project Skills

Project-scoped skills for team processes. Each subdirectory is a self-contained skill (SKILL.md + scripts + references + evals).

## First time here?

1. **Inside Tide, no setup needed.** Claude Code auto-discovers everything in this directory.
2. **To use these skills outside Tide** (e.g. `/coral` or `/issue` from another repo), run the sync once:
   ```sh
   ./scripts/sync-skills.sh
   ```
   This copies the portable skills (`bugbash`, `coral`, `council`, `cross-review`, `design`, `issue`, `author-skill`, `audit-skill`, `root-cause`, `prfaq`) into `~/.claude/skills/` so they're available everywhere.
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

- **`coral/`** — Lightweight expert iteration. Knows about the `/issue` handoff (offers to bootstrap deferred slices and end-of-session phase 2 as a tracked issue) and offers `/cross-review` at synthesis when specialist outputs touch a shared boundary.
- **`cross-review/`** — Standalone cross-review action between the orchestrator and the coral/council experts. Dispatches the relevant specialists to **independently** review a produced artifact (design, plan, diff, or set of expert outputs), then synthesizes a COMPATIBLE / MISMATCH / MISSING findings table. Enforces blinded review + an assigned dissenter + evidence-bearing findings to defeat rubber-stamping and consensus theater. The review counterpart to coral's "produce"; `/council` invokes it as its cross-review phase.
- **`council/`** — Full-ceremony multi-component design and scope-tier selection. The heavier sibling of coral; delegates its cross-review phase to `/cross-review`. Teammates will mostly use coral, but council ships alongside so the coral → council handoff works from anywhere.

### Workstream Bootstrap
Two complementary artifact-capture skills. Coral / council should offer them at handoff moments — `/issue` for **next** work, `/design` for **this** work's design pass. Both pre-fill from session context; user reviews and confirms.

- **`issue/`** — Synthesize the current session into a standard-format issue that bootstraps the next pickup, filed to **GitHub or Linear** (asked at the create step). Required body sections: Problem, Impact, Relevant experts. Fires when a deferred slice surfaces, the user cuts scope, or the session closes with an obvious phase 2.
- **`design/`** — Capture the current session's design as a markdown doc under `docs/designs/` (or repo-specific path; Tide → `design/milestones/` or `design/high-level/`). ADR-flavored body with mermaid diagrams encouraged. Threads bidirectional lineage to the source issue (frontmatter `Issue: #n` forward; offers to update issue's References reverse). Fires when the deliverable IS a design (LLD, architecture sketch, system-tier decision).
- **`prfaq/`** — Author or review a PRFAQ (Amazon working-backwards Press Release + FAQ) before greenlighting a product/feature/initiative. Forces customer-thesis discipline (named customer, named pain, named alternatives, falsification thresholds). Refuses theater: buzzword soup, customer-absent prose, FAQ-as-marketing, polished perfectionism over thinking. Three modes: Author / Review / Verdict. Companion to `/design` (capture this design) and `/issue` (capture next workstream).

### Skill Authoring & Auditing
- **`author-skill/`** — Author a new skill for a specific domain. Drives Intake → deep web research (parallel subagents) → subagent-based RED-GREEN-REFACTOR pressure testing → scaffolds the skill into `<repo>/.claude/skills/<name>/` per `SKILL-TEMPLATE.md`, with evals derived from the surviving pressure scenarios. Built on Anthropic's skill-authoring best practices and Obra's TDD-for-skills methodology (`references/obra-best-practices.md`, `references/testing-with-subagents.md`, `references/persuasion-principles.md`). Refuses to overwrite canonical workflow skills (coral, council, design, issue, bugbash) or skip the RED baseline.
- **`audit-skill/`** — Audit an existing skill against the team's conventions catalog (`references/conventions-catalog.md`). Two phases: **audit-only** is the default — runs static + semantic + pressure checks and produces a findings report at `docs/skill-audits/<skill>-<date>.md`, no edits made. **Refactor** is opt-in via `--apply` — per-finding diffs, diff-before-write gate, append-only evals, automatic rollback on verify failure. Canonical skills are auditable freely; refactor requires `--override-protected`. The brown-field sibling of `/author-skill`.

### Hardening
- **`bugbash/`** — Long-running, read-only adversarial review of an existing system by the council of experts. Loops discovery + challenger passes against a named target (`/bugbash SeiNode controller`) until the experts converge on a launch verdict. Output is a structured findings log at `docs/bugbash/<target>.md` with per-item Scenario / Impact / Issue / Fix sketch / Test coverage. Inspired by the [RALPHY loop](https://github.com/snarktank/ralph), reframed for hardening before launch. Distinct from `/security-review` (single-pass, security-only) and `/coral` (collaborative iteration, not adversarial).

### Investigation
- **`root-cause/`** — Disciplined, data-driven, multi-expert investigation of complex problems in the Sei platform stack (sei-k8s-controller, seictl, sei-sidecar, sei-chain, release-test/qa-testing, platform/K8s). Forces signals before hypotheses, ≥2 competing hypotheses before evidence, retrieved provenance (not paraphrased), and falsification before conclusion. Dispatches `.claude/agents/` specialists in **parallel + blinded + with assigned dissent** to prevent the consensus-theater / sycophancy failure mode documented in the multi-agent LLM literature. Output is a multi-cause ranked conclusion — never a single root cause. Distinct from `/bugbash` (pre-launch adversarial), `/coral` (collaborative iteration), and live incident command (mitigate first; this skill is for understanding). Problems outside the Sei platform stack are out of scope.

### Authoring Discipline (Tide-local — not synced)

These two are project-scoped disciplines applied during authoring inside Tide. They are intentionally **not** in any sync category (CLAUDE.md / AGENTS.md reference them as in-repo disciplines):

- **`brevity/`** — Tighten agent-produced PR descriptions and in-code comments before they ship. Self-determines floor; agents don't pre-skip.
- **`pr-quality/`** — Pre-PR review of the staged diff + planned body (verbosity via `/brevity` dispatch + convention rules). Suggestive only; never gates merge.

### Release Operations
- **`chaos-suite/`** — Execute the full chaos test suite (runbook: sei-protocol/platform#169) against a dev or staging Sei cluster and collate results into a release summary. **Status: scaffold** — follows the template; scripts are placeholders pending authoring against the live runbook. Tracking issue: sei-protocol/platform#170.
- **`validate-release/`** — Collect a completed chaos-suite run's results from S3 + Thanos/Grafana, derive per-scenario metrics and panel PNGs, and push a structured release-validation report to Notion. Companion to `/chaos-suite` (run) → `/validate-release` (report).

### Impact Hub (project management)
- **`impact-weekly/`** — Roll up an engineer's Linear week (+ linked PRs) into the matching Impact Hub bet as a substantiated, executive-summary Weekly-log entry, draft→confirm→write. The producer in the work loop in `docs/designs/impact-hub-pm-skill-suite.md`; failure modes (mis-tracking, bloat, unsubstantiated claims) are engineered as refusals.
- **`impact-portfolio/`** — The weekly cross-project executive report: one human-confirmed Notion page per week under the Impact Hub's Weekly Reports (exec summary + per-project sections with owner, Overall Confidence, ≤3 substantiated bullets). Reads the week's per-bet Weekly-log toggles (+ a Linear `impact:<slug>` activity scan); read-only on bets, writes only its own report page. The reader-facing synthesis tail; design at `docs/designs/impact-portfolio-weekly-report.md`. (`impact-eoq`, the per-engineer quarter rollup, is the remaining deferred phase-2 sibling.)

### Engineer Self-Service
- **`harbor-dev/`** — Engineer-facing interface to the harbor EKS cluster. Translates natural-language intent (spin up an ephemeral chain, attach an RPC fleet, run a bench, onboard me, tear it down) into `seictl nd` invocations and PR-based GitOps deliveries against `sei-protocol/harbor-engineering-workspace`. Built on `seictl` v0.0.43+.

### Future Slots
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
./scripts/sync-skills.sh --categories all   # also sync sei skills (chaos-suite, harbor-dev)
./scripts/sync-skills.sh --target ~/work/sei-k8s-controller --force  # to another repo
```

If a tracked file in the target differs from Tide's version, the skill is reported as a conflict and skipped — re-run with `--force` to overwrite. Target-only files (user customizations, runtime artifacts) are preserved.

Sibling of `scripts/sync-agents.sh` — same shape, same flags. Categories: `portable` (`bugbash`, `coral`, `council`, `cross-review`, `design`, `issue`, `author-skill`, `audit-skill`, `root-cause`, `prfaq`), `sei` (`chaos-suite`, `harbor-dev`, `validate-release`), `all`. Update the lists in the script when a skill is added, renamed, or re-categorized.

For procedural skills like `chaos-suite` that operate on remote infrastructure, you can also just run them from Tide and pass `--repo` / target paths to direct work elsewhere — no sync needed.
