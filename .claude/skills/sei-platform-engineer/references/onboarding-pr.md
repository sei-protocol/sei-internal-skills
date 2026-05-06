# Onboarding PR (the one-time tenant registration)

A new engineer's onboarding is a single PR against `sei-protocol/platform` adding one file:

```
clusters/harbor/engineers/<alias>/kustomization.yaml
```

The file references the shared base layer (`../base`) and uses a configMapGenerator + replacements block to template the `tenant` placeholder in the base into the engineer's `<alias>`. When the PR merges, Flux reconciles the tenant in ~60s and the engineer's namespace, RBAC, workload service account, and Flux watcher all come online together.

Last verified: 2026-05-05 against sei-protocol/platform#427 (the fromtherain pilot — the canonical example) and the multi-tenancy design in `docs/designs/harbor-multi-tenancy-lld.md`.

## The canonical example: fromtherain (PR #427)

`clusters/harbor/engineers/fromtherain/kustomization.yaml`:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../base

configMapGenerator:
  - name: tenant-info
    literals:
      - alias=fromtherain
    options:
      annotations:
        config.kubernetes.io/local-config: "true"
      disableNameSuffixHash: true

replacements:
  - source:
      kind: ConfigMap
      name: tenant-info
      fieldPath: data.alias
    targets:
      - select:
          kind: ServiceAccount
        reject:
          - name: workload-service-account
        fieldPaths:
          - metadata.name
      - select:
          kind: Role
        fieldPaths:
          - metadata.name
      - select:
          kind: RoleBinding
        fieldPaths:
          - metadata.name
          - subjects.0.name
          - roleRef.name
      - select:
          kind: Kustomization
          group: kustomize.toolkit.fluxcd.io
        fieldPaths:
          - metadata.name
          - spec.serviceAccountName
      - select:
          kind: Namespace
        fieldPaths:
          - metadata.labels.[tide.sei.io/owner]
          - metadata.labels.[toolkit.fluxcd.io/tenant]
      - select:
          kind: ServiceAccount
          name: workload-service-account
        fieldPaths:
          - metadata.labels.[tide.sei.io/owner]
  - source:
      kind: ConfigMap
      name: tenant-info
      fieldPath: data.alias
    targets:
      - select:
          kind: Namespace
        fieldPaths:
          - metadata.name
          - metadata.labels.[app.kubernetes.io/name]
        options:
          delimiter: "-"
          index: 1
      - select:
          kind: ServiceAccount
        fieldPaths:
          - metadata.namespace
        options:
          delimiter: "-"
          index: 1
      - select:
          kind: Role
        fieldPaths:
          - metadata.namespace
        options:
          delimiter: "-"
          index: 1
      - select:
          kind: RoleBinding
        fieldPaths:
          - metadata.namespace
          - subjects.0.namespace
        options:
          delimiter: "-"
          index: 1
      - select:
          kind: Kustomization
          group: kustomize.toolkit.fluxcd.io
        fieldPaths:
          - metadata.namespace
          - spec.targetNamespace
        options:
          delimiter: "-"
          index: 1
      - select:
          kind: Kustomization
          group: kustomize.toolkit.fluxcd.io
        fieldPaths:
          - spec.path
        options:
          delimiter: "/"
          index: 2
```

Plus an aggregator one level up at `clusters/harbor/engineers/kustomization.yaml` listing the new tenant directory:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - fromtherain
```

The PR adds the tenant's directory + appends one line to the aggregator. Two files touched.

## What the base layer ships

`clusters/harbor/engineers/base/`:

| File | Provides |
|---|---|
| `kustomization.yaml` | Aggregates the rest. |
| `namespace.yaml` | `eng-tenant` Namespace with cells-forward labels (`tide.sei.io/cell-type=personal`, `tide.sei.io/owner=tenant`, `toolkit.fluxcd.io/tenant=tenant`, `app.kubernetes.io/managed-by=flux`, `app.kubernetes.io/name=eng-tenant`). |
| `rbac.yaml` | ServiceAccount `tenant`, namespace-scoped Role with the tenant's permission surface (snd/sn CRUD, derived resources read-only, events/configmaps/jobs CRUD, Flux Kustomization+GitRepository read+patch), RoleBinding `tenant`. |
| `sync.yaml` | Flux `Kustomization` `tenant` watching `harbor-engineering-workspace` GitRepository at `./engineers/tenant`. The reconciler runs as the `tenant` ServiceAccount; cross-namespace writes are denied by RBAC. |
| `workload-service-account.yaml` | `workload-service-account` ServiceAccount with `tide.sei.io/cell-type=personal` and `tide.sei.io/owner=tenant`. Used by in-namespace workloads (SeiNode pods, Jobs). |

