#!/usr/bin/env bash
# Lint prose against the writing contract. This is the one definition of how the
# gate runs, and both CI and a person invoke it.
#
#   writing/scripts/lint.sh                 # everything the contract governs
#   writing/scripts/lint.sh <path>...       # just these, read where you typed them
#   writing/scripts/lint.sh --print-flags   # the flags, for a caller that needs them
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

# --no-global: a runner has no user-level Vale config, and a laptop that has one
# would otherwise lint this repository through it.
FLAGS=(--no-global "--glob=$GLOB")

if [ "${1:-}" = "--print-flags" ]; then
  printf '%s\n' "${FLAGS[*]}"
  exit 0
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

if [ "$#" -eq 0 ]; then
  cd "$REPO_ROOT"
  exec vale "${FLAGS[@]}" writing
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
