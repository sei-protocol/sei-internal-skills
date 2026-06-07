---
name: harbor-dev
category: engineer-self-service
model: claude-opus-4-8
description: "Use when an engineer needs ephemeral Sei chains, RPC fleets, benches, comparative benches, or namespace teardown on the harbor EKS dev cluster — 'spin up a chain', 'start a chain', 'ephemeral chain', 'give me N validators', 'add an RPC fleet', 'attach RPC nodes', 'tear down my chain', 'what's running in my namespace', 'compare PR X to main', 'bench latest sei-chain', 'onboard me', 'set me up on harbor', '/harbor-dev'. Also fires on first-time engineer setup against the harbor cluster. Anti-triggers: NEVER for production clusters — harbor is dev-only; NOT for sei-k8s-controller code changes; NOT for autobake nightly cron changes; NOT for chaos testing (use chaos-suite); NOT for cross-tenant work — operates only on the caller's eng-<alias> namespace. For multi-component design work, use /council or /coral."
---

# harbor-dev

Engineer-facing interface to Sei platform infrastructure on the **harbor** EKS cluster. The engineer describes what they want; intent translates into `seictl nd` invocations against their namespace. The engineer doesn't need to remember the SeiNodeDeployment field set, what kustomize replacements look like, or which preset wires the rpc peer selector — `seictl` and this skill carry that.

This is the conversational layer over `seictl nodedeployment` (sei-protocol/seictl, v0.0.51+). It mirrors `kubectl`-shaped semantics — apply, get, list, watch, delete — driven by preset names instead of hand-rolled YAML.

## Guardrails

Operate against **harbor**. Engineers don't have prod kubeconfig contexts locally — the auth boundary enforces the separation, no duplication needed here.

The hard rules:

1. **Cluster must be harbor.** `kubectl config current-context` confirms; refuse on prod outright.
2. **Namespace is `eng-<alias>`.** Every `seictl nd` invocation passes `-n eng-<alias>`. Cross-namespace work is out of scope; the agent doesn't operate on shared infra.
3. **Scope echo on first side-effecting verb.** Echo cluster + namespace + preset + chain-id + image digest before the first `seictl nd apply`. Wait for confirmation.
4. **Refuse-and-surface, don't auto-remediate.** Where pre-flight has an in-band recovery (write a kubeconfig), run it. Where the recovery is out-of-band (SSO login, EKS access entry, onboarding PR merge), surface the next step and halt. Never silently work around a missing prereq.
5. **Speak as the platform expert.** Surface what's happening, what's wrong, what to do. The engineer never needs to know an instruction layer exists — phrases like "per skill protocol," "as my instructions say," "halting per procedure," or "I was told to check X" are leaks. Report findings authoritatively: ✓ "Halt — gates 4 and 5 failed. Gate 4: kubectl gets Forbidden on eng-fromtherain. Gate 5: namespace doesn't exist yet. Recovery for each:…" ✗ "Halt per skill protocol." Same rule for halt-and-recover messages, plan echoes, and post-action reports — name the cause and the next action; never the rule book.
6. **Never enable snapshot generation on an eng-workspace SND.** `spec.template.spec.fullNode.snapshotGeneration` stays unset. Eng-workspace chains are ephemeral consumers, not snapshot publishers; enabling generation pollutes `harbor-sei-snapshots` with non-canonical state and disables seid pruning (data volume grows unbounded). If the rendered CR carries `snapshotGeneration` from a stale `--set` or a hand edit, strip it before opening the PR. Publishing is the snapshot-publisher workload's job.
7. **Never set `spec.template.spec.sidecar` overrides by default.** seictl does not emit this field; if it appears in the rendered CR, it came from a hand edit or stale `--set` and should be stripped. The seictl sidecar image is wired by sei-k8s-controller from cluster config — overriding it pins a specific seictl/sidecar version and obscures the failure mode when reproducing a seid bug. The single legitimate use is **testing a platform / seictl / sidecar change** (not a sei-chain change). When the engineer's intent is sei-chain testing, the field must be absent.

## Mental model

You operate against the **harbor cluster** (eu-central-1 EKS). It runs the **sei-k8s-controller** which watches `SeiNode` and `SeiNodeDeployment` CRs and reconciles them into StatefulSets, PVCs, Services, and HTTPRoutes.

**Where you work:** `eng-<alias>` namespace, registered to the engineer via a one-time PR against `sei-protocol/platform`. The namespace is the isolation boundary.

**Three ServiceAccounts in `eng-<alias>`, each scoped to one purpose:**

| SA | Runs as | k8s authority | AWS authority (via Pod Identity) |
|---|---|---|---|
| `<alias>` | Flux reconciler for the engineer's path in `harbor-engineering-workspace` | Narrow namespace-scoped Role: snd/sn/jobs/configmaps CRUD, derived resources read-only | None |
| `engineer-service-account` | Engineer-launched workloads (seiload Jobs, scripts, custom tooling) | Namespace-admin via `RoleBinding` to built-in `admin` `ClusterRole` | `aws_iam_policy.engineer`: `s3:PutObject` on `harbor-validation-results/eng-<alias>/*` (auto-scoped via session tag), ECR auth + sei-chain image read |
| `seid-node` | SeiNode StatefulSet pods (sei-k8s-controller-managed) | None (default SA permissions) | `aws_iam_policy.seid_node`: snapshot read, genesis r/w, EC2 `DescribeInstances` |

