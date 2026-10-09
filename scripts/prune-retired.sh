#!/usr/bin/env bash
# prune-retired.sh — remove resources this repo has retired from a synced .claude/ tree.
#
# WHY THIS EXISTS: sync-skills.sh and sync-agents.sh never delete. That is deliberate —
# a target-only file is usually a user's own work, and a sync that pruned by
# difference would eat it. The cost is that RETIRING a resource does not un-install
# it. Claude Code discovers every installed copy, so a stale copy is not inert:
# it stays dispatchable and competes with the resource that replaced it.
#
# This script closes that gap, and it is the ONLY script here that deletes.
#
# RETIRED — hand-maintained below. These names no longer exist in this repo. The
# list is hardcoded, never inferred, so a human reviews every retirement in a
# diff. This script cannot restore a removed name; recover one from git history.
#
# WHAT IT WILL NEVER TOUCH: anything in the current core, and anything it does not
# recognize. A skill it has never heard of is presumed to be yours — the user's own
# authored work, or a skill from somewhere else — and is reported, not removed.
# That guard is why the lists are explicit rather than "delete whatever is not in
# the source tree", which would delete exactly those.
#
# Usage:
#   prune-retired.sh [--target <path>] [--apply] [--check]
#
# --target:        target directory (the script appends .claude/). Default: $HOME.
# --apply:         actually delete. WITHOUT THIS FLAG THE SCRIPT ONLY REPORTS.
# --check:         print one hint line if anything is prunable, then exit 0. Silent
#                  when the environment is clean. `make update` calls this so a
#                  stale environment announces itself, without a routine sync ever
#                  deleting anything on its own.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# --- The retired list -------------------------------------------------------
# Each entry records where it went, so a future reader can recover it.

# Recover any name below from this repository's git history:
#   git log --diff-filter=D -- .claude/skills/<name>
RETIRED_SKILLS=(
  audit-skill  # cut; its checker and rubric live in scripts/skill-package-checks.sh
               # and scripts/skill-package-rubric.md
  author-skill # cut with it: the authoring workflow around the same rubric
  brevity      # dissolved into the AGENTS.md Output discipline: BLUF, the verb rule,
               # the hedge rule, and the 4-line/20-line comment bounds
  pr-quality   # dissolved: its LLM judges are comment/prose discipline the contract
               # now states; its two mechanical scans belong to the platform repo
  chaos-suite  # declared 8 scripts and shipped none; its guardrails named a 9th.
               # The false-green, baseline and leaked-chaos knowledge moved into
               # validate-release before the cut.
  data-mesh    # data architecture / data mesh
  prfaq        # Amazon working-backwards PRFAQ
  tee          # trusted execution environments
  diagram      # house-grammar Lucid diagrams
  lingua       # RENAMED, not cut — superseded by `language` (#294)
  language     # the dual-audience premise did not hold up; its rules that did
               # are folded into the prose-steward agent, which no longer needs
               # a skill behind it

  # The aggregators. Linear covers their job natively: Custom Views answer
  # "which projects are at risk / shipped last quarter / are mine", and Pulse
  # delivers daily-or-weekly project-update digests to the Inbox. A Project's
  # own Activity tracker is the per-project record. Scraping all of that into a
  # second surface was the job; the job is gone.
  impact-weekly     # weekly roll-up into a bet's Weekly log
  impact-portfolio  # cross-project weekly exec report page
  execution-plan    # bet<->design<->issue<->PR lineage decoration

  # The experimental/ tier, removed whole. Recover one from git history:
  #   git log --diff-filter=D -- experimental/skills/<name>
  bugbash code-structure coral council design ebpf interview issue
  linear-ticket project-brief research workstream
)
RETIRED_AGENTS=(
  go-to-market-specialist  # no skill referenced it
  data-platform-architect   # backed by /data-mesh
  tee-specialist            # backed by /tee
  diagram-architect         # backed by /diagram
  technical-program-manager # backed by /execution-plan; the agent is a thin
                            # wrapper over that mechanism, so it retires with it

  sei-interview-expert   # experimental/agents/, removed whole with the tier
)

# --- Argument parsing -------------------------------------------------------

TARGET="$HOME"
APPLY=false
CHECK=false

usage() {
  grep '^#' "$0" | sed 's/^# \{0,1\}//' | grep -v '^!'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target)       TARGET="$2"; shift 2 ;;
    --apply)        APPLY=true; shift ;;
    --check)        CHECK=true; shift ;;
    -h|--help)      usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

TARGET="${TARGET/#\~/$HOME}"
T_SKILLS="${TARGET%/}/.claude/skills"
T_AGENTS="${TARGET%/}/.claude/agents"

# --- Read the repo's current shape ------------------------------------------
# The core set is the safety guard: a name in it is never removable. A skill is
# a directory that holds a SKILL.md.

