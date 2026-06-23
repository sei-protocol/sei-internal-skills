"""The real :class:`PagerDutyPoster` for the omni-trigger receiver (PLT-715 follow-up).

The receiver core (``receiver.py``) depends on the ``PagerDutyPoster`` Protocol and ships a
``LoggingPoster`` stub; this module is the live client that funnels WallE's one terminal note
into PagerDuty. It is **propose-only / notes-only** (INV-7): the entire surface is *read an
incident* + *add a note*. There is NO resolve / ack / reassign / escalate / priority /
responder / automation call anywhere in this module — the egress is structurally a comment,
never an action. The PD token the manifest injects MUST be scoped to a notes-only role (the
operator's dedicated WallE PD user); the client never exercises a wider scope, but a
mis-scoped token is a privilege the client should not be handed.

The propose-only flow, in order — every step's failure mode is fail-closed:

1. **Find** the open incident by the AM-derived ``incident_key`` (= PD ``incident_key``):
   ``GET /incidents?incident_key=<key>&statuses[]=triggered&statuses[]=acknowledged``. No
   open incident → fail-closed (log + skip); WallE does NOT create or post to nothing.
2. **Verify enrollment** (DEFINING REQ 2 — closes groupKey-forge graffiti): the inbound
   bearer authenticates AM-to-receiver, but an attacker holding it could forge a ``groupKey``
   /dedup_key that maps to an *arbitrary* real PD incident. So before posting, confirm the
   found incident's PD ``service.id`` is in the manifest-injected WallE-enrolled set. Not
   enrolled → fail-closed (log + skip). The enrolled-set is the structural authorization
   boundary on what WallE may comment on, independent of who triggered the receiver.
3. **Idempotency** (DEFINING REQ 1 — closes the restart / double-launch double-propose):
   PD notes have NO native dedup and the notes endpoint exposes no ``Idempotency-Key`` header
   (verified against the PD REST API v2 surface — only the *Events API v2* trigger carries a
   ``dedup_key``; the REST notes endpoint does not), so the portable, restart-durable
   mechanism is an **app-side marker**: every WallE note carries a stable hidden
   ``[walle:run:<incident_key>]`` token, and before posting the client lists the incident's
   existing notes and SKIPS if a WallE-marked note for this ``incident_key`` already exists.
   A retry OR a process restart re-running ``post_note`` is then a no-op — the marker lives in
   PD (durable), not in the receiver's in-memory ``_posted`` (lost on restart). This is the
   load-bearing close; there is no native-header backstop to fall back to.
4. **Add note**: ``POST /incidents/{id}/notes`` with ``{"note": {"content": <marked,
   already-redacted text>}}``, the ``From: <WallE PD user email>`` header (PD requires a valid
   requester email on a note write) and ``Authorization: Token token=<PD_API_TOKEN>``.

Reliability (Design 12 §2; systems/reliability REL1, REL3, REL4): every PD call carries an
explicit timeout and a bounded retry with exponential backoff + full jitter on transient
failures (timeouts, connract errors, 429, 5xx) — never on a 4xx (a 401/403 is a token/scope
fault, not transient). The note POST is retry-SAFE because the marker-check makes a re-post a
no-op; a retry that lands after a partial first attempt finds the marker and skips. The http
client + the clock + the sleep are injected so the timeout / retry / idempotency logic is
provable against a fake transport with no live PagerDuty and no real wall-clock sleeps
(mirrors ``driver.py`` / ``_dedup.py``'s injected-clock discipline).

BOUNDARIES (the manifest injects, this module receives): ``PD_API_TOKEN`` (notes-only role),
the WallE PD user email (the ``From`` header), and the WallE-enrolled PD service-id set.

Design 12 §2, §3.5; INV-7; systems/reliability REL1/REL3/REL4, api-design API2.
"""

from __future__ import annotations

import asyncio
import logging
import random
import time
from collections.abc import Callable, Iterable, Mapping, Sequence
from dataclasses import dataclass

import httpx

_log = logging.getLogger("sei_omnigent.omni.pagerduty")

#: PD REST API v2 base. The manifest may override for a regional/EU endpoint.
_DEFAULT_BASE_URL = "https://api.pagerduty.com"

#: The open-incident statuses the find-by-key query filters to. A resolved incident is not a
#: live page — WallE has nothing to propose against a closed incident, so the find skips it.
_OPEN_STATUSES = ("triggered", "acknowledged")

