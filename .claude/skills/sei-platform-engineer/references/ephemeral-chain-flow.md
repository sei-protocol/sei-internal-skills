# Ephemeral chain flow (GitOps, engineer-facing)

The skill's daily-driver flow for engineers spinning up an ephemeral Sei chain on harbor — typically for benchmarks, release-test runs, or ad-hoc protocol experiments. Engineer says one English sentence; the agent translates into seictl render → git commit on the engineer's workspace branch → push → Flux reconciles → Grafana dashboard URL returned. Total wall-clock: ~3 minutes.

Last verified: 2026-05-04 against [Tide#25] (architectural synopsis) and the harbor release-test orchestrator at `clusters/harbor/nightly/release/` (proof-of-concept that seictl can be driven by a non-human caller).

## The architectural model in three lines

1. **`seictl` is a render layer with two terminal actions.** `--apply` drives kubectl directly (used by automated callers like the release-test CronJob). For engineer-facing flows, the skill renders only and lets git+Flux do the apply. Same render code, two terminal actions.
2. **Branches aren't the isolation boundary, paths are.** Each engineer has one persistent branch `eng-${alias}-workspace`. Within it, each task lives at its own path under `clusters/harbor/eng/${alias}/<task-name>/`. Per-task PRs are unnecessary friction; per-task directories are sufficient isolation.
3. **Flux watches the branch, not main.** Onboarding provisions a per-engineer Flux `GitRepository` + `Kustomization` pointing at the workspace branch. Direct push, no PR loop. PR only at *promotion* (workspace → main, rare and deliberate).

```mermaid
flowchart LR
  E[Engineer: natural language] --> A[sei-platform-engineer skill]
  A -->|seictl chain up<br/>render only| Y[YAML manifests]
  Y -->|git commit + push| B[(eng-${alias}-workspace branch)]
  B -->|Flux watches| F[Flux Kustomization in nightly]
  F -->|kubectl apply| K[Kubernetes API]
  K --> S[SeiNodeDeployment]
  S --> P[seid pods + Service]
  P -->|PodMonitor| M[Prometheus]
  M --> G[Grafana dashboard]
  G --> E
```

## Convention: per-engineer workspace branch

| Property | Value |
|---|---|
| Branch name | `eng-${alias}-workspace` (one per engineer, persistent, never deleted) |
| Watcher | Flux `GitRepository` + `Kustomization` provisioned by `seictl onboard` |
| Push policy | Direct push by the engineer (or by the skill on their behalf). No PR. |
| Promotion | When something graduates to shared infra (e.g., a long-running test profile), open a PR from `eng-${alias}-workspace` → `main`. This is rare. |
| Cleanup | `git rm` the path when a task is done. Flux reconciles the deletion. |

Why one persistent branch instead of one branch per task: branches-per-task multiplies Flux Kustomization configuration N-ways and makes the connective tissue harder to debug. Path-per-task within a single watched branch keeps the onboarding artifact constant.

## Convention: per-task path under the workspace

| Property | Value |
|---|---|
| Root | `clusters/harbor/eng/${alias}/` |
| Per-task subdir | `clusters/harbor/eng/${alias}/<task-name>/` (engineer- or skill-chosen task name) |
| Contents | Rendered SND YAML, RPC YAML, ConfigMap, Job, kustomization.yaml — whatever `seictl chain up` / `seictl rpc up` / `seictl bench up` emits in `data.manifests[]` |
| Lifetime | Engineer choice. Delete when no longer needed; Flux reconciles the deletion within ~60s. |

The task name is the isolation key. `clusters/harbor/eng/bdchatham/bench-mempool-ttl/` and `clusters/harbor/eng/bdchatham/release-2026-05-04/` coexist on the same branch with no interaction.

## What `seictl` provides today vs. what's pending

| Capability | Status |
|---|---|
| `seictl chain up [--apply]` — render a chain (validators) | shipped |
| `seictl rpc up [--apply]` — render an RPC fleet | shipped |
| `seictl bench up [--apply]` — render a benchmark workload (sei-load) | shipped |
| Render-only mode (no `--apply`) emits JSON envelope with `data.manifests[]` | shipped |
| `seictl onboard` provisions namespace + RBAC + IAM | shipped (per-engineer-scoped, [seictl#81]) |
| In-cluster auth fallback (seictl runs from K8s Jobs) | shipped ([seictl#124]) |
| `seictl chain up --to-pr` / `--to-branch` (combined render + write-to-branch + commit + push) | **pending** ([Tide#25] item 1) |
| `seictl onboard` extension to provision per-engineer Flux `GitRepository` + `Kustomization` | **pending** ([Tide#25] item 2) |
| Grafana dashboard with `sei.io/chain-id` filter | **pending** ([Tide#25] item 4) |

Until `--to-pr` ships, the skill performs the same shape using existing primitives: `seictl <verb>` (render only) → `git` to commit + push on the workspace branch → `gh` for any read-only inspection. **The skill should never invoke `seictl chain up --to-pr` until that flag is shipped.** When it ships, the skill collapses two manual steps (write file, git commit/push) into one tool call and this reference is updated.

## Flow: spinup ephemeral chain (today's primitives)

The procedure the skill follows when an engineer says something like "start a chain of 4 validators with seid sha=abc and load it with the profile we used last week."

1. **Identity + cluster check.** `seictl context` — confirm cluster is `harbor` and engineer identity is loaded. Refuse on prod.
2. **Workspace branch check.** Engineer's local platform repo checkout has `eng-${alias}-workspace` branch checked out (or fetched + checked out). If absent, halt and route through `seictl onboard` extension (pending) or surface the manual `git checkout -b eng-${alias}-workspace` step.
3. **Task name.** Derive from the engineer's stated intent or ask one question. The task name becomes the path segment and the chain ID suffix.
4. **Render the chain.** `seictl chain up --image <ref> --validators N --name <task-name>` (no `--apply`) → captures the JSON envelope, extracts `data.manifests[]`, writes each manifest to `clusters/harbor/eng/${alias}/<task-name>/`.
5. **Render the load (if requested).** `seictl bench up --against <chain-id> --profile <profile> --duration <minutes>` (no `--apply`) → same shape, manifests join the same task directory.
6. **Commit + push.** `git add clusters/harbor/eng/${alias}/<task-name>/` → `git commit` with a structured message including chain-id and task-name → `git push origin eng-${alias}-workspace`.
7. **Flux watch.** `kubectl get kustomization eng-${alias}-workspace -n flux-system -o jsonpath='{.status.lastAppliedRevision}'` polled until it matches the pushed commit. Surface the wait to the engineer ("Flux reconciling…").
8. **Pod readiness.** Once Flux has reconciled, watch the SND's pods come up via `kubectl get pods -n eng-${alias} -l sei.io/chain-id=<chain-id>`.
9. **Report.** Print: chain-id, namespace, Grafana dashboard URL filtered by `sei.io/chain-id=<chain-id>`, and the teardown command (`git rm` the task path + push).

## Halt conditions specific to this flow

Stop and report (don't auto-remediate):

- **Workspace branch doesn't exist.** Engineer hasn't been onboarded with the GitOps extension. Surface `seictl onboard --apply` (and the pending `--with-gitops` flag note) and stop.
- **Flux Kustomization absent or unhealthy.** `kubectl get kustomization eng-${alias}-workspace` returns NotFound, or `.status.conditions[type=Ready].status=False`. Surface the Kustomization name + namespace and stop.
- **Engineer's working tree dirty on a different branch.** Don't switch branches under uncommitted changes. Halt and ask the engineer to stash or commit first.
- **Push rejected (non-fast-forward).** Don't force-push the workspace branch — there may be commits the engineer made out-of-band (or another agent session). Halt, surface `git pull --rebase origin eng-${alias}-workspace`, and let the engineer resolve.
- **Flux reconcile timeout.** If `lastAppliedRevision` doesn't match the pushed commit within 90s, halt and surface `kubectl describe kustomization eng-${alias}-workspace -n flux-system` for the engineer to inspect.
- **Manifest collision at the task path.** The path already exists with content. Don't silently overwrite — halt, surface the path, and ask whether to choose a new task name or `git rm` the existing one first.

## Why this is the engineer-facing path (and `--apply` isn't)

`--apply` is the right shape for **automated callers** — release-test CronJobs, CI/CD pipelines, anything that runs in-cluster and doesn't need a human-readable audit trail. The release-test orchestrator at `clusters/harbor/nightly/release/` is the canonical example: the CronJob runs `seictl chain up --apply`, captures the JSON envelope, applies a downstream Job. Fast, atomic, no git dependency.

`--to-pr` (or today's manual git equivalent) is the right shape for **engineer-facing flows** — daily-driver experiments, benchmark spinups, demo provisioning. The git history *is* the audit trail. The workspace branch *is* the engineer's playground. PR only at promotion.

Both modes share the same render code; only the terminal action differs.

## Promotion: workspace → main

When something on the engineer's workspace branch should graduate to shared infrastructure (e.g., a load profile that should be in nightly, a SND template that other engineers want), open a PR from `eng-${alias}-workspace` → `main` for just that path. The PR follows `references/pr-conventions.md`. After merge, `git rm` the path from the workspace branch.

This is the only PR loop in the engineer flow. It's deliberate, rare, and reviewed.

## References

- [Tide#25] — architectural synopsis that produced this flow (the skill embeds the gist; the issue carries the full discussion).
- [seictl#124] — in-cluster auth fallback (lets seictl run from K8s Jobs; load-bearing for the release-test parallel).
- [seictl#127] — `keys generate` subcommand for ephemeral admin keypair (related enhancement).
- `clusters/harbor/nightly/release/` (sei-protocol/platform) — proof-of-concept that seictl can be driven by a non-human caller. The CronJob shape is what the engineer flow mirrors with `git push` substituted for `kubectl apply`.
- `cluster/internal/githubpr/` (sei-protocol/seictl) — the PR-creation infrastructure the planned `--to-pr` flag will reuse. Already proven by `seictl onboard`.
