"""Pure AM-webhook model + injection-containment allowlist (PLT-715) — no omnigent import.

The AM→receiver edge's parse half, kept pure + unit-testable (mirrors ``engine.py`` /
``profile.py``): the omnigent-touching launch is the wrapper's job; this module holds
only the *parse + containment* policy, which is what must be provably bounded.

Two load-bearing transforms, both pure:

1. **Field-allowlist + framing** (INV-6 prompt-injection containment) — the alert becomes
   agent context, so :func:`extract_allowlisted` extracts ONLY a fixed set of fields, caps
   every string by length, caps the annotation summary's count, and frames the whole result
   in a ``<untrusted-data>`` boundary. The raw webhook is NEVER passed downstream: an
   attacker who controls an alert label/annotation controls agent input, so the blast radius
   is bounded to "a capped, framed, fixed-shape dict" rather than "arbitrary text".
2. **Deterministic incident key** — :func:`derive_incident_key` maps the AM groupKey (the
   field AM guarantees stable across re-fires of the same alert group) to the dedup /
   single-flight key. Webhook retries + duplicate alerts MUST collapse to the same key, or
   ``engine.admit_run``'s single-flight is voided.

Design 12 §2, §3.5; INV-6.
"""

from __future__ import annotations

from collections.abc import Mapping, Sequence
from dataclasses import dataclass

# --- containment caps (INV-6) --------------------------------------------------
# Every string the agent sees is capped here, not downstream. The numbers are
# blast-radius bounds, not display limits: an alert is attacker-influenceable
# (a label/annotation an alerting rule renders), so a hostile alert can inflate
# at most to these bounds before framing. Tune for context-window economy, not
# correctness — the framing + fixed-shape extraction is what contains injection.
_MAX_FIELD_LEN = 256  # a single allowlisted scalar (alertname/severity/namespace/chain_id)
_MAX_SUMMARY_LEN = 1024  # the concatenated annotation summary
_MAX_ANNOTATIONS = 8  # how many annotation entries fold into the summary

# The annotation keys folded into the summary, in order. Anything else is dropped:
# a new annotation key cannot reach the agent without being added here (allowlist,
# not denylist — the safe default is "not forwarded").
_ANNOTATION_KEYS: tuple[str, ...] = ("summary", "description", "runbook_url")

# The frame the agent's context renderer keys on. A fixed, non-attacker-controlled
# boundary so downstream prompt assembly can delimit "this block is untrusted alert
# text" without trusting any byte inside it.
_FRAME_OPEN = "<untrusted-data>"
_FRAME_CLOSE = "</untrusted-data>"


@dataclass(frozen=True)
class Alert:
    """One alert inside an AM webhook v4 payload (the bounded inbound model).

    Only the fields the receiver reads are modeled; AM may send more (``generatorURL``,
    ``fingerprint``, …) and they are intentionally dropped at parse — an unmodeled field
    is one fewer attacker-controlled byte reaching the agent. ``labels`` / ``annotations``
    are the attacker-influenceable surfaces; :func:`extract_allowlisted` is the only
    sanctioned reader of them.
    """

    status: str
    labels: Mapping[str, str]
    annotations: Mapping[str, str]
    starts_at: str


@dataclass(frozen=True)
class Webhook:
    """An AM webhook v4 payload (the bounded inbound model).

    ``group_key`` is AM's stable per-group identifier — load-bearing for dedup
    (:func:`derive_incident_key`). ``common_labels`` carries the group-level
    ``walle``/``severity``/``namespace`` labels; ``alerts`` are the individual firings.
    """

    version: str
    group_key: str
    status: str
    common_labels: Mapping[str, str]
    alerts: tuple[Alert, ...]


class WebhookError(ValueError):
    """An inbound payload is not a parseable AM webhook v4 body.

    Distinct from a *non-match* (a well-formed webhook that is not WallE-enrolled or
    not firing — the receiver answers those 200 + no-op): a ``WebhookError`` means the
    body is malformed. The receiver maps it to a 200 no-op too (an unparseable body is
    not retryable — a 5xx would only invite AM to re-deliver the same bad bytes), but
    the distinction is in the metric (``parse_error`` vs ``not_enrolled``).
    """


