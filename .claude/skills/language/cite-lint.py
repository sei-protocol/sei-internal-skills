#!/usr/bin/env python3
"""Sonar cite-lint — mechanical verification of /language corpus citations (Design 06 / PLT-493).

Validates that every `cite: <vertical>/<kind>/<target>` prose-string in the corpus resolves to a
declared anchor — the grep-invisible defect class (wrapped spans, renamed/removed anchors) that manual
review catches by hand today.

A `cite:` is a Sonar Resource Name with the `language` domain elided: the full SRN is
`srn:language:<vertical>/<kind>/<target>` (Design 06). This lint validates the domain-elided prose form
in place; nothing is rewritten.

Resolution rules (the three kinds):
  <v>/shape/<t>     -> exemplars/<v>/canonical-shape.md      '## <t>'   (v in hld|lld|one-pager)
  <v>/exemplar/<t>  -> exemplars/<v>/annotated-exemplars.md  '## <t>'   (v in hld|lld|one-pager)
  prfaq/shape/<t>   -> /prfaq (sibling skill) canonical-shape.md '## <t>'   (cross-skill; domain=prfaq)
  prfaq/source/<Q>  -> /prfaq (sibling skill) primary-sources.md  Q-id       (cross-skill)

Anchors are matched GitHub-slug style (so /prfaq's prose headings like
'## Press release — section-by-section' resolve as 'press-release--section-by-section'); the /language
headings are already kebab, on which slugify is idempotent.

Reserved-source check: DEFERRED (Design 06 §4). The spec calls for flagging a cite whose resolved target
is a Reserved license class AND sits adjacent to a quoted span, for human review. No quoted span is
currently adjacent to a reserved cite, so per the YAGNI floor this lint only reports that reserved
classes exist (advisory); the per-cite adjacency flag is implemented when a quoted span actually lands
next to a reserved cite. Proving non-quotation is not mechanical (paraphrase-evasion), so this stays
assisted-review, never an enforcement gate.

A line containing `lint:ignore-cite` opts that line's cite out (for documentation of the cite *syntax*).

Exit 0 = every (non-ignored) cite resolves. Exit 1 = at least one unresolved cite (the failure gate).
"""
from __future__ import annotations
import re
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent           # .../skills/language
REF = HERE / "references"
PRFAQ = HERE.parent / "prfaq" / "references"      # sibling skill (cross-skill prfaq cites)

CITE_RE = re.compile(r"cite:\s*([a-z0-9-]+)/([a-z0-9-]+)/([A-Za-z0-9._-]+)")
H2_RE = re.compile(r"^##\s+(.+?)\s*$", re.M)
QID_RE = re.compile(r"\bQ\d+[a-z]*\b")            # Q1, Q2b, Q3c, ...
LOCAL_VERTICALS = ("hld", "lld", "one-pager")
IGNORE = "lint:ignore-cite"


def read(path: Path) -> str:
    # explicit utf-8: the corpus has em-dashes / accents; a C/POSIX-locale CI runner would otherwise raise.
    return path.read_text(encoding="utf-8")


def slugify(heading: str) -> str:
    """GitHub-style heading anchor: lowercase, drop punctuation (e.g. em-dash), each whitespace char ->
    one hyphen (NOT collapsed — so ' — ' becomes '--'). Idempotent on already-kebab headings.
    The \\w/\\s classes are intentionally Unicode-aware (re default) so 'café' slugs as 'café'."""
    s = heading.strip().lower()
    s = re.sub(r"[^\w\s-]", "", s)   # remove non-word/space/hyphen (— and other punctuation)
    s = re.sub(r"\s", "-", s)         # each whitespace char -> a hyphen (do NOT collapse repeats)
    return s


def h2_anchors(path: Path) -> set[str]:
    return {slugify(h) for h in H2_RE.findall(read(path))} if path.exists() else set()


def qids(path: Path) -> set[str]:
    return set(QID_RE.findall(read(path))) if path.exists() else set()


def build_index() -> dict[tuple[str, str], set[str]]:
    idx: dict[tuple[str, str], set[str]] = {}
    for v in LOCAL_VERTICALS:
        idx[(v, "shape")] = h2_anchors(REF / "exemplars" / v / "canonical-shape.md")
        idx[(v, "exemplar")] = h2_anchors(REF / "exemplars" / v / "annotated-exemplars.md")
    idx[("prfaq", "shape")] = h2_anchors(PRFAQ / "canonical-shape.md")
    idx[("prfaq", "source")] = qids(PRFAQ / "primary-sources.md")
    return idx


def prfaq_file_for(kind: str) -> Path:
    return PRFAQ / ("primary-sources.md" if kind == "source" else "canonical-shape.md")


def reserved_classes() -> int:
    """Count source rows whose license class is reserved (cite-and-link only) — advisory only."""
    sources = REF / "sources.md"
    if not sources.exists():
        return 0
    return sum(1 for ln in read(sources).splitlines()
               if "|" in ln and re.search(r"reserved|cite-and-link only", ln, re.I))


def main() -> int:
    idx = build_index()
    files = sorted(p for p in REF.rglob("*.md") if p.is_file())
    unresolved: list[str] = []
    warnings: list[str] = []
    total = 0

    for f in files:
        rel = f.relative_to(REF)
        text = read(f)
        for m in CITE_RE.finditer(text):
            lineno = text.count("\n", 0, m.start()) + 1
            line = text.splitlines()[lineno - 1] if lineno - 1 < text.count("\n") + 1 else ""
            if IGNORE in line:
                continue
            total += 1
            vertical, kind, target = m.group(1), m.group(2), m.group(3)
            key = (vertical, kind)
            loc = f"{rel}:{lineno}  cite: {vertical}/{kind}/{target}"

            if vertical == "prfaq" and not prfaq_file_for(kind).exists():
                warnings.append(f"{loc}  (cross-skill /prfaq {prfaq_file_for(kind).name} absent — skipped)")
                continue
            if key not in idx:
                unresolved.append(f"{loc}  (unknown vertical/kind '{vertical}/{kind}')")
            elif target not in idx[key]:
                where = (f"Q-id in /prfaq primary-sources" if kind == "source"
                         else f"'## {target}' anchor in {vertical}/{kind}")
                unresolved.append(f"{loc}  (no {where})")

    print(f"cite-lint: {total} citations across {len(files)} corpus files")
    rc = reserved_classes()
    if rc:
        print(f"  advisory: {rc} reserved-source class(es) present — cites resolving to these are "
              f"cite-and-link only (per-cite adjacency flag deferred, Design 06 §4).")
    for w in warnings:
        print(f"  ! {w}")
    if unresolved:
        print(f"\n  FAIL — {len(unresolved)} unresolved citation(s):")
        for u in unresolved:
            print(f"    ✗ {u}")
        return 1
    print("\n  OK — every citation resolves to a declared anchor.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
