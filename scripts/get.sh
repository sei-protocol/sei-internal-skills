#!/usr/bin/env bash
# get.sh — take ONE piece of sei-internal-skills, without cloning it.
#
# The installer is all-or-nothing: it clones the repo and syncs the whole core.
# That is right for a teammate adopting the toolkit and wrong for someone who
# wants the output style, or one skill, in a repo that will never want the rest.
# This is the narrow door.
#
# Run it over the wire — sei-internal-skills is an internal repo, so the fetch needs
# `gh` auth and a bare curl will not work:
#
#   gh api repos/sei-protocol/sei-internal-skills/contents/scripts/get.sh \
#     -H 'Accept: application/vnd.github.raw' | bash -s -- <target> [name]
#
# Targets:
#   list                    show everything available, by kind
#   output-style [name]     default: asd-ste100    -> ~/.claude/output-styles/
#   skill <name>            core or experimental   -> ~/.claude/skills/<name>/
#   agent <name>            core or experimental   -> ~/.claude/agents/<name>.md
#
# Environment:
#   SEI_SKILLS_REF      branch or tag to fetch from (default: main)
#   SEI_SKILLS_TARGET   install root, the script appends .claude/ (default: $HOME)
#   SEI_SKILLS_LOCAL    path to an existing checkout; skips the download entirely
#
# WHAT IT WILL NOT DO: delete anything, touch settings.json, or install a second
# resource you did not name. It overwrites only the resource you asked for, and
# it says so before it does.
#
# WHY IT EXTRACTS THE WHOLE TARBALL: fetching one directory through the contents
# API means walking it recursively, one request per file. The tarball is a few MB
# and one request. The alternative — `tar --include` — is bsdtar syntax that GNU
# tar spells differently, and a documented one-liner that works on a Mac and
# fails on a Linux CI box is worse than a slightly larger download.

set -euo pipefail

REPO="sei-protocol/sei-internal-skills"
REF="${SEI_SKILLS_REF:-main}"
TARGET="${SEI_SKILLS_TARGET:-$HOME}"
TARGET="${TARGET/#\~/$HOME}"

die() { echo "Error: $*" >&2; exit 1; }

command -v gh >/dev/null 2>&1 || die "gh is required — sei-internal-skills is internal, so the fetch needs its auth."
gh auth status >/dev/null 2>&1 || die "gh is not authenticated. Run: gh auth login"

# --- Fetch the tree once, into a temp dir that always gets cleaned up ---------

WORK=""
FETCHED=false
# Must return 0. An EXIT trap that exits non-zero becomes the script's exit
# status, so a successful run would report failure whenever WORK is unset —
# which is every run that used SEI_SKILLS_LOCAL.
cleanup() { [ -n "$WORK" ] && rm -rf "$WORK"; return 0; }
trap cleanup EXIT

fetch_tree() {
  $FETCHED && return 0
  FETCHED=true

  # An existing checkout short-circuits the download. This is what makes the
  # script testable without the network, and it is genuinely useful: if you
  # already cloned the repo, there is no reason to re-fetch it over the wire.
  if [ -n "${SEI_SKILLS_LOCAL:-}" ]; then
    ROOT="${SEI_SKILLS_LOCAL/#\~/$HOME}"
    [ -d "$ROOT/.claude/skills" ] || die "SEI_SKILLS_LOCAL is not a sei-internal-skills checkout: $ROOT"
    return 0
  fi

  WORK="$(mktemp -d)"
  echo "→ fetching ${REPO}@${REF} …" >&2
  gh api "repos/$REPO/tarball/$REF" 2>/dev/null | tar xz -C "$WORK" \
    || die "could not fetch or extract the tarball (check gh auth and the ref '$REF')"
  ROOT="$(find "$WORK" -mindepth 1 -maxdepth 1 -type d | head -1)"
  [ -n "$ROOT" ] || die "unexpected tarball layout"
}

# A skill or agent may live in the shipped core or in experimental/. Look in both,
# and report which tier it came from — the tiers mean different things.
find_skill() {
  local n="$1"
  [ -d "$ROOT/.claude/skills/$n" ]      && { echo "$ROOT/.claude/skills/$n|core"; return 0; }
  [ -d "$ROOT/experimental/skills/$n" ] && { echo "$ROOT/experimental/skills/$n|experimental"; return 0; }
  return 1
}
find_agent() {
  local n="$1"
  [ -f "$ROOT/.claude/agents/$n.md" ]      && { echo "$ROOT/.claude/agents/$n.md|core"; return 0; }
  [ -f "$ROOT/experimental/agents/$n.md" ] && { echo "$ROOT/experimental/agents/$n.md|experimental"; return 0; }
  return 1
}

