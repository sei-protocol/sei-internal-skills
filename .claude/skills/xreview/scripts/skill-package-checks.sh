#!/usr/bin/env bash
# skill-package-checks.sh — Run the deterministic subset of the conventions catalog.
#
# Usage:
#   skill-package-checks.sh --skill-dir <abs-path> [--output <file.jsonl>]
#
# Output: JSONL — one finding per line — to stdout or to the file given by --output.
# Exit code: 0 on success (regardless of findings), non-zero on tool errors.

set -euo pipefail

SKILL_DIR=""
OUTPUT=""

die() { printf 'skill-package-checks.sh: %s\n' "$1" >&2; exit "${2:-1}"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skill-dir) SKILL_DIR="$2"; shift 2 ;;
    --output)    OUTPUT="$2"; shift 2 ;;
    *) die "unknown flag: $1" 1 ;;
  esac
done

[[ -n "$SKILL_DIR" ]] || die "--skill-dir is required" 1
[[ -d "$SKILL_DIR" ]] || die "skill dir does not exist: $SKILL_DIR" 1
[[ -f "$SKILL_DIR/SKILL.md" ]] || die "SKILL.md missing in $SKILL_DIR" 1

# Set up output destination
if [[ -n "$OUTPUT" ]]; then
  mkdir -p "$(dirname "$OUTPUT")"
  exec 3> "$OUTPUT"
else
  exec 3>&1
fi

# Emit a JSONL finding.
emit() {
  # Args: id severity title result evidence catalog_ref
  local id="$1" severity="$2" title="$3" result="$4" evidence="$5" catalog_ref="$6"
  # Escape JSON-special chars in evidence: backslash, double-quote, newline, tab, CR.
  local esc_evidence="${evidence//\\/\\\\}"
  esc_evidence="${esc_evidence//\"/\\\"}"
  esc_evidence="${esc_evidence//$'\n'/\\n}"
  esc_evidence="${esc_evidence//$'\t'/\\t}"
  esc_evidence="${esc_evidence//$'\r'/\\r}"
  printf '{"id":"%s","severity":"%s","title":"%s","result":"%s","evidence":"%s","catalog_ref":"%s","source":"static"}\n' \
    "$id" "$severity" "$title" "$result" "$esc_evidence" "$catalog_ref" >&3
}

SKILL_NAME="$(basename "$SKILL_DIR")"
SKILL_MD="$SKILL_DIR/SKILL.md"

# Resolve REPO_ROOT once up front so evidence paths can be emitted as
# repo-relative. Falls back to empty string if the skill isn't under a git
# repo, in which case paths stay absolute (no leak — the script is being
# run against an out-of-tree skill).
REPO_ROOT="$(cd "$SKILL_DIR" && git rev-parse --show-toplevel 2>/dev/null || echo "")"

# rel <path> — strip REPO_ROOT prefix from a path (and the trailing slash).
# Used in evidence strings so committed audit reports don't leak the
# developer's absolute home directory. Pass a single string or a comma-
# separated list; works on each.
rel() {
  if [[ -z "$REPO_ROOT" ]]; then
    printf '%s' "$1"
  else
    printf '%s' "${1//$REPO_ROOT\//}"
  fi
}

# --- Description checks (parse the YAML frontmatter) ---
# Extract everything between the first two '---' lines, then grab the 'description:' value.
DESC="$(awk '
  /^---$/ { fence++; if (fence == 2) exit; next }
  fence == 1 { print }
' "$SKILL_MD" | awk '
  /^description:/ { capturing = 1; sub(/^description: */, ""); body = $0; next }
  capturing && /^[a-zA-Z]+:/ { capturing = 0 }
  capturing { body = body " " $0 }
  END { print body }
')"

# Strip surrounding quotes (single or double) from the description value
DESC_STRIPPED="$DESC"
DESC_STRIPPED="${DESC_STRIPPED#\"}"
DESC_STRIPPED="${DESC_STRIPPED%\"}"
DESC_STRIPPED="${DESC_STRIPPED#\'}"
DESC_STRIPPED="${DESC_STRIPPED%\'}"

