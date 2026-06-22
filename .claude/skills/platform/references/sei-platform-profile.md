# Sei-platform profile (always-first overlay)

Loaded before any design or review. It encodes the **Sei platform fleet's own enforced conventions** and **overrides generic Kubernetes/AWS best-practice** in either direction — the way `/idiomatic`'s repo profile outranks generic idiom. Distilled (and cited) from the `platform` repo (`/tmp/sei-arch/platform` at research time) + `sei-infra`.

**Snapshot caveat:** a portable distillation for review and for platform work elsewhere. When you are working *inside* the `platform` repo, its live `README.md` + `.agent/runbooks/` (esp. `cell-bootstrap.md`) are the authority — read them and flag drift, don't follow a stale copy.

## The fleet in one paragraph

A **multi-cell EKS + Flux-CD GitOps fleet** at K8s 1.34, single AWS account `189176372795`. Cells: `prod` (eu-central-1 hub — Grafana + Thanos query, **VPC-CNI**), `prod-euw1`, `prod-use2` (producer cells — **Cilium**, `grafana.enabled:false`, federate up), `harbor` (multi-tenant dev/staging), `dev`. Secrets are **SOPS-in-git**; cloud-auth is **EKS Pod Identity**. The companion repo **`sei-infra`** is a *separate, legacy EC2-based* fleet (terraform + `user_data` + SSH keys) — **read-mostly**; the EKS fleet is the migration target. The current `platform-engineer` agent's expertise (Python images, LLM API, EIP-712 KMS signing, GitHub-App JWT, IRSA, Secrets-Manager/CSI) is **stale and wrong for this fleet** — see the corrections below.

## Hard conventions (these override generic habit)

1. **GitOps via Flux v2 — the reconcile spine.** A `GitRepository(main)` → a root `Kustomization` per `clusters/<cell>`, with `prune: true`, `interval: 10m`, and `decryption: { provider: sops }`. **There is no `spec.dependsOn` between Kustomization CRDs** — ordering is (a) resource order within a `kustomization.yaml`, then (b) HelmRelease-level `dependsOn` (e.g. karpenter `dependsOn` cilium). *Cited:* `clusters/prod/flux-system/gotk-sync.yaml`, `clusters/base/cni-cilium/kustomization.yaml`.

2. **Two-layer Kustomize composition (plain Kustomize, not classic overlays).** `clusters/base/<infra-component>` (cluster services) vs `manifests/base/<workload>` (seid/waterway/monitoring/genesis). A cell dir overlays via `resources: [../../base/X]` + **`patches:`** + **`components:`**, and per-cell variance is `configMapGenerator` + **`replacements`** — **never `postBuild.substitute`** (zero repo use). *Cited:* `clusters/prod/cert-manager/kustomization.yaml`, `clusters/base/cni-cilium/`.

3. **Cloud-auth is EKS Pod Identity — NOT IRSA.** The `eks-pod-identity-agent` addon on every cluster; per-workload `terraform-aws-modules/eks-pod-identity/aws` associations scoped to (cluster, namespace, ServiceAccount). **No OIDC `eks.amazonaws.com/role-arn` SA annotations.** S3 access is scoped by session tag (`aws:PrincipalTag/kubernetes-namespace`). *Cited:* `terraform/aws/189176372795/eu-west-1/prod-euw1/*.tf`, `cell-bootstrap.md` Appendix A. **(Generic best-practice says IRSA; this fleet overrides it with Pod Identity — `sources.md` §IRSA is the generic floor, this is what actually applies.)**

4. **Secrets are SOPS-in-git + per-cell KMS — NOT Secrets-Manager / CSI.** One KMS CMK + alias per cell (`alias/<cell>`); each `clusters/<cell>/.sops.yaml` has ONE `creation_rules` entry (`path_regex: .*`, `encrypted_regex: ^(data|stringData|.+=)$`) → that cell's KMS ARN; Flux's kustomize-controller decrypts. **Encrypt from *inside* the cell dir** (the `.sops.yaml` is CWD-resolved — encrypting elsewhere picks the wrong key). SOPS-wrapped validator signing keys + pagerduty keys live in-tree. **No External Secrets Operator, no Sealed Secrets, no Secrets Store CSI / SecretProviderClass, no AWS Secrets Manager.** *Cited:* `clusters/prod/.sops.yaml`, `cell-bootstrap.md` Appendix B. **(Generic best-practice says CSI/SecretProviderClass; this fleet overrides it — `sources.md` §secrets is the generic floor.)**

