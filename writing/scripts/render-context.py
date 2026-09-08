#!/usr/bin/env python3
"""Render generated consumers from writing/anchors/registry.yaml.

The registry is the single source of truth. writing/CONTEXT.md is generated from it.
Never hand-edit a generated file.

CONTEXT.md is the short form an agent loads: the anchors, named, with nothing else.
CONTRACT.md is the long form a person reads, and a person writes it by hand.

The `bundle-prompt` target renders one section of an Omnigent bundle prompt. A bundle
carries no file the host reads, so its prompt is the only text that reaches the model.
writing/scripts/check-bundle-prompt.sh holds agents/sei-spec/config.yaml to this output.

Usage:
    python3 writing/scripts/render-context.py --target agents > writing/CONTEXT.md
    python3 writing/scripts/render-context.py --target table   # a table for docs/
    python3 writing/scripts/render-context.py --target bundle-prompt

A `style` target used to appear here and argparse never accepted it, so the file it
named was generated once and then drifted with nothing to regenerate it.
"""
import argparse
import pathlib
import sys
import textwrap

try:
    import yaml
except ImportError:
    sys.exit("pip install pyyaml")

ROOT = pathlib.Path(__file__).resolve().parent.parent
REGISTRY = ROOT / "anchors" / "registry.yaml"
BANNER = "<!-- GENERATED FILE. Source: writing/anchors/registry.yaml. Run writing/scripts/render-context.py. -->"


def load():
    with REGISTRY.open() as fh:
        return yaml.safe_load(fh)


INTRO = (
    "You write text that another agent or a non-native reader must parse without a "
    "back-channel. Follow the anchors below. Each anchor names a public standard, so you "
    "can resolve it from the name alone. A more specific instruction from the user or from "
    "the file you edit takes precedence on whatever it addresses."
)

VERIFY = [
    "Run `vale <path>` before you report the work as done. A finding names a rule. A rule "
    "names a clause. If you disagree with a finding, say which rule and why. Do not silence "
    "a rule to make the output pass.",
    "Vale checks part of ASD-STE100, not all of it. `vale` exit code 0 means \"no finding "
    "at or above the gate\", not \"compliant\".",
]

WIDTH = 90

# PROMPT_WIDTH is narrower than WIDTH because the output nests two spaces inside a
# YAML block scalar, and the surrounding prompt wraps at 78 columns.
PROMPT_WIDTH = 76
PROMPT_INDENT = "  "

PROMPT_HEADING = "## Hold every artifact to the writing contract"

PROMPT_INTRO = (
    "A specification is prose, so the writing contract governs it. It governs "
    "`spec.md`, `plan.md`, `tasks.md`, a commit subject, a pull request body, and "
    "what you report in the session. Apply it as you draft, not as a pass at the end."
)

# Lane 1 of AGENTS.md. These are local rules rather than anchors: EARS and RFC 2119
# name the syntax, and neither standard says a requirement carries an ID or that a
# test cites one. An anchor cannot carry a rule its own standard does not state.
PROMPT_NORMATIVE = [
    "Write each requirement in one EARS template: ubiquitous, `WHEN`, `IF..THEN`, "
    "`WHILE`, or `WHERE`.",
    "Write a normative keyword in uppercase: `MUST`, `MUST NOT`, `SHOULD`, `MAY`. "
    "Use it only for a requirement.",
    "Give every requirement an ID. Every test names the ID it covers.",
]

# Quoted from the banned list in AGENTS.md Lane 2. Every entry is backticked, which
# keeps the list inside Vale's code scope: a substitution rule reads bare prose and
# would otherwise report each word this list exists to forbid.
PROMPT_BANNED = [
    "leverage",
    "seamless",
    "robust",
    "delve",
    "deep dive",
    "unlock",
    "elevate",
    "game-changing",
    "in today's landscape",
    "it is important to note",
    "this is not just X, it is Y",
    "comprehensive as a filler word",
]

