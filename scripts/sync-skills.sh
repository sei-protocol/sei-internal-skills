#!/usr/bin/env bash
# sync-skills.sh — copy Tide skill directories to a target .claude/skills/ directory.
#
# Sibling of sync-agents.sh. Tide is the canonical home; this pushes outward to
# user-scope (~/.claude/skills/) and other repos so they stay current.
#
# Daily flow (defaults):
#   ./scripts/sync-skills.sh
#   # equivalent to: ./scripts/sync-skills.sh --target ~ --categories portable
#
# Usage:
#   sync-skills.sh [--target <path>] [--categories portable,sei,all] [--dry-run] [--force]
#
# --target:      target directory (the script appends .claude/skills/). Default: $HOME.
# --categories:  comma-separated list of categories. Default: portable.
#                domains: workflow, workstream-bootstrap, hardening, investigation,
#                         skill-authoring, code-quality, writing-quality,
#                         product-management, project-management,
#                         release-operations, engineer-self-service
#                aliases: portable (workflow+workstream-bootstrap+hardening+
#                           investigation+skill-authoring+code-quality+
#                           writing-quality+product-management),
#                         sei (project-management+release-operations+engineer-self-service),
#                         all
#                (output-quality — brevity, pr-quality — is Tide-local, not synced)
# --dry-run:     print what would be copied without copying
# --force:       overwrite existing target skills without prompting
#
# Source of truth: the portable / sei lists below. Update these when skills
# are added, renamed, or re-categorized.
#
# Skills are directories (SKILL.md + references/ + ...), not single files. Sync
# uses cp -R, so target-only files are preserved (i.e. user customizations and
# runtime artifacts like council/workspace/ in the target tree are not deleted).
# If a tracked source file differs from its target counterpart, the skill is
# reported as a conflict and skipped unless --force is set.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILLS_DIR="$(cd "$SCRIPT_DIR/../.claude/skills" && pwd)"

# --- Category lists (source of truth) ---------------------------------------

# Domain categories — the source-of-truth grouping (mirrors each skill's
# `category:` frontmatter and the catalog README sections). Claude discovers
# skills FLAT under ~/.claude/skills/; these domains are metadata for humans +
# selective sync, NOT on-disk folders.
WORKFLOW=(
  coral
  council
  cross-review
  workstream
)

WORKSTREAM_BOOTSTRAP=(
  design
  issue
)

HARDENING=(
  bugbash
)

INVESTIGATION=(
  root-cause
)

SKILL_AUTHORING=(
  author-skill
  audit-skill
)

CODE_QUALITY=(
  idiomatic
  systems
)

WRITING_QUALITY=(
  lingua
)

PRODUCT_MANAGEMENT=(
  prfaq
)

PROJECT_MANAGEMENT=(
  execution-plan
  impact-weekly
  impact-portfolio
)

RELEASE_OPERATIONS=(
  chaos-suite
  validate-release
)

ENGINEER_SELF_SERVICE=(
  harbor-dev
)

# output-quality (brevity, pr-quality) is intentionally Tide-local — not synced.

# Meta-aliases cross-cut the domains (back-compat with the Makefile / muscle memory):
#   portable = workflow + workstream-bootstrap + hardening + investigation
#              + skill-authoring + code-quality + writing-quality + product-management
#   sei      = project-management + release-operations + engineer-self-service
#   all      = every domain above
# (output-quality is Tide-local — deliberately has no sync case below.)

# --- Argument parsing -------------------------------------------------------

TARGET="$HOME"
CATEGORIES="portable"
DRY_RUN=false
FORCE=false

usage() {
  grep '^#' "$0" | sed 's/^# \{0,1\}//' | grep -v '^!'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target)
      TARGET="$2"; shift 2 ;;
    --categories)
      CATEGORIES="$2"; shift 2 ;;
    --dry-run)
      DRY_RUN=true; shift ;;
    --force)
      FORCE=true; shift ;;
    -h|--help)
      usage; exit 0 ;;
    *)
      echo "Unknown argument: $1" >&2
      usage; exit 2 ;;
  esac
done

# Expand ~ if present
TARGET="${TARGET/#\~/$HOME}"
TARGET_SKILLS="${TARGET%/}/.claude/skills"

# --- Build skill list from categories ---------------------------------------

