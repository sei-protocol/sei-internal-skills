"""The single Omnigent-coupling surface for the Sei overlay (DECISION-1).

DECISION-1 (B) accepts that ``build_server`` clones Omnigent's ``cli.py`` serve
boot sequence rather than calling a (not-yet-existing) upstream injection hook.
The cost of that clone is *drift*: the boot sequence is the function Omnigent
iterates hardest in alpha (0.1.x, no SemVer). This module isolates that cost.

**Every** Omnigent symbol the overlay touches is re-exported here — public API
(``create_app``, ``init`` as ``init_runtime``, ``RuntimeCaps``, ``AgentCache``,
the store ABCs) *and* the private CLI helpers the clone unavoidably reuses
(``_create_artifact_store``, ``_preregister_agent``, ``_ensure_sqlite_parent_dir``,
``_default_db_uri``, ``_default_artifact_location``, ``config_str_list``). The
private ones are the drift-prone half: when the pinned tag bumps, this file is
the one place to re-verify against the new ``omnigent/cli.py``.

Verified against omnigent == 0.1.1 (see sei_omnigent.PINNED_OMNIGENT):
  - create_app                       omnigent/server/app.py:672
  - init (init_runtime)              omnigent/runtime/__init__.py:31
  - RuntimeCaps                      omnigent/runtime/caps.py
  - AgentCache                       omnigent/runtime/agent_cache.py
  - SqlAlchemy* stores               omnigent/stores/<name>/sqlalchemy_store.py
  - HostStore                        omnigent/stores/host_store.py
  - parse_default_policies/_llm      omnigent/spec
  - parse_sandbox_config             omnigent/server/managed_hosts.py
  - resolve_auth_source/create_auth_provider/UnifiedAuthProvider
                                     omnigent/server/auth.py
  - private CLI helpers              omnigent/cli.py  (drift-prone — re-verify on bump)
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
    resolve_auth_source,
    UnifiedAuthProvider,
)

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
from omnigent.stores.host_store import HostStore

# --- Private CLI helpers (the drift-prone half — re-verify on every bump) ----
# These are not public Omnigent API; the clone reuses them rather than
# re-implementing artifact-store selection, agent pre-registration, and the
# default db/artifact locations. Importing functions has no click side effects.
from omnigent.cli import (  # noqa: PLC2701  (intentional private reuse, isolated here)
    _create_artifact_store,
    _preregister_agent,
    _ensure_sqlite_parent_dir,
    _default_db_uri,
    _default_artifact_location,
    config_str_list,
)

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
    "resolve_auth_source",
    "UnifiedAuthProvider",
    "SqlAlchemyAgentStore",
    "SqlAlchemyFileStore",
    "SqlAlchemyConversationStore",
    "SqlAlchemyCommentStore",
    "SqlAlchemyPolicyStore",
    "SqlAlchemyPermissionStore",
    "HostStore",
    "_create_artifact_store",
    "_preregister_agent",
    "_ensure_sqlite_parent_dir",
    "_default_db_uri",
    "_default_artifact_location",
    "config_str_list",
]
