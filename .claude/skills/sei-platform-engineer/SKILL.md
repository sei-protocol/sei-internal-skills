---
name: sei-platform-engineer
description: "Engineer-facing interface to Sei platform infrastructure on the harbor EKS cluster. Translates natural-language intent into seictl invocations + GitOps writes — spin up ephemeral chains via the engineer's workspace branch, provision benchmarks, onboard new engineers, inspect cluster state. Trigger on 'spinup benchmark', 'run a benchmark', 'benchmark this image', 'spin up a chain', 'start a chain', 'ephemeral chain', 'give me N validators', 'load this chain', 'onboard me', 'set me up on harbor', 'what's running on harbor', 'where am I on the cluster'. NOT for sei-k8s-controller code changes. NOT for autobake nightly cron changes. NOT for chaos testing (use chaos-suite). For multi-component design work, use /council or /coral."
---

# sei-platform-engineer

Engineer-facing interface to Sei platform infrastructure on the **harbor** EKS cluster. You describe what you want; the skill translates intent into `seictl` invocations. You don't need to know that SeiNode/SeiNodeDeployment CRDs exist, what Kustomize is, or where snapshots live in S3 — `seictl` knows. The skill maps engineer intent to the right invocation.

This is the conversational layer over `seictl` (sei-protocol/seictl). When MCP graduation happens, the same procedures become tool calls; the skill content is the contract today.

## Guardrails

This skill operates against **harbor**. Engineers don't have prod kubeconfig contexts locally — the auth boundary enforces the separation, the skill doesn't duplicate it. Before any side-effecting action:

1. **Identity check** — `~/.seictl/engineer.json` must exist before any `seictl chain`, `seictl rpc`, or `seictl bench` command. If absent, route through `seictl onboard` first.
2. **Scope echo on first invocation** — on the first side-effecting verb of a session, echo the resolved cluster + namespace + image digest back to the engineer for confirmation.
3. **Refusal conditions** — refuse to proceed if:
   - `seictl` is not on `$PATH`
   - The engineer's namespace doesn't exist and they haven't run `seictl onboard --apply`
   - The requested image isn't pullable (digest resolution fails)

Never auto-remediate. Surface the problem; the engineer decides.

## Preconditions

