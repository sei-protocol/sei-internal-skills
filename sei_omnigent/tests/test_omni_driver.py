"""Spike unit test for the budget driver (PLT-715) — proves the loop terminator works.

The prove-the-run spike's *logic half*: with no live key/server, drive a fake session
through :func:`omni.driver.drive_to_terminal` and assert that each budget axis trips →
the driver issues ``cancel()`` → it returns a *truncated* outcome carrying the tripped
axis + the partial artifact. This is what proves the corrected mechanism (a client-side
driver, not the observer-only Stop hook, is the load-bearing terminator). The live
end-to-end run swaps the fake for the real omnigent session (operator env).

Async tests run via ``asyncio.run`` so no ``pytest-asyncio`` dep is needed (the suite
stays on ``pytest`` + ``ruff`` only).
"""

from __future__ import annotations

import asyncio
from collections.abc import Callable

import httpx

from sei_omnigent.omni.driver import drive_to_terminal
from sei_omnigent.omni.engine import Budget, TerminalReason


class _FakeSession:
    """A session that emits a fixed event list and never self-terminates.

    Models the failure mode the driver exists to bound: an agent loop that would run
    forever if nothing stopped it. Records ``cancel()`` calls so a test can assert the
    driver actually pulled the plug.
    """

    def __init__(self, events: list[dict]) -> None:
        self._events = events
        self.cancel_calls = 0

    async def stream(self):
        for event in self._events:
            yield event

    async def cancel(self) -> None:
        self.cancel_calls += 1

    @property
    def status(self) -> str:
        return "cancelled" if self.cancel_calls else "running"


def _fixed_clock(value: float = 0.0) -> Callable[[], float]:
    return lambda: value


def _advancing_clock(values: list[float]) -> Callable[[], float]:
    """Return successive values per call (then hold the last) — simulates wall-clock."""
    it = iter(values)
    last = [values[0]]

    def _now() -> float:
        try:
            last[0] = next(it)
        except StopIteration:
            pass
        return last[0]

    return _now


# Extractors over the fake event shape {"tokens": int, "iter": bool, "text": str}.
def _tokens(e: object) -> int:
    return int(e.get("tokens", 0))  # type: ignore[union-attr]


def _is_iter(e: object) -> bool:
    return bool(e.get("iter", False))  # type: ignore[union-attr]


def _chunk(e: object) -> str:
    return str(e.get("text", ""))  # type: ignore[union-attr]


def _budget(**overrides) -> Budget:
    base = dict(
        wall_clock_s=1_000.0,
        tokens=1_000_000,
        queries=1_000,
        per_source_queries={},
        max_iterations=1_000,
        no_progress_iterations=1_000,
    )
    base.update(overrides)
    return Budget(**base)  # type: ignore[arg-type]


def test_max_iterations_trips_and_cancels() -> None:
    """A run past the iteration ceiling is cancelled + truncated on the 'iterations' axis."""
    session = _FakeSession([{"iter": True, "tokens": 10, "text": f"step{i} "} for i in range(10)])
    outcome = asyncio.run(
        drive_to_terminal(
            session,
            _budget(max_iterations=3),
            now=_fixed_clock(),  # no wall-clock pressure
            token_delta=_tokens,
            is_iteration=_is_iter,
            artifact_chunk=_chunk,
        )
    )
    assert outcome.terminal_reason is TerminalReason.BUDGET_EXHAUSTED
    assert outcome.truncated is True
    assert outcome.tripped == "iterations"
    assert outcome.cancelled is True
    assert outcome.iterations == 3  # stopped AT the ceiling, did not run all 10
    assert session.cancel_calls == 1
    assert outcome.artifact == "step0 step1 step2 "  # partial artifact assembled to the cut


def test_wall_clock_trips_and_cancels() -> None:
    """Wall-clock is observed at event boundaries via the injected clock; trips → cancel."""
    session = _FakeSession([{"iter": True, "tokens": 1} for _ in range(10)])
    # start=0; per-event elapsed = 2,4,6,... → crosses 5 at the 3rd event.
    outcome = asyncio.run(
        drive_to_terminal(
            session,
            _budget(wall_clock_s=5.0, max_iterations=1_000),
            now=_advancing_clock([0.0, 2.0, 4.0, 6.0, 8.0]),
            token_delta=_tokens,
            is_iteration=_is_iter,
        )
    )
    assert outcome.terminal_reason is TerminalReason.BUDGET_EXHAUSTED
    assert outcome.truncated is True
    assert outcome.tripped == "wall-clock"
    assert outcome.cancelled is True
    assert session.cancel_calls == 1
    assert outcome.elapsed_s >= 5.0


def test_token_axis_trips() -> None:
    """The token ceiling trips before iterations when tokens accrue fast."""
    session = _FakeSession([{"iter": True, "tokens": 40} for _ in range(10)])
    outcome = asyncio.run(
        drive_to_terminal(
            session,
            _budget(tokens=100, max_iterations=1_000),
            now=_fixed_clock(),
            token_delta=_tokens,
            is_iteration=_is_iter,
        )
    )
    assert outcome.terminal_reason is TerminalReason.BUDGET_EXHAUSTED
    assert outcome.tripped == "tokens"
    assert outcome.tokens >= 100
    assert session.cancel_calls == 1


