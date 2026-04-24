# Chaos Suite Summary Template

Filled in by `scripts/collate-summary.sh` at the end of a successful run. Saved to `<platform-repo>/clusters/<env>/release-<version>/results/<YYYY-MM-DD>-summary.md`.

Format target: the release-6-5 session's summary doc.

Placeholders are `<ANGLE_BRACKET>` — scripts substitute them literally.

---

# Release <VERSION> Chaos Suite Results — <DATE>

**Cluster:** `<CONTEXT>`
**Namespace:** `<NS>`
**Run ID:** `<RUN_TS>`
**Operator:** `<OPERATOR>`
**Runbook version:** `<RUNBOOK_REF>` (sei-protocol/platform#169 at `<COMMIT_SHA>`)

## Headline

**Total tests:** <N>
**Pass:** <P>
**Fail:** <F>
**Flagged (manual follow-up):** <FL>

<ONE_LINE_NARRATIVE — what the overall picture looks like>

---

## Per-Test Results

### <TEST_ID>: <TEST_NAME>

| Phase | Signal | Value |
|-------|--------|-------|
| Baseline | Height | `<H0>` |
| Baseline | Sec/block | `<S0>` |
| Baseline | Mempool depth | `<M0>` |
| Baseline | Per-pod spread | `<SPREAD0>` |
| Mid-chaos | `<SIGNAL>` | `<VALUE>` |
| Mid-chaos | Injection verified | `<Y/N>` — `<EVIDENCE>` |
| Post-chaos | Height | `<H1>` |
| Post-chaos | Sec/block | `<S1>` |
| Post-chaos | Recovery | `<Y/N>` — `<NARRATIVE>` |
| Post-chaos | Leftover check | `<PASS>` or `<LEAKED: what, remediation>` |

**What worked:** <narrative>

**What broke:** <narrative>

**Independent verification:** <narrative — e.g., `tc -s qdisc` output showing packet-loss was applied, tproxy logs showing HTTPChaos rerouting, log-timestamp delta for time-skew>

**Action items:**
- Platform: <items, or none>
- Protocol: <items, or none>

---

<!-- repeat the above block per test -->

---

## Session Summary

**Auto-remediated:** <list, or "none — no auto-remediation was performed">
**Manual intervention required:** <list, or "none">

## Platform Action Items

<aggregated from per-test sections — deduplicated, prioritized if obvious>

## Protocol Action Items

<aggregated from per-test sections — deduplicated, prioritized if obvious>

## Appendix: Run State

Per-test state files: `<skill-path>/state/run-<TS>/*.yaml`
Audit log: `<skill-path>/state/run-<TS>/audit.log`

These are NOT committed (gitignored). For debugging a failed run, reference them via the run ID above.
