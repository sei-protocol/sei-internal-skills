---
name: audit-skill
category: skill-authoring
model: claude-opus-4-8
description: "Use when auditing an existing skill against sei-internal-skills and Anthropic best-practice conventions — 'audit the X skill', 'review this skill against conventions', 'check if this skill meets standards', 'pressure-test the existing X skill', 'how does X measure up to conventions', '/audit-skill'. Anti-triggers: NOT for authoring a new skill (use /author-skill); NOT for in-conversation code review of arbitrary code (this audits skill-shaped artifacts under .claude/skills/); NOT for adversarial review of running systems (use /bugbash); NOT for Claude Code built-ins like /loop, /schedule, /init. For multi-component design work, use /council. For capturing a session's design, use /design."
---

# Audit Skill

Audits an existing skill against the team's known conventions and produces a findings report. Optionally — and only on explicit opt-in — proposes and applies tightening to bring the skill into compliance.

The brown-field sibling of `/author-skill`. Where `/author-skill` runs RED-GREEN-REFACTOR against a *blank* page, `/audit-skill` runs the same loop against a skill that *already exists* — with the explicit goal of measuring it against documented conventions rather than discovering new ones.

The centerpiece is `references/conventions-catalog.md` — every rule the audit checks, with ID and severity. Static checks fire from `scripts/static-checks.sh`; semantic checks fire as subagent dispatches; pressure tests reuse the methodology from `../author-skill/references/testing-with-subagents.md`.

## Guardrails

This skill operates on **`<repo>/.claude/skills/<name>/`** (or `~/.claude/skills/<name>/` with `--user`). Before any side-effecting action:

1. **Audit-only is the default.** No edits are made unless the user passes `--apply` (or confirms after viewing findings).
2. **Diff-before-write.** When `--apply` is on, every proposed change is shown as a unified diff and requires explicit confirmation. No silent edits.
3. **Append-only evals.** Refactor pass *appends* surviving pressure scenarios to `evals/evals.json` — it never overwrites existing entries.
4. **Protected-list policy.** Canonical workflow skills (coral, council, design, issue, bugbash, author-skill, audit-skill itself, chaos-suite, harbor-dev) are auditable freely. Refactor requires `--override-protected` plus a separate confirmation gate.
5. **Refusal conditions** — this skill refuses to run if:
   - The target skill directory does not exist or has no `SKILL.md`.
   - `--apply` was passed without a prior audit-only pass on the same skill in this session (no skipping straight to edits).
   - The shape can't be inferred from the SKILL.md (no frontmatter, no recognizable sections) — halt and ask the user to classify.
   - The conventions catalog is missing or unreadable.
   - The DRI `<engineer>-designs` repo where the report must land can't be resolved in a non-interactive run — **HALT and surface** rather than guessing a path or silently writing in-repo (Design 13 §4).

See `references/guardrails.md` for the detailed safety model.

## Two phases

### Phase 1 — Audit (default)

Reads the target skill, runs static + semantic + pressure checks against the conventions catalog, and produces a findings report in the DRI's `<engineer>-designs` repo at `designs/<arc>/audits/<skill>-<date>.md` (Design 13 — process-lineage relocation; resolve the DRI repo as `/design` does, sei-internal-skills's repo-default arc is `sei-internal-skills-stack`; in-repo `docs/skill-audits/` only when no DRI repo is resolvable and the user confirms). Default mode — no edits.

### Phase 2 — Refactor (opt-in via `--apply`)

Per-finding refactor proposals, shown as diffs, applied only on confirmation. Re-runs pressure scenarios after each apply (GREEN verify). REFACTOR cycle, capped at 3, if new rationalizations surface.

The two phases are *sequenced*, not coupled. You can run audit alone today and refactor tomorrow; the findings report is the durable handoff.

## Preconditions

- `Agent` tool available for semantic checks and pressure testing.
- `git` available; CWD is a git repo.
- `gh` CLI authenticated only if the refactor pass will open a PR.
- The target skill's directory exists and contains `SKILL.md`.
- For repo-scope audits: the conventions catalog at `<repo>/.claude/skills/audit-skill/references/conventions-catalog.md` is the source of truth. The audit reads it on every run, so updates to the catalog flow into the next audit automatically.

## Procedure

### Audit phase

1. **Resolve target.** From `--skill <name>` or `--path <abs-path>`. Resolve to `<repo>/.claude/skills/<name>/` or `~/.claude/skills/<name>/`. Confirm the path and skill name with the user before reading any files.

