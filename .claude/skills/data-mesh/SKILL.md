---
name: data-mesh
category: data-architecture
model: claude-opus-4-8
description: "Use when designing or reviewing data architecture — domain decomposition, data products & contracts, federated governance, self-serve data platforms, data quality/observability — especially federated, verifiable data sharing across organizations: '/data-mesh', 'is a data mesh right for us', 'design this data product/contract', 'design cross-org data governance', 'review this data architecture'. A citable corpus (Dehghani's four principles, ODCS contracts, OpenLineage, the observability pillars) + an always-first profile encoding the cross-org / no-trusted-operator use-case (verifiable data products, federated computational governance, provenance-as-circuit-breaker, claim-class quality) + pluggable kits. Backs the data-platform-architect agent. NOT product→architecture (product-engineer); NOT the infra it runs on (platform-engineer); NOT the telemetry stack (observability agents); NOT building the gating/bonding contracts (solidity-developer). Designs the data architecture; doesn't run the platform."
---

# Data Mesh

Design and review **data architecture** — domain decomposition, data products and contracts, federated governance, self-serve platforms, and data quality/observability — with **federated, verifiable data sharing across organizations** as the core use-case. A *reference/technique* skill with a discipline spine. It is the operating manual for the `data-platform-architect` agent and is directly invocable (`/data-mesh <target>`).

## Why this skill exists

A capable model knows the Data Mesh literature. The skill's job is two things that literature doesn't give you. First, the **citable corpus** — the precise canon (Dehghani's four principles, the data-product attributes, ODCS contracts, OpenLineage, the observability pillars) with currency discipline, so guidance cites a real 2026 standard, not a stale blog. Second, and load-bearing: an **always-first profile** that encodes the **cross-organizational, no-trusted-operator** use-case and **overrides generic single-org Data Mesh habit**. Generic mesh assumes one company on a trusted platform; this use-case is independent orgs that don't trust each other — which *forces* verifiable data products (trust the certificate, not the pipeline), federated *computational* governance (enforce operator-independently, not via a trusted platform team), and per-assertion provenance as the cascade circuit-breaker. The failure mode it prevents: applying single-org mesh patterns where the trust model is inverted — **or recommending a mesh at all when a lakehouse + a central team is the right answer** (the fit-check is first-class).

The corpus is grounded in primary sources (`references/sources.md`) and stays copyright-clean: our-own-words checklists that cite, never reproduce.

## Guardrails

Refusal conditions — they hold under "just design us a data mesh" pressure:

1. **Fit-check first.** Data Mesh is a sociotechnical, org-scale approach and is frequently the *wrong* answer. Before designing one, run the fit-check (`references/method.md` stage 0): if a lakehouse + a good central team delivers the value without sacrificing it, **say so**. Don't manufacture a mesh.
2. **Profile- and kit-first.** Load `references/data-mesh-profile.md` (the always-first overlay — the cross-org conventions that **override generic single-org habit**) **and** the relevant kit before designing or reviewing.
3. **Cite every finding; stay copyright-clean.** A primary source (`sources.md`) and/or a profile rule per finding — and respect the currency flags (target **ODCS** — the consolidating contract-level standard — not the deprecated Data Contract Spec; the six attributes, not "DATSIS" as Dehghani's term; the **product-level ODPS/DPDS** descriptor standards are unsettled, contract-level ODCS is not).
4. **Cross-org overrides generic.** Where the federated/no-trusted-operator profile differs from single-org mesh literature, the profile wins: trust the verifiable product not the pipeline; enforce governance computationally + operator-independently; expose provenance per assertion.
5. **Don't duplicate the adjacent lenses.** Product vision→architecture → `product-engineer`. The infra it runs on → `platform-engineer`. The telemetry/metrics stack → the observability agents (this skill owns *data-product* observability). Building the gating/bonding contracts → `solidity-developer` (`/evm`); designing the attestation → `tee-specialist` (`/tee`). This skill is the *data architecture* — domains, products, contracts, the governance model, quality, interoperability.

## The method

`references/method.md` holds the full method; the spine:

1. **Load the profile + the kit(s)** for the concern (domain decomposition, data products & contracts, federated governance, self-serve platform, data quality/observability, interoperability/lineage).
2. **Run the fit-check (stage 0)** — confirm mesh is the right answer before designing one.
3. **Design or review against the profile first, the canon second.** The profile (cross-org, verifiable products, computational governance, provenance circuit-breaker, claim-class quality) is what applies here; `sources.md` is the generic floor the profile sits on and sometimes overrides.
4. **Score/identify by the six dimensions** (`method.md`): domain decomposition & ownership · data-product design & contracts · federated computational governance & policy-as-code · self-serve platform leverage · data quality/observability/SLOs · interoperability & lineage. Cite every finding; rank trust/governance/one-way-door findings above style.

## Kit index

| Concern | Kit |
|---|---|
| Domain decomposition & ownership (incl. the fit-check) — DDD boundaries, real ownership vs governance theater, the trust boundary | `references/kit-domain-decomposition.md` |
| Data products & contracts — the architectural quantum, the six attributes, ODCS v3.x, ports/SLOs, the verifiable-certificate model | `references/kit-data-products-contracts.md` |
| Federated computational governance — global/local split, policy-as-code, the neutral-registry moat, the org-boundary trust switch | `references/kit-federated-governance.md` |
| Self-serve platform — the three planes, cognitive load, bounded/scoped/push-capable access | `references/kit-self-serve-platform.md` |
| Data quality & observability — the five pillars, OpenLineage, SLOs, the claim-class taxonomy, authenticity≠correctness | `references/kit-data-quality-observability.md` |
| Interoperability & lineage — global addressing/identity, polyglot ports, extend-without-migration, the product-descriptor standards | `references/kit-interoperability-lineage.md` |
| `kit-lakehouse-fabric-adjacency`, `kit-data-contract-tooling`, `kit-migration-brownfield`, `kit-cost-finops` | *(deferred — see `references/kit-TEMPLATE.md` roster; add by use)* |

## How the data-platform-architect agent hooks in

The `data-platform-architect` persona's first step loads `data-mesh-profile.md` + the kit for the work, runs the fit-check, then designs or reviews against the profile first. The agent owns the data architecture; `product-engineer` translates product vision, `platform-engineer` runs the infra, the observability agents own the telemetry backend, and `solidity-developer`/`tee-specialist` build the contracts/attestation the governance model specifies.

## Halt conditions

- **No target** to design/review — ask for the data architecture / domains / products in scope; never review from memory.
- **Mesh isn't the right answer** — say so (the fit-check); recommend the simpler architecture rather than designing an unneeded mesh.
- **A one-way door** (a namespace-authority binding, a published data-product contract/port, the shared ontology, an EIP-712-style governance type hash, a deployed gating/bonding governance config) — flag for human approval, don't assert.
- **The work is really another lens** — product→architecture (`product-engineer`), infra (`platform-engineer`), telemetry stack (observability agents), contract/attestation build (`/evm`, `/tee`) — redirect.

## What this skill defers

The deferred kits in `references/kit-TEMPLATE.md`'s roster (`kit-lakehouse-fabric-adjacency`, `kit-data-contract-tooling`, `kit-migration-brownfield`, `kit-cost-finops`) — add by use. The data-mesh standards layer moves fast — treat ODCS version numbers, the ODPS/DPDS product-descriptor standards, and OpenLineage facets as point-in-time; query the current state at design time. The profile encodes a specific cross-org use-case; for a different data-architecture problem, apply the fit-check and the generic canon directly.
