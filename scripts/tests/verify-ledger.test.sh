#!/usr/bin/env bash
# Regression suite for scripts/verify-ledger.sh.
#
# This exists because the linter was shipped without one, and the review that
# caught that also caught two ways it went vacuous — a `Tier: T2` line satisfying
# the rule-id assertion, and a bolded `Class:` silencing it entirely. Both are
# regex edits away from returning. The whole argument of this branch is that a
# checker with no gate behind it is where defects reach review.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
LINT="$SCRIPT_DIR/../verify-ledger.sh"

PASS=0; FAIL=0
ok() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
no() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT

# A conforming skill-package ledger. Cases mutate one thing from this.
good() {
  cat <<'EOF'
# Review ledger — probe
Target:       a branch
Class:        skill-package
Tier:         T3

## Round 1

Round:        1
State:        RESOLVED
OpenFindings: 0
Convergence:  unanimous
Blinded:      yes
Dissenter:    security-specialist
Lenses:       4

| Lens | Role | Verdict |
|---|---|---|
| rubric lens | pinned | RATIFY — cited `T1`, `S2` |
| prose-steward | pinned | RATIFY |
| systems-engineer | §4a | RATIFY |
| security-specialist | dissenter | RATIFY |
EOF
}

# check <desc> <expect-code|CLEAN> — writes $TMP/l.md from stdin first.
check() {
  local desc="$1" want="$2" out rc
  out="$("$LINT" "$TMP/l.md" 2>&1)"; rc=$?
  if [ "$want" = "CLEAN" ]; then
    [ "$rc" -eq 0 ] && ok "$desc" || no "$desc (rc=$rc: $out)"
  else
    if [ "$rc" -ne 0 ] && printf '%s' "$out" | grep -q "$want"; then ok "$desc"
    else no "$desc (rc=$rc, wanted $want, got: $out)"; fi
  fi
}

echo "a conforming ledger passes"
good > "$TMP/l.md"; check "conforming skill-package ledger" CLEAN

echo "the four defects that motivated the linter"
good | sed 's/^State:        RESOLVED/**State**: RESOLVED/' > "$TMP/l.md"
check "a bolded round field is caught" BOLD-FIELD
good | sed 's/^Convergence:  unanimous/Convergence:  unanimous — 3 RATIFY after a split/' > "$TMP/l.md"
check "free prose after a tokens-only field is caught" BAD-CONVERGENCE
good | grep -v '^Round:' > "$TMP/l.md"
check "a missing Round: line is caught" NO-ROUND
good | sed 's/^State:        RESOLVED/State:        DONE/' > "$TMP/l.md"
check "a State: outside the enum is caught" BAD-STATE

echo "the two ways the rule-id assertion went vacuous"
# Tier: takes T1|T2|T3, and T1/T2 are literal rubric rows. A file-wide id scan
# was satisfied by the schema's own mandatory field — vacuous exactly when
# someone had just trimmed review depth to the T2 floor.
good | sed 's/^Tier:         T3/Tier:         T2/' \
     | sed 's/| rubric lens | pinned | RATIFY — cited `T1`, `S2` |/| rubric lens | pinned | RATIFY, looks fine |/' > "$TMP/l.md"
check "Tier: T2 does not satisfy the rule-id assertion" NO-RULE-ID
# Class: gates the assertion, so bolding it silenced the only enforcement of the pin.
good | sed 's/^Class:        skill-package/**Class**: skill-package/' > "$TMP/l.md"
check "a bolded Class: is caught, not silently skipped" BOLD-FIELD
good | sed 's/^Class:        skill-package/Class:        skill-packages/' > "$TMP/l.md"
check "a Class: outside the six routing classes is caught" BAD-CLASS
good | grep -v '^Class:' > "$TMP/l.md"
check "a missing Class: is caught" NO-CLASS

echo "the rubric-lens pin"
good | sed 's/| rubric lens | pinned | RATIFY — cited `T1`, `S2` |/| rubric lens | pinned | RATIFY, read it and it is fine |/' > "$TMP/l.md"
check "a rubric-lens row citing no rule id is caught" NO-RULE-ID
good | grep -v 'rubric lens' | sed 's/^Lenses:       4/Lenses:       3/' > "$TMP/l.md"
check "a skill-package ledger with no rubric-lens row is caught" NO-LENS-ROW
# An id cited outside the lens row must not satisfy it: the assertion is about
# what the lens said, not what the document contains.
good | sed 's/| rubric lens | pinned | RATIFY — cited `T1`, `S2` |/| rubric lens | pinned | RATIFY |\n\nElsewhere the ledger mentions T1 and S2./' > "$TMP/l.md"
check "an id outside the lens row does not satisfy the assertion" NO-RULE-ID

echo "state and arity"
good | sed 's/^OpenFindings: 0/OpenFindings: 3/' > "$TMP/l.md"
check "RESOLVED with open findings is a contradiction" CONTRADICTION
good | sed 's/^State:        RESOLVED/State:        OPEN-BLOCKED/' > "$TMP/l.md"
check "OPEN-BLOCKED with zero open findings is a contradiction" CONTRADICTION
good | grep -v '^Lenses:' > "$TMP/l.md"
check "a missing Lenses: line is caught" MISSING-FIELD
good | sed 's/^Lenses:       4/Lenses:       many/' > "$TMP/l.md"
check "a non-integer Lenses: is caught" BAD-LENSES
good | sed 's/^Dissenter:    security-specialist/Dissenter:    /' > "$TMP/l.md"
check "an empty Dissenter: is caught" EMPTY-DISSENTER
good | sed 's/^Blinded:      yes/Blinded:      partially/' > "$TMP/l.md"
check "a Blinded: outside yes|no is caught" BAD-BLINDED

