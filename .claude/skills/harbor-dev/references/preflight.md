# Pre-flight: getting an engineer on the rails

The first job of any new session is to confirm — or establish — that the engineer can actually drive `seictl network` / `seictl node` against their namespace. Pre-flight is a sequenced ramp from "fresh laptop" to "ready to apply." Each gate either passes (continue) or fails. A failure carries either an in-band recovery that runs through to completion, or an out-of-band recovery to surface and halt on.

## Why pre-flight is a ramp, not just a gate

A pre-flight that just rejects on missing prereqs gives engineers an error and walks away. The value is in being the rails — pre-flight should *land the engineer in the ready state*, not just diagnose the gap. Where the recovery is in reach (kubeconfig write, the onboarding PR), execute it and continue. Where the recovery is out-of-band (SSO login, EKS access entry, PR merge), surface the next step and halt cleanly.

The end state pre-flight delivers:

- `seictl` on PATH carrying the `--cpu`/`--memory`/`--storage` resource flags. v0.0.59 shipped the split `network`/`node` surface, and seictl#248 added the resource flags and the preset footprint. **v0.0.72 is the first tag that carries them**, together with the `--iops`/`--throughput` pair from seictl#249. Gate 1 still probes the capability rather than the version, because a version string cannot reveal a stale binary shadowing the new one on `PATH`.
- `yq` on PATH (the render path pipes `seictl network|node apply --dry-run` through it)
- `flux` CLI on PATH (used to force-reconcile harbor after a merge instead of waiting on the natural poll interval)
- AWS SSO session active under the engineer's chosen profile
- `harbor` kubectl context present **and current**
- kubectl can list `seinetworks` in `eng-<alias>` (proof of EKS access entry + RBAC)
- `eng-<alias>` namespace exists and Flux reconciles it

That is the floor for `seictl network|node apply`. Below this floor, no procedure can proceed safely.

`--iops`/`--throughput` sit above that floor rather than in it. They gate a storage-performance selection only, so a standard-tier render proceeds without them. Gate 1 check 4 covers them separately.

## The gates

### Gate 1: a `seictl` that carries the resource and config flags

**Verifies:** `seictl` is on `$PATH`, ships the split `network`/`node` surface, and carries the resource flags. This gate probes the binary only, so it runs on a fresh laptop with no SSO session and no kubeconfig. The cluster-side twin of check 3 lives in gate 5, which is the first gate that has cluster access.

Five-part check:

1. `command -v seictl` returns 0.
2. `seictl node apply --help` exits 0 and the help text includes `--network`. `--network` is the peer-rail flag on the split `node` tree. It exists only in v0.0.59+, so its presence proves the binary has the split trees (the old `nd apply` had no such flag). It is the breaking-cut sentinel: an older binary that still carries `nd` but not the split trees fails this gate. That is correct, because `nd` targets the deleted `SeiNodeDeployment` Kind and hard-fails at apply against new-CRD clusters. Optionally also probe `seictl network apply --help` for `--genesis-override`.

3. `seictl node apply --help` includes `--cpu`. This is the resource-flag sentinel, and the one check whose failure is otherwise **silent**. A binary that predates seictl#248 carries presets with no resource block, so it renders a CR with no resource fields. The controller then fills in its per-mode default of 16 CPU / 128Gi. Nothing errors — the engineer gets a mainnet-shaped dev chain while the plan echo claims 4 CPU / 32Gi. Probe the capability, not a version string — same reasoning as check 2.

4. `seictl node apply --help` includes `--iops`. This one gates the storage-performance selection only, so it blocks a performance tier rather than the whole render. `--iops` and `--throughput` came in seictl#249 and first shipped in v0.0.72, the same tag that first carries the resource flags. Probe the capability anyway, for the shadowing reason given above.

   This failure is loud once the flags reach the binary, exactly as check 3's is. An older `seictl` exits non-zero with `flag provided but not defined: -iops`. The silence arrives one step later, in the workaround. Dropping the two flags clears the parse error and renders the standard tier. The plan echo still promises 10000 IOPS, and the bench then measures the wrong disk. On a failure, either upgrade or drop to the standard tier, and state which one the render used.

