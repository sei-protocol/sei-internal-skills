#!/usr/bin/env bash
# Golden fixtures for the gate scripts.
#
# The rules had fixtures and the gates did not, so a gate could reject a correct
# document and nothing would notice. One did: check-verifiers.sh demanded the
# criterion layout specs/001 happens to use and rejected the one the template
# teaches, reporting that a compliant specification had no criteria at all.
#
# Each directory under evals/gates/<script>/ is a root the script runs against.
# expected.txt holds the exit code on its first line and the output after it.
set -uo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fail=0

for case_dir in "$root"/evals/gates/*/*/; do
  script="$(basename "$(dirname "$case_dir")")"
  name="$(basename "$case_dir")"
  expected_file="${case_dir}expected.txt"

  if [ ! -f "$expected_file" ]; then
    echo "FAIL $script/$name has no expected.txt — a case that asserts nothing cannot fail"
    fail=1
    continue
  fi

  actual="$("$root/scripts/$script.sh" "$case_dir" 2>&1)"
  rc=$?
  actual="exit: $rc
$actual"
  expected="$(cat "$expected_file")"

  if [ "$actual" = "$expected" ]; then
    echo "ok   $script/$name"
  else
    echo "FAIL $script/$name"
    diff <(printf '%s\n' "$expected") <(printf '%s\n' "$actual") | sed 's/^/       /'
    fail=1
  fi
done

# An if block, not `&&`. Appending to that line left the operator binding to the
# comment below it, so both summary lines printed on a failing run -- a harness
# announcing that every fixture matched while reporting that one did not.
if [ "$fail" -eq 0 ]; then
  # Named, not counted. "All gate fixtures match" is true and reads as "the gates
  # are tested"; only the gates with a directory here have a case at all.
  #
  # Anchored on $root like every other path in this file. Relative globs read the
  # caller's working directory, so the inventory would describe wherever the run
  # started from rather than the tree under test.
  covered="$(ls -d "$root"/evals/gates/*/ 2>/dev/null | xargs -n1 basename | sort | paste -sd', ' -)"
  total="$(ls "$root"/scripts/check-*.sh 2>/dev/null | wc -l | tr -d ' ')"
  echo "All gate fixtures match their golden files."
  echo "Covered: ${covered:-none} — of $total gates in scripts/. The rest have no case."
fi
exit "$fail"
