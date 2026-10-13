# Platform procedures

This file holds the common platform procedures. Each one names its writes. Run a write only on an explicit request from the user, and echo the cell, the object and the change before it (Guardrail 1). A pull request is the normal path for every change. The bold query names resolve in [observability](observability.md#2-named-queries).

Contents:

- [A: read-only health sweep](#a-read-only-health-sweep)
- [B: add a cell](#b-add-a-cell)
- [C: run Terraform safely](#c-run-terraform-safely)
- [D: edit a SOPS secret](#d-edit-a-sops-secret)
- [E: add an alert](#e-add-an-alert)
- [F: expose a service](#f-expose-a-service)
- [G: review an engineer onboarding](#g-review-an-engineer-onboarding)
- [H: retire a SeiNode](#h-retire-a-seinode)
- [I: roll a chain release](#i-roll-a-chain-release)
- [J: one-way doors](#j-one-way-doors)

## A: Read-only health sweep

No writes. Run the steps in this order and report each result with its source.

1. **Federation.** Run **cells-reporting**. Five cells must answer.
2. **Sync.** Run **flux-revision**, and compare each sha with `git rev-parse origin/main`.
3. **Flux.** Run **flux-not-ready** and **flux-suspended**. A healthy fleet returns nothing for both.
4. **Alerts.** Run **firing** and **pending**.
5. **Edge.** Run **gateway-replicas** and **cert-days-left**. On each cell, run `kubectl --context <cell> get certificates -A` and look for `READY` other than `True`.
6. **Capacity.** Run **pvc-fill**, **nodepool-usage**, **node-not-ready** and **crashloop**.
7. **Dev.** Dev is not federated. Run `kubectl --context dev get kustomizations,helmreleases -A` and `kubectl --context dev get pods -A --field-selector=status.phase!=Running,status.phase!=Succeeded`.

## B: Add a cell

Writes: Terraform applies in two roots, and a pull request. `.agent/runbooks/cell-bootstrap.md` in the repository is the procedure, in phases. Five points cause most failures:

1. **Flux shape.** Copy the Terraform from the newest Cilium cell, but take `flux.tf` from prod-apne1, which uses Flux Operator. Merge `clusters/<cell>/flux-system/flux-instance.yaml` before the apply, because the sync reads it from `main`.
2. **Cilium cluster ID.** Take the next free ID. A later change needs a full Cilium restart of the cell.
3. **Apply order.** Apply the hub first, then the cell, then the hub side of the peering.
4. **VXLAN.** Open UDP 8472 between nodes. Without it, pod traffic between nodes drops with no error.
5. **Monitoring.** Encrypt the PagerDuty key file under the new cell's key ([procedure D](#d-edit-a-sops-secret)). Then add the cell to the hub's Thanos stores, and watch **thanos-store-errors** and **cells-reporting**.

## C: Run Terraform safely

Writes: `terraform apply`, only on an explicit request. A plan reads state and AWS, and it needs the engineer's AWS credentials.

1. Sign in with the engineer's AWS SSO profile. A harbor apply also needs the AWS CLI, and a root that manages Flux needs GitHub credentials.
2. Tell the team which root you are about to apply. Nothing stops two applies on one root, and two at once can corrupt its state. Only S3 versioning can restore it.
3. Run `terraform plan -out=tfplan` in the root's directory, and put the plan in the pull request. An engineer onboarding is the one exception ([G](#g-review-an-engineer-onboarding)).
4. After the merge, run `terraform apply tfplan` from the same directory.
5. If the change alters an output that another root reads, apply that root first. The hub reads every spoke, and every spoke reads the hub.
6. Do not run `terraform init -upgrade` as a side effect. The lock files pin providers but not modules.
7. Do not let Terraform recreate `flux_bootstrap_git`. The apply fails, because the resource pushes to `main`.
8. Read every `destroy` and `replace` in the plan. A replace of a KMS key, a state bucket, a VPC or a peering connection is a one-way door ([J](#j-one-way-doors)).

## D: Edit a SOPS secret

The engineer runs this procedure, not the agent (Guardrail 6). The agent explains the steps and checks the result.

1. Change into the cell tree: `cd clusters/<cell>/`. sops finds its rule by walking up from the current directory.
2. To edit an existing secret, the engineer runs `sops <path>`. sops decrypts it into an editor and encrypts it again on save.
3. To add a secret, the engineer writes the plain file, then runs `sops --encrypt --in-place <path>`.
4. The agent checks the committed file: it has a `sops:` block, its `kms` entry names the cell's own key, and every value under `data` or `stringData` starts with `ENC[`.

Each step needs KMS access to `alias/<cell>` through SSO. A file encrypted under another cell's key fails to decrypt on its own cell, and Flux reports the failure only on the Kustomization ([FM1](diagnosis.md#fm1-a-merged-change-does-not-reach-the-cell)).

## E: Add an alert

Writes: a pull request. The `sre-engineer` agent designs the expression, the threshold and the runbook.

1. Put the rule in the right directory: `manifests/base/monitoring/alerts/` for the fleet, `clusters/prod/monitoring/alerts/` for the hub only, `clusters/<cell>/monitoring/alerts/` for one cell. List it in that directory's `kustomization.yaml`. A new directory on prod-apne1 also goes in `clusters/prod-apne1/monitoring-configs/kustomization.yaml`.
2. Set `severity` (`critical`, `warning`, `info` or `blackhole`) and `team` (`platform` or `protocol`). Add `chain_id` to a protocol alert. The protocol route exists only on prod.
3. Run **metric-exists** on every metric in the expression, on the cells where the rule runs.
4. Generate the Explore link: `python3 scripts/grafana_explore_url.py generate --cell prod --datasource prometheus-prod --expr '<expr>' --from now-6h`. Add a `runbook_url` in sei-protocol/runbooks.
5. Write `tests/<rulefile>_test.yaml` beside the rule file.
6. Run `python3 -m unittest discover -s scripts -p "test_*.py"` and `python3 scripts/validate_monitoring_configs.py`. Use the promtool and amtool versions that `.github/workflows/monitoring-config-validation.yml` pins, because a newer promtool can pass a test that CI fails.
7. If routing changes, add a case to `scripts/test_alertmanager_routes.py`.

## F: Expose a service

Writes: a pull request. Attach an HTTPRoute to the shared gateway, and pin the HTTPS listener:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: grafana
spec:
  parentRefs:
    - name: sei-gateway
      namespace: gateway
      sectionName: https
  hostnames:
    - grafana.prod.platform.sei.io
  rules:
    - backendRefs:
        - name: grafana
          port: 80
```

With `sectionName: https`, plain HTTP on that hostname gets the shared redirect to HTTPS. A hostname under `<cell>.platform.sei.io` uses the `https` listener. A chain hostname on prod uses that chain's listener, for example `https-pacific-1`. external-dns publishes the hostname from the route. If the record never appears, follow [FM5](diagnosis.md#fm5-a-hostname-does-not-resolve).

## G: Review an engineer onboarding

`/harbor-dev` opens the onboarding pull requests for an engineer. This procedure is for the platform reviewer. One platform pull request touches four files:

1. `clusters/harbor/engineers/<alias>/kustomization.yaml`, a copy of an existing engineer's file with the alias changed;
2. the roster in `clusters/harbor/engineers/kustomization.yaml`;
3. `terraform/aws/189176372795/eu-central-1/harbor/engineers/<alias>.tf`, the Pod Identity associations;
4. `clusters/harbor/monitoring/podmonitor-seiload-eng.yaml`, so that Prometheus scrapes the cell's load jobs.

A sibling pull request in `harbor-engineering-workspace` adds the workspace. It merges first or together.

The onboarding is the one Terraform change whose plan runs after the merge, as `/harbor-dev` does it. After the merge, an engineer runs `terraform plan -target=module.engineers -out=tfplan` from the harbor root. The plan must add only the new engineer's Pod Identity resources, and change or destroy nothing. Only then does the engineer run `terraform apply tfplan` ([C](#c-run-terraform-safely)).

Offboarding removes the same four entries, and Flux deletes the tenant cell, because tenant Namespaces stay prunable.

## H: Retire a SeiNode

Writes: a pull request, then deletes in the cell. Most SeiNodes and their external objects carry `prune: disabled`, so a merge that removes them deletes nothing.

1. Before the pull request merges, confirm with the user that the node is the right one and that nothing needs its data. Then read its guard: `kubectl --context <cell> -n <ns> get seinode <name> -o jsonpath='{.metadata.annotations.kustomize\.toolkit\.fluxcd\.io/prune}'`. An empty result means that the merge itself deletes the node and its data volume.
2. Merge the removal from git.
3. Sync the cell: `flux --context <cell> reconcile kustomization flux-system -n flux-system --with-source`. If you delete before the merge reaches the cell, Flux recreates the objects.
4. Delete the SeiNode, and wait for its pod to go. The finalizer deletes the data PVC, and its storage class deletes the volume.
5. Delete its HTTPRoutes and Services. A LoadBalancer Service delete also deletes its NLB.
6. If the node imports a static volume, as archive and frozen nodes do, the PV is `Retain`. Delete the PVC, check that the PV shows `Released`, then delete the PV. Then delete the EBS volume in AWS, because a `Retain` volume outlives its PV and keeps billing.
7. Delete the Route53 records and their `extdns` TXT records by hand, because external-dns runs `upsert-only`.

A SeiNode without `prune: disabled` skips step 4. Flux deletes it as soon as the merge reaches the cell, and the finalizer then deletes its data volume. For that reason, step 1 comes before the merge for every SeiNode. Step 3 applies to every SeiNode. A manual delete in step 5 that runs before Flux applies the merge comes back at the next apply.

## I: Roll a chain release

Writes: one pull request per stage. The runbook `platform/chain-release/seid-release-and-arctic-1-rollout.md` in sei-protocol/runbooks is the source of truth. Each roll adds a record in `docs/rollouts/`. An arctic-1 roll moves in five steps:

1. **Cut.** Cut the release in sei-chain. The fleet deploys it by commit sha and digest.
2. **Classify.** Read the diff. A compatible release takes the plain roll. A consensus-breaking release takes the governance path.
3. **Batch.** A batch of validators that restart together must hold less than one third of voting power. Read the voting power from the chain before you plan the batches.
4. **Stage.** Open one pull request per stage, in the order that the runbook gives. The harbor staging syncer goes first, then each prod cell in turn, then the seeds and the reserve nodes. Frozen nodes never move.
5. **Merge.** Merge each stage only after the previous batch signs at a settled height and every node shows `spec.image` equal to `status.currentImage`.

The steps that failed in past rolls:

- **Drift budget.** With the controller's drift budget, `NodeUpdateInProgress=False` can mean "deferred". Check `spec.image` against `status.currentImage` on every node.
- **Vote timing.** Merge the `GovVote` tasks only with the verified proposal ID, after the proposal is on chain.
- **Task names.** A SeiNodeTask name runs once. A resubmit needs a new suffix.
- **Halt timing.** Merge the governance image change only after the upgrade height commits and the pods log `UPGRADE NEEDED`. A binary with the upgrade handler panics with `BINARY UPDATED BEFORE TRIGGER` below the plan height.
- **pacific-1 coverage.** pacific-1 runs on prod, prod-use2, prod-euw1 and harbor. List every pacific-1 SeiNode in those four cells before you merge.
- **Rollback.** `git revert` rolls back a compatible release only. After more than two thirds of voting power commits past the upgrade height, no rollback exists.

## J: One-way doors

A one-way door needs the owner's approval before anyone changes it (Guardrail 5).

| Door | Why it is one-way |
|---|---|
| Cilium cluster ID | A change needs a full Cilium restart of the cell. |
| Prometheus `externalLabels` | The Thanos compactor halts on mixed label sets. |
| kube-prometheus-stack `fullnameOverride` | The Thanos sidecar NLB selects `<fullnameOverride>-prometheus`, so a change drops the cell from the hub's queries. |
| P2P NLB hostnames | Seeds publish them and validators hard-code them. A recreated Service gets a new hostname. |
| Seed NodeIDs | Peers hold them in their configuration. |
| `loadBalancerClass` | It is immutable on a Service. The adopt-only manifests omit it, so never create a Service from them. |
| SOPS and state KMS keys | Their loss makes the secrets or the state unreadable. |
| Gateway NLB hostname | DNS records that Terraform manages on the hub point at it, and they change only on a Terraform apply. |
| A SeiNode's data volume | A delete loses the chain data. A validator also loses its signing state. |
