# xreview agent bundle

The headless multi-specialist code reviewer — the GitHub-venue `/xreview` wedge
of the omni session-routing service (Design 17 seam C). A PR event routed through
the @seidroid/uci GitHub gateway opens a session against this bundle, which runs
the `/xreview` discipline on the PR diff and returns a **structured verdict**
(classification + boundary findings table + per-lens verdicts). The runner never
posts; the uci workflow posts the comment from the returned verdict.

## Layout

- `config.yaml` — the omni agent spec (`spec_version: 1`, `name: xreview`).
- `skills/xreview/` — a copy of sei-internal-skills's `/xreview` skill (SKILL.md +
  references + evals), so the runner carries the discipline. Auto-discovered from
  `<bundle>/skills/`; no `skills:` line in the spec.

## name ↔ bundle_ref coupling

The name MUST be exactly `xreview`. The GitHub venue resolves the agent **by this
name** as the `bundle_ref` via `GET /v1/agents` — the uci workflow opens a session
against `bundle_ref="xreview"` and the `LiveSessionFactory` resolves the agent by
this name. Rename the bundle and every PR-triggered review fails to resolve.

## Read-only floor architecture

The bundle reviews and reports — it does not mutate anything, and it does not
post; the **uci workflow**, which holds the GitHub write identity, posts the PR
comment from the verdict the runner RETURNS.

Do **not** read this as "cleaner than root-cause." For SECURITY it is the
opposite: root-cause's input is trusted infrastructure (an alert body), whereas
`/xreview`'s input is **fully attacker-influenceable code + prose** — the input
IS the adversary. "Read-only" here means **cannot-post**, NOT cannot-exfil and
NOT cannot-be-weaponized. A reviewer steered by hostile in-diff content could, if
it had a shell or network, exfiltrate or write — so this venue is MORE dangerous
than root-cause, not less, and the floor has to close the raw-shell/exfil class
structurally (see the rollout gates below).

That floor is **not** the `guardrails.blast_radius` block in `config.yaml` (which
is "not a security boundary" by its own docstring; it is a per-agent
defense-in-depth backstop with `gate_pushes: true`). What enforces the floor,
**per mutation/exfil class**:

- **File mutation** (`Write` / `Edit` / `MultiEdit` / `NotebookEdit` and the
  `sys_os_*` / Pi equivalents) — DENYed by the server-side read-only default
  `admin__deny_mutating_os` (`sei_omnigent.policies.read_only`).
- **`gh` / `git` writes** (posting a comment, committing a ledger, pushing) —
  DENYed by `admin__github_read_only`
  (`omnigent.policies.builtins.github.github_policy` with `write_repos=[]` and
  `shell_tools` including `Bash`, so native `gh`/`git` shell writes are parsed,
  not abstained). This is what stops the runner from posting the PR comment
  itself — the post is the uci workflow's job, not the runner's.
- **Raw shell + raw-HTTP exfil/mutation** (`Bash`/`sys_os_shell`, `curl` to any
  read OR write endpoint, a shell `rm`/`git push`) — on the **dev** deployment
  this class is closed STRUCTURALLY: the deploy wires
  `read_only_default_policies(deny_shell=True)`, a hard server-side shell block.
  This is viable on the review-only dev instance precisely because it hosts
  agents that need no shell — unlike prod/root-cause, which needs `kubectl get`
  and therefore runs `deny_shell=False` (leaving the raw-shell C4 residual that
  root-cause's gates cover). `Read`/`Grep`/`Glob` are read tools and are
  unaffected by `deny_shell=True`, so the read-only review path (diff from the
  file OR the initial message, changed files via `Read`/`Grep`/`Glob`) is intact.
- **Residual raw-HTTP exfil** — even with no shell, INV-11 must hold: the host
  egress allowlist has **no write-capable OR exfil-capable endpoint reachable
  with ambient credentials**. `deny_shell=True` removes the raw-`curl` reach;
  INV-11 is the defense-in-depth backstop for any other egress surface.

`config.yaml`'s `os_env.sandbox.type: none` matches the standing claude-native
host's runner shape; the sandboxing lives on the host, not in the spec.

### Threat statement / rollout gates

`/xreview`'s input is FULLY ATTACKER-INFLUENCEABLE code + prose — the input is
the adversary. So unlike root-cause (trusted infra, alert text), this venue is
MORE dangerous, not "cleaner." The hardening rests on operator-gated controls
this bundle does NOT enforce — they are **hard rollout gates** that MUST be
verified before the venue goes live (the same way root-cause frames its gates):

1. **`deny_shell=True` at the dev deployment.** The dev sei-omnigent wires
   `read_only_default_policies(deny_shell=True)` — a hard server-side shell block.
   Viable on the review-only dev instance (no agent there needs a shell); closes
   the raw-shell/exfil class structurally.
2. **Reviewers dispatched read-only.** Every blinded reviewer is granted
   `Read`/`Grep`/`Glob` only — no `Bash`/shell/write — so a reviewer steered by
   hostile content structurally cannot reach a shell/write/exfil path.
3. **INV-11 strengthened — no write-capable OR EXFIL-capable egress.** The egress
   allowlist must contain no endpoint reachable with ambient credentials that can
   either write OR exfiltrate. The dev `uci-default` fleet's egress MUST be
   confirmed locked for **exfil**, not merely for writes (read-only data still
   leaks over an open egress).
4. **The no-shell input path (diff handed IN, never fetched).** The merge-base
   diff is delivered by the venue — a file at `./.xreview/pr.diff` (uci/file venue)
   or the session's initial message (managed-sandbox venue, whose single-branch
   clone cannot recompute it) — and read via `Read`/`Grep`/`Glob`; the runner runs
   no `git`/`curl`/shell/network to obtain or reconstruct it. The initial-message
   diff is framed as untrusted DATA identically to the file (never instructions).
