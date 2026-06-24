"""Live-run glue binding the real ``omnigent_client`` session to the budget driver (PLT-715).

IMPURE — this is the one omni/ module that imports ``omnigent_client`` (the live path).
It is deliberately NOT re-exported by ``omni/__init__``: the pure driver (``driver.py``)
and its unit tests never import this, so the unit suite needs no omnigent install. Import
it only on the live run.

It supplies what :func:`omni.driver.drive_to_terminal` needs to drive a *real* session:

* :func:`make_extractors` — the ``(token_delta, is_iteration, artifact_chunk)`` triple,
  reading omnigent 0.2.0's typed stream events. The wire carries **cumulative**
  ``Response.usage.total_tokens`` on each turn-terminal event, so ``token_delta`` diffs
  against a running max (the driver accumulates additively).
* :class:`GoalSession` — wraps a :class:`SessionsChat` so it satisfies
  :class:`omni.driver.SessionLike` *and* posts the goal: its ``stream()`` calls
  ``chat.send(goal)`` (post-the-goal-then-stream-the-turn), ``cancel()`` →
  ``chat.cancel()``, ``status`` → ``chat.status``.
* :class:`LiveSessionFactory` — the production ``SessionFactory`` the receiver calls per
  investigation: it mints a :class:`SessionsChat` against the standing host (the in-cluster
  omnigent server, header-mode identity ``omni-root-cause``) and returns the
  ``GoalSession`` + the extractor triple. This is the ONE impure seam the receiver's
  ``serve_receiver`` entrypoint wires; it stays here (off ``omni/__init__``) so the
  omnigent-free unit suite never imports ``omnigent_client``.

CREATE FLOW (Design 13 slice-3): a session is a **host-bound JSON create against a
boot-registered built-in agent**, NOT a per-trigger bundle upload. The agent is registered
on the coordinator at boot (``--agent``); the factory resolves its id by name off
``GET /v1/agents`` and issues ``POST /v1/sessions`` JSON ``{agent_id, host_id, workspace}``
bound to the standing host. The create runs a **bounded state machine** on the HTTP status —
409 / 400-offline are the transient host roll/restart window (retried with backoff), while
400-workspace-invalid / 404-host-not-found are config conditions (raised immediately). The
server returns 200 even when the runner launch fails ("created-but-launch-failed"); that
surfaces later as a 503 on the first event, which the driver classifies on the goal-post
outcome (ERRORED), so create-success is NOT trusted as runner-ready.

SPIKE-GRADE IMPORT NOTE: the turn-terminal event classes are not on the public
``omnigent_client`` surface, so they are imported from the private ``_sessions_chat``
module here, in ONE place, with this marker. Re-confirm on the next omnigent pin bump
(same discipline as ``_omnigent_shim``). The production receiver should prefer the public
``StreamHooks`` lifecycle (``on_response_end``) over private event-type imports.

VERIFY ON FIRST LIVE RUN (env-coupled; not exercised by the unit suite):
  - whether a multi-turn ``/root-cause`` loop surfaces as repeated ``send()`` turns or one
    long turn (the extractors handle both — ``is_iteration`` counts turn-terminal events);
  - that the server + kube-rbac-proxy sidecar READ the forwarded auth — the X-Forwarded-Email
    principal off the static client header, the bearer off the per-request ``_FreshBearerAuth``
    flow. (RESOLVED on installed 0.2.0: auth attaches to the CLIENT ctor — the JSON create
    posts through ``namespace._http``, the authed httpx client — and ``OmnigentClient`` forwards
    ``headers``/``auth`` onto its underlying httpx client, so the bearer rotates via the Auth,
    read fresh per request.)
  - the create-handshake latency (the ``GET /v1/agents`` + ``POST /v1/sessions`` + retry
    budget) against the driver's wall-clock budget; that the created session's owner resolves
    to the X-Forwarded-Email principal; the claude-native-headless launch on the standing
    host; and INV-11 egress — all are prove-the-run items the live cut closes.
  - that ``Response.usage`` is populated on terminal events (it is ``Usage | None`` → 0 delta
    when absent, which is safe — the run is then bounded by wall-clock/iterations, not tokens).

Design 13 (Router core; slice-3 create-seam); Design 12 §2, §3.2.
"""

