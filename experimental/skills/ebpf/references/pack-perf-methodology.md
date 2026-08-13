# Perf-methodology pack (substrate-agnostic) — for /ebpf

> The substrate-agnostic method + the bpftrace/bcc tool checklist. Conforms to the `/idiomatic` `language-pack-TEMPLATE.md` shape (dimensions → authorities → divergences → anti-patterns → overlays → severity, + a §7 verify-anchor table), adapted from "language idiom" to "perf method." Authorities are in §2; cite them in findings. Tool flags vary across bcc/bpftrace versions — verify against the man pages on the target before locking a flag.

## 1. dimensions[] — the method

| id | dimension | rule | cue | authority |
|----|-----------|------|-----|-----------|
| M1 | Establish the effect first | State the observable in measurable terms (signal, onset, baseline, blast radius) before any probe. No baseline → no investigation, only a feeling. | "feels slow"; a probe chosen before a baseline | `/root-cause` Step 1; USE method |
| M2 | USE method (run first) | For every resource (CPU, mem, disk, NIC, locks): check **U**tilization, **S**aturation, **E**rrors — errors first (cheapest). Converts unknowns into a question list, not "whatever tool is installed." | jumping to one tool; no resource sweep | Gregg, USE method |
| M3 | On-CPU vs off-CPU | On-CPU (sampled stacks) answers "where are cycles burned"; off-CPU answers "where/why blocked." Together = 100% of thread time. A slow-but-idle system is *blocked, not busy* → off-CPU first. | high latency + low CPU% treated as "not the CPU, so unknown" | Gregg, Off-CPU Analysis |
| M4 | Distributions, not averages | Power-of-2 latency histograms expose multimodality + the p99/p999 tail; scraped averages/quantiles smear it. | reasoning from a mean; a single p99 number with no shape | Gregg, BPF tools |
| M5 | Retrieved, not extrapolated | Every cited signal = the literal probe + verbatim output. "The logs/stacks probably show…" is fabrication. | a perf claim with no probe output attached | `/root-cause` Rule 3 |
| M6 | Comment discipline *(required)* | An eBPF/bpftrace program is terse; comments are a rare exception. A comment earns its place only for a non-obvious *why* — and the **verifier-forced idioms** (per-CPU map scratch for the 512B stack, a bounded loop, `__always_inline`) are exactly that: they look wrong, so one line says why. **No history/changelog in the program** ("was kprobe, moved to fentry"); **no tombstone** for a removed probe — that lives in the PR/commit. Present state only. | `// what` restating the next line; "we used to attach…"; a removed-probe tombstone; commented-out probe variants | `/idiomatic` comment-discipline (PLT-626); the pack `divergences[]` below |

## 2. authorities[]

- **Gregg — USE method** — https://www.brendangregg.com/usemethod.html (+ Linux checklist /USEmethod/use-linux.html) — the resource sweep + procedure.
- **Gregg — Off-CPU Analysis** — https://www.brendangregg.com/offcpuanalysis.html — on/off-CPU, wakeup analysis, the overhead caveat.
- **Gregg — Flame Graphs** — https://www.brendangregg.com/flamegraphs.html (CPU / off-CPU / hot-cold) — the visualization.
- **Gregg — "CPU Utilization is Wrong"** — https://www.brendangregg.com/blog/2017-05-09/cpu-utilization-is-wrong.html — IPC/stall-cycles; CPU% is non-idle time, not work.
- **Gregg — eBPF tools** + **BPF Performance Tools** (book) — https://www.brendangregg.com/ebpf.html — the tool catalog + one-liners.
- **bcc** — https://github.com/iovisor/bcc ; **bpftrace** — https://github.com/bpftrace/bpftrace (tools/README.md) — the tools + man pages (authoritative for flags per version).
- **Coordinated omission** — Gil Tene's framing (open-loop vs closed-loop tail measurement) — the harness rule in §3.

## 3. divergences[] — where kernel-perf rejects general wisdom (the load-bearing section)

