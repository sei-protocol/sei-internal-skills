# validate-release scripts

Each script handles one logical step. All are standalone — run them directly for debugging.

## Scripts

| Script | Purpose | Key args |
|---|---|---|
| `check-grafana.sh` | Validate GRAFANA_TOKEN against the live endpoint | (reads env) |
| `resolve-suite-id.py` | Find latest SUITE_ID from S3, or validate a provided one | `--suite-id <id>` |
| `collect-reports.py` | Download seiload JSON reports for all 13 scenarios | `--suite-id <id> --out <dir>` |
| `query-grafana.py` | Query Grafana data API for per-scenario time series | `--suite-id <id> --out <dir>` |
| `render-panels.py` | Render TPS/block-time/errors panel PNGs per scenario | `--suite-id <id> --out <dir>` |
| `upload-images.py` | Upload PNGs to S3, return presigned URLs | `--dir <dir> --suite-id <id>` |
| `push-notion.py` | Create Notion page from assembled report data | `--suite-id <id> --state-dir <dir>` |

## Common env vars

All scripts read from environment:
- `AWS_PROFILE` — AWS profile for S3 (default: `sei`)
- `GRAFANA_TOKEN` — service account token
- `GRAFANA_BASE_URL` — defaults to `https://grafana.prod.platform.sei.io`
- `GRAFANA_DASHBOARD_UID` — defaults to `nightly`
- `S3_BUCKET` — defaults to `harbor-validation-results`
- `NOTION_DATABASE_ID` — target Notion database

## Output convention

Scripts write structured YAML to `--out <dir>/`:
- `collect-reports.py` → `<out>/<scenario>/report.json`
- `query-grafana.py` → `<out>/<scenario>/{baseline,chaos,recovery}.json`
- `render-panels.py` → `<out>/<scenario>/{tps,blocktime,errors}.png`
- `upload-images.py` → `<out>/image-urls.yaml` (map of scenario → panel → S3 URL)

Each script appends timestamped entries to `../audit.log` relative to `--out`.
