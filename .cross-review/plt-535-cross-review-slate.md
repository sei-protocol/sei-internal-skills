# Cross-review ledger — PLT-535 /cross-review slate routing + review ledger

Target:       PR #164 / branch plt-535-cross-review-slate (.claude/skills/cross-review/**, .claude/skills/coral/SKILL.md; design of record: bdchatham-designs Design 08)
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
- Slate: audit-skill (steward), author-skill (steward), prose-steward (steward, agent), systems-engineer (domain — control-system/gate), technical-program-manager (assigned dissenter)
- Auto-wired stewards: audit+author+prose — skill-package change, unconditional pin (§4). audit-skill/author-skill resolved from `.claude/skills/`; prose-steward from `.claude/agents/`.
- Overrides: none

### Per-lens verdicts
| Lens | Verdict | Finding (evidence-bearing) | Resolution |
|---|---|---|---|
| audit-skill | RATIFY | static 22/22; four axes (gating-set token-identity, steward pin unconditional-vs-file-type, tier floor-vs-override, gate-read fields) zero-contradiction | n/a |
| author-skill | RATIFY | R1 (Step 5 DISSENT-vs-advisory bridge) + R2 (halt-open-steward-dissent eval) closed; 6/6 RED probes pass | n/a |
| prose-steward | RATIFY | P1 (gating-set "stated identically" wording) + P2 (gate-read field order) closed; no correctness-grade prose findings | n/a |
| systems-engineer | RATIFY | gate-read total over input domain; round-selection fail-closed (S1) + single-writer assumption (S2) closed; split→accepted-risk laundering closed mechanically | n/a |
| technical-program-manager | DISSENT | A (correctness): Design 08 gate table failed Convergence only on absent/unparseable, skill fails closed on out-of-enum — divergent gates. B (correctness): Design 08 schema carried flat top-level Round: line, skill uses per-round header blocks + round-selection-fails-closed that the contradictory-ledger eval depends on. | resolved @ Design 08 R4 (bdchatham-designs 8a7d418) — see Round 2 |

### Boundary findings
| Interface / Boundary | Provider | Consumer | Status | Evidence | Raised by |
|---|---|---|---|---|---|
| review-ledger gate-read schema ↔ Design 08 gate table | review-ledger.md (skill) | Design 08 (of-record contract) | MISMATCH | Convergence out-of-enum handling (A); per-round-header model (B) | technical-program-manager |
| slate-routing steward registry ↔ SKILL.md HALT rule | slate-routing.md §4 | SKILL.md Guardrails/Halt | COMPATIBLE | two-kinds dispatch stated identically across all sites | audit-skill |

### Prose addendum (skill is prose — correctness-grade blocks, style advisory)
- P1, P2 (above) — both style severity, applied; no correctness-grade prose finding.

### Rejected findings
| Finding (as raised) | Raised by | Why rejected, and how verified |
|---|---|---|
| (none) | — | all raised findings were accepted and resolved |

### Verdict
OPEN — 2 correctness-grade findings (A, B), both design↔skill divergences where the as-built skill outran Design 08. Convergence split (TPM dissent). Closed in Round 2 after the comprehensive Design 08 contract-surface reconciliation.

## Round 2
Round:        2
State:        RESOLVED
OpenFindings: 0
Convergence:  unanimous
Blinded:      yes
Dissenter:    technical-program-manager

### Routing
- Slate: technical-program-manager (assigned dissenter, re-verify) — the four RATIFY lenses' surfaces were unchanged except the additive round-selection sequence-invariant + concurrency cross-ref the systems-engineer itself requested; only the dissenter's findings drove new edits, so the re-verification round re-dispatched the dissenter against the reconciled Design 08 + skill.
- Auto-wired stewards: unchanged (skill-package pin).
- Overrides: none

### Per-lens verdicts
| Lens | Verdict | Finding (evidence-bearing) | Resolution |
|---|---|---|---|
| technical-program-manager | RATIFY | A closed: Design 08 gate table Convergence row + cross-field row + fails-closed paragraph now byte-for-contract identical to review-ledger.md. B closed: Design 08 schema now per-round header blocks (Target/Class/Tier once at top; five round-scoped fields per ## Round N) + round-selection-fails-closed + single-writer assumption. C closed: strict < total order. No new drift. | design↔skill byte-for-contract consistent (Design 08 8a7d418 ↔ skill 5420df3) |

### Boundary findings
| Interface / Boundary | Provider | Consumer | Status | Evidence | Raised by |
|---|---|---|---|---|---|
| review-ledger gate-read schema ↔ Design 08 gate table | review-ledger.md (skill) | Design 08 (of-record contract) | COMPATIBLE | A/B reconciled; gate row set, enum tokens, schema shape, fail-closed conditions all consistent | technical-program-manager |

### Verdict
RESOLVED — all correctness-grade findings closed and verified by the assigned dissenter. Slate unanimous (audit-skill, author-skill, prose-steward, systems-engineer, technical-program-manager all RATIFY); a dissenter was assigned every round and concluded RATIFY only after the findings were closed. First-hand RED tests confirmed the two load-bearing behaviors (skill does not HALT on its own skill-package dogfood; an open steward DISSENT blocks RESOLVED under "just being picky" pressure). Cursor Bugbot: clean. Confidence high (blinded, independent, assigned-dissent-to-convergence).
