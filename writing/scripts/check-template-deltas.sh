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
# This does three things. It asserts each delta is still present, as a marker
# check rather than a diff. Five of the ten checks read a section heading --
# Semantic Anchors, Glossary, Boundary Context, Requirement N, and Acceptance
# Criteria -- and prose under such a heading is free to change.
#
# The other five read text, and that text is not free. The Objective's
# `As a ... I want ... so that ...` sentence, the EARS criterion's
# `THE <actor> SHALL`, the `**Traces to:**` label, the `*Verifier:*` label, and a
# filled does-not-cover cell are each a delta CONTRACT.md states in that form.
# Rewording one is changing the delta, so change the contract in the same edit. It compares the fork's body against the pinned upstream
# baseline, which is the re-init reversion itself. It runs Vale's spec rules
# against that body, because the template is the spec contract's only worked
# example and .vale.ini scopes those rules to specs/**/spec.md.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

T="writing/templates/spec-template.md"
U="writing/templates/spec-template.upstream.md"
fail=0

[ -f "$T" ] || { echo "missing $T"; exit 1; }
[ -f "$U" ] || { echo "missing $U — the fork is no longer diffable"; exit 1; }

# THE BASELINE IS PINNED, BECAUSE THE REVERSION CHECK TRUSTS IT. That check fires
# only on byte-identical bodies, so one edited character in $U means the two can
# never match again and the comparison goes quiet for good -- no error, no
# output, exit 0. Pinning turns a silent disable into a failure.
#
# $U is github/spec-kit templates/spec-template.md at 756d632. Refreshing the
# baseline means replacing the file, updating this digest and the note at the
# head of the fork, and re-reading the delta table: upstream may have adopted a
# delta, which retires the row.
UPSTREAM_SHA256=3945437fc35cd30a5b2bf7beea680337c3516826d3efa5a6b92c4a7eca1ba28e
actual=$(shasum -a 256 "$U" 2>/dev/null | cut -d' ' -f1 \
         || sha256sum "$U" | cut -d' ' -f1)
if [ "$actual" != "$UPSTREAM_SHA256" ]; then
  echo "$U does not match its pinned digest"
  echo "    expected: $UPSTREAM_SHA256"
  echo "    actual:   $actual"
  echo "    why: the reversion check compares against this file, and an edited"
  echo "         baseline disables that check instead of failing it"
  exit 1
fi

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

