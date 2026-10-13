# Image resolution

Canonical recipes for translating an engineer's request — "PR 3399", "commit abc1234", "the latest on main" — into a pinned, verifiable container image. The agent never invents a tag; either the build workflow produces it or the resolve fails.

## Repos and conventions

| Repo | Used for | Registry | Tag format | Auto-built on |
|---|---|---|---|---|
| `sei-protocol/sei-chain` | `seid` (chain pods) | `189176372795.dkr.ecr.us-east-2.amazonaws.com/sei/sei-chain` | `<ref-or-sha>`, `mock-<ref-or-sha>` | push to `main`, `release/**` |
| `sei-protocol/sei-load` | sei-load (bench Job) | `ghcr.io/sei-protocol/sei-load` | `sha-<full-commit-sha>` | push to `main`, `v*` tags. A pull request builds the image but does not publish it. |

Both workflows accept `workflow_dispatch`. sei-chain's builds the ref you name. sei-load's builds the head of the branch it runs on, whatever `sha` says. Both **need** an image input — never silently default to nightly's pin. A silent default makes the engineer believe they are testing their change when they are actually running unrelated code. "Just use the latest" maps to `--branch main`, resolved at render time.

## sei-chain image conventions

| | Value |
|---|---|
| Registry | `189176372795.dkr.ecr.us-east-2.amazonaws.com/sei/sei-chain` |
| Tag format | `<ref-or-sha>` (regular build), `mock-<ref-or-sha>` (mock_balances build) |
| Auto-built on | push to `main`, push to `release/**` |
| Manual dispatch | `workflow_dispatch` with required `ref` input + optional `tag` |

The same workflow run publishes the `mock-<sha>` variant (`GO_BUILD_TAGS=mock_balances`) alongside the regular tag — once the run completes, both tags are in ECR.

**Main/nightly images need no SeiDB write-mode override.** config-apply omits the write/read-mode keys, so each seid image applies its own native default. If a chain needs an explicit mode (e.g. `migrate_evm` for a SeiDB-migration chain), set a value the image accepts using the **unified key** `storage.state_commit.write_mode`. The raw `state-commit.sc-write-mode` is silently rejected. See `troubleshooting-seinode.md` → *seid CrashLoopBackOff: invalid state-commit.sc-write-mode*.

## Resolution flow

1. **Resolve to a full commit SHA** from the engineer's input.
2. **Construct the expected image tag** per the convention above.
3. **Probe the registry.** If the tag pulls cleanly, proceed.
4. **If absent** (`AccessDenied` / `ImageNotFoundException`), find or trigger the build workflow run.
5. **Watch the run to completion**, then re-probe.

```sh
# --- Resolve ---
# From a PR
SHA=$(gh pr view <pr-number> -R sei-protocol/sei-chain --json headRefOid -q .headRefOid)

# From a short or full commit
SHA=$(gh api repos/sei-protocol/sei-chain/commits/<short-or-full> --jq .sha)

# From a branch tip
SHA=$(gh api repos/sei-protocol/sei-chain/commits/<branch> --jq .sha)

IMAGE="189176372795.dkr.ecr.us-east-2.amazonaws.com/sei/sei-chain:${SHA}"
MOCK_IMAGE="189176372795.dkr.ecr.us-east-2.amazonaws.com/sei/sei-chain:mock-${SHA}"

# --- Probe ---
aws ecr describe-images --profile <chosen> --region us-east-2 \
  --repository-name sei/sei-chain --image-ids imageTag=${SHA} \
  --query 'imageDetails[0].imageDigest' --output text 2>/dev/null

# --- Find existing run for this SHA ---
RUN_ID=$(gh run list -R sei-protocol/sei-chain --workflow=ecr.yml \
  --json databaseId,headSha --limit 20 \
  | jq -r --arg sha "${SHA}" '.[] | select(.headSha == $sha) | .databaseId' \
  | head -1)

# --- Trigger if no run exists ---
# `workflow_dispatch` runs report `headSha = default-branch HEAD`, not the
# input SHA — filtering by `headSha == $sha` won't find dispatched runs.
# Filter by event type and pick the most recent.
if [ -z "${RUN_ID}" ]; then
  gh workflow run ecr.yml -R sei-protocol/sei-chain -f ref=${SHA}
  sleep 3
  RUN_ID=$(gh run list -R sei-protocol/sei-chain --workflow=ecr.yml \
    --event=workflow_dispatch --json databaseId,createdAt --limit 5 \
    | jq -r 'sort_by(.createdAt) | reverse | .[0].databaseId')
fi

# --- Watch ---
gh run watch ${RUN_ID} -R sei-protocol/sei-chain --exit-status
# After watch returns 0, the image is in ECR; re-probe.
```

`<chosen>` is the AWS profile resolved at pre-flight gate 3.

## Branch and tag inputs

"The latest on main":

```sh
SHA=$(gh api repos/sei-protocol/sei-chain/commits/main --jq .sha)
IMAGE="189176372795.dkr.ecr.us-east-2.amazonaws.com/sei/sei-chain:${SHA}"
```

For named release tags (`release/**`), the symbolic tag points at the underlying digest — use it directly without resolving.