# The last bullet is the residue: Principle V of writing/CONTRACT.md asks every
# anchor to state what it misses.
PROMPT_VERIFY = [
    "Run `sei-writing-lint <path>` on each artifact a phase produced, before you "
    "report that phase as done. Fix what it reports.",
    "A finding names a rule. A rule names a clause. If you disagree with a finding, "
    "say which rule and why. Never silence a rule to make the output pass.",
    "The gate reads patterns in finished text. It cannot read meaning, so it cannot "
    "tell you that a requirement misses the problem. Exit code 0 means \"no finding "
    "at or above the gate\", not \"correct\".",
]


def agents(reg):
    """The model-context contract.

    Prose lives here because it describes the contract as a whole. Per-anchor text
    lives in the registry under `context`. An anchor with no `context` block is not
    part of the contract, and this omits it.
    """
    listed = sorted(
        (a for a in reg["anchors"] if a.get("context")),
        key=lambda a: a["context"]["order"],
    )

    out = [BANNER, "# Writing contract", "", textwrap.fill(INTRO, WIDTH, break_on_hyphens=False), ""]
    for a in listed:
        c = a["context"]
        out.append(
            textwrap.fill(
                f"**{c['label']}** — {' '.join(c['text'].split())}",
                WIDTH,
                initial_indent="- ",
                subsequent_indent="  ",
                break_on_hyphens=False,
            )
        )
    out += ["", "## Verify before you claim compliance", ""]
    for para in VERIFY:
        out += [textwrap.fill(para, WIDTH, break_on_hyphens=False), ""]
    return "\n".join(out).rstrip("\n")


def bundle_prompt(reg):
    """One section of an Omnigent bundle prompt, indented for a YAML block scalar.

    The anchor bullets come from the same `context` blocks CONTEXT.md renders, so a
    registry edit reaches the bundle and the context file together.
    """
    listed = sorted(
        (a for a in reg["anchors"] if a.get("context")),
        key=lambda a: a["context"]["order"],
    )

    def para(text, bullet=False):
        return textwrap.fill(
            text,
            PROMPT_WIDTH,
            initial_indent=PROMPT_INDENT + ("- " if bullet else ""),
            subsequent_indent=PROMPT_INDENT + ("  " if bullet else ""),
            break_on_hyphens=False,
        )

    out = [PROMPT_INDENT + PROMPT_HEADING, "", para(PROMPT_INTRO), ""]

    out += [PROMPT_INDENT + "### Requirements carry normative language", ""]
    out += [para(rule, bullet=True) for rule in PROMPT_NORMATIVE]
    out += ["", PROMPT_INDENT + "### The anchors", ""]
    for a in listed:
        c = a["context"]
        out.append(para(f"**{c['label']}** — {' '.join(c['text'].split())}", bullet=True))

    out += ["", PROMPT_INDENT + "### Never write these", ""]
    out += [
        para(
            "They read as machine filler: "
            + ", ".join(f"`{w}`" for w in PROMPT_BANNED)
            + "."
        ),
        "",
        PROMPT_INDENT + "### Verify before you report a phase as done",
        "",
    ]
    for text in PROMPT_VERIFY:
        out += [para(text), ""]
    return "\n".join(out).rstrip("\n")


def table(reg):
    rows = ["| Anchor | Standard | Coverage | Rules |", "|---|---|---|---|"]
    for a in reg["anchors"]:
        v = a["verifier"]
        rows.append(
            f"| `{a['id']}` | [{a['name']}]({a['url']}) | {v.get('coverage')} | "
            f"{len(v.get('rules') or [])} |"
        )
    return "\n".join(rows)


def main():
    p = argparse.ArgumentParser()
    p.add_argument(
        "--target", choices=["agents", "table", "bundle-prompt"], default="agents"
    )
    args = p.parse_args()
    reg = load()
    targets = {"agents": agents, "table": table, "bundle-prompt": bundle_prompt}
    print(targets[args.target](reg))


if __name__ == "__main__":
    main()
