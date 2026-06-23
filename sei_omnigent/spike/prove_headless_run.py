#!/usr/bin/env python3
"""PLT-715 spike — the LIVE half: prove the budget driver bounds a real headless run.

The unit suite (``tests/test_omni_driver.py``) proves the driver's loop logic with a fake
session. This script runs the SAME driver against a REAL ``omnigent==0.2.0`` session to
close Design 12 Open-Q #2: that a budget-truncated ``/root-cause``-shaped run terminates
``incomplete``/``cancelled`` with a partial artifact — i.e. the client-side driver, not the
observer-only Stop hook, is the load-bearing terminator on the live runtime.

It needs an environment this repo's CI does not have, so it is a hand-run spike, not a test:
  * a running omnigent 0.2.0 server (``--server``), in header mode;
  * an ``ANTHROPIC_API_KEY`` reachable by the host/runner that executes the agent;
  * a gzipped agent bundle (``--bundle``) — e.g. omnigent's ``examples/polly/agents/claude_code``
    packaged as a tarball. You supply a known-good bundle so this script need not build one.

Usage (operator env):
    python spike/prove_headless_run.py \
        --server http://127.0.0.1:8443 \
        --bundle /path/to/agent.tar.gz \
        --goal "Investigate: error rate spiked on service X at 14:02 UTC; find the cause." \
        --wall-clock-s 20 --max-iterations 3

Expected: a TRUNCATED outcome (``budget-exhausted``, ``truncated=True``, ``cancelled=True``,
``tripped`` = the axis hit) with a non-empty partial artifact — the driver cancelled the loop
at the budget. A within-budget finish (``truncated=False``) is inconclusive (budget too loose) —
lower the caps and re-run.

VERIFY ON FIRST RUN (env-coupled — see ``omni/_omnigent_session.py``): the ``OmnigentClient``
construction + the auth header below is the most likely thing to need a tweak for your server.
"""

from __future__ import annotations

import argparse
import asyncio
import time

from omnigent_client import OmnigentClient
from omnigent_client._sessions_chat import SessionsChat

from sei_omnigent.omni._omnigent_session import GoalSession, make_extractors
from sei_omnigent.omni.driver import drive_to_terminal
from sei_omnigent.omni.engine import Budget


def _parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description="PLT-715 live prove-the-run spike")
    p.add_argument("--server", required=True, help="omnigent server base URL (header mode)")
    p.add_argument("--bundle", required=True, help="path to a gzipped agent tarball")
    p.add_argument("--goal", required=True, help="the investigation goal posted to the session")
    p.add_argument(
        "--forwarded-email",
        default="system:serviceaccount:walle:omni-root-cause",
        help="X-Forwarded-Email the header-mode server trusts (the sidecar sets this in "
        "prod; for a local spike server, whatever identity it admits)",
    )
    # Tight defaults so truncation is easy to demonstrate; loosen for a real investigation.
    p.add_argument("--wall-clock-s", type=float, default=20.0)
    p.add_argument("--tokens", type=int, default=200_000)
    p.add_argument("--queries", type=int, default=1_000)
    p.add_argument("--max-iterations", type=int, default=3)
    p.add_argument("--no-progress-iterations", type=int, default=1_000)
    return p.parse_args()


async def _run(args: argparse.Namespace) -> int:
    with open(args.bundle, "rb") as fh:
        bundle = fh.read()

    # VERIFY on first run: ctor kwargs + the trusted identity header. The header-mode server
    # resolves identity from X-Forwarded-Email (the sidecar sets it after TokenReview in prod).
    client = OmnigentClient(
        base_url=args.server,
        headers={"X-Forwarded-Email": args.forwarded_email},
    )
    try:
        # Verified classmethod form: SessionsChat.create(namespace, bundle, ...); namespace =
        # client.sessions. VERIFY on first run: the sessions accessor name on your client.
        chat = await SessionsChat.create(client.sessions, bundle)
        session = GoalSession(chat, args.goal)
        token_delta, is_iteration, artifact_chunk = make_extractors()
        budget = Budget(
            wall_clock_s=args.wall_clock_s,
            tokens=args.tokens,
            queries=args.queries,
            per_source_queries={},
            max_iterations=args.max_iterations,
            no_progress_iterations=args.no_progress_iterations,
        )
        outcome = await drive_to_terminal(
            session,
            budget,
            now=time.monotonic,
            token_delta=token_delta,
            is_iteration=is_iteration,
            artifact_chunk=artifact_chunk,
        )
    finally:
        await client.aclose()

    print("=" * 72)
    print(f"terminal_reason : {outcome.terminal_reason}")
    print(f"truncated       : {outcome.truncated}")
    print(f"tripped axis    : {outcome.tripped}")
    print(f"cancelled       : {outcome.cancelled}")
    print(f"iterations      : {outcome.iterations}")
    print(f"tokens          : {outcome.tokens}")
    print(f"elapsed_s       : {outcome.elapsed_s:.2f}")
    print("-" * 72)
    print("partial artifact (first 800 chars):")
    print(outcome.artifact[:800])
    print("=" * 72)

    # PASS = a tight budget truncated the run via cancel (the property under test). A
    # within-budget finish is inconclusive (loosen-and-rerun), not a failure.
    if outcome.truncated and outcome.cancelled:
        print("SPIKE RESULT: PASS — driver bounded a real headless run (budget -> cancel).")
        return 0
    print("SPIKE RESULT: INCONCLUSIVE — finished within budget; lower the caps and re-run.")
    return 2


def main() -> int:
    return asyncio.run(_run(_parse_args()))


if __name__ == "__main__":
    raise SystemExit(main())
