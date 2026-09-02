#!/usr/bin/env bash
# A rule this repository scopes to a path needs the same scope in every configuration.
#
# Three files decide which rules run where, and nothing kept them in step:
# .vale.ini, templates/consumer.vale.ini, and the user-level reference under
# docs/. Drift has reached all three. Design-Arc42Order, EARS-CriterionShall and
# Spec-AcceptanceCriteria were path-scoped in .vale.ini and unscoped in the
# consumer template; EARS-CriterionShall and Spec-AcceptanceCriteria were then
# unscoped again in the reference.
#
# The consequence is asymmetric, which is why this is a gate and not a habit.
# A structure rule left unscoped runs on every Markdown file, so `vale` reports
# that a README is missing an Acceptance Criteria heading. Each configuration
# below carries the consequence of its own omission, because the three differ.
# The reference is a user-level global. An omission there reaches every
# repository on the machine that has no .vale.ini of its own.
#
# New rules are on by default in .vale.ini on purpose; a structure rule still
# belongs on the path its structure describes.
#
# A rule scoped in NONE of the three is fine. Those are the prose rules, which
# are meant to run everywhere.
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

# The three configurations, each with the consequence of leaving a rule
# unscoped in it. The repository's own configuration sits at the root, one level
# above this toolkit. The consumer template and the reference ship inside it.
#
# The reference is documentation today: nothing reads it, and install.sh writes
# no user-level config. Issue #373 restores that copy, which turns every
# omission here into every machine's global rule set. This gate holds it to the
# same symmetry as the other two for that reason.
CONFIGS = [
    ('.vale.ini',
     root.parent / '.vale.ini',
     'this repository does not test the rule the way a consumer runs it'),
    ('templates/consumer.vale.ini',
     root / 'templates' / 'consumer.vale.ini',
     'a consumer runs it on every Markdown file'),
    ('docs/vale-global-config.reference.ini',
     root / 'docs' / 'vale-global-config.reference.ini',
     'a machine install runs it on every Markdown file, in every repository '
     'that has no .vale.ini of its own'),
]

missing = [label for label, path, _ in CONFIGS if not path.is_file()]
if missing:
    # Named, not a traceback. A renamed or moved configuration is drift of its
    # own, and nothing else in the repository points at the reference file.
    for label in missing:
        print(f"FAIL {label} is not there — a configuration this gate compares has moved")
    sys.exit(1)

off = {label: disabled_in_default_section(path) for label, path, _ in CONFIGS}
consequence = {label: text for label, _, text in CONFIGS}
labels = [label for label, _, _ in CONFIGS]
rules = {f'AgenticWriting.{p.stem}' for p in (root / 'styles' / 'AgenticWriting').glob('*.yml')}

# A difference that is deliberate, with the reason. This gate catches drift; a
# recorded decision is not drift, and hiding it behind a narrower pattern was
# not the same as accounting for it.
DELIBERATE = {
    'Vale.Spelling': 'a global spell check reports every identifier and product '
                     'name, so it needs a curated accept list. This repository '
                     'has one; a repository installing the toolkit does not yet, '
                     'and a user-level global configuration never will.',
}

if not rules:
    print("FAIL no rule files found — refusing to report success on an empty set")
    sys.exit(1)

# A rule is in step when every configuration scopes it or none does. Anything
# in between is drift, and the report names the configurations that omit it.
split = {r for r in set().union(*off.values())
         if any(r in off[l] for l in labels) and any(r not in off[l] for l in labels)}

bad = []
for r in sorted(split & set(DELIBERATE)):
    print(f"deliberate  {r}: {DELIBERATE[r]}")
for label in labels:
    off[label] -= set(DELIBERATE)
split -= set(DELIBERATE)

for r in sorted(split):
    scoped = [l for l in labels if r in off[l]]
    for label in [l for l in labels if r not in off[l]]:
        bad.append(f"'{r}' is path-scoped in {' and '.join(scoped)} and unscoped in "
                   f"{label}, so {consequence[label]}")
# Only AgenticWriting rules have a file under styles/AgenticWriting. A key from
# Vale's own style or from a package names a rule this repository does not ship.
named = set().union(*off.values())
for r in sorted(k for k in named - rules if k.startswith('AgenticWriting.')):
    bad.append(f"'{r}' is named in a configuration but is not a rule file")

print(f"rules: {len(rules)}")
for label in labels:
    print(f"  off in [*.md]: {len(off[label]):3}   {label}")
print(f"on everywhere in all three, by design: {len(rules - named)}")

if bad:
    print()
    for b in bad:
        print(f"FAIL {b}")
    sys.exit(1)
print(f"\nThe {len(CONFIGS)} configurations scope the same rules.")
PY
