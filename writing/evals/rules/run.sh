#!/usr/bin/env bash
# Per-rule golden-file harness.
#
# Each directory under evals/rules/ isolates one rule: its own .vale.ini naming
# only that rule, a test.md carrying both conforming and non-conforming input,
# and expected.txt holding the exact output.
#
# Golden files beat "did rule X fire". They pin the line, the column and the
# message, so a rule that starts reporting the wrong span, or a message someone
# edited without thinking, fails the run.
#
# --no-global is not optional. Vale loads the user's global configuration IN
# ADDITION to a local one, so without it a machine with the toolkit installed
# gets extra findings and the golden file never matches.
#
# A DIRECTORY MISSING A PIECE IS A FAILURE, NEVER A SKIP. The loop used to pass
# over any directory with no test.md, so a fixture that lost its input still
# reported `ok` for every rule that read it, and the admission gate — which
# looked for expected.txt alone — went on calling the rule covered. An empty
# expected.txt fails for the same reason: every fixture here carries
# non-conforming input, so a golden asserting nothing has stopped asserting.
set -uo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
fail=0

for dir in "$root"/evals/rules/*/; do
  name="$(basename "$dir")"

  missing=""
  for piece in .vale.ini test.md expected.txt; do
    [ -f "${dir}${piece}" ] || missing="$missing $piece"
  done
  if [ -n "$missing" ]; then
    echo "FAIL $name is not a fixture: missing$missing"
    fail=1
    continue
  fi
  if [ ! -s "${dir}expected.txt" ]; then
    echo "FAIL $name has an empty expected.txt — a golden that asserts nothing cannot fail"
    fail=1
    continue
  fi
  actual="$(cd "$dir" && vale --no-global --no-exit --output=line . 2>&1 | sort)"
  expected="$(cat "${dir}expected.txt" 2>/dev/null || true)"

  if [ "$actual" = "$expected" ]; then
    echo "ok   $name"
  else
    echo "FAIL $name"
    diff <(printf '%s\n' "$expected") <(printf '%s\n' "$actual") | sed 's/^/       /'
    fail=1
  fi
done

[ "$fail" -eq 0 ] && echo "All rule fixtures match their golden files."
exit "$fail"
