#!/usr/bin/env bash
# install.sh — install the agentic-writing toolkit on a machine.
#
#   curl -fsSL https://raw.githubusercontent.com/sei-protocol/sei-internal-skills/main/writing/scripts/install.sh | bash
#
# That is the whole setup. After it, `vale` works in any directory, with no
# per-project configuration and nothing to commit.
#
# WHICH RULES RUN DEPENDS ON WHERE THE FILE SITS. The contract is a directory
# convention rather than a config file:
#
#   specs/<feature>/spec.md   spec structure, RFC 2119 casing, prose
#   docs/adr/NNNN-name.md     the four Nygard sections, prose, no sentence cap
#   docs/design/name.md       the four design sections, prose, no sentence cap
#   docs/procedures/name.md   the tighter procedure limits
#   tickets/id.md             the seven ticket sections, prose
#   anywhere else             prose only
#
# Those paths are relative to wherever you run `vale`, so a scratch directory
# works as well as a repository.
#
# WHAT IT INSTALLS
#
#   ~/.agentic-writing              the checkout, holding the rules, the Spec Kit
#                                   templates under .specify/templates, and
#                                   scripts/build-spec-artifact.sh
#   <user vale dir>/styles/         symlinks to the rules, so vale finds them
#   <user vale dir>/.vale.ini       the fallback config, if you have none
#
# It is idempotent. Re-run it to pick up new rules.
#
# WHAT IT WILL NOT DO: overwrite a Vale config you already have, write into any
# repository, commit anything, or push.
#
# THERE IS ALSO A `repo` MODE, and most people do not need it. It wires a
# repository into CI so the checks run for everyone rather than for whoever
# installed this. Reach for it when a team wants the contract enforced on merge,
# not while one engineer is trying the framework out:
#
#   curl -fsSL .../scripts/install.sh | bash -s -- repo
#
# Environment:
#   AGENTIC_WRITING_HOME   checkout location (default: ~/.agentic-writing)
#   AGENTIC_WRITING_REF    branch or tag to install (default: main)
#   AGENTIC_WRITING_REPO   source to clone from (default: the GitHub repository).
#                          A local path works, which is how the installer gets
#                          tested against a change before it is pushed.
#
# --dry-run reports what a mode would do, and writes nothing.
set -euo pipefail

REPO_URL="${AGENTIC_WRITING_REPO:-https://github.com/sei-protocol/sei-internal-skills}"
RAW="https://raw.githubusercontent.com/sei-protocol/sei-internal-skills"
HOME_DIR="${AGENTIC_WRITING_HOME:-$HOME/.agentic-writing}"
REF="${AGENTIC_WRITING_REF:-main}"
DRY_RUN=false

usage() {
  cat <<'USAGE'
install.sh — the agentic-writing toolkit.

  install.sh                 install on this machine (the usual case)
  install.sh repo            additionally wire the current repository into CI
  install.sh --dry-run       report what would happen, write nothing
  install.sh --help          this text

After a machine install, `vale docs/` works anywhere. Which rules run depends on
the directory a file sits in:

  specs/<feature>/spec.md   spec structure, RFC 2119 casing, prose
  docs/adr/NNNN-name.md     Nygard sections, prose, no sentence cap
  docs/design/name.md       design sections, prose, no sentence cap
  tickets/id.md             the seven ticket sections, prose
  anywhere else             prose only

It will not overwrite a Vale config you already have, and it commits nothing.
USAGE
}

say() { printf '%s\n' "$*"; }

# Runs a command, or reports it under --dry-run. Arguments pass through as argv,
# never through eval: a repository path holding a space or an apostrophe broke
# the eval form, and a path is user input this script does not control.
run() {
  if $DRY_RUN; then
    printf '  would: %s\n' "$*"
  else
    "$@"
  fi
}

# Same, for the two operations that need a shell redirection.
render() {  # render TEMPLATE DEST — substitute the pinned ref
  if $DRY_RUN; then
    printf '  would: render %s -> %s\n' "$1" "$2"
  else
    sed "s|@REF@|$REF|g" "$1" > "$2"
  fi
}
append() {  # append FILE TEXT
  if $DRY_RUN; then
    printf '  would: append to %s\n' "$1"
  else
    printf '%s' "$2" >> "$1"
  fi
}

