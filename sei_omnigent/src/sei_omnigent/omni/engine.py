"""Pure goal+guardrail engine core for the omni-overlay (PLT-716) — no omnigent import.

The reliability core of a headless run, expressed as pure decision functions so it
is unit-testable in isolation (mirrors ``_posture.py`` / ``profile.py``). The
omnigent-touching wiring — the ``/goal`` Stop hook, the session loop, the trace
sink, and the dedup/lock store — lives in the launch wrapper; this module holds
only the *policy*, which is what must be provably correct.

Four concerns, all pure:

1. **Terminal model** — the bounded :class:`TerminalReason` the Stop hook *reads*
   (it does not re-derive done-ness — the review-gate's "read the ledger, never
   re-derive from the transcript", applied to termination), plus the
   truncated-vs-surveyed distinction §3.5 makes load-bearing: a budget-exhausted
   run must NOT borrow the clean-punt "all-clear" headline.
2. **Budget** — the four-axis budget vector (wall-clock · token · query-count with
   per-source sub-caps · max-iterations) + a no-progress detector → a terminal.
3. **Degradation** — fail-closed source classification: unreachable/stale/empty/
   partial ⇒ ``inconclusive``, never a silent healthy-assumption (the guard
   primitive's spine; empty ≠ healthy).
4. **At-most-once** — the single-flight admission policy keyed on the PagerDuty
   ``incident_key`` (attach-not-queue; a storm sheds; one post per incident at the
   post chokepoint), with cross-crash semantics resting on attach-not-queue +
   fresh-``run_id``-supersedes.

Design 12 §2, §3.5.
"""

from __future__ import annotations

import math
from collections.abc import Mapping
from dataclasses import dataclass
from enum import StrEnum
from types import MappingProxyType


class TerminalReason(StrEnum):
    """Why a headless run stopped — a bounded enum the Stop hook reads and the output renders.

    Every terminal posts the partial artifact + halts; a run never silent-dies.
    ``GOAL_REACHED`` / ``CLEAN_PUNT`` are *surveyed* terminals (the run did its job);
    ``INSUFFICIENT_CONTEXT`` is a clean pre-investigation halt; ``BUDGET_EXHAUSTED`` /
    ``NO_PROGRESS`` are *truncated* (cut short) — and §3.5 forbids a truncated run
    from rendering the clean-punt all-clear headline, so the distinction is
    load-bearing for the misleading-rate SLO (see :func:`is_truncated`).
    ``ERRORED`` is a run that could not run or died before completing (a session-create
    or transport failure, the watchdog firing) — it is NOT a surveyed terminal and NOT a
    budget cut: its note must read as an honest "could not run", carrying the failure
    detail, never an all-clear and never a budget-axis line.

    Deliberately out of scope for Phase-1: a **gate-halt** terminal (the INV-1
    HALT-and-report state — "absence of a human is a NO"). Report-only never crosses
    a one-way-door gate, so the branch never fires (Design 12 §2); the day a
    build-mode profile lands it is load-bearing and a gate-halt terminal must be
    added (and must NOT be a surveyed terminal — it is not an all-clear). Recorded
    here so a future build-mode author adds it rather than shoehorning a halt into
    ``CLEAN_PUNT``/``INSUFFICIENT_CONTEXT``.
    """

    GOAL_REACHED = "goal-reached"  # structurally-complete, evidence-backed report
    CLEAN_PUNT = "clean-punt"  # surveyed: no confident candidate, here's what was ruled out
    INSUFFICIENT_CONTEXT = "insufficient-context"  # cannot localize; needs more context
    BUDGET_EXHAUSTED = "budget-exhausted"  # truncated at a budget axis
    NO_PROGRESS = "no-progress"  # truncated: no new evidence/hypothesis movement
    ERRORED = "errored"  # the run could not run / failed before completing (transport/create
    # failure, watchdog) — distinct from a budget cut; carries the exception detail, not an axis


# Truncated terminals must not present as a surveyed clean-punt (§3.5). A run that
# stopped here surveyed only *part* of the space; saying "all clear" would mislead.
_TRUNCATED: frozenset[TerminalReason] = frozenset(
    {TerminalReason.BUDGET_EXHAUSTED, TerminalReason.NO_PROGRESS}
)


def is_truncated(reason: TerminalReason) -> bool:
    """True if the run was cut short (budget/no-progress) rather than surveying its space.

    The output renderer uses this to refuse the clean-punt "all-clear" headline for
    a truncated run (§3.5) — a truncated run ≠ a surveyed clean punt.
    """
    return reason in _TRUNCATED


