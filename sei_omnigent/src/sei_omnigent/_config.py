"""Pure config assembly for the runnable entrypoint (PLT-672).

omnigent-free, like :mod:`sei_omnigent._posture` — the entrypoint
(:mod:`sei_omnigent.server.serve_main`, which lives under ``server/`` whose
``__init__`` eagerly pulls the omnigent-coupled seam) imports these, but the
logic here is unit-testable without omnigent installed.
"""

from __future__ import annotations

import os
import re
from pathlib import Path
from typing import Any

from sei_omnigent.policies.read_only import (
    READ_ONLY_POLICY_MODULES,
    read_only_default_policies,
)


def bool_env(name: str, *, default: bool = False) -> bool:
    """Parse a boolean env var (``1/true/yes/on`` → True), else *default*."""
    raw = os.environ.get(name)
    if raw is None:
        return default
    return raw.strip().lower() in {"1", "true", "yes", "on"}


def int_env(name: str, *, default: int) -> int:
    """Parse an int env var, falling back to *default* when unset OR set-but-empty.

    A set-but-empty var (e.g. a manifest ``value: ""`` or an unresolved
    ``valueFrom``) returns ``""`` from ``os.environ.get(name, default)``, and
    ``int("")`` would crash the boot — so empty falls back to *default*. A
    genuinely malformed value (e.g. ``"abc"``) still raises ``ValueError``
    (fail-loud, the right behavior for a single-replica server with a bad port).
    """
    raw = os.environ.get(name)
    if raw is None or raw.strip() == "":
        return default
    return int(raw)


def build_effective_config(raw_cfg: dict[str, Any], *, deny_shell: bool) -> dict[str, Any]:
    """Merge the overlay's read-only server-default policies into a loaded config.

    The overlay's read-only policies (``admin__github_read_only`` /
    ``admin__deny_mutating_os``) are **authoritative on a key collision**: spread
    last, so an operator config may *add* policies but cannot silently drop a
    backstop by reusing its key. (This is *key*-level precedence only — DENY
    precedence among differently-named policies is omnigent's evaluator's job, not
    asserted here.) ``READ_ONLY_POLICY_MODULES`` is unioned into ``policy_modules``
    (order-preserving, deduped) to populate the policy-registry *catalog* and
    permit runtime re-attach via the policy-write APIs.

    Resolution TIMING: omnigent's ``resolve_function_policy``
    instantiates a server-default policy by direct ``import_module`` — bypassing
    the registry allowlist (which gates only untrusted *attach* routes) — but it
    does so **lazily, per session** (``runtime/policies/builder.py`` ←
    ``routes/sessions.py``), NOT at boot. So a malformed / un-importable spec
    boots green and first surfaces at session-create, not as a boot crash. A
    boot-time resolve probe that makes this fail **closed at boot** is tracked in
    PLT-686. The module union here is belt-and-suspenders for catalog/re-attach,
    not what makes the backstop fire.
    """
    cfg = dict(raw_cfg)
    cfg["policies"] = {
        **(cfg.get("policies") or {}),
        **read_only_default_policies(deny_shell=deny_shell),  # overlay wins on collision
    }
    # A scalar string is a common one-item YAML form; list() on a str would
    # split it into characters (corrupting the allow-list), so coerce first.
    raw_modules = cfg.get("policy_modules") or []
    if isinstance(raw_modules, str):
        raw_modules = [raw_modules]
    modules = list(raw_modules)
    for module in READ_ONLY_POLICY_MODULES:
        if module not in modules:
            modules.append(module)
    cfg["policy_modules"] = modules
    return cfg


