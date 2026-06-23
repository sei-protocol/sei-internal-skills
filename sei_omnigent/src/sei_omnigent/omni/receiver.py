"""omni-trigger receiver: the no-LLM trigger layer for WallE (PLT-715).

The thin lifecycle supervisor over a server-run agent (Design 12 §2, slice shape (a)):
**verify → parse+contain → derive key → admit → launch-and-ack-fast → observe-to-terminal
→ propose-only post-back**. It runs NO LLM itself; it admits a trigger, launches a tracked
background investigation against the armed omnigent host, and funnels the one terminal note
through a single redacted egress chokepoint.

Composition (does NOT reimplement): the reliability core is :mod:`engine` (admission +
budget + post chokepoint), the loop terminator is :func:`driver.drive_to_terminal`, the
live session glue is :class:`_omnigent_session.GoalSession` + :func:`make_extractors`, and
the parse/containment is :mod:`_alert`. This module is the HTTP edge + the wiring, fully
dependency-injected (session factory, poster, dedup store, clock) so the flow is provable
without a live omnigent / PagerDuty (mirrors ``driver.py``'s injected-deps discipline).

THE EDGE (handle_webhook), in order — every step's failure mode is the design:

1. **Verify** the bearer on the RAW request → wrong/absent = **401**, never 5xx (a 5xx
   invites AM to retry an unverifiable body; a 401 tells it to stop). The only enforced
   auth on the AM→receiver edge.
2. **Parse + field-allowlist** into a bounded, framed dict (INV-6). Non-firing / non-
   enrolled / unparseable → **200 + no-op** (the page already routed to the human).
3. **Derive ``incident_key``** deterministically (retries collapse to one key).
4. **Admit**: ``dedup.claim_run`` atomically → ``engine.admit_run``. SHED → **200 +
   deduped** (attach, not queue). A global in-flight cap also SHEDs (back-pressure).
5. **Launch** ``supervise_run`` as a tracked bg task and **ack 200 immediately** (AM's
   webhook timeout is seconds; the investigation is minutes — blocking guarantees retries).
   Only ever **5xx** on a fast fail-closed dedup-store error.

Reliability posture: every external call timeout+bounded (the cancel in ``driver``, the
poster's own deadline); back-pressure is SHED-not-queue; idempotent under AM retry by
construction (the ``incident_key`` + ``claim_run``/``claim_post``); bg tasks bounded +
tracked + drained on shutdown (structured concurrency). A truncated terminal posts the
PARTIAL artifact (``is_truncated`` headline), never an all-clear.

BOUNDARIES (the manifest injects, this module receives): the inbound bearer
(``OMNIGENT_TRIGGER_WEBHOOK_TOKEN_FILE``), the ``omni-root-cause`` token + ``host_id`` the
session factory binds to, and the PD token the real poster carries. The real
:class:`PagerDutyPoster` client is a FOLLOW-UP PR — this module depends only on the
Protocol; the spike ships :class:`LoggingPoster`.

Design 12 §2, §3.5; INV-6.
"""

from __future__ import annotations

import asyncio
import hmac
import logging
import os
import time
import uuid
from collections.abc import Awaitable, Callable, Mapping
from dataclasses import dataclass, field
from pathlib import Path
from typing import Protocol

from . import _alert
from ._dedup import DedupStore
from .driver import RunOutcome, SessionLike, drive_to_terminal
from .engine import Budget, RunAdmission, TerminalReason, admit_post, admit_run, is_truncated

_log = logging.getLogger("sei_omnigent.omni.receiver")

#: Env var pointing at the shared inbound bearer file (the AM→receiver edge auth).
WEBHOOK_TOKEN_FILE_ENV = "OMNIGENT_TRIGGER_WEBHOOK_TOKEN_FILE"
WEBHOOK_TOKEN_ENV = "OMNIGENT_TRIGGER_WEBHOOK_TOKEN"