- **"High CPU% means CPU-bound."** No — CPU% is *non-idle time*; a thread stalled on memory counts as "busy." Only IPC / stall-cycle PMCs distinguish retiring from stalled. → Don't conclude compute-bound from CPU%; check IPC (`< 1.0` ≈ memory-bound).
- **"A closed-loop load test measures the tail."** No — if the generator backs off when the system stalls, measured p99 *understates* the real tail by the stall (coordinated omission), which is catastrophic for a latency-cliff investigation. → Any tail-latency benchmark uses an **open-loop / rate-controlled** harness (constant arrival, measure queueing), or explicitly corrects for CO.
- **"eBPF replaces the in-process profiler."** No — for language-semantic heap/goroutine/allocation detail, `pprof` (etc.) wins; eBPF wins off-CPU + kernel-boundary + zero-instrumentation. → Pair them; for Go, kernel off-CPU stacks show *thread* (futex/netpoll) blocking, not goroutine identity — cross-check with `pprof` block/mutex profiles.
- **"This eBPF code is over-engineered."** Verifier-forced idioms are not smells: the **512-byte stack** forces a per-CPU-array map for any non-trivial scratch struct; **bounded loops** (or `bpf_loop`) are required (no unbounded loops); `__always_inline` / map-of-maps are normal. → Don't flag verifier-mandated structure as over-engineering (the 1M-instruction + 512B-stack + bounded-loop limits are the constraint).
- **"Attach a uprobe to the function."** On a stripped/optimized Go binary, uprobes are fragile (moved goroutine stacks, non-standard calling convention, missing symbols; uretprobes can corrupt the Go runtime). → Prefer kernel-side **tracepoints** (futex/block/sched/tcp) keyed by PID/cgroup; reserve uprobes for where language-semantic attribution is essential and symbols exist.
- **"Aggregation is premature optimization."** Overhead = per-event cost × frequency. An event-streaming probe is **drain-bound** (ringbuf egress dominates); a map-aggregated probe is **event-rate-bound** (no egress). → On a hot path, aggregate in-kernel (histogram/count maps), read the map periodically — this is *why* `biolatency`/`runqlat` are cheap despite firing on every I/O/wakeup.

## 4. anti_patterns[]

- **Guess-don't-measure** — cue: "probably blocked on the lock" with no probe. Rewrite: run `offcputime`/futex tracing; show the stack.
- **Closed-loop tail benchmark** — cue: a load generator that waits for a response before sending the next (backs off on stall). Rewrite: open-loop / rate-controlled harness.
- **Unbounded hot-path probe** — cue: a high-frequency kprobe (per-packet/per-context-switch) streaming every event to userspace. Rewrite: in-kernel aggregation + rate-limit; prefer a stable tracepoint over a kprobe.
- **uprobe on a hot Go function** — cue: a uretprobe/uprobe on a stripped Go binary's hot path. Rewrite: kernel tracepoint keyed by PID/cgroup; pair with `pprof`.
- **CPU%-as-saturation** — cue: concluding CPU-bound from cAdvisor CPU%. Rewrite: `runqlat` for run-queue *latency* (saturation); IPC for memory-stall.
- **Average-not-distribution** — cue: a perf claim from a mean/single quantile. Rewrite: a power-of-2 histogram (`*latency`/`*dist` tools).

## 5. tool → signal → concern map (the core checklist)

Cite a tool only from this table or §7; verify flags on the target. bcc tools are often `-bpfcc`-suffixed; most have a bpftrace `.bt` sibling.

| Concern | Tool(s) | Signal | What it catches |
|---|---|---|---|
| Lock / mutex contention | `offcputime` (futex-bottomed stacks), futex tracing / `futexctn`; `klockstat` *(kernel locks only — a userspace Go `sync.Mutex` slow-paths to a `futex`, so use offcputime/futexctn for it, NOT klockstat)* | off-CPU flame graph; per-futex wait histogram | time *blocked on a lock* vs. *working* — the contention no metric names |
| Scheduler / CPU saturation | `runqlat`, `runqlen` | run-queue latency histogram | runnable-but-not-running (noisy-neighbor / oversubscription); multimodal ⇒ throttle/saturation |
| On-CPU hot path | `profile` (49 Hz) → flame graph; PMC `perf stat` (IPC) | CPU flame graph; IPC | where cycles go; IPC < 1.0 ⇒ memory-stalled not compute |
| Off-CPU (blocked) | `offcputime` | off-CPU flame graph | the master "blocked not busy" tool — disk/net/lock/page-fault/preempt |
| Block-I/O latency | `biolatency`, `biosnoop`, `biotop`, `bitesize` | bio latency distribution + tail; per-I/O attribution | device tail (incl. network-storage round-trip on EBS); foreground vs. background I/O |
| fsync / FS latency | `funclatency` on fsync, `ext4slower`/`xfsslower`, `fileslower` | fsync-syscall latency distribution | the *syscall* latency that gates a commit (journaling/writeback) — distinct from raw `bio` |
| Syscall latency | `syscount`, `funclatency` | dominant syscall + its latency spread | which syscall dominates; per-call distribution |
| TCP / network | `tcpretrans`, `tcpconnlat`, `tcplife`, `tcptop` | retransmits/RTT per peer; connect latency; churn | packet loss / congestion / peer flapping on (even encrypted) sockets |
| Memory / cache | `cachestat`, page-fault tracing, PMC | cache hit ratio; major-fault rate; stall cycles | page-cache spillover; memory-stall vs. compute |

