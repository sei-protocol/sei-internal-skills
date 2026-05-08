# Onboarding PR (the one-time tenant registration)

A new engineer's onboarding is one PR against `sei-protocol/platform` adding three files. After the PR merges, run a targeted `terraform apply`. Both pieces complete in under five minutes.

Last verified: 2026-05-06 against sei-protocol/platform#432 (the fromtherain pilot — the canonical example).

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

A second `replacements:` block substitutes `eng-tenant` → `eng-<alias>` (delimiter `-`, index 1) for namespace fields, and `engineers/tenant` → `engineers/<alias>` (delimiter `/`, index 2) for the Flux Kustomization's `spec.path`.

Fetch the canonical content with `gh pr diff 432 --repo sei-protocol/platform -- clusters/harbor/engineers/fromtherain/kustomization.yaml` and substring-replace `fromtherain` → `<alias>`.

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
- `additional_policy_arns` → either `seid_node = var.iam_policy_seid_node_arn` or `engineer = var.iam_policy_engineer_arn`.
- `associations.this = { cluster_name = var.cluster_name, namespace = "eng-<alias>", service_account = "<sa-name>" }`.
- `tags = var.tags`.

Fetch canonical with `gh pr diff 432 --repo sei-protocol/platform -- terraform/aws/189176372795/eu-central-1/harbor/engineers/fromtherain.tf` and substring-replace `fromtherain` → `<alias>`.

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

Shared Terraform at `terraform/aws/189176372795/eu-central-1/harbor/`:

- `engineers-shared.tf` — `aws_iam_policy.engineer` (`s3:PutObject` and `s3:ListBucket` on `harbor-validation-results/${aws:PrincipalTag/kubernetes-namespace}/*`, ECR auth, `sei/sei-chain` image read).
- `sei-k8s-controller.tf` — `aws_iam_policy.seid_node` (snapshot read, genesis r/w, `ec2:DescribeInstances`).
- `engineers.tf` — `module "engineers"` wrapper passing `cluster_name`, `name_prefix`, `iam_policy_seid_node_arn`, `iam_policy_engineer_arn`, `tags` into the submodule.

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

After merge, run from `terraform/aws/189176372795/eu-central-1/harbor/` with the AWS profile resolved at pre-flight gate 2:

```sh
AWS_PROFILE=<chosen> terraform plan -target=module.engineers -out=tfplan
AWS_PROFILE=<chosen> terraform apply tfplan
```

`<chosen>` is the engineer's AWS profile (resolved by pre-flight gate 2 — never literal `sei`; engineers configure their own).

`Plan: 6 to add, 0 to change, 0 to destroy`. Apply confirms `Resources: 6 added`. The six resources:

- `module.engineers.module.eng_<alias>_seid_node_pod_identity.{aws_iam_role.this[0], aws_iam_role_policy_attachment.this["seid_node"], aws_eks_pod_identity_association.this["this"]}` — binds `(harbor, eng-<alias>, seid-node)` to `aws_iam_policy.seid_node`.
- `module.engineers.module.eng_<alias>_engineer_pod_identity.{aws_iam_role.this[0], aws_iam_role_policy_attachment.this["engineer"], aws_eks_pod_identity_association.this["this"]}` — binds `(harbor, eng-<alias>, engineer-service-account)` to `aws_iam_policy.engineer`.

Pods running as `engineer-service-account` see `aws:PrincipalTag/kubernetes-namespace=eng-<alias>` in the assumed-role session; S3 writes auto-scope to `harbor-validation-results/eng-<alias>/*`. SeiNode pods running as `seid-node` get snapshot read + peer discovery via `aws_iam_policy.seid_node`.

## The agent's job

