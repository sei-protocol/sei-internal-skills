# Interim namespace strategy

Why we ship engineer namespaces today with cells-forward labels, even though the personal-cells project is ~1–2 weeks out.

Last verified: 2026-04-26.

## The bridge

Engineers need a place to run benchmarks today without contaminating shared autobake state. The full personal-cells scaffold (quotas, NetworkPolicy, admission policies, Karpenter NodePool isolation) lands later. The interim shape ships only what's needed to be useful, *without* committing to anything cells will overwrite.

## What ships today (per engineer, via `seictl onboard --apply`)

Two side effects, both performed by `seictl onboard`:

**1. Platform repo PR** containing:

```
clusters/harbor/engineers/<alias>/
  kustomization.yaml
  namespace.yaml             # eng-<alias> with cells-forward labels (below)
  bench-seiload-sa.yaml      # ServiceAccount (Pod Identity wired by AWS SDK, not in this PR)
```

**2. AWS resources** created directly via AWS SDK (no Terraform):

- IAM policy `harbor-bench-seiload-eng-<alias>` — per-engineer scoped to `s3://harbor-sei-autobake-results/bench-<alias>-*/`
- IAM role with the policy attached
- EKS Pod Identity association binding `(eng-<alias>, bench-seiload)` to the role

That's it. No quota. No NetworkPolicy. No admission policies. Bare minimum for benchmarks to function.

## Cells-forward labels (the contract)

`namespace.yaml` carries these labels from day 1:

```yaml
labels:
  tide.sei.io/cell-type: personal       # admission policy keys on this when cells land
  tide.sei.io/owner: <alias>             # platform-team can find owner of any resource
  tide.sei.io/managed-by: seictl         # provenance
  app.kubernetes.io/managed-by: flux
```

These labels are the **interface contract** between this skill (Design A) and the personal-cells project (Design B). When cells land:

- Cell admission policies match `namespaceSelector: tide.sei.io/cell-type=personal` to scope mutations and validations
- Per-cell IAM scoping uses `tide.sei.io/owner` as the identity key
- `tide.sei.io/managed-by` lets platform-team distinguish skill-onboarded namespaces from manually-created ones

**Don't change these labels** without coordinating across both projects.

## What changes when cells land

[outline]

The engineer's workflow is unchanged. Their PRs and `seictl bench up` invocations keep working. Cells layer on top via:

- A `_bases/personal-cell/` Kustomize component that the per-engineer kustomization adds as a base
- Cluster-scoped admission policies (Kyverno + ValidatingAdmissionPolicy) selected on the existing namespace label
- A new Karpenter NodePool with `sei.io/workload=engineer-cell` taint, mutated into engineer pods via admission

Engineers don't re-run `seictl onboard` when cells land. The platform-team's PR adds the base reference; Flux applies the new resources; engineers wake up next day with quotas + NetPol active.

## What we explicitly defer

- Auto-suspend of benchmark workloads after duration + grace
- Per-engineer S3 prefix scoping in IAM (everyone shares `bench-seiload-policy` for now)
- Per-engineer ECR image scoping
- Cross-cluster (dev, prod) cell support
- Automated offboarding on engineer departure
