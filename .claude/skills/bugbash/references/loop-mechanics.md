# Loop Mechanics

The bugbash loop is RALPHY-shaped — a stateful iteration that runs until a sentinel fires — but adapted for hardening rather than feature completion. Where RALPHY exits when every story has `passes: true`, bugbash exits when adversarial review *saturates* (no new findings ≥ Medium) AND every expert posts a launch verdict they can stand behind.

This document covers the mechanics: pass structure, the challenger pattern, severity calibration, convergence, the verdict round, and the rationale behind each choice.

## Why a loop and not a single pass

A single round of expert review produces N parallel checklists. The findings each expert surfaces reflect their own initial reading of the system. What a loop adds:

1. **Saturation evidence.** When a pass produces no new ≥ Medium findings, the experts have collectively seen everything they would have seen given enough time. One pass can't tell you the experts are done; two consecutive empty passes can.
2. **Cross-pollination.** Each pass, an expert reads the previous pass's findings before doing their next discovery round. Findings from one expert prime another to look at adjacent failure modes. This is most of the bug-bash effect.
3. **Adversarial pressure on findings.** The challenger pass is the adversarial heart of bugbash — without it, you have a checklist. With it, each finding has had at least one other expert try to invalidate it.

## Pass structure

Each pass has four phases, executed in order. The merge phase (Phase 2) was added after a dry-run found that 5 experts × 7 findings produced 35 candidates with substantial cross-lens overlap, and per-finding challenger dispatch didn't scale. Merging first cuts the challenger workload in half or more without losing signal.

### Phase 1: Discovery (parallel)

Every expert in the slate gets the same brief simultaneously. Brief shape:

```
Read <target path> in full. You may also read interface registries, CLAUDE.md,
and any docs the target references. You MAY NOT propose code edits — read-only.

Adversarially review the target for, within your domain:
- Logical errors and validation gaps
- Race conditions and ordering hazards
- Error-handling holes (silent failures, swallowed errors, partial state on failure)
- Operational risk (resource exhaustion, retry budgets, eviction handling)
- Deployment safety (graceful rollout, in-flight work survives pod restart, no
  stuck state across release cuts, safe rollback, leader-election handoff
  without split-brain)
- Bottlenecks (lock contention, serial operations that should be parallel)
- Interface violations (provider/consumer mismatches against the registry)

Read the previous findings in the bugbash log (designs/<arc>/bugbash/<target>.md
in the DRI repo, or the in-repo docs/bugbash/<target>.md fallback) before you
start, so you don't re-surface what's already logged.

Output: UP TO 5 candidate findings. Prioritize the most important within your
domain over breadth — five high-quality candidates beat fifteen middling ones,
because every candidate carries a per-finding challenger cost downstream.

For each, give exactly:
1. A one-sentence title (no severity hint, no "critical" / "silent" framing)
2. The file:line citation
3. A two-sentence sketch of the failure mode — what goes wrong on what path,
   under what conditions, stated plainly

DO NOT assign severity. DO NOT propose fixes. DO NOT use words like "critical",
"silent broken-window", "operational footgun", or any framing that implies a
severity tier — the challenger phase assigns severity, and severity hints in
discovery prime the challenger toward confirmation. Just describe what you see.
```

The cap of 5 is a soft cap — an expert who genuinely sees seven critical issues should output them — but treat the cap as the default and the seventh-finding override as exceptional. The cap exists because every candidate produces a challenger dispatch; uncapped discovery turns the challenger phase into a denial-of-orchestrator.

The orchestrator collects every expert's candidates into the working set for this pass.

### Phase 2: Merge (orchestrator)

Real findings overlap across expert lenses. A non-defensive template renderer surfaces as both a "future-template footgun" (kubernetes lens) and a "${VAR} injection vector" (security lens) — same root cause, different framings. A kubeconfig that honors `users[].exec` plugins surfaces as both a "cluster-routing trust gap" (network lens) and an "arbitrary code execution vector" (security lens) — same code, different consequences.

If you skip merging, the challenger phase pays N times for one underlying finding. Worse, two challengers may give different verdicts for what is functionally the same issue, producing inconsistent severity in the final artifact.

#### Merge rubric

Walk every pair of candidates. **Merge** when any of these is true:

- They cite the same file:line, even if framed differently.
- They describe the same root cause via different file:lines (e.g., one cites the call site, one cites the helper function).
- They describe distinct symptoms of the same root cause (e.g., "rendered alias can break YAML quoting" and "rendered alias can break JSON escaping" — both are "render.Render does no escaping").

**Do not merge** when:

