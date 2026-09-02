# The writing contract

Vale rules that hold prose in this repository to one bar. The rules name public
standards rather than restating them, so a reader can follow a finding to a clause
somebody else published.

This directory is arriving as a stacked series from its own repository. What is
here today:

| Path | Holds |
|---|---|
| `styles/AgenticWriting/` | the rules, one file per checkable rule |
| `styles/config/vocabularies/` | terms this repository accepts, and their casing |
| `scripts/` | the generator for the section rules, and the gates |
| `anchors/` | the registry of public standards, and one page per anchor so far |
| `coverage/` | which topics of each standard the rules reach, and which they miss |
| `CONTRACT.md` | the contract itself: the anchors, and the rules with no public prior |
| `templates/` | the spec template, and the upstream baseline it forked from |
| `evals/` | fixtures, per-rule golden files, and the recognition method |
| `specs/` | what this toolkit is for, and the one verifier it still lacks |

The prose lint skips the trees that hold non-conforming prose on purpose. A
fixture is the clearest case: being non-conforming is what makes it a test.
`writing/scripts/lint.sh` names each tree and the reason. The contract governs
prose elsewhere under `evals/` like any other.

What arrives next, in order:

1. the gates that hold the registry and the contract to their own claims
2. the four test harnesses
3. the consumer install path
4. the specifications that describe the whole thing

## Running it

Vale 3.17.1 or later. CI pins that version in `.github/workflows/writing.yml`
and checks the archive against a recorded sha256. The floor is not a preference:
on 3.14.0 a `sequence` rule whose tokens are all `tag:` entries matches nothing,
which disables `STE-NounCluster` in silence.

A local run reports on every line. CI reports only on lines the pull request
touched, so a local run can show findings CI never will.

```sh
vale sync                  # fetch write-good, which is not committed
./writing/scripts/lint.sh                    # the prose gate, as CI runs it
./writing/scripts/check-generated-rules.sh   # the rules match their manifest
./writing/scripts/check-coverage.sh          # the manifest tells the truth
./writing/scripts/check-template-deltas.sh   # the fork keeps its deltas
./writing/scripts/check-contract-anchors.sh  # every anchor named resolves
./writing/scripts/check-artifact-length.sh   # nothing restates a standard
./writing/evals/run.sh                       # every rule still fires
./writing/evals/rules/run.sh                 # goldens pin line, column, message
./writing/scripts/check-admission.sh         # an anchor carries its artifacts
./writing/evals/gates/run.sh                 # the gates that have a case are tested
./writing/scripts/check-anchor-authorities.sh  # no anchor cites a skill
./writing/scripts/check-consumer-scoping.sh  # all three configs scope the same rules
./writing/scripts/check-verifiers.sh         # every criterion names a verifier
./writing/evals/consumer/run.sh              # another repository can install it
```

That second command is what CI runs. `--no-global` matters: Vale merges a
user-level configuration with this one, so a laptop with the toolkit installed
sees different rules than a runner does.

`writing/scripts/lint.sh` holds the exclusions and a reason for each, and both CI
and a person run that script. This paragraph used to restate the list and had
drifted: it said two paths beside a command naming five. Vale takes a single glob
expression and keeps the last. They ride in one brace expression, which is the
other reason the list lives in one place.

## Generated rules

Eighteen of the thirty-two rules come from `scripts/modes.yaml`. Each has to know
where a fenced code block starts and ends. Raw text does not distinguish a real
heading from one quoted inside a fence. A document that merely showed the
required format used to satisfy the check.

Fence tracking needs a script rule. The scripting language cannot import a shared
helper, and eighteen hand-copied loops drift. Edit the manifest and run the
generator. `check-generated-rules.sh` fails if the two disagree.

## Anchors, and what they do not cover

`anchors/registry.yaml` is the single source of truth. Each entry names a public
standard, its steward, its licence, the rules that verify it, and the parts of it
no rule can reach. That last list is the point: a partial verifier that claims to
be complete is worse than no verifier.

`coverage/` says the same thing a second way, per topic. `check-coverage.sh`
fails when the two disagree. They assert it twice on purpose. An orphan check
catches a rule with no recorded purpose. Only the cross-check catches a rule
credited to the wrong standard, which is how one anchor came to claim rules that
check something else.

## The contract

`CONTRACT.md` is the file to read first. It names the anchors, states the rules
that have no public prior, and says which gate checks what. Everything else in
this directory serves it.

## The debt has a home

`anchors/unregistered.txt` names the anchors the contract cites that have no
registry entry yet. The gate prints the count and the names on every run. This
file repeats neither, because a number here goes stale the moment an anchor earns
an entry. The gate fails on a name in neither place, and on a line that has since
earned an entry. It compares the file against `main`, so the list can only shrink.

A stated gap beats a silent one. Nothing checked this direction before, because
the coverage gate reads the registry and never the contract.

## Testing a rule

A rule that stops firing is a silent failure. Silent failure is the reason a
prompt alone earns no trust. Two harnesses catch it.

`evals/run.sh` lints each fixture and asserts the rules that must fire did.
`evals/rules/run.sh` isolates one rule per directory and pins the exact line,
column and message, so a rule that starts reporting the wrong span fails.

A fixture directory missing a piece fails rather than skipping. The harness once
passed over any directory with no input, so a rule still reported `ok` after
somebody deleted its fixture.

## An anchor is not a skill

An anchor earns its place by being a standard somebody else published and
maintains. A reader can then follow the name to a clause this repository does not
control. A skill here is not that. Citing one as an anchor's authority makes the
catalogue circular: the rule is right because our skill says so.

`check-anchor-authorities.sh` holds that line across the registry, the anchor
pages and the coverage manifest. Prose elsewhere may reference a skill freely.

## Another repository can use this

`templates/` and `scripts/install.sh` wire a different repository into the same
checks. Its CI calls `writing-contract.yml` here rather than copying it, so a
rule fix reaches it when it raises the pin.

## Reading a finding

Every rule file opens with the clause it enforces and the standard that clause
comes from. A rule that cannot express a constraint says so rather than
approximating it.

## Disclaimer

This work is an approximation of ASD-STE100. It is not certified. It is not
affiliated with or endorsed by ASD or STEMG. A passing Vale run is not a
certificate of compliance.

Keep this section in a fork. `writing/NOTICE.md` holds the licensing boundary
for every standard this repository names.
