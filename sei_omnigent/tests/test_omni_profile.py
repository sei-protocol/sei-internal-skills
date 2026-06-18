"""Tests for the omni-profile launcher core (PLT-714).

The launcher is omnigent-free and tested directly (mirrors test_header_posture's
pure half). Covers: schema load/validate, the fail-closed disposition default
(one-way door #2), and the posture-overrides-profile launch refusal (#3).
"""

from __future__ import annotations

import pytest

from sei_omnigent.omni import (
    API_VERSION,
    Disposition,
    Posture,
    ProfileError,
    launch_refusal,
    load_profile,
)


def _valid_raw() -> dict[str, object]:
    """The Design 12 §1 root-cause profile, as a raw mapping."""
    return {
        "apiVersion": API_VERSION,
        "skill": "root-cause",
        "trigger": {
            "kind": "pagerduty",
            "match": {"event_action": "trigger", "severity": ["critical", "error"]},
        },
        "goal_template": "/root-cause\nEffect: {{ alert.summary }}\nInvestigate read-only.",
        "gate_transposition": {
            "checkpoint": "fail-closed",
            "one_way_door": "fail-closed",
            "review_gate": "not-applicable",
        },
        "required_permissions": {
            "posture": "read-only",
            "egress": ["pagerduty", "slack", "telemetry"],
        },
        "output_sink": {"kind": "pagerduty-incident-note", "fallback": "slack-channel"},
        "identity": "omni-root-cause@sei.internal",
    }


# --- load_profile: happy path ------------------------------------------------


def test_loads_the_root_cause_profile() -> None:
    p = load_profile(_valid_raw())
    assert p.skill == "root-cause"
    assert p.required_posture is Posture.READ_ONLY
    assert p.required_egress == ("pagerduty", "slack", "telemetry")
    assert p.gates.checkpoint is Disposition.FAIL_CLOSED
    assert p.gates.one_way_door is Disposition.FAIL_CLOSED
    assert p.gates.review_gate is None  # not-applicable
    assert p.identity == "omni-root-cause@sei.internal"


def test_profile_is_immutable() -> None:
    p = load_profile(_valid_raw())
    with pytest.raises((AttributeError, TypeError)):
        p.skill = "other"  # type: ignore[misc]


# --- load_profile: structural validation -------------------------------------


@pytest.mark.parametrize("field", [
    "apiVersion", "skill", "trigger", "goal_template",
    "gate_transposition", "required_permissions", "output_sink", "identity",
])
def test_missing_required_field_is_rejected(field: str) -> None:
    raw = _valid_raw()
    del raw[field]
    with pytest.raises(ProfileError, match="missing required field"):
        load_profile(raw)


@pytest.mark.parametrize("field", ["skill", "goal_template", "identity"])
def test_empty_required_field_is_rejected(field: str) -> None:
    raw = _valid_raw()
    raw[field] = ""
    with pytest.raises(ProfileError, match="missing required field"):
        load_profile(raw)


def test_unsupported_api_version_is_rejected() -> None:
    raw = _valid_raw()
    raw["apiVersion"] = "omni-profile/v2"
    with pytest.raises(ProfileError, match="apiVersion"):
        load_profile(raw)


@pytest.mark.parametrize("field", ["trigger", "required_permissions", "output_sink"])
def test_non_mapping_structural_field_is_rejected(field: str) -> None:
    raw = _valid_raw()
    raw[field] = ["not", "a", "mapping"]
    with pytest.raises(ProfileError, match="must be a mapping"):
        load_profile(raw)


def test_missing_posture_is_rejected() -> None:
    raw = _valid_raw()
    raw["required_permissions"] = {"egress": ["pagerduty"]}
    with pytest.raises(ProfileError, match="posture is required"):
        load_profile(raw)


def test_unknown_posture_is_rejected_not_coerced() -> None:
    raw = _valid_raw()
    raw["required_permissions"] = {"posture": "read-wrte", "egress": ["pagerduty"]}  # typo
    with pytest.raises(ProfileError, match="not a known posture"):
        load_profile(raw)


@pytest.mark.parametrize("egress", [
    [],                       # empty
    "pagerduty",              # bare str (char-split trap)
    {"pagerduty": 1},         # mapping (key-iteration trap — Bugbot medium)
    ["pagerduty", None],      # non-string element (Bugbot low — null→"None")
    ["pagerduty", True],      # non-string element (bool→"True")
    ["pagerduty", 5],         # non-string element (number→"5")
])
def test_bad_egress_is_rejected(egress: object) -> None:
    raw = _valid_raw()
    raw["required_permissions"] = {"posture": "read-only", "egress": egress}
    with pytest.raises(ProfileError):
        load_profile(raw)


def test_egress_defaults_are_validated_as_nonempty() -> None:
    raw = _valid_raw()
    raw["required_permissions"] = {"posture": "read-only"}  # egress omitted
    with pytest.raises(ProfileError, match="at least one destination"):
        load_profile(raw)


# --- Disposition: fail-closed default (one-way door #2) -----------------------


