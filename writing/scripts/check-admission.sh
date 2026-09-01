#!/usr/bin/env bash
# An anchor arrives with four artifacts or it does not arrive. This enforces
# that, and counts the anchors exempt from it.
#
# The exemption is the point. Every anchor present when the rule was written
# predates it, so exempting them silently would make the rule decorative. They
# are marked `grandfathered` in the registry and listed in
# anchors/grandfathered.txt.
#
# "THE COUNT ONLY SHRINKS" WAS A SENTENCE THIS SCRIPT PRINTED, NOT A RULE IT
# APPLIED. Marking a new anchor `grandfathered` bought a permanent exemption
# and the gate said nothing. Two checks now hold it:
#
#   * the registry's grandfathered set must equal the file, so an exemption is
#     always a visible one-line diff;
#   * the file is compared against origin/main and may not gain a line.
#
# The comparison needs history. A shallow clone has none, so the script says
# which arm it ran rather than passing quietly on one arm.
#
# The root is anchored to this script, not to the caller's directory, and an
# empty anchor set is a failure. A checker whose empty case is "pass" answers
# "all is well" and "I found nothing" identically.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
exec python3 - "$ROOT" <<'PY'
import os, pathlib, subprocess, sys, yaml


BASELINE_REF = os.environ.get('AGENTIC_BASELINE_REF', 'origin/main')


def git_grew(root, rel, current):
    """Which ids this file gained since the baseline ref.

    Reports how it decided. A shallow clone has no origin/main, and a check
    that quietly skips its own comparison is the defect this arm exists to fix.
    AGENTIC_BASELINE_REF names another ref, which is how this arm is tested.
    """
    def git(*args):
        return subprocess.run(['git', '-C', str(root), *args],
                              capture_output=True, text=True, timeout=15)
    try:
        if git('rev-parse', '--verify', '--quiet', BASELINE_REF).returncode != 0:
            return {'added': set(), 'how': f'not compared — {BASELINE_REF} is absent. A shallow '
                                           'clone has no history; this run checked the list '
                                           'against the registry only'}
        # An absent ref and an absent path are different facts. Treating the
        # second as an empty list would call every line new and fail the commit
        # that introduces the baseline.
        prev = git('show', f'{BASELINE_REF}:{rel}')
        if prev.returncode != 0:
            return {'added': set(), 'how': f'not compared — {rel} does not exist on {BASELINE_REF} yet'}
    except (OSError, subprocess.SubprocessError) as e:
        return {'added': set(), 'how': f'not compared — git did not run ({e})'}
    was = {l.strip() for l in prev.stdout.splitlines()
           if l.strip() and not l.strip().startswith('#')}
    added = current - was
    return {'added': added, 'how': f'compared against {BASELINE_REF} — {len(was)} there, '
                                   f'{len(current)} here, {len(was - current)} graduated'}


root = pathlib.Path(sys.argv[1])
reg = yaml.safe_load((root / 'anchors' / 'registry.yaml').read_text())
anchors = reg.get('anchors') or []
bad, rows, exempt = [], [], []

if not anchors:
    print("FAIL no anchors in the registry — refusing to report success on an empty set")
    sys.exit(1)

def why_uncounted(entry):
    """A count is a number someone measured, not a key someone added.

    `EARS-CriterionShall:` with nothing under it parses to None and used to
    satisfy artifact 4, because the check asked whether the key was present.
    Returns the reason it does not count, or None when it does.
    """
    if entry is ...:
        return "is not listed under false_positives.rules"
    if not isinstance(entry, dict) or not entry:
        return f"is listed but holds {entry!r} — an empty entry is not a measurement"
    if not entry.get('severity'):
        return "records no severity, so the count is not tied to a gate level"
    for sample in entry.values():
        if isinstance(sample, dict):
            n = sample.get('false_positives')
            if isinstance(n, int) and not isinstance(n, bool):
                return None
    return "has no sample carrying an integer false_positives"


