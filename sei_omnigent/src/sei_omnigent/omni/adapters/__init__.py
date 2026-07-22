"""Trigger ingress seams: the per-venue ``TriggerAdapter`` Protocol + its implementations.

A trigger adapter normalizes a venue event into a venue-agnostic
:class:`~sei_omnigent.omni.adapters.base.NormalizedTrigger` the router consumes (Design 13
Component map). Alertmanager is the first adapter
(:mod:`sei_omnigent.omni.adapters.alertmanager`); its containment discipline (allowlist +
neutralize + ``<untrusted-data>`` frame + caps) is the template every adapter must follow.
"""

from __future__ import annotations

from sei_omnigent.omni.adapters.base import NoOp, NormalizedTrigger, TriggerAdapter

__all__ = ["NoOp", "NormalizedTrigger", "TriggerAdapter"]
