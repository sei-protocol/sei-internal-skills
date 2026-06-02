# Round 1 input — platform-engineer

## Scenarios I'd dispatch to a Cilium expert

### Scenario 1: BPF map cap sizing on heterogeneous instance fleets (PR #772 + #778 + #779 + #780)
- **Task:** A fresh Karpenter-provisioned r6i.12xlarge (384 GiB) joined harbor and cilium-agent crashlooped with `BPF_MAP_CREATE: cannot allocate memory`. Chart-default `mapDynamicSizeRatio: 0.0025` sized `cilium_ct4_global` to ~12M entries (~1 GiB pre-allocated), which overflowed the agent's 512Mi memcg (kernel ≥5.11 charges BPF map memory to the calling cgroup). Agent never cleared `node.cilium.io/agent-not-ready`, Karpenter refused to disrupt the un-initialized node, workload sat Pending until a human noticed.
- **Why my expertise wasn't enough:** I can ship the helm values and observe ENOMEM, but I do not have a model for (a) how each BPF map's working set scales with instance/workload class, (b) the precise CT/NAT/policy/lb map families and which knob sizes which map, (c) the cgroup-charging kernel boundary at 5.11, or (d) which Cilium values map to which `cilium-config` keys when the chart's per-field render guards (`{{- if .Values.bpf.natMax }}` with int64 coercion) silently null quoted-string values. I burned three follow-up PRs (#778 quote-strings, #779 move to `extraConfig`, #780 drop redundant ratio key) discovering this empirically because the chart template logic is non-obvious.
- **What an expert would deliver:** A sizing rubric — for harbor's expected CPS, pod count, and Service count, what `ctTcpMax / ctAnyMax / natMax / policyMapMax / lbMapMax` should be, and the resulting BPF memcg budget per agent. Plus a clear table of "which keys go on `bpf:` vs which need `extraConfig:`" for this chart version, with the upstream parser key name (e.g. `bpf-ct-global-tcp-max`) so the verification command is deterministic.

### Scenario 2: cluster-pool IPAM CIDR/mask choices that are one-way doors
- **Task:** Harbor uses `clusterPoolIPv4PodCIDRList: 100.64.0.0/14` with `/26` per-node slices (64 IPs). Future ClusterMesh siblings sit on `100.68.0.0/14`, `100.72.0.0/14`. When a .24xlarge starts hitting pod-density limits the plan is `/26 → /25 → /24`, and we banked the assumption that bumping `clusterPoolIPv4MaskSize` on a running cluster is safe because the operator only allocates new slices on node create.
- **Why my expertise wasn't enough:** I want a Cilium expert to corroborate or refute that claim against the actual `cilium-operator` allocator behavior. Whether existing nodes ever re-mask, what happens when the pool fragments, and how to plan a graceful pool expansion (or migration to a second pool) is something I do not want to learn from a production incident.
- **What an expert would deliver:** Confirmation of the safe-to-bump-mask claim with operator source/changelog citations; documented ceiling for pod density per `/26`/`/25`/`/24` on Bottlerocket; the migration path to add a second pool when `100.64.0.0/14` exhausts; and pre-staged alerting for IPAM exhaustion that we explicitly deferred from #772.

### Scenario 3: kube-proxy replacement cutover on a non-empty cluster
- **Task:** Harbor was the green field. The next cluster won't be. When we eventually flip prod or a sibling from VPC CNI + kube-proxy to full Cilium, I need a known-good sequence: helm install Cilium chained → run connectivity test → set `kubeProxyReplacement: "true"` → delete `kube-proxy` DaemonSet → flip `cni.exclusive: true` → roll Karpenter nodes. We did this on harbor as a single big-bang PR (#166) only because harbor had no tenants.
- **Why my expertise wasn't enough:** I know the Kustomize/Flux ordering. I do not have a model for the connection-tracking handoff between iptables-based and eBPF-based service LB, what survives a live cutover, what doesn't (long-lived connections in iptables conntrack vs Cilium CT map), and which Hubble/agent signals tell me the cutover is healthy.
- **What an expert would deliver:** A staged cutover runbook with explicit per-step verification, prerequisites for `kubeProxyReplacement: true` on EKS specifically (the `k8sServiceHost`/`k8sServicePort` Terraform-rendered bit is fragile), and the failure modes that mean "stop and roll back" vs "keep going."

