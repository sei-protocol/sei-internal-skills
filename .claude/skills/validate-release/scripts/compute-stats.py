#!/usr/bin/env python3
"""
Compute deterministic statistics and outcome classification for each scenario.
Reads the three window JSONs from query-grafana.py and emits verdict.json.

The agent reads verdict.json and narrates it — it does not derive numbers.

Usage:
  compute-stats.py --metrics-dir <dir> --out <dir>

Input:  <metrics-dir>/<scenario>/{baseline,chaos,recovery}.json
Output: <out>/<scenario>/verdict.json
"""
import argparse, json, os, sys
from pathlib import Path

# Outcome thresholds (from analysis-guide.md — now enforced in code)
DEGRADED_BLOCK_TIME_RATIO = 1.20   # >20% increase from baseline = DEGRADED
HALT_BLOCK_COUNT_THRESHOLD = 2     # fewer than 2 block_time samples = halted
CLEAN_RECOVERY_MULTIPLIER = 2.0    # recovery_seconds < 2× chaos_duration = clean
NOISE_MIN_SAMPLES = 6              # flag as noisy when fewer than this many samples

OUTCOMES = ("PASS", "DEGRADED", "HALT+RECOVER", "FAIL")


def stats_from_window(window: dict) -> dict:
    """Extract per-metric stats from a single window dict."""
    result = {}
    for metric in ("block_time_p50", "block_time_p95", "tps", "mempool_size"):
        m = window.get(metric, {})
        if not isinstance(m, dict) or m.get("no_data"):
            result[metric] = {"avg": None, "min": None, "max": None, "count": 0}
        else:
            result[metric] = {
                "avg": m.get("avg"),
                "min": m.get("min"),
                "max": m.get("max"),
                "count": m.get("count", 0),
            }
    return result


def compute_recovery_seconds(chaos_window: dict, recovery_window: dict, baseline_avg: float) -> float | None:
    """
    Find how many seconds into the recovery window the metric first returned
    to ≤110% of baseline. Returns None if it never recovered.
    """
    if baseline_avg is None or baseline_avg == 0:
        return None
    threshold = baseline_avg * 1.10
    series = recovery_window.get("block_time_p50", {}).get("series", [])
    if not series:
        return None
    chaos_end = chaos_window.get("_window", {}).get("end", 0)
    for ts, val in series:
        try:
            if float(val) <= threshold:
                return float(ts) - chaos_end
        except (ValueError, TypeError):
            pass
    return None


def classify_outcome(
    baseline_stats: dict,
    chaos_stats: dict,
    recovery_seconds: float | None,
    chaos_duration: float,
) -> str:
    """Deterministic four-way outcome classification."""
    bt_baseline = baseline_stats.get("block_time_p50", {}).get("avg")
    bt_chaos_count = chaos_stats.get("block_time_p50", {}).get("count", 0)
    bt_chaos_avg = chaos_stats.get("block_time_p50", {}).get("avg")

    # HALT: use block height delta rather than recording-rule sample count.
    # Recording rules carry stale values forward (Prometheus 5m staleness),
    # so count >= 2 even during a real halt. Rate==0 reliably means no blocks.
    bt_height_rate = chaos_stats.get("block_height_delta", {}).get("avg")
    halted = (bt_height_rate is not None and bt_height_rate == 0) or bt_chaos_count < HALT_BLOCK_COUNT_THRESHOLD
    if halted:
        if recovery_seconds is not None and recovery_seconds < chaos_duration * CLEAN_RECOVERY_MULTIPLIER:
            return "HALT+RECOVER"
        return "FAIL"  # halted and did not recover cleanly

    # DEGRADED: block time meaningfully increased
    if bt_baseline and bt_chaos_avg and bt_chaos_avg > bt_baseline * DEGRADED_BLOCK_TIME_RATIO:
        return "DEGRADED"

    return "PASS"


def compute_verdict(scenario_dir: Path) -> dict:
    """Compute the full verdict for one scenario."""
    def load(name):
        f = scenario_dir / f"{name}.json"
        return json.loads(f.read_text()) if f.exists() else {}

    baseline = load("baseline")
    chaos = load("chaos")
    recovery = load("recovery")

    b = stats_from_window(baseline)
    c = stats_from_window(chaos)
    r = stats_from_window(recovery)

    chaos_dur = (chaos.get("_window", {}).get("end", 0)
                 - chaos.get("_window", {}).get("start", 0)) or 600

    recovery_seconds = compute_recovery_seconds(chaos, recovery, b["block_time_p50"]["avg"])

    outcome = classify_outcome(b, c, recovery_seconds, chaos_dur)

    # Compute deltas (chaos vs baseline)
    def delta(metric):
        bv = b.get(metric, {}).get("avg")
        cv = c.get(metric, {}).get("avg")
        if bv is None or cv is None or bv == 0:
            return None
        return {"absolute": round(cv - bv, 4), "ratio": round(cv / bv, 3)}

    # Noise flag: warn agent when sample count is too low for reliable classification
    noise_flag = (
        b["block_time_p50"]["count"] < NOISE_MIN_SAMPLES
        or c["block_time_p50"]["count"] < NOISE_MIN_SAMPLES
    )

    return {
        "outcome": outcome,
        "noise_flag": noise_flag,
        "baseline": b,
        "chaos": c,
        "recovery": r,
        "deltas": {
            "block_time_p50": delta("block_time_p50"),
            "block_time_p95": delta("block_time_p95"),
            "tps":            delta("tps"),
            "mempool_size":   delta("mempool_size"),
        },
        "recovery_seconds": round(recovery_seconds, 1) if recovery_seconds is not None else None,
        "chaos_duration_seconds": chaos_dur,
        "windows": {
            "baseline": baseline.get("_window", {}),
            "chaos":    chaos.get("_window", {}),
            "recovery": recovery.get("_window", {}),
        },
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--metrics-dir", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()

    metrics_root = Path(args.metrics_dir)
    out_root = Path(args.out)
    out_root.mkdir(parents=True, exist_ok=True)

    results = {}
    for scenario_dir in sorted(metrics_root.iterdir()):
        if not scenario_dir.is_dir():
            continue
        scenario = scenario_dir.name
        out_dir = out_root / scenario
        out_dir.mkdir(exist_ok=True)

        verdict = compute_verdict(scenario_dir)
        verdict_file = out_dir / "verdict.json"
        verdict_file.write_text(json.dumps(verdict, indent=2))
        results[scenario] = verdict["outcome"]
        flag = " [NOISY]" if verdict["noise_flag"] else ""
        print(f"  {scenario}: {verdict['outcome']}{flag}", file=sys.stderr)

    # Write summary
    summary = {
        "outcomes": results,
        "pass":  sum(1 for v in results.values() if v == "PASS"),
        "degraded": sum(1 for v in results.values() if v == "DEGRADED"),
        "halt_recover": sum(1 for v in results.values() if v == "HALT+RECOVER"),
        "fail":  sum(1 for v in results.values() if v == "FAIL"),
    }
    (out_root / "summary.json").write_text(json.dumps(summary, indent=2))
    print(f"\nSummary: {summary['pass']} PASS, {summary['degraded']} DEGRADED, "
          f"{summary['halt_recover']} HALT+RECOVER, {summary['fail']} FAIL", file=sys.stderr)


if __name__ == "__main__":
    main()
