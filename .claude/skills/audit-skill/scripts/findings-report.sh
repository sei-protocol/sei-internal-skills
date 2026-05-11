#!/usr/bin/env bash
# findings-report.sh — Render merged JSONL findings into a markdown audit report.
#
# Usage:
#   findings-report.sh --skill <name> --shape <shape> --input <file.jsonl> [--input <file2.jsonl>] --output <report.md>
#
# Reads one or more JSONL files (static, semantic, pressure), merges, groups by
# severity, and writes a markdown report. Uses python3 for JSON parsing because
# shell is too clumsy for nested structure.

set -euo pipefail

SKILL=""
SHAPE=""
INPUTS=()
OUTPUT=""

die() { printf 'findings-report.sh: %s\n' "$1" >&2; exit "${2:-1}"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skill)  SKILL="$2"; shift 2 ;;
    --shape)  SHAPE="$2"; shift 2 ;;
    --input)  INPUTS+=("$2"); shift 2 ;;
    --output) OUTPUT="$2"; shift 2 ;;
    *) die "unknown flag: $1" 1 ;;
  esac
done

[[ -n "$SKILL" ]]  || die "--skill is required" 1
[[ -n "$SHAPE" ]]  || die "--shape is required" 1
[[ -n "$OUTPUT" ]] || die "--output is required" 1
[[ ${#INPUTS[@]} -gt 0 ]] || die "at least one --input is required" 1

for f in "${INPUTS[@]}"; do
  [[ -f "$f" ]] || die "input not found: $f" 1
done

mkdir -p "$(dirname "$OUTPUT")"
TODAY="$(date -u +%Y-%m-%d)"
AUTHOR="$(git config user.name 2>/dev/null || echo unknown)"

python3 - "$SKILL" "$SHAPE" "$TODAY" "$AUTHOR" "$OUTPUT" "${INPUTS[@]}" <<'PY'
import json
import sys
from pathlib import Path
from collections import defaultdict

skill, shape, today, author, output, *inputs = sys.argv[1:]

findings = []
for path in inputs:
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                findings.append(json.loads(line))
            except json.JSONDecodeError:
                continue

# Counts by severity (excluding skipped, which is a result, not a severity)
counts = defaultdict(int)
skipped = []
pressure_findings = []
for f in findings:
    if f.get("result") == "skip":
        skipped.append(f)
        continue
    if f.get("source") == "pressure":
        pressure_findings.append(f)
        continue
    if f.get("result") == "fail":
        counts[f.get("severity", "info")] += 1

# Group failing non-pressure findings by severity
grouped = defaultdict(list)
for f in findings:
    if f.get("result") == "fail" and f.get("source") != "pressure":
        grouped[f.get("severity", "info")].append(f)

# Sort each group by catalog_ref
for sev in grouped:
    grouped[sev].sort(key=lambda x: x.get("catalog_ref", ""))

# Generate report
lines = []
lines.append("---")
lines.append(f"skill: {skill}")
lines.append(f"shape: {shape}")
lines.append(f"audited_on: {today}")
lines.append(f"auditor: {author}")
lines.append("audit_skill_version: 1")
lines.append("phase: audit-only")
lines.append("---")
lines.append("")
lines.append(f"# Skill Audit — `{skill}`")
lines.append("")
lines.append(f"**Shape:** {shape}")
lines.append(f"**Audited:** {today} by {author}")
lines.append(f"**Phase:** audit-only")
lines.append("")
lines.append("## Summary")
lines.append("")
lines.append(f"- **Block:**   {counts.get('block', 0)}")
lines.append(f"- **Warn:**    {counts.get('warn', 0)}")
lines.append(f"- **Info:**    {counts.get('info', 0)}")
lines.append(f"- **Skipped:** {len(skipped)}")
lines.append("")

# Top blockers preview
block_findings = grouped.get("block", [])
if block_findings:
    lines.append("Top blockers:")
    lines.append("")
    for f in block_findings[:5]:
        lines.append(f"- **{f.get('id')}** — {f.get('title', '')}")
    lines.append("")
elif counts.get("warn", 0) > 0:
    lines.append("No blockers — proceed to warnings.")
    lines.append("")
else:
    lines.append("Clean audit — no findings of warn or block severity.")
    lines.append("")

# Sections per severity
for sev_key, sev_label, blurb in [
    ("block", "Block findings", "The skill is not ready to ship with these outstanding."),
    ("warn",  "Warn findings",  "Address before broad rollout."),
    ("info",  "Info findings",  "Observations worth surfacing."),
]:
    items = grouped.get(sev_key, [])
    if not items:
        continue
    lines.append(f"## {sev_label}")
    lines.append("")
    lines.append(f"> Severity: **{sev_key}**. {blurb}")
    lines.append("")
    for f in items:
        lines.append(f"### {f.get('id')} — {f.get('title', '(no title)')}")
        lines.append("")
        lines.append(f"- **Source:** {f.get('source', 'unknown')}")
        lines.append(f"- **Catalog rule:** {f.get('catalog_ref', f.get('id', ''))}")
        lines.append(f"- **Evidence:** {f.get('evidence', '(none)')}")
        lines.append("")

# Pressure scenarios
if pressure_findings:
    lines.append("## Pressure scenarios")
    lines.append("")
    lines.append("> What the skill was tested against and how it responded.")
    lines.append("")
    for p in sorted(pressure_findings, key=lambda x: x.get("scenario_id", "")):
        lines.append(f"### {p.get('id', '?')} — {p.get('title', '(no title)')}")
        lines.append("")
        lines.append(f"- **Scenario:** `{p.get('scenario_id', 'n/a')}`")
        lines.append(f"- **Severity:** {p.get('severity', 'unknown')}")
        lines.append(f"- **Result:** {p.get('result', 'unknown')}")
        evidence = p.get("evidence", "(none)")
        lines.append("")
        lines.append("Subagent reasoning (verbatim):")
        lines.append("")
        for evline in evidence.splitlines() or [evidence]:
            lines.append(f"> {evline}")
        lines.append("")

# Skipped
if skipped:
    lines.append("## Skipped checks")
    lines.append("")
    lines.append("> Rules that couldn't be evaluated in this audit pass.")
    lines.append("")
    for s in skipped:
        lines.append(f"- **{s.get('id', '?')}** — {s.get('evidence', s.get('title', 'no reason given'))}")
    lines.append("")

lines.append("## References")
lines.append("")
lines.append("- Conventions catalog: `.claude/skills/audit-skill/references/conventions-catalog.md`")
lines.append("- Audit methodology: `.claude/skills/audit-skill/SKILL.md`")
lines.append("")

Path(output).write_text("\n".join(lines) + "\n")
print(output)
PY
