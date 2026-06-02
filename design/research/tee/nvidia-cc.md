# NVIDIA Confidential Compute Attestation — Empirical Reference

> Research doc for Tide's `tee-specialist` agent + future on-chain NVIDIA attestation
> verifier work. Cites primary NVIDIA documentation and peer-reviewed sources. Where
> specs are not publicly available (NVIDIA explicitly withholds some attestation
> internals), this doc says so and marks the boundary.
>
> **Status:** Draft (research-grade reference). Last updated 2026-06-01.

---

## Boundary disclosure (read first)

NVIDIA publishes architectural overviews and the verifier client (open-source
[`nvtrust`](https://github.com/NVIDIA/nvtrust)) but **deliberately withholds**
per-index SPDM measurement semantics, internal firmware structures, and
per-component golden hashes (checked indirectly via the RIM service). NVIDIA's
stated stance ([developer forum](https://forums.developer.nvidia.com/t/attestation-report-measurements/322575)):

> "We do not post the exact measurements, as they are limited to internal
> states, registers, etc. wherein an evidence-policy is not valuable to a
> relying policy."

What IS public and used here: the chain-of-trust shape, NRAS API behavior, the
verifier code, SPDM session/measurement protocol, cryptographic primitives,
deployment mode semantics, and performance characteristics.

---

## 1. NVIDIA GPU attestation report — structure

### 1.1 Wire format

NVIDIA GPU attestation evidence is carried in **DMTF SPDM 1.1 `MEASUREMENTS`
response messages**. The GPU firmware's `RISC-V` security processor (NVIDIA
calls it the GSP / FSP family) acts as an SPDM responder; the host driver
(running inside the CPU TEE) is the SPDM requester.

> "DMTF's SPDM 1.1 MEASUREMENT response message is used as the attestation
> report. An attestation report is generated that provides a cryptographically
> signed set of measurements."
> ([ACM Queue — "Creating the First Confidential GPUs"](https://queue.acm.org/detail.cfm?id=3623391))

Each measurement entry has four fields per the SPDM spec:

| Field            | Type      | Notes                                           |
| ---------------- | --------- | ----------------------------------------------- |
| `index`          | uint8     | Slot number (NVIDIA does not publish mapping)   |
| `type`           | uint8     | SPDM-defined measurement type                   |
| `size`           | uint16    | Length of the value                             |
| `value`          | bytes     | Hash or raw measurement                         |

(Field shape per SPDM 1.1 §10.11.1, confirmed by NVIDIA forum response.)

### 1.2 What the report measures

NVIDIA's H100 whitepaper lists the categories collected — `Confidential
Compute on NVIDIA Hopper H100` WP-11459-001, §"Attesting the GPU":

> "Before a CVM uses the GPU, it must authenticate all GPUs as genuine before
> including it in its trust boundary. It does this by retrieving a Device
> Identity Certificate (signed with a device-unique ECC-384 key pair) from the
> device or the NVIDIA Device Identity Service."

The attestation report on H100 contains **64 structured measurement records**
per the MLSys 2026 paper [*NVIDIA GPU Confidential Computing Demystified*](https://arxiv.org/abs/2507.02770)
(Gu et al.):

> "64 structured records, each containing a measurement specification, a size
> field, and a cryptographic hash"

The measurements span:

- **GPU hardware identity** — Per-Device Identifier (PDI), burned at manufacture
- **Firmware versions** — bootrom, GSP firmware, BIOS, microcode versions
- **VBIOS** — Video BIOS hash
- **Driver-observed runtime state** — register checks, fuses, mode selection
  (CC-Off / CC-On / CC-DevTools)
- **DevTools / debug flags** — JTAG-disable bit, performance-counter mask
- **Memory configuration** — ECC enable, memory firewall state

The GPU's internal **chain of trust during boot** is (per MLSys 2026):

```
CEC EROT  ->  FSP  ->  GSP  ->  SEC2
```

Where:
- **CEC EROT** — External Root of Trust co-processor (immutable)
- **FSP** — Firmware Security Processor (boots first, verifies GSP)
- **GSP** — GPU Security Processor (signs attestation; SPDM responder)
- **SEC2** — Security engine for workload launch / scrubber

Each stage cryptographically validates the next; the measurements of all four
contribute to the attestation report.

(Categories from NVIDIA H100 whitepaper §"Threats and Mitigations", ACM Queue
description of SPDM measurement set, and MLSys 2026 paper.)

### 1.3 Freshness — nonce mechanism

The verifier issues a 128-bit nonce; the GPU embeds it into the signed
SPDM response. From NVIDIA H100 whitepaper §"Attesting the GPU":

> "The NVIDIA Attestation generates a random nonce of at least 128-bits using
> a secure random number generator meeting NIST SP800-90 A/B/C standards…
> The attester embeds the nonce in endorsed evidence."

NRAS also caches `(nonce, device_cert_hash)` with a **24-hour TTL** to prevent
replay (whitepaper, step 10 of the workflow).

### 1.4 Hopper (H100/H200) vs Blackwell (B100/B200/GB200) differences

| Property                       | Hopper (H100/H200)                | Blackwell (B100/B200/GB200)                  |
| ------------------------------ | --------------------------------- | -------------------------------------------- |
| **CPU↔GPU data path**          | Encrypted bounce buffer in shared memory (software) | **TEE-I/O** with inline link-layer encryption (PCIe IDE + TDISP) |
| **Multi-GPU encryption**       | Bounce buffers in unprotected GPU memory for NVLink peer transfer | **Inline NVLink encryption** (hardware link-layer) |
| **Throughput vs non-CC mode**  | CPU-GPU limited to ~4 GB/s (CPU encrypt bottleneck) | "Nearly identical throughput performance as compared to unencrypted modes" |
| **CVM ↔ GPU shared session**   | SPDM session in driver, encrypted command/data buffers | TDISP-bound device assignment with IDE-encrypted link |
| **Multi-GPU attestation**      | Per-GPU attestation, aggregate via NRAS | Adds **NVSwitch attestation** and **PPCIE verifier** (`nvtrust/guest_tools/ppcie-verifier`) |

Sources:
- [NVIDIA Secure AI with Blackwell and Hopper GPUs whitepaper](https://docs.nvidia.com/nvidia-secure-ai-with-blackwell-and-hopper-gpus-whitepaper.pdf) (WP-12554-001 v1.3)
- [Corvex blog on B200 Confidential Computing](https://www.corvex.ai/blog/confidential-computing-meets-nvidia-hgxtm-b200-secure-ai-without-the-performance-trade-off)
- [NVIDIA developer forum on Blackwell NVLink encryption](https://forums.developer.nvidia.com/t/how-does-blackwell-support-high-performace-nvlink-encryption/337614)

**TEE-I/O is the critical Blackwell delta** — PCI-SIG IDE (Integrity & Data
Encryption) + TDISP (TEE Device Interface Security Protocol) replace Hopper's
software bounce-buffer, eliminating the ~4 GB/s host↔GPU bandwidth bottleneck.

---

## 2. Signing chain

### 2.1 Hierarchy

```
NVIDIA Device Identity Root CA          (long-lived, NVIDIA-controlled root)
        |
        +-- NVIDIA GPU CA (intermediate, per family)
                |
                +-- Per-device Identity Cert (IK)
                        - ECC P-384 keypair
                        - Private key burned into GPU fuses at manufacture
                        - Public key retained by NVIDIA; private destroyed
                        |
                        +-- Attestation Key (AK) Cert
                                - Derived deterministically at each chip reset
                                - Signed by the per-device IK
                                - Signs the SPDM MEASUREMENTS evidence
```

Verbatim from H100 whitepaper, §"Attesting the GPU":

> "Evidence is endorsed with an Attestation Key (AK). The AK, for a GPU, is
> generated deterministically at each full chip reset. A certificate is
> generated for the AK and signed by a device identity key that is per
> device. That certificate chain has one or more intermediate certificates
> above the identity key to prove trust in the AK."

> "This device-unique private Identity Key (IK) is burned into fuses of each
> H100; the public key is retained, but all copies of these private keys are
> destroyed during the manufacturing process."

### 2.2 Certificate format

X.509 certs with **ECDSA P-384 (secp384r1)** signing per `Confidential Compute
on NVIDIA Hopper H100`. Hash is SHA-384 (paired with the curve under SP 800-186
guidance). NVIDIA's PKI complies with RFC 5280 chain validation rules per
NRAS workflow doc.

### 2.3 Where the chain is fetched

- **Device cert** — retrievable from the GPU itself via SPDM
  `GET_CERTIFICATE`, or from NVIDIA's Device Identity Service via the PDI
- **Intermediate certs / Root** — pulled from NVIDIA's public PKI
  distribution endpoints
- **RIM bundle** (Reference Integrity Measurements — golden hashes per
  driver/firmware version) — fetched from the **NVIDIA RIM Service**
- **Revocation status** — OCSP via NVIDIA's `CertZapper` OCSP endpoint
  (whitepaper, step 16)

From H100 whitepaper §"Attesting the GPU" (steps 14–16):

> "NRAS fetches RIM Bundle (Golden Measurements) from RIM Service for the
> device using the driver-version and GPU model provided in the evidence."
>
> "NRAS gets the certificate chain from RIMM Bundle and call the CertZapper
> OCSP endpoint to validate whether the certificate is valid."

The DMTF-aligned **CoRIM** (Concise Reference Integrity Manifest) format is
the structured form NVIDIA publishes RIM data in for the broader RATS
ecosystem — see [NVIDIA Device Attestation and CoRIM-based Reference
Measurement Sharing v5.0](https://docs.nvidia.com/networking/display/dpunicattestation) (this URL is for the NIC product line but
documents the same CoRIM approach NVIDIA uses across product families).

---

## 3. Verifier architectures

### 3.1 NRAS — NVIDIA Remote Attestation Service

Cloud-hosted RESTful verifier at `https://nras.attestation.nvidia.com/v1/`.
The client (typically the relying party or the workload itself) does:

1. Generates a nonce (≥128-bit, NIST SP 800-90 RNG)
2. Asks the GPU (via `cc_admin` / `nvml`) for SPDM evidence with that nonce
3. Posts evidence + device cert chain + nonce to NRAS over HTTPS
4. Receives a signed **EAT** (Entity Attestation Token) in JWT format

#### NRAS API — `POST /v1/attest/gpu`

Per the [NVIDIA Attestation API reference](https://docs.api.nvidia.com/attestation/reference/attestgpu):

**Request body (JSON):**

| Field         | Type        | Description                                                |
| ------------- | ----------- | ---------------------------------------------------------- |
| `nonce`       | string      | Randomly generated 64-character hexadecimal (256-bit)      |
| `arch`        | enum string | `HOPPER` or `BLACKWELL`                                    |
| `evidence`    | string      | Base64-encoded SPDM MEASUREMENTS response                  |
| `certificate` | string      | Base64-encoded GPU attestation certificate chain           |

**Response:**
- 200 OK → `application/json` body containing a single JWT (claims version 1.0)
- The JWT is signed by an ephemeral NRAS L3 cert (24h TTL) with `alg=ES384`

**Standard NVIDIA claims** in the JWT (per Intel Trust Authority's documentation
of single-GPU NRAS tokens):

- `x-nvidia-gpu-driver-version`
- `hwmodel`
- `x-nvidia-attestation-detailed-result`
- Standard JWT claims: `iss`, `jti`, `exp`, `iat`, `nonce`

**Multi-GPU response** uses NRAS V3 format per Intel Trust Authority docs:

- `x-nvidia-overall-att-result` (boolean) — aggregate verdict
- `claim_details` — per-device object keyed by GPU identifier
- Wraps per-device tokens as **RATS §A.2.3 Detached EAT Bundles**

#### NRAS verification workflow (from H100 whitepaper)

NRAS internals (from H100 whitepaper §"Attesting the GPU") in detail:

- Validates evidence against SPDM 1.1
- Validates the cert chain per RFC 5280
- Calls the **RIM service** for the matching driver+GPU version → fetches
  golden hashes
- Calls **CertZapper OCSP** for revocation
- Applies **UAM (Unified Appraisal Module)** — an Open Policy Agent / Rego
  policy engine — to decide pass/fail per appraisal policy
- Signs the EAT JWT with an **ephemeral L3 cert (24h TTL)**
- Returns a JWT with `jti` (JWT ID, anti-replay) + expiry

> "NRAS uses an ephemeral certificate L3 for signing the EAT.
> This L3 certificate will be short-lived, 24 hours of ttl."

The JWKS endpoint (for relying parties to fetch the verifier's signing
public key) is served by NRAS itself; the relying party validates the EAT
signature against keys retrieved from that endpoint.

**Trust trade-off:** NRAS is a third-party trust dependency. The relying party
trusts NVIDIA's NRAS service + its ephemeral L3 key not just the GPU root.
For deployments that need to minimize the trust set (e.g., air-gapped or
sovereign deployments), use Local Verifier.

### 3.2 Local Verifier — `nvtrust` / `nv-local-gpu-verifier`

Open-source Python verifier at
`nvtrust/guest_tools/gpu_verifiers/local_gpu_verifier/`. The library has the
following modules (per the nvtrust README):

| Module          | Responsibility                                                    |
| --------------- | ----------------------------------------------------------------- |
| `cc_admin`      | Entry point; orchestrates the whole flow                          |
| `nvmlHandler`   | NVML wrapper — fetches driver version, GPU cert, attestation report |
| `Attestation`   | Parses SPDM MEASUREMENTS, verifies the AK signature               |
| `RIM`           | Parses RIM (SWID-tag schema), verifies XML signature              |
| `Verifier`      | Compares runtime measurements to RIM golden values                |

End-to-end flow:

- Issues an SPDM session with the local GPU
- Generates its own nonce
- Parses the returned `MEASUREMENTS` block
- Fetches the device cert (from GPU) and intermediate/root certs (from NVIDIA
  PKI)
- Fetches RIM bundle from the public RIM service (keyed on driver version +
  GPU model)
- Performs OCSP revocation check
- Compares measurements against RIM golden values
- Returns a structured pass/fail result (the library can also emit an EAT for
  downstream consumers)

The Python verifier (`nv-local-gpu-verifier`) is **deprecated as of 2026**
in favor of a C++ replacement at
[`NVIDIA/attestation-sdk`](https://github.com/NVIDIA/attestation-sdk),
which is positioned as the long-term standard SDK. The Python tool is still
shipped and supported in nvtrust but new integrations should target the C++
SDK / CLI.

**Trade-offs (NRAS vs Local):**

| Property              | NRAS                         | Local Verifier                |
| --------------------- | ---------------------------- | ----------------------------- |
| Trust set             | + NVIDIA verifier service    | NVIDIA PKI + RIM + OCSP only  |
| Cert/RIM caching      | Server-side, optimized       | Client must manage caches     |
| Air-gap support       | No (needs HTTPS to NVIDIA)   | Yes (with pre-staged RIM)     |
| Replay protection     | Server-side nonce cache (24h)| Client-managed nonce         |
| Result format         | Signed EAT JWT (verifiable)  | In-process result struct      |
| Cross-tenant trust    | Same EAT JWKS for everyone   | Per-tenant policy             |

### 3.3 Intel Trust Authority (ITA) — third option

[Intel Trust Authority](https://docs.trustauthority.intel.com/main/articles/articles/ita/concept-gpu-attestation.html)
publishes a "GPU Remote Attestation" capability that combines NVIDIA's SPDM
evidence with Intel TDX quotes. This is the canonical "joint CPU+GPU
attestation" reference verifier from Intel's side.

**ITA API endpoint:** `POST /appraisal/v2/attest`

**Modes:**
- *Composite attestation* — Client supplies both `tdx_args` and `gpu_args`
  to the Python `get_token_v2` method; ITA verifies both legs and emits a
  combined token
- *GPU-only attestation* — Verifies GPU evidence only (ITA proxies to NRAS)

**Multi-GPU:** batch attestation up to 8 devices per request via an
`evidence_list` array.

**Requirements:** Ubuntu 24.04 LTS, kernel 6.8+, root for TDX evidence
collection.

ITA's value-add over raw NRAS: a single relying-party trust anchor (Intel)
spanning CPU + GPU evidence with a unified Rego policy framework. The
trade-off is the broader trust set (Intel + NVIDIA + ITA service).

---

## 4. Joint CPU + GPU attestation

### 4.1 The binding problem

A pure GPU attestation says "this is a genuine H100 in CC-On mode." A pure
CPU TEE attestation (TDX quote, SEV-SNP report) says "this is a TDX/SNP TVM
running this measurement." Neither alone proves the CVM and the GPU are
in the same TCB. The binding must come from a shared cryptographic value.

### 4.2 Hopper binding (software-based)

For H100, binding is via **nonce coupling and an SPDM session key**:

1. The CVM (TDX or SNP) generates a TEE attestation quote with `REPORT_DATA`
   set to a hash that includes the SPDM session public key (or an attestation
   nonce derived from it).
2. The CVM drives SPDM `KEY_EXCHANGE` with the GPU — binds a Diffie-Hellman
   ephemeral key; the GPU's signed `MEASUREMENTS` transitively bind through
   the session key.
3. Relying party verifies all three: CPU TEE quote (TDX/SNP genuine,
   `REPORT_DATA` matches), GPU attestation (genuine H100 CC-On, AK-signed,
   fresh nonce), and the session-key linkage.

H100 whitepaper §"Secure Session Establishment":
> "There are two parts to establishing the secure session: A Diffie-Hellman
> key exchange for setting up a shared symmetric session key. Retrieving the
> Device Attestation Report that contains the measurements of the HW and SW
> components."

The TEE quote is generated separately via the CSP attestation path (Azure
Attestation, GCP, or Intel TDX DCAP). Relying party gathers both and verifies
the binding off-platform.

### 4.3 Blackwell binding (hardware TEE-I/O)

Blackwell brings the binding into the link layer via PCI-SIG TDISP/IDE:

- **IDE (Integrity & Data Encryption)** establishes link-layer encryption
  between the TDISP-aware host bridge and the GPU
- **TDISP** assigns the device to a single TVM and enforces it in hardware;
  the host CPU's TEE Security Manager and the GPU mutually attest before the
  device transitions into the **LOCKED** TDISP state
- The TVM measurement and the GPU attestation are bound via the TDISP
  device-interface report

This means a relying party gets a single mutual attestation derived from a
single chain of trust spanning CPU TEE → host TDISP root → device IDE
endpoint, rather than stitching two attestations together off-platform.

(Source: NVIDIA Secure AI whitepaper §"TEE-I/O"; PCI-SIG TDISP 1.0 specification.)

### 4.4 IOMMU and PCIe configuration in CC mode

For H100, hardware enforces:

- IOMMU is locked into a configuration where GPU DMA can only touch **Shared
  memory pages** (TDX/SNP "Shared" attribute)
- PCIe access to GPU MMIO BARs is filtered through firewalls — in
  CC-On mode, all out-of-band paths (BMC SMBus, JTAG) are disabled
- The H100 whitepaper (§"Setting up a Confidential Compute Environment"):
  > "During this reset, a memory lock is engaged which blocks access to the
  > GPU's memory until it has been scrubbed (mitigating cold boot attacks).
  > GPU Firmware initiates a scrub of memory and states in registers and
  > SRAMs before the GPU is handed over to the user."

### 4.5 Multi-GPU: Protected PCIe (PPCIE)

For multi-GPU CC deployments, NVIDIA ships a **PPCIE verifier** in
`nvtrust/guest_tools/ppcie-verifier` that does joint attestation across
all GPUs + NVSwitches in the deployment. This becomes load-bearing for
GB200 NVL36/NVL72 deployments where dozens of GPUs share a CVM trust
boundary.

---

## 5. Confidential Compute deployment mechanics

### 5.1 Modes (H100)

Three modes per H100 whitepaper §"Goals for Hopper H100 Confidential
Computing":

- **CC-Off** — "Standard H100 operation. None of the encryption/authentication
  paths are active, and none of the other CC-specific system blocks/firewalls/
  sideband-channels are active."
- **CC-On** — "The H100, with the drivers on the CPU will have fully
  activated all the CC features available; all firewalls are active, and all
  sideband channels have been closed."
- **CC-DevTools** — "the GPU is in full 'CC-On' mode, as described above,
  except for the data paths required to run the developer tools." Performance
  counters are exposed (otherwise they'd enable side-channel inference).

### 5.2 Mode-change is reset-required

Switching modes requires a **Function-Level Reset (FLR)** of the GPU.
From whitepaper §"Steps 1-4":

> "The APIs to enable/disable CC are provided as both in-band PCIe commands
> from the Host or out-of-band BMC commands. A toggle operation requires a
> Function Level Reset (FLR) of the GPU for the mode to take effect. During
> this reset, a memory lock is engaged which blocks access to the GPU's
> memory until it has been scrubbed (mitigating cold boot attacks)."

You **cannot** toggle CC mode per-workload at runtime; the GPU is in one
mode until the next FLR. This is a deployment-scheduling consideration —
mixing CC and non-CC workloads on the same physical GPU requires a reset
gap.

### 5.3 Performance overhead — empirical numbers

**H100 / Hopper (bounce-buffer model):**

From H100 whitepaper §"Performance Samples":
> "CPU-GPU interconnect bandwidth is limited by CPU encryption performance
> to approximately 4 GBytes/sec."

From [arXiv 2409.03992v2 — "Confidential Computing on nVIDIA H100 GPU: A
Performance Benchmark Study"](https://arxiv.org/html/2409.03992v2):

| Workload         | TPS overhead (TEE-on vs off) | QPS overhead | TTFT overhead |
| ---------------- | ---------------------------- | ------------ | ------------- |
| Llama-3.1-8B     | 6.85%                        | 3.22%        | 19.03%        |
| Phi-3-14B-128k   | 4.58%                        | 2.31%        | 18.02%        |
| Llama-3.1-70B    | -0.13% (within noise)        | -0.36%       | -0.41%        |

> "The average overhead is less than 7%" across tested LLM inference workloads.

The pattern: **large models hide the overhead** (compute dominates I/O); small
models pay 5-7% in throughput and 15-20% in first-token latency because the
encryption of CPU↔GPU traffic dominates.

**Blackwell:**
> "Blackwell Confidential Computing delivers nearly identical throughput
> performance as compared to unencrypted modes."
> ([NVIDIA Blackwell architecture page](https://www.nvidia.com/en-us/data-center/technologies/blackwell-architecture/))

This is the headline architectural improvement of TEE-I/O — eliminating
the bounce-buffer overhead.

**Attestation cost:**
> "Attestation adds a one-time startup cost of 1-3 seconds per instance
> provisioning event."
> ([Corvex on B200 CC](https://www.corvex.ai/blog/confidential-computing-meets-nvidia-hgxtm-b200-secure-ai-without-the-performance-trade-off))

### 5.4 Driver requirements

- **H100** — NVIDIA Driver R535.86+ (initial CC GA support), CUDA 12.2 Update 1+
- The driver must be **CC-aware** — pre-CC drivers will not establish the
  SPDM session
- The host requires `nvidia-persistenced` to be running because tearing down
  the driver destroys the SPDM session and requires an FLR to reestablish
  (whitepaper §"Developer Considerations")
- **Blackwell** — newer CUDA + driver stacks (R555+ family) required; check
  Secure AI Compatibility Matrix in NVIDIA docs

### 5.5 Supported host CPU TEEs

NVIDIA H100 (whitepaper §"Confidential Computing – A Feature for Secured Systems"):

> "you must have specific CPU hardware SKUs to enable Confidential Compute
> with NVIDIA's H100:
> - Intel CPUs must support 'Trusted Domain eXtensions' (TDX)
> - AMD CPUs must support 'Secure Encrypted Virtualization with Secure Nested
>   Paging' (SEV-SNP)
> - ARM CPUs must support ARM 'Confidential Compute Architecture' (CCA)"

CCA support is forward-looking — at the time of writing (2026-06), production
deployments are predominantly TDX (Azure, GCP) and SEV-SNP (Azure, GCP, AWS
Nitro adjacent).

---

## 6. Open-standards alignment

### 6.1 IETF RATS (RFC 9334) + EAT (RFC 9711)

NRAS aligns with **RATS Architecture (RFC 9334)**: GPU = Attester producing
Evidence; NRAS = Verifier producing Attestation Result; downstream service =
Relying Party. Evidence is the SPDM-signed `MEASUREMENTS` block + cert chain;
Attestation Result is an **EAT JWT** (RFC 9711) signed by NRAS — JWT is a
special-case of JSON EAT per RFC 9711 §7.

Multi-GPU/NVSwitch responses follow **RATS §A.2.3 "JSON-encoded Detached EAT
Bundle"** — an outer JWT carrying the overall verdict plus per-device detached
EAT bundles (`gpu-0`, `gpu-1`, …, `switch-0`, …).

Standard claims observed in NRAS tokens: `iss`, `jti`, `exp`, `nonce`,
`hwmodel`, `x-nvidia-gpu-driver-version`, `x-nvidia-attestation-detailed-result`.
NVIDIA has not published a stand-alone "NVIDIA EAT profile" RFC as of
2026-06; claim namespace is documented in NRAS API docs.

### 6.3 DMTF SPDM

NVIDIA's evidence layer is **DMTF SPDM 1.1** (Security Protocol and Data
Model). SPDM 1.2/1.3 add CXL-specific extensions but the deployed Hopper
stack uses 1.1. Blackwell with TEE-I/O leans on **SPDM 1.2+ extensions for
TDISP** in coordination with PCI-SIG TDISP 1.0.

### 6.4 DMTF CoRIM (Concise Reference Integrity Manifest)

NVIDIA's RIM bundles are **CoRIM-formatted**. CoRIM is a CBOR-encoded
manifest describing expected component measurements + the signer's identity
+ policy. The RATS-aligned approach is: relying party fetches a CoRIM for a
specific (component, firmware-version) tuple and matches it against the
Evidence presented in the EAT.

### 6.5 TPM 2.0 / TCG alignment

There is **no direct TPM-equivalent** on the GPU. The conceptual mapping:

- **PCR-like measurements** — SPDM `MEASUREMENTS` block (multiple
  indexes, each a hash of a subsystem)
- **EK (Endorsement Key)** equivalent — per-device IK (ECC P-384, fused)
- **AK (Attestation Key)** — derived AK signed by IK, per-reset
- **Quote** — SPDM `MEASUREMENTS` response signed by AK

But the GPU is **not a TPM** — there is no TCG TPM 2.0 command interface, no
PCR extend semantics. Drawing TPM analogies is helpful for intuition but
should not be taken to imply API/format compatibility.

---

## 7. Cryptographic primitives

### 7.1 Asymmetric signing

- **Curve:** NIST P-384 (secp384r1)
- **Algorithm:** ECDSA
- **Hash:** SHA-384

This is enforced for the device identity key (IK), the per-reset Attestation
Key (AK), and intermediate certs. NVIDIA's choice of P-384 aligns with
CNSA Suite 2.0 (NSA's post-CRQC suite recommends LMS/XMSS but P-384 remains
acceptable for transitional deployments).

NRAS's L3 ephemeral signing cert is also P-384 / SHA-384 per the EAT JWS
signature. The JWT header `alg` value is `ES384`.

### 7.2 Symmetric encryption (PCIe traffic in CC-On)

- **Cipher:** AES-256-GCM (256-bit key, 96-bit IV per AES-GCM spec)
- **Mode:** Rolling-IV; key rotated when IV counter approaches
  exhaustion ((2^96)−1)
- **Integrity:** GCM AuthTag

H100 whitepaper §"In-Band Attacks":
> "NVIDIA has built-in encryption and decryption engines across all its
> ingress/egress paths; these engines use 256-bit AES-GCM encryption. Any
> request or response which enters or exits the GPU must be encrypted."

> "In AES-GCM, a 96bit IV is required... The total number of 'uniqueness' is
> limited to (2^96)-1, after which an IV is considered 'exhausted' and the
> encryption key must be rotated out and replaced."

This applies to:
- The bounce-buffer ↔ GPU traffic
- All command buffers and CUDA kernels crossing PCIe
- Synchronization primitives and driver metadata

### 7.3 SPDM session key establishment

- **Protocol:** SPDM 1.1 `KEY_EXCHANGE` → **ECDHE on P-384**
- **PRF:** HKDF-SHA-384
- **Session keys:** Per-direction (req→rsp, rsp→req) AES-GCM keys derived
  from the ECDHE shared secret

Per the [MLSys 2026 paper](https://arxiv.org/html/2507.02770v2), the SPDM
master secret is then expanded into **46 distinct keys** for different
intra-GPU channels:

- **GSP keys (6)** — `gsp_cpu_locked_rpc`, `cpu_gsp_dma`, and others
  protecting RPC + memory transfers between the driver and GSP
- **SEC2 keys (6)** — Workload launch and scrubber channel protection
- **Copy Engine keys (32)** — 8 LCEs (LWcopy/Crypto Engines) × 2 directions
  × 2 modes (encrypted-DMA, encrypted-data)

The paper notes: "separate session keys, derived from the master secret,
protect data transmitted through the bi-directional RPC and DMA channels."

### 7.3a Side channels identified (MLSys 2026)

The MLSys 2026 paper (PSIRT-disclosed before publication) identified:

- **RPC metadata leakage** — "the physical address table, queue headers, and
  queue element headers remain in plaintext" → adversaries can locate queue
  pointers and infer RPC invocation patterns
- **Timing channel in CPU↔GSP transfers** — bimodal latency for transfers
  >256 bytes (4KB transfers show "significant time shift compared to
  smaller ones")
- **UVM protection gaps** — "SEC2's command queue structures, specifically
  GPPUT, GPFIFO, and tracking semaphores, remain unprotected in shared memory"
- **BAR0 residual exposure** — firewall reduces non-zero fields by 99.78%
  in CC-On (7.94% → 0.02%) but ~1042 fields still return non-zero

These are **architectural side channels**, not direct key-extraction primitives.
For Tide's threat model they matter only insofar as a co-tenant adversary
shares the host; most deployments put the host inside the trust boundary.

### 7.4 NVLink encryption (Blackwell-specific)

NVLink Gen5 (Blackwell) carries **link-layer AES-GCM** for confidentiality
+ integrity. Detailed crypto parameters are not publicly published; the
NVIDIA Secure AI whitepaper confirms NVLink frames are encrypted and
authenticated in CC mode but does not specify the link-layer key
provisioning protocol in public docs. (This is a place where the spec
boundary noted in §0 applies.)

### 7.5 Key wrapping / key management

- Long-term keys: Fused into silicon, never exported
- Per-reset keys (AK): Derived in hardware, ephemeral, never exported
- Session keys: HKDF-derived, lifetime = SPDM session, never exported
- All key material destroyed on FLR / VM teardown (whitepaper
  §"Availability")

---

## 8. On-chain verification cost (EVM)

> **This is novel territory.** As of 2026-06, there is no widely-deployed,
> production on-chain NVIDIA attestation verifier. The estimate below is
> derived from the cryptographic primitive cost and the structure of the
> verification, not from a deployed verifier benchmark.

### 8.1 Cost breakdown per attestation

To verify an NVIDIA GPU attestation on-chain (EVM), a verifier must do:

| Step                                               | Primitive            | EVM cost (rough)         |
| -------------------------------------------------- | -------------------- | ------------------------ |
| Verify NRAS EAT JWT signature (`ES384`)            | ECDSA-P384 verify    | NOT a precompile         |
| Verify L3 → NRAS-signing-CA chain                  | ECDSA-P384 × N certs | NOT a precompile         |
| OR (Local Verifier path) verify per-device cert    | ECDSA-P384 × 2-3     | NOT a precompile         |
| Verify SPDM MEASUREMENTS signature (AK)            | ECDSA-P384           | NOT a precompile         |
| Validate nonce / freshness                         | Storage read/write   | ~20K-40K gas             |
| Verify RIM bundle signature                        | ECDSA-P384           | NOT a precompile         |
| Compare measurements against committed RIM hashes  | SHA-256/SHA-384      | SHA-256 precompile = 60+12/word; SHA-384 NOT a precompile |

### 8.2 The P-384 problem

EVM's precompiles cover P-256 (EIP-7212 / EIP-7951 — `secp256r1` ecRecover
analog) and `secp256k1` (native ecrecover). **There is no P-384 precompile.**
P-384 verification in pure Solidity is expensive because 384-bit math
does not fit in the EVM's 256-bit words — the implementor must build the
secp384r1 base field, generator-order field, curve arithmetic, and ECDSA
verify themselves.

Published gas-cost data points for on-chain P-384 verify:

- [Estonian e-ID Solidity P-384 verify](https://github.com/LogvinovLeon/estid-sig)
  — naive implementation ~**500M gas**; with precomputation + assembly
  optimization reduced to ~**20M gas** per verify
- For comparison, secp256r1 with EIP-7951 precompile is ~3,450 gas; secp256k1
  via `ecrecover` is 3,000 gas; pure-Solidity secp256r1 is ~2-5M gas

This makes on-chain NRAS-EAT verification (which requires multiple P-384
verifies — leaf cert, intermediate, root, AK signature on evidence,
RIM signature) prohibitively expensive on Ethereum L1 (>100M gas per full
attestation) without optimization. Tractable on L2s with batch verification
or with a custom P-384 precompile.

Sei EVM (chain ID 1329) inherits the standard precompile set. As of 2026
no P-384 precompile is upstream in `go-ethereum`; adding one to Sei would
be a chain-protocol change.

### 8.3 Strategies the ecosystem is using

1. **NRAS-proxied** (cheapest) — A trusted relayer verifies the EAT
   off-chain, then submits a signed (secp256k1) attestation-confirmation
   on-chain. The trust set adds the relayer. Used by some "verifiable
   inference" providers.

2. **ZK proof of attestation** — Generate a SNARK/STARK proving "I have a
   valid NRAS EAT signed by a key chained to NVIDIA Root CA, measuring this
   workload, dated within N hours." On-chain verifies only the SNARK.
   Constraints: P-384 ECDSA inside a circuit is expensive but feasible
   (~10-30M constraints for one verify); proving time is the bottleneck.

3. **Native P-384 verifier in Solidity** — SCL or equivalent; full chain
   verification on-chain. ~3-5M gas per attestation realistic.

4. **L2 with custom precompile** — A rollup adds a P-384 verify precompile
   (cf. Phala / Automata's roadmaps). Cost drops to tens of thousands of
   gas; the trust set adds the rollup operator + sequencer.

### 8.4 Prior art references

- **Phala Network** — operates a GPU TEE coordinator that performs NRAS
  verification off-chain and binds results into Phala worker state (see
  [Phala GPU TEE Deep Dive](https://phala.com/posts/Phala-GPU-TEE-Deep-Dive)).
  Phala explicitly notes the off-chain verification path.
- **Marlin Protocol** — Intel TDX + NVIDIA dual-attestation flow for
  Oyster CVMs; verification path is currently relayer-mediated.
- **Automata Network** — has explored on-chain DCAP (Intel TDX) verifiers;
  the NVIDIA leg would follow a similar pattern with the added P-384 cost.

### 8.5 Tide-specific considerations

For a Tide on-chain NVIDIA verifier, practical options ranked:

1. **MVP — Relayer-proxy**: Tide Operator verifies NRAS EAT off-chain (Go),
   submits a verified-attestation event signed by a trusted operator key.
   Lowest cost, smallest trust hit (operator already trusted).
2. **Mid-term — Solidity P-384**: Verifier reads submitted EAT JWT, validates
   chain to a committed root CA hash. ~100M+ gas full chain; appropriate
   only for highest-value jobs.
3. **Long-term — SNARK-proven**: Proving service produces a Groth16/Plonk
   proof; on-chain verify ~200K gas. Requires a prover (Tide-hosted or
   marketplace).

---

## 9. References

### Primary (NVIDIA)

- [Confidential Compute on NVIDIA Hopper H100 (WP-11459-001, Jul 2023)](https://images.nvidia.com/aem-dam/en-zz/Solutions/data-center/HCC-Whitepaper-v1.0.pdf) — Canonical Hopper CC architecture.
- [NVIDIA Secure AI with Blackwell and Hopper (WP-12554-001 v1.3, Aug 2025)](https://docs.nvidia.com/nvidia-secure-ai-with-blackwell-and-hopper-gpus-whitepaper.pdf) — Blackwell TEE-I/O / NVLink encryption deltas.
- [NVIDIA Confidential Computing docs index](https://docs.nvidia.com/confidential-computing/)
- [NVIDIA Technical Blog — Confidential Computing on H100](https://developer.nvidia.com/blog/confidential-computing-on-h100-gpus-for-secure-and-trustworthy-ai/)
- [nvtrust GitHub (Apache 2.0)](https://github.com/NVIDIA/nvtrust) — Python verifier, attestation SDK, PPCIE verifier, host tools.
- [NVIDIA attestation-sdk (C++)](https://github.com/NVIDIA/attestation-sdk) — Long-term replacement for Python local verifier.
- [NRAS docs portal](https://docs.attestation.nvidia.com/NRAS/nras_introduction.html), [NRAS `/v1/attest/gpu` API reference](https://docs.api.nvidia.com/attestation/reference/attestgpu)
- [NVIDIA forum — Attestation report measurements](https://forums.developer.nvidia.com/t/attestation-report-measurements/322575) — NVIDIA's stated non-disclosure boundary.

### Academic / peer-reviewed

- ["Creating the First Confidential GPUs" — ACM Queue](https://queue.acm.org/detail.cfm?id=3623391) (Nertney et al.) — Peer-reviewed Hopper CC architecture.
- ["NVIDIA GPU Confidential Computing Demystified", MLSys 2026, arXiv 2507.02770](https://arxiv.org/abs/2507.02770) (Gu et al.) — Security analysis: 64 records, 46 keys, CEC EROT → FSP → GSP → SEC2 boot chain, BAR0 firewall, side channels (PSIRT-disclosed).
- ["Confidential Computing on nVIDIA H100 GPU: A Performance Benchmark Study", arXiv 2409.03992v2](https://arxiv.org/html/2409.03992v2) — LLM benchmark numbers used in §5.3.

### Standards

- [RFC 9334 — RATS Architecture](https://datatracker.ietf.org/doc/rfc9334/) (roles + Detached EAT Bundle §A.2.3 used by multi-GPU NRAS)
- RFC 9711 — Entity Attestation Token (EAT)
- DMTF SPDM 1.1 (DSP0274) — Underlying attestation protocol
- DMTF CoRIM — Reference integrity manifest used by RIM service
- PCI-SIG TDISP 1.0 + IDE — Blackwell TEE-I/O link-layer protection

### Third-party / ecosystem

- [Intel Trust Authority — GPU Remote Attestation](https://docs.trustauthority.intel.com/main/articles/articles/ita/concept-gpu-attestation.html) — Joint Intel TDX + NVIDIA `/appraisal/v2/attest`.
- [Phala GPU TEE Deep Dive](https://phala.com/posts/Phala-GPU-TEE-Deep-Dive), [Phala H100 benchmark](https://phala.com/posts/confidential-computing-on-nvidia-h100-gpu-a-performance-benchmark-study) — Verifiable-inference platform view.
- [Corvex — HGX B200 CC](https://www.corvex.ai/blog/confidential-computing-meets-nvidia-hgxtm-b200-secure-ai-without-the-performance-trade-off) — Production B200 deployment ("1-3s attestation cost").
- [Estonian e-ID Solidity P-384 verify](https://github.com/LogvinovLeon/estid-sig) — On-chain P-384 gas-cost reference.

---

## Load-bearing claims

The 10 highest-leverage facts in this document (each anchored to a primary source):

1. **NVIDIA GPU attestation evidence is carried in DMTF SPDM 1.1
   `MEASUREMENTS` response messages signed by an on-die Attestation Key
   (AK) derived at each chip reset.** (ACM Queue; H100 whitepaper)

2. **The signing chain is NVIDIA Root CA → intermediate(s) → per-device
   Identity Key (IK, ECC P-384, fused at manufacture, private destroyed)
   → per-reset AK signing the SPDM evidence.** (H100 whitepaper §"Attesting
   the GPU")

3. **NRAS issues a signed EAT (Entity Attestation Token) in JWT form with
   `alg=ES384`, signed by an ephemeral L3 cert with 24-hour TTL; multi-GPU
   results use RATS §A.2.3 Detached EAT Bundles.** (H100 whitepaper steps
   20-21; NRAS docs)

4. **Local verification is possible via `nvtrust/guest_tools/gpu_verifiers/
   local_gpu_verifier` (Python, Apache 2.0); the Python verifier is
   deprecated in 2026 in favor of the C++ `NVIDIA/attestation-sdk`.**
   (nvtrust README; deprecation notice)

5. **H100 has three CC modes — CC-Off, CC-On, CC-DevTools — and switching
   modes requires a Function-Level Reset (FLR); CC mode cannot be toggled
   per-workload at runtime.** (H100 whitepaper §"Steps 1-4")

6. **In CC-On, all PCIe traffic between the CVM and the GPU is AES-256-GCM
   encrypted with 96-bit rolling IVs; keys rotate when the IV counter
   approaches `(2^96)−1`.** (H100 whitepaper §"In-Band Attacks")

7. **Hopper CPU-GPU encrypted interconnect is bottlenecked at ~4 GB/s due
   to CPU encryption performance, producing 5-7% LLM inference TPS
   overhead and 15-20% TTFT overhead on smaller models; Blackwell's
   TEE-I/O eliminates this via inline IDE/TDISP link-layer encryption.**
   (H100 whitepaper §"Performance Samples"; arXiv 2409.03992v2; NVIDIA
   Blackwell architecture page)

8. **Joint CPU+GPU attestation on Hopper is software-bound via SPDM session
   keys hashed into the CPU TEE quote's `REPORT_DATA`; Blackwell binds
   them in hardware via PCI-SIG TDISP/IDE between the CPU TEE Security
   Manager and the GPU.** (H100 whitepaper §"Secure Session Establishment";
   Secure AI whitepaper §"TEE-I/O")

9. **NVIDIA explicitly withholds per-index SPDM measurement semantics — the
   relying party is expected to consume verifier appraisal results, not
   parse measurement indices directly.** (NVIDIA developer forum response,
   "Attestation report measurements")

10. **There is no EVM P-384 precompile; an attestation involves multiple
    P-384 verifies (leaf cert chain + AK signature + RIM signature).
    Published Solidity P-384 verify costs are ~20M gas per verify after
    heavy optimization (~500M naive). On-chain NRAS-EAT verification today
    requires either (a) ~100M+ gas via Solidity, (b) ZK-proven attestation
    reducing to a SNARK verify (~200K gas), or (c) a trusted relayer that
    verifies off-chain and signs (secp256k1) on-chain.**
    (EVM precompile inventory; Estonian e-ID benchmarks; ecosystem prior
    art — Phala / Marlin / Intel Trust Authority)

11. **The H100 attestation report contains 64 structured measurement
    records; the internal boot chain-of-trust is CEC EROT → FSP → GSP →
    SEC2; and the SPDM master secret is expanded into 46 derived keys
    protecting GSP RPC, SEC2 channels, and 8 LCE copy engines.**
    (MLSys 2026 paper, Gu et al., *NVIDIA GPU Confidential Computing
    Demystified*)
