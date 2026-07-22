#!/usr/bin/env python3
"""
Shared helpers for the validate-release scripts.

Single source of truth for:
  - the known 10-scenario chaos set (AC5 reconciliation depends on this)
  - chaos-<token>-<scenario> chain_id parsing + base36 token ordering
  - the Thanos-aware Prometheus query path (federated prometheus-prod
    datasource proxy, max_source_resolution=0, partial_response detection)

Imported by the sibling scripts; not run directly. Import works because the
script's own directory is on sys.path[0] when invoked as
`python3 .../scripts/<name>.py`.
"""
import os

import requests

# --- environment ---------------------------------------------------------
BASE_URL = os.environ.get("GRAFANA_BASE_URL", "https://grafana.prod.platform.sei.io")
TOKEN = os.environ.get("GRAFANA_TOKEN", "")
# Federated datasource: prod thanos-query fans out to harbor. NOT harbor-direct
# (C4: max_source_resolution=0 is Thanos-only, and harbor-local retention is far
# shorter than the 15d bucket the freshness bound relies on).
DS_UID = os.environ.get("PROM_DS_UID", "prometheus-prod")

# Harbor raw retention (Thanos bucket) — the enforced freshness bound (C6).
RAW_RETENTION = os.environ.get("RAW_RETENTION", "15d")

# Where the chaos chains live, and where the harness Job runs (C5).
NIGHTLY_NS = os.environ.get("NIGHTLY_NAMESPACE", "nightly")
HARBOR_CONTEXT = os.environ.get("HARBOR_CONTEXT", "harbor")
# Job names created by the `nightly-harness-suite` CronJob (harbor).
HARNESS_JOB_PREFIX = os.environ.get("HARNESS_JOB_PREFIX", "nightly-harness-suite")
# Log freshness (Go verdict lines are GC'd ~7d, vs ~15d metrics — the C6 band).
LOG_RETENTION = os.environ.get("LOG_RETENTION", "7d")

# --- the chaos scenario set (authoritative) ------------------------------
# The 10 scenarios TestNightlyChaosSuite runs. Order = report/render order.
# A run has UP TO 10 chains sharing one <token>; any absent scenario reconciles
# to DID NOT RUN (log) / NO DATA (metrics) — never a silent drop, never a green 0.
SCENARIOS = (
    "network-partition",
    "packet-loss",
    "network-latency",
    "bandwidth-limit",
    "byzantine",
    "pod-failure",
    "container-kill",
    "cpu-stress",
    "time-skew",
    "memory-stress",
)

# Live-confirmed raw harbor metric names (C1). NOT the prod-scoped chaos-suite
# recording rules (C4 — empty for harbor).
M_HEIGHT = "tendermint_consensus_height"
M_BLOCK_INTERVAL_BUCKET = "tendermint_consensus_block_interval_seconds_bucket"
M_TPS = "sei_cosmos_throughput_transaction_count"
M_MEMPOOL = "tendermint_mempool_size"

QUORUM_OF_FOUR = 3  # ceil(2/3 * 4): "not a halt" needs >= this many validators advancing


def parse_duration_seconds(dur: str) -> int:
    """Parse a Prometheus-style duration (e.g. '15d', '3600s', '90m', '2h') to seconds."""
    dur = dur.strip()
    units = {"s": 1, "m": 60, "h": 3600, "d": 86400, "w": 604800}
    unit = dur[-1]
    if unit in units:
        return int(float(dur[:-1]) * units[unit])
    return int(float(dur))  # bare number = seconds


def parse_chain_id(chain_id: str):
    """
    chaos-<token>-<scenario> -> (token, scenario), or None if it is not a
    recognised chaos chain. Scenario names contain hyphens, so we match the
    known suffix set rather than splitting on '-' (C2).
    """
    if not chain_id.startswith("chaos-"):
        return None
    remainder = chain_id[len("chaos-"):]
    for scenario in SCENARIOS:
        suffix = f"-{scenario}"
        if remainder.endswith(suffix):
            token = remainder[: -len(suffix)]
            if token:
                return token, scenario
    return None