1a. **Resolve the DRI report home.** Resolve the DRI `<engineer>-designs` repo where the report will land, **as `/design` does** (`--designs-repo` → sibling `<engineer>-designs` checkout → ask). In a **non-interactive (headless/cron)** run, **HALT and surface — never write to a guessed path** (Design 13). The report lands at `designs/<arc>/audits/<name>-<date>.md` (sei-internal-skills repo-default arc `sei-internal-skills-stack`); the in-repo `docs/skill-audits/` is used **only when no DRI repo is resolvable and the user confirms**.

2. **Read & classify.** Load `SKILL.md`. Parse frontmatter (name, description). Infer shape — discipline / technique / pattern / reference / procedural — using the heuristics in `references/semantic-checks.md` (procedural has scripts/ and state/; discipline has a rationalization table or red-flags; reference is mostly TOC + entries). If shape can't be inferred, ask the user. Write to `state/run-<ts>/classify.yaml`.

3. **Static checks.** Run `scripts/static-checks.sh --skill-dir <path> --output state/run-<ts>/static-findings.jsonl`. The script runs the deterministic subset of the conventions catalog (description length, line count, refs one-level deep, scripts have set -euo pipefail, evals.json present and non-empty, state in .gitignore, etc.). Outputs JSONL — one finding per line. See `references/static-checks.md` for the full check list.

