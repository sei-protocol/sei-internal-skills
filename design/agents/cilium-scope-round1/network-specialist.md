# Round 1 input — network-specialist

Context I'm grounding in: harbor runs Cilium 1.16.6, `routingMode: tunnel` / `tunnelProtocol: vxlan`, `kubeProxyReplacement: "true"` (coexisting with the EKS kube-proxy addon during the cutover), `ipam.mode: cluster-pool` on `100.64.0.0/14` with `clusterPoolIPv4MaskSize: 26`, `cni.exclusive: true` flipping in tandem with the vpc-cni addon removal, Hubble + Relay + UI enabled, and ENOMEM was resolved on the agent in platform#772. There are zero `CiliumNetworkPolicy` / `CiliumClusterwideNetworkPolicy` resources committed yet, so a lot of the work below is about getting policy + observability landed on a cluster that just finished its CNI cutover.

## Scenarios I'd dispatch to a Cilium expert

### Scenario 1: Diagnosing the agent ENOMEM that triggered platform#772
- **Task**: Decide *why* the cilium-agent OOMed on harbor and whether the fix in #772 (memory bump alone) is durable, or whether we needed to tune `bpf-map-dynamic-size-ratio`, cap `bpf-ct-global-tcp-max`, or shrink the policy map.
- **Why my depth wasn't enough**: I can read a pod OOMKill event and bump `limits.memory`, but I can't tell whether the agent is sized by BPF maps (CT, NAT, policy, lb4_services), by Hubble ringbuffer, or by identity churn — and the wrong root cause means we'll OOM again the first time pod density jumps.
- **What an expert would deliver**: Map-by-map memory accounting (`cilium-dbg map list`, `cilium-dbg bpf ct list global | wc`, `bpftool map show`), the formula linking node CPU count and `bpf-map-dynamic-size-ratio` to actual map bytes, and a verdict on whether 512Mi is durable through a Karpenter scale-up to `.24xlarge` nodes.

### Scenario 2: Replacing NetworkPolicy with CiliumNetworkPolicy / CCNP for the workload namespaces
- **Task**: Author the harbor egress policy set (DNS to kube-dns, Sei RPC FQDNs, AWS APIs via VPC endpoints, IMDSv2 block) using `toFQDNs` + `toCIDRSet` + `toServices` instead of vanilla NetworkPolicy.
- **Why my depth wasn't enough**: I know the *semantics* of NetworkPolicy (additive, OR), but Cilium's policy engine has its own evaluation order — `toFQDNs` requires the DNS proxy, `toServices` only resolves ClusterIP services with `enable-k8s-endpoint-slice`, and `default-deny` interactions between CNP and v1 NetworkPolicy are not symmetric. I'd guess wrong on at least one of those.
- **What an expert would deliver**: The exact CNP shape that gets DNS-rewriting + FQDN allowlists working (including the `rules.dns` matchPattern needed so the proxy learns the IP), plus the trade-off between CNP and CCNP for cluster-wide IMDS/link-local blocks.

### Scenario 3: kube-proxy replacement cutover validation
- **Task**: Confirm we can remove the EKS kube-proxy addon without breaking NodePort/LoadBalancer traffic for ingress-nginx and the loki-gateway NLB, given `kubeProxyReplacement: "true"` is already on.
- **Why my depth wasn't enough**: I understand iptables vs IPVS vs eBPF socket-LB conceptually, but the failure modes during a *coexistence* window — duplicate DNAT, conntrack divergence between netfilter and cilium's CT map, NLB health-check black-holing — are Cilium-specific and I'd ship blind.
- **What an expert would deliver**: A pre-flight checklist (`cilium-dbg service list`, `cilium-dbg status --verbose` KubeProxyReplacement section, `bpftool prog show` for the cgroup connect4 hook), the exact order of operations (drain → remove addon → restart agent → verify), and the rollback signal.

### Scenario 4: Tunnel vs native routing decision before scale-out
- **Task**: Decide whether to stay on `routingMode: tunnel` (vxlan) or move to native routing with `eni` IPAM ahead of pushing past ~200 nodes / multi-cluster connectivity.
- **Why my depth wasn't enough**: I know VXLAN adds 50 bytes and that EKS supports native routing via ENI prefix delegation, but the actual decision drivers — MTU interaction with the EKS-managed VPC, `enable-endpoint-routes`, IP exhaustion math on `100.64.0.0/14`, whether ClusterMesh later forces our hand — are Cilium-architecture choices I can't make confidently.
- **What an expert would deliver**: A side-by-side with the failure modes for each (tunnel = MTU drops on jumbo, native = security-group sprawl + pod IP burn), and a forcing-function list (ClusterMesh, BGP, Egress Gateway) that would flip the decision.

