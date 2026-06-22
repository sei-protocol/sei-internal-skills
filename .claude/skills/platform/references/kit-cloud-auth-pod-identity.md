# Cloud-auth (EKS Pod Identity) kit

## 1. What this concern is

Workloads get AWS credentials via **EKS Pod Identity** — the `eks-pod-identity-agent` addon + a per-workload IAM association scoped to (cluster, namespace, ServiceAccount). The generic mental model — **IRSA** (an OIDC `eks.amazonaws.com/role-arn` annotation on the SA) — is the textbook pattern and a strong default *elsewhere*; **here Pod Identity is the default**, so reaching for IRSA *by reflex* is wrong. But IRSA is **not absent**: it is the deliberate, documented escape hatch for a workload whose SDK can't consume the Pod-Identity credential endpoint (see §2) — so a `role-arn` annotation **with that rationale is correct**, not a defect. *Cited:* `terraform/aws/189176372795/<region>/<cell>/*.tf` (per-workload `eks-pod-identity` associations), `terraform/aws/189176372795/eu-central-1/prod/state-size-analyzer.tf:1-9` (the IRSA exception), `.agent/runbooks/cell-bootstrap.md` Appendix A; `sources.md` §irsa (the generic this defaults away from).

## 2. The pattern (how this fleet does it)

- **The addon + the association.** Every cluster runs `eks-pod-identity-agent`. Each workload that needs AWS gets a `terraform-aws-modules/eks-pod-identity/aws` association binding an IAM role to its (cluster, namespace, ServiceAccount) — defined in the cell's terraform, **not** as an SA annotation. Wired for: sei-k8s-controller, external-dns, cert-manager, karpenter, aws-lb-controller, EBS-CSI, thanos, loki, the flux kustomize-controller. *Cited:* `terraform/aws/189176372795/eu-west-1/prod-euw1/sei-k8s-controller.tf`, `external-dns.tf`.
- **Session-tag scoping.** S3/resource access is narrowed by the session tag `aws:PrincipalTag/kubernetes-namespace` (e.g. harbor per-engineer S3 scoping), not a per-engineer role. *Cited:* `cell-bootstrap.md` (harbor Pod-Identity).
- **external-dns Deny-on-Condition.** The external-dns IAM uses a `ForAnyValue:StringNotLike` deny condition (a load-bearing operator) — don't "simplify" it to an allow. *Cited:* `external-dns.tf`.
- **ECR image pull** is via the node role / standard EKS, images at `189176372795.dkr.ecr.us-east-2.amazonaws.com/sei/*`.
- **The IRSA exception (legitimate, not an anti-pattern).** Prod `pacific-1` runs two CronJobs (`state-size-analyzer`, `archive-volume-snapshot`) on IRSA — a dedicated IAM role with an `AssumeRoleWithWebIdentity` trust policy (OIDC `sub` = `system:serviceaccount:pacific-1:<sa>`), with a `role-arn` SA annotation on the CronJob. **Why:** `seidb` bundles an `aws-sdk-go` too old to consume the Pod-Identity endpoint (`169.254.170.23`) — it rejects the non-loopback container-credentials host and finds no providers; IRSA's web-identity flow works on old SDKs. The TF carries the rationale comment. When you see a `role-arn` annotation, **read for that justification before flagging** — an old-SDK workload on IRSA is the right call. *Cited:* `terraform/aws/189176372795/eu-central-1/prod/state-size-analyzer.tf:1-9` + `archive-volume-snapshot.tf`, `clusters/prod/pacific-1/state-size-analyzer-cron.yaml`.

## 3. Anti-patterns / failure modes

- **Reaching for IRSA *by reflex* (without the old-SDK justification).** Adding `eks.amazonaws.com/role-arn` to a SA / provisioning an OIDC trust policy *because IRSA is the habit*, for a workload whose SDK can consume the Pod-Identity endpoint. Cue: a `role-arn` SA annotation **with no rationale comment**, on a workload that isn't the old-SDK exception. Rewrite: a Pod-Identity association in the cell TF (this fleet's default). **Do not flag the documented exception** — a `role-arn` annotation that *cites* the old-SDK reason (§2) is correct; rewriting it to Pod Identity would break the CronJob.
- **Over-broad role.** A role granting `*` or cross-namespace access instead of the per-(cluster,ns,SA) scope. Cue: wildcards / a shared role across workloads. Rewrite: one association per workload, least-privilege actions.
- **Dropping the session-tag / deny-condition.** Removing the `kubernetes-namespace` tag scoping or the external-dns `StringNotLike` deny. Cue: a broadened S3/route53 policy. Rewrite: keep the tag scope / deny condition.
- **Node-role credential bleed / IMDS.** Relying on the node instance profile instead of a scoped association; leaving IMDS reachable. Cue: a workload using node creds. Rewrite: a scoped association; restrict IMDS.

## 4. Review cues

- **Dimension 6 (cloud-identity boundary):** Pod-Identity association per (cluster, ns, SA) as the default; a `role-arn` IRSA annotation is OK **only** with the documented old-SDK justification (§2) — flag an *unjustified* one, ratify a justified one; session-tag scoping; no node-role bleed/IMDS exposure. *Basis:* profile §3, `sources.md` §irsa/§nsa-cisa.
- **Dimension 1 (security/least-privilege):** the role's actions scoped minimal, not wildcard. *Basis:* `sources.md` §nsa-cisa.

## 5. One-way doors in this concern

- **A Pod-Identity / IAM trust-scope change** is a security boundary — widening a role, a namespace scope, or a session-tag condition is blast-radius-wide; flag for human approval.
- **Switching a workload's auth mechanism** (Pod-Identity ↔ IRSA) is a fleet-convention change, not a per-workload tweak; flag it.
