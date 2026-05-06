# AWS dependencies

What the skill (and seictl) assume about AWS resources.

Last verified: 2026-05-05 against the harbor multi-tenancy onboarding pattern (sei-protocol/platform#427) and `seictl nd` v0.0.43+ (post-#133 verb tree).

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

EKS Pod Identity (not IRSA OIDC) is the auth mechanism on harbor.

Existing (Terraform-managed in `terraform/aws/189176372795/eu-central-1/harbor/autobake.tf`):

- `autobake/seid-node` → `harbor-autobake-seid-node` IAM policy (snapshot read, EC2 describe)
- `autobake/autobake-seiload` → `harbor-autobake-seiload` IAM policy (S3 PutObject to results bucket)

Per-engineer (deferred — tracked at sei-protocol/platform#426):

- `eng-<alias>/workload-service-account` ships from the base layer as a stub — no `eks.amazonaws.com/role-arn` annotation, no Pod Identity association.
- The intended target is **Pattern C**: per-purpose IAM roles (e.g., `harbor-snapshot-reader`, `harbor-results-writer`) with wildcard OIDC trust matching `eng-*/<purpose>` ServiceAccount patterns. An engineer who needs S3 read/write attaches the appropriate role to their workload SA via a follow-up PR.
- Until Pattern C lands, in-namespace workloads needing AWS access either run with no AWS access or use a one-off Pod Identity association created by the platform team. Flag it on the onboarding PR.

Engineer-side AWS access (from the engineer's laptop, not from a workload Pod) uses the engineer's SSO role directly — not the workload SA.

## ECR

| Repo | Purpose |
|---|---|
| `189176372795.dkr.ecr.us-east-2.amazonaws.com/sei/sei-chain` | The seid image autobake + benchmarks consume |

Image digest resolution flow (used when the agent surfaces a digest in the plan echo before `seictl nd apply`):

1. `aws ecr describe-images --repository-name sei/sei-chain --region us-east-2 --image-ids imageTag=<tag> --profile sei`
2. Extract `imageDetails[0].imageDigest`
3. Short digest = `sha256:` stripped, first 12 chars
4. Race-guard retry: 3 attempts, 60s sleep — sei-chain CI sometimes pushes after a request lands. Don't loop silently; surface the retry to the engineer.

`seictl nd apply` itself does not enforce ECR-only images — `--image` accepts any ref the apiserver and downstream pull secrets can resolve. Pre-flight `--image` validation is the agent's responsibility, not the CLI's.

## IAM principals

[outline]

- GitHub Actions OIDC role for autobake nightly: `arn:aws:iam::189176372795:role/harbor-autobake-gha`
- Engineer IAM principals (today: SSO-assigned roles like `arn:aws:iam::189176372795:role/sso-engineer-<alias>`) — mapped to k8s groups via `aws_eks_access_entry`

Engineer's SSO role currently has admin permissions in the cluster account, which is sufficient for read-side ECR + S3 access from the laptop. When SSO permissions get scoped down, the access-entry surface and any laptop-side AWS reads revisit. The onboarding PR shape itself doesn't depend on the SSO role's IAM permissions — it only writes to the platform repo.
