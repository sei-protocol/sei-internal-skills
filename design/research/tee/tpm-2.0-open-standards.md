# TPM 2.0 and the Open Standards Ecosystem for Attestation

**Status:** Design research, draft
**Last updated:** 2026-06-01
**Audience:** Tide `tee-specialist` agent (cross-vendor grounding)
**Companion docs:** AMD SEV-SNP, Intel SGX/TDX, NVIDIA Confidential Compute, AWS Nitro Enclaves research streams

This document captures the *standards layer* that sits underneath (or alongside)
the per-vendor TEE attestation flows. Several vendors plug into these standards
(IETF RATS, EAT, TCG measurement model, DICE, SPDM); some bypass them; some define
profiles. The cross-vendor mapping table at the end summarises where each lands.

---

## 1. TPM 2.0 Architecture

### 1.1 The TPM 2.0 Library Specification

The authoritative reference for TPM 2.0 is the **TCG TPM 2.0 Library Specification**,
maintained by the Trusted Computing Group (TCG). It is published as four parts:

- **Part 1: Architecture** — concepts, hierarchies, sessions, protections, algorithms
- **Part 2: Structures** — wire formats for all TPM commands/responses
- **Part 3: Commands** — definition of every TPM 2.0 command (`TPM2_*`)
- **Part 4: Supporting Routines** — reference C code

