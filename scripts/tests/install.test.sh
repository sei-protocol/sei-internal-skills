#!/usr/bin/env bash
# Regression suite for scripts/install.sh, both modes. It never reaches the network.
# Run: scripts/tests/install.test.sh  (or `make test-install`).
#
# The targeted mode reads this checkout through SEI_INTERNAL_SKILLS_HOME, which is
# the real short-circuit: the script reads an existing checkout and downloads nothing.
# The download path runs against curl and gh shims placed first on PATH. One set
# fails, and one set serves a tarball built from this checkout.
#
# The no-argument mode clones into a home directory and runs make, so the suite
# asserts its structure and does not run it.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$SCRIPT_DIR/../.." && pwd)"
GET="$REPO/scripts/install.sh"

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
  SEI_INTERNAL_SKILLS_HOME="$REPO" SEI_SKILLS_TARGET="$t" bash "$GET" "$@"
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
for section in "Output styles" "Skills" "Agents"; do
  if [[ "$out" == *"$section"* ]]; then ok "lists: $section"; else no "missing section: $section"; fi
done
if [[ "$out" != *experimental* ]]; then ok "list names no experimental tier"; else no "list still names an experimental tier"; fi
# The trailing newline keeps `kubernetes` from matching `kubernetes-specialist`.
if [[ "$out" == *$'\n  harbor-dev\n'* && "$out" == *$'\n  kubernetes\n'* ]]; then ok "list names a known skill"; else no "list names a known skill"; fi

echo "output-style defaults to asd-ste100 and lands as a file"
t="$scratch/os"
check "exits 0"        run "$t" output-style
check "file landed"    test -f "$t/.claude/output-styles/asd-ste100.md"
check "content is the style, not a stub" grep -q "Simplified Technical English" "$t/.claude/output-styles/asd-ste100.md"

# Asking for the style by name IS the request to use it, so a targeted install
# activates it. Everything below guards the edges of that.
echo "output-style activates itself"
check "settings.json created"     test -f "$t/.claude/settings.json"
check "outputStyle set to it"     bash -c "python3 -c \"import json;assert json.load(open('$t/.claude/settings.json'))['outputStyle']=='ASD-STE100'\""
# On a fresh target: grep_out re-invokes, and by the second call the style is
# already active, so the takes-effect line only appears on a target that has
# not been activated yet.
grep_out "says when it takes effect" "next session" "$scratch/os1b" output-style
grep_out "re-running is a no-op"     "Already your active output style" "$t" output-style

echo "…but it never overwrites a style you already chose"
t="$scratch/os2"; mkdir -p "$t/.claude"
printf '{"outputStyle":"Explanatory","model":"opus"}\n' > "$t/.claude/settings.json"
grep_out "reports the existing choice" "already have" "$t" output-style
check    "the existing style survives" bash -c "python3 -c \"import json;assert json.load(open('$t/.claude/settings.json'))['outputStyle']=='Explanatory'\""

echo "…and preserves every other key when it does write"
t="$scratch/os3"; mkdir -p "$t/.claude"
printf '{"model":"opus","permissions":{"allow":["Bash"]}}\n' > "$t/.claude/settings.json"
silent run "$t" output-style
check "other keys survive"   bash -c "python3 -c \"import json;d=json.load(open('$t/.claude/settings.json'));assert d['model']=='opus' and d['permissions']['allow']==['Bash']\""
check "style was added"      bash -c "python3 -c \"import json;assert json.load(open('$t/.claude/settings.json'))['outputStyle']=='ASD-STE100'\""
check "a backup was taken"   bash -c "ls '$t/.claude/settings.json.bak-'* >/dev/null 2>&1"

# A settings file it cannot parse is the one case where writing could destroy
# real configuration, so it refuses rather than guessing.
echo "…and refuses a settings.json it cannot parse"
t="$scratch/os4"; mkdir -p "$t/.claude"
printf '{ "outputStyle": "Explanatory",  <<< broken\n' > "$t/.claude/settings.json"
before="$(cat "$t/.claude/settings.json")"
grep_out "reports the malformed file" "does not parse" "$t" output-style
if [ "$before" = "$(cat "$t/.claude/settings.json")" ]; then ok "malformed file left byte-identical"; else no "malformed file was modified"; fi