5. `seictl node apply --help` includes `--config-value`. This gates typed `spec.configValues` (seictl#253), which post-dates the v0.0.72 floor: a v0.0.72 binary passes checks 1–4 and fails loud here (`flag provided but not defined: -config-value`). Skip only when the request sets no config.toml/app.toml key. On a failure, install from `@main` (the check-5 carve-out in the recovery block below); never substitute `--set spec.configOverrides`, which lands at first boot only and silently misses a Running node.

**Why:** every engineer-facing verb is a `seictl network …` / `seictl node …` invocation. The `--network` auto-wire makes "spin up chain + RPC fleet on the same network" a one-shot. Catching an old binary here beats a confusing `NotFound`-on-CRD at apply. For check 3 it beats something worse: a chain that runs four times its intended size without complaint. **Do not weaken this gate to pass on either old or new** — that lets a broken binary through.

**Recovery (out-of-band):**

Recommended path: `go install` from the newest release. The method itself works only from seictl v0.0.71 on, because seictl#246 removed the `replace` directives that blocked a module-aware install.

**`@latest` is the correct target again.** An earlier version of this runbook forbade it and sent engineers to `@main`. The newest tag was then v0.0.71, which predates the resource flags. seictl v0.0.72 carries both seictl#248 and seictl#249, so `@latest` clears checks 1 through 4. Do not restore the old prohibition for those four checks. Check 5 is the one exception: `--config-value` (seictl#253) is on `main` and on no tag, so a tag cannot clear it.

```sh
go install github.com/sei-protocol/seictl@latest   # clears checks 1–4
go install github.com/sei-protocol/seictl@main     # check 5 as well, until a tag carries seictl#253
```

Re-run the checks afterwards: 1 through 4 after `@latest`, 1 through 5 after `@main`. Say in the plan echo which target the binary came from. The version floor is **v0.0.72** for checks 1 through 4; check 5 has no tagged floor yet. Treat that floor as the recovery target rather than the pass condition. The checks above still read the help text, because a floor cannot see which binary `PATH` resolves.

**All three paths below clear checks 1 through 4.** The `-ldflags` recipe installs a tag, and the release tarball serves `releases/latest`. Both land on v0.0.72 or newer, so the provenance stamp no longer costs a failed gate. Neither clears check 5 — a tag does not carry seictl#253. Build-from-source from `main` does, and it is the one path that does not depend on a published release.

**On an upgrade, the install is only half the job.** `go install` writes to the Go bin directory, but every engineer who followed an earlier version of this runbook has `seictl` in `/usr/local/bin`. On a stock `PATH`, `/usr/local/bin` precedes `~/go/bin`. The install then succeeds while `command -v seictl` still resolves the old binary, so gate 1 keeps failing its `--network` probe. Do not re-run the install; it will keep succeeding.

The pass condition is capability, not location: whichever copy `command -v seictl` resolves must satisfy the `--network` probe. A binary in `/usr/local/bin` is perfectly fine when it is a current one — the tarball and build-from-source paths below put it there on purpose.

Re-run the probe first, and look at paths only when it fails:

```sh
seictl node apply --help | grep -- --network    # the actual gate; passing here ends it
command -v seictl                               # which copy won
go env GOBIN GOPATH                             # where go install put the new one
```

When the probe fails and those last two disagree, a stale copy is shadowing the new one. Remove the stale copy and re-run the probe:

```sh
sudo rm /usr/local/bin/seictl
```

Do not try `GOBIN=/usr/local/bin go install …` instead. That directory is root-owned on Linux and on Apple Silicon macOS, so the install fails with `permission denied`. Prefixing `sudo` does not help either: `sudo GOBIN=… go install …` builds against root's `GOPATH` and module cache, not the engineer's. That is a second confusing failure inside the section meant to end one.

Three things to know about it:

- The binary lands in `$(go env GOBIN)`, or `$(go env GOPATH)/bin` when `GOBIN` is empty. That directory must be on `$PATH` at all, or check 1 still fails — separate from the shadowing case above. To place it somewhere already on `$PATH`, set `GOBIN` for the call: `GOBIN=$HOME/.local/bin go install github.com/sei-protocol/seictl@latest`.
- seictl's `go.mod` declares `go 1.26.0`. On an older toolchain `go install` prints `switching to go1.26.8` and downloads it. Treat that line as normal output, not an error.
- A plain `go install` leaves the `seictl.sei.io/version` provenance annotation reading `dev` on every resource seictl applies. The version comes from an `-ldflags` stamp the Makefile passes, and `go install` does not pass it. Nothing breaks; the applied resources just do not record which seictl produced them.

To keep the provenance stamp, pass the flag. The version appears twice, so set it once — and set it to the release you mean to install, not the example value:

```sh
# Set V to the release you are installing. Where gh is available,
# `gh release view --repo sei-protocol/seictl --json tagName --jq .tagName`
# prints the newest tag. The floor is v0.0.72, the first tag carrying the
# resource and storage-performance flags. The ${V:?} guards below refuse to
# run on an unset V, so an empty value fails loudly instead of stamping "".
V=v0.0.72
go install -ldflags "-X 'github.com/sei-protocol/seictl/internal/cliutil.Version=${V:?set V to the release tag}'" \
  "github.com/sei-protocol/seictl@${V:?set V to the release tag}"
```

`${V:?…}` matters here. An unset or empty `V` would otherwise expand to
`…/seictl@`, and Go reports that as an invalid version with nothing pointing
back at the tag.

Alternative — prebuilt binary from the GitHub releases page. These carry the version stamp already and need no Go toolchain. Per-platform tarballs at `https://github.com/sei-protocol/seictl/releases/latest`:

```sh
# macOS (Apple Silicon)
curl -LO https://github.com/sei-protocol/seictl/releases/latest/download/seictl_Darwin_arm64.tar.gz
tar -xzf seictl_Darwin_arm64.tar.gz
sudo mv seictl /usr/local/bin/

# macOS (Intel) — seictl_Darwin_x86_64.tar.gz
# Linux — seictl_Linux_x86_64.tar.gz / seictl_Linux_arm64.tar.gz
```

Build-from-source fallback — needed when the engineer requires a commit newer than the latest release:

```sh
git clone git@github.com:sei-protocol/seictl.git
cd seictl
make build
sudo mv build/seictl /usr/local/bin/
```

**Do not** use `brew` — no tap exists for `sei-protocol/seictl`.

`go install` was unusable before seictl v0.0.71 and the runbook forbade it. Eleven `replace` directives in `go.mod`, inherited from sei-chain, made Go reject any module-aware install. seictl#246 removed them, and v0.0.71 is the first release that installs this way. The old prohibition no longer applies. If `go install` ever fails again with `contains ... replace directives`, a new one has crept back into `go.mod` — that is a seictl bug, not an install-method problem.

Halt until the first three checks pass: PATH, `node apply --help` lists `--network`, and `node apply --help` lists `--cpu`. Check 4 gates only the performance tier, so a render that stays on standard storage may proceed without it.

### Gate 2: `yq` installed

**Verifies:** `yq` is on `$PATH`.

```sh
command -v yq
```

**Why:** the canonical render path is `seictl network|node apply --dry-run | yq -P 'del(...)'` — JSON to clean YAML with server-side fields stripped (see `ephemeral-chain-flow.md`). Without `yq`, the agent has no way to produce the workspace-repo file.

**Recovery (in-band):**

```sh
# macOS
brew install yq

# Linux (x86_64) — prebuilt binary
sudo curl -L https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64 -o /usr/local/bin/yq
sudo chmod +x /usr/local/bin/yq
```

Halt until `command -v yq` returns 0.

### Gate 2b: `flux` CLI installed

**Verifies:** `flux` is on `$PATH`.

```sh
command -v flux
```

**Why:** the post-merge reconcile pattern (`flux reconcile kustomization flux-system --with-source -n flux-system`) is the fast path from "PR merged" to "manifests applied in cluster." Without `flux`, the fallback is `kubectl annotate kustomization flux-system reconcile.fluxcd.io/requestedAt=$(date +%s) --overwrite -n flux-system`, which works but does not fetch the latest source revision in the same call.

**Recovery (in-band):**

```sh
# macOS
brew install fluxcd/tap/flux

# Linux (any arch) — install script
curl -s https://fluxcd.io/install.sh | sudo bash
```

`flux` reuses kubectl's kubeconfig + current context; no separate auth setup. Halt until `command -v flux` returns 0.

### Gate 3: AWS SSO session active for the engineer's chosen profile

**Verifies:** `aws sts get-caller-identity --profile <profile>` returns 0 with an `Arn` field, where `<profile>` is the engineer's chosen AWS profile (resolved per the detection flow below). After resolution, **echo the resolved Arn back to the engineer** — they should see what's about to act on the cluster.

#### Profile detection flow

Engineers configure their own profiles; do not hardcode `sei` (or any other name). Resolution sequence:

1. **`$AWS_PROFILE` has a value in the environment** → respect it as an explicit choice. Validate via `aws sts get-caller-identity --profile $AWS_PROFILE` and continue. Echo:
   > Using `AWS_PROFILE=<value>` (from environment) — resolved as: `<arn>`.

2. **`$AWS_PROFILE` is unset** → list configured profiles with `aws configure list-profiles`:

   - **Zero profiles** → walk the engineer through profile setup using the canonical Sei SSO session below (do not make them guess the start URL/region). Halt until at least one profile exists.
   - **Exactly one profile** → use it directly. Echo:
     > Using AWS profile `<name>` (only one configured) — resolved as: `<arn>`.
   - **Multiple profiles** → present the list and ask the engineer to choose. Default the prompt to `sei` if it is among them (the most common harbor-account profile name); otherwise no default. Frame the prompt:
     > I will use this AWS profile to authenticate kubectl + observe your harbor cluster resources. Which profile?
     > - `sei` (suggested)
     > - `<other-1>`
     > - `<other-2>`

3. **Once chosen**, the profile name is the session's profile. Every downstream AWS-touching invocation runs with `--profile <chosen>` — `aws eks update-kubeconfig …`, `aws ecr describe-images …`, `aws s3 …`. If the parent shell does not already export `AWS_PROFILE`, prepend `AWS_PROFILE=<chosen>` to Bash invocations to keep the choice consistent.

The whole point: the engineer chose what's authenticating — they should be able to point at it in the echo.

#### Why this gate exists

harbor's EKS auth and ECR image pulls need live AWS credentials. SSO sessions expire (default 12h); refreshing is one command. Sessions that *look* alive (configured profile, recent login) but do not have the right *role* surface as `Forbidden` later. Gate 5 catches that on the kubectl side; AWS-side permission gaps surface naturally per-operation.

**Recovery (out-of-band):** `aws sso login --profile <chosen>`. If `~/.aws/config` is empty (truly fresh laptop), route them through profile setup using the canonical Sei SSO session below.

#### Canonical Sei SSO session

A fresh-laptop engineer should not have to guess the start URL or Identity Center region. Drop this `sso-session` block into `~/.aws/config`:

```ini
[sso-session sei]
sso_start_url = https://d-916729b434.awsapps.com/start
sso_region = us-west-1
sso_registration_scopes = sso:account:access
```

Then run `aws configure sso --sso-session sei` — it reuses this session and prompts for the account and role: select account **`189176372795`** (the Sei/harbor account) and your granted role. It writes a `[profile …]` that references the session. Log in any time after with `aws sso login --sso-session sei`. Note `sso_region` (`us-west-1`, where Identity Center lives) is distinct from the resulting profile's `region` (the harbor cluster's region, `eu-central-1`) — they are not the same value.

