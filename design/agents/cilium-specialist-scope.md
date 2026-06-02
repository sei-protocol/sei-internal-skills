# Cilium specialist — scope (Coral consensus)

This document scopes a future `cilium-specialist` Coral agent. It is the **intake** for a downstream `/author-skill`-style authoring pass — it does not author the agent itself.

The scope is the synthesized output of Round 1 of a multi-round Coral exercise: 6 specialists (network-specialist, platform-engineer, sre-engineer, kubernetes-specialist, security-specialist, observability-platform-engineer) each provided parallel, blinded input on (a) Cilium-specific scenarios they'd dispatch to a dedicated expert and (b) what depth that expert would need to satisfy them. 43 scenarios were captured. This document synthesizes them, identifies the load-bearing core, names what's out of scope, curates authoritative documentation sources, and is the basis for Round 3 sign-off from each of the 6 specialists.

The Round 1 inputs verbatim live under `/Users/brandon/tide-workspace/Tide/.claude/skills/author-skill/state/run-cilium-scope-2026-06-02T05-07-10Z/round1/` — each specialist's `<name>.md`. Read them if the synthesis below loses fidelity for your lane.

## Why this agent exists

Cilium is load-bearing on the harbor EKS cluster (verified via PR sei-protocol/platform#772: ENOMEM cascade resolution that required four follow-up PRs to land cleanly because the team lacked dedicated Cilium depth). The platform team operates Cilium 1.16.6 with `routingMode: tunnel` / `tunnelProtocol: vxlan`, `kubeProxyReplacement: "true"` (coexisting with the EKS kube-proxy addon during the cutover), `ipam.mode: cluster-pool` on `100.64.0.0/14` with `clusterPoolIPv4MaskSize: 26`, `cni.exclusive: true`, Hubble + Relay + UI enabled, and zero `CiliumNetworkPolicy` resources committed yet.

The PR #772 → #778 → #779 → #780 saga (Helm float coercion + chart per-field render guards + duplicate-key YAML conflict + redundant ratio key) is exactly the kind of moment a dedicated Cilium expert would have shortened. Adjacent specialists (network, platform-engineer, sre) had to figure things out under time pressure with the documentation surface scattered across Cilium docs, the chart `values.yaml`, kernel commit history (memcg+BPF accounting at 5.11), and the `cilium-dbg` CLI surface.

This agent's job: encode that depth so the next incident, policy authoring pass, IPAM resize, KPR cutover, or upgrade-window decision lands in minutes instead of hours.

## Cilium expertise dimensions to encode

The 20 dimensions that recur across specialists' Round 1 scenarios, grouped by where the consensus weight sits.

### Load-bearing (MVP — appears in 4+ specialists' Round 1 scenarios)

1. **BPF map sizing physics** — CT4/CT6, NAT, policy, LB, endpoints, lxc, ipcache. How each is sized; how `mapDynamicSizeRatio` works; the chart's `bpf:` per-field render guards vs `extraConfig:` escape hatch (the PR #772 lesson); kernel ≥5.11 cgroup BPF memory accounting; per-map memory accounting via `cilium-dbg map list` / `bpftool map show`. Sources: platform-engineer S1, network S1, sre S1, observability S1+S5.
2. **CiliumNetworkPolicy + CCNP authoring (with identity-aware semantics)** — full policy language (`endpointSelector`, `toEndpoints`, `toServices`, `toEntities`, `toCIDRSet`, `toFQDNs`, `toPorts.rules.dns`), namespaced vs cluster-wide semantics, default-deny interactions with v1 NetworkPolicy (not symmetric), L7 HTTP rules and the Envoy proxy hop, audit mode. **Critical sub-section — identity model as load-bearing for CNP correctness**: identity is derived from labels; an attacker who can `kubectl label` a pod (or run a controller that does) can mint themselves into a privileged identity, and the entire policy graph collapses. Label-driven identity escalation, stale-identity windows under label mutation, label-source filters (`--labels`), and identity allocation timing relative to pod readiness are all CNP-correctness concerns — not separable. See dimension #6. Sources: kubernetes S3, network S2, security S1+S3.
3. **Hubble drop-reason taxonomy + pipeline health** — `cilium/cilium` `api/v1/flow/flow.proto` `DropReason` enum, mapping drop reason → root cause (Policy denied → which rule, `CT: Map insertion failed` → CT map full, `Invalid source IP` → masquerade misconfig). Operator-level Hubble CLI fluency: `hubble observe --verdict DROPPED --from-pod ...`. **Plus Hubble pipeline-blindness detection-of-detection**: `hubble_flows_processed_total`, `hubble_lost_events_total`, Relay-to-agent gRPC health, ring buffer sizing, and the fallback flow-source runbook for "Hubble is down — what do I use before deciding NetworkPolicy is the cause of customer X's 5xx?" Drop-reason triage is useless when the pipeline itself is dark. Sources: kubernetes S6, network S5, security S7, sre S5 cross-cutting.
4. **ConnTrack physics** — `bpf-ct-global-tcp-max` / `bpf-ct-global-any-max`, `--conntrack-gc-interval`, `bpf-ct-timeout-*` tunables, eviction policy under saturation (table full → reject new vs evict LRU → silent drop of established), eviction signals at the Hubble layer. Sources: network S1 cross-cutting, sre S2, observability S5, platform-engineer cross-cutting.
5. **kube-proxy replacement (KPR) cutover** — modes (`strict` / `partial` / `true` and the v1.16 string-vs-bool migration), socket-LB internals (cgroup connect4/6 hooks, `socketLB.hostNamespaceOnly`), nodeport-LB, DSR + Maglev hashing, `externalTrafficPolicy: Local` vs `Cluster` source-IP preservation, AWS NLB instance-target interaction, the `k8sServiceHost`/`k8sServicePort` requirement on EKS, conntrack handoff during a coexistence window. Sources: platform-engineer S3, network S3, sre S3, kubernetes S5.

### Material (MVP — appears in 2-3 specialists' scenarios)

6. **Cilium identity model** — label → identity derivation, `identity-allocation-mode` (CRD vs kvstore), the 64K cluster-wide identity cap, identity GC, stale-identity windows under label mutation, short-lived Job pod identity slot pressure, `hostNetwork: true` and reserved-identity semantics. Sources: kubernetes S4, security S3.
7. **FQDN egress + DNS proxy** — `toFQDNs.matchPattern`, the separate `toPorts.rules.dns` block required for proxy IP-learning, `--tofqdns-min-ttl`, `--tofqdns-dns-reject-response-code`, interaction with kube-dns / coredns / NodeLocal DNSCache, DoH/DoT bypass concerns, HTTP/2 connection coalescing implications. Sources: kubernetes S3, network S2, security S2.
8. **cluster-pool IPAM** — `clusterPoolIPv4PodCIDRList`, `clusterPoolIPv4MaskSize`, per-node slice immutability (is bumping mask on running cluster safe?), pool fragmentation, exhaustion symptoms (`InsufficientIPsForNodeCIDR`), migration to a second pool. Sources: platform-engineer S2, network S7, sre S6.
9. **IMDS / IRSA blocking via Cilium policy** — CIDR-based egress to 169.254.169.254 + `sts.amazonaws.com`, host-firewall mode, policy enforcement ordering relative to pod readiness (race window between pod-ready and first-allowed-egress). Sources: network S2 cross-cutting, security S4.
10. **Hubble metric cardinality and shape** — which Hubble metrics safe cluster-wide (`dns`, `drop`, `flow`, `tcp`) vs which scoped (`http` on high-churn workloads), `hubble.metrics.enabled` context options (`sourceContext`, `destinationContext`) cardinality explosion, stream-label vs structured-metadata vs body classification for Loki. Sources: platform-engineer S4, observability S2+S3.
11. **Agent restart blast radius** — bpffs pinning (`/sys/fs/bpf`), which BPF maps survive agent restart vs which are rebuilt, CT/NAT map persistence guarantees, rolling-upgrade tenant-visibility, `cilium status` + `cilium-dbg` post-restart verification. Sources: platform-engineer S6, sre S7, kubernetes cross-cutting.
12. **EKS-specific install + upgrade traps** — VPC CNI replacement vs chaining, `cni.exclusive` semantics, IRSA traffic paths through Cilium policy, EKS-version-specific upgrade procedures, `aws-node` DaemonSet neutering, agent restart storms on chart upgrade. Sources: kubernetes S7, platform-engineer S3 cross-cutting.

### Adjacent (appears in 1-2 specialists' scenarios; valuable but lower MVP priority)

13. **Cilium ↔ Istio coexistence** — socket-LB intercept vs sidecar (Envoy capture order), `socketLB.hostNamespaceOnly` workaround, CNP vs Istio AuthorizationPolicy semantics, decision tree for "Cilium or Istio is dropping this." Sources: network S6, platform-engineer S7.
14. **Transparent encryption (WireGuard / IPsec)** — `enable-wireguard`, `encryption.type`, exact list of traffic in-scope vs out-of-scope (host netns, ClusterIP local-resolve, hostPort), key rotation cadence, threat model. Sources: security S6.
15. **Tetragon TracingPolicy** — kprobe/tracepoint selection, in-kernel filters, enforcement actions (`Sigkill`, `Override`), event volume scaling on hot syscalls, policy lifecycle, backpressure under export-pipeline saturation. Sources: security S5, observability S4. **(Note: deferrable — see "Deferred with un-defer triggers" below.)**
16. **NetworkPolicy enforcement drift (fail-open vs fail-closed)** — identity allocation health, `cilium_policy_import_errors_total`, endpoint regeneration stalls, fail-open semantics during agent restart vs endpoint regeneration. Sources: sre S4. **(Security co-territory — see anti-scope #6.)**
17. **Agent health vs metric signal lag** — `/healthz` + `/readyz` vs metric scrape lag, conditions surfaced via metrics vs status vs log line only (`cilium_unreachable_nodes`, `cilium_unreachable_health_endpoints`, `cilium_controllers_failing`). Sources: observability S7.
18. **NodePort / LoadBalancer + KPR datapath** — DSR vs SNAT, Maglev hashing, source-IP preservation on AWS NLB instance-target, WebSocket session affinity for long-lived JSON-RPC. Sources: kubernetes S5.
19. **VXLAN MTU on EKS + ENA + Bottlerocket** — 50-byte encap overhead, ENA MTU 9001, PMTUD validation, `mtu:` Helm value vs auto-detection, MTU-drop signal in Hubble. Sources: platform-engineer S5.
20. **Self-hosted GitHub Actions runner egress** — CNP egress with FQDN + CIDR for GitHub Actions endpoints, ghcr / ecr image pulls, fork-PR hostile-by-default assumption. Sources: security S8. **(Deferrable — un-defer when fork-PR builds land on harbor.)**

### Deferred (multi-cluster or future-state; un-defer triggers explicit)

21. **ClusterMesh** — multi-cluster identity propagation, CIDR planning, cluster-id constraints. **Un-defer when**: cluster #2 exists.
22. **BGP / LB-IPAM** — Cilium BGP control plane, LoadBalancer IP allocation. **Un-defer when**: harbor moves off NLB or adds bare-metal sites.
23. **Egress Gateway** — Cilium egress gateway, SNAT through a dedicated node. **Un-defer when**: outbound IP allowlisting becomes a compliance requirement.
24. **Gateway API for Cilium** — Cilium's Gateway API implementation. **Un-defer when**: harbor adopts Gateway API for ingress.

## MVP scope (load-bearing now)

Six areas. The agent's v1 should be able to credibly answer to depth-bar level on all six. If forced to ship in pieces, ship them in this order — each unblocks a different specialist's near-term work.

1. **BPF map sizing + ENOMEM diagnostics** (closes the PR #772 follow-on saga; un-blocks Karpenter scale-out to `.24xlarge` nodes)
2. **CiliumNetworkPolicy authoring** (zero CNPs committed yet; security's east-west containment baseline is gated on this)
3. **Hubble drop-reason triage** (turns 4-hour incidents into 10-minute ones; cross-cutting value across SRE, security, kubernetes, network)
4. **ConnTrack physics + SLI authoring** (sre's #2 scenario; gates the "page vs ticket" decisions in PR-772's alert set)
5. **kube-proxy replacement cutover semantics** (next non-empty cluster will need this; harbor was green-field)
6. **cluster-pool IPAM safety claims** (planned `/26 → /25 → /24` mask resize needs operator-source verification; pre-staged alerting on pool exhaustion deferred from PR #772)

### Fast-follow (between MVP and deferred)

- **PR sei-protocol/platform#772 dashboard vs upstream Cilium mixin reconciliation.** The dashboard added in #772 is a hand-rolled vendor of a subset of the upstream mixin. The agent should know to vendor `install/kubernetes/cilium/files/cilium-agent/dashboards/` from upstream and document where harbor's panels deviate, rather than carry a parallel fork. One PR's worth of work; lands after MVP.

## Deferred with un-defer triggers

Items below are valuable but not load-bearing today. Each has an explicit un-defer condition; the agent doesn't need to encode these at v1.

| Area | Un-defer trigger |
|---|---|
| Tetragon TracingPolicy depth | First agent-runtime exploit OR untrusted-code-execution workload requirement (security S5 un-defer) |
| Self-hosted Actions runner CNP | Decision to host fork-PR builds on harbor (security S8 un-defer) |
| Transparent encryption / WireGuard | Inter-region requirement OR compliance ask for "data in transit encrypted within cluster" (security S6) |
| Cilium ↔ Istio coexistence debug | First "Cilium vs Istio finger-pointing" production incident (platform-engineer S7 un-defer) |
| MTU/VXLAN tuning | "Works on small packets, hangs on large" symptom OR jumbo-frame workload (platform-engineer S5 un-defer) |
| ClusterMesh / BGP / Egress Gateway / Gateway API | See dimensions 21-24 above |
| Native routing vs tunnel decision | Push past ~200 nodes OR ClusterMesh adoption OR BGP requirement (network S4 un-defer) |
| Cilium upgrade-window runbook | Planned Cilium minor-version bump OR CVE-driven upgrade need (sre S7 un-defer) |
| IPAM pool migration to a second pool | `cilium_ipam_capacity` crosses 60% pool utilization (platform-engineer S2 + sre S6) |
| Hubble cardinality re-tune | First Prometheus federation cardinality alert (platform-engineer S4 un-defer) |
| Cilium upgrade safety / rolling-upgrade tenant impact | First tenant-impacting rolling upgrade (platform-engineer S6 un-defer) |

## Required depth bar

What every specialist would consider sufficient depth in their lane:

- **Has operated Cilium at >100 nodes through at least one outage** where the metric signals or `cilium-dbg` output was the diagnostic path. Not "has read the docs."
- **Has shipped at least one production CNP** including FQDN + identity-aware rules and debugged a "policy looks right but traffic still drops" incident.
- **Has done a kube-proxy replacement cutover** on a non-empty cluster.
- **Knows the chart 1.16.x quirks** specifically: per-field render guards, the Helm float-coercion bug class, `bpf:` vs `extraConfig:` decision points.
- **Can read `api/v1/flow/flow.proto`** in the Cilium repo and map a `DropReason` enum value back to the BPF program that emitted it.
- **Can size BPF maps from observed metrics** (`cilium_endpoint`, `cilium_ct_entries`, `cilium_services_events_total`) for harbor's actual workload, not generic vendor folklore.
- **Can write the exact CNP shape** for controller informer/watch traffic (apiserver `:443`, kube-dns, webhook callbacks) without iteration.
- **Knows the EKS-specific Cilium constraints**: ENI vs overlay vs native routing, VPC CNI compatibility, `aws-node` neutering, IRSA path through Cilium policy.
- **Has interpreted Hubble flow output for security telemetry** in production — knows what's leaked (HTTP paths with tokens in query strings, DNS queries betraying internal hostnames, **identity labels that include tenant-identifying data**), what's configurable to redact, and the auth model for Hubble Relay multi-tenant access.

## Anti-scope (what this expert is NOT)

Consensus across all 6 specialists. Each line maps to an existing Coral specialist whose lane the Cilium expert must NOT overlap with.

1. **NOT a controller-runtime expert** — kubernetes-specialist owns informer / workqueue / leader-election internals + CRD design.
2. **NOT a generic K8s networking expert** — network-specialist owns L2/L3, Service/Ingress, kube-proxy basics, BGP fundamentals, MTU at the VPC layer. The Cilium expert owns the *Cilium-specific delta* (CNP/CCNP, DNS proxy, FQDN policy, identity model, eBPF datapath specifics).
3. **NOT a service mesh / Istio expert** — Istio VirtualService / sidecar injection / AuthorizationPolicy / mTLS are out of scope. The Cilium expert speaks only to the seam (socket-LB vs sidecar, CNP vs Istio AuthorizationPolicy boundary).
4. **NOT a capacity-math expert** — k8s-capacity-management owns node sizing, workload requests/limits, BPF map *value selection*. The Cilium expert names *what knobs exist*; capacity picks *what values to set*.
5. **NOT a PromQL/LogQL author** — observability-platform-engineer owns query authorship, recording rules, dashboard panels. The Cilium expert specifies *what metric and what shape*; observability writes the expression.
6. **NOT a security threat-model designer** — security-specialist owns threat modeling and what attacker tradecraft to detect. The Cilium expert translates intent into CNP / Tetragon syntax — does not become the policy gatekeeper for every namespace.
7. **NOT an SLO designer** — sre-engineer owns SLI/SLO + alert-tier (page vs ticket vs silent) + runbook framing. The Cilium expert provides the *physics* (what does pressure 0.8 mean at the eBPF datapath); sre picks the threshold and tier.
8. **NOT a Grafana dashboarding expert** — observability-platform-engineer owns panel wiring, layout, mixin vendoring mechanics.
9. **NOT a generic eBPF tutor** — no CO-RE / libbpf primer. The expertise is eBPF *as Cilium ships it* (specific maps, specific hooks, specific `cilium-dbg` surface).
10. **NOT a kernel / bpfilter developer** — patching Cilium itself or the kernel is out. We run upstream Cilium on Bottlerocket.
11. **NOT a sei-protocol / seid networking expert** — sei-network-specialist owns CometBFT P2P, seid port semantics. The Cilium expert's lane stops at the pod boundary.
12. **NOT a NetworkPolicy author for application teams** — application owners write their own CNPs; the Cilium expert advises on syntax + scope + gotchas, does not own per-tenant policies.
13. **NOT a Tetragon specialist deep-dive** — Tetragon is in scope at the awareness + scoping level (event volume sizing, TracingPolicy CRD basics) but not as a primary lane. Different product, different operational surface; un-defer-triggered.
14. **NOT a ClusterMesh operator** — until cluster #2 exists (see dimension 21).
15. **NOT a chart-values plumber for non-observability bits** — platform-engineer owns HelmRelease structure; the Cilium expert says *which values to set*, not *how the manifest renders*.
16. **NOT a Karpenter / node-lifecycle expert** — k8s-capacity-management + platform-engineer own node provisioning. The Cilium expert speaks to per-node CIDR slice behavior on scale-up, not instance selection.
17. **NOT a compliance / audit-letter writer** — technical answers only, not SOC2-friendly prose.
18. **NOT an AWS VPC / security-group designer** — VPC peering, TGW, route tables stay with network-specialist + platform-engineer.
19. **NOT an AppSec / pen-test expert** — security-specialist brings the threat model and attack tradecraft; the Cilium expert translates intent into CNP / Tetragon syntax + identifies enforcement-ordering pitfalls.

## Curated documentation sources

Pinned to Cilium 1.16.x (harbor's current version). Each source includes a one-line note on what's load-bearing.

### Tier 1 — Cilium upstream documentation

1. **[docs.cilium.io/en/v1.16/](https://docs.cilium.io/en/v1.16/)** — base entry; the v1.16 anchor is non-negotiable (semantics differ across minor versions).
2. **[Network Policy language reference](https://docs.cilium.io/en/v1.16/security/policy/language/)** — full CNP language; mandatory for dimension #2.
3. **[Kube-Proxy Free / KPR docs](https://docs.cilium.io/en/v1.16/network/kubernetes/kubeproxy-free/)** — socket-LB, DSR, Maglev, the `kubeProxyReplacement: "true"` string-vs-bool migration; mandatory for dimension #5.
4. **[BPF + XDP Reference Guide](https://docs.cilium.io/en/v1.16/bpf/)** — datapath fundamentals; back-references for dimensions #1 (BPF maps) and #3 (drop reasons).
5. **[Operations: Performance tuning](https://docs.cilium.io/en/v1.16/operations/performance/tuning/)** — `mapDynamicSizeRatio`, per-map sizing; load-bearing for dimension #1.
6. **[Hubble observability](https://docs.cilium.io/en/v1.16/observability/hubble/)** — Hubble flow schema + metrics; mandatory for dimensions #3, #10.
7. **[cluster-pool IPAM](https://docs.cilium.io/en/v1.16/network/concepts/ipam/cluster-pool/)** — IPAM mode + mask sizing semantics; mandatory for dimension #8.
8. **[Identity-aware security](https://docs.cilium.io/en/v1.16/concepts/security/identity/)** — identity model; mandatory for dimension #6.
9. **[Routing concepts: tunnel vs native](https://docs.cilium.io/en/v1.16/network/concepts/routing/)** — encap modes, MTU, ENI; load-bearing for dimensions #4 and #19.
10. **[Troubleshooting](https://docs.cilium.io/en/v1.16/operations/troubleshooting/)** — drop-reason taxonomy, `cilium-dbg` surface; mandatory for dimension #3.
11. **[FQDN policies](https://docs.cilium.io/en/v1.16/security/policy/language/#dns-based)** — `toFQDNs` deep semantics; mandatory for dimension #7.
12. **Cilium Helm chart `values.yaml` (v1.16.x, Hubble section specifically)** — `hubble.metrics.enabled`, `hubble.metrics.enableOpenMetrics`, `hubble.metrics.dynamic.*` context options, `hubble.relay.tls`, `hubble.ui.tls`, `hubble.eventQueueSize`, `hubble.flowBufferSize`. Pin to the chart truth (the agent must speak from this, not the docs paraphrase). Located in the `cilium/cilium` repo at `install/kubernetes/cilium/values.yaml`.

### Tier 2 — Cilium GitHub source paths

13. **[cilium/cilium](https://github.com/cilium/cilium)** — the source. Specific paths called out across Round 1:
    - `pkg/policy/api/` — CNP CRD schema (network S2, kubernetes S3)
    - `pkg/ipam/allocator/clusterpool/` — cluster-pool allocator (platform-engineer S2, network S7)
    - `pkg/datapath/loader/` — datapath compile + load (network S4)
    - `pkg/metrics/` — agent metric source (observability S1)
    - `pkg/controller/` + `pkg/health/` — agent health subsystem (observability S7)
    - `bpf/sockops/` — socket-LB BPF programs (network S6)
    - `daemon/cmd/daemon.go` — agent initialization (platform-engineer S6)
    - `api/v1/flow/flow.proto` — Hubble flow schema + `DropReason` enum (network S5, observability S2)
14. **[cilium/hubble](https://github.com/cilium/hubble)** — Relay, ringbuffer, flow protobuf.
15. **[cilium/tetragon](https://github.com/cilium/tetragon)** — TracingPolicy CRD, examples directory.

### Tier 3 — practitioner / vendor blog

16. **[Isovalent engineering blog](https://isovalent.com/blog/)** — Specific posts cited in Round 1:
    - "Tuning Cilium for large clusters" (BPF map sizing — network S1, platform-engineer S1)
    - "DNS-based policies" deep dive (FQDN — network S2)
    - "Migrating from VPC CNI to Cilium on EKS" (network S4, platform-engineer S3)
    - Cilium + Istio joint posts (network S6, platform-engineer S7)
    - "Tetragon in production" (security S5, observability S4)
    - BPF-map-pressure post(s) (sre S1, observability S5)

### Tier 4 — adjacent / cross-reference

17. **[eBPF.io](https://ebpf.io/)** — eBPF primitives reference; load-bearing for dimension #1 background, not for `cilium-dbg` operations.
18. **Linux kernel docs — memcg + BPF accounting** (`Documentation/admin-guide/cgroup-v2.rst`, `Documentation/admin-guide/sysctl/net.rst`) — the 5.11+ cgroup BPF charging that turned PR #772 into a four-PR saga.
19. **AWS EKS Best Practices Guide — Networking** — IMDS blocking, VPC CNI replacement, ENI mode constraints; cross-references dimensions #9, #12.

## How this scope informs the next step

This document is the **intake** for a downstream `/author-skill` pass. The pass will:

1. Use the MVP scope (six load-bearing areas) as the agent's primary domain expertise sections.
2. Use the deferred-with-un-defer-triggers as the agent's explicit "out of scope for now" framing.
3. Use the anti-scope to write the agent's "what this skill does NOT do" section.
4. Use the curated documentation sources as the agent's authoritative-references table.
5. Use the depth bar as the agent's "First Step — Always" section + sub-agent dispatch criteria.
6. Apply the standard skill-authoring methodology (RED-GREEN-REFACTOR pressure testing) against the resulting draft.

## Round 3 sign-off

Each of the 6 specialists who provided Round 1 input must explicitly sign off that this synthesized scope captures what they'd want to consume from the future `cilium-specialist` agent. Sign-off criterion: "with this expertise scope + documentation sources, would the resulting agent save me time on the kind of work I dispatched in Round 1?"

| Specialist | Round 1 contributions | Sign-off | Notes |
|---|---|---|---|
| `network-specialist` | 7 scenarios; pinned chart 1.16.6 / tunnel / KPR-cutover state | ✅ SIGN OFF (cycle 1) | All 7 scenarios mapped cleanly; anti-scope preserved; sources cited |
| `platform-engineer` | 7 scenarios; PR #772/#778/#779/#780 saga + helm-chart 1.16.x quirks | ✅ SIGN OFF (cycle 1) | PR #772 four-PR saga lineage explicit in dimension #1; chart-quirks depth-bar nails the lesson |
| `sre-engineer` | 7 scenarios; SLO/alert-tier framing on PR-772 alert set | ✅ SIGN OFF (cycle 2) | After Hubble pipeline-blindness was folded into MVP area #3 / dimension #3 with explicit pipeline-health detection-of-detection sub-bullet |
| `kubernetes-specialist` | 7 scenarios; controller-runtime + CNP + Job lifecycle intersections | ✅ SIGN OFF (cycle 1) | CNP-for-controller-watches + Hubble triage at MVP; anti-scope items #1, #4, #12 all clean |
| `security-specialist` | 8 scenarios; CNP design + FQDN + identity boundary + IMDS + Tetragon | ✅ SIGN OFF (cycle 2) | After: identity model elevated as critical sub-section of dimension #2 (label-driven identity escalation → "policy graph collapses"); Hubble depth-bar adds identity-label leak; anti-scope #19 added for AppSec/pen-test parity |
| `observability-platform-engineer` | 7 scenarios; metric-physics seam to the obs stack | ✅ SIGN OFF (cycle 2) | After: PR-772-vs-upstream-mixin reconciliation added as Fast-follow; Tier-1 source #12 pins chart `values.yaml` Hubble section |

**Iteration log** (≥1 round of disagreement-then-convergence required per the acceptance criteria — satisfied):

- _Round 3 cycle 1_: 3 sign-offs (network, platform-engineer, kubernetes); 3 concerns flagged (sre on Hubble pipeline-blindness missing from MVP; security on identity boundary tier + Hubble identity-label leak + AppSec exclusion parity; observability on PR-772 mixin reconciliation + chart values.yaml Tier-1 pinning).
- _Round 3 cycle 2_: 5 surgical edits applied — dimension #3 expanded to "drop-reason taxonomy + pipeline health"; dimension #2 expanded with identity-model critical sub-section; Hubble depth-bar adds identity-label leak; anti-scope #19 names AppSec exclusion; Fast-follow subsection added between MVP and Deferred for PR-772 dashboard vs upstream mixin; Tier-1 source #12 pins chart values.yaml. All 3 concerns re-checked and closed. Six-of-six consensus.
