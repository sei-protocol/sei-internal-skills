#!/usr/bin/env bash
# Regression suite for the sync scripts and their catalog guard
# (scripts/sync-skills.sh, scripts/sync-agents.sh).
# Run: scripts/tests/catalog-coverage.test.sh  (or `make verify-catalog` for the guard alone).
# Exits non-zero on any failure.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILLS_SH="$SCRIPT_DIR/../sync-skills.sh"
AGENTS_SH="$SCRIPT_DIR/../sync-agents.sh"
SKILLS_DIR="$SCRIPT_DIR/../../.claude/skills"
AGENTS_DIR="$SCRIPT_DIR/../../.claude/agents"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

PASS=0
FAIL=0
ok() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
no() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }
silent() { "$@" >/dev/null 2>&1; }
check()      { local d="$1"; shift; if silent "$@"; then ok "$d"; else no "$d"; fi; }   # PASS when cmd exits 0
check_fail() { local d="$1"; shift; if silent "$@"; then no "$d"; else ok "$d"; fi; }   # PASS when cmd exits non-zero
check_eq()   { local d="$1" want="$2" got="$3"; if [ "$want" = "$got" ]; then ok "$d"; else no "$d (want '$want', got '$got')"; fi; }
# PASS when cmd exits non-zero AND its combined output contains <pat> — guards
# against a silent crash satisfying a bare exit-code assertion (the D1/D2 trap).
check_fail_msg() {
  local d="$1" pat="$2"; shift 2
  local out rc
  out="$("$@" 2>&1)"; rc=$?
  if [ "$rc" -ne 0 ] && printf '%s' "$out" | grep -q "$pat"; then ok "$d"; else no "$d"; fi
}

# Fixtures are created inside the live tree; clean them up even on interrupt so a
# stray dir can't break the real `make verify-catalog` afterwards.
TMP_FIXTURES=()
trap 'rm -rf "${TMP_FIXTURES[@]}"' EXIT

echo "catalog guard — the live catalog is consistent"
check      "skills --verify passes on the real tree" "$SKILLS_SH" --verify
check      "agents --verify passes on the real tree" "$AGENTS_SH" --verify

# The names come from the tree, so a new or retired skill needs no edit here.
echo "every skill directory syncs"
skills_plan="$("$SKILLS_SH" --target /tmp/_cov --dry-run 2>/dev/null)"
for d in "$SKILLS_DIR"/*/; do
  if [ -f "${d}SKILL.md" ]; then
    n="$(basename "$d")"
    check "dry run lists skill $n" grep -qx "  - $n" <<< "$skills_plan"
  fi
done
echo "every agent syncs"
agents_plan="$("$AGENTS_SH" --target /tmp/_cov --dry-run 2>/dev/null)"
for f in "$AGENTS_DIR"/*.md; do
  n="$(basename "$f" .md)"
  check "dry run lists agent $n" grep -qx "  - $n" <<< "$agents_plan"
done

echo "guard fails closed (with a diagnostic) — a skill whose name: is not its directory"
tmpskill="$SKILLS_DIR/__cov_test_mismatch__"
TMP_FIXTURES+=("$tmpskill")
mkdir -p "$tmpskill"
printf -- '---\nname: something-else\ndescription: "Use when testing."\n---\n' > "$tmpskill/SKILL.md"
check_fail_msg "skills --verify FAILS + names the mismatch" "does not match its directory" "$SKILLS_SH" --verify
check_fail     "sync refuses to run with an inconsistent catalog" "$SKILLS_SH" --target /tmp/_cov --dry-run
rm -rf "$tmpskill"
check          "guard recovers after the fixture is removed"     "$SKILLS_SH" --verify

echo "guard fails closed (with a diagnostic) — a skill with no name:"
tmpskill2="$SKILLS_DIR/__cov_test_noname__"
TMP_FIXTURES+=("$tmpskill2")
mkdir -p "$tmpskill2"
printf -- '---\ndescription: "Use when testing."\n---\n' > "$tmpskill2/SKILL.md"
# Must EXIT non-zero AND print the message — not crash silently under
# set -e/pipefail (which would satisfy a bare exit check).
check_fail_msg "skills --verify FAILS + reports the missing name" "no 'name:'" "$SKILLS_SH" --verify
rm -rf "$tmpskill2"

# After a pull, git keeps a removed skill's directory when it still holds
# ignored files. Such a directory has no SKILL.md, so it is not a skill.
echo "a leftover directory without a SKILL.md is not a skill"
tmpskill3="$SKILLS_DIR/__cov_test_leftover__"
TMP_FIXTURES+=("$tmpskill3")
mkdir -p "$tmpskill3/state"
printf 'run output\n' > "$tmpskill3/state/run.md"
check      "skills --verify still passes" "$SKILLS_SH" --verify
# Capture first: a dry run that fails with no output also greps empty, so a
# bare "grep finds nothing" proves nothing.
leftover_plan="$("$SKILLS_SH" --target /tmp/_cov --dry-run 2>/dev/null)"; rc=$?
real_skill=""
for d in "$SKILLS_DIR"/*/; do
  if [ -f "${d}SKILL.md" ]; then real_skill="$(basename "$d")"; break; fi
