"""Tests for the live session factory's auth + lifecycle posture (PLT-715).

``_omnigent_session`` imports ``omnigent_client`` at module scope (it is the ONE impure seam),
so this whole module is skipped where omnigent is absent (the omnigent-free unit suite) and runs
only in the omnigent-installed environment. It pins the behavioral fixes that do NOT need a live
server: the per-create FRESH SA-bearer read (S-CRIT), boot-fail-loud on the missing bearer-file
env, the idempotent ``aclose`` (B4), and the no-secret-in-repr posture (SF1). The actual
OmnigentClient/SessionsChat wire calls stay VERIFY-ON-LIVE.
"""

from __future__ import annotations

import asyncio

import pytest

pytest.importorskip("omnigent_client")  # the seam imports it at module scope

from sei_omnigent.omni._omnigent_session import (
    BUNDLE_PATH_ENV,
    FORWARDED_EMAIL_ENV,
    PROXY_BEARER_FILE_ENV,
    SERVER_URL_ENV,
    LiveSessionFactory,
    _proxy_bearer_token,
)

_ALL_ENV = (SERVER_URL_ENV, FORWARDED_EMAIL_ENV, BUNDLE_PATH_ENV, PROXY_BEARER_FILE_ENV)


@pytest.fixture(autouse=True)
def _clean_env(monkeypatch: pytest.MonkeyPatch) -> None:
    for name in _ALL_ENV:
        monkeypatch.delenv(name, raising=False)


def _factory(*, bearer_file: str, email: str = "walle@seinetwork.io") -> LiveSessionFactory:
    # Construct directly (skip from_env's OmnigentClient build) — the client is an opaque sentinel
    # so these tests stay off the live wire.
    return LiveSessionFactory(
        client=object(),
        bundle=b"bundle-bytes",
        forwarded_email=email,
        proxy_bearer_file=bearer_file,
    )


def test_from_env_fails_loud_on_missing_bearer_file(
    monkeypatch: pytest.MonkeyPatch, tmp_path
) -> None:
    # S-CRIT: the projected SA bearer token file is REQUIRED in prod — its absence must fail at
    # boot, not surface as a 401 at the sidecar on the first investigation.
    bundle = tmp_path / "bundle.tgz"
    bundle.write_bytes(b"x")
    monkeypatch.setenv(SERVER_URL_ENV, "http://omnigent.sei.svc:8080")
    monkeypatch.setenv(FORWARDED_EMAIL_ENV, "walle@seinetwork.io")
    monkeypatch.setenv(BUNDLE_PATH_ENV, str(bundle))
    # PROXY_BEARER_FILE_ENV intentionally unset
    with pytest.raises(RuntimeError, match=PROXY_BEARER_FILE_ENV):
        LiveSessionFactory.from_env()


def test_proxy_bearer_is_read_fresh_per_call(tmp_path) -> None:
    # S-CRIT: the SA bearer is read FRESH each call (rotation-safe) — a rewrite of the file
    # between reads is picked up, NOT cached from the first read.
    token_file = tmp_path / "sa-token"
    token_file.write_text("token-v1\n", encoding="utf-8")
    assert _proxy_bearer_token(str(token_file)) == "token-v1"
    token_file.write_text("token-v2-rotated\n", encoding="utf-8")
    assert _proxy_bearer_token(str(token_file)) == "token-v2-rotated"


def test_proxy_bearer_fails_closed_on_unreadable_or_empty(tmp_path) -> None:
    with pytest.raises(RuntimeError):
        _proxy_bearer_token(str(tmp_path / "does-not-exist"))
    empty = tmp_path / "empty"
    empty.write_text("", encoding="utf-8")
    with pytest.raises(RuntimeError):
        _proxy_bearer_token(str(empty))


def test_auth_headers_carry_fresh_bearer_and_principal(tmp_path) -> None:
    # The per-create headers attach BOTH the authenticating SA bearer (for the sidecar
    # TokenReview) and the X-Forwarded-Email app principal (within that authed channel).
    token_file = tmp_path / "sa-token"
    token_file.write_text("sa-jwt-abc\n", encoding="utf-8")
    factory = _factory(bearer_file=str(token_file))
    headers = factory._auth_headers()
    assert headers["Authorization"] == "Bearer sa-jwt-abc"
    assert headers["X-Forwarded-Email"] == "walle@seinetwork.io"
    # Fresh on the next call after a rotation.
    token_file.write_text("sa-jwt-rotated\n", encoding="utf-8")
    assert factory._auth_headers()["Authorization"] == "Bearer sa-jwt-rotated"


def test_aclose_is_idempotent(tmp_path) -> None:
    # B4: a double aclose must not double-close the client pool.
    closed = {"n": 0}

    class _Client:
        async def close(self) -> None:
            closed["n"] += 1

    token_file = tmp_path / "sa-token"
    token_file.write_text("t\n", encoding="utf-8")
    factory = LiveSessionFactory(
        client=_Client(),
        bundle=b"b",
        forwarded_email="walle@seinetwork.io",
        proxy_bearer_file=str(token_file),
    )

    async def _twice() -> None:
        await factory.aclose()
        await factory.aclose()

    asyncio.run(_twice())
    assert closed["n"] == 1  # second call is a no-op


def test_no_secret_in_repr(tmp_path) -> None:
    # SF1: neither the bearer-bearing client nor the bundle bytes appear in the repr (repr=False).
    # The bearer path is a filename (not the secret) so it may appear; the token itself never
    # transits a repr because it is read fresh from the file, never stored on the dataclass.
    token_file = tmp_path / "sa-token"
    token_file.write_text("super-secret-jwt\n", encoding="utf-8")

    class _Client:
        def __repr__(self) -> str:
            return "OmnigentClient(headers={'Authorization': 'Bearer super-secret-jwt'})"

    factory = LiveSessionFactory(
        client=_Client(),
        bundle=b"sensitive-bundle-bytes",
        forwarded_email="walle@seinetwork.io",
        proxy_bearer_file=str(token_file),
    )
    text = repr(factory)
    assert "super-secret-jwt" not in text  # client repr (which could leak a header) is excluded
    assert "sensitive-bundle-bytes" not in text  # bundle bytes excluded
