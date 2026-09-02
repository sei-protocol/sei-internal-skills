#!/usr/bin/env bash
# Prepare a consuming repository for a lint run, and say which paths to lint.
#
#   scripts/consumer-lint-setup.sh <fetched-rules-dir> [paths-json]
#
# THE REUSABLE WORKFLOW AND ITS TEST CALL THIS SAME SCRIPT. The logic used to
# live inline in .github/workflows/writing-contract.yml, where nothing could run
# it: that workflow is the one every consumer depends on and the one thing in
# this repository no job had ever invoked. Inline shell in a workflow is
# reachable only by merging and hoping.
#
# It does two things the caller cannot skip:
#
#   1. Installs the fetched styles, then grafts the caller's own accepted terms
#      into config/vocabularies/Local. The copy replaces everything inside the
#      fetched tree, so a vocabulary kept inside it would not survive.
#   2. Chooses the paths. Vale treats a named path that does not exist as a
#      fatal argument error rather than as nothing to lint, and almost no
#      repository has every document directory.
#
# Writes `files=<json>` to $GITHUB_OUTPUT when that is set, and always prints the
# JSON on stdout so a test can read it without GitHub.
set -euo pipefail

SRC="${1:?usage: consumer-lint-setup.sh <fetched-rules-dir> [paths-json]}"
GIVEN="${2:-}"

[ -d "$SRC/styles/AgenticWriting" ] || {
  echo "no styles at $SRC/styles/AgenticWriting — the rules were not fetched" >&2
  exit 1
}

# This installs into .vale/styles, and Vale reads whatever StylesPath the
# consumer's own .vale.ini names -- a file this contract explicitly does not own
# or overwrite. When the two disagree the rules land where Vale never looks, and
# the job dies on a Vale runtime error instead of reporting a finding. Checked
# rather than assumed, and fail-closed: an unreadable or absent declaration is a
# mismatch, because guessing wrong is the silent failure being prevented.
declared="$(sed -n 's/^[[:space:]]*StylesPath[[:space:]]*=[[:space:]]*//p' .vale.ini 2>/dev/null \
              | head -1 | tr -d '\r' | sed 's/[[:space:]]*$//')"
if [ "$declared" != ".vale/styles" ]; then
  echo "StylesPath in .vale.ini is '${declared:-<unset>}', and this workflow installs" >&2
  echo "the rules into .vale/styles. Vale would read the wrong tree." >&2
  echo "Set 'StylesPath = .vale/styles', or drop the reusable workflow and run vale yourself." >&2
  exit 1
fi

mkdir -p .vale/styles
rm -rf .vale/styles/AgenticWriting .vale/styles/config
cp -R "$SRC/styles/AgenticWriting" .vale/styles/
cp -R "$SRC/styles/config" .vale/styles/

# The caller's own accepted terms, committed outside the fetched tree.
mkdir -p .vale/styles/config/vocabularies/Local
if [ -f .vale/vocab/accept.txt ]; then
  cp .vale/vocab/accept.txt .vale/styles/config/vocabularies/Local/accept.txt
else
  : > .vale/styles/config/vocabularies/Local/accept.txt
fi

# The config declares Packages, and those are not committed anywhere. They are
# gitignored in agentic-writing, so the checkout that fetches the rules does not
# carry them, and a consumer's runner has never run install.sh. Without this,
# Vale stops with "style 'write-good' does not exist on StylesPath" before it
# reads a single document — the whole check fails as a runtime error rather than
# as a finding. The same step exists in this repository's own CI, where the
# comment beside it says exactly this. It was in one workflow and not the other.
if command -v vale >/dev/null 2>&1; then
  vale sync >&2 || {
    echo "vale sync failed; the declared packages are missing" >&2
    exit 1
  }
else
  echo "vale is not on PATH; install it before this script" >&2
  exit 1
fi

# A NAMED PATH IS FILTERED TOO. Vale treats a path that does not exist as a fatal
# argument error, so the default list is built from what is there. The caller's
# own list used to skip that filter, which made naming a tree you plan to have —
# docs/, designs/ — a hard failure rather than a no-op. Naming one is now safe
# and the skip is reported, so a typo is visible without breaking the run.
if [ -n "$GIVEN" ]; then
  files="$(GIVEN="$GIVEN" python3 - <<'PYEOF'
import json, os, sys
try:
    want = json.loads(os.environ['GIVEN'])
except json.JSONDecodeError as e:
    sys.stderr.write(f"paths input is not JSON: {e}\n")
    sys.exit(2)
if not isinstance(want, list) or not all(isinstance(x, str) for x in want):
    sys.stderr.write("paths input must be a JSON array of strings\n")
    sys.exit(2)
have = [p for p in want if os.path.exists(p)]
for p in want:
    if p not in have:
        sys.stderr.write(f"  skipping '{p}': not in this repository yet\n")
print(json.dumps(have, separators=(',', ':')) if have else '')
PYEOF
)" || exit 2
  if [ -z "$files" ]; then
    echo "none of the caller's paths exist; nothing to lint" >&2
  else
    echo "Linting the caller's paths: $files" >&2
  fi
else
  found=""
  for p in README.md docs specs tickets designs; do
    [ -e "$p" ] || continue
    found="${found:+$found,}\"$p\""
  done
  if [ -z "$found" ]; then
    echo "no document paths found; nothing to lint" >&2
    files=""
  else
    files="[$found]"
    echo "Linting: $files" >&2
  fi
fi

[ -n "${GITHUB_OUTPUT:-}" ] && echo "files=$files" >> "$GITHUB_OUTPUT"
printf '%s\n' "$files"