@dataclass(frozen=True)
class Budget:
    """The four-axis budget caps + the no-progress window. Caps are hard ceilings.

    A non-positive cap would make the run terminate before doing any work, which is
    a misconfiguration, not a valid "no budget" — so every cap must be positive.
    ``per_source_queries`` sub-caps a named source's query count *within* the
    aggregate ``queries`` ceiling (a source absent from the map is bounded only by
    the aggregate).
    """

    wall_clock_s: float
    tokens: int
    queries: int
    per_source_queries: Mapping[str, int]
    max_iterations: int
    no_progress_iterations: int

    def __post_init__(self) -> None:
        caps = {
            "wall_clock_s": self.wall_clock_s,
            "tokens": self.tokens,
            "queries": self.queries,
            "max_iterations": self.max_iterations,
            "no_progress_iterations": self.no_progress_iterations,
        }
        # Finiteness FIRST: a NaN/inf cap passes a "<= 0" test (nan<=0 and inf<=0 are
        # both False) and would silently DISABLE that budget axis — a fail-open hole
        # in the one module whose job is to bound a run against a degraded system.
        non_finite = [name for name, value in caps.items() if not math.isfinite(value)]
        if non_finite:
            raise ValueError(
                f"Budget caps must be finite; got non-finite: {', '.join(non_finite)}"
            )
        non_positive = [name for name, value in caps.items() if value <= 0]
        if non_positive:
            raise ValueError(
                f"Budget caps must be positive; got non-positive: {', '.join(non_positive)}"
            )
        bad_sub = [
            src for src, cap in self.per_source_queries.items()
            if not math.isfinite(cap) or cap <= 0
        ]
        if bad_sub:
            raise ValueError(
                f"Budget per-source caps must be finite + positive: {', '.join(bad_sub)}"
            )
        # Defensive freeze: a caller mutating the dict it passed in must not change a
        # validated cap under the frozen dataclass (mirrors profile.py's _freeze).
        object.__setattr__(
            self, "per_source_queries", MappingProxyType(dict(self.per_source_queries))
        )


@dataclass(frozen=True)
class Usage:
    """A snapshot of consumption the engine evaluates against a :class:`Budget`."""

    elapsed_s: float
    tokens: int
    queries: int
    per_source_queries: Mapping[str, int]
    iterations: int
    iterations_since_progress: int

    def __post_init__(self) -> None:
        # Non-finite usage is the symmetric fail-open of a non-finite cap: a NaN
        # elapsed_s makes `elapsed_s >= wall_clock_s` False forever, so the axis
        # never trips. Reject it (negative under-reports and trips late — less
        # dangerous, but a finite-and-non-negative snapshot is the only honest one).
        numerics = {
            "elapsed_s": self.elapsed_s,
            "tokens": self.tokens,
            "queries": self.queries,
            "iterations": self.iterations,
            "iterations_since_progress": self.iterations_since_progress,
        }
        bad = [n for n, v in numerics.items() if not math.isfinite(v) or v < 0]
        # Per-source counts get the same guard: a NaN/negative count makes
        # `get(source, 0) >= cap` permanently False, failing that sub-cap OPEN while
        # the rest of the engine stays fail-closed.
        bad += [
            f"per_source_queries[{s!r}]"
            for s, v in self.per_source_queries.items()
            if not math.isfinite(v) or v < 0
        ]
        if bad:
            raise ValueError(
                f"Usage values must be finite and non-negative; bad for: {', '.join(bad)}"
            )
        object.__setattr__(
            self, "per_source_queries", MappingProxyType(dict(self.per_source_queries))
        )


_NO_PROGRESS_AXIS = "no-progress"


def tripped_axis(budget: Budget, usage: Usage) -> str | None:
    """Name the FIRST budget axis a usage snapshot has hit, or ``None`` if within budget.

    The output renderer uses this for §3.5's "investigation truncated at budget;
    unsurveyed: X" line — the axis name (``wall-clock`` / ``tokens`` / ``queries`` /
    ``queries:<source>`` / ``iterations`` / ``no-progress``) IS X. Deriving it here,
    from the same ``(Budget, Usage)`` the Stop hook reads, keeps the renderer from
    re-deriving done-ness (the §2 "read, don't re-derive" discipline) — and keeps
    :func:`budget_terminal` and the rendered axis from ever disagreeing.

    Precondition (wrapper contract): an uncapped source is bounded only by the
    aggregate ``queries``, so the wrapper MUST count every source's queries into
    ``usage.queries`` (the per-source map and the aggregate are independent fields
    the core cannot cross-check).
    """
    if usage.elapsed_s >= budget.wall_clock_s:
        return "wall-clock"
    if usage.tokens >= budget.tokens:
        return "tokens"
    if usage.queries >= budget.queries:
        return "queries"
    for source, cap in budget.per_source_queries.items():
        if usage.per_source_queries.get(source, 0) >= cap:
            return f"queries:{source}"
    if usage.iterations >= budget.max_iterations:
        return "iterations"
    if usage.iterations_since_progress >= budget.no_progress_iterations:
        return _NO_PROGRESS_AXIS
    return None


