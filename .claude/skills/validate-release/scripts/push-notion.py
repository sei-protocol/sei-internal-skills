#!/usr/bin/env python3
"""
Create the Notion report page using the claude.ai Notion MCP.
This script is a thin wrapper — the actual Notion API call is made by
Claude's mcp__claude_ai_Notion__notion-create-pages tool, which this
script signals via its exit state and payload file.

Usage:
  push-notion.py --suite-id <id> --state-dir <dir>

The script assembles the Notion page JSON from state-dir/ output and
writes it to state-dir/notion-payload.json. It then signals Claude to
call mcp__claude_ai_Notion__notion-create-pages with that payload.
Claude writes the resulting page URL to state-dir/notion-url.txt.

On failure (Notion API error), the payload remains at notion-payload.json
so the user can paste it manually.
"""
import argparse, json, os, sys, yaml
from pathlib import Path

DATABASE_ID = os.environ.get("NOTION_DATABASE_ID", "")


def load_image_urls(state_dir):
    url_file = state_dir / "panels" / "image-urls.yaml"
    if not url_file.exists():
        return {}
    return yaml.safe_load(url_file.read_text()) or {}


def load_reports(state_dir):
    reports = {}
    reports_dir = state_dir / "reports"
    if reports_dir.exists():
        for scenario_dir in reports_dir.iterdir():
            if scenario_dir.is_dir():
                rf = scenario_dir / "report.json"
                if rf.exists():
                    reports[scenario_dir.name] = json.loads(rf.read_text())
    return reports


def load_metrics(state_dir):
    metrics = {}
    metrics_dir = state_dir / "metrics"
    if metrics_dir.exists():
        for scenario_dir in metrics_dir.iterdir():
            if scenario_dir.is_dir():
                metrics[scenario_dir.name] = {}
                for window in ("baseline", "chaos", "recovery"):
                    wf = scenario_dir / f"{window}.json"
                    if wf.exists():
                        metrics[scenario_dir.name][window] = json.loads(wf.read_text())
    return metrics


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--suite-id", required=True)
    parser.add_argument("--state-dir", required=True)
    args = parser.parse_args()

    if not DATABASE_ID:
        print("ERROR: NOTION_DATABASE_ID not set", file=sys.stderr)
        sys.exit(1)

    state_dir = Path(args.state_dir)
    image_urls = load_image_urls(state_dir)
    reports = load_reports(state_dir)
    metrics = load_metrics(state_dir)

    sha7 = args.suite_id.split("-")[0]
    date_str = args.suite_id.split("-", 1)[-1] if "-" in args.suite_id else ""
    passed = sum(1 for r in reports.values() if r.get("exit_code", 1) == 0)
    failed = len(reports) - passed

    # The payload signals to Claude what data is assembled and ready.
    # Claude's platform-release-manager agent reads this file and calls
    # mcp__claude_ai_Notion__notion-create-pages with the structured blocks
    # built from analysis-guide.md + report-template.md.
    payload = {
        "database_id": DATABASE_ID,
        "suite_id": args.suite_id,
        "sha7": sha7,
        "date": date_str,
        "passed": passed,
        "failed": failed,
        "reports": reports,
        "metrics": metrics,
        "image_urls": image_urls,
        "state_dir": str(state_dir),
    }

    payload_file = state_dir / "notion-payload.json"
    payload_file.write_text(json.dumps(payload, indent=2))
    print(f"Notion payload written to {payload_file}", file=sys.stderr)
    print(f"Reports: {passed}/13 passed, {failed} failed", file=sys.stderr)
    # Signal to Claude that data is assembled and ready for Notion push.
    print("READY_FOR_NOTION_PUSH")


if __name__ == "__main__":
    main()
