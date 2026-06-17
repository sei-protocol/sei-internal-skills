# TPM 2.0 / open-standards (RATS) kit

> Ground truth: the vendor specs, RFCs, and reference implementations cited inline below — this kit distills them and is **self-contained**. Every load-bearing claim cites a primary source (a vendor spec, an RFC, or `sei-chain` source); do not paraphrase — cite the primary source. Deeper derivation is archived in the Sei-internal `bdchatham-designs` TEE corpus (access-gated) — **provenance only, not required to use this kit**.
>
> **Dual-purpose kit.** (a) TPM 2.0 as an Attester platform (a real chip on the host); (b) the open-standards layer — RATS / EAT / CCEL / DICE / SPDM / CoRIM — that the *other* kits' cross-vendor concerns map onto. This is the **home of the RATS RFC 9334 role vocabulary** the whole skill uses, and the anchor for **VP9** (policy separation) and **VP16** (cross-vendor trust-set deltas). When a vendor kit says "feeds VP16" or "→ method VP9", it is pointing back here.

## 1. Identity & RATS roles

- **What it is** — a real **TPM 2.0 chip on the host** (the only ecosystem with one; every other TEE replaces the chip with on-die / in-firmware logic — §10 "Notable cross-vendor observations"). It protects a **measured-boot host-integrity** boundary: SRTM measures firmware → bootloader → kernel into PCR 0–15 (§2.1, §3.1); DRTM late-launch measures an MLE into PCR 17–22 (§3.1–3.2). The attestation surface is `TPM2_Quote` (platform state) and `TPM2_Certify` (key residency) (§3.3).
- **RATS role mapping** (RFC 9334 §5.1, the canonical 5-role vocabulary — load-bearing claim 5) — **Attester** = host platform + TPM chip producing `TPM2_Quote` + event log; **Endorser** = the **TPM manufacturer** (Infineon, ST Micro, Nuvoton, Microsoft vTPM) via the **EK certificate** baked in at manufacture, or a **Privacy CA** that issues the AK credential; **Reference-Value-Provider** = OEM / OS-distro RIMs (and, for Sei, governance — see profile §3); **Verifier** = the entity appraising Evidence against policy (off-chain service or contract); **Relying Party** = the entity gating the decision on the Attestation Result (§5.1, §5.3).
- **Trust root / Endorser** — pin the **TPM-vendor CA** that anchors the EK certificate chain, OR the **Privacy CA** in privacy-CA flows. **Trust set = TPM silicon manufacturer** (per-vendor CA: Infineon/ST/Nuvoton/MS vTPM), distinct from the AMD/Intel silicon-vendor KDS/PCS, distinct from the AWS-hypervisor PKI, distinct from NVIDIA NRAS — this distinction *is* VP16 (§4.3, §5.3).
- **RATS topologies** — Passport (Attester holds the Attestation Result, presents it to many RPs) vs Background-Check (RP is the gateway to the Verifier) (§5.2). Neither is mandatory; pick per connection.
- **Provenance** — derived from the Sei-internal TEE research archive (`bdchatham-designs`, access-gated); the load-bearing facts are inlined here with their primary-source citations.

**Standards-layer caveat (the home of the cross-vendor vocabulary):** RFC 9334 is the architecture vocabulary; do not invent project-local role names (load-bearing claim 5). RFC 9711 (EAT) is the verifier-**output** claim envelope, **not** the on-chain Evidence input — feeding EAT to an on-chain verifier is a category error (§6, load-bearing claim 6; method "Note on output format"). CoRIM is the cross-vendor **reference-value** format to align on (profile §6, `trusted-execution-on-sei.md` load-bearing claim 12) — VP15 lives in the profile, not here.

## 2. Evidence format

`TPM2_Quote` returns a signed **`TPMS_ATTEST`** structure plus a separate signature — **not** raw PCR values (§4.1, load-bearing claim 3). Carried alongside is the **TCG event log** (the log the verifier replays). The signed payload (`TPMS_ATTEST`, Part 2: Structures §10.12 — §4.1):

