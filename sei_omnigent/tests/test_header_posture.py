"""Tests for the header-mode auth posture (PLT-669).

The invariant logic (``_posture.header_posture_error``) is omnigent-free and
tested directly. The *wiring* into build_server (the boot-assert call + the
header default) is guarded structurally via AST, since importing serve.py pulls
the shim → omnigent (a pinned runtime dep absent from unit CI).
"""

from __future__ import annotations

import ast
from pathlib import Path

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


def test_build_server_wires_the_boot_assert_and_header_default() -> None:
    """Structural guard: build_server must call _assert_header_posture and default
    OMNIGENT_AUTH_PROVIDER to 'header' (the posture can't be enforced if unwired)."""
    tree = ast.parse(_SERVE.read_text())
    build = next(
        n for n in ast.walk(tree)
        if isinstance(n, ast.FunctionDef) and n.name == "build_server"
    )
    calls = [
        n.func.id
        for n in ast.walk(build)
        if isinstance(n, ast.Call) and isinstance(n.func, ast.Name)
    ]
    assert "_assert_header_posture" in calls, "build_server must call _assert_header_posture"

    # setdefault("OMNIGENT_AUTH_PROVIDER", "header") present in build_server
    setdefaults = [
        n for n in ast.walk(build)
        if isinstance(n, ast.Call)
        and isinstance(n.func, ast.Attribute)
        and n.func.attr == "setdefault"
        and any(isinstance(a, ast.Constant) and a.value == "OMNIGENT_AUTH_PROVIDER" for a in n.args)
    ]
    assert setdefaults, "build_server must default OMNIGENT_AUTH_PROVIDER to header"
    assert any(
        isinstance(a, ast.Constant) and a.value == "header"
        for sd in setdefaults for a in sd.args
    ), "the AUTH_PROVIDER default must be 'header'"
