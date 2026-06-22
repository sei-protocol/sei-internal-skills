# Cloud-auth (EKS Pod Identity) kit

## 1. What this concern is

Workloads get AWS credentials via **EKS Pod Identity** — the `eks-pod-identity-agent` addon + a per-workload IAM association scoped to (cluster, namespace, ServiceAccount). The generic mental model — **IRSA** (an OIDC `eks.amazonaws.com/role-arn` annotation on the SA) — is the textbook pattern and a strong default *elsewhere*, but **this fleet does not use it**; reaching for an IRSA annotation here is wrong. *Cited:* `terraform/aws/189176372795/<region>/<cell>/*.tf` (per-workload `eks-pod-identity` associations), `.agent/runbooks/cell-bootstrap.md` Appendix A; `sources.md` §irsa (the generic this overrides).

## 2. The pattern (how this fleet does it)

- **The addon + the association.** Every cluster runs `eks-pod-identity-agent`. Each workload that needs AWS gets a `terraform-aws-modules/eks-pod-identity/aws` association binding an IAM role to its (cluster, namespace, ServiceAccount) — defined in the cell's terraform, **not** as an SA annotation. Wired for: sei-k8s-controller, external-dns, cert-manager, karpenter, aws-lb-controller, EBS-CSI, thanos, loki, the flux kustomize-controller. *Cited:* `terraform/aws/189176372795/eu-west-1/prod-euw1/sei-k8s-controller.tf`, `external-dns.tf`.
- **Session-tag scoping.** S3/resource access is narrowed by the session tag `aws:PrincipalTag/kubernetes-namespace` (e.g. harbor per-engineer S3 scoping), not a per-engineer role. *Cited:* `cell-bootstrap.md` (harbor Pod-Identity).
- **external-dns Deny-on-Condition.** The external-dns IAM uses a `ForAnyValue:StringNotLike` deny condition (a load-bearing operator) — don't "simplify" it to an allow. *Cited:* `external-dns.tf`.
- **ECR image pull** is via the node role / standard EKS, images at `189176372795.dkr.ecr.us-east-2.amazonaws.com/sei/*`.

## 3. Anti-patterns / failure modes

- **Reaching for IRSA.** Adding `eks.amazonaws.com/role-arn` to a SA or provisioning an OIDC trust policy. Cue: a `role-arn` SA annotation, an IRSA TF module. Rewrite: a Pod-Identity association in the cell TF (this fleet's mechanism). (The IRSA *concept* is fine knowledge — `sources.md` §irsa — it's just not how this fleet wires it.)
- **Over-broad role.** A role granting `*` or cross-namespace access instead of the per-(cluster,ns,SA) scope. Cue: wildcards / a shared role across workloads. Rewrite: one association per workload, least-privilege actions.
- **Dropping the session-tag / deny-condition.** Removing the `kubernetes-namespace` tag scoping or the external-dns `StringNotLike` deny. Cue: a broadened S3/route53 policy. Rewrite: keep the tag scope / deny condition.
- **Node-role credential bleed / IMDS.** Relying on the node instance profile instead of a scoped association; leaving IMDS reachable. Cue: a workload using node creds. Rewrite: a scoped association; restrict IMDS.

## 4. Review cues

- **Dimension 6 (cloud-identity boundary):** Pod-Identity association per (cluster, ns, SA); no IRSA annotation; session-tag scoping; no node-role bleed/IMDS exposure. *Basis:* profile §3, `sources.md` §irsa/§nsa-cisa.
- **Dimension 1 (security/least-privilege):** the role's actions scoped minimal, not wildcard. *Basis:* `sources.md` §nsa-cisa.

## 5. One-way doors in this concern

- **A Pod-Identity / IAM trust-scope change** is a security boundary — widening a role, a namespace scope, or a session-tag condition is blast-radius-wide; flag for human approval.
- **Switching a workload's auth mechanism** (Pod-Identity ↔ IRSA) is a fleet-convention change, not a per-workload tweak; flag it.