DESC_LEN=${#DESC_STRIPPED}

# D1 — starts with "Use when" (case-insensitive via tr — bash 3.2 compatible)
DESC_LOWER="$(printf '%s' "$DESC_STRIPPED" | tr '[:upper:]' '[:lower:]')"
if [[ "$DESC_LOWER" == "use when"* ]]; then
  emit "D1" "block" "Description starts with 'Use when'" "pass" "" "D1"
else
  preview="${DESC_STRIPPED:0:60}"
  emit "D1" "block" "Description starts with 'Use when'" "fail" "starts with: '${preview}...'" "D1"
fi

# D2 — under 1024 chars
if (( DESC_LEN <= 1024 )); then
  emit "D2" "block" "Description under 1024 chars" "pass" "$DESC_LEN chars" "D2"
else
  emit "D2" "block" "Description under 1024 chars" "fail" "$DESC_LEN chars (over by $((DESC_LEN - 1024)))" "D2"
fi

# D3 — anti-triggers present
if printf '%s' "$DESC_STRIPPED" | grep -qE 'NOT for|do NOT|SKIP if|Anti-trigger'; then
  emit "D3" "warn" "Description includes anti-triggers" "pass" "" "D3"
else
  emit "D3" "warn" "Description includes anti-triggers" "fail" "no anti-trigger markers found" "D3"
fi

# D5 — third person (no "I ", "I'm", "I'd", "I can")
if printf '%s' "$DESC_STRIPPED" | grep -qE "\\bI('m| can| will| would| have|d)?\\s"; then
  matched=$(printf '%s' "$DESC_STRIPPED" | grep -oE "\\bI('m| can| will| would| have|d)?\\s\\w+" | head -1)
  emit "D5" "block" "Description in third person" "fail" "first-person token: '$matched'" "D5"
else
  emit "D5" "block" "Description in third person" "pass" "" "D5"
fi

# D8 — ≥3 trigger phrases (heuristic: count single-quoted phrases)
# `|| true`: no quoted phrase means grep exits 1, and under `set -euo pipefail`
# that kills the run — dropping every check after D8, block rules included.
TRIGGER_COUNT=$(printf '%s' "$DESC_STRIPPED" | { grep -oE "'[^']+'" || true; } | wc -l | tr -d ' ')
if (( TRIGGER_COUNT >= 3 )); then
  emit "D8" "warn" "Description includes ≥3 concrete trigger phrases" "pass" "$TRIGGER_COUNT quoted phrases" "D8"
else
  emit "D8" "warn" "Description includes ≥3 concrete trigger phrases" "fail" "$TRIGGER_COUNT quoted phrases" "D8"
fi

# --- SKILL.md body checks ---
SKILL_MD_LINES=$(wc -l < "$SKILL_MD" | tr -d ' ')

# B1 — under 500 lines
if (( SKILL_MD_LINES <= 500 )); then
  emit "B1" "block" "SKILL.md under 500 lines" "pass" "$SKILL_MD_LINES lines" "B1"
else
  emit "B1" "block" "SKILL.md under 500 lines" "fail" "$SKILL_MD_LINES lines (over by $((SKILL_MD_LINES - 500)))" "B1"
fi

# B2, B3 — Guardrails / Halt sections (warn-level since some shapes legitimately omit;
# block-level for procedural/discipline shapes — the shape-conditional severity is the
# load-bearing context for triaging the finding).
# Case-insensitive: matches '## Guardrails', '## guardrails', '## Halt Conditions',
# '## Halt conditions', '## Halt Condition', etc.
if grep -qiE '^## Guardrails' "$SKILL_MD"; then
  emit "B2" "warn" "SKILL.md has Guardrails stanza" "pass" "" "B2"
else
  emit "B2" "warn" "SKILL.md has Guardrails stanza" "fail" "no '## Guardrails' heading found (block for procedural/discipline shapes)" "B2"
fi

if grep -qiE '^## Halt Conditions?' "$SKILL_MD"; then
  emit "B3" "warn" "SKILL.md has Halt Conditions section" "pass" "" "B3"
else
  emit "B3" "warn" "SKILL.md has Halt Conditions section" "fail" "no '## Halt Conditions' heading found (block for procedural/discipline shapes)" "B3"
fi

# B4 — numbered procedure with bolded step names
PROC_STEPS=$(grep -cE '^[0-9]+\. \*\*' "$SKILL_MD" || true)
if (( PROC_STEPS >= 3 )); then
  emit "B4" "warn" "SKILL.md has numbered procedure" "pass" "$PROC_STEPS numbered+bolded steps" "B4"
else
  emit "B4" "warn" "SKILL.md has numbered procedure" "fail" "only $PROC_STEPS numbered+bolded steps (skip for non-procedural shapes)" "B4"
fi

# --- References checks ---
if [[ -d "$SKILL_DIR/references" ]]; then
  # R1 — one-level deep
  nested=$(find "$SKILL_DIR/references" -mindepth 2 -name '*.md' 2>/dev/null || true)
  if [[ -z "$nested" ]]; then
    emit "R1" "block" "References one-level-deep" "pass" "" "R1"
  else
    emit "R1" "block" "References one-level-deep" "fail" "nested: $(rel "$nested")" "R1"
  fi

  # R2 — large files have TOC
  large_no_toc=""
  while IFS= read -r f; do
    [[ -z "$f" ]] && continue
    lines=$(wc -l < "$f" | tr -d ' ')
    if (( lines > 100 )); then
      if ! head -50 "$f" | grep -qE '^## '; then
        large_no_toc="${large_no_toc}${f##$SKILL_DIR/} ($lines lines), "
      fi
    fi
  done < <(find "$SKILL_DIR/references" -name '*.md' -type f 2>/dev/null)

  if [[ -z "$large_no_toc" ]]; then
    emit "R2" "warn" "Large reference files have TOC" "pass" "" "R2"
  else
    emit "R2" "warn" "Large reference files have TOC" "fail" "${large_no_toc%, }" "R2"
  fi

  # R3 — no @skills force-loads
  force_loads=$(grep -lrE '@skills/' "$SKILL_DIR/references" "$SKILL_MD" 2>/dev/null || true)
  if [[ -z "$force_loads" ]]; then
    emit "R3" "info" "No @skills force-load syntax" "pass" "" "R3"
  else
    emit "R3" "info" "No @skills force-load syntax" "fail" "found in: $(rel "$force_loads")" "R3"
  fi
fi

# --- Scripts checks (only if scripts/ exists — procedural shape) ---
if [[ -d "$SKILL_DIR/scripts" ]]; then
  for script in "$SKILL_DIR/scripts"/*.sh; do
    [[ -f "$script" ]] || continue
    script_name="${script##*/}"

    # S1 — shebang
    if head -1 "$script" | grep -qE '^#!'; then
      emit "S1.$script_name" "block" "Script $script_name has shebang" "pass" "" "S1"
    else
      emit "S1.$script_name" "block" "Script $script_name has shebang" "fail" "no shebang on line 1" "S1"
    fi

    # S2 — set -euo pipefail
    if grep -q 'set -euo pipefail' "$script"; then
      emit "S2.$script_name" "block" "Script $script_name has set -euo pipefail" "pass" "" "S2"
    else
      emit "S2.$script_name" "block" "Script $script_name has set -euo pipefail" "fail" "missing 'set -euo pipefail'" "S2"
    fi

    # S6 — flag-style args (heuristic)
    if grep -qE -- '--[a-z]+\)' "$script" || grep -q 'getopts' "$script"; then
      emit "S6.$script_name" "warn" "Script $script_name uses flag-style args" "pass" "" "S6"
    else
      emit "S6.$script_name" "warn" "Script $script_name uses flag-style args" "fail" "no --flag patterns or getopts found" "S6"
    fi
  done

  # S4 — scripts/README.md exists
  if [[ -f "$SKILL_DIR/scripts/README.md" ]]; then
    emit "S4" "warn" "scripts/README.md documents the scripts" "pass" "" "S4"
  else
    emit "S4" "warn" "scripts/README.md documents the scripts" "fail" "missing scripts/README.md" "S4"
  fi
