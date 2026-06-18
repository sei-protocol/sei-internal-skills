"""Pure omni-profile loader + launch-refusal invariants (PLT-714) — no omnigent import.

The omni-overlay's per-skill contract. An ``omni-profile`` declares the things
that specialize a mode-agnostic Tide skill for **headless** execution — the
direct analog of ``/tee``'s kit and ``/idiomatic``'s pack. This module is the
pure, unit-testable *launcher core*: it loads + validates a profile and enforces
the **posture-overrides-profile** invariant (a profile can never widen the
deployed read-only posture). It is read by the thin launch wrapper, **never** by
the omnigent server, and makes zero runtime omnigent imports (mirrors
``_posture.py`` — the omnigent-touching paths stay out of the unit surface).

Design 12 §1. One-way doors: #1 (schema / ``apiVersion`` — every profile binds
to it), #2 (the three-disposition enum + fail-closed default), #3
(posture-overrides-profile precedence).
"""

from __future__ import annotations

from collections.abc import Iterable, Mapping
from dataclasses import dataclass
from enum import StrEnum

# ONE-WAY DOOR #1: the schema id every future profile binds to. A bump is a
# deliberate, reviewed schema migration — not a silently-accepted variant.
API_VERSION = "omni-profile/v1"


class Disposition(StrEnum):
    """The single gate-disposition vocabulary — by reference to ``/workstream``'s gate-kinds.

    NOT a second vocabulary: these mirror how ``/workstream`` already handles a
    checkpoint / guard / review-gate. ``FAIL_CLOSED`` is the default for any
    unclassified or unrecognized gate (one-way door #2) — a typo'd or
    future-dated disposition must degrade to halt-and-report, never to
    auto-proceed. ``ESCALATE_ASYNC`` is named in the enum but **unbuilt** for
    Phase-1 (root-cause never uses it).
    """

    AUTO_PROCEED_IF_REVERSIBLE = "auto-proceed-if-reversible"
    ESCALATE_ASYNC = "escalate-async"
    FAIL_CLOSED = "fail-closed"

    @classmethod
    def coerce(cls, value: object) -> Disposition:
        """Map a raw value to a ``Disposition``; anything unrecognized → ``FAIL_CLOSED``.

        Fail-closed-as-default is a safety invariant, not a parse convenience: an
        unknown disposition is *more* dangerous than a missing one, so it is
        pinned to the most restrictive gate rather than rejected (a profile is
        never blocked from launching by an over-cautious gate, only by an
        over-permissive one — see :func:`launch_refusal`).
        """
        if isinstance(value, cls):
            return value
        try:
            return cls(value)
        except ValueError:
            return cls.FAIL_CLOSED


class Posture(StrEnum):
    """The deploy posture a profile may request, ordered by capability.

    ``read-only`` is a *subset* of ``read-write``. The launcher refuses a profile
    whose requested posture exceeds the deployed posture (one-way door #3); unlike
    a disposition, an unknown posture is **rejected** (a :class:`ProfileError`),
    never coerced — silently downgrading a mis-typed posture to read-only would
    mask a misconfigured profile.
    """

    READ_ONLY = "read-only"
    READ_WRITE = "read-write"


# Capability rank for the subset test: a requested posture is permitted only if
# its rank is <= the deployed posture's rank.
_POSTURE_RANK: dict[Posture, int] = {Posture.READ_ONLY: 0, Posture.READ_WRITE: 1}


class ProfileError(ValueError):
    """An omni-profile is structurally invalid and cannot be loaded.

    Distinct from a *launch refusal* (:func:`launch_refusal`): a ``ProfileError``
    means the profile is malformed; a launch refusal means a well-formed profile
    asks for more than the deployment grants.
    """


@dataclass(frozen=True)
class GateTransposition:
    """How each ``/workstream`` gate-kind is transposed for headless execution.

    ``review_gate`` is ``None`` when the skill emits no review-gate
    (``not-applicable`` in the profile YAML) — distinct from a gate that exists
    and is set to ``FAIL_CLOSED``.
    """

    checkpoint: Disposition
    one_way_door: Disposition
    review_gate: Disposition | None


@dataclass(frozen=True)
class OmniProfile:
    """A validated omni-profile. Immutable: the launcher never mutates a loaded profile."""

    api_version: str
    skill: str
    trigger: Mapping[str, object]
    goal_template: str
    gates: GateTransposition
    required_posture: Posture
    required_egress: tuple[str, ...]
    output_sink: Mapping[str, object]
    identity: str


_REQUIRED_FIELDS: tuple[str, ...] = (
    "apiVersion",
    "skill",
    "trigger",
    "goal_template",
    "gate_transposition",
    "required_permissions",
    "output_sink",
    "identity",
)

_NOT_APPLICABLE = "not-applicable"


def _require_mapping(value: object, field: str) -> Mapping[str, object]:
    if not isinstance(value, Mapping):
        raise ProfileError(
            f"omni-profile field {field!r} must be a mapping, got {type(value).__name__}"
        )
    return value


