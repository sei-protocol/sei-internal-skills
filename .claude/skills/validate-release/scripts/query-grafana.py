#!/usr/bin/env python3
"""
Query Grafana data API for each scenario's time windows.
Reads window timestamps from each scenario's own S3 report (correct per-scenario
timing), with a date-based fallback. Produces three windows per scenario:
baseline, chaos, and recovery.

Usage:
  query-grafana.py --suite-id <id> --out <dir>

Output: <out>/<scenario>/{baseline,chaos,recovery}.json
"""
import argparse, json, os, requests, sys, time
from datetime import datetime
from pathlib import Path

BASE_URL = os.environ.get("GRAFANA_BASE_URL", "https://grafana.prod.platform.sei.io")
TOKEN = os.environ.get("GRAFANA_TOKEN", "")
BUCKET = os.environ.get("S3_BUCKET", "harbor-validation-results")
PROFILE = os.environ.get("AWS_PROFILE", "sei")
PREFIX = os.environ.get("S3_REPORT_PREFIX", "nightly")
DS_UID = "prometheus-prod"

SCENARIOS = [
    ("chaos-network-partition",    "chaos-net-part"),
    ("chaos-network-latency",      "chaos-net-lat"),
    ("chaos-packet-loss",          "chaos-pkt-loss"),
    ("chaos-pod-failure",          "chaos-pod-fail"),
    ("chaos-cpu-stress",           "chaos-cpu"),
    ("chaos-memory-stress",        "chaos-mem"),
    ("chaos-disk-io-latency",      "chaos-disk-io"),
    ("chaos-container-kill",       "chaos-ctr-kill"),
    ("chaos-bandwidth-limit",      "chaos-bw-limit"),
    ("chaos-dns-chaos",            "chaos-dns"),
    ("chaos-time-skew",            "chaos-time"),
    ("chaos-byzantine-simulation", "chaos-byzantine"),
    ("chaos-rpc-chaos",            "chaos-rpc"),
]

# Recording-rule-backed queries — agent reads scalars, not raw histograms.
QUERIES = {
    "block_time_p50":    'chaos_suite:block_time_p50:rate2m{{chain_id="{cid}"}}',
    "block_time_p95":    'chaos_suite:block_time_p95:rate2m{{chain_id="{cid}"}}',
    "tps":               'chaos_suite:tps:rate1m{{chain_id="{cid}"}}',
    "mempool_size":      'chaos_suite:mempool_size:max{{chain_id="{cid}"}}',
    # Used by compute-stats for halt detection (rate<0.01 = effectively no new blocks)
    # Backed by tendermint_consensus_latest_block_height
    "block_height_delta": 'chaos_suite:block_height_delta:rate2m{{chain_id="{cid}"}}',
}

EMPTY_WINDOW = {"no_data": True, "min": None, "max": None, "avg": None, "count": 0, "series": []}


def query_range(expr, start, end, step="30s"):
    """Returns a stats dict. Never returns a list — uses EMPTY_WINDOW on no data."""
    url = f"{BASE_URL}/api/datasources/proxy/uid/{DS_UID}/api/v1/query_range"
    try:
        resp = requests.get(url,
            headers={"Authorization": f"Bearer {TOKEN}"},
            params={"query": expr, "start": start, "end": end, "step": step},
            timeout=30)
        resp.raise_for_status()
        data = resp.json()
        if data.get("status") != "success":
            return {**EMPTY_WINDOW, "error": "non-success status"}
        results = data.get("data", {}).get("result", [])
        if not results:
            return {**EMPTY_WINDOW}
        values = results[0].get("values", [])
        floats = [float(v[1]) for v in values if v[1] not in ("NaN", "+Inf", "-Inf")]
        if not floats:
            return {**EMPTY_WINDOW}
        return {
            "no_data": False,
            "series": [[v[0], v[1]] for v in values[:60]],
            "min": min(floats),
            "max": max(floats),
            "avg": sum(floats) / len(floats),
            "count": len(floats),
        }
    except Exception as e:
        return {**EMPTY_WINDOW, "error": str(e)}


def derive_windows_for_scenario(suite_id, scenario_name):
    """
    Reads THIS scenario's own S3 report for accurate per-scenario timestamps.
    Each scenario runs sequentially with different start/end times.
    Falls back to date from SUITE_ID when the report is unavailable.
    """
    import boto3
    try:
        session = boto3.Session(profile_name=PROFILE)
        s3 = session.client("s3")
        key = f"{PREFIX}/{scenario_name}/{suite_id}/report.json"
        obj = s3.get_object(Bucket=BUCKET, Key=key)
        report = json.loads(obj["Body"].read())
        start_ts = report.get("start_time_unix")
        duration = report.get("duration_seconds", 600)
        if start_ts:
            return {
                "baseline": (start_ts - 300,       start_ts),
                "chaos":    (start_ts,              start_ts + duration),
                "recovery": (start_ts + duration,   start_ts + duration + 300),
            }
    except Exception:
        pass
    # Fallback: chaos suite fires at 00:00 UTC Saturday
    date_str = suite_id.split("-", 1)[-1]
    try:
        d = datetime.strptime(date_str, "%Y%m%d").replace(hour=0, minute=0)
        epoch = int(d.timestamp())
        return {
            "baseline": (epoch - 300,   epoch),
            "chaos":    (epoch,          epoch + 600),
            "recovery": (epoch + 600,    epoch + 900),
        }
    except Exception:
        now = int(time.time())
        return {
            "baseline": (now - 3900, now - 3600),
            "chaos":    (now - 3600, now - 3000),
            "recovery": (now - 3000, now - 2700),
        }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--suite-id", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()

    if not TOKEN:
        print("ERROR: GRAFANA_TOKEN not set", file=sys.stderr)
        sys.exit(1)

    out_root = Path(args.out)

    for scenario_name, chain_prefix in SCENARIOS:
        chain_id = f"{chain_prefix}-{args.suite_id}"
        scenario_dir = out_root / scenario_name
        scenario_dir.mkdir(parents=True, exist_ok=True)

        windows = derive_windows_for_scenario(args.suite_id, scenario_name)

        for window_name, (start, end) in windows.items():
            result = {"_window": {"start": start, "end": end, "chain_id": chain_id}}
            for metric_name, query_tmpl in QUERIES.items():
                query = query_tmpl.format(cid=chain_id)
                result[metric_name] = query_range(query, start, end)

            out_file = scenario_dir / f"{window_name}.json"
            out_file.write_text(json.dumps(result, indent=2))
            any_data = any(
                isinstance(v, dict) and v.get("count", 0) > 0
                for k, v in result.items() if k != "_window"
            )
            print(f"  {scenario_name}/{window_name}: {'ok' if any_data else 'no data'}", file=sys.stderr)


if __name__ == "__main__":
    main()
