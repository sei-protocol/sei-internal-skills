# Pre-flight: getting an engineer on the rails

The first job of any new session is to confirm — or establish — that the engineer can actually use the platform. Pre-flight is a sequenced ramp from "fresh laptop" to "ready to run the GitOps flow." Each gate either passes (continue), fails with an in-line recovery that runs through to completion, or fails with an out-of-band recovery to surface and halt on.

Last verified: 2026-05-04 against shipped seictl v1, harbor EKS cluster (eu-central-1), and the personal-cells namespace topology.

## Why pre-flight is a ramp, not just a gate

A pre-flight that just rejects on missing prereqs gives engineers an error and walks away. The value here is in being the rails — running pre-flight should *land the engineer in the ready state*, not just diagnose the gap. Where the recovery is in reach (kubeconfig write, worktree refresh, identity-file create, onboarding PR), execute it and continue. Where the recovery is out-of-band (SSO login in another terminal, EKS access entry from the platform team, PR merge), surface the exact next step and halt cleanly.

The end state pre-flight delivers:

- `seictl` on PATH
- AWS SSO session active
- `harbor` kubectl context present and authorized
- Platform repo locatable, on a clean worktree pointing at `main`, fresh with origin
- `~/.seictl/engineer.json` populated
- `eng-<alias>` namespace exists and is reconciled by Flux
- `eng-<alias>-workspace` branch exists with a per-engineer Flux Kustomization watching it

That's the floor for the GitOps headline procedure. Below this floor, no procedure can proceed safely.

## The eight gates

### Gate 1: `seictl` installed

**Verifies:** `seictl --version` returns 0 with a v1.x version string.

**Why:** every cluster-facing verb is a `seictl` invocation. Without it, nothing else matters.

**Recovery (out-of-band):** install via the seictl release page. Surface:

```sh
# macOS
brew install sei-protocol/tap/seictl
# Or download release binary directly
# https://github.com/sei-protocol/seictl/releases/latest
```

Halt until `seictl --version` succeeds.

### Gate 2: AWS SSO session active

**Verifies:** `aws sts get-caller-identity` returns 0 with an `Arn` field.

**Why:** harbor's EKS auth, ECR image pulls, and the IAM provisioning that `seictl onboard` performs all require live AWS credentials. SSO sessions expire (default 12h); refreshing is a one-liner.

**Recovery (out-of-band):** `aws sso login --profile sei`. `sei` is the canonical profile name for harbor's AWS account. If the engineer's `~/.aws/config` doesn't have a `sei` profile yet (truly fresh laptop), surface `aws configure sso` and route them through profile setup, with the SSO start URL and the `sei` profile name pre-populated.

**Edge case — expired session mid-run:** SSO can expire between verbs (default 12h). Halt conditions catch this (any AWS call returns `ExpiredToken`); re-run gate 2 and resume.

**Edge case — different `AWS_PROFILE` in the shell:** if the engineer's shell has `AWS_PROFILE` set to something other than `sei`, `seictl context` will surface an unexpected `awsAccount`. Re-run gate 2 with `AWS_PROFILE=sei` explicitly (`AWS_PROFILE=sei aws sts get-caller-identity` and onward).

### Gate 3: harbor kubeconfig context exists

**Verifies:** `kubectl config get-contexts -o name` lists `harbor` (or `arn:aws:eks:eu-central-1:...:cluster/harbor`).

**Why:** kubectl needs the cluster endpoint, CA cert, and auth provider config in the kubeconfig before any `kubectl ...` command can resolve harbor.

**Recovery (in-band):**

```sh
aws eks update-kubeconfig --name harbor --region eu-central-1
```

This writes the harbor context into `~/.kube/config` (idempotent — re-running is safe). The skill executes this directly on a fresh laptop, then re-checks the gate and continues.

**Edge case — engineer prefers a non-default kubeconfig path:** respect `$KUBECONFIG`. The `update-kubeconfig` command writes to whichever file `$KUBECONFIG` points at (or `~/.kube/config` if unset). The skill doesn't override.

### Gate 4: kubectl can reach harbor (EKS access entry granted)

