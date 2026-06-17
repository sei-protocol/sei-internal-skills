# TEE research corpus

Ground-truth, source-cited reference material on Trusted Execution Environment attestation, organized per vendor + cross-cutting standards. Lives alongside (and is referenced by) the `tee-specialist` agent at `.claude/agents/tee-specialist.md`.

## Documents

| Document | Scope |
|---|---|
| [`amd-sev-snp.md`](amd-sev-snp.md) | AMD SEV-SNP `ATTESTATION_REPORT` (byte-by-byte), ARK→ASK→VCEK/VLEK signing chain, generation differences (Milan/Genoa/Bergamo/Turin), on-chain verification cost |
| [`intel-sgx-tdx.md`](intel-sgx-tdx.md) | Intel SGX `EREPORT` + TDX `TDREPORT` verbatim, DCAP flow, MRENCLAVE vs MRTD vs RTMR semantics, EPID deprecation, Sei P256VERIFY economics |
| [`nvidia-cc.md`](nvidia-cc.md) | NVIDIA H100 / Blackwell confidential compute, SPDM 1.1 measurements, NRAS architecture, joint CPU+GPU attestation, performance overheads |
| [`aws-nitro-enclaves.md`](aws-nitro-enclaves.md) | Nitro attestation CDDL, COSE_Sign1 wrapping, PCR semantics, KMS `kms:RecipientAttestation:*` condition keys, Marlin Oyster amortization |
| [`tpm-2.0-open-standards.md`](tpm-2.0-open-standards.md) | TPM 2.0 architecture + PCRs, IETF RATS (RFC 9334), EAT (RFC 9711), TCG attestation model, CCEL, DICE/DPE, SPDM, cross-vendor mapping |
| [`trusted-execution-on-sei.md`](trusted-execution-on-sei.md) | Sei-domain framing: TEE applications fitting Sei's threat model, per-vendor fit on Sei, trust roots, Tide-specific patterns |
| [`automata-onchain-attestation.md`](automata-onchain-attestation.md) | `/research` prior art: Automata Network's on-chain DCAP verifier (V3/V4/V5 contracts, on-chain PCCS), the zkVM gas table (RiscZero/SP1), SEV-SNP-via-zkVM, Trail-of-Bits audit, per-chain-PCCS + not-on-Sei caveats |

## How to read

- **`trusted-execution-on-sei.md` is the integrative entry point.** Read it first if you're trying to decide which TEE for which Sei application — it ties the per-vendor research to concrete Sei threat models.
- **The per-vendor docs are authoritative reference material.** Each has a "Load-bearing claims" section at the end with the 5–11 highest-leverage facts; the body delivers the empirical grounding (verbatim spec excerpts, signing chains, cryptographic primitives, on-chain verification cost).
- **The TPM 2.0 + open standards doc covers cross-cutting vocabulary.** Use the RATS roles (Attester / Verifier / Relying Party / Endorser / Reference Value Provider) when reasoning across vendors — it's how the cross-vendor design decisions get clean.

## Provenance

- Authored via parallel WebFetch/WebSearch research streams, one per vendor + standards, with write-to-disk-early discipline to survive network outages.
- ~27,000 words total. Every load-bearing claim cites a primary source (vendor spec PDF, IETF RFC, GitHub reference implementation, academic paper, or vendor blog).
- Cross-reviewed by the Coral team (see PR sei-protocol/Tide#101).
- Closes sei-protocol/Tide#101.
