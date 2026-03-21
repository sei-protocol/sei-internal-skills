# Milestone 1 — Platform & Operator

**Owner:** Kubernetes Specialist (Operator) + Platform Engineer (Manifests)
**Phase:** 0.7–2
**Dependencies:** M0 (contract addresses + ABIs)

## Scope

Stand up the Kubernetes runtime platform and the off-chain orchestration operator:

1. **K8s Platform Manifests** — Namespaces (`tide-system`, `tide-agents`), RBAC with per-agent ServiceAccounts/IRSA, ResourceQuotas, NetworkPolicies, SecretProviderClasses, Prometheus alerting
2. **Tide Operator** — Go binary with controller-runtime: CRD types (TideProposal, TideJob), Sei event indexer, reconciliation state machines, K8s Job generation with full env var wiring

## Deliverables

| Spec | Output |
|------|--------|
| `lld-k8s-manifests.md` | Kustomize base + overlays (testnet, mainnet) |
| `lld-tide-operator.md` | Go binary, CRD definitions, Helm chart or raw manifests |

## Done Criteria

- Manifests applied to EKS testnet cluster
- Operator indexing TideCouncil and TideJobHook events on arctic-1
- CRDs installed and reconciling
- Per-agent ServiceAccounts with IRSA verified
- NetworkPolicies blocking IMDS and private ranges confirmed
