# Sei deployment profile — what THIS deployment mandates

> Ground truth: `sei-chain` source, the vendor specs, and the RFCs cited inline below — this profile is **self-contained** and stands alone on those public sources. Every load-bearing claim cites a primary source (a vendor spec, an RFC, or `sei-chain` source); do not paraphrase — cite the primary source.

This is the **always-first overlay** (method Step 1). It is the higher-priority layer: where generic vendor knowledge and this profile disagree, the profile wins — **including in the hard direction**, where it establishes Sei-specific exceptions to a vendor's stated security property (method Rule 2). The kits fill in the per-vendor specifics around what this profile mandates; the profile gates the design. Absent → design reduced-confidence and flag the missing-profile gap (you lose the economics + validator-as-host + registry-as-RVP layer).

## 1. The Sei EVM verification economics (a decision input, not the decision)

Sei EVM ships a **P-256 precompile** (RIP-7212-equivalent surface) — verified against `sei-chain/precompiles/p256/p256.go:24-25`:

- Address `0x0000000000000000000000000000000000001011` (`0x1011`).
- `GasCostPerByte = 300`, 160-byte input → **`300 × 160 = 48,000 gas per verify`**. Below Solidity P-256 (~200k) but above EIP-7951's flat ~6k (`sei-chain/precompiles/p256/p256.go:24-25`; EIP-7951).

The per-attestation **cost ranking** is the decision input — state it, then do the trust-model analysis. The cross-vendor magnitudes below are a **synthesized planning estimate (no public primary source for the cross-vendor synthesis; treat as unverified)**; the anchored public figures are the `0x1011` precompile floor (`sei-chain`), the public Marlin Oyster/NitroProver Nitro numbers, and the public Trail-of-Bits-audited Automata DCAP numbers (Intel/AMD):

| Attester | Scheme | Sei EVM cost (cold) | Strategy this profile mandates |
|---|---|---|---|
| **AMD SEV-SNP** | P-384 single verify | ~1.5–2M gas Solidity / ~250k ZK | Cheapest direct on-chain on Sei |
| **Intel SGX / TDX (DCAP)** | P-256, 5–8 verifies | ~3.5–4M gas via P256VERIFY | Competitive; multi-cert overhead dominates |
| **AWS Nitro** | P-384, COSE_Sign1 | <70M cold / <20M warm, OR ~3k amortized | Marlin Oyster (verify-once → secp256k1 `ecrecover`) |
| **NVIDIA CC** | P-384, multi-verify | ~100M+ direct / ~200k ZK / ~3k relayer | ZK-proven attestation or trusted relayer only |

**Mandate: gas is one input, never the decision.** Trust-model fit (validator-as-host, fingerprint exposure, ecosystem maturity, operational footprint) determines which TEE for which Sei application. A design that picks the cheapest verify before pinning the claim violates Rule 1.

## 2. The validator-as-host caveat (the headline Rule-2 exception)

**A platform whose trust model assumes the host is honest does NOT defend against a relying party who IS the host operator.** Nitro's model assumes the **AWS host is trusted** — the parent EC2 instance controls the enclave's lifecycle and the non-encrypted `vsock` I/O path (AWS Nitro Enclaves User Guide, isolation model; `kit-aws-nitro.md` §1). That is fine for many workloads, but it is the **wrong fit for any surface that must defend against the host operator** — the canonical case being a validator-as-AWS-host running an MEV-resistant ordering surface, where the validator IS the host.

This **overrides vendor marketing trust models** for validator-side designs. The override direction:

- **MEV-resistant ordering / sequencing** must defend against validator-as-host → **Intel TDX** (full-VM, attested at boot, memory confidentiality vs. the host VMM — Intel TDX Module Base Architecture Spec 348549002; `kit-intel-sgx-tdx.md` §1), not Nitro.
- Never carry "Nitro protects the signing key from the host" into a validator-as-host design — the verifier then attests to a boundary the relying party itself controls, protecting nothing.
- Non-AWS / bare-metal / self-hosted validators push to **AMD SEV-SNP or Intel TDX** (silicon-vendor-rooted, full-machine memory encryption against the hypervisor — AMD SEV-SNP Firmware ABI Spec pub. 56860; Intel TDX 348549002; `kit-amd-sev-snp.md` §1, `kit-intel-sgx-tdx.md` §1), not Nitro.

## 3. The reference-value registry / governance as the RATS Reference-Value-Provider

The on-chain image-hash registry (a SeiJobHook / SeiCouncil registry of approved PCR0 / MRTD / measurement values) **is the RATS Reference-Value-Provider.** VP7 (revocation) and VP15 (registry integrity) live HERE — the kits defer them to this profile.

