# Kustomize composition kit

## 1. What this concern is

Per-cell config is composed with **plain Kustomize in two layers** — shared bases under `clusters/base/<infra-component>` and `manifests/base/<workload>`, with each cell overlaying via `resources` + `patches` + `components`, and per-cell variance expressed by `configMapGenerator` + **`replacements`**. The generic Flux habit of `postBuild.substitute` (envsubst-style variable substitution) is **not used here** and is the wrong reach. *Cited:* `clusters/prod/cert-manager/kustomization.yaml`, `manifests/base/`; `sources.md` §kustomize.

## 2. The pattern (how this fleet does it)

- **Two base layers.** `clusters/base/<component>` = cluster services (cert-manager, monitoring, cni-cilium, kube-system, sei-k8s-controller); `manifests/base/<workload>` = workloads (seid, waterway, genesis, monitoring). *Cited:* `clusters/base/`, `manifests/base/`.
- **The overlay nests two levels — mind the `..` depth.** The **cell root** (`clusters/<cell>/kustomization.yaml`) aggregates per-component overlay dirs as bare-name `resources` (`cert-manager`, `arctic-1`, …) and pulls cross-cutting `components: [../base/cni-cilium]` — from the cell root, **one** `..` reaches `clusters/`, so the component base is `../base/X`. Each **per-component overlay** (`clusters/<cell>/<component>/kustomization.yaml`) is one level deeper and does `resources: [../../base/<component>]` (**two** `..` to reach `clusters/`) + its own `patches:` (e.g. the prod-euw1 cert-manager `startupapicheck` disable). Don't mix the two depths in one file. *Cited:* `clusters/prod-euw1/kustomization.yaml` (cell root: bare-name resources + `../base/cni-cilium`), `clusters/prod-euw1/cert-manager/kustomization.yaml` (component overlay: `../../base/cert-manager`).
- **Per-cell variance = `configMapGenerator` + `replacements`** (copy a value into N targets) — **never `postBuild.substitute`** (zero repo use). *Cited:* `clusters/`/`manifests/` (replacements), confirmed-absent postBuild.
- **Staged remote-base pin.** `clusters/base/sei-k8s-controller/` references the controller repo at one `?ref=<sha>`; a prod cell pins a newer sha for staged rollout. *Cited:* `clusters/base/sei-k8s-controller/` vs `clusters/prod/sei-k8s-controller/`.

## 3. Anti-patterns / failure modes

- **`postBuild.substitute`.** Reaching for Flux envsubst variables. Cue: a `postBuild.substitute` block. Rewrite: `configMapGenerator` + `replacements`, or a `patch`.
- **Forked per-cell copies.** Duplicating a base into each cell instead of overlaying. Cue: near-identical files across cells with no shared base. Rewrite: a `clusters/base`/`manifests/base` + per-cell `patches`.
- **Cross-layer confusion.** Putting a workload in `clusters/base` or a cluster-service in `manifests/base`. Cue: a seid manifest under `clusters/base`. Rewrite: workloads → `manifests/base`, cluster services → `clusters/base`.
- **Unpinned remote base.** A `?ref=` on a branch instead of a sha for the controller. Cue: `?ref=main`. Rewrite: pin a sha (staged rollout is a deliberate sha bump).

## 4. Review cues

- **Dimension 4 (multi-env/cell structure):** clean two-layer split; per-cell difference via `patches`/`components`/`replacements`, not forks, not `postBuild.substitute`; the `?ref=` controller pin honored. *Basis:* profile §2/§7, `sources.md` §kustomize.
- **Dimension 3 (GitOps-reconcilability):** the overlay renders deterministically (no envsubst dependency). *Basis:* `sources.md` §opengitops.

## 5. One-way doors in this concern

- **Bumping the prod `?ref=` sei-k8s-controller sha** rolls the controller in prod — a staged-rollout decision; flag for human approval (it's the deploy half of a controller change `/kubernetes` authored).
- **Restructuring the base layout** (moving a component between `clusters/base` and `manifests/base`) churns every cell overlay — flag the blast radius.
