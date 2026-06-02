# AWS Nitro Enclaves — Attestation Deep Reference

Source-cited reference material for the Tide `tee-specialist` agent and any future on-chain Nitro attestation verifier. Every load-bearing claim is grounded in AWS canonical documentation, the `aws-nitro-enclaves-nsm-api` repo, IETF RFCs, or a named open-source verifier implementation. Where AWS publishes verbatim language we quote it.

Status: working draft, captured during research run `run-tee-research-2026-06-02T04-19-53Z`.

---

## 1. Attestation Document — verbatim structure

Nitro attestation documents are **CBOR-encoded payloads wrapped in a COSE_Sign1 envelope** signed by the Nitro Hypervisor on behalf of the enclave. The canonical structure is published in [`aws/aws-nitro-enclaves-nsm-api/docs/attestation_process.md`](https://github.com/aws/aws-nitro-enclaves-nsm-api/blob/main/docs/attestation_process.md).

### 1.1 CBOR `AttestationDocument` map (verbatim)

```cddl
AttestationDocument = {
    module_id:   text,             ; issuing Nitro hypervisor module ID
    timestamp:   uint .size 8,     ; UTC time in milliseconds since UNIX epoch
    digest:      digest,           ; digest function for register values
    pcrs:        { + index => pcr },; map of locked PCRs
    certificate: cert,             ; infrastructure certificate, DER encoded
    cabundle:    [* cert],         ; issuing CA bundle
    ? public_key: user_data,       ; optional DER-encoded key
    ? user_data:  user_data,       ; additional signed user data
    ? nonce:      user_data,       ; optional cryptographic nonce
}
```

Field constraints (from the same source):

| Field         | Type / size                                                        | Notes |
| ------------- | ------------------------------------------------------------------ | ----- |
| `module_id`   | non-empty text                                                     | Identifies the Nitro Hypervisor module issuing the doc |
| `digest`      | text string, must be `"SHA384"`                                    | Implicitly fixes register and signature digest |
| `timestamp`   | uint, ms since UNIX epoch, > 0                                     | Wall clock as observed by the hypervisor |
| `pcrs`        | map, 1–32 entries, keys 0–31, values 32 / 48 / 64 bytes            | In practice 48-byte SHA-384 values |
| `certificate` | DER cert bytes, 1–1024 bytes                                       | Leaf cert for the signing key |
| `cabundle`    | array of DER certs, each 1–1024 bytes, at least one entry          | Issuing CA chain |
| `public_key`  | byte string, 0–1024 bytes, optional                                | Enclave-supplied; binds an external key into the signed doc |
| `user_data`   | byte string, 0–1024 bytes, optional                                | Application-controlled signed payload |
| `nonce`       | byte string, 0–1024 bytes, optional                                | Verifier-supplied freshness challenge |

> Source-of-truth note: The AWS Nitro Enclaves User Guide CDDL ([verify-root.html](https://docs.aws.amazon.com/enclaves/latest/user/verify-root.html)) defines `user_data = bytes .size (0..1024)` and reuses that type for `public_key`, `user_data`, and `nonce`. The `aws-nitro-enclaves-nsm-api` README lists `0..512` for `user_data` and `nonce` and `1..1024` for `public_key`. The AWS User Guide CDDL is the canonical spec; treat 1024 B as the documented ceiling. If you're implementing the consumer side, defensively cap at 1024.

### 1.2 COSE_Sign1 envelope (verbatim)

The signed wrapper uses CBOR tag 18 (`COSE_Sign1`) per [RFC 8152 §4.2](https://datatracker.ietf.org/doc/html/rfc8152#section-4.2):

```
18(
    {1: -35},   ; protected header: alg = ES384 (ECDSA w/ SHA-384)
    {},         ; unprotected header (empty)
    bstr,       ; attestation document payload (the CBOR map above)
    bstr        ; signature (raw r || s; 96 bytes for P-384)
)
```

`alg = -35` is the COSE algorithm identifier for `ES384` per [IANA COSE Algorithms](https://www.iana.org/assignments/cose/cose.xhtml#algorithms).

AWS describes this verbatim in [Verifying the root of trust](https://docs.aws.amazon.com/enclaves/latest/user/verify-root.html):

> ```
> 18(/* COSE_Sign1 CBOR tag is 18 */
>     {1: -35}, /* This is equivalent with {algorithm: ECDS 384} */
>     {}, /* We have nothing in unprotected */
>     $ATTESTATION_DOCUMENT_CONTENT /* Attestation Document */,
>     signature /* This is the signature */
> )
> ```
>
> "An attestation document will always have its CA bundle in the following order: `[ ROOT_CERT - INTERM_1 - INTERM_2 .... - INTERM_N]` … Keep this ordering in mind, as some existing tools, such as Java's CertPath … might require them to be ordered differently."
>
> "CRL must be disabled when doing the validation."

PKI subject (verbatim from the same source):

> ```
> CN=aws.nitro-enclaves, C=US, O=Amazon, OU=AWS
> ```

### 1.3 `Sig_structure` (what is actually signed)

Per [RFC 8152 §4.4](https://datatracker.ietf.org/doc/html/rfc8152#section-4.4), the bytes signed are the CBOR encoding of:

```
Sig_structure = [
    context:        "Signature1",
    body_protected: bstr,   ; the protected header bstr from the COSE_Sign1
    external_aad:   bstr,   ; empty byte string h''
    payload:        bstr    ; the CBOR-encoded AttestationDocument
]
```

This is the most common verifier bug: `external_aad` MUST be the empty byte string (`h''`, length 0), not the empty CBOR map or omitted. The Marlin and `veracruz-project/nitro-enclave-attestation-document` implementations confirm this.

---

## 2. PCR semantics (verbatim from AWS)

Source: [Cryptographic attestation — Where to get an enclave's measurements](https://docs.aws.amazon.com/enclaves/latest/user/set-up-attestation.html).

| PCR  | Hash of …                                       | Description |
| ---- | ----------------------------------------------- | ----------- |
| PCR0 | Enclave image file                              | "A contiguous measure of the contents of the image file, without the section data." |
| PCR1 | Linux kernel and bootstrap                      | "A contiguous measurement of the kernel and boot ramfs data." |
| PCR2 | Application                                     | "A contiguous, in-order measurement of the user applications, without the boot ramfs." |
| PCR3 | IAM role assigned to the parent instance        | "Ensures that the attestation process succeeds only when the parent instance has the correct IAM role." |
| PCR4 | Instance ID of the parent instance              | "Ensures that the attestation process succeeds only when the parent instance has a specific instance ID." |
| PCR8 | Enclave image file signing certificate          | "Ensures that the attestation process succeeds only when the enclave was booted from an enclave image file signed by a specific certificate." |

PCR5, PCR6, PCR7 are unused in standard usage.

Important correction from the research brief: PCR3 is **the IAM role hash**, not the AWS account ID. AWS documents it as `sha384(48 zero bytes || iam_role_arn)`. PCR4 is `sha384(48 zero bytes || instance_id)`. Both are pre-padded with 48 null bytes — this is the initial PCR state (see §8.4).

PCR0/PCR1/PCR2 are produced as build outputs of `nitro-cli build-enclave`. Example (AWS docs):

```text
Enclave Image successfully created.
{
  "Measurements": {
    "HashAlgorithm": "Sha384 { ... }",
    "PCR0": "7fb5c55bc2ecbb68ed99a13d7122abfc0666b926a79d5379bc58b9445c84217f59cfdd36c08b2c79552928702efe23e4",
    "PCR1": "235c9e6050abf6b993c915505f3220e2d82b51aff830ad14cbecc2eec1bf0b4ae749d311c663f464cde9f718acca5286",
    "PCR2": "0f0ac32c300289e872e6ac4d19b0b5ac4a9b020c98295643ff3978610750ce6a86f7edff24e3c0a4a445f2ff8a9ea79d"
  }
}
```

PCR8 is exposed only when `--private-key` and `--signing-certificate` are supplied to `nitro-cli build-enclave`. The signing certificate identity is independent of AWS PKI and is verifier-defined.

> Debug / console-attached enclaves: "Enclaves booted in debug mode generate attestation documents with PCRs that are made up entirely of zeros. These attestation documents can't be used for cryptographic attestation." A verifier MUST reject all-zero PCRs in any production policy. — AWS docs.

---

## 3. Signing chain

Source: [Verifying the root of trust](https://docs.aws.amazon.com/enclaves/latest/user/verify-root.html), [`attestation_process.md`](https://github.com/aws/aws-nitro-enclaves-nsm-api/blob/main/docs/attestation_process.md).

```
AWS Nitro Attestation PKI Root (G1)
    └── AWS Nitro Attestation Intermediate CA(s)
            └── Enclave Leaf Certificate  ← signs the COSE_Sign1 payload
```

Properties:

- All certificates and the COSE signature use **ECDSA on P-384 with SHA-384**.
- The root CA is **AWS Nitro Enclaves Root-G1**, distributed as PEM in the zip at:
  `https://aws-nitro-enclaves.amazonaws.com/AWS_NitroEnclaves_Root-G1.zip`
- Published SHA-256 fingerprint of the root cert:
  `64:1A:03:21:A3:E2:44:EF:E4:56:46:31:95:D6:06:31:7E:D7:CD:CC:3C:17:56:E0:98:93:F3:C6:8F:79:BB:5B`
- The root is backed by an AWS Private CA private key with a **30-year lifetime**.
- The **leaf certificate is per-enclave-instance and ephemeral**; AWS rotates the leaf on every enclave lifecycle event. Validity windows for leaf certs are typically on the order of **hours**, which is why verifiers must rely on `timestamp` from the document being within the leaf's `notBefore`/`notAfter`, not on wall-clock at verification time.
- Intermediates are valid for longer windows but rotate as well; pinning intermediates is brittle. Pinning the root is the correct posture.

`cabundle` ordering in the attestation document is **[root, intermediate_1, …, intermediate_N]**. A validator that uses an X.509 path-building API typically reverses this to `[leaf, intermediate_N, …, intermediate_1, root]` and runs path validation against the pinned root. The leaf cert lives in the `certificate` field, not in `cabundle`.

Validation requirements (`attestation_process.md`):

- Each certificate must be within its `notBefore`/`notAfter` at the **document `timestamp`**, not at verification wall-clock.
- Root and intermediates require the `keyCertSign` key-usage bit.
- The leaf requires the `digitalSignature` key-usage bit.
- `pathLenConstraint` must accommodate chain depth on CA certs.
- **CRL validation is disabled** in the canonical procedure; revocation is out of band for this PKI.

Operational note: AWS does publish revocation events out of band, but for an on-chain verifier the practical posture is "trust the chain if the timestamp is recent and within validity windows; accept the staleness window as the freshness model."

---

## 4. COSE_Sign1 verification details

References: [RFC 8152](https://datatracker.ietf.org/doc/html/rfc8152), [RFC 9052](https://datatracker.ietf.org/doc/html/rfc9052) (COSE bis), [IANA COSE Algorithms registry](https://www.iana.org/assignments/cose/cose.xhtml).

The protected header for Nitro is the canonical CBOR encoding of `{1: -35}`, where:

- Key `1` is `alg`.
- Value `-35` is `ES384` = "ECDSA w/ SHA-384".

The unprotected header is the empty CBOR map `{}` (canonical encoding `0xa0`).

Signature format: **raw concatenated `r || s` integers**, big-endian, 48 bytes each ⇒ 96-byte signature. This is the COSE convention per RFC 8152 §8.1, **not** the X.509 DER-encoded `SEQUENCE { r INTEGER, s INTEGER }` shape. Verifiers using OpenSSL's `EVP_DigestVerify` must convert before calling.

Verification recipe:

1. Parse the outer `COSE_Sign1` (tagged value 18 or untagged 4-element array).
2. Extract `protected` (bstr), `unprotected` (map), `payload` (bstr), `signature` (bstr).
3. Reconstruct `Sig_structure = ["Signature1", protected, h'', payload]` and CBOR-encode it.
4. SHA-384 the encoding; verify ECDSA-P384 signature using the leaf cert's public key.
5. Parse `payload` as CBOR `AttestationDocument`.
6. Walk `cabundle` + `certificate` and validate path against the pinned Nitro root (G1).
7. Apply PCR policy to the `pcrs` map; apply `nonce` / `user_data` policy.

Edge cases that bite implementers:

- **External AAD must be `h''`** (zero-length bstr), not absent and not the empty map.
- **CBOR canonical encoding matters**: the `protected` bytes the signer used must be the bytes you hash. Do not re-encode the protected map; pass the bstr through.
- **Signature ordering**: raw `r || s`, not DER.
- **Timestamp**: in milliseconds, not seconds. Multiply/divide accordingly when comparing to cert validity (which is seconds).

---

## 5. Nitro Security Module (NSM) device

The enclave-side interface to the hypervisor's attestation service.

- Character device: **`/dev/nsm`** inside the enclave.
- Linux driver: `nsm` (in-tree since Linux 6.8 for the Arm64 / x86_64 enclave variant; previously out-of-tree).
- Request/response transport: CBOR over an ioctl.
- Canonical client: [`aws/aws-nitro-enclaves-nsm-api`](https://github.com/aws/aws-nitro-enclaves-nsm-api), Rust crates `aws-nitro-enclaves-nsm-api` and `nsm-io`.

Request variants (Rust enum, from `nsm-io`):

| Variant            | Purpose |
| ------------------ | ------- |
| `DescribeNSM`      | Query module ID, version, capabilities, locked PCRs |
| `DescribePCR { index }` | Read a single PCR (returns `lock`, `data`) |
| `ExtendPCR { index, data }` | Extend a PCR with `data` — only allowed on unlocked, writable PCRs (16–31 by convention) |
| `LockPCR { index }` | Make a PCR read-only for the enclave lifetime |
| `LockPCRs { range }` | Lock a contiguous range |
| `GetRandom`        | Cryptographically secure random from the hypervisor RNG |
| `GetAttestationDoc { user_data, nonce, public_key }` | Produce a signed attestation document binding the three optional fields |

PCRs 0–15 are hypervisor-measured and locked at enclave start; PCRs 16–31 are application-extendable then lockable. The Rust API surface lives in:
[`aws-nitro-enclaves-nsm-api/nsm-io/src/lib.rs`](https://github.com/aws/aws-nitro-enclaves-nsm-api/blob/main/nsm-io/src/lib.rs)
and the user-facing helper at [`nsm-lib/src/lib.rs`](https://github.com/aws/aws-nitro-enclaves-nsm-api/blob/main/nsm-lib/src/lib.rs).

The C SDK ([`aws-nitro-enclaves-sdk-c`](https://github.com/aws/aws-nitro-enclaves-sdk-c)) wraps the same surface and provides `nitro_enclaves_attestation` plus KMS helpers.

`kmstool-enclave-cli` (in the same repo) is the reference application that exchanges an attestation document for a KMS-encrypted blob.

---

## 6. AWS KMS integration

Source: [How Nitro Enclaves uses AWS KMS](https://docs.aws.amazon.com/kms/latest/developerguide/services-nitro-enclaves.html), [AWS KMS condition keys for attested platforms](https://docs.aws.amazon.com/kms/latest/developerguide/conditions-attestation.html), [Condition keys for Nitro Enclaves](https://docs.aws.amazon.com/kms/latest/developerguide/conditions-nitro-enclaves.html).

### 6.1 The `Recipient` parameter

AWS KMS supports cryptographic attestation on these operations:

- `Decrypt`
- `DeriveSharedSecret`
- `GenerateDataKey`
- `GenerateDataKeyPair`
- `GenerateRandom`

When called from an enclave, the request carries a `Recipient` parameter containing:

- `AttestationDocument`: the COSE_Sign1 blob (max ~64 KiB),
- `KeyEncryptionAlgorithm`: at present `RSAES_OAEP_SHA_256` (binds the enclave-supplied RSA public key in `public_key`).

KMS verifies the attestation document (path validation up to the pinned Nitro root, signature, freshness via `timestamp`, evaluation of `kms:RecipientAttestation:*` conditions). On success, KMS **replaces the plaintext in the standard response** with a ciphertext that encrypts that plaintext under the `public_key` extracted from the attestation document. The enclave decrypts inside the TEE with its private key, which never leaves the enclave.

Verbatim from AWS:

> "AWS KMS verifies that the attestation document came from a valid source (either a Nitro enclave or NitroTPM). Then, instead of returning plaintext data in the response, these APIs encrypt the plaintext with the public key from the attestation document and return ciphertext that can be decrypted only by the corresponding private key in the enclave or EC2 instance."

Behavioral table (verbatim, abridged):

| Operation             | Standard response                          | Attested response |
| --------------------- | ------------------------------------------ | ----------------- |
| `Decrypt`             | plaintext                                  | plaintext encrypted to attested `public_key` |
| `DeriveSharedSecret`  | raw shared secret                          | shared secret encrypted to attested `public_key` |
| `GenerateDataKey`     | plaintext data key + wrapped data key      | data key encrypted to attested `public_key` + wrapped data key |
| `GenerateDataKeyPair` | plaintext private key + public key + wrapped private key | private key encrypted to attested `public_key` + public key + wrapped private key |
| `GenerateRandom`      | random bytes                               | random bytes encrypted to attested `public_key` |

### 6.2 Condition keys

KMS exposes the following condition keys to gate access:

| Condition key                                | Source field |
| -------------------------------------------- | ------------ |
| `kms:RecipientAttestation:ImageSha384`       | PCR0 (image hash) — convenience alias |
| `kms:RecipientAttestation:PCR0`              | PCR0 |
| `kms:RecipientAttestation:PCR1`              | PCR1 |
| `kms:RecipientAttestation:PCR2`              | PCR2 |
| `kms:RecipientAttestation:PCR3`              | PCR3 (IAM role hash) |
| `kms:RecipientAttestation:PCR4`              | PCR4 (instance ID hash) |
| `kms:RecipientAttestation:PCR8`              | PCR8 (signing cert hash) |

Recommended operator: `StringEqualsIgnoreCase` (PCR values are hex).

Example policy statement (composed from AWS docs language):

```json
{
  "Sid": "Allow only the audited enclave image",
  "Effect": "Allow",
  "Principal": { "AWS": "arn:aws:iam::111122223333:role/enclave-host" },
  "Action": [
    "kms:Decrypt",
    "kms:GenerateDataKey",
    "kms:GenerateRandom"
  ],
  "Resource": "*",
  "Condition": {
    "StringEqualsIgnoreCase": {
      "kms:RecipientAttestation:ImageSha384":
        "7fb5c55bc2ecbb68ed99a13d7122abfc0666b926a79d5379bc58b9445c84217f59cfdd36c08b2c79552928702efe23e4"
    }
  }
}
```

Operational implication for Tide: KMS-gated secret release is the simplest correct-by-construction path to "secret unsealed only to a known-good Nitro image". The on-chain verifier is the fallback for cases where the secret consumer is not KMS.

---

## 7. Open-standards alignment

- **RFC 9334 — RATS Architecture** ([link](https://datatracker.ietf.org/doc/html/rfc9334)). Nitro maps onto RATS as: Attester = enclave + NSM, Verifier = service-side verifier (e.g., AWS KMS or our on-chain contract), Relying Party = the entity gating release of secrets / privileges. The Nitro Hypervisor is the "endorser" via the Nitro PKI. Nitro is a strong example of "verifier as a service" when KMS is the verifier; it can also be used in passport mode when the attestation doc is forwarded.
- **RFC 8152 — COSE** ([link](https://datatracker.ietf.org/doc/html/rfc8152)) and its successor [RFC 9052](https://datatracker.ietf.org/doc/html/rfc9052): the signing envelope is `COSE_Sign1`. The COSE bis spec (9052) is wire-compatible; verifiers can use either citation.
- **RFC 9711 — Entity Attestation Token (EAT)** ([link](https://datatracker.ietf.org/doc/html/rfc9711)). Nitro's CBOR claim set has notable conceptual alignment with EAT (CBOR map of claims, `nonce`, `ueid`-style identifier in `module_id`) but **is not formally an EAT** — it does not use the EAT-registered CBOR claim keys, does not declare an EAT profile, and predates RFC 9711. Treat as "EAT-adjacent."
- **CDDL — RFC 8610** ([link](https://datatracker.ietf.org/doc/html/rfc8610)) describes the data-definition language used in the attestation document spec.

---

## 8. Cryptographic primitives

### 8.1 Signature algorithm

- ECDSA on **NIST P-384** (secp384r1) with SHA-384 throughout the chain.
- COSE alg ID: `-35` (`ES384`).
- Signature wire format inside COSE: raw `r || s`, 96 bytes total.
- Cert signatures: X.509 DER `SEQUENCE { r INTEGER, s INTEGER }` (standard).

### 8.2 Digest

- SHA-384 everywhere. The `digest` field in the attestation document is the literal text `"SHA384"`.

### 8.3 PCR representation

- 48-byte SHA-384 measurement per PCR, big-endian.
- Indices 0–31; AWS uses 0, 1, 2, 3, 4, 8 in standard practice.
- Stored as CBOR byte string of length 48.

### 8.4 PCR extension rule

PCRs follow the classic TPM-style extend:

```
PCR_new = SHA384( PCR_old || data )
```

- Initial PCR state: **all zeros** (48 bytes of `0x00`).
- This is why PCR3 / PCR4 in AWS examples are computed as `sha384(48*b'\0' || arn_or_id)` — they are a single extend from the zero state, performed by the hypervisor before the enclave starts.
- The `48*b'\0' || …` pattern in the AWS example commands is documenting that exact extend.

---

## 9. On-chain verification (EVM) cost and approaches

### 9.1 Cost shape

There is no EVM precompile for ECDSA-P384 or for X.509 path validation. Everything must be implemented in Solidity / Yul, including:

- Big-integer modular arithmetic over the P-384 prime field (`p = 2^384 − 2^128 − 2^96 + 2^32 − 1`).
- Point addition and scalar multiplication on P-384.
- SHA-384 (Solidity must implement or rely on a Yul SHA-512-family reduction; the EVM only natively offers SHA-256 / Keccak-256).
- DER parsing for the two CA certs.
- COSE_Sign1 reconstruction (CBOR canonical encoding of `Sig_structure`).

The COSE signature itself is a single P-384 verify (~one scalar mul + one point add). Each X.509 cert verification is another P-384 verify. So the floor is **3 × P-384 verify** (root → intermediate, intermediate → leaf, leaf → COSE) plus SHA-384 plus DER plumbing.

### 9.2 Empirical numbers

- [Marlin NitroProver](https://github.com/marlinprotocol/NitroProver) (Solidity) reports:
  - **~63 M gas** to validate a full attestation **with no prior verified certs**.
  - **< 20 M gas** for the second-stage attestation once the cert chain is pre-cached on-chain.
  See [Marlin blog: "On-chain verification of AWS Nitro Enclave attestations"](https://blog.marlin.org/on-chain-verification-of-aws-nitro-enclave-attestations).
- [`base/nitro-validator`](https://github.com/base/nitro-validator) is Coinbase Base's Solidity verifier in production use, similar profile.

The 63M-gas-cold number is impractical on Ethereum L1 (block gas limit ~30M), and tight even on most L2s in a single tx. The standard mitigation is the Marlin pattern: **verify the chain once, store the leaf's public key on-chain, then verify subsequent attestations with cached intermediates**.

### 9.3 Cost-reduction approaches

1. **Cert caching** (Marlin): on-chain mapping `keccak256(leaf_cert_der) => bool verified`, set after the expensive path validation. Future attestations from the same leaf skip cert validation entirely. Works because the leaf is stable for an enclave's lifetime.
2. **Two-tier attestation** (Marlin attestation-verifier enclave): a dedicated enclave verifies attestations off-chain and signs a small **secp256k1** statement that the EVM verifies cheaply via the `ecrecover` precompile. The verifier enclave's own attestation is verified on-chain **once**, anchoring a chain of trust. Brings per-attestation cost to ~`ecrecover` (~3,000 gas).
3. **ZK proof of attestation validity**: produce a SNARK over (a) X.509 path validation, (b) COSE signature, (c) PCR policy. Verifier on-chain runs a single Groth16/PLONK verify (~250k–500k gas). Active research area; not yet a turnkey library for Nitro specifically.
4. **L2 settlement + L1 anchor**: do the cold verification on a cheap L2, anchor the resulting "verified-image" claim to L1 via a bridge or a signed attestation, then settle on L1. Practical when the relying contract is multi-chain.

For Tide: if the contract enforcing Nitro attestation lives on Sei EVM, a cold 63M-gas verify is **above Sei's per-tx ceiling** in practical scenarios. The verifier-enclave-with-secp256k1 pattern is the realistic posture; this matches the brief's reference to `oyster-tee-coordinator`.

---

## 10. Empirical references

Canonical / first-party:

- AWS Nitro Enclaves User Guide — https://docs.aws.amazon.com/enclaves/latest/user/
- Cryptographic attestation — https://docs.aws.amazon.com/enclaves/latest/user/set-up-attestation.html
- Verifying the root of trust — https://docs.aws.amazon.com/enclaves/latest/user/verify-root.html
- AWS Nitro Root certificate G1 (PEM, zip) — https://aws-nitro-enclaves.amazonaws.com/AWS_NitroEnclaves_Root-G1.zip
  SHA-256 fingerprint: `64:1A:03:21:A3:E2:44:EF:E4:56:46:31:95:D6:06:31:7E:D7:CD:CC:3C:17:56:E0:98:93:F3:C6:8F:79:BB:5B`
- AWS Nitro System security whitepaper — https://docs.aws.amazon.com/whitepapers/latest/security-design-of-aws-nitro-system/
- KMS — How Nitro Enclaves uses AWS KMS — https://docs.aws.amazon.com/kms/latest/developerguide/services-nitro-enclaves.html
- KMS condition keys for Nitro Enclaves — https://docs.aws.amazon.com/kms/latest/developerguide/conditions-nitro-enclaves.html
- KMS attested calls — https://docs.aws.amazon.com/kms/latest/developerguide/attested-calls.html
- `aws/aws-nitro-enclaves-nsm-api` — https://github.com/aws/aws-nitro-enclaves-nsm-api
- `aws/aws-nitro-enclaves-nsm-api/docs/attestation_process.md` — https://github.com/aws/aws-nitro-enclaves-nsm-api/blob/main/docs/attestation_process.md
- `aws/aws-nitro-enclaves-cli` — https://github.com/aws/aws-nitro-enclaves-cli
- `aws/aws-nitro-enclaves-sdk-c` — https://github.com/aws/aws-nitro-enclaves-sdk-c
- AWS blog: Validating attestation documents produced by AWS Nitro Enclaves — https://aws.amazon.com/blogs/compute/validating-attestation-documents-produced-by-aws-nitro-enclaves/

Standards:

- RFC 9334 (RATS Architecture) — https://datatracker.ietf.org/doc/html/rfc9334
- RFC 9711 (EAT) — https://datatracker.ietf.org/doc/html/rfc9711
- RFC 8152 (COSE) — https://datatracker.ietf.org/doc/html/rfc8152
- RFC 9052 (COSE bis) — https://datatracker.ietf.org/doc/html/rfc9052
- RFC 8610 (CDDL) — https://datatracker.ietf.org/doc/html/rfc8610
- IANA COSE Algorithms — https://www.iana.org/assignments/cose/cose.xhtml

Open-source verifiers:

- Marlin NitroProver (Solidity) — https://github.com/marlinprotocol/NitroProver
- Marlin blog: On-chain verification of AWS Nitro Enclave attestations — https://blog.marlin.org/on-chain-verification-of-aws-nitro-enclave-attestations
- Marlin Oyster docs — https://docs.marlin.org/oyster/build-cvm/examples/attestation-verifier
- `base/nitro-validator` (Solidity) — https://github.com/base/nitro-validator
- `hf/nitrite` (Go) — https://pkg.go.dev/github.com/hf/nitrite
- `veracruz-project/nitro-enclave-attestation-document` — https://github.com/veracruz-project/nitro-enclave-attestation-document
- `aws/aws-nitro-enclaves-acm` — https://github.com/aws/aws-nitro-enclaves-acm

---

## Load-bearing claims

1. **Wire format**: Nitro attestation = **COSE_Sign1 (RFC 8152, tag 18) over a CBOR map**. Protected header `{1: -35}` (alg = ES384). External AAD is the **empty byte string** for signing. ([attestation_process.md](https://github.com/aws/aws-nitro-enclaves-nsm-api/blob/main/docs/attestation_process.md))
2. **Mandatory fields**: `module_id`, `digest = "SHA384"`, `timestamp` (ms), `pcrs`, `certificate`, `cabundle`. Optional: `public_key`, `user_data`, `nonce`, each ≤1024 B per the AWS User Guide CDDL. ([verify-root.html](https://docs.aws.amazon.com/enclaves/latest/user/verify-root.html))
3. **PCR3 = IAM role hash**, **PCR4 = parent-instance-ID hash** — not account ID. Both are `sha384(48 zero bytes || string)`. PCR0/1/2 are EIF-derived; PCR8 is signing-cert-derived; PCR5/6/7 unused. ([AWS docs](https://docs.aws.amazon.com/enclaves/latest/user/set-up-attestation.html))
4. **Debug enclaves emit all-zero PCRs** and MUST be rejected by production verifiers. ([AWS docs](https://docs.aws.amazon.com/enclaves/latest/user/set-up-attestation.html))
5. **Cryptography is uniformly ECDSA-P384 + SHA-384** at every layer (root, intermediates, leaf, COSE signature, PCR digest). Root is AWS Nitro Enclaves **Root-G1**, 30-year lifetime, SHA-256 fingerprint `64:1A:03:21:…:BB:5B`. ([AWS docs](https://docs.aws.amazon.com/enclaves/latest/user/verify-root.html))
6. **Leaf certs are ephemeral (hours-scale validity)** and rotate per enclave lifecycle. Verifiers MUST validate cert windows against the document's `timestamp`, not wall-clock at verify time. ([attestation_process.md](https://github.com/aws/aws-nitro-enclaves-nsm-api/blob/main/docs/attestation_process.md))
7. **KMS attested operations** (`Decrypt`, `GenerateDataKey`, `GenerateDataKeyPair`, `GenerateRandom`, `DeriveSharedSecret`) **encrypt the response to the `public_key`** from the attestation document; gating uses `kms:RecipientAttestation:PCR<n>` and `kms:RecipientAttestation:ImageSha384` (PCR0 alias). ([AWS KMS docs](https://docs.aws.amazon.com/kms/latest/developerguide/services-nitro-enclaves.html))
8. **NSM interface**: `/dev/nsm` ioctl, requests `DescribeNSM`, `DescribePCR`, `ExtendPCR`, `LockPCR`, `LockPCRs`, `GetRandom`, `GetAttestationDoc`. Canonical client: `aws-nitro-enclaves-nsm-api` Rust crate. ([aws-nitro-enclaves-nsm-api](https://github.com/aws/aws-nitro-enclaves-nsm-api))
9. **On-chain cold verification cost**: ~**63M gas** for a full unwarmed attestation in Solidity (Marlin NitroProver); ~**<20M gas** with cached cert chain. EVM has no P-384 or SHA-384 precompile — every primitive is Solidity/Yul. ([Marlin blog](https://blog.marlin.org/on-chain-verification-of-aws-nitro-enclave-attestations))
10. **Practical on-chain pattern**: verify a single "verifier enclave" attestation on-chain once, then accept secp256k1-signed statements from that enclave via `ecrecover` (~3k gas) for subsequent attestations. This is the Marlin Oyster posture and the only realistic path for high-throughput chains. ([Marlin Oyster docs](https://docs.marlin.org/oyster/build-cvm/examples/attestation-verifier))
