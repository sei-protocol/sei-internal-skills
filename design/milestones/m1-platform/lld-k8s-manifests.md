# Component: K8s Platform Manifests

**Date:** 2026-03-21
**Status:** Draft

---

## Owner

Platform Engineer

## Phase

Phase 0.7

## Purpose

The complete set of Kubernetes manifests that establish the runtime platform before any application code deploys. Provides namespace isolation, resource governance, RBAC, network segmentation, secret management, and observability primitives for the `tide-system` (control plane) and `tide-agents` (sandbox) namespaces on the Sei Platform EKS cluster.

**Business needs served:**
- #1, #3, #4, #5 — All workloads (review, execution, orchestrator) run on this platform. Without it, nothing deploys.

---

## Dependencies

### External Systems Consumed

| System | Interface | Notes |
|--------|-----------|-------|
| AWS EKS | Kubernetes API v1.28+ | Target cluster. Must have VPC CNI with NetworkPolicy support (Calico or native). |
| AWS Secrets Store CSI Driver (ASCP) | `secrets-store.csi.x-k8s.io/v1` CRD | Must be pre-installed on the cluster. These manifests create `SecretProviderClass` resources that consume it. |
| AWS Secrets Manager | HTTPS via IRSA | Stores GitHub App keys, API keys, contract ABIs. Accessed by CSI driver, not directly by pods. |
| AWS IAM (IRSA) | Pod identity via ServiceAccount annotations | ServiceAccounts annotated with IAM role ARNs for KMS and Secrets Manager access. |
| Prometheus Operator | `monitoring.coreos.com/v1` CRDs | Must be pre-installed. These manifests create `ServiceMonitor` and `PrometheusRule` resources. |

### Internal Tide Components Consumed

| Component | Interface | Notes |
|-----------|-----------|-------|
| Tide Operator (`lld-tide-operator.md`) | The Operator runs in `tide-system`. Its Deployment, ConfigMaps, and ServiceAccount are defined by the Operator team, not here. These manifests provide the namespace, RBAC, and NetworkPolicy it runs within. |
| Agent Review Runtime (`lld-agent-review-runtime.md`) | Runs as Jobs in `tide-agents`. These manifests provide the namespace, ResourceQuota, LimitRange, NetworkPolicy, and SecretProviderClass the Jobs consume. |
| Agent Execution Runtime (`lld-agent-execution-runtime.md`) | Same as review runtime — consumes `tide-agents` namespace resources. |

### Explicit Exclusions

