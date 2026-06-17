# Research: Automata Network — on-chain attestation validation (prior art)

**Status:** Draft
**Date:** 2026-06-17
**Issue:** PLT-677 — https://linear.app/seilabs/issue/PLT-677
**Impact:** sei-agentic-mesh — https://linear.app/seilabs/project/sei-agentic-mesh
**Authors:** brandon (+ /research)

Prior-art reference for the TEE corpus: the most production-advanced **on-chain** TEE attestation verifier in the ecosystem. The `/tee` kits (`kit-intel-sgx-tdx`, `kit-sei-onchain`) cite this doc for the concrete Intel-DCAP on-chain verification mechanism, its gas profile, and its limits as a general verifier. This is empirical ground truth (a `design/research/tee/*` record), not a build decision (PLT-671 owns that).

## Question

**Decision it informs:** whether/how Sei on-chain attestation verification should point at Automata's prior-art mechanisms as the production path for Intel SGX/TDX, and how its zkVM / Multi-Prover approaches compare to the Sei P256VERIFY-direct and Marlin-Oyster-amortization patterns already in the corpus.

**Falsifiable claims sought:** (1) Automata ships open-source on-chain Intel SGX/TDX DCAP verifier contracts; (2) its on-chain DCAP gas profile vs the corpus's ~3.5–4M Sei P256VERIFY estimate; (3) a RISC Zero / SP1 zkVM path reducing on-chain cost to a single SNARK verify; (4) Multi-Prover AVS as a distinct mechanism; (5) which TEEs/vendors it supports on-chain; (6) Sei-EVM deployment / which chains.

**Scope boundary:** IN — Automata's on-chain attestation-validation mechanisms as prior art. OUT — Automata's non-attestation products/token; a Sei build decision (PLT-671); re-deriving the corpus's own cost numbers.

## Sweep coverage

Three blind angles (the inline ≤3 norm): **by-source/entity** (Automata GitHub repos + deployment manifests + docs — the strongest, primary grounding), **by-time** (blog/release evolution, last ~24 months), **by-counter-thesis** (evidence against "production-ready, reusable, point-Sei-at-it"). Deliberately not swept: Automata's identity/2FA products and token; the EigenLayer restaking economics; a hands-on gas benchmark on Sei (a build/bench task, not research).

## Findings

