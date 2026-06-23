"""Tests for the receiver's egress redactor (PLT-715).

The redactor is the production fill-in for ``receiver._require_redactor`` (the fail-closed
sentinel): it scrubs common secret SHAPES from a terminal note before it reaches PagerDuty.
Pure + omnigent-free, so the full pattern set is provable here. Over-redaction is the safe
failure direction (the note is a human-read proposal), so a test asserts a secret is GONE,
not that surrounding prose is byte-preserved.
"""

from __future__ import annotations

from sei_omnigent.omni._redact import (
    _MAX_NOTE_CHARS,
    _PLACEHOLDER,
    _TRUNCATION_MARKER,
    redact,
)


def test_scrubs_anthropic_key() -> None:
    secret = "sk-ant-api03-" + "A1b2C3d4" * 6
    out = redact(f"the agent logged {secret} in the transcript")
    assert secret not in out
    assert _PLACEHOLDER in out


def test_scrubs_aws_access_key_id() -> None:
    out = redact("found AKIAIOSFODNN7EXAMPLE in the env dump")
    assert "AKIAIOSFODNN7EXAMPLE" not in out
    assert _PLACEHOLDER in out


def test_scrubs_github_token() -> None:
    secret = "ghp_" + "x9Y8z7W6" * 5
    out = redact(f"git remote carried {secret}")
    assert secret not in out
    assert _PLACEHOLDER in out


def test_scrubs_authorization_bearer_header() -> None:
    secret = "eyJhbGciOiJ" + "AbCdEf12" * 4
    out = redact(f"curl -H 'Authorization: Bearer {secret}' https://api")
    assert secret not in out
    assert _PLACEHOLDER in out


def test_scrubs_pd_token_token_form() -> None:
    secret = "Token token=" + "k7N3pQ9rS1tU" * 2
    out = redact(f"the poster sent {secret}")
    assert "k7N3pQ9rS1tU" * 2 not in out
    assert _PLACEHOLDER in out


def test_scrubs_long_hex_secret() -> None:
    secret = "a3f" + "0" * 60  # 63 hex chars (a hex-encoded HMAC/key)
    out = redact(f"signature was {secret}")
    assert secret not in out
    assert _PLACEHOLDER in out


def test_scrubs_long_base64ish_secret() -> None:
    secret = "Zm9vYmFyYmF6cXV4" * 4 + "=="  # 64+ url-safe base64 chars
    out = redact(f"opaque session cookie {secret}")
    assert secret not in out
    assert _PLACEHOLDER in out


def test_leaves_ordinary_prose_and_short_identifiers_intact() -> None:
    # A normal artifact line: short identifiers (a UUID, a short id) are NOT secret-shaped and
    # must survive — over-redaction is safe but eating every short token would gut the note.
    note = "incident on service api-gateway at 14:02 UTC; pod sei-omni-7f4 restarted"
    assert redact(note) == note


def test_caps_overlong_note_with_truncation_marker() -> None:
    # Spaces every few chars so the run is NOT itself a long-token secret shape (which would
    # collapse to one placeholder and never hit the length cap) — this exercises the cap on
    # genuinely long, non-secret prose.
    note = ("word " * (_MAX_NOTE_CHARS // 2))
    assert len(note) > _MAX_NOTE_CHARS
    out = redact(note)
    assert len(out) <= _MAX_NOTE_CHARS
    assert out.endswith(_TRUNCATION_MARKER)


def test_under_cap_note_is_not_truncated() -> None:
    note = "a short clean root-cause artifact"
    out = redact(note)
    assert not out.endswith(_TRUNCATION_MARKER)
    assert out == note


def test_is_pure_and_deterministic() -> None:
    note = "key sk-ant-api03-" + "Q1w2E3r4" * 6 + " and AKIAIOSFODNN7EXAMPLE"
    assert redact(note) == redact(note)
