# Platform kit contract (TEMPLATE)

A platform kit is **data** the method loads. Every kit must provide the seven sections below, in this order, so the method stays platform-agnostic. **Adding a platform = drop one file conforming to this template** at `references/kit-<platform>.md`.

This section schema is a **soft one-way door**: changing it churns every existing kit. Revise deliberately.

The kit is the **opinionated, pluggable layer over the research** — it does not replace `design/research/tee/<doc>.md`. Every load-bearing claim **cites** that research doc (a `§` or load-bearing-claim number), a vendor spec, or an RFC; the kit never paraphrases the research into a second, drifting copy. If a fact isn't in the research doc, it doesn't belong in the kit (raise it as a research gap instead).

Copy the skeleton below and fill it. See `kit-aws-nitro.md` for a complete worked kit.

---

```markdown
# <Platform> kit

> Ground truth: `design/research/tee/<doc>.md`. Every claim below cites it (§ or load-bearing claim #), a vendor spec, or an RFC. Do not paraphrase — cite.

## 1. Identity & RATS roles

- **What it is** — process-level enclave / VM-level CVM / GPU TEE; the boundary it protects.
- **RATS role mapping** — Attester = <…>; Endorser = <the PKI / trust root>; typical Verifier = <on-chain contract / KMS / off-chain service>.
- **Trust root / Endorser** — the root of trust to pin, and **whose trust set it is** (silicon vendor vs cloud hypervisor vs NRAS) — this feeds VP16.
- **Ground-truth doc** — `design/research/tee/<doc>.md`.

## 2. Evidence format

The wire format the Attester produces and the Verifier consumes. Signature scheme, envelope, the canonical spec citation, and the **parser gotchas** an implementer hits (the bytes that are actually signed, encoding canonicalization, signature shape). This is the VP9 "parse Evidence" half and the VP13 version set.

## 3. Identity & measurement fields (VP2)

What proves **binary identity** on this platform — the measurement register(s), their semantics, size, and hash. State exactly which fields are *required together* (a partial identity that's exploitable is a finding, e.g. TDX `MRTD` alone). Cite the field semantics to the research doc.

## 4. Verifier-policy specifics — the per-vendor fill-ins

The table that plugs this platform into `method.md`'s cross-cutting dimensions. Include a row for **every** method dimension VP1–VP16 so each has a landing spot — the per-vendor specific, a **pointer** (VP6→§6, VP7/VP15→profile registry, VP9/VP16→method + §1 trust-set), or an explicit **N/A** with the reason. The explicit-N/A rule matters: it lets a reviewer applying method Step 4 cite the kit for "vetted, not applicable" rather than hunt for (or fabricate) a field that doesn't exist on this platform. Keep the `method dimension` IDs verbatim so the method ↔ kit mapping stays mechanical. Include a **VP2** row here (the binary-binding *policy*) even though the measurement-field detail lives in §3.

| method dimension | this platform's specific | cite |
|---|---|---|
| VP1 freshness | <field, offset, size> | <doc §> |
| VP2 binary binding | <which measurement(s) gate identity; required-together?> | <doc §> |
| VP3 debug-mode | <the field/bit to reject> | <doc §> |
| VP4 anti-rollback | <TCB field + policy> | <doc §> |
| VP5 cert-chain / generation | <chain shape, order, pinned root, per-gen cert selection> | <doc §> |
| VP6 key isolation | → §6 (the platform's key-release pattern) | §6 |
| VP7 revocation | → profile (governance revocation in the registry) | profile |
| VP8 joint-attester | <binding mechanism, or N/A> | <doc §> |
| VP9 policy separation | <how Evidence parses into a normalized claim set> | method + §2 |
| VP10 advisories | <field or N/A — why> | <doc §> |
| VP11 known-CVE bits | <bit + CVE, or N/A> | <doc §> |
| VP12 host-controlled-but-signed | <fields, or N/A> | <doc §> |
| VP13 version pinning | <the version set to pin> | <doc §> |
| VP14 privacy / fingerprint | <permanent device ID field + mitigation> | <doc §> |
| VP15 registry integrity | → profile (multisig/time-lock/transparency/revocation) | profile |
| VP16 cross-vendor delta | <this platform's trust set — silicon vendor / cloud hypervisor / NRAS> | §1 |

## 5. On-chain verification (Sei)

The cost + path on Sei EVM: direct (with the precompile that applies), amortized (the secp256k1 / verify-once pattern), or ZK-proven — with the gas figure and the production posture. Cite the cost research (`<doc>.md` §on-chain + `trusted-execution-on-sei.md` decision-driver).

## 6. Key-release / integration pattern

The platform's idiomatic secret-release / key-binding pattern (VP6) — e.g. attested-KMS condition keys, or the verify-once-then-bind amortization pattern. The path that makes "secret unsealed only to a known-good image" correct-by-construction on this platform.

## 7. Citations

The research doc sections + the primary sources (vendor spec, RFC, reference verifier implementation) the kit's claims rest on. Cite the research doc by **repo-root-relative path as text** — `` `design/research/tee/<doc>.md` `` — **not** a markdown link (a working link from `references/` would need `../../../../`, which is brittle; cite the path as text and let the reader resolve from repo root). External primary sources cite by URL.
```

---

## Notes for kit authors

- **Cite, never paraphrase.** The research doc is the record; the kit is the opinionated checklist over it. A claim with no citation to the research / a spec / an RFC does not go in the kit.
- **Mark N/A dimensions explicitly** in §4 (e.g. "VP11 BadRAM — N/A, AMD-specific") so the reviewer cites the kit for "vetted, not applicable" rather than fabricating a field.
- **The §4 table is the pluggability mechanism** — it is how this platform fills `method.md`'s cross-cutting dimensions. Keep the `method dimension` IDs verbatim so the method ↔ kit mapping stays mechanical.
- **Non-attester / verification-layer kits may adapt the section shape** — a kit for a layer that is not an Attester (e.g. the on-chain Verifier) can repurpose §1/§2/§3/§6, but must mark each deviation with an explicit **`Template-fit note:`** so neither reader mistakes a deliberate adaptation for a gap, and must still address every VP1–VP16 in §4. See `kit-sei-onchain.md` for the worked adaptation.
