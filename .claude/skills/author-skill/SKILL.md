---
name: author-skill
description: "Use when authoring a new skill for a specific domain — the user says 'create a skill for X', 'we need a skill that handles Y', 'scaffold a skill for Z', 'author a skill', '/author-skill'. Also fires when refining an existing skill before deployment ('pressure-test this skill', 'harden this skill before shipping'). Anti-triggers: NOT for editing the canonical workflow skills owned by Tide (coral, council, design, issue, bugbash); NOT for in-conversation TODOs (use TaskCreate); NOT for Claude Code built-ins like /loop or /schedule (the harness owns those). For multi-component design work, use /council. For capturing an emerged design, use /design. For filing a deferred slice as an issue, use /issue."
---

# Author Skill

Generates a new skill for a specific domain. Drives an opinionated loop: **Intake → Research → Draft → Test under pressure → Scaffold → Catalog**. The output is a skill that has been pressure-tested against subagents *before* it ships, not a Markdown file that hopes for the best.

The methodology is RED-GREEN-REFACTOR for documentation (see `references/testing-with-subagents.md`). The conventions are the Tide `SKILL-TEMPLATE.md` shape plus Anthropic's skill-authoring best practices (see `references/obra-best-practices.md`).

## Guardrails

This skill operates on **`<repo>/.claude/skills/<name>/`** by default. Before any side-effecting action:

1. **Context check** — confirm the target directory does not already exist and is not a canonical workflow skill (coral, council, design, issue, bugbash, author-skill itself). These are off-limits — redirect to direct editing in a PR.
2. **Scope confirmation** — echo the resolved skill name, target path, scope (project vs user), and shape (procedural / orchestration / discipline / technique / pattern / reference) back to the user. Require explicit 'confirm' before scaffolding.
3. **Refusal conditions** — this skill will refuse to run if:
   - The author cannot write a guardrails stanza for the proposed skill (no clear refusal conditions → not safe to author).
   - The RED phase produces zero rationalizations across all pressure scenarios — the skill may be unnecessary (the constraint might be enforceable mechanically, not via documentation).
   - The target sits at user-scope (`~/.claude/skills/`) and `--user` was not passed.
   - The target directory already exists and is non-empty (no silent overwrites — halt and ask).
   - The GREEN phase still fails after 3 REFACTOR cycles — the skill design has a deeper problem; halt and surface.

See `references/guardrails.md` for the detailed safety model.

## Three invocation modes

### 1. New skill (primary)

User asks for a skill in a domain they care about (e.g., "create a skill for OAuth flows", "we need a skill that handles terraform module reviews"). Run the full Intake → Research → Draft → Test → Scaffold procedure below.

### 2. Refine an existing skill

User points at a skill that's already on disk and asks to harden it ("pressure-test the `xyz` skill", "this skill keeps getting bypassed, fix it"). Skip Intake, read the existing SKILL.md, jump to RED — run pressure scenarios that target the *current* skill's weakest sections. The REFACTOR loop is where most of the work happens.

### 3. From an issue (`/author-skill --issue <n>`)

User has an issue (typically filed via `/issue`) that proposes a skill. Fetch the issue body, seed Intake from it (Problem → domain, Proposed approach → expertise needed), confirm with the user, then run the full loop. After landing the skill, offer to comment on the issue with the skill path.

## Preconditions

- `WebFetch` and `WebSearch` available (or equivalent MCP web tools) — research is non-negotiable.
- `Agent` tool available — the RED/GREEN/REFACTOR phases dispatch general-purpose subagents to run pressure scenarios.
- `git` available in CWD; CWD is a git repo (or the user passes `--repo <path>`).
- `gh` CLI authenticated only if invoking with `--issue <n>` or if the catalog entry will be PR'd.
- A markdown-friendly editor for review (file is shown to the user before write).

## Procedure

Each step names what it produces and where (`state/run-<ISO-timestamp>/...`).

1. **Intake.** Interactive. Capture: skill **domain** (one sentence: what the skill is *about*), **focus** (the specific slice within the domain the user cares about), **expertise needed** (which kinds of experts would have authored this if it were a person — e.g., "senior SRE who has done 50 postmortems"), **type** (discipline / technique / pattern / reference — see `references/skill-shapes.md`), **target scope** (project — default — or user via `--user`), and **trigger phrases** (concrete sentences the user would say to invoke the skill). Write to `state/run-<ts>/intake.yaml`.

