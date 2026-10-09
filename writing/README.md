# The writing contract

This directory holds the contract, the Vale rules and the gates that hold prose in
this repository to one bar. Before you push a prose change, run `vale sync` once.
Then run `./writing/scripts/lint.sh <file>` on each file you changed.

The rules name public standards and do not restate them. A finding therefore leads
to a clause that somebody else published. `CONTRACT.md` is the file to read first.
It names the anchors, states the rules that have no public prior, and says which
gate checks each rule.

## Layout

| Path | Holds |
|---|---|
| `CONTRACT.md` | the contract: the anchors, and the rules with no public prior |
| `CONTEXT.md` | the short form for an agent, generated from the registry |
| `NOTICE.md` | the licensing boundary for every standard this repository names |
| `styles/AgenticWriting/` | the rules, one file per checkable rule |
| `styles/config/vocabularies/AgenticWriting/` | public technical names and their casing. These travel to every consuming repository |
| `styles/config/vocabularies/Local/` | the names of this repository alone. The install scripts remove this directory from the tree a consumer gets |
| `anchors/` | the registry of public standards, the two debt lists, and the ASD-STE100 page |
| `coverage/` | the topics of each standard that the rules reach, and the topics they miss |
| `scripts/` | the lint script, the rule generator, the gates and the consumer installer |
| `templates/` | the spec template, its upstream baseline, and the files a consumer installs |
| `evals/` | fixtures, golden files per rule and per gate, the consumer test, and the recognition method |
| `docs/` | the architecture, the writing modes, the ADRs, the design documents, and a reference global configuration |
| `specs/` | the specifications of this toolkit |

## Run the checks

Use Vale 3.17.1 or later. CI pins that version in `.github/workflows/writing.yml`
and checks the archive against a recorded sha256. On 3.14.0, a `sequence` rule
whose tokens are all `tag:` entries matches nothing. `STE-NounCluster` then stops
in silence.

```sh
vale sync                                      # fetch write-good, which is not committed
./writing/scripts/lint.sh                      # the prose gate, as CI runs it
./writing/scripts/lint.sh README.md            # the prose gate on the files you name
./writing/scripts/check-generated-rules.sh     # the generated rules match their manifest
./writing/scripts/check-coverage.sh            # the manifest tells the truth
./writing/scripts/check-template-deltas.sh     # the fork keeps its deltas
./writing/scripts/check-contract-anchors.sh    # every anchor named resolves
./writing/scripts/check-artifact-length.sh     # nothing restates a standard
./writing/scripts/check-admission.sh           # an anchor carries its artifacts
./writing/scripts/check-anchor-authorities.sh  # no anchor cites a skill
./writing/scripts/check-consumer-scoping.sh    # all three configs scope the same rules
./writing/scripts/check-verifiers.sh           # every criterion names a verifier
./writing/scripts/check-bundle-prompt.sh       # the sei-spec prompt matches the registry
./writing/evals/run.sh                         # every rule still fires
./writing/evals/rules/run.sh                   # goldens pin line, column, message
./writing/evals/gates/run.sh                   # the gates that have a case are tested
./writing/evals/consumer/run.sh                # another repository can install it
```

`lint.sh` adds `--no-global`. A user-level Vale configuration on a laptop
therefore cannot change the rules that run. `lint.sh` also holds the list of
linted trees and the list of excluded trees, with the reason for each exclusion.
The workflow reads both lists from the script, so CI and a local run cover the
same paths.

## What CI reports

The `writing` workflow runs each check above on each pull request and on each
push to `main`. On a pull request, reviewdog reports a finding only on a line that
the pull request touches. An existing finding never blocks a pull request. On a
push to `main`, the job reports every finding in the tree and fails on any error.

A local run reports on every line, so it can show findings that a pull request
never shows. Run `./writing/scripts/lint.sh` for the current count.

## Generated rules

Eighteen of the thirty-two rules come from `scripts/modes.yaml`. Each generated
rule tracks fenced code blocks, so a heading quoted inside a fence does not
satisfy it. A script rule in Vale cannot import a shared helper, so one
generator writes all eighteen. Edit the manifest, then run
`scripts/generate-mode-rules.py`. `check-generated-rules.sh` fails when the rules
and the manifest disagree.

## Anchors and their gaps

`anchors/registry.yaml` is the single source of truth. Each entry names a public
standard, its steward, its licence, the rules that verify it, and the parts that
no rule can reach. A partial verifier that claims to be complete is worse than no
verifier.

`coverage/` records the same facts per topic. `check-coverage.sh` fails when the
registry and the coverage files disagree. That cross-check catches a rule that
names the wrong standard. An orphan check catches only a rule with no recorded
purpose.

Two files record the debt. `anchors/unregistered.txt` names each anchor that the
contract cites with no registry entry. `anchors/grandfathered.txt` names each
anchor that is exempt from admission. `check-contract-anchors.sh` and
`check-admission.sh` print each list on every run. Each gate compares its file
against `main`, so each list can only shrink.

## Rule tests

A rule that stops firing is a silent failure. Two harnesses catch it.
`evals/run.sh` lints each fixture and asserts which rules fire and which stay
silent. `evals/rules/run.sh` isolates one rule per directory and pins the exact
line, column and message. A fixture directory with a missing piece fails, and
the harness never skips it.

## An anchor is not a skill

An anchor cites a standard that somebody else publishes and maintains. A reader
can then follow the name to a clause outside this repository. A skill in this
repository does not meet that bar. `check-anchor-authorities.sh` holds this rule
across the registry, the anchor pages and the coverage manifest. Other prose can
cite a skill freely.

## Another repository

`writing/templates/` and `writing/scripts/install.sh` connect another repository
to the same checks. The CI of that repository calls the reusable `writing-contract.yml`
workflow here and does not copy it. A rule fix reaches that repository when it
moves its pin.

## Reading a finding

Every rule file opens with the clause it enforces and the standard that the clause
comes from. A rule that cannot express a constraint says so and does not
approximate it.

## Disclaimer

This work is an approximation of ASD-STE100. It is not certified. It is not
affiliated with or endorsed by ASD or STEMG. A passing Vale run is not a
certificate of compliance.

Keep this section in a fork. `writing/NOTICE.md` holds the licensing boundary
for every standard this repository names.
