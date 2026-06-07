# TF cell standup — prod-use2 + prod-euw1

Status: draft
Sequel to: PR sei-protocol/platform#883 (clusters/base/ refactor + dormant cell manifests)
Driver: arctic-1 chain requires regionally distributed validators; this workstream provisions the AWS infrastructure (VPC + EKS + IAM + KMS + ECR access + DNS + Flux bootstrap) that activates the dormant `clusters/prod-use2/` + `clusters/prod-euw1/` trees already merged on main.

## 1. Background

PR #883 landed two dormant cell manifest trees:
- `clusters/prod-use2/` — Kustomize root targeting us-east-2
- `clusters/prod-euw1/` — Kustomize root targeting eu-west-1

Both trees are byte-validated to render + dry-run apply cleanly. They sit unused on main because:
1. The target EKS clusters don't exist yet
2. No Flux source-controller is pointed at them
3. No Route 53 zone, KMS key, IAM role, or ECR access exists for the new regions

This workstream creates the AWS infrastructure that lights both cells up. After it lands, applying the TF + Flux bootstrap activates the manifests from #883, and the cells begin reconciling.

## 2. Goals

1. Provision two new EKS cells: `prod-use2` (us-east-2) and `prod-euw1` (eu-west-1)
2. Each cell stands alone — public ingress, no cross-cell dependencies at the network layer
3. Cells are prod-shaped (Cilium CNI, KMS-encrypted EBS, ECR-pinned images, multi-AZ NAT)
4. Flux source-controller in each cell pointed at its `clusters/<cell>/` tree from PR #883
5. Per-cell trust boundary: SOPS key, IAM roles, Pod Identity associations are all per-cell, not shared
6. After `terraform apply` + cell-subsystem-manifests precursor PR, the cells reconcile cleanly with no manual intervention

## 2.5 Prerequisites — cell-subsystem-manifests precursor PR

PR #883 added the `cni-cilium` Kustomize Component (patches that adapt kube-system HelmReleases for Cilium) but did NOT add the underlying `kube-system/` and `istio-system/` HelmReleases for the new cells. Without them, the cells reconcile-stall on Flux bootstrap — no Cilium → no pod networking; no AWS LB Controller → no NLBs; no istiod → Gateway has no class; no Karpenter → no nodes.

A **precursor PR against sei-protocol/platform** must land before TF apply, mirroring harbor's Cilium-based pattern:

- `clusters/prod-use2/kube-system/{cilium,aws-load-balancer-controller,karpenter,metrics-server}.yaml`
- `clusters/prod-use2/istio-system/{istiod,namespace}.yaml` (and any base/overlay needed)
- `clusters/prod-euw1/kube-system/{...}.yaml` (mirror)
- `clusters/prod-euw1/istio-system/{...}.yaml` (mirror)
- Plus referencing those resources from `clusters/<cell>/kustomization.yaml` (currently lists `cert-manager`, `external-dns`, `default`, `gateway`, `sei-k8s-controller`, `arctic-1` only)

The precursor PR is a copy of harbor's `kube-system/` + `istio-system/` trees with cell-name substitution. **Substitutions required**: KMS key ARN/alias (cell-scoped), region tags, Karpenter `EC2NodeClass.subnetSelectorTerms` discovery tag value (`prod-use2` / `prod-euw1` — copy-paste from harbor would leave the value as `harbor` and Karpenter scale-ups would fail silently with empty subnet selectors). Cilium cluster-pool pod CIDR stays at `100.64.0.0/14` (same across cells — fine because cross-cell peering is deferred; flag for the eventual peering workstream).

**Pin policy**: every `chart.spec.version` must be an exact version pin matching harbor's value at copy-time. Image refs use digests wherever harbor does. Floating tags from harbor must NOT leak into prod cells.

**Scope-cut discipline**: this precursor adds `kube-system/` + `istio-system/` only. **Do not** pull in `monitoring/` or `flux-system/` overlays — those are explicitly Category D, deferred.

Byte-validates the same way PR #883 did: `kubectl kustomize`, dry-run apply, confirm no unexpected manifest additions vs harbor's render.

