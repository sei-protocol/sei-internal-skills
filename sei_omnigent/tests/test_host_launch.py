"""Tests for the B1 host-bearer launch patch (PLT-715).

Proves the patch logic without a live server: the proxy-bearer header is read fresh from the
configured file, the wrap MERGES it into whatever the original returns (managed-token path keeps
its header + gains the bearer), and the install is idempotent + targets a symbol that still exists
on the pinned omnigent (the bump drift-guard). The in-process ``main`` launcher is the thin
entrypoint and is exercised by the live spike, not here.
"""

from __future__ import annotations

import pytest

from sei_omnigent.host_launch import (
    PROXY_BEARER_FILE_ENV,
    _PATCH_MARKER,
    _wrap_build_headers,
    install_proxy_bearer_patch,
    proxy_bearer_header,
)


def test_bearer_header_unset_env_is_empty(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv(PROXY_BEARER_FILE_ENV, raising=False)
    assert proxy_bearer_header() == {}


def test_bearer_header_reads_token_from_file(monkeypatch: pytest.MonkeyPatch, tmp_path) -> None:
    f = tmp_path / "token"
    f.write_text("jwt-abc\n")  # trailing newline is stripped
    monkeypatch.setenv(PROXY_BEARER_FILE_ENV, str(f))
    assert proxy_bearer_header() == {"Authorization": "Bearer jwt-abc"}


def test_bearer_header_reads_fresh_each_call(monkeypatch: pytest.MonkeyPatch, tmp_path) -> None:
    """Per-connect freshness: a rotated token file yields the new value (no caching)."""
    f = tmp_path / "token"
    f.write_text("old")
    monkeypatch.setenv(PROXY_BEARER_FILE_ENV, str(f))
    assert proxy_bearer_header()["Authorization"] == "Bearer old"
    f.write_text("rotated")  # kubelet rewrites in place
    assert proxy_bearer_header()["Authorization"] == "Bearer rotated"


def test_bearer_header_missing_or_empty_file_is_empty(
    monkeypatch: pytest.MonkeyPatch, tmp_path
) -> None:
    monkeypatch.setenv(PROXY_BEARER_FILE_ENV, str(tmp_path / "nope"))
    assert proxy_bearer_header() == {}  # unreadable → {}, fail closed at the sidecar not here
    empty = tmp_path / "empty"
    empty.write_text("   \n")
    monkeypatch.setenv(PROXY_BEARER_FILE_ENV, str(empty))
    assert proxy_bearer_header() == {}


def test_wrap_merges_bearer_into_managed_token_headers(
    monkeypatch: pytest.MonkeyPatch, tmp_path
) -> None:
    """The managed-token path's headers are PRESERVED and gain the Authorization bearer (B1)."""
    f = tmp_path / "token"
    f.write_text("sa-jwt")
    monkeypatch.setenv(PROXY_BEARER_FILE_ENV, str(f))

    # Stand in for HostProcess._build_connect_headers' managed-token return (Origin + host-token).
    def original(self) -> dict[str, str]:
        return {"Origin": "omnigent://internal", "X-Omnigent-Host-Token": "managed-tok"}

    wrapped = _wrap_build_headers(original)
    headers = wrapped(object())
    assert headers == {
        "Origin": "omnigent://internal",
        "X-Omnigent-Host-Token": "managed-tok",  # preserved → server app-check still works
        "Authorization": "Bearer sa-jwt",  # added → sidecar TokenReview gate
    }


def test_wrap_overrides_a_preexisting_bearer(monkeypatch: pytest.MonkeyPatch, tmp_path) -> None:
    """If the original set its own (OIDC/Databricks) bearer, the SA token wins — it's what the
    sidecar validates."""
    f = tmp_path / "token"
    f.write_text("sa-jwt")
    monkeypatch.setenv(PROXY_BEARER_FILE_ENV, str(f))

    def original(self) -> dict[str, str]:
        return {"Authorization": "Bearer oidc-token"}

    assert _wrap_build_headers(original)(object())["Authorization"] == "Bearer sa-jwt"


def test_wrap_noop_when_unconfigured(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv(PROXY_BEARER_FILE_ENV, raising=False)

    def original(self) -> dict[str, str]:
        return {"X-Omnigent-Host-Token": "managed-tok"}

    assert _wrap_build_headers(original)(object()) == {"X-Omnigent-Host-Token": "managed-tok"}


def test_install_targets_existing_symbol_and_is_idempotent() -> None:
    """Drift-guard: the shim's HostProcess (the patch target) must carry _build_connect_headers on
    the pinned omnigent; install is idempotent (wrap-once). Requires omnigent installed."""
    shim = pytest.importorskip("sei_omnigent._omnigent_shim")
    host_process = shim.HostProcess  # the exact object install_proxy_bearer_patch patches
    assert hasattr(host_process, "_build_connect_headers"), "omnigent bump moved the patch target"

    install_proxy_bearer_patch()
    assert getattr(host_process._build_connect_headers, _PATCH_MARKER, False) is True
    patched_ref = host_process._build_connect_headers
    install_proxy_bearer_patch()  # second call is a no-op (marker check)
    assert host_process._build_connect_headers is patched_ref


def test_patch_fires_on_the_real_method(monkeypatch: pytest.MonkeyPatch, tmp_path) -> None:
    """Bind the drift-guard to the REAL upstream method (not a stand-in): install, then invoke the
    patched HostProcess._build_connect_headers and assert the SA bearer is actually merged. Catches
    an upstream signature/behaviour change the symbol-existence check misses -- the failure mode
    most dangerous for an INV-2-gate client (the wrap silently no-ops or throws at connect time).

    Driven via the managed-token path, which builds its headers from env only and never touches
    ``self`` -- so a bare ``object()`` stands in for the host instance without a live server."""
    shim = pytest.importorskip("sei_omnigent._omnigent_shim")
    host_identity = pytest.importorskip("omnigent.host.identity")

    token = tmp_path / "token"
    token.write_text("sa-jwt")
    monkeypatch.setenv(PROXY_BEARER_FILE_ENV, str(token))
    monkeypatch.setenv(host_identity.HOST_TOKEN_ENV_VAR, "managed-tok")  # take the managed path

    install_proxy_bearer_patch()
    headers = shim.HostProcess._build_connect_headers(object())
    assert headers.get("Authorization") == "Bearer sa-jwt"  # the wrap fired on the real method
    assert headers.get("X-Omnigent-Host-Token") == "managed-tok"  # host-token preserved (B1)
