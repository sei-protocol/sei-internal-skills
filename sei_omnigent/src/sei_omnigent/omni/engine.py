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

from collections.abc import Mapping
from dataclasses import dataclass
from enum import StrEnum


class TerminalReason(StrEnum):
    """Why a headless run stopped — a bounded enum the Stop hook reads and the output renders.

    Every terminal posts the partial artifact + halts; a run never silent-dies.
    ``GOAL_REACHED`` / ``CLEAN_PUNT`` are *surveyed* terminals (the run did its job);
    ``INSUFFICIENT_CONTEXT`` is a clean pre-investigation halt; ``BUDGET_EXHAUSTED`` /
    ``NO_PROGRESS`` are *truncated* (cut short) — and §3.5 forbids a truncated run
    from rendering the clean-punt all-clear headline, so the distinction is
    load-bearing for the misleading-rate SLO (see :func:`is_truncated`).
    """

    GOAL_REACHED = "goal-reached"  # structurally-complete, evidence-backed report
    CLEAN_PUNT = "clean-punt"  # surveyed: no confident candidate, here's what was ruled out
    INSUFFICIENT_CONTEXT = "insufficient-context"  # cannot localize; needs more context
    BUDGET_EXHAUSTED = "budget-exhausted"  # truncated at a budget axis
    NO_PROGRESS = "no-progress"  # truncated: no new evidence/hypothesis movement


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
        positive = {
            "wall_clock_s": self.wall_clock_s,
            "tokens": self.tokens,
            "queries": self.queries,
            "max_iterations": self.max_iterations,
            "no_progress_iterations": self.no_progress_iterations,
        }
        non_positive = [name for name, value in positive.items() if value <= 0]
        if non_positive:
            raise ValueError(
                f"Budget caps must be positive; got non-positive: {', '.join(non_positive)}"
            )
        bad_sub = [src for src, cap in self.per_source_queries.items() if cap <= 0]
        if bad_sub:
            raise ValueError(
                f"Budget per-source caps must be positive; non-positive for: {', '.join(bad_sub)}"
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


def _per_source_exceeded(budget: Budget, usage: Usage) -> bool:
    """True if any sub-capped source has met or exceeded its per-source cap."""
    return any(
        usage.per_source_queries.get(source, 0) >= cap
        for source, cap in budget.per_source_queries.items()
    )


def budget_terminal(budget: Budget, usage: Usage) -> TerminalReason | None:
    """Return the terminal a usage snapshot has hit, or ``None`` if within budget.

    A hit on any hard axis (wall-clock, token, aggregate or per-source query count,
    iteration ceiling) is ``BUDGET_EXHAUSTED``; budget takes precedence over the
    no-progress detector. Reaching the no-progress window is ``NO_PROGRESS``. Both
    are *truncated* terminals (:func:`is_truncated`). Comparison is ``>=`` (the cap
    is a ceiling the run must not cross, not exceed-by-one).
    """
    if (
        usage.elapsed_s >= budget.wall_clock_s
        or usage.tokens >= budget.tokens
        or usage.queries >= budget.queries
        or usage.iterations >= budget.max_iterations
        or _per_source_exceeded(budget, usage)
    ):
        return TerminalReason.BUDGET_EXHAUSTED
    if usage.iterations_since_progress >= budget.no_progress_iterations:
        return TerminalReason.NO_PROGRESS
    return None


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
    """
    return RunAdmission.SHED if incident_in_flight else RunAdmission.PROCEED


def admit_post(*, incident_already_posted: bool) -> bool:
    """The post chokepoint: post at most once per incident.

    Enforced at the single egress chokepoint (the same place redaction and the
    kill-switch act). Returns ``True`` only if nothing has been posted for this
    incident yet — so a re-run after a crash, or a within-run post retry, cannot
    emit a second note for the same incident.
    """
    return not incident_already_posted
