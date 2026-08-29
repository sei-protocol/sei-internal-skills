#!/usr/bin/env bash
# An anchor cites a published standard. It does not cite a skill in this repository.
#
# THIS REPLACES A CHECK WHOSE PREMISE DISSOLVED. In its own repository the toolkit
# was public and the skills were not, so the rule was "name no private skill".
# Here the skills sit two directories away and are public, which makes that rule
# both meaningless and unenforceable: 245 files would trip it.
#
# The rule worth keeping is the one underneath it. An anchor earns its place by
# being a standard somebody else published and maintains, so a reader can follow
# the name to a clause this repository does not control. A skill in this
# repository is not that. Citing one as an anchor's authority makes the catalogue
# circular: the rule is right because our skill says so.
#
# Scope is the anchor layer only — the registry, the anchor pages, and the
# coverage manifest. Prose elsewhere may reference a skill freely; the contract
# does that itself.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO="$(cd "$ROOT/.." && pwd)"
cd "$REPO"

# Every skill and agent this repository ships, by directory name.
names="$( { ls -d .claude/skills/*/ experimental/skills/*/ agents/*/ 2>/dev/null | xargs -n1 basename
            ls .claude/agents/*.md 2>/dev/null | xargs -n1 basename | sed 's/\.md$//'
          } | sort -u | grep -e '-' || true )"

if [ -z "$names" ]; then
  echo "found no skill or agent names to check against — refusing to report success"
  echo "  on an empty denylist. Did .claude/skills or .claude/agents move?"
  exit 1
fi

pattern="$(printf '%s' "$names" | paste -sd'|' -)"
scanned=0 fail=0

while IFS= read -r f; do
  [ -f "$f" ] || continue
  scanned=$((scanned + 1))
  if grep -nIE "(^|[^A-Za-z0-9/-])($pattern)([^A-Za-z0-9-]|$)" "$f" >/dev/null 2>&1; then
    echo "AN ANCHOR CITES A SKILL in $f:"
    grep -nIE "(^|[^A-Za-z0-9/-])($pattern)([^A-Za-z0-9-]|$)" "$f" | sed 's/^/    /'
    echo "    An anchor names a standard a reader can follow outside this repository."
    fail=1
  fi
# .txt too: grandfathered.txt and unregistered.txt are lists of anchor names,
# which is the likeliest place for a skill name to appear, and the first
# version skipped them.
done < <(find writing/anchors writing/coverage -type f \
         \( -name '*.md' -o -name '*.yaml' -o -name '*.yml' -o -name '*.txt' \) 2>/dev/null)

if [ "$fail" -eq 0 ]; then
  echo "No anchor cites a skill. Checked $scanned files against $(printf '%s' "$names" | wc -l | tr -d ' ') names."
fi
exit "$fail"
