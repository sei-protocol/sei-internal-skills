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

## Round 2

State: RESOLVED-WITH-ACCEPTED-RISK
OpenFindings: 0
Convergence: unanimous
Blinded: yes
Dissenter: systems-engineer

Slate re-dispatched blinded on the fixed code, each verifying its own Round-1 findings closed (traced, not faith-accepted) + red-teaming the fixes.

Verdicts:
- **systems-engineer (dissenter): RATIFY.** All 4 HIGH/MED + LOW findings traced closed. Verified live that `ConnectTimeout` subclasses `TimeoutException` (not `ConnectError`) → the connect-vs-read retry split correctly retries only provably-unsent POSTs (B closed); pagination bounded + UNCONFIRMED fail-closed-skip confirmed (C closed); owner-token ABA tested, `admit_post` predicate wiring intact (F closed); prose now present-state best-effort (A closed). No new breaking bug.
- **idiomatic-reviewer: RATIFY.** Round-1 carryovers resolved (history-prose gone; `base_url` single-source + allowlisted, internally consistent). New code (`_MarkerScan` enum, `idempotent` retry-split, `aclose`, `_require_redactor` sentinel, owner-token dict) reads native. One divergence-with-consequence (dead `now` seam) + 2 comment-discipline style nits — all applied (below).
- **security-specialist: APPROVE-WITH-CHANGES → resolved.** All 5 Round-1 findings traced closed in code (host-allowlist confirmed a real `urlsplit` host parse — host-confusion/userinfo/path tricks rejected). One new finding + nits, all applied (below).

Round-2 fix batch (commit pending) — applied exactly as specified:
- **[security, correctness-grade] https-scheme guard** in `from_config` — completes 5a (host-allowlist left the cleartext-`http://` token-exfil vector open). Test `test_from_config_rejects_a_cleartext_http_endpoint`.
- **[security, LOW] `_PD_ID_RE` → `\A[A-Z0-9]+\Z`** — closes the trailing-newline-slips-past-`$` residual (1a).
- **[idiomatic, divergence-with-consequence] dropped the dead `now` injected seam** (+ `import time`, the test kwarg, 2 docstring refs) — config that read load-bearing but was wired to nothing (same class as the R1 `base_url` find).
- **[idiomatic, style/comment-discipline] removed stale "follow-up PR" narration** at `receiver.py` banner + `LoggingPoster` docstring, and one test transition-narration parenthetical.

Verification: ruff clean; full suite **221 passed**.

Accepted-with-risk (operator, 2026-06-23): **3a** forgeable predictable marker → denial-of-diagnosis, MVP-accepted on the named un-defer condition (any non-operator/external note-write on an enrolled PD service). This is the sole reason the terminal is `RESOLVED-WITH-ACCEPTED-RISK` rather than `RESOLVED`.

Deferred to the manifest/wiring PR (not an open finding against this component-only PR, which has no `Receiver`/client boot wiring): the production boot path must route through `from_config` (host/enrolled/scheme guards live there, not the raw ctor) and inject a real redactor — security's "production boot path covered" evidence lands there.

## Round 3 (Cursor Bugbot — declared review-gate check)

State: RESOLVED-WITH-ACCEPTED-RISK
OpenFindings: 0
Convergence: unanimous
Blinded: yes
Dissenter: systems-engineer

Bugbot (a declared automated check) returned NEUTRAL but with 3 posted findings — the review-gate fails closed on that, so each was assessed on the merits against HEAD:

| Finding | Severity | Verdict | Resolution |
|---|---|---|---|
| Skip paths retain post slot | High | REAL (slate missed it) | `post_note` → `bool`; `post_back` claims only on a real write, releases on a fail-closed skip. The missed-post/denial-of-diagnosis (inverse of the double-post bug). |
| Note POST retries duplicate notes | Medium | STALE | Already fixed in R1 (`_add_note` posts `idempotent=False`); Bugbot scored a pre-fix commit. Covered by `test_note_post_read_timeout_is_not_retried_no_double_post`. |
| Empty notes page ends scan | Low | REAL | Empty page + `more=True` → UNCONFIRMED (fail-closed), not ABSENT — was an early-exit that could miss a later-page marker. |

Re-review (systems-dissent + idiomatic; blinded) on the fix:
- **systems-engineer (dissenter): RATIFY.** All 5 points COMPATIBLE. Verified releasing-on-skip introduces **no** double-post: the durable guard is the PD marker scan inside `post_note` (untouched); the in-memory slot was never the cross-restart guarantee and on a skip path guarded a note that was never written. `posted` provably bound on every non-raise path; scan-split loop-safe. Suggested one composition test → **added** (`test_post_back_does_not_double_post_across_a_restart_rescan`).
- **idiomatic-reviewer: RATIFY.** Zero findings. The bare `bool` is the right call (the caller needs only posted-vs-not; the 5 skip reasons are logged at origin; the alternative outcome is a raise, not a third return — so it's genuinely binary). The `_scan_for_marker` split de-conflates the two termination conditions and reads clearer. No comment-discipline regression — updated docstrings are present-state.

Verification: ruff clean; full suite **224 passed**; omnigent-free 218 passed / 5 skipped.

Follow-up (obs wiring, not a code defect): the new `"skipped"` metric label needs to be in the Grafana/result enum on the dashboard side — observability-platform agent, tracked for the wiring PR.
