#!/usr/bin/env bash
# Lint prose against the writing contract. This is the one definition of how the
# gate runs, and both CI and a person invoke it.
#
#   writing/scripts/lint.sh                 # everything the contract governs
#   writing/scripts/lint.sh <path>...       # just these
#   writing/scripts/lint.sh --print-flags   # the flags, for a caller that needs them
#
# THE EXCLUSIONS ARE WHY THIS FILE EXISTS. Each names a tree that holds
# non-conforming prose on purpose, so linting it reports errors by design:
#
#   writing/styles/write-good/**      a fetched package's own documentation
#   writing/evals/fixtures/**         fixtures that have to produce findings
#   writing/evals/rules/**            each golden directory carries its own config
#   writing/evals/consumer/tree/**    linted by the consumer eval, against a golden
#   writing/templates/spec-*.upstream.md  the unedited fork parent
#
# That list used to live in the workflow, in README.md, and in a success
# criterion, and two of the three had already drifted -- one said "two paths"
# beside a command naming five, another said the lint skips `evals/` when it
# skips three subtrees of it. A criterion naming `vale ...` directly is also a
# claim no gate can check: check-verifiers.sh skips any argument holding a glob
# character, so the exclusions were unverifiable by construction. Naming this
# script instead gives the gate a path it can resolve.
set -euo pipefail

GLOB='!{writing/styles/write-good/**,writing/evals/fixtures/**,writing/evals/rules/**,writing/evals/consumer/tree/**,writing/templates/spec-template.upstream.md}'

# --no-global: a runner has no user-level Vale config, and a laptop that has one
# would otherwise lint this repository through it.
FLAGS=(--no-global "--glob=$GLOB")

if [ "${1:-}" = "--print-flags" ]; then
  printf '%s\n' "${FLAGS[*]}"
  exit 0
fi

cd "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

if [ "$#" -gt 0 ]; then
  exec vale "${FLAGS[@]}" "$@"
fi
exec vale "${FLAGS[@]}" writing
