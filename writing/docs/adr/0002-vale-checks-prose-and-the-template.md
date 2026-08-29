# 2. Vale checks prose and the template, and nothing that needs an open set

## Status

Accepted, 2026-08-24. Refines ADR 0001, which declared the anchor-linter seam but did not
bound the linter's side of it.

## Context

ADR 0001 paired each semantic anchor with a Vale rule and recorded how much of the standard
that rule covers. What it never settled is where the linter's responsibility ends, and a
specification cycle in sei-load ran into that edge in August 2026.

That cycle carried a contract registry from a 23-requirement specification through to a
reviewed pull request, and it produced four failures that every gate passed. Two of the four
are relational, meaning a rule that caught them would have to compare one part of the
document against another. In the first, a verification table cited a requirement ID the
document never declared anywhere. In the second, a requirement declared its ID and then never
appeared in verification at all. Both failures look like linter work, so writing the two
rules was the obvious next step.

Both rules turned out to be writable, and writing either one is a mistake. The reason lives
in the mechanics rather than in taste.

A rule relating declared IDs to referenced IDs has to use `conditional`, the one Vale rule
type that compares two patterns, and `conditional` compares `first`'s full match against
`second`'s capture group rather than against `second`'s full match. That rule also has to run
under `scope: raw`, and the choice of scope is not cosmetic. Three fixtures that catch a real
missing-verification defect under `raw` go silent under `text`, and a rule that always passes
is worse than no rule, because it gets trusted.

Running under `raw` then carries its own cost, because `raw` reads the whole file including
fenced code blocks, so the rule flags a sample log that happens to contain `REQ-404`. The
usual escape hatch fails as well, since the inline `<!-- vale Rule = NO -->` comment has no
effect on a raw rule, which leaves editing the rule file as the only way out. Checking that
IDs are unique needs a Tengo script instead, and that puts a program inside a YAML file with
no test harness and no debugger.

The FROZEN rule fails for a different reason, and the difference matters because open sets
have nothing to do with it. A correct version has to sweep an arbitrary section body looking
for an approver, and Vale runs on `regexp2`, which backtracks where RE2 would refuse. That
sweep crashes with `maximum backtracking stack size exceeded` once a section reaches roughly
400 lines. The version that survives uses a bounded window, and it passes an approver sitting
in an unrelated section.

My first attempt at this boundary used arity, one location for Vale and two or more for a
test. A fixture disproved it in about a minute, and the disproof is worth keeping because the
boundary sounded right. A rule requiring `## Problem` to appear before `## Impact` relates two
locations, which arity would forbid, and Vale enforces it correctly using a variable-length
lookbehind. That rule ran over a 2,523-line document in 130 ms without crashing, because its
pattern anchors to two known literals instead of sweeping an unbounded body. Arity would
therefore have banned a rule the template genuinely wants.

The research literature arrives at the same narrow scope from a different direction. RETA, the
one rigorous EARS checker, reports that its approach cannot detect semantic mismatches, and
AQUSA reached 72.2% precision overall against 42.3% on well-formedness before its authors
deliberately held the tool to what they called the clerical part of requirements engineering.

## Decision

Vale validates prose and enforces the template, and it checks nothing that needs an open
set. The discriminator behind that split is a single question. Can the rule state the thing
it looks for?

- **Prose.** Approved words, active voice, sentence length, noun clusters, RFC 2119
  casing, and the unbounded terms ISO/IEC/IEEE 29148:2018 clause 5.2.7 names. A closed
  vocabulary, known to whoever writes the rule.
- **Template.** Section presence, and section order. A closed set of headings, known to
  whoever writes the rule. The 16 heading rules already shipping stay, and an order rule
  qualifies on the same grounds.
- **Neither.** Anything whose members are unknown until someone writes the document:
  requirement IDs, cross-references, coverage of one set by another, uniqueness within a
  set. No Vale rule. A test, or nothing.

Two rules follow from the failure modes above rather than from the discriminator:

- A pattern MUST NOT scan an unbounded body. Bound every quantifier, or do not write the
  rule.
- A template rule states presence or order only. It MUST NOT claim the section says
  anything, and its comment says so, as the shipping rules already do.

## Consequences

Positive:

- The next person answers the question without judgement: can you write the thing you are
  checking for into the rule? Headings yes, IDs no.
- Every rule the package ships keeps working. The decision bounds growth; it removes
  nothing.
- The style package stays small, and it stays honest about what it does not check.
- Rules stop being the reason to reshape an artifact. A specification adopts a `REQ-nnn`
  convention because tickets link IDs, not because a linter needs a pattern to match.

Negative:

- Two of the four failures from the cycle get no automated gate. They need a reader, and
  the contract must say so rather than implying coverage.
- `verifier.coverage: none` becomes more common in the registry. That field carries more
  weight now, and an unstated `none` reads as an oversight.
- A template rule enforces a house shape, not a public standard. It cannot cite a clause
  the way a prose rule cites 29148 §5.2.7, so its message states the consequence instead.
- Order rules are newly permitted and none exist yet. Someone will write one badly before
  someone writes one well.

## Alternatives considered

**Put the boundary at arity: one location for Vale, two for a test.** Rejected on
evidence. A heading-order rule relates two locations, works, and is fast. That boundary
would have banned a rule the template genuinely wants.

**Write the relational rules anyway, and accept the workarounds.** Rejected. The
`conditional` rule that misses under `scope: text` fails silently, and a rule that always
passes is worse than no rule, because it gets trusted.

**Drop Vale and rely on anchors.** Rejected, and ADR 0001 already gives the reason. An
anchor shapes what a model writes and produces no evidence about what it wrote. Dropping
the linter leaves a model's own claim as the only evidence, which is the failure this
cycle demonstrated three times.

**Replace Vale with a parser for everything.** Rejected for now. A parser handles open sets
and would also handle prose, but the prose rules work today and rewriting them buys
nothing. Revisit if a repository needs the open-set checks enough to build the parser
anyway.
