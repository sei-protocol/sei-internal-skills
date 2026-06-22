#!/usr/bin/env bash
# sync-skills.sh — copy Tide skill directories to a target .claude/skills/ directory.
#
# Sibling of sync-agents.sh. Tide is the canonical home; this pushes outward to
# user-scope (~/.claude/skills/) and other repos so they stay current.
#
# SINGLE SOURCE OF TRUTH: each skill's own `category:` SKILL.md frontmatter.
# Membership in a sync alias (portable / sei) is DERIVED from that category via
# the small domain->alias map below — there is no hand-maintained per-skill list
# to drift out of sync (the bug that silently orphaned gov-ops and ebpf). Adding
# a skill = drop the dir with a `category:` that maps to an alias; it syncs
# automatically. The coverage guard (--verify) fails closed if any skill's
# category maps to no alias, so a miscategorized skill is caught in CI, never
# silently dropped.
#
# Daily flow:
#   make update                     # from the Tide repo: pull + sync everything + verify
#   ./scripts/sync-skills.sh        # equivalent to: --target ~ --categories portable
#
# Usage:
#   sync-skills.sh [--target <path>] [--categories portable,sei,all,<domain>] \
#                  [--dry-run] [--force] [--verify] [--inject-doctrine]
#
# --target:      target directory (the script appends .claude/skills/). Default: $HOME.
# --categories:  comma-separated aliases or domains. Default: portable.
#                aliases: portable, sei, all
#                domains: any value a skill declares in `category:` (workflow,
#                         code-quality, performance, release-operations, …)
# --dry-run:     print what would be copied without copying
# --force:       overwrite existing target skills without prompting
# --verify:      run ONLY the coverage guard (every skill's category resolves to a
#                known alias) and exit non-zero on any gap. No copying. For CI.
# --inject-doctrine: also inject the Tide operating-doctrine managed block into
#                <target>/AGENTS.md (+ a CLAUDE.md pointer). Off by default;
#                intended for a consuming package, not user-scope ($HOME).
#
# To re-categorize a skill, edit its SKILL.md `category:` — not this script.
# To add/rename a DOMAIN or change which alias it belongs to, edit the
# domain->alias map below (the only hand-maintained categorization left).
#
# Skills are directories (SKILL.md + references/ + ...), not single files. Sync
# uses cp -R, so target-only files are preserved (user customizations and runtime
# artifacts like council/workspace/ in the target tree are not deleted). If a
# tracked source file differs from its target counterpart, the skill is reported
# as a conflict and skipped unless --force is set.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILLS_DIR="$(cd "$SCRIPT_DIR/../.claude/skills" && pwd)"

# shellcheck source=lib/inject-doctrine.sh
. "$SCRIPT_DIR/lib/inject-doctrine.sh"

# --- Domain -> alias map (the ONLY hand-maintained categorization) ----------
#
# Every domain a skill may declare in `category:` must appear in exactly one of
# the three lists below. The coverage guard enforces this. `all` = PORTABLE+SEI
# (Tide-local domains are deliberately never synced outward).

PORTABLE_DOMAINS="workflow workstream-bootstrap hardening investigation skill-authoring code-quality performance writing-quality product-management security"
SEI_DOMAINS="project-management release-operations engineer-self-service recruiting"
# Tide-local — deliberately NOT synced outward:
#   output-quality (brevity, pr-quality) — Tide-development meta-skills.
# NOTE: security (tee) is now PORTABLE — its kits are self-contained on public
# primary sources (vendor specs, RFCs, sei-chain); the research corpus relocated
# to bdchatham-designs (Design 05 / PLT-709) and is non-required provenance.
TIDE_LOCAL_DOMAINS="output-quality"

# --- small helpers ----------------------------------------------------------

# in_list <needle> <space-separated-haystack>
in_list() { case " $2 " in *" $1 "*) return 0 ;; *) return 1 ;; esac; }

