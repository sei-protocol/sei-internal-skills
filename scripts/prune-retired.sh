#!/usr/bin/env bash
# prune-retired.sh — remove resources this repo has retired from a synced .claude/ tree.
#
# WHY THIS EXISTS: sync-skills.sh and sync-agents.sh never delete. That is deliberate —
# a target-only file is usually a user's own work, and a sync that pruned by
# difference would eat it. The cost is that RETIRING a resource does not un-install
# it. After the 2026-08 slim-down, every environment that had ever synced still
# carried the removed skills, and after the lingua->language rename every
# environment carried both. Claude Code discovers all of them, so a stale copy is
# not inert: it stays dispatchable and competes with the resource that replaced it.
#
# This script closes that gap, and it is the ONLY script here that deletes.
#
# TWO LISTS, DELIBERATELY DIFFERENT IN KIND:
#
#   RETIRED — hand-maintained below. These no longer exist in this repo under any
#             tier. Removing one is not reversible from here; it is recoverable
#             only from the archive snapshot. Hardcoded precisely because it must
#             be reviewed by a human in a diff, never inferred.
#
#   PARKED  — DERIVED from experimental/ at runtime, never hardcoded. These still
#             exist in the repo, so removing one is reversible: `make
#             sync-experimental` puts it back. Derived so it cannot drift; promote
#             a skill out of experimental/ and it drops off this list by itself.
#
# WHAT IT WILL NEVER TOUCH: anything in the current core, and anything it does not
# recognize. A skill it has never heard of is presumed to be yours — the user's own
# authored work, or a skill from somewhere else — and is reported, not removed.
# That guard is why the lists are explicit rather than "delete whatever is not in
# the source tree", which would delete exactly those.
#
# Usage:
#   prune-retired.sh [--target <path>] [--apply] [--retired-only] [--check]
#
# --target:        target directory (the script appends .claude/). Default: $HOME.
# --apply:         actually delete. WITHOUT THIS FLAG THE SCRIPT ONLY REPORTS.
# --retired-only:  prune the retired list only; leave the parked resources installed.
# --check:         print one hint line if anything is prunable, then exit 0. Silent
#                  when the environment is clean. `make update` calls this so a
#                  stale environment announces itself, without a routine sync ever
#                  deleting anything on its own.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# --- The retired list -------------------------------------------------------
# Each entry records where it went, so a future reader can recover it.

# Product explorations cut in the 2026-08 slim-down (#291). Full history lives in
# the private snapshot bdchatham/sei-internal-skills-archive.
RETIRED_SKILLS=(
  data-mesh    # data architecture / data mesh
  prfaq        # Amazon working-backwards PRFAQ
  tee          # trusted execution environments
  diagram      # house-grammar Lucid diagrams
  lingua       # RENAMED, not cut — superseded by `language` (#294)

  # The aggregators. Linear covers their job natively: Custom Views answer
  # "which projects are at risk / shipped last quarter / are mine", and Pulse
  # delivers daily-or-weekly project-update digests to the Inbox. A Project's
  # own Activity tracker is the per-project record. Scraping all of that into a
  # second surface was the job; the job is gone.
  impact-weekly     # weekly roll-up into a bet's Weekly log
  impact-portfolio  # cross-project weekly exec report page
  execution-plan    # bet<->design<->issue<->PR lineage decoration
)
RETIRED_AGENTS=(
  data-platform-architect   # backed by /data-mesh
  tee-specialist            # backed by /tee
  diagram-architect         # backed by /diagram
  technical-program-manager # backed by /execution-plan; the agent is a thin
                            # wrapper over that mechanism, so it retires with it
)

# --- Argument parsing -------------------------------------------------------

TARGET="$HOME"
APPLY=false
RETIRED_ONLY=false
CHECK=false

usage() {
  grep '^#' "$0" | sed 's/^# \{0,1\}//' | grep -v '^!'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target)       TARGET="$2"; shift 2 ;;
    --apply)        APPLY=true; shift ;;
    --retired-only) RETIRED_ONLY=true; shift ;;
    --check)        CHECK=true; shift ;;
    -h|--help)      usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

