# Federated computational governance kit

## 1. What this concern is

Governance that splits **global** rules (interoperability, identity, security, the shared ontology) from **local** domain autonomy, and **enforces the global rules automatically by the platform** rather than by manual committee review — the "computational" in federated computational governance. The generic single-org habit assumes a guild decides and a trusted central platform enforces. Cross-org there is **no trusted platform operator**: enforcement must be **operator-independent**, and the canonical registry that defines the global rules is the **moat and the crown-jewel attack surface**. *Cited:* `sources.md` §federated-governance; profile §4.

## 2. The pattern (how to do it)

- **Split global from local.** Global (decided by the federation/guild): cross-domain identity/entities, addressing conventions, security, the shared ontology, what counts as a valid transformation. Local (sovereign to each domain): the domain's own data models. Keep the global set **minimal** — every global rule is a coordination cost and a capture surface. **Reference/master data is a global concern:** cross-domain entity resolution (a "customer" in domain A correlated with one in domain B) needs a federated answer to *who owns the golden record* — and cross-org, where no operator is trusted, that ownership is itself a registry/governance question, not a central MDM team. *Cited:* `sources.md` §federated-governance.
- **Enforce computationally (policy-as-code), not manually.** Global policy is executed by the platform — schema/quality validation in the contract pipeline, automated access-gating, bonds — not attested by humans in a review meeting. *("Policy-as-code" is the implementation label; the canon's word is "computational.")* *Cited:* `sources.md` §federated-governance.
- **Cross-org: enforcement is operator-independent.** A manual or single-operator gate re-introduces the trusted party the use-case exists to dissolve. The authority (who may access what, whether a bond fired) is read from a **neutral venue** any conformant node enforces identically — apply the org-boundary trust switch: only "a contract must atomically react" forces full on-chain enforcement; existence/ordering alone earns an **anchor** (off-chain signed log + periodic Merkle-root). Default anchored; make the chain earn each function. *Cited:* profile §1, §4.
- **Defend the thin neutral registry as the moat.** The registry of "what the namespace/ontology/valid-derivations are" is the un-buyable, credibly-neutral asset — and the highest-leverage attack surface (corrupt it and every downstream gate/bond validates forgeries). Defend it: **small + objective + fail-closed + m-of-n + time-locked (approve-slow / revoke-fast) + reference-value-transparent + irreversible/ossifiable**. Shrink the prize; **remove the lever, don't merely distribute it**. No named winners (abstract open rules), no discretionary minter. *Cited:* profile §4. (Hand the on-chain registry/gating/bonding *implementation* to the `/evm` skill — it owns the delegation/gating/bonding contract kit.)
- **Reconcile gating with neutrality.** Contract-gated access is in tension with the open/permissionless neutrality that creates the network effect; gate by a **public contract whose rules anyone reads and any node enforces identically**, not by a privileged operator — so gating and neutrality reconcile because the gate is operator-independent. *Cited:* profile §4.

## 3. Anti-patterns / failure modes

- **Manual / operator-dependent enforcement cross-org.** Cue: a human-review gate, or one org's server deciding grants/quality for a shared product. Rewrite: computational, operator-independent enforcement read from a neutral venue. *Cited:* profile §4.
- **A fat, mutable, contestable registry.** Cue: a registry encoding lots of discretionary parameters or named-winner allocations, changeable instantly by a small set. Rewrite: thin/objective/abstract-rules + m-of-n + timelocks + irreversibility. *Cited:* profile §4.
- **Decentralization theater in governance.** Cue: an "m-of-n" that's really one team; a guild with no real authority. Rewrite: genuine multi-party control + machine-checkable irreversibility (not just more signers). *Cited:* `sources.md` §fit-and-failure-modes; profile §4.
- **Federated-governance gridlock.** Cue: every cross-domain decision blocks on unanimous guild consensus. Rewrite: minimal global set + clear local autonomy; the guild governs *uniqueness/interoperability*, not every domain's choices. *Cited:* `sources.md` §fit-and-failure-modes.
- **Global-izing what should be local (or vice versa).** Cue: the ontology dictating a domain's internal model, or each domain inventing its own identity scheme. Rewrite: global = cross-cutting interoperability only; local = domain models.

## 4. Review cues

- **Dimension 3 (federated computational governance & policy-as-code):** clean global/local split with a minimal global set; global policy enforced **automatically**, not manually; cross-org enforcement is operator-independent (read from a neutral venue; chain earns each function); the registry is thin/objective/fail-closed/m-of-n/time-locked/irreversible/transparent with no discretionary minter; the guild is empowered but not a gridlock. *Basis:* `sources.md` §federated-governance, §fit-and-failure-modes; profile §4.
- **Dimension 1 (domain ownership):** the registry answers who may own/grant each namespace. *Basis:* profile §1.

## 5. One-way doors in this concern

- **The registry/governance configuration** (the m-of-n set, timelock windows, the global rule set, namespace→authority bindings) — a compromise mints valid-looking authority for anything; human approval + irreversibility discipline, never a quiet edit.
- **The shared ontology / valid-transformation set** — published; downstream products + gates depend on it; a change is a coordinated migration.
- **A deployed gating/bonding governance config** — outstanding grants/bonds depend on it; coordinate the change with `/evm` and flag it (the on-chain pieces are one-way doors in their own right).
