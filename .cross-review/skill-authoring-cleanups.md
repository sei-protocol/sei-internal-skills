# Cross-review ledger — skill-authoring cleanups (latent gaps from #169/#171 review)

Target:       branch framework/skill-authoring-cleanups (.claude/skills/audit-skill/references/pressure-testing-for-audit.md, .claude/skills/audit-skill/references/conventions-catalog.md, CLAUDE.md); PR opened from this branch
Class:        skill-package
Tier:         T2
Scope:        Two latent gaps surfaced during the #169/#171 reviews: (1) the `audit-skill` cross-skill `../` path bug in `references/pressure-testing-for-audit.md` (single `../` from inside `references/` under-resolves), and (2) the missing CLAUDE.md "Writing conventions (skill prose)" stanza capturing the durable authoring lessons (imperative voice, failure-consequence-after-each-rule, literals-in-backticks, the `../../` cross-skill link form).

## Round 1
Round:        1
State:        OPEN
OpenFindings: 1 (high) + 1 (high) + minors
Convergence:  split
Blinded:      no

### Routing
- Slate (skill-package, scoped to a small cleanup): audit-skill (reference integrity, dispatched as a skill via general-purpose), prose-steward (the CLAUDE.md stanza prose), product-manager (scope/YAGNI).
- **Assigned dissenter: product-manager** (formally designated up front).

### Per-lens verdicts
| Lens | Verdict | Finding (evidence-bearing) | Resolution |
|---|---|---|---|
| product-manager (assigned dissenter) | REVISE | **high**: the new CLAUDE.md bullet asserted `../../` while `audit-skill/references/conventions-catalog.md` R5 (and R1) documented the cross-skill form as single `../` — the *wrong* rule, and the **root cause** of the very bug being fixed. Shipping the right rule in CLAUDE.md while the auditor's catalog said the wrong one = a split source of truth (the auditor would bless broken links / flag correct ones). Must fix R5 in the same PR. **high**: the backtick clause lossily duplicated `notion-flavored-markdown.md` (demote to a pointer). + med (over-specified for a terse CLAUDE.md — trim) + low (drop the dual-audience parenthetical ceremony). | applied @ R2 |
| prose-steward | REVISE | minor: the stanza was one ~90-word run-on packing four rules + an orphan `/lingua` clause; the example list mixed backticked literals with bare category words; later rules lacked their failure-consequence. (Resolved by the trim-to-pointer direction, not sub-bullet expansion.) | applied @ R2 |
| audit-skill | RATIFY | both corrected `pressure-testing-for-audit.md` paths resolve; consistent with line 7; no remaining under-resolving refs in that file. (Did not flag R5 — scoped to the path fix; the assigned dissenter's broader scope caught the catalog root cause.) | n/a |

### Verdict
OPEN — the assigned dissenter surfaced the load-bearing root cause (the auditor's own conventions-catalog R1/R5 documented the wrong `../` depth) that the path fix alone left as a split source of truth. R1 + R5 corrected to `../../` (relative form), repo-root-absolute alternate retained; backtick mechanics demoted to a pointer; CLAUDE.md bullet trimmed.

## Round 2
Round:        2
State:        OPEN
OpenFindings: 1 (high) + 1 (low)
Convergence:  split
Blinded:      no

### Routing
- Full slate re-dispatch on the round-1 fixes (audit-skill now also reviews the new conventions-catalog R1/R5 edit). Assigned dissenter: product-manager.

### Per-lens verdicts
| Lens | Verdict | Finding (evidence-bearing) | Resolution |
|---|---|---|---|
| product-manager (assigned dissenter) | REVISE | **high**: the round-1 fix demoted the *backtick* mechanics to a pointer but left the full `../../` *rationale* inline in CLAUDE.md — now a verbatim duplicate of conventions-catalog R5's parenthetical, re-opening the split source of truth *from the other direction* (the same rationale in two governed files will drift). Apply the same remedy: demote to a cue + pointer. + low: the backtick pointer path was repo-relative (`impact-weekly/references/...`) while the section uses `.claude/skills/...`-rooted paths. | applied @ R3 |
| audit-skill | RATIFY | R1/R5 now state the correct `../../` depth, mutually consistent; all three sources (catalog, live links, CLAUDE.md) agree; both pressure-testing paths resolve; diff is exactly the intended files. | n/a |
| prose-steward | RATIFY | the trim cleared all round-1 prose findings (run-on gone, category/literal mix gone, the `../` rule carries its consequence inline, `/lingua` clause is a clean terminal pointer); the bullet practices its own backticking. | (the inline consequence it praised was relocated @ R3 to close the PM's duplication finding — re-verified RATIFY) |

### Verdict
OPEN — one verified high (rationale duplicated from the other direction) + a low path-consistency nit. Fixed by demoting the CLAUDE.md cross-skill clause to a terse "note the double `../`" cue + a pointer to R5 (the single home), and rooting both pointer paths at `.claude/skills/...`.

## Round 3
Round:        3
State:        RESOLVED
OpenFindings: 0
Convergence:  unanimous
Blinded:      no

### Routing
- Full slate re-dispatch (convergence-confirming). Assigned dissenter: product-manager.

### Per-lens verdicts
| Lens | Verdict | Finding | Resolution |
|---|---|---|---|
| product-manager (assigned dissenter) | RATIFY | the verbatim-rationale duplication is closed (R5 is the single home; CLAUDE.md cues + points); paths section-consistent; bullet terse. One optional [low] no-fix mnemonic-echo note. | n/a |
| prose-steward | RATIFY | the local "note the double `../`" cue preserves the agent reader's *action* inline while the *rationale* delegates correctly to R5 (the correct action/explanation tiering under D2 centralization); no regression. | n/a |
| audit-skill | RATIFY | both CLAUDE.md pointer paths resolve; the cited R5 exists and is the cross-skill-reference rule (accurate pointer, not dangling); no regression in R1/R5/pressure-testing. | n/a |

### Verdict
RESOLVED — all high + minor findings closed and verified over three rounds; slate unanimous (product-manager [assigned dissenter], prose-steward, audit-skill all RATIFY; zero open findings). The orchestrator (not a reviewer) applied every fix. The assigned dissenter (product-manager) drove the substance: R1 surfaced the root-cause catalog rule that the path fix alone would have left as a split source of truth; R2 caught the same duplication re-introduced from the other direction. Cursor Bugbot + CI: pending on PR open (Bugbot skip/NEUTRAL satisfies the check half for this doc-only skill PR per the recorded review-gate policy).