5. **Pod Security = PSS `restricted`, version-pinned, + a CEL ValidatingAdmissionPolicy.** Namespaces pin `enforce/audit/warn: restricted` at the cluster minor (`v1.34`); a `ValidatingAdmissionPolicy` (in-tree GA, CEL) adds what PSS misses — bars ephemeral containers + enforces a container-name allowlist (the `kubectl debug` / sidecar-injection netns vector). **NetworkPolicy is enforced only on Cilium cells** — on the VPC-CNI hub it's documentation-grade, so PSS + VAP is the real lock. *Cited:* `clusters/prod/walle/namespace.yaml`, `walle/podcomposition-policy.yaml`.

6. **CNI is split: Cilium (producer/harbor) vs VPC-CNI (prod hub).** Cilium cells use kubeProxyReplacement, cluster-pool IPAM (`100.64.0.0/14`), VXLAN UDP/8472, and a **hand-allocated `cluster.id`** per cell (registry in `cell-bootstrap.md` Phase 0). The Cilium overlay creates the **cilium#30111** apiserver→pod-CIDR dial failure — metrics-server / istiod need `hostNetwork`; NLBs fronting pods must be **`target-type: instance`**. *Cited:* `clusters/base/cni-cilium/cilium.yaml`, `clusters/base/kube-system/metrics-server.yaml`, `.agent/runbooks/cilium.md`.

7. **sei-k8s-controller is deployed here, staged via `?ref=` pinning.** `clusters/base/sei-k8s-controller/` references the controller repo at one `?ref=<sha>`; a prod cell pins a **newer** sha for staged rollout; config via `manager-patch.yaml` env + a `controller-config.yaml` ConfigMap. **This skill owns that manifest/patch/config; `/kubernetes` owns the controller *code*.** *Cited:* `clusters/base/sei-k8s-controller/` vs `clusters/prod/sei-k8s-controller/`.

8. **Container runtime: static-musl `seid`.** `containers/seid-musl/` is a multi-stage Alpine **static musl** link of `seid`/`price-feeder`/`seictl` (pulling `libwasmvm*_muslc.a`); `containers/genesis/` pre-bakes multi-node genesis at build; `containers/actions-runner/` is the in-cluster self-hosted GHA runner. Images → ECR `189176372795.dkr.ecr.us-east-2.amazonaws.com/sei/*`. `waterway` is the EVM JSON-RPC/WS reverse proxy. *Cited:* `containers/*/Dockerfile`, `manifests/base/{seid,waterway}/`.

## The agent boundary (confirmed from the repos)

- **platform-engineer** (this skill): HelmRelease plumbing + `valuesFrom`/version-pin, Kustomize structure, Pod-Identity/IRSA→PodIdentity wiring, SOPS/KMS, the controller-deploy manifests, terraform cell provisioning, container builds.
- **observability-platform-engineer**: the telemetry-stack *values' contents* + PromQL/LogQL (you own the HelmRelease shell + `valuesFrom`, they own what's inside).
- **k8s-capacity-management**: NodePool sizing, requests/limits, scheduling.
- **kubernetes-specialist** (`/kubernetes`): the sei-k8s-controller *code* + CRDs (you deploy it; they author it).
- **network-specialist**: NetworkPolicy/CiliumNetworkPolicy intent + the Cilium datapath design (you own the CNI bring-up + hostNetwork/NLB plumbing). **sei-network-specialist**: Sei node P2P/RPC.
- **sre-engineer**: SLOs/alerts/runbooks/incidents on the running system.

## Stale-agent corrections (drop these from the old persona)

Confirmed *absent* from both repos — do **not** carry them: Python container images (runtimes are static-musl Go); LLM/Anthropic API integration; EIP-712 KMS transaction signing (KMS here = SOPS decryption + EKS secrets-at-rest, not signing); GitHub-App JWT auth (git auth is Flux deploy keys); **IRSA** (it's Pod Identity); **AWS Secrets Manager + CSI SecretProviderClass** (it's SOPS-in-git + KMS).
