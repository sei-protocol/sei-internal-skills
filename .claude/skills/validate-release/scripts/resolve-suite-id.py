#!/usr/bin/env python3
"""
Resolve a chaos suite SUITE_ID from S3 or validate a provided one.
If --suite-id is provided, validates it first (even when S3 listing fails).
Otherwise lists the 5 most recent runs and returns the latest.

Usage:
  resolve-suite-id.py [--suite-id <id>] [--list]

Output (stdout): the resolved SUITE_ID
Exit: 0 on success, 1 if not found or ambiguous
"""
import argparse, boto3, os, sys

BUCKET = os.environ.get("S3_BUCKET", "harbor-validation-results")
PREFIX = os.environ.get("S3_REPORT_PREFIX", "nightly")
PROFILE = os.environ.get("AWS_PROFILE", "sei")
ANCHOR_SCENARIO = "chaos-network-partition"


def list_suite_ids(n=5):
    session = boto3.Session(profile_name=PROFILE)
    s3 = session.client("s3")
    prefix = f"{PREFIX}/{ANCHOR_SCENARIO}/"
    paginator = s3.get_paginator("list_objects_v2")
    ids = set()
    for page in paginator.paginate(Bucket=BUCKET, Prefix=prefix, Delimiter="/"):
        for p in page.get("CommonPrefixes", []):
            suite_id = p["Prefix"].rstrip("/").split("/")[-1]
            if suite_id:
                ids.add(suite_id)
    return sorted(ids, reverse=True)[:n]


def count_reports(suite_id):
    session = boto3.Session(profile_name=PROFILE)
    s3 = session.client("s3")
    scenarios = [
        "chaos-network-partition", "chaos-network-latency", "chaos-packet-loss",
        "chaos-pod-failure", "chaos-cpu-stress", "chaos-memory-stress",
        "chaos-disk-io-latency", "chaos-container-kill", "chaos-bandwidth-limit",
        "chaos-dns-chaos", "chaos-time-skew", "chaos-byzantine-simulation",
        "chaos-rpc-chaos",
    ]
    found = 0
    for sc in scenarios:
        try:
            s3.head_object(Bucket=BUCKET, Key=f"{PREFIX}/{sc}/{suite_id}/report.json")
            found += 1
        except Exception:
            pass
    return found


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--suite-id", help="Explicit SUITE_ID to validate")
    parser.add_argument("--list", action="store_true", help="List recent IDs and exit")
    args = parser.parse_args()

    # Validate explicit --suite-id FIRST, before any S3 listing that might fail
    if args.suite_id and not args.list:
        n = count_reports(args.suite_id)
        if n == 0:
            print(f"ERROR: No S3 reports found for suite-id '{args.suite_id}'", file=sys.stderr)
            sys.exit(1)
        print(f"Suite {args.suite_id}: {n}/13 reports found", file=sys.stderr)
        print(args.suite_id)
        return

    # List or auto-discover
    try:
        recent = list_suite_ids()
    except Exception as e:
        print(f"ERROR: S3 listing failed: {e}", file=sys.stderr)
        sys.exit(1)

    if args.list or not recent:
        print("Recent suite IDs:", file=sys.stderr)
        for i, sid in enumerate(recent):
            n = count_reports(sid)
            print(f"  {i+1}. {sid}  ({n}/13 reports)", file=sys.stderr)
        if not recent:
            print("  (none found)", file=sys.stderr)
            sys.exit(1)
        sys.exit(0)

    # Default: latest
    suite_id = recent[0]
    n = count_reports(suite_id)
    print(f"Using latest suite: {suite_id} ({n}/13 reports)", file=sys.stderr)
    print(suite_id)


if __name__ == "__main__":
    main()
