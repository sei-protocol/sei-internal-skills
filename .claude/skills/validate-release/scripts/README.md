# validate-release scripts

Each script handles one logical step. All are standalone — run them directly for
debugging. `chaoslib.py` is the shared module (the 10-scenario set, `chaos-<token>-
<scenario>` parsing + base36 ordering, and the Thanos-aware Prometheus query path);
it is imported, not run.

## Scripts

| Script | Purpose | Key args |
|---|---|---|
| `check-grafana.sh` | Validate `GRAFANA_TOKEN` auth (`/api/org`) — not the datasource's presence | (reads env) |
| `resolve-run.py` | Discover chaos runs from Prometheus; resolve one run token | `[--run <token>] [--list] [--out <dir>]` |
| `query-grafana.py` | Raw harbor metrics per scenario, time-scoped to the run window | `--run <token> --out <dir>` |
| `collect-run-log.py` | Join token→harbor Job; extract release image + per-scenario PASS/FAIL | `--run <token> --out <dir>` |
| `compute-stats.py` | Join Job-log verdict (authoritative) + metrics; headline | `--run-log <dir> --metrics-dir <dir> --out <dir>` |
| `render-panels.py` | Block-time / TPS / mempool panel PNGs per scenario | `--run <token> --metrics-dir <dir> --out <dir>` |
| `upload-images.py` | Upload PNGs to S3, write scenario→panel→URL map | `--dir <dir> --suite-id <token> --out <dir>/image-urls.yaml` |
| `push-notion.py` | Assemble the Notion payload from verdicts + images | `--run <token> --state-dir <dir>` |

`upload-images.py` keeps its `--suite-id` flag; pass the run token as its value (it is
only an S3 key namespace).

## Common env vars

- `GRAFANA_TOKEN` — service-account token (Viewer)
- `GRAFANA_BASE_URL` — defaults to `https://grafana.prod.platform.sei.io`
- `PROM_DS_UID` — federated datasource UID; defaults to `prometheus-prod`
- `GRAFANA_DASHBOARD_UID` — defaults to `nightly` (panel renders)
- `RAW_RETENTION` — raw freshness bound; defaults to `15d`
- `HARBOR_CONTEXT` / `NIGHTLY_NAMESPACE` — kubectl target; default `harbor` / `nightly`
- `HARNESS_JOB_PREFIX` — Job name prefix; default `nightly-harness-suite`
- `LOG_RETENTION` — Job-log freshness bound (for messages); default `7d`
- `RUN_WINDOW_SECONDS` / `PRE_BUFFER_SECONDS` / `QUERY_STEP` — metric window shape
- `HALT_FINAL_SAMPLES` — final-N samples the set must still be advancing over; default `4`
- `LOG_BYTE_CAP` — bound on the Job-log read (default 20 MiB); a hit sets truncated
- `JOIN_TOLERANCE_SECONDS` — tolerance on the Job window's trailing edge for token containment; default `120`
- `AWS_PROFILE` — for `upload-images.py` only (default `sei`)
- `NOTION_DATABASE_ID` — target Notion database

## Output convention

Scripts write structured JSON under `--out <dir>/`:
- `resolve-run.py` → `<dir>/run.json` (when `--out` given) + token to stdout
- `query-grafana.py` → `<dir>/<scenario>.json`
- `collect-run-log.py` → `<dir>/verdicts.json`
- `compute-stats.py` → `<dir>/<scenario>.json` + `<dir>/summary.json`
- `render-panels.py` → `<dir>/<scenario>/{blocktime,tps,mempool}.png`
- `upload-images.py` → `<dir>/image-urls.yaml`
- `push-notion.py` → `<state-dir>/notion-payload.json`

## Standalone smoke tests

```bash
export GRAFANA_TOKEN=... NOTION_DATABASE_ID=...
python3 resolve-run.py --list                                   # discover runs
TOKEN=$(python3 resolve-run.py)                                 # latest token
python3 query-grafana.py   --run "$TOKEN" --out /tmp/r/metrics
python3 collect-run-log.py --run "$TOKEN" --out /tmp/r/run-log
python3 compute-stats.py   --run-log /tmp/r/run-log --metrics-dir /tmp/r/metrics --out /tmp/r/verdicts
python3 push-notion.py     --run "$TOKEN" --state-dir /tmp/r
```
