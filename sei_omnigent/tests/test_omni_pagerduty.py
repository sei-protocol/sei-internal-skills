"""Tests for the real PagerDutyPoster client (PLT-715 follow-up).

The PD HTTP is mocked with an ``httpx.MockTransport`` — a fake transport that routes each
request to a handler returning a canned ``httpx.Response``. No live PagerDuty, no network. The
client's clock + sleep + jitter-rand are injected so the timeout / retry / backoff logic is
provable with no real wall-clock sleeps and a deterministic backoff.

Coverage: find-by-key (found / not-found→skip), the note POST shape + the From/Authorization
headers, marker-idempotency (skip if a WallE-marked note exists), enrollment-verify (skip if
the incident's service is not enrolled), notes-only (a structural assertion that no act
endpoint is reachable from the module's source), fail-closed on not-found / not-enrolled, and
timeout + bounded retry with backoff.
"""

from __future__ import annotations

import ast
import asyncio
import io
import json as _json
import pathlib
import tokenize

import httpx
import pytest

from sei_omnigent.omni._pagerduty import (
    PagerDutyClient,
    PagerDutyError,
    _marker,
)

def _strip_docstrings_and_comments(source: str) -> str:
    """Return the module source with docstrings + comments removed, code string LITERALS kept.

    The INV-7 structural test scans EXECUTABLE surface only: the module's docstrings name the
    forbidden action verbs precisely to disclaim them, and that prose must not trip the grep —
    but a real action call (``json={"escalation_policy": ...}`` or a ``/resolve`` URL literal)
    is a CODE string literal that MUST still trip it. So this drops only docstring nodes (via
    AST, by line range) and comments (via tokenize), keeping every other string literal.
    """
    tree = ast.parse(source)
    docstring_lines: set[int] = set()
    for node in ast.walk(tree):
        if isinstance(node, (ast.Module, ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
            doc = ast.get_docstring(node, clean=False)
            if doc is None:
                continue
            expr = node.body[0]
            docstring_lines.update(range(expr.lineno, (expr.end_lineno or expr.lineno) + 1))
    kept_lines = [
        "" if (lineno + 1) in docstring_lines else line
        for lineno, line in enumerate(source.splitlines())
    ]
    without_docstrings = "\n".join(kept_lines)
    out: list[str] = []
    for tok in tokenize.generate_tokens(io.StringIO(without_docstrings).readline):
        if tok.type == tokenize.COMMENT:
            continue
        out.append(tok.string)
    return " ".join(out)


_TOKEN = "pd-notes-only-token"
_FROM = "walle@seinetwork.io"
_ENROLLED = "PSERVICE1"
_KEY = '{}:{alertname="ChainHalted"}'
_INCIDENT_ID = "PINCIDENT1"


def _incident(*, incident_id: str = _INCIDENT_ID, service_id: str = _ENROLLED) -> dict:
    return {"id": incident_id, "incident_key": _KEY, "service": {"id": service_id}}


class _Recorder:
    """Records every request the transport sees — the structural assertion surface."""

    def __init__(self) -> None:
        self.requests: list[httpx.Request] = []

    def record(self, request: httpx.Request) -> None:
        self.requests.append(request)

    def calls(self, method: str, path_contains: str) -> list[httpx.Request]:
        return [
            r
            for r in self.requests
            if r.method == method and path_contains in r.url.path
        ]


def _client(handler, recorder: _Recorder | None = None, **over) -> PagerDutyClient:
    rec = recorder or _Recorder()

    def _routed(request: httpx.Request) -> httpx.Response:
        rec.record(request)
        return handler(request)

    transport = httpx.MockTransport(_routed)
    http = httpx.AsyncClient(transport=transport, base_url="https://api.pagerduty.com")
    kwargs: dict = {
        "http": http,
        "from_email": _FROM,
        "enrolled_service_ids": frozenset({_ENROLLED}),
        "token": _TOKEN,
        # No real sleeps; deterministic backoff (rand=0 → sleep(0) on every retry).
        "sleep": _noop_sleep,
        "rand": lambda: 0.0,
        "now": lambda: 0.0,
    }
    kwargs.update(over)
    return PagerDutyClient(**kwargs)


async def _noop_sleep(_seconds: float) -> None:
    return None


def _find_response(incidents: list[dict]) -> httpx.Response:
    return httpx.Response(200, json={"incidents": incidents})


def _notes_response(notes: list[dict], *, more: bool = False) -> httpx.Response:
    return httpx.Response(200, json={"notes": notes, "more": more})


# --- find-by-key --------------------------------------------------------------


def test_find_queries_open_incident_by_dedup_key() -> None:
    rec = _Recorder()

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/incidents":
            return _find_response([_incident()])
        if request.url.path.endswith("/notes") and request.method == "GET":
            return _notes_response([])
        return httpx.Response(201, json={"note": {"id": "PNOTE1"}})

    client = _client(handler, rec)
    asyncio.run(client.post_note(_KEY, "findings"))

    find = rec.calls("GET", "/incidents")[0]
    assert find.url.params.get("incident_key") == _KEY
    # Filtered to OPEN statuses server-side (a resolved incident is never returned).
    assert set(find.url.params.get_list("statuses[]")) == {"triggered", "acknowledged"}


def test_no_open_incident_fails_closed_skip() -> None:
    # find returns no incident → the client posts NOTHING (do not create / post to nothing).
    rec = _Recorder()

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/incidents":
            return _find_response([])  # none open for this key
        return httpx.Response(201)

    client = _client(handler, rec)
    asyncio.run(client.post_note(_KEY, "findings"))
    assert rec.calls("POST", "/notes") == []  # nothing posted


# --- the note POST shape + headers --------------------------------------------


def test_note_post_shape_and_headers() -> None:
    rec = _Recorder()

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/incidents":
            return _find_response([_incident()])
        if request.url.path.endswith("/notes") and request.method == "GET":
            return _notes_response([])
        return httpx.Response(201, json={"note": {"id": "PNOTE1"}})

    client = _client(handler, rec)
    asyncio.run(client.post_note(_KEY, "the redacted findings"))

    post = rec.calls("POST", "/notes")[0]
    assert post.url.path == f"/incidents/{_INCIDENT_ID}/notes"
    body = _json.loads(post.content)
    assert "note" in body and "content" in body["note"]
    assert "the redacted findings" in body["note"]["content"]
    # The required PD headers: From (a valid requester email) + Token auth.
    assert post.headers["From"] == _FROM
    assert post.headers["Authorization"] == f"Token token={_TOKEN}"


def test_note_carries_the_walle_marker() -> None:
    rec = _Recorder()

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/incidents":
            return _find_response([_incident()])
        if request.url.path.endswith("/notes") and request.method == "GET":
            return _notes_response([])
        return httpx.Response(201)

    client = _client(handler, rec)
    asyncio.run(client.post_note(_KEY, "findings"))
    post = rec.calls("POST", "/notes")[0]
    content = _json.loads(post.content)["note"]["content"]
    assert _marker(_KEY) in content  # the idempotency anchor is embedded


# --- DEFINING REQ 1: marker idempotency ---------------------------------------


def test_skips_when_a_walle_marked_note_already_exists() -> None:
    # A re-post (retry OR restart re-run) is a no-op: the GET /notes finds the prior marker.
    rec = _Recorder()

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/incidents":
            return _find_response([_incident()])
        if request.url.path.endswith("/notes") and request.method == "GET":
            return _notes_response([{"content": f"earlier WallE note {_marker(_KEY)}"}])
        return httpx.Response(201)

    client = _client(handler, rec)
    asyncio.run(client.post_note(_KEY, "findings"))
    assert rec.calls("POST", "/notes") == []  # the marker already present → no second note


def test_marker_is_per_incident_so_another_incidents_note_does_not_suppress() -> None:
    # A marker for a DIFFERENT incident_key must not suppress this incident's post.
    rec = _Recorder()

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/incidents":
            return _find_response([_incident()])
        if request.url.path.endswith("/notes") and request.method == "GET":
            return _notes_response([{"content": f"note {_marker('a-different-incident')}"}])
        return httpx.Response(201)

    client = _client(handler, rec)
    asyncio.run(client.post_note(_KEY, "findings"))
    assert len(rec.calls("POST", "/notes")) == 1  # posts — the other marker does not match


# --- DEFINING REQ 2: enrollment verification ----------------------------------


def test_skips_when_incident_service_is_not_enrolled() -> None:
    # A forged groupKey mapping to a real-but-unenrolled incident must NOT be graffiti'd.
    rec = _Recorder()

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/incidents":
            return _find_response([_incident(service_id="POTHERSERVICE")])
        return httpx.Response(201)

    client = _client(handler, rec)
    asyncio.run(client.post_note(_KEY, "findings"))
    assert rec.calls("POST", "/notes") == []  # not enrolled → fail-closed, nothing posted
    # The notes-scan is not even reached (enrollment gate is before idempotency).
    assert rec.calls("GET", "/notes") == []


def test_posts_when_incident_service_is_enrolled() -> None:
    rec = _Recorder()

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/incidents":
            return _find_response([_incident(service_id=_ENROLLED)])
        if request.url.path.endswith("/notes") and request.method == "GET":
            return _notes_response([])
        return httpx.Response(201)

    client = _client(handler, rec)
    asyncio.run(client.post_note(_KEY, "findings"))
    assert len(rec.calls("POST", "/notes")) == 1


# --- reliability: timeout + bounded retry with backoff ------------------------


def test_retries_a_transient_5xx_then_succeeds() -> None:
    rec = _Recorder()
    state = {"find_attempts": 0}

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/incidents":
            state["find_attempts"] += 1
            if state["find_attempts"] <= 2:
                return httpx.Response(503)  # transient → retried
            return _find_response([_incident()])
        if request.url.path.endswith("/notes") and request.method == "GET":
            return _notes_response([])
        return httpx.Response(201)

    client = _client(handler, rec)
    asyncio.run(client.post_note(_KEY, "findings"))
    assert state["find_attempts"] == 3  # two 503s retried, third succeeded
    assert len(rec.calls("POST", "/notes")) == 1


def test_retries_a_transient_timeout() -> None:
    state = {"attempts": 0}

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/incidents":
            state["attempts"] += 1
            if state["attempts"] == 1:
                raise httpx.ReadTimeout("PD slow", request=request)
            return _find_response([_incident()])
        if request.url.path.endswith("/notes") and request.method == "GET":
            return _notes_response([])
        return httpx.Response(201)

    client = _client(handler)
    asyncio.run(client.post_note(_KEY, "findings"))
    assert state["attempts"] == 2  # the timeout was retried


def test_exhausts_retries_then_raises_pagerduty_error() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(503)  # always transient → exhaust retries

    client = _client(handler, max_retries=2)
    with pytest.raises(PagerDutyError):
        asyncio.run(client.post_note(_KEY, "findings"))


def test_does_not_retry_a_non_retryable_4xx() -> None:
    # A 403 (bad token / scope) is a client fault — retrying only repeats it. Raise at once.
    rec = _Recorder()

    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(403)

    client = _client(handler, rec, max_retries=5)
    with pytest.raises(PagerDutyError):
        asyncio.run(client.post_note(_KEY, "findings"))
    assert len(rec.calls("GET", "/incidents")) == 1  # no retries burned on a 4xx


def test_backoff_is_bounded_and_full_jitter() -> None:
    # rand=1.0 → the full ceiling; the ceiling is min(cap, base*2**n), so it never exceeds cap.
    client = _client(lambda r: httpx.Response(200, json={"incidents": []}),
                     rand=lambda: 1.0, backoff_base_s=0.5, backoff_max_s=8.0)
    assert client._backoff(0) == 0.5
    assert client._backoff(1) == 1.0
    assert client._backoff(2) == 2.0
    assert client._backoff(10) == 8.0  # capped


# --- INV-7: notes-only surface (structural) -----------------------------------


def test_module_surface_is_notes_only_no_act_endpoints() -> None:
    # The structural propose-only guarantee (INV-7): in the EXECUTABLE source (docstrings +
    # comments stripped, so prose that *names* the forbidden verbs to disclaim them does not
    # trip this), no PD action endpoint or action-body key appears. If an act endpoint is ever
    # added to the code, this fails loudly. Scoped to act PATHS (`/...`) + an action-body
    # marker, which prose does not form.
    source = pathlib.Path(
        pathlib.Path(__file__).parent.parent / "src/sei_omnigent/omni/_pagerduty.py"
    ).read_text(encoding="utf-8")
    code = _strip_docstrings_and_comments(source)
    forbidden = (
        "/resolve",
        "/acknowledge",
        "/escalate",
        "/reassign",
        "/snooze",
        "/responder_requests",
        "/merge",
        "escalation_policy",
        "automation_actions",
    )
    hits = [token for token in forbidden if token in code]
    assert hits == [], f"non-notes PD surface leaked into the module code: {hits}"
    # Positively: the only endpoint paths the code forms are the read + the comment endpoint.
    assert "/incidents" in code
    assert "/notes" in code


def test_only_get_and_post_notes_methods_issued_at_runtime() -> None:
    # Belt-and-suspenders to the source grep: at runtime the client issues ONLY GET /incidents,
    # GET .../notes, POST .../notes — never a PUT/DELETE or a non-notes POST.
    rec = _Recorder()

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/incidents":
            return _find_response([_incident()])
        if request.url.path.endswith("/notes") and request.method == "GET":
            return _notes_response([])
        return httpx.Response(201)

    client = _client(handler, rec)
    asyncio.run(client.post_note(_KEY, "findings"))
    for req in rec.requests:
        if req.method == "GET":
            assert req.url.path == "/incidents" or req.url.path.endswith("/notes")
        elif req.method == "POST":
            assert req.url.path.endswith("/notes")
        else:
            raise AssertionError(f"unexpected method {req.method} {req.url.path}")


# --- item 2: the non-idempotent POST is not blindly retried -------------------


def test_note_post_read_timeout_is_not_retried_no_double_post() -> None:
    # A POST that ReadTimeouts AFTER PD may have committed the note must NOT be re-issued — the
    # transport must see exactly ONE POST (a re-POST would be a double note).
    rec = _Recorder()

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/incidents":
            return _find_response([_incident()])
        if request.url.path.endswith("/notes") and request.method == "GET":
            return _notes_response([])
        # The POST: simulate a read-timeout after the bytes were sent.
        raise httpx.ReadTimeout("PD slow after commit", request=request)

    client = _client(handler, rec, max_retries=5)
    with pytest.raises(PagerDutyError):
        asyncio.run(client.post_note(_KEY, "findings"))
    assert len(rec.calls("POST", "/notes")) == 1  # exactly one POST — never re-issued


def test_note_post_retryable_status_is_not_retried() -> None:
    # A 503 on the POST: PD may have committed before erroring → do NOT re-POST (raise at once).
    rec = _Recorder()

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/incidents":
            return _find_response([_incident()])
        if request.url.path.endswith("/notes") and request.method == "GET":
            return _notes_response([])
        return httpx.Response(503)  # the POST

    client = _client(handler, rec, max_retries=5)
    with pytest.raises(PagerDutyError):
        asyncio.run(client.post_note(_KEY, "findings"))
    assert len(rec.calls("POST", "/notes")) == 1  # not retried on a retryable status either


def test_note_post_connect_error_is_retried() -> None:
    # A connect-phase error (bytes provably never sent) is the ONE retryable POST failure.
    rec = _Recorder()
    state = {"post_attempts": 0}

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/incidents":
            return _find_response([_incident()])
        if request.url.path.endswith("/notes") and request.method == "GET":
            return _notes_response([])
        state["post_attempts"] += 1
        if state["post_attempts"] == 1:
            raise httpx.ConnectError("connection refused", request=request)
        return httpx.Response(201)

    client = _client(handler, rec)
    asyncio.run(client.post_note(_KEY, "findings"))
    assert state["post_attempts"] == 2  # the connect-phase failure was retried then succeeded
    assert len(rec.calls("POST", "/notes")) == 2


# --- item 3: paginated marker-scan + fail-closed on the hard cap --------------


def test_marker_found_on_a_later_page_skips_the_post() -> None:
    # The marker scrolls onto page 2 (page 1 is full + `more`); the scan paginates and finds it.
    rec = _Recorder()
    fillers = [{"content": f"human note {i}"} for i in range(100)]

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/incidents":
            return _find_response([_incident()])
        if request.url.path.endswith("/notes") and request.method == "GET":
            offset = int(request.url.params.get("offset", "0"))
            if offset == 0:
                return _notes_response(fillers, more=True)
            return _notes_response([{"content": f"walle {_marker(_KEY)}"}], more=False)
        return httpx.Response(201)

    client = _client(handler, rec)
    asyncio.run(client.post_note(_KEY, "findings"))
    assert rec.calls("POST", "/notes") == []  # the marker on page 2 suppressed the post
    assert len(rec.calls("GET", "/notes")) == 2  # paginated past page 1


def test_scan_cap_hit_without_marker_fails_closed_skips_post() -> None:
    # Every page is full + signals `more` and never carries the marker → the cap is hit without
    # confirming absence → fail CLOSED (skip), never double-post on an unconfirmed scan.
    rec = _Recorder()
    full_page = [{"content": f"human note {i}"} for i in range(100)]

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/incidents":
            return _find_response([_incident()])
        if request.url.path.endswith("/notes") and request.method == "GET":
            return _notes_response(full_page, more=True)  # always more, never the marker
        return httpx.Response(201)

    client = _client(handler, rec)
    asyncio.run(client.post_note(_KEY, "findings"))
    assert rec.calls("POST", "/notes") == []  # cap hit, unconfirmed → fail-closed skip
    assert len(rec.calls("GET", "/notes")) == 5  # bounded at the page cap


def test_scan_exhausts_a_short_list_then_posts() -> None:
    # A short list (no `more`) is fully scanned without the marker → ABSENT → posts.
    rec = _Recorder()

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/incidents":
            return _find_response([_incident()])
        if request.url.path.endswith("/notes") and request.method == "GET":
            return _notes_response([{"content": "a human note"}], more=False)
        return httpx.Response(201)

    client = _client(handler, rec)
    asyncio.run(client.post_note(_KEY, "findings"))
    assert len(rec.calls("POST", "/notes")) == 1
    assert len(rec.calls("GET", "/notes")) == 1  # one page exhausted the list


# --- item 4: secrets are not in the repr --------------------------------------


def test_repr_does_not_leak_token_or_email() -> None:
    client = _client(lambda r: httpx.Response(200, json={"incidents": []}))
    text = repr(client)
    assert _TOKEN not in text
    assert _FROM not in text


# --- item 7: aclose closes the injected client --------------------------------


def test_aclose_closes_the_injected_client() -> None:
    client = _client(lambda r: httpx.Response(200, json={"incidents": []}))
    asyncio.run(client.aclose())
    assert client.http.is_closed


# --- items 6 + 10 + 11: from_config boot guards + path-id validation ----------


def _async_client() -> httpx.AsyncClient:
    return httpx.AsyncClient(transport=httpx.MockTransport(lambda r: httpx.Response(200)))


def test_from_config_rejects_empty_enrolled_set() -> None:
    with pytest.raises(ValueError, match="enrolled_service_ids"):
        PagerDutyClient.from_config(
            from_email=_FROM, enrolled_service_ids=[], token=_TOKEN, http=_async_client()
        )


def test_from_config_rejects_whitespace_only_enrolled_ids() -> None:
    with pytest.raises(ValueError, match="enrolled_service_ids"):
        PagerDutyClient.from_config(
            from_email=_FROM, enrolled_service_ids=["", "  "], token=_TOKEN, http=_async_client()
        )


def test_from_config_rejects_non_allowlisted_host() -> None:
    with pytest.raises(ValueError, match="not a PagerDuty endpoint"):
        PagerDutyClient.from_config(
            from_email=_FROM,
            enrolled_service_ids=[_ENROLLED],
            token=_TOKEN,
            base_url="https://evil.example.com",
            http=_async_client(),
        )


def test_from_config_accepts_the_eu_endpoint() -> None:
    client = PagerDutyClient.from_config(
        from_email=_FROM,
        enrolled_service_ids=[_ENROLLED],
        token=_TOKEN,
        base_url="https://api.eu.pagerduty.com",
        http=_async_client(),
    )
    assert client.base_url == "https://api.eu.pagerduty.com"


def test_malformed_incident_id_fails_closed_no_post() -> None:
    # A PD response whose incident id is not a plain alphanumeric id (path-injection shaped)
    # must fail closed — the id is never interpolated into a request path.
    rec = _Recorder()

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/incidents":
            return _find_response([_incident(incident_id="../resolve")])
        return httpx.Response(201)

    client = _client(handler, rec)
    asyncio.run(client.post_note(_KEY, "findings"))
    assert rec.calls("POST", "/notes") == []  # never built a request from the bad id
    assert rec.calls("GET", "/notes") == []