4. **Semantic checks.** Dispatch a `general-purpose` subagent via `Agent` with the prompt from `references/semantic-checks.md`. The subagent reads the SKILL.md + references + scripts and returns findings for the rules that need judgment (description is third-person, guardrails stanza is substantive vs. stubby, persuasion stack matches shape, prose doesn't smuggle in workflow summary). Findings written to `state/run-<ts>/semantic-findings.jsonl`.

5. **Pressure testing.** Reuse the methodology from `../author-skill/references/testing-with-subagents.md`. Instantiate 3 shape-appropriate scenarios from `../author-skill/references/pressure-scenario-templates.md`. Dispatch subagents WITH the target skill loaded; capture rationalizations the skill failed to prevent into `state/run-<ts>/pressure-findings.jsonl`. (Different from author-skill: there is no baseline-without-skill pass — we're auditing the *current* skill, so all scenarios run skill-loaded.)

6. **Synthesize findings.** Run `scripts/findings-report.sh --input state/run-<ts>/*-findings.jsonl --skill <name> --shape <inferred> --output <DRI-repo>/designs/<arc>/audits/<name>-<YYYY-MM-DD>.md` (an **absolute** path into the DRI checkout resolved in step 1a — never a path relative to the code repo, or a bare `designs/…` would create the directory inside the code repo and undo the Design 13 evacuation). Produces a markdown report grouped by severity (block / warn / info), with each finding linked back to its conventions-catalog ID, evidence, and recommended remediation.

7. **Show the report.** Display the findings count by severity, the top 3-5 blockers, and the path to the full report. **Stop here unless `--apply` was passed or the user explicitly opts into Phase 2.**

### Refactor phase (opt-in)

8. **Confirm refactor gate.** Echo:
   ```
   About to enter refactor phase for: <name>
     Findings:       <N block, M warn>
     Protected:      <yes|no>  (override required for protected skills)
     Will diff each proposed change before writing.
   Confirm? (yes / audit-only-stop / abort)
   ```
   Require literal "yes". For protected skills, also require `--override-protected` to have been passed at invocation.

9. **Per-finding refactor proposals.** For each `block` finding (and any `warn` the user opts into), generate a tightening proposal. The proposal is a unified diff against the current SKILL.md or reference file. Save each to `state/run-<ts>/proposals/<finding-id>.diff`.

10. **Diff review.** For each proposal, show the diff inline. User confirms / skips / edits. Don't batch — one diff, one decision.

11. **Apply confirmed diffs.** Run `scripts/apply-refactor.sh --diff <path> --target <file>` for each confirmed proposal. The script makes a backup at `state/run-<ts>/backups/<file>.before`, applies the diff with `patch`, verifies the result is parseable (parse frontmatter, count lines, lint shell scripts if applicable), rolls back on failure.

12. **GREEN — re-run pressure scenarios.** Re-dispatch the same scenarios from step 5. The skill should now resist the rationalizations it previously fell to. Save outcomes to `state/run-<ts>/green-verify.md`.

13. **REFACTOR — close new loopholes.** Capture any new rationalizations from GREEN and propose additional tightening. Cap at 3 cycles total (counting from step 9). If REFACTOR isn't converging by cycle 3, halt and surface — the skill has a structural problem deeper than the conventions check.

14. **Append to evals.json.** New pressure scenarios that survived REFACTOR become new evals — *appended* to the target skill's `evals/evals.json`, not overwriting existing entries.

15. **Final report.** Update the audit report with a Refactor section: what was applied, what was skipped, the GREEN verification result, the new evals added.

16. **End-of-turn summary.** One short paragraph: audit-only result, or audit + refactor result with finding counts and the path to the report.

## Halt Conditions

Stop and report to the user if:

- Target skill directory does not exist or has no `SKILL.md` — show the resolved path and ask the user to verify.
- Shape can't be inferred from SKILL.md — show what was parsed and ask the user to classify.
- Conventions catalog is missing or has no parseable rules — the audit can't run without it.
- `--apply` was passed without a prior audit-only pass on the same skill in this session — refuse and run audit-only first.
- `apply-refactor.sh` fails to verify the post-apply file (frontmatter unparseable, line count exceeds 500 after edit, shell script lint fails) — automatically roll back the change and report.
- REFACTOR doesn't converge after 3 cycles — surface the residual rationalizations and ask for guidance.
- User asks to refactor a protected canonical skill without `--override-protected` — refuse and prompt for the explicit override.
- The DRI `<engineer>-designs` repo can't be resolved for the report output in a non-interactive run — **HALT and surface** rather than guessing a path or silently writing in-repo (Design 13 §4; matches `references/guardrails.md`).

**Never auto-remediate without surfacing.** The user decides the remediation, and every edit passes a diff gate.

## State Management

Per-run state lives in `state/run-<ISO-timestamp>/`:

```
state/run-<ts>/
  classify.yaml              # shape, frontmatter, structure summary
  static-findings.jsonl      # one finding per line, from static-checks.sh
  semantic-findings.jsonl    # one finding per line, from subagent
  pressure-findings.jsonl    # one finding per line, captured from pressure scenarios
  proposals/                 # per-finding refactor diffs (Phase 2 only)
    D2.diff
    B1.diff
    ...
  backups/                   # pre-edit file backups (Phase 2 only)
    SKILL.md.before
    references/foo.md.before
  green-verify.md            # GREEN-phase pressure outcomes (Phase 2 only)
  refactor-N.md              # per-cycle REFACTOR notes (Phase 2 only)
  audit.log                  # timestamped log of every script call, subagent dispatch, confirm gate
```

`state/` is gitignored at the repo level. On interrupted runs, the next invocation detects the latest incomplete `run-<ts>/` and offers **resume** / **archive** / **start-fresh**.

The audit *report* (the durable lineage artifact) lives in the DRI's `<engineer>-designs` repo at `designs/<arc>/audits/<skill>-<YYYY-MM-DD>.md` (Design 13; sei-internal-skills's repo-default arc is `sei-internal-skills-stack`; in-repo `docs/skill-audits/` only when no DRI repo is resolvable and the user confirms) — outside `state/`, committable, the thing PRs reference.

## What this skill doesn't do

- **Author new skills.** That's `/author-skill`. Audit is brown-field; author is green-field.
- **Lint arbitrary code.** This audits skill-shaped artifacts in `.claude/skills/<name>/`. For repo-wide code review, use the relevant code-review skill.
- **Fix conventions for you.** Audit reports findings; the refactor pass is opt-in and diff-gated. You drive the remediation.
- **Update the conventions catalog.** When a new rule should be added, edit `references/conventions-catalog.md` directly (in a PR). The audit reads the catalog every run; catalog edits flow into the next audit automatically.
- **Bulk-audit a directory of skills.** One target per invocation. For a sweep, run repeatedly — each run produces a separate report with the same shape so they're comparable.

## Output

End-of-turn summary: one short paragraph. Skill audited, finding counts by severity, top blocker, report path. If Phase 2 ran, also: cycles consumed, edits applied, edits skipped, GREEN result. Example:

> Audited `coral` (orchestration shape). 14 findings: 2 block, 8 warn, 4 info. Top blocker: description includes workflow summary that pre-empts the body's flowchart (Obra CSO trap). Report at `designs/sei-internal-skills-stack/audits/coral-2026-05-10.md`. Stopped at audit-only — refactor pass deferred per user.
