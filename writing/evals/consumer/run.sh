#!/usr/bin/env bash
# End-to-end test of every path a user installs on.
#
# Three of them carried every user and none had ever run:
#
#   scripts/install.sh machine     the documented one-line install
#   scripts/install.sh repo        writes the config, the workflow and the rules
#   writing-contract.yml           the reusable workflow their CI calls
#
# Everything this repository verifies about itself goes through .github/workflows
# /writing.yml, which is a different file that installs rules differently. Green
# there said nothing about any of the three.
#
# Machine mode runs first, into a home of its own. Then this builds a scratch
# repository from evals/consumer/tree, installs into it, asserts what the
# installer claims it wrote, and runs the same setup script the reusable workflow
# runs, comparing the lint output against a golden file.
#
# NOT COVERED, and worth saying: the vale-action step, reviewdog's reporting, and
# the cross-repository checkout that fetches the rules. Those need real GitHub,
# and the checkout is where the first real consumer run failed — the workflow read
# github.workflow_ref, which is the caller's entry workflow, so it asked for a
# pull-request ref that exists only in the caller. Nothing local could see that.
# The assertion below is what a local test CAN hold: the right variable name.
set -uo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
ref="${GITHUB_SHA:-$(git -C "$root" rev-parse HEAD)}"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fail=0

note() { printf '  %s\n' "$1"; }
check() {  # check <description> <expected> <actual>
  if [ "$2" = "$3" ]; then
    note "ok   $1"
  else
    note "FAIL $1"
    note "       expected: $2"
    note "       actual:   $3"
    fail=1
  fi
}

# MACHINE MODE FIRST. It is the documented one-line install and nothing ran it,
# so the directory move broke it in silence: two symlinks pointing at the
# pre-move path. `ln -sfn` replaces an existing link, so re-running it on a
# working machine destroyed that install too.
#
# XDG_CONFIG_HOME TRAVELS WITH HOME, and off Darwin it decides alone. install.sh
# resolves the user Vale directory as "${XDG_CONFIG_HOME:-$HOME/.config}/vale"
# there, so overriding HOME by itself left the installer writing into the
# developer's own directory: their styles/AgenticWriting and styles/config links
# replaced by links into $fake_home, which the rm below then leaves dangling.
# Measured with the variable set -- both links clobbered, and the check still
# reported zero, because find under $fake_home never saw them. That is the
# destruction this test exists to catch, inflicted by the test.
fake_home="$(mktemp -d)"
# Clone first so the installer finds an existing checkout and skips its own
# `git clone --branch`, which needs a branch name and gets handed a commit.
git clone --quiet "$root" "$fake_home/.agentic-writing"
git -C "$fake_home/.agentic-writing" checkout --quiet "$ref"
if env HOME="$fake_home" XDG_CONFIG_HOME="$fake_home/.config" \
       AGENTIC_WRITING_HOME="$fake_home/.agentic-writing" \
       AGENTIC_WRITING_REPO="$root" AGENTIC_WRITING_REF="$ref" \
       "$root/writing/scripts/install.sh" machine >"$fake_home/log" 2>&1; then
  note "ok   machine mode installs"
else
  note "FAIL machine mode installs"; sed 's/^/       /' "$fake_home/log"; fail=1
fi
# COUNTED, NOT ONLY INSPECTED. A dangling count over an empty set is zero, so the
# assertion below reported success on an install that had written somewhere else
# entirely. Containment is worth having only if something fails when it breaks,
# and nothing here could until this counted what it had to look at.
links=0 dangling=0
while IFS= read -r l; do
  links=$((links + 1))
  [ -e "$l" ] || dangling=$((dangling + 1))
done < <(find "$fake_home" -type l 2>/dev/null)
# NAME THE PATH, DO NOT COUNT. A count is green on any one symlink anywhere under
# $fake_home, so it survives the installer dropping `config`, and -- the defect
# this block exists for -- it survives the destination moving: a link that
# resolves somewhere Vale never reads is still live and still counted. These
# assert where Vale actually looks, which is what broke.
#
# RESOLVED THE WAY install.sh RESOLVES IT, not written out. The two platforms put
# the user Vale directory in different places, so a literal path passes on one and
# fails on the other -- which is how this assertion was first written, green on
# Linux and red on a developer's Mac.
case "$(uname -s)" in
  Darwin) want_vale_dir="$fake_home/Library/Application Support/vale" ;;
  *)      want_vale_dir="$fake_home/.config/vale" ;;
esac
for style in AgenticWriting config; do
  check "machine mode links $style where Vale reads it" "yes" \
    "$(if [ -L "$want_vale_dir/styles/$style" ] \
        && [ -e "$want_vale_dir/styles/$style" ]; then echo yes; else echo no; fi)"
done
check "machine mode leaves no dangling symlink" "0" "$dangling"
rm -rf "$fake_home"

cp -R "$root/writing/evals/consumer/tree/." "$work/"
git -C "$work" init -q .

# CD FIRST. install.sh resolves the repository from the working directory, which
# is correct, so running it from anywhere else installs into that repository
# instead. The first draft of this test did exactly that and wrote a consumer
# workflow into agentic-writing. The installer now refuses that case; this stays
# because a test should not depend on the guard it is meant to leave alone.
cd "$work"
[ "$(git rev-parse --show-toplevel)" = "$(cd "$work" && pwd -P)" ] || {
  echo "refusing to install: the working directory is not the scratch repository" >&2
  exit 1; }

AGENTIC_WRITING_HOME="$root" AGENTIC_WRITING_REPO="$root" AGENTIC_WRITING_REF="$ref" \
  "$root/writing/scripts/install.sh" repo >install.log 2>&1 || {
    echo "install.sh repo failed:"; sed 's/^/    /' install.log; exit 1; }

