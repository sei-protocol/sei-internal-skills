---
name: sei-platform-engineer
description: "Engineer-facing interface to Sei platform infrastructure on the harbor EKS cluster. Translates natural-language intent into seictl invocations + GitOps writes — spin up ephemeral chains via the engineer's workspace branch, provision benchmarks, onboard new engineers, inspect cluster state. Trigger on 'spinup benchmark', 'run a benchmark', 'benchmark this image', 'spin up a chain', 'start a chain', 'ephemeral chain', 'give me N validators', 'load this chain', 'onboard me', 'set me up on harbor', 'what's running on harbor', 'where am I on the cluster'. NOT for sei-k8s-controller code changes. NOT for autobake nightly cron changes. NOT for chaos testing (use chaos-suite). For multi-component design work, use /council or /coral."
---

# sei-platform-engineer

Engineer-facing interface to Sei platform infrastructure on the **harbor** EKS cluster. The engineer describes what they want; intent translates into `seictl` invocations and GitOps writes. The engineer doesn't need to know that SeiNode/SeiNodeDeployment CRDs exist, what Kustomize is, or where snapshots live in S3 — `seictl` knows.

This is the conversational layer over `seictl` (sei-protocol/seictl). When MCP graduation happens, the same procedures become tool calls; the content here is the contract today.

## Guardrails

Operate against **harbor**. Engineers don't have prod kubeconfig contexts locally — the auth boundary enforces the separation, no duplication needed here.

The full discipline lives in **Pre-flight** below (and `references/preflight.md`). The hard rules:

1. **Cluster must be harbor.** `seictl context` confirms; refuse on prod outright.
2. **Identity required.** `~/.seictl/engineer.json` must exist before any `seictl chain`, `seictl rpc`, or `seictl bench` command. Pre-flight gate 6 routes to First Run if absent.
3. **Scope echo on first side-effecting verb.** Echo cluster + namespace + image digest + the workspace path about to be written. Wait for confirmation.
4. **Refuse-and-surface, don't auto-remediate.** Where pre-flight has an in-band recovery (write a kubeconfig, create the identity file, refresh main in a worktree), run it. Where the recovery is out-of-band (SSO login, EKS access entry, PR merge), surface the next step and halt. Never silently work around a missing prereq.

## Preconditions

These are the *steady state* requirements. The skill doesn't assume the engineer arrives in this state — Pre-flight (next section) walks them there. List exists for reference.

