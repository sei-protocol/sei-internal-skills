# Pre-flight: getting an engineer on the rails

The first job of any new session is to confirm — or establish — that the engineer can actually drive `seictl nd` against their namespace. Pre-flight is a sequenced ramp from "fresh laptop" to "ready to apply." Each gate either passes (continue), fails with an in-band recovery that runs through to completion, or fails with an out-of-band recovery to surface and halt on.

## Why pre-flight is a ramp, not just a gate

A pre-flight that just rejects on missing prereqs gives engineers an error and walks away. The value is in being the rails — pre-flight should *land the engineer in the ready state*, not just diagnose the gap. Where the recovery is in reach (kubeconfig write, the onboarding PR), execute it and continue. Where the recovery is out-of-band (SSO login, EKS access entry, PR merge), surface the next step and halt cleanly.

The end state pre-flight delivers:

- `seictl` ≥ v0.0.43 on PATH (the version that ships peer auto-wire)
- `yq` on PATH (the render path pipes `seictl nd apply --dry-run` through it)
- AWS SSO session active under the engineer's chosen profile
- `harbor` kubectl context present **and current**
- kubectl can list `seinodedeployments` in `eng-<alias>` (proof of EKS access entry + RBAC)
- `eng-<alias>` namespace exists and is reconciled by Flux

That's the floor for `seictl nd apply`. Below this floor, no procedure can proceed safely.

## The six gates

### Gate 1: `seictl ≥ v0.0.43` installed

**Verifies:** `seictl` is on `$PATH` and supports the `nodedeployment` verb tree with peer auto-wire.

Two-part check:

1. `command -v seictl` returns 0.
2. `seictl nodedeployment --help` exits 0. The `nodedeployment` verb tree ships in v0.0.41+; peer auto-wire (`spec.template.spec.peers[0].label.selector.sei.io/chain` populated automatically when `--chain-id` is set on the `rpc` preset) is v0.0.43+. Without v0.0.43, the rpc fleet won't peer to the genesis chain on a single `--chain-id`, and the agent has to plumb `--set` overrides — friction that's avoidable.

**Why:** every engineer-facing verb is a `seictl nd …` invocation, and the auto-wire is what makes "spin up chain + RPC fleet on the same chain-id" a one-shot. Older binaries silently drop the wiring.

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

# Linux (ARM64) / (ARMv7) — same shape with seictl_Linux_arm64.tar.gz / seictl_Linux_armv7.tar.gz
```

Build-from-source fallback — needed when the engineer requires a commit newer than the latest release:

```sh
git clone git@github.com:sei-protocol/seictl.git
cd seictl
make build
sudo mv build/seictl /usr/local/bin/
```

**Don't** use `brew` (no tap exists for `sei-protocol/seictl`). **Don't** use `go install` directly — seictl's go.mod requires source-tree build-args that bare `go install` doesn't pass. `make install` is also `go install` under the hood; same caveat. Use `make build` then move the binary.

Halt until both checks (PATH + `nodedeployment --help`) pass.

### Gate 2: `yq` installed

**Verifies:** `yq` is on `$PATH`.

```sh
command -v yq
```

**Why:** the canonical render path is `seictl nd apply --dry-run | yq -P 'del(...)'` — JSON to clean YAML with server-side fields stripped (see `ephemeral-chain-flow.md`). Without `yq`, the agent has no way to produce the workspace-repo file.

**Recovery (in-band):**

```sh
# macOS
brew install yq

