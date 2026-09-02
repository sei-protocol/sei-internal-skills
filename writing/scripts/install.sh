#!/usr/bin/env bash
# install.sh — install the agentic-writing toolkit on a machine.
#
#   curl -fsSL https://raw.githubusercontent.com/sei-protocol/sei-internal-skills/main/writing/scripts/install.sh | bash
#
# That links the rules into your user Vale directory. A user-level .vale.ini has
# to select them, and this script does not write one: the reference config it used
# to copy lives in the standalone repository and was not carried across. Until it
# is, machine mode gets you the rules and you supply the configuration -- or use
# repo mode below, which installs a working one.
#
# WHICH RULES RUN DEPENDS ON WHERE THE FILE SITS, once a configuration selects
# the rules. The contract is a directory convention rather than a config file:
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
#   ~/.agentic-writing              the checkout, holding the rules under
#                                   writing/ and the templates beside them
#   <user vale dir>/styles/         symlinks to the rules, so vale finds them
#
# It writes no .vale.ini. See the note at the top.
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
# main is a branch, not a pin. Passing a tag gets a fixed ruleset; the rendered
# workflow and config then record it, and a rule change reaches the consumer only
# when they raise it. Left as main because a consumer with no tag still needs a
# working install, and because `git clone --branch` below takes a branch or a tag
# and not a commit, so resolving this to a SHA would record a ref this installer
# cannot itself consume.
REF="${AGENTIC_WRITING_REF:-main}"
DRY_RUN=false

