#!/usr/bin/env python3
"""
Discover chaos runs from Prometheus and resolve one run token.

A chaos run = up to 10 chains sharing one <token>, each named
`chaos-<token>-<scenario>`. Discovery lists distinct chains over the raw
retention window and groups them by token; tokens order NUMERICALLY by the
base36 UnixNano they encode (D1/C2). The run id IS the token — no S3.

Usage:
  resolve-run.py [--run <token>] [--list] [--out <state-dir>]

Output (stdout): the resolved run token.
Exit: 0 on success; 1 if none found, or --run token has no chains.
"""
import argparse
import json
import sys
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path

import chaoslib as cl


def discover_runs():
    """
    -> {token: {scenario: chain_id}} for every chaos chain scraped in the
    retention window. Time-scoped via last_over_time (C3 — instant returns
    nothing after a run is torn down).
    """
    expr = (
        f'group by (chain_id)(last_over_time('
        f'{cl.M_HEIGHT}{{namespace="{cl.NIGHTLY_NS}", chain_id=~"chaos-.*"}}'
        f"[{cl.RAW_RETENTION}]))"
    )
    parsed = cl.prom_instant(expr)
    if not parsed["ok"]:
        raise RuntimeError(f"Prometheus discovery query failed: {parsed['error']}")
    runs = defaultdict(dict)
    for series in parsed["result"]:
        chain_id = series.get("metric", {}).get("chain_id", "")
        parsed_id = cl.parse_chain_id(chain_id)
        if parsed_id is None:
            continue
        token, scenario = parsed_id
        runs[token][scenario] = chain_id
    return runs, parsed["partial"]


def ordered_tokens(runs):
    """Tokens newest-first, ordered by base36-decoded UnixNano."""
    return sorted(runs.keys(), key=cl.token_sort_key, reverse=True)


def describe(token, scenarios):
    ts = cl.token_unixnano(token)
    when = ""
    if ts is not None:
        when = datetime.fromtimestamp(ts / 1e9, tz=timezone.utc).strftime("%Y-%m-%d %H:%M UTC")
    return f"{token}  ({len(scenarios)}/10 chains, {when})"


def write_manifest(out_dir, token, scenarios, partial):
    ts = cl.token_unixnano(token)
    manifest = {
        "token": token,
        "token_unixnano": ts,
        "run_start_utc": (
            datetime.fromtimestamp(ts / 1e9, tz=timezone.utc).isoformat() if ts is not None else None
        ),
        "chains": scenarios,  # {scenario: chain_id} actually discovered
        "discovered_scenarios": sorted(scenarios.keys()),
        "scenario_set": list(cl.SCENARIOS),
        "discovery_partial": partial,
    }
    out = Path(out_dir)
    out.mkdir(parents=True, exist_ok=True)
    (out / "run.json").write_text(json.dumps(manifest, indent=2))
    print(f"Run manifest written to {out / 'run.json'}", file=sys.stderr)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--run", help="Explicit run token to resolve")
    parser.add_argument("--list", action="store_true", help="List recent runs and exit")
    parser.add_argument("--out", help="State dir to write run.json manifest")
    args = parser.parse_args()

    if not cl.TOKEN:
        print("ERROR: GRAFANA_TOKEN not set", file=sys.stderr)
        sys.exit(1)

    try:
        runs, partial = discover_runs()
    except RuntimeError as e:
        print(f"ERROR: {e}", file=sys.stderr)
        sys.exit(1)

    if partial:
        print("WARNING: discovery returned a Thanos partial response — the run list may be incomplete.",
              file=sys.stderr)

    tokens = ordered_tokens(runs)

    if args.list or (not args.run and not tokens):
        print("Recent chaos runs (newest first):", file=sys.stderr)
        for i, tok in enumerate(tokens[:5]):
            print(f"  {i + 1}. {describe(tok, runs[tok])}", file=sys.stderr)
        if not tokens:
            print("  (none found in the retention window)", file=sys.stderr)
            sys.exit(1)
        if args.list:
            sys.exit(0)

    if args.run:
        if args.run not in runs:
            print(f"ERROR: no chaos chains found for run token '{args.run}' "
                  f"(older than the {cl.RAW_RETENTION} raw bound, or wrong token).",
                  file=sys.stderr)
            sys.exit(1)
        token = args.run
    else:
        token = tokens[0]

    scenarios = runs[token]
    print(f"Using run: {describe(token, scenarios)}", file=sys.stderr)
    missing = [s for s in cl.SCENARIOS if s not in scenarios]
    if missing:
        print(f"  scenarios with no chain series: {', '.join(missing)} (will reconcile downstream)",
              file=sys.stderr)

    if args.out:
        write_manifest(args.out, token, scenarios, partial)

    print(token)


if __name__ == "__main__":
    main()