### Scenario 4: Hubble metric cardinality and what's safe to enable
- **Task:** I enabled Hubble metrics `dns:query;ignoreAAAA`, `drop`, `tcp`, `flow`, `icmp`, `http` because they're commonly recommended. I do not know the cardinality cost of `http` on a Sei p2p workload where pod IP/port churn is high, or whether `flow` ends up duplicative with `tcp`. We've seen Prometheus federation blow up before from cardinality I didn't predict.
- **Why my expertise wasn't enough:** I can read the Hubble metric docs. I cannot predict the label-set explosion for a specific workload shape, and the observability-platform-engineer agent owns query/dashboard tuning but doesn't own *which Hubble metric to enable*.
- **What an expert would deliver:** A "safe defaults / opt-in extras" recommendation for Hubble metrics on harbor's workload mix (validators, RPC, archive), with order-of-magnitude cardinality estimates per metric family and which to keep off until needed.

### Scenario 5: VXLAN/encap MTU and AWS interaction
- **Task:** Harbor uses `routingMode: tunnel` + `tunnelProtocol: vxlan`. VXLAN adds 50 bytes of overhead and AWS ENA defaults to MTU 9001. I never explicitly reasoned about pod MTU vs ENA MTU vs VXLAN overhead, and I haven't measured whether validator p2p gossip is doing PMTUD correctly across VXLAN.
- **Why my expertise wasn't enough:** I know how to set `mtu:` in helm values. I do not know what the right value *is* for EKS + Bottlerocket + ENA + Cilium VXLAN, or how to validate it. Silent MTU mismatches cause "works on small packets, hangs on large ones" which is the worst class of network bug.
- **What an expert would deliver:** Recommended MTU values for tunnel mode on EKS, a one-shot validation procedure (large-packet path test from pod-to-pod across AZ), and the signal in Hubble/agent metrics that flags MTU-related drops.

### Scenario 6: agent restart blast radius — what state is wiped, what's persisted
- **Task:** PR #772 set `updateStrategy.rollingUpdate.maxUnavailable: 1` because we believed per-node CT state is wiped on agent restart. I asserted that in the PR body but I cannot cite the upstream behavior precisely — is it CT, NAT, both, neither? Is BPF map state pinned to the bpffs (mounted to `/sys/fs/bpf`) and reattached, or rebuilt from scratch?
- **Why my expertise wasn't enough:** I know the DaemonSet rollout knobs. I don't know the precise data-plane state machine across agent restarts, which determines whether a rolling upgrade is a 1-second blip or a connection-draining event.
- **What an expert would deliver:** A clear statement of "on agent restart, X is preserved via bpffs pin, Y is rebuilt, Z drops connections," with the upstream code/docs citation. Drives whether rolling upgrades are tenant-visible and whether we need any pre-drain coordination.

### Scenario 7: Cilium ↔ Istio coexistence boundary
- **Task:** Cilium owns L3/L4 and kube-proxy replacement; Istio owns north-south + mTLS for sidecar-injected pods. We explicitly skip Istio sidecar on validators. I do not have a confident model for where Cilium NetworkPolicy stops and Istio AuthorizationPolicy starts, or whether eBPF socket-LB intercepts traffic *before* the Envoy sidecar in injected pods (and whether that matters).
- **Why my expertise wasn't enough:** I shipped the integration; I cannot debug the next "validator pod can't reach Service X but a curl from a debug pod works" issue without an expert who can read both stacks together.
- **What an expert would deliver:** A boundary diagram + decision tree for "when traffic is misbehaving, is it Cilium or Istio's responsibility" with the specific `cilium monitor` / `hubble observe` / Envoy access-log commands to localize the fault.

## Required depth per scenario

### Scenario 1 (BPF map sizing)
- **Knowledge area:** BPF map families (CT4/CT6, NAT, policy, LB, endpoints), how each is sized, kernel cgroup BPF memory accounting (5.11+), `mapDynamicSizeRatio` semantics, chart-vs-`cilium-config` key mapping per chart version, `extraConfig` escape hatch.
- **Depth bar:** Must be able to compute per-agent BPF memory envelope from a fleet's max-instance + workload-CPS profile, name which map dominates, and write the helm values that survive the chart's per-field render quirks for chart 1.16.x. Must know that kernel ≥5.11 charges to cgroup.
- **Authoritative sources:** Cilium docs `concepts/ebpf/maps`, `operations/performance/tuning`, the chart `values.yaml` for the pinned version, kernel commit memcg/BPF accounting (5.11 mainline), Cilium issues/PRs around `mapDynamicSizeRatio`.

### Scenario 2 (IPAM)
- **Knowledge area:** `cluster-pool` IPAM operator allocator, CiliumNode resource fields, pool exhaustion behavior, multi-pool support, ClusterMesh CIDR constraints.
- **Depth bar:** Cite operator behavior on `clusterPoolIPv4MaskSize` change against a running cluster; describe pool fragmentation; describe migration to a second pool without re-IPAM of existing nodes.
- **Authoritative sources:** `cilium-operator` source (`pkg/ipam/clusterpool`), Cilium IPAM concept docs, ClusterMesh CIDR planning guide.

