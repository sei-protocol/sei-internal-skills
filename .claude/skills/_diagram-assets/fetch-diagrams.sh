#!/usr/bin/env bash
# Export every catalog skill's Lucid diagram to its assets/<skill>.png.
#
# Source of truth: MANIFEST.json (skill -> lucidDocId). Pulls a read-only PNG via
# the Lucid REST API (GET /documents/{id}, Accept: image/png) using a Lucid API
# KEY stored in the macOS Keychain (service "lucid-api-key"). The key is read
# straight into curl's stdin-config (-K -) so it is NEVER placed in argv (invisible
# to `ps`) and never printed. Run it again any time the diagrams change.
#
# Usage:   bash .claude/skills/_diagram-assets/fetch-diagrams.sh
#   DPI=256 bash .../fetch-diagrams.sh     # higher-res export (default 144)
#
# Get a key: https://lucid.app/developer#/apikeys  ->  Create API Key
#   grant DocumentReadonly, copy it, then store it (you run this; the key never
#   passes through anything but Keychain):
#       security add-generic-password -a "$USER" -s lucid-api-key -U -w
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILLS_DIR="$(cd "$HERE/.." && pwd)"
MANIFEST="$HERE/MANIFEST.json"
KC_SERVICE="lucid-api-key"
DPI="${DPI:-144}"
API="https://api.lucid.co/documents"

[[ -f "$MANIFEST" ]] || { echo "FATAL: $MANIFEST not found"; exit 2; }
command -v python3 >/dev/null 2>&1 || { echo "FATAL: python3 required (parses MANIFEST.json)"; exit 2; }

# --- ensure the key exists in Keychain (offer to store it if missing) ----------
if ! security find-generic-password -s "$KC_SERVICE" -w >/dev/null 2>&1; then
  echo "No Keychain entry '$KC_SERVICE' found."
  echo "Generate one at https://lucid.app/developer#/apikeys (grant: DocumentReadonly)."
  read -r -p "Store it now? You'll be prompted for the key (hidden). [y/N] " ans
  [[ "${ans:-}" == [yY]* ]] || { echo "Aborting. Store it, then re-run."; exit 1; }
  security add-generic-password -a "$USER" -s "$KC_SERVICE" -U -w   # hidden prompt; not echoed
fi

# --- skill -> docId pairs (only skills that have a diagram) ---------------------
pairs="$(python3 -c "import json,sys
d=json.load(open('$MANIFEST'))['diagrams']
for k,v in d.items():
    if v.get('lucidDocId'): print(k, v['lucidDocId'])
")"

total=0; ok=0; fail=0; failed=()
while read -r skill id; do
  [[ -n "$skill" ]] || continue
  total=$((total+1))
  out_dir="$SKILLS_DIR/$skill/assets"
  out="$out_dir/$skill.png"
  mkdir -p "$out_dir"
  # key -> curl via stdin config (-K -): never in argv, never printed
  code="$(printf 'header = "Authorization: Bearer %s"\n' \
            "$(security find-generic-password -s "$KC_SERVICE" -w)" \
          | curl -sS -K - \
              -H "Accept: image/png;dpi=$DPI" \
              -H "Lucid-Api-Version: 1" \
              -w '%{http_code}' -o "$out" \
              "$API/$id?page=1" || echo "000")"
  if [[ "$code" == "200" && -s "$out" ]]; then
    sz="$(wc -c < "$out" | tr -d ' ')"
    printf '  ok   %-18s %8s bytes  (%s)\n' "$skill" "$sz" "$id"
    ok=$((ok+1))
  else
    rm -f "$out"               # drop any error-body written to the .png
    printf '  FAIL %-18s http=%s  (%s)\n' "$skill" "$code" "$id"
    failed+=("$skill:$code"); fail=$((fail+1))
  fi
done <<< "$pairs"

echo
echo "exported $ok/$total  (failed: $fail)"
(( fail )) && printf '  failures: %s\n' "${failed[*]}"

echo
echo "--- assertion ---"
bash "$HERE/assert-diagrams.sh" || true
echo
echo "Next: cd \"$SKILLS_DIR/../..\" && git add .claude/skills/*/assets/*.png && git commit"
