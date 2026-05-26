# Notion Report Template

Structure of the Notion page created by `platform-release-manager`. The agent fills in placeholders; this template is static. Written to be shared directly with engineering leadership.

---

## Page properties

```json
{
  "parent": { "database_id": "<NOTION_DATABASE_ID>" },
  "properties": {
    "Name":       { "title": [{ "text": { "content": "Release Validation — <SHA7> (<YYYYMMDD>)" } }] },
    "Status":     { "select": { "name": "<PASS|PARTIAL|FAIL>" } },
    "Commit":     { "rich_text": [{ "text": { "content": "<SHA7>" } }] },
    "Date":       { "date": { "start": "<YYYY-MM-DD>" } },
    "Scenarios":  { "number": 13 },
    "Passed":     { "number": <N> },
    "Failed":     { "number": <N> }
  }
}
```

---

## Block sequence

### 1. Header callout

Emoji: ✅ (all pass), ⚠️ (partial), ❌ (failures present)

```
Release Validation — <SHA7> · <YYYY-MM-DD>
<N>/13 scenarios passed · Recommendation: <proceed|hold>
```

### 2. Executive Summary (heading 1)

Four paragraphs max. See `analysis-guide.md` for synthesis instructions.

**Paragraph 1** — Recommendation sentence + overall BFT result.

**Paragraph 2** — Fault families covered, what they exercise, how the chain behaved across each family.

**Paragraph 3** — Notable findings, regressions, required PRs (if any). "None" if clean.

**Paragraph 4** — Team alignment and deployment decision.

### 3. What Was Tested (heading 2)

Table: Fault Family | Scenarios | What's being exercised

| Family | Scenarios | What's being exercised |
|---|---|---|
| Infrastructure | Pod Failure, Container Kill | Platform recovery paths |
| Network degradation | Network Partition, Latency, Packet Loss, Bandwidth Limit | p2p gossip resilience |
| Resource starvation | CPU Stress, Memory Stress, Disk I/O Latency | Hardware headroom under load |
| Adversarial | Time Skew, Byzantine Fault, RPC Chaos | Protocol-layer defenses |

Followed by 1-sentence operating context: cluster size (N validators, M RPCs), load rate, suite scope.

### 4. Test Results (heading 2)

Repeat the following block ×13, one per scenario. Order: by fault family (infrastructure → network → resource → adversarial).

---

#### [Scenario Name] · ✅/⚠️/❌ PASS|DEGRADED|HALT+RECOVER|FAIL (heading 3)

**Summary** — one sentence.

**Key Signals**

Metrics paragraph with exact numbers. Quote at minimum:
- Block time: `Xs → Ys during injection (Z× baseline); recovered in Tm`
- TPS: `~N tps baseline → ~M tps during chaos; full recovery in Tm`
- Success rate: `N%` (or note if unchanged)

Include the BFT threshold context when applicable.

**Release Significance**

One paragraph: what failure mode does this rule out? What would a failure here mean for production?

[Image block — TPS panel]
*TPS during <scenario> — chain_id: <chain_id>*

[Image block — block time panel]
*Block time during <scenario>*

[Image block — error rate panel]
*Failed transactions during <scenario>*

---

*(repeat for all 13 scenarios)*

### 5. Platform Action Items (heading 2)

Table (if any). Columns: # | Item | Description | Link

If no action items: "No platform action items identified."

### 6. Protocol Action Items (heading 2)

Table (if any). Columns: # | Item | Description | Link

If none: "No protocol action items surfaced."

### 7. Divider + Appendix (heading 2)

**Per-scenario metric tables** — one table per scenario with baseline/chaos/recovery values for all 5 metrics. Label columns clearly: Metric | Baseline | Chaos | Recovery | Delta.

**S3 raw data link** paragraph:
```
Raw seiload reports: s3://harbor-validation-results/nightly/chaos-*/SUITE_ID/
Panel images: s3://harbor-validation-results/chaos-suite-reports/SUITE_ID/
```

---

## Tone and style

- Lead with the recommendation, not methodology
- Quote actual numbers in every "Key Signals" section
- Apply BFT theory explicitly when it explains an observation
- "Release Significance" explains implications, not test descriptions
- Executive Summary stands alone — someone who skips the detail sections gets the full picture
- Avoid hedge language unless genuinely uncertain ("it appears", "seems to")
- If a scenario produced no data, say so directly: "No Grafana data was available for this scenario. The S3 report shows..."
