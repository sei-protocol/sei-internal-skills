# Cross-review ledger — PLT-677 tee-specialist → pluggable knowledge-kit model

Target:       branch framework/tee-knowledge-kit (.claude/skills/tee/** [new], .claude/agents/tee-specialist.md [slimmed], .claude/skills/README.md + scripts/sync-skills.sh [wiring]); PR opened from this branch
Class:        skill-package
Tier:         T3 (Component — a self-contained skill package + agent refactor on the established /idiomatic template)
Scope:        PLT-677 — refactor the monolithic `tee-specialist` agent onto the pluggable knowledge-kit model (/idiomatic pattern): stand up a `/tee` skill (vendor-agnostic method + the always-first Sei profile overlay + a pluggable per-platform kit contract + 6 platform kits that CITE the `design/research/tee/*` ground truth, not paraphrase it), slim the agent to the thin persona backed by the skill, wire catalog/sync. Out of scope (per the issue): the TEE foothold build (PLT-671), authoring new TEE research, refactoring other domain experts.
Dissenter:    security-specialist (assigned throughout — the fidelity lens; the dominant risk was knowledge drift in extracting the agent's per-vendor specifics + the 16 verifier-policy properties into the skill/kits)

## Phase 1 — Design (the kit shape: SKILL.md + method.md + kit-TEMPLATE.md + kit-aws-nitro exemplar)

### Round 1
State:        OPEN
OpenFindings: 2 (blocking) + majors
Convergence:  split

| Lens | Verdict | Finding | Resolution |
|---|---|---|---|
| security-specialist (assigned dissenter, fidelity) | REVISE | **blocking**: kit-aws-nitro PCR8 trigger dropped `--private-key`; the 63M-vs-70M gas phrasing was unreconciled across kit/method. **major**: §4 table lacked landing rows for VP6/VP7/VP9/VP15/VP16; missing the RSA/`RSAES_OAEP_SHA_256` key-shape detail. **minor**: "Hypervisor signs" read as contradicting "Attester=enclave+NSM"; invented `§critical-9/11` anchors. Confirmed all 16 verifier-policy properties survived extraction as VP1–VP16 with no drops; trust-model spine + severity ordering correct. | applied @ R2 |
| product-manager | RATIFY | scope/YAGNI sound; the /idiomatic mirror is the right call. | n/a |
| audit-skill | RATIFY | shape conforms; the repo-root-relative research citations are the correct form (not broken links); catalog/sync = fan-out. | n/a |
| author-skill | RATIFY (refine) | **[high]** name `tee-profile.md` as the highest-priority fan-out artifact (spine-load-bearing); **[medium]** promote the trust-model-honesty gate to its own Rule 5; pre-wrote 7 eval seeds. | applied @ R2 |
| prose-steward | REVISE | guardrail(5)-vs-spine(4) count mismatch; Rule 2 lacked its failure-consequence; VP9/10/11/14 tier-3 rows at table-uniform weight; EAT clause ambiguous antecedent; `‖` non-ASCII glyph. | applied @ R2 |

### Round 2
State:        RESOLVED
OpenFindings: 0
Convergence:  unanimous

All R1 findings applied (extended the shared `betGraph`-analogue contract is N/A here; instead: added `createdAt`-style fidelity fixes to the kit + method; promoted Rule 5; reconciled the gas phrasing; added the VP6/7/9/15/16 landing rows + RSA detail; fixed the anchors; tier-tagged VP9/10/11/14; lifted the EAT note; `‖`→`||`). All five lenses RATIFY. The orchestrator (not a reviewer) applied every fix.

## Phase 2 — Implementation (the fan-out: tee-profile + 5 remaining kits + evals + slimmed persona + wiring)

### Round 1
State:        OPEN
OpenFindings: 0 blocking — all findings non-blocking (tightening)
Convergence:  unanimous (8 lenses)

The 5 remaining kits + the profile were sub-agent-authored against the vendor research, so the gating concern was per-kit fidelity. The assigned-dissenter (security-specialist) ran a **5-way per-kit fidelity sweep** — each kit audited claim-by-claim against its `design/research/tee/<doc>.md`.

| Lens | Verdict | Finding (evidence-bearing) |
|---|---|---|
| security-specialist — kit-intel-sgx-tdx fidelity | RATIFY | every offset/size/hash/claim traces to a real research § (MRENCLAVE offset 64, MRTD+RTMR required-together, ATTRIBUTES.DEBUG offset 48, 3-layer PCK, advisoryIDs/SGAxe, ~3.5–4M Sei cost, EPID EOL). No fabricated fact. |
| security-specialist — kit-amd-sev-snp fidelity | RATIFY | `MEASUREMENT` `0x090` launch-only, `REPORT_DATA` `0x050`, policy bit 19, `REPORTED_TCB` anti-rollback, VCEK/VLEK, BadRAM `ALIAS_CHECK_COMPLETE`/AMD-SB-3015, `HOST_DATA`, `CHIP_ID`, ~1.5–2M cheapest-direct — all trace. |
| security-specialist — kit-nvidia-cc fidelity | RATIFY | the subtle VP8 Hopper (SPDM-session-key-in-`REPORT_DATA`) vs Blackwell (TDISP/IDE) binding is **not garbled**; 128-bit nonce, 5-cert Hopper chain, PDI, ~100M+/~200k-zk/~3k-relayer all correct. |
| security-specialist — kit-tpm-rats fidelity | RATIFY | RATS/EAT/CCEL/DICE/SPDM/CoRIM correctly represented; CoRIM/claim-12/13 correctly routed to the profile (not mis-attributed to the TPM doc); **the absent TPM Sei gas figure is honestly flagged, not fabricated**. |
| security-specialist — kit-sei-onchain fidelity | RATIFY | precompile verified against `sei-chain/precompiles/p256/p256.go:24-25`; cost ranking + Marlin-Oyster amortization + the witness-key-freshness triad all trace; the Verifier-layer template adaptation is explicitly signposted and sound. |
| audit-skill (whole package) | RATIFY | the `kit-intel-tdx`→`kit-intel-sgx-tdx` cross-ref mismatch fixed everywhere; all kit/research cross-refs resolve (`kit-arm-cca.md` correctly absent — the no-kit eval target); evals valid; wiring accurate; state hygiene clean. |
| author-skill (whole package) | RATIFY | faithful /idiomatic mirror; per-vendor knowledge fully MOVED OUT of the persona (zero offsets in the agent); kit contract sound. Non-blocking: Rule-1 + EAT eval gaps; 2 degrade-evals mistyped as halt-condition; 2 N/A cite-cells bare; persona keeps Write/Edit (deliberate divergence from suggest-only idiomatic-reviewer). |
| prose-steward (whole package) | RATIFY | the six kits read as one voice + density; N/A dimensions typed (no fabricated fields); dense §4 tables serve the agent reader. Non-blocking: a bare `§2` cite in the Nitro exemplar collides with the profile's §2; minor citation-form nits. |

### Round 2 (convergence-confirming)
State:        RESOLVED
OpenFindings: 0
Convergence:  unanimous

All non-blocking findings folded in as orchestrator-finalization: added the Rule-1 (`discipline-claim-trust-gate`) and EAT-category-error (`discipline-eat-not-onchain-input`) evals; retyped the two degrade-don't-refuse evals from `halt-condition` to `discipline`; added the falsification-condition forbidden to `discipline-no-manufactured-tee`; moved the N/A reasons into the cite cells (amd VP8, nvidia VP11); disambiguated the Nitro `§2` citation; backticked `POLICY` bit 19 in sei-onchain; normalized `§ boundary disclosure`→`§boundary-disclosure`; blessed the `Template-fit note:` pattern in kit-TEMPLATE; noted the persona's deliberate Write/Edit divergence; added `security` to the README Domains line + a `security)` arm to sync-skills.sh. A focused author-skill re-verify of the eval changes RATIFY'd (all 5 spine rules + 3 degrade/halt behaviors covered, 10 evals, no non-inversions).

## Verdict
RESOLVED — design converged over 2 rounds, implementation converged over 2 rounds, all lenses RATIFY with zero open findings. The assigned dissenter (security-specialist) carried the fidelity load across both phases and a 5-way per-kit sweep, confirming every vendor offset/field/bit/version/gas-figure traces to a real `design/research/tee/*` section with no drift, distortion, or fabrication — the dominant risk of extracting a monolithic expert into a pluggable kit. Note: the "ASCII-only" instruction in the fidelity prompts was over-strict — the `§`/em-dash/arrow typography is established corpus house-style (used by the /idiomatic-mirrored exemplar and method); not a defect. Cursor Bugbot + CI: pending on PR open (Bugbot skip/NEUTRAL satisfies the check half for this doc-only skill PR per the recorded review-gate policy).