from __future__ import annotations

import asyncio
import logging
import os
import random
from collections.abc import AsyncIterator, Callable, Iterator
from dataclasses import dataclass, field
from pathlib import Path
from typing import Protocol

import httpx

# --- spike-grade private import (see module docstring); re-confirm on omnigent bump ---
from omnigent_client._sessions_chat import (
    _TURN_TERMINAL_EVENT_TYPES,
    OutputTextDeltaEvent,
)

_log = logging.getLogger("sei_omnigent.omni._omnigent_session")

#: Bounded retry on the transient create window (host offline at the workspace-stat or
#: launch-bind moment — a roll/restart). Small + jittered-exponential, capped well under the
#: driver's wall-clock budget so an exhausted retry still leaves the run time to fail ERRORED.
_CREATE_MAX_RETRIES = 3
_CREATE_BACKOFF_BASE_S = 0.5


def make_extractors() -> tuple[
    Callable[[object], int],
    Callable[[object], bool],
    Callable[[object], str],
]:
    """Return ``(token_delta, is_iteration, artifact_chunk)`` for :func:`drive_to_terminal`.

    ``token_delta`` is stateful (a closure over the running-max cumulative total): the wire
    reports cumulative ``Response.usage.total_tokens`` per turn-terminal event, so the
    per-event increment is ``max(0, cumulative_now - cumulative_seen)``. Build a fresh
    triple per run (the closure must not leak token state across runs).
    """
    seen_total = 0

    def token_delta(event: object) -> int:
        nonlocal seen_total
        if isinstance(event, _TURN_TERMINAL_EVENT_TYPES):
            usage = getattr(event.response, "usage", None)  # type: ignore[attr-defined]
            total = int(getattr(usage, "total_tokens", 0) or 0)
            # The wire reports MONOTONIC cumulative total_tokens per turn-terminal event;
            # the max(0,...) clamp is correct for that. NAMED LIMITATION (not a silent gap):
            # a regression (counter reset / out-of-order terminal) would under-count and fail
            # the token axis OPEN — assumed-monotonic per run, so build a fresh triple per run.
            delta = max(0, total - seen_total)
            seen_total = max(seen_total, total)
            return delta
        return 0

    def is_iteration(event: object) -> bool:
        # A turn-terminal event (Completed/Failed/Incomplete/Cancelled) closes one turn.
        return isinstance(event, _TURN_TERMINAL_EVENT_TYPES)

    def artifact_chunk(event: object) -> str:
        return event.delta if isinstance(event, OutputTextDeltaEvent) else ""

    return token_delta, is_iteration, artifact_chunk


class _ChatLike(Protocol):
    """The ``SessionsChat`` surface :class:`GoalSession` adapts.

    A typed inbound seam so the adapter carries no ``type: ignore`` and a rename of the
    chat's ``send``/``cancel``/``status`` is caught structurally. ``SessionsChat`` satisfies
    this (``send(input, *, files=None)`` matches a positional ``send(goal)`` call).
    """

    def send(self, goal: str) -> AsyncIterator[object]: ...
    async def cancel(self) -> None: ...
    @property
    def status(self) -> str: ...


class GoalSession:
    """Adapt a ``SessionsChat`` to :class:`omni.driver.SessionLike`, posting ``goal`` on stream.

    ``stream()`` posts the goal and yields the turn's events (``chat.send(goal)``); the driver
    drives that stream. ``cancel()``/``status`` delegate. A ``SessionsChat`` already exposes
    ``cancel()`` + ``status``; this wrapper's only addition is turning the bare subscription
    into a goal-posting one so the agent actually does work for the budget to bound.
    """

    def __init__(self, chat: _ChatLike, goal: str) -> None:
        self._chat = chat
        self._goal = goal

    def stream(self) -> AsyncIterator[object]:
        return self._chat.send(self._goal)

    async def cancel(self) -> None:
        await self._chat.cancel()

    @property
    def status(self) -> str:
        return self._chat.status


# --- the live SessionFactory (the production seam serve_receiver wires) ----------