**Edge case — `Unable to locate credentials`:** a `--profile`-less `aws` call landed somewhere downstream. Every AWS-touching invocation needs `--profile <chosen>` explicit on the command (or `AWS_PROFILE=<chosen>` in the environment). Most common false-negative on this gate.

**Edge case — expired session mid-run:** SSO can expire between verbs. Halt conditions catch this (any AWS call returns `ExpiredToken`); re-run gate 3 and resume.

**Edge case — engineer's chosen profile lacks harbor permissions.** The resolved `Arn` is from a non-harbor account, or kubectl-reach (gate 5) returns Forbidden despite a valid session. Surface the Arn from the gate-3 echo. Prompt the engineer to either pick a different profile or re-engage the platform team for an access-entry update.

### Gate 4: harbor kubeconfig context exists and is current

**Verifies:** `kubectl config get-contexts -o name` lists `harbor` (or the EKS ARN form `arn:aws:eks:eu-central-1:189176372795:cluster/harbor`), AND `kubectl config current-context` returns either of those forms.

**Why:** kubectl needs the cluster endpoint, CA cert, and auth provider config in the kubeconfig before any `kubectl …` (or `seictl network|node …`, which reuses kubeconfig) can resolve harbor. `seictl network|node apply` has no `--context` flag; it uses whatever context is currently set. If the engineer last used a different cluster, every `seictl network|node …` invocation would silently hit that cluster instead of harbor.

