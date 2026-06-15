# Cross-review ledger — PLT-530 bets → Linear projects

Target:       PR #166 / branch plt-530-bet-to-project (.claude/skills/execution-plan/**, .claude/agents/technical-program-manager.md; design of record: bdchatham-designs Design 10)
Class:        skill-package
Tier:         T3

## Round 1
Round:        1
State:        OPEN
OpenFindings: 2
Convergence:  split
Blinded:      yes
Dissenter:    technical-program-manager

### Routing
- Slate: audit-skill (steward), author-skill (steward), prose-steward (steward, agent), product-manager (domain — scope coherence of the rewrite's boundaries), technical-program-manager (assigned dissenter — does the Guardrail-1 flip reintroduce the container-as-identity corruption?)
- Auto-wired stewards: audit+author+prose — skill-package change, unconditional pin.
- Overrides: none. Note: the *model* (bet→project, single-bet-per-issue, no labels) is an operator-approved decision (the reserved crux); the slate reviewed the IMPLEMENTATION's safety/coherence, not the decision.

### Per-lens verdicts
| Lens | Verdict | Finding (evidence-bearing) | Resolution |
|---|---|---|---|
| audit-skill | RATIFY | static 22/22; write-classes, no-label-identity, eval↔guardrail mapping all zero-contradiction (1 info note: SKILL issue-field list omitted designLinked) | info note applied @ R2 |
| author-skill | RATIFY | 5/5 RED probes hold (refuse-bet-label, single-bet-conflict, container-as-identity, multi-bet, project-drift); 2 nits | applied @ R2 |
| prose-steward | RATIFY | identity-vs-alias, stamp write-class split, single-bet hardness, multi-bet decisiveness all multiply-anchored, no soft-modal drift | n/a |
| product-manager | RATIFY | scope boundaries minimal + coherent (consumer-migration burn-down sound; legacy-label deferral clean; single-bet workaround workable); 2 nits | applied @ R2 |
| technical-program-manager | DISSENT | F1 (BLOCKER, double-count): designLinkedNotInProject didn't exclude issues already in another bet's project → an issue in bet B's project linking A's design counted under both. F2 (BLOCKER, field-name fork): Design 10 `issuesLinkingDesignNotInProject` vs skill `designLinkedNotInProject`. F3 (minor): Design 10 dropped projectStatus. **Confirmed the load-bearing safety question SOUND: the Guardrail-1 flip does NOT reintroduce the container-as-identity corruption** (page-ID cache is the join everywhere; drift human-resolved). | resolved @ R2 |

### Verdict
OPEN — 2 correctness-grade blockers (double-count hole; field-name fork) + advisories. The load-bearing safety property (no container-as-identity corruption) verified sound by the assigned dissenter. Closed in Round 2.

## Round 2
Round:        2
State:        RESOLVED
OpenFindings: 0
Convergence:  unanimous
Blinded:      yes
Dissenter:    technical-program-manager

### Routing
- Slate (re-verify): technical-program-manager (block owner — F1/F2/F3), audit-skill (the betGraph field-set/semantics change is its domain). prose/author/product-manager RATIFIED in R1 with advisories applied verbatim.
- Overrides: none

### Per-lens verdicts
| Lens | Verdict | Finding (evidence-bearing) | Resolution |
|---|---|---|---|
| technical-program-manager | RATIFY | F1 closed: designLinkedNotInProject now excludes issues already in another bet's project (the double-count is mechanically impossible — inclusion predicate + exclusion clause partition cleanly); pinned by a new eval. F2 closed: field name uniform (designLinkedNotInProject) across Design 10/SKILL/reference/evals. F3 closed: all four native rollup fields present everywhere. No new contradiction. | n/a |
| audit-skill | RATIFY | betGraph field set single-valued across SKILL ↔ reference; exclusion clause consistent; Op2 AUTO-link-on-conflict closes the loop with Guardrail 4 + the exclusion (no contradiction); both new/retyped evals well-formed; static green | n/a |

### Verdict
RESOLVED — both blockers closed and verified by the assigned dissenter; field set/name reconciled; the double-count hole made mechanically impossible. Slate unanimous: audit-skill, author-skill, prose-steward, product-manager, technical-program-manager (assigned dissent) all RATIFY. A dissenter was assigned every round and concluded RATIFY only after the blockers were closed. The Guardrail-1 flip preserves the page-ID-identity property the old guardrail protected (project is the alias, never the identity). Cursor Bugbot: clean. Confidence high (blinded, independent, assigned-dissent-to-convergence). Design 10 is the design of record. The operator-approved model (bet→project, single-bet-per-issue, no bet labels) is the reserved-decision basis; consumer deep migration + legacy-label strip are the kit's named follow-up burn-down.
