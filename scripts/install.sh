#!/usr/bin/env bash
# install.sh — install the sei-internal-skills toolkit into ~/.claude, or take one piece of it,
# even if you've never cloned the repo. Safe to run over the wire.
#
# sei-internal-skills is an INTERNAL GitHub repo, so the fetch needs auth — `gh` provides it.
#
#   # everything (the default): clone or fast-forward, then sync the whole core
#   gh api repos/sei-protocol/sei-internal-skills/contents/scripts/install.sh \
#     -H 'Accept: application/vnd.github.raw' | bash
#
#   # one piece: no clone, nothing else installed
#   … | bash -s -- output-style
#   … | bash -s -- skill xreview
#   … | bash -s -- agent idiomatic-reviewer
#   … | bash -s -- list
#
# Or, once cloned, `make update` from the repo.
#
# THE TWO MODES DIFFER ON PURPOSE:
#
#   No arguments — you are adopting the toolkit. It keeps a checkout at
#   ~/.sei-internal-skills so `make update` works later, and syncs the whole core.
#   Idempotent; safe to re-run any time.
#
#   A target — you want one resource in an environment that will never want the
#   rest. It does NOT clone and leaves no checkout behind, unless you already
#   have one, in which case it reads that instead of re-downloading.
#
# WHAT A TARGETED INSTALL WILL NOT DO: delete anything, or install a second
# resource you did not name. Ask for a skill and you get that skill, not its
# agent and not the skills it references.
#
# ONE EXCEPTION, AND ONLY ONE: `output-style` sets outputStyle in settings.json,
# because naming a style is the request to use it. It backs the file up first,
# preserves every other key, and REFUSES to overwrite a style you already chose
# — that one gets reported and left alone. `--no-activate` installs the file
# without touching settings. Nothing else this script does writes settings.json.
#
# Environment:
#   SEI_INTERNAL_SKILLS_HOME   checkout location (default: ~/.sei-internal-skills)
#   SEI_SKILLS_REF             branch or tag for a targeted fetch (default: main)
#   SEI_SKILLS_TARGET          install root; the script appends .claude/ (default: $HOME)

set -euo pipefail

REPO="sei-protocol/sei-internal-skills"
SEI_INTERNAL_SKILLS_HOME="${SEI_INTERNAL_SKILLS_HOME:-$HOME/.sei-internal-skills}"
REF="${SEI_SKILLS_REF:-main}"
TARGET="${SEI_SKILLS_TARGET:-$HOME}"
TARGET="${TARGET/#\~/$HOME}"

# Consumed before dispatch so it can precede the target.
NO_ACTIVATE=false
if [ "${1:-}" = "--no-activate" ]; then NO_ACTIVATE=true; shift; fi

have() { command -v "$1" >/dev/null 2>&1; }
die()  { echo "Error: $*" >&2; exit 1; }

usage() {
  cat <<'USAGE'
install.sh — install the sei-internal-skills toolkit, or take one piece of it.

  gh api repos/sei-protocol/sei-internal-skills/contents/scripts/install.sh \
    -H 'Accept: application/vnd.github.raw' | bash [-s -- <target> [name]]

Everything (no arguments)
  Clones or fast-forwards ~/.sei-internal-skills, then syncs the whole core into
  ~/.claude. This is what you want if you are adopting the toolkit.

One piece
  list                    show everything available, by kind
  output-style [name]     default: asd-ste100    -> ~/.claude/output-styles/
                          also activates it in settings.json; never overwrites
                          a style you already chose. --no-activate to skip.
  skill <name>            core or experimental   -> ~/.claude/skills/<name>/
  agent <name>            core or experimental   -> ~/.claude/agents/<name>.md

  No clone, and nothing installed but the resource you name.

Environment
  SEI_INTERNAL_SKILLS_HOME   checkout location (default: ~/.sei-internal-skills)
  SEI_SKILLS_REF             branch or tag for a targeted fetch (default: main)
  SEI_SKILLS_TARGET          install root; appends .claude/ (default: $HOME)
USAGE
}

# ============================================================================
# Mode 1 — the whole toolkit. Unchanged behaviour; this is the documented path.
# ============================================================================

install_everything() {
  clone_repo() {
    if have gh; then
      gh repo clone "$REPO" "$SEI_INTERNAL_SKILLS_HOME"
    else
      echo "→ gh not found; falling back to git clone (needs SSH or credential-helper auth for an internal repo)" >&2
      git clone "git@github.com:$REPO.git" "$SEI_INTERNAL_SKILLS_HOME" 2>/dev/null \
        || git clone "https://github.com/$REPO.git" "$SEI_INTERNAL_SKILLS_HOME"
    fi
  }

  if [ -d "$SEI_INTERNAL_SKILLS_HOME/.git" ]; then
    echo "→ updating existing sei-internal-skills checkout at $SEI_INTERNAL_SKILLS_HOME"
    git -C "$SEI_INTERNAL_SKILLS_HOME" pull --ff-only
  elif [ -e "$SEI_INTERNAL_SKILLS_HOME" ]; then
    echo "Error: $SEI_INTERNAL_SKILLS_HOME exists but is not a sei-internal-skills git checkout." >&2
    echo "       Move it aside or set SEI_INTERNAL_SKILLS_HOME=<free path> and re-run." >&2
    exit 1
  else
    echo "→ cloning $REPO into $SEI_INTERNAL_SKILLS_HOME"
    clone_repo
  fi

  # Single sync entrypoint, shared with `make update`. No git pull here — the
  # clone/fast-forward above already made the checkout current.
  make -C "$SEI_INTERNAL_SKILLS_HOME" sync-all

  echo "✓ sei-internal-skills toolkit synced into ~/.claude (checkout: $SEI_INTERNAL_SKILLS_HOME)"
  echo "  Re-run any time with:  make -C \"$SEI_INTERNAL_SKILLS_HOME\" update"
}

