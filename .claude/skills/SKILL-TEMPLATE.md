# Skill template

This page is the authoring standard for a skill in this repository. Read it before you write a skill. Use its checklist when you review one. The rules that it cites by ID live in `scripts/skill-package-rubric.md`.

A skill here lives in `.claude/skills/<name>/`. `make update` copies it into `~/.claude/skills/`, so the skill must not assume a working directory. A skill that only one repository needs belongs in the `.claude/skills/` of that repository.

## Three shapes

A **procedural skill** runs a fixed sequence of steps with side effects on external systems: clusters, CI, deployments or on-chain state. Most of this page describes this shape.

A **reference skill** has no side effects. `/kubernetes` is the example. It holds a citable corpus (`references/sources.md`), an always-first profile of the target codebase (`references/sei-controller-profile.md`), a method (`references/method.md`) and pluggable kits (`references/kit-*.md`). It has no `scripts/` directory. It still has guardrails, halt conditions, `evals/evals.json` and a catalog entry. The script sections below do not apply to it.

A **conversational skill** drives a CLI through a conversation. `/harbor-dev` and `/giga-dev` are the examples. It keeps its commands in `SKILL.md` and `references/`, and its guardrails gate each side effect. It has no `scripts/` or `state/` directory. The script and state sections below do not apply to it.

## Directory shape

```
.claude/skills/<skill-name>/
  SKILL.md                   # trigger description + procedure
  scripts/                   # deterministic steps (shell, python)
    <step-1>.sh
    <step-2>.sh
    README.md                # what each script does, and its exit codes
  references/
    guardrails.md            # expanded safety model
    summary-template.md      # output format (if the skill produces an artifact)
    <other-reference>.md     # whatever the procedure needs
  evals/
    evals.json               # at minimum: one happy-path, one halt-condition
  state/                     # per-run working state (gitignored)
    .gitkeep
```

Claude Code finds a skill only as a direct subdirectory of `.claude/skills/`. It does not find a nested folder. Group skills in the catalog, `.claude/skills/README.md`, not in the directory tree. Put each reference file directly under `references/` (rule R1).

## SKILL.md anatomy

A procedural `SKILL.md` has these sections, in this order.

### 1. Frontmatter

```yaml
---
name: <skill-name>
model: claude-opus-5   # optional: pins the model that runs the skill
description: "Use when <situation>. Triggers: '<phrase 1>', '<phrase 2>', '<phrase 3>'. Anti-triggers: NOT for <case> (use <other skill>)."
---
```

