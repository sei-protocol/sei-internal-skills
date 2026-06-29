# AWS Nitro Enclaves kit

> Ground truth: the AWS Nitro primary specs, RFCs, and reference implementations cited inline below (see §7) — this kit is a **self-contained kernel** that stands alone on those **public** sources. Every load-bearing claim cites a primary source (a vendor spec, an RFC, or `sei-chain` source); do not paraphrase — cite the primary source. No dependency on any private/internal research archive.

## 1. Identity & RATS roles

- **What it is** — a VM-isolated enclave carved from a parent EC2 instance: no persistent storage, no network, parent↔enclave only over `vsock` (§5). Attestation is point-in-time — re-attest for freshness.
- **RATS role mapping** — **Attester** = enclave + Nitro Security Module (NSM); **Endorser** = the AWS Nitro Attestation PKI; **Verifier** = AWS KMS (verifier-as-a-service) or an on-chain contract; **Relying Party** = the entity gating secret/privilege release (§7).
- **Trust root / Endorser** — pin **AWS Nitro Enclaves Root-G1** (PEM at `aws-nitro-enclaves.amazonaws.com/AWS_NitroEnclaves_Root-G1.zip`, SHA-256 fingerprint `64:1A:03:21:…:BB:5B`, 30-year lifetime — AWS Nitro Enclaves User Guide `verify-root.html`). **Trust set = AWS hypervisor + AWS PKI** (not a silicon vendor) — feeds VP16 and the validator-as-host caveat below.

**Trust-model caveat (VP16 / profile Rule 2):** Nitro's model assumes the **AWS host is trusted** (AWS Nitro Enclaves User Guide, isolation model). It is the natural fit for harbor-hosted sei-internal-skills agent runtimes and bridges (existing AWS EKS posture), but it does **not** defend against a relying party who *is* the AWS host operator — so it is the wrong fit for a validator-as-host MEV/ordering surface (see `tee-profile.md` §2).

## 2. Evidence format

CBOR `AttestationDocument` wrapped in a **COSE_Sign1** envelope (RFC 8152, CBOR tag 18), signed by the Nitro Hypervisor **on behalf of the enclave** — the NSM (§1) is the enclave-side request interface; the hypervisor holds the signing key (§1.1–1.2, load-bearing claim 1):

- Protected header `{1: -35}` → `alg = ES384` (ECDSA P-384 + SHA-384); unprotected header empty `{}`.
- Signature is **raw `r || s`, 96 bytes** (COSE convention) — **not** X.509 DER `SEQUENCE`. Verifiers using OpenSSL must convert (§4).
- Mandatory fields: `module_id`, `digest = "SHA384"`, `timestamp` (ms since epoch), `pcrs`, `certificate` (leaf), `cabundle`. Optional ≤1024 B: `public_key`, `user_data`, `nonce` (§1.1, load-bearing claim 2).
- **Parser gotchas (the common verifier bugs):** the signed `Sig_structure`'s `external_aad` MUST be the **empty byte string `h''`** — not absent, not an empty map (§1.3, §4); pass the `protected` bstr through verbatim (don't re-encode); `timestamp` is **milliseconds** (cert validity is seconds) (§4).
- **VP13 version pinning:** the CDDL is evolving — pin accepted document shapes and reject downgrade (§7, method VP13).

## 3. Identity & measurement fields (VP2)

PCRs are 48-byte SHA-384 measurements, TPM-style extend from an all-zero start (§2):

| PCR | binds | source |
|---|---|---|
| PCR0 | enclave image file (EIF) | `nitro-cli build-enclave` output |
| PCR1 | Linux kernel + bootstrap | build output |
| PCR2 | application | build output |
| PCR3 | parent instance **IAM role** hash `sha384(48×0x00 || role_arn)` (`||` = byte concatenation; single extend from the all-zero start) | hypervisor (load-bearing claim 3) |
| PCR4 | parent **instance-ID** hash | hypervisor (load-bearing claim 3) |
| PCR8 | enclave image **signing-cert** hash | only when `--private-key` and `--signing-certificate` are supplied to `nitro-cli build-enclave` |

Binary identity for a sei-internal-skills agent image gates on **PCR0** (and optionally PCR1/PCR2/PCR8); PCR3/PCR4 additionally bind the parent's IAM role + instance. PCR5/6/7 are unused. Reference value enters via the on-chain registry / governance RVP (profile, VP7/VP15). (§2, load-bearing claim 3.)

## 4. Verifier-policy specifics — the per-vendor fill-ins

