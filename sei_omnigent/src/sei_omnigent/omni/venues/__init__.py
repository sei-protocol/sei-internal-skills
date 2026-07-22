"""Venue egress seams: the per-venue ``Venue`` Protocol + its implementations.

A venue is where a result leaves the router (Design 13 Component map). The router depends on
the :class:`~sei_omnigent.omni.venues.base.Venue` Protocol; PagerDuty is Venue #1
(:mod:`sei_omnigent.omni.venues.pagerduty`).
"""

from __future__ import annotations

from sei_omnigent.omni.venues.base import Venue, VenueHandle

__all__ = ["Venue", "VenueHandle"]
