# Intel SGX and TDX Attestation — Empirical Reference for Tide

**Purpose.** Ground-truth, source-cited reference informing the `tee-specialist`
agent and future on-chain Intel attestation verifier implementations in Tide.
Every load-bearing claim is anchored to a primary source URL. Spec excerpts
are quoted verbatim where the wording is normative.

**Scope.** Intel SGX (Software Guard Extensions) and Intel TDX (Trust Domain
Extensions) DCAP (Data Center Attestation Primitives) attestation only.
Legacy EPID is covered only insofar as it has been deprecated.

---

## 1. SGX `EREPORT` + Quote — verbatim structures

### 1.1 `sgx_report_body_t` (the heart of every attestation)

The canonical C definition, taken verbatim from Open Enclave's
`include/openenclave/bits/sgx/sgxtypes.h` (Microsoft-maintained mirror of the
Intel ABI):

```c
typedef struct _sgx_report_body
{
    uint8_t  cpusvn[SGX_CPUSVN_SIZE];      // 16 bytes — platform TCB SVN
    uint32_t miscselect;                   //  4 bytes — selector for MISC SSA fields
    uint8_t  reserved1[12];
    uint8_t  isvextprodid[16];             // 16 bytes — extended product id
    sgx_attributes_t attributes;           // 16 bytes — flags (DEBUG, MODE64BIT, …) + XFRM
    uint8_t  mrenclave[OE_SHA256_SIZE];    // 32 bytes — SHA-256 of enclave build
    uint8_t  reserved2[32];
    uint8_t  mrsigner[OE_SHA256_SIZE];     // 32 bytes — SHA-256 of signer RSA pubkey
    uint8_t  reserved3[32];
    uint8_t  configid[64];                 // 64 bytes
    uint16_t isvprodid;                    //  2 bytes — ISV-assigned product id
    uint16_t isvsvn;                       //  2 bytes — ISV-assigned security version
    uint16_t configsvn;                    //  2 bytes
    uint8_t  reserved4[42];
    uint8_t  isvfamilyid[16];              // 16 bytes
    sgx_report_data_t report_data;         // 64 bytes — caller-supplied
} sgx_report_body_t;
```

