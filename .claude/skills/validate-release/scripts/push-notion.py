#!/usr/bin/env python3
"""
Assemble the Notion report payload for a run and signal Claude to push it.

Thin wrapper: it builds the structured payload from compute-stats' summary +
per-scenario verdicts + metric annotations + panel image URLs, writes it to
state-dir/notion-payload.json, and prints READY_FOR_NOTION_PUSH. The actual
Notion write is Claude's mcp__claude_ai_Notion__notion-create-pages call.

Rendering contract (D4 / AC9):
  - Headline is `LIVENESS GO` / `LIVENESS NO-GO` (never a bare "GO"), with the
    "tx-correctness not validated" caveat inline.
  - When compute-stats suppressed the headline (verdict GC'd / run expired), NO
    go/no-go headline is emitted — never a synthesized pass.
  - Run-identity header: run token, release image, run age vs the 15d raw bound.
  - Per-cell provenance marker: OK / NO DATA / PARTIAL / VERDICT-GC'd.

Usage:
  push-notion.py --run <token> --state-dir <dir>
"""
import argparse
import json
import os
import sys
from pathlib import Path

import yaml

DATABASE_ID = os.environ.get("NOTION_DATABASE_ID", "")


def fmt_age(seconds):
    if seconds is None:
        return "unknown"
    days = seconds / 86400
    return f"{days:.1f}d"


def load_image_urls(state_dir: Path) -> dict:
    url_file = state_dir / "panels" / "image-urls.yaml"
    if not url_file.exists():
        return {}
    return yaml.safe_load(url_file.read_text()) or {}


def load_verdicts(state_dir: Path):
    vdir = state_dir / "verdicts"
    summary = json.loads((vdir / "summary.json").read_text())
    scenarios = {}
    for f in vdir.glob("*.json"):
        if f.name == "summary.json":
            continue
        data = json.loads(f.read_text())
        scenarios[data["scenario"]] = data
    return summary, scenarios


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--run", required=True, help="Run token")
    parser.add_argument("--state-dir", required=True)
    args = parser.parse_args()

    if not DATABASE_ID:
        print("ERROR: NOTION_DATABASE_ID not set", file=sys.stderr)
        sys.exit(1)

    state_dir = Path(args.state_dir)
    summary, scenarios = load_verdicts(state_dir)
    image_urls = load_image_urls(state_dir)

    retention_days = summary["retention_seconds"] / 86400
    run_age = fmt_age(summary["run_age_seconds"])

    # Headline block (AC9): never bare "GO"; caveat inline; suppressed when unavailable.
    if summary["headline_suppressed"]:
        headline_block = {
            "suppressed": True,
            "notice": f"NO GO/NO-GO — {summary['headline_reason']}",
        }
    else:
        headline_block = {
            "suppressed": False,
            "text": summary["headline"],  # "LIVENESS GO" / "LIVENESS NO-GO"
            "reason": summary["headline_reason"],
            "caveat": summary["liveness_caveat"],
        }

    identity = {
        "run_token": summary["token"],
        "release_image": summary.get("release_image") or "<unknown — log unavailable>",
        "run_age": run_age,
        "freshness_bound_days": retention_days,
        "within_bound": not summary.get("raw_expired_bound", False),
        "job_name": summary.get("job_name"),
        "log_truncated": summary.get("log_truncated", False),
    }

    payload = {
        "database_id": DATABASE_ID,
        "run_token": args.run,
        "identity": identity,
        "headline": headline_block,
        "counts": summary["counts"],
        "scenarios": scenarios,        # each carries outcome + provenance + metrics
        "image_urls": image_urls,
        "state_dir": str(state_dir),
    }

    payload_file = state_dir / "notion-payload.json"
    payload_file.write_text(json.dumps(payload, indent=2))
    print(f"Notion payload written to {payload_file}", file=sys.stderr)
    if headline_block["suppressed"]:
        print(f"Headline SUPPRESSED — {summary['headline_reason']}", file=sys.stderr)
    else:
        print(f"Headline: {summary['headline']} "
              f"({summary['counts']['pass']} PASS / {summary['counts']['fail']} FAIL)",
              file=sys.stderr)
    print("READY_FOR_NOTION_PUSH")


if __name__ == "__main__":
    main()
