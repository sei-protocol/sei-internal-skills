# AWS dependencies

What the skill (and seictl) assume about AWS resources.

## Account & region

| Property | Value |
|---|---|
| Account | `189176372795` |
| EKS region | `eu-central-1` |
| ECR region | `us-east-2` (cross-region — sei-chain CI pushes images there) |

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

**Layout**: `s3://harbor-sei-snapshots/<chainID>/state-sync/<height>.tar.gz`. Each chain publishes a `latest.txt` pointer — useful for engineers picking a target height, but **the sidecar does not read it on restore** (see Mechanism below).

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

**Wiring into a follower SeiNode**: the CRD field is `spec.fullNode.snapshot.s3.targetHeight` (int64, minimum 1; flat on SeiNode — no `spec.template`). Pass via `--set` on `seictl node apply --dry-run`:

```sh
seictl node apply <id>-rpc-<k> --preset rpc --chain-id <id> --image <ref> --network <id> \
  --cpu <cpu> --memory <mem> --storage <size> [--iops <iops> --throughput <tput>] \
  --set spec.fullNode.snapshot.s3.targetHeight=<height> \
  -n eng-<alias> --dry-run
```

Pass the footprint and any storage-performance selection explicitly here, exactly as `ephemeral-chain-flow.md` step 4 requires. Both are create-only. A render that omits them takes the preset shape in silence, and no later apply can correct it.

**Size the data volume for the restored state, not for a dev chain.** The `rpc` preset's `--storage` default is 500Gi, which suits a fresh-genesis chain and not `pacific-1` or `atlantic-2`. The field is create-only. Nothing can resize a follower that outgrows its volume — delete and recreate it instead.

**Do not size from the S3 listing.** `aws s3 ls` reports the byte size of the `<height>.tar.gz` object, which is the compressed archive and not the unpacked database the PVC has to hold. The expansion factor follows from the snapshot's contents, and this runbook pins no such factor. Sizing off the listing therefore undershoots by a multiple, and a headroom allowance does not cover a multiple. Treat the archive size as a floor only.

Measure the restored footprint instead, in this order:

1. **A running node on the same chain** — the direct measurement:

   ```sh
   kubectl exec -n <ns> <pod> -c seid --context=harbor -- du -sh /root/.sei/data
   ```

2. **The provisioned volume of an existing follower on that chain**, when no pod is reachable:

   ```sh
   kubectl get pvc -A --context=harbor \
     -o custom-columns='NS:.metadata.namespace,NAME:.metadata.name,SIZE:.spec.resources.requests.storage'
   ```

   This reports what an operator provisioned rather than what the state occupies, so read it as a peer's judgement and not as a measurement.

3. **Neither available** — surface that the restored size is unmeasured, and agree an explicit `--storage` with the engineer before rendering. Do not derive one from the listing.

Add headroom for continued sync on top of whichever figure the steps above produce.

**Mechanism**: `targetHeight` is a **ceiling, not an exact pin**. The seictl sidecar lists `*.tar.gz` under the chain prefix, parses heights from filenames, and picks `max(height ≤ targetHeight)`. `targetHeight=0` means "use the newest available." If no snapshot ≤ targetHeight exists, the `snapshot-restore` task fails with `no snapshot found at or below height <H>`. `latest.txt` is publisher bookkeeping; the sidecar ignores it. Source: `sei-protocol/seictl/sidecar/tasks/snapshot_restore.go:162-210`.

**Halt conditions**:

- `aws s3 ls` returns `AccessDenied` from the laptop — SSO profile is wrong; re-run pre-flight gate 2.
- No snapshots present under `s3://harbor-sei-snapshots/<chainID>/state-sync/` — snapshot-publisher has not run for that chain. Offer the engineer fresh-sync (omit the snapshot block) or pick a different chain.
- Engineer pins a specific height that does not exist — surface the available heights via `aws s3 ls` and ask them to pick one (or use `0`).

## Pod Identity associations

EKS Pod Identity is the auth mechanism on harbor. All Pod Identity associations are Terraform-managed.

Per `eng-<alias>` namespace (created by the engineer's onboarding `terraform/.../harbor/engineers/<alias>.tf`):

- `eng-<alias>/seid-node` → `aws_iam_policy.seid_node_engineer` (snapshot read, genesis r/w).
- `eng-<alias>/engineer-service-account` → `aws_iam_policy.engineer` (S3 `PutObject` and `ListBucket` on `harbor-validation-results/${aws:PrincipalTag/kubernetes-namespace}/*` — auto-scoped per namespace via the session tag of Pod Identity — plus ECR auth and `sei/sei-chain` image read).

Pre-existing Pod Identity associations for the platform:

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

Image digest resolution flow (used when the agent surfaces a digest in the plan echo before `seictl network|node apply`):

1. `aws ecr describe-images --repository-name sei/sei-chain --region us-east-2 --image-ids imageTag=<tag> --profile <chosen>` (`<chosen>` = the engineer's AWS profile from pre-flight gate 3)
2. Extract `imageDetails[0].imageDigest`
3. Short digest = `sha256:` stripped, first 12 chars
4. Race-guard retry: 3 attempts, 60s sleep — sei-chain CI sometimes pushes after a request lands. Do not loop silently; surface the retry to the engineer.

`seictl network|node apply` itself does not enforce ECR-only images — `--image` accepts any ref the apiserver and downstream pull secrets can resolve. Pre-flight `--image` validation is the agent's responsibility, not the CLI's.

## IAM principals

- GitHub Actions OIDC role for autobake nightly: `arn:aws:iam::189176372795:role/harbor-autobake-gha`
- Engineer IAM principals — SSO-assigned roles (e.g., `arn:aws:iam::189176372795:role/sso-engineer-<alias>`), mapped to k8s groups via `aws_eks_access_entry`.

The onboarding PR shape does not depend on the SSO role's IAM permissions — it only writes to the platform repo.
