"""Pure auth-posture invariant checks (PLT-669) — no omnigent import.

Kept omnigent-free so the Phase-1 header-mode invariant is unit-testable in
isolation (Omnigent is a pinned runtime dep, absent from lint/unit CI). The
``serve.py`` wrapper feeds it ``omni.resolve_auth_source()`` /
``omni.local_single_user_enabled()`` and raises on a non-None return.
"""

from __future__ import annotations


def header_posture_error(*, auth_source: str, single_user_enabled: bool) -> str | None:
    """Return an error message if the deployed auth posture is invalid, else None.

    Phase-1 trusted-operator posture:
    - must resolve to ``header`` mode (identity delegated to Sei SSO at the
      ingress); enabling oidc/accounts is a one-way door re-crossing the
      ``app.py:1748`` accounts construction (deferred past Phase-1);
    - ``OMNIGENT_LOCAL_SINGLE_USER`` must be off, or a header-less request falls
      back to the admin ``local`` user (impersonation) — loopback dev only.
    """
    if auth_source != "header":
        return (
            f"sei_omnigent is header-mode only (OMNIGENT_AUTH_PROVIDER=header); "
            f"resolved {auth_source!r}. Enabling accounts/OIDC is a one-way door "
            f"(re-crosses app.py:1748) and is not part of the Phase-1 posture."
        )
    if single_user_enabled:
        return (
            "OMNIGENT_LOCAL_SINGLE_USER is set on a deployed server: a header-less "
            "request would fall back to the admin 'local' user. Unset it "
            "(loopback single-user dev only)."
        )
    return None