declare -a CORE_SKILLS=() CORE_AGENTS=()
while IFS= read -r d; do
  if [ -n "$d" ] && [ -f "$d/SKILL.md" ]; then CORE_SKILLS+=("$(basename "$d")"); fi
done < <(find "$REPO_ROOT/.claude/skills" -mindepth 1 -maxdepth 1 -type d | sort)
while IFS= read -r f; do [ -n "$f" ] && CORE_AGENTS+=("$(basename "$f" .md)"); done \
  < <(find "$REPO_ROOT/.claude/agents" -maxdepth 1 -type f -name '*.md' | sort)

# --- Classify what is installed ---------------------------------------------

declare -a DEL_RETIRED=() KEPT_CORE=() KEPT_UNKNOWN=() GUARDED=()

# Namerefs (local -n) need bash 4.2; macOS ships 3.2, and every other script here
# is 3.2-compatible. So the two relevant lists are flattened to space-delimited
# strings and matched by substring on padded boundaries.
in_str() { case " $2 " in *" $1 "*) return 0 ;; *) return 1 ;; esac; }

classify() {  # classify <name> <kind> <path> <core-str> <retired-str>
  local n="$1" kind="$2" path="$3" core="$4" retired="$5"

  # The guard runs FIRST and outranks every list. If a name is in the core, it
  # stays — even if some list below still names it. That is what makes a stale
  # entry in RETIRED_SKILLS a no-op rather than a data-loss bug.
  if in_str "$n" "$core"; then
    if in_str "$n" "$retired"; then GUARDED+=("$kind/$n"); else KEPT_CORE+=("$kind/$n"); fi
    return
  fi
  if in_str "$n" "$retired"; then DEL_RETIRED+=("$kind|$n|$path"); return; fi
  KEPT_UNKNOWN+=("$kind/$n")
}

CORE_SK_STR=" ${CORE_SKILLS[*]-} "; CORE_AG_STR=" ${CORE_AGENTS[*]-} "
RET_SK_STR=" ${RETIRED_SKILLS[*]-} "; RET_AG_STR=" ${RETIRED_AGENTS[*]-} "

if [ -d "$T_SKILLS" ]; then
  while IFS= read -r d; do
    [ -n "$d" ] && classify "$(basename "$d")" skill "$d" "$CORE_SK_STR" "$RET_SK_STR"
  done < <(find "$T_SKILLS" -mindepth 1 -maxdepth 1 -type d | sort)
fi
if [ -d "$T_AGENTS" ]; then
  while IFS= read -r f; do
    [ -n "$f" ] && classify "$(basename "$f" .md)" agent "$f" "$CORE_AG_STR" "$RET_AG_STR"
  done < <(find "$T_AGENTS" -maxdepth 1 -type f -name '*.md' | sort)
fi

# --- Report -----------------------------------------------------------------

if $CHECK; then
  n=${#DEL_RETIRED[@]}
  if [ "$n" -gt 0 ]; then
    echo "→ ${n} retired resource(s) still installed in ${TARGET%/}/.claude — review with 'make -C ${REPO_ROOT} prune-retired', remove with 'make -C ${REPO_ROOT} prune-retired-apply'"
  fi
  exit 0
fi

echo "Repo:   $REPO_ROOT"
echo "Target: ${TARGET%/}/.claude"
echo ""

# A record is kind|name|path. Name is field 2; path is everything after the last
# '|', so a path containing '|' still round-trips.
record_label() { local e="$1"; local rest="${e#*|}"; echo "${e%%|*}/${rest%%|*}"; }

# has_state_files <path> — 0 if an installed skill's state/ holds any file other
# than .gitkeep. Such a file is usually the user's own run output (an audit log,
# a report), and --apply deletes it with the skill.
has_state_files() {
  [ -d "$1/state" ] || return 1
  [ -n "$(find "$1/state" -type f ! -name .gitkeep -print -quit 2>/dev/null)" ]
}

show() {  # show <header> <entries...>
  local header="$1"; shift
  echo "$header ($#)"
  [ "$#" -eq 0 ] && { echo "  (none)"; return; }
  local e suffix
  for e in "$@"; do
    suffix=""
    if [ "${e%%|*}" = skill ] && has_state_files "${e##*|}"; then
      suffix="  (has state/ files — copy what you need before --apply)"
    fi
    echo "  - $(record_label "$e")$suffix"
  done
}

if [ "${#DEL_RETIRED[@]}" -gt 0 ]; then
  show "RETIRED — no longer in this repository (recover from git history)" "${DEL_RETIRED[@]}"
else
  show "RETIRED — no longer in this repository (recover from git history)"
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
remove_all "${DEL_RETIRED[@]}"

echo ""
echo "Removed: $REMOVED"
