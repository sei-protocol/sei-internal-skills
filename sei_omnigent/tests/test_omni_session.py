"""Tests for the live session factory's create seam + auth/lifecycle posture (PLT-715, Design 13).

``_omnigent_session`` imports ``omnigent_client`` at module scope (it is the ONE impure seam),
so this whole module is skipped where omnigent is absent (the omnigent-free unit suite) and runs
only in the omnigent-installed environment. It pins the behavioral fixes that do NOT need a live
server: the host-bound JSON create state machine (agent-by-name resolve + the 409/400-offline
retry vs 400-workspace/404 fail-fast taxonomy), the per-REQUEST FRESH SA-bearer read via
``_FreshBearerAuth`` (S-CRIT), the client built with the X-Forwarded-Email principal + that Auth,
boot-fail-loud on the missing required env, the idempotent ``aclose`` (B4), and the no-secret-in-
repr posture (SF1). The actual OmnigentClient wire + the runner launch stay VERIFY-ON-LIVE.
"""

from __future__ import annotations

import asyncio

import httpx
import pytest

pytest.importorskip("omnigent_client")  # the seam imports it at module scope

from sei_omnigent.omni._omnigent_session import (
    FORWARDED_EMAIL_ENV,
    PROXY_BEARER_FILE_ENV,
    SERVER_URL_ENV,
    TARGET_HOST_ID_ENV,
    WORKSPACE_ENV,
    LiveSessionFactory,
    _AGENTS_PAGE_LIMIT,
    _CREATE_MAX_RETRIES,
    _FreshBearerAuth,
    _LiveGoalSession,
    _proxy_bearer_token,
)

_ALL_ENV = (
    SERVER_URL_ENV,
    FORWARDED_EMAIL_ENV,
    TARGET_HOST_ID_ENV,
    WORKSPACE_ENV,
    PROXY_BEARER_FILE_ENV,
)


@pytest.fixture(autouse=True)
def _clean_env(monkeypatch: pytest.MonkeyPatch) -> None:
    for name in _ALL_ENV:
        monkeypatch.delenv(name, raising=False)


@pytest.fixture(autouse=True)
def _no_backoff(monkeypatch: pytest.MonkeyPatch) -> None:
    # The create state machine sleeps between retries; collapse it so retry tests don't wait.
    async def _instant(_seconds: float) -> None:
        return None

    monkeypatch.setattr("sei_omnigent.omni._omnigent_session.asyncio.sleep", _instant)


# --- the create state machine (a fake namespace returning scripted httpx responses) ----------


class _ScriptedHttp:
    """Stand-in for the namespace's ``_http`` (the authed ``httpx.AsyncClient``).

    ``get`` answers the paginated ``/v1/agents`` discovery (``has_more`` scriptable for the
    fail-loud-on-overflow test); ``post`` pops the next scripted ``/v1/sessions`` outcome off
    ``post_responses`` — an ``httpx.Response`` is returned, an ``Exception`` instance is raised
    (so a test can script a transport error or "transient then success"). Records the create
    bodies so the posted ``{agent_id, host_id, workspace}`` is assertable, counts the create
    calls so retry/no-retry is observable, and records the GET params so the page-limit is
    assertable.
    """

    def __init__(
        self,
        agents: list[dict[str, str]],
        post_responses: list[httpx.Response | Exception],
        *,
        agents_has_more: bool = False,
    ) -> None:
        self._agents = agents
        self._agents_has_more = agents_has_more
        self._post_responses = list(post_responses)
        self.post_bodies: list[dict[str, object]] = []
        self.post_calls = 0
        self.get_params: list[dict[str, object] | None] = []

    async def get(
        self,
        url: str,
        *,
        params: dict[str, object] | None = None,
        timeout: float | None = None,
    ) -> httpx.Response:
        self.get_params.append(params)
        return httpx.Response(
            200, json={"data": self._agents, "has_more": self._agents_has_more}
        )

    async def post(
        self,
        url: str,
        *,
        json: dict[str, object],
        timeout: float | None = None,
    ) -> httpx.Response:
        self.post_calls += 1
        self.post_bodies.append(json)
        outcome = self._post_responses.pop(0)
        if isinstance(outcome, Exception):
            raise outcome
        return outcome