def budget_terminal(budget: Budget, usage: Usage) -> TerminalReason | None:
    """Return the terminal a usage snapshot has hit, or ``None`` if within budget.

    A hit on any hard axis (wall-clock, token, aggregate or per-source query count,
    iteration ceiling) is ``BUDGET_EXHAUSTED``; budget takes precedence over the
    no-progress detector. Reaching the no-progress window is ``NO_PROGRESS``. Both
    are *truncated* terminals (:func:`is_truncated`). Comparison is ``>=`` (the cap
    is a ceiling the run must not cross, not exceed-by-one). Shares its logic with
    :func:`tripped_axis` so the terminal and the rendered axis never disagree.
    """
    axis = tripped_axis(budget, usage)
    if axis is None:
        return None
    if axis == _NO_PROGRESS_AXIS:
        return TerminalReason.NO_PROGRESS
    return TerminalReason.BUDGET_EXHAUSTED


class SourceOutcome(StrEnum):
    """The fail-closed classification of a single insight-source read.

    ``INCONCLUSIVE`` means "could not confirm" — it is recorded as a couldn't-check
    surviving-uncertainty, never folded into a healthy assumption. Only a reachable,
    complete, fresh, non-empty read is ``OK``.
    """

    OK = "ok"
    INCONCLUSIVE = "inconclusive"


def classify_source_read(
    *,
    reachable: bool,
    complete: bool,
    stale: bool,
    empty: bool,
) -> SourceOutcome:
    """Classify an insight-source read fail-closed.

    Any of unreachable / incomplete (partial-response ``warnings``) / stale / empty
    ⇒ ``INCONCLUSIVE``. ``empty`` is deliberately inconclusive, not healthy: an empty
    metric series or log window is "couldn't observe", not "observed nothing wrong"
    (the guard primitive's empty ≠ healthy). Never concludes on data it cannot
    confirm.

    Caller obligation (the fail-closed guarantee is conditional on honest inputs):
    the caller MUST set ``complete=False`` on ANY partial / error-envelope /
    parse-failure / timeout / auth-error / rate-limited read — i.e. every "couldn't
    fully confirm" mode collapses onto these four flags (a 200 wrapping an error is
    *not* ``complete``). ``empty`` must reflect the post-parse result set, not HTTP
    success. The dead-source backoff / circuit-breaker (so an ``INCONCLUSIVE`` source
    isn't hammered, consuming the query budget — §2 per-source bulkhead) is the
    wrapper's job; this function only classifies a single read.
    """
    if not reachable or not complete or stale or empty:
        return SourceOutcome.INCONCLUSIVE
    return SourceOutcome.OK


class RunAdmission(StrEnum):
    """Whether a fresh trigger for a PagerDuty incident may start a run.

    ``SHED`` is attach-not-queue: when a run already owns the ``incident_key``, the
    new trigger is dropped (the existing run covers the incident), never enqueued —
    a storm of re-fires sheds rather than building an unbounded backlog.
    """

    PROCEED = "proceed"
    SHED = "shed"


def admit_run(*, incident_in_flight: bool) -> RunAdmission:
    """Single-flight admission keyed on the PagerDuty ``incident_key``.

    ``SHED`` if a run is already in flight for this incident (attach-not-queue);
    else ``PROCEED``. Cross-crash at-most-once rests on this plus
    fresh-``run_id``-supersedes: a crashed run is no longer in flight, so a later
    trigger correctly ``PROCEED``s from scratch under a new ``run_id`` — and
    :func:`admit_post` is the chokepoint that still prevents a double post.

    **Atomicity precondition (the guarantee is the caller's, not this predicate's):**
    this is a pure decode of an *already-read* in-flight bit. The caller MUST read
    the in-flight state AND claim it (set in-flight for this ``incident_key``) under
    a single atomic step (compare-and-set / conditional write / per-key lock) — a
    non-atomic check-then-act lets two concurrent triggers both observe
    ``incident_in_flight=False`` and both ``PROCEED``, voiding single-flight.
    **The in-flight claim MUST be leased / TTL'd** (its counterpart to attach-not-
    queue): a claim that is never released on crash wedges the incident permanently
    in ``SHED`` — even legitimate retries shed. A bare boolean cannot provide either
    property; the dedup/lock store (wrapper) must.
    """
    return RunAdmission.SHED if incident_in_flight else RunAdmission.PROCEED


def admit_post(*, incident_already_posted: bool) -> bool:
    """The post chokepoint: post at most once per incident.

    Enforced at the single egress chokepoint (the same place redaction and the
    kill-switch act). Returns ``True`` only if nothing has been posted for this
    incident yet — so a re-run after a crash, or a within-run post retry, cannot
    emit a second note for the same incident.

    **Atomicity precondition:** same as :func:`admit_run` — the caller MUST commit
    "posted" for this ``incident_key`` and emit the note as one atomic transition
    (record-then-post under a per-key guard, or an idempotent conditional post). A
    crash *between* this returning ``True`` and the record landing re-opens the
    double-post window; the wrapper closes it (e.g. write-intent-then-post, or a
    post idempotency key). A pure boolean cannot.
    """
    return not incident_already_posted
