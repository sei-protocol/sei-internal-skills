"""Pure auth-posture invariant checks (PLT-669) — no omnigent import.

Kept omnigent-free so the Phase-1 header-mode invariant is unit-testable in
isolation (Omnigent is a pinned runtime dep, absent from lint/unit CI). The
``serve.py`` wrapper feeds it ``omni.resolve_auth_source()`` /
``omni.local_single_user_enabled()`` and raises on a non-None return.
"""

from __future__ import annotations

from collections.abc import Mapping

#: The explicit auth-provider selector; must resolve to ``header`` mode.
_AUTH_PROVIDER_ENV = "OMNIGENT_AUTH_PROVIDER"
#: The accounts-mode switches. Presence of EITHER (even ``=0``) is refused —
#: ``OMNIGENT_AUTH_ENABLED`` is the current name; ``OMNIGENT_ACCOUNTS_ENABLED``
#: is the deprecated pre-rename name omnigent 0.6.0 still honors.
_ACCOUNTS_MODE_ENVS = ("OMNIGENT_AUTH_ENABLED", "OMNIGENT_ACCOUNTS_ENABLED")


def accounts_mode_env_error(env: Mapping[str, str]) -> str | None:
    """Return an error if the pod env could silently flip 0.6.0 to accounts mode.

    The fresh 0.3.0→0.6.0 cutover must carry NO leftover accounts auth env:
    ``OMNIGENT_AUTH_PROVIDER`` must be explicitly ``header``, and NEITHER
    ``OMNIGENT_AUTH_ENABLED`` NOR the deprecated-but-honored
    ``OMNIGENT_ACCOUNTS_ENABLED`` may be present — either enables 0.6.0's
    accounts mode (the first-user-is-admin bootstrap). Fail CLOSED on presence:
    even a leftover ``=0`` is refused, because it is a latent flip waiting for
    ``OMNIGENT_AUTH_PROVIDER`` to be unset. Complements
    :func:`header_posture_error` (which reads the *resolved* source — an explicit
    provider masks the leftover flags this negative assertion catches).
    """
    provider = env.get(_AUTH_PROVIDER_ENV)
    if provider is None or provider.strip().lower() != "header":
        return (
            f"{_AUTH_PROVIDER_ENV} must be explicitly 'header' on a deployed "
            f"sei_omnigent server; got {provider!r}. An unset/other provider risks "
            f"0.6.0's accounts-mode bootstrap."
        )
    present = [name for name in _ACCOUNTS_MODE_ENVS if name in env]
    if present:
        return (
            f"accounts-mode env is set ({', '.join(present)}) — this flips 0.6.0 "
            f"into accounts mode (first-user-is-admin) even alongside "
            f"{_AUTH_PROVIDER_ENV}=header. Scrub it from the pod env; the "
            f"header-mode cutover carries none of {_ACCOUNTS_MODE_ENVS}."
        )
    return None


def header_posture_error(*, auth_source: str, single_user_enabled: bool) -> str | None:
    """Return an error message if the deployed auth posture is invalid, else None.

    Phase-1 trusted-operator posture:
    - must resolve to ``header`` mode (identity delegated to Sei SSO at the
      ingress); enabling oidc/accounts is a one-way door re-crossing the
      ``app.py:2074`` login_url-gated accounts/OIDC auth block (deferred past
      Phase-1);
    - ``OMNIGENT_LOCAL_SINGLE_USER`` must be off, or a header-less request falls
      back to the admin ``local`` user (impersonation) — loopback dev only.
    """
    if auth_source != "header":
        return (
            f"sei_omnigent is header-mode only (OMNIGENT_AUTH_PROVIDER=header); "
            f"resolved {auth_source!r}. Enabling accounts/OIDC is a one-way door "
            f"(re-crosses the app.py:2074 login_url gate) and is not part of the "
            f"Phase-1 posture."
        )
    if single_user_enabled:
        return (
            "OMNIGENT_LOCAL_SINGLE_USER is set on a deployed server: a header-less "
            "request would fall back to the admin 'local' user. Unset it "
            "(loopback single-user dev only)."
        )
    return None
