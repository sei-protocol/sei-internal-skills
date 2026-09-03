#!/usr/bin/env bash
# Fail if an /xreview review ledger does not satisfy the schema /xreview ships.
#
# This exists because the schema had five typed fields that nothing validated, and
# the first ledger written against it violated the contract four ways: bolded field
# names, free prose after a tokens-only field, no `Round:` line, and a path neither
# consumer candidate checks. A schema whose first instance fails it stops being
# believed.
#
# It also enforces the one guarantee the rubric lens has instead of an absence
# check: on a `skill-package` ledger, the rubric lens must cite at least one rule
# id that resolves to a row in skill-package-rubric.md. "A verdict citing no rule
# id is not a rubric review" was stated in eleven places and gated in none.
#
# Usage: verify-ledger.sh [path ...]   (default: .xreview/*.md)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUBRIC="$REPO_ROOT/.claude/skills/xreview/references/skill-package-rubric.md"
ERRORS=0
err() { printf '%-14s %s\n' "$1" "$2"; ERRORS=$((ERRORS + 1)); }

files=("$@")
if [[ ${#files[@]} -eq 0 ]]; then
  while IFS= read -r f; do files+=("$f"); done < <(
    { find "$REPO_ROOT/.xreview" -maxdepth 1 -name '*.md' 2>/dev/null || true; } | sort)
fi
[[ ${#files[@]} -eq 0 ]] && { echo "✓ verify-ledger: no ledger to check."; exit 0; }

for f in "${files[@]}"; do
  rel="${f#"$REPO_ROOT"/}"

  # Bolded field names look like the contract and do not match it. The gate reads
  # `^Field: token`, so `**State**: RESOLVED` is invisible to it.
  while IFS=: read -r ln _; do
    err "BOLD-FIELD" "$rel:$ln  a header field is bolded; the gate matches '^Field: token' literally"
  done < <(grep -nE '^\*\*(Round|State|OpenFindings|Convergence|Blinded|Dissenter)\*\*:' "$f" || true)

  rounds=$(grep -cE '^Round:[[:space:]]+[0-9]+$' "$f" || true)
  [[ "$rounds" -eq 0 ]] && err "NO-ROUND" "$rel  no 'Round: <integer>' line; round selection fails closed without it"

  # Every round needs its own complete typed block.
  for field in State OpenFindings Convergence Blinded Dissenter; do
    n=$(grep -cE "^${field}:[[:space:]]+" "$f" || true)
    [[ "$n" -lt "$rounds" ]] && err "MISSING-FIELD" "$rel  $rounds round(s) but $n '${field}:' line(s)"
  done

  while IFS= read -r v; do
    case "$v" in OPEN|RESOLVED|RESOLVED-WITH-ACCEPTED-RISK|OPEN-BLOCKED) ;;
      *) err "BAD-STATE" "$rel  State: '$v' is not one of OPEN|RESOLVED|RESOLVED-WITH-ACCEPTED-RISK|OPEN-BLOCKED" ;;
    esac
  done < <(sed -nE 's/^State:[[:space:]]+(.*)$/\1/p' "$f")

  # Tokens only. A prior-round split that this round resolved is `unanimous`;
  # explaining that on the same line is what the schema forbids.
  while IFS= read -r v; do
    case "$v" in unanimous|split) ;;
      *) err "BAD-CONVERGENCE" "$rel  Convergence: '$v' — tokens only (unanimous|split), never free prose" ;;
    esac
  done < <(sed -nE 's/^Convergence:[[:space:]]+(.*)$/\1/p' "$f")

  while IFS= read -r v; do
    case "$v" in yes|no) ;; *) err "BAD-BLINDED" "$rel  Blinded: '$v' — must be yes or no" ;; esac
  done < <(sed -nE 's/^Blinded:[[:space:]]+(.*)$/\1/p' "$f")

  while IFS= read -r v; do
    [[ -z "${v// }" ]] && err "EMPTY-DISSENTER" "$rel  Dissenter: is empty; unanimity without an assigned dissenter is consensus theater"
  done < <(sed -nE 's/^Dissenter:[[:space:]]+(.*)$/\1/p' "$f")

  # A passing terminal claims zero open findings. Saying both is a contradiction
  # the gate would otherwise read as a pass.
  paste -d' ' \
    <(sed -nE 's/^State:[[:space:]]+(.*)$/\1/p' "$f") \
    <(sed -nE 's/^OpenFindings:[[:space:]]+(.*)$/\1/p' "$f") \
  | while read -r st n; do
      case "$st" in
        RESOLVED|RESOLVED-WITH-ACCEPTED-RISK)
          [[ "$n" == "0" ]] || echo "CONTRADICTION $rel  State: $st requires OpenFindings: 0, found $n" ;;
        OPEN-BLOCKED)
          [[ "$n" =~ ^[0-9]+$ ]] && [[ "$n" -ge 1 ]] || echo "CONTRADICTION $rel  State: OPEN-BLOCKED requires OpenFindings >= 1, found $n" ;;
      esac
    done > "$REPO_ROOT/.ledger-contradictions.tmp"
  while read -r _ rest; do
    [[ -n "$rest" ]] && err "CONTRADICTION" "$rest"
  done < "$REPO_ROOT/.ledger-contradictions.tmp"
  rm -f "$REPO_ROOT/.ledger-contradictions.tmp"

  # The rubric lens has no absence check. A cited rule id is the only evidence it
  # ran, so on a skill-package ledger at least one must resolve to a rubric row.
  if grep -qE '^Class:[[:space:]]+skill-package$' "$f"; then
    ids=$(grep -oE '\b[A-Z][0-9]+\b' "$f" | sort -u || true)
    resolved=0
    for id in $ids; do grep -qE "^\| $id " "$RUBRIC" && resolved=$((resolved + 1)); done
    [[ "$resolved" -eq 0 ]] && err "NO-RULE-ID" "$rel  Class: skill-package but no cited rule id resolves to a rubric row — a verdict citing no rule id is not a rubric review"
  fi
done

if [[ "$ERRORS" -gt 0 ]]; then
  echo "✗ verify-ledger: $ERRORS finding(s) across ${#files[@]} ledger(s)."
  exit 1
fi
echo "✓ verify-ledger: ${#files[@]} ledger(s) conform."