#: Env: the in-cluster omnigent server base URL the host registered against (http→the host
#: derives the WS tunnel). The receiver reaches the server over the same Service.
SERVER_URL_ENV = "OMNI_RECEIVER_SERVER_URL"
#: Env: the header-mode trusted identity the server resolves the run under (the sidecar sets
#: ``X-Forwarded-Email`` after TokenReview in prod; this is the WallE root-cause SA identity).
FORWARDED_EMAIL_ENV = "OMNI_RECEIVER_FORWARDED_EMAIL"
#: Env: the standing host's registered id — the ``host_id`` value posted in the create body that
#: binds the session to that host. NOT the host's own ``OMNIGENT_HOST_*`` identity triple (that
#: authenticates the host→server tunnel); this is just the create-body reference to it.
TARGET_HOST_ID_ENV = "OMNI_RECEIVER_TARGET_HOST_ID"
#: Env: a fixed absolute directory that must PRE-EXIST on the standing host — the session's
#: workspace (cwd). The server stats it on the host at create; a missing/not-a-dir path is a
#: 400 workspace-invalid (raised immediately, not retried — a config/page condition).
WORKSPACE_ENV = "OMNI_RECEIVER_WORKSPACE"
#: Env: path to the projected ``omnigent-api`` SA token the kube-rbac-proxy sidecar TokenReviews
#: (mirrors host_launch's PROXY_BEARER_FILE_ENV). Read FRESH per session-create, NOT baked once
#: at boot: the kubelet rotates the projected token (~hourly), so a standing client carrying a
#: boot-time bearer goes stale and the sidecar 401s mid-life. Required in prod (fail-loud at boot).
PROXY_BEARER_FILE_ENV = "OMNI_RECEIVER_PROXY_BEARER_TOKEN_FILE"


def _proxy_bearer_token(path: str) -> str:
    """Read the projected SA token FRESH from ``path`` (per session-create). Fail-closed.

    Read on every create so the kubelet's in-place token rotation is picked up at the next
    session; the JWT's own ``exp`` governs validity (no local expiry bookkeeping). An unreadable
    or empty file raises — the create then fails ERRORED rather than reaching the sidecar with a
    stale/absent bearer and 401ing (the driver classifies the raise as ERRORED, the honest note).
    """
    try:
        token = Path(path).read_text(encoding="utf-8").strip()
    except OSError as exc:
        raise RuntimeError(
            f"{PROXY_BEARER_FILE_ENV}={path!r} is unreadable: {exc} (cannot authenticate the "
            "session-create to the kube-rbac-proxy sidecar)."
        ) from exc
    if not token:
        raise RuntimeError(
            f"{PROXY_BEARER_FILE_ENV}={path!r} is empty (cannot authenticate the session-create)."
        )
    return token


class _FreshBearerAuth(httpx.Auth):
    """An ``httpx.Auth`` that re-reads the projected SA bearer from its token file PER REQUEST.

    httpx invokes :meth:`auth_flow` on every outbound request, so the token file is read fresh
    each call — the kubelet's in-place rotation (~hourly) is picked up without rebuilding the
    standing client. Fail-closed by :func:`_proxy_bearer_token`: an unreadable/empty file raises
    during the request, which surfaces through the driver as an ERRORED run rather than reaching
    the sidecar with a stale/absent bearer and 401ing.
    """

    def __init__(self, token_file: str) -> None:
        self._token_file = token_file

    def auth_flow(self, request: httpx.Request) -> Iterator[httpx.Request]:
        request.headers["Authorization"] = f"Bearer {_proxy_bearer_token(self._token_file)}"
        yield request


def _is_transient_create_status(status: int, body: str) -> bool:
    """Classify a non-2xx create status as the transient host roll/restart window.

    The standing host can be momentarily offline at the workspace-stat or launch-bind moment (a
    pod roll / restart). The server maps that to **409** (``_host_launch`` — "host is offline" /
    an already-bound runner conflict) or to a **400 whose body contains "offline"**
    (``_workspace_validation`` — the host didn't reply to the workspace stat). Both are retryable;
    every other 4xx is a config/page condition (see :meth:`_LiveGoalSession._create_session`).
    """
    if status == 409:
        return True
    return status == 400 and "offline" in body.lower()