TARGET="${TARGET/#\~/$HOME}"
T_SKILLS="${TARGET%/}/.claude/skills"
T_AGENTS="${TARGET%/}/.claude/agents"

# --- Read the repo's current shape ------------------------------------------
# The core set is the safety guard: nothing in it is ever removable, no matter
# what any list says. A resource promoted out of experimental/ back into the core
# must survive a prune.

declare -a CORE_SKILLS=() CORE_AGENTS=() PARKED_SKILLS=() PARKED_AGENTS=()
while IFS= read -r d; do [ -n "$d" ] && CORE_SKILLS+=("$(basename "$d")"); done \
  < <(find "$REPO_ROOT/.claude/skills" -mindepth 1 -maxdepth 1 -type d | sort)
while IFS= read -r f; do [ -n "$f" ] && CORE_AGENTS+=("$(basename "$f" .md)"); done \
  < <(find "$REPO_ROOT/.claude/agents" -maxdepth 1 -type f -name '*.md' | sort)

# Always built, even under --retired-only. The flag decides what gets DELETED, never
# what gets RECOGNIZED: a parked resource reported as "not from this repo" is
# indistinguishable from the user's own work, and telling those two apart is the
# entire value of this report.
if [ -d "$REPO_ROOT/experimental" ]; then
  while IFS= read -r d; do [ -n "$d" ] && PARKED_SKILLS+=("$(basename "$d")"); done \
    < <(find "$REPO_ROOT/experimental/skills" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | sort)
  while IFS= read -r f; do [ -n "$f" ] && PARKED_AGENTS+=("$(basename "$f" .md)"); done \
    < <(find "$REPO_ROOT/experimental/agents" -maxdepth 1 -type f -name '*.md' 2>/dev/null | sort)
fi

# --- Classify what is installed ---------------------------------------------

declare -a DEL_RETIRED=() DEL_PARKED=() KEPT_CORE=() KEPT_UNKNOWN=() GUARDED=()

# Namerefs (local -n) need bash 4.2; macOS ships 3.2, and every other script here
# is 3.2-compatible. So the three relevant lists are flattened to space-delimited
# strings and matched by substring on padded boundaries.
in_str() { case " $2 " in *" $1 "*) return 0 ;; *) return 1 ;; esac; }

classify() {  # classify <name> <kind> <path> <core-str> <retired-str> <parked-str>
  local n="$1" kind="$2" path="$3" core="$4" retired="$5" parked="$6"

  # The guard runs FIRST and outranks every list. If a name is in the core, it
  # stays — even if some list below still names it. That is what makes a stale
  # entry in RETIRED_SKILLS a no-op rather than a data-loss bug.
  if in_str "$n" "$core"; then
    if in_str "$n" "$retired"; then GUARDED+=("$kind/$n"); else KEPT_CORE+=("$kind/$n"); fi
    return
  fi
  if in_str "$n" "$retired"; then DEL_RETIRED+=("$kind|$n|$path"); return; fi
  if in_str "$n" "$parked";  then DEL_PARKED+=("$kind|$n|$path");  return; fi
  KEPT_UNKNOWN+=("$kind/$n")
}

CORE_SK_STR=" ${CORE_SKILLS[*]-} "; CORE_AG_STR=" ${CORE_AGENTS[*]-} "
RET_SK_STR=" ${RETIRED_SKILLS[*]-} "; RET_AG_STR=" ${RETIRED_AGENTS[*]-} "
PARK_SK_STR=" ${PARKED_SKILLS[*]-} "; PARK_AG_STR=" ${PARKED_AGENTS[*]-} "

if [ -d "$T_SKILLS" ]; then
  while IFS= read -r d; do
    [ -n "$d" ] && classify "$(basename "$d")" skill "$d" "$CORE_SK_STR" "$RET_SK_STR" "$PARK_SK_STR"
  done < <(find "$T_SKILLS" -mindepth 1 -maxdepth 1 -type d | sort)
