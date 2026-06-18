# Sei-load bench

Engineer-driven load tests run as a `Job` + `ConfigMap` pair under the engineer's namespace. Sei-load is vanilla K8s (no CRD), so the rendering lives in this skill rather than `seictl network|node`. The agent fills in the placeholders, opens a PR against `sei-protocol/harbor-engineering-workspace`, and the engineer's per-engineer Flux Kustomization reconciles the Job on merge.

## Substrate facts the manifest depends on

- **Sei-load image is distroless** — no shell, no `aws-cli`, no `bash`. Main container can't run a wrapper script. Upload happens from a separate sidecar.
- **`engineer-service-account` is namespace-admin** in `eng-<alias>` (RoleBinding to built-in `admin` ClusterRole). Grants `pods/log get` (used by the sidecar) and S3 write via Pod Identity (`aws_iam_policy.engineer`, scoped to `harbor-validation-results/eng-<alias>/*`).
- **Per-engineer Flux Kustomization SA has `update`+`patch` on `batch/jobs`** (sei-protocol/platform/clusters/harbor/engineers/base/rbac.yaml). Without those verbs, Flux's server-side apply fails on the second reconcile of any Job.
- **Profile JSON has live placeholders** — `__SEI_CHAIN_ID__` and `__RPC_ENDPOINTS__` in `clusters/harbor/nightly/load/profiles/*.json`. The agent must substitute both at render time. `__RPC_ENDPOINTS__` is the fleet of per-follower RPC URLs read across the network's `rpc` SeiNodes: `seictl node list -l sei.io/seinetwork=<id>,sei.io/role=node -o json | jq -r '[.items[].status.endpoint.evmJsonRpc | select(.)]'`. Each follower publishes its own `.status.endpoint` scalar (its stable per-node URL); there is no aggregate ClusterIP — assemble the fleet across CRs, never reconstruct a URL.
- **The chain's rpc follower SeiNodes must be `Running` at render time.** Each follower's `.status.endpoint.evmJsonRpc` is published only after that SeiNode reaches `Running` (a SeiNode has no `Ready` phase — terminal is `Running`).

## Inputs the agent gathers

| Input | Source / default |
|---|---|
| Chain ID | The SeiNetwork name in the engineer's namespace. The bench targets the network's rpc follower SeiNodes (named `<chain-id>-rpc-0 .. <chain-id>-rpc-(N-1)`, selected by `sei.io/seinetwork=<chain-id>,sei.io/role=node`). |
| Sei-load image | **Required input** — engineer specifies a PR / commit / branch / explicit `--image`; never silently default. Resolution per `references/image-resolution.md`. |
| Profile | Default `nightly_evm_transfer`. Override via `--profile <name>` matching a file in `clusters/harbor/nightly/load/profiles/`. |
| Duration (minutes) | Default 10. Override `--duration <minutes>`. |
| Bench tag (RUN_ID component) | Same precedence as chain-IDs (Linear ticket → sei-chain PR → sei-load PR → commit substring → explicit `--tag`). On a re-render, parse the existing PR branch (`feat/eng-<alias>-bench-<run-id>`) to recover the original `<RUN_ID>` rather than minting a new one. |

## Render-time prerequisites

Before writing any manifest:

1. **At least one rpc follower SeiNode exists and is `Running`.** `seictl node list -n eng-<alias> -l sei.io/seinetwork=<chain-id>,sei.io/role=node -o json | jq -r '[.items[].status.phase]'` shows at least one `Running` follower. Halt and ask the engineer to wait if not — per-follower URLs aren't published otherwise (a SeiNode has no `Ready` phase — terminal is `Running`).
2. **Fleet RPC URLs available.** `seictl node list -n eng-<alias> -l sei.io/seinetwork=<chain-id>,sei.io/role=node -o json | jq -r '[.items[].status.endpoint.evmJsonRpc | select(.)]'` returns a non-empty list (one URL per `Running` follower).
3. **Image resolved + verified in registry** per `references/image-resolution.md`'s sei-load section.

## `<RUN_ID>` derivation and re-render determinism