def token_sort_key(token: str) -> int:
    """
    Order tokens NUMERICALLY by decoding the base36 UnixNano (C2). Falls back to
    a stable low key for a malformed token so discovery never crashes on junk.
    """
    try:
        return int(token, 36)
    except (ValueError, TypeError):
        return -1


def token_unixnano(token: str):
    """Decode the base36 token to its UnixNano start, or None if malformed."""
    try:
        return int(token, 36)
    except (ValueError, TypeError):
        return None


def chain_id_for(token: str, scenario: str) -> str:
    return f"chaos-{token}-{scenario}"


# --- Prometheus / Thanos query path --------------------------------------
def _proxy_url(endpoint: str) -> str:
    return f"{BASE_URL}/api/datasources/proxy/uid/{DS_UID}/api/v1/{endpoint}"


def _headers() -> dict:
    return {"Authorization": f"Bearer {TOKEN}"}


def _parse_response(resp) -> dict:
    """
    Normalise a Prometheus/Thanos JSON response into a common shape.
    Distinguishes error, partial-response (Thanos warnings), and empty result.
    Never raises — callers must be able to mark a cell degraded rather than crash.
    """
    try:
        resp.raise_for_status()
        data = resp.json()
    except Exception as e:  # noqa: BLE001 - any transport/parse failure is a degraded cell
        return {"ok": False, "partial": False, "error": str(e), "result": [], "warnings": []}
    if data.get("status") != "success":
        return {
            "ok": False,
            "partial": False,
            "error": data.get("error", "non-success status"),
            "result": [],
            "warnings": data.get("warnings", []),
        }
    warnings = data.get("warnings", []) or []
    return {
        "ok": True,
        # Thanos surfaces a partial fan-out as top-level warnings; treat as degraded (C, D2).
        "partial": bool(warnings),
        "error": None,
        "result": data.get("data", {}).get("result", []),
        "warnings": warnings,
    }


def prom_instant(expr: str, at=None, timeout: int = 60) -> dict:
    """Time-scoped instant query (evaluated at `at`). Forces raw resolution."""
    params = {"query": expr, "max_source_resolution": "0"}
    if at is not None:
        params["time"] = at
    try:
        resp = requests.get(_proxy_url("query"), headers=_headers(), params=params, timeout=timeout)
    except Exception as e:  # noqa: BLE001
        return {"ok": False, "partial": False, "error": str(e), "result": [], "warnings": []}
    return _parse_response(resp)


def prom_range(expr: str, start, end, step="15s", timeout: int = 60) -> dict:
    """Range query over [start, end]. Forces raw resolution (never instant — C3)."""
    params = {
        "query": expr,
        "start": start,
        "end": end,
        "step": step,
        "max_source_resolution": "0",
    }
    try:
        resp = requests.get(
            _proxy_url("query_range"), headers=_headers(), params=params, timeout=timeout
        )
    except Exception as e:  # noqa: BLE001
        return {"ok": False, "partial": False, "error": str(e), "result": [], "warnings": []}
    return _parse_response(resp)


def scalar_from_instant(parsed: dict):
    """Max numeric value across the returned instant vectors, or None if absent."""
    vals = []
    for series in parsed.get("result", []):
        v = series.get("value")
        if v and len(v) == 2:
            try:
                f = float(v[1])
                if f == f and f not in (float("inf"), float("-inf")):  # skip NaN/Inf
                    vals.append(f)
            except (ValueError, TypeError):
                pass
    return max(vals) if vals else None


def series_numeric_values(series: dict):
    """[(ts, float)] for a range-query series, dropping NaN/Inf."""
    out = []
    for ts, raw in series.get("values", []):
        try:
            f = float(raw)
        except (ValueError, TypeError):
            continue
        if f != f or f in (float("inf"), float("-inf")):
            continue
        out.append((float(ts), f))
    return out
