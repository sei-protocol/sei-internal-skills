# Image resolution

Canonical recipes for translating an engineer's request — "PR 3399", "commit abc1234", "the latest on main" — into a pinned, verifiable container image. Two repos, near-identical pattern; this doc covers both side-by-side because the differences (registry, auth, tag format) are small and the resolve+wait+verify shape is the same.

Last verified: 2026-05-08 against `sei-protocol/sei-chain` `.github/workflows/ecr.yml` and `sei-protocol/sei-load` `.github/workflows/containerize.yaml`.

## Image conventions

| Repo | Used by | Registry | Tag format | Auto-built on |
|---|---|---|---|---|
| `sei-protocol/sei-chain` | `seid` (chain pods) | `189176372795.dkr.ecr.us-east-2.amazonaws.com/sei/sei-chain` | `<ref-or-sha>` (regular) and `mock-<ref-or-sha>` (mock_balances build flag) | push to `main`, push to `release/**` |
| `sei-protocol/sei-load` | `sei-load` (bench Job) | `ghcr.io/sei-protocol/sei-load` | `sha-<full-commit-sha>` | push to `main`, every PR push, `v*` tags |

Both workflows accept `workflow_dispatch` for manual builds — used when the agent needs an image for an unpushed commit, or when CI hasn't completed yet.

## Resolution flow (same shape, both repos)

The agent's job for a given engineer input ("PR 3399" / "commit abc1234" / "branch main"):

1. **Resolve to a full commit SHA.**
2. **Construct the expected image tag** per the repo's tag format.
3. **Try the image.** If it pulls cleanly, done — proceed with the resolved digest.
4. **If the image isn't there yet** (`manifest unknown` / `not found` / `Forbidden` from the registry), find or trigger the build workflow.
5. **Watch the build to completion**, then re-try the image.

Never let the agent invent a tag. Either the workflow run produces it (auto or dispatched) or the resolve fails.

## sei-load (`ghcr.io/sei-protocol/sei-load`)

```sh
# --- Resolve ---
# From a PR
SHA=$(gh pr view <pr-number> -R sei-protocol/sei-load --json headRefOid -q .headRefOid)

# From a short or full commit
SHA=$(gh api repos/sei-protocol/sei-load/commits/<short-or-full> --jq .sha)

# From a branch tip
SHA=$(gh api repos/sei-protocol/sei-load/commits/<branch> --jq .sha)

IMAGE="ghcr.io/sei-protocol/sei-load:sha-${SHA}"

# --- Try ---
# Probe the image without pulling: HEAD the GHCR manifest.
# Authenticated GHCR pulls use the engineer's token; if the engineer hasn't
# authenticated, the pod will fail when it tries to pull. Surfacing that early
# is fine — it's the same recovery either way.
docker manifest inspect "${IMAGE}" >/dev/null 2>&1 && echo "image present"

# --- Trigger if missing ---
# For PRs: a workflow run almost certainly already exists (auto-built on PR
# push). Find the most recent run for this SHA, get its database ID:
RUN_ID=$(gh run list -R sei-protocol/sei-load --workflow=containerize.yaml \
  --json databaseId,headSha --limit 20 \
  | jq -r --arg sha "${SHA}" '.[] | select(.headSha == $sha) | .databaseId' \
  | head -1)

# For unpushed commits / no existing run: dispatch manually.
if [ -z "${RUN_ID}" ]; then
  gh workflow run containerize.yaml -R sei-protocol/sei-load -f sha=${SHA}
  # Dispatched runs take a beat to register; pause then list, filtering by
  # SHA — `--limit 1` alone could grab another run triggered concurrently.
  sleep 3
  RUN_ID=$(gh run list -R sei-protocol/sei-load --workflow=containerize.yaml \
    --json databaseId,headSha --limit 10 \
    | jq -r --arg sha "${SHA}" '.[] | select(.headSha == $sha) | .databaseId' \
    | head -1)
fi

# --- Watch ---
gh run watch ${RUN_ID} -R sei-protocol/sei-load --exit-status
# After watch returns 0, the image is in registry; re-try the docker manifest inspect.
```

## sei-chain (`189176372795.dkr.ecr.us-east-2.amazonaws.com/sei/sei-chain`)

ECR rather than GHCR; auth via the engineer's resolved AWS profile (gate 2). The workflow is `ecr.yml` and accepts `ref` (required) + optional `tag`.

