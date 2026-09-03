# Notion Report Template

Structure of the Notion page created by `platform-release-manager`. The agent fills
placeholders; this template is static. Written to be shared with engineering leadership.
It is a **liveness** report — the headline reads `LIVENESS GO` / `LIVENESS NO-GO`, never
a bare "GO", and carries the tx-correctness caveat inline.

---

## Page properties

```json
{
  "parent": { "database_id": "<NOTION_DATABASE_ID>" },
  "properties": {
    "Name":       { "title": [{ "text": { "content": "Release Validation — <RELEASE_IMAGE> (run <TOKEN>)" } }] },
    "Status":     { "select": { "name": "<LIVENESS GO|LIVENESS NO-GO|VERDICT UNAVAILABLE>" } },
    "Image":      { "rich_text": [{ "text": { "content": "<RELEASE_IMAGE>" } }] },
    "RunToken":   { "rich_text": [{ "text": { "content": "<TOKEN>" } }] },
    "Scenarios":  { "number": 10 },
    "Passed":     { "number": <N> },
    "Failed":     { "number": <N> }
  }
}
```

---

## Block sequence

### 1. Run-identity header (callout)

```
Release Validation — run <TOKEN>
Release image:  <SEID_IMAGE_CHAOS>
Run age:        <D.d>d   (raw freshness bound: 15d — <within/EXPIRED>)
Job:            <job_name>
```

### 2. Headline (callout)

- **Not suppressed** — the outcome, emphasized, with the caveat inline:
  ```
  LIVENESS GO — all 10 scenarios stayed live and recovered.
  Caveat: liveness gate only; transaction correctness is NOT validated by this suite.
  ```
  or `LIVENESS NO-GO — <N> scenario(s) failed the liveness gate. Caveat: …`
- **Suppressed** (verdict unavailable / run expired) — NO go/no-go; render:
  ```
  NO GO/NO-GO — verdict log GC'd but metrics survive (7d/15d band).
  Do NOT ship on metrics alone. Re-run the nightly to obtain a verdict.
  ```

### 3. Executive Summary (heading 1)

Four paragraphs max. See `analysis-guide.md`. Paragraph 1 = the recommendation +
liveness scope; 2 = fault-family coverage; 3 = notable findings (FAILs, metric/verdict
disagreements, NO DATA / PARTIAL cells, DID NOT RUN gaps); 4 = the decision.

### 4. What Was Tested (heading 2)

| Family | Scenarios | What's being exercised |
|---|---|---|
| Infrastructure | pod-failure, container-kill | Controller recovery paths |
| Network | network-partition, packet-loss, network-latency, bandwidth-limit | p2p gossip resilience |
| Resource | cpu-stress, memory-stress | Hardware headroom under load |
| Adversarial | byzantine, time-skew | Protocol-layer defenses |

One-sentence operating context: 5 nodes per chain (4 validators + 1 rpc node),
`CHAOS_DURATION=3m` per scenario.

### 5. Test Results (heading 2)

Repeat ×10, ordered by fault family.

---

#### [scenario] · <PASS|FAIL|DID NOT RUN|VERDICT UNAVAILABLE> · provenance:<OK|NO DATA|PARTIAL|VERDICT-GC'd> (heading 3)

**Summary** — one sentence: fault injected + Job-log outcome.

**Independent verification:** `NOT VERIFIED` by default. Replace only with the
evidence that the fault actually landed — `tc -s qdisc` output, a log-timestamp
delta, tproxy logs. A scenario whose injection evidence is absent reports
`NOT VERIFIED`; it does not report a clean run.

**Key Signals** (supporting context — not a second verdict)
- Halt/liveness: `<ADVANCING|HALTED|NO DATA>` — blocks produced, quorum advanced
- Block time: `p95 ~ Xs (bucket-bounded); height-derived mean ~ Ys`
- TPS / mempool: `~0 by design` — chaos runs no load generator; transparency-only,
  NOT a release signal (do not narrate a degradation shape or backpressure)

Include BFT reasoning when a halt is observed. If the provenance is `NO DATA` /
`PARTIAL` / `VERDICT-GC'd`, say so directly — never present it as a clean measurement.

**Release Significance** — what liveness failure mode a PASS rules out; what a FAIL
would mean for production.

[Image — block time panel]  ·  [Image — TPS panel]  ·  [Image — mempool panel]

---

*(repeat for all 10 scenarios)*

### 6. Action Items (heading 2)

Platform + protocol action-item tables (or "none identified").

### 7. Appendix (heading 2)

**Per-scenario metric tables** — one table per scenario: Metric | Value | Provenance.

**Raw data pointers**
```
Metrics: federated prometheus-prod, chain_id=chaos-<TOKEN>-<scenario>, max_source_resolution=0
Job log: harbor / ns nightly / <job_name>
Panel images: s3://harbor-validation-results/chaos-suite-reports/<TOKEN>/
```

---

## Tone and style

- Lead with the liveness recommendation, not methodology.
- Quote actual numbers in every "Key Signals" section; label metrics as supporting context.
- Apply BFT theory explicitly when it explains a halt.
- Never dress a `NO DATA` / `PARTIAL` / verdict-unavailable cell as a clean pass.
- If the headline is suppressed, the page states it plainly and recommends a re-run.
