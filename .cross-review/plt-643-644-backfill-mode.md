# Cross-review ledger — PLT-643 / PLT-644 impact-weekly historical-backfill mode

Target:       branch framework/impact-weekly-backfill-mode (.claude/skills/impact-weekly/**, .claude/skills/author-skill/references/eval-format.md, .claude/skills/execution-plan/references/graph-and-decoration.md); PR opened from this branch
Class:        skill-package
Tier:         T3
Scope:        PLT-643 (pin the `/impact-weekly` Status-line rule) + PLT-644 (capture the artifact-backfill methodology as a reusable `/impact-weekly` mode). Combined into one coherent surface — the Status line only exists inside the backfill 4-part toggle — and shipped as `references/backfill-mode.md` + a SKILL.md section + evals. Synthesized from the 2026-06 impact-backfill experience.

## Round 1
Round:        1
State:        OPEN
OpenFindings: 2 (blocking) + several (major)
Convergence:  split
Blinded:      no (re-review of a single author's draft; orchestrator finalized)

### Routing
- Slate (skill-package, slate-routing §4): prose-steward (steward), product-manager (scope/YAGNI steward), audit-skill + author-skill (stewards, dispatched as **skills loaded by general-purpose agents** — they are skills, not agent types), technical-program-manager (domain — attribution/lineage).
- **Assigned dissenter: technical-program-manager** (formally designated up front — closes the PLT-535 assigned-dissent gap logged on the #169 run).

### Per-lens verdicts
| Lens | Verdict | Finding (evidence-bearing) | Resolution |
|---|---|---|---|
| technical-program-manager (assigned dissenter) | REVISE | **blocking**: histogram said "a real Linear histogram" → latent reimplement; must compose `/execution-plan` `betGraph(...).rollup.byStatus`, never a hand-rolled `list_issues`/label count. **blocking**: contemporaneous test undefined for a mixed week (some contemporaneous + some backfilled-today). **major**: net-zero force-rollout exception had no substantiation/single-bet guard (laundering path). **major**: cite-or-cut conflicted with `mapping-and-coverage.md` (PR link config-contingent). **major**: live re-run could overwrite a backfilled PR-only toggle. + 2 minors (Lineage PLT-530 wording; merge-date vs completion-week placement). | applied @ R2 |
| product-manager | REVISE | **major**: "Idempotency + confirm — unchanged" section was pure ceremony restating `write-contract.md`. **major**: "Lineage line" section was a 3rd restatement of routing→Lineage. + minor (SKILL.md section re-stated the whole reference) + nits. | applied @ R2 |
| prose-steward | REVISE | minor: buried lead; net-zero exception buried in a parenthetical; template `*Status:*` used a forward-ref to the literals. + nits. | applied @ R2 |
| author-skill | REVISE | **major**: no eval for the force-rollout exception. **major**: no backfill-specific render/verify eval. + minors (no positive outcome-verb headline eval; no backfill halt-condition eval) + nit (`discipline` type undocumented). | applied @ R2 |
| audit-skill | RATIFY | 2 nits (force-rollout not eval-covered; sub-agent-write failure mode not in a Rationalization row). | folded @ R2 |

### Verdict
OPEN — two verified blocking findings (compose-betGraph; mixed-week test) + ceremony/restatement majors + eval-coverage majors. All applied before Round 2.

## Round 2
Round:        2
State:        OPEN
OpenFindings: 1 (blocking) + 1 (major)
Convergence:  split
Blinded:      no

### Routing
- Full slate re-dispatch on the round-1 fixes; same assigned dissenter (technical-program-manager).

### Per-lens verdicts
| Lens | Verdict | Finding (evidence-bearing) | Resolution |
|---|---|---|---|
| technical-program-manager (assigned dissenter) | REVISE | **blocking**: the all-or-nothing test gated on `issue.createdAt`, but the documented `betGraph` `plan.issues[]` projection returned `completedAt` and **not** `createdAt` — applying the test forced a raw `list_issues` read, the exact hand-rolled path the other blocking fix forbids. The two fixes contradicted at the field level. + minor: `betGraph` window documented as single lower bound (`updatedAt ≥ start`) vs SKILL.md's "both ends bounded" claim. | applied @ R3 |
| product-manager | REVISE | **major**: "routed-out / net-zero → Lineage, never Status" was stated 3× (template comment + status rule + Lineage section) — the round-1 restatement defect relocated, not eliminated. + minors (opening re-enumerated guardrails; verify-render instruction restated). | applied @ R3 |
| prose-steward | RATIFY | round-1 prose findings resolved; 2 nits (template trailing "see the rule below" forward-ref; a competing mid-run bold in the opening). | applied @ R3 |
| audit-skill | RATIFY | all paths resolve; the `discipline` eval type now documented in `eval-format.md`; force-rollout exception now eval-covered; corpus eval-type vocabulary consistent. | n/a |
| author-skill | RATIFY | all round-1 eval majors closed (force-rollout, render/verify, positive headline, backfill halt-condition); 1 nit (merge-date placement rule lacked a dedicated eval assertion). | applied @ R3 |

### Verdict
OPEN — one verified blocking field-level contradiction (`createdAt` not in the `betGraph` projection) + the relocated 3×-restatement major + the window-bound minor + the merge-date eval nit. Fixed before Round 3 by extending the shared `betGraph` projection (`graph-and-decoration.md`: `plan.issues[]` gains `createdAt`; window bounded on both ends) so the contemporaneous test composes over betGraph instead of reaching around it, and collapsing the restatement to one authoritative home.

## Round 3
Round:        3
State:        RESOLVED
OpenFindings: 0
Convergence:  unanimous
Blinded:      no

### Routing
- Full slate re-dispatch (convergence-confirming round) on the round-2 fixes — including the cross-skill `graph-and-decoration.md` change. Assigned dissenter: technical-program-manager.

### Per-lens verdicts
| Lens | Verdict | Finding | Resolution |
|---|---|---|---|
| technical-program-manager (assigned dissenter) | RATIFY | the `createdAt` contradiction is resolved — the all-or-nothing test now composes over `plan.issues[].createdAt`, a field the contract exposes; window bounded both ends; betGraph call shape matches; no attribution-chain regression. | n/a |
| product-manager | RATIFY | the 3× restatement is collapsed to one authoritative home (status rule) + a non-restating back-reference; guardrails de-enumerated; verify-render reduced to justification. | n/a |
| prose-steward | RATIFY | both nits resolved; the load-bearing lead still leads; round-3 edits introduced no new buried lead / forward-ref / ambiguous antecedent. | n/a |
| author-skill | RATIFY | the merge-date placement rule now has a faithful eval assertion (+ a paired forbidden signal folded in); four backfill evals meet the bar (discipline/happy coverage + a halt-condition where stopping is right). | n/a |
| audit-skill | RATIFY | the two `graph-and-decoration.md` edits are structurally sound; `issue.createdAt` + `rollup.byStatus` both resolve to defined contract fields; no contradiction with `execution-plan/SKILL.md`; all cross-refs resolve (no dead links); evals.json valid (13), eval-type vocabulary corpus-consistent. | n/a |

### Verdict
RESOLVED — all blocking + major findings closed and verified over three rounds; slate unanimous (technical-program-manager [assigned dissenter], product-manager, prose-steward, author-skill, audit-skill all RATIFY; zero open findings). The orchestrator (not a reviewer) applied every fix — the author→review→author-finalize contract, applied to itself. The assigned dissenter (TPM) drove two blocking findings across R1→R2 (compose-betGraph, then the createdAt field-level contradiction the first fix exposed) to resolution. Cursor Bugbot + CI: pending on PR open (the review-gate's check half — see PR / workstream review-gate).