usage() {
  cat <<'USAGE'
install.sh — the agentic-writing toolkit.

  install.sh                 install on this machine (the usual case)
  install.sh repo            additionally wire the current repository into CI
  install.sh --dry-run       report what would happen, write nothing
  install.sh --help          this text

A machine install links the rules; a user-level .vale.ini has to select them,
and this script writes none. Repo mode installs a working configuration. Once a
configuration is in place, which rules run depends on the directory a file sits
in:

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
render() {  # render TEMPLATE DEST — substitute the ref this install used
  # $REF reaches sed as a replacement, where `\`, `&` and `|` are not literal.
  # A ref holding `&` renders `@REF@` verbatim into the output; one holding `|`
  # closes the expression and sed fails after the redirection has already
  # truncated the destination. Both are valid in a Git ref name, and REF comes
  # from the environment. Same reasoning as the argv note above: this is input
  # the script does not control.
  if $DRY_RUN; then
    printf '  would: render %s -> %s\n' "$1" "$2"
  else
    local escaped
    escaped="$(printf '%s' "$REF" | sed 's/[\\&|]/\\&/g')"
    sed "s|@REF@|$escaped|g" "$1" > "$2"
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

  # The rules live under writing/ here. This installer began in a repository whose
  # root was the toolkit, so every path that still assumes that layout points at
  # nothing -- and a symlink to a missing directory fails at lint time, in the
  # consumer's repository, not here.

  # macOS and Linux put the user Vale directory in different places.
  case "$(uname -s)" in
    Darwin) vale_dir="$HOME/Library/Application Support/vale" ;;
    *)      vale_dir="${XDG_CONFIG_HOME:-$HOME/.config}/vale" ;;
  esac
  run mkdir -p "$vale_dir/styles"

  # `-n` applies only when the destination is a symlink to a directory. Against
  # a real directory `ln` links *inside* it -- styles/config/config, exit 0, no
  # output -- and `-f` will not remove a directory to prevent that. Vale creates
  # <StylesPath>/config/vocabularies itself, so the user this collides with is
  # the one the next branch supports: the one who already has a Vale config.
  # `rm -rf` is too blunt against a directory they may own.
  #
  # `config` links the rules repository's Local vocabulary too, and this mode
  # grafts nothing over it. What keeps those names off every repository on the
  # machine is the user-level Vocab, which names AgenticWriting alone --
  # check-consumer-scoping.sh holds the reference configuration to that.
  for style in AgenticWriting config; do
    dest="$vale_dir/styles/$style"
    if [ -d "$dest" ] && [ ! -L "$dest" ]; then
      say "  $dest is a real directory, not a link this script owns." >&2
      say "    Move it aside and re-run. Linking into it would put the rules at" >&2
      say "    $dest/$style, where Vale does not look for them." >&2
      exit 2
    fi
    say "  linking $style into the user styles directory"
    run ln -sfn "$HOME_DIR/writing/styles/$style" "$dest"
  done

  # No fallback config is written. The reference this used to copy lives in the
  # standalone repository and was not carried across, so under `set -e` the cp
  # aborted the install outright for anybody who did not already have a
  # user-level .vale.ini -- which is most first-time users.
  if [ -f "$vale_dir/.vale.ini" ]; then
    say "  user Vale config exists, leaving it alone"
  else
    say "  no user Vale config; the styles are linked but nothing selects them"
    say "    write $vale_dir/.vale.ini naming the AgenticWriting styles you want,"
    say "    or use repo mode, which installs a configuration for a repository"
  fi

  # `vale sync` reads Packages out of a config, so it needs one: with no
  # .vale.ini it stops at "no .vale.ini file found" and fetches nothing. Syncing
  # is worth doing only for a user who already has a config -- without one the
  # first `vale` run has no rules selected, so a missing package is not yet what
  # stands between them and a working setup.
  if [ -f "$vale_dir/.vale.ini" ]; then
    # Without this the first `vale` run fails with "style 'write-good' does not
    # exist". Repo mode already syncs; machine mode did not, and the gap hid on
    # a machine whose styles directory was already populated.
    say "  fetching the packages your config declares"
    if ! $DRY_RUN; then
      ( cd "$vale_dir" && vale sync >/dev/null 2>&1 ) || \
        say "    note: 'vale sync' failed. Run it in $vale_dir to see why."
    fi
  else
    say "  skipping the package fetch; there is no config yet to declare any"
  fi

  say ""
  say 'Done. The styles are linked; a user-level .vale.ini selects them.'
  say ""
  say "Once a config selects them, which rules run depends on where a file sits:"
  say "  specs/<feature>/spec.md   spec structure, RFC 2119 casing, prose"
  say "  docs/adr/NNNN-name.md     Nygard sections, prose, no sentence cap"
  say "  docs/design/name.md       design sections, prose, no sentence cap"
  say "  tickets/id.md             the seven ticket sections, prose"
  say ""
  say "Starting a document:"
  say "  $HOME_DIR/writing/templates/"
  say ""
  say "CI for a whole team is a separate, optional step:"
  say "  cd <repo> && curl -fsSL $RAW/main/writing/scripts/install.sh | bash -s -- repo"
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

  # What this run wrote, what it had already installed, and what belongs to the
  # repository. The closing summary is built from these rather than a fixed list.
  #
  # A file already installed by this toolkit is not the same as a file the
  # repository brought: re-running is invited, and reporting that the checks are
  # unwired on a correctly installed repository is as wrong as the fixed list
  # was. Each template leaves a marker, so the two cases are told apart by
  # reading the file rather than by its existence.
  wrote=""
  mine=""
  theirs=""

  if [ ! -f "$root/.vale.ini" ]; then
    say "  writing .vale.ini"
    render "$src/writing/templates/consumer.vale.ini" "$root/.vale.ini"
    wrote="$wrote .vale.ini"
  elif grep -q 'installs from:' "$root/.vale.ini"; then
    say "  .vale.ini is this toolkit's, leaving it alone"
    mine="$mine .vale.ini"
  else
    say "  .vale.ini is this repository's own, leaving it alone"
    say "    compare against $src/writing/templates/consumer.vale.ini"
    theirs="$theirs .vale.ini"
  fi

  wf="$root/.github/workflows/writing.yml"
  if [ -f "$wf" ] && grep -q 'sei-internal-skills/.github/workflows/writing-contract.yml@' "$wf"; then
    say "  writing.yml is this toolkit's, leaving it alone"
    mine="$mine .github/workflows/writing.yml"
  elif [ -f "$wf" ]; then
    say "  writing.yml is this repository's own, leaving it alone"
    say "    compare against $src/writing/templates/writing.yml"
    say "    it needs the 'uses:' call to writing-contract.yml to run the checks"
    theirs="$theirs .github/workflows/writing.yml"
  else
    say "  writing .github/workflows/writing.yml"
    run mkdir -p "$root/.github/workflows"
    render "$src/writing/templates/writing.yml" "$wf"
    wrote="$wrote .github/workflows/writing.yml"
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
  # This repository's own accepted terms. Vale reads a vocabulary from
  # StylesPath/config/vocabularies, which the fetch above overwrites, so the
  # committed copy lives outside it and gets installed into place. CI does the
  # same. Vale errors on a Vocab it cannot find, so the file always exists.
  run mkdir -p "$root/.vale/vocab"
  # No template to compare against, and an existing one stops nothing, so it is
  # named only when this run creates it.
  if [ ! -f "$root/.vale/vocab/accept.txt" ]; then
    wrote="$wrote .vale/vocab/accept.txt"
  fi
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
  # THE DELETE IS THE BOUNDARY, NOT THE COPY. The fetched tree carries a Local
  # vocabulary of its own -- the rules repository's agent and skill identifiers,
  # which Vale reads only from StylesPath/config/vocabularies and so cannot keep
  # anywhere else. Vale.Terms would impose their casing on this repository's
  # prose. Overwriting accept.txt alone left that to statement order.
  run rm -rf "$root/.vale/styles/config/vocabularies/Local"
  run mkdir -p "$root/.vale/styles/config/vocabularies/Local"
  run cp "$root/.vale/vocab/accept.txt" \
        "$root/.vale/styles/config/vocabularies/Local/accept.txt"

  if ! $DRY_RUN; then
    ( cd "$root" && vale sync >/dev/null 2>&1 ) || \
      say "    note: 'vale sync' did not run. Run it to fetch write-good and proselint."
  fi

  if ! grep -qxF '.vale/styles/' "$root/.gitignore" 2>/dev/null; then
    wrote="$wrote .gitignore"
    say "  adding the fetched paths to .gitignore"
    append "$root/.gitignore" '
# agentic-writing fetches the rules into these. Anything else under .vale/ is
# yours, including .vale/vocab/accept.txt.
.vale/styles/
.vale/src/
'
  fi

  say ""
  if [ -n "$wrote" ]; then
    say "Commit these:"
    for f in $wrote; do say "  $f"; done
  fi
  if [ -n "$mine" ]; then
    say "Already installed, and refreshed by this run:"
    for f in $mine; do say "  $f"; done
  fi
  if [ -n "$theirs" ]; then
    say "Left alone, because this repository has its own:"
    for f in $theirs; do say "  $f"; done
    say "  The checks do not run until these match the templates above."
  else
    say "  The checks run for everyone on every branch."
  fi
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
