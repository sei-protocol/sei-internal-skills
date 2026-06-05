# design/

Durable design reference kept in the repo — research corpora and notable outputs from skill-driven design sessions. This is reference material, not an active design pipeline; design work itself flows through the `/coral`, `/council`, `/cross-review`, and `/design` skills.

## Layout

```
design/
├── research/
│   └── tee/                  # Trusted Execution Environment research corpus
│       ├── aws-nitro-enclaves.md
│       ├── intel-sgx-tdx.md
│       ├── amd-sev-snp.md
│       ├── nvidia-cc.md
│       ├── tpm-2.0-open-standards.md
│       └── trusted-execution-on-sei.md
└── agents/                   # Outputs from a multi-specialist coral exercise
    ├── cilium-specialist-scope.md
    └── cilium-scope-round1/   # Per-specialist scoping inputs
```

## What's here

- **`research/tee/`** — a standalone TEE/attestation research corpus (AWS Nitro, Intel SGX/TDX, AMD SEV-SNP, NVIDIA confidential compute, TPM 2.0) feeding the `tee-specialist` agent's persona. Portable reference, useful to any Sei-adjacent TEE work.
- **`agents/`** — the output of a `/coral` exercise scoping a Cilium specialist persona: the synthesized scope doc plus the per-specialist round-1 inputs it was distilled from. Kept as a worked example of multi-specialist design.

New design docs produced by `/design` land under `docs/designs/` (the skill's default), not here.
