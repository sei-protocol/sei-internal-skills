# Pre-flight: getting an engineer on the rails

The first job of any new session is to confirm — or establish — that the engineer can actually drive `seictl network` / `seictl node` against their namespace. Pre-flight is a sequenced ramp from "fresh laptop" to "ready to apply." Each gate either passes (continue), fails with an in-band recovery that runs through to completion, or fails with an out-of-band recovery to surface and halt on.

## Why pre-flight is a ramp, not just a gate

A pre-flight that just rejects on missing prereqs gives engineers an error and walks away. The value is in being the rails — pre-flight should *land the engineer in the ready state*, not just diagnose the gap. Where the recovery is in reach (kubeconfig write, the onboarding PR), execute it and continue. Where the recovery is out-of-band (SSO login, EKS access entry, PR merge), surface the next step and halt cleanly.

The end state pre-flight delivers:

- `seictl` ≥ v0.0.59 on PATH (the version that ships the split `network`/`node` surface)
- `yq` on PATH (the render path pipes `seictl network|node apply --dry-run` through it)
- `flux` CLI on PATH (used to force-reconcile harbor after a merge instead of waiting on the natural poll interval)
- AWS SSO session active under the engineer's chosen profile
- `harbor` kubectl context present **and current**
- kubectl can list `seinetworks` in `eng-<alias>` (proof of EKS access entry + RBAC)
- `eng-<alias>` namespace exists and is reconciled by Flux

That's the floor for `seictl network|node apply`. Below this floor, no procedure can proceed safely.

## The gates

### Gate 1: `seictl ≥ v0.0.59` installed

**Verifies:** `seictl` is on `$PATH` and ships the split `network`/`node` surface.

Two-part check:

1. `command -v seictl` returns 0.
2. `seictl node apply --help` exits 0 and the help text includes `--network`. `--network` is the peer-rail flag on the split `node` tree; it exists only in v0.0.59+, so its presence proves the binary has the split trees (the old `nd apply` had no such flag). It is the breaking-cut sentinel: an older binary that still carries `nd` but not the split trees fails this gate, which is correct — `nd` targets the deleted `SeiNodeDeployment` Kind and hard-fails at apply against new-CRD clusters. Optionally also probe `seictl network apply --help` for `--genesis-override`.

**Why:** every engineer-facing verb is a `seictl network …` / `seictl node …` invocation, and `--network` auto-wire is what makes "spin up chain + RPC fleet on the same network" a one-shot. Catching an old binary here is strictly better than a confusing `NotFound`-on-CRD at apply. **Do not weaken this gate to pass on either old or new** — that lets a broken binary through.

**Recovery (out-of-band):**

Recommended path: prebuilt binary from the GitHub releases page. Per-platform tarballs at `https://github.com/sei-protocol/seictl/releases/latest`. Pick the right asset:

```sh
# macOS (Apple Silicon)
curl -LO https://github.com/sei-protocol/seictl/releases/latest/download/seictl_Darwin_arm64.tar.gz
tar -xzf seictl_Darwin_arm64.tar.gz
sudo mv seictl /usr/local/bin/

# macOS (Intel)
curl -LO https://github.com/sei-protocol/seictl/releases/latest/download/seictl_Darwin_x86_64.tar.gz
tar -xzf seictl_Darwin_x86_64.tar.gz
sudo mv seictl /usr/local/bin/

# Linux (x86_64)
curl -LO https://github.com/sei-protocol/seictl/releases/latest/download/seictl_Linux_x86_64.tar.gz
tar -xzf seictl_Linux_x86_64.tar.gz
sudo mv seictl /usr/local/bin/

# Linux — same shape with seictl_Linux_arm64.tar.gz / seictl_Linux_x86_64.tar.gz
```

Build-from-source fallback — needed when the engineer requires a commit newer than the latest release:

```sh
git clone git@github.com:sei-protocol/seictl.git
cd seictl
make build
sudo mv build/seictl /usr/local/bin/
```

**Don't** use `brew` (no tap exists for `sei-protocol/seictl`). **Don't** use `go install` directly — seictl's go.mod requires source-tree build-args that bare `go install` doesn't pass. `make install` is also `go install` under the hood; same caveat. Use `make build` then move the binary.

