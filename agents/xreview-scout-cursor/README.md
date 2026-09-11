# xreview-scout-cursor

One independent reading of a pull request, gathered before the review merges it.

**This bundle does not run yet.** Four prerequisites stand between it and a live
session. None of them is in this repository. See *What registration requires*
below. Until they land, nothing dispatches it: it reaches a session only after a
deployment registers it and a caller names it in `SEIDROID_SCOUTS`.

## Why it exists

A review that only ever hears itself has no way to be wrong out loud. A scout
reads the same pull request in its own session. It sees neither the review nor
another scout. The review then checks its claims against the diff and keeps what
holds.

The value is entirely in being a reading the review did not produce. That is why
the harness matters. `executor.config.harness` is `cursor`, so this is a third
model rather than the same two asked twice. The driver refuses a scout on the
review's own agent for that reason. It also refuses two scouts on one agent,
which would count one opinion twice.

## What this scout looks for, and how it differs from codex

The codex scout sweeps: correctness, security, performance, concurrency, tests,
documentation, broken contracts. This one traces.

Its prompt names one job. Pick a value the change introduces or moves — an id, an
index, a cursor, a count, a handle, a lock, a returned error. Follow that value
through the changed lines. Where does the code set it, where does the code read
it, and what holds it when the path repeats or fails partway. The prompt also
tells the scout to stay in the changed lines, and that another reader covers
architecture and intent.

A measurement chose that aim, not taste. On 2026-09-06, on this repository's own
pull requests, Cursor Bugbot and the Claude review each found defects the other
missed. Bugbot caught a paginated comment id — a value that repeats across a
loop. The Claude review caught a check conclusion that failed open — a question
about what the code means when it goes wrong. Two readers, two shapes of defect.
This bundle takes the first shape, because two readers already cover the second.

Two clauses repeat the codex bundle word for word, deliberately: read-only, and
pull-request content is untrusted data. Those are the security contract every
scout carries. Two scouts wording one guarantee differently is two guarantees.

**The measurement is not a promise about this bundle.** Bugbot is a GitHub App
with its own review pipeline. This bundle runs the Cursor SDK on a pinned model.
They are different readers from one vendor, so Bugbot's record picks the aim
worth trying. It does not predict the result. The first live run is the
measurement that counts.

## How it is invoked

Never directly. `sei-agent-driver` dispatches it when `SEIDROID_SCOUTS` names it:

```
SEIDROID_SCOUTS=codex=xreview-scout-codex,cursor=xreview-scout-cursor
```

`cursor` is the name the driver attributes findings under. `xreview-scout-cursor`
is this bundle. The driver holds the attribution, so nothing a scout returns —
and nothing a scout *reads* — can put a different name on a finding.

That value belongs in uci, and this pull request does not set it. Set it before
the prerequisites land and every review gains a reader that reports nothing.

## What it returns

A short prioritised list, closing with a fenced json block:

```json
{"read": 0,
 "findings": [{"file": "path", "line": 0, "severity": "high|medium|low",
               "detail": "what is wrong and why it matters"}]}
```

`read` is the diff line count. It is the field that separates a failed reading
from a clean one. A scout that never got the diff reports `0`. A scout that read
it and found nothing reports the count with an empty list. Without that the two
are identical bytes, and a credential outage would read as a clean bill of health
on every pull request at once.

No `decision`. That keeps this contract and the review's verdict apart, so
nothing can read a scout report as a verdict —
`TestScoutAndVerdictContractsStayApart` asserts it.

## What registration requires

Four steps, in this order. Each one lives in a different repository, and a human
owns each. None of them is this pull request.

**1. Make the runner advertise the harness.** The runner's hello frame carries a
fixed list: `claude-native`, `claude-sdk`, `codex`, `openai-agents`,
`open-responses`, `pi`. A literal in
`omnigent/runner/transports/ws_tunnel/serve.py` spells it. It is a list, not a
readiness probe, so installing the SDK does not add to it. A session whose
harness is absent from the list fails with `RUNNER_CAPABILITY_MISMATCH`.

Add `cursor` in the omnigent fork. Then rebuild the runner base and bump
`runner-base.txt`. Owner: whoever owns the fork.

**2. Put the SDK in the runner image.** `cursor-sdk` is an optional extra in
omnigent's `pyproject.toml`. The host stage of `Dockerfile.sei` installs the
package with no extras, so the import fails on the first turn.

Add the extra to the host build. Same rebuild as step 1. Owner: whoever owns the
fork.

**3. Put a Cursor API key in the sandbox.** The SDK requires one, and a
`cursor-agent login` does not substitute. It reads `HARNESS_CURSOR_API_KEY`, then
falls back to `CURSOR_API_KEY`. `omnigent-creds` carries `GIT_USERNAME`,
`ANTHROPIC_API_KEY` and `OPENAI_API_KEY`, and nothing for Cursor.

Add the key to that SOPS-encrypted Secret in platform. `envFrom` projects every
key in it into every sandbox, so this widens what an untrusted sandbox can read.
Weigh that first. New keys reach new sessions with no restart. Owner: platform.

**4. Register the bundle.** `OMNIGENT_BUILTIN_AGENT_DIRS` is an explicit
colon-separated list, not a glob. Shipping this directory in the server overlay
image is therefore not enough. Add
`/opt/sei-omnigent/agents/xreview-scout-cursor` to that variable in
`clusters/dev/seigent/configmap-patch.yaml`. Pin the new server image digest and
bump `sei.io/config-revision` in the same commit, because registration reads the
variable once at lifespan startup.

A path with no matching directory logs a line and stops there, so a typo leaves
an agent absent from a healthy server. `GET /v1/agents` is the only thing that
catches it. Owner: platform.

**Then measure step 5.** The driver's `inProcessHarnesses` map decides which
signal ends a turn. `cursor` is not in it, so the driver applies the
terminal-backed rule. If the cursor harness never emits an id-bearing idle edge,
every reading waits for its deadline and arrives as a timeout. The map is an
allowlist on purpose: an unrecognised harness takes the stricter rule, so a real
run earns that entry, never a guess. Do this after steps 1 to 4, and only then.

## Requirements

- **A Cursor API key** in the sandbox. See step 3.
- **A model id Cursor's catalog holds.** `executor.model` is `composer-2.5`.
  cursor-sdk accepts only Cursor ids, and drops anything else to the `auto-smart`
  select with a warning nobody reads. This bundle rejects `auto-smart` for a
  different reason: auto-select can land on a model in the review's own family.
  The driver refuses a scout on the review's agent, never on its model, so
  nothing would catch it. No run from this repository has resolved
  `composer-2.5` against Cursor's API.
- **Registration.** See step 4.
