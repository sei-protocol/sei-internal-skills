"""Tests for the omni-trigger receiver's serve-wiring (PLT-715 — the production entrypoint).

The PURE assembly is provable without omnigent / a live PD: the budget + ReceiverConfig boot
validation, the env→poster from_config wiring (incl. the security-reviewed guards), and the
fail-loud-on-missing-env boots. The live session factory's OmnigentClient construction is
live-only (it imports ``omnigent_client``), so it is NOT exercised here — an AST guard asserts
that import stays deferred inside ``main`` so this module imports without omnigent (the same
discipline as test_serve_main).
"""

from __future__ import annotations

import ast
from pathlib import Path

import pytest

from sei_omnigent.omni.engine import Budget
from sei_omnigent.omni.serve_receiver import (
    _DEFAULT_LEASE_MARGIN_S,
    _PD_ENROLLED_ENV,
    _PD_FROM_EMAIL_ENV,
    _PD_TOKEN_ENV,
    build_budget,
    build_control_plane,
    build_poster,
    build_receiver_config,
)

_SERVE_RECEIVER = (
    Path(__file__).resolve().parent.parent
    / "src"
    / "sei_omnigent"
    / "omni"
    / "serve_receiver.py"
)

# Every env this module reads, cleared before each test so a stray host var cannot leak in.
_ALL_ENV = (
    "OMNI_RECEIVER_WALL_CLOCK_S",
    "OMNI_RECEIVER_TOKENS",
    "OMNI_RECEIVER_QUERIES",
    "OMNI_RECEIVER_MAX_ITERATIONS",
    "OMNI_RECEIVER_NO_PROGRESS_ITERATIONS",
    "OMNI_RECEIVER_LEASE_S",
    "OMNI_RECEIVER_MAX_IN_FLIGHT",
    "OMNI_RECEIVER_PORT",
    "OMNI_RECEIVER_HOST",
    _PD_TOKEN_ENV,
    f"{_PD_TOKEN_ENV}_FILE",
    _PD_FROM_EMAIL_ENV,
    _PD_ENROLLED_ENV,
    "SEI_OMNIGENT_PD_BASE_URL",
)


@pytest.fixture(autouse=True)
def _clean_env(monkeypatch: pytest.MonkeyPatch) -> None:
    for name in _ALL_ENV:
        monkeypatch.delenv(name, raising=False)


# --- budget + ReceiverConfig assembly -----------------------------------------


def test_budget_uses_defaults_when_env_unset() -> None:
    budget = build_budget()
    assert budget.wall_clock_s > 0
    assert budget.tokens > 0
    assert budget.max_iterations > 0