`<RUN_ID> = <bench-tag>-<UTC-timestamp>` on first render. Branch name is `feat/eng-<alias>-bench-<RUN_ID>`. On any re-render against the same engineer + bench-tag, the agent reuses `<RUN_ID>` from the existing branch. `gh pr list --head` requires an exact branch name, so filter via `--json` + `jq` startswith:

```sh
EXISTING=$(gh pr list --repo sei-protocol/harbor-engineering-workspace \
  --state open --json headRefName \
  --jq ".[] | select(.headRefName | startswith(\"feat/eng-${ALIAS}-bench-${BENCH_TAG}-\")) | .headRefName" \
  | head -1)
if [ -n "${EXISTING}" ]; then
  RUN_ID="${EXISTING#feat/eng-${ALIAS}-bench-}"
else
  RUN_ID="${BENCH_TAG}-$(date -u +%Y%m%d-%H%M%S)"
fi
```

`<RUN_ID>` is the join key across `SEILOAD_RUN_ID` env, `sei.io/bench-name` label, and the S3 key path. Same string in all three.

## Manifest templates

The agent renders these two files into `engineers/<alias>/bench-<RUN_ID>/` in `harbor-engineering-workspace`, plus the kustomization aggregator. Substitution markers are bare `<UPPER_SNAKE>` placeholders.

### `seiload-configmap.yaml`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: seiload-profile-<RUN_ID>
  namespace: eng-<ALIAS>
  labels:
    app.kubernetes.io/name: seiload
    app.kubernetes.io/managed-by: harbor-dev
    app.kubernetes.io/part-of: seictl-bench
    app.kubernetes.io/component: load
    sei.io/chain-id: <CHAIN_ID>
    sei.io/bench-name: <RUN_ID>
data:
  profile.json: |
    <PROFILE_JSON_SUBSTITUTED>
```

`<PROFILE_JSON_SUBSTITUTED>` is the content of `clusters/harbor/nightly/load/profiles/<profile>.json` with `seiChainId` set to the chain-id and `endpoints` set to the per-pod RPC URLs. Use `jq --argjson` rather than `sed` — robust against URL special characters and produces guaranteed-valid JSON:

```sh
RPC_ENDPOINTS=$(seictl node list -n eng-<alias> -l sei.io/seinetwork=<chain-id>,sei.io/role=node -o json \
  | jq -c '[.items[].status.endpoint.evmJsonRpc | select(.)]')

PROFILE_RAW=$(gh api repos/sei-protocol/platform/contents/clusters/harbor/nightly/load/profiles/<profile>.json \
  --jq .content | base64 -d)

PROFILE_SUBSTITUTED=$(echo "${PROFILE_RAW}" | jq \
  --arg cid "<chain-id>" \
  --argjson eps "${RPC_ENDPOINTS}" \
  '.seiChainId = $cid | .endpoints = $eps')
