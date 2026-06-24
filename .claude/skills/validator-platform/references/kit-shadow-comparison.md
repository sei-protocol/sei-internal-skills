# Shadow result-export (comparison-mode) kit

## 1. What this concern is

**Driving a shadow `result-export` task in comparison mode against a node that is ALREADY running the supported shadow features, then reading the verdict** — sending the task to the node's seictl sidecar, then reading divergences from S3 + Prometheus. The node's shadow-readiness (it is configured for the shadow build, e.g. an EVM-first / migrate-EVM mode mismatch against canonical) is a **precondition this kit assumes, not a step it teaches** — getting a node into that state is a separate, situational concern and is out of scope here. The generic "submit a task" mental model gets two things wrong: it reaches for seictl's **typed client** (`SubmitResultExportTask` / `ResultExportTask`) — which silently drops every EVM/migration param — and it treats AppHash divergence as a failure, when on an AppHash-breaking migration shadow it is **expected every block** and the verdict must key on execution results instead. *Cited:* `sources.md` §seictl-shadow (`sidecar/tasks/{result_export,result_compare}.go`, `sidecar/shadow/*`, `sidecar/client/tasks.go`, `sidecar/server/server.go` @ `origin/main`); the shadow research doc (`bdchatham-designs/.../seictl-shadow-result-export-tasks.md`).

## 2. The pattern (how this platform does it)

- **The task + its 9 params.** Type string `result-export` (`engine.TaskResultExport`, `sidecar/engine/types.go:28`). `ResultExportRequest` (`sidecar/tasks/result_export.go:36-72`): `bucket` + `region` are **REQUIRED** (validated in `Handler()`, `:96-103`); `prefix` (S3 key prefix); `rpcEndpoint` (the local CometBFT RPC, **defaults `http://localhost:<PortRPC>`** — leave unset); **`canonicalRpc` is the comparison-mode DISCRIMINATOR** — `Handler()` dispatches `ExportAndCompare` **iff `canonicalRpc != ""`**, else plain `Export` (`:104-107`); `migrationMode` (AppHash divergence expected/tolerated → verdict keys on `LastResultsHash` + per-tx receipts); `shadowEvmRpc` + `canonicalEvmRpc` (both set → enables Layer 2, the logical EVM state diff for touched keys); `traceRpc` (prestate trace → touched keys; **defaults to `canonicalEvmRpc`**, `result_compare.go:79-81`).

