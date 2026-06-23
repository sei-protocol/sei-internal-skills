"""Launch-wrapper budget driver for the omni-overlay (PLT-715 spike) — the loop terminator.

The corrected reliability mechanism (Design 12 §2, prove-the-run spike Finding 1):
omnigent's injected ``Stop`` hook is **observer-only** on the pinned tag — it records
and returns 0; a blocking ``decision`` exists only on ``PreToolUse``/``PermissionRequest``,
never on ``Stop`` — so it **cannot halt the loop**. Budget/goal termination is therefore
enforced *client-side* by this driver: it watches usage over the session event stream,
evaluates :func:`engine.budget_terminal`, and on breach calls the session's
**cancel/interrupt** → an ``incomplete``/``cancelled`` terminal whose partial artifact is
rendered with the truncated ``terminal-reason`` (§3.5). The agent bundle's
``max_turns``/``max_iterations`` is the server-side backstop; this driver is the
load-bearing terminator.

This module is **omnigent-free and dependency-injected** (mirrors ``engine.py`` /
``profile.py``): it drives anything satisfying :class:`SessionLike` and reads usage via
injected extractors + an injected clock. That keeps the *loop logic* unit-testable with a
fake session (no live key / server) — the spike's logic half. The thin adapter binding the
real ``omnigent_client`` session (its ``stream()``/``cancel()``/``status`` + ``Response.usage``)
to :class:`SessionLike` is the live-run glue (Brandon's env); see :func:`drive_to_terminal`'s
contract.

Design 12 §2, §3.5.
"""

from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator, Callable
from contextlib import aclosing
from dataclasses import dataclass, field
from typing import Protocol

from .engine import Budget, TerminalReason, Usage, budget_terminal, is_truncated, tripped_axis

# Best-effort cancel must not itself hang the terminator: the cancel POSTs an interrupt to the
# same backend that may be the degraded dependency which caused the breach. The breach is already
# decided + the partial artifact stands, so a cancel that won't return in this bound is abandoned.
_CANCEL_TIMEOUT_S = 10.0


class SessionLike(Protocol):
    """The minimal session surface the driver drives.

    The real ``omnigent_client`` chat session satisfies this structurally
    (``stream()`` async-iterates server events, ``cancel()`` interrupts the run,
    ``status`` is the last-known session status). A test fake implements the same
    three members — so the driver's loop logic is provable without a live server.
    """

    def stream(self) -> AsyncIterator[object]: ...
    async def cancel(self) -> None: ...
    @property
    def status(self) -> str: ...


@dataclass(frozen=True)
class RunOutcome:
    """The terminal result the output renderer consumes (§3.5).

    ``truncated`` (from :func:`engine.is_truncated`) is load-bearing: a truncated run
    MUST NOT borrow the clean-punt "all-clear" headline, and ``tripped`` names the axis
    for the "investigation truncated at budget; unsurveyed: X" line. ``cancelled``
    records whether the driver issued the cancel (a budget breach) vs. the run ending
    on its own.
    """

    terminal_reason: TerminalReason
    truncated: bool
    tripped: str | None
    cancelled: bool
    elapsed_s: float
    tokens: int
    iterations: int
    artifact: str


# Extractors map one stream event onto usage deltas. Kept injectable so the loop is
# omnigent-free: the live adapter reads omnigent's ResponseEnd/usage; the test fakes them.
TokenDelta = Callable[[object], int]
"""event -> additional tokens consumed by this event (0 if it carries none)."""
IterationBoundary = Callable[[object], bool]
"""event -> True if this event closes an agent turn/response (an iteration)."""
ProgressSignal = Callable[[object], bool]
"""event -> True if this event represents NEW progress (resets the no-progress window).
Default is "every iteration is progress" — the no-progress axis only bites when the caller
wires a real progress detector (new evidence / hypothesis movement, §2); without one it
must not fire, so the honest default never starves a working run."""
ArtifactChunk = Callable[[object], str]
"""event -> any partial-artifact text this event contributes ("" if none)."""


@dataclass
class _Acc:
    """Mutable accumulation across the stream (the snapshot the engine reads each tick)."""

    tokens: int = 0
    iterations: int = 0
    iterations_since_progress: int = 0
    artifact_parts: list[str] = field(default_factory=list)


def _snapshot(acc: _Acc, elapsed_s: float) -> Usage:
    # queries=0: query-budget accounting belongs to the read toolkit (§3.4), not the loop
    # driver; engine.Usage validates the snapshot finite + non-negative.
    return Usage(
        elapsed_s=elapsed_s,
        tokens=acc.tokens,
        queries=0,
        per_source_queries={},
        iterations=acc.iterations,
        iterations_since_progress=acc.iterations_since_progress,
    )


async def _best_effort_cancel(session: SessionLike) -> None:
    """Cancel the session, time-bounded + swallowing — the breach is already decided.

    Bounded by ``_CANCEL_TIMEOUT_S`` because the cancel POSTs an interrupt to the same backend
    that may be the degraded dependency behind the breach; a cancel that raises OR hangs must
    not mask the truncation we already decided (the partial artifact stands).
    """
    try:
        async with asyncio.timeout(_CANCEL_TIMEOUT_S):
            await session.cancel()
    except Exception:
        pass