def test_unknown_disposition_coerces_fail_closed() -> None:
    raw = _valid_raw()
    raw["gate_transposition"] = {"checkpoint": "auto-proceed-someday", "one_way_door": None}
    p = load_profile(raw)
    assert p.gates.checkpoint is Disposition.FAIL_CLOSED
    assert p.gates.one_way_door is Disposition.FAIL_CLOSED


def test_known_dispositions_round_trip() -> None:
    raw = _valid_raw()
    raw["gate_transposition"] = {
        "checkpoint": "auto-proceed-if-reversible",
        "one_way_door": "fail-closed",
        "review_gate": "escalate-async",
    }
    p = load_profile(raw)
    assert p.gates.checkpoint is Disposition.AUTO_PROCEED_IF_REVERSIBLE
    assert p.gates.one_way_door is Disposition.FAIL_CLOSED
    assert p.gates.review_gate is Disposition.ESCALATE_ASYNC


def test_coerce_passes_through_enum_instances() -> None:
    auto = Disposition.AUTO_PROCEED_IF_REVERSIBLE
    assert Disposition.coerce(auto) is auto
    assert Disposition.coerce(None) is Disposition.FAIL_CLOSED
    assert Disposition.coerce("nonsense") is Disposition.FAIL_CLOSED


# --- launch_refusal: posture-overrides-profile (one-way door #3) --------------


def test_read_only_profile_on_read_only_deploy_is_allowed() -> None:
    p = load_profile(_valid_raw())
    assert launch_refusal(
        p, deployed_posture=Posture.READ_ONLY,
        deployed_egress=["pagerduty", "slack", "telemetry", "extra"],
    ) is None


def test_read_write_profile_on_read_only_deploy_is_refused() -> None:
    raw = _valid_raw()
    raw["required_permissions"] = {"posture": "read-write", "egress": ["pagerduty"]}
    p = load_profile(raw)
    err = launch_refusal(p, deployed_posture=Posture.READ_ONLY, deployed_egress=["pagerduty"])
    assert err is not None and "posture-overrides-profile" in err


def test_egress_outside_allowlist_is_refused() -> None:
    p = load_profile(_valid_raw())  # needs pagerduty, slack, telemetry
    err = launch_refusal(
        p, deployed_posture=Posture.READ_ONLY, deployed_egress=["pagerduty", "slack"]
    )
    assert err is not None and "telemetry" in err


def test_read_only_profile_on_read_write_deploy_is_allowed() -> None:
    """read-only ⊆ read-write — a more-capable deploy still permits a read-only profile."""
    p = load_profile(_valid_raw())
    assert launch_refusal(
        p, deployed_posture=Posture.READ_WRITE,
        deployed_egress=["pagerduty", "slack", "telemetry"],
    ) is None


def test_bare_string_deployed_egress_is_rejected() -> None:
    """A bare str iterates by character — set('pagerduty') = {'p','a',...} would fail
    OPEN. launch_refusal must reject it, not silently char-split (the dissenter B1 trap)."""
    p = load_profile(_valid_raw())
    with pytest.raises(TypeError, match="bare str"):
        launch_refusal(p, deployed_posture=Posture.READ_ONLY, deployed_egress="pagerduty")


def test_empty_deployed_egress_refuses_a_profile_needing_egress() -> None:
    p = load_profile(_valid_raw())  # needs pagerduty, slack, telemetry
    err = launch_refusal(p, deployed_posture=Posture.READ_ONLY, deployed_egress=[])
    assert err is not None and "pagerduty" in err


# --- immutability: a loaded profile is isolated from source mutation -----------


def test_loaded_profile_is_isolated_from_source_mutation() -> None:
    raw = _valid_raw()
    p = load_profile(raw)
    raw["trigger"]["kind"] = "MUTATED"  # mutate the caller's retained source...
    raw["trigger"]["match"]["severity"].append("info")  # ...including nested
    assert p.trigger["kind"] == "pagerduty"  # ...the loaded profile is unaffected
    assert "info" not in p.trigger["match"]["severity"]


def test_loaded_mappings_are_read_only() -> None:
    p = load_profile(_valid_raw())
    with pytest.raises(TypeError):
        p.trigger["kind"] = "x"  # type: ignore[index]  # MappingProxyType is read-only
    with pytest.raises(TypeError):
        p.output_sink["kind"] = "x"  # type: ignore[index]


def test_review_gate_omitted_entirely_is_none() -> None:
    raw = _valid_raw()
    raw["gate_transposition"] = {"checkpoint": "fail-closed", "one_way_door": "fail-closed"}
    p = load_profile(raw)
    assert p.gates.review_gate is None


@pytest.mark.parametrize("field,bad", [
    ("skill", 0),
    ("goal_template", False),
    ("identity", ["not", "a", "string"]),
])
def test_non_string_scalar_field_is_rejected(field: str, bad: object) -> None:
    """Falsy/wrong-type scalars (0, False, list) must be rejected, not str()-coerced."""
    raw = _valid_raw()
    raw[field] = bad
    with pytest.raises(ProfileError, match="must be a string"):
        load_profile(raw)