class _LiveGoalSession:
    """A :class:`omni.driver.SessionLike` that lazily mints a real ``SessionsChat`` per run.

    The host-bound JSON create is async but :meth:`stream` is the sync-returns-async-iterator
    the driver expects, so the session is created on the FIRST stream iteration (inside the
    async generator): resolve the boot-registered agent's id by name, JSON-create a host-bound
    session (with the bounded retry state machine), attach a drivable :class:`SessionsChat`, then
    post the goal and yield the turn's events. ``cancel()`` before the chat exists is a no-op
    (nothing to cancel); ``status`` reports ``pending`` until then. One investigation = one
    instance (no chat reuse across runs).
    """

    def __init__(
        self,
        namespace: object,
        *,
        agent_name: str,
        host_id: str,
        workspace: str,
        goal: str,
        model_override: str | None,
    ) -> None:
        self._namespace = namespace
        self._agent_name = agent_name
        self._host_id = host_id
        self._workspace = workspace
        self._goal = goal
        self._model_override = model_override
        self._chat: _ChatLike | None = None

    async def _resolve_agent_id(self) -> str:
        """Resolve the boot-registered agent's id by name off ``GET /v1/agents``. Fail-loud.

        The built-in agent set is deploy-time-static, so resolving per-create is fine. A name
        with no matching agent is a config error (the wrong ``bundle_ref`` or an un-registered
        agent) → ``RuntimeError`` (the driver classifies the raise as ERRORED).
        """
        resp = await self._namespace._http.get(  # type: ignore[attr-defined]
            f"{self._namespace._base}/v1/agents"  # type: ignore[attr-defined]
        )
        if resp.status_code != 200:
            raise RuntimeError(
                f"GET /v1/agents returned {resp.status_code} resolving agent "
                f"{self._agent_name!r}: {resp.text}"
            )
        # The list is paginated: items ride under ``data``, each carrying ``{id, name, ...}``.
        items = resp.json().get("data", [])
        for item in items:
            if item.get("name") == self._agent_name:
                return str(item["id"])
        raise RuntimeError(
            f"no boot-registered agent named {self._agent_name!r} in /v1/agents "
            f"(found: {[i.get('name') for i in items]!r})"
        )

    async def _create_session(self, agent_id: str) -> str:
        """JSON host-bound create with the bounded retry state machine. Returns the session id.

        ``POST /v1/sessions`` JSON ``{agent_id, host_id, workspace}`` binds the session to the
        standing host. The state machine on the HTTP status:

        * **409 / 400-offline** — the transient host roll/restart window: retry with bounded
          jittered-exponential backoff (:data:`_CREATE_MAX_RETRIES`); an exhausted retry raises
          (ERRORED).
        * **400 workspace-invalid** (a 400 NOT containing "offline" — the workspace path is
          missing/not-a-dir) or **404 host-not-found** (a stale ``host_id``) — a config/page
          condition: raise immediately, no retry.

        A 200 is NOT trusted as runner-ready: the server returns 200 even when the runner launch
        fails, which surfaces later as a 503 on the first event (the driver classifies the run on
        the goal-post outcome). So there is deliberately no readiness probe here.
        """
        last_exc: RuntimeError | None = None
        for attempt in range(_CREATE_MAX_RETRIES + 1):
            resp = await self._namespace._http.post(  # type: ignore[attr-defined]
                f"{self._namespace._base}/v1/sessions",  # type: ignore[attr-defined]
                json={
                    "agent_id": agent_id,
                    "host_id": self._host_id,
                    "workspace": self._workspace,
                },
            )
            if resp.status_code // 100 == 2:
                created = resp.json()
                # JSON create returns the full session snapshot (``id``); the multipart create
                # returns ``{"session_id": ...}``. Accept either so a server-shape shift doesn't
                # strand us.
                session_id = created.get("id") or created.get("session_id")
                if not session_id:
                    raise RuntimeError(f"POST /v1/sessions 2xx carried no session id: {created!r}")
                return str(session_id)
            body = resp.text
            transient = _is_transient_create_status(resp.status_code, body)
            if transient and attempt < _CREATE_MAX_RETRIES:
                # Jittered exponential backoff; capped attempts keep the total well under the
                # driver wall-clock budget so an exhausted retry still fails ERRORED in time.
                backoff = _CREATE_BACKOFF_BASE_S * (2**attempt) * (1 + random.random())
                _log.warning(
                    "session-create transient %s (attempt %d/%d), retrying in %.2fs: %s",
                    resp.status_code,
                    attempt + 1,
                    _CREATE_MAX_RETRIES,
                    backoff,
                    body,
                )
                last_exc = RuntimeError(
                    f"POST /v1/sessions exhausted retries on transient {resp.status_code}: {body}"
                )
                await asyncio.sleep(backoff)
                continue
            raise RuntimeError(f"POST /v1/sessions failed {resp.status_code}: {body}")
        # Loop fell through: the last attempt was a transient status (no immediate raise above).
        raise last_exc or RuntimeError("POST /v1/sessions exhausted retries")

    async def _events(self) -> AsyncIterator[object]:
        from omnigent_client._sessions_chat import SessionsChat  # noqa: PLC0415 -- impure seam

        # Auth attaches to the standing client (the X-Forwarded-Email principal on its static
        # header store + the per-request _FreshBearerAuth on namespace._http): the create posts
        # through that authed client, the FRESH-read SA bearer (S-CRIT) read per REQUEST by the
        # Auth flow, so a kubelet-rotated token is current without rebuilding the client.
        agent_id = await self._resolve_agent_id()
        session_id = await self._create_session(agent_id)
        if self._model_override is not None:
            # Best-effort, post-create: model selection is deferred/non-load-bearing, so a
            # failure must not fail the run. VERIFY-ON-LIVE whether this override lands before
            # the first drive (the create→get→send sequence is sub-second, the override a
            # separate PATCH).
            try:
                await self._namespace.set_model_override(  # type: ignore[attr-defined]
                    session_id, model_override=self._model_override, silent=True
                )
            except Exception:  # best-effort; logged, never fails the run
                _log.warning(
                    "set_model_override(%r) failed for session %s; continuing on the agent "
                    "default model",
                    self._model_override,
                    session_id,
                    exc_info=True,
                )
        session = await self._namespace.get(session_id)  # type: ignore[attr-defined]
        chat = SessionsChat(self._namespace, None, None, session)
        self._chat = chat
        async for event in chat.send(self._goal):
            yield event

    def stream(self) -> AsyncIterator[object]:
        return self._events()

    async def cancel(self) -> None:
        if self._chat is not None:
            await self._chat.cancel()

    @property
    def status(self) -> str:
        return self._chat.status if self._chat is not None else "pending"


