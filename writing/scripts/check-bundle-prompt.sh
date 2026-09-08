#!/usr/bin/env bash
# The sei-spec bundle prompt carries the writing contract. This holds it to the registry.
#
# WHY A THIRD COPY EXISTS AT ALL. An Omnigent bundle has no guidelines file and no
# skill the host resolves: `agents/sei-spec/config.yaml` names `skills: none`, and its
# prompt is the only text that reaches the model. So the contract sits inside that
# prompt as literal words, beside CONTRACT.md and CONTEXT.md. Generation is what keeps
# the three in step, and this gate is what makes generation binding.
#
# Two assertions, because the copy can fail two ways:
#
#   1. DRIFT. The section in config.yaml matches
#      `render-context.py --target bundle-prompt` byte for byte. A hand edit here is
#      worse than a hand-written prompt: a registry change silently reverts it, and a
#      paraphrase weakens the contract while still reading like it.
#   2. UNCHECKED PROSE. Vale reads Markdown and lints no YAML at all, so a prompt is
#      the one artifact in this repository that states the contract and sits outside
#      every check of it. This lints the rendered section as the Markdown it is.
#
# Vale needs the section dedented. Two spaces of YAML block-scalar indent make the
# whole thing an indented code block, which Vale reads as code and skips in silence —
# a green run over nothing.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CONFIG="$ROOT/agents/sei-spec/config.yaml"
HEADING='  ## Hold every artifact to the writing contract'

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

python3 "$ROOT/writing/scripts/render-context.py" --target bundle-prompt > "$work/want.txt"

# The heading delimits the section, so the prompt needs no sentinel comment. A YAML
# comment cannot sit inside a block scalar: it would reach the model as literal text.
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

sed 's/^  //' "$work/want.txt" > "$work/section.md"
if ! vale --no-global --config="$ROOT/.vale.ini" --minAlertLevel=error "$work/section.md"; then
  echo
  echo "The rendered bundle prompt fails the contract it installs."
  echo "Fix the text in writing/scripts/render-context.py or writing/anchors/registry.yaml."
  exit 1
fi

echo "bundle-prompt-ok: the sei-spec prompt matches the registry and passes Vale"