fi
if [ -d "$T_AGENTS" ]; then
  while IFS= read -r f; do
    [ -n "$f" ] && classify "$(basename "$f" .md)" agent "$f" "$CORE_AG_STR" "$RET_AG_STR" "$PARK_AG_STR"
  done < <(find "$T_AGENTS" -maxdepth 1 -type f -name '*.md' | sort)
fi

# --- Report -----------------------------------------------------------------

if $CHECK; then
  n=${#DEL_RETIRED[@]}
  $RETIRED_ONLY || n=$(( n + ${#DEL_PARKED[@]} ))
  if [ "$n" -gt 0 ]; then
    echo "→ ${n} retired/parked resource(s) still installed in ${TARGET%/}/.claude — run 'make prune-retired' to review"
  fi
  exit 0
fi

echo "Repo:   $REPO_ROOT"
echo "Target: ${TARGET%/}/.claude"
echo ""

# A record is kind|name|path. Name is field 2; path is everything after the last
# '|', so a path containing '|' still round-trips.
record_label() { local e="$1"; local rest="${e#*|}"; echo "${e%%|*}/${rest%%|*}"; }

show() {  # show <header> <entries...>
  local header="$1"; shift
  echo "$header ($#)"
  [ "$#" -eq 0 ] && { echo "  (none)"; return; }
  local e
  for e in "$@"; do echo "  - $(record_label "$e")"; done
}

if [ "${#DEL_RETIRED[@]}" -gt 0 ]; then
  show "RETIRED — gone from the repo, recoverable only from the archive" "${DEL_RETIRED[@]}"
else
  show "RETIRED — gone from the repo, recoverable only from the archive"
fi
echo ""
if $RETIRED_ONLY; then
  echo "PARKED — recognized but skipped (--retired-only): ${#DEL_PARKED[@]}"
  if [ "${#DEL_PARKED[@]}" -gt 0 ]; then
    for e in "${DEL_PARKED[@]}"; do echo "  - $(record_label "$e")"; done
  fi
elif [ "${#DEL_PARKED[@]}" -gt 0 ]; then
  show "PARKED — still in experimental/, restore with 'make sync-experimental'" "${DEL_PARKED[@]}"
else
  show "PARKED — still in experimental/, restore with 'make sync-experimental'"
fi
echo ""
echo "KEPT — current core: ${#KEPT_CORE[@]}"
echo "KEPT — not from this repo, left alone: ${#KEPT_UNKNOWN[@]}"
if [ "${#KEPT_UNKNOWN[@]}" -gt 0 ]; then printf '  - %s\n' "${KEPT_UNKNOWN[@]}"; fi
if [ "${#GUARDED[@]}" -gt 0 ]; then
  echo ""
  echo "GUARDED — named by the retired list but present in the core, so NOT removed:"
  printf '  - %s\n' "${GUARDED[@]}"
  echo "  Drop these from the retired list in $0 — the list has gone stale." >&2
fi

TOTAL=${#DEL_RETIRED[@]}
$RETIRED_ONLY || TOTAL=$(( TOTAL + ${#DEL_PARKED[@]} ))
echo ""
if [ "$TOTAL" -eq 0 ]; then
  echo "Nothing to prune."
  exit 0
fi

if ! $APPLY; then
  echo "$TOTAL resource(s) would be removed. This was a dry run — nothing was deleted."
  echo "Re-run with --apply to remove them."
  exit 0
fi

# --- Execute ----------------------------------------------------------------

REMOVED=0
remove_all() {
  local e path
  for e in "$@"; do
    path="${e##*|}"
    rm -rf -- "$path"
    echo "  removed $(record_label "$e")"
    REMOVED=$((REMOVED+1))
  done
}
[ "${#DEL_RETIRED[@]}" -gt 0 ] && remove_all "${DEL_RETIRED[@]}"
if ! $RETIRED_ONLY && [ "${#DEL_PARKED[@]}" -gt 0 ]; then remove_all "${DEL_PARKED[@]}"; fi

echo ""
echo "Removed: $REMOVED"
if [ "${#DEL_PARKED[@]}" -gt 0 ] && ! $RETIRED_ONLY; then
  echo "The parked ones are reversible: make sync-experimental"
fi
