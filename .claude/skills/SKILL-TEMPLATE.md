# Skill Template — Procedural Team Skills

This document defines the canonical shape for skills that codify a team process. Read it before authoring a new procedural skill; use it as a checklist in review.

A **procedural skill** executes a fixed sequence of steps with side effects on external systems (clusters, CI, deployments, on-chain state, etc.). It is different from an orchestration skill like `/council` or `/coral`, which coordinate agents and don't typically have side effects themselves.

Procedural skills live at **project scope** (`<repo>/.claude/skills/<name>/`) unless they're truly repo-agnostic, in which case they live at user scope (`~/.claude/skills/<name>/`). The default is project scope.

## Canonical Directory Shape

```
<repo>/.claude/skills/<skill-name>/
  SKILL.md                   # trigger description + procedure
  scripts/                   # deterministic steps (shell, python)
    <step-1>.sh
    <step-2>.sh
    README.md                # what each script does, how to author/edit them
  references/
    guardrails.md            # expanded safety model
    summary-template.md      # output format (if the skill produces an artifact)
    <other-reference>.md     # whatever the procedure needs
  evals/
    evals.json               # at minimum: one happy-path, one halt-condition
  state/                     # per-run working state (gitignored)
    .gitkeep
```

Claude Code discovers skills as direct subdirectories of `.claude/skills/`. Nested folders are NOT discovered. Logical grouping across skills happens in `.claude/skills/README.md` (the catalog), not in directory structure.

## SKILL.md Anatomy

Every procedural SKILL.md has these sections, in order:

### 1. Frontmatter

```yaml
---
name: <skill-name>
description: "<one sentence purpose. Concrete trigger phrases. Anti-triggers — explicit conditions NOT to invoke on. Sibling-skill redirects if user intent is adjacent.>"
---
```

Description crafting is the highest-leverage work in a skill — it's the only thing that routes invocation:

- **Triggers** — exact phrases a user would say. Not synonyms, not intent-level paraphrases. The runtime matches on the text.
- **Anti-triggers** — "NOT for production clusters", "SKIP if X", "do NOT use when Y". Prevent over-matching.
- **Redirects** — "for single-test runs, use /chaos-single". Pushes adjacent intents to the right tool.

### 2. Guardrails Stanza (first section after the title)

Non-negotiable safety rules. Must come before anything else in the body. Format:

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

If you can't write the guardrails stanza, the skill isn't safe to author. Write it first.

### 3. Preconditions

What must be true before the skill runs: tools available, env vars set, auth state, files that must exist.

### 4. Procedure (the main body)

The step-by-step procedure. Each step:
- Calls a script from `scripts/` — don't embed shell commands in prose
- Names what it's doing and which signal it's capturing
- Specifies success criteria for that step
- References the halt conditions that would fire

Use numbered lists. Keep each step to a few sentences. Long explanations belong in `references/`.

### 5. Halt Conditions

Explicit list — when the skill stops and asks for help instead of recovering:

```markdown
## Halt Conditions

Stop and report to the user if:
- <condition 1> — report what was captured, what state is dirty
- <condition 2> — ...
```

**Never auto-remediate without surfacing.** If the skill detects a problem, the user decides the remediation.

### 6. State Management

Per-run state lives in `state/run-<ISO-timestamp>/`. Every script writes to this directory. On interrupted runs, the next invocation detects incomplete state and offers resume / archive / start-fresh. Summary artifacts are written to a user-facing location (outside `state/`) only after the full procedure succeeds.

### 7. Summary

Where and how the skill produces its output artifact. Reference `references/summary-template.md` if the skill generates a document. Include the in-chat session-end summary format separately — what the user sees as the last message.

## Script Layout