for f in .vale.ini .github/workflows/writing.yml .gitignore .vale/vocab/accept.txt; do
  [ -f "$f" ] && note "ok   wrote $f" || { note "FAIL did not write $f"; fail=1; }
done

# The pin is the whole update story. A workflow pinned to a ref that is not the
# one the rules came from fetches a different set on the consumer's next run.
check "workflow pins the installed ref" \
  "uses: sei-protocol/sei-internal-skills/.github/workflows/writing-contract.yml@$ref" \
  "$(grep -o 'uses: sei-protocol/sei-internal-skills/.github/workflows/writing-contract.yml@.*' .github/workflows/writing.yml)"
check "config records the same ref" "$ref" \
  "$(grep -o 'installs from: .*' .vale.ini | sed 's/installs from: //')"
check "fetched rules are gitignored" "yes" \
  "$(grep -qxF '.vale/styles/' .gitignore && echo yes || echo no)"

# THE RULES REPOSITORY'S OWN NAMES MUST NOT ARRIVE. Vale reads a vocabulary only
# from StylesPath/config/vocabularies, so the rules repository keeps its agent
# and skill identifiers in styles/config/vocabularies/Local -- and both install
# paths copy styles/config whole. A leak here puts another repository's names
# under Vale.Terms in this one, which reports every other casing of them as an
# error in prose that has nothing to do with them.
#
# Reads the entries rather than a count, so adding one to the rules repository
# extends this check instead of dating it. The file listing is the half an
# overwritten accept.txt cannot cover: a second file beside it survives the copy.
theirs="$root/writing/styles/config/vocabularies/Local/accept.txt"
leaked() {  # leaked <installed-Local-dir>
  find "$1" -mindepth 1 ! -name accept.txt -exec basename {} \; | tr '\n' ' '
  grep -v -e '^[[:space:]]*#' -e '^[[:space:]]*$' "$theirs" \
    | while IFS= read -r term; do
        grep -qxF "$term" "$1/accept.txt" 2>/dev/null && printf '%s ' "$term"
      done
}
check "install.sh grafts a Local the rules repository does not own" "" \
  "$(leaked .vale/styles/config/vocabularies/Local)"

# The one thing a local test can say about the cross-repository checkout.
# Read off the WORKFLOW_REF binding, not the file. Scanning the whole workflow
# matched the header comment first, so reverting the binding to
# github.workflow_ref -- the exact defect this exists to catch -- still passed.
check "the workflow resolves its own ref, not the caller's" "job_workflow_ref" \
  "$(grep -oE 'WORKFLOW_REF: \$\{\{ github\.[a-z_]+ \}\}' \
       "$root/.github/workflows/writing-contract.yml" \
     | head -1 | grep -oE 'github\.[a-z_]+' | sed 's/github\.//')"

# From here the script stops being the laptop and becomes the runner.
#
# A runner starts from a fresh checkout. .vale/styles/ and .vale/src/ are
# gitignored, so neither arrives, and install.sh never runs there — the only
# thing CI executes is the reusable workflow. Deleting them is what makes this
# half a test of that workflow rather than a second test of the installer.
rm -rf .vale/styles .vale/src

# COPY ONLY WHAT GIT TRACKS. CI fetches the rules with actions/checkout, which
# never carries a gitignored path, and styles/write-good and styles/proselint are
# gitignored here — they arrive by `vale sync`, not by checkout. The first draft
# copied the whole working tree, so the packages a laptop already had made the
# test pass over a consumer path that fails on a clean runner.
# TRACKED PATHS, WORKING-TREE CONTENT. `git archive HEAD` would test the last
# commit rather than the change in hand, which is a confusing way to iterate.
# Listing tracked files instead keeps the property that matters — a gitignored
# path never arrives, because actions/checkout never carries one — while still
# testing what is about to be pushed.
mkdir -p .vale/src
( cd "$root" && git ls-files -z writing/styles writing/scripts | tar -cf - --null -T - ) | tar -xf - -C .vale/src
files="$(./.vale/src/writing/scripts/consumer-lint-setup.sh .vale/src/writing 2>/dev/null)"
check "path selection skips what does not exist" '["README.md","docs","specs"]' "$files"

# The same boundary on the CI path. This one is the load-bearing half: a
# consumer's runner executes the reusable workflow and nothing else, so
# install.sh never runs there and this script is the only thing between the
# fetched tree and the lint.
check "the reusable workflow grafts a Local the rules repository does not own" "" \
  "$(leaked .vale/styles/config/vocabularies/Local)"

# The caller's own list is filtered too. Naming a tree the repository plans to
# have used to be a fatal Vale argument error rather than a no-op.
given="$(./.vale/src/writing/scripts/consumer-lint-setup.sh .vale/src/writing \
         '["README.md","docs","designs"]' 2>/dev/null)"
check "a named path that does not exist is skipped" '["README.md","docs"]' "$given"

./.vale/src/writing/scripts/consumer-lint-setup.sh .vale/src/writing 'not json' >/dev/null 2>&1
check "a malformed paths input fails loudly" "2" "$?"

actual="$(vale --no-global --no-exit --output=line --glob='!.vale/**' \
          README.md docs specs 2>&1 | sort)"
expected="$(cat "$root/writing/evals/consumer/expected.txt")"
if [ "$actual" = "$expected" ]; then
  note "ok   lint output matches the golden"
else
  note "FAIL lint output differs from evals/consumer/expected.txt"
  diff <(printf '%s\n' "$expected") <(printf '%s\n' "$actual") | sed 's/^/       /'
  fail=1
fi

[ "$fail" -eq 0 ] && note "The consumer path works: install, configure, fetch, lint."
exit "$fail"