**Verifies:** `kubectl auth can-i list namespaces --context=harbor` returns `yes`.

**Why:** the EKS cluster authorizes principals via *access entries* — separate from kubeconfig presence. A fresh principal with a valid kubeconfig can still get `Forbidden` on every kubectl call until the access entry is added.

**Recovery (out-of-band):** the platform team grants the access entry. This is *not* something `seictl onboard` does today — onboarding creates IAM policy + Pod Identity association for in-cluster workloads, which is different from user-level kubectl auth. Surface:

> Your AWS principal can't list resources on harbor. This means the EKS access entry isn't in place yet. Ask the platform team to add you — file a one-line request in `#harbor-onboarding` with your AWS principal ARN (visible in `aws sts get-caller-identity`).

Halt until the access entry lands. The platform team typically turns this around same-day.

**Edge case — read-only access vs full access:** some engineers may have a read-only access entry (can `get` / `list` / `describe` but not `create`). Gate 4 only verifies *some* kubectl reach. Side-effecting verbs (`seictl onboard --apply`) will fail later with `Forbidden`; surface that as a separate gap when it surfaces.

### Gate 5: platform repo worktree on main, fresh

**Verifies:** the `sei-protocol/platform` repo is locatable on disk, has a worktree pointing at `main` with a clean working tree, and that worktree is at or behind `origin/main` (fast-forwardable).

**Why:** every engineer-facing flow that touches git — onboarding (which generates a namespace+RBAC PR), GitOps chain spinup (which writes manifests to the workspace branch) — needs a clean main-checked-out workspace to branch from. The engineer's primary checkout often has WIP on a different branch; trampling that is unacceptable. A separate worktree gives a clean, isolated workspace without disturbing the primary.

**Detection (in order):**

1. Locate the repo: prefer `$SEI_PLATFORM_REPO` if set; fall back to `~/sei-workspace/platform`, then `~/platform`. The first existing path that resolves to a `sei-protocol/platform` clone wins.
2. Confirm it's a git repo with the expected origin remote (`git -C <path> remote get-url origin` returns a URL containing `sei-protocol/platform`).
3. Look for a worktree on `main`: parse `git -C <path> worktree list --porcelain` for an entry with `branch refs/heads/main`. If the primary checkout is on main *and* clean, that counts. Otherwise, look for `~/.seictl/worktrees/platform-main` (the canonical isolated-worktree location).
4. Confirm the worktree's `main` is fast-forward with `origin/main`: `git -C <worktree> fetch origin && git -C <worktree> merge-base --is-ancestor main origin/main`.

**Recovery (in-band where possible):**

- **Repo not located.** Surface:
  ```sh
  git clone git@github.com:sei-protocol/platform.git ~/sei-workspace/platform
  # Or set SEI_PLATFORM_REPO to point at an existing clone
  ```
  Halt until the clone exists.

- **Repo located, but no worktree on main.** Create the canonical isolated worktree:
  ```sh
  git -C <repo> worktree add ~/.seictl/worktrees/platform-main main
  ```
  Continue using that worktree as the platform-repo path for subsequent steps.

- **Primary checkout is on main but dirty.** Don't override engineer state. Either fall through to creating an isolated worktree (preferred — leaves the primary alone), or halt and surface:
  > Your primary platform checkout is on main with uncommitted changes. Stash or commit before continuing, or set `$SEI_PLATFORM_REPO` to a different clone.

- **Worktree main is stale.** Run `git -C <worktree> pull --ff-only origin main`. Continue.

- **Local main has unpushed commits diverging from `origin/main`.** Halt and surface:
  > Local main is ahead of origin/main by N commits. Investigate before continuing — onboarding will branch off main and your local commits would land in the onboarding PR.

**Edge case — engineer prefers a non-default repo location:** respect `$SEI_PLATFORM_REPO` absolutely. If the env var is set but the path doesn't resolve to a valid clone, surface the discrepancy and halt — don't silently fall back.