def test_budget_reads_env_overrides(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("OMNI_RECEIVER_WALL_CLOCK_S", "120")
    monkeypatch.setenv("OMNI_RECEIVER_TOKENS", "50000")
    budget = build_budget()
    assert budget.wall_clock_s == 120.0
    assert budget.tokens == 50000


def test_receiver_config_default_lease_clears_the_floor() -> None:
    # The default lease must clear ReceiverConfig's floor (wall_clock + margin) — a default that
    # underran would fail every boot. build_receiver_config must produce a valid config.
    budget = build_budget()
    config = build_receiver_config(budget)
    assert config.lease_s == float(int(budget.wall_clock_s) + _DEFAULT_LEASE_MARGIN_S)


def test_receiver_config_default_lease_derives_from_raised_wall_clock(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    # The regression #1 guards: raising wall_clock WITHOUT also setting the lease must still boot —
    # the default lease is DERIVED from the effective wall_clock (not a fixed 1020), so it clears
    # ReceiverConfig's floor instead of failing at __post_init__.
    monkeypatch.setenv("OMNI_RECEIVER_WALL_CLOCK_S", "2000")
    # OMNI_RECEIVER_LEASE_S intentionally unset
    budget = build_budget()
    config = build_receiver_config(budget)  # would ValueError if the lease defaulted to 1020
    assert config.lease_s == 2000.0 + _DEFAULT_LEASE_MARGIN_S
    assert config.lease_s >= budget.wall_clock_s + config.min_lease_margin_s


def test_receiver_config_rejects_a_lease_below_the_budget_floor(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    # A manifest lease shorter than wall_clock + margin double-launches a still-running incident;
    # ReceiverConfig.__post_init__ fails closed at boot, surfaced through build_receiver_config.
    monkeypatch.setenv("OMNI_RECEIVER_WALL_CLOCK_S", "900")
    monkeypatch.setenv("OMNI_RECEIVER_LEASE_S", "100")  # < 900 + margin
    with pytest.raises(ValueError):
        build_receiver_config(build_budget())


def test_control_plane_binds_every_entry_budget_to_the_lease_floor() -> None:
    # C1 over the table: the run runs under the matched ENTRY's budget (plan.budget), so the PDP
    # must enforce the lease floor on the entry, not just RouterConfig on the shared budget. A
    # table entry whose budget.wall_clock_s overruns the config lease (a second, longer route would
    # be this) fails CLOSED at construction — at deploy, not silently at request. Here the lease is
    # derived from a 900s wall_clock budget, but the entry carries a 5000s budget → boot abort.
    config = build_receiver_config(build_budget())  # lease derived from the default 900s budget
    overrunning = Budget(
        wall_clock_s=5_000.0, tokens=400_000, queries=1_000, per_source_queries={},
        max_iterations=40, no_progress_iterations=6,
    )
    with pytest.raises(ValueError, match="double-launch"):
        build_control_plane(overrunning, config)


def test_control_plane_wiring_constructs_for_the_default_budget() -> None:
    # The happy path: the same budget the lease is derived from resolves the dogfood route cleanly
    # (the floor binds without rejecting the in-spec single route).
    budget = build_budget()
    cp = build_control_plane(budget, build_receiver_config(budget))
    assert cp.table  # the one MVP route is present and within the lease floor


# --- the PagerDuty poster from_config wiring ----------------------------------


def _set_pd_env(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path, **over: str
) -> None:
    # File-only PD token (SF1): the manifest mounts it from a Secret as a file — no inline env.
    token_file = tmp_path / "pd-token"
    token_file.write_text(over.get("token", "pd-notes-only-token"), encoding="utf-8")
    monkeypatch.setenv(f"{_PD_TOKEN_ENV}_FILE", str(token_file))
    monkeypatch.setenv(_PD_FROM_EMAIL_ENV, over.get("email", "sei-omnigent@seinetwork.io"))
    monkeypatch.setenv(_PD_ENROLLED_ENV, over.get("enrolled", "PSVC001, PSVC002"))


def test_build_poster_wires_env_into_from_config(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    _set_pd_env(monkeypatch, tmp_path)
    poster = build_poster()
    assert poster.from_email == "sei-omnigent@seinetwork.io"
    # Comma-split + whitespace-trimmed into the enrolled set (the structural authz boundary).
    assert poster.enrolled_service_ids == frozenset({"PSVC001", "PSVC002"})
    assert poster.base_url == "https://api.pagerduty.com"


def test_build_poster_fails_closed_on_all_blank_enrolled_set(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    # A comma/space-only value is non-empty (passes _require_env) but splits to no service-ids;
    # from_config's empty-enrolled guard (a silent deny-all outage) then fails closed at boot.
    _set_pd_env(monkeypatch, tmp_path, enrolled=" , ")
    with pytest.raises(ValueError):
        build_poster()


def test_build_poster_fails_loud_on_unset_enrolled_env(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    # An entirely unset enrolled env fails at _require_env (before from_config) — also fail-loud.
    monkeypatch.setenv(_PD_TOKEN_ENV, "pd-token")
    monkeypatch.setenv(_PD_FROM_EMAIL_ENV, "sei-omnigent@seinetwork.io")
    with pytest.raises(RuntimeError):
        build_poster()


def test_build_poster_fails_closed_on_off_pd_base_url(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    # from_config's host-allowlist guard: a tampered base_url pointing off-PD must not be
    # handed the token (exfiltration). Surfaced at boot.
    _set_pd_env(monkeypatch, tmp_path)
    monkeypatch.setenv("SEI_OMNIGENT_PD_BASE_URL", "https://evil.example.com")
    with pytest.raises(ValueError):
        build_poster()


def test_build_poster_fails_closed_on_http_pd_base_url(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    # SF1: a plaintext-http base_url (token sent in clear) must fail closed at boot, same as an
    # off-PD host — the from_config scheme guard rejects http://.
    _set_pd_env(monkeypatch, tmp_path)
    monkeypatch.setenv("SEI_OMNIGENT_PD_BASE_URL", "http://api.pagerduty.com")
    with pytest.raises(ValueError):
        build_poster()


def test_build_poster_fails_loud_on_missing_token(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv(_PD_FROM_EMAIL_ENV, "sei-omnigent@seinetwork.io")
    monkeypatch.setenv(_PD_ENROLLED_ENV, "PSVC001")
    # No PD_API_TOKEN → fail-closed at boot (never start with no PD credential).
    with pytest.raises(RuntimeError):
        build_poster()


def test_build_poster_fails_loud_on_missing_from_email(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv(_PD_TOKEN_ENV, "pd-token")
    monkeypatch.setenv(_PD_ENROLLED_ENV, "PSVC001")
    with pytest.raises(RuntimeError):
        build_poster()


def test_build_poster_reads_token_from_file(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    token_file = tmp_path / "pd-token"
    token_file.write_text("file-mounted-pd-token\n", encoding="utf-8")
    monkeypatch.setenv(f"{_PD_TOKEN_ENV}_FILE", str(token_file))
    monkeypatch.setenv(_PD_FROM_EMAIL_ENV, "sei-omnigent@seinetwork.io")
    monkeypatch.setenv(_PD_ENROLLED_ENV, "PSVC001")
    poster = build_poster()
    assert poster.token == "file-mounted-pd-token"


def test_build_poster_fails_loud_on_unreadable_token_file(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    monkeypatch.setenv(f"{_PD_TOKEN_ENV}_FILE", str(tmp_path / "does-not-exist"))
    monkeypatch.setenv(_PD_FROM_EMAIL_ENV, "sei-omnigent@seinetwork.io")
    monkeypatch.setenv(_PD_ENROLLED_ENV, "PSVC001")
    with pytest.raises(RuntimeError):
        build_poster()


# --- import discipline: the omnigent seam stays deferred ----------------------


def test_omnigent_imports_are_deferred_into_main() -> None:
    """The live session factory (and thus omnigent_client) must NOT be imported at module
    scope — it lives inside ``main`` so the pure assembly helpers import without omnigent. A
    refactor that hoists it breaks the omnigent-free unit suite silently; this catches it."""
    tree = ast.parse(_SERVE_RECEIVER.read_text())
    deferred = {"uvicorn", "sei_omnigent.omni._omnigent_session"}
    module_level: list[str] = []
    for node in tree.body:
        if isinstance(node, ast.Import):
            module_level.extend(a.name for a in node.names)
        elif isinstance(node, ast.ImportFrom) and node.module:
            module_level.append(node.module)
    leaked = deferred & set(module_level)
    assert not leaked, (
        f"omnigent-touching imports must stay inside main(); at module scope: {sorted(leaked)}"
    )
