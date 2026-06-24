"""Runnable entrypoint for the Alertmanager serve-wiring (Design 13 — the production wiring).

``router.build_app(router)`` (the Starlette app + lifespan drain/aclose) is generic; this
module is the Alertmanager-specific assembly: load config from env → assemble the router with
its REAL seams (the Alertmanager adapter, the PagerDuty venue, the redactor, the live session
factory) → bind uvicorn on the app. It is the container ``command`` in the receiver Deployment
(mirrors ``server/serve_main.py``'s shape).

Boot is FAIL-LOUD by construction (mirrors ``serve_main`` + the router's ``__post_init__``
guards): a missing inbound bearer, a missing/mis-scoped PD config, an empty enrolled-service
set, a missing session-factory env, or an out-of-range port aborts here — the router never
starts half-configured (a half-configured trigger router is a silent outage or, worse, a
fail-open egress).

The omnigent-touching import is the live session factory (``_omnigent_session.LiveSessionFactory``
→ ``omnigent_client``); it is deferred into :func:`main` so the rest of this module — and the
pure assembly helpers the unit suite exercises — import without omnigent. The PagerDuty venue,
the redactor, the adapter, and the router core are all omnigent-free.

BOUNDARIES (the manifest injects, this module reads): the inbound AM bearer
(``OMNIGENT_TRIGGER_WEBHOOK_TOKEN_FILE``), the PD notes-only token + PD user email + enrolled
service-id set, the bind host/port, the budget caps + the run-lease, and the live session
factory's server URL / identity / agent bundle (the last three owned by ``_omnigent_session``).

Design 13; Design 12 §2, §3.5; INV-5, INV-6, INV-7.
"""

from __future__ import annotations

import os
from pathlib import Path

import httpx

from sei_omnigent._config import int_env
from sei_omnigent.omni import _redact
from sei_omnigent.omni._dedup import InMemoryDedupStore
from sei_omnigent.omni.adapters.alertmanager import AlertmanagerAdapter
from sei_omnigent.omni.engine import Budget
from sei_omnigent.omni.router import (
    Router,
    RouterConfig,
    build_app,
    load_webhook_token,
)
from sei_omnigent.omni.venues.pagerduty import PagerDutyClient

#: Bind all interfaces — the pod is reachable only through its Service + the default-deny
#: NetworkPolicy (mirrors serve_main: the bind breadth is not the trust boundary, the
#: NetworkPolicy + the inbound bearer are).
_DEFAULT_HOST = "0.0.0.0"  # noqa: S104
_DEFAULT_PORT = 8080
_PORT_ENV = "OMNI_RECEIVER_PORT"
_HOST_ENV = "OMNI_RECEIVER_HOST"

#: uvicorn graceful-shutdown window. The lifespan drains in-flight investigations (bounded by
#: the tracker's own drain_deadline_s) THEN closes the poster pool; this is the outer uvicorn
#: bound. Set above the tracker drain so uvicorn does not force-close mid-drain.
_SHUTDOWN_TIMEOUT_ENV = "OMNI_RECEIVER_SHUTDOWN_TIMEOUT_S"
_SHUTDOWN_TIMEOUT_DEFAULT = 75

# --- PagerDuty config env (the manifest injects; from_config carries the host/scheme/enrolled
#     guards — this module never builds the raw ctor) ----------------------------------------
_PD_TOKEN_ENV = "PD_API_TOKEN"
_PD_FROM_EMAIL_ENV = "WALLE_PD_FROM_EMAIL"
_PD_ENROLLED_ENV = "WALLE_PD_ENROLLED_SERVICE_IDS"
_PD_BASE_URL_ENV = "WALLE_PD_BASE_URL"
_DEFAULT_PD_BASE_URL = "https://api.pagerduty.com"

# --- budget + lease env (the profile's wall_clock sets lease_s; ReceiverConfig.__post_init__
#     enforces lease_s >= wall_clock + margin) -----------------------------------------------
_WALL_CLOCK_S_ENV = "OMNI_RECEIVER_WALL_CLOCK_S"
_TOKENS_ENV = "OMNI_RECEIVER_TOKENS"
_QUERIES_ENV = "OMNI_RECEIVER_QUERIES"
_MAX_ITERATIONS_ENV = "OMNI_RECEIVER_MAX_ITERATIONS"
_NO_PROGRESS_ITERATIONS_ENV = "OMNI_RECEIVER_NO_PROGRESS_ITERATIONS"
_LEASE_S_ENV = "OMNI_RECEIVER_LEASE_S"
_MAX_IN_FLIGHT_ENV = "OMNI_RECEIVER_MAX_IN_FLIGHT"

