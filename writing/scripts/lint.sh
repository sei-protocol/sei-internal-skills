#!/usr/bin/env bash
# Lint prose against the writing contract. This is the one definition of what the
# gate covers and how it runs, and both CI and a person invoke it.
#
#   writing/scripts/lint.sh                 # everything the contract governs
#   writing/scripts/lint.sh <path>...       # just these, read where you typed them
#   writing/scripts/lint.sh --print-flags   # the flags, for the workflow
#   writing/scripts/lint.sh --print-paths   # the paths as JSON, for the workflow
#
# THE EXCLUSIONS ARE WHY THIS FILE EXISTS. Each entry in EXCLUDED names a tree
# that holds non-conforming prose on purpose, so linting it reports errors by
# design, and each carries its reason on the same line. The list has one copy and
# the glob is built from it, so an entry cannot drift from its reason.
#
# The list used to sit in the workflow, in README.md and in a success criterion,
# and two of the three disagreed. A criterion naming `vale ...` directly is also
# a claim no gate can check: check-verifiers.sh skips any argument holding a glob
# character. Naming this script gives the gate a path it can resolve.
set -euo pipefail

EXCLUDED=(
  'writing/styles/write-good/**'                 # a fetched package's own documentation
  'writing/evals/fixtures/**'                    # fixtures that have to produce findings
  'writing/evals/rules/**'                       # each golden directory carries its own config
  'writing/evals/consumer/tree/**'               # linted by the consumer eval, against a golden
  'writing/templates/spec-template.upstream.md'  # the unedited fork parent
)

# ONE --glob, and no space inside the braces. Vale takes a single glob expression
# and keeps the last occurrence, and the workflow splits this output on
# whitespace to build its own flags.
GLOB="!{$(IFS=','; printf '%s' "${EXCLUDED[*]}")}"

# THE SAME REASON HOLDS FOR THE PATHS. The workflow derives the action's `files:`
# input from --print-paths, so CI and a person running this with no arguments
# cover one set of paths. A list restated in the workflow can name its own paths
# beside a script that defaults to something narrower, and a contributor then
# passes locally and fails the gate on a file the documented command never read.
#
# scripts/ is here for sei-internal-skills-doctrine.md, the file that fans out
# into every consuming repository's AGENTS.md.
#
# sei-agent-driver/ is out, and it is the only tracked prose that is. It is a
# separate Go module with its own go.mod, and its two README files are package
# documentation that installs the contract on its own terms.
#
# Bare paths, no quote and no backslash: --print-paths writes them into a JSON
# array without escaping them.
SCOPED=(
  README.md
  AGENTS.md
  CLAUDE.md
  agents
  .claude
  experimental
  scripts
  writing
)

# --no-global: a runner has no user-level Vale config, and a laptop that has one
# would otherwise lint this repository through it.
FLAGS=(--no-global "--glob=$GLOB")

case "${1:-}" in
  --print-flags)
    printf '%s\n' "${FLAGS[*]}"
    exit 0
    ;;
  --print-paths)
    # A JSON ARRAY, NOT A BARE LIST. The action resolves a single existing path
    # directly and parses anything else as JSON, so a comma-separated string
    # reaches JSON.parse -- and a parse failure there warns and lints `.`
    # instead of failing, which would widen the gate in silence.
    #
    # Same shape as consumer-lint-setup.sh, which builds the same input for the
    # reusable workflow.
    json=''
    for entry in "${SCOPED[@]}"; do
      json="${json:+$json,}\"$entry\""
    done
    printf '[%s]\n' "$json"
    exit 0
    ;;
esac

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

if [ "$#" -eq 0 ]; then
  cd "$REPO_ROOT"
  exec vale "${FLAGS[@]}" "${SCOPED[@]}"
fi

# A PATH ARGUMENT IS READ WHERE THE CALLER TYPED IT. The lint runs from the
# repository root, so resolving after the move read `README.md` from the root for
# a caller standing in writing/ -- the wrong file, and a clean exit whenever the
# root copy happened to be clean.
#
# Root-relative, never absolute: Vale matches the exclusions above against the
# argument string, so an absolute path lints an excluded tree. Vale resolves a
# `..` segment itself, so a prefix strip is enough and no normaliser is needed.
targets=()
for arg in "$@"; do
  case "$arg" in
    -*)
      targets+=("$arg")
      continue
      ;;
    /*) resolved="$arg" ;;
    *)  resolved="$PWD/$arg" ;;
  esac
  # Vale reads standard input when a path does not exist, and reports success on
  # it, so a mistyped argument passed having linted nothing.
  if [ ! -e "$resolved" ]; then
    printf 'lint.sh: no such file or directory: %s\n' "$arg" >&2
    exit 2
  fi
  case "$resolved" in
    "$REPO_ROOT"/*) targets+=("${resolved#"$REPO_ROOT"/}") ;;
    *)              targets+=("$resolved") ;;
  esac
done

cd "$REPO_ROOT"
exec vale "${FLAGS[@]}" "${targets[@]}"
