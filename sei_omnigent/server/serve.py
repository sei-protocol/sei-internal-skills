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
``artifact_location`` resolution against the config-file dir (stock cli.py:2927)
is NOT done here — ``make_stores`` takes a pre-loaded ``cfg`` dict and never sees
``config_path``; that resolution belongs to the config-load step (PLT-672).
"""

from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path
from typing import TYPE_CHECKING, Any

from sei_omnigent import _omnigent_shim as omni
from sei_omnigent._posture import header_posture_error

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
    to ``omnigent serve`` (``cli.py:2932-2940`` + the ``AgentCache`` build). A
    later phase overrides individual fields here — e.g.::

        stores = make_stores(cfg)
        stores = replace(stores, agent=ChainAgentStore(...))  # Phase-2

    or by subclassing this factory — without touching Omnigent or build_server.
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


def build_server(cfg: dict[str, Any], *, stores: Stores | None = None) -> FastAPI:
    """Build the FastAPI app from the seam.

    Mirrors the stock serve boot sequence (init_runtime → telemetry → agent
    pre-registration → sandbox parse → auth → create_app), but sources every
    store from :func:`make_stores`. Pass ``stores=`` to inject a custom seam
    (the Phase-2/3 path). Location resolution is owned by ``make_stores`` — this
    function does not recompute it.
    """
    s = stores if stores is not None else make_stores(cfg)

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

    # Header-mode trusted-operator posture (PLT-669). Default the entrypoint to
    # header (the stock entrypoint's LOCAL_SINGLE_USER auto-set is bypassed by
    # this custom serve, so we set the provider explicitly rather than rely on
    # env defaults), then fail fast if the posture is violated. account_store
    # stays None so the OIDC/accounts-only construction (app.py:1747) is never
    # reached — enabling accounts/OIDC is a one-way door deferred past Phase-1.
    os.environ.setdefault("OMNIGENT_AUTH_PROVIDER", "header")
    _assert_header_posture()
    auth_provider = omni.create_auth_provider()
    account_store = None

    # ONE-WAY DOOR: keyword-only — create_app's parameter order (app.py:672)
    # differs from any natural store ordering; positional wiring mis-binds.
    return omni.create_app(
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
    )
