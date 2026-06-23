# Data products & contracts kit

## 1. What this concern is

The **data product** is the central unit — the architectural quantum (code + data/metadata + infra) a domain owns and publishes; the **data contract** is its producer↔consumer interface (schema + semantics + SLOs + ports). The generic single-org habit trusts a product because the producing domain's pipeline is trusted. Cross-org you cannot trust the pipeline, so the product must be a **verifiable certificate** that carries its own proof and travels *with* the data. *Cited:* `sources.md` §data-product, §data-contracts; profile §2.

## 2. The pattern (how to do it)

- **Build the product as the architectural quantum.** Independently deployable, high cohesion: its pipelines/APIs/policy (code) + analytical data + metadata + the infra it needs. Owned end-to-end by the domain team. *Cited:* `sources.md` §data-product.
- **Make it satisfy the six usability attributes:** **discoverable** (catalog/metadata), **addressable** (a unique address under a global convention), **trustworthy** (published SLOs on quality/integrity), **self-describing** (schema + semantics), **interoperable** (global standards), **secure** (global access control). *(These six attributes are the canon; "DATSIS"/the port taxonomy is a mnemonic — see `sources.md` Verify-before-print.)* *Cited:* `sources.md` §data-product.
- **Express the contract in a real standard — ODCS v3.x.** Open Data Contract Standard (Bitol/LF AI & Data): fundamentals + schema (properties/relationships) + **data-quality rules** + **SLA/SLOs** + servers + team + support. Validate with the Data Contract CLI. **Do not use the deprecated standalone "Data Contract Specification"** for new work — target ODCS (a contract describes a dataset/API; a *product* may bundle several contracts). *Cited:* `sources.md` §data-contracts.
- **Cross-org: the product is a verifiable certificate.** Layer the contract with proof the consumer can check without trusting the producer: a **content-address** (integrity, partial re-verification), a **typed signature + issuer identity** chaining to signed source state (provenance), a **transparency-log/anchor** (tamper-evidence), an **availability commitment** (bought, not assumed), and — only when proving a *derivation* — a **pluggable correctness proof** (succinct proof for deterministic work, attested execution for heavy inference). Carry a queryable **verification class** (`signed-at-source | derived | asserted`). Small facts need only a provenance reference; reserve the expensive layers for large context or proven derivations. **Authenticity ≠ correctness ≠ truth.** *Cited:* profile §2; `sources.md` §observability.

## 3. Anti-patterns / failure modes

- **Trusting the pipeline, not the product (cross-org).** Cue: a cross-org data share where the consumer trusts the producer's pipeline output as-is. Rewrite: a verifiable certificate (content-address + signature + provenance + verification class) the consumer re-checks. *Cited:* profile §2.
- **The deprecated Data Contract Specification.** Cue: a new contract authored against datacontract.com's standalone spec. Rewrite: ODCS v3.x (the standalone spec's support ends end-2026). *Cited:* `sources.md` §data-contracts.
- **A "data product" that's really a pipeline or a table.** Cue: a dataset with no contract, no SLOs, no owner, not independently deployable. Rewrite: the architectural quantum — code + data + infra + a contract + an accountable owner.
- **Missing usability attributes.** Cue: not discoverable (no catalog entry), not addressable (no stable address), not self-describing (no schema/semantics), no published SLOs. Rewrite: satisfy all six attributes before calling it a product.
- **Bonding/SLO-ing a claim that isn't re-checkable.** Cue: a correctness guarantee on a subjective/judgment output. Rewrite: see `kit-data-quality-observability` — only Class-I claims carry a correctness bond; judgment is a signed opinion + consumer policy.

## 4. Review cues

- **Dimension 2 (data-product design & contracts):** the product is the architectural quantum with all six usability attributes; the contract is ODCS v3.x (not the deprecated spec) with schema + semantics + SLOs + ports; cross-org, the product is a verifiable certificate (content-address + signature + provenance + verification class), not a trusted-pipeline output. *Basis:* `sources.md` §data-product, §data-contracts; profile §2.
- **Dimension 5 (data quality & observability):** the verification class is set correctly and only re-checkable claims carry a correctness guarantee (authenticity ≠ correctness). *Basis:* profile §5.

## 5. One-way doors in this concern

- **A published data-product contract + its ports** (schema, semantics, address, output-port formats) — consumers build against them; a breaking change is a versioned migration, not an edit. Flag.
- **The verification/provenance scheme** (the certificate format, the signature/typed-data layout) — outstanding products are signed against it; changing it invalidates them. Flag (coordinate with `/evm` for any on-chain typehash).
