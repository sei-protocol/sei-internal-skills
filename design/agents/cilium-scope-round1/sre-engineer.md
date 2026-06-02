# Round 1 input — sre-engineer

Cilium on harbor sits on the "if this is unhealthy, the cluster is unhealthy" tier — kube-proxy replacement, BPF masquerade, cluster-pool IPAM, CNI exclusive. When the agent is sick, Pods can't IP, Services don't NAT, NetworkPolicy stops enforcing, and Hubble visibility collapses *exactly when I need it*. PR sei-protocol/platform#772 gave us a starting alert set and a dashboard, but it's stitched together from Cilium's published mixin and our intuitions, not from a lived understanding of which signals predict user impact. I want a Cilium expert who can tell me what each metric *means in the BPF datapath* and which ones earn a page.

## Scenarios I'd dispatch to a Cilium expert

### Scenario 1: Cilium agent ENOMEM / OOMKilled, BPF map saturation cascade
The ENOMEM incident we lived through. Agent crashlooped on a few nodes, BPF maps (`cilium_lxc`, `cilium_ct_*`, `cilium_lb*`) were near/at the configured size, and we had no leading indicator — only the lagging `cilium_agent_up == 0` page. I need to know: what's the right pre-saturation alert? Is it `cilium_bpf_map_pressure` (when does that exist; threshold curve), or per-map fill ratio via `cilium_bpf_map_ops_total` + size? What's the sane page threshold vs. ticket threshold given that map resize requires agent restart and a tunable bump? At the time we *also* didn't know whether the OOMKill was the BPF maps themselves (locked memory) or the userspace agent heap — those are different fixes (raise map size + memory limit vs. raise memory limit + GC tuning).

### Scenario 2: ConnTrack table pressure under high-fanout workloads
Harbor runs RPC nodes with thousands of short-lived inbound connections plus Karpenter churn. `cilium_ct_*` (or `cilium_bpf_map_pressure{map_name=~"cilium_ct_.*"}`) climbing toward eviction territory degrades NAT silently — connections get dropped and the user sees latency tails, not a clean 5xx. I need an SLI tied to user-visible NAT drops (Hubble flow `verdict=DROPPED reason=CT_*` rate) and a page threshold that fires *before* the table starts evicting healthy entries. Today this is a dashboard-only signal in #772 and I can't defend that tier without a Cilium expert telling me what eviction actually looks like at the datapath.

### Scenario 3: kube-proxy replacement / socket-LB regressions
We run `kubeProxyReplacement: "true"` and the EKS kube-proxy addon coexists transitionally. When a Cilium upgrade ships a socket-LB or DSR regression, Services partially break — some Pods see backends, others don't, and `kubectl get svc` looks fine. I need: which metrics distinguish "service program failed to install" from "agent up but BPF service map drift" (`cilium_services_events_total`, `cilium_bpf_map_ops_total{map_name="cilium_lb*", operation="update", outcome="fail"}`)? Page-tier? And critically — what's the runbook decision tree between "roll back Cilium chart" vs. "re-enable kube-proxy addon as bypass" vs. "delete-and-recreate the affected Services"? This is a one-way-door operational question I can't answer without expertise.

### Scenario 4: NetworkPolicy enforcement drift (silent allow / silent deny)
NetworkPolicy is supposed to be deterministic, but a sick agent on one node can either fail-open (policy not loaded, all traffic allowed) or fail-closed (identity allocation stuck, everything dropped). Both are security/availability incidents and our current alerts don't distinguish them. I need: which signals indicate identity allocation health (`cilium_identity` count, `cilium_ipcache_errors_total`), policy import failures (`cilium_policy_import_errors_total`), and endpoint regeneration stalls (`cilium_endpoint_regeneration_*` histogram tail)? Where does each sit on page/ticket/silent? And what's the SLI — "% of nodes with policy in sync" or something more user-facing?

### Scenario 5: Hubble pipeline back-pressure / observability blindness
Hubble Relay is our flow telemetry source of truth for "did Cilium actually drop this packet." When Hubble itself is unhealthy (`hubble_flows_processed_total` drops to zero, `hubble_lost_events_total` climbs, Relay disconnects from peers), our network drop SLI silently goes blind. Detection-of-detection failure. I need an alert on Hubble pipeline health that pages independently of the agent's main health metric, plus a runbook for "Hubble is down — what flow-source do we fall back to before deciding NetworkPolicy is the cause of customer X's 5xx?"

