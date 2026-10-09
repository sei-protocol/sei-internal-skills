#!/usr/bin/env bash
# verify-references.test.sh — regression suite for the citation gate.
#
# Each case pins a defect the gate shipped with, or would have.
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
  mkdir -p "$d/.claude/skills/alpha" "$d/.claude/agents"
  printf '%s' "$d"
}

echo "verify-references regression suite"

t=$(tree); printf '# alpha\n\nNothing cited here.\n' > "$t/.claude/skills/alpha/SKILL.md"
check      "clean tree exits 0" "$VERIFY" --target "$t" --quiet

t=$(tree); printf '# alpha\n\nUse `/nope`.\n' > "$t/.claude/skills/alpha/SKILL.md"
check_fail "an absent citation fails" "$VERIFY" --target "$t"

# A failure with zero output is indistinguishable from a crash, so the finding
# must name what it found.
t=$(tree); printf '# alpha\n\nUse `/nope`.\n' > "$t/.claude/skills/alpha/SKILL.md"
check      "an absent citation is reported by name" \
  bash -c '"$1" --target "$2" 2>&1 | grep -q "cites /nope"' _ "$VERIFY" "$t"

# After a pull, git keeps a removed skill's directory when it still holds
# ignored files. That directory has no SKILL.md, so a citation of it is ABSENT.
t=$(tree); mkdir -p "$t/.claude/skills/ghost/state"; printf 'x\n' > "$t/.claude/skills/ghost/state/x"
printf '# alpha\n\nUse `/ghost`.\n' > "$t/.claude/skills/alpha/SKILL.md"
check_fail "a leftover directory without a SKILL.md is not a skill" "$VERIFY" --target "$t"
check      "and the citation reports ABSENT" \
  bash -c '"$1" --target "$2" 2>&1 | grep -q "^ABSENT.*cites /ghost"' _ "$VERIFY" "$t"

# The run output in that leftover directory, or in a skill's own state/, ships
# nowhere, so a citation inside it is not a finding.
t=$(tree); printf '# alpha\n\nNothing cited here.\n' > "$t/.claude/skills/alpha/SKILL.md"
mkdir -p "$t/.claude/skills/ghost/state" "$t/.claude/skills/alpha/state"
printf "Ledger: dispatched \`/nope\`.\n" > "$t/.claude/skills/ghost/state/x.md"
printf "Ledger: dispatched \`/nope\`.\n" > "$t/.claude/skills/alpha/state/run-1.md"
check      "run output under state/ is not scanned" "$VERIFY" --target "$t" --quiet

# One marker must clear every name on the line. Ten lines in this corpus carry
# two or three, so a one-token hatch prints a remediation that cannot succeed.
t=$(tree)
printf '# alpha\n<!-- gap: /aaa /bbb — deferred -->\nUse `/aaa` and `/bbb`.\n' \
  > "$t/.claude/skills/alpha/SKILL.md"
check      "one marker clears several names on one line" "$VERIFY" --target "$t" --quiet

t=$(tree)
printf '# alpha\n<!-- gap: /alpha — stale -->\nUse `/alpha`.\n' \
  > "$t/.claude/skills/alpha/SKILL.md"
check_fail "a marker naming a held resource is stale" "$VERIFY" --target "$t"

t=$(tree)
printf '# alpha\n\nRun `scripts/ghost.sh`.\n' > "$t/.claude/skills/alpha/SKILL.md"
check_fail "a named script that exists nowhere fails" "$VERIFY" --target "$t"

check      "--installed never gates" bash -c '"$1" --installed --quiet' _ "$VERIFY"
# A CI runner has no installed tree. Exiting 2 there made this mode neither a
# pass nor a fail on the machine that runs it most.
check      "--installed on a machine with nothing synced still exits 0" \
  bash -c 'HOME="$(mktemp -d)" "$1" --installed --quiet' _ "$VERIFY"
check_fail "--target with no value is a usage error" "$VERIFY" --target

echo "  $pass passed, $fail failed"
[ "$fail" -eq 0 ]
