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

_PKG = Path(__file__).resolve().parent.parent
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
    for py in _PKG.rglob("*.py"):
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
                    offenders.append(f"{py.relative_to(_PKG)} imports {mod}")
    assert not offenders, (
        "only sei_omnigent._omnigent_shim may import omnigent (DECISION-1 drift isolation): "
        + "; ".join(offenders)
    )


def test_pinned_omnigent_matches_pyproject() -> None:
    """The PINNED_OMNIGENT constant and the pyproject pin agree."""
    import sei_omnigent  # no omnigent import in the top package

    pyproject = tomllib.loads((_PKG / "pyproject.toml").read_text())
    deps = pyproject["project"]["dependencies"]
    pin = next(d for d in deps if d.startswith("omnigent=="))
    assert pin == f"omnigent=={sei_omnigent.PINNED_OMNIGENT}", (
        f"pyproject pin {pin!r} != PINNED_OMNIGENT {sei_omnigent.PINNED_OMNIGENT!r}"
    )
