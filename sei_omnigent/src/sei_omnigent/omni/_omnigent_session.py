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
    flow. (RESOLVED on installed 0.2.0: auth attaches to the CLIENT ctor — ``SessionsChat.create``
    takes no per-call ``headers`` — and ``OmnigentClient`` forwards ``headers``/``auth`` onto its
    underlying httpx client, so the bearer rotates via the Auth, read fresh per request.)
  - the ``SessionsChat.create`` namespace accessor + bundle source (see
    :class:`LiveSessionFactory` — the spike used ``client.sessions``; the 0.2.0 client exposes
    that attribute, but the per-investigation create + the agent-bundle provisioning are the
    open prove-the-run items);
  - that ``Response.usage`` is populated on terminal events (it is ``Usage | None`` → 0 delta
    when absent, which is safe — the run is then bounded by wall-clock/iterations, not tokens).

Design 12 §2, §3.2.
"""

from __future__ import annotations

import os
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
#: Env: path to the gzipped agent bundle the session runs (the ``/root-cause`` agent tarball,
#: mounted into the receiver pod). Read once at boot — the same bundle launches every run.
BUNDLE_PATH_ENV = "OMNI_RECEIVER_AGENT_BUNDLE"
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


class _LiveGoalSession:
    """A :class:`omni.driver.SessionLike` that lazily mints a real ``SessionsChat`` per run.

    ``SessionsChat.create`` is async but :meth:`stream` is the sync-returns-async-iterator the
    driver expects, so the chat is created on the FIRST stream iteration (inside the async
    generator), then the goal is posted and the turn's events are yielded. ``cancel()`` before
    the chat exists is a no-op (nothing to cancel); ``status`` reports ``pending`` until then.
    One investigation = one instance (no chat reuse across runs).
    """

    def __init__(self, namespace: object, bundle: bytes, goal: str) -> None:
        self._namespace = namespace
        self._bundle = bundle
        self._goal = goal
        self._chat: _ChatLike | None = None

    async def _events(self) -> AsyncIterator[object]:
        # VERIFY-ON-LIVE: SessionsChat.create(namespace, bundle) + the namespace accessor
        # (client.sessions) — live-coupled, not exercised by the unit suite (see module
        # docstring). The create is per-run so each investigation gets its own session.
        from omnigent_client._sessions_chat import SessionsChat  # noqa: PLC0415 -- impure seam

        # Auth attaches to the standing client (the X-Forwarded-Email principal on its static
        # header store + the per-request _FreshBearerAuth), NOT per-create: SessionsChat.create on
        # 0.2.0 takes no per-call headers. The FRESH-read SA bearer (S-CRIT) is read per REQUEST by
        # the Auth flow, so a kubelet-rotated token is current without rebuilding the client.
        chat = await SessionsChat.create(self._namespace, self._bundle)
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
    closed via :meth:`aclose` on shutdown) + the boot-loaded agent bundle. Calling it with a
    ``RunContext`` returns the receiver's ``SessionLaunch`` tuple: a lazy live session bound to
    ``ctx.goal`` + a FRESH extractor triple (the token-delta closure must not leak state across
    runs — see :func:`make_extractors`).

    Auth posture (S-CRIT; mirrors host_launch's B1): the kube-rbac-proxy sidecar fronting the
    server TokenReviews an ``Authorization: Bearer <SA token>``; ``X-Forwarded-Email`` is the app
    principal resolved WITHIN that authed channel (a spoofable claim on its own). Both attach to
    the STANDING client (built in :meth:`from_env`): the static X-Forwarded-Email on the client's
    header store, the bearer via :class:`_FreshBearerAuth` which re-reads the projected
    ``omnigent-api`` SA token file per REQUEST (the kubelet rotates it ~hourly — a boot-time token
    goes stale, so it is never baked once).

    VERIFY-ON-LIVE (the prove-the-run is still open; see module docstring): the ``client.sessions``
    namespace accessor + that the server/sidecar READ the forwarded auth (the ctor-forwarding onto
    the httpx client is verified against installed 0.2.0). Built to the best-known omnigent 0.2.0
    client API — NOT asserted end-to-end; the live run closes it.
    """

    #: an omnigent_client.OmnigentClient — typed object so this stays import-light. repr=False:
    #: the client carries the auth/header state in its repr; keep it off this dataclass's repr.
    client: object = field(repr=False)
    bundle: bytes = field(repr=False)  # bundle bytes are noise in a repr; off it
    _closed: bool = field(default=False, init=False, repr=False)

    @classmethod
    def from_env(cls) -> LiveSessionFactory:
        """Build the factory from env (server URL + identity + bundle + bearer file). Fail-loud.

        Mirrors the receiver's boot discipline: a missing server URL / identity / bundle / bearer
        file is a misconfiguration that must fail at boot, not on the first webhook (a half-
        configured factory would error every investigation). The bundle is read once here; the
        SA bearer is NOT read here (the client's :class:`_FreshBearerAuth` reads it fresh per
        request) — only its path is required.
        """
        server_url = (os.environ.get(SERVER_URL_ENV) or "").strip()
        forwarded_email = (os.environ.get(FORWARDED_EMAIL_ENV) or "").strip()
        bundle_path = (os.environ.get(BUNDLE_PATH_ENV) or "").strip()
        bearer_file = (os.environ.get(PROXY_BEARER_FILE_ENV) or "").strip()
        missing = [
            name
            for name, value in (
                (SERVER_URL_ENV, server_url),
                (FORWARDED_EMAIL_ENV, forwarded_email),
                (BUNDLE_PATH_ENV, bundle_path),
                (PROXY_BEARER_FILE_ENV, bearer_file),
            )
            if not value
        ]
        if missing:
            raise RuntimeError(
                f"LiveSessionFactory missing required env: {', '.join(missing)}. The receiver "
                "cannot launch an investigation without the server URL, the trusted identity, "
                "the agent bundle, and the projected SA bearer token file (fail-closed at boot)."
            )
        try:
            bundle = Path(bundle_path).read_bytes()
        except OSError as exc:
            raise RuntimeError(
                f"{BUNDLE_PATH_ENV}={bundle_path!r} is unreadable: {exc}. The agent bundle must "
                "be mounted and readable at boot."
            ) from exc

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
        return cls(client=client, bundle=bundle)

    def __call__(self, ctx: object) -> tuple[object, ...]:
        # ctx is an omni.receiver.RunContext (typed object so this module stays receiver-import-
        # light + the single-coupling seam carries no receiver dependency cycle). ctx.goal is the
        # contained, rendered investigation goal.
        session = _LiveGoalSession(self._namespace(), self.bundle, ctx.goal)
        return (session, *make_extractors())

    def _namespace(self) -> object:
        # VERIFY-ON-LIVE: client.sessions is the SessionsNamespace SessionsChat.create wants
        # (the 0.2.0 client exposes it; the spike used the same accessor). Isolated here so a
        # bump that renames it is a one-line fix at the seam.
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
