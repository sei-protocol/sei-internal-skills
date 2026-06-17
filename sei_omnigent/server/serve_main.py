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

from sei_omnigent._config import bool_env, build_effective_config, load_config

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


def main(argv: list[str] | None = None) -> None:
    """Load config → build the app via the seam → bind uvicorn.

    The omnigent-touching imports live here (not at module scope) so
    :mod:`sei_omnigent._config`'s helpers stay importable without omnigent.
    """
    import uvicorn  # noqa: PLC0415

    from sei_omnigent import _omnigent_shim as omni  # noqa: PLC0415
    from sei_omnigent.server.serve import build_server  # noqa: PLC0415

    deny_shell = bool_env("SEI_OMNIGENT_DENY_SHELL", default=False)
    cfg = build_effective_config(load_config(os.environ.get("OMNIGENT_CONFIG")), deny_shell=deny_shell)

    # build_server runs the header-mode posture boot-assert (PLT-669) before it
    # constructs the auth provider — a non-header / LOCAL_SINGLE_USER posture
    # aborts here, fail-closed, rather than booting an open server.
    app = build_server(cfg)

    host = os.environ.get("OMNIGENT_SERVER_HOST", _DEFAULT_HOST)
    port = int(os.environ.get("OMNIGENT_SERVER_PORT", str(_DEFAULT_PORT)))
    shutdown_timeout = int(os.environ.get(_SHUTDOWN_TIMEOUT_ENV, str(_SHUTDOWN_TIMEOUT_DEFAULT)))

    uvicorn.run(
        app,
        host=host,
        port=port,
        ws_max_size=omni.RUNNER_TUNNEL_MAX_MESSAGE_BYTES,
        timeout_graceful_shutdown=shutdown_timeout,
    )


if __name__ == "__main__":
    main()
