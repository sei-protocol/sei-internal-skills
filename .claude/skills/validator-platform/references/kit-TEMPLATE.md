# Validator-platform concern kit (TEMPLATE)

A kit is **data** the method loads for one validator-platform concern (controller-behavior machinery, gov-manifest authoring, …). It teaches the platform's actual behavior for that concern, cites the corpus pin beneath it, and gives review cues + the failure modes to catch. Adding a concern = drop one file conforming to this template at `references/kit-<concern>.md`.

Each kit provides the five sections below, in order, so the method stays concern-agnostic. Copy the skeleton; see `kit-platform-machinery.md` for a worked kit.

This section schema is a **soft one-way door** — changing it churns every kit. Revise deliberately.

---

```markdown
# <Concern> kit

## 1. What this concern is
One paragraph: the platform's actual behavior for this concern, and what generic
CRD/gov mental model gets it wrong here (the override). Cite a corpus pin.

## 2. The pattern (how this platform does it)
The concrete shape — the kind, the payload, the file, the sidecar call — cited to
the corpus (`sources.md` §controller / §seictl / §gov-ops) AND, where it's
`/gov-ops`'s, cited to §gov-ops rather than restated. "Do it this way."

## 3. Anti-patterns / failure modes
Named smells with a detection cue and the correct rewrite — the generic habits
that are wrong here (e.g. delete-recreate to retry a submit; reading status.outputs;
an integer-as-JSON-number; a double-encoded param value; assuming node_admin signs).

## 4. Review cues
What a reviewer looks for, mapped to the method's five dimensions. Cite the profile
rule / `sources.md` pin each cue rests on. Always write `Dimension N (name)`.

## 5. One-way doors in this concern
The irreversible / blast-radius-wide decisions (a submit broadcast + deposit, a
mainnet-adjacent context, a delete-recreate on a non-idempotent kind) that must be
refused or flagged for human approval / routed to `/gov-ops`, not asserted.
```

---

**Authoring rules:**
- **Cite the pin:** the controller/seictl path-at-sha, the LLD contract, `/gov-ops`, or `sei-skill` (`sources.md`). A claim with no pin is not a kit entry.
- The **profile** (`sei-validator-profile.md`) holds the cross-cutting hard behavior — kits reference it, don't restate it.
- **Cite, never restate `/gov-ops`.** A fact `/gov-ops` owns (a gate, the GovVote fan-out template, the fee floor, the mainnet allowlist) is cited to §gov-ops — never re-written into a kit.
- **Never cite the LLD for topology** — it is STALE there; take topology/signing from §controller.
- Keep review cues mapped to the five method dimensions so findings stay rankable. **Always write the dimension as `Dimension N (name)`** — keep the parenthetical name, never a bare `Dimension N`. The number→name map lives only in `method.md`, so a kit pulled into a windowed context must carry the name with it.

## Kit roster (shipped + deferred)

Shipped:
- `kit-platform-machinery.md` — the canonical home for ALL controller-behavior invariants: ownership, sidecar execution at `:8443`, idempotency-per-kind, per-kind result location, task-ID re-join vs delete-recreate, `requirePhase` terminality, structural RPC pin.
- `kit-seinodetask-gov-manifests.md` — authoring + GitOps-applying the 3 gov kinds as per-node manifests: camelCase payloads, integer-as-JSON-string + param-struct-as-JSON-object traps, the keyring resolution ladder, the per-node fan-out (cite `/gov-ops` `fan-out.md` template + fee floor), poll/verify-on-chain.
- `kit-shadow-comparison.md` — driving the shadow `result-export` task in comparison mode against a node ALREADY running the supported shadow features, then reading results: the 9 params (`canonicalRpc` is the discriminator), the typed-client gap (the EVM/migration params need the RAW params map / `/v0/tasks` POST, not `ResultExportTask`), the L0/L1/L2 + touched-keys + watermark model, `migrationMode` (AppHash divergence expected), and reading S3 (`*.compare.ndjson.gz`, `divergence-{h}.report.json.gz`) + Prom (`seictl_shadow_*`). Node shadow-readiness is a precondition, not taught here.

Deferred (add as a conforming kit when first encountered — the corpus grows by use):
- `kit-gitops-networking` — the SeiNetwork/SeiNode + manual-NLB/HTTPRoute model (per-node networking, the cross-region NLB/HTTPRoute wiring). **Deferred — un-defer at M2 (`/harbor-dev`)**, which reuses `kit-platform-machinery` + this networking model. (Shares a seam with `/platform` cell-networking + `sei-network-specialist` — keep to the validator-platform operator side.)
