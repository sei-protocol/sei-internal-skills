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

#: Secret SHAPES paired with their replacement template, ordered most-specific-first (a
#: vendor-prefixed key is matched as that key, not folded into the generic long-token rule).
#: Most rules collapse the whole match to ``_PLACEHOLDER``; the two CONTEXT-preserving rules
#: (a connection-string userinfo, a `label=value` secret) carry a backreference template that
#: keeps the surrounding context (the scheme/host, the label) and scrubs only the secret run —
#: so the note still reads while the credential is gone. Each is conservative — anchored on a
#: literal vendor prefix or a header keyword — so a benign value of similar length is not
#: over-matched by the prefixed rules. The trailing generic rules (long hex / base64) are the
#: catch-all for unprefixed high-entropy strings and are deliberately length-gated to avoid
#: eating ordinary identifiers (a UUID, a git sha) that are shorter than a real secret.
_PATTERNS: tuple[tuple[re.Pattern[str], str], ...] = (
    # A PEM private-key block (RSA/EC/OPENSSH/generic): the whole armored block collapses to one
    # placeholder. DOTALL-scoped to the block so the multi-line body between the markers is eaten;
    # placed FIRST so the base64 body inside is never partially matched by the generic rules.
    (
        re.compile(
            r"-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----",
            re.DOTALL,
        ),
        _PLACEHOLDER,
    ),
    # A Slack incoming-webhook URL — a post-anywhere credential in the path; scrub the whole URL.
    (re.compile(r"https://hooks\.slack\.com/services/[A-Za-z0-9/_-]+"), _PLACEHOLDER),
    # Anthropic API key: `sk-ant-` + the key body (alnum, dash, underscore).
    (re.compile(r"sk-ant-[A-Za-z0-9_-]{16,}"), _PLACEHOLDER),
    # Generic OpenAI-style `sk-` secret key (covers the broader `sk-...` family; placed AFTER
    # the Anthropic rule so an `sk-ant-` key is reported as the more specific shape first).
    (re.compile(r"sk-[A-Za-z0-9]{20,}"), _PLACEHOLDER),
    # AWS access key id: `AKIA`/`ASIA` + 16 uppercase-alnum.
    (re.compile(r"\b(?:AKIA|ASIA)[A-Z0-9]{16}\b"), _PLACEHOLDER),
    # GitHub tokens: `ghp_`/`gho_`/`ghu_`/`ghs_`/`ghr_` + body.
    (re.compile(r"\bgh[pousr]_[A-Za-z0-9]{20,}\b"), _PLACEHOLDER),
    # A connection-string with inline userinfo (`scheme://user:pass@host`): scrub ONLY the
    # `:pass` segment, preserving the scheme/user/host so the note still reads as a DSN. The
    # username is OPTIONAL (`redis://:pass@host` — a passwordful no-user DSN), and the password
    # run is any non-`@` chars (it may contain `/`, e.g. `mongodb://u:p/w@h`). The backreference
    # keeps the `scheme://[user]` + `@`; the placeholder replaces just the password.
    (
        re.compile(r"(?P<pre>\b[a-zA-Z][a-zA-Z0-9+.-]*://[^\s:/@]*):[^\s@]+@"),
        rf"\g<pre>:{_PLACEHOLDER}@",
    ),
    # An Authorization header value: `Authorization: Bearer/Token/Basic <secret>` (header name +
    # the scheme + the token). Case-insensitive on the keyword; the secret body is the catch.
    (
        re.compile(r"(?i)\bauthorization\b\s*[:=]\s*(?:bearer|token|basic)\s+[A-Za-z0-9._=+/~-]{8,}"),
        _PLACEHOLDER,
    ),
    # A bare `Bearer <token>` / `Basic <b64>` / `Token token=<token>` not preceded by the header
    # name (a curl `-H` the agent quoted, or a raw PD `Token token=` form).
    (re.compile(r"(?i)\b(?:bearer|basic|token=?)\s+[A-Za-z0-9._=+/~-]{16,}"), _PLACEHOLDER),
    # A bare JWT: three url-safe-base64 segments separated by dots, leading `eyJ` (the `{"`
    # header). Scrubs a JWT pasted with no `Bearer` scheme in front of it.
    (re.compile(r"\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+"), _PLACEHOLDER),
    # A `label=value` secret: `password=...`, `api_key: ...`, `secret=...`, `token: ...` etc.
    # Scrub ONLY the value (keeping the label + separator so the note still reads). Value is a
    # non-space run (quotes/commas/semicolons end it too) — a labelled secret need not match a
    # high-entropy shape, so this catches a bare password the shape rules miss.
    (
        re.compile(
            r"(?i)(?P<label>\b(?:password|passwd|pwd|secret|api[_-]?key|token|access[_-]?key))"
            r"(?P<sep>\s*[:=]\s*)['\"]?[^\s'\",;]+",
        ),
        rf"\g<label>\g<sep>{_PLACEHOLDER}",
    ),
    # Generic long base64 / token secret: 40+ chars of the base64 alphabet (url-safe `-_` AND
    # standard `+/`), optional `=` padding. Length-gated above a UUID (36) so it does not eat
    # ordinary identifiers; the catch-all for an unprefixed high-entropy credential. The `+`/`/`
    # inclusion catches a bare STANDARD-base64 blob the url-safe-only rule would miss.
    (re.compile(r"[A-Za-z0-9+/_-]{40,}={0,2}"), _PLACEHOLDER),
    # Generic long hex secret: 40+ hex chars (a sha-256, a hex-encoded key/HMAC). 40 is the
    # sha-1 length; a git sha is also caught, which is acceptable over-redaction in a note.
    (re.compile(r"\b[0-9a-fA-F]{40,}\b"), _PLACEHOLDER),
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
    for pattern, replacement in _PATTERNS:
        scrubbed = pattern.sub(replacement, scrubbed)
    if len(scrubbed) > _MAX_NOTE_CHARS:
        # Truncate, not drop: a partial proposal still helps the on-call. The marker keeps a
        # truncated note from reading as a complete one (the §3.5 misleading concern, by length).
        keep = _MAX_NOTE_CHARS - len(_TRUNCATION_MARKER)
        scrubbed = scrubbed[:keep] + _TRUNCATION_MARKER
    return scrubbed
