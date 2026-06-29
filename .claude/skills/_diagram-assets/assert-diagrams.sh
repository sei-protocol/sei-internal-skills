#!/usr/bin/env bash
# Asserts each catalog skill's diagram is created AND used:
#   (a) the skill's README.md references assets/<skill>.png
#   (b) that PNG file actually exists on disk
# Source of truth: MANIFEST.json (skills whose lucidDocId is non-null have a diagram).
# RED until the 28 PNGs are dropped into each skill's assets/ dir; GREEN once present.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
skills_dir="$(cd "$here/.." && pwd)"
manifest="$here/MANIFEST.json"

[[ -f "$manifest" ]] || { echo "FATAL: $manifest not found"; exit 2; }

missing_readme=(); missing_ref=(); missing_png=(); ok=()

# skills with a diagram = lucidDocId != null
while IFS= read -r name; do
  readme="$skills_dir/$name/README.md"
  png="$skills_dir/$name/assets/$name.png"
  if [[ ! -f "$readme" ]]; then missing_readme+=("$name"); continue; fi
  if ! grep -q "assets/$name.png" "$readme"; then missing_ref+=("$name"); continue; fi
  if [[ ! -f "$png" ]]; then missing_png+=("$name"); continue; fi
  ok+=("$name")
done < <(python3 -c "import json,sys; d=json.load(open('$manifest')); [print(k) for k,v in d['diagrams'].items() if v.get('lucidDocId')]")

total=$(( ${#ok[@]} + ${#missing_readme[@]} + ${#missing_ref[@]} + ${#missing_png[@]} ))
echo "diagram-assert: ${#ok[@]}/$total skills fully satisfied (README references an existing assets/<skill>.png)"
fail=0
if (( ${#missing_readme[@]} )); then echo "  MISSING README:        ${missing_readme[*]}"; fail=1; fi
if (( ${#missing_ref[@]} ));    then echo "  README lacks img ref:  ${missing_ref[*]}";    fail=1; fi
if (( ${#missing_png[@]} ));     then echo "  MISSING PNG (drop in assets/): ${missing_png[*]}"; fail=1; fi
exit $fail
