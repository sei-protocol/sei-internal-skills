"""Contract guards for the store/identity seam (PLT-667).

These are static/source-level checks — they assert the *invariants* the
cross-review slate flagged, and run without Omnigent installed (Omnigent is a
pinned runtime dependency, not present in lint/unit CI). Behavioral tests that
actually boot the server live behind an ``omnigent``-installed integration job.
"""

from __future__ import annotations

import ast
import tomllib
from pathlib import Path

# tests/ live at the PROJECT ROOT (Tide/sei_omnigent/tests/); the importable
# package is under src/. _ROOT holds pyproject.toml + tests/; _PKG is src/sei_omnigent.
_ROOT = Path(__file__).resolve().parent.parent
_PKG = _ROOT / "src" / "sei_omnigent"

# Build/packaging artifact dirs to skip when walking _ROOT for source — `python
# -m build` (the wheel-smoke command) drops copies of the package into build/,
# which would otherwise perturb the import-discipline scan below.
_ARTIFACT_DIRS = {"build", "dist"}
_SERVE = _PKG / "server" / "serve.py"

# The seam contract (ONE-WAY DOOR): every Phase-2/3 store swap binds to this
# field set. A change here is a deliberate, human-signed-off contract change.
_EXPECTED_STORE_FIELDS = {
    "agent",
    "file",
    "conversation",
    "comment",
    "policy",
    "permission",
    "artifact",
    "host",
    "agent_cache",
}


# The 5 genuinely-private ``omnigent.cli`` helpers the clone reuses — the
# drift-prone half of the shim (``_omnigent_shim.py:94-100``). A pinned-tag bump
# that moves/renames any of them breaks the overlay at *server boot*; the
# canaries below (PLT-673) surface it at CI instead.
_PRIVATE_CLI_HELPERS = {
    "_create_artifact_store",
    "_preregister_agent",
    "_ensure_sqlite_parent_dir",
    "_default_db_uri",
    "_default_artifact_location",
    "_server_uvicorn_log_config",
}


def _serve_ast() -> ast.Module:
    return ast.parse(_SERVE.read_text())


def test_stores_field_set_is_the_seam_contract() -> None:
    """Stores declares exactly the documented seam fields (named, not positional)."""
    tree = _serve_ast()
    stores = next(
        n for n in ast.walk(tree)
        if isinstance(n, ast.ClassDef) and n.name == "Stores"
    )
    fields = {
        n.target.id
        for n in stores.body
        if isinstance(n, ast.AnnAssign) and isinstance(n.target, ast.Name)
    }
    assert fields == _EXPECTED_STORE_FIELDS, (
        "Stores field set is a one-way-door contract; a change needs sign-off. "
        f"got {sorted(fields)}"
    )


def test_create_app_is_called_keyword_only() -> None:
    """create_app must be wired by keyword — positional order != signature order (C2/C8)."""
    tree = _serve_ast()
    calls = [
        n for n in ast.walk(tree)
        if isinstance(n, ast.Call)
        and isinstance(n.func, ast.Attribute)
        and n.func.attr == "create_app"
    ]
    assert calls, "expected a create_app call in serve.py"
    for call in calls:
        assert not call.args, "create_app must be called with keyword args only"
        assert not any(k.arg is None for k in call.keywords), (
            "no **kwargs splat — every create_app argument is explicit"
        )