#: Default lease on a run-claim — a hard ceiling on how long one incident SHEDs re-fires.
#: Must exceed the investigation budget's wall-clock, or the lease expires mid-run and a
#: re-fire double-launches; the manifest sets it from the profile's wall_clock + margin.
_DEFAULT_LEASE_S = 1800.0

#: The global in-flight cap (back-pressure): the receiver admits at most this many
#: concurrent investigations; beyond it a fresh incident SHEDs rather than the host (and the
#: receiver's own task set) growing unbounded. Per-incident dedup is the first shed; this is
#: the system-wide one. Sized to the host's concurrent-session capacity (manifest-tunable).
_DEFAULT_MAX_IN_FLIGHT = 16

#: Hard cap on the inbound request body. The AM→receiver edge is authenticated, but the
#: body is read BEFORE the bearer can be trusted, so an unauthenticated client can still
#: stream bytes at us: a body over this is rejected unread rather than buffered into JSON.
#: AM webhook v4 bodies are kilobytes; this is generous headroom, not a tuning knob.
_MAX_BODY_BYTES = 1 << 20  # 1 MiB


# --- the post-back interface (the real client is a follow-up PR) ----------------


class PagerDutyPoster(Protocol):
    """The post-back seam: find the incident by dedup key, add a note. Propose-only.

    The receiver depends on this Protocol, never a concrete client — the real PD client
    (with its token, find-incident-by-dedup-key call, and the idempotency-key on the note
    that closes the double-propose window) is a FOLLOW-UP PR. ``post_note`` MUST carry its
    OWN timeout + bounded retry (the receiver does not wrap it — the chokepoint fails closed
    on any raise). ``incident_key`` is the AM groupKey-derived dedup key; a real client maps
    it to the PD incident (PD's own ``dedup_key``) and SHOULD send it as the note's
    idempotency key (the preferred close for the restart double-propose window, see
    ``_dedup`` RESIDUAL).
    """

    async def post_note(self, incident_key: str, note: str) -> None: ...


@dataclass(frozen=True)
class LoggingPoster:
    """A no-op poster that logs the note — the spike's stand-in for the real PD client.

    Lets the full flow (and its tests) run with NO PagerDuty token / network. The real
    client lands in the follow-up PR; this one proves the chokepoint wiring end-to-end.
    """

    async def post_note(self, incident_key: str, note: str) -> None:
        _log.info("walle.post_back incident_key=%s note_len=%d", incident_key, len(note))


# --- the session factory seam (the live glue is _omnigent_session) --------------

# A factory the receiver calls to mint a goal-bound session for one investigation. The live
# impl builds a GoalSession over the armed omnigent host (bound to host_id + the
# omni-root-cause token); the test passes a fake. Returns the session + its extractor triple.
SessionLaunch = tuple[
    "SessionLike",
    Callable[[object], int],
    Callable[[object], bool],
    Callable[[object], str],
]
SessionFactory = Callable[["RunContext"], SessionLaunch]


@dataclass(frozen=True)
class RunContext:
    """The bounded, contained context for one investigation — the unit a bg task carries.

    ``alert`` is the ``extract_allowlisted`` output (capped + ``<untrusted-data>``-framed);
    the raw webhook is NEVER carried here (INV-6). ``goal`` is the profile's goal_template
    rendered with that contained alert. ``incident_key`` keys every metric/log + the dedup.
    """

    incident_key: str
    run_id: str
    goal: str
    alert: Mapping[str, str]


#: Floor on how far the run-lease must exceed the budget wall-clock. The lease is the
#: crash backstop that SHEDs re-fires for one incident; if it expires while the run is
#: still inside its budget, a re-fire wins a fresh claim and double-launches. The margin
#: covers the post-back + release that happen AFTER the budget wall-clock elapses.
_DEFAULT_MIN_LEASE_MARGIN_S = 30.0


