---
name: platform
category: platform-infra
model: claude-opus-5
description: "Use when designing or reviewing platform/infra artifacts — Kustomize manifests, Flux GitOps, EKS cloud-auth, secrets, Pod Security, terraform cell provisioning — especially the Sei platform fleet: '/platform', 'review this manifest', 'is this GitOps-reconcilable', 'how should this secret be handled', 'wire cloud identity for X', 'design the cell overlay'. A citable corpus (OpenGitOps, Kustomize, Pod Security Standards, EKS, NSA/CISA hardening) + an always-first Sei-platform profile (Flux GitOps, two-layer Kustomize, EKS Pod Identity, SOPS + per-cell KMS, PSS + CEL VAP, Cilium/VPC-CNI) + pluggable kits. Backs the platform-engineer agent. NOT controller/CRD code (/kubernetes); NOT right-sizing/Karpenter/HPA (k8s-capacity-management); NOT telemetry-stack values/PromQL (observability agents); NOT node P2P/RPC (sei-network-specialist); NOT SLO/alerts/runbooks (sre-engineer). Designs/reviews the platform; doesn't operate it."
---

# Platform

Design and review the **platform layer** — the GitOps-reconciled Kustomize manifests, EKS cloud-auth, secrets, Pod Security posture, and terraform that stand up and run the Sei fleet — so it is secure, reconcilable, and consistent across cells. A *reference/technique* skill with a discipline spine. It is the operating manual for the `platform-engineer` agent and is directly invocable (`/platform <target>`).

## Why this skill exists

A capable model knows generic Kubernetes + AWS. The skill's job is the **citable corpus** (the specific standard + source) plus the **always-first Sei-platform profile** — the fleet's real, non-obvious conventions that *override* generic habit, the way `/idiomatic`'s repo profile outranks generic idiom. The failure mode it prevents: applying textbook defaults that are *wrong here* — reaching for IRSA when the fleet uses **EKS Pod Identity**, for a Secrets Store CSI `SecretProviderClass` when secrets are **SOPS-in-git + per-cell KMS**, for `postBuild.substitute` when per-cell variance is **`patches`/`components`/`replacements`**, or assuming NetworkPolicy is enforced on a VPC-CNI cell where it is documentation-grade.

The corpus is grounded in primary sources (`references/sources.md`) and stays copyright-clean: our-own-words checklists that cite, never reproduce.

## Guardrails

Refusal conditions — they hold under time pressure and a "just ship the manifest" urge:

1. **Profile- and kit-first.** Load `references/sei-platform-profile.md` (the always-first overlay — it encodes the fleet's hard conventions and **overrides generic best-practice**) **and** the relevant kit before designing or reviewing. When working *in* the platform repo, read its `README.md` / `.agent/runbooks/` — the live repo wins over this skill's snapshot; flag drift.
2. **Cite every finding; stay copyright-clean.** A primary source (`sources.md`) and/or a profile rule per finding — never a naked "this isn't secure." The generic external standard is the floor; the Sei profile is what *actually* applies here, and it overrides the generic where they differ (e.g. Pod-Identity over IRSA).
3. **Suggest-when-reviewing; author-when-building.** As a review lens, produce findings the human/calling agent applies. As `platform-engineer` building manifests, write them — but flag one-way doors (below) for human approval before finalizing.
4. **Prod / cloud-identity / secrets / CNI changes are one-way doors.** A change touching a prod cell, a Pod-Identity/IAM trust scope, a KMS/SOPS key boundary, a Cilium `cluster.id` / shared pod CIDR, or a published wire/secret format is irreversible or blast-radius-wide — flag for human approval; never assert it as the fix. (Council's one-way-door gate.)
5. **Don't duplicate the adjacent lenses.** Controller/CRD code → `/kubernetes`. Capacity/scheduling (requests/limits, Karpenter NodePool, HPA) → `k8s-capacity-management`. Telemetry-stack *values*/PromQL → the observability agents (you own the HelmRelease plumbing + `valuesFrom`, they own the values' contents). NetworkPolicy *intent* / the Cilium datapath design → `network-specialist`; Sei node P2P/RPC → `sei-network-specialist`. SLOs/alerts/runbooks/incidents → `sre-engineer`. This skill is the *platform manifests, GitOps, cloud-auth, secrets, and IaC*.

## The method

`references/method.md` holds the full method; the spine:

1. **Load the profile + the kit(s)** for the concern (GitOps, Kustomize composition, cloud-auth, secrets). Read the platform repo's runbooks if working in it.
2. **Design or review against the profile first, the external canon second.** The profile (Pod-Identity, SOPS, two-layer Kustomize, PSS+VAP, Cilium/VPC-CNI) is what applies here; `sources.md` is the generic floor the profile sits on and sometimes overrides.
3. **Score/identify by the six platform dimensions** (`method.md`): security posture & least-privilege · secrets handling · GitOps-reconcilability · multi-env/cell structure · supply-chain integrity · cloud-identity boundary.
4. **Cite every finding** and rank one-way-door / security findings above style. Flag prod/identity/secrets/CNI one-way doors for human approval.

## Kit index

| Concern | Kit |
|---|---|
| Flux GitOps — GitRepository → Kustomization per cell, reconcile ordering, SOPS decryption | `references/kit-gitops-flux.md` |
| Kustomize composition — the two-layer `clusters/base` + `manifests/base` model, patches/components/replacements | `references/kit-kustomize-composition.md` |
| Cloud auth — EKS **Pod Identity** default (IRSA = documented old-SDK exception), per-workload TF associations, session-tag scoping | `references/kit-cloud-auth-pod-identity.md` |
| Secrets — **SOPS-in-git + per-cell KMS** delivery (not CSI/ESO/Sealed), the encrypt-from-cell-dir footgun | `references/kit-secrets-sops-kms.md` |
| Pod Security — PSS `restricted` (version-pinned) + the CEL ValidatingAdmissionPolicy for the vectors PSS misses | `references/kit-pod-security-vap.md` |
| `cell-networking`, `terraform-cell-provisioning`, `container-runtime`, `legacy-ec2-sei-infra` | *(deferred — see `references/kit-TEMPLATE.md` roster; add by use)* |

## How the platform-engineer agent hooks in

The `platform-engineer` persona's first step loads `sei-platform-profile.md` + the kit for the work, then designs or reviews against the profile first. The agent owns the manifest plumbing / IaC; `/kubernetes` owns the controller *code* it deploys, the observability agents own the telemetry *values*, and `sre-engineer` operates the running system.

## Halt conditions

- **No target** to design/review — ask for the manifest/IaC/repo; never review platform config from memory.
- **A one-way door** (prod cell, cloud-identity/IAM scope, KMS/SOPS boundary, Cilium cluster.id / pod CIDR, wire/secret format) — flag for human approval, don't assert.
- **The work is really another lens** — controller code (`/kubernetes`), capacity (`k8s-capacity-management`), telemetry values (observability agents), network intent (`network-specialist`/`sei-network-specialist`), or operating the system (`sre-engineer`) — redirect.

## What this skill defers

The deferred kits in `references/kit-TEMPLATE.md`'s roster (`cell-networking`, `terraform-cell-provisioning`, `container-runtime`, `legacy-ec2-sei-infra`) — add by use. The `legacy-ec2-sei-infra` surface (`sei-infra`) is **read-mostly** (the EKS fleet is the migration target). The Sei-platform profile is a *snapshot* of the platform repo's conventions — when working there, its live runbooks are authoritative.
