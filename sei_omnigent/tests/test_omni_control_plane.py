"""Tests for the fail-closed ControlPlane (the PDP) — Design 13.

The resolution-table decisions are pure (no omnigent / no live host): an allowed entry resolves
to a RunPlan; an unknown (venue, locus, trigger_kind) or an ungranted skill fails closed
(deny-by-default); a write-posture entry is rejected at table-construction (the MVP floor,
INV-7'). The PD-dogfood route resolves to an allowed plan end-to-end against the same adapter the
serve-wiring uses. The per-human initiator gate is a documented seam, NOT built this slice — its
placeholder test is skip-marked (gated on the Blocking dependency).
"""

from __future__ import annotations

import pytest

from sei_omnigent.omni.adapters.alertmanager import (
    _AM_LOCUS,
    _AM_ROOT_CAUSE_SKILL,
    _AM_TRIGGER_KIND,
    _AM_VENUE,
    AlertmanagerAdapter,
)
from sei_omnigent.omni.adapters.base import NormalizedTrigger
from sei_omnigent.omni.control_plane import (
    DENY_NOT_PERMITTED,
    ControlPlane,
    Posture,
    RunPlan,
    TableEntry,
)
from sei_omnigent.omni.engine import Budget


def _budget() -> Budget:
    return Budget(
        wall_clock_s=900.0, tokens=400_000, queries=1_000, per_source_queries={},
        max_iterations=40, no_progress_iterations=6,
    )


def _entry(**over: object) -> TableEntry:
    base: dict = {
        "bundle_ref": "root-cause",
        "skills": frozenset({"root-cause"}),
        "posture": Posture.PROPOSE_ONLY,
        "budget": _budget(),
    }
    base.update(over)
    return TableEntry(**base)


def _trigger(**over: object) -> NormalizedTrigger:
    base: dict = {
        "initiator": "system:alertmanager",
        "goal": "g",
        "venue": "pagerduty",
        "locus": "alertmanager",
        "trigger_kind": "alert",
        "venue_handle": "inc-1",
        "dedup_key": "inc-1",
        "trust": "system",
        "requested_skills": ("root-cause",),
    }
    base.update(over)
    return NormalizedTrigger(**base)


def _table() -> dict:
    return {("pagerduty", "alertmanager", "alert"): _entry()}


# --- boot validation (fail-loud on an empty/malformed table) ------------------


def test_empty_table_fails_loud_at_construction() -> None:
    # An empty PDP table denies every trigger — a silent total outage. Fail CLOSED at boot
    # (mirrors RouterConfig / Budget), not at the first request.
    with pytest.raises(ValueError, match="empty"):
        ControlPlane(table={})


def test_write_posture_entry_is_rejected_at_construction() -> None:
    # The MVP floor selects only read postures (INV-7'); a write-posture entry is a config error
    # the table rejects at boot, never a per-request deny.
    with pytest.raises(ValueError, match="selectable"):
        _entry(posture=Posture.IRREVERSIBLE)
    with pytest.raises(ValueError, match="selectable"):
        _entry(posture=Posture.REVERSIBLE)


def test_malformed_key_fails_loud() -> None:
    with pytest.raises(ValueError, match="triple"):
        ControlPlane(table={("pagerduty", "alertmanager"): _entry()})


# --- the gating intersection (allow / deny-by-default) ------------------------


def test_known_route_with_granted_skill_resolves_allowed() -> None:
    plan = ControlPlane(table=_table()).resolve(_trigger())
    assert plan.allowed is True
    assert plan.deny_reason is None
    assert plan.bundle_ref == "root-cause"
    assert plan.budget.max_iterations == 40  # the entry's per-trigger budget rides through
    assert plan.labels["posture"] == "propose-only"


def test_unknown_route_fails_closed() -> None:
    # An unknown (venue, locus, trigger_kind) → deny-by-default. The safe answer to an
    # unrecognized route is "no", never an implicit admit.
    cp = ControlPlane(table=_table())
    assert cp.resolve(_trigger(venue="slack")).allowed is False
    assert cp.resolve(_trigger(locus="some-channel")).allowed is False
    assert cp.resolve(_trigger(trigger_kind="mention")).allowed is False
    denied = cp.resolve(_trigger(venue="github"))
    assert denied.deny_reason == DENY_NOT_PERMITTED


def test_ungranted_skill_fails_closed() -> None:
    # A trigger requesting a skill the matched route does not grant → deny. The entry grants only
    # root-cause; asking for immunefi-zeroday (the design's example of a gated skill) is denied.
    plan = ControlPlane(table=_table()).resolve(
        _trigger(requested_skills=("root-cause", "immunefi-zeroday"))
    )
    assert plan.allowed is False
    assert plan.deny_reason == DENY_NOT_PERMITTED


def test_denied_plan_is_total_with_a_reason() -> None:
    # A denied plan is a TOTAL value (non-nullable budget) carrying a bounded reason — the router
    # branches on allowed alone, never a nullable budget.
    plan = ControlPlane(table=_table()).resolve(_trigger(venue="unknown"))
    assert plan.allowed is False
    assert plan.deny_reason == DENY_NOT_PERMITTED
    assert plan.budget is not None  # total, not a nullable budget the caller must guard
    assert plan.bundle_ref == ""


def test_runplan_rejects_inconsistent_allow_and_reason() -> None:
    # A deny with no reason is an un-labelable metric; an allow with a reason is a contradiction.
    with pytest.raises(ValueError):
        RunPlan(allowed=True, deny_reason="x", bundle_ref="b", budget=_budget())
    with pytest.raises(ValueError):
        RunPlan(allowed=False, deny_reason=None, bundle_ref="b", budget=_budget())


# --- the PD-dogfood route resolves allowed end-to-end -------------------------


def test_pd_dogfood_route_resolves_allowed_against_the_am_adapter() -> None:
    # The dogfood path end-to-end: the AM adapter's declared route + requested skill must match a
    # table entry keyed on the same (venue, locus, trigger_kind) and granting the same skill, so
    # the existing AM→PD flow resolves to an allowed RunPlan. This pins the adapter↔table contract
    # (a drift in either constant would silently deny the dogfood path).
    table = {
        (_AM_VENUE, _AM_LOCUS, _AM_TRIGGER_KIND): _entry(
            skills=frozenset({_AM_ROOT_CAUSE_SKILL})
        )
    }
    body = {
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
    trigger = AlertmanagerAdapter().parse(body)
    assert isinstance(trigger, NormalizedTrigger)
    plan = ControlPlane(table=table).resolve(trigger)
    assert plan.allowed is True
    assert plan.labels["posture"] == "propose-only"


# --- the per-human initiator gate (the Blocking-dependency seam — NOT built) ---


@pytest.mark.skip(
    reason="Per-human initiator eligibility (Stage-B authz) is GATED on the Blocking dependency "
    "(the auth substrate must be proven to carry a per-human principal). This slice builds the "
    "seam (_initiator_eligible) for the system/machine path only — N/A for the PD path's system "
    "principal. Un-defer with the Slack slice once the per-human principal is proven."
)
def test_per_human_initiator_eligibility_gate() -> None:  # pragma: no cover - gated placeholder
    # When un-deferred: a trigger from a human NOT eligible for the requested (skill, venue,
    # channel) must DENY even when the route + skill + posture all grant — the fourth gate in the
    # intersection. The system-path seam returns True unconditionally today.
    raise AssertionError("gated — see skip reason")