fi

# --- Evals checks ---
EVALS_JSON="$SKILL_DIR/evals/evals.json"
if [[ -f "$EVALS_JSON" ]]; then
  # E1 — parseable JSON
  if python3 -c "import json,sys; json.load(open(sys.argv[1]))" "$EVALS_JSON" 2>/dev/null; then
    emit "E1" "block" "evals.json is parseable" "pass" "" "E1"

    # E2, E3, E4 — counts and source field. One pass, path via argv (not
    # interpolated), and both eval shapes: a top-level list and {"evals": [...]}.
    # A skill using the list form used to raise AttributeError here and kill the
    # run mid-stream, dropping T1 and C1 — two block rules — with no trace.
    EVAL_COUNTS=$(python3 - "$EVALS_JSON" <<'PY' 2>/dev/null || true
import json,sys
d=json.load(open(sys.argv[1]))
evals = d if isinstance(d,list) else d.get('evals',[])
evals = [e for e in evals if isinstance(e,dict)]
print(sum(1 for e in evals if e.get('type')=='happy-path'),
      sum(1 for e in evals if e.get('type')=='halt-condition'),
      len(evals),
      sum(1 for e in evals if not e.get('source')))
PY
)
    if [[ -z "$EVAL_COUNTS" ]]; then
      # Visibly skipped, never silently dropped: a check that could not run is
      # a finding, not an absence.
      emit "E2" "block" "evals.json has ≥1 happy-path + ≥1 halt-condition" "skipped" "could not read eval entries" "E2"
      emit "E3" "warn" "evals.json has ≥5 evals" "skipped" "could not read eval entries" "E3"
      emit "E4" "warn" "every eval carries a source field" "skipped" "could not read eval entries" "E4"
      HAPPY=0; HALT=0; TOTAL=0; NO_SOURCE=0
      EVAL_SKIP=1
    else
      read -r HAPPY HALT TOTAL NO_SOURCE <<< "$EVAL_COUNTS"
      EVAL_SKIP=0
    fi

    if (( EVAL_SKIP == 0 )); then

    if (( HAPPY >= 1 && HALT >= 1 )); then
      emit "E2" "block" "evals.json has ≥1 happy-path + ≥1 halt-condition" "pass" "happy-path=$HAPPY, halt-condition=$HALT" "E2"
    else
      emit "E2" "block" "evals.json has ≥1 happy-path + ≥1 halt-condition" "fail" "happy-path=$HAPPY, halt-condition=$HALT" "E2"
    fi

    if (( TOTAL >= 3 )); then
      emit "E3" "warn" "evals.json has ≥3 entries (Obra ideal)" "pass" "$TOTAL entries" "E3"
    else
      emit "E3" "warn" "evals.json has ≥3 entries (Obra ideal)" "fail" "$TOTAL entries" "E3"
    fi

    if (( NO_SOURCE == 0 )); then
      emit "E4" "warn" "Every eval has a source field" "pass" "" "E4"
    else
      emit "E4" "warn" "Every eval has a source field" "fail" "$NO_SOURCE entries missing source" "E4"
    fi
    fi
  else
    emit "E1" "block" "evals.json is parseable" "fail" "JSON parse error" "E1"
  fi