install_machine() {
  say "agentic-writing: machine install (ref $REF)"

  if [ -d "$HOME_DIR/.git" ]; then
    say "  checkout exists at $HOME_DIR, updating"
    run git -C "$HOME_DIR" fetch --quiet origin "$REF"
    run git -C "$HOME_DIR" checkout --quiet "$REF"
    $DRY_RUN || git -C "$HOME_DIR" merge --quiet --ff-only "origin/$REF" 2>/dev/null || true
  else
    say "  cloning into $HOME_DIR"
    run git clone --quiet --branch "$REF" "$REPO_URL" "$HOME_DIR"
  fi

  # macOS and Linux put the user Vale directory in different places.
  case "$(uname -s)" in
    Darwin) vale_dir="$HOME/Library/Application Support/vale" ;;
    *)      vale_dir="${XDG_CONFIG_HOME:-$HOME/.config}/vale" ;;
  esac
  run mkdir -p "$vale_dir/styles"

  for style in AgenticWriting config; do
    say "  linking $style into the user styles directory"
    run ln -sfn "$HOME_DIR/styles/$style" "$vale_dir/styles/$style"
  done

  if [ -f "$vale_dir/.vale.ini" ]; then
    say "  user Vale config exists, leaving it alone"
    say "    compare against $HOME_DIR/docs/vale-global-config.reference.ini"
  else
    say "  installing the fallback Vale config"
    run cp "$HOME_DIR/docs/vale-global-config.reference.ini" "$vale_dir/.vale.ini"
  fi

  # Without this the first `vale` run fails with "style 'write-good' does not
  # exist". Repo mode already syncs; machine mode did not, and the gap hid on a
  # machine whose styles directory was already populated.
  say "  fetching the declared packages"
  if ! $DRY_RUN; then
    ( cd "$vale_dir" && vale sync >/dev/null 2>&1 ) || \
      say "    note: 'vale sync' failed. Is vale on PATH? Run it in $vale_dir." 
  fi

  say ""
  say 'Done. `vale docs/` now works in any directory.'
  say ""
  say "Which rules run depends on where a file sits:"
  say "  specs/<feature>/spec.md   spec structure, RFC 2119 casing, prose"
  say "  docs/adr/NNNN-name.md     Nygard sections, prose, no sentence cap"
  say "  docs/design/name.md       design sections, prose, no sentence cap"
  say "  tickets/id.md             the seven ticket sections, prose"
  say ""
  say "Starting a document:"
  say "  $HOME_DIR/.specify/templates/"
  say ""
  say "Publishing a spec as an artifact:"
  say "  $HOME_DIR/scripts/build-spec-artifact.sh --help"
  say ""
  say "CI for a whole team is a separate, optional step:"
  say "  cd <repo> && curl -fsSL $RAW/main/scripts/install.sh | bash -s -- repo"
}

