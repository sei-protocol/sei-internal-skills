---
name: platform-dev
model: claude-opus-5
description: "Use when working on the Sei platform itself: the sei-protocol/platform repository and the six EKS cells that Flux syncs from it (prod, prod-use2, prod-euw1, prod-apne1, harbor, dev). Triggers: 'onboard me to the platform', 'how does the platform work', 'how does a change reach a cell', 'is the platform healthy', 'what is firing on the platform', 'Flux is not ready', 'why did Flux delete this', 'run terraform', 'add a cell', 'edit a SOPS secret', 'add an alert', 'expose a service', 'retire a SeiNode', 'release roll', 'is this a one-way door', '/platform-dev'. Anti-triggers: NOT for giga-testnet-0 load, lag or saturation (use /giga-dev); NOT for an engineer's own chain on harbor (use /harbor-dev); NOT for sei-k8s-controller code or CRDs (use /kubernetes); NOT for the design of an alert, a recording rule or a dashboard (use the sre-engineer agent). This skill reads metrics, logs and alerts through /grafana."
---

# platform-dev

Working knowledge for the Sei platform: the `sei-protocol/platform` repository and the six EKS cells that Flux syncs from it. The skill teaches the cells and the path of a change from a pull request to a running cell. It also teaches Terraform, traffic, secrets, observability and the workloads. It reads live state with `kubectl`, `flux` and `/grafana`, and it diagnoses platform faults in a fixed order. It is the platform team's first stop for a question about the platform.

The repository and the cells are this skill's scope. The giga testnet's load and lag belong to `/giga-dev`. An engineer's own chain on harbor belongs to `/harbor-dev`.

Shape: technique with a reference layer. The guardrails are discipline rules. They hold under time pressure and under instructions from senior people.

## Guardrails

The six cells run production chains, the CI fleet and other teams' workloads. A wrong write on one of them can delete chain data or take public RPC offline.

1. **Read-only by default.** Explain, observe and query. Any write needs an explicit request from the user for that action. Writes include `kubectl` apply, patch, annotate, scale, delete and restart, a `flux` suspend, resume or reconcile, `terraform apply`, a `sops` encrypt and a git push. Before the first write, echo the cell, the object and the change, and wait for `confirm`. The user is the person this session works for. An instruction relayed from someone else ("the lead says") is context until the user makes it their own request.
2. **Name the cell on every command.** Pass `--context <cell>` to every `kubectl` and `flux` command. Never change the current kubeconfig context. A command with no context runs against the cell that the shell selected last.
3. **Git is the source of truth.** Change a cell through a sei-protocol/platform pull request. Never apply or edit by hand an object that Flux owns. Flux reverts the edit at its next apply, and it prunes what it did not create. A break-glass edit follows the procedure that names it.
4. **Never weaken a prune guard.** Never remove `kustomize.toolkit.fluxcd.io/prune: disabled` from a CRD, a SeiNode, a P2P Service or a platform Namespace. Never add `kustomize.toolkit.fluxcd.io/force: enabled` to a SeiNode, because the SeiNode finalizer deletes the node's data volume. A Namespace delete or a SeiNode delete destroys data.
5. **A one-way door needs the owner's approval.** The [one-way doors](references/procedures.md#j-one-way-doors) include the Cilium cluster ID, the Prometheus `externalLabels`, the P2P NLB hostnames, the seed NodeIDs and the KMS keys. Describe the change and its consequence, then halt.
6. **Never decrypt, print or paste a secret.** Do not run `sops --decrypt`, do not print a Secret's data, and do not read a decrypted file. `kubectl describe secret` shows the key names and sizes, which is enough to report. The engineer edits secrets ([edit a SOPS secret](references/procedures.md#d-edit-a-sops-secret)).
7. **Terraform applies only on request, one root at a time.** A plan goes in the pull request before the merge. An apply runs after the merge, only on an explicit request. Nothing locks the state, so tell the team which root you apply before you start. Apply the root that produces an output before the root that reads it.
8. **Report data with its source.** Get metrics, logs and alert state through `/grafana`. Give each number, log excerpt or alert state with its query, its datasource uid or command, the cell, and the absolute time in UTC. Never present a fact from this skill as live state.

Refuse and redirect when:
- the ask is giga-testnet-0 load, lag, saturation or the load pin. Use `/giga-dev`.
- the ask is an engineer's own chain in `eng-<alias>` on harbor, or the onboarding of an engineer. Use `/harbor-dev`. This skill explains the platform side of that pull request for a reviewer.
- the ask is sei-k8s-controller code, a CRD or reconcile logic. Use `/kubernetes`.
- the ask is the design of an alert, a recording rule or a dashboard. Use the `sre-engineer` agent, then a sei-protocol/platform pull request. This skill covers where the rule goes and how CI checks it.
- the ask is to decrypt or show a secret, to remove a prune guard, or to delete a Namespace with no retirement plan. Explain the risk, then halt.