**Merge order**: precursor PR first; then this TF workstream can apply and Flux bootstrap pulls the now-complete cell trees.

## 3. Non-goals

These are deferred to separate workstreams (and exist on their own un-defer triggers):

- **Cross-region VPC peering** — each cell uses public internet for any cross-cell traffic. Un-defer if bandwidth costs from cross-cell P2P chatter exceed the cost of peering, or if Thanos cross-region scrape lands.
- **`eu-west-1/common/`** — no region-local resources needed in v1. ECR stays in us-east-2 (with replication, see §4.5); GHA OIDC is account-scoped. Un-defer when a region-local resource (regional KMS multi-region key, SES, SSM Parameter Store) genuinely appears.
- **arctic-1 SeiNodeDeployments** — TF only ships infra + Flux bootstrap. The validator manifests come in the next workstream (chain-ops migration of EC2 validators to K8s).
- **Cell-side monitoring stack** — `clusters/<cell>/monitoring/` (Thanos federation, Loki, Alloy) is part of Category D extraction, separate workstream. Cells run no observability in v1; metrics flow when Category D lands.
- **Cell-side kube-system / istio-system / flux-system overlay** — Category D extraction. Cells run Cilium + sei-k8s-controller + cert-manager + external-dns + Gateway API only in v1.
- **GHA OIDC subject claim tightening + TF state KMS key-policy tightening** — security follow-ups flagged in Phase 0 discovery; not in this PR's scope.
- **VPC interface endpoints (ECR, S3, secretsmanager)** — defer until NAT egress cost shows up as a budget line or a security audit requires private-only image pulls.

## 4. Architecture

### 4.1 TF root filesystem layout

```
terraform/aws/189176372795/
├── bootstrap/                          # account-scoped (existing, unchanged)
├── eu-central-1/
│   ├── prod/                           # existing — small edit: add outputs for child NS delegation
│   └── harbor/                         # existing, unchanged
├── us-east-2/
│   ├── common/                         # existing — single edit: add ECR replication config
│   ├── dev/                            # existing, unchanged
│   └── prod-use2/                      # NEW
│       ├── backend.tf
│       ├── versions.tf
│       ├── locals.tf
│       ├── main.tf
│       ├── outputs.tf
│       ├── vpc.tf
│       ├── eks.tf
│       ├── kms.tf
│       ├── karpenter.tf
│       ├── aws-lb-controller.tf
│       ├── cert-manager.tf
│       ├── external-dns.tf
│       ├── route53.tf
│       ├── flux.tf
│       └── sei-k8s-controller.tf
└── eu-west-1/                          # NEW region (no common/)
    └── prod-euw1/                      # NEW, mirrors prod-use2 modulo per-cell parameters
        └── (same file set)
```

Per-cell `outputs.tf` exports the four values with concrete v1 consumers: `cluster_name`, `external_dns_zone_name`, `external_dns_zone_id`, `sops_kms_key_arn`. Add others (cluster endpoint, VPC id) when something reads them.

Single edit to `us-east-2/common/ecr.tf` adds replication (see §4.5). Single edit to `eu-central-1/prod/outputs.tf` exposes `platform_sei_io_zone_id` for the new cells' NS delegation (see §4.6). `sei.io` is a same-account data-source lookup in each cell's TF, so no prod-output is needed for it.

### 4.2 Per-cell parameter inventory

