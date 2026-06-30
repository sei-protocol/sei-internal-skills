"""The single Omnigent-coupling surface for the Sei overlay (DECISION-1).

DECISION-1 (B) accepts that ``build_server`` clones Omnigent's ``cli.py`` serve
boot sequence rather than calling a (not-yet-existing) upstream injection hook.
The cost of that clone is *drift*: the boot sequence is the function Omnigent
iterates hardest in alpha (0.1.x, no SemVer). This module isolates that cost.

**Every** Omnigent symbol the overlay touches is re-exported here — public API
(``create_app``, ``init`` as ``init_runtime``, ``RuntimeCaps``, ``AgentCache``,
the store **ABCs** (so the seam can type its fields under ``TYPE_CHECKING``
without importing omnigent outside this module), the concrete ``SqlAlchemy*``
stores, and ``config_str_list``) *and* the genuinely-private CLI helpers the
clone unavoidably reuses (``_create_artifact_store``, ``_preregister_agent``,
``_ensure_sqlite_parent_dir``, ``_default_db_uri``, ``_default_artifact_location``).
Those five live only in ``omnigent/cli.py`` and are the drift-prone half: when
the pinned tag bumps, this file is the one place to re-verify against the new
``omnigent/cli.py``.

Authored/verified against the local ``omnigent`` checkout; the deploy pin is
``omnigent == 0.3.0`` (see ``sei_omnigent.PINNED_OMNIGENT``) — re-verified
symbol-by-symbol against the 0.3.0 source on the 0.2.0→0.3.0 bump (all symbols
present, no signature breaks; ``create_app`` reordered its params but the seam
wires every arg by keyword, so the reorder is absorbed). Re-confirm the
private-helper locations against the wheel on the next bump.
  - create_app                       omnigent/server/app.py:967
  - init (init_runtime)              omnigent/runtime/__init__.py:31  (keyword-only)
  - RuntimeCaps / AgentCache         omnigent/runtime/{caps,agent_cache}.py
  - store ABCs                       omnigent/stores/<name>/__init__.py
  - SqlAlchemy* stores               omnigent/stores/<name>/sqlalchemy_store.py
  - HostStore                        omnigent/stores/host_store.py  (concrete, no ABC)
  - parse_default_policies/_llm      omnigent/spec
  - parse_sandbox_config             omnigent/server/managed_hosts.py
  - config_str_list                  omnigent/server/server_config.py  (NOT a cli internal)
  - RUNNER_TUNNEL_MAX_MESSAGE_BYTES   omnigent/runner/transports/ws_tunnel/limits.py
  - resolve_auth_source/create_auth_provider/UnifiedAuthProvider
                                     omnigent/server/auth.py
  - _create_artifact_store/_preregister_agent/_ensure_sqlite_parent_dir/
    _default_db_uri/_default_artifact_location/_server_uvicorn_log_config
                                     omnigent/cli.py  (private — re-verify on bump)
"""

from __future__ import annotations

# --- Public, stable-ish Omnigent surface ------------------------------------
from omnigent.server.app import create_app
from omnigent.runtime import init as init_runtime
from omnigent.runtime.agent_cache import AgentCache
from omnigent.runtime.caps import RuntimeCaps
from omnigent.runtime import telemetry
from omnigent.spec import parse_default_policies, parse_server_llm
from omnigent.server.managed_hosts import parse_sandbox_config
from omnigent.server.auth import (
    create_auth_provider,
    local_single_user_enabled,
    resolve_auth_source,
    UnifiedAuthProvider,
)

# Store ABCs — the seam (Stores) types its fields to these. Re-exported here so
# serve.py can import them under TYPE_CHECKING *through the shim* and keep the
# "only the shim imports omnigent" contract intact.
from omnigent.stores.agent_store import AgentStore
from omnigent.stores.file_store import FileStore
from omnigent.stores.conversation_store import ConversationStore
from omnigent.stores.comment_store import CommentStore
from omnigent.stores.policy_store import PolicyStore
from omnigent.stores.permission_store import PermissionStore
from omnigent.stores.artifact_store import ArtifactStore

