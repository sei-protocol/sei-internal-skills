# Comparative bench

Side-by-side bench of two `seid` images against identical sei-load configuration. Engineer says "compare PR 3399 to main on sei-chain" or "bench latest sei-chain against commit b7b4868" — the agent renders two ephemeral chains (each a SeiNetwork running its own image, each with its own follower SeiNode fleet), two sei-load Jobs (identical profile, duration, RPC fleet), opens one PR. After merge: both networks reach `Ready` in parallel, both follower fleets reach `Running`, both benches run in parallel, both reports land in S3 paired by `<RUN_ID>`. The agent fetches both, extracts canonical metrics, and surfaces a side-by-side table.

## Substrate facts

- **Each `seid` image runs its own chain.** A chain is the binary it executes; you cannot bench two binaries against one chain. Comparative bench therefore spins up two SeiNetworks + two follower SeiNode fleets, side `a` and side `b`.
- **Both benches must use identical sei-load configuration.** Different profiles or durations turn the comparison into noise. The skill enforces parity: one resolved profile, one duration, one sei-load image, applied to both Jobs.
- **Resource budget is ~2x a single bench.** Two ephemeral chains in one namespace = ~2x (4 validators + N rpc followers) seid pods + 2 sei-load Jobs. The cluster handles this fine on Karpenter scaling; no per-namespace resource gate, but worth flagging for engineers used to single-bench footprint.
- **Result fetch runs in-cluster, not from the engineer's laptop.** The engineer's local AWS profile (resolved at preflight gate 3) doesn't have `s3:GetObject` on `harbor-validation-results/*` — only the in-cluster `engineer-service-account` does (via Pod Identity, scoped to `eng-<alias>/*` by session tag). The agent fetches both reports by running a one-shot `kubectl run` with `serviceAccountName: engineer-service-account`, then prints the report contents. See "Fetch + render the comparison" below.
- **`<COMPARE_RUN_ID>` is the join key.** Same string in: branch name, both bench `<RUN_ID>`s, the four manifest dirs, the S3 prefix. The two side reports differ only by the `/a/` vs `/b/` segment.

## Inputs the agent gathers

