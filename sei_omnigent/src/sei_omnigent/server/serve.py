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

0.6.0 deltas: (a) the sharing lockdown is wired via STATIC ``sharing_mode`` /
``public_sharing`` on ``create_app`` — static, not env, because the static path
freezes ``*_writable=False`` so ``PUT /v1/sharing`` returns 403 (env would leave
the runtime override open); (b) ``scheduled_task_store`` and ``server_config``
are omitted (both → None): the scheduler stays off, and product telemetry is
disabled via env rather than the config-based kill-path; (c) the 0.6.0
product-analytics phone-home is disabled, asserted at boot.
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
# 0.6.0 (``omnigent/server/app.py``). This tuple is a DRIFT TRIPWIRE: the overlay
# wires create_app 100% by keyword, so its own call cannot mis-bind — the
# exact-order equality instead forces a seam re-verify whenever upstream reshapes
# the signature on a pin bump. 0.6.0, for instance, inserted
# ``scheduled_task_store`` mid-signature (between ``permission_store`` and
# ``auth_provider``) and appended ``sharing_mode`` / ``public_sharing`` /
# ``server_config``; equality trips on any such move at boot/CI, prompting a
# re-review of what the new params mean for the seam. Keep in sync on every
# pinned-tag bump.
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


def _assert_create_app_arity() -> None:
    """Assert create_app's signature matches the 0.6.0-verified param tuple.

    Runs before ``create_app`` is invoked (omnigent installed → real signature).
    The exact name-and-order equality is a drift tripwire: it forces a seam
    re-verify if upstream reshapes the signature on a pin bump. It is NOT a guard
    against the overlay's own call, which is 100% keyword and cannot mis-bind. If
    a param the overlay passes (e.g. ``sharing_mode``) is ever dropped upstream,
    equality fails first with the expected-vs-got message.
    """
    params = tuple(inspect.signature(omni.create_app).parameters)
    if params != _EXPECTED_CREATE_APP_PARAMS:
        raise RuntimeError(
            "omnigent.create_app signature drifted from the 0.6.0-verified param "
            "order — re-verify the seam wiring against the new signature. "
            f"expected {_EXPECTED_CREATE_APP_PARAMS}, got {params}"
        )


def _assert_boot_posture(app: FastAPI) -> None:
    """Assert the built app's sharing lockdown and telemetry posture hold.

    The real gate (the signature tripwire only proves the wiring can bind).
    Sharing: ``RESTRICTED_READ_ONLY``, public sharing off, and BOTH ``*_writable``
    frozen ``False`` (the static-path freeze; see the module header for the
    ``PUT /v1/sharing`` 403 linkage). Telemetry: the 0.6.0 product-analytics
    phone-home is disabled, so the remote-config fetch thread never spawns.

    ``product_telemetry_is_disabled()`` memoizes on first call and also trips on
    CI env vars / a config setting, so this proves the EFFECT (telemetry off);
    the env-var CAUSE (``OMNIGENT_DISABLE_TELEMETRY`` in the pod) is guaranteed
    separately by the deploy manifest.
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

    # Drift tripwire: assert create_app's signature still matches the
    # 0.6.0-verified tuple before the call, so an upstream reshape prompts a seam
    # re-verify at boot/CI (see _EXPECTED_CREATE_APP_PARAMS).
    _assert_create_app_arity()

    # ONE-WAY DOOR: create_app is wired 100% by keyword — its parameter order
    # differs from any natural store ordering and shifts across pins (see
    # _EXPECTED_CREATE_APP_PARAMS), so keyword wiring is what keeps the call
    # stable. `scheduled_task_store` + `server_config` are OMITTED (→ None): None
    # keeps the scheduler off and discards omnigent's config-based telemetry
    # kill-path (disabled via env instead). Static `sharing_mode` +
    # `public_sharing` (NOT env) are load-bearing: the static path freezes
    # `*_writable=False`; env would leave it True — do not "simplify" to env. See
    # the module header for the full lockdown rationale.
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

    # The real gate: the sharing lockdown took and is frozen non-writable, and
    # product-analytics telemetry is disabled.
    _assert_boot_posture(app)
    return app
