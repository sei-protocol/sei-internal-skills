#!/usr/bin/env bash
# Generate the approved-word substitution rule from the OpenSTE wordset.
#
# Why this script exists: the ASD-STE100 dictionary is copyrighted and MUST NOT be
# vendored here. OpenSTE is MIT-licensed, so its wordset can be fetched and used to
# build an approximation. See writing/NOTICE.md.
#
# IT WRITES A CANDIDATE, NOT AN INSTALLED RULE. The output goes to
# styles/openste/, which no BasedOnStyles names, so Vale ignores it exactly as it
# ignores styles/artifact/. Two reasons it cannot land in styles/AgenticWriting/:
# every rule file there is armed the moment it appears, and a name that keeps a
# second dot resolves to the rule name the hand-written seed already holds, which
# makes Vale refuse to load the configuration at all.
#
# Activating it is a vocabulary decision, not a mechanical step. The wordset is
# not a superset of the seed: the seed carries house entries the wordset has no
# opinion about. The closing message names that decision.
#
# Usage: writing/scripts/sync-openste.sh
set -euo pipefail

SRC="${OPENSTE_URL:-https://raw.githubusercontent.com/openste/openste/main/vocabulary/openste.json}"

# Anchored to this script, not to the caller's working directory. The output is a
# fixed destination inside the toolkit, so the script owns the path. A path a
# caller passes is a different thing, and lint.sh resolves those where they were
# typed.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$ROOT/styles/openste/STE-ApprovedWords.yml"

TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

echo "Fetching OpenSTE wordset from ${SRC}"
curl -fsSL --retry 3 --max-time 120 "${SRC}" -o "${TMP}"

mkdir -p "$(dirname "$OUT")"
python3 - "${TMP}" "${OUT}" <<'PY'
import json, pathlib, sys

src, out = sys.argv[1], sys.argv[2]
data = json.load(open(src))

# The upstream pairs a non-approved word with an approved one under a top-level
# `alternatives` list, each entry carrying `title` and `alt_title`. A title can
# repeat with a second alternative; the first wins, because a substitution rule
# offers one replacement.
#
# FAIL CLOSED ON A SHAPE THIS DOES NOT RECOGNISE. Pairing any top-level string
# value would turn `set_name` and `description` into substitutions, and the
# "nothing extracted" guard below would not fire on a set that holds those two.
alternatives = data.get("alternatives") if isinstance(data, dict) else None
if not isinstance(alternatives, list):
    sys.exit("No top-level 'alternatives' list. Inspect the upstream schema and update this script.")

pairs = {}
for entry in alternatives:
    if not isinstance(entry, dict):
        continue
    bad, good = entry.get("title"), entry.get("alt_title")
    if isinstance(bad, str) and isinstance(good, str) and bad and good:
        pairs.setdefault(bad, good)

if not pairs:
    sys.exit("No pairs extracted. Inspect the upstream schema and update this script.")

lines = [
    "# GENERATED FILE. Run writing/scripts/sync-openste.sh.",
    "# Derived from the MIT-licensed OpenSTE wordset: https://github.com/openste/openste",
    "# This is NOT the ASD-STE100 dictionary. See writing/NOTICE.md.",
    "#",
    "# A CANDIDATE. Vale does not load this directory. See the script for what",
    "# activating it costs.",
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

cat <<'MSG'

.vale.ini needs no entry: BasedOnStyles names the AgenticWriting style, which
arms every rule file in it. This candidate sits outside that style on purpose.

To adopt it, diff it against styles/AgenticWriting/STE-ApprovedWords.yml and
move it over that file. Keep the seed entries the wordset does not carry, then
run: writing/scripts/lint.sh writing
MSG
