#!/usr/bin/env bash
# sync-agents.sh — copy Tide agent definitions to a target .claude/agents/ directory.
#
# Usage:
#   sync-agents.sh --target <path> [--categories portable,sei,all] [--dry-run] [--force]
#
# --target:      target directory (the script appends .claude/agents/)
# --categories:  comma-separated list of categories (default: portable)
#                domains: platform-infra, observability, security, blockchain,
#                         product-management, release-operations
#                aliases: portable (all non-Sei agents), sei (sei-network-specialist), all
# --dry-run:     print what would be copied without copying
# --force:       overwrite existing target files without prompting
#
# Source of truth: the portable / sei lists below. Update the lists here
# when agents are added, renamed, or re-categorized.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENTS_DIR="$(cd "$SCRIPT_DIR/../.claude/agents" && pwd)"

# --- Category lists (source of truth) ---------------------------------------

# Domain categories — source-of-truth grouping (mirrors each agent's `category:`
# frontmatter and the AGENTS.md roster). Claude discovers agents FLAT under
# ~/.claude/agents/; these domains are metadata for humans + selective sync.
PLATFORM_INFRA=(
  kubernetes-specialist
  platform-engineer
  network-specialist
  k8s-capacity-management
  sei-network-specialist
)

OBSERVABILITY=(
  opentelemetry-expert
  observability-platform-engineer
  sre-engineer
)

SECURITY=(
  security-specialist
  tee-specialist
)

BLOCKCHAIN=(
  solidity-developer
)

CODE_QUALITY=(
  idiomatic-reviewer
  systems-engineer
)

PRODUCT_MANAGEMENT=(
  product-engineer
  product-manager
  go-to-market-specialist
)

RELEASE_OPERATIONS=(
  platform-release-manager
)

# Meta-aliases cross-cut the domains (back-compat). `sei` is the Sei-only slice
# (sei-network-specialist, which is also in the platform-infra DOMAIN); `portable`
# is everything else; `all` is every agent.
PORTABLE=(
  kubernetes-specialist
  platform-engineer
  network-specialist
  k8s-capacity-management
  opentelemetry-expert
  observability-platform-engineer
  sre-engineer
  security-specialist
  tee-specialist
  solidity-developer
  product-engineer
  product-manager
  go-to-market-specialist
  platform-release-manager
  idiomatic-reviewer
  systems-engineer
)

SEI=(
  sei-network-specialist
)

# --- Argument parsing -------------------------------------------------------

TARGET=""
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

if [[ -z "$TARGET" ]]; then
  echo "Error: --target is required" >&2
  usage
  exit 2
fi

# Expand ~ if present
TARGET="${TARGET/#\~/$HOME}"
TARGET_AGENTS="${TARGET%/}/.claude/agents"

# --- Build agent list from categories ---------------------------------------

declare -a AGENTS_TO_SYNC=()
IFS=',' read -ra CAT_ARRAY <<< "$CATEGORIES"
for cat in "${CAT_ARRAY[@]}"; do
  case "$cat" in
    platform-infra)      AGENTS_TO_SYNC+=("${PLATFORM_INFRA[@]}") ;;
    observability)       AGENTS_TO_SYNC+=("${OBSERVABILITY[@]}") ;;
    security)            AGENTS_TO_SYNC+=("${SECURITY[@]}") ;;
    blockchain)          AGENTS_TO_SYNC+=("${BLOCKCHAIN[@]}") ;;
    code-quality)        AGENTS_TO_SYNC+=("${CODE_QUALITY[@]}") ;;
    product-management)  AGENTS_TO_SYNC+=("${PRODUCT_MANAGEMENT[@]}") ;;
    release-operations)  AGENTS_TO_SYNC+=("${RELEASE_OPERATIONS[@]}") ;;
    portable)  AGENTS_TO_SYNC+=("${PORTABLE[@]}") ;;
    sei)       AGENTS_TO_SYNC+=("${SEI[@]}") ;;
    all)       AGENTS_TO_SYNC+=("${PORTABLE[@]}" "${SEI[@]}") ;;
    *)
      echo "Unknown category: $cat" >&2
      exit 2 ;;
  esac
done

# Deduplicate while preserving order (bash 3.2 compatible — no assoc arrays)
UNIQUE_LIST=$(printf '%s\n' "${AGENTS_TO_SYNC[@]}" | awk '!seen[$0]++')
# Read back into array
AGENTS_TO_SYNC=()
while IFS= read -r a; do
  [[ -n "$a" ]] && AGENTS_TO_SYNC+=("$a")
done <<< "$UNIQUE_LIST"

# --- Report plan ------------------------------------------------------------

echo "Source: $AGENTS_DIR"
echo "Target: $TARGET_AGENTS"
echo "Categories: $CATEGORIES"
echo "Agents to sync (${#AGENTS_TO_SYNC[@]}):"
printf '  - %s\n' "${AGENTS_TO_SYNC[@]}"

if $DRY_RUN; then
  echo ""
  echo "(dry-run — no files copied)"
  exit 0
fi

# --- Execute ----------------------------------------------------------------

mkdir -p "$TARGET_AGENTS"

COPIED=0
SKIPPED=0
CONFLICTS=0

for agent in "${AGENTS_TO_SYNC[@]}"; do
  src="$AGENTS_DIR/$agent.md"
  dst="$TARGET_AGENTS/$agent.md"

  if [[ ! -f "$src" ]]; then
    echo "  ! source missing, skipping: $src" >&2
    SKIPPED=$((SKIPPED+1))
    continue
  fi

  if [[ -f "$dst" ]]; then
    if cmp -s "$src" "$dst"; then
      # identical — nothing to do
      SKIPPED=$((SKIPPED+1))
      continue
    fi
    if ! $FORCE; then
      echo "  ! conflict (differs, use --force to overwrite): $dst" >&2
      CONFLICTS=$((CONFLICTS+1))
      continue
    fi
  fi

  cp "$src" "$dst"
  echo "  ✓ $agent"
  COPIED=$((COPIED+1))
done

echo ""
echo "Copied: $COPIED   Skipped (identical/missing): $SKIPPED   Conflicts: $CONFLICTS"

if [[ $CONFLICTS -gt 0 ]]; then
  echo "Re-run with --force to overwrite conflicting files." >&2
  exit 1
fi
