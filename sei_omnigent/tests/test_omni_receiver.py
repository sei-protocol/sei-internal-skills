"""Tests for the omni-trigger receiver edge + supervisor (PLT-715).

The handler is framework-free (takes the raw Authorization header + decoded body, returns
``(status, body)``), so the edge flow is provable without FastAPI/HTTP. ``supervise_run`` and
``post_back`` are driven against a fake GoalSession + a recording poster + an injected
dedup store, so the full lifecycle (drive_to_terminal → post the partial on a truncated
terminal; post_back idempotency) is provable without a live omnigent / PagerDuty.

Async tests run via ``asyncio.run`` (no pytest-asyncio dep — the suite stays pytest+ruff).
"""

from __future__ import annotations

import asyncio

import pytest

from sei_omnigent.omni._dedup import InMemoryDedupStore
from sei_omnigent.omni.driver import RunOutcome
from sei_omnigent.omni.engine import Budget, TerminalReason
from sei_omnigent.omni.receiver import (
    _MAX_BODY_BYTES,
    Receiver,
    ReceiverConfig,
    RunContext,
    build_app,
    post_back,
    render_note,
    supervise_run,
    verify_bearer,
)

_TOKEN = "s3cr3t-shared-bearer"


def _budget() -> Budget:
    return Budget(
        wall_clock_s=1_000.0,
        tokens=1_000_000,
        queries=1_000,
        per_source_queries={},
        max_iterations=1_000,
        no_progress_iterations=1_000,
    )


def _body(**over: object) -> dict:
    base: dict = {
        "version": "4",
        "groupKey": '{}:{alertname="ChainHalted"}',
        "status": "firing",
        "commonLabels": {"walle": "enabled", "severity": "critical", "namespace": "sei"},
        "alerts": [
            {
                "status": "firing",
                "labels": {"alertname": "ChainHalted", "chain_id": "pacific-1"},
                "annotations": {"summary": "stalled", "runbook_url": "https://rb/halt"},
                "startsAt": "2026-06-23T10:00:00Z",
            }
        ],
    }
    base.update(over)
    return base


class _FakeSession:
    """A session emitting a fixed event list and never self-terminating (mirrors driver test)."""

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


def _extractors():
    return (
        lambda e: int(e.get("tokens", 0)),  # token_delta
        lambda e: bool(e.get("iter", False)),  # is_iteration
        lambda e: str(e.get("text", "")),  # artifact_chunk
    )


class _RecordingPoster:
    def __init__(self) -> None:
        self.notes: list[tuple[str, str]] = []

    async def post_note(self, incident_key: str, note: str) -> None:
        self.notes.append((incident_key, note))


def _receiver(
    session_factory, *, dedup=None, poster=None, max_in_flight=16, redact=None
) -> Receiver:
    return Receiver(
        config=ReceiverConfig(
            budget=_budget(), lease_s=1_100.0, max_in_flight=max_in_flight, now=lambda: 0.0
        ),
        dedup=dedup if dedup is not None else InMemoryDedupStore(now=lambda: 0.0),
        session_factory=session_factory,
        expected_token=_TOKEN,
        poster=poster if poster is not None else _RecordingPoster(),
        redact=redact if redact is not None else (lambda note: note),
    )


# --- config validation (C1: the lease must outlive the budget) ----------------


def test_lease_floor_guard_rejects_a_lease_that_underruns_the_budget() -> None:
    # C1: a lease shorter than budget.wall_clock_s + margin would expire mid-run and let a
    # re-fire double-launch the still-running incident — fail CLOSED at boot, not silently.
    with pytest.raises(ValueError):
        ReceiverConfig(budget=_budget(), lease_s=10.0)  # 10 < 1000 + 30 floor


def test_lease_floor_guard_accepts_a_lease_above_the_floor() -> None:
    cfg = ReceiverConfig(budget=_budget(), lease_s=_budget().wall_clock_s + 30.0)
    assert cfg.lease_s == _budget().wall_clock_s + 30.0


# --- bearer verification (401 vs proceed) -------------------------------------


def test_verify_bearer_accepts_the_shared_token() -> None:
    assert verify_bearer(f"Bearer {_TOKEN}", _TOKEN) is True


def test_verify_bearer_rejects_wrong_absent_and_malformed() -> None:
    assert verify_bearer(f"Bearer {_TOKEN}x", _TOKEN) is False
    assert verify_bearer(None, _TOKEN) is False
    assert verify_bearer("", _TOKEN) is False
    assert verify_bearer(_TOKEN, _TOKEN) is False  # no scheme
    assert verify_bearer("Basic abc", _TOKEN) is False


# --- handle_webhook edge flow -------------------------------------------------


