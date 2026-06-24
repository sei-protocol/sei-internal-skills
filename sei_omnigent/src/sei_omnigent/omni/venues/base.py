"""The venue egress seam: the ``Venue`` Protocol + the opaque ``VenueHandle`` (Design 13).

A venue is the per-venue result sink the router writes a terminal note into. The router
depends on this Protocol, never a concrete client — PagerDuty is Venue #1
(:class:`sei_omnigent.omni.venues.pagerduty.PagerDutyClient`); :class:`LoggingVenue` is the
no-network stub. Generalizes Design 12's ``PagerDutyPoster`` (``incident_key`` becomes an
opaque :data:`VenueHandle`).

``post_result`` carries the venue's own timeout + bounded retry (the router does not wrap it —
it fails closed on any raise) and is responsible for its own idempotency (PD narrows
double-propose via a durable per-incident note marker). It returns ``True`` iff a result was
actually written and ``False`` on a fail-closed skip (no target / not enrolled / already-marked
/ scan-unconfirmed); a raise means an unrecoverable venue failure the router must escalate.

``aclose`` releases the venue's resources (an HTTP connection pool) on shutdown.

Design 13 Component map (Venue); INV-5 (redaction is the router's egress chokepoint — a venue
receives already-redacted text and writes nothing further).
"""

from __future__ import annotations

import logging
from typing import Protocol

_log = logging.getLogger("sei_omnigent.omni.venues")

#: An opaque per-venue address the router routes a result to. For PagerDuty this is the PD
#: incident key (the AM-derived dedup key, 1:1 with PD's own ``incident_key``); other venues
#: carry their own address shape behind the same alias. A thin ``str`` newtype — the router
#: never inspects it, it only carries it from the trigger's ``venue_handle`` to ``post_result``.
VenueHandle = str


class Venue(Protocol):
    """The result-egress seam: post a propose-only result to a venue handle. Propose-only.

    The router depends on this Protocol, never a concrete client. The real implementation is
    :class:`sei_omnigent.omni.venues.pagerduty.PagerDutyClient`; :class:`LoggingVenue` is the
    no-network stub. The concrete venue carries its OWN timeout + bounded retry (the router
    chokepoint does not wrap it — it fails closed on any raise) and its own idempotency.
    ``post_result`` returns ``True`` iff a result was actually written and ``False`` on a
    fail-closed skip; a raise means an unrecoverable venue failure the router must escalate.

    :meth:`aclose` releases the venue's resources (the HTTP connection pool) on shutdown.
    """

    async def post_result(self, handle: VenueHandle, body: str) -> bool: ...
    async def aclose(self) -> None: ...


class LoggingVenue:
    """A no-op venue that logs the result — the stand-in for a real venue.

    Lets the full flow (and its tests) run with NO venue token / network. The networked
    implementation for PagerDuty is :class:`sei_omnigent.omni.venues.pagerduty.PagerDutyClient`.
    """

    async def post_result(self, handle: VenueHandle, body: str) -> bool:
        _log.info("omni.post_back handle=%s body_len=%d", handle, len(body))
        return True

    async def aclose(self) -> None:
        # No network / no pool to release — the stub satisfies the Protocol with a no-op.
        return None
