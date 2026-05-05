# Pre-flight: getting an engineer on the rails

The first job of any new session is to confirm — or establish — that the engineer can actually use the platform. Pre-flight is a sequenced ramp from "fresh laptop" to "ready to run the GitOps flow." Each gate either passes (continue), fails with an in-line recovery that runs through to completion, or fails with an out-of-band recovery to surface and halt on.

Last verified: 2026-05-04 against shipped seictl v1, harbor EKS cluster (eu-central-1), and the personal-cells namespace topology.

## Why pre-flight is a ramp, not just a gate

A pre-flight that just rejects on missing prereqs gives engineers an error and walks away. The value here is in being the rails — running pre-flight should *land the engineer in the ready state*, not just diagnose the gap. Where the recovery is in reach (kubeconfig write, fresh clone in CWD, identity-file create, onboarding PR), execute it and continue. Where the recovery is out-of-band (SSO login in another terminal, EKS access entry from the platform team, PR merge), surface the exact next step and halt cleanly.

The end state pre-flight delivers:

- `seictl` ≥ v0.0.40 on PATH (the version that generates Flux wiring + workspace branch as part of onboard)
- AWS SSO session active
- `harbor` kubectl context present and authorized
- Platform repo cloned into `<cwd>/seictl-platform/` (or `$SEI_PLATFORM_REPO`), on `main`, fresh with origin
- `~/.seictl/config.json` populated
- `eng-<alias>` namespace exists and is reconciled by Flux
- `eng-<alias>-workspace` branch exists, reconciled by a per-engineer Flux `Kustomization` (both created by `seictl onboard --apply`; no platform-team handoff)

That's the floor for the GitOps headline procedure. Below this floor, no procedure can proceed safely.

## The eight gates

### Gate 1: `seictl ≥ v0.0.40` installed

**Verifies:** `seictl` is on `$PATH` *and* is at least v0.0.40 (the version that generates per-engineer Flux wiring + workspace-branch creation in `onboard --apply`).

Two-part check:

1. `command -v seictl` returns 0 (POSIX-portable PATH check; `seictl --version` is **not** a real flag).
2. Feature-detect the v0.0.40 capability: `seictl onboard --help 2>&1 | grep -qi workspace` succeeds. The `workspace` mention only exists in v0.0.40+ onboard help output. If it's absent, the binary is older than v0.0.40 and `seictl onboard --apply` won't generate the Flux wiring or push the workspace branch.

**Why:** every cluster-facing verb is a `seictl` invocation, and the post-v0.0.40 onboarding contract is what the rest of the skill assumes. An older binary appears to onboard successfully but leaves Flux + workspace-branch as a manual platform-team handoff — which the agent then can't reason about correctly.

**Recovery (out-of-band):**

- **`command -v` fails (binary not installed):**
  ```sh
  brew install sei-protocol/tap/seictl
  # Or grab the release binary directly:
  # https://github.com/sei-protocol/seictl/releases/latest
  ```
- **Binary present but feature-detect fails (older than v0.0.40):**
  ```sh
  brew upgrade seictl
  # Or re-run the release-binary install above with the latest tag.
  ```

Halt until both checks pass.

### Gate 2: AWS SSO session active for the `sei` profile

**Verifies:** `aws sts get-caller-identity --profile sei` returns 0 with an `Arn` field.

**Always pass `--profile sei` (for `aws` calls) or prepend `AWS_PROFILE=sei` (for `seictl` calls).** The engineer's default profile may not have credentials configured even when the `sei` profile is active and logged in — running bare `aws sts get-caller-identity` fails with `Unable to locate credentials`, and bare `seictl <verb>` exits code 40 `aws-unavailable` even when SSO is fine. Apply the rule to *every* AWS-touching invocation downstream:

- `aws eks update-kubeconfig ... --profile sei`
- `aws ecr describe-images ... --profile sei`
- `AWS_PROFILE=sei seictl onboard ...` (seictl has no `--profile` flag; it reads from env)
- `AWS_PROFILE=sei seictl context`
- `AWS_PROFILE=sei seictl chain up ...` (etc.)

The `sei` profile is the canonical name for harbor's AWS account.