### Scenario 6: IPAM exhaustion / cluster-pool PodCIDR slice starvation
We're on `cluster-pool` with `clusterPoolIPv4MaskSize: 26` (64 IPs/node) over `100.64.0.0/14`. Karpenter churn + .24xlarge nodes will eventually deplete either the per-node slice (Pods Pending on `FailedScheduling: no IPs available`) or the pool itself. The chart comment says bumping the mask is safe-going-forward but doesn't backfill old nodes. I need a leading-indicator SLI (% of pool allocated, distribution of per-node slice utilization) and a page threshold that gives enough lead time to roll a NodePool refresh.

### Scenario 7: Cilium upgrade / Helm reconcile safety
Flux reconciles the chart on a 5m interval with `rollOutCiliumPods: true`. A bad-values commit or a chart-version bump can roll the agent fleet faster than I can catch it. I need a runbook with named pre-flight checks (canary one node, watch endpoint regeneration latency, watch `cilium status` parity across nodes) and a clear abort criterion. Not a generic "Helm upgrade went bad" runbook — a Cilium-specific one that knows `cilium-dbg` and the BPF map persistence guarantees across restarts.

## Required depth per scenario

### Scenario 1 (BPF map saturation / ENOMEM)
- **Knowledge area:** BPF map types and sizing (`bpf-ct-global-*`, `bpf-nat-global`, `bpf-lb-map-max`, `bpf-policy-map-max`), locked-memory accounting (`RLIMIT_MEMLOCK` vs. cgroup memory), the relationship between map size config and agent RSS at steady state, `cilium_bpf_map_pressure` semantics across versions (1.13+ vs. 1.16).
- **Depth bar:** Can defend a specific page threshold (e.g., "page at 80% pressure sustained 10m, ticket at 70%") with reasoning grounded in eviction behavior, not vibes. Can size BPF maps from observed `cilium_endpoint`, `cilium_ct_entries`, `cilium_services_events_total` for harbor's actual workload.
- **Authoritative sources:** Cilium docs §"BPF and XDP Reference Guide" / "Resource Management"; `cilium-agent --help` for map size flags; Cilium 1.16 release notes; `Documentation/operations/troubleshooting.rst` in the cilium/cilium repo; Isovalent's BPF-map-pressure blog posts.

### Scenario 2 (ConnTrack pressure)
- **Knowledge area:** CT GC interval (`--conntrack-gc-interval`), `bpf-ct-timeout-*` tunables, the difference between "table full → reject new" vs. "evict LRU → silent drop of established," Hubble's `verdict=DROPPED reason=CT_*` and `reason=POLICY_DENIED` taxonomy.
- **Depth bar:** Can author a Hubble-flow-derived SLI ("network NAT availability ≥ 99.9% / 7d, measured as 1 - rate(hubble_drop_total{reason=~'CT_.*'}) / rate(hubble_flows_processed_total)") with a defensible burn-rate alert. Can recommend CT sizing for harbor's connection profile.
- **Authoritative sources:** Cilium docs §"Connection Tracking"; `cilium-dbg bpf ct list`; `bpf-ct-*` flag reference; Hubble metrics docs.

### Scenario 3 (kube-proxy replacement)
- **Knowledge area:** Socket-LB vs. DSR vs. SNAT mode, the `cilium_services_events_total` and `cilium_bpf_map_ops_total{map_name="cilium_lb*"}` signal pair, what `cilium status --verbose` says when service program install fails, downgrade path when KPR misbehaves.
- **Depth bar:** Can write a decision tree that distinguishes "rollback chart" / "re-enable kube-proxy" / "manual cleanup." Knows whether re-enabling kube-proxy on a KPR cluster is safe, and what `kube-proxy-replacement-healthz-bind-address` lets you probe.
- **Authoritative sources:** Cilium docs §"Kubernetes Without kube-proxy"; CHANGELOG entries on KPR; the upstream issue tracker for known regressions in 1.16.x.

### Scenario 4 (NetworkPolicy enforcement drift)
- **Knowledge area:** Identity allocation (kvstore-backed vs. CRD-backed), `cilium_identity`, `cilium_policy_import_errors_total`, `cilium_endpoint_regeneration_time_stats_seconds`, fail-open-vs-fail-closed semantics during agent restart and during endpoint regeneration.
- **Depth bar:** Can articulate the security implication of each tier choice — silencing a policy-drift alert is a security-adjacent decision that needs security-specialist sign-off (per my own boundary). Can write the SLI as a user-meaningful number, not just "policy import errors == 0."
- **Authoritative sources:** Cilium docs §"Network Policy" + §"Identity-Aware"; upstream issues on policy fail-open behavior during regeneration; CiliumNetworkPolicy CRD reference.

