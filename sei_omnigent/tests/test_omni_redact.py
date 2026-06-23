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


# --- S-HIGH: the six added patterns + the widened base64 rule -----------------


def test_scrubs_pem_private_key_block() -> None:
    block = (
        "-----BEGIN RSA PRIVATE KEY-----\n"
        "MIIEpAIBAAKCAQEA0Z3VS5JJcds3xfnygWyF0qZ3VS5JJcds3xfn\n"
        "ygWyF0qZ3VS5JJcds3xfnygWyF0qZ3VS5JJcds3xfnygWyF0qZ3\n"
        "-----END RSA PRIVATE KEY-----"
    )
    out = redact(f"the agent dumped a key:\n{block}\nin the transcript")
    assert "BEGIN RSA PRIVATE KEY" not in out
    assert "MIIEpAIBAAKCAQEA" not in out
    assert _PLACEHOLDER in out


def test_scrubs_bare_jwt() -> None:
    jwt = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N7"
    out = redact(f"the session carried {jwt} with no Bearer prefix")
    assert jwt not in out
    assert _PLACEHOLDER in out


def test_scrubs_connection_string_password_keeping_context() -> None:
    out = redact("DSN postgres://walle:s3cr3tP4ss@db.sei.internal:5432/incidents")
    assert "s3cr3tP4ss" not in out  # the password is gone
    assert "postgres://walle" in out  # the scheme + user survive (the note still reads)
    assert "db.sei.internal" in out  # the host survives
    assert _PLACEHOLDER in out


def test_scrubs_no_username_dsn_password() -> None:
    # A passwordful DSN with NO username (`redis://:pass@host`) — the password is still scrubbed.
    out = redact("cache redis://:s3cretValue99@cache.sei.internal:6379/0")
    assert "s3cretValue99" not in out
    assert "redis://" in out and "cache.sei.internal" in out
    assert _PLACEHOLDER in out


def test_scrubs_dsn_password_containing_a_slash() -> None:
    # A password with a `/` (`mongodb://u:p/w@h`) — scrubbed despite the slash.
    out = redact("mongodb://admin:pa/ss/word@mongo.sei.internal:27017/incidents")
    assert "pa/ss/word" not in out
    assert "mongodb://admin" in out and "mongo.sei.internal" in out
    assert _PLACEHOLDER in out


def test_scrubs_authorization_basic_header() -> None:
    secret = "d2FsbGU6c3VwZXJzZWNyZXRwYXNzd29yZA=="
    out = redact(f"curl -H 'Authorization: Basic {secret}' https://api")
    assert secret not in out
    assert _PLACEHOLDER in out


def test_scrubs_kv_labelled_secret_keeping_the_label() -> None:
    # A bare (low-entropy) password the shape rules would miss, caught by the label rule. The
    # label survives so the note still reads; only the value is scrubbed.
    out = redact("config had password=hunter2 and api_key: short-key-99")
    assert "hunter2" not in out
    assert "short-key-99" not in out
    assert "password" in out  # the label survives
    assert "api_key" in out
    assert _PLACEHOLDER in out


def test_scrubs_slack_webhook_url() -> None:
    url = "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX"
    out = redact(f"the runbook posts to {url} on alert")
    assert "hooks.slack.com/services" not in out
    assert "XXXXXXXXXXXXXXXXXXXXXXXX" not in out
    assert _PLACEHOLDER in out


def test_scrubs_standard_base64_blob() -> None:
    # A bare STANDARD-base64 blob (uses `+`/`/`, not just url-safe `-`/`_`) — the widened rule
    # must catch it where the url-safe-only rule would have missed the `+`/`/` chars.
    blob = "QWxhZGRpbjpvcGVuIHNl+c2Ftf/ZWFiY2RlZmdoaWprbG1ub3BxcnN0dXZ3eHl6QUJD"
    assert "+" in blob and "/" in blob and len(blob) >= 40
    out = redact(f"opaque blob {blob} in the dump")
    assert blob not in out
    assert _PLACEHOLDER in out


def test_redact_is_idempotent() -> None:
    # redact(redact(x)) == redact(x): re-running over already-scrubbed text must be a fixpoint
    # (the placeholder must not re-trigger a pattern into a different result).
    note = (
        "key sk-ant-api03-" + "Q1w2E3r4" * 6 + " AKIAIOSFODNN7EXAMPLE "
        "Authorization: Bearer " + "AbCdEf12" * 4 + " "
        "postgres://walle:s3cr3tP4ss@db.sei.internal/incidents "
        "password=hunter2 token: " + "Zm9vYmFy" * 6 + " "
        "https://hooks.slack.com/services/T0/B0/XXXXXXXXXXXXXXXX "
        "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.dozjgNryP4J3jVmNHl0w5N7"
    )
    once = redact(note)
    assert redact(once) == once