@dataclass(frozen=True)
class ReceiverConfig:
    """Static config the manifest injects (clock + caps + budget). Frozen — set at boot."""

    budget: Budget
    lease_s: float = _DEFAULT_LEASE_S
    max_in_flight: int = _DEFAULT_MAX_IN_FLIGHT
    min_lease_margin_s: float = _DEFAULT_MIN_LEASE_MARGIN_S
    now: Callable[[], float] = time.monotonic

    def __post_init__(self) -> None:
        # C1: a lease that does not outlive the budget (plus a post-back/release margin) is a
        # double-launch waiting to happen — fail CLOSED at boot rather than ship a config that
        # silently voids single-flight. The manifest sets lease_s from the profile wall_clock.
        floor = self.budget.wall_clock_s + self.min_lease_margin_s
        if self.lease_s < floor:
            raise ValueError(
                f"lease_s={self.lease_s} must be >= budget.wall_clock_s "
                f"({self.budget.wall_clock_s}) + min_lease_margin_s ({self.min_lease_margin_s}) "
                f"= {floor}, or an expiring lease double-launches a still-running incident."
            )


# --- metrics seam (low-cardinality, keyed by decision; the backend is the obs agents') ---


class Metrics(Protocol):
    """The decision-point metric sink, low-cardinality by contract.

    Labels are the bounded decision enums (``verified`` bool, ``decision`` in admitted-set,
    ``reason`` in TerminalReason, ``result`` in post-result-set), NEVER the ``incident_key``
    (unbounded cardinality). ``incident_key`` goes to the structured LOG line, not the metric
    label. The real sink (Prometheus counters via the OTel SDK) is the observability agents'
    wiring; the receiver only decides WHAT to count + WHERE. The default is a no-op.
    """

    def received(self, *, verified: bool) -> None: ...
    def admitted(self, *, decision: str) -> None: ...
    def terminal(self, *, reason: str) -> None: ...
    def post(self, *, result: str) -> None: ...


class _NoopMetrics:
    """The default sink: counts nothing (the obs agents wire the real Prometheus backend)."""

    def received(self, *, verified: bool) -> None: ...
    def admitted(self, *, decision: str) -> None: ...
    def terminal(self, *, reason: str) -> None: ...
    def post(self, *, result: str) -> None: ...


# --- bearer verification (edge auth) --------------------------------------------


def load_webhook_token() -> str:
    """Load the shared inbound bearer from the file env (preferred) or the inline env.

    File-first (the manifest mounts it from a Secret), env as a dev fallback. An unset /
    unreadable / empty token is a misconfiguration that fails CLOSED at boot — a receiver
    with no token would 401 every webhook (no auth bypass), so surface it loudly here.
    """
    path = os.environ.get(WEBHOOK_TOKEN_FILE_ENV)
    if path:
        try:
            token = Path(path).read_text(encoding="utf-8").strip()
        except OSError as exc:
            raise RuntimeError(
                f"{WEBHOOK_TOKEN_FILE_ENV}={path!r} is unreadable: {exc}. The trigger "
                "receiver cannot verify any webhook without it (fail-closed at boot)."
            ) from exc
    else:
        token = (os.environ.get(WEBHOOK_TOKEN_ENV) or "").strip()
    if not token:
        raise RuntimeError(
            f"No inbound webhook bearer set ({WEBHOOK_TOKEN_FILE_ENV} or "
            f"{WEBHOOK_TOKEN_ENV}); the receiver would 401 every webhook. Set it."
        )
    return token


def verify_bearer(authorization: str | None, expected_token: str) -> bool:
    """Constant-time check of an ``Authorization: Bearer <token>`` header.

    ``hmac.compare_digest`` (not ``==``) so a token-guessing attacker cannot time the
    comparison byte-by-byte. A missing/malformed header is ``False`` (→ 401). The expected
    token is non-empty by :func:`load_webhook_token`'s boot guard, so an all-empty-vs-empty
    accidental match cannot happen.
    """
    if not authorization:
        return False
    scheme, _, presented = authorization.partition(" ")
    if scheme.lower() != "bearer" or not presented:
        return False
    return hmac.compare_digest(presented.strip(), expected_token)


# --- the supervisor + the egress chokepoint ------------------------------------


