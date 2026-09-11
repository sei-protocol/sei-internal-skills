---
name: harbor-dev
category: engineer-self-service
model: claude-opus-5
description: "Use when an engineer needs ephemeral Sei chains, RPC fleets, benches, comparative benches, or namespace teardown on the harbor EKS dev cluster — 'spin up a chain', 'ephemeral chain', 'give me N validators', 'add an RPC fleet', 'tear down my chain', 'what's running in my namespace', 'compare PR X to main', 'bench latest sei-chain', 'onboard me', 'set me up on harbor', 're-bootstrap a stuck node', 'state-sync a follower', 'migrate a node to giga store', 'giga chain', 'autobahn network', 'set an app.toml value', 'change block time', 'partition a validator', 'inject latency', 'run chaos on my chain', '/harbor-dev'. Also fires on first-time harbor setup. Anti-triggers: NEVER for production clusters — harbor is dev-only; NOT for sei-k8s-controller code changes; NOT for autobake nightly cron changes; NOT for the nightly chaos release gate (chaos on the engineer's own chain IS in scope); NOT for cross-tenant work — operates only on the caller's eng-<alias> namespace. Multi-component design: /council or /coral."
---

# harbor-dev

Engineer-facing interface to Sei platform infrastructure on the **harbor** EKS cluster. The engineer describes what they want; intent translates into `seictl network` / `seictl node` invocations against their namespace. The engineer does not need to remember the SeiNetwork / SeiNode field set, what kustomize replacements look like, or which preset wires the rpc peer selector. `seictl` and this skill carry that.

This is the conversational layer over `seictl network` + `seictl node` (sei-protocol/seictl, v0.0.72 or newer — that tag is the first carrying the resource flags). It mirrors `kubectl`-shaped semantics — apply, get, list, watch, delete — driven by preset names instead of hand-rolled YAML. A genesis chain is one `SeiNetwork`; an RPC fleet of N is N standalone `SeiNode` CRs (no fleet Kind, no `--replicas` on a node). A third tree, `seictl workflow`, is the imperative operational path for re-bootstrapping or migrating an *existing* node in place. It runs far less often than the declarative network/node bring-up, and it wipes the node's local chain state. See Guardrails and `references/seictl-cli.md`.

## Guardrails

Operate against **harbor**. Engineers do not have prod kubeconfig contexts locally — the auth boundary enforces the separation, no duplication needed here.

The hard rules:

1. **Cluster must be harbor.** `kubectl config current-context` confirms; refuse on prod outright.
2. **Namespace is `eng-<alias>`.** Every `seictl network` / `seictl node` / `seictl workflow` invocation passes `-n eng-<alias>`. Cross-namespace work is out of scope; the agent does not operate on shared infra.
3. **Scope echo on first side-effecting verb.** Echo cluster + namespace + preset + chain-id + image digest before the first `seictl network apply` / `seictl node apply`. Wait for confirmation.
4. **Refuse-and-surface, do not auto-remediate.** Where pre-flight has an in-band recovery (write a kubeconfig), run it. Where the recovery is out-of-band (SSO login, EKS access entry, onboarding PR merge), surface the next step and halt. Never silently work around a missing prereq.
5. **Speak as the platform expert.** Surface what's happening, what's wrong, what to do. The engineer never needs to know an instruction layer exists. Phrases like "per skill protocol," "as my instructions say," "halting per procedure," or "my instructions told me to check X" are leaks.

   Report findings authoritatively: ✓ "Halt — gates 4 and 5 failed. Gate 4: kubectl gets Forbidden on eng-fromtherain. Gate 5: namespace does not exist yet. Recovery for each:…" ✗ "Halt per skill protocol." Same rule for halt-and-recover messages, plan echoes, and post-action reports — name the cause and the next action; never the rule book.
