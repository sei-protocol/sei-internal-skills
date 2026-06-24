"""The Alertmanager trigger adapter: webhook model + injection containment (Design 13).

The first :class:`~sei_omnigent.omni.adapters.base.TriggerAdapter` — the AM webhook ingress.
Its containment discipline (allowlist + neutralize + ``<untrusted-data>`` frame + caps) is the
TEMPLATE every adapter must follow (INV-6). No omnigent import — pure + unit-testable (mirrors
``engine.py`` / ``profile.py``): the omnigent-touching launch is the router's job; this module
holds only the *parse + containment + normalize* policy, which is what must be provably bounded.

Three load-bearing transforms, all pure:

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
3. **Normalize** — :meth:`AlertmanagerAdapter.parse` wraps the parse → is_firing/is_enrolled →
   extract_allowlisted → derive_incident_key chain into a venue-agnostic
   :class:`~sei_omnigent.omni.adapters.base.NormalizedTrigger` (or a :class:`NoOp` carrying the
   reason for a non-matching event). The AM path's initiator is a SYSTEM identity (no human — that
   is the Slack slice's concern); ``venue_handle`` and ``dedup_key`` are both the incident key.

Design 13 (TriggerAdapter #1); Design 12 §2, §3.5; INV-6.
"""

from __future__ import annotations

import itertools
import logging
from collections.abc import Mapping, Sequence
from dataclasses import dataclass

from sei_omnigent.omni.adapters.base import NoOp, NormalizedTrigger

_log = logging.getLogger("sei_omnigent.omni.adapters.alertmanager")

# --- containment caps (INV-6) --------------------------------------------------
# Every string the agent sees is capped here, not downstream. The numbers are
# blast-radius bounds, not display limits: an alert is attacker-influenceable
# (a label/annotation an alerting rule renders), so a hostile alert can inflate
# at most to these bounds before framing. Tune for context-window economy, not
# correctness — the framing + fixed-shape extraction is what contains injection.
_MAX_FIELD_LEN = 256  # a single allowlisted scalar (alertname/severity/namespace/chain_id)
_MAX_SUMMARY_LEN = 1024  # the concatenated annotation summary
_MAX_ANNOTATIONS = 8  # how many annotation entries fold into the summary
# How many alert entries we parse from one webhook. extract_allowlisted reads only the
# first alert, so a hostile body with thousands of entries would otherwise burn parse
# CPU + memory building Alert objects no one reads. Capped at parse, before per-entry work.
_MAX_ALERTS = 64

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

    Distinct from a *non-match* (a well-formed webhook that is not enrolled or not
    firing): a ``WebhookError`` means the body is malformed. Both normalize to a ``NoOp``
    (an unparseable body is not retryable — a 5xx would only invite AM to re-deliver the same
    bad bytes), but to DISTINCT reasons: the parse-error path carries ``NoOp("parse_error")``
    and the non-match path ``NoOp("not_enrolled")``, so a malformed-body storm and a benign
    not-enrolled flood stay separable on-call signals rather than one undifferentiated no-op.
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


def _neutralize(text: str) -> str:
    """Escape angle brackets so an untrusted value cannot FORGE the ``<untrusted-data>`` frame.

    Without this, an annotation value containing a literal ``</untrusted-data>`` could close
    the frame early and smuggle the bytes after it out of the contained block — the framing
    would be defeated by its own delimiter. Escaping ``<``/``>`` to ``&lt;``/``&gt;`` makes the
    frame delimiters the ONLY real angle brackets in the rendered block. Applied AFTER the
    length-cap (escaping expands a char up to 4×, so capping the input first keeps it bounded;
    the output is bounded at 4× the cap, still finite).
    """
    return text.replace("<", "&lt;").replace(">", "&gt;")


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

    # Cap the parsed entries (_MAX_ALERTS) before building any Alert: a hostile body cannot
    # make us materialize an unbounded list of objects only the first of which is ever read.
    capped = itertools.islice(raw_alerts or (), _MAX_ALERTS)
    alerts = tuple(_parse_alert(entry) for entry in capped if isinstance(entry, Mapping))
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
    """True if this group is enrolled (a ``walle: enabled`` label + a runbook).

    Non-enrolled ⇒ the adapter normalizes to ``NoOp``: the page already routed to the
    human, so a non-match is not an error. Enrollment is checked on the GROUP labels
    (``commonLabels``), the level at which an alerting rule opts a group in — a per-alert
    opt-in is deliberately not supported in v1 (one knob, the group). The ``walle: enabled``
    label key is a DURABLE AM-webhook data contract (alerting rules set it), kept verbatim.
    """
    if webhook.common_labels.get("walle", "").strip().lower() != "enabled":
        return False
    # A runbook is required: the investigation is runbook-driven, so a group with no runbook
    # annotation has nothing to drive and must route to the human untouched.
    return any(alert.annotations.get("runbook_url", "").strip() for alert in webhook.alerts)