echo "Lenses: is measured against the slate, not self-reported"
good | sed 's/^Lenses:       4/Lenses:       6/' > "$TMP/l.md"
check "a Lenses: count above the slate rows is caught" LENSES-MISMATCH
good | sed 's/^Lenses:       4/Lenses:       2/' > "$TMP/l.md"
check "a Lenses: count below the slate rows is caught" LENSES-MISMATCH
# Claiming lenses without listing them is the transcription failure this repo has
# hit three times; a round with no slate table can only honestly be one lens.
good | sed '/^| Lens | Role | Verdict |/,$d' > "$TMP/l.md"
printf '<!-- ledger-exempt: NO-LENS-ROW — no slate table in this fixture at all -->\n' >> "$TMP/l.md"
check "declaring 4 lenses with no slate table is caught" LENSES-UNLISTED

echo "exemptions live on the verifier, not in the ledger"
# The old design read a `<!-- ledger-exempt: -->` marker out of the file, which put
# the opt-out on the writable side of the gate: any ledger could grant itself one.
good | grep -v 'rubric lens' | sed 's/^Lenses:       4/Lenses:       3/' > "$TMP/l.md"
printf '<!-- ledger-exempt: NO-LENS-ROW — I would like this to pass -->\n' >> "$TMP/l.md"
check "a ledger cannot exempt itself with an in-file marker" NO-LENS-ROW
good | sed 's/^State:        RESOLVED/State:        DONE/' > "$TMP/l.md"
printf '<!-- ledger-exempt: BAD-STATE — please -->\n' >> "$TMP/l.md"
check "an in-file marker cannot silence an unrelated rule either" BAD-STATE
# The one allowlisted path is exempt, and only for the one rule.
if "$LINT" "$REPO_ROOT/docs/xreview/hardened-core.md" >/dev/null 2>&1; then
  ok "the allowlisted historical ledger is exempt"
else
  no "the allowlisted ledger fails: $("$LINT" "$REPO_ROOT/docs/xreview/hardened-core.md" 2>&1 | head -2)"
fi
# Differential: the allowlist must name a path that exists and still needs it.
grep -q 'EXEMPT_NO_LENS_ROW="docs/xreview/hardened-core.md"' "$LINT" \
  && [ -f "$REPO_ROOT/docs/xreview/hardened-core.md" ] \
  && ok "the allowlist names a path that exists" \
  || no "the allowlist names a path that is not in the tree"

echo "semantic checks are per round, not file-wide"
# A file-wide scan let round 1's citation satisfy every later round. Two rounds,
# the second with a lens row citing nothing.
{ good; cat <<'EOF'

## Round 2

Round:        2
State:        RESOLVED
OpenFindings: 0
Convergence:  unanimous
Blinded:      yes
Dissenter:    security-specialist
Lenses:       4

| Lens | Role | Verdict |
|---|---|---|
| rubric lens | pinned | RATIFY, read it and it is fine |
| prose-steward | pinned | RATIFY |
| systems-engineer | §4a | RATIFY |
| security-specialist | dissenter | RATIFY |
EOF
} > "$TMP/l.md"
check "round 1's cited ids do not satisfy round 2" NO-RULE-ID
# A per-round Class: overrides the file default; a round that changed class mid
# review must still be checked.
{ printf 'Target:       t\nClass:        shared-stack\nTier:         T3\n\n## Round 1\n\n'
  printf 'Round:        1\nState:        RESOLVED\nOpenFindings: 0\nConvergence:  unanimous\n'
  printf 'Blinded:      yes\nDissenter:    x\nLenses:       2\nClass:        skill-package\n\n'
  printf '| Lens | Role | Verdict |\n|---|---|---|\n| prose-steward | pinned | RATIFY |\n| systems-engineer | lens | RATIFY |\n'
} > "$TMP/l.md"
check "a round that declares its own Class: is checked on that class" NO-LENS-ROW

echo "a one-lens round is degenerate, not unanimous"
good | sed 's/^Lenses:       4/Lenses:       1/' \
     | sed '/^| prose-steward/d;/^| systems-engineer/d;/^| security-specialist/d' > "$TMP/l.md"
check "Lenses: 1 with Convergence: unanimous is caught" UNDECLARED-DEGENERATE
good | sed 's/^Lenses:       4/Lenses:       1/' | sed 's/^Convergence:  unanimous/Convergence:  degenerate/' \
     | sed '/^| prose-steward/d;/^| systems-engineer/d;/^| security-specialist/d' > "$TMP/l.md"
check "Lenses: 1 with Convergence: degenerate passes" CLEAN

echo "the live ledgers conform"
if "$LINT" >/dev/null 2>&1; then ok "every ledger in the tree conforms"
else no "the tree's own ledgers do not conform: $("$LINT" 2>&1 | tail -3)"; fi

echo "both ledger locations are discovered"
n="$("$LINT" 2>&1 | grep -oE '[0-9]+ ledger' | grep -oE '^[0-9]+')"
[ "${n:-0}" -ge 3 ] && ok "discovers .xreview/ and docs/xreview/ ($n ledgers)" \
  || no "discovered only ${n:-0} ledger(s); docs/xreview/ may be invisible again"

echo ""
echo "verify-ledger: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