```sh
# --- Resolve --- (same as sei-load)
SHA=$(gh pr view <n> -R sei-protocol/sei-chain --json headRefOid -q .headRefOid)
# or:
SHA=$(gh api repos/sei-protocol/sei-chain/commits/<short> --jq .sha)
# or branch:
SHA=$(gh api repos/sei-protocol/sei-chain/commits/<branch> --jq .sha)

# Tag follows ${tag-or-ref-or-sha}. If the engineer didn't supply --tag, the
# tag is the SHA itself (or `mock-${SHA}` for the mock_balances variant).
IMAGE="189176372795.dkr.ecr.us-east-2.amazonaws.com/sei/sei-chain:${SHA}"
MOCK_IMAGE="189176372795.dkr.ecr.us-east-2.amazonaws.com/sei/sei-chain:mock-${SHA}"

# --- Try ---
aws ecr describe-images --profile <chosen> --region us-east-2 \
  --repository-name sei/sei-chain --image-ids imageTag=${SHA} \
  --query 'imageDetails[0].imageDigest' --output text 2>/dev/null

# --- Trigger if missing ---
RUN_ID=$(gh run list -R sei-protocol/sei-chain --workflow=ecr.yml \
  --json databaseId,headSha --limit 20 \
  | jq -r --arg sha "${SHA}" '.[] | select(.headSha == $sha) | .databaseId' \
  | head -1)

if [ -z "${RUN_ID}" ]; then
  gh workflow run ecr.yml -R sei-protocol/sei-chain -f ref=${SHA}
  sleep 3
  # Filter by SHA — concurrent runs are possible during the sleep window.
  RUN_ID=$(gh run list -R sei-protocol/sei-chain --workflow=ecr.yml \
    --json databaseId,headSha --limit 10 \
    | jq -r --arg sha "${SHA}" '.[] | select(.headSha == $sha) | .databaseId' \
    | head -1)
fi

# --- Watch ---
gh run watch ${RUN_ID} -R sei-protocol/sei-chain --exit-status
```

The `mock-<SHA>` variant (build-args `GO_BUILD_TAGS=mock_balances`) is published from the same workflow run; once the run completes, both tags are in ECR.

## Branch and tag inputs

For "the latest on main":

```sh
# sei-load
SHA=$(gh api repos/sei-protocol/sei-load/commits/main --jq .sha)
IMAGE="ghcr.io/sei-protocol/sei-load:sha-${SHA}"

# sei-chain
SHA=$(gh api repos/sei-protocol/sei-chain/commits/main --jq .sha)
IMAGE="189176372795.dkr.ecr.us-east-2.amazonaws.com/sei/sei-chain:${SHA}"
```

For named release tags (`v*` on sei-load, `release/**` on sei-chain), use the tag directly without resolving — both registries publish the symbolic tag pointing at the underlying digest.

## Image input is required — never default silently

Don't ship "the default is the nightly pin" as a fallback. The whole point of an engineer-driven bench is to validate a specific change; a silent default to nightly's image makes the engineer believe they're testing their work when they're actually benching unrelated code. **Always prompt** for a PR / commit / branch / explicit `--image` if the engineer's intent doesn't already supply one. If the engineer says "I don't care, just use the latest" — accept that as `--branch main` (resolved to a specific SHA at render time and surfaced in the plan-echo so they see exactly what's running).

For reference (e.g., when the engineer asks "what's nightly running?"), the current nightly pins are in `clusters/harbor/nightly/load/cronjob.yaml`:

```sh
grep -A1 'SEID_IMAGE\|SEILOAD_IMAGE' clusters/harbor/nightly/load/cronjob.yaml
```

That's a comparison input, not a default.

## Halt conditions

- **Workflow run fails (`gh run watch ... --exit-status` returns non-zero).** Build broke. Surface `gh run view <id> --log-failed -R <repo>` for triage; halt — don't retry blindly.
- **Image still not in registry after a successful run.** Race or registry-side delay; sleep 30s and re-try the manifest probe once. If still missing, halt and surface — something's wrong with the publish step.
- **`gh workflow run` errors** — usually a permissions issue (the engineer's `gh auth status` doesn't grant `workflow` scope). Surface `gh auth refresh -h github.com -s workflow` and halt.
- **PR is on a fork** — head SHA exists but auto-build didn't run (fork-PR builds are gated). Manual dispatch on the PR's head SHA still works.
