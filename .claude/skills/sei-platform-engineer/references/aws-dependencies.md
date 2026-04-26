# AWS dependencies

What the skill (and seictl) assume about AWS resources.

Last verified: 2026-04-26 against `terraform/aws/189176372795/eu-central-1/harbor/`.

## Account & region

| Property | Value |
|---|---|
| Account | `189176372795` |
| EKS region | `eu-central-1` |
| ECR region | `us-east-2` (cross-region — sei-chain images are pushed there) |

## S3 buckets

| Bucket | Purpose | Lifecycle |
|---|---|---|
| `harbor-sei-autobake-results` | Benchmark results (nightly autobake + engineer benchmarks) | 90 days |
| `harbor-sei-snapshots` | Snapshot storage for SeiNode bootstrap | n/a (managed by snapshot-publisher) |
| `harbor-sei-k8s-genesis-artifacts` | Genesis assembly storage | n/a |
| `harbor-sei-shadow-results` | Shadow replayer output | n/a |

[outline: Path conventions per bucket — refer to `intent-benchmark.md` for the autobake-derived path]

## Pod Identity associations

EKS Pod Identity (not IRSA OIDC) is the auth mechanism on harbor. Each `(namespace, ServiceAccount)` tuple gets an explicit `aws_eks_pod_identity_association` in Terraform.

Existing:

- `autobake/seid-node` → `harbor-autobake-seid-node` IAM policy (snapshot read, EC2 describe)
- `autobake/autobake-seiload` → `harbor-autobake-seiload` IAM policy (S3 PutObject to results bucket)

To be added per engineer (the one TF addition for v1):

- `eng-<alias>/bench-seiload` → reuses `harbor-autobake-seiload` policy
  (lives in a new TF file: `terraform/aws/189176372795/eu-central-1/harbor/personal-cells.tf`)

[outline: trust policy template, conditions like `aws:ResourceTag/owner` if scoping is added in cells]

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

When cells land, per-engineer IAM additions become a `for_each` over `local.engineers` in `personal-cells.tf` rather than a copy-paste-fork pattern.