- Does NOT include the Tide Operator Deployment, ConfigMap, or Service manifests (owned by the Operator team).
- Does NOT include application-level CRDs (e.g., `TideJob` CRD — owned by the Operator team).
- Does NOT include Flux/ArgoCD GitOps configuration (that's cluster infrastructure, not application manifests).
- Does NOT include the AWS Secrets Store CSI Driver installation itself (pre-requisite).
- Does NOT include the Prometheus Operator installation itself (pre-requisite).

---

## Interface Specification

### Kustomize Directory Structure

```
k8s/
├── base/
│   ├── kustomization.yaml
│   ├── namespaces/
│   │   ├── tide-system.yaml
│   │   └── tide-agents.yaml
│   ├── rbac/
│   │   ├── tide-system-sa.yaml
│   │   ├── tide-system-roles.yaml
│   │   ├── tide-agents-sa.yaml
│   │   └── tide-agents-roles.yaml
│   ├── resource-governance/
│   │   ├── tide-system-quota.yaml
│   │   ├── tide-system-limitrange.yaml
│   │   ├── tide-agents-quota.yaml
│   │   └── tide-agents-limitrange.yaml
│   ├── network/
│   │   ├── tide-system-default-deny.yaml
│   │   ├── tide-system-egress-allow.yaml
│   │   ├── tide-system-ingress-allow.yaml
│   │   ├── tide-agents-default-deny.yaml
│   │   └── tide-agents-egress-allow.yaml
│   ├── secrets/
│   │   ├── secretproviderclass-agent-alpha.yaml
│   │   ├── secretproviderclass-agent-beta.yaml
│   │   ├── secretproviderclass-agent-gamma.yaml
│   │   └── secretproviderclass-shared-config.yaml
│   └── monitoring/
│       ├── servicemonitor-orchestrator.yaml
│       ├── prometheusrule-tide.yaml
│       └── podmonitor-agents.yaml
├── overlays/
│   ├── testnet/
│   │   ├── kustomization.yaml
│   │   ├── namespace-labels-patch.yaml
│   │   ├── quota-patch-tide-system.yaml
│   │   ├── quota-patch-tide-agents.yaml
│   │   └── configmap-tide-platform.yaml
│   └── mainnet/
│       ├── kustomization.yaml
│       ├── namespace-labels-patch.yaml
│       ├── quota-patch-tide-system.yaml
│       ├── quota-patch-tide-agents.yaml
│       └── configmap-tide-platform.yaml
└── README.md
```

**Key design constraint:** Overlays change only ConfigMap values, resource limits/quotas, and labels. They do NOT add or remove structural resources (namespaces, RBAC, NetworkPolicies). A new agent's SecretProviderClass is added to `base/secrets/` — the overlays reference the same set of agents.

### Namespace Definitions

#### `tide-system`

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: tide-system
  labels:
    app.kubernetes.io/part-of: tide
    tide.sei.io/component: control-plane
    pod-security.kubernetes.io/enforce: baseline
    pod-security.kubernetes.io/enforce-version: latest
    pod-security.kubernetes.io/warn: restricted
    pod-security.kubernetes.io/warn-version: latest
```

PSS `baseline` enforcement for `tide-system` — the Orchestrator needs slightly more flexibility than `restricted` (e.g., it may need to run as a specific UID that isn't 1000). `restricted` is set as a warning to surface anything that could be tightened.

#### `tide-agents`

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: tide-agents
  labels:
    app.kubernetes.io/part-of: tide
    tide.sei.io/component: agent-sandbox
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/enforce-version: latest
```

PSS `restricted` enforcement for `tide-agents` — maximum security for agent workloads. Every agent Job pod MUST satisfy: `runAsNonRoot`, no privilege escalation, all capabilities dropped, seccomp RuntimeDefault, read-only root filesystem.

### RBAC

#### ServiceAccounts

```yaml
# tide-system/tide-orchestrator ServiceAccount
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tide-orchestrator
  namespace: tide-system
  labels:
    app.kubernetes.io/name: tide-orchestrator
    app.kubernetes.io/part-of: tide
  annotations:
    eks.amazonaws.com/role-arn: "arn:aws:iam::{ACCOUNT_ID}:role/tide-orchestrator-irsa"
automountServiceAccountToken: true
---
# tide-system/tide-secrets ServiceAccount (for CSI driver)
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tide-secrets
  namespace: tide-system
  labels:
    app.kubernetes.io/name: tide-secrets
    app.kubernetes.io/part-of: tide
  annotations:
    eks.amazonaws.com/role-arn: "arn:aws:iam::{ACCOUNT_ID}:role/tide-secrets-irsa"
automountServiceAccountToken: true
---
# tide-agents/tide-agent ServiceAccount (zero-privilege)
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tide-agent
  namespace: tide-agents
  labels:
    app.kubernetes.io/name: tide-agent
    app.kubernetes.io/part-of: tide
  annotations:
    eks.amazonaws.com/role-arn: "arn:aws:iam::{ACCOUNT_ID}:role/tide-agent-irsa"
automountServiceAccountToken: false
```

**IRSA Role Policies (defined in Terraform/CloudFormation, referenced here for completeness):**

| Role | Permissions |
|------|------------|
| `tide-orchestrator-irsa` | `kms:Sign`, `kms:GetPublicKey` on all agent KMS keys. `secretsmanager:GetSecretValue` on `tide/config/*`. `sts:AssumeRole` for cross-account if needed. |
| `tide-secrets-irsa` | `secretsmanager:GetSecretValue`, `secretsmanager:DescribeSecret` on `tide/agents/*` and `tide/config/*`. |
| `tide-agent-irsa` | `kms:Sign`, `kms:GetPublicKey` on agent's own KMS key only (scoped via `kms:ResourceAliases` condition or per-agent roles). |

**Note on per-agent KMS scoping:** The `tide-agent` ServiceAccount in `tide-agents` is shared across all agent Jobs. To scope KMS access per agent, each agent needs its own IAM role. Options:
- (a) One ServiceAccount per agent (`tide-agent-alpha`, `tide-agent-beta`, `tide-agent-gamma`), each with its own IRSA role.
- (b) Single ServiceAccount, but the agent's KMS key ID is passed via env var and the IRSA role allows access to all agent keys.

**Decision: Option (a) — one ServiceAccount per agent.** The blast radius of a compromised agent is limited to its own KMS key. The Operator sets the correct `serviceAccountName` when creating the Job.

> **Cross-review finding (2026-03-21):** The Operator's `buildExecutionJob()` currently hardcodes `ServiceAccountName: "tide-agent"` (a single shared SA). This is a mismatch — the Operator MUST use `fmt.Sprintf("tide-agent-%s", agent.Name)` to reference the per-agent ServiceAccounts defined below. Filed for the Operator team to fix. Without this, IRSA role scoping breaks and all agents share the same KMS permissions.

```yaml
# Per-agent ServiceAccount (one per agent, in tide-agents namespace)
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tide-agent-alpha
  namespace: tide-agents
  labels:
    app.kubernetes.io/name: tide-agent
    app.kubernetes.io/part-of: tide
    tide.sei.io/agent-id: alpha
  annotations:
    eks.amazonaws.com/role-arn: "arn:aws:iam::{ACCOUNT_ID}:role/tide-agent-alpha-irsa"
automountServiceAccountToken: false
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tide-agent-beta
  namespace: tide-agents
  labels:
    app.kubernetes.io/name: tide-agent
    app.kubernetes.io/part-of: tide
    tide.sei.io/agent-id: beta
  annotations:
    eks.amazonaws.com/role-arn: "arn:aws:iam::{ACCOUNT_ID}:role/tide-agent-beta-irsa"
automountServiceAccountToken: false
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tide-agent-gamma
  namespace: tide-agents
  labels:
    app.kubernetes.io/name: tide-agent
    app.kubernetes.io/part-of: tide
    tide.sei.io/agent-id: gamma
  annotations:
    eks.amazonaws.com/role-arn: "arn:aws:iam::{ACCOUNT_ID}:role/tide-agent-gamma-irsa"
automountServiceAccountToken: false
```

#### Roles and RoleBindings

```yaml
# Orchestrator needs to manage Jobs in tide-agents namespace
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: tide-orchestrator-jobs
  namespace: tide-agents
  labels:
    app.kubernetes.io/part-of: tide
rules:
  - apiGroups: ["batch"]
    resources: ["jobs"]
    verbs: ["create", "get", "list", "watch", "delete"]
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["pods/log"]
    verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: tide-orchestrator-jobs
  namespace: tide-agents
  labels:
    app.kubernetes.io/part-of: tide
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: tide-orchestrator-jobs
subjects:
  - kind: ServiceAccount
    name: tide-orchestrator
    namespace: tide-system
---
# Orchestrator needs ConfigMap access in its own namespace (event cursor)
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: tide-orchestrator-config
  namespace: tide-system
  labels:
    app.kubernetes.io/part-of: tide
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "create", "update"]
    resourceNames: ["tide-event-cursor"]
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["create"]
  - apiGroups: ["coordination.k8s.io"]
    resources: ["leases"]
    verbs: ["get", "create", "update"]
    resourceNames: ["tide-orchestrator-leader"]
  - apiGroups: ["coordination.k8s.io"]
    resources: ["leases"]
    verbs: ["create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: tide-orchestrator-config
  namespace: tide-system
  labels:
    app.kubernetes.io/part-of: tide
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: tide-orchestrator-config
subjects:
  - kind: ServiceAccount
    name: tide-orchestrator
    namespace: tide-system
```

**No ClusterRoles.** All RBAC is namespace-scoped. The Orchestrator has cross-namespace access via a RoleBinding in `tide-agents` that references the `tide-system` ServiceAccount — this is the standard pattern for controllers that manage resources in other namespaces.

### Resource Governance

#### `tide-system` ResourceQuota

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: tide-system-quota
  namespace: tide-system
  labels:
    app.kubernetes.io/part-of: tide
spec:
  hard:
    requests.cpu: "2"
    requests.memory: 4Gi
    limits.cpu: "4"
    limits.memory: 8Gi
    count/deployments.apps: "5"
    count/pods: "10"
    count/services: "5"
    count/configmaps: "20"
```

#### `tide-system` LimitRange

```yaml
apiVersion: v1
kind: LimitRange
metadata:
  name: tide-system-limitrange
  namespace: tide-system
  labels:
    app.kubernetes.io/part-of: tide
spec:
  limits:
    - type: Container
      default:
        cpu: "500m"
        memory: 1Gi
      defaultRequest:
        cpu: "250m"
        memory: 512Mi
      max:
        cpu: "2"
        memory: 4Gi
      min:
        cpu: "50m"
        memory: 64Mi
```

#### `tide-agents` ResourceQuota

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: tide-agents-quota
  namespace: tide-agents
  labels:
    app.kubernetes.io/part-of: tide
spec:
  hard:
    requests.cpu: "8"
    requests.memory: 16Gi
    limits.cpu: "16"
    limits.memory: 32Gi
    count/jobs.batch: "20"
    count/pods: "20"
    requests.ephemeral-storage: 100Gi
```

#### `tide-agents` LimitRange

```yaml
apiVersion: v1
kind: LimitRange
metadata:
  name: tide-agents-limitrange
  namespace: tide-agents
  labels:
    app.kubernetes.io/part-of: tide
spec:
  limits:
    - type: Container
      default:
        cpu: "2"
        memory: 4Gi
      defaultRequest:
        cpu: "500m"
        memory: 1Gi
      max:
        cpu: "4"
        memory: 8Gi
      min:
        cpu: "100m"
        memory: 256Mi
    - type: Pod
      max:
        cpu: "5"
        memory: 10Gi
```

### Network Policies

#### `tide-agents`: Default Deny

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: tide-agents
  labels:
    app.kubernetes.io/part-of: tide
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
```

#### `tide-agents`: Egress Allowlist

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: agent-egress-allow
  namespace: tide-agents
  labels:
    app.kubernetes.io/part-of: tide
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/component: agent
  policyTypes:
    - Egress
  egress:
    # DNS resolution via kube-dns
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
      ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53
    # HTTPS egress to external services:
    # GitHub API, Anthropic API, Sei RPC, AWS KMS/STS endpoints
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
            except:
              - 169.254.169.254/32   # EC2 IMDS — blocked
              - 10.0.0.0/8           # VPC internal — blocked
              - 172.16.0.0/12        # Private ranges — blocked
              - 192.168.0.0/16       # Private ranges — blocked
      ports:
        - protocol: TCP
          port: 443
```

**Blocked ranges explained:**
- `169.254.169.254/32` — EC2 Instance Metadata Service. If reachable, a compromised agent pod could obtain the node's IAM role credentials, which may have broader permissions than the pod's IRSA role.
- `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16` — Private RFC 1918 ranges. Blocks agents from probing internal cluster services, other namespaces' pods, or VPC-internal resources.
- Only port `443` (HTTPS) is allowed. No HTTP, SSH, or other protocols.

**Note:** If the VPC CIDR is not within `10.0.0.0/8` (e.g., it uses `172.16.x.x`), adjust the `except` blocks accordingly. The overlay can patch this.

#### `tide-system`: Default Deny

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: tide-system
  labels:
    app.kubernetes.io/part-of: tide
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
```

#### `tide-system`: Orchestrator Egress

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: orchestrator-egress-allow
  namespace: tide-system
  labels:
    app.kubernetes.io/part-of: tide
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: tide-orchestrator
  policyTypes:
    - Egress
  egress:
    # DNS
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
      ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53
    # K8s API server (for Job management, leader election)
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
      ports:
        - protocol: TCP
          port: 443
    # Sei RPC, GitHub API, AWS APIs are all HTTPS on 443
```

#### `tide-system`: Orchestrator Ingress (Prometheus Scrape)

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: orchestrator-ingress-prometheus
  namespace: tide-system
  labels:
    app.kubernetes.io/part-of: tide
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: tide-orchestrator
  policyTypes:
    - Ingress
  ingress:
    # Prometheus scraping /metrics
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: monitoring
      ports:
        - protocol: TCP
          port: 8080
```

### Secret Management (SecretProviderClass)

One SecretProviderClass per agent, plus one for shared config. These are templates — the `{ACCOUNT_ID}` and `{REGION}` placeholders are filled by the overlay.

#### Per-Agent SecretProviderClass (template for Alpha)

```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: tide-agent-alpha-secrets
  namespace: tide-agents
  labels:
    app.kubernetes.io/part-of: tide
    tide.sei.io/agent-id: alpha
spec:
  provider: aws
  parameters:
    objects: |
      - objectName: "tide/agents/alpha/github-app-key"
        objectType: "secretsmanager"
        objectAlias: "github-app-key.pem"
      - objectName: "tide/agents/alpha/anthropic-api-key"
        objectType: "secretsmanager"
        objectAlias: "anthropic-api-key"
      - objectName: "tide/agents/alpha/system-prompt"
        objectType: "secretsmanager"
        objectAlias: "agent-system-prompt.txt"
      - objectName: "tide/agents/alpha/execution-system-prompt"
        objectType: "secretsmanager"
        objectAlias: "agent-execution-system-prompt.txt"
      - objectName: "tide/config/tide-council-abi"
        objectType: "secretsmanager"
        objectAlias: "tide-council-abi.json"
      - objectName: "tide/config/acp-abi"
        objectType: "secretsmanager"
        objectAlias: "acp-abi.json"
```

Beta and Gamma follow the same pattern with `alpha` replaced by `beta`/`gamma`.

> **Cross-review finding (2026-03-21) — RESOLVED:** The Operator's `/secrets` layout has been updated to list all six files: `github-app-key.pem`, `anthropic-api-key`, `agent-system-prompt.txt`, `agent-execution-system-prompt.txt`, `tide-council-abi.json`, `acp-abi.json`. The Operator references per-agent SPCs via `agent.SecretProviderClass`.

**Adding a new agent:** Copy any agent's SecretProviderClass, replace the agent name in `metadata.name`, `tide.sei.io/agent-id` label, and all `objectName` paths. Add the new file to `base/secrets/` and add it to `base/kustomization.yaml`. Create the corresponding secrets in AWS Secrets Manager and the IRSA role/ServiceAccount.

#### Shared Config SecretProviderClass

```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: tide-shared-config
  namespace: tide-system
  labels:
    app.kubernetes.io/part-of: tide
spec:
  provider: aws
  parameters:
    objects: |
      - objectName: "tide/config/tide-council-abi"
        objectType: "secretsmanager"
        objectAlias: "tide-council-abi.json"
      - objectName: "tide/config/acp-abi"
        objectType: "secretsmanager"
        objectAlias: "acp-abi.json"
```

### Observability

#### ServiceMonitor (Orchestrator)

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: tide-orchestrator
  namespace: tide-system
  labels:
    app.kubernetes.io/part-of: tide
    app.kubernetes.io/name: tide-orchestrator
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: tide-orchestrator
  namespaceSelector:
    matchNames:
      - tide-system
  endpoints:
    - port: metrics
      interval: 30s
      path: /metrics
```

#### PodMonitor (Agent Jobs)

Agent Jobs are ephemeral and don't have Services. A PodMonitor scrapes their metrics endpoint if present. This is optional — agent containers are not required to expose metrics. If they do, use port name `metrics` on port `9090`.

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: tide-agent-jobs
  namespace: tide-agents
  labels:
    app.kubernetes.io/part-of: tide
spec:
  selector:
    matchLabels:
      app.kubernetes.io/component: agent
  namespaceSelector:
    matchNames:
      - tide-agents
  podMetricsEndpoints:
    - port: metrics
      interval: 30s
      path: /metrics
```

#### PrometheusRule (Alerting)

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: tide-alerts
  namespace: tide-system
  labels:
    app.kubernetes.io/part-of: tide
    prometheus: kube-prometheus
spec:
  groups:
    - name: tide-orchestrator
      rules:
        - alert: TideOrchestratorDown
          expr: |
            absent(up{job="tide-orchestrator", namespace="tide-system"} == 1)
          for: 2m
          labels:
            severity: critical
            team: platform
          annotations:
            summary: "Tide Orchestrator is down"
            description: "No healthy Tide Orchestrator pod has been scraped for 2 minutes."
            runbook_url: "https://wiki.internal/tide/runbooks/orchestrator-down"

        - alert: TideEventLagHigh
          expr: |
            tide_event_processing_lag_seconds{namespace="tide-system"} > 60
          for: 5m
          labels:
            severity: warning
            team: platform
          annotations:
            summary: "Tide event processing lag is high"
            description: "Orchestrator is {{ $value }}s behind chain HEAD."

        - alert: TideTokenRefreshFailing
          expr: |
            increase(tide_token_refresh_errors_total{namespace="tide-system"}[15m]) > 3
          labels:
            severity: critical
            team: platform
          annotations:
            summary: "GitHub App token refresh is failing"
            description: "{{ $value }} token refresh errors in the last 15 minutes."

        - alert: TideGitHubRateLimit
          expr: |
            tide_github_api_remaining{namespace="tide-system"} < 500
          for: 5m
          labels:
            severity: warning
            team: platform
          annotations:
            summary: "GitHub API rate limit is low"
            description: "Agent {{ $labels.agent_id }} has {{ $value }} API calls remaining."

    - name: tide-agents
      rules:
        - alert: TideJobTimeout
          expr: |
            (time() - kube_job_status_start_time{namespace="tide-agents"})
            / on(job_name) group_left
            kube_job_spec_active_deadline_seconds{namespace="tide-agents"}
            > 0.8
          for: 1m
          labels:
            severity: warning
            team: platform
          annotations:
            summary: "Tide agent job approaching timeout"
            description: "Job {{ $labels.job_name }} is at {{ $value | humanizePercentage }} of its deadline."

        - alert: TideJobFailureRate
          expr: |
            rate(kube_job_status_failed{namespace="tide-agents"}[1h])
            / (rate(kube_job_status_succeeded{namespace="tide-agents"}[1h]) + rate(kube_job_status_failed{namespace="tide-agents"}[1h]))
            > 0.3
          for: 30m
          labels:
            severity: warning
            team: platform
          annotations:
            summary: "Tide agent job failure rate is high"
            description: "{{ $value | humanizePercentage }} of agent jobs failed in the last hour."

        - alert: TideAgentOOM
          expr: |
            increase(kube_pod_container_status_last_terminated_reason{namespace="tide-agents", reason="OOMKilled"}[1h]) > 0
          labels:
            severity: warning
            team: platform
          annotations:
            summary: "Tide agent container OOMKilled"
            description: "Container {{ $labels.container }} in pod {{ $labels.pod }} was OOMKilled."

        - alert: TideAgentQuotaExhausted
          expr: |
            kube_resourcequota{namespace="tide-agents", type="used", resource="count/jobs.batch"}
            / kube_resourcequota{namespace="tide-agents", type="hard", resource="count/jobs.batch"}
            > 0.9
          for: 5m
          labels:
            severity: warning
            team: platform
          annotations:
            summary: "Tide agents job quota nearly exhausted"
            description: "{{ $value | humanizePercentage }} of the agent job quota is in use."
```

### Platform ConfigMap

A ConfigMap in `tide-system` holding platform-wide configuration consumed by the Orchestrator. The overlay patches this per environment.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tide-platform-config
  namespace: tide-system
  labels:
    app.kubernetes.io/part-of: tide
data:
  SEI_RPC_URL: "https://evm-rpc-testnet.sei-apis.com"
  SEI_CHAIN_ID: "713715"
  TIDE_COUNCIL_ADDRESS: "0x0000000000000000000000000000000000000000"
  TIDE_ACP_ADDRESS: "0x0000000000000000000000000000000000000000"
  USDC_ADDRESS: "0xe15fC38F6D8c56aF07bbCBe3BAf5708A2Bf42392"
  GITHUB_ORG: "sei-tide"
  PROPOSALS_REPO: "sei-tide/proposals"
  DELIVERABLES_REPO: "sei-tide/deliverables"
  LOG_LEVEL: "debug"
  LLM_MODEL: "claude-sonnet-4-20250514"
  LLM_TOKEN_BUDGET_REVIEW: "100000"
  LLM_TOKEN_BUDGET_EXECUTION: "1000000"
  MAX_ITERATIONS_EXECUTION: "15"
```

Contract addresses are `0x000...` in the base — the overlay patches them with actual deployed addresses.

### Kustomize Base

```yaml
# k8s/base/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  # Namespaces
  - namespaces/tide-system.yaml
  - namespaces/tide-agents.yaml
  # RBAC
  - rbac/tide-system-sa.yaml
  - rbac/tide-system-roles.yaml
  - rbac/tide-agents-sa.yaml
  - rbac/tide-agents-roles.yaml
  # Resource governance
  - resource-governance/tide-system-quota.yaml
  - resource-governance/tide-system-limitrange.yaml
  - resource-governance/tide-agents-quota.yaml
  - resource-governance/tide-agents-limitrange.yaml
  # Network policies
  - network/tide-system-default-deny.yaml
  - network/tide-system-egress-allow.yaml
  - network/tide-system-ingress-allow.yaml
  - network/tide-agents-default-deny.yaml
  - network/tide-agents-egress-allow.yaml
  # Secrets
  - secrets/secretproviderclass-agent-alpha.yaml
  - secrets/secretproviderclass-agent-beta.yaml
  - secrets/secretproviderclass-agent-gamma.yaml
  - secrets/secretproviderclass-shared-config.yaml
  # Monitoring
  - monitoring/servicemonitor-orchestrator.yaml
  - monitoring/prometheusrule-tide.yaml
  - monitoring/podmonitor-agents.yaml

commonLabels:
  app.kubernetes.io/managed-by: kustomize
  app.kubernetes.io/part-of: tide
```

### Kustomize Overlays

#### Testnet

```yaml
# k8s/overlays/testnet/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../base

patches:
  # Namespace label patches (e.g., add testnet-specific annotations)
  - path: namespace-labels-patch.yaml
    target:
      kind: Namespace
      name: tide-agents
  # Resource quota overrides for testnet (lower limits)
  - path: quota-patch-tide-system.yaml
    target:
      kind: ResourceQuota
      name: tide-system-quota
  - path: quota-patch-tide-agents.yaml
    target:
      kind: ResourceQuota
      name: tide-agents-quota

configMapGenerator:
  - name: tide-platform-config
    namespace: tide-system
    behavior: replace
    literals:
      - SEI_RPC_URL=https://evm-rpc-testnet.sei-apis.com
      - SEI_CHAIN_ID=713715
      - TIDE_COUNCIL_ADDRESS=0xTESTNET_COUNCIL_ADDRESS
      - TIDE_ACP_ADDRESS=0xTESTNET_ACP_ADDRESS
      - USDC_ADDRESS=0xe15fC38F6D8c56aF07bbCBe3BAf5708A2Bf42392
      - GITHUB_ORG=sei-tide
      - PROPOSALS_REPO=sei-tide/proposals
      - DELIVERABLES_REPO=sei-tide/deliverables
      - LOG_LEVEL=debug
      - LLM_MODEL=claude-sonnet-4-20250514
      - LLM_TOKEN_BUDGET_REVIEW=50000
      - LLM_TOKEN_BUDGET_EXECUTION=500000
      - MAX_ITERATIONS_EXECUTION=10
```

Testnet quota patch (lower limits to save cost):

```yaml
# k8s/overlays/testnet/quota-patch-tide-agents.yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: tide-agents-quota
  namespace: tide-agents
spec:
  hard:
    requests.cpu: "4"
    requests.memory: 8Gi
    limits.cpu: "8"
    limits.memory: 16Gi
    count/jobs.batch: "10"
    count/pods: "10"
    requests.ephemeral-storage: 50Gi
```

```yaml
# k8s/overlays/testnet/quota-patch-tide-system.yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: tide-system-quota
  namespace: tide-system
spec:
  hard:
    requests.cpu: "1"
    requests.memory: 2Gi
    limits.cpu: "2"
    limits.memory: 4Gi
```

```yaml
# k8s/overlays/testnet/namespace-labels-patch.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: tide-agents
  labels:
    tide.sei.io/environment: testnet
```

#### Mainnet

```yaml
# k8s/overlays/mainnet/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../base

patches:
  - path: namespace-labels-patch.yaml
    target:
      kind: Namespace
      name: tide-agents
  - path: quota-patch-tide-system.yaml
    target:
      kind: ResourceQuota
      name: tide-system-quota
  - path: quota-patch-tide-agents.yaml
    target:
      kind: ResourceQuota
      name: tide-agents-quota

configMapGenerator:
  - name: tide-platform-config
    namespace: tide-system
    behavior: replace
    literals:
      - SEI_RPC_URL=https://evm-rpc.sei-apis.com
      - SEI_CHAIN_ID=1329
      - TIDE_COUNCIL_ADDRESS=0xMAINNET_COUNCIL_ADDRESS
      - TIDE_ACP_ADDRESS=0xMAINNET_ACP_ADDRESS
      - USDC_ADDRESS=0xe15fC38F6D8c56aF07bbCBe3BAf5708A2Bf42392
      - GITHUB_ORG=sei-tide
      - PROPOSALS_REPO=sei-tide/proposals
      - DELIVERABLES_REPO=sei-tide/deliverables
      - LOG_LEVEL=info
      - LLM_MODEL=claude-sonnet-4-20250514
      - LLM_TOKEN_BUDGET_REVIEW=200000
      - LLM_TOKEN_BUDGET_EXECUTION=2000000
      - MAX_ITERATIONS_EXECUTION=25
```

```yaml
# k8s/overlays/mainnet/quota-patch-tide-agents.yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: tide-agents-quota
  namespace: tide-agents
spec:
  hard:
    requests.cpu: "16"
    requests.memory: 32Gi
    limits.cpu: "32"
    limits.memory: 64Gi
    count/jobs.batch: "30"
    count/pods: "30"
    requests.ephemeral-storage: 200Gi
```

```yaml
# k8s/overlays/mainnet/quota-patch-tide-system.yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: tide-system-quota
  namespace: tide-system
spec:
  hard:
    requests.cpu: "4"
    requests.memory: 8Gi
    limits.cpu: "8"
    limits.memory: 16Gi
```

```yaml
# k8s/overlays/mainnet/namespace-labels-patch.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: tide-agents
  labels:
    tide.sei.io/environment: mainnet
```

---

## State Model

### What State Exists

| State | Where | Source of Truth | Lifecycle |
|-------|-------|----------------|-----------|
| Namespace configuration | etcd (K8s API) | Git manifests (Kustomize) → applied via Flux or `kubectl` | Long-lived. Changed only on manifest updates. |
| ResourceQuotas / LimitRanges | etcd | Git manifests | Long-lived. Overlay patches per environment. |
| RBAC (SA, Roles, Bindings) | etcd | Git manifests | Long-lived. New agent = new ServiceAccount + IRSA role. |
| NetworkPolicies | etcd | Git manifests | Long-lived. Changed only if new egress targets needed. |
| SecretProviderClass | etcd | Git manifests | Long-lived per agent. New agent = new SPC file in `base/secrets/`. |
| Actual secrets | AWS Secrets Manager | Secrets Manager console/API | Managed outside K8s. Rotated via Secrets Manager. CSI driver syncs on pod mount. |
| Platform ConfigMap | etcd | Git manifests (overlay) | Updated on contract deployment (new addresses), environment changes. |
| ServiceMonitor / PrometheusRule | etcd | Git manifests | Long-lived. Updated when new metrics or alerts are defined. |

### State Transitions

The manifests are declarative — there are no runtime state transitions. Changes flow via:

```
Git commit → Flux/kubectl apply → K8s API → etcd
```

The only "transition" is adding a new agent, which follows the procedure in §Internal Design.

---

## Internal Design

### Adding a New Agent

When a new agent (e.g., `delta`) is onboarded, the following manifest changes are required:

```
1. Create AWS resources (outside these manifests):
   a. AWS KMS key: tide-agent-delta (secp256k1)
   b. AWS IAM role: tide-agent-delta-irsa
      - kms:Sign, kms:GetPublicKey on the new KMS key
      - secretsmanager:GetSecretValue on tide/agents/delta/*
   c. AWS Secrets Manager secrets:
      - tide/agents/delta/github-app-key
      - tide/agents/delta/anthropic-api-key
      - tide/agents/delta/system-prompt
      - tide/agents/delta/execution-system-prompt

2. Add ServiceAccount (k8s/base/rbac/tide-agents-sa.yaml):
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: tide-agent-delta
     namespace: tide-agents
     labels:
       tide.sei.io/agent-id: delta
     annotations:
       eks.amazonaws.com/role-arn: "arn:aws:iam::{ACCOUNT_ID}:role/tide-agent-delta-irsa"
   automountServiceAccountToken: false

3. Add SecretProviderClass (k8s/base/secrets/secretproviderclass-agent-delta.yaml):
   Copy from alpha, replace "alpha" → "delta" in metadata and objectName paths.

4. Add to kustomization.yaml (k8s/base/kustomization.yaml):
   resources:
     ...existing...
     - rbac/tide-agents-sa.yaml  # already includes delta (appended to file)
     - secrets/secretproviderclass-agent-delta.yaml  # new line

5. Commit and push. Flux/kubectl applies the changes.
```

**Time estimate:** ~15 minutes for the K8s manifest changes (copy-paste-rename). The AWS resource creation takes longer but is scripted.

### Manifest Validation

Before applying, validate manifests with:

```bash
# Build and validate with kustomize
kustomize build k8s/overlays/testnet | kubectl apply --dry-run=server -f -
kustomize build k8s/overlays/mainnet | kubectl apply --dry-run=server -f -

# Validate YAML syntax
kustomize build k8s/overlays/testnet | kubeval --strict

# Validate against Pod Security Standards
kustomize build k8s/overlays/testnet | kubectl auth can-i --list
```

### Apply Order

Kustomize handles dependency ordering within a single `kubectl apply`, but for initial setup the order matters:

```
1. Namespaces (must exist before anything else)
2. ResourceQuotas and LimitRanges (governance before workloads)
3. RBAC (ServiceAccounts, Roles, RoleBindings)
4. NetworkPolicies (security before workloads)
5. SecretProviderClasses (secrets before workloads)
6. Monitoring (ServiceMonitor, PrometheusRule)
7. ConfigMap (platform config)
```

In practice, `kustomize build | kubectl apply -f -` applies in the correct order because K8s API handles creation order for these resource types. For initial bootstrap, apply twice to resolve any ordering issues with CRDs.

---

## Error Handling

| Error Condition | Detection | Impact | Recovery |
|----------------|-----------|--------|----------|
| Namespace doesn't exist when Job created | K8s API rejects Job creation | Orchestrator cannot create agent Jobs | Apply manifests. Verify `kustomize build` output includes namespaces. |
| ResourceQuota exceeded | K8s API rejects Pod creation with `403 Forbidden: exceeded quota` | New agent Jobs cannot start | Wait for existing Jobs to complete, or increase quota in overlay. Alert `TideAgentQuotaExhausted` fires. |
| LimitRange violated | K8s API rejects Pod creation | Job with excessive resource requests fails to schedule | Fix the Operator's Job template to respect LimitRange `max` values. |
| NetworkPolicy blocks required egress | Agent container gets connection timeouts to GitHub/Anthropic/Sei | Agent Job fails with exit 20/30/51 | Verify NetworkPolicy egress rules. Test with `kubectl run --rm -it test --image=curlimages/curl -- curl https://api.github.com` in `tide-agents`. |
| SecretProviderClass misconfigured | Pod fails to start, CSI volume mount error | Agent Job stuck in `ContainerCreating` | Check `kubectl describe pod` for CSI errors. Verify Secrets Manager path, IRSA role, CSI driver health. |
| IRSA not configured | Pod gets default node IAM role (or no credentials) | KMS signing fails, Secrets Manager access fails | Verify `eks.amazonaws.com/role-arn` annotation on ServiceAccount. Check OIDC provider configuration on EKS cluster. |
| Prometheus Operator CRDs missing | ServiceMonitor/PrometheusRule rejected by API | No monitoring or alerting | Install Prometheus Operator CRDs first. Manifests are non-blocking — other resources apply fine. |
| CSI Driver not installed | SecretProviderClass exists but pods can't mount | All agent Jobs fail at startup | Install AWS Secrets Store CSI Driver via Helm or EKS addon. |
| Kustomize build fails | `kustomize build` exits non-zero | Cannot apply manifests | Fix YAML syntax, resource references. Run `kustomize build` locally before push. |

---

## Test Specification

### Unit Tests (Manifest Validation)

| Test | Setup | Action | Expected Result |
|------|-------|--------|-----------------|
| `test_kustomize_build_testnet` | Manifests in `k8s/` | `kustomize build k8s/overlays/testnet` | Exits 0. Valid YAML output containing all resources. |
| `test_kustomize_build_mainnet` | Manifests in `k8s/` | `kustomize build k8s/overlays/mainnet` | Exits 0. Valid YAML output containing all resources. |
| `test_namespace_pss_labels` | Built manifests | Parse Namespace YAML | `tide-agents` has `pod-security.kubernetes.io/enforce: restricted`. `tide-system` has `baseline`. |
| `test_agent_sa_no_automount` | Built manifests | Parse ServiceAccount YAML for `tide-agent-*` | All agent ServiceAccounts have `automountServiceAccountToken: false`. |
| `test_agent_sa_irsa_annotation` | Built manifests | Parse ServiceAccount YAML | All agent ServiceAccounts have `eks.amazonaws.com/role-arn` annotation. |
| `test_default_deny_both_namespaces` | Built manifests | Parse NetworkPolicy YAML | Both namespaces have a `default-deny-all` policy with empty `podSelector`. |
| `test_agent_egress_no_imds` | Built manifests | Parse agent egress NetworkPolicy | `169.254.169.254/32` is in the `except` list. |
| `test_agent_egress_no_private_ranges` | Built manifests | Parse agent egress NetworkPolicy | `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16` are in `except` list. |
| `test_agent_egress_https_only` | Built manifests | Parse agent egress NetworkPolicy | Only port 443 TCP is allowed for external egress. |
| `test_quota_values_testnet` | Testnet overlay | Parse ResourceQuota | `tide-agents` CPU requests ≤ base values. |
| `test_quota_values_mainnet` | Mainnet overlay | Parse ResourceQuota | `tide-agents` CPU requests ≥ base values. |
| `test_configmap_testnet_chain_id` | Testnet overlay | Parse ConfigMap | `SEI_CHAIN_ID=713715`. |
| `test_configmap_mainnet_chain_id` | Mainnet overlay | Parse ConfigMap | `SEI_CHAIN_ID=1329`. |
| `test_secretproviderclass_per_agent` | Built manifests | Count SecretProviderClass resources | One per agent (alpha, beta, gamma) + one shared. |
| `test_all_agents_have_spc` | Built manifests | Cross-reference agent SAs with SPCs | Every `tide-agent-*` ServiceAccount has a corresponding SecretProviderClass. |
| `test_prometheusrule_valid` | Built manifests | Parse PrometheusRule, validate PromQL | All `expr` fields are valid PromQL (use `promtool check rules`). |

### Integration Tests

| Test | Setup | Action | Expected Result |
|------|-------|--------|-----------------|
| `test_apply_to_testnet_cluster` | EKS testnet cluster with ASCP + Prometheus Operator | `kustomize build k8s/overlays/testnet \| kubectl apply -f -` | All resources created. No errors. |
| `test_agent_pod_creation_succeeds` | Manifests applied | Create a test Job in `tide-agents` with agent security context | Job pod starts, enters Running state. |
| `test_agent_pod_pss_violation_rejected` | Manifests applied | Create a Job with `privileged: true` in `tide-agents` | Rejected by PSS admission controller. |
| `test_agent_egress_github` | Manifests applied, test pod in `tide-agents` | `curl https://api.github.com` from test pod | HTTP 200. |
| `test_agent_egress_imds_blocked` | Manifests applied, test pod in `tide-agents` | `curl http://169.254.169.254/latest/meta-data/` from test pod | Connection timeout or refused. |
| `test_agent_egress_internal_blocked` | Manifests applied, test pod in `tide-agents` | `curl http://10.0.0.1:443` from test pod | Connection timeout or refused. |
| `test_secret_mount` | Manifests applied, test secrets in Secrets Manager | Create test pod with CSI volume mount using agent SPC | Secrets available at expected file paths inside pod. |
| `test_quota_enforcement` | Manifests applied | Create Jobs exceeding quota | Final Job is rejected with quota exceeded error. |
| `test_limitrange_defaults` | Manifests applied | Create Job without resource requests | Pod gets default requests/limits from LimitRange. |
| `test_prometheus_scrape` | Manifests applied, Orchestrator running with /metrics | Check Prometheus targets | `tide-orchestrator` appears as a healthy scrape target. |
| `test_alert_fires` | Manifests applied | Simulate `tide_event_processing_lag_seconds > 60` for 5+ minutes | `TideEventLagHigh` alert fires in Prometheus/Alertmanager. |

### E2E Smoke Test

```bash
#!/bin/bash
# Smoke test: apply manifests, create a test pod, verify security posture

ENV=${1:-testnet}

# 1. Apply manifests
kustomize build k8s/overlays/$ENV | kubectl apply -f -

# 2. Wait for namespace to be active
kubectl get ns tide-agents -o jsonpath='{.status.phase}' | grep -q Active

# 3. Verify PSS label
PSS=$(kubectl get ns tide-agents -o jsonpath='{.metadata.labels.pod-security\.kubernetes\.io/enforce}')
[ "$PSS" = "restricted" ] || { echo "FAIL: PSS not restricted, got $PSS"; exit 1; }

# 4. Create a test pod that should succeed (restricted-compliant)
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: tide-smoke-test
  namespace: tide-agents
  labels:
    app.kubernetes.io/component: agent
spec:
  serviceAccountName: tide-agent-alpha
  automountServiceAccountToken: false
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    fsGroup: 1000
    seccompProfile:
      type: RuntimeDefault
  containers:
    - name: test
      image: curlimages/curl:latest
      command: ["sh", "-c", "curl -sf https://api.github.com > /dev/null && echo OK || echo FAIL"]
      securityContext:
        allowPrivilegeEscalation: false
        capabilities:
          drop: ["ALL"]
        readOnlyRootFilesystem: true
      resources:
        requests:
          cpu: 100m
          memory: 128Mi
        limits:
          cpu: 200m
          memory: 256Mi
      volumeMounts:
        - name: tmp
          mountPath: /tmp
  restartPolicy: Never
  volumes:
    - name: tmp
      emptyDir: {}
EOF

kubectl wait --for=condition=Ready pod/tide-smoke-test -n tide-agents --timeout=60s

# 5. Verify egress to GitHub works
RESULT=$(kubectl logs tide-smoke-test -n tide-agents)
[ "$RESULT" = "OK" ] || { echo "FAIL: GitHub egress blocked"; exit 1; }

# 6. Clean up
kubectl delete pod tide-smoke-test -n tide-agents

# 7. Verify IMDS is blocked (create another test pod)
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: tide-smoke-imds
  namespace: tide-agents
  labels:
    app.kubernetes.io/component: agent
spec:
  serviceAccountName: tide-agent-alpha
  automountServiceAccountToken: false
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    seccompProfile:
      type: RuntimeDefault
  containers:
    - name: test
      image: curlimages/curl:latest
      command: ["sh", "-c", "curl -sf --max-time 5 http://169.254.169.254/latest/meta-data/ && echo REACHABLE || echo BLOCKED"]
      securityContext:
        allowPrivilegeEscalation: false
        capabilities:
          drop: ["ALL"]
        readOnlyRootFilesystem: true
      resources:
        requests:
          cpu: 100m
          memory: 128Mi
      volumeMounts:
        - name: tmp
          mountPath: /tmp
  restartPolicy: Never
  volumes:
    - name: tmp
      emptyDir: {}
EOF

kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/tide-smoke-imds -n tide-agents --timeout=30s || true
IMDS_RESULT=$(kubectl logs tide-smoke-imds -n tide-agents)
[ "$IMDS_RESULT" = "BLOCKED" ] || { echo "FAIL: IMDS reachable!"; exit 1; }

kubectl delete pod tide-smoke-imds -n tide-agents

echo "PASS: K8s platform manifests smoke test ($ENV)"
```

---

## Deployment

### Build and Apply

There is no container image to build for this component. Deployment is manifest application.

```bash
# Testnet
kustomize build k8s/overlays/testnet | kubectl apply --server-side -f -

# Mainnet
kustomize build k8s/overlays/mainnet | kubectl apply --server-side -f -
```

`--server-side` is recommended for large manifest sets to avoid client-side field management conflicts.

### GitOps Integration

These manifests are designed for Flux or ArgoCD reconciliation:

```yaml
# Flux Kustomization (example)
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: tide-platform
  namespace: flux-system
spec:
  interval: 5m
  path: ./k8s/overlays/testnet   # or mainnet
  prune: true
  sourceRef:
    kind: GitRepository
    name: tide-platform
  healthChecks:
    - apiVersion: v1
      kind: Namespace
      name: tide-agents
    - apiVersion: v1
      kind: Namespace
      name: tide-system
```

### Pre-requisites Checklist

Before applying these manifests, verify:

| Prerequisite | Verification Command | Expected |
|-------------|---------------------|----------|
| EKS cluster accessible | `kubectl cluster-info` | Cluster endpoint responds |
| kubectl version ≥ 1.28 | `kubectl version --client` | v1.28+ |
| kustomize installed | `kustomize version` | v5.0+ |
| ASCP CSI driver installed | `kubectl get csidriver secrets-store.csi.k8s.io` | Found |
| Prometheus Operator CRDs | `kubectl get crd servicemonitors.monitoring.coreos.com` | Found |
| NetworkPolicy enforcement | `kubectl get ds -n kube-system calico-node` or VPC CNI policy support | CNI supports NetworkPolicy |
| OIDC provider configured | `aws eks describe-cluster --name {cluster} --query cluster.identity.oidc` | OIDC issuer URL present |
| AWS Secrets Manager secrets created | `aws secretsmanager get-secret-value --secret-id tide/agents/alpha/github-app-key` | Secret exists |

### Testnet vs Mainnet Differences

| Aspect | Testnet | Mainnet |
|--------|---------|---------|
| `tide-agents` CPU quota | 4 CPU requests / 8 limits | 16 requests / 32 limits |
| `tide-agents` Memory quota | 8Gi requests / 16Gi limits | 32Gi requests / 64Gi limits |
| `tide-agents` Job count | 10 | 30 |
| `tide-system` CPU quota | 1 request / 2 limits | 4 requests / 8 limits |
| `tide-system` Memory quota | 2Gi requests / 4Gi limits | 8Gi requests / 16Gi limits |
| Platform ConfigMap `SEI_CHAIN_ID` | `713715` | `1329` |
| Platform ConfigMap `SEI_RPC_URL` | Testnet RPC | Mainnet RPC |
| Platform ConfigMap `LOG_LEVEL` | `debug` | `info` |
| Platform ConfigMap `LLM_TOKEN_BUDGET_*` | Lower (cost control) | Higher (production) |
| Namespace label `tide.sei.io/environment` | `testnet` | `mainnet` |
| Contract addresses | Testnet deployments | Mainnet deployments |

---

## Deferred (Do Not Build)

| Feature | Rationale |
|---------|-----------|
| PodDisruptionBudget for agent Jobs | YAGNI — agent Jobs are ephemeral and fail-fast. The Operator retries. PDB is meaningful only for long-lived Deployments (the Orchestrator owns its own PDB). |
| Istio/service mesh integration | YAGNI — NetworkPolicy provides sufficient network segmentation. Service mesh adds complexity without benefit for batch Jobs. |
| Pod priority classes | YAGNI — with 3 agents and low concurrency, priority-based scheduling is unnecessary. |
| Horizontal Pod Autoscaler for Orchestrator | YAGNI — 2 fixed replicas with leader election is sufficient for Phase 0-2 load. |
| Namespace-per-agent isolation | YAGNI — a single `tide-agents` namespace with per-agent ServiceAccounts and IRSA roles provides sufficient isolation. Namespace-per-agent adds operational overhead for 3 agents. Reconsider at 10+ agents. |
| VPA (Vertical Pod Autoscaler) | YAGNI — resource requests are manually tuned via overlays. VPA is useful at scale. |
| OPA/Gatekeeper policies | YAGNI — PSS admission controller handles Pod security. Custom policies (e.g., "all agent Jobs must have `tide.sei.io/job-id` label") can be added in Phase 3 when more governance is needed. |
| Multi-cluster manifest support | YAGNI — single EKS cluster for Phase 0-2. Multi-cluster for regional redundancy is Phase 3+. |
| ArgoCD ApplicationSet for per-agent generation | YAGNI — 3 agents don't warrant dynamic manifest generation. Copy-paste-rename is fine. |
| Cost allocation via Kubecost labels | YAGNI for manifests — the labels (`tide.sei.io/agent-id`, `tide.sei.io/job-id`) are defined in the Operator's Job template, not in these platform manifests. |
