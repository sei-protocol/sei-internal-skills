# Method — the attestation design/review protocol

The reusable, platform-agnostic procedure. The SKILL.md gives the five steps; this file is the detail, the RATS vocabulary, the Sei cost ranking, and the cross-cutting verifier-policy dimensions every kit fills in. It is **vendor-agnostic**: per-platform specifics live in `kit-<platform>.md`, which cites public primary sources (vendor specs, RFCs, `sei-chain`) and is self-contained.

## Step 1 — Load the deployment profile (first overlay)

Before designing, read `tee-profile.md` (the Sei overlay) and the repo's governing docs. The profile is the higher-priority overlay; the kit fills what the profile is silent on. If the profile is absent, design on the method + first principles and **flag the missing-profile gap** (you lose the Sei economics + validator-as-host + registry-as-RVP layer) — do not refuse.

## Step 2 — Scope the attestation problem (the hard gate)

Name four things before any design (Rule 1 of the spine):

- **Platform(s) in scope.** Which TEE — and is it forced (existing AWS/harbor posture → Nitro) or open?
- **The claim to prove.** Exactly one or more of: **binary identity** (the code/measurement is the approved build), **data integrity** (the data was produced inside the boundary), **key binding** (a key exists only inside the boundary), **freshness** (this attestation is live, not replayed). The claim drives every later check.
- **The trust model.** What the TEE protects against, and **what is explicitly out of scope.** The dominant error is leaving "out of scope" unstated. Always evaluate the **validator-as-host** case: a platform whose trust model assumes the host is honest (Nitro assumes the AWS host) does not defend against a relying party who *is* the host operator.
- **The RATS roles** (RFC 9334): **Attester** (the TEE producing Evidence), **Verifier** (appraises Evidence against policy — an on-chain contract or off-chain service), **Relying Party** (consumes the Verifier's result to gate a decision), **Endorser** (vouches for the platform — the silicon/cloud vendor PKI), **Reference-Value-Provider** (supplies the expected measurements — governance / an on-chain registry).

**Note on output format:** EAT (RFC 9711) is the **Verifier's *output*** format, not the on-chain *input* — on-chain Verifiers consume vendor-native Evidence (SNP report / TDX quote / Nitro COSE_Sign1 bytes) directly, and EAT enters only at a multi-vendor verifier-result abstraction layer (RFC 9711; `trusted-execution-on-sei.md` §open-standards, claim 7). Feeding EAT to an on-chain verifier is a category error.

## Step 3 — Load the platform kit(s)

Load `kit-<platform>.md` for each in-scope platform. The kit supplies the Evidence format, identity/measurement fields, the per-vendor verifier-policy specifics (the §4 fill-ins for the dimensions below), the on-chain cost/path, and the key-release pattern — each citing primary sources (vendor spec, RFC, `sei-chain`). No kit → design against the method + the vendor's primary specs/RFCs + first principles, and flag the missing-kit gap. **Never assert a per-vendor specific from memory** (Rule 3).

## Step 4 — Apply the verifier-policy checklist

These are the cross-cutting verifier-policy dimensions — the things a verifier MUST do that **vendors do not enforce automatically**. They are vendor-agnostic *rules*; each kit's §4 fills in the per-vendor *specifics* (the field, offset, bit, version set). Walk every dimension against the design; for each, pull the platform specific from the kit and cite it.

| id | dimension | the rule (vendor-agnostic) | consequence if missed | per-vendor specifics |
|----|-----------|----------------------------|------------------------|----------------------|
| VP1 | **Freshness binding** | Every attestation carries a verifier-issued nonce in the platform's freshness channel. | Old attestations **replay**. | kit §4 (e.g. Nitro `nonce`; AMD `REPORT_DATA` offset `0x050`) |
| VP2 | **Binary binding** | The measurement field MUST match a governance-approved reference value. | Unapproved/forged code passes as the approved build. | kit §3 + §4 (e.g. Nitro PCR0/1/2; TDX `MRTD`+`RTMR[0..3]`) |
| VP3 | **Debug-mode rejection** | Reject debug/console-attached builds (memory is inspectable). | Secrets/keys inside a "TEE" are readable by the operator. | kit §4 (e.g. Nitro all-zero PCRs; AMD policy bit 19; Intel `ATTRIBUTES.DEBUG`) |
| VP4 | **Anti-rollback / TCB policy** | Enforce `reported_TCB ≥ minimum_acceptable`; reject revoked TCB; explicit policy on out-of-date. Vendors sign reports for *any* historical TCB. | A known-vulnerable TCB passes. | kit §4 (e.g. AMD `REPORTED_TCB`; Intel `tcbStatus`; Nitro leaf validity vs doc `timestamp`) |
| VP5 | **Generation / cert-chain selection** | Fetch the right Endorser cert for the chip generation + TCB; validate the full chain to a pinned root; respect chain order. | "Valid signature, lies about platform" — wrong-cert acceptance. | kit §4 (e.g. AMD VCEK per-chip+TCB; Intel 3-layer PCK; Nitro `cabundle` order, pinned Root-G1) |
| VP6 | **Key isolation** | Signing keys never exist outside the boundary; long-lived keys gated by attested storage, ephemeral keys rebound per enclave start. | Key leaks defeat the whole scheme. | kit §6 (e.g. Nitro KMS `RecipientAttestation` condition keys) |
| VP7 | **Revocation** | A compromised measurement value can be revoked immediately; the reference-value registry supports governance-driven revocation. | A known-bad image keeps passing after disclosure. | profile (on-chain registry) + kit §3 |
| VP8 | **Joint-attester binding** | When two TEEs co-attest (CPU+GPU), the verifier checks the *binding* between them, not the two reports independently. | A real GPU report + an unrelated CPU report passes. | kit §4 (NVIDIA: SPDM session key in CPU `REPORT_DATA` on Hopper; TDISP/IDE on Blackwell) |
| VP9 | **Verifier-policy separation** | Parse vendor Evidence into a normalized claim set, then apply policy (acceptable measurements, min TCB, revoked images) as a *separate* layer. Don't hard-code vendor parsing into policy. | *(policy hygiene — tier 3)* Policy can't generalize; each vendor change breaks the policy layer. | method (this rule) + per-vendor parse in kit §2 |
| VP10 | **Side-channel advisory handling** | Surface vendor advisory IDs to the relying-party policy layer; don't silently accept out-of-date TCB. | *(policy hygiene — tier 3)* The SGAxe failure mode — accepting a TCB with a live exploit. | kit §4 (Intel `advisoryIDs`) |
| VP11 | **Known-CVE mitigation bits** | Require platform-info bits that prove a named-CVE mitigation TCB was applied, for chips in the affected window. | *(policy hygiene — tier 3)* A chip in the vulnerable window passes unmitigated. | kit §4 (AMD BadRAM `PLATFORM_INFO.ALIAS_CHECK_COMPLETE` for AMD-SB-3015) |
| VP12 | **Host-controlled-but-signed fields** | Fields the host supplies that are signed but **not measured** need a *separate* trust assumption; they are evidence of host input, not enclave behavior. | Policy gated on attacker-controllable bytes that look attested. | kit §4 (Nitro `user_data`; AMD `HOST_DATA`) |
| VP13 | **Quote/report version pinning** | Pin acceptable attestation format versions; reject downgrade. | Version-confusion verifier bugs (a known DCAP bug class). | kit §2/§4 (per-vendor version set) |
| VP14 | **Privacy / device fingerprinting** | Raw on-chain attestations expose permanent device IDs; prefer fleet-scoped credentials or ZK that hides device-unique fields. | *(policy hygiene — tier 3)* A permanent ledger-wide device fingerprint for validator-as-attester patterns. | kit §4 (AMD `CHIP_ID`/VLEK-vs-VCEK; Nitro `module_id`; NVIDIA PDI) |
| VP15 | **Registry / governance integrity** | The reference-value registry MUST have multisig, time-locks, transparency (CoRIM/public RVP), and an emergency revocation path. | **Highest-leverage attack** — compromise the registry and mint valid attestations for any payload. | profile (registry/governance as RVP) |
| VP16 | **Cross-vendor trust-set deltas** | A multi-vendor verifier surfaces *which* trust set applies per attestation (silicon vendor vs cloud hypervisor vs NRAS); `tee_type` is not a fungible switch. | The relying party misreads the security posture across vendors. | kit §1 (per-vendor Endorser/trust root) + method |

## The Sei on-chain verification cost ranking (decision input)

Sei EVM has a P-256 precompile at `address(0x1011)` charging `300 gas/byte × 160 bytes = 48,000 gas per verify` (cheaper than Solidity P-256 ~200k, above EIP-7951's flat ~6k). Per-attestation, cold (from `bdchatham-designs/designs/sei-agentic-mesh/research/tee/trusted-execution-on-sei.md` §decision-driver):

| Attester | Scheme | Sei EVM cost (cold) | Strategy |
|---|---|---|---|
| **AMD SEV-SNP** | P-384 single verify | ~1.5–2M gas Solidity, ~250k via ZK | Cheapest direct on-chain |
| **Intel SGX / TDX (DCAP)** | P-256, 5–8 verifies | ~3.5–4M gas via P256VERIFY | Competitive; multi-cert overhead dominates |
| **AWS Nitro** | P-384, COSE_Sign1 | <70M cold / <20M warm, OR ~3k amortized | Marlin Oyster (verify-once, then secp256k1 `ecrecover`) |
| **NVIDIA CC** | P-384, multi-verify | ~100M+ direct, ~200k ZK, ~3k relayer | ZK-proven attestation or trusted relayer only |

**Gas is one input, not the decision.** Trust-model fit (validator-as-host, fingerprint exposure, ecosystem maturity, operational footprint) often dominates. State the ranking; don't let it pre-empt the trust-model analysis (Rule 1).

## Step 5 — Severity model

Rank every finding:

1. **Attestation-defeating + registry integrity** (correctness). An attacker passes a bad attestation, or the scheme's root of trust is compromised: replay (VP1), forged identity (VP2 unbound, VP3 debug accepted, VP4 rollback, VP5 wrong cert, VP13 downgrade, VP8 binding skipped), key leak (VP6), registry/governance compromise (VP15). Always wins.
2. **Trust-model misrepresentation.** The design *claims* a security property it doesn't have: validator-as-host violated (Rule 2), cross-vendor trust-delta hidden (VP16), host-controlled-but-signed field gated without a separate assumption (VP12), revocation path absent (VP7).
3. **Policy hygiene.** Important but not scheme-defeating on its own: advisory surfacing (VP10), known-CVE mitigation bits (VP11), privacy/fingerprint exposure (VP14), verifier-policy-separation as a maintainability concern (VP9). Bundle these; never lead with them.

## Citation and anti-fabrication discipline (Rule 3)

Every load-bearing vendor claim cites the kit (which cites a vendor spec, an RFC, or `sei-chain` source). A wrong, falsifiable detail (offset, register, bit, version, cert order) asserted from memory is worse than none — it ships into a verifier and breaks it. If the kit lacks the detail and you can't cite a primary source, say so and flag the gap; never fabricate the offset to satisfy a "show me the field" challenge.

## Trust-model-honesty discipline

Every design states what the TEE does **not** cover. On a design with no real verifier-policy gap, say "no attestation-defeating gap — these dimensions vetted" and list what you checked; don't manufacture findings. The cost of a padded finding is that the next real bypass gets ignored.
