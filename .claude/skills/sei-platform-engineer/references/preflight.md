# Pre-flight: getting an engineer on the rails

The skill's first job in any new session is to confirm — or establish — that the engineer can actually use the platform. Pre-flight is a sequenced ramp from "fresh laptop" to "ready to run the GitOps flow." Each gate either passes (continue), fails with an in-line recovery the skill walks through, or fails with an out-of-band recovery the skill surfaces and halts on.

Last verified: 2026-05-04 against shipped seictl v1, harbor EKS cluster (eu-central-1), and the personal-cells namespace topology.

## Why pre-flight is a ramp, not just a gate

A pre-flight that just rejects on missing prereqs gives engineers an error and walks away. The skill's value is in being the rails — running pre-flight should *land the engineer in the ready state*, not just diagnose the gap. Where the recovery is in the skill's reach (kubeconfig write, identity-file create, onboarding PR), the skill executes the recovery and continues. Where the recovery is out-of-band (SSO login in another terminal, EKS access entry from the platform team, PR merge), the skill surfaces the exact next step and halts cleanly.

The end state pre-flight delivers:

- `seictl` on PATH
- AWS SSO session active
- `harbor` kubectl context present and authorized
- `~/.seictl/engineer.json` populated
- `eng-<alias>` namespace exists and is reconciled by Flux
- `eng-<alias>-workspace` branch exists with a per-engineer Flux Kustomization watching it

That's the floor for the GitOps headline procedure. Below this floor, the skill cannot proceed.

## The seven gates

### Gate 1: `seictl` installed

**Verifies:** `seictl --version` returns 0 with a v1.x version string.

**Why:** every cluster-facing verb in the skill is a `seictl` invocation. Without it, nothing else matters.

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

**Recovery (out-of-band):** `aws sso login` with the appropriate profile. The engineer's profile name varies — common patterns are `harbor`, `sei-platform`, or the AWS account number. If the engineer's profile is unknown, surface `aws configure sso` and route them through profile setup.

**Edge case — expired session mid-run:** SSO can expire between verbs. Halt conditions catch this (any AWS call returns `ExpiredToken`); the skill re-runs gate 2 and resumes.

**Edge case — multiple profiles:** if the engineer's shell has `AWS_PROFILE` set to a non-harbor account, `seictl context` will surface a non-harbor `awsAccount`. The skill should re-run gate 2 with the harbor profile explicitly.

### Gate 3: harbor kubeconfig context exists

**Verifies:** `kubectl config get-contexts -o name` lists `harbor` (or `arn:aws:eks:eu-central-1:...:cluster/harbor`).

**Why:** kubectl needs the cluster endpoint, CA cert, and auth provider config in the kubeconfig before any `kubectl ...` command can resolve harbor.

**Recovery (in-band, the skill runs this):**

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

**Edge case — read-only access vs full access:** some engineers may have a read-only access entry (can `get` / `list` / `describe` but not `create`). Gate 4 only verifies *some* kubectl reach. Side-effecting verbs (`seictl onboard --apply`) will fail later with `Forbidden`; the skill surfaces that as a separate gap.

### Gate 5: identity file exists

**Verifies:** `~/.seictl/engineer.json` exists, parses as JSON, has `alias` and `name` fields, and `alias` matches the regex `^[a-z]([a-z0-9-]{0,28}[a-z0-9])?$`.

**Why:** every per-engineer artifact (namespace, workspace branch, S3 prefix, IAM role) keys off the alias. The identity file is the canonical source.

**Recovery (in-band, partially):** route to **First Run**. The skill prompts for alias + name (defaulting from `$USER` and `git config user.name`), writes `~/.seictl/engineer.json`, then runs `seictl onboard --apply`. Onboarding opens a PR for the namespace + RBAC manifests; the engineer must merge it. The skill surfaces the PR URL and halts pending merge — Flux reconcile happens automatically once merged (~60s).

**Edge case — corrupted identity file:** the file exists but doesn't parse, or has an invalid alias. Halt and prompt the engineer to inspect / delete / recreate. Don't try to repair.

**Edge case — alias mismatch with namespace:** if `~/.seictl/engineer.json` says `alias=foo` but `kubectl get namespace eng-foo` returns NotFound while `kubectl get namespace eng-bar` exists, something has drifted. Halt and surface both states; let the engineer decide.

### Gate 6: namespace reconciled

**Verifies:** `kubectl get namespace eng-<alias>` returns 0.

**Why:** every workload the engineer creates lands in their namespace. If the namespace doesn't exist, every subsequent kubectl call fails.