- They look superficially similar but the failure paths differ (e.g., "bench up has no rollback" and "bench down has no rollback" — same shape, different code paths, distinct fixes).
- One is a special case of the other but the special case has a meaningfully different fix.
- They share a file:line but the failure modes are independent (rare but happens — same line might have both a validation gap and a perf concern).

When merging, the merged candidate inherits all finder attributions: `Finder: kubernetes-specialist + security-specialist (same finding via different lenses)`. The challenger is then chosen as a *third* expert — never one of the finders.

#### Output of the merge phase

Update the resume state `designs/<arc>/bugbash/<target>.yaml` in the DRI repo (in-repo `.bugbash/<target>.yaml` fallback; Design 13 R3) with the merged candidate set under `pass-N.merged:`. Each entry tracks:

```yaml
- merged_id: 1
  finders: [security-specialist, kubernetes-specialist]
  source_candidates: [security-1, k8s-7]
  title: "render.Render performs unescaped ${VAR} substitution"
  file_lines: ["cluster/internal/render/render.go:25", "cluster/internal/render/render.go:43"]
  failure_mode_synthesis: "<one-paragraph synthesis of the merged framings>"
```

If the merge collapses ~30% of raw candidates (typical for a 5-expert slate), the challenger phase saves a third of its dispatches. If it collapses less than 10%, the experts are already producing distinct findings — fine, proceed.

### Phase 3: Challenger (parallel per merged finding)

Each merged candidate is challenged. Challengers run in **parallel** — every challenge is independent of every other challenge, so there is no reason to serialize them. Earlier versions of this skill said "sequential per finding"; that was wrong. Each candidate's challenge is its own dispatch and they all start at once.

Challenger choice:

1. Never one of the finders. With merged candidates, this means no challenger from the merged finder set.
2. Prefer the expert whose lens is most likely to invalidate the finding (e.g., a network finding goes to the kubernetes-specialist for challenge; a contract finding goes to the security-specialist).
3. Rotate fairly across the slate so no one expert challenges all findings.

Challenger brief:

```
Another expert (or experts, if this finding was merged across lenses) surfaced
this candidate:

  Title: <finding title>
  File:line(s): <citations>
  Failure mode: <sketch>

Try to refute it. Is this:
- Actually a bug? (confirm)
- Already mitigated upstream by <something>? (refute, name the mitigation)
- Out of scope for this target? (refute, name the boundary)
- Real but lower severity than it looks? (downgrade, propose new severity)

Write a one-paragraph verdict. If you confirm or downgrade, propose a severity
per references/severity-rubric.md and a one-line fix sketch. The severity
rubric is the authoritative tier definition — read it before assigning.

Be honest. The cost of an over-cautious confirmation is one item in the
artifact that gets fixed unnecessarily. The cost of a refutation that should
have been a confirmation is a missed bug. Lean confirm when in genuine doubt.
```

Verdict outcomes:

- **Confirm** — finding advances to triage with the challenger's proposed severity.
- **Downgrade** — finding advances at reduced severity. Recorded in state with the original framing's implied severity vs. the downgraded value, for audit.
- **Refute** — finding is dropped from the findings log. Recorded in the resume state `designs/<arc>/bugbash/<target>.yaml` in the DRI repo (in-repo `.bugbash/<target>.yaml` fallback; Design 13 R3) under `refuted:` with the reason, so the next pass doesn't re-surface it.

A challenger may not propose a *different* finding while challenging — they either resolve the current candidate or pass. Drift here weakens the convergence test.

#### Severity is assigned by the challenger, not the finder

A dry-run against a small CLI surface found that finders consistently overstated severity through framing language ("Critical — silent broken-window…") even though the discovery brief told them not to assign severity formally. Those framings primed the challenger toward confirmation. Three of three sampled challengers downgraded or refuted, demonstrating that severity is best assigned by the expert who didn't write the original framing.

The discovery brief's prohibition on severity hints is therefore non-negotiable. If a finder violates it, the orchestrator should either: (a) re-dispatch the finder with stricter framing, or (b) note the discovered severity hint and ignore it during the challenger brief, presenting the candidate to the challenger in neutral terms.

### Phase 4: Triage and write

The orchestrator:

1. Walks each surviving finding (confirmed or downgraded).
2. Calibrates severity against `severity-rubric.md`. The challenger's proposed severity is the starting point; the orchestrator adjusts if the rubric suggests otherwise. When the rubric is unambiguous, it wins over the challenger's call.
3. Drafts the entry per `format-spec.md` and appends to the findings log at `designs/<arc>/bugbash/<target>.md` in the DRI repo (in-repo `docs/bugbash/<target>.md` fallback) with the next sequential item number.
4. Updates the resume state `designs/<arc>/bugbash/<target>.yaml` in the DRI repo (in-repo `.bugbash/<target>.yaml` fallback; Design 13 R3):
   - Increment `pass`.
   - Append finding entries.
   - Update `convergence_counter` (see below).

