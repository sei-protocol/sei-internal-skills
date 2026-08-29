#!/usr/bin/env bash
# The coverage manifest says what each anchor and each contract actually checks.
# This asserts the manifest is true, because a manifest nobody checks is a
# claim, and the registry's single-source-of-truth promise rests on it.
#
# THE ROOT IS ANCHORED TO THIS SCRIPT, NOT TO THE CALLER'S DIRECTORY. Bound to
# the working directory, every set came out empty, every invariant held
# vacuously, and it printed success having checked nothing. A checker whose
# empty case is "pass" gives the same answer for "all is well" and "I found
# nothing", so an empty rule set is now a failure.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
exec python3 - "$ROOT" <<'PY'
import pathlib, sys, yaml
from collections import defaultdict

root = pathlib.Path(sys.argv[1])
cov_dir, rules_dir = root / 'coverage', root / 'styles' / 'AgenticWriting'

on_disk = {p.stem for p in rules_dir.glob('*.yml')}   # matches Vale's own discovery:
                                                      # flat, .yml only. Do not widen.
claimed, bad, totals = defaultdict(list), [], {}

if not on_disk:
    print(f"FAIL no rule files under {rules_dir} — refusing to report success on an empty set")
    sys.exit(1)

for f in sorted(cov_dir.glob('*.yml')):
    doc = yaml.safe_load(f.read_text())
    if not isinstance(doc, dict):
        bad.append(f"{f.name}: does not parse to a mapping")
        continue
    owner = doc.get('anchor') or doc.get('contract')
    if not owner:
        bad.append(f"{f.name}: neither 'anchor' nor 'contract' is set")
        continue

    topics = doc.get('topics')
    if not isinstance(topics, dict) or not topics:
        bad.append(f"{f.name}: 'topics' is empty or missing — a file claiming coverage must list some")
        continue

    declared, enumerated = doc.get('total_topics'), doc.get('enumerated')
    if enumerated not in ('full', 'partial'):
        bad.append(f"{f.name}: 'enumerated' must be 'full' or 'partial', got {enumerated!r}")
    elif not isinstance(declared, int):
        bad.append(f"{f.name}: 'total_topics' must be an integer")
    elif enumerated == 'full' and declared != len(topics):
        bad.append(f"{f.name}: enumerated is full but total_topics={declared} and {len(topics)} topics are listed")
    elif enumerated == 'partial' and declared < len(topics):
        bad.append(f"{f.name}: total_topics={declared} is smaller than the {len(topics)} topics listed")

    covered = 0
    for topic, value in topics.items():
        if value is False:
            continue
        if not isinstance(value, list) or not value or not all(isinstance(x, str) for x in value):
            bad.append(f"{f.name}: topic '{topic}' must be false or a non-empty list of rule names")
            continue
        covered += 1
        for rule in value:
            claimed[rule].append((f.name, topic))
            if rule not in on_disk:
                bad.append(f"{f.name}: topic '{topic}' names '{rule}', which is not a rule file")
    totals[owner] = (covered, len(topics), declared, enumerated)

for r in sorted(on_disk - set(claimed)):
    bad.append(f"rule '{r}' appears in no coverage file — its purpose is unrecorded. See coverage/README.md")

for rule, where in sorted(claimed.items()):
    if len(where) > 1:
        bad.append(f"rule '{rule}' is claimed more than once ({', '.join(f'{f}:{t}' for f, t in where)})")

# The manifest and the registry assert the same thing twice, on purpose. The
# arc42 incident was false attribution, and an orphan check catches only a rule
# with NO recorded purpose, never one with two conflicting purposes.
reg = yaml.safe_load((root / 'anchors' / 'registry.yaml').read_text())
cov_by_anchor, declared = defaultdict(set), {}
for f in sorted(cov_dir.glob('*.yml')):
    doc = yaml.safe_load(f.read_text())
    if isinstance(doc, dict) and doc.get('anchor'):
        declared[doc['anchor']] = f.name
        for v in (doc.get('topics') or {}).values():
            if isinstance(v, list):
                cov_by_anchor[doc['anchor']] |= set(v)
# A coverage file for an anchor the registry does not list is never visited by
# the loop below, so every claim in it passes unexamined. The admission gate
# reads coverage/<id>.yml by name, so a file whose name and owner disagree is
# the same hole wearing a different hat.
# Read from the declared owner, not from the rules it credits. An anchor whose
# topics are all `false` credits no rules, so a set built from the credits alone
# does not contain it and the ghost passes.
known = {a['id'] for a in reg['anchors']}
for aid, fname in sorted(declared.items()):
    if aid not in known:
        bad.append(f"{fname}: claims anchor '{aid}', which the registry does not list — nothing cross-checks it")
    if fname != f'{aid}.yml':
        bad.append(f"{fname}: owns anchor '{aid}' — the admission gate reads coverage/{aid}.yml and will not find it")

for a in reg['anchors']:
    want = {x.split('.', 1)[1] for x in (a['verifier'].get('rules') or [])}
    got = cov_by_anchor.get(a['id'], set())
    for r in sorted(want - got):
        bad.append(f"registry credits '{a['id']}' with '{r}', coverage does not")
    for r in sorted(got - want):
        bad.append(f"coverage credits '{a['id']}' with '{r}', the registry does not")

# A ratio against a partial enumeration reads as a percentage whatever the file
# says, so the columns are never paired.
print("owner                    coverage")
for owner, (c, listed, declared, enum) in sorted(totals.items()):
    suffix = "" if enum == 'full' else f" · {declared} stated"
    print(f"{owner:<24}{c} checked · {listed} examined{suffix}")
print(f"\nrule files: {len(on_disk)}   claimed: {len(claimed)}   orphans: {len(on_disk - set(claimed))}")

if bad:
    print()
    for b in bad:
        print(f"FAIL {b}")
    sys.exit(1)
print("\nCoverage manifest is true: every rule has a home, every claim resolves, and the registry agrees.")
PY
