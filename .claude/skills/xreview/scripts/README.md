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

The script checks 26 of the rubric's 51 rules. The other 25 are semantic — a
reviewer judges them. The rubric marks each rule `[static]` or `[semantic]`.

A skill whose own body documents a forbidden pattern will fail the rule that
forbids it. The rubric itself does this. Read the evidence before you file it.