- `seictl` v1.x installed and on `$PATH` (see [seictl install docs](https://github.com/sei-protocol/seictl#installation))
- AWS SSO session active for the **`sei`** profile (`aws sts get-caller-identity --profile sei` returns 0). Always pass `--profile sei` to AWS CLI invocations — the engineer's default profile may have no credentials.
- `kubectl` configured against harbor (`aws eks update-kubeconfig --name harbor --region eu-central-1 --profile sei`)
- EKS access entry granting your AWS principal cluster auth (granted by the platform team, not by `seictl onboard`)
- `gh` authenticated for any verb that opens a PR (`gh auth status` → ok)
- Identity file `~/.seictl/engineer.json` (created by `seictl onboard` on first run)
- Engineer's namespace `eng-<alias>` reconciled by Flux (depends on onboarding PR being merged)
- Workspace branch `eng-<alias>-workspace` exists with a per-engineer Flux Kustomization watching it (manual today; pending the [Tide#25] onboard extension)
- AWS credentials with read access to `189176372795.dkr.ecr.us-east-2.amazonaws.com` for image digest resolution

## Pre-flight (run at session start, before any side-effecting action)

Pre-flight is the first job of any new session. It's a **ramp**, not just a gate — where there's an in-band recovery (write a kubeconfig, refresh a worktree, create the identity file, run `seictl onboard --apply`), execute it and continue. Where the recovery is out-of-band (SSO login, EKS access entry from platform team, PR merge), surface the exact next step and halt cleanly. The goal is to get the engineer **on the rails** — namespace exists, kubectl works, platform repo ready, GitOps flow available — as quickly as possible.

Run the gates in order. Halt on the first failure; later gates depend on earlier ones.

| # | Gate | Detect with | If missing → |
|---|---|---|---|
| 1 | `seictl` on PATH | `command -v seictl` returns 0 (or `seictl help` exits 0 — `--version` isn't a flag) | Surface `brew install sei-protocol/tap/seictl` (or release URL); halt. |
| 2 | AWS SSO session active for `sei` profile | `aws sts get-caller-identity --profile sei` returns 0 | Surface `aws sso login --profile sei`; halt. **Always pass `--profile sei` (or `AWS_PROFILE=sei`) — the engineer's default profile may not have credentials even when the `sei` profile is active.** |
| 3 | harbor kubeconfig context exists | `kubectl config get-contexts -o name` lists `harbor` | Run `aws eks update-kubeconfig --name harbor --region eu-central-1 --profile sei` directly, re-check, continue. |
| 4 | kubectl can reach harbor | `kubectl auth can-i list namespaces --context=harbor` returns `yes` | EKS access entry not granted. Not something `seictl onboard` provisions today. Surface "ask the platform team via `#harbor-onboarding` with your AWS principal ARN"; halt. |
| 5 | Platform repo worktree on main, fresh | `$SEI_PLATFORM_REPO` (or fallback) is a `sei-protocol/platform` clone with a clean worktree on `main` at or behind `origin/main` | If the repo isn't located, surface the clone command and halt. If on main but dirty, halt and ask the engineer to stash/commit. If on a different branch with WIP, create an isolated worktree (`git worktree add ~/.seictl/worktrees/platform-main main`) and operate from there — never trample the engineer's primary checkout. If main is stale, run `git fetch origin && git pull --ff-only origin main` in the worktree, then continue. |
| 6 | Identity file present | `~/.seictl/engineer.json` parses + has `alias` + `name` | Route to **First Run** below — capture alias + name, write the identity file, run `seictl onboard --apply` from the gate-5 worktree to open the namespace + IAM provisioning PR. |
| 7 | Namespace reconciled | `kubectl get namespace eng-<alias>` returns 0 | Onboarding PR not merged or Flux hasn't reconciled yet. Surface the PR URL; offer to poll until the namespace appears (~60s post-merge). |
| 8 | Workspace branch ready (GitOps only) | `git ls-remote origin eng-<alias>-workspace` returns a ref | Pending [Tide#25] item 2 (onboard extension). Surface the manual bootstrap (`git checkout -b eng-<alias>-workspace && git push`, then ask platform team for the Flux Kustomization); halt the GitOps flow. |

Once all eight pass, cache the pass for the session — subsequent verbs skip the gates unless a halt condition (SSO expiry, kubectl context drift, worktree drift) triggers a targeted re-check.

For deep detail per gate (recovery commands, edge cases, the full new-engineer walk-through, mid-session drift handling), see `references/preflight.md`.

## Mental model

You operate against the **harbor cluster** (eu-central-1 EKS). It runs the **sei-k8s-controller** which watches `SeiNode` and `SeiNodeDeployment` CRs across all namespaces and reconciles them into StatefulSets, PVCs, Services, and HTTPRoutes.

**Where you work:**
- `eng-<your-alias>` namespace, governed by the personal-cells security posture (quotas, NetworkPolicy, admission). Onboarding is a one-time PR.
- Active workloads land on a per-engineer workspace branch (`eng-<alias>-workspace`) at task-specific paths under `clusters/harbor/eng/<alias>/`. Flux reconciles the branch.

### House style: GitOps is the default. `--apply` is for automated callers only.

**The skill renders YAML and pushes it to the engineer's workspace branch. The platform reconciles. The skill never invokes `seictl ... --apply` for an engineer-facing intent.**

Why this is the strong opinion:

- The git history *is* the audit trail. Every chain, RPC fleet, and load run is a commit on a known branch at a known path. `kubectl apply` leaves no equivalent record.
- Flux owns reconciliation. If the engineer's manifests drift, Flux re-asserts them. With `--apply`, drift goes unnoticed until the next manual reconcile.
- Teardown is `git rm`. One mechanism in, one mechanism out — both visible in git.
- The same workspace branch is the substrate for promotion to shared infra (workspace → main PR). `--apply` produces nothing promotable.
- Cluster headroom incidents are easier to forensic — `git log clusters/harbor/eng/` tells you exactly who did what when, across all engineers.

**`--apply` is reserved for automated callers** — the release-test CronJob at `clusters/harbor/nightly/release/`, CI/CD pipelines, anything that runs in-cluster, doesn't need a human-readable audit trail, and benefits from the speed of skipping the git+Flux loop. **Don't take this path for engineer-facing intents.** If an engineer explicitly asks for `--apply`, first ask whether the GitOps flow would serve the same need; only proceed if they confirm they want no git history (rare; e.g., a one-shot CI debug session).

Same render layer, two terminal actions. **The terminal action for engineer-facing intents is `git push`, not `kubectl apply`.**

**Branches aren't the isolation boundary, paths are.** One persistent branch per engineer, never deleted. Each task is its own directory. PR only at *promotion* (workspace → main, rare and deliberate).

**The split:**
- **Long-lived infra** (namespace, RBAC, ServiceAccounts, per-engineer Flux `GitRepository` + `Kustomization`) — Flux-managed via the platform repo. Onboarding adds the engineer's namespace and workspace-watcher via PR.
- **Ephemeral workloads** (chain validators, RPC fleet, seiload Job) — rendered by `seictl`, written to the engineer's workspace branch, reconciled by Flux. Tear down via `git rm` on the task path + push.

See `references/ephemeral-chain-flow.md` for the architectural model (branch convention, path scheme, what's pending), `references/harbor-cluster.md` for cluster facts, `references/interim-namespace-strategy.md` for the cells-forward labels we use today.

## First Run (the recovery for pre-flight gate 6)

When pre-flight gate 6 fails (`~/.seictl/engineer.json` missing), enter First Run. By this point gates 1–5 have already passed — seictl, SSO, kubeconfig, access entry, and a clean platform repo worktree on main are all in place — so the onboarding PR can be opened and the cluster will accept it.

```
sei-platform-engineer: First time — let's set up your identity.
Alias [defaults from $USER]:
Name [defaults from `git config user.name`]:
Saved to ~/.seictl/engineer.json.

Generating onboarding PR for clusters/harbor/engineers/<alias>/...
```

The skill calls `seictl onboard --alias <alias> --apply` which:

1. Validates the alias (lowercase, k8s-namespace-safe — `^[a-z]([a-z0-9-]{0,28}[a-z0-9])?$`)
2. Generates `clusters/harbor/engineers/<alias>/{kustomization,namespace,bench-seiload-sa}.yaml` in the platform repo working tree
3. Branches `seictl/onboard-<alias>`, commits, opens a PR via `gh`
4. Creates the IAM policy + Pod Identity association directly via AWS SDK in the engineer's SSO session — no Terraform

Engineer reviews and merges the PR, Flux reconciles in ~60s, and the engineer's namespace exists.

See `references/pr-conventions.md` for branch + PR conventions.

## What you can do

Every chain/RPC/bench intent maps to the **GitOps flow**: render with `seictl` (no `--apply`), write to the engineer's workspace path, commit, push. Flux reconciles. The skill does not invoke `--apply` for engineer-facing intents — see "House style" above.

| Engineer says | Skill maps to |
|---|---|
| "Onboard me" / "set me up on harbor" / "I'm new" | `seictl onboard --alias <alias> [--apply]` (this is the one place `--apply` is used — it provisions the workspace itself) |
| "Spin up a chain of N validators with image X" / "give me an ephemeral chain" / "start a chain" | **GitOps flow** (see Procedure: spinup ephemeral chain): `seictl chain up` (render-only) → write to `clusters/harbor/eng/<alias>/<task>/` → commit → push to `eng-<alias>-workspace`. Flux applies. |
| "Add an RPC fleet to chain X" / "I need RPC nodes" | `seictl rpc up --against <chain-id>` (render-only) → same workspace path → commit → push. |
| "Load chain X with profile Y" / "hit it with traffic" | `seictl bench up --against <chain-id> --profile <profile>` (render-only) → same workspace path → commit → push. |
| "Run a benchmark against image X" / "spinup benchmark" / "benchmark this image" | **GitOps flow.** The skill renders `seictl chain up` + `seictl bench up` together, writes both to the same task directory, commits, pushes. The skill *does not* invoke `bench up --apply` here — see escape hatch below. |
| "Tear down my chain/bench at path X" / "wipe task Y" | `git rm -r clusters/harbor/eng/<alias>/<task>/` → commit → push. Flux reconciles the deletion. |
| "What's running on my workspace" / "what tasks do I have" | `git ls-tree --name-only HEAD clusters/harbor/eng/<alias>/` on the workspace branch (authoritative) + `kubectl get seinodedeployment -n eng-<alias>` (live cluster view). |
| "Where am I" / "what cluster am I on" / "who am I" | `seictl context` |

**Escape hatch — direct `--apply` (rare, requires explicit engineer confirmation):** if an engineer specifically asks to bypass git for a one-shot debug/CI session and confirms they understand there will be no audit trail, falling through to `seictl bench up --apply` is permitted. First offer the GitOps path and ask "are you sure you want to skip the workspace branch?" — only proceed on explicit yes. Don't volunteer this path; honor it only when the engineer asks twice.

## Procedure: spinup ephemeral chain (the headline — GitOps flow)

Engineer says "spin up a chain of 4 validators with seid sha=abc, then load it with the profile we used last week." This is the daily-driver flow. The skill renders manifests with `seictl`, writes them to the engineer's workspace branch at a per-task path, commits, pushes — Flux reconciles within ~60s and the engineer gets a Grafana URL.

**Read `references/ephemeral-chain-flow.md` first.** It carries the architectural model: per-engineer workspace branch, path-based task isolation, the render/write/push/reconcile shape, and what's pending vs shipped today. The procedure below is the operational restatement.

1. **Pre-flight** — if not already passed this session, run all eight gates (see Pre-flight section above and `references/preflight.md`). Halt on first gate failure with the recovery surfaced. Past this point, the engineer is on the rails: cluster reachable, platform repo on main, namespace exists, workspace branch present.
2. **Local checkout on workspace branch** — engineer's local platform repo working tree is on `eng-<alias>-workspace`. If on a different branch with a dirty tree, halt and ask the engineer to stash or commit first. If on a clean different branch, run `git checkout eng-<alias>-workspace && git pull --ff-only origin eng-<alias>-workspace` and continue.
3. **Task name** — derive from the engineer's stated intent (one English sentence) or ask one question. Lowercase, k8s-namespace-safe. Becomes the path segment and the chain-id suffix.
4. **Render the chain** — `seictl chain up --image <ref> --validators N --name <task-name>` (no `--apply`). Capture the JSON envelope. Extract `data.manifests[]` and write each manifest to `clusters/harbor/eng/<alias>/<task-name>/`.
5. **Render the load (if requested)** — `seictl bench up --against <chain-id> --profile <profile> --duration <minutes>` (no `--apply`). Same shape; manifests join the same task directory.
6. **Plan echo & confirm** — on the first side-effecting call of the session, show the engineer the resolved plan: chain-id, image digest, fleet size, task path under the workspace, what's about to be committed and pushed. Wait for confirmation.
7. **Commit + push** — `git add clusters/harbor/eng/<alias>/<task-name>/`, then a structured commit message (`feat(eng/<alias>): spin up <task-name> — chain-id=<id>, image=<digest-prefix>`), then `git push origin eng-<alias>-workspace`. Don't force-push.
8. **Wait for Flux** — poll `kubectl get kustomization eng-<alias>-workspace -n flux-system -o jsonpath='{.status.lastAppliedRevision}'` until it matches the pushed commit. 90s timeout; halt and surface `kubectl describe kustomization` on overrun.
9. **Wait for pods** — `kubectl get pods -n eng-<alias> -l sei.io/chain-id=<chain-id>` until all SND pods are Ready.
10. **Report** — print: chain-id, namespace, Grafana dashboard URL filtered by `sei.io/chain-id=<chain-id>` (target shape; the dashboard's chain-id-filter is pending per [Tide#25] item 4 — surface the unfiltered URL and the label until it lands), the teardown command (`git rm -r clusters/harbor/eng/<alias>/<task-name>/` + commit + push).

**What's pending and what to do meanwhile.** The convenience flag `seictl chain up --to-pr` (which would collapse steps 4 + 7 into one call) is on the seictl roadmap but not shipped. **Don't invoke `--to-pr` until it's released** — perform render → write → commit → push manually using `seictl` (render-only) + `git`. When `--to-pr` ships, this procedure compresses; until then the manual shape is the contract.

See `references/ephemeral-chain-flow.md` for the full architectural detail.

## Procedure: direct `--apply` benchmark (escape hatch, not the default)

The skill's default for "run a benchmark" is the GitOps flow above — render `seictl chain up` + `seictl bench up` (no `--apply`), write both to the engineer's workspace path, push. **This `--apply` procedure exists only for the rare case where an engineer explicitly opts out of git** — typically a CI debug session, or a one-shot run they don't want in the workspace branch's history. The skill does not volunteer this path. It only takes it when the engineer asks twice.

**Steer first.** Before running any of the steps below, ask the engineer: "I can do this through the GitOps flow on your workspace branch (audit trail, Flux reconciles, `git rm` to tear down) — do you want that, or do you specifically need a direct-apply run with no git history?" If the engineer wants GitOps, route to the headline procedure above. **Only proceed below on explicit confirmation that GitOps is not what they want.**

1. **Pre-flight** — if not already passed this session, run gates 1–7 (gate 8 / workspace branch is not required for the `--apply` path; gate 5 / platform repo is also not strictly required for `--apply`, but run it for halt-condition completeness). Halt on failure.
2. **Image resolution** — engineer provided an image ref. seictl resolves to immutable digest internally; surface failures cleanly.
3. **Ask up to 3 questions**, in order, only when defaults would surprise:
   - "What are you testing? (one sentence — goes in chain ID name)"
   - "Fleet size: small (4 validators), medium (10), large (21)? [s]"
   - "Duration in minutes (1–240)? [30]"
4. **Plan echo & confirm** — show the engineer the resolved invocation: chain ID (`bench-<alias>-<name>`), image digest, fleet size, duration, S3 results path, **and a reminder that this run will not appear in the workspace branch's git history**. Wait for confirmation.
5. **Invoke** — `seictl bench up --image <ref> --name <name> --size <size> --duration <duration> --apply`. seictl renders templates, applies via kubectl.
6. **Report** — print the chain ID, S3 results path, and the `seictl bench list` + `seictl bench down` follow-up commands.

See `references/intent-benchmark.md` for the full conversation tree, default selection rationale, and the autobake-derived templates that drive the fleet shape.

## Procedure: troubleshooting (manual; no `seictl diagnose` verb in v1)

Engineer says "X is stuck" or "diagnose seinode foo." There's no automated `seictl seinode diagnose` in v1 — walk the engineer through the manual `kubectl`-driven flow documented in `references/troubleshooting-seinode.md`.

1. Read `.status.phase`: `kubectl get seinode <name> -o jsonpath='{.status.phase}'`
2. Branch on phase:
   - **Pending** → check controller pods + leader lease in `sei-system`
   - **Initializing** → read `.status.plan` for the failing PlannedTask; map task name to root cause (snapshot-restore → S3 / Pod Identity, configure-genesis → genesis URL, discover-peers → EC2 tags, mark-ready → seid health)
   - **Running** → check seid logs, HTTPRoute routing, pod restarts
   - **Failed** → read `.status.conditions[type=Ready].message`; decide retry vs escalate
3. Surface the matching kubectl invocations from `references/troubleshooting-seinode.md`.

If recurring patterns surface enough to be worth automating, codify them as `seictl seinode diagnose` in v1.1.

## Procedure: read-only verbs (`context`, `bench list`)

Pure invocations. Skill calls `seictl <verb>` and surfaces the structured output to the engineer in plain English. No questions, no confirmation.

## Halt conditions

Stop and report to the user if:

- **kubectl context drifts mid-session** — engineer switched contexts in another terminal. Re-confirm before any side effect.
- **`seictl` exits non-zero with an unexpected error code** — surface stderr. Do not retry silently.
- **Identity file becomes invalid** — corrupted JSON or missing fields. Halt and prompt the engineer to fix or re-create `~/.seictl/engineer.json`.
- **Engineer's onboarding PR isn't merged but they're trying to spin up a chain or bench** — namespace doesn't exist yet. Stop and report. Don't auto-create.
- **Image digest resolution fails** — image not in ECR or auth missing. Stop and surface the recovery command.
- **Image not yet in ECR** — sei-chain CI may be behind. Surface the explicit retry command per the autobake race-guard pattern; don't loop silently.

GitOps-flow specific (the new headline procedure):

- **Workspace branch doesn't exist** — engineer hasn't been onboarded with the GitOps extension (or never ran `seictl onboard`). Surface `seictl onboard --apply` and stop. Don't auto-create the branch.
- **Flux Kustomization absent or unhealthy** — `kubectl get kustomization eng-<alias>-workspace -n flux-system` returns NotFound, or `.status.conditions[type=Ready].status=False`. Surface the Kustomization name + namespace and stop.
- **Working tree dirty on a different branch** — don't switch branches under uncommitted changes. Halt and ask the engineer to stash or commit first.
- **Push rejected (non-fast-forward)** — there may be commits the engineer made out-of-band or in another agent session. Don't force-push. Halt, surface `git pull --rebase origin eng-<alias>-workspace`, let the engineer resolve.
- **Flux reconcile timeout** — `lastAppliedRevision` doesn't match the pushed commit within 90s. Halt and surface `kubectl describe kustomization eng-<alias>-workspace -n flux-system` for the engineer to inspect.
- **Manifest collision at the task path** — the path already exists with content. Don't silently overwrite. Halt, surface the path, and ask whether to choose a new task name or `git rm` the existing one first.

## Reference index

| File | Scope |
|---|---|
| `preflight.md` | **Read this first on a new session or when an engineer is fresh.** Seven-gate ramp from "fresh laptop" to "on the rails," per-gate recovery, mid-session drift handling, full new-engineer walk-through |
| `ephemeral-chain-flow.md` | **Read this first if the engineer asks for a chain.** Architectural model: workspace branch, path-based task isolation, render/write/push/reconcile shape, what's pending vs shipped |
| `seictl-cli.md` | Canonical command surface (regenerated from `seictl --help` periodically) |
| `intent-benchmark.md` | Full legacy `bench up --apply` conversation tree + autobake-derived fleet conventions |
| `seinode-crd.md` | Operations-load-bearing fields (the 6 spec, 4 status fields you'll touch) |
| `seinodedeployment-crd.md` | Same discipline for the SND CRD |
| `troubleshooting-seinode.md` | Phase-by-phase symptom → cause → inspection decision tree |
| `harbor-cluster.md` | CNI (Cilium), Istio + Gateway API, DNS, Flux topology, EKS access entries |
| `aws-dependencies.md` | S3 buckets (snapshots, genesis, results), Pod Identity, ECR conventions |
| `interim-namespace-strategy.md` | Cells-forward-compatible labels we ship today; how cells will retrofit |
| `pr-conventions.md` | Branch naming, PR body shape, reviewer conventions for onboarding PRs and workspace→main promotion PRs |

When this skill drifts from `seictl`'s actual behavior, **`seictl --help` wins.** Reference files include a dated last-verified note per section to help spot drift.

## Permission pre-approval

Pre-approve in `.claude/settings.local.json` (user-specific, not committed):

```json
{
  "permissions": {
    "allow": [
      "Bash(seictl context:*)",
      "Bash(seictl bench list:*)",
      "Bash(kubectl config current-context:*)",
      "Bash(kubectl get seinode:*)",
      "Bash(kubectl get seinodedeployment:*)",
      "Bash(kubectl get kustomization:*)",
      "Bash(kubectl get pods:*)",
      "Bash(kubectl logs:*)",
      "Bash(aws sts get-caller-identity:*)",
      "Bash(gh auth status:*)",
      "Bash(git status:*)",
      "Bash(git log:*)",
      "Bash(git diff:*)",
      "Bash(git ls-tree:*)",
      "Bash(git branch:*)",
      "Bash(git rev-parse:*)"
    ]
  }
}
```

**Leave interactive** (never pre-approve):

- `seictl bench up` (`--apply` legacy path) — provisions resources directly via kubectl; requires explicit confirmation
- `seictl bench down` — destroys resources; requires explicit confirmation
- `seictl onboard` — opens a PR; requires explicit confirmation
- `git add` / `git commit` / `git push` — workspace-branch writes. Each side-effecting step in the GitOps flow gets explicit confirmation; engineers can audit the diff before commit and the commit before push.
- `git rm` — task teardown writes; same reasoning.

Note: `seictl chain up` / `rpc up` / `bench up` *without* `--apply` are render-only (pure functions, no cluster writes) and could be pre-approved if a session uses them frequently — but the safer default is to leave interactive so the engineer sees the rendered manifests before they're written to the workspace.

Use the `fewer-permission-prompts` skill against a real session transcript once the skill is in active use.

## State management

No per-run state is maintained here — `seictl` owns it. Operation is stateless between invocations: every cluster-facing verb starts with a fresh `seictl context` call to establish ground truth. The engineer's identity lives at `~/.seictl/engineer.json` (managed by `seictl`), and active resources live in the cluster (queryable by `seictl bench list`).

If `~/.seictl/engineer.json` exists but is malformed, halt and prompt the engineer to fix or re-create the file. Don't try to repair.

---

## Status: v1 surface shipped

The five cluster verbs (`context`, `bench up`, `bench down`, `bench list`, `onboard`) all merged in sei-protocol/seictl during the cluster-cli LLD workstream:

- LLD merged: #65
- `bench up` apply path + envelope rename: #71, #72, #73
- Go bump for cli-runtime adoption: #74
- `kube.Client` reshape around `genericclioptions.ConfigFlags` + envtest coverage: #76, #77, #79
- `bench down` + `bench list`: #78
- `onboard`: #81 (per-engineer-scoped IAM; option-B per-engineer K8s identity tracked at #80)

The verb table above reflects what `seictl --help` actually emits. References are aligned to the shipped envelope (`apiVersion: seictl.sei.io/v1`), exit-code matrix, and result types. When this file disagrees with `seictl --help`, the CLI wins.

See `references/seictl-cli.md` for the canonical command surface.
