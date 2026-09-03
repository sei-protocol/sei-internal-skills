# xreview scripts

## `skill-package-checks.sh`

Runs the deterministic subset of `references/skill-package-rubric.md` against one
skill directory. A reviewer on a `skill-package` change runs it first, then reads
the rubric for the rules a script cannot check.

```bash
./skill-package-checks.sh --skill-dir <abs-path> [--output <file>]
```

`--output` takes a **file path**, not a format name — findings go there instead of stdout.
Passing `--output json` writes them to a file literally named `json` and prints nothing, which
reads as "no failures."

**Exit codes:** `0` when the run completes, whatever the findings say — a `block` failure is
reported in the stream, not in the status. `1` on a bad argument or an unreadable skill
directory. Read the stream, never the status, to learn whether a rule failed.

One JSON object per rule on stdout: `id`, `severity`, `title`, `result`,
`evidence`, `catalog_ref`, `source`. `result` is `pass`, `fail`, or **`skipped`** — a check that could not run, which a consumer must
handle: an unrun rule that leaves no trace reads as a rule that passed. A `block` severity that
fails is a finding. An `info` or `warn` is advisory.

The script checks 26 of the rubric's 51 rules. Of the other 25, 24 are `[semantic]` — a reviewer
judges them by reading the skill — and one is `[pressure]`: **P7**, `block` severity, run by the
method in `../references/pressure-testing.md`. Do not judge P7 by reading; that is the skip the
method exists to prevent.

A skill whose own body documents a forbidden pattern will fail the rule that
forbids it. The rubric itself does this. Read the evidence before you file it.