def is_firing(webhook: Webhook) -> bool:
    """True if the webhook is a ``firing`` notification (not a ``resolved`` one).

    A ``resolved`` webhook is a no-op: the incident cleared on its own, so there is nothing
    to investigate (the adapter normalizes to ``NoOp``).
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
    return _neutralize(_cap(" | ".join(parts), _MAX_SUMMARY_LEN))


def extract_allowlisted(webhook: Webhook) -> Mapping[str, str]:
    """Extract a bounded, length-capped, ``<untrusted-data>``-framed dict (INV-6).

    The ONLY sanctioned path from an attacker-influenceable alert into agent context.
    Extracts a fixed field set — ``alertname`` / ``severity`` / ``namespace`` /
    ``chain_id`` / ``starts_at`` + a capped annotation summary — each capped to
    ``_MAX_FIELD_LEN`` (the summary to ``_MAX_SUMMARY_LEN``), angle-bracket-escaped (so no
    value can FORGE the frame delimiter — :func:`_neutralize`), then frames the summary in
    ``<untrusted-data>`` so downstream prompt assembly can delimit untrusted text without
    trusting any byte inside it. The raw webhook is NEVER forwarded: this fixed-shape,
    capped, escaped, framed dict is the entire blast radius an attacker controls.

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
        return _neutralize(_cap(labels.get(name, "").strip(), _MAX_FIELD_LEN))

    summary = _summarize_annotations(first) if first is not None else ""
    starts_at = (
        _neutralize(_cap(first.starts_at.strip(), _MAX_FIELD_LEN)) if first is not None else ""
    )

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


# --- the TriggerAdapter implementation (the normalize step) --------------------

#: The system initiator for the AM path. The AM edge carries no human principal — the trigger
#: is a machine/alerting-rule firing, so the initiator is this fixed system identity. The
#: per-human-initiator principal is the Slack slice's concern (gated on the Blocking dependency).
_AM_SYSTEM_INITIATOR = "system:alertmanager"

#: The trust class the ControlPlane keys on for the AM path. A machine-origin trigger.
_AM_TRUST = "system"

#: The default goal template — the trusted (manifest-injected) frame the contained alert is
#: interpolated INTO. The contained alert is a {alert} VALUE, never promoted to the template
#: position, so an alert value carrying its own "{...}" cannot re-template.
_DEFAULT_GOAL_TEMPLATE = "Investigate this alert and root-cause it:\n{alert}"


@dataclass(frozen=True)
class AlertmanagerAdapter:
    """Normalize an AM webhook body into a :class:`NormalizedTrigger` (or a :class:`NoOp`).

    Implements the webhook-shaped :class:`~sei_omnigent.omni.adapters.base.TriggerAdapter`. The
    parse → is_firing/is_enrolled → extract_allowlisted → derive_incident_key chain is the
    containment template every adapter follows (INV-6) — preserved verbatim here. A non-firing /
    non-enrolled / unparseable body yields a :class:`NoOp` (the router answers a no-op, not an
    error: the page already routed to the human).

    ``goal_template`` is TRUSTED (manifest-injected). The contained alert is interpolated as the
    ``{alert}`` value, never promoted to the template position.
    """

    goal_template: str = _DEFAULT_GOAL_TEMPLATE

    def parse(self, body: object) -> NormalizedTrigger | NoOp:
        """Parse + contain + normalize one AM webhook body, or return a :class:`NoOp`.

        Wraps the existing pure chain, distinguishing the two non-match cases in the NoOp's
        reason: unparseable → ``NoOp("parse_error")``; non-firing or non-enrolled →
        ``NoOp("not_enrolled")``; otherwise the contained, framed alert (``extract_allowlisted``)
        + the derived incident key become a :class:`NormalizedTrigger`. ``venue_handle`` and
        ``dedup_key`` are both the incident key (for AM they coincide); the initiator is the
        fixed system identity (no human principal on the AM path).
        """
        try:
            webhook = parse_webhook(body)
        except WebhookError:
            # A malformed body — log the offender (it is attacker-influenceable) and surface the
            # distinct reason so a parse-error storm is separable from a not-enrolled flood.
            _log.exception("omni.am.parse_error — unparseable AM webhook body, no-op")
            return NoOp("parse_error")
        if not is_firing(webhook) or not is_enrolled(webhook):
            return NoOp("not_enrolled")
        incident_key = derive_incident_key(webhook)
        alert = extract_allowlisted(webhook)
        return NormalizedTrigger(
            initiator=_AM_SYSTEM_INITIATOR,
            goal=self.goal_template.format(alert=alert),
            venue_handle=incident_key,
            dedup_key=incident_key,
            trust=_AM_TRUST,
            requested_skills=(),
            payload=alert,
        )
