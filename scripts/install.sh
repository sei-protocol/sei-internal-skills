#!/usr/bin/env bash
# install.sh: install the sei-internal-skills catalog into ~/.claude, or one piece of it.
# It needs no clone and runs safely over the wire. `install.sh -h` lists the targets and
# the environment variables.
#
# sei-internal-skills is a public GitHub repository: curl fetches it with no auth; gh works too.
#
#   curl -fsSL https://raw.githubusercontent.com/sei-protocol/sei-internal-skills/main/scripts/install.sh | bash
#   gh api repos/sei-protocol/sei-internal-skills/contents/scripts/install.sh \
#     -H 'Accept: application/vnd.github.raw' | bash
#
# The two modes differ on purpose. With no arguments, you adopt the catalog: the script
# keeps a checkout at ~/.sei-internal-skills, so `make update` works later, and installs
# every skill, agent and output style. With a target, you want one resource: the script
# clones nothing and installs only that resource. It reads an existing checkout if one is
# there. A targeted install deletes nothing. Only `output-style` writes settings.json: it
# sets outputStyle, backs the file up, keeps every other key, edits through a symlink,
# and never replaces a style you already chose.

set -euo pipefail

REPO="sei-protocol/sei-internal-skills"
SEI_INTERNAL_SKILLS_HOME="${SEI_INTERNAL_SKILLS_HOME:-$HOME/.sei-internal-skills}"
REF="${SEI_SKILLS_REF:-main}"
TARGET="${SEI_SKILLS_TARGET:-$HOME}"
TARGET="${TARGET/#\~/$HOME}"

# Consumed before dispatch so it can precede the target. It must NOT leave the
# argument list empty: that would fall through to the no-argument path, which
# clones and syncs everything. Someone reaching for the escape hatch is asking
# to install less, not to mutate their whole environment.
NO_ACTIVATE=false
if [ "${1:-}" = "--no-activate" ]; then
  NO_ACTIVATE=true; shift
  [ -n "${1:-}" ] || { echo "Error: --no-activate needs a target, e.g. output-style" >&2; exit 2; }
fi

have() { command -v "$1" >/dev/null 2>&1; }
die()  { echo "Error: $*" >&2; exit 1; }

usage() {
  cat <<'USAGE'
install.sh: install the sei-internal-skills catalog, or one piece of it.

sei-internal-skills is a public GitHub repository. curl needs no auth.

  curl -fsSL https://raw.githubusercontent.com/sei-protocol/sei-internal-skills/main/scripts/install.sh \
    | bash [-s -- <target> [name]]

  gh api repos/sei-protocol/sei-internal-skills/contents/scripts/install.sh \
    -H 'Accept: application/vnd.github.raw' | bash [-s -- <target> [name]]

Everything (no arguments)
  Clones this repository into ~/.sei-internal-skills, or fast-forwards that
  checkout. Then installs every skill, agent and output style into ~/.claude.
  Use this to adopt the catalog.

One piece (no clone; installs only the resource you name)
  list                    show everything available, by kind
  output-style [name]     default: asd-ste100    -> ~/.claude/output-styles/
                          also activates it in settings.json; never overwrites
                          a style you already chose. --no-activate to skip.
  skill <name>            -> ~/.claude/skills/<name>/
  agent <name>            -> ~/.claude/agents/<name>.md

  A targeted install reads an existing checkout and does not pull it. With no
  checkout, it downloads the tree with gh (when signed in) or with curl.

Environment
  SEI_INTERNAL_SKILLS_HOME   checkout location (default: ~/.sei-internal-skills)
  SEI_SKILLS_REF             branch, tag or commit for a download (default: main);
                             ignored when a checkout exists
  SEI_SKILLS_TARGET          install root for one piece; appends .claude/ (default: $HOME)
USAGE
}

# ============================================================================
# Mode 1: the whole catalog. This is the documented path.
# ============================================================================

# gh, when it is signed in, clones with the git protocol you chose for it.
# Without it, an anonymous https clone works, because the repository is public.
gh_ready() { have gh && gh auth status >/dev/null 2>&1; }

install_everything() {
  clone_repo() {
    if gh_ready; then
      gh repo clone "$REPO" "$SEI_INTERNAL_SKILLS_HOME"
    else
      git clone "https://github.com/$REPO.git" "$SEI_INTERNAL_SKILLS_HOME"
    fi
  }

  have git  || die "git is required. Install it, then re-run."
  have make || die "make is required. On macOS, run: xcode-select --install"

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

  # One sync entry point, shared with `make update`. It does no git pull: the
  # clone or fast-forward above already made the checkout current.
  make -C "$SEI_INTERNAL_SKILLS_HOME" sync-all

  echo "✓ sei-internal-skills catalog installed into ~/.claude (checkout: $SEI_INTERNAL_SKILLS_HOME)"
  echo "  Start a new Claude Code session to load it."
  echo "  Stay current with:  make -C \"$SEI_INTERNAL_SKILLS_HOME\" update"
}

# ============================================================================
# Mode 2: one resource. No clone.
# ============================================================================

WORK=""
ROOT=""
# Must return 0. An EXIT trap that exits non-zero becomes the script's exit
# status, so a successful run would report failure whenever WORK is unset —
# which is every run that read an existing checkout instead of downloading.
cleanup() { [ -n "$WORK" ] && rm -rf "$WORK"; return 0; }

