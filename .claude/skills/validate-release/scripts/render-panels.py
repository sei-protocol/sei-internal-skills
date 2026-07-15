#!/usr/bin/env python3
"""
Render Grafana panel PNGs per scenario, scoped to the run's per-scenario window
and the chaos chain_id. Re-pointed off the removed prod-scoped recording rules
to the RAW harbor metrics (D4): block interval, TPS degradation, mempool.

The panels are defined by the nightly dashboard (platform-owned); this script
resolves them by title and renders d-solo scoped to `chaos-<token>-<scenario>`
and the run window from query-grafana. If a panel title is absent the PNG is
skipped (a text fallback is embedded downstream) — never a broken image.

Usage:
  render-panels.py --run <token> --metrics-dir <dir> --out <dir>

Output: <out>/<scenario>/{blocktime,tps,mempool}.png
"""
import argparse
import json
import os
import sys
from pathlib import Path

import requests

import chaoslib as cl

DASHBOARD_UID = os.environ.get("GRAFANA_DASHBOARD_UID", "nightly")

# Raw-metric panels (title match against the nightly dashboard). Update the
# title_match if the dashboard renames a panel.
PANELS = {
    "blocktime": {"id": None, "title_match": "Block Time"},  # specific — must NOT match a "Block Height" panel
    "tps":       {"id": None, "title_match": "TPS"},
    "mempool":   {"id": None, "title_match": "Mempool"},
}


def resolve_panel_ids():
    url = f"{cl.BASE_URL}/api/dashboards/uid/{DASHBOARD_UID}"
    resp = requests.get(url, headers={"Authorization": f"Bearer {cl.TOKEN}"}, timeout=30)
    resp.raise_for_status()
    panels = resp.json().get("dashboard", {}).get("panels", [])
    for cfg in PANELS.values():
        for p in panels:
            if cfg["title_match"].lower() in p.get("title", "").lower():
                cfg["id"] = p["id"]
                break
    missing = [k for k, v in PANELS.items() if v["id"] is None]
    if missing:
        print(f"WARNING: could not resolve panel IDs for: {missing}", file=sys.stderr)


def render(panel_id, chain_id, start_ms, end_ms, width=800, height=300):
    url = f"{cl.BASE_URL}/render/d-solo/{DASHBOARD_UID}"
    params = {
        "orgId": 1,
        "panelId": panel_id,
        "from": start_ms,
        "to": end_ms,
        "var-chain_id": chain_id,
        "var-benchmark_namespaces": cl.NIGHTLY_NS,
        "width": width,
        "height": height,
        "theme": "dark",
    }
    resp = requests.get(
        url, headers={"Authorization": f"Bearer {cl.TOKEN}"}, params=params, timeout=60
    )
    if resp.status_code == 200:
        return resp.content
    print(f"    WARNING: render returned HTTP {resp.status_code}", file=sys.stderr)
    return None


def window_ms(metrics_root, scenario, token):
    """Prefer the query-grafana window; fall back to the token-derived envelope."""
    if metrics_root:
        mfile = metrics_root / f"{scenario}.json"
        if mfile.exists():
            w = json.loads(mfile.read_text()).get("window", {})
            if w.get("start") and w.get("end"):
                return int(w["start"] * 1000), int(w["end"] * 1000)
    run_start = cl.token_unixnano(token) / 1e9
    return int(run_start * 1000), int((run_start + 3600) * 1000)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--run", required=True, help="Run token")
    parser.add_argument("--metrics-dir", help="metrics dir from query-grafana.py (for windows)")
    parser.add_argument("--out", required=True)
    args = parser.parse_args()

    if not cl.TOKEN:
        print("ERROR: GRAFANA_TOKEN not set", file=sys.stderr)
        sys.exit(1)
    if cl.token_unixnano(args.run) is None:
        print(f"ERROR: run token '{args.run}' is not valid base36", file=sys.stderr)
        sys.exit(1)

    resolve_panel_ids()

    out_root = Path(args.out)
    metrics_root = Path(args.metrics_dir) if args.metrics_dir else None

    for scenario in cl.SCENARIOS:
        chain_id = cl.chain_id_for(args.run, scenario)
        scenario_dir = out_root / scenario
        scenario_dir.mkdir(parents=True, exist_ok=True)
        start_ms, end_ms = window_ms(metrics_root, scenario, args.run)

        for panel_name, cfg in PANELS.items():
            if cfg["id"] is None:
                print(f"  {scenario}/{panel_name}: skipped (panel ID unknown)", file=sys.stderr)
                continue
            png = render(cfg["id"], chain_id, start_ms, end_ms)
            if png:
                (scenario_dir / f"{panel_name}.png").write_bytes(png)
                print(f"  {scenario}/{panel_name}: {len(png)} bytes", file=sys.stderr)
            else:
                (scenario_dir / f"{panel_name}.failed").write_text("render failed")


if __name__ == "__main__":
    main()
