#!/usr/bin/env python3
"""
Upload rendered panel PNGs to S3 and generate 7-day presigned URLs.

Usage:
  upload-images.py --dir <panels-dir> --suite-id <id> [--out <yaml-file>]

Output: YAML file mapping scenario → panel → presigned URL.
        Prints to stdout if --out not provided.
"""
import argparse, boto3, os, sys, yaml
from pathlib import Path

BUCKET = os.environ.get("S3_BUCKET", "harbor-validation-results")
PROFILE = os.environ.get("AWS_PROFILE", "sei")
EXPIRY = 604800  # 7 days


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--dir", required=True, help="panels/ directory from render-panels.py")
    parser.add_argument("--suite-id", required=True)
    parser.add_argument("--out", help="output YAML file path")
    args = parser.parse_args()

    session = boto3.Session(profile_name=PROFILE)
    s3 = session.client("s3")
    panels_root = Path(args.dir)
    urls = {}

    for scenario_dir in sorted(panels_root.iterdir()):
        if not scenario_dir.is_dir():
            continue
        scenario = scenario_dir.name
        urls[scenario] = {}
        for png_file in scenario_dir.glob("*.png"):
            panel_name = png_file.stem
            s3_key = f"chaos-suite-reports/{args.suite_id}/{scenario}/{panel_name}.png"
            s3.upload_file(str(png_file), BUCKET, s3_key,
                           ExtraArgs={"ContentType": "image/png"})
            url = s3.generate_presigned_url(
                "get_object",
                Params={"Bucket": BUCKET, "Key": s3_key},
                ExpiresIn=EXPIRY,
            )
            urls[scenario][panel_name] = url
            print(f"  {scenario}/{panel_name}: uploaded", file=sys.stderr)

        for failed_file in scenario_dir.glob("*.failed"):
            panel_name = failed_file.stem
            urls[scenario][panel_name] = None
            print(f"  {scenario}/{panel_name}: skipped (render failed)", file=sys.stderr)

    output = yaml.dump(urls, default_flow_style=False)
    if args.out:
        Path(args.out).write_text(output)
        print(f"Image URLs written to {args.out}", file=sys.stderr)
    else:
        print(output)


if __name__ == "__main__":
    main()