Halt until both checks (PATH + `node apply --help` lists `--network`) pass.

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

**Why:** the post-merge reconcile pattern (`flux reconcile kustomization flux-system --with-source -n flux-system`) is the fast path from "PR merged" to "manifests applied in cluster." Without `flux`, the fallback is `kubectl annotate kustomization flux-system reconcile.fluxcd.io/requestedAt=$(date +%s) --overwrite -n flux-system`, which works but doesn't fetch the latest source revision in the same call.

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

Engineers configure their own profiles; don't hardcode `sei` (or any other name). Resolution sequence:

1. **`$AWS_PROFILE` is set in the environment** → respect it as an explicit choice. Validate via `aws sts get-caller-identity --profile $AWS_PROFILE` and continue. Echo:
   > Using `AWS_PROFILE=<value>` (from environment) — resolved as: `<arn>`.

2. **`$AWS_PROFILE` is unset** → list configured profiles with `aws configure list-profiles`:

   - **Zero profiles** → walk the engineer through profile setup using the canonical Sei SSO session below (don't make them guess the start URL/region). Halt until at least one profile exists.
   - **Exactly one profile** → use it directly. Echo:
     > Using AWS profile `<name>` (only one configured) — resolved as: `<arn>`.
   - **Multiple profiles** → present the list and ask the engineer to choose. Default the prompt to `sei` if it's among them (the most common harbor-account profile name); otherwise no default. Frame the prompt clearly:
     > I'll use this AWS profile to authenticate kubectl + observe your harbor cluster resources. Which profile?
     > - `sei` (suggested)
     > - `<other-1>`
     > - `<other-2>`

3. **Once chosen**, the profile name is the session's profile. Every downstream AWS-touching invocation runs with `--profile <chosen>` — `aws eks update-kubeconfig …`, `aws ecr describe-images …`, `aws s3 …`. If the parent shell doesn't already export `AWS_PROFILE`, prepend `AWS_PROFILE=<chosen>` to Bash invocations to keep the choice consistent.

The whole point: the engineer chose what's authenticating — they should be able to point at it in the echo.

#### Why this gate exists

harbor's EKS auth and ECR image pulls require live AWS credentials. SSO sessions expire (default 12h); refreshing is one command. Sessions that *look* alive (configured profile, recent login) but don't have the right *role* surface as `Forbidden` later — gate 5 catches that on the kubectl side; AWS-side permission gaps surface naturally per-operation.

**Recovery (out-of-band):** `aws sso login --profile <chosen>`. If `~/.aws/config` is empty (truly fresh laptop), route them through profile setup using the canonical Sei SSO session below.

#### Canonical Sei SSO session

A fresh-laptop engineer shouldn't have to guess the start URL or Identity Center region. Drop this `sso-session` block into `~/.aws/config`:

```ini
[sso-session sei]
sso_start_url = https://d-916729b434.awsapps.com/start
sso_region = us-west-1
sso_registration_scopes = sso:account:access
```

Then run `aws configure sso --sso-session sei` — it reuses this session and prompts for the account and role: select account **`189176372795`** (the Sei/harbor account) and your granted role. It writes a `[profile …]` that references the session. Log in any time after with `aws sso login --sso-session sei`. Note `sso_region` (`us-west-1`, where Identity Center lives) is distinct from the resulting profile's `region` (the harbor cluster's region, `eu-central-1`) — they are not the same value.

**Edge case — `Unable to locate credentials`:** a `--profile`-less `aws` call landed somewhere downstream. Every AWS-touching invocation needs `--profile <chosen>` explicit on the command (or `AWS_PROFILE=<chosen>` in the environment). Most common false-negative on this gate.

**Edge case — expired session mid-run:** SSO can expire between verbs. Halt conditions catch this (any AWS call returns `ExpiredToken`); re-run gate 3 and resume.

**Edge case — engineer's chosen profile lacks harbor permissions:** the resolved `Arn` is from a non-harbor account, or kubectl-reach (gate 5) returns Forbidden despite a valid session. Surface the Arn from the gate-3 echo and prompt the engineer to either pick a different profile or re-engage the platform team for an access-entry update.

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