## sei-load image conventions

```sh
# --- Resolve ---
# From a PR
SHA=$(gh pr view <pr-number> -R sei-protocol/sei-load --json headRefOid -q .headRefOid)

# From a short or full commit
SHA=$(gh api repos/sei-protocol/sei-load/commits/<short-or-full> --jq .sha)

# From a branch tip
SHA=$(gh api repos/sei-protocol/sei-load/commits/<branch> --jq .sha)

IMAGE="ghcr.io/sei-protocol/sei-load:sha-${SHA}"

# --- Probe, and build only if the image is absent ---
# GHCR public image — anonymous HEAD on the manifest works.
if ! docker manifest inspect "${IMAGE}" >/dev/null 2>&1; then
  # A pull request run builds the image but never publishes it, so there is no
  # run to wait for. Dispatch on the branch whose head is ${SHA}.
  # containerize.yaml requires a `sha` input and ignores it: it builds the head
  # of the --ref branch and tags it sha-<that head>. Check the head first, or the
  # image carries other code under a tag the engineer did not ask for.
  BRANCH=<the branch whose head is SHA, e.g. the PR's headRefName>
  [ "$(gh api repos/sei-protocol/sei-load/commits/${BRANCH} --jq .sha)" = "${SHA}" ] \
    || { echo "${BRANCH} head is not ${SHA}; halt"; exit 1; }

  # Name this dispatch's run by elimination: the dispatch runs on ${BRANCH}
  # that existed before it are listed first, so another engineer's dispatch
  # elsewhere, or an older run here, is never the one watched.
  list_runs() {
    gh run list -R sei-protocol/sei-load --workflow=containerize.yaml \
      --event=workflow_dispatch --branch "${BRANCH}" --json databaseId --limit 20
  }
  BEFORE=$(list_runs | jq -c '[.[].databaseId]')
  gh workflow run containerize.yaml -R sei-protocol/sei-load --ref "${BRANCH}" -f sha="${SHA}"
  RUN_ID=
  for _ in 1 2 3 4 5 6; do
    sleep 5
    RUN_ID=$(list_runs | jq -r --argjson before "${BEFORE}" \
      '[.[].databaseId | select(. as $id | $before | index($id) | not)] | .[0] // empty')
    [ -n "${RUN_ID}" ] && break
  done
  [ -n "${RUN_ID}" ] || { echo "no new dispatch run on ${BRANCH}; halt"; exit 1; }

  gh run watch "${RUN_ID}" -R sei-protocol/sei-load --exit-status

  # The branch can move between the head check and the build, and the image is
  # then sha-<new head>. Probe again, so that case halts here and not at render.
  docker manifest inspect "${IMAGE}" >/dev/null 2>&1 \
    || { echo "${IMAGE} is still absent; ${BRANCH} may have moved; halt"; exit 1; }
fi
```

Only a push to `main` or a `v*` tag publishes an image without a dispatch. For a sei-load PR, the image exists only after someone dispatches `containerize.yaml` on the PR's branch, which takes a few minutes. The dispatch also publishes `:<branch-slug>` into the shared package.

## Branch and tag inputs

"The latest on main":

```sh
# sei-chain
SHA=$(gh api repos/sei-protocol/sei-chain/commits/main --jq .sha)
IMAGE="189176372795.dkr.ecr.us-east-2.amazonaws.com/sei/sei-chain:${SHA}"

# sei-load
SHA=$(gh api repos/sei-protocol/sei-load/commits/main --jq .sha)
IMAGE="ghcr.io/sei-protocol/sei-load:sha-${SHA}"
```

For named release tags (`release/**` on sei-chain, `v*` on sei-load), the symbolic tag points at the underlying digest — use it directly without resolving.

## Comparison reference: nightly pins

Engineers occasionally want to compare against what nightly is running. The pins are in the platform repo:

```sh
grep -A1 'SEID_IMAGE\|SEILOAD_IMAGE' clusters/harbor/nightly/harness/cronjobs.yaml
```

That is a comparison input — not a default. Always prompt the engineer for an explicit input when their intent does not supply one.

## Halt conditions

- **Workflow run fails** (`gh run watch ... --exit-status` returns non-zero). Surface `gh run view <id> --log-failed -R <repo>` for triage; halt.
- **Image still absent from registry after a successful run.** Sleep 30s and re-probe once. If still missing, halt — something's wrong with the publish step.
- **`gh workflow run` errors with permissions.** Engineer's `gh auth status` does not grant `workflow` scope. Surface `gh auth refresh -h github.com -s workflow` and halt.
- **`aws ecr describe-images` returns `AccessDenied`.** The engineer's SSO role lacks `ecr:DescribeImages` on `arn:aws:ecr:us-east-2:189176372795:repository/sei/sei-chain`. Surface the path: contact the platform team to add the permission to the engineer's SSO permission set.
- **`docker manifest inspect` fails on `ghcr.io/sei-protocol/sei-load`** with auth required. The package may have gone private — surface and halt.
- **PR is on a fork.** A dispatch runs only on a branch of `sei-protocol/sei-load`, so it cannot build a fork's head. Halt, and ask a maintainer to push the branch to the repository.
