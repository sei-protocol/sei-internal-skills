# Writing modes

Three document types carry a structure contract, not just a prose contract. Each
one has a path, a set of required sections, and one command that checks it.

Ordinary prose — a README, a comment, a reply — gets the prose rules only. A
missing `## Trade-offs` is a defect in a design document and meaningless in a
README, so a structure rule turns on for one mode and stays off everywhere else.

## The three modes

| Mode | Path | Command |
|---|---|---|
| Spec | `writing/specs/**/*.md` | `vale specs/<NNN>-<slug>/` |
| Ticket | `tickets/**/*.md` | `vale tickets/<id>.md` |
| Design | `docs/design/**`, `designs/**` | `vale designs/<arc>/<doc>.md` |

### Spec

| Required | Rule | Why |
|---|---|---|
| `## Semantic Anchors` | `Spec-Anchors` | Methods named once. Without the block, the body restates them. |
| `## Success Criteria` | `Spec-SuccessCriteria` | A spec with no measurable outcome cannot be closed. |
| `**Independent Test**` | `Spec-IndependentTest` | A story without one becomes a ticket whose implementer invents the bar. |

RFC 2119 keywords are an error in this mode. A lowercase `must` in a
specification carries no obligation.

### Ticket

Seven sections, all mandatory: `Problem`, `Impact`, `Relevant experts`,
`Proposed approach`, `Acceptance criteria`, `Out of scope`, `References`.
An empty one says `None.`, so a reader can tell an empty section from a
forgotten one.

A ticket body is not a file in this repository. `/linear-ticket` stages the
rendered body under `tickets/` and lints it before it files anything. That is how
an agent validates its own output rather than asserting the output is fine.

### Design

Four sections, chosen because they are the ones that go missing when a document
is rushed: `Non-goals`, `Alternatives`, `Trade-offs`, `Open questions`.

`Background`, `Goals`, `Design` and `References` are not checked. They are
present in every document that was written at all, so a rule for them costs
noise and catches nothing.

## A fourth scope: procedures

`docs/procedures/**` is not a structure mode. It carries no required sections. It
changes which **prose** rules apply, because two ASD-STE100 rules govern a
procedure and not description:

| Rule | In a procedure | Elsewhere |
|---|---|---|
| `STE-SentenceLength-Procedure` | 20 words | off — the 25-word rule applies |
| `STE-Gerund-Instruction` | a step starts with the imperative | off |

The gerund rule was global until it fired on four documents in a row, every
time on a descriptive list. The design mode requires a `## Non-goals` section,
and a Non-goals list is naturally written as `Replacing X`, so a global rule put
two rules here in direct contradiction. Its coverage now lives in
`writing/evals/fixtures/procedures/`, and `writing/evals/fixtures/ste-violations.md` keeps a
gerund-led list item as the negative control: the rule must stay silent there.

## Publishing a mode as an artifact

A specification or a design document published to claude.ai renders through the
platform's own Markdown stylesheet. That stylesheet sets a section heading below
the contrast of the body text it heads.
`writing/styles/artifact/heading-hierarchy.css` is the canonical correction, and the file
states why each rule exists.

`writing/scripts/build-spec-artifact.sh` performs the step. It reads the specification
from git, prepends the canonical style, and writes a file the caller publishes:

```sh
scripts/build-spec-artifact.sh --repo ~/sei-load \
  --ref brandon2/spec-contract-deployment-registry \
  --out /tmp/artifacts contract-deployment-registry
```

Use the script rather than assembling the file by hand. It reads from git, so an
uncommitted edit cannot reach a published artifact. It reads the style from the
canonical file, so an artifact cannot drift from it. Prepending at publish time
also keeps the presentation out of the specification source, and one file then
holds the style for every artifact.

Two things follow from prepending rather than committing. The published artifact
stops being a byte-for-byte render of its source file, so the export stays
mechanical: read the file from git, prepend the block, publish. A reader who
edits the published page edits a copy, and that change never reaches the
repository.

The block changes no heading level, so every rule in the tables above still
matches. ADR 0002 explains why that constraint holds.

## What the modes do not check

A structure rule reports a missing heading. It cannot report an empty section, a
section that says nothing, or a section in the wrong order. Those limits are
recorded per anchor in `writing/anchors/registry.yaml` under `not_checkable`.

Two more gaps worth naming:

- A spec's per-story pairing is unchecked. The gate confirms that
  `**Independent Test**` appears at least once. It cannot confirm that every
  user story has one, because a Vale rule cannot compare two token counts.
- A section present but stated as `None.` passes. That is deliberate: `None.` is
  a real answer, and distinguishing it from evasion is judgement.

## Adding a mode

1. Write one rule per required section. An `occurrence` check carries one token
   and one message, so a section cannot share a rule.
2. Set every new rule to `NO` under `[*.md]`, then to `YES` in the mode's own
   section. Vale scopes a rule's on and off state per path, but applies a
   **level** globally, so toggle rather than restate the level.
3. Add two fixtures: one incomplete document that fires the rules, one complete
   document that fires none. The second matters more — a presence rule that
   fires on a complete document is worse than no rule.
4. Name the silent rules in `must_not_include_rules` in the expectation file.
5. The mode's glob must also cover the fixture path. A Vale section glob is
   anchored at the repository root, so `tickets/**/*.md` does not match
   `writing/evals/fixtures/tickets/`. Name both paths in one brace pattern.
