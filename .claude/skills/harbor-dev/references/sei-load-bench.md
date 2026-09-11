# Sei-load bench

Engineer-driven load tests run as a `Job` + `ConfigMap` pair under the engineer's namespace. Sei-load is vanilla K8s (no CRD), so the rendering lives in this skill rather than `seictl network|node`. The agent fills in the placeholders, opens a PR against `sei-protocol/harbor-engineering-workspace`, and the engineer's per-engineer Flux Kustomization reconciles the Job on merge.

## Substrate facts the manifest depends on

- **Sei-load image is distroless** — no shell, no `aws-cli`, no `bash`. Main container cannot run a wrapper script. Upload happens from a separate sidecar.
- **`engineer-service-account` is namespace-admin** in `eng-<alias>` (RoleBinding to built-in `admin` ClusterRole). Grants `pods/log get` (used by the sidecar) and S3 write via Pod Identity (`aws_iam_policy.engineer`, scoped to `harbor-validation-results/eng-<alias>/*`).
- **Per-engineer Flux Kustomization SA has `update`+`patch` on `batch/jobs`** (sei-protocol/platform/clusters/harbor/engineers/base/rbac.yaml). Without those verbs, Flux's server-side apply fails on the second reconcile of any Job.
- **Profile JSON has live placeholders** — `__SEI_CHAIN_ID__` and `__RPC_ENDPOINTS__` in `clusters/harbor/nightly/harness/profiles/*.json`. The agent must substitute both at render time. `__RPC_ENDPOINTS__` is the fleet of per-follower RPC URLs read across the network's `rpc` SeiNodes. Read it with:

  ```sh
  seictl node list -n eng-<alias> -l sei.io/seinetwork=<id>,sei.io/role=node -o json | jq -r '[.items[].status.endpoint.evmJsonRpc | select(.)]'
  ```

  Each follower publishes its own `.status.endpoint` (an object of per-protocol URL leaves — its stable per-node addresses). Assemble the fleet across CRs; never reconstruct a URL. The SeiNetwork's `<network>-internal` ClusterIP fronts only its validator children (which serve no EVM). Each pod surfaces its own EVM JSON-RPC/WS by design, because stateful EVM protocols do not load-balance behind kube-proxy. The per-follower list is therefore the correct target set, never an aggregate Service.
- **The chain's rpc follower SeiNodes must show phase `Running` at render time.** Each follower publishes `.status.endpoint.evmJsonRpc` only after it reaches `Running` (a SeiNode has no `Ready` phase — `Running` is the terminal one).

## Inputs the agent gathers

| Input | Source / default |
|---|---|
| Chain ID | The SeiNetwork name in the engineer's namespace. The bench targets the network's rpc follower SeiNodes (named `<chain-id>-rpc-0 .. <chain-id>-rpc-(N-1)`, selected by `sei.io/seinetwork=<chain-id>,sei.io/role=node`). |
| Sei-load image | **Required input** — engineer specifies a PR / commit / branch / explicit `--image`; never silently default. Resolution per `references/image-resolution.md`. |
| Profile | Default `nightly_evm_transfer`. Override via `--profile <name>` matching a file in `clusters/harbor/nightly/harness/profiles/`. |
| Duration (minutes) | Default 10. Override `--duration <minutes>`. |
| Bench tag (RUN_ID component) | Same precedence as chain-IDs (Linear ticket → sei-chain PR → sei-load PR → commit substring → explicit `--tag`). On a re-render, parse the existing PR branch (`feat/eng-<alias>-bench-<run-id>`) to recover the original `<RUN_ID>` rather than minting a new one. |

## Render-time prerequisites

Before writing any manifest:

1. **At least one rpc follower SeiNode is present and `Running`.** `seictl node list -n eng-<alias> -l sei.io/seinetwork=<chain-id>,sei.io/role=node -o json | jq -r '[.items[].status.phase]'` shows at least one `Running` follower. Halt and ask the engineer to wait if not — per-follower URLs are not published otherwise (a SeiNode has no `Ready` phase — terminal is `Running`).
2. **Fleet RPC URLs available.** `seictl node list -n eng-<alias> -l sei.io/seinetwork=<chain-id>,sei.io/role=node -o json | jq -r '[.items[].status.endpoint.evmJsonRpc | select(.)]'` returns a non-empty list (one URL per `Running` follower).
3. **Image resolved + verified in registry** per `references/image-resolution.md`'s sei-load section.

## Workload surface — what a profile can say

Source of truth is the `seiload` binary itself (sei-protocol/sei-load `main` at `4398610` or newer). Ask it before writing or editing a profile; do not write a knob from memory.