**Why:** harbor's EKS auth, ECR image pulls, and the IAM provisioning that `seictl onboard` performs all require live AWS credentials *under the sei profile*. SSO sessions expire (default 12h); refreshing is a one-liner.

**Recovery (out-of-band):** `aws sso login --profile sei`. If the engineer's `~/.aws/config` doesn't have a `sei` profile yet (truly fresh laptop), surface `aws configure sso` and route them through profile setup, with the SSO start URL and the `sei` profile name pre-populated.

**Edge case — `Unable to locate credentials`:** a `--profile`-less `aws` call landed somewhere. Re-issue with `--profile sei` explicitly. This is the most common false-negative on gate 2 — the engineer is logged into `sei` but the agent invoked the bare command.

**Edge case — `seictl` exit code 40 / `aws-unavailable`:** seictl couldn't resolve AWS credentials. Cause is almost always a missing `AWS_PROFILE=sei` prefix. seictl has no `--profile` flag; it reads from env. Re-run with `AWS_PROFILE=sei seictl <verb> ...`. The error message itself surfaces this hint, but the agent should never let the friction surface in the first place — every `seictl` invocation gets the prefix.

**Edge case — expired session mid-run:** SSO can expire between verbs (default 12h). Halt conditions catch this (any AWS call returns `ExpiredToken`); re-run gate 2 and resume.

**Edge case — different `AWS_PROFILE` in the shell:** if the engineer's shell has `AWS_PROFILE` set to something other than `sei`, `seictl context` will surface an unexpected `awsAccount`. Re-run gate 2 (and downstream AWS calls) with `AWS_PROFILE=sei` explicitly.

### Gate 3: harbor kubeconfig context exists

**Verifies:** `kubectl config get-contexts -o name` lists `harbor` (or `arn:aws:eks:eu-central-1:...:cluster/harbor`).

**Why:** kubectl needs the cluster endpoint, CA cert, and auth provider config in the kubeconfig before any `kubectl ...` command can resolve harbor.

**Recovery (in-band):**

```sh
aws eks update-kubeconfig --name harbor --region eu-central-1 --profile sei
```

`--profile sei` is required (same rule as gate 2 — bare `aws` calls may not have credentials even when the `sei` profile is logged in). This writes the harbor context into `~/.kube/config` (idempotent — re-running is safe). Execute directly on a fresh laptop, then re-check the gate and continue.

**Edge case — engineer prefers a non-default kubeconfig path:** respect `$KUBECONFIG`. The `update-kubeconfig` command writes to whichever file `$KUBECONFIG` points at (or `~/.kube/config` if unset). Don't override.

### Gate 4: kubectl can reach harbor (EKS access entry granted)

**Verifies:** `kubectl auth can-i list namespaces --context=harbor` returns `yes`.

**Why:** the EKS cluster authorizes principals via *access entries* — separate from kubeconfig presence. A fresh principal with a valid kubeconfig can still get `Forbidden` on every kubectl call until the access entry is added.

**Recovery (out-of-band):** the platform team grants the access entry. This is *not* something `seictl onboard` does today — onboarding creates IAM policy + Pod Identity association for in-cluster workloads, which is different from user-level kubectl auth. Surface:

> Your AWS principal can't list resources on harbor. This means the EKS access entry isn't in place yet. Ask the platform team to add you — file a one-line request in `#harbor-onboarding` with your AWS principal ARN (visible in `aws sts get-caller-identity --profile sei`).

Halt until the access entry lands. The platform team typically turns this around same-day.

**Edge case — read-only access vs full access:** some engineers may have a read-only access entry (can `get` / `list` / `describe` but not `create`). Gate 4 only verifies *some* kubectl reach. Side-effecting verbs (`seictl onboard --apply`) will fail later with `Forbidden`; surface that as a separate gap when it surfaces.

### Gate 5: platform repo clone in CWD, on main, fresh

**Verifies:** `<cwd>/seictl-platform/` is a `sei-protocol/platform` clone, on `main`, at or behind `origin/main` (fast-forwardable). Or `$SEI_PLATFORM_REPO` is set and resolves to a clone matching that shape.

