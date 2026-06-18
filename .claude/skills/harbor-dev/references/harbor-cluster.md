# Harbor cluster — operating context

Cluster facts the skill assumes.

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
| Karpenter | NodePool tainted `CriticalAddonsOnly=true:NoSchedule` for platform |

## Namespaces

| Namespace | Purpose |
|---|---|
| `flux-system` | Flux controllers |
| `kube-system` | core (EKS managed) |
| `cert-manager` | TLS certificate issuance |
| `external-dns` | DNS record management |
| `gateway` | Istio Gateway + HTTPRoutes |
| `istio-system` | Istio control plane |
| `monitoring` | Prometheus, kube-state-metrics |
| `sei-k8s-controller` | The controller manager |
| `autobake` | Nightly load-test workload |
| `eng-<alias>` | Per-engineer namespaces (one per onboarded engineer) |

## sei-k8s-controller config

Manager-patch env vars (`clusters/harbor/sei-k8s-controller/manager-patch.yaml`):

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

The controller watches `SeiNetwork` and `SeiNode` cluster-wide.