The `tenant` placeholder is what the per-tenant overlay's `replacements:` substitutes for the engineer's alias. The base is `eng-tenant` everywhere; the overlay rewrites it to `eng-<alias>`.

## What reconciles when the PR merges

Within ~60s of merge:

| Resource | Where | Notes |
|---|---|---|
| `Namespace eng-<alias>` | cluster-scoped | Cells-forward labels, tagged for Flux multi-tenancy. |
| `ServiceAccount <alias>` | `eng-<alias>` | The Flux reconciler identity for this tenant. |
| `Role <alias>` + `RoleBinding <alias>` | `eng-<alias>` | Permission surface for the tenant SA. |
| `Kustomization <alias>` | `eng-<alias>` | Watches `harbor-engineering-workspace`@`./engineers/<alias>`, reconciles every 5m, runs as `serviceAccountName: <alias>`. |
| `ServiceAccount workload-service-account` | `eng-<alias>` | For in-namespace workloads. |

The engineer can immediately run `seictl nd apply` against `eng-<alias>` (their EKS access entry — separate from the workload SA — authorizes the kubectl write).

## The agent's job

For a new engineer (pre-flight gate 5 fails because `eng-<alias>` doesn't exist):

1. **Capture the alias.**
   - Default: `$USER` lowercased.
   - Validate: matches `^[a-z]([a-z0-9-]{0,28}[a-z0-9])?$` (DNS-1123 label, k8s-namespace-safe).
   - If the engineer wants something different from `$USER`, they say so.

2. **Render the kustomization.yaml.**
   - Take the fromtherain example above and substitute `fromtherain` → `<alias>` in the `literals:` line and the `kind: Kustomization` `name:` (no — actually only the `alias=` literal changes; the rest stays as-is, because the *replacements* drive substitution from the configMap value into all the placeholders elsewhere).
   - Concretely: copy the fromtherain `kustomization.yaml` verbatim, change exactly one line (`- alias=fromtherain` → `- alias=<alias>`).

3. **Update the aggregator.**
   - Append `- <alias>` to `clusters/harbor/engineers/kustomization.yaml`'s `resources:` list. Maintain alphabetical order if the existing file is sorted.

4. **Branch and commit.**
   - Branch name: `onboard/<alias>` (matches the convention).
   - Commit message (conventional): `feat(harbor/engineers): onboard <alias>`.

5. **Open the PR.**
   - Title: `feat(harbor/engineers): onboard <alias>`.
   - Body: see template below.
   - Use `gh pr create --repo sei-protocol/platform --base main`.

6. **Surface the URL and halt.**
   - Engineer reviews and merges. Flux reconciles in ~60s.
   - Pre-flight gate 5 polls for `kubectl get namespace eng-<alias>`; once it returns 0, the engineer is on the rails.

## PR body template

```markdown
## What

Onboards `<alias>` as a tenant on harbor.

Adds:
- `clusters/harbor/engineers/<alias>/kustomization.yaml` — references `../base`, configMapGenerator + replacements substitute `tenant` → `<alias>`.
- One-line append to `clusters/harbor/engineers/kustomization.yaml` aggregator.

Mirrors the most recent prior onboarding PR verbatim — only the `alias=<alias>` literal differs.

## What reconciles on merge

Within ~60s of merge, Flux materializes:
- `Namespace eng-<alias>` with cells-forward labels.
- `ServiceAccount <alias>` + `Role <alias>` + `RoleBinding <alias>` (Flux reconciler identity, namespace-scoped).
- `Kustomization <alias>` watching `harbor-engineering-workspace`@`./engineers/<alias>`.
- `ServiceAccount workload-service-account` (for in-namespace workloads).

After merge I can immediately run `seictl nd apply` against `eng-<alias>`.
```

## Subsequent onboardings

Each engineer is one PR. Each PR is two files (the per-engineer kustomization + the aggregator update). Each merge reconciles in ~60s. The base layer stays stable; onboarding cost stays linear.

When the second engineer onboards, the agent should suggest the engineer review the most recent prior onboarding PR (e.g., the fromtherain PR) as the diff template — same shape, only the alias literal differs.

## When NOT to use this flow

- **Non-engineer tenants** (CI bot, nightly orchestrator, shared workloads) — those use a different pattern under `clusters/harbor/<bucket>/`, not under `engineers/`. The agent doesn't onboard those; platform team does.
- **Cluster-wide changes** (CRD updates, controller config, Flux infrastructure) — separate PR shapes, not this template.