- **[verified]** Automata ships an open-source **on-chain Intel DCAP quote verifier** for EVM (+ Solana): entrypoint `AutomataDcapAttestation(Fee)` parses the quote header and routes to version-specific `V3/V4/V5QuoteVerifier`, reading collateral from a `PCCSRouter`. Source: https://github.com/automata-network/automata-dcap-attestation (README + the Optimism deployment manifest with live contract addresses). Refutation tried: "is this a design, not deployed?" — refuted by per-chain deployment manifests with concrete addresses across ~22 networks.
- **[verified]** **On-chain PCCS** (Provisioning Certificate Caching Service) is a permissionless Solidity collateral store (QEIdentity / TCBInfo / PCK X509 DER-decoder + CRLs as Helper contracts; DAO Base contracts modeled on Intel's SGX PCCS Design Guide) — and it is **deployed per-chain**. Source: https://github.com/automata-network/automata-on-chain-pccs. Refutation tried: "can you just point at an existing instance?" — refuted; each network has its own PCCS+DAO addresses, so reuse on a new chain means deploying + maintaining the PCCS/TCB-DAO stack there.
- **[verified]** **Gas profile (primary repo "Verification Methods" table):** native on-chain ~**4–5M gas** (~4M with the RIP-7212 / P256 precompile, ~5M without); zkVM SNARK-verify path — **RiscZero Groth16 522k**, **SP1 Groth16 493k**, **SP1 Plonk 569k**. Source: https://github.com/automata-network/automata-dcap-attestation. Refutation tried: a June-2024 RISC Zero blog cited "~350K gas" — resolved as a **stale/older** measurement point (different proof approach/era) superseded by the repo README; the README is the current primary source. This **corroborates the corpus's ~3.5–4M-native / ~500k-zkVM Intel figures** (`kit-intel-sgx-tdx` §5, `trusted-execution-on-sei.md` §decision-driver).
- **[verified]** zkVM proving is via **RISC Zero (Bonsai/Boundless), SP1 (Succinct Prover Network), and Pico (Brevis, local-only as of v1.1)** — proofs generated **off-chain** on a prover network, then a single SNARK verified on-chain. Source: https://github.com/automata-network/automata-dcap-attestation (Features + Rust workspace). Refutation tried: "is the cheap path fully on-chain?" — refuted; the ~500k figure is the *on-chain verify* of a proof produced off-chain (an off-chain prover dependency + latency, e.g. SP1 <30s–2min).
- **[verified]** **AMD SEV-SNP** is supported on-chain **via the zkVM path only** (a separate `amd-sev-snp-attestation-sdk`, `verifyAndAttestWithZKProof(output, zkCoprocessor, proofBytes)`), **not** the native DCAP Solidity path. Source: https://github.com/automata-network/amd-sev-snp-attestation-sdk. Refutation tried: this resolved the apparent "Intel-only" conflict — the *native* on-chain verifier is Intel SGX/TDX only (DCAP is an Intel primitive); the *zkVM* verifier generalizes to SEV-SNP.
- **[verified-negative]** **No NVIDIA** on-chain support found (no NVIDIA repo in the Automata org, no doc). Source: Automata GitHub org + v1.1 blog. Refutation tried: searched the org for an NVIDIA repo — none (absence of evidence, not a documented exclusion).
- **[verified]** **v1.1 (Nov 2025, "for agentic systems")** adds **Quote V5** (unifies SGX bodies + TDX 1.0 + TDX 1.5 reports), **EIP-7951 secp256r1-precompile** support (~1M gas reduction on the native path on Fusaka-hardfork chains), and TCB-Evaluation-Data-Number pinning (`AutomataTcbEvalDao`). Source: https://github.com/automata-network/automata-dcap-attestation v1.1 NOTE + https://blog.ata.network/automatas-release-of-dcap-attestation-v1-1-for-agentic-systems-84ae98900370.
- **[verified]** EVM contracts (on-chain PCCS + DCAP Attestation, v1.0) were **audited by Trail of Bits, Feb 2025**. Source: repo "Security Audits" + https://github.com/trailofbits/publications (2025-02 DCAP/PCCS review). Refutation tried: a secondary summary said "March 2025" — refuted by the primary repo + the ToB publications date (Feb 2025). A later OpenZeppelin audit (Oct 2025) found a PCCS Router timestamp issue, fixed in v1.1.0 (Nov 2025).
- **[verified-negative]** **Not deployed on Sei.** Sei mainnet (chainId `1329`) and atlantic-2 (`1328`) are **absent** from the v1.1.0 network-registry; no Sei mention in any repo, NOTE, or blog. Deployed chains (~22 networks — ≈10 mainnets + their testnets) include Ethereum, Optimism, Base, Arbitrum One, BNB, Polygon, Unichain, World Chain, Avalanche C-Chain, HyperEVM, and Automata's own OP-Stack L2 (chainId `65536`). Source: https://github.com/automata-network/automata-dcap-attestation network-registry (v1.1.0 tree). Refutation tried: checked the registry chain-ID list directly — Sei not present.
- **[unverified]** **AWS Nitro on-chain** support — only a loose Medium-blog mention of an on-chain Nitro verifier in Automata's suite; **no primary repo/doc** confirms one. Kept unverified; not asserted.
- **[unverified]** The **Multi-Prover AVS ↔ DCAP-verifier coupling.** The Multi-Prover AVS (EigenLayer; TEE prover committees spanning Intel/AMD/AWS; validated Scroll Sepolia) is a **distinct** Automata product; whether the AVS's prover-committee attestations are verified *by* the `AutomataDcapAttestation` contract is architecturally plausible but unconfirmed — no fetchable primary doc states it. Source: https://docs.ata.network/tee-overview/multi-prover-avs-eigenlayer. Kept unverified.

*(No findings were refuted outright; the "~350K gas" and "March 2025 audit" secondaries were corrected against primary sources rather than dropped.)*

## Completeness assessment

- **docs.ata.network** GitBook pages 404 on direct fetch (client-rendered); the load-bearing facts came from the **GitHub-API READMEs + deployment manifests + the ToB publications repo** (stronger primary sources), so the gap doesn't affect the verified set.
- **Unverified, kept out of the recommendation:** on-chain AWS Nitro support; the precise AVS↔DCAP coupling. Resolving them needs a primary doc/repo that wasn't found.
- **Inference, not a finding:** Sei independently implements the RIP-7212 / P256 precompile (`address(0x1011)`, per the corpus `tee-profile.md` + Sei docs), so a *hypothetical* Sei deployment of Automata's native verifier would land in the cheaper ~4M-gas regime — but no Automata-on-Sei integration exists, so this is a projection for a future build (PLT-671), not an established fact.
- No gap changes the recommendation; no re-sweep needed.

## Synthesis & recommendation

Automata Network is the **production-grade prior art for on-chain Intel SGX/TDX DCAP verification** — audited (Trail of Bits Feb 2025), multi-chain (~22 networks), V3/V4/V5, with a zkVM path (~493–569k gas) that also extends on-chain coverage to **AMD SEV-SNP** (zkVM-only). Its gas profile **independently corroborates the corpus's Intel numbers** (~4M native with a P256 precompile; ~500k via the zkVM path).

But it is **not a turnkey, general, Sei-ready verifier**, for three verified reasons: (1) the **native** on-chain verifier is **Intel-only** (SEV-SNP rides the zkVM path; no native non-Intel, no confirmed Nitro, no NVIDIA); (2) it requires **per-chain PCCS + TCB-DAO deployment** and ongoing Intel TCB/QE-collateral maintenance, plus an **off-chain prover** (Bonsai/SP1) for the cheap path; (3) it is **not deployed on Sei**.

**For the corpus:** cite Automata in `kit-intel-sgx-tdx` (the concrete Intel-DCAP on-chain verifier + the precise zkVM gas breakdown + the V5/precompile evolution) and in `kit-sei-onchain` (as the production exemplar of the on-chain-DCAP / zkVM-amortization pattern, with the per-chain-PCCS + off-chain-prover + not-on-Sei caveats). It is the closest prior art to the `kit-sei-onchain` "verify-once / amortize" posture — for Intel; the Marlin-Oyster secp256k1 pattern remains the corpus answer for Nitro, and SEV-SNP-direct (P-384 Solidity) vs Automata-zkVM is a real per-vendor choice the kits should surface. Whether Sei should host an Automata deployment is a **PLT-671 build decision**, informed by — not decided by — this research.

## References

- https://github.com/automata-network/automata-dcap-attestation — on-chain Intel DCAP verifier (contracts, V3/V4/V5, gas table, zkVM support, v1.1 NOTE, network-registry)
- https://github.com/automata-network/automata-on-chain-pccs — on-chain PCCS (permissionless collateral store + TCB DAOs)
- https://github.com/automata-network/amd-sev-snp-attestation-sdk — SEV-SNP zkVM on-chain verification
- https://github.com/automata-network/tdx-attestation-sdk — TDX on-chain attestation SDK
- https://github.com/trailofbits/publications — DCAP/PCCS security review (2025-02)
- https://blog.ata.network/automatas-release-of-dcap-attestation-v1-1-for-agentic-systems-84ae98900370 — v1.1 (Quote V5, EIP-7951, agentic framing)
- https://blog.ata.network/zkvm-for-tees-attestation-aggregation-and-tls-verification-with-risc-zero-3573a66c6723 — RISC Zero zkVM-for-TEEs (2024; superseded gas figures)
- https://docs.ata.network/tee-overview/multi-prover-avs-eigenlayer — Multi-Prover AVS (distinct product)
- Corpus: `design/research/tee/trusted-execution-on-sei.md` (Sei cost ranking), `design/research/tee/intel-sgx-tdx.md`; kits `kit-intel-sgx-tdx.md`, `kit-sei-onchain.md`. Related: PLT-677 (the /tee kit model), PLT-671 (the TEE foothold build).
