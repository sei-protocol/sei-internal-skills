"""Host launcher: present the omnigent-host's K8s SA token to the sidecar (PLT-715, B1).

Resolves the host->server auth gap (Design 12 §3.2; the host-auth slate's Option B1). omnigent's
host connect, in managed mode (``OMNIGENT_HOST_TOKEN`` set), sends only ``X-Omnigent-Host-Token``
and **never an Authorization bearer** (``HostProcess._build_connect_headers`` returns early). But
the ``kube-rbac-proxy`` sidecar fronting the loopback-bound server gates on a TokenReview-able K8s
SA-JWT **bearer** (audience ``omnigent-api``) -- the only cross-pod path to the server (INV-2). So
as shipped the host tunnel reaches the sidecar with no bearer -> 401 -> cannot register.

Fix (B1 -- bearer ALONGSIDE the host-token): a launch-time **wrap-not-replace** patch on
``HostProcess._build_connect_headers`` that merges ``Authorization: Bearer <projected SA token>``
(read fresh per connect from ``OMNIGENT_PROXY_BEARER_TOKEN_FILE``) into whatever the original
returns. The bearer satisfies the sidecar (INV-2 gate); the unchanged ``X-Omnigent-Host-Token``
satisfies the server's app-layer registration. No refresher sidecar, no ``$HOME`` copy (the token
never leaves the host process -> out of the forked runner's reach), the JWT's own ``exp`` governs.

Overlay-carried **bridge**; the durable fix is an upstream omnigent bearer-from-file auth source.
The host Deployment runs ``python -m sei_omnigent.host_launch --server <url>`` (or the
``sei-omnigent-host`` console script): install the patch, then invoke ``omnigent host``
**in-process** (same process, so the patch is live when the WS tunnel connects). All omnigent
coupling routes through ``_omnigent_shim`` (the single coupling surface); re-confirm
``HostProcess._build_connect_headers`` exists on bump (the suite guards the symbol).
"""

from __future__ import annotations

import functools
import os
import sys
from collections.abc import Callable
from pathlib import Path

#: Env var pointing at the projected SA token file (audience ``omnigent-api``) the sidecar checks.
PROXY_BEARER_FILE_ENV = "OMNIGENT_PROXY_BEARER_TOKEN_FILE"
_PATCH_MARKER = "_sei_proxy_bearer_patched"


def proxy_bearer_header() -> dict[str, str]:
    """The ``Authorization`` header carrying the projected SA token, or ``{}`` if unset/unreadable.

    Read FRESH on every call (per (re)connect) so the kubelet's in-place token rotation is picked
    up at the next connect; the JWT's own ``exp`` governs validity (no local expiry bookkeeping).
    An unset env var or an unreadable/empty file yields ``{}`` -- the connect then fails closed at
    the sidecar, not here.
    """
    path = os.environ.get(PROXY_BEARER_FILE_ENV)
    if not path:
        return {}
    try:
        token = Path(path).read_text().strip()
    except OSError:
        return {}
    return {"Authorization": f"Bearer {token}"} if token else {}


def _wrap_build_headers(
    original: Callable[..., dict[str, str]],
) -> Callable[..., dict[str, str]]:
    """Wrap ``_build_connect_headers`` to MERGE the proxy bearer into the original's output.

    Augment-not-replace: call the original, then set ``Authorization`` from the projected token.
    Robust to omnigent internals -- whatever the managed/unmanaged branch returns, the SA bearer
    the sidecar validates is the one that must be on the wire, so it wins. In B1's managed-token
    deployment the original emits no ``Authorization`` (the managed path returns early), so the
    merge is purely additive; the override only fires on the non-managed branch (defense-in-depth).
    Keep it unconditional -- do not gate it, or a stale OIDC bearer could reach the sidecar.
    """

    @functools.wraps(original)  # keep __name__/__doc__/__wrapped__ for introspection + tracebacks
    def patched(self) -> dict[str, str]:
        headers = original(self)
        headers.update(proxy_bearer_header())
        return headers

    setattr(patched, _PATCH_MARKER, True)  # idempotency guard (wraps provides none)
    return patched


def install_proxy_bearer_patch() -> None:
    """Patch HostProcess._build_connect_headers to present the projected SA token. Idempotent."""
    # Through the shim (the single omnigent-coupling surface); function-local so this module's pure
    # helpers stay importable without omnigent.
    from ._omnigent_shim import HostProcess  # noqa: PLC0415 -- deferred: pure helpers omnigent-free

    original = HostProcess._build_connect_headers
    if getattr(original, _PATCH_MARKER, False):
        return
    HostProcess._build_connect_headers = _wrap_build_headers(original)


def main(argv: list[str] | None = None) -> int:
    """Install the proxy-bearer patch, then run ``omnigent host`` in-process with the args.

    In-process is load-bearing: the patch must be live in the SAME process that opens the tunnel.
    ``omnigent``'s CLI signals exit status via ``SystemExit`` (click), not a return value, so a
    non-zero exit propagates through ``raise SystemExit(main())`` below; ``return 0`` is the clean
    fall-through. Re-verify on bump that ``cli.main`` still exits via ``SystemExit`` -- were it to
    RETURN a code, swallowing it would report a failed launch as success.

    Note: omnigent's CLI inserts the CWD at sys.path[0] and this process holds the SA bearer, so
    the host Deployment must pin a non-writable workingDir + readOnlyRootFilesystem (CWD must not
    be an import-injection surface into the credential-bearing process; manifest-side hardening).
    """
    install_proxy_bearer_patch()
    args = sys.argv[1:] if argv is None else argv

    from ._omnigent_shim import omnigent_cli_main  # noqa: PLC0415 -- deferred (heavy shim import)

    sys.argv = ["omnigent", "host", *args]
    omnigent_cli_main()  # exits via SystemExit (click); see docstring
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
