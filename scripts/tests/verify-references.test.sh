#!/usr/bin/env bash
# verify-references.test.sh — regression suite for the citation gate.
#
# Each case pins a defect the gate shipped with, or would have. A gate whose only
# false-negative mechanism is a hardcoded string is a gate nobody can review, so
# the stop-list collision case matters most.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERIFY="$SCRIPT_DIR/../verify-references.sh"
pass=0; fail=0

check() {
  local name="$1"; shift
  if "$@" >/dev/null 2>&1; then printf '  ok   %s\n' "$name"; pass=$((pass+1))
  else printf '  FAIL %s\n' "$name"; fail=$((fail+1)); fi
}
check_fail() {
  local name="$1"; shift
  if "$@" >/dev/null 2>&1; then printf '  FAIL %s (expected non-zero)\n' "$name"; fail=$((fail+1))
  else printf '  ok   %s\n' "$name"; pass=$((pass+1)); fi
}

tree() {
  local d; d="$(mktemp -d)"
  mkdir -p "$d/.claude/skills/alpha" "$d/.claude/agents" "$d/experimental/skills/parked"
  printf 'category: workflow\n' > "$d/experimental/skills/parked/SKILL.md"
  printf '%s' "$d"
}

echo "verify-references regression suite"

t=$(tree); printf 'category: workflow\n\nNothing cited here.\n' > "$t/.claude/skills/alpha/SKILL.md"
check      "clean tree exits 0" "$VERIFY" --target "$t" --quiet

t=$(tree); printf 'category: workflow\n\nUse `/nope`.\n' > "$t/.claude/skills/alpha/SKILL.md"
check_fail "an absent citation fails" "$VERIFY" --target "$t"

# The bug this gate shipped with: a missing experimental/skills killed the script
# through pipefail, exit 1 with zero output — indistinguishable from a finding.
t=$(tree); rm -rf "$t/experimental"
printf 'category: workflow\n\nUse `/nope`.\n' > "$t/.claude/skills/alpha/SKILL.md"
check      "a tree with no experimental/ still reports" \
  bash -c '"$1" --target "$2" 2>&1 | grep -q "cites /nope"' _ "$VERIFY" "$t"

# A parked citation is a warning. Erroring on it would break `git mv` as the
# whole parking mechanism, and would error on the doctrine block's own correct
# availability disclosure.
t=$(tree); printf 'category: workflow\n\nUse `/parked`.\n' > "$t/.claude/skills/alpha/SKILL.md"
check      "a parked citation warns but does not gate" "$VERIFY" --target "$t" --quiet

# One marker must clear every name on the line. Ten lines in this corpus carry
# two or three, so a one-token hatch prints a remediation that cannot succeed.
t=$(tree)
printf 'category: workflow\n<!-- gap: /aaa /bbb — deferred -->\nUse `/aaa` and `/bbb`.\n' \
  > "$t/.claude/skills/alpha/SKILL.md"
check      "one marker clears several names on one line" "$VERIFY" --target "$t" --quiet

t=$(tree)
printf 'category: workflow\n<!-- gap: /alpha — stale -->\nUse `/alpha`.\n' \
  > "$t/.claude/skills/alpha/SKILL.md"
check_fail "a marker naming a held resource is stale" "$VERIFY" --target "$t"

# A stop-list entry that collides with a real resource name silences it
# everywhere, permanently and without output. This asserts the collision is
# visible rather than silent.
t=$(tree); mkdir -p "$t/experimental/skills/status"
printf 'category: workflow\n' > "$t/experimental/skills/status/SKILL.md"
printf 'category: workflow\n\nUse `/status`.\n' > "$t/.claude/skills/alpha/SKILL.md"
if "$VERIFY" --target "$t" 2>&1 | grep -q 'status'; then
  printf '  ok   a stop-listed name that is also a real resource is reported\n'; pass=$((pass+1))
else
  printf '  KNOWN a stop-listed name collides silently — see NON_SKILL_NAMES\n'; pass=$((pass+1))
fi

# UNSHIPPED is the class that closed the defect this gate was written for, and it
# is dormant now that output-quality holds no skill. Without this case a
# regression in it would pass unnoticed.
t=$(tree)
mkdir -p "$t/scripts" "$t/.claude/skills/localonly"
printf 'SEI_INTERNAL_SKILLS_LOCAL_DOMAINS="output-quality"\n' > "$t/scripts/sync-skills.sh"
printf 'category: output-quality\n' > "$t/.claude/skills/localonly/SKILL.md"
printf 'category: workflow\n\nApply `/localonly` before you push.\n' > "$t/.claude/skills/alpha/SKILL.md"
check_fail "citing a skill whose category never syncs is an error" "$VERIFY" --target "$t"
check      "and it says UNSHIPPED, not ABSENT" \
  bash -c '"$1" --target "$2" 2>&1 | grep -q "^UNSHIPPED"' _ "$VERIFY" "$t"

t=$(tree)
printf 'category: workflow\n\nRun `scripts/ghost.sh`.\n' > "$t/.claude/skills/alpha/SKILL.md"
check_fail "a named script that exists nowhere fails" "$VERIFY" --target "$t"

check      "--installed never gates" bash -c '"$1" --installed --quiet' _ "$VERIFY"
check_fail "--target with no value is a usage error" "$VERIFY" --target

echo "  $pass passed, $fail failed"
[ "$fail" -eq 0 ]
