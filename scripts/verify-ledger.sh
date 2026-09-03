#!/usr/bin/env bash
# Fail if an /xreview review ledger does not satisfy the schema /xreview ships.
#
# This exists because the schema had typed fields that nothing validated, and the
# first ledger written against it violated the contract four ways: bolded field
# names, free prose after a tokens-only field, no `Round:` line, and a path neither
# consumer candidate checks. A schema whose first instance fails it stops being
# believed.
#
# It also enforces the one guarantee the rubric lens has instead of an absence
# check: on a `skill-package` round, the rubric lens must cite at least one rule id
# that resolves to a row in skill-package-rubric.md. "A verdict citing no rule id is
# not a rubric review" was stated in eleven places and gated in none.
#
# The semantic checks are PER ROUND. They were file-wide once, and round 1's cited
# ids satisfied every later round — the same vacuity as a whole-file id scan being
# satisfied by the schema's own `Tier: T2`.
#
# Usage: verify-ledger.sh [path ...]   (default: .xreview/*.md and docs/xreview/*.md)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUBRIC="$REPO_ROOT/.claude/skills/xreview/references/skill-package-rubric.md"
ERRORS=0
err() { printf '%-18s %s\n' "$1" "$2"; ERRORS=$((ERRORS + 1)); }

# Exemptions live HERE, on the verifier, never in the ledger. An in-file marker puts
# the opt-out on the writable side of the gate, so any new ledger could grant itself
# one — the property the prose asserted and nothing enforced. This is the
# block-baseline.txt shape: enumerated, and differential in both directions, so an
# entry that stops being needed fails too.
#
# docs/xreview/hardened-core.md — that review ran before the rubric lens existed.
# Its slate pinned `audit-skill` and `author-skill` as skill-stewards and records,
# correctly, that neither ran. Adding a rubric-lens row would falsify the record,
# and review-ledger.md says a round is never edited in place. The skills it names
# were cut in PR #383, so no later ledger qualifies.
EXEMPT_NO_LENS_ROW="docs/xreview/hardened-core.md"

# docs/xreview/slice-a-reference-gate.md — its rounds predate the append-only rule.
# Round 2 names a smaller slate than round 1, and the record does not say which
# lenses were deliberately not re-dispatched versus out of scope. Reconstructing
# that would be inventing it, and a round is never edited in place.
EXEMPT_LENS_DROPPED="docs/xreview/slice-a-reference-gate.md"

