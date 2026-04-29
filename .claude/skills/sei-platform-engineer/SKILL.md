---
name: sei-platform-engineer
description: "Engineer-facing interface to Sei platform infrastructure on the harbor EKS cluster. Translates natural-language intent into seictl invocations — provision benchmarks against candidate seid images, onboard new engineers, inspect cluster state. Trigger on 'spinup benchmark', 'run a benchmark', 'benchmark this image', 'onboard me', 'set me up on harbor', 'what's running on harbor', 'where am I on the cluster'. NOT for sei-k8s-controller code changes. NOT for autobake nightly cron changes. NOT for chaos testing (use chaos-suite). For multi-component design work, use /council or /coral."
---

# sei-platform-engineer

Engineer-facing interface to Sei platform infrastructure on the **harbor** EKS cluster. You describe what you want; the skill translates intent into `seictl` invocations. You don't need to know that SeiNode/SeiNodeDeployment CRDs exist, what Kustomize is, or where snapshots live in S3 — `seictl` knows. The skill maps engineer intent to the right invocation.

This is the conversational layer over `seictl` (sei-protocol/seictl). When MCP graduation happens, the same procedures become tool calls; the skill content is the contract today.

## Guardrails

This skill operates against **harbor**. Engineers don't have prod kubeconfig contexts locally — the auth boundary enforces the separation, the skill doesn't duplicate it. Before any side-effecting action:

1. **Identity check** — `~/.seictl/engineer.json` must exist before any `seictl bench` command. If absent, route through `seictl onboard` first.
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
- Today: `eng-<your-alias>` namespace. PR-driven onboarding (one-time), direct-apply ephemeral workloads (per benchmark).
- Soon (~1–2 weeks): same namespace gains quotas, NetworkPolicy, and admission policies via the personal-cells project. Your workflow doesn't change — only the security posture beneath it tightens.

**The split:**
- **Long-lived infra** (namespace, RBAC, ServiceAccounts) — Flux-managed via the platform repo. Onboarding adds your namespace via PR.
- **Ephemeral workloads** (benchmark validators, RPC fleet, seiload Job) — direct `kubectl apply` via `seictl bench up`. Tear down via `seictl bench down` or revert the onboarding PR for a clean wipe.

This mirrors how autobake works today: namespace + RBAC are Flux-managed; per-run resources are imperatively applied by a workflow.

See `references/harbor-cluster.md` for cluster facts, `references/interim-namespace-strategy.md` for the cells-forward labels we use today.

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

| Engineer says | Skill maps to |
|---|---|
| "Onboard me" / "set me up on harbor" / "I'm new" | `seictl onboard --alias <alias> [--apply]` |
| "Run a benchmark against image X" / "spinup benchmark" / "benchmark this image" | Ask up to 3 questions → `seictl bench up --image <ref> --name <name> [--size s\|m\|l] [--duration <minutes>] --apply` |
| "Tear down my benchmark" / "stop benchmark X" | `seictl bench down --name <name>` |
| "What benchmarks am I running" / "what's running" | `seictl bench list` |
| "Where am I" / "what cluster am I on" / "who am I" | `seictl context` |

## Procedure: spinup benchmark (the headline)

Engineer says "run a benchmark against image X." Skill ascertains the missing parameters via at most 3 questions, then invokes `seictl bench up`.

1. **Identity check** — verify `~/.seictl/engineer.json` exists. If not, route through First Run above.
2. **Cluster check** — invoke `seictl context` and verify cluster is `harbor`. Refuse on prod.
3. **Image resolution** — engineer provided an image ref. seictl resolves to immutable digest internally; surface failures cleanly.
4. **Ask up to 3 questions**, in order, only when defaults would surprise:
   - "What are you testing? (one sentence — goes in PR title and chain ID name)"
   - "Fleet size: small (4 validators), medium (10), large (21)? [s]"
   - "Duration in minutes (1–240)? [30]"
5. **Pre-flight echo** — show the engineer the resolved invocation: chain ID (`bench-<alias>-<name>`), image digest, fleet size, duration, S3 results path. Wait for confirmation on the first side-effecting call of the session.
6. **Invoke** — `seictl bench up --image <ref> --name <name> --size <size> --duration <duration>`. seictl renders templates, applies via kubectl.
7. **Report** — print the chain ID, S3 results path, and the `seictl bench list` follow-up command.

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
- **Engineer's onboarding PR isn't merged but they're trying to bench** — namespace doesn't exist yet. Stop and report. Don't auto-create.
- **Image digest resolution fails** — image not in ECR or auth missing. Stop and surface the recovery command.
- **Image not yet in ECR** — sei-chain CI may be behind. Surface the explicit retry command per the autobake race-guard pattern; don't loop silently.

## Reference index

| File | Scope |
|---|---|
| `seictl-cli.md` | Canonical command surface (regenerated from `seictl --help` periodically) |
| `intent-benchmark.md` | Full benchmark conversation tree + autobake-derived fleet conventions |
| `seinode-crd.md` | Operations-load-bearing fields (the 6 spec, 4 status fields you'll touch) |
| `seinodedeployment-crd.md` | Same discipline for the SND CRD |
| `troubleshooting-seinode.md` | Phase-by-phase symptom → cause → inspection decision tree |
| `harbor-cluster.md` | CNI (Cilium), Istio + Gateway API, DNS, Flux topology, EKS access entries |
| `aws-dependencies.md` | S3 buckets (snapshots, genesis, results), Pod Identity, ECR conventions |
| `interim-namespace-strategy.md` | Cells-forward-compatible labels we ship today; how cells will retrofit |
| `pr-conventions.md` | Branch naming, PR body shape, reviewer conventions for onboarding PRs |

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
      "Bash(kubectl get pods:*)",
      "Bash(kubectl logs:*)",
      "Bash(aws sts get-caller-identity:*)",
      "Bash(gh auth status:*)"
    ]
  }
}
```

**Leave interactive** (never pre-approve):

- `seictl bench up` — provisions resources; requires explicit confirmation
- `seictl bench down` — destroys resources; requires explicit confirmation
- `seictl onboard` — opens a PR; requires explicit confirmation

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
