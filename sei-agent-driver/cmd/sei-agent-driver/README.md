# xreview

`xreview` drives one sei-droid review of a pull request inside an Omnigent
managed sandbox: it resolves the `sei-droid` agent, creates or adopts **the
session for that pull request**, sends the review prompt,
answers whatever permission prompts the turn parks on (allowing only what an
operator listed — see [Permission policy](#permission-policy)), and reads a
verdict out of the
agent's final message. It is the Go successor to this repository's Python
driver and keeps that driver's contract: the same environment variables and the
same exit codes.

**One session per pull request, not per invocation.** The session is the agent's
memory of having reviewed this pull request, so a review ends by *stopping* the
turn rather than deleting it, and the next dispatch on the same pull
request picks up the same conversation — it can say what changed since, and
whether what it raised last time was addressed. An earlier version keyed the
session on the trigger and deleted it on the way out, which meant every
invocation met the diff with no memory of the previous one.

The session is destroyed when the pull request closes or merges (`--close`). That
is also the only thing that reclaims the sandbox: the Kubernetes launcher reports
`resume_stopped=false`, so a stopped host cannot be woken in place and the host
stays up while the session exists. Nothing else reclaims it: the Kubernetes
launcher sets no lifetime cap and the server runs no sweep, so a close event that
never arrives leaves a pod holding its cpu and memory indefinitely.

It is normally invoked by a workflow rather than run by hand. That caller lives in
the repository being reviewed: `sei-protocol/sandbox`'s `xreview.yml` dispatches
`seidroid-xreview.yml` on a `seidroid xreview` comment, and on a pull request
closing. Every input below is a plain environment variable or CLI flag, so it also
runs the same way in a terminal.

## Usage

```
sei-agent-driver xreview <owner/name> <pr-number> [--out FILE] [--trigger-id ID]
```

- `<owner/name> <pr-number>` — the pull request to review. Both are
  required; anything else is a configuration error (`ExitConfig`), not a
  request sent to the server.
- `--out FILE` — write the verdict text here for a caller to post. **The file
  is written only when the turn produced a real verdict.** Its absence is
  how a caller distinguishes "nothing to post" from "posted"; a failed or
  no-verdict run must never see a stale or placeholder file where a previous
  good review's output used to be. (This CLI never writes it as a
  placeholder — the caller-side rule is simply: don't create one and don't
  upsert on absence.)
- `--close` — delete this pull request's session instead of reviewing, and the
  scouts' sessions with it. The only thing that reclaims a sandbox. Wired in the
  reviewed repository's caller, on `pull_request: [closed]`; nothing in this
  repository invokes it.
- `--trigger-id ID` — identifies this dispatch in the logs, e.g. the GitHub
  comment id that fired the run. **Not** part of the session key any more — the
  session is keyed on the pull request. Defaults to
  `run:$GITHUB_RUN_ID/$GITHUB_RUN_ATTEMPT` (from the environment) and, failing
  that, a deterministic `manual:owner/name#pr` key. Every dispatch for a pull
  request adopts that pull request's one session and drives a fresh review turn
  on the current tree, whatever the trigger id is — nothing is skipped as a
  duplicate. Pass an explicit id so an automated dispatch is identifiable when
  reading back a reused session's history.

Machine-readable output (session id, exit code, teardown flag, and the
decision when there is one) is printed to stdout as JSON on every run;
logs go to stderr as structured (`slog`) JSON.

## Environment variables

### Connection and identity

| Variable | Default | Meaning |
|---|---|---|
| `OMNIGENT_BASE_URL` | `http://127.0.0.1:6767` | The Omnigent server. The default is loopback so a bare local run fails locally rather than reaching for someone's deployment; the review workflow always passes the real one (`https://seigent.dev.platform.sei.io`). It must be https or loopback: anything else is refused twice over, once when the client is built and once when the token mint declines to put the client secret on the wire. The in-cluster ClusterIP Service is not usable for this reason — it is plain http on port 80. |
| `OMNIGENT_ORIGIN` | `omnigent://internal` | Sent as the `Origin` header on every request to satisfy the server's trusted-origin CSRF guard on state-changing POSTs. This process is not a browser and sends no Origin of its own, so it announces this sentinel instead. |
| `SEIDROID_AGENT_ID` | `sei-droid` | The agent **name** to resolve to an id. There is no lookup-by-name route server-side, so the driver pages the agent listing until this name matches. |

The driver also reads `GITHUB_RUN_ID` and `GITHUB_RUN_ATTEMPT` when
`--trigger-id` is not given. These now only label a dispatch in the logs — they
do not affect which session is used, so a re-run reuses the pull request's
session like any other invocation.

### Credential — pick one path

The driver needs exactly one of these two credential shapes. Supplying
neither is a configuration error; supplying half of the machine-client pair
is also a configuration error (a distinct, more specific one), because the
two mistakes have different fixes.

| Variable | Meaning |
|---|---|
| `OMNIGENT_API_TOKEN` | A bearer token minted elsewhere. Never logged. |
| `OMNIGENT_API_TOKEN_FILE` | Path to a file holding that token, re-read on every invocation so a rotated credential is picked up without a redeploy. Takes precedence over `OMNIGENT_API_TOKEN` when both are set. |
| `OMNIGENT_MACHINE_CLIENT_ID` | The confidential client's identifier. Mirrors the server's own `OMNIGENT_MACHINE_CLIENT_ID` — an operator configures one value, not two vocabularies for the two ends of the same client-credentials pair. |
| `OMNIGENT_MACHINE_CLIENT_SECRET` | That client's secret, in plaintext, on this side only. **The server stores just a digest of it**, under `OMNIGENT_MACHINE_CLIENT_SECRET_HASH` — note the `_HASH` that only the server-side name carries; do not cross-wire the two. Never logged, and never written to a workflow step output. |

Setting `OMNIGENT_MACHINE_CLIENT_ID` and `OMNIGENT_MACHINE_CLIENT_SECRET` is
the preferred path for an automated caller: the driver exchanges them for a
short-lived bearer **in-process**, at `POST /oauth/token` with
`grant_type=client_credentials`, so the token itself never has to transit a
workflow step output or a log line to reach the process that uses it. An
explicit `OMNIGENT_API_TOKEN` (or `_FILE`) overrides the exchange if both
forms are present.

**`OMNIGENT_MACHINE_CLIENT_SECRET` is the one secret an operator has to
configure** to run this tool from CI — everything else above has a workable
default or, for the client id, defaults to `sei-droid`.

### Permission policy

| Variable | Default | Meaning |
|---|---|---|
| `XREVIEW_ALLOW_POLICIES` | *(empty)* | Comma-separated exact `policy_name` values to accept automatically. |
| `XREVIEW_ALLOW_TOOLS` | *(empty)* | Comma-separated exact `tool_name` values to accept automatically, e.g. `Bash`. This deployment does send `tool_name`, measured, so this is the narrower of the two allowlists: `policy_name` is `claude_native_permission` for every native permission request, which makes allowing it equivalent to accepting every tool call. The review workflow passes `Bash,Read` — the binary's own default stays closed, so a bare local run allows nothing. |

See [Permission policy](#permission-policy) below — **read this before
assuming a blank allowlist is merely "cautious."**

### Independent readings

| Variable | Default | Meaning |
|---|---|---|
| `XREVIEW_SCOUTS` | *(empty)* | Scouts to gather a reading from before the review, as `name=agent,name=agent` — e.g. `codex=sei-droid-codex,cursor=sei-droid-cursor`. Empty runs the review alone, which is the behaviour without this set. Each scout reads the same pull request in its **own session on its own agent**, seeing neither the review nor another scout; the review then verifies their claims against the diff and merges what holds. The agent is what fixes the harness, so a scout naming the review's own agent is refused — a reading from the same harness is not independent of the one it is checking — as are two scouts on one agent, which would count one opinion twice. A scout that fails contributes a note the review is shown rather than nothing, because a reading that failed must not look like a reading that found nothing. Scouts share `XREVIEW_RUN_DEADLINE_S`: they run in parallel with two fifths of it between them, and the review keeps its own full deadline, so a run costs at worst `deadline x 1.4` end to end. |

### Timeouts (seconds)

| Variable | Default | Meaning |
|---|---|---|
| `XREVIEW_RUN_DEADLINE_S` | `1200` | Bounds the whole run: resolve, create or adopt, and drive. On expiry the run ends and the session is left as it is — the turn keeps running server-side, and the next invocation's prompt queues behind it. |
| `XREVIEW_REQUEST_TIMEOUT_S` | `30` | Bounds the requests this driver times itself: the token mint and the post-turn reply read. It is deliberately not handed to the SDK as a unary timeout, so the client's own calls (listing, create, send, resolve) keep the SDK's default and tightening this does not tighten those. The event stream is bounded by `XREVIEW_STREAM_IDLE_TIMEOUT_S` instead. |
| `XREVIEW_STREAM_IDLE_TIMEOUT_S` | `300` | How long the event stream may sit silent before it's treated as dead. The server heartbeats an idle stream every 15s, so this must stay comfortably above that or a healthy idle stream gets torn down between turns. Minutes rather than seconds because a *newly created* session is quiet while its sandbox provisions, clones the repository and connects a runner: a measured launch produced two heartbeats in 90 seconds while cloning a large repo, and the old 90s default killed the review before the agent existed. The run deadline is the real backstop. |

Each of these must parse as a positive number; zero, negative, or
non-numeric values are rejected as configuration errors rather than silently
producing an unbounded run.

## Exit codes

The calling workflow branches on these, so the numbers are a contract with
it — carried over unchanged from the Python driver this repository
replaces, and any workflow pinned to an older ref must keep reading them the
same way.

| Code | Constant | Meaning |
|---|---|---|
| `0` | `ExitOK` | A completed turn that produced a verdict. A duplicate trigger is not a separate outcome: it adopts the existing session and drives it to a verdict like any other run. |
| `2` | `ExitConfig` | The run was rejected before it reached the review API: bad arguments, a missing or rejected credential, a base URL the client refuses, or a failed token exchange. Note that a token exchange failing on the *network* also lands here, so a flapping `2` can mean "retry" rather than "fix the secret" — the logged error distinguishes them. |
| `3` | `ExitTimeout` | The run deadline (`XREVIEW_RUN_DEADLINE_S`) expired. The turn is abandoned rather than stopped — it keeps running server-side, and the next invocation's prompt queues behind it. The conversation is kept. |
| `4` | `ExitTurnFailed` | The session reported failure — the agent's outcome, not the driver's. |
| `5` | `ExitNoVerdict` | The turn ended without a verdict this driver could attribute and parse. The turn may have succeeded and simply not produced the closing block the caller needs. Also reported when more than one turn replied into the session while ours ran, when no reply carries this turn's response id, and when a reply looks like it carries a credential — the log's `reason` distinguishes them, and every one of those refuses rather than posting text it cannot attribute. |
| `6` | `ExitTransport` | The stream or a request failed in a way retrying inside one run can't fix. |
| `7` | `ExitCancelled` | A terminate signal (SIGINT/SIGTERM) unwound the run. Teardown is attempted on the way out. |
| `8` | `ExitTeardownLeak` | A `--close` whose session could not be deleted. A review never reports it: it deletes nothing, because the session is meant to outlive the run. The delete is the only thing that reclaims the sandbox — the Kubernetes launcher sets no lifetime cap and the server runs no sweep — so this means a pod is holding its reserved cpu and memory with nothing left to use it. |

`1` is not used by this program; nothing here relies on it meaning anything
in particular.

## Permission policy

A review turn runs with the session owner's execution identity, and some of
what it might do — a shell command, a write, anything else the server's
policy engine has a phase for — parks the turn on a permission prompt
(an *elicitation*) instead of running it. `xreview` answers every one of
those prompts itself; there is no human in the loop during a run.

**The policy denies by default and accepts only on an exact match** against
`XREVIEW_ALLOW_POLICIES` (matched on the server-attested `policy_name`) or
 `XREVIEW_ALLOW_TOOLS` (matched on `tool_name`, which this deployment does
stamp). There is no substring,
prefix, or pattern matching anywhere in this. That's deliberate: a prompt
raised while the agent reads a pull request is raised over content the
agent was asked to review, i.e. attacker-influenced input in the general
case. Any matcher whose behavior a PR's diff, description or comments could
influence is a matcher that untrusted content could steer into approving
its own escalation — so the only fields this policy will ever classify on
are ones the server's policy engine stamps, never the model-influenced
`message` or `content_preview` that ride along with a prompt for logging.

**An empty `XREVIEW_ALLOW_POLICIES` (the default) means every prompt is
declined, with no exceptions.** This is not a "reasonably conservative"
default that still lets ordinary reads through — a review turn that needs
to run something gated behind a policy simply cannot, until an operator
adds that exact `policy_name` to the allowlist. Concretely: on a deployment
where every one of the turn's read operations happens to be gated, a blank
allowlist means the turn parks on every one of them, gets `decline`d, and
the review effectively can't read anything the gate covers. This is the
same fail-closed posture the Python driver had in practice (it classified
on a `tool_name` the server never actually sent, so its allowlist could
never match anything either) — the difference here is that this is the
documented, intended behavior rather than an accidental one, and it is
configurable rather than permanently closed.

**The review prompt's first step is a shell command**, so this is not a knob
that trades review depth for caution. `gh pr diff` needs `Bash`; decline it and the turn reports that it could not
read the diff, which is what the prompt tells it to do rather than review from
the title. A read *inside* the agent's working directory raises no prompt, which
is why the diff stages there — a recorded run staged it in `/tmp`, had the read
refused, and spent three tool calls copying the file somewhere it could read.
`seidroid-xreview.yml` therefore passes `Bash,Read`. The access control on a review
is its trigger gate: the comment must come from a non-bot account whose association
is `OWNER`, `MEMBER` or `COLLABORATOR`, and where `allowed-team` is set, from an
active member of that team.

To find out what else to allow: run once, watch the structured logs for
`"deciding a permission prompt"` records, and read off the **`tool_name`**
each declined prompt carried. Add the ones this deployment's review turns
legitimately need to  `XREVIEW_ALLOW_TOOLS` (comma-separated, exact values,
no wildcards). Every accepted prompt is logged the same way, so an allow
decision is exactly as visible after the fact as a decline is.

Read off `tool_name` and not `policy_name`, and the reason is the whole
point of the table above: on this deployment every native permission prompt
carries the same `policy_name`, `claude_native_permission`, because the
field names the policy that fired rather than the action it gated. Putting
that one value in `XREVIEW_ALLOW_POLICIES` accepts every tool call the agent
makes. `tool_name` is per-tool, so it is the narrowest thing here that can
be allowed deliberately.

Neither knob buys a read-only guarantee. `Bash` is arbitrary shell, so
allowing it allows a push as readily as a diff read; the read-only posture
rests on the prompt's instruction and the server-side gate.