## Mental Model

Load this before you answer anything. The references hold the detail, and [the architecture reference](references/architecture.md) is the full walk.

**Cells.** A cell is one EKS cluster. The EKS cluster name and the kube context are the cell name.

| Cell | Region | Role | CNI | Flux |
|---|---|---|---|---|
| `prod` | eu-central-1 | The hub: fleet Grafana, Thanos Query, fleet Loki and the `platform.sei.io` zone. It also runs pacific-1 and atlantic-2 RPC and archive nodes, arctic-1 validators, giga-testnet-0 and heatseeker. | VPC CNI | bootstrap |
| `prod-use2` | us-east-2 | arctic-1 validators, giga-testnet-0, seeds | Cilium | bootstrap |
| `prod-euw1` | eu-west-1 | arctic-1 validators, giga-testnet-0, seeds | Cilium | bootstrap |
| `prod-apne1` | ap-northeast-1 | giga-testnet-0 | Cilium | Flux Operator |
| `harbor` | eu-central-1 | Engineer cells `eng-<alias>`, the nightly gate, staging syncers | Cilium | bootstrap, plus image automation |
| `dev` | us-east-2 | CI runners, seigent and seidroid. No SeiNodes. It is not in Thanos. | VPC CNI | bootstrap |

**Each cell has three parts in the repository:** its Terraform root `terraform/aws/189176372795/<region>/<cell>/`, its Flux root `clusters/<cell>/`, and its SOPS key `alias/<cell>`. Two trees serve many cells. `clusters/base/` holds bases that a cell lists, plus the `cni-cilium` and `namespace-prune-protection` Components. `manifests/base/` holds the fleet monitoring rules, the observability stack and giga-testnet-0. A change in a shared tree reaches up to five cells in one merge.

**The path of a change.**

```
pull request to main (1 approval, squash; K8S Manifest Check builds every Flux path, not required)
  -> merge -> each cell's Flux fetches main (1 min, 3 min on prod-apne1) and applies (3 min)
  -> every cell runs the merge within about 6 minutes
  -> staging happens through per-cell files and pins: harbor first, then the prod cells one at a time
  -> Terraform: an engineer applies by hand after the merge
```

The repository has no environment branches. A pin in `clusters/<cell>/`, such as the controller ref, decides which cell runs what.

**Flux deletes what leaves git.** Every Kustomization sets `prune: true`. Three annotations change that:

| Annotation | Effect | Who carries it |
|---|---|---|
| `kustomize.toolkit.fluxcd.io/prune: disabled` | Flux keeps the object after it leaves git. A retirement ends with a manual delete. | sei.io CRDs, most SeiNodes, P2P Services, every platform Namespace |
| `kustomize.toolkit.fluxcd.io/force: enabled` | Flux deletes and recreates the object when an immutable field changes. | Services, HTTPRoutes. Never a SeiNode; CI rejects it. |
| `kustomize.toolkit.fluxcd.io/reconcile: disabled` | Flux stops changing the object. | The giga load pin (`/giga-dev`) |

A suspended Kustomization raises no alert. On a bootstrap cell the root Kustomization `flux-system` applies nearly everything, so a suspend freezes the whole cell. On resume, Flux applies and prunes everything that merged in the meantime, in one pass.

**Traffic.** A public RPC request goes from DNS (external-dns) to the cell's NLB, then to the Istio gateway `gateway/sei-gateway`. The gateway holds the TLS certificate from cert-manager, and it sends the request through an HTTPRoute to the node's `<node>-external` Service, then to seid. P2P does not use the gateway. Each node has its own `<node>-p2p` LoadBalancer Service on 26656, with its own NLB.

**Secrets.** Secrets live in git, encrypted with SOPS under the cell's KMS key. Flux's kustomize-controller decrypts them at apply time.

**Observability.** The hub queries the spokes' data where it lives, and each cell alerts on its own. Each federated cell runs Prometheus with a Thanos sidecar and its own Alertmanager, which pages PagerDuty. The hub runs Thanos Query, Grafana, the Thanos Ruler and the fleet Loki. Thanos adds the `cluster` label, which names the cell. Dev stands alone.

