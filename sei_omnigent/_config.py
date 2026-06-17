"""Pure config assembly for the runnable entrypoint (PLT-672).

omnigent-free, like :mod:`sei_omnigent._posture` — the entrypoint
(:mod:`sei_omnigent.server.serve_main`, which lives under ``server/`` whose
``__init__`` eagerly pulls the omnigent-coupled seam) imports these, but the
logic here is unit-testable without omnigent installed.
"""

from __future__ import annotations

import os
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
    is unioned into ``policy_modules`` (order-preserving, deduped) so the
    ``deny_mutating_os`` handler is allow-listed by ``load_registry``.
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


def load_config(path: str | None) -> dict[str, Any]:
    """Load the server YAML config (``yaml.safe_load``), or ``{}`` when no path.

    Mirrors omnigent ``cli.py::_load_config``. ``yaml`` is imported lazily (it is
    an omnigent dependency) so this module imports — and the no-path branch runs —
    without it.
    """
    if not path:
        return {}
    import yaml  # noqa: PLC0415

    with open(path) as f:
        return yaml.safe_load(f) or {}
