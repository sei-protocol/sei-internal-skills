# GitOps (Flux) kit

## 1. What this concern is

The fleet's reconcile spine is **Flux CD v2**: a `GitRepository(main)` feeds a root `Kustomization` per cell, which Flux continuously reconciles (pull-based, prune-on, SOPS-decrypting). The generic "kubectl apply / Helm install from CI" push model is wrong here — nothing is applied imperatively; the cluster converges to git. *Cited:* `clusters/prod/flux-system/gotk-sync.yaml`; `sources.md` §opengitops.

## 2. The pattern (how this fleet does it)

- **The topology.** One `GitRepository` (branch `main`) → one root `Kustomization` per `clusters/<cell>` with `prune: true`, `interval: 10m`, `decryption: { provider: sops }`. *Cited:* `clusters/prod/flux-system/`.
- **Ordering is NOT `spec.dependsOn` between Kustomizations.** Ordering is (a) resource order within a `kustomization.yaml`, then (b) **HelmRelease-level `dependsOn`** (e.g. karpenter's HelmRelease `dependsOn` cilium, patched in via the `cni-cilium` Component). There is essentially no inter-Kustomization `dependsOn` in the fleet. *Cited:* `clusters/base/cni-cilium/kustomization.yaml`.
- **HelmReleases** (`helm.toolkit.fluxcd.io/v2`) ship as inline HelmRelease + HelmRepository pairs with a pinned chart version and `valuesFrom` a ConfigMap — cert-manager, kube-prometheus-stack, Karpenter (OCI), Cilium, istiod, external-dns. **This skill owns the HelmRelease shell + version pin + `valuesFrom`; the observability agents own the values' *contents*.** *Cited:* `clusters/prod/monitoring/prometheus-operator.yaml`, `clusters/base/kube-system/karpenter.yaml`.
- **Splitting a Kustomization** uses a two-commit adopt-then-orphan cutover (so Flux doesn't prune mid-move). *Cited:* `cell-bootstrap.md`.

## 3. Anti-patterns / failure modes

- **Imperative apply.** A CI step doing `kubectl apply` / `helm install` / a mutating webhook instead of committing to git. Cue: an apply outside Flux. Rewrite: declare it; let Flux reconcile.
- **`spec.dependsOn` between Kustomization CRDs.** Reaching for inter-Kustomization ordering. Cue: a `dependsOn` on a Flux `Kustomization`. Rewrite: order resources within the kustomization, or use HelmRelease `dependsOn`.
- **Unpinned HelmRelease.** A chart without a pinned version. Cue: a floating/latest chart version. Rewrite: pin it (staged upgrades are a deliberate version bump).
- **Editing telemetry values here.** Changing PromQL/dashboards/ingester sizing in a values block. Cue: PromQL or stack-sizing edits. Rewrite: that's the observability agents' lane — you own the HelmRelease shell + `valuesFrom`, not the values' contents.
- **Disabling prune to "be safe."** Cue: `prune: false`. Rewrite: prune stays on (drift correction is the point); fix the manifest, not the guard.

## 4. Review cues

- **Dimension 3 (GitOps-reconcilability):** fully declarative; `prune`/`interval`/SOPS set; ordering via resource-order + HelmRelease `dependsOn` (not Kustomization `dependsOn`); no imperative step / live drift. *Basis:* profile §1, `sources.md` §opengitops.
- **Dimension 5 (supply-chain):** HelmRelease chart versions pinned; OCI/repo sources trusted. *Basis:* `sources.md` §nsa-cisa.

## 5. One-way doors in this concern

- **A change to a prod cell's root `Kustomization`** (prune scope, the GitRepository ref, decryption) reconciles fleet-wide on the next interval — flag for human approval.
- **A Kustomization split/move** that could prune live resources mid-cutover — use the adopt-then-orphan two-commit sequence; flag the cutover.