2. **Decide shape.** Map the intake to a shape using `references/skill-shapes.md`. Show the user the chosen shape and the rationale. Discipline-enforcing skills get the heaviest persuasion treatment; reference skills get the lightest. Halt and ask if the type isn't obvious — don't guess.

3. **Research.** Use `references/research-recipe.md` as the prompt template. Dispatch 2–4 parallel `general-purpose` subagents via `Agent`, one per research stream (best practices, common failure modes, authoritative sources, terminology + idioms). Each subagent returns a concise summary; the orchestrator synthesizes to `state/run-<ts>/research-notes.md`. The user reviews and adds anything missing from their own context. **Do not draft the skill from training data alone** — research grounds the skill in current, verifiable material.

4. **Draft the guardrails stanza FIRST.** Before any other body content, write the refusal conditions for the proposed skill. If you can't articulate what the skill refuses to do, **halt** — the skill isn't ready to author. Save to `state/run-<ts>/draft/guardrails.md`.

5. **Draft the description.** Apply `references/description-crafting.md`. The description is the highest-leverage field — it routes invocation. "Use when..." style, concrete trigger phrases, anti-triggers, sibling-skill redirects. **Never summarize workflow in the description** (Obra's CSO trap — Claude will follow the description instead of reading the skill). Show to user; iterate to ≤1024 chars.

6. **Outline the body.** Propose the section order. User confirms or adjusts before any prose is written. Save to `state/run-<ts>/draft/outline.md`.

7. **RED — baseline pressure test.** From `references/pressure-scenario-templates.md`, instantiate 3 scenarios that combine multiple pressures (time + sunk cost + authority + exhaustion + social) and would produce visible failures in the absence of the skill. Dispatch a `general-purpose` subagent for each scenario **without** the skill loaded. Capture the subagent's rationalizations *verbatim* into `state/run-<ts>/red-baseline.md`. If no scenario produces a rationalization, **halt** — either the constraint is enforceable mechanically (no skill needed) or the scenarios are too weak (escalate pressure and re-run once).

8. **GREEN — draft the body addressing the baseline failures.** Write the SKILL.md body to address the specific rationalizations captured in RED. Use the persuasion principles from `references/persuasion-principles.md` based on the chosen shape (authority + commitment + social proof for discipline; light authority + unity for technique/pattern; clarity only for reference). Stay under 500 lines in SKILL.md — push detail to `references/*` one level deep. Re-dispatch the same pressure scenarios **with** the skill loaded. The subagents should now comply. Save outcomes to `state/run-<ts>/green-verify.md`.

9. **REFACTOR — close loopholes.** Capture any *new* rationalizations the GREEN-phase subagents produced. Add explicit counters to the SKILL.md (a rationalization table is the canonical shape — see `references/persuasion-principles.md`). Re-run the scenarios. Up to **3 cycles**; halt and surface if compliance isn't reached. Save each cycle's diffs to `state/run-<ts>/refactor-<n>.md`.

10. **Generate evals.** Minimum: one happy-path eval and one halt-condition eval. The pressure scenarios that survived REFACTOR convert directly to evals (the prompt + the expected compliance behavior). Write to `state/run-<ts>/draft/evals.json`. See `references/eval-format.md`.

11. **Scaffold the skill on disk.** Call `scripts/scaffold.sh --name <name> --scope <project|user> --shape <shape> --draft-dir <state/run-<ts>/draft>`. The script creates the directory tree under `<repo>/.claude/skills/<name>/` (or `~/.claude/skills/<name>/` with `--scope user`), populates SKILL.md / references/ / scripts/ / evals/ / state/.gitkeep from the draft, and refuses to overwrite an existing non-empty directory.

    **Preview first with `--dry-run`.** Before the real apply, run with `--dry-run` to print the manifest of directories and files that will be created. Show this to the user; on confirmation, re-run without the flag to apply. This matches the pattern in `scripts/add-catalog-entry.sh` and `scripts/sync-check.sh` and gives the user a final review gate before the scaffold lands.

12. **Add to the catalog.** Call `scripts/add-catalog-entry.sh --name <name> --section <Workflow|Workstream Bootstrap|Hardening|Release Operations|Engineer Self-Service|Future Slots>`. The script proposes the catalog line and the section; the user confirms before the edit lands. Reuses the format already in `.claude/skills/README.md`.

13. **Sync-list decision.** Call `scripts/sync-check.sh` — if the skill is portable (general-purpose, not Tide-specific), offer to add it to `PORTABLE=( ... )` in `scripts/sync-skills.sh`. The script proposes the diff; the user confirms.

14. **Issue lineage (if `--issue <n>`).** Offer to comment on the issue with:
    ```
    Skill authored: .claude/skills/<name>/SKILL.md
    ```

15. **End-of-turn summary.** One short paragraph: the skill path, the chosen shape, the number of REFACTOR cycles, and whether it was added to the sync list. Plus the followups (catalog entry, sync, issue lineage) that are still pending if any.

## Halt Conditions

Stop and report to the user if:

- Guardrails stanza for the proposed skill can't be drafted — the constraint is too fuzzy to encode. Report what was captured in Intake and ask for sharper refusal conditions.
- Research phase returns no authoritative sources after the second pass — the domain may be too new or too proprietary for the web research approach. Surface and ask if the user wants to provide the source material directly.
- RED phase produces zero rationalizations across all scenarios — the constraint is probably enforceable mechanically (regex, validation, linter). Per Obra: "Don't create [skills] for mechanical constraints — automate it." Surface and confirm before continuing.
- GREEN/REFACTOR fails after 3 cycles — the skill design has a structural problem; show the rationalizations and ask for guidance.
- Target directory already exists and is non-empty — never silently overwrite. Show what's there and ask: resume / archive-and-fresh / abort.
- The proposed name collides with a canonical workflow skill (coral, council, design, issue, bugbash, author-skill itself) — refuse. These are owned by Tide directly and shouldn't be regenerated through this skill.

**Never auto-remediate without surfacing.** The user decides the remediation.

## State Management

Per-run state lives in `state/run-<ISO-timestamp>/`:

```
state/run-<ts>/
  intake.yaml
  research-notes.md
  draft/
    guardrails.md
    description.md
    outline.md
    SKILL.md           # the draft, before scaffold
    evals.json
  red-baseline.md      # subagent outputs from RED (verbatim)
  green-verify.md      # subagent outputs from GREEN
  refactor-1.md        # diffs and re-run outputs per REFACTOR cycle
  refactor-2.md
  audit.log            # timestamped log of every script call and subagent dispatch
```

`state/` is gitignored at the repo level (`.claude/skills/*/state/`). On interrupted runs, the next invocation detects the latest incomplete `run-<ts>/` and offers **resume** / **archive** / **start-fresh**.

## What this skill doesn't do

- **Edit canonical workflow skills.** coral, council, design, issue, bugbash, and author-skill itself are owned by Tide directly. Edit them in a PR, with the council if substantive.
- **Author Claude Code built-ins.** /loop, /schedule, /init are harness-owned, not user skills.
- **Maintain skills over time.** The skill is shipped once; ongoing edits use the same pattern (run RED against the current skill, REFACTOR) but the user owns those passes.
- **Sync to remote.** If `--user` is passed, the skill lands at `~/.claude/skills/<name>/`. Cross-repo distribution is `scripts/sync-skills.sh`'s job — this skill only proposes the sync-list addition.
- **Replace Tide's existing repo conventions.** When the SKILL-TEMPLATE.md shape and Obra best practices conflict, the local template wins (it's tested against this team's workflow). The differences are called out in `references/skill-shapes.md`.

## Output

End-of-turn summary: one short paragraph. Skill name, path written, shape chosen, REFACTOR cycles consumed, and the followup list (catalog entry status, sync-list status, issue lineage status). Example:

> Authored `terraform-review` at `.claude/skills/terraform-review/SKILL.md` as a **technique** skill. Took 2 REFACTOR cycles to close the "I'll just lint after merge" rationalization. Catalog entry added under **Hardening**. Added to `PORTABLE` in `scripts/sync-skills.sh`. No issue lineage (standalone invocation).