declare -a SKILLS_TO_SYNC=()
IFS=',' read -ra CAT_ARRAY <<< "$CATEGORIES"
for cat in "${CAT_ARRAY[@]}"; do
  case "$cat" in
    workflow)              SKILLS_TO_SYNC+=("${WORKFLOW[@]}") ;;
    workstream-bootstrap)  SKILLS_TO_SYNC+=("${WORKSTREAM_BOOTSTRAP[@]}") ;;
    hardening)             SKILLS_TO_SYNC+=("${HARDENING[@]}") ;;
    investigation)         SKILLS_TO_SYNC+=("${INVESTIGATION[@]}") ;;
    skill-authoring)       SKILLS_TO_SYNC+=("${SKILL_AUTHORING[@]}") ;;
    code-quality)          SKILLS_TO_SYNC+=("${CODE_QUALITY[@]}") ;;
    writing-quality)       SKILLS_TO_SYNC+=("${WRITING_QUALITY[@]}") ;;
    product-management)    SKILLS_TO_SYNC+=("${PRODUCT_MANAGEMENT[@]}") ;;
    project-management)    SKILLS_TO_SYNC+=("${PROJECT_MANAGEMENT[@]}") ;;
    release-operations)    SKILLS_TO_SYNC+=("${RELEASE_OPERATIONS[@]}") ;;
    engineer-self-service) SKILLS_TO_SYNC+=("${ENGINEER_SELF_SERVICE[@]}") ;;
    portable)  SKILLS_TO_SYNC+=("${WORKFLOW[@]}" "${WORKSTREAM_BOOTSTRAP[@]}" "${HARDENING[@]}" "${INVESTIGATION[@]}" "${SKILL_AUTHORING[@]}" "${CODE_QUALITY[@]}" "${WRITING_QUALITY[@]}" "${PRODUCT_MANAGEMENT[@]}") ;;
    sei)       SKILLS_TO_SYNC+=("${PROJECT_MANAGEMENT[@]}" "${RELEASE_OPERATIONS[@]}" "${ENGINEER_SELF_SERVICE[@]}") ;;
    all)       SKILLS_TO_SYNC+=("${WORKFLOW[@]}" "${WORKSTREAM_BOOTSTRAP[@]}" "${HARDENING[@]}" "${INVESTIGATION[@]}" "${SKILL_AUTHORING[@]}" "${CODE_QUALITY[@]}" "${WRITING_QUALITY[@]}" "${PRODUCT_MANAGEMENT[@]}" "${PROJECT_MANAGEMENT[@]}" "${RELEASE_OPERATIONS[@]}" "${ENGINEER_SELF_SERVICE[@]}") ;;
    output-quality)
      echo "output-quality (brevity, pr-quality) is a Tide-local domain — not synced. Edit it in Tide." >&2
      exit 2 ;;
    *)
      echo "Unknown category: $cat" >&2
      exit 2 ;;
  esac
done

# Deduplicate while preserving order (bash 3.2 compatible — no assoc arrays)
if [[ ${#SKILLS_TO_SYNC[@]} -gt 0 ]]; then
  UNIQUE_LIST=$(printf '%s\n' "${SKILLS_TO_SYNC[@]}" | awk '!seen[$0]++')
  SKILLS_TO_SYNC=()
  while IFS= read -r s; do
    [[ -n "$s" ]] && SKILLS_TO_SYNC+=("$s")
  done <<< "$UNIQUE_LIST"
fi

# --- Report plan ------------------------------------------------------------

echo "Source: $SKILLS_DIR"
echo "Target: $TARGET_SKILLS"
echo "Categories: $CATEGORIES"
echo "Skills to sync (${#SKILLS_TO_SYNC[@]}):"
if [[ ${#SKILLS_TO_SYNC[@]} -eq 0 ]]; then
  echo "  (none — selected categories are empty)"
else
  printf '  - %s\n' "${SKILLS_TO_SYNC[@]}"
fi

if $DRY_RUN; then
  echo ""
  echo "(dry-run — no files copied)"
  exit 0
fi

if [[ ${#SKILLS_TO_SYNC[@]} -eq 0 ]]; then
  exit 0
fi

# --- Execute ----------------------------------------------------------------

mkdir -p "$TARGET_SKILLS"

COPIED=0
IN_SYNC=0
MISSING=0
CONFLICTS=0

# Compare source-skill against target-skill at the file level.
# Returns 0 if every file in source is present and identical in target
# (target may have additional files; those are preserved by the cp -R sync).
# Returns 1 otherwise.
source_subset_of_target() {
  local src_dir="$1"
  local dst_dir="$2"
  [[ -d "$dst_dir" ]] || return 1
  while IFS= read -r -d '' src_file; do
    local rel="${src_file#"$src_dir"/}"
    local dst_file="$dst_dir/$rel"
    if [[ ! -f "$dst_file" ]] || ! cmp -s "$src_file" "$dst_file"; then
      return 1
    fi
  done < <(find "$src_dir" -type f -print0)
  return 0
}

for skill in "${SKILLS_TO_SYNC[@]}"; do
  src="$SKILLS_DIR/$skill"
  dst="$TARGET_SKILLS/$skill"

  if [[ ! -d "$src" ]]; then
    echo "  ! source missing, skipping: $src" >&2
    MISSING=$((MISSING+1))
    continue
  fi

  if [[ -d "$dst" ]]; then
    if source_subset_of_target "$src" "$dst"; then
      # source content already present in target — nothing to do
      IN_SYNC=$((IN_SYNC+1))
      continue
    fi
    if ! $FORCE; then
      echo "  ! conflict (target differs, use --force to overwrite): $dst" >&2
      CONFLICTS=$((CONFLICTS+1))
      continue
    fi
  fi

  # cp -R copies dir contents on top of existing dir; missing target dir is
  # created. Target-only files are preserved (cp does not delete).
  mkdir -p "$dst"
  cp -R "$src/." "$dst/"
  echo "  ✓ $skill"
  COPIED=$((COPIED+1))
done

echo ""
echo "Copied: $COPIED   In sync: $IN_SYNC   Source missing: $MISSING   Conflicts: $CONFLICTS"

if [[ $CONFLICTS -gt 0 ]]; then
  echo "Re-run with --force to overwrite conflicting skills." >&2
  exit 1
fi
