# Scenario Analysis Guide

How `platform-release-manager` translates the live-harness signal into a
release-quality narrative. The **outcome is the Job-log verdict** (authoritative);
metrics are *supporting context, not an independent second opinion* — there is no
pre-chaos baseline phase in the nightly harness.

## What a PASS means (read this first)

`TestNightlyChaosSuite` asserts **liveness**: per scenario it gates injection, then
`WaitHeightAdvances(+3)` under fault, then recovery, validators `Ready`, and
`WaitCaughtUp`. So **PASS = "the chain stayed live under the fault and recovered."**
It is **blind to partial tx-correctness** (e.g. 40% tx rejection while blocks still
produce). Every report says this; the go/no-go it supports is a **liveness** gate.
The headline is `LIVENESS GO` / `LIVENESS NO-GO`, never a bare "GO".

## The 10 scenarios

network-partition, packet-loss, network-latency, bandwidth-limit, byzantine,
pod-failure, container-kill, cpu-stress, time-skew, memory-stress.

A scenario absent from the log reconciles to `DID NOT RUN`; a scenario with no
metric series is `NO DATA` — never a green 0.

## Outcome vocabulary (do not reclassify — narrate)

| Outcome | Source | Meaning |
|---|---|---|
| `PASS` / `FAIL` | Job log | The Go test's liveness verdict — authoritative |
| `DID NOT RUN` | reconciliation | No `--- PASS\|FAIL` line for this scenario in a complete log |
| `UNKNOWN (log truncated)` | reconciliation | Log hit the byte cap before this scenario's line — not "missing" |
| `VERDICT UNAVAILABLE — metrics-only` | verdict-unavailable rule | Log GC'd (7d) but metrics survive (15d) — **headline suppressed**, never a synthesized pass |
| `RUN EXPIRED` | freshness | Neither log nor raw metrics survive — re-run |

## Metric annotations (supporting evidence)

Each scenario carries a metric summary. Quote exact numbers; label them as *context*.

### Halt / liveness (validator set)

Computed on `tendermint_consensus_height{component="validators"}` as **set-level
advancement**: max validator height at window-end > window-start, still advancing in
the final N samples. This is restart-aware — a restarted validator returns as a **new
`pod`/`instance_name` series**, so a single node restart (expected in
`pod-failure`/`container-kill`) does not read as a halt, and a real halt (the set
stops advancing) is caught even though height is a gauge (a `rate()`-based check would
be fooled by the counter-reset-looking gauge drop). Narrate a metric-observed halt as
supporting the log verdict — and when the log says PASS but metrics show a halt, flag
the disagreement for the reader rather than overriding the log.

For a genuine liveness loss: explain the BFT reasoning — Tendermint chooses safety over
liveness when 2f+1 cannot be maintained; the chain stops rather than risk divergence.

### Block time

`histogram_quantile(0.95, ...block_interval_seconds_bucket...)` reported as a
**bucket-bounded p95** (bounded by histogram bucket edges — not a precise worst-case),
with a **height-derived mean** interval as the always-present fallback. Quote both when
present: "p95 block interval ~ Xs (bucket-bounded); height-derived mean ~ Ys."

### TPS and mempool (transparency-only — NOT release signals)

`sei_cosmos_throughput_transaction_count` and `tendermint_mempool_size` are **~0 by
design**: the chaos suite runs **no load generator** (seiload is benchmark-only), so
there is no throughput to measure and the mempool stays empty. Carry these values for
transparency only — **never narrate a TPS "degradation shape" or a mempool
"backpressure" note**. The chaos release signals are **halt + block-interval**;
throughput is the **deferred phase-2 benchmark report**.

### Provenance marker (per cell)

`OK` (measured) / `NO DATA` (absent series) / `PARTIAL` (Thanos partial response —
understated, treat as degraded) / `VERDICT-GC'd` (log gone, metrics-only). Surface the
marker; never present a `PARTIAL`/`NO DATA` cell as a clean measurement.

## Fault-family narratives (for the "Release Significance" paragraph)

- **Infrastructure** (pod-failure, container-kill): controller recovery without human
  intervention; a single restart is expected, not a halt.
- **Network** (partition, packet-loss, latency, bandwidth-limit): gossip resilience;
  did the chain slow or halt; did gossip route around dropped edges.
- **Resource** (cpu-stress, memory-stress): does hardware pressure bleed into consensus
  timing; usually not unless a pod is OOM-killed.
- **Adversarial** (byzantine, time-skew): protocol-layer defense. Time-skew: BFT-time
  uses the median of validator vote timestamps, so minority clock drift is irrelevant.
  Byzantine: stacked defense (MAC, TCP checksum, application-layer validation).

## Executive summary synthesis

Write it LAST, <=4 short paragraphs:
1. **Recommendation** — one sentence: `LIVENESS GO` / `LIVENESS NO-GO`, with the
   tx-correctness caveat. If the headline is suppressed, say so and recommend a re-run —
   never invent a verdict.
2. **Coverage** — fault families exercised and the overall liveness result.
3. **Notable findings** — FAILs, metric/verdict disagreements, `NO DATA`/`PARTIAL`
   cells, DID NOT RUN gaps.
4. **Decision** — the team's position, in plain language, scoped to liveness.

## Per-scenario section

**Summary** — one sentence: fault injected + log outcome.
**Key Signals** — the metric annotation with exact numbers, labeled as supporting
context; the provenance marker; BFT reasoning when a halt is observed.
**Release Significance** — what liveness failure mode a PASS rules out; what a FAIL
would mean for production.
[Panels: block-time, TPS, mempool]
