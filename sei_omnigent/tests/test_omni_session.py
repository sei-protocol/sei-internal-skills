"""Tests for the live session factory's auth + lifecycle posture (PLT-715).

``_omnigent_session`` imports ``omnigent_client`` at module scope (it is the ONE impure seam),
so this whole module is skipped where omnigent is absent (the omnigent-free unit suite) and runs
only in the omnigent-installed environment. It pins the behavioral fixes that do NOT need a live
server: the per-REQUEST FRESH SA-bearer read via ``_FreshBearerAuth`` (S-CRIT), the client built
with the X-Forwarded-Email principal + that Auth, boot-fail-loud on the missing bearer-file env,
the idempotent ``aclose`` (B4), and the no-secret-in-repr posture (SF1). The actual
OmnigentClient/SessionsChat wire calls stay VERIFY-ON-LIVE.
"""

from __future__ import annotations

import asyncio

import httpx
import pytest

pytest.importorskip("omnigent_client")  # the seam imports it at module scope

from sei_omnigent.omni._omnigent_session import (
    BUNDLE_PATH_ENV,
    FORWARDED_EMAIL_ENV,
    PROXY_BEARER_FILE_ENV,
    SERVER_URL_ENV,
    LiveSessionFactory,
    _FreshBearerAuth,
    _proxy_bearer_token,
)

_ALL_ENV = (SERVER_URL_ENV, FORWARDED_EMAIL_ENV, BUNDLE_PATH_ENV, PROXY_BEARER_FILE_ENV)


@pytest.fixture(autouse=True)
def _clean_env(monkeypatch: pytest.MonkeyPatch) -> None:
    for name in _ALL_ENV:
        monkeypatch.delenv(name, raising=False)


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


def test_from_env_builds_client_with_principal_header_and_fresh_bearer_auth(
    monkeypatch: pytest.MonkeyPatch, tmp_path
) -> None:
    # The auth attaches to the CLIENT (not per-create): the static X-Forwarded-Email principal on
    # the client's header store + a _FreshBearerAuth on its auth. SessionsChat.create takes no
    # per-call headers on 0.2.0, so this is the keystone of the auth path.
    bundle = tmp_path / "bundle.tgz"
    bundle.write_bytes(b"bundle")
    token_file = tmp_path / "sa-token"
    token_file.write_text("sa-jwt\n", encoding="utf-8")
    monkeypatch.setenv(SERVER_URL_ENV, "http://omnigent.sei.svc:8080")
    monkeypatch.setenv(FORWARDED_EMAIL_ENV, "walle@seinetwork.io")
    monkeypatch.setenv(BUNDLE_PATH_ENV, str(bundle))
    monkeypatch.setenv(PROXY_BEARER_FILE_ENV, str(token_file))

    factory = LiveSessionFactory.from_env()
    # OmnigentClient forwards the ctor headers/auth onto its underlying httpx client (_http) —
    # verified against installed 0.2.0, so this asserts the wiring directly off that client.
    http = factory.client._http  # type: ignore[attr-defined]
    # The principal is a static header on the standing client.
    assert http.headers["X-Forwarded-Email"] == "walle@seinetwork.io"
    # The bearer rotates via a _FreshBearerAuth (read per request), not a baked header.
    assert isinstance(http.auth, _FreshBearerAuth)
    assert "authorization" not in http.headers  # the bearer is NOT a static header


def test_fresh_bearer_auth_rereads_token_per_request(tmp_path) -> None:
    # S-CRIT: httpx calls auth_flow PER REQUEST, so the token file is re-read each call — a kubelet
    # rotation between requests is picked up without rebuilding the standing client.
    token_file = tmp_path / "sa-token"
    token_file.write_text("token-v1\n", encoding="utf-8")
    auth = _FreshBearerAuth(str(token_file))

    def _bearer() -> str:
        request = httpx.Request("GET", "http://omnigent.sei.svc:8080/x")
        # auth_flow is a generator that mutates the request then yields it.
        next(auth.auth_flow(request))
        return request.headers["Authorization"]

    assert _bearer() == "Bearer token-v1"
    token_file.write_text("token-v2-rotated\n", encoding="utf-8")
    assert _bearer() == "Bearer token-v2-rotated"


def test_fresh_bearer_auth_fails_closed_on_unreadable_or_empty(tmp_path) -> None:
    # The fail-closed read surfaces during the request (→ ERRORED at the driver), not a silent 401.
    auth = _FreshBearerAuth(str(tmp_path / "does-not-exist"))
    request = httpx.Request("GET", "http://omnigent.sei.svc:8080/x")
    with pytest.raises(RuntimeError):
        next(auth.auth_flow(request))


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


def test_aclose_is_idempotent() -> None:
    # B4: a double aclose must not double-close the client pool.
    closed = {"n": 0}

    class _Client:
        async def close(self) -> None:
            closed["n"] += 1

    factory = LiveSessionFactory(client=_Client(), bundle=b"b")

    async def _twice() -> None:
        await factory.aclose()
        await factory.aclose()

    asyncio.run(_twice())
    assert closed["n"] == 1  # second call is a no-op


def test_no_secret_in_repr() -> None:
    # SF1: neither the auth/header-bearing client nor the bundle bytes appear in the repr
    # (repr=False). The bearer never transits a repr — it is read fresh from the file by the
    # client's _FreshBearerAuth, never stored on this dataclass.
    class _Client:
        def __repr__(self) -> str:
            return "OmnigentClient(headers={'X-Forwarded-Email': 'walle@seinetwork.io'})"

    factory = LiveSessionFactory(client=_Client(), bundle=b"sensitive-bundle-bytes")
    text = repr(factory)
    assert "walle@seinetwork.io" not in text  # client repr (which could leak a header) is excluded
    assert "sensitive-bundle-bytes" not in text  # bundle bytes excluded
