"""Tests for the venue-agnostic router edge + supervisor (Design 13).

The handler is framework-free (takes the raw Authorization header + decoded body, returns
``(status, body)``), so the edge flow is provable without FastAPI/HTTP. ``supervise_run`` and
``post_back`` are driven against a fake GoalSession + a recording venue + an injected dedup
store, so the full lifecycle (drive_to_terminal → post the partial on a truncated terminal;
post_back idempotency) is provable without a live omnigent / PagerDuty.

Async tests run via ``asyncio.run`` (no pytest-asyncio dep — the suite stays pytest+ruff).
"""

from __future__ import annotations

import asyncio
import logging

import pytest

from sei_omnigent.omni._dedup import InMemoryDedupStore
from sei_omnigent.omni.adapters.alertmanager import AlertmanagerAdapter
from sei_omnigent.omni.driver import RunOutcome
from sei_omnigent.omni.engine import Budget, TerminalReason
from sei_omnigent.omni.router import (
    _MAX_BODY_BYTES,
    Router,
    RouterConfig,
    RunContext,
    _TaskTracker,
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


class _RecordingVenue:
    def __init__(self) -> None:
        self.results: list[tuple[str, str]] = []

    async def post_result(self, handle: str, body: str) -> bool:
        self.results.append((handle, body))
        return True


class _RecordingMetrics:
    """A metrics sink that records the admission decisions for assertion."""

    def __init__(self) -> None:
        self.admissions: list[str] = []

    def received(self, *, verified: bool) -> None: ...
    def admitted(self, *, decision: str) -> None:
        self.admissions.append(decision)

    def terminal(self, *, reason: str) -> None: ...
    def post(self, *, result: str) -> None: ...


def _router(
    session_factory, *, dedup=None, venue=None, max_in_flight=16, redact=None, metrics=None
) -> Router:
    return Router(
        config=RouterConfig(
            budget=_budget(), lease_s=1_100.0, max_in_flight=max_in_flight, now=lambda: 0.0
        ),
        adapter=AlertmanagerAdapter(),
        dedup=dedup if dedup is not None else InMemoryDedupStore(now=lambda: 0.0),
        session_factory=session_factory,
        expected_token=_TOKEN,
        venue=venue if venue is not None else _RecordingVenue(),
        redact=redact if redact is not None else (lambda body: body),
        metrics=metrics if metrics is not None else _RecordingMetrics(),
    )


# --- config validation (C1: the lease must outlive the budget) ----------------


def test_lease_floor_guard_rejects_a_lease_that_underruns_the_budget() -> None:
    # C1: a lease shorter than budget.wall_clock_s + margin would expire mid-run and let a
    # re-fire double-launch the still-running incident — fail CLOSED at boot, not silently.
    with pytest.raises(ValueError):
        RouterConfig(budget=_budget(), lease_s=10.0)  # 10 < 1000 + 30 floor


def test_lease_floor_guard_accepts_a_lease_above_the_floor() -> None:
    cfg = RouterConfig(budget=_budget(), lease_s=_budget().wall_clock_s + 30.0)
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
    rec = _router(lambda ctx: (_FakeSession([]), *_extractors()))
    status, body = rec.handle_webhook("Bearer wrong", _body())
    assert status == 401
    assert body["status"] == "unauthorized"


def test_non_firing_is_200_noop() -> None:
    rec = _router(lambda ctx: (_FakeSession([]), *_extractors()))
    status, body = rec.handle_webhook(f"Bearer {_TOKEN}", _body(status="resolved"))
    assert status == 200
    assert body["status"] == "noop"


def test_non_enrolled_is_200_noop() -> None:
    rec = _router(lambda ctx: (_FakeSession([]), *_extractors()))
    status, body = rec.handle_webhook(
        f"Bearer {_TOKEN}", _body(commonLabels={"severity": "critical"})
    )
    assert status == 200
    assert body["status"] == "noop"


def test_parse_error_is_200_noop_not_5xx_with_parse_error_metric() -> None:
    # A malformed body MUST be a 200-noop, never a 5xx (a 5xx invites AM to re-deliver the same
    # bad bytes). The distinction is in the reason/metric, not the status: the parse-error path
    # emits decision="parse_error" so a malformed-body storm is visible on-call.
    metrics = _RecordingMetrics()
    rec = _router(lambda ctx: (_FakeSession([]), *_extractors()), metrics=metrics)
    status, body = rec.handle_webhook(f"Bearer {_TOKEN}", "not a json object")
    assert status == 200  # 200, NOT 5xx — a bad body is not retryable
    assert body["status"] == "noop"
    assert body["reason"] == "parse_error"
    assert metrics.admissions == ["parse_error"]


def test_not_enrolled_is_200_noop_with_not_enrolled_metric() -> None:
    # A well-formed-but-non-enrolled webhook is a 200-noop too, but a DISTINCT signal: the
    # not_enrolled path emits decision="not_enrolled" so a benign not-enrolled flood does not
    # look like a malformed-body storm.
    metrics = _RecordingMetrics()
    rec = _router(lambda ctx: (_FakeSession([]), *_extractors()), metrics=metrics)
    status, body = rec.handle_webhook(
        f"Bearer {_TOKEN}", _body(commonLabels={"severity": "critical"})
    )
    assert status == 200
    assert body["status"] == "noop"
    assert body["reason"] == "not_enrolled"
    assert metrics.admissions == ["not_enrolled"]


def test_parse_error_and_not_enrolled_noops_are_distinguishable() -> None:
    # The on-call signal restored: the two non-match classes must NOT collapse into one
    # undifferentiated no-op — their reason + admission metric differ.
    parse_metrics = _RecordingMetrics()
    enroll_metrics = _RecordingMetrics()
    parse_rec = _router(lambda ctx: (_FakeSession([]), *_extractors()), metrics=parse_metrics)
    enroll_rec = _router(lambda ctx: (_FakeSession([]), *_extractors()), metrics=enroll_metrics)
    _, parse_body = parse_rec.handle_webhook(f"Bearer {_TOKEN}", "not a json object")
    _, enroll_body = enroll_rec.handle_webhook(
        f"Bearer {_TOKEN}", _body(commonLabels={"severity": "critical"})
    )
    assert parse_body["reason"] != enroll_body["reason"]
    assert parse_metrics.admissions != enroll_metrics.admissions


def test_proceed_launches_and_acks_fast() -> None:
    launched: list[RunContext] = []

    def factory(ctx: RunContext):
        launched.append(ctx)
        return _FakeSession([]), *_extractors()

    rec = _router(factory)

    async def _run() -> tuple[int, dict]:
        status, body = rec.handle_webhook(f"Bearer {_TOKEN}", _body())
        await rec.drain()  # let the bg task complete so the factory ran
        return status, dict(body)

    status, body = asyncio.run(_run())
    assert status == 200
    assert body["status"] == "launched"
    assert "dedup_key" in body
    assert len(launched) == 1
    # The contained alert reached the context; the raw webhook did not.
    assert launched[0].payload["alertname"] == "ChainHalted"


def test_duplicate_incident_sheds_200() -> None:
    dedup = InMemoryDedupStore(now=lambda: 0.0)
    # Pre-claim the trigger so the handler's claim loses (a run already owns it).
    key = '{}:{alertname="ChainHalted"}'
    dedup.claim_run(key, "other-run", lease_s=100.0)
    rec = _router(lambda ctx: (_FakeSession([]), *_extractors()), dedup=dedup)
    status, body = rec.handle_webhook(f"Bearer {_TOKEN}", _body())
    assert status == 200
    assert body["status"] == "deduped"
    assert body["reason"] == "in_flight"


def test_global_capacity_cap_sheds() -> None:
    # A router at its in-flight cap sheds a fresh trigger (back-pressure, not queue).
    hang = asyncio.Event()

    class _HangSession(_FakeSession):
        async def stream(self):
            await hang.wait()
            if False:  # pragma: no cover
                yield {}

    rec = _router(lambda ctx: (_HangSession([]), *_extractors()), max_in_flight=1)

    async def _run() -> tuple[int, dict]:
        # First trigger occupies the single slot (its task hangs in stream()).
        s1, _ = rec.handle_webhook(f"Bearer {_TOKEN}", _body(groupKey="inc-1"))
        await asyncio.sleep(0)  # let the bg task start + register
        # Second, distinct trigger → at capacity → shed.
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

    rec = _router(
        lambda ctx: (_FakeSession([]), *_extractors()), dedup=_BoomStore(now=lambda: 0.0)
    )
    status, body = rec.handle_webhook(f"Bearer {_TOKEN}", _body())
    assert status == 503
    assert body["status"] == "error"


# --- supervise_run → post_back ------------------------------------------------


def _ctx(**over: object) -> RunContext:
    base: dict = {
        "dedup_key": "inc-1",
        "venue_handle": "inc-1",
        "run_id": "run-a",
        "goal": "g",
        "payload": {},
    }
    base.update(over)
    return RunContext(**base)


def test_supervise_posts_the_partial_on_a_truncated_terminal() -> None:
    # A run that blows the iteration budget → driver cancels → truncated outcome → the
    # post carries the PARTIAL artifact + the truncated headline (NOT an all-clear).
    session = _FakeSession([{"iter": True, "tokens": 1, "text": f"s{i} "} for i in range(10)])
    venue = _RecordingVenue()
    dedup = InMemoryDedupStore(now=lambda: 0.0)
    budget = Budget(
        wall_clock_s=1_000.0, tokens=1_000_000, queries=1_000, per_source_queries={},
        max_iterations=3, no_progress_iterations=1_000,
    )
    ctx = _ctx()

    asyncio.run(
        supervise_run(
            ctx,
            budget=budget,
            session_factory=lambda c: (session, *_extractors()),
            dedup=dedup,
            venue=venue,
            redact=lambda body: body,
            now=lambda: 0.0,
            metrics=_router(lambda c: None).metrics,
        )
    )
    assert session.cancel_calls == 1  # the driver pulled the plug at the budget breach
    assert len(venue.results) == 1
    handle, body = venue.results[0]
    assert handle == "inc-1"
    assert "TRUNCATED" in body
    assert "NOT an all-clear" in body
    assert "s0 s1 s2 " in body  # the partial artifact assembled to the cut
    # The run-claim was released on completion (the lease is only the crash backstop).
    assert dedup.claim_run("inc-1", "run-b", lease_s=100.0) is True


def test_supervise_failure_still_posts_an_errored_outcome() -> None:
    # A session-factory blowup must NOT vanish silently — the operator is waiting; an ERRORED
    # outcome is posted (an honest "could not run", NOT a budget-axis TRUNCATED note).
    def boom_factory(ctx):
        raise RuntimeError("could not create session")

    venue = _RecordingVenue()
    ctx = _ctx()
    asyncio.run(
        supervise_run(
            ctx,
            budget=_budget(),
            session_factory=boom_factory,
            dedup=InMemoryDedupStore(now=lambda: 0.0),
            venue=venue,
            redact=lambda body: body,
            now=lambda: 0.0,
            metrics=_router(lambda c: None).metrics,
        )
    )
    assert len(venue.results) == 1
    body = venue.results[0][1]
    assert "ERRORED" in body
    assert "TRUNCATED" not in body  # a create failure is NOT a budget cut
    assert "unsurveyed" not in body  # no budget axis is named


def test_post_back_is_idempotent() -> None:
    # A second post_back for the same trigger is a no-op (admit_post once-ness) — idempotent
    # under retry / a within-run retry.
    venue = _RecordingVenue()
    dedup = InMemoryDedupStore(now=lambda: 0.0)
    ctx = _ctx()
    outcome = RunOutcome(
        terminal_reason=TerminalReason.GOAL_REACHED, truncated=False, tripped=None,
        cancelled=False, elapsed_s=1.0, tokens=10, iterations=1, artifact="findings",
    )

    async def _twice() -> None:
        for _ in range(2):
            await post_back(ctx, outcome, dedup=dedup, venue=venue,
                            redact=lambda body: body, metrics=_router(lambda c: None).metrics)

    asyncio.run(_twice())
    assert len(venue.results) == 1  # posted once; the second call shed


def test_post_back_releases_the_slot_on_a_fail_closed_skip() -> None:
    # The venue returned False (a fail-closed skip — no target / not enrolled / scan-
    # unconfirmed): NO result was written, so the provisional slot must be RELEASED. Otherwise a
    # later re-fire (when the target opens / settles) is deduped away and never posts — the
    # missed-post / denial-of-diagnosis the slot-retain bug would cause.
    class _SkippingVenue:
        def __init__(self) -> None:
            self.calls = 0

        async def post_result(self, handle: str, body: str) -> bool:
            self.calls += 1
            return False  # always a fail-closed skip

    venue = _SkippingVenue()
    dedup = InMemoryDedupStore(now=lambda: 0.0)
    ctx = _ctx()
    outcome = RunOutcome(
        terminal_reason=TerminalReason.GOAL_REACHED, truncated=False, tripped=None,
        cancelled=False, elapsed_s=1.0, tokens=10, iterations=1, artifact="findings",
    )
    metrics = _router(lambda c: None).metrics

    async def _twice() -> None:
        for _ in range(2):
            await post_back(ctx, outcome, dedup=dedup, venue=venue,
                            redact=lambda n: n, metrics=metrics)

    asyncio.run(_twice())
    # Each re-fire re-attempts (the slot was released after the skip), not deduped away.
    assert venue.calls == 2
    # The slot is free — a corrected re-post / later re-fire can still claim it.
    assert dedup.claim_post("inc-1", "probe") is True


def test_post_back_does_not_double_post_across_a_restart_rescan() -> None:
    # The composition the slot-release fix turns on, pinned directly: run A posts a real result
    # (True, keeps the slot); a restart loses the in-memory slot; run B re-fires against a FRESH
    # dedup store and the venue now reports the marker is already in PD (False) → slot released,
    # NO second post. Exactly one real write — the durable venue marker, not the in-memory slot,
    # is the cross-restart guard.
    class _MarkerAwareVenue:
        def __init__(self) -> None:
            self.writes = 0

        async def post_result(self, handle: str, body: str) -> bool:
            if self.writes == 0:  # first run: marker absent → a real write
                self.writes += 1
                return True
            return False  # re-fire: the marker is already in PD → fail-closed skip

    venue = _MarkerAwareVenue()
    outcome = RunOutcome(
        terminal_reason=TerminalReason.GOAL_REACHED, truncated=False, tripped=None,
        cancelled=False, elapsed_s=1.0, tokens=10, iterations=1, artifact="findings",
    )
    metrics = _router(lambda c: None).metrics

    async def _restart_rescan() -> None:
        # Run A — fresh store, real post.
        await post_back(
            _ctx(run_id="run-a"),
            outcome, dedup=InMemoryDedupStore(now=lambda: 0.0), venue=venue,
            redact=lambda n: n, metrics=metrics,
        )
        # Restart: a brand-new store (the in-memory slot is gone). Run B re-fires.
        await post_back(
            _ctx(run_id="run-b"),
            outcome, dedup=InMemoryDedupStore(now=lambda: 0.0), venue=venue,
            redact=lambda n: n, metrics=metrics,
        )

    asyncio.run(_restart_rescan())
    assert venue.writes == 1  # exactly one real post despite the re-fire after the lost slot


def test_post_back_fails_closed_on_redaction_error() -> None:
    # A redaction failure must post NOTHING (never raw text) — fail-closed escalation. The
    # once-slot is RELEASED (post-then-claim claims only on success), so a corrected re-post
    # is not locked out.
    venue = _RecordingVenue()
    dedup = InMemoryDedupStore(now=lambda: 0.0)

    def boom_redact(body: str) -> str:
        raise RuntimeError("redaction engine down")

    ctx = _ctx()
    outcome = RunOutcome(
        terminal_reason=TerminalReason.GOAL_REACHED, truncated=False, tripped=None,
        cancelled=False, elapsed_s=1.0, tokens=10, iterations=1, artifact="secrets here",
    )
    asyncio.run(
        post_back(ctx, outcome, dedup=dedup, venue=venue,
                  redact=boom_redact, metrics=_router(lambda c: None).metrics)
    )
    assert venue.results == []  # nothing posted — no raw-text leak
    # Once-slot released: a corrected re-post can re-claim (not stranded by a failed attempt).
    assert dedup.claim_post("inc-1", "probe") is True


def test_post_back_retries_after_a_transient_post_failure() -> None:
    # Post-then-claim: a transient post failure does NOT consume the once-slot — it re-raises
    # AND leaves the slot free, so a retry re-attempts the post. On the retry's success the
    # slot is claimed.
    class _FlakyVenue:
        def __init__(self) -> None:
            self.attempts = 0
            self.results: list[tuple[str, str]] = []

        async def post_result(self, handle: str, body: str) -> bool:
            self.attempts += 1
            if self.attempts == 1:
                raise RuntimeError("transient PD blip")
            self.results.append((handle, body))
            return True

    venue = _FlakyVenue()
    dedup = InMemoryDedupStore(now=lambda: 0.0)
    ctx = _ctx()
    outcome = RunOutcome(
        terminal_reason=TerminalReason.GOAL_REACHED, truncated=False, tripped=None,
        cancelled=False, elapsed_s=1.0, tokens=10, iterations=1, artifact="findings",
    )
    metrics = _router(lambda c: None).metrics

    async def _flow() -> None:
        with pytest.raises(RuntimeError):
            await post_back(ctx, outcome, dedup=dedup, venue=venue,
                            redact=lambda n: n, metrics=metrics)
        # The slot is free after the failed attempt → the retry re-attempts and succeeds.
        await post_back(ctx, outcome, dedup=dedup, venue=venue,
                        redact=lambda n: n, metrics=metrics)

    asyncio.run(_flow())
    assert venue.attempts == 2
    assert len(venue.results) == 1  # posted exactly once, on the retry
    assert dedup.claim_post("inc-1", "probe") is False  # the once-slot is now claimed


def test_supervise_absorbs_a_post_failure_without_killing_the_task() -> None:
    # post_back re-raises a post failure; supervise_run absorbs it so the bg task ends cleanly
    # (no unretrieved-exception noise) and STILL releases the run-claim in finally.
    class _DeadVenue:
        async def post_result(self, handle: str, body: str) -> bool:
            raise RuntimeError("PD unreachable after retries")

    dedup = InMemoryDedupStore(now=lambda: 0.0)
    ctx = _ctx()

    asyncio.run(
        supervise_run(
            ctx,
            budget=_budget(),
            session_factory=lambda c: (_FakeSession([]), *_extractors()),
            dedup=dedup,
            venue=_DeadVenue(),
            redact=lambda n: n,
            now=lambda: 0.0,
            metrics=_router(lambda c: None).metrics,
        )
    )
    # The run-claim was released despite the post failure (finally ran); the once-slot is free
    # too (post failed → unclaimed), so a re-fire can re-attempt.
    assert dedup.claim_run("inc-1", "run-b", lease_s=100.0) is True
    assert dedup.claim_post("inc-1", "probe") is True


# --- note rendering (the §3.5 truncated-vs-surveyed distinction) --------------


def test_render_note_truncated_carries_the_truncated_headline() -> None:
    ctx = _ctx()
    truncated = RunOutcome(
        terminal_reason=TerminalReason.BUDGET_EXHAUSTED, truncated=True, tripped="iterations",
        cancelled=True, elapsed_s=1.0, tokens=10, iterations=3, artifact="partial",
    )
    note = render_note(truncated, ctx)
    assert "TRUNCATED" in note
    assert "unsurveyed: iterations" in note
    assert "NOT an all-clear" in note


def test_render_note_surveyed_is_not_truncated() -> None:
    ctx = _ctx()
    surveyed = RunOutcome(
        terminal_reason=TerminalReason.CLEAN_PUNT, truncated=False, tripped=None,
        cancelled=False, elapsed_s=1.0, tokens=10, iterations=2, artifact="ruled out X",
    )
    note = render_note(surveyed, ctx)
    assert "TRUNCATED" not in note
    assert "complete" in note


def test_render_note_errored_is_honest_not_budget_or_all_clear() -> None:
    # An ERRORED run (create/transport failure, watchdog) must render an honest "could not run /
    # errored" headline that escalates to the human — never a budget-axis TRUNCATED line and
    # never the surveyed all-clear. The captured failure detail rides in the body.
    ctx = _ctx()
    errored = RunOutcome(
        terminal_reason=TerminalReason.ERRORED, truncated=True, tripped=None,
        cancelled=False, elapsed_s=0.0, tokens=0, iterations=0,
        artifact="[run errored before completing: RuntimeError: server down]",
    )
    note = render_note(errored, ctx)
    assert "ERRORED" in note
    assert "TRUNCATED" not in note  # not a budget cut
    assert "unsurveyed" not in note  # no budget axis is named
    assert "complete" not in note  # not an all-clear
    assert "escalating to the human" in note
    assert "server down" in note  # the failure detail reaches the human


# --- the HTTP edge: body-bomb guard (build_app) -------------------------------


def test_oversized_body_is_413_before_auth_and_parse() -> None:
    # The body is buffered before the bearer can be trusted, so an unauthenticated client
    # can still stream bytes — a body over the cap is rejected 413, unread, regardless of the
    # (here absent) bearer. Starlette rides in via omnigent; skip cleanly where it's absent.
    starlette_testclient = pytest.importorskip("starlette.testclient")
    rec = _router(lambda ctx: (_FakeSession([]), *_extractors()))
    client = starlette_testclient.TestClient(build_app(rec))
    resp = client.post("/webhook", content=b"x" * (_MAX_BODY_BYTES + 1))
    assert resp.status_code == 413
    assert resp.json()["reason"] == "body_too_large"


def test_within_cap_body_reaches_the_handler() -> None:
    # A normal-sized body flows past the guard to the handler (here a bad/absent bearer → 401),
    # proving the guard does not reject legitimate payloads.
    starlette_testclient = pytest.importorskip("starlette.testclient")
    rec = _router(lambda ctx: (_FakeSession([]), *_extractors()))
    client = starlette_testclient.TestClient(build_app(rec))
    resp = client.post("/webhook", json={"version": "4"})  # no bearer → handler returns 401
    assert resp.status_code == 401


# --- item 5: the redactor must not default to a fail-open identity ------------


def test_router_rejects_a_missing_redactor_at_boot() -> None:
    # An unconfigured redactor is a fail-OPEN (raw text into the venue) — boot must reject it
    # loudly, matching the RouterConfig / load_webhook_token fail-closed-at-boot discipline.
    with pytest.raises(ValueError, match="redact"):
        Router(
            config=RouterConfig(budget=_budget(), lease_s=1_100.0, now=lambda: 0.0),
            adapter=AlertmanagerAdapter(),
            dedup=InMemoryDedupStore(now=lambda: 0.0),
            session_factory=lambda ctx: (_FakeSession([]), *_extractors()),
            expected_token=_TOKEN,
            venue=_RecordingVenue(),
            # no redact= → the sentinel default → boot rejects it
        )


# --- item 8: an unexpected venue exception is surfaced distinctly -------------


def test_supervise_surfaces_an_unexpected_venue_exception_distinctly(caplog) -> None:
    # A PROGRAMMING bug in the venue (KeyError) must NOT degrade to the recoverable post_failed
    # warning — it is logged at exception level, distinct from a designed PagerDutyError. The bg
    # task still ends cleanly (finally runs, run-claim released).
    class _BuggyVenue:
        async def post_result(self, handle: str, body: str) -> bool:
            raise KeyError("a programming bug in the venue")

        async def aclose(self) -> None:
            return None

    dedup = InMemoryDedupStore(now=lambda: 0.0)
    ctx = _ctx()
    with caplog.at_level(logging.ERROR, logger="sei_omnigent.omni.router"):
        asyncio.run(
            supervise_run(
                ctx,
                budget=_budget(),
                session_factory=lambda c: (_FakeSession([]), *_extractors()),
                dedup=dedup,
                venue=_BuggyVenue(),
                redact=lambda n: n,
                now=lambda: 0.0,
                metrics=_router(lambda c: None).metrics,
            )
        )
    # Surfaced at exception/error level via the distinct unexpected-error log line, NOT the
    # plain post_failed warning.
    assert any(
        rec.levelno >= logging.ERROR and "post_unexpected_error" in rec.message
        for rec in caplog.records
    )
    assert not any("post_failed" in rec.message for rec in caplog.records)
    # The bg task still ended cleanly: run-claim released, post-slot freed.
    assert dedup.claim_run("inc-1", "run-b", lease_s=100.0) is True
    assert dedup.claim_post("inc-1", "probe") is True


# --- item 7: aclose closes the venue on shutdown ------------------------------


def test_router_aclose_closes_the_venue() -> None:
    closed = {"n": 0}

    class _ClosableVenue:
        async def post_result(self, handle: str, body: str) -> bool: ...

        async def aclose(self) -> None:
            closed["n"] += 1

    rec = _router(lambda ctx: (_FakeSession([]), *_extractors()), venue=_ClosableVenue())
    asyncio.run(rec.aclose())
    assert closed["n"] == 1


# --- B3: a run outliving the drain is CANCELLED, not abandoned-then-closed-under -----


def test_drain_cancels_a_survivor_past_the_deadline() -> None:
    # B3: a run still in flight at the drain deadline must be CANCELLED and its cancellation
    # awaited — NOT abandoned-but-left-running (which would let it keep holding the shared
    # session-factory client while the lifespan goes on to close that client).
    tracker = _TaskTracker(drain_deadline_s=0.05, cancel_grace_s=1.0)
    cancelled = {"hit": False}

    async def _flow() -> None:
        async def _long_run() -> None:
            try:
                await asyncio.Event().wait()  # never completes on its own
            except asyncio.CancelledError:
                cancelled["hit"] = True
                raise

        started = asyncio.Event()

        async def _wrapped() -> None:
            started.set()
            await _long_run()

        tracker.spawn(_wrapped())
        await started.wait()
        await tracker.drain()  # deadline elapses → survivor cancelled + awaited

    asyncio.run(_flow())
    assert cancelled["hit"] is True  # the survivor saw the cancellation (did not run on)


def test_aclose_closes_factory_only_after_drain_completes() -> None:
    # B3 ordering: the session-factory client must NOT be closed while a run may still hold it.
    # Here a run is in flight at drain time; the drain cancels + awaits it, THEN aclose closes
    # the factory — so the factory close never races a live run.
    events: list[str] = []

    class _ClosableFactory:
        def __call__(self, ctx):
            return _FakeSession([]), *_extractors()

        async def aclose(self) -> None:
            events.append("factory_closed")

    class _ClosableVenue:
        async def post_result(self, handle: str, body: str) -> bool:
            return True

        async def aclose(self) -> None:
            events.append("venue_closed")

    factory = _ClosableFactory()
    rec = Router(
        config=RouterConfig(budget=_budget(), lease_s=1_100.0, now=lambda: 0.0),
        adapter=AlertmanagerAdapter(),
        dedup=InMemoryDedupStore(now=lambda: 0.0),
        session_factory=factory,
        expected_token=_TOKEN,
        venue=_ClosableVenue(),
        redact=lambda n: n,
    )
    # Drive the lifespan order directly: drain (which records nothing here) then aclose.
    rec._tracker.drain_deadline_s = 0.05

    async def _shutdown() -> None:
        # Spawn a survivor so the drain has something to cancel before the factory closes.
        async def _long() -> None:
            await asyncio.Event().wait()

        rec._tracker.spawn(_long())
        await asyncio.sleep(0)
        await rec.drain()
        events.append("drained")
        await rec.aclose()

    asyncio.run(_shutdown())
    # The factory close happens AFTER the drain returned (no client-close under a live run).
    assert events.index("drained") < events.index("factory_closed")
    assert events.index("venue_closed") < events.index("factory_closed")
