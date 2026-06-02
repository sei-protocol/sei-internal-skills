# Round 1 input — kubernetes-specialist

## Scenarios I'd dispatch to a Cilium expert

These are the moments where my controller code or the workloads my CRDs spawn collide with Cilium-specific semantics that I cannot read off the standard K8s API.

### Scenario 1: Controller informer/watch traffic under CiliumNetworkPolicy default-deny
The Tide Operator and sei-k8s-controller both maintain long-lived watch connections to `kube-apiserver` via controller-runtime managers. On harbor, if a default-deny CiliumNetworkPolicy lands in the controller namespace (or the apiserver-facing path), watches go silent without surfacing as a Go error — they just stop receiving events. I need an expert to tell me the exact CNP egress shape required for controller-manager pods (apiserver `:443`, kube-dns, optionally webhook/metrics callers) and whether `endpointSelector: {}` policies in `kube-system` can shadow it. Also: what failure mode to expect (TCP RST vs. silent drop vs. `dropped by policy` Hubble trace) and how that surfaces (or doesn't) to client-go.

### Scenario 2: Headless Service + StatefulSet pod-to-pod traffic for SeiNode P2P
`SeiNode` workloads are StatefulSets with a headless Service for CometBFT P2P (port 26656) and Geth devp2p (30303). Pods address each other by `pod-name.svc` DNS. Under Cilium with kube-proxy replacement (KPR), headless service semantics are subtly different from iptables mode — specifically around how Cilium synthesizes identities for pods that haven't been observed by the local agent yet, and how that interacts with rolling restarts where pod IPs churn. I need an expert to confirm: does KPR change anything for headless services (it shouldn't, but I want it stated), and what's the identity-allocation latency between pod-ready and first-allowed-egress.

### Scenario 3: Authoring CiliumNetworkPolicy from inside the controller
The controller may need to emit per-`SeiNode` CNPs (e.g., "this archive node may only egress to the snapshotter S3 endpoint and the cluster's chain peers"). I need expert guidance on:
- L7 HTTP rules vs. L3/L4 — when L7 is worth the proxy hop for our use case (probably never for P2P, possibly for control-plane HTTP to the Operator)
- `toFQDNs` semantics — TTL handling, DNS-proxy implications, what happens if the DNS proxy is down
- Whether to use `CiliumNetworkPolicy` (namespaced) or `CiliumClusterwideNetworkPolicy` for cross-namespace rules (Operator -> agent namespace)
- Policy ordering and how rule conflicts resolve

### Scenario 4: Job lifecycle and identity GC
The Tide Operator launches K8s Jobs in `tide-agents` per on-chain event. Each Job pod gets a short-lived Cilium identity. At our expected throughput (dozens of Jobs/hour, each <10 min), I want to confirm we don't blow through identity slots (default 64k cluster-wide) or hit identity-allocation lag on cold starts. I also need to know what happens when a Job pod terminates mid-reconcile — is the identity reused immediately, and could that cause a CNP allow-rule to briefly apply to a different workload?

### Scenario 5: NodePort / LoadBalancer + KPR semantics for RPC ingress
SeiNode RPC (port 8545/26657) is exposed via Service. Under Cilium KPR, `externalTrafficPolicy: Local` and `Cluster` behave the same as iptables in principle, but DSR (Direct Server Return) and Maglev hashing change source-IP preservation and connection affinity. For our load balancer (AWS NLB -> NodePort), I need an expert to tell me: do we get client source IP, what's the failover behavior on pod restart, and does Cilium's session affinity work with our long-lived JSON-RPC WebSocket connections.

### Scenario 6: Observability — Hubble for debugging "why is my pod's traffic dropped"
When a SeiNode pod can't reach a peer or the operator can't reach apiserver, I want a checklist for using Hubble (`hubble observe --verdict DROPPED --from-pod ...`) to root-cause. This is the thing that turns a 4-hour incident into a 10-minute one. I want the expert to codify the standard triage flow as a runbook fragment we can paste into incident response.

