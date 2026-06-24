"""The venue-agnostic session-routing core (Design 13).

The router is the venue-agnostic pipeline extracted from the merged receiver (Design 12 #216):
**verify → normalize (via a TriggerAdapter) → admit/dedup → supervise_run → redacted post-back
(via a Venue)**. It runs NO LLM itself; it admits a normalized trigger, launches a tracked
background investigation against the armed omnigent host, and funnels the one terminal result
through a single redacted egress chokepoint.

It depends only on Protocol seams — :class:`~sei_omnigent.omni.adapters.base.TriggerAdapter`,
:class:`~sei_omnigent.omni.venues.base.Venue`, :class:`~sei_omnigent.omni._dedup.DedupStore`,
:class:`Metrics`, the existing ``SessionFactory``, and the fail-closed
:class:`~sei_omnigent.omni.control_plane.ControlPlane` (the PDP — deny-by-default skill/venue/
posture gating, evaluated before launch). The reliability core
is :mod:`engine`, the loop terminator is :func:`driver.drive_to_terminal`, the live session
glue is :mod:`_omnigent_session`. This module is the HTTP edge + the wiring, fully
dependency-injected so the flow is provable without a live omnigent / venue.

THE EDGE (handle_webhook), in order — every step's failure mode is the design:

1. **Verify** the bearer on the RAW request → wrong/absent = **401**, never 5xx (a 5xx invites
   the venue to retry an unverifiable body; a 401 tells it to stop).
2. **Normalize** via the :class:`TriggerAdapter` into a bounded, framed
   :class:`~sei_omnigent.omni.adapters.base.NormalizedTrigger` (INV-6). A non-matching event
   (the adapter returns a ``NoOp`` carrying its reason) → **200 + no-op** (the page already
   routed to the human); the reason (``parse_error`` vs ``not_enrolled``) is the metric label.
3. **Admit**: ``dedup.claim_run`` atomically → ``engine.admit_run``. SHED → **200 + deduped**
   (attach, not queue). A global in-flight cap also SHEDs (back-pressure).
4. **Launch** ``supervise_run`` as a tracked bg task and **ack 200 immediately** (the venue's
   webhook timeout is seconds; the investigation is minutes). Only ever **5xx** on a fast
   fail-closed dedup-store error.

Reliability posture: every external call timeout+bounded; back-pressure is SHED-not-queue;
idempotent under retry by construction (the ``dedup_key`` + ``claim_run``/``claim_post``); bg
tasks bounded + tracked + drained on shutdown (structured concurrency). A truncated terminal
posts the PARTIAL artifact, never an all-clear.

REDACTION IS THE SINGLE EGRESS CHOKEPOINT (INV-5): the router applies ``redact`` at
:func:`post_back` before ``Venue.post_result``, never in the runner or the driver.

Design 13 (Router core); Design 12 §2, §3.5; INV-5, INV-6.
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

from sei_omnigent.omni.adapters.base import NoOp, NormalizedTrigger, TriggerAdapter
from sei_omnigent.omni.control_plane import RunPlan
from sei_omnigent.omni.driver import RunOutcome, SessionLike, drive_to_terminal
from sei_omnigent.omni.engine import (
    Budget,
    RunAdmission,
    TerminalReason,
    admit_post,
    admit_run,
    is_truncated,
)
from sei_omnigent.omni.venues.base import Venue, VenueHandle
from sei_omnigent.omni.venues.pagerduty import PagerDutyError

from ._dedup import DedupStore

_log = logging.getLogger("sei_omnigent.omni.router")

#: Env var pointing at the shared inbound bearer file (the venue→router edge auth).
WEBHOOK_TOKEN_FILE_ENV = "OMNIGENT_TRIGGER_WEBHOOK_TOKEN_FILE"
WEBHOOK_TOKEN_ENV = "OMNIGENT_TRIGGER_WEBHOOK_TOKEN"

#: Default lease on a run-claim — a hard ceiling on how long one trigger SHEDs re-fires. Must
#: exceed the investigation budget's wall-clock, or the lease expires mid-run and a re-fire
#: double-launches; the manifest sets it from the profile's wall_clock + margin.
_DEFAULT_LEASE_S = 1800.0

#: The global in-flight cap (back-pressure): the router admits at most this many concurrent
#: investigations; beyond it a fresh trigger SHEDs rather than the host (and the router's own
#: task set) growing unbounded. Per-trigger dedup is the first shed; this is the system-wide
#: one. Default 1: the live session factory's OmnigentClient is shared across sessions and its
#: concurrency-safety is unproven, so serialize until a live N-concurrent soak clears
#: cross-stream bleed + bounded pool-wait. Manifest-tunable via the serve-wiring env.
_DEFAULT_MAX_IN_FLIGHT = 1

#: Hard cap on the inbound request body. The venue→router edge is authenticated, but the body
#: is read BEFORE the bearer can be trusted, so an unauthenticated client can still stream bytes
#: at us: a body over this is rejected unread rather than buffered into JSON. AM webhook v4
#: bodies are kilobytes; this is generous headroom, not a tuning knob.
_MAX_BODY_BYTES = 1 << 20  # 1 MiB


def _require_redactor(_body: str) -> str:
    """Sentinel default for ``Router.redact`` — its identity is the "unconfigured" marker.

    Used only as a default-object identity check in ``Router.__post_init__``; never called (a
    router that reaches a real post has a real redactor). If it ever is called, it fail-closes
    loudly rather than leaking the raw result.
    """
    raise RuntimeError("Router.redact was not configured (fail-closed; see __post_init__).")


# --- the session factory seam (the live glue is _omnigent_session) --------------

# A factory the router calls to mint a goal-bound session for one investigation. The live impl
# builds a GoalSession over the armed omnigent host (bound to host_id + the omni-root-cause
# token); the test passes a fake. Returns the session + its extractor triple.
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

    ``payload`` is the adapter's contained output (capped + ``<untrusted-data>``-framed); the
    raw venue event is NEVER carried here (INV-6). ``goal`` is the rendered goal. ``dedup_key``
    keys the single-flight; ``venue_handle`` is where the result posts back. Both metrics/logs
    key on ``dedup_key``.

    ``bundle_ref`` / ``model_override`` / ``reasoning_effort`` ride from the resolved
    :class:`~sei_omnigent.omni.control_plane.RunPlan` — the catalog key + model levers the
    SessionFactory will APPLY in a later session-application slice. This slice carries them on the
    context (they reach the factory via ``ctx``); it does NOT yet rewire ``create(bundle)`` /
    ``set_model_override``.
    """

    dedup_key: str
    venue_handle: VenueHandle
    run_id: str
    goal: str
    payload: Mapping[str, str]
    bundle_ref: str = ""
    model_override: str | None = None
    reasoning_effort: str | None = None


