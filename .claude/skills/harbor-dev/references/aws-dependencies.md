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

## Snapshot discovery (harbor-sei-snapshots)

Use this when an engineer wants to bootstrap an RPC fleet from a state-sync snapshot instead of a fresh sync (much faster for `pacific-1` / `atlantic-2`). Engineers have read access via SSO (pre-flight gate 2). SeiNode pods read via Pod Identity (`aws_iam_policy.seid_node_engineer`). Bucket lives in `eu-central-1`.

**Layout**: `s3://harbor-sei-snapshots/<chainID>/state-sync/<height>.tar.gz`. A `latest.txt` pointer is published per chain — useful for engineers picking a target height, but **the sidecar does not read it on restore** (see Mechanism below).

**List available snapshot heights**:

```sh
aws s3 ls s3://harbor-sei-snapshots/<chainID>/state-sync/ \
  --region eu-central-1 --profile <chosen>
```

**Read the latest published height (engineer-side reference only)**:

```sh
aws s3 cp s3://harbor-sei-snapshots/<chainID>/state-sync/latest.txt - \
  --region eu-central-1 --profile <chosen>
```

**Common chain IDs**: `pacific-1` (mainnet), `atlantic-2` (testnet), `arctic-1` (devnet). Confirm with `aws s3 ls s3://harbor-sei-snapshots/ --region eu-central-1 --profile <chosen>`.

**Wiring into a SeiNodeDeployment**: the CRD field is `spec.template.spec.fullNode.snapshot.s3.targetHeight` (int64, minimum 1). Pass via `--set` on `seictl nd apply --dry-run`:

```sh
seictl nd apply <id>-rpc --preset rpc --chain-id <id> --image <ref> \
  --set spec.template.spec.fullNode.snapshot.s3.targetHeight=<height> \
  -n eng-<alias> --dry-run
```

**Mechanism**: `targetHeight` is a **ceiling, not an exact pin**. The seictl sidecar lists `*.tar.gz` under the chain prefix, parses heights from filenames, and picks `max(height ≤ targetHeight)`. `targetHeight=0` means "use the newest available." If no snapshot ≤ targetHeight exists, the `snapshot-restore` task fails with `no snapshot found at or below height <H>`. `latest.txt` is publisher bookkeeping; the sidecar ignores it.

**Halt conditions**:

- `aws s3 ls` returns `AccessDenied` from the laptop — SSO profile is wrong; re-run pre-flight gate 2.
- No snapshots present under `s3://harbor-sei-snapshots/<chainID>/state-sync/` — snapshot-publisher hasn't run for that chain. Offer the engineer fresh-sync (omit the snapshot block) or pick a different chain.
- Engineer pins a specific height that doesn't exist — surface the available heights via `aws s3 ls` and ask them to pick one (or use `0`).

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
