"""build_server + make_stores — the store/identity seam (PLT-667).

Why this exists
---------------
Omnigent's stock ``omnigent serve`` (``cli.py``) hard-codes the concrete stores
and offers no injection point, so Sei cannot substitute a chain-backed
``AgentStore`` (Phase-2) or a TEE-sealed ``ConversationStore`` (Phase-3) without
either forking Omnigent or owning its boot. DECISION-1 (B) chooses the latter,
*minimally*: this module reproduces the serve boot sequence but carves the
store-construction block out into :func:`make_stores` — the single seam every
later phase swaps through. ``build_server`` wires the seam's output into the
upstream ``create_app`` / ``init_runtime`` by **keyword** (see ``ONE-WAY DOOR``).

All Omnigent coupling flows through :mod:`sei_omnigent._omnigent_shim` so a
pinned-tag bump's drift lands in one file.

ONE-WAY DOOR (needs human sign-off — the review-gate does not discharge it)
---------------------------------------------------------------------------
* :class:`Stores` field set / names — the seam contract every Phase-2/3 store
  swap binds to. Named fields (not a positional tuple) deliberately defuse the
  cross-review C2/C8 "tuple-order vs create_app signature-order" mis-bind risk,
  and the fields are typed to the upstream store ABCs so a *type* mis-bind
  (e.g. ``host=`` vs ``agent_cache=``) is caught too.
* ``create_app`` is called **by keyword only** — its parameter order differs
  from any natural store ordering; positional wiring would silently mis-bind
  ``artifact_store`` / ``agent_cache``.

Scope note: PLT-667 wired the *stock* stores + ``account_store=None``; PLT-668
added the server-default read-only policy (see ``sei_omnigent.policies``);
PLT-669 adds the header-mode boot-assert (``_assert_header_posture``) here. Still
deferred: the harness/roster guard (PLT-670); the oauth2-proxy + NetworkPolicy
manifests and the running-config wiring + uvicorn bind (PLT-672). Hooks are left
where they attach.

Behavior deltas vs stock ``cli.py`` serve (all intentional, header-mode posture):
the accounts cookie-secret / BASE_URL setdefault and the ``SqlAlchemyAccountStore``
construction are omitted (dead under header mode); ``parse_sandbox_config`` is
called without the stock ``ValueError -> click.ClickException`` rewrap (no Click
context here) — a ``sandbox:`` typo surfaces as a raw ``ValueError``. Relative
``artifact_location`` resolution against the config-file dir (stock cli.py:3003)
is NOT done here — ``make_stores`` takes a pre-loaded ``cfg`` dict and never sees
``config_path``; that resolution belongs to the config-load step (PLT-672).
"""

from __future__ import annotations

import inspect
import os
from dataclasses import dataclass
from pathlib import Path
from typing import TYPE_CHECKING, Any

from sei_omnigent import _omnigent_shim as omni
from sei_omnigent._posture import accounts_mode_env_error, header_posture_error

if TYPE_CHECKING:  # types only — no runtime import of omnigent (drift contract)
    from fastapi import FastAPI

    # Imported *through the shim* so the "only the shim imports omnigent" guard
    # holds (the AST test keys on the `omnigent*` module name, not the shim).
    from sei_omnigent._omnigent_shim import (
        AgentCache,
        AgentStore,
        ArtifactStore,
        CommentStore,
        ConversationStore,
        FileStore,
        HostStore,
        PermissionStore,
        PolicyStore,
    )


# RuntimeCaps default; matches Omnigent's documented 2h execution timeout.
_DEFAULT_EXECUTION_TIMEOUT_S = 7200


@dataclass(frozen=True)
class Stores:
    """The store/identity seam — the unit every later phase swaps through.

    Phase-1 holds the stock SqlAlchemy/Local implementations. Phase-2 replaces
    ``agent`` + ``artifact`` with chain-backed implementations; Phase-3 replaces
    ``conversation`` with a TEE-sealed one — via :func:`dataclasses.replace` or a
    custom factory, a one-line change, no Omnigent fork.

    Fields are typed to the upstream store ABCs (``host`` is the one concrete
    type — ``HostStore`` has no ABC upstream, the known C10 carried cost).
    ``agent_cache`` rides on the seam: it is derived from ``artifact`` and is a
    required ``create_app`` / ``init_runtime`` input (cross-review C8).
    """

    agent: AgentStore
    file: FileStore
    conversation: ConversationStore
    comment: CommentStore
    policy: PolicyStore
    permission: PermissionStore
    artifact: ArtifactStore
    host: HostStore
    agent_cache: AgentCache


