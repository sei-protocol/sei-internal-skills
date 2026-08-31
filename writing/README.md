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

What arrives next, in order:

1. the generated section rules and the manifest they come from
2. the anchor registry, the anchor pages, and the coverage manifest
3. the contract, and the spec template it governs
4. the gates that hold the registry and the contract to their own claims
5. the four test harnesses
6. the consumer install path
7. the specifications that describe the whole thing

## Running it

```sh
vale sync                  # fetch write-good, which is not committed
vale --no-global writing                     # what CI checks today
./writing/scripts/check-generated-rules.sh   # the rules match their manifest
./writing/scripts/check-coverage.sh          # the manifest tells the truth
```

`--no-global` matters. Vale merges a user-level configuration with this one, so a
laptop with the toolkit installed sees different rules than a runner does.

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

## Reading a finding

Every rule file opens with the clause it enforces and the standard that clause
comes from. A rule that cannot express a constraint says so rather than
approximating it.
