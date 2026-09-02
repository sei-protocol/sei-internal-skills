#!/usr/bin/env bash
# A rule this repository scopes to a path must be scoped for consumers too.
#
# .vale.ini and templates/consumer.vale.ini both decide which rules run where,
# and nothing kept them in step. Three rules had already drifted:
# Design-Arc42Order, EARS-CriterionShall and Spec-AcceptanceCriteria were
# path-scoped here and unscoped there.
#
# The consequence is asymmetric, which is why this is a gate and not a habit.
# A structure rule left unscoped runs on every Markdown file a consumer has, so
# `vale` reports that their README is missing an Acceptance Criteria heading.
# New rules are on by default here on purpose; a structure rule still belongs on
# the path its structure describes.
#
# A rule scoped in NEITHER file is fine. Those are the prose rules, which are
# meant to run everywhere.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
exec python3 - "$ROOT" <<'PY'
import pathlib, re, sys

root = pathlib.Path(sys.argv[1])

def disabled_in_default_section(path):
    """Rules turned OFF in [*.md].

    Naming a rule somewhere is not scoping it. A rule enabled on its own path
    but never disabled in [*.md] still runs on every file, which is the exact
    drift this gate exists to catch, so the section boundary matters.
    """
    out, section = set(), None
    for line in pathlib.Path(path).read_text().splitlines():
        s = line.strip()
        if s.startswith('['):
            section = s
            continue
        # The prefix takes a hyphen too: `write-good` is a style name.
        m = re.match(r'([A-Za-z0-9-]+)\.([A-Za-z0-9-]+)\s*=\s*NO\s*$', s)
        if m and section == '[*.md]':
            out.add(f'{m.group(1)}.{m.group(2)}')
    return out

# The repository's own configuration sits at the root, one level above this
# toolkit. The consumer template ships inside it.
repo = disabled_in_default_section(root.parent / '.vale.ini')
consumer = disabled_in_default_section(root / 'templates' / 'consumer.vale.ini')
rules = {f'AgenticWriting.{p.stem}' for p in (root / 'styles' / 'AgenticWriting').glob('*.yml')}

# A difference that is deliberate, with the reason. This gate catches drift; a
# recorded decision is not drift, and hiding it behind a narrower pattern was
# not the same as accounting for it.
DELIBERATE = {
    'Vale.Spelling': 'a global spell check reports every identifier and product '
                     'name, so it needs a curated accept list. This repository '
                     'has one; a repository installing the toolkit does not yet.',
}

if not rules:
    print("FAIL no rule files found — refusing to report success on an empty set")
    sys.exit(1)

bad = []
for r in sorted((repo ^ consumer) & set(DELIBERATE)):
    print(f"deliberate  {r}: {DELIBERATE[r]}")
repo -= set(DELIBERATE)
consumer -= set(DELIBERATE)

for r in sorted(repo - consumer):
    bad.append(f"'{r}' is path-scoped in .vale.ini and unscoped in templates/consumer.vale.ini, "
               f"so a consumer runs it on every Markdown file")
for r in sorted(consumer - repo):
    bad.append(f"'{r}' is scoped for consumers and unscoped here, so this repository "
               f"does not test the rule the way a consumer runs it")
# Only AgenticWriting rules have a file under styles/AgenticWriting. A key from
# Vale's own style or from a package names a rule this repository does not ship.
for r in sorted(k for k in (repo | consumer) - rules if k.startswith('AgenticWriting.')):
    bad.append(f"'{r}' is named in a configuration but is not a rule file")

print(f"rules: {len(rules)}   off in [*.md] here: {len(repo)}   for consumers: {len(consumer)}")
print(f"on everywhere in both, by design: {len(rules - repo - consumer)}")

if bad:
    print()
    for b in bad:
        print(f"FAIL {b}")
    sys.exit(1)
print("\nThe two configurations scope the same rules.")
PY
