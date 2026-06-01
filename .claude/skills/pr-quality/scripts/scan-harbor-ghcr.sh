#!/usr/bin/env bash
# scan-harbor-ghcr.sh — emit `<file>:<line>` for ghcr.io occurrences in
# added/modified lines under clusters/harbor/**.
#
# Usage: scan-harbor-ghcr.sh <unified-diff-file>
#        OR pipe a unified diff to stdin: gh pr diff <PR> | scan-harbor-ghcr.sh
# Stdout: one line per finding.
#
# State machine walks the unified diff: tracks current `+++ b/<path>` and
# `@@ ... +<line>,<count> @@` hunk headers, counts added lines (`+`) and
# context lines (` `) — match emits the path + post-hunk line number.

set -euo pipefail

DIFF_INPUT="${1:--}"
awk '
  /^\+\+\+ b\// {
    current_file = substr($0, 7)
    in_harbor = (current_file ~ /^clusters\/harbor\//) ? 1 : 0
    next
  }
  /^@@/ {
    # Match the +<line>,<count> portion of @@ -a,b +c,d @@
    match($0, /\+[0-9]+/)
    if (RSTART > 0) line_num = substr($0, RSTART+1, RLENGTH-1) + 0 - 1
    next
  }
  /^\+/ && !/^\+\+\+/ {
    line_num++
    if (in_harbor && /ghcr\.io/) print current_file ":" line_num
    next
  }
  /^[- ]/ && !/^---/ { line_num++ }
' "${DIFF_INPUT}"