```
TPMS_ATTEST {
  magic            TPM_GENERATED_VALUE   // 0xFF544347 ("\xffTCG") — FIRST field
  type             TPMI_ST_ATTEST        // TPM_ST_ATTEST_QUOTE for a Quote
  qualifiedSigner  TPM2B_NAME            // qualified name of the AK
  extraData        TPM2B_DATA            // caller nonce / qualifyingData (VP1)
  clockInfo        TPMS_CLOCK_INFO       // clock, resetCount, restartCount, safe
  firmwareVersion  UINT64
  attested         TPMU_ATTEST           // TPMS_QUOTE_INFO for a Quote
}
TPMS_QUOTE_INFO { pcrSelect TPML_PCR_SELECTION; pcrDigest TPM2B_DIGEST }
```

- **Signature schemes (§4.2):** ECDSA P-256 + SHA-256 (`TPM_ALG_ECDSA` + `TPM_ALG_SHA256`), or P-384 / SHA-384 for higher-assurance profiles; RSASSA-PKCS1-v1_5 + SHA-256 for RSA-2048 AKs. PC Client profile mandates SHA-256 in all chips; SHA-384 increasingly required for confidential-compute profiles.
- **Parser gotchas (the verifier-side checks — §4.3, load-bearing claim 3):** verify **both** the signature **and** the event-log replay against `pcrDigest` — a valid signature over a `pcrDigest` whose log you did not replay binds nothing. Check `magic == TPM_GENERATED_VALUE (0xFF544347)` (non-restricted user keys are TPM-prohibited from signing buffers starting with this value, so the magic check is what proves a restricted-key path — §4.1). Check `type == TPM_ST_ATTEST_QUOTE`. `extraData` is the nonce slot (VP1). `pcrDigest` is a digest over the **concatenation of the selected PCRs**, not the PCRs themselves (§4.1).
- **Event log = TCG crypto-agile event log** (`EFI_TCG2_PCR_EVENT2` entries) (§8). For **confidential VMs** this same format is carried in **CCEL** (an ACPI table) with PCR indexes replaced by **RTMR** indexes — so TPM event-log replay tooling is reused for TDX/SEV-SNP CVMs (§8, load-bearing claim 7). See §3 for the RTMR mapping.
- **VP13 version pinning:** pin acceptable `TPMS_ATTEST` `type` + the AK signature-algorithm set; reject downgrade (§4.1–4.2, method VP13).

## 3. Identity & measurement fields (VP2)

Binary/platform identity is the **PCR set** named in `pcrSelect`, bound via `pcrDigest` and confirmed by event-log replay (§2.1, §4.3). TPM 2.0 PC Client defines **24 PCRs** (`PCR[0..23]`) per hash-algorithm bank (SHA-256 universal; SHA-384 increasingly required) (§2.1). The only mutation is extend (§2.2, load-bearing claim 3):

```
PCR_new = HASH( PCR_old || measurement )         // || = byte concatenation
```

Standardised PC Client usage and **which PCRs may gate a trust decision** (§2.1, §2.3, load-bearing claim 2):

| PCR(s) | binds | trust-decision use |
|---|---|---|
| 0–7 | SRTM: CRTM/BIOS, host config, option ROMs, boot manager, **Secure Boot policy (PCR7)** | durable — reset **only** on platform reset |
| 8–9 | OS: bootloader-stage, kernel cmdline / initrd / IMA | durable |
| 10 | Linux IMA | durable |
| 11–15 | OS / application measurements | durable |
| **16** | debug | **NEVER trust** — resettable any time, any locality (load-bearing claim 2) |
| 17–22 | DRTM: MLE hash (PCR17) + trusted-OS stages | reset **only** from Locality 4 via `SENTER`/`SKINIT` (§3.1) |
| **23** | application support | **NEVER trust** — resettable from Locality 0 (load-bearing claim 2) |

**Required-together / exploitable-partial rule:** state exactly which PCRs the policy gates on; a `pcrSelect` that omits a PCR carrying load-bearing state is an exploitable partial identity. **PCR 16 and PCR 23 must never appear in a trust decision** — a design that quotes them is a bug (load-bearing claim 2). For DRTM "trusted launch", the relevant measurement is PCR 17 (the MLE), which lets the verifier ignore the entire SRTM boot history (§3.2). Reference values enter as **RIMs / golden PCRs** from the RVP (§4.3 step 7); on Sei the RVP is governance (profile §3).