1. **Prompt for the alias.** Default the prompt to `$USER` lowercased — don't silently use it. Validate the response against `^[a-z]([a-z0-9-]{0,28}[a-z0-9])?$`. **Then check uniqueness** with `kubectl get namespace eng-<alias> --context harbor`: if the namespace already exists, the alias is taken — halt with "pick another, or contact the platform team if it's yours." Don't attempt partial-state recovery (separate runbook). Continue only when the alias is free.
2. **Fetch the diff template.** `gh pr list --repo sei-protocol/platform --search "feat(harbor/engineers): onboard" --state merged --limit 1` returns the most recent prior onboarding PR. `gh pr diff <num>` gives the three-file content.
3. **Render the three files locally** in a fresh clone of `sei-protocol/platform`. Substring-replace the prior alias with `<alias>` throughout. Branch: `feat/engineers-<alias>-onboard`.
4. **Open the platform-repo PR.** Title: `feat(harbor/engineers): onboard <alias>`. Body: see template below. `gh pr create --repo sei-protocol/platform --base main`.
5. **Open the workspace-repo scaffolding PR (sibling).** Without this, the engineer's per-engineer Flux Kustomization fails reconcile post-merge with `kustomization path not found: ./engineers/<alias>` — the bug pattern that broke amir for ~7 hours after onboarding (see platform#446 / harbor-engineering-workspace#1). Open against `sei-protocol/harbor-engineering-workspace`:

   ```sh
   # In a fresh clone of sei-protocol/harbor-engineering-workspace
   git checkout -b feat/onboard-<alias> main
   mkdir -p engineers/<alias>
   cat > engineers/<alias>/kustomization.yaml <<'EOF'
   apiVersion: kustomize.config.k8s.io/v1beta1
   kind: Kustomization
   resources: []
   EOF
   git add engineers/<alias>/kustomization.yaml
   git commit -m "feat(engineers): scaffold workspace dir for <alias>"
   git push -u origin feat/onboard-<alias>
   gh pr create --repo sei-protocol/harbor-engineering-workspace --base main \
     --title "feat(engineers): scaffold workspace dir for <alias>" \
     --body "Sibling to sei-protocol/platform#<onboarding-pr-num>. Empty resources list — the engineer fills this in via subsequent task PRs (chain spinups, benches). Required so the per-engineer Flux Kustomization can reconcile cleanly on first apply."
   ```

   Both PRs land independently of each other; the platform-repo PR shouldn't merge before the workspace-repo PR is at least open. Surface both URLs.

6. **Surface and halt:**
   > Onboarding opened in two PRs:
   > - Platform: `<platform-pr-url>`
   > - Workspace: `<workspace-pr-url>`
   >
   > Merge the workspace PR first (or merge both within seconds of each other). After merge of the platform PR, Flux reconciles namespace + RBAC + Flux watcher in ~60s. Then run `AWS_PROFILE=<chosen> terraform apply -target=module.engineers` from `terraform/aws/189176372795/eu-central-1/harbor/` to land the Pod Identity associations. (`<chosen>` is the AWS profile resolved at pre-flight gate 2 — never literal `sei`.)
7. **After merge,** poll `kubectl get namespace eng-<alias> --context harbor` until it returns 0.
8. **Run the targeted apply.** `terraform plan -target=module.engineers -out=tfplan` then `terraform apply tfplan`. Confirm `Resources: 6 added`.
9. **Verify** `aws eks list-pod-identity-associations --cluster-name harbor --namespace eng-<alias> --region eu-central-1 --profile <chosen>` returns two associations (one for `seid-node`, one for `engineer-service-account`). Verify the engineer's Flux Kustomization is Ready: `kubectl get kustomization eng-<alias> -n flux-system -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'` returns `True`.

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

`AWS_PROFILE=<chosen> terraform apply -target=module.engineers` (`<chosen>` = the AWS profile resolved at pre-flight gate 2)

Plan: 6 to add, 0 to change, 0 to destroy. Six resources binding `eng-<alias>/seid-node` and `eng-<alias>/engineer-service-account` to their respective IAM policies via Pod Identity.

## Verification

- `kubectl get sa -n eng-<alias> --context harbor` lists `engineer-service-account`, `<alias>`, `seid-node`.
- `aws eks list-pod-identity-associations --cluster-name harbor --namespace eng-<alias> --region eu-central-1 --profile <chosen>` returns two associations.
```

## When NOT to use this flow

- Non-engineer tenants (CI bot, nightly orchestrator, shared workloads) — different pattern under `clusters/harbor/<bucket>/`.
- Cluster-wide changes (CRD updates, controller config, Flux infrastructure) — separate PR shape.