@dataclass
class LiveSessionFactory:
    """The production ``SessionFactory``: mint a goal-bound live session for one investigation.

    Holds the standing :class:`OmnigentClient` (one keep-alive client for the receiver's life;
    closed via :meth:`aclose` on shutdown) + the standing host's id + the fixed workspace. Calling
    it with a ``RunContext`` returns the receiver's ``SessionLaunch`` tuple: a lazy live session
    bound to ``ctx.goal`` (created host-bound against the boot-registered agent named by
    ``ctx.bundle_ref``) + a FRESH extractor triple (the token-delta closure must not leak state
    across runs — see :func:`make_extractors`).

    Auth posture (S-CRIT; mirrors host_launch's B1): the kube-rbac-proxy sidecar fronting the
    server TokenReviews an ``Authorization: Bearer <SA token>``; ``X-Forwarded-Email`` is the app
    principal resolved WITHIN that authed channel (a spoofable claim on its own). Both attach to
    the STANDING client (built in :meth:`from_env`): the static X-Forwarded-Email on the client's
    header store, the bearer via :class:`_FreshBearerAuth` which re-reads the projected
    ``omnigent-api`` SA token file per REQUEST (the kubelet rotates it ~hourly — a boot-time token
    goes stale, so it is never baked once). The create (``GET /v1/agents`` + ``POST /v1/sessions``)
    posts through this client's ``_http``, so the same auth rides every create call.

    VERIFY-ON-LIVE (the prove-the-run is still open; see module docstring): the create-handshake
    latency against the driver budget, that the created session's owner resolves to the
    X-Forwarded-Email principal, the claude-native-headless launch on the standing host, and INV-11
    egress. Built to the best-known omnigent 0.2.0 client API — NOT asserted end-to-end; the live
    run closes it.
    """

    #: an omnigent_client.OmnigentClient — typed object so this stays import-light. repr=False:
    #: the client carries the auth/header state in its repr; keep it off this dataclass's repr.
    client: object = field(repr=False)
    #: the standing host's registered id (a create-body value, not the host's identity triple).
    host_id: str
    #: a fixed directory that must pre-exist on the standing host (the session workspace/cwd).
    workspace: str
    _closed: bool = field(default=False, init=False, repr=False)

    @classmethod
    def from_env(cls) -> LiveSessionFactory:
        """Build the factory from env (server URL + identity + host id + workspace + bearer file).

        Fail-loud, mirroring the receiver's boot discipline: a missing server URL / identity /
        target host id / workspace / bearer file is a misconfiguration that must fail at boot, not
        on the first webhook (a half-configured factory would error every investigation). The agent
        is boot-registered on the coordinator (``--agent``), so the factory reads no bundle. The SA
        bearer is NOT read here (the client's :class:`_FreshBearerAuth` reads it fresh per request)
        — only its path is required.
        """
        server_url = (os.environ.get(SERVER_URL_ENV) or "").strip()
        forwarded_email = (os.environ.get(FORWARDED_EMAIL_ENV) or "").strip()
        host_id = (os.environ.get(TARGET_HOST_ID_ENV) or "").strip()
        workspace = (os.environ.get(WORKSPACE_ENV) or "").strip()
        bearer_file = (os.environ.get(PROXY_BEARER_FILE_ENV) or "").strip()
        missing = [
            name
            for name, value in (
                (SERVER_URL_ENV, server_url),
                (FORWARDED_EMAIL_ENV, forwarded_email),
                (TARGET_HOST_ID_ENV, host_id),
                (WORKSPACE_ENV, workspace),
                (PROXY_BEARER_FILE_ENV, bearer_file),
            )
            if not value
        ]
        if missing:
            raise RuntimeError(
                f"LiveSessionFactory missing required env: {', '.join(missing)}. The receiver "
                "cannot launch an investigation without the server URL, the trusted identity, "
                "the standing host id, the workspace, and the projected SA bearer token file "
                "(fail-closed at boot)."
            )

        # The standing client carries the auth: the static X-Forwarded-Email principal on its
        # header store + the per-request _FreshBearerAuth (the SA bearer is read fresh per request,
        # so a kubelet-rotated token is current and a stale boot-time bearer never reaches the
        # sidecar). OmnigentClient forwards headers/auth onto its httpx client (verified on 0.2.0).
        from omnigent_client import OmnigentClient  # noqa: PLC0415 -- the single impure seam

        client = OmnigentClient(
            base_url=server_url,
            headers={"X-Forwarded-Email": forwarded_email},
            auth=_FreshBearerAuth(bearer_file),
        )
        return cls(client=client, host_id=host_id, workspace=workspace)

    def __call__(self, ctx: object) -> tuple[object, ...]:
        # ctx is an omni.router.RunContext (typed object so this module stays router-import-
        # light + the single-coupling seam carries no router dependency cycle). ctx.bundle_ref is
        # the boot-registered agent NAME, ctx.goal the contained rendered investigation goal,
        # ctx.model_override the (optional) per-run model.
        session = _LiveGoalSession(
            self._namespace(),
            agent_name=ctx.bundle_ref,  # type: ignore[attr-defined]
            host_id=self.host_id,
            workspace=self.workspace,
            goal=ctx.goal,  # type: ignore[attr-defined]
            model_override=ctx.model_override,  # type: ignore[attr-defined]
        )
        return (session, *make_extractors())

    def _namespace(self) -> object:
        # client.sessions is the SessionsNamespace the create posts through (it exposes ``_http``
        # — the authed httpx client — and ``_base``). Isolated here so a bump that renames it is a
        # one-line fix at the seam.
        return self.client.sessions  # type: ignore[attr-defined]

    async def aclose(self) -> None:
        """Close the standing client's HTTP pool on shutdown (the receiver owns the lifecycle).

        Idempotent: a double-call (e.g. a re-entered lifespan) is a no-op after the first close,
        so it cannot double-close the client pool.
        """
        if self._closed:
            return
        self._closed = True
        # OmnigentClient exposes async ``close()`` (NOT ``aclose``) — verified against installed
        # omnigent 0.2.0 (close() is the real method; aclose does not exist). Re-confirm on bump.
        await self.client.close()  # type: ignore[attr-defined]