else
  emit "E1" "block" "evals.json exists" "fail" "missing $(rel "$EVALS_JSON")" "E1"
fi

# --- State checks ---
GITIGNORE_OK=0
if [[ -n "$REPO_ROOT" ]] && [[ -f "$REPO_ROOT/.gitignore" ]]; then
  if grep -qE "\.claude/skills/\*/state/?|^state/" "$REPO_ROOT/.gitignore"; then
    GITIGNORE_OK=1
  fi
fi
if [[ -f "$SKILL_DIR/.gitignore" ]]; then
  if grep -qE "^state/?" "$SKILL_DIR/.gitignore"; then
    GITIGNORE_OK=1
  fi
fi

if (( GITIGNORE_OK == 1 )); then
  emit "T1" "block" "state/ is gitignored" "pass" "" "T1"
else
  emit "T1" "block" "state/ is gitignored" "fail" "no matching pattern in repo or local .gitignore" "T1"
fi

if [[ -f "$SKILL_DIR/state/.gitkeep" ]]; then
  emit "T2" "info" "state/.gitkeep exists" "pass" "" "T2"
else
  emit "T2" "info" "state/.gitkeep exists" "fail" "missing state/.gitkeep" "T2"
fi

# --- Catalog & sync checks ---
CATALOG="$REPO_ROOT/.claude/skills/README.md"
if [[ -f "$CATALOG" ]]; then
  if grep -qE "\`${SKILL_NAME}/\`" "$CATALOG"; then
    emit "C1" "block" "Skill listed in catalog README" "pass" "" "C1"
  else
    emit "C1" "block" "Skill listed in catalog README" "fail" "no entry for ${SKILL_NAME}/ in $(rel "$CATALOG")" "C1"
  fi
