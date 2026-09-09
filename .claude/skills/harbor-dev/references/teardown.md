# Teardown (chain, bench, namespace)

Teardown removes an engineer's workloads from `eng-<alias>` through the same PR contract that created them. One ordering rule governs the whole file: **patch `spec.deletionPolicy` to `Delete` on every SeiNetwork you are about to remove, and land that patch before the removal merges.** A SeiNetwork deleted under the default `Retain` orphans its generated validator SeiNodes, and each orphan keeps its PVC and its EBS disk running with nothing left to clean it up.

- [The `deletionPolicy: Retain` trap](#the-deletionpolicy-retain-trap)
- [Procedure: tear down a chain, bench, or comparison](#procedure-tear-down-a-chain-bench-or-comparison)
- [Verify the teardown](#verify-the-teardown)
  - [Target the workspace Kustomization, not `flux-system`](#target-the-workspace-kustomization-not-flux-system)
- [Procedure: empty or remove my namespace](#procedure-empty-or-remove-my-namespace)
- [Find and clean up already-leaked resources](#find-and-clean-up-already-leaked-resources)
- [Halt conditions](#halt-conditions)

## The `deletionPolicy: Retain` trap

`SeiNetwork.spec.deletionPolicy` defaults to `Retain`. On deletion under that policy the controller **strips the owner reference** from each generated validator SeiNode instead of deleting it. The result is silent:

1. The SeiNetwork disappears. Every phase read and every `kubectl get seinetwork` says the teardown worked.
2. The validator SeiNodes keep running. They lost their owner reference, so Kubernetes garbage collection has nothing to follow.
3. Flux prune never reaches them. The controller created those validators, so they were never in Flux's inventory. Prune is already enabled on the shared engineer reconciler and works correctly for what Flux owns.
4. Each orphan holds its PVC, and each PVC holds an EBS disk. The disk survives until somebody deletes the SeiNode by hand.

**The storage class is not the bug.** A `Delete` reclaim policy releases the disk only when the PVC itself gets deleted. An orphaned SeiNode never releases its PVC, so reclaim never runs. Do not change a storage class to fix this.

### The patch works only before deletion

`spec.deletionPolicy` is **mutable** — no CEL validation rule and no webhook makes it immutable, unlike `spec.genesis`, `spec.replicas`, `spec.dataVolume`, and `spec.resources`. An operator can therefore flip a live SeiNetwork from `Retain` to `Delete`.

That window closes at deletion. Once a `Retain` deletion has stripped the owner references and removed the parent SeiNetwork, no patch brings the cascade back — the parent is gone and the children are top-level objects. The leftover SeiNodes and PVCs then need the manual cleanup in [Find and clean up already-leaked resources](#find-and-clean-up-already-leaked-resources).

**Patch first, delete second. No later step recovers a teardown that ran in the other order.**

### Read the current policy

```sh
kubectl --context harbor get seinetwork <chain-id> -n eng-<alias> \
  -o jsonpath='{.spec.deletionPolicy}'
# → Retain  (the default — teardown will orphan the validators)
# → Delete  (the cascade works end to end; proceed)
# → (empty) the field is unset, which means Retain
```

An empty result is `Retain`, not "no policy". Treat it the same way.

### Set it to `Delete`

Two paths. Both must land before the removal PR merges.

**Path A — a manifest PR (default).** Add `deletionPolicy: Delete` to `spec` in `engineers/<alias>/<task>/seinetwork-<chain-id>.yaml`, merge it, and confirm the live object reads `Delete` with the command above. Then open the removal PR. This keeps the change in git, which is where every other spec field for this chain lives.

**Path B — a live patch (fast path for a disposable chain).** One PR instead of two:

```sh
kubectl --context harbor patch seinetwork <chain-id> -n eng-<alias> \
  --type=merge -p '{"spec":{"deletionPolicy":"Delete"}}'
kubectl --context harbor get seinetwork <chain-id> -n eng-<alias> \
  -o jsonpath='{.spec.deletionPolicy}'   # must print Delete before you go on
```

The patch mutates a live object outside git. That is acceptable here only because the object is about to be deleted, and only after the verify read prints `Delete`. Say in the removal PR body that the patch ran, so the reviewer sees the whole teardown.

### Render new chains with `Delete` from the start

The trap disappears if the SeiNetwork never carries `Retain`. At render time on a disposable chain, pass:

```sh
seictl network apply <chain-id> --preset genesis-chain --chain-id <chain-id> \
  --image <ref> -n eng-<alias> --dry-run --set spec.deletionPolicy=Delete
```

`--dry-run` runs server-side apply against the apiserver, so a wrong path fails the render rather than the teardown. Keep `Retain` only when the engineer wants a validator's disk preserved for forensics after the network goes away, and say so in the PR body — a retained disk is a cost the engineer is choosing.

### What the `Delete` cascade actually does

With `deletionPolicy: Delete` the chain runs end to end: SeiNetwork deleted → generated validator SeiNodes deleted through their owner references → each SeiNode's finalizer (`sei.io/seinode-finalizer`) deletes the node's data PVC → the storage class's `Delete` reclaim policy releases the EBS volume.

The finalizer **skips an imported PVC** (`spec.import` set on the SeiNode). An imported PVC is preserved by design; its disk is not a leak.

That finalizer is also why the per-engineer Role carries no `delete` on `persistentvolumeclaims`. The controller owns PVC lifecycle, and PVCs never appear in the workspace repo, so Flux prune never targets them. Do not ask for that verb — it does not fix this bug.

## Procedure: tear down a chain, bench, or comparison

Teardown follows the same PR contract as spinup: render the change, open a PR, let the engineer merge, verify what Flux did. Never `kubectl delete` a Flux-owned CR — the next reconcile re-applies it and the removal PR never lands.

1. **Pre-flight** — the five gates. Halt on first failure.
2. **Name what goes away** — the task dir, every SeiNetwork and SeiNode in it, and the PVCs those nodes hold. List them for the engineer before touching anything:

   ```sh
   kubectl --context harbor get seinetwork,seinode -n eng-<alias> \
     -l sei.io/seinetwork=<chain-id> \
     -o custom-columns='KIND:.kind,NAME:.metadata.name,ROLE:.metadata.labels.sei\.io/role,PHASE:.status.phase'
   kubectl --context harbor get pvc -n eng-<alias>
   ```
3. **Check `deletionPolicy` on every SeiNetwork in the task dir** — read it with the command in [Read the current policy](#read-the-current-policy). On `Retain` (or empty), halt and route to [Set it to `Delete`](#set-it-to-delete). Do not open the removal PR while a SeiNetwork still reads `Retain`.
4. **Confirm the policy landed** — the live object must read `Delete`. This is the gate for step 5; a removal that merges ahead of it leaks the validators' disks.
5. **Remove the manifests** — `git rm -r engineers/<alias>/<task>/` **and** remove the `<task>` entry from `engineers/<alias>/kustomization.yaml`'s `resources:` list. Both edits are required: Kustomize fails to render with a missing-resource entry, and Flux then applies nothing at all.
6. **Commit + push** — branch `feat/eng-<alias>-teardown-<task>`. Commit message: `feat(eng/<alias>): tear down <task> — chain-id=<chain-id>`.
7. **Open the PR** — title `feat(eng/<alias>): tear down <task>`. The body names the chain-id, every CR that goes away, the `deletionPolicy` value the SeiNetwork now carries, and the patch path (A or B) that set it. `gh pr create --repo sei-protocol/harbor-engineering-workspace --base main`.
8. **After merge — reconcile and verify** — [Verify the teardown](#verify-the-teardown). A merged PR is not a completed teardown.
9. **Report what survives** — the chain-id's S3 genesis artifacts are **not** purged by teardown, so the chain-id is burned. A later respin uses a fresh chain-id or purges the `<chain-id>/` prefix in `harbor-sei-k8s-genesis-artifacts` first.

## Verify the teardown

Two separate questions, and the second is the one that catches a leak: did the right reconciler run, and did the objects actually disappear?

### Target the workspace Kustomization, not `flux-system`

The engineer's manifests are applied by the Flux `Kustomization <alias>` in namespace `eng-<alias>`, which watches `harbor-engineering-workspace` at `./engineers/<alias>` and reconciles every 5 minutes. The root `flux-system` Kustomization tracks `sei-protocol/platform` at `clusters/harbor`. Reconciling `flux-system` after a workspace-repo merge reconciles a different repository and reports success without applying the engineer's change.

Use `flux-system` after a **platform**-repo merge (onboarding). Use `<alias>` after a **workspace**-repo merge (every chain, bench, and teardown).

```sh
flux --context harbor reconcile kustomization <alias> -n eng-<alias> --with-source
```

Fallback when `flux` is absent:

```sh
kubectl --context harbor -n eng-<alias> annotate kustomization <alias> \
  reconcile.fluxcd.io/requestedAt="$(date +%s)" --overwrite
```

Then confirm the merge commit landed:

```sh
kubectl --context harbor -n eng-<alias> get kustomization <alias> \
  -o jsonpath='{.status.lastAppliedRevision}'
```

Compare that revision to the merge commit SHA. A stale revision means Flux has not applied the removal yet, so any disappearance check below is premature.

If `--with-source` returns `Forbidden`, the `GitRepository` the Kustomization references sits outside `eng-<alias>` and the engineer's namespace-scoped Role does not reach it. Drop `--with-source` and reconcile the Kustomization alone; it applies the revision the source has already fetched, and the source polls on its own schedule.

### Confirm the resources disappeared

A successful reconcile says Flux applied the change. It does not say the objects are gone. Deletion is asynchronous and finalizers hold objects in `Terminating` while the controller releases their PVCs, so poll instead of asserting once:

```sh
end=$((SECONDS + 300))
while [ "$SECONDS" -lt "$end" ]; do
  left=$(kubectl --context harbor get seinetwork,seinode -n eng-<alias> \
    -l sei.io/seinetwork=<chain-id> -o name | wc -l)
  if [ "$left" -eq 0 ]; then echo "all objects gone"; break; fi
  echo "$left object(s) remain"; sleep 10
done
```

Then confirm the disks went with them:

```sh
kubectl --context harbor get pvc -n eng-<alias> \
  -o custom-columns='NAME:.metadata.name,STATUS:.status.phase,VOLUME:.spec.volumeName,CLASS:.spec.storageClassName'
```

Every PVC belonging to the torn-down chain must be gone. A `Bound` PVC that outlives its SeiNode is a held disk.

### A stuck `Terminating` object is a real signal

If the poll runs out with objects still present, do not report the teardown as done and do not force the objects away. Read why they are held:

```sh
kubectl --context harbor get seinetwork,seinode -n eng-<alias> \
  -l sei.io/seinetwork=<chain-id> \
  -o custom-columns='NAME:.metadata.name,PHASE:.status.phase,DELETED:.metadata.deletionTimestamp,FINALIZERS:.metadata.finalizers'
```

`sei.io/seinode-finalizer` on a SeiNode blocks its deletion until the controller releases the PVC. A node parked there means the controller is unhealthy or the EBS CSI driver flaked — check `kubectl logs -n sei-k8s-controller-system -l app.kubernetes.io/name=sei-k8s-controller --tail=100` and surface what it says. Removing the finalizer by hand (`troubleshooting-seinode.md` → *PVC stuck after delete*) abandons the PVC and its disk, which is the leak this file exists to prevent. Take that step only after the engineer accepts the orphaned PVC, and record the PVC name so somebody can clean it up.

## Procedure: empty or remove my namespace

"Destroy my namespace" means one of two very different things. Ask which before acting.

### What a workspace-repo PR removes

A workspace-repo PR governs `engineers/<alias>/` only. Removing every task dir removes:

- Every SeiNetwork and SeiNode the engineer's manifests declared, and their pods, StatefulSets, headless Services, and controller-managed PVCs.
- Every bench Job and ConfigMap.
- Any engineer-owned exposure YAML (`Service`, `HTTPRoute`) in those task dirs.

It does **not** remove:

- **The `Namespace` object.** It comes from the platform repo (`clusters/harbor/engineers/base/namespace.yaml`) and stays.
- The three ServiceAccounts, the `<alias>` Role and RoleBinding, or the `engineer-admin` RoleBinding — all platform-owned.
- The Flux `Kustomization <alias>` itself. It keeps reconciling an empty `engineers/<alias>/`.
- Anything created outside git — an escape-hatch direct apply, a `SeiNodeTaskWorkflow`, or an orphaned SeiNode from an earlier `Retain` teardown. Flux prune only reaches what Flux applied.
- S3 artifacts: the chain-ids' genesis prefixes and the bench results under `harbor-validation-results/eng-<alias>/`.

### Empty the namespace (the common case)

1. Inventory everything first, including what git does not know about:

   ```sh
   kubectl --context harbor get seinetwork,seinode,job,pvc -n eng-<alias>
   ```
2. For every SeiNetwork in the inventory, run the `deletionPolicy` gate in [The `deletionPolicy: Retain` trap](#the-deletionpolicy-retain-trap). One `Retain` network is enough to leak a set of disks.
3. `git rm -r` every task dir under `engineers/<alias>/`, and reduce `engineers/<alias>/kustomization.yaml` to `resources: []`. Keep that file: deleting it makes the Flux Kustomization fail reconcile with `path not found`, which is the same breakage the onboarding scaffolding PR exists to prevent.
4. Open the PR, merge, then run [Verify the teardown](#verify-the-teardown) with no `-l` selector, so the poll covers the whole namespace.
5. Sweep for what git never owned — [Find and clean up already-leaked resources](#find-and-clean-up-already-leaked-resources).

### Remove the namespace entirely (offboarding)

This is a platform-repo change and the engineer cannot do it from the workspace repo. It reverses the onboarding PR: delete `clusters/harbor/engineers/<alias>/`, remove `<alias>` from `clusters/harbor/engineers/kustomization.yaml`, remove `eng-<alias>` from `clusters/harbor/monitoring/podmonitor-seiload-eng.yaml`, delete `terraform/aws/189176372795/eu-central-1/harbor/engineers/<alias>.tf`, and run the targeted `terraform apply` to drop the six Pod Identity resources.

Empty the namespace first, through the steps above. Deleting the `Namespace` object while SeiNetworks still live in it starts a namespace-wide cascade that races the controller's finalizers and can strand PVCs with no owning CR to inspect.

The file list mirrors the onboarding shape in `onboarding-pr.md`; the reverse flow has no worked example in this skill. Surface it to the platform team through `#harbor-onboarding` rather than opening the PR unassisted.

## Find and clean up already-leaked resources

Run this after any teardown that ran under `Retain`, and any time an engineer asks where the harbor spend is going.

### Orphaned SeiNodes

An orphaned validator has **no `ownerReferences`** and no live parent SeiNetwork. Absence of owner references alone is not the signal: a follower applied through `seictl node apply` is a top-level object and legitimately has none. The signature is `sei.io/role=validator` **and** no owner references.

```sh
kubectl --context harbor get seinode -n eng-<alias> -l sei.io/role=validator -o json \
  | jq -r '.items[]
      | select((.metadata.ownerReferences // []) | length == 0)
      | "\(.metadata.name)\t\(.metadata.labels["sei.io/seinetwork"] // "-")\t\(.status.phase // "-")\t\(.metadata.creationTimestamp)"'
```

Each line is a validator still running with nothing that will ever delete it. Confirm the parent is gone before treating one as an orphan:

```sh
kubectl --context harbor get seinetwork <seinetwork-label-value> -n eng-<alias>
# NotFound → the parent is gone and this node is orphaned
```

### Held and leaked disks

An orphaned SeiNode still shows a **`Bound`** PVC — the disk is attached and billing, not free-floating. A disk whose PVC has already gone shows up on the AWS side as **`available`**. Check both.

```sh
kubectl --context harbor describe pvc <name> -n eng-<alias> | grep -A2 'Used By'
# Used By: <pod>   → an orphaned node is holding it
# Used By: <none>  → Bound but unattached; nothing in-cluster references it
```

On the AWS side, the EBS CSI driver tags each volume with the PVC it was provisioned for:

```sh
aws ec2 describe-volumes --region eu-central-1 --profile <chosen> \
  --filters "Name=tag:kubernetes.io/created-for/pvc/namespace,Values=eng-<alias>" \
  --query 'Volumes[].{id:VolumeId,state:State,size:Size,created:CreateTime,pvc:Tags[?Key==`kubernetes.io/created-for/pvc/name`]|[0].Value}' \
  --output table
```

`state: in-use` with an orphaned SeiNode above it is a running leak. `state: available` is a disk nothing references at all. Those tag keys are the EBS CSI driver's own convention rather than something this skill's repos set — run the command once without `--filters` against a volume you know is live to confirm the keys are present before trusting an empty result.

The engineer's SSO profile may lack `ec2:DescribeVolumes`. On `AccessDenied`, surface the ask to the platform team with the namespace and the orphaned node names; do not treat the denial as "no leaked disks".

### Clean them up

**Delete the orphaned SeiNode. That is the whole cleanup for a held disk.**

```sh
kubectl --context harbor delete seinode <name> -n eng-<alias>
```

The node's finalizer deletes its data PVC, and the `Delete` reclaim policy on `gp3-10k-750` (validators) and `gp3` (default) releases the EBS volume. `gp3-archive` is `Retain` by design — a volume on that class stays after its PVC goes, and its removal is an AWS-side decision, not a mistake to correct here.

Two checks before you run it:

- The node must be a confirmed orphan by the signature above. `kubectl delete seinode` against a follower that still has a manifest in the workspace repo is undone by the next Flux reconcile, and the safer `git rm` path never lands.
- Poll the disappearance and the PVC afterwards, exactly as in [Verify the teardown](#verify-the-teardown). An orphan can stick in `Terminating` for the same finalizer reasons.

An imperative `kubectl delete` is right here and nowhere else in teardown: the object was never in git, so there is no manifest to `git rm`.

**A volume already `available` in EC2 has no in-cluster handle left.** Deleting it needs `ec2:DeleteVolume`, which the engineer's profile is unlikely to carry. Collect the volume IDs, sizes, and creation times, and escalate to the platform team through `#harbor-onboarding`. Do not report the cleanup as complete while those IDs are outstanding.

## Halt conditions

Stop and report. Do not auto-remediate.

- **A SeiNetwork in the teardown reads `deletionPolicy: Retain` or empty.** Removing it orphans the validators and leaks their disks. Halt before opening the removal PR; route to [Set it to `Delete`](#set-it-to-delete).
- **The removal PR merged while a SeiNetwork still read `Retain`.** The cascade is gone and no patch restores it. Do not re-apply the SeiNetwork to "reattach" the children — a fresh network under the same chain-id wedges at height 0 on the burned genesis artifacts. Go straight to [Find and clean up already-leaked resources](#find-and-clean-up-already-leaked-resources).
- **`kustomization <alias>` is `NotFound` in `eng-<alias>`.** The engineer's Flux wiring is missing, so no workspace-repo merge reconciles at all. Surface to the platform team; do not create the Kustomization.
- **`lastAppliedRevision` does not reach the merge commit within two reconcile intervals (~10 min).** Read the Ready condition's message (`cluster-inspection-recipes.md` recipe #8). A render error in `engineers/<alias>/kustomization.yaml` — most often a `resources:` entry pointing at the dir that was just removed — blocks every later apply in the namespace, not only this teardown.
- **An object is still `Terminating` after the poll budget.** Report the finalizer and the controller's log line. Do not strip the finalizer to make the check pass.
- **Orphaned SeiNodes found in a namespace the engineer does not own.** Cross-tenant cleanup is out of scope. Hand the platform team the namespace and the node names.
- **`aws ec2 describe-volumes` returns `AccessDenied`.** The leak check did not run. Say that, rather than reporting a clean result.
