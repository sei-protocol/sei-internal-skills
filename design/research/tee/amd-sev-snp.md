# AMD SEV-SNP Attestation — Empirical Reference

**Status:** Design-research reference for the Tide `tee-specialist` agent and future on-chain SEV-SNP verifier implementations.

**Last updated:** 2026-06-01

**Scope.** This document captures byte-level layout, signing-chain hierarchy, generational drift, measurement methodology, and on-chain verification cost for AMD SEV-SNP attestation. Every load-bearing claim cites its primary source. Verbatim spec excerpts are favored over paraphrase wherever possible.

---

## 1. `ATTESTATION_REPORT` struct — byte-by-byte

The authoritative source is the **AMD SEV Secure Nested Paging Firmware ABI Specification**, AMD publication number **56860** ("SEV Secure Nested Paging Firmware ABI Specification"). The struct is defined in **Table 23** ("Attestation Report Format") of the spec — the table number was 22 in early revisions, renumbered to 23 around rev 1.53. The most recent public revision is **rev 1.57 (January 2025)**.

Primary URLs:
- Current: [https://www.amd.com/content/dam/amd/en/documents/developer/56860.pdf](https://www.amd.com/content/dam/amd/en/documents/developer/56860.pdf)
- EPYC docs path (alt): [https://www.amd.com/content/dam/amd/en/documents/epyc-technical-docs/specifications/56860.pdf](https://www.amd.com/content/dam/amd/en/documents/epyc-technical-docs/specifications/56860.pdf)
- HTML index: [https://docs.amd.com/v/u/en-US/56860](https://docs.amd.com/v/u/en-US/56860)
- Rev 1.54 mirror: [http://kib.kiev.ua/x86docs/AMD/SEV/56860-r1.54.pdf](http://kib.kiev.ua/x86docs/AMD/SEV/56860-r1.54.pdf)
- CoRIM profile (offsets confirmed against draft): [draft-deeglaze-amd-sev-snp-corim-profile-02](https://www.ietf.org/archive/id/draft-deeglaze-amd-sev-snp-corim-profile-02.html)

The companion document covering KDS and VCEK certificates is **AMD publication 57230** ("Versioned Chip Endorsement Key (VCEK) Certificate and KDS Interface Specification") — [https://www.amd.com/content/dam/amd/en/documents/epyc-technical-docs/specifications/57230.pdf](https://www.amd.com/content/dam/amd/en/documents/epyc-technical-docs/specifications/57230.pdf).

Cross-checked against the canonical reference implementations:
- AMDESE/sev-guest C header `include/attestation.h` ([source](https://github.com/AMDESE/sev-guest/blob/main/include/attestation.h)) — quoted verbatim in §1.6 below.
- VirTEE/sev Rust crate `src/firmware/guest/types/snp.rs` ([source](https://github.com/virtee/sev/blob/main/src/firmware/guest/types/snp.rs)).

### 1.1 Layout table

The body of the report is **0x000–0x29F (672 bytes)**; the trailing signature occupies **0x2A0–0x49F (512 bytes)**, yielding a **total report size of 0x4A0 = 1184 bytes**. This matches the `<report-size>` of `0x4A0` reported by the `MSG_REPORT_RSP` wrapper (Table 24 in 56860).

| Offset | Size | Field | Notes |
|--------|-----:|-------|-------|
| `0x000` | 4 | `VERSION` | Report version. `2` (Milan, early Genoa), `3` (Genoa+ adds `CPUID_FAM/MOD/STEP`), `5` (Turin adds mitigation vectors). |
| `0x004` | 4 | `GUEST_SVN` | Guest security version number, supplied at launch. |
| `0x008` | 8 | `POLICY` | Guest launch policy bitfield (see §1.2). |
| `0x010` | 16 | `FAMILY_ID` | Guest-chosen 128-bit family identifier. |
| `0x020` | 16 | `IMAGE_ID` | Guest-chosen 128-bit image identifier. |
| `0x030` | 4 | `VMPL` | Virtual Machine Privilege Level that requested the report (0–3). |
| `0x034` | 4 | `SIGNATURE_ALGO` | `1 = ECDSA P-384 with SHA-384` (only value defined today). |
| `0x038` | 8 | `CURRENT_TCB` | `TCB_VERSION` struct — TCB at time of report (see §1.3). |
| `0x040` | 8 | `PLATFORM_INFO` | Host platform info bitfield (see §1.4). |
| `0x048` | 4 | `KEY_INFO` / `AUTHOR_KEY_EN`-`MASK_CHIP_KEY`-`SIGNING_KEY` | Bit 0 = `AUTHOR_KEY_EN`; bit 1 = `MASK_CHIP_KEY`; bits 2-4 = `SIGNING_KEY` (0=VCEK, 1=VLEK, 7=NONE). |
| `0x04C` | 4 | _Reserved_ | Must be zero. |
| `0x050` | 64 | `REPORT_DATA` | Guest-supplied nonce / binding data. Verifier uses this to bind the report to a fresh challenge. |
| `0x090` | 48 | `MEASUREMENT` | Launch measurement (SHA-384 over guest pages + initial state, computed by AMD-SP — see §4). |
| `0x0C0` | 32 | `HOST_DATA` | Host-supplied at launch (hypervisor-provided, attested but not measured). |
| `0x0E0` | 48 | `ID_KEY_DIGEST` | SHA-384 of the ID block ECDSA-P384 public key (if used). |
| `0x110` | 48 | `AUTHOR_KEY_DIGEST` | SHA-384 of the author key (present iff `AUTHOR_KEY_EN`=1). |
| `0x140` | 32 | `REPORT_ID` | Unique per-report ID (firmware-generated). |
| `0x160` | 32 | `REPORT_ID_MA` | Migration-agent report ID; all-FF if no MA. |
| `0x180` | 8 | `REPORTED_TCB` | `TCB_VERSION` at the time the signing key was derived. **This is the TCB used in VCEK URL lookups.** |
| `0x188` | 1 | `CPUID_FAM_ID` | (Version ≥ 3) CPUID family ID. |
| `0x189` | 1 | `CPUID_MOD_ID` | (Version ≥ 3) CPUID model. |
| `0x18A` | 1 | `CPUID_STEP` | (Version ≥ 3) CPUID stepping. |
| `0x18B` | 21 | _Reserved_ | |
| `0x1A0` | 64 | `CHIP_ID` | Per-chip identifier (zeroed if `MASK_CHIP_KEY`=1). Used as `hwid` in KDS VCEK URL. |
| `0x1E0` | 8 | `COMMITTED_TCB` | `TCB_VERSION` committed (will not roll back below this). |
| `0x1E8` | 1 | `CURRENT_BUILD` | Build number of current firmware. |
| `0x1E9` | 1 | `CURRENT_MINOR` | Minor version. |
| `0x1EA` | 1 | `CURRENT_MAJOR` | Major version. |
| `0x1EB` | 1 | _Reserved_ | |
| `0x1EC` | 1 | `COMMITTED_BUILD` | Build number of committed firmware. |
| `0x1ED` | 1 | `COMMITTED_MINOR` | |
| `0x1EE` | 1 | `COMMITTED_MAJOR` | |
| `0x1EF` | 1 | _Reserved_ | |
| `0x1F0` | 8 | `LAUNCH_TCB` | `TCB_VERSION` at VM launch. |
| `0x1F8` | 168 | _Reserved_ | Zero (V2/V3); used for `LAUNCH_MIT_VECTOR` / `CURRENT_MIT_VECTOR` in V5 (Turin). |
| `0x2A0` | 512 | `SIGNATURE` | ECDSA P-384 signature struct (see §1.5). |

Offsets and sizes verified against the CoRIM draft, which mirrors AMD spec Table 23 ([draft-deeglaze, "Core Attestation Report Fields"](https://www.ietf.org/archive/id/draft-deeglaze-amd-sev-snp-corim-profile-02.html)).

### 1.2 `POLICY` field (offset `0x008`, 8 bytes)

Bit layout per [virtee/sev `GuestPolicy`](https://github.com/virtee/sev/blob/main/src/firmware/guest/types/snp.rs):

| Bits | Name | Meaning |
|------|------|---------|
| 0–7 | `ABI_MINOR` | Minimum SEV-SNP ABI minor version the guest requires. |
| 8–15 | `ABI_MAJOR` | Minimum SEV-SNP ABI major version. |
| 16 | `SMT_ALLOWED` | Guest permits running on SMT-enabled host. **Must be 1 in current firmware.** |
| 17 | _Reserved_ | Must be 1. |
| 18 | `MIGRATE_MA_ALLOWED` | Guest permits live migration via a migration agent. |
| 19 | `DEBUG_ALLOWED` | Guest permits debug access. **Production guests must have this 0.** |
| 20 | `SINGLE_SOCKET_REQUIRED` | Guest restricted to a single socket. |
| 21 | `CXL_ALLOWED` | (Genoa+) Guest permits CXL devices. |
| 22 | `MEM_AES_256_XTS` | (Genoa+) Memory encryption uses AES-256-XTS instead of AES-128. |
| 23 | `RAPL_DIS` | (Genoa+) Guest requires RAPL be disabled (side-channel hardening). |
| 24 | `CIPHERTEXT_HIDING` | (Genoa+) Guest requires ciphertext-hiding DRAM mode. |
| 25 | `PAGE_SWAP_DISABLED` | (Turin) Guest requires page-swap disabled. |
| 26–63 | _Reserved_ | |

### 1.3 `TCB_VERSION` struct (8 bytes)

Used by `CURRENT_TCB`, `REPORTED_TCB`, `COMMITTED_TCB`, `LAUNCH_TCB`. Per the [Contrast docs](https://docs.edgeless.systems/contrast/architecture/attestation/amd-details) and 56860 Table 5:

| Byte | Field | Meaning |
|------|-------|---------|
| 0 | `BOOT_LOADER` | AMD-SP bootloader SVN |
| 1 | `TEE` | AMD-SP OS (TEE) SVN |
| 2–5 | _Reserved_ | Zero |
| 6 | `SNP` | SNP firmware SVN |
| 7 | `MICROCODE` | x86 microcode patch level |

**Critical for verifiers:** the VCEK is derived from `CHIP_ID || REPORTED_TCB`. A verifier MUST fetch the VCEK certificate for the exact `REPORTED_TCB` value carried in the report, not a cached cert for a different TCB.

### 1.4 `PLATFORM_INFO` field (offset `0x040`, 8 bytes)

| Bit | Name | Meaning |
|-----|------|---------|
| 0 | `SMT_ENABLED` | Hyperthreading enabled on host. |
| 1 | `TSME_ENABLED` | Transparent SME enabled. |
| 2 | `ECC_ENABLED` | DRAM ECC enabled. |
| 3 | `RAPL_DISABLED` | RAPL disabled (mitigates power-side-channel). |
| 4 | `CIPHERTEXT_HIDING_ENABLED` | Ciphertext hiding active in DRAM. |
| 5 | `ALIAS_CHECK_COMPLETE` | Memory-alias check done at boot. |
| 6 | _Reserved_ | |
| 7 | `TIO_ENABLED` | Trusted I/O active. |
| 8–63 | _Reserved_ | |

Source: [virtee/sev `PlatformInfo`](https://github.com/virtee/sev/blob/main/src/firmware/guest/types/snp.rs); confirmed by [Contrast docs Table 24](https://docs.edgeless.systems/contrast/architecture/attestation/amd-details).

### 1.5 `SIGNATURE` struct (offset `0x2A0`, 512 bytes)

```c
struct signature {
    uint8_t  r[72];     // big-endian P-384 r component
    uint8_t  s[72];     // big-endian P-384 s component
    uint8_t  reserved[368];
};
```

Source: AMDESE/sev-guest `attestation.h` ("Signature Structure: Contains two 72-byte fields (r and s values for ECDSA-P384) plus 512 bytes of reserved space"). Reserved bytes are zero in v2/v3 reports and may be used in future versions.

**Signing input:** the firmware signs `report[0x000:0x2A0]` — i.e., the first 672 bytes (everything up to the signature field itself). This is critical for verifier implementations: hash the first 672 bytes (not the trailing signature region) with SHA-384, then ECDSA-P384 verify against VCEK/VLEK public key.

### 1.6 Verbatim AMD C reference header

The following is reproduced verbatim from [AMDESE/sev-guest `include/attestation.h`](https://github.com/AMDESE/sev-guest/blob/main/include/attestation.h) (Copyright AMD 2021). This is the canonical "ground truth" C layout for v2 reports; the VirTEE Rust types extend it to v3/v5.

```c
/* Copyright (C) 2021 Advanced Micro Devices, Inc. */

#ifndef ATTESTATION_H
#define ATTESTATION_H

#include <stdint.h>

#define POLICY_DEBUG_SHIFT        19
#define POLICY_MIGRATE_MA_SHIFT   18
#define POLICY_SMT_SHIFT          16
#define POLICY_ABI_MAJOR_SHIFT     8
#define POLICY_ABI_MINOR_SHIFT     0

#define POLICY_DEBUG_MASK         (1UL << (POLICY_DEBUG_SHIFT))
#define POLICY_MIGRATE_MA_MASK    (1UL << (POLICY_MIGRATE_MA_SHIFT))
#define POLICY_SMT_MASK           (1UL << (POLICY_SMT_SHIFT))
#define POLICY_ABI_MAJOR_MASK     (0xFFUL << (POLICY_ABI_MAJOR_SHIFT))
#define POLICY_ABI_MINOR_MASK     (0xFFUL << (POLICY_ABI_MINOR_SHIFT))

#define SIG_ALGO_ECDSA_P384_SHA384  0x1

#define PLATFORM_INFO_SMT_EN_SHIFT 0
#define PLATFORM_INFO_SMT_EN_MASK  (1UL << (PLATFORM_INFO_SMT_EN_SHIFT))

#define AUTHOR_KEY_EN_SHIFT 0
#define AUTHOR_KEY_EN_MASK  (1UL << (AUTHOR_KEY_EN_SHIFT))

union tcb_version {
    struct {
        uint8_t boot_loader;
        uint8_t tee;
        uint8_t reserved[4];
        uint8_t snp;
        uint8_t microcode;
    };
    uint64_t raw;
};

struct signature {
    uint8_t r[72];
    uint8_t s[72];
    uint8_t reserved[512-144];
};

struct attestation_report {
    uint32_t          version;          /* 0x000 */
    uint32_t          guest_svn;        /* 0x004 */
    uint64_t          policy;           /* 0x008 */
    uint8_t           family_id[16];    /* 0x010 */
    uint8_t           image_id[16];     /* 0x020 */
    uint32_t          vmpl;             /* 0x030 */
    uint32_t          signature_algo;   /* 0x034 */
    union tcb_version platform_version; /* 0x038 */
    uint64_t          platform_info;    /* 0x040 */
    uint32_t          flags;            /* 0x048 */
    uint32_t          reserved0;        /* 0x04C */
    uint8_t           report_data[64];  /* 0x050 */
    uint8_t           measurement[48];  /* 0x090 */
    uint8_t           host_data[32];    /* 0x0C0 */
    uint8_t           id_key_digest[48];/* 0x0E0 */
    uint8_t           author_key_digest[48]; /* 0x110 */
    uint8_t           report_id[32];    /* 0x140 */
    uint8_t           report_id_ma[32]; /* 0x160 */
    union tcb_version reported_tcb;     /* 0x180 */
    uint8_t           reserved1[24];    /* 0x188 */
    uint8_t           chip_id[64];      /* 0x1A0 */
    uint8_t           reserved2[192];   /* 0x1E0 */
    struct signature  signature;        /* 0x2A0 */
};

struct msg_report_resp {
    uint32_t status;
    uint32_t report_size;
    uint8_t  reserved[0x20-0x8];
    struct attestation_report report;
};

#endif  /* ATTESTATION_H */
```

Note the v2-only fields: the `reserved1[24]` block at offset `0x188` is where v3 reports place `cpuid_fam_id`, `cpuid_mod_id`, `cpuid_step` (one byte each), and the `reserved2[192]` block at `0x1E0` is where v3+ reports place `COMMITTED_TCB`, build/minor/major numbers, and `LAUNCH_TCB`. V5 reports (Turin) additionally use this region for mitigation vectors.

---

## 2. Signing chain

Hierarchy: **ARK → ASK → VCEK (or VLEK) → AttestationReport**.

Per [Contrast docs](https://docs.edgeless.systems/contrast/architecture/attestation/amd-details): "AMD Root CA --> AMD SEV CA --> VCEK -- signs --> Report".

### 2.1 ARK — AMD Root Key

- **Role:** Root of trust for all AMD CPU attestation. Each CPU generation has its own ARK (Milan ARK ≠ Genoa ARK ≠ Turin ARK).
- **Provisioning:** Burned at fab; private key held by AMD HSM in their fab.
- **Cert format:** X.509, self-signed.
- **Cryptography:** RSA-4096 with SHA-384 in current generations (per AMD KDS public certs).
- **Fetch URL (Milan example):** `https://kdsintf.amd.com/vcek/v1/Milan/cert_chain` returns ASK+ARK concatenated.

### 2.2 ASK — AMD SEV Key (a.k.a. AMD SEV CA)

- **Role:** Intermediate per-generation signing key. Signs every VCEK on a given CPU generation.
- **Cert format:** X.509, signed by ARK. Subject CN includes generation name (`SEV-Milan`, `SEV-Genoa`, etc.).
- **Cryptography:** RSA-4096 / SHA-384.
- **Fetch URL:** Same `cert_chain` endpoint as ARK.

### 2.3 VCEK — Versioned Chip Endorsement Key (default since Milan)

- **Role:** Per-chip, per-TCB-version endorsement key that signs attestation reports.
- **Derivation:** `VCEK = HKDF(chip_secret, CHIP_ID || TCB_VERSION)` — deterministic from chip + TCB inputs. Re-derived by AMD-SP each boot. Specified in AMD doc 57230 ("Versioned Chip Endorsement Key (VCEK) Certificate and KDS Interface Specification").
- **Cert format:** X.509 v3 DER, signed by ASK. Custom AMD OIDs in extensions encode the TCB the cert binds to:
  - `1.3.6.1.4.1.3704.1.3.2` — `blSPL` (Bootloader SPL / SVN), INTEGER
  - `1.3.6.1.4.1.3704.1.3.3` — `teeSPL` (TEE SPL / SVN), INTEGER
  - `1.3.6.1.4.1.3704.1.3.5` — `snpSPL` (SNP firmware SPL / SVN), INTEGER
  - `1.3.6.1.4.1.3704.1.4` — `ucodeSPL` (microcode SVN), INTEGER (and `HWID` carrier on some generations — OID space evolved)
  - `1.3.6.1.4.1.3704.1.5` — `productName` (e.g., `Milan-B0`, `Genoa-B1`, `Turin-B0`)

  OID list sourced from AMD doc 57230 ("Versioned Chip Endorsement Key (VCEK) Certificate and KDS Interface Specification") and confirmed by the [IETF RATS AMD KDS reference values wiki](https://wiki.ietf.org/group/rats/referencevalues/amd-key-distribution-service). These OIDs match the byte values inside `REPORTED_TCB` and `CHIP_ID` in the attestation report — verifier checks consistency.
- **Cryptography:** ECDSA P-384 / SHA-384.
- **Fetch URL:** `https://kdsintf.amd.com/vcek/v1/{product_name}/{hwid}?blSPL={bl}&teeSPL={tee}&snpSPL={snp}&ucodeSPL={ucode}`. Per AMD 57230: "GET vcek/v1/{product_name}/{hwID}?{parameters}" returns the leaf VCEK Certificate corresponding to the TCB with the specified SPL values; unspecified SPLs default to zero. `{hwid}` is the lowercase hex `CHIP_ID`. **`hwid` length varies by generation:** 128 hex chars (64 bytes) for Family 19h (Milan, Genoa, Bergamo); 16 hex chars (8 bytes) for Turin and later — the Turin attestation report still carries 64 bytes in `CHIP_ID`, but only the first 8 are the canonical chip identifier (the remainder is zero). Live example URL (from a Milan EPYC):

  ```
  https://kdsintf.amd.com/vcek/v1/Milan/8ba826b2dd6ab65e401e0c4d4128ef4b434ed0ccb213f66c5f577b518730ef58
  92f78a78be259976973125a3b9b3d19f286c912cf5776fdfcee5260fa4576c4b
  ?blSPL=00&teeSPL=00&snpSPL=03&ucodeSPL=29
  ```

  Transport is **HTTPS / TLS 1.2** per the KDS spec. Response is the DER-encoded VCEK cert.

### 2.4 VLEK — Versioned Loaded Endorsement Key (alternative; added in firmware 1.55+)

- **Role:** Alternative to VCEK. Loaded into firmware by an authorized CSP rather than derived from chip secrets. Allows CSPs (e.g., Azure, GCP) to anonymize attestation across a fleet — verifier sees CSP identity rather than per-chip identity.
- **Provenance:** CSP requests VLEKs from AMD's KDS; AMD signs CSP-specific VLEK certs and the CSP loads them onto SEV firmware via `SNP_VLEK_LOAD`.
- **Cert chain root:** AMD ASVK (AMD SEV VLEK Key) → VLEK. ASVK is a separate intermediate from ASK.
- **Cert SAN encoding:** Contains `CSP_ID` (UTF-8 string) instead of per-chip `hwid` in `1.3.6.1.4.1.3704.1.4`.
- **`KEY_INFO` selector:** `SIGNING_KEY = 1` indicates VLEK; `SIGNING_KEY = 0` indicates VCEK.

Per the [CoRIM draft](https://www.ietf.org/archive/id/draft-deeglaze-amd-sev-snp-corim-profile-02.html): "VLEK (Version Loaded Endorsement Key): an alternative SEV-SNP Attestation Report signing key shared between AMD and CSP."

### 2.5 Revocation

- **CRL endpoint:** `https://kdsintf.amd.com/vcek/v1/{product_name}/crl` — AMD publishes a CRL per generation.
- **Cert validity:** VCEK certs are perpetual but become "stale" when TCB rolls forward. The verifier MUST refuse a report whose `REPORTED_TCB` is lower than its expected minimum TCB (anti-rollback policy).
- **Practical risk:** AMD rarely revokes ARK/ASK; the principal mechanism is forcing minimum-TCB enforcement, which obsoletes older VCEK certs by policy.

---

## 3. Generation differences

| Generation | Codename | EPYC line | Year | Key facts |
|-----------|---------|-----------|------|-----------|
| Zen 3 | **Milan** | EPYC 7003 | 2021 | First SEV-SNP generation. ABI spec v1.50–1.52. VCEK only (no VLEK). Attestation report `VERSION=2`. |
| Zen 4 | **Genoa** | EPYC 9004 | 2022 | ABI spec v1.53–1.55. Adds `CXL_ALLOWED`, `MEM_AES_256_XTS`, `RAPL_DIS`, `CIPHERTEXT_HIDING` policy bits. Attestation report `VERSION=3` adds `CPUID_FAM/MOD/STEP`. VLEK support introduced in firmware ≥ 1.55. |
| Zen 4c | **Bergamo** | EPYC 9704 | 2023 | Same SEV-SNP capabilities as Genoa; same ABI spec line. |
| Zen 5 | **Turin** | EPYC 9005 | 2024 | ABI spec v1.56–1.57. Attestation report `VERSION=5` adds `LAUNCH_MIT_VECTOR` / `CURRENT_MIT_VECTOR` fields and `PAGE_SWAP_DISABLED` policy bit. New ARK (Turin root) — Milan/Genoa ARKs do **not** validate Turin VCEKs. |

KDS endpoints differ by `product_name`:
- `https://kdsintf.amd.com/vcek/v1/Milan/...`
- `https://kdsintf.amd.com/vcek/v1/Genoa/...`
- `https://kdsintf.amd.com/vcek/v1/Bergamo/...`
- `https://kdsintf.amd.com/vcek/v1/Turin/...`

A verifier MUST extract `product_name` from the report (either via CPUID fields at offset `0x188`–`0x18A` in v3+ reports, or via configuration matching the deployment) and fetch the correct ARK/ASK chain for that generation.

---

## 4. Measurement methodology

### 4.1 Launch measurement (`MEASUREMENT`, offset `0x090`, 48 bytes)

The launch measurement is computed by the **AMD Secure Processor (AMD-SP / PSP)** at VM launch, in the firmware command `SNP_LAUNCH_UPDATE` / `SNP_LAUNCH_FINISH`.

Per AMD's "[SEV-SNP Attestation: Establishing Trust in Guests](https://www.amd.com/content/dam/amd/en/documents/developer/lss-snp-attestation.pdf)" white paper (Jeremy Powell, AMD): the measurement is a SHA-384 digest computed incrementally over each launch page as it is encrypted into guest memory. The hash chain includes:

1. Initial CPU state pages (VMSA — VM Save Area for the BSP and APs).
2. Guest OVMF / firmware pages.
3. Any pages loaded by `SNP_LAUNCH_UPDATE` (typically the bootloader and kernel images, depending on whether direct boot or OVMF-driven boot is used).

The hash is "incremental" in the sense that each page contributes to a running SHA-384 state, with page metadata (GPA, page type, VMPL, permissions) included in the hash input.

Per [Contrast](https://docs.edgeless.systems/contrast/architecture/attestation/amd-details), in practice for Linux guests the measurement covers "kernel, initrd, and cmdline" — but only because OVMF measures those into the launch hash before handing off. The PSP itself hashes only what is loaded via firmware commands; the guest OS chain is anchored via OVMF's measured boot.

### 4.2 `HOST_DATA` (offset `0x0C0`, 32 bytes)

Host-supplied at `SNP_LAUNCH_FINISH`. **Not measured into the launch hash**, but signed as part of the attestation report. Typical use: hypervisor binds a configuration document (e.g., Kata initdata, K8s pod spec digest) into the report. Verifier compares against expected configuration.

### 4.3 `REPORT_DATA` (offset `0x050`, 64 bytes)

Supplied by the **guest** in the `SNP_GUEST_REQUEST` ioctl when requesting a fresh report. This is the verifier's freshness channel — the verifier provides a nonce (typically a random challenge) to the guest, which the guest passes verbatim as `REPORT_DATA`. The verifier checks `REPORT_DATA == challenge` to defeat replay.

### 4.4 Runtime extensions

VMPL changes do **not** extend the measurement. SEV-SNP has no "extend" PCR-style register. The launch measurement is a one-shot snapshot of the VM at launch; runtime integrity (e.g., kernel module loading) is out of scope for the SEV-SNP attestation report and must be handled by an in-guest mechanism (IMA, dm-verity, etc.).

VMPL-specific attestation reports: a guest at VMPL=N can request a report identifying that VMPL via the `MSG_REPORT_REQ` field — useful for paravisor-style designs (e.g., Microsoft OpenHCL, AMD SVSM) where a privileged VMPL=0 paravisor attests to a less-privileged VMPL=2 guest workload.

---

## 5. Open standards alignment

### 5.1 IETF RATS (RFC 9334)

SEV-SNP attestation maps cleanly onto RFC 9334's **passport model**:
- **Attester** = SEV-SNP guest + AMD-SP firmware.
- **Verifier** = relying party (e.g., on-chain verifier contract, or off-chain service).
- **Evidence** = the 1184-byte attestation report + VCEK/VLEK cert chain.
- **Attestation Results** = verifier-issued claims (e.g., signed JWT) consumable by a downstream Relying Party.

### 5.2 IETF EAT (RFC 9711) / CoRIM

AMD does not natively emit EAT (Entity Attestation Token, [RFC 9711](https://www.rfc-editor.org/rfc/rfc9711)). The attestation report is a raw binary struct. However, the **CoRIM draft** ([draft-deeglaze-amd-sev-snp-corim-profile-02](https://www.ietf.org/archive/id/draft-deeglaze-amd-sev-snp-corim-profile-02.html)) defines a CBOR mapping for SEV-SNP reports that aligns with the RATS architecture and lets verifiers express reference values and endorsements in a standard format.

The CoRIM draft registers:
- Media type `application/vnd.amd.sev-snp.attestation-report` (raw binary).
- Media type `application/vnd.amd.ghcb.guid-table`.
- CoAP Content-Format IDs 10572 and 10573 respectively.

It also defines two attestation classes via OID-encoded `class-id`:
- "By Chip" (VCEK-based): OID `1.3.6.1.4.1.3704.3.1`
- "By CSP" (VLEK-based): OID `1.3.6.1.4.1.3704.3.2`

### 5.3 TCG alignment

SEV-SNP is **not** a TPM. There is no PCR bank, no extend-after-launch primitive, no quote-by-name. Conceptual mapping:

| TCG / TPM concept | SEV-SNP analogue |
|------|------|
| Endorsement Key (EK) | VCEK (per-chip) or VLEK (per-CSP). |
| Attestation Identity Key (AIK) | None; the EK signs directly. The ID block key provides a secondary identity. |
| PCR digest | `MEASUREMENT` field (launch only — no runtime extends). |
| Quote | `ATTESTATION_REPORT`. |
| TPM2_Quote nonce | `REPORT_DATA`. |

There is no formal TCG profile for SEV-SNP. Cross-platform "DICE/TCG-compatible" verification layers (e.g., Microsoft Azure Attestation, IBM Confidential Containers) wrap SEV-SNP reports into higher-level tokens but do not change the underlying primitive.

---

## 6. Cryptographic primitives

| Component | Algorithm | Parameters |
|-----------|-----------|-----------|
| Attestation report signature | **ECDSA P-384** | SHA-384 digest of report bytes `0x000..0x2A0`; signature is 96 bytes (r||s, 48 bytes each), zero-padded to 72 bytes per field in the report. |
| VCEK key | ECDSA P-384 | Derived by HKDF from chip secret + `TCB_VERSION`. |
| ASK key | RSA-4096 | SHA-384 hashing. |
| ARK key | RSA-4096 | Self-signed, SHA-384. |
| VLEK key | ECDSA P-384 | Same curve as VCEK; loaded rather than derived. |
| Launch measurement | SHA-384 | Hash chain over launch pages + VMSA + metadata. |
| `ID_KEY_DIGEST`, `AUTHOR_KEY_DIGEST` | SHA-384 | Of the corresponding ECDSA-P384 public key DER. |

**Curve.** All ECDSA in the AMD chain uses NIST P-384 (`secp384r1`), with the **point-encoded as 48 bytes big-endian per coordinate** in DER but **zero-padded to 72 bytes per coordinate** when embedded in the `SIGNATURE` struct in the report. This 72-byte padding is the source of many verifier bugs.

**Signature input domain.** The signed bytes are the first 672 bytes (`0x000..0x2A0`) of the report — the signature struct itself is excluded from the hash.

---

## 7. On-chain verification cost (EVM)

### 7.1 Cost components

| Item | Approximate cost |
|------|------|
| ECDSA P-384 verify (Solidity, no precompile) | **~1.5M–2.0M gas** typical; ~700k with heavy optimization. |
| RSA-4096 verify (for ASK→ARK and ARK self-sig) | ~400k–800k gas per signature using `modexp` (precompile 0x05). |
| SHA-384 of 672-byte report | ~10k gas. |
| Cert chain DER parsing in Solidity | 200k–500k gas depending on what you skip. |
| **Total naive on-chain verification** | **~4M–5M gas** for a full chain (ARK→ASK→VCEK→Report). |

The EVM has no P-384 precompile (only secp256k1 via `ecrecover`, plus `bn128` and `bls12-381`). EIP-7212 (RIP-7212 on rollups) adds secp256r1 / P-256, **not** P-384. All P-384 work is done in Solidity / Yul, typically using a Shamir's trick + window-NAF implementation.

### 7.2 Real-world implementations and measured costs

The single best public datapoint comes from **Automata DCAP** (Intel SGX/TDX target, but the gas-cost shape is identical to SEV-SNP — both are ECDSA P-256/P-384 + RSA cert chain + DER parsing):

| Verification path | Gas cost |
|-------------------|---------:|
| Full on-chain DCAP verify (no RIP-7212) | **~5M gas** |
| Full on-chain DCAP verify (with RIP-7212 precompile) | **~4M gas** |
| RISC Zero Groth16 proof verification | **~522k gas** |
| SP1 Groth16 proof verification | **~493k gas** |
| SP1 Plonk proof verification | **~569k gas** |

Source: [Automata-Network/automata-dcap-attestation](https://github.com/automata-network/automata-dcap-attestation). SEV-SNP follows the same pattern with P-384 instead of P-256 — adds ~30–50% to the on-chain path (more bits per scalar multiply, no precompile available), so a "full on-chain SEV-SNP verify" lands in the **5M–7M gas** range; ZK-proof-on-chain stays in the **500–600k gas** range regardless of which TEE generated the report.

Other relevant prior art:

- **Automata DCAP/zkDCAP** ([github.com/automata-network/automata-dcap-attestation](https://github.com/automata-network/automata-dcap-attestation)) — production-deployed on Automata 1RPC, supports Risc0 / SP1 / SP1 Plonk / Pico proof systems. SEV-SNP support is on the roadmap as of late 2025.
- **Marlin Oyster** ([marlin.org/oyster](https://www.marlin.org/oyster)) — TEE coprocessor framework. Original AWS Nitro path uses on-chain `NitroProver`. SEV-SNP path is verified off-chain by Oyster operators and bonded via stake-slashing.
- **Phala Network dstack** ([Dstack-TEE/dstack](https://github.com/Dstack-TEE/dstack)) — TEE-agnostic attestation orchestrator; uses an off-chain verifier service for SEV-SNP with on-chain anchoring of the verified result. Avoids on-chain P-384.
- **Hyperlane ISM** — Attestation handled at the Interchain Security Module layer; SEV-SNP path uses a multisig of off-chain validators rather than direct on-chain crypto verification.
- **LIT Protocol sev-snp-utils** ([github.com/LIT-Protocol/sev-snp-utils](https://github.com/LIT-Protocol/sev-snp-utils)) — Rust crate, off-chain verifier only.
- **ufcg-lsd SPIRE attestor** ([github.com/ufcg-lsd/spire-amd-sev-snp-node-attestor](https://github.com/ufcg-lsd/spire-amd-sev-snp-node-attestor)) — SPIRE plugin, off-chain verifier with cert-chain caching against KDS.

### 7.3 Reduction strategies

1. **ZK proof of attestation validity.** Run the entire P-384 + RSA + cert-parsing pipeline in a zkVM (Risc0, SP1, Jolt) or a circuit (Halo2, Plonky2). Post a ~200-byte SNARK/STARK; on-chain verification ~250k gas. This is the dominant pattern today.
2. **Off-chain attestation, on-chain anchoring.** Trust a multisig of verifiers (Hyperlane model) — pragmatic but trust-shifted.
3. **Single-signer on-chain.** Skip ARK/ASK on-chain; assume a hard-coded VCEK pubkey is trusted (insecure for multi-chip fleets — only useful for single-machine demos).
4. **Optimized P-384 in Solidity/Yul.** Implementations like [tdx-zk/p384-sol](https://github.com/automata-network/p384-sol-verifier) push verify to ~700k–1M gas. Still expensive enough that ZK is preferred for production.

### 7.4 Proof / data sizes

- Attestation report: **1184 bytes**.
- VCEK cert (DER): typically 1.2–1.6 KB.
- ASK cert (DER): ~1.5 KB.
- ARK cert (DER): ~1.5 KB.
- **Full cert chain + report: ~5–6 KB**.

Posting raw to EVM at 16 gas/byte for nonzero calldata is ~80k–100k gas just for the data. On L2s with calldata compression this is much cheaper.

---

## 8. Empirical references

### Primary AMD sources
- **SEV-SNP Firmware ABI Specification (56860)** — [PDF, current rev](https://www.amd.com/content/dam/amd/en/documents/developer/56860.pdf); [HTML index](https://docs.amd.com/v/u/en-US/56860); [rev 1.54 mirror](http://kib.kiev.ua/x86docs/AMD/SEV/56860-r1.54.pdf). Defines `ATTESTATION_REPORT` (Table 23), `TCB_VERSION` (Table 5), `POLICY` bits, `PLATFORM_INFO` (Table 24), `MSG_REPORT_RSP` (Table 24).
- **VCEK Certificate and KDS Interface Specification (57230)** — [PDF](https://www.amd.com/content/dam/amd/en/documents/epyc-technical-docs/specifications/57230.pdf). Defines KDS URL structure, VCEK X.509 extensions / OIDs, CRL endpoint, per-generation `hwid` length.
- **AMD SEV-SNP Attestation white paper** (Jeremy Powell, AMD) — [PDF](https://www.amd.com/content/dam/amd/en/documents/developer/lss-snp-attestation.pdf).
- **"SEV-SNP: Strengthening VM Isolation"** — AMD white paper on SEV-SNP architecture, [PDF](https://www.amd.com/content/dam/amd/en/documents/epyc-business-docs/white-papers/SEV-SNP-strengthening-vm-isolation-with-integrity-protection-and-more.pdf).
- **AMD SEV public page** — [https://www.amd.com/en/developer/sev.html](https://www.amd.com/en/developer/sev.html).
- **AMD KDS** — `https://kdsintf.amd.com/` (root); endpoints:
  - `/vcek/v1/{Milan,Genoa,Bergamo,Turin}/cert_chain` (returns ASK+ARK concatenated, DER)
  - `/vcek/v1/{gen}/{hwid}?blSPL=...&teeSPL=...&snpSPL=...&ucodeSPL=...` (leaf VCEK cert, DER)
  - `/vcek/v1/{gen}/crl` (revocation list)
  - `/vlek/v1/{gen}/cert_chain` (VLEK chain — ASVK+ARK)
- **AMD Security Bulletin AMD-SB-3015** — BadRAM mitigation firmware update advisory.

### AMD reference implementations
- **AMDESE/sev-guest** (archived Jan 2024) — [github.com/AMDESE/sev-guest](https://github.com/AMDESE/sev-guest); `include/attestation.h` defines the C struct.
- **AMDESE/AMDSEV** — [github.com/AMDESE/AMDSEV](https://github.com/AMDESE/AMDSEV) — firmware build scripts, host kernel patches.
- **AMDESE/linux** — SEV-SNP host + guest kernel branches.
- **AMDESE/sev-tool** (host PSP tool).

### Successor (VirTEE)
- **virtee/sev** (Rust crate) — [github.com/virtee/sev](https://github.com/virtee/sev); `src/firmware/guest/types/snp.rs` is the canonical Rust `AttestationReport`.
- **virtee/snpguest** — [github.com/virtee/snpguest](https://github.com/virtee/snpguest) — guest CLI; fetches certs from KDS and validates locally.
- **virtee/sevctl** — host-side counterpart.

### IETF / standards
- **RFC 9334 — RATS Architecture** — [rfc-editor.org/rfc/rfc9334](https://www.rfc-editor.org/rfc/rfc9334).
- **RFC 9711 — EAT** — [rfc-editor.org/rfc/rfc9711](https://www.rfc-editor.org/rfc/rfc9711).
- **draft-deeglaze-amd-sev-snp-corim-profile-02** — [ietf.org/archive/id/draft-deeglaze-amd-sev-snp-corim-profile-02.html](https://www.ietf.org/archive/id/draft-deeglaze-amd-sev-snp-corim-profile-02.html).
- **IETF RATS wiki — AMD KDS reference values** — [wiki.ietf.org/group/rats/referencevalues/amd-key-distribution-service](https://wiki.ietf.org/group/rats/referencevalues/amd-key-distribution-service).

### Academic / industry analyses
- **Buhren, Werling, Seifert — "One Glitch to Rule Them All"** (CCS 2021) — voltage glitching attack against AMD-SP. Demonstrates VCEK extraction on Naples/Rome; later mitigated for Milan+. [ACM DL](https://dl.acm.org/doi/abs/10.1145/3460120.3484779), [arXiv](https://arxiv.org/abs/2108.04575), [proof-of-attack repo](https://github.com/PSPReverse/amd-sp-glitch).
- **De Meulemeester, Wilke, Oswald, Eisenbarth, Verbauwhede, Van Bulck — "BadRAM: Practical Memory Aliasing Attacks on Trusted Execution Environments"** (S&P 2025) — DRAM-aliasing attack against SEV-SNP via tampered SPD; bypasses attestation. Mitigated by firmware updates that "securely validate memory configurations during the processor's boot process" (per AMD-SB-3015). The `ALIAS_CHECK_COMPLETE` bit in `PLATFORM_INFO` (bit 5) indicates the post-mitigation alias check has run. [Project site & paper](https://badram.eu).
- **Schlüter, Sridhara, et al. — "WeSee"** (S&P 2024) — interrupt-injection attack against SEV-SNP.
- **Microsoft Research — Veracruz, OpenHCL** — paravisor architectures for SEV-SNP at VMPL=0 attesting to VMPL=2 workloads.

### On-chain verifier prior art
- **Automata Network DCAP** — [github.com/automata-network/automata-dcap-attestation](https://github.com/automata-network/automata-dcap-attestation).
- **Marlin SEV-SNP verifier** — [github.com/marlinprotocol/sev-snp-verifier](https://github.com/marlinprotocol/sev-snp-verifier).
- **Phala dstack** — [github.com/Dstack-TEE/dstack](https://github.com/Dstack-TEE/dstack).
- **LIT-Protocol/sev-snp-utils** (Rust verifier, off-chain) — [github.com/LIT-Protocol/sev-snp-utils](https://github.com/LIT-Protocol/sev-snp-utils).
- **Edgeless Contrast** — [docs.edgeless.systems/contrast/architecture/attestation/amd-details](https://docs.edgeless.systems/contrast/architecture/attestation/amd-details).
- **SPIRE SEV-SNP plugin RFC** — [github.com/spiffe/spire/issues/4469](https://github.com/spiffe/spire/issues/4469).

---

## 9. Load-bearing claims (for `tee-specialist`)

The following are the highest-leverage facts the agent must encode when reasoning about SEV-SNP verification design:

1. **The attestation report is 1184 bytes (0x4A0); the signed region is the first 672 bytes (0x000..0x2A0); the signature itself occupies the final 512 bytes.** Hashing the wrong region is the most common verifier bug.

2. **Signature is ECDSA P-384 with SHA-384.** No EVM precompile exists. Native Solidity verification costs ~1.5M–2M gas; production designs use ZK rollup-style proof aggregation (Automata, Marlin) to reduce on-chain cost to ~250k.

3. **VCEK is per-chip + per-`REPORTED_TCB`.** The verifier MUST fetch the VCEK cert for the exact `REPORTED_TCB` value carried in the report — `https://kdsintf.amd.com/vcek/v1/{gen}/{hwid}?blSPL={bl}&teeSPL={tee}&snpSPL={snp}&ucodeSPL={ucode}` — and MUST NOT cache by chip ID alone. A stale cert (different TCB) will verify the signature but lie about the TCB.

4. **VLEK is the CSP alternative to VCEK.** Bit `SIGNING_KEY` in `KEY_INFO` (offset `0x048`) selects: 0=VCEK, 1=VLEK. VLEK certs are signed by AMD's ASVK (not ASK) and are loaded into firmware by the CSP. Verifiers must support both chains.

5. **Cert chain is generation-specific.** Milan ARK, Genoa ARK, Turin ARK are different keys. The verifier must derive the generation either from `product_name` configuration or from the V3+ report's `CPUID_FAM/MOD/STEP` fields at offsets `0x188`–`0x18A`.

6. **`REPORT_DATA` (offset `0x050`, 64 bytes) is the freshness channel.** Verifier issues a nonce, includes it as `REPORT_DATA`, and rejects any report whose `REPORT_DATA` doesn't match the expected challenge. **Anti-replay enforcement is the verifier's job, not AMD's.**

7. **`MEASUREMENT` (offset `0x090`, 48 bytes) covers launch state only.** SEV-SNP has no PCR-extend mechanism. Runtime integrity must be enforced in-guest (IMA, dm-verity) and surfaced separately if the verifier needs runtime evidence.

8. **Anti-rollback requires policy.** The chip will happily sign reports with any historical TCB version that was once valid. The verifier MUST enforce `REPORTED_TCB ≥ minimum_acceptable_TCB` to defeat rollback attacks. AMD provides no automatic CRL for stale TCBs — it is purely a verifier-policy decision.

9. **Policy bit 19 (`DEBUG_ALLOWED`) must be 0 for production.** A debug-allowed guest can have its memory inspected by the hypervisor; verifier MUST reject such reports for any sensitive workload.

10. **There is no on-chain P-384 precompile, on any major EVM chain (Ethereum mainnet, OP Stack, Arbitrum, Sei).** Plan the Tide on-chain verifier around either (a) Solidity P-384 verification at ~1.5M gas (acceptable on L2/L3, expensive on L1), or (b) zkSEV-SNP proofs via Risc0/SP1 that compress the entire verification to a ~250k-gas SNARK.
