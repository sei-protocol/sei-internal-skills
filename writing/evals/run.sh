#!/usr/bin/env bash
# Rule regression test: does each rule still find what it must find?
# A rule that stops firing is a silent failure, and silent failures are the reason
# a prompt-only approach cannot be trusted.
#
# The root is anchored to this script. Run from anywhere else it used to die on
# `fixtures[@]: unbound variable`, which is loud but says nothing, and an empty
# fixture list is a failure rather than a clean run over nothing.
set -euo pipefail
# RUN FROM THE REPOSITORY ROOT. .vale.ini scopes a rule by path, and Vale matches
# that glob against the path it is handed. Run from writing/ and a fixture reaches
# Vale as evals/fixtures/..., matching no mode section, so every structure rule
# stays off and the harness reports a fixture that fires nothing.
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

command -v vale >/dev/null || { echo "vale not on PATH: https://vale.sh/docs"; exit 2; }
command -v jq   >/dev/null || { echo "jq not on PATH"; exit 2; }

fail=0

# Fixtures nest: a rule scoped to a path in .vale.ini needs a fixture on that
# path. evals/fixtures/specs/ is one, because RFC 2119 is specification-scoped.
fixtures=()
while IFS= read -r f; do fixtures+=("$f"); done \
  < <(find writing/evals/fixtures -type f -name '*.md' | sort)

if [ "${#fixtures[@]}" -eq 0 ]; then
  echo "no fixtures under writing/evals/fixtures — refusing to report success on an empty set"
  exit 1
fi

for fixture in "${fixtures[@]}"; do
  # Mirror the fixture's path under writing/evals/expected, so two fixtures in different
  # mode directories can share a name without silently sharing an expectation.
  rel="${fixture#writing/evals/fixtures/}"
  base="${rel%.md}"
  expected="writing/evals/expected/${base}.json"
  [ -f "$expected" ] || { echo "no expectation for ${fixture}"; fail=1; continue; }

  found="$(vale --no-global --output=JSON "$fixture" || true)"
  rules="$(echo "$found" | jq -r '.[][] | .Check' | sort -u)"
  count="$(echo "$found" | jq '[.[][]] | length')"

  while read -r want; do
    [ -z "$want" ] && continue
    if ! echo "$rules" | grep -qx "$want"; then
      echo "MISS ${base}: expected rule ${want} did not fire"
      fail=1
    fi
  done < <(jq -r '.must_include_rules[]' "$expected")

  # A presence rule that fires on a complete document is worse than no rule.
  # A fixture may name the rules that must stay silent on it.
  while read -r unwanted; do
    [ -z "$unwanted" ] && continue
    if echo "$rules" | grep -qx "$unwanted"; then
      echo "FIRED ${base}: rule ${unwanted} fired but must stay silent"
      fail=1
    fi
  done < <(jq -r '.must_not_include_rules // [] | .[]' "$expected")

  min="$(jq -r '.min_findings' "$expected")"
  if [ "$count" -lt "$min" ]; then
    echo "LOW  ${base}: ${count} findings, expected at least ${min}"
    fail=1
  fi
done

[ "$fail" -eq 0 ] && echo "All fixture expectations met."
exit "$fail"