# ONE PER SECTION, NOT ONE PER FILE. CONTRACT.md scopes five deltas to each
# requirement or each success criterion, and a file-wide check is satisfied by
# the first one. Delete all of them from Requirement 2 and such a check stays
# quiet: the body still differs from upstream, and Spec-AcceptanceCriteria and
# EARS-CriterionShall are presence-only.
#
# A count is not enough either. Requirement 1 carries five EARS criteria and
# Requirement 2 carries two, so a file-wide 7 clears a threshold of 2 with
# Requirement 2 stripped bare: the surplus in one section hides the absence in
# the next. This walks the sections and asks each one for itself.
#
# awk, not grep: POSIX awk has no \b, so a marker needing a word boundary spells
# it out. A section heading opens a block, and a block that reaches the next
# heading with no match is named.
#
# THE PATTERNS TRAVEL IN THE ENVIRONMENT, NOT IN -v. awk processes escapes in a
# -v assignment, so `\*Verifier:\*` arrives as `*Verifier:*` -- an ERE that opens
# with a repeat and is illegal. Reached directly awk aborts; reached behind a
# section guard that its own mangling already broke, awk never evaluates it, the
# loop opens no block, and the check passes having read nothing. ENVIRON does no
# escape processing, so a pattern arrives as written.
# NO INTERVAL EXPRESSIONS IN THESE PATTERNS. The BWK awk macOS shipped before
# 20200612 reads a brace as a literal, so a bounded repeat matches nothing there,
# no block closes on a peer heading, and every check silently widens to end of
# file. The alternations below need no interval support.
#
# $5 CLOSES THE BLOCK. Without it a block ends only at the next section start, so
# the last one runs to end of file and the check asserts "somewhere at or after
# the final heading" rather than "inside each section". Nothing leaks from the
# template today, but the gate would not notice a delta that drifted out of its
# requirement and into a later part of the document.
require_each() {  # $1 = section-start ERE, $2 = marker ERE, $3 = why, $4 = noun, $5 = block-end ERE
  # A file with no section satisfies the loop below vacuously. Assert the
  # sections exist before trusting an empty result.
  if ! body "$T" | grep -qE "$1"; then
    echo "MISSING from $T: no $4 matches $1"
    echo "    the section this delta is scoped to is not in the template"
    echo "    why: $3"
    fail=1
    return
  fi

  # An awk that dies prints nothing, which reads as "no misses". Take the exit
  # status separately, the way the Vale probe below does.
  set +e
  misses=$(body "$T" | START="$1" MARKER="$2" ENDS="$5" awk '
    $0 ~ ENVIRON["START"] {
      if (open && !hit) print name
      open = 1; hit = 0; name = $0; next
    }
    open && $0 ~ ENVIRON["ENDS"] { if (!hit) print name; open = 0; next }
    open && $0 ~ ENVIRON["MARKER"] { hit = 1 }
    END { if (open && !hit) print name }
  ')
  rc=$?
  set -e
  if [ "$rc" -ne 0 ]; then
    echo "awk exited $rc walking $1 in $T — the delta was not checked"
    fail=1
    return
  fi

  if [ -n "$misses" ]; then
    echo "$misses" | while IFS= read -r m; do
      echo "MISSING from $T, under: $m"
      echo "    marker: $2"
    done
    echo "    why: $3"
    fail=1
  fi
}

require '^## Semantic Anchors'      'methods named once, or the body restates them'
# THE FILLED CELL IS THE DELTA, NOT THE COLUMN HEADING. CONTRACT.md states this
# row as the block *with* a does-not-cover column, and Principle V is what makes
# the column load-bearing: an anchor is a hint, and the gap is the honest part.
# Spec-Anchors is presence-only on the section heading, and a marker reading
# `[Dd]oes not cover` matches the table header alone -- blank the third cell of
# every anchor and both still pass on a block that teaches no gap.
#
# EACH ANCHOR, NOT ONE OF THEM. CONTRACT.md reads "each with a does not cover
# column", and the template says the same in its own words. A check asking for
# one filled cell passes on a table where only the first anchor states its gap.
#
# So this reads every data row: a pipe line inside the block whose first cell is
# neither the column heading nor the separator. It names each row that states no
# gap, and a table with no data row at all.
#
# The field count is checked AFTER the row is counted, never before. A row can
# lose the gap column two ways -- the cell blanked, or the cell deleted -- and
# awk's split returns separators plus one, so a two-cell row yields four fields.
# Skipping on the count would drop the deleted-cell row before counting it, and
# a table of such rows would report nothing.
gaps=$(body "$T" | awk '
  /^## Semantic Anchors/ { open = 1; next }
  open && /^#[ \t]|^##[ \t]/ { open = 0 }
  open && /^\|/ {
    n = split($0, c, "|")
    first = (n >= 2) ? c[2] : ""
    gap   = (n >= 4) ? c[4] : ""
    gsub(/^[ \t]+|[ \t]+$/, "", first)
    gsub(/^[ \t]+|[ \t]+$/, "", gap)
    if (first == "Anchor" || first ~ /^:?-+:?$/) next
    rows++
    if (n < 5 || gap == "" || gap ~ /^:?-+:?$/)
      print (first == "" ? "(unnamed row)" : first)
  }
  # A sentinel, not an empty line: command substitution strips a trailing
  # newline, so an empty print leaves $gaps empty and the check passes.
  END { if (rows == 0) print "!NOROWS!" }
')
if [ -n "$gaps" ]; then
  echo "$gaps" | while IFS= read -r a; do
    if [ "$a" = "!NOROWS!" ]; then
      echo "MISSING from $T: the Semantic Anchors table carries no anchor row"
    else
      echo "MISSING from $T: anchor '$a' states no 'does not cover' entry"
    fi
  done
  echo "    why: Principle V — an anchor is a hint, and the gap is the honest part"
  fail=1
fi
require '^## Glossary'              'an agent reads linearly and cannot ask what a term means'
require '^## Boundary Context'      'a spec with no stated boundary grows while it is open'
require '^### Requirement [0-9]+:'  'requirements carry their own criteria, so none is an orphan'
require_each '^### Requirement [0-9]+:' '^\*\*Objective:\*\* As a .+, I want .+, so that .+' \
  'names the beneficiary, not only the behaviour' 'requirement' '^#[ \t]|^##[ \t]|^###[ \t]'
require_each '^### Requirement [0-9]+:' '^\*\*Traces to:\*\*'       'every requirement points back at the story it serves' 'requirement' '^#[ \t]|^##[ \t]|^###[ \t]'
require_each '^### Requirement [0-9]+:' '^#### Acceptance Criteria' 'the heading EARS-CriterionShall keys on' 'requirement' '^#[ \t]|^##[ \t]|^###[ \t]'
# THE ACTOR IS THE DELTA. CONTRACT.md states this row as EARS with a named actor,
# and its reason is that `System MUST` names nobody. A pattern asking only for
# SHALL passes on `1. WHEN [trigger], System SHALL [response].`, which is the
# shape the delta exists to replace; EARS-CriterionShall is a presence check for
# `shall` inside a list item and would not notice either.
#
# It is anchored to a numbered criterion for a second reason: the paragraph above
# the criteria explains SHALL, so a bare `\bSHALL\b` is satisfied by that
# sentence on a body with every EARS line deleted.
require_each '^### Requirement [0-9]+:' '^[0-9]+[.)] .*THE .+ SHALL( |$)' 'EARS with a named actor; System MUST names no actor' 'requirement' '^#[ \t]|^##[ \t]|^###[ \t]'
# PER CRITERION. CONTRACT.md scopes *Verifier:* to each success criterion, and a
# file-wide check passes on SC-001's while SC-002 has none. Spec-SuccessCriteria
# is presence-only on the heading, so nothing else would notice.
require_each '^- \*\*SC-[0-9]+\*\*' '\*Verifier:\*' \
  'a criterion nothing checks is a wish' 'success criterion' '^#+[ \t]'

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
# A GATE THAT CANNOT RUN ITS CHECK FAILS. It does not pass with a note: the exit
# status is what a caller reads, and a pass verdict for a run that checked
# nothing is the shape this whole file exists to stop. CI installs Vale four
# steps before this one; a fresh checkout does not, so say what to do.
if ! command -v vale >/dev/null 2>&1; then
  echo "vale is not on PATH, so the spec rules did not run against $T"
  echo "    install it: writing/README.md names the version CI pins"
  exit 1
fi

scratch="$(mktemp -d)"
# The probe is .vale.ini with the spec section re-scoped, generated rather than
# written out, so its rule list cannot drift from the one a real specification
# gets. It sits beside .vale.ini because StylesPath and Packages resolve relative
# to the configuration file.
#
# mktemp, not a fixed name: two runs against one checkout would otherwise race,
# and the first to finish would delete the configuration the second is reading.
# The X's trail and there is no .ini suffix, because BSD mktemp substitutes only
# a trailing run: `.vale-template-probe.XXXXXX.ini` creates that literal name on
# macOS and every concurrent run shares it. Vale reads whatever --config names.
probe="$(mktemp ./.vale-template-probe.XXXXXX)"
trap 'rm -rf "$scratch" "$probe"' EXIT

# ASSERT THE RE-SCOPE. sed copies its input through when the pattern misses, so
# a renamed or widened section leaves the spec rules scoped to specs/**/spec.md.
# The scratch body then matches [*.md] alone, where all five are off, and Vale
# reports nothing at rc 0 -- a probe degraded to a no-op that says it passed.
# The brace form counts: a section widened to reach the fixtures still scopes the
# spec rules to specs/**/spec.md, and the whole heading is replaced either way. What
# the guard is for is a heading that has gone or no longer names that scope at all.
section='^\[\{?specs/\*\*/spec\.md.*\]$'
if ! grep -Eq "$section" .vale.ini; then
  echo "no [specs/**/spec.md] section in .vale.ini — the probe cannot re-scope it"
  echo "    why: without the re-scope the spec rules never reach $T"
  exit 1
fi

sed -E "s|${section}|[**/spec-body.md]|" .vale.ini > "$probe"
body "$T" > "$scratch/spec-body.md"

# Report against $T. The scratch path is an implementation detail, and the
# difference in line count is the header body() stripped, which is the offset
# every reported line needs. Counting it beats matching the header's end marker:
# body() also handles a file with no header, and there `-->` appears only inside
# an example.
offset=$(( $(wc -l < "$T") - $(wc -l < "$scratch/spec-body.md") ))

# Judge on the output, and on the exit status separately. Vale exits non-zero
# when it reports a finding, and also when it cannot load a configuration; the
# second prints nothing, and a check that read only the text would pass.
#
# --minAlertLevel=error: the descriptive rules run at warning here, and the
# placeholder text a template carries is not prose anyone reads.
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

[ "$fail" -eq 0 ] && echo "Spec template carries every delta, and passes the spec rules."
exit "$fail"
