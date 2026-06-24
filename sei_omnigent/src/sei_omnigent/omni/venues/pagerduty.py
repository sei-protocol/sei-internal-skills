"""The real :class:`Venue` for PagerDuty — Venue #1 (Design 13).

The router (``router.py``) depends on the ``Venue`` Protocol and ships a ``LoggingVenue``
stub; this module is the live client that funnels the one terminal note into PagerDuty. It is
**propose-only / notes-only** (INV-7): the entire surface is *read an incident* + *add a note*.
There is NO resolve / ack / reassign / escalate / priority / responder / automation call
anywhere in this module — the egress is structurally a comment, never an action. The PD token
the manifest injects MUST be scoped to a notes-only role (a dedicated PD user); the client
never exercises a wider scope, but a mis-scoped token is a privilege the client should not be
handed.

The propose-only flow, in order — every step's failure mode is fail-closed (``handle`` is the
PD incident key, the venue-handle the router routes a result to):

1. **Find** the open incident by the PD ``incident_key`` (= the AM-derived ``handle``):
   ``GET /incidents?incident_key=<key>&statuses[]=triggered&statuses[]=acknowledged``. No
   open incident → fail-closed (log + skip); the client does NOT create or post to nothing.
2. **Verify enrollment** (DEFINING REQ 2 — closes groupKey-forge graffiti): the inbound
   bearer authenticates the trigger edge, but an attacker holding it could forge a
   handle/dedup_key that maps to an *arbitrary* real PD incident. So before posting, confirm
   the found incident's PD ``service.id`` is in the manifest-injected enrolled set. Not
   enrolled → fail-closed (log + skip). The enrolled-set is the structural authorization
   boundary on what the client may comment on, independent of who triggered the router.
3. **Idempotency marker — best-effort, NOT atomic** (DEFINING REQ 1): PD notes have no native
   dedup and the notes endpoint exposes no conditional-create / ``Idempotency-Key`` header
   (only the *Events API v2* trigger carries a ``dedup_key``; the REST notes endpoint does
   not), so the portable, restart-durable mechanism is an **app-side marker**: every note
   carries a stable hidden ``[walle:run:<handle>]`` token (a durable PD-side wire format — the
   marker string is unchanged so it still matches notes already posted in production), and
   before posting the client paginates the incident's existing notes and SKIPS if a marked
   note for this ``handle`` already exists. The marker lives in PD (durable), not in the
   router's in-memory post-claim (lost on restart), so a retry or a restart re-run of
   ``post_result`` re-scans and skips. This **NARROWS** the double-propose window to PD's
   notes-list read-after-write propagation lag — two posters that both scan before either's
   note is visible can both post. RESIDUAL: a true close needs a durable cross-process claim
   store (the multi-replica un-defer); the marker is the single-replica best-effort guard, not
   an atomic once-guarantee.
4. **Add note**: ``POST /incidents/{id}/notes`` with ``{"note": {"content": <marked,
   already-redacted text>}}``, the ``From: <PD user email>`` header (PD requires a valid
   requester email on a note write) and ``Authorization: Token token=<PD_API_TOKEN>``.

Reliability (Design 12 §2; systems/reliability REL1, REL3, REL4): every PD call carries an
explicit timeout. The idempotent READS (the find GET, the notes-scan GET) carry a bounded
retry with exponential backoff + full jitter on transient failures (timeouts, transport
errors, 429, 5xx) — never on a 4xx (a 401/403 is a token/scope fault, not transient). The
note POST is NOT idempotent (no conditional-create) and a timeout after PD committed the note
cannot be distinguished from a non-delivery, so it is NOT blindly retried: only a
connect-phase error that guarantees the bytes were never sent is retried; any other POST
failure raises and the next invocation's marker-scan dedups (best-effort, per the residual
above). The http client and the sleep are injected so the timeout / retry /
idempotency logic is provable against a fake transport with no live PagerDuty and no real
wall-clock sleeps (mirrors ``driver.py`` / ``_dedup.py``'s injected-clock discipline).

BOUNDARIES (the manifest injects, this module receives): ``PD_API_TOKEN`` (notes-only role),
the PD user email (the ``From`` header), and the enrolled PD service-id set.

Design 13 (Venue #1); Design 12 §2, §3.5; INV-7; systems/reliability REL1/REL3/REL4,
api-design API2.
"""

