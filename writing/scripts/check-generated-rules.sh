#!/usr/bin/env bash
# The eighteen section rules are generated. This fails if they drift from their source.
#
# A generated file that someone edited by hand is worse than a hand-written one: the
# next run of the generator silently reverts it. The gate catches three shapes — a rule
# that differs from the manifest, a rule the manifest names that is missing, and a rule
# carrying the generated banner that the manifest no longer names.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
exec python3 "$ROOT/scripts/generate-mode-rules.py" --check
