#!/usr/bin/env python3
"""
Query Grafana data API for each scenario's time windows.
Produces three windows per scenario: baseline, chaos, recovery.

Usage:
  query-grafana.py --suite-id <id> --out <dir>

Output: <out>/<scenario>/{baseline,chaos,recovery}.json — each contains
        {block_time_p50, block_time_p95, tps, tx_success_rate, mempool_size}
        with timestamps, series values, and computed stats.
"""
import argparse, json, os, requests, sys, time
from datetime import datetime, timedelta
from pathlib import Path

BASE_URL = os.environ.get("GRAFANA_BASE_URL", "https://grafana.prod.platform.sei.io")
TOKEN = os.environ.get("GRAFANA_TOKEN", "")
BUCKET = os.environ.get("S3_BUCKET", "harbor-validation-results")
PROFILE = os.environ.get("AWS_PROFILE", "sei")
DS_UID = "prometheus-prod"  # from dashboard JSON spec.datasource.uid

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

# Key Prometheus queries — parameterised by chain_id
QUERIES = {
    "block_time_avg": 'avg(rate(tendermint_consensus_block_interval_seconds_sum{{chain_id="{cid}"}}[2m])/rate(tendermint_consensus_block_interval_seconds_count{{chain_id="{cid}"}}[2m]))',
    "block_time_p95": 'histogram_quantile(0.95, rate(tendermint_consensus_block_interval_seconds_bucket{{chain_id="{cid}"}}[2m]))',
    "tps": 'sum(rate(sei_tx_flow_total{{chain_id="{cid}"}}[1m]))',
    "tx_success_rate": 'sum(rate(sei_tx_flow_total{{chain_id="{cid}",accepted="true"}}[1m])) / sum(rate(sei_tx_flow_total{{chain_id="{cid}"}}[1m]))',
    "mempool_size": 'max(tendermint_mempool_size{{chain_id="{cid}"}})',
}


def query_range(expr, start, end, step="30s"):
    url = f"{BASE_URL}/api/datasources/proxy/uid/{DS_UID}/api/v1/query_range"
    resp = requests.get(url,
        headers={"Authorization": f"Bearer {TOKEN}"},
        params={"query": expr, "start": start, "end": end, "step": step},
        timeout=30)
    resp.raise_for_status()
    data = resp.json()
    if data.get("status") != "success":
        return []
    results = data.get("data", {}).get("result", [])
    if not results:
        return []
    values = results[0].get("values", [])
    if not values:
        return []
    floats = [float(v[1]) for v in values if v[1] != "NaN"]
    if not floats:
        return []
    return {
        "series": values[:50],  # first 50 points for sparkline
        "min": min(floats),
        "max": max(floats),
        "avg": sum(floats) / len(floats),
        "count": len(floats),
    }


def derive_windows(suite_id, chain_id_prefix):
    """
    Derive time windows by inspecting S3 report timestamps.
    Falls back to wall-clock estimation if unavailable.
    """
    import boto3
    try:
        session = boto3.Session(profile_name=PROFILE)
        s3 = session.client("s3")
        obj = s3.get_object(Bucket=BUCKET,
                            Key=f"nightly/chaos-network-partition/{suite_id}/report.json")
        report = json.loads(obj["Body"].read())
        start_ts = report.get("start_time_unix")
        duration = report.get("duration_seconds", 600)
        if start_ts:
            # Baseline: 5min before start; chaos: start + duration; recovery: +5min
            return {
                "baseline": (start_ts - 300, start_ts),
                "chaos":    (start_ts, start_ts + duration),
                "recovery": (start_ts + duration, start_ts + duration + 300),
            }
    except Exception:
        pass
    # Fallback: use the date from SUITE_ID
    date_str = suite_id.split("-", 1)[-1]  # e.g. "20260527" from "abc1234-20260527"
    try:
        d = datetime.strptime(date_str, "%Y%m%d").replace(hour=0)  # suite fires at 01:00 UTC
        epoch = int(d.timestamp())
        return {
            "baseline": (epoch - 300,           epoch),
            "chaos":    (epoch,                  epoch + 600),
            "recovery": (epoch + 600,            epoch + 900),
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
        print("ERROR: GRAFANA_TOKEN not set", file=sys.stderr); sys.exit(1)

    out_root = Path(args.out)
    sha7 = args.suite_id.split("-")[0]

    for scenario_name, chain_prefix in SCENARIOS:
        chain_id = f"{chain_prefix}-{args.suite_id}"
        scenario_dir = out_root / scenario_name
        scenario_dir.mkdir(parents=True, exist_ok=True)

        windows = derive_windows(args.suite_id, chain_prefix)

        for window_name, (start, end) in windows.items():
            result = {}
            for metric_name, query_tmpl in QUERIES.items():
                query = query_tmpl.format(cid=chain_id)
                try:
                    result[metric_name] = query_range(query, start, end)
                except Exception as e:
                    result[metric_name] = {"error": str(e)}

            result["_window"] = {"start": start, "end": end, "chain_id": chain_id}
            out_file = scenario_dir / f"{window_name}.json"
            out_file.write_text(json.dumps(result, indent=2))
            print(f"  {scenario_name}/{window_name}: written", file=sys.stderr)

    print("query-grafana done", file=sys.stderr)


if __name__ == "__main__":
    main()
