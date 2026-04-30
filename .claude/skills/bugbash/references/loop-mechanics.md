# Loop Mechanics

The bugbash loop is RALPHY-shaped — a stateful iteration that runs until a sentinel fires — but adapted for hardening rather than feature completion. Where RALPHY exits when every story has `passes: true`, bugbash exits when adversarial review *saturates* (no new findings ≥ Medium) AND every expert posts a launch verdict they can stand behind.

This document covers the mechanics: pass structure, the challenger pattern, severity calibration, convergence, the verdict round, and the rationale behind each choice.

## Why a loop and not a single pass

A single round of expert review produces N parallel checklists. The findings each expert surfaces reflect their own initial reading of the system. What a loop adds:

1. **Saturation evidence.** When a pass produces no new ≥ Medium findings, the experts have collectively seen everything they would have seen given enough time. One pass can't tell you the experts are done; two consecutive empty passes can.
2. **Cross-pollination.** Each pass, an expert reads the previous pass's findings before doing their next discovery round. Findings from one expert prime another to look at adjacent failure modes. This is most of the bug-bash effect.
3. **Adversarial pressure on findings.** The challenger pass is the adversarial heart of bugbash — without it, you have a checklist. With it, each finding has had at least one other expert try to invalidate it.

## Pass structure

Each pass has three phases, executed in order.

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
- Deployment safety (graceful rollout, in-flight work survives pod restart, no stuck state across release cuts, safe rollback, leader-election handoff without split-brain)
- Bottlenecks (lock contention, serial operations that should be parallel)
- Interface violations (provider/consumer mismatches against the registry)

Read the previous findings in docs/bugbash/<target>.md before you start, so
you don't re-surface what's already logged.

Output: a list of candidate findings. For each, give:
1. A one-sentence title
2. The file:line citation
3. A two-sentence sketch of the failure mode
Do NOT assign severity. Do NOT propose fixes yet.
```

The orchestrator collects every expert's candidates into the working set for this pass.

### Phase 2: Challenger (sequential per finding)

For each candidate, dispatch a *different* expert from the slate as challenger. The challenger is chosen by:

1. Prefer the expert whose lens is most likely to invalidate the finding (e.g., a network finding goes to the kubernetes-specialist for challenge; a contract finding goes to the security-specialist).
2. Never the same expert who surfaced the finding.
3. Rotate fairly across the slate so no one expert challenges all findings.

Challenger brief:

```
Another expert surfaced this finding:

  Title: <finding title>
  File:line: <citation>
  Failure mode: <sketch>

Try to refute it. Is this:
- Actually a bug? (confirm)
- Already mitigated upstream by <something>? (refute, name the mitigation)
- Out of scope for this target? (refute, name the boundary)
- Real but lower severity than it looks? (downgrade, propose new severity)

Write a one-paragraph verdict. If you confirm, also propose a severity per the
rubric and a one-line fix sketch.
```

Verdict outcomes:

- **Confirm** — finding advances to triage with the challenger's proposed severity.
- **Downgrade** — finding advances at reduced severity. Original severity recorded in state for audit.
- **Refute** — finding is dropped from the findings log. Recorded in `.bugbash/<target>.yaml` under `refuted:` with the reason, so the next pass doesn't re-surface it.

A challenger may not propose a *different* finding while challenging — they either resolve the current candidate or pass. Drift here weakens the convergence test.

### Phase 3: Triage and write

The orchestrator:

1. Walks each surviving finding (confirmed or downgraded).
2. Calibrates severity against `severity-rubric.md`. The challenger's proposed severity is a starting point; the orchestrator adjusts if the rubric suggests otherwise. If finder and challenger disagree on severity by more than one tier, the orchestrator picks the higher one (when in doubt, lean blocker — easier to downgrade later than to ship a missed Critical).
3. Drafts the entry per `format-spec.md` and appends to `docs/bugbash/<target>.md` with the next sequential item number.
4. Updates `.bugbash/<target>.yaml`:
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
Read docs/bugbash/<target>.md in full. Given everything captured, post a
launch verdict for this target.

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

The skill is long-running by design — multi-session for a non-trivial target is expected. State lives in `.bugbash/<target>.yaml`. See SKILL.md for the shape.

Two important state invariants:

1. **The findings log is append-only across sessions.** Item numbers stay stable. Resuming a run never reorders or renumbers.
2. **The expert slate is fixed for a run.** Adding or removing an expert mid-run invalidates the convergence test (a new expert would surface findings the others missed but didn't reset the counter on). If the slate is wrong, archive the run and start over.

## Comparison to RALPHY

| RALPHY | Bugbash |
|--------|---------|
| Fresh agent per iteration | Fresh expert dispatch per pass |
| `progress.txt` accumulates learnings | `docs/bugbash/<target>.md` accumulates findings |
| `prd.json` tracks story completion | `.bugbash/<target>.yaml` tracks pass count + convergence |
| Exits on `<promise>COMPLETE</promise>` when all stories pass | Exits on convergence + launch verdict |
| Loop produces working code | Loop produces a hardening report |

Both treat the loop body as the unit of work; both rely on persistent state so each iteration starts with a clean context but full visibility into prior work.
