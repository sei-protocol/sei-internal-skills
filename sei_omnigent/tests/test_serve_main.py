"""Tests for the runnable entrypoint (PLT-672).

The config-merge helpers live in the omnigent-free :mod:`sei_omnigent._config`
and are tested directly. An AST guard asserts the entrypoint's omnigent-touching
imports stay deferred inside ``main`` so ``_config`` (and thus these helpers)
import without omnigent installed — the same import-discipline pattern as the
seam/posture tests.
"""

from __future__ import annotations

import ast
from pathlib import Path

from sei_omnigent._config import build_effective_config, load_config

_SERVE_MAIN = Path(__file__).resolve().parent.parent / "server" / "serve_main.py"

_READ_ONLY_KEYS = {"admin__github_read_only", "admin__deny_mutating_os"}
_READ_ONLY_MODULE = "sei_omnigent.policies.read_only"


def test_injects_read_only_policies_into_empty_config() -> None:
    cfg = build_effective_config({}, deny_shell=False)
    assert _READ_ONLY_KEYS <= set(cfg["policies"])
    assert _READ_ONLY_MODULE in cfg["policy_modules"]


def test_overlay_policies_win_on_key_collision() -> None:
    """An operator config cannot silently drop the read-only backstop by reusing
    a key — the overlay's policy is spread last and wins."""
    tampered = {"policies": {"admin__github_read_only": {"type": "function", "function": {"path": "evil"}}}}
    cfg = build_effective_config(tampered, deny_shell=False)
    assert cfg["policies"]["admin__github_read_only"]["function"]["path"] == (
        "omnigent.policies.builtins.github.github_policy"
    )


def test_operator_policies_are_preserved_alongside() -> None:
    cfg = build_effective_config({"policies": {"custom_audit": {"type": "function"}}}, deny_shell=False)
    assert "custom_audit" in cfg["policies"]
    assert _READ_ONLY_KEYS <= set(cfg["policies"])


def test_policy_modules_union_is_deduped_and_order_preserving() -> None:
    cfg = build_effective_config(
        {"policy_modules": ["other.mod", _READ_ONLY_MODULE]}, deny_shell=False
    )
    assert cfg["policy_modules"].count(_READ_ONLY_MODULE) == 1
    assert cfg["policy_modules"][0] == "other.mod"


def test_deny_shell_propagates_into_the_policy_arg() -> None:
    cfg = build_effective_config({}, deny_shell=True)
    assert cfg["policies"]["admin__deny_mutating_os"]["function"]["arguments"]["deny_shell"] is True


def test_load_config_none_returns_empty() -> None:
    assert load_config(None) == {}
    assert load_config("") == {}


def test_uvicorn_bind_mirrors_omnigent_kwargs() -> None:
    """The bind must mirror omnigent cli.py's uvicorn.run kwargs — in particular
    ``log_config`` (installs RequestDurationAccessFormatter, the only per-request
    duration in the access log), ``ws_max_size``, and ``timeout_graceful_shutdown``
    (the C11 drain window). A refactor that silently drops one diverges from the
    named source of truth with no other signal; this catches it."""
    tree = ast.parse(_SERVE_MAIN.read_text())
    call = next(
        n for n in ast.walk(tree)
        if isinstance(n, ast.Call)
        and isinstance(n.func, ast.Attribute)
        and n.func.attr == "run"
        and isinstance(n.func.value, ast.Name)
        and n.func.value.id == "uvicorn"
    )
    kwargs = {k.arg for k in call.keywords if k.arg is not None}
    required = {"host", "port", "log_config", "ws_max_size", "timeout_graceful_shutdown"}
    missing = required - kwargs
    assert not missing, f"uvicorn.run is missing kwargs that mirror omnigent's bind: {sorted(missing)}"


def test_omnigent_imports_are_deferred_into_main() -> None:
    """The entrypoint's module-level imports must NOT pull omnigent/the shim/the
    seam/uvicorn — those live inside ``main`` so the pure helpers import without
    omnigent. A refactor that hoists them to module scope breaks the unit suite
    silently; this catches it."""
    tree = ast.parse(_SERVE_MAIN.read_text())
    deferred = {"uvicorn", "sei_omnigent._omnigent_shim", "sei_omnigent.server.serve"}
    module_level: list[str] = []
    for node in tree.body:  # top-level statements only
        if isinstance(node, ast.Import):
            module_level.extend(a.name for a in node.names)
        elif isinstance(node, ast.ImportFrom) and node.module:
            module_level.append(node.module)
    leaked = deferred & set(module_level)
    assert not leaked, f"omnigent-touching imports must stay inside main(), found at module scope: {sorted(leaked)}"
