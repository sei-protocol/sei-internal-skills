# Self-serve data platform kit

## 1. What this concern is

The platform that lets domain teams **create and consume data products without deep platform expertise** — reducing their cognitive load so decentralization doesn't mean every domain rebuilds infrastructure. The generic single-org habit assumes a *trusted central platform team* runs it. Cross-org, no party trusts another's platform operator, and the access surface must expose **provenance per assertion** and be **bounded/scoped** so one global store isn't the bottleneck and corruption surface. *Cited:* `sources.md` §self-serve-platform; profile §8.

## 2. The pattern (how to do it)

- **Cover the three planes.** (1) **Infrastructure provisioning** (storage, compute, orchestration, access control); (2) **data-product developer-experience** (the day-to-day plane — high-level abstractions a typical domain dev uses to build/serve a product without low-level infra work); (3) **mesh supervision** (discovery/catalog, cross-product correlation, mesh-level governance). The middle plane is where cognitive-load reduction actually happens. *Cited:* `sources.md` §self-serve-platform.
- **Platform is domain-agnostic; domains own products + semantics + SLOs.** The platform provides the generic capability; it does not own any domain's data or meaning. Draw the build-vs-domain line explicitly so the platform doesn't become a central bottleneck. *Cited:* `sources.md` §self-serve-platform.
- **The access surface is bounded, scoped, and push-capable (cross-org).** Namespacing (per-org/per-domain/per-consumer) is first-class in resource addresses + query scope. Retrieval is **associative** (query by *what*, not by address — semantic + N-hop), **bounded** (sub-graph/region/summary with pagination — never "return the whole store"; unbounded sharing actively degrades consumers), **push-capable** (subscribe/notify, not poll), and returns **per-assertion provenance** (source-anchor, issuer, timestamp, version, verification class, is-contested) so the consumer can verify-before-act. *Cited:* profile §8, §3.
- **Reserve strong consistency for the few ops that need it.** Session-sticky read-your-writes as the floor; causal consistency cross-consumer; global ordering only for uniqueness/lease/atomic-claim ops — don't pay for linearizability everywhere. *Cited:* profile §8.

## 3. Anti-patterns / failure modes

- **A trusted-central-platform assumption cross-org.** Cue: a shared platform whose operator every org must trust for access/quality decisions. Rewrite: operator-independent authority (`kit-federated-governance`); the platform serves, the neutral venue governs.
- **Whole-store / whole-graph dumps.** Cue: a `read_all` / unbounded export endpoint. Rewrite: bounded, paginated, scoped queries — unbounded sharing degrades consumers past a threshold. *Cited:* profile §8.
- **Polling instead of subscribe/notify.** Cue: consumers polling for changes. Rewrite: push on state change (subscribe/notify) for live handoff. *Cited:* profile §8.
- **Serving data without provenance.** Cue: a read that returns values with no source-anchor/issuer/verification-class. Rewrite: per-assertion provenance on every read (the verify-before-act input). *Cited:* profile §3.
- **The platform absorbing domain semantics.** Cue: the central platform team defining domains' data meaning/SLOs. Rewrite: domain-agnostic platform; domains own semantics — else it's recentralization (governance theater).
- **A single global store.** Cue: one un-namespaced store all domains write to. Rewrite: first-class namespacing — one global store is both the bottleneck and the corruption surface. *Cited:* profile §8.

## 4. Review cues

- **Dimension 4 (self-serve platform leverage):** the three planes are covered; the developer-experience plane genuinely reduces domain cognitive load (not undifferentiated heavy-lifting pushed onto domains → sprawl); the platform is domain-agnostic; access is namespaced, bounded, push-capable, and provenance-bearing; consistency is reserved for ops that need it. *Basis:* `sources.md` §self-serve-platform; profile §8, §3.
- **Dimension 3 (governance):** cross-org, platform access decisions are operator-independent, not a trusted-operator gate. *Basis:* profile §4.

## 5. One-way doors in this concern

- **The access-surface contract** (the query/subscription API, the resource-addressing/namespacing scheme) — domains + consumers build against it; a breaking change is a versioned migration across every participant. Flag.
- **The consistency model exposed to consumers** (what ordering/visibility guarantees the surface promises) — consumers design around it; weakening it post-adoption breaks correctness assumptions. Flag.
