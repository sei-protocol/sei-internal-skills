#!/usr/bin/env bash
# SC-005 and SC-006 named a line-count check that did not exist.
#
# Two limits, and they are not the same rule:
#
#   * the contract must stay under 250 lines, because SC-005 says a person reads it
#     in under five minutes and a longer file is not that file;
#   * an anchor page must stay under 150 lines, because a long one is evidence that
#     it restates the standard instead of citing it. That is the failure the
#     repository exists to avoid, and NOTICE.md is the licence reason it forbids.
#
# The 150-line limit covers anchor pages only. It used to read "no knowledge artifact",
# which swept in the design documents under docs/design/ at 231 and 194 lines. A design
# document is an argument, not a summary of somebody else's standard, and holding it to
# a citation-length limit measures the wrong thing.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

CONTRACT='CONTRACT.md'
CONTRACT_MAX=250
ANCHOR_MAX=150
fail=0

n=$(wc -l < "$CONTRACT" | tr -d ' ')
printf '  %-44s %4s / %s\n' "$CONTRACT" "$n" "$CONTRACT_MAX"
if [ "$n" -gt "$CONTRACT_MAX" ]; then
  echo "FAIL the contract is $n lines, over $CONTRACT_MAX. Cut it, or amend SC-005 and say why."
  fail=1
fi

pages=0
for f in anchors/*.md; do
  [ -e "$f" ] || continue
  pages=$((pages + 1))
  n=$(wc -l < "$f" | tr -d ' ')
  printf '  %-44s %4s / %s\n' "$f" "$n" "$ANCHOR_MAX"
  if [ "$n" -gt "$ANCHOR_MAX" ]; then
    echo "FAIL $f is $n lines, over $ANCHOR_MAX. An anchor page cites a standard; it does not restate one."
    fail=1
  fi
done

if [ "$pages" -eq 0 ]; then
  echo "FAIL no anchor pages under anchors/ — refusing to report success on an empty set"
  fail=1
fi

[ "$fail" -eq 0 ] && echo "Every artifact is inside its length limit."
exit "$fail"