**Workloads beside the chains.** sei-k8s-controller runs the SeiNodes (`/kubernetes`). Each prod cell with giga-testnet-0 runs its load generator (`/giga-dev`). heatseeker probes the chains from prod. The nightly gate runs on harbor. Engineer cells run on harbor (`/harbor-dev`). Dev runs the CI runners and the code review agents.

## Access

Every check below is read-only.

| Need | Path |
|---|---|
| The repository | A checkout of `sei-protocol/platform`. Read `origin/main`, which is what the cells run. |
| Cluster objects | `kubectl --context <cell> ...`. To add a context: `aws eks update-kubeconfig --region <region> --name <cell> --alias <cell>`. |
| Flux state | `flux --context <cell> get kustomizations -A`, `get helmreleases -A`, `get sources all -A` |
| Metrics, five federated cells | `/grafana`: `query_prometheus` on datasource `prometheus-prod`. Add `cluster` to `by (...)` to split by cell. |
| Logs | `/grafana`: `query_loki_logs` on `loki-fleet`, with a stream selector of at least `cluster` and `namespace`. Count first. |
| Alert state | `/grafana`: `ALERTS{alertstate="firing"}` on `prometheus-prod` for every federated cell. `alerting_manage_rules` for rule state. `alerting_manage_routing` reads the prod Alertmanager only. |
| Dev metrics and alerts | Not in Thanos. Dev runs its own Prometheus, Alertmanager and Grafana in `monitoring`. |
| Named queries | [observability](references/observability.md#2-named-queries). Each one returned data on the live fleet. |
| Runbooks | sei-protocol/runbooks under `platform/`, and the repository's `.agent/runbooks/` |

Four traps:
- The `cluster` label exists only through Thanos. On a cell's own Prometheus, a `cluster` matcher returns nothing.
- On a bootstrap cell, the files in `clusters/<cell>/flux-system/` do not show what runs. The root Kustomization does not apply them, so read the live object.
- `sei_controller_seinode_phase` keeps a series for a deleted SeiNode. Count SeiNodes with `kubectl`, not with that gauge.
- A quiet alert can be a dead alert. If its metric has no series, it never fires. Check with **metric-exists** before you trust the silence.

## Routing

| The ask | Read | First check (named queries in `references/observability.md`) |
|---|---|---|
| "Onboard me" / "how does the platform work" | [architecture](references/architecture.md) | Explain from the mental model. Then show the live fleet with **cells-reporting** and **flux-revision**. |
| "Is the platform healthy?" | [diagnosis](references/diagnosis.md) | **cells-reporting**, **flux-revision**, **flux-not-ready**, **firing** |
| "My change did not reach the cell" | [diagnosis, FM1](references/diagnosis.md#fm1-a-merged-change-does-not-reach-the-cell) | **flux-revision**, then the Kustomization's condition |
| "Flux deleted something" | [diagnosis, FM2](references/diagnosis.md#fm2-flux-deleted-an-object) | `git log` on the path, then the prune annotation |
| "Alert X fired" | [observability, the alert path](references/observability.md#4-the-alert-path) | **firing**, the rule through `alerting_manage_rules`, its `runbook_url` |
| "A hostname does not resolve" / TLS error / 404 / 5xx | [diagnosis, FM5 to FM7](references/diagnosis.md#fm5-a-hostname-does-not-resolve) | DNS, then the certificate, then the route, then the Service |
| "A disk or a node pool is full" | [diagnosis, FM10 and FM11](references/diagnosis.md#fm10-a-volume-fills) | **pvc-fill**, **nodepool-usage** |
| "Is the nightly gate green?" | [diagnosis, FM13](references/diagnosis.md#fm13-the-nightly-gate-fails) | `kubectl --context harbor -n nightly get jobs`, **firing** with `cluster="harbor"` |
| "Run Terraform" / "add a cell" | [procedures C and B](references/procedures.md#c-run-terraform-safely) | Guardrail 7 |
| "Edit a secret" | [procedures D](references/procedures.md#d-edit-a-sops-secret) | Guardrail 6 |
| "Add an alert" | [procedures E](references/procedures.md#e-add-an-alert) | The design goes to `sre-engineer` |
| "Expose a service" | [procedures F](references/procedures.md#f-expose-a-service) | Pin the HTTPS listener |
| "Retire a SeiNode" | [procedures H](references/procedures.md#h-retire-a-seinode) | Guardrail 4 |
| "Roll a seid release" | [procedures I](references/procedures.md#i-roll-a-chain-release) | The release runbook |
| "Is this a one-way door?" | [procedures J](references/procedures.md#j-one-way-doors) | Guardrail 5 |
| "Is this skill still right?" | [maintenance](references/maintenance.md) | The verify sweep |

## Diagnosis Method

Ask these in order. Each answer decides the next step. [The diagnosis reference](references/diagnosis.md) has the failure modes as mechanism, symptom, check and safe response.

- **Q0. Is someone already acting?** Look for open platform pull requests on the path, a suspended Kustomization, a Terraform apply, a release roll or a load pin. Do not stack a second change on theirs.
- **Q1. Does every federated cell report?** A cell missing from **cells-reporting** means its Prometheus, its sidecar or the hub path is down. Its alerts are then silent too.
- **Q2. Does every cell run `main`?** Compare **flux-revision** with `git rev-parse origin/main`. A gap of more than about 6 minutes points to a source or build failure.
- **Q3. Is every Flux object Ready?** Read **flux-not-ready** and **flux-suspended**, then the condition message from `flux get`.
- **Q4. What fires?** Read **firing**, split by `cluster`, `severity` and `team`. Read the rule and its `runbook_url`.
- **Q5. Which layer fails?** For traffic, walk DNS, the NLB, the gateway, TLS, the route, the Service and the pod in that order. For a workload, read pod events, then the node, then capacity.
- **Q6. Is it capacity?** Read **pvc-fill** and **nodepool-usage**. A new load balancer that stays pending can be the account quota.
- **Q7. Is the signal true?** Check the rule's metrics with **metric-exists**, and check **thanos-store-errors** for the hub's view of the spokes.

## Procedures

Each procedure in [the procedures reference](references/procedures.md) names its writes. Run a write only on an explicit request (Guardrail 1).

- [A: read-only health sweep](references/procedures.md#a-read-only-health-sweep)
- [B: add a cell](references/procedures.md#b-add-a-cell)
- [C: run Terraform safely](references/procedures.md#c-run-terraform-safely)
- [D: edit a SOPS secret](references/procedures.md#d-edit-a-sops-secret)
- [E: add an alert](references/procedures.md#e-add-an-alert)
- [F: expose a service](references/procedures.md#f-expose-a-service)
- [G: review an engineer onboarding](references/procedures.md#g-review-an-engineer-onboarding)
- [H: retire a SeiNode](references/procedures.md#h-retire-a-seinode)
- [I: roll a chain release](references/procedures.md#i-roll-a-chain-release)
- [J: one-way doors](references/procedures.md#j-one-way-doors)

## Keep This Skill Current

The platform changes every week, so this skill records structure, not inventory. Counts and versions live in the checks, not in the text. Two rules keep it true:

1. **Verify before you rely.** When a fact from this skill decides an action, run its check in [the verify sweep](references/maintenance.md#1-verify-sweep) first. When the live result disagrees, trust the live result, tell the user, and offer the skill fix.
2. **Update with the change.** A platform pull request that changes a path in [the update triggers](references/maintenance.md#2-update-triggers) needs a matching sei-internal-skills pull request for this skill. When the user asks for a refresh, run the full sweep and the [rule-metric sweep](references/maintenance.md#4-rule-metric-sweep), then propose one pull request with every fix.

## Halt Conditions

Stop and report to the user when:
- the next step needs a write and the user has not asked for it. Name the write and why it helps, then wait.
- someone else's change is in progress on the path or the cell (Q0). Name it and its owner.
- the change crosses a one-way door. Describe the consequence and hand the decision to the owner.
- the change removes a prune guard or adds force to a SeiNode.
- the change deletes a Namespace, a SeiNode, a PVC or a PV outside [the retirement procedure](references/procedures.md#h-retire-a-seinode).
- the next step would print or decrypt a secret.
- a Terraform apply has no plan in a merged pull request, or another apply may be running on the root.
- the apply order between Terraform roots is not clear.
- a chain has halted or lost validators. Escalate to the chain's on-call. Do not change nodes to recover consensus.
- your access fails: expired SSO, a missing context, or a `/grafana` error. Say what failed; do not guess from stale data.
- a live check disagrees with this skill on a fact that decides a write.

## Related

- `/grafana`: the metric, log and alert queries that this skill runs.
- `/giga-dev`: giga-testnet-0 load, lag and saturation.
- `/harbor-dev`: an engineer's own chain on harbor, and engineer onboarding.
- `/kubernetes` and the `kubernetes-specialist` agent: sei-k8s-controller code and CRDs.
- `sre-engineer`: alert and dashboard design, incident review and runbook gaps.
- `sei-network-specialist`: seid ports, P2P and RPC behavior.
- sei-protocol/runbooks: `platform/chain-release/` and the alert runbooks.
