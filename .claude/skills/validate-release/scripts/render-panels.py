#!/usr/bin/env python3
"""
Render Grafana panel PNGs for each scenario's chaos window.
Produces TPS, block-time, and error-rate panels per scenario.

Usage:
  render-panels.py --suite-id <id> --out <dir>

Output: <out>/<scenario>/{tps,blocktime,errors}.png
"""
import argparse, json, os, requests, sys
from pathlib import Path

BASE_URL = os.environ.get("GRAFANA_BASE_URL", "https://grafana.prod.platform.sei.io")
TOKEN = os.environ.get("GRAFANA_TOKEN", "")
DASHBOARD_UID = os.environ.get("GRAFANA_DASHBOARD_UID", "nightly")

# Panel IDs from grafana-dashboards-nightly.yaml — update if dashboard changes.
# Discover live: GET /api/dashboards/uid/<uid> | python3 -c "import json,sys; [print(p['id'], p['title']) for p in json.load(sys.stdin)['dashboard']['panels']]"
PANELS = {
    "tps":       {"id": None, "title_match": "TPS"},
    "blocktime": {"id": None, "title_match": "Block Time"},
    "errors":    {"id": None, "title_match": "Tx success"},
}


def resolve_panel_ids():
    """Fetch dashboard JSON and resolve panel IDs by title match."""
    url = f"{BASE_URL}/api/dashboards/uid/{DASHBOARD_UID}"
    resp = requests.get(url, headers={"Authorization": f"Bearer {TOKEN}"}, timeout=30)
    resp.raise_for_status()
    panels = resp.json().get("dashboard", {}).get("panels", [])
    for key, cfg in PANELS.items():
        for p in panels:
            if cfg["title_match"].lower() in p.get("title", "").lower():
                cfg["id"] = p["id"]
                break
    missing = [k for k, v in PANELS.items() if v["id"] is None]
    if missing:
        print(f"WARNING: could not resolve panel IDs for: {missing}", file=sys.stderr)


def render(panel_id, chain_id, start_ms, end_ms, width=800, height=300):
    url = (
        f"{BASE_URL}/render/d-solo/{DASHBOARD_UID}"
        f"?orgId=1&panelId={panel_id}"
        f"&from={start_ms}&to={end_ms}"
        f"&var-chain_id={chain_id}"
        f"&var-benchmark_namespaces=nightly"
        f"&width={width}&height={height}&theme=dark"
    )
    resp = requests.get(url, headers={"Authorization": f"Bearer {TOKEN}"}, timeout=60)
    if resp.status_code == 200:
        return resp.content
    print(f"    WARNING: render returned HTTP {resp.status_code}", file=sys.stderr)
    return None


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--suite-id", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--metrics-dir", help="metrics/ dir from query-grafana.py for timestamps")
    args = parser.parse_args()

    if not TOKEN:
        print("ERROR: GRAFANA_TOKEN not set", file=sys.stderr); sys.exit(1)

    resolve_panel_ids()

    out_root = Path(args.out)
    metrics_root = Path(args.metrics_dir) if args.metrics_dir else None

    SCENARIOS = [
        "chaos-network-partition", "chaos-network-latency", "chaos-packet-loss",
        "chaos-pod-failure", "chaos-cpu-stress", "chaos-memory-stress",
        "chaos-disk-io-latency", "chaos-container-kill", "chaos-bandwidth-limit",
        "chaos-dns-chaos", "chaos-time-skew", "chaos-byzantine-simulation",
        "chaos-rpc-chaos",
    ]
    CHAIN_PREFIXES = {
        "chaos-network-partition": "chaos-net-part",
        "chaos-network-latency": "chaos-net-lat",
        "chaos-packet-loss": "chaos-pkt-loss",
        "chaos-pod-failure": "chaos-pod-fail",
        "chaos-cpu-stress": "chaos-cpu",
        "chaos-memory-stress": "chaos-mem",
        "chaos-disk-io-latency": "chaos-disk-io",
        "chaos-container-kill": "chaos-ctr-kill",
        "chaos-bandwidth-limit": "chaos-bw-limit",
        "chaos-dns-chaos": "chaos-dns",
        "chaos-time-skew": "chaos-time",
        "chaos-byzantine-simulation": "chaos-byzantine",
        "chaos-rpc-chaos": "chaos-rpc",
    }

    for scenario in SCENARIOS:
        chain_id = f"{CHAIN_PREFIXES[scenario]}-{args.suite_id}"
        scenario_dir = out_root / scenario
        scenario_dir.mkdir(parents=True, exist_ok=True)

        # Get window times from metrics dir if available
        start_ms, end_ms = None, None
        if metrics_root:
            chaos_file = metrics_root / scenario / "chaos.json"
            if chaos_file.exists():
                w = json.loads(chaos_file.read_text()).get("_window", {})
                start_ms = w.get("start", 0) * 1000
                end_ms = w.get("end", 0) * 1000

        if not start_ms:
            # Wide fallback: last 2h
            import time
            end_ms = int(time.time()) * 1000
            start_ms = end_ms - 7200000

        for panel_name, cfg in PANELS.items():
            if cfg["id"] is None:
                print(f"  {scenario}/{panel_name}: skipped (panel ID unknown)", file=sys.stderr)
                continue
            png = render(cfg["id"], chain_id, start_ms, end_ms)
            out_file = scenario_dir / f"{panel_name}.png"
            if png:
                out_file.write_bytes(png)
                print(f"  {scenario}/{panel_name}: {len(png)} bytes", file=sys.stderr)
            else:
                # Write a placeholder marker
                (scenario_dir / f"{panel_name}.failed").write_text("render failed")


if __name__ == "__main__":
    main()