**Two repos:**

- `sei-protocol/platform` — tenant registration (one-time PR per engineer) and platform-wide infrastructure.
- `sei-protocol/harbor-engineering-workspace` — engineer workloads at `engineers/<alias>/<task>/`, reconciled by the per-engineer Flux Kustomization. The agent opens task PRs here on the engineer's behalf for chain spinup; engineers also push directly when the workload is theirs to author manually (long-lived archive nodes, shared profiles).

**Default for engineer-facing intents — render → PR → Flux applies.** The team is GitOps-opinionated: every engineer side effect on the cluster goes through a PR against `sei-protocol/harbor-engineering-workspace`, never direct apply. The agent renders the SND CR via `seictl nd apply --dry-run` (output captured as JSON, converted to YAML via `yq -P`), writes it to `engineers/<alias>/<task>/snd-<id>.yaml`, opens the PR, surfaces the URL, and halts. The engineer reviews and merges; Flux reconciles within ~60s. After merge, the agent watches via `seictl nd watch <id> --until=Ready -n eng-<alias>` to report when pods are healthy. The CR carries its own provenance via labels and annotations; `kubectl get snd -n eng-<alias>` is the authoritative live view post-merge. Direct `seictl nd apply` (server-side apply, no PR) is an explicit escape hatch — see "Escape hatch" in the procedure below; never the default.

See `references/onboarding-pr.md` for the one-time tenant-registration flow, `references/ephemeral-chain-flow.md` for the headline daily-driver procedure, `references/seictl-cli.md` for the `nd` verb tree, `references/harbor-cluster.md` for cluster facts.

## Pre-flight (run at session start, before any side-effecting action)

Pre-flight is a sequenced ramp from "fresh laptop" to "ready to apply against eng-<alias>." Each gate either passes (continue), fails with an in-band recovery that runs through to completion, or fails with an out-of-band recovery to surface and halt on. Run gates in order; halt on first failure.

| # | Gate | Detect with | If missing → |
|---|---|---|---|
| 1 | `seictl ≥ v0.0.51` on PATH | `command -v seictl` returns 0; `seictl nodedeployment --help` exits 0 and lists `--genesis-override` (added v0.0.51 alongside peer auto-wire from v0.0.43+) | See `references/preflight.md#install-seictl` for the platform-specific install pipeline (prebuilt binaries from GitHub releases; build-from-source fallback when newer than latest release). **Not** via `brew` (no tap) and **not** via `go install` (source-tree build flags required). Halt until `command -v seictl` succeeds. |
| 2 | AWS SSO session active for the engineer's chosen profile | `aws sts get-caller-identity --profile <profile>` returns 0, where `<profile>` is resolved per the profile-detection flow below. After resolution, **echo the assumed identity** (`Arn` from the response) so the engineer sees what's about to act on the cluster. | If session is expired: surface `aws sso login --profile <profile>`; halt. If no profile is configured: route through the canonical Sei SSO session in `references/preflight.md` (Gate 3); halt. See "Profile detection" below for the resolution flow. |
| 3 | harbor kubeconfig context exists | `kubectl config get-contexts -o name` lists `harbor` (or the EKS ARN form) | Run `aws eks update-kubeconfig --name harbor --region eu-central-1 --profile <chosen>` directly (using the profile resolved at gate 2); re-check; continue. |
| 4 | kubectl can reach harbor with engineer-side reach | `kubectl auth can-i list seinodedeployments -n eng-<alias> --context=harbor` returns `yes` | EKS access entry not granted, or scoped read-only. Surface "ask the platform team via `#harbor-onboarding` with your AWS principal ARN"; halt. |
| 5 | Namespace `eng-<alias>` reconciled | `kubectl get namespace eng-<alias>` returns 0 | The engineer hasn't been onboarded yet, or the onboarding PR hasn't merged. Route to **First Run** below to open the PR; otherwise surface the open PR URL and offer to poll until the namespace appears (~60s post-merge). |

Once all five pass, cache the pass for the session — subsequent verbs skip the gates unless a halt condition (SSO expiry, kubectl context drift) triggers a targeted re-check.

For deep detail per gate (recovery commands, edge cases, the full new-engineer walk-through, mid-session drift handling), see `references/preflight.md`.

### Profile detection (gate 2)

Don't hardcode a profile name; engineers configure their own.

1. **If `$AWS_PROFILE` is set in the environment**, respect it as an explicit choice. Validate via `aws sts get-caller-identity --profile $AWS_PROFILE`. Echo the resolved Arn.
2. **Otherwise list the configured profiles** with `aws configure list-profiles` and branch on the count:
   - **Zero profiles** → route through profile setup using the canonical Sei SSO session in `references/preflight.md` (Gate 3); halt until a profile exists.
   - **Exactly one profile** → use it. Echo `"Using AWS profile <name> (only one configured)"`.
   - **Multiple profiles** → present the choice. Default-suggest `sei` if it's among them.
     > I'll use this AWS profile to authenticate kubectl + observe your harbor cluster resources. Which profile?
     > - `sei` (suggested)
     > - `<other>`
     > - `<other>`

