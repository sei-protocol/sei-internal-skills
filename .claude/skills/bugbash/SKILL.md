---
name: bugbash
category: hardening
model: claude-opus-4-8
description: "Use when an existing system works on the happy path but needs adversarial hardening before launch — 'bug bash', 'bugbash', 'pressure-test the X', 'red-team the X', 'harden the X before launch', 'find bugs in the X', 'adversarial review of X', '/bugbash'. Anti-triggers: NOT for greenfield design (use /council); NOT for collaborative iteration on a feature in progress (use /coral); NOT for PR review (use /review); NOT for security-only single-pass review (use /security-review — bugbash is broader and looped); NOT for production incident triage. Inspired by the RALPHY loop, reframed for hardening rather than greenfield work."
---

# Bugbash

Read-only adversarial review of an existing system. The orchestrator dispatches the council of experts in repeating passes against a named target — each pass surfaces new findings, then a *challenger* pass tries to refute or downgrade them — until the experts converge on a launch verdict.

The output is a single structured markdown findings **log** — a **lineage artifact** that lands in the DRI's `<engineer>-designs` repo at `designs/<arc>/bugbash/<target>.md` (repo-default arc `tide-skill-stack` for Tide; Design 13 — process-artifact relocation). Resolve the DRI repo as `/design` does (`--designs-repo` → sibling `<engineer>-designs` checkout → ask; in a non-interactive run, HALT — never guess); the in-repo `docs/bugbash/<target>.md` is the fallback used **only when no DRI repo is resolvable and the user confirms**. The resume **state** (`.bugbash/<target>.yaml` + `.bugbash/archive/`) now **also relocates to the DRI repo** at `designs/<arc>/bugbash/<target>.yaml` (alongside the log), resolved via the same `/design` resolver (Design 13 R3 — all process artifacts, incl. coordination state, leave the code repo); the in-repo `.bugbash/<target>.yaml` is the fallback used **only when no DRI repo is resolvable**. Because the state is read at session start, the read is **fail-loud** per Design 13 §4 (see Resume State below). The skill never edits source code; it only writes to the findings log and the state file.

This is the right tool when the system exists and works on the happy path, but you want to harden it before a launch by drilling for logical errors, validation gaps, race conditions, operational risk, and bottlenecks the original authors missed.

## Guardrails

This skill operates in **read-only mode** on the target component. Before any action:

1. **Permissioned mode** — the skill MAY read source code, configs, manifests, and existing docs. The skill MUST NOT edit, create, or delete any file under the target's source tree. The only files this skill writes are the findings **log** — `designs/<arc>/bugbash/<target>.md` in the DRI `<engineer>-designs` repo (in-repo `docs/bugbash/<target>.md` only as the no-DRI-repo fallback (with user confirmation)) — and the resume **state** `designs/<arc>/bugbash/<target>.yaml` in the same DRI repo (in-repo `.bugbash/<target>.yaml` only as the no-DRI-repo fallback (with user confirmation); Design 13 R3).
2. **Scope confirmation** — the skill requires an explicit target on first invocation (e.g., `/bugbash SeiNode controller`, `/bugbash review-runtime`). Without a target, the skill asks for one and refuses to proceed.
3. **Refusal conditions** — this skill will refuse to run if:
   - No target component is named.
   - The target's source path doesn't exist or can't be located in the repo.
   - A specialist proposes a code change during a pass — the orchestrator records the finding and reminds the specialist this is read-only.

If a finding warrants an immediate fix, surface it to the user; do not edit code from inside this skill. The user can dispatch `/coral` or `/council` separately to act on a finding.

## Preconditions

- A target repo with `.claude/agents/*.md` defining the specialist roster (or the user names experts explicitly).
- A reachable target component — a directory, package, CRD, runtime, contract, or interface boundary the experts can read in full within their context.
- Write access to the DRI `<engineer>-designs` repo's `designs/<arc>/bugbash/` — home of both the findings log (in-repo `docs/bugbash/` only as the no-DRI-repo fallback (with user confirmation)) and the resume state `<target>.yaml` (in-repo `.bugbash/` only as the no-DRI-repo fallback (with user confirmation); Design 13 R3) — created if missing.