- `seictl` v1.x installed and on `$PATH` (see [seictl install docs](https://github.com/sei-protocol/seictl#installation))
- `kubectl` configured against harbor (`aws eks update-kubeconfig --name harbor --region eu-central-1`)
- `gh` authenticated for any verb that opens a PR (`gh auth status` → ok)
- Identity file `~/.seictl/engineer.json` (created by `seictl onboard` on first run)
- AWS credentials with read access to `189176372795.dkr.ecr.us-east-2.amazonaws.com` for image digest resolution

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

**`--apply` is reserved for automated callers** — the release-test CronJob at `clusters/harbor/nightly/release/`, CI/CD pipelines, anything that runs in-cluster, doesn't need a human-readable audit trail, and benefits from the speed of skipping the git+Flux loop. **The skill itself doesn't take this path.** If an engineer explicitly asks for `--apply`, the skill first asks whether the GitOps flow would serve the same need; only proceeds if the engineer confirms they want no git history (rare; e.g., a one-shot CI debug session).

Same render layer, two terminal actions. **The skill's terminal action is `git push`, not `kubectl apply`.**

**Branches aren't the isolation boundary, paths are.** One persistent branch per engineer, never deleted. Each task is its own directory. PR only at *promotion* (workspace → main, rare and deliberate).

**The split:**
- **Long-lived infra** (namespace, RBAC, ServiceAccounts, per-engineer Flux `GitRepository` + `Kustomization`) — Flux-managed via the platform repo. Onboarding adds the engineer's namespace and workspace-watcher via PR.
- **Ephemeral workloads** (chain validators, RPC fleet, seiload Job) — rendered by `seictl`, written to the engineer's workspace branch, reconciled by Flux. Tear down via `git rm` on the task path + push.

See `references/ephemeral-chain-flow.md` for the architectural model (branch convention, path scheme, what's pending), `references/harbor-cluster.md` for cluster facts, `references/interim-namespace-strategy.md` for the cells-forward labels we use today.

## First run

If `~/.seictl/engineer.json` doesn't exist when an engineer invokes any verb, route them through onboarding first:

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

**Escape hatch — direct `--apply` (rare, requires explicit engineer confirmation):** if an engineer specifically asks to bypass git for a one-shot debug/CI session and confirms they understand there will be no audit trail, the skill *may* fall through to `seictl bench up --apply`. The skill must first offer the GitOps path and ask "are you sure you want to skip the workspace branch?" — only proceed on explicit yes. The skill does not volunteer this path; it only honors it when the engineer asks twice.

## Procedure: spinup ephemeral chain (the headline — GitOps flow)

Engineer says "spin up a chain of 4 validators with seid sha=abc, then load it with the profile we used last week." This is the daily-driver flow. The skill renders manifests with `seictl`, writes them to the engineer's workspace branch at a per-task path, commits, pushes — Flux reconciles within ~60s and the engineer gets a Grafana URL.

**Read `references/ephemeral-chain-flow.md` first.** It carries the architectural model: per-engineer workspace branch, path-based task isolation, the render/write/push/reconcile shape, and what's pending vs shipped today. The procedure below is the operational restatement.

1. **Identity + cluster check** — `seictl context`. Confirm `harbor`. Refuse on prod. Refuse if `~/.seictl/engineer.json` missing → route through First Run.
2. **Workspace check** — engineer's local platform repo checkout has `eng-<alias>-workspace` branch checked out. If absent, halt and surface the recovery (either `seictl onboard --apply` if onboarding never ran, or `git fetch && git checkout eng-<alias>-workspace` if it just isn't checked out locally). Refuse to switch branches if the working tree is dirty.
3. **Task name** — derive from the engineer's stated intent (one English sentence) or ask one question. Lowercase, k8s-namespace-safe. Becomes the path segment and the chain-id suffix.
4. **Render the chain** — `seictl chain up --image <ref> --validators N --name <task-name>` (no `--apply`). Capture the JSON envelope. Extract `data.manifests[]` and write each manifest to `clusters/harbor/eng/<alias>/<task-name>/`.
5. **Render the load (if requested)** — `seictl bench up --against <chain-id> --profile <profile> --duration <minutes>` (no `--apply`). Same shape; manifests join the same task directory.
6. **Pre-flight echo** — on the first side-effecting call of the session, show the engineer the resolved plan: chain-id, image digest, fleet size, task path under the workspace, what's about to be committed and pushed. Wait for confirmation.
7. **Commit + push** — `git add clusters/harbor/eng/<alias>/<task-name>/`, then a structured commit message (`feat(eng/<alias>): spin up <task-name> — chain-id=<id>, image=<digest-prefix>`), then `git push origin eng-<alias>-workspace`. Don't force-push.
8. **Wait for Flux** — poll `kubectl get kustomization eng-<alias>-workspace -n flux-system -o jsonpath='{.status.lastAppliedRevision}'` until it matches the pushed commit. 90s timeout; halt and surface `kubectl describe kustomization` on overrun.
9. **Wait for pods** — `kubectl get pods -n eng-<alias> -l sei.io/chain-id=<chain-id>` until all SND pods are Ready.
10. **Report** — print: chain-id, namespace, Grafana dashboard URL filtered by `sei.io/chain-id=<chain-id>` (target shape; the dashboard's chain-id-filter is pending per [Tide#25] item 4 — surface the unfiltered URL and the label until it lands), the teardown command (`git rm -r clusters/harbor/eng/<alias>/<task-name>/` + commit + push).

**What's pending and what to do meanwhile.** The convenience flag `seictl chain up --to-pr` (which would collapse steps 4 + 7 into one call) is on the seictl roadmap but not shipped. **Don't invoke `--to-pr` until it's released** — the skill performs the render → write → commit → push manually using `seictl` (render-only) + `git`. When `--to-pr` ships, this procedure compresses; until then the manual shape is the contract.

See `references/ephemeral-chain-flow.md` for the full architectural detail.

## Procedure: direct `--apply` benchmark (escape hatch, not the default)

The skill's default for "run a benchmark" is the GitOps flow above — render `seictl chain up` + `seictl bench up` (no `--apply`), write both to the engineer's workspace path, push. **This `--apply` procedure exists only for the rare case where an engineer explicitly opts out of git** — typically a CI debug session, or a one-shot run they don't want in the workspace branch's history. The skill does not volunteer this path. It only takes it when the engineer asks twice.

**Steer first.** Before running any of the steps below, the skill asks: "I can do this through the GitOps flow on your workspace branch (audit trail, Flux reconciles, `git rm` to tear down) — do you want that, or do you specifically need a direct-apply run with no git history?" If the engineer wants GitOps, route to the headline procedure above. **Only proceed below on explicit confirmation that GitOps is not what they want.**

1. **Identity check** — verify `~/.seictl/engineer.json` exists. If not, route through First Run above.
2. **Cluster check** — invoke `seictl context` and verify cluster is `harbor`. Refuse on prod.
3. **Image resolution** — engineer provided an image ref. seictl resolves to immutable digest internally; surface failures cleanly.
4. **Ask up to 3 questions**, in order, only when defaults would surprise:
   - "What are you testing? (one sentence — goes in chain ID name)"
   - "Fleet size: small (4 validators), medium (10), large (21)? [s]"
   - "Duration in minutes (1–240)? [30]"
5. **Pre-flight echo** — show the engineer the resolved invocation: chain ID (`bench-<alias>-<name>`), image digest, fleet size, duration, S3 results path, **and a reminder that this run will not appear in the workspace branch's git history**. Wait for confirmation.
6. **Invoke** — `seictl bench up --image <ref> --name <name> --size <size> --duration <duration> --apply`. seictl renders templates, applies via kubectl.
7. **Report** — print the chain ID, S3 results path, and the `seictl bench list` + `seictl bench down` follow-up commands.

See `references/intent-benchmark.md` for the full conversation tree, default selection rationale, and the autobake-derived templates that drive the fleet shape.

## Procedure: troubleshooting (manual; no `seictl diagnose` verb in v1)

Engineer says "X is stuck" or "diagnose seinode foo." There's no automated `seictl seinode diagnose` in v1 — the skill walks the engineer through the manual `kubectl`-driven flow documented in `references/troubleshooting-seinode.md`.

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

This skill doesn't maintain its own per-run state — `seictl` does. The skill is stateless between invocations: every cluster-facing verb starts with a fresh `seictl context` call to establish ground truth. The engineer's identity lives at `~/.seictl/engineer.json` (managed by `seictl`), and active resources live in the cluster (queryable by `seictl bench list`).

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