def test_bad_bearer_is_401_not_5xx() -> None:
    rec = _receiver(lambda ctx: (_FakeSession([]), *_extractors()))
    status, body = rec.handle_webhook("Bearer wrong", _body())
    assert status == 401
    assert body["status"] == "unauthorized"


def test_non_firing_is_200_noop() -> None:
    rec = _receiver(lambda ctx: (_FakeSession([]), *_extractors()))
    status, body = rec.handle_webhook(f"Bearer {_TOKEN}", _body(status="resolved"))
    assert status == 200
    assert body["status"] == "noop"


def test_non_enrolled_is_200_noop() -> None:
    rec = _receiver(lambda ctx: (_FakeSession([]), *_extractors()))
    status, body = rec.handle_webhook(
        f"Bearer {_TOKEN}", _body(commonLabels={"severity": "critical"})
    )
    assert status == 200
    assert body["status"] == "noop"


def test_unparseable_body_is_200_noop_not_5xx() -> None:
    rec = _receiver(lambda ctx: (_FakeSession([]), *_extractors()))
    status, body = rec.handle_webhook(f"Bearer {_TOKEN}", "not a json object")
    assert status == 200
    assert body["status"] == "noop"
    assert body["reason"] == "parse_error"


def test_proceed_launches_and_acks_fast() -> None:
    launched: list[RunContext] = []

    def factory(ctx: RunContext):
        launched.append(ctx)
        return _FakeSession([]), *_extractors()

    rec = _receiver(factory)

    async def _run() -> tuple[int, dict]:
        status, body = rec.handle_webhook(f"Bearer {_TOKEN}", _body())
        await rec.drain()  # let the bg task complete so the factory ran
        return status, dict(body)

    status, body = asyncio.run(_run())
    assert status == 200
    assert body["status"] == "launched"
    assert "incident_key" in body
    assert len(launched) == 1
    # The contained alert reached the context; the raw webhook did not.
    assert launched[0].alert["alertname"] == "ChainHalted"


def test_duplicate_incident_sheds_200() -> None:
    dedup = InMemoryDedupStore(now=lambda: 0.0)
    # Pre-claim the incident so the handler's claim loses (a run already owns it).
    key = '{}:{alertname="ChainHalted"}'
    dedup.claim_run(key, "other-run", lease_s=100.0)
    rec = _receiver(lambda ctx: (_FakeSession([]), *_extractors()), dedup=dedup)
    status, body = rec.handle_webhook(f"Bearer {_TOKEN}", _body())
    assert status == 200
    assert body["status"] == "deduped"
    assert body["reason"] == "in_flight"


def test_global_capacity_cap_sheds() -> None:
    # A receiver at its in-flight cap sheds a fresh incident (back-pressure, not queue).
    hang = asyncio.Event()

    class _HangSession(_FakeSession):
        async def stream(self):
            await hang.wait()
            if False:  # pragma: no cover
                yield {}

    rec = _receiver(lambda ctx: (_HangSession([]), *_extractors()), max_in_flight=1)

    async def _run() -> tuple[int, dict]:
        # First incident occupies the single slot (its task hangs in stream()).
        s1, _ = rec.handle_webhook(f"Bearer {_TOKEN}", _body(groupKey="inc-1"))
        await asyncio.sleep(0)  # let the bg task start + register
        # Second, distinct incident → at capacity → shed.
        s2, b2 = rec.handle_webhook(f"Bearer {_TOKEN}", _body(groupKey="inc-2"))
        hang.set()
        await rec.drain()
        return (s1, s2), b2

    (s1, s2), b2 = asyncio.run(_run())
    assert s1 == 200
    assert s2 == 200
    assert b2["status"] == "deduped"
    assert b2["reason"] == "at_capacity"


def test_dedup_store_error_is_503_fail_closed() -> None:
    class _BoomStore(InMemoryDedupStore):
        def claim_run(self, *a, **k) -> bool:
            raise RuntimeError("store unavailable")

    rec = _receiver(
        lambda ctx: (_FakeSession([]), *_extractors()), dedup=_BoomStore(now=lambda: 0.0)
    )
    status, body = rec.handle_webhook(f"Bearer {_TOKEN}", _body())
    assert status == 503
    assert body["status"] == "error"


# --- supervise_run → post_back ------------------------------------------------