def test_only_the_shim_imports_omnigent() -> None:
    """Drift discipline: _omnigent_shim is the single Omnigent-coupling surface."""
    offenders: list[str] = []
    # Scan from the project root so this covers BOTH the package (src/sei_omnigent)
    # AND tests/ — a test importing omnigent directly is also a drift violation.
    for py in _ROOT.rglob("*.py"):
        # Skip build artifacts (copies of the package) + the egg-info so the
        # canary keys on real source, not a stray build/ a dev may have left.
        if any(p in _ARTIFACT_DIRS or p.endswith(".egg-info") for p in py.parts):
            continue
        if py.name == "_omnigent_shim.py":
            continue
        for node in ast.walk(ast.parse(py.read_text())):
            # Collect every module name a node could import — including each
            # name in a multi-name `import a, omnigent` statement (not just the
            # first), which a naive node.names[0] check would miss.
            mods: list[str] = []
            if isinstance(node, ast.ImportFrom):
                if node.module:
                    mods.append(node.module)
            elif isinstance(node, ast.Import):
                mods.extend(alias.name for alias in node.names)
            for mod in mods:
                if mod == "omnigent" or mod.startswith("omnigent."):
                    offenders.append(f"{py.relative_to(_ROOT)} imports {mod}")
    assert not offenders, (
        "only sei_omnigent._omnigent_shim may import omnigent (DECISION-1 drift isolation): "
        + "; ".join(offenders)
    )


def test_shim_reexports_the_private_cli_helpers() -> None:
    """Static (always runs): the 5 drift-prone ``omnigent.cli`` helpers stay in the
    shim ``__all__``. Catches an accidental drop without needing omnigent installed."""
    shim = ast.parse((_PKG / "_omnigent_shim.py").read_text())
    exported = {
        elt.value
        for node in ast.walk(shim)
        if isinstance(node, ast.Assign)
        and any(isinstance(t, ast.Name) and t.id == "__all__" for t in node.targets)
        for elt in node.value.elts
        if isinstance(elt, ast.Constant)
    }
    missing = _PRIVATE_CLI_HELPERS - exported
    assert not missing, f"shim __all__ dropped private cli helpers: {sorted(missing)}"


def test_private_cli_helpers_resolve_in_omnigent() -> None:
    """Behavioral drift-canary (PLT-673): the 5 private ``omnigent.cli`` helpers the
    clone depends on still import and are callable on the pinned tag — converting an
    upstream rename/move from a server-boot crash into a CI failure.

    Gate on whether the *base* ``omnigent`` package is installed, NOT on the shim
    import: if a helper moved, the shim's ``from omnigent.cli import (...)`` raises,
    and catching that would *skip* the exact drift this canary exists to catch. So —
    omnigent absent ⇒ skip (unit CI; the integration job covers it); omnigent present
    ⇒ the shim MUST import cleanly and expose all five as callables, else fail loudly.

    Presence is probed via ``find_spec`` (a string, not a literal ``import
    omnigent``) so the "only the shim imports omnigent" guard above does not flag
    this test file.
    """
    import importlib.util  # noqa: PLC0415

    if importlib.util.find_spec("omnigent") is None:
        return  # omnigent absent (unit CI) — integration job exercises this
    # NOT wrapped in try/except — a moved helper must FAIL here, not skip.
    from sei_omnigent import _omnigent_shim as omni  # noqa: PLC0415
    unresolved = [name for name in _PRIVATE_CLI_HELPERS if not callable(getattr(omni, name, None))]
    assert not unresolved, (
        "private omnigent.cli helpers moved/renamed/non-callable on the pinned tag — "
        f"re-verify the shim against the new omnigent/cli.py: {sorted(unresolved)}"
    )


def test_pinned_omnigent_matches_pyproject() -> None:
    """The PINNED_OMNIGENT constant and the pyproject pin agree."""
    # deferred: this test asserts the top package imports without pulling omnigent.
    import sei_omnigent  # noqa: PLC0415

    pyproject = tomllib.loads((_ROOT / "pyproject.toml").read_text())
    deps = pyproject["project"]["dependencies"]
    pin = next(d for d in deps if d.startswith("omnigent=="))
    assert pin == f"omnigent=={sei_omnigent.PINNED_OMNIGENT}", (
        f"pyproject pin {pin!r} != PINNED_OMNIGENT {sei_omnigent.PINNED_OMNIGENT!r}"
    )
