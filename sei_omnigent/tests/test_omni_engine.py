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
    tripped_axis,
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


def test_errored_is_a_distinct_terminal_never_emitted_by_budget() -> None:
    # ERRORED is a run that could not run / died before completing — distinct from a budget
    # cut. budget_terminal never produces it (a budget breach is BUDGET_EXHAUSTED/NO_PROGRESS);
    # the driver/supervisor classify it directly. It is NOT in the is_truncated set (the §3.5
    # refusal of an all-clear rides on the outcome's explicit truncated flag + render_note's
    # ERRORED branch, not on enum membership) — but it is its own enum member.
    assert TerminalReason.ERRORED not in {
        TerminalReason.BUDGET_EXHAUSTED,
        TerminalReason.NO_PROGRESS,
    }
    assert not is_truncated(TerminalReason.ERRORED)
    assert TerminalReason.ERRORED.value == "errored"


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
    with pytest.raises(ValueError, match="per-source caps"):
        _budget(per_source_queries={"thanos": 0})


def test_non_finite_caps_are_rejected() -> None:
    # nan/inf pass a "<= 0" test but disable the axis (nan>=cap is always False) —
    # reject at construction. (Budget precision is best-effort; this just prevents
    # a silently-disabled axis, not a tracking guarantee.)
    inf = float("inf")
    nan = float("nan")
    for bad in ({"wall_clock_s": inf}, {"wall_clock_s": nan}):
        with pytest.raises(ValueError, match="must be finite"):
            _budget(**bad)


def test_loaded_budget_caps_are_isolated_from_source_mutation() -> None:
    caps = {"thanos": 20, "loki": 20}
    b = _budget(per_source_queries=caps)
    caps["thanos"] = 0  # mutate the caller's dict after construction
    assert b.per_source_queries["thanos"] == 20  # validated cap is frozen
    with pytest.raises(TypeError):
        b.per_source_queries["thanos"] = 1  # type: ignore[index]  # read-only proxy


def test_usage_rejects_non_finite_and_negative() -> None:
    with pytest.raises(ValueError, match="finite and non-negative"):
        _usage(elapsed_s=float("nan"))
    with pytest.raises(ValueError, match="finite and non-negative"):
        _usage(queries=-1)


def test_usage_rejects_bad_per_source_count() -> None:
    # a NaN/negative per-source count would fail that sub-cap OPEN (get>=cap False forever)
    with pytest.raises(ValueError, match="per_source_queries"):
        _usage(per_source_queries={"thanos": float("nan")})
    with pytest.raises(ValueError, match="per_source_queries"):
        _usage(per_source_queries={"thanos": -1})


# --- tripped_axis: names the unsurveyed axis for the §3.5 output ---------------


def test_tripped_axis_none_within_budget() -> None:
    assert tripped_axis(_budget(), _usage(queries=3)) is None


@pytest.mark.parametrize("usage_over,axis", [
    ({"elapsed_s": 300.0}, "wall-clock"),
    ({"tokens": 100_000}, "tokens"),
    ({"queries": 40}, "queries"),
    ({"iterations": 2}, "iterations"),
    ({"queries": 21, "per_source_queries": {"thanos": 20}}, "queries:thanos"),
    ({"iterations_since_progress": 2}, "no-progress"),
])
def test_tripped_axis_names_the_hit_axis(usage_over: dict[str, object], axis: str) -> None:
    assert tripped_axis(_budget(), _usage(**usage_over)) == axis


def test_per_source_axis_reported_before_iterations_when_both_hit() -> None:
    # documented order puts queries:<source> ahead of iterations — naming the
    # exhausted bulkhead is more informative than "iterations" for §3.5's line.
    usage = _usage(queries=21, per_source_queries={"thanos": 20}, iterations=2)
    assert tripped_axis(_budget(), usage) == "queries:thanos"


def test_tripped_axis_and_budget_terminal_never_disagree() -> None:
    # both derive from the same logic; a no-progress axis ⇒ NO_PROGRESS, any other ⇒
    # BUDGET_EXHAUSTED, and None ⇒ None.
    for usage_over in ({"tokens": 100_000}, {"iterations_since_progress": 2}, {"queries": 1}):
        u = _usage(**usage_over)
        axis = tripped_axis(_budget(), u)
        term = budget_terminal(_budget(), u)
        if axis is None:
            assert term is None
        elif axis == "no-progress":
            assert term is TerminalReason.NO_PROGRESS
        else:
            assert term is TerminalReason.BUDGET_EXHAUSTED


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
