# Scripts — audit-skill

Deterministic steps used by the audit procedure. Each script is debuggable standalone.

## `static-checks.sh`

Runs the deterministic subset of the conventions catalog against a target skill. Outputs JSONL findings (one per line) to stdout or to the `--output` file.

```bash
./static-checks.sh --skill-dir /abs/path/to/.claude/skills/coral \
                   --output state/run-2026-05-10T20-00-00/static-findings.jsonl
```

Exit codes:
- `0` — success (findings emitted regardless of pass/fail).
- `1` — bad args / missing skill dir / missing SKILL.md.

Covers: D1, D2, D3, D5, D8, B1, B2, B3, B4, R1, R2, R3, S1, S2, S4, S6, E1-E4, T1, T2, C1, C3, A1, A2.

See `references/static-checks.md` for the full check list and `references/conventions-catalog.md` for rule statements.

## `findings-report.sh`

Renders merged JSONL findings into a markdown audit report. Accepts one or more `--input` files (static, semantic, pressure findings concat).

```bash
./findings-report.sh --skill coral \
                     --shape orchestration \
                     --input state/run-<ts>/static-findings.jsonl \
                     --input state/run-<ts>/semantic-findings.jsonl \
                     --input state/run-<ts>/pressure-findings.jsonl \
                     --output <dri-repo>/designs/tide-skill-stack/audits/coral-2026-05-10.md
```
`--output` must be an **absolute path into the DRI `<engineer>-designs` checkout** (Design 13) — never a path relative to the code repo, or a bare `designs/…` would create the directory inside the code repo and undo the evacuation.

Exit codes:
- `0` — report written to `--output`.
- `1` — bad args / missing input files.

Output structure documented in `references/findings-report-format.md`. Uses `python3` for JSON parsing.

## `apply-refactor.sh`

Applies a unified diff to a target file with backup + verify + automatic rollback. Used in Phase 2 (refactor).

```bash
./apply-refactor.sh --diff state/run-<ts>/proposals/B1.diff \
                    --target /path/to/skill/SKILL.md \
                    --state-dir state/run-<ts>
```

Exit codes:
- `0` — diff applied; verify passed; audit-log line emitted.
- `1` — bad args / missing files.
- `2` — `patch` failed; target rolled back from backup.
- `4` — verify failed (frontmatter, line count, syntax, etc); target rolled back.

Verify rules per file type:
- `SKILL.md` — frontmatter parseable, ≤500 lines, description ≤1024 chars.
- `*.sh`     — `bash -n` syntax OK, shebang present, `set -euo pipefail` present.
- `*.md`     — file readable (minimal check).

## Authoring guidelines

When adding a new script:

- Flag-based args. No positional args.
- `set -euo pipefail` at the top.
- A `die` helper that takes (message, exit-code) so error formatting is consistent.
- `--dry-run` for any side-effecting script.
- Exit codes documented in this README.
- Portable across macOS (BSD) and Linux (GNU). Prefer `awk` and `python3` over `sed -i` and GNU-only flags.
- Idempotent when possible — running twice should either succeed or refuse cleanly.
- Preserve target file mode when overwriting (use `cat $TMP > $TARGET; rm $TMP`, not `mv`).
