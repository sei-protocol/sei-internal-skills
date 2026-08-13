#!/usr/bin/env bash
# Regression suite for the experimental/ tier — the guarantee that nothing in it
# reaches a teammate's environment unless they ask for it by name.
# Run: scripts/tests/experimental-isolation.test.sh  (or `make test-experimental`).
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$SCRIPT_DIR/../.." && pwd)"

PASS=0
FAIL=0
ok() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
no() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }
silent() { "$@" >/dev/null 2>&1; }
check()      { local d="$1"; shift; if silent "$@"; then ok "$d"; else no "$d"; fi; }
check_fail() { local d="$1"; shift; if silent "$@"; then no "$d"; else ok "$d"; fi; }

scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT

echo "the tier exists and is populated"
check "experimental/skills/ exists" test -d "$REPO/experimental/skills"
check "experimental/agents/ exists" test -d "$REPO/experimental/agents"
check "experimental/README.md exists" test -f "$REPO/experimental/README.md"

# The whole point. A default install is what `make sync-all` produces; if any
# experimental resource appears in it, the tier has stopped being a tier.
echo "a DEFAULT install contains no experimental resource"
t="$scratch/default"
silent "$REPO/scripts/sync-agents.sh" --target "$t" --categories all --force
silent "$REPO/scripts/sync-skills.sh" --target "$t" --categories all --force

leaked=0
for d in "$REPO"/experimental/skills/*/; do
  [ -d "$d" ] || continue
  n="$(basename "$d")"
  if [ -d "$t/.claude/skills/$n" ]; then echo "    leaked skill: $n"; leaked=1; fi
done
for f in "$REPO"/experimental/agents/*.md; do
  [ -f "$f" ] || continue
  n="$(basename "$f")"
  if [ -f "$t/.claude/agents/$n" ]; then echo "    leaked agent: $n"; leaked=1; fi
done
if [ "$leaked" -eq 0 ]; then ok "no experimental skill or agent in a default install"; else no "experimental resource leaked into a default install"; fi

# Exclusion must stay STRUCTURAL. If a sync script ever learns to read
# experimental/, the guarantee becomes a flag someone can flip by accident.
echo "the exclusion is structural, not a flag"
check_fail "sync-skills.sh does not read experimental/" grep -q "experimental" "$REPO/scripts/sync-skills.sh"
check_fail "sync-agents.sh does not read experimental/" grep -q "experimental" "$REPO/scripts/sync-agents.sh"
check_fail "sync-all does not invoke sync-experimental" bash -c "make -C '$REPO' -n sync-all 2>/dev/null | grep -q sync-experimental"
check_fail "bootstrap does not invoke sync-experimental" bash -c "make -C '$REPO' -n bootstrap 2>/dev/null | grep -q sync-experimental"
check_fail "update does not invoke sync-experimental"    bash -c "make -C '$REPO' -n update 2>/dev/null | grep -q sync-experimental"

echo "opting in by name does install them"
t="$scratch/optin"
check "sync-experimental.sh exits 0" "$REPO/scripts/sync-experimental.sh" --target "$t" --force
allthere=0
for d in "$REPO"/experimental/skills/*/; do
  [ -d "$d" ] || continue
  [ -d "$t/.claude/skills/$(basename "$d")" ] || allthere=1
done
for f in "$REPO"/experimental/agents/*.md; do
  [ -f "$f" ] || continue
  [ -f "$t/.claude/agents/$(basename "$f")" ] || allthere=1
done
if [ "$allthere" -eq 0 ]; then ok "every experimental resource installed on opt-in"; else no "opt-in install was incomplete"; fi

echo "dry-run writes nothing"
t="$scratch/dry"
check      "dry-run exits 0"          "$REPO/scripts/sync-experimental.sh" --target "$t" --dry-run
check_fail "dry-run created no files" test -d "$t/.claude"

echo "runtime state under experimental/ is gitignored"
check "experimental/skills/*/state/** ignored" grep -q 'experimental/skills/\*/state/\*\*' "$REPO/.gitignore"

echo ""
echo "experimental-isolation: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
