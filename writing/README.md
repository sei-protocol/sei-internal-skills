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
| `scripts/` | the generator for the section rules, and the gate that holds it |

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

## Reading a finding

Every rule file opens with the clause it enforces and the standard that clause
comes from. A rule that cannot express a constraint says so rather than
approximating it.
