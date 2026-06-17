"""Pure config assembly for the runnable entrypoint (PLT-672).

omnigent-free, like :mod:`sei_omnigent._posture` — the entrypoint
(:mod:`sei_omnigent.server.serve_main`, which lives under ``server/`` whose
``__init__`` eagerly pulls the omnigent-coupled seam) imports these, but the
logic here is unit-testable without omnigent installed.
"""

from __future__ import annotations

import os
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


def build_effective_config(raw_cfg: dict[str, Any], *, deny_shell: bool) -> dict[str, Any]:
    """Merge the overlay's read-only server-default policies into a loaded config.

    The overlay's read-only policies (``admin__github_read_only`` /
    ``admin__deny_mutating_os``) are **authoritative**: they are spread last, so
    they win on a key collision. An operator config may *add* policies but cannot
    silently drop the read-only backstop by reusing a key. ``READ_ONLY_POLICY_MODULES``
    is unioned into ``policy_modules`` (order-preserving, deduped) to populate the
    policy-registry *catalog* and permit runtime re-attach of the handler via the
    policy-write APIs. Note: the backstop itself does **not** depend on this union —
    server-default policies are instantiated by direct ``import_module`` in
    omnigent's ``resolve_function_policy``, which bypasses the registry allowlist
    (that allowlist gates only untrusted attach routes), so ``deny_mutating_os``
    fires at boot whether or not the module is listed. The union is belt-and-suspenders.
    """
    cfg = dict(raw_cfg)
    cfg["policies"] = {
        **(cfg.get("policies") or {}),
        **read_only_default_policies(deny_shell=deny_shell),  # overlay wins on collision
    }
    modules = list(cfg.get("policy_modules") or [])
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


def load_config(path: str | None) -> dict[str, Any]:
    """Load the server YAML config (``yaml.safe_load``), or ``{}`` when no path.

    Mirrors omnigent ``cli.py::_load_config`` + the config-dir relative-path
    resolution (``cli.py:2922-2925``, via :func:`resolve_relative_locations`).
    ``yaml`` is imported lazily (it is an omnigent dependency) so this module
    imports — and the no-path branch runs — without it.
    """
    if not path:
        return {}
    import yaml  # noqa: PLC0415

    with open(path, encoding="utf-8") as f:
        cfg = yaml.safe_load(f) or {}
    return resolve_relative_locations(cfg, config_path=path)
