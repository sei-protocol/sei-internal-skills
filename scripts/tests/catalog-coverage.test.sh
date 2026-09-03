#!/usr/bin/env bash
# Regression suite for the frontmatter-derived sync + coverage guard
# (scripts/sync-skills.sh, scripts/sync-agents.sh).
# Run: scripts/tests/catalog-coverage.test.sh  (or `make verify-catalog` for the guard alone).
# Exits non-zero on any failure.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILLS_SH="$SCRIPT_DIR/../sync-skills.sh"
AGENTS_SH="$SCRIPT_DIR/../sync-agents.sh"
SKILLS_DIR="$SCRIPT_DIR/../../.claude/skills"
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

echo "coverage guard — the live catalog is complete"
check      "skills --verify passes on the real tree" "$SKILLS_SH" --verify
check      "agents --verify passes on the real tree" "$AGENTS_SH" --verify

echo "derivation — known skills land in the expected alias"
check      "gov-ops is in sei (the bug this fixes)"        bash -c "'$SKILLS_SH' --target /tmp/_cov --categories sei --dry-run 2>/dev/null | grep -q '^  - gov-ops$'"
check      "gov-ops is in all"                              bash -c "'$SKILLS_SH' --target /tmp/_cov --categories all --dry-run 2>/dev/null | grep -q '^  - gov-ops$'"
check      "idiomatic is in portable (code-quality→portable)" bash -c "'$SKILLS_SH' --target /tmp/_cov --categories portable --dry-run 2>/dev/null | grep -q '^  - idiomatic$'"
check      "root-cause is in portable (investigation→portable)" bash -c "'$SKILLS_SH' --target /tmp/_cov --categories portable --dry-run 2>/dev/null | grep -q '^  - root-cause$'"
# A parked skill is outside every alias. This is the experimental tier's contract
# stated where the derivation itself is under test — if `all` ever picks up a
# skill from experimental/, the tier has silently stopped being a tier.
check_fail "a parked skill (coral) is in no alias, not even all" bash -c "'$SKILLS_SH' --target /tmp/_cov --categories all --dry-run 2>/dev/null | grep -q '^  - coral$'"
check      "sei-network-specialist is in sei (name override)" bash -c "'$AGENTS_SH' --target /tmp/_cov --categories sei --dry-run 2>/dev/null | grep -q '^  - sei-network-specialist$'"
check_fail "sei-network-specialist is NOT in portable"        bash -c "'$AGENTS_SH' --target /tmp/_cov --categories portable --dry-run 2>/dev/null | grep -q '^  - sei-network-specialist$'"

echo "guard fails closed (with a diagnostic) — an orphaned category is rejected"
tmpskill="$SKILLS_DIR/__cov_test_orphan__"
TMP_FIXTURES+=("$tmpskill")
mkdir -p "$tmpskill"
printf -- '---\nname: cov-test\ncategory: nonexistent-domain-xyz\n---\n' > "$tmpskill/SKILL.md"
check_fail_msg "skills --verify FAILS + names the bad category" "maps to no sync alias" "$SKILLS_SH" --verify
check_fail     "sync refuses to run with an incomplete catalog" "$SKILLS_SH" --target /tmp/_cov --categories all --dry-run
rm -rf "$tmpskill"
check          "guard recovers after the orphan is removed"     "$SKILLS_SH" --verify

echo "guard fails closed (with a diagnostic) — a category-less skill is rejected"
tmpskill2="$SKILLS_DIR/__cov_test_nocat__"
TMP_FIXTURES+=("$tmpskill2")
mkdir -p "$tmpskill2"
printf -- '---\nname: cov-test2\n---\n' > "$tmpskill2/SKILL.md"
# D1 regression: must EXIT non-zero AND print the "no category" message — not
# crash silently under set -e/pipefail (which would satisfy a bare exit check).
check_fail_msg "skills --verify FAILS + reports the missing category" "no 'category:'" "$SKILLS_SH" --verify
rm -rf "$tmpskill2"

# The README's headline counts drifted three times during the 2026-08/09 slim,
# each time because the number was transcribed rather than measured. Assert it.
echo "README's declared core counts match the tree"
declared_skills=$(sed -n 's/.*the core | \([0-9]*\) skills, \([0-9]*\) agents.*/\1/p' "$REPO_ROOT/README.md")
declared_agents=$(sed -n 's/.*the core | \([0-9]*\) skills, \([0-9]*\) agents.*/\2/p' "$REPO_ROOT/README.md")
actual_skills=$(find "$REPO_ROOT/.claude/skills" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')
actual_agents=$(find "$REPO_ROOT/.claude/agents" -maxdepth 1 -name '*.md' | wc -l | tr -d ' ')
# An empty declared_* means the README row moved or was reworded, which needs a
# different repair than a stale number. Separate the two messages.
if [ -z "$declared_skills" ] || [ -z "$declared_agents" ]; then
  no "README core-count row not found (the sed pattern no longer matches — did the row move?)"
else
check_eq "README skill count ($declared_skills) == tree ($actual_skills)" "$actual_skills" "$declared_skills"
check_eq "README agent count ($declared_agents) == tree ($actual_agents)" "$actual_agents" "$declared_agents"
fi

# The README states the skill count in more than one place, and the guard above
# reads one of them. A stale "13 skills" survived in a second sentence because of
# exactly that. Assert every count claim in the file, not just the table row.
# Only lines that name `.claude/` — the experimental row (12) and the sentence
# about the prior 33-skill generation are both correct and must not trip this.
bad_counts=$(grep -F '.claude/' "$REPO_ROOT/README.md" \
  | grep -oE '[0-9]+ (self-contained Claude Code )?skills' \
  | grep -oE '^[0-9]+' | sort -u | grep -v "^${actual_skills}$" | tr '\n' ' ')
check_eq "every README skill-count claim reads $actual_skills" "" "$bad_counts"

echo ""
echo "catalog-coverage: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