### Scenario 5: Hubble policy verdict triage for "this pod can't reach X"
- **Task**: When a workload pod can't reach an external endpoint, use Hubble to determine whether the drop is policy, DNS, encryption, or a kernel/conntrack issue — not just "the policy is wrong, try again."
- **Why my depth wasn't enough**: I can read `hubble observe` output, but mapping a `DROPPED (Policy denied)` vs `DROPPED (CT: Map insertion failed)` vs `FORWARDED` with no return traffic to a *fix* requires knowing Cilium's drop reason taxonomy and which BPF program emitted it.
- **What an expert would deliver**: A drop-reason-to-root-cause table (Policy denied → which rule? CT insertion failed → ct map full, raise size; Invalid source IP → masquerade misconfig), plus the `cilium-dbg policy trace` invocation that resolves the exact rule.

### Scenario 6: Sei-node + Istio sidecar interaction with Cilium's socket-LB
- **Task**: When an Istio-injected workload talks to a ClusterIP service, decide whether Cilium's socket-LB short-circuits before the sidecar sees the connection, and what to do about it.
- **Why my depth wasn't enough**: This is the classic Cilium-meets-Istio footgun — socket-LB in `connect()` rewrites the destination before the iptables-redirect that captures into Envoy, and the fix (`socketLB.hostNamespaceOnly: true`, or disabling socket-LB for the sidecar netns) is Cilium-specific.
- **What an expert would deliver**: The exact Helm value for harbor's Cilium release that preserves Istio interception, plus the Hubble query that proves it (sidecar-originated flows show 127.0.0.1:15001 as a hop).

### Scenario 7: Cluster-pool IPAM mask resize for .24xlarge nodes
- **Task**: Plan the safe path from `clusterPoolIPv4MaskSize: 26` (64 IPs) to `/25` or `/24` ahead of provisioning instances that pack >60 pods.
- **Why my depth wasn't enough**: The values file comment says this is safe because existing nodes keep their slice — I'd accept that, but I can't verify the claim against the cluster-pool operator code, and "safe at Helm reconcile time" doesn't tell me what happens if a node CIDR slice is fully allocated when the operator restarts.
- **What an expert would deliver**: Confirmation of the per-node-slice immutability (which controller, which CRD field on CiliumNode), the operator log lines to watch during the resize, and the exhaustion symptom (`InsufficientIPsForNodeCIDR` event or similar).

## Required depth per scenario

### For Scenario 1
- **Knowledge area**: BPF map sizing (`bpf-map-dynamic-size-ratio`, `bpf-ct-global-tcp-max`, `bpf-policy-map-max`, `bpf-lb-map-max`), agent memory accounting, Hubble ringbuffer sizing.
- **Depth bar**: Names the specific maps consuming the most memory on a node of size N CPUs / M pods, gives the formula (not "it depends"), and produces a `cilium-dbg`/`bpftool` command set that reproduces the accounting.
- **Authoritative sources**: `docs.cilium.io/en/v1.16/operations/performance/tuning/` (BPF map sizing), `docs.cilium.io/en/v1.16/cmdref/cilium-dbg_map_list/`, `Documentation/admin-guide/sysctl/net.rst` for conntrack overlap, Isovalent blog "Tuning Cilium for large clusters."

### For Scenario 2
- **Knowledge area**: CNP/CCNP policy engine — DNS proxy, FQDN matching, `toServices` constraints, default-deny merging with v1 NetworkPolicy, `enable-policy: default`.
- **Depth bar**: Writes a working CNP that combines `toFQDNs.matchPattern`, `toEndpoints`, and `toPorts.rules.dns` correctly the first time, and can explain *why* a separate `rules.dns` block is required (the proxy needs the match to learn the IP).
- **Authoritative sources**: `docs.cilium.io/en/v1.16/security/policy/language/` (the full policy language reference), `docs.cilium.io/en/v1.16/security/policy/kubernetes/`, `pkg/policy/api/` in the cilium repo for the CRD schema, Isovalent "DNS-based policies" deep dive.

### For Scenario 3
- **Knowledge area**: KPR (kube-proxy replacement) modes — `strict` vs `partial` vs `true`, socket-LB vs nodeport-LB, the cgroup connect4/6 hooks, conntrack ownership during coexistence.
- **Depth bar**: Tells me precisely which services are already served by Cilium vs kube-proxy *today* (`cilium-dbg service list` output interpretation), and predicts the exact failure mode if we remove the addon mid-day vs during drain.
- **Authoritative sources**: `docs.cilium.io/en/v1.16/network/kubernetes/kubeproxy-free/`, the KEP/PR thread for `kubeProxyReplacement: "true"` semantics change (the string-vs-bool migration), `cilium-dbg status --verbose` man page.

