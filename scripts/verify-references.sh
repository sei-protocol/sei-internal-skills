#!/usr/bin/env bash
# verify-references.sh — fail if a shipped artifact cites a resource an engineer
# cannot reach.
#
# The defect presents as the AGENT misbehaving — "it told me to run /pr-quality
# and nothing happened" — not as an incomplete install, which is why it erodes
# confidence in the skills that do work. No other gate catches it: verify-catalog
# checks that a category maps to a sync alias, and verify-doctrine-block checks
# this repo's AGENTS.md against the source file. Neither resolves a cited name.
#
# Citation forms:
#   `/name`                a backticked slash-name in prose
#   .claude/skills/<name>  a path form
#
# Error classes — any one fails the run:
#   ABSENT          the resource is in neither tier
#   UNSHIPPED       the resource is in the core, but its category never syncs
#                   outward, so no default install places it. This is the class
#                   the opening paragraph names: /brevity and /pr-quality are
#                   category output-quality, which sync-skills.sh holds in
#                   SEI_INTERNAL_SKILLS_LOCAL_DOMAINS, and 14 shipped agents
#                   hard-path to them.
#   STALE-MARKER    a gap marker names a resource the core does hold
#   MISSING-SCRIPT  a SKILL.md names a script that exists nowhere
#
# Warning class — reported, never fails the run:
#   PARKED          the resource lives in experimental/
#
# PARKED is a warning rather than an error on purpose. The doctrine block names
# every parked skill in one sentence that states the tier, names the install
# command, and forbids assuming availability. That sentence is the correct
# authoring pattern, and a gate that errors on the right answer teaches authors
# to delete it. Parking a skill also stays a single `git mv`, which is the
# property experimental/README.md calls the whole mechanism.
#
# Escape hatch, deliberately narrow. Put the marker on the line ABOVE:
#
#   <!-- gap: /code-review — deferred; un-defer on the first correctness defect -->
#
# It exempts one name. A line citing several unresolved names needs one marker
# line per name, or a rewrite.
#
# Usage:
#   verify-references.sh [--target <path>] [--installed] [--quiet]
#
# --target <path>   tree to check (default: this repository)
# --installed       read <path> as an installed ~/.claude tree rather than a
#                   repository. Diagnostic, not a gate: an installed tree is
#                   per-machine, drifts, and is not this repository's to fix, so
#                   this mode always exits 0. It is the only scope that sees what
#                   an engineer's machine actually holds.
# --quiet           suppress the success line
#
# Exit codes:
#   0  no error-class finding (or --installed, which never gates)
#   1  at least one ABSENT, UNSHIPPED, STALE-MARKER or MISSING-SCRIPT finding
#   2  invalid usage

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TARGET="$REPO_ROOT"
QUIET=false
INSTALLED=false
TARGET_SET=false

usage() { grep '^#' "$0" | sed 's/^# \{0,1\}//' | grep -v '^!'; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target)
      [[ ${2-} ]] || { echo "Error: --target needs a path" >&2; usage; exit 2; }
      TARGET="${2/#\~/$HOME}"; TARGET="${TARGET%/}"; TARGET_SET=true; shift 2 ;;
    --installed) INSTALLED=true; shift ;;
    --quiet)     QUIET=true; shift ;;
    -h|--help)   usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

# --installed changes the layout assumption only. --target stays the one tree
# selector, so the two compose instead of overwriting each other.
if $INSTALLED; then
  $TARGET_SET || TARGET="$HOME"
  TARGET="$TARGET/.claude"
fi

if $INSTALLED; then
  [[ -d "$TARGET/skills" ]] || { echo "no skills/ under $TARGET — nothing synced here" >&2; exit 2; }
else
  [[ -d "$TARGET/.claude/skills" ]] || { echo "no .claude/skills under $TARGET" >&2; exit 2; }
fi

# A backticked /name that is not a skill citation. Kept short and explicit: every
# entry is a real thing this repo's prose names, and a long list here would be a
# way to make the gate pass rather than a way to make it correct.
#
# Every entry names a real thing this repo's prose carries, so a reader can check
# it. The list stays short on purpose: growing it is the easy way to make this
# gate pass rather than to make it correct, so an addition is a reviewed act.
#
#   proc metrics status health   HTTP and probe endpoints
#   websocket block block_results lag_status genesis   CometBFT RPC routes
#   oauth token v1 auth          server routes this repo documents
#   opt home tmp usr etc var dev bin mnt   absolute paths in image and deploy prose
NON_SKILL_NAMES=" proc metrics status health genesis websocket block block_results lag_status oauth token v1 auth opt home tmp usr etc var dev bin mnt "

