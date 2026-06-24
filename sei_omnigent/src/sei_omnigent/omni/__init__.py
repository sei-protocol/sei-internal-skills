"""The omni-overlay: the general mechanism for running Tide skills headless (Design 13).

A venue-agnostic session-routing service over vanilla omni: trigger adapters normalize venue
events, the router admits + supervises a run, and venues receive the redacted result. The
PagerDuty root-cause route is Venue #1 (the dogfood route). This package holds the pure cores
(``profile``, ``engine``, ``router``, the ``adapters``/``venues`` seams); omnigent-touching
wiring lives in the launch wrapper (``_omnigent_session``), not here.
"""

from __future__ import annotations

from sei_omnigent.omni.engine import (
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
from sei_omnigent.omni.profile import (
    API_VERSION,
    Disposition,
    GateTransposition,
    OmniProfile,
    Posture,
    ProfileError,
    launch_refusal,
    load_profile,
)
from sei_omnigent.omni.adapters.base import (
    NoOp,
    NormalizedTrigger,
    TriggerAdapter,
)
from sei_omnigent.omni.router import (
    RouterConfig,
    Router,
    RunContext,
    build_app,
    load_webhook_token,
    post_back,
    render_note,
    supervise_run,
    verify_bearer,
)
from sei_omnigent.omni.venues.base import (
    LoggingVenue,
    Venue,
    VenueHandle,
)

__all__ = [
    # profile (the per-skill contract + launcher core)
    "API_VERSION",
    "Disposition",
    "GateTransposition",
    "OmniProfile",
    "Posture",
    "ProfileError",
    "launch_refusal",
    "load_profile",
    # engine (the goal+guardrail decision core)
    "Budget",
    "RunAdmission",
    "SourceOutcome",
    "TerminalReason",
    "Usage",
    "admit_post",
    "admit_run",
    "budget_terminal",
    "classify_source_read",
    "is_truncated",
    "tripped_axis",
    # adapters (the venue ingress seam)
    "NoOp",
    "NormalizedTrigger",
    "TriggerAdapter",
    # venues (the venue egress seam)
    "LoggingVenue",
    "Venue",
    "VenueHandle",
    # router (the venue-agnostic trigger edge + lifecycle supervisor)
    "Router",
    "RouterConfig",
    "RunContext",
    "build_app",
    "load_webhook_token",
    "post_back",
    "render_note",
    "supervise_run",
    "verify_bearer",
]