## Convergence

The convergence counter tracks how many *consecutive* passes ended with zero new findings of severity ≥ Medium. It is the loop terminator.

- **0 new ≥ Medium in this pass** → `convergence_counter += 1`
- **≥ 1 new ≥ Medium** → `convergence_counter = 0` (reset)
- `convergence_counter == 2` → advance to the verdict round

### Why two and not one

One empty pass can be coincidence. Two consecutive empty passes is a much stronger signal that the experts have actually saturated against this target. Costs one extra pass; buys real confidence in the convergence claim.

### Why ≥ Medium and not all severities

Findings keep surfacing forever at the Low end — nitpicks, minor docs gaps, "this could be cleaner." Gating convergence on Lows means the loop never ends. The threshold is ≥ Medium because that's the severity floor for "things the team would care about before launch."

If three passes in a row produce only Lows, the team is spending expert time on diminishing returns; that's the signal to converge.

### When convergence stalls

If the convergence counter never advances past 0 after 5 passes, halt. The most common causes:

- Target is too broad — the experts keep finding new surfaces.
- Slate is wrong — an expert who keeps surfacing irrelevant findings (out-of-domain) skews the counter.
- The system genuinely isn't ready — the Critical/High count is large enough that the team should fix what's known before continuing the bash.

Report and ask the user. Don't loop forever.

## Verdict round

Once the convergence counter hits 2, the loop exits the pass cycle and runs one final round: the launch verdict.

Every expert in the slate is dispatched in parallel with the full findings log and this brief:

```
Read the bugbash findings log in full (designs/<arc>/bugbash/<target>.md in the
DRI repo, or the in-repo docs/bugbash/<target>.md fallback). Given everything
captured, post a launch verdict for this target.

Pick exactly one:

- ship-it: the system is safe to launch as-is, OR all blockers are already
  closed/mitigated. Give a one-sentence rationale.

- conditional: ship-it IF the following findings are closed first: [list of
  Item IDs]. Only Critical or High items belong here — Mediums are tracked
  but don't block launch.

- don't-ship: the system is not safe to launch even if every named finding
  is addressed. Explain why — there is something about the design or
  posture that no individual finding captures.

Be honest. The cost of an over-cautious verdict is one more pass; the cost
of a missed blocker is shipping broken code.
```

### Verdict acceptance rules

The skill exits successful when:

- **All ship-it.** Done.
- **Mix of ship-it and conditional**, AND every finding ID named across all conditionals has severity Critical or High. The launch criteria are: close those items.
- **Any don't-ship.** Skill halts. Report the blocker; do not retry the verdict round automatically. The user decides whether to address the structural concern (often by running `/council`) and re-run bugbash later.
- **A conditional names a Medium or Low.** Push back: "Expert X named Item N (Medium) as a launch blocker. Mediums don't block launch by rubric. Either re-evaluate severity, re-evaluate the verdict, or escalate to the user." If the expert maintains the position, surface to the user — there may be a rubric gap.

## State persistence

The skill is long-running by design — multi-session for a non-trivial target is expected. State lives in `designs/<arc>/bugbash/<target>.yaml` in the DRI `<engineer>-designs` repo (in-repo `.bugbash/<target>.yaml` only as the no-DRI-repo fallback; Design 13 R3), read fail-loud at session start per Design 13 §4. See SKILL.md for the shape and the fail-loud bootstrap rule.

Two important state invariants:

1. **The findings log is append-only across sessions.** Item numbers stay stable. Resuming a run never reorders or renumbers.
2. **The expert slate is fixed for a run.** Adding or removing an expert mid-run invalidates the convergence test (a new expert would surface findings the others missed but didn't reset the counter on). If the slate is wrong, archive the run and start over.

## Comparison to RALPHY

| RALPHY | Bugbash |
|--------|---------|
| Fresh agent per iteration | Fresh expert dispatch per pass |
| `progress.txt` accumulates learnings | `designs/<arc>/bugbash/<target>.md` (DRI repo) accumulates findings |
| `prd.json` tracks story completion | `designs/<arc>/bugbash/<target>.yaml` (DRI repo) tracks pass count + convergence |
| Exits on `<promise>COMPLETE</promise>` when all stories pass | Exits on convergence + launch verdict |
| Loop produces working code | Loop produces a hardening report |

Both treat the loop body as the unit of work; both rely on persistent state so each iteration starts with a clean context but full visibility into prior work.
