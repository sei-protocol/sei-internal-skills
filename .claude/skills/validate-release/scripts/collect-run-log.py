#!/usr/bin/env python3
"""
Read the harbor harness Job for a run: the release image under test from the Job
SPEC env (SEID_IMAGE_CHAOS, a Flux-substituted literal), and the AUTHORITATIVE
per-scenario verdict from the pod LOG (`--- PASS|FAIL: TestNightlyChaosSuite/<scenario>`)
(C5, D3). The image is not parsed from the log (the chaos suite emits no image line;
the only image line belongs to the upgrade suite's chain — parsing it would mis-source).

Join token -> Job by WINDOW CONTAINMENT + LOG VERIFICATION: among nightly harness
Jobs, keep those whose run window (creationTimestamp .. completionTime+tol) contains
the token time, then bind to the first candidate whose pod-log actually references
this run's chains (`chaos-<token>-`). No verified candidate -> VERDICT UNAVAILABLE;
image + verdict are never mis-attributed to an unrelated nightly.

The log read is byte-bounded; a truncated read is FLAGGED, not treated as
"scenario missing". Scenarios with no PASS/FAIL line reconcile against the
known 10-set: DID NOT RUN (complete log) vs UNKNOWN (truncated).

When no Job/log survives (the 7d/15d band) this writes log_available=false and
exits 0 — compute-stats then renders VERDICT UNAVAILABLE, never a synthesized pass.

Usage:
  collect-run-log.py --run <token> --out <dir>
"""
import argparse
import json
import os
import re
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

import chaoslib as cl

LOG_BYTE_CAP = int(os.environ.get("LOG_BYTE_CAP", str(20 * 1024 * 1024)))  # 20 MiB
# Tolerance on the Job's completion time when testing whether the run window
# CONTAINS the token, absorbing clock/ordering wobble at the trailing edge.
JOIN_TOLERANCE_SECONDS = int(os.environ.get("JOIN_TOLERANCE_SECONDS", "120"))

# The nightly wrapper logs indented `--- PASS|FAIL: TestNightlyChaosSuite/<scenario>`
# subtest lines (^\s* tolerates Go's indentation). Nightly-only: PR-triggered
# TestChaosSuite runs are out of slice-1 scope.
VERDICT_RE = re.compile(r"^\s*--- (PASS|FAIL): TestNightlyChaosSuite/(\S+)", re.MULTILINE)


def _norm(name: str) -> str:
    return name.lower().replace("_", "-").replace(" ", "-")


def _compact(name: str) -> str:
    return _norm(name).replace("-", "")


def match_scenario(subtest: str):
    """Map a Go subtest name to one of the 10 known scenarios, or None."""
    n, c = _norm(subtest), _compact(subtest)
    for scenario in cl.SCENARIOS:
        if n == _norm(scenario) or c == _compact(scenario):
            return scenario
    return None


def kubectl_json(args):
    cmd = ["kubectl", "--context", cl.HARBOR_CONTEXT, "-n", cl.NIGHTLY_NS, *args, "-o", "json"]
    out = subprocess.run(cmd, capture_output=True, text=True, timeout=60)
    if out.returncode != 0:
        raise RuntimeError(out.stderr.strip() or f"kubectl {' '.join(args)} failed")
    return json.loads(out.stdout)


def parse_k8s_ts(ts: str) -> float:
    # K8s metav1.Time is usually whole-second `...Z` but MAY carry fractional seconds;
    # fromisoformat handles both (+ the Z offset) where strptime("%...SZ") would raise.
    return datetime.fromisoformat(ts.replace("Z", "+00:00")).timestamp()


def find_job(token_epoch: float):
    """
    Candidate nightly harness Jobs whose run WINDOW contains the token time:
    creationTimestamp <= token_time <= completionTime + tolerance. The chaos
    suite starts well after the Job is created, so a tight +/-120s bound around
    creation is wrong — the token (a chaos-chain start) falls INSIDE the run
    window, not next to its start. A still-running Job (no completionTime) uses
    now as the window end. Returned newest-first; the caller VERIFIES each
    candidate against the run's own chains before trusting its verdict/image.
    """
    jobs = kubectl_json(["get", "jobs"]).get("items", [])
    now = datetime.now(timezone.utc).timestamp()
    candidates = []
    for job in jobs:
        name = job.get("metadata", {}).get("name", "")
        if not name.startswith(cl.HARNESS_JOB_PREFIX):
            continue
        created = job.get("metadata", {}).get("creationTimestamp")
        if not created:
            continue
        c_epoch = parse_k8s_ts(created)
        completion = job.get("status", {}).get("completionTime")
        end_epoch = (parse_k8s_ts(completion) if completion else now) + JOIN_TOLERANCE_SECONDS
        if c_epoch <= token_epoch <= end_epoch:
            candidates.append((c_epoch, job))
    candidates.sort(key=lambda x: x[0], reverse=True)
    return [job for _c, job in candidates]


def log_references_run(log_text: str, token: str) -> bool:
    """A candidate Job's log must actually mention this run — its chaos chains
    (`chaos-<token>-`) or the bare token — before we trust its verdict/image.
    Guards against binding a run to an unrelated nightly whose window overlaps."""
    if not log_text:
        return False
    return f"chaos-{token}-" in log_text or token in log_text


