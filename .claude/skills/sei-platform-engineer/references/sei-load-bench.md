# Sei-load bench

Engineer-driven load-test rendering. The skill embeds the canonical Job + ConfigMap shape; the agent substitutes specifics for the engineer's request and opens a PR against `sei-protocol/harbor-engineering-workspace`. After merge, Flux applies the manifests, the Job runs against the engineer's chain, and results land in S3.

Last verified: 2026-05-08 against `clusters/harbor/nightly/load/seiload-job.yaml` + `orchestrate.sh` + the `engineer-service-account` IAM policy (S3 PutObject on `harbor-validation-results/eng-<alias>/*`).

## Why this is in the skill, not in seictl

`seictl nd` operates on `SeiNodeDeployment` CRDs. Sei-load runs a vanilla K8s `batch/v1` Job — no CRD. The bench rendering can't use `seictl nd apply --preset <x>` the way chain spinup does. Instead, the skill carries the manifest templates and the agent fills in the per-bench specifics. PR-based delivery to `harbor-engineering-workspace` keeps the GitOps audit trail; Flux applies on merge.

The skill should match nightly's shape (`clusters/harbor/nightly/load/`) so engineer-driven benches are run-over-run-comparable with nightly results.

## Inputs the agent gathers

The agent prompts only when context doesn't supply a value:

| Input | Source / default |
|---|---|
| **Chain ID** | The chain the bench targets. Engineer specifies which existing chain (the genesis-chain SND name). The bench's RPC URL is derived: `http://<chain-id>-rpc-internal.eng-<alias>.svc:8545`. |
| **Bench tag (RUN_ID component)** | Same precedence as chain-IDs (Linear ticket → sei-chain PR → sei-load PR → commit substring → explicit `--tag`). Forms part of the S3 key and the bench-name label. |
| **sei-load image** | Default to the digest pinned in `clusters/harbor/nightly/load/cronjob.yaml`'s `SEILOAD_IMAGE` env; override with `--commit <sha>` / `--pr <n>` / `--image <full-ref>`. Resolution flow: `references/image-resolution.md`. |
| **Profile** | Default `evm_transfer` (the nightly default). Engineer can override with `--profile <name>` matching a file under `clusters/harbor/nightly/load/profiles/`. The chosen profile JSON is embedded into the rendered ConfigMap verbatim — copy the file content from the platform repo. |
| **Duration** | Default `10` (minutes). Override `--duration <minutes>`. |
| **Replicas / parallelism** | Default `1`. Bench Jobs are typically single-shot; multi-replica Jobs are an explicit override. |

## Hardcoded manifests

The agent renders **two files** into `engineers/<alias>/<bench-name>/` in the workspace repo. Substitution markers are `<UPPER_SNAKE>` placeholders the agent replaces in-place — no templating engine required.

### `seiload-configmap.yaml`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: seiload-profile-<RUN_ID>
  namespace: eng-<ALIAS>
  labels:
    app.kubernetes.io/name: seiload
    app.kubernetes.io/managed-by: sei-platform-engineer
    app.kubernetes.io/part-of: seictl-bench
    app.kubernetes.io/component: load
    sei.io/chain-id: <CHAIN_ID>
    sei.io/bench-name: <RUN_ID>
data:
  profile.json: |
    <PROFILE_JSON_CONTENT>
