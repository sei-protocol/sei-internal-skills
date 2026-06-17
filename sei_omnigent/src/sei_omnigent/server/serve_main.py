"""Runnable server entrypoint for the Sei overlay (PLT-672).

``build_server(cfg)`` (PLT-667) wires the FastAPI app but does NOT load config
or bind a socket — that was deferred (no point shipping a console script that
crashes before there is a deploy). This module is that last core sliver: load
the YAML config, merge in the overlay's read-only server-default policies
(:mod:`sei_omnigent._config`, PLT-668), build the app via the seam, and bind
uvicorn with the C11 graceful-shutdown window.

It is the container ``command`` in the K8s deploy (PLT-672), deliberately
bypassing omnigent's stock ``entrypoint.py`` — which auto-sets
``OMNIGENT_LOCAL_SINGLE_USER`` when ``OMNIGENT_AUTH_PROVIDER`` is unset (the C3
footgun → every request becomes admin ``local``). ``build_server``'s header-mode
posture boot-assert (PLT-669) fails closed on that, so this entrypoint inherits
the safe posture without re-implementing the check.

Run as ``python -m sei_omnigent.server.serve_main`` with ``OMNIGENT_CONFIG``
pointing at the server YAML (absent → stock defaults + the read-only policies).

The pure config assembly lives in :mod:`sei_omnigent._config` (omnigent-free,
unit-tested). This module's only job is the omnigent-touching wiring, so its
omnigent imports (the seam's ``build_server``, the shim's WS constant,
``uvicorn``) are deferred into :func:`main` — exercised by the omnigent-installed
integration job, not the unit suite.
"""

from __future__ import annotations

import os

from sei_omnigent._config import bool_env, build_effective_config, int_env, load_config

#: Bind all interfaces — the pod is reachable only through the Service, and the
#: default-deny NetworkPolicy (PLT-672 §2.4a) gates ingress to oauth2-proxy +
#: the host. So the bind breadth is not the trust boundary; the NetworkPolicy is.
_DEFAULT_HOST = "0.0.0.0"  # noqa: S104
_DEFAULT_PORT = 8000

#: Mirrors omnigent ``cli.py``'s uvicorn graceful-shutdown default. The deploy
#: overrides it to 85s (env below) so WS/SSE drain inside the pod's 90s grace
#: (C11) instead of uvicorn force-closing at 30s.
_SHUTDOWN_TIMEOUT_ENV = "OMNIGENT_SERVER_SHUTDOWN_TIMEOUT_S"
_SHUTDOWN_TIMEOUT_DEFAULT = 30


def main() -> None:
    """Load config → build the app via the seam → bind uvicorn.

    The omnigent-touching imports live here (not at module scope) so
    :mod:`sei_omnigent._config`'s helpers stay importable without omnigent.
    """
    import uvicorn  # noqa: PLC0415

    from sei_omnigent import _omnigent_shim as omni  # noqa: PLC0415
    from sei_omnigent.server.serve import build_server  # noqa: PLC0415

    deny_shell = bool_env("SEI_OMNIGENT_DENY_SHELL", default=False)
    cfg = build_effective_config(
        load_config(os.environ.get("OMNIGENT_CONFIG")), deny_shell=deny_shell
    )

    # build_server runs the header-mode posture boot-assert (PLT-669) before it
    # constructs the auth provider — a non-header / LOCAL_SINGLE_USER posture
    # aborts here, fail-closed, rather than booting an open server.
    app = build_server(cfg)

    # `or _DEFAULT_HOST` handles a set-but-empty OMNIGENT_SERVER_HOST; int_env
    # handles set-but-empty port/timeout (manifest `value: ""`) without crashing,
    # while a genuinely malformed value still fails loud.
    host = os.environ.get("OMNIGENT_SERVER_HOST") or _DEFAULT_HOST
    port = int_env("OMNIGENT_SERVER_PORT", default=_DEFAULT_PORT)
    shutdown_timeout = int_env(_SHUTDOWN_TIMEOUT_ENV, default=_SHUTDOWN_TIMEOUT_DEFAULT)

    # Fail loud on an out-of-range port (a `value: "0"`/negative manifest typo)
    # with an actionable message — otherwise it surfaces as an opaque OSError
    # deep in uvicorn's socket bind.
    if not 1 <= port <= 65535:
        raise ValueError(f"OMNIGENT_SERVER_PORT={port} out of range (1-65535)")

    # NOTE: omnigent's CLI does a port-bindable preflight (`_assert_server_port_bindable`,
    # cli.py:2808) for a clean "port in use" message; we omit it deliberately. The
    # deploy is `replicas:1` + `strategy: Recreate` (server-deployment.yaml), so the
    # old pod fully releases :8000 before the new one starts — no in-cluster bind
    # race — and the startupProbe's 150s headroom absorbs a transient. If a future
    # topology drops Recreate (rolling/HA), restore a preflight here for the diagnostic.

    # Mirror omnigent cli.py's uvicorn bind EXACTLY (cli.py:3126-3132), including
    # log_config — it installs RequestDurationAccessFormatter, the only source of
    # per-request duration in the access log. create_app wires the middleware that
    # populates the duration context (app.py:1019 -> performance_metrics), so the
    # suffix is live. Dropping it would silently lose latency signal in the pod.
    uvicorn.run(
        app,
        host=host,
        port=port,
        log_config=omni._server_uvicorn_log_config(),
        ws_max_size=omni.RUNNER_TUNNEL_MAX_MESSAGE_BYTES,
        timeout_graceful_shutdown=shutdown_timeout,
    )


if __name__ == "__main__":
    main()