def extract_image(job: dict):
    spec = job.get("spec", {}).get("template", {}).get("spec", {})
    for container in spec.get("containers", []):
        for env in container.get("env", []):
            if env.get("name") == "SEID_IMAGE_CHAOS":
                if "value" in env:
                    return env["value"]
                return "<valueFrom — not resolvable from Job spec>"
    return None


def read_log(job_name: str):
    """(text, truncated) or (None, False) when no pod log survives (GC'd)."""
    cmd = [
        "kubectl", "--context", cl.HARBOR_CONTEXT, "-n", cl.NIGHTLY_NS,
        "logs", f"job/{job_name}", f"--limit-bytes={LOG_BYTE_CAP}", "--prefix=false",
    ]
    out = subprocess.run(cmd, capture_output=True, text=True, timeout=120)
    if out.returncode != 0:
        return None, False
    text = out.stdout
    truncated = len(text.encode("utf-8", "replace")) >= LOG_BYTE_CAP
    return text, truncated


def build_verdicts(log_text: str, truncated: bool):
    found = {}
    for verdict, subtest in VERDICT_RE.findall(log_text):
        scenario = match_scenario(subtest)
        if scenario is None:
            continue
        # Last result line wins. Go emits one final --- PASS/FAIL per subtest; if a
        # retry (test.count>1) emits several, the LAST is the authoritative outcome, so a
        # recovered retry (PASS after an earlier FAIL) correctly clears the FAIL rather
        # than leaving it stuck.
        found[scenario] = verdict
    scenarios = {}
    for scenario in cl.SCENARIOS:
        if scenario in found:
            scenarios[scenario] = found[scenario]
        elif truncated:
            scenarios[scenario] = "UNKNOWN (log truncated)"
        else:
            scenarios[scenario] = "DID NOT RUN"
    return scenarios


def unavailable(token, reason):
    return {
        "token": token,
        "log_available": False,
        "unavailable_reason": reason,
        "release_image": None,
        "job_name": None,
        "log_truncated": False,
        "scenarios": {s: "VERDICT GC'd" for s in cl.SCENARIOS},
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--run", required=True, help="Run token")
    parser.add_argument("--out", required=True)
    args = parser.parse_args()

    out_dir = Path(args.out)
    out_dir.mkdir(parents=True, exist_ok=True)
    out_file = out_dir / "verdicts.json"

    token_nano = cl.token_unixnano(args.run)
    if token_nano is None:
        print(f"ERROR: run token '{args.run}' is not valid base36", file=sys.stderr)
        sys.exit(1)
    token_epoch = token_nano / 1e9

    try:
        candidates = find_job(token_epoch)
    except (RuntimeError, subprocess.SubprocessError, FileNotFoundError, ValueError) as e:
        # kubectl unavailable / no cluster creds — degrade gracefully, don't crash.
        result = unavailable(args.run, f"kubectl unavailable: {e}")
        out_file.write_text(json.dumps(result, indent=2))
        print(f"WARNING: {result['unavailable_reason']} — verdict unavailable", file=sys.stderr)
        return

    # Verify, don't guess: a candidate window can enclose the token by coincidence,
    # so confirm the Job's own log references this run's chains before trusting it.
    # No verified candidate -> VERDICT UNAVAILABLE; never bind to an unrelated nightly.
    verified = None
    saw_gcd_log = False
    for job in candidates:
        job_name = job["metadata"]["name"]
        log_text, truncated = read_log(job_name)
        if log_text is None:
            saw_gcd_log = True
            continue
        if log_references_run(log_text, args.run):
            verified = (job, job_name, log_text, truncated)
            break

    if verified is None:
        if not candidates:
            reason = f"no harness Job window contains the token (GC'd past {cl.LOG_RETENTION}, or wrong token)"
        elif saw_gcd_log:
            reason = (f"candidate Job present but pod logs GC'd past {cl.LOG_RETENTION} — "
                      f"cannot verify the token->Job join")
        else:
            reason = "no candidate Job log references this run's chains — refusing to bind to an unrelated nightly"
        result = unavailable(args.run, reason)
        out_file.write_text(json.dumps(result, indent=2))
        print(f"VERDICT UNAVAILABLE (no matching Job) — {reason}", file=sys.stderr)
        return

    job, job_name, log_text, truncated = verified
    image = extract_image(job)
    scenarios = build_verdicts(log_text, truncated)
    result = {
        "token": args.run,
        "log_available": True,
        "unavailable_reason": None,
        "release_image": image,
        "job_name": job_name,
        "job_creation_timestamp": job["metadata"].get("creationTimestamp"),
        "token_start_utc": datetime.fromtimestamp(token_epoch, tz=timezone.utc).isoformat(),
        "log_truncated": truncated,
        "scenarios": scenarios,
    }
    out_file.write_text(json.dumps(result, indent=2))

    for scenario, verdict in scenarios.items():
        print(f"  {scenario}: {verdict}", file=sys.stderr)
    if truncated:
        print(f"WARNING: log hit the {LOG_BYTE_CAP}-byte cap — unmatched scenarios marked UNKNOWN, "
              f"not DID NOT RUN", file=sys.stderr)
    print(f"\nRelease image: {image}  (Job {job_name})", file=sys.stderr)


if __name__ == "__main__":
    main()