# Bugbot #326: consuming the flag left the argument list empty, which fell
# through to the no-argument path and cloned + synced everything. Someone
# reaching for the escape hatch is asking to install LESS.
echo "--no-activate alone must not become the full install"
check_fail "bare --no-activate exits non-zero" run "$scratch/na" --no-activate
o="$(run "$scratch/na" --no-activate 2>&1 || true)"
if [[ "$o" == *"needs a target"* ]]; then ok "says what is missing"; else no "did not explain the missing target"; fi
if [[ "$o" == *"git checkout"* ]]; then no "reached the clone path"; else ok "never reached the clone path"; fi

# Bugbot #326: os.replace onto a symlink swaps the LINK for a regular file,
# detaching a dotfiles-managed settings.json and leaving the real file without
# the change. Editing through the link is what making it a link meant.
echo "a symlinked settings.json is edited through, not replaced"
t="$scratch/sym"; mkdir -p "$t/.claude" "$t/dotfiles"
printf '{"model":"opus","permissions":{"allow":["Bash"]}}\n' > "$t/dotfiles/settings.json"
ln -s "$t/dotfiles/settings.json" "$t/.claude/settings.json"
silent run "$t" output-style
check "the symlink survives"          test -L "$t/.claude/settings.json"
check "the real file got the style"   bash -c "python3 -c \"import json;assert json.load(open('$t/dotfiles/settings.json'))['outputStyle']=='ASD-STE100'\""
check "its other keys survived"       bash -c "python3 -c \"import json;d=json.load(open('$t/dotfiles/settings.json'));assert d['model']=='opus' and d['permissions']['allow']==['Bash']\""

echo "--no-activate installs without touching settings"
t="$scratch/os5"
check      "exits 0"                  run "$t" --no-activate output-style
check      "the style file landed"    test -f "$t/.claude/output-styles/asd-ste100.md"
check_fail "no settings.json written" test -f "$t/.claude/settings.json"

echo "skill fetches the whole directory"
t="$scratch/sk"
check "skill exits 0"               run "$t" skill kubernetes
check "SKILL.md landed"             test -f "$t/.claude/skills/kubernetes/SKILL.md"
check "references/ came too"        bash -c "ls '$t/.claude/skills/kubernetes/references/'*.md >/dev/null 2>&1"
check "evals came too"              test -f "$t/.claude/skills/kubernetes/evals/evals.json"

echo "agent fetches a single file"
t="$scratch/ag"
check "agent exits 0"         run "$t" agent sre-engineer
check "agent file landed"     test -f "$t/.claude/agents/sre-engineer.md"

# A retired name must fail with the lookup message, not crash some other way.
echo "a retired name fails with a clear message"
o="$(run "$scratch/ret" skill xreview 2>&1)"; rc=$?
if [ "$rc" -ne 0 ] && [[ "$o" == *"no skill named"* ]]; then ok "retired skill xreview: no skill named"; else no "retired skill xreview (rc=$rc)"; fi
o="$(run "$scratch/ret" agent prose-steward 2>&1)"; rc=$?
if [ "$rc" -ne 0 ] && [[ "$o" == *"no agent named"* ]]; then ok "retired agent prose-steward: no agent named"; else no "retired agent prose-steward (rc=$rc)"; fi

# After a pull, git keeps a removed skill's directory when it still holds
# ignored files. Such a directory has no SKILL.md, so it is not a skill.
echo "a leftover directory without a SKILL.md is not a skill"
fake="$scratch/fake"
mkdir -p "$fake/.claude/skills/ghost/state"
cp -R "$REPO/.claude/skills/kubernetes" "$fake/.claude/skills/kubernetes"
printf 'run output\n' > "$fake/.claude/skills/ghost/state/x"
o="$(SEI_INTERNAL_SKILLS_HOME="$fake" SEI_SKILLS_TARGET="$scratch/ghost" bash "$GET" skill ghost 2>&1)"; rc=$?
if [ "$rc" -ne 0 ] && [[ "$o" == *"no skill named"* ]]; then ok "skill ghost: no skill named"; else no "skill ghost (rc=$rc)"; fi
o="$(SEI_INTERNAL_SKILLS_HOME="$fake" SEI_SKILLS_TARGET="$scratch/ghost" bash "$GET" list 2>/dev/null)"
if [[ "$o" != *ghost* && "$o" == *$'\n  kubernetes\n'* ]]; then ok "list does not name ghost"; else no "list names ghost, or lost kubernetes"; fi

