#!/usr/bin/env bash
# Regression suite for scripts/lib/inject-doctrine.sh — the doctrine-block injector.
# Run: scripts/tests/inject-doctrine.test.sh  (or `make test-doctrine`). Exits non-zero on any failure.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB="$SCRIPT_DIR/../lib/inject-doctrine.sh"
BODY="$SCRIPT_DIR/../tide-doctrine.md"

PASS=0
FAIL=0
ok() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
no() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }
silent() { "$@" >/dev/null 2>&1; }
check()      { local d="$1"; shift; if silent "$@"; then ok "$d"; else no "$d"; fi; }   # PASS when cmd exits 0
check_fail() { local d="$1"; shift; if silent "$@"; then no "$d"; else ok "$d"; fi; }   # PASS when cmd exits non-zero

# shellcheck source-path=SCRIPTDIR
# shellcheck source=../lib/inject-doctrine.sh
. "$LIB"

scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT

# A `set -e` caller is the real production context (sync-agents.sh / sync-skills.sh
# both run `set -euo pipefail`); run a snippet under it.
run_under_set_e() {
  local body="$1" runner
  runner="$(mktemp)"
  printf 'set -euo pipefail\n. "%s"\n%s\n' "$LIB" "$body" > "$runner"
  bash "$runner"
  local rc=$?
  rm -f "$runner"
  return "$rc"
}

echo "PLT-647 — existing blockless AGENTS.md + set -e caller, write → appends (no abort)"
d="$scratch/noblock"; mkdir -p "$d"
printf '# My Package\n\nhand-authored content the package owns.\n' > "$d/AGENTS.md"
check      "did not abort under set -e (PLT-647 regression)" run_under_set_e "inject_doctrine \"$d\" \"$BODY\" write"
check      "block appended"            grep -qF "Operating with Tide resources" "$d/AGENTS.md"
check      "package content preserved" grep -qF "hand-authored content the package owns." "$d/AGENTS.md"

echo "PLT-627 — malformed prior block (BEGIN, no END) is refused (not silently destructive)"
d="$scratch/malformed"; mkdir -p "$d"
{ printf 'keep\n\n'; printf '%s\n' "$DOCTRINE_BEGIN"; printf 'stale, no end\n'; } > "$d/AGENTS.md"
check_fail "malformed block refused (BEGIN != END guard)" inject_doctrine "$d" "$BODY" write
check      "malformed file left untouched"                grep -qF "keep" "$d/AGENTS.md"

echo "PLT-627 — fresh create + idempotent re-run"
d="$scratch/fresh"
silent inject_doctrine "$d" "$BODY" write
check "fresh create injects block" grep -qF "$DOCTRINE_BEGIN" "$d/AGENTS.md"
check "CLAUDE.md pointer added"    grep -qF "AGENTS.md](./AGENTS.md)" "$d/CLAUDE.md"
cp "$d/AGENTS.md" "$scratch/fresh.bak"
silent inject_doctrine "$d" "$BODY" write
check "re-run is byte-identical (idempotent)" cmp -s "$d/AGENTS.md" "$scratch/fresh.bak"

echo "PLT-646 — check mode: in-sync → 0, drift → non-zero, writes nothing"
d="$scratch/check"; mkdir -p "$d"
silent inject_doctrine "$d" "$BODY" write
check "in-sync → exit 0" inject_doctrine "$d" "$BODY" check
perl -0pi -e 's/Operating with Tide resources/Operating with Tide resources EDITED/' "$d/AGENTS.md"
check_fail "block drift → non-zero" inject_doctrine "$d" "$BODY" check
d="$scratch/check-ro"; mkdir -p "$d"
silent inject_doctrine "$d" "$BODY" check
check "check wrote nothing (read-only)" test -z "$(ls -A "$d")"

echo "shellcheck — the lib under test is clean"
if command -v shellcheck >/dev/null 2>&1; then
  check "lib shellcheck clean" shellcheck "$LIB"
else
  echo "  SKIP: shellcheck not installed"
fi

echo ""
echo "RESULT: $PASS passed, $FAIL failed"
[[ $FAIL -eq 0 ]]
