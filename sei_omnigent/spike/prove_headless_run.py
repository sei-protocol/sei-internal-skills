#!/usr/bin/env python3
"""PLT-715 prove-the-run — the LIVE half: prove the budget driver bounds a real headless run,
AND probe the auth-principal Blocking dependency. Hand-run in the operator's env (not CI).

The unit suite (``tests/test_omni_driver.py``) proves the driver's loop logic with a fake
session. This script runs the SAME merged driver against a REAL ``omnigent==0.2.0`` session.

It answers two things:

  PURPOSE 1 (foundational — Design 12/13 Open-Q #2): does a budget-truncated ``/root-cause``-
  shaped run terminate TRUNCATED (``budget-exhausted``, ``cancelled=True``) with a partial
  artifact — i.e. the client-side driver, not the observer-only Stop hook, is the load-bearing
  terminator on the live runtime? Answerable against ANY runnable 0.2.0 server.

  PURPOSE 2 (the Design-13 Blocking dependency): what PRINCIPAL does the server resolve the
  session under? It prints the server-recorded ``owner``. This is load-bearing for the pivot's
  PDP/owner model: if, behind the production kube-rbac-proxy sidecar, the resolved owner is the
  *SA name* (``system:serviceaccount:walle:omni-trigger``) rather than the forwarded human
  email, the per-human-initiator PDP/owner design needs the in-band-initiator rework Design 13
  flags. CAVEAT: a LOCAL spike server has no sidecar, so it trusts the ``X-Forwarded-Email`` we
  send verbatim — Purpose 2 is only conclusive against the SIDECAR-FRONTED deployed server (or a
  kube-rbac-proxy config audit). Against a local server this just confirms the header round-trips.

Environment it needs (why it is a hand-run spike, not a test):
  * a running omnigent 0.2.0 server (``--server``) in header mode;
  * an ``ANTHROPIC_API_KEY`` reachable by the host/runner that executes the agent;
  * a gzipped agent bundle (``--bundle``) — e.g. omnigent's ``examples/polly/agents/claude_code``
    packaged as a tarball. You supply a known-good bundle so this script need not build one.

Usage (operator env):
    python spike/prove_headless_run.py \
        --server http://127.0.0.1:8443 \
        --bundle /path/to/agent.tar.gz \
        --goal "Investigate: error rate spiked on service X at 14:02 UTC; find the cause." \
        --wall-clock-s 20 --max-iterations 3

PASS = a tight budget truncated the run via cancel (``truncated=True`` + ``cancelled=True``) with
a non-empty partial artifact. A within-budget finish (``truncated=False``) is INCONCLUSIVE for
Purpose 1 (budget too loose) — lower the caps and re-run; it is not a failure. A connect/auth
failure surfaces as ``terminal_reason=ERRORED`` (the driver classifies it honestly) — that is a
FAIL of Purpose 1 with the reason in the artifact.

VERIFY ON FIRST RUN: the ``OmnigentClient`` construction + the identity below is the most likely
thing to need a tweak for your server (see ``omni/_omnigent_session.py``). This spike uses a
direct ``X-Forwarded-Email`` header (the simple local-server path); the production receiver uses
``_FreshBearerAuth`` (a per-request SA bearer the sidecar TokenReviews) — see the handoff doc.
"""

from __future__ import annotations

import argparse
import asyncio
import time

from omnigent_client import OmnigentClient
from omnigent_client._sessions_chat import SessionsChat

from sei_omnigent.omni._omnigent_session import GoalSession, make_extractors
from sei_omnigent.omni.driver import drive_to_terminal
from sei_omnigent.omni.engine import Budget, TerminalReason


def _parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description="PLT-715 live prove-the-run spike")
    p.add_argument("--server", required=True, help="omnigent server base URL (header mode)")
    p.add_argument("--bundle", required=True, help="path to a gzipped agent tarball")
    p.add_argument("--goal", required=True, help="the investigation goal posted to the session")
    p.add_argument(
        "--forwarded-email",
        default="walle@seinetwork.io",
        help="the X-Forwarded-Email identity sent to a header-mode server. NOTE: behind the "
        "production sidecar this is OVERWRITTEN by the TokenReview'd SA — the printed 'resolved "
        "owner' is what actually decides the Blocking dependency (Purpose 2).",
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

    # VERIFY on first run: ctor kwargs + the identity. A header-mode server resolves identity from
    # X-Forwarded-Email; behind the sidecar the sidecar sets it from the TokenReview'd SA.
    client = OmnigentClient(
        base_url=args.server,
        headers={"X-Forwarded-Email": args.forwarded_email},
    )
    session_id: str | None = None
    resolved_owner = "<unread>"
    try:
        # Verified 0.2.0 form: SessionsChat.create(namespace, bundle); namespace = client.sessions.
        chat = await SessionsChat.create(client.sessions, bundle)
        session_id = getattr(chat, "session_id", None)

        # PURPOSE 2 probe — what principal did the SERVER resolve? Best-effort.
        try:
            if session_id is not None:
                info = await client.sessions.get(session_id)
                resolved_owner = getattr(info, "owner", None) or "<no owner field on snapshot>"
        except Exception as exc:  # the probe must never fail Purpose 1
            resolved_owner = f"<unavailable: {type(exc).__name__}>"

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
        await client.close()  # 0.2.0: OmnigentClient exposes close() (NOT aclose).

    print("=" * 72)
    print(f"session_id      : {session_id}")
    print(f"terminal_reason : {outcome.terminal_reason}")
    print(f"truncated       : {outcome.truncated}")
    print(f"tripped axis    : {outcome.tripped}")
    print(f"cancelled       : {outcome.cancelled}")
    print(f"iterations      : {outcome.iterations}")
    print(f"tokens          : {outcome.tokens}")
    print(f"elapsed_s       : {outcome.elapsed_s:.2f}")
    print("-" * 72)
    print("PURPOSE 2 — auth principal (the Blocking dependency):")
    print(f"  sent X-Forwarded-Email : {args.forwarded_email}")
    print(f"  server-resolved owner  : {resolved_owner}")
    print("  -> if (behind the sidecar) the resolved owner is the SA name, not the sent email,")
    print("     the per-human PDP/owner model needs the in-band-initiator rework (Design 13).")
    print("-" * 72)
    print("partial artifact (first 800 chars):")
    print(outcome.artifact[:800])
    print("=" * 72)

    # Purpose 1 PASS = a tight budget truncated the run via cancel. ERRORED = connect/auth/run
    # failure (FAIL). A within-budget finish = INCONCLUSIVE (loosen-and-rerun), not a failure.
    if outcome.terminal_reason is TerminalReason.ERRORED:
        print("SPIKE RESULT: FAIL (Purpose 1) — run ERRORED before completing; see artifact.")
        return 1
    if outcome.truncated and outcome.cancelled:
        print("SPIKE RESULT: PASS (Purpose 1) — driver bounded a real run (budget -> cancel).")
        print("              Read the 'server-resolved owner' above for Purpose 2.")
        return 0
    print("SPIKE RESULT: INCONCLUSIVE — finished within budget; lower the caps and re-run.")
    return 2


def main() -> int:
    return asyncio.run(_run(_parse_args()))


if __name__ == "__main__":
    raise SystemExit(main())
