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

### Set it to `Delete`

**The change must land in git. A `kubectl patch` alone does not survive to merge time.** Flux reconciles `engineers/<alias>/` every 5 minutes against what git declares. The manifest that spun the chain up was rendered from `seictl network apply --dry-run`, which captures the server-defaulted CR, so `deletionPolicy: Retain` is normally written out in the committed file. Flux owns that field, and the next reconcile reverts the patch — typically while the removal PR sits in review. The engineer then merges a teardown they believe is safe, and it orphans the validators anyway. Do not rely on server-side-apply field ownership to keep a patch alive across a reconcile, even where git happens to omit the field.

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

**Path B — stack the removal PR on the policy PR.** Where preparing two changes serially is too slow, write both up front: branch the removal from the policy branch and open its PR as a **draft**. Then merge the policy PR, run the step-2 reads, and only mark the removal ready once both reads say `Delete`. This is still two separately landed changes — the saving is in preparation, not in the ordering. A single PR carrying both the policy edit and the `git rm` is not this path: it merges as one revision, so the policy and the removal reach the cluster in the same reconcile and the ordering never exists.

**The live patch is a repair, and it is not available for a Flux-owned network.** `kubectl patch seinetwork <chain-id> -n eng-<alias> --type=merge -p '{"spec":{"deletionPolicy":"Delete"}}'` is correct in exactly one situation: the SeiNetwork is **not** in the workspace repo at all — an escape-hatch direct apply, or an object already orphaned from an earlier teardown — so no reconcile will revert it. Confirm that with the workspace search in [the other resources git never owned](#the-other-resources-git-never-owned) before relying on it.

**No "read it again just before merging" version of this exists for a Flux-owned network.** A pre-merge read narrows the window; it does not order your read against Flux's reconcile, and the losing sequence needs no unusual timing:

1. Git declares `Retain`. The engineer patches the live object to `Delete`.
2. The pre-merge read returns `Delete`. It is true, and it is already stale.
3. Flux reconciles the existing revision — the one that still declares `Retain` — and restores `Retain`.
4. The removal merges. The SeiNetwork is deleted under `Retain`. The validators are orphaned.

Nothing in that sequence is a mistake by the engineer, which is why the exception is closed rather than gated. For a Flux-owned SeiNetwork the requirement is committed `Delete`, a successful reconcile, and a read-back of **both** git and the live object.

### Render new chains with `Delete` from the start

The trap disappears if the SeiNetwork never carries `Retain`. At render time on a disposable chain, pass:

```sh
seictl network apply <chain-id> --preset genesis-chain --chain-id <chain-id> \
  --image <ref> -n eng-<alias> --dry-run --set spec.deletionPolicy=Delete
```

`--dry-run` runs server-side apply against the apiserver, so a wrong path fails the render rather than the teardown. Keep `Retain` only when the engineer wants a validator's disk preserved for forensics after the network goes away, and say so in the PR body — a retained disk is a cost the engineer is choosing.

### What the `Delete` cascade actually does

With `deletionPolicy: Delete` the chain runs end to end: SeiNetwork deleted → generated validator SeiNodes deleted through their owner references → each SeiNode's finalizer (`sei.io/seinode-finalizer`) deletes the node's data PVC → the storage class's `Delete` reclaim policy releases the EBS volume.

The finalizer **skips an imported PVC** (`spec.dataVolume.import` set on the SeiNode, naming the claim in `.pvcName`). An imported PVC is preserved by design; its disk is not a leak. See the field-path caveat under inventory step 2 before relying on that path in a query.

That finalizer is also why the per-engineer Role carries no `delete` on `persistentvolumeclaims`. The controller owns PVC lifecycle, and PVCs never appear in the workspace repo, so Flux prune never targets them. Do not ask for that verb — it does not fix this bug.

## Procedure: tear down a chain, bench, or comparison

Teardown follows the same PR contract as spinup: render the change, open a PR, let the engineer merge, verify what Flux did. Never `kubectl delete` a Flux-owned CR — the next reconcile re-applies it and the removal PR never lands.

1. **Pre-flight** — the five gates. Halt on first failure.
2. **Inventory what goes away and what stays — by name, before anything is deleted.** The verification in step 8 polls *named* claims, so this step produces those names. It has to run first: once the SeiNodes are gone, nothing in the cluster still records which claims were imported and which the controller managed.

   Save this as `inventory.sh` and run it with `sh inventory.sh`. It aborts on the first API or parse failure, because a partial inventory under-reports what must disappear and then reads as a clean teardown later.

   ```sh
   #!/bin/sh
   set -eu
   # jq's `unique` sorts by codepoint; `comm` assumes its input is sorted the way
   # the current locale collates, and a locale that ignores punctuation orders
   # hyphenated claim names differently. Pin both to codepoint order.
   export LC_ALL=C
   ALIAS=<alias>; CHAIN=<chain-id>
   [ -n "$ALIAS" ] && [ -n "$CHAIN" ] || { echo 'inventory: ALIAS and CHAIN are required'; exit 2; }
   INV=./teardown-inventory-$CHAIN

   # A FRESH directory per run. Reusing one leaves a previous run's completeness
   # certificate in place, and `set -e` exits on the first failed read below —
   # before anything invalidates it. The verifier would then accept a stale OK
   # sitting beside half-refreshed lists.
   rm -rf "$INV"
   mkdir -p "$INV"
   K="kubectl --context harbor -n eng-$ALIAS"

   # Raw reads, each REDIRECTED to a file rather than piped. In a POSIX shell
   # `kubectl ... | jq ...` exits with jq's status, so a Forbidden from kubectl
   # would pass through as success — the same defect this file exists to prevent.
   #
   # No completeness certificate is written until the very end. Until then the
   # file is simply absent, which read_inventory reports as UNVERIFIED — so an
   # abort at any point below leaves the verifier refusing to pass, with no
   # window in which a stale certificate could authorize anything.
   $K get seinetwork,seinode -l "sei.io/seinetwork=$CHAIN" \
     -o custom-columns='KIND:.kind,NAME:.metadata.name,ROLE:.metadata.labels.sei\.io/role,PHASE:.status.phase' \
     > "$INV/crs.txt"
   $K get seinode -l "sei.io/seinetwork=$CHAIN" -o json > "$INV/nodes.json"
   $K get pods    -l "sei.io/seinetwork=$CHAIN" -o json > "$INV/pods.json"
   $K get pvc                                   -o json > "$INV/pvcs.json"

   # Imported claims — PRESERVED by design. `unique` sorts, which comm needs.
   # Every jq call is a single command with a redirect: in a pipeline its
   # status would be masked, and a parse failure would look like an empty list.
   jq -r '[ .items[] | select(.spec.dataVolume.import.pvcName != null)
            | .spec.dataVolume.import.pvcName ] | unique | .[]' \
     "$INV/nodes.json" > "$INV/imported-claims.txt"

   # node -> claim, attributed through the pod that DECLARES the volume.
   # Pod phase is deliberately not consulted: a Pending pod still declares its
   # volumes, and requiring Running would drop exactly the nodes most likely
   # to be leaking.
   jq -r '[ .items[] as $p
            | ($p.metadata.ownerReferences[0].name // "") as $owner
            | $p.spec.volumes[]? | select(.persistentVolumeClaim)
            | { node: $owner, claim: .persistentVolumeClaim.claimName } ]
          | unique | .[] | "\(.node)\t\(.claim)"' \
     "$INV/pods.json" > "$INV/node-claims.tsv"

   # Each transformation is its own command. In a POSIX shell `cut … | sort -u`
   # exits with sort's status, so a failed cut yields a successful EMPTY claim
   # list — and the inventory would then certify itself complete while omitting
   # every managed claim. set -e does not catch a non-final pipeline failure.
   cut -f2 "$INV/node-claims.tsv" > "$INV/all-claims.raw"
   sort -u "$INV/all-claims.raw" > "$INV/all-claims.txt"

   # Controller-managed = attributed minus imported. These MUST disappear.
   comm -23 "$INV/all-claims.txt" "$INV/imported-claims.txt" > "$INV/managed-claims.txt"

   # ---- completeness check 1: every node must resolve to storage ----------
   # A node resolves if it imports a claim, or if a pod attributed to it
   # declares one. A node that resolves to NEITHER contributes nothing to the
   # lists above — and a provisioned PVC with no pod is precisely the leak
   # this document exists to catch, so it must never pass silently.
   jq -r '[ .items[].metadata.name ] | unique | .[]' \
     "$INV/nodes.json" > "$INV/nodes.txt"
   jq -r '[ .items[] | select(.spec.dataVolume.import.pvcName != null)
            | .metadata.name ] | unique | .[]' \
     "$INV/nodes.json" > "$INV/imported-nodes.txt"
   # Drop the empty owner field: a pod with no ownerReferences still yields its
   # claim above, but attributes to no node — so its node stays unresolved.
   # Three separate commands, same reason as above. grep's exit 1 (nothing
   # matched) is a legitimate outcome here; 2 or more is a real failure.
   cut -f1 "$INV/node-claims.tsv" > "$INV/owners.raw"
   if grep -v '^$' "$INV/owners.raw" > "$INV/owners.nonempty"; then :; else
     gs=$?
     [ "$gs" -eq 1 ] || { echo 'inventory: owner filter failed'; exit 2; }
   fi
   sort -u "$INV/owners.nonempty" > "$INV/nodes-with-claims.txt"
   sort -u "$INV/imported-nodes.txt" "$INV/nodes-with-claims.txt" > "$INV/resolved-nodes.txt"
   comm -23 "$INV/nodes.txt" "$INV/resolved-nodes.txt" > "$INV/unresolved-nodes.txt"

   # ---- completeness check 2: namespace claims nobody claimed -------------
   # Not this chain's business to delete, but worth surfacing: a claim here is
   # either another chain's or already leaked.
   jq -r '[ .items[].metadata.name ] | unique | .[]' "$INV/pvcs.json" > "$INV/ns-claims.txt"
   sort -u "$INV/all-claims.txt" "$INV/imported-claims.txt" > "$INV/attributed.txt"
   comm -23 "$INV/ns-claims.txt" "$INV/attributed.txt" > "$INV/unattributed-claims.txt"

   printf '== must disappear (controller-managed) ==\n'; cat "$INV/managed-claims.txt"
   printf '== must survive (imported) ==\n';             cat "$INV/imported-claims.txt"
   if [ -s "$INV/unattributed-claims.txt" ]; then
     printf '== unattributed claims in this namespace (leak sweep, not this teardown) ==\n'
     cat "$INV/unattributed-claims.txt"
   fi

   if [ -s "$INV/unresolved-nodes.txt" ]; then
     printf 'INVENTORY INCOMPLETE — SeiNodes that resolve to no storage\n'
     cat "$INV/unresolved-nodes.txt"
     printf 'No certificate written; the verifier will report UNVERIFIED.\n'
     exit 2
   fi

   # The certificate names the target it certifies. A complete inventory for a
   # DIFFERENT namespace or chain must not authorize this one, and verify_teardown
   # compares this string against the target it was called with.
   printf 'OK eng-%s sei.io/seinetwork=%s\n' "$ALIAS" "$CHAIN" > "$INV/status"
   ```

   Claim names come from the **pods' own `spec.volumes[].persistentVolumeClaim.claimName`**, not from a guessed naming rule — the controller owns how it names a generated claim, and a rule inferred here would desync the moment it changes.

   **A node with no pod resolves to nothing, and that is the leak case, not a nuisance.** The controller reconciles each SeiNode into a StatefulSet (`seinode-crd.md`), so a node whose StatefulSet has no pod — scaled down, unschedulable, evicted — still has its PVC and its EBS volume. The old version of this inventory dropped that node's claim silently and the teardown then verified clean. Check 1 makes the gap executable: the node lands in `unresolved-nodes.txt`, the script exits non-zero, `status` stays `UNRESOLVED`, and the verifier forces `UNVERIFIED`.

   > **Attribution caveat.** A pod is attributed to a node by its **first owner reference's name matching the SeiNode name**. `seinode-crd.md` documents the one-StatefulSet-per-SeiNode shape but not the name the controller gives it, so this is a convention, not a contract. If it does not hold, the node lands in `unresolved-nodes.txt` and the run stops — the failure direction is safe. Confirm with `kubectl get pod <pod> -n eng-<alias> -o jsonpath='{.metadata.ownerReferences[0].name}'` before assuming an empty `unresolved-nodes.txt` means full coverage.

   > **Field-path caveat.** `.spec.dataVolume.import.pvcName` is read from `sei-protocol/sei-k8s-controller` `api/v1alpha1/seinode_types.go` on **repo main** (`DataVolume` → nested `Import` → `PVCName`), not from the CRD deployed on harbor. Confirm against the live cluster before trusting an empty imported list — `kubectl explain seinode.spec.dataVolume.import` — and if the deployed CRD disagrees, **the CRD wins**. An empty `imported-claims.txt` from a wrong path is indistinguishable from a chain that genuinely imports nothing, and it silently reclassifies a preserved claim as one that must disappear.
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

A `Forbidden` on `--with-source` **may** mean the `GitRepository` the Kustomization references sits outside `eng-<alias>`, beyond the engineer's namespace-scoped Role. It may equally be an expired session, a missing EKS access entry, or a Role that never carried the Flux verbs. Read the message before concluding which. Dropping `--with-source` helps only the first cause: reconciling the Kustomization alone applies the revision the source has already fetched, and the source polls on its own schedule. It repairs nothing for an expired session, a missing access entry, or a Role without the Flux verbs — those fail the same way with or without the flag. **The fallback has to succeed on its own terms.** If the reconcile without `--with-source` also fails, you have no reconcile at all: stop, fix the access problem, and do not proceed to the disappearance check, whose result would be `UNVERIFIED` anyway.

### Confirm the resources disappeared

A successful reconcile says Flux applied the change. It does not say the objects are gone. Deletion is asynchronous and finalizers hold objects in `Terminating` while the controller releases their PVCs, so poll instead of asserting once.

**Three outcomes, and they are not interchangeable:**

| Outcome | Meaning | What to report |
|---|---|---|
| `GONE` | The API answered and matched nothing. | Teardown verified for these objects. |
| `PRESENT` | The API answered and objects remain at the deadline. | Not torn down. Read the finalizers below. |
| `UNVERIFIED` | The API call failed — `Forbidden`, expired credential, connection error. | **Teardown not confirmed.** Say the check could not run. |

**A failed API read is never a pass.** A `Forbidden` or a dropped connection returns zero lines, and a check that counts lines without reading the exit status prints "gone" precisely when it cannot see the cluster.

**And a later success must never overwrite an earlier failure.** Printing `UNVERIFIED` is not enough on its own: a `break` out of a loop, or a bare call whose return code nobody reads, still leaves the block exiting 0. A human sees the warning; a wrapper script or an agent reading `$?` sees success. Every check records its outcome into a running verdict, and the worst one wins.

**All of that lives in one function.** `verify_teardown` in `cluster-inspection-recipes.md` recipe #9 carries the completeness gate, the checked list reads, the empty-list branches, the polls, and the aggregation. This file calls it and reads its return value — there is deliberately no verification shell here to drift out of step with the library:

```sh
. ./verify-lib.sh          # the library block from recipe #9

rc=0
verify_teardown eng-<alias> seinetwork,seinode,pod \
  "sei.io/seinetwork=<chain-id>" ./teardown-inventory-<chain-id> || rc=$?
record "$rc"
exit "$VERDICT"
```

That is the whole verification step. **If you find yourself writing a `poll_gone` line in this file, stop** — a second copy of the orchestration is how this exact defect survived four review rounds, reappearing one layer up each time: duplicated poll bodies, then the caller chain, then the arguments feeding the callers, then a canonical caller that bypassed the fixed library entirely while the paragraph above it said not to re-implement.

The empty-list cases are handled inside the function, as code rather than as advice here: an empty `managed-claims.txt` takes a `NOTE` branch instead of calling `poll_gone` with no names, because `kubectl get persistentvolumeclaim` with no arguments lists the whole namespace — the sweep this design exists to avoid.

**Zero PVCs is the wrong expectation, and a namespace sweep is the wrong check.** The SeiNode finalizer deliberately skips an imported claim, so those survive by design, and other chains' claims are none of this teardown's business. Both make a sweep report `PRESENT` after a correct teardown — a false alarm that trains the reader to ignore the check. The expected end state is precise: every controller-managed claim of this chain gone, every imported claim still present.

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

1. **List what is there, then inventory each chain properly.** The display read below is an overview, not an inventory — it produces none of the named-claim files the verifier consumes, so it cannot stand in for step 2 of the per-chain procedure:

   ```sh
   kubectl --context harbor get seinetwork,seinode,job,pvc -n eng-<alias>
   ```

   Then discover the chain-ids **with the discovery's own status checked**. Piping `kubectl` into `sort` exits with sort's status, so a `Forbidden` becomes a successful empty list — and "no chains found" then reads as "nothing to do", which is the whole defect class this document exists to close:

   ```sh
   . ./verify-lib.sh          # recipe #9 — provides ERRF, _note_stderr, record, VERDICT
   VERDICT=0

   if raw=$(kubectl --context harbor get seinode -n eng-<alias> \
              -o jsonpath='{range .items[*]}{.metadata.labels.sei\.io/seinetwork}{"\n"}{end}' 2>"$ERRF")
   then
     chains=$(printf '%s\n' "$raw" | grep -v '^$' | sort -u || true)
   else
     echo 'UNVERIFIED: chain discovery failed — the namespace inventory is unknown'
     _note_stderr; record 2; chains=''
   fi
   ```

   Run `inventory.sh` **once per chain-id**, each writing its own `./teardown-inventory-<chain-id>`. A namespace usually holds more than one chain, and a single sweep cannot tell one chain's controller-managed claim from another's. Any chain whose `inventory.sh` exits non-zero writes no certificate, and its verification then reports `UNVERIFIED` — emptying a namespace on an incomplete inventory is how a leak becomes invisible. Claims no chain attributes land in each run's `unattributed-claims.txt`; take those to the leak sweep in step 5, not to a delete.
2. For every SeiNetwork in the inventory, run the `deletionPolicy` gate in [The `deletionPolicy: Retain` trap](#the-deletionpolicy-retain-trap). One `Retain` network is enough to leak a set of disks.
3. `git rm -r` every task dir under `engineers/<alias>/`, and reduce `engineers/<alias>/kustomization.yaml` to `resources: []`. Keep that file: deleting it makes the Flux Kustomization fail reconcile with `path not found`, which is the same breakage the onboarding scaffolding PR exists to prevent.
4. Open the PR, merge, then verify **every chain, retaining the worst result**. One `VERDICT` spans the whole namespace, so a clean second chain cannot cover an unverified first one:

   ```sh
   # Same shell as step 1 — the library is already sourced and $VERDICT already
   # carries a 2 if chain discovery failed.
   for c in $chains; do
     rc=0
     verify_teardown eng-<alias> seinetwork,seinode,pod \
       "sei.io/seinetwork=$c" "./teardown-inventory-$c" || rc=$?
     record "$rc"
   done

   # Anything left that carries no chain label at all — an escape-hatch apply,
   # or an orphan whose labels were stripped. No inventory applies, so `-`.
   rc=0
   verify_teardown eng-<alias> seinetwork,seinode,pod "" - || rc=$?
   record "$rc"

   case "$VERDICT" in
     0) echo 'NAMESPACE EMPTIED — every chain verified' ;;
     1) echo 'NAMESPACE NOT EMPTY — objects remain in at least one chain' ;;
     2) echo 'NAMESPACE UNVERIFIED — at least one check could not run' ;;
   esac
   exit "$VERDICT"
   ```

   An empty `$chains` after a **successful** discovery is legitimate — the namespace has no labelled chains — and the unlabelled sweep still runs. An empty `$chains` after a **failed** discovery already recorded `2`, so the loop running zero times cannot pass. Do not substitute a namespace-wide PVC poll anywhere here: it matches imported and unattributed claims too, so it reports `PRESENT` after a correct teardown.
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

Treat both as **candidate** signals, then resolve ownership before calling anything garbage.

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

Every hop separates the API call from the parse, and checks both. A `kubectl … | jq …` pipeline exits with `jq`'s status, so a `Forbidden` would read as "no match found" — which on this walk is the difference between *unowned* and *could not look*.

```sh
# ---- hop 1: volume ID → PV. The CSI volume handle is the EBS volume ID. -----
raw=$(kubectl --context harbor get pv -o json 2>&1) || {
  printf 'UNRESOLVED: PV list failed — cannot tell unowned from unreadable\n%s\n' "$raw"
  exit 2; }
pv=$(printf '%s' "$raw" | jq -r --arg v '<vol-id>' '.items[]
      | select(.spec.csi.volumeHandle == $v)
      | "\(.metadata.name)\t\(.status.phase)\t\(.spec.persistentVolumeReclaimPolicy)\t\(.spec.claimRef.namespace // "-")\t\(.spec.claimRef.name // "-")"') || {
  printf 'UNRESOLVED: PV parse failed\n'; exit 2; }
[ -n "$pv" ] || echo 'no PV references this volume — see the verdict table'
printf '%s\n' "$pv"
```

**Hop 2 is a scope gate, not just a lookup.** `kubectl get pv` is cluster-scoped, so the `claimRef` it returns can name *any* namespace. Assert it is this tenant's before inspecting further — a claim in another namespace is another tenant's disk, and this skill does not investigate those.

```sh
# ---- hop 2: claimRef → PVC, inside this tenant only ------------------------
claim_ns=<claim-namespace-from-hop-1>; claim=<claim-name-from-hop-1>
if [ "$claim_ns" != "eng-<alias>" ]; then
  printf 'OUT OF SCOPE: volume claimed by %s/%s — escalate, do not inspect\n' "$claim_ns" "$claim"
  exit 2
fi
kubectl --context harbor get pvc "$claim" -n eng-<alias> --ignore-not-found -o name \
  || { echo 'UNRESOLVED: PVC read failed'; exit 2; }
```

```sh
# ---- hop 3: PVC → the pod that mounts it → that pod's owner ---------------
# This is the hop that names WHICH node references the candidate. Listing every
# SeiNode in the namespace does not establish a relationship to this claim.
raw=$(kubectl --context harbor get pods -n eng-<alias> -o json 2>&1) || {
  printf 'UNRESOLVED: pod list failed\n%s\n' "$raw"; exit 2; }
if users=$(printf '%s' "$raw" | jq -r --arg c "$claim" '.items[] as $p
  | $p.spec.volumes[]? | select(.persistentVolumeClaim.claimName == $c)
  | "pod=\($p.metadata.name)\towner=\($p.metadata.ownerReferences[0].kind // "-")/\($p.metadata.ownerReferences[0].name // "-")\tnetwork=\($p.metadata.labels["sei.io/seinetwork"] // "-")"')
then :; else
  echo 'UNRESOLVED: pod parse failed — cannot tell "no pod mounts it" from "could not look"'
  exit 2
fi
if [ -z "$users" ]; then
  echo 'UNRESOLVED: no pod currently mounts this claim — a stopped workload looks identical'
  exit 2
fi
printf '%s\n' "$users"
```

An empty hop-3 result means **no pod currently mounts the claim**. That is not evidence the claim is unwanted — it is exactly the stopped-workload state that made the volume read `available` in the first place. Treat it as unresolved.

The owner reference names the StatefulSet the controller created for the node, not the SeiNode directly. Map it back to a SeiNode by name and confirm that node is a **confirmed orphan** by the signature in [Orphaned SeiNodes](#orphaned-seinodes). If you cannot make that link, the hop is unresolved.

| What the walk found | Verdict |
|---|---|
| Volume → PV → PVC → pod → a SeiNode confirmed orphaned by the signature | Reclaimable. Delete the **SeiNode**, not the volume — see below. |
| Volume → PV → PVC → pod → a live, wanted workload | **Not garbage.** Leave it. `available` only meant the workload was stopped. |
| Volume → PV → PVC whose claim is gone, PV `Released` | **Unresolved candidate.** Platform review required. Report the PV, PVC name, and reclaim policy; do not act on it here. |
| Volume → no PV, no claimRef, tags name a PVC that no longer exists | **Unresolved candidate.** The tag is provenance, not ownership. Platform review required. |
| PVC exists but no pod mounts it | **Unresolved.** A stopped workload looks identical to an abandoned claim from here. |
| `claimRef` names a namespace other than `eng-<alias>` | **Out of scope.** Another tenant's disk. Escalate; do not inspect. |
| Any hop returned `Forbidden`, errored, or found nothing | **UNRESOLVED.** Escalate as unresolved. Never as confirmed-safe. |

Only the first two rows are verdicts. Every other row is an escalation, and the platform team is told which row it came from.

`kubectl get pv` is cluster-scoped and the per-engineer Role is namespaced, so `Forbidden` at hop 1 is the **normal** case for an engineer — an unresolved result, not a clean one. When it happens, hand the volume IDs to the platform team and let them walk the chain; do not substitute the tag data for the walk.

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
| SeiNetwork/SeiNode from an escape-hatch direct apply | Applied with `seictl` outside the PR flow | Prove no workspace manifest names it — see the ownership search below — then gate on `deletionPolicy` exactly as a Flux-owned network, then delete with `kubectl --context harbor delete seinetwork\|seinode <name> -n eng-<alias>`. **Not `seictl delete`** — see the context note below. If a manifest does exist, it is Flux-owned: use the PR path. |
| `SeiNodeTaskWorkflow` | Never committed to the workspace repo, by Guardrail #9 | A `Complete` workflow is the deliberate audit trail — leave it. Force-delete only a `Failed` workflow holding a node, with the `sei.io/force-delete-workflow` annotation first (`seictl-cli.md`). |
| Bench Job/ConfigMap applied by hand | Ran outside the PR flow | Same ownership search first, then `kubectl --context harbor delete job <name> -n eng-<alias>` / `… delete configmap <name> -n eng-<alias>`, by name. Results already in S3 are untouched and are not garbage. |
| Controller-managed PVC with no SeiNode | The controller owns PVC lifecycle; the engineer's Role has no `delete` on PVCs | Escalate with the PVC name and its PV. Do not request the verb. |
| S3 genesis prefixes, bench results | Never Kubernetes objects | Out of scope for teardown. Purging a `<chain-id>/` genesis prefix is a deliberate act that unburns the chain-id; the engineer decides. |

Anything not in this table, or any case where the ownership question stays open, escalates as unresolved rather than getting a guess.

**Every namespace-scoped command above names its namespace, and every destructive one names its context.** An unqualified `delete` deletes wherever the shell happens to point, and that is not a typo you can retry — it is a delete in the wrong place.

**This is why the direct deletes above use `kubectl`, not `seictl`.** `seictl`'s common flags are `--kubeconfig` and `-n/--namespace` only (`seictl-cli.md` → *Common flags on every verb*) — **there is no `--context`**, and the namespace falls back to the kubeconfig context's default. A `seictl delete` therefore cannot pin the cluster on its own command line; writing "(harbor context)" beside it states an intention the command does not enforce. `kubectl --context harbor delete <kind> <name> -n eng-<alias>` pins both on the line that does the deleting, and issues the same Delete against the same CR (`seictl-cli.md` → `seictl network|node delete`).

**No `seictl` alternative for network or node deletion belongs here, guarded or otherwise.** A `kubectl config current-context` check reads mutable state rather than pinning the config the delete then consumes, so it leaves a window between the check and the call — and it buys nothing, because `kubectl --context harbor delete` does the same deletion against the same CR with no window at all.

Should some genuinely `seictl`-only destructive verb ever need documenting, the pin belongs on the invocation: hand it a kubeconfig that contains the harbor cluster and nothing else, via `--kubeconfig <harbor-only-file>` (a documented `seictl` flag). A file that cannot name another cluster cannot select one. Do not substitute a current-context check.

### The ownership search that authorizes a direct delete

Before deleting anything imperatively, prove the object is **not** in the workspace repo. A search that fails must never read as "no manifest found" — `grep` exits 1 for no match and 2 or more for an error, and an unreadable or stale clone produces the same empty output as a genuinely absent manifest.

```sh
# Run inside a FRESH clone of harbor-engineering-workspace at origin/main.
# A stale working copy can miss a manifest somebody merged an hour ago.
git -C <workspace-clone> fetch origin main && git -C <workspace-clone> checkout -q origin/main \
  || { echo 'UNRESOLVED: cannot refresh the workspace clone — do not delete'; exit 2; }

grep -rn -- '<object-name>' <workspace-clone>/engineers/<alias>/ && gs=0 || gs=$?
case "$gs" in
  0) echo 'FLUX-OWNED: a manifest names it — use the PR path, do not delete'; exit 1 ;;
  1) echo 'NOT IN GIT: safe to consider for a direct delete, after the other gates'; exit 0 ;;
  *) echo 'UNRESOLVED: the search itself failed — do not delete'; exit 2 ;;
esac
```

**Only exit status 0 from this block authorizes a direct delete.** Every branch used to end in a successful `echo`, so the block's own status was 0 whatever it found — a scripted caller could not tell "not in git" from "the search failed", which is the same class of defect as counting lines without reading an exit status. `1` routes to the PR path; `2` means the question was never answered.

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
