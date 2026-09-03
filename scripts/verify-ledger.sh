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
  # Both known ledger locations. docs/xreview/ is not a location any consumer gate
  # checks, which is precisely why ledgers there must not escape the linter — the
  # linter certifying a repo that holds unfindable ledgers is the F1 defect again.
  while IFS= read -r f; do files+=("$f"); done < <(
    { find "$REPO_ROOT/.xreview" "$REPO_ROOT/docs/xreview" -maxdepth 1 -name '*.md' 2>/dev/null || true; } | sort)
fi
[[ ${#files[@]} -eq 0 ]] && { echo "✓ verify-ledger: no ledger to check."; exit 0; }

for f in "${files[@]}"; do
  rel="${f#"$REPO_ROOT"/}"

  # An explicit, reasoned exemption, same shape as verify-references.sh's `gap:`
  # marker: `<!-- ledger-exempt: CODE — why -->`. A round is never edited in place
  # (review-ledger.md), so a ledger recording a routing that no longer exists
  # cannot be brought to a rule written after it without falsifying the record.
  exempt=$(grep -oE '<!--[[:space:]]*ledger-exempt:[[:space:]]*[A-Z-]+' "$f" 2>/dev/null \
           | grep -oE '[A-Z-]+$' | sort -u | tr '\n' ' ' || true)
  is_exempt() { case " $exempt " in *" $1 "*) return 0 ;; *) return 1 ;; esac; }

  # Bolded field names look like the contract and do not match it. The gate reads
  # `^Field: token`, so `**State**: RESOLVED` is invisible to it.
  while IFS=: read -r ln _; do
    err "BOLD-FIELD" "$rel:$ln  a header field is bolded; the gate matches '^Field: token' literally"
  done < <(grep -nE '^\*\*(Round|State|OpenFindings|Convergence|Blinded|Dissenter|Class|Target|Tier|Lenses)\*\*:' "$f" || true)

  # Class: gates the rubric-lens assertion below, so an unvalidated Class: means a
  # bolded or annotated line silences the only enforcement of the pin.
  cls=$(sed -nE 's/^Class:[[:space:]]+([a-z-]+)[[:space:]]*$/\1/p' "$f" | head -1)
  if [[ -z "$cls" ]]; then
    err "NO-CLASS" "$rel  no bare 'Class: <token>' line; the rubric-lens check keys off it"
  else
    case "$cls" in doc-only|mechanical|component|cross-component|shared-stack|skill-package) ;;
      *) err "BAD-CLASS" "$rel  Class: '$cls' is not one of the six routing classes" ;;
    esac
  fi

  rounds=$(grep -cE '^Round:[[:space:]]+[0-9]+$' "$f" || true)
  [[ "$rounds" -eq 0 ]] && err "NO-ROUND" "$rel  no 'Round: <integer>' line; round selection fails closed without it"

  # Every round needs its own complete typed block.
  for field in State OpenFindings Convergence Blinded Dissenter Lenses; do
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
    if ! [[ "$v" =~ ^[0-9]+$ ]] || [[ "$v" -lt 1 ]]; then
      err "BAD-LENSES" "$rel  Lenses: '$v' — must be a positive integer"
    fi
  done < <(sed -nE 's/^Lenses:[[:space:]]+(.*)$/\1/p' "$f")

  while IFS= read -r v; do
    [[ -z "${v// }" ]] && err "EMPTY-DISSENTER" "$rel  Dissenter: is empty; unanimity without an assigned dissenter is consensus theater"
  done < <(sed -nE 's/^Dissenter:[[:space:]]+(.*)$/\1/p' "$f")

  # A passing terminal claims zero open findings. Saying both is a contradiction
  # the gate would otherwise read as a pass.
  while read -r st n; do
    case "$st" in
      RESOLVED|RESOLVED-WITH-ACCEPTED-RISK)
        if [[ "$n" != "0" ]]; then
          err "CONTRADICTION" "$rel  State: $st requires OpenFindings: 0, found '$n'"
        fi ;;
      OPEN-BLOCKED)
        if ! [[ "$n" =~ ^[0-9]+$ ]] || [[ "$n" -lt 1 ]]; then
          err "CONTRADICTION" "$rel  State: OPEN-BLOCKED requires OpenFindings >= 1, found '$n'"
        fi ;;
    esac
  done < <(paste -d' ' \
    <(sed -nE 's/^State:[[:space:]]+(.*)$/\1/p' "$f") \
    <(sed -nE 's/^OpenFindings:[[:space:]]+(.*)$/\1/p' "$f"))

  # The rubric lens has no absence check. A cited rule id is the only evidence it
  # ran, so on a skill-package ledger at least one must resolve to a rubric row.
  if [[ "$cls" == "skill-package" ]]; then
    # Scoped to the rubric lens's own row, not the whole file. A file-wide scan is
    # satisfied by the schema's mandatory `Tier: T1`/`T2` — both literal rubric rows
    # — and by any mention of S3. Scoping also makes the assertion mean what the
    # doctrine says: the rubric lens cited ids, not the document contains an id.
    lens_row=$(grep -E '^\|[[:space:]]*(the )?rubric lens[[:space:]]*\|' "$f" || true)
    if [[ -z "$lens_row" ]]; then
      is_exempt NO-LENS-ROW \
        || err "NO-LENS-ROW" "$rel  Class: skill-package but the slate table has no rubric-lens row"
    else
      resolved=0
      for id in $(printf '%s' "$lens_row" | grep -oE '\b[A-Z][0-9]+\b' | sort -u || true); do
        grep -qE "^\| $id " "$RUBRIC" && resolved=$((resolved + 1))
      done
      if [[ "$resolved" -eq 0 ]]; then
        err "NO-RULE-ID" "$rel  the rubric-lens row cites no rule id that resolves to a rubric row — a verdict citing no rule id is not a rubric review"
      fi
    fi
  fi
done

if [[ "$ERRORS" -gt 0 ]]; then
  echo "✗ verify-ledger: $ERRORS finding(s) across ${#files[@]} ledger(s)."
  exit 1
fi
echo "✓ verify-ledger: ${#files[@]} ledger(s) conform."