def test_supervise_posts_the_partial_on_a_truncated_terminal() -> None:
    # A run that blows the iteration budget → driver cancels → truncated outcome → the
    # post carries the PARTIAL artifact + the truncated headline (NOT an all-clear).
    session = _FakeSession([{"iter": True, "tokens": 1, "text": f"s{i} "} for i in range(10)])
    poster = _RecordingPoster()
    dedup = InMemoryDedupStore(now=lambda: 0.0)
    budget = Budget(
        wall_clock_s=1_000.0, tokens=1_000_000, queries=1_000, per_source_queries={},
        max_iterations=3, no_progress_iterations=1_000,
    )
    ctx = RunContext(incident_key="inc-1", run_id="run-a", goal="g", alert={})

    asyncio.run(
        supervise_run(
            ctx,
            budget=budget,
            session_factory=lambda c: (session, *_extractors()),
            dedup=dedup,
            poster=poster,
            redact=lambda note: note,
            now=lambda: 0.0,
            metrics=_receiver(lambda c: None).metrics,
        )
    )
    assert session.cancel_calls == 1  # the driver pulled the plug at the budget breach
    assert len(poster.notes) == 1
    incident_key, note = poster.notes[0]
    assert incident_key == "inc-1"
    assert "TRUNCATED" in note
    assert "NOT an all-clear" in note
    assert "s0 s1 s2 " in note  # the partial artifact assembled to the cut
    # The run-claim was released on completion (the lease is only the crash backstop).
    assert dedup.claim_run("inc-1", "run-b", lease_s=100.0) is True


def test_supervise_failure_still_posts_a_truncated_outcome() -> None:
    # A session-factory blowup must NOT vanish silently — the operator is waiting; a
    # truncated outcome is posted.
    def boom_factory(ctx):
        raise RuntimeError("could not create session")

    poster = _RecordingPoster()
    ctx = RunContext(incident_key="inc-1", run_id="run-a", goal="g", alert={})
    asyncio.run(
        supervise_run(
            ctx,
            budget=_budget(),
            session_factory=boom_factory,
            dedup=InMemoryDedupStore(now=lambda: 0.0),
            poster=poster,
            redact=lambda note: note,
            now=lambda: 0.0,
            metrics=_receiver(lambda c: None).metrics,
        )
    )
    assert len(poster.notes) == 1
    assert "TRUNCATED" in poster.notes[0][1]


def test_post_back_is_idempotent() -> None:
    # A second post_back for the same incident is a no-op (admit_post once-ness) — idempotent
    # under AM retry / a within-run retry.
    poster = _RecordingPoster()
    dedup = InMemoryDedupStore(now=lambda: 0.0)
    ctx = RunContext(incident_key="inc-1", run_id="run-a", goal="g", alert={})
    outcome = RunOutcome(
        terminal_reason=TerminalReason.GOAL_REACHED, truncated=False, tripped=None,
        cancelled=False, elapsed_s=1.0, tokens=10, iterations=1, artifact="findings",
    )

    async def _twice() -> None:
        for _ in range(2):
            await post_back(ctx, outcome, dedup=dedup, poster=poster,
                            redact=lambda note: note, metrics=_receiver(lambda c: None).metrics)

    asyncio.run(_twice())
    assert len(poster.notes) == 1  # posted once; the second call shed


def test_post_back_fails_closed_on_redaction_error() -> None:
    # A redaction failure must post NOTHING (never raw text) — fail-closed escalation. The
    # once-slot is RELEASED (post-then-claim claims only on success), so a corrected re-post
    # is not locked out.
    poster = _RecordingPoster()
    dedup = InMemoryDedupStore(now=lambda: 0.0)

    def boom_redact(note: str) -> str:
        raise RuntimeError("redaction engine down")

    ctx = RunContext(incident_key="inc-1", run_id="run-a", goal="g", alert={})
    outcome = RunOutcome(
        terminal_reason=TerminalReason.GOAL_REACHED, truncated=False, tripped=None,
        cancelled=False, elapsed_s=1.0, tokens=10, iterations=1, artifact="secrets here",
    )
    asyncio.run(
        post_back(ctx, outcome, dedup=dedup, poster=poster,
                  redact=boom_redact, metrics=_receiver(lambda c: None).metrics)
    )
    assert poster.notes == []  # nothing posted — no raw-text leak
    # Once-slot released: a corrected re-post can re-claim (not stranded by a failed attempt).
    assert dedup.claim_post("inc-1") is True


