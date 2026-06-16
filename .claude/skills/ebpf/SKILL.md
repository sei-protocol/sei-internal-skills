---
name: ebpf
category: performance
model: claude-opus-4-8
description: "Use when designing or running kernel-level observability or a performance benchmark with eBPF/bpftrace — 'profile this with eBPF', 'why is this slow — blocked, not busy?', 'off-CPU / lock-contention analysis', 'design a bpftrace probe for X', 'benchmark <workload> performance', 'kernel-level observability for <service>', '/ebpf'. Pluggable packs; composed by the systems-engineer agent. Anti-triggers: NOT for application-side OpenTelemetry SDK instrumentation (use opentelemetry-expert); NOT for the telemetry backend, PromQL/LogQL, or dashboards (use observability-platform-engineer); NOT for SLO/SLI targets, alert tier, or runbooks (use sre-engineer); NOT for app/architecture review where eBPF is just the diagnostic (use /systems); NOT for Cilium NetworkPolicy intent or Istio config (use network-specialist). Deploying a privileged probe to a cluster is a one-way door — design-first, human + security sign-off required."
---

# eBPF

Design and run **kernel-level observability and performance benchmarks** that see what pod/Prometheus telemetry structurally cannot — off-CPU/blocked time, lock/futex contention, syscall and block-I/O latency *distributions*, scheduler run-queue latency, TCP-level health. This is a *technique* skill (a how-to you adapt to the target) with a *discipline spine* (rules that survive pressure). The technique is the measure-first method; the spine is what stops a capable model from guessing a cause it never measured, DoSing a hot node with a careless probe, or shipping a benchmark that hides the very stall it exists to find.

This skill is the operating manual for the `systems-engineer` agent's eBPF/kernel-perf work and is also directly invocable.

## Why this skill exists (read this first)

A capable model already knows Brendan Gregg's USE method and the bcc/bpftrace tool catalog. Pressure-testing the failure modes shows the gap is not knowledge — it is discipline:
- It **guesses** ("probably blocked on the lock") instead of retrieving the signal (`offcputime`, futex tracing) — the plausibility-hallucination that `/root-cause` also guards.
- It **reaches for a tool before establishing the effect + USE baseline**, so it measures the wrong resource.
- It ships a tail-latency benchmark on a **closed-loop** load generator that backs off when the system stalls — *understating* p99 by exactly the stall it was built to find (coordinated omission).
- It treats eBPF as a **replacement** for in-process profilers, losing the Go-semantic detail `pprof` gives.
- It attaches a **high-frequency probe on a hot node** without bounding overhead — the verifier proves memory-safety and termination, never performance.

So the value here is **not** a kernel-tracing textbook. It is the discipline that makes every perf claim measured-and-cited, every probe overhead-bounded and safe, and every privileged deploy gated. The pack is the tool→signal→concern checklist and the citable authority; the spine is the product.

## Guardrails

Refusal conditions — these hold under time pressure, a senior voice anchoring on one theory, and a tidy-looking flamegraph:

1. **Measure, don't assume — every signal is retrieved, not extrapolated.** No perf conclusion without the literal probe invocation + its verbatim output (a histogram, a folded stack, a count). "It's probably the lock" is a hypothesis to test with `offcputime`/futex tracing, never a finding. Paraphrased output is banned. (Inherits `/root-cause`'s evidence rule.)
2. **No privileged eBPF on a shared/prod node without the one-way-door gate — attach OR deploy.** *Any* privileged eBPF attach is root-on-node: a DaemonSet, a one-shot Job, an ephemeral `bpftrace -e` one-liner, or pointing a pre-approved standing agent at a new target — all cross the gate equally. "It's just a read-only one-liner, not a deployment" is **not** an exemption (the gate is about the privileged kernel attach, not the verb). This skill *designs and authors* probes/harnesses; it never attaches or runs a privileged probe on a shared or production cluster without explicit human + security sign-off, and **never on a validator/consensus-critical node** at all.
3. **Overhead is part of the probe — bound it before you attach.** Overhead = per-event cost × event frequency. A high-frequency kprobe or an event-streaming probe (no in-kernel aggregation) can perturb or DoS a hot node and skew the very measurement. Prefer in-kernel aggregation (map histograms), rate-limit, and measure overhead on a non-prod target first. The verifier guarantees memory-safety + termination, NOT low overhead.
4. **Open-loop for tail latency.** A benchmark whose metric is p99/p99.9/tail MUST use an open-loop / coordinated-omission-resistant harness (constant arrival rate, measure queueing) — a closed-loop generator that backs off on stall understates the tail by the stall.
5. **eBPF complements, never replaces, in-process profilers.** For language-semantic heap/goroutine/allocation detail, the in-process profiler (e.g. Go `pprof`) wins; eBPF wins on off-CPU, kernel-boundary, and zero-instrumentation correlation. Pair them; don't claim one replaces the other.

## Halt Conditions

