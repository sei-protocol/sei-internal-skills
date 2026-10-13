# Platform architecture

This file walks the platform from the top down: the cells, the repository, the change flow, Terraform, traffic, secrets, observability and the workloads. It extends the mental model in `SKILL.md`. Facts that drift have a check in [maintenance](maintenance.md#1-verify-sweep).

Contents:

1. [Cells](#1-cells)
2. [Repository map](#2-repository-map)
3. [Flux and the change flow](#3-flux-and-the-change-flow)
4. [Prune, force and reconcile](#4-prune-force-and-reconcile)
5. [Terraform and identity](#5-terraform-and-identity)
6. [Networking and traffic](#6-networking-and-traffic)
7. [Secrets](#7-secrets)
8. [Observability stack](#8-observability-stack)
9. [Workloads](#9-workloads)

## 1. Cells

All six cells live in one AWS account, 189176372795. Cell names follow `<env>-<region>`. The names `prod` and `dev` are older, and `harbor` is a deliberate exception.

The `prod` cell in eu-central-1 is the hub. It holds the only fleet Grafana, the Thanos query layer, the Thanos Ruler, the fleet log store and the `platform.sei.io` DNS zone. Three spoke cells run validators and nodes in us-east-2, eu-west-1 and ap-northeast-1. Harbor is the engineering cell. Dev runs CI and the agent infrastructure.

Two CNIs run in the fleet:

- **Cilium** runs on harbor and the three spokes. Each cell has a unique Cilium cluster ID. Cilium uses VXLAN and replaces kube-proxy. Pods get addresses from 100.64.0.0/14, which the VPC cannot route, so the API server cannot reach a pod. For that reason, each component that the API server calls runs on the host network. These are the cert-manager webhook, the AWS Load Balancer Controller, metrics-server and KEDA.
- **VPC CNI** runs on prod and dev, with network policy enforcement off. A NetworkPolicy does nothing there. A policy that you test on prod behaves differently on a Cilium cell.

Two Kustomize Components apply across cells:

- `clusters/base/cni-cilium` installs Cilium and moves the webhooks onto the host network. Harbor and the three Cilium spokes include it.
- `clusters/base/namespace-prune-protection` stops Flux from deleting a platform Namespace. Every Flux path that renders a Namespace includes it, and CI fails when one does not.

Dev uses none of the `clusters/base` bases. Its trees are older standalone copies, and it includes only the Namespace guard.

## 2. Repository map

| Path | What it holds |
|---|---|
| `clusters/<cell>/` | The Flux root of a cell. Its `kustomization.yaml` lists every directory that the cell runs. |
| `clusters/<cell>/flux-system/` | Flux's own objects. Harbor and prod-apne1 apply them from git. The other prod cells apply only `loadgen.yaml` from it. |
| `clusters/<cell>/.sops.yaml` | The cell's SOPS rule and key. |
| `clusters/base/` | Bases that harbor and the prod cells list, and the two Components. |
| `manifests/base/` | Fleet monitoring rules, the observability stack and giga-testnet-0. |
| `terraform/aws/189176372795/` | The Terraform roots: `bootstrap`, `common` and one root per cell under `<region>/<cell>/`. |
| `.agent/runbooks/` | `cell-bootstrap.md` and `networking-orphan-adopt.md`. Read the first lines of the others: some carry a Superseded mark. |
| `docs/rollouts/` | One record for each release roll of a chain. |
| `scripts/` | The monitoring validators, the Explore URL generator, the sei-load pin scripts. |
| `.github/workflows/` | The manifest check, the monitoring validation, image builds, the nightly image bump. |

## 3. Flux and the change flow

Every cell syncs `main`. Three shapes exist:

- **Bootstrap.** Terraform bootstrapped dev, prod, prod-use2 and prod-euw1 with `flux_bootstrap_git`. The root Kustomization, `flux-system`, applies `clusters/<cell>/` but not the bootstrap files in `clusters/<cell>/flux-system/`. An edit to those files in git changes nothing on these cells, and the live intervals differ from the files. Read the live objects.
- **Bootstrap with image automation.** Harbor uses the same bootstrap. Its root also applies `flux-system/`, which adds the image automation for the nightly gate.
- **Flux Operator.** prod-apne1 runs a `FluxInstance` and staged Kustomizations: `nodepools`, `configs`, `monitoring-configs`, `apps` and `loadgen`. Each stage waits for the one before it. This is the shape for a new cell, because `flux_bootstrap_git` pushes to `main` and the branch rules reject that push.

A change moves in four steps:

1. **Pull request.** A pull request to `main` needs one approval and merges by squash. The required check is `shellcheck`. The K8S Manifest Check builds every Flux path, checks that no SeiNode carries Flux force, and checks that every platform Namespace carries the prune guard. It does not gate the merge, so read its result yourself.
2. **Sync.** Flux fetches `main` every minute (every 3 minutes on prod-apne1) and applies every 3 minutes. A merge reaches every cell within about 6 minutes. To apply at once, for example during a chain halt, run `flux --context <cell> reconcile kustomization flux-system -n flux-system --with-source`. That is a write.
3. **Staging.** No environment branches exist. Per-cell files and pins stage a change, such as the controller ref in `clusters/<cell>/sei-k8s-controller/`. Harbor takes a new controller first, and the prod cells follow one at a time.
4. **Terraform.** An engineer applies Terraform by hand after the merge ([section 5](#5-terraform-and-identity)).

## 4. Prune, force and reconcile

Every Flux Kustomization sets `prune: true`, so Flux deletes whatever leaves a build. Three annotations change that behavior.

- `kustomize.toolkit.fluxcd.io/prune: disabled` keeps an object after it leaves git. The sei.io CRDs, most SeiNodes, the P2P Services and every platform Namespace carry it. A removal from git deletes nothing, so a retirement ends with a manual delete ([procedure H](procedures.md#h-retire-a-seinode)). Flux deletes a SeiNode without it as soon as its removal reaches the cell, and the finalizer then deletes its data volume.
- `kustomize.toolkit.fluxcd.io/force: enabled` lets Flux delete and recreate an object when an immutable field changes. Services and HTTPRoutes carry it. A SeiNode never carries it, because a recreate runs the finalizer, which deletes the data volume. CI rejects it.
- `kustomize.toolkit.fluxcd.io/reconcile: disabled` stops Flux from changing an object. The giga load pin uses it, together with KEDA's `paused-replicas` (`/giga-dev`).

To list the SeiNodes in a cell without the prune guard:

```sh
kubectl --context <cell> get seinodes -A -o json \
  | jq -r '.items[] | select(.metadata.annotations["kustomize.toolkit.fluxcd.io/prune"] != "disabled") | "\(.metadata.namespace)/\(.metadata.name)"'
```

A suspended Kustomization raises no alert, because `FluxObjectNotReady` skips suspended objects. On a bootstrap cell the root is almost the only Kustomization, so a suspend freezes the whole cell. On resume, Flux applies and prunes everything that merged in the meantime, in one pass.

## 5. Terraform and identity

Terraform describes the AWS side in separate root modules under `terraform/aws/189176372795/`:

- `bootstrap` holds the state bucket and its KMS key. Its own state is local and in git.
- `common` holds the GitHub Actions OIDC provider and role, the `sei/*` ECR repositories and their replication.
- Each cell has its own root.

State lives in the S3 bucket `sei-platform-terraform-state` in us-east-2, versioned and encrypted. No root sets a state lock, and no CI job runs Terraform. An engineer applies from a workstation after the merge and records the plan in the pull request ([procedure C](procedures.md#c-run-terraform-safely)).

Each cell root is a copy of an earlier cell, with no shared module. A cell root creates these resources:

- a VPC across three availability zones, with a NAT gateway in each;
- EKS, with a small managed node group for Karpenter and the system add-ons. The Karpenter NodePools live in git under `clusters/base/default/`.
- the cell's SOPS key, and S3 buckets for genesis artifacts, snapshots, Thanos blocks and Loki;
- a public zone, `<cell>.platform.sei.io`, and a private zone, `<cell>.internal.platform.sei.io`;
- VPC peering to the hub;
- one EKS Pod Identity role per workload, named `<cell>-<purpose>`.

Dev is older and smaller. It has one NAT gateway, and no private zone, peering or Thanos bucket.

The hub and the spokes read each other's state. The hub reads each spoke's state for peering and Thanos, and each spoke reads the hub's state. The spokes also write three objects that the hub owns: the peering accepter, the private zone association and the NS records in `platform.sei.io`. When a change alters an output that another root reads, apply the root that produces it first. To add a cell, apply the hub first. To remove a cell, edit the hub first.

Identity follows three patterns:

- **Workloads** use EKS Pod Identity. A few hub workloads with older AWS SDKs use IRSA.
- **GitHub Actions** use one OIDC role in `common` to push and pull `sei/*` images. No role lets CI run Terraform.
- **Engineers** use SSO permission sets, which live outside Terraform. Human EKS access entries are also outside Terraform.

Some KMS keys encrypt data that nobody can create again, so they carry `prevent_destroy`. These are the state key, each SOPS key, the Grafana RDS key, the fleet Loki key and the agent credentials key. Terraform refuses a plan that destroys or replaces them. The guard lives in the resource block, so a deleted block still destroys the key.

## 6. Networking and traffic

Public RPC traffic reaches a node in four steps:

1. **DNS.** external-dns publishes `<name>.<chain>.platform.sei.io` and `<name>.<cell>.platform.sei.io` from each HTTPRoute.
2. **Load balancer.** One internet-facing NLB per cell forwards ports 80 and 443 to the Istio gateway `gateway/sei-gateway`, on the Gateway API. The gateway runs two Envoy replicas on separate nodes with a PodDisruptionBudget. The ConfigMap `sei-gateway-parameters` sets that shape.
3. **TLS.** The gateway terminates TLS with a wildcard certificate per cell or per chain. cert-manager issues it through the `letsencrypt` ClusterIssuer with Route53 DNS-01.
4. **Route.** An HTTPRoute sends the request to the node's `<node>-external` ClusterIP Service. Each node has four routes: `-rpc` (26657), `-evm` (8545, with WebSocket upgrades to 8546), `-grpc` (9090) and `-rest` (1317).

```
client -> node-0-rpc.pacific-1.platform.sei.io     (external-dns, zone platform.sei.io)
       -> NLB :443 -> gateway/sei-gateway-istio     (2 Envoy replicas, TLS terminated)
       -> HTTPRoute pacific-1/node-0-rpc -> Service node-0-external :26657
       -> seid pod :26657

P2P:   peer -> <node>-p2p NLB :26656 (one per node, vanity hostname) -> seid :26656
```

P2P traffic does not use the gateway. Each node has its own `<node>-p2p` LoadBalancer Service on 26656, with one NLB and a vanity hostname from external-dns. Seeds publish `NodeID@host`, and validators keep peer hostnames in a ConfigMap in git. A recreated Service gets a new NLB hostname, so the Service is a one-way door. The NLB target type differs by cell: prod uses `ip`, and the Cilium cells use `instance`.

Three properties of the edge need care:

- **Listener pinning.** A route with no `sectionName` attaches to every listener, port 80 included, and then answers in plain HTTP. A new route pins `sectionName` ([procedure F](procedures.md#f-expose-a-service)).
- **Upsert-only DNS.** external-dns runs with `upsert-only`, so it never deletes a record. A retired route leaves its records until someone deletes them. A name outside the external-dns IAM allowlist fails with `AccessDenied`. The external-dns log names the record, and `ExternalDNSRegistryErrors` fires on the error count.
- **Harbor tenants.** Each engineer cell has its own Gateway and NLB on `*.eng-<alias>.harbor.platform.sei.io`. Admission policies keep tenant hostnames and chaos experiments inside the tenant's namespace. The shared gateway does not accept routes from tenant namespaces.

## 7. Secrets

Secrets live in git, encrypted with SOPS under the cell's KMS key, `alias/<cell>`. Each `clusters/<cell>/.sops.yaml` has one rule, which encrypts `data` and `stringData`. Flux's kustomize-controller decrypts at apply time through Pod Identity. Validator signing keys and node keys are among these files. [Procedure D](procedures.md#d-edit-a-sops-secret) is the edit procedure, and an engineer runs it, not this skill.

External Secrets Operator runs only on dev, where it mints the GitHub App token for the code review agents.

The loss of a cell's key makes that cell's secrets unreadable, validator keys included. The key's `prevent_destroy` guards against that.

## 8. Observability stack

**Metrics.**

- Each federated cell runs Prometheus with a Thanos sidecar and 30 days of local data. The sidecar uploads 2-hour blocks to the cell's Thanos bucket. Each spoke also runs its own compactor and store gateway.
- The hub runs Thanos Query behind a query frontend. It reads each spoke's sidecar and store gateway through internal NLBs and VPC peering.
- Grafana on the hub reads the query frontend, at `https://grafana.prod.platform.sei.io`.
- Thanos keeps 90 days of raw data on the hub.

**Logs.** An Alloy DaemonSet in each cell writes to the cell's own Loki and also to `loki-fleet` on the hub. Both keep 90 days.

**Dev** stands alone, with its own Prometheus, Alertmanager, Grafana and Loki. Nothing from dev reaches the hub.

**External labels.** Each cell sets `externalLabels` `{cluster, region}`. Harbor and the spokes set them in `clusters/<cell>/monitoring/po-cell-vars.yaml`, prod in `prometheus-operator.yaml`, and dev in `kube-prometheus-stack.yaml`. Never change them after the first apply, because the Thanos compactor halts on a bucket with mixed label sets.

[The observability reference](observability.md) holds the queries, the alert path and the dashboards.

## 9. Workloads

The controller and its SeiNodes are the main workload. `/kubernetes` covers the controller.

| Chain | Where it runs |
|---|---|
| arctic-1 | Validators across prod, prod-use2 and prod-euw1. Also RPC, archive, syncer, frozen and reserve nodes, seeds, the public RPC pool and the governance tasks. Harbor runs staging syncers. |
| pacific-1 | No Sei-run validators. prod runs archives, RPC nodes and a syncer. Seeds run in prod, prod-use2 and prod-euw1. Harbor runs staging syncers. |
| atlantic-2 | No Sei-run validators. prod runs an archive, RPC nodes, a syncer and a seed. Other seeds run in prod-use2 and prod-euw1. Harbor runs a staging syncer. |
| giga-testnet-0 | Validators and full nodes in each of the four prod cells, plus load generation (`/giga-dev`). |

To count the nodes of a chain, ask the cells:

```sh
for c in prod prod-use2 prod-euw1 prod-apne1 harbor; do
  kubectl --context "$c" get seinodes -A --no-headers | awk -v c="$c" '{print c, $1}'
done | sort | uniq -c
```

Beside the chains, the platform runs these workloads:

- **Load generation.** Each prod cell with giga-testnet-0 runs a `seiload-saturation` Deployment. A KEDA ScaledObject sets its replica count from the cell's Prometheus. A separate Flux Kustomization, `loadgen`, applies it, so a KEDA fault cannot block the validators. `/giga-dev` owns its operation.
- **heatseeker.** A synthetic prober in `prod/heatseeker`, one Deployment per chain. It checks faucet balances, RPC health, snapshot age and exchange deposits. Its alerts page the protocol service.
- **The nightly gate.** On harbor, the `nightly-harness-suite` CronJob in `nightly` runs the controller's nightly integration suites at 08:00 UTC. Flux image automation bumps the seid nightly images, and a workflow merges each bump after a guard checks it.
- **Harbor engineers.** Each engineer has a cell, `eng-<alias>`, which Flux syncs from `sei-protocol/harbor-engineering-workspace`. Tenant Namespaces stay prunable, so an offboarding deletes the tenant cell.
- **Dev.** Dev runs the GitHub Actions runner scale set for sei-chain and other repositories, seigent, the agent sandboxes, Kyverno and External Secrets Operator.
- **Prod CronJobs.** `archive-volume-snapshot` pauses one archive node per chain once a week, takes an EBS snapshot of its volume, and resumes it. `state-size-analyzer` runs daily. The `snapshot-upload` CronJobs stay suspended in git.