done
check_eq   "the dry run exits 0" "0" "$rc"
check      "the dry run still lists skill $real_skill" grep -qx "  - $real_skill" <<< "$leftover_plan"
check_fail "the dry run does not list it" grep -q "__cov_test_leftover__" <<< "$leftover_plan"
rm -rf "$tmpskill3"

echo "agent guard fails closed (with a diagnostic)"
tmpagent="$AGENTS_DIR/__cov_test_agent_mismatch__.md"
TMP_FIXTURES+=("$tmpagent")
printf -- '---\nname: something-else\ndescription: "Testing."\n---\n' > "$tmpagent"
check_fail_msg "agents --verify FAILS + names the mismatch" "does not match its file name" "$AGENTS_SH" --verify
check_fail     "agent sync refuses to run with an inconsistent catalog" "$AGENTS_SH" --target /tmp/_cov --dry-run
rm -f "$tmpagent"
tmpagent2="$AGENTS_DIR/__cov_test_agent_nodesc__.md"
TMP_FIXTURES+=("$tmpagent2")
printf -- '---\nname: __cov_test_agent_nodesc__\n---\n' > "$tmpagent2"
check_fail_msg "agents --verify FAILS + reports the missing description" "no 'description:'" "$AGENTS_SH" --verify
rm -f "$tmpagent2"
check          "agent guard recovers after the fixtures are removed" "$AGENTS_SH" --verify

# The README states counts in more than one place. A number transcribed rather
# than measured drifts, so assert it against the tree.
echo "README's declared catalog counts match the tree"
declared_skills=$(sed -n 's/.*the catalog | \([0-9]*\) skills, \([0-9]*\) agents.*/\1/p' "$REPO_ROOT/README.md")
declared_agents=$(sed -n 's/.*the catalog | \([0-9]*\) skills, \([0-9]*\) agents.*/\2/p' "$REPO_ROOT/README.md")
actual_skills=0
for d in "$REPO_ROOT/.claude/skills"/*/; do
  if [ -f "${d}SKILL.md" ]; then actual_skills=$((actual_skills + 1)); fi
done
actual_agents=$(find "$REPO_ROOT/.claude/agents" -maxdepth 1 -name '*.md' | wc -l | tr -d ' ')
# An empty declared_* means the README row moved or was reworded, which needs a
# different repair than a stale number. Separate the two messages.
if [ -z "$declared_skills" ] || [ -z "$declared_agents" ]; then
  no "README catalog-count row not found (the sed pattern no longer matches — did the row move?)"
else
check_eq "README skill count ($declared_skills) == tree ($actual_skills)" "$actual_skills" "$declared_skills"
check_eq "README agent count ($declared_agents) == tree ($actual_agents)" "$actual_agents" "$declared_agents"
fi

# The README states the skill count in more than one place, and the guard above
# reads one of them. Assert every count claim in the file, not just the table
# row. Only lines that name `.claude/` count, so a number elsewhere in the
# README cannot trip this.
bad_counts=$(grep -F '.claude/' "$REPO_ROOT/README.md" \
  | grep -oE '[0-9]+ (self-contained Claude Code )?skills' \
  | grep -oE '^[0-9]+' | sort -u | grep -v "^${actual_skills}$" | tr '\n' ' ')
check_eq "every README skill-count claim reads $actual_skills" "" "$bad_counts"

echo ""
echo "catalog-coverage: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