### For Scenario 4
- **Knowledge area**: Datapath modes — tunnel vxlan/geneve, native routing, ENI mode, `enable-endpoint-routes`, MTU calculation, ClusterMesh and BGP control-plane prerequisites.
- **Depth bar**: Names the EKS-specific constraints (no SG-per-pod in tunnel mode unless `enable-endpoint-routes`, ENI mode requires VPC CNI absence which we already have), and the migration path's downtime profile.
- **Authoritative sources**: `docs.cilium.io/en/v1.16/network/concepts/routing/`, `docs.cilium.io/en/v1.16/installation/cni-chaining-aws-cni/` (anti-pattern reference), Isovalent "Migrating from VPC CNI to Cilium on EKS," `pkg/datapath/loader/`.

### For Scenario 5
- **Knowledge area**: Hubble flow schema, drop reason codes (`api/v1/flow/flow.proto` `DropReason` enum), `cilium-dbg policy trace`, `cilium-dbg monitor --type drop`.
- **Depth bar**: Gives a drop-reason → action mapping that distinguishes policy, CT/NAT exhaustion, encryption (WireGuard/IPsec), and encapsulation drops; not "check the policy."
- **Authoritative sources**: `api/v1/flow/flow.proto` in cilium/cilium, `docs.cilium.io/en/v1.16/observability/hubble/`, `docs.cilium.io/en/v1.16/operations/troubleshooting/`.

### For Scenario 6
- **Knowledge area**: socket-LB internals (cgroup-attached BPF at `connect()`/`sendmsg()`), `socketLB.hostNamespaceOnly`, interaction with iptables REDIRECT used by Istio's `istio-init`.
- **Depth bar**: Explains the order: cgroup connect4 fires before iptables in the pod netns, so socket-LB rewrites dest before Envoy capture *unless* hostNamespaceOnly. References the exact Helm key and what `cilium-dbg status` shows when configured.
- **Authoritative sources**: `docs.cilium.io/en/v1.16/network/kubernetes/kubeproxy-free/#socket-loadbalancer-bypass-in-pod-namespace`, Isovalent + Solo.io joint posts on Istio-on-Cilium, `bpf/sockops/` in cilium repo.

### For Scenario 7
- **Knowledge area**: cluster-pool IPAM operator (`pkg/ipam/clusterpool/`), CiliumNode CRD `spec.ipam.podCIDRs`, mask-size mutation semantics, exhaustion events.
- **Depth bar**: Cites the controller code path that makes per-node slices immutable, and the operator event/log line emitted on exhaustion.
- **Authoritative sources**: `docs.cilium.io/en/v1.16/network/concepts/ipam/cluster-pool/`, `pkg/ipam/allocator/clusterpool/` in cilium/cilium, CiliumNode CRD schema docs.

## What I would NOT want this expert to be

- Not a general K8s NetworkPolicy v1 expert — that's my lane; the expert's job is the *Cilium* delta (CNP/CCNP, DNS proxy, FQDN policy, identity model).
- Not a service-mesh expert — Istio internals (VirtualService, sidecar injection, mTLS PeerAuthentication) belong to whoever owns mesh; the Cilium expert only owns the *Cilium-side* surface where the two collide (socket-LB, transparent encryption vs mTLS double-encrypt, L7 policy vs Envoy ext-authz).
- Not a generic eBPF tutor — I don't need a CO-RE or libbpf primer; I need eBPF *as Cilium ships it* (the specific maps, the specific hooks, the specific cilium-dbg surface).
- Not a Hubble UI / Grafana dashboard author — visualization is observability-platform-engineer's lane; the Cilium expert owns the flow taxonomy and the drop-reason semantics, not the dashboard pixels.
- Not an AWS VPC / security-group expert — VPC peering, TGW, route tables stay with me; the Cilium expert weighs in only where the datapath choice (tunnel vs native, masquerade vs not) forces a VPC-side change.
- Not a Karpenter / node-lifecycle expert — node provisioning is k8s-capacity-management / platform-engineer; Cilium expert speaks to per-node CIDR slice behavior on scale-up, not to instance selection.
- Not a Sei-protocol / seid networking expert — `sei-network-specialist` owns CometBFT P2P and seid port semantics; the Cilium expert's job stops at the pod boundary.
