# xreview scripts

## `skill-package-checks.sh`

Runs the deterministic subset of `references/skill-package-rubric.md` against one
skill directory. A reviewer on a `skill-package` change runs it first, then reads
the rubric for the rules a script cannot check.

```bash
./skill-package-checks.sh --skill-dir <abs-path> [--output json]
```

One JSON object per rule on stdout: `id`, `severity`, `title`, `result`,
`evidence`, `catalog_ref`, `source`. A `block` severity that fails is a finding.
An `info` or `warn` is advisory.

The script checks 26 of the rubric's 51 rules. Of the other 25, 24 are `[semantic]` — a reviewer
judges them by reading the skill — and one is `[pressure]`: **P7**, `block` severity, run by the
method in `../references/pressure-testing.md`. Do not judge P7 by reading; that is the skip the
method exists to prevent.

A skill whose own body documents a forbidden pattern will fail the rule that
forbids it. The rubric itself does this. Read the evidence before you file it.