`model:` is optional. Set it only to pin the model that runs the skill. The seidroid runner seeds the skill files, so a change to `model:` is a runner change (see [Changing the catalog](../../CLAUDE.md#changing-the-catalog)).

The description is the only text that routes a request to the skill. Write it with the most care.

- **Triggers.** Use the exact phrases that a user says, not synonyms or paraphrases. The runtime matches on the text. Give at least three (rule D8).
- **Anti-triggers.** Name each case where the skill must not run: "NOT for production clusters", "SKIP if X". They stop an over-match (rule D3).
<!-- gap: /example-skill — a placeholder in this template, not a citation. -->
- **Redirects.** Send an adjacent request to the right skill: "for single-test runs, use /example-skill" (rule D4).
- **Form.** Start with "Use when". Write in the third person (rules D1 and D5). Keep the description under 1024 characters (rule D2).

### 2. Guardrails stanza

The guardrails are the first section after the title. They hold the rules that the skill never breaks. Use this format:

```markdown
## Guardrails

This skill operates on **<scope>** only. Before any side-effecting action:

1. **Context check** — verify <how the skill confirms it's safe>
2. **Scope confirmation** — echo <target> back to user; require 'confirm' on first side-effecting call
3. **Refusal conditions** — this skill will refuse to run if:
   - <condition 1>
   - <condition 2>

See `references/guardrails.md` for the detailed safety model.
```

Name at least three refusal conditions (rule B8). If you cannot write the guardrails, the skill is not safe to write. Write them first.

### 3. Preconditions

State what must be true before the skill runs: the tools, the environment variables, the auth state and the files that must exist.

### 4. Procedure

The procedure is the main body. Number its steps. In each step:

- Call a script from `scripts/`. Do not put a shell command in the prose.
- Name the action and the signal that the step captures.
- State the success criterion of the step.
- Name the halt conditions that can fire.

Keep each step to a few sentences. Put a long explanation in `references/`.

### 5. Halt conditions

List each condition where the skill stops and asks for help, instead of a recovery:

```markdown
## Halt Conditions

Stop and report to the user if:
- <condition 1> — report what was captured, what state is dirty
- <condition 2> — ...
```

**Never fix a problem silently.** When the skill finds a problem, it reports the problem, and the user chooses the fix.

### 6. State management

Each run writes its state to `state/run-<ISO-timestamp>/`, and every script writes to that directory. When a run stops before the end, the next run finds the incomplete state. It then offers three choices: resume, archive or start again. A script writes the summary artifact outside `state/`, and only after the full procedure succeeds.

### 7. Summary

Say where and how the skill writes its output artifact. If the skill writes a document, point to `references/summary-template.md`. Give the format of the last chat message separately: it is the summary that the user sees at the end of the session.

## Script layout

- Write each script in shell (`.sh`) or Python (`.py`), not as prose in `SKILL.md`.
- Give each logical step its own script. Do not write one large script.
- Use flag arguments (rule S6). Each script must run on its own, outside the skill: `./scripts/<step>.sh --target <id> --dry-run`.
- Start each shell script with a shebang and `set -euo pipefail` (rules S1 and S2).
- Give each side-effecting script a `--dry-run` flag (rule S3).
- Exit non-zero on a failure. The procedure checks each exit code.
- Send YAML state to `state/run-<ts>/`, human-readable output to stdout, and the audit log to `state/run-<ts>/audit.log`.
- Document each script and its exit codes in `scripts/README.md` (rule S4).

## Guardrails reference (`references/guardrails.md`)

Expand the guardrails stanza in full:

- The scope: the environments, clusters and namespaces.
- The pre-flight checks: context patterns, environment variables, auth state and a health check.
- The scope confirmation: the exact prompt text that the skill shows.
- Each destructive action that needs an extra confirmation, also on the happy path.
- How an interrupted run avoids a partial state.
- What the skill never does, with or without a pre-approval.

## Summary template (`references/summary-template.md`)

If the skill writes an artifact, this template sets its format. Keep the format the same across runs, so that a diff between two runs shows only real changes. The collation script fills in the placeholders. The template itself stays static.

## State files

- `state/run-<ISO-timestamp>/<step-or-subject>.yaml`: the state of one step.
- `state/run-<ts>/audit.log`: a timestamped log of each command, its exit code and its output.
- `.gitignore` covers `state/` (rule T1). Commit only the final artifact, at its user-facing path.
- Keep `state/.gitkeep`, so the directory exists when it is empty (rule T2).
- Make each run resumable. The next run reads `state/`, finds the latest incomplete run and offers to resume it.

## Permission pre-approval

Pre-approve the Bash patterns of the happy path, so the skill runs without a permission prompt on its normal path. Document these in `SKILL.md` or in a README:

- The Bash command patterns that the allow list admits. For example, `kubectl get pods -n <ns>`, but not `kubectl delete *`.
- The tools that stay interactive: each destructive tool, and each tool outside the declared scope.
- The environment variables that the skill reads.

`make update` installs a skill in user scope, so it runs in every repository. Each engineer puts its patterns in `~/.claude/settings.json`, or in the `.claude/settings.local.json` of the project where they run it. Never put them in the tracked `.claude/settings.json` of this repository. That file holds only the read-only set from `scripts/agent-permissions.json`. `make verify-agent-permissions` fails on a mutating pattern, and on a difference from that set.

The `fewer-permission-prompts` skill proposes an allow list from a real session. It writes that list into the project `.claude/settings.json`. In this repository, move the patterns that it adds to one of the files above. Then restore `.claude/settings.json` with `git checkout -- .claude/settings.json`.

## Evals

`evals/evals.json` holds at least two evals (rule E2):

1. **Happy path**: a scripted run that checks that the skill runs the full procedure and writes the expected artifact.
2. **Halt condition**: a scripted run that fires a halt condition and checks that the skill stops and reports.

Aim for three or more (rule E3). Give each eval a `source` field that points to a scenario, a halt condition or a production incident (rule E4). Add evals as the skill grows.

## Authoring checklist

- [ ] The skill name is short kebab-case that works as a slash command.
- [ ] The description starts with "Use when" and has triggers, anti-triggers and redirects.
- [ ] You wrote the guardrails FIRST. If you cannot say what the skill refuses to do, stop.
- [ ] Procedural only: the procedure has discrete steps, and each step calls one script under `scripts/`.
- [ ] The halt conditions are explicit: stop and report, never fix silently.
- [ ] The summary template exists, if the skill writes an artifact.
- [ ] Procedural only: the state convention holds. Git ignores `state/`, and each run has its own directory and `audit.log`.
- [ ] A skill with side effects documents its happy-path permissions.
- [ ] `evals/evals.json` has at least one happy-path eval and one halt-condition eval.
- [ ] `.claude/skills/README.md` lists the skill (rule C1).
- [ ] Procedural only: `.gitignore` covers `state/` (`.claude/skills/*/state/`).
- [ ] `name:` matches the directory (`make verify-catalog`).
- [ ] `scripts/skill-package-checks.sh --skill-dir <path>` reports no `block` failure.
- [ ] The rest of the steps: [Changing the catalog](../../CLAUDE.md#changing-the-catalog) in `CLAUDE.md`.

## Anti-patterns

These signs show that a procedural skill goes wrong:

- **Shell commands in `SKILL.md` prose.** `SKILL.md` is documentation, not a script. Put commands in `scripts/`.
- **Vague trigger phrases.** "Use this when working on the platform" is too broad, so it over-matches. Be specific.
- **No anti-triggers.** If the skill must not run in some scope, the description says so.
- **Silent auto-remediation.** The skill finds a problem and fixes it without a report. This is almost always wrong. Halt and ask.
- **Secrets, credentials or tokens in the skill.** Put them in environment variables or configuration, not in the skill files. This repository is public, so a committed secret is a leaked secret.
- **No state directory.** When a run fails, you have nothing to debug. Always write state.
- **A permission prompt on each Bash call.** The skill becomes unusable. Pre-approve the happy path.
