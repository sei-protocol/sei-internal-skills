# Cross-review ledger — /idiomatic Python pack · FastAPI framework overlay

Target:       branch `framework/idiomatic-python-fastapi-overlay` (`.claude/skills/idiomatic/references/language-pack-python.md` §4/§5/§6/§7; `.claude/skills/idiomatic/references/examples-python.md` FastAPI section); PR from this branch
Class:        skill-package (framework overlay added to an existing language pack)
Tier:         T2 (Component — a §5 framework overlay + companion examples + §7 anchors, within the shipped Python pack #191)
Scope:        Move FastAPI from the Python pack's deferred sub-packs to an authored §5 overlay (rows FA1–FA6), grounded in FastAPI's official docs, with every lint anchor verified by running the real `ruff` (0.15.17). The strategic line holds: encode the framework's OWN published guidance ("Prefer the `Annotated` version", the `response_model` filtering contract, the def-vs-async-def threadpool rule, lifespan-over-`on_event`) + the FastAPI-specific lint surface (`FAST001/002/003`, the `B008` inversion + `extend-immutable-calls` exemption, `ASYNC210/251`), not a hand-rolled opinion. pydantic/Django stay deferred. Rolls up into both standards champions automatically (generic detection — no per-framework wiring).
Dissenter:    FastAPI-fidelity (assigned dissenter — the dominant risk is a fabricated/mis-cited lint anchor or an over-claimed doc citation; the dissenter independently re-runs `ruff` on every anchor and reads the live fastapi.tiangolo.com docs + the cited GitHub discussion)

## Round 1
State:        REVISE
OpenFindings: resolved in Round 2 (3 blocking + assigned-dissent defect + non-blocking polish)
Convergence:  split

| Lens | Verdict | Finding |
|---|---|---|
| FastAPI-fidelity (assigned dissenter — runs `ruff`, reads docs) | DISSENT | **Every lint anchor fired with exact-match messages and all five interrogated doc claims held against the live docs** (the "Prefer `Annotated`" through-line verbatim on /tutorial/dependencies + /query-params; discussion #7463 real + recommends both `fastapi.Depends` and `fastapi.params.Depends`; `FAST003`-not-ordering split honest; returned-`HTTPException` and `on_event`/lifespan accurate; `FAST004+` confirmed nonexistent). One surviving defect: **FA5's pack-table anchor cited `ASYNC251/ASYNC230`**, but `ASYNC230` is `blocking-open-call` (file open), NOT the blocking-HTTP hazard the row's `requests` cue names — the examples file correctly cited `ASYNC210`. Table ⇔ examples disagreed. Explicit flip-condition: fix the anchor → RATIFY. |
| pack-conformance (idiomatic-reviewer) | REVISE | **[blocking] F1** — the new codes (`FAST001/002/003`, `ASYNC210`) were cited in §5 + examples but **absent from the §7 `lint_anchors[]` table**, violating the pack's own "cite a lint code only from §7" rule. **[blocking] F2** — FA-overlay intro enumeration imprecise + FA4 was the one lint-anchored row with **no example pair**. **[non-blocking]** F3 (split FA4's anchored/judgment consequence), F4 (cross-ref §4 `B008` → FA2), F6 (place the overlay in §6 severity_model). Verified FA2 correctly frames the `B008` inversion as the documented exception (cross-refs, doesn't contradict the base pack); every FA row carries a genuine runtime consequence; example format conforms. |
| prose-steward | RATIFY | Dual-aligned; reads correctly for the scanning human + the acting agent. Lead sentences load-bearing; judgment-only halves typed explicitly; no markdown defect, no multi-code-span-in-italic, no phrase-repetition. Two optional [style] notes: FA4's slash-fused consequence could pair each failure to its cue; FA6's middle idioms (`BackgroundTasks`/`APIRouter`) lacked a per-idiom consequence (the one spot diverging from the repo's "each rule states its failure-consequence" convention). |

### Round 1 resolution (applied before Round 2)
- **Dissent (FA5 anchor):** `ASYNC230` → `ASYNC210` in the FA5 table cell (`ASYNC251` for `time.sleep`, `ASYNC210` for a blocking HTTP call); table now agrees with the examples file. Re-verified: `ASYNC210` fires on `requests.get` in an `async def`, `ASYNC230` does not.
- **F1 (blocking):** added three FastAPI rows to the §7 `lint_anchors[]` table (FA1/FA3/FA4 → `FAST002`/`FAST001`/`FAST003`; FA2 → the `B008` inversion + `extend-immutable-calls` exemption; FA5 → `ASYNC251`/`ASYNC210`) with rule-set, opt-in caveat, version caveat, and the hidden-fix note; extended the anti-fabrication note to type FA6 + FA4-ordering as judgment-only. Provenance (hidden/unsafe fix, FastAPI/flake8-async linter origin) confirmed by running `ruff`.
- **F2 (blocking):** FA-overlay intro now maps FA1 (`FAST002`), FA3 (`FAST001`), FA4 (`FAST003`) explicitly; added a verified `FAST003` example pair to `examples-python.md`.
- **F3 / F4 / F6 (non-blocking):** FA4 consequence split into anchored vs judgment halves; §4 mutable-default line cross-refs §5 FA2; §6 severity_model places the overlay (FA3/FA5 correctness, FA1/FA2/FA4 idiom-with-consequence, FA6 mixed).
- **Prose notes:** FA4 consequence paired to its cue; FA6's `BackgroundTasks`/`APIRouter` each given a one-clause consequence.

## Round 2
State:        RESOLVED
OpenFindings: 0 (one new non-blocking provenance fix folded in as finalization)
Convergence:  unanimous

| Lens | Verdict | Finding |
|---|---|---|
| FastAPI-fidelity (assigned dissenter) | RATIFY | Round-1 defect fixed — FA5 table now cites `ASYNC251`+`ASYNC210`, `ASYNC230` removed entirely, table ⇔ examples ⇔ live tool all agree. The three new §7 rows + the new `FAST003` example are faithful: every code's canonical slug, upstream linter origin, firing behavior, and caveat (opt-in, hidden-fix, the `B008` inversion) re-verified by observed `ruff` output, not asserted. No new fidelity defect. |
| pack-conformance (idiomatic-reviewer) | RATIFY | Both Round-1 blockers (F1, F2) and all three non-blockers (F3, F4, F6) genuinely resolved — re-confirmed by re-running every anchor, not by reading prose. No §5↔§7 contradiction, no example-vs-anchor mismatch, no §6 tier conflict. **[non-blocking N1]:** the §7 autofix cell omitted that `FAST003` also carries a hidden unsafe fix (verified) → fold "all 3: hidden" / "FAST001/002/003". |
| prose-steward | RATIFY | Both Round-1 notes resolved; the new prose (§6 tier sentence, three §7 rows, §4 cross-ref clause, FAST003 example) is table-clean, backtick-balanced, internally consistent, dual-aligned. Two optional [style] residuals only (the §7 flags already align with anchor order; FA6 cell density — no action advised). |

### Verdict
RESOLVED — unanimous RATIFY in Round 2; zero open findings. The assigned fidelity dissenter independently re-ran `ruff 0.15.17` on every anchor (FAST001/002/003 fire with exact-match messages; the `B008` inversion reproduces and the `Annotated` form passes `--select FAST,B008` clean; `ASYNC251`+`ASYNC210` fire on the blocking async route) and verified every doc claim against the live FastAPI docs + GitHub discussion #7463. The dominant risk (a fabricated or mis-cited anchor) is closed: every cited code exists, derives from the stated upstream linter, and was demonstrated firing. The one Round-2 non-blocking finding (N1 — `FAST003`'s hidden fix omitted from the §7 autofix cell) was folded in as finalization.

**Rollup completeness:** the FastAPI overlay lives inside `language-pack-python.md`, which both standards champions already load generically (idiomatic-reviewer "detect language → load the pack"; systems-engineer "the language-idiom layer is owned by /idiomatic … load the relevant pack") — no per-framework wiring needed. Cursor Bugbot + CI: evaluated on PR open (Bugbot skip/NEUTRAL satisfies the check half for this doc/skill PR per the recorded review-gate policy; failing/findings still block).