**Why:** every engineer-facing flow that touches git — onboarding (which generates a namespace+RBAC PR), GitOps chain spinup (which writes manifests to the workspace branch) — needs a clean main-checked-out workspace to branch from. **Default to creating a fresh clone in the current working directory rather than searching for and operating on the engineer's existing checkouts.** The engineer's primary platform checkout often has WIP on a different branch; the agent should never discover or modify it. A dedicated session-scoped clone in CWD keeps state self-contained, predictable, and disposable.

**Detection (in order):**

1. If `$SEI_PLATFORM_REPO` is set, honor that path as an explicit override (skip steps 2-3 and use it directly).
2. Check `<cwd>/seictl-platform/` — does it exist as a valid clone of `sei-protocol/platform`? Verify with `git -C <cwd>/seictl-platform remote get-url origin` returning a URL containing `sei-protocol/platform`.
3. If it doesn't exist, that's a recovery path (clone fresh) — not a halt.

Once a clone is selected (existing or freshly created):

4. Confirm `main` is checked out: `git -C <clone> rev-parse --abbrev-ref HEAD` returns `main`. If a different branch is checked out, run `git -C <clone> checkout main` (the dedicated clone has no WIP to protect).
5. Confirm `main` is fast-forward with `origin/main`: `git -C <clone> fetch origin && git -C <clone> merge-base --is-ancestor main origin/main`.

**Recovery (in-band, no halts in the common case):**

- **`<cwd>/seictl-platform/` doesn't exist.** Clone it directly:
  ```sh
  git clone git@github.com:sei-protocol/platform.git <cwd>/seictl-platform
  ```
  This is fully in-band — the dedicated clone is the agent's working copy, not the engineer's primary. No confirmation needed.

- **Clone exists but is on a non-main branch.** Run `git -C <cwd>/seictl-platform checkout main` directly. Since this clone is agent-managed, no WIP-protection concerns.

- **Main is stale.** Run `git -C <cwd>/seictl-platform fetch origin && git -C <cwd>/seictl-platform pull --ff-only origin main`. Continue.

- **Local main has unpushed commits diverging from `origin/main`.** This shouldn't happen on the agent's dedicated clone (no one else commits there). If it does, halt and surface the divergence — investigate before continuing.

