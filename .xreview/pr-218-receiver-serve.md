# xreview ledger — PR #218 (receiver production serve-wiring)

Target: `sei_omnigent/src/sei_omnigent/omni/{serve_receiver.py,_redact.py,_omnigent_session.py,receiver.py}` (PR #218, branch `plt-715-receiver-serve`)
Class: component
Tier: T2

## Round 1

State: OPEN
OpenFindings: 8
Convergence: split
Blinded: yes
Dissenter: systems-engineer

Verdicts: security = APPROVE-WITH-CHANGES · idiomatic = RATIFY (1 style nit) · systems = **DISSENT** (1 blocker + 1 must + 2 should).

### Findings

| # | Sev | Lens | Finding | Resolution |
|---|---|---|---|---|
| B1 | BLOCKER | systems | A lazy-`create` / transport failure (omnigent server unreachable at investigation time) is caught by `drive_to_terminal`'s blanket `except Exception` (`driver.py:213`) and returned as `BUDGET_EXHAUSTED` — a PD note that says the investigation ran and hit its envelope when it never started. Fail-into-a-confident-lie on the most likely live failure. | Add a distinct `ERRORED`/`FAILED` terminal to the driver; classify unexpected (non-budget) stream exceptions as it; `render_note` surfaces an honest "investigation failed to start/run" artifact, not a budget axis. |
| S-CRIT | CRITICAL-cond | security | `OmnigentClient` is built with only `X-Forwarded-Email` (a spoofable claim) and **no authenticating credential** — if the server resolves identity from the header alone, full identity-escalation. | Attach the projected `omnigent-api` SA token as `Authorization: Bearer` (mirror the host's B1 `proxy_bearer_header`), read **fresh per call** (kubelet rotates the projected token). The server's kube-rbac-proxy sidecar TokenReviews it; the email is the app principal *within* the authed channel. Manifest (next PR) provisions the projected token + netpol. Attachment mechanism stays VERIFY-ON-LIVE. |
| S-HIGH | HIGH | security | Redactor leaks: PEM private-key block, bare JWT, conn-string userinfo (`user:pass@`), `Authorization: Basic`, labeled secret (`password: …`), Slack webhook URL. | Add all six patterns (append-only) + tests. |
| B3 | MUST | systems | `Receiver.aclose` closes the standing client while `drain()` may have **abandoned** in-flight runs (60s drain timeout abandons, doesn't cancel) → use-after-close on exactly the slow-shutdown case WallE exists for. | Cancel-then-await survivors before the factory client close, or gate the factory close behind drain-completed. |
| B2 | SHOULD | systems | Default `max_in_flight=16` fans an **unproven** shared single `OmnigentClient` 16-wide (cross-stream bleed = INV-6 risk, or pool starvation). | Default `max_in_flight=1` until a live concurrent soak proves the shared client concurrency-clean; name the un-defer condition. |
| B4 | SHOULD | systems | `LiveSessionFactory.aclose` not idempotent (unconditional `client.close()`) → double-close hazard. | Guard with a closed-flag. |
| SF1 | SHOULD | security | Inline `PD_API_TOKEN` env fallback re-exposes the token (proc env / crash dump) — file path is the secure one. | Drop the inline fallback (file-only) + `field(repr=False)` on the client; add an `http://` PD-URL downgrade test (cleartext token). |
| I1 | style | idiomatic | `_omnigent_session.py:270` comment narrates "the spike's `client.aclose()` was wrong" — history, not present-state. | Trim to the present-state assertion. |

### Idiom addendum
- RATIFY. Deferred-import seam consistent with `serve_main`; the duck-typed `aclose` probe is the *correct* idiom (optional capability, not a Protocol contract); the lazy-async-create-behind-sync-stream reconciles the `SessionLike` signature with an async ctor idiomatically. Only nit: I1 (above).

### Bundle / supply-chain (flagged, manifest/image owner, not this PR)
- The agent bundle (`OMNI_RECEIVER_AGENT_BUNDLE`) has no integrity check here — a tampered bundle changes agent behavior (could undermine INV-7). Verify provenance at the image/manifest layer.
- In-cluster transport to the omnigent server (`OMNI_RECEIVER_SERVER_URL`) scheme unchecked — relies on mesh/netpol; document the assumption.

Resolution plan: fix B1, S-CRIT, S-HIGH, B3, B2, B4, SF1, I1 (Round 2); the OmnigentClient API specifics stay honestly VERIFY-ON-LIVE; supply-chain items routed to the manifest PR.

## Round 2

State: RESOLVED
OpenFindings: 0
Convergence: unanimous
Blinded: yes
Dissenter: systems-engineer

All 8 R1 findings traced closed (not faith-accepted); fixes introduced no regression.
- **systems-engineer (dissenter): RATIFY.** B1 closed — ERRORED is additive, excluded from `_TRUNCATED`, no exhaustive `match` to break; a budget breach is still BUDGET_EXHAUSTED, a genuine exception now ERRORED with an honest render. B3 closed — drain cancels+awaits survivors within `cancel_grace_s=5` before the factory close (ordering test-pinned). B2/B4 closed. No regression (additive enum well-handled; fresh-bearer failure correctly surfaces as ERRORED; cancel-mid-post strictly safer than the old abandon — run-claim released in `finally`, PD marker covers any half-post). 2 non-blocking live-run-hardening notes logged.
- **security-specialist: APPROVE.** SA bearer read fresh-per-create + fail-closed + boot-fail-loud + no-secret-in-repr (X-Forwarded-Email-alone escalation closed); all 6 redactor patterns verified against live inputs + backref-safe + idempotent; PD token file-only + `http://` fails closed.
- **idiomatic-reviewer: RATIFY.** R1 comment nit fixed; the non-frozen `LiveSessionFactory` correctly matches the package's value-vs-runtime dataclass partition (same category as `_TaskTracker`); ERRORED extension, redactor restructure, per-create header callable all native. 2 pure-style nits (enum-comment wrap, a below-gate ternary).

Folded in before merge (cheap, closes named-real gaps):
- The two security follow-up redactor patterns — no-username DSN (`redis://:pass@`) + `/`-bearing password (`mongodb://u:p/w@`) — now caught (username optional, password allows `/`). + 2 tests.
- The idiom enum-comment wrap → single present-state line. (The below-gate ternary left to the formatter, per P11.)

Verification: ruff clean; installed suite **270 passed**; omnigent-free **259 passed / 6 skipped**.

Carried VERIFY-ON-LIVE (honest, not asserted): the `OmnigentClient`/`SessionsChat.create` per-call `headers=` attachment mechanism — needs the prove-the-run.
Routed to the manifest PR: the receiver SA + projected `omnigent-api` token volume (`OMNI_RECEIVER_PROXY_BEARER_TOKEN_FILE`) + the PD token mounted as a file + a default-deny netpol (AM→receiver ingress, receiver→server egress); agent-bundle integrity (supply-chain).

## Round 3 (Cursor Bugbot — declared review-gate check)

State: RESOLVED
OpenFindings: 0
Convergence: unanimous
Blinded: yes
Dissenter: systems-engineer

Bugbot (NEUTRAL with 4 posted findings → gate fails closed → each assessed on merits against HEAD, two of them resolved by introspecting the INSTALLED omnigent 0.2.0 surface):

| Finding | Sev | Verdict | Resolution |
|---|---|---|---|
| Watchdog timeout misclassified ERRORED | Med | REAL (regression from B1) | The `asyncio.timeout` watchdog raises builtins `TimeoutError`, which B1's blanket-except laundered into ERRORED. Added `except TimeoutError` → BUDGET_EXHAUSTED (wall-clock axis, cancelled) before the blanket. `httpx.TimeoutException` is NOT a `TimeoutError` subclass (verified) → transport timeouts correctly stay ERRORED. Both directions test-pinned. |
| Live session auth wiring mismatch | High | REAL | `SessionsChat.create` has no `headers=` param (verified) — the per-create kwarg would `TypeError`. Auth moved to the `OmnigentClient` ctor: `headers={X-Forwarded-Email}` + `auth=_FreshBearerAuth` (httpx.Auth, fresh-per-request token read). Verified the client forwards `headers`/`auth` onto its shared httpx client + every namespace method/SSE stream uses it → both creds ride every request. |
| Wrong OmnigentClient shutdown method | Med | FALSE POSITIVE | `OmnigentClient` has `close()`, NOT `aclose` (verified against installed 0.2.0) — Bugbot compared the stale spike. `client.close()` is correct; comment updated to verified-on-0.2.0. |
| Lease default ignores wall clock | Med | REAL (footgun) | Default lease now derived = `wall_clock + 120` (clears the C1 `__post_init__` floor); a raised wall_clock boots instead of failing. 900→1020 unchanged. |

Re-verify (systems-dissent + security; blinded) on the fix delta:
- **systems-engineer (dissenter): RATIFY.** All 4 closed + verified against ground truth; watchdog×`aclosing` red-team clean (inner-to-outer unwind closes the stream before the single best-effort cancel); the auth rework also removed a latent `TypeError`. No cross-fix regression.
- **security-specialist: APPROVE.** Verified the rework against the 0.2.0 *source* — the escalation is closed (X-Forwarded-Email can never travel without the bearer), fresh-per-request, fail-closed, no secret in repr.

VERIFY-ON-LIVE collapsed this round (introspection-grounded, no longer assumed): `OmnigentClient.close()`, `SessionsChat.create`'s real signature, and the `headers`/`auth` forwarding onto the httpx client. **Remaining honest VERIFY-ON-LIVE:** that the server + kube-rbac-proxy sidecar actually *read* X-Forwarded-Email and TokenReview the bearer (env-dependent — the prove-the-run); `client.sessions` accessor + agent-bundle provisioning.

Tracked non-blocking nits (recorded, not gating): the keystone test couples to the private `client._http` (add a "pinned-version internal" marker on the next omnigent bump); `_FreshBearerAuth.auth_flow` does a sync file read on the event loop (moot at `max_in_flight=1`; switch to async `auth_flow` at the N>1 un-defer soak).

Verification: ruff clean; installed **274 passed**; omnigent-free **261 passed / 6 skipped**.

### Round 3b (Bugbot re-scan on the fix — one follow-up)

Bugbot re-scanned the #4 fix and flagged **"Stream TimeoutError misclassified as budget"** — a real refinement: my `except TimeoutError` over-caught, mapping ANY `TimeoutError` (incl. one raised *inside* the stream/session — an inner per-op deadline) to BUDGET_EXHAUSTED. (The R3 systems-dissent had called this exact case "acceptable/edge"; Bugbot escalated it correctly.) Fixed: gate the budget branch on `asyncio.timeout`'s `watchdog.expired()` (3.11+) — only a genuine watchdog expiry is BUDGET_EXHAUSTED; a non-watchdog `TimeoutError` falls through to ERRORED. New test `test_inner_timeouterror_stays_errored_not_budget`. ruff clean; installed **275 passed**; omnigent-free **262 passed / 6 skipped**.
