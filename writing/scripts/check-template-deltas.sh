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

require() {  # $1 = grep -E pattern, $2 = why it exists
  if ! grep -qE "$1" "$T"; then
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

if cmp -s "$T" "$U"; then
  echo "$T is identical to upstream — the deltas are gone"
  fail=1
fi

[ "$fail" -eq 0 ] && echo "Spec template carries every delta."
exit "$fail"