def _resolve_locations(cfg: dict[str, Any]) -> tuple[str, str]:
    """db_uri + artifact_location, mirroring the stock serve resolution."""
    db_uri = cfg.get("database_uri") or omni._default_db_uri()
    art_loc = cfg.get("artifact_location") or omni._default_artifact_location()
    return db_uri, art_loc


def make_stores(cfg: dict[str, Any]) -> Stores:
    """Construct the store layer. **The seam.**

    Phase-1: returns the stock SqlAlchemy/Local stores, byte-for-byte equivalent
    to ``omnigent serve`` (``cli.py`` serve + the ``AgentCache`` build at
    ``cli.py:3027``). A
    later phase overrides individual fields here — e.g.::

        stores = make_stores(cfg)
        stores = replace(stores, agent=ChainAgentStore(...))  # Phase-2

    or by subclassing this factory — without touching Omnigent or build_server.

    SYNC-ONLY (boot path): this does blocking I/O — ``_ensure_sqlite_parent_dir``
    (mkdir), the SqlAlchemy engine constructions, the artifact store, the agent
    cache dir. Call it at boot, never from an ``async`` context. A Phase-2/3
    re-invocation from a FastAPI ``lifespan`` or an async "reload config" route
    would block the event loop and starve every in-flight request (and the
    ``async`` ``/health`` probe → kubelet kill). If a later phase needs to rebuild
    stores on a request path, run it in a threadpool (``run_in_executor`` / a
    plain ``def`` route), not inline on the loop.
    """
    db_uri, art_loc = _resolve_locations(cfg)

    # SQLite needs its parent dir to exist before the first store connects.
    omni._ensure_sqlite_parent_dir(db_uri)

    artifact = omni._create_artifact_store(art_loc)
    agent_cache = omni.AgentCache(
        artifact_store=artifact,
        cache_dir=Path(art_loc) / ".cache",
    )

    return Stores(
        agent=omni.SqlAlchemyAgentStore(db_uri),
        file=omni.SqlAlchemyFileStore(db_uri),
        conversation=omni.SqlAlchemyConversationStore(db_uri),
        comment=omni.SqlAlchemyCommentStore(db_uri),
        policy=omni.SqlAlchemyPolicyStore(db_uri),
        permission=omni.SqlAlchemyPermissionStore(db_uri),
        artifact=artifact,
        host=omni.HostStore(db_uri),
        agent_cache=agent_cache,
    )


def _runner_tunnel_tokens() -> frozenset[str] | None:
    """Pre-shared tunnel token from the env, else None (accept any token-bound runner)."""
    token = (os.environ.get("OMNIGENT_RUNNER_TUNNEL_TOKEN") or "").strip()
    return frozenset({token}) if token else None


def _assert_header_posture() -> None:
    """Fail fast unless the header-mode trusted-operator posture holds (PLT-669).

    The invariant logic lives in :mod:`sei_omnigent._posture` (omnigent-free,
    unit-tested); this wrapper feeds it the live omnigent-resolved values.
    """
    err = header_posture_error(
        auth_source=omni.resolve_auth_source(),
        single_user_enabled=omni.local_single_user_enabled(),
    )
    if err is not None:
        raise RuntimeError(err)

    # Negative assertion (0.6.0): fail closed if the pod env carries any
    # leftover accounts-mode switch. resolve_auth_source() honors an explicit
    # OMNIGENT_AUTH_PROVIDER first, so a stray OMNIGENT_AUTH_ENABLED /
    # OMNIGENT_ACCOUNTS_ENABLED passes the header check above yet is a latent
    # flip to accounts mode — refuse it here.
    accounts_err = accounts_mode_env_error(os.environ)
    if accounts_err is not None:
        raise RuntimeError(accounts_err)


# The exact ``create_app`` parameter names AND order verified against omnigent
# 0.6.0 (``omnigent/server/app.py``). The pin canary asserts this tuple so
# the mid-signature ``scheduled_task_store`` insertion (0.6.0) — or any future
# reorder — fails at boot/CI rather than as a silent positional mis-bind. A bare
# ``signature.bind()`` succeeds against shifted slots, so the ordered tuple, not
# just the bind, is the real guard. Keep in sync on every pinned-tag bump.
_EXPECTED_CREATE_APP_PARAMS = (
    "agent_store",
    "file_store",
    "conversation_store",
    "artifact_store",
    "agent_cache",
    "runner_tunnel_tokens",
    "comment_store",
    "policy_store",
    "permission_store",
    "scheduled_task_store",
    "auth_provider",
    "host_store",
    "account_store",
    "extra_routers",
    "policy_modules",
    "admins",
    "allowed_domains",
    "sandbox_config",
    "sharing_mode",
    "public_sharing",
    "server_config",
)