def render_note(outcome: RunOutcome, ctx: RunContext) -> str:
    """Render the propose-only PagerDuty note from a terminal outcome (§3.5).

    A TRUNCATED outcome (``is_truncated`` — budget/no-progress/transport) MUST carry the
    truncated headline, never the surveyed all-clear: a partial investigation that says
    "all clear" is the misleading-rate SLO failure §3.5 forbids. The note is a PROPOSAL
    (WallE never acts), so it states what was found + that it is truncated, and names the
    tripped axis so the human knows what was NOT surveyed.
    """
    headline = (
        f"WallE investigation TRUNCATED ({outcome.terminal_reason.value}"
        + (f", unsurveyed: {outcome.tripped}" if outcome.tripped else "")
        + ") — PARTIAL findings, NOT an all-clear:"
        if is_truncated(outcome.terminal_reason)
        else f"WallE investigation complete ({outcome.terminal_reason.value}):"
    )
    artifact = outcome.artifact or "(no artifact produced)"
    return f"[incident {ctx.incident_key}] {headline}\n\n{artifact}"


async def post_back(
    ctx: RunContext,
    outcome: RunOutcome,
    *,
    dedup: DedupStore,
    poster: PagerDutyPoster,
    redact: Callable[[str], str],
    metrics: Metrics,
) -> None:
    """The SINGLE egress chokepoint: admit-once → redact → propose-only note. Fail-closed.

    The one place a note leaves the receiver. ``dedup.claim_post`` feeds
    ``engine.admit_post`` so a re-run / retry posts AT MOST ONCE per incident (idempotent
    under AM retry). Then the note is REDACTED before it leaves; a redaction OR post error
    posts NOTHING and escalates (logs at error) — it never emits raw / unredacted text. The
    poster owns its own timeout+retry (the Protocol contract); the chokepoint only enforces
    once-ness, redaction, and fail-closed.
    """
    if not admit_post(incident_already_posted=not dedup.claim_post(ctx.incident_key)):
        # A note already went out for this incident — the retry/re-run is a no-op (not an
        # error): the human already has the proposal.
        metrics.post(result="deduped")
        _log.info("walle.post_back.deduped incident_key=%s", ctx.incident_key)
        return

    try:
        redacted = redact(render_note(outcome, ctx))
        await poster.post_note(ctx.incident_key, redacted)
    except Exception:
        # Fail-closed: a redaction or post failure must NEVER fall through to emitting raw
        # text. Nothing is posted; the operator sees the escalation in the error log. (The
        # claim_post already consumed the once-slot — a retry won't re-attempt the post; the
        # MVP accepts a dropped note over a raw-text leak. Durable retry is a follow-up.)
        metrics.post(result="error")
        _log.exception("walle.post_back.failed incident_key=%s — escalating, posted nothing",
                       ctx.incident_key)
        return

    metrics.post(result="posted")
    _log.info(
        "walle.post_back.posted incident_key=%s reason=%s truncated=%s",
        ctx.incident_key,
        outcome.terminal_reason.value,
        is_truncated(outcome.terminal_reason),
    )