**Recovery (out-of-band, then automatic):** if the onboarding PR hasn't been merged, surface the PR URL (captured in `~/.seictl/engineer.json` or recoverable via `gh pr list --search seictl/onboard-<alias>`) and halt. Once merged, Flux reconciles in ~60s — the skill can offer to poll until the namespace appears.

**Edge case — onboarding PR was merged but Flux is unhealthy:** check `kubectl get kustomization -A | grep harbor-validation-shared-rules` (or whichever Kustomization owns `clusters/harbor/engineers/`) for `Ready=True`. If reconcile is failing, surface the Kustomization name + status and halt.

**Edge case — namespace exists but quota/NetworkPolicy not applied:** the personal-cells security posture (quotas, NetworkPolicy, admission) is layered on by the cells project; on early namespaces these may lag. Gate 6 only checks namespace existence, not policy lamination — if a workload later fails admission, the halt conditions catch it.

### Gate 7: workspace branch ready (GitOps flow only)

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

**Halt mode:** if gate 7 fails, the GitOps flow is unavailable but pre-flight gates 1–6 still pass. The skill can offer the legacy `--apply` escape hatch *if the engineer explicitly requests it* (per the steer-first rule in SKILL.md). The skill should not volunteer `--apply` here — surface the manual workspace bootstrap above and let the engineer choose.

## Caching pre-flight within a session

Once all seven gates pass, the skill marks pre-flight as complete for the session and doesn't re-run on subsequent verbs. Halt conditions trigger a targeted re-check — e.g., a `kubectl` call that returns `ExpiredToken` re-runs gate 2 (SSO), then proceeds without re-running gates 1, 3–7.

The skill never caches across sessions; every fresh invocation runs pre-flight from gate 1.

## When pre-flight succeeds in pass 1 but fails mid-session

Common drift modes:

- **SSO expires (most frequent).** Re-run gate 2; if recovery succeeds, resume the in-flight verb.
- **kubectl context switched in another terminal.** Re-run gate 3 + gate 4. If the engineer is now on a different cluster, refuse the in-flight verb and ask them to switch back.
- **EKS access entry revoked.** Gate 4 fails. This is unusual mid-session — surface and halt.
- **Namespace deleted by another engineer / Flux re-reconcile.** Gate 6 fails. Surface and halt; the engineer decides whether to re-onboard or escalate.

In every case, the recovery is to re-run the relevant gate and resume. The skill doesn't try to silently work around drift.

## The full new-engineer walk-through

For a literal "fresh laptop" engineer, the skill's first session looks like:

1. Engineer says something like "set me up on harbor" or "I'm new."
2. Pre-flight gate 1 fails (no seictl). Skill surfaces install command, halts.
3. Engineer installs seictl, says "ok try again."
4. Pre-flight gate 1 passes. Gate 2 might fail (no SSO). Skill surfaces `aws sso login`, halts.
5. Engineer runs SSO login. Continue.
6. Gate 3 fails (no kubeconfig). Skill runs `aws eks update-kubeconfig --name harbor --region eu-central-1` directly. Continue.
7. Gate 4 fails (no access entry). Skill surfaces "ask platform team in #harbor-onboarding," halts.
8. Engineer pings the channel, gets the access entry. Comes back, says "ok try again."
9. Gates 1–4 pass. Gate 5 fails (no identity file). Skill enters First Run: prompts for alias + name, writes identity file, runs `seictl onboard --apply`. PR is opened.
10. Skill surfaces the PR URL and halts. "Merge this; ping me when done."
11. Engineer merges, says "merged."
12. Skill polls gate 6 until the namespace appears (~60s). Gate 6 passes.
13. Gate 7 fails (no workspace branch). Skill surfaces the manual bootstrap (until the seictl onboard extension ships), halts.
14. Engineer runs the bootstrap, asks platform team to add the per-engineer Flux Kustomization.
15. All seven gates pass. The skill says: "You're on the rails. Try `spin up a chain of 4 validators with image X`."

Total elapsed wall-clock: typically one platform-team turnaround (gate 4) plus one PR merge (gate 5–6). On a second session the entire pre-flight runs in <5s.

## What pre-flight is *not* responsible for

- **Provisioning the EKS access entry** (gate 4). That's a platform-team action. Pre-flight detects, surfaces, halts.
- **Granting AWS SSO permissions** (gate 2). The engineer's IdP / IAM Identity Center role determines what SSO returns. Pre-flight only verifies the session is live.
- **Validating image refs** (`seictl chain up` does this when invoked). Pre-flight only confirms ECR is reachable; per-image digest resolution is a procedure step, not a pre-flight gate.
- **Cluster headroom checks.** That's a procedure step (the legacy `bench up` flow already does this). Pre-flight is about access, not capacity.
