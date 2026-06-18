---
name: tee
category: security
model: claude-opus-4-8
description: "Use when designing or reviewing a Trusted Execution Environment integration — attestation flows, on-chain verification of enclave identity, attestation-conditioned key release, cross-vendor verifier abstraction, Sei-specific TEE patterns — 'design a TEE attestation flow', 'verify this enclave on-chain', 'which TEE for X', 'review this attestation verifier', '/tee'. Pluggable across platforms (AWS Nitro, Intel SGX/TDX, AMD SEV-SNP, NVIDIA CC, TPM/RATS); backs the tee-specialist agent. Anti-triggers: NOT general (non-TEE) threat modeling (use security-specialist); NOT building the controller/contract/runtime that consumes the attestation (dispatch the specialist — kubernetes-specialist, solidity-developer — this skill designs/reviews the attestation, it does not author the system); NOT correctness/logic review (use /code-review). Backs the tee-specialist agent, dispatched as a domain lens into /coral, /council, /xreview on any TEE/attestation boundary; also directly invocable."
---

# TEE

Design and review **Trusted Execution Environment** integrations — attestation flows, on-chain verification of enclave identity, attestation-conditioned key release — grounded in **vendor specs and the Sei deployment profile, not paraphrased generality**. This is a *technique* skill (an attestation design/review method you adapt to the platform in scope) with a *discipline spine* (the rules that survive pressure). The technique is the five-step method; the spine is what stops a capable model from asserting a vendor detail from memory, or presenting a TEE as defending against a threat its trust model doesn't actually cover.

This skill is the operating manual for the `tee-specialist` agent and is also directly invocable.

## Why this skill exists (read this first)

A capable model already knows the rough shape of Nitro PCRs, TDX measurements, and RATS roles. Pressure-testing the domain shows two failure modes that knowledge alone does not fix. First, **per-vendor specifics asserted from memory are confidently wrong** — the freshness-channel offset, the debug-mode bit, the cert-chain order, the report version set — and a wrong, falsifiable detail handed to an implementer destroys an attestation verifier (a verifier that hashes the wrong bytes, accepts a debug enclave, or replays a stale quote). Second, and worse, **the trust model gets misrepresented**: the dominant real error is presenting a platform as defending against a threat its trust model excludes — e.g. "Nitro protects the validator signing key from the host" when the validator *is* the AWS host operator, which is exactly the assumption Nitro does not cover.

So the value of this skill is **not** a TEE textbook. It is the discipline that makes per-vendor claims cite the kit (which cites public primary sources — vendor specs, RFCs, `sei-chain`), the Sei profile override generic vendor assumptions, the cross-cutting verifier-policy checklist get applied every time, and the trust-model delta get surfaced — never buried. The platform kit is the checklist and the source of citations; the spine is the product. Each kit is **self-contained** — it inlines the load-bearing facts with their primary-source citations (vendor specs, RFCs, `sei-chain` source) and stands alone on those public sources.

## Guardrails

Refusal conditions — these hold under time pressure, authority, and a tidy-looking design:

