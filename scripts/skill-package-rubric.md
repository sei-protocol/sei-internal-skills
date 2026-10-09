# Skill-package rubric

The rules a skill under `.claude/skills/` must hold. A reviewer cites them on a pull request that changes a skill. `scripts/skill-package-checks.sh --skill-dir <path>` checks the `[static]` rules. `make test-skill-package-checks` sweeps it over every skill and diffs `block` failures against `scripts/tests/block-baseline.txt`. A finding that cites a rule ID resolves here or nowhere.

**Severities:**

- **block** — the skill is not ready to ship with this finding outstanding.
- **warn** — the skill works but has a known weakness the author should close before broad rollout.
- **info** — observation worth surfacing but not a quality bar.

**Source legend:**

- **[static]** — checkable by `scripts/skill-package-checks.sh` (deterministic).
- **[semantic]** — a reviewer judges it by reading the skill, or by trying a scenario against it.

When a rule applies only to certain shapes, the row names the shape (e.g., `[procedural only]`).

---

## Description (frontmatter)

| ID | Severity | Source | Rule |
|----|----------|--------|------|
| D1 | block | static | Description starts with "Use when" (third person) |
| D2 | block | static | Description is under 1024 characters total |
| D3 | warn | static | Description includes anti-triggers ("NOT for X", "do NOT use", "SKIP if", "Anti-triggers:") |
| D4 | warn | semantic | Description includes ≥1 sibling-skill redirect ("For X, use /other") when adjacent skills exist |
| D5 | block | static | Description is third person (no "I ", "I'm", "I'd", "I can") |
| D6 | warn | semantic | Description does NOT summarize workflow (the Obra CSO trap — workflow summary makes Claude skip the body) |
| D7 | info | semantic | Description keywords match user vocabulary, not synonyms |
| D8 | warn | static | Description includes ≥3 concrete trigger phrases (not just one paraphrase) |

## SKILL.md body

| ID | Severity | Source | Rule |
|----|----------|--------|------|
| B1 | block | static | SKILL.md is under 500 lines (Obra ceiling) |
| B2 | block | static | SKILL.md has a `## Guardrails` stanza [procedural, discipline] |
| B3 | block | static | SKILL.md has a `## Halt Conditions` (or "Halt Conditions") section [procedural, discipline] |

> **B2 and B3 are `block` here and `warn` in the script.** The script cannot tell a procedural or
> discipline skill from a reference skill, and these two rules only block for the former. Read the
> script's `warn` on B2/B3 as "block if this skill is procedural or discipline" and make that call
> yourself. This is the one place the script deliberately under-reports.

| B4 | warn | static | SKILL.md has a numbered procedure (`^[0-9]+\. \*\*`) [procedural]. |
| B5 | warn | semantic | Each procedure step has a clear success criterion (judgment call) [procedural]. |
| B6 | warn | semantic | Body uses one consistent term per concept (no synonyms drifting). |
| B7 | warn | semantic | Body does not embed shell commands in prose (commands belong in `scripts/`) [procedural]. |
| B8 | block | semantic | Guardrails stanza is substantive, not a stub (≥3 refusal conditions named) [procedural, discipline]. |

## References (one level deep)

| ID | Severity | Source | Rule |
|----|----------|--------|------|
| R1 | block | static | All reference files live directly under `references/` (no `references/sub/foo.md`). **Scope:** within a single skill's own `references/` directory. R5 permits cross-skill refs via `../../<sibling-skill>/references/<file>.md`. |
| R2 | warn | static | Reference files >100 lines have a Table of Contents (heading scan in first 50 lines) |
| R3 | info | static | Cross-references to other skills use the skill name only (no `@skills/...` force-loads). Plain-markdown relative links to sibling skills are fine — see R5. |
| R4 | warn | semantic | Reference files do not duplicate SKILL.md content — they extend it |
| R5 | info | semantic | A skill may cross-reference a sibling as `../../<sibling-skill>/references/<file>.md` (relative) or `.claude/skills/<sibling>/references/<file>.md` (repo-root form). Note the double `../`: from inside a `references/` dir, a single `../` resolves to the skill root, not `.claude/skills/`, so it links to a path that does not exist. The rubric permits the link between skills in the same `.claude/skills/` directory. The link must encode a handoff contract or shared methodology. These are documentation links, not force-loads — they do not violate R1, and surface as info-only so reviewers see the cross-skill coupling. |
| R7 | block | semantic | A `references/` file does not contradict `SKILL.md`. Where they diverge the divergence **is** the finding; where the reference declares itself authoritative on divergence, it is correctness-grade. |

