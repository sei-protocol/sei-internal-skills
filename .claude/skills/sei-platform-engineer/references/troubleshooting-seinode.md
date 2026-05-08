# Troubleshooting SeiNode (manual)

`seictl` doesn't ship a diagnose verb — this file documents the manual `kubectl`-driven flow.

Last verified: 2026-05-05.

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

Once `.status.phase == Failed`, the controller stops reconciling. Action:

```sh
kubectl get seinode <name> -o jsonpath='{.status.conditions[?(@.type=="Ready")].message}'
```

Read the message, decide retry (delete + recreate) or escalate.

PVC retention: deleting a Failed SeiNode does **not** delete its PVC (`sei.io/seinode-finalizer` blocks until manually released). Recreating with the same name reuses existing data.

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
