# Writing modes

Four document types carry a structure contract, not just a prose contract. Each
one has its own paths and its own required sections. One command checks all
four: `vale <path>`. CI invokes it through `writing/scripts/lint.sh`, which adds
`--no-global` and the exclusion glob.

Ordinary prose — a README, a comment, a reply — gets the prose rules only. A
missing `## Trade-offs` is a defect in a design document and meaningless in a
README. A structure rule therefore turns on for one mode and stays off
everywhere else.

## The four modes

`writing/scripts/modes.yaml` declares the modes and `.vale.ini` scopes them.
This table restates both, so read it against those two files. Every rule named
here runs at `error`.

Each path list holds the brace pattern from `.vale.ini`, fixture paths included.
Step 5 of [Adding a mode](#adding-a-mode) says why a mode names its own fixture
path.

| Mode | Paths in `.vale.ini` | Structure rules |
|---|---|---|
| ADR | `docs/adr/*.md`, `writing/docs/adr/*.md`, `writing/evals/fixtures/adr/*.md` | `ADR-Status`, `ADR-Context`, `ADR-Decision`, `ADR-Consequences` |
| Design | `docs/design/**/*.md`, `designs/**/*.md`, `writing/docs/design/**/*.md`, `writing/evals/fixtures/design/**/*.md` | `Design-Arc42Order`, `Design-NonGoals`, `Design-Alternatives`, `Design-Tradeoffs`, `Design-OpenQuestions` |
| Spec | `specs/**/spec.md`, `writing/specs/**/spec.md`, `writing/evals/fixtures/specs/**/spec.md` | `Spec-Anchors`, `Spec-SuccessCriteria`, `Spec-AcceptanceCriteria`, `Spec-IndependentTest`, `EARS-CriterionShall` |
| Ticket | `tickets/**/*.md`, `writing/evals/fixtures/tickets/**/*.md` | the seven `Ticket-*` rules under [Ticket](#ticket) |

Two of the four drop one prose rule. ADR and Design both set
`STE-SentenceLength-Description` to `NO`. Reasoning travels through
subordination — because, so that, unless — and a 25-word cap forces a split that
drops the connective carrying the causal link. ADR 0002 records the evidence.

### ADR

Four sections: `Status`, `Context`, `Decision`, `Consequences`. Nygard's record
names a fifth, Title, and the H1 already carries it, so no rule checks it.

These four rules accept any heading level. `modes.yaml` gives them
`levels: '#+'`, where every other mode uses `levels: '##'`, so a `### Decision`
nested inside a longer document counts.

### Design

Four sections, chosen because each one goes missing when an author rushes a
document: `Non-goals`, `Alternatives`, `Trade-offs`, `Open questions`. The four
are this repository's own contract. arc42 defines none of them.

`Design-Arc42Order` checks a fifth thing, and it is the only verifier the arc42
anchor has. It reads the arc42 sections a document uses and asserts they run in
template order. arc42 permits an empty section to go missing, so the rule never
demands presence. A house section interleaves freely and the rule ignores it.

`Background`, `Goals`, `Design` and `References` carry no rule. Every document
that anyone wrote at all holds them, so a rule for them costs noise and catches
nothing.

### Spec

| Required | Rule | Why |
|---|---|---|
| `## Semantic Anchors` | `Spec-Anchors` | Methods named once. Without the block, the body restates them. |
| `## Success Criteria` | `Spec-SuccessCriteria` | Nobody can close a spec that states no measurable outcome. |
| `#### Acceptance Criteria` | `Spec-AcceptanceCriteria` | The heading `EARS-CriterionShall` keys on. Without it, that rule checks nothing and reports success. |
| `**Independent Test**` | `Spec-IndependentTest` | A story without one becomes a ticket whose implementer invents the bar. |

`EARS-CriterionShall` then reads every criterion under that heading and demands
a `SHALL`. RFC 2119 keywords are an error in this mode too. A lowercase `must`
in a specification carries no obligation.

The mode has two scopes, not one. Prose rules cover `specs/**/*.md`, which is
every file in a feature directory. The four section rules,
`EARS-CriterionShall` and `RFC2119-Keywords` cover `specs/**/spec.md` alone,
because a plan, a research note and a task list are not specifications.

### Ticket

Seven sections, all mandatory: `Problem`, `Impact`, `Relevant experts`,
`Proposed approach`, `Acceptance criteria`, `Out of scope`, `References`.
An empty one says `None.`, so a reader can tell an empty section from a
forgotten one.

A ticket body is not a file in this repository. `/linear-ticket` stages the
rendered body under `tickets/` and lints it before it files anything. That is how
an agent validates its own output rather than asserting the output is fine.

## A fifth scope: procedures

`docs/procedures/**` is not a structure mode. It carries no required sections. It
changes which **prose** rules apply, because two ASD-STE100 rules govern a
procedure and not description:

| Rule | In a procedure | Elsewhere |
|---|---|---|
| `STE-SentenceLength-Procedure` | 20 words | off — the 25-word rule applies |
| `STE-Gerund-Instruction` | a step starts with the imperative | off |

The gerund rule was global until it fired on four documents in a row, every
time on a descriptive list. The design mode requires a `## Non-goals` section. A
writer naturally states a non-goal as `Replacing X`, so a global gerund rule and
the design mode contradicted each other. Its coverage now lives in
`writing/evals/fixtures/procedures/`. `writing/evals/fixtures/ste-violations.md`
keeps a gerund-led list item as the negative control: the rule must stay silent
there.

## Publishing a mode as an artifact

A specification or a design document published to claude.ai renders through the
platform's own Markdown stylesheet. That stylesheet sets a section heading below
the contrast of the body text it heads.
`writing/styles/artifact/heading-hierarchy.css` holds the canonical correction,
and that file states why each rule exists.

`writing/scripts/build-spec-artifact.sh` performs the step. It reads the
specification from git, prepends the canonical style, and writes a file the
caller publishes:

```sh
writing/scripts/build-spec-artifact.sh --repo ~/sei-load \
  --ref brandon2/spec-contract-deployment-registry \
  --out /tmp/artifacts contract-deployment-registry
```

Use the script rather than assembling the file by hand. It reads from git, so an
uncommitted edit cannot reach a published artifact. It reads the style from the
canonical file, so an artifact cannot drift from it. Prepending at publish time
also keeps the presentation out of the specification source, and one file then
holds the style for every artifact.

Two things follow from prepending rather than committing. The published artifact
stops being a byte-for-byte render of its source file. The export therefore
stays mechanical: read the file from git, prepend the block, publish. A reader
who edits the published page edits a copy, and that change never reaches the
repository.

The block changes no heading level, so every rule in the tables above still
matches. ADR 0002 explains why that constraint holds.

## What the modes do not check

A structure rule reports a missing heading. It cannot report an empty section, a
section that says nothing, or a section in the wrong order. Those limits sit per
anchor in `writing/anchors/registry.yaml` under `not_checkable`.

Three more gaps worth naming:

- A spec's per-story pairing is unchecked. The gate confirms that
  `**Independent Test**` appears at least once. It cannot confirm that every
  user story has one, because a Vale rule cannot compare two token counts.
- A section present but stated as `None.` passes. That is deliberate: `None.` is
  a real answer, and distinguishing it from evasion is judgement.
- The ADR mode has no fixtures. `.vale.ini` covers
  `writing/evals/fixtures/adr/*.md` and no such directory exists, so step 3
  below is unmet for the four `ADR-*` rules.

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
5. The mode's glob must also cover the fixture path. Vale anchors a section glob
   at the repository root, so `tickets/**/*.md` does not match
   `writing/evals/fixtures/tickets/`. Name both paths in one brace pattern.
