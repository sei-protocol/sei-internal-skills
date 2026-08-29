#!/usr/bin/env bash
# Generate the approved-word substitution rule from the OpenSTE wordset.
#
# Why this script exists: the ASD-STE100 dictionary is copyrighted and MUST NOT be
# vendored here. OpenSTE is MIT-licensed, so its wordset can be fetched and used to
# build an approximation. See writing/NOTICE.md.
#
# Usage: writing/scripts/sync-openste.sh
set -euo pipefail

SRC="${OPENSTE_URL:-https://raw.githubusercontent.com/openste/openste/main/wordset.json}"
# Anchored to this script, like every other in the toolkit. It used to be
# relative to the working directory, so running it from the repository root —
# which its own usage line asks for — wrote nowhere.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$ROOT/styles/AgenticWriting/STE-ApprovedWords.generated.yml"
TMP="$(mktemp)"

echo "Fetching OpenSTE wordset from ${SRC}"
curl -fsSL "${SRC}" -o "${TMP}"

python3 - "${TMP}" "${OUT}" <<'PY'
import json, sys, pathlib

src, out = sys.argv[1], sys.argv[2]
data = json.load(open(src))

# The upstream shape may change. Adapt this extraction, do not assume it.
# Expected: a mapping or list that pairs a non-approved word with an approved one.
pairs = {}
if isinstance(data, dict):
    for k, v in data.items():
        if isinstance(v, str):
            pairs[k] = v
        elif isinstance(v, dict) and v.get("approved") is False and v.get("alternatives"):
            pairs[k] = v["alternatives"][0]
elif isinstance(data, list):
    for e in data:
        if isinstance(e, dict) and e.get("word") and e.get("replacement"):
            pairs[e["word"]] = e["replacement"]

if not pairs:
    sys.exit("No pairs extracted. Inspect the upstream schema and update this script.")

lines = [
    "# GENERATED FILE. Run scripts/sync-openste.sh.",
    "# Derived from the MIT-licensed OpenSTE wordset: https://github.com/openste/openste",
    "# This is NOT the ASD-STE100 dictionary. See writing/NOTICE.md.",
    "extends: substitution",
    'message: "Use \'%s\' instead of \'%s\' (OpenSTE-derived approved word)."',
    "level: warning",
    "ignorecase: true",
    "swap:",
]
for bad, good in sorted(pairs.items()):
    lines.append(f'  "{bad}": "{good}"')

pathlib.Path(out).write_text("\n".join(lines) + "\n")
print(f"Wrote {out} with {len(pairs)} pairs.")
PY

rm -f "${TMP}"
echo "Now add AgenticWriting.STE-ApprovedWords.generated to .vale.ini and run: vale ls-config"
