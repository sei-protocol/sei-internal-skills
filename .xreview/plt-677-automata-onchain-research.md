# Cross-review ledger — Automata on-chain attestation /research (PLT-677 follow-on)

Target:       branch framework/automata-onchain-research (design/research/tee/automata-onchain-attestation.md [new], design/research/tee/README.md [index], .claude/skills/tee/references/kit-intel-sgx-tdx.md + kit-sei-onchain.md [citation additions]); PR opened from this branch
Class:        research-artifact + skill-package citation
Scope:        Capture prior-art /research on Automata Network's on-chain attestation-validation mechanisms (on-chain Intel-DCAP verifier, on-chain PCCS, RISC Zero/SP1 zkVM paths, SEV-SNP-via-zkVM, Multi-Prover AVS, gas profile, deployment) into the TEE corpus, and cite it from the two relevant kits. Ran the /research discipline: scope → 3-angle blind sweep → adversarial verify → completeness → synthesis. User-requested follow-on to PLT-677.
Dissenter:    security-specialist (fidelity — verify the artifact does not overclaim against primary sources)

## Round 1
State:        RESOLVED
OpenFindings: 0 (all findings non-blocking; applied as finalization)
Convergence:  unanimous

| Lens | Verdict | Finding |
|---|---|---|
| security-specialist (assigned dissenter, fidelity) | RATIFY | **Independently fetched the primary sources LIVE** (automata-dcap-attestation README + v1.1 NOTE + v1.1.0 network-registry DEPLOYMENT.md; automata-on-chain-pccs README; amd-sev-snp-attestation-sdk README; trailofbits/publications Feb-2025 review PDF, HTTP 200). Every `[verified]` claim holds — gas table (~4-5M native / 522k RiscZero / 493k SP1 Groth16 / 569k SP1 Plonk), SEV-SNP-via-zkVM-only, Intel-only-native, no-Sei (grep of the registry confirmed; ~22 networks), ToB Feb-2025. The unverified items (Nitro on-chain, AVS↔DCAP coupling) are correctly held out of the recommendation; the Sei-precompile projection is correctly flagged as inference. Kit citations faithful, no drift. Recommendation rests only on verified findings. Non-blocking nits: "~22 networks" precision; the NVIDIA verified-negative over-framed absence-of-evidence. |
| audit-skill | RATIFY | artifact conforms to the /research shape (frontmatter + Question + Sweep coverage + tagged Findings + Completeness + Synthesis + References); the new doc path resolves from both kits (repo-root-relative text); gas figures consistent across artifact ↔ kit ↔ ground truth; frontmatter Issue=PLT-677 / Impact=sei-agentic-mesh well-formed; kit edits purely additive, no regressions. [info]: the new doc wasn't in the corpus README index. |
| prose-steward | RATIFY | dual-aligned: decision-first Synthesis with the (1)/(2)/(3) disqualifier list; `[verified]`/`[unverified]`/`[verified-negative]` tags scannable; the four load-bearing caveats (Intel-only-native, per-chain-PCCS, not-on-Sei, SEV-SNP-via-zkVM) surfaced not buried; kit additions carry the caveat in a dedicated non-buried sentence; literals backticked. Style nits: "architecturally implied" soft hedge in the AVS unverified finding; the Nitro "their" antecedent; an optional kit parenthetical to pre-empt the ~3.5-4M-vs-~4-5M apparent conflict. |

### Verdict
RESOLVED — unanimous RATIFY round 1; zero blocking findings. The fidelity dissenter's live primary-source re-fetch is the gating evidence: no overclaim, no misapplied tag, no distorted kit citation. All non-blocking nits folded in as finalization: added the corpus README index row; reworded the AVS unverified finding ("architecturally plausible but unconfirmed" + named the `AutomataDcapAttestation` contract); fixed the Nitro antecedent; softened the NVIDIA verified-negative to absence-of-evidence; clarified "~22 networks (≈10 mainnets + testnets)"; added the corpus-estimate-vs-Automata-table disambiguation to the intel kit. Cursor Bugbot + CI: pending on PR open (Bugbot skip/NEUTRAL satisfies the check half for this doc-only PR per the recorded review-gate policy).

Note: this is a /research *findings* artifact (discover, not decide) — whether Sei should host an Automata deployment is a PLT-671 build decision it informs, not decides. Lineage: threaded to PLT-677 as a corpus reference (per /research lineage precision — the research doc is an additional reference, not the bet's design URL).
