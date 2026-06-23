"""Conservative secret-scrubbing redactor for the receiver's egress chokepoint (PLT-715).

The receiver's ``post_back`` funnels every terminal note through a single ``redact`` callable
before it reaches PagerDuty (``receiver.py`` §3.5). ``_require_redactor`` is the fail-closed
sentinel — production must inject a REAL one; this is it. It scrubs common secret SHAPES from
the rendered note so an investigation transcript that quoted a credential (a leaked key in a
log line, an ``Authorization`` header in a curl the agent ran) does not land in a PD note that
a wide audience can read.

Fail-closed by contract: the chokepoint treats ANY raise from ``redact`` as "post nothing"
(``post_back`` releases the slot + escalates rather than leaking raw text). So this module
must NOT swallow an internal error into a pass-through — :func:`redact` lets an unexpected
error propagate, and the one place it would otherwise return unredacted text (a value longer
than the cap) instead truncates with an explicit marker.

SECURITY-REVIEWED MVP SCOPE: the pattern set below is the conservative first cut — the shapes
most likely to leak from an ops investigation. It is pattern-based (shape matching), NOT a
secret scanner: a credential with no recognizable shape (a bare password, a short opaque
token) is NOT caught. The review slate hardens this — additions are cheap (append a pattern),
and the design is conservative-by-construction (over-redaction is safe; the note is a proposal,
not a machine-parsed payload). It is pure + unit-tested (no omnigent / network / I/O), so it is
never on the omnigent-free unit suite's import path.

Design 12 §3.5; INV (single redacted egress).
"""

from __future__ import annotations

import re

#: The replacement every matched secret collapses to. A fixed token (not a length-preserving
#: mask) so a match leaks neither the secret's value nor its length.
_PLACEHOLDER = "[REDACTED]"

#: Hard cap on the post-redaction note length. A pathological artifact (a runaway transcript,
#: a megabyte of log paste) must not become a multi-MB PD note — PD's own limits aside, an
#: unbounded note defeats the human skim the proposal exists for. Over the cap, the note is
#: truncated with an explicit marker (NOT dropped — a truncated proposal still helps). Sized
#: well above a normal root-cause artifact; this is a backstop, not a tuning knob.
_MAX_NOTE_CHARS = 16_000

#: The truncation marker appended when a note exceeds the cap, so the reader knows the tail
#: was cut (a silently-truncated note could read as a complete one — the §3.5 misleading-rate
#: concern, applied to length).
_TRUNCATION_MARKER = "\n\n[note truncated — exceeded redactor length cap]"

#: Secret SHAPES, ordered most-specific-first (a vendor-prefixed key is matched as that key,
#: not folded into the generic long-token rule). Each is conservative — anchored on a literal
#: vendor prefix or a header keyword — so a benign value of similar length is not over-matched
#: by the prefixed rules. The trailing generic rules (long hex / base64-ish) are the catch-all
#: for unprefixed high-entropy strings and are deliberately length-gated to avoid eating
#: ordinary identifiers (a UUID, a git sha) that are shorter than a real secret.
_PATTERNS: tuple[re.Pattern[str], ...] = (
    # Anthropic API key: `sk-ant-` + the key body (alnum, dash, underscore).
    re.compile(r"sk-ant-[A-Za-z0-9_-]{16,}"),
    # Generic OpenAI-style `sk-` secret key (covers the broader `sk-...` family; placed AFTER
    # the Anthropic rule so an `sk-ant-` key is reported as the more specific shape first).
    re.compile(r"sk-[A-Za-z0-9]{20,}"),
    # AWS access key id: `AKIA`/`ASIA` + 16 uppercase-alnum.
    re.compile(r"\b(?:AKIA|ASIA)[A-Z0-9]{16}\b"),
    # GitHub tokens: `ghp_`/`gho_`/`ghu_`/`ghs_`/`ghr_` + body.
    re.compile(r"\bgh[pousr]_[A-Za-z0-9]{20,}\b"),
    # An Authorization header value: `Authorization: Bearer/Token <secret>` (header name + the
    # scheme + the token). Case-insensitive on the keyword; the secret body is the catch.
    re.compile(
        r"(?i)\bauthorization\b\s*[:=]\s*(?:bearer|token)\s+[A-Za-z0-9._=+/~-]{8,}"
    ),
    # A bare `Bearer <token>` / `Token token=<token>` not preceded by the header name (e.g. a
    # curl `-H 'Authorization: ...'` the agent quoted, or a raw PD `Token token=` form).
    re.compile(r"(?i)\b(?:bearer|token=?)\s+[A-Za-z0-9._=+/~-]{16,}"),
    # Generic long base64-ish / token secret: 40+ chars of url-safe base64 alphabet. Length-
    # gated above a UUID (36) so it does not eat ordinary identifiers; the catch-all for an
    # unprefixed high-entropy credential (a generic API token, a session cookie value).
    re.compile(r"\b[A-Za-z0-9_\-]{40,}={0,2}\b"),
    # Generic long hex secret: 40+ hex chars (a sha-256, a hex-encoded key/HMAC). 40 is the
    # sha-1 length; a git sha is also caught, which is acceptable over-redaction in a note.
    re.compile(r"\b[0-9a-fA-F]{40,}\b"),
)


def redact(note: str) -> str:
    """Scrub recognized secret shapes from ``note`` and cap its length. Fail-closed by contract.

    Applies each shape pattern (most-specific-first) replacing matches with ``[REDACTED]``,
    then enforces the hard length cap with an explicit truncation marker. Returns the scrubbed
    text; raises nothing it can foresee, but does NOT defensively swallow — an unexpected error
    propagates so the chokepoint fails closed (posts nothing) rather than leaking raw text.

    Pure + deterministic: no I/O, no omnigent, no network. Over-redaction is the safe failure
    direction (the note is a human-read proposal, not a machine payload), so the patterns lean
    aggressive on shape and the length cap is a backstop, not a tuning knob.
    """
    scrubbed = note
    for pattern in _PATTERNS:
        scrubbed = pattern.sub(_PLACEHOLDER, scrubbed)
    if len(scrubbed) > _MAX_NOTE_CHARS:
        # Truncate, not drop: a partial proposal still helps the on-call. The marker keeps a
        # truncated note from reading as a complete one (the §3.5 misleading concern, by length).
        keep = _MAX_NOTE_CHARS - len(_TRUNCATION_MARKER)
        scrubbed = scrubbed[:keep] + _TRUNCATION_MARKER
    return scrubbed
