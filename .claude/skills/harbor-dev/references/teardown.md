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

`spec.deletionPolicy` is **mutable** — no CEL validation rule and no webhook makes it immutable. `SeiNetworkSpec` carries exactly three immutability rules, on `spec.genesis`, `spec.replicas`, and `spec.dataVolume`. An operator can therefore move a SeiNetwork from `Retain` to `Delete`.

That window closes at deletion. Once a `Retain` deletion has stripped the owner references and removed the parent SeiNetwork, no patch brings the cascade back — the parent is gone and the children are top-level objects. The leftover SeiNodes and PVCs then need the manual cleanup in [Find and clean up already-leaked resources](#find-and-clean-up-already-leaked-resources).

**Set the policy first, delete second. No later step recovers a teardown that ran in the other order.**

### Read the current policy

```sh
kubectl --context harbor get seinetwork <chain-id> -n eng-<alias> \
  -o jsonpath='{.spec.deletionPolicy}'
# → Retain  (the default — teardown will orphan the validators)
# → Delete  (the cascade works end to end; proceed)
# → (empty) the field is unset, which means Retain
```

An empty result is `Retain`, not "no policy". Treat it the same way.

### Set it to `Delete` — the change must land in git

**A `kubectl patch` alone does not survive to merge time.** Flux reconciles `engineers/<alias>/` every 5 minutes against what git declares. The manifest that spun the chain up was rendered from `seictl network apply --dry-run`, which captures the server-defaulted CR, so `deletionPolicy: Retain` is normally written out in the committed file. Flux owns that field, and the next reconcile reverts the patch — typically while the removal PR sits in review. The engineer then merges a teardown they believe is safe, and it orphans the validators anyway. Do not rely on server-side-apply field ownership to keep a patch alive across a reconcile, even where git happens to omit the field.

**The policy change goes in git, and it reconciles, before the removal merges.** Two orderings do that correctly.

**Path A — two PRs (the default).**

1. **Policy PR.** Set `deletionPolicy: Delete` in `spec` in `engineers/<alias>/<task>/seinetwork-<chain-id>.yaml`. Nothing else. Merge it.
2. **Confirm it reconciled onto the live object** — the reconcile, then the read-back, then the git state:

   ```sh
   flux --context harbor reconcile kustomization <alias> -n eng-<alias> --with-source
   kubectl --context harbor get seinetwork <chain-id> -n eng-<alias> \
     -o jsonpath='{.spec.deletionPolicy}'   # must print Delete
   grep -n 'deletionPolicy' engineers/<alias>/<task>/seinetwork-<chain-id>.yaml   # must read Delete
   ```

   Both reads must agree on `Delete`. A live object reading `Delete` while git still declares `Retain` is the drift this path exists to close.
3. **Removal PR.** Only now `git rm` the task dir, per the procedure below.

**Path B — one PR that sets the policy and removes nothing else yet.** Where two PRs are too much ceremony, put the policy edit in the removal branch as its **own commit**, merge the branch, and then confirm with the three reads above before the removal commit is allowed to land. This is Path A with the review collapsed, not a shortcut past the ordering. If the branch merges as one unit, it is not this path — it is a `Retain` teardown.

**The live patch is a repair, not a fast path.** `kubectl patch seinetwork <chain-id> -n eng-<alias> --type=merge -p '{"spec":{"deletionPolicy":"Delete"}}'` is correct in one situation: the SeiNetwork is **not** in the workspace repo at all (an escape-hatch direct apply, or an object already orphaned from an earlier teardown), so no reconcile will revert it. Against a Flux-owned SeiNetwork the patch is drift that Flux undoes on its own schedule. If an engineer insists on it anyway, re-read `.spec.deletionPolicy` **immediately before the removal PR merges** rather than once at patch time — a read taken minutes earlier proves history, not the state at merge.

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
2. **Name what goes away, and what stays** — the task dir, every SeiNetwork and SeiNode in it, and the PVCs those nodes hold. List them for the engineer before touching anything. **Record which SeiNodes carry `spec.import`**: their PVCs survive the teardown by design, and once the nodes are deleted nothing in the cluster still says which PVCs those were.

   ```sh
   kubectl --context harbor get seinetwork,seinode -n eng-<alias> \
     -l sei.io/seinetwork=<chain-id> \
     -o custom-columns='KIND:.kind,NAME:.metadata.name,ROLE:.metadata.labels.sei\.io/role,PHASE:.status.phase'

   # Imported PVCs — expected to SURVIVE. Everything else is controller-managed.
   kubectl --context harbor get seinode -n eng-<alias> -l sei.io/seinetwork=<chain-id> -o json \
     | jq -r '.items[] | select(.spec.import != null) | "\(.metadata.name)\timported"'

   kubectl --context harbor get pvc -n eng-<alias>
   ```
3. **Check `deletionPolicy` on every SeiNetwork in the task dir** — read it with the command in [Read the current policy](#read-the-current-policy). On `Retain` (or empty), halt and route to [Set it to `Delete`](#set-it-to-delete). Do not open the removal PR while a SeiNetwork still reads `Retain`.
4. **Confirm the policy landed in git and on the object** — the committed manifest and the live object must both read `Delete`. The live object alone is not enough: Flux reverts a policy that git still declares `Retain`, and it does so on its own schedule, which can fall inside the removal PR's review window. This is the gate for step 5.
5. **Remove the manifests** — `git rm -r engineers/<alias>/<task>/` **and** remove the `<task>` entry from `engineers/<alias>/kustomization.yaml`'s `resources:` list. Both edits are required: Kustomize fails to render with a missing-resource entry, and Flux then applies nothing at all.
6. **Commit + push** — branch `feat/eng-<alias>-teardown-<task>`. Commit message: `feat(eng/<alias>): tear down <task> — chain-id=<chain-id>`.
7. **Open the PR** — title `feat(eng/<alias>): tear down <task>`. The body names the chain-id, every CR that goes away, the `deletionPolicy` value the SeiNetwork now carries in git and on the live object, and which path set it. `gh pr create --repo sei-protocol/harbor-engineering-workspace --base main`.
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

A `Forbidden` on `--with-source` **may** mean the `GitRepository` the Kustomization references sits outside `eng-<alias>`, beyond the engineer's namespace-scoped Role. It may equally be an expired session, a missing EKS access entry, or a Role that never carried the Flux verbs. Read the message before concluding which. Whatever the cause, dropping `--with-source` and reconciling the Kustomization alone still applies the revision the source has already fetched, and the source polls on its own schedule.

### Confirm the resources disappeared

A successful reconcile says Flux applied the change. It does not say the objects are gone. Deletion is asynchronous and finalizers hold objects in `Terminating` while the controller releases their PVCs, so poll instead of asserting once.

**Three outcomes, and they are not interchangeable:**

| Outcome | Meaning | What to report |
|---|---|---|
| `GONE` | The API answered and matched nothing. | Teardown verified for these objects. |
| `PRESENT` | The API answered and objects remain at the deadline. | Not torn down. Read the finalizers below. |
| `UNVERIFIED` | The API call failed — `Forbidden`, expired credential, connection error. | **Teardown not confirmed.** Say the check could not run. |

**A failed API read is never a pass.** A `Forbidden` or a dropped connection returns zero lines, and a check that counts lines without reading the exit status prints "gone" precisely when it cannot see the cluster. Capture the status separately, every time.

```sh
# POSIX sh. Polls the chain's CRs to gone. Drop -l to sweep the whole namespace.
# Prints exactly one of GONE / PRESENT / UNVERIFIED.
deadline=$(( $(date +%s) + 300 ))
while : ; do
  out=$(kubectl --context harbor get seinetwork,seinode -n eng-<alias> \
    -l sei.io/seinetwork=<chain-id> -o name 2>&1); rc=$?
  if [ "$rc" -ne 0 ]; then
    printf 'UNVERIFIED: the API read failed (exit %s) — teardown NOT confirmed\n%s\n' "$rc" "$out"
    break
  fi
  left=$(printf '%s' "$out" | grep -c . || true)
  if [ "$left" -eq 0 ]; then echo 'GONE: no SeiNetwork or SeiNode matches'; break; fi
  if [ "$(date +%s)" -ge "$deadline" ]; then
    printf 'PRESENT at deadline: %s object(s)\n%s\n' "$left" "$out"; break
  fi
  echo "$left object(s) remain"; sleep 10
done
```

Two details are load-bearing. `rc` is captured from the `kubectl` call itself, not from a pipeline whose status belongs to `wc`. And the deadline is arithmetic on `date +%s` rather than Bash's `SECONDS`, which is unset under `sh` — there the comparison fails with `Illegal number` and the loop never runs at all.

**Then poll the disks with the same shape.** Controller-managed PVCs go away with their SeiNodes; the poll below is the same loop with the resource swapped:

```sh
deadline=$(( $(date +%s) + 300 ))
while : ; do
  out=$(kubectl --context harbor get pvc -n eng-<alias> -o name 2>&1); rc=$?
  if [ "$rc" -ne 0 ]; then
    printf 'UNVERIFIED: PVC read failed (exit %s) — disks NOT confirmed released\n%s\n' "$rc" "$out"
    break
  fi
  left=$(printf '%s' "$out" | grep -c . || true)
  # Compare `left` against the imported-PVC list from inventory step 2, not against zero.
  printf 'PVCs still in the namespace: %s\n%s\n' "$left" "$out"
  if [ "$(date +%s)" -ge "$deadline" ]; then break; fi
  sleep 10
done
```

**Zero is the wrong expectation.** The SeiNode finalizer deliberately skips an **imported** PVC (`spec.import` on the node), so an imported PVC surviving the teardown is correct behavior, not a leak. The expected end state is: every **controller-managed** PVC of the torn-down chain gone, and every imported PVC still present. That is why inventory step 2 records which nodes carry `spec.import` — after the SeiNodes are deleted, nothing in the cluster still says which PVCs were imported.

A controller-managed PVC that outlives its SeiNode is a held disk. Take it to [Find and clean up already-leaked resources](#find-and-clean-up-already-leaked-resources).

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

**Empty the namespace first, through the steps above.** This is an operational preference, not a claim about a failure mode: the Kubernetes namespace controller does remove namespaced resources on its own. Emptying first keeps the `deletionPolicy` gate, the disappearance poll, and the leak sweep available while the objects are still there to inspect. Once the namespace is going away, a `Retain` SeiNetwork's orphans are much harder to reason about, and there is no inventory left to check them against.

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

### Candidate disks — and why neither signal proves anything on its own

**Nothing in this section identifies garbage. It identifies candidates.** Two readings look conclusive and are not:

- **EC2 `state: available` does not mean unowned.** It means unattached. A volume backing a live PV whose PVC is `Bound` reads `available` the moment its workload stops — a scaled-to-zero StatefulSet, a pod stuck `Pending`, a node drained mid-reschedule. Deleting on that signal destroys a disk somebody is coming back to.
- **`Used By: <pod>` does not prove the pod belongs to an orphan**, and `Used By: <none>` does not prove the PVC is unwanted. `describe pvc` reports current pod attachment, not ownership.

So treat both as **candidate** signals, then resolve ownership before calling anything garbage.

```sh
# Candidate list only. Scoped to this tenant; do not widen the filter.
aws ec2 describe-volumes --region eu-central-1 --profile <chosen> \
  --filters "Name=tag:kubernetes.io/created-for/pvc/namespace,Values=eng-<alias>" \
  --query 'Volumes[].{id:VolumeId,state:State,size:Size,created:CreateTime,pvc:Tags[?Key==`kubernetes.io/created-for/pvc/name`]|[0].Value}' \
  --output table
```

Those tag keys are the EBS CSI driver's own convention rather than something this skill's repos set. To confirm they are present before trusting an empty result, describe **one volume you already know is live, by ID** — never re-run without `--filters`, which enumerates every volume in the account including other tenants':

```sh
aws ec2 describe-volumes --region eu-central-1 --profile <chosen> \
  --volume-ids <vol-id-you-know-is-live> --query 'Volumes[].Tags' --output table
```

### Resolve ownership before calling a disk garbage

Walk the chain from the volume back to a workload. Each hop either names an owner or fails, and a failed hop means unresolved, not unowned.

```sh
# 1. Volume ID → PV. The CSI volume handle is the EBS volume ID.
kubectl --context harbor get pv -o json \
  | jq -r --arg v '<vol-id>' '.items[]
      | select(.spec.csi.volumeHandle == $v)
      | "\(.metadata.name)\t\(.status.phase)\t\(.spec.persistentVolumeReclaimPolicy)\tclaim=\(.spec.claimRef.namespace // "-")/\(.spec.claimRef.name // "-")"'

# 2. PV claimRef → PVC. Does the claim still exist?
kubectl --context harbor get pvc <claim-name> -n <claim-namespace>

# 3. PVC → the workload that wants it.
kubectl --context harbor describe pvc <claim-name> -n <claim-namespace> | sed -n '/Used By/,+3p'
kubectl --context harbor get seinode -n <claim-namespace> -o json \
  | jq -r '.items[] | "\(.metadata.name)\t\(.status.phase // "-")"'
```

`kubectl get pv` is cluster-scoped, and the per-engineer Role is namespaced. Expect `Forbidden` here as the normal case for an engineer — that is an **unresolved** result, not a clean one. Hand the volume IDs to the platform team and let them walk the chain.

| What the walk found | Verdict |
|---|---|
| Volume → PV → PVC → a SeiNode that is a confirmed orphan | Reclaimable. Delete the **SeiNode**, not the volume — see below. |
| Volume → PV → PVC → a live, wanted workload | **Not garbage.** Leave it. `available` only meant the workload was stopped. |
| Volume → PV → PVC whose claim is gone, PV `Released` | Candidate for platform-team deletion. Report the PV, PVC name, and reclaim policy. |
| Volume → no PV, no claimRef, tags name a PVC that no longer exists | Candidate. Still report rather than delete — the tag is provenance, not ownership. |
| Any hop returned `Forbidden`, errored, or found nothing | **UNRESOLVED.** Escalate as unresolved. Never as confirmed-safe. |

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

An imperative `kubectl delete` is right for a confirmed orphaned SeiNode because the object was never in git, so there is no manifest to `git rm`. The same reasoning covers the other non-Git resources below; it never covers anything Flux owns.

**Never delete an EBS volume from this skill.** Even a volume the ownership walk resolved to a dead PVC goes to the platform team: `ec2:DeleteVolume` is outside the engineer's policy, the walk can be wrong, and an EBS delete is unrecoverable. Hand over the volume IDs, sizes, creation times, and the walk's verdict per volume — including every `UNRESOLVED` one, labelled as unresolved. Escalate through `#harbor-onboarding`. Do not report the cleanup as complete while any ID is outstanding.

### The other resources git never owned

Deleting an orphaned SeiNode is the one cleanup with a paved road. The rest of what a workspace PR leaves behind needs its own handling, so nothing in the [what a workspace-repo PR removes](#what-a-workspace-repo-pr-removes) list is left with no next step:

| Resource | Why git never owned it | What to do |
|---|---|---|
| Orphaned validator SeiNode | Controller-generated, then owner-reference stripped | Delete it, per above. Confirm the orphan signature first. |
| SeiNetwork/SeiNode from an escape-hatch direct apply | Applied with `seictl` outside the PR flow | Confirm no workspace-repo manifest names it (`grep -r <name> engineers/<alias>/`). If none, gate on `deletionPolicy` exactly as a Flux-owned network, then `seictl network\|node delete`. If a manifest does exist, it is Flux-owned — use the PR path. |
| `SeiNodeTaskWorkflow` | Never committed to the workspace repo, by Guardrail #9 | A `Complete` workflow is the deliberate audit trail — leave it. Force-delete only a `Failed` workflow holding a node, with the `sei.io/force-delete-workflow` annotation first (`seictl-cli.md`). |
| Bench Job/ConfigMap applied by hand | Ran outside the PR flow | `kubectl delete job\|configmap` by name. Results already in S3 are untouched and are not garbage. |
| Controller-managed PVC with no SeiNode | The controller owns PVC lifecycle; the engineer's Role has no `delete` on PVCs | Escalate with the PVC name and its PV. Do not request the verb. |
| S3 genesis prefixes, bench results | Never Kubernetes objects | Out of scope for teardown. Purging a `<chain-id>/` genesis prefix is a deliberate act that unburns the chain-id; the engineer decides. |

Anything not in this table, or any case where the ownership question stays open, escalates as unresolved rather than getting a guess.

## Halt conditions

Stop and report. Do not auto-remediate.

- **A SeiNetwork in the teardown reads `deletionPolicy: Retain` or empty.** Removing it orphans the validators and leaks their disks. Halt before opening the removal PR; route to [Set it to `Delete`](#set-it-to-delete).
- **The removal PR merged while a SeiNetwork still read `Retain`.** The cascade is gone and no patch restores it. Do not re-apply the SeiNetwork to "reattach" the children — a fresh network under the same chain-id wedges at height 0 on the burned genesis artifacts. Go straight to [Find and clean up already-leaked resources](#find-and-clean-up-already-leaked-resources).
- **`kustomization <alias>` is `NotFound` in `eng-<alias>`.** The engineer's Flux wiring is missing, so no workspace-repo merge reconciles at all. Surface to the platform team; do not create the Kustomization.
- **`lastAppliedRevision` does not reach the merge commit within two reconcile intervals (~10 min).** Read the Ready condition's message (`cluster-inspection-recipes.md` recipe #8). A render error in `engineers/<alias>/kustomization.yaml` — most often a `resources:` entry pointing at the dir that was just removed — blocks every later apply in the namespace, not only this teardown.
- **An object is still `Terminating` after the poll budget.** Report the finalizer and the controller's log line. Do not strip the finalizer to make the check pass.
- **A verification read returned `UNVERIFIED`.** The API call failed, so the teardown state is unknown. Report it as unknown — never as verified-gone, and never as still-present. Re-run once the access problem is fixed; a teardown with an unverified check is not a finished teardown.
- **A `deletionPolicy` patch landed on the live object but git still declares `Retain`.** Flux reverts it, and the removal PR may merge after the revert. Halt and land the policy in git before the removal.
- **An EBS volume's ownership walk did not resolve.** Any hop that returned `Forbidden`, errored, or found nothing leaves the disk unresolved. Escalate it as unresolved with the volume ID; do not present it as confirmed garbage, and do not delete it.
- **Orphaned SeiNodes found in a namespace the engineer does not own.** Cross-tenant cleanup is out of scope. Hand the platform team the namespace and the node names.
- **`aws ec2 describe-volumes` returns `AccessDenied`.** The leak check did not run. Say that, rather than reporting a clean result.
