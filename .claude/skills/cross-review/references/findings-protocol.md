# Findings Protocol

The synthesized output of cross-review (Step 4) and the research grounding the discipline.

## The findings table

| Interface / Boundary | Provider | Consumer | Status | Evidence | Raised by |
|---|---|---|---|---|---|

- **One row per boundary**, not per reviewer. Merge the independent reviews; de-duplicate.
- **Provider / Consumer** — who defines the interface, who adapts to it. The provider's definition is canonical; the consumer adapts. This is the tie-break when reviewers disagree.
- **Evidence** — the specific contract, field, signature, error, or line. A row with no evidence is not a finding.
- **Raised by** — which reviewer (and flag if it came from the assigned dissenter — those deserve a second look).

## Status values

- **COMPATIBLE** — provider and consumer agree on this boundary; the evidence confirms it. Only valid if a reviewer actually checked it (cited evidence), not inferred.
- **MISMATCH** — provider and consumer disagree on this boundary. Always carries a category (below) and the specific divergence.
- **MISSING** — the artifact assumes this boundary/behavior but never defines it. A gap is a finding: unstated error behavior, undefined default, unspecified ordering, an interface one side expects that the other never provides.

## Mismatch categories

When a finding is MISMATCH, name the category — it sets the severity and the fix:

- **Signature** — the shape doesn't line up: parameter count/order, endpoint path/verb, message fields, function arity. (Analog: `NoSuchMethodError` — a syntactic break.)
- **Type** — same field, incompatible type or representation (int vs string, enum value sets that don't match, units).
- **Error-contract** — the provider raises errors the consumer doesn't handle, or the consumer expects error semantics the provider doesn't honor. Errors are part of the interface.
- **Naming** — the same concept under different names across the boundary, or the same name meaning different things.
- **Sequencing / behavioral** — shapes match but order, timing, idempotency, or state assumptions differ. The dangerous class — no syntactic error, fails at integration. (Analog: a semantic break with no compile error.)

## Verdict

- **COMPATIBLE (overall)** — every boundary is COMPATIBLE; no open MISMATCH/MISSING. State the confidence: high if reviewers were blinded and independent, lower (and say why) if not.
- **OPEN (N findings)** — one or more MISMATCH/MISSING unresolved and not yet accepted. List each with what would close it. This is a legitimate, valuable output — not a failure to be smoothed into a green light.

Resolved findings move provider-first: update the interface source / provider definition, then the consumer adapts, then re-check.

## Research grounding (why the discipline is shaped this way)

- **Independent preparation is the engine.** Fagan inspection's individual-preparation phase — reviewers commit findings before the group meeting — drove its high defect-detection rates. Cross-review's blinded dispatch replicates it. (Fagan inspection.)
- **A confident voice anchors a group.** In Asch's conformity experiments ~75% conformed to an obviously-wrong majority at least once; a single independent dissenter restored judgment. Hence blinded reviews + an assigned dissenter. (Asch conformity experiments.)
- **Rubber-stamping is the default failure.** "LGTM" approvals train teams to treat review as noise; evidence-bearing findings are the counter. (Code-review practice literature.)
- **Multi-agent LLMs collapse into sycophantic consensus** when they see each other's outputs before committing — sometimes scoring *below* a single agent, and LLM judges can prefer a persuasive falsehood. Independence + evidence-anchored synthesis (not rhetorical synthesis) are the mitigations. (Multi-agent LLM debate / sycophancy literature, e.g. arXiv 2509.23055, 2509.05396; CONSENSAGENT, ACL 2025.)
- **Provider owns the interface; consumer adapts.** API-evolution practice: additive, backward-compatible change preserves consumers; breaking change is a MAJOR (SemVer) event. Mismatch categories above mirror compatibility-checker break classes. (API versioning / backward-compatibility practice.)