### Scenario 5 (Hubble pipeline)
- **Knowledge area:** Hubble Relay topology, `hubble_flows_processed_total`, `hubble_lost_events_total`, Relay-to-agent gRPC health, ring buffer sizing, the difference between "agent stopped emitting" and "Relay can't keep up."
- **Depth bar:** Can author an independent page that catches Hubble blindness even when the rest of Cilium looks healthy. Can specify the fallback flow source for the runbook.
- **Authoritative sources:** Cilium Hubble docs; `hubble observe` CLI reference; the upstream Hubble repo metrics doc.

### Scenario 6 (IPAM exhaustion)
- **Knowledge area:** `cluster-pool` operator behavior, `cilium_ipam_capacity` / `cilium_ipam_allocation_ops`, per-node slice exhaustion vs. pool exhaustion semantics, the interaction with Karpenter NodePool churn.
- **Depth bar:** Can specify a leading-indicator SLI with a page threshold that gives ≥ 24h lead time on the current churn pattern. Knows the safe migration path when bumping `clusterPoolIPv4MaskSize` for new vs. old nodes (the chart comment is correct but the operational story isn't fully written).
- **Authoritative sources:** Cilium IPAM docs §"Cluster Scope"; cilium-operator metrics reference; Karpenter NodePool refresh procedure (k8s-capacity-management owns the refresh, I just need the IPAM signal to trigger it).
- **Co-owner flag:** the page threshold here is k8s-capacity-management co-territory because the response is a NodePool refresh.

### Scenario 7 (Upgrade safety / runbook)
- **Knowledge area:** `cilium status`, `cilium-dbg`, BPF map persistence across agent restart (which maps are pinned, which are recreated), canary procedure for a DaemonSet that's `system-node-critical`.
- **Depth bar:** Can write a step-by-step pre-flight + abort runbook with named commands and observable signals at each step. Not generic Helm guidance.
- **Authoritative sources:** Cilium upgrade guide for 1.16.x; `cilium-cli` reference; upstream upgrade runbooks.

## What I would NOT want this expert to be
- **Not a general observability expert** (that's observability-platform-engineer) — they shouldn't be writing PromQL/LogQL beyond what's needed to specify a Cilium metric's meaning. The Cilium expert *specifies the signal*; the observability-platform-engineer *expresses it as a rule/dashboard*. Same seam I have with them.
- **Not a general Kubernetes networking expert.** I already have `network-specialist` and `sei-network-specialist`. The Cilium expert is for the eBPF datapath and Cilium-internal semantics — not for Service/Ingress/Istio interop except where Cilium's behavior differs.
- **Not an SLO designer.** That's my lane. I want them to tell me *what each metric means and how it predicts user-visible failure*; I'll convert that into SLI/SLO and tier the alerts.
- **Not a controller-runtime / CRD expert** for Cilium CRDs. I want operational depth, not "how is CiliumNetworkPolicy implemented in code." If we need to reason about CRD reconcile correctness, that's a kubernetes-specialist question.
- **Not a security-policy author.** They can tell me what `cilium_policy_import_errors_total` means; the threat model for NetworkPolicy enforcement is security-specialist's call.
- **Not a capacity-tuner for BPF maps.** They specify the signal and a defensible threshold; if the action is "raise the limit and roll the DaemonSet," that's k8s-capacity-management co-owned because it changes the agent's memory ceiling.

## Scope-cutter (what I'd defer first if asked for MVP)
If we have to MVP this expert role, I'd keep **Scenarios 1, 2, 5** (BPF map saturation, ConnTrack, Hubble blindness) as the load-bearing trio — those are the ones that directly burn an availability SLO and we have no in-house depth for. **Defer Scenarios 3, 6, 7** until we hit a Cilium upgrade window or an IPAM near-miss; un-defer when (3) a Cilium minor-version bump is planned, (6) `cilium_ipam_capacity` crosses 60% pool utilization, (7) the upgrade runbook is needed for a CVE response. **Scenario 4** (NetworkPolicy drift) I'd keep on the MVP list only if security-specialist signs that fail-open during regeneration is in our threat model; otherwise it's the next defer candidate.