# It installs what you named and nothing else. A fetcher that quietly pulled a
# dependency would defeat the point of a targeted door.
echo "it installs ONLY what was named"
t="$scratch/only"
silent run "$t" skill kubernetes
n_sk="$(find "$t/.claude/skills" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l | tr -d ' ')"
if [ "$n_sk" = "1" ]; then ok "one skill requested, one skill installed"; else no "expected 1 skill, found $n_sk"; fi
check_fail "no agents dragged in"        test -d "$t/.claude/agents"
check_fail "no output styles dragged in" test -d "$t/.claude/output-styles"
# Only output-style may write settings. A skill or agent doing so would be
# changing how Claude talks on the strength of an unrelated request.
silent run "$t" agent sre-engineer
check_fail "skill and agent never write settings.json" test -f "$t/.claude/settings.json"

echo "unknown names fail loudly rather than installing nothing quietly"
t="$scratch/err"
check_fail "unknown skill"        run "$t" skill definitely-not-a-skill
check_fail "unknown agent"        run "$t" agent definitely-not-an-agent
check_fail "unknown output style" run "$t" output-style definitely-not-a-style
check_fail "unknown target"       run "$t" bogus
check_fail "skill with no name"   run "$t" skill
check_fail "agent with no name"   run "$t" agent
check_fail "nothing was created on any failure" test -d "$t/.claude"

# No arguments means INSTALL EVERYTHING. That path clones into a real home and
# runs make, so it is asserted structurally rather than executed — running it
# from a test would mutate the machine.
echo "no arguments routes to the full install, not to usage"
if grep -qE '^[[:space:]]*"") *install_everything' "$GET"; then ok "empty target routes to install_everything"; else no "empty target does not route to install_everything"; fi
if grep -q 'make -C "\$SEI_INTERNAL_SKILLS_HOME" sync-all' "$GET"; then ok "full install still calls make sync-all"; else no "full install lost its sync-all"; fi
if grep -q 'git -C "\$SEI_INTERNAL_SKILLS_HOME" pull --ff-only' "$GET"; then ok "full install still fast-forwards an existing checkout"; else no "full install lost its fast-forward"; fi

echo "-h prints usage and exits 0"
check "usage exits 0"     run "$scratch/u" -h
grep_out "usage names the targets" "output-style" "$scratch/u" -h
grep_out "usage explains the default mode" "no arguments" "$scratch/u" -h
grep_out "usage shows the curl one-liner" \
  "curl -fsSL https://raw.githubusercontent.com/sei-protocol/sei-internal-skills/main/scripts/install.sh" "$scratch/u" -h

# The repository is public. A message that calls it internal sends an engineer
# after gh auth they do not need.
echo "the script states a public repository"
if grep -q 'curl -fsSL "https://codeload.github.com/' "$GET"; then ok "the script has a curl fetch path"; else no "the script has no curl fetch path"; fi
if ! grep -qi 'internal repo\|is[[:space:]]internal' "$GET"; then ok "no 'internal' claim"; else no "the script still calls the repository internal"; fi

# The documented invocation pipes this script into bash, where $0 is "bash".
# The sibling scripts print usage by grepping $0; that idiom silently reads the
# shell binary here, which is how it broke the first time.
echo "the PIPED invocation works — that is the documented one"
t="$scratch/pipe"
piped() { cat "$GET" | SEI_INTERNAL_SKILLS_HOME="$REPO" SEI_SKILLS_TARGET="$t" bash -s -- "$@"; }
check "piped fetch exits 0"       piped skill kubernetes
check "piped fetch landed"        test -f "$t/.claude/skills/kubernetes/SKILL.md"
check "piped usage exits 0"       piped -h
u="$(piped -h 2>&1)"
if [[ "$u" == *"No such file or directory"* ]]; then no "piped usage does not grep \$0"; else ok "piped usage does not grep \$0"; fi
if [[ "$u" == *"output-style"* ]];             then ok "piped usage names the targets"; else no "piped usage names the targets"; fi
check_fail "piped unknown target fails"  piped bogus