Source: [TCG TPM 2.0 Library Specification page](https://trustedcomputinggroup.org/resource/tpm-library-specification/);
the canonical PDF for Part 1 has been published as
[TPM-Rev-2.0-Part-1-Architecture-01.38.pdf](https://trustedcomputinggroup.org/wp-content/uploads/TPM-Rev-2.0-Part-1-Architecture-01.38.pdf).

A useful TCG companion is the **PC Client Platform TPM Profile (PTP) Specification
for TPM 2.0**, which constrains the library spec to the PC platform
([PC-Client-Specific-Platform-TPM-Profile-for-TPM-2p0-v1p07_rc1_12Dec2025.pdf](https://trustedcomputinggroup.org/wp-content/uploads/PC-Client-Specific-Platform-TPM-Profile-for-TPM-2p0-v1p07_rc1_12Dec2025.pdf)).

### 1.2 Hierarchy model

TPM 2.0 separates control surfaces into **four hierarchies**, each rooted in a
distinct **Primary Seed** stored inside the TPM:

| Hierarchy | Root Seed | Owner / Purpose |
|-----------|-----------|-----------------|
| **Platform Hierarchy (PH)** | Platform Primary Seed (PPS) | Owned by the platform firmware/manufacturer; used for firmware-level operations |
| **Storage Hierarchy (SH)** | Storage Primary Seed (SPS) | Owned by the platform owner / OS; root for user storage keys |
| **Endorsement Hierarchy (EH)** | Endorsement Primary Seed (EPS) | Owned by the TPM vendor (privacy-sensitive); root for the Endorsement Key |
| **Null Hierarchy** | Null Seed (regenerated each reset) | Ephemeral; objects under Null vanish on TPM reset |

Each hierarchy has an independent owner-authorization value, an independent
enable/disable control, and produces objects that are cryptographically bound
to that hierarchy's seed. The seeds never leave the TPM; primary objects under
a hierarchy are *derived deterministically* from the seed plus the object's
template, so a primary key can be recreated without storage.

Sources:
- [TCG TPM 2.0 Library Part 1 Architecture (01.38)](https://trustedcomputinggroup.org/wp-content/uploads/TPM-Rev-2.0-Part-1-Architecture-01.38.pdf)
- [TPM 2.0 Library Specification: The Parts (ebrary summary)](https://ebrary.net/24734/computer_science/library_specification_parts)

The Platform Hierarchy is intended for the platform manufacturer's firmware;
the Storage and Endorsement hierarchies (and Null) are used by OS-present
applications.

### 1.3 Key types

The keys the rest of the attestation world cares about all live in (or are
certified by) these hierarchies:

- **Storage Root Key (SRK)** — a Primary Object in the Storage Hierarchy,
  derived from the SPS plus a fixed template. Acts as the wrapping root for
  user storage keys. It is regenerated deterministically each time it is needed
  (no persistent storage required, though it can be made persistent).
- **Endorsement Key (EK)** — a Primary Object in the Endorsement Hierarchy,
  derived from the EPS plus the "EK template" (TCG-defined). Because the EPS
  is provisioned at manufacture and never changes, the EK is *intrinsic to the
  TPM* — it is the cryptographic identity of the chip itself. The TPM vendor
  issues an **EK Certificate** chaining the EK to the vendor's CA, baked into
  the TPM at manufacture.
- **Attestation Key (AK), historically "AIK"** — a restricted signing key
  under the Endorsement (or Storage) Hierarchy, used to sign attestation
  output (Quotes, Certifications). The AK is *not* the EK; it is a separate,
  often ephemeral, key. The EK is *not* allowed to sign arbitrary attestations
  (it is `restricted` + `decrypt` only on most templates), so the AK exists to
  carry signing authority while preserving privacy.

### 1.4 AK certification — the EK→AK credential

Because the EK has a manufacturer certificate but is privacy-sensitive (it
uniquely identifies the TPM, like a serial number), TPM 2.0 attestation flows
typically:

1. Create a fresh AK inside the TPM.
2. Run **`TPM2_MakeCredential` / `TPM2_ActivateCredential`** to prove to a
   Privacy CA (or directly to a verifier) that "this AK is in the same TPM
   as the EK whose certificate I'm showing you."
3. The Privacy CA issues an **AK Certificate** binding the AK's public key
   to the holder's identity attributes (without revealing the EK).

This is the classic mechanism that lets a verifier accept a `TPM2_Quote`
signed by the AK with the same trust they would place in the EK manufacturer
certificate — without exposing the EK across remote attestation exchanges.

### 1.5 Object lifetime — transient vs persistent vs NV

TPM 2.0 objects come in three flavours:

- **Transient** — live in TPM RAM, addressed by a session-local handle, lost
  on `TPM2_FlushContext` or TPM reset. Most working keys are transient.
- **Persistent** — moved to TPM NV with `TPM2_EvictControl`. Addressed by a
  fixed handle in the persistent handle range (`0x81000000..0x81FFFFFF`).
  NV is small (typically a few KB), so very few keys live here — usually
  just the EK, SRK, and a long-lived AK.
- **NV Indexes** — separate NV storage cells (`TPM2_NV_DefineSpace` /
  `TPM2_NV_Write` / `TPM2_NV_Read`), used for monotonic counters, sealed
  PCR policy data, vendor certificates, etc.

---

## 2. PCR semantics

### 2.1 The 24 PCRs and their standardised usage

TPM 2.0 PC Client profile defines **24 Platform Configuration Registers**
(PCR[0..23]) per supported hash algorithm bank (typically SHA-1 and SHA-256
in legacy systems, SHA-256/SHA-384 in modern). PCR usage on PC platforms is
standardised by the
[TCG PC Client Platform Firmware Profile (PFP)](https://trustedcomputinggroup.org/resource/pc-client-specific-platform-firmware-profile-specification/):

| PCR | Owner | Typical contents |
|-----|-------|------------------|
| 0   | SRTM  | CRTM, BIOS, Host Platform Extensions, Embedded Option ROMs, PI drivers |
| 1   | SRTM  | Host Platform Configuration (SMBIOS, ACPI tables, etc.) |
| 2   | SRTM  | UEFI driver code from add-in cards (Option ROMs) |
| 3   | SRTM  | UEFI driver configuration (from add-in cards) |
| 4   | SRTM  | UEFI Boot Manager Code and boot attempts (bootloader hash) |
| 5   | SRTM  | UEFI Boot Manager configuration / GPT |
| 6   | SRTM  | Host Platform Manufacturer specific |
| 7   | SRTM  | Secure Boot policy (PK, KEK, db, dbx, SecureBoot enable flag) |
| 8   | OS    | Bootloader-stage measurements (e.g. GRUB hashes kernel into PCR 8/9) |
| 9   | OS    | Linux IMA / kernel cmdline / initrd |
| 10  | OS    | IMA (Linux Integrity Measurement Architecture) |
| 11–15 | OS / app | Reserved for OS, application-level measurements |
| 16  | Debug | Resettable on the locality used for debug (PCR is *not* trusted) |
| 17  | DRTM  | MLE (Measured Launch Environment) hash — Intel TXT / AMD SKINIT |
| 18  | DRTM  | Trusted OS startup module |
| 19  | DRTM  | Trusted OS — kernel measurements |
| 20  | DRTM  | Trusted OS — application configuration |
| 21  | DRTM  | Defined by trusted OS |
| 22  | DRTM  | Defined by trusted OS |
| 23  | App   | Application support (resettable from locality 0 — for application use) |

Source: TCG PC Client Platform Firmware Profile Specification (PFP)
([trustedcomputinggroup.org](https://trustedcomputinggroup.org/resource/pc-client-specific-platform-firmware-profile-specification/)).

### 2.2 The Extend operation

PCRs are not writable. The only mutation is `TPM2_PCR_Extend`:

```
PCR_new = HASH( PCR_old || measurement )
```

— a cryptographic accumulation. Each PCR bank uses its own hash (SHA-256 is
universal in modern PC platforms; SHA-384 is increasingly required by
confidential-compute profiles; SHA-1 is deprecated but still present in
banks for backwards compat). Because hash is collision-resistant, the final
PCR value commits to the entire ordered sequence of measurements, and a
verifier can replay an event log to confirm the trace.

### 2.3 Reset semantics

Reset rules differ by PCR group:

- **PCR 0–15**: reset *only* on full platform reset (cold or warm). Software
  cannot reset them via TPM command — this is what makes them durable
  measurement registers.
- **PCR 16**: resettable at any time from any locality; used for debug. Never
  trust PCR 16 in attestations.
- **PCR 17–22**: resettable *only from Locality 4* (the DRTM locality). The
  CPU enters Locality 4 via the DRTM late-launch instruction
  (`GETSEC[SENTER]` on Intel, `SKINIT` on AMD), which *also* extends PCR 17
  with the MLE measurement.
- **PCR 23**: resettable from Locality 0 by application code.

(See TCG PC Client PFP §3.3 "PCR Reset Locality", and the architecture spec
"Locality" definition in Part 1.)

---

## 3. RTM, SRTM, and DRTM

### 3.1 Definitions

- **RTM (Root of Trust for Measurement)** — the immutable element that takes
  the *first measurement* in the measurement chain. By construction it cannot
  measure itself, so it must be trusted *by location* (in firmware ROM) or
  *by hardware* (microcode).
- **SRTM (Static RTM)** — the **CRTM** (Core RTM) located in firmware
  BootBlock measures the next stage (BIOS/UEFI), which measures the next,
  building a chain from power-on through bootloader through kernel. All
  measurements feed PCR 0–7.
- **DRTM (Dynamic RTM)** — a *late-launch* primitive that lets software
  initiate a fresh, hardware-rooted measurement chain *at any time*, without
  rebooting. On Intel this is `GETSEC[SENTER]` (TXT); on AMD this is
  `SKINIT` (originally Presidio/SVM). The CPU enters a special locality,
  PCR 17–22 are reset to 0, PCR 17 is extended with the **MLE hash**, and
  control transfers to the measured environment.

### 3.2 Why DRTM matters

DRTM lets you ignore the entire boot history (firmware, bootloader, OS) for
attestation purposes: the relevant measurement is the late-launched MLE.
This is exactly what Intel TXT did for "trusted launch" and what Trenchboot
revives in modern Linux for measured launch via `SKINIT` / `SENTER`. For
confidential VMs the analogous concept is the per-VM measurement (TDX MRTD,
SEV-SNP `MEASUREMENT`) which conceptually plays the DRTM role inside a guest.

### 3.3 `TPM2_Quote` vs `TPM2_Certify`

The library defines two distinct attestation surface commands:

- **`TPM2_Quote`** — the TPM signs over a `TPMS_QUOTE_INFO` structure,
  which embeds the selected PCRs (`pcrSelect`), a digest of those PCRs
  (`pcrDigest`), a caller-supplied `qualifyingData` (nonce), `clockInfo`,
  and `firmwareVersion`. This is the "what is the platform state?" command.
- **`TPM2_Certify`** — the TPM signs a statement about *another TPM object*
  (e.g. "this key has these attributes, is bound to this hierarchy"). Used
  to certify AKs, prove key residency, etc.

Both return a `TPMS_ATTEST` structure (signed) and a separate signature.

---

## 4. Attestation flow

### 4.1 `TPM2_Quote` signed structure

The signed payload (`TPMS_ATTEST`, defined in Part 2: Structures, §10.12) is:

```
TPMS_ATTEST {
  magic              TPM_GENERATED_VALUE   // 0xFF544347 ("\xffTCG")
  type               TPMI_ST_ATTEST        // 0x8018 for Quote
  qualifiedSigner    TPM2B_NAME            // qualified name of the AK
  extraData          TPM2B_DATA            // caller's nonce / qualifyingData
  clockInfo          TPMS_CLOCK_INFO       // clock, resetCount, restartCount, safe
  firmwareVersion    UINT64                // vendor-defined firmware version
  attested           TPMU_ATTEST           // type-specific body (TPMS_QUOTE_INFO for Quote)
}

TPMS_QUOTE_INFO {
  pcrSelect          TPML_PCR_SELECTION    // which PCRs (per bank) were quoted
  pcrDigest          TPM2B_DIGEST          // digest over the concatenation of selected PCRs
}
```

Key safety property: `magic = TPM_GENERATED_VALUE = 0xFF544347` is the first
field. Because non-restricted user keys are *prohibited by the TPM* from
signing buffers that start with `TPM_GENERATED_VALUE`, you can be certain a
signature over such a structure came from a TPM-restricted key path.

### 4.2 Signature algorithms

Modern TPM 2.0 AKs are typically:

- **ECDSA on NIST P-256 with SHA-256** (`TPM_ALG_ECDSA` + `TPM_ALG_SHA256`),
  or P-384 / SHA-384 for higher-assurance profiles.
- **RSASSA-PKCS1-v1_5 with SHA-256** for 2048-bit RSA AKs.

The PC Client profile mandates SHA-256 support in all TPM 2.0 chips; SHA-384
support is increasingly required for confidential-compute profiles.

### 4.3 AK certification chain to a verifier

Putting §1.4 and §4.1 together, a verifier presented with a `TPM2_Quote`
runs the following:

1. **AK Certificate validation** — verify the AK certificate chains back to
   a TPM-vendor CA (Infineon, ST Micro, Nuvoton, Microsoft vTPM, etc.) via
   the EK certificate. (For privacy-CA flows, validate the privacy CA's
   binding instead.)
2. **Signature check** — verify the quote signature with the AK public key.
3. **Magic check** — confirm `magic == TPM_GENERATED_VALUE`.
4. **Type check** — confirm `type == TPM_ST_ATTEST_QUOTE`.
5. **Nonce check** — confirm `extraData == nonce` issued by the verifier.
6. **PCR replay** — verify the event log replayed against the named PCR
   bank produces a digest equal to `pcrDigest`.
7. **Policy check** — match the resulting PCR set against the verifier's
   reference values (golden PCRs / RIM).

---

## 5. IETF RATS framework — RFC 9334

The IETF **Remote ATtestation procedureS (RATS)** working group standardised
a vendor-neutral architecture in **RFC 9334**, published **January 2023**.
The architecture defines an abstract model that every concrete attestation
system (TPM, SGX, TDX, SEV-SNP, CCA, Nitro) can be mapped onto.

Source: [RFC 9334 — Remote ATtestation procedureS (RATS) Architecture](https://datatracker.ietf.org/doc/rfc9334/);
working group page: [datatracker.ietf.org/wg/rats/about/](https://datatracker.ietf.org/wg/rats/about/).

### 5.1 Roles

Five abstract roles, each defined in RFC 9334:

- **Attester** — "An entity (typically a device) whose Evidence must be
  appraised in order to infer the extent to which the Attester is considered
  trustworthy." Produces **Evidence**.
- **Verifier** — "An entity that appraises the validity of Evidence about
  an Attester and produces Attestation Results to be used by a Relying
  Party." Consumes Evidence + Reference Values + Endorsements; produces
  **Attestation Results**.
- **Relying Party** — "An entity that depends on the validity of information
  about an Attester for purposes of reliably applying application-specific
  actions." Consumes Attestation Results; makes authorisation decisions.
- **Endorser** — "An entity (typically a manufacturer) whose Endorsements
  may help Verifiers appraise the authenticity of Evidence." Issues vouchers
  (EK certificates, AMD VCEK certs, Intel PCK certs, etc.) about device
  capabilities.
- **Reference Value Provider** — "An entity (typically a manufacturer)
  whose Reference Values help Verifiers appraise Evidence." Publishes the
  golden measurements (firmware hashes, MR-enclave values, etc.).

### 5.2 Topologies — Passport vs Background-Check

- **Passport model**: Attester → Verifier → (Attestation Result back to
  Attester) → Relying Party. The Attester holds the Result like a passport
  and presents it to multiple RPs.
- **Background-Check model**: Attester → Relying Party → Verifier → (Result
  back to RP). The RP acts as the gateway; the Attester never holds a
  Result.

These are not exclusive — many real systems use one for one connection and
the other for another. SEV-SNP guests typically run a Background-Check
flow with the host's KMS as the RP; Azure Attestation acts as a Verifier
returning JWTs in a Passport flow.

### 5.3 Mapping vendor flows onto RATS

| Vendor | Attester | Evidence | Endorser | RVP |
|--------|----------|----------|----------|-----|
| **TPM 2.0** | Platform + TPM chip | `TPM2_Quote` + event log | TPM manufacturer (EK cert) | OEM / OS distro RIMs |
| **AMD SEV-SNP** | Guest in SNP VM | `MSG_REPORT_REQ` response → ATTESTATION_REPORT | AMD via VCEK + ASK + ARK chain | AMD (firmware), OS image vendor |
| **Intel SGX** | Enclave | `EREPORT` → Quote (DCAP) | Intel via PCK Cert from PCS | Intel for QE/PCE, ISV for MR-Enclave |
| **Intel TDX** | TD guest | TDREPORT → Quote | Intel via PCK + TCB info | Intel for TDX module, ISV for MRTD |
| **ARM CCA** | Realm | Realm Token + Platform Token | ARM via PSA-defined chain | ARM / OEM |
| **AWS Nitro** | Enclave | COSE_Sign1 attestation doc | AWS via Nitro PKI root | AWS (PCR0 default values) |

All six fit the RATS architecture cleanly because RATS was deliberately
designed to be vendor-neutral.

---

## 6. Entity Attestation Token — RFC 9711

**RFC 9711, "The Entity Attestation Token (EAT)"**, published as an IETF
Standards Track document in **April 2025**, defines a standard envelope for
attestation Evidence and Attestation Results, expressible as a
**CBOR Web Token (CWT)** or a **JSON Web Token (JWT)**.

Sources:
- [RFC 9711 — datatracker.ietf.org](https://datatracker.ietf.org/doc/rfc9711/)
- [RFC 9711 PDF — rfc-editor.org](https://www.rfc-editor.org/rfc/rfc9711.pdf)
- [RFC 9711 info page — rfc-editor.org](https://www.rfc-editor.org/info/rfc9711/)

(Note: the original task brief listed "Jan 2024" — the published Standards
Track document carries an April 2025 date. Earlier drafts in 2023–2024
predated the final RFC.)

### 6.1 Standard claims

EAT defines a set of attestation-oriented claims. The most load-bearing for
cross-vendor work:

| Claim | Type | Purpose |
|-------|------|---------|
| `eat_nonce` | bytes/array | Verifier-supplied freshness nonce; ≥ 64 bits entropy required |
| `ueid` | bytes | Universal Entity ID — globally unique per device (think serial number) |
| `sueids` | map | Semi-permanent UEIDs that can roll at lifecycle events |
| `oemid` | int/bytes/text | Three-format OEM identifier |
| `hwmodel` | bytes | Hardware model, unique within OEM |
| `hwversion` | array | Hardware version |
| `swname` | text | Software name (free-form) |
| `swversion` | array | Software version |
| `dbgstat` | enum | Debug status — `enabled`, `disabled`, `permanently-disabled`, `fully-disabled` |
| `oemboot` | bool | OEM-authorised boot confirmation (replaces older `secboot`) |
| `measurements` | array | Software measurements (typically CoSWID) |
| `measres` | array | Measurement results (success/fail/not-run/absent) |
| `manifests` | array | Software manifests (CoSWID/SWID/etc.) |
| `location` | map | Geolocation |
| `uptime` | int | Seconds since boot |
| `bootcount` | int | Boot count |
| `bootseed` | bytes | Public boot-session differentiator |
| `dloas` | array | Digital Letters of Approval (certifications) |
| `submods` | map | Submodules for composite entities |
| `iat` | int | Issued-At (integer, no floats) |
| `eat_profile` | URI/OID | Profile identifier |
| `intuse` | text/int | Intended use |

### 6.2 The profile mechanism

EAT is intentionally generic; a **profile** is a normative narrowing for a
specific use case. A profile specifies:

- Encoding choice (CWT/JWT)
- CBOR serialisation rules (definite-length, preferred)
- Cryptographic protection (`COSE_Sign1`, `COSE_Sign`, plus algorithm set)
- Key identification method (COSE `kid` / UEID / X.509)
- Freshness requirements (nonce mandate)
- Mandatory/prohibited/constrained claims

The normatively-defined **Constrained Device Standard Profile** (URI:
`urn:ietf:rfc:rfc9711`) mandates CBOR with definite-length preferred
serialisation, `COSE_Sign1`, ES256/384/512, and a unique nonce per request.

### 6.3 Vendor profiles aligning with EAT

- **ARM PSA Attestation Token** — formally an EAT profile
  ([draft-tschofenig-rats-psa-token](https://datatracker.ietf.org/doc/draft-tschofenig-rats-psa-token/),
  now consumed by ARM CCA).
- **Intel TDX** — has working drafts mapping TDX quote contents to EAT
  claims (see the
  [draft-ietf-rats-tdx-attestation](https://datatracker.ietf.org/doc/draft-ietf-rats-eat-tdx-evidence/)
  work item).
- **AMD SEV-SNP** — has been discussed for an EAT mapping but the canonical
  format remains the AMD-specific `ATTESTATION_REPORT` (§ AMD SEV-SNP doc).
- **AWS Nitro** — uses COSE_Sign1 with a custom claim set that is
  structurally EAT-shaped but is not a registered EAT profile.

---

## 7. TCG attestation reference model

Beyond the Library Spec, TCG publishes several attestation-related
documents that fill in the verifier side:

### 7.1 Reference Integrity Manifest (RIM)

The **TCG Reference Integrity Manifest** is the standardised container for
*known-good* measurements: "this firmware build expects these PCR values
after a clean boot." A RIM provider (OEM, OS distro, BMC vendor) publishes
RIMs; a verifier compares actual quoted PCRs against the RIM's expected
values.

Spec: [TCG Reference Integrity Manifest (RIM) Information Model](https://trustedcomputinggroup.org/resource/tcg-reference-integrity-manifest-rim-information-model/)
and the binding to the SWID/CoSWID format.

### 7.2 Canonical Event Log (CEL)

The **TCG Canonical Event Log Format** standardises the event log carried
alongside a quote. Standardised at the IETF as
[RFC 9393, "Concise Software Identification Tags"](https://datatracker.ietf.org/doc/rfc9393/),
and at TCG as
[TCG Canonical Event Log Format](https://trustedcomputinggroup.org/resource/canonical-event-log-format/).

Each event encodes which PCR/RTMR was extended, with what digest, and what
the event was (firmware blob, kernel measurement, UEFI variable change, etc.).
The verifier replays the log to confirm that the final PCR/RTMR digest in the
quote matches.

### 7.3 Trusted Attestation Protocol (TAP)

The TCG **Trusted Attestation Protocol** defines the wire protocol for
interacting with an attestation service: how to request a nonce, send the
quote + event log, receive a verdict. Spec:
[TCG TAP Information Model](https://trustedcomputinggroup.org/resource/tcg-tap-information-model/).

---

## 8. CCEL — Confidential Computing Event Log

The **CCEL (Confidential Computing Event Log)** is an **ACPI table** that
carries an event log for confidential VMs — TDX in production today, with
SEV-SNP and future CC architectures expected to converge on the same format.
It generalises the TPM event log to settings where there is no TPM, only
**RTMRs** (Runtime Measurement Registers, in TDX).

A guest OS discovers the CCEL ACPI table at runtime — under Linux the
canonical paths are `/sys/firmware/acpi/tables/ccel` (the ACPI header) and
`/sys/firmware/acpi/tables/data/ccel` (the event log payload). It reads the
event log out of the indicated memory range and presents it alongside the
TDX Quote (or SNP attestation report) for the verifier to replay.

Sources:
- [td-shim spec — confidential-containers/td-shim](https://github.com/confidential-containers/td-shim/blob/main/doc/tdshim_spec.md)
  — "The TD-Shim shall report the TD event log via 'CCEL' ACPI table defined
  in the Guest Hypervisor communication interface."
- [google/go-eventlog `ccel` package](https://pkg.go.dev/github.com/google/go-eventlog/ccel)
  — "Package ccel implements event log parsing and replay for the
  Confidential Computing event log. It only supports the CCEL based on the
  TCG crypto-agile event log (including the 'Spec ID Event03' signature)."
- [Intel Runtime Integrity Measurement and Attestation in a Trust Domain](https://www.intel.com/content/www/us/en/developer/articles/community/runtime-integrity-measure-and-attest-trust-domain.html)
- [pytdxmeasure on PyPI](https://pypi.org/project/pytdxmeasure/) — Python
  reference implementation of CCEL parsing and RTMR replay.
- UEFI 2.10 §38 defines `EFI_CC_MEASUREMENT_PROTOCOL`.

Key property: the **event log format** in CCEL is the **TCG crypto-agile
event log format** (`EFI_TCG2_PCR_EVENT2` entries with SHA-384 digests for
TDX), so existing TPM tooling that knows how to replay a TCG event log can
be reused for CC environments. The only change is that PCR indexes are
replaced with **RTMR** indexes. TDX exposes **four RTMRs** (`RTMR[0..3]`),
extended via `EFI_CC_MEASUREMENT_PROTOCOL` with the conventional usage:

- **RTMR[0]** — virtual firmware (TDVF / OVMF)
- **RTMR[1]** — OS loader (shim, GRUB)
- **RTMR[2]** — kernel + initrd
- **RTMR[3]** — application / runtime

This RTMR allocation mirrors the SRTM PCR 0/4/8/9 split on a TPM-attested PC.

---

## 9. Related standards — DICE, SPDM, DPE

### 9.1 DICE (Device Identifier Composition Engine)

TCG **DICE** is a layered identity model for devices too constrained for a
full TPM (microcontrollers, SoCs). The core idea:

1. A device has a **UDS (Unique Device Secret)** burned in at manufacture.
2. The first stage code computes
   `CDI₀ = KDF(UDS, H(first-stage-code))` — the **Compound Device Identifier**.
3. Each subsequent layer derives `CDIₙ₊₁ = KDF(CDIₙ, H(next-stage-code))`,
   producing a chain of measurement-bound secrets.
4. Each layer can derive a signing key from its CDI and publish a
   certificate chain; the verifier walks the chain back to the UDS-anchored
   root.

The CDI is therefore *implicitly* a measurement of every preceding layer.
Spec: [TCG DICE Layering Architecture](https://trustedcomputinggroup.org/resource/dice-layering-architecture/)
and [Hardware Requirements for a Device Identifier Composition Engine](https://trustedcomputinggroup.org/resource/hardware-requirements-for-a-device-identifier-composition-engine/).

### 9.2 DPE (DICE Protection Environment)

**DICE Protection Environment** is a higher-level, software-agnostic API
specification on top of DICE: a uniform interface for clients to derive
keys/certificates from a CDI without knowing the underlying hardware.
Caliptra (the OCP open-source root-of-trust) implements DPE on top of DICE.

Spec: [TCG DICE Protection Environment](https://trustedcomputinggroup.org/resource/dice-protection-environment-specification/).

### 9.3 SPDM (Security Protocol and Data Model)

**SPDM** is the **DMTF**-standardised protocol for device authentication and
firmware attestation over PCIe, USB, MCTP, I²C, etc. It is the canonical
device-to-device attestation protocol on the platform fabric. Spec:
[DMTF DSP0274 — Security Protocol and Data Model (SPDM) Specification](https://www.dmtf.org/standards/spdm).

SPDM defines (high level):
- **GET_VERSION / GET_CAPABILITIES / NEGOTIATE_ALGORITHMS** handshake.
- **GET_DIGESTS / GET_CERTIFICATE** to fetch device certificate chain.
- **CHALLENGE / CHALLENGE_AUTH** to prove possession of device cert key.
- **GET_MEASUREMENTS** to fetch device measurements (firmware hashes, etc.).
- **KEY_EXCHANGE / FINISH** to establish a secure session (Secured Messages).

**NVIDIA H100 Confidential Compute** uses SPDM as its in-band attestation
channel. The
[NVIDIA H100 CC blog post](https://developer.nvidia.com/blog/confidential-computing-on-h100-gpus-for-secure-and-trustworthy-ai/)
states:

> "The GPU PF driver uses SPDM for session establishment and the attestation
> report. ... The device-unique, private identity key is burned into the
> fuses of each H100 GPU. ... Verification of this certificate against the
> NVIDIA Certificate Authority will verify that the device was manufactured
> by NVIDIA."

Each H100 carries a device-unique **ECC-384 keypair** burned into fuses;
the GPU runs measured boot internally, exposes measurements over SPDM to
the CPU-side driver (which itself runs inside the CPU TEE — TDX or
SEV-SNP), and the driver forwards the attestation report to the **NVIDIA
Remote Attestation Service (NRAS)** which acts as the Verifier per the
RATS architecture. Revocation is checked via NVIDIA OCSP. NRAS issues an
Attestation Result that is bundled with the CPU TEE's attestation for the
relying party.

This is the canonical **split-attester** pattern — two Attesters (CPU TEE
and GPU) each producing Evidence, a composite Verifier producing one fused
Attestation Result. Designs that incorporate accelerator TEEs must treat
SPDM measurement freshness, transport binding (the GPU↔driver session),
and certificate chains as separate concerns from the CPU TEE attestation.

### 9.4 How they interact

DICE → device identity. SPDM → device-to-device attestation transport.
RATS → architectural roles. EAT → claim envelope. RIM/CEL → reference values
+ event log. TPM 2.0 → the canonical platform-firmware RoT for x86 PCs and
servers. Each layer is largely orthogonal; vendors pick a mix.

---

## 10. Cross-vendor mapping

| Standard / vendor | TPM 2.0 chip | RATS roles | EAT envelope | TCG event log / CEL | DICE | SPDM | Notes |
|---|---|---|---|---|---|---|---|
| **TCG TPM 2.0 (PC client)** | Yes (host TPM) | Native | Optional (via profile) | Yes (TCG EventLog) | Sibling | No | Canonical baseline |
| **AMD SEV-SNP** | No (no host TPM involved in guest attestation) | Yes — VCEK = Endorser, AMD = RVP | Not native; report has its own format | No native log (driver / OVMF / kernel add CCEL) | Sibling | No | `ATTESTATION_REPORT` is an AMD-defined structure |
| **Intel SGX** | No | Yes — Intel = Endorser + RVP | Not native (DCAP Quote format) | No | No | No | DCAP quotes carry MR-Enclave / MR-Signer |
| **Intel TDX** | No (per-TD) | Yes | Draft EAT mapping (`draft-ietf-rats-eat-tdx-evidence`) | Yes via CCEL | No | No | RTMRs replace PCRs; CCEL replaces TCG eventlog |
| **ARM CCA / PSA** | No | Yes | Yes — PSA token is the canonical EAT profile | n/a | Often | No | Most "EAT-native" vendor flow |
| **NVIDIA H100 CC** | No (GPU has its own RoT) | Yes — NRAS = Verifier | Quote uses NVIDIA-specific schema; NRAS-issued result can be JWT/EAT-shaped | No | Yes (GPU RoT uses DICE) | Yes (SPDM measurements over PCIe) | Combined with host TEE attestation in practice |
| **AWS Nitro Enclaves** | No | Yes — AWS = Endorser, customer = RVP | COSE_Sign1 + custom claims; not registered EAT profile | n/a (uses 16 PCR-like SHA-384 registers) | No | No | PCR0/1/2 = image/kernel/app; PCR3 = parent IAM; PCR4 = parent instance ID; PCR8 = signing cert |

(Cross-references: see each vendor's dedicated research doc for the exact
field-level mapping.)

### AWS Nitro Enclaves PCR semantics (verified)

Nitro reuses TPM-style PCR vocabulary but with vendor-defined contents. Per
[AWS docs — Cryptographic attestation](https://docs.aws.amazon.com/enclaves/latest/user/set-up-attestation.html):

| PCR | Contents |
|-----|----------|
| PCR0 | SHA-384 of the enclave image file (without section data) — built by `nitro-cli build-enclave` |
| PCR1 | SHA-384 of the Linux kernel + boot ramfs |
| PCR2 | SHA-384 of the application (user payload, no boot ramfs) |
| PCR3 | SHA-384 of the IAM role ARN assigned to the parent EC2 instance (48-null-byte prefix) |
| PCR4 | SHA-384 of the parent instance ID (48-null-byte prefix) |
| PCR8 | SHA-384 of the signing certificate for the signed enclave image file |

Notable: PCR3/PCR4 bind the enclave's attestation to the parent host's
identity (IAM role, instance ID) — this is *not* the TPM model, which has
no notion of "this measurement comes from the parent system's IAM
identity." Designs that quote-then-verify Nitro attestations must apply
parent-instance policy on PCR3/PCR4 if they want to defend against
relocation/replay of a signed `.eif` to a different IAM context.

Debug-mode enclaves emit attestation documents with all-zero PCRs and
MUST NOT be accepted by production verifiers.

### Notable cross-vendor observations

- **TPM 2.0 is the only ecosystem with a real chip on the host**. Every
  TEE vendor builds a *virtual* / chip-internal attestation system that
  reuses TCG's measurement and event-log primitives but replaces the
  chip's role with on-die or in-firmware logic.
- **EAT is becoming the lingua franca only at the Attestation Result layer**
  (verifier output), not at the Evidence layer. Each vendor's hardware
  evidence format remains vendor-specific; verifiers translate into EAT
  for relying parties.
- **CCEL has unified the event log story for confidential VMs**. The same
  Linux `securityfs` / `tpm` tooling that replays a TPM event log works on
  TDX and SEV-SNP via CCEL.
- **SPDM is the only widely-deployed accelerator attestation transport**
  and is rapidly becoming required for any non-CPU TEE that participates
  in a confidential workload (GPUs, DPUs, NICs).

---

## Load-bearing claims

The `tee-specialist` agent should ground cross-vendor TEE designs on the
following facts, all sourced above:

1. **TPM 2.0 hierarchies are non-fungible.** The EK lives in the Endorsement
   Hierarchy (vendor-rooted, privacy-sensitive); the SRK lives in the
   Storage Hierarchy (owner-rooted); they cannot be substituted for each
   other. Designs must explicitly say which hierarchy a key is in.

2. **PCRs 0–15 reset only on platform reset.** PCRs 17–22 reset on DRTM
   late-launch (Locality 4). PCR 16 and 23 are resettable and *should never
   appear in a trust decision*. Designs that quote PCR 16 or 23 are bugs.

3. **`TPM2_Quote` returns `TPMS_ATTEST`, not raw PCR values.** The signature
   covers `magic(=0xFF544347) || type || qualifiedSigner || extraData(nonce) ||
   clockInfo || firmwareVersion || pcrSelect || pcrDigest`. Replaying the
   event log against `pcrDigest` is what binds quoted state to observed
   firmware/OS — verifiers MUST verify both signature and replay.

4. **AK certification — not direct EK signing — is how TPM quotes earn
   trust.** Direct EK signing of arbitrary data is prohibited by the EK
   template. Verifier code must walk: AK cert → (privacy CA or)
   `TPM2_ActivateCredential` proof → EK cert → manufacturer CA.

5. **RFC 9334 (RATS) is the cross-vendor architecture vocabulary.** When
   coordinating across SGX, TDX, SEV-SNP, CCA, Nitro, GPU TEEs, use
   Attester / Verifier / Relying Party / Endorser / Reference Value
   Provider role names; do not invent project-local terms.

6. **RFC 9711 (EAT) is the standard claim envelope, not the standard
   Evidence format.** Vendor Evidence formats (TDX Quote, SNP report,
   SGX DCAP quote) remain heterogeneous; EAT enters the system as the
   verifier's *output* (Attestation Result) or as a profile-defined
   Evidence shape (ARM PSA / CCA).

7. **CCEL is the TPM event log for confidential VMs.** TDX (RTMRs) and
   SEV-SNP (launch + supplemental measurements) both rely on CCEL ACPI
   tables carrying TCG-formatted event log entries. Tooling that replays
   a TPM event log works for CC VMs with the PCR→RTMR substitution.

8. **DICE provides identity *without* a TPM chip.** If a deployment cannot
   ship a TPM (constrained device, custom silicon, in-package RoT), DICE
   + DPE is the TCG-blessed substitute and is what Caliptra and most GPU
   / DPU RoTs use.

9. **SPDM is the device-fabric attestation protocol.** Any accelerator
   participating in a confidential workload (NVIDIA H100 CC, future DPUs)
   exposes attestation over SPDM; the host or guest collects SPDM
   measurements and forwards them to a Verifier alongside the CPU TEE
   evidence. Split-attester designs MUST account for SPDM measurement
   freshness, transport binding, and certificate chains separately.

10. **Provider owns the format; verifier owns the policy.** The vendor
    defines what Evidence looks like (and which Endorsers / RVPs exist);
    the verifier defines which Reference Values count as "good." Designs
    must keep these layers separated: hard-coding vendor-specific Evidence
    parsing into relying-party policy makes cross-vendor portability
    impossible. The RATS roles exist precisely to enforce this separation.