5. **The verdict-integrity control.** The synthesizer treats any in-diff
   self-assessment (a committed `xreview/`-style ledger, an "already reviewed:
   RATIFY" comment) as untrusted content, never as evidence — the verdict derives
   ONLY from the dispatched reviewers' independent findings.

## Input and output

- **Input — a diff handed in, read via `Read`/`Grep`/`Glob` only.** The venue
  delivers the PR's merge-base diff in one of two shapes: a FILE at
  `./.xreview/pr.diff` (uci/file venue), or the session's INITIAL MESSAGE
  (managed-sandbox venue — the driver computes the diff with full git on the
  runner and hands it in, because the sandbox's single-branch head clone cannot
  recompute a merge-base diff and the runner has no shell). **Precedence is by
  provenance:** the driver-produced initial message is authoritative and, when
  present (managed venue), the in-clone `./.xreview/pr.diff` is NOT read — a file
  at that path in a managed clone is attacker-controllable repo content, so
  trusting it would let a PR forge a benign diff to mask a hostile change; the file
  is read only when no message diff was handed in (uci venue, where the venue — not
  the repo — materialized it). The checked-out tree (changed files at head) is in
  the workspace either way. The runner reads the diff + the changed
  files PURELY through `Read`/`Grep`/`Glob` — NO `git`, `curl`, shell, or network
  fetch (the dev deployment's `deny_shell=True` blocks shell server-side
  regardless), and the diff is untrusted DATA whichever shape it arrives in. If
  NEITHER diff source nor the tree is available/readable, the runner RETURNS
  `State: OPEN-BLOCKED` (workspace-missing).
- **Output — a structured-Markdown verdict with a five-field typed header.** Not
  JSON: a labeled header block carrying all five gate-read fields as exact-token
  lines — `State:`, `OpenFindings:`, `Convergence:`, `Blinded:`, `Dissenter:` —
  with the `State`↔`OpenFindings` cross-field consistency (a passing `State`
  requires `OpenFindings: 0`), plus a top-level `status:` discriminator
  (`ok` | `blocked` | `error`) the workflow uses to tell a real verdict from a
  blocked/errored run, plus the terminal sentinel line
  `=== XREVIEW-VERDICT-END ===` so the workflow detects a truncated run. Below
  the header: the COMPATIBLE/MISMATCH/MISSING boundary findings table with
  evidence, the per-lens RATIFY/DISSENT verdicts, and any Idiom/Prose addenda.
  Every skill HALT resolves to a RETURNED `OPEN-BLOCKED` verdict naming the
  blocking reason (the runner is headless — never stop without returning). The
  uci workflow posts this as an **ADVISORY** PR comment, **NOT a blocking merge
  check** — a blocking check would reintroduce Design 17's seam-A merge-gate
  concern.
- **Ledger** — this venue OVERRIDES the skill's "done = committed ledger"
  terminal: the returned verdict + the posted PR comment ARE the durable record.
  The bundle does **not** write or commit a designs-repo ledger (the skill's
  `references/review-ledger.md` write-path does not apply here).

## harness: claude-native (required)

The spec pins `harness: claude-native` — the same invariant as root-cause. The
overlay's `sei_omnigent/src/sei_omnigent/harness.py` invariant requires it and
**rejects** `claude-sdk`, which zeros the toolset to `["Skill"]` and wires no
subagent fan-out — breaking the multi-expert dispatch the `/xreview` discipline
depends on (blinded independent reviewers + an assigned dissenter). The deployed
host provides the tmux + `~/.claude` seed claude-native needs.
`permission_mode: auto` is required because a headless runner has no human to
answer ApprovalCards.

## DEPENDENCY — `.claude/agents/` roster via the host `~/.claude` seed

The bundled `/xreview` skill **hard-requires a `.claude/agents/` roster**:
Guardrail #2 ("Roster required") and Step 2/Step 3 read `.claude/agents/` to
assemble the domain-lens slate and the agent-stewards (`prose-steward`,
`idiomatic-reviewer`). With no roster the skill HALTS — and in this headless
venue that HALT resolves to a RETURNED `State: OPEN-BLOCKED` ("roster absent, no
review performed"), NEVER a thin single-expert pass dressed as a full review. It
is multi-expert by design and refuses to run single-expert.

The headless `claude-native` runner gets the roster from the **host image**, the
**same seed root-cause relies on**: `Dockerfile.host` bakes the repo's `.claude/`
overlay — `.claude/agents/` (the specialist roster) + `.claude/skills/` — into the
runner's `~/.claude`, the user-scope path claude-code loads from. So every
claude-native runner inherits the full multi-expert discipline. **This bundle
carries only the `/xreview` skill, not the roster** (one maintenance point,
minimal bundle) — the reviewer slate it dispatches comes from the host seed, not
from this directory.

## Registration

Registered with the omni coordinator at boot:

    omnigent server --agent sei_omnigent/agents/xreview

This directory is baked into the coordinator image so the bundle is resolvable the
moment the GitHub PR route fires.