- **`$SEI_PLATFORM_REPO` is set but the path doesn't resolve to a valid clone.** Halt and surface the discrepancy — don't silently fall back to creating a CWD clone (the engineer's intent was explicit; honor it or stop).

**Edge case — engineer wants a single shared clone instead of one per CWD:** set `$SEI_PLATFORM_REPO=<path>` to a stable location (e.g., `~/seictl-platform`). The env var overrides the CWD-clone default and the same clone is reused across sessions. Trade-off: shared state across sessions; pick whichever fits the workflow.

**Edge case — engineer's CWD is itself the platform repo:** unlikely (the agent typically runs from a workspace dir, not inside the platform repo), but if it happens, `<cwd>/seictl-platform` would create a nested clone, which is fine — the dedicated clone is independent of the surrounding repo's state. The engineer can `gitignore` the directory or remove it after the session.

**Edge case — `<cwd>/seictl-platform` is dirty (uncommitted changes from a prior interrupted session):** halt and surface what's there. The agent doesn't auto-clean a clone that may have in-flight work; let the engineer inspect and `git stash` or `rm -rf` as appropriate.

### Gate 6: identity file exists

**Verifies:** `~/.seictl/config.json` exists, parses as JSON, has `alias` and `namespace` fields, `alias` matches the regex `^[a-z]([a-z0-9-]{0,28}[a-z0-9])?$`, file mode is `0600` (parent dir `0700` — seictl refuses to read on looser perms).

**Why:** every per-engineer artifact (namespace, workspace branch, S3 prefix, IAM role) keys off the alias. The identity file is the canonical source. The `namespace` field is the operating namespace seictl uses for cluster-facing verbs — by convention `eng-<alias>` for engineer cells, but writeable verbatim so non-engineer flows (nightly, CI) can drop a shim with whatever namespace they target.

**Recovery (in-band, partially):** route to **First Run**. Prompt for alias only (default from `$USER`); namespace is derived as `eng-<alias>` by `seictl onboard`. `seictl onboard --apply` writes the config file with mode 0600 and opens a PR for the namespace + RBAC manifests; the engineer must merge it. Surface the PR URL and halt pending merge — Flux reconcile happens automatically once merged (~60s).

**Edge case — corrupted identity file:** the file exists but doesn't parse, or has an invalid alias. Halt and prompt the engineer to inspect / delete / recreate. Don't try to repair.

**Edge case — alias mismatch with namespace:** if `~/.seictl/config.json` says `alias=foo` but `kubectl get namespace eng-foo` returns NotFound while `kubectl get namespace eng-bar` exists, something has drifted. Halt and surface both states; let the engineer decide.

### Gate 7: namespace reconciled

**Verifies:** `kubectl get namespace eng-<alias>` returns 0.

**Why:** every workload the engineer creates lands in their namespace. If the namespace doesn't exist, every subsequent kubectl call fails.

**Recovery (out-of-band, then automatic):** if the onboarding PR hasn't been merged, surface the PR URL (captured in `~/.seictl/config.json` or recoverable via `gh pr list --search seictl/onboard-<alias>`) and halt. Once merged, Flux reconciles in ~60s — offer to poll until the namespace appears.

**Edge case — onboarding PR was merged but Flux is unhealthy:** check `kubectl get kustomization -A | grep harbor-validation-shared-rules` (or whichever Kustomization owns `clusters/harbor/engineers/`) for `Ready=True`. If reconcile is failing, surface the Kustomization name + status and halt.

**Edge case — namespace exists but quota/NetworkPolicy not applied:** the personal-cells security posture (quotas, NetworkPolicy, admission) is layered on by the cells project; on early namespaces these may lag. Gate 7 only checks namespace existence, not policy lamination — if a workload later fails admission, the halt conditions catch it.

### Gate 8: workspace branch + Flux Kustomization both reconciled (GitOps flow only)

**Verifies:** two things together:

1. `git ls-remote origin eng-<alias>-workspace` returns a ref.
2. `kubectl get kustomization eng-<alias>-workspace -n flux-system -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'` returns `True`.

Both are created by `seictl onboard --apply` (≥ v0.0.40) — they land together when the onboarding PR merges. If either is missing, onboarding either hasn't run, hasn't merged, or Flux hasn't reconciled the merged manifests yet.

**Why:** the GitOps headline procedure pushes manifests to this branch and depends on Flux applying them. Both halves of the wiring are needed: the branch (so the push has somewhere to land) and the Kustomization (so the push gets reconciled). Pre-v0.0.40 the branch was an agent in-band creation and the Kustomization was a platform-team handoff — neither is true anymore. Both are now seictl's responsibility, materialized through the onboarding PR.

**Recovery (out-of-band, then automatic):** the onboarding PR is the single source. Surface its URL (recoverable from `~/.seictl/config.json` if seictl persists it, otherwise from `gh pr list --search seictl/onboard-<alias>`):

> Your onboarding PR isn't merged yet. Once you merge it, your namespace, Flux GitRepository + Kustomization, and workspace branch all come online together. I'll poll for ~60s after merge and continue when Flux reports Ready.

Offer to poll. Don't try to create the branch or the Kustomization yourself — both are part of the onboarding PR's reconciled state, and an agent-side workaround would diverge from what the PR ultimately produces.

**Edge case — branch exists but Kustomization isn't Ready=True (or NotFound):** something on the platform-repo side has gone wrong with reconciliation — likely the per-engineer Kustomization manifest didn't merge, or Flux is failing to apply it. Surface:

```sh
kubectl describe kustomization eng-<alias>-workspace -n flux-system
flux logs -n flux-system --since=5m | grep eng-<alias>
```

Halt until a human investigates.

**Edge case — Kustomization Ready=True but the branch was force-pushed elsewhere:** Flux re-reconciles to the new HEAD on next interval. Operating normally; just note that prune will delete anything no longer present.

## Caching pre-flight within a session

Once all eight gates pass, mark pre-flight as complete for the session and skip on subsequent verbs. Halt conditions trigger a targeted re-check — e.g., a `kubectl` call that returns `ExpiredToken` re-runs gate 2 (SSO), then proceeds without re-running gates 1, 3–8.

Never cache across sessions; every fresh invocation runs pre-flight from gate 1.

## When pre-flight succeeds in pass 1 but fails mid-session

Common drift modes:

- **SSO expires (most frequent).** Re-run gate 2; if recovery succeeds, resume the in-flight verb.
- **kubectl context switched in another terminal.** Re-run gate 3 + gate 4. If the engineer is now on a different cluster, refuse the in-flight verb and ask them to switch back.
- **EKS access entry revoked.** Gate 4 fails. Unusual mid-session — surface and halt.
- **Platform repo clone drift (someone touched the dedicated clone in another window, or it went stale).** Re-run gate 5; recovery is auto-fetch + ff merge against `origin/main`.
- **Namespace deleted by another engineer / Flux re-reconcile.** Gate 7 fails. Surface and halt; the engineer decides whether to re-onboard or escalate.

In every case, the recovery is to re-run the relevant gate and resume. Don't silently work around drift.

## The full new-engineer walk-through

For a literal "fresh laptop" engineer, the first session looks like:

1. Engineer says something like "set me up on harbor" or "I'm new."
2. Pre-flight gate 1 fails (no seictl). Surface install command, halt.
3. Engineer installs seictl, says "ok try again."
4. Gate 1 passes. Gate 2 might fail (no SSO). Surface `aws sso login --profile sei`, halt.
5. Engineer runs SSO login. Continue.
6. Gate 3 fails (no kubeconfig). Run `aws eks update-kubeconfig --name harbor --region eu-central-1 --profile sei` directly. Continue.
7. Gate 4 fails (no access entry). Surface "ask platform team in #harbor-onboarding," halt.
8. Engineer pings the channel, gets the access entry. Comes back, says "ok try again."
9. Gate 5 fails (no `<cwd>/seictl-platform/` clone). **Recovery is in-band** — clone fresh into `<cwd>/seictl-platform`, check out main, fetch. No halt.
10. Gate 5 now passes — the dedicated clone is the agent's working copy for the rest of the session. The engineer's primary platform checkout (if any) is never touched.
11. Gate 6 fails (no identity file). Enter First Run: prompt for alias (default from `$USER`); run `AWS_PROFILE=sei seictl onboard --apply` (the `AWS_PROFILE=sei` prefix is mandatory — see gate 2). It writes `~/.seictl/config.json` (alias + namespace=eng-<alias>, mode 0600), generates the full onboarding bundle (namespace + RBAC + Flux GitRepository + Flux Kustomization + flux-reconciler SA/RoleBinding), pushes `eng-<alias>-workspace` to origin with seeded `.gitkeep`, and opens the PR. Echo `data.workspaceBranch` from the envelope back to the engineer.
12. Surface the PR URL and halt. "Merge this; ping me when done."
13. Engineer merges, says "merged."
14. Poll gates 7 + 8 — namespace + IAM + Flux `GitRepository` + `Kustomization` all reconcile from the same merge (the workspace branch was already pushed in step 11). Both `kubectl get namespace eng-<alias>` and `kubectl get kustomization eng-<alias>-workspace -n flux-system` reach Ready ~60s post-merge.
15. All eight gates pass. "You're on the rails. Try `spin up a chain of 4 validators with image X`."

Total elapsed wall-clock: typically one platform-team turnaround (gate 4) plus one PR merge (gate 6–7). On a second session in the same CWD, gate 5 is just `git -C <cwd>/seictl-platform fetch + ff merge` (~1s); a different CWD triggers a fresh clone (~5s). Pre-flight overall in the warm case: <5s.

## What pre-flight is *not* responsible for

- **Provisioning the EKS access entry** (gate 4). That's a platform-team action. Pre-flight detects, surfaces, halts.
- **Granting AWS SSO permissions** (gate 2). The engineer's IdP / IAM Identity Center role determines what SSO returns. Pre-flight only verifies the session is live.
- **Validating image refs** (`seictl chain up` does this when invoked). Pre-flight only confirms ECR is reachable; per-image digest resolution is a procedure step, not a pre-flight gate.
- **Cluster headroom checks.** That's a procedure step (the legacy `bench up` flow already does this). Pre-flight is about access, not capacity.