# The kwargs the overlay actually passes to create_app — bound against the live
# signature (layer 1) to prove every name resolves. NOT scheduled_task_store /
# server_config / extra_routers (omitted → None, deliberate: None keeps the
# scheduler off, and telemetry is disabled via env not config).
_OVERLAY_CREATE_APP_KWARGS = (
    "agent_store",
    "file_store",
    "conversation_store",
    "comment_store",
    "policy_store",
    "artifact_store",
    "agent_cache",
    "runner_tunnel_tokens",
    "permission_store",
    "auth_provider",
    "host_store",
    "account_store",
    "policy_modules",
    "admins",
    "allowed_domains",
    "sandbox_config",
    "sharing_mode",
    "public_sharing",
)


def _assert_create_app_arity() -> None:
    """Signature guard, run before ``create_app`` is invoked (layers 1-2).

    Runs before ``create_app`` is invoked (omnigent installed → real signature).
    Layer 1: the parameter names AND order match the 0.6.0-verified tuple —
    guards the mid-signature ``scheduled_task_store`` insertion a bare ``bind``
    against shifted slots would silently accept — and binds the exact overlay
    kwargs to prove each name resolves. Layer 2: the ``sharing_mode`` +
    ``public_sharing`` params the sharing lockdown rides on are present.

    Layers 1-2 alone still pass a fail-open build (they only prove the wiring
    *can* bind); :func:`_assert_boot_posture` is the real gate.
    """
    sig = inspect.signature(omni.create_app)
    params = tuple(sig.parameters)
    if params != _EXPECTED_CREATE_APP_PARAMS:
        raise RuntimeError(
            "omnigent.create_app signature drifted from the 0.6.0-verified param "
            "order (the mid-signature scheduled_task_store insertion is exactly the "
            "silent positional mis-bind this guards). Re-verify the seam wiring. "
            f"expected {_EXPECTED_CREATE_APP_PARAMS}, got {params}"
        )
    for required in ("sharing_mode", "public_sharing"):
        if required not in sig.parameters:
            raise RuntimeError(
                f"omnigent.create_app lost the {required!r} param — the sharing "
                "lockdown cannot be wired; abort rather than boot fail-open."
            )
    # Bind the exact kwargs the overlay passes; a removed/renamed param fails
    # here, not deep in the call. Placeholder None values — bind checks names/
    # arity, not types.
    sig.bind(**{name: None for name in _OVERLAY_CREATE_APP_KWARGS})


def _assert_boot_posture(app: FastAPI) -> None:
    """Boot-assert on the built app (layers 3-4) — the real gates.

    Layer 3: the sharing lockdown actually took on the built app —
    ``RESTRICTED_READ_ONLY``, public sharing off, and BOTH ``*_writable`` frozen
    ``False`` (the static-value path in create_app sets writable=False → ``PUT
    /v1/sharing`` returns 403). Layer 4: the 0.6.0 product-analytics phone-home
    is disabled, so the remote-config fetch thread never spawns (the confirmed
    control is ``OMNIGENT_DISABLE_TELEMETRY`` in the pod env, asserted here).
    """
    mode = app.state.sharing_mode()
    if mode != omni.SharingMode.RESTRICTED_READ_ONLY:
        raise RuntimeError(
            f"sharing_mode resolved {mode!r}, expected RESTRICTED_READ_ONLY "
            "(the sharing lockdown did not take)."
        )
    if app.state.public_sharing() is not False:
        raise RuntimeError("public_sharing is enabled, expected False (sharing lockdown).")
    if app.state.sharing_mode_writable is not False:
        raise RuntimeError(
            "sharing_mode_writable is True — the PUT /v1/sharing override is "
            "NOT frozen off (a non-static sharing_mode leaked through)."
        )
    if app.state.public_sharing_writable is not False:
        raise RuntimeError(
            "public_sharing_writable is True — the public-sharing override is "
            "NOT frozen off."
        )
    if omni.product_telemetry_is_disabled() is not True:
        raise RuntimeError(
            "omnigent product telemetry is NOT disabled — the phone-home "
            "config-fetch thread would spawn. Set OMNIGENT_DISABLE_TELEMETRY=1 in "
            "the pod env."
        )


