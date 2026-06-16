# eBPF / Kernel-Observability Expertise — Technical Strategy (Phase-1 Design)

**Status:** DRAFT (revised post-cross-review) for design-approval. Workstream: eBPF kernel-observability capability. Scope tier: System. **Design-approval authorizes Phase-2 MVP only (the `/ebpf` skill + a Mode-B one-shot probe Job + the P0-1 PoC — no cluster-standing components). It does NOT authorize the deferred items (the standing Tetragon DaemonSet, enforcement, the dedicated agent), each of which carries its own one-way-door gate + security review when un-deferred (§6, §7).**

## 1. Thesis

Add a new agentic capability to the mesh: **deep eBPF / kernel-tooling expertise** that lets the Coral team see and benchmark what pod-level and Prometheus telemetry structurally cannot — off-CPU/blocked time, lock/futex contention, syscall and block-I/O latency *distributions*, scheduler run-queue latency, and TCP-level health on encrypted p2p. The **destination** is a full capability serving two modes:

- **Mode A — long-lived observability strategy:** a standing kernel-event substrate (Tetragon + Hubble on the Cilium datapath already present) feeding the existing observability plane. **Deferred** (§7) — un-defers when a continuous cluster-wide kernel signal has a named consumer.
- **Mode B — ad-hoc, mid-workstream instrumentation:** during a benchmarking or root-cause workstream, stand up the *specific* kernel probe a scenario needs (a `bpftrace`/`perf` one-shot), observe, tear it down — scoped, bounded, audited. **This is the Phase-2 MVP.**

Centered on **Cilium-CNI-on-EKS** (the real platform — the `harbor` cluster), with a portable CO-RE (Compile-Once-Run-Everywhere) toolchain as a secondary path for dedicated benchmark rigs. Cross-review re-scoped Phase-1 to the **leanest slice that proves the capability**: the expertise as a skill + a self-contained ad-hoc probe + a single PoC that reproduces a *known root-caused incident* (PLT-537). The dedicated agent, the standing substrate, and the second PoC are deferred with named triggers — this keeps the biggest one-way door (a standing privileged node agent) off the critical path.

## 2. Why kernel-level, why now (the evidence)

Kernel-level instrumentation earns its place now because the PLT-537 incident is the template: a **synchronous memIAVL commit held `cs.mtx`** (the CometBFT consensus mutex), serialized through the single `processStateCh` drain goroutine, producing a bistable latency cliff — diagnosable from code + a stack dump, but **named by no Prometheus metric**. That class (a CPU/IO-bound critical section under a lock gating a serial drain) is exactly what off-CPU + futex profiling surfaces.

What kernel signals show that the existing seid Prometheus + cAdvisor + Thanos stack structurally cannot:
1. **Lock-wait vs. work time** — futex/off-CPU attribution (the PLT-537 signal). *Caveat for the pack:* Go's `sync.Mutex` spins before parking on a `futex`, so off-CPU/futex *under-attributes* short, spin-dominated contention; PLT-537's long held critical section is the favorable, cleanly-surfaced case.
2. **I/O latency *distribution*, not throughput** — split **`fsync`-syscall latency** (what gates `cs.mtx`; includes journaling/writeback) from **`bio` device latency** (the EBS gp3 network-storage tail). Both matter; they answer different questions.
3. **Run-queue latency** — runnable-but-not-scheduled (noisy-neighbor on shared nodes), distinct from CFS throttle (which the no-CPU-limits convention avoids — making runqlat the *only* CPU-starvation signal left).
4. **Per-goroutine/per-thread blocking** — the trap lived in *one* goroutine; aggregates hide it.
5. **TCP-level p2p health on encrypted custom-binary traffic** (26656, MConnection/ChaCha20) — opaque to Istio/L7; eBPF socket probes are the only in-cluster view. (Scope: TCP-layer health — retransmits/RTT/queue depth — *not* CometBFT-semantic peer health, which lives in seid RPC `/net_info`.)
6. **OCC (optimistic-concurrency-control) effectiveness** — whether parallel EVM workers parallelize vs. conflict-abort thrash.

**Honest boundary:** seid's own `pprof` wins on Go-semantic heap/goroutine detail; eBPF wins on off-CPU, kernel-boundary, and zero-instrumentation correlation. Complementary — never eBPF-replaces-pprof.

## 3. The capability shape (Phase-2 MVP vs. deferred)

### 3.1 Expertise — the `/ebpf` skill, composed by `systems-engineer` (MVP); dedicated agent deferred

**MVP:** ship the **`/ebpf` skill** (technique-with-spine, the `/idiomatic`/`/systems` precedent) and have the **existing `systems-engineer`** compose it — exactly as it already composes `/idiomatic` and `/systems`, and as its own description already claims `eBPF`/`bpftrace`/USE-method/off-CPU/tail-latency fluency. No new persona at MVP.

