# Harbor cluster — operating context

Cluster facts the skill assumes.

Last verified: 2026-04-26 against `clusters/harbor/` in sei-protocol/platform.

## At a glance

| Property | Value |
|---|---|
| Region | eu-central-1 (AWS) |
| Cluster name | `harbor` |
| Purpose | Perf-validation; runs autobake (nightly chaos/load) + sei-k8s-controller |
| Auth | AWS EKS access entries → Kubernetes groups (IAM-driven, not OIDC connector) |
| CNI | Cilium (kube-proxy replacement, eBPF socket-LB) |
| Service mesh | Istio v1.22.1+ with Gateway API; **PERMISSIVE** mTLS by default |
| DNS suffix | `*.harbor.platform.sei.io` |
| GitOps | Flux v2.7.3 from `sei-protocol/platform` path `clusters/harbor`, 3-min sync |
| Karpenter | NodePool tainted `CriticalAddonsOnly=true:NoSchedule` for platform; user pool TBD with cells |

## Namespaces (today)

[outline]

| Namespace | Purpose | Source |
|---|---|---|
| `flux-system` | Flux controllers | bootstrapped |
| `kube-system` | core | EKS managed |
| `cert-manager` | TLS certificate issuance | Flux |
| `external-dns` | DNS record management | Flux |
| `gateway` | Istio Gateway + HTTPRoutes | Flux |
| `istio-system` | Istio control plane | Flux |
| `monitoring` | Prometheus, kube-state-metrics | Flux |
| `sei-k8s-controller` | The controller manager | Flux |
| `autobake` | Nightly load-test workload | Flux + GHA-driven applies |
| `eng-<alias>` | Per-engineer cells | Flux (this skill) |

## Key prior art the skill builds on

- **`clusters/harbor/autobake/`** — namespace + RBAC + ServiceAccounts pattern for a workload-scoped cell
- **`clusters/harbor/autobake/`** + `autobake/templates/` (repo root) — the SeiNodeDeployment + seiload Job templates we render benchmarks from
- **`terraform/aws/189176372795/eu-central-1/harbor/autobake.tf`** — Pod Identity association pattern for `bench-seiload` SAs
- **`.github/workflows/k8s_autobake.yml`** — the imperative apply orchestration, including image-digest pinning and S3 result keying

## sei-k8s-controller config (the manager-patch)

[outline of env vars from `clusters/harbor/sei-k8s-controller/manager-patch.yaml`]

```
SEI_GATEWAY_NAME              sei-gateway
SEI_GATEWAY_NAMESPACE         gateway
SEI_GATEWAY_PUBLIC_DOMAIN     platform.sei.io
SEI_SNAPSHOT_BUCKET           harbor-sei-snapshots
SEI_SNAPSHOT_REGION           eu-central-1
SEI_RESULT_EXPORT_BUCKET      harbor-sei-shadow-results
SEI_RESULT_EXPORT_PREFIX      shadow-results/
SEI_GENESIS_BUCKET            harbor-sei-k8s-genesis-artifacts
```

Note: the skill's benchmark results bucket is `harbor-sei-autobake-results` (defined in autobake.tf, separate from the controller's shadow-results bucket).

## What's missing today (cells will fix)

- No default-deny `NetworkPolicy` (or `CiliumNetworkPolicy`) at the namespace level
- No cluster-wide `ResourceQuota` or `LimitRange`
- No admission policies enforcing PSS-restricted, IRSA-self-grant denial, or HTTPRoute hostname pinning

The interim namespace strategy compensates with labels (`tide.sei.io/cell-type=personal`, `tide.sei.io/owner=<alias>`) so cells can layer enforcement on top without changing the engineer's workflow. See `interim-namespace-strategy.md`.
