#!/usr/bin/env python3
"""
Join the AUTHORITATIVE Job-log verdict (D3) with the D2 metric summary into a
per-scenario outcome, and compute the run headline (D4).

The per-scenario outcome IS the Job-log PASS/FAIL — metrics are supporting
context, never a second opinion. The anti-fabrication rule (AC3): when the log
is GC'd but metrics survive (the 7d/15d band), each scenario renders
`VERDICT UNAVAILABLE — metrics-only` and the go/no-go headline is SUPPRESSED —
never a synthesized "N/N passed".

Usage:
  compute-stats.py --run-log <dir> --metrics-dir <dir> --out <dir>

Input:  <run-log>/verdicts.json  and  <metrics-dir>/<scenario>.json
Output: <out>/<scenario>.json (per scenario) + <out>/summary.json
"""
import argparse
import json
import sys
import time
from pathlib import Path

import chaoslib as cl

VERDICT_UNAVAILABLE = "VERDICT UNAVAILABLE — metrics-only"
RUN_EXPIRED = "RUN EXPIRED — no log, no raw metrics"


def metric_summary(metrics: dict) -> dict:
    """Condense the D2 stats dict into the supporting-evidence annotation."""
    if not metrics:
        return {"provenance": "NO DATA", "halt": "NO DATA"}
    halt = metrics.get("halt", {})
    bt = metrics.get("block_time", {})
    tps = metrics.get("tps", {})
    mp = metrics.get("mempool", {})
    return {
        "provenance": metrics.get("provenance", "NO DATA"),
        "halt": halt.get("verdict"),
        "blocks_produced": halt.get("blocks_produced"),
        "quorum_advanced": halt.get("quorum_advanced"),
        "block_time_p95_seconds": bt.get("p95_seconds"),
        "block_time_mean_seconds": bt.get("mean_interval_seconds"),
        "block_time_source": bt.get("source"),
        # TPS + mempool are ~0 by design (chaos runs no load generator) — transparency
        # only, NOT chaos release signals (halt + block-interval are). Never narrated as
        # a TPS degradation shape or mempool backpressure.
        "transparency_only": {
            "note": "chaos runs no load generator; TPS/mempool ~0 by design — NOT release signals",
            "tps_last": None if tps.get("no_data") else tps.get("last"),
            "tps_max": None if tps.get("no_data") else tps.get("max"),
            "mempool_peak": mp.get("peak"),
        },
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--run-log", required=True, help="dir containing verdicts.json")
    parser.add_argument("--metrics-dir", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()

    log = json.loads((Path(args.run_log) / "verdicts.json").read_text())
    metrics_root = Path(args.metrics_dir)
    out_root = Path(args.out)
    out_root.mkdir(parents=True, exist_ok=True)

    token = log.get("token")
    log_available = log.get("log_available", False)
    log_verdicts = log.get("scenarios", {})

    # Run age vs the raw freshness bound (C6).
    retention_seconds = cl.parse_duration_seconds(cl.RAW_RETENTION)
    run_nano = cl.token_unixnano(token) if token else None
    run_age = int(time.time() - run_nano / 1e9) if run_nano else None
    raw_expired_bound = run_age is not None and run_age > retention_seconds

    scenarios = {}
    for scenario in cl.SCENARIOS:
        mfile = metrics_root / f"{scenario}.json"
        metrics = json.loads(mfile.read_text()) if mfile.exists() else {}
        summ = metric_summary(metrics)
        has_metrics = bool(metrics) and not metrics.get("no_data", True)

        if not log_available:
            # 7d/15d band: metrics may survive but the verdict does not.
            if has_metrics:
                outcome = VERDICT_UNAVAILABLE
                provenance = "VERDICT-GC'd"
            else:
                outcome = RUN_EXPIRED
                provenance = "NO DATA"
        else:
            outcome = log_verdicts.get(scenario, "DID NOT RUN")
            provenance = summ["provenance"]

        entry = {
            "scenario": scenario,
            "outcome": outcome,          # Job-log verdict is authoritative
            "provenance": provenance,    # OK / NO DATA / PARTIAL / VERDICT-GC'd
            "metrics": summ,
        }
        scenarios[scenario] = entry
        (out_root / f"{scenario}.json").write_text(json.dumps(entry, indent=2))
        print(f"  {scenario}: {outcome} [{provenance}]", file=sys.stderr)

    # Headline (D4): from the Job-log verdicts only; suppressed when unavailable.
    passed = sum(1 for e in scenarios.values() if e["outcome"] == "PASS")
    failed = sum(1 for e in scenarios.values() if e["outcome"] == "FAIL")
    did_not_run = sum(1 for e in scenarios.values() if e["outcome"] == "DID NOT RUN")
    unknown = sum(1 for e in scenarios.values() if str(e["outcome"]).startswith("UNKNOWN"))

    headline_suppressed = not log_available
    headline = None
    headline_reason = None
    if headline_suppressed:
        headline_reason = (
            "verdict log GC'd but metrics survive (7d/15d band) — re-run; do NOT ship on metrics alone"
            if any(e["outcome"] == VERDICT_UNAVAILABLE for e in scenarios.values())
            else "no log and no raw metrics survive — run expired; re-run"
        )
    elif failed > 0:
        headline = "LIVENESS NO-GO"
        headline_reason = f"{failed} scenario(s) failed the liveness gate"
    elif did_not_run > 0 or unknown > 0:
        headline = "LIVENESS NO-GO"
        headline_reason = f"incomplete run: {did_not_run} did not run, {unknown} unknown (log truncated)"
    elif passed == len(cl.SCENARIOS):
        headline = "LIVENESS GO"
        headline_reason = "all 10 scenarios passed the liveness gate"
    else:
        headline = "LIVENESS NO-GO"
        headline_reason = "not all scenarios reported PASS"

    summary = {
        "token": token,
        "release_image": log.get("release_image"),
        "job_name": log.get("job_name"),
        "log_available": log_available,
        "log_unavailable_reason": log.get("unavailable_reason"),
        "log_truncated": log.get("log_truncated", False),
        "run_age_seconds": run_age,
        "retention_seconds": retention_seconds,
        "raw_expired_bound": raw_expired_bound,
        "headline": headline,
        "headline_suppressed": headline_suppressed,
        "headline_reason": headline_reason,
        "liveness_caveat": (
            "Liveness gate only — TestNightlyChaosSuite asserts the chain stayed live and "
            "recovered; tx-correctness is NOT validated."
        ),
        "counts": {
            "pass": passed, "fail": failed,
            "did_not_run": did_not_run, "unknown": unknown,
        },
        "outcomes": {s: e["outcome"] for s, e in scenarios.items()},
    }
    (out_root / "summary.json").write_text(json.dumps(summary, indent=2))

    if headline_suppressed:
        print(f"\nHeadline SUPPRESSED — {headline_reason}", file=sys.stderr)
    else:
        print(f"\nHeadline: {headline} — {headline_reason}", file=sys.stderr)


if __name__ == "__main__":
    main()
