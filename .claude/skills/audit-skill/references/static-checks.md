# Static Checks

What `scripts/static-checks.sh` verifies and how. Each check maps to one or more rules in `conventions-catalog.md`.

## Invocation

```bash
./scripts/static-checks.sh --skill-dir <abs-path> [--output <file.jsonl>]
```

Output is JSONL (one finding per line) to stdout, or to `--output` if specified. Exit code is 0 on success regardless of findings; non-zero only on tool errors (missing dir, unreadable files).

## Output format

One JSON object per line:

```json
{"id":"D2","severity":"block","title":"Description under 1024 chars","result":"pass","evidence":"708 chars","catalog_ref":"D2"}
{"id":"R1","severity":"block","title":"References one-level-deep","result":"fail","evidence":"Found references/sub/nested.md","catalog_ref":"R1"}
```

Fields:

- `id` — finding ID (typically matches catalog ID; can be suffixed `D2.1`, `D2.2` for multiple instances).
- `severity` — `block` | `warn` | `info`.
- `title` — short rule statement.
- `result` — `pass` | `fail` | `skip` (rule didn't apply to this shape).
- `evidence` — the concrete observation (chars counted, lines found, file paths).
- `catalog_ref` — the catalog ID for traceability.

## Checks performed

### Description checks (frontmatter)

The script extracts the `description:` field from `SKILL.md` frontmatter (between the `---` delimiters) and runs:

| Catalog ID | Check |
|------------|-------|
| D1 | description begins with `Use when` (case-insensitive) |
| D2 | length ≤ 1024 chars |
| D3 | contains anti-trigger marker: `NOT for` OR `do NOT` OR `SKIP if` OR `Anti-trigger` |
| D5 | does NOT contain first-person markers: `\bI \b` OR `\bI'\w` OR `\byou should\b` |
| D8 | contains ≥3 trigger phrases (heuristic: ≥3 single-quoted phrases or ≥3 occurrences of "X" patterns in the description) |

D4, D6, D7 are semantic — they go to the subagent.

### SKILL.md body checks

| Catalog ID | Check |
|------------|-------|
| B1 | `wc -l SKILL.md` ≤ 500 |
| B2 | `grep -E '^## Guardrails'` returns ≥1 match (skipped for non-procedural/non-discipline shapes) |
| B3 | `grep -E '^## Halt Conditions?'` returns ≥1 match (skipped for non-procedural/non-discipline shapes) |
| B4 | `grep -cE '^[0-9]+\. \*\*'` ≥ 3 (procedural shape) |

### References

| Catalog ID | Check |
|------------|-------|
| R1 | `find references -mindepth 2 -name '*.md'` returns empty |
| R2 | For each `references/*.md` with line count >100: contains `^## ` (a section heading) in first 50 lines |
| R3 | `grep -lE '@skills/' references/*.md SKILL.md` returns empty |

### Scripts (procedural only)

| Catalog ID | Check |
|------------|-------|
| S1 | every `scripts/*.sh` has shebang on line 1 (`head -1 | grep -E '^#!'`) |
| S2 | every `scripts/*.sh` contains `set -euo pipefail` |
| S4 | `scripts/README.md` exists |
| S6 | every `scripts/*.sh` shows flag-style args: contains `--[a-z]` OR uses `getopts` OR uses `case "$1" in --*` |

### Evals

| Catalog ID | Check |
|------------|-------|
| E1 | `evals/evals.json` exists and is valid JSON (parse via `python3 -c 'import json,sys;json.load(open(sys.argv[1]))'`) |
| E2 | counts: ≥1 entry with `type == "happy-path"` AND ≥1 with `type == "halt-condition"` |
| E3 | total entry count ≥ 3 |
| E4 | every entry has a non-empty `source` field |

### State

| Catalog ID | Check |
|------------|-------|
| T1 | `state/` pattern present in `.gitignore` at repo root OR local `.gitignore` in skill dir |
| T2 | `state/.gitkeep` exists |

### Catalog & sync

| Catalog ID | Check |
|------------|-------|
| C1 | skill name appears in `.claude/skills/README.md` (via `grep -E "\\\`<name>/\\\`"` in catalog file) |
| C3 | skill name appears in `PORTABLE=( ... )` or `SEI=( ... )` in `scripts/sync-skills.sh` (info-level, since not every skill needs to sync) |

### Anti-patterns

| Catalog ID | Check |
|------------|-------|
| A1 | grep for "as of [0-9]{4}", "in the latest version", "currently" in SKILL.md + references (case-insensitive) |
| A2 | grep for `\\\\` (Windows backslash) in path-like strings: e.g., `\w+\\\\\w+` |
| A3 | grep for `@skills/` in any markdown (overlap with R3) |

## What the script does NOT check

- D4 (sibling redirects) — needs to know what adjacent skills exist; punt to semantic.
- D6 (workflow summary trap) — heuristics aren't reliable; semantic check.
- D7 (keyword vocabulary) — needs domain knowledge; semantic.
- B5 (success criteria per step) — judgment call; semantic.
- B6 (consistent terminology) — semantic.
- B7 (no shell in prose) — heuristic too noisy; semantic.
- B8 (guardrails substantive) — counts ≥3 refusal conditions textually but quality is semantic.
- R4 (refs don't duplicate SKILL.md) — semantic.
- S3 (dry-run for side-effecting) — needs to know which scripts are side-effecting; semantic.
- S5 (cross-platform) — semantic.
- E5 (observable signals) — semantic.
- C2 (appropriate section) — judgment; semantic.
- P1-P7 (persuasion stack) — semantic.
- A4, A5, A6 (anti-patterns by judgment) — semantic.

These all surface in the semantic-checks pass (Step 4 of the procedure).

## Adding a new static check

When adding a rule to `conventions-catalog.md` and the rule is mechanically checkable:

1. Add the check function in `static-checks.sh` following the existing pattern.
2. Emit a JSONL finding with the new ID, severity, and check logic.
3. Add a row to this file's check table.
4. Add an eval to `evals/evals.json` that triggers the new check.