| Field | prod-use2 | prod-euw1 |
|---|---|---|
| Region | us-east-2 | eu-west-1 |
| VPC CIDR | 10.10.0.0/16 | 10.70.0.0/16 |
| AZ count | 3 (selected via `data.aws_availability_zones { state="available" }`, no hard-pin) | 3 (same; eu-west-1 has 3 generally-available AZs; data-source filter handles any historical AZ-allocation quirks per account) |
| NAT topology | per-AZ (3 NAT GWs) | per-AZ (3 NAT GWs) |
| EKS version | (match dev's current pin; lock in implementation PR) | (same) |
| EKS endpoint | private + public (mirror dev's `cluster_endpoint_public_access_cidrs` set if dev pins; otherwise `0.0.0.0/0` with auth-only gate, same as dev) | same |
| EKS managed addons | `coredns`, `eks-pod-identity-agent`, `kube-proxy`, `vpc-cni` (matches dev) | same |
| Subnet layout | 3 public /20 + 3 private /19 + 3 intra /22 (mirror `us-east-2/dev/vpc.tf`) | same |
| Required subnet tags | `kubernetes.io/role/elb=1` (public), `kubernetes.io/role/internal-elb=1` (private), `karpenter.sh/discovery=<cell>` (private) — applied via `module.vpc.public_subnet_tags` / `private_subnet_tags` | same |
| KMS SOPS alias | `alias/prod-use2` | `alias/prod-euw1` |
| Route 53 child zone | `prod-use2.platform.sei.io` | `prod-euw1.platform.sei.io` |
| TF state key | `platform/us-east-2/prod-use2/terraform.tfstate` | `platform/eu-west-1/prod-euw1/terraform.tfstate` |
| TF state locking | S3-native (TF ≥1.10 lockfile in state bucket; no DynamoDB table) | same |
| Flux path | `./clusters/prod-use2` | `./clusters/prod-euw1` |
| Karpenter discovery tag | `prod-use2` | `prod-euw1` |
| sei-k8s-controller chain map | `arctic-1` only | `arctic-1` only |
| Pod Identity role prefix | `prod-use2-` | `prod-euw1-` |
| S3 buckets | `prod-use2-sei-snapshots` (SSE-S3, default; same as existing prod bucket pattern — no SSE-KMS overhead for non-secret artifacts) | `prod-euw1-sei-snapshots` (same) |
| Cilium pod CIDR | `100.64.0.0/14` (CGNAT, NOT VPC-routable, matches harbor) | same |

CIDR scheme is non-overlapping with the existing allocations: `10.0` (dev), `10.50` (prod), `10.60` (harbor). `10.10` keeps us-east-2 cells contiguous; `10.70` clears harbor in the EU block. Cilium pod CIDR `100.64.0.0/14` is reserved CGNAT — never overlaps any VPC CIDR (one-way door, do not change).

EKS managed addons follow dev's pattern: `vpc-cni` and `eks-pod-identity-agent` ship with `before_compute = true` so they're available before any node joins. Note: `vpc-cni` ships as an addon but is replaced at the dataplane by Cilium (PR #883's `cni-cilium` Component patches its DaemonSet to hostNetwork; the addon's CRDs + IAM remain for compatibility).

Cilium HelmRelease + AWS LB Controller + Karpenter HelmRelease + metrics-server HelmRelease all ship via the cell-subsystem-manifests precursor PR (§2.5), not TF. TF wires the Pod Identity associations + Karpenter module IAM; the cluster's `kube-system/karpenter.yaml` HelmRelease installs the controller pod.

### 4.3 IAM trust boundary catalog

Each cell creates its own Pod Identity associations (no IRSA/OIDC; Pod Identity Agent is trust-bound to the EKS cluster name + namespace + SA tuple, so OIDC issuer forgery is not a concern).

| Service | Role | NS/SA | Policy scope |
|---|---|---|---|
| AWS Load Balancer Controller | `<cell>-aws-lb-controller` | `kube-system/aws-load-balancer-controller` | Module-canned (`attach_aws_lb_controller_policy`) |
| cert-manager | `<cell>-cert-manager` | `cert-manager/cert-manager` | Route 53 `ChangeResourceRecordSets` on the cell's zone ARN |
| external-dns | `<cell>-external-dns` | `external-dns/external-dns` | Route 53 `ChangeResourceRecordSets` on the cell's zone ARN |
| Karpenter controller | (module-managed) | `kube-system/karpenter` | Module canned |
| Karpenter nodes | `<cell>` (cluster role, matches local.name) | EC2 instances | `AmazonSSMManagedInstanceCore` + EKS worker policies |
| Flux kustomize-controller | `<cell>-flux-kustomize-controller` | `flux-system/kustomize-controller` | `kms:Decrypt` on cell SOPS key only |
| sei-k8s-controller | `<cell>-sei-k8s-controller` | `sei-k8s-controller-system/sei-k8s-controller-manager` | RW on `<cell>-sei-snapshots`, `ec2:DescribeInstances *` |

The `arctic-1` seid-node Pod Identity role + `<cell>-sei-snapshots` bucket prefix policy ship in the **next workstream** (arctic-1 SeiNodeDeployment migration) when the `arctic-1/seid-node` SA actually exists.

**Hardening for v1:** no role is reachable from another cell. The only shared trust boundary is the GHA OIDC role (`role/common/gha`) for ECR push — flagged in Phase 0 (Phase 0 confirmed per-repo enumeration in `us-east-2/common/gha.tf`, not org wildcard, so the un-defer trigger is narrower than initially framed). Deferred to a separate security follow-up.

### 4.4 KMS topology

Per-cell SOPS key, no multi-region replication.

| Key | Alias | Region | Replicated? |
|---|---|---|---|
| prod-use2 SOPS | `alias/prod-use2` | us-east-2 | no |
| prod-euw1 SOPS | `alias/prod-euw1` | eu-west-1 | no |

Rationale: a multi-region replica would let any cell's Flux decrypt any cell's secrets. SOPS files in the repo are encrypted to the destination cell's key only; Flux in cell A literally cannot read cell B's ciphertext. This is a hard one-way door — multi-region SOPS keys cannot be retrofitted without a re-encryption pass.

Key policy follows the existing prod pattern: account-root admin, SSO `AWSReservedSSO_EngWithLimitedPacific-1_*` encrypt/decrypt for engineer access, Pod Identity for Flux kustomize-controller decrypt.

The TF state KMS key (`bootstrap_s3_bucket_kms_key`) stays as-is — single shared key for `sei-platform-terraform-state`. Tightening its policy is a known follow-up.

### 4.5 ECR access

ECR is single-region in `us-east-2` (account-wide repos at `us-east-2/common/ecr.tf`). For cross-region pulls:

- **prod-use2 → us-east-2 ECR**: in-region, trivial. NAT egress only.
- **prod-euw1 → us-east-2 ECR**: cross-region without replication = NAT egress + public ECR endpoint + cross-region transfer + cold-pull latency on Karpenter scale-ups.

**Decision: enable ECR replication us-east-2 → eu-west-1 only.** Same-account replication (`registry_id` = current account), so no new trust path. eu-central-1 replication would close existing prod-cell pain but that's a different workstream — keeping this PR scoped to what prod-euw1 needs.

```hcl
resource "aws_ecr_replication_configuration" "this" {
  replication_configuration {
    rule {
      destination {
        region      = "eu-west-1"
        registry_id = data.aws_caller_identity.current.account_id
      }
      repository_filter {
        filter      = "sei/*"
        filter_type = "PREFIX_MATCH"
      }
    }
  }
}
```

All current repos in `us-east-2/common/ecr.tf` (`sei/genesis`, `sei/seid`, `sei/sei-chain`, `sei/sei-k8s-controller`, `sei/seitask-runner`, `sei/sei-cosmos-exporter`, `sei/actions-runner`, `sei/release-test`, `sei/build-cache`) are covered by `PREFIX_MATCH sei/*`. **All new ECR repos MUST use the `sei/` prefix**, or the replication config must be amended; non-prefixed repos silently won't replicate. Note: `aws_ecr_replication_configuration` is one resource per source region per account — verify no existing config on us-east-2 ECR before applying (current `bootstrap` state shows `replication_configuration: []`, confirmed clear).

Cost: ~1x extra ECR storage in eu-west-1 for replicated repos. Latency: cold-pull stays in-region for prod-euw1. Karpenter scale-up is the load-bearing case.

### 4.6 Route 53 parent-zone NS delegation

The public parent zone `platform.sei.io` lives in `eu-central-1/prod/route53.tf`. The grandparent `sei.io` is a data lookup in the same account. Existing convention (load-bearing comment block in prod's route53.tf): **both `sei.io` AND `platform.sei.io` must hold child NS records for every cell**, because resolvers that cache the more-specific `platform.sei.io` delegation never traverse back to `sei.io`. Mirroring prod's `prod_subdomain_ns` / `dev_subdomain_ns` / `harbor_subdomain_ns` pattern, new cells write into both zones.

**Decision: use `terraform_remote_state` to read the parent + grandparent zone IDs from `eu-central-1/prod`'s state.** The new cell's `route53.tf` declares:

```hcl
data "aws_route53_zone" "parent" {
  name = "sei.io"
}

resource "aws_route53_zone" "external_dns_domain" {
  name = "${module.eks.cluster_name}.platform.${data.aws_route53_zone.parent.name}"
  tags = local.tags
}

resource "aws_route53_record" "domain_ns_in_sei" {
  zone_id = data.aws_route53_zone.parent.zone_id
  name    = aws_route53_zone.external_dns_domain.name
  type    = "NS"
  ttl     = 300
  records = aws_route53_zone.external_dns_domain.name_servers
}

data "terraform_remote_state" "parent_dns" {
  backend = "s3"
  config = {
    bucket = "sei-platform-terraform-state"
    key    = "platform/eu-central-1/prod/terraform.tfstate"
    region = "us-east-2"
  }
}

resource "aws_route53_record" "domain_ns_in_platform" {
  zone_id = data.terraform_remote_state.parent_dns.outputs.platform_sei_io_zone_id
  name    = aws_route53_zone.external_dns_domain.name
  type    = "NS"
  ttl     = 300
  records = aws_route53_zone.external_dns_domain.name_servers
}
```

`sei.io` is the data-source lookup (same account; lookup works directly). `platform.sei.io` is a managed resource in prod's state, hence the remote-state read.

**TF runner principal**: cross-state read requires `s3:GetObject` on `s3://sei-platform-terraform-state/platform/eu-central-1/prod/terraform.tfstate` and `kms:Decrypt` on the bootstrap state KMS key. The current TF runner is engineer SSO (`AWSReservedSSO_AdministratorAccess_*`), which already holds both. No new IAM. When a CI runner identity is added (deferred), it must inherit the same — surface this in the §8 trigger.

Caveat: the remote-state read leaks all of prod's outputs, not just zone IDs. Prod's existing outputs are non-sensitive (`vpc_id`, `vpc_cidr`, `region`, `thanos_bucket_*`). Acceptable for v1; document in the PR body that adding sensitive outputs to prod's state has cross-state-read implications.

Prereq: `eu-central-1/prod/outputs.tf` must export `platform_sei_io_zone_id` (from `aws_route53_zone.public_domain.zone_id`). One new output. Prod plan diff: outputs-only, no resource changes. (Skipping `sei_io_zone_id`: `sei.io` is a data-source lookup in each cell's TF directly — no prod-output required.)

**Why not factor the parent zone to a `common/` state:** that's a larger refactor touching the existing prod cell's state. Two new cells don't justify it; un-defer when a third cell or a non-prod-state parent zone shows up.

### 4.7 Flux bootstrap shape

Mirrors the existing prod and dev pattern (`flux.tf` is byte-identical in shape across both today). Per-cell changes:

- `flux_bootstrap_git.this.path = "clusters/<cell>"`
- `flux_bootstrap_git.this.toleration_keys = ["CriticalAddonsOnly"]` (prod pattern)
- `kustomization_override` injects `decryption.provider: sops` on the root Kustomization
- Deploy key is `read_only = true`. Re-key when image-automation actually lands (two-way door, low cost).
- SOPS decryption uses **Pod Identity only** — no `secretRef` on the Flux root Kustomization's `decryption` block. The kustomize-controller SA picks up KMS-decrypt credentials from the Pod Identity Agent (Pod Identity association wired in §4.3).

**Do NOT** copy harbor's `components_extra = ["image-reflector-controller", "image-automation-controller"]` — that's nightly-bump infra harbor needs, not prod cells.

**TF apply credential**: Flux bootstrap requires a GitHub PAT with `admin:repo_hook` + `repo` scope to push the deploy key and write the Flux manifests. Same env-var contract as existing prod/dev TF (`GITHUB_TOKEN` from operator's local environment at apply-time, or sourced from 1Password/SSO-bound). Document in PR body for the operator running bootstrap.

**SOPS chicken/egg**: per-cell SOPS keys must exist before any `.sops.yaml` rule can reference their ARN. Sequencing: `kms.tf` applies → KMS ARN known → `.sops.yaml` rules for `clusters/<cell>/` get the ARN → SOPS-encrypted secret files become writable. For v1 there are no SOPS-encrypted secrets in the new cell trees yet, so this is documentation-only; the first secret-bearing PR (post-cell-activation) handles `.sops.yaml`.

### 4.8 Deployment sequencing

```
0. Precursor PR: cell-subsystem-manifests (§2.5) lands on main
1. PR opens (this workstream) — TF for prod-use2 + prod-euw1 + ECR replication + prod outputs.tf edit
2. terraform init/plan in eu-central-1/prod (small) — verify outputs-only diff (2 new outputs, no resource changes)
3. terraform apply in eu-central-1/prod — expose outputs for downstream cells
4. terraform init/plan/apply in us-east-2/common — adds ECR replication (rule writes only; existing repos unchanged)
5. terraform init/plan/apply in us-east-2/prod-use2 — provision cell    ┐
6. terraform init/plan/apply in eu-west-1/prod-euw1 — provision cell    ┘ steps 5 + 6 parallel
7. After apply: Flux bootstrap runs as part of TF — source-controller starts pulling clusters/prod-use2/ and clusters/prod-euw1/
8. Watch Kustomization Ready transitions on each new cluster
9. Confirm Cilium + cert-manager + external-dns + sei-k8s-controller + Karpenter + Gateway come up
```

Step 0 is hard-prerequisite (§6). Step 2-3 must precede steps 5-6 (cell `route53.tf` reads prod's state outputs). Step 4 is independent of step 3 but logically grouped here. Steps 5 + 6 run in parallel after step 3.

DNS propagation: child NS records get TTL 300 from the parent. cert-manager's first DNS01 challenge may take 5-10 minutes on initial bootstrap as upstream resolvers refresh. Acceptable for v1.

### 4.9 Validation strategy

- `terraform plan` against real account at every step — expected diff documented in PR body
- Post-apply: smoke-test the same way PR #883 reconciliation watch was done (flux get kustomization, HelmRelease gen==obsGen, sei-k8s-controller Deployment Ready)
- Expected first-pass failures and recovery:
  - DNS propagation lag — child zone NS records take 30-60s to propagate; cert-manager DNS01 challenges will retry; first cert may take 5-10 minutes
  - ECR replication initial backfill — first euw1 Karpenter scale-up may race the replication backfill window. AMI-baked images (Bottlerocket, EKS-managed) don't race; HelmRelease-pulled images might. Cross-region public ECR fall-through works as a tolerated slow-path until replication catches up. Documented as expected, not a bug.

## 5. Rationale

**Why mirror dev's shape, not prod's**: prod accumulates singletons (mainnet archive, Grafana RDS, Thanos store, heatseeker, state-size-analyzer) that don't belong in fresh cells. dev is the closest existing approximation to "prod-shape minus singletons." Starting from dev produces cleaner, more readable cell TF.

**Why per-AZ NAT instead of single-NAT**: with cross-region peering deferred, the only path out of a cell is its own NAT GW. Single-NAT failure during a validator-active period = full reconciliation freeze + no chain peering. Cost is ~$65/mo extra per cell; trivial against the cost of a validator outage.

**Why per-cell SOPS keys (no replication)**: blast radius. Compromised Flux in cell A reading cell B's secrets is the failure mode multi-region replication enables. SOPS encrypts to the destination key's ARN; per-cell keys means cell A's ciphertext is unreadable in cell B. This is the cleanest trust boundary available and matches the existing prod/harbor/dev pattern.

**Why enable ECR replication**: Karpenter scale-ups are the load-bearing case. Every new node pulls every container fresh. Cross-region pull adds 80-90ms TLS RTT + cold transfer time per layer. Replication cost (~2x storage on small repos) is much smaller than the latency tax on every node provision.

**Why provider-alias for parent zone NS (not factor-out)**: factoring `platform.sei.io` into a separate state is a real refactor touching the existing prod cell. Two new cells don't justify it. Provider alias is one resource per cell, no shared-state coupling beyond a `data.terraform_remote_state` read.

## 6. Honest blockers

**Cell-subsystem-manifests precursor PR (§2.5) must merge first**, or applying this TF leaves Flux source-controller in a reconcile-stall (no Cilium → no pod network → cluster has nodes but workloads can't schedule). This is a hard ordering dependency.

Other knowns:

- TF state bucket is shared and already-bootstrapped.
- GHA OIDC role at `us-east-2/common/gha.tf` enumerates 5 repos explicitly (`sei-protocol/{platform,sei-chain,sei-k8s-controller,sei-cosmos-exporter,qa-testing}`); branch wildcard is `repo:<repo>:*`. New repos pushing to ECR require an explicit add. Two more cells don't change the OIDC scope (cells consume ECR via Pod Identity, not GHA).
- KMS bootstrap key for TF state SSE already exists.

Open question: the `EngWithLimitedPacific-1` SSO role name on existing KMS key policies is pacific-1-themed but functionally gates engineer KMS access. Carrying it forward for parity in v1; un-defer if/when SSO roles get a generic rename pass.

## 7. Don't-do guardrails

- **Do not multi-region replicate SOPS KMS keys.** Per-cell only. Hard one-way door.
- **Do not collapse to single-NAT** on cells hosting live validators. Per-AZ.
- **Do not skip the `arctic-1` chain map trim** — leaving `atlantic-2` and `pacific-1` in the new cell's IRSA grants snapshots-bucket prefix access for chains that aren't in the cell.
- **Do not copy harbor's `components_extra`** Flux components. Prod cells run no image-automation.
- **Do not reuse a CIDR.** Tracking table is the cell parameter inventory; un-allocated bands are `10.20-10.49` (us-east-2 future) and `10.51-10.59 + 10.61-10.69 + 10.71+` (EU future).
- **Do not add sensitive outputs to `eu-central-1/prod/outputs.tf`** (RDS passwords, secret ARNs, anything that shouldn't leak across cells) without first factoring the parent zone into its own state. The cross-state read in §4.6 leaks all of prod's outputs to every cell's TF principal.

## 8. Open follow-ups (out of scope)

| Item | Trigger to un-defer |
|---|---|
| Cross-region VPC peering | Cross-cell P2P bandwidth cost or private Thanos scrape |
| `eu-west-1/common/` directory | Region-local resource (KMS multi-region, SES) needed |
| Parent zone factor-out to common state | 3rd or 4th cell added |
| GHA OIDC subject tightening | First fork-PR incident or quarterly audit |
| TF state KMS key-policy tightening | New TF runner identity (CI bot, second account) added, OR first cross-cell `terraform_remote_state` consumer beyond parent-zone NS |
| VPC interface endpoints | NAT egress shows up as a budget line, or audit requires private-only pulls |
| arctic-1 SeiNodeDeployments | This workstream lands |
| Category D extraction (kube-system/istio/flux/monitoring base sharing) | Cells are stable + a third cell is being planned |
| Cell-side monitoring stack | Category D lands |
| eu-central-1 meta-cluster Cilium migration | filed separately (Tide issue) |

## 9. References

- sei-protocol/platform#883 — clusters/base/ refactor + dormant cell manifests (merged at b5544ad)
- sei-protocol/Tide#124 — clusters/base/ refactor design doc (sibling)
- `terraform/aws/189176372795/eu-central-1/prod/` — existing prod cell, reference for prod-shape patterns
- `terraform/aws/189176372795/us-east-2/dev/` — existing dev cell, primary template (slim prod-shape)
- `terraform/aws/189176372795/us-east-2/common/{ecr,gha}.tf` — region-scoped shared resources
- Phase 0 discovery: platform-engineer + security-specialist + network-specialist outputs (this workstream session)
