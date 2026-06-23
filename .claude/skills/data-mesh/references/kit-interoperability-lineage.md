# Interoperability & lineage kit

## 1. What this concern is

Making data products **work together across domains and organizations** — global addressing/identity conventions, polyglot output ports, end-to-end lineage, and a product-descriptor strategy — so a consumer in one domain/org can find, address, and correlate a product from another. The generic single-org habit assumes one company can mandate a shared schema. Cross-org, **semantic interoperability is a multi-year standards/political effort** and the hardest project risk; the addressing scheme must let relationships extend **without migration**. *Cited:* `sources.md` §interoperability; profile §7.

## 2. The pattern (how to do it)

- **Globally standardize the cross-cutting concerns only.** Cross-domain identity/entities (so a "customer" in domain A can be correlated with one in domain B), addressing conventions, polyglot output formats. Keep this set minimal (it's federated-governance global policy) — over-standardizing crushes domain autonomy. *Cited:* `sources.md` §interoperability, §federated-governance.
- **Address under one uniform, hierarchically-extensible scheme.** `domain:vertical/kind/target[?dims][#view]` — `domain` is the org/namespace authority, `vertical` behaves like a resolver-bringing sub-namespace (add a category = register a `(vertical, kind)` resolver, never rewrite the scheme). **Identity = `domain:vertical/kind/target` only**; `?dims` and `#view` are *not* identity. So **a new relationship axis is a new dimension, never a new node and never a migration** — this is what keeps the multi-year interop cost bounded. Embed external standards verbatim in leaf grammars (account/asset/network identifiers, content-addresses) rather than reinventing. *Cited:* profile §7.
- **Polyglot output ports.** A product serves its data in one or many output ports (warehouse table, object-store Parquet, a stream/topic, an API) as defined by its contract — consumers pick the port that fits. *Cited:* `sources.md` §interoperability, §data-product.
- **Lineage end-to-end with OpenLineage.** dataset/job/run + facets across the mesh; thread one provenance anchor (capture→stamp→expose). Lineage is also an interoperability surface — it's how a consumer traces a cross-domain product back to source. *Cited:* `sources.md` §observability; profile §3.
- **Pick a product-descriptor strategy deliberately (the layer is unsettled).** Contract-level is consolidating on **ODCS** (datasets/APIs). The **product**-level descriptor layer — **DPDS** (opendatamesh) and **ODPS** (Linux Foundation, expanding into a 2026 "standards family") — is *not* settled. Choose with eyes open; don't assume a winner; verify the current state at design time. *Cited:* `sources.md` §interoperability, Verify-before-print.

## 3. Anti-patterns / failure modes

- **A new relationship requiring a new node / a migration.** Cue: adding a relationship axis by minting new entities or rewriting the addressing scheme. Rewrite: a new **dimension** on the existing identity (dims ≠ identity) — no migration. *Cited:* profile §7.
- **Reinventing identifier grammars.** Cue: a bespoke account/asset/network/content identifier. Rewrite: embed the external standard verbatim in the leaf grammar. *Cited:* profile §7.
- **Over-standardizing the global set.** Cue: the global ontology dictating a domain's internal model. Rewrite: globally standardize cross-cutting interoperability only; local models stay sovereign.
- **Assuming a product-descriptor winner.** Cue: building hard on one of ODPS/DPDS as "the standard." Rewrite: treat the product-level layer as unsettled (contract-level = ODCS); verify current state, keep the choice swappable. *Cited:* `sources.md` Verify-before-print.
- **Single-format lock-in.** Cue: a product served only as one warehouse table when consumers need a stream/API. Rewrite: polyglot output ports per the contract.

## 4. Review cues

- **Dimension 6 (interoperability & lineage):** cross-cutting concerns globally standardized (identity/addressing/formats) without crushing domain autonomy; one uniform addressing scheme where a new axis is a dimension (not a migration) and leaf grammars embed external standards; polyglot output ports per contract; OpenLineage end-to-end; a deliberate, swappable product-descriptor strategy (ODCS contract-level; ODPS/DPDS product-level treated as unsettled). *Basis:* `sources.md` §interoperability, §observability; profile §7.
- **Dimension 3 (governance):** the global interoperability standards are minimal and federated, not unilaterally imposed. *Basis:* profile §4.

## 5. One-way doors in this concern

- **The addressing scheme + the shared ontology / identity conventions** — every product's address and every cross-domain correlation depends on them; a change is a coordinated migration across all participants. Flag.
- **The chosen product-descriptor standard** once products publish descriptors against it — a switch re-descriptors every product; pick deliberately and flag a later change.