# Linux (x86_64) — prebuilt binary
sudo curl -L https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64 -o /usr/local/bin/yq
sudo chmod +x /usr/local/bin/yq
```

Halt until `command -v yq` returns 0.

### Gate 3: AWS SSO session active for the engineer's chosen profile

**Verifies:** `aws sts get-caller-identity --profile <profile>` returns 0 with an `Arn` field, where `<profile>` is the engineer's chosen AWS profile (resolved per the detection flow below). After resolution, **echo the resolved Arn back to the engineer** — they should see what's about to act on the cluster.

#### Profile detection flow

Engineers configure their own profiles; don't hardcode `sei` (or any other name). Resolution sequence:

1. **`$AWS_PROFILE` is set in the environment** → respect it as an explicit choice. Validate via `aws sts get-caller-identity --profile $AWS_PROFILE` and continue. Echo:
   > Using `AWS_PROFILE=<value>` (from environment) — resolved as: `<arn>`.

2. **`$AWS_PROFILE` is unset** → list configured profiles with `aws configure list-profiles`:

   - **Zero profiles** → surface `aws configure sso` (run through profile setup). Halt until at least one profile exists.
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

**Recovery (out-of-band):** `aws sso login --profile <chosen>`. If `~/.aws/config` is empty (truly fresh laptop), `aws configure sso` and route them through profile setup.

**Edge case — `Unable to locate credentials`:** a `--profile`-less `aws` call landed somewhere downstream. Every AWS-touching invocation needs `--profile <chosen>` explicit on the command (or `AWS_PROFILE=<chosen>` in the environment). Most common false-negative on this gate.

**Edge case — expired session mid-run:** SSO can expire between verbs. Halt conditions catch this (any AWS call returns `ExpiredToken`); re-run gate 3 and resume.

**Edge case — engineer's chosen profile lacks harbor permissions:** the resolved `Arn` is from a non-harbor account, or kubectl-reach (gate 5) returns Forbidden despite a valid session. Surface the Arn from the gate-3 echo and prompt the engineer to either pick a different profile or re-engage the platform team for an access-entry update.

### Gate 4: harbor kubeconfig context exists and is current

**Verifies:** `kubectl config get-contexts -o name` lists `harbor`, AND `kubectl config current-context` returns `harbor`.

**Why:** kubectl needs the cluster endpoint, CA cert, and auth provider config in the kubeconfig before any `kubectl …` (or `seictl nd …`, which reuses kubeconfig) can resolve harbor. `seictl nd apply` has no `--context` flag; it uses whatever context is currently set. If the engineer last used a different cluster, every `seictl nd …` invocation would silently hit that cluster instead of harbor.

**Recovery (in-band):**

```sh
# Add the context if missing
aws eks update-kubeconfig --name harbor --region eu-central-1 --profile <chosen>

# Set it as current
kubectl config use-context harbor
```

`<chosen>` is the profile resolved at gate 4 — never literal `sei` (engineers configure their own). The first command is idempotent; the second sets the active context. Re-check both — `current-context` must return `harbor` before continuing.

**Edge case — engineer prefers a non-default kubeconfig path:** respect `$KUBECONFIG`. The `update-kubeconfig` command writes to whichever file `$KUBECONFIG` points at (or `~/.kube/config` if unset). Don't override.

**Edge case — context drift mid-session:** if a later kubectl call returns an unexpected cluster's resource (or fails with `cluster.local` errors), re-run gate 5 and resume.

### Gate 5: kubectl can reach harbor with engineer-side reach

**Verifies:** `kubectl auth can-i list seinodedeployments -n eng-<alias>` returns `yes`.

**Why:** the EKS cluster authorizes principals via *access entries* — separate from kubeconfig presence. A fresh principal with a valid kubeconfig can still get `Forbidden` on every kubectl call until the access entry is added. The check is intentionally narrow (list SNDs in the engineer's namespace) — that's exactly what `seictl nd apply` and `seictl nd watch` need.

**Recovery (out-of-band):** the platform team grants the access entry. Surface:

> Your AWS principal can't list seinodedeployments in `eng-<alias>` on harbor. This means the EKS access entry isn't in place yet. Ask the platform team to add you — file a one-line request in `#harbor-onboarding` with your AWS principal ARN (the same one gate 4 echoed when it resolved your profile).

Halt until the access entry lands. Same-day turnaround typically.

**Edge case — alias not yet known.** On a brand-new engineer, the alias is captured in First Run (gate 6 path) before they have an `eng-<alias>` namespace. Run gate 6 against the *resolved* alias from First Run; if the engineer is mid-onboarding (PR open but not merged), gate 6 may still pass on namespace-list reach even though the namespace doesn't exist yet. Gate 6 covers the namespace-existence check.

**Edge case — gate passes but `apply` later fails with `Forbidden`:** the access entry may be read-only. Surface that as a separate gap when `seictl nd apply` returns `metav1.Status.reason=Forbidden`. The platform team escalates the access entry to write.

### Gate 6: namespace `eng-<alias>` reconciled

**Verifies:** `kubectl get namespace eng-<alias>` returns 0.

**Why:** every workload the engineer creates lives in their namespace. If the namespace doesn't exist, `seictl nd apply` fails immediately (`metav1.Status.reason=NotFound`).

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
12. All five gates pass. "You're on the rails. Try `spin up a chain of 4 validators with image X`."

Total elapsed wall-clock: typically one platform-team turnaround (gate 5) plus one PR merge (gate 6). Pre-flight in the warm case (returning engineer): <2s.

## What pre-flight is *not* responsible for

- **Provisioning the EKS access entry** (gate 5). That's a platform-team action. Pre-flight detects, surfaces, halts.
- **Granting AWS SSO permissions** (gate 3). The engineer's IdP / IAM Identity Center role determines what SSO returns. Pre-flight only verifies the session is live.
- **Validating image refs** (`seictl nd apply` does this when invoked — image not in registry surfaces as a `metav1.Status` on stderr). Pre-flight only confirms the registry is reachable; per-image digest resolution is a procedure step.
- **Cluster headroom checks.** That's a procedure step in the chain-spinup flow, not a pre-flight gate.