def test_natural_end_within_budget_is_not_truncated() -> None:
    """A stream that ends within budget → a surveyed terminal, no cancel, not truncated."""
    session = _FakeSession(
        [{"iter": True, "tokens": 5, "text": "a"}, {"iter": True, "tokens": 5, "text": "b"}]
    )
    outcome = asyncio.run(
        drive_to_terminal(
            session,
            _budget(),  # generous
            now=_fixed_clock(),
            token_delta=_tokens,
            is_iteration=_is_iter,
            artifact_chunk=_chunk,
            on_natural_end=TerminalReason.CLEAN_PUNT,
        )
    )
    assert outcome.terminal_reason is TerminalReason.CLEAN_PUNT
    assert outcome.truncated is False
    assert outcome.tripped is None
    assert outcome.cancelled is False
    assert session.cancel_calls == 0
    assert outcome.iterations == 2
    assert outcome.artifact == "ab"


def test_no_progress_default_does_not_fire() -> None:
    """With the honest default (every iteration = progress), the no-progress axis never bites.

    Guards against the driver starving a working run: a low no_progress_iterations cap must
    NOT trip when the caller wires no real progress detector (the default resets the window
    each iteration).
    """
    session = _FakeSession([{"iter": True, "tokens": 1} for _ in range(20)])
    outcome = asyncio.run(
        drive_to_terminal(
            session,
            _budget(no_progress_iterations=2, max_iterations=1_000),
            now=_fixed_clock(),
            token_delta=_tokens,
            is_iteration=_is_iter,
        )
    )
    # Ran the whole stream to natural end — no-progress never tripped.
    assert outcome.terminal_reason is TerminalReason.GOAL_REACHED
    assert outcome.truncated is False
    assert session.cancel_calls == 0


def test_cancel_that_raises_does_not_mask_truncation() -> None:
    """A cancel() that itself raises must not lose the truncated outcome (best-effort swallow).

    Guards driver.py's documented invariant ("a cancel that itself errors must not mask the
    truncation we already decided") — the branch most likely to regress if someone tightens
    the except. The budget breach is the truth; the partial outcome stands, cancelled=True.
    """

    class _RaisingCancelSession(_FakeSession):
        async def cancel(self) -> None:
            self.cancel_calls += 1
            raise RuntimeError("cancel failed")

    session = _RaisingCancelSession([{"iter": True, "tokens": 1} for _ in range(10)])
    outcome = asyncio.run(
        drive_to_terminal(
            session,
            _budget(max_iterations=2),
            now=_fixed_clock(),
            token_delta=_tokens,
            is_iteration=_is_iter,
        )
    )
    assert outcome.terminal_reason is TerminalReason.BUDGET_EXHAUSTED
    assert outcome.truncated is True
    assert outcome.cancelled is True  # cancel was attempted; the breach stands despite it raising
    assert session.cancel_calls == 1


def test_stream_failure_is_errored_not_budget() -> None:
    """A mid-run stream error is an ERRORED terminal (not BUDGET_EXHAUSTED) and stays truncated.

    A transport drop / create reject is NOT a budget breach: it must not borrow the budget axis
    (tripped is None) nor the all-clear headline (truncated stays True), and the failure detail
    rides in the artifact so the rendered note is an honest "could not run".
    """

    class _BoomSession(_FakeSession):
        async def stream(self):
            yield {"iter": True, "tokens": 1, "text": "partial"}
            raise RuntimeError("transport dropped")

    session = _BoomSession([])
    outcome = asyncio.run(
        drive_to_terminal(
            session,
            _budget(),
            now=_fixed_clock(),
            token_delta=_tokens,
            is_iteration=_is_iter,
            artifact_chunk=_chunk,
        )
    )
    assert outcome.terminal_reason is TerminalReason.ERRORED
    assert outcome.terminal_reason is not TerminalReason.BUDGET_EXHAUSTED
    assert outcome.truncated is True
    assert outcome.tripped is None  # no budget axis was hit — the run errored
    assert outcome.cancelled is False  # the stream died; we didn't cancel
    assert "partial" in outcome.artifact  # what we had at the drop still stands
    assert "transport dropped" in outcome.artifact  # the failure detail is captured


def test_first_iteration_raise_is_errored_not_budget() -> None:
    """A stream whose FIRST iteration raises (a SessionsChat.create reject) → ERRORED.

    This is the B1 path: a server-down / TokenReview-reject / bad-bundle failure surfaces inside
    the stream loop and MUST be classified ERRORED, never laundered into BUDGET_EXHAUSTED.
    """

    class _CreateRejectSession(_FakeSession):
        async def stream(self):
            raise RuntimeError("SessionsChat.create rejected: 401 TokenReview")
            if False:  # pragma: no cover — makes this an async generator
                yield {}

    session = _CreateRejectSession([])
    outcome = asyncio.run(
        drive_to_terminal(
            session,
            _budget(),
            now=_fixed_clock(),
            token_delta=_tokens,
            is_iteration=_is_iter,
            artifact_chunk=_chunk,
        )
    )
    assert outcome.terminal_reason is TerminalReason.ERRORED
    assert outcome.terminal_reason is not TerminalReason.BUDGET_EXHAUSTED
    assert outcome.truncated is True
    assert outcome.tripped is None
    assert outcome.iterations == 0  # nothing ran
    assert "TokenReview" in outcome.artifact  # the create-reject detail is captured


