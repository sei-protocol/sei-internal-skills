---
skill: cross-review
shape: procedural (with a discipline spine)
audited_on: 2026-06-14
auditor: bdchatham
audit_skill_version: 1
phase: audit-only
trigger: semantic-conventions pass after the slate-routing / review-ledger / classify-before-dispatch change (commits 74a29b7 + f967019, branch plt-535-cross-review-slate), incl. the /coral cite edit
---

# Skill Audit — cross-review (semantic conventions pass)

**Shape:** procedural with a discipline spine (classify-before-dispatch HALT gate + rationalization/red-flags stack)
**Audited:** 2026-06-14 by bdchatham
**Phase:** audit-only (judgment-based semantic pass; static checks already pass 22/22)
**Scope:** the PLT-535 change — `cross-review/SKILL.md` (§0 HALT gate + Step 2 routing), the two new references `slate-routing.md` and `review-ledger.md`, the extended `evals/evals.json`, and the `/coral` edit that now *cites* the shared `slate-routing.md`. The question this pass answers: do the conventions hold with no drift introduced by extracting the shared routing table into `cross-review/references/` while `/coral` cites it cross-skill?

## Summary

- **Block:**   0
- **Warn:**    1
- **Info:**    1
- **Skipped:** 0

The extraction is conventions-clean. The shared routing table lives single-sourced in `cross-review/references/slate-routing.md`, declares itself the source of truth ("If a skill's prose ever diverges from this table, the table wins"), and `/coral` **cites it by relative path without duplicating the taxonomy or the tier map** — exactly the R5 cross-skill-documentation-link pattern (info-only), not a second router and not an `@skills` force-load. The three drift-prone facts that *could* have diverged — the six-class taxonomy, the class→tier map, and the unconditional `audit+author+prose` skill-package pin — are stated once in the table and only *referenced* (by §-number) from SKILL.md and coral, so there is no contradicting second copy. The SKILL.md ↔ slate-routing.md ↔ review-ledger.md triad is internally consistent: §0's "Tier read off the table, never re-derived" matches the table's authoritative class→tier map (§3) and the table's "How each skill applies this" guidance; the ledger's typed-header contract (`State:`/`OpenFindings:`/`Convergence:`/`Blinded:`/`Dissenter:` one-per-line, exact-token) is consistent everywhere it is referenced (SKILL.md Step 4/5, the evals). **One drift finding (warn):** `/coral`'s intro line still asserts "No tier selection," which now contradicts its own edited Core-Loop step (and the table's coral-application guidance) that has coral read the tier off the shared table. **One info:** the two new >100-line references carry no explicit Table-of-Contents list.

## Block findings

None.

## Warn findings

> Severity: **warn**. Address before broad rollout.

### B6 (one consistent concept) / cross-skill consistency — coral says "No tier selection" but the edit makes coral read tier off the shared table

- **Rule:** B6 (body uses one consistent term per concept; no drift) applied across the two skills sharing the table; this is the cross-skill-consistency axis the pass is asked to guard.
- **Source:** semantic.
- **File / line:** `.claude/skills/coral/SKILL.md:10` vs `:23` (and `:29`).
- **Evidence:** Line 10 (unchanged by this PR) frames coral as lightweight: *"No tier selection, no cross-review rounds, no workstream file."* The PLT-535 edit to line 23 now has coral *"route off the shared table … both read the same change-type × **tier** table"*, and line 29 keys the standards-stewards to the table's §4 wiring — which the table itself (`slate-routing.md` §"How each skill applies this table", lines 124-127) describes as coral *"read[ing] the tier off §3."* So coral now demonstrably **does** consult tier, while its own intro asserts it does not. A reader (or an agent) reconciling the two lines sees a contradiction: is coral tier-aware or not?
- **Why it is only `warn`, not `block`:** the substance is coherent under a charitable read — line 10's "No tier selection" was written to contrast coral against `/council`'s scope-*tier ceremony* (the heavyweight Product/System/Component/Feature gate), not to deny that coral reads a depth dial. The new behavior is real and intended (a `shared-stack`/`skill-package` slice should pull stewards in production too — the whole point of the shared table). Nothing routes wrong; only the intro's wording is now stale against the body. No eval depends on the "No tier selection" phrasing.
- **Recommendation:** Reword `coral/SKILL.md:10` to disambiguate the depth dial from council's ceremony — e.g. *"No scope-tier **ceremony**, no cross-review rounds, no workstream file"* (coral still reads the routing table's depth dial; what it skips is council's formal scope-tier gate). One-line change; do not touch line 23/29.

