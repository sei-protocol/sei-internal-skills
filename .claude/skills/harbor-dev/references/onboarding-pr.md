# Onboarding PR (the one-time tenant registration)

A new engineer's onboarding is one PR against `sei-protocol/platform` adding three files plus a sibling PR against `sei-protocol/harbor-engineering-workspace` adding one file. After both PRs merge, run a targeted `terraform apply`. Both pieces complete in under five minutes.

**Canonical reference PR:** [sei-protocol/platform#587](https://github.com/sei-protocol/platform/pull/587) — the fromtherain re-onboard. Use this as the diff template for new engineers; the shape is current with the latest base-template additions (including the templated rbac-proxy ClusterRoleBinding and the controller `node-configmaps-writer` Role + `controller-configmaps-writer` RoleBinding).

Literal file shapes live in [`onboarding-pr-template.md`](./onboarding-pr-template.md). Substitute `fromtherain` → `<alias>` throughout.

## Files in the PR

| Path | Action |
|---|---|
| `clusters/harbor/engineers/<alias>/kustomization.yaml` | New. Per-engineer overlay. Mirrors the most recent prior onboarding PR; only the `alias=<alias>` literal differs. |
| `clusters/harbor/engineers/kustomization.yaml` | Modified. Adds `- <alias>` to `resources`. |
| `terraform/aws/189176372795/eu-central-1/harbor/engineers/<alias>.tf` | New. Two `eks-pod-identity` module instances. Mirrors the prior engineer's file with substring replacement of the alias throughout. |

## Per-engineer overlay (file 1)

References `../base`, sets `alias=<alias>` via `configMapGenerator`, runs `replacements:` to substitute the `tenant` placeholder for the alias across the rendered base. Selectors:

- `ServiceAccount` → `metadata.name` aliased; rejects `engineer-service-account` and `seid-node` so those names stay literal.
- `Role name: tenant` and `RoleBinding name: tenant` → name + subjects + roleRef aliased. Positive selector keeps `engineer-admin` `RoleBinding` untouched.
- `Kustomization` (Flux) → `metadata.name` and `spec.serviceAccountName` aliased.
- `Namespace` → `tide.sei.io/owner` and `toolkit.fluxcd.io/tenant` labels aliased.
- `ServiceAccount name: engineer-service-account` and `name: seid-node` → `tide.sei.io/owner` label aliased.

A second `replacements:` block substitutes `eng-tenant` → `eng-<alias>` (delimiter `-`, index 1) for namespace fields, and `engineers/tenant` → `engineers/<alias>` (delimiter `/`, index 2) for the Flux Kustomization's `spec.path`. The RoleBinding `subjects.0.namespace` aliasing rejects `controller-configmaps-writer`: that binding's subject is the controller SA in `sei-k8s-controller-system`, which must stay literal (aliasing it would point the grant at a non-existent namespace).

Literal content in [`onboarding-pr-template.md`](./onboarding-pr-template.md) → File 1. Substring-replace `fromtherain` → `<alias>`.

## Aggregator update (file 2)

`clusters/harbor/engineers/kustomization.yaml` lists every onboarded engineer:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - <prior-alias-1>
  - <prior-alias-2>
  - <alias>
```

Append `<alias>` to `resources`. Alphabetical if the existing list is sorted.

## Per-engineer Terraform (file 3)

Two `module "eng_<alias>_seid_node_pod_identity"` and `module "eng_<alias>_engineer_pod_identity"` blocks at `terraform/aws/189176372795/eu-central-1/harbor/engineers/<alias>.tf`. Each:

- `source = "registry.terraform.io/terraform-aws-modules/eks-pod-identity/aws"`, `version = ">= 2.4"`.
- `name = "${var.name_prefix}-eng-<alias>-<purpose>"`.
- `additional_policy_arns` → either `seid_node = var.iam_policy_seid_node_engineer_arn` or `engineer = var.iam_policy_engineer_arn`.
- `associations.this = { cluster_name = var.cluster_name, namespace = "eng-<alias>", service_account = "<sa-name>" }`.
- `tags = var.tags`.

Literal content in [`onboarding-pr-template.md`](./onboarding-pr-template.md) → File 3. Substring-replace `fromtherain` → `<alias>`.

The wrapper `terraform/.../harbor/engineers.tf` and the submodule's `engineers/variables.tf` already exist; this PR does not touch them.

## Base layer (already in place)

`clusters/harbor/engineers/base/`:

| File | Provides |
|---|---|
| `namespace.yaml` | `eng-tenant` Namespace with `tide.sei.io/cell-type=personal`, `tide.sei.io/owner=tenant`, `toolkit.fluxcd.io/tenant=tenant`, `app.kubernetes.io/managed-by=flux`, `app.kubernetes.io/name=eng-tenant`. |
| `rbac.yaml` | `tenant` Role (snd/sn CRUD, derived resources read-only, jobs/configmaps/events CRUD, Flux resources read+patch); `tenant` `RoleBinding` (binds `tenant` SA to `tenant` Role); `engineer-admin` `RoleBinding` (binds `engineer-service-account` to built-in `admin` `ClusterRole`). |
| `sync.yaml` | Flux `Kustomization` `tenant` watching `harbor-engineering-workspace` GitRepository at `./engineers/tenant`. |
| `engineer-service-account.yaml` | `engineer-service-account` ServiceAccount with `tide.sei.io/cell-type=personal` and `tide.sei.io/owner=tenant` labels. |
| `seid-node-sa.yaml` | `seid-node` ServiceAccount with the same labels. |
| `controller-configmaps-rbac.yaml` | `node-configmaps-writer` Role (configmaps CRUD) + `controller-configmaps-writer` RoleBinding granting the controller SA (`sei-k8s-controller-manager@sei-k8s-controller-system`) configmaps write here — for the rbac-proxy/workflow-vars ConfigMaps it manages (the controller's ClusterRole is read-only on configmaps; PLT-471). |

Shared Terraform at `terraform/aws/189176372795/eu-central-1/harbor/`:

- `engineers-shared.tf` — `aws_iam_policy.engineer` (`s3:PutObject` and `s3:ListBucket` on `harbor-validation-results/${aws:PrincipalTag/kubernetes-namespace}/*`, ECR auth, `sei/sei-chain` image read).
- `engineers-shared.tf` — `aws_iam_policy.seid_node_engineer` (snapshot read, genesis r/w).
- `engineers.tf` — `module "engineers"` wrapper passing `cluster_name`, `name_prefix`, `iam_policy_seid_node_engineer_arn`, `iam_policy_engineer_arn`, `tags` into the submodule.

## What reconciles on PR merge

Within ~60s, Flux materializes:

| Resource | Where | Notes |
|---|---|---|
| `Namespace eng-<alias>` | cluster-scoped | Cells-forward labels, tagged for Flux multi-tenancy. |
| `ServiceAccount <alias>` | `eng-<alias>` | Flux reconciler identity. |
| `Role <alias>` + `RoleBinding <alias>` | `eng-<alias>` | Reconciler permission surface. |
| `ServiceAccount engineer-service-account` | `eng-<alias>` | Engineer-launched workloads. |
| `RoleBinding engineer-admin` | `eng-<alias>` | Binds `engineer-service-account` to `admin` `ClusterRole`. Namespace-scoped. |
| `ServiceAccount seid-node` | `eng-<alias>` | SeiNode StatefulSet pods. |
| Flux `Kustomization <alias>` | `eng-<alias>` | Watches `harbor-engineering-workspace`@`./engineers/<alias>`, reconciles every 5m, runs as `serviceAccountName: <alias>`. |

## What reconciles on terraform apply

After merge, run from `terraform/aws/189176372795/eu-central-1/harbor/` with the AWS profile resolved at pre-flight gate 3:

```sh
AWS_PROFILE=<chosen> terraform init
AWS_PROFILE=<chosen> terraform plan -target=module.engineers -out=tfplan
AWS_PROFILE=<chosen> terraform apply tfplan
```

`<chosen>` is the engineer's AWS profile (resolved by pre-flight gate 3 — never literal `sei`; engineers configure their own).

Run `terraform init` (no `-upgrade` flag) to fetch providers and modules; `-upgrade` would silently bump the provider lockfile and is not what this flow wants. Always `plan -out=tfplan` then `apply tfplan` — never direct `terraform apply` (skips plan review). Run from a single worktree at the platform repo's main branch — terraform state lives in S3 (`sei-platform-terraform-state`); concurrent worktrees on different commits won't corrupt state but can produce confusing plan diffs.

`Plan: 6 to add, 0 to change, 0 to destroy`. Apply confirms `Resources: 6 added`. The six resources:

- `module.engineers.module.eng_<alias>_seid_node_pod_identity.{aws_iam_role.this[0], aws_iam_role_policy_attachment.this["seid_node"], aws_eks_pod_identity_association.this["this"]}` — binds `(harbor, eng-<alias>, seid-node)` to `aws_iam_policy.seid_node_engineer`.
- `module.engineers.module.eng_<alias>_engineer_pod_identity.{aws_iam_role.this[0], aws_iam_role_policy_attachment.this["engineer"], aws_eks_pod_identity_association.this["this"]}` — binds `(harbor, eng-<alias>, engineer-service-account)` to `aws_iam_policy.engineer`.

Pods running as `engineer-service-account` see `aws:PrincipalTag/kubernetes-namespace=eng-<alias>` in the assumed-role session; S3 writes auto-scope to `harbor-validation-results/eng-<alias>/*`. SeiNode pods running as `seid-node` get snapshot read + genesis r/w via `aws_iam_policy.seid_node_engineer`.

## The agent's job

1. **Prompt for the alias.** Default the prompt to `$USER` lowercased — don't silently use it. Validate the response against `^[a-z]([a-z0-9-]{0,28}[a-z0-9])?$`. **Then check uniqueness** with `kubectl get namespace eng-<alias> --context harbor`: if the namespace already exists, the alias is taken — halt with "pick another, or contact the platform team if it's yours." Don't attempt partial-state recovery (separate runbook). Continue only when the alias is free.
2. **Render the three platform-repo files** from [`onboarding-pr-template.md`](./onboarding-pr-template.md) (Files 1–3) in a fresh clone of `sei-protocol/platform`. Substring-replace `fromtherain` → `<alias>` throughout. Branch: `feat/engineers-<alias>-onboard`.
3. **Open the platform-repo PR.** Title: `feat(harbor/engineers): onboard <alias>`. Body: see template below. `gh pr create --repo sei-protocol/platform --base main`.
4. **Open the workspace-repo scaffolding PR (sibling).** Render [`onboarding-pr-template.md`](./onboarding-pr-template.md) File 4 in a fresh clone of `sei-protocol/harbor-engineering-workspace`. Branch: `feat/onboard-<alias>`. Without this, the engineer's per-engineer Flux Kustomization fails reconcile post-merge with `kustomization path not found: ./engineers/<alias>`. Both PRs land independently; the platform-repo PR shouldn't merge before the workspace-repo PR is at least open. Surface both URLs.
5. **Surface and halt:**
   > Onboarding opened in two PRs:
   > - Platform: `<platform-pr-url>`
   > - Workspace: `<workspace-pr-url>`
   >
   > Merge the workspace PR first (or merge both within seconds of each other). After merge of the platform PR, Flux reconciles namespace + RBAC + Flux watcher in ~60s. Then from `terraform/aws/189176372795/eu-central-1/harbor/` run `AWS_PROFILE=<chosen> terraform init && terraform plan -target=module.engineers -out=tfplan && terraform apply tfplan` to land the Pod Identity associations. (`<chosen>` is the AWS profile resolved at pre-flight gate 3 — never literal `sei`.)
6. **After merge,** poll `kubectl get namespace eng-<alias>` until it returns 0.
7. **Run terraform.** From `terraform/aws/189176372795/eu-central-1/harbor/` at the platform repo's main branch: `AWS_PROFILE=<chosen> terraform init` (no `-upgrade` flag), then `AWS_PROFILE=<chosen> terraform plan -target=module.engineers -out=tfplan`, then `AWS_PROFILE=<chosen> terraform apply tfplan`. Confirm `Resources: 6 added`.
8. **Verify** `aws eks list-pod-identity-associations --cluster-name harbor --query 'associations[?namespace==`eng-<alias>`]' --region eu-central-1 --profile <chosen>` returns two associations (one for `seid-node`, one for `engineer-service-account`). Verify the engineer's Flux Kustomization is Ready: `kubectl get kustomization <alias> -n eng-<alias> -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'` returns `True`.

## PR body template

```markdown
Onboards `<alias>` as a tenant on harbor.

## Files

| Path | Action |
|---|---|
| `clusters/harbor/engineers/<alias>/kustomization.yaml` | New. |
| `clusters/harbor/engineers/kustomization.yaml` | Modified. |
| `terraform/.../harbor/engineers/<alias>.tf` | New. |

## What reconciles on merge

- Namespace `eng-<alias>` with cells-forward labels.
- `ServiceAccount <alias>` + `Role <alias>` + `RoleBinding <alias>` (Flux reconciler).
- `ServiceAccount engineer-service-account` + `RoleBinding engineer-admin` (binds to `admin` `ClusterRole`).
- `ServiceAccount seid-node`.
- Flux `Kustomization <alias>` watching `harbor-engineering-workspace` at `./engineers/<alias>`.

## What reconciles on terraform apply

`AWS_PROFILE=<chosen> terraform init && terraform plan -target=module.engineers -out=tfplan && terraform apply tfplan` (`<chosen>` = the AWS profile resolved at pre-flight gate 3)

Plan: 6 to add, 0 to change, 0 to destroy. Six resources binding `eng-<alias>/seid-node` and `eng-<alias>/engineer-service-account` to their respective IAM policies via Pod Identity.

## Verification

- `kubectl get sa -n eng-<alias> --context harbor` lists `engineer-service-account`, `<alias>`, `seid-node`.
- `aws eks list-pod-identity-associations --cluster-name harbor --namespace eng-<alias> --region eu-central-1 --profile <chosen>` returns two associations.
```

## When NOT to use this flow

- Non-engineer tenants (CI bot, nightly orchestrator, shared workloads) — different pattern under `clusters/harbor/<bucket>/`.
- Cluster-wide changes (CRD updates, controller config, Flux infrastructure) — separate PR shape.