**CVM bridge (CCEL / RTMR):** in a confidential VM there is no host TPM in the guest path — the per-VM measurement (TDX `MRTD`, SEV-SNP `MEASUREMENT`) plays the DRTM role inside the guest (§3.2), and **RTMRs replace PCRs** in the CCEL event log (§8, load-bearing claim 7). TDX exposes **four RTMRs** mirroring the SRTM split (§8):

| RTMR | binds | mirrors |
|---|---|---|
| RTMR[0] | virtual firmware (TDVF / OVMF) | PCR 0 |
| RTMR[1] | OS loader (shim, GRUB) | PCR 4 |
| RTMR[2] | kernel + initrd | PCR 8/9 |
| RTMR[3] | application / runtime | — |

(The TDX `MRTD` + `RTMR[0..3]` required-together rule is the **Intel TDX kit's** VP2 — see `kit-intel-sgx-tdx.md`; here it is the CCEL replay tie-in only.)

## 4. Verifier-policy specifics — the per-vendor fill-ins

| method dimension | TPM 2.0 / standards specific | cite |
|---|---|---|
| VP1 freshness | `extraData` (`qualifyingData`) in `TPMS_ATTEST` carries the verifier-issued nonce; confirm `extraData == nonce` issued by the verifier | §4.1, §4.3 step 5 |
| VP2 binary binding | the selected PCR set (via `pcrDigest` + event-log replay) matches governance-approved golden PCRs / RIM; **never gate on PCR 16 or 23**; name the required PCRs | §2.1, §4.3, claim 2/3 |
| VP3 debug-mode | **PCR 16 is the debug register and is resettable from any locality — never include it in a trust decision** (TPM has no single "debug bit"; the analogue is refusing untrusted/resettable PCRs). EAT `dbgstat` is the cross-vendor claim a Verifier maps debug state into (`enabled`/`disabled`/`permanently-disabled`/`fully-disabled`) | §2.1, §2.3, claim 2; §6.1 |
| VP4 anti-rollback | **N/A — no TCB integer.** TPM has no signed reportable TCB version like AMD `REPORTED_TCB` / Intel `tcbStatus`. Firmware/OS freshness is expressed through the *measured* PCR values vs current RIM, not a monotonic TCB field. See `kit-amd-sev-snp.md` / `kit-intel-sgx-tdx.md` for TCB-integer anti-rollback | §4 (no TCB field) |
| VP5 cert-chain / generation | walk **AK cert → (Privacy-CA binding OR `TPM2_ActivateCredential` proof) → EK cert → TPM-manufacturer CA**; **direct EK signing of arbitrary data is prohibited** by the EK template (`restricted`+`decrypt`-only), so the AK is what signs Quotes | §1.4, §4.3 step 1, claim 4 |
| VP6 key isolation | → §6 (EK/AK live in the Endorsement Hierarchy; SRK in the Storage Hierarchy; seeds never leave the TPM) | §6 |
| VP7 revocation | → profile (governance-driven revocation in the on-chain reference-value registry / RVP) | profile §3 |
| VP8 joint-attester | **N/A for the host TPM itself** (single attester). The **SPDM** device-attestation channel (§9.3) is where joint CPU+GPU binding lives — see `kit-nvidia-cc.md` (SPDM session key in CPU `REPORT_DATA` on Hopper; TDISP/IDE on Blackwell) | §9.3, claim 9 |
| VP9 **policy separation (central)** | **Parse heterogeneous vendor Evidence (`TPM2_Quote`/SNP report/TDX quote/Nitro COSE) into a *normalized claim set*, then apply policy (acceptable PCR/measurement values, revoked images) as a *separate* layer. Provider owns the format; verifier owns the policy.** The RATS roles exist precisely to enforce this separation — do not hard-code vendor parsing into the policy layer | method VP9 + §2; claim 10; profile §6 |
| VP10 advisories | **N/A — no advisory-ID field** in `TPMS_ATTEST` (that is Intel `advisoryIDs` — see `kit-intel-sgx-tdx.md`). EAT surfaces it cross-vendor via `swversion`/`measres` at the result layer | §4; §6.1 |
| VP11 known-CVE bits | **N/A — no platform-info / mitigation bit** (that is AMD BadRAM `PLATFORM_INFO.ALIAS_CHECK_COMPLETE` — see `kit-amd-sev-snp.md`) | §4 (no such field) |
| VP12 host-controlled-but-signed | **N/A** — `TPMS_ATTEST` has no host-supplied-but-unmeasured field analogous to Nitro `user_data` / AMD `HOST_DATA`; `extraData` is the verifier's nonce (VP1), not host input. EAT `bootseed`/`location` are host-asserted claims a Verifier must treat as input, not enclave behavior | §4.1; §6.1 |
| VP13 version pinning | pin the accepted `TPMS_ATTEST` `type` + AK signature-algorithm set; reject downgrade | §4.1–4.2 |
| VP14 privacy / fingerprint | the **EK is a permanent device identifier** (intrinsic to the chip, like a serial number — derived from the never-changing EPS). **Mitigation = the Privacy-CA / AK indirection**: prove AK-in-same-TPM-as-EK via `TPM2_MakeCredential`/`ActivateCredential` so the verifier trusts the Quote without the EK appearing on the wire. Raw on-chain publication of EK certs is a permanent ledger-wide fingerprint — prefer fleet-scoped / ZK (cross-vendor list: TPM EK, AMD `CHIP_ID`, Nitro `module_id`, NVIDIA PDI) | §1.3–1.4, claim 1/4; claim 13 |
| VP15 registry integrity | → profile (multisig + time-lock + transparency via **CoRIM public RVP** + emergency revocation on the reference-value registry) | profile §3, §6 |
| VP16 **cross-vendor delta (central)** | **trust set = TPM silicon manufacturer (per-vendor CA: Infineon/ST/Nuvoton/MS vTPM).** This kit is the anchor that explains why `tee_type` is **not a fungible switch**: each vendor roots in a *different* trust set (TPM mfr CA vs AMD KDS vs Intel PCS vs AWS PKI vs NVIDIA NRAS), produces a *different* Evidence format, and exposes *different* fields — a multi-vendor Verifier MUST surface *which* trust set applies per attestation. **CoRIM/RATS are how you normalize across them**: RFC 9334 gives the shared role vocabulary, CoRIM gives the shared reference-value manifest | §5.3, §10; method VP16; profile §6 |

## 5. On-chain verification (Sei)

**TPM 2.0 is not a typical direct-on-chain Sei attester target — frame it as the standards / host-integrity anchor, not a Sei on-chain Attester.** The research doc treats TPM as the *cross-vendor grounding* layer (§ audience line; §10 "Notable cross-vendor observations": the host TPM is the canonical baseline that the on-chain CVM attesters reuse primitives from), not as a Sei-bound on-chain verification target. The Sei cost ranking in `method.md` and `trusted-execution-on-sei.md` §decision-driver covers **AMD SEV-SNP / Intel TDX / AWS Nitro / NVIDIA CC** — **it does not list TPM 2.0**, and the research doc gives **no TPM-specific Sei gas figure**. Do not invent one.

Realistic posture:

- **TPM's role here is host-integrity attestation + the standards layer**, consumed by an **off-chain Verifier** (RATS Background-Check) that appraises the `TPM2_Quote` (signature + event-log replay + RIM match — §4.3) and issues an Attestation Result. On Sei, the on-chain Attesters are the CVM platforms (TDX/SEV-SNP/Nitro), whose CCEL event logs reuse the TPM event-log replay machinery (§8, load-bearing claim 7).
- **If** a TPM Quote ever needs on-chain appraisal, the signature is ECDSA P-256/P-384 or RSASSA (§4.2): a P-256 AK maps to Sei's `P256VERIFY` precompile at `0x1011` (`48,000 gas/verify`, profile §1), but the dominant cost is event-log replay + AK→EK→manufacturer chain validation, which has no precompile — making the verify-once-then-`ecrecover` amortization pattern (profile §1, `kit-aws-nitro.md` §5) the only realistic steady-state path. **This is engineering inference, not a measured figure from the research — flag it as a gap if a number is needed.**

## 6. Key-release / integration pattern (VP6)

TPM key isolation is **hierarchy-rooted**, and the hierarchies are **non-fungible** (load-bearing claim 1):

- **EK** — Primary Object in the **Endorsement Hierarchy** (rooted in the EPS, provisioned at manufacture, never changes → the EK is the chip's intrinsic identity). Privacy-sensitive; `restricted`+`decrypt`-only — it does **not** sign arbitrary attestations (§1.3).
- **AK** — restricted signing key under the Endorsement (or Storage) Hierarchy; signs Quotes/Certifications. Earns EK-equivalent trust via the **`TPM2_MakeCredential`/`TPM2_ActivateCredential`** Privacy-CA credential, **not** by being the EK (§1.4, claim 4).
- **SRK** — Primary Object in the **Storage Hierarchy** (rooted in the SPS, owner-controlled); the wrapping root for user storage keys (§1.3).
- **Seeds never leave the TPM**; primaries are *derived deterministically* from `seed + template`, so the EK/SRK can be recreated without persistent storage (§1.2–1.3). Long-lived keys (EK, SRK, a persistent AK) are pinned in NV via `TPM2_EvictControl` (handle range `0x81000000..0x81FFFFFF`); most working keys are transient and lost on `TPM2_FlushContext` / reset (§1.5).

The correct-by-construction pattern: **gate secret release on a `TPM2_Quote`/`TPM2_Certify` over a known-good PCR set**, with the signing AK proven (via `ActivateCredential`) to reside in the same TPM as a manufacturer-endorsed EK — the secret unseals only to a key bound inside the measured boundary. **Designs must state which hierarchy each key lives in** (load-bearing claim 1).

## 7. Citations

- **Ground truth:** `bdchatham-designs/designs/sei-agentic-mesh/research/tee/tpm-2.0-open-standards.md` §1 (architecture, hierarchies, EK/AK/SRK, AK certification, object lifetime), §2 (PCRs, extend, reset), §3 (RTM/SRTM/DRTM, `TPM2_Quote` vs `TPM2_Certify`), §4 (`TPMS_ATTEST`, sig algorithms, AK cert chain), §5 (RATS RFC 9334 roles + topologies), §6 (EAT RFC 9711 claims + profiles), §7 (RIM/CEL/TAP), §8 (CCEL + RTMR mapping), §9 (DICE/DPE/SPDM), §10 (cross-vendor mapping) + load-bearing claims 1–10.
- **Profile:** `bdchatham-designs/designs/sei-agentic-mesh/research/tee/trusted-execution-on-sei.md` §open-standards + load-bearing claims 5, 6, 7, 10, 12, 13; `references/tee-profile.md` §3 (registry-as-RVP), §6 (RATS/EAT/CoRIM alignment). The research doc provides **no TPM-specific Sei gas figure** (§5 above) — that is a stated gap, not an omission.
- **Sibling kits:** `kit-intel-sgx-tdx.md` (TDX `MRTD`+`RTMR[0..3]` VP2, `tcbStatus` VP4, `advisoryIDs` VP10), `kit-amd-sev-snp.md` (`REPORTED_TCB` VP4, BadRAM VP11, `CHIP_ID` VP14), `kit-aws-nitro.md` (`user_data` VP12, `module_id` VP14, amortization §5), `kit-nvidia-cc.md` (SPDM joint-attester VP8).
- **Primary sources:** TCG TPM 2.0 Library Specification Parts 1–4 (`trustedcomputinggroup.org/resource/tpm-library-specification/`); TCG PC Client Platform Firmware Profile (PFP) — PCR usage + reset locality; TCG RIM Information Model; TCG Canonical Event Log Format / RFC 9393 (CoSWID); RFC 9334 (RATS architecture, 5 roles); RFC 9711 (EAT, April 2025 — verifier-output claim envelope); DMTF DSP0274 (SPDM); TCG DICE Layering Architecture + DICE Protection Environment (DPE); UEFI 2.10 §38 (`EFI_CC_MEASUREMENT_PROTOCOL`) for CCEL; CoRIM (Concise Reference Integrity Manifest — emerging cross-vendor reference-value format).