## Info findings

> Severity: **info**. Observations worth surfacing; not a quality bar.

### R2 — the two new >100-line references have no explicit Table of Contents

- **Rule:** R2 (warn in the catalog; surfaced here as info because both files are right at/just past the threshold and well-sectioned).
- **File / line:** `references/slate-routing.md` (127 lines) and `references/review-ledger.md` (166 lines).
- **Observation:** Both exceed the 100-line R2 trigger but use numbered/`##` section headings only — no leading TOC list in the first 50 lines. They are highly scannable (slate-routing is §1–§6 + "How each skill applies"; review-ledger is a clear section sequence), so navigability is fine in practice, and the repo's other references follow the same heading-only style. Static checks scored these PASS, so the project's R2 heuristic (heading scan in first 50 lines) is satisfied by the section headings.
- **Recommendation (optional):** if the team wants strict R2 conformance for references past 150 lines, add a 6-line TOC to `review-ledger.md`. Not required; noted for consistency only.

## Conventions explicitly re-verified (held after the change)

| Rule | Check | Result |
|---|---|---|
| B1 | SKILL.md ≤ 500 lines | PASS — 199 lines |
| D2 | description < 1024 chars | PASS — cross-review 957, coral 720 |
| D1/D5 | description third-person, "Use when…" | PASS (both) |
| D3/D4 | anti-triggers + sibling redirects present | PASS — cross-review redirects to coral/council/bugbash/code-review/design/root-cause; description still accurate to behavior (the §0 HALT gate is a guardrail, not a workflow summary, so D6 holds) |
| D6 | description routes triggers, does not summarize the body's workflow (Obra CSO trap) | PASS — description names *when*, not the §0→Step1–5 *how* |
| R1 | references one level deep | PASS — all 5 files directly under `references/`; no `references/sub/` |
| R5 | cross-skill ref is a documentation link, not a force-load | PASS — coral cites `.claude/skills/cross-review/references/slate-routing.md` as plain markdown; this is the sanctioned handoff/shared-methodology link (info-only), not `@skills/...` |
| R3/A3 | no `@skills/...` force-loads | PASS — none in any changed file |
| R4 | references extend, do not duplicate, SKILL.md; internally consistent | PASS — slate-routing carries the full taxonomy/tier/steward mechanism that SKILL.md only *references*; review-ledger carries the full schema that SKILL.md Step 4/5 reference. No duplicated body content; no contradiction between the two refs and SKILL.md |
| — | cross-skill: shared table single-sourced, not duplicated | PASS — coral restates **no** class taxonomy and **no** tier map; it names the table by path and §-number and declares "the table wins" on divergence. No second router |
| E1/E2 | evals.json parseable; ≥1 happy + ≥1 halt | PASS — parses; 2 happy-path + 5 halt-condition |
| E3 | ≥3 entries | PASS — 7 |
| E4 | each eval sourced | PASS — all 7 trace to SKILL.md §0/Steps/Four Rules, slate-routing §§, review-ledger Gate-read contract, or Design 08 |
| B2/B3 | Guardrails + Halt Conditions present | PASS |
| B8 | Guardrails stanza substantive (≥3 refusal conditions) | PASS — 5 named refusal conditions + the §0 HALT gate |
| P1/P2/P3 | discipline persuasion stack (rationalization table, red-flags list, authority language) | PASS — 9-row rationalization table, 8-item red-flags list, consistent MUST/Never/Refuse |
| A1 | no time-sensitive content in the new refs | PASS — none |
| A2 | no Windows-style paths | PASS |