# ============================================================================
# Mode 2 — one resource. No clone.
# ============================================================================

WORK=""
ROOT=""
# Must return 0. An EXIT trap that exits non-zero becomes the script's exit
# status, so a successful run would report failure whenever WORK is unset —
# which is every run that read an existing checkout instead of downloading.
cleanup() { [ -n "$WORK" ] && rm -rf "$WORK"; return 0; }

resolve_tree() {
  [ -n "$ROOT" ] && return 0

  # An existing checkout is authoritative and free. Re-downloading a repo the
  # caller already has is pure waste, and it is also what makes this testable
  # without the network.
  if [ -d "$SEI_INTERNAL_SKILLS_HOME/.claude/skills" ]; then
    ROOT="$SEI_INTERNAL_SKILLS_HOME"
    echo "→ reading your checkout at $ROOT" >&2
    return 0
  fi

  have gh || die "gh is required — sei-internal-skills is internal, so the fetch needs its auth."
  gh auth status >/dev/null 2>&1 || die "gh is not authenticated. Run: gh auth login"

  trap cleanup EXIT
  WORK="$(mktemp -d)"
  echo "→ fetching ${REPO}@${REF} …" >&2
  # One request for the whole tree beats walking the contents API per file, and
  # `tar --include` is bsdtar syntax that GNU tar spells differently — a
  # published one-liner must not depend on which tar the caller has.
  gh api "repos/$REPO/tarball/$REF" 2>/dev/null | tar xz -C "$WORK" \
    || die "could not fetch or extract the tarball (check gh auth and the ref '$REF')"
  ROOT="$(find "$WORK" -mindepth 1 -maxdepth 1 -type d | head -1)"
  [ -n "$ROOT" ] || die "unexpected tarball layout"
}

# A skill or agent may live in the shipped core or in experimental/. Look in
# both, and report which tier it came from — the tiers mean different things to
# whoever is about to depend on one.
find_skill() {
  [ -d "$ROOT/.claude/skills/$1" ]      && { echo "$ROOT/.claude/skills/$1|core"; return 0; }
  [ -d "$ROOT/experimental/skills/$1" ] && { echo "$ROOT/experimental/skills/$1|experimental"; return 0; }
  return 1
}
find_agent() {
  [ -f "$ROOT/.claude/agents/$1.md" ]      && { echo "$ROOT/.claude/agents/$1.md|core"; return 0; }
  [ -f "$ROOT/experimental/agents/$1.md" ] && { echo "$ROOT/experimental/agents/$1.md|experimental"; return 0; }
  return 1
}

list_names() {  # list_names <dir> <dir|file>
  [ -d "$1" ] || return 0
  if [ "$2" = "dir" ]; then find "$1" -mindepth 1 -maxdepth 1 -type d -exec basename {} \; | sort
  else find "$1" -maxdepth 1 -type f -name '*.md' -exec basename {} .md \; | sort; fi
}

note_tier() {
  [ "$1" = "experimental" ] || return 0
  echo "  Experimental: parked in the repo, not part of the shipped core, and may change."
}

# Reads settings.json and prints the current outputStyle, or nothing. Prints
# MALFORMED if the file exists but will not parse, so the caller can refuse
# rather than overwrite a file it cannot understand.
read_current_style() {
  local f="$1"
  [ -f "$f" ] || return 0
  python3 - "$f" <<'PYEOF' 2>/dev/null || echo MALFORMED
import json, sys
try:
    with open(sys.argv[1]) as fh:
        d = json.load(fh)
except Exception:
    print("MALFORMED"); sys.exit(0)
print(d.get("outputStyle", "") if isinstance(d, dict) else "MALFORMED")
PYEOF
}

# Sets outputStyle while preserving every other key. Backs the file up first:
# this is the user's own settings file, and the script is usually run piped from
# the internet, where a bad write is not something they can easily undo.
write_style() {
  local f="$1" style="$2"
  mkdir -p "$(dirname "$f")"
  [ -f "$f" ] && cp "$f" "$f.bak-$(date +%Y%m%d%H%M%S)"
  python3 - "$f" "$style" <<'PYEOF'
import json, os, sys
path, style = sys.argv[1], sys.argv[2]
d = {}
if os.path.exists(path):
    with open(path) as fh:
        d = json.load(fh)
    if not isinstance(d, dict):
        raise SystemExit("settings.json is not a JSON object")
d["outputStyle"] = style
tmp = path + ".tmp"
with open(tmp, "w") as fh:
    json.dump(d, fh, indent=2)
    fh.write("\n")
os.replace(tmp, path)
PYEOF
}