cd "$TARGET"

if $INSTALLED; then
  # An installed tree is flat and has no tiers: a resource is present or it is
  # not. So parked is empty and every unresolved citation reports ABSENT, which
  # is the truth an engineer experiences.
  SKILL_ROOT="skills"; AGENT_ROOT="agents"; DOCTRINE=""
else
  SKILL_ROOT=".claude/skills"; AGENT_ROOT=".claude/agents"; DOCTRINE="scripts/sei-internal-skills-doctrine.md"
fi

# `|| true` guards the whole pipeline, not just the message. `find` on a missing
# directory exits 1, pipefail promotes it, and set -e kills the script with zero
# output — indistinguishable from a legitimate failure. A consuming repo has no
# experimental/skills, so that is the documented --target mode.
list_dirs() { { find "$1" -mindepth 1 -maxdepth 1 -type d -exec basename {} \; 2>/dev/null || true; } | sort | tr '\n' ' '; }

core=" $(list_dirs "$SKILL_ROOT") "
if $INSTALLED; then parked=" "; else parked=" $(list_dirs experimental/skills) "; fi

# A core skill whose category never syncs outward reaches no machine, so citing
# it is the same defect as citing something absent. The domain list is read from
# sync-skills.sh rather than restated, so the two cannot drift.
unshipped=" "
if ! $INSTALLED && [ -f scripts/sync-skills.sh ]; then
  local_domains=$(sed -n 's/^SEI_INTERNAL_SKILLS_LOCAL_DOMAINS="\(.*\)"$/\1/p' scripts/sync-skills.sh)
  for d in $SKILL_ROOT/*/; do
    [ -f "${d}SKILL.md" ] || continue
    cat=$(sed -n 's/^category:[[:space:]]*//p' "${d}SKILL.md" | head -1)
    for ld in $local_domains; do
      [ "$cat" = "$ld" ] && unshipped="$unshipped$(basename "$d") "
    done
  done
fi

