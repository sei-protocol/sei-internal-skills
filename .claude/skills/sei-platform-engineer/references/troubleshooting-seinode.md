# Troubleshooting SeiNode

Phase-by-phase decision tree. The skill's `seictl seinode diagnose` runs an automated version; this file documents the human/agent fallback for cases the automation doesn't cover.

Last verified: 2026-04-26 against sei-k8s-controller `<version-pending>`.

## Decision tree by phase

### Phase: Pending (controller hasn't picked it up)

[outline]

Common causes:

- Controller leader lease unhealthy → `seictl controller inspect`
- Controller's namespace selector excludes the engineer's namespace → CRD created but not reconciled
- RBAC denying the controller's get/watch on engineer namespace → unlikely under cluster-wide watch (which seictl's controller uses)

### Phase: Initializing (task plan running)

[outline]

Most common: failing PlannedTask. Inspect `.status.plan[?state==Failed].lastError`.

| Failed task | Usual cause | Where to look |
|---|---|---|
| `snapshot-restore` | S3 403 (Pod Identity wrong) | seictl init container logs; `aws sts get-caller-identity` from pod |
| `configure-genesis` (retried 180×) | genesis URL missing or ConfigMap not mounted | `.status.plan[?name==configure-genesis].lastError` |
| `discover-peers` (returns 0) | EC2 tag query empty or peer label selector mismatch | `aws ec2 describe-instances` from pod with the same filter |
| `mark-ready` | seid health check timing out | container logs (`kubectl logs <pod> -c seid`) |

### Phase: Running (steady-state issues)

[outline]

Symptoms that show up after Ready:

- Block production stalls — check `.status.conditions[type=Ready].lastTransitionTime`, then seid logs for consensus errors
- HTTPRoute hostname returns 503 — verify Gateway parentRefs, AuthorizationPolicy not denying, `istioctl analyze`
- Pod restart loops — OOMKill, image pull error, init container failure

### Phase: Failed (terminal)

[outline]

Once `.status.phase == Failed`, the controller stops reconciling. Action: read `.status.conditions[type=Ready].message` for cause, decide retry (delete + recreate) or escalate.

PVC retention: deleting a Failed SeiNode does **not** delete its PVC (`sei.io/seinode-finalizer` blocks until manually released). Recreating with the same name reuses existing data.

## Cross-cutting issues

### PVC lifecycle

[outline: finalizer behavior, manual override `kubectl patch seinode ... -p '{"metadata":{"finalizers":[]}}' --type=merge`, when it's safe to do this]

### HTTPRoute hostname unreachable

[outline: parentRefs check, hostname pattern match against gateway's listener, AuthorizationPolicy review, `istioctl analyze -n <ns>`]

### Pod can't reach S3

[outline: Pod Identity association check (TF), token mounted (`/var/run/secrets/eks.amazonaws.com/serviceaccount/`), Cilium egress policy]