Source:
[openenclave/bits/sgx/sgxtypes.h](https://github.com/openenclave/openenclave/blob/master/include/openenclave/bits/sgx/sgxtypes.h)

Field-level facts that matter for an on-chain verifier:

- **`MRENCLAVE` (offset 64, 32 bytes).** "Enclave measurement represented as
  SHA256 digest (as defined in FIPS PUB 180-4)." It is a hash chain over
  `EADD` / `EEXTEND` / `EINIT` of every page the enclave loader places into
  the EPC at signing time. It pins the *exact binary* of the enclave.
  Sources:
  [Intel SGX Attestation API spec, p. 17](https://www.intel.com/content/dam/develop/public/us/en/documents/sgx-attestation-api-spec.pdf),
  [sidsbits — SGX Attestation Part 1](https://sidsbits.com/Intel-SGX-Attestation-Part-1/).
- **`MRSIGNER` (offset 128, 32 bytes).** "SHA256 digest (as defined in FIPS
  PUB 180-4) of the big endian format modulus of the RSA public key of the
  enclave's signing key pair." Pins the *signer identity* rather than the
  enclave; a policy of "trust anything from this signer" lets the signer
  ship upgrades.
- **`CPUSVN` (offset 0, 16 bytes).** Platform TCB security version. Opaque
  to software — it is a structured token interpreted only by Intel's PCS
  (and by verifiers consulting the PCS-served TCB Info JSON).
- **`ATTRIBUTES` (offset 48, 16 bytes).** A flags word plus an XFRM mask.
  The critical bit for verifiers is the **`DEBUG` bit** (`ATTRIBUTES.DEBUG = 1`):
  if set, the enclave can be inspected by a debugger and *must not* be
  trusted for production attestation. Any on-chain verifier MUST reject
  quotes with `DEBUG=1`.
- **`ISVPRODID` / `ISVSVN`.** ISV-supplied identity. `MRSIGNER`-bound
  policies typically pin `ISVPRODID` and require `ISVSVN >= N`.
- **`REPORT_DATA` (last 64 bytes).** Caller-supplied. This is the field a
  relying party uses to bind the attestation to an application-specific
  claim — e.g. `SHA-512(ephemeral_pubkey || nonce)`. **On-chain verifiers
  treat REPORT_DATA as the payload; everything else is gating.**

### 1.2 `sgx_report_t` (output of `EREPORT`)

```c
typedef struct _sgx_report
{
    sgx_report_body_t body;          // 384 bytes
    uint8_t keyid[SGX_KEYID_SIZE];   //  32 bytes — key derivation context
    uint8_t mac[SGX_MAC_SIZE];       //  16 bytes — AES-CMAC over body+keyid
} sgx_report_t;
```

Source:
[openenclave/bits/sgx/sgxtypes.h](https://github.com/openenclave/openenclave/blob/master/include/openenclave/bits/sgx/sgxtypes.h).

The MAC is computed with a key derived via `EGETKEY(REPORT_KEY)` keyed to
the *target* enclave's identity — this is what makes **local attestation**
work: only the target enclave can re-derive the report key and verify the
MAC. The MAC is unverifiable off-platform; for remote attestation the
Quoting Enclave verifies it, replaces it with an ECDSA-P256 signature over
the same body, and the result is a Quote.

### 1.3 `sgx_quote_t` v3 (legacy DCAP, ECDSA-P256)

```c
typedef struct _sgx_quote
{
    uint16_t version;             // 3
    uint16_t sign_type;           // 2 = ECDSA-P256 with PCK cert
    uint32_t tee_type;            // 0x00000000 = SGX
    uint16_t qe_svn;
    uint16_t pce_svn;
    uint8_t  uuid[16];            // QE vendor id (Intel GUID)
    uint8_t  user_data[20];
    sgx_report_body_t report_body;// 384 bytes
    uint32_t signature_len;
    uint8_t  signature[];         // variable: ECDSA sig + QE report + cert chain
} sgx_quote_t;
```

Source:
[openenclave/bits/sgx/sgxtypes.h](https://github.com/openenclave/openenclave/blob/master/include/openenclave/bits/sgx/sgxtypes.h),
[Intel SGX ECDSA Quote Library Reference (DCAP)](https://download.01.org/intel-sgx/sgx-dcap/1.3/linux/docs/Intel_SGX_ECDSA_QuoteLibReference_DCAP_API.pdf).

The variable `signature` payload (Quote v3 ECDSA-P256) carries, in order:
ECDSA-P256 signature over the body (64 bytes, r||s); the attestation
public key (64 bytes, x||y); the QE report (an `sgx_report_body_t`
attesting the QE itself, with `REPORT_DATA = SHA-256(att_pubkey ||
qe_auth_data)`); a QE report signature by the PCK leaf cert; QE auth data;
and a certification data blob (typically a PCK cert chain in PEM).

### 1.4 Quote v4 (DCAP, TD-friendly)

Quote v4 generalises the format so a single verifier can handle SGX and
TDX. Per the deepwiki-indexed Intel quote-verification-library:

> "The header is consistent across all versions with these fields:
> `version` (2 bytes), `attestationKeyType` (2 bytes), `teeType` (4 bytes),
> `qeVendorId` (16 bytes), `userData` (20 bytes)."

Source:
[Quote Data Structures — Intel SGX-TDX-DCAP-QuoteVerificationLibrary](https://deepwiki.com/intel/SGX-TDX-DCAP-QuoteVerificationLibrary/3.3-quote-data-structures).

Quote v4 introduces a `body_type` discriminator (4 bytes) + `size`
(2 bytes) so the body can be either an SGX `enclave_report` (384 bytes)
or a TDX `td_report10` (584 bytes) / `td_report15` (648 bytes). The
`certification_data` block is required to be **type 6** (the PCK cert
chain in PEM format concatenated). Source:
[Intel TDX DCAP Quoting Library API](https://download.01.org/intel-sgx/latest/dcap-latest/linux/docs/Intel_TDX_DCAP_Quoting_Library_API.pdf).

**Quote v5** adds Quote Verification Enclave (QVE) identity flexibility
and finer-grained TCB recovery controls; Automata's v1.1 contracts support
v5. Source:
[Automata DCAP v1.1 announcement](https://blog.ata.network/automatas-release-of-dcap-attestation-v1-1-for-agentic-systems-84ae98900370).

---

## 2. TDX `TDREPORT` — verbatim

### 2.1 Where it comes from

A TD guest invokes `TDCALL[TDG.MR.REPORT]` (the TDX-equivalent of `EREPORT`)
to ask the TDX Module for a report. The TDX Module produces a
`TDREPORT_STRUCT` whose `REPORTMACSTRUCT` is bound to the platform via a key
that "is only accessible to a valid SGX enclave (such as the TDQE) on the
same platform." That MAC is then verified by the TD Quoting Enclave using
the SGX instruction `EVERIFYREPORT2`, after which the TDQE signs the
report contents with the platform's ECDSA-P256 attestation key and emits
a TDX Quote. Source:
[Intel TDX DCAP Quoting Library API](https://download.01.org/intel-sgx/latest/dcap-latest/linux/docs/Intel_TDX_DCAP_Quoting_Library_API.pdf).

### 2.2 `TDREPORT_STRUCT` layout

The TDREPORT is a fixed-size structure containing three pieces in order:

1. **`REPORTMACSTRUCT`** (256 bytes): header binding the report to the
   platform, with the HMAC field. Contains:
   - `REPORTTYPE` (4 bytes): type / subtype / version (identifies "TDX")
   - `RESERVED` (12 bytes)
   - `CPUSVN` (16 bytes): platform SVN
   - `TEE_TCB_INFO_HASH` (48 bytes, SHA-384)
   - `TEE_INFO_HASH` (48 bytes, SHA-384, over `TDINFO_STRUCT`)
   - `REPORTDATA` (64 bytes): caller-supplied
   - `RESERVED` (32 bytes)
   - `MAC` (32 bytes): HMAC-SHA-256 keyed to the platform report key
2. **`TEE_TCB_INFO`** (239 bytes per TDX 1.5; structurally similar to the
   SGX TCB): contains the TDX Module's measurement (MRSEAM /
   MRSEAMSIGNER), TEE_TCB_SVN, attributes.
3. **`TDINFO_STRUCT`** (512 bytes for TDX 1.0 layout; 528 bytes in 1.5
   when the additional `MR_SERVICETD` family is included; see the v15
   variant): the actual TD identity and runtime measurements.

Per
[DeepWiki Quote Data Structures](https://deepwiki.com/intel/SGX-TDX-DCAP-QuoteVerificationLibrary/3.3-quote-data-structures),
the two flavors observed in practice are:

> "**TDReport10**: Fixed 584-byte structure containing TEE TCB SVN and SEAM
> measurements. **TDReport15**: Extended 648-byte structure with
> additional security features."

### 2.3 `TDINFO_STRUCT` — fields (all sizes verbatim)

Per Intel's TDX 1.5 Base Architecture Specification
([cdrdv2-public.intel.com/733575](https://cdrdv2-public.intel.com/733575/intel-tdx-module-1.5-base-spec-348549002.pdf))
and the
[enclaive TDX attestation-report doc](https://docs.enclaive.cloud/confidential-cloud/technology-in-depth/intel-tdx/technology/fundamentals/dcap-attestation/attestation-report):

| Field            | Size      | Meaning |
|------------------|-----------|---------|
| `ATTRIBUTES`     | 8 bytes   | TD attributes flag word (DEBUG bit, KL, etc.) |
| `XFAM`           | 8 bytes   | Extended Features Available Mask for the TD |
| `MRTD`           | 48 bytes  | **SHA-384** hash of initial TD memory contents and config (set at TD build) |
| `MRCONFIGID`     | 48 bytes  | Software-defined non-owner config ID |
| `MROWNER`        | 48 bytes  | Software-defined ID for the TD's owner |
| `MROWNERCONFIG`  | 48 bytes  | Software-defined owner-config ID |
| `RTMR[0]`        | 48 bytes  | Runtime measurement register 0 (SHA-384) |
| `RTMR[1]`        | 48 bytes  | Runtime measurement register 1 (SHA-384) |
| `RTMR[2]`        | 48 bytes  | Runtime measurement register 2 (SHA-384) |
| `RTMR[3]`        | 48 bytes  | Runtime measurement register 3 (SHA-384) |
| `SERVICETD_HASH` | 48 bytes  | (TDX 1.5 only) Hash of allowed service TDs |

The hash algorithm for MRTD and RTMR[i] is **SHA-384** (not SHA-256 as in
SGX). RTMR registers are *extendable* at runtime by a TD-issued
`TDG.MR.RTMR.EXTEND` call:

> `RTMR[i]_new = SHA384( RTMR[i]_old || SHA384(event) )`

Source:
[Phala — Understanding TDX Attestation Reports](https://phala.com/posts/understanding-tdx-attestation-reports-a-developers-guide).

Conventional usage from a typical Linux TD boot chain
(Confidential Containers / dstack / GCE-TCB conventions):

- `RTMR[0]` — virtual hardware environment (vTPM, virtual firmware tables,
  ACPI), measured by the virtual firmware (TDVF / OVMF).
- `RTMR[1]` — Linux kernel image (vmlinuz).
- `RTMR[2]` — kernel cmdline + initrd.
- `RTMR[3]` — application-defined (workload-level events).

Sources:
[Phala — Understanding TDX Attestation Reports](https://phala.com/posts/understanding-tdx-attestation-reports-a-developers-guide),
[google/gce-tcb-verifier issue #73](https://github.com/google/gce-tcb-verifier/issues/73),
[td-shim spec](https://github.com/confidential-containers/td-shim/blob/main/doc/tdshim_spec.md).

### 2.4 Semantic gap: SGX `MRENCLAVE` vs TDX `MRTD` (load-bearing)

| Property             | SGX `MRENCLAVE`              | TDX `MRTD`                              |
|----------------------|------------------------------|------------------------------------------|
| Hash function        | SHA-256                      | SHA-384                                  |
| Size                 | 32 bytes                     | 48 bytes                                 |
| Measures             | enclave binary + EPC build   | initial TD memory + config (build-time)  |
| Extends at runtime   | **No** — pinned at `EINIT`   | **No** — pinned at `TDH.MR.FINALIZE`     |
| Runtime extension    | n/a (enclave is sealed)      | `RTMR[0..3]` (caller-driven SHA-384)     |
| Signer policy hash   | `MRSIGNER` (RSA pubkey)      | `MRSIGNERSEAM` / `MRSEAM` (TDX Module)   |

The single most important consequence: **for SGX, identity is fully
captured by `MRENCLAVE` (or `MRSIGNER`+`ISVPRODID`+`ISVSVN`) at quote
time.** For TDX, identity requires *both* `MRTD` (the launch image) and the
runtime `RTMR[0..3]` values — otherwise an attacker who can boot any
kernel they like into the same TD launch image looks identical at the
`MRTD` layer.

---

## 3. Attestation flow (DCAP)

### 3.1 Roles

- **Intel SGX PCS (Provisioning Certification Service).** Intel-operated
  REST service. Hands out PCK certificates, PCK CRLs, TCB Info JSON, and
  QE Identity JSON. Documented at:
  [api.portal.trustedservices.intel.com/provisioning-certification](https://api.portal.trustedservices.intel.com/provisioning-certification).
- **PCCS (Provisioning Certificate Caching Service).** Optional on-prem
  reference cache that proxies PCS so attestation does not require
  internet egress on every quote.
  [Design Guide](https://download.01.org/intel-sgx/sgx-dcap/1.10/linux/docs/SGX_DCAP_Caching_Service_Design_Guide.pdf).
- **PCE (Provisioning Certification Enclave).** Intel-signed enclave on
  the host that uses `EGETKEY` to derive the PCK and a Platform
  Provisioning ID (PPID). The PCK is "a 256-bit ECDSA signing key using
  the NIST p-256 curve, unique to the device, current SGX TCB SVNs, and
  PCE `ISVSVN`." Source:
  [enclaive — DCAP Attestation Framework](https://docs.enclaive.cloud/confidential-cloud/technology-in-depth/intel-sgx/technology/concepts/dcap-attestation-framework).
- **QE (Quoting Enclave).** Intel-signed enclave that generates the per-
  platform ECDSA attestation key (also P-256), seeds it from `EGETKEY`
  (Seal Key) — "a repeatable signing key, which is not known to Intel" —
  has it certified by the PCE, then signs reports with it. Source:
  [enclaive — DCAP Attestation Framework](https://docs.enclaive.cloud/confidential-cloud/technology-in-depth/intel-sgx/technology/concepts/dcap-attestation-framework).
- **TDQE (TD Quoting Enclave).** SGX-enclave variant of the QE that
  consumes TDX `TDREPORT_STRUCT` via `EVERIFYREPORT2` and emits TDX Quotes.

### 3.2 Local vs remote attestation

- **Local attestation.** Inside one platform, enclave A asks `EREPORT` for
  a report targeted at enclave B. The CPU MACs the report with a key
  `EGETKEY(REPORT_KEY)` derived from B's identity. B re-derives the same
  key and verifies the MAC. The result is a single-platform trust link;
  it does not leave the box. Used to bootstrap the trust between an
  application enclave and the QE.
- **Remote attestation.** The application enclave produces a local report
  targeted at the QE (`REPORT_DATA` typically = `SHA-256(app_pubkey ||
  nonce)`). The QE `EVERIFYREPORT`s it, then signs an
  `sgx_report_body_t`-shaped payload with the ECDSA-P256 attestation key.
  The signature, the AK, the QE's own self-report (signed by the PCK), and
  the PCK cert chain become the Quote.

### 3.3 PCK certificate chain

Three layers, X.509:

1. **Intel SGX Root CA** — self-signed root, distributed out-of-band.
2. **Intel SGX Processor CA** *or* **Intel SGX Platform CA** — intermediate
   CA. Intel issues two distinct intermediates (multi-package server
   platforms get Platform CA; single-package processors get Processor CA).
3. **PCK leaf certificate** — issued per platform per TCB level. Carries
   non-standard X.509 extensions encoding the platform's `CPUSVN`,
   `PCESVN`, and `FMSPC` (Family-Model-Stepping-Platform-Customization)
   tuple — the verifier reads these out of the cert to look up the right
   TCB Info entry.

Source:
[Safeheron — Demystify Remote Attestation: Explore the DCAP Certificate Chain](https://safeheron.com/blog/what-is-remote-attestation/),
[enclaive — DCAP Attestation Framework](https://docs.enclaive.cloud/confidential-cloud/technology-in-depth/intel-sgx/technology/concepts/dcap-attestation-framework).

Intel also publishes a **PCK CRL** signed by each intermediate CA and a
**Root CRL** signed by the root.

### 3.4 TCB Info JSON

Served by PCS at
`/sgx/certification/v4/tcb?fmspc={fmspc}` (SGX) and
`/tdx/certification/v4/tcb?fmspc={fmspc}` (TDX). The verifier walks the
ordered `tcbLevels[]` array and finds the first level where every SVN in
`tcb.sgxtcbcomponents[i].svn` is ≤ the platform's reported component SVN
(plus `tcb.pcesvn` ≤ platform PCESVN). The matched level's `tcbStatus` is
one of:

- `UpToDate`
- `OutOfDate`
- `OutOfDateConfigurationNeeded`
- `ConfigurationNeeded`
- `ConfigurationAndSWHardeningNeeded`
- `SWHardeningNeeded`
- `Revoked`

The `advisoryIDs[]` array lists the Intel security advisories that apply
at this level. The JSON envelope is signed by the **TCB Signing CA** (a
separate chain from the PCK chain). Source:
[Intel PCS API portal](https://api.portal.trustedservices.intel.com/provisioning-certification),
[Intel SGX DCAP Orientation Guide](https://download.01.org/intel-sgx/latest/dcap-latest/linux/docs/DCAP_ECDSA_Orientation.pdf).

For TDX, the structure is the same but adds a `tdxtcbcomponents[]` array
that pins the TDX Module SVN.

### 3.5 QE Identity JSON

Pins the legitimate QE / TDQE measurement so the verifier can confirm the
quote came from an Intel-signed QE, not an arbitrary enclave on the
platform. Compared field-for-field against the embedded QE report inside
the Quote. Also signed by the TCB Signing CA.

---

## 4. SGX vs TDX — comparison

| Dimension                  | SGX                                     | TDX                                              |
|----------------------------|------------------------------------------|--------------------------------------------------|
| Trust boundary             | Single enclave (one user process)        | Full guest VM (Trust Domain)                     |
| Code authoring             | Special "enclave" binary, signed at build | Unmodified guest OS + workload                   |
| Measurement granularity    | Build-time (`MRENCLAVE`); pinned at `EINIT` | Build-time (`MRTD`); runtime extends (`RTMR`)  |
| Hash function              | SHA-256                                  | SHA-384                                          |
| Memory                     | EPC: ≤ ~256 MB before SGX2, GB-scale after | Whole-VM RAM (limited by host)                 |
| Attestation key            | ECDSA-P256 (DCAP)                        | ECDSA-P256 (DCAP, via TDQE)                      |
| TCB recovery cadence       | Per platform package; Intel-driven         | TDX Module SVN + platform SVN; Intel-driven    |
| Workload fit               | Small attested compute, key custodians, side-car services | Full workloads, unmodified app stacks |
| Memory encryption          | MEE (Memory Encryption Engine), TME-keyed | MKTME (Multi-Key TME), per-TD key              |

The decision rule for Tide is: if the workload is a **specific code unit**
(a signer, a small policy oracle, an attested key vault), SGX is a tighter
fit because `MRENCLAVE` directly pins the bits. If the workload is a
**conventional containerised process or VM** (an LLM agent runtime, a
sidecar that needs unmodified Linux), TDX is the only practical option;
the verifier policy then has to pin both `MRTD` and the relevant
`RTMR[i]` values.

---

## 5. Open-standards alignment

### 5.1 IETF RATS RFC 9334

[RFC 9334](https://datatracker.ietf.org/doc/rfc9334/) — "Remote ATtestation
procedureS (RATS) Architecture" — is the IETF reference model. SGX/TDX
DCAP maps onto it as:

- **Attester** = the (enclave / TD) + QE/TDQE.
- **Verifier** = the off-platform DCAP verifier (the Solidity contract in
  Tide's case, or a `quote-verification-library` instance).
- **Relying Party** = the contract / service consuming the attestation
  result.
- **Endorser** = Intel PCS (issues PCK certs and TCB Info).
- **Reference Value Provider** = whoever sets the policy values (the on-
  chain registry of allowed `MRENCLAVE` / `MRTD` / `RTMR[i]` values).

DCAP is a *passport*-pattern flow in RFC-9334 terms: the attester collects
its quote and presents it to relying parties without round-tripping a
verifier on every check.

### 5.2 EPID is deprecated

Intel formally deprecated EPID-based attestation:

> "EPID will be EOL for production environments on April 2, 2025, and is
> already broken for developer's keys."

Source:
[Gramine — Introduction to SGX](https://gramine.readthedocs.io/en/latest/sgx-intro.html),
[Intel community: DCAP/ECDSA and IAS](https://community.intel.com/t5/Intel-Software-Guard-Extensions/DCAP-ECDSA-and-IAS/m-p/1380032).

Hardware support is asymmetric: "Scalable series processors support only
ECDSA DCAP, while Intel Xeon E/D series processors supporting SGX with
Intel SPS feature can support both EPID IAS and ECDSA DCAP." Any new
build — and certainly Tide's on-chain verifier — should be ECDSA DCAP
only.

### 5.3 EAT (Entity Attestation Token) mappings

Intel publishes EAT profile mappings for TDX:
[draft-kdyxy-rats-tdx-eat-profile-01](https://www.ietf.org/archive/id/draft-kdyxy-rats-tdx-eat-profile-01.html)
and an Intel Trust Authority EAT profile
([portal.trustauthority.intel.com/eat_profile.html](https://portal.trustauthority.intel.com/eat_profile.html)).
Microsoft Azure Attestation also publishes a TDX EAT profile
([learn.microsoft.com/en-us/azure/attestation/trust-domain-extensions-eat-profile](https://learn.microsoft.com/en-us/azure/attestation/trust-domain-extensions-eat-profile)).
RFC 9711 (the EAT base spec) gives the CBOR/JWT envelope; the Intel/
Microsoft profiles add the SGX/TDX-specific claim names (`mrtd`,
`rtmr0..3`, `mrenclave`, `mrsigner`, …).

For Tide: relying on EAT is *not* required at the verifier — a Solidity
verifier reads the raw DCAP quote bytes — but it is useful if Tide ever
wants to consume an attestation that has already been verified by an
off-chain service (Intel Trust Authority, Azure Attestation), since those
services emit JWT-EAT.

---

## 6. Cryptographic primitives

| Primitive                   | Where used                                              | Source |
|-----------------------------|---------------------------------------------------------|--------|
| **ECDSA-P256 / SHA-256**    | Quote signature, PCK signature, TCB Info JWT signature  | [Intel SGX DCAP Orientation Guide](https://download.01.org/intel-sgx/latest/dcap-latest/linux/docs/DCAP_ECDSA_Orientation.pdf), [enclaive](https://docs.enclaive.cloud/confidential-cloud/technology-in-depth/intel-sgx/technology/concepts/dcap-attestation-framework) |
| **AES-CMAC**                | SGX local-attestation `REPORT.MAC`                      | [Intel SDM Vol. 3D, EREPORT] |
| **HMAC-SHA-256**            | TDX `REPORTMACSTRUCT.MAC`                               | [Intel TDX 1.5 Base Spec](https://cdrdv2-public.intel.com/733575/intel-tdx-module-1.5-base-spec-348549002.pdf) |
| **SHA-256**                 | `MRENCLAVE`, `MRSIGNER`, all SGX measurements           | FIPS PUB 180-4 |
| **SHA-384**                 | `MRTD`, `RTMR[i]`, `MR_SEAM`, all TDX measurements      | [Intel TDX 1.5 Base Spec](https://cdrdv2-public.intel.com/733575/intel-tdx-module-1.5-base-spec-348549002.pdf) |
| **RSA-3072 / OAEP**         | PPID encryption to Intel during platform registration   | [enclaive — DCAP Attestation Framework](https://docs.enclaive.cloud/confidential-cloud/technology-in-depth/intel-sgx/technology/concepts/dcap-attestation-framework) |
| **RSA-3072 / RSA-SSA**      | Enclave signing (the `MRSIGNER` modulus is RSA-3072)    | Intel SGX SDK SIGSTRUCT |

Curves and hashes are *not* configurable in DCAP — every production
deployment uses P-256 + SHA-256 for ECDSA and SHA-384 for TDX
measurements. This is the single biggest cost advantage Intel attestation
has over AMD SEV-SNP attestation, which uses ECDSA-P384 (more expensive on
EVM).

---

## 7. On-chain verification cost (EVM)

### 7.1 Real implementations

- **Automata DCAP Attestation** — production Solidity. v1.1 deployed to
  20+ chains (Optimism, HyperEVM, World Chain, etc.); supports Quote v3,
  v4, v5; SGX and TDX. The architecture is three contracts:
  1. `PCCSRouter` — reads PCK certs / TCB Info / QE Identity from an
     on-chain PCCS mirror.
  2. `AutomataDcapAttestation` — entrypoint; routes by quote version.
  3. version-specific `QuoteVerifierV3 / V4 / V5` — parses + checks.
  - Repo: [automata-network/automata-dcap-attestation](https://github.com/automata-network/automata-dcap-attestation).
  - On-chain PCCS: [automata-network/automata-on-chain-pccs](https://github.com/automata-network/automata-on-chain-pccs).
- **Phala** — `ts-sgx-quote-verify` and a Phala-published DCAP verifier
  used by their TEE coprocessor.
- **Optimism's "kailua" verifier** — wraps Automata's library for use as
  a fault proof oracle.

### 7.2 Gas profile (Automata DCAP v1.1)

Per Automata's published numbers
([blog.ata.network/automatas-release-of-dcap-attestation-v1-1-for-agentic-systems](https://blog.ata.network/automatas-release-of-dcap-attestation-v1-1-for-agentic-systems-84ae98900370)):

| Path                                | Approx total gas |
|-------------------------------------|------------------|
| Pure on-chain, no precompile        | ~5,000,000 gas   |
| Pure on-chain, RIP-7212 P256 precompile | ~4,000,000 gas |
| Pure on-chain, EIP-7951 P256 precompile (v1.1) | ~3,000,000 gas (P-256 step alone drops from ~330k → 6k) |
| RiscZero Groth16 proof              | ~522,000 gas     |
| SP1 Groth16 proof                   | ~493,000 gas     |
| SP1 Plonk proof                     | ~569,000 gas     |

The "pure on-chain" number is dominated by three things:

1. **ECDSA-P256 signature checks** — the quote sig (over the body), the
   QE report sig by the PCK, and one cert-chain sig per layer. Without a
   P-256 precompile, each is ~300k gas; with EIP-7951 each is ~6k gas.
2. **X.509 cert-chain parsing** — the PCK cert + Processor/Platform CA
   intermediate + Root CA = 3 layers. Each parse + signature verify is
   the bulk of pre-precompile cost.
3. **TCB Info JSON parsing** — JSON parsing in Solidity is expensive; the
   on-chain PCCS pre-parses the structured fields so the runtime cost is
   keccak comparisons rather than re-parsing.

### 7.3 Sei EVM applicability

Sei EVM is chain ID 1329, an Ethereum-compatible EVM with the same gas
model. **Sei has already implemented RIP-7212**, exposing a P256VERIFY
precompile at `address(0x100)` at a cost of 3,450 gas per verification —
roughly 60× cheaper than a Solidity P-256 implementation. Source:
[Sei docs — P256 Precompile](https://docs.sei.io/evm/precompiles/p256-precompile).

This places Sei in the "with P-256 precompile" gas tier: an Automata-
class Intel DCAP verifier on Sei should land in the **~3-4M gas range**
per quote, not the ~5M-without-precompile tier.

The remaining cost drivers are:

- **PCK cert + TCB Info delivery.** Either a Tide-operated on-chain PCCS
  mirror (Automata pattern), or zk-proof handoff (RiscZero / SP1 / Pico
  pattern). The zk route trades verifier complexity for proof-generation
  off-chain, dropping on-chain gas to ~500k.
- **X.509 ASN.1 parsing.** The cert chain (Root → Processor/Platform →
  PCK leaf) requires DER parsing in Solidity. This is the next-largest
  cost component after signature verification.
- **TCB Info JSON keccak comparisons** against on-chain mirrored data.

Compared to AMD SEV-SNP on-chain verification (~7-8M gas due to ECDSA-
P384 + signed VCEK chain), Intel DCAP on Sei is roughly **half the on-
chain cost** at the same security level — and Sei's existing P-256
precompile is what makes this true today, not a future Sei roadmap item.

---

## 8. Defense-in-depth: known SGX / TDX attacks

A production verifier — on-chain or otherwise — has to assume the SGX/TDX
trust boundary is not perfect. The relevant attack surface and the
mechanisms the verifier needs to honour:

### 8.1 SGX

- **Foreshadow (L1TF, 2018).** Transient-execution attack extracting SGX
  sealing and report keys; an attacker who exploits it can forge local
  attestation reports.
  [Foreshadow paper — USENIX Security 2018](https://www.usenix.org/system/files/conference/usenixsecurity18/sec18-van_bulck.pdf).
  Mitigation: microcode + TCB bump; the verifier rejects quotes with
  insufficient `CPUSVN` via TCB Info `tcbStatus`.
- **Plundervolt (CVE-2019-11157).** Voltage-fault injection via MSR
  write; faults SGX-protected computations.
  [plundervolt.com](https://plundervolt.com/).
  Mitigation: microcode disables undervolting; TCB Info reflects the fix.
- **SGAxe (2020).** MDS-based extraction of SGX attestation keys; could
  sign counterfeit quotes outside SGX.
  [SGAxe paper](https://sgaxe.com/files/SGAxe.pdf).
  Mitigation: TCB recovery / re-keying; verifier must check `tcbStatus`
  and refuse `Revoked` quotes.
- **ÆPIC Leak, CacheOut, LVI, ZombieLoad, SMoTherSpectre, etc.** —
  collected at
  [sgx.fail](https://sgx.fail/).
  Each maps to one or more `advisoryIDs` in the TCB Info JSON. A verifier
  that ignores `advisoryIDs` is one CVE away from accepting forged
  quotes.

**Verifier obligation:** reject any quote where the resolved TCB level is
`Revoked`. For all other levels, surface `tcbStatus` and `advisoryIDs`
to the relying party — *do not silently treat `OutOfDate` as acceptable*.

### 8.2 TDX

Fewer public attacks than SGX (newer; smaller deployed surface). The
load-bearing public reviews are:

- **Google Project Zero, 2023.** "Release of a Technical Report into
  Intel Trust Domain Extensions."
  [projectzero.google/2023/04/technical-report-into-intel-tdx.html](https://projectzero.google/2023/04/technical-report-into-intel-tdx.html).
  Found 10 confirmed vulns out of 81 candidate attack vectors. Nine
  fixed in the TDX Module before final shipping; one BIOS-guide change.
- **Google / Intel 2025 security review** (TDX 1.5 live-migration &
  Trusted Domain Partitioning).
  [securityweek.com — Google-Intel TDX audit](https://www.securityweek.com/google-intel-security-audit-reveals-severe-tdx-vulnerability-allowing-full-compromise/).
  One full-TD-compromise vuln + four confidential-memory leaks
  (mitigated before GA).

**Verifier obligation:** TDX TCB Info includes a `tdxtcbcomponents[]`
array pinning the TDX Module SVN. Treat it with the same discipline as
the SGX `tcbStatus` — reject `Revoked`, surface `OutOfDate`.

---

## 9. Empirical references (one-line each)

- Intel SGX Developer Documentation portal:
  [download.01.org/intel-sgx/latest/dcap-latest/linux/docs/](https://download.01.org/intel-sgx/latest/dcap-latest/linux/docs/)
- Intel SGX DCAP source:
  [github.com/intel/SGXDataCenterAttestationPrimitives](https://github.com/intel/SGXDataCenterAttestationPrimitives)
- Intel TDX tools:
  [github.com/intel/tdx-tools](https://github.com/intel/tdx-tools)
- Intel TDX Module 1.5 Base Architecture Spec (348549002):
  [cdrdv2-public.intel.com/733575/…](https://cdrdv2-public.intel.com/733575/intel-tdx-module-1.5-base-spec-348549002.pdf)
- Intel TDX Module 1.5 TD Migration Spec:
  [cdrdv2-public.intel.com/839190/…](https://cdrdv2-public.intel.com/839190/intel-tdx-module-1.5-td-migration-spec-348550003.pdf)
- Intel SGX ECDSA Quote Library Reference (DCAP):
  [download.01.org/.../Intel_SGX_ECDSA_QuoteLibReference_DCAP_API.pdf](https://download.01.org/intel-sgx/sgx-dcap/1.3/linux/docs/Intel_SGX_ECDSA_QuoteLibReference_DCAP_API.pdf)
- Intel TDX DCAP Quoting Library API:
  [download.01.org/.../Intel_TDX_DCAP_Quoting_Library_API.pdf](https://download.01.org/intel-sgx/latest/dcap-latest/linux/docs/Intel_TDX_DCAP_Quoting_Library_API.pdf)
- Intel SGX DCAP Orientation Guide:
  [download.01.org/.../DCAP_ECDSA_Orientation.pdf](https://download.01.org/intel-sgx/latest/dcap-latest/linux/docs/DCAP_ECDSA_Orientation.pdf)
- Intel DCAP PCCS Design Guide:
  [download.01.org/.../SGX_DCAP_Caching_Service_Design_Guide.pdf](https://download.01.org/intel-sgx/sgx-dcap/1.10/linux/docs/SGX_DCAP_Caching_Service_Design_Guide.pdf)
- Intel PCS portal (production):
  [api.portal.trustedservices.intel.com/provisioning-certification](https://api.portal.trustedservices.intel.com/provisioning-certification)
- Automata DCAP Attestation (Solidity):
  [github.com/automata-network/automata-dcap-attestation](https://github.com/automata-network/automata-dcap-attestation)
- Automata on-chain PCCS:
  [github.com/automata-network/automata-on-chain-pccs](https://github.com/automata-network/automata-on-chain-pccs)
- Phala — Understanding TDX Attestation Reports:
  [phala.com/posts/understanding-tdx-attestation-reports-a-developers-guide](https://phala.com/posts/understanding-tdx-attestation-reports-a-developers-guide)
- IETF RFC 9334 (RATS Architecture):
  [datatracker.ietf.org/doc/rfc9334/](https://datatracker.ietf.org/doc/rfc9334/)
- IETF draft EAT-TDX profile:
  [ietf.org/archive/id/draft-kdyxy-rats-tdx-eat-profile-01.html](https://www.ietf.org/archive/id/draft-kdyxy-rats-tdx-eat-profile-01.html)
- Intel Trust Authority EAT profile:
  [portal.trustauthority.intel.com/eat_profile.html](https://portal.trustauthority.intel.com/eat_profile.html)
- "Attestation Mechanisms for Trusted Execution Environments Demystified" (arXiv 2206.03780):
  [arxiv.org/pdf/2206.03780](https://arxiv.org/pdf/2206.03780)
- SGX vulnerabilities clearinghouse: [sgx.fail](https://sgx.fail/)
- Foreshadow paper:
  [usenix.org/.../sec18-van_bulck.pdf](https://www.usenix.org/system/files/conference/usenixsecurity18/sec18-van_bulck.pdf)
- SGAxe paper: [sgaxe.com/files/SGAxe.pdf](https://sgaxe.com/files/SGAxe.pdf)
- Plundervolt: [plundervolt.com](https://plundervolt.com/)
- Google Project Zero TDX report (2023):
  [projectzero.google/2023/04/technical-report-into-intel-tdx.html](https://projectzero.google/2023/04/technical-report-into-intel-tdx.html)
- Google-Intel TDX security audit (2025):
  [securityweek.com — Google-Intel TDX audit](https://www.securityweek.com/google-intel-security-audit-reveals-severe-tdx-vulnerability-allowing-full-compromise/)

---

## Load-bearing claims (for `tee-specialist`)

The following claims are the ones the `tee-specialist` agent MUST treat
as primitive — every design decision flows from them.

1. **SGX measurement = SHA-256, 32 bytes; TDX measurement = SHA-384, 48
   bytes.** Any verifier that confuses these will fail. Source:
   [Open Enclave SGX types](https://github.com/openenclave/openenclave/blob/master/include/openenclave/bits/sgx/sgxtypes.h),
   [Phala TDX guide](https://phala.com/posts/understanding-tdx-attestation-reports-a-developers-guide).
2. **`MRENCLAVE` pins the SGX enclave binary; `MRTD` pins only the TD
   launch image.** TDX identity REQUIRES `RTMR[0..3]` policy values in
   addition to `MRTD`. A `MRTD`-only policy is exploitable — any kernel
   loadable into the same TDVF passes.
3. **EPID is deprecated (EOL 2025-04-02). All new attestation MUST use
   ECDSA-DCAP.** Source:
   [Gramine SGX intro](https://gramine.readthedocs.io/en/latest/sgx-intro.html).
4. **The DCAP attestation key is ECDSA-P256 over NIST P-256, with the
   PCK leaf cert also using P-256.** This is what makes Intel DCAP
   on-chain verification cheaper than AMD SEV-SNP (P-384). Source:
   [enclaive — DCAP Attestation Framework](https://docs.enclaive.cloud/confidential-cloud/technology-in-depth/intel-sgx/technology/concepts/dcap-attestation-framework).
5. **The verifier MUST check `ATTRIBUTES.DEBUG = 0`.** A debug enclave is
   inspectable; trusting one is trivially exploitable. Source:
   [Intel SGX SDM, EREPORT semantics].
6. **The verifier MUST resolve `tcbStatus` against the PCS-served TCB
   Info JSON and MUST reject `Revoked`.** Silently accepting `OutOfDate`
   = accepting known-vulnerable platforms. Source:
   [Intel PCS portal](https://api.portal.trustedservices.intel.com/provisioning-certification),
   [Intel DCAP Orientation](https://download.01.org/intel-sgx/latest/dcap-latest/linux/docs/DCAP_ECDSA_Orientation.pdf).
7. **REPORT_DATA / REPORTDATA (64 bytes, last field of the body) is the
   only verifier-controlled binding.** Everything else in the body is
   measured / hardware-derived. The relying party uses REPORT_DATA to bind
   the quote to a session pubkey / nonce / claim. Source:
   [Open Enclave SGX types](https://github.com/openenclave/openenclave/blob/master/include/openenclave/bits/sgx/sgxtypes.h).
8. **PCK cert chain has 3 layers: Intel SGX Root CA → Processor *or*
   Platform CA → PCK leaf.** TCB Info JSON is signed by a *separate* TCB
   Signing CA, also chained off the SGX Root. Source:
   [Safeheron — DCAP cert chain](https://safeheron.com/blog/what-is-remote-attestation/).
9. **Quote v4 unifies SGX + TDX behind a `tee_type` discriminator and a
   typed body (`enclave_report` 384 bytes vs `td_report` 584/648 bytes);
   v4 mandates certification-data type 6 (PEM cert chain).** A single
   verifier contract can dispatch on `tee_type`. Source:
   [DeepWiki — Quote Data Structures](https://deepwiki.com/intel/SGX-TDX-DCAP-QuoteVerificationLibrary/3.3-quote-data-structures).
10. **Realistic on-chain gas: ~3M with EIP-7951/RIP-7212 P-256 precompile,
    ~5M without; ~500k via zk-proof (Automata zkVM path).** Source:
    [Automata DCAP v1.1 announcement](https://blog.ata.network/automatas-release-of-dcap-attestation-v1-1-for-agentic-systems-84ae98900370).
11. **Sei EVM has RIP-7212 deployed — P256VERIFY at `address(0x100)`,
    3,450 gas per call (~60× cheaper than Solidity).** This is the
    deciding cost lever, and it is already in place. Source:
    [Sei docs — P256 Precompile](https://docs.sei.io/evm/precompiles/p256-precompile).
