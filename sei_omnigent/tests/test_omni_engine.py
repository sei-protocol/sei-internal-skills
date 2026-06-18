"""Tests for the goal+guardrail engine core (PLT-716).

Pure, omnigent-free (mirrors test_omni_profile). Covers the terminal model +
truncated distinction (§3.5), the four-axis budget + no-progress detector,
fail-closed source degradation, and the single-flight at-most-once policy (§2).
"""

from __future__ import annotations

import pytest

from sei_omnigent.omni import (
    Budget,
    RunAdmission,
    SourceOutcome,
    TerminalReason,
    Usage,
    admit_post,
    admit_run,
    budget_terminal,
    classify_source_read,
    is_truncated,
)


def _budget(**over: object) -> Budget:
    base: dict[str, object] = {
        "wall_clock_s": 300.0,
        "tokens": 100_000,
        "queries": 40,
        "per_source_queries": {"thanos": 20, "loki": 20},
        "max_iterations": 2,
        "no_progress_iterations": 2,
    }
    base.update(over)
    return Budget(**base)  # type: ignore[arg-type]


def _usage(**over: object) -> Usage:
    base: dict[str, object] = {
        "elapsed_s": 0.0,
        "tokens": 0,
        "queries": 0,
        "per_source_queries": {},
        "iterations": 0,
        "iterations_since_progress": 0,
    }
    base.update(over)
    return Usage(**base)  # type: ignore[arg-type]


# --- terminal model + truncated distinction (§3.5) ----------------------------


def test_truncated_terminals_are_budget_and_no_progress() -> None:
    assert is_truncated(TerminalReason.BUDGET_EXHAUSTED)
    assert is_truncated(TerminalReason.NO_PROGRESS)


@pytest.mark.parametrize("reason", [
    TerminalReason.GOAL_REACHED,
    TerminalReason.CLEAN_PUNT,
    TerminalReason.INSUFFICIENT_CONTEXT,
])
def test_surveyed_terminals_are_not_truncated(reason: TerminalReason) -> None:
    # §3.5: a surveyed terminal may carry the clean-punt/all-clear headline; a
    # truncated one may not. This is the load-bearing distinction.
    assert not is_truncated(reason)


# --- budget: within, and each axis ---------------------------------------------


def test_within_budget_returns_none() -> None:
    usage = _usage(elapsed_s=10.0, tokens=5, queries=3, iterations=1)
    assert budget_terminal(_budget(), usage) is None


@pytest.mark.parametrize("usage_over", [
    {"elapsed_s": 300.0},          # wall-clock ceiling (>= is a hit)
    {"tokens": 100_000},           # token ceiling
    {"queries": 40},               # aggregate query ceiling
    {"iterations": 2},             # iteration ceiling
])
def test_each_hard_axis_trips_budget_exhausted(usage_over: dict[str, object]) -> None:
    assert budget_terminal(_budget(), _usage(**usage_over)) is TerminalReason.BUDGET_EXHAUSTED


def test_per_source_subcap_trips_even_under_aggregate() -> None:
    # thanos hits its 20 sub-cap while aggregate (21) is under the 40 ceiling.
    usage = _usage(queries=21, per_source_queries={"thanos": 20, "loki": 1})
    assert budget_terminal(_budget(), usage) is TerminalReason.BUDGET_EXHAUSTED


def test_unknown_source_is_bounded_only_by_aggregate() -> None:
    # a source with no sub-cap does not trip per-source; aggregate still governs.
    usage = _usage(queries=5, per_source_queries={"events": 999})
    assert budget_terminal(_budget(), usage) is None


def test_no_progress_trips_after_budget_is_clear() -> None:
    usage = _usage(iterations_since_progress=2)
    assert budget_terminal(_budget(), usage) is TerminalReason.NO_PROGRESS


def test_budget_takes_precedence_over_no_progress() -> None:
    usage = _usage(tokens=100_000, iterations_since_progress=99)
    assert budget_terminal(_budget(), usage) is TerminalReason.BUDGET_EXHAUSTED


# --- budget validation ---------------------------------------------------------


@pytest.mark.parametrize("bad", [
    {"wall_clock_s": 0.0},
    {"tokens": 0},
    {"queries": -1},
    {"max_iterations": 0},
    {"no_progress_iterations": 0},
])
def test_non_positive_caps_are_rejected(bad: dict[str, object]) -> None:
    with pytest.raises(ValueError, match="must be positive"):
        _budget(**bad)


def test_non_positive_per_source_cap_is_rejected() -> None:
    with pytest.raises(ValueError, match="per-source caps must be positive"):
        _budget(per_source_queries={"thanos": 0})


# --- fail-closed source degradation --------------------------------------------


def test_healthy_read_is_ok() -> None:
    outcome = classify_source_read(reachable=True, complete=True, stale=False, empty=False)
    assert outcome is SourceOutcome.OK


@pytest.mark.parametrize("bad", [
    {"reachable": False},
    {"complete": False},   # partial-response warnings
    {"stale": True},
    {"empty": True},       # empty != healthy
])
def test_degraded_reads_are_inconclusive(bad: dict[str, object]) -> None:
    kwargs = {"reachable": True, "complete": True, "stale": False, "empty": False}
    kwargs.update(bad)
    assert classify_source_read(**kwargs) is SourceOutcome.INCONCLUSIVE  # type: ignore[arg-type]


# --- at-most-once: single-flight admission + post chokepoint -------------------


def test_admit_run_proceeds_when_no_run_in_flight() -> None:
    assert admit_run(incident_in_flight=False) is RunAdmission.PROCEED


def test_admit_run_sheds_when_incident_already_in_flight() -> None:
    # attach-not-queue: a re-fire/storm sheds, never enqueues.
    assert admit_run(incident_in_flight=True) is RunAdmission.SHED


def test_admit_post_allows_first_post_only() -> None:
    assert admit_post(incident_already_posted=False) is True
    assert admit_post(incident_already_posted=True) is False