### Scenario 7: EKS-specific Cilium gotchas
We run on EKS with the VPC CNI replaced (or chained) by Cilium. Known sharp edges: ENI-mode IP allocation, security-group-per-pod compatibility, IRSA traffic paths, and the `aws-node` DaemonSet still running but neutered. I want an expert to confirm our install mode (ENI vs. overlay/VXLAN vs. native routing) and call out any EKS-version-specific upgrade traps that bite controllers (pod restart storms when Cilium agent restarts).

## Required depth per scenario

### For Scenario 1 (CNP + controller watches)
- **Knowledge area:** CiliumNetworkPolicy authoring, identity model, default-deny semantics
- **Depth bar:** Can write a working CNP from scratch for a controller pod and explain why each rule is there; can read Hubble drops and map back to the missing rule
- **Sources:** Cilium docs `Network Policy` section, `cilium policy trace`, `cilium endpoint get`

### For Scenario 2 (headless + StatefulSet + KPR)
- **Knowledge area:** kube-proxy replacement internals, service backend selection, endpoint slice handling
- **Depth bar:** Knows the difference between socket-LB and per-packet LB modes, knows how Cilium handles `ClusterIP: None`, can explain identity-allocation timing relative to pod readiness
- **Sources:** Cilium KPR docs, `cilium service list`, `cilium bpf lb list`

### For Scenario 3 (authoring CNPs from operator)
- **Knowledge area:** CNP CRD schema, L7 proxy (Envoy), FQDN policy
- **Depth bar:** Has shipped operator-managed CNPs to production; knows the failure modes of `toFQDNs`; can articulate when L7 is overkill
- **Sources:** Cilium CRD reference, FQDN policy design doc

### For Scenario 4 (identity lifecycle for Jobs)
- **Knowledge area:** Identity allocation, GC, CRD-based identity (`CiliumIdentity`)
- **Depth bar:** Knows the identity cap, GC interval, and whether identity reuse is racy
- **Sources:** Cilium identity-management docs, `cilium identity list`

### For Scenario 5 (NodePort/LB + KPR)
- **Knowledge area:** DSR, Maglev, `externalTrafficPolicy`, session affinity
- **Depth bar:** Can answer "do we lose source IP" definitively for our NLB config; knows WebSocket affinity behavior
- **Sources:** Cilium load-balancing docs, NLB + target-type IP interaction

### For Scenario 6 (Hubble triage)
- **Knowledge area:** Hubble CLI, flow filtering, verdict semantics
- **Depth bar:** Operator-level — can produce a 5-step "my pod can't talk, what do I run" runbook
- **Sources:** Hubble docs, `hubble observe` reference

### For Scenario 7 (EKS specifics)
- **Knowledge area:** Cilium-on-EKS install modes, VPC CNI chaining vs. replacement, upgrade procedure
- **Depth bar:** Has done a Cilium upgrade on a production EKS cluster without dropping controller traffic
- **Sources:** Cilium EKS install guide, AWS VPC CNI compatibility notes, release notes for the version harbor runs

## What I would NOT want this expert to be

- **Not a controller-runtime expert** — informer/workqueue/leader-election internals are my lane.
- **Not a CRD-schema designer for our domain CRDs** — SeiNode/SeiNodeDeployment schema is mine; the expert advises on CNP authoring, not on whether to add a `networkPolicy` field to our spec.
- **Not a Kubernetes-networking generalist** — I want Cilium-specific depth (eBPF datapath, identity model, KPR, Hubble). Generic `NetworkPolicy` and Service knowledge I already have.
- **Not a service-mesh expert** — Istio/Linkerd/Cilium Service Mesh comparisons are out of scope unless harbor adopts Cilium Service Mesh (it hasn't).
- **Not a capacity planner** — node sizing, agent resource requests, eBPF map sizing math belongs to `k8s-capacity-management`. The Cilium expert tells us *what knobs exist*; capacity tells us *what values*.
- **Not a security-policy author** — security-specialist owns the threat model and what we're trying to prevent. Cilium expert translates that intent into CNP syntax.
- **Not an SRE/incident-response generalist** — `sre-engineer` owns runbook framing; the Cilium expert contributes the Hubble/eBPF-specific paragraphs.
