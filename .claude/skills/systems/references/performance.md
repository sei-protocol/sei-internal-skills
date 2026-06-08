# Performance & Linux

Checklist for code that must behave well on the machine and under load. Measure first; design across the latency hierarchy; engineer for the tail, not the mean. Our-own-words rules; cite the authority (see `sources.md`).

Sections: `## checklist` · `## severity model`. Anchors: Brendan Gregg, Jeff Dean (latency numbers; Tail at Scale), Thompson/LMAX Disruptor, Acton (DOD). Companion: `observability.md` (USE/RED), `safety-quality.md` (bounded loops/buffers).

## checklist

| id | principle | rule | cue | authority |
|----|-----------|------|-----|-----------|
| PERF1 | Measure before optimizing | Don't change code for speed until a profile under *realistic* load points at the cost. Intuition about hot spots is usually wrong; let data name the target. This rule gates every micro-optimization below. | A "perf" change with no before/after profile; tuning a path the load never exercises. | Gregg: Systems Performance |
| PERF2 | Latency hierarchy | Know the orders of magnitude (L1 ns → memory ~100ns → SSD µs → network/disk ms → cross-region tens of ms) and structure data and calls around them — a remote call is ~6 orders over an L1 hit. | A hot loop crossing a network/disk boundary per iteration; treating an RPC like a function call. | Dean: Latency Numbers |
| PERF3 | Optimize hot, amortize cold | Spend effort where the program spends time; leave cold paths simple. A 1% path tuned to perfection buys nothing. | Hand-optimized startup/config code; the inner loop left naive. | Gregg: Systems Performance |
| PERF4 | Engineer for the tail | Tune for p99/p99.9, not the mean — at scale every request fans out and the slowest component sets user-visible latency. | SLOs/dashboards stated as averages; capacity sized to mean latency. | Dean & Barroso: Tail at Scale |
| PERF5 | Tail-tolerant request patterns | Cut tail latency with hedged or tied requests (a delayed backup, or two replicas with the loser cancelled) rather than chasing every slow component. | A fan-out that waits on the single slowest replica with no hedge/backup. | Dean & Barroso: Tail at Scale |
| PERF6 | Kill head-of-line blocking | Don't let one slow item stall a shared queue/connection — separate slow and fast work into independent lanes so a straggler can't serialize everything behind it. | One unbounded FIFO mixing cheap and expensive work; one connection multiplexing latency-sensitive and bulk traffic. | Dean & Barroso: Tail at Scale |
| PERF7 | USE method for resources | For each resource (CPU, memory, disk, net) check Utilization, Saturation, Errors — a structured sweep finds the bottleneck faster than guessing. | A perf investigation with no resource-level U/S/E baseline. | Gregg: USE method |
| PERF8 | Right tool per symptom | Match the instrument to the symptom: `perf`/flame graphs for on-CPU, off-CPU/eBPF for blocked time, `strace` for syscalls, `ftrace` for kernel paths, `pprof` for in-process CPU/alloc. | One favorite tool regardless of whether the time is on- or off-CPU. | Gregg: flame graphs |
| PERF9 | Bounded concurrency + backpressure | Cap in-flight work (worker pool / semaphore) and propagate backpressure so a fast producer can't exhaust memory or overwhelm a slow stage. Unbounded parallelism is a latency and OOM hazard. | A goroutine/thread per request with no ceiling; a producer with no signal to slow down. | Gregg: Systems Performance |
| PERF10 | Batch to amortize fixed cost | Group items to spread per-op fixed overhead (syscall, RPC round-trip, commit) across many records — but bound batch size and flush latency so the tail doesn't suffer. | One syscall/RPC per record in a loop; an unbounded batch that starves latency. | Gregg: Systems Performance |
| PERF11 | Allocation & GC pressure | Treat allocation as a cost: reuse buffers, pool/preallocate, keep short-lived garbage out of the hot path so the collector isn't the bottleneck. *(Advisory — profile-gated by PERF1.)* | Allocations dominating a heap/alloc profile under load; per-request churn the GC chases. | Gregg: Systems Performance |
| PERF12 | Mechanical sympathy | Lay out and access data to suit the hardware — hot fields on the same cache line, avoid false sharing of contended fields across cores, prefer pre-allocated ring buffers over per-item allocation on the hot path. *(Advisory — profile-gated.)* | Two cores hammering adjacent fields of one struct; per-message allocate in a hot loop. | Thompson / LMAX Disruptor |
| PERF13 | Data-oriented layout | Organize data by how it's processed — contiguous arrays the loop streams over (struct-of-arrays) beat pointer-chasing graphs of small objects. *(Advisory / cut-first — profile-driven.)* | A hot loop walking a linked structure of heap objects; AoS where only one field is touched. | Acton: Data-Oriented Design |
| PERF14 | io_uring at high fan-in | At high connection counts, prefer `io_uring`'s batched submission/completion over per-event `epoll`+syscall churn. *(Advisory / cut-first — platform-specific; only when the syscall rate is proven hot.)* | A C10k+ server doing a syscall per readiness event with a measured syscall bottleneck. | io_uring vs epoll (kernel) |

## severity model

- **correctness/safety** — PERF9 escalates here when unbounded concurrency can OOM or deadlock the process (a crash, not a slowdown).
- **consequence-under-load** — PERF2 (a boundary crossed in a hot loop), PERF4–PERF6 (tail behavior users feel), PERF9–PERF10 (overwhelm/starvation under burst). Bite only when traffic arrives; tie each to measured or projected load.
- **advisory** — PERF1, PERF3, PERF7, PERF8 (method/process) and PERF11–PERF14 (micro-optimizations). Profile-gated by PERF1: don't flag without a profile pointing at the cost. Never lead with a micro-opt over a tail or backpressure finding. Cut-first for MVP: PERF13, PERF14, and the false-sharing detail of PERF12.
