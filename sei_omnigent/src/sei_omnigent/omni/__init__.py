"""The omni-overlay: the general mechanism for running Tide skills headless (PLT-712).

WallE — the ops-bot persona running ``/root-cause`` headless — is the first
*use-case* of this overlay; the overlay itself is skill-agnostic. This package
holds the pure launcher core (``profile``); omnigent-touching wiring lives in the
launch wrapper, not here.
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
]