| method dimension | Nitro's specific | cite |
|---|---|---|
| VP1 freshness | verifier-issued nonce in the `nonce` field (≤1024 B); bind it per attestation | §1.1, claim 2 |
| VP2 binary binding | PCR0 (+ PCR1/2/8) matches a governance-approved reference value | §2, claim 3 |
| VP3 debug-mode | **debug/console enclaves emit all-zero PCRs — reject any all-zero PCR set** | §2, claim 4 |
| VP4 anti-rollback | no TCB integer; enforce **leaf-cert validity against the document `timestamp`** (leaf is hours-scale, ephemeral), not wall-clock | §3, claim 6 |
| VP5 cert-chain / generation | `cabundle` order is `[ROOT, INTERM_1, …, INTERM_N]`; the leaf is in `certificate` (not `cabundle`); path-building reverses to `[leaf … root]`; **pin Root-G1**; `keyCertSign` on CA certs, `digitalSignature` on leaf; **CRL disabled** in the canonical procedure | §3, claim 5 |
| VP6 key isolation | → §6 (attested KMS condition keys; or per-restart ephemeral rebind) | §6 |
| VP7 revocation | → profile (governance-driven revocation in the on-chain reference-value registry) | profile |
| VP8 joint-attester | **N/A** — single attester (NVIDIA-specific) | — |
| VP9 policy separation | parse the COSE/CBOR Evidence (§2) into a normalized claim set; apply PCR/measurement policy as a separate layer | method VP9 + §2 |
| VP10 advisories | **N/A** — no advisory-ID field (Intel-specific); freshness is the cert-validity window | §3 |
| VP11 known-CVE bits | **N/A** — no platform-info mitigation bit (AMD BadRAM-specific) | — |
| VP12 host-controlled-but-signed | `user_data` (≤1024 B) is **host-supplied, signed but NOT measured** — any policy gating on it needs a separate trust assumption | §1.1; AWS Nitro NSM API `attestation_process.md` (`user_data` is caller-supplied) |
| VP13 version pinning | pin accepted CDDL document shapes; reject downgrade | §7 |
| VP14 privacy / fingerprint | `module_id` is a per-hypervisor identifier; raw on-chain attestations leak it — Nitro has **no per-chip ID** (fleet-friendlier than AMD `CHIP_ID`); prefer ZK for validator-as-attester | §1.1 (`module_id` field); AWS Nitro NSM API `attestation_process.md` |
| VP15 registry integrity | → profile (multisig + time-lock + transparency + emergency revocation on the reference-value registry) | profile |
| VP16 cross-vendor delta | trust set = **AWS hypervisor + AWS PKI** (§1) — surface it when this kit sits beside a silicon-vendor or NRAS-rooted kit | §1 |

## 5. On-chain verification (Sei)

No EVM precompile for ECDSA-P384 or X.509 path validation — every primitive (P-384 field math, SHA-384, DER, COSE reconstruction) is Solidity/Yul. Floor is **3× P-384 verify** (root→interm, interm→leaf, leaf→COSE) plus SHA-384 + DER:

- **~63M gas cold** for a full unwarmed attestation (public Marlin NitroProver numbers); **<20M warm** with the cert chain cached on-chain. (The Sei cost ranking in `method.md` renders this same Marlin measurement as the **<70M** ceiling — one number, two roundings, not two measurements.) Above Sei's practical per-tx ceiling cold.
- **Production path — Marlin Oyster amortization:** verify **one** "verifier-enclave" attestation on-chain once, then accept **secp256k1**-signed statements from that enclave via the `ecrecover` precompile (**~3k gas**) for every subsequent attestation (public Marlin Oyster docs). The only realistic posture for steady-state on Sei.
- **Witness-key freshness:** the amortization pattern reuses a secp256k1 binding key — without per-message sequence numbers + binding-key rotation on TCB/image change + an on-chain validity window, a stolen binding key outlives the attestation it was minted under (engineering discipline over the Marlin Oyster reuse pattern; see also `method.md` VP1/VP4).

## 6. Key-release / integration pattern (VP6)

**Attested KMS condition keys** are the correct-by-construction path (§6): KMS attested operations (`Decrypt`, `GenerateDataKey`, `GenerateDataKeyPair`, `GenerateRandom`, `DeriveSharedSecret`) verify the attestation, then **encrypt the response to the `public_key` carried in the attestation document** — the plaintext key never exists outside the enclave. The enclave `public_key` must be **RSA**: KMS binds via `KeyEncryptionAlgorithm: RSAES_OAEP_SHA_256` (§6.1) — an EC `public_key` is rejected. Gate with `kms:RecipientAttestation:ImageSha384` (PCR0 alias) or `kms:RecipientAttestation:PCR<n>`, operator `StringEqualsIgnoreCase` (§6.2, load-bearing claim 7). The on-chain verifier is the fallback for non-KMS consumers.

## 7. Citations

- **Ground truth (primary sources):** AWS Nitro Enclaves User Guide (`set-up-attestation.html`, `verify-root.html`); `aws/aws-nitro-enclaves-nsm-api` `docs/attestation_process.md`; AWS KMS Nitro docs; RFC 8152 / 9052 (COSE), RFC 9334 (RATS), RFC 9711 (EAT — Nitro is EAT-*adjacent*, not a formal EAT); RFC 8610 (CDDL).
- **Reference verifiers:** Marlin NitroProver + Oyster docs; `base/nitro-validator`; `hf/nitrite` (Go); `veracruz-project/nitro-enclave-attestation-document`.
- Distilled from Sei-internal TEE research; this kit stands alone on the public sources above.
