# Domain decomposition & ownership kit

## 1. What this concern is

Drawing data ownership along **business domains** (DDD), with a team accountable end-to-end for each domain's data products — *and first deciding whether to decentralize at all*. The generic single-org habit gets two things wrong for this use-case: it assumes the org is large/mature enough that a mesh beats a central team (often false), and it treats the domain boundary as purely organizational. Cross-org, **the domain boundary is also a trust boundary** — each domain is owned by an independent organization that the others don't trust. This kit carries the **fit-check** (is mesh even right?) and the ownership model. *Cited:* `sources.md` §principles, §fit-and-failure-modes; profile §1, fit-check.

## 2. The pattern (how to do it)

- **Run the fit-check before decomposing.** Mesh is sociotechnical and org-scale. Ask: is the org large/complex enough that domain decentralization beats a central team? Is there genuine domain expertise to own data products? If a lakehouse + a good central team delivers the value without sacrificing it, **recommend that instead**. The cross-org use-case is the strongest *fit* case — decentralization is forced by the org boundary, not chosen. *Cited:* `sources.md` §fit-and-failure-modes.
- **Decompose along business domains (DDD), not along pipeline stages or tech.** Each domain owns its analytical data end-to-end (source-aligned, aggregate, or consumer-aligned products). Ownership = **autonomy + accountability**: the domain team controls and is answerable for its products' quality and SLOs. *Cited:* `sources.md` §principles.
- **Name the namespace authority per domain.** Each domain owns a slice of a uniform, hierarchically-extensible namespace (`domain:vertical/kind/target`); the `domain` segment is the ownership/authority anchor. Cross-org: a thin neutral registry answers *which org/account is authorized to own/grant a namespace* (see `kit-federated-governance`). *Cited:* profile §1, §7.
- **The domain boundary is the trust boundary (cross-org).** Intra-domain, the owning org is the trusted operator. Cross-domain (across orgs), assume mutual distrust — products crossing the boundary must be verifiable (see `kit-data-products-contracts`), access governed computationally (`kit-federated-governance`). *Cited:* profile §1, §2.

## 3. Anti-patterns / failure modes

- **Mesh where a central team would do.** Cue: a small org / small data footprint, or no domain team able to own products. Rewrite: recommend a lakehouse + central team; mesh is for genuine scale/complexity. *Cited:* `sources.md` §fit-and-failure-modes.
- **Governance theater (central team in disguise).** Cue: "domains" that are folders in one dbt repo owned by a central data team; ownership without autonomy or accountability. Rewrite: real domain ownership, or admit it's centralized and drop the mesh framing. *Cited:* `sources.md` §fit-and-failure-modes.
- **Decomposition by pipeline stage / technology.** Cue: "ingest", "transform", "serve" as "domains". Rewrite: business-domain boundaries (DDD) so a team owns a cohesive slice end-to-end. *Cited:* `sources.md` §principles.
- **Ownership drift / producer-consumer misalignment.** Cue: products with no owning team, or owners who don't know their consumers. Rewrite: a named accountable team per product + a contract that binds producer to consumer (`kit-data-products-contracts`).
- **Treating a cross-org boundary as a mere org-chart line.** Cue: a cross-org data share that assumes the other org's platform/operator is trusted. Rewrite: model the boundary as a trust boundary — verifiable products + computational governance. *Cited:* profile §1.

## 4. Review cues

- **Dimension 1 (domain decomposition & ownership):** the fit-check passed (mesh is warranted, not a default); boundaries follow business domains (DDD); each product has an accountable owning team with real autonomy (not governance theater); the namespace authority per domain is named; cross-org, the domain boundary is treated as a trust boundary. *Basis:* `sources.md` §principles, §fit-and-failure-modes; profile §1.
- **Dimension 3 (federated computational governance):** who may own/grant a namespace is answerable by the neutral registry, not a single operator. *Basis:* profile §4.

## 5. One-way doors in this concern

- **A namespace-authority binding** (which org/account owns or may grant a domain namespace) — a trust anchor consumers and grants depend on; one-to-one and hard to walk back. Flag for human approval.
- **The domain decomposition itself** once products are published and consumed — re-drawing boundaries breaks published products' addresses + contracts; treat a re-decomposition as a migration, flag the blast radius.