# One awk pass over every shipped artifact. A per-line grep subshell is the
# obvious shape and it is unusable: two subprocesses per line across this tree is
# tens of thousands of spawns.
#
# The doctrine block is included because it renders into every consuming repo's
# AGENTS.md, so a dangling name there propagates outward.
#
# shellcheck disable=SC2016  # the awk program is single-quoted on purpose
AWK_PROG='
    FNR == 1 { prev = "" }
    {
      marker = " "
      if (match(prev, /<!--[ \t]*gap:/)) {
        tail = substr(prev, RSTART, RLENGTH + 400)
        while (match(tail, /\/[a-z][a-z0-9-]{2,30}/)) {
          nm = substr(tail, RSTART + 1, RLENGTH - 1); tail = substr(tail, RSTART + RLENGTH)
          marker = marker nm " "
          if (index(core, " " nm " ") && !index(unshipped, " " nm " "))
            printf "STALE-MARKER   %s:%d  marks /%s as a gap, but the core holds it\n", FILENAME, FNR-1, nm
        }
      }
      line = $0
      for (pass = 1; pass <= 2; pass++) {
        rest = line
        re = (pass == 1) ? "`\\/[a-z][a-z0-9-]{2,30}`" : "\\.claude\\/skills\\/[a-z][a-z0-9-]{2,30}"
        while (match(rest, re)) {
          tok = substr(rest, RSTART, RLENGTH); rest = substr(rest, RSTART + RLENGTH)
          gsub(/[`]/, "", tok); sub(/^.*\//, "", tok)
          if (index(stop, " " tok " ")) continue
          if (index(marker, " " tok " ")) continue
          if (seen[FILENAME ":" FNR ":" tok]++) continue
          if (index(unshipped, " " tok " "))
            printf "UNSHIPPED      %s:%d  cites /%s, whose category never syncs outward, so no default install places it\n", FILENAME, FNR, tok
          else if (index(core, " " tok " ")) continue
          else if (index(parked, " " tok " "))
            printf "PARKED         %s:%d  cites /%s, which lives in experimental/ and installs only on opt-in\n", FILENAME, FNR, tok
          else
            printf "ABSENT         %s:%d  cites /%s, which exists in neither tier\n", FILENAME, FNR, tok
        }
      }
      prev = $0
    }'

scan=$(
  { find "$AGENT_ROOT" -type f -name '*.md' 2>/dev/null || true
    find "$SKILL_ROOT" -type f -name '*.md'
    # `|| true`: with an empty DOCTRINE this test is the group's last command, so
    # the group exits 1, pipefail fails the pipeline, and set -e kills the script
    # before it prints anything.
    { [ -n "$DOCTRINE" ] && [ -f "$DOCTRINE" ] && echo "$DOCTRINE"; } || true
  } | sort | tr '\n' '\0' | xargs -0 awk -v core="$core" -v parked="$parked" \
        -v unshipped="$unshipped" -v stop="$NON_SKILL_NAMES" "$AWK_PROG"
)

# One findings array, counted once. Two counting paths would be correct only
# while each emits exactly one line per finding.
findings=()
while IFS= read -r l; do [ -n "$l" ] && findings+=("$l"); done <<< "$scan"

# A skill that names a script it does not ship promises a procedure nobody wrote.
# A name may be the skill's own or one of the repository's; only a name that
# resolves in neither is a broken promise. In an installed tree there is no
# repository scripts/, so this check runs in repository scope only.
if ! $INSTALLED; then
  for d in "$SKILL_ROOT"/*/; do
    [ -f "${d}SKILL.md" ] || continue
    for sc in $(grep -oE '[a-z][a-z0-9-]*\.sh' "${d}SKILL.md" | sort -u); do
      [ -f "${d}scripts/$sc" ] && continue
      [ -f "scripts/$sc" ] && continue
      # Same escape hatch as a citation: a marker anywhere in the SKILL.md that
      # names this script records a deliberate gap.
      grep -q "gap:.*$sc" "${d}SKILL.md" && continue
      findings+=("MISSING-SCRIPT ${d}SKILL.md names $sc, which exists in neither ${d}scripts/ nor scripts/")
    done
  done
fi

# Canary. The scan leans on ERE interval syntax inside a regex awk builds from a
# string, and awk differs across platforms — BSD here, mawk on the runner. If that
# ever stops matching, `scan` is empty and this gate prints success while checking
# nothing. That is the worst failure this file has, so it is measured rather than
# assumed: a citation planted in a temp file must come back as a finding.
canary_dir=$(mktemp -d)
trap 'rm -rf "$canary_dir"' EXIT
printf 'Use `/zzz-canary-absent` here.\n' > "$canary_dir/canary.md"
if ! awk -v core=" " -v parked=" " -v unshipped=" " -v stop=" " "$AWK_PROG" "$canary_dir/canary.md" \
     | grep -q 'zzz-canary-absent'; then
  echo "✗ verify-references: the scanner did not detect a planted citation." >&2
  echo "  awk is not matching, so a passing run would mean nothing. Refusing to report." >&2
  exit 2
fi

errors=0; warnings=0
for l in "${findings[@]:-}"; do
  [ -n "$l" ] || continue
  printf '%s\n' "$l" >&2
  case "$l" in PARKED*) warnings=$((warnings+1)) ;; *) errors=$((errors+1)) ;; esac
done

if $INSTALLED; then
  # 12 installed skills come from elsewhere — the speckit family, brandon-*,
  # project-brief. This repository cannot close a finding in one of them, and a
  # report that can never reach zero is a report nobody reads. They are counted
  # separately so the number this repository owns stays legible.
  own=0; foreign=0
  for l in "${findings[@]:-}"; do
    [ -n "$l" ] || continue
    sk=$(printf '%s' "$l" | sed -n 's|.*skills/\([a-z0-9-]*\)/.*|\1|p')
    if [ -n "$sk" ] && [ ! -d "$REPO_ROOT/.claude/skills/$sk" ] && [ ! -d "$REPO_ROOT/experimental/skills/$sk" ]
      then foreign=$((foreign+1)); else own=$((own+1)); fi
  done
  $QUIET || echo "verify-references --installed: $own finding(s) in resources this repository ships, $foreign in resources it does not. Diagnostic only; never gates."
  exit 0
fi

if (( errors )); then
  { echo
    echo "✗ verify-references: $errors error(s), $warnings warning(s)."
    echo "Close the citation, or mark a deliberate gap on the line above it:"
    echo '  <!-- gap: /name — why, and what un-defers it -->'
  } >&2
  exit 1
fi

$QUIET || echo "✓ verify-references: no error-class finding ($warnings parked-citation warning(s))."
