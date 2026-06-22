# Cross-review ledger — sei-interview-expert agent + /interview skill

Target:       branch `agent/sei-interview-expert` (.claude/agents/sei-interview-expert.md; .claude/skills/interview/ — SKILL.md, references/{method, sei-hiring-profile, kit-TEMPLATE, kit-coding-takehome, sources}.md, evals/evals.json, state/.gitkeep; scripts/sync-{skills,agents}.sh + READMEs + AGENTS.md for the `recruiting` domain wiring); PR from this branch
Class:        agent + skill-package (a new kit-based skill + thin persona, kit pattern)
Tier:         T2–T3 (Component — new agent + new skill + corpus + evals + sync wiring); no one-way door
Scope:        A new `sei-interview-expert` whose **primary customer is a human** (the hiring engineer): it reviews a candidate's technical interview artifact (the mempool coding take-home) and produces an evidence-grounded read + deep-dive verticals **tailored to the candidate's own implementation** (productionizing the system as the north star). Outputs follow the /lingua R6 human rails. Backed by the /interview skill (method + always-first Sei hiring-bar profile + pluggable per-format kits). Design-approved by the operator before build (the rubric is theirs to refine).
Dissenter:    rubric-fidelity (assigned) — re-verify every external citation against its primary source online; the dominant risk is a fabricated/over-claimed citation (a prior skill shipped a fabricated attributed quote caught in review).

## Round 1
State:        REVISE
OpenFindings: resolved in Round 2 (1 prose blocking; systems high-value additions folded; 2 RATIFY)
Convergence:  split

| Lens | Verdict | Finding |
|---|---|---|
| prose-steward (R6 human-output doctrine owner) | REVISE | **[blocking]** The skill correctly mandates R6 human-first output and the output format itself practices R6 (load-bearing lead + progressive disclosure; jargon ban enforced), BUT R6's **fidelity bound** was gestured at, never stated *as a bound* — under "just tell me yes/no" pressure the agent could lead with a clean recommendation while burying the deciding caveat in scorecard row 6 (the exact R6 failure the bound forbids, most acute here). Add the bound to Guardrail 1 + method §5 + the agent. [non-blocking] L4/L6 first-mention gloss; frontmatter safety-property placement. |
| rubric-fidelity (assigned dissenter — verifies citations online) | RATIFY | All 7 citations verified against primary sources (re:Work verbatim, BARS + its leniency caveat, Schmidt & Hunter with the honest debated-coefficient caveat, Tech Interview Handbook, Holloway paid/cite-link-only with **zero textual overlap**, levels.fyi, Canva + Meta). **No fabrication** (unlike the prior shipped skill), no over-claim, no reserved-source reproduction. The team-judgment boundary is drawn honestly — Dim 7 "team-judgment, leveling-informed", Dim 8 "cited direction; weighting is ours". |
| kit/sync audit | RATIFY | All 7 mechanical checks pass: kit conforms to kit-TEMPLATE (six sections, 1–4 observable-behavior anchor table), the 8 dimensions are byte-identical across method.md + kit, skill structure matches tee/idiomatic, `recruiting → sei` wired in both sync scripts (verify-catalog EXIT 0; catalog-coverage 13/0; interview ∈ sei, ∉ portable), doc-mirrors + AGENTS.md roster complete, evals valid JSON (8 unique non-vacuous ids, shape shared with 3 sibling skills), markdown well-formed. |
| systems-engineer (mempool technical soundness) | RATIFY | **Zero technically-wrong claims** — complexity claims, the concurrency trap, the soft-state DR framing, and the EIP-1559 framing all accurate. Flagged **high-value MISSING verticals** (non-blocking but "needed before this is the primary screening tool"): per-sender **nonce ordering** (the most common place a naive global-fee heap is actually *wrong* on an account-model chain) and **replace-by-fee** (min fee-bump anti-spam); plus gas-knapsack, MEV (scope down), and four sharpenings (indexed-PQ; lock-not-across-select + shard-by-sender; canonical-serialization vs block-hash; base-fee/block-utilization vs min-fee/mempool-pressure). |

### Round 1 resolution (applied)
- **Fidelity bound (prose, blocking):** added as an explicit bound to SKILL.md Guardrail 1, method.md §5, and the agent — "distill the altitude, never the deciding signal; a close call / disqualifying gap / load-bearing caveat rides in the lead's one-line why or one layer down, never compressed away (R3/R4 outrank R6)."
- **High-value mempool verticals (systems):** added per-sender nonce ordering ⭐ + replace-by-fee ⭐ + gas-knapsack + MEV (flagged optional/advanced reach, with an explicit "don't over-weight a MEV name-drop over nonce ordering" guard); folded the four sharpenings; §6 L6 note now names executability as the sharpest senior tell.

## Round 2
State:        RESOLVED
OpenFindings: 0
Convergence:  unanimous

| Lens | Verdict | Finding |
|---|---|---|
| prose-steward | RATIFY | The blocker is resolved — the R6 fidelity bound is stated *as a bound* in all three artifacts, faithful to the canonical wording, R3/R4-precedence cited correctly. Flagged the duplicate DR bullet (see systems) + a style note (the `(R3/R4 outrank R6)` tag missing from Guardrail 1) — both folded. |
| systems-engineer | RATIFY | All six re-checked items **technically correct**, including the scrutinized figures (geth ~10% PriceBump; EIP-1559 ±12.5%/target-utilization). One structural defect introduced by the round-1 edit: a **duplicated "Disaster recovery" bullet** (the new soft-state version added without removing the old WAL-as-strong version → conflicting Strong bars). Delete the old one. |
| rubric-fidelity | RATIFY | (Round 1 — citations untouched by the fix; no re-run needed.) |
| kit/sync audit | RATIFY | (Round 1 — sync wiring + structure untouched by the §5 prose edits; verify-catalog re-confirmed green, §4 anchor table intact.) |

### Round 2 finalization (folded)
- Removed the duplicate "Disaster recovery" bullet (kept the soft-state / re-gossip version — both reviewers caught it; verified one DR bullet remains).
- Added the `(R3/R4 outrank R6)` precedence tag to SKILL.md Guardrail 1 (prose style note — full-fidelity echo).

### Verdict
RESOLVED — unanimous RATIFY in Round 2; zero open findings. The assigned rubric-fidelity dissenter verified all seven citations against primary sources (no fabrication — the failure mode a prior skill hit). The systems-engineer confirmed the mempool kit is technically sound and lifted it from "credible" to "authoritative" by adding nonce-ordering + replace-by-fee (the highest-signal mempool verticals). The prose-steward (R6 doctrine owner) confirmed the agent practices the R6 human-first discipline it preaches, with the fidelity bound now explicit so distillation can't drop the deciding signal. The kit/sync audit confirmed the `recruiting` domain is correctly wired into the frontmatter-derived sync.

**Mechanical gates:** verify-catalog green (recruiting → sei); catalog-coverage suite 13/0; evals.json valid (8 cases). Cursor Bugbot + CI evaluated on PR open (Bugbot skip/NEUTRAL satisfies the check half for this doc/skill PR per the recorded review-gate policy; failing/findings block).
