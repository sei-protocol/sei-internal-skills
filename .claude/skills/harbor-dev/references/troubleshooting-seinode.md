# Troubleshooting SeiNode (manual)

`seictl` doesn't ship a diagnose verb — this file documents the manual `kubectl`-driven flow.

## Decision tree by phase

Read `.status.phase` first: `kubectl get seinode <name> -o jsonpath='{.status.phase}'`.

### Phase: Pending (controller hasn't picked it up)

Common causes:

- Controller leader lease unhealthy → `kubectl get lease -n sei-system` and `kubectl get pods -n sei-system`
- Controller pod missing or crashlooping → `kubectl describe pod -n sei-system -l app.kubernetes.io/name=sei-k8s-controller`

If controller looks healthy but the SeiNode is still Pending after a minute, something deeper is wrong; capture controller logs and escalate.

### Phase: Initializing (task plan running)

Most common: failing PlannedTask. Inspect:

```sh
kubectl get seinode <name> -o jsonpath='{.status.plan}' | jq
```

Look for the task with `state=Failed` and read `.lastError`.

| Failed task | Usual cause | Where to look |
|---|---|---|
| `snapshot-restore` | S3 403 (Pod Identity wrong) | `kubectl logs <name>-0 -c seictl` (init container); `kubectl exec <name>-0 -- aws sts get-caller-identity` |
| `configure-genesis` (retried 180×) | Genesis URL missing or ConfigMap not mounted | `.status.plan[?name==configure-genesis].lastError` |
| `discover-peers` (returns 0) | EC2 tag query empty or peer label selector mismatch | `aws ec2 describe-instances --filters Name=tag:<key>,Values=<value>` from your laptop with the same filter |
| `mark-ready` | seid health check timing out | `kubectl logs <name>-0 -c seid` |

### Phase: Running (steady-state issues)

Symptoms after Ready:

- **Block production stalls** — `kubectl get seinode <name> -o jsonpath='{.status.conditions[?(@.type=="Ready")].lastTransitionTime}'`, then `kubectl logs <name>-0 -c seid` for consensus errors.
- **HTTPRoute hostname returns 503** — `kubectl get httproute -n <ns>`, verify `parentRefs` points at the shared Gateway, hostname matches `*.harbor.platform.sei.io`. Run `istioctl analyze -n <ns>`.
- **Pod restart loops** — `kubectl describe pod <name>-0 -n <ns>`. Common: OOMKill, image pull error, init container failure.

### Phase: Failed (terminal)

Once `.status.phase == Failed`, the controller stops reconciling — Failed is terminal, including across controller image upgrades. Recovery is delete-and-recreate so the parent SND rebuilds the SeiNode with the current pod template:

```sh
kubectl get seinode <name> -o jsonpath='{.status.conditions[?(@.type=="Ready")].message}'
# read the message; if structural, recreate:
kubectl delete seinode <name> -n eng-<alias>
```

The SND owner watcher recreates the SeiNode within seconds.

**PVC behavior** — verify before deleting on stateful nodes:
- For **imported** PVCs (`spec.import` set on the SeiNode): the PVC is preserved; the recreated SeiNode reuses existing data.
- For **controller-managed** PVCs (no `spec.import`): the controller's `handleNodeDeletion` path deletes the PVC during teardown. Delete-and-recreate **wipes data**. Safe for ephemeral chains being recreated from genesis; not safe for archive nodes or any chain with state worth preserving.

## SND plan stuck — "plan in progress, skipping SeiNode mutations"

Symptom: child SeiNodes are missing or in an unexpected state, controller logs show `plan in progress, skipping SeiNode mutations` for the SND. Typically caused by an SND-level plan that can't advance (e.g., `assemble-and-upload-genesis` can't find an assembler because child SeiNodes were deleted mid-plan).

Recovery — delete the SND so Flux re-applies it from the workspace repo on next reconcile:

```sh
kubectl delete snd <name> -n eng-<alias>
```

Flux reconciles in ~60s and the controller rebuilds the plan from scratch.

**Only safe with `spec.deletionPolicy: Delete`** (the default). If the SND has `deletionPolicy: Retain`, deleting orphans the child SeiNodes and their networking — the Flux re-apply creates a fresh SND that may collide with the orphaned resources. Check first with `kubectl get snd <name> -o jsonpath='{.spec.deletionPolicy}'`.

## apply-statefulset fails: rollingUpdate not allowed for OnDelete

Symptom: controller logs show

```
applying statefulset: StatefulSet.apps "<name>" is invalid:
  spec.updateStrategy.rollingUpdate: Invalid value: {"Partition":0,"MaxUnavailable":null}:
  only allowed for updateStrategy 'RollingUpdate'
```

Cause: a legacy StatefulSet has a stale `rollingUpdate` field from an earlier controller version that used `type: RollingUpdate`. The current controller sets `type: OnDelete` but doesn't claim ownership of `rollingUpdate`, so SSA leaves the stale field and the merged result is invalid.

Recovery — patch the StatefulSet to drop the stale field:

```sh
kubectl patch sts -n eng-<alias> <name> --type=merge \
  -p='{"spec":{"updateStrategy":{"type":"OnDelete","rollingUpdate":null}}}'
```

Metadata-only patch; pods are not restarted. The controller's next reconcile applies the new template successfully, then `replace-pod` rolls pods normally.

## Cross-cutting issues

### PVC stuck after delete

`sei.io/seinode-finalizer` blocks SeiNode deletion until the controller releases the PVC. If the controller is unhealthy or EBS CSI flaked, the SeiNode sits `Terminating` forever.

Inspection: `kubectl get seinode <name> -o jsonpath='{.metadata.finalizers}'` and controller logs (`kubectl logs -n sei-system -l app.kubernetes.io/name=sei-k8s-controller --tail=100`).

Manual override (only after confirming PVC orphan is acceptable):

```sh
kubectl patch seinode <name> -p '{"metadata":{"finalizers":[]}}' --type=merge
```

### HTTPRoute hostname unreachable

1. `kubectl get httproute <name> -n <ns> -o yaml` — verify `parentRefs` points at the shared Gateway.
2. Hostname must match `*.harbor.platform.sei.io` (the Gateway's listener pattern).
3. Check `AuthorizationPolicy` isn't denying: `kubectl get authorizationpolicy -n <ns>`.
4. Run `istioctl analyze -n <ns>` for cross-cutting Istio issues.

### Pod can't reach S3 / 0 peers discovered

Pod Identity check from inside the pod:

```sh
kubectl exec <name>-0 -c seid -- aws sts get-caller-identity
```

If this fails, the Pod Identity association is missing or wrong. Check via AWS:

```sh
aws eks list-pod-identity-associations --cluster-name harbor --namespace eng-<alias> --profile <chosen>
```

For peer discovery: `kubectl exec <name>-0 -c seid -- aws ec2 describe-instances --filters Name=tag:<key>,Values=<value> --region eu-central-1`. If empty, the tag query is wrong (verify `spec.peers.ec2Tags` in the SeiNode spec) or no instances are tagged.

If a `CiliumNetworkPolicy` is active in the namespace, check `kubectl get ciliumnetworkpolicy -n eng-<alias>` for egress denies.
