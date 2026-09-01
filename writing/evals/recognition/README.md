# Anchor recognition tests

An anchor works only if the model resolves the term. This is the one property that no
amount of prompt engineering can add, and it can change with a model version. Test it.

**This suite is not built.** The method below is the design. Nothing here runs it, no
anchor carries a recorded verdict, and `recognition.verified` is empty for all eight.
Read a claim about anchor strength in this repository as untested until that changes.

## Method

For each anchor in `anchors/registry.yaml`, send the `recognition.question` string to
the model in a clean context, with no other instruction. Ask nothing else.

```
What concepts do you associate with '<anchor name>'?
```

Score the answer on four axes:

| Axis | Question | Failure it catches |
|---|---|---|
| Recognition | Does the model know the term at all? | Confabulation |
| Accuracy | Are the associated concepts correct? | Silent substitution |
| Depth | Does it go past a one-line definition? | A name without the method |
| Specificity | Does it separate the term from a near neighbour? | Transparent substitution |

The four axes are the ones the spec names in FR-008. Score `Accuracy` against the
`recognition.expect` list already in the registry: a passing answer names those
concepts, and names nothing that contradicts them.

**Why an open question rather than multiple choice.** Multiple choice measures
discrimination, not resolution. A model can eliminate three wrong options and still
substitute a neighbouring concept when it writes. Substitution is visible in an open
answer and invisible in a forced choice, and substitution is the failure this test
exists to catch.

## Verdict

| Verdict | Meaning | Action |
|---|---|---|
| `strong` | All four axes pass. | Use the bare anchor name. |
| `partial` | Recognition passes, depth or specificity fails. | Use the anchor plus a one-line normative summary. |
| `absent` | The model does not resolve the term. | Do not use it as an anchor. Inline the full instruction. |

Record the verdict in `registry.yaml` under `recognition.verified`, with the raw
answer beside it. The answer is what makes the verdict re-scorable: a reader who
disagrees can read what the model said rather than take the label.

```yaml
    recognition:
      verified:
        - model: claude-opus-5
          date: 2026-08-19
          verdict: strong
          answer: |
            <the model's reply, verbatim>
```

## Why this matters more than it looks

This test is the difference between an explainable system and a folk remedy. It answers
the question of why the output improved. The model resolves this term to this published
body of work. Here is the question that shows it, and here is the answer, on this model
version and on this date.

Re-run the whole matrix on every model upgrade. Treat a `strong` to `partial` regression
as a breaking change to the writing contract.