### Scenario 3 (kube-proxy replacement cutover)
- **Knowledge area:** `kubeProxyReplacement` modes, socket-LB vs nodeport datapath, `k8sServiceHost`/`k8sServicePort` requirement, connection-tracking handoff from iptables to BPF, `cilium connectivity test` semantics.
- **Depth bar:** Provide a stepwise migration with explicit verification at each step; explain which long-lived connections survive cutover and which don't.
- **Authoritative sources:** Cilium `kube-proxy replacement` docs, upstream migration guide, EKS-specific notes (no LoadBalancer hairpin on AWS NLB instance-target — relates to PR #771).

### Scenario 4 (Hubble cardinality)
- **Knowledge area:** Hubble metric label sets per metric family, exporters, sampling/filtering, `enableOpenMetrics`.
- **Depth bar:** Order-of-magnitude cardinality estimates per metric family on a given workload shape; clear opt-in/opt-out recommendation.
- **Authoritative sources:** Hubble metrics reference, upstream issues on cardinality, real series counts from `count({__name__=~"hubble_.*"})` on harbor today.

### Scenario 5 (MTU/VXLAN)
- **Knowledge area:** Encapsulation overhead, AWS ENA MTU, kernel PMTUD, Cilium MTU auto-detection vs explicit `mtu:`.
- **Depth bar:** Concrete recommendation with a validation script (large-packet cross-AZ test), and the agent/Hubble signals that flag MTU drops.
- **Authoritative sources:** Cilium MTU docs, AWS ENA documentation, Bottlerocket networking notes.

### Scenario 6 (Agent restart state)
- **Knowledge area:** bpffs pinning, agent restart lifecycle, what BPF maps live across restart vs which get rebuilt.
- **Depth bar:** Specific list of "preserved / rebuilt / drops connections" with upstream citations; impact on rolling upgrade safety.
- **Authoritative sources:** Cilium agent restart docs, `bpffs` mount semantics, `daemon/cmd/daemon.go` initialization path.

### Scenario 7 (Cilium ↔ Istio)
- **Knowledge area:** CNI-chained vs replace, socket-LB intercept point vs sidecar, CiliumNetworkPolicy vs Istio AuthorizationPolicy semantics, Hubble visibility of sidecar-injected pods.
- **Depth bar:** Boundary diagram + a working `cilium monitor` + `hubble observe` + Envoy access-log command sequence for fault localization.
- **Authoritative sources:** Cilium + Istio integration docs (both projects), Hubble observe reference.

## What I would NOT want this expert to be
- A **deep eBPF programmer** writing custom probes. We explicitly deferred CometBFT-aware eBPF in the harbor design doc; if/when we reach for it, that is a different specialist.
- A **Hubble UI/dashboard tuner**. That's observability-platform-engineer's lane. The Cilium expert tells me *what to emit*; the obs agent owns *how to query and render it*.
- A **PromQL/LogQL author**. The harbor BPF alerts work I did (#772, follow-ups) needed PromQL recompositions to attach `node` labels for inhibits — obs-platform owned that. Cilium expert should specify *which metric and what shape*; alert authoring is downstream.
- A **kernel/bpfilter developer**. We run upstream Cilium on Bottlerocket. Patching Cilium itself or the kernel is out of scope.
- A **NetworkPolicy author for application teams**. Application owners write their own `CiliumNetworkPolicy`; the expert advises on syntax, scope, and gotchas — does not become the policy gatekeeper for every namespace.
- A **Tetragon/process-observability specialist**. Different product, different operational surface. Out of scope until Cilium itself is boring on harbor.
- A **ClusterMesh operator** until we actually have a second cluster. The CIDR/cluster-id choices we already made are the only ClusterMesh-relevant artifact today; full mesh wiring is Phase 2 and not load-bearing for this agent's first version.

**Scope-cutter if forced to MVP:** Keep Scenarios 1, 2, 3 (BPF sizing, IPAM safety, kube-proxy-replacement cutover) as load-bearing — these are the ones that have already cost us, or that will cost us on the next cluster. Defer Scenarios 4–7 until concrete operational pain materializes. Un-defer trigger for each: 4 when Prom federation throws a cardinality alert; 5 when we see "works on small payloads, hangs on large" symptoms; 6 when a rolling upgrade tenant-impacts; 7 on the first Cilium-vs-Istio finger-pointing incident.