`<chosen>` is the profile resolved at gate 3 — never literal `sei` (engineers configure their own). The first command is idempotent; the second sets the active context. Re-check both — `current-context` must return `harbor` (or the ARN form) before continuing.

**Edge case — engineer prefers a non-default kubeconfig path:** respect `$KUBECONFIG`. The `update-kubeconfig` command writes to whichever file `$KUBECONFIG` points at (or `~/.kube/config` if unset). Don't override.

**Edge case — context drift mid-session:** if a later kubectl call returns an unexpected cluster's resource (or fails with `cluster.local` errors), re-run gates 4 + 5 and resume.

### Gate 5: kubectl can reach harbor with engineer-side reach

**Verifies:** `kubectl auth can-i list seinetworks -n eng-<alias>` returns `yes`.

**Why:** the EKS cluster authorizes principals via *access entries* — separate from kubeconfig presence. A fresh principal with a valid kubeconfig can still get `Forbidden` on every kubectl call until the access entry is added. The check is intentionally narrow (list SeiNetworks in the engineer's namespace) — that's exactly what `seictl network apply` and `seictl network watch` need. The eng-`<alias>` Role grants `seinetwork`/`seinode` CRUD; if the migration hasn't reached the Role, this gate false-negatives — verify the Role was migrated.

**Recovery (out-of-band):** the platform team grants the access entry. Surface:

> Your AWS principal can't list seinetworks in `eng-<alias>` on harbor. This means the EKS access entry isn't in place yet. Ask the platform team to add you — file a one-line request in `#harbor-onboarding` with your AWS principal ARN (the same one gate 3 echoed when it resolved your profile).

Halt until the access entry lands. Same-day turnaround typically.

**Workflow-CRD sub-gate:** before the first `seictl workflow` invocation in a session, separately verify `kubectl auth can-i patch seinodetaskworkflows -n eng-<alias> --context=harbor` returns `yes` — `patch` is the verb server-side apply exercises. A `no` means the namespace Role predates the workflow CRD; halt all `workflow` verbs and ask the platform team via `#harbor-onboarding` to add `seinodetaskworkflows` (verbs `get`, `list`, `watch`, `create`, `patch`, `delete`, plus `seinodetaskworkflows/status` read) to the Role. The failure otherwise surfaces mid-operation as `is forbidden: ... cannot patch resource "seinodetaskworkflows"`.

**Edge case — alias not yet known.** On a brand-new engineer, the alias is captured in First Run (gate 6 path) before they have an `eng-<alias>` namespace. Run this gate against the *resolved* alias from First Run; if the engineer is mid-onboarding (PR open but not merged), it may still pass on namespace-list reach even though the namespace doesn't exist yet — gate 6 owns the namespace-existence check.

**Edge case — gate passes but `apply` later fails with `Forbidden`:** the access entry may be read-only. Surface that as a separate gap when `seictl network|node apply` returns `metav1.Status.reason=Forbidden`. The platform team escalates the access entry to write.

### Gate 6: namespace `eng-<alias>` reconciled

**Verifies:** `kubectl get namespace eng-<alias>` returns 0.

**Why:** every workload the engineer creates lives in their namespace. If the namespace doesn't exist, `seictl network|node apply` fails immediately (`metav1.Status.reason=NotFound`).

**Recovery (out-of-band, with in-band lead):** if the engineer doesn't have an onboarding PR yet, route to **First Run** (capture the alias, generate the PR body, open the PR via `gh pr create`). Surface the PR URL and halt pending merge — Flux reconciles in ~60s once merged.

If the PR is open but not merged, surface the URL and offer to poll until the namespace appears:

```sh
gh pr list --repo sei-protocol/platform --search "head:onboard/<alias>" --json url,state
```

Don't try to create the namespace yourself. The onboarding PR is the source of truth — base layer + replacements produce it as a Flux-reconciled artifact, not an agent-side `kubectl apply`.

**Edge case — PR was merged but Flux hasn't reconciled yet:**

```sh
flux reconcile kustomization clusters --with-source -n flux-system  # forces a fast reconcile
kubectl get kustomization -n flux-system | grep harbor
```

Wait ~60s and re-check. If still missing, inspect the parent kustomization's status for reconciliation errors.

**Edge case — namespace exists but RBAC isn't wired:** the base layer's `rbac.yaml` should have landed with the namespace. If `kubectl auth can-i` fails on workloads despite the namespace existing, check `kubectl get role,rolebinding -n eng-<alias>` — both should reference `<alias>` (post-replacement). If the role is missing or empty, the kustomization may have failed to apply the base; surface that and halt.

**Edge case — namespace exists but the SAs don't:** same answer. The base layer ships `engineer-service-account` and `seid-node` alongside the renamed `<alias>` reconciler SA. If any are absent, the kustomization didn't fully reconcile.

## Caching pre-flight within a session

Once all six gates pass, mark pre-flight as complete for the session and skip on subsequent verbs. Halt conditions trigger a targeted re-check — e.g., a `kubectl` call that returns `ExpiredToken` re-runs gate 3 (SSO), then proceeds without re-running gates 1, 2, 4–6.

Never cache across sessions; every fresh invocation runs pre-flight from gate 1.

## When pre-flight succeeds in pass 1 but fails mid-session

Common drift modes:

- **SSO expires (most frequent).** Re-run gate 3; if recovery succeeds, resume the in-flight verb.
- **kubectl context switched in another terminal.** Re-run gate 4 + gate 5. If the engineer is now on a different cluster, refuse the in-flight verb and ask them to switch back.
- **EKS access entry revoked.** Gate 5 fails. Unusual mid-session — surface and halt.
- **Namespace deleted by another engineer / Flux re-reconcile.** Gate 6 fails. Surface and halt; the engineer decides whether to re-onboard or escalate.

In every case, the recovery is to re-run the relevant gate and resume. Don't silently work around drift.

## The full new-engineer walk-through

For a literal "fresh laptop" engineer, the first session looks like:

1. Engineer says something like "set me up on harbor" or "I'm new."
2. Pre-flight gate 1 fails (no seictl). Surface install command, halt.
3. Engineer installs seictl, says "ok try again."
4. Gate 1 passes. Gate 3 detection runs (list profiles via `aws configure list-profiles`; if `$AWS_PROFILE` is set, respect it; if multiple are configured, ask the engineer to pick — frame the prompt around "this profile authenticates kubectl + observes your harbor cluster"). Once chosen, validate via `aws sts get-caller-identity --profile <chosen>` — failure surfaces `aws sso login --profile <chosen>`, halt. Echo the resolved Arn.
5. Engineer runs SSO login. Continue.
6. Gate 4 fails (no kubeconfig). Run `aws eks update-kubeconfig --name harbor --region eu-central-1 --profile <chosen>` directly (using the gate-3 profile). Continue.
7. Gate 5 fails (no access entry). Surface "ask platform team in #harbor-onboarding," halt.
8. Engineer pings the channel, gets the access entry. Comes back, says "ok try again."
9. Gate 6 fails (namespace doesn't exist). Enter First Run: prompt for alias (default from `$USER`), validate the regex, generate the PR body following the fromtherain pattern, open the PR via `gh pr create`. Surface the PR URL and halt. "Merge this; ping me when done."
10. Engineer merges, says "merged."
11. Poll gate 6 — namespace + RBAC + workload SA + Flux watcher all reconcile from the same merge (~60s). Once `kubectl get namespace eng-<alias>` returns 0, gate 6 passes.
12. All gates pass. "You're on the rails. Try `spin up a chain of 4 validators with image X`."

Total elapsed wall-clock: typically one platform-team turnaround (gate 5) plus one PR merge (gate 6). Pre-flight in the warm case (returning engineer): <2s.

## What pre-flight is *not* responsible for

- **Provisioning the EKS access entry** (gate 5). That's a platform-team action. Pre-flight detects, surfaces, halts.
- **Granting AWS SSO permissions** (gate 3). The engineer's IdP / IAM Identity Center role determines what SSO returns. Pre-flight only verifies the session is live.
- **Validating image refs** (`seictl network|node apply` does this when invoked — image not in registry surfaces as a `metav1.Status` on stderr). Pre-flight only confirms the registry is reachable; per-image digest resolution is a procedure step.
- **Cluster headroom checks.** That's a procedure step in the chain-spinup flow, not a pre-flight gate.