files=("$@")
if [[ ${#files[@]} -eq 0 ]]; then
  # Both known ledger locations. docs/xreview/ is not a location any consumer gate
  # checks, which is exactly why ledgers there must not escape the linter — a linter
  # certifying a repo that holds unfindable ledgers is the defect it was built for.
  while IFS= read -r f; do files+=("$f"); done < <(
    { find "$REPO_ROOT/.xreview" "$REPO_ROOT/docs/xreview" -maxdepth 1 -name '*.md' 2>/dev/null || true; } | sort)
fi
[[ ${#files[@]} -eq 0 ]] && { echo "✓ verify-ledger: no ledger to check."; exit 0; }

for f in "${files[@]}"; do
  rel="${f#"$REPO_ROOT"/}"

  # A bolded field looks like the contract and does not match it: the gate reads
  # `^Field: token` literally, so `**State**: RESOLVED` is invisible to it.
  while IFS=: read -r ln _; do
    err "BOLD-FIELD" "$rel:$ln  a header field is bolded; the gate matches '^Field: token' literally"
  done < <(grep -nE '^\*\*(Round|State|OpenFindings|Convergence|Blinded|Dissenter|Class|Target|Tier|Lenses)\*\*:' "$f" || true)

  rounds=$(grep -cE '^Round:[[:space:]]+[0-9]+$' "$f" || true)
  [[ "$rounds" -eq 0 ]] && err "NO-ROUND" "$rel  no 'Round: <integer>' line; round selection fails closed without it"

  headings=$(grep -cE '^## Round[[:space:]]+[0-9]+' "$f" || true)
  if [[ "$headings" -ne "$rounds" ]]; then
    err "NO-ROUND-HEADING" "$rel  $rounds 'Round:' line(s) but $headings '## Round N' heading(s); the per-round checks key off the heading and would evaluate nothing"
  fi

  for field in State OpenFindings Convergence Blinded Dissenter Lenses; do
    n=$(grep -cE "^${field}:[[:space:]]+" "$f" || true)
    [[ "$n" -lt "$rounds" ]] && err "MISSING-FIELD" "$rel  $rounds round(s) but $n '${field}:' line(s)"
  done

  # Class: gates the rubric-lens assertion, so an unvalidated one means a bolded or
  # annotated line silences the only enforcement of the pin.
  if ! grep -qE '^Class:[[:space:]]+[a-z-]+[[:space:]]*$' "$f"; then
    err "NO-CLASS" "$rel  no bare 'Class: <token>' line; the rubric-lens check keys off it"
  fi
  while IFS= read -r v; do
    case "$v" in doc-only|mechanical|component|cross-component|shared-stack|skill-package) ;;
      *) err "BAD-CLASS" "$rel  Class: '$v' is not one of the six routing classes" ;;
    esac
  done < <(sed -nE 's/^Class:[[:space:]]+([a-z-]+)[[:space:]]*$/\1/p' "$f")

  while IFS= read -r v; do
    case "$v" in OPEN|RESOLVED|RESOLVED-WITH-ACCEPTED-RISK|OPEN-BLOCKED) ;;
      *) err "BAD-STATE" "$rel  State: '$v' is not one of OPEN|RESOLVED|RESOLVED-WITH-ACCEPTED-RISK|OPEN-BLOCKED" ;;
    esac
  done < <(sed -nE 's/^State:[[:space:]]+(.*)$/\1/p' "$f")

  # Tokens only. `degenerate` is the honest token for a one-lens round: unanimity
  # across a single reviewer is the consensus theater the dissent rule exists to
  # catch, with the field filled in.
  while IFS= read -r v; do
    case "$v" in unanimous|split|degenerate) ;;
      *) err "BAD-CONVERGENCE" "$rel  Convergence: '$v' — tokens only (unanimous|split|degenerate), never free prose" ;;
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

  # --- Per-round semantic checks -------------------------------------------------
  # One awk pass emits a record per round; bash judges each. Everything below needs
  # round boundaries: file-wide, round 1's citation satisfied round 4, and a
  # positional State/OpenFindings pairing named the wrong round's numbers.
  # The `|| true` must guard the WHOLE pipeline, not the first stage: under
  # pipefail the second grep also exits 1 on empty input, which killed the sweep
  # after NO-ROUND printed and left every later ledger unchecked.
  last_round=$( { grep -oE '^Round:[[:space:]]+[0-9]+' "$f" \
                  | grep -oE '[0-9]+' | sort -n | tail -1; } || true )
  last_round="${last_round:-0}"
  prev_lenses=""; prev_round=""
  while IFS='|' read -r rnd cls lenses slate has_row ids st n cnv dissents accepted lenslist; do
    [[ -z "$rnd" ]] && continue

    if [[ "$slate" == "none" ]]; then
      [[ "$lenses" == "1" ]] || err "LENSES-UNLISTED" \
        "$rel  round $rnd declares Lenses: $lenses but has no slate table listing them"
    elif [[ "$lenses" != "$slate" ]]; then
      err "LENSES-MISMATCH" "$rel  round $rnd declares Lenses: $lenses, slate table lists $slate"
    fi

    case "$st" in
      RESOLVED|RESOLVED-WITH-ACCEPTED-RISK)
        [[ "$n" == "0" ]] || err "CONTRADICTION" "$rel  round $rnd: State: $st requires OpenFindings: 0, found '$n'" ;;
      OPEN-BLOCKED)
        { [[ "$n" =~ ^[0-9]+$ ]] && [[ "$n" -ge 1 ]]; } \
          || err "CONTRADICTION" "$rel  round $rnd: State: OPEN-BLOCKED requires OpenFindings >= 1, found '$n'" ;;
    esac
    # A verdict is the lens's to give. An orchestrator that fixes what a lens
    # objected to closes the finding, not the verdict — so a round still holding a
    # DISSENT row cannot read `unanimous`. Without this the rule is prose, and the
    # substitution is tempting precisely because the fix is real by then.
    if [[ "${dissents:-0}" -gt 0 && "$cnv" == "unanimous" ]]; then
      err "REVISED-VERDICT" "$rel  round $rnd declares Convergence: unanimous with $dissents DISSENT row(s) — a verdict changes only when its lens is re-dispatched and says so"
    fi
    # A standing DISSENT and `State: RESOLVED` is only honest when every finding
    # that DISSENT raised is closed. If one was closed by an operator accepting a
    # named risk instead, the state says so.
    if [[ "${dissents:-0}" -gt 0 && "$st" == "RESOLVED" && "$rnd" == "$last_round" && "$accepted" == "yes" ]]; then
      err "UNSTATED-ACCEPTED-RISK" "$rel  round $rnd stamps State: RESOLVED over a standing DISSENT while recording an accepted risk — that is RESOLVED-WITH-ACCEPTED-RISK"
    fi
    if [[ "$lenses" == "1" && "$cnv" != "degenerate" ]]; then
      err "UNDECLARED-DEGENERATE" "$rel  round $rnd has Lenses: 1 but Convergence: $cnv — a single-reviewer pass is 'degenerate', not unanimity"
    fi

    # The pin is asserted on the LATEST round, matching review-ledger.md's gate-read
    # contract ("the gate reads the latest round"). An earlier round is history and
    # is never edited in place; what a consumer acts on is the newest state.
    # Append-only across rounds. Deleting a dissenting lens's row and decrementing
    # Lenses: defeats every row-text check more cheaply than any rewording — the
    # same orchestrator-over-lens substitution, done by omission. A lens leaves only
    # by being recorded as not re-dispatched.
    if [[ -n "$prev_lenses" && -n "$lenslist" ]]; then
      for pl in ${prev_lenses//,/ }; do
        [[ -z "$pl" ]] && continue
        case ",$lenslist," in
          *",$pl,"*) ;;
          *) [[ "$rel" == "$EXEMPT_LENS_DROPPED" ]] && continue
             err "LENS-DROPPED" "$rel  round $rnd drops '$pl', named in round $prev_round — record it as not re-dispatched rather than removing the row" ;;
        esac
      done
    fi
    [[ -n "$lenslist" ]] && { prev_lenses="$lenslist"; prev_round="$rnd"; }

    if [[ "$cls" == "skill-package" && "$rnd" == "$last_round" ]]; then
      if [[ "$has_row" == "no" ]]; then
        [[ "$rel" == "$EXEMPT_NO_LENS_ROW" ]] \
          || err "NO-LENS-ROW" "$rel  round $rnd is Class: skill-package but its slate has no rubric-lens row"
      else
        resolved=0
        for id in $ids; do grep -qE "^\| $id " "$RUBRIC" && resolved=$((resolved + 1)); done
        [[ "$resolved" -eq 0 ]] && err "NO-RULE-ID" \
          "$rel  round $rnd: the rubric-lens row cites no rule id that resolves to a rubric row"
      fi
    fi
  done < <(awk '
    function flush() {
      if (rnd != "")
        printf "%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s\n", rnd, (rcls != "" ? rcls : fcls), lenses,
               (sawtable ? nrows : "none"), (haslens ? "yes" : "no"), ids, state, openf, conv, ndis,
               (nacc ? "yes" : "no"), lenslist
      rcls=""; lenses=""; nrows=0; sawtable=0; intable=0; haslens=0; ids=""; state=""; openf=""; conv=""; ndis=0; nacc=0; lenslist=""
    }
    /^Class:[ \t]+/          { if (rnd == "") fcls=$2; else rcls=$2; next }
    /^## Round[ \t]+[0-9]+/  { flush(); rnd=$3; next }
    /^Lenses:[ \t]+/         { lenses=$2; next }
    /^State:[ \t]+/          { state=$2; next }
    /^OpenFindings:[ \t]+/   { openf=$2; next }
    /^Convergence:[ \t]+/    { conv=$2; next }
    /accepted-with-risk|accepted risk|accept-with-risk/ { if (rnd != "") nacc=1 }
    /^\|[ \t]*Lens[ \t]*\|/  {
      intable=1; sawtable=1; nrows=0; vcol=0
      nf = split($0, hdr, "|")
      for (c = 2; c <= nf; c++) { h = hdr[c]; gsub(/[ \t*`]/, "", h); if (h == "Verdict") vcol = c - 1 }
      next
    }
    intable && /^\|[ \t]*-+/ { next }
    intable && /^\|/ {
      nrows++
      # The verdict cell, located by the header rather than a fixed index: the
      # schema and the fixtures disagree on column order, and a substring scan is
      # defeated by "DISSENT, later RATIFY" in a Resolution cell.
      if (vcol > 0) {
        nf = split($0, cell, "|")
        v = (vcol + 1 <= nf) ? cell[vcol + 1] : ""
        gsub(/[ \t*`]/, "", v)
        if (v ~ /^DISSENT/) ndis++
      } else if ($0 ~ /DISSENT/ && $0 !~ /RATIFY/) ndis++
      lensname = cell[2]; gsub(/[ \t*`]/, "", lensname)
      if (lensname != "") lenslist = lenslist "," lensname
      if ($0 ~ /^\|[ \t]*`?(the )?rubric lens`?[ \t]*\|/) {
        haslens=1; line=$0
        while (match(line, /[A-Z][0-9]+/)) {
          ids = ids " " substr(line, RSTART, RLENGTH); line = substr(line, RSTART+RLENGTH)
        }
      }
      next
    }
    intable { intable=0 }
    END { flush() }
  ' "$f")
done

if [[ "$ERRORS" -gt 0 ]]; then
  echo "✗ verify-ledger: $ERRORS finding(s) across ${#files[@]} ledger(s)."
  exit 1
fi
echo "✓ verify-ledger: ${#files[@]} ledger(s) conform."