# Shims that stand in for the network. Each one sits first on PATH, so the
# script under test finds it before the real tool, and the suite stays offline.
shim() {  # shim <dir> <name> <body>
  mkdir -p "$1"
  printf '#!/usr/bin/env bash\n%s\n' "$3" > "$1/$2"
  chmod +x "$1/$2"
}
shim "$scratch/nonet" curl 'exit 1'
shim "$scratch/nonet" gh   'exit 1'

# A path that is not a checkout must fall through to the download rather than be
# read as a tree. Reading a wrong directory would install nothing and claim success.
echo "a non-checkout SEI_INTERNAL_SKILLS_HOME does not masquerade as a tree"
o="$(PATH="$scratch/nonet:$PATH" SEI_INTERNAL_SKILLS_HOME="$scratch" SEI_SKILLS_TARGET="$scratch/bad" bash "$GET" list 2>&1)"; rc=$?
if [[ "$o" == *"reading your checkout"* ]]; then no "does not read a non-checkout as a tree"; else ok "does not read a non-checkout as a tree"; fi
if [ "$rc" -ne 0 ] && [[ "$o" == *"could not fetch"* ]]; then ok "no curl and no gh: could not fetch"; else no "no curl and no gh (rc=$rc)"; fi

# A tarball shaped like the one GitHub serves: one top-level directory that
# holds the tree. The shims log their arguments, so a case can name the URL.
tb="$scratch/tb/sei-protocol-sei-internal-skills-0000000"
mkdir -p "$tb/.claude/skills" "$tb/.claude/agents"
cp -R "$REPO/.claude/skills/kubernetes" "$tb/.claude/skills/kubernetes"
cp "$REPO/.claude/agents/sre-engineer.md" "$tb/.claude/agents/"
tar czf "$scratch/tree.tgz" -C "$scratch/tb" "$(basename "$tb")"
log="$scratch/net.log"
serve="printf '%s %s\\n' \"\$(basename \"\$0\")\" \"\$*\" >> '$log'; cat '$scratch/tree.tgz'"
shim "$scratch/curlnet" gh   'exit 1'
shim "$scratch/curlnet" curl "$serve"
shim "$scratch/ghnet"   gh   "case \"\$1\" in auth) exit 0 ;; api) $serve ;; *) exit 1 ;; esac"
shim "$scratch/ghnet"   curl 'exit 1'
# shellcheck disable=SC2016  # the shim expands $1 when it runs
shim "$scratch/ghfail"  gh   'case "$1" in auth) exit 0 ;; *) exit 1 ;; esac'
shim "$scratch/ghfail"  curl "$serve"

fetch() {  # fetch <shim-dir> <target-root> <args...>
  local p="$1" t="$2"; shift 2
  : > "$log"
  PATH="$p:$PATH" SEI_INTERNAL_SKILLS_HOME="$scratch/none" SEI_SKILLS_REF=test-ref \
    SEI_SKILLS_TARGET="$t" bash "$GET" "$@"
}

echo "with no checkout and no gh, curl fetches the tree from codeload"
t="$scratch/dl-curl"
check "curl path exits 0"      fetch "$scratch/curlnet" "$t" skill kubernetes
check "curl path installed it" test -f "$t/.claude/skills/kubernetes/SKILL.md"
if grep -q '^curl .*https://codeload.github.com/sei-protocol/sei-internal-skills/tar.gz/test-ref' "$log"; then
  ok "curl asked codeload for the ref"; else no "curl did not ask codeload for the ref"; fi

echo "a signed-in gh fetches the tarball first"
t="$scratch/dl-gh"
check "gh path exits 0"        fetch "$scratch/ghnet" "$t" agent sre-engineer
check "gh path installed it"   test -f "$t/.claude/agents/sre-engineer.md"
if grep -q '^gh api repos/sei-protocol/sei-internal-skills/tarball/test-ref' "$log"; then
  ok "gh asked for the ref's tarball"; else no "gh did not ask for the ref's tarball"; fi

echo "a failed gh fetch falls back to curl"
t="$scratch/dl-fallback"
check "fallback exits 0"        fetch "$scratch/ghfail" "$t" skill kubernetes
check "fallback installed it"   test -f "$t/.claude/skills/kubernetes/SKILL.md"
if grep -q '^curl ' "$log"; then ok "curl ran after gh failed"; else no "curl did not run after gh failed"; fi

echo ""
echo "install: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