**Edge case — multiple worktrees on main:** if `git worktree list` shows main checked out in two places (the primary + an isolated worktree), prefer the isolated worktree (`~/.seictl/worktrees/platform-main`) for any agent-driven operations. The primary belongs to the engineer.

**Edge case — the canonical isolated worktree exists but points at a different branch:** prune and recreate.
```sh
git -C <repo> worktree remove ~/.seictl/worktrees/platform-main
git -C <repo> worktree add ~/.seictl/worktrees/platform-main main
```

### Gate 6: identity file exists

**Verifies:** `~/.seictl/engineer.json` exists, parses as JSON, has `alias` and `name` fields, and `alias` matches the regex `^[a-z]([a-z0-9-]{0,28}[a-z0-9])?$`.

**Why:** every per-engineer artifact (namespace, workspace branch, S3 prefix, IAM role) keys off the alias. The identity file is the canonical source.

**Recovery (in-band, partially):** route to **First Run**. The skill prompts for alias + name (defaulting from `$USER` and `git config user.name`), writes `~/.seictl/engineer.json`, then runs `seictl onboard --apply`. Onboarding opens a PR for the namespace + RBAC manifests; the engineer must merge it. The skill surfaces the PR URL and halts pending merge — Flux reconcile happens automatically once merged (~60s).

**Edge case — corrupted identity file:** the file exists but doesn't parse, or has an invalid alias. Halt and prompt the engineer to inspect / delete / recreate. Don't try to repair.

**Edge case — alias mismatch with namespace:** if `~/.seictl/engineer.json` says `alias=foo` but `kubectl get namespace eng-foo` returns NotFound while `kubectl get namespace eng-bar` exists, something has drifted. Halt and surface both states; let the engineer decide.

### Gate 7: namespace reconciled

**Verifies:** `kubectl get namespace eng-<alias>` returns 0.

**Why:** every workload the engineer creates lands in their namespace. If the namespace doesn't exist, every subsequent kubectl call fails.

**Recovery (out-of-band, then automatic):** if the onboarding PR hasn't been merged, surface the PR URL (captured in `~/.seictl/engineer.json` or recoverable via `gh pr list --search seictl/onboard-<alias>`) and halt. Once merged, Flux reconciles in ~60s — offer to poll until the namespace appears.

**Edge case — onboarding PR was merged but Flux is unhealthy:** check `kubectl get kustomization -A | grep harbor-validation-shared-rules` (or whichever Kustomization owns `clusters/harbor/engineers/`) for `Ready=True`. If reconcile is failing, surface the Kustomization name + status and halt.

**Edge case — namespace exists but quota/NetworkPolicy not applied:** the personal-cells security posture (quotas, NetworkPolicy, admission) is layered on by the cells project; on early namespaces these may lag. Gate 7 only checks namespace existence, not policy lamination — if a workload later fails admission, the halt conditions catch it.

### Gate 8: workspace branch ready (GitOps flow only)

**Verifies:** `git ls-remote origin eng-<alias>-workspace` returns a ref.

**Why:** the GitOps headline procedure pushes manifests to this branch. If it doesn't exist, the push fails and the engineer can't use the GitOps flow.