If the repo maintains an interface registry or equivalent source of truth, specialists read it as authoritative when reviewing interface boundaries.

## Locating the Target Repo and Roster

1. CWD is the target repo unless the user says otherwise.
2. Read `CLAUDE.md` if present — repo conventions, governing principles, interface registry pointer.
3. Read `.claude/agents/*.md` — the specialist roster. If absent, ask the user which experts to use.
4. **Resolve-then-read the resume state, fail-LOUD (Design 13 §4).** The resume state and the findings log both live in the DRI repo, so before reading either, resolve the DRI repo via the `/design` resolver. If the DRI repo is **expected but can't be trusted** — present-but on an unexpected branch / mid-rebase / dirty-in-conflict / behind-remote / un-fetched, or **headless with no user to confirm the mode** → **HALT and surface** — never silently "start fresh," never conclude "no run in progress," never miss a resume point against a stale or wrong checkout. An unconfirmed read is `inconclusive ⇒ halt`. **Mode discriminator (resolves the HALT-vs-fallback question):** in the normal **DRI-repo mode** the in-repo `.bugbash/<target>.yaml` is migration-emptied, so it is **producer-write-only — never a session-start read source**, and an unreachable expected DRI repo HALTS. **Only when the user has confirmed no-DRI-repo mode** is the in-repo path the legitimate store, read normally (it is not emptied in that mode). The HALT is about an *expected-but-unreachable* DRI repo, not the confirmed-local mode. Once the store is resolved, check for prior state:
   - `designs/<arc>/bugbash/<target>.yaml` in the DRI repo (resume state) — if it exists, a previous session left an in-progress run. Read it before acting.
   - The findings log at `designs/<arc>/bugbash/<target>.md` in the DRI repo (resolved fail-loud per the step above — **never read from the migration-emptied in-repo `docs/bugbash/` dir**, which would resume from a stale/empty log) — if it exists, it is the source of truth for what has already been reviewed.

When prior state exists, surface it to the user: "Found a bugbash in progress for `<target>` — pass <N>, <K> findings, convergence counter <C>/2. Continue, or archive and start over?"

## Procedure

### 1. Scope the Target

Confirm the target component with the user. Ask for the root path (directory, package, CRD spec file, or interface boundary). Echo back: "Bugbashing `<target>` rooted at `<path>`. Read-only — I'll only write the findings log to `designs/<arc>/bugbash/<target>.md` and the resume state to `designs/<arc>/bugbash/<target>.yaml`, both in the DRI `<engineer>-designs` repo (in-repo `docs/bugbash/<target>.md` / `.bugbash/<target>.yaml` only if no DRI repo). Proceed?"

If the target is unclear or too broad ("the whole controller" with no narrowing), push back: bugbash works best on a single component or interface boundary. Suggest splitting.

### 2. Build the Expert Slate

From `.claude/agents/`, pick the experts whose lens applies to this target. Aim for 3–6 experts covering:

- The component's **primary owner** (e.g., `kubernetes-specialist` for a controller, `solidity-developer` for a contract).
- **Adjacent component owners** (consumers/providers across the target's interface boundary).
- A **security lens** (`security-specialist` if present).
- An **operability lens** (`platform-engineer`, or whoever owns runtime/deployment).
- A **scope-discipline lens** (`product-manager` or equivalent) for triage and severity calibration during the verdict round.

Record the slate in `designs/<arc>/bugbash/<target>.yaml` (the DRI-repo resume state; in-repo `.bugbash/<target>.yaml` fallback) under `experts:`. Once chosen, the slate is fixed for the run — switching mid-bugbash invalidates convergence.

### 3. Run a Pass

A pass has four phases. See `references/loop-mechanics.md` for the full mechanics.

**3a. Discovery (parallel).** Dispatch every expert in the slate in parallel with the same brief: "Read `<target path>`. Adversarially review for logical errors, validation gaps, race conditions, error-handling holes, operational risk, deployment safety (graceful rollout, restart survival, no stuck state across release cuts), and bottlenecks within your domain. Output **up to 5 candidate findings** — prioritize the most important within your domain over breadth. Output findings only — no proposed fixes, no severity, no impact dramatization. State observations as plainly as possible: title, file:line, what goes wrong on what path. Do NOT use words like 'critical' or 'silent broken-window' in your framing — the challenger phase assigns severity. Do NOT propose code edits — read-only."

Each specialist returns up to 5 candidates. Append the union to a working set in `designs/<arc>/bugbash/<target>.yaml` (DRI-repo resume state; in-repo `.bugbash/<target>.yaml` fallback) under `pass-N.candidates:`.

**3b. Merge (orchestrator).** Before the challenger phase, the orchestrator deduplicates the candidate set. Real findings overlap across expert lenses — e.g., a non-defensive template renderer surfaces as both a "future-template footgun" (k8s lens) and a "${VAR} injection vector" (security lens), but it's one finding. Walk every pair of candidates and merge when they share a root cause or cite the same file:line, attributing both finder experts on the merged candidate. The challenger then evaluates the merged finding once instead of N times. See `references/loop-mechanics.md#orchestrator-merge` for the merge rubric.

**3c. Challenger (parallel).** Each merged candidate is challenged by a *different* expert from the slate (never one of the finders), dispatched in parallel with the brief: "Try to refute this finding. Is it actually a bug? Already mitigated upstream? Out of scope for this target? Lower severity than it looks? Write a one-paragraph verdict: confirm / refute / downgrade. If you confirm or downgrade, propose a severity per `references/severity-rubric.md`." Confirmed and downgraded findings advance; refuted ones are dropped and recorded in state with the challenger's reasoning.

**3d. Triage and write.** For each surviving finding, the orchestrator calibrates the challenger's proposed severity against `references/severity-rubric.md` (adjust if the rubric suggests otherwise), drafts the entry per `references/format-spec.md`, and appends to the findings log at `designs/<arc>/bugbash/<target>.md` in the DRI repo (in-repo `docs/bugbash/<target>.md` fallback). Update the resume state `designs/<arc>/bugbash/<target>.yaml` (in-repo `.bugbash/<target>.yaml` fallback): increment pass counter, record finding IDs, update convergence counter (see step 4).

### 4. Convergence Test

After each pass, count new findings of severity ≥ Medium added in that pass.

- If **0 new ≥ Medium** findings in this pass: increment `convergence_counter` by 1.
- If **≥ 1 new ≥ Medium** finding: reset `convergence_counter` to 0.
- When `convergence_counter` reaches **2** (two consecutive passes with no new ≥ Medium): advance to the launch verdict (step 5).

This is the loop's terminator — analogous to RALPHY's `<promise>COMPLETE</promise>` sentinel, but driven by saturation rather than checklist completion. See `references/loop-mechanics.md` for why 2 and not 1.

### 5. Launch Verdict

Dispatch every expert in the slate one final time with the full findings log and this brief: "Given the findings in the bugbash log (`designs/<arc>/bugbash/<target>.md` in the DRI repo, or the in-repo `docs/bugbash/<target>.md` fallback), post a launch verdict for the target. Choose one: **ship-it** (all blockers addressed or never present), **conditional** (ship-it if the following findings are closed: [IDs]), or **don't-ship** (the system is not safe to launch even if listed findings are addressed — explain why)."

Append the verdicts as a `## Launch Verdict` section in the findings log (`designs/<arc>/bugbash/<target>.md` in the DRI repo; in-repo `docs/bugbash/<target>.md` fallback). The skill is **done** when:

- Every expert posts ship-it, OR
- Every expert posts ship-it OR conditional, AND every finding ID named across all conditionals is **Critical** or **High** severity (Mediums must be tracked but don't block launch).

If any expert posts don't-ship, the skill reports the blocker to the user and stops. Don't-ship overrides everything — no launch until that expert is satisfied.

### 6. Hand-off

Once the verdict converges, summarize for the user:

- Counts: `<X> Critical, <Y> High, <Z> Medium, <W> Low` and which are launch-blockers.
- The artifact path: the findings log at `designs/<arc>/bugbash/<target>.md` in the DRI repo (in-repo `docs/bugbash/<target>.md` if no DRI repo was resolvable).
- Suggested next step: "Run `/issue` over each Critical/High to file a tracked issue, or `/coral` against a finding to start the fix."

The skill does not file issues itself. The findings log is the synopsis; `/issue` is how individual items become tracked work.

## Halt Conditions

Stop and report rather than auto-recovering when:

- A specialist refuses to read the target (missing files, permissions). Report what was captured; ask the user to resolve.
- The convergence counter never advances past 0 across 5+ passes — the target may be too broad. Report and suggest narrowing.
- An expert posts don't-ship at the verdict round. Report the blocker; do not retry the verdict round automatically.
- The DRI `<engineer>-designs` repo is **expected but unreachable/untrustworthy** at session start — present-but on an unexpected branch / mid-rebase / dirty-in-conflict / behind-remote, or headless with no user to confirm the mode. **HALT fail-loud** rather than read a migration-emptied in-repo `docs/bugbash/`/`.bugbash/` and resume from a stale or empty state. (When the user has **confirmed no-DRI-repo mode**, the in-repo path is the legitimate store and is read normally — the HALT is about an expected-but-unreachable DRI repo, not the confirmed-local mode.) (Design 13 §4)
- The user interrupts mid-pass. State is in `designs/<arc>/bugbash/<target>.yaml` in the DRI repo (in-repo `.bugbash/<target>.yaml` fallback); next invocation offers resume.

## Rationalization Table

Pressure patterns that surface during long-running bugbashes and their counters. These mostly fire when the loop is long, the room is tired, or the launch deadline is close — the moments when the read-only / convergence-mechanical / scope-disciplined defaults feel like overhead.

| Excuse | Reality |
|---|---|
| "We have enough findings — the launch decision is clear, stop now." | Convergence is mechanical (counter==2 plus verdict round). The expert slate owns the verdict, not the requester. "Enough material to decide" and "the loop has converged" are different things. |
| "Just one more pass and we'll be done." | Convergence is two consecutive passes with zero new ≥ Medium findings, not a feeling. One more pass might surface a new finding that resets the counter — which is the point. |
| "Let me push the fix now — sitting on a known bug is malpractice." | Read-only is non-negotiable. Target mutation invalidates the run (the findings log no longer describes a single coherent thing). Record the finding, finish bugbash, then patch and start a fresh `/bugbash` to validate. |
| "Accept the fix, restart the discovery cycle against the fixed version." | No reconvergence after fixes — that's a fresh `/bugbash` run, not the current one continued. Reconvergence rules out the "patch-and-rerun-the-same-loop" anti-pattern. |
| "The team wants improvements / refactors / feature ideas captured here too." | Bugbash is adversarial review of *existing behavior*, not design or ideation. Hand off to `/coral` or `/council` for the design ideas; keep the findings log on its job. The verdict (ship-it / conditional / don't-ship) is incoherent on "the API could be cleaner." |
| "Let me expand the target mid-run — we're already in the code." | Slate is fixed once chosen; widening the target invalidates convergence and over-runs the experts' context. Run a separate `/bugbash` for the adjacent target. |
| "We don't need the challenger pass for this one — it's obviously a bug." | Discovery + challenger is the merge-and-refute step. "Obviously a bug" is the framing the challenger pass exists to test. Always run the challenger. |

## Red Flags — STOP and Reset

Phrases that signal you're about to violate one of the bugbash defaults. If any of these surface in your own reasoning or a teammate's framing, stop and reset to the documented rule:

- "We have enough" / "the verdict is clear"
- "Just one more pass"
- "I'll just fix this one"
- "Let me restart the cycle against the fixed version"
- "We should also capture these improvements"
- "Let me expand the target"
- "We don't need the challenger pass for this one"
- "It's still in dev, the rule doesn't really apply"

All of these mean: re-read the relevant SKILL.md section, apply the rule as written, and move forward. If the rule is genuinely wrong for this context, that's a SKILL.md edit through a PR — not a one-off override during a run.

## State Management

`designs/<arc>/bugbash/<target>.yaml` shape (in the DRI repo; in-repo `.bugbash/<target>.yaml` only as the no-DRI-repo fallback (with user confirmation)):

```yaml
target: SeiNode-controller
target_path: pkg/controllers/seinode
started: "2026-04-29T10:00:00Z"
updated: "2026-04-29T14:30:00Z"
experts:
  - kubernetes-specialist
  - sei-network-specialist
  - security-specialist
  - platform-engineer
  - product-manager
pass: 3
convergence_counter: 1
findings:
  - id: 1
    severity: Critical
    title: "..."
    expert_finder: kubernetes-specialist
    expert_challenger: security-specialist
    challenger_verdict: confirm
  - id: 2
    severity: High
    title: "..."
    expert_finder: sei-network-specialist
    expert_challenger: kubernetes-specialist
    challenger_verdict: downgrade
    severity_before_downgrade: Critical
verdicts: {}  # populated at step 5
```

**All process artifacts relocate to the DRI repo — lineage AND coordination state (Design 13 R3).** The findings **log** is a lineage artifact: it lives in the DRI `<engineer>-designs` repo at `designs/<arc>/bugbash/<target>.md` (in-repo `docs/bugbash/<target>.md` only as the no-DRI-repo fallback (with user confirmation)) — it is the canonical artifact for reviewers and downstream `/issue` filings. The resume **state** `<target>.yaml` (and `archive/`) is coordination state and now **also lives in the DRI repo** at `designs/<arc>/bugbash/<target>.yaml`, resolved via the same `/design` resolver (in-repo `.bugbash/<target>.yaml` only as the no-DRI-repo fallback (with user confirmation)). R3 reverses the earlier "state stays local" scope: the owner principle is "nothing Tide-specific inside the repos we work on." Because the state is read at **session start**, the move is safe **only** under the fail-loud bootstrap contract (Design 13 §4): resolve the DRI repo first and **HALT** on an unresolvable / unexpected-branch / mid-rebase / dirty-in-conflict checkout rather than silently starting fresh — see "Check for prior state" above. This mirrors the council convention along the same axis: both lineage (design docs via `/design`) and coordination state (`workstream.yaml`/`escalations/`/`archive/`) now route into the DRI repo.

When a run finishes (verdict converges), archive `designs/<arc>/bugbash/<target>.yaml` to `designs/<arc>/bugbash/archive/<date>-<target>.yaml` in the DRI repo (in-repo `.bugbash/archive/<date>-<target>.yaml` only as the no-DRI-repo fallback (with user confirmation)) so a fresh `/bugbash <target>` doesn't trip the "in-progress run detected" branch.

## Composition with Other Skills

- **`/security-review`** — single-pass, security-only review of a diff or branch. Bugbash is broader (logic, ops, perf, validation, race conditions — not just security) and looped to convergence. Run `/security-review` for a PR; run `/bugbash` to harden a system before launch.
- **`/coral`** — collaborative iteration on an idea. If `/coral` is going off the rails because the underlying system has too many unknown bugs, suggest pausing coral and running bugbash first.
- **`/council`** — full-ceremony design and implementation. After bugbash produces blockers, the user may run `/council` to fix the Critical findings as a workstream.
- **`/issue`** — files individual findings as tracked GitHub issues. Bugbash hands off; it doesn't auto-file.
- **`/design`** — captures a design. If a finding requires re-designing a component (not just patching it), the user runs `/design` after bugbash to capture the redesign.

## What Bugbash Doesn't Do

- **No code edits.** Read-only. Findings include suggested fix sketches, not patches.
- **No auto-issue-filing.** The user runs `/issue` per finding when they want it tracked outside the artifact.
- **No new-system design.** If the target doesn't exist yet, that's a `/council` Product-tier job — not a bugbash.
- **No PR review.** PR review is `/review`.
- **No reconvergence after fixes.** Once shipping criteria are met and the team starts fixing findings, the bugbash run is done. To validate the fixes, start a fresh `/bugbash` run.