## Scripts [procedural only]

| ID | Severity | Source | Rule |
|----|----------|--------|------|
| S1 | block | static | Every `.sh` script begins with a shebang (`#!/usr/bin/env bash` or similar) |
| S2 | block | static | Every `.sh` script contains `set -euo pipefail` |
| S3 | warn | semantic | Side-effecting scripts accept `--dry-run` |
| S4 | warn | static | `scripts/README.md` exists and documents exit codes for each script |
| S5 | info | semantic | Scripts are portable across macOS (BSD tools) and Linux (GNU tools) — no GNU-only extensions like `sed -i ''` |
| S6 | warn | static | Scripts use flag-based args, not positional (heuristic: presence of `--name` / `getopts` / `case "$1" in --*`) |

## Evals

| ID | Severity | Source | Rule |
|----|----------|--------|------|
| E1 | block | static | `evals/evals.json` exists and is parseable |
| E2 | block | static | evals.json has at least 1 happy-path and 1 halt-condition entry (sei-internal-skills minimum) |
| E3 | warn | static | evals.json has at least 3 entries (Obra ideal) |
| E4 | warn | static | Each eval has a `source` field tracing to a scenario, a halt condition, or a production incident |
| E5 | info | semantic | Eval compliance signals are observable (not subjective) |

## State

| ID | Severity | Source | Rule |
|----|----------|--------|------|
| T1 | block | static | Git ignores the skill's `state/` directory (via `.claude/skills/*/state/` at repo level, or local `.gitignore`) |
| T2 | info | static | `state/.gitkeep` exists so the directory survives when empty |

## Catalog

| ID | Severity | Source | Rule |
|----|----------|--------|------|
| C1 | block | static | The `.claude/skills/README.md` catalog lists the skill |
| C2 | warn | semantic | Catalog entry is in an appropriate section (judgment based on skill purpose) |

## Persuasion stack (shape-dependent)

| ID | Severity | Source | Rule |
|----|----------|--------|------|
| P1 | warn | semantic | [discipline] Includes a rationalization table (excuse → reality) |
| P2 | warn | semantic | [discipline] Includes a red-flags list (phrases that signal STOP) |
| P3 | warn | semantic | [discipline] Uses authority language consistently ("YOU MUST", "Never", "No exceptions") |
| P4 | info | semantic | [discipline] Uses social-proof language ("Every time", "Always", failure mode universality) |
| P5 | warn | semantic | [technique, pattern] Balances authority with unity ("we", "our codebase") rather than pure imperative |
| P6 | warn | semantic | [reference] Uses no persuasion language — clarity only |
| P7 | block | semantic | A scenario the skill's shape invites (a shortcut request, time pressure, a plausible exception) does not bypass the skill. The reviewer names the scenario tried. |

## Anti-patterns

| ID | Severity | Source | Rule |
|----|----------|--------|------|
| A1 | warn | static | No time-sensitive content ("as of 2025", "in the latest version", "currently") |
| A2 | warn | static | No Windows-style paths (backslashes in path-like strings) |
| A3 | warn | semantic | No `@skills/...` force-load syntax (force-loads burn context) |
| A4 | warn | semantic | No multi-language code examples for one technique (one strong example beats many mediocre ones) |
| A5 | warn | semantic | No fill-in-the-blank templates (use concrete examples instead) |
| A6 | warn | semantic | No deeply-nested file references (refs are one level deep) — overlaps R1, surfaces from prose context |

---

## Adding a new rule

When the team identifies a new convention:

1. Add a row to the appropriate section in this table.
2. Assign an ID (next free number in the section's prefix; e.g., next description rule = D9).
3. Pick a severity honestly. New rules default to `warn` — promote to `block` after one cycle of real audits shows the rule is load-bearing.
4. Update `scripts/skill-package-checks.sh` if the rule is checkable, and add a case to `scripts/tests/skill-package-checks.test.sh`. Otherwise tag it `[semantic]` and say in the row what a reviewer looks for.
5. State the rule so a reviewer can cite it and be wrong. A rule nobody can fail is not a rule.

## Rules that do not appear here

Things deliberately *not* checked:

- **Specific prose style.** Tone, voice, sentence length — these are author choices, not conventions.
- **Subjective "is this useful."** The reviewer measures conformance, not value. Worthwhile-ness is the owner's call.
- **Domain-specific correctness.** A terraform-review skill that gives bad terraform advice is failing a domain check, not a conventions check. It does not validate the *content*, only the *form*.
- **Performance.** Skill load latency, script execution time. Not in scope.