```

`<PROFILE_JSON_CONTENT>` is the verbatim content of `clusters/harbor/nightly/load/profiles/<profile>.json` from the platform repo (read via `gh api repos/sei-protocol/platform/contents/clusters/harbor/nightly/load/profiles/<profile>.json --jq .content | base64 -d`).

### `seiload-job.yaml`

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: seiload-<RUN_ID>
  namespace: eng-<ALIAS>
  labels:
    app.kubernetes.io/name: seiload
    app.kubernetes.io/managed-by: sei-platform-engineer
    app.kubernetes.io/part-of: seictl-bench
    app.kubernetes.io/component: load
    sei.io/chain-id: <CHAIN_ID>
    sei.io/bench-name: <RUN_ID>
  annotations:
    sei.io/image-sha: <IMAGE_SHA>
spec:
  backoffLimit: 0
  activeDeadlineSeconds: <JOB_DEADLINE_SECONDS>   # = (DURATION_MINUTES * 60) + 600
  ttlSecondsAfterFinished: 3600
  template:
    metadata:
      labels:
        app.kubernetes.io/name: seiload
        app.kubernetes.io/managed-by: sei-platform-engineer
        app.kubernetes.io/part-of: seictl-bench
        app.kubernetes.io/component: load
        sei.io/chain-id: <CHAIN_ID>
        sei.io/bench-name: <RUN_ID>
    spec:
      serviceAccountName: engineer-service-account
      restartPolicy: Never
      terminationGracePeriodSeconds: 60
      volumes:
        - name: profile
          configMap:
            name: seiload-profile-<RUN_ID>
        - name: shared
          emptyDir: {}
      containers:
        - name: seiload
          image: <SEILOAD_IMAGE>
          command: ["/bin/bash", "-c"]
          args:
            - |
              set -euo pipefail
              # Trap exit so the uploader unblocks even on failure.
              # Bash + pipefail means seiload's exit code propagates through tee —
              # if seiload crashes, the container exits non-zero, the Job
              # reports Failed (not Complete), and the agent's status poll
              # sees the right signal.
              trap 'echo done > /shared/done' EXIT
              seiload \
                --config /etc/seiload/profile.json \
                --duration=<DURATION_MINUTES>m \
                --post-summary-flush-delay=45s \
                --track-receipts=true \
                2>&1 | tee /shared/report.log
          ports:
            - name: metrics
              containerPort: 9090
              protocol: TCP
          env:
            - name: SEILOAD_RUN_ID
              value: <RUN_ID>
            - name: SEILOAD_CHAIN_ID
              value: <CHAIN_ID>
            - name: SEILOAD_RPC_URL
              value: http://<CHAIN_ID>-rpc-internal.eng-<ALIAS>.svc:8545
            - name: SEILOAD_COMMIT_ID
              value: <IMAGE_SHA>
            - name: SEILOAD_WORKLOAD
              value: engineer
          volumeMounts:
            - name: profile
              mountPath: /etc/seiload
              readOnly: true
            - name: shared
              mountPath: /shared
          resources:
            requests:
              cpu: "2"
              memory: "4Gi"
            limits:
              cpu: "4"
              memory: "8Gi"
        - name: upload-results
          image: amazon/aws-cli:2.17.43
          command: ["/bin/sh", "-c"]
          args:
            - |
              set -eu
              # Block until seiload signals completion (via trap on its exit).
              while [ ! -f /shared/done ]; do sleep 5; done
              # Upload whatever's in /shared/report.log — including partial results
              # on a failed run, so post-mortem isn't blind.
              if [ -f /shared/report.log ]; then
                aws s3 cp /shared/report.log "s3://${S3_BUCKET}/${S3_KEY}"
                echo "uploaded: s3://${S3_BUCKET}/${S3_KEY}"
              else
                echo "no report.log produced — nothing to upload"
              fi
          env:
            - name: S3_BUCKET
              value: harbor-validation-results
            - name: S3_KEY
              value: eng-<ALIAS>/<PROFILE_NAME>/<RUN_ID>/report.log
          volumeMounts:
            - name: shared
              mountPath: /shared
          resources:
            requests: { cpu: "100m", memory: "128Mi" }
            limits:   { cpu: "500m", memory: "256Mi" }
```

### Why this upload pattern

- Two sidecars share an `emptyDir`; seiload writes the report, marks completion via `/shared/done`.
- Uploader polls for the marker, runs `aws s3 cp`, exits.
- Both containers must exit for the Pod to terminate; Job's `activeDeadlineSeconds` bounds total runtime even if seiload never marks done.
- Failure mode handled: seiload's `trap '...' EXIT` writes the marker on any exit (success, error, or signal), so the uploader doesn't hang on a crashed seiload — it uploads whatever partial results are in `/shared/report.log` so post-mortem isn't blind.
- IAM: `engineer-service-account` already carries `s3:PutObject` on `harbor-validation-results/eng-<alias>/*` via `aws_iam_policy.engineer` (Pod Identity). No additional terraform.
- S3 key matches nightly's convention: `s3://harbor-validation-results/<namespace>/<profile>/<run-id>/report.log`. Engineers and nightly share the same bucket, partitioned by namespace.

## RPC targeting — derived, not looked up

The bench's RPC URL is constructed from the chain ID + namespace by convention:

```
http://<chain-id>-rpc-internal.eng-<alias>.svc:8545
```

This is the ClusterIP of the `<chain-id>-rpc` SND's internal service — see `cluster-inspection-recipes.md` recipe #1 for the field provenance. Two important properties:

1. **Render-time deterministic.** The agent doesn't need to query the live cluster to know the URL — it follows from the chain-id alone.
2. **Order-independent merge.** The bench PR can be opened before the chain PR is merged; Flux orders the apply, and the bench Job won't pull a missing service (it crashes; `activeDeadlineSeconds` bounds the failure; engineer re-applies after chain is up).

Validators (sei.io/role=validator) reject foreign tx submission — never point sei-load at them. The convention above always resolves to the RPC SND, not the validator SND.

## Bench-name (`<RUN_ID>`) format

`<RUN_ID>` is `<bench-tag>-<timestamp>`:

- `<bench-tag>` follows the same precedence as chain-IDs (see SKILL.md "Procedure: spin up an ephemeral chain" step 2): Linear ticket / PR / commit substring / explicit `--tag` / one-question fallback.
- `<timestamp>` is `$(date -u +%Y%m%d-%H%M%S)`, identical to nightly's `orchestrate.sh`.

Example: `bench-name=harbor-pr-3399-20260508-191234` for a bench tagged to sei-load PR 3399 launched on May 8, 2026 at 19:12:34 UTC.

This format means the S3 key (`eng-<alias>/<profile>/harbor-pr-3399-20260508-191234/report.log`) is itself a breadcrumb back to: which engineer ran it, which profile, which work it served, when. Don't deviate.

## PR target + path

```
sei-protocol/harbor-engineering-workspace
└── engineers/<alias>/<bench-name>/
    ├── seiload-job.yaml
    ├── seiload-configmap.yaml
    └── kustomization.yaml      # lists the two above as resources
```

`kustomization.yaml` is the standard Kustomize aggregator:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - seiload-configmap.yaml
  - seiload-job.yaml
```

If `engineers/<alias>/kustomization.yaml` exists at the top level (it should — see `onboarding-pr.md` workspace-repo scaffolding), the agent appends `<bench-name>` to its `resources` list. Otherwise the engineer's per-engineer Flux Kustomization needs an updated `path:` (rare; flag rather than rewriting).

PR title: `feat(eng/<alias>): bench <bench-name> against <chain-id>`. PR body lists chain-id, sei-load image, profile, duration, expected S3 key on completion.

## What reconciles on PR merge

- ConfigMap `seiload-profile-<RUN_ID>` lands in `eng-<alias>` with the profile JSON.
- Job `seiload-<RUN_ID>` starts immediately (Flux applies, kubelet schedules pod).
- Pod runs both containers — seiload tees `report.log`, then exits; uploader uploads to S3 then exits.
- `ttlSecondsAfterFinished: 3600` cleans up the Job + Pod 1 hour after completion. ConfigMap stays until the engineer prunes it (the workspace-repo `git rm`).

## Reporting back to the engineer

After PR merge, the agent's report:

- The S3 key the result will land at (`s3://harbor-validation-results/eng-<alias>/<profile>/<run-id>/report.log`).
- Recipes for live observation: `kubectl logs -n eng-<alias> -l sei.io/bench-name=<run-id> -c seiload -f` (tail seiload output) and `kubectl logs -n eng-<alias> -l sei.io/bench-name=<run-id> -c upload-results` (sidecar progress).
- Teardown command: `git rm -r engineers/<alias>/<bench-name>/ && git commit && git push` against the workspace repo (Flux prunes the Job + ConfigMap on next reconcile).

## Halt conditions

- **Profile JSON not in platform repo** under `clusters/harbor/nightly/load/profiles/`. Surface available profile names (`gh api repos/sei-protocol/platform/contents/clusters/harbor/nightly/load/profiles --jq '.[].name'`); halt and ask the engineer to pick.
- **Chain doesn't exist in the engineer's namespace** — `kubectl get snd <chain-id> -n eng-<alias>` returns NotFound. Engineer needs to run chain spinup first, OR the bench PR will sit waiting for the chain to land. Surface and confirm; don't proceed silently.
- **Image not in registry and CI failed** — see `image-resolution.md` halt conditions.
- **Bench-name collision** — `engineers/<alias>/<bench-name>/` already exists in the workspace repo. Halt and ask whether to bump the timestamp or pick a different tag.
- **Engineer's `gh auth status` doesn't include workflow scope** — needed for the manual-dispatch fallback in image-resolution. Surface `gh auth refresh -h github.com -s workflow` and halt.