#: Floor on how far the run-lease must exceed the budget wall-clock. The lease is the crash
#: backstop that SHEDs re-fires for one trigger; if it expires while the run is still inside its
#: budget, a re-fire wins a fresh claim and double-launches. The margin covers the post-back +
#: release that happen AFTER the budget wall-clock elapses.
_DEFAULT_MIN_LEASE_MARGIN_S = 30.0


@dataclass(frozen=True)
class RouterConfig:
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


# --- the control-plane seam (the fail-closed PDP) -------------------------------


class ControlPlaneLike(Protocol):
    """The PDP seam the router routes every trigger through before launch.

    The production impl is the fail-closed :class:`~sei_omnigent.omni.control_plane.ControlPlane`
    (deny-by-default skill/venue/posture gating); a test passes a trivial double. The router
    depends only on this ``resolve`` shape, never the concrete table.
    """

    def resolve(self, trigger: NormalizedTrigger) -> RunPlan: ...


# --- metrics seam (low-cardinality, keyed by decision; the backend is the obs agents') ---


class Metrics(Protocol):
    """The decision-point metric sink, low-cardinality by contract.

    Labels are the bounded decision enums (``verified`` bool, ``decision`` in admitted-set,
    ``reason`` in TerminalReason, ``result`` in post-result-set), NEVER the ``dedup_key``
    (unbounded cardinality). ``dedup_key`` goes to the structured LOG line, not the metric
    label. The real sink (Prometheus counters via the OTel SDK) is the observability agents'
    wiring; the router only decides WHAT to count + WHERE. The default is a no-op.
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
    unreadable / empty token is a misconfiguration that fails CLOSED at boot — a router with no
    token would 401 every webhook (no auth bypass), so surface it loudly here.
    """
    path = os.environ.get(WEBHOOK_TOKEN_FILE_ENV)
    if path:
        try:
            token = Path(path).read_text(encoding="utf-8").strip()
        except OSError as exc:
            raise RuntimeError(
                f"{WEBHOOK_TOKEN_FILE_ENV}={path!r} is unreadable: {exc}. The trigger router "
                "cannot verify any webhook without it (fail-closed at boot)."
            ) from exc
    else:
        token = (os.environ.get(WEBHOOK_TOKEN_ENV) or "").strip()
    if not token:
        raise RuntimeError(
            f"No inbound webhook bearer set ({WEBHOOK_TOKEN_FILE_ENV} or "
            f"{WEBHOOK_TOKEN_ENV}); the router would 401 every webhook. Set it."
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
    """Render the propose-only result body from a terminal outcome (§3.5).

    Three headline classes, none allowed to read as an all-clear it is not:

    * ERRORED — the investigation could not run / died before completing (a session-create or
      transport failure, the watchdog). It is NOT a budget cut: it carries the failure detail
      and tells the human the system is escalating, never a budget-axis "unsurveyed" line.
    * TRUNCATED (``is_truncated`` — budget/no-progress) — a partial survey: PARTIAL findings,
      NOT an all-clear (§3.5's misleading-rate SLO), naming the tripped axis so the human knows
      what was NOT surveyed.
    * surveyed (goal-reached / clean-punt / insufficient-context) — the run did its job.
    """
    if outcome.terminal_reason is TerminalReason.ERRORED:
        headline = (
            "investigation ERRORED — the investigation could not run / errored before "
            "completing; escalating to the human. Details below, NOT an all-clear:"
        )
    elif is_truncated(outcome.terminal_reason):
        headline = (
            f"investigation TRUNCATED ({outcome.terminal_reason.value}"
            + (f", unsurveyed: {outcome.tripped}" if outcome.tripped else "")
            + ") — PARTIAL findings, NOT an all-clear:"
        )
    else:
        headline = f"investigation complete ({outcome.terminal_reason.value}):"
    artifact = outcome.artifact or "(no artifact produced)"
    return f"[incident {ctx.dedup_key}] {headline}\n\n{artifact}"


async def post_back(
    ctx: RunContext,
    outcome: RunOutcome,
    *,
    dedup: DedupStore,
    venue: Venue,
    redact: Callable[[str], str],
    metrics: Metrics,
) -> None:
    """The SINGLE egress chokepoint: redact → propose-only post → claim-ON-SUCCESS. Fail-closed.

    The one place a result leaves the router, ordered **post-then-claim** (claim provisionally,
    post, keep the claim only when a result was actually written): claim under ``ctx.run_id`` →
    redact (INV-5: the only egress redaction point) → post via :meth:`Venue.post_result` → keep
    the claim iff the venue reports it wrote a result. A failed redact, a failed post, OR a
    fail-closed skip (the venue wrote nothing) releases the claim (owner-checked), so the trigger
    stays eligible for a corrected re-post or a later re-fire. Double-propose across the
    in-memory claim's loss (restart) is narrowed at the venue layer: the PD venue embeds a
    durable per-incident marker and skips a re-post whose marker is already visible. That guard
    is best-effort, not atomic — a true cross-process close is the multi-replica shared-store
    un-defer.

    ``claim_post`` is the cheap local fast-path: if THIS process already holds the slot for this
    trigger, skip without a venue round-trip — it also keeps two concurrent post_backs in one
    process from both hitting the venue. It is a fast-path, not the durable guard.

    Fail-closed by error class: a redaction error or a designed ``PagerDutyError`` posts NOTHING
    (never raw text), releases the claim, and escalates. A ``PagerDutyError`` is the venue's own
    fail-closed signal → ``warning``; an UNEXPECTED exception from the venue (a programming bug)
    is re-raised after logging at ``exception`` so it surfaces distinctly.
    """
    if not admit_post(incident_already_posted=not dedup.claim_post(ctx.dedup_key, ctx.run_id)):
        # This process already holds the post-slot for this trigger — a within-process
        # retry/re-run is a no-op (not an error): the human already has the proposal. (A
        # cross-restart re-post is narrowed at the venue layer by the marker, not here.)
        metrics.post(result="deduped")
        _log.info("omni.post_back.deduped dedup_key=%s", ctx.dedup_key)
        return

    try:
        redacted = redact(render_note(outcome, ctx))
    except Exception:
        # Redaction fail-closed: NEVER fall through to raw text. Release the slot so a corrected
        # re-post can still reach the human, and escalate.
        dedup.unclaim_post(ctx.dedup_key, ctx.run_id)
        metrics.post(result="error")
        _log.exception(
            "omni.post_back.redact_failed dedup_key=%s — escalating, posted nothing",
            ctx.dedup_key,
        )
        return

    try:
        posted = await venue.post_result(ctx.venue_handle, redacted)
    except PagerDutyError:
        # The venue's designed fail-closed signal (timeout/4xx/exhausted retries). Release the
        # slot and re-raise so the supervisor escalates; this is recoverable (the venue marker
        # keeps a re-attempt from double-posting). The text was already redacted → re-raising
        # leaks nothing. warning, not exception — an expected failure mode, not a bug.
        dedup.unclaim_post(ctx.dedup_key, ctx.run_id)
        metrics.post(result="error")
        _log.warning("omni.post_back.failed dedup_key=%s — escalating, posted nothing",
                     ctx.dedup_key)
        raise
    except Exception:
        # An UNEXPECTED exception from the venue (e.g. a KeyError/AttributeError — a programming
        # bug, not a fail-closed venue outcome). Release the slot and re-raise; log at exception
        # so it is NOT folded into the recoverable post-failed narrative the supervisor absorbs.
        dedup.unclaim_post(ctx.dedup_key, ctx.run_id)
        metrics.post(result="error")
        _log.exception(
            "omni.post_back.unexpected_error dedup_key=%s — bug in the venue, escalating",
            ctx.dedup_key,
        )
        raise

    if not posted:
        # The venue returned a fail-closed skip (no target / not enrolled / already-marked /
        # scan-unconfirmed) — NO result was written. Release the slot so a later re-fire
        # re-evaluates; keeping it would dedup away every future attempt for this trigger. The
        # venue marker (not this slot) is what prevents a double-post once a result exists.
        dedup.unclaim_post(ctx.dedup_key, ctx.run_id)
        metrics.post(result="skipped")
        _log.info(
            "omni.post_back.skipped dedup_key=%s — venue wrote nothing, slot released",
            ctx.dedup_key,
        )
        return

    metrics.post(result="posted")
    _log.info(
        "omni.post_back.posted dedup_key=%s reason=%s truncated=%s",
        ctx.dedup_key,
        outcome.terminal_reason.value,
        is_truncated(outcome.terminal_reason),
    )


async def supervise_run(
    ctx: RunContext,
    *,
    budget: Budget,
    session_factory: SessionFactory,
    dedup: DedupStore,
    venue: Venue,
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

    Fail-closed end to end: ANY exception (a session-create failure, an unexpected driver error)
    is mapped to a truncated ``ERRORED`` outcome and STILL posted — a run that died must surface
    an honest "could not run" proposal, never silently vanish (the operator is waiting on the
    page) and never a budget-axis note (it was not a budget cut). The run-claim is released in
    ``finally`` so a fast natural end frees the trigger before the lease expires (the lease is
    the crash backstop, the release is the happy path).
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
            # operator is waiting — surface an ERRORED outcome (NOT an all-clear, NOT a budget
            # cut) so the post-back still fires with an honest "could not run". driver.py already
            # classifies in-stream errors as ERRORED; this covers the create + the un-anticipated
            # before the stream loop.
            _log.exception("omni.supervise.failed dedup_key=%s — errored outcome", ctx.dedup_key)
            outcome = RunOutcome(
                terminal_reason=TerminalReason.ERRORED,
                truncated=True,
                tripped=None,
                cancelled=False,
                elapsed_s=0.0,
                tokens=0,
                iterations=0,
                artifact="The investigation could not start (internal error).",
            )
        metrics.terminal(reason=outcome.terminal_reason.value)
        try:
            await post_back(
                ctx, outcome, dedup=dedup, venue=venue, redact=redact, metrics=metrics
            )
        except PagerDutyError:
            # A designed fail-closed venue failure: post_back already escalated (logged +
            # metric'd) and released the slot before re-raising. Absorb it so the bg task ends
            # cleanly (no unretrieved-exception noise) — the venue marker narrows a later
            # re-fire's double-post, so a dropped post here is recoverable, not a double-propose.
            _log.warning("omni.supervise.post_failed dedup_key=%s", ctx.dedup_key)
        except Exception:
            # An UNEXPECTED exception escaped post_back (a programming bug, not a fail-closed
            # venue outcome). Surface it distinctly at exception level — do NOT degrade a bug to
            # the recoverable post_failed warning line. The bg task still ends cleanly (finally).
            _log.exception("omni.supervise.post_unexpected_error dedup_key=%s", ctx.dedup_key)
    finally:
        dedup.release_run(ctx.dedup_key, ctx.run_id)


# --- the HTTP app + the handler -------------------------------------------------


@dataclass
class _TaskTracker:
    """Bounded, tracked set of in-flight investigation tasks — structured concurrency.

    Every bg task is held here so shutdown can DRAIN them (no orphaned investigation, no "task
    was destroyed but it is pending" warning, no lost post-back). The set self-prunes via each
    task's done-callback so it does not grow unbounded across the router's life. The global
    in-flight CAP (admission back-pressure) is enforced against ``len`` here.
    """

    #: Bound on the shutdown drain. A wedged investigation (a venue that ignores its own
    #: deadline, say) must not hold the drain past the K8s SIGTERM grace window — at the deadline
    #: the drain CANCELS the survivors and awaits their cancellation (bounded by
    #: ``cancel_grace_s``) so no run is still holding the shared client when the factory closes.
    drain_deadline_s: float = 60.0
    #: Bound on awaiting the cancellation of the survivors after the drain deadline. Short — a
    #: cancelled task should unwind promptly; this only covers its finally/cleanup. If even this
    #: elapses the tasks are abandoned (the SIGTERM grace at the K8s layer is the hard backstop),
    #: but the factory close is still gated behind this await (it does not race a live stream).
    cancel_grace_s: float = 5.0
    _tasks: set[asyncio.Task[None]] = field(default_factory=set, init=False, repr=False)

    def in_flight(self) -> int:
        return len(self._tasks)

    def spawn(self, coro: Awaitable[None]) -> None:
        task = asyncio.create_task(coro)
        self._tasks.add(task)
        task.add_done_callback(self._tasks.discard)

    async def drain(self) -> None:
        """Await in-flight tasks on shutdown, then CANCEL + await any survivors (bounded).

        A copy is awaited because each task's done-callback mutates ``_tasks`` during drain.
        Exceptions are swallowed (a dying task must not break shutdown). On the drain deadline the
        survivors are CANCELLED and their cancellation awaited (bounded by ``cancel_grace_s``):
        this is the B3 guarantee — no run may still be holding the shared session-factory client
        (the SSE stream / httpx pool slot) when the lifespan goes on to close that client. Closing
        the client under a live stream yanks the pool from under it. If even the cancel-grace
        elapses the tasks are abandoned (the SIGTERM grace at the K8s layer is the hard backstop),
        but the factory close is still ordered AFTER this returns.
        """
        if not self._tasks:
            return
        pending = tuple(self._tasks)
        try:
            async with asyncio.timeout(self.drain_deadline_s):
                await asyncio.gather(*pending, return_exceptions=True)
        except TimeoutError:
            survivors = [task for task in pending if not task.done()]
            _log.warning(
                "omni.drain.timeout deadline_s=%s survivors=%d — cancelling before client close",
                self.drain_deadline_s,
                len(survivors),
            )
            for task in survivors:
                task.cancel()
            # Await the cancellation so no survivor is still driving a stream when the factory
            # client closes. Bounded again: a task that ignores cancellation is abandoned, but
            # we never close the client WHILE a run is provably still live.
            try:
                async with asyncio.timeout(self.cancel_grace_s):
                    await asyncio.gather(*survivors, return_exceptions=True)
            except TimeoutError:
                abandoned = sum(1 for task in survivors if not task.done())
                _log.warning(
                    "omni.drain.cancel_timeout grace_s=%s abandoned=%d — shutdown proceeding",
                    self.cancel_grace_s,
                    abandoned,
                )


@dataclass
class Router:
    """The venue-agnostic router's wiring + handler — fully dependency-injected (testable).

    Holds every seam the handler needs (config, trigger adapter, dedup store, session factory,
    venue, redactor, metrics, control plane) + the bg-task tracker. :meth:`handle_webhook` is the
    edge; it is framework-agnostic (takes the raw ``Authorization`` header + the decoded JSON
    body and returns a ``(status, body)`` pair), so it is unit-testable WITHOUT FastAPI/HTTP, and
    :func:`build_app` adapts it to a Starlette route.
    """

    config: RouterConfig
    adapter: TriggerAdapter
    dedup: DedupStore
    session_factory: SessionFactory
    expected_token: str
    venue: Venue
    #: The control plane (PDP) — REQUIRED, no default: a default would have to be either a
    #: fail-open admit-all (the bug this slice removes) or an empty fail-closed table (denies every
    #: trigger). The serve-wiring injects the real declarative-table ControlPlane; a test passes a
    #: trivial double. The handler routes every trigger through ``resolve`` before launch.
    control_plane: ControlPlaneLike
    #: The redaction chokepoint. NO identity default — an identity redactor is a fail-OPEN that
    #: pipes raw investigation text into the venue. The sentinel default fails loud at boot
    #: (__post_init__) so a router wired without a real redactor never starts.
    redact: Callable[[str], str] = _require_redactor
    metrics: Metrics = field(default_factory=_NoopMetrics)
    _tracker: _TaskTracker = field(default_factory=_TaskTracker, init=False, repr=False)

    def __post_init__(self) -> None:
        # Fail CLOSED at boot on a missing redactor (mirrors RouterConfig / load_webhook_token):
        # a router with no redactor would leak raw text into the venue on the first post.
        if self.redact is _require_redactor:
            raise ValueError(
                "Router.redact is required: an unconfigured (identity) redactor would pipe raw "
                "investigation text into the venue. Inject the redaction chokepoint at boot."
            )

    def handle_webhook(
        self, authorization: str | None, body: object
    ) -> tuple[int, Mapping[str, object]]:
        """The venue→router edge: verify → normalize → admit → launch+ack-fast.

        Returns ``(http_status, json_body)``. Never blocks on the investigation — the bg task is
        spawned and the handler acks immediately (the venue's webhook timeout is seconds). Status
        contract:

        * **401** — bad/absent bearer (NOT 5xx: a 5xx invites a retry of an unverifiable body;
          a 401 tells the venue to stop).
        * **200 + ``status: noop``** — verified but NOT admitted: the adapter returned a ``NoOp``
          (non-firing / non-enrolled / unparseable — the page already routed to the human), the
          control-plane denied the trigger (``not_permitted``), or the resolved plan's budget would
          outlive the lease (``lease_floor_violation`` — a static config error, fail-closed so an
          expiring lease cannot double-launch; the ERROR log pages). The ``reason`` carries the
          distinct class (``parse_error`` / ``not_enrolled`` / ``not_permitted`` /
          ``lease_floor_violation``).
        * **200 + ``status: deduped``** — SHED (a run owns the trigger, or the global in-flight
          cap is hit): attach, not queue.
        * **200 + ``status: launched``** — admitted; the investigation runs in the bg.
        * **5xx** — ONLY a fast fail-closed dedup-store error (the one retryable failure).
        """
        if not verify_bearer(authorization, self.expected_token):
            self.metrics.received(verified=False)
            return 401, {"status": "unauthorized"}
        self.metrics.received(verified=True)

        result = self.adapter.parse(body)
        if isinstance(result, NoOp):
            # A non-matching event — answer a no-op, not an error (the page already routed to the
            # human; a 5xx would re-deliver bad bytes). The NoOp carries its reason so the two
            # non-match classes stay distinct on-call signals: a parse_error storm (malformed
            # bodies) and a not_enrolled flood do NOT collapse into one undifferentiated metric.
            self.metrics.admitted(decision=result.reason)
            return 200, {"status": "noop", "reason": result.reason}
        trigger = result

        # The control-plane (PDP): fail-closed skill/venue/posture gating BEFORE launch. A deny is
        # a 200-noop (the page already routed to the human; a 5xx would re-deliver), carrying a
        # DISTINCT reason (not_permitted) alongside parse_error/not_enrolled so a denial flood is
        # separable on-call. The allowed plan's per-trigger budget is what the run is supervised
        # under (replacing the boot-time budget); the bundle/model levers ride on the context for
        # the later session-application slice.
        plan = self.control_plane.resolve(trigger)
        if not plan.allowed:
            self.metrics.admitted(decision=plan.deny_reason)
            _log.info(
                "omni.admit.denied dedup_key=%s reason=%s", trigger.dedup_key, plan.deny_reason
            )
            return 200, {"status": "noop", "reason": plan.deny_reason}

        # C1 claim-time guard (the run runs under plan.budget, not config.budget): a run whose
        # budget wall-clock + the lease margin exceeds the lease would expire mid-run and let a
        # re-fire double-launch (2x session, 2x cost). The boot guard (ControlPlane /
        # RouterConfig __post_init__) catches this at deploy for the wired table; this is the
        # runtime backstop on plan.budget itself, so a plan from an un-boot-validated source (a
        # dynamic table) cannot slip a lease-underrunning run past the claim. It is a STATIC
        # config error (not attacker input, not retry-fixable), so fail closed as a 200-noop (a
        # distinct reason, the parse_error/not_permitted no-op style — a 5xx would invite an AM
        # retry storm) and do NOT claim/launch. The ERROR log is the page; the metric is bounded.
        lease_floor = plan.budget.wall_clock_s + self.config.min_lease_margin_s
        if lease_floor > self.config.lease_s:
            self.metrics.admitted(decision="lease_floor_violation")
            _log.error(
                "omni.admit.lease_floor_violation dedup_key=%s budget_wall_clock_s=%s "
                "min_lease_margin_s=%s lease_s=%s — NOT launching (an expiring lease would "
                "double-launch this incident); fix the route budget/lease config",
                trigger.dedup_key,
                plan.budget.wall_clock_s,
                self.config.min_lease_margin_s,
                self.config.lease_s,
            )
            return 200, {"status": "noop", "reason": "lease_floor_violation"}

        # Global back-pressure: at the cap, SHED a fresh trigger rather than growing the task set
        # + host load unbounded. Checked BEFORE the claim so a shed does not consume a claim slot.
        # (Per-trigger dedup below is the first, finer shed.)
        if self._tracker.in_flight() >= self.config.max_in_flight:
            self.metrics.admitted(decision="shed_capacity")
            return 200, {"status": "deduped", "reason": "at_capacity"}

        run_id = uuid.uuid4().hex
        try:
            claimed = self.dedup.claim_run(
                trigger.dedup_key, run_id, lease_s=self.config.lease_s
            )
        except Exception:
            # A dedup-store error is the ONE fast fail-closed 5xx: we cannot safely admit without
            # the single-flight guarantee, and the store error IS retryable (unlike a bad body),
            # so let the venue retry. Never launch without a won claim.
            self.metrics.admitted(decision="store_error")
            _log.exception("omni.admit.store_error dedup_key=%s", trigger.dedup_key)
            return 503, {"status": "error", "reason": "dedup_store_unavailable"}

        if admit_run(incident_in_flight=not claimed) is RunAdmission.SHED:
            # A run already owns this trigger — attach, not queue. The existing run's post-back
            # covers it.
            self.metrics.admitted(decision=RunAdmission.SHED.value)
            return 200, {"status": "deduped", "reason": "in_flight"}

        ctx = RunContext(
            dedup_key=trigger.dedup_key,
            venue_handle=trigger.venue_handle,
            run_id=run_id,
            goal=trigger.goal,
            payload=trigger.payload,
            bundle_ref=plan.bundle_ref,
            model_override=plan.model_override,
            reasoning_effort=plan.reasoning_effort,
        )
        self._tracker.spawn(
            supervise_run(
                ctx,
                budget=plan.budget,
                session_factory=self.session_factory,
                dedup=self.dedup,
                venue=self.venue,
                redact=self.redact,
                now=self.config.now,
                metrics=self.metrics,
            )
        )
        return 200, {
            "status": "launched",
            "dedup_key": trigger.dedup_key,
            "run_id": run_id,
        }

    async def drain(self) -> None:
        """Await in-flight investigations on shutdown (called from the app lifespan)."""
        await self._tracker.drain()

    async def aclose(self) -> None:
        """Release owned resources (the venue's + the session factory's HTTP pools) on shutdown.

        Drains first (in the lifespan) so in-flight investigations can post + finish; then this
        closes the venue client and — if the session factory owns a closeable resource (the live
        factory holds a standing omnigent client pool) — closes that too, so no minted keep-alive
        connection pool is leaked on a clean stop. A factory with no ``aclose`` (the test fake) is
        skipped. The factory closes AFTER the venue, both after the drain, so the last in-flight
        investigation can still both reach its session and post its result.
        """
        await self.venue.aclose()
        factory_aclose = getattr(self.session_factory, "aclose", None)
        if callable(factory_aclose):
            await factory_aclose()


def build_app(router: Router) -> object:
    """Adapt a :class:`Router` to a Starlette app with the ``/webhook`` route + lifespan.

    The HTTP framing only — the decision logic is :meth:`Router.handle_webhook` (which is
    framework-free + unit-testable without this). The lifespan drains in-flight investigations on
    shutdown (structured concurrency: no orphaned run, no lost post-back). ``/healthz`` is a
    liveness probe. FastAPI/starlette are imported lazily (inside this builder) so the pure
    handler + its unit tests need no web framework on the import path.

    The route reads the raw ``Request`` itself (rather than a typed body model) so the bearer is
    verified BEFORE any body parsing — the decision logic (auth-first, then parse) lives wholly
    in :meth:`Router.handle_webhook`, not in a request model.
    """
    from contextlib import asynccontextmanager  # noqa: PLC0415 -- deferred (web-glue only)

    from starlette.applications import Starlette  # noqa: PLC0415 -- deferred web glue
    from starlette.requests import Request  # noqa: PLC0415
    from starlette.responses import JSONResponse  # noqa: PLC0415
    from starlette.routing import Route  # noqa: PLC0415

    @asynccontextmanager
    async def lifespan(app: object):
        yield
        # Drain on shutdown so an in-flight investigation finishes (and posts) rather than being
        # torn down mid-run — the SIGTERM grace window bounds it at the K8s layer — THEN close the
        # venue's HTTP pool (after the drain, so the last posts can still go out).
        await router.drain()
        await router.aclose()

    async def webhook(request: Request) -> JSONResponse:
        authorization = request.headers.get("authorization")
        # Body-bomb guard: read the RAW bytes with a hard cap BEFORE JSON-parsing, since the body
        # is buffered before the bearer can be trusted (an unauthenticated client can still stream
        # at us). A Content-Length over the cap is rejected outright; absent/lying that, the
        # streamed total is capped too. 413, not 5xx — it is a client fault.
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
            # A non-JSON body verified-or-not is handled the same as a non-match AFTER auth; but
            # auth must run first, so hand the handler a non-mapping so the adapter no-ops it (it
            # owns the noop policy), NOT 5xx.
            body = None
        status, payload = router.handle_webhook(authorization, body)
        return JSONResponse(status_code=status, content=dict(payload))

    async def healthz(request: Request) -> JSONResponse:
        return JSONResponse({"status": "ok", "in_flight": router._tracker.in_flight()})

    return Starlette(
        lifespan=lifespan,
        routes=[
            Route("/webhook", webhook, methods=["POST"]),
            Route("/healthz", healthz, methods=["GET"]),
        ],
    )
