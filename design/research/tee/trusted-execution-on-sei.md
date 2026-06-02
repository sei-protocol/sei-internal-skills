# Trusted execution on Sei

This document ties the per-vendor TEE attestation research (AMD SEV-SNP, Intel SGX/TDX, NVIDIA confidential compute, AWS Nitro Enclaves, TPM 2.0 + open standards) to concrete Sei threat models and named applications. It is the **integrative** piece of the TEE research corpus and the primary input to the `tee-specialist` agent when reasoning about Sei-bound TEE designs.

It is **not** a product proposal. It is the domain framing — what TEE patterns fit Sei's threat model, where each vendor's attestation lands operationally and economically on Sei, and what's open vs decided.

## Scope and what's out

**In scope.** TEE applications whose security depends on Sei consensus, Sei-EVM verification, sei-chain validator infrastructure, the sei-sidecar (seictl controller-to-pod task API on `:7777`), Waterway (the EVM JSON-RPC HTTP/WS reverse proxy upstream of the platform gateway), sei-k8s-controller-managed workloads, harbor cluster realities, and Tide's on-chain agentic harness contracts. The lens is "what does it cost to make an attestation load-bearing for a Sei-side decision."

**Out of scope.** TEE work that lives entirely off-chain with no Sei-side claim (general AWS Nitro enterprise workloads, Intel SGX for non-blockchain secrets management, etc.) — these are well-covered in the per-vendor docs and don't earn additional Sei-domain framing. Also out of scope: vendor selection at the *product* level (which TEE for which Sei feature) — that's a downstream design pass that should consume this document, not a part of it.

## The decision-driver: Sei EVM verification cost picture

