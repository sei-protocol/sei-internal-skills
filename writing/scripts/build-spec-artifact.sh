#!/usr/bin/env bash
# Build a Markdown artifact from a specification in another repository.
#
# A published artifact is not a copy of spec.md. It is spec.md with the canonical
# heading style prepended, because the platform renders Markdown with its own
# stylesheet that sets a section heading below the contrast of the body text it
# heads. See writing/styles/artifact/heading-hierarchy.css and writing/docs/writing-modes.md.
#
# Publishing therefore has two inputs and a step between them, and this is that
# step. It exists so the step is the same every time rather than reconstructed
# from prose, which is what writing/docs/writing-modes.md alone asks a reader to do.
#
# Three things it does that a hand-assembled artifact gets wrong:
#
#   1. It reads the specification from git, not from the working tree, so an
#      uncommitted edit cannot reach a published artifact.
#   2. It reads the style from the canonical file rather than reproducing it, so
#      the artifact cannot drift from writing/styles/artifact/heading-hierarchy.css.
#   3. It takes the feature name as an argument, so every specification goes
#      through identical steps.
#
# It writes files. It does not publish them: publishing is a claude.ai call the
# caller makes, with the artifact's existing URL so the link stays stable.
#
# Usage:
#   writing/scripts/build-spec-artifact.sh --repo PATH --ref GITREF --out DIR FEATURE...
#
# Example:
#   writing/scripts/build-spec-artifact.sh \
#     --repo ~/sei-load --ref brandon2/spec-contract-deployment-registry \
#     --out /tmp/artifacts contract-deployment-registry transaction-outcome-tracking
set -euo pipefail

# DERIVED, NOT RESTATED. The header comment above is the one copy of the usage,
# so --help cannot drift from what a reader of this file sees.
usage() {
  awk 'NR==1 {next} /^set -euo pipefail$/ {exit} {sub(/^# ?/, ""); print}' \
    "${BASH_SOURCE[0]}"
}

STYLE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CSS="$STYLE_DIR/styles/artifact/heading-hierarchy.css"

repo="" ref="" out=""
while [ $# -gt 0 ]; do
  case "$1" in
    --repo) repo="${2:?--repo needs a path}"; shift 2 ;;
    --ref)  ref="${2:?--ref needs a git ref}"; shift 2 ;;
    --out)  out="${2:?--out needs a directory}"; shift 2 ;;
    --help|-h) usage; exit 0 ;;
    --) shift; break ;;
    -*) echo "unknown flag: $1. Run --help." >&2; exit 2 ;;
    *) break ;;
  esac
done

for required in repo ref out; do
  if [ -z "${!required}" ]; then
    echo "missing --$required. Run --help." >&2
    exit 2
  fi
done
if [ $# -eq 0 ]; then
  echo "name at least one feature directory under specs/" >&2
  exit 2
fi
if [ ! -f "$CSS" ]; then
  echo "canonical style missing: $CSS" >&2
  exit 1
fi

mkdir -p "$out"
for feature in "$@"; do
  src="$ref:specs/$feature/spec.md"
  if ! git -C "$repo" cat-file -e "$src" 2>/dev/null; then
    echo "not in $ref: specs/$feature/spec.md" >&2
    exit 1
  fi
  target="$out/$feature.md"
  {
    printf '<style>\n'
    cat "$CSS"
    printf '</style>\n\n'
    git -C "$repo" show "$src"
  } > "$target"
  printf '%s  %s lines  from %s\n' "$target" "$(wc -l < "$target" | tr -d ' ')" "$ref"
done
