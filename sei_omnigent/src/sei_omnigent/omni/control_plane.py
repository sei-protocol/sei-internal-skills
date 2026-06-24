"""The fail-closed Policy Decision Point (the ControlPlane / PDP) — Design 13.

The router's trust boundary: a pure, deny-by-default decision function evaluated BEFORE
session-create. It maps a contained :class:`~sei_omnigent.omni.adapters.base.NormalizedTrigger`
to a :class:`RunPlan` — the carried decision (allow + bundle + model + budget + posture labels)
the supervisor runs under. No omnigent import: pure + unit-testable (mirrors ``engine.py`` /
``adapters/alertmanager.py``), so the gating is provable without a live host.

Two things this slice gates — the constraints that do NOT need a per-human principal (the
machine/PD-dogfood path's initiator is a system identity):

1. **skill/venue/channel grant** — a declarative resolution table keyed on
   ``(venue, locus, trigger_kind)``. An unknown key, or a requested skill the matched entry does
   not grant, is a DENY (deny-by-default: the safe answer to an unrecognized trigger is "no").
2. **posture class** — the MVP floor is ``propose-only`` / ``read-only`` (INV-7'); a table entry
   declaring a write-posture is DENIED here (write-postures are gated/un-built).

NOT gated here (the Blocking dependency): per-human **initiator eligibility** (Stage-B). The PD /
machine path's initiator is a system principal, so initiator-eligibility is N/A — the seam is
:meth:`_initiator_eligible`, which the design's per-human authz fills once the auth substrate is
proven to carry a per-human principal. Do NOT key any ACL on the human here.

INTEGRITY IS PROCESS-LOCALITY (INV-8, MVP): the table is a plain in-process declarative config,
not a signed/GitOps-gated catalog. The cryptographically-signed, version-pinned catalog is the
deferred INV-8 target. A malformed/empty table fails LOUD at construction (mirrors
``RouterConfig.__post_init__`` / ``Budget.__post_init__``) — an empty PDP table would deny every
trigger, a silent outage.

Design 13 (ControlPlane / Trust-model / INV-7', INV-8); Blocking dependency (per-human authz).
"""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass, field
from enum import StrEnum
from types import MappingProxyType

from sei_omnigent.omni.adapters.base import NormalizedTrigger
from sei_omnigent.omni.engine import Budget


class Posture(StrEnum):
    """A skill's maximum declared effect (INV-7'). The MVP floor selects only the read classes.

    ``PROPOSE_ONLY`` / ``READ_ONLY`` are the only postures in force in the MVP (the global floor
    across all venues). ``REVERSIBLE`` / ``IRREVERSIBLE`` are the target write-posture shape — a
    table entry declaring one is DENIED here (un-built, gated one audited skill at a time). Named
    now so a future write-skill author selects an existing posture rather than inventing one.
    """

    PROPOSE_ONLY = "propose-only"
    READ_ONLY = "read-only"
    REVERSIBLE = "reversible"
    IRREVERSIBLE = "irreversible"


#: The postures selectable under the MVP floor (INV-7'). A table entry outside this set is denied
#: at resolve — write-postures are gated/un-built. The frozenset is the single source the gate
#: and the boot validation both read.
_SELECTABLE_POSTURES: frozenset[Posture] = frozenset(
    {Posture.PROPOSE_ONLY, Posture.READ_ONLY}
)

#: The deny reason the router surfaces for a control-plane denial (alongside the adapter's
#: ``parse_error`` / ``not_enrolled``). A single low-cardinality label: the WHY (unknown key vs
#: ungranted skill vs write-posture) goes to the structured log, not the metric — a deny-reason
#: enum the on-call dashboard groups on must stay bounded.
DENY_NOT_PERMITTED = "not_permitted"


