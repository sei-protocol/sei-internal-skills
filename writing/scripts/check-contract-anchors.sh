#!/usr/bin/env bash
# SC-002: every anchor the contract names resolves to a registry entry.
#
# THE CRITERION NAMED "the registry consistency job" AS ITS VERIFIER. That job checks
# the registry against the rule files and against coverage. Nothing read the contract,
# so the one direction SC-002 is about went unchecked, and twelve of the nineteen
# anchors the contract names had no registry entry at all.
#
# The gap is real, and closing it means writing twelve registry entries with the four
# admission artifacts each. Until then anchors/unregistered.txt names them, on the same
# terms as anchors/grandfathered.txt: the list is visible, it is printed on every run,
# and it may only shrink. A name that is in the contract, absent from the registry and
# absent from that file fails the build.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
exec python3 - "$ROOT" <<'PY'
import os, pathlib, re, subprocess, sys, unicodedata, yaml

root = pathlib.Path(sys.argv[1])
BASELINE_REF = os.environ.get('AGENTIC_BASELINE_REF', 'origin/main')
BASELINE = 'writing/anchors/unregistered.txt'


def key(s):
    """Fold a display name onto a registry id. 'ADR (Nygard)' and 'adr-nygard' meet
    here; so do 'Diátaxis' and 'diataxis', which differ only by an accent."""
    s = unicodedata.normalize('NFKD', s)
    s = ''.join(c for c in s if not unicodedata.combining(c))
    return re.sub(r'[^a-z0-9]', '', s.lower())


contract = (root / 'CONTRACT.md').read_text().splitlines()

# The anchor table is the one under "## The anchors" whose header column reads
# "Anchor". Anchoring on the heading keeps the writing-modes and spec-contract tables
# out of it.
named, in_section, in_table = [], False, False
for line in contract:
    if line.startswith('## '):
        in_section = line.strip() == '## The anchors'
        in_table = False
        continue
    if not in_section:
        continue
    if line.startswith('| Anchor '):
        in_table = True
        continue
    if in_table:
        if not line.startswith('|'):
            in_table = False
            continue
        cell = line.split('|')[1].strip()
        if cell and not set(cell) <= set('-: '):
            named.append(cell)

if not named:
    print("FAIL found no anchor table under '## The anchors' — refusing to report success "
          "on an empty set. Did the heading or the table header change?")
    sys.exit(1)

reg = yaml.safe_load((root / 'anchors' / 'registry.yaml').read_text())
known = {}
for a in reg['anchors']:
    known[key(a['id'])] = a['id']
    known[key(a.get('name', ''))] = a['id']

baseline_file = root / 'anchors' / 'unregistered.txt'
listed = [l.strip() for l in baseline_file.read_text().splitlines()
          if l.strip() and not l.strip().startswith('#')]
excused = {key(x) for x in listed}

# One lookup, because the two directions have to agree. Resolution accepts a prefix
# so the contract can cite 'EARS' against a registry name of 'EARS requirement
# syntax'; the staleness check below asks the same question in reverse, and asking it
# with exact matching let a prefix-resolved name sit in the debt file unflagged --
# which is the one thing that file promises cannot happen.
def resolve(name):
    k = key(name)
    return known.get(k) or next((v for kk, v in known.items() if kk and kk.startswith(k)), None)


bad, resolved, waiting = [], [], []
for name in named:
    k = key(name)
    hit = resolve(name)
    if hit:
        resolved.append((name, hit))
    elif k in excused:
        waiting.append(name)
    else:
        bad.append(f"the contract names '{name}' as an anchor, and no registry entry resolves it. "
                   f"Write the entry with its four artifacts, or add the name to {BASELINE} "
                   f"and let a reviewer see the debt")

for x in listed:
    if resolve(x):
        bad.append(f"{BASELINE} still lists '{x}', which now has a registry entry. "
                   f"Delete the line in the commit that adds the entry")
    elif key(x) not in {key(n) for n in named}:
        bad.append(f"{BASELINE} lists '{x}', which the contract does not name. "
                   f"A name nobody cites is not a debt; delete the line")

print(f"anchors named in the contract: {len(named)}")
print(f"  resolved to the registry:    {len(resolved)}")
print(f"  awaiting an entry:           {len(waiting)} — {', '.join(waiting) or 'none'}")


# Needs no baseline: a line repeated here is a line gained whatever main holds. It sits
# outside grew() because every arm that cannot compare returns early, and this check
# would have gone with them.
dupes = len(listed) - len({key(x) for x in listed})
if dupes:
    bad.append(f"{BASELINE} lists {dupes} name(s) twice. The file may not gain a line, and a "
               f"repeated line is a gained line that set comparison would not see")


def grew():
    """Return (gained, note, compared). compared False means the arm did not run."""
    def git(*a):
        return subprocess.run(['git', '-C', str(root), *a], capture_output=True, text=True, timeout=15)
    try:
        if git('rev-parse', '--verify', '--quiet', BASELINE_REF).returncode != 0:
            return set(), f'{BASELINE_REF} is absent', False
        prev = git('show', f'{BASELINE_REF}:{BASELINE}')
        if prev.returncode != 0:
            # The bootstrap case, and the only one that legitimately passes: the file
            # does not exist on the baseline yet, so there is nothing it can have gained.
            return set(), f'not compared — {BASELINE} does not exist on {BASELINE_REF} yet', True
    except (OSError, subprocess.SubprocessError) as e:
        return set(), f'git did not run ({e})', False
    prev_lines = [l.strip() for l in prev.stdout.splitlines()
                  if l.strip() and not l.strip().startswith('#')]
    # Folded on both sides, so an editorial respelling of a name the contract also
    # respelled is not read as a new debt.
    was = {key(x) for x in prev_lines}
    now = {key(x) for x in listed}
    gained = {x for x in listed if key(x) not in was}
    return gained, (f'compared against {BASELINE_REF} — {len(prev_lines)} there, '
                    f'{len(listed)} here, {len(was - now)} registered since'), True


added, how, compared = grew()
print(f"  monotonicity:                {how}")
# Fail rather than pass on one arm. An absent ref or a git that would not run means the
# gate could not answer, and the documents that call this file enforced say it compares
# against main -- a silent skip makes them false and leaves AGENTIC_BASELINE_REF an
# escape hatch that costs one line of stdout.
if not compared:
    bad.append(f"the monotonicity comparison did not run: {how}. This gate claims "
               f"{BASELINE} may not gain a line against {BASELINE_REF}, so it fails rather "
               f"than reporting a check it did not make. Fetch {BASELINE_REF}, or point "
               f"AGENTIC_BASELINE_REF at a ref that exists")
for x in sorted(added):
    bad.append(f"{BASELINE} gained '{x}' since {BASELINE_REF}. The list only shrinks: a name "
               f"leaves it by earning a registry entry, and nothing puts one back. If you "
               f"renamed this anchor in the contract rather than adding a debt, rename it on "
               f"{BASELINE_REF} in the same change, because this gate reads the two together")

if bad:
    print()
    for b in bad:
        print(f"FAIL {b}")
    sys.exit(1)
print("\nEvery anchor the contract names either resolves to the registry or is a recorded debt.")
PY