list_names() {  # list_names <dir> <suffix-to-strip>
  [ -d "$1" ] || return 0
  if [ "$2" = "dir" ]; then find "$1" -mindepth 1 -maxdepth 1 -type d -exec basename {} \; | sort
  else find "$1" -maxdepth 1 -type f -name '*.md' -exec basename {} .md \; | sort; fi
}

usage() {
  cat <<'USAGE'
get.sh — take ONE piece of sei-internal-skills, without cloning it.

  gh api repos/sei-protocol/sei-internal-skills/contents/scripts/get.sh \
    -H 'Accept: application/vnd.github.raw' | bash -s -- <target> [name]

Targets
  list                    show everything available, by kind
  output-style [name]     default: asd-ste100    -> ~/.claude/output-styles/
  skill <name>            core or experimental   -> ~/.claude/skills/<name>/
  agent <name>            core or experimental   -> ~/.claude/agents/<name>.md

Environment
  SEI_SKILLS_REF          branch or tag to fetch from (default: main)
  SEI_SKILLS_TARGET       install root; the script appends .claude/ (default: $HOME)
  SEI_SKILLS_LOCAL        path to an existing checkout; skips the download

It never deletes anything, never touches settings.json, and installs only the
resource you name.
USAGE
}

# --- Targets -----------------------------------------------------------------

cmd_list() {
  fetch_tree
  echo ""
  echo "Output styles"
  list_names "$ROOT/.claude/output-styles" file | sed 's/^/  /'
  echo ""
  echo "Skills — core (what a full install ships)"
  list_names "$ROOT/.claude/skills" dir | sed 's/^/  /'
  echo ""
  echo "Skills — experimental (parked; never installed by default)"
  list_names "$ROOT/experimental/skills" dir | sed 's/^/  /'
  echo ""
  echo "Agents — core"
  list_names "$ROOT/.claude/agents" file | sed 's/^/  /'
  echo ""
  echo "Agents — experimental"
  list_names "$ROOT/experimental/agents" file | sed 's/^/  /'
  echo ""
  echo "Take one:  … | bash -s -- skill xreview"
}

cmd_output_style() {
  local name="${1:-asd-ste100}"
  fetch_tree
  local src="$ROOT/.claude/output-styles/$name.md"
  [ -f "$src" ] || die "no output style named '$name'. Run with 'list' to see what exists."
  local dst="$TARGET/.claude/output-styles"
  mkdir -p "$dst"
  cp "$src" "$dst/$name.md"
  echo "✓ $dst/$name.md"
  echo ""
  # Same contract as sync-output-styles.sh: ship the file, never flip the switch.
  local style
  style="$(grep -m1 '^name:' "$src" | sed 's/^name: *//')"
  if grep -q '"outputStyle"' "$TARGET/.claude/settings.json" 2>/dev/null; then
    echo "  You already have an outputStyle set, so this one is installed but not active."
  else
    echo "  Installed but NOT active — activating a style is your call, not this script's."
    echo "  Turn it on with:  /config  →  Output Style  →  $style"
    echo "  Or add to $TARGET/.claude/settings.json:   \"outputStyle\": \"$style\""
  fi
}

cmd_skill() {
  local name="${1:-}"
  [ -n "$name" ] || die "usage: skill <name>   (run 'list' to see what exists)"
  fetch_tree
  local found tier src
  found="$(find_skill "$name")" || die "no skill named '$name'. Run with 'list' to see what exists."
  src="${found%|*}"; tier="${found#*|}"
  local dst="$TARGET/.claude/skills/$name"
  mkdir -p "$dst"
  cp -R "$src/." "$dst/"
  echo "✓ $dst  ($tier)"
  [ "$tier" = "experimental" ] && echo "  Experimental: parked in the repo, not part of the shipped core, and may change."
  echo ""
  echo "  Edit it in sei-internal-skills, not here — a later sync overwrites this copy."
}

cmd_agent() {
  local name="${1:-}"
  [ -n "$name" ] || die "usage: agent <name>   (run 'list' to see what exists)"
  fetch_tree
  local found tier src
  found="$(find_agent "$name")" || die "no agent named '$name'. Run with 'list' to see what exists."
  src="${found%|*}"; tier="${found#*|}"
  local dst="$TARGET/.claude/agents"
  mkdir -p "$dst"
  cp "$src" "$dst/$name.md"
  echo "✓ $dst/$name.md  ($tier)"
  [ "$tier" = "experimental" ] && echo "  Experimental: parked in the repo, not part of the shipped core, and may change."
  echo ""
  echo "  An agent may reference skills it expects installed. If it names one, take that too."
}

case "${1:-}" in
  list)          cmd_list ;;
  output-style)  cmd_output_style "${2:-}" ;;
  skill)         cmd_skill "${2:-}" ;;
  agent)         cmd_agent "${2:-}" ;;
  -h|--help|"")  usage ;;
  *)             die "unknown target '${1}'. Try: list | output-style | skill | agent" ;;
esac
