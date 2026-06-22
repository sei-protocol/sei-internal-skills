# The method — designing & reviewing platform artifacts

Two modes, one spine: **design** (the platform-engineer authoring manifests/IaC) and **review** (a lens over existing platform config). Both load the profile + kit first, judge against the profile's hard conventions before the external canon, and rank one-way-door / security findings above style.

## The four stages

1. **Load.** `sei-platform-profile.md` (always first — its conventions override generic best-practice) + the kit(s) for the concern (GitOps, Kustomize, cloud-auth, secrets). If working *in* the platform repo, read its `README.md` + `.agent/runbooks/`; the live repo wins over this snapshot — flag drift.
2. **Read the whole target.** For review: the `kustomization.yaml`(s) + the resources/patches/components they pull, the HelmReleases, the `.sops.yaml` + any secrets, the Pod-Identity TF, the namespace PSS labels/VAP. For design: the cell's existing layout + the base it overlays. Never review platform config from a generic mental model — this fleet has specific patterns (Pod-Identity, SOPS, two-layer Kustomize) a default reading gets wrong.
3. **Apply the six dimensions** (below), profile-first. Each finding cites a `sources.md` anchor and/or a profile rule. Flag a genuinely-uncertain call rather than forcing it.
4. **Rank + surface.** One-way doors (prod cell, cloud-identity scope, KMS/SOPS boundary, Cilium cluster.id / pod CIDR, wire/secret format) and security defects lead; structure/style is bundled. Flag one-way doors for human approval — never assert the irreversible change as the fix.

## The six dimensions (the scorecard)

Grounded in the external canon (`sources.md`) and specialized by the profile.

1. **Security posture & least-privilege.** Non-root, `readOnlyRootFilesystem`, dropped capabilities, no privilege escalation; PSS `restricted` enforced + version-pinned + the CEL ValidatingAdmissionPolicy for the vectors PSS misses. **RBAC cue (a primary control on a restricted-PSS fleet, not a sub-bullet):** Role/ClusterRole verbs+resources minimized — no wildcard `*` verb/resource/apiGroup, no cluster-wide `ClusterRoleBinding` where a namespaced `RoleBinding` suffices, no `escalate`/`bind`/`impersonate` unless justified. *Basis:* `sources.md` §nsa-cisa, §pod-security; profile §5.

2. **Secrets handling.** Externalized correctly — **SOPS-in-git + per-cell KMS** here (not plaintext, not a generic CSI SecretProviderClass); `encrypted_regex` covers `data`/`stringData`; encrypted from inside the cell dir (right key); no secret material in a non-encrypted file or a ConfigMap. *Basis:* `sources.md` §secrets; profile §4.

3. **GitOps-reconcilability.** Fully declarative, no imperative/mutating step, no live drift; reconcilable by Flux (pull-based); ordering expressed via resource-order + HelmRelease `dependsOn` (not `spec.dependsOn` between Kustomizations); `prune`/`interval`/SOPS-decryption set. **Ordering-convergence cue:** the fleet uses **no Flux `healthChecks`/`wait:`** on Kustomizations — a new cross-component dependency relies entirely on intra-kustomization resource-order + HelmRelease `dependsOn` to sequence. Check that a newly-introduced dependency *actually* sequences (the producer is ordered before / `dependsOn`'d by the consumer) rather than racing on the next reconcile. *Basis:* `sources.md` §opengitops; profile §1.

4. **Multi-env / cell structure.** Clean two-layer Kustomize (`clusters/base` + `manifests/base`); per-cell difference via `patches`/`components`/`replacements`, not forked copies and not `postBuild.substitute`; the hub-vs-producer-cell split (Grafana/Thanos) and harbor multi-tenancy (`replacements` + 2nd GitRepo) respected; the `?ref=` staged controller pin honored. *Basis:* `sources.md` §kustomize; profile §2, §7.

5. **Supply-chain integrity.** Minimal/pinned base images (the static-musl seid build), digest/tag pinning, no `:latest`; provenance via ECR; the in-cluster GHA runner trust scope. *Basis:* `sources.md` §nsa-cisa; profile §8.

6. **Cloud-identity boundary.** **EKS Pod Identity** scoped per (cluster, namespace, ServiceAccount) with a tight association as the default (an IRSA OIDC `role-arn` annotation is the documented exception — fine *with* the old-SDK justification, flag an unjustified reflex one); session-tag S3 scoping; no node-role credential bleed / IMDS exposure; external-dns Deny-on-Condition IAM intact. *Basis:* `sources.md` §irsa (the generic the profile defaults away from), §nsa-cisa; profile §3.

## Design discipline (when authoring, not just reviewing)

- Argue the **maximum scope you'd defend** in the platform domain, then name what you'd **cut first** for an MVP and the condition that un-defers it — the orchestrator/human picks the minimum (the established agent output-discipline).
- A change touching a **prod cell, a Pod-Identity/IAM trust scope, a KMS/SOPS key boundary, a Cilium `cluster.id` / shared pod CIDR, or a published wire/secret format** is a one-way door — design it as if you can't take it back, and flag it for human approval before finalizing.
- Reach for the profile's pattern over a generic one: cloud access is a Pod-Identity association (not an IRSA annotation); a secret is SOPS-encrypted in the cell dir (not a CSI SecretProviderClass); per-cell variance is a `patch`/`replacement` (not `postBuild.substitute`).
