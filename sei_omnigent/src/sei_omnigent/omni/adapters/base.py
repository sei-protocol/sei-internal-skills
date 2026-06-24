"""The trigger ingress seam: ``TriggerAdapter`` + ``NormalizedTrigger`` + ``NoOp`` (Design 13).

A trigger adapter is the per-venue glue that authenticates the message (Stage A) and
normalizes a venue event into a venue-agnostic :class:`NormalizedTrigger` — the unit the router
admits and supervises. The router depends only on this Protocol, never a concrete venue parse.

Two adapter shapes (Design 13): a webhook ``parse(body)`` (Alertmanager) and a streaming
``run() -> AsyncIterator[trigger]`` (Slack Socket Mode — a later slice). This slice ships only
the webhook shape; the streaming shape is the seam a streaming adapter will add.

CONTAINMENT IS THE TEMPLATE (INV-6): the Alertmanager adapter's allowlist + neutralize +
``<untrusted-data>`` frame + caps discipline is the contract every adapter must follow — venue
user text (Slack/GitHub) is *more* attacker-influenceable than an AM annotation. A trigger that
does not match (non-firing / non-enrolled / unparseable) normalizes to a :class:`NoOp` carrying
the reason, never an error.

Design 13 Component map (TriggerAdapter); INV-6 (containment), INV-10 (venue authenticity).
"""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass, field
from typing import Protocol


@dataclass(frozen=True)
class NoOp:
    """A non-trigger sentinel CARRYING its reason: this venue event is not a run to admit.

    A well-formed-but-non-matching event (an AM webhook that is non-firing / non-enrolled, an
    unparseable body) yields a :class:`NoOp` — the router answers it as a no-op, NOT an error
    (the page already routed to the human). Distinct from a :class:`NormalizedTrigger`, so the
    router branches via ``isinstance(result, NoOp)`` rather than a nullable field.

    ``reason`` is the bounded, low-cardinality discriminator the router emits as the admission
    metric + the 200-body reason. The two non-match cases are DELIBERATELY distinguished
    (``"parse_error"`` vs ``"not_enrolled"``): a malformed-body storm and a benign
    not-enrolled flood must not collapse into one on-call signal.
    """

    reason: str


@dataclass(frozen=True)
class NormalizedTrigger:
    """A venue-agnostic, contained trigger — the unit the router admits and supervises.

    The single shape every adapter produces, so the router never sees a venue-specific event.
    Every attacker-influenceable field is already contained by the adapter (INV-6) — the router
    treats the whole struct as bounded, framed input.

    * ``initiator`` — who triggered the run. For a machine/system path (Alertmanager) this is a
      system identity, NOT a human principal; the per-human-initiator principal is the Slack
      slice's concern (gated on the Blocking dependency).
    * ``goal`` — the rendered investigation goal (the trusted template interpolated with the
      contained payload; the payload is a value, never promoted to the template position).
    * ``venue_handle`` — the opaque address the result posts back to (the
      :data:`~sei_omnigent.omni.venues.base.VenueHandle`; for AM, the PD incident key).
    * ``requested_skills`` — the skills the trigger asks for (the ControlPlane's input in a later
      slice; empty here — skill-gating is deferred).
    * ``dedup_key`` — the deterministic single-flight key (for AM, the AM-derived incident key).
    * ``trust`` — the trust class of the initiator (a coarse label the ControlPlane keys on;
      ``"system"`` for the AM path).
    """

    initiator: str
    goal: str
    venue_handle: str
    dedup_key: str
    trust: str
    requested_skills: tuple[str, ...] = field(default=())
    #: The contained, allowlisted payload (capped + ``<untrusted-data>``-framed, INV-6). The raw
    #: venue event is NEVER carried here — only this bounded, fixed-shape mapping.
    payload: Mapping[str, str] = field(default_factory=dict)


class TriggerAdapter(Protocol):
    """The per-venue ingress seam: normalize a venue event into a :class:`NormalizedTrigger`.

    The webhook shape (``parse``) is the only shape this slice ships. ``parse`` returns a
    :class:`NormalizedTrigger` for a matching event or a :class:`NoOp` (carrying its reason) for
    a non-matching one; it must apply the containment discipline (INV-6) before anything
    attacker-influenceable reaches the returned trigger. A streaming adapter (Slack) adds an
    async ``run()`` in a later slice.
    """

    def parse(self, body: object) -> NormalizedTrigger | NoOp: ...