async def supervise_run(
    ctx: RunContext,
    *,
    budget: Budget,
    session_factory: SessionFactory,
    dedup: DedupStore,
    poster: PagerDutyPoster,
    redact: Callable[[str], str],
    now: Callable[[], float],
    metrics: Metrics,
) -> None:
    """Supervise one investigation: create session → drive to terminal → post the result.

    The lifecycle body of a tracked bg task. Creates the goal-bound session via the injected
    factory (the live one binds the armed host + the omni-root-cause token), drives it to a
    terminal under ``budget`` (:func:`driver.drive_to_terminal` — the load-bearing loop
    terminator, since the Stop hook is observer-only), then funnels the outcome through the
    single :func:`post_back` chokepoint.

    Fail-closed end to end: ANY exception (session-create failure, an unexpected driver
    error) is mapped to a truncated ``BUDGET_EXHAUSTED`` outcome and STILL posted — a run
    that died must surface a truncated proposal, never silently vanish (the operator is
    waiting on the page). The run-claim is released in ``finally`` so a fast natural end
    frees the incident before the lease expires (the lease is the crash backstop, the
    release is the happy path).
    """
    metrics.admitted(decision=RunAdmission.PROCEED.value)
    try:
        try:
            session, token_delta, is_iteration, artifact_chunk = session_factory(ctx)
            outcome = await drive_to_terminal(
                session,
                budget,
                now=now,
                token_delta=token_delta,
                is_iteration=is_iteration,
                artifact_chunk=artifact_chunk,
            )
        except Exception:
            # Session create / unexpected drive failure: the run produced nothing, but the
            # operator is waiting — surface a truncated outcome (NOT an all-clear) so the
            # post-back still fires. driver.py already fails closed on stream errors; this
            # covers the create + the un-anticipated.
            _log.exception("walle.supervise.failed incident_key=%s — truncated outcome",
                           ctx.incident_key)
            outcome = RunOutcome(
                terminal_reason=TerminalReason.BUDGET_EXHAUSTED,
                truncated=True,
                tripped="run-error",
                cancelled=False,
                elapsed_s=0.0,
                tokens=0,
                iterations=0,
                artifact="WallE could not complete the investigation (internal error).",
            )
        metrics.terminal(reason=outcome.terminal_reason.value)
        await post_back(ctx, outcome, dedup=dedup, poster=poster, redact=redact, metrics=metrics)
    finally:
        dedup.release_run(ctx.incident_key, ctx.run_id)


# --- the HTTP app + the handler -------------------------------------------------


@dataclass
class _TaskTracker:
    """Bounded, tracked set of in-flight investigation tasks — structured concurrency.

    Every bg task is held here so shutdown can DRAIN them (no orphaned investigation, no
    "task was destroyed but it is pending" warning, no lost post-back). The set self-prunes
    via each task's done-callback so it does not grow unbounded across the receiver's life.
    The global in-flight CAP (admission back-pressure) is enforced against ``len`` here.
    """

    #: Bound on the shutdown drain. A wedged investigation (a poster that ignores its own
    #: deadline, say) must not hold the drain past the K8s SIGTERM grace window — at the
    #: deadline the drain returns and the survivors are abandoned (logged, not awaited).
    drain_deadline_s: float = 60.0
    _tasks: set[asyncio.Task[None]] = field(default_factory=set, init=False, repr=False)

    def in_flight(self) -> int:
        return len(self._tasks)

    def spawn(self, coro: Awaitable[None]) -> None:
        task = asyncio.create_task(coro)
        self._tasks.add(task)
        task.add_done_callback(self._tasks.discard)

    async def drain(self) -> None:
        """Await all in-flight tasks on shutdown, bounded by ``drain_deadline_s``.

        A copy is awaited because each task's done-callback mutates ``_tasks`` during drain.
        Exceptions are swallowed (a dying task must not break shutdown); on deadline the
        unfinished tasks are abandoned (logged, not cancelled) — the SIGTERM grace window at
        the K8s layer is the hard backstop.
        """
        if not self._tasks:
            return
        pending = tuple(self._tasks)
        try:
            async with asyncio.timeout(self.drain_deadline_s):
                await asyncio.gather(*pending, return_exceptions=True)
        except TimeoutError:
            abandoned = sum(1 for task in pending if not task.done())
            _log.warning(
                "walle.drain.timeout deadline_s=%s abandoned=%d — shutdown proceeding",
                self.drain_deadline_s,
                abandoned,
            )


