# xreview-scout-codex

One independent reading of a pull request, gathered before the review merges it.

## Why it exists

A review that only ever hears itself has no way to be wrong out loud. A scout
reads the same pull request in its own session, seeing neither the review nor
another scout, and the review then verifies its claims against the diff and keeps
what holds.

The value is entirely in being a reading the review did not produce. That is why
the harness matters: `executor.config.harness` is `codex-native`, so this is a
different model rather than the same one asked twice. The driver refuses a scout
configured on the review's own agent for exactly that reason, and refuses two
scouts sharing one agent, which would count one opinion twice.

## How it is invoked

Never directly. `sei-agent-driver` dispatches it when `XREVIEW_SCOUTS` names it:

```
XREVIEW_SCOUTS=codex=xreview-scout-codex
```

`codex` is the name findings are attributed under; `xreview-scout-codex` is this
bundle. The driver holds the attribution, so nothing a scout returns — and
nothing a scout *reads* — can put a different name on a finding.

## What it returns

A short prioritised list, closing with a fenced json block:

```json
{"read": 0,
 "findings": [{"file": "path", "line": 0, "severity": "high|medium|low",
               "detail": "what is wrong and why it matters"}]}
```

`read` is the diff line count, and it is the field that makes a failed reading
distinguishable from a clean one. A scout that never got the diff reports `0`;
a scout that read it and found nothing reports the count with an empty list.
Without that the two are identical bytes, and a credential outage would read as a
clean bill of health on every pull request at once.

No `decision`. That keeps this contract and the review's verdict apart, so a
scout report can never be parsed as a verdict — asserted by
`TestScoutAndVerdictContractsStayApart`.

## Requirements

- **`OPENAI_API_KEY`** in the sandbox. It rides `omnigent-creds`, projected into
  every runner Pod via `envFrom`. Absent, the scout fails and the review is shown
  a note saying so rather than an empty reading.
- **Registration.** `OMNIGENT_BUILTIN_AGENT_DIRS` is an explicit colon-separated
  list, not a glob, so shipping this directory in the server overlay image is not
  enough — the variable must name it, and the deployment's `sei.io/config-revision`
  must bump, because registration reads that variable once at lifespan startup.