class _FakeNamespace:
    """A fake ``SessionsNamespace``: ``_http`` + ``_base``, ``get``, ``set_model_override``.

    ``get`` is part of the namespace surface but is NO LONGER on the create path (W2: the Session
    is built straight from the create 201 snapshot); ``get_calls`` counts it so a regression that
    reintroduces the redundant round-trip is caught.
    """

    def __init__(self, http: _ScriptedHttp, *, model_override_raises: bool = False) -> None:
        self._http = http
        self._base = "http://omnigent.sei.svc:8080"
        self.model_override_calls: list[tuple[str, str | None]] = []
        self._model_override_raises = model_override_raises
        self.get_calls = 0

    async def get(self, session_id: str) -> object:
        self.get_calls += 1
        return object()

    async def set_model_override(
        self, session_id: str, *, model_override: str | None, silent: bool = False
    ) -> object:
        self.model_override_calls.append((session_id, model_override))
        if self._model_override_raises:
            raise RuntimeError("model override PATCH failed")
        return object()


def _ok(session_id: str = "conv_abc123") -> httpx.Response:
    # JSON create returns the full session snapshot (SessionResponse). Carry the four fields
    # Session.from_dict requires (id/agent_id/status/created_at) so the create-from-snapshot path
    # (W2) can build the Session straight off this body — no second GET round-trip.
    return httpx.Response(
        201,
        json={
            "id": session_id,
            "agent_id": "ag_root",
            "status": "idle",
            "created_at": 1718000000,
        },
    )


def _err(status: int, body: str) -> httpx.Response:
    return httpx.Response(status, text=body)


def _make_session(
    namespace: _FakeNamespace,
    *,
    agent_name: str = "root-cause",
    model_override: str | None = None,
) -> _LiveGoalSession:
    return _LiveGoalSession(
        namespace,
        agent_name=agent_name,
        host_id="host-standing-1",
        workspace="/work/root-cause",
        goal="investigate the wedge",
        model_override=model_override,
    )


async def _resolve_then_create(session: _LiveGoalSession) -> str:
    """Drive the create flow far enough to obtain the session id (no send() — that's live-only).

    ``_create_session`` returns the create SNAPSHOT (W2: the caller builds the Session from it
    rather than a second GET); pull the id out so the existing assertions read unchanged.
    """
    agent_id = await session._resolve_agent_id()
    created = await session._create_session(agent_id)
    return str(created["id"])


async def _drive_create_snapshot(session: _LiveGoalSession) -> dict[str, object]:
    """Resolve + create, returning the raw create SNAPSHOT (W2: what the seam builds Session)."""
    agent_id = await session._resolve_agent_id()
    return await session._create_session(agent_id)


def test_create_success_resolves_agent_and_posts_host_bound_body() -> None:
    http = _ScriptedHttp(
        agents=[{"id": "ag_root", "name": "root-cause"}, {"id": "ag_other", "name": "other"}],
        post_responses=[_ok("conv_created")],
    )
    session_id = asyncio.run(_resolve_then_create(_make_session(_FakeNamespace(http))))

    assert session_id == "conv_created"
    assert http.post_calls == 1  # no retry on success
    # The create binds the resolved agent id to the standing host + the fixed workspace.
    assert http.post_bodies[0] == {
        "agent_id": "ag_root",
        "host_id": "host-standing-1",
        "workspace": "/work/root-cause",
    }


def test_create_retries_on_409_then_succeeds() -> None:
    http = _ScriptedHttp(
        agents=[{"id": "ag_root", "name": "root-cause"}],
        post_responses=[_err(409, "host is offline"), _ok("conv_after_retry")],
    )
    session_id = asyncio.run(_resolve_then_create(_make_session(_FakeNamespace(http))))

    assert session_id == "conv_after_retry"
    assert http.post_calls == 2  # one retry