@dataclass
class Receiver:
    """The trigger receiver's wiring + handler — fully dependency-injected (testable).

    Holds every seam the handler needs (config, dedup store, session factory, poster,
    redactor, metrics) + the bg-task tracker. :meth:`handle_webhook` is the edge; it is
    framework-agnostic (takes the raw ``Authorization`` header + the decoded JSON body and
    returns a ``(status, body)`` pair), so it is unit-testable WITHOUT FastAPI/HTTP, and
    :func:`build_app` adapts it to a FastAPI route.
    """

    config: ReceiverConfig
    dedup: DedupStore
    session_factory: SessionFactory
    expected_token: str
    poster: PagerDutyPoster = field(default_factory=LoggingPoster)
    redact: Callable[[str], str] = field(default=lambda note: note)
    metrics: Metrics = field(default_factory=_NoopMetrics)
    goal_template: str = "Investigate this alert and root-cause it:\n{alert}"
    _tracker: _TaskTracker = field(default_factory=_TaskTracker, init=False, repr=False)

    def handle_webhook(
        self, authorization: str | None, body: object
    ) -> tuple[int, Mapping[str, object]]:
        """The AM→receiver edge: verify → parse+contain → derive → admit → launch+ack-fast.

        Returns ``(http_status, json_body)``. Never blocks on the investigation — the bg
        task is spawned and the handler acks immediately (AM's webhook timeout is seconds).
        Status contract:

        * **401** — bad/absent bearer (NOT 5xx: a 5xx invites AM to retry an unverifiable
          body; a 401 tells it to stop).
        * **200 + ``status: noop``** — verified but non-firing / non-enrolled / unparseable
          (the page already routed to the human; a non-match is not an error).
        * **200 + ``status: deduped``** — SHED (a run owns the incident, or the global
          in-flight cap is hit): attach, not queue.
        * **200 + ``status: launched``** — admitted; the investigation runs in the bg.
        * **5xx** — ONLY a fast fail-closed dedup-store error (the one retryable failure).
        """
        if not verify_bearer(authorization, self.expected_token):
            self.metrics.received(verified=False)
            return 401, {"status": "unauthorized"}
        self.metrics.received(verified=True)

        try:
            webhook = _alert.parse_webhook(body)
        except _alert.WebhookError:
            # Unparseable is not retryable — a 5xx would re-deliver the same bad bytes. 200
            # + noop, distinguished in the metric (the page already paged the human).
            self.metrics.admitted(decision="parse_error")
            return 200, {"status": "noop", "reason": "parse_error"}

        if not _alert.is_firing(webhook) or not _alert.is_enrolled(webhook):
            self.metrics.admitted(decision="not_enrolled")
            return 200, {"status": "noop", "reason": "not_enrolled"}

        incident_key = _alert.derive_incident_key(webhook)

        # Global back-pressure: at the cap, SHED a fresh incident rather than growing the
        # task set + host load unbounded. Checked BEFORE the claim so a shed does not consume
        # a claim slot. (Per-incident dedup below is the first, finer shed.)
        if self._tracker.in_flight() >= self.config.max_in_flight:
            self.metrics.admitted(decision="shed_capacity")
            return 200, {"status": "deduped", "reason": "at_capacity"}

        run_id = uuid.uuid4().hex
        try:
            claimed = self.dedup.claim_run(incident_key, run_id, lease_s=self.config.lease_s)
        except Exception:
            # A dedup-store error is the ONE fast fail-closed 5xx: we cannot safely admit
            # without the single-flight guarantee, and the store error IS retryable (unlike a
            # bad body), so let AM retry. Never launch without a won claim.
            self.metrics.admitted(decision="store_error")
            _log.exception("walle.admit.store_error incident_key=%s", incident_key)
            return 503, {"status": "error", "reason": "dedup_store_unavailable"}

        if admit_run(incident_in_flight=not claimed) is RunAdmission.SHED:
            # A run already owns this incident — attach, not queue. The existing run's
            # post-back covers it.
            self.metrics.admitted(decision=RunAdmission.SHED.value)
            return 200, {"status": "deduped", "reason": "in_flight"}

        ctx = self._make_context(webhook, incident_key, run_id)
        self._tracker.spawn(
            supervise_run(
                ctx,
                budget=self.config.budget,
                session_factory=self.session_factory,
                dedup=self.dedup,
                poster=self.poster,
                redact=self.redact,
                now=self.config.now,
                metrics=self.metrics,
            )
        )
        return 200, {"status": "launched", "incident_key": incident_key, "run_id": run_id}

    def _make_context(self, webhook: _alert.Webhook, incident_key: str, run_id: str) -> RunContext:
        """Build the contained :class:`RunContext` — only the allowlisted alert reaches it."""
        alert = _alert.extract_allowlisted(webhook)
        # goal_template is TRUSTED (manifest-injected); the contained alert is the only
        # untrusted input and it is interpolated as a {alert} VALUE, never promoted to the
        # template position — so an alert value carrying its own "{...}" cannot re-template.
        goal = self.goal_template.format(alert=alert)
        return RunContext(incident_key=incident_key, run_id=run_id, goal=goal, alert=alert)

    async def drain(self) -> None:
        """Await in-flight investigations on shutdown (called from the app lifespan)."""
        await self._tracker.drain()