#: Budget defaults — a sane root-cause investigation envelope; the manifest overrides per the
#: omni-profile. Positive by construction (Budget.__post_init__ rejects a non-positive cap).
_DEFAULT_WALL_CLOCK_S = 900
_DEFAULT_TOKENS = 400_000
_DEFAULT_QUERIES = 1_000
_DEFAULT_MAX_ITERATIONS = 40
_DEFAULT_NO_PROGRESS_ITERATIONS = 6
#: Lease default cushion above wall_clock — a generous post-back/release margin that clears
#: ReceiverConfig's floor of wall_clock + min_lease_margin_s (30s). The default lease is DERIVED
#: from the EFFECTIVE wall_clock at boot (see build_receiver_config), not a fixed constant, so
#: raising OMNI_RECEIVER_WALL_CLOCK_S without also setting the lease still clears the floor.
_DEFAULT_LEASE_MARGIN_S = 120
#: Default 1: the standing OmnigentClient is SHARED across concurrent sessions and its
#: concurrency-safety is unproven — N-wide on it risks cross-stream bleed / pool starvation. The
#: manifest overrides via OMNI_RECEIVER_MAX_IN_FLIGHT. UN-DEFER to a higher value only after a
#: live N-concurrent soak shows no cross-stream bleed + bounded pool-wait on the shared client.
_DEFAULT_MAX_IN_FLIGHT = 1


def _require_env(name: str) -> str:
    """Read a required env var, failing LOUD at boot if unset/empty (mirrors load_webhook_token).

    A half-configured receiver is the failure mode this guards: a missing PD token / email is a
    silent fail-closed egress (WallE posts nothing) or a mis-scoped credential — surface it here,
    not on the first investigation's post-back.
    """
    value = (os.environ.get(name) or "").strip()
    if not value:
        raise RuntimeError(
            f"{name} is required for the omni-trigger receiver but is unset/empty "
            "(fail-closed at boot — the receiver must never start half-configured)."
        )
    return value


def _pd_token() -> str:
    """The PD notes-only API token, read from the Secret-mounted file ONLY (the secure path).

    File-only (no inline env fallback): the manifest mounts the token from a Secret as a file so
    it never transits the pod env, where a process listing or a crash dump could surface it. An
    unset / unreadable / empty token file fails closed at boot — the receiver must never start
    with no PD credential (a silent fail-closed egress: WallE posts nothing).
    """
    path = (os.environ.get(f"{_PD_TOKEN_ENV}_FILE") or "").strip()
    if not path:
        raise RuntimeError(
            f"{_PD_TOKEN_ENV}_FILE is required (file-only — the manifest mounts the PD token "
            "from a Secret as a file; no inline env fallback). Fail-closed at boot."
        )
    try:
        token = Path(path).read_text(encoding="utf-8").strip()
    except OSError as exc:
        raise RuntimeError(
            f"{_PD_TOKEN_ENV}_FILE={path!r} is unreadable: {exc} (fail-closed at boot)."
        ) from exc
    if not token:
        raise RuntimeError(f"{_PD_TOKEN_ENV}_FILE={path!r} is empty (fail-closed at boot).")
    return token


def build_budget() -> Budget:
    """Assemble the four-axis :class:`Budget` from env (defaults per the root-cause envelope).

    Caps are positive by construction (Budget.__post_init__ rejects non-positive/non-finite);
    int_env fails loud on a malformed value and falls back to the default on set-but-empty.
    """
    return Budget(
        wall_clock_s=float(int_env(_WALL_CLOCK_S_ENV, default=_DEFAULT_WALL_CLOCK_S)),
        tokens=int_env(_TOKENS_ENV, default=_DEFAULT_TOKENS),
        queries=int_env(_QUERIES_ENV, default=_DEFAULT_QUERIES),
        per_source_queries={},
        max_iterations=int_env(_MAX_ITERATIONS_ENV, default=_DEFAULT_MAX_ITERATIONS),
        no_progress_iterations=int_env(
            _NO_PROGRESS_ITERATIONS_ENV, default=_DEFAULT_NO_PROGRESS_ITERATIONS
        ),
    )