- **CRITICAL — the typed client cannot set the EVM/migration params; build the RAW params map.** seictl's typed `ResultExportTask` (`sidecar/client/tasks.go:305-311`) carries only `{Bucket, Prefix, Region, CanonicalRPC}`, and `ToTaskRequest()` (`:324-336`) emits only those four. `migrationMode` / `shadowEvmRpc` / `canonicalEvmRpc` / `traceRpc` are **honored by the sidecar handler but unreachable through the typed path**. To pass them you MUST hand-build a raw `TaskRequest{Type, Params: map[string]any}` and submit it via `SubmitTask` (`client/client.go:96`), or POST the equivalent body to `/v0/tasks` directly. *Cited:* §seictl-shadow (`client/tasks.go:305-336`, `client/client.go:96`); research finding (consequential) — typed-client gap.

  **Worked: one comparison-mode submission (raw map — the operator's real example values):**
  ```go
  // Reachable cross-pod at :8443 (the controller's RBACProxyPort kube-rbac-proxy,
  // §controller noderesource.go:65) under trusted-header authn; 7777 is the in-pod
  // bind behind the proxy (sidecar/server/auth.go). Submit the RAW request — the
  // typed ResultExportTask would drop everything below canonicalRpc.
  id, err := c.SubmitTask(ctx, client.TaskRequest{
      Type: client.TaskTypeResultExport,                 // "result-export"
      Params: &map[string]any{
          "bucket":          "sei-shadow-exports",        // REQUIRED
          "region":          "us-east-1",                 // REQUIRED
          "prefix":          "arctic-1/migrate-evm/",      // own this prefix — don't collide
          "canonicalRpc":    "http://archive-0-rpc:26657", // DISCRIMINATOR → comparison mode (CometBFT RPC)
          "migrationMode":   true,                         // AppHash divergence EXPECTED → verdict on results
          "shadowEvmRpc":    "http://localhost:8545",      // local shadow EVM JSON-RPC
          "canonicalEvmRpc": "http://archive-0-evm:8545",  // canonical EVM JSON-RPC → Layer 2
          "traceRpc":        "http://archive-0-evm:8545",  // prestate trace → touched keys (defaults to canonicalEvmRpc)
      },
  })
  ```
  The equivalent raw HTTP is `POST /v0/tasks` with body `{"type":"result-export","params":{…same map…}}` (`server/server.go:67`,`:38-46` `TaskRequest{Type,Params}`).

- **The LAYER model + watermark.** `sidecar/shadow/types.go:1-12`: **L0** = block-header hashes (AppHash / LastResultsHash / gas) via CometBFT `/block`,`/block_results` on `canonicalRpc`; **L1** = per-tx receipt compare (runs on an L0 divergence, **and always in migration mode** — there it is load-bearing); **L2** = logical EVM state (storage/code/nonce) for **touched keys**, read via `canonicalEvmRpc`/`shadowEvmRpc` go-ethereum clients (enabled only when both are set, `result_compare.go:69-89`). Touched keys come from `traceRpc` `debug_traceBlockByNumber` `prestateTracer{diffMode:true}` (`shadow/keysource.go:12-25`) — a hot-state sampling oracle, **not** a full-keyspace check. **Migration verdict:** in migration mode the real L0 divergence is `!LastResultsHashMatch` (AppHash mismatch alone is informational); an indeterminate L1/L2 (RPC error) **fails closed** → `Match=false` (`shadow/comparator.go:97-117`). **WATERMARK:** `exportState{LastExportedHeight}` in `.sei-sidecar-last-export.json` (`result_export.go:25`,`:69-71`); cold-start bootstraps from the restorer `SnapshotHeightFile` (`:bootstrapExportState`); the loop walks **`last+1 → latest`** (`result_compare.go:99`), pages every **100 blocks** (`comparePageSize`, `result_compare.go:16`). *(Plain non-compare `Export` pages at 1000.)*

- **READING RESULTS.** **S3** (under `{bucket}/{prefix}`): comparison pages `{prefix}{start}-{end}.compare.ndjson.gz` (`result_compare.go:241`); a divergence report `{prefix}divergence-{h}.report.json.gz` — the gzipped structured `DivergenceReport` (raw block + block_results from both chains, `shadow/report.go:11-25`; `result_compare.go:205`), which `shadow/render.go:11 RenderMarkdown` renders to a human investigation report. **Prometheus** (`/v0/metrics`, `shadow/metrics.go`): `seictl_shadow_blocks_compared_total` and `seictl_shadow_divergences_total{divergence_layer}` — `divergence_layer` is `"0"`/`"1"`/`"2"` for the first layer that diverged. *Cited:* §seictl-shadow `shadow/{metrics,report,render}.go`, `tasks/result_compare.go`.

- **Adjacent (one line):** `evm-logical-digest` (`engine/types.go:38`; `tasks/evm_logical_digest.go`) is a distinct **at-tip full-keyspace** logical-digest task that **shells out to the `seidb` binary** — it needs a seidb-capable sidecar image and is not the per-block comparison-mode flow this kit covers.

## 3. Anti-patterns / failure modes

- **Using the typed client and silently dropping the EVM/migration params.** Cue: a comparison driven through `SubmitResultExportTask` / `ResultExportTask` (or a `ToTaskRequest()` derived from it) while expecting Layer 2 or migration behavior. Rewrite: build the **raw `TaskRequest` params map** (`SubmitTask`) or POST `/v0/tasks` directly — the typed struct only carries `{bucket,prefix,region,canonicalRpc}`, so `migrationMode`/`shadowEvmRpc`/`canonicalEvmRpc`/`traceRpc` are dropped without error (Dimension 1 (Manifest correctness & encoding); §seictl-shadow `client/tasks.go:305-336`).
- **Forgetting `migrationMode` on an AppHash-breaking migration.** Cue: a migrate-EVM / mode-mismatch shadow run with `migrationMode` unset. Rewrite: set `migrationMode: true` — otherwise the expected per-block AppHash divergence reads as a real L0 divergence and every block reports false divergence; with it set the verdict keys on `LastResultsHash` + receipts (Dimension 4 (Result verification); §seictl-shadow `comparator.go:97-101`).
- **Pointing the task at a node NOT running the supported shadow features.** Cue: a `result-export` comparison aimed at a node that is not in the supported shadow state. Rewrite: shadow-readiness is a precondition — confirm the target node is already running the supported shadow features before submitting; this kit does not get a node into that state (Dimension 4 (Result verification); §1 — shadow-readiness is an assumed precondition, §seictl-shadow).

## 4. Review cues

- **Dimension 1 (Manifest correctness & encoding):** comparison-mode params are passed via the **raw params map** (`SubmitTask` / `/v0/tasks`), never the typed `ResultExportTask` (which drops the EVM/migration fields); `bucket` + `region` present; `canonicalRpc` set (the discriminator); EVM Layer 2 needs **both** `shadowEvmRpc` and `canonicalEvmRpc`; `traceRpc` defaults to `canonicalEvmRpc`. *Basis:* §seictl-shadow `client/tasks.go:305-336`, `result_export.go:36-107`, `result_compare.go:69-89`.
- **Dimension 4 (Result verification):** `migrationMode` set when AppHash divergence is expected (verdict on `LastResultsHash` + receipts); results read from **S3** (`*.compare.ndjson.gz`, `divergence-{h}.report.json.gz`) + **Prom** (`seictl_shadow_blocks_compared_total`, `seictl_shadow_divergences_total{divergence_layer}`), not `status.outputs`; the target is confirmed shadow-ready. *Basis:* §seictl-shadow `comparator.go:97-117`, `metrics.go`, `result_compare.go:205,241`; profile §6.
- **Dimension 5 (Citation discipline):** task/shadow shapes cited to seictl@origin/main paths; the migrate-EVM / mode-mismatch enum is referenced only as a **precondition concept** (it lives in external `sei-config`/the seid build, not seictl), never restated as a step. *Basis:* `sources.md` §seictl-shadow; research doc out-of-boundary note.

## 5. One-way doors in this concern

- **`result-export` is read-only on the node** — it reads the chain (CometBFT + EVM RPC) and writes to S3; it does **not** mutate node state, so it is **low-risk** and is **NOT** a schema or prod one-way door. Re-running is safe (the watermark advances from `last+1`).
- **Honest load/collision notes (not irreversible, but real):** the task adds sidecar + RPC load (CometBFT `canonicalRpc`, EVM RPCs, and `debug_traceBlockByNumber` on `traceRpc`, which needs the `debug_` namespace on an operator-owned node), and it **writes to the S3 `{prefix}`** — give each run its own prefix so concurrent or repeated runs don't collide pages/reports.