@dataclass(frozen=True)
class RunPlan:
    """The carried decision a resolved trigger runs under — the PDP's output.

    A frozen value the supervisor threads: ``allowed`` gates the run; ``budget`` is the
    per-trigger budget (replacing the single boot-time budget); ``bundle_ref`` is a catalog KEY
    (a string the SessionFactory resolves to bytes in a LATER slice — this slice carries it, does
    not load it); ``model_override`` / ``reasoning_effort`` are the decided-but-not-yet-applied
    model levers (applied post-create in a later slice); ``labels`` carry the decision context
    (venue / locus / posture) for the structured log.

    On a deny, ``allowed`` is ``False`` and ``deny_reason`` carries the bounded reason; the run
    fields are still populated with the safe denied defaults so the struct is total (no nullable
    budget the caller must guard).
    """

    allowed: bool
    deny_reason: str | None
    bundle_ref: str
    budget: Budget
    model_override: str | None = None
    reasoning_effort: str | None = None
    labels: Mapping[str, str] = field(default_factory=dict)

    def __post_init__(self) -> None:
        # A denied plan must carry a reason and an allowed plan must NOT — a deny with no reason
        # is an un-labelable metric, an allow with a reason is a contradiction the caller would
        # mis-branch on. Fail loud at construction (this is a programming error, not input).
        if self.allowed and self.deny_reason is not None:
            raise ValueError("an allowed RunPlan must not carry a deny_reason")
        if not self.allowed and not self.deny_reason:
            raise ValueError("a denied RunPlan must carry a deny_reason")
        object.__setattr__(self, "labels", MappingProxyType(dict(self.labels)))


@dataclass(frozen=True)
class TableEntry:
    """One declarative grant: what a ``(venue, locus, trigger_kind)`` route may run under.

    ``skills`` is the grant set — a trigger's ``requested_skills`` must be a SUBSET (deny-by-
    default: a skill not listed here is not granted). ``posture`` is the entry's declared posture
    (validated selectable at boot — a write-posture entry is a config error the table rejects).
    ``bundle_ref`` / ``model_override`` / ``reasoning_effort`` ride into the :class:`RunPlan`.
    ``budget`` is the per-trigger budget for this route.
    """

    bundle_ref: str
    skills: frozenset[str]
    posture: Posture
    budget: Budget
    model_override: str | None = None
    reasoning_effort: str | None = None

    def __post_init__(self) -> None:
        # Fail LOUD on a write-posture entry: the MVP floor selects only read postures (INV-7'),
        # so a write-posture in the table is a config error, not a runtime deny — surface it at
        # boot rather than denying every trigger that route would carry at request time.
        if self.posture not in _SELECTABLE_POSTURES:
            raise ValueError(
                f"TableEntry posture {self.posture.value!r} is not selectable under the MVP "
                f"floor (INV-7'); only {sorted(p.value for p in _SELECTABLE_POSTURES)} are. A "
                "write-posture route is gated/un-built."
            )
        object.__setattr__(self, "skills", frozenset(self.skills))


#: The table key: the route a trigger resolves against. Coarse + declared (never derived from the
#: opaque ``venue_handle``) — the adapter sets ``venue`` / ``locus`` / ``trigger_kind`` on the
#: NormalizedTrigger, so the PDP keys on stated fields, not a handle shape it would have to parse.
RouteKey = tuple[str, str, str]
ResolutionTable = Mapping[RouteKey, TableEntry]