def test_wall_clock_watchdog_is_budget_exhausted_not_errored() -> None:
    """A silently hung turn → the watchdog fires → BUDGET_EXHAUSTED on the wall-clock axis.

    Regression guard (Bugbot #4): the asyncio.timeout watchdog raises builtins TimeoutError on
    expiry, which the prior blanket ``except Exception`` MIS-classified as ERRORED. The watchdog
    firing IS a wall-clock BUDGET breach (a turn hung past wall_clock_s + grace with no events for
    the per-event axis to catch), so it MUST be BUDGET_EXHAUSTED with the wall-clock axis tripped
    and a best-effort cancel issued — NOT ERRORED (which would misdirect the on-call to a transport
    fault). The outer wait_for is a safety net: a broken watchdog fails this fast, never hangs.
    """

    class _HangingSession(_FakeSession):
        async def stream(self):
            # An async generator that yields nothing and then blocks forever.
            if False:  # pragma: no cover - makes this a generator without emitting an event
                yield {}
            await asyncio.Event().wait()

    session = _HangingSession([])

    async def _drive() -> object:
        return await drive_to_terminal(
            session,
            _budget(wall_clock_s=0.05),
            now=_fixed_clock(),  # per-event axis never fires (no events) — the watchdog must
            token_delta=_tokens,
            is_iteration=_is_iter,
            watchdog_grace_s=0.05,  # fire at ~0.1s real time
        )

    outcome = asyncio.run(asyncio.wait_for(_drive(), timeout=2.0))
    assert outcome.terminal_reason is TerminalReason.BUDGET_EXHAUSTED
    assert outcome.terminal_reason is not TerminalReason.ERRORED  # the regression this guards
    assert outcome.tripped == "wall-clock"  # the watchdog firing IS the wall-clock breach
    assert outcome.truncated is True
    assert outcome.cancelled is True  # best-effort cancel issued (mirrors the in-loop breach)
    assert session.cancel_calls == 1
    assert outcome.elapsed_s >= 0.05  # clamped to wall_clock_s so the wall-clock axis resolves


def test_httpx_transport_timeout_stays_errored_not_budget() -> None:
    """An httpx.TimeoutException is NOT builtins TimeoutError → it stays ERRORED, not budget.

    The watchdog's ``except TimeoutError`` must catch ONLY the asyncio watchdog. A transport
    timeout (httpx.TimeoutException) is a dependency fault, not a budget cut — it must fall to the
    blanket ``except Exception`` and classify ERRORED, or the on-call is misdirected to wall-clock.
    """

    class _TransportTimeoutSession(_FakeSession):
        async def stream(self):
            yield {"iter": True, "tokens": 1, "text": "partial"}
            raise httpx.ReadTimeout("transport read timed out")

    session = _TransportTimeoutSession([])
    outcome = asyncio.run(
        drive_to_terminal(
            session,
            _budget(),  # generous — no budget axis is near
            now=_fixed_clock(),
            token_delta=_tokens,
            is_iteration=_is_iter,
            artifact_chunk=_chunk,
        )
    )
    assert outcome.terminal_reason is TerminalReason.ERRORED
    assert outcome.terminal_reason is not TerminalReason.BUDGET_EXHAUSTED
    assert outcome.tripped is None  # no budget axis hit — it errored
    assert "transport read timed out" in outcome.artifact


def test_inner_timeouterror_stays_errored_not_budget() -> None:
    """A builtins TimeoutError raised from WITHIN the stream (an inner per-op deadline) → ERRORED.

    Regression guard (Bugbot, on the #4 fix): the watchdog branch must gate on
    ``watchdog.expired()``, NOT a bare ``except TimeoutError`` — else an inner TimeoutError (the
    session/client's own per-operation deadline, not the wall-clock watchdog) is mislabeled a
    BUDGET cut and misdirects the on-call to the wall-clock axis. The watchdog has NOT expired
    here (the budget is generous, the raise is immediate), so this is a genuine run error.
    """

    class _InnerTimeoutSession(_FakeSession):
        async def stream(self):
            yield {"iter": True, "tokens": 1, "text": "partial"}
            raise TimeoutError("inner per-operation deadline")  # builtins, NOT the watchdog

    session = _InnerTimeoutSession([])
    outcome = asyncio.run(
        drive_to_terminal(
            session,
            _budget(),  # generous — the wall-clock watchdog is nowhere near
            now=_fixed_clock(),
            token_delta=_tokens,
            is_iteration=_is_iter,
            artifact_chunk=_chunk,
        )
    )
    assert outcome.terminal_reason is TerminalReason.ERRORED  # NOT budget (watchdog didn't fire)
    assert outcome.tripped is None
    assert "inner per-operation deadline" in outcome.artifact
