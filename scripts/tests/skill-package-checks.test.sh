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

echo "every skipped finding carries a skip_reason, and the two values are distinct"
# G6 split `skipped` into inapplicable (no subject) and unavailable (subject
# unreachable) because SKILL.md now branches on the field. A `skipped` with neither
# value matches no branch, so a block rule that could not run reads as nothing.
missing=0
for d in "$REPO_ROOT"/.claude/skills/*/; do
  n="$("$CHECKS" --skill-dir "$d" 2>/dev/null | grep '^{' | python3 -c "
import sys,json
b=[json.loads(l) for l in sys.stdin]
print(sum(1 for x in b if x['result']=='skipped' and not x.get('skip_reason')))")"
  [ "$n" -ne 0 ] && { no "$(basename "$d"): $n skipped finding(s) with no skip_reason"; missing=1; }
done
[ "$missing" -eq 0 ] && ok "no skipped finding is missing skip_reason"

echo "a rule with no subject is inapplicable, not an alarm"
probe="$TMP/inapplicable"; mkdir -p "$probe/state" "$probe/evals"
printf -- '---\nname: p\ndescription: Use when probing. NOT for real work.\ncategory: workflow\n---\n# x\n' > "$probe/SKILL.md"
printf '{"evals":[]}\n' > "$probe/evals/evals.json"
out="$("$CHECKS" --skill-dir "$probe" 2>/dev/null | grep '^{')"
r="$(printf '%s' "$out" | python3 -c "
import sys,json
b=[json.loads(l) for l in sys.stdin]
s1=[x for x in b if x['catalog_ref']=='S1']
print(s1[0]['result'], s1[0].get('skip_reason')) if s1 else print('absent none')")"
[ "$r" = "skipped inapplicable" ] && ok "S1 on a skill with no scripts/ is skipped/inapplicable" \
  || no "S1 with no scripts/: got '$r', wanted 'skipped inapplicable'"

echo "an unreadable evals.json does not drop the three rules that read it"
printf 'not json\n' > "$probe/evals/evals.json"
n="$("$CHECKS" --skill-dir "$probe" 2>/dev/null | grep -c '^{')"
[ "$n" -eq 26 ] && ok "unparseable evals.json still emits 26 findings" \
  || no "unparseable evals.json emitted $n findings, wanted 26 (E2/E3/E4 dropped?)"
rm -f "$probe/evals/evals.json"
n="$("$CHECKS" --skill-dir "$probe" 2>/dev/null | grep -c '^{')"
[ "$n" -eq 26 ] && ok "a missing evals.json still emits 26 findings" \
  || no "missing evals.json emitted $n findings, wanted 26"

# The core skills all have a parseable evals.json, so the sweep above never
# exercises the E2/E3/E4 skip path. Assert skip_reason where it actually fires.
printf 'not json\n' > "$probe/evals/evals.json"
for id in E2 E3 E4; do
  r="$("$CHECKS" --skill-dir "$probe" 2>/dev/null | grep '^{' | python3 -c "
import sys,json
b=[json.loads(l) for l in sys.stdin]
m=[x for x in b if x['catalog_ref']=='$id']
print(m[0]['result'], m[0].get('skip_reason')) if m else print('ABSENT none')")"
  [ "$r" = "skipped unavailable" ] && ok "$id on an unreadable evals.json is skipped/unavailable" \
    || no "$id on an unreadable evals.json: got '$r', wanted 'skipped unavailable'"
done
# The counter-breakage path (a working evals.json the counter cannot read) is the
# other producer of these three, and it must carry the reason too.
printf '{"evals":[]}\n' > "$probe/evals/evals.json"
pydir="$TMP/nopy"; mkdir -p "$pydir"; printf '#!/bin/sh\nexit 127\n' > "$pydir/python3"; chmod +x "$pydir/python3"
# Parse the JSON; do not pattern-match field adjacency. skip_reason is emitted
# last, so a positional grep silently matches nothing and the case reads as green.
r="$(PATH="$pydir:/usr/bin:/bin" "$CHECKS" --skill-dir "$probe" 2>/dev/null | grep '^{' \
     | python3 -c "
import sys,json
b=[json.loads(l) for l in sys.stdin]
print(sum(1 for x in b if x['result']=='skipped' and x.get('skip_reason')=='unavailable'
          and x['catalog_ref'].startswith('E')))")"
[ "${r:-0}" -ge 4 ] && ok "a broken interpreter yields 4 skipped/unavailable E-rules" \
  || no "broken interpreter: $r skipped/unavailable E-rules, wanted >= 4"
rm -f "$probe/evals/evals.json"

echo "the sibling-repo case: a rule whose subject is unreachable says so"
# sync-skills.sh --target <repo> is a supported flow. That repo is a git repo with
# no catalog README and no sync-skills.sh, so REPO_ROOT is non-empty and C1's input
# is absent — the case where guarding the skip on REPO_ROOT dropped C1 silently.
sib="$TMP/sibling"; mkdir -p "$sib/.claude/skills"
( cd "$sib" && git init -q . )
cp -R "$REPO_ROOT/.claude/skills/root-cause" "$sib/.claude/skills/"
out="$("$CHECKS" --skill-dir "$sib/.claude/skills/root-cause" 2>/dev/null | grep '^{')"
n="$(printf '%s' "$out" | grep -c '^{')"
[ "$n" -eq 26 ] && ok "a skill in a sibling repo still emits 26 findings" \
  || no "sibling repo emitted $n findings, wanted 26"
for id in C1 C3 T1; do
  r="$(printf '%s' "$out" | python3 -c "
import sys,json
b=[json.loads(l) for l in sys.stdin]
m=[x for x in b if x['catalog_ref']=='$id']
print(m[0]['result'], m[0].get('skip_reason')) if m else print('ABSENT none')")"
  [ "$r" = "skipped unavailable" ] && ok "$id is skipped/unavailable, not dropped or failed" \
    || no "$id in a sibling repo: got '$r', wanted 'skipped unavailable'"
done

echo "rules that could never fail, now can"
# A2 required DOUBLED backslashes, which a real Windows path never has, so it
# passed on every skill regardless of content. "A rule nobody can fail is not a rule."
a2probe="$TMP/a2"; mkdir -p "$a2probe/state" "$a2probe/references"
printf -- '---\nname: p\ndescription: Use when probing. NOT for real work.\ncategory: workflow\n---\n# x\n' > "$a2probe/SKILL.md"
a2() { "$CHECKS" --skill-dir "$a2probe" 2>/dev/null | grep '^{' | python3 -c "
import sys,json
b=[json.loads(l) for l in sys.stdin]
print([x['result'] for x in b if x['catalog_ref']=='A2'][0])"; }
printf 'Open C:\\Users\\dev\\project.txt\n' > "$a2probe/references/w.md"
[ "$(a2)" = "fail" ] && ok "A2 fires on a drive-letter path in references/" || no "A2 missed C:\\Users\\dev"
printf 'Use \\\\server\\share\\file.txt\n' > "$a2probe/references/w.md"
[ "$(a2)" = "fail" ] && ok "A2 fires on a UNC path" || no "A2 missed a UNC path"
# jq format strings are single-char escapes, not paths. A2 must not fire on them.
printf 'jq -r "\\(.metadata.name)\\t\\(.status)"\n' > "$a2probe/references/w.md"
[ "$(a2)" = "pass" ] && ok "A2 does not fire on a jq format string" || no "A2 false-positives on jq escapes"
printf 'A clean a/posix/path and a `--flag`.\n' > "$a2probe/references/w.md"
[ "$(a2)" = "pass" ] && ok "A2 passes clean prose" || no "A2 false-positives on clean prose"

# B1 and D2 are stated strictly-under in the rubric; the checker admitted equality,
# so a body at exactly the ceiling passed a block rule stated as "under".
bprobe="$TMP/b1"; mkdir -p "$bprobe/state"
{ printf -- '---\nname: p\ndescription: Use when probing. NOT for real work.\ncategory: workflow\n---\n'
  i=1; while [ $i -le 495 ]; do echo "line $i"; i=$((i+1)); done; } > "$bprobe/SKILL.md"
n_lines=$(wc -l < "$bprobe/SKILL.md" | tr -d ' ')
b1() { "$CHECKS" --skill-dir "$bprobe" 2>/dev/null | grep '^{' | python3 -c "
import sys,json
b=[json.loads(l) for l in sys.stdin]
print([x['result'] for x in b if x['catalog_ref']=='B1'][0])"; }
# The fixture must be EXACTLY 500. At 501 the case passes under `<=` too, and the
# assertion proves nothing — assert the count before asserting the verdict.
if [ "$n_lines" -ne 500 ]; then
  no "B1 boundary fixture is $n_lines lines, not 500 — the case cannot test the boundary"
elif [ "$(b1)" = "fail" ]; then ok "B1 fails at exactly 500 lines (rubric says under 500)"
else no "B1 passed at exactly 500 lines; the rubric states strictly-under"; fi
# Rebuild at 499 rather than trimming: wc -l counts newlines, so an in-place
# delete can leave the count where it was.
{ printf -- '---\nname: p\ndescription: Use when probing. NOT for real work.\ncategory: workflow\n---\n'
  i=1; while [ $i -le 494 ]; do echo "line $i"; i=$((i+1)); done; } > "$bprobe/SKILL.md"
n_lines=$(wc -l < "$bprobe/SKILL.md" | tr -d ' ')
[ "$n_lines" -eq 499 ] && [ "$(b1)" = "pass" ] && ok "B1 passes at 499 lines" \
  || no "B1 at $n_lines lines: got $(b1), wanted pass at 499"

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
