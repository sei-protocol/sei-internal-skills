# Data quality & observability kit

## 1. What this concern is

Making the data product's "**trustworthy**" attribute operational — measured SLOs, observability, lineage — *and* deciding what a quality guarantee can even mean. The generic single-org habit equates "trustworthy" with a catalog entry + some freshness checks. Cross-org that's necessary-but-insufficient: trustworthiness is **provenance + a verification class + (where re-checkable) an economic bond**, exposed per assertion — and **authenticity ≠ correctness ≠ truth**. *Cited:* `sources.md` §observability; profile §5, §3.

## 2. The pattern (how to do it)

- **Monitor the five pillars** (Monte Carlo's framing, de-facto common — *not* Dehghani canon): **freshness, volume, distribution, schema, lineage**. These operationalize "trustworthy" as SLOs on each product. *Cited:* `sources.md` §observability.
- **Publish SLOs on the product + alert on breach.** The contract (ODCS) carries the SLA/quality rules; observability verifies them continuously and surfaces breaches to the owning domain. *Cited:* `sources.md` §data-contracts, §observability.
- **Instrument lineage with OpenLineage.** The open standard (LF AI & Data Graduate): dataset/job/run entities + facets, end-to-end. Cross-org, thread one provenance anchor end-to-end — capture-at-ingestion → stamp-on-every-element → expose-on-read — so every element is re-verifiable to source, cheap per element, and replayable. *Cited:* `sources.md` §observability; profile §3.
- **Classify every claim — the claim-class taxonomy governs what a guarantee can be.** **Class I — deterministically re-checkable** (output of a deterministic program over committed, available inputs; verify by re-execution or a succinct proof — no vote, capture-resistant). **Class II — objective but not re-executable** (unambiguously phrased + resolution rules; verify by optimistic-challenge + economically-secured dispute adjudication). **Class III — subjective/judgment** (represent as a **signed opinion with provenance** + consumer trust policy — **never a correctness bond**). Discipline: **engineer claims into Class I** (bond a deterministic *check function* over a non-deterministic output, not the free-form output). *Cited:* profile §5.
- **Bond only re-checkable claims.** An economic bond converts a per-fact authenticity certificate into a per-fact correctness guarantee — *only* where the claim is Class-I (or Class-II with a sound dispute layer). The durability of the whole quality story ≈ the fraction of claims kept objectively re-checkable. *Cited:* profile §5.
- **Validity-time semantics: supersede, don't overwrite.** Facts are invalidated/superseded with temporal validity, not mutated in place — so lineage + history stay intact and a consumer can see "is-this-contested." *Cited:* profile §3.

## 3. Anti-patterns / failure modes

- **Authenticity treated as correctness.** Cue: "it's signed/attested, so it's correct/true." Rewrite: a signature proves *who attested which bytes*, never that the claim is correct or true — set the verification class; re-check (Class I) or rely on a bond/dispute (Class II). *Cited:* profile §5.
- **Bonding a judgment claim.** Cue: a correctness bond on a subjective/Class-III output. Rewrite: a signed opinion + consumer policy; bonding a judgment invites plutocratic capture of the dispute layer. *Cited:* profile §5.
- **"Trustworthy" = a catalog entry.** Cue: a product marked trustworthy with no measured SLOs / no provenance exposed. Rewrite: the five pillars monitored + SLOs published + per-assertion provenance.
- **No lineage / proprietary lineage.** Cue: no end-to-end lineage, or a bespoke format. Rewrite: OpenLineage; thread one provenance anchor end-to-end.
- **Overwriting facts in place.** Cue: an update that mutates a fact and loses its history. Rewrite: supersede with validity-time; keep the lineage + the contested flag.

## 4. Review cues

- **Dimension 5 (data quality, observability & SLOs):** the five pillars monitored; SLOs published on products + breach-alerted; OpenLineage end-to-end; every claim carries a verification class; **only Class-I/II claims carry a correctness guarantee** (Class-III = signed opinion + consumer policy); authenticity is not conflated with correctness/truth; facts supersede rather than overwrite. *Basis:* `sources.md` §observability; profile §5, §3.
- **Dimension 2 (data products):** the contract's SLA/quality rules match what observability actually enforces. *Basis:* `sources.md` §data-contracts.

## 5. One-way doors in this concern

- **A product's published SLOs / quality guarantees** — consumers depend on them; weakening a published SLO after adoption breaks downstream assumptions. Flag.
- **The verification-class / bonding scheme** — what a class means and what a bond covers; outstanding bonded products depend on it; coordinate any on-chain bonding change with `/evm`. Flag.
