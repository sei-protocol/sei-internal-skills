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
vale --no-global writing   # what CI checks today
```

`--no-global` matters. Vale merges a user-level configuration with this one, so a
laptop with the toolkit installed sees different rules than a runner does.

## Reading a finding

Every rule file opens with the clause it enforces and the standard that clause
comes from. A rule that cannot express a constraint says so rather than
approximating it.