def _str(value: object) -> str:
    """Coerce a JSON scalar to ``str`` defensively — never raise on a hostile type.

    A label/annotation value SHOULD be a string, but the body is attacker-influenceable;
    a list/dict/number there is a containment concern, not a crash. Coerce to ``str`` so
    the cap + frame still apply (a malicious ``{"x": {...}}`` becomes a capped ``"{...}"``,
    contained, not propagated as structure).
    """
    return value if isinstance(value, str) else str(value)


def _cap(text: str, limit: int) -> str:
    """Length-cap a string to ``limit`` chars (INV-6). Slicing is the containment bound."""
    return text if len(text) <= limit else text[:limit]


def _mapping(value: object) -> Mapping[str, object]:
    return value if isinstance(value, Mapping) else {}


def parse_webhook(body: object) -> Webhook:
    """Parse a decoded AM webhook v4 JSON body into a :class:`Webhook`, or raise.

    Defensive against a hostile body by construction: a non-mapping body, a non-list
    ``alerts``, or a non-mapping alert entry raises :class:`WebhookError` rather than
    crashing the handler with a raw ``KeyError``/``TypeError`` (the inbound bytes are
    attacker-influenceable — see :class:`WebhookError`). Missing scalars default to
    ``""`` so a partial body parses into a well-formed-but-non-matching webhook (which
    the receiver answers 200 + no-op), never a 5xx.
    """
    if not isinstance(body, Mapping):
        raise WebhookError(f"AM webhook body must be a JSON object, got {type(body).__name__}")
    raw_alerts = body.get("alerts")
    if raw_alerts is not None and not isinstance(raw_alerts, Sequence):
        raise WebhookError(f"AM webhook 'alerts' must be a list, got {type(raw_alerts).__name__}")
    if isinstance(raw_alerts, (str, bytes)):
        raise WebhookError("AM webhook 'alerts' must be a list, got a string")

    alerts = tuple(
        _parse_alert(entry) for entry in (raw_alerts or ()) if isinstance(entry, Mapping)
    )
    return Webhook(
        version=_str(body.get("version", "")),
        group_key=_str(body.get("groupKey", "")),
        status=_str(body.get("status", "")),
        common_labels={k: _str(v) for k, v in _mapping(body.get("commonLabels")).items()},
        alerts=alerts,
    )


def _parse_alert(entry: Mapping[str, object]) -> Alert:
    return Alert(
        status=_str(entry.get("status", "")),
        labels={k: _str(v) for k, v in _mapping(entry.get("labels")).items()},
        annotations={k: _str(v) for k, v in _mapping(entry.get("annotations")).items()},
        starts_at=_str(entry.get("startsAt", "")),
    )


def is_enrolled(webhook: Webhook) -> bool:
    """True if this group is WallE-enrolled (a ``walle: enabled`` label + a runbook).

    Non-enrolled ⇒ the receiver answers 200 + no-op: the page already routed to the
    human, so a non-match is not an error. Enrollment is checked on the GROUP labels
    (``commonLabels``), the level at which an alerting rule opts a group into WallE — a
    per-alert opt-in is deliberately not supported in v1 (one knob, the group).
    """
    if webhook.common_labels.get("walle", "").strip().lower() != "enabled":
        return False
    # A runbook is required: WallE runs a runbook-driven investigation, so a group with
    # no runbook annotation has nothing to drive and must route to the human untouched.
    return any(alert.annotations.get("runbook_url", "").strip() for alert in webhook.alerts)


def is_firing(webhook: Webhook) -> bool:
    """True if the webhook is a ``firing`` notification (not a ``resolved`` one).

    A ``resolved`` webhook is a no-op for WallE: the incident cleared on its own, so
    there is nothing to investigate (the receiver answers 200 + no-op).
    """
    return webhook.status.strip().lower() == "firing"