def build_receiver_config(budget: Budget) -> RouterConfig:
    """Assemble the :class:`RouterConfig` (lease + in-flight cap) around a budget.

    ``lease_s`` defaults to the EFFECTIVE wall_clock + margin (DERIVED, not a fixed constant), so
    raising OMNI_RECEIVER_WALL_CLOCK_S without also setting the lease still clears
    RouterConfig.__post_init__'s floor (wall_clock + min_lease_margin_s) instead of failing the
    boot. An explicit OMNI_RECEIVER_LEASE_S still overrides; the floor guard catches an explicit
    lease that underruns the budget (a double-launch of a still-running incident).
    """
    default_lease = int(budget.wall_clock_s) + _DEFAULT_LEASE_MARGIN_S
    return RouterConfig(
        budget=budget,
        lease_s=float(int_env(_LEASE_S_ENV, default=default_lease)),
        max_in_flight=int_env(_MAX_IN_FLIGHT_ENV, default=_DEFAULT_MAX_IN_FLIGHT),
    )


def build_poster() -> PagerDutyClient:
    """Build the real notes-only PagerDuty venue from env via the security-reviewed from_config.

    ``from_config`` carries the host/scheme allowlist + the empty-enrolled-set guard (never the
    raw ctor): a tampered base_url that points off-PD, or an empty enrolled set (a silent
    deny-all outage), fails closed there. The router owns the minted ``AsyncClient``'s lifecycle
    (the lifespan ``aclose`` releases its pool).
    """
    enrolled = [s.strip() for s in _require_env(_PD_ENROLLED_ENV).split(",") if s.strip()]
    return PagerDutyClient.from_config(
        from_email=_require_env(_PD_FROM_EMAIL_ENV),
        enrolled_service_ids=enrolled,
        token=_pd_token(),
        base_url=os.environ.get(_PD_BASE_URL_ENV) or _DEFAULT_PD_BASE_URL,
        http=httpx.AsyncClient(),
    )


def serve(router: Router) -> None:
    """The generic serve core: build the app on a wired :class:`Router` and bind uvicorn.

    Venue-agnostic — every venue's serve-wiring assembles its own router (its adapter + venue)
    and hands it here. The bind host/port/shutdown-window come from the shared env contract.
    """
    import uvicorn  # noqa: PLC0415 -- deferred (web glue only)

    app = build_app(router)
    host = os.environ.get(_HOST_ENV) or _DEFAULT_HOST
    port = int_env(_PORT_ENV, default=_DEFAULT_PORT)
    shutdown_timeout = int_env(_SHUTDOWN_TIMEOUT_ENV, default=_SHUTDOWN_TIMEOUT_DEFAULT)
    if not 1 <= port <= 65535:
        raise ValueError(f"{_PORT_ENV}={port} out of range (1-65535)")
    uvicorn.run(app, host=host, port=port, timeout_graceful_shutdown=shutdown_timeout)


def main() -> None:
    """Assemble the Alertmanager router from env → serve it.

    The Alertmanager-specific assembly: the AM adapter + the PD venue + the redactor + the live
    session factory. The live session factory (the one omnigent-touching seam) is imported here,
    not at module scope, so the pure assembly helpers above import without omnigent (the unit
    suite's discipline — mirrors serve_main's deferred omnigent imports).
    """
    from sei_omnigent.omni._omnigent_session import LiveSessionFactory  # noqa: PLC0415 -- seam

    budget = build_budget()
    config = build_receiver_config(budget)
    expected_token = load_webhook_token()
    venue = build_poster()
    # The live session factory binds the standing host + the omni-root-cause identity; from_env
    # fails loud on a missing server URL / identity / agent bundle (its own boot guard).
    session_factory = LiveSessionFactory.from_env()

    router = Router(
        config=config,
        adapter=AlertmanagerAdapter(),
        dedup=InMemoryDedupStore(),
        session_factory=session_factory,
        expected_token=expected_token,
        venue=venue,
        redact=_redact.redact,
    )
    serve(router)


if __name__ == "__main__":
    main()