- Shell (`.sh`) or Python (`.py`) — not prose embedded in SKILL.md.
- One logical step per script. No monolithic bash blobs.
- Flag-based args. Scripts should be debuggable standalone (`./scripts/apply-chaos.sh --test-id CH-TS-01` should work outside the skill).
- Exit non-zero on failure. The procedure checks exit codes.
- Structured output: YAML state updates go to `state/run-<ts>/`, human-readable output to stdout, audit log to `state/run-<ts>/audit.log`.

## Guardrails (`references/guardrails.md`)

Expand the SKILL.md guardrails stanza into full detail:
- Scope (which envs, clusters, namespaces)
- Pre-flight checks (context patterns, env vars, auth state, health check)
- Scope confirmation ritual (exact prompt text the skill echoes)
- Destructive actions requiring extra confirmation, even inside the happy path
- Anti-corruption patterns (how interrupted runs avoid leaving the world in a partial state)
- Unsafe patterns (things the skill NEVER does, pre-approved or not)

## Summary Template (`references/summary-template.md`)

If the skill produces an artifact, the template defines the format. Keep it consistent across runs so diffs are meaningful. The collation script fills in placeholders; the template itself is static.

## State File Convention

- `state/run-<ISO-timestamp>/<step-or-subject>.yaml` — per-step state
- `state/run-<ts>/audit.log` — timestamped log of every command, exit code, output
- `state/` is **gitignored**. Only the final artifact (at its user-facing path) gets committed.
- Runs must be resumable. Next invocation reads `state/` → detects latest incomplete run → offers resume.

## Permission Pre-Approval

Pre-approve the skill's happy-path Bash patterns in `.claude/settings.json` or `.claude/settings.local.json` so the skill runs without permission prompts on its normal path. Document in SKILL.md (or a README):

- Which Bash command patterns should be allowlisted (e.g., `kubectl get pods -n <ns>` but not `kubectl delete *`)
- Which tools to leave interactive (anything destructive, anything outside the skill's declared scope)
- Env vars the skill relies on

Use the `fewer-permission-prompts` skill to generate the allowlist from a transcript of a real run.

## Evals

Minimum bar: two evals in `evals/evals.json`:
1. **Happy path** — scripted run that verifies the skill executes the full procedure and produces the expected artifact.
2. **Halt condition** — scripted run that triggers a halt condition and verifies the skill stops and reports rather than proceeding.

Additional evals as the skill's surface area grows.

## Authoring Checklist

When creating a new procedural skill:

- [ ] Skill name is slash-command-friendly (kebab-case, short).
- [ ] Description field has triggers AND anti-triggers AND sibling redirects.
- [ ] Guardrails stanza drafted FIRST. If you can't articulate what the skill refuses to do, stop and think again.
- [ ] Procedure broken into discrete steps. Each step → one script under `scripts/`.
- [ ] Halt conditions written. The "stop, don't auto-remediate" list is explicit.
- [ ] Summary template drafted (if the skill produces an artifact).
- [ ] State convention followed — `state/` gitignored, run-ID subdir, audit.log.
- [ ] Happy-path permissions pre-approved in `settings.json` or documented.
- [ ] At least one happy-path eval and one halt-path eval.
- [ ] Entry added to `.claude/skills/README.md` catalog.
- [ ] `state/` is covered by `.gitignore` (usually via `.claude/skills/*/state/`).

## Anti-Patterns

Things that signal a procedural skill is going wrong:

- **Shell commands in SKILL.md prose** — the SKILL.md is documentation, not a script. Put commands in `scripts/`.
- **Vague trigger phrases** — "use this when working on the platform." Too broad; will over-match. Be specific.
- **Missing anti-triggers** — if there's ANY scope where invoking this skill would be wrong, the description must say so.
- **Auto-remediation** — the skill detects a problem and "just fixes it." Almost always wrong. Halt and ask.
- **Embedded secrets or cluster identifiers** — these belong in env vars or config, never checked into the skill.
- **No state directory** — the skill runs, something goes wrong, you have nothing to debug. Always write state.
- **Permission-prompt spam** — if every Bash call prompts, the skill is unusable. Pre-approve the happy path.
