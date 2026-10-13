# Platform diagnosis

This file holds the failure modes behind the diagnosis method in `SKILL.md`. Each failure mode gives the mechanism, the symptom, the check and the safe response. Every check is read-only. A safe response that needs a write runs only on an explicit request (Guardrail 1). The bold query names resolve in [observability](observability.md#2-named-queries).

Contents:

- [FM1: a merged change does not reach the cell](#fm1-a-merged-change-does-not-reach-the-cell)
- [FM2: Flux deleted an object](#fm2-flux-deleted-an-object)
- [FM3: a Kustomization stays suspended](#fm3-a-kustomization-stays-suspended)
- [FM4: a HelmRelease fails](#fm4-a-helmrelease-fails)
- [FM5: a hostname does not resolve](#fm5-a-hostname-does-not-resolve)
- [FM6: a certificate does not renew](#fm6-a-certificate-does-not-renew)
- [FM7: the gateway returns errors](#fm7-the-gateway-returns-errors)
- [FM8: a cell leaves the hub's view](#fm8-a-cell-leaves-the-hubs-view)
- [FM9: logs stop](#fm9-logs-stop)
- [FM10: a volume fills](#fm10-a-volume-fills)
- [FM11: a pod cannot get a node](#fm11-a-pod-cannot-get-a-node)
- [FM12: a load balancer stays pending](#fm12-a-load-balancer-stays-pending)
- [FM13: the nightly gate fails](#fm13-the-nightly-gate-fails)
- [FM14: an alert never fires](#fm14-an-alert-never-fires)

## FM1: a merged change does not reach the cell

- **Mechanism.** The cell's GitRepository fails to fetch, or the Kustomization fails to build or apply. A change can also sit behind a per-cell pin that the pull request did not move. A SOPS file encrypted under another cell's key fails to decrypt, and only the Kustomization reports it.
- **Symptom.** The object in the cell does not match `main` more than about 6 minutes after the merge.
- **Check.** Run **flux-revision** and compare with `git rev-parse origin/main`. Then run `flux --context <cell> get kustomizations -A` and `flux --context <cell> get sources git -A`, and read the message. On prod-apne1, a stage waits for the stage before it, so read all five. Check that the cell's `kustomization.yaml` chain lists the changed file, and that no pin in `clusters/<cell>/` holds an older ref.
- **Safe response.** Fix the cause in a pull request. A `flux reconcile` only retries; it fixes nothing that git still breaks. A reconcile is a write.

## FM2: Flux deleted an object

- **Mechanism.** Every Kustomization sets `prune: true`. Flux deletes an object that leaves the build, unless it carries `prune: disabled`. An object leaves the build when someone removes its file, when a `kustomization.yaml` stops listing it, or when a rename gives it a new identity. A SeiNode delete runs the finalizer, which deletes the data volume.
- **Symptom.** The cell no longer has the object. The kustomize-controller log names the delete.
- **Check.** Read the kustomize-controller log for the object name ([logs](observability.md#3-logs)). Run `git log -p --since=<time> -- <path>` on the repository. Read the object's last manifest in git for the prune annotation.
- **Safe response.** Restore the object through a revert pull request. Nothing restores a deleted SeiNode volume. The node must sync again from a snapshot or a peer, and a validator also needs its keys. Halt and hand that to the chain's owner. Then add `prune: disabled` to the object class in a pull request.

## FM3: a Kustomization stays suspended

- **Mechanism.** Someone suspended a Kustomization and did not resume it. No alert covers a suspend. On a bootstrap cell the root `flux-system` applies nearly everything, so the whole cell stops taking changes.
- **Symptom.** Merges stop reaching one cell, and **flux-revision** shows an old commit for it.
- **Check.** Run **flux-suspended**, or `flux --context <cell> get kustomizations -A` and read the SUSPENDED column.
- **Safe response.** Find who suspended it and why, before anyone resumes. On resume, Flux applies and prunes everything that merged meanwhile, in one pass. List what that pass would delete with `flux --context <cell> diff kustomization <name> --path <dir>` first. The resume is a write.

## FM4: a HelmRelease fails

- **Mechanism.** A chart upgrade fails its install, its test or its timeout. helm-controller retries, and the release stays at its last good revision or in a failed state.
- **Symptom.** **flux-not-ready** shows the HelmRelease. The component runs its old version, or it runs in a degraded state.
- **Check.** Run `flux --context <cell> get helmreleases -A`, then `kubectl --context <cell> -n <ns> describe helmrelease <name>`. Read the helm-controller log for the release name.
- **Safe response.** Fix the values or the chart version in a pull request. Do not run `helm` by hand against a release that Flux owns.

## FM5: a hostname does not resolve

- **Mechanism.** external-dns publishes records from HTTPRoutes. It fails with `AccessDenied` for a name outside its IAM allowlist, and it writes that only to its log. It runs `upsert-only`, so it never deletes a record, and a stale record can point to a retired load balancer.
- **Symptom.** `dig` returns no answer, or an address that no longer serves.
- **Check.** Read the HTTPRoute's hostnames and its status. Search the external-dns log of the cell for the hostname and for `AccessDenied` ([logs](observability.md#3-logs)). Check that the hostname sits under a zone that the cell may write: `<cell>.platform.sei.io`, or a chain zone on prod.
- **Safe response.** Fix the hostname or the IAM allowlist in a pull request. Delete stale records and their `extdns` TXT records in Route53 only on an explicit request.

## FM6: a certificate does not renew

- **Mechanism.** cert-manager renews a certificate from the `letsencrypt` ClusterIssuer at about 30 days left, with a Route53 DNS-01 challenge. The challenge fails when the cell's role cannot write the zone or when the cert-manager webhook is unreachable. On a Cilium cell the webhook must run on the host network.
- **Symptom.** The certificate's days left fall below 30 and keep falling. Clients then see an expired certificate.
- **Check.** Run **cert-days-left** for prod. On any cell, run `kubectl --context <cell> get certificates,certificaterequests,challenges -A` and describe the failing one. Read the cert-manager log.
- **Safe response.** Fix the cause in a pull request: the IAM permission, the webhook placement or the issuer. Do not delete the certificate's Secret to force a renewal.

## FM7: the gateway returns errors

- **Mechanism.** The Istio gateway routes to a node's `-external` Service. A 503 means no ready endpoint behind the Service. A 404 means no route matches the hostname and path. An unavailable gateway means fewer than two ready Envoy replicas or a failed NLB target.
- **Symptom.** Clients see 404, 503 or connection failures on a hostname.
- **Check.** Run **gateway-replicas** and **gateway-5xx**. Then `kubectl --context <cell> -n <ns> get httproute <name> -o yaml` and read `status.parents`. Then `kubectl --context <cell> -n <ns> get endpointslices -l kubernetes.io/service-name=<node>-external`. Read the gateway log for the hostname.
- **Safe response.** A node with no ready endpoint is a node problem: read its SeiNode and pod. A route that the gateway does not accept is a manifest fix. Never scale or restart the shared gateway on your own initiative.

## FM8: a cell leaves the hub's view

- **Mechanism.** The hub reads each spoke's sidecar and store gateway on port 10901 through an internal NLB and VPC peering. A change to the kube-prometheus-stack `fullnameOverride` breaks the NLB's selector. A broken peering route or a stopped Prometheus has the same effect.
- **Symptom.** The cell is missing from **cells-reporting**, its Grafana panels go blank, and **firing** no longer shows its alerts. Paging depends on the cause. A sidecar, NLB or peering break leaves the cell's own Prometheus and Alertmanager paging. A stopped Prometheus or Alertmanager pages nobody.
- **Check.** Run **cells-reporting**, **thanos-stores** and **thanos-store-errors**. Then run `kubectl --context <cell> -n monitoring get pods` to see whether the cell's Prometheus and Alertmanager run, and read the sidecar NLB Service.
- **Safe response.** Restore the cell's Prometheus or the path through a pull request. While the cell is dark, check its health directly with `kubectl`.

## FM9: logs stop

- **Mechanism.** Alloy in each cell writes to the cell's Loki and to `loki-fleet`. A write path failure, a rate limit or a stopped Alloy DaemonSet drops lines.
- **Symptom.** `loki-fleet` returns nothing for a cell, or the newest line is old.
- **Check.** Run **loki-ingest**. Then **firing** for `Loki*` alerts. Then query the cell's own Loki datasource to see whether only the fleet path fails.
- **Safe response.** Use the cell's own Loki while the fleet path is down. Fix the cause in a pull request.

## FM10: a volume fills

- **Mechanism.** Chain data grows without bound, and archive and syncer volumes grow fastest. Monitoring volumes grow with retention and cardinality.
- **Symptom.** `PVCNearFull` at 85%, or a pod that crashes on a write error.
- **Check.** Run **pvc-fill** now and with a `range` query over 7 days to read the growth rate.
- **Safe response.** A volume resize is a manifest change in a pull request. For a SeiNode, the controller and its storage class decide how a resize applies; ask `/kubernetes` before you change it. Never delete data to free space.

## FM11: a pod cannot get a node

- **Mechanism.** Karpenter adds nodes for pending pods within each NodePool's limits. A pool at its CPU limit, a missing instance type, or an EC2 capacity error leaves the pod pending.
- **Symptom.** A pod stays `Pending`, and its events say that no node fits.
- **Check.** Run **nodepool-usage** and **node-not-ready**. Then `kubectl --context <cell> get nodepools,nodeclaims` and the pod's events. Read the Karpenter log.
- **Safe response.** A NodePool limit change is a pull request to `clusters/base/default/` or the cell's pool file.

## FM12: a load balancer stays pending

- **Mechanism.** Each P2P Service and each gateway has its own NLB. A region has an account quota for load balancers, and prod and harbor share eu-central-1. A quota hit leaves a new Service without an address.
- **Symptom.** A LoadBalancer Service shows no external address. The AWS Load Balancer Controller logs a quota error.
- **Check.** Run `kubectl --context <cell> -n <ns> describe svc <name>` and read the events. Count the region's load balancers with `aws elbv2 describe-load-balancers --region <region> --query 'length(LoadBalancers)'`.
- **Safe response.** Ask the platform team to raise the quota, or retire unused load balancers through [procedure H](procedures.md#h-retire-a-seinode).

## FM13: the nightly gate fails

- **Mechanism.** The `nightly-harness-suite` CronJob on harbor runs the controller's nightly suites against fresh chains. A failure can come from seid, from the harness, from the load generator, or from harbor capacity.
- **Symptom.** A failed Job in `nightly`, and `Nightly*` alerts on harbor. Harbor has no protocol route, so they page the platform service.
- **Check.** Run `kubectl --context harbor -n nightly get jobs,seinodes` and **firing** with `cluster="harbor"`. Read the Job's pod log for the first failed test.
- **Safe response.** Name the failed suite and its first error. A seid regression goes to the chain team with the evidence. Do not delete nightly SeiNodes by hand while a run is active.

## FM14: an alert never fires

- **Mechanism.** A rule whose metric has no series evaluates to nothing. An exporter upgrade renames a metric, or a component loses its scrape target. A rule on dev reads dev's own Prometheus only.
- **Symptom.** A fault that the rule describes happens and nothing pages.
- **Check.** Run **metric-exists** on each metric in the rule. Check that the component has a scrape target: `count by (cluster, job) (up{job=~".*<component>.*"})`.
- **Safe response.** Fix the rule or the scrape target in a pull request. The `sre-engineer` agent reviews the expression. The [rule-metric sweep](maintenance.md#4-rule-metric-sweep) finds every such rule at once.
