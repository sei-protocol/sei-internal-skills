# AWS dependencies

What the skill (and seictl) assume about AWS resources.

Last verified: 2026-04-28 against shipped `seictl onboard` (#81) and platform repo's `harbor-validation-results` schema (#72).

## Account & region

| Property | Value |
|---|---|
| Account | `189176372795` |
| EKS region | `eu-central-1` |
| ECR region | `us-east-2` (cross-region — sei-chain images are pushed there) |

## S3 buckets

| Bucket | Purpose | Lifecycle |
|---|---|---|
| `harbor-validation-results` | Engineer benchmark results (and other validation artifacts) | Managed in platform repo |
| `harbor-sei-autobake-results` | Nightly autobake-only results | 90 days |
| `harbor-sei-snapshots` | Snapshot storage for SeiNode bootstrap | n/a (managed by snapshot-publisher) |
| `harbor-sei-k8s-genesis-artifacts` | Genesis assembly storage | n/a |
| `harbor-sei-shadow-results` | Shadow replayer output | n/a |

`harbor-validation-results` uses the schema `<namespace>/<job>/<run>/...`. Engineer benchmarks live under `eng-<alias>/seiload/<bench-name>/`. See `intent-benchmark.md` §S3 results convention.

## Pod Identity associations

EKS Pod Identity (not IRSA OIDC) is the auth mechanism on harbor.

Existing (Terraform-managed in `terraform/aws/189176372795/eu-central-1/harbor/autobake.tf`):

- `autobake/seid-node` → `harbor-autobake-seid-node` IAM policy (snapshot read, EC2 describe)
- `autobake/autobake-seiload` → `harbor-autobake-seiload` IAM policy (S3 PutObject to results bucket)

Per-engineer (created at onboard time via AWS SDK direct, **not** Terraform):

- `eng-<alias>/bench-seiload` → per-engineer scoped policy and role under IAM path `/seictl/`. Policy grants `s3:ListBucket` on `arn:aws:s3:::harbor-validation-results` (with prefix condition `eng-<alias>/*`) and `s3:PutObject` on `arn:aws:s3:::harbor-validation-results/eng-<alias>/*`. Trust policy is `pods.eks.amazonaws.com` with `sts:AssumeRole + sts:TagSession` (Pod Identity requires both) and confused-deputy conditions on `eks:cluster-name` + `kubernetes.io/namespace`. Shared policies are explicitly rejected as a security risk that doesn't scale.
- `seictl onboard --apply` performs `iam:CreatePolicy`, `iam:CreateRole`, `iam:AttachRolePolicy`, `eks:CreatePodIdentityAssociation` in the engineer's SSO session. Idempotent: pre-existing resources are detected by ARN and reported as `action: "exists"` with no mutation; a Pod Identity association bound to a different role is a hard failure.

## ECR

| Repo | Purpose |
|---|---|
| `189176372795.dkr.ecr.us-east-2.amazonaws.com/sei/sei-chain` | The seid image autobake + benchmarks consume |

Image digest resolution flow (used by `seictl bench up` pre-flight, mirroring `k8s_autobake.yml`):

1. `aws ecr describe-images --repository-name sei/sei-chain --region us-east-2 --image-ids imageTag=<tag>`
2. Extract `imageDetails[0].imageDigest`
3. Short digest = `sha256:` stripped, first 12 chars
4. Race-guard retry: 3 attempts, 60s sleep — sei-chain CI sometimes pushes after the benchmark is requested

## IAM principals

[outline]

- GitHub Actions OIDC role for autobake nightly: `arn:aws:iam::189176372795:role/harbor-autobake-gha`
- Engineer IAM principals (today: SSO-assigned roles like `arn:aws:iam::189176372795:role/sso-engineer-<alias>`) — mapped to k8s groups via `aws_eks_access_entry`

Engineer's SSO role currently has admin permissions (sufficient for `iam:CreatePolicy`, `iam:CreateRole`, `iam:AttachRolePolicy`, `eks:CreatePodIdentityAssociation`). When SSO permissions get scoped down, the onboard flow revisits.