**Recovery (in-band, manual until [Tide#25] item 2 ships):** the planned `seictl onboard` extension provisions the workspace branch + per-engineer Flux Kustomization automatically. Until that lands, surface the manual sequence:

```sh
# In the engineer's local platform repo checkout
git checkout main
git pull
git checkout -b eng-<alias>-workspace
mkdir -p clusters/harbor/eng/<alias>
touch clusters/harbor/eng/<alias>/.gitkeep
git add clusters/harbor/eng/<alias>/.gitkeep
git commit -m "chore: bootstrap eng-<alias> workspace"
git push origin eng-<alias>-workspace
```

Then ask the platform team to add a Flux Kustomization watching the branch. (When the seictl onboard extension ships, this whole gate's recovery becomes one command.)

**Halt mode:** if gate 8 fails, the GitOps flow is unavailable but pre-flight gates 1–7 still pass. Offer the legacy `--apply` escape hatch *only if the engineer explicitly requests it* (per the steer-first rule in SKILL.md). Don't volunteer `--apply` here — surface the manual workspace bootstrap above and let the engineer choose.

## Caching pre-flight within a session

Once all eight gates pass, mark pre-flight as complete for the session and skip on subsequent verbs. Halt conditions trigger a targeted re-check — e.g., a `kubectl` call that returns `ExpiredToken` re-runs gate 2 (SSO), then proceeds without re-running gates 1, 3–8.

Never cache across sessions; every fresh invocation runs pre-flight from gate 1.

## When pre-flight succeeds in pass 1 but fails mid-session

Common drift modes:

- **SSO expires (most frequent).** Re-run gate 2; if recovery succeeds, resume the in-flight verb.
- **kubectl context switched in another terminal.** Re-run gate 3 + gate 4. If the engineer is now on a different cluster, refuse the in-flight verb and ask them to switch back.
- **EKS access entry revoked.** Gate 4 fails. Unusual mid-session — surface and halt.
- **Platform repo worktree state changed (e.g., main pulled in another window, dirty tree).** Re-run gate 5; recovery is auto-fetch + ff or worktree refresh.
- **Namespace deleted by another engineer / Flux re-reconcile.** Gate 7 fails. Surface and halt; the engineer decides whether to re-onboard or escalate.

In every case, the recovery is to re-run the relevant gate and resume. Don't silently work around drift.

## The full new-engineer walk-through

For a literal "fresh laptop" engineer, the first session looks like:

1. Engineer says something like "set me up on harbor" or "I'm new."
2. Pre-flight gate 1 fails (no seictl). Surface install command, halt.
3. Engineer installs seictl, says "ok try again."
4. Gate 1 passes. Gate 2 might fail (no SSO). Surface `aws sso login --profile sei`, halt.
5. Engineer runs SSO login. Continue.
6. Gate 3 fails (no kubeconfig). Run `aws eks update-kubeconfig --name harbor --region eu-central-1` directly. Continue.
7. Gate 4 fails (no access entry). Surface "ask platform team in #harbor-onboarding," halt.
8. Engineer pings the channel, gets the access entry. Comes back, says "ok try again."
9. Gate 5 fails (no platform repo). Surface `git clone git@github.com:sei-protocol/platform.git ~/sei-workspace/platform`, halt.
10. Engineer clones, says "ok." Gate 5 re-runs: now locates the repo. Primary checkout is on main, clean — gate 5 passes (no isolated worktree needed yet). If the primary had been on a different branch with WIP, would create `~/.seictl/worktrees/platform-main` instead and use that.
11. Gate 6 fails (no identity file). Enter First Run: prompt for alias + name, write `~/.seictl/engineer.json`, run `seictl onboard --apply` from the gate-5 worktree. PR is opened.
12. Surface the PR URL and halt. "Merge this; ping me when done."
13. Engineer merges, says "merged."
14. Poll gate 7 until the namespace appears (~60s). Gate 7 passes.
15. Gate 8 fails (no workspace branch). Surface the manual bootstrap (until the seictl onboard extension ships), halt.
16. Engineer runs the bootstrap, asks platform team to add the per-engineer Flux Kustomization.
17. All eight gates pass. "You're on the rails. Try `spin up a chain of 4 validators with image X`."

Total elapsed wall-clock: typically one platform-team turnaround (gate 4) plus one PR merge (gate 6–7). On a second session the entire pre-flight runs in <5s — gate 5 in particular is just `git -C <worktree> fetch + ff merge`.

## What pre-flight is *not* responsible for

- **Provisioning the EKS access entry** (gate 4). That's a platform-team action. Pre-flight detects, surfaces, halts.
- **Granting AWS SSO permissions** (gate 2). The engineer's IdP / IAM Identity Center role determines what SSO returns. Pre-flight only verifies the session is live.
- **Validating image refs** (`seictl chain up` does this when invoked). Pre-flight only confirms ECR is reachable; per-image digest resolution is a procedure step, not a pre-flight gate.
- **Cluster headroom checks.** That's a procedure step (the legacy `bench up` flow already does this). Pre-flight is about access, not capacity.