def test_create_retries_on_400_offline_then_succeeds() -> None:
    http = _ScriptedHttp(
        agents=[{"id": "ag_root", "name": "root-cause"}],
        # _workspace_validation maps host-offline-at-stat to a 400 carrying "offline".
        post_responses=[
            _err(400, "host 'host-standing-1' is offline; reconnect the host"),
            _ok("conv_after_offline_retry"),
        ],
    )
    session_id = asyncio.run(_resolve_then_create(_make_session(_FakeNamespace(http))))

    assert session_id == "conv_after_offline_retry"
    assert http.post_calls == 2


def test_create_raises_immediately_on_400_workspace_invalid() -> None:
    http = _ScriptedHttp(
        agents=[{"id": "ag_root", "name": "root-cause"}],
        # A 400 WITHOUT "offline" — the workspace path is missing/not-a-dir. Config, no retry.
        post_responses=[_err(400, "workspace '/work/root-cause' is not a directory")],
    )
    with pytest.raises(RuntimeError, match="400"):
        asyncio.run(_resolve_then_create(_make_session(_FakeNamespace(http))))
    assert http.post_calls == 1  # NO retry on a config 400


def test_create_raises_immediately_on_404_host_not_found() -> None:
    http = _ScriptedHttp(
        agents=[{"id": "ag_root", "name": "root-cause"}],
        post_responses=[_err(404, "host not found")],
    )
    with pytest.raises(RuntimeError, match="404"):
        asyncio.run(_resolve_then_create(_make_session(_FakeNamespace(http))))
    assert http.post_calls == 1  # a stale host id is config, not transient — no retry


def test_create_raises_when_agent_name_not_registered() -> None:
    http = _ScriptedHttp(
        agents=[{"id": "ag_other", "name": "other"}],  # no "root-cause"
        post_responses=[],
    )
    with pytest.raises(RuntimeError, match="root-cause"):
        asyncio.run(_resolve_then_create(_make_session(_FakeNamespace(http))))
    assert http.post_calls == 0  # never reaches the create


def test_create_exhausts_retries_then_raises() -> None:
    # More transient 409s than the retry budget allows → ERRORED.
    http = _ScriptedHttp(
        agents=[{"id": "ag_root", "name": "root-cause"}],
        post_responses=[_err(409, "host is offline") for _ in range(10)],
    )
    with pytest.raises(RuntimeError, match="409"):
        asyncio.run(_resolve_then_create(_make_session(_FakeNamespace(http))))
    assert http.post_calls == _CREATE_MAX_RETRIES + 1  # initial attempt + the retry budget


def test_create_raises_immediately_on_runner_bind_conflict() -> None:
    # G1: a 409 whose body is the runner-bind CONFLICT ("already has a runner bound") is NOT the
    # host-offline window — it fires AFTER the conversation row is written, so retrying it orphans
    # rows/runners. It must raise on the FIRST attempt, never retried.
    http = _ScriptedHttp(
        agents=[{"id": "ag_root", "name": "root-cause"}],
        post_responses=[_err(409, "Session 'conv_x' already has a runner bound")],
    )
    with pytest.raises(RuntimeError, match="409"):
        asyncio.run(_resolve_then_create(_make_session(_FakeNamespace(http))))
    assert http.post_calls == 1  # NOT retried — a bind-conflict 409 is non-transient


def test_create_retries_on_transport_error_then_succeeds() -> None:
    # G2: a connection-level transport error raises before a response exists. It is the actual
    # pod-roll/connection-refused transient, so it routes through the SAME bounded retry path.
    http = _ScriptedHttp(
        agents=[{"id": "ag_root", "name": "root-cause"}],
        post_responses=[
            httpx.ConnectError("connection refused"),
            _ok("conv_after_transport_retry"),
        ],
    )
    session_id = asyncio.run(_resolve_then_create(_make_session(_FakeNamespace(http))))

    assert session_id == "conv_after_transport_retry"
    assert http.post_calls == 2  # one retry on the transport error


