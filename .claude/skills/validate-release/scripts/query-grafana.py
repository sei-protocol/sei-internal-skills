#!/usr/bin/env python3
"""
Query RAW harbor metrics (federated prometheus-prod datasource) for each
scenario chain of a run, TIME-SCOPED to the run window and forced to raw
resolution (max_source_resolution=0). Emits one per-scenario stats dict that
classifies liveness for the narrative (D2).

Key correctness points (D2 / C3 / C1):
  - Series are ephemeral -> range/instant queries at the run window, NEVER instant-now.
  - Halt = validator-SET advancement (max validator height at window-end > window-start,
    advancing in the final N samples), NOT rate()==0 (height is a gauge; a restarted
    validator returns as a NEW pod/instance series, which a per-fixed-series delta would
    misread). A single node restarting is expected, not a halt.
  - TPS/mempool are ~0 by design (the chaos suite runs no load generator; seiload is
    benchmark-only) — carried for transparency, NOT chaos release signals. The chaos
    release signals are halt + block-interval; throughput is the deferred phase-2 report.
  - Distinguishes NO DATA (absent series) from a measured 0, and marks a cell PARTIAL on
    a Thanos partial-response / query error.

Usage:
  query-grafana.py --run <token> --out <dir>

Output: <out>/<scenario>.json  (one stats dict per scenario)
"""
import argparse
import json
import os
import sys
from collections import defaultdict
from pathlib import Path

import chaoslib as cl

RUN_WINDOW_SECONDS = int(os.environ.get("RUN_WINDOW_SECONDS", "3600"))  # generous run envelope
PRE_BUFFER_SECONDS = int(os.environ.get("PRE_BUFFER_SECONDS", "120"))
STEP = os.environ.get("QUERY_STEP", "15s")
HALT_FINAL_SAMPLES = int(os.environ.get("HALT_FINAL_SAMPLES", "4"))


def height_series_by_validator(parsed):
    """Group validator height series by instance_name (a restart = a new series)."""
    groups = defaultdict(list)
    for series in parsed.get("result", []):
        metric = series.get("metric", {})
        key = metric.get("instance_name") or metric.get("pod") or metric.get("instance")
        vals = cl.series_numeric_values(series)
        if vals:
            groups[key].append(vals)
    return groups


def analyze_halt(chain_id, start, end):
    """Restart-aware set-level liveness on the validator set (component='validators')."""
    expr = f'{cl.M_HEIGHT}{{chain_id="{chain_id}", component="validators"}}'
    parsed = cl.prom_range(expr, start, end, step=STEP)
    out = {"verdict": None, "partial": parsed["partial"], "error": parsed["error"]}
    if not parsed["ok"] or not parsed["result"]:
        out["verdict"] = "NO DATA"
        return out

    # Set-level aggregate: max validator height at each timestamp.
    agg = {}
    for series in parsed["result"]:
        for ts, val in cl.series_numeric_values(series):
            agg[ts] = max(agg.get(ts, val), val)
    if not agg:
        out["verdict"] = "NO DATA"
        return out
    ordered_ts = sorted(agg)
    start_max = agg[ordered_ts[0]]
    end_max = agg[ordered_ts[-1]]
    advanced = end_max > start_max

    # Guard advance-early-then-stop: the set must still be advancing in the final N samples.
    if len(ordered_ts) > HALT_FINAL_SAMPLES:
        final_advanced = agg[ordered_ts[-1]] > agg[ordered_ts[-1 - HALT_FINAL_SAMPLES]]
    else:
        final_advanced = advanced

    # Best-effort quorum: distinct validators (by instance_name) that advanced over their life.
    groups = height_series_by_validator(parsed)
    quorum_advanced = 0
    for _key, series_list in groups.items():
        first = min(v[0][1] for v in series_list if v)
        last = max(v[-1][1] for v in series_list if v)
        if last > first:
            quorum_advanced += 1

    # Primary halt signal = set-max advanced AND advancing in the final N samples
    # (sound and scrape-count-independent). The D2 quorum only TIGHTENS it when the
    # full validator set was scraped: a partial scrape (fewer than 4 validators seen —
    # a transient Prometheus miss, not a halt) must NOT false-halt, so quorum is
    # required only when validators_seen == 4; below that we trust set-max + final.
    quorum_ok = quorum_advanced >= cl.QUORUM_OF_FOUR or len(groups) < 4
    out.update({
        "verdict": (
            "ADVANCING"
            if (advanced and final_advanced and quorum_ok)
            else "HALTED"
        ),
        "validator_start_max": start_max,
        "validator_end_max": end_max,
        "blocks_produced": end_max - start_max,
        "set_advanced": advanced,
        "final_window_advanced": final_advanced,
        "quorum_advanced": quorum_advanced,
        "quorum_required": cl.QUORUM_OF_FOUR,
        "validators_seen": len(groups),
        "window_start_ts": ordered_ts[0],
        "window_end_ts": ordered_ts[-1],
    })
    return out


