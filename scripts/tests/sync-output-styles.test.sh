#!/usr/bin/env bash
# Regression suite for scripts/sync-output-styles.sh — the output-style syncer.
# Run: scripts/tests/sync-output-styles.test.sh  (or `make test-output-styles`). Exits non-zero on any failure.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SYNC="$SCRIPT_DIR/../sync-output-styles.sh"
STYLES_DIR="$SCRIPT_DIR/../../.claude/output-styles"

PASS=0
FAIL=0
ok() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
no() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }
silent() { "$@" >/dev/null 2>&1; }
check()      { local d="$1"; shift; if silent "$@"; then ok "$d"; else no "$d"; fi; }   # PASS when cmd exits 0
check_fail() { local d="$1"; shift; if silent "$@"; then no "$d"; else ok "$d"; fi; }   # PASS when cmd exits non-zero

scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT

# Emits the notice? The opt-in hint is the script's only user-facing contract
# beyond the copy itself, so assert on it directly.
prints_optin() { "$SYNC" --target "$1" 2>/dev/null | grep -q "NOT active"; }

echo "the shipped style is well-formed"
check "asd-ste100.md exists"        test -f "$STYLES_DIR/asd-ste100.md"
check "declares a name:"            grep -qE '^name: ASD-STE100$' "$STYLES_DIR/asd-ste100.md"
check "declares a description:"     grep -qE '^description: ' "$STYLES_DIR/asd-ste100.md"
check "keeps coding instructions"   grep -qE '^keep-coding-instructions: true$' "$STYLES_DIR/asd-ste100.md"

echo "dry-run reports the plan and writes nothing"
d="$scratch/dry"
check      "dry-run exits 0"          "$SYNC" --target "$d" --dry-run
check_fail "dry-run created no files" test -d "$d/.claude"

echo "fresh sync + idempotent re-run"
d="$scratch/fresh"
check "fresh sync exits 0"    "$SYNC" --target "$d"
check "style landed"          test -f "$d/.claude/output-styles/asd-ste100.md"
check "byte-identical to src" cmp -s "$STYLES_DIR/asd-ste100.md" "$d/.claude/output-styles/asd-ste100.md"
check "re-run exits 0"        "$SYNC" --target "$d"
check "re-run reports in-sync" bash -c "\"$SYNC\" --target \"$d\" | grep -q 'In sync: 1'"

echo "a user's own hand-written style survives a sync"
printf 'my own style\n' > "$d/.claude/output-styles/my-personal.md"
silent "$SYNC" --target "$d"
check "target-only file preserved" test -f "$d/.claude/output-styles/my-personal.md"

echo "a locally-edited style is a conflict, not a silent overwrite"
printf 'locally edited\n' >> "$d/.claude/output-styles/asd-ste100.md"
check_fail "conflict exits non-zero"     "$SYNC" --target "$d"
check      "local edit left intact"      grep -qF "locally edited" "$d/.claude/output-styles/asd-ste100.md"
check      "--force exits 0"             "$SYNC" --target "$d" --force
check      "--force restored the source" cmp -s "$STYLES_DIR/asd-ste100.md" "$d/.claude/output-styles/asd-ste100.md"

# The whole point of the script: it ships the file and leaves activation alone.
# If this ever regresses, `make update` silently rewrites how Claude talks to
# every user in every repo — the one outcome this design exists to prevent.
echo "sync NEVER activates a style"
d="$scratch/noactivate"
silent "$SYNC" --target "$d"
check_fail "no settings.json created"   test -f "$d/.claude/settings.json"
check      "prints the opt-in notice"   prints_optin "$d"

echo "the notice stops once the user has chosen a style"
d="$scratch/optedin"; mkdir -p "$d/.claude"
printf '{"outputStyle":"ASD-STE100"}\n' > "$d/.claude/settings.json"
silent "$SYNC" --target "$d"
check_fail "silent after opting in" prints_optin "$d"
check      "settings.json untouched" grep -qF '{"outputStyle":"ASD-STE100"}' "$d/.claude/settings.json"

d="$scratch/otherstyle"; mkdir -p "$d/.claude"
printf '{"outputStyle":"Explanatory"}\n' > "$d/.claude/settings.json"
silent "$SYNC" --target "$d"
check_fail "silent when another style is set" prints_optin "$d"
check      "other style not overwritten"      grep -qF 'Explanatory' "$d/.claude/settings.json"

echo "argument handling"
check      "--help exits 0"           "$SYNC" --help
check_fail "unknown arg exits non-zero" "$SYNC" --nonsense

echo ""
echo "sync-output-styles: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
