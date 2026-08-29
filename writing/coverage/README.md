# Coverage

What each anchor and each local contract actually checks, topic by topic.

The registry says `coverage: partial`. This directory says *partial of what*.
Hand it to a teammate who asks "does this catch X?". It turns the
single-source-of-truth claim into something a reader can check.

## Shape

One file per anchor, plus `contracts.yml` for the rules that implement this
repository's own conventions rather than a public standard.

```yaml
anchor: asd-ste100          # or: contract: <name>
source: "..."               # the edition or document the topics come from
total_topics: 53            # the standard's own count, when it states one
enumerated: partial         # whether the topic list below is complete
topics:
  a-topic-we-check: [RuleName, OtherRule]
  a-topic-we-do-not: false
```

A topic maps to the rules that check it, or to `false`. `false` is a real
answer, and most standards collect many of them.

## Why the value is a rule name, not `true`

`vale-cli/Google` writes `true` and names the rule in a comment. A comment is
not data. A renamed rule leaves the manifest claiming coverage it no longer has,
which is the drift their own test exists to catch. Naming the rule as the value
makes the claim machine-checkable without parsing comments.

## What the test enforces

`scripts/check-coverage.sh` fails when:

1. A named rule does not exist as a file.
2. A rule file appears in no coverage file. An orphan rule is a rule whose
   purpose nobody wrote down.
3. More than one topic in more than one file claims the same rule.
4. A value is neither `false` nor a non-empty list of strings.

The second check is the one with history. `arc42` once recorded `partial` and
named two rules that check this repository's design contract, not arc42. The
rules that check something else delivered that coverage. That is now a build
failure rather than a reading comprehension exercise.

## What the test does not catch yet

A reviewer looked at each and deferred it, so the gap stands written down rather
than assumed away:

- **Mis-filing.** A rule filed under a wrong-but-existing topic passes every
  invariant. Confirmed by moving `RFC2119-Keywords` under Diátaxis: the manifest
  then asserted that an uppercase-keyword regex checks mode purity, and the run
  was green. The semantic question is human; the attribution question is now
  covered by the registry cross-check.
- **A duplicate topic key.** PyYAML keeps the last one silently, so a recorded
  gap can vanish and the count still passes.
- **A coverage file for an anchor that does not exist**, an anchor with no
  coverage file, and two files claiming the same anchor. The last also
  overwrites a row in the report, so the human-readable output can be wrong
  while the machine check passes.
- **A file that parses to a list** fails by traceback rather than through the
  `FAIL` channel.

## On copyright

Topic names are ours. ASD-STE100 and the EARS papers are not reproduced here,
and no rule file reproduces one. `total_topics` records a count the standard
states about itself. See `NOTICE.md`.