**What kernel signals show that pod/Prometheus metrics structurally cannot:** off-CPU (blocked) time + stacks; lock/futex contention; latency *distributions/tails* (not 15–60s averages); run-queue latency; per-thread blocking; causal wakeup chains; per-syscall latency; the CPU%-is-misleading (IPC) correction.

## 6. severity / overhead model

- **correctness (act on these):** a measured saturation/contention/tail with a retrieved signal; a coordinated-omission-corrected tail.
- **divergence-with-consequence:** an unbounded hot-path probe (perturbs the measurement); a uprobe on a stripped hot Go path (fragile/unsafe); a closed-loop tail harness (wrong number).
- **style:** tool-flag nits; histogram-bucketing taste.

**Overhead model:** the map-aggregated-vs-event-streaming model lives in §3 (the "Aggregation is premature optimization" divergence) — map-aggregated ⇒ ~ event rate; event-streaming ⇒ ~ rate × egress, drain-bound. The verifier bounds memory + termination (1M-insn complexity, 512B stack, bounded loops) — **never** overhead. Measure on a non-prod target before a hot/prod attach.

## 7. verify_anchors[] — the checkable tools (cite ONLY from here)

**Cite a tool/flag only from this table — never assert one from memory.** Mark genuinely judgment-only calls (which methodology to apply, how to read a flame graph) as judgment-only; record version caveats (eBPF behavior is kernel- and tool-version-dependent — encode as a provenance caveat, not "currently").

| dimension / concern | anchor(s) | catalog | caveat |
|---|---|---|---|
| off-CPU / lock | `offcputime`, `klockstat`, futex tracing | bcc + bpftrace | `futexctn` is a **newer libbpf** tool — version-dependent; on older bcc derive futex contention from `offcputime` futex-bottomed stacks. off-CPU tracing overhead scales with context-switch rate — short windows / aggregating tools on hot nodes |
| scheduler | `runqlat`, `runqlen` | bcc + bpftrace | high-resolution per-wakeup probe — bounded windows on a busy node |
| on-CPU | `profile`; `perf stat` (IPC) | bcc; perf | IPC needs PMCs (often unavailable in VMs/containers — verify on target) |
| block-I/O | `biolatency`, `biosnoop`, `biotop`, `bitesize` | bcc + bpftrace | on network storage (EBS) `biolatency` includes the network round-trip — a feature for prod-realistic numbers, not portable to local SSD |
| fs / fsync | `ext4slower`/`xfsslower`/`fileslower`, `funclatency` | bcc | fs-specific tool must match the on-disk filesystem |
| syscall | `syscount`, `funclatency` | bcc | — |
| TCP | `tcpretrans`, `tcpconnlat`, `tcplife`, `tcptop` | bcc + bpftrace | sees TCP-layer health, NOT protocol-semantic peer health |
| memory | `cachestat`, PMC | bcc; perf | **`cachestat` accuracy drifts across kernel versions** — validate on target before relying on the hit ratio |
| CO / tail harness | (no tool) | — | **judgment-only** — open-loop harness design is a method call, not a checkable tool; cite the coordinated-omission authority (§2) |
| which method to apply / flame-graph reading | (no tool) | — | **judgment-only** — cite the §2 method authority; never fabricate a tool that "decides" the method |

**Genuinely judgment-only — never fabricate a tool:** choosing the method (USE vs. off-CPU vs. workload-characterization); reading a flame graph; the open-loop/CO harness decision; whether a measured signal is the *cause* vs. a correlate. Cite the §2 authority and say plainly there is no checkable tool.