Per [Sei docs](https://docs.sei.io/evm/precompiles/p256-precompile) and verified directly against `sei-chain/precompiles/p256/p256.go:24-25`, **Sei EVM has a P-256 precompile** (RIP-7212-equivalent surface):

- Address: `0x0000000000000000000000000000000000001011` (i.e., `0x1011`)
- Gas: `GasCostPerByte = 300`, input is 160 bytes → **48,000 gas per verify**

Meaningfully cheaper than Solidity P-256 (~200k/verify) but materially more expensive per-verify than EIP-7951's flat ~6k on Ethereum mainnet — Sei's per-byte schedule makes DCAP cost on Sei land between the two.

Combined with the per-vendor research, the on-chain verification cost picture on Sei is:

| Attester | Signature scheme | Per-attestation Sei EVM cost | Strategy |
|---|---|---|---|
| **AMD SEV-SNP** | ECDSA P-384 + SHA-384 (single verify) | **~1.5–2M gas** Solidity P-384, OR ~250k gas via Risc0/SP1 ZK proof | Cheapest direct on-chain on Sei due to single P-384 verify simplicity. No native P-384 precompile. |
| **Intel SGX / TDX (DCAP v4)** | ECDSA P-256 + SHA-256 (5–8 verifies: AK + 3-layer PCK chain + TCB Info + QE Identity) | **~3.5–4M gas** with Sei P256VERIFY (5–8 verifies × 48k ≈ 240–384k for verifies + ~3M parsing/quote overhead). ~5M without precompile. ~500k via Automata zkVM. | Sei's per-byte P256VERIFY narrows the cost gap that EIP-7951's flat fee would have widened. Multi-cert-chain overhead dominates. |
| AWS Nitro | ECDSA P-384 + SHA-384 (COSE_Sign1, 4–5 verifies) | <70M gas cold / <20M warm full on-chain (Marlin NitroProver verbatim), OR ~3k gas via secp256k1-recoverable signature from a verified Nitro relay | Direct on-chain is too expensive for steady-state. Use Marlin Oyster pattern: verify one "verifier enclave" attestation on-chain, then accept secp256k1 attestations from that enclave via `ecrecover`. |
| NVIDIA H100 / Blackwell | ECDSA P-384 + SHA-384 (multiple verifies: leaf cert chain + AK signature + RIM signature) | ~100M+ gas direct on-chain, OR ~200k gas via ZK-proven attestation, OR ~3k gas via trusted relayer (secp256k1) | Direct on-chain not viable. ZK-proven attestation or relayer pattern only. |

**The takeaway, corrected:** AMD SEV-SNP is cheapest direct on-chain on Sei (~1.5–2M gas for a single P-384 verify), with Intel SGX/TDX competitive (~3.5–4M including multi-cert-chain overhead — Sei's per-byte precompile schedule narrows what would otherwise be a much bigger Intel advantage on Ethereum mainnet). Nitro and NVIDIA are not viable direct; both require amortization (Marlin Oyster secp256k1 pattern) or ZK-proven attestation. **Gas cost is one input, not the deciding factor** — trust model fit (validator-as-host scenarios, vendor ecosystem maturity, operational footprint, privacy/fingerprint exposure) determines which TEE for which Sei application.

Sources: `intel-sgx-tdx.md` §7 + load-bearing claim 11; `amd-sev-snp.md` §7 + load-bearing claim 10; `aws-nitro-enclaves.md` §9 + load-bearing claims 9–10; `nvidia-cc.md` §8 + load-bearing claim 10; `sei-chain/precompiles/p256/p256.go:24-25` for the verified Sei precompile address and gas schedule.

## Application categories on Sei

The TEE patterns that fit Sei's threat model, ranked by maturity of the on-chain pattern. Each entry names the threat the TEE addresses, the load-bearing claim from the per-vendor research, and the practical Sei-side decisions that follow.

### 1. Validator signing-key protection

**Threat**: validator infrastructure compromise (host breach, insider, supply chain) leaks the validator's CometBFT signing key. Once leaked, the validator can be impersonated until rotation completes — and rotation cycles on Sei are operationally expensive.

**TEE answer**: signing key is generated and used inside a TEE; never exists in cleartext outside the boundary. Attestation proves the key is bound to a specific enclave image hash.

**Best fit for harbor-hosted validators / harbor-related workloads**: **AWS Nitro Enclaves**. Harbor is on AWS EKS; Nitro Enclaves are a first-class EC2 feature with mature KMS integration. Validator signing logic runs inside the enclave; the enclave-derived public key is published in validator registration; KMS `kms:RecipientAttestation:PCR0` condition keys gate any wrap/unwrap to the attested image. No on-chain verification needed — the validator's *operational* attestation is enforced by AWS, not Sei consensus.

**Best fit for self-hosted validators on bare metal**: **AMD SEV-SNP** (validator host is a single CVM, full-machine encryption) **or Intel TDX**. SEV-SNP is more prevalent in colo + cloud (Azure, GCP, AWS m6a) as of 2026; TDX is newer but cleaner attestation. Either works; SEV-SNP has broader fleet support.

**Sei-side claim**: minimal. Sei validators registering their attested signing pubkeys could optionally publish the attestation evidence on-chain for delegator inspection, but this is **transparency**, not gating. No per-block verification.

**Load-bearing reference**: `aws-nitro-enclaves.md` §6 (KMS-condition-keyed key release); `amd-sev-snp.md` load-bearing claim 9 (debug-policy rejection); `intel-sgx-tdx.md` load-bearing claim 5 (`ATTRIBUTES.DEBUG = 0`).

### 2. MEV-resistant transaction ordering (sequencer / orderer pattern)

**Threat**: validator (or sequencer) observes the pending-tx pool and reorders / front-runs to extract value. Standard MEV on every chain with a discretionary ordering surface.

**TEE answer**: ordering decision happens inside a TEE. Pending txs are encrypted to the TEE's published pubkey; only the TEE decrypts; ordering is deterministic given a public seed (e.g. previous block hash). Validators learn the ordering after committing to it.

**Best fit**: **Intel TDX** for full-VM ordering enclaves at validator-set scale. The ordering enclave is registered on-chain via its TDX attestation; chain logic accepts ordering decisions signed by the registered enclave's pubkey. Attestation is verified only on registration / rotation (~3.5–4M gas) — the steady-state per-block path uses the registered-enclave pubkey via Sei's P256VERIFY at 48k gas per signature check.

**Why not Nitro here**: Nitro PCR0 attests the enclave image; AWS controls the parent instance's host kernel and could (theoretically) influence non-encrypted I/O paths. For an MEV-resistant ordering surface that must defend against the host operator (a validator running on AWS), Nitro's threat model assumes the AWS host is trusted — which is fine for many use cases but undermines the MEV-resistance claim if the validator IS the host operator.

**Sei-side claim**: substantial. The chain accepts orderings only from registered TDX-attested enclaves. Attestation registration is a governance / on-chain event; attestation rotation on TCB updates is a defined flow.

**Open question**: does Sei consensus tolerate the latency of TEE-bound ordering at Sei's current sub-second block target (pacific-1 cadence is currently ~400ms; the historical 200ms goal remains a longer-term target)? TDX has measured per-call overhead from VM exits and crypto; sustained ordering decisions inside the TEE must complete within the block time minus consensus overhead. This is a benchmark / design decision, not a research question — promoted to the Open Questions section below.

**Load-bearing references**: `intel-sgx-tdx.md` load-bearing claims 2 (`MRTD` + `RTMR[0..3]` required for TDX identity) and 11 (Sei P256VERIFY); `tpm-2.0-open-standards.md` load-bearing claim 7 (CCEL for confidential VM event log replay).

### 3. ZK proving in TEE (proof acceleration)

**Threat**: ZK proof generation is computationally expensive; running it on untrusted infrastructure exposes the prover to: (a) tampered proofs (malicious operator generates a proof that "verifies" but uses a wrong witness), (b) leaked private inputs (the witness is sensitive). Trust the proof's binding to its inputs.

**TEE answer**: ZK prover runs inside a TEE. Attestation binds the prover image to a known-good build. Private inputs are sealed to the enclave; proof output includes an attestation of the prover identity that the verifier (on-chain or off-chain) can check.

**Best fit for GPU-accelerated proving**: **NVIDIA H100 / Blackwell confidential compute**. ZK proving on modern systems (Risc0, SP1, Halo2 GPU variants) leans heavily on GPU compute. H100 confidential compute attestation + CPU-side TDX/SEV-SNP joint attestation lets a prover bind its proof to the GPU + CPU TEE identity.

**Best fit for CPU-only proving** (smaller circuits, statement aggregation): **Intel TDX** or **SGX**. TDX is operationally easier (full VM, no enclave-specific build); SGX is more proven for small attested compute kernels but requires the prover code to be SGX-enclave-shaped.

**Sei-side claim**: the on-chain verifier accepts a ZK proof if (a) the proof is valid by the underlying ZK verifier, AND (b) the proof was produced by an attested prover whose attestation root matches a registered list. Pattern is: ZK-proven attestation (the prover proves "I am a registered TEE prover" inside the SNARK itself) → ~200k–500k gas on-chain. Per `nvidia-cc.md` load-bearing claim 10 and `amd-sev-snp.md` §7 — this is the only practical path for high-frequency provers.

**Load-bearing reference**: `nvidia-cc.md` §4 (joint CPU+GPU attestation via SPDM session keys hashed into TEE REPORT_DATA on Hopper; hardware TDISP/IDE binding on Blackwell); `amd-sev-snp.md` §7 (Risc0 / SP1 zkSEV-SNP at ~500k gas).

### 4. Cross-chain bridge security

**Threat**: bridge operator compromise allows minting tokens on the destination chain without a corresponding deposit on the source chain. Largest aggregate value lost to attacks in the crypto industry by category.

**TEE answer**: bridge state-machine runs inside a TEE. Attestation binds bridge logic to a known image. Witness signatures (proving "this deposit happened on source chain") are produced inside the TEE and verifiable as TEE-attested on the destination chain.

**Best fit**: **AWS Nitro Enclaves** via Marlin Oyster pattern. Mature, well-documented, KMS-integrated. The Marlin pattern: verify one bridge-enclave attestation on-chain at deploy / rotation (~63M gas cold, amortized over the bridge's lifetime); accept secp256k1-signed bridge attestations via `ecrecover` (~3k gas) for every cross-chain message.

**Alternative**: **Intel TDX** with Automata DCAP contracts deployed on Sei; ~3.5–4M gas per bridge message verification using P256VERIFY — economical at moderate volume but more expensive than the Nitro+secp256k1 amortization pattern at high volume.

**Witness-key freshness — load-bearing concern.** The Marlin Oyster amortization pattern reuses a secp256k1 binding key across many cross-chain messages. Without explicit per-message nonce/sequence binding, a stolen binding key persists past TCB rotation and bridge image upgrade. The design MUST include: (a) sequence numbers in each witness signature, (b) bridge-enclave-driven binding-key rotation on TCB updates, (c) on-chain registry of binding-key validity windows tied to the underlying attestation registration.

**Sei-side claim**: substantial. Bridge mint/burn logic on Sei accepts only TEE-attested witness signatures with valid sequence numbers and current binding-key validity. Attestation root + PCR / MRTD reference values are stored in a registry contract. Image rotation requires governance.

**Load-bearing references**: `aws-nitro-enclaves.md` load-bearing claims 9–10 (Marlin Oyster pattern + ~63M cold / 3k secp256k1 amortization); `intel-sgx-tdx.md` load-bearing claim 11 (Sei P256VERIFY economics).

### 5. Confidential mempool / encrypted state

**Threat**: pre-confirmation visibility of transactions enables MEV (overlaps with §2) AND off-chain analytics surveillance. Some applications (private DEXes, sealed-bid auctions, confidential payments) require pre-confirmation privacy as a product requirement, not just a defense.

**TEE answer**: txs are encrypted to a TEE-held key. Mempool contains only ciphertexts. TEE decrypts at ordering time, produces an ordering decision, and reveals txs to consensus only after the ordering is committed.

**Best fit**: **Intel TDX** (full-VM mempool, persistent state across blocks via sealed storage, MRTD + RTMR identity). TDX's full-VM model fits a long-lived mempool process better than SGX's enclave model. Per `intel-sgx-tdx.md` load-bearing claim 2, MRTD-only identity is insufficient — RTMR-based runtime identity is required to prove the mempool kernel + boot config is unchanged.

**Sei-side claim**: significant. Sei consensus accepts encrypted txs; the TEE-attested ordering decision is part of the block production protocol. This is a consensus-layer change, not a smart-contract pattern. Out of scope for Tide directly but worth understanding the TEE primitives that would back it.

**Load-bearing reference**: `intel-sgx-tdx.md` load-bearing claims 1–2 (TDX measurement semantics, MRTD + RTMR requirement); `tpm-2.0-open-standards.md` load-bearing claim 7 (CCEL event log).

### 6. Tide-specific: TEE-bound agent runtimes

**Threat (Tide context)**: Tide's review and execution agent runtimes are Python containers running on the harbor EKS cluster (or downstream consumer infrastructure). On-chain TideCouncil / TideJobHook contracts release escrow against agent decisions. An attacker who compromises an agent runtime can produce false reviews or execute jobs in ways that drain escrow.

**TEE answer**: agent runtimes run inside Nitro Enclaves. Attestation binds the runtime to a registered image (PCR0 / PCR1 / PCR2). On-chain contracts release escrow only to agent identities that have presented a valid Nitro attestation matching a governance-approved image hash.

**Best fit**: **AWS Nitro Enclaves**, given harbor's existing EKS + AWS posture. Standard Marlin Oyster amortization pattern: each agent registration verifies one Nitro attestation on-chain (~63M cold gas — paid by the agent at registration, amortized over their service life); subsequent agent submissions use a registered secp256k1 key whose binding to the Nitro attestation was verified at registration time.

**Sei-side claim**: TideJobHook stores a registry of approved agent image hashes (PCR0 values). Agent submissions must be signed by a registered enclave-bound key. Image rotation is a governance action via TideCouncil.

**Open questions** (flagged for downstream Tide design):
- Where does the Nitro PCR0 reference value enter the system? On-chain registry vs. off-chain reference value provider per IETF RATS?
- Does Tide accept multiple TEE vendors (Nitro for AWS-hosted agents, TDX for non-AWS agents)? The on-chain verifier must dispatch on attester type. **Trust-set deltas are not fungible** — Nitro trusts AWS hypervisor; SEV-SNP/TDX trust silicon vendor; NVIDIA trusts NRAS + NVIDIA PKI. A multi-vendor verifier must surface which trust set applies to each attestation, not just dispatch on `tee_type`.
- How does the agent's secp256k1 binding key get rotated? On every enclave restart (new ephemeral key) or pinned to a longer-lived KMS-attested key?
- **Registry / governance compromise of TideJobHook image-hash registry.** This is the highest-leverage attack on the entire scheme: an attacker who compromises TideCouncil governance and adds a malicious PCR0 to the registry can mint TEE-attested approvals for any payload. The threat model MUST include governance multisig requirements, time-locks on registry changes, and reference-value transparency mechanisms (CoRIM with public RVP).

**Load-bearing references**: `aws-nitro-enclaves.md` load-bearing claims 3 (PCR3 = IAM role hash, PCR4 = parent instance ID hash — both verifiable), 7 (KMS `kms:RecipientAttestation:ImageSha384`), 10 (Marlin Oyster amortization pattern).

## Trust roots in the Sei ecosystem

Where each layer's trust terminates, and which TEE primitive can root that trust differently.

| Sei component | Trust root today | TEE-based alternative | What that buys |
|---|---|---|---|
| sei-chain validator signing | Validator-operated key on validator host | Nitro / TDX / SEV-SNP enclave-bound key | Host compromise no longer leaks the signing key |
| sei-chain block ordering | Validator discretion (MEV-extractable) | TDX-attested ordering enclave registered on-chain | MEV-resistance becomes a chain-level guarantee, not validator-policy |
| sei-sidecar (seictl controller-to-pod task API, port 7777) | Sidecar process trust in the pod | TEE-bound sidecar (Nitro/TDX) if controller-issued task auth is high-value | Compromised pod can't impersonate controller's task surface |
| Waterway (EVM JSON-RPC HTTP/WS reverse proxy, upstream of the platform gateway) | Process trust in the proxy + gateway-level TLS termination | TEE-bound TLS termination at the gateway/Waterway boundary | TLS private key custody moves into the TEE; gateway compromise doesn't leak in-flight RPC |
| sei-k8s-controller orchestration | Controller leader trust; per-pod RBAC | Controller refuses to register/peer un-attested SeiNodes in its peer set (status-level gating). **Note**: hard "attested-pod-only" admission requires a `ValidatingAdmissionPolicy` outside controller-runtime; that's a separate K8s primitive, not a controller-runtime feature. | Peer set + status reflect attestation; off-cluster image compromise becomes detectable at controller level |
| Harbor cluster compute | AWS EKS + IAM | Same, plus Nitro Enclaves for sensitive workloads | Harbor-hosted agents/bridges get TEE-bound posture without leaving the existing platform |
| Tide on-chain contracts (TideCouncil, TideJobHook) | EVM consensus + Solidity verification | Same, augmented with on-chain TEE attestation verification (Intel via P256VERIFY; AMD via Solidity P-384; others via amortization) | On-chain coordination contracts can gate escrow release on TEE attestation evidence |

The pattern across the table: TEE doesn't replace any of these trust roots — it **layers** a defense-in-depth attestation primitive over them. Sei consensus still validates blocks; AWS still operates the EKS substrate; the K8s controller still owns scheduling. TEE attestation answers a different question: "for *this specific* decision, can I prove that the code running in this specific compute boundary is the code we collectively approved?"

## Open standards alignment for Sei

Per `tpm-2.0-open-standards.md` load-bearing claims 5–6: IETF RATS (RFC 9334) is the cross-vendor vocabulary; EAT (RFC 9711) is the standard claim envelope but vendor Evidence formats remain heterogeneous.

For Sei-bound designs, this implies:

- **Use RATS role names in design docs.** Attester (the TEE), Verifier (the on-chain contract or off-chain verifier service), Relying Party (Tide contracts or Sei consensus), Endorser (AMD KDS, Intel PCS, AWS, NVIDIA NRAS), Reference Value Provider (governance / TideCouncil for Tide-relevant images).
- **EAT is the verifier-output format, not the on-chain input format.** On-chain contracts consume vendor-native Evidence (SNP report bytes, TDX quote bytes, COSE_Sign1 Nitro doc bytes) directly. EAT becomes useful at the verifier-result layer if Tide adopts a multi-vendor verifier abstraction.
- **CoRIM (Concise Reference Integrity Manifest)** is the emerging cross-vendor reference-value format; AMD has a published CoRIM profile draft, Intel and Nitro are pursuing equivalents. For Tide's reference-value management, CoRIM is the format to align on long-term.

## Critical defense-in-depth considerations

Cross-cutting from the vendor research. These are the things a Sei-side TEE verifier MUST do that vendors don't enforce automatically.

1. **Reject debug attestations.** AMD: policy bit 19 (`DEBUG_ALLOWED`) must be 0. Intel: `ATTRIBUTES.DEBUG` must be 0. Nitro: all-zero PCRs indicate debug mode — reject. NVIDIA: similar debug-flag pattern. The verifier's first check on any attestation is the debug-mode bit.

2. **Anti-rollback policy.** AMD SEV-SNP signs reports for any historical TCB version (per `amd-sev-snp.md` load-bearing claim 8); the verifier MUST enforce `REPORTED_TCB >= minimum_acceptable_TCB`. Intel TCB Info has `tcbStatus` — `Revoked` MUST be rejected, `OutOfDate` requires explicit policy. AWS Nitro leaf cert validity windows must be checked against the document `timestamp`, not wall-clock.

3. **Freshness binding.** Every attestation must include a verifier-issued nonce. AMD: `REPORT_DATA` (offset 0x050, 64 bytes). Intel: `REPORTDATA` (last 64 bytes of report body). Nitro: `nonce` field (≤1024 B). NVIDIA: 128-bit nonce in SPDM request. Without nonce binding, attestations replay.

4. **Generation / version selection.** AMD: VCEK is per-chip + per-`REPORTED_TCB` — fetch the right cert. Intel: PCK cert chain is 3 layers — fetch all. NVIDIA: 5-cert chain on Hopper. Caching the wrong cert produces "valid signature, lies about platform" outcomes.

5. **Joint-attester binding.** NVIDIA confidential compute requires *both* GPU attestation AND CPU TEE attestation. The two are bound via SPDM session keys hashed into the CPU TEE's REPORT_DATA on Hopper, or via TDISP/IDE in hardware on Blackwell. Verifiers must check the binding, not just the two reports independently (per `nvidia-cc.md` load-bearing claim 8).

6. **Verifier policy is separate from Evidence parsing.** Per `tpm-2.0-open-standards.md` load-bearing claim 10: provider owns format, verifier owns policy. A Sei-side Verifier should parse vendor Evidence into a normalized claim set, then apply policy (which PCR values are acceptable, which TCB versions are minimum, which images are revoked) as a separate layer. Don't hard-code vendor-specific Evidence parsing into the policy layer.

7. **Side-channel advisory handling.** Intel TCB Info includes `advisoryIDs` (the list of Intel SA-XXXX security advisories applicable to the platform's current TCB). Silently accepting `tcbStatus == OutOfDate` without surfacing or policy-checking advisories is the SGAxe failure mode. The verifier MUST surface `advisoryIDs` to the relying party policy layer; relying-party policy decides which advisories are acceptable for which workload class.

8. **BadRAM mitigation for AMD SEV-SNP.** Chips in the BadRAM-vulnerable window require `PLATFORM_INFO.ALIAS_CHECK_COMPLETE` bit = 1 to indicate the AMD-SB-3015 mitigation TCB has been applied. The verifier MUST require this bit for any AMD SEV-SNP chip whose generation is in the BadRAM-affected set.

9. **Host-controlled-but-signed fields.** Nitro `user_data` (≤1024 B, host-controlled — written by the parent EC2 instance) and AMD SEV-SNP `HOST_DATA` (32 B, hypervisor-supplied) are signed by the attestation but **not measured**. A malicious or compromised host can attach attacker-controlled bytes to a legitimate attestation. Any verifier policy that gates on `user_data` or `HOST_DATA` content needs a separate trust assumption — the values are evidence of the host's input, not of the enclave/guest's behavior.

10. **Quote/report version pinning.** Each vendor has multiple attestation format versions (Intel SGX EREPORT, Quote v3, v4, v5; AMD SEV-SNP report v2, v3, v5; Nitro evolving CDDL). Verifiers MUST pin acceptable versions and reject downgrade. Version confusion is a known DCAP-verifier-bug class.

11. **Privacy / device fingerprinting on-chain.** Publishing raw attestations on-chain exposes uniquely-identifying device IDs as permanent ledger entries: AMD `CHIP_ID` (8 bytes, per-chip from manufacture), AWS Nitro `module_id`, TPM EK certificate, NVIDIA PDI. For validator-as-attester patterns, this is a long-term identity leak. Consider: (a) Nitro for fleet-wide attestations (no per-chip identifier), (b) AMD VLEK instead of VCEK (one VLEK per CSP fleet, not per chip), (c) zero-knowledge attestation proofs that hide device-unique identifiers while preserving the attested-claim property.

## Open questions and deferred decisions

These are explicitly **not** answered by this research and would need a downstream Tide design pass:

- **Vendor selection per Tide subsystem.** Which TEE vendor for review-runtime agents? For execution-runtime agents? For TideCouncil signing? The research informs but does not decide.
- **Cross-vendor verifier strategy.** Does Tide accept attestations from multiple vendors, or commit to one for v1? If multi-vendor, what's the on-chain dispatch mechanism (per-attester verifier contract addresses, or a unified verifier with `tee_type` discriminator)? Trust-set deltas must be surfaced to relying-party policy.
- **Reference-value management.** On-chain registry of acceptable PCR / MRTD / measurement values vs off-chain Reference Value Provider per RATS. Governance flow for adding / rotating reference values. Time-locks + multisig requirements on registry changes (registry compromise is the highest-leverage attack on the system).
- **Continuous attestation cadence.** One-shot at registration (cheaper) vs. periodic re-attestation (fresher). The right answer depends on the threat model per application — needs per-subsystem framing.
- **TEE-hosted KMS / signing key custody.** Pure ephemeral keys (regenerated per enclave start; rebound via attestation on each restart) vs. KMS-attested durable keys (the AWS Nitro + KMS condition-keyed pattern). Trade-off: simplicity vs. operational continuity across enclave restarts.
- **MEV-resistant ordering latency at Sei block time.** Does TDX per-call overhead (VM exits, crypto) leave enough budget inside Sei's sub-second block target to support a TEE-bound ordering decision? Benchmark question for §2's pattern.
- **Tide image-hash registry placement.** Does the registry live in TideJobHook or TideCouncil? Constitutional ownership matters for one-way doors (storage layout).
- **Privacy posture for validator-as-attester patterns.** If validators use VCEK (per-chip) for signing-key protection, the chip identifier becomes a permanent fingerprint on-chain. Use VLEK (per-CSP-fleet) or accept the fingerprint exposure?

## References (the rest of this corpus)

- [`amd-sev-snp.md`](amd-sev-snp.md) — AMD SEV-SNP attestation report, signing chain, generation differences (Milan / Genoa / Bergamo / Turin), on-chain verification cost
- [`intel-sgx-tdx.md`](intel-sgx-tdx.md) — Intel SGX EREPORT, TDX TDREPORT, DCAP flow, EPID deprecation, Sei P256VERIFY economics
- [`nvidia-cc.md`](nvidia-cc.md) — NVIDIA H100 / Blackwell confidential compute, joint CPU+GPU attestation, NRAS, performance overheads
- [`aws-nitro-enclaves.md`](aws-nitro-enclaves.md) — Nitro attestation CDDL, PCR semantics, KMS condition keys, Marlin Oyster amortization pattern
- [`tpm-2.0-open-standards.md`](tpm-2.0-open-standards.md) — TPM 2.0, RATS (RFC 9334), EAT (RFC 9711), CCEL, DICE, SPDM, cross-vendor mapping

## Load-bearing claims for the `tee-specialist` agent (Sei-specific)

1. **Sei EVM's P-256 precompile at `0x0000000000000000000000000000000000001011` charges `300 gas/byte × 160 bytes = 48,000 gas per verify.** This is well below Solidity P-256 (~200k/verify) but above EIP-7951's flat ~6k. Concretely: a full DCAP attestation on Sei costs ~3.5–4M gas (5–8 verifies + parsing); AMD SEV-SNP is ~1.5–2M (single P-384 Solidity verify). AMD is cheapest direct on-chain on Sei; Intel is competitive. Nitro and NVIDIA require amortization or ZK-proven attestation.
2. **For high-volume operations on non-Intel TEEs, the Marlin Oyster amortization pattern (verify one enclave attestation on-chain, accept secp256k1 attestations after) is the production path.** Direct on-chain verification of Nitro / NVIDIA attestations per-operation is not economically viable; AMD direct is borderline at high volume.
3. **TDX identity requires `MRTD` + `RTMR[0..3]` together; MRTD-only is exploitable.** Any Sei design that gates on TDX attestation MUST consume RTMR values, not MRTD alone.
4. **AWS Nitro on harbor is the natural choice for Tide agent runtimes**, given the existing AWS EKS posture. Marlin Oyster pattern + KMS condition keys + `kms:RecipientAttestation:ImageSha384` gating is the well-trodden path.
5. **MEV-resistant ordering on Sei must defend against the validator-as-host threat**, which Intel TDX handles (full-VM, attested at boot) but AWS Nitro does not (assumes AWS host is trusted; validator-as-AWS-host violates that assumption).
6. **Joint CPU+GPU attestation** is required for any NVIDIA-CC ZK proving design. Software-bound on Hopper (SPDM session key in CPU TEE REPORT_DATA); hardware-bound on Blackwell (TDISP/IDE). The verifier must check the binding, not just the two reports independently.
7. **EAT (RFC 9711, April 2025) is the verifier-output format, not the on-chain input format.** On-chain Sei contracts consume vendor-native Evidence directly; EAT enters at the verifier-result abstraction layer if Tide adopts a multi-vendor verifier.
8. **Every attestation verifier MUST enforce: debug-bit-rejection, anti-rollback policy on TCB version, freshness binding (nonce in REPORT_DATA / REPORTDATA / Nitro nonce / SPDM nonce), generation-specific cert chain selection, version pinning (reject downgrade), Intel `advisoryIDs` surfacing, AMD BadRAM `ALIAS_CHECK_COMPLETE` for chips in the affected window, and a separate trust assumption for any policy that gates on host-controlled-but-signed fields (Nitro `user_data`, AMD `HOST_DATA`).** These are verifier-policy, not vendor-automatic.
9. **Cross-vendor trust-set deltas are not fungible.** Multi-vendor verifier must surface which trust set applies (Nitro: AWS hypervisor + AWS PKI; Intel/AMD: silicon vendor PKI; NVIDIA: NRAS + NVIDIA PKI). Designs that treat `tee_type` as a switch without surfacing the underlying trust delta misrepresent the security posture to the relying party.
10. **Registry / governance compromise of reference values is the highest-leverage attack on the entire scheme.** Any TideJobHook / TideCouncil image-hash registry MUST include: multisig requirements, time-locks on changes, reference-value transparency (CoRIM with public RVP), and an emergency revocation path. The TEE only matters if the registry that defines "what's a valid measurement" is itself trustworthy.
11. **TEE doesn't replace existing Sei trust roots — it layers attestation as defense-in-depth.** Designs that claim TEE "replaces consensus" or "removes the need to trust AWS" are over-claiming; the right framing is "for *this specific* decision, prove the code is the registered code."
12. **Reference-value management is the long pole.** The CoRIM standard is emerging; Tide should align on it for cross-vendor reference values rather than building a Tide-specific registry format that won't compose with future tooling.
13. **Privacy / device fingerprinting on-chain.** Raw attestations published on-chain expose permanent device identifiers (AMD `CHIP_ID`, Nitro `module_id`, TPM EK, NVIDIA PDI). For validator-as-attester patterns, prefer VLEK (per-CSP-fleet) over VCEK (per-chip), or use ZK-attestation to hide device-unique fields while preserving attested claims.