```

Indent `<PROFILE_JSON_SUBSTITUTED>` four spaces in the rendered ConfigMap (block scalar `|` syntax).

### `seiload-job.yaml`

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: seiload-<RUN_ID>
  namespace: eng-<ALIAS>
  labels:
    app.kubernetes.io/name: seiload
    app.kubernetes.io/managed-by: harbor-dev
    app.kubernetes.io/part-of: seictl-bench
    app.kubernetes.io/component: load
    sei.io/chain-id: <CHAIN_ID>
    sei.io/bench-name: <RUN_ID>
  annotations:
    sei.io/image-sha: <IMAGE_SHA>
spec:
  backoffLimit: 0
  activeDeadlineSeconds: <JOB_DEADLINE_SECONDS>   # = (<DURATION_MINUTES> * 60) + 660
  template:
    metadata:
      labels:
        app.kubernetes.io/name: seiload
        app.kubernetes.io/managed-by: harbor-dev
        app.kubernetes.io/part-of: seictl-bench
        app.kubernetes.io/component: load
        sei.io/chain-id: <CHAIN_ID>
        sei.io/bench-name: <RUN_ID>
    spec:
      serviceAccountName: engineer-service-account
      restartPolicy: Never
      terminationGracePeriodSeconds: 60
      shareProcessNamespace: true
      volumes:
        - name: profile
          configMap:
            name: seiload-profile-<RUN_ID>
      containers:
        - name: seiload
          image: <SEILOAD_IMAGE>
          args:
            - --config
            - /etc/seiload/profile.json
            - --duration=<DURATION_MINUTES>m
            - --post-summary-flush-delay=45s
            - --track-receipts=true
          env:
            - { name: SEILOAD_RUN_ID, value: <RUN_ID> }
            - { name: SEILOAD_CHAIN_ID, value: <CHAIN_ID> }
            - { name: SEILOAD_COMMIT_ID, value: <IMAGE_SHA> }
            - { name: SEILOAD_WORKLOAD, value: engineer }
          ports:
            - { name: metrics, containerPort: 9090, protocol: TCP }
          volumeMounts:
            - { name: profile, mountPath: /etc/seiload, readOnly: true }
          resources:
            requests: { cpu: "2", memory: "4Gi" }
            limits:   { cpu: "4", memory: "8Gi" }
        - name: upload-results
          image: amazon/aws-cli:2.17.43
          command: ["/bin/sh", "-c"]
          args:
            - |
              set -uo pipefail
              # Detect seiload's process via /proc rather than pgrep — pgrep
              # isn't installed in amazon/aws-cli. shareProcessNamespace: true
              # exposes seiload's /proc/<pid>/comm in this sidecar's view.
              # /proc/<pid>/comm is the basename of the executable (max 15
              # chars); for seiload's distroless ENTRYPOINT it's literally
              # "seiload".
              seiload_running() {
                grep -q '^seiload$' /proc/[0-9]*/comm 2>/dev/null
              }

              # Container start order isn't guaranteed in a multi-container
              # Pod. Wait up to 60s for seiload to appear before checking
              # whether it's exited.
              APPEAR_DEADLINE=$((SECONDS + 60))
              until seiload_running; do
                if [ "${SECONDS}" -ge "${APPEAR_DEADLINE}" ]; then
                  echo "seiload never appeared; uploading empty report"
                  break
                fi
                sleep 1
              done

              # Bound how long the uploader runs even if seiload never exits.
              # Pod's activeDeadlineSeconds is the outer bound; this is inner.
              MAX_WAIT=$(( <DURATION_MINUTES> * 60 + 540 ))
              ELAPSED=0
              while seiload_running; do
                if [ "${ELAPSED}" -ge "${MAX_WAIT}" ]; then
                  echo "uploader timeout after ${MAX_WAIT}s; uploading whatever's available"
                  break
                fi
                sleep 5
                ELAPSED=$((ELAPSED + 5))
              done

              # Brief flush window: kubelet writes the last buffered stdout
              # to disk after the container exits, with a small lag on
              # abnormal exit (SIGKILL, OOM). 2s is generous for the
              # post-exit flush.
              sleep 2

              # Pull seiload's captured stdout from the kubernetes API via the
              # Pod's in-cluster service-account token. Kubelet retains logs
              # in /var/log/pods/... after container exit (until Pod GC), so
              # we capture partial output on a crashed bench.
              TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
              NS=$(cat /var/run/secrets/kubernetes.io/serviceaccount/namespace)
              POD=$(hostname)
              curl -sS --cacert /var/run/secrets/kubernetes.io/serviceaccount/ca.crt \
                -H "Authorization: Bearer ${TOKEN}" \
                "https://kubernetes.default.svc/api/v1/namespaces/${NS}/pods/${POD}/log?container=seiload" \
                > /tmp/report.log || echo "log fetch failed; uploading whatever's in /tmp/report.log (may be empty)"
              aws s3 cp /tmp/report.log "s3://${S3_BUCKET}/${S3_KEY}" \
                && echo "uploaded: s3://${S3_BUCKET}/${S3_KEY}"
          env:
            - { name: S3_BUCKET, value: harbor-validation-results }
            - { name: S3_KEY, value: eng-<ALIAS>/<PROFILE_NAME>/<RUN_ID>/report.log }
          resources:
            requests: { cpu: "100m", memory: "128Mi" }
            limits:   { cpu: "500m", memory: "256Mi" }
```

### `kustomization.yaml`

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - seiload-configmap.yaml
  - seiload-job.yaml
