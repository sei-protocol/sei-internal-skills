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

[ "$fail" -eq 0 ] && echo "All gate fixtures match their golden files."
exit "$fail"
