# AWS dependencies

What the skill (and seictl) assume about AWS resources.

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

`harbor-validation-results` uses the schema `<namespace>/<job>/<run>/...`. Engineer-driven runs (when an engineer composes a chain + a hand-rolled seiload Job) land under `eng-<alias>/seiload/<run-name>/`. The nightly autobake orchestrator and the release-test CronJob both write to this bucket as well, under their own namespace prefixes.

## Pod Identity associations

EKS Pod Identity is the auth mechanism on harbor. All Pod Identity associations are Terraform-managed.

Per `eng-<alias>` namespace (created by the engineer's onboarding `terraform/.../harbor/engineers/<alias>.tf`):

- `eng-<alias>/seid-node` → `aws_iam_policy.seid_node_engineer` (snapshot read, genesis r/w).
- `eng-<alias>/engineer-service-account` → `aws_iam_policy.engineer` (S3 `PutObject` and `ListBucket` on `harbor-validation-results/${aws:PrincipalTag/kubernetes-namespace}/*` — auto-scoped per namespace via Pod Identity session tag — plus ECR auth and `sei/sei-chain` image read).

Pre-existing platform Pod Identity associations:

- `nightly/seid-node` → `aws_iam_policy.seid_node`.
- `nightly/workload-service-account` → `aws_iam_policy.nightly_workload`.
- `pacific-1/seid-node` → `aws_iam_policy.seid_node`.
- `autobake/seid-node` → `harbor-autobake-seid-node` IAM policy.
- `autobake/autobake-seiload` → `harbor-autobake-seiload` IAM policy.

Engineer-side AWS access (from the engineer's laptop, not from a Pod) uses the engineer's SSO role directly.

## ECR

| Repo | Purpose |
|---|---|
| `189176372795.dkr.ecr.us-east-2.amazonaws.com/sei/sei-chain` | The seid image autobake + benchmarks consume |

Image digest resolution flow (used when the agent surfaces a digest in the plan echo before `seictl nd apply`):

1. `aws ecr describe-images --repository-name sei/sei-chain --region us-east-2 --image-ids imageTag=<tag> --profile <chosen>` (`<chosen>` = the engineer's AWS profile from pre-flight gate 3)
2. Extract `imageDetails[0].imageDigest`
3. Short digest = `sha256:` stripped, first 12 chars
4. Race-guard retry: 3 attempts, 60s sleep — sei-chain CI sometimes pushes after a request lands. Don't loop silently; surface the retry to the engineer.

`seictl nd apply` itself does not enforce ECR-only images — `--image` accepts any ref the apiserver and downstream pull secrets can resolve. Pre-flight `--image` validation is the agent's responsibility, not the CLI's.

## IAM principals

- GitHub Actions OIDC role for autobake nightly: `arn:aws:iam::189176372795:role/harbor-autobake-gha`
- Engineer IAM principals — SSO-assigned roles (e.g., `arn:aws:iam::189176372795:role/sso-engineer-<alias>`), mapped to k8s groups via `aws_eks_access_entry`.

The onboarding PR shape doesn't depend on the SSO role's IAM permissions — it only writes to the platform repo.
