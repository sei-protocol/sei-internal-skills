#!/usr/bin/env bash
# The spec template is a fork of the one GitHub Spec Kit ships. Running the Spec
# Kit CLI rewrites its bundled assets, so a re-init or a CLI upgrade silently
# reverts a forked template and takes the deltas with it. This repository keeps
# the fork under writing/templates for that reason, away from any path the CLI
# writes.
#
# Nothing else would notice. The rules that depend on those headings would stop
# finding them, EARS-CriterionShall would check nothing, and the gate would go
# green — the same silent-no-op shape the review of that rule found three times.
#
# This asserts each delta is still present. It is a marker check, not a diff:
# the wording inside a section is free to change, the section is not.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

T="writing/templates/spec-template.md"
U="writing/templates/spec-template.upstream.md"
fail=0

[ -f "$T" ] || { echo "missing $T"; exit 1; }
[ -f "$U" ] || { echo "missing $U — the fork is no longer diffable"; exit 1; }

# THE HEADER COMMENT IS NOT THE TEMPLATE. `require 'SHALL'` matched the comment's
# own explanation of SHALL, and the Verifier pattern matched the example line
# inside it. Stripping every EARS criterion and every *Verifier:* from the body
# still passed. Everything from the first Markdown heading onward is the
# template; the block above it explains the template and cannot satisfy it.
# Strip a LEADING HTML comment, and only a leading one. Both templates carry
# `-->` further down inside their own examples, so cutting at the first one found
# anywhere would silently drop the top of a file that has no header. The fork has
# a header and upstream does not, which is why this takes a file argument and
# both sides go through it: an asymmetric extraction compares two different
# things and stops comparing the moment either file changes shape.
body() {
  awk '
    NR == 1 && $0 !~ /^<!--/ { passthrough = 1 }
    passthrough { print; next }
    started { print; next }
    /^-->[[:space:]]*$/ { started = 1 }
  ' "$1"
}

require() {  # $1 = grep -E pattern, $2 = why it exists
  if ! body "$T" | grep -qE "$1"; then
    echo "MISSING from $T: $1"
    echo "    why: $2"
    fail=1
  fi
}

require '^## Semantic Anchors'      'methods named once, or the body restates them'
require '^## Glossary'              'an agent reads linearly and cannot ask what a term means'
require '^## Boundary Context'      'a spec with no stated boundary grows while it is open'
require '^### Requirement [0-9]+:'  'requirements carry their own criteria, so none is an orphan'
require '^\*\*Objective:\*\*'       'names the beneficiary, not only the behaviour'
require '^\*\*Traces to:\*\*'       'every requirement points back at the story it serves'
require '^#### Acceptance Criteria' 'the heading EARS-CriterionShall keys on'
require 'SHALL'                     'EARS and RFC 2119 agree only on the uppercase spelling'
require '^\*Verifier:\*|^  \*Verifier:\*' 'a criterion nothing checks is a wish'

# COMPARE THE BODIES, NOT THE FILES. `cmp -s "$T" "$U"` could never fire: the
# fork carries a 28-line header the upstream copy does not, so the two differ
# from line one in every scenario including a total overwrite. It was dead code,
# and the workflow justified carrying the upstream copy on the grounds that this
# script diffs against it, which it did not.
#
# Stripping the fork header makes the comparison the one that matters. A re-init
# that overwrites the fork leaves a body identical to upstream's, which is
# exactly the reversion this file exists to catch.
if diff -q <(body "$T") <(body "$U") >/dev/null 2>&1; then
  echo "$T has the same body as upstream — the deltas are gone"
  fail=1
fi

[ "$fail" -eq 0 ] && echo "Spec template carries every delta."
exit "$fail"