**Mandate — every registry design carries (this profile's deployment mandate; the RVP role is RFC 9334 §3):**

- **Multisig** on registry writes; **time-locks** on registry changes.
- **Transparency** — a public Reference-Value-Provider, CoRIM-formatted (CoRIM, IETF RATS WG draft; see §6).
- **Emergency revocation path (VP7)** — a compromised measurement value is revoked immediately; a known-bad image MUST stop passing the instant it is disclosed.

**Registry compromise is the highest-leverage attack on the entire scheme:** an attacker who compromises governance and adds a malicious PCR0 mints valid TEE-attested approvals for any payload. This follows directly from the RATS appraisal model (RFC 9334 §3 — the Verifier appraises Evidence against Reference Values; a forged Reference Value forges the verdict). The TEE only matters if the registry that defines "what is a valid measurement" is itself trustworthy. Treat the registry **storage layout** as a one-way door (method Rule 4) — once live, changing it is a migration / hard fork; flag, never assert.

## 4. Harbor / EKS realities (what's forced vs. open)

Harbor is **AWS EKS** (this deployment's substrate). This makes the platform choice *forced* for the AWS-hosted cases and *open* for the rest:

- **sei-internal-skills agent runtimes + bridges (harbor-hosted) → AWS Nitro Enclaves** is the natural fit: first-class EC2 feature, mature KMS integration, the existing AWS posture (AWS Nitro Enclaves User Guide; AWS KMS Nitro condition-keys docs; `kit-aws-nitro.md` §1, §6). Standard pattern: Marlin Oyster amortization (~63M cold at registration → secp256k1 `ecrecover` ~3k per submission — public Marlin Oyster/NitroProver numbers) + KMS `kms:RecipientAttestation:ImageSha384` condition keys.
- **Non-AWS / validator-host / bare-metal cases → TDX or SEV-SNP** (see §2). Don't default these to Nitro because harbor is on AWS — the host-trust assumption breaks (§2).

## 5. The Sei trust-roots layering (the over-claiming framing to reject)

**TEE layers attestation as defense-in-depth over existing Sei trust roots; it does NOT replace them** (this profile's mandate; consistent with the RATS scope — RFC 9334 §1 frames attestation as evidence for an appraisal, not a substitute for the relying party's other trust anchors). Sei consensus still validates blocks; AWS still operates the EKS substrate; the K8s controller still owns scheduling. TEE attestation answers a *different* question: "for **this specific** decision, can I prove the code running in this specific compute boundary is the code we collectively approved?"

**Reject the over-claim.** A design that says a TEE "replaces consensus" or "removes the need to trust AWS" misrepresents the posture. The mandated framing is always *"for this specific decision, prove the code is the registered code"* — never *"the TEE makes this trustless."* Note the boundaries the controller layer keeps: hard "attested-pod-only" admission needs a `ValidatingAdmissionPolicy` (a separate Kubernetes primitive — Kubernetes admission-control docs), not controller-runtime; the controller can only status-gate its peer set.

## 6. Open-standards alignment (RATS / EAT / CoRIM)

Anchored to the RATS standards suite (RFC 9334, RFC 9711, the CoRIM IETF RATS WG draft):

- **Use RATS role names (RFC 9334 §3) in every design.** Attester = the TEE; Verifier = the on-chain contract or off-chain service; Relying Party = Sei contracts / Sei consensus; Endorser = AMD KDS / Intel PCS / AWS PKI / NVIDIA NRAS; **Reference-Value-Provider = governance / SeiCouncil** (this profile's §3 registry).
- **EAT (RFC 9711) is the Verifier *output* format, not the on-chain input.** On-chain contracts consume vendor-native Evidence directly (SNP report bytes, TDX quote bytes, Nitro COSE_Sign1 bytes); EAT enters only at a multi-vendor verifier-result abstraction layer (RFC 9711 §1; RFC 9334 §3 places EAT at the Attestation Results boundary). Feeding EAT to an on-chain verifier is a category error.
- **CoRIM is the cross-vendor reference-value format to align on** (CoRIM, IETF RATS WG draft). AMD has a published CoRIM profile draft (`draft-deeglaze-amd-sev-snp-corim-profile`); Intel and Nitro are pursuing equivalents. Align on CoRIM rather than a sei-internal-skills-specific registry format that won't compose with future tooling — and it is the transparency mechanism §3 mandates.

## What this profile leaves to the kits

The kits (`kit-<platform>.md`) fill in everything per-vendor this profile is silent on: the Evidence format, the identity/measurement fields and offsets, the §4 verifier-policy specifics, the cert-chain order, the key-release pattern. This profile gates **which** platform, **what** the trust model excludes, and **how** the reference-value registry must be governed. Profile mandates win; the kit fills the rest.