def resolve_relative_locations(cfg: dict[str, Any], *, config_path: str) -> dict[str, Any]:
    """Resolve a relative ``artifact_location`` against the config file's directory.

    Mirrors stock omnigent ``cli.py:2922-2925``: when ``artifact_location`` comes
    from the config file and is relative, it is resolved against the config-file
    dir — NOT the process CWD — or artifacts (and a SQLite artifact path) land in
    the wrong place at boot. The overlay has no CLI override, so stock's
    "artifact_location came from config, not CLI" guard is always true here.

    ``database_uri`` is intentionally NOT resolved — stock omnigent doesn't either
    (it only ensures the SQLite parent dir exists, which ``make_stores`` already
    does via ``_ensure_sqlite_parent_dir``). Returns a shallow copy when it
    rewrites, else the input unchanged.
    """
    art = cfg.get("artifact_location")
    if isinstance(art, str) and art and not Path(art).is_absolute():
        resolved = dict(cfg)
        resolved["artifact_location"] = str(Path(config_path).parent / art)
        return resolved
    return cfg


_BOOL_TAG = "tag:yaml.org,2002:bool"
# YAML 1.2 booleans only — true/false and their title/upper-case forms. This
# matches omnigent's own narrowed config-loader semantics (omnigent/spec/parser.py):
# the YAML 1.1 aliases on/off/yes/no stay plain strings.
_YAML_1_2_BOOL_RE = re.compile(r"^(?:true|True|TRUE|false|False|FALSE)$")

_config_loader: type | None = None


def _bool_preserving_loader() -> type:
    """A ``SafeLoader`` subclass that owns its resolver table and always resolves
    YAML-1.2 booleans — immune to omnigent's in-process mutation of the global loader.

    omnigent ``spec/parser.py`` strips the bool implicit resolver from the *shared*
    ``yaml.SafeLoader.yaml_implicit_resolvers`` dict IN PLACE (it mutates the inherited
    dict without copying it onto its subclass first — unlike ``inner/loader.py`` which
    copies). Importing the server seam triggers that import BEFORE ``load_config`` runs,
    so a bare ``yaml.safe_load`` here would read ``in_cluster: true`` as the string
    ``'true'`` — which omnigent's strict ``_parse_provider_bool`` then rejects, crashing
    the boot. We defend by: (1) deep-copying the resolver lists onto OUR subclass (never
    touching the global), and (2) re-registering the YAML-1.2 bool resolver
    UNCONDITIONALLY — so a config bool parses to a real ``bool`` regardless of import
    order or whether the global has already been polluted. Cached (the table is static).
    """
    global _config_loader
    if _config_loader is not None:
        return _config_loader
    import yaml  # noqa: PLC0415

    class _ConfigLoader(yaml.SafeLoader):
        pass

    # Own copy of the resolver table (tuples are immutable, so a per-key list copy
    # suffices) — assigning a fresh dict to the subclass means the global is untouched.
    _ConfigLoader.yaml_implicit_resolvers = {
        ch: [(tag, rx) for tag, rx in resolvers if tag != _BOOL_TAG]
        for ch, resolvers in yaml.SafeLoader.yaml_implicit_resolvers.items()
    }
    # Re-add the narrowed bool resolver on t/T/f/F (idempotent w.r.t. the copy state).
    _ConfigLoader.add_implicit_resolver(_BOOL_TAG, _YAML_1_2_BOOL_RE, list("tTfF"))
    _config_loader = _ConfigLoader
    return _ConfigLoader


def load_config(path: str | None) -> dict[str, Any]:
    """Load the server YAML config, or ``{}`` when no path.

    Uses :func:`_bool_preserving_loader` rather than a bare ``yaml.safe_load``:
    importing the omnigent server seam mutates the global ``yaml.SafeLoader`` so
    that ``true``/``false`` deserialize as strings, and omnigent's config parser
    then rejects them (see that helper's docstring). ``yaml`` is imported lazily
    (an omnigent dependency) so this module imports — and the no-path branch runs
    — without it. Mirrors the config-dir relative-path resolution
    (``cli.py:2922-2925``, via :func:`resolve_relative_locations`).
    """
    if not path:
        return {}
    import yaml  # noqa: PLC0415

    with open(path, encoding="utf-8") as f:
        cfg = yaml.load(f, Loader=_bool_preserving_loader()) or {}  # noqa: S506 -- own bool-safe SafeLoader subclass (safe_load-equivalent, see _bool_preserving_loader)
    return resolve_relative_locations(cfg, config_path=path)