**Recovery (in-band):**

```sh
# Add the context if missing
aws eks update-kubeconfig --name harbor --region eu-central-1 --profile <chosen>

# Set it as current
kubectl config use-context harbor
```

`<chosen>` is the profile resolved at gate 3, whatever its name — engineers configure their own, so never guess it. The first command is idempotent; the second sets the active context. Re-check both — `current-context` must return `harbor` (or the ARN form) before continuing.

**Edge case — engineer prefers a non-default kubeconfig path:** respect `$KUBECONFIG`. The `update-kubeconfig` command writes to whichever file `$KUBECONFIG` points at (or `~/.kube/config` if unset). Do not override.

**Edge case — context drift mid-session.** If a later kubectl call returns an unexpected cluster's resource (or fails with `cluster.local` errors), re-run gates 4 + 5 and resume.

### Gate 5: kubectl can reach harbor with engineer-side reach

**Verifies:** `kubectl auth can-i list seinetworks -n eng-<alias>` returns `yes`.

**Why:** the EKS cluster authorizes principals via *access entries* — separate from kubeconfig presence. A fresh principal with a valid kubeconfig can still get `Forbidden` on every kubectl call until the platform team adds the access entry. The check is intentionally narrow (list SeiNetworks in the engineer's namespace) — that is exactly what `seictl network apply` and `seictl network watch` need. The eng-`<alias>` Role grants `seinetwork`/`seinode` CRUD; if the migration has not reached the Role, this gate false-negatives — confirm the migration reached the Role.

**Recovery (out-of-band):** the platform team grants the access entry. Surface:

> Your AWS principal does not have permission to list seinetworks in `eng-<alias>` on harbor. This means the EKS access entry is not in place yet. Ask the platform team to add you. File a one-line request in `#harbor-onboarding` with your AWS principal ARN (the same one gate 3 echoed when it resolved your profile).

Halt until the access entry lands. Same-day turnaround typically.

**Workflow-CRD sub-gate:** before the first `seictl workflow` invocation in a session, separately verify `kubectl auth can-i patch seinodetaskworkflows -n eng-<alias> --context=harbor` returns `yes` — `patch` is the verb server-side apply exercises. A `no` means the namespace Role predates the workflow CRD. Halt all `workflow` verbs and ask the platform team via `#harbor-onboarding` to add `seinodetaskworkflows` (verbs `get`, `list`, `watch`, `create`, `patch`, `delete`, plus `seinodetaskworkflows/status` read) to the Role. The failure otherwise surfaces mid-operation as `is forbidden: ... cannot patch resource "seinodetaskworkflows"`.

**Resource-CRD sub-gate:** `kubectl explain seinode.spec.resources --context=harbor` must exit 0. This is the cluster-side twin of gate 1 check 3, and it fails just as silently. A cluster whose CRDs predate the resource work **prunes** `spec.resources` from the applied object, with no error, because structural-schema pruning drops unknown fields. The node then takes the controller's 16 CPU / 128Gi default while the rendered file on disk says 4 CPU / 32Gi. This check reaches the cluster, which is why it sits here rather than in gate 1.

Probe the CRD rather than reading the controller image tag. `clusters/<cluster>/sei-k8s-controller/kustomization.yaml` pins the image and the CRDs by the same ref, so a stale pin moves both. On a failure, halt every render that passes `--cpu`/`--memory`/`--storage`. Ask the platform team to advance the controller pin for the cluster.

**Storage-performance sub-gate.** A render that also passes `--iops`/`--throughput` needs two more things on the cluster, and each one gets its own probe:

```sh
kubectl explain seinode.spec.dataVolume.storage.volumeAttributesClassName --context=harbor
kubectl get volumeattributesclass sei-gp3-performance-v1 --context=harbor
```

**The first probe is the load-bearing one, and it fails on every harbor cluster today.** The CRD field comes from `sei-k8s-controller` PR 533, which nobody has merged. Pruning then makes the failure silent, in the same shape as the resource fields. The apiserver drops the unknown `volumeAttributesClassName` without an error. The PVC provisions on the plain gp3 StorageClass, and the plan echo still promises 10000 IOPS. `kubectl explain` reads the served schema, so this probe is namespace-independent and every engineer can run it.

On a non-zero exit from that probe, halt every render that passes `--iops`/`--throughput`. Offer the standard tier instead, and state that the render used it. Never fall back in silence.

**The second probe is advisory, because `VolumeAttributesClass` is cluster-scoped.** The `eng-<alias>` Role is namespace-scoped, so it grants no cluster-scoped read at all. Read the two failures apart rather than treating both as a missing class:

- `NotFound` — the class is genuinely absent. The platform team owns the catalog (`platform` `clusters/base/default/volume-attributes-class.yaml`). Halt and ask them.
- `Forbidden` — the engineer cannot read cluster-scoped objects. That is a Role question, not a catalog one, and this check is **inconclusive**. Do not halt on it. Say that the class went unverified, and proceed on the first probe's verdict.

State the cost of that uncertainty alongside it. If the class turns out to be absent, the PVC sits `Pending` on `ProvisioningFailed`. The field is create-only, so the remedy is a fresh chain-id rather than a fix. The engineer can then decide whether to ask the platform team to confirm the class first. In practice the catalog ships from `platform` `clusters/base/default/`, so every reconciled cluster carries it.

Halting on `Forbidden` would put the performance tier out of reach for the engineers this runbook serves. It would also blame the platform catalog for a Role gap.

**Edge case — alias not yet known.** On a brand-new engineer, First Run (gate 6 path) captures the alias before they have an `eng-<alias>` namespace. Run this gate against the *resolved* alias from First Run. If the engineer is mid-onboarding (PR open but not merged), it may still pass on namespace-list reach. The namespace does not exist yet; gate 6 owns the namespace-existence check.

**Edge case — gate passes but `apply` later fails with `Forbidden`:** the access entry may be read-only. Surface that as a separate gap when `seictl network|node apply` returns `metav1.Status.reason=Forbidden`. The platform team escalates the access entry to write.

### Gate 6: namespace `eng-<alias>` reconciled

**Verifies:** `kubectl get namespace eng-<alias>` returns 0.

**Why:** every workload the engineer creates lives in their namespace. If the namespace does not exist, `seictl network|node apply` fails immediately (`metav1.Status.reason=NotFound`).

**Recovery (out-of-band, with in-band lead):** if the engineer does not have an onboarding PR yet, route to **First Run**. First Run captures the alias, generates the PR body, and opens the PR via `gh pr create`. Surface the PR URL and halt pending merge — Flux reconciles in ~60s once merged.

If the PR is open but not merged, surface the URL and offer to poll until the namespace appears:

```sh
gh pr list --repo sei-protocol/platform --search "head:onboard/<alias>" --json url,state
```

Do not try to create the namespace yourself. The onboarding PR is the source of truth — base layer + replacements produce it as a Flux-reconciled artifact, not an agent-side `kubectl apply`.

**Edge case — PR merged but Flux has not reconciled yet:**

```sh
flux reconcile kustomization clusters --with-source -n flux-system  # forces a fast reconcile
kubectl get kustomization -n flux-system | grep harbor
```

Wait ~60s and re-check. If still missing, inspect the parent kustomization's status for reconciliation errors.

**Edge case — namespace exists but RBAC is not wired:** the base layer's `rbac.yaml` should have landed with the namespace. If `kubectl auth can-i` fails on workloads despite the namespace existing, check `kubectl get role,rolebinding -n eng-<alias>` — both should reference `<alias>` (post-replacement). If the role is missing or empty, the kustomization may have failed to apply the base; surface that and halt.

**Edge case — namespace exists but the SAs do not:** same answer. The base layer ships `engineer-service-account` and `seid-node` alongside the renamed `<alias>` reconciler SA. If any are absent, the kustomization did not fully reconcile.

## Caching pre-flight within a session

Once all six gates pass, mark pre-flight as complete for the session and skip on subsequent verbs. Halt conditions trigger a targeted re-check. For example, a `kubectl` call that returns `ExpiredToken` re-runs gate 3 (SSO), then proceeds without re-running gates 1, 2, 4–6.

Never cache across sessions; every fresh invocation runs pre-flight from gate 1.

## When pre-flight succeeds in pass 1 but fails mid-session

Common drift modes:

- **SSO expires (most frequent).** Re-run gate 3; if recovery succeeds, resume the in-flight verb.
- **kubectl context switched in another terminal.** Re-run gate 4 + gate 5. If the engineer is now on a different cluster, refuse the in-flight verb and ask them to switch back.
- **EKS access entry revoked.** Gate 5 fails. Unusual mid-session — surface and halt.
- **Namespace deleted by another engineer / Flux re-reconcile.** Gate 6 fails. Surface and halt; the engineer decides whether to re-onboard or escalate.

In every case, the recovery is to re-run the relevant gate and resume. Do not silently work around drift.

## The full new-engineer walk-through

For a literal "fresh laptop" engineer, the first session looks like:

1. Engineer says something like "set me up on harbor" or "I'm new."
2. Pre-flight gate 1 fails (no seictl). Surface install command, halt.
3. Engineer installs seictl, says "ok try again."
4. Gate 1 passes. Gate 3 detection runs: list profiles via `aws configure list-profiles`. If `$AWS_PROFILE` has a value, respect it. If the engineer has multiple profiles, ask them to pick, and frame the prompt around "this profile authenticates kubectl + observes your harbor cluster". Once chosen, validate via `aws sts get-caller-identity --profile <chosen>` — failure surfaces `aws sso login --profile <chosen>`, halt. Echo the resolved Arn.
5. Engineer runs SSO login. Continue.
6. Gate 4 fails (no kubeconfig). Run `aws eks update-kubeconfig --name harbor --region eu-central-1 --profile <chosen>` directly (using the gate-3 profile). Continue.
7. Gate 5 fails (no access entry). Surface "ask platform team in #harbor-onboarding," halt.
8. Engineer pings the channel, gets the access entry. Comes back, says "ok try again." Gate 5 now passes, so run its resource-CRD sub-gate here. This is the first point in the ramp that can reach the cluster.
9. Gate 6 fails (namespace does not exist). Enter First Run: prompt for alias (default from `$USER`), validate the regex, generate the PR body following the fromtherain pattern, open the PR via `gh pr create`. Surface the PR URL and halt. "Merge this; ping me when done."
10. Engineer merges, says "merged."
11. Poll gate 6 — namespace + RBAC + workload SA + Flux watcher all reconcile from the same merge (~60s). Once `kubectl get namespace eng-<alias>` returns 0, gate 6 passes.
12. All gates pass. "You are on the rails. Try `spin up a chain of 4 validators with image X`."

Total elapsed wall-clock: typically one platform-team turnaround (gate 5) plus one PR merge (gate 6). Pre-flight in the warm case (returning engineer): <2s.

## What pre-flight is *not* responsible for

- **Provisioning the EKS access entry** (gate 5). That is a platform-team action. Pre-flight detects, surfaces, halts.
- **Granting AWS SSO permissions** (gate 3). The engineer's IdP / IAM Identity Center role determines what SSO returns. Pre-flight only verifies the session is live.
- **Validating image refs** (`seictl network|node apply` does this when invoked — image not in registry surfaces as a `metav1.Status` on stderr). Pre-flight only confirms the registry is reachable; per-image digest resolution is a procedure step.
- **Cluster headroom checks.** That is a procedure step in the chain-spinup flow, not a pre-flight gate.