def build_server(cfg: dict[str, Any], *, stores: Stores | None = None) -> FastAPI:
    """Build the FastAPI app from the seam.

    Mirrors the stock serve boot sequence (init_runtime → telemetry → agent
    pre-registration → sandbox parse → auth → create_app), but sources every
    store from :func:`make_stores`. Pass ``stores=`` to inject a custom seam
    (the Phase-2/3 path). Location resolution is owned by ``make_stores`` — this
    function does not recompute it.
    """
    s = stores if stores is not None else make_stores(cfg)

    # 0.3.0 added RuntimeCaps.routing_client (default None = routing disabled) and
    # a stock-serve step that builds an LLMRoutingClient under OMNIGENT_SMART_ROUTING
    # + an llm: config. The clone intentionally omits it — Phase-1 has no
    # smart-routing parity; the None default is fail-safe. Wire it here if a later
    # phase wants parity.
    caps = omni.RuntimeCaps(
        execution_timeout=int(cfg.get("execution_timeout") or _DEFAULT_EXECUTION_TIMEOUT_S),
        default_policies=omni.parse_default_policies(cfg.get("policies")),
        llm=omni.parse_server_llm(cfg.get("llm")),
    )

    # Runtime globals so workflow code reaches the stores via getters.
    omni.init_runtime(
        conversation_store=s.conversation,
        agent_store=s.agent,
        agent_cache=s.agent_cache,
        file_store=s.file,
        artifact_store=s.artifact,
        comment_store=s.comment,
        policy_store=s.policy,
        caps=caps,
    )

    omni.telemetry.init()  # no-op unless OTEL_EXPORTER_OTLP_ENDPOINT is set

    for agent_dir in cfg.get("agent_dirs", []) or []:
        omni._preregister_agent(Path(agent_dir), s.agent, s.artifact, s.agent_cache)

    # Fail fast on a sandbox-config typo. Stock serve rewraps this ValueError as
    # a click.ClickException; here (no Click context) it surfaces raw — same
    # fail-fast effect, different exception type.
    sandbox_config = omni.parse_sandbox_config(cfg.get("sandbox"))

    # Header-mode trusted-operator posture (PLT-669). `header` is the natural
    # env-unset default of resolve_auth_source(); _assert_header_posture fails
    # fast on any non-header *intent* — an explicit OMNIGENT_AUTH_PROVIDER=
    # oidc/accounts OR OMNIGENT_AUTH_ENABLED=1 (which resolve_auth_source maps to
    # accounts/oidc) — rather than silently forcing header. We do NOT mutate the
    # process env. In header mode create_auth_provider() returns login_url=None,
    # so the accounts/OIDC auth block (gated at app.py:2074 on a truthy login_url)
    # is never entered — and account_store stays None as defense-in-depth (the
    # accounts sub-branch also requires it). Enabling accounts/OIDC is a one-way
    # door deferred past Phase-1.
    _assert_header_posture()
    auth_provider = omni.create_auth_provider()
    account_store = None

    # Signature guard (layers 1-2): assert create_app's param names/order match
    # the 0.6.0-verified tuple and the overlay kwargs bind, BEFORE the call — so a
    # signature drift (esp. the mid-signature scheduled_task_store insertion)
    # surfaces here, not as a silent positional mis-bind at boot.
    _assert_create_app_arity()

    # ONE-WAY DOOR: keyword-only — create_app's parameter order differs from any
    # natural store ordering, and 0.6.0 inserts `scheduled_task_store`
    # mid-signature (between permission_store and auth_provider), so positional
    # wiring past permission_store mis-binds. `scheduled_task_store` and
    # `server_config` are OMITTED (→ None): None keeps the scheduler off and
    # discards omnigent's config-based telemetry kill-path (disabled via env
    # instead). The sharing lockdown: static `sharing_mode` + `public_sharing`
    # freeze the sharing posture at RESTRICTED_READ_ONLY / disabled AND set
    # `*_writable=False` (the static path), so the runtime `PUT /v1/sharing`
    # override is frozen off. Static values (not env) are required — env would
    # leave `*_writable=True`.
    app = omni.create_app(
        agent_store=s.agent,
        file_store=s.file,
        conversation_store=s.conversation,
        comment_store=s.comment,
        policy_store=s.policy,
        artifact_store=s.artifact,
        agent_cache=s.agent_cache,
        runner_tunnel_tokens=_runner_tunnel_tokens(),
        permission_store=s.permission,
        auth_provider=auth_provider,
        host_store=s.host,
        account_store=account_store,
        policy_modules=cfg.get("policy_modules"),
        admins=omni.config_str_list(cfg.get("admins")),
        allowed_domains=omni.config_str_list(cfg.get("allowed_domains")),
        sandbox_config=sandbox_config,
        sharing_mode=omni.SharingMode.RESTRICTED_READ_ONLY,
        public_sharing=False,
    )

    # Boot-assert (layers 3-4, the real gates): the sharing lockdown took and
    # is frozen non-writable, and product-analytics telemetry is disabled.
    _assert_boot_posture(app)
    return app
