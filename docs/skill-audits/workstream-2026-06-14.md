# Skill audit — `/workstream` (2026-06-14)

Shape: procedural + discipline spine. Read-only audit (PLT-534). Refactor lands via PR-with-council.

## Verdict
Mechanically clean (static: all pass; 162-line SKILL.md, substantive guardrails, rationalization table, evals). The `/goal` discipline spine is the skill's strongest dimension and **held under pressure**. The hardening gap is concentrated in the **review-to-ship gates**: four field-tested lessons are MISSING and all block-severity, clustered at two underspecified trigger definitions.

## Findings

| ID | Sev | Finding | Seam |
|---|---|---|---|
| **F1** | block | **Confirmed-consensus not required.** `design-approval` trigger ("captured AND cross-reviewed") is satisfied by a *single* cross-review pass — no requirement that findings be resolved-AND-re-confirmed by their raiser, or that the loop iterate to converged consensus. | `design-approval` |
| **F2** | block | **Slate-completeness not enforced.** workstream composes `/cross-review` but never constrains the slate; a *domain-only* slate (no prose/idiom axes) satisfies "cross-reviewed" — the exact config that let verbose comments through. | `design-approval` |
| **F3** | block | **No doc-alignment gate.** Nothing requires the diff reconcile with the approved `/design` doc or the **interface-registry** (the repo's stated source of truth). `pr-sign-off` = "CI green," which says nothing about design/registry fidelity. *Highest Tide-specific risk.* | `pr-sign-off` |
| **F4** | block | **Automated review not co-equal.** `pr-sign-off` names only CI; Cursor Bugbot / Claude review is absent — not required to be iterated (High = blocking) before the gate. | `pr-sign-off` |
| **F5** | warn | **`/goal` spine — strong, two residual loopholes.** (a) shipped as prose; hook-wiring deferred → add an eval acceptance criterion (operator-offline + Stop-hook scenario). (b) guard→rollback handoff doesn't restate the no-self-execute / human-token rule. | spine |
| **P1-loophole** | warn | **Reversibility framing is reversible.** "Reversibility is the axis" could be turned around to argue a *declared* gate is waivable for a reversible change — state that reversibility raises the bar to *create* a checkpoint but never lowers it to *waive a declared one*. | guardrails |
| **D6** | warn | Description smuggles a workflow summary (Obra CSO trap) — trim the "Scaffolds/Composes + phase chain" to a when-clause. | description |
| **B6** | info | Term drift: "Verify" used as the phase label vs the "cross-review" operation. | — |

Pressure tests corroborate: P1 (self-approval) held; P2 (domain-only single-pass) confirmed F1+F2+F4; P3 (doc-divergence) confirmed F3 (one-way-door subclass covered, general design-fidelity gate missing).

## Scoped hardening (the fix is two trigger redefinitions + spine clarifications)
- **`design-approval` trigger** → "cross-reviewed" means: slate covered all axes (domain + idiomatic + prose) or operator waived a named axis with rationale (F2); AND every finding resolved-and-re-confirmed by its raiser or explicitly accepted (converged consensus, not single-pass) (F1).
- **`pr-sign-off` trigger** → "PR ready" means: CI green AND automated-review (Bugbot/Claude) findings iterated, High = blocking (F4); AND the diff reconciles with the approved `/design` + interface-registry, divergence surfaced as gate evidence (F3; optionally compose `/verify`).
- **Spine** → reversibility-can't-waive-a-declared-gate (P1); guard never self-executes rollback (F5b); eval criterion for the deferred hook-wiring (F5a).
- **Description** → trim the workflow summary (D6).

These mirror the `/cross-review` findings (slate-completeness, confirmed-consensus, doc-divergence) — harden both together, via PR-with-council.
