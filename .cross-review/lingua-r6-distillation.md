# Cross-review ledger — /lingua R6 (distill for the human; layer for the agent)

Target:       branch `skill/lingua-r6-distillation` (.claude/skills/lingua/references/audience-model.md, SKILL.md, evals/evals.json; lingua packs + exemplars + README.md + cross-review/SKILL.md for the R1–R5→R1–R6 rename); PR from this branch
Class:        skill-package (a doctrine rule added to the /lingua audience model)
Tier:         T1–T2 (Atom/Component) — a focused doctrine addition to one skill; no one-way door
Scope:        Add rule **R6 — "Distill for the human; layer for the agent"**: succinct technical distillation as a human-facing force multiplier (the executive-summary intent), framed as the volume-axis asymmetry R3 names, reconciled by progressive disclosure (distilled lead → composes R2; full detail layered beneath for the agent), with a fidelity bound (R3/R4 outrank R6 — never distill away a constraint). Grounded in online /research; the rule set renamed R1–R5 → R1–R6 across the corpus.
Dissenter:    fidelity (assigned) — re-verify every NEW citation against its primary source online (BLUF/AR 25-50, Minto, Purdue OWL, Grice/SEP, NN/g, USC); the dominant risk for a citation-bearing doctrine change is a fabricated/over-claimed source.

## Round 1
State:        REVISE
OpenFindings: resolved in Round 2 (1 fidelity DISSENT [blocking], 1 audit REVISE [blocking], 1 prose non-blocking)
Convergence:  split

| Lens | Verdict | Finding |
|---|---|---|
| fidelity (assigned dissenter — verifies citations online) | DISSENT | **5 of 6 citations VERIFIED** verbatim/faithful against primary sources (AR 25-50/BLUF exact, with the unverified section correctly omitted; Minto cite-and-link; Purdue OWL; Grice/SEP exact; NN/g). **1 REFUTED [blocking]:** the R6 tier line attributed *"does not replace the full document"* to USC Libraries as a quote — USC does **not** say it (the page leans the other way: the summary "can stand alone"). A fabricated attributed quote, propagated from the research sweep's paraphrase-as-quote. No license violation; the four-domain convergence claim is defensible. |
| consistency-audit | REVISE | 9 of 10 checks pass (rename complete — zero stray `R1–R5`; Rust namespace untouched; JSON valid; cite-lint clean; numbering unique; markdown intact). **1 [blocking]:** `SKILL.md:94` still read "the **five** dual-aligned rules R1–R6" — the rename updated the range but missed the stale word "five", contradicting the corpus-wide "six". |
| prose-steward (doctrine owner of record) | RATIFY | R6 fits the dual-aligned frame (resolves to dual-alignment via layering, same family as R1/R5); basis-tier split correct (practice **Cited** via four-domain convergence + Grice root; the agent-benefits-from-noise contrast **Stated-opinion** inheriting R3, with a falsification line; nothing opinion dressed as citation); composition with R2 and the R3/R4 precedence correct; the R3-mandated-redundancy vs distill tension resolved (distill *altitude*, restate *constraints*); fidelity bound at least as tight as Guardrails 2/3; both evals non-vacuous. **[non-blocking]:** R6's "the one place the audiences diverge on volume" overstates — R3 already names that asymmetry; reframe to build on R3, not claim novelty. (Style: dense citation block — optional, matches ratified R4 density.) |

### Round 1 resolution (applied)
- **Fabricated USC quote (fidelity, blocking):** removed `"does not replace the full document"` and its USC attribution from R6 and from the eval that echoed it; USC now cited only for the decision-first, stand-alone executive-summary form (which it genuinely supports); the layer-not-replace counter-thesis stands as **this skill's own** fidelity bound, explicitly not attributed to USC.
- **Stale "five" (audit, blocking):** `SKILL.md:94` five → six.
- **R3 lineage (prose, non-blocking):** R6 reframed — "R6 turns the volume asymmetry R3 already names into a move" — dropping the false-uniqueness claim. (Prose style #2, the citation-block density, declined as optional — matches the already-ratified R4 density.)

## Round 2
State:        RESOLVED
OpenFindings: 0
Convergence:  unanimous

| Lens | Verdict | Finding |
|---|---|---|
| fidelity (assigned dissenter) | RATIFY | Fabrication fully removed (grep: zero `does not replace the full document` corpus-wide, incl. the eval); USC re-fetched and the remaining "decision-first, stand-alone overview" claim confirmed accurate; counter-thesis correctly owned by the skill; the other five citations undisturbed; no new citation/attributed quote introduced by the fix. **Dissent withdrawn.** |
| consistency-audit | RATIFY | All 7 checks PASS — `SKILL.md:94` now "six"; zero stray `R1–R5`; Rust namespace untouched; evals.json valid (9 unique ids); cite-lint exit 0 (every citation resolves); audience-model heading "six … (R1–R6)" with R1..R6 unique + Precedence referencing R6; markdown/JSON integrity intact. Round-1 REVISE resolved, no regressions. |
| prose-steward (doctrine owner) | RATIFY | (Round 1; the non-blocking R3-lineage fix was applied as suggested. No re-dispatch needed — the change since is the citation/rename fix, which is fidelity/audit territory.) |

### Verdict
RESOLVED — unanimous RATIFY in Round 2; zero open findings. The assigned fidelity dissenter verified every citation against its primary source online and caught the one genuinely serious defect — a fabricated quote attributed to USC, carried in from the research sweep — which is exactly the failure that lens exists to prevent in a doctrine that is itself about citation discipline. Fixed by re-scoping USC to what it supports and owning the counter-thesis as the skill's own fidelity bound. The doctrine owner (prose-steward) confirmed R6's logic, tier split, composition, and fidelity bound are sound; the audit confirmed the R1–R5→R1–R6 rename is complete and the Rust dimension namespace untouched.

**Mechanical gates:** cite-lint clean (37 citations resolve); evals.json valid (9 cases). Cursor Bugbot + CI evaluated on PR open (Bugbot skip/NEUTRAL satisfies the check half for this doc/skill PR per the recorded review-gate policy; failing/findings block).