async def drive_to_terminal(
    session: SessionLike,
    budget: Budget,
    *,
    now: Callable[[], float],
    token_delta: TokenDelta,
    is_iteration: IterationBoundary,
    is_progress: ProgressSignal = lambda _event: True,
    artifact_chunk: ArtifactChunk = lambda _event: "",
    on_natural_end: TerminalReason = TerminalReason.GOAL_REACHED,
    watchdog_grace_s: float = 15.0,
) -> RunOutcome:
    """Drive a session to a terminal, enforcing ``budget`` by cancelling on breach.

    The loop: stream events; per event, accumulate usage and evaluate
    :func:`engine.budget_terminal` against a fresh snapshot; on the FIRST breach,
    **cancel the session** and return a truncated :class:`RunOutcome` carrying the
    tripped axis + the partial artifact assembled so far. If the stream ends without a
    breach, the run terminated on its own → ``on_natural_end`` (a *surveyed* terminal;
    the caller maps the real goal-vs-clean-punt decision from the agent's own report —
    that is the agent's predicate, not the driver's).

    Why cancel-on-breach and not a Stop hook: the Stop hook is observer-only on the
    pinned tag (module docstring) — this driver is the only thing that can actually stop
    the loop, which is why the budget check runs *before* awaiting the next event and the
    cancel is issued the instant any axis hits.

    **Two termination paths, both load-bearing:**

    * *Per-event budget axis* — ``now`` (injected monotonic seconds; testable without
      sleeping) is read once per event into the snapshot, so the budget axes are observed
      at every event boundary.
    * *Wall-clock watchdog* — the per-event check cannot fire between events, so a turn
      that hangs SILENTLY (the SSE stream open but emitting nothing) would never trip it.
      An ``asyncio.timeout(budget.wall_clock_s + watchdog_grace_s)`` (REAL event-loop time,
      independent of injected ``now``) makes ``wall_clock_s`` a hard deadline even with no
      events; firing surfaces as the fail-closed truncated outcome below. ``watchdog_grace_s``
      covers per-event check lag; lower it in tests to exercise the watchdog quickly.

    **Resource lifecycle:** the stream is driven under :func:`contextlib.aclosing`, so the
    underlying SSE async generator is ``aclose()``d on EVERY exit (breach-return, error,
    natural end) — the real ``send()`` generator releases its httpx pool slot only on close,
    and ``cancel()`` (a separate server-interrupt POST) does NOT close the client stream.

    **Concurrency contract (for the receiver/host reusing this):** one ``drive_to_terminal``
    per session (``GoalSession`` opens one SSE subscription per drive) and a FRESH extractor
    triple per call (``make_extractors``' token closure is per-run state — sharing it across
    concurrent drives under-counts tokens, failing the token axis OPEN).
    """
    acc = _Acc()
    start = now()

    try:
        # Wall-clock watchdog (see docstring) + aclosing so the SSE generator is always closed.
        async with asyncio.timeout(budget.wall_clock_s + watchdog_grace_s):
            async with aclosing(session.stream()) as stream:
                async for event in stream:
                    chunk = artifact_chunk(event)
                    if chunk:
                        acc.artifact_parts.append(chunk)
                    acc.tokens += token_delta(event)
                    if is_iteration(event):
                        acc.iterations += 1
                        acc.iterations_since_progress = (
                            0 if is_progress(event) else acc.iterations_since_progress + 1
                        )

                    snapshot = _snapshot(acc, now() - start)
                    terminal = budget_terminal(budget, snapshot)
                    if terminal is not None:
                        # Load-bearing: stop the loop the Stop hook can't. The breach is the
                        # truth + the partial artifact stands, so the cancel is best-effort AND
                        # time-bounded (must not raise OR hang the terminator — _CANCEL_TIMEOUT_S).
                        await _best_effort_cancel(session)
                        return RunOutcome(
                            terminal_reason=terminal,
                            truncated=is_truncated(terminal),
                            tripped=tripped_axis(budget, snapshot),
                            cancelled=True,
                            elapsed_s=snapshot.elapsed_s,
                            tokens=acc.tokens,
                            iterations=acc.iterations,
                            artifact="".join(acc.artifact_parts),
                        )
    except Exception:
        # A stream/transport failure OR the wall-clock watchdog firing mid-run is a truncated
        # run, not a clean punt: we surveyed only part of the space. Fail-closed onto the
        # truncated headline so the renderer never presents a partial as all-clear (§3.5).
        # (Receiver carry-forward: add a distinct FAILED terminal so a transport error reads
        # differently from a budget cut — the enum has no errored-terminal today.)
        snapshot = _snapshot(acc, now() - start)
        return RunOutcome(
            terminal_reason=TerminalReason.BUDGET_EXHAUSTED,
            truncated=True,
            tripped=tripped_axis(budget, snapshot),
            cancelled=False,
            elapsed_s=snapshot.elapsed_s,
            tokens=acc.tokens,
            iterations=acc.iterations,
            artifact="".join(acc.artifact_parts),
        )

    # Stream ended within budget → the run reached its own terminal.
    return RunOutcome(
        terminal_reason=on_natural_end,
        truncated=is_truncated(on_natural_end),
        tripped=None,
        cancelled=False,
        elapsed_s=now() - start,
        tokens=acc.tokens,
        iterations=acc.iterations,
        artifact="".join(acc.artifact_parts),
    )