# Turning a style on is the whole point of asking for it by name, so a targeted
# install activates it. What it will not do is take that decision away from
# someone who already made one: an existing, different style is reported and
# left alone. Silently replacing it is the failure this used to avoid by never
# activating at all.
activate_style() {
  local style="$1" settings="$TARGET/.claude/settings.json"

  if ! have python3; then
    echo "  Installed. Could not activate automatically, python3 was not found."
    echo "  Turn it on with:  /config  ->  Output Style  ->  $style"
    return 0
  fi

  local current; current="$(read_current_style "$settings")"

  if [ "$current" = "MALFORMED" ]; then
    echo "  Installed, but $settings does not parse as JSON, so it was left untouched."
    echo "  Fix that file, then:  /config  ->  Output Style  ->  $style"
    return 0
  fi
  if [ "$current" = "$style" ]; then
    echo "  Already your active output style. Nothing else to do."
    return 0
  fi
  if [ -n "$current" ]; then
    echo "  Installed, but NOT activated. You already have \"$current\" set, and that is your call."
    echo "  To switch:  /config  ->  Output Style  ->  $style"
    return 0
  fi

  if write_style "$settings" "$style"; then
    echo "  Activated in $settings."
    echo "  Takes effect in your next session, or after /clear in this one."
  else
    echo "  Installed, but could not write $settings."
    echo "  Turn it on with:  /config  ->  Output Style  ->  $style"
  fi
}

cmd_list() {
  resolve_tree
  echo ""
  echo "Output styles";                                       list_names "$ROOT/.claude/output-styles" file | sed 's/^/  /'
  echo ""
  echo "Skills — core (what a full install ships)";           list_names "$ROOT/.claude/skills" dir | sed 's/^/  /'
  echo ""
  echo "Skills — experimental (never installed by default)";  list_names "$ROOT/experimental/skills" dir | sed 's/^/  /'
  echo ""
  echo "Agents — core";                                       list_names "$ROOT/.claude/agents" file | sed 's/^/  /'
  echo ""
  echo "Agents — experimental";                               list_names "$ROOT/experimental/agents" file | sed 's/^/  /'
  echo ""
  echo "Take one:  … | bash -s -- skill xreview"
}

cmd_output_style() {
  local name="${1:-asd-ste100}"
  resolve_tree
  local src="$ROOT/.claude/output-styles/$name.md"
  [ -f "$src" ] || die "no output style named '$name'. Run with 'list' to see what exists."
  mkdir -p "$TARGET/.claude/output-styles"
  cp "$src" "$TARGET/.claude/output-styles/$name.md"
  echo "✓ $TARGET/.claude/output-styles/$name.md"
  echo ""
  local style; style="$(grep -m1 '^name:' "$src" | sed 's/^name: *//')"
  if $NO_ACTIVATE; then
    echo "  Installed, not activated (--no-activate)."
    echo "  Turn it on with:  /config  ->  Output Style  ->  $style"
  else
    activate_style "$style"
  fi
}

cmd_skill() {
  local name="${1:-}"
  [ -n "$name" ] || die "usage: skill <name>   (run 'list' to see what exists)"
  resolve_tree
  local found; found="$(find_skill "$name")" || die "no skill named '$name'. Run with 'list' to see what exists."
  local src="${found%|*}" tier="${found#*|}"
  mkdir -p "$TARGET/.claude/skills/$name"
  cp -R "$src/." "$TARGET/.claude/skills/$name/"
  echo "✓ $TARGET/.claude/skills/$name  ($tier)"
  note_tier "$tier"
  echo ""
  echo "  Edit it in sei-internal-skills, not here — a later sync overwrites this copy."
}

cmd_agent() {
  local name="${1:-}"
  [ -n "$name" ] || die "usage: agent <name>   (run 'list' to see what exists)"
  resolve_tree
  local found; found="$(find_agent "$name")" || die "no agent named '$name'. Run with 'list' to see what exists."
  local src="${found%|*}" tier="${found#*|}"
  mkdir -p "$TARGET/.claude/agents"
  cp "$src" "$TARGET/.claude/agents/$name.md"
  echo "✓ $TARGET/.claude/agents/$name.md  ($tier)"
  note_tier "$tier"
  echo ""
  echo "  An agent may reference skills it expects installed. If it names one, take that too."
}

# ============================================================================

case "${1:-}" in
  "")            install_everything ;;
  list)          cmd_list ;;
  output-style)  cmd_output_style "${2:-}" ;;
  skill)         cmd_skill "${2:-}" ;;
  agent)         cmd_agent "${2:-}" ;;
  -h|--help)     usage ;;
  *)             die "unknown target '${1}'. Try: list | output-style | skill | agent   (or no arguments to install everything)" ;;
esac