install_repo() {
  if ! git rev-parse --show-toplevel >/dev/null 2>&1; then
    say "not inside a git repository. cd to one first." >&2
    exit 2
  fi
  root="$(git rev-parse --show-toplevel)"
  if [ "$root" = "$HOME" ]; then
    say "refusing: \$HOME is a git repository, and this would write .vale.ini there." >&2
    say "  A .vale.ini in \$HOME shadows the machine config for every repository" >&2
    say "  that has none. cd to the project you meant." >&2
    exit 2
  fi
  # Installing the toolkit into the toolkit. It writes a consumer workflow beside
  # the reusable one it calls, appends to .gitignore, and drops a .vale/ tree that
  # the repository's own config does not use. Nothing here is destructive and all
  # of it is wrong, so it is easier to refuse than to explain. A test that wants a
  # scratch repository has to cd into one first; that is how this was found.
  if [ -f "$root/writing/templates/consumer.vale.ini" ] && [ -d "$root/writing/styles/AgenticWriting" ]; then
    say "refusing: $root holds the toolkit itself." >&2
    say "  This mode wires a CONSUMING repository into the checks. Run it from the" >&2
    say "  repository you want checked, not from the one holding the rules." >&2
    exit 2
  fi
  say "agentic-writing: repository install in $root (pinning ref $REF)"
  say "  This is the CI path. One engineer trying the framework needs only the"
  say "  machine install, which requires nothing here."

  src="$HOME_DIR"
  if [ ! -d "$src/writing/templates" ]; then
    if $DRY_RUN; then
      say "  no local checkout; a real run would fetch templates at $REF"
      say "  (nothing further to report without them)"
      return 0
    fi
    TMP_SRC="$(mktemp -d)"
    trap 'rm -rf "$TMP_SRC"' EXIT
    src="$TMP_SRC"
    say "  no local checkout, fetching templates at $REF"
    git clone --quiet --depth 1 --branch "$REF" "$REPO_URL" "$src"
  fi

  if [ -f "$root/.vale.ini" ]; then
    say "  .vale.ini exists, leaving it alone"
    say "    compare against $src/templates/consumer.vale.ini"
  else
    say "  writing .vale.ini"
    render "$src/writing/templates/consumer.vale.ini" "$root/.vale.ini"
  fi

  wf="$root/.github/workflows/writing.yml"
  if [ -f "$wf" ]; then
    say "  writing.yml exists, leaving it alone"
  else
    say "  writing .github/workflows/writing.yml"
    run mkdir -p "$root/.github/workflows"
    render "$src/writing/templates/writing.yml" "$wf"
  fi

  # Fetch the rules now, so `vale` works immediately rather than failing with
  # E201 on a StylesPath that nothing has populated. A repository-local
  # .vale.ini means Vale never consults the user styles directory, so a machine
  # install does not cover this.
  say "  fetching the rules into .vale/styles"
  run mkdir -p "$root/.vale/styles"
  for style in AgenticWriting config; do
    # Replace only what this script owns. A repository may already keep its own
    # rules under StylesPath, and removing the parent would delete them while
    # the message above says the config was left alone.
    run rm -rf "$root/.vale/styles/$style"
    run cp -R "$src/writing/styles/$style" "$root/.vale/styles/"
  done
  if ! $DRY_RUN; then
    ( cd "$root" && vale sync >/dev/null 2>&1 ) || \
      say "    note: 'vale sync' did not run. Run it to fetch write-good and proselint."
  fi

  # This repository's own accepted terms. Vale reads a vocabulary from
  # StylesPath/config/vocabularies, which the fetch above overwrites, so the
  # committed copy lives outside it and gets installed into place. CI does the
  # same. Vale errors on a Vocab it cannot find, so the file always exists.
  run mkdir -p "$root/.vale/vocab"
  if ! $DRY_RUN && [ ! -f "$root/.vale/vocab/accept.txt" ]; then
    cat > "$root/.vale/vocab/accept.txt" <<'VOCAB'
# Terms this repository accepts, one per line, case-sensitive. Commit this file.
# The rules ship their own vocabulary and overwrite it on every install, so a
# term that belongs to this repository belongs here.
#
# Start with your hyphenated identifiers. Vale reads YAML frontmatter as prose,
# so a line like `name: my-thing-specialist` is checked like a sentence.
#
# Add a hyphenated name, not a bare word. An entry here is not a permission; it
# is the one correct casing, and every other form becomes an error. `evm` makes
# `evm` wrong wherever you wrote `EVM`. A hyphenated name collides with nothing.
VOCAB
  fi
  run mkdir -p "$root/.vale/styles/config/vocabularies/Local"
  run cp "$root/.vale/vocab/accept.txt" \
        "$root/.vale/styles/config/vocabularies/Local/accept.txt"

  if ! grep -qxF '.vale/styles/' "$root/.gitignore" 2>/dev/null; then
    say "  adding the fetched paths to .gitignore"
    append "$root/.gitignore" '
# agentic-writing fetches the rules into these. Anything else under .vale/ is
# yours, including .vale/vocab/accept.txt.
.vale/styles/
.vale/src/
'
  fi

  say ""
  say "Commit these, and the checks run for everyone on every branch:"
  say "  .vale.ini  .github/workflows/writing.yml  .gitignore  .vale/vocab/accept.txt"
  say ""
  say "The rules themselves are in .vale/, which is gitignored. Re-run this to"
  say "refresh them, or raise the pin in .vale.ini and writing.yml together." 
  say ""
  say "Spec Kit is separate and this script does not install it. If this"
  say "repository writes specifications, it also needs a constitution:"
  say "  specify init --here --integration claude"
  say ""
  say "This repository vendors the spec template and nothing else of Spec Kit's."
}

while [ $# -gt 0 ]; do
  case "$1" in
    --dry-run) DRY_RUN=true; shift ;;
    machine|repo) MODE="$1"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) say "unknown argument: $1" >&2; exit 2 ;;
  esac
done

case "${MODE:-machine}" in
  machine) install_machine ;;
  repo)    install_repo ;;
esac