def _parse_posture(value: object) -> Posture:
    try:
        return Posture(value)
    except ValueError:
        permitted = ", ".join(repr(p.value) for p in Posture)
        raise ProfileError(
            f"omni-profile required_permissions.posture {value!r} is not a known posture "
            f"(expected one of {permitted}); an unknown posture is rejected, never coerced."
        ) from None


def _parse_egress(value: object) -> tuple[str, ...]:
    if isinstance(value, str) or not isinstance(value, Iterable):
        raise ProfileError(
            "omni-profile required_permissions.egress must be a list of destination names "
            f"(e.g. ['pagerduty', 'telemetry']), got {type(value).__name__}"
        )
    egress = tuple(str(dest) for dest in value)
    if not egress:
        raise ProfileError(
            "omni-profile required_permissions.egress must name at least one destination"
        )
    return egress


def _parse_gates(raw: Mapping[str, object]) -> GateTransposition:
    """Build the gate triad, coercing each disposition fail-closed (one-way door #2)."""
    raw_review = raw.get("review_gate")
    review_gate = None if raw_review in (None, _NOT_APPLICABLE) else Disposition.coerce(raw_review)
    return GateTransposition(
        checkpoint=Disposition.coerce(raw.get("checkpoint")),
        one_way_door=Disposition.coerce(raw.get("one_way_door")),
        review_gate=review_gate,
    )


def load_profile(raw: Mapping[str, object]) -> OmniProfile:
    """Parse + validate a raw omni-profile mapping into an :class:`OmniProfile`.

    Raises :class:`ProfileError` on any structural problem: a missing/empty
    required field, an unsupported ``apiVersion`` (one-way door #1), a non-mapping
    ``trigger``/``required_permissions``/``output_sink``, or an unknown posture.
    Unknown *dispositions* are not errors — they coerce fail-closed (one-way door
    #2). Loading never consults the deployment; the posture-overrides-profile
    check is the separate :func:`launch_refusal` gate.
    """
    missing = [f for f in _REQUIRED_FIELDS if raw.get(f) in (None, "")]
    if missing:
        raise ProfileError(f"omni-profile missing required field(s): {', '.join(missing)}")

    api_version = raw["apiVersion"]
    if api_version != API_VERSION:
        raise ProfileError(
            f"omni-profile apiVersion {api_version!r} is unsupported; this launcher binds "
            f"{API_VERSION!r}. A schema bump is a deliberate one-way door (#1), not an "
            "auto-accepted variant."
        )

    permissions = _require_mapping(raw["required_permissions"], "required_permissions")
    if "posture" not in permissions:
        raise ProfileError("omni-profile required_permissions.posture is required")

    return OmniProfile(
        api_version=api_version,
        skill=str(raw["skill"]),
        trigger=_require_mapping(raw["trigger"], "trigger"),
        goal_template=str(raw["goal_template"]),
        gates=_parse_gates(_require_mapping(raw["gate_transposition"], "gate_transposition")),
        required_posture=_parse_posture(permissions["posture"]),
        required_egress=_parse_egress(permissions.get("egress", ())),
        output_sink=_require_mapping(raw["output_sink"], "output_sink"),
        identity=str(raw["identity"]),
    )


def launch_refusal(
    profile: OmniProfile,
    *,
    deployed_posture: Posture,
    deployed_egress: Iterable[str],
) -> str | None:
    """Return a refusal message if the deployment must NOT launch this profile, else ``None``.

    The **posture-overrides-profile** invariant (one-way door #3), in the hard
    direction: a profile may only run within capabilities the deployment already
    grants, never widen them. Two subset checks, both fail-closed:

    1. **Posture** — the requested posture must be a subset of (no more capable
       than) the deployed posture. A read-write profile on a read-only server is
       refused at launch, not honored.
    2. **Egress** — every destination the profile requires must be in the deployed
       egress allowlist. A profile that needs an unlisted destination is refused
       rather than silently running without that reach.

    Mirrors ``_posture.py``'s ``header_posture_error`` shape (message-or-``None``)
    so the launch wrapper raises on a non-``None`` return.
    """
    if _POSTURE_RANK[profile.required_posture] > _POSTURE_RANK[deployed_posture]:
        return (
            f"omni-profile for skill {profile.skill!r} requests posture "
            f"{profile.required_posture.value!r}, which exceeds the deployed posture "
            f"{deployed_posture.value!r}. posture-overrides-profile (one-way door #3): a "
            "profile cannot widen the deployed read-only posture — refused at launch."
        )

    allowed = set(deployed_egress)
    unlisted = [dest for dest in profile.required_egress if dest not in allowed]
    if unlisted:
        return (
            f"omni-profile for skill {profile.skill!r} requires egress to "
            f"{', '.join(repr(d) for d in unlisted)}, not in the deployed egress allowlist "
            f"({', '.join(repr(d) for d in sorted(allowed)) or 'none'}). Refused at launch — "
            "the profile cannot reach a destination the deployment does not permit."
        )

    return None
