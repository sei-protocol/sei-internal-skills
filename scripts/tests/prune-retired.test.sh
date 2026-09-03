#!/usr/bin/env bash
# Regression suite for scripts/prune-retired.sh — the only script here that deletes.
# Run: scripts/tests/prune-retired.test.sh  (or `make test-prune`).
#
# The invariants that matter are the ones about what it must NOT remove. A prune
# that misses a stale skill is an annoyance; a prune that eats a user's own work,
# or a core skill, is data loss.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$SCRIPT_DIR/../.." && pwd)"
PRUNE="$REPO/scripts/prune-retired.sh"

PASS=0
FAIL=0
ok() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
no() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }
silent() { "$@" >/dev/null 2>&1; }
check()      { local d="$1"; shift; if silent "$@"; then ok "$d"; else no "$d"; fi; }
check_fail() { local d="$1"; shift; if silent "$@"; then no "$d"; else ok "$d"; fi; }

scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT

# A target that mirrors a real synced environment: the core, the parked set, the
# retired leftovers a pre-slim-down sync would have left, and the user's own work.
seed_env() {
  local t="$1"
  rm -rf "$t"; mkdir -p "$t/.claude/skills" "$t/.claude/agents"
  silent "$REPO/scripts/sync-skills.sh" --target "$t" --categories all --force
  silent "$REPO/scripts/sync-agents.sh" --target "$t" --categories all --force
  silent "$REPO/scripts/sync-experimental.sh" --target "$t" --force
  local s
  for s in data-mesh prfaq tee diagram lingua; do
    mkdir -p "$t/.claude/skills/$s"; echo stale > "$t/.claude/skills/$s/SKILL.md"
  done
  for s in data-platform-architect tee-specialist diagram-architect; do
    echo stale > "$t/.claude/agents/$s.md"
  done
  for s in my-own-skill another-personal; do
    mkdir -p "$t/.claude/skills/$s"; echo MINE > "$t/.claude/skills/$s/SKILL.md"
  done
  echo MINE > "$t/.claude/agents/my-own-agent.md"
}

echo "dry run is the default and writes nothing"
t="$scratch/dry"; seed_env "$t"
before="$(find "$t" -type f | wc -l | tr -d ' ')"
check "dry run exits 0" "$PRUNE" --target "$t"
after="$(find "$t" -type f | wc -l | tr -d ' ')"
if [ "$before" = "$after" ]; then ok "file count unchanged after a dry run"; else no "dry run deleted files ($before -> $after)"; fi
check "dry run says nothing was deleted" bash -c "'$PRUNE' --target '$t' | grep -q 'nothing was deleted'"

echo "--apply removes the retired set"
t="$scratch/apply"; seed_env "$t"
check "apply exits 0" "$PRUNE" --target "$t" --apply
for s in data-mesh prfaq tee diagram lingua; do
  check_fail "retired skill removed: $s" test -d "$t/.claude/skills/$s"
done
for a in data-platform-architect tee-specialist diagram-architect; do
  check_fail "retired agent removed: $a" test -f "$t/.claude/agents/$a.md"
done

# The rename case: the replacement must outlive the thing it replaced.
check "the replacement survives: idiomatic" test -d "$t/.claude/skills/idiomatic"

echo "it NEVER removes the user's own work"
for s in my-own-skill another-personal; do
  check "unknown skill preserved: $s"  test -f "$t/.claude/skills/$s/SKILL.md"
  check "…with content intact: $s"     grep -q MINE "$t/.claude/skills/$s/SKILL.md"
done
check "unknown agent preserved" grep -q MINE "$t/.claude/agents/my-own-agent.md"

