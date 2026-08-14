#!/usr/bin/env bash
# Regression suite for scripts/get.sh — the targeted fetcher.
# Run: scripts/tests/get.test.sh  (or `make test-get`).
#
# Runs entirely offline via SEI_SKILLS_LOCAL, pointed at this checkout. The
# network path shares every code path below it, so what is left untested here is
# the tarball download itself.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$SCRIPT_DIR/../.." && pwd)"
GET="$REPO/scripts/get.sh"

PASS=0
FAIL=0
ok() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
no() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }
silent() { "$@" >/dev/null 2>&1; }
check()      { local d="$1"; shift; if silent "$@"; then ok "$d"; else no "$d"; fi; }
check_fail() { local d="$1"; shift; if silent "$@"; then no "$d"; else ok "$d"; fi; }

scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT

run() {  # run <target-root> <args...>
  local t="$1"; shift
  SEI_SKILLS_LOCAL="$REPO" SEI_SKILLS_TARGET="$t" bash "$GET" "$@"
}

# `run` is a shell function, so it cannot be reached from inside `bash -c`.
# These assert on captured output in-process.
# Matching is done on a captured string, NOT by piping into `grep -q`. Under
# `set -o pipefail`, grep -q exits on its first match and closes the pipe, the
# producer takes SIGPIPE (141), and pipefail reports that as the pipeline's
# status — so a matching pattern reads as a failure.
grep_out()      { local d="$1" pat="$2"; shift 2; local o; o="$(run "$@" 2>/dev/null)"
                  if [[ "$o" == *"$pat"* ]]; then ok "$d"; else no "$d"; fi; }
grep_out_fail() { local d="$1" pat="$2"; shift 2; local o; o="$(run "$@" 2>/dev/null)"
                  if [[ "$o" == *"$pat"* ]]; then no "$d"; else ok "$d"; fi; }

echo "list enumerates every kind"
out="$(run "$scratch/l" list 2>/dev/null)"
for section in "Output styles" "Skills — core" "Skills — experimental" "Agents — core"; do
  if [[ "$out" == *"$section"* ]]; then ok "lists: $section"; else no "missing section: $section"; fi
done
if [[ "$out" == *$'\n  xreview'* ]];       then ok "list names a known core skill"; else no "list names a known core skill"; fi
if [[ "$out" == *$'\n  project-brief'* ]]; then ok "list names a known experimental skill"; else no "list names a known experimental skill"; fi

echo "output-style defaults to asd-ste100 and lands as a file"
t="$scratch/os"
check "exits 0"        run "$t" output-style
check "file landed"    test -f "$t/.claude/output-styles/asd-ste100.md"
check "content is the style, not a stub" grep -q "Simplified Technical English" "$t/.claude/output-styles/asd-ste100.md"

# The whole contract of the output style: shipped, never switched on.
echo "output-style NEVER activates itself"
check_fail "no settings.json created" test -f "$t/.claude/settings.json"
grep_out   "prints the opt-in" "NOT active" "$t" output-style

echo "…and does not nag once a style is already chosen"
t="$scratch/os2"; mkdir -p "$t/.claude"
printf '{\"outputStyle\":\"Explanatory\"}\n' > "$t/.claude/settings.json"
grep_out      "reports installed-but-not-active" "already have an outputStyle" "$t" output-style
grep_out_fail "does not print the turn-it-on pitch" "Turn it on" "$t" output-style
check      "settings.json untouched"  grep -qF '"outputStyle":"Explanatory"' "$t/.claude/settings.json"

echo "skill fetches the whole directory, from either tier"
t="$scratch/sk"
check "core skill exits 0"          run "$t" skill xreview
check "SKILL.md landed"             test -f "$t/.claude/skills/xreview/SKILL.md"
check "references/ came too"        bash -c "ls '$t/.claude/skills/xreview/references/'*.md >/dev/null 2>&1"
check "evals came too"              test -f "$t/.claude/skills/xreview/evals/evals.json"
check "experimental skill exits 0"  run "$t" skill project-brief
grep_out "and is labelled experimental" "experimental" "$t" skill project-brief

echo "agent fetches a single file, from either tier"
t="$scratch/ag"
check "core agent exits 0"    run "$t" agent idiomatic-reviewer
check "agent file landed"     test -f "$t/.claude/agents/idiomatic-reviewer.md"
check "experimental agent"    run "$t" agent sei-interview-expert

# It installs what you named and nothing else. A fetcher that quietly pulled a
# dependency would defeat the point of a targeted door.
echo "it installs ONLY what was named"
t="$scratch/only"
silent run "$t" skill xreview
n_sk="$(find "$t/.claude/skills" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l | tr -d ' ')"
if [ "$n_sk" = "1" ]; then ok "one skill requested, one skill installed"; else no "expected 1 skill, found $n_sk"; fi
check_fail "no agents dragged in"        test -d "$t/.claude/agents"
check_fail "no output styles dragged in" test -d "$t/.claude/output-styles"

echo "unknown names fail loudly rather than installing nothing quietly"
t="$scratch/err"
check_fail "unknown skill"        run "$t" skill definitely-not-a-skill
check_fail "unknown agent"        run "$t" agent definitely-not-an-agent
check_fail "unknown output style" run "$t" output-style definitely-not-a-style
check_fail "unknown target"       run "$t" bogus
check_fail "skill with no name"   run "$t" skill
check_fail "agent with no name"   run "$t" agent
check_fail "nothing was created on any failure" test -d "$t/.claude"

echo "no args prints usage and exits 0"
check "usage exits 0"     run "$scratch/u"
grep_out "usage names the targets" "output-style" "$scratch/u"

# The documented invocation pipes this script into bash, where $0 is "bash".
# The sibling scripts print usage by grepping $0; that idiom silently reads the
# shell binary here, which is how it broke the first time.
echo "the PIPED invocation works — that is the documented one"
t="$scratch/pipe"
piped() { cat "$GET" | SEI_SKILLS_LOCAL="$REPO" SEI_SKILLS_TARGET="$t" bash -s -- "$@"; }
check "piped fetch exits 0"       piped skill brevity
check "piped fetch landed"        test -f "$t/.claude/skills/brevity/SKILL.md"
check "piped usage exits 0"       piped
u="$(piped 2>&1)"
if [[ "$u" == *"No such file or directory"* ]]; then no "piped usage does not grep \$0"; else ok "piped usage does not grep \$0"; fi
if [[ "$u" == *"output-style"* ]];             then ok "piped usage names the targets"; else no "piped usage names the targets"; fi
check_fail "piped unknown target fails"  piped bogus

echo "a bad SEI_SKILLS_LOCAL is refused, not silently ignored"
if SEI_SKILLS_LOCAL="$scratch" SEI_SKILLS_TARGET="$scratch/bad" bash "$GET" list >/dev/null 2>&1
then no "refuses a non-checkout path"; else ok "refuses a non-checkout path"; fi

echo ""
echo "get: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