def analyze_block_time(chain_id, start, end, halt):
    """Bucket-bounded p95 block interval, with a height-derived mean fallback."""
    dur = int(end - start)
    expr = (
        f"histogram_quantile(0.95, sum by (le)(rate("
        f'{cl.M_BLOCK_INTERVAL_BUCKET}{{chain_id="{chain_id}"}}[{dur}s])))'
    )
    parsed = cl.prom_instant(expr, at=end)
    p95 = cl.scalar_from_instant(parsed) if parsed["ok"] else None

    mean_derived = None
    blocks = halt.get("blocks_produced")
    ws, we = halt.get("window_start_ts"), halt.get("window_end_ts")
    if blocks and blocks > 0 and ws and we and we > ws:
        mean_derived = round((we - ws) / blocks, 3)

    return {
        "p95_seconds": round(p95, 3) if p95 is not None else None,
        "mean_interval_seconds": mean_derived,
        "source": "histogram_p95" if p95 is not None else "height-derived-mean",
        "partial": parsed["partial"],
    }


def analyze_tps(chain_id, start, end):
    """Committed-tx rate over the lifetime. The chaos suite applies NO load generator
    (seiload is benchmark-only), so committed TPS is ~0 by design — NOT a throughput or
    degradation signal for chaos. The chaos release signals are halt + block-interval;
    throughput belongs to the deferred phase-2 benchmark report. Carried for transparency only."""
    expr = f'sum(rate({cl.M_TPS}{{chain_id="{chain_id}"}}[2m]))'
    parsed = cl.prom_range(expr, start, end, step=STEP)
    if not parsed["ok"] or not parsed["result"]:
        return {"no_data": True, "partial": parsed["partial"]}
    vals = [v for _ts, v in cl.series_numeric_values(parsed["result"][0])]
    if not vals:
        return {"no_data": True, "partial": parsed["partial"]}
    return {
        "no_data": False,
        "first": round(vals[0], 3),
        "last": round(vals[-1], 3),
        "min": round(min(vals), 3),
        "max": round(max(vals), 3),
        "avg": round(sum(vals) / len(vals), 3),
        "samples": len(vals),
        "note": "~0 by design — chaos runs no load generator; transparency-only, NOT a release signal",
        "partial": parsed["partial"],
    }


def analyze_mempool(chain_id, start, end):
    dur = int(end - start)
    expr = f'max_over_time({cl.M_MEMPOOL}{{chain_id="{chain_id}"}}[{dur}s])'
    parsed = cl.prom_instant(expr, at=end)
    peak = cl.scalar_from_instant(parsed) if parsed["ok"] else None
    return {
        "peak": peak,
        "no_data": peak is None,
        "partial": parsed["partial"],
    }


def scenario_stats(token, scenario):
    chain_id = cl.chain_id_for(token, scenario)
    run_start = cl.token_unixnano(token) / 1e9
    start = run_start - PRE_BUFFER_SECONDS
    end = run_start + RUN_WINDOW_SECONDS

    halt = analyze_halt(chain_id, start, end)
    block_time = analyze_block_time(chain_id, start, end, halt)
    tps = analyze_tps(chain_id, start, end)
    mempool = analyze_mempool(chain_id, start, end)

    # NO DATA (absent series) vs measured — never a green 0.
    no_data = (
        halt["verdict"] == "NO DATA"
        and tps.get("no_data", True)
        and mempool.get("no_data", True)
    )
    any_partial = any(x.get("partial") for x in (halt, block_time, tps, mempool))
    if any_partial:
        provenance = "PARTIAL"
    elif no_data:
        provenance = "NO DATA"
    else:
        provenance = "OK"

    return {
        "chain_id": chain_id,
        "window": {"start": start, "end": end},
        "provenance": provenance,
        "no_data": no_data,
        "halt": halt,
        "block_time": block_time,
        "tps": tps,
        "mempool": mempool,
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--run", required=True, help="Run token")
    parser.add_argument("--out", required=True)
    args = parser.parse_args()

    if not cl.TOKEN:
        print("ERROR: GRAFANA_TOKEN not set", file=sys.stderr)
        sys.exit(1)
    if cl.token_unixnano(args.run) is None:
        print(f"ERROR: run token '{args.run}' is not valid base36", file=sys.stderr)
        sys.exit(1)

    out_root = Path(args.out)
    out_root.mkdir(parents=True, exist_ok=True)

    for scenario in cl.SCENARIOS:
        stats = scenario_stats(args.run, scenario)
        (out_root / f"{scenario}.json").write_text(json.dumps(stats, indent=2))
        halt = stats["halt"]["verdict"]
        print(f"  {scenario}: {stats['provenance']} / halt={halt}", file=sys.stderr)


if __name__ == "__main__":
    main()