### Discovery verbs (run from the resolved image, no checkout, no chain)

```sh
IMG=ghcr.io/sei-protocol/sei-load@sha256:<digest>          # the image the Job will run
docker run --rm $IMG explain                                # whole embedded docs/workload-spec.md
docker run --rm $IMG explain StorageRW                      # one scenario section; unknown name lists the valid ones
docker run --rm -v "$PWD:/p:ro" $IMG validate /p/profile.json
#   → ok: 5 scenario(s), 5 runnable   (parse + Scenario.Validate + registry check + weight check; sends nothing)
```

`explain` prints Markdown; there is no `--json` schema, `--skeleton`, `manifest`, or `mcp` verb yet (PLT-1245 tracks them). The image is distroless: `validate` and `explain` are the only agent-facing verbs, and `--help` is the flag inventory. In a cluster, the same verbs run as a one-off Pod with the profile ConfigMap mounted; prefer the local `docker run` when the engineer's laptop has GHCR auth.

**Capability gate.** Images older than `4398610` have no `explain`/`validate` — `docker run --rm $IMG validate --help` exits non-zero. Fall back to the `jq -e .` syntax gate, say so in the plan echo, and expect the strict decoder to be the first real check (at Job start, seconds in). Never call a profile "validated" on an image that could not validate it.

### The eleven scenarios (`generator/scenarios/factory.go`)

| Scenario | Exercises | Knobs it reads |
|---|---|---|
| `EVMTransfer` | native transfers | fixed shape |
| `EVMTransferFast` | reduced-work send path, values multiples of 1e12, no tip | fixed shape |
| `EVMTransferNoop` | zero-value self-transfer (one account) | fixed shape |
| `ERC20` | token transfers against a deployed ERC20 | `gasPicker`, contract selection |
| `ERC20Noop` | ERC20 calls touching no balance | fixed shape |
| `ERC20Conflict` | transfers contending on one balance slot | fixed shape |
| `ERC721` | mints with unique token IDs | fixed shape |
| `AMM` | both legs of one swap pair, contending on reserves | `operations` (`swap_a_to_b`, `swap_b_to_a`; weights are repetition counts) |
| `Disperse` | one tx fanning out to many recipients | fixed shape |
| `StorageRW` | read/write/rmw on caller-chosen slots — the **cheap** OCC retry | full set: `recordCount`+`keyDistribution`, `sizeBuckets`+`sizeDistribution`, `operations` (`rmw`,`read`,`write`) |
| `DivergentRW` | slots derived during execution from contended state — the **expensive** OCC retry | full set + `fanout`, `targetSpace`, `operations` (`divergent_rmw`,`divergent_read`,`control_rmw`) |

`Prewarm` is a label, not a selectable scenario. `StorageRW` vs `DivergentRW` is the contention-shape comparison an OCC/Giga bench wants; `control_rmw` is `DivergentRW`'s matched control arm.

**A knob a scenario does not read is accepted and ignored** (open gap, PLT-1245 R-DESC-2). `keyDistribution` on `ERC20` validates, runs, and measures the unconfigured baseline. Before putting a contention knob in a profile, confirm the scenario is in the "reads it" column, or the run reports a number nobody can attribute.

### Profile shape

- **Envelope** (`config.LoadConfig`): `chainId`, `seiChainID` (note the capital `ID`), `endpoints` (required; sending shards across them), `receiptEndpoint`, `accounts`, `scenarios[]`, `mockDeploy`, `settings`, `funding`, `reportPath`, `seed`. `gasFeeCapWei` is *not* a field — the fee cap resolves from the chain at startup.
- **Scenario entry** (`config.Scenario`): `name`, `weight` (selection weight across scenarios), `accounts`, `gasPicker`, `gasFeeCapPicker`, `gasTipCapPicker`, `keyDistribution`, `sizeDistribution`, `recordCount`, `sizeBuckets`, `operations`, `fanout`, `targetSpace`, `contractKey`, `contractAddress`, `forceDeploy`.
- **Discriminated unions key on `Name`** — the only PascalCase key on the surface:

  ```json
  "keyDistribution": { "Name": "zipfian", "theta": 0.9 }     "gasPicker": { "Name": "fixed",  "Gas": 120000 }
  "keyDistribution": { "Name": "uniform" }                    "gasPicker": { "Name": "random", "Min": 100000, "Max": 200000 }
  ```

  `theta` on `uniform` is silently dropped. Omitted, `null`, `{}` and `{"Name":""}` all mean unset.
