# Scenario Analysis Guide

How `platform-release-manager` translates raw metrics into release-quality narrative.

## Metric windows

For each scenario, three windows are queried:
- **Baseline** — 5 minutes before chaos injection. Establishes the normal operating point.
- **Chaos** — the injection duration. What the chain looked like under stress.
- **Recovery** — 5 minutes after chaos expires. How quickly the chain returned to baseline.

## Outcome classification

| Outcome | Criteria |
|---|---|
| `PASS` | Block production continued throughout; metrics recovered to ≤110% of baseline within 2× chaos duration |
| `HALT+RECOVER` | Block production stopped during chaos (correct behavior when >1/3 validators affected); resumed cleanly post-chaos; zero data loss |
| `DEGRADED` | Block production continued but with measurable impact (>20% block time increase or >10% TPS reduction); recovered to baseline |
| `FAIL` | Block production stopped AND chain did not self-recover, OR unexpected divergence/corruption detected |

## Per-metric interpretation

### Block time (p50, p95)

| Delta from baseline | Language |
|---|---|
| < 10% | "no measurable impact on block interval" |
| 10–30% | "modest block time inflation; chain continued at degraded pace" |
| 30–100% | "block time stretched from Xs to Ys during injection" (quote exact numbers) |
| > 100% | "block time more than doubled; chain was under severe liveness pressure" |
| No blocks | "chain halted — correct BFT behavior when >1/3 validators could not commit" |

For halts: always explain WHY halting is correct ("Tendermint chooses safety over liveness when 2f+1 cannot be maintained; the chain stopped rather than risk divergent state").

### TPS

Quote the actual numbers: "Run TPS dropped from ~300 to ~180 during the injection window." Include the recovery time: "Both metrics returned to baseline within three minutes of the chaos expiring."

If TPS drops to near-zero but block time is normal, note the disconnect: this indicates the load generator may have been affected rather than the chain.

### Tx success rate

Express as a percentage of baseline: "Transaction success rate held at 99.2% (baseline: 99.7%)." For rates below 95%, flag explicitly.

### Mempool size

Quote peak values: "Mempool grew from ~90 to 594 txs during the injection and drained within the recovery window." Unbounded growth (>2× baseline at chaos-end) warrants a note.

## BFT threshold rule

When chaos is scoped to >1/3 of validators, explicitly apply the BFT reasoning:

> "The test applied [fault] to [N/total] validators — crossing the 1/3 BFT durability threshold. The chain correctly halted rather than risk safety. This is expected behavior, not a failure."

When chaos is scoped to <1/3:

> "With [N/total] validators affected, the remaining 9 held quorum at 2f+1 and drove consensus at [rate]. The affected validators reconverged automatically after recovery."

## Fault family narratives

### Infrastructure faults (pod-kill, container-kill)

Focus: does the controller recover without human intervention? Quote the mark-ready timing. Note whether the sidecar survived the container kill (different recovery path than a full pod kill).

### Network degradation (partition, latency, packet loss, bandwidth)

Focus: gossip resilience. Note whether the chain slowed or halted. For partition tests: was it a true split-brain or did gossip route around the dropped edges? Quote the p2p topology observation.

### Resource starvation (CPU, memory, disk I/O)

Focus: did hardware pressure bleed into consensus timing? CPU and memory usually don't affect block production unless the pod is OOM-killed. Disk I/O at >50% of all validators typically forces a halt (WAL fsync timeout).

### Adversarial conditions (time skew, byzantine, RPC chaos)

Focus: protocol-layer defense. For time skew: Tendermint's BFT-time design uses the median of validator vote timestamps — explain this makes minority clock drift irrelevant. For byzantine/packet-corruption: explain the stacked defense (MAC, TCP checksum, Tendermint application-layer validation). For RPC chaos: note the architectural separation between validator p2p and external RPC.

## Executive Summary synthesis

Write the executive summary LAST. Structure:

1. **Recommendation sentence** — one sentence, first. ("Recommendation: proceed with arctic-1 deployment on 4/30.")
2. **Fault coverage paragraph** — which fault families, overall BFT result, recovery quality.
3. **Notable findings** — any FAIL outcomes, regressions, bugs found, PRs required.
4. **Deployment alignment** — final plain-language statement of the team's position.

Keep the executive summary to ≤4 short paragraphs. Every sentence earns its place.

## Per-scenario section template

Each of the 13 scenarios gets:

### [Scenario Name]

**Summary** — one sentence stating what was injected and what happened at the chain level.

**Key Signals** — data narrative with exact numbers. Baseline → chaos delta → recovery timing. Quote at least block time and TPS. If the chain halted, explain the BFT reasoning.

**Release Significance** — one paragraph connecting the result to the release decision. What failure mode does passing this test rule out? What would a failure here mean for production?

[3 Grafana panel embeds: TPS, block time, error rate]