**Deferred — a dedicated `ebpf-specialist` agent.** Un-defers when there are **durable, shipped eBPF artifacts to own** (a maintained `TracingPolicy` library, in-repo CO-RE programs) — i.e. when Mode-A ships and "who owns this verifier-safe BPF program" recurs. The boundary then carved is: `systems-engineer` *uses* eBPF as a diagnostic on code it owns; `ebpf-specialist` *authors eBPF programs / TracingPolicies / benchmark harnesses as durable artifacts*. (The full roster-boundary fence — vs. opentelemetry-expert, observability-platform-engineer, sre-engineer, network-specialist, security-specialist, k8s-capacity-management, sei-network-specialist — is captured in the deferred-agent issue, not built now.)

### 3.2 The skill — `/ebpf` (technique-with-spine), one pack at MVP

A thin **method spine** (the observability + perf methodology that never changes; profile-first, cite-every-finding, degrade-don't-refuse) + expertise as **data in pluggable packs** conforming to the existing `language-pack-TEMPLATE.md` (6 sections + §7 lint-anchors, mandatory comment-discipline dimension, `divergences[]` as the load-bearing section).

- **MVP pack — `pack-perf-methodology`** (with a `bpftrace` section folded in): USE method, on/off-CPU + flamegraphs, the tool→signal→concern map, the "what kernel shows that Prometheus can't" list, and **statistically valid benchmarking** — explicitly the **open-loop / coordinated-omission-resistant** harness rule (a closed-loop generator that backs off when the system stalls *understates* the p99 by the stall, which is catastrophic for the very latency-cliff P0-1 reproduces). Plus the **overhead model** as a `divergence`: map-aggregated probes (overhead ~ event rate) vs. event-streaming probes (overhead ~ rate × egress, drain-bound); and the verifier-forced idioms (512B stack → per-CPU map scratch; bounded loops) as `divergences[]` not smells.
- **Deferred pack — `pack-cilium-tetragon`** — Hubble (L7 opt-in via Envoy), Tetragon `TracingPolicy`, monitor→enforce. Un-defers **with** the standing DaemonSet (§7); none of it is exercised by the Mode-B Job or P0-1.

Authored via `/author-skill` (guardrails-first, ≥3 refusals; RED-GREEN-REFACTOR; evals ≥ happy+halt, target 3) and audited via `/audit-skill` against the conventions-catalog.

### 3.3 Mode-B ad-hoc probe — a one-shot privileged Job (MVP); standing substrate deferred

**MVP mechanism:** a **one-shot privileged Job** pinned to the target node (`nodeName`/nodeSelector), in the dedicated privileged namespace, **`hostPID: true`** so it observes seid by host PID/cgroup *without any SeiNode pod-spec change* (this is the load-bearing attach mechanism — not `ShareProcessNamespace`, which is incidental). Runs `bpftrace`/`perf`, resolves seid by binary/comm (PID changes per restart — re-resolve, don't cache), uploads artifacts to S3, and self-reaps via `activeDeadlineSeconds` + `ttlSecondsAfterFinished`. A reaped Job is a far smaller security ask than a permanent node agent.

**Deferred:** the standing **Tetragon DaemonSet** + the `TracingPolicyNamespaced` path (which requires the DaemonSet) + Hubble — all Mode-A. `kubectl debug --profile=sysadmin` is **live-triage only and is NOT a routine instrumentation path** — it leaves no audit trail, so if used it requires an *enforced* K8s API-audit-policy trail (capturing node-debug/ephemeral-containers), not a convention.

## 4. Deployment architecture (Cilium-on-EKS / harbor) — Phase-2 MVP

- **Substrate:** Cilium is already the CNI (kube-proxy replacement, eBPF socket-LB) → the eBPF datapath, bpffs, and BTF (BPF Type Format — the kernel metadata CO-RE relocates against) are present. The MVP probe Job attaches via kprobe/tracepoint (futex/bio/sched/tcp) — a different surface from Cilium's tc/XDP, no contention (pin into our own bpffs subdir, never `tc/globals`).
- **Privilege & PSS (load-bearing security boundary):** the probe Job runs in a **dedicated `kernel-observability` namespace labeled `pod-security.kubernetes.io/enforce: privileged`** (warn/audit at baseline). **Never relax PSS on harbor's existing namespaces — `sei-k8s-controller`, `autobake`, `monitoring`, `eng-*`.** Capability ladder: prefer `CAP_BPF`+`CAP_PERFMON`(+`CAP_SYS_RESOURCE`); ship `privileged: true` to land, file an issue to ratchet down. **The Job stays in the privileged namespace even after the cap ratchet** (hostPID + bpffs mounts still breach baseline/restricted) — the ratchet narrows caps, it does not graduate the workload out of the privileged namespace.
- **Security constraints (hard, not caveats):**
  - **Image integrity:** the probe image is **digest-pinned and cosign-signature-verified** via admission policy on the privileged namespace — a root-on-node workload is the highest-value supply-chain target.
  - **Egress containment:** a **CiliumNetworkPolicy egress allowlist** restricts the Job to {S3 endpoint, metrics-scrape ingress} — a privileged kernel-reader with open egress is a turnkey exfiltration channel; on a Cilium cluster this is cheap.
  - **Data capture:** any captured argv/flow data is treated **sensitive-by-default** — the S3 artifact bucket is **SSE-KMS encrypted, least-privilege write-only scoped** to the Job's identity; a "no secrets in captured data" check is part of the PoC. (Hubble-L7 / Tetragon-argv capture with Redact-on-by-default is a *deferred-pack* constraint, gated with Mode-A.)
- **Identity:** harbor uses **EKS Pod Identity, not IRSA**. Ad-hoc artifact upload uses the engineer's namespace-scoped `engineer-service-account` (per-engineer scoped IAM). **Note:** acknowledge that a root-on-node workload can read *other* pods' Pod Identity credentials on its node — so egress containment + image integrity (above), not IAM scope, are the real mitigations. **CPU requests, no CPU limits** (a throttled eBPF probe drops events and skews the benchmark).
- **Enforcement:** **out of scope for Phase-1/2 entirely.** The PoC is pure observation. Enforcement (kprobe-override kill) is a slashing-grade availability/integrity risk on consensus nodes; if ever un-deferred it gets its own default-off gate, per-policy time-bounded, **excluded from validators**, and carries the error-injection-allowlist + TOCTOU caveats.

## 5. The Sei performance-benchmark PoC — P0-1 only (MVP)

Runs on harbor against a **single full node on `atlantic-2`** (the testbed — **not** pacific-1 mainnet, and **not** a validator: a synthetic-write sweep under probe overhead risks missed-block/double-sign exposure on consensus-critical signing paths).

**P0-1 — State-commit latency under write load (the PLT-537 reproduction):** sweep `async_commit_buffer` (default 100; the sweep toward 0 reintroduces back-pressure into the locked critical section). **Before relying on this as the reproduction, cite the PLT-537 RCA to confirm `async_commit_buffer=0` restores the specific synchronous-commit-holds-`cs.mtx` path** (vs. merely throttling the buffer; memIAVL commit synchronicity may involve additional knobs) — or reproduce via the exact pre-fix config the RCA names. This is the credibility hinge of the whole PoC. Observe with the **open-loop/CO-resistant harness**: `fsync`-syscall + `bio` (biolatency), futex on `cs.mtx`, off-CPU on `processStateCh`, runqlat. Mark seid restart boundaries in the run artifact (PID re-resolution).

**Deferred — P0-2 (EVM-heavy vs transfer-heavy, OCC/`concurrency_workers`):** it's *exploration*, not *validation* — un-defers immediately once P0-1 validates the tool. When un-deferred, **pair the eBPF per-worker contention with seid's OCC abort/conflict metrics** (or pprof) so EVM-state hotspots aren't mis-attributed as scheduler inefficiency.

## 6. One-way doors & open questions

**What design-approval authorizes:** Phase-2 MVP (§7.1) — the `/ebpf` skill + the Mode-B one-shot probe Job + P0-1 on atlantic-2. **What it does NOT authorize** (each needs its own gate when un-deferred): the standing Tetragon DaemonSet, enforcement, the dedicated agent, the cilium-tetragon pack, Mode-A.

**One-way doors (explicit human approval; never review-gate-discharged) — most are deferred off the MVP path:**
- **MVP:** the **dedicated privileged `kernel-observability` namespace + PSS exception** + the **reaped privileged probe Job** — a real but bounded trust boundary (transient, not standing). Security-specialist review + your sign-off, with image-signing + egress-allowlist + KMS as preconditions.
- **Deferred (with their features):** the **standing privileged node DaemonSet**; **enforcement (kprobe-override kill)** as a *separate, default-off* gate excluded from validators; **cluster-scoped `TracingPolicy` RBAC authority** (who may apply a policy the standing agent executes cluster-wide); any **`ShareProcessNamespace` SeiNode pod-spec change** (avoided entirely by the hostPID node-Job model — flagged so it stays avoided); any **persisted CRD/S3 schema**.

**Open questions to verify before MVP build (research-flagged unverified):**
- **Node AMI / kernel / BTF presence on harbor** (AL2023 vs Bottlerocket; `/sys/kernel/btf/vmlinux`) → CO-RE vs BTFHub fallback. Check `sei-protocol/platform` `clusters/harbor/` `EC2NodeClass` or `kubectl get nodes -o wide`.
- **Karpenter taint tolerations** — the NodePool is tainted `CriticalAddonsOnly=true:NoSchedule`; the probe Job must tolerate whatever the benchmark node carries or it won't schedule on the node that matters.
- **PodMonitor CRD presence** in `monitoring` (operator vs. scrape-config) — for any metrics path.
- **`engineer-service-account` GetObject grant** — the Mode-B "observe-then-render" round-trip needs it; the authoritative policy doc lists Put+List only. Confirm before relying on in-cluster artifact fetch.
- **Quantified eBPF/probe overhead** — measure on-target; do not cite a percentage as fact.
- **Governance conflict:** the Tide LLD assumes VPC CNI/Calico; harbor is Cilium — the capability targets harbor's Cilium reality.

## 7. Phasing & MVP

- **Phase 1 (this workstream):** the design (this doc) + the Linear definition (filed on approval — §8). **Gated on your design-approval.**
- **Phase 2 — MVP (post-approval), in order:**
  1. **The verification spike** (the §6 open questions — BTF/AMI, Karpenter taint, PodMonitor, GetObject). *Blocks 2–3.*
  2. **The `/ebpf` skill** — `pack-perf-methodology` (bpftrace folded), authored via `/author-skill`, audited via `/audit-skill`; composed by `systems-engineer`. *Zero cluster risk — ships first/parallel to the spike.*
  3. **The Mode-B one-shot probe Job** — the `kernel-observability` namespace + the digest-pinned/signed/egress-locked privileged Job (one-way-door gated; security review).
  4. **P0-1** — the PLT-537 reproduction on atlantic-2, open-loop harness.
- **Deferred, with named un-defer triggers:** the dedicated `ebpf-specialist` agent (durable BPF artifacts to own); the standing Tetragon DaemonSet + `pack-cilium-tetragon` + Hubble L7 + Mode-A (a named continuous-signal consumer); enforcement (a concrete use-case + owner + rollback runbook); P0-2 (immediately, once P0-1 validates); the portable-rigs CO-RE path (a second rig with a real need).

## 8. Linear initiative (Sei Agentic Mesh project)

Parent issue (the capability + this design) + children: (a) the verification spike; (b) the `/ebpf` skill + `pack-perf-methodology`; (c) the Mode-B probe-Job deployment pattern (one-way-door gated); (d) P0-1 PoC. Plus deferred-tracking issues (dedicated agent; standing DaemonSet + cilium-tetragon pack; P0-2) carrying their un-defer triggers so the full-capability destination stays discoverable. Exact filing on design-approval.

## 9. Build-time refinements surfaced in review (carry into the Phase-2 issues — not design-approval blockers)

These are implementation-spec details the cross-review surfaced; they bind the §8 issues / the verification spike, not this approval:
- **Memlock / perf-buffer budget** — when the standing Tetragon DaemonSet un-defers (Mode-A), enumerate the shared kernel memlock + perf-buffer budget vs. Cilium as a named precondition on that gate (not rediscovered at deploy). (MVP one-shot Job doesn't meaningfully contend.)
- **S3 egress** — the Cilium egress allowlist must constrain the *destination bucket*, not just the regional S3 FQDN (a shared regional endpoint still permits an attacker bucket); pair with a VPC gateway-endpoint / bucket-resource condition.
- **Image integrity** — cosign verification must assert a *pinned key / Fulcio identity* (issuer+subject), not mere signature-presence.
- **S3 read path** — the Mode-B observe-then-render round-trip reads under the engineer's *interactive* identity, NOT by widening the Job's write-only role (keeps the exfil read-path closed); resolve with the §6 GetObject open question.
- **futex-on-`cs.mtx` resolution** — on the stripped seid Go binary, the named-mutex futex probe may need a BTF/DWARF or uprobe fallback; the off-CPU-on-`processStateCh` + biolatency signals stand on their own if it does.
- **Probe lifetime vs. upload** — the one-shot Job's `activeDeadlineSeconds` must budget for the S3 flush (or flush incrementally) so a deadline-kill mid-capture doesn't lose the artifact.
- **Metrics path** — a transient Job is a poor `PodMonitor` target; the spike may resolve the metrics path to push-to-artifact rather than a scrape.

## References

Synthesized from five cited research streams + a six-reviewer blinded cross-review (systems-engineer, platform-engineer, security-specialist, sei-network-specialist, product-manager, prose-steward). Key external anchors: Cilium/Tetragon docs; Brendan Gregg (USE method, off-CPU analysis, BPF Performance Tools); libbpf/CO-RE (Nakryiko); the verifier (kernel.org/LWN). Internal: PLT-537 RCA, `sei-config` (`async_commit_buffer`), `sei-k8s-controller` (`ShareProcessNamespace`, seid security context, the bash-`exec`-seid entrypoint), `harbor-dev` (harbor cluster + bench workflow). Per-claim citations live in the research records.