- **Settings** (`config.Settings`, CLI flag > file > default via Viper): `tps`, `statsInterval`, `inclusionReapAfter` (≥ `1s`), `bufferSize`, `trackReceipts`, `trackBlocks`, `trackUserLatency`, `prewarm`, `rampUp`, `targetGas`, `numBlocksToWrite`, `postSummaryFlushDelay`, `arrivalModel`, `gasMargin`, `gasFeeCapMultiplier`, `maxInFlight`. **`workers` is not a key** — the strict decoder rejects it (that is the nightly-profile bug PLT-1254 fixed).
- **Arrival model**: `arrivalModel: "open_loop"` schedules tx *i* at t₀ + i/λ and drops on overrun (the coordinated-omission fix; `maxInFlight` bounds it); `closed_loop` is the legacy lockstep baseline. Use `open_loop` for latency claims, `closed_loop` only to reproduce an old run.
- **`seed`**: fixes the PRNG so two runs draw the same sequence. Set it, and keep it equal across the two sides of a comparative bench; otherwise the A/B difference includes sampling noise.
- **Funding** (`config.FundingConfig`): `rootKeyFile` (preferred, a mounted Secret — not `rootKeyEnv`, which lands in `/proc/<pid>/environ`), `fundAmountWei` (a decimal **string**), `batchSize`. Requires `newAccountRate: 0`. **A chain built from a vanilla `seid` image has zero-balance generated accounts** — every value-sending scenario fails on the first tx unless the profile funds them or the seid image is a `mock_balances` build. Ask which one the engineer's image is before rendering; the nightly profiles assume the mock build.

### Bounds and cross-field rules (`Scenario.Validate` rejects; nothing clamps)

| Rule | Value / shape |
|---|---|
| `recordCount` | ≤ 10,000,000 (`MaxRecordCount`); set together with `keyDistribution` or refused |
| `sizeBuckets` | pad ≤ 128 KiB (`MaxCalldataPadBytes`); set together with `sizeDistribution` or refused |
| `fanout` | ≤ 64 (`MaxFanout`), default 8 |
| `targetSpace` | ≥ the **effective** fanout (default 1,048,576) — `targetSpace: 4` with no `fanout` is refused against the default 8 |
| zipfian `theta` | in [0, 1) |
| `random` gas picker | `Min < Max` |
| `contractAddress` / `forceDeploy` | mutually exclusive |
| unknown key | `DisallowUnknownFields`, reported against the scenario carrying it; case-variant and repeated keys still slip through |

### Reproducibility contract

Operation names, their declaration order, per-scenario config keys and the per-scenario PRNG draw order are frozen; saved workloads key on `config_sha256`. Pin in the PR description: image digest, profile SHA-256 (`sha256sum` of the substituted `profile.json`), `seed`, `arrivalModel`, `--duration`. A re-run that changes any of them is a new experiment, not a repeat.

### Job flags that come from the binary, not from taste

`--duration` (0 = run until SIGTERM; always set it in a Job), `--post-summary-flush-delay` (default 25s; the template uses 45s so Prometheus scrapes the run summary before exit), `--track-receipts`, `--report-path` (text only; the sidecar uploads it), `--nodes N` (0 = all endpoints), `--arrival-model`, `--max-in-flight`, `--inclusion-reap-after`. `--dry-run` mocks deploy and sends for a config smoke test. `/healthz` answers at bind; `/readyz` reports the startup phase (fund → deploy → prewarm) while refusing, which can take minutes against a cold chain — a `Running` pod that is not `Ready` yet is normal, not stuck.

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

`<PROFILE_JSON_SUBSTITUTED>` is the content of `clusters/harbor/nightly/harness/profiles/<profile>.json` with `seiChainId` set to the chain-id and `endpoints` set to the per-pod RPC URLs. The raw profile is deliberately **not valid JSON** — `"endpoints": [__RPC_ENDPOINTS__]` carries a bare placeholder inside the brackets — so jq cannot parse it as input. Substitute textually, then hard-validate the result with jq before it lands in the ConfigMap:

```sh
RPC_ENDPOINTS=$(seictl node list -n eng-<alias> -l sei.io/seinetwork=<chain-id>,sei.io/role=node -o json \
  | jq -c '[.items[].status.endpoint.evmJsonRpc | select(.)]')

# __RPC_ENDPOINTS__ sits INSIDE existing [ ] in the profile — insert
# comma-joined quoted URLs, no outer brackets.
EPS_INNER=$(printf '%s' "${RPC_ENDPOINTS}" | jq -r 'map(@json) | join(", ")')

PROFILE_RAW=$(gh api repos/sei-protocol/platform/contents/clusters/harbor/nightly/harness/profiles/<profile>.json \
  --jq .content | base64 -d)

PROFILE_SUBSTITUTED=$(printf '%s' "${PROFILE_RAW}" \
  | sed -e "s|__SEI_CHAIN_ID__|<chain-id>|" -e "s|__RPC_ENDPOINTS__|${EPS_INNER}|")

# Hard gate: the substituted profile must be valid JSON.
printf '%s' "${PROFILE_SUBSTITUTED}" | jq -e . >/dev/null
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

The agent also appends `bench-<RUN_ID>` to `engineers/<alias>/kustomization.yaml`'s top-level `resources:` list. Without that, the per-engineer Flux Kustomization does not see the new task dir.

## Why this upload pattern

- **`shareProcessNamespace: true`** lets the sidecar see seiload's process by scanning `/proc/<pid>/comm`. When seiload exits — successfully, on error, or via SIGKILL from `activeDeadlineSeconds` — the comm scan returns empty and the sidecar moves to upload.
- **Logs via Kubernetes API** — kubelet captures stdout in `/var/log/pods/...` regardless of how the container exited. The sidecar's `engineer-service-account` token has `pods/log get` (via the namespace-admin RoleBinding). The sidecar uploads partial logs on crash; nothing is silently lost.
- **Bounded `MAX_WAIT`** — the sidecar cannot hang past `<DURATION_MINUTES> * 60 + 540` seconds, well inside the Job's `activeDeadlineSeconds`. If seiload truly wedges, the sidecar uploads whatever's there and exits. The Job completes (Failed if seiload is still running, Complete if both containers exited cleanly).
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

**Canonical procedure: see `SKILL.md` → `Procedure: spin up a load test`.** This file carries the per-step templates, halt conditions, S3 conventions, and substitution recipes. The procedure steps themselves live in `SKILL.md` to keep the conversational entry path tight. When the procedure changes, edit `SKILL.md`. The named observation recipes referenced from step 11 (`bench:live-tail`, `bench:terminal-check`, `bench:teardown`) live in `references/cluster-inspection-recipes.md` under "Bench observation recipes (named)".

## seiload dies when its target node restarts

seiload does **not** survive a restart of the node it streams from. The block-collector websocket drops (`Error: block collector: websocket: close 1006 (abnormal closure): unexpected EOF`) and seiload exits. The upload sidecar flushes whatever partial report exists to S3, and the Job lands **Failed**. Any fleet-wide event that rolls SeiNode pods kills every in-flight bench targeting those nodes. Examples: a sidecar-image bump, a controller restart, a node update plan.

No resume exists. Recovery is a **fresh run**: re-enter the bench flow so it mints a new `<RUN_ID>`. A Job's pod template is immutable and a Failed Job never re-runs, so nobody can reuse the old name. Before starting it, confirm the chain is healthy again, or the new run dies the same way. Healthy means every follower `Running` **and** block height advancing between two `/status` reads (`catching_up=false`).

## Halt conditions

- **No rpc follower SeiNodes found.** `seictl node list -n eng-<alias> -l sei.io/seinetwork=<chain-id>,sei.io/role=node -o json` returns an empty `.items`. The network exists but has no rpc followers yet. The engineer must apply at least one first, per `references/ephemeral-chain-flow.md` step 6 (PR-based or direct). Halt; do not attempt to render the bench. That recipe carries the create-only footprint and storage-performance flags; a bare command drops them.
- **No follower `Running`** at render time. Per-follower URLs are not published. Surface each follower's phase + offer to poll (`seictl node watch <chain-id>-rpc-<k> --until=Running` per follower) before continuing.
- **Endpoints absent** even though a follower is `Running`. Likely a pre-endpoint-publication race (`PhaseRunning` precedes a serving EVM listener). Sleep 30s and retry once; halt with the followers' full status if still empty.
- **Parent `engineers/<alias>/kustomization.yaml` missing.** The per-engineer Flux Kustomization has nothing to aggregate the new `bench-<RUN_ID>/` task dir into; merging the PR is a no-op for Flux. Onboarding (or a prior teardown sequence) did not ship the parent kustomization. Halt; surface that the engineer's onboarding PR is incomplete or someone removed the parent file manually.
- **Profile JSON not in platform repo.** Surface available profiles (`gh api repos/sei-protocol/platform/contents/clusters/harbor/nightly/harness/profiles --jq '.[].name'`); ask the engineer to pick.
- **Bench-name collision** — `engineers/<alias>/bench-<RUN_ID>/` already exists with a closed PR. Halt and ask whether to bump the bench-tag or reuse.
- **Sei-load image build workflow fails.** Surface `gh run view <id> --log-failed -R sei-protocol/sei-load`; do not retry blindly.
- **PR push rejected** — engineer or another agent pushed concurrently. Do not force-push. Halt; surface `git pull --rebase`.