def artifacts(a):
    """The four artifacts, computed rather than declared."""
    aid = a['id']
    rules = [r.split('.', 1)[1] for r in (a['verifier'].get('rules') or [])]
    cov = root / 'coverage' / f'{aid}.yml'

    have_entry = bool(a.get('steward') and a.get('license') and a.get('recognition'))
    have_cov = cov.is_file()

    # All three pieces. expected.txt alone was enough, and the harness skips a
    # directory with no test.md, so a rule could be "covered" by a fixture that
    # never ran.
    fixture_dir = root / 'evals' / 'rules'
    missing_fixture = [(r, piece) for r in rules
                       for piece in ('.vale.ini', 'test.md', 'expected.txt')
                       if not (fixture_dir / r / piece).is_file()]
    have_fixtures = not missing_fixture

    measured = {}
    if have_cov:
        doc = yaml.safe_load(cov.read_text()) or {}
        fp = doc.get('false_positives') or {}
        measured = fp.get('rules') or {}
    missing_count = [(r, why_uncounted(measured.get(r, ...))) for r in rules
                     if why_uncounted(measured.get(r, ...))]
    have_counts = not missing_count

    return rules, have_entry, have_fixtures, have_cov, have_counts, missing_fixture, missing_count

for a in anchors:
    aid = a['id']
    status = a.get('admission')
    if status not in ('admitted', 'grandfathered'):
        bad.append(f"anchor '{aid}': admission must be 'admitted' or 'grandfathered', got {status!r}")
        continue

    rules, entry, fixtures, cov, counts, miss_fix, miss_cnt = artifacts(a)
    mark = lambda b: 'yes' if b else 'no '
    note = 'no rules' if not rules else f'{len(rules)} rules'
    rows.append(f"{aid:<24}{status:<16}{mark(entry):<7}{mark(fixtures):<10}{mark(cov):<10}{mark(counts):<8}{note}")

    if status == 'grandfathered':
        exempt.append(aid)
        continue

    # An anchor with no rules satisfied artifacts 2 and 4 by vacuous truth:
    # "every rule has a fixture" is true of no rules. A standard nothing checks
    # is exactly what the admission rule exists to keep out.
    if not rules:
        bad.append(f"anchor '{aid}' is admitted but names no rules, so it checks nothing. "
                   f"An anchor with no verifier belongs at coverage: none and admission: grandfathered")

    if not entry:
        bad.append(f"anchor '{aid}' is admitted but its registry entry lacks steward, licence or recognition test")
    if not cov:
        bad.append(f"anchor '{aid}' is admitted but has no coverage/{aid}.yml")
    for r, piece in miss_fix:
        bad.append(f"anchor '{aid}' is admitted but rule '{r}' has no {piece} at evals/rules/{r}/")
    for r, why in miss_cnt:
        bad.append(f"anchor '{aid}' is admitted but rule '{r}' {why} in coverage/{aid}.yml")

print(f"{'anchor':<24}{'admission':<16}{'entry':<7}{'fixtures':<10}{'coverage':<10}{'counts':<8}rules")
for r in sorted(rows):
    print(r)
baseline_file = root / 'anchors' / 'grandfathered.txt'
baseline = {l.strip() for l in baseline_file.read_text().splitlines()
            if l.strip() and not l.strip().startswith('#')}

for aid in sorted(set(exempt) - baseline):
    bad.append(f"anchor '{aid}' is marked grandfathered but is not in anchors/grandfathered.txt. "
               f"The exemption is for anchors that predate the rule; a new one is a decision, "
               f"so write the line and let a reviewer see it")
for aid in sorted(baseline - set(exempt)):
    bad.append(f"anchors/grandfathered.txt still lists '{aid}', which is no longer grandfathered. "
               f"Delete the line in the commit that promotes it")

grew = git_grew(root, 'writing/anchors/grandfathered.txt', baseline)

print(f"\ngrandfathered: {len(exempt)} of {len(anchors)} — {', '.join(sorted(exempt)) or 'none'}")
print(f"monotonicity: {grew['how']}")
if grew['added']:
    for aid in sorted(grew['added']):
        bad.append(f"anchors/grandfathered.txt gained '{aid}' since {BASELINE_REF}. The list only shrinks: "
                   f"an anchor leaves it by earning all four artifacts, and nothing puts one back")

if bad:
    print()
    for b in bad:
        print(f"FAIL {b}")
    sys.exit(1)
print("\nEvery admitted anchor carries its four artifacts.")
PY