#: Per-call deadline (connect + read). A PD call with no timeout is a coroutine we may never
#: get back (REL1); the post-back path is already off the AM webhook ack, but an un-bounded
#: PD call would still pin a bg task + a connection-pool slot indefinitely.
_DEFAULT_TIMEOUT_S = 10.0

#: Bounded retry on transient PD failures (REL3). Total attempts = 1 + this. Small: the note
#: is a best-effort proposal, not a critical write, and the marker-check makes each retry a
#: safe no-op-or-post, so a generous ceiling buys little and risks holding a bg task.
_DEFAULT_MAX_RETRIES = 3

#: Exponential-backoff base; the nth retry sleeps a random point in [0, base * 2**n] (full
#: jitter, REL4) so independent receivers re-firing after a PD blip do not resynchronize into
#: a thundering herd against a recovering PD.
_DEFAULT_BACKOFF_BASE_S = 0.5

#: Cap on a single backoff sleep — full jitter is unbounded in the exponent otherwise, and a
#: 30s+ sleep on a best-effort note serves no one (the bg task would rather give up).
_DEFAULT_BACKOFF_MAX_S = 8.0

#: How many of an incident's existing notes the idempotency check scans. PD returns newest
#: first; a WallE marker, if present, is among the most recent (WallE posts at most once per
#: incident and posts late in the incident's life). One page is the bound — a hostile/busy
#: incident with thousands of human notes cannot make the marker-check unbounded.
_NOTES_SCAN_LIMIT = 100

#: HTTP statuses worth retrying: rate-limit + server-side. A 4xx other than 429 is a client
#: fault (bad token, bad incident id, malformed body) that a retry only repeats (REL3).
_RETRYABLE_STATUSES = frozenset({429, 500, 502, 503, 504})


def _marker(incident_key: str) -> str:
    """The stable, per-incident WallE signature embedded in (and scanned for in) the note.

    Keyed on ``incident_key`` so the marker is identical across a retry and across a restart
    re-run of the SAME incident (the idempotency anchor), and distinct across incidents (a
    note for incident A never suppresses the note for incident B). Hidden-ish: a bracketed
    token a human skims past but an exact substring match finds deterministically.
    """
    return f"[walle:run:{incident_key}]"


class PagerDutyError(RuntimeError):
    """A PD call failed after exhausting retries, or returned a non-retryable error.

    Part of the client's interface (errors-are-interface): ``post_note`` raises this on an
    unrecoverable PD failure so the receiver's ``post_back`` chokepoint fails closed (logs +
    escalates, posts nothing) rather than swallowing a silent drop. A *fail-closed skip*
    (no open incident, not enrolled, already-marked) is NOT this — those return cleanly.
    """


