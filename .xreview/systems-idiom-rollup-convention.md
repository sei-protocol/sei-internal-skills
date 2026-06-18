# Cross-review ledger — /systems ↔ /idiomatic comment-discipline cross-reference (rollup convention)

Target:       branch framework/systems-idiom-rollup-convention (.claude/skills/systems/SKILL.md, references/safety-quality.md, evals/evals.json; .claude/skills/idiomatic/references/language-pack-TEMPLATE.md); PR from this branch
Class:        skill-package (cross-skill convention fix)
Tier:         T2 (Component — a wiring-convention fix across /systems + the /idiomatic TEMPLATE)
Scope:        The language packs roll up into BOTH standards-champion agents (idiomatic-reviewer + systems-engineer) generically by detection — so Python (and every pack) auto-loads into both. The one per-language hardcode was `/systems`'s tombstone carve-out, which cited the idiom comment-discipline dimension as a stale "Go D10 / Rust R11" — incomplete for every pack added after Rust (TypeScript T9, Solidity S11, Bash D12, Python P10). This generalizes that cross-reference to "the active language pack's comment-discipline dimension", centralizes the complete per-language map in one canonical place, and adds a cross-skill upkeep note to the TEMPLATE so future packs don't re-drift. (Surfaced while verifying the Python pack #191 was wired per convention.)
Dissenter:    audit-skill (ID-map fidelity — the dominant risk is a WRONG dimension id in the new map, the same drift class being closed)

## Round 1
State:        RESOLVED
OpenFindings: 0 (all findings non-blocking; folded in as finalization)
Convergence:  unanimous

| Lens | Verdict | Finding |
|---|---|---|
| audit-skill (assigned dissenter, ID-map fidelity) | RATIFY | **Independently resolved all 6 ids against each pack's §1 table** — Go `D10`, Rust `R11`, TypeScript `T9`, Solidity `S11`, Bash `D12`, Python `P10` all correct; no stale "Go D10/Rust R11"-only hardcode survives; all surfaces consistent; evals.json valid; rollup into both champions confirmed generic; the only other id-literal (`sources.md` `P10`) is the NASA Power-of-Ten paper URL (correctly identified false positive). |
| author-skill | RATIFY | carve-out semantics preserved (systems still flags ONLY the tombstone, by citing not re-deriving; general comment idiom stays /idiomatic); the eval boundary intact; the TEMPLATE cross-skill note sound + correctly placed; future-proof (a 7th pack needs no SKILL.md edit). [low, non-blocking]: the canonical roster is a manually-curated illustration that could go stale-by-omission → documented an upkeep step. |
| prose-steward | RATIFY | dual-aligned; single-source-of-truth (canonical map in one place, four pointers auto-correct) is exactly what kills the staleness class; all 6 literals backticked + verified; the five-spot repetition is correct R3 anchored-locally redundancy. [style, optional]: a minor lead-in verb variance ("idiom" vs "idiomatic pack's"). |

### Verdict
RESOLVED — unanimous RATIFY round 1; zero blocking findings. The assigned dissenter (audit-skill) re-verified every per-language comment-discipline id against the live pack tables — the dominant risk (a wrong id re-introducing the drift) is clean. Non-blocking polish folded in as finalization: removed a "comment-discipline dimension (… dimension …)" phrase-repetition my generalization introduced (tightened the 5 SKILL.md + 2 evals parentheticals to "its per-language id — e.g. Go `D10`, Python `P10`; full map in safety-quality.md"); added a roster-upkeep step to the TEMPLATE cross-skill note (author [low]).

**Verified rollup completeness:** both idiomatic-reviewer ("detect language → load `language-pack-<lang>.md`") and systems-engineer ("the language-idiom layer is owned by /idiomatic and its language packs; load the relevant pack") load packs generically — Python and every present/future pack auto-rolls-up into both. The tombstone carve-out was the sole per-language hardcode; it is now generic + complete + future-proofed (the canonical map in `safety-quality.md` + the TEMPLATE upkeep note). Cursor Bugbot + CI: pending on PR open (Bugbot skip/NEUTRAL satisfies the check half for this doc/skill PR per the recorded review-gate policy).
