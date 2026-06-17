# Sei on-chain verification-layer kit

> Ground truth: `design/research/tee/trusted-execution-on-sei.md`. Every claim below cites it (§ or load-bearing claim #), the `tee-profile.md` overlay, a vendor spec, or an RFC. Do not paraphrase — cite.
>
> **This kit is NOT an attester platform kit.** It describes the **Verifier-side / on-chain verification LAYER** — the Sei-EVM contract that appraises vendor-native Evidence against policy. It has no Evidence of its own, no measurement registers, and no single trust root: it **inherits** the trust set of whichever attester kit produced the Evidence it is verifying. Where the §1–§7 template is shaped for an Attester, this kit adapts the section and says why. The per-vendor attester kits are `kit-aws-nitro.md`, `kit-amd-sev-snp.md`, `kit-intel-sgx-tdx.md`, `kit-nvidia-cc.md` (siblings); load the matching one alongside this kit per method Step 3.

## 1. Identity & RATS roles

This layer is the **Verifier**, not the Attester. It produces no Evidence and holds no enclave identity of its own.

- **What it is** — a Sei-EVM contract (or contract set) that consumes vendor-native Evidence bytes, appraises them against a policy + an on-chain reference-value registry, and returns a verdict the Relying Party gates on. It is the on-chain realization of the RATS Verifier (`trusted-execution-on-sei.md` §open-standards, load-bearing claim 7).
- **RATS role mapping** — **Verifier** = this on-chain contract; **Relying Party** = the Tide/Sei contract that consumes the verdict to gate escrow / ordering / mint (TideCouncil, TideJobHook, Sei consensus) (`trusted-execution-on-sei.md` §open-standards; `tee-profile.md` §6); **Attester** = the in-scope TEE (its kit); **Endorser** = that vendor's PKI (its kit §1); **Reference-Value-Provider** = governance / TideCouncil via the on-chain registry (`tee-profile.md` §3, load-bearing claim 10).
- **Trust root / Endorser — there is NO single trust root for this layer.** It **inherits** each attester kit's trust set: Nitro = AWS hypervisor + AWS PKI (`kit-aws-nitro.md` §1); AMD/Intel = silicon-vendor PKI; NVIDIA = NRAS + NVIDIA PKI (`trusted-execution-on-sei.md` load-bearing claim 9). A multi-vendor Verifier MUST surface **which** trust set applies per attestation — `tee_type` is not a fungible switch (VP16; load-bearing claim 9). See each attester kit's §1 and VP16 below.
- **Ground-truth doc** — `design/research/tee/trusted-execution-on-sei.md`.

**Template-fit note:** the template's §1 "Endorser / trust root to pin" and "what it protects" framing is Attester-shaped. For the Verifier the answer is *inherited, plural, and per-attestation* — captured above as VP16's central concern. The Verifier's own correctness depends not on a pinned silicon root but on the **registry** that defines acceptable measurements (§3, VP15) — registry compromise, not a leaked attester key, is this layer's highest-leverage attack (`tee-profile.md` §3, load-bearing claim 10).

## 2. Evidence format — what the on-chain Verifier consumes

This layer consumes **vendor-native Evidence bytes directly**, never EAT. EAT (RFC 9711) is the Verifier's *output* format, not its on-chain input; feeding EAT to an on-chain verifier is a category error (`tee-profile.md` §6; `trusted-execution-on-sei.md` §open-standards, load-bearing claim 7).

What lands on-chain as input, per attester (parse detail in each attester kit §2):

- **AMD SEV-SNP** — the SNP attestation report bytes (single P-384 + SHA-384 verify) (`trusted-execution-on-sei.md` §decision-driver table; `kit-amd-sev-snp.md` §2).
- **Intel SGX/TDX (DCAP v4)** — the DCAP quote bytes (P-256 + SHA-256; 5–8 verifies: AK + 3-layer PCK chain + TCB Info + QE Identity) (`trusted-execution-on-sei.md` §decision-driver; `kit-intel-sgx-tdx.md` §2).
- **AWS Nitro** — the COSE_Sign1 `AttestationDocument` bytes (P-384, raw `r || s` 96-byte sig, `external_aad = h''`) (`kit-aws-nitro.md` §2).
- **NVIDIA CC** — the multi-verify cert-chain + AK + RIM Evidence (P-384) — direct on-chain not viable; arrives via ZK or relayer (`trusted-execution-on-sei.md` §decision-driver; `kit-nvidia-cc.md` §2).

**The core primitive is the Sei P-256 precompile** at `address(0x0000000000000000000000000000000000001011)` (`0x1011`): `GasCostPerByte = 300`, 160-byte input → **`300 × 160 = 48,000 gas per verify`** (verified against `sei-chain/precompiles/p256/p256.go:24-25`; `tee-profile.md` §1, load-bearing claim 1). Below Solidity P-256 (~200k), above EIP-7951's flat ~6k. It is the workhorse for **Intel DCAP** (P-256) and for the **steady-state secp256k1-adjacent** path; **AMD/Nitro/NVIDIA are P-384**, for which Sei has **no native precompile** — P-384 verifies are Solidity/Yul (§5).

**VP13 version pinning is the Verifier's job here:** the Verifier MUST pin accepted Evidence-format versions (Intel Quote v3/v4/v5; AMD report v2/v3/v5; Nitro evolving CDDL) and reject downgrade — version confusion is a known DCAP-verifier-bug class (`trusted-execution-on-sei.md` "Critical defense-in-depth" #10, load-bearing claim 8).

**Template-fit note:** §2 normally describes *the wire format an Attester produces*. Here it is reframed as *the set of vendor-native formats the Verifier must parse* — one parser per in-scope attester, each citing that attester's kit §2. The Verifier adds nothing to the wire format; its §2 contribution is the **parse-fan-in + the precompile primitive + version pinning**.

## 3. Reference values into the chain — NOT identity-measurement

**N/A as identity-measurement.** This layer produces no measurement registers (no PCR0 / MRTD / MEASUREMENT of its own) — those are attester-kit concerns. For binary-identity field semantics, redirect to the attester kits: Nitro PCR0/1/2 (`kit-aws-nitro.md` §3); TDX `MRTD` + `RTMR[0..3]` together — MRTD-alone is exploitable (`trusted-execution-on-sei.md` load-bearing claim 3; `kit-intel-sgx-tdx.md` §3); AMD `MEASUREMENT` (`kit-amd-sev-snp.md` §3).

What this layer *does* own is **how reference values ENTER on-chain** — the registry side of VP2:

- The on-chain image-hash registry (a TideJobHook / TideCouncil registry of approved PCR0 / MRTD / MEASUREMENT values) **is the RATS Reference-Value-Provider** (`tee-profile.md` §3, load-bearing claim 10).
- At verify time the Verifier extracts the attester's measurement from parsed Evidence (§2) and checks it for **membership** against the registry's current approved set (VP2-match), and **non-membership** against the revoked set (VP7) (`tee-profile.md` §3).
- Reference values SHOULD be CoRIM-formatted for cross-vendor composability and transparency (`tee-profile.md` §6, load-bearing claim 12).
- The registry **storage layout is a one-way door** (method Rule 4): once live, changing it is a migration / hard fork — flag, never silently assert a layout (`tee-profile.md` §3).

**Template-fit note:** the template's §3 "what proves binary identity, the registers, sizes, required-together rule" is intrinsically an Attester property. The Verifier has none, so §3 is **redirected** to the attester kits and **repurposed** to the one identity-adjacent thing the Verifier owns: the registry the reference values flow into. This is the deliberate adaptation the instructions call for, not a gap.

## 4. Verifier-policy specifics — what THIS layer enforces vs. attester-kit specifics

The split: a dimension is **ENFORCED here** when the on-chain Verifier is the component that performs the check at appraisal time; it is an **attester-kit specific** when the per-vendor field/offset/bit detail lives in the attester kit and this layer only consumes it. VP7/VP15 (registry/governance) are **central to this kit**. IDs verbatim per method.

| method dimension | enforced-here vs. attester-specific | cite |
|---|---|---|
| VP1 freshness | **ENFORCED here** — the Verifier issues the nonce and binds it via on-chain entropy (block number / `blockhash` / a contract-issued challenge) written into the attester's freshness channel (Nitro `nonce`; AMD `REPORT_DATA` `0x050`; Intel `REPORTDATA`); reject if the echoed nonce is stale or unmatched | `trusted-execution-on-sei.md` "Critical defense-in-depth" #3, load-bearing claim 8; attester kit §4 for the field |
| VP2 binary binding | **ENFORCED here** — measurement parsed from Evidence (§2) MUST be a member of the registry's current approved set; the field/required-together rule is attester-specific (§3 → attester kits) | `tee-profile.md` §3; `trusted-execution-on-sei.md` load-bearing claim 10; attester kit §3 |
| VP3 debug-mode | **ENFORCED here** — reject the debug indicator: Nitro all-zero PCRs; AMD `POLICY` bit 19 (`DEBUG_ALLOWED`); Intel `ATTRIBUTES.DEBUG`; the *bit location* is attester-specific | `trusted-execution-on-sei.md` "Critical defense-in-depth" #1, load-bearing claim 8; attester kit §4 |
| VP4 anti-rollback | **ENFORCED here** — the Verifier enforces `reported_TCB >= minimum_acceptable`, rejects `Revoked`, applies explicit policy on out-of-date (vendors sign reports for *any* historical TCB); Nitro: leaf-cert validity vs document `timestamp`, not wall-clock. Minimum-TCB policy is a registry/governance value | `trusted-execution-on-sei.md` "Critical defense-in-depth" #2, load-bearing claim 8; attester kit §4 |
| VP5 cert-chain / generation | **ENFORCED here** — validate the full Endorser chain to the pinned root for the attester in hand; the chain shape/order/per-gen cert selection is attester-specific (AMD VCEK per-chip+TCB; Intel 3-layer PCK; Nitro `cabundle` order + Root-G1) | `trusted-execution-on-sei.md` "Critical defense-in-depth" #4, load-bearing claim 8; attester kit §4 |
| VP6 key isolation | → §6 (the steady-state registered-key amortization is this layer's key-binding pattern); per-attester secret-release (e.g. Nitro KMS condition keys) is attester-specific | §6; attester kit §6 |
| VP7 revocation | **CENTRAL here** — the Verifier checks the measurement against the registry's revoked set; a disclosed-bad image MUST stop passing the instant governance revokes it. Emergency revocation path is a registry mandate | `tee-profile.md` §3, load-bearing claim 10 |
| VP8 joint-attester | **attester-specific (NVIDIA)** — when CPU+GPU co-attest the Verifier checks the *binding* (SPDM session key in CPU `REPORT_DATA` on Hopper; TDISP/IDE on Blackwell), not the two reports independently; binding mechanism in the NVIDIA kit | `trusted-execution-on-sei.md` "Critical defense-in-depth" #5, load-bearing claim 6; `kit-nvidia-cc.md` §4 |
| VP9 policy separation | **ENFORCED here (this is the layer's organizing principle)** — parse each vendor's Evidence (§2) into a normalized claim set, then apply policy (acceptable measurements, min TCB, revoked images) as a *separate* layer; don't hard-code vendor parsing into policy | `trusted-execution-on-sei.md` "Critical defense-in-depth" #6, load-bearing claim 8; method VP9 |
| VP10 advisories | **ENFORCED here (Intel)** — surface Intel `advisoryIDs` to the relying-party policy layer; don't silently accept `OutOfDate` (the SGAxe failure mode); the field is Intel-specific | `trusted-execution-on-sei.md` "Critical defense-in-depth" #7, load-bearing claim 8; `kit-intel-sgx-tdx.md` §4 |
| VP11 known-CVE bits | **attester-specific (AMD)** — require AMD `PLATFORM_INFO.ALIAS_CHECK_COMPLETE` (AMD-SB-3015 / BadRAM) for chips in the affected window; the bit is AMD-specific; the Verifier enforces it when verifying an AMD report | `trusted-execution-on-sei.md` "Critical defense-in-depth" #8, load-bearing claim 8; `kit-amd-sev-snp.md` §4 |
| VP12 host-controlled-but-signed | **ENFORCED here (as a policy refusal)** — any Verifier policy gating on Nitro `user_data` or AMD `HOST_DATA` (signed but NOT measured) needs a *separate* trust assumption; treat these as host-input evidence, not enclave behavior; the field is attester-specific | `trusted-execution-on-sei.md` "Critical defense-in-depth" #9, load-bearing claim 8; attester kit §4 |
| VP13 version pinning | **ENFORCED here** — pin accepted Evidence-format versions per vendor and reject downgrade (Intel Quote v3/v4/v5; AMD report v2/v3/v5; Nitro CDDL); the version set is attester-specific (§2) | `trusted-execution-on-sei.md` "Critical defense-in-depth" #10, load-bearing claim 8; §2; attester kit §2 |
| VP14 privacy / fingerprint | **policy hygiene, surfaced here** — raw Evidence on-chain is a permanent ledger entry exposing device-unique IDs (AMD `CHIP_ID`, Nitro `module_id`, NVIDIA PDI); for validator-as-attester prefer fleet-scoped creds (AMD VLEK) or ZK that hides device-unique fields. The Verifier should accept the ZK/relayer form to avoid the leak (§5) | `trusted-execution-on-sei.md` "Critical defense-in-depth" #11, load-bearing claim 13 |
| VP15 registry integrity | **CENTRAL here** — the on-chain reference-value registry MUST carry multisig on writes, time-locks on changes, transparency (CoRIM / public RVP), and an emergency revocation path; registry compromise is the **highest-leverage attack** on the whole scheme | `tee-profile.md` §3, load-bearing claim 10 |
| VP16 cross-vendor delta | **CENTRAL here** — a multi-vendor Verifier MUST surface *which* trust set applies per attestation (Nitro: AWS hypervisor + PKI; AMD/Intel: silicon vendor; NVIDIA: NRAS + PKI); `tee_type` is not a fungible switch; dispatch on attester type carries the trust delta to the relying party | `trusted-execution-on-sei.md` load-bearing claim 9; §1; attester kit §1 |

## 5. On-chain verification (Sei) — the core of this kit

The full Sei-EVM cost ranking, cold per-attestation (from `trusted-execution-on-sei.md` §decision-driver + `tee-profile.md` §1, load-bearing claim 1; the P-256 precompile at `0x1011` = `48,000 gas/verify`):

| Attester | Scheme | Sei EVM cost (cold) | Path |
|---|---|---|---|
| **AMD SEV-SNP** | P-384 single verify | **~1.5–2M gas** Solidity P-384 (no native precompile), ~250k ZK | **direct** — cheapest direct on-chain on Sei |
| **Intel SGX/TDX (DCAP)** | P-256, 5–8 verifies | **~3.5–4M gas** via P256VERIFY (5–8 × 48k ≈ 240–384k verifies + ~3M parse/quote overhead) | **direct** — competitive; multi-cert overhead dominates |
| **AWS Nitro** | P-384, COSE_Sign1 | **<70M cold / <20M warm** direct, OR **~3k amortized** | **amortized** (direct too expensive steady-state) |
| **NVIDIA CC** | P-384, multi-verify | **~100M+ direct**, ~200k ZK, ~3k relayer | **ZK-proven** or trusted relayer only |

The three paths the Verifier supports:

1. **Direct** — verify the full Evidence on-chain every time. Viable for **AMD** (~1.5–2M gas, single P-384 in Solidity/Yul) and **Intel** (~3.5–4M gas via the `0x1011` P256VERIFY; the per-byte schedule narrows what would be a bigger Intel advantage under EIP-7951's flat ~6k). **Not** viable for Nitro (<70M cold) or NVIDIA (~100M+) (`trusted-execution-on-sei.md` §decision-driver, load-bearing claims 1–2).

2. **Amortized (Marlin Oyster, the steady-state posture — §6)** — verify **one** "verifier-enclave" attestation on-chain once (e.g. Nitro ~63M cold at registration), then accept **secp256k1**-signed statements from that enclave via the `ecrecover` precompile at **~3k gas/msg** for every subsequent attestation. The only realistic steady-state for Nitro, and the high-volume path for non-Intel TEEs generally (`trusted-execution-on-sei.md` §decision-driver, application-categories #4/#6, load-bearing claim 2; `kit-aws-nitro.md` §5).
   - **Witness-key-freshness concern (load-bearing, `trusted-execution-on-sei.md` application-categories #4 + load-bearing claim 2):** the amortized path reuses a secp256k1 binding key across many messages. Without (a) **per-message sequence numbers** in each witness signature, (b) **binding-key rotation** driven by the enclave on TCB / image change, and (c) an **on-chain validity window** tying the binding key to its underlying attestation registration, a stolen binding key **outlives the attestation it was minted under** and persists past TCB rotation / image upgrade. The Verifier MUST enforce all three.

3. **ZK-proven (~200–500k gas)** — the prover proves "I am a registered TEE attester" inside the SNARK; the Verifier checks the SNARK + that the proven attestation root is registered. The only practical path for **NVIDIA** (~200k ZK vs ~100M+ direct) and for high-frequency provers; also AMD's ~250k ZK option and the privacy-preserving form for VP14 (hides device-unique fields while preserving the attested claim) (`trusted-execution-on-sei.md` §decision-driver, application-categories #3, load-bearing claims 1/13).

**Prior art — Automata Network** is the production on-chain exemplar of paths 1+3 for **Intel**: an audited (Trail of Bits, Feb 2025), ~22-chain on-chain Intel-DCAP verifier (V3/V4/V5 + on-chain PCCS) with a zkVM path (~493–569k gas) that also extends on-chain coverage to **AMD SEV-SNP** (zkVM-only). Its numbers corroborate the Intel/AMD rows above. Caveats for a Sei adoption: **not deployed on Sei**; the on-chain PCCS deploys **per-chain** (a deploy + TCB/QE-collateral-maintenance commitment); the zkVM path needs an off-chain prover (Bonsai/SP1). The corpus's **Marlin Oyster secp256k1** pattern (path 2 / §6) remains the answer for **Nitro** — Automata does not cover Nitro/NVIDIA on-chain. Full treatment: `design/research/tee/automata-onchain-attestation.md`.

**Gas is one input, not the decision** — trust-model fit (validator-as-host, fingerprint exposure, ecosystem maturity, operational footprint) dominates path choice; state the ranking, don't let it pre-empt the trust-model analysis (`tee-profile.md` §1, method Rule 1).

## 6. Registered-key / amortization integration pattern (VP6)

The steady-state on-chain posture for this layer is the **registered-key amortization** pattern, not per-operation full verification (`trusted-execution-on-sei.md` application-categories #4 + #6, load-bearing claim 2):

1. **Register** — the attester presents one full Evidence (Nitro COSE_Sign1 / TDX quote / SNP report) to the Verifier on-chain *once*. The Verifier runs the full §4 checklist (freshness, measurement-in-registry, debug, TCB, cert-chain, version) and, on pass, **binds a fresh secp256k1 key** carried in the attestation's user/report-data channel to the verified measurement, recording it with a **validity window** tied to the attestation's TCB/registration.
2. **Steady state** — subsequent submissions are **secp256k1-signed** by that registered key; the Relying Party gates on `ecrecover` (~3k gas) + a check that the recovered key is currently within its validity window and not revoked. No per-message full re-verification.
3. **Rotation / revocation** — the binding key is **rotated** by the enclave on every TCB update / image change; old windows close; governance can revoke a key (VP7) the instant a measurement is disclosed-bad. Each message carries a **sequence number** so a replayed or out-of-window signature is rejected (the witness-key-freshness mandate, §5).

For AWS-hosted agents/bridges this composes with the attester-side KMS condition-key release (`kms:RecipientAttestation:ImageSha384`) so the secret is unsealed only to the attested image off-chain, while the on-chain Verifier gates the registered key (`kit-aws-nitro.md` §6, `tee-profile.md` §4). The on-chain registered-key path is the **correct-by-construction** on-chain steady state; full direct verification (§5 path 1) is the fallback for low-volume AMD/Intel where it is cheap enough.

**Template-fit note:** §6 normally describes an Attester's *secret-release* pattern (e.g. KMS condition keys). For the Verifier the idiomatic pattern is the *verify-once-then-bind* amortization — the on-chain counterpart that makes the registered key trustworthy. Attester-side key release is redirected to the attester kits' §6.

## 7. Citations

- Ground truth: `design/research/tee/trusted-execution-on-sei.md` — §decision-driver (cost ranking + "gas is one input"), §application-categories #1–#6 (validator signing, MEV ordering, ZK proving, bridges, confidential mempool, Tide agent runtimes), §trust-roots, §open-standards (RATS/EAT/CoRIM), "Critical defense-in-depth" #1–#11, load-bearing claims 1–2 (precompile + Marlin amortization), 7 (EAT is verifier-output), 8 (verifier-policy MUSTs), 9 (cross-vendor trust-set deltas), 10 (registry is the highest-leverage attack), 11 (TEE layers, doesn't replace), 12 (CoRIM), 13 (on-chain fingerprint privacy).
- Profile overlay: `tee-profile.md` §1 (Sei EVM economics + `0x1011`), §3 (registry as RVP, VP7/VP15), §5 (layering, reject over-claim), §6 (RATS roles, EAT-as-output, CoRIM).
- Sibling attester kits (the per-vendor Evidence/measurement/cert/key detail this layer consumes): `kit-aws-nitro.md` (§2 COSE_Sign1, §3 PCRs, §5 Marlin Oyster, §6 KMS), `kit-amd-sev-snp.md`, `kit-intel-sgx-tdx.md`, `kit-nvidia-cc.md` (siblings; load the matching one per method Step 3).
- Primary: `sei-chain/precompiles/p256/p256.go:24-25` (verified `0x1011` address + `GasCostPerByte = 300`); RFC 9334 (RATS roles); RFC 9711 (EAT — verifier-output, not on-chain input); Sei P256VERIFY docs (`docs.sei.io/evm/precompiles/p256-precompile`); Marlin NitroProver + Oyster (the amortization reference); Automata DCAP-on-chain contracts (Intel direct path) — full prior-art treatment in `design/research/tee/automata-onchain-attestation.md` (contracts, zkVM gas table, per-chain-PCCS + not-on-Sei caveats).