def build_app(receiver: Receiver) -> object:
    """Adapt a :class:`Receiver` to a Starlette app with the ``/webhook`` route + lifespan.

    The HTTP framing only — the decision logic is :meth:`Receiver.handle_webhook` (which is
    framework-free + unit-testable without this). The lifespan drains in-flight
    investigations on shutdown (structured concurrency: no orphaned run, no lost post-back).
    ``/healthz`` is a liveness probe. FastAPI/starlette are imported lazily (inside this
    builder) so the pure handler + its unit tests need no web framework on the import path.

    The route reads the raw ``Request`` itself (rather than a typed body model) so the
    bearer is verified BEFORE any body parsing — the decision logic (auth-first, then
    parse) lives wholly in :meth:`Receiver.handle_webhook`, not in FastAPI's request model.
    """
    from contextlib import asynccontextmanager  # noqa: PLC0415 -- deferred (web-glue only)

    from starlette.applications import Starlette  # noqa: PLC0415 -- deferred web glue
    from starlette.requests import Request  # noqa: PLC0415
    from starlette.responses import JSONResponse  # noqa: PLC0415
    from starlette.routing import Route  # noqa: PLC0415

    @asynccontextmanager
    async def lifespan(app: object):
        yield
        # Drain on shutdown so an in-flight investigation finishes (and posts) rather than
        # being torn down mid-run — the SIGTERM grace window bounds it at the K8s layer.
        await receiver.drain()

    async def webhook(request: Request) -> JSONResponse:
        authorization = request.headers.get("authorization")
        # Body-bomb guard: read the RAW bytes with a hard cap BEFORE JSON-parsing, since the
        # body is buffered before the bearer can be trusted (an unauthenticated client can
        # still stream at us). A Content-Length over the cap is rejected outright; absent/
        # lying that, the streamed total is capped too. 413, not 5xx — it is a client fault.
        declared = request.headers.get("content-length")
        if declared is not None and declared.isdigit() and int(declared) > _MAX_BODY_BYTES:
            return JSONResponse(
                status_code=413, content={"status": "error", "reason": "body_too_large"}
            )
        raw = b""
        async for chunk in request.stream():
            raw += chunk
            if len(raw) > _MAX_BODY_BYTES:
                return JSONResponse(
                    status_code=413, content={"status": "error", "reason": "body_too_large"}
                )
        try:
            import json  # noqa: PLC0415 -- deferred web glue

            body = json.loads(raw) if raw else None
        except Exception:
            # A non-JSON body verified-or-not is handled the same as a parse error AFTER
            # auth; but auth must run first, so hand the handler a non-mapping so it 200-noops
            # (it owns the noop mapping), NOT 5xx.
            body = None
        status, payload = receiver.handle_webhook(authorization, body)
        return JSONResponse(status_code=status, content=dict(payload))

    async def healthz(request: Request) -> JSONResponse:
        return JSONResponse({"status": "ok", "in_flight": receiver._tracker.in_flight()})

    return Starlette(
        lifespan=lifespan,
        routes=[
            Route("/webhook", webhook, methods=["POST"]),
            Route("/healthz", healthz, methods=["GET"]),
        ],
    )
