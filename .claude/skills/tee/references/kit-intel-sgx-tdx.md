# Intel SGX / TDX (DCAP) kit

> Ground truth: `design/research/tee/intel-sgx-tdx.md`. Every claim below cites it (§ or load-bearing claim #), a vendor spec, or an RFC. Do not paraphrase — cite. This kit covers **both** Intel SGX (process-level enclave) and Intel TDX (VM-level CVM) over DCAP/ECDSA-P256; the two share a flow but differ on the identity surface (§3) and measurement hash — noted at each divergence.

## 1. Identity & RATS roles

- **What it is** — two boundaries, one attestation flow. **SGX** isolates a single user *process* into an enclave: a purpose-built, signed enclave binary whose exact bits are pinned by `MRENCLAVE` (§1.1, §4). **TDX** isolates a full guest VM (a Trust Domain / CVM) running an unmodified OS + workload, with launch image pinned by `MRTD` and runtime state in `RTMR[0..3]` (§2.3, §4). Decision rule: a specific code unit (signer, key vault, policy oracle) → SGX; a conventional container/VM (LLM agent runtime, unmodified Linux sidecar) → TDX (§4). Attestation is point-in-time (DCAP passport pattern, §5.1) — re-attest for freshness.
- **RATS role mapping** — **Attester** = the enclave/TD + QE/TDQE that signs the Quote (§3.1, §5.1); **Endorser** = Intel SGX PCS (issues PCK certs, PCK CRLs, TCB Info JSON, QE Identity JSON) (§3.1, §5.1); **Verifier** = the on-chain Solidity contract or an off-chain `quote-verification-library` instance (§5.1); **Relying Party** = the contract/service consuming the result; **Reference-Value-Provider** = the on-chain registry of allowed `MRENCLAVE`/`MRTD`/`RTMR[i]` values (profile, §5.1).
- **Trust root / Endorser** — pin the **Intel SGX Root CA** (self-signed, distributed out-of-band) (§3.3). **Trust set = the Intel silicon vendor PKI** — Intel is the single root for both the PCK chain *and* (via a separate TCB Signing CA chained off the same Root) the TCB Info / QE Identity (§3.3, §3.4). This is a silicon-vendor trust set, not a cloud-hypervisor one — feeds VP16.
- **Ground-truth doc** — `design/research/tee/intel-sgx-tdx.md`.

**Trust-model note (VP16 / profile Rule 2):** Intel's root is the silicon vendor, not a cloud host, so unlike Nitro the trust does not rest on a hypervisor operator. This makes SGX/TDX a candidate for a validator-as-host MEV/ordering surface where the relying party must not have to trust the box operator — but the analysis still turns on the workload fit (SGX tight binding vs TDX whole-VM, §4), not on gas alone (method Rule 1).

## 2. Evidence format

The wire format is a **DCAP Quote**: the enclave/TD produces a local report (`EREPORT` → `sgx_report_t`; TDX `TDCALL[TDG.MR.REPORT]` → `TDREPORT_STRUCT`), the QE/TDQE verifies its MAC and re-signs the report body with the platform ECDSA-P256 attestation key, emitting the Quote (§1.2, §2.1, §3.2). Signature scheme is **ECDSA over NIST P-256 + SHA-256** for every signature in the path — Quote sig, QE-report sig by PCK, PCK cert-chain sigs, TCB Info JWT sig (§6, load-bearing claim 4). This single curve/hash choice is what makes Intel DCAP cheaper on-chain than AMD's P-384 (§6).

- **SGX Quote v3 body** is an `sgx_report_body_t` (384 bytes); the variable `signature` payload carries, in order: ECDSA-P256 sig over the body (64 B, `r||s`), the attestation pubkey (64 B, `x||y`), the QE's own self-report (`sgx_report_body_t` with `REPORT_DATA = SHA-256(att_pubkey || qe_auth_data)`), the QE-report sig by the PCK leaf, QE auth data, and a certification-data blob (PCK cert chain in PEM) (§1.3).
- **TDX report** is `REPORTMACSTRUCT` (256 B, `MAC` = HMAC-SHA-256) + `TEE_TCB_INFO` + `TDINFO_STRUCT`; the body inside the Quote is a `td_report10` (584 B) or `td_report15` (648 B) (§2.2, §2.3, claim 9).
- **Quote v4** unifies SGX + TDX: a header consistent across versions (`version` 2 B, `attestationKeyType` 2 B, `teeType` 4 B, `qeVendorId` 16 B, `userData` 20 B) plus a `body_type` discriminator + `size`, so one verifier dispatches on `tee_type` to either an `enclave_report` (384 B) or a `td_report` (584/648 B). v4 **mandates certification-data type 6** (concatenated PEM PCK chain) (§1.4, claim 9).
- **Parser gotchas (the common verifier bugs):** signatures are **raw `r||s` (64 bytes)** in the Quote, not X.509 DER — convert before any OpenSSL-style API (§1.3). The PCK leaf carries non-standard X.509 extensions (`CPUSVN`, `PCESVN`, `FMSPC`) the verifier MUST parse out of the cert to select the right TCB Info entry — not from the report body (§3.3). The signed-over bytes for the Quote sig are the report body; `REPORT_DATA`/`REPORTDATA` (last 64 B of the body) is the only verifier-controlled field, everything else is measured/hardware-derived (§1.1, claim 7).
- **VP13 version pinning:** pin the accepted Quote version set `{v3, v4, v5}` and reject downgrade; v5 (Automata v1.1) adds QVE identity flexibility + finer TCB recovery (§1.3, §1.4, §7.1). Version confusion is a known DCAP verifier bug class (method VP13).

## 3. Identity & measurement fields (VP2)

**SGX** — measurements are **SHA-256, 32 bytes** (§1.1, claim 1):

| field | offset | size | binds |
|---|---|---|---|
| `MRENCLAVE` | 64 | 32 B | SHA-256 hash chain over `EADD`/`EEXTEND`/`EINIT` — the **exact enclave binary**; pinned at `EINIT`, never extends (§1.1, §2.4) |
| `MRSIGNER` | 128 | 32 B | SHA-256 of the signer's RSA-3072 pubkey modulus — *signer identity*, not the binary; "trust this signer" lets the signer ship upgrades (§1.1) |
| `ISVPRODID` / `ISVSVN` | — | 2 B each | ISV product id + security version; `MRSIGNER`-bound policy pins `ISVPRODID` and requires `ISVSVN >= N` (§1.1) |

SGX identity is fully captured at quote time by **`MRENCLAVE`** (or `MRSIGNER`+`ISVPRODID`+`ISVSVN`) (§2.4, claim 2).

**TDX** — measurements are **SHA-384, 48 bytes each** (§2.3, claim 1):

| field | size | binds |
|---|---|---|
| `MRTD` | 48 B | SHA-384 of initial TD memory + config; pinned at `TDH.MR.FINALIZE`, never extends (§2.3, §2.4) |
| `RTMR[0]` | 48 B | virtual HW env (vTPM, virtual firmware/ACPI), measured by TDVF/OVMF (§2.3) |
| `RTMR[1]` | 48 B | Linux kernel image (vmlinuz) (§2.3) |
| `RTMR[2]` | 48 B | kernel cmdline + initrd (§2.3) |
| `RTMR[3]` | 48 B | application-defined workload events (§2.3) |

`RTMR[i]` extend at runtime: `RTMR[i]_new = SHA384( RTMR[i]_old || SHA384(event) )` (§2.3).

**Required-together (load-bearing):** **`MRTD` alone is exploitable** — any kernel loadable into the same TDVF launch image looks identical at the `MRTD` layer. TDX identity REQUIRES `MRTD` **and** the relevant `RTMR[0..3]` policy values together; a `MRTD`-only policy is a finding (§2.4, claim 2). Reference values enter via the on-chain registry / governance RVP (profile, VP7/VP15).

## 4. Verifier-policy specifics — the per-vendor fill-ins

| method dimension | Intel's specific (SGX / TDX) | cite |
|---|---|---|
| VP1 freshness | verifier-issued nonce bound into `REPORT_DATA`/`REPORTDATA` (last 64 B of body), e.g. `SHA-512(ephemeral_pubkey \|\| nonce)` — the only verifier-controlled field | §1.1, claim 7 |
| VP2 binary binding | SGX: `MRENCLAVE` (or `MRSIGNER`+`ISVPRODID`+`ISVSVN`) matches a governance-approved reference value. TDX: `MRTD` **and** `RTMR[0..3]` together — `MRTD`-only is exploitable | §3 + §2.4, claim 2 |
| VP3 debug-mode | SGX: reject `ATTRIBUTES.DEBUG = 1` (offset 48; debug enclave is debugger-inspectable). TDX: reject the equivalent `TDINFO.ATTRIBUTES` DEBUG bit (8-byte attributes word) | §1.1, claim 5 / §2.3 |
| VP4 anti-rollback | resolve `tcbStatus` against PCS TCB Info JSON; **reject `Revoked`**; explicit relying-party policy on `OutOfDate` (do NOT silently accept) — vendors sign reports for any historical TCB. TDX adds `tdxtcbcomponents[]` (TDX Module SVN), same discipline | §3.4, §8, claim 6 |
| VP5 cert-chain / generation | 3-layer X.509 PCK chain: **Intel SGX Root CA → Processor *or* Platform CA → PCK leaf** (multi-package server → Platform CA; single-package → Processor CA); fetch all + validate to the pinned Root; read `FMSPC`/`CPUSVN`/`PCESVN` from the leaf extensions to select the TCB Info entry. TCB Info / QE Identity ride a **separate TCB Signing CA** chained off the same Root | §3.3, §3.4, claim 8 |
| VP6 key isolation | → §6 (verify-attestation-at-registration, then a registered enclave pubkey signs steady-state) | §6 |
| VP7 revocation | → profile (governance-driven revocation in the on-chain reference-value registry); Intel also publishes PCK CRL + Root CRL | profile + §3.3 |
| VP8 joint-attester | **N/A** — single attester. Joint CPU+GPU co-attestation is NVIDIA-specific | §3.1 |
| VP9 policy separation | parse the DCAP Quote (§2) into a normalized claim set (measurements, `tcbStatus`, `advisoryIDs`, `REPORT_DATA`); apply acceptable-measurement / min-TCB / revoked-image policy as a separate layer | method VP9 + §2 |
| VP10 advisories | surface Intel TCB Info **`advisoryIDs[]`** to the relying-party policy — **Intel is THE platform with this field**. Ignoring it is the SGAxe failure mode (accept a TCB with a live exploit) | §3.4, §8.1, claim 6 |
| VP11 known-CVE bits | **N/A** — no per-CVE platform-info mitigation bit (that is AMD BadRAM `PLATFORM_INFO.ALIAS_CHECK_COMPLETE`-specific); Intel handles named-CVE mitigation through TCB recovery + `tcbStatus`/`advisoryIDs` (VP4/VP10) instead | §8.1 |
| VP12 host-controlled-but-signed | **N/A** — no host-supplied signed-but-unmeasured field equivalent to Nitro `user_data` / AMD `HOST_DATA`. `REPORT_DATA` is enclave/TD-controlled (VP1), not host-controlled; TDX `MRCONFIGID`/`MROWNER*` are config IDs, not attacker-injected host bytes | §1.1, §2.3 |
| VP13 version pinning | pin Quote version set `{v3, v4, v5}`; reject downgrade; v4 mandates cert-data type 6 | §1.3, §1.4, §7.1 |
| VP14 privacy / fingerprint | the PCK leaf binds a per-platform `FMSPC`/`CPUSVN`/PPID tuple; raw on-chain quotes expose a platform-linkable cert — prefer the zk path (§5) or fleet-scoped registration for validator-as-attester patterns | §3.3, §7.2 |
| VP15 registry integrity | → profile (multisig + time-lock + transparency + emergency revocation on the reference-value registry) | profile |
| VP16 cross-vendor delta | trust set = **Intel silicon-vendor PKI** (single Root CA over both PCK and TCB Signing chains) — surface it when this kit sits beside a cloud-hypervisor (Nitro) or NRAS (NVIDIA) kit; `tee_type` is not a fungible switch | §1, §3.3 |

## 5. On-chain verification (Sei)

Sei EVM exposes **P256VERIFY** at `address(0x0000000000000000000000000000000000001011)`, priced `300 gas/byte × 160 bytes = 48,000 gas per verify` (`sei-chain/precompiles/p256/p256.go:24-25`; §7.3, claim 11) — cheaper than Solidity P-256 (~200k) but above EIP-7951's flat ~6k. Because DCAP uses P-256 + SHA-256 throughout (§6, claim 4), the precompile applies directly to every signature in the path.

- **Direct on-chain — ~3.5–4M gas per quote (cold):** an Automata-class verifier runs **5–8 P-256 verifies** (quote-body sig, QE-report sig by PCK, one per cert-chain layer) × 48k ≈ 240–384k for the verifies; the dominant cost is the **multi-cert-chain + parsing overhead** — X.509 DER parsing of the 3-layer PCK chain and TCB Info JSON keccak comparisons (§7.2, §7.3, claim 10). This is the **Intel SGX/TDX (DCAP) ~3.5–4M gas** line in the method's Sei cost ranking — competitive; multi-cert overhead dominates (`design/research/tee/trusted-execution-on-sei.md` decision-driver). Roughly half the on-chain cost of AMD SEV-SNP at the same security level, because P-256 < P-384 (§7.3).
- **ZK-proven — ~500k gas:** the Automata zkVM path (RiscZero Groth16 ~522k, SP1 Groth16 ~493k, SP1 Plonk ~569k) trades verifier complexity for off-chain proof generation, dropping on-chain cost to **~500k** (§7.2, claim 10).
- **PCK / TCB Info delivery** is the other cost driver: either a Tide-operated on-chain PCCS mirror (Automata pattern, pre-parses structured fields so runtime cost is keccak comparisons) or the zk-proof handoff (§7.2, §7.3).

Gas is one input, not the decision (method Rule 1): the Intel trust-set fit (§1, VP16) and SGX-vs-TDX workload fit (§4) dominate.

## 6. Key-release / integration pattern (VP6)

DCAP is point-in-time (passport pattern, §5.1) and a full on-chain verify is ~3.5–4M gas (§5), so the idiomatic Sei pattern is **registered-enclave-pubkey steady state**: verify the enclave/TD attestation **once at registration** (~3.5–4M gas), binding the enclave's ephemeral signing pubkey via `REPORT_DATA` (the only verifier-controlled field, §1.1 claim 7); thereafter the registered enclave signs steady-state statements that the relying party checks with a single **P256VERIFY at ~48k gas** (§5, claim 11). The plaintext signing key never leaves the enclave/TD (VP6); reference measurements (`MRENCLAVE`/`MRTD`+`RTMR`) gate registration via the on-chain registry RVP (profile, VP7/VP15).

**Witness-key freshness (carries from the method's amortization concern):** the registered pubkey is reused across messages — bind per-message sequence numbers, rotate the registered key on any TCB/image change (a `tcbStatus` transition or new `MRENCLAVE`/`MRTD`), and enforce an on-chain validity window, so a stolen registered key does not outlive the attestation it was minted under (method VP1/VP4; `design/research/tee/trusted-execution-on-sei.md` decision-driver).

## 7. Citations

- Ground truth: `design/research/tee/intel-sgx-tdx.md` §1 (SGX EREPORT/Quote v3/v4/v5), §2 (TDX TDREPORT, `MRTD`/`RTMR`, the §2.4 SGX-vs-TDX semantic gap), §3 (DCAP flow: PCS/PCCS/PCE/QE/TDQE, 3-layer PCK chain, TCB Info JSON `tcbStatus`/`advisoryIDs`, QE Identity), §4 (SGX vs TDX comparison), §5 (RATS RFC 9334 mapping, EPID deprecation EOL 2025-04-02, EAT profiles), §6 (crypto primitives — P-256/SHA-256, SHA-384 for TDX), §7 (on-chain gas + Sei P256VERIFY), §8 (Foreshadow/Plundervolt/SGAxe + TDX reviews) + load-bearing claims 1–11; `design/research/tee/trusted-execution-on-sei.md` decision-driver (Sei cost ranking).
- Primary: Intel SGX ECDSA Quote Library Reference (DCAP); Intel TDX DCAP Quoting Library API; Intel SGX DCAP Orientation Guide; Intel TDX Module 1.5 Base Architecture Spec (348549002); Intel PCS portal (`api.portal.trustedservices.intel.com/provisioning-certification`); Open Enclave `bits/sgx/sgxtypes.h` (ABI mirror); RFC 9334 (RATS); RFC 9711 + draft-kdyxy-rats-tdx-eat-profile-01 (EAT — Verifier *output*, not on-chain input); FIPS PUB 180-4 (SHA-256/384).
- Reference verifiers: `automata-network/automata-dcap-attestation` (Solidity v3/v4/v5, SGX+TDX) + `automata-network/automata-on-chain-pccs`; Phala `ts-sgx-quote-verify`; Intel `SGX-TDX-DCAP-QuoteVerificationLibrary`.
