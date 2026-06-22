# Platform-concern kit (TEMPLATE)

A kit is **data** the method loads for one platform concern (GitOps, Kustomize composition, cloud-auth, secrets, …). It teaches the pattern this fleet actually uses, cites the external canon beneath it, and gives review cues + the failure modes to catch. Adding a concern = drop one file conforming to this template at `references/kit-<concern>.md`.

Each kit provides the five sections below, in order, so the method stays concern-agnostic. Copy the skeleton; see `kit-cloud-auth-pod-identity.md` for a worked kit.

This section schema is a **soft one-way door** — changing it churns every kit. Revise deliberately.

---

```markdown
# <Concern> kit

## 1. What this concern is
One paragraph: the pattern as the Sei fleet actually does it, and what generic
mental model gets it wrong here (the override).

## 2. The pattern (how this fleet does it)
The concrete shape — the files, the resources, the convention — cited to the repo
(file path) and to the external canon (`sources.md` §anchor). "Do it this way."

## 3. Anti-patterns / failure modes
Named smells with a detection cue and the correct rewrite — the generic habits
that are wrong here (e.g. an IRSA role-arn annotation; a CSI SecretProviderClass;
postBuild.substitute; a plaintext secret).

## 4. Review cues
What a reviewer looks for, mapped to the method's six dimensions. Cite the profile
rule / `sources.md` anchor each cue rests on.

## 5. One-way doors in this concern
The irreversible / blast-radius-wide decisions (prod cell, cloud-identity scope,
KMS/SOPS boundary, Cilium cluster.id / pod CIDR, wire/secret format) that must be
flagged for human approval, not asserted.
```

---

**Authoring rules:**
- **Cite both layers:** the repo pattern (a file path in `platform`/`sei-infra`) AND the external canon (`sources.md`) it specializes or overrides. A claim with neither is not a kit entry.
- The **profile** (`sei-platform-profile.md`) holds the cross-cutting hard conventions — kits reference it, don't restate it.
- Where the fleet **overrides** a generic standard (Pod-Identity over IRSA, SOPS over CSI), say so explicitly and cite the generic as the floor (`sources.md`).
- Keep review cues mapped to the six method dimensions so findings stay rankable. **Always write the dimension as `Dimension N (name)`** — keep the parenthetical name, never a bare `Dimension N`. The number→name map lives only in `method.md`, so a kit pulled into a windowed context must carry the name with it.

## Kit roster (shipped + deferred)

Shipped:
- `kit-gitops-flux.md` — Flux GitRepository → Kustomization per cell; reconcile ordering.
- `kit-kustomize-composition.md` — the two-layer `clusters/base` + `manifests/base` model.
- `kit-cloud-auth-pod-identity.md` — EKS Pod Identity default; IRSA as the old-SDK exception.
- `kit-secrets-sops-kms.md` — SOPS-in-git + per-cell KMS delivery (not CSI/ESO/Sealed).
- `kit-pod-security-vap.md` — PSS `restricted` + enforce-version pin + the CEL ValidatingAdmissionPolicy for the vectors PSS misses.

Deferred (add as a conforming kit when first encountered — the corpus grows by use):
- `kit-cell-networking` — Cilium-vs-VPC-CNI split, cluster-pool IPAM / `cluster.id` / VXLAN, cilium#30111 hostNetwork, NLB `target-type` (CNI-dependent: `ip` on VPC-CNI, `instance` on Cilium), cross-region peering. (Shares a seam with `network-specialist` — keep to the platform-plumbing side.)
- `kit-terraform-cell-provisioning` — the per-cell `<region>/<cell>/` TF file-set, EKS/Karpenter/Flux-bootstrap, the cell-bootstrap phase sequence.
- `kit-container-runtime` — the static-musl seid build (libwasmvm), genesis pre-bake, in-cluster GHA runner, seid StatefulSet entrypoint, waterway EVM proxy.
- `kit-legacy-ec2-sei-infra` — the EC2 validator/snapshotter/state-syncer ops, `keys/` rotation. **Read-mostly** — the EKS fleet is the migration target.
