#!/usr/bin/env bash
# The sei-spec bundle prompt carries the writing contract. This diffs it against the
# registry and lints it. Vale reads Markdown and lints no YAML at all, so
# `vale agents/sei-spec/config.yaml` reports zero findings in zero files.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CONFIG="$ROOT/agents/sei-spec/config.yaml"
HEADING='  ## Hold every artifact to the writing contract'

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

python3 "$ROOT/writing/scripts/render-context.py" --target bundle-prompt > "$work/want.txt"

# A sentinel comment inside a YAML block scalar would reach the model as literal
# text, so the heading is what delimits the section.
HEADING="$HEADING" python3 - "$CONFIG" > "$work/have.txt" <<'PY'
import os, sys
heading = os.environ["HEADING"]
lines = open(sys.argv[1]).read().split("\n")
try:
    start = lines.index(heading)
except ValueError:
    sys.exit(f"{sys.argv[1]} has no line {heading!r}: the contract is not in the prompt")
out = [lines[start]]
for line in lines[start + 1:]:
    if line.startswith("  ## "):
        break
    out.append(line)
while out and not out[-1].strip():
    out.pop()
print("\n".join(out))
PY

if ! diff -u "$work/want.txt" "$work/have.txt"; then
  echo
  echo "agents/sei-spec/config.yaml has drifted from writing/anchors/registry.yaml."
  echo "Regenerate the section and paste it back:"
  echo "  python3 writing/scripts/render-context.py --target bundle-prompt"
  exit 1
fi

# Two spaces of block-scalar indent make the whole section an indented code block,
# which Vale reads as code and skips in silence.
sed 's/^  //' "$work/want.txt" > "$work/section.md"
if ! vale --no-global --config="$ROOT/.vale.ini" --minAlertLevel=error "$work/section.md"; then
  echo
  echo "The rendered bundle prompt fails the contract it installs."
  echo "Fix the text in writing/scripts/render-context.py or writing/anchors/registry.yaml."
  exit 1
fi

echo "bundle-prompt-ok: the sei-spec prompt matches the registry and passes Vale"