| Input | Source / default |
|---|---|
| Image A (engineer's PR / commit / branch) | **Required** — engineer specifies; resolution per `references/image-resolution.md`. Never silently default. |
| Image B (baseline) | **Required** — defaults to nothing; engineer supplies (typically `--branch main` for "compare against latest main"). Same resolution flow as A. |
| Compare tag (`<COMPARE_RUN_ID>` component) | Linear ticket → "<imageA-tag>-vs-<imageB-tag>" → explicit `--tag`. On a re-render, parse the existing PR branch (`feat/eng-<alias>-compare-<run-id>`) to recover the original `<COMPARE_RUN_ID>`. |
| Profile | Default `nightly_evm_transfer`. Override `--profile`. Applied to **both** sides. |
| Duration (minutes) | Default 10. Override `--duration`. Applied to **both** sides. |
| Replica count per chain | Default per `genesis-chain` preset (4 validators, immutable once created). RPC fleet size is N standalone `node apply` calls per side (no `--replicas` on a SeiNode). Both sides match. |

## Render-time prerequisites

Before writing any manifest:

1. **Both images resolved + verified in registry** per `references/image-resolution.md`. Halt if either is missing — surface the build-workflow trigger / watch path for the missing side; do not render half a comparison.
2. **No CR name collision in `eng-<alias>`** for either side's planned names — the two SeiNetworks (`<chain-tag>-a`, `<chain-tag>-b`) and their follower SeiNodes (`<chain-tag>-a-rpc-<k>`, `<chain-tag>-b-rpc-<k>`). `kubectl get seinetwork,seinode -n eng-<alias>` must show none of the planned names. Halt if any exist; surface and ask whether to bump the compare-tag or `git rm` the colliding object first.
3. **Workspace task path is free.** `engineers/<alias>/compare-<COMPARE_RUN_ID>/` does not exist on the workspace branch. Halt on collision (per the standard task-path-collision rule).

## `<COMPARE_RUN_ID>` derivation and re-render determinism

`<COMPARE_RUN_ID> = <compare-tag>-<UTC-timestamp>` on first render. Branch: `feat/eng-<alias>-compare-<COMPARE_RUN_ID>`. On re-render against the same engineer + compare-tag, reuse the existing branch's `<COMPARE_RUN_ID>` rather than minting a new one (same shape as single-bench).

```sh
EXISTING=$(gh pr list --repo sei-protocol/harbor-engineering-workspace \
  --state open --json headRefName \
  --jq ".[] | select(.headRefName | startswith(\"feat/eng-${ALIAS}-compare-${COMPARE_TAG}-\")) | .headRefName" \
  | head -1)
if [ -n "${EXISTING}" ]; then
  COMPARE_RUN_ID="${EXISTING#feat/eng-${ALIAS}-compare-}"
else
  COMPARE_RUN_ID="${COMPARE_TAG}-$(date -u +%Y%m%d-%H%M%S)"
fi
```

**Re-run on the same `<COMPARE_RUN_ID>` overwrites prior reports in S3.** If the engineer iterates on a comparative bench (fixes a config, re-merges the PR), the upload sidecar pushes to the same `s3://harbor-validation-results/eng-<alias>/<profile>/<COMPARE_RUN_ID>/{a,b}/report.log` keys, replacing the prior content. Surface this in the plan-echo so engineers know "merging this PR a second time replaces the prior results." If the engineer wants a fresh comparison rather than overwriting, instruct them to pick a new compare-tag (which mints a new `<COMPARE_RUN_ID>`).

## Naming convention

| Resource | Side A | Side B |
|---|---|---|
| SeiNetwork name + chain-id | `<chain-tag>-a` | `<chain-tag>-b` |
| Follower SeiNode names | `<chain-tag>-a-rpc-0 .. -(N-1)` | `<chain-tag>-b-rpc-0 .. -(N-1)` |
| Bench Job name | `seiload-<COMPARE_RUN_ID>-a` | `seiload-<COMPARE_RUN_ID>-b` |
| Bench ConfigMap name | `seiload-profile-<COMPARE_RUN_ID>-a` | `seiload-profile-<COMPARE_RUN_ID>-b` |
| S3 report key | `eng-<alias>/<profile>/<COMPARE_RUN_ID>/a/report.log` | `eng-<alias>/<profile>/<COMPARE_RUN_ID>/b/report.log` |

`<chain-tag>` is the same for both sides — it identifies the comparison, not the side. Length budget: chain-id regex caps at 30 chars (`^[a-z]([a-z0-9-]{0,28}[a-z0-9])?$`) and the longest auto-suffix is `-a-rpc-<k>` (≥8 chars for a single-digit ordinal), so `<chain-tag>` itself must be ≤22 chars. Validate this **before** any rendering call (i.e., at input-resolution time, not when the `node apply --dry-run` rejects on regex mismatch).

Auto-derivation can overflow easily. `harbor-pr-3399-vs-pr-3400` is 25 chars — already over the 22-char budget. Most cross-ticket compares (`harbor-plt-1234-vs-plt-5678`, 27 chars) overflow too. When the auto-derived tag is too long, prompt the engineer for an explicit `--tag` rather than truncating silently — anonymous chain IDs in Grafana / logs / cluster state can't be tied back to the work they served.

## Labels — every resource in the compare carries these

```yaml
labels:
  app.kubernetes.io/managed-by: harbor-dev
  app.kubernetes.io/part-of: seictl-compare
  sei.io/compare-name: <COMPARE_RUN_ID>
  sei.io/compare-side: a   # or b
```

`sei.io/compare-name=<COMPARE_RUN_ID>` is the cross-resource selector. The kubectl `all` category alias does NOT include `SeiNetwork`/`SeiNode` (CRDs participate only when their schema declares the alias) and does NOT include `ConfigMap`. Enumerate the resource types explicitly:

```sh
kubectl get seinetwork,seinode,jobs,configmaps,pods -l sei.io/compare-name=<COMPARE_RUN_ID> -n eng-<alias>
```

Returns the two SeiNetworks + their follower SeiNodes + two Jobs + two ConfigMaps + the seid pods and bench pods sourced from each.

## Workspace layout

```
engineers/<alias>/compare-<COMPARE_RUN_ID>/
├── kustomization.yaml              # aggregates the four sub-dirs
├── chain-a/
│   ├── seinetwork-<chain-tag>-a.yaml
│   ├── seinode-<chain-tag>-a-rpc-<k>.yaml   # one per follower (k = 0..N-1)
│   └── kustomization.yaml
├── chain-b/
│   ├── seinetwork-<chain-tag>-b.yaml
│   ├── seinode-<chain-tag>-b-rpc-<k>.yaml   # one per follower
│   └── kustomization.yaml
├── bench-a/
│   ├── seiload-job.yaml
│   ├── seiload-configmap.yaml
│   └── kustomization.yaml
└── bench-b/
    ├── seiload-job.yaml
    ├── seiload-configmap.yaml
    └── kustomization.yaml
```

The top-level `kustomization.yaml` is:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - chain-a
  - chain-b
  - bench-a
  - bench-b
commonLabels:
  sei.io/compare-name: <COMPARE_RUN_ID>
  app.kubernetes.io/part-of: seictl-compare
```

The agent appends `compare-<COMPARE_RUN_ID>` to `engineers/<alias>/kustomization.yaml`'s top-level `resources:` list. Without that, the per-engineer Flux Kustomization doesn't see the new task dir.

## Per-side manifests

### Chain CRs (`chain-a/` and `chain-b/`)

Render with `seictl network apply --dry-run` (the SeiNetwork) + a loop of `seictl node apply --dry-run` (the followers), same recipe as single-chain (`references/ephemeral-chain-flow.md`):

```sh
# Side A genesis network
seictl network apply <chain-tag>-a \
  --preset genesis-chain --chain-id <chain-tag>-a \
  --image <IMAGE_A_REF> -n eng-<alias> --dry-run \
  | yq -P > chain-a/seinetwork-<chain-tag>-a.yaml

# Side A RPC followers — loop k = 0..N-1; --network wires each to side A's network
for k in $(seq 0 $((N-1))); do
  seictl node apply <chain-tag>-a-rpc-${k} \
    --preset rpc --chain-id <chain-tag>-a --network <chain-tag>-a \
    --image <IMAGE_A_REF> -n eng-<alias> --dry-run \
    | yq -P > chain-a/seinode-<chain-tag>-a-rpc-${k}.yaml
done

# Side B (mirror with image B, -b suffix, and --network <chain-tag>-b)
```

The compare-labels (`sei.io/compare-name`, `sei.io/compare-side`) come from the top-level `kustomization.yaml`'s `commonLabels` and `bench-{a,b}/kustomization.yaml`'s side label — Kustomize propagates `commonLabels` to all resources sourced from listed sub-dirs, so each CR inherits both fields without per-resource edits. The `--network` peer-rail labels (`sei.io/seinetwork`, `sei.io/role=node`) are stamped by `node apply` and are independent of the compare-labels.

`chain-a/kustomization.yaml`:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - seinetwork-<chain-tag>-a.yaml
  - seinode-<chain-tag>-a-rpc-0.yaml
  # ... one entry per follower (seinode-<chain-tag>-a-rpc-<k>.yaml)
commonLabels:
  sei.io/compare-side: a
```

(Same shape for `chain-b/` with `-b` and `compare-side: b`. Same shape for `bench-a/` and `bench-b/` over the seiload Job + ConfigMap.)

### Bench Job + ConfigMap (`bench-a/` and `bench-b/`)

Reuse the templates in `references/sei-load-bench.md`. Three changes per side:

- `<RUN_ID>` becomes `<COMPARE_RUN_ID>-a` (or `-b`)
- `S3_KEY` becomes `eng-<ALIAS>/<PROFILE_NAME>/<COMPARE_RUN_ID>/a/report.log` (or `/b/`)
- Profile substitution targets the side's follower fleet: side A's bench reads the fleet for `sei.io/seinetwork=<chain-tag>-a,sei.io/role=node`; side B reads `<chain-tag>-b`'s.

**Plus an initContainer that gates seiload on RPC Service availability.** Both sides' chains and benches land in the cluster simultaneously when the PR merges; Flux applies them in arbitrary order, and the bench Pod can start before the followers' per-node Services exist. With `restartPolicy: Never` + `backoffLimit: 0` (inherited from single-bench), seiload's first DNS failure terminates the Job before the chain ever reaches Ready — no CrashLoopBackoff to ride out. Add this initContainer to **both** bench Jobs:

```yaml
initContainers:
  - name: wait-for-rpc
    image: busybox:1.37
    command: ["/bin/sh", "-c"]
    args:
      - |
        # Block until the side's first follower's headless RPC Service is
        # resolvable. Each follower SeiNode gets its own per-node headless
        # Service; it resolves only after that SeiNode reaches Running. We
        # probe follower 0 (the first one provisioned) on port 8545.
        TARGET="<chain-tag>-<a-or-b>-rpc-0.eng-<alias>.svc"
        DEADLINE=$((SECONDS + 600))   # 10m budget for the follower to reach Running
        until nslookup "${TARGET}" >/dev/null 2>&1; do
          if [ "${SECONDS}" -ge "${DEADLINE}" ]; then
            echo "rpc Service ${TARGET} did not resolve within 10m; failing"
            exit 1
          fi
          sleep 5
        done
        echo "rpc Service ${TARGET} resolves; proceeding"
    resources:
      requests: { cpu: "10m", memory: "16Mi" }
      limits:   { cpu: "100m", memory: "32Mi" }
```

Substitute `<a-or-b>` with the side's letter at render time. The 10-minute deadline matches the follower-watch timeout — if the follower doesn't reach Running within that window, the bench Job fails fast with a clear DNS message rather than waiting on `activeDeadlineSeconds`.

Profile substitution per side (same `jq --argjson` recipe as single-bench):

```sh
# Side A
RPC_ENDPOINTS_A=$(seictl node list -n eng-<alias> -l sei.io/seinetwork=<chain-tag>-a,sei.io/role=node -o json \
  | jq -c '[.items[].status.endpoint.evmJsonRpc | select(.)]')

PROFILE_RAW=$(gh api repos/sei-protocol/platform/contents/clusters/harbor/nightly/load/profiles/<profile>.json \
  --jq .content | base64 -d)

PROFILE_A=$(echo "${PROFILE_RAW}" | jq \
  --arg cid "<chain-tag>-a" \
  --argjson eps "${RPC_ENDPOINTS_A}" \
  '.seiChainId = $cid | .endpoints = $eps')

# Side B mirrors with -b
```

Both sides use the same `<profile>.json` source — the substitution differs only in chain-id and endpoints. Identical `--duration`, `--track-receipts`, and post-summary flush settings.

The agent enforces parity with a **positive must-match field list** rather than a byte-equal check (byte-equal is fragile against any future profile field that's allowed to vary, e.g., a `_meta.generatedAt` block). Required-equal fields:

```
.workload, .targetTPS, .duration, .accountCount, .txTypes, .gasPrice,
.txsPerBlock, .chainType, .precompiles, .blockTimeMs
```

Plus: `--duration` flag in the Job spec, `--track-receipts` flag, `--post-summary-flush-delay`, container `resources` block, follower fleet size (N rpc SeiNodes) on each side.

Field list lives in a single source-of-truth array in the skill's render path; updates require a deliberate edit rather than a pattern match. Allowed-to-differ: `.seiChainId`, `.endpoints`. Anything else differing → halt and surface the offending field.

## Why one PR

- **Atomic delivery.** Merging starts both chains and both benches at once. Splitting into multiple PRs adds an ordering hazard (one chain Ready, the other still Pending → side A's bench starts before side B's, the comparison clock is staggered).
- **Single audit-trail entry.** Reviewers see "this is a comparison of A vs B" once; the diff lays out both sides for direct comparison.
- **Single teardown.** `git rm -r engineers/<alias>/compare-<COMPARE_RUN_ID>/` removes everything; Flux prunes both chains, both benches, all child resources.

## PR target + path

```
sei-protocol/harbor-engineering-workspace
└── engineers/<alias>/compare-<COMPARE_RUN_ID>/
    ├── kustomization.yaml
    ├── chain-a/  (seinetwork-*-a.yaml + seinode-*-a-rpc-<k>.yaml + kustomization.yaml)
    ├── chain-b/  (seinetwork-*-b.yaml + seinode-*-b-rpc-<k>.yaml + kustomization.yaml)
    ├── bench-a/  (seiload-job.yaml + seiload-configmap.yaml + kustomization.yaml)
    └── bench-b/  (seiload-job.yaml + seiload-configmap.yaml + kustomization.yaml)
```

Branch: `feat/eng-<alias>-compare-<COMPARE_RUN_ID>`. PR title: `feat(eng/<alias>): compare <imageA-tag> vs <imageB-tag> (<COMPARE_RUN_ID>)`. Body lists both image digests + source PR/commit, profile, duration, expected S3 paths.

## Post-merge: parallel watch + fetch

After merge, Flux reconciles both chains in parallel. The agent watches in this order:

1. **Genesis networks (parallel).** Two `seictl network watch` invocations against `<chain-tag>-a` and `<chain-tag>-b`, both `--until=Ready --timeout=15m`. Run concurrently. If either reaches a terminal Failed phase, halt the comparison — surface the failed side's `.status.plan.failedTaskDetail.error` and route to the half-comparison teardown (see Halt conditions). The orphan follower SeiNodes on the failed side keep reconciling on their own; the `git rm` cleanup prunes them.
2. **Follower fleets (parallel).** Per side, loop `seictl node watch <chain-tag>-<a|b>-rpc-<k> --until=Running --timeout=15m` over each follower (a SeiNode has no `Ready` — `--until=Ready` errors `Invalid`). Each follower's endpoint publishes only after it reaches Running; the bench Jobs' `wait-for-rpc` initContainer holds the bench Pod until its side's follower headless Services resolve (10m budget per side, matching the follower-watch timeout).
3. **Bench Jobs (parallel).** Poll both `seiload-<COMPARE_RUN_ID>-{a,b}` for `Complete=True` or `Failed=True`. Wait for both even if one Failed early — the failed side's report is partial but informative for the comparison.
4. **Fetch + render.** Once both Jobs are terminal, fetch both reports via in-cluster `kubectl run` (under `engineer-service-account`'s Pod Identity), produce the side-by-side table.

```sh
# Parallel watch on the genesis networks. Capture both PIDs; wait
# individually so a failure on either side surfaces its exit code (plain
# `wait` swallows it). A SeiNetwork's terminal phase is Ready.
seictl network watch <chain-tag>-a --until=Ready --timeout=15m -n eng-<alias> &
PID_A=$!
seictl network watch <chain-tag>-b --until=Ready --timeout=15m -n eng-<alias> &
PID_B=$!
A_RC=0; B_RC=0
wait ${PID_A} || A_RC=$?
wait ${PID_B} || B_RC=$?
if [ "${A_RC}" -ne 0 ] || [ "${B_RC}" -ne 0 ]; then
  echo "genesis watch failed: A=${A_RC} B=${B_RC}"
  exit 1
fi

# Follower fleets — loop each side's N followers to Running (no Ready on a
# SeiNode; --until=Ready errors Invalid). Watch all followers concurrently.
RC=0
for side in a b; do
  for k in $(seq 0 $((N-1))); do
    seictl node watch <chain-tag>-${side}-rpc-${k} --until=Running --timeout=15m -n eng-<alias> &
  done
done
for pid in $(jobs -p); do wait ${pid} || RC=$?; done
[ "${RC}" -ne 0 ] && { echo "follower watch failed"; exit 1; }

# Bench Job poll (single loop; both jobs share a shell). kubectl jsonpath
# filter expressions don't support `||`, so iterate over .status.conditions
# and grep on the host side.
DEADLINE=$((SECONDS + <DURATION_MINUTES> * 60 + 660))
A_DONE="" ; B_DONE=""
while [ -z "${A_DONE}" ] || [ -z "${B_DONE}" ]; do
  [ "${SECONDS}" -ge "${DEADLINE}" ] && break
  A_DONE=$(kubectl get job -n eng-<alias> seiload-<COMPARE_RUN_ID>-a \
    -o jsonpath='{range .status.conditions[*]}{.type}={.status}{"\n"}{end}' \
    | grep -E '^(Complete|Failed)=True$' | head -1)
  B_DONE=$(kubectl get job -n eng-<alias> seiload-<COMPARE_RUN_ID>-b \
    -o jsonpath='{range .status.conditions[*]}{.type}={.status}{"\n"}{end}' \
    | grep -E '^(Complete|Failed)=True$' | head -1)
  sleep 10
done
```

**Session-resume note.** The `SECONDS` deadline is measured from the agent's invocation. On a session resumed after both chains are already up, recompute the budget from each Job's `.status.startTime` rather than from `SECONDS=0` — otherwise the loop deadlines pre-emptively before the bench finishes.

## Fetch + render the comparison

The upload sidecar in each bench Job pushes its report to S3 on Pod exit (success, failure, or SIGKILL — see `references/sei-load-bench.md`). After both Jobs terminate, the agent pulls both reports back **in-cluster** and produces a side-by-side metrics table.

The fetch runs in-cluster, not from the engineer's laptop, because:

- Pod Identity scopes `s3:GetObject` per namespace via session tag (`aws:PrincipalTag/kubernetes-namespace`); the tag is only injected for in-cluster pods authenticating via `engineer-service-account`.
- The engineer's local AWS profile (their SSO IAM Identity Center principal) carries no `s3:Get*` on `harbor-validation-results/*`. A local `aws s3 cp` returns `AccessDenied`.
- Running the fetch as `engineer-service-account` keeps the per-engineer IAM boundary intact: no engineer can read another engineer's results, even via the agent.

```sh
# One-shot Pod under engineer-service-account that pulls both reports and
# prints them in a single multipart payload the agent parses on stdout.
kubectl run "fetch-${COMPARE_RUN_ID}" \
  -n eng-<alias> \
  --image=amazon/aws-cli:2.17.43 \
  --restart=Never --rm -i --quiet \
  --overrides='{"spec":{"serviceAccountName":"engineer-service-account"}}' \
  -- /bin/sh -c '
    set -e
    A_KEY="eng-<alias>/<profile>/<COMPARE_RUN_ID>/a/report.log"
    B_KEY="eng-<alias>/<profile>/<COMPARE_RUN_ID>/b/report.log"
    echo "===== SIDE-A ====="
    aws s3 cp "s3://harbor-validation-results/${A_KEY}" - || echo "<a fetch failed>"
    echo "===== SIDE-B ====="
    aws s3 cp "s3://harbor-validation-results/${B_KEY}" - || echo "<b fetch failed>"
    echo "===== END ====="
  '
```

The agent splits stdout on the `===== SIDE-A =====` / `===== SIDE-B =====` markers to recover each report. `--rm` cleans up the Pod after exit; `--quiet` suppresses kubectl's `pod ... deleted` line so the parser sees only the report content.

### Metric extraction

Sei-load emits a **result summary block** at the end of its stdout — TPS, latencies, transaction counts, error breakdown, all in a single coherent block. The agent doesn't have to parse the whole log or invent extraction heuristics; it locates the summary block per side and presents both directly. The summary's canonical fields cover everything the comparison needs:

- **TPS achieved** vs target
- **Latency P50 / P90 / P99** — submission-to-receipt
- **Tx submitted / accepted / dropped** — counts and derived success rate
- **Error breakdown** — top error reasons + counts

Strategy:

1. **Locate the summary block.** It's the last contiguous block of structured output before EOF. The exact delimiter sei-load uses varies by version; if the agent can't find a recognized header, fall back to the last 50 lines (where the summary lives by convention).
2. **Present both summary blocks verbatim** in the agent's output — engineers trust the source over a synthesized table. This is the always-on baseline.
3. **Compute deltas for canonical fields** when both blocks parse cleanly. The delta table sits *above* the raw blocks as the primary at-a-glance signal; the blocks below are the audit trail.

Never fabricate a missing field — if the summary block doesn't include something (older sei-load versions, parser drift), drop the row from the delta table rather than guessing.

### Output shape

```
Comparison: <imageA-tag> vs <imageB-tag>
Profile: <profile>   Duration: <DURATION> min   Run: <COMPARE_RUN_ID>

                       <imageA-tag>           <imageB-tag>          Δ
                       (<digest-prefix>)      (<digest-prefix>)
TPS achieved           1,234                  1,189                 +3.8%   better
P50 latency            42 ms                  39 ms                 +7.7%   worse
P99 latency            180 ms                 175 ms                +2.9%   worse
Success rate           99.94 %                99.97 %               -0.03pp worse
Tx accepted            740,400                713,400               +27,000 better
Tx dropped             445                    230                   +215    worse
Top error              "<top-error-A>" (445)  "<top-error-B>" (230) —

Reports:
  A: s3://harbor-validation-results/eng-<alias>/<profile>/<COMPARE_RUN_ID>/a/report.log
  B: s3://harbor-validation-results/eng-<alias>/<profile>/<COMPARE_RUN_ID>/b/report.log

------------------------------------------------------------------------
Side A — sei-load result summary (verbatim):
<...the block sei-load emitted on side A...>
------------------------------------------------------------------------
Side B — sei-load result summary (verbatim):
<...the block sei-load emitted on side B...>
```

The `Δ` column carries the numeric delta plus a value judgment. Mechanical rule: **good-when-higher** metrics (TPS, success rate, tx accepted) flag positive Δ as `better`; **good-when-lower** metrics (latency, tx dropped, error counts) flag negative Δ as `better`. Field membership is fixed in the skill — never inferred per run — so the verdict is consistent across comparisons.

The verbatim summary blocks below the delta table are the audit trail: any field the agent dropped from the table (parser miss, sei-load version drift) is still visible there.

When the summary block can't be located on either side (sei-load failed before emitting it, or the agent doesn't recognize the format), fall back to:

```
Comparison: <imageA-tag> vs <imageB-tag>   (summary block not found)
Profile: <profile>   Duration: <DURATION> min   Run: <COMPARE_RUN_ID>

A — last 50 lines of report:
<verbatim tail>

B — last 50 lines of report:
<verbatim tail>

Reports:
  A: s3://...   B: s3://...
```

## Procedure

1. **Pre-flight** — five gates from `preflight.md`. Halt on first failure.
2. **Resolve both images** — image A (engineer's input) and image B (baseline; engineer-supplied, no silent default). Per `references/image-resolution.md`.
3. **Resolve compare-tag** — Linear ticket → "<imageA-tag>-vs-<imageB-tag>" → explicit `--tag`. Validate `<chain-tag>` length budget (≤24 chars before the `-{a,b}-rpc` suffixes).
4. **Resolve `<COMPARE_RUN_ID>`** — check for existing branch `feat/eng-<alias>-compare-<compare-tag>-*`; reuse on match, else mint `<compare-tag>-<UTC-timestamp>`.
5. **Resolve profile + duration** — defaults `nightly_evm_transfer` / 10 min. Both sides use the same values.
6. **Verify no CR name collisions** — `kubectl get seinetwork,seinode -n eng-<alias>` for the two planned SeiNetworks + their planned follower SeiNodes. Halt on any match.
7. **Plan echo & confirm** — both image digests + source refs, both chain-ids, profile, duration, `<COMPARE_RUN_ID>`, target workspace path, both expected S3 keys, total estimated runtime (`<DURATION> + ~6 min` for chain spinup + upload). Wait for confirmation.
8. **Render** — write the four sub-dirs and the aggregator kustomization.yaml. Append `compare-<COMPARE_RUN_ID>` to `engineers/<alias>/kustomization.yaml` `resources:` if not already present. Chain rendering uses `seictl network apply --dry-run | yq -P` for the SeiNetwork plus a `seictl node apply --dry-run | yq -P` loop for the N followers per side; bench rendering uses the templates from `references/sei-load-bench.md` per side.
9. **Verify config parity** — read both bench ConfigMaps back, diff the substituted JSONs; abort the render if they differ on anything except `seiChainId` and `endpoints`. The whole point of the comparison is identical workload — silent drift breaks the result.
10. **Commit + push** — branch `feat/eng-<alias>-compare-<COMPARE_RUN_ID>`. Message: `feat(eng/<alias>): compare <imageA-tag> vs <imageB-tag> (<COMPARE_RUN_ID>)`.
11. **Open the PR** — surface the URL and halt: "Merge to start. Both chains spin up in parallel (~5 min), both benches run for `<DURATION>` minutes, then I'll fetch the reports and surface the comparison."
12. **Watch — networks parallel** — `seictl network watch <chain-tag>-a` and `<chain-tag>-b` concurrently, `--until=Ready --timeout=15m`. Halt the whole comparison if either reaches Failed.
13. **Watch — follower fleets parallel** — loop `seictl node watch <chain-tag>-<a|b>-rpc-<k> --until=Running` over every follower on both sides (no `Ready` on a SeiNode).
14. **Poll bench Jobs to terminal** — both `seiload-<COMPARE_RUN_ID>-a` and `-b` to `Complete` or `Failed`. Deadline `<DURATION> * 60 + 660` seconds.
15. **Fetch + render** — `aws s3 cp` both reports; extract metrics; render the side-by-side table. On any extraction gap, fall back to the raw-tail format with both S3 paths surfaced.
16. **Teardown guidance** — `git rm -r engineers/<alias>/compare-<COMPARE_RUN_ID>/` and remove the entry from `engineers/<alias>/kustomization.yaml` `resources:`. Flux prunes both SeiNetworks, all follower SeiNodes, and both Jobs; child pods/PVCs cascade.

## Halt conditions

- **Either image fails to resolve.** Surface the missing-side build path; do not render half a comparison. Halt.
- **`<chain-tag>` exceeds the 22-char budget** when the `-{a,b}-rpc-<k>` suffix is added. Surface the overflow and ask the engineer to pick a shorter tag.
- **CR name collision on either side.** Halt before render; surface the existing object's age + labels.
- **One network reaches `Ready` while the other reaches `Failed`.** The comparison is invalid. Surface the failed side's `.status.plan.failedTaskDetail.error`. The half-teardown is two coordinated edits, **both required** — Flux refuses to apply a kustomization with a missing resource:
  - `git rm -r engineers/<alias>/compare-<COMPARE_RUN_ID>/chain-<a-or-b>/`
  - Edit `engineers/<alias>/compare-<COMPARE_RUN_ID>/kustomization.yaml` to remove the matching `- chain-<a-or-b>` line from `resources:`
  - Commit + push + merge; Flux prunes the SeiNetwork and all its follower SeiNodes on the failed side. The orphan followers were reconciling on their own until pruned.
  - **Note:** `bench-<a-or-b>` references chain-side-specific RPC URLs (substituted at render-time), so removing only `bench-<a-or-b>` while keeping `chain-<a-or-b>` doesn't make the bench retryable against a fresh chain — re-deriving the URLs requires a fresh render. Teardown the chain *and* the bench together when the chain failed.
- **One bench `Complete` and the other `Failed`.** Still fetch both reports — the failed side's report is partial but informative. Surface the `Failed` reason from the Job condition (`activeDeadlineSeconds` / `backoffLimit` exhaustion) alongside the comparison table; flag that the comparison is degraded.
- **Failed + still-running.** If side A reaches `Failed=True` while side B is still running, keep polling B to its terminal state — partial-pair results inform the engineer about whether the failure was image-specific or load-pattern-specific. Don't kill B early.
- **In-cluster fetch returns `AccessDenied`.** Pod Identity session tag mismatch or namespace-prefix mismatch on the bucket key — the `engineer-service-account` policy scopes `s3:GetObject` to `harbor-validation-results/${aws:PrincipalTag/kubernetes-namespace}/*` via the Pod Identity session tag, so the resolved namespace must match the bucket prefix. Inspect `aws sts get-caller-identity` from inside the Pod and verify the bucket key starts with `eng-<alias>/`.
- **In-cluster fetch returns `NoSuchKey`.** The upload sidecar didn't run on the failing side. Surface `kubectl logs -n eng-<alias> -l sei.io/compare-name=<COMPARE_RUN_ID>,sei.io/compare-side=<a|b> -c upload-results` to diagnose. Common cause: side terminated via `activeDeadlineSeconds` before the sidecar reached its `aws s3 cp` step.
- **Bench config parity check fails** — the two substituted profile JSONs differ on a field other than `seiChainId` / `endpoints`. Halt before push; the rendered manifests would produce a non-comparable result. Surface the diff and ask the engineer to retry (usually a transient issue with side B's followers not yet publishing `.status.endpoint` when side A was substituted).
- **PR push rejected** — same handling as single-bench; `git pull --rebase` and let the engineer resolve.
