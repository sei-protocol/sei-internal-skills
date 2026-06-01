#!/usr/bin/env bash
# scan-yaml-cpu.sh — emit `<file>:<line>` for every cpu: set inside a limits: block.
#
# Usage: scan-yaml-cpu.sh <file1.yaml> [file2.yaml ...]
# Stdout: one line per finding. Empty stdout = no findings.
# Stderr: human-readable status; non-empty does not signal violation.
#
# Stateful awk: tracks indent + parent-block name. Prints every match (no
# early return) so multi-container files surface all violations. Skips
# Kustomization files (no container resources).

set -euo pipefail

for file in "$@"; do
  [[ -z "${file}" ]] && continue
  [[ ! -f "${file}" ]] && continue
  if grep -qx 'kind: Kustomization' "${file}" 2>/dev/null; then
    continue
  fi
  awk '
    function indent(s) { match(s, /^ */); return RLENGTH }
    {
      line = $0
      # Strip trailing comment
      sub(/[ \t]+#.*$/, "", line)
      if (line ~ /^[ \t]*limits:[ \t]*$/) {
        limits_indent = indent($0)
        in_limits = 1
        next
      }
      if (in_limits && indent($0) <= limits_indent && length(line) > 0) {
        in_limits = 0
      }
      if (in_limits && match(line, /^[ \t]+cpu:[ \t]*[^[:space:]]/)) {
        print FILENAME ":" NR
      }
    }
  ' "${file}"
done