from __future__ import annotations

import asyncio
import enum
import logging
import random
import re
from collections.abc import Callable, Iterable, Mapping, Sequence
from dataclasses import dataclass, field
from urllib.parse import urlsplit

import httpx

from sei_omnigent.omni.venues.base import VenueHandle

_log = logging.getLogger("sei_omnigent.omni.pagerduty")

#: PD REST API v2 base. The manifest may override for a regional/EU endpoint.
_DEFAULT_BASE_URL = "https://api.pagerduty.com"

#: The only hosts a manifest-injected base_url may point at. Bounds a tampered manifest from
#: redirecting the token-bearing requests at an attacker-controlled host (token exfiltration).
_ALLOWED_HOSTS = frozenset({"api.pagerduty.com", "api.eu.pagerduty.com"})

#: PD object ids are alphanumeric (e.g. ``PINCIDENT1``). Validated before an id is
#: interpolated into a request path — defense-in-depth so the notes-only surface holds even
#: against a compromised-PD response-injection (a crafted ``id`` cannot escape the path).
#: ``\A..\Z`` (not ``^..$``) so a trailing newline cannot slip past the anchor.
_PD_ID_RE = re.compile(r"\A[A-Z0-9]+\Z")

#: The open-incident statuses the find-by-key query filters to. A resolved incident is not a
#: live page — there is nothing to propose against a closed incident, so the find skips it.
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

#: Per-page size for the notes-scan. PD returns newest first; the scan follows ``more``/offset
#: pages until the marker is found or the list is exhausted or the page cap is hit.
_NOTES_PAGE_LIMIT = 100

#: Hard cap on the notes-scan pagination. A hostile/busy incident with thousands of human
#: notes cannot make the marker-check unbounded; at the cap WITHOUT having found the marker or
#: exhausted the list, the scan cannot confirm "not already posted" → fail-CLOSED (skip the
#: post), never double-post on an unconfirmed scan.
_NOTES_SCAN_MAX_PAGES = 5

#: HTTP statuses worth retrying: rate-limit + server-side. A 4xx other than 429 is a client
#: fault (bad token, bad incident id, malformed body) that a retry only repeats (REL3).
_RETRYABLE_STATUSES = frozenset({429, 500, 502, 503, 504})


class _MarkerScan(enum.Enum):
    """The outcome of the notes-scan idempotency check.

    ``FOUND`` — a marker for this incident is present (skip the post). ``ABSENT`` — the notes
    list was fully scanned (or exhausted) and no marker was found (proceed to post).
    ``UNCONFIRMED`` — the page cap was hit before the list was exhausted and before a marker
    was found, so absence cannot be confirmed (fail-closed: skip, do NOT double-post).
    """

    FOUND = enum.auto()
    ABSENT = enum.auto()
    UNCONFIRMED = enum.auto()


def _marker(incident_key: str) -> str:
    """The stable, per-incident signature embedded in (and scanned for in) the note.

    Keyed on ``incident_key`` so the marker is identical across a retry and across a restart
    re-run of the SAME incident (the idempotency anchor), and distinct across incidents (a
    note for incident A never suppresses the note for incident B). Hidden-ish: a bracketed
    token a human skims past but an exact substring match finds deterministically. The literal
    ``walle:run`` prefix is a DURABLE PD-side wire format — it is unchanged through the
    de-personation so the scan still matches notes already posted in production.
    """
    return f"[walle:run:{incident_key}]"


class PagerDutyError(RuntimeError):
    """A PD call failed after exhausting retries, or returned a non-retryable error.

    Part of the client's interface (errors-are-interface): ``post_result`` raises this on an
    unrecoverable PD failure so the router's ``post_back`` chokepoint fails closed (logs +
    escalates, posts nothing) rather than swallowing a silent drop. A *fail-closed skip*
    (no open incident, not enrolled, already-marked) is NOT this — those return cleanly.
    """