@dataclass(frozen=True)
class ControlPlane:
    """The fail-closed PDP: ``resolve(trigger) -> RunPlan``. Deny-by-default.

    Holds a declarative :data:`ResolutionTable` (process-local; the signed catalog is the
    deferred INV-8 target). :meth:`resolve` is pure: it keys the table on the trigger's declared
    ``(venue, locus, trigger_kind)``, then intersects the constraints that need NO per-human
    principal — skill grant ∩ posture class. An unknown key, an ungranted skill, or a
    non-selectable posture is a DENY. The per-human initiator gate (:meth:`_initiator_eligible`)
    is the Blocking-dependency seam, N/A for the system/machine path.
    """

    table: ResolutionTable

    def __post_init__(self) -> None:
        # Fail LOUD on an empty/malformed table (mirrors RouterConfig / Budget): an empty PDP
        # table denies every trigger — a silent total outage, worse than a loud boot abort. A
        # mistyped key/value would also surface here rather than as a deny storm at request time.
        if not self.table:
            raise ValueError(
                "ControlPlane resolution table is empty — every trigger would be denied "
                "(fail-closed at boot; an empty PDP is a silent outage)."
            )
        for key, entry in self.table.items():
            is_route_key = (
                isinstance(key, tuple)
                and len(key) == 3
                and all(isinstance(part, str) for part in key)
            )
            if not is_route_key:
                raise ValueError(
                    f"ControlPlane table key {key!r} must be a (venue, locus, trigger_kind) "
                    "string triple."
                )
            if not isinstance(entry, TableEntry):
                raise ValueError(
                    f"ControlPlane table value for {key!r} must be a TableEntry, got "
                    f"{type(entry).__name__}."
                )
        object.__setattr__(self, "table", MappingProxyType(dict(self.table)))

    def _initiator_eligible(self, trigger: NormalizedTrigger) -> bool:
        """Per-human initiator-eligibility — the Stage-B seam (Blocking dependency).

        DELIBERATELY a no-op pass for this slice: the machine/PD path's initiator is a system
        principal, so per-human ACL is N/A — there is no human principal at the omni layer to key
        on until the auth substrate is proven to carry one. The per-human authz logic (server-side
        membership resolution, fail-closed) lands HERE in the gated Slack slice; until then the
        gate rests on the skill/venue/posture intersection below, which needs no principal.

        Returns ``True`` unconditionally — the system path is eligible by construction; this is
        the documented seam, NOT an implemented ACL.
        """
        return True

    def resolve(self, trigger: NormalizedTrigger) -> RunPlan:
        """Resolve a contained trigger to a :class:`RunPlan` — fail-closed, deny-by-default.

        The gating intersection for the machine path: the route must be a known table entry
        (skill/venue/channel grant), every requested skill must be granted by that entry, and the
        entry's posture must be selectable under the MVP floor (INV-7'). Any miss is a DENY with
        :data:`DENY_NOT_PERMITTED`. The per-human initiator gate is N/A here (system principal).

        A deny returns a total RunPlan (denied defaults), never a partial — the caller branches on
        ``allowed`` and never sees a nullable budget.
        """
        if not self._initiator_eligible(trigger):
            # Unreachable on the machine path (the seam passes); kept as the explicit first gate so
            # the Stage-B authz lands in front of the skill-gate, not behind it, when it un-defers.
            return self._deny(trigger)

        entry = self.table.get((trigger.venue, trigger.locus, trigger.trigger_kind))
        if entry is None:
            # Unknown (venue, locus, trigger_kind) → deny-by-default. The safe answer to an
            # unrecognized route is "no", never an implicit admit.
            return self._deny(trigger)

        # Skill grant: every requested skill must be in the entry's grant set. An empty request is
        # trivially granted (the AM path requests its one route skill explicitly). A subset miss
        # means the trigger asked for a skill this route does not grant → deny.
        if not set(trigger.requested_skills).issubset(entry.skills):
            return self._deny(trigger)

        # Posture is validated selectable at table-construction (TableEntry.__post_init__), so a
        # matched entry is always within the MVP floor here — the boot guard is the enforcement
        # point; this resolve never has to re-deny a write-posture (it could not have been built).
        return RunPlan(
            allowed=True,
            deny_reason=None,
            bundle_ref=entry.bundle_ref,
            budget=entry.budget,
            model_override=entry.model_override,
            reasoning_effort=entry.reasoning_effort,
            labels={
                "venue": trigger.venue,
                "locus": trigger.locus,
                "trigger_kind": trigger.trigger_kind,
                "posture": entry.posture.value,
            },
        )

    def _deny(self, trigger: NormalizedTrigger) -> RunPlan:
        """Build the total denied RunPlan. The budget is a denied placeholder, never applied.

        A denied plan still satisfies :class:`RunPlan`'s "budget is non-nullable" contract, so the
        router branches on ``allowed`` alone. The budget here is never threaded into a run (the
        deny path returns before launch); it is the minimal valid Budget so the frozen struct is
        constructible without leaking a real route's budget into a denied decision.
        """
        return RunPlan(
            allowed=False,
            deny_reason=DENY_NOT_PERMITTED,
            bundle_ref="",
            budget=_DENIED_BUDGET,
            labels={
                "venue": trigger.venue,
                "locus": trigger.locus,
                "trigger_kind": trigger.trigger_kind,
            },
        )


#: The placeholder budget a denied RunPlan carries — never threaded into a run (the deny path
#: returns before launch). The minimal valid Budget (every cap positive, per Budget's own guard)
#: so the denied plan is a constructible total value without surfacing a real route's budget.
_DENIED_BUDGET = Budget(
    wall_clock_s=1.0,
    tokens=1,
    queries=1,
    per_source_queries={},
    max_iterations=1,
    no_progress_iterations=1,
)