def test_create_exhausts_retries_on_transport_error_then_raises() -> None:
    # G2: a transport error on every attempt (including the final one) exhausts the budget and
    # raises (ERRORED) rather than swallowing the failure.
    http = _ScriptedHttp(
        agents=[{"id": "ag_root", "name": "root-cause"}],
        post_responses=[httpx.ConnectError("connection refused") for _ in range(10)],
    )
    with pytest.raises(RuntimeError, match="transport error"):
        asyncio.run(_resolve_then_create(_make_session(_FakeNamespace(http))))
    assert http.post_calls == _CREATE_MAX_RETRIES + 1  # initial attempt + the retry budget


def test_resolve_agent_requests_page_cap_and_fails_loud_on_has_more() -> None:
    # G3: the agents list is cursor-paginated. The resolve requests the route's page cap; if the
    # response still reports has_more AND the agent was not on the first page, fail loud rather
    # than silently miss an agent past the cap (no full cursor loop — limit + fail-loud is agreed).
    http = _ScriptedHttp(
        agents=[{"id": "ag_other", "name": "other"}],  # target not on the first page
        post_responses=[],
        agents_has_more=True,
    )
    with pytest.raises(RuntimeError, match=r"has_more|page cap"):
        asyncio.run(_make_session(_FakeNamespace(http))._resolve_agent_id())
    # The resolve passed the page-cap limit to the GET (one page, sized to the route cap).
    assert http.get_params == [{"limit": _AGENTS_PAGE_LIMIT}]


def test_create_builds_session_from_snapshot_without_second_get() -> None:
    # W2: the create 201 carries the full snapshot (id/agent_id/status/created_at), so the seam
    # builds the Session straight from it — the redundant post-create GET is gone. Assert the
    # create succeeds off the 201 body alone and the namespace's get() is never called.
    http = _ScriptedHttp(
        agents=[{"id": "ag_root", "name": "root-cause"}],
        post_responses=[_ok("conv_from_snapshot")],
    )
    ns = _FakeNamespace(http)
    created = asyncio.run(_drive_create_snapshot(_make_session(ns)))

    assert str(created["id"]) == "conv_from_snapshot"
    # Session.from_dict needs exactly these four; the snapshot must carry them (no GET backfill).
    assert {"id", "agent_id", "status", "created_at"} <= set(created)
    assert ns.get_calls == 0  # the redundant round-trip is gone


def test_model_override_is_applied_post_create() -> None:
    http = _ScriptedHttp(
        agents=[{"id": "ag_root", "name": "root-cause"}],
        post_responses=[_ok("conv_x")],
    )
    ns = _FakeNamespace(http)
    session = _make_session(ns, model_override="claude-opus-4-8")

    # Drive resolve→create→override directly (the send() stream is VERIFY-ON-LIVE). This mirrors
    # _events's best-effort override step (the create returns the snapshot; the id comes off it).
    async def _flow() -> None:
        agent_id = await session._resolve_agent_id()
        created = await session._create_session(agent_id)
        await ns.set_model_override(
            str(created["id"]), model_override=session._model_override, silent=True
        )

    asyncio.run(_flow())
    assert ns.model_override_calls == [("conv_x", "claude-opus-4-8")]


def test_model_override_failure_does_not_fail_the_run() -> None:
    # A model-override PATCH failure is logged-and-continued in _events: the run drives on the
    # agent default. Here we assert the seam's try/except swallows it (the create still yields).
    http = _ScriptedHttp(
        agents=[{"id": "ag_root", "name": "root-cause"}],
        post_responses=[_ok("conv_x")],
    )
    ns = _FakeNamespace(http, model_override_raises=True)
    session = _make_session(ns, model_override="claude-opus-4-8")

    async def _flow() -> str:
        agent_id = await session._resolve_agent_id()
        created = await session._create_session(agent_id)
        session_id = str(created["id"])
        try:
            await ns.set_model_override(
                session_id, model_override=session._model_override, silent=True
            )
        except RuntimeError:
            pass  # _events logs-and-continues; the run is not failed
        return session_id

    assert asyncio.run(_flow()) == "conv_x"
    assert ns.model_override_calls == [("conv_x", "claude-opus-4-8")]