def _summarize_annotations(alert: Alert) -> str:
    """Fold the allowlisted annotation keys into one capped, framed-elsewhere summary.

    Only ``_ANNOTATION_KEYS`` are read, in order, capped to ``_MAX_ANNOTATIONS`` entries
    and ``_MAX_SUMMARY_LEN`` chars total. A novel annotation key (an attacker adding
    ``annotations.injected: "ignore previous instructions…"``) is simply never read —
    the allowlist is the containment, the cap is the blast-radius bound.
    """
    parts: list[str] = []
    for key in _ANNOTATION_KEYS:
        value = alert.annotations.get(key, "").strip()
        if value:
            parts.append(f"{key}: {value}")
        if len(parts) >= _MAX_ANNOTATIONS:
            break
    return _cap(" | ".join(parts), _MAX_SUMMARY_LEN)


def extract_allowlisted(webhook: Webhook) -> Mapping[str, str]:
    """Extract a bounded, length-capped, ``<untrusted-data>``-framed dict (INV-6).

    The ONLY sanctioned path from an attacker-influenceable alert into agent context.
    Extracts a fixed field set — ``alertname`` / ``severity`` / ``namespace`` /
    ``chain_id`` / ``starts_at`` + a capped annotation summary — each capped to
    ``_MAX_FIELD_LEN`` (the summary to ``_MAX_SUMMARY_LEN``), then frames the summary in
    ``<untrusted-data>`` so downstream prompt assembly can delimit untrusted text without
    trusting any byte inside it. The raw webhook is NEVER forwarded: this fixed-shape,
    capped, framed dict is the entire blast radius an attacker controls.

    Reads the FIRST alert's labels/annotations falling back to the group ``commonLabels``
    (AM puts shared labels at the group level; per-alert labels override). An empty
    ``alerts`` list yields the group-level fields only — still well-formed.
    """
    first = webhook.alerts[0] if webhook.alerts else None
    labels: Mapping[str, str] = {
        **dict(webhook.common_labels),
        **(dict(first.labels) if first is not None else {}),
    }

    def label(name: str) -> str:
        return _cap(labels.get(name, "").strip(), _MAX_FIELD_LEN)

    summary = _summarize_annotations(first) if first is not None else ""
    starts_at = _cap(first.starts_at.strip(), _MAX_FIELD_LEN) if first is not None else ""

    return {
        "alertname": label("alertname"),
        "severity": label("severity"),
        "namespace": label("namespace"),
        "chain_id": label("chain_id"),
        "starts_at": starts_at,
        # The framed, capped summary — the only multi-line attacker-influenceable field.
        # Framing happens HERE (not at prompt assembly) so no downstream path can forget it.
        "annotation_summary": f"{_FRAME_OPEN}{summary}{_FRAME_CLOSE}",
    }


def derive_incident_key(webhook: Webhook) -> str:
    """Derive the deterministic dedup / single-flight key from the webhook (§2).

    The key MUST be stable across webhook retries and duplicate deliveries of the same
    alert group, or ``engine.admit_run``'s single-flight is voided (two triggers for one
    incident both ``PROCEED``). AM's ``groupKey`` is exactly that stable identifier — it
    is AM's own per-group dedup key, constant across re-fires of the group — so it is the
    key. Falls back to a deterministic compose of the matching labels (``alertname`` +
    ``namespace``) only when ``groupKey`` is absent (a malformed/old AM): a deterministic
    fallback still collapses retries, where an empty key would make every retry a fresh
    incident.
    """
    group_key = webhook.group_key.strip()
    if group_key:
        return group_key
    labels = dict(webhook.common_labels)
    if webhook.alerts:
        labels = {**labels, **dict(webhook.alerts[0].labels)}
    alertname = labels.get("alertname", "").strip()
    namespace = labels.get("namespace", "").strip()
    return f"fallback:{alertname}:{namespace}"