### Cross-skill drift sweep — the three facts that could have diverged

The risk of extracting a shared table is that the citing skill keeps (and then drifts) a private copy of the routed facts. Checked each:

- **Six-class taxonomy** — stated once in `slate-routing.md` §1. SKILL.md §0/Step 2 list the six tokens *as a `Class:` enum* (the ledger header values, which must be literal) and tag them `per references/slate-routing.md`; coral restates **none** of them. The enum appearing in SKILL.md/review-ledger is the *header contract* (the gate matches these tokens), not a second taxonomy definition — so it is intentional single-sourcing of a token list, not drift.
- **Class→tier map** — authoritative in `slate-routing.md` §3. SKILL.md §0/Step 2 explicitly say tier is *"read off the table, never re-derived by hand"* and restate only the **floor rule** (shared-stack/skill-package T3, cannot drop below T2) as the load-bearing invariant — consistent with §3 and the §3 floor note. Coral restates no tier map. No divergent copy.
- **Unconditional `audit+author+prose` skill-package pin** — stated in `slate-routing.md` §4 (the load-bearing wiring rule, marked "state it in the citing skill"). SKILL.md Step 2.4 states it ("pins audit-skill + author-skill + prose-steward unconditionally — dropping any of the three requires an operator override"); coral line 29 states the production-phase counterpart ("the same trio `/cross-review` pins"). Both match §4 verbatim in substance. This is the one fact the table explicitly *asks* the citing skill to restate, and both restatements agree with the source. No drift.

The `shared-stack ≈ skill-package` tie-break (resolves to skill-package, the strict superset) is stated once in §2 and referenced consistently by the `halt-classify-before-dispatch` and `happy-skill-package-routes-t3-with-stewards` evals. The ledger's typed-header / fail-closed gate contract is single-sourced in `review-ledger.md` and referenced (not re-specified) by SKILL.md Step 4/5 and by the `halt-gate-fail-closed-on-malformed-ledger` eval. No contradictions found.

## Pressure scenarios

Not re-run in this pass — this is the conventions/consistency semantic pass following a structured-reference extraction, not a full re-pressure of the discipline spine. The discipline spine's load-bearing scenarios are already encoded as evals: the four original halts (no-artifact, dispatch-is-not-review, consensus-theater) plus the two new PLT-535 halts (`halt-classify-before-dispatch` exercising the §0 HALT gate, `halt-gate-fail-closed-on-malformed-ledger` exercising the ledger fail-closed contract). These remain valid and well-sourced.

## Skipped checks

None.

## Verdict

**Conventions upheld; no cross-skill router drift — one wording-consistency finding (warn) + one info.** The shared-table extraction is done correctly: the routing mechanism is single-sourced in `cross-review/references/slate-routing.md`, `/coral` cites it (R5 documentation link) without duplicating a router, and the one fact the table asks citing skills to restate (the skill-package steward pin) is restated consistently in both skills. The SKILL.md / slate-routing / review-ledger triad is internally consistent and the evals trace cleanly. The only drift is intra-coral wording: its "No tier selection" intro is now stale against the body that reads tier off the shared table — recommend the one-line "No scope-tier ceremony" reword before broad rollout. The R2 TOC observation is optional.

## References

- Conventions catalog: `.claude/skills/audit-skill/references/conventions-catalog.md`
- Audit methodology: `.claude/skills/audit-skill/SKILL.md` (semantic checks: `references/semantic-checks.md`)
- Change audited: commits `74a29b7` + `f967019` on branch `plt-535-cross-review-slate`
- Shared routing table: `.claude/skills/cross-review/references/slate-routing.md`; ledger schema: `.claude/skills/cross-review/references/review-ledger.md`; consumer: PLT-536 `/workstream` review-gate