```

The agent also appends `bench-<RUN_ID>` to `engineers/<alias>/kustomization.yaml`'s top-level `resources:` list. Without that, the per-engineer Flux Kustomization doesn't see the new task dir.

## Why this upload pattern

- **`shareProcessNamespace: true`** lets the sidecar see seiload's process by scanning `/proc/<pid>/comm`. When seiload exits — successfully, on error, or via SIGKILL from `activeDeadlineSeconds` — the comm scan returns empty and the sidecar moves to upload.
- **Logs via kubernetes API** — kubelet captures stdout in `/var/log/pods/...` regardless of how the container exited. The sidecar's `engineer-service-account` token has `pods/log get` (via the namespace-admin RoleBinding). Partial logs are uploaded on crash; nothing is silently lost.
- **Bounded `MAX_WAIT`** — the sidecar can't hang past `<DURATION_MINUTES> * 60 + 540` seconds, well inside the Job's `activeDeadlineSeconds`. If seiload truly wedges, the sidecar uploads whatever's there and exits; the Job completes (Failed if seiload is still running, Complete if both containers exited cleanly).
- **No `ttlSecondsAfterFinished`** — Flux's `prune: true` on the per-engineer Kustomization re-creates Jobs cleaned up by TTL, causing them to re-run every reconcile interval. `git rm` against the workspace repo is the only cleanup mechanism.

## PR target + path

```
sei-protocol/harbor-engineering-workspace
└── engineers/<alias>/bench-<RUN_ID>/
    ├── seiload-job.yaml
    ├── seiload-configmap.yaml
    └── kustomization.yaml
```

Branch: `feat/eng-<alias>-bench-<RUN_ID>`. PR title: `feat(eng/<alias>): bench <RUN_ID> against <chain-id>`. Body lists chain-id, sei-load image (resolved digest + source PR/commit), profile, duration, expected S3 key.

The agent appends `bench-<RUN_ID>` to `engineers/<alias>/kustomization.yaml`'s `resources:` list as part of the same PR.

## Procedure

**Canonical procedure: see `SKILL.md` → `Procedure: spin up a load test`.** This file carries the per-step templates, halt conditions, S3 conventions, and substitution recipes; the procedure steps themselves live in `SKILL.md` to keep the conversational entry path tight. When the procedure changes, edit `SKILL.md`. The named observation recipes referenced from step 11 (`bench:live-tail`, `bench:terminal-check`, `bench:teardown`) live in `references/cluster-inspection-recipes.md` under "Bench observation recipes (named)".

## Halt conditions

- **No rpc follower SeiNodes found.** `seictl node list -n eng-<alias> -l sei.io/seinetwork=<chain-id>,sei.io/role=node -o json` returns an empty `.items`. The network exists but has no rpc followers yet — the engineer must apply at least one first via `seictl node apply <chain-id>-rpc-0 --preset rpc --chain-id <chain-id> --network <chain-id>` (PR-based or direct). Halt; do not attempt to render the bench.
- **No follower `Running`** at render time. Per-follower URLs aren't published. Surface each follower's phase + offer to poll (`seictl node watch <chain-id>-rpc-<k> --until=Running` per follower) before continuing.
- **Endpoints absent** even though a follower is `Running`. Likely a pre-endpoint-publication race (`PhaseRunning` precedes a serving EVM listener). Sleep 30s and retry once; halt with the followers' full status if still empty.
- **Parent `engineers/<alias>/kustomization.yaml` missing.** The per-engineer Flux Kustomization has nothing to aggregate the new `bench-<RUN_ID>/` task dir into; merging the PR is a no-op for Flux. Onboarding (or a prior teardown sequence) didn't ship the parent kustomization. Halt; surface that the engineer's onboarding PR is incomplete or the parent file was removed manually.
- **Profile JSON not in platform repo.** Surface available profiles (`gh api repos/sei-protocol/platform/contents/clusters/harbor/nightly/load/profiles --jq '.[].name'`); ask the engineer to pick.
- **Bench-name collision** — `engineers/<alias>/bench-<RUN_ID>/` already exists with a closed PR. Halt and ask whether to bump the bench-tag or reuse.
- **Sei-load image build workflow fails.** Surface `gh run view <id> --log-failed -R sei-protocol/sei-load`; don't retry blindly.
- **PR push rejected** — engineer or another agent pushed concurrently. Don't force-push. Halt; surface `git pull --rebase`.