6. **Never enable snapshot generation on an eng-workspace follower.** `spec.fullNode.snapshotGeneration` stays unset on a SeiNode. Eng-workspace chains are ephemeral consumers, not snapshot publishers; enabling generation pollutes `harbor-sei-snapshots` with non-canonical state and disables seid pruning (data volume grows unbounded). If the rendered CR carries `snapshotGeneration` from a stale `--set` or a hand edit, strip it before opening the PR. Publishing is the snapshot-publisher workload's job.
7. **Never render a `--genesis-override` key you cannot source from a real genesis.** Validation covers only the module segment; a wrong deeper field passes render, apply, and assembly silently, then crash-loops every node at InitChain. Read the module's actual genesis first (`/genesis` on 26657, or sei-config's embedded chains). Never guess keys from upstream-Cosmos docs (sei forks diverge; see the sharp-edge note in `references/seictl-cli.md`).
8. **Never set `spec.sidecar` overrides by default.** seictl does not emit this field; if it appears in the rendered SeiNode, it came from a hand edit or stale `--set` — strip it. sei-k8s-controller wires the seictl sidecar image from cluster config. Overriding it pins a specific seictl/sidecar version and obscures the failure mode when reproducing a seid bug. The single legitimate use is **testing a platform / seictl / sidecar change** (not a sei-chain change). When the engineer's intent is sei-chain testing, the field must be absent.
9. **Two paths wipe a node's chain data — gate both.** `seictl workflow state-sync` is the destructive **paved road**; a mutating `seictl task submit` is the destructive **escape hatch**. Neither is ever the default, and the agent volunteers neither.
   - **`seictl workflow state-sync`** re-bootstraps an existing node by wiping its local chain state (an `rm -rf` on that node's data). It may add an irreversible `--migration GigaStore --backend <pebbledb|rocksdb>` store change; both tokens go together, never `--migration` alone. Get explicit engineer sign-off before the non-dry-run apply, verify the target node against the live cluster first, and `--dry-run` to inspect. For any shared or long-lived `pacific-1`/`atlantic-2` follower, escalate to the owner — never wipe on agent initiative. Never commit a workflow CR to the Flux workspace repo (a one-shot, spec-immutable request object; force-delete recovery fights Flux). Full gate in `references/seictl-cli.md` → `seictl workflow state-sync`.
   - **`seictl task submit`** POSTs a raw task straight to one pod's sidecar, and the accepted types include `reset-data`. Submitted that way the wipe runs with **none** of the recipe's protections. It gets no `mark-not-ready` hold, no `stop-seid` first, no ordering, and no adoption pointer telling the controller that a workflow holds the node. It is strictly more dangerous than the paved road, not a lighter-weight version of it. Prefer `workflow state-sync` for anything the recipe covers; a mutating `task submit` requires explicit sign-off naming node, namespace, and task type. `task get` / `task list` are reads and safe; full gate in `references/seictl-cli.md` → `seictl task`.
10. **A day-2 `spec.configValues` or `spec.scheduling.nodeIsolation` edit hits every validator at once — gate both.** The network stamps the new set onto every child SeiNode in one reconcile, and each Running child builds a config-update plan (`config-patch → config-validate → mark-ready → restart-seid`: the sidecar regenerates the TOML and SIGTERMs seid in place; the pod is not replaced — controller `internal/planner/config_update.go`, `validator.go:buildRunningPlan`). Every validator does this at the same time, so on a chain under load or mid-bench it is a liveness event, not a config tweak. `spec.scheduling.nodeIsolation` is heavier: it is not in the SeiNetwork immutability set, the network stamps it onto every child, and a Running child whose `.status.currentNodeIsolation` differs builds a pod-template drift plan (`apply-statefulset → apply-service → config-patch → config-validate → replace-pod → observe-image → mark-ready`, `planner.go:podTemplateDrifted` / `validator.go:buildRunningPlan` — the same path as an image roll), so every validator *pod* is deleted and rescheduled, and a Shared→Dedicated flip can leave pods `Pending` if no free single-tenant worker exists. Before the PR that carries either edit, say so in the plan echo ("this restarts seid on all N validators" / "this replaces every validator pod; the bench in flight is invalid") and get the engineer's explicit go-ahead. Never batch it into a PR that does something else. Two things this does **not** cover: `spec.configOverrides` / `spec.overrides` on a Running node is a silent no-op (`references/troubleshooting-seinode.md` → *configOverrides edits never reach a Running node*), and a validator that was already Running before the controller learned configValues (`.status.currentConfigValuesHash` empty) does not restart on a later edit — the new set lands only with its next image roll (`planner.go:configValuesDrifted`).

## Mental model

You operate against the **harbor cluster** (eu-central-1 EKS). It runs the **sei-k8s-controller** which watches `SeiNetwork` and `SeiNode` CRs and reconciles them into StatefulSets, PVCs, and per-node headless Services. A SeiNetwork additionally gets a controller-created `<network>-internal` ClusterIP Service fronting its **validator children only** (selector `sei.io/nodedeployment=<network>`), published as `.status.internalService` alongside `.status.perPodServices[]` and composed `.status.endpoints`.

The Service exposes `rpc`/`evm-http`/`rest` ports. It excludes EVM WebSocket, gRPC, and P2P: they pin per-connection state and do not survive a kube-proxy L4 LB. `.status.endpoints` advertises aggregate URLs for Tendermint RPC/REST only. It steers EVM consumers to per-pod URLs (filters, mempool nonces, finalized-tag reads pin to a node). With a validator-only backend the aggregate's evm-http port is inert anyway, since ModeValidator disables EVM.

A standalone follower SeiNode publishes just its own `.status.endpoint`. The controller creates **no** load balancer, ingress, or HTTPRoute — external exposure is engineer-owned Flux YAML in the task dir; see the networking section in `references/ephemeral-chain-flow.md`.

**Where you work:** `eng-<alias>` namespace, registered to the engineer via a one-time PR against `sei-protocol/platform`. The namespace is the isolation boundary.

**Three ServiceAccounts in `eng-<alias>`, each scoped to one purpose:**

| SA | Runs as | k8s authority | AWS authority (via Pod Identity) |
|---|---|---|---|
| `<alias>` | Flux reconciler for the engineer's path in `harbor-engineering-workspace` | Narrow namespace-scoped Role: seinetwork/seinode/jobs/configmaps CRUD, derived resources read-only | None |
| `engineer-service-account` | Engineer-launched workloads (seiload Jobs, scripts, custom tooling) | Namespace-admin via `RoleBinding` to built-in `admin` `ClusterRole` | `aws_iam_policy.engineer`: `s3:PutObject` on `harbor-validation-results/eng-<alias>/*` (auto-scoped via session tag), ECR auth + sei-chain image read |
| `seid-node` | SeiNode StatefulSet pods (sei-k8s-controller-managed) | None (default SA permissions) | `aws_iam_policy.seid_node`: snapshot read, genesis r/w, EC2 `DescribeInstances` |

**Two repos:**

- `sei-protocol/platform` — tenant registration (one-time PR per engineer) and platform-wide infrastructure.
- `sei-protocol/harbor-engineering-workspace` — engineer workloads at `engineers/<alias>/<task>/`, reconciled by the per-engineer Flux Kustomization. The agent opens task PRs here on the engineer's behalf for chain spinup. Engineers also push directly when the workload is theirs to author manually (long-lived archive nodes, shared profiles).

**Default for engineer-facing intents — render → PR → Flux applies.** The team is GitOps-opinionated, so every engineer side effect on the cluster from the `network`/`node` trees goes through a PR against `sei-protocol/harbor-engineering-workspace`. Direct apply is never the path. The agent renders the CRs via `seictl network apply --dry-run` / `seictl node apply --dry-run` (output captured as JSON, converted to YAML via `yq -P`), and writes them to `engineers/<alias>/<task>/seinetwork-<id>.yaml` and `seinode-<id>-rpc-<k>.yaml`. It then opens the PR, surfaces the URL, and halts.

The engineer reviews and merges; Flux reconciles within ~60s. After merge, the agent watches via `seictl network watch <id> --until=Ready -n eng-<alias>` (and `seictl node watch <id>-rpc-<k> --until=Running` per follower) to report when pods are healthy. The CRs carry their own provenance via labels and annotations; `kubectl get seinetwork,seinode -n eng-<alias>` is the authoritative live view post-merge. Direct `seictl network|node apply` (server-side apply, no PR) is an explicit escape hatch — see "Escape hatch" in the procedure below; never the default.

**The `workflow` tree is the one deliberate exception to this default**, not a variant of the escape hatch. The agent *always* runs `seictl workflow state-sync` imperatively and never commits it to the Flux workspace repo (Guardrail #9). The GitOps default above governs `network`/`node` only.

See `references/onboarding-pr.md` for the one-time tenant-registration flow, `references/ephemeral-chain-flow.md` for the headline daily-driver procedure, `references/seictl-cli.md` for the `network`/`node`/`workflow` verb trees, `references/harbor-cluster.md` for cluster facts.

## Pre-flight (run at session start, before any side-effecting action)

Pre-flight is a sequenced ramp from "fresh laptop" to "ready to apply against eng-<alias>." Each gate either passes (continue) or fails. A failure has an in-band recovery that runs through to completion, or an out-of-band recovery to surface and halt on. Run gates in order; halt on first failure.

| # | Gate | Detect with | If missing → |
|---|---|---|---|
| 1 | `seictl` carries the resource flags | `command -v seictl` returns 0; `node apply --help` lists **both** `--network` and `--cpu` | Install or upgrade with `go install github.com/sei-protocol/seictl@latest`, which resolves to v0.0.72 or newer. **Not** via `brew` (no tap). After a `go install`, re-run the `--network` probe rather than trusting the install's exit code. Halt until `seictl node apply --help` lists **both** `--network` and `--cpu`. The version floor, the shadowed-binary check, the `workflow`/`task` tree probes, the storage-performance probe, and the `--config-value` probe (check 5, required before any config.toml/app.toml request) are in *Gate 1 detail* below. |
| 2 | AWS SSO session active for the engineer's chosen profile | `aws sts get-caller-identity --profile <profile>` returns 0, where the profile-detection flow below resolves `<profile>`. After resolution, **echo the assumed identity** (`Arn` from the response) so the engineer sees what's about to act on the cluster. | If the session has expired: surface `aws sso login --profile <profile>`; halt. If no profile exists: route through the canonical Sei SSO session in `references/preflight.md` (Gate 3); halt. See "Profile detection" below for the resolution flow. |
| 3 | harbor kubeconfig context exists | `kubectl config get-contexts -o name` lists `harbor` (or the EKS ARN form) | Run `aws eks update-kubeconfig --name harbor --region eu-central-1 --profile <chosen>` directly (using the profile resolved at gate 2); re-check; continue. |
| 4 | kubectl can reach harbor with engineer-side reach | `kubectl auth can-i list seinetworks -n eng-<alias> --context=harbor` returns `yes` | EKS access entry not granted, or scoped read-only. Surface "ask the platform team via `#harbor-onboarding` with your AWS principal ARN"; halt. **This probe checks `seinetworks` only.** Before the first `seictl workflow` invocation in a session, separately confirm `kubectl auth can-i patch seinodetaskworkflows -n eng-<alias> --context=harbor` returns `yes`. Namespace Roles written before the workflow CRD omit the resource, and the run then fails at apply time with `is forbidden: ... cannot patch resource "seinodetaskworkflows"`. On `no`, the Role needs `seinodetaskworkflows` (verbs `get`, `list`, `watch`, `create`, `patch`, `delete`) plus `seinodetaskworkflows/status` read; surface the ask to the platform team via `#harbor-onboarding` and halt all `workflow` verbs until granted. |
| 4b | Cluster CRDs carry the resource fields | `kubectl explain seinode.spec.resources --context=harbor` exits 0 | A cluster whose CRDs predate the resource work prunes `spec.resources` from the applied object, silently. The node then takes the controller's 16 CPU / 128Gi default while the rendered file says 4 CPU / 32Gi. Halt every render that passes the resource flags, and ask the platform team to advance the cluster's controller pin. See `references/preflight.md` gate 5. **Storage performance needs its own probe.** Run that gate's storage-performance sub-gate before you offer a performance tier. |
| 5 | Namespace `eng-<alias>` reconciled | `kubectl get namespace eng-<alias>` returns 0 | The engineer has no onboarding yet, or the onboarding PR has not merged. Route to **First Run** below to open the PR. Otherwise surface the open PR URL and offer to poll until the namespace appears (~60s post-merge). |

Once every gate passes, cache the pass for the session. Subsequent verbs then skip the gates, unless a halt condition (SSO expiry, kubectl context drift) triggers a targeted re-check. (`references/preflight.md`'s detailed ramp also gates on `yq` and the `flux` CLI. Its numbering therefore runs one ahead of this table from SSO onward, and its gate 6 is this table's gate 5. Gate 4b here is that ramp's gate 5 resource-CRD sub-gate.)

For deep detail per gate (recovery commands, edge cases, the full new-engineer walk-through, mid-session drift handling), see `references/preflight.md`.

### Gate 1 detail

The version floor is v0.0.72, the first tag carrying the resource flags. `go install` itself is only possible from v0.0.71 on. An older copy in `/usr/local/bin` can shadow the new binary. See `references/preflight.md` gate 1 for that check, the release-tarball and build-from-source alternatives, and the provenance-stamp caveat. A bare `command -v seictl` also passes on a shadowed stale copy. The `--cpu` probe fails silently when skipped — see `preflight.md` gate 1 check 3.

This gate reads the binary only, so it runs before SSO; the matching cluster-side probe is gate 4's resource-CRD sub-gate. **This probe confirms `network`/`node` only.** The `--network` flag predates the `workflow` tree, so passing gate 1 does not guarantee `seictl workflow` exists. Before the first `seictl workflow` invocation in a session, separately confirm with `seictl workflow --help` (exits 0, lists `state-sync`). If it does not, the binary needs the change that added the `workflow` tree, which gate 1's probes do not cover. The same applies to `seictl task` — confirm with `seictl task --help` before its first use.

Gate 1 names two capabilities and no version, so passing it says nothing about which other trees the binary carries. **Storage performance needs its own probe.** `--iops` and `--throughput` merged after the resource flags, so a build taken from `main` between those merges satisfies the `--cpu` probe and still lacks them. Before offering a performance tier, confirm `seictl node apply --help` lists `--iops`. A binary that lacks it renders the standard tier in silence, exactly the way a binary predating seictl#248 drops the resource block. See `preflight.md` gate 1 check 4. **Typed config needs its own probe too.** `--config-value` (seictl#253) post-dates every tag that carries `--iops`, so a tagged binary passes checks 1–4 and fails `flag provided but not defined: -config-value` at render. Before a request that sets a config.toml/app.toml key, confirm `seictl node apply --help` lists `--config-value`; on a miss install from `@main` (preflight.md gate 1 check 5), never substitute `--set spec.configOverrides`.

### Profile detection (gate 2)

Do not hardcode a profile name; engineers configure their own.

1. **If the environment sets `$AWS_PROFILE`**, respect it as an explicit choice. Validate via `aws sts get-caller-identity --profile $AWS_PROFILE`. Echo the resolved Arn.
2. **Otherwise list the configured profiles** with `aws configure list-profiles`. Branch on the count:
   - **Zero profiles** → route through profile setup using the canonical Sei SSO session in `references/preflight.md` (Gate 3); halt until a profile exists.
   - **Exactly one profile** → use it. Echo `"Using AWS profile <name> (only one configured)"`.
   - **Multiple profiles** → present the choice. Default-suggest `sei` if it is among them.
     > I will use this AWS profile to authenticate kubectl + observe your harbor cluster resources. Which profile?
     > - `sei` (suggested)
     > - `<other>`
     > - `<other>`

The chosen profile is the session's profile — every downstream `aws ...` invocation runs with `--profile <chosen>`. Persist the choice for the session (e.g., shell-prefix every Bash call with `AWS_PROFILE=<chosen>` if not exported in the parent shell).

## First Run (the recovery for pre-flight gate 5)

When pre-flight gate 5 fails (`eng-<alias>` namespace does not exist), enter First Run. Gates 1–4 have passed — seictl, SSO, kubeconfig, and cluster reach are in place.

Onboarding is one PR against `sei-protocol/platform` touching four files. After merge, run a targeted `terraform apply`. Both pieces complete in under five minutes.

**Files the PR touches:**

| Path | Action |
|---|---|
| `clusters/harbor/engineers/<alias>/kustomization.yaml` | New. Per-engineer overlay. Mirrors the most recent prior onboarding PR; only the `alias=<alias>` literal differs. |
| `clusters/harbor/engineers/kustomization.yaml` | Modified. Adds `- <alias>` to `resources`. |
| `terraform/aws/189176372795/eu-central-1/harbor/engineers/<alias>.tf` | New. Two `eks-pod-identity` module instances for the engineer's `seid-node` and `engineer-service-account` SAs. Mirror the prior engineer's file with substring replacement of the alias. |
| `clusters/harbor/monitoring/podmonitor-seiload-eng.yaml` | Modified. Appends `eng-<alias>` to `namespaceSelector.matchNames`. Nothing fails without it, and the cell's seiload goes unscraped. |

**Procedure:**

1. **Prompt for the alias** — do not silently use `$USER`. Default the prompt to `$USER` lowercased. Validate the response against `^[a-z]([a-z0-9-]{0,28}[a-z0-9])?$`. **Then check uniqueness** with `kubectl get namespace eng-<alias>`. If it returns 0, another engineer holds the alias. Halt with "someone already holds this alias; pick another or contact the platform team if it is yours." Do not attempt partial-state recovery (that is a separate runbook); continue only when the alias is free.
2. Take the file list from the table above, not from a prior PR. [platform#587](https://github.com/sei-protocol/platform/pull/587) (the fromtherain re-onboard) is a two-file diff. It predates both the per-engineer terraform and the PodMonitor roster, so `gh pr diff 587` renders an incomplete onboarding. Read it for the shape of the cell overlay only, or copy the most recently onboarded engineer's files. Branch: `feat/engineers-<alias>-onboard`, cut from `main`.
3. Render the four platform-repo files. Literal shapes live in [`references/onboarding-pr-template.md`](./references/onboarding-pr-template.md); the full procedure is in [`references/onboarding-pr.md`](./references/onboarding-pr.md). Substring-replace the alias in the two new files; the two rosters are **append-only** — add one entry and leave every existing engineer's alone. Verify with `kustomize build clusters/harbor` (or `kubectl kustomize`) before opening the PR, and confirm the rendered hostnames read `*.eng-<alias>.harbor.platform.sei.io`.
4. Open the **platform-repo PR**. Title: `feat(harbor/engineers): onboard <alias>`.
5. Open the **workspace-repo sibling PR** against `sei-protocol/harbor-engineering-workspace`: branch `feat/onboard-<alias>`, scaffold `engineers/<alias>/kustomization.yaml` with `resources: []`. Without this, the per-engineer Flux Kustomization fails reconcile post-merge with `path not found: ./engineers/<alias>`.
6. Surface both PR URLs and halt:
   > Onboarding opened in two PRs:
   > - Platform: `<platform-url>`
   > - Workspace: `<workspace-url>`
   >
   > Merge the workspace PR first (or both within seconds). After the platform PR merges, Flux reconciles namespace + RBAC + Flux watcher in ~60s. Then from `terraform/aws/189176372795/eu-central-1/harbor/` run `export AWS_PROFILE=<chosen>`, then `terraform init && terraform plan -target=module.engineers -out=tfplan && terraform apply tfplan` to land the Pod Identity associations. Never apply without a reviewed plan. (`<chosen>` = the AWS profile resolved at gate 2.)
7. After merge, poll `kubectl get namespace eng-<alias>` until it returns 0.
8. Run `terraform plan -target=module.engineers -out=tfplan` and confirm `Plan: 6 to add, 0 to change, 0 to destroy`. Apply with `terraform apply tfplan`. `Resources: 6 added` confirms.
9. Gate 5 passes. Chain-spinup requests render via the skill, land in `harbor-engineering-workspace` PRs, and reconcile via Flux on merge. Pods running as `engineer-service-account` get S3 write to their own namespace prefix; SeiNode pods running as `seid-node` get snapshot read + peer discovery.

See `references/onboarding-pr.md` for the complete file content, base-layer details, and PR body template.

## What you can do

Every engineer-facing intent maps to a `seictl network`, `seictl node`, or (for in-place re-bootstrap of an existing node) `seictl workflow` verb against `eng-<alias>`. **Convention for every example below:** the namespace flag `-n eng-<alias>` is implicit and always present.

| Engineer says | Skill maps to |
|---|---|
| "Onboard me" / "set me up on harbor" / "I'm new" | First Run — open the platform-repo PR (see above + `references/onboarding-pr.md`). |
| "Spin up a chain of N validators with image X" / "start a chain" / "give me an ephemeral chain" | **PR-based** (see Procedure: spin up an ephemeral chain). Render via `seictl network apply --dry-run -n eng-<alias>` (JSON to stdout) → `yq -P` to YAML → write to `engineers/<alias>/<task>/seinetwork-<chain-id>.yaml` in `harbor-engineering-workspace` → commit → push → open PR. After merge, Flux applies; `seictl network watch <id> --until=Ready -n eng-<alias>` reports liveness. |
| "Add an RPC fleet to chain X" / "attach RPC nodes" | Same render → PR shape, but **N standalone SeiNode CRs**. Loop `seictl node apply <id>-rpc-<k> --preset rpc --chain-id <id> --image <ref> --network <id>` plus `--cpu`/`--memory`/`--storage` (and `--iops`/`--throughput` on a performance tier), for `k` in `0..N-1`. Write one `seinode-<id>-rpc-<k>.yaml` per follower in the same task dir (more commits to the open chain PR, or a follow-up PR). `--network <id>` auto-wires `peers[0].label.selector.sei.io/seinetwork=<id>` and stamps `sei.io/seinetwork=<id>,sei.io/role=node`, so the followers find the genesis validators. A node has no `--replicas`; the skill owns the loop. |
| "Attach a node via state sync" / "add an RPC node without replaying from genesis" / "bootstrap from my own chain" | Same follower shape plus a `spec.fullNode.snapshot` block: `stateSync: {}` + ≥2 `rpcServers` witness endpoints from the engineer's own chain (replaces the platform syncer registry — self-service, no platform PR). **Read `references/state-sync-bootstrap.md` first — its known-issue banner governs. State-sync bootstrap is currently broken on ceremony-fresh eng chains (PLT-794, genesis carries no validator set). Genesis replay is the working path until it lands.** Witnesses verify the trust point; snapshot chunks come over p2p from peers that have actually produced a snapshot. |
| "What's running in my namespace" / "what chains do I have" | `seictl network list -n eng-<alias>` for the chains + `seictl node list -n eng-<alias>` for the followers (yaml default; `-o name` for short, `-o jsonpath=...` for one-shot field reads). |
| "Show me chain X" / "what's the status of X" | `seictl network get <name> -n eng-<alias>` for the network (`.status.phase`); `seictl node get <name>-rpc-<k> -n eng-<alias>` for a follower (`.status.phase`, `.status.endpoint`). |
| "Tear down chain X" | `git rm -r engineers/<alias>/<task>/` **and** remove `<task>` from `engineers/<alias>/kustomization.yaml`'s `resources:` list (Kustomize fails to render with a missing-resource entry), then commit → push → merge. Flux prunes the SeiNetwork + follower SeiNodes on next reconcile; under `spec.deletionPolicy: Delete` (the CRD default) the validators, their pods, and their controller-managed PVCs go too. **Read `spec.deletionPolicy` first**: a SeiNetwork admitted under the old default carries `Retain`, which keeps its validators and EBS volumes running (remedies: `references/seinetwork-crd.md` → *Deletion*). **Teardown does NOT purge the chain-id's S3 genesis artifacts.** A respin needs a fresh chain-id (see the naming step) or a purge of the `<chain-id>/` genesis-bucket prefix. `bench:teardown` in `references/cluster-inspection-recipes.md` is the bench variant. **Confirm the volumes went too**: `kubectl get pvc -n eng-<alias> -l sei.io/chain=<chain-id>` empty **and** `kubectl get pvc -n eng-<alias>` shows no `<chain-id>-*` names (a label typo also returns nothing). PVCs are `gp3` / `reclaimPolicy: Delete`, so the EBS volume follows the PVC; a survivor is a `Retain` network or an imported PVC (see Halt Conditions). |
| "Partition validator 0" / "add 200ms latency" / "kill a validator mid-bench" / "run chaos against my chain" | **PR-based fault** (see `references/chaos-faults.md`). Pick the fault from the ten-entry catalog, substitute `<NS>/<CHAIN>/<RUN>/<DUR>` into the harness template, write `chaos-<fault>.yaml` into the experiment dir (`engineers/<alias>/exp-<RUN_ID>/`, same dir as the chain and the bench), commit → PR. Flux applies; Chaos Mesh injects; read `AllInjected` / `AllRecovered` on the CR. One victim on a 4-validator pool (`f=1`); refuse `dns-chaos` and `disk-io-latency` (deferred, reasons in the reference). Never `kubectl apply` a Chaos CR, never sleep-and-poll to sequence faults — a Chaos Mesh `Workflow` sequences them declaratively. |
| "Run a load test against chain X" / "bench it" / "stress test chain X" | **PR-based bench** (see Procedure: spin up a load test). Live-fetch chain rpc per-pod URLs, substitute into the profile JSON, render Job + ConfigMap from the templates in `references/sei-load-bench.md`, open a PR against `harbor-engineering-workspace` at `engineers/<alias>/bench-<RUN_ID>/`. Merge → Flux applies → seiload runs → the uploader sidecar pushes the results to S3. |
| "Compare PR 3399 to main on sei-chain" / "bench A against B" / "diff the perf of these two commits" | **PR-based comparative bench** (see Procedure: comparative bench). Renders two ephemeral chains (each running its own seid image) + two sei-load Jobs (identical profile + duration) into a single PR. After merge, watches both chains in parallel and polls both Jobs to terminal. Then fetches both reports from S3 and surfaces a side-by-side metrics table (TPS / latency / success rate / errors). Lives at `engineers/<alias>/compare-<COMPARE_RUN_ID>/`. |
| "Spin up a giga chain" / "enable the giga executor" / "autobahn network" / "evm-only chain" / "reproduce the autobahn README on harbor" | **Tier check first** (see `references/autobahn-giga.md`). Giga executor/storage = `spec.configValues` on a normal chain — deployable via the ephemeral-chain PR flow. Autobahn consensus and EVM-only are **not deployable** on the controller today (`autobahn.json` ceremony has no CRD substrate; EVM-only stops the CometBFT RPC the readiness probe reads) — say so, offer the Giga subset, link PLT-1249. Never render `evm-only = true`. |
| "Where am I" / "what cluster am I on" / "who am I" | `kubectl config current-context` + `aws sts get-caller-identity --profile <chosen>` (the gate-2 profile). (No dedicated `seictl context` verb in this surface.) |
| "Re-bootstrap / state-sync an existing node" / "migrate a node's store to giga" | **Destructive, imperative — NOT the PR flow:** `seictl workflow state-sync <node>` re-bootstraps an existing SeiNode via a `SeiNodeTaskWorkflow`, optionally with `--migration GigaStore --backend <pebbledb\|rocksdb>` (both tokens required together). Gated: engineer sign-off, target verification, `--dry-run` first, escalate-to-owner on shared/long-lived followers. For a disposable ephemeral follower that has merely fallen behind, prefer `delete` + re-apply **through the GitOps PR flow** over the imperative wipe. That flow is: git rm the manifest, PR, merge, re-apply via a new PR. After a bare `seictl node delete` outside that flow, Flux recreates the CR on the next reconcile and the safer path never lands. Full gate: `references/seictl-cli.md` → `seictl workflow state-sync`. |

**Override and composition:**

- Discrete flags (`--chain-id`, `--image`, `--cpu`, `--memory`, `--storage`, `--iops`, `--throughput`; `--replicas` on `network` only) override preset defaults.
- `--set <dotted.path>=<value>` (repeatable) does strategic-merge overrides on the CR spec; wins over discrete flags on collision. Maps merge per-key, lists replace wholesale. Example: `--set spec.image=ghcr.io/...:abc123` (the spec is flat — no `spec.template`).
- `--config-value <file>.toml:<dotted.key>=<value>` (repeatable, both trees) writes a **typed** `spec.configValues` entry — the config surface that reaches a Running node (a node provisioned before the feature waits for its next image roll, `ConfigBaselineUnobserved`). Values parse as JSON (`true`, `400`, `["a"]`) and fall back to string; `null`, non-`.toml` filenames and >100 entries are refused at render; repeating a flag for a `(fileName,key)` already in the list **replaces** that entry in place (a re-apply that changes a value is the normal path). The key is the **raw TOML path** inside the file (`config.toml:rpc.pprof_laddr`), not the unified-schema path `spec.overrides` takes (`network.rpc.pprof_listen_address`) — carrying one into the other fails silently or at `config-validate`. Needs a seictl carrying seictl#253 (Gate 1 check 5); v0.0.72 does not. Use it for every `config.toml`/`app.toml` knob (block interval, evm/giga toggles, rpc/api enables) instead of `--set spec.configOverrides...` / `--override`, which land only at first boot. **A configValues edit on a Running SeiNetwork restarts every validator at once** — provision benches with the values from the start, never edit them inside a measurement window. Verify on the cluster (`ConfigValuesValid` condition, `kubectl exec ... cat /sei/config/app.toml`), not from the spec. Full rules: `references/seictl-cli.md` → *Typed config values*.
- `--dry-run` on `apply` runs server-side-apply in dry-run mode and emits the would-be-applied CR without persisting. Right shape for "show me what this would do" before committing.

## Post-merge reconciliation

Force-reconcile proactively after a relevant PR merges; do not wait for Flux's natural poll.

**Trigger:** any PR merge the agent helped open against `sei-protocol/platform` or `sei-protocol/harbor-engineering-workspace`, OR when the engineer says "merged" / "I merged X". Do not wait for the engineer to ask.

**Preferred:**

```sh
flux --context harbor reconcile kustomization flux-system --with-source -n flux-system
```

**Fallback** (only when `flux` is not available):

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

Engineer says "spin up a chain of 4 validators with seid sha=abc, then add an RPC fleet." This is the daily-driver flow. The skill renders a SeiNetwork CR (and N SeiNode CRs for the fleet) via `seictl network apply --dry-run` / `seictl node apply --dry-run`, writes them to `engineers/<alias>/<task>/` in `harbor-engineering-workspace`, opens a PR. Engineer merges → Flux applies → agent watches the network to Ready and each follower to Running → reports endpoints.

**Read `references/ephemeral-chain-flow.md` first.** It carries the architectural context — preset taxonomy, what each preset wires automatically, the watch protocol, exit-code conventions. The procedure below is the operational restatement.

1. **Pre-flight** — if not already passed this session, run all five gates. Halt on first failure with the recovery surfaced.
2. **Resolve naming** — derive a chain ID from caller context, in priority order:
   1. **Explicit `--tag <slug>`** the engineer passes, if any.
   2. **Linear ticket** mentioned in the request → `harbor-<ticket-slug>` (e.g., `harbor-plt-327`).
   3. **sei-chain PR number** mentioned → `harbor-pr-<n>` (e.g., `harbor-pr-3399`).
   4. **commit SHA substring** mentioned → `harbor-<sha[:7]>` (e.g., `harbor-b7b4868`).
   5. **None of the above** → ask **one** question for an explicit tag. Do not silently fall back to a timestamp — nobody can tie anonymous IDs in Grafana / logs / cluster state back to the work they served.

   The resolved ID becomes the SeiNetwork name and `--chain-id`. Lowercase, k8s-namespace-safe (regex `^[a-z]([a-z0-9-]{0,28}[a-z0-9])?$`). For "chain X with RPC," the genesis network is `<id>` and the followers are `<id>-rpc-0 .. <id>-rpc-(N-1)`.

   **Never reuse a chain-id that has existed before.** The genesis ceremony keys its S3 artifacts by chain-id; a delete-and-recreate under the same ID wedges the new chain at height 0. See *Chain wedged at height 0 after delete-and-recreate* in `references/troubleshooting-seinode.md`.

   The derivation above is deterministic, so a **respin of the same ticket/PR collides by construction**. Before rendering, confirm the ID is fresh: `kubectl get seinetwork <id> -n eng-<alias>` returns `NotFound`. The ID must also not appear as a previously-torn-down chain in the engineer's workspace-repo history. On a respin, suffix an incarnation counter (`harbor-plt-327-2`) or purge the old ID's `<chain-id>/` prefix in the genesis-artifacts bucket first.
3. **Resolve image** — sei-chain (`seid`) image. **Required input** (PR / commit / branch / explicit `--image`); never silently default. Resolve to a full SHA and verify the image is in registry per `references/image-resolution.md`. Surface the resolved digest in the plan echo so the engineer sees what they are about to run.
4. **Resolve resources** — the seid container footprint and the data-volume size. The default is **4 CPU / 32Gi memory / 500Gi storage**, roughly a quarter of the mainnet shape. Pass all three flags explicitly on every render: `--cpu 4 --memory 32Gi --storage 500Gi`. Each flag overrides one dimension, so `--cpu 8` alone leaves memory and storage untouched. Every value must be a Kubernetes quantity (`32Gi`, not `32GB`); `--cpu` accepts `4` or `500m`. A resize needs delete-and-recreate, because the CRD fixes all three fields at creation.

   Every node in the chain takes this footprint, validators and followers alike. A 4-validator chain with a 4-follower fleet therefore books about 4Ti of create-only EBS. Raise that total with the engineer before rendering whenever the chain totals more than four nodes. Pass a smaller `--storage` on the follower loop if they want one.

   **The 500Gi default assumes a fresh-genesis chain. A follower that restores an existing chain's state needs an explicit, larger `--storage`.** Snapshot restore (`references/aws-dependencies.md`) and state-sync (`references/state-sync-bootstrap.md`) both load an accumulated history — `pacific-1` and `atlantic-2` are far past 500Gi. Size the volume from a measurement of the restored state, plus headroom for continued sync. The size column in an `aws s3 ls` listing reports the compressed archive, not the restored database, so it undershoots by a multiple. `references/aws-dependencies.md` gives the measurement order to use instead.

   **Pass the flags even when the engineer accepts the default.** An older `seictl` carries presets with no resource block. That binary renders a CR with no resource fields, and the controller then applies its own per-mode default of 16 CPU / 128Gi. Nothing errors on that path — the CR applies cleanly, and the chain runs four times the intended footprint. Passing the flags converts that silent mis-size into a loud parse error on a binary too old to accept them. Gate 1 in `references/preflight.md` probes for `--cpu` and catches the same case earlier.

   **Resolve storage performance too, and default to standard.** The menu has two entries. Standard means neither flag, and the gp3 StorageClass defaults apply. Performance means `--iops 10000 --throughput 750`, which seictl resolves to `sei-gp3-performance-v1`. Offer the performance tier only for a storage-bound bench, because a dev chain rarely needs it. The selection is create-only, like the other three.

   You supply the pair and seictl resolves the class name. The direction never runs the other way, and no flag takes a class name. An unsupported pair fails at the flag, and the message names the supported set.

   **The performance tier needs `--storage 20Gi` or more.** EBS gp3 caps IOPS at 500 times the volume size in GiB, so 10000 IOPS needs at least 20Gi. The 500Gi default clears that easily, and a smaller `--storage` on a follower loop may not. seictl catches the combination locally, but the apiserver cannot. The ratio is an AWS rule rather than a schema rule. Admission therefore passes, and the PVC then fails to provision with the pod `Pending`.

   A resize also destroys the data PVC (`references/troubleshooting-seinode.md`). Step 2's fresh-chain-id rule governs the respin.
5. **Render** — `seictl network apply <id> --preset genesis-chain --chain-id <id> --image <ref> [--replicas N] --cpu <cpu> --memory <mem> --storage <stor> [--iops <iops> --throughput <tput>] -n eng-<alias> --dry-run` emits the post-apply CR as JSON on stdout. Pipe through `yq -P` to convert to YAML. For the RPC fleet (if requested), **loop** N times: `seictl node apply <id>-rpc-<k> --preset rpc --chain-id <id> --image <ref> --network <id> --cpu <cpu> --memory <mem> --storage <stor> [--iops <iops> --throughput <tput>] -n eng-<alias> --dry-run` for `k` in `0..N-1` — one SeiNode CR per follower. (`--network <id>` is the peer rail; there is no `--replicas` on a node.)

   **Verify the footprint in the rendered CR before you write it.** Each CR must carry `spec.resources.requests.cpu`, `spec.resources.requests.memory`, and `spec.dataVolume.storage.resources.requests.storage`. A CR missing any of the three means the flag never reached the object. Two causes produce that: the binary ignored it, or the cluster's CRD pruned it. Re-run gate 1 check 3 and gate 4's resource-CRD sub-gate to tell them apart, and halt rather than committing the CR.

   **A performance selection adds a fourth path to check.** When the render passed `--iops`/`--throughput`, the CR must also carry `spec.dataVolume.storage.volumeAttributesClassName`. `seictl` prints the object the apiserver returned, so a missing name means the cluster pruned the field rather than that seictl skipped it. Halt and re-run gate 4b's storage-performance probe.

   **On `resources.limits`.** A CR rendered through this procedure carries none: the controller derives the memory limit from the request. Write the CR verbatim and never strip fields in the `yq` pass. A `limits.memory` appears only when the engineer passed one through `--set`. Leave it — the CRD requires it to equal the memory request, and the apiserver rules on that. A `limits.cpu` cannot appear at all, because seictl refuses to render one.
6. **Plan echo & confirm** — on the first side-effecting call of the session, show the engineer the plan below, then wait for confirmation. Read the resource footprint out of the rendered CR from step 5, never from this file's stated default. Show:

   - cluster (harbor) and namespace (`eng-<alias>`)
   - network name, follower count, and the follower names
   - chain-id and image digest
   - validator replica count
   - resource footprint per node group: CPU request, memory request, storage size
   - storage performance per node group: the `volumeAttributesClassName` read out of the rendered CR, or `standard` when the CR carries none
   - the task path under the workspace repo, and the files the agent will commit and push
7. **Write to workspace repo** — fresh clone of `sei-protocol/harbor-engineering-workspace` (or reuse a session-scoped clone). Write to `engineers/<alias>/<task>/seinetwork-<id>.yaml` (and `seinode-<id>-rpc-<k>.yaml` per follower, or one multi-doc file). Update `engineers/<alias>/<task>/kustomization.yaml` listing all of them as resources. Append `<task>` to `engineers/<alias>/kustomization.yaml`'s `resources:` list if not already present.
8. **Commit + push** — branch `feat/eng-<alias>-<task>`. Commit message: `feat(eng/<alias>): spin up <task> — chain-id=<id>, image=<digest-prefix>`. Push.
9. **Open the PR** — title: `feat(eng/<alias>): spin up <task>`; body lists chain-id, image digest, preset(s), expected endpoints. `gh pr create --repo sei-protocol/harbor-engineering-workspace --base main`.
10. **Surface and halt** — engineer reviews and merges. Surface the PR URL with: "after merge, Flux reconciles in ~60s; ping me to watch the network to Ready and report endpoints."
11. **After merge — watch** — `seictl network watch <id> --until=Ready --timeout=15m -n eng-<alias>` for genesis (the network reaches `Ready`), then per follower `seictl node watch <id>-rpc-<k> --until=Running --timeout=15m -n eng-<alias>` (a node reaches `Running` — **never `Ready`**, which is illegal for a node and errors at parse). NDJSON stream; exits 0 on the matched phase. Halt on non-zero (`metav1.Status` on stderr — `jq -r .reason` discriminates Timeout vs terminal Failed phase vs API failure).
12. **Report** — use the canonical inspection recipes from `references/cluster-inspection-recipes.md` rather than inferring jsonpath at runtime. Recipe #1 returns the fleet's RPC endpoints (target these for any load tools — never validators, which serve no EVM). Recipe #4 lists the network + its follower SeiNodes with phase + readiness in one shot. Plus teardown: `git rm -r engineers/<alias>/<task>/` and remove the `<task>` entry from `engineers/<alias>/kustomization.yaml`'s `resources:` list, then commit → push → merge. Flux prunes the SeiNetwork + SeiNodes; under `deletionPolicy: Delete` the validators, pods, and controller-managed PVCs go too. A `Retain` SeiNetwork keeps its validators — see the teardown row in the operation table.

### Escape hatch: direct `seictl network|node apply` (rare; engineer asks twice)

An engineer may ask to bypass the PR loop for a one-shot debug session. Proceed only after they confirm they understand the result will not be in git history. Apply the network with `seictl network apply <id> --preset genesis-chain --chain-id <id> --image <ref> --cpu <cpu> --memory <mem> --storage <stor> [--iops <iops> --throughput <tput>] -n eng-<alias>`, which server-side applies and emits the post-apply CR on stdout. Then apply each follower with `seictl node apply <id>-rpc-<k> --preset rpc --chain-id <id> --image <ref> --network <id> --cpu <cpu> --memory <mem> --storage <stor> [--iops <iops> --throughput <tput>] -n eng-<alias>`. Then `seictl network watch <id> --until=Ready -n eng-<alias>` and `seictl node watch <id>-rpc-<k> --until=Running -n eng-<alias>`.

The escape hatch skips step 5's rendered-CR check, so pass the resource flags here too rather than trusting the preset. Pass the same footprint to **both** commands, and pass the same storage-performance selection with it. A footprint or a tier given to the followers alone leaves the validators on the preset shape. That asymmetry is create-only, so correcting it costs the chain a fresh chain-id and its PVCs. `node apply` also needs `--image`, because the `rpc` preset carries none.

**Steer first.** Before running, ask: "I can do this through the GitOps PR flow (audit trail, Flux reconciles, `git rm` to tear down). Do you want that, or do you specifically need a direct-apply run with no git history?" Only proceed below on explicit confirmation. The agent does not volunteer this path.

## Procedure: spin up a load test (PR-based)

Engineer says "load chain X with seiload" or "bench against PR 3399 of sei-load." Sei-load runs as vanilla K8s — no CRD — so the rendering lives in this skill rather than `seictl network|node`. The agent fills in placeholders, opens a PR against `sei-protocol/harbor-engineering-workspace`, and the engineer's per-engineer Flux Kustomization reconciles the Job on merge. An uploader sidecar in the Pod pushes the seiload report to S3 when seiload exits (success, failure, or timeout).

**Read `references/sei-load-bench.md` first.** It carries the manifest templates, the live profile substitution recipe, the upload-sidecar pattern, the S3 key convention, and the run-id determinism rule.

1. **Pre-flight** — five gates from `preflight.md`. Halt on first failure.
2. **Verify the chain's followers are Running** — the bench requires per-follower RPC URLs, which only populate once each SeiNode reaches `Running`. `seictl node get <chain-id>-rpc-<k> -n eng-<alias> -o jsonpath='{.status.phase}'` returns `Running` for each follower. Halt and offer to poll if not.
3. **Resolve sei-load image** per `references/image-resolution.md` — required input, never silent default.
4. **Resolve profile** — default `nightly_evm_transfer`. Read profile JSON via `gh api ... --jq .content | base64 -d`.
5. **Resolve `<RUN_ID>`** — check for an existing PR branch matching `feat/eng-<alias>-bench-<bench-tag>-*`; reuse on match, else mint `<bench-tag>-<UTC-timestamp>`.
6. **Live-fetch the fleet's RPC URLs** — `seictl node list -n eng-<alias> -l sei.io/seinetwork=<chain-id>,sei.io/role=node -o json | jq -r '[.items[].status.endpoint.evmJsonRpc | select(.)]'`. `select(.)` drops any follower not yet `Running` (unset endpoint). Substitute `__SEI_CHAIN_ID__` and `__RPC_ENDPOINTS__` into the profile JSON.
7. **Plan echo & confirm** — chain-id, the names of the rpc follower SeiNodes (`<chain-id>-rpc-0..N-1`) + fleet count, sei-load image (resolved digest + source PR/commit). Also profile, duration, `<RUN_ID>`, target path, expected S3 key. Wait for confirmation.
8. **Render the manifests** — Job + ConfigMap + kustomization.yaml in `engineers/<alias>/bench-<RUN_ID>/`. Append `bench-<RUN_ID>` to `engineers/<alias>/kustomization.yaml`'s `resources:` list.
9. **Commit + push** — branch `feat/eng-<alias>-bench-<RUN_ID>`. Message: `feat(eng/<alias>): bench <RUN_ID> against <chain-id> (image=<digest-prefix>)`.
10. **Open the PR** against `sei-protocol/harbor-engineering-workspace`. Surface URL: "Merge to start the bench. Job runs `<DURATION>` minutes; results land at `s3://harbor-validation-results/eng-<alias>/<profile>/<RUN_ID>/report.log`."
11. **Report observation recipes** — surface three named recipes from `references/cluster-inspection-recipes.md`:
    - **live-tail** (`bench:live-tail`).
    - **terminal-check** (`bench:terminal-check`) — the kubectl jsonpath + grep recipe. jsonpath filter expressions do not support `||`, so the recipe iterates on the host side.
    - **teardown** (`bench:teardown`) — `git rm` of `engineers/<alias>/bench-<RUN_ID>/` plus the `kustomization.yaml` entry removal.

## Procedure: comparative bench (two images, side-by-side)

Engineer says "compare PR 3399 to main on sei-chain" or "bench latest sei-chain against commit b7b4868." Renders two ephemeral chains (each running its own seid image) + two sei-load Jobs (identical profile + duration + replica counts) into a single PR. After merge, both chains spin up in parallel; both benches run in parallel; both reports land in S3 paired by `<COMPARE_RUN_ID>`. The agent fetches both, extracts canonical metrics, and surfaces a side-by-side table.

**Read `references/comparative-bench.md` first.** It carries the four-subdir layout, naming convention, post-merge parallel-watch pattern, and S3 fetch + metric-extraction recipe. It also carries fallback rendering and halt conditions specific to the comparative shape.

1. **Pre-flight** — five gates from `preflight.md`. Halt on first failure.
2. **Resolve both images** — image A (engineer's input — PR / commit / branch) and image B (baseline — engineer-supplied; no silent default). Per `references/image-resolution.md` for both. Halt if either is missing in registry; do not render half a comparison.
3. **Resolve compare-tag** — Linear ticket → `<imageA-tag>-vs-<imageB-tag>` → explicit `--tag`. Validate the chain-tag length budget (**≤22 chars**). The name regex caps at 30 and the longest auto-suffix is now `-a-rpc-<k>`, ≥8 chars for a single-digit ordinal. See `references/comparative-bench.md`.
4. **Resolve `<COMPARE_RUN_ID>`** — same shape as single-bench. Branch is `feat/eng-<alias>-compare-<compare-tag>-*`; reuse on match, else mint.
5. **Resolve profile + duration** — defaults `nightly_evm_transfer` / 10 min. Both sides use identical values; the skill enforces this.
6. **Verify no CR name collisions** — the planned names (`<chain-tag>-{a,b}` networks + each side's `<chain-tag>-{a,b}-rpc-<k>` followers) must all return `NotFound` in `eng-<alias>`.
7. **Plan echo & confirm** — both image digests + source refs, both chain-ids, profile, duration, `<COMPARE_RUN_ID>`, target workspace path, both expected S3 keys, total estimated runtime (`<DURATION> + ~6 min`).
8. **Render the four sub-dirs** (chain-a, chain-b, bench-a, bench-b) per the templates in `references/comparative-bench.md`. Append `compare-<COMPARE_RUN_ID>` to `engineers/<alias>/kustomization.yaml`'s `resources:` if not already present.
9. **Verify config parity** — read both rendered bench ConfigMaps back, diff the substituted JSONs; abort the render if they differ on anything except `seiChainId` and `endpoints`.
10. **Commit + push** — branch `feat/eng-<alias>-compare-<COMPARE_RUN_ID>`. Message: `feat(eng/<alias>): compare <imageA-tag> vs <imageB-tag> (<COMPARE_RUN_ID>)`.
11. **Open the PR** — surface URL: "Merge to start. Both chains spin up in parallel (~5 min), both benches run for `<DURATION>` minutes, then I will fetch the reports and surface the comparison."
12. **After merge — watch networks in parallel** — `seictl network watch <chain-tag>-a` and `<chain-tag>-b` concurrently (`--until=Ready --timeout=15m`). Halt the whole comparison if either reaches Failed; the comparison is invalid against half a setup.
13. **Watch follower fleets in parallel** — per follower on each side, `seictl node watch <chain-tag>-{a,b}-rpc-<k> --until=Running --timeout=15m` (a node's terminal is `Running`, not `Ready`).
14. **Poll both bench Jobs to terminal** — single loop that checks both `seiload-<COMPARE_RUN_ID>-{a,b}` for `Complete=True` or `Failed=True`. Deadline: `<DURATION> * 60 + 660` seconds.
15. **Fetch + render** — pull both reports via in-cluster `kubectl run` under `engineer-service-account` (the engineer's local SSO profile lacks `s3:GetObject` on the prefix). Locate sei-load's summary block in each report. Surface both summaries verbatim under a delta table that highlights canonical metrics (TPS, P50/P99 latency, success rate, tx counts) with `better` / `worse` verdicts. On summary-block miss, fall back to last-50-lines format with both S3 paths.
16. **Teardown guidance** — `git rm -r engineers/<alias>/compare-<COMPARE_RUN_ID>/` and remove the entry from `engineers/<alias>/kustomization.yaml`'s `resources:`. Flux prunes both SeiNetworks + all follower SeiNodes + two Jobs; under `deletionPolicy: Delete` the validators, pods, and controller-managed PVCs go too. A `Retain` SeiNetwork keeps its validators and volumes — see the teardown row in the operation table.

## Procedure: troubleshooting (manual)

Engineer says "X is stuck" or "diagnose chain foo." `seictl` has no diagnose verb — walk the engineer through the manual `kubectl`-driven flow documented in `references/troubleshooting-seinode.md`.

1. Read `.status.phase` via recipe #2 in `references/cluster-inspection-recipes.md` (`network get` for the genesis network, `node get` for a follower). If the CR is in `Failed` or stuck `Initializing`, recipe #3 returns the failed task name + error directly — do not try to re-derive the path.
2. Branch on phase:
   - **Pending** → check sei-k8s-controller pods + leader lease in `sei-k8s-controller-system`. `Pending` means the controller has not picked the network up; a validator *pod* that exists but will not schedule is not this branch — the network is `Initializing`, see the pod-Pending step below.
   - **Initializing** → read `.status.plan` for the failing PlannedTask; map task name to root cause. Mapping: snapshot-restore → S3 / Pod Identity, configure-genesis → genesis URL, discover-peers → label selector, mark-ready → seid health
   - **Any phase, a validator pod sits `Pending`** (usually while the network is `Initializing`: the controller has already rendered the StatefulSet) → read the pod events (`kubectl describe pod <pod> -n eng-<alias>`; look for `0/N nodes are available`): `Insufficient cpu` / `Insufficient memory` means the requested size exceeds every worker in the pool (oversize validator: the remedy is a smaller `--cpu`/`--memory`, and the request is create-only so that means a fresh chain-id); `didn't match pod anti-affinity rules` on a Dedicated pool means no free single-tenant worker (a capacity ask to the platform team, not a chain fault). On a controller at or after `7163c98`, `.status.nodes[*].placement` on the SeiNetwork reports `Pending` for exactly those pods. A pool you did not ask to be Dedicated can still be: the legacy `sei.io/dedicated-node` annotation makes it Dedicated when `spec.scheduling.nodeIsolation` is unset, so omitting the flag does not by itself mean Shared — read the annotation too.
   - **Running** (node) / **Ready** (network) → check seid logs, pod restarts; for an engineer-owned exposure route, check the HTTPRoute they rendered (the controller creates none)
   - **Failed** → read `.status.plan.failedTaskDetail.error` (also lifted to stderr by `seictl network|node watch` on terminal Failed)
3. Surface the matching `kubectl` invocations from `references/troubleshooting-seinode.md`.

## Procedure: read-only verbs (`get`, `list`)

Pure invocations. Skill calls `seictl network|node <verb>` and surfaces the structured output to the engineer in plain English. No questions, no confirmation.

## Halt Conditions

Stop and report to the user if:

- **kubectl context drifts mid-session** — engineer switched contexts in another terminal. Re-confirm before any side effect.
- **`seictl network|node <verb>` exits non-zero** — surface the `metav1.Status` on stderr (`jq -r .reason && jq -r .message`). Do not retry silently.
- **Re-apply rejected `Invalid` on an immutable field** — the apiserver rejects `network apply <same-name>` when `--chain-id`, `--replicas`, `--cpu`, `--memory`, `--storage`, `--iops`, or `--throughput` changes. `spec.genesis`, `spec.replicas`, `spec.resources`, and `spec.dataVolume.storage` are all admission-immutable. The fix is `delete` + re-create, not a re-apply. Surface it; do not retry. A resize therefore costs the chain its state: the data PVC goes with the node, and the new chain needs a fresh chain-id.
- **Image digest resolution fails** — image not in ECR/GHCR or auth missing. Stop and surface the recovery command.
- **Image not in registry** — sei-chain CI may be behind. Surface the explicit retry command per the autobake race-guard pattern; do not loop silently.
- **`seictl network|node watch` exits with `metav1.Status.reason=Timeout`** — the CR has not reached the requested phase within `--timeout` (default 15m). Halt; surface `.status.plan.tasks[]` from the last NDJSON line for the engineer to inspect.
- **`seictl network|node watch` exits on terminal Failed phase** — `.status.plan.failedTaskDetail.error` is on stderr; surface it and the failed task name. Do not auto-retry.
- **`node watch --until=Ready` errors `Invalid` at parse** — a node has no `Ready` phase; its terminal is `Running`. Use `--until=Running`, or `--until=caught-up` (the node-only serve-readiness sentinel) when the intent reads "up **and** serving". (Symptom of carrying network vocab onto a node.)
- **`seictl network|node get` / `watch` exits with `metav1.Status.reason=NotFound`** — the named CR does not exist in the namespace. The fix is to apply it (PR-based, or escape-hatch direct apply), not to retry. For a bench whose follower `<chain-id>-rpc-<k>` is `NotFound`, the chain has no followers yet — surface the apply path (a follower render per `references/ephemeral-chain-flow.md` step 6) and halt. That recipe carries the create-only footprint and storage-performance flags; a bare command drops them.
- **CR name collision in the namespace** — `kubectl get seinetwork|seinode <name> -n eng-<alias>` returns an existing CR while the engineer's PR proposes a new one. Halt before opening the PR and surface the existing object's age and labels. Ask whether to pick a different chain-id, or `git rm` the existing object's manifest from the workspace repo first.
- **Workspace-repo task path collision** — `engineers/<alias>/<task>/` already exists. Do not silently overwrite. Halt and ask whether to reuse the dir (and add new files alongside the existing ones) or pick a different `<task>` name.
- **SeiNode Pending with `StateSyncReady=False/NoSyncersConfigured` and no StatefulSet.** A snapshot-bootstrap node whose witnesses the controller cannot resolve — the controller holds StatefulSet creation until the gate opens (deliberate; prevents a stranded Pending pod). The condition message names both remediations: declare ≥2 `spec.fullNode.snapshot.rpcServers`, or drop `stateSync` and genesis-replay. Do not delete/recreate the pod or chase storage — the block is upstream. See `references/state-sync-bootstrap.md`.
- **Followers not Running when rendering a bench.** The bench requires per-follower RPC URLs, which populate only once each SeiNode reaches `Running`. Halt and offer to poll (`seictl node watch <chain-id>-rpc-<k> --until=Running -n eng-<alias>`) before continuing.
- **Follower `.status.endpoint` present but dial refused.** `Running` means config applied + sidecar self-marked ready, NOT that the EVM listener is accepting connections — there is a real post-Running window. Before driving load, run `seictl node watch <name> --until=caught-up -n eng-<alias>` (the SDK readiness gate: height>1 with `catching_up=false`, plus EVM serving when the node publishes an EVM endpoint). Halt with the follower's full status if it stays refused.
- **Comparative bench: one side reaches Ready, the other Failed.** The comparison is invalid against half a setup. Surface the failed side's `.status.plan.failedTaskDetail.error`; offer to teardown the Ready side via `git rm` against just that sub-dir + commit. Do not run the bench against half a comparison.
- **Comparative bench: config parity check fails post-render.** The two substituted profile JSONs differ on a field other than `seiChainId` / `endpoints`. Halt before push; the rendered manifests would produce a non-comparable result.
- **Comparative bench: chain-tag exceeds the 22-char budget** once the `-{a,b}-rpc-<k>` follower suffix lands on it (the name regex caps at 30). Surface the overflow and ask the engineer to pick a shorter tag.
- **Comparative bench: S3 GetObject fails on a report.** `NoSuchKey` means the upload sidecar did not run — surface `kubectl logs -n eng-<alias> -l sei.io/compare-name=<COMPARE_RUN_ID>,sei.io/compare-side=<a|b> -c upload-results` to diagnose. `AccessDenied` means the engineer's profile lacks `s3:GetObject` (the engineer SA's IAM policy already covers it; the active profile is wrong).
- **Chaos CR admitted but `AllInjected` never `True`.** The namespace lacks `chaos-mesh.org/inject: enabled` (platform ask, PLT-1253), or the selector matched nothing (`kubectl describe <kind> <name>`). Do not patch the namespace annotation by hand; do not widen the selector to another namespace — the admission policy rejects it and it would be cross-tenant.
- **Chaos CR rejected at admission (`tenant-chaos-policy`).** A selector lacks `namespaces: ["eng-<alias>"]`, a `pods:` key names another namespace, or a `Schedule`/`Workflow` nests a scheduler. Fix the manifest per `references/chaos-faults.md` → *Selector contract*; the policy is not negotiable.
- **Engineer asks for a fault that halts the chain** — two victims on a 4-validator pool, `mode: all` on a kill, or any single victim on a 3-validator pool of equal power (one loss leaves exactly 2/3, not more). `f=1` no longer holds and the chain stops; say so, and render it only if the engineer confirms a liveness-loss experiment is the intent.
- **Engineer asks for `dns-chaos` or `disk-io-latency`.** Deferred in the harness; `IOChaos` corrupts a live SeiDB store rather than delaying it. Refuse and cite the reasons in `references/chaos-faults.md`.
- **Chaos CR in the same commit as a not-yet-Ready SeiNetwork without a readiness gate.** A one-shot kill during genesis assembly breaks the ceremony. The only in-Workflow readiness gate is a leading `StatusCheck` against the follower RPC (`references/chaos-faults.md` → *Sequencing with a Workflow*); a `Suspend` is a timer, and a cold image pull outruns it into genesis. Either lead the `Workflow` with a `StatusCheck` or split into two PRs — do not render a `Suspend`-only Workflow as the gate.
- **PR push rejected (non-fast-forward)** — engineer or another agent pushed to the same branch. Do not force-push. Halt; surface `git pull --rebase origin <branch>` and let the engineer resolve.
- **`seictl workflow state-sync` failed / the workflow is `Failed`** — a Failed `SeiNodeTaskWorkflow` holds the node not-ready until its removal. Recovery is force-delete first: annotate `sei.io/force-delete-workflow=<reason>`, then `seictl workflow delete <name>`, which releases the node; only then re-run. The annotation is not optional — an un-annotated delete parks the workflow `Terminating` with the node still held (`WorkflowDeleteHeld` event). Re-running with `--name` (a fresh name) *without* first removing the Failed workflow is neither a recovery nor a second wipe. Adoption is exclusive, so the new workflow never gets adopted, no plan compiles, no `reset-data` runs, and the watch burns its `--timeout` while the node stays held. See `references/seictl-cli.md`.
- **Cluster CRD predates a field the render carries (`configValues`, `scheduling.nodeIsolation`).** Server-side apply prunes the unknown field with no error, so the render succeeds and the value vanishes: values-less validators, or a Shared pool that was asked to be Dedicated. Probe `kubectl explain seinetwork.spec.configValues --context=harbor` and `kubectl explain seinetwork.spec.scheduling.nodeIsolation --context=harbor` before a render that uses either (preflight.md gate 5); on a non-zero exit halt that render and ask the platform team to advance the controller pin. Never drop the field and proceed in silence.
- **PVCs survive a teardown.** After the teardown PR merges, `kubectl get pvc -n eng-<alias> -l sei.io/chain=<chain-id>` must return nothing (the controller stamps `sei.io/chain=<spec.genesis.chainId>` on every data PVC: `internal/noderesource/noderesource.go` `GenerateDataPVC` → `ResourceLabels`; `sei.io/seinetwork` is the CR-selection key and is also present on network children, but `sei.io/chain` covers followers too). An empty result is only a pass if `kubectl get pvc -n eng-<alias>` shows no `<chain-id>-*` names either — a label typo also returns nothing. Controller-managed PVCs sit on the `gp3` StorageClass (`reclaimPolicy: Delete`), so a deleted PVC takes its EBS volume with it; a PVC still listed means either a `Retain` SeiNetwork (orphaned validators are still running — see the teardown row) or an imported PVC (`spec.dataVolume.import`, which the controller never deletes). Surface which one; do not `kubectl delete pvc` on agent initiative.
- **`seictl workflow state-sync` watch times out** — the watch ends when the workflow releases the node (15m default, 60m on older binaries; catch-up happens after `Complete`). A timeout therefore points at a wedged recipe step. Do not kill-and-retry: inspect `.status.plan.tasks` on the one in-flight workflow (`seictl workflow list -n eng-<alias>` to find it) before acting. An archive-scale `reset-data` still clearing means raise `--timeout` and wait; any other step parked past its budget means force-delete. Starting a second workflow alongside a wedged one is never the shortcut — adoption is exclusive, so it parks unadopted and times out too. After a `Complete` exit, verify catch-up separately with `seictl node watch <node> --until=caught-up`.

## Reference index

| File | Scope |
|---|---|
| `preflight.md` | **Read this first on a new session or when an engineer is fresh.** Five-gate ramp from "fresh laptop" to "ready to apply," per-gate recovery, mid-session drift handling, full new-engineer walk-through |
| `onboarding-pr.md` | **Read this if the engineer is new.** The one-time tenant-registration PR shape. Canonical example: `clusters/harbor/engineers/fromtherain/kustomization.yaml`. What the base layer provides |
| `ephemeral-chain-flow.md` | **Read this if the engineer asks for a chain.** Preset taxonomy (`genesis-chain`, `rpc`), what each preset wires automatically, watch protocol, exit-code conventions |
| `seictl-cli.md` | Canonical `seictl network` + `seictl node` + `seictl workflow` + `seictl task` verb trees (regenerated from `seictl <tree> --help` periodically). Carries the full destructive-op gate for `seictl workflow state-sync`, and the escape-hatch gate for `seictl task submit` |
| `seinetwork-crd.md` | Operations-load-bearing fields on `SeiNetwork` (the genesis validator pool), including `.status.phase`, immutability, the `.status.plan` |
| `seinode-crd.md` | Operations-load-bearing fields on `SeiNode` (a single node / follower), including `.status.phase` (terminal `Running`), `.status.endpoint`, `.status.plan` |
| `cluster-inspection-recipes.md` | **Canonical structured-extraction recipes.** Use these directly instead of inferring jsonpath at runtime. Covers RPC endpoints (recipe #1, also resolves "target RPC, not validator"), phase + readiness, failed task, image drift, a network's validator SeiNodes, Flux Kustomization Ready |
| `sei-load-bench.md` | **Read this if the engineer asks for a load test or bench.** Job + ConfigMap templates with substitution markers, live profile-JSON substitution recipe, two-container upload pattern (seiload + amazon/aws-cli sidecar with `shareProcessNamespace`). Also S3 archival convention, run-id determinism on re-render |
| `chaos-faults.md` | **Read this if the engineer wants a fault on their chain.** Ten-fault catalog from the controller harness (partition, latency, loss, bandwidth, corrupt, cpu, memory, time-skew, pod-kill, container-kill), the two deferred ones and why, namespace annotation + admission selector contract, `sei.io/nodedeployment` / `sei.io/node` selectors, `sei.io/harness-run` correlation, `f=1` rule, one-shot vs duration lifecycle, `AllInjected`/`AllRecovered`/follower-height gates, the `exp-<RUN_ID>` experiment-dir convention, Workflow sequencing |
| `comparative-bench.md` | **Read this if the engineer asks to compare two images.** Four-subdir layout (chain-a / chain-b / bench-a / bench-b), naming convention, per-side profile substitution, parallel post-merge watch sequence. Also S3 fetch + canonical-metric extraction with raw-tail fallback, comparison-output table format |
| `image-resolution.md` | **Canonical image-resolution recipes** for sei-chain (ECR) and sei-load (GHCR). PR/commit/branch input → full SHA → expected tag → registry probe → trigger + watch the build workflow if missing |
| `state-sync-bootstrap.md` | **Read this if the engineer wants a node that bootstraps via state sync from their own chain.** The `rpcServers` witness recipe: witnesses-vs-snapshot-providers model, preconditions, spec shape, `StateSyncReady` gate semantics, failure table (incl. the wrong-chain witness footgun) |
| `autobahn-giga.md` | **Read this if the engineer says giga, autobahn, evm-only, flatkv, or cites `sei-chain/integration_test/autobahn/README.md`.** Three-tier deployability verdict (Giga via `configValues` — yes; Autobahn / EVM-only — no, PLT-1249), the `app.toml` key set for executor + storage + receipts with the `sc-write-mode` enum, the README-to-CRD gap table, and every assumption elsewhere in this skill that EVM-only breaks (two-method RPC, validators serve EVM, height 0 idle, no funding) |
| `troubleshooting-seinode.md` | Phase-by-phase symptom → cause → inspection decision tree |
| `harbor-cluster.md` | CNI (Cilium), Istio + Gateway API, DNS, Flux topology, EKS access entries |
| `aws-dependencies.md` | S3 buckets (snapshots, genesis, results), Pod Identity status, ECR conventions |

When this skill drifts from `seictl`'s actual behavior, **`seictl network --help` / `seictl node --help` / `seictl workflow --help` win.** Reference files include a dated last-verified note per section to help spot drift.

## Permission pre-approval

Pre-approve in `.claude/settings.local.json` (user-specific, not committed):

```json
{
  "permissions": {
    "allow": [
      "Bash(seictl network get:*)",
      "Bash(seictl network list:*)",
      "Bash(seictl network watch:*)",
      "Bash(seictl node get:*)",
      "Bash(seictl node list:*)",
      "Bash(seictl node watch:*)",
      "Bash(seictl workflow get:*)",
      "Bash(seictl workflow list:*)",
      "Bash(kubectl config current-context:*)",
      "Bash(kubectl config get-contexts:*)",
      "Bash(kubectl get seinetwork:*)",
      "Bash(kubectl get seinode:*)",
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

- `seictl network apply` / `seictl node apply` (without `--dry-run`) — direct server-side apply; the escape-hatch path. Requires explicit confirmation per session.
- `seictl workflow state-sync` / `seictl workflow apply` (without `--dry-run`) — **destructive**: wipes the target node's local chain state and re-syncs (optionally an irreversible `--migration` store change). Requires explicit engineer sign-off per invocation; the agent never volunteers it. See the gate in `references/seictl-cli.md`.
- `seictl task submit` — **destructive-capable**, and ungated by the recipe. It POSTs any accepted task type to one pod's sidecar, `reset-data` included, with no hold, no `stop-seid`, and no ordering. Requires explicit engineer sign-off per invocation naming node, namespace, and task type (Guardrail #9). `seictl task delete` cancels a running task — also interactive. `seictl task get` / `seictl task list` are reads and safe to pre-approve.
- `seictl task snapshot-upload` — non-destructive but a real side effect (submits a snapshot upload that can run for hours and publishes to S3); requires explicit confirmation per invocation.
- `seictl network delete` / `seictl node delete` / `seictl workflow delete` — destroys a CR + propagates deletion to children; requires explicit confirmation. For `workflow delete`, the primary use is the force-delete recovery for a Failed workflow holding a node. A Complete workflow deliberately stays in-cluster as the audit trail (see `references/seictl-cli.md`); do not routinely delete it. For network/node, the default teardown runs `git rm` against the workspace-repo manifest, not this verb.
- `gh pr create` — opens onboarding and chain-spinup PRs; requires explicit confirmation per PR.
- `git push` — pushes engineer-task branches to `harbor-engineering-workspace`; requires explicit confirmation.

`seictl network|node apply --dry-run` is safe to pre-approve since it is render-only (returns the would-be-applied CR, no cluster mutation). Add `Bash(seictl network apply * --dry-run:*)` and `Bash(seictl node apply * --dry-run:*)` to the allow list if a session does heavy rendering.

Use the `fewer-permission-prompts` skill against a real session transcript once the skill is in active use.

## State management

This skill maintains no per-run state. Operation is stateless between invocations: every cluster-facing verb starts fresh. The engineer's identity is `eng-<alias>` namespace + EKS access entry — both managed by the cluster, not the agent. Active resources live in the cluster (queryable by `seictl network list -n eng-<alias>` + `seictl node list -n eng-<alias>`).

---

The verb tables above reflect what `seictl network --help`, `seictl node --help`, and `seictl workflow --help` emit. Network/node arrived in v0.0.59. The resource flags arrived in seictl#248 and first shipped in v0.0.72, and gate 1 probes for `--cpu` rather than for that version. The `workflow` tree arrived in the migration-flag release. The reference files track the shipped output shape: native CR on stdout, `metav1.Status` on stderr, NDJSON for watch. When this file disagrees with `seictl <tree> --help`, the CLI wins.

The `--iops`/`--throughput` storage-performance flags arrived in seictl#249 and shipped in that same v0.0.72 tag. Gate 1 probes that capability instead of a version string, because a version cannot reveal a shadowed binary.