resolve_tree() {
  [ -n "$ROOT" ] && return 0

  # An existing checkout is authoritative and free, and it keeps the test suite
  # offline. The script reads it as it is and never pulls it.
  if [ -d "$SEI_INTERNAL_SKILLS_HOME/.claude/skills" ]; then
    ROOT="$SEI_INTERNAL_SKILLS_HOME"
    echo "→ reading your checkout at $ROOT (not updated; run: make -C $ROOT update)" >&2
    return 0
  fi

  trap cleanup EXIT
  WORK="$(mktemp -d)"
  echo "→ fetching ${REPO}@${REF} …" >&2
  fetch_with_gh || fetch_with_curl \
    || die "could not fetch $REPO@$REF (check the ref; the download needs curl or an authenticated gh)"
  ROOT="$(find "$WORK" -mindepth 1 -maxdepth 1 -type d | head -1)"
  [ -n "$ROOT" ] || die "unexpected tarball layout"
}

# Each fetch takes the whole tree as one tarball into an empty WORK. A plain
# `tar xz` works with both bsdtar and GNU tar. A failed gh fetch empties WORK,
# so the curl fetch starts clean.
fetch_with_gh() {
  gh_ready || return 1
  if gh api "repos/$REPO/tarball/$REF" 2>/dev/null | tar xz -C "$WORK" 2>/dev/null; then
    return 0
  fi
  find "$WORK" -mindepth 1 -delete
  return 1
}

# The repository is public, so codeload serves the tarball with no auth. The
# timeouts make a stalled network fail fast and reach the error message.
fetch_with_curl() {
  have curl || return 1
  curl -fsSL --connect-timeout 15 --max-time 300 "https://codeload.github.com/$REPO/tar.gz/$REF" 2>/dev/null \
    | tar xz -C "$WORK" 2>/dev/null
}

# A skill is a directory under .claude/skills/ that holds a SKILL.md. A leftover
# directory without one (ignored state/ files after a pull) is not a skill, so
# neither lookup nor list ever offers it.
find_skill() {
  [ -f "$ROOT/.claude/skills/$1/SKILL.md" ] && { echo "$ROOT/.claude/skills/$1"; return 0; }
  return 1
}
find_agent() {
  [ -f "$ROOT/.claude/agents/$1.md" ] && { echo "$ROOT/.claude/agents/$1.md"; return 0; }
  return 1
}

list_names() {  # list_names <dir> <dir|file>
  [ -d "$1" ] || return 0
  local d
  if [ "$2" = "dir" ]; then
    for d in "$1"/*/; do
      if [ -f "${d}SKILL.md" ]; then basename "$d"; fi
    done | sort
  else find "$1" -maxdepth 1 -type f -name '*.md' -exec basename {} .md \; | sort; fi
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
  [ -e "$f" ] && cp "$f" "$f.bak-$(date +%Y%m%d%H%M%S)"
  python3 - "$f" "$style" <<'PYEOF'
import json, os, sys
path, style = sys.argv[1], sys.argv[2]

# Resolve the symlink before writing. os.replace onto a symlink swaps the LINK
# for a regular file, which silently detaches a dotfiles-managed settings.json
# and leaves the real file without the change. Editing through the link is what
# the user meant by making it a link.
path = os.path.realpath(path)

d = {}
if os.path.exists(path):
    with open(path) as fh:
        d = json.load(fh)
    if not isinstance(d, dict):
        raise SystemExit("settings.json is not a JSON object")
d["outputStyle"] = style
# Temp file beside the real target so the rename is atomic on one filesystem.
tmp = path + ".tmp"
with open(tmp, "w") as fh:
    json.dump(d, fh, indent=2)
    fh.write("\n")
os.replace(tmp, path)
PYEOF
}

# Turning a style on is the point of asking for it by name, so a targeted
# install activates it. It never takes that decision from someone who already
# made one: an existing, different style is reported and left alone.
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
  echo "Output styles"; list_names "$ROOT/.claude/output-styles" file | sed 's/^/  /'
  echo ""
  echo "Skills";        list_names "$ROOT/.claude/skills" dir | sed 's/^/  /'
  echo ""
  echo "Agents";        list_names "$ROOT/.claude/agents" file | sed 's/^/  /'
  echo ""
  echo "Take one:  … | bash -s -- skill harbor-dev"
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
  local src; src="$(find_skill "$name")" || die "no skill named '$name'. Run with 'list' to see what exists."
  mkdir -p "$TARGET/.claude/skills/$name"
  cp -R "$src/." "$TARGET/.claude/skills/$name/"
  echo "✓ $TARGET/.claude/skills/$name"
  echo ""
  echo "  Edit it in sei-internal-skills, not here. A later sync overwrites this copy."
}

cmd_agent() {
  local name="${1:-}"
  [ -n "$name" ] || die "usage: agent <name>   (run 'list' to see what exists)"
  resolve_tree
  local src; src="$(find_agent "$name")" || die "no agent named '$name'. Run with 'list' to see what exists."
  mkdir -p "$TARGET/.claude/agents"
  cp "$src" "$TARGET/.claude/agents/$name.md"
  echo "✓ $TARGET/.claude/agents/$name.md"
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