echo "it NEVER removes a core resource"
missing=0
for d in "$REPO"/.claude/skills/*/; do
  n="$(basename "$d")"
  [ -d "$t/.claude/skills/$n" ] || { echo "    core skill deleted: $n"; missing=1; }
done
for f in "$REPO"/.claude/agents/*.md; do
  [ -f "$t/.claude/agents/$(basename "$f")" ] || { echo "    core agent deleted: $(basename "$f")"; missing=1; }
done
if [ "$missing" -eq 0 ]; then ok "every core skill and agent survived"; else no "a core resource was deleted"; fi

echo "--retired-only leaves the parked set installed"
t="$scratch/retonly"; seed_env "$t"
check      "retired-only exits 0"        "$PRUNE" --target "$t" --apply --retired-only
check_fail "retired still removed"       test -d "$t/.claude/skills/data-mesh"
check      "parked skill left installed" test -d "$t/.claude/skills/coral"
check      "parked agent left installed" test -f "$t/.claude/agents/sei-interview-expert.md"

echo "parked removal is reversible"
t="$scratch/restore"; seed_env "$t"
silent "$PRUNE" --target "$t" --apply
check_fail "parked skill gone after prune" test -d "$t/.claude/skills/coral"
silent "$REPO/scripts/sync-experimental.sh" --target "$t" --force
check "sync-experimental restores it" test -d "$t/.claude/skills/coral"

echo "running twice is a no-op"
t="$scratch/twice"; seed_env "$t"
silent "$PRUNE" --target "$t" --apply
check "second run reports nothing to prune" bash -c "'$PRUNE' --target '$t' | grep -q 'Nothing to prune'"

# The guard that makes a stale RETIRED entry harmless. If a name is promoted back
# into the core but someone forgets to drop it from the list, the core copy must
# survive — the list must never outrank the live tree.
echo "a stale retired entry cannot delete a core resource"
fake="$scratch/fakerepo"
mkdir -p "$fake/scripts" "$fake/.claude/skills/lingua" "$fake/.claude/agents" \
         "$fake/experimental/skills" "$fake/experimental/agents"
cp "$PRUNE" "$fake/scripts/"
echo "promoted back" > "$fake/.claude/skills/lingua/SKILL.md"
ft="$scratch/faketarget"; mkdir -p "$ft/.claude/skills/lingua" "$ft/.claude/agents"
echo "installed" > "$ft/.claude/skills/lingua/SKILL.md"
out="$("$fake/scripts/prune-retired.sh" --target "$ft" --apply 2>&1)"
check "the core copy survives a stale list entry" test -d "$ft/.claude/skills/lingua"
if echo "$out" | grep -q "GUARDED"; then ok "and the stale entry is reported as GUARDED"; else no "no GUARDED diagnostic emitted"; fi

# Bugbot #299, low severity, confirmed. --retired-only skipped BUILDING the parked
# list, so installed experimental resources fell through to "not from this repo,
# left alone" — the same bucket as the user's own work. On the one command whose
# value is accurate classification, that mislabels 14 resources.
echo "--retired-only still RECOGNIZES the parked set"
t="$scratch/retlabel"; seed_env "$t"
out="$("$PRUNE" --target "$t" --retired-only 2>&1)"
unknown="$(echo "$out" | sed -n 's/^KEPT — not from this repo, left alone: \([0-9]*\)$/\1/p')"
if [ "$unknown" = "3" ]; then ok "only the 3 user-authored resources count as unknown"; else no "unknown bucket is $unknown, expected 3 (parked leaked into it)"; fi
check "parked reported as recognized-but-skipped" bash -c "echo '$out' | grep -q 'PARKED — recognized but skipped'"
check "a parked skill is named in that section"   bash -c "echo '$out' | grep -q 'skill/coral'"

# Bugbot #299, high severity, NOT reproducible: the ${arr+"${arr[@]}"} idiom does
# preserve quoting. Kept as a regression test anyway — this is the only script that
# deletes, and a target path with spaces is ordinary on macOS.
echo "a target path containing spaces is handled exactly"
t="$scratch/dir with spaces"; seed_env "$t"
check      "apply exits 0 under a spaced path"  "$PRUNE" --target "$t" --apply
check_fail "retired removed under a spaced path" test -d "$t/.claude/skills/tee"
check      "core survived under a spaced path"   test -d "$t/.claude/skills/idiomatic"
check      "user work survived under a spaced path" grep -q MINE "$t/.claude/skills/my-own-skill/SKILL.md"

echo "argument handling"
check      "--help exits 0"             "$PRUNE" --help
check_fail "unknown arg exits non-zero" "$PRUNE" --nonsense

echo ""
echo "prune-retired: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
