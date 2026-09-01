#!/usr/bin/env bash
# Every success criterion names a verifier that exists, or says plainly that it does not.
#
# THREE OF THE SEVEN CITED SOMETHING THAT DOES NOT RUN. SC-002 named "the registry
# consistency job", which reads the registry and never the contract. SC-003 named "the
# probe suite", which was never written. SC-005 and SC-006 named "a line-count check in
# CI" that no workflow contained. A criterion with a verifier nobody built reads exactly
# like one with a verifier that passes, which is the failure this repository exists to
# stop.
#
# A verifier line takes one of three shapes:
#
#   *Verifier:* `scripts/check-coverage.sh`
#   *Verifier:* judgement — a reviewer attempts it on three findings and reports.
#   *Verifier:* not built — the recognition suite does not exist yet.
#
# The first must name an executable path. The other two must give a reason, because
# "not built" with no explanation is a shrug rather than a statement.
#
# BOTH LAYOUTS COUNT. The first version demanded '**SC-001**' at column zero and an
# unindented verifier, which is the shape specs/001 happens to use. The template
# prescribes the other one:
#
#     - **SC-001**: [Measurable outcome.]
#       *Verifier:* `[the command]`
#
# So the gate rejected the layout its own template teaches, and said the criteria did
# not exist rather than that it could not read them. An agent following the template
# produced exactly that file and the gate called it empty.
#
# Takes an optional root, which is how evals/gates/run.sh points it at a fixture tree.
set -euo pipefail
ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
ROOT="$(cd "$ROOT" && pwd)"
exec python3 - "$ROOT" <<'PY'
import os
import pathlib, re, sys

root = pathlib.Path(sys.argv[1])
specs = sorted(root.glob('specs/*/spec.md'))
if not specs:
    print(f"FAIL no specs under {root}/specs — refusing to report success on an empty set")
    sys.exit(1)

PATHISH = re.compile(r'(?:[\w.-]+/)+[\w.-]*|[\w.-]+\.(?:sh|py|yml|yaml|md|ini|txt)')
# A criterion at the head of a line, or as a list item, with or without a colon.
SC_LINE = re.compile(r'^[ \t]*(?:[-*+][ \t]+|\d+\.[ \t]+)?\*\*(SC-\d+)\*\*')
# A verifier line, indented under its criterion or not.
V_LINE = re.compile(r'^[ \t]*\*Verifier:\*')
# Anything that looks like a criterion id, to tell "none" from "unreadable".
SC_ANY = re.compile(r'\bSC-\d+\b')
COMMANDS = {'vale'}          # a tool the repository already requires on PATH
bad, rows = [], []

for spec in specs:
    lines = spec.read_text().splitlines()
    rel = spec.relative_to(root)
    criteria = [(i, m) for i, l in enumerate(lines) if (m := SC_LINE.match(l))]
    if not criteria:
        loose = sorted({m.group(0) for l in lines for m in [SC_ANY.search(l)] if m})
        if loose:
            bad.append(f"{rel}: names {', '.join(loose[:4])} but in a shape this gate cannot "
                       f"read. Write each as '**SC-001**' at the head of a line or of a list "
                       f"item, with its '*Verifier:*' line under it")
        else:
            bad.append(f"{rel}: no success criteria found. A specification states what would "
                       f"show it worked")
        continue

    for idx, (i, m) in enumerate(criteria):
        sc = m.group(1)
        end = criteria[idx + 1][0] if idx + 1 < len(criteria) else len(lines)
        vlines = [l for l in lines[i:end] if V_LINE.match(l)]
        if len(vlines) != 1:
            bad.append(f"{rel} {sc}: has {len(vlines)} verifier lines, expected exactly one")
            continue

        body = vlines[0].strip()[len('*Verifier:*'):].strip()
        marker = re.match(r'^(not built|judgement)\s+—\s+(.+)$', body)
        if marker:
            if len(marker.group(2)) < 15:
                bad.append(f"{rel} {sc}: '{marker.group(1)}' with no reason. Say what is missing, "
                           f"or who applies the judgement and how")
            rows.append(f"  {sc:<10}{marker.group(1):<12}{marker.group(2)[:56]}")
            continue

        spans = re.findall(r'`([^`]+)`', body)
        if not spans:
            bad.append(f"{rel} {sc}: names '{body}' as its verifier, which is prose, not a command. "
                       f"Name a path in backticks, or mark it 'not built — <reason>' or "
                       f"'judgement — <how>'")
            continue

        # Before a path is required, not inside the loop over paths. PATHISH matches
        # only a string with a slash or a known extension, so a bare command never
        # reaches `targets` -- the allowlist could not fire where it used to sit, and
        # a verifier reading `vale ...` was rejected for naming no path.
        first = spans[0].split()[0] if spans[0].split() else ''
        if first in COMMANDS:
            rows.append(f"  {sc:<10}{'runs':<12}{' '.join(spans)[:56]}")
            continue

        targets = [m for s in spans for m in PATHISH.findall(s)]
        if not targets:
            bad.append(f"{rel} {sc}: the backticks hold '{' '.join(spans)}', which names no path. "
                       f"A verifier is something a reader can run")
            continue
        # Existing is not running. A criterion naming writing/README.md or a bare
        # directory satisfied .exists() and was reported as `runs`, which is the
        # confusion this gate names as its reason for existing: a verifier nobody
        # built reads exactly like one that passes.
        def resolve(tok):
            # Both spellings. Targets resolve against writing/, so `scripts/x.sh`
            # worked and `writing/scripts/x.sh` did not -- and the second is the form
            # an author reaches for, because writing/README.md lists every local
            # command with the ./writing/ prefix. A gate that rejects the documented
            # spelling and says the file "does not exist" is the least useful failure
            # it could give.
            rel_tok = tok.rstrip('/')
            for base in (root, root.parent):
                candidate = base / rel_tok
                if candidate.exists():
                    return candidate
            return None

        missing = False
        for tgt in targets:
            path = resolve(tgt)
            if path is None:
                bad.append(f"{rel} {sc}: names '{tgt}', which does not exist")
                missing = True
            elif tgt == first and not (path.is_file() and os.access(path, os.X_OK)):
                # Only the first token is what the criterion runs. The rest are its
                # arguments, and an input is not an executable -- requiring it of all
                # of them made a verifier that takes a file reject itself, and
                # reported a directory argument in the same words as a phantom.
                bad.append(f"{rel} {sc}: runs '{tgt}', which exists but is not executable. "
                           f"A verifier is something a reader can run")
                missing = True
        # The table is what a reader scans, so it must not show a broken criterion in
        # the same column as a working one.
        rows.append(f"  {sc:<10}{('missing' if missing else 'runs'):<12}{' '.join(spans)[:56]}")

print(f"  {'criterion':<10}{'kind':<12}verifier")
for r in rows:
    print(r)

if bad:
    print()
    for b in bad:
        print(f"FAIL {b}")
    sys.exit(1)
print("\nEvery success criterion names a verifier that runs, or says why none does.")
PY