fi

# C3 resolves the skill's `category:` against the three domain lists in
# sync-skills.sh, using that script's own whole-word semantics. An earlier form
# grepped the file unanchored, which passed `portable`, `sei`, `all`, `work` and
# `cp` — every one of them rejected by `sync-skills.sh --verify`. A checker that
# passes wrongly ships; one that fails loudly gets fixed.
SYNC="$REPO_ROOT/scripts/sync-skills.sh"
if [[ -f "$SYNC" ]]; then
  CAT="$(sed -n 's/^category:[[:space:]]*//p' "$SKILL_MD" | head -1 | tr -d '"'"'"'\r' | awk '{$1=$1;print}')"
  DOMAINS=""
  for v in PORTABLE_DOMAINS SEI_DOMAINS SEI_INTERNAL_SKILLS_LOCAL_DOMAINS; do
    DOMAINS="$DOMAINS $(sed -n "s/^${v}=\"\(.*\)\"$/\1/p" "$SYNC")"
  done
  if [[ -n "$CAT" ]] && printf ' %s ' "$DOMAINS" | grep -q " $CAT "; then
    emit "C3" "warn" "Skill category resolves to a sync alias" "pass" "$CAT" "C3"
  else
    emit "C3" "warn" "Skill category resolves to a sync alias" "fail" "category '$CAT' is in no domain list" "C3"
  fi
fi

# A1 — no time-sensitive content. Skips two false-positive shapes:
#   1. lines inside fenced code blocks (```) — code examples may contain literal
#      "currently" / dates that don't claim time-sensitivity in skill prose
#   2. lines containing template-placeholder slots like [phase] or <alias> —
#      these are templates the skill emits at runtime, not claims about content
#
# Word boundary on 'currently' is via character-class flanking (POSIX awk has
# no \b). 'as of <year>' and 'in the latest version' don't need flanking.
A1_FILES=()
for f in "$SKILL_MD" "$SKILL_DIR/references"/*.md; do
  [[ -f "$f" ]] || continue
  if awk '
    /^```/ { in_code = !in_code; next }
    in_code { next }
    /\[[a-z][^]]*\]/ { next }   # bracket placeholder, e.g. [phase]
    /<[a-z][^>]*>/ { next }      # angle placeholder, e.g. <alias>
    { low = tolower($0) }
    low ~ /as of [0-9][0-9][0-9][0-9]/         { print; exit }
    low ~ /in the latest version/              { print; exit }
    low ~ /(^|[^a-z])currently([^a-z]|$)/      { print; exit }
  ' "$f" 2>/dev/null | grep -q .; then
    A1_FILES+=("$f")
  fi
done
if (( ${#A1_FILES[@]} > 0 )); then
  hits=$(IFS=,; printf '%s' "${A1_FILES[*]:0:3}")
  emit "A1" "warn" "No time-sensitive content" "fail" "found in: $(rel "$hits")" "A1"
else
  emit "A1" "warn" "No time-sensitive content" "pass" "" "A1"
fi

if grep -qE '\\\\\w+\\\\' "$SKILL_MD" 2>/dev/null; then
  emit "A2" "warn" "No Windows-style paths" "fail" "backslash-separated path patterns found" "A2"
else
  emit "A2" "warn" "No Windows-style paths" "pass" "" "A2"
fi

# Close output FD
exec 3>&-
