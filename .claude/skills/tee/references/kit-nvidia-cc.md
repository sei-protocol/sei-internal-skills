# NVIDIA Confidential Compute kit

> Ground truth: `design/research/tee/nvidia-cc.md`. Every claim below cites it (§ or load-bearing claim #), a vendor doc, or an RFC. Do not paraphrase — cite.

## 1. Identity & RATS roles

- **What it is** — a **GPU TEE**: an NVIDIA Hopper (H100/H200) or Blackwell (B100/B200/GB200) GPU running in **CC-On** mode, attested as a genuine NVIDIA device whose firmware/VBIOS/mode match approved values. The GPU is never attested alone — it co-attests with a **host CPU TEE** (Intel TDX, AMD SEV-SNP, or ARM CCA) that holds it in the same TCB (§5.5, load-bearing claim 5). The boundary it protects is the CVM↔GPU data path (AES-256-GCM bounce-buffer on Hopper, link-layer IDE on Blackwell) (§1.4, §7.2).
- **RATS role mapping** — **Attester** = the GPU (its `RISC-V` GSP security processor is the SPDM responder, signing `MEASUREMENTS` with the per-reset AK) (§1.1, claim 1); **Endorser** = NVIDIA Device Identity PKI + the NVIDIA RIM Service (golden measurements) (§2.3); **Verifier** = NRAS (cloud), the `nvtrust`/`attestation-sdk` Local Verifier, or an on-chain contract; **Relying Party** = the entity gating secret/job release (§3.1).
- **Trust root / Endorser** — pin the **NVIDIA Device Identity Root CA** (long-lived, NVIDIA-controlled) above the per-family GPU CA and per-device Identity Key (§2.1). **Trust set = NRAS + NVIDIA PKI** — distinct from a silicon-vendor-CPU trust set (AMD/Intel) and from AWS's hypervisor+PKI set (`trusted-execution-on-sei.md` load-bearing claim 12). This feeds VP16.
- **Ground-truth doc** — `design/research/tee/nvidia-cc.md`.

**Trust-model note (VP16 / VP8):** NVIDIA CC is the platform that exercises **joint-attester binding** (VP8) — a relying party that trusts a GPU attestation in isolation has proven nothing about the CVM that drives the GPU. The headline correctness obligation here is checking the **CPU-TEE↔GPU binding**, not the two reports independently (§4.1, claim 8). NRAS itself is a third-party trust dependency (the relying party trusts NVIDIA's verifier service + its ephemeral L3 key, not just the GPU root) — air-gapped/sovereign deployments use the Local Verifier to shrink the trust set (§3.1 "Trust trade-off").

## 2. Evidence format

The Attester produces **DMTF SPDM 1.1 `MEASUREMENTS` response messages**, signed by the per-reset Attestation Key (AK) (§1.1, claim 1). The Verifier consumes that plus the device cert chain; downstream, NRAS re-emits the appraisal as an **EAT JWT** (`alg=ES384`, RFC 9711) — the EAT is the Verifier *output*, not the on-chain *input* (method "Note on output format"; §6.1).

- **Evidence signature scheme:** ECDSA **P-384 (secp384r1)** + **SHA-384**, enforced for the IK, the per-reset AK, and intermediate certs (§7.1, §2.2). The NRAS L3 EAT-signing cert is also P-384/SHA-384; JWT header `alg = ES384` (§7.1, claim 3).
- **SPDM measurement entry shape** (4 fields per SPDM 1.1 §10.11.1): `index` (uint8), `type` (uint8), `size` (uint16), `value` (bytes). H100 evidence carries **64 structured measurement records** (§1.1–§1.2, claim 11).
- **Parser gotchas (the verifier-bug surface):**
  - **NVIDIA withholds per-index measurement semantics** — the relying party is expected to consume verifier *appraisal results* (RIM-matched pass/fail), **not** hand-parse measurement indices to assign meaning (§boundary-disclosure; claim 9). Hard-coding an index→meaning map is a fabrication trap (method Rule 3).
  - Golden values are checked **indirectly** against the NVIDIA RIM Service (CoRIM/SWID-tag, XML-signed) keyed on `(driver-version, GPU-model)` — the verifier never holds a flat allowlist of hashes (§2.3, §3.2 `RIM` module, §6.4).
  - If consuming the NRAS path, you verify the **EAT JWT** (one P-384 verify + the L3→NRAS-CA chain), not the raw SPDM bytes; if consuming the Local path, you verify the **AK signature on the SPDM block + the device cert chain + the RIM signature** directly (§8.1).
- **VP13 version pinning:** pin the SPDM version (Hopper = **SPDM 1.1**; Blackwell TEE-I/O leans on **SPDM 1.2+** TDISP extensions), the EAT claims version (NRAS `1.0`), and the multi-GPU envelope (NRAS V3 / RATS §A.2.3 Detached EAT Bundle). Reject downgrade (§6.3, §3.1, §6.1).

## 3. Identity & measurement fields (VP2)

Binary identity is the set of **64 SPDM `MEASUREMENTS` records** appraised against the RIM golden values — NVIDIA does not publish per-index semantics, so identity is gated *through the RIM appraisal*, not by reading named fields (§1.2, claim 11; §boundary-disclosure, claim 9). The measurement set spans (§1.2):

| measurement category | binds | note |
|---|---|---|
| GPU hardware identity | Per-Device Identifier (PDI), burned at manufacture | permanent device fingerprint — see VP14 |
| firmware versions | bootrom, GSP firmware, BIOS, microcode | RIM-matched per driver/firmware version |
| VBIOS | Video BIOS hash | RIM-matched |
| driver-observed runtime state | register/fuse checks, **mode selection (CC-Off / CC-On / CC-DevTools)** | gates VP2 + VP3 |
| DevTools / debug flags | JTAG-disable bit, performance-counter mask | gates VP3 |
| memory configuration | ECC enable, memory firewall state | — |

**Required together:** a GPU-identity-genuine result is *necessary but not sufficient*. Binary identity for a Tide GPU-prover job gates on (a) the RIM appraisal passing for the approved `(driver, firmware, VBIOS)` tuple, (b) **mode = CC-On** (not CC-Off, not CC-DevTools), and (c) the **CPU-TEE↔GPU binding** (VP8) — a genuine-GPU report bound to an *unrelated* CVM is exploitable (§4.1, claim 8). The internal boot chain CEC EROT → FSP → GSP → SEC2 each contributes measurements; all four are in the report (§1.2). Reference values enter via the RIM Service (Endorser) and the on-chain governance registry (profile, VP7/VP15).

## 4. Verifier-policy specifics — the per-vendor fill-ins

| method dimension | NVIDIA's specific | cite |
|---|---|---|
| VP1 freshness | verifier-issued **128-bit nonce** embedded in the signed SPDM response (NIST SP 800-90 RNG); NRAS additionally caches `(nonce, device_cert_hash)` with a **24h TTL** (NRAS API takes a 256-bit hex nonce) | §1.3, §3.1 |
| VP2 binary binding | the 64-record SPDM `MEASUREMENTS` set passes RIM appraisal for the approved `(driver, firmware, VBIOS)` tuple **AND** mode = CC-On (§3) | §1.2, claim 11 |
| VP3 debug-mode | reject **CC-DevTools** mode (performance counters exposed → side-channel inference) and any set DevTools/debug flag (JTAG-disable bit, perf-counter mask) in evidence; require mode = CC-On | §5.1, §1.2 |
| VP4 anti-rollback | no single TCB integer — enforce **min driver+firmware versions via RIM** (reject golden-measurement matches for known-vulnerable versions) and the NRAS L3 / EAT validity window (24h TTL) | §2.3, §3.1 |
| VP5 cert-chain / generation | chain is Root CA → per-family GPU CA → per-device IK → per-reset AK; **5-cert chain on Hopper** (`trusted-execution-on-sei.md` claim, §generation-selection); validate per RFC 5280 to the pinned NVIDIA Root; fetch device cert from GPU via SPDM `GET_CERTIFICATE`, intermediates/root from NVIDIA PKI, revocation via **CertZapper OCSP**; caching the wrong-generation cert yields "valid signature, lies about platform" | §2.1, §2.3 |
| VP6 key isolation | → §6 (HKDF-SHA-384 SPDM session keys + fused IK / per-reset AK, never exported; FLR destroys all session/AK key material) | §6 |
| VP7 revocation | → profile (governance revocation in the on-chain reference-value registry); platform-side, CertZapper OCSP revokes device/L3 certs | profile + §2.3 |
| VP8 joint-attester | **the headline for this kit.** NVIDIA CC requires BOTH a GPU attestation AND a CPU-TEE attestation (TDX quote / SNP report), **bound** together; the verifier MUST check the binding, not the two reports independently. **Hopper (software-bound):** the SPDM `KEY_EXCHANGE` session public key (ECDHE-P-384) is hashed into the CPU TEE quote's `REPORT_DATA`; the relying party verifies all three legs — CPU quote genuine + `REPORT_DATA` match, GPU genuine/CC-On/fresh-nonce, and the session-key linkage. **Blackwell (hardware-bound):** PCI-SIG **TDISP/IDE** binds the device to a single TVM in the LOCKED state; CPU TEE Security Manager ↔ GPU mutually attest, yielding one chain CPU TEE → host TDISP root → device IDE endpoint. Multi-GPU adds the **PPCIE verifier** (`nvtrust/guest_tools/ppcie-verifier`) + NVSwitch attestation | §4.1–§4.3, §4.5, claim 8 |
| VP9 policy separation | parse SPDM/EAT Evidence (§2) into a normalized claim set, then apply policy (approved RIM tuple, min versions, mode, binding) as a separate layer — note NVIDIA's own design (consume appraisal, don't parse indices) already enforces this separation | method VP9 + §2 |
| VP10 advisories | **N/A** — no `advisoryIDs` field (Intel-specific); side-channel posture is RIM version-gating + the MLSys-2026 architectural side channels (RPC metadata, CPU↔GSP timing, BAR0 residue), which matter only for a co-tenant-host threat model | §7.3a |
| VP11 known-CVE bits | **N/A** — no `PLATFORM_INFO` mitigation bit (AMD BadRAM / AMD-SB-3015 specific) | N/A (see kit-amd-sev-snp) |
| VP12 host-controlled-but-signed | **N/A** — NVIDIA evidence is firmware/hardware-measured (RIM-appraised); there is no Nitro-`user_data`/AMD-`HOST_DATA`-style host-supplied-but-signed field in the GPU report. (The CPU-TEE leg may carry one — that is the *CPU* kit's VP12, gated under VP8.) | §boundary-disclosure |
| VP13 version pinning | pin SPDM version (Hopper 1.1 / Blackwell 1.2+ TDISP), EAT claims version (NRAS `1.0`), multi-GPU envelope (NRAS V3 / RATS §A.2.3); reject downgrade | §6.3, §6.1 |
| VP14 privacy / fingerprint | **PDI (Per-Device Identifier)**, burned at manufacture, is a permanent per-chip fingerprint in the measurement set; raw on-chain attestations leak it as a ledger-wide identity — prefer ZK that hides device-unique fields, or fleet-scoped policy, for validator-as-attester patterns | §1.2, `trusted-execution-on-sei.md` claim 13 |
| VP15 registry integrity | → profile (multisig + time-lock + transparency/CoRIM + emergency revocation on the reference-value registry) | profile |
| VP16 cross-vendor delta | trust set = **NRAS + NVIDIA PKI** (§1) — neither a silicon-CPU-vendor set nor AWS's hypervisor+PKI set; surface it explicitly when this kit sits beside an AMD/Intel/Nitro kit, and note the joint-attester case spans *two* trust sets (NVIDIA + the host CPU vendor) | §1, claim 12 |

## 5. On-chain verification (Sei)

No EVM **P-384** precompile exists; Sei EVM (chain ID 1329) inherits the standard precompile set (P-256 at `0x1011`, native `secp256k1` `ecrecover`) but not P-384, so every P-384 verify is pure Solidity (§8.2). A full attestation is **multiple** P-384 verifies — leaf cert chain + AK signature on evidence + RIM signature (claim 10). Three postures (from `nvidia-cc.md` §8 + `trusted-execution-on-sei.md` decision-driver):

- **Direct on-chain — NOT viable: ~100M+ gas.** Pure-Solidity P-384 is ~20M gas/verify after heavy optimization (~500M naive, Estonian e-ID benchmark); the multi-verify full chain lands at **~100M+ gas**, above Sei's practical per-tx ceiling (§8.2, §8.4, claim 10).
- **ZK-proven attestation — ~200k gas.** A SNARK proves "I hold a valid NRAS EAT chained to NVIDIA Root CA, measuring this workload, fresh within N hours"; on-chain verifies only the SNARK (~200k gas). Proving time is the bottleneck (P-384 ECDSA in-circuit ~10–30M constraints) (§8.3 #2, §8.5 #3, claim 10).
- **Trusted relayer — ~3k gas.** A trusted relayer verifies the EAT off-chain, then submits a **secp256k1**-signed confirmation via `ecrecover` (~3k gas). Smallest cost, adds the relayer to the trust set (§8.3 #1, §8.5 #1, claim 10).

**Posture: direct on-chain is not viable; use ZK-proven attestation or a trusted relayer.** Apply the same witness-key-freshness discipline as the Nitro amortization pattern — without sequence numbers + binding-key rotation on TCB/version change + an on-chain validity window, a stolen relayer/binding key outlives the attestation it was minted under. (`design/research/tee/nvidia-cc.md` §8 + load-bearing claim 10; `design/research/tee/trusted-execution-on-sei.md` decision-driver + load-bearing claims 1–2.)

## 6. Key-release / integration pattern (VP6)

NVIDIA's idiomatic pattern is **attested ECDHE session binding**, not a KMS condition-key release. The SPDM `KEY_EXCHANGE` runs **ECDHE on P-384** with an **HKDF-SHA-384** PRF; the master secret expands into **46 per-channel AES-256-GCM keys** (6 GSP, 6 SEC2, 32 copy-engine) (§7.3, claim 11). Fused IK and per-reset AK are never exported; all session/AK key material is destroyed on FLR / VM teardown (§7.5). The correct-by-construction release is: **bind the secret to the joint CPU-TEE↔GPU attestation** — the SPDM session key is hashed into the CPU TEE quote's `REPORT_DATA` (Hopper) or carried by the TDISP LOCKED-state device report (Blackwell), so a secret released against a verified joint attestation is unsealed only to a known-good GPU held by a known-good CVM (§4.2–§4.3, claim 8).

**Best fit (the kit's recommendation):** NVIDIA CC is the **best fit for GPU-accelerated ZK proving** — ZK provers (Risc0, SP1, Halo2 GPU variants) lean on GPU compute, and the joint GPU+CPU attestation **binds the prover image** to both the GPU and CPU TEE identity, so the on-chain verifier accepts a proof only from a registered, attested prover (`trusted-execution-on-sei.md` "Best fit for GPU-accelerated proving"; §4; claim 8). This is the ~200k-gas ZK-proven-attestation path of §5.

## 7. Citations

- Ground truth: `design/research/tee/nvidia-cc.md` §boundary-disclosure, §1 (wire format / measurements / nonce / Hopper-vs-Blackwell), §2 (signing chain / P-384 / cert fetch / OCSP / CoRIM), §3 (NRAS / Local Verifier / ITA), §4 (joint CPU+GPU binding, IOMMU, PPCIE), §5 (CC modes / FLR / drivers / supported host CPU TEEs), §6 (RATS/EAT/SPDM/CoRIM/TPM alignment), §7 (P-384/SHA-384, AES-256-GCM, ECDHE session keys, side channels), §8 (on-chain cost), + load-bearing claims 1–11 (esp. 8 joint-attester, 9 non-disclosure boundary, 10 on-chain cost, 11 64-records/46-keys/boot-chain).
- Integrative: `design/research/tee/trusted-execution-on-sei.md` decision-driver (NVIDIA cost row), "Best fit for GPU-accelerated proving", + load-bearing claims 1–2 (Sei P-256 precompile + amortization), 6 (joint attestation required), 12 (trust-set deltas — NVIDIA = NRAS + NVIDIA PKI), 13 (PDI fingerprint on-chain).
- Primary: NVIDIA *Confidential Compute on Hopper H100* WP-11459-001; *Secure AI with Blackwell and Hopper* WP-12554-001 v1.3; NRAS `/v1/attest/gpu` API reference; `NVIDIA/nvtrust` (Local Verifier, PPCIE) + `NVIDIA/attestation-sdk` (C++).
- Standards: RFC 9334 (RATS, roles + §A.2.3 Detached EAT Bundle); RFC 9711 (EAT — NRAS EAT JWT); DMTF SPDM 1.1 (DSP0274, §10.11.1 measurement shape); DMTF CoRIM (RIM bundles); PCI-SIG TDISP 1.0 + IDE (Blackwell TEE-I/O); RFC 5280 (cert-chain validation).
- Academic: ACM Queue "Creating the First Confidential GPUs" (Nertney et al.); MLSys 2026 "NVIDIA GPU Confidential Computing Demystified" arXiv 2507.02770 (Gu et al. — 64 records, 46 keys, CEC EROT→FSP→GSP→SEC2, side channels); arXiv 2409.03992v2 (H100 perf); Estonian e-ID Solidity P-384 benchmark.
