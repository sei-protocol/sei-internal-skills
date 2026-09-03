# Onboarding PR file shapes

Concrete file contents for the platform-repo and workspace-repo onboarding PRs, taken from the canonical fromtherain onboarding.

**Substitute `fromtherain` → `<alias>`** when rendering for a new engineer (lowercase, regex `^[a-z]([a-z0-9-]{0,28}[a-z0-9])?$`).

That substitution is file-wide for Files 1, 3 and 5, which are new files. Files 2 and 4 are **append-only** edits to existing rosters: each already lists every onboarded engineer, including `fromtherain`. Add one entry there and change nothing else. A blanket replace on a roster renames another engineer's entry instead of adding yours.

## File 1: `clusters/harbor/engineers/<alias>/kustomization.yaml` (platform repo, new)

```yaml
# fromtherain engineer cell on Harbor.
# Composes the shared engineers/base template, propagating the alias
# `fromtherain` into Namespace, ServiceAccount, Role, RoleBinding, and
# Flux Kustomization names via the replacements below. The tenant-info
# ConfigMap is local-config (used as the alias source, not emitted) and
# is namespaced so the parent harbor build can compose multiple cells
# without resource-id collision.
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../base

configMapGenerator:
  - name: tenant-info
    namespace: eng-fromtherain
    literals:
      - alias=fromtherain
    options:
      annotations:
        config.kubernetes.io/local-config: "true"
      disableNameSuffixHash: true

replacements:
  - source:
      kind: ConfigMap
      name: tenant-info
      fieldPath: data.alias
    targets:
      - select:
          kind: ServiceAccount
        reject:
          - name: engineer-service-account
          - name: seid-node
        fieldPaths:
          - metadata.name
      - select:
          kind: Role
          name: tenant
        fieldPaths:
          - metadata.name
      - select:
          kind: RoleBinding
          name: tenant
        fieldPaths:
          - metadata.name
          - subjects.0.name
          - roleRef.name
      - select:
          kind: Kustomization
          group: kustomize.toolkit.fluxcd.io
        fieldPaths:
          - metadata.name
          - spec.serviceAccountName
      - select:
          kind: Namespace
        fieldPaths:
          - metadata.labels.[tide.sei.io/owner]
          - metadata.labels.[toolkit.fluxcd.io/tenant]
      - select:
          kind: ServiceAccount
          name: engineer-service-account
        fieldPaths:
          - metadata.labels.[tide.sei.io/owner]
      - select:
          kind: ServiceAccount
          name: seid-node
        fieldPaths:
          - metadata.labels.[tide.sei.io/owner]
  - source:
      kind: ConfigMap
      name: tenant-info
      fieldPath: data.alias
    targets:
      - select:
          kind: Namespace
        fieldPaths:
          - metadata.name
          - metadata.labels.[app.kubernetes.io/name]
        options:
          delimiter: "-"
          index: 1
      - select:
          kind: ServiceAccount
        fieldPaths:
          - metadata.namespace
        options:
          delimiter: "-"
          index: 1
      - select:
          kind: Role
        fieldPaths:
          - metadata.namespace
        options:
          delimiter: "-"
          index: 1
      - select:
          kind: RoleBinding
        fieldPaths:
          - metadata.namespace
        options:
          delimiter: "-"
          index: 1
      # controller-configmaps-writer binds the controller SA in
      # sei-k8s-controller-system; alias its metadata.namespace but never its subject.
      - select:
          kind: RoleBinding
        reject:
          - name: controller-configmaps-writer
        fieldPaths:
          - subjects.0.namespace
        options:
          delimiter: "-"
          index: 1
      - select:
          kind: Kustomization
          group: kustomize.toolkit.fluxcd.io
        fieldPaths:
          - metadata.namespace
          - spec.targetNamespace
        options:
          delimiter: "-"
          index: 1
      - select:
          kind: Kustomization
          group: kustomize.toolkit.fluxcd.io
        fieldPaths:
          - spec.path
        options:
          delimiter: "/"
          index: 2
      - select:
          kind: ClusterRoleBinding
        fieldPaths:
          - metadata.name
        options:
          delimiter: "-"
          index: 5
      - select:
          kind: ClusterRoleBinding
        fieldPaths:
          - subjects.0.namespace
        options:
          delimiter: "-"
          index: 1
```

## File 2: `clusters/harbor/engineers/kustomization.yaml` (platform repo, modified)

Append `- <alias>` to the `resources:` list in alphabetical position.

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - amir
  - brandon
  - fromtherain
```

## File 3: `terraform/aws/189176372795/eu-central-1/harbor/engineers/<alias>.tf` (platform repo, new)

```hcl
# Pairs with clusters/harbor/engineers/fromtherain/ — the SAs bound below
# are Flux-managed there.

module "eng_fromtherain_seid_node_pod_identity" {
  source  = "registry.terraform.io/terraform-aws-modules/eks-pod-identity/aws"
  version = ">= 2.4"

  name = "${var.name_prefix}-eng-fromtherain-seid-node"

  additional_policy_arns = {
    seid_node = var.iam_policy_seid_node_engineer_arn
  }

  associations = {
    this = {
      cluster_name    = var.cluster_name
      namespace       = "eng-fromtherain"
      service_account = "seid-node"
    }
  }

  tags = var.tags
}

module "eng_fromtherain_engineer_pod_identity" {
  source  = "registry.terraform.io/terraform-aws-modules/eks-pod-identity/aws"
  version = ">= 2.4"

  name = "${var.name_prefix}-eng-fromtherain-engineer"

  additional_policy_arns = {
    engineer = var.iam_policy_engineer_arn
  }

  associations = {
    this = {
      cluster_name    = var.cluster_name
      namespace       = "eng-fromtherain"
      service_account = "engineer-service-account"
    }
  }

  tags = var.tags
}
```

## File 4: `clusters/harbor/monitoring/podmonitor-seiload-eng.yaml` (platform repo, modified)

Append `eng-<alias>` to `namespaceSelector.matchNames`, keeping the list
alphabetical. Only that list changes; leave the comment and the rest of the
spec alone.

Unlike Files 1, 3 and 5, this block is not the whole file. It shows one edit
inside a much larger document. This doc elides the existing entries on purpose.
Write the roster back from a snapshot here, and you drop every engineer who
joined after that snapshot.

```yaml
  namespaceSelector:
    matchNames:
      # ... every existing eng-* entry, unchanged ...
      - eng-<alias>          # <- the only line you add, in alphabetical position
```

The selector enumerates `matchNames` because the PodMonitor CRD accepts only
`any` or `matchNames`. It matches no label and no glob. `any: true` also selects
the nightly namespace's seiload and scrapes it twice.

Prometheus scrapes a cell only after this list names its namespace. Onboard a
cell without this edit and seiload runs, but no loadgen metrics arrive.

## File 5: `engineers/<alias>/kustomization.yaml` (workspace repo: `sei-protocol/harbor-engineering-workspace`, new)

```yaml
# fromtherain's Harbor workspace.
# Engineer-rendered tasks (chains, RPC fleets, benchmarks via the
# sei-platform-engineer skill) commit to sub-paths under this directory
# and reference them in `resources`. Flux on harbor reconciles whatever
# lands here into eng-fromtherain.
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources: []
```
