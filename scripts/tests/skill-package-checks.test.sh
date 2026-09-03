#!/usr/bin/env bash
# Regression suite for .claude/skills/xreview/scripts/skill-package-checks.sh.
#
# This suite exists because the checker crashed on real in-repo content — it died
# mid-sweep on validate-release and dropped two block rules with no trace — and no
# gate noticed. A sweep asserting rc=0 and parseable output would have caught it.
# Run: scripts/tests/skill-package-checks.test.sh
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CHECKS="$REPO_ROOT/.claude/skills/xreview/scripts/skill-package-checks.sh"
RUBRIC="$REPO_ROOT/.claude/skills/xreview/references/skill-package-rubric.md"
# Beside the test, not inside the skill package: sync-skills.sh copies a skill
# directory wholesale, so a baseline parked there ships to every install.
BASELINE="$SCRIPT_DIR/block-baseline.txt"

# Severity comes from the RUBRIC, not the emitted finding. B2/B3 are `block` in the
# rubric and `warn` in the script on purpose, so filtering the emitted severity would
# hide a skill that deleted its `## Guardrails` stanza.
RUBRIC_SEV="$(mktemp)"
grep -oE '^\| [A-Z][0-9]+ \| [a-z]+ ' "$RUBRIC" | tr -d '|' | awk '{print $1, $2}' > "$RUBRIC_SEV"

PASS=0; FAIL=0
ok() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
no() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }

TMP="$(mktemp -d)"; trap 'rm -rf "$TMP" "${RUBRIC_SEV:-}"' EXIT

echo "the sweep completes on every core skill"
for d in "$REPO_ROOT"/.claude/skills/*/; do
  n="$(basename "$d")"
  out="$("$CHECKS" --skill-dir "$d" 2>/dev/null)"; rc=$?
  if [ "$rc" -ne 0 ]; then no "$n: rc=$rc (the checker died mid-sweep)"; continue; fi
  if printf '%s\n' "$out" | grep '^{' | python3 -c "
import sys,json
[json.loads(l) for l in sys.stdin]" 2>/dev/null; then ok "$n: rc=0, every line parses"
  else no "$n: emitted a line that is not valid JSON"; fi
done

echo "every emitted rule id resolves to a rubric row"
# Union across every skill. Deriving this from one skill made it depend on that
# skill happening to have references/ and scripts/ — a false FAIL waiting to happen.
: > "$TMP/refs.raw"
for d in "$REPO_ROOT"/.claude/skills/*/; do
  "$CHECKS" --skill-dir "$d" 2>/dev/null | grep '^{' \
    | python3 -c "
import sys,json
print('\n'.join(json.loads(l)['catalog_ref'] for l in sys.stdin))" >> "$TMP/refs.raw"
done
sort -u "$TMP/refs.raw" > "$TMP/refs"
orphans=""
while read -r id; do
  grep -qE "^\| $id " "$RUBRIC" || orphans="$orphans $id"
done < "$TMP/refs"
[ -z "$orphans" ] && ok "no orphan catalog_ref" || no "orphan catalog_ref:$orphans"

echo "no rule tagged [static] is left unimplemented"
unimpl=""
for id in $(grep -oE '^\| [A-Z][0-9]+ \| [a-z]+ \| static ' "$RUBRIC" | grep -oE '[A-Z][0-9]+'); do
  grep -qx "$id" "$TMP/refs" || unimpl="$unimpl $id"
done
[ -z "$unimpl" ] && ok "every [static] rule is emitted" || no "tagged [static], never emitted:$unimpl"

echo "block failures match the committed baseline (differential gate)"
for d in "$REPO_ROOT"/.claude/skills/*/; do
  n="$(basename "$d")"
  got="$("$CHECKS" --skill-dir "$d" 2>/dev/null | grep '^{' | python3 -c "
import sys,json
sev=dict(l.split() for l in open('$RUBRIC_SEV') if l.split())
b=[json.loads(l) for l in sys.stdin]
f=sorted({x['catalog_ref'] for x in b
          if x['result']=='fail' and sev.get(x['catalog_ref'])=='block'})
print(','.join(f) if f else '-')")"
  want="$(awk -v n="$n" '$0 !~ /^[[:space:]]*#/ && $1==n {print $2}' "$BASELINE")"
  if [ -z "$want" ]; then
    no "$n: not in the baseline — add it (a new skill needs a baseline line)"
  elif [ "$got" = "$want" ]; then ok "$n: $got"
  else no "$n: baseline says '$want', checker says '$got' — update the baseline in this commit"; fi
done

echo "the baseline names no skill that no longer exists"
stale=0
while read -r n _; do
  case "$n" in \#*|"") continue ;; esac
  [ -d "$REPO_ROOT/.claude/skills/$n" ] || { no "baseline names $n, which is not in the tree"; stale=1; }
done < <(grep -vE '^[[:space:]]*#' "$BASELINE")
[ "$stale" -eq 0 ] && ok "baseline has no stale entry"

echo "a flag with no value is a usage error, not a crash"
out="$("$CHECKS" --skill-dir 2>&1)"; rc=$?
if [ "$rc" -ne 0 ] && printf '%s' "$out" | grep -q 'needs a value'; then
  ok "--skill-dir with no value diagnoses"
else no "--skill-dir with no value: rc=$rc, output: $out"; fi

echo "a check that cannot run is reported skipped, not dropped"
mkdir -p "$TMP/py"; printf '#!/bin/sh\nexit 127\n' > "$TMP/py/python3"; chmod +x "$TMP/py/python3"
n_skipped="$(PATH="$TMP/py:/usr/bin:/bin" "$CHECKS" --skill-dir "$REPO_ROOT/.claude/skills/xreview" 2>/dev/null \
  | grep -c '"result":"skipped"')"
[ "$n_skipped" -ge 4 ] && ok "broken python3: $n_skipped checks reported skipped" \
  || no "broken python3: expected >=4 skipped, got $n_skipped"

echo ""
echo "skill-package-checks: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