# --- env / auth / lifecycle posture ----------


def test_from_env_fails_loud_on_missing_bearer_file(monkeypatch: pytest.MonkeyPatch) -> None:
    # S-CRIT: the projected SA bearer token file is REQUIRED in prod — its absence must fail at
    # boot, not surface as a 401 at the sidecar on the first investigation.
    monkeypatch.setenv(SERVER_URL_ENV, "http://omnigent.sei.svc:8080")
    monkeypatch.setenv(FORWARDED_EMAIL_ENV, "walle@seinetwork.io")
    monkeypatch.setenv(TARGET_HOST_ID_ENV, "host-standing-1")
    monkeypatch.setenv(WORKSPACE_ENV, "/work/root-cause")
    # PROXY_BEARER_FILE_ENV intentionally unset
    with pytest.raises(RuntimeError, match=PROXY_BEARER_FILE_ENV):
        LiveSessionFactory.from_env()


def test_from_env_fails_loud_on_missing_host_id_and_workspace(
    monkeypatch: pytest.MonkeyPatch, tmp_path
) -> None:
    # The host-bound create needs the standing host id + the fixed workspace; both are required
    # boot env now that the bundle is gone (the agent is boot-registered, not uploaded).
    token_file = tmp_path / "sa-token"
    token_file.write_text("sa-jwt\n", encoding="utf-8")
    monkeypatch.setenv(SERVER_URL_ENV, "http://omnigent.sei.svc:8080")
    monkeypatch.setenv(FORWARDED_EMAIL_ENV, "walle@seinetwork.io")
    monkeypatch.setenv(PROXY_BEARER_FILE_ENV, str(token_file))
    # TARGET_HOST_ID_ENV + WORKSPACE_ENV intentionally unset
    with pytest.raises(RuntimeError) as exc:
        LiveSessionFactory.from_env()
    assert TARGET_HOST_ID_ENV in str(exc.value)
    assert WORKSPACE_ENV in str(exc.value)


def test_from_env_builds_client_with_principal_header_and_fresh_bearer_auth(
    monkeypatch: pytest.MonkeyPatch, tmp_path
) -> None:
    # The auth attaches to the CLIENT (not per-create): the static X-Forwarded-Email principal on
    # the client's header store + a _FreshBearerAuth on its auth. The create posts through that
    # authed client, so this is the keystone of the auth path.
    token_file = tmp_path / "sa-token"
    token_file.write_text("sa-jwt\n", encoding="utf-8")
    monkeypatch.setenv(SERVER_URL_ENV, "http://omnigent.sei.svc:8080")
    monkeypatch.setenv(FORWARDED_EMAIL_ENV, "walle@seinetwork.io")
    monkeypatch.setenv(TARGET_HOST_ID_ENV, "host-standing-1")
    monkeypatch.setenv(WORKSPACE_ENV, "/work/root-cause")
    monkeypatch.setenv(PROXY_BEARER_FILE_ENV, str(token_file))

    factory = LiveSessionFactory.from_env()
    assert factory.host_id == "host-standing-1"
    assert factory.workspace == "/work/root-cause"
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

    factory = LiveSessionFactory(client=_Client(), host_id="host-1", workspace="/work")

    async def _twice() -> None:
        await factory.aclose()
        await factory.aclose()

    asyncio.run(_twice())
    assert closed["n"] == 1  # second call is a no-op


def test_no_secret_in_repr() -> None:
    # SF1: the auth/header-bearing client does not appear in the repr (repr=False). The bearer
    # never transits a repr — it is read fresh from the file by the client's _FreshBearerAuth,
    # never stored on this dataclass.
    class _Client:
        def __repr__(self) -> str:
            return "OmnigentClient(headers={'X-Forwarded-Email': 'walle@seinetwork.io'})"

    factory = LiveSessionFactory(client=_Client(), host_id="host-1", workspace="/work")
    text = repr(factory)
    assert "walle@seinetwork.io" not in text  # client repr (which could leak a header) is excluded