# Concrete stock store implementations. Phase-1 wires these unchanged; later
# phases substitute chain-backed / TEE-sealed implementations *in make_stores*,
# not here — these stay as the default backends.
from omnigent.stores.agent_store.sqlalchemy_store import SqlAlchemyAgentStore
from omnigent.stores.file_store.sqlalchemy_store import SqlAlchemyFileStore
from omnigent.stores.conversation_store.sqlalchemy_store import (
    SqlAlchemyConversationStore,
)
from omnigent.stores.comment_store.sqlalchemy_store import SqlAlchemyCommentStore
from omnigent.stores.policy_store.sqlalchemy_store import SqlAlchemyPolicyStore
from omnigent.stores.permission_store.sqlalchemy_store import SqlAlchemyPermissionStore
from omnigent.stores.host_store import HostStore  # concrete — no ABC upstream

# Public-ish config helper — lives in server_config, NOT a cli internal.
from omnigent.server.server_config import config_str_list

# WS tunnel frame ceiling — uvicorn.run(ws_max_size=...) must match what the
# runner tunnel allows, or large frames are truncated at the socket (PLT-672
# entrypoint mirrors cli.py's uvicorn bind).
from omnigent.runner.transports.ws_tunnel.limits import RUNNER_TUNNEL_MAX_MESSAGE_BYTES

# Policy types — re-exported so overlay policy modules (sei_omnigent.policies.*)
# can type-hint their handlers under TYPE_CHECKING without importing omnigent
# directly (keeps the single-coupling-surface contract). The PolicyEvent ->
# PolicyResponse contract: omnigent/policies/schema.py.
from omnigent.policies.schema import PolicyCallable, PolicyEvent, PolicyResponse

# --- Private CLI helpers (the drift-prone half — re-verify on every bump) ----
# These five live only in omnigent/cli.py. NOTE: `import omnigent.cli` is a
# heavy, side-effecting import — it builds the entire click command tree and
# pulls the CLI's deps at module load. It performs no network/filesystem writes
# and never invokes cli(), so reuse is functionally safe, but it is NOT free.
# If upstream later exposes these from a lighter module, source them there.
from omnigent.cli import (
    _create_artifact_store,
    _preregister_agent,
    _ensure_sqlite_parent_dir,
    _default_db_uri,
    _default_artifact_location,
    _server_uvicorn_log_config,
    main as omnigent_cli_main,
)

# Host-launcher coupling (PLT-715): the host-connect class the proxy-bearer patch wraps. Routed
# through the shim so host_launch.py never imports omnigent directly (single-coupling-surface).
# Re-verify HostProcess on bump. (The CLI `main` rides in the cli import block above.)
from omnigent.host.connect import HostProcess

__all__ = [
    "create_app",
    "init_runtime",
    "AgentCache",
    "RuntimeCaps",
    "telemetry",
    "parse_default_policies",
    "parse_server_llm",
    "parse_sandbox_config",
    "create_auth_provider",
    "local_single_user_enabled",
    "resolve_auth_source",
    "UnifiedAuthProvider",
    # store ABCs (seam types)
    "AgentStore",
    "FileStore",
    "ConversationStore",
    "CommentStore",
    "PolicyStore",
    "PermissionStore",
    "ArtifactStore",
    # concrete stores
    "SqlAlchemyAgentStore",
    "SqlAlchemyFileStore",
    "SqlAlchemyConversationStore",
    "SqlAlchemyCommentStore",
    "SqlAlchemyPolicyStore",
    "SqlAlchemyPermissionStore",
    "HostStore",
    # policy types (for overlay policy-handler type hints)
    "PolicyCallable",
    "PolicyEvent",
    "PolicyResponse",
    # config + private cli helpers
    "config_str_list",
    "RUNNER_TUNNEL_MAX_MESSAGE_BYTES",
    "_create_artifact_store",
    "_preregister_agent",
    "_ensure_sqlite_parent_dir",
    "_default_db_uri",
    "_default_artifact_location",
    "_server_uvicorn_log_config",
    # host-launcher coupling (PLT-715)
    "omnigent_cli_main",
    "HostProcess",
]