Stop and surface rather than proceeding when:
- **The effect isn't stated measurably** (no baseline, "feels slow") — establish the observable + the USE-method resource sweep first; don't reach for a probe to go fishing.
- **No pack for the substrate** — don't refuse and don't invent one; design on the method spine + first principles and flag the missing-pack gap (reduced confidence).
- **A privileged eBPF attach/run on a shared or production node is required** — a deploy OR an ephemeral one-liner; any privileged attach is root-on-node — STOP and route to the human + security one-way-door gate; never self-approve, and never on a validator.
- **Probe overhead can't be bounded or measured on a safe target first** — don't attach on a hot/prod node; reduce to an aggregating probe or a non-prod rehearsal.
- **A kernel probe target can't be resolved** (stripped binary, missing BTF, renamed symbol) — degrade to a kernel-side tracepoint or flag the gap; don't fabricate a working probe.

## When to use / when not

| Use `/ebpf` for… | Use instead… |
|---|---|
| "Why is this slow — blocked or busy?"; off-CPU / lock-contention / latency-distribution analysis | — |
| Designing a bpftrace / CO-RE (compile-once-run-everywhere) probe or a Tetragon `TracingPolicy` as a deliverable | — |
| A rigorous performance benchmark (USE method, tail latency, flamegraphs, a reproducible harness) | — |
| Application-side OTel spans/metrics at the emit site | `opentelemetry-expert` |
| The telemetry backend, PromQL/LogQL, recording rules, dashboards | `observability-platform-engineer` |
| SLO/SLI targets, alert tier, runbooks, "is it healthy now" | `sre-engineer` |
| App/architecture-level systems quality where eBPF is just the diagnostic | `/systems` (systems-engineer) |
| NetworkPolicy intent / Istio config (vs. the Cilium eBPF datapath) | `network-specialist` |

`/ebpf` is the *discovery + measurement* surface; deploying anything privileged to a cluster crosses into a one-way-door gate the operator owns.

## The method (five steps)

Full protocol + the tool→signal→concern map live in `references/pack-perf-methodology.md`. In short:

1. **Establish the effect + USE baseline — FIRST, always.** State the observable in measurable terms (the `/root-cause` Step-1 discipline); run the USE-method sweep (Utilization/Saturation/Errors per resource) to localize the bottleneck *before* reaching for a specific probe. A probe chosen before the baseline measures the wrong resource.
2. **Load the pack.** `references/pack-perf-methodology.md` — the method, the tool→signal→concern map, the `divergences[]` (where kernel-perf rejects general wisdom + verifier-forced idioms), the overhead model, and the §lint/verify anchors. If no pack fits the substrate, design on the spine + first principles and flag the gap.
3. **Design the probe / benchmark — minimal and overhead-bounded.** Pick the smallest probe that yields the signal; prefer in-kernel aggregation over event streaming; rate-limit on hot paths; for a benchmark, specify an open-loop harness. State the literal tool, the expected output shape, and the overhead class (map-aggregated vs. drain-bound). A privileged deploy stops at the one-way-door gate.
4. **Run + attribute on retrieved signal.** Capture the verbatim output (histogram / folded stack / count); build on/off-CPU flamegraphs; attribute time to the mechanism (futex → lock, off-CPU → blocked-not-busy, bio → device tail). Pair with the in-process profiler where language-semantic detail is needed.
5. **Conclude measured + cited.** A ranked set of contributing factors with the probe evidence for each, the remaining uncertainty, and the next safe probe — never a single un-measured "root cause."

## The discipline spine

The five guardrails above are the spine; they are not negotiable under time pressure, a senior theory, or a deadline. The two that fail silently if skipped:

### Measure-don't-assume (Guardrail 1)
The highest-severity failure is a confident perf claim with no retrieved signal. When you notice yourself writing "probably", "likely blocked on", "the lock is the issue" about *system state* (not a prediction of a future measurement) — STOP and run the probe. Cite the tool call or tag the claim `unverified`.

### Overhead-bound + open-loop (Guardrails 3, 4)
A probe that perturbs the system measures a different system; a closed-loop harness measuring tail latency measures a fiction. Both are silent — the output *looks* fine. Bound overhead before attaching; use an open-loop harness for any tail metric.

## Pack-loading mechanism

The method is substrate-agnostic; the expertise is **data** in `references/pack-<domain>.md`, conforming to the pack contract (the 6-section + §anchors shape `/idiomatic`'s `language-pack-TEMPLATE.md` defines, adapted to perf/observability). **Adding a substrate = drop one conforming pack.** MVP ships `pack-perf-methodology` (the substrate-agnostic perf method, with a bpftrace section). A `pack-cilium-tetragon` (the standing Tetragon/Hubble substrate) is deferred until that substrate is built (per the eBPF capability design, PLT-649).

## References

- `references/pack-perf-methodology.md` — the perf-analysis method (USE, on/off-CPU, flamegraphs), the tool→signal→concern map, the `divergences[]` (verifier-forced idioms; eBPF-vs-pprof; the open-loop/coordinated-omission rule), the overhead model, and the §verify anchors (cite-only-from-the-table discipline). Includes the bpftrace section.
