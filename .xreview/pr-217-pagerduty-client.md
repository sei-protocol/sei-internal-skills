# xreview ledger — PR #217 (PagerDuty client + post-then-claim post_back)

Target: `sei_omnigent/src/sei_omnigent/omni/{_pagerduty.py,receiver.py,_dedup.py}` (PR #217, branch `plt-715-pagerduty-client`)
Class: component
Tier: T2

## Round 1

State: OPEN
OpenFindings: 11
Convergence: split
Blinded: yes
Dissenter: systems-engineer

Slate: security-specialist · idiomatic-reviewer (auto-wired, code diff) · systems-engineer (assigned dissenter).
Verdicts: security = APPROVE-WITH-CHANGES · idiomatic = RATIFY (1 divergence-with-consequence + comment-discipline) · systems = **DISSENT** (2 HIGH).

### Boundary table

| Interface / Boundary | Provider | Consumer | Status | Evidence | Raised by |
|---|---|---|---|---|---|
| Marker idempotency = "closed" double-propose | `_pagerduty` | `_dedup`/`receiver` | MISMATCH | GET-notes (`:228`)→POST (`:240`) is check-then-act TOCTOU; PD notes have no conditional-create/read-after-write atomicity. Prose asserts "CLOSED" ×3 (`_dedup.py:14`, `receiver.py:292`); true strength = "narrowed to list-read lag, best-effort" | systems (A, HIGH) |
| Note-POST retry vs idempotency | `_pagerduty` | — | MISMATCH | retry loop `_request:317-345` re-POSTs on post-commit timeout without re-running the marker scan (`:228` is outside the loop) → double note; "retry is a safe no-op" claim (`:40/:79`) not implemented; test retries the *find* GET only (`test:887`) | systems (B, HIGH) |
| Notes-scan completeness | `_pagerduty` | — | MISMATCH | `_NOTES_SCAN_LIMIT=100` single-page (`:273`) → marker scrolls off a >100-note incident → false-negative → double-post; silent | systems (C, MED) **+** security (3b, LOW) — independent |
| `httpx.AsyncClient` lifecycle | `_pagerduty` | `receiver` | MISMATCH | `from_config` mints client (`:235`) with no `aclose` on the `PagerDutyPoster` Protocol; receiver lifespan (`receiver.py:600-605`) never closes it → fd/pool leak; docstring names an owner that can't close it | systems (D, MED) |
| `unclaim_post` Protocol addition | `_dedup` | `receiver` | MISMATCH | unconditional `discard` (`:147-149`), no owner-token unlike `release_run(incident_key, run_id)` (`:134-138`) → ABA hazard for the documented shared-store impl | systems (F, LOW) |
| post_back / supervise_run error classes | `receiver` | — | MISMATCH | bare `except Exception` absorb (`:409-414`) downgrades non-PD programming bugs to a `warning`, hiding real defects behind the reliability story | systems (E, LOW–MED) |
| Secret exposure via repr | `_pagerduty` | — | MISMATCH | frozen dataclass default `__repr__` includes `token`/`from_email` cleartext (`:137/:141`); frame-locals/Sentry capture leaks the token | security (4a, MED) |
| Redactor default (raw text → PD) | `receiver` | `_pagerduty` | MISMATCH | `redact` defaults to identity (`receiver.py:485`) — fail-*open* into PD if the manifest omits a real redactor; breaks the fail-loud-at-boot discipline | security (4c, MED) |
| Enrolled-set misconfig | `_pagerduty` | — | MISMATCH | empty `enrolled_service_ids` → silent total deny-all (safe but a silent WallE outage); no boot assert, unlike `ReceiverConfig.__post_init__` | security (2a, MED) |
| `base_url` source-of-truth | `_pagerduty` | — | MISMATCH | dead `AsyncClient` base_url (every URL absolute, `:235/:313`); tests assert a misleading `AsyncClient(base_url=...)` (`test:689`); + no host-allowlist → tampered manifest can exfiltrate the token to an arbitrary host | idiomatic (divergence) **+** security (5a, LOW) |
| `incident_id` path interpolation | `_pagerduty` | — | MISSING | `incident.get("id")` interpolated into the path (`:290`) with no shape validation — notes-only holds only by assuming a well-formed PD id | security (1a, LOW) |

### Idiom addendum
- RATIFY. Seam shape native (frozen dataclass + structural Protocol + DI + named-ctor); EAFP transport + `from last_exc` correct; INV-7 notes-only is test-guarded (source-grep + runtime request-shape) — a documented invariant with a real guard.
- Divergence-with-consequence: dead `base_url` (in boundary table above).
- Comment-discipline (owned axis): history-in-source prose in `post_back`/`_dedup`/receiver module docstrings ("the old order", "now closed", "the reliability fix") — narrate the transition, not the end state. Trim to present-state. (Corroborates systems-A's overclaim finding.)
- Style nit: `isinstance(value, (str, bytes))` → `str | bytes` (py312).

### Deferred / accepted-risk (operator decision pending)
- **security 3a (MED) — forgeable predictable marker → denial-of-diagnosis.** Marker `[walle:run:<incident_key>]` is predictable; a PD-tenant insider with note-write can pre-plant it to suppress a real WallE diagnosis. Bounded: requires in-tenant note-write; WallE is propose-only (degrades a diagnostic aid, causes no wrong action); the human still has the original page. Fix = HMAC the marker under a manifest secret (removes forgeability, preserves restart idempotency). Security reviewer: defer-acceptable for MVP given a fully-trusted-operator PD tenant; **un-defer condition: any non-operator integration or external responder gains note-write on an enrolled service.** → **operator (Brandon) ACCEPTED-WITH-RISK for MVP, 2026-06-23**, on the named un-defer condition above (PD tenant is operator-only today). No code change this PR.

Resolution plan: fix the 11 boundary findings (Round 2); 3a routed to the operator.
