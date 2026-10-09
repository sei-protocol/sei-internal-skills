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

# The names the experimental/ removal retired: 12 skills and 1 agent. Stated
# here, not read from the script, so an entry dropped from the script's list
# fails a case.
CUT_SKILLS="bugbash code-structure coral council design ebpf interview issue
  linear-ticket project-brief research workstream"
CUT_AGENTS="sei-interview-expert"
OLD_SKILLS="data-mesh prfaq tee diagram lingua audit-skill author-skill"
OLD_AGENTS="data-platform-architect tee-specialist diagram-architect"

# A target that mirrors a real synced environment: the core, the retired
# leftovers an older sync would have left, and the user's own work.
seed_env() {
  local t="$1"
  rm -rf "$t"; mkdir -p "$t/.claude/skills" "$t/.claude/agents"
  silent "$REPO/scripts/sync-skills.sh" --target "$t" --categories all --force
  silent "$REPO/scripts/sync-agents.sh" --target "$t" --categories all --force
  local s
  for s in $OLD_SKILLS $CUT_SKILLS; do
    mkdir -p "$t/.claude/skills/$s"; echo stale > "$t/.claude/skills/$s/SKILL.md"
  done
  for s in $OLD_AGENTS $CUT_AGENTS; do
    echo stale > "$t/.claude/agents/$s.md"
  done
  # A retired skill with the user's own run output in state/.
  mkdir -p "$t/.claude/skills/research/state/run-1"
  echo "notes" > "$t/.claude/skills/research/state/run-1/notes.md"
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
out="$("$PRUNE" --target "$t" 2>&1)"
unknown="$(printf '%s\n' "$out" | sed -n 's/^KEPT — not from this repo, left alone: \([0-9]*\)$/\1/p')"
if [ "$unknown" = "3" ]; then ok "only the 3 user-authored resources count as unknown"; else no "unknown bucket is '$unknown', expected 3"; fi
if printf '%s\n' "$out" | grep -q 'skill/research.*has state/ files'; then ok "names skill/research with has state/ files"; else no "research state/ files not flagged"; fi
if printf '%s\n' "$out" | grep -q 'skill/coral.*has state/ files'; then no "coral flagged with no state/ files"; else ok "a skill without state/ files is not flagged"; fi

echo "--check prints a hint only when something is prunable"
hint="$("$PRUNE" --target "$t" --check 2>&1)"
if [[ "$hint" == *"make -C"* && "$hint" == *"prune-retired-apply"* ]]; then ok "--check hint names make -C and prune-retired-apply"; else no "--check hint: '$hint'"; fi
clean="$scratch/clean"; rm -rf "$clean"; mkdir -p "$clean"
silent "$REPO/scripts/sync-skills.sh" --target "$clean" --categories all --force
silent "$REPO/scripts/sync-agents.sh" --target "$clean" --categories all --force
hint="$("$PRUNE" --target "$clean" --check 2>&1)"
if [ -z "$hint" ]; then ok "--check on a clean target prints nothing"; else no "--check on a clean target printed: '$hint'"; fi

echo "--apply removes the retired set"
t="$scratch/apply"; seed_env "$t"
check "apply exits 0" "$PRUNE" --target "$t" --apply
for s in $OLD_SKILLS $CUT_SKILLS; do
  check_fail "retired skill removed: $s" test -d "$t/.claude/skills/$s"
done
for a in $OLD_AGENTS $CUT_AGENTS; do
  check_fail "retired agent removed: $a" test -f "$t/.claude/agents/$a.md"
done

echo "it NEVER removes the user's own work"
for s in my-own-skill another-personal; do
  check "unknown skill preserved: $s"  test -f "$t/.claude/skills/$s/SKILL.md"
  check "…with content intact: $s"     grep -q MINE "$t/.claude/skills/$s/SKILL.md"
done
check "unknown agent preserved" grep -q MINE "$t/.claude/agents/my-own-agent.md"

echo "it NEVER removes a core resource"
missing=0
for d in "$REPO"/.claude/skills/*/; do
  [ -f "${d}SKILL.md" ] || continue
  n="$(basename "$d")"
  [ -d "$t/.claude/skills/$n" ] || { echo "    core skill deleted: $n"; missing=1; }
done
for f in "$REPO"/.claude/agents/*.md; do
  [ -f "$t/.claude/agents/$(basename "$f")" ] || { echo "    core agent deleted: $(basename "$f")"; missing=1; }
done
if [ "$missing" -eq 0 ]; then ok "every core skill and agent survived"; else no "a core resource was deleted"; fi

echo "running twice is a no-op"
t="$scratch/twice"; seed_env "$t"
silent "$PRUNE" --target "$t" --apply
check "second run reports nothing to prune" bash -c "'$PRUNE' --target '$t' | grep -q 'Nothing to prune'"

# The guard that makes a stale RETIRED entry harmless. If a name is promoted back
# into the core but someone forgets to drop it from the list, the core copy must
# survive — the list must never outrank the live tree.
echo "a stale retired entry cannot delete a core resource"
fake="$scratch/fakerepo"
mkdir -p "$fake/scripts" "$fake/.claude/skills/lingua" "$fake/.claude/agents"
cp "$PRUNE" "$fake/scripts/"
echo "promoted back" > "$fake/.claude/skills/lingua/SKILL.md"
ft="$scratch/faketarget"; mkdir -p "$ft/.claude/skills/lingua" "$ft/.claude/agents"
echo "installed" > "$ft/.claude/skills/lingua/SKILL.md"
out="$("$fake/scripts/prune-retired.sh" --target "$ft" --apply 2>&1)"
check "the core copy survives a stale list entry" test -d "$ft/.claude/skills/lingua"
if echo "$out" | grep -q "GUARDED"; then ok "and the stale entry is reported as GUARDED"; else no "no GUARDED diagnostic emitted"; fi

# After a pull, git keeps a removed skill's directory when it still holds
# ignored files. That directory has no SKILL.md, so it is not core, and the
# guard must not shield the installed copy of a retired skill.
echo "a leftover directory in the repo does not guard a retired skill"
fake2="$scratch/fakerepo2"
mkdir -p "$fake2/scripts" "$fake2/.claude/skills/research/state" "$fake2/.claude/agents"
cp "$PRUNE" "$fake2/scripts/"
echo "run output" > "$fake2/.claude/skills/research/state/run-1.md"
ft2="$scratch/faketarget2"; mkdir -p "$ft2/.claude/skills/research" "$ft2/.claude/agents"
echo "installed" > "$ft2/.claude/skills/research/SKILL.md"
out="$("$fake2/scripts/prune-retired.sh" --target "$ft2" --apply 2>&1)"
check_fail "the installed retired copy is removed" test -d "$ft2/.claude/skills/research"
if printf '%s\n' "$out" | grep -q "GUARDED"; then no "a leftover directory was treated as core (GUARDED)"; else ok "and nothing is reported as GUARDED"; fi

# Bugbot #299, high severity, NOT reproducible: the ${arr+"${arr[@]}"} idiom does
# preserve quoting. Kept as a regression test anyway — this is the only script that
# deletes, and a target path with spaces is ordinary on macOS.
echo "a target path containing spaces is handled exactly"
t="$scratch/dir with spaces"; seed_env "$t"
check      "apply exits 0 under a spaced path"  "$PRUNE" --target "$t" --apply
check_fail "retired removed under a spaced path" test -d "$t/.claude/skills/tee"
check      "core survived under a spaced path"   test -d "$t/.claude/skills/kubernetes"
check      "user work survived under a spaced path" grep -q MINE "$t/.claude/skills/my-own-skill/SKILL.md"

echo "argument handling"
check      "--help exits 0"             "$PRUNE" --help
check_fail "unknown arg exits non-zero" "$PRUNE" --nonsense
check_fail "--retired-only is gone"     "$PRUNE" --retired-only

echo ""
echo "prune-retired: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
