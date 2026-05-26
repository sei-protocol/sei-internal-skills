#!/usr/bin/env python3
"""
Resolve a chaos suite SUITE_ID.
If --suite-id is provided, validates it exists in S3.
Otherwise lists the 5 most recent runs and returns the latest.

Usage:
  resolve-suite-id.py [--suite-id <id>] [--list]

Output (stdout): the resolved SUITE_ID
Exit: 0 on success, 1 if not found or ambiguous
"""
import argparse, boto3, json, os, sys
from datetime import datetime

BUCKET = os.environ.get("S3_BUCKET", "harbor-validation-results")
PREFIX = os.environ.get("S3_REPORT_PREFIX", "nightly")
PROFILE = os.environ.get("AWS_PROFILE", "sei")
ANCHOR_SCENARIO = "chaos-network-partition"  # guaranteed to run first


def list_suite_ids(n=5):
    session = boto3.Session(profile_name=PROFILE)
    s3 = session.client("s3")
    prefix = f"{PREFIX}/{ANCHOR_SCENARIO}/"
    paginator = s3.get_paginator("list_objects_v2")
    ids = set()
    for page in paginator.paginate(Bucket=BUCKET, Prefix=prefix, Delimiter="/"):
        for p in page.get("CommonPrefixes", []):
            # prefix looks like "nightly/chaos-network-partition/abc1234-20260527/"
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
        except s3.exceptions.ClientError:
            pass
    return found


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--suite-id", help="Explicit SUITE_ID to validate")
    parser.add_argument("--list", action="store_true", help="List recent IDs and exit")
    args = parser.parse_args()

    recent = list_suite_ids()

    if args.list or not recent:
        print("Recent suite IDs:", file=sys.stderr)
        for i, sid in enumerate(recent):
            n = count_reports(sid)
            print(f"  {i+1}. {sid}  ({n}/13 reports)", file=sys.stderr)
        sys.exit(0)

    if args.suite_id:
        n = count_reports(args.suite_id)
        if n == 0:
            print(f"ERROR: No S3 reports found for suite-id '{args.suite_id}'", file=sys.stderr)
            sys.exit(1)
        print(args.suite_id)
        return

    # Default: latest
    suite_id = recent[0]
    n = count_reports(suite_id)
    print(f"Using latest suite: {suite_id} ({n}/13 reports)", file=sys.stderr)
    print(suite_id)


if __name__ == "__main__":
    main()