@dataclass(frozen=True)
class PagerDutyClient:
    """Propose-only PagerDuty REST v2 client: find-incident-by-dedup-key + add a marked note.

    Implements the receiver's ``PagerDutyPoster`` Protocol (``post_note(incident_key, note)``).
    Fully dependency-injected — the ``httpx.AsyncClient``, the clock, and the sleep are passed
    so the find / verify / idempotency / retry logic is provable against a fake transport.

    Notes-only by construction: the only PD verbs this client issues are ``GET /incidents``,
    ``GET /incidents/{id}/notes``, and ``POST /incidents/{id}/notes``. No action endpoint is
    reachable from any method here (INV-7).
    """

    http: httpx.AsyncClient
    from_email: str
    #: The manifest-injected set of WallE-enrolled PD service-ids. An incident whose service
    #: is not in this set is NOT something WallE may comment on — fail-closed (DEFINING REQ 2).
    enrolled_service_ids: frozenset[str]
    token: str
    base_url: str = _DEFAULT_BASE_URL
    timeout_s: float = _DEFAULT_TIMEOUT_S
    max_retries: int = _DEFAULT_MAX_RETRIES
    backoff_base_s: float = _DEFAULT_BACKOFF_BASE_S
    backoff_max_s: float = _DEFAULT_BACKOFF_MAX_S
    #: Injected for testability — defaults to real monotonic time + real async sleep.
    now: Callable[[], float] = time.monotonic
    sleep: Callable[[float], object] = asyncio.sleep
    #: Injected randomness for the jitter — seeded in tests for a deterministic backoff.
    rand: Callable[[], float] = random.random

    @classmethod
    def from_config(
        cls,
        *,
        from_email: str,
        enrolled_service_ids: Iterable[str],
        token: str,
        base_url: str = _DEFAULT_BASE_URL,
        http: httpx.AsyncClient | None = None,
    ) -> PagerDutyClient:
        """Build a client from the manifest-injected config, minting an ``AsyncClient`` if needed.

        The injected ``http`` path is the test seam (a fake transport); the default path mints a
        keep-alive ``AsyncClient`` the caller is responsible for closing (the receiver owns its
        lifecycle alongside the host). ``enrolled_service_ids`` is frozen at construction —
        enrollment is boot config, not a per-request input.
        """
        client = http or httpx.AsyncClient()
        return cls(
            http=client,
            from_email=from_email,
            enrolled_service_ids=frozenset(enrolled_service_ids),
            token=token,
            base_url=base_url.rstrip("/"),
        )

    @property
    def _headers(self) -> dict[str, str]:
        # Token auth + the JSON Accept PD's v2 API requires; the From header is added only on
        # the write (PD requires a requester email on a note POST, not on a read).
        return {
            "Authorization": f"Token token={self.token}",
            "Accept": "application/vnd.pagerduty+json;version=2",
            "Content-Type": "application/json",
        }

    async def post_note(self, incident_key: str, note: str) -> None:
        """Find the WallE-enrolled open incident for ``incident_key`` and add ONE marked note.

        Fail-closed at every gate: no open incident → skip; incident's service not enrolled →
        skip; a WallE-marked note for this incident already exists → skip (idempotent). Only a
        clean find + enrolled + un-marked incident gets the note. ``note`` is the
        ALREADY-REDACTED text from the receiver's chokepoint — this client redacts nothing; it
        only prepends the idempotency marker before posting.

        Raises :class:`PagerDutyError` on an unrecoverable PD failure (so ``post_back`` fails
        closed); returns cleanly on every fail-closed skip (those are not errors).
        """
        incident = await self._find_open_incident(incident_key)
        if incident is None:
            _log.warning(
                "walle.pd.find.no_open_incident incident_key=%s — skipping (nothing to post to)",
                incident_key,
            )
            return

        incident_id = str(incident.get("id", ""))
        service_id = str(_mapping(incident.get("service")).get("id", ""))
        if not incident_id:
            _log.warning("walle.pd.find.no_incident_id incident_key=%s — skipping", incident_key)
            return

        if service_id not in self.enrolled_service_ids:
            # DEFINING REQ 2: the found incident is not on a WallE-enrolled service. A forged
            # groupKey matching a real-but-unenrolled incident dies here — WallE will not
            # graffiti an incident it was never enrolled to comment on.
            _log.warning(
                "walle.pd.enrollment.not_enrolled incident_key=%s incident_id=%s service_id=%s "
                "— skipping (forged-key / cross-service guard)",
                incident_key,
                incident_id,
                service_id,
            )
            return

        if await self._already_posted(incident_id, incident_key):
            # DEFINING REQ 1: a WallE-marked note already exists for this incident — a retry or
            # a restart re-run is a no-op. The marker lives in PD, so this survives a receiver
            # restart that lost the in-memory _posted set.
            _log.info(
                "walle.pd.idempotent.already_posted incident_key=%s incident_id=%s — skipping",
                incident_key,
                incident_id,
            )
            return

        content = f"{note}\n\n{_marker(incident_key)}"
        await self._add_note(incident_id, content)
        _log.info(
            "walle.pd.posted incident_key=%s incident_id=%s note_len=%d",
            incident_key,
            incident_id,
            len(content),
        )

    async def _find_open_incident(self, incident_key: str) -> Mapping[str, object] | None:
        """GET the open (triggered/acknowledged) incident for the PD ``incident_key``, or None.

        PD's ``incident_key`` is its own dedup key — the AM-derived key maps to it 1:1. The
        query filters to open statuses server-side so a resolved incident is never returned.
        Returns the first matching incident (an ``incident_key`` is unique among OPEN incidents
        in PD; a resolved one with the same key does not collide because we filter statuses).
        """
        params: list[tuple[str, str]] = [("incident_key", incident_key)]
        params.extend(("statuses[]", status) for status in _OPEN_STATUSES)
        resp = await self._request("GET", "/incidents", params=params)
        incidents = _sequence(_json(resp).get("incidents"))
        for incident in incidents:
            if isinstance(incident, Mapping):
                return incident
        return None

    async def _already_posted(self, incident_id: str, incident_key: str) -> bool:
        """True if a WallE-marked note for ``incident_key`` is already on the incident.

        The portable, restart-durable idempotency backstop (DEFINING REQ 1): scans up to
        ``_NOTES_SCAN_LIMIT`` of the incident's existing notes for the per-incident marker. PD
        has no native note dedup and the notes endpoint takes no idempotency header, so this
        marker-scan IS the dedup. Bounded scan — a busy incident cannot make it unbounded.
        """
        resp = await self._request(
            "GET", f"/incidents/{incident_id}/notes", params=[("limit", str(_NOTES_SCAN_LIMIT))]
        )
        marker = _marker(incident_key)
        for entry in _sequence(_json(resp).get("notes")):
            if isinstance(entry, Mapping) and marker in str(entry.get("content", "")):
                return True
        return False

    async def _add_note(self, incident_id: str, content: str) -> None:
        """POST the marked, already-redacted note to the incident. The ONLY write this client does.

        Notes-only (INV-7): the body is ``{"note": {"content": ...}}`` against the comment
        endpoint, with the ``From`` requester header. There is no path from here to an incident
        action (resolve/ack/escalate/...) — the URL is structurally ``/notes``.
        """
        headers = {**self._headers, "From": self.from_email}
        await self._request(
            "POST",
            f"/incidents/{incident_id}/notes",
            json={"note": {"content": content}},
            headers=headers,
        )

    async def _request(
        self,
        method: str,
        path: str,
        *,
        params: Sequence[tuple[str, str]] | None = None,
        json: Mapping[str, object] | None = None,
        headers: Mapping[str, str] | None = None,
    ) -> httpx.Response:
        """One PD call with an explicit timeout + bounded backoff-with-jitter retry.

        Retries only TRANSIENT failures — a connect/read timeout, a transport error, or a
        retryable status (429 + 5xx). A non-retryable 4xx (bad token/scope, bad incident id,
        malformed body) raises immediately: a retry only repeats it (REL3). Exhausting the
        retries raises :class:`PagerDutyError` so the chokepoint fails closed.
        """
        url = f"{self.base_url}{path}"
        request_headers = dict(headers) if headers is not None else self._headers
        attempt = 0
        last_exc: Exception | None = None
        while True:
            try:
                resp = await self.http.request(
                    method,
                    url,
                    params=list(params) if params is not None else None,
                    json=json,
                    headers=request_headers,
                    timeout=self.timeout_s,
                )
            except (httpx.TimeoutException, httpx.TransportError) as exc:
                # Transport-level transient: timeout / connection reset / DNS blip. Retryable.
                last_exc = exc
            else:
                if resp.status_code < 400:
                    return resp
                if resp.status_code not in _RETRYABLE_STATUSES:
                    # Non-retryable client fault — fail closed now, do not burn retries on it.
                    raise PagerDutyError(
                        f"PD {method} {path} returned non-retryable {resp.status_code}"
                    )
                last_exc = PagerDutyError(f"PD {method} {path} returned {resp.status_code}")

            if attempt >= self.max_retries:
                raise PagerDutyError(
                    f"PD {method} {path} failed after {attempt + 1} attempts: {last_exc}"
                ) from last_exc
            await self.sleep(self._backoff(attempt))
            attempt += 1

    def _backoff(self, attempt: int) -> float:
        """Full-jitter exponential backoff (REL4): a random point in [0, min(cap, base·2**n)].

        Full jitter (not equal jitter) so independent receivers do not resynchronize their
        retries into a herd against a recovering PD. Capped so a single sleep on a best-effort
        note stays bounded.
        """
        ceiling = min(self.backoff_max_s, self.backoff_base_s * (2**attempt))
        return self.rand() * ceiling


def _json(resp: httpx.Response) -> Mapping[str, object]:
    """Decode a PD JSON response defensively — a non-mapping body yields an empty mapping.

    PD's responses are attacker-adjacent only via a compromised PD, but a malformed/empty body
    must not crash the post-back path with a raw decode error; an empty mapping flows through
    the find/notes scans as "nothing found" (fail-closed), which is the safe default.
    """
    try:
        body = resp.json()
    except (ValueError, UnicodeDecodeError):
        return {}
    return body if isinstance(body, Mapping) else {}


def _sequence(value: object) -> Sequence[object]:
    """Coerce a JSON field to a list defensively — a non-list yields an empty list."""
    if isinstance(value, Sequence) and not isinstance(value, (str, bytes)):
        return value
    return ()


def _mapping(value: object) -> Mapping[str, object]:
    """Coerce a JSON field to a mapping defensively — a non-mapping yields an empty mapping."""
    return value if isinstance(value, Mapping) else {}
