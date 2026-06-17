"""Tests for the header-mode auth posture (PLT-669).

The invariant logic (``_posture.header_posture_error``) is omnigent-free and
tested directly. The *wiring* into build_server (the boot-assert call + the
header default) is guarded structurally via AST, since importing serve.py pulls
the shim → omnigent (a pinned runtime dep absent from unit CI).
"""

from __future__ import annotations

import ast
from pathlib import Path

import pytest

from sei_omnigent._posture import header_posture_error

_SERVE = Path(__file__).resolve().parent.parent / "server" / "serve.py"


def test_header_mode_no_single_user_is_ok() -> None:
    assert header_posture_error(auth_source="header", single_user_enabled=False) is None


def test_local_single_user_is_rejected() -> None:
    err = header_posture_error(auth_source="header", single_user_enabled=True)
    assert err is not None and "OMNIGENT_LOCAL_SINGLE_USER" in err


def test_non_header_modes_are_rejected() -> None:
    for source in ("oidc", "accounts", "", "HEADER_TYPO"):
        err = header_posture_error(auth_source=source, single_user_enabled=False)
        assert err is not None and "header-mode only" in err, f"{source!r} must be rejected"


def _build_server_body() -> list[ast.stmt]:
    tree = ast.parse(_SERVE.read_text())
    fn = next(
        n for n in ast.walk(tree)
        if isinstance(n, ast.FunctionDef) and n.name == "build_server"
    )
    return fn.body


def _direct_call_index(body: list[ast.stmt], name: str) -> int | None:
    """Index of a top-level (non-nested) Expr-statement call ``name(...)``, else None."""
    for i, stmt in enumerate(body):
        if (
            isinstance(stmt, ast.Expr)
            and isinstance(stmt.value, ast.Call)
            and isinstance(stmt.value.func, ast.Name)
            and stmt.value.func.id == name
        ):
            return i
    return None


def test_posture_assert_runs_on_the_main_path_before_auth() -> None:
    """build_server must call _assert_header_posture as a direct (non-nested,
    non-dead) statement that runs BEFORE create_auth_provider — guards against a
    refactor reordering it below the provider build or hiding it under a branch.
    (The AST walk only proves presence; this proves placement on the executed path.)"""
    body = _build_server_body()
    assert_idx = _direct_call_index(body, "_assert_header_posture")
    assert assert_idx is not None, "_assert_header_posture must be a direct statement in build_server"
    auth_idx = next(
        (
            i for i, s in enumerate(body)
            if isinstance(s, ast.Assign)
            and isinstance(s.value, ast.Call)
            and isinstance(s.value.func, ast.Attribute)
            and s.value.func.attr == "create_auth_provider"
        ),
        None,
    )
    assert auth_idx is not None, "create_auth_provider call not found in build_server body"
    assert assert_idx < auth_idx, "posture assert must precede create_auth_provider (and the create_app return)"


def test_serve_omni_attrs_are_exported_by_the_shim() -> None:
    """Every ``omni.<attr>`` serve.py reaches must be in the shim's __all__ — an
    unexported name AttributeErrors at boot (the exact class of bug that shipped
    a self-crashing posture guard)."""
    used = {
        n.attr
        for n in ast.walk(ast.parse(_SERVE.read_text()))
        if isinstance(n, ast.Attribute) and isinstance(n.value, ast.Name) and n.value.id == "omni"
    }
    shim = ast.parse((_SERVE.parent.parent / "_omnigent_shim.py").read_text())
    exported = {
        elt.value
        for node in ast.walk(shim)
        if isinstance(node, ast.Assign)
        and any(isinstance(t, ast.Name) and t.id == "__all__" for t in node.targets)
        for elt in node.value.elts
        if isinstance(elt, ast.Constant)
    }
    missing = used - exported
    assert not missing, f"serve.py uses omni attrs absent from the shim __all__: {sorted(missing)}"


def test_assert_header_posture_raises_behaviorally(monkeypatch: pytest.MonkeyPatch) -> None:
    """Behavioral: with omnigent installed, _assert_header_posture raises on a
    non-header provider and on LOCAL_SINGLE_USER. Skipped in unit CI (no omnigent);
    runs in the omnigent-installed integration job — closes the executed-path gap
    the pure + AST tests structurally cannot."""
    try:
        from sei_omnigent.server import serve  # noqa: PLC0415
    except Exception:  # omnigent absent (unit CI) — integration job covers this
        pytest.skip("omnigent not installed")

    # monkeypatch auto-restores the env on teardown — no manual save/finally ladder.
    monkeypatch.setenv("OMNIGENT_AUTH_PROVIDER", "oidc")
    monkeypatch.delenv("OMNIGENT_LOCAL_SINGLE_USER", raising=False)
    with pytest.raises(RuntimeError):
        serve._assert_header_posture()

    monkeypatch.setenv("OMNIGENT_AUTH_PROVIDER", "header")
    monkeypatch.setenv("OMNIGENT_LOCAL_SINGLE_USER", "1")
    with pytest.raises(RuntimeError):
        serve._assert_header_posture()