# skill_category <skill-name> — its declared `category:` (empty if none).
# `grep || true` so a no-match (no category:) does NOT fail the pipeline and
# abort the caller under `set -e`/`pipefail` — the coverage guard must still
# print its "no category" diagnostic, not crash silently. `tr -d '\r'` strips a
# CRLF terminator (GNU sed's [[:space:]] doesn't, so it would otherwise leak
# into the category on Linux CI).
skill_category() {
  { grep -m1 '^category:' "$SKILLS_DIR/$1/SKILL.md" 2>/dev/null || true; } \
    | tr -d '\r' \
    | sed 's/^category:[[:space:]]*//; s/[[:space:]]*$//; s/^["'"'"']//; s/["'"'"']$//'
}

# alias_for_domain <domain> — echoes portable|sei|tide-local|UNKNOWN
alias_for_domain() {
  if   in_list "$1" "$PORTABLE_DOMAINS";   then echo portable
  elif in_list "$1" "$SEI_DOMAINS";        then echo sei
  elif in_list "$1" "$TIDE_LOCAL_DOMAINS"; then echo tide-local
  else echo UNKNOWN; fi
}

# all skill dirs (those containing a SKILL.md), one per line, sorted
list_skill_dirs() {
  for d in "$SKILLS_DIR"/*/; do
    [ -f "${d}SKILL.md" ] && basename "$d"
  done | sort
}

# --- coverage guard ---------------------------------------------------------
# Fail closed if any skill lacks a category or its category maps to no alias.
run_coverage_guard() {
  local errs=0 name cat al
  while IFS= read -r name; do
    cat="$(skill_category "$name")"
    if [ -z "$cat" ]; then
      echo "  ✗ $name: no 'category:' in SKILL.md frontmatter" >&2
      errs=$((errs+1)); continue
    fi
    al="$(alias_for_domain "$cat")"
    if [ "$al" = "UNKNOWN" ]; then
      echo "  ✗ $name: category '$cat' maps to no sync alias — add '$cat' to PORTABLE_DOMAINS, SEI_DOMAINS, or TIDE_LOCAL_DOMAINS in sync-skills.sh" >&2
      errs=$((errs+1))
    fi
  done < <(list_skill_dirs)
  if [ "$errs" -gt 0 ]; then
    echo "skill catalog coverage: $errs problem(s) — every skill's category must map to an alias." >&2
    return 1
  fi
  echo "skill catalog coverage ✓ (every skill's category resolves to portable/sei/tide-local)"
  return 0
}

# --- Argument parsing -------------------------------------------------------

TARGET="$HOME"
CATEGORIES="portable"
DRY_RUN=false
FORCE=false
VERIFY=false
INJECT_DOCTRINE=false

usage() {
  grep '^#' "$0" | sed 's/^# \{0,1\}//' | grep -v '^!'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target)     TARGET="$2"; shift 2 ;;
    --categories) CATEGORIES="$2"; shift 2 ;;
    --dry-run)    DRY_RUN=true; shift ;;
    --force)      FORCE=true; shift ;;
    --verify)     VERIFY=true; shift ;;
    --inject-doctrine) INJECT_DOCTRINE=true; shift ;;
    -h|--help)    usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

# --verify: coverage guard only, no target needed.
if $VERIFY; then
  run_coverage_guard
  exit $?
fi

# Expand ~ if present
TARGET="${TARGET/#\~/$HOME}"
TARGET_SKILLS="${TARGET%/}/.claude/skills"

# Doctrine injection writes a managed block to the package root. Refuse $HOME.
if $INJECT_DOCTRINE && [[ "${TARGET%/}" == "${HOME%/}" ]]; then
  echo "Error: --inject-doctrine refuses \$HOME ($HOME) — target a package directory." >&2
  exit 2
fi

# Run the coverage guard first so a miscategorized skill fails loudly here too,
# not just in CI.
if ! run_coverage_guard >/dev/null 2>&1; then
  run_coverage_guard >&2 || true
  echo "Refusing to sync with an incomplete catalog (see above). Fix the category mapping first." >&2
  exit 1
fi

# --- Build skill list from requested categories -----------------------------
# A requested token is an alias (portable|sei|all) or a literal domain name.
# A skill is included if its category's alias — or the category itself — matches.
# Requesting a Tide-local domain by name is an explicit error.

TIDE_LOCAL_REQUESTED=false
want_skill() {  # want_skill <category> <requested-token> -> 0 include / 1 exclude
  local cat="$1" tok="$2" al
  al="$(alias_for_domain "$cat")"
  case "$tok" in
    all)      [ "$al" = "portable" ] || [ "$al" = "sei" ] ;;
    portable) [ "$al" = "portable" ] ;;
    sei)      [ "$al" = "sei" ] ;;
    output-quality)
      TIDE_LOCAL_REQUESTED=true; return 1 ;;
    *)        [ "$cat" = "$tok" ] ;;   # literal domain request
  esac
}

declare -a SKILLS_TO_SYNC=()
while IFS= read -r name; do
  cat="$(skill_category "$name")"
  IFS=',' read -ra TOKENS <<< "$CATEGORIES"
  for tok in "${TOKENS[@]}"; do
    if want_skill "$cat" "$tok"; then SKILLS_TO_SYNC+=("$name"); break; fi
  done
done < <(list_skill_dirs)

if $TIDE_LOCAL_REQUESTED; then
  echo "Note: output-quality is a Tide-local domain — its skills are not synced outward. Edit them in Tide." >&2
fi

# Deduplicate while preserving order (bash 3.2 compatible)
if [[ ${#SKILLS_TO_SYNC[@]} -gt 0 ]]; then
  UNIQUE_LIST=$(printf '%s\n' "${SKILLS_TO_SYNC[@]}" | awk '!seen[$0]++')
  SKILLS_TO_SYNC=()
  while IFS= read -r s; do [[ -n "$s" ]] && SKILLS_TO_SYNC+=("$s"); done <<< "$UNIQUE_LIST"
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

if $INJECT_DOCTRINE; then
  doctrine_mode="write"; $DRY_RUN && doctrine_mode="dry-run"
  inject_doctrine "$TARGET" "$SCRIPT_DIR/tide-doctrine.md" "$doctrine_mode"
fi

if $DRY_RUN; then
  echo ""; echo "(dry-run — no files copied)"; exit 0
fi
[[ ${#SKILLS_TO_SYNC[@]} -eq 0 ]] && exit 0

# --- Execute ----------------------------------------------------------------

mkdir -p "$TARGET_SKILLS"
COPIED=0; IN_SYNC=0; MISSING=0; CONFLICTS=0

# Return 0 if every file in source is present and identical in target (target
# may have additional files; those are preserved by the cp -R sync).
source_subset_of_target() {
  local src_dir="$1" dst_dir="$2" src_file rel dst_file
  [[ -d "$dst_dir" ]] || return 1
  while IFS= read -r -d '' src_file; do
    rel="${src_file#"$src_dir"/}"
    dst_file="$dst_dir/$rel"
    if [[ ! -f "$dst_file" ]] || ! cmp -s "$src_file" "$dst_file"; then return 1; fi
  done < <(find "$src_dir" -type f -print0)
  return 0
}

for skill in "${SKILLS_TO_SYNC[@]}"; do
  src="$SKILLS_DIR/$skill"; dst="$TARGET_SKILLS/$skill"
  if [[ ! -d "$src" ]]; then
    echo "  ! source missing, skipping: $src" >&2; MISSING=$((MISSING+1)); continue
  fi
  if [[ -d "$dst" ]]; then
    if source_subset_of_target "$src" "$dst"; then IN_SYNC=$((IN_SYNC+1)); continue; fi
    if ! $FORCE; then
      echo "  ! conflict (target differs, use --force to overwrite): $dst" >&2
      CONFLICTS=$((CONFLICTS+1)); continue
    fi
  fi
  mkdir -p "$dst"; cp -R "$src/." "$dst/"
  echo "  ✓ $skill"; COPIED=$((COPIED+1))
done

echo ""
echo "Copied: $COPIED   In sync: $IN_SYNC   Source missing: $MISSING   Conflicts: $CONFLICTS"
if [[ $CONFLICTS -gt 0 ]]; then
  echo "Re-run with --force to overwrite conflicting skills." >&2; exit 1
fi
