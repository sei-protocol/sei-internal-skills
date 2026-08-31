#!/usr/bin/env bash
# The spec template is a fork of the one GitHub Spec Kit ships. Running the Spec
# Kit CLI rewrites its bundled assets, so a re-init or a CLI upgrade silently
# reverts a forked template and takes the deltas with it. This repository keeps
# the fork under writing/templates for that reason, away from any path the CLI
# writes.
#
# Nothing else would notice. The rules that depend on those headings would stop
# finding them, EARS-CriterionShall would check nothing, and the gate would go
# green.
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

# THE HEADER COMMENT IS NOT THE TEMPLATE. Everything from the first Markdown
# heading onward is the template; the block above it explains the template and
# names every delta, so a marker matched inside the header is satisfied by prose
# about the template rather than by the template.
#
# Strip a LEADING HTML comment, and only a leading one. Both templates carry
# `-->` further down inside their own examples, so cutting at the first one found
# anywhere drops the top of a file that has no header. The fork has a header and
# upstream does not, so this takes a file argument and both sides go through it:
# an asymmetric extraction compares two different things.
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
# ANCHORED TO A CRITERION, NOT TO THE WORD. The paragraph above the criteria
# explains SHALL, so a bare `\bSHALL\b` is satisfied by that sentence on a body
# with every EARS line deleted. The delta is a numbered item carrying a SHALL
# clause, and the pattern says so.
require '^[0-9]+[.)] .*\bSHALL\b' 'EARS and RFC 2119 agree only on the uppercase spelling'
require '^\*Verifier:\*|^  \*Verifier:\*' 'a criterion nothing checks is a wish'

# COMPARE THE BODIES, NOT THE FILES. The fork carries a header the upstream copy
# does not, so `cmp -s "$T" "$U"` differs from line one in every scenario,
# a total overwrite included, and can never fire. Stripping the fork header makes
# the comparison the one that matters: a re-init that overwrites the fork leaves
# a body identical to upstream's, which is the reversion this file catches.
if diff -q <(body "$T") <(body "$U") >/dev/null 2>&1; then
  echo "$T has the same body as upstream — the deltas are gone"
  fail=1
fi

# THE TEMPLATE IS THE SPEC CONTRACT'S ONLY WORKED EXAMPLE, so the spec rules run
# against it. `.vale.ini` scopes them to `specs/**/spec.md`, which the template's
# own path does not match, so nothing else points them here.
#
# They run against the extracted body, for the reason the marker checks above use
# it. Those rules read `scope: raw` and the fork's header names every required
# heading, so Vale pointed at the file as it sits reports a clean template whose
# body has the delta deleted.
#
# The probe configuration is `.vale.ini` with the spec section re-scoped. It is
# generated rather than written out, so the rule list cannot drift from the one
# a real specification gets. It sits beside `.vale.ini` because StylesPath and
# Packages resolve relative to the configuration file.
if command -v vale >/dev/null 2>&1; then
  probe=".vale-template-probe.ini"
  scratch="$(mktemp -d)"
  trap 'rm -rf "$scratch" "$probe"' EXIT

  sed 's|^\[specs/\*\*/spec\.md\]|[**/spec-body.md]|' .vale.ini > "$probe"
  body "$T" > "$scratch/spec-body.md"

  # Errors only. The descriptive rules run at warning here and the placeholder
  # text a template carries is not prose anyone reads.
  # Report against $T. The scratch path is an implementation detail, and the
  # difference in line count is the header body() stripped, which is the offset
  # every reported line needs. Counting it beats matching the header's end
  # marker: body() also handles a file with no header, and there `-->` appears
  # only inside an example.
  offset=$(( $(wc -l < "$T") - $(wc -l < "$scratch/spec-body.md") ))

  # Judge on the output, and on the exit status separately. Vale exits non-zero
  # when it reports a finding, and also when it cannot load a configuration; the
  # second prints nothing, and a check that read only the text would pass.
  set +e
  out=$(vale --no-global --config="$probe" --minAlertLevel=error \
        --output=line "$scratch/spec-body.md")
  rc=$?
  set -e

  if [ -n "$out" ]; then
    echo "$out" | awk -F: -v f="$T" -v o="$offset" '{ $1 = f; $2 = $2 + o; print }' OFS=:
    echo "    why: the template is the spec contract's worked example and fails it"
    fail=1
  elif [ "$rc" -ne 0 ]; then
    echo "vale exited $rc and reported nothing — the spec rules did not run against $T"
    fail=1
  fi
else
  echo "NOTE: vale is not on PATH — the spec rules did not run against $T"
fi

[ "$fail" -eq 0 ] && echo "Spec template carries every delta, and passes the spec rules."
exit "$fail"