@dataclass(frozen=True)
class PagerDutyClient:
    """Propose-only PagerDuty REST v2 client: find-incident-by-dedup-key + add a marked note.

    Implements the router's ``Venue`` Protocol (``post_result(handle, body)``, with ``handle``
    the PD incident key). Fully dependency-injected — the ``httpx.AsyncClient``, the sleep, and
    the jitter source are passed so the find / verify / idempotency / retry logic is provable
    against a fake transport.

    Notes-only by construction: the only PD verbs this client issues are ``GET /incidents``,
    ``GET /incidents/{id}/notes``, and ``POST /incidents/{id}/notes``. No action endpoint is
    reachable from any method here (INV-7).
    """

    http: httpx.AsyncClient
    #: repr=False: the From header is the PD user email — keep it out of logs/tracebacks.
    from_email: str = field(repr=False)
    #: The manifest-injected set of enrolled PD service-ids. An incident whose service is not
    #: in this set is NOT something the client may comment on — fail-closed (DEFINING REQ 2).
    enrolled_service_ids: frozenset[str] = field(default=frozenset())
    #: repr=False: the PD API token is a secret — never let it surface in a repr/log/traceback.
    token: str = field(default="", repr=False)
    base_url: str = _DEFAULT_BASE_URL
    timeout_s: float = _DEFAULT_TIMEOUT_S
    max_retries: int = _DEFAULT_MAX_RETRIES
    backoff_base_s: float = _DEFAULT_BACKOFF_BASE_S
    backoff_max_s: float = _DEFAULT_BACKOFF_MAX_S
    #: Injected for testability — defaults to a real async sleep (a no-op in tests).
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
        keep-alive ``AsyncClient`` the caller is responsible for closing (the router owns its
        lifecycle alongside the host). ``enrolled_service_ids`` is frozen at construction —
        enrollment is boot config, not a per-request input.

        Fails CLOSED at boot on a misconfiguration: an empty enrolled set (a silent deny-all
        that would make the venue a no-op outage), or a ``base_url`` whose host is not on the PD
        allowlist (a tampered manifest must not redirect the token-bearing requests off-PD).
        """
        normalized_base = base_url.rstrip("/")
        split = urlsplit(normalized_base)
        if split.scheme != "https":
            raise ValueError(
                f"base_url scheme {split.scheme!r} is not https; refusing to send the PD "
                "token over cleartext (a passive MITM would capture it)."
            )
        host = (split.hostname or "").lower()
        if host not in _ALLOWED_HOSTS:
            raise ValueError(
                f"base_url host {host!r} is not a PagerDuty endpoint "
                f"(allowed: {sorted(_ALLOWED_HOSTS)}); refusing to send the PD token off-PD."
            )
        enrolled = frozenset(s for s in enrolled_service_ids if s and s.strip())
        if not enrolled:
            raise ValueError(
                "enrolled_service_ids is empty: the venue would have nothing to comment on (a "
                "silent deny-all outage). Enroll at least one PD service-id at boot."
            )
        client = http or httpx.AsyncClient()
        return cls(
            http=client,
            from_email=from_email,
            enrolled_service_ids=enrolled,
            token=token,
            base_url=normalized_base,
        )

    async def aclose(self) -> None:
        """Close the underlying ``AsyncClient`` (release the connection pool).

        Called from the router's lifespan shutdown so a minted keep-alive pool is not leaked
        on a clean stop. Idempotent — closing an already-closed client is a no-op in httpx.
        """
        await self.http.aclose()

    @property
    def _headers(self) -> dict[str, str]:
        # Token auth + the JSON Accept PD's v2 API requires; the From header is added only on
        # the write (PD requires a requester email on a note POST, not on a read).
        return {
            "Authorization": f"Token token={self.token}",
            "Accept": "application/vnd.pagerduty+json;version=2",
            "Content-Type": "application/json",
        }

    async def post_result(self, handle: VenueHandle, body: str) -> bool:
        """Find the enrolled open incident for ``handle`` (the PD incident key) and add ONE note.

        Implements ``Venue.post_result``. Returns ``True`` iff a note was actually written;
        ``False`` on every fail-closed skip. The caller (``post_back``) keeps its dedup slot only
        on ``True`` — a skip releases it so a later re-fire re-evaluates (the situation may have
        changed: an incident opened, or a chatty incident's notes settled). ``False`` never risks
        a double-post: the PD marker is the durable dedup, so a re-fire after a real post re-scans
        and skips.

        Fail-closed at every gate (each returns ``False``): no open incident → skip; the id is
        not a plain PD id → skip; the incident's service is not enrolled → skip; a marked note
        already exists → skip (idempotent); the notes-scan is UNCONFIRMED (cap hit) → skip. Only
        a clean find + enrolled + un-marked incident gets the note. ``body`` is the
        ALREADY-REDACTED text from the router's chokepoint — this client redacts nothing; it
        only prepends the idempotency marker before posting.

        Raises :class:`PagerDutyError` on an unrecoverable PD failure (so ``post_back`` fails
        closed); returns cleanly on every fail-closed skip (those are not errors).
        """
        incident = await self._find_open_incident(handle)
        if incident is None:
            _log.warning(
                "omni.pd.find.no_open_incident handle=%s — skipping (nothing to post to)",
                handle,
            )
            return False

        incident_id = str(incident.get("id", ""))
        service_id = str(_mapping(incident.get("service")).get("id", ""))
        if not _PD_ID_RE.match(incident_id):
            # The id is interpolated into the notes path; a value that is not a plain PD id
            # (empty, or carrying path/query characters) could only come from a malformed or
            # compromised PD response. Fail-closed — never build a request from it.
            _log.warning(
                "omni.pd.find.bad_incident_id handle=%s incident_id=%r — skipping",
                handle,
                incident_id,
            )
            return False

        if service_id not in self.enrolled_service_ids:
            # DEFINING REQ 2: the found incident is not on an enrolled service. A forged
            # groupKey matching a real-but-unenrolled incident dies here — the client will not
            # graffiti an incident it was never enrolled to comment on.
            _log.warning(
                "omni.pd.enrollment.not_enrolled handle=%s incident_id=%s service_id=%s "
                "— skipping (forged-key / cross-service guard)",
                handle,
                incident_id,
                service_id,
            )
            return False

        scan = await self._scan_for_marker(incident_id, handle)
        if scan is _MarkerScan.FOUND:
            # DEFINING REQ 1: a marked note already exists for this incident — a retry or a
            # restart re-run is a no-op. The marker lives in PD, so this survives a process
            # restart that lost the in-memory post-claim.
            _log.info(
                "omni.pd.idempotent.already_posted handle=%s incident_id=%s — skipping",
                handle,
                incident_id,
            )
            return False
        if scan is _MarkerScan.UNCONFIRMED:
            # The notes-scan hit the page cap without finding the marker AND without exhausting
            # the list — we cannot confirm a prior note is absent. Fail CLOSED: skip rather
            # than risk a double-post on an over-long incident.
            _log.warning(
                "omni.pd.idempotent.scan_capped handle=%s incident_id=%s pages=%d "
                "— cannot confirm not-already-posted, skipping (fail-closed)",
                handle,
                incident_id,
                _NOTES_SCAN_MAX_PAGES,
            )
            return False

        content = f"{body}\n\n{_marker(handle)}"
        await self._add_note(incident_id, content)
        _log.info(
            "omni.pd.posted handle=%s incident_id=%s note_len=%d",
            handle,
            incident_id,
            len(content),
        )
        return True

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

    async def _scan_for_marker(self, incident_id: str, incident_key: str) -> _MarkerScan:
        """Paginate the incident's notes for the per-incident marker (DEFINING REQ 1).

        The portable, restart-durable best-effort idempotency check (NOT atomic — see module
        docstring): PD has no native note dedup, so this marker-scan IS the guard. Follows PD's
        ``more``/offset pages until the marker is FOUND, the list is exhausted (ABSENT), or the
        page cap is hit. Hitting the cap before exhausting the list returns UNCONFIRMED — the
        caller fails CLOSED (skips) rather than risk a double-post on an over-long incident.
        """
        marker = _marker(incident_key)
        offset = 0
        for _page in range(_NOTES_SCAN_MAX_PAGES):
            resp = await self._request(
                "GET",
                f"/incidents/{incident_id}/notes",
                params=[("limit", str(_NOTES_PAGE_LIMIT)), ("offset", str(offset))],
            )
            body = _json(resp)
            notes = _sequence(body.get("notes"))
            for entry in notes:
                if isinstance(entry, Mapping) and marker in str(entry.get("content", "")):
                    return _MarkerScan.FOUND
            if not _truthy(body.get("more")):
                # PD signals no further pages → the list is exhausted and the marker is absent.
                return _MarkerScan.ABSENT
            if not notes:
                # An empty page while ``more`` is still true: we cannot advance the offset (it
                # would not move) and cannot confirm absence — a marker may sit on a later page.
                # Fail CLOSED (UNCONFIRMED → the caller skips) rather than treat it as exhausted.
                return _MarkerScan.UNCONFIRMED
            offset += len(notes)
        return _MarkerScan.UNCONFIRMED

    async def _add_note(self, incident_id: str, content: str) -> None:
        """POST the marked, already-redacted note to the incident. The ONLY write this client does.

        Notes-only (INV-7): the body is ``{"note": {"content": ...}}`` against the comment
        endpoint, with the ``From`` requester header. There is no path from here to an incident
        action (resolve/ack/escalate/...) — the URL is structurally ``/notes``.

        The POST is issued with ``idempotent=False``: a POST that times out (or fails on a
        retryable status) AFTER PD may have committed the note must NOT be re-issued, or a
        single terminal becomes two notes. Only a connect-phase error (bytes provably never
        sent) is retried; any other failure raises and the next invocation's marker-scan
        dedups (best-effort, per the module docstring's residual).
        """
        headers = {**self._headers, "From": self.from_email}
        await self._request(
            "POST",
            f"/incidents/{incident_id}/notes",
            json={"note": {"content": content}},
            headers=headers,
            idempotent=False,
        )

    async def _request(
        self,
        method: str,
        path: str,
        *,
        params: Sequence[tuple[str, str]] | None = None,
        json: Mapping[str, object] | None = None,
        headers: Mapping[str, str] | None = None,
        idempotent: bool = True,
    ) -> httpx.Response:
        """One PD call with an explicit timeout + bounded backoff-with-jitter retry.

        For an IDEMPOTENT read (the find / notes-scan GETs) retries every TRANSIENT failure — a
        connect/read timeout, a transport error, or a retryable status (429 + 5xx). For a
        NON-idempotent write (the note POST, ``idempotent=False``) retries ONLY a connect-phase
        error (``httpx.ConnectError`` / ``ConnectTimeout`` — the bytes were never sent, so a
        re-issue cannot double-write); a read-timeout or a retryable status is NOT retried (the
        write may already have landed) and raises at once. A non-retryable 4xx (bad token/scope,
        bad incident id, malformed body) always raises immediately. Exhausting the retries raises
        :class:`PagerDutyError` so the chokepoint fails closed.
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
            except (httpx.ConnectError, httpx.ConnectTimeout) as exc:
                # Connect-phase: the request bytes were never put on the wire, so a re-issue
                # cannot double-write — retryable even for the non-idempotent POST.
                last_exc = exc
            except (httpx.TimeoutException, httpx.TransportError) as exc:
                # Post-connect transient (read timeout / reset mid-flight). Retryable for an
                # idempotent read; for a write the bytes may already have landed → do NOT retry.
                if not idempotent:
                    raise PagerDutyError(
                        f"PD {method} {path} failed mid-flight ({type(exc).__name__}); not "
                        "retrying a non-idempotent write (the note may already be committed)"
                    ) from exc
                last_exc = exc
            else:
                if resp.status_code < 400:
                    return resp
                if resp.status_code not in _RETRYABLE_STATUSES:
                    # Non-retryable client fault — fail closed now, do not burn retries on it.
                    raise PagerDutyError(
                        f"PD {method} {path} returned non-retryable {resp.status_code}"
                    )
                if not idempotent:
                    # A retryable status on a write: PD may have committed before erroring →
                    # do NOT re-POST. Raise; the next run's marker-scan dedups (best-effort).
                    raise PagerDutyError(
                        f"PD {method} {path} returned {resp.status_code}; not retrying a "
                        "non-idempotent write (the note may already be committed)"
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
    if isinstance(value, Sequence) and not isinstance(value, str | bytes):
        return value
    return ()


def _mapping(value: object) -> Mapping[str, object]:
    """Coerce a JSON field to a mapping defensively — a non-mapping yields an empty mapping."""
    return value if isinstance(value, Mapping) else {}


def _truthy(value: object) -> bool:
    """Coerce PD's ``more`` paging flag defensively — only a real boolean True means more."""
    return value is True
