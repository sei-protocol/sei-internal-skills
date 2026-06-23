"""Live-run glue binding the real ``omnigent_client`` session to the budget driver (PLT-715).

IMPURE — this is the one omni/ module that imports ``omnigent_client`` (the live path).
It is deliberately NOT re-exported by ``omni/__init__``: the pure driver (``driver.py``)
and its unit tests never import this, so the unit suite needs no omnigent install. Import
it only on the live run.

It supplies what :func:`omni.driver.drive_to_terminal` needs to drive a *real* session:

* :func:`make_extractors` — the ``(token_delta, is_iteration, artifact_chunk)`` triple,
  reading omnigent 0.2.0's typed stream events. The wire carries **cumulative**
  ``Response.usage.total_tokens`` on each turn-terminal event, so ``token_delta`` diffs
  against a running max (the driver accumulates additively).
* :class:`GoalSession` — wraps a :class:`SessionsChat` so it satisfies
  :class:`omni.driver.SessionLike` *and* posts the goal: its ``stream()`` calls
  ``chat.send(goal)`` (post-the-goal-then-stream-the-turn), ``cancel()`` →
  ``chat.cancel()``, ``status`` → ``chat.status``.

SPIKE-GRADE IMPORT NOTE: the turn-terminal event classes are not on the public
``omnigent_client`` surface, so they are imported from the private ``_sessions_chat``
module here, in ONE place, with this marker. Re-confirm on the next omnigent pin bump
(same discipline as ``_omnigent_shim``). The production receiver should prefer the public
``StreamHooks`` lifecycle (``on_response_end``) over private event-type imports.

VERIFY ON FIRST LIVE RUN (env-coupled; not exercised by the unit suite):
  - whether a multi-turn ``/root-cause`` loop surfaces as repeated ``send()`` turns or one
    long turn (the extractors handle both — ``is_iteration`` counts turn-terminal events);
  - the ``OmnigentClient`` construction + the ``omni-root-cause``/``omnigent-api`` auth header;
  - that ``Response.usage`` is populated on terminal events (it is ``Usage | None`` → 0 delta
    when absent, which is safe — the run is then bounded by wall-clock/iterations, not tokens).

Design 12 §2, §3.2.
"""

from __future__ import annotations

from collections.abc import AsyncIterator, Callable

# --- spike-grade private import (see module docstring); re-confirm on omnigent bump ---
from omnigent_client._sessions_chat import (
    _TURN_TERMINAL_EVENT_TYPES,
    OutputTextDeltaEvent,
)


def make_extractors() -> tuple[
    Callable[[object], int],
    Callable[[object], bool],
    Callable[[object], str],
]:
    """Return ``(token_delta, is_iteration, artifact_chunk)`` for :func:`drive_to_terminal`.

    ``token_delta`` is stateful (a closure over the running-max cumulative total): the wire
    reports cumulative ``Response.usage.total_tokens`` per turn-terminal event, so the
    per-event increment is ``max(0, cumulative_now - cumulative_seen)``. Build a fresh
    triple per run (the closure must not leak token state across runs).
    """
    seen_total = 0

    def token_delta(event: object) -> int:
        nonlocal seen_total
        if isinstance(event, _TURN_TERMINAL_EVENT_TYPES):
            usage = getattr(event.response, "usage", None)  # type: ignore[attr-defined]
            total = getattr(usage, "total_tokens", 0) or 0
            delta = max(0, int(total) - seen_total)
            seen_total = max(seen_total, int(total))
            return delta
        return 0

    def is_iteration(event: object) -> bool:
        # A turn-terminal event (Completed/Failed/Incomplete/Cancelled) closes one turn.
        return isinstance(event, _TURN_TERMINAL_EVENT_TYPES)

    def artifact_chunk(event: object) -> str:
        return event.delta if isinstance(event, OutputTextDeltaEvent) else ""

    return token_delta, is_iteration, artifact_chunk


class GoalSession:
    """Adapt a ``SessionsChat`` to :class:`omni.driver.SessionLike`, posting ``goal`` on stream.

    ``stream()`` posts the goal and yields the turn's events (``chat.send(goal)``); the driver
    drives that stream. ``cancel()``/``status`` delegate. A ``SessionsChat`` already exposes
    ``cancel()`` + ``status``; this wrapper's only addition is turning the bare subscription
    into a goal-posting one so the agent actually does work for the budget to bound.
    """

    def __init__(self, chat: object, goal: str) -> None:
        self._chat = chat
        self._goal = goal

    def stream(self) -> AsyncIterator[object]:
        return self._chat.send(self._goal)  # type: ignore[attr-defined]

    async def cancel(self) -> None:
        await self._chat.cancel()  # type: ignore[attr-defined]

    @property
    def status(self) -> str:
        return self._chat.status  # type: ignore[attr-defined]
