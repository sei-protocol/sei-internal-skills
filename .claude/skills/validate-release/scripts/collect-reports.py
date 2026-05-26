#!/usr/bin/env python3
"""
Download seiload JSON reports for all 13 scenarios from S3.

Usage:
  collect-reports.py --suite-id <id> --out <dir>

Output: <out>/<scenario>/report.json for each found scenario.
        Exits 0 even if some reports are missing (logs which are absent).
        Exits 1 if fewer than 4 reports are found (suite likely incomplete).
"""
import argparse, boto3, json, os, sys
from pathlib import Path

BUCKET = os.environ.get("S3_BUCKET", "harbor-validation-results")
PREFIX = os.environ.get("S3_REPORT_PREFIX", "nightly")
PROFILE = os.environ.get("AWS_PROFILE", "sei")

SCENARIOS = [
    "chaos-network-partition", "chaos-network-latency", "chaos-packet-loss",
    "chaos-pod-failure", "chaos-cpu-stress", "chaos-memory-stress",
    "chaos-disk-io-latency", "chaos-container-kill", "chaos-bandwidth-limit",
    "chaos-dns-chaos", "chaos-time-skew", "chaos-byzantine-simulation",
    "chaos-rpc-chaos",
]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--suite-id", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()

    session = boto3.Session(profile_name=PROFILE)
    s3 = session.client("s3")
    out_root = Path(args.out)
    found = 0

    for scenario in SCENARIOS:
        key = f"{PREFIX}/{scenario}/{args.suite_id}/report.json"
        scenario_dir = out_root / scenario
        scenario_dir.mkdir(parents=True, exist_ok=True)
        out_file = scenario_dir / "report.json"
        try:
            obj = s3.get_object(Bucket=BUCKET, Key=key)
            data = json.loads(obj["Body"].read())
            out_file.write_text(json.dumps(data, indent=2))
            found += 1
            print(f"  {scenario}: downloaded", file=sys.stderr)
        except s3.exceptions.NoSuchKey:
            print(f"  {scenario}: MISSING (no report at s3://{BUCKET}/{key})", file=sys.stderr)
            (scenario_dir / "report.missing").write_text(key)
        except Exception as e:
            print(f"  {scenario}: ERROR — {e}", file=sys.stderr)

    print(f"\nCollected {found}/13 reports for suite {args.suite_id}", file=sys.stderr)
    if found < 4:
        print("ERROR: fewer than 4 reports found — suite likely incomplete", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