The chosen profile is the session's profile — every downstream `aws ...` invocation runs with `--profile <chosen>`. Persist the choice for the session (e.g., shell-prefix every Bash call with `AWS_PROFILE=<chosen>` if not exported in the parent shell).

## First Run (the recovery for pre-flight gate 6)

When pre-flight gate 6 fails (`eng-<alias>` namespace doesn't exist), enter First Run. Gates 1–5 have passed — seictl, yq, SSO, kubeconfig, and EKS access entry are in place.

Onboarding is one PR against `sei-protocol/platform` adding three files. After merge, run a targeted `terraform apply`. Both pieces complete in under five minutes.

**Files the PR adds:**

| Path | Action |
|---|---|
| `clusters/harbor/engineers/<alias>/kustomization.yaml` | New. Per-engineer overlay. Mirrors the most recent prior onboarding PR; only the `alias=<alias>` literal differs. |
| `clusters/harbor/engineers/kustomization.yaml` | Modified. Adds `- <alias>` to `resources`. |
| `terraform/aws/189176372795/eu-central-1/harbor/engineers/<alias>.tf` | New. Two `eks-pod-identity` module instances for the engineer's `seid-node` and `engineer-service-account` SAs. Mirror the prior engineer's file with substring replacement of the alias. |

**Procedure:**

1. **Prompt for the alias** — don't silently use `$USER`. Default the prompt to `$USER` lowercased. Validate the response against `^[a-z]([a-z0-9-]{0,28}[a-z0-9])?$`. **Then check uniqueness** with `kubectl get namespace eng-<alias>` — if it returns 0, the alias is taken; halt with "this alias is taken; pick another or contact the platform team if it's yours." Don't attempt partial-state recovery (that's a separate runbook). Continue only when the alias is free.
2. Use [sei-protocol/platform#587](https://github.com/sei-protocol/platform/pull/587) (the fromtherain re-onboard) as the diff template — `gh pr diff 587 --repo sei-protocol/platform`. Branch: `feat/engineers-<alias>-onboard`.
3. Render the three platform-repo files (`gh pr diff` on the prior PR + substring replace on the alias).
4. Open the **platform-repo PR**. Title: `feat(harbor/engineers): onboard <alias>`.
5. Open the **workspace-repo sibling PR** against `sei-protocol/harbor-engineering-workspace`: branch `feat/onboard-<alias>`, scaffold `engineers/<alias>/kustomization.yaml` with `resources: []`. Without this, the per-engineer Flux Kustomization fails reconcile post-merge with `path not found: ./engineers/<alias>`.
6. Surface both PR URLs and halt:
   > Onboarding opened in two PRs:
   > - Platform: `<platform-url>`
   > - Workspace: `<workspace-url>`
   >
   > Merge the workspace PR first (or both within seconds). After the platform PR merges, Flux reconciles namespace + RBAC + Flux watcher in ~60s. Then run `AWS_PROFILE=<chosen> terraform apply -target=module.engineers` from `terraform/aws/189176372795/eu-central-1/harbor/` to land the Pod Identity associations. (`<chosen>` = the AWS profile resolved at gate 2.)
7. After merge, poll `kubectl get namespace eng-<alias>` until it returns 0.
8. Run `terraform plan -target=module.engineers -out=tfplan` and confirm `Plan: 6 to add, 0 to change, 0 to destroy`. Apply with `terraform apply tfplan`. `Resources: 6 added` confirms.
9. Gate 5 passes. Chain-spinup requests render via the skill, land in `harbor-engineering-workspace` PRs, and reconcile via Flux on merge. Pods running as `engineer-service-account` get S3 write to their own namespace prefix; SeiNode pods running as `seid-node` get snapshot read + peer discovery.

See `references/onboarding-pr.md` for the complete file content, base-layer details, and PR body template.

## What you can do

Every engineer-facing intent maps to a `seictl nd` verb against `eng-<alias>`. **Convention for every example below:** the namespace flag `-n eng-<alias>` is implicit and always present.

| Engineer says | Skill maps to |
|---|---|
| "Onboard me" / "set me up on harbor" / "I'm new" | First Run — open the platform-repo PR (see above + `references/onboarding-pr.md`). |
| "Spin up a chain of N validators with image X" / "start a chain" / "give me an ephemeral chain" | **PR-based** (see Procedure: spin up an ephemeral chain). Render via `seictl nd apply --dry-run -n eng-<alias>` (JSON to stdout) → `yq -P` to YAML → write to `engineers/<alias>/<task>/snd-<chain-id>.yaml` in `harbor-engineering-workspace` → commit → push → open PR. After merge, Flux applies; `seictl nd watch <id> --until=Ready -n eng-<alias>` reports liveness. |
| "Add an RPC fleet to chain X" / "attach RPC nodes" | Same render → PR shape with `--preset rpc --chain-id <same-id>`. Land in the same task dir as a sibling file (`snd-<id>-rpc.yaml`). Either as additional commits to an open chain PR or a follow-up PR. The `rpc` preset auto-wires `peers[0].label.selector.sei.io/chain=<chain-id>`, so the same `--chain-id` as the genesis chain gets the fleet pointing at it. |
| "What's running in my namespace" / "what chains do I have" | `seictl nd list -n eng-<alias>` (yaml default; `-o name` for short, `-o jsonpath=...` for one-shot field reads). |
| "Show me chain X" / "what's the status of X" | `seictl nd get <name> -n eng-<alias>` (full CR including `.status.phase`, `.status.endpoints`, `.status.perPodServices`). |
| "Tear down chain X" | `git rm -r engineers/<alias>/<task>/` **and** remove `<task>` from `engineers/<alias>/kustomization.yaml`'s `resources:` list (Kustomize fails to render with a missing-resource entry). Commit → push → merge. Flux prunes the SND on next reconcile, which cascades to child SeiNodes / pods / PVCs per k8s deletion propagation. See `bench:teardown` recipe in `references/cluster-inspection-recipes.md` for the bench-specific variant. |
| "Run a load test against chain X" / "bench it" / "stress test chain X" | **PR-based bench** (see Procedure: spin up a load test). Live-fetch chain rpc per-pod URLs, substitute into the profile JSON, render Job + ConfigMap from the templates in `references/sei-load-bench.md`, open a PR against `harbor-engineering-workspace` at `engineers/<alias>/bench-<RUN_ID>/`. Merge → Flux applies → seiload runs → uploader sidecar pushes results to S3. |
| "Compare PR 3399 to main on sei-chain" / "bench A against B" / "diff the perf of these two commits" | **PR-based comparative bench** (see Procedure: comparative bench). Renders two ephemeral chains (each running its own seid image) + two sei-load Jobs (identical profile + duration) into a single PR. After merge, watches both chains in parallel, polls both Jobs to terminal, fetches both reports from S3, and surfaces a side-by-side metrics table (TPS / latency / success rate / errors). Lives at `engineers/<alias>/compare-<COMPARE_RUN_ID>/`. |
| "Where am I" / "what cluster am I on" / "who am I" | `kubectl config current-context` + `aws sts get-caller-identity --profile <chosen>` (the gate-2 profile). (No dedicated `seictl context` verb in this surface.) |

**Override and composition:**

- Discrete flags (`--chain-id`, `--image`, `--replicas`) override preset defaults.
- `--set <dotted.path>=<value>` (repeatable) does strategic-merge overrides on the SND spec; wins over discrete flags on collision. Maps merge per-key, lists replace wholesale. Example: `--set spec.template.spec.image=ghcr.io/...:abc123`.
- `--dry-run` on `apply` runs server-side-apply in dry-run mode and emits the would-be-applied CR without persisting. Right shape for "show me what this would do" before committing.

## Post-merge reconciliation

Force-reconcile proactively after a relevant PR merges; don't wait for Flux's natural poll.

**Trigger:** any PR merge the agent helped open against `sei-protocol/platform` or `sei-protocol/harbor-engineering-workspace`, OR when the engineer says "merged" / "I merged X". Don't wait for the engineer to ask.

**Preferred:**

```sh
flux --context harbor reconcile kustomization flux-system --with-source -n flux-system
```

**Fallback** (only when `flux` isn't available):

```sh
kubectl --context harbor -n flux-system annotate kustomization flux-system \
  reconcile.fluxcd.io/requestedAt="$(date +%s)" --overwrite
```

**Verify** the merge commit SHA landed before proceeding to verbs that depend on the new state:

```sh
kubectl --context harbor -n flux-system get kustomization flux-system \
  -o jsonpath='{.status.lastAppliedRevision}'
```

## Procedure: spin up an ephemeral chain (the headline — PR-based)

Engineer says "spin up a chain of 4 validators with seid sha=abc, then add an RPC fleet." This is the daily-driver flow. The skill renders SND CRs via `seictl nd apply --dry-run`, writes them to `engineers/<alias>/<task>/` in `harbor-engineering-workspace`, opens a PR. Engineer merges → Flux applies → agent watches each SND to Ready → reports endpoints.

**Read `references/ephemeral-chain-flow.md` first.** It carries the architectural context — preset taxonomy, what each preset wires automatically, the watch protocol, exit-code conventions. The procedure below is the operational restatement.

1. **Pre-flight** — if not already passed this session, run all five gates. Halt on first failure with the recovery surfaced.
2. **Resolve naming** — derive a chain ID from caller context, in priority order:
   1. **Explicit `--tag <slug>`** the engineer passes, if any.
   2. **Linear ticket** mentioned in the request → `harbor-<ticket-slug>` (e.g., `harbor-plt-327`).
   3. **sei-chain PR number** mentioned → `harbor-pr-<n>` (e.g., `harbor-pr-3399`).
   4. **commit SHA substring** mentioned → `harbor-<sha[:7]>` (e.g., `harbor-b7b4868`).
   5. **None of the above** → ask **one** question for an explicit tag. Don't silently fall back to a timestamp — anonymous IDs in Grafana / logs / cluster state can't be tied back to the work they served.

   The resolved ID becomes both the SND name and `--chain-id`. Lowercase, k8s-namespace-safe (regex `^[a-z]([a-z0-9-]{0,28}[a-z0-9])?$`). For "chain X with RPC," the genesis SND is `<id>` and the RPC SND is `<id>-rpc`.
3. **Resolve image** — sei-chain (`seid`) image. **Required input** (PR / commit / branch / explicit `--image`); never silently default. Resolve to a full SHA and verify the image is in registry per `references/image-resolution.md`. Surface the resolved digest in the plan echo so the engineer sees what they're about to run.
4. **Render** — `seictl nd apply <id> --preset genesis-chain --chain-id <id> --image <ref> [--replicas N] -n eng-<alias> --dry-run` emits the post-apply CR as JSON on stdout. Pipe through `yq -P` to convert to YAML. Repeat for the RPC fleet (if requested) with `--preset rpc --chain-id <same-id>` against name `<id>-rpc`.
5. **Plan echo & confirm** — on the first side-effecting call of the session, show the engineer: cluster (harbor), namespace (`eng-<alias>`), task path under workspace repo, SND names, chain-id, image digest, replica count, what's about to be committed and pushed. Wait for confirmation.
6. **Write to workspace repo** — fresh clone of `sei-protocol/harbor-engineering-workspace` (or reuse a session-scoped clone). Write to `engineers/<alias>/<task>/snd-<id>.yaml` (and `snd-<id>-rpc.yaml` if RPC). Update `engineers/<alias>/<task>/kustomization.yaml` listing both as resources. Append `<task>` to `engineers/<alias>/kustomization.yaml`'s `resources:` list if not already present.
7. **Commit + push** — branch `feat/eng-<alias>-<task>`. Commit message: `feat(eng/<alias>): spin up <task> — chain-id=<id>, image=<digest-prefix>`. Push.
8. **Open the PR** — title: `feat(eng/<alias>): spin up <task>`; body lists chain-id, image digest, preset(s), expected endpoints. `gh pr create --repo sei-protocol/harbor-engineering-workspace --base main`.
9. **Surface and halt** — engineer reviews and merges. Surface the PR URL with: "after merge, Flux reconciles in ~60s; ping me to watch the SND to Ready and report endpoints."
10. **After merge — watch** — `seictl nd watch <id> --until=Ready --timeout=15m -n eng-<alias>` for genesis, then the same for `<id>-rpc`. NDJSON stream of SND events; exits 0 when `.status.phase=Ready`. Halt on non-zero (`metav1.Status` on stderr — `jq -r .reason` discriminates Timeout vs terminal Failed phase vs API failure).
11. **Report** — use the canonical inspection recipes from `references/cluster-inspection-recipes.md` rather than inferring jsonpath at runtime. Recipe #1 returns the chain's RPC endpoints (target these for any load tools — never validators). Recipe #4 lists the chain's full SND set (validator + RPC) with phase + readiness in one shot. Plus teardown: `git rm -r engineers/<alias>/<task>/` and remove the `<task>` entry from `engineers/<alias>/kustomization.yaml`'s `resources:` list, then commit → push → merge (Flux prunes the SNDs and cascades the deletion to pods/PVCs).

### Escape hatch: direct `seictl nd apply` (rare; engineer asks twice)

If an engineer specifically asks to bypass the PR loop for a one-shot debug session and confirms they understand the result won't be in git history, fall through to direct apply: `seictl nd apply <id> --preset genesis-chain --chain-id <id> --image <ref> -n eng-<alias>` (server-side applies; emits the post-apply CR on stdout). Then `seictl nd watch <id> --until=Ready -n eng-<alias>`.

**Steer first.** Before running, ask: "I can do this through the GitOps PR flow (audit trail, Flux reconciles, `git rm` to tear down) — do you want that, or do you specifically need a direct-apply run with no git history?" Only proceed below on explicit confirmation. The agent does not volunteer this path.

## Procedure: spin up a load test (PR-based)

Engineer says "load chain X with seiload" or "bench against PR 3399 of sei-load." Sei-load runs as vanilla K8s — no CRD — so the rendering lives in this skill rather than `seictl nd`. The agent fills in placeholders, opens a PR against `sei-protocol/harbor-engineering-workspace`, and the engineer's per-engineer Flux Kustomization reconciles the Job on merge. An uploader sidecar in the Pod pushes the seiload report to S3 when seiload exits (success, failure, or timeout).

**Read `references/sei-load-bench.md` first.** It carries the manifest templates, the live profile substitution recipe, the upload-sidecar pattern, the S3 key convention, and the run-id determinism rule.

1. **Pre-flight** — five gates from `preflight.md`. Halt on first failure.
2. **Verify chain rpc SND is Ready** — the bench requires per-pod RPC URLs, which only populate after the rpc SND reconciles. `seictl nd get <chain-id>-rpc -n eng-<alias> -o jsonpath='{.status.phase}'` returns `Ready`. Halt and offer to poll if not.
3. **Resolve sei-load image** per `references/image-resolution.md` — required input, never silent default.
4. **Resolve profile** — default `nightly_evm_transfer`. Read profile JSON via `gh api ... --jq .content | base64 -d`.
5. **Resolve `<RUN_ID>`** — check for an existing PR branch matching `feat/eng-<alias>-bench-<bench-tag>-*`; reuse on match, else mint `<bench-tag>-<UTC-timestamp>`.
6. **Live-fetch per-pod RPC URLs** — `seictl nd get <chain-id>-rpc -o json | jq '.status.endpoints.evmJsonRpc[1:]'`. Substitute `__SEI_CHAIN_ID__` and `__RPC_ENDPOINTS__` into the profile JSON.
7. **Plan echo & confirm** — chain-id, rpc SND name, sei-load image (resolved digest + source PR/commit), profile, duration, `<RUN_ID>`, target path, expected S3 key. Wait for confirmation.
8. **Render the manifests** — Job + ConfigMap + kustomization.yaml in `engineers/<alias>/bench-<RUN_ID>/`. Append `bench-<RUN_ID>` to `engineers/<alias>/kustomization.yaml`'s `resources:` list.
9. **Commit + push** — branch `feat/eng-<alias>-bench-<RUN_ID>`. Message: `feat(eng/<alias>): bench <RUN_ID> against <chain-id> (image=<digest-prefix>)`.
10. **Open the PR** against `sei-protocol/harbor-engineering-workspace`. Surface URL: "Merge to start the bench. Job runs `<DURATION>` minutes; results land at `s3://harbor-validation-results/eng-<alias>/<profile>/<RUN_ID>/report.log`."
11. **Report observation recipes** — surface three named recipes from `references/cluster-inspection-recipes.md`: **live-tail** (`bench:live-tail`), **terminal-check** (`bench:terminal-check` — the kubectl jsonpath + grep recipe; jsonpath filter expressions don't support `||` so the recipe iterates on the host side), and **teardown** (`bench:teardown` — `git rm` of `engineers/<alias>/bench-<RUN_ID>/` plus the `kustomization.yaml` entry removal).

## Procedure: comparative bench (two images, side-by-side)

Engineer says "compare PR 3399 to main on sei-chain" or "bench latest sei-chain against commit b7b4868." Renders two ephemeral chains (each running its own seid image) + two sei-load Jobs (identical profile + duration + replica counts) into a single PR. After merge, both chains spin up in parallel; both benches run in parallel; both reports land in S3 paired by `<COMPARE_RUN_ID>`. The agent fetches both, extracts canonical metrics, and surfaces a side-by-side table.

**Read `references/comparative-bench.md` first.** It carries the four-subdir layout, naming convention, post-merge parallel-watch pattern, S3 fetch + metric-extraction recipe, fallback rendering, and halt conditions specific to the comparative shape.

1. **Pre-flight** — five gates from `preflight.md`. Halt on first failure.
2. **Resolve both images** — image A (engineer's input — PR / commit / branch) and image B (baseline — engineer-supplied; no silent default). Per `references/image-resolution.md` for both. Halt if either is missing in registry; do not render half a comparison.
3. **Resolve compare-tag** — Linear ticket → `<imageA-tag>-vs-<imageB-tag>` → explicit `--tag`. Validate the chain-tag length budget (≤24 chars, since the longest auto-suffix is `-a-rpc`).
4. **Resolve `<COMPARE_RUN_ID>`** — same shape as single-bench. Branch is `feat/eng-<alias>-compare-<compare-tag>-*`; reuse on match, else mint.
5. **Resolve profile + duration** — defaults `nightly_evm_transfer` / 10 min. Both sides use identical values; the skill enforces this.
6. **Verify no SND name collisions** — four planned names (`<chain-tag>-{a,b}` + `<chain-tag>-{a,b}-rpc`) must all be `NotFound` in `eng-<alias>`.
7. **Plan echo & confirm** — both image digests + source refs, both chain-ids, profile, duration, `<COMPARE_RUN_ID>`, target workspace path, both expected S3 keys, total estimated runtime (`<DURATION> + ~6 min`).
8. **Render the four sub-dirs** (chain-a, chain-b, bench-a, bench-b) per the templates in `references/comparative-bench.md`. Append `compare-<COMPARE_RUN_ID>` to `engineers/<alias>/kustomization.yaml`'s `resources:` if not already present.
9. **Verify config parity** — read both rendered bench ConfigMaps back, diff the substituted JSONs; abort the render if they differ on anything except `seiChainId` and `endpoints`.
10. **Commit + push** — branch `feat/eng-<alias>-compare-<COMPARE_RUN_ID>`. Message: `feat(eng/<alias>): compare <imageA-tag> vs <imageB-tag> (<COMPARE_RUN_ID>)`.
11. **Open the PR** — surface URL: "Merge to start. Both chains spin up in parallel (~5 min), both benches run for `<DURATION>` minutes, then I'll fetch the reports and surface the comparison."
12. **After merge — watch chains in parallel** — `seictl nd watch <chain-tag>-a` and `<chain-tag>-b` concurrently (`--until=Ready --timeout=15m`). Halt the whole comparison if either reaches Failed; the comparison is invalid against half a setup.
13. **Watch RPC fleets in parallel** — same shape against `<chain-tag>-a-rpc` and `<chain-tag>-b-rpc`.
14. **Poll both bench Jobs to terminal** — single loop that checks both `seiload-<COMPARE_RUN_ID>-{a,b}` for `Complete=True` or `Failed=True`. Deadline: `<DURATION> * 60 + 660` seconds.
15. **Fetch + render** — pull both reports via in-cluster `kubectl run` under `engineer-service-account` (the engineer's local SSO profile lacks `s3:GetObject` on the prefix). Locate sei-load's summary block in each report; surface both summaries verbatim under a delta table that highlights canonical metrics (TPS, P50/P99 latency, success rate, tx counts) with `better` / `worse` verdicts. On summary-block miss, fall back to last-50-lines format with both S3 paths.
16. **Teardown guidance** — `git rm -r engineers/<alias>/compare-<COMPARE_RUN_ID>/` and remove the entry from `engineers/<alias>/kustomization.yaml`'s `resources:`. Flux prunes the four SNDs + two Jobs; child pods/PVCs cascade.

## Procedure: troubleshooting (manual)

Engineer says "X is stuck" or "diagnose snd foo." `seictl` has no diagnose verb — walk the engineer through the manual `kubectl`-driven flow documented in `references/troubleshooting-seinode.md`.

1. Read `.status.phase` via recipe #2 in `references/cluster-inspection-recipes.md`. If the SND is in `Failed` or stuck `Initializing`, recipe #3 returns the failed task name + error directly — don't try to re-derive the path.
2. Branch on phase:
   - **Pending** → check sei-k8s-controller pods + leader lease in `sei-system`
   - **Initializing** → read `.status.plan` for the failing PlannedTask; map task name to root cause (snapshot-restore → S3 / Pod Identity, configure-genesis → genesis URL, discover-peers → label selector, mark-ready → seid health)
   - **Running** → check seid logs, HTTPRoute routing, pod restarts
   - **Failed** → read `.status.plan.failedTaskDetail.error` (also lifted to stderr by `seictl nd watch` on terminal Failed)
3. Surface the matching `kubectl` invocations from `references/troubleshooting-seinode.md`.

## Procedure: read-only verbs (`get`, `list`)

Pure invocations. Skill calls `seictl nd <verb>` and surfaces the structured output to the engineer in plain English. No questions, no confirmation.

## Halt conditions

Stop and report to the user if:

- **kubectl context drifts mid-session** — engineer switched contexts in another terminal. Re-confirm before any side effect.
- **`seictl nd <verb>` exits non-zero** — surface the `metav1.Status` on stderr (`jq -r .reason && jq -r .message`). Do not retry silently.
- **Image digest resolution fails** — image not in ECR/GHCR or auth missing. Stop and surface the recovery command.
- **Image not in registry** — sei-chain CI may be behind. Surface the explicit retry command per the autobake race-guard pattern; don't loop silently.
- **`seictl nd watch` exits with `metav1.Status.reason=Timeout`** — chain hasn't reached the requested phase within `--timeout` (default 15m). Halt; surface `.status.plan.tasks[]` from the last NDJSON line for the engineer to inspect.
- **`seictl nd watch` exits on terminal Failed phase** — `.status.plan.failedTaskDetail.error` is on stderr; surface it and the failed task name. Don't auto-retry.
- **`seictl nd get` / `seictl nd watch` exits with `metav1.Status.reason=NotFound`** — the named SND doesn't exist in the namespace. The fix is to apply it (PR-based, or escape-hatch direct apply), not to retry. For a bench targeting `<chain-id>-rpc` that's `NotFound`, the chain has no rpc SND yet — surface the apply path (`seictl nd apply <chain-id>-rpc --preset rpc --chain-id <chain-id>`) and halt.
- **SND name collision in the namespace** — `kubectl get snd <name> -n eng-<alias>` returns an existing CR while the engineer's PR proposes a new one. Halt before opening the PR; surface the existing object's age and labels, and ask whether to pick a different chain-id, or `git rm` the existing object's manifest from the workspace repo first.
- **Workspace-repo task path collision** — `engineers/<alias>/<task>/` already exists. Don't silently overwrite. Halt and ask whether to reuse the dir (and add new files alongside the existing ones) or pick a different `<task>` name.
- **Chain rpc SND not Ready when rendering a bench.** The bench requires per-pod RPC URLs, which populate only after the rpc SND reconciles. Halt and offer to poll (`seictl nd watch <chain-id>-rpc --until=Ready -n eng-<alias>`) before continuing.
- **Per-pod RPC URLs absent despite phase=Ready.** Pre-Service-population race. Sleep 30s and retry once; halt with the SND's full status if still empty.
- **Comparative bench: one side reaches Ready, the other Failed.** The comparison is invalid against half a setup. Surface the failed side's `.status.plan.failedTaskDetail.error`; offer to teardown the Ready side via `git rm` against just that sub-dir + commit. Don't run the bench against half a comparison.
- **Comparative bench: config parity check fails post-render.** The two substituted profile JSONs differ on a field other than `seiChainId` / `endpoints`. Halt before push; the rendered manifests would produce a non-comparable result.
- **Comparative bench: chain-tag exceeds the 24-char budget** when the `-{a,b}-rpc` suffix is added. Surface the overflow and ask the engineer to pick a shorter tag.
- **Comparative bench: S3 GetObject fails on a report.** `NoSuchKey` means the upload sidecar didn't run — surface `kubectl logs -n eng-<alias> -l sei.io/compare-name=<COMPARE_RUN_ID>,sei.io/compare-side=<a|b> -c upload-results` to diagnose. `AccessDenied` means the engineer's profile lacks `s3:GetObject` (the engineer SA's IAM policy already covers it; the active profile is wrong).
- **PR push rejected (non-fast-forward)** — engineer or another agent pushed to the same branch. Don't force-push. Halt; surface `git pull --rebase origin <branch>` and let the engineer resolve.

## Reference index

| File | Scope |
|---|---|
| `preflight.md` | **Read this first on a new session or when an engineer is fresh.** Five-gate ramp from "fresh laptop" to "ready to apply," per-gate recovery, mid-session drift handling, full new-engineer walk-through |
| `onboarding-pr.md` | **Read this if the engineer is new.** The one-time tenant-registration PR shape. Canonical example: `clusters/harbor/engineers/fromtherain/kustomization.yaml`. What the base layer provides |
| `ephemeral-chain-flow.md` | **Read this if the engineer asks for a chain.** Preset taxonomy (`genesis-chain`, `rpc`), what each preset wires automatically, watch protocol, exit-code conventions |
| `seictl-cli.md` | Canonical `seictl nd` verb tree (regenerated from `seictl nd --help` periodically) |
| `seinode-crd.md` | Operations-load-bearing fields on `SeiNode` |
| `seinodedeployment-crd.md` | Operations-load-bearing fields on `SeiNodeDeployment`, including `.status.phase`, `.status.endpoints`, `.status.perPodServices`, `.status.plan` |
| `cluster-inspection-recipes.md` | **Canonical structured-extraction recipes.** Use these directly instead of inferring jsonpath at runtime — RPC endpoints (recipe #1, also resolves "target RPC, not validator"), phase + readiness, failed task, image drift, per-pod services, Flux Kustomization Ready |
| `sei-load-bench.md` | **Read this if the engineer asks for a load test or bench.** Job + ConfigMap templates with substitution markers, live profile-JSON substitution recipe, two-container upload pattern (seiload + amazon/aws-cli sidecar with `shareProcessNamespace`), S3 archival convention, run-id determinism on re-render |
| `comparative-bench.md` | **Read this if the engineer asks to compare two images.** Four-subdir layout (chain-a / chain-b / bench-a / bench-b), naming convention, per-side profile substitution, parallel post-merge watch sequence, S3 fetch + canonical-metric extraction with raw-tail fallback, comparison-output table format |
| `image-resolution.md` | **Canonical image-resolution recipes** for sei-chain (ECR) and sei-load (GHCR). PR/commit/branch input → full SHA → expected tag → registry probe → trigger + watch the build workflow if missing |
| `troubleshooting-seinode.md` | Phase-by-phase symptom → cause → inspection decision tree |
| `harbor-cluster.md` | CNI (Cilium), Istio + Gateway API, DNS, Flux topology, EKS access entries |
| `aws-dependencies.md` | S3 buckets (snapshots, genesis, results), Pod Identity status, ECR conventions |

When this skill drifts from `seictl nd`'s actual behavior, **`seictl nd --help` wins.** Reference files include a dated last-verified note per section to help spot drift.

## Permission pre-approval

Pre-approve in `.claude/settings.local.json` (user-specific, not committed):

```json
{
  "permissions": {
    "allow": [
      "Bash(seictl nd get:*)",
      "Bash(seictl nd list:*)",
      "Bash(seictl nd watch:*)",
      "Bash(seictl nodedeployment get:*)",
      "Bash(seictl nodedeployment list:*)",
      "Bash(seictl nodedeployment watch:*)",
      "Bash(kubectl config current-context:*)",
      "Bash(kubectl config get-contexts:*)",
      "Bash(kubectl get seinode:*)",
      "Bash(kubectl get seinodedeployment:*)",
      "Bash(kubectl get namespace:*)",
      "Bash(kubectl get pods:*)",
      "Bash(kubectl logs:*)",
      "Bash(kubectl auth can-i:*)",
      "Bash(aws sts get-caller-identity:*)",
      "Bash(gh auth status:*)",
      "Bash(git status:*)",
      "Bash(git log:*)",
      "Bash(git diff:*)"
    ]
  }
}
```

**Leave interactive** (never pre-approve):

- `seictl nd apply` (without `--dry-run`) — direct server-side apply; the escape-hatch path. Requires explicit confirmation per session.
- `seictl nd delete` — destroys a CR + propagates deletion to children; requires explicit confirmation. (Default teardown is `git rm` against the workspace-repo manifest, not this verb.)
- `gh pr create` — opens onboarding and chain-spinup PRs; requires explicit confirmation per PR.
- `git push` — pushes engineer-task branches to `harbor-engineering-workspace`; requires explicit confirmation.

`seictl nd apply --dry-run` is safe to pre-approve since it's render-only (returns the would-be-applied CR, no cluster mutation). Add `Bash(seictl nd apply * --dry-run:*)` to the allow list if a session does heavy rendering.

Use the `fewer-permission-prompts` skill against a real session transcript once the skill is in active use.

## State management

No per-run state is maintained here. Operation is stateless between invocations: every cluster-facing verb starts fresh. The engineer's identity is `eng-<alias>` namespace + EKS access entry — both managed by the cluster, not the agent. Active resources live in the cluster (queryable by `seictl nd list -n eng-<alias>`).

---

The verb table above reflects what `seictl nd --help` emits in v0.0.51+. Reference files are aligned to the shipped output shape (native CR on stdout, `metav1.Status` on stderr, NDJSON for watch). When this file disagrees with `seictl nd --help`, the CLI wins.
