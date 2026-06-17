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

PASS=0
FAIL=0
ok() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
no() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }
silent() { "$@" >/dev/null 2>&1; }
check()      { local d="$1"; shift; if silent "$@"; then ok "$d"; else no "$d"; fi; }   # PASS when cmd exits 0
check_fail() { local d="$1"; shift; if silent "$@"; then no "$d"; else ok "$d"; fi; }   # PASS when cmd exits non-zero

echo "coverage guard — the live catalog is complete"
check      "skills --verify passes on the real tree" "$SKILLS_SH" --verify
check      "agents --verify passes on the real tree" "$AGENTS_SH" --verify

echo "derivation — known skills land in the expected alias"
check      "gov-ops is in sei (the bug this fixes)"        bash -c "'$SKILLS_SH' --target /tmp/_cov --categories sei --dry-run 2>/dev/null | grep -q '^  - gov-ops$'"
check      "gov-ops is in all"                              bash -c "'$SKILLS_SH' --target /tmp/_cov --categories all --dry-run 2>/dev/null | grep -q '^  - gov-ops$'"
check      "ebpf is in portable (performance→portable)"     bash -c "'$SKILLS_SH' --target /tmp/_cov --categories portable --dry-run 2>/dev/null | grep -q '^  - ebpf$'"
check_fail "tee is NOT synced (Tide-local security)"        bash -c "'$SKILLS_SH' --target /tmp/_cov --categories all --dry-run 2>/dev/null | grep -q '^  - tee$'"
check_fail "brevity is NOT synced (Tide-local output-quality)" bash -c "'$SKILLS_SH' --target /tmp/_cov --categories all --dry-run 2>/dev/null | grep -q '^  - brevity$'"
check      "sei-network-specialist is in sei (name override)" bash -c "'$AGENTS_SH' --target /tmp/_cov --categories sei --dry-run 2>/dev/null | grep -q '^  - sei-network-specialist$'"
check_fail "sei-network-specialist is NOT in portable"        bash -c "'$AGENTS_SH' --target /tmp/_cov --categories portable --dry-run 2>/dev/null | grep -q '^  - sei-network-specialist$'"

echo "guard fails closed — an orphaned category is rejected"
tmpskill="$SKILLS_DIR/__cov_test_orphan__"
mkdir -p "$tmpskill"
printf -- '---\nname: cov-test\ncategory: nonexistent-domain-xyz\n---\n' > "$tmpskill/SKILL.md"
check_fail "skills --verify FAILS with an unmapped category" "$SKILLS_SH" --verify
check_fail "sync refuses to run with an incomplete catalog"  "$SKILLS_SH" --target /tmp/_cov --categories all --dry-run
rm -rf "$tmpskill"
check      "guard recovers after the orphan is removed"      "$SKILLS_SH" --verify

echo "guard fails closed — a category-less skill is rejected"
tmpskill2="$SKILLS_DIR/__cov_test_nocat__"
mkdir -p "$tmpskill2"
printf -- '---\nname: cov-test2\n---\n' > "$tmpskill2/SKILL.md"
check_fail "skills --verify FAILS with no category" "$SKILLS_SH" --verify
rm -rf "$tmpskill2"

echo ""
echo "catalog-coverage: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