def test_post_back_retries_after_a_transient_post_failure() -> None:
    # Post-then-claim: a transient post failure does NOT consume the once-slot — it re-raises
    # AND leaves the slot free, so a retry re-attempts the post (the old claim-then-post order
    # dropped the note forever). On the retry's success the slot is claimed.
    class _FlakyPoster:
        def __init__(self) -> None:
            self.attempts = 0
            self.notes: list[tuple[str, str]] = []

        async def post_note(self, incident_key: str, note: str) -> None:
            self.attempts += 1
            if self.attempts == 1:
                raise RuntimeError("transient PD blip")
            self.notes.append((incident_key, note))

    poster = _FlakyPoster()
    dedup = InMemoryDedupStore(now=lambda: 0.0)
    ctx = RunContext(incident_key="inc-1", run_id="run-a", goal="g", alert={})
    outcome = RunOutcome(
        terminal_reason=TerminalReason.GOAL_REACHED, truncated=False, tripped=None,
        cancelled=False, elapsed_s=1.0, tokens=10, iterations=1, artifact="findings",
    )
    metrics = _receiver(lambda c: None).metrics

    async def _flow() -> None:
        with pytest.raises(RuntimeError):
            await post_back(ctx, outcome, dedup=dedup, poster=poster,
                            redact=lambda n: n, metrics=metrics)
        # The slot is free after the failed attempt → the retry re-attempts and succeeds.
        await post_back(ctx, outcome, dedup=dedup, poster=poster,
                        redact=lambda n: n, metrics=metrics)

    asyncio.run(_flow())
    assert poster.attempts == 2
    assert len(poster.notes) == 1  # posted exactly once, on the retry
    assert dedup.claim_post("inc-1") is False  # the once-slot is now claimed


def test_supervise_absorbs_a_post_failure_without_killing_the_task() -> None:
    # post_back re-raises a post failure; supervise_run absorbs it so the bg task ends cleanly
    # (no unretrieved-exception noise) and STILL releases the run-claim in finally.
    class _DeadPoster:
        async def post_note(self, incident_key: str, note: str) -> None:
            raise RuntimeError("PD unreachable after retries")

    dedup = InMemoryDedupStore(now=lambda: 0.0)
    ctx = RunContext(incident_key="inc-1", run_id="run-a", goal="g", alert={})

    asyncio.run(
        supervise_run(
            ctx,
            budget=_budget(),
            session_factory=lambda c: (_FakeSession([]), *_extractors()),
            dedup=dedup,
            poster=_DeadPoster(),
            redact=lambda n: n,
            now=lambda: 0.0,
            metrics=_receiver(lambda c: None).metrics,
        )
    )
    # The run-claim was released despite the post failure (finally ran); the once-slot is free
    # too (post failed → unclaimed), so a re-fire can re-attempt.
    assert dedup.claim_run("inc-1", "run-b", lease_s=100.0) is True
    assert dedup.claim_post("inc-1") is True


# --- note rendering (the §3.5 truncated-vs-surveyed distinction) --------------


def test_render_note_truncated_carries_the_truncated_headline() -> None:
    ctx = RunContext(incident_key="inc-1", run_id="run-a", goal="g", alert={})
    truncated = RunOutcome(
        terminal_reason=TerminalReason.BUDGET_EXHAUSTED, truncated=True, tripped="iterations",
        cancelled=True, elapsed_s=1.0, tokens=10, iterations=3, artifact="partial",
    )
    note = render_note(truncated, ctx)
    assert "TRUNCATED" in note
    assert "unsurveyed: iterations" in note
    assert "NOT an all-clear" in note


def test_render_note_surveyed_is_not_truncated() -> None:
    ctx = RunContext(incident_key="inc-1", run_id="run-a", goal="g", alert={})
    surveyed = RunOutcome(
        terminal_reason=TerminalReason.CLEAN_PUNT, truncated=False, tripped=None,
        cancelled=False, elapsed_s=1.0, tokens=10, iterations=2, artifact="ruled out X",
    )
    note = render_note(surveyed, ctx)
    assert "TRUNCATED" not in note
    assert "complete" in note


# --- the HTTP edge: body-bomb guard (build_app) -------------------------------


def test_oversized_body_is_413_before_auth_and_parse() -> None:
    # The body is buffered before the bearer can be trusted, so an unauthenticated client
    # can still stream bytes — a body over the cap is rejected 413, unread, regardless of the
    # (here absent) bearer. Starlette rides in via omnigent; skip cleanly where it's absent.
    starlette_testclient = pytest.importorskip("starlette.testclient")
    rec = _receiver(lambda ctx: (_FakeSession([]), *_extractors()))
    client = starlette_testclient.TestClient(build_app(rec))
    resp = client.post("/webhook", content=b"x" * (_MAX_BODY_BYTES + 1))
    assert resp.status_code == 413
    assert resp.json()["reason"] == "body_too_large"


def test_within_cap_body_reaches_the_handler() -> None:
    # A normal-sized body flows past the guard to the handler (here a bad/absent bearer → 401),
    # proving the guard does not reject legitimate payloads.
    starlette_testclient = pytest.importorskip("starlette.testclient")
    rec = _receiver(lambda ctx: (_FakeSession([]), *_extractors()))
    client = starlette_testclient.TestClient(build_app(rec))
    resp = client.post("/webhook", json={"version": "4"})  # no bearer → handler returns 401
    assert resp.status_code == 401