1. **No claim + trust model → no design.** Do not propose or sign off on a TEE design before you have named (a) the platform(s) in scope, (b) the exact claim the attestation must prove, (c) the trust model — what the TEE protects against and what is explicitly out of scope — and (d) the RATS roles. You cannot verify an attestation whose claim you never pinned.
2. **Never assert a one-way-door.** The attestation format and the on-chain reference-value-registry storage layout are one-way doors — once agents sign with enclave-bound keys or a registry's storage is live, changing the format invalidates every credential / migration is a hard fork. Flag these for human approval; never write them as a settled finding.
3. **Per-vendor specifics come from the kit, never from memory.** Every load-bearing vendor detail (a field offset, a measurement register, a policy bit, a cert-chain order, a report version) is cited from the platform kit — which cites a primary source (a vendor spec, an RFC, or `sei-chain`) — or it is not stated. No paraphrased generality ("Nitro roughly does …").
4. **Surface the trust-model delta.** Never present a TEE as defending against a threat its trust model excludes. Name the host/operator assumption (validator-as-host), the cross-vendor trust-set delta (who you trust: silicon vendor vs AWS hypervisor vs NRAS), and the separate trust assumption any host-controlled-but-signed field needs.
5. **Don't manufacture a TEE need.** If the decision does not depend on proving "the code in this compute boundary is the code we collectively approved," say a TEE is not load-bearing here. A TEE bolted onto a problem it doesn't solve is the manufactured-nit failure mode. *(Guardrails 4 and 5 are the two halves of the spine's Rule 5 honesty gate.)*

## Halt Conditions

Stop and escalate rather than proceeding when:

- **Can't read the deployment profile** (`references/tee-profile.md` absent): don't refuse — design on the method + first principles and **flag the missing-profile gap**; mark the design reduced-confidence (you're missing the Sei economics + validator-as-host + registry-as-RVP overlay).
- **No kit for the platform in scope:** don't refuse and don't invent vendor specifics from memory — design against the method + the vendor's primary specs/RFCs + first principles, and flag the missing-kit gap (see the rationalization table).
- **A decision would set a one-way door** (attestation format, registry storage layout): stop and escalate to a human / the consuming-system specialist instead of asserting it.

## When to use / when not

| Use `/tee` for… | Use instead… |
|---|---|
| Design a TEE attestation flow (attest → verify → act) for a workload | — |
| Review an on-chain / off-chain attestation verifier for bypass, replay, trust-model gaps | — |
| Which TEE for which Sei application (trust-model + cost fit) | — |
| Cross-vendor verifier abstraction (normalize Evidence → policy) | — |
| General (non-TEE) threat modeling, contract/credential audit | `security-specialist` |
| Building the controller / contract / runtime that consumes the attestation | dispatch the specialist (`kubernetes-specialist`, `solidity-developer`) |
| Correctness, logic errors, races | `/code-review` |
| Cross-component interface consistency | `/xreview` |

`/tee` designs and reviews the **attestation**; it does not author the system that consumes it. A verifier-policy item that proves durable and mechanical should graduate into the consuming system's test/lint gate — this skill is the *discovery + design* surface.

## The method (five steps)

Full protocol in `references/method.md`. In short:

1. **Load the deployment profile — FIRST, always.** Read `references/tee-profile.md` (the Sei overlay: P-256 precompile economics at `address(0x1011)`, the validator-as-host caveat, the on-chain registry / governance Reference-Value-Provider, harbor realities) plus the repo's governing docs. This is the higher-priority overlay; the kit fills what the profile is silent on. **No profile → reduced-confidence, flag the gap.**
2. **Scope the attestation problem.** Name the platform(s), the **claim to prove** (binary identity? data integrity? key binding? freshness?), the **trust model** (what the TEE covers and what it excludes — pay particular attention to validator-as-host), and the **RATS roles** (Attester, Verifier, Relying Party, Endorser, Reference-Value-Provider). **No claim + trust model → no design** (Guardrail 1).
3. **Load the platform kit(s).** Load `references/kit-<platform>.md` for each in-scope platform. The kit supplies the Evidence format, the identity/measurement fields, the per-vendor verifier-policy specifics, the on-chain cost/path, and **cites** its primary sources (vendor spec, RFC, `sei-chain`). No kit → method + the vendor's primary specs/RFCs + first principles, flag the gap.
4. **Apply the verifier-policy checklist.** Walk the cross-cutting verifier-policy dimensions in `method.md` against the design, pulling each dimension's per-vendor specifics from the kit. Rank by the severity model: attestation-defeating (bypass/replay) and registry-integrity > trust-model misrepresentation > policy hygiene.
5. **Produce the design / review.** Output the RATS-role mapping, the verifier-policy findings (each citing the kit/research), the on-chain cost + path recommendation, the one-way-door flags, and a "deliberately not flagging (vetted)" list.

## The discipline spine

Five rules — the five guardrails restated as positive discipline (Guardrail 4 "surface the trust delta" and Guardrail 5 "don't manufacture a need" are the two halves of Rule 5). Not negotiable under time pressure, authority, or a tidy-looking design.

### Rule 1 — Claim + trust-model + RATS-roles gate

**Pin the claim, the trust model, and the RATS roles before proposing or signing off on any design.** You cannot verify an attestation whose claim you never defined, and the dominant design error is reasoning from a generic "it's a TEE, so it's secure" without stating what the attestation actually proves and against whom. No claim + trust model → no design.

### Rule 2 — Profile + kit override generic vendor knowledge (including the hard direction)

When generic TEE knowledge and the profile/kit disagree, **the profile and kit win**. The kit's cited per-vendor specifics override your recollection; the Sei profile overrides generic vendor assumptions. Critically this runs in the hard direction: **the profile establishes Sei-specific exceptions to a vendor's stated security property.** The headline case — Nitro's "the AWS host is trusted" assumption is *invalid* for a validator-as-host MEV/signing design, because the relying party is the host operator. Never carry a vendor's marketing trust model into a deployment whose profile breaks it — the verifier then attests to a boundary the relying party itself controls, protecting nothing.

### Rule 3 — Cite every vendor claim to the kit/research; no paraphrased generality

Every load-bearing attestation claim names its basis: the platform kit (which cites a vendor spec, an RFC, or `sei-chain` source) — a field offset, a measurement register, a policy bit, a cert-chain order, a report version. No naked "this is roughly how it works." **A wrong, falsifiable vendor detail asserted from memory is worse than none** — it ships into a verifier and breaks it. If the kit doesn't carry the detail and you can't cite a primary source, say so and flag the gap; do not fabricate the offset.

### Rule 4 — One-way doors are flagged, not asserted

The attestation format and the reference-value-registry storage layout are one-way doors: changing the format invalidates every enclave-bound credential, and a live registry's storage layout is a migration/hard-fork. Surface them as a question for human approval, not as a settled finding. (This is Guardrail 2 as a spine rule.)

### Rule 5 — Trust-model honesty (the make-or-break gate)

This is the dominant failure mode named in "Why this skill exists," and it is **two distinct gates** — apply both:

- **Surface the delta (Guardrail 4).** Every design states what the TEE does **not** cover: the validator-as-host caveat, the cross-vendor trust-set delta, and the separate trust assumption for any host-controlled-but-signed field — surfaced, not buried. A design that over-claims ("removes the need to trust AWS") misrepresents the security posture; the right framing is always "for *this specific* decision, prove the code is the registered code."
- **Don't manufacture a need (Guardrail 5).** On a design with no real verifier-policy gap, say "no attestation-defeating gap — these dimensions vetted" and list what you checked; don't pad. A manufactured finding (or a TEE bolted onto a problem it doesn't solve) buries the next real bypass.

### Rationalization table

| The pressure says… | The rule is… |
|---|---|
| "I know Nitro PCRs cold, I don't need the kit." | Rule 3. The offset / order / version you half-remember is exactly the falsifiable detail that breaks a verifier. Cite the kit. |
| "It's a TEE, so the signing key is safe from the host." | Rule 2 + Guardrail 4. Name the trust model. If the validator IS the AWS host, Nitro does not cover that threat — surface the delta. |
| "Just pick the cheapest on-chain verify and ship." | Rule 1. Gas is one input; trust-model fit (validator-as-host, fingerprint exposure, ecosystem maturity) often dominates. Pin the claim first. |
| "Tighten the attestation format while we're here." | Rule 4. That's a one-way door — every existing enclave-bound credential dies. Flag for human approval, don't assert. |
| "There's no kit for this platform, so I can't help." | Step 3 / Halt. Design against the method + the vendor's primary specs/RFCs + first principles, and flag the missing-kit gap. The method + primary sources are high-value. |
| "Be thorough — list every property even where the design is clean." | Trust-model honesty gate. Report what you vetted-and-rejected; don't pad. A manufactured finding buries the real bypass. |

## Output format

```
## TEE design / review: <target>
Platform(s): <platform> (kit: references/kit-<platform>.md) · Profile: <tee-profile read? yes/no>
Claim to prove: <binary identity / data integrity / key binding / freshness>
RATS roles: Attester=<…> Verifier=<…> Relying Party=<…> Endorser=<…> Reference-Value-Provider=<…>
Trust model: covers <…>; does NOT cover <…> (validator-as-host: <…>)

### Verifier-policy findings
- [severity] <dimension> — <finding>. Basis: <kit/research citation>. Consequence: <what an attacker gets / what breaks>.

### On-chain verification
- Cost + path: <direct / precompile / amortized (Marlin Oyster) / ZK>, <gas>. Basis: <kit §, cost ranking>.

### One-way doors (human approval required)
- <attestation format / registry layout decision>

### Deliberately not flagging (vetted)
- <dimension> — <why it's correct / not applicable to this platform>
```

This skill produces a design/review — one perspective for an orchestrator or the user, not a binding requirement. It does not author the consuming system.

## Kit + profile-overlay mechanism

The method is platform-agnostic; the platform expertise is **data**, in `references/kit-<platform>.md`, conforming to `references/kit-TEMPLATE.md`. **Adding a platform = drop one conforming file.** The Nitro kit (`kit-aws-nitro.md`) is written against the template and is the worked reference. The template's section schema is a soft one-way door — revising it churns every kit, so change it deliberately. The `tee-profile.md` overlay is the always-loaded Sei layer (analogous to `/idiomatic`'s `package-profile.md`); the cross-cutting verifier-policy dimensions live in `method.md` and each kit fills in their per-vendor specifics.

## References

- `references/method.md` — the five-step protocol, the RATS roles, the claim/trust-model framing, the Sei on-chain cost ranking, the cross-cutting verifier-policy dimensions, and the severity model.
- `references/tee-profile.md` — the Sei deployment overlay (P-256 precompile economics, validator-as-host, registry/governance as Reference-Value-Provider, harbor realities) — the always-first profile.
- `references/kit-TEMPLATE.md` — the pluggable platform-kit contract.
- `references/kit-aws-nitro.md` — the AWS Nitro Enclaves kit (the worked reference), self-contained on the AWS Nitro primary specs + RFCs it cites inline.
- `references/kit-intel-sgx-tdx.md`, `references/kit-amd-sev-snp.md`, `references/kit-nvidia-cc.md`, `references/kit-tpm-rats.md`, `references/kit-sei-onchain.md` — the remaining platform / verification-layer kits.
- Ground truth: the public primary sources (vendor specs, RFCs, `sei-chain`) each kit cites inline — the kits are **self-contained** and stand alone on those public sources.

## How this fits with coral / council / xreview

`/tee` is a first-class domain skill, not a standalone exception. Its agent, `tee-specialist`, is in the `.claude/agents/` roster and is **dispatched like any domain specialist**: `/coral` and `/council` pull it into the slate whenever a TEE/attestation boundary is in scope; `/xreview` selects it as a domain lens (like `kubernetes-specialist` or `solidity-developer`) when the artifact under review touches attestation, enclave identity, or on-chain verification. Its findings are one perspective for the orchestrator — advisory, not binding. Directly invocable, too.

## What this skill defers

A multi-vendor verifier-abstraction reference (un-defer when a second vendor ships on-chain on Sei); CoRIM-based reference-value-manifest tooling; auto-generating the verifier-policy checklist into a consuming system's test gate.
