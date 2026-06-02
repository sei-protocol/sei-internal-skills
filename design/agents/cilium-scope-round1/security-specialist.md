# Round 1 input — security-specialist

## Scenarios I'd dispatch to a Cilium expert

### Scenario 1: CiliumNetworkPolicy design for east-west containment on harbor
Harbor runs heterogeneous workloads (Sei nodes, Tide operator, agent runtimes in `tide-agents`, observability stack). A compromised agent pod must not be able to pivot to seid RPC, control-plane addons, secrets store CSI driver, or another tenant's namespace. I need a Cilium expert to design a default-deny CCNP baseline + per-namespace CNP overlays that close east-west pivots while still allowing the K8s service traffic the platform requires (kube-dns, kube-apiserver, webhook callbacks, secrets-store-csi sidecar IPC). I cannot reason about this without knowing the exact match-semantics of `endpointSelector` vs `nodeSelector` vs `entity: kube-apiserver`, and how Cilium resolves selectors when an identity has overlapping labels.

### Scenario 2: FQDN-based egress for agent runtimes — DNS spoof / TOFU threat model
Agent runtimes egress to GitHub (clone), AWS APIs (STS, S3, ECR), and a small allowlist of public package mirrors. FQDN policy is the natural fit, but I need to understand: where does the DNS proxy live, what TTL semantics does it enforce, can a compromised pod poison the cache for sibling pods, does Cilium accept the resolver answer at face value (DNS spoofing inside the cluster), and what happens if the workload uses DoH/DoT to bypass kube-dns. Also: how does FQDN policy interact with IP-based connection reuse (HTTP/2 coalescing, connection pools holding an old IP after the policy IP shifts).

### Scenario 3: Cilium security identity vs K8s NetworkPolicy podSelector — trust boundary
Cilium identities are derived from labels. If an attacker who can `kubectl label` a pod (or run a controller that does) can mint themselves into a privileged identity, the entire policy graph collapses. I need to know: which labels contribute to identity, how identity is computed and cached, whether label mutation is observed in real-time, and what RBAC posture closes label-driven identity escalation. Also: how does Cilium handle identity for pods in `hostNetwork: true`, and how does identity interact with the world / remote-node / unmanaged entities.

### Scenario 4: IMDS blocking and STS / IRSA token exfiltration surface
EKS workloads have AWS credentials reachable via IMDSv2 (169.254.169.254) and via the projected SA token used by IRSA / Pod Identity. A compromised pod that can hit IMDS can assume the node role; if it can read the projected token it can call STS directly. I want Cilium to enforce: (a) deny IMDS for everything except a tiny set of node-level daemons, (b) deny `sts.amazonaws.com` egress for pods that have no business calling STS, (c) ensure the policy is applied before the pod becomes ready (no race where the workload runs unpoliced for the first N seconds). The Cilium expert needs to tell me the policy enforcement ordering relative to pod start, and whether host-namespace traffic (kubelet, csi-driver) is subject to CNP at all.

### Scenario 5: Tetragon TracingPolicy for runtime exec / file / kill enforcement
Beyond network, Tetragon gives us syscall-level observability and enforcement. For agent runtimes I want kill-on-detect for: unexpected `execve` of shells, writes to `/proc/self/mem`, ptrace, `openat` of `/var/run/secrets/...` outside the expected process tree. I need an expert to design TracingPolicies that match attacker tradecraft without false-positive killing legitimate runtime behavior, plus the operational story: how policies are loaded, how to test them offline, how to roll back a bad policy without losing the eBPF program state.

### Scenario 6: Transparent encryption (WireGuard vs IPsec) — what threat does it actually mitigate
Cilium can encrypt node-to-node pod traffic with WireGuard or IPsec. I need a clear statement of: what traffic is in scope (pod-to-pod across nodes? host-to-host? service traffic through kube-proxy replacement?), what is out of scope (anything terminating in the host netns, anything to ClusterIP that resolves to the local node, anything going through hostPort), key rotation cadence, and the failure mode if a node's key is compromised. Without this I can't tell our security review that "intra-cluster traffic is encrypted" — the honest answer is probably "most of it, conditionally."

### Scenario 7: Hubble flow logs as security telemetry — what they capture, what they leak
Hubble emits L3/L4 and (with the proxy) L7 flow data. As a SIEM source this is gold for east-west detection. But it can also leak: HTTP paths with tokens in query strings, DNS queries that betray internal hostnames, identity labels that include tenant-identifying data. I need the expert to specify the redaction surface (what's configurable, what isn't), retention defaults, and the auth model for the Hubble Relay / UI (who can query what, can a tenant see another tenant's flows).

### Scenario 8: GitHub Actions runner egress — Cilium policy for self-hosted runners
If we run self-hosted runners on harbor, fork-PR jobs are hostile-by-default. Cilium policy must constrain them to: GitHub Actions endpoints, ghcr/ecr for image pulls, nothing else. This is the egress-firewall use case but with a known-malicious workload assumption. I want the expert to design the policy and tell me what the runner needs that I'd naively block (e.g., dynamic GitHub IP ranges, action authors' arbitrary URLs).

## Required depth per scenario

### For Scenario 1 (CNP design)
- **Knowledge area**: CNP / CCNP CRD semantics, `endpointSelector`, `toEndpoints`, `toServices`, `toEntities`, policy ordering, default-deny semantics, audit mode.
- **Depth bar**: Must have written a default-deny baseline in production and debugged at least one "policy looks right but traffic still drops" incident.
- **Sources**: Cilium docs `gettingstarted/policy-creation`, `policy/language`, `policy/intermediate/`, Isovalent labs on CNP.

### For Scenario 2 (FQDN egress)
- **Knowledge area**: `toFQDNs`, DNS proxy architecture (`--tofqdns-*` flags), DNS TTL handling, interaction with kube-dns / coredns / NodeLocal DNSCache.
- **Depth bar**: Can explain the proxy's failure modes from memory and has tuned `--tofqdns-min-ttl` / `--tofqdns-dns-reject-response-code` in anger.
- **Sources**: Cilium docs `policy/language/#dns-based`, Isovalent blog on FQDN policy internals, CVE history on the DNS proxy.

### For Scenario 3 (identity / label boundary)
- **Knowledge area**: Cilium identity model, `identity-allocation-mode` (CRD vs kvstore), label sources (`k8s`, `container`, `reserved`), `--labels` filter.
- **Depth bar**: Can walk through what happens when a label changes on a running pod, including stale-identity windows.
- **Sources**: Cilium `concepts/security/identity`, `operations/troubleshooting` identity sections.

### For Scenario 4 (IMDS / IRSA)
- **Knowledge area**: CNP with CIDR egress rules, host-firewall mode, policy enforcement ordering relative to pod readiness.
- **Depth bar**: Has shipped IMDS blocking on EKS and validated it with a side-channel test (curl from inside a pod that should not have access).
- **Sources**: AWS EKS best-practices guide on IMDS, Cilium `gettingstarted/host-firewall`, Isovalent blog on EKS + Cilium.

### For Scenario 5 (Tetragon)
- **Knowledge area**: TracingPolicy CRD, kprobe / tracepoint selection, in-kernel filters, enforcement (`Sigkill`, `Override`), policy lifecycle.
- **Depth bar**: Has written and tested a TracingPolicy that kills a process based on a syscall arg match, and knows the perf cost.
- **Sources**: Tetragon docs, `cilium/tetragon` examples dir, Isovalent's "Tetragon in production" content.

### For Scenario 6 (transparent encryption)
- **Knowledge area**: `enable-wireguard`, `encryption.type`, traffic-in-scope matrix, kube-proxy replacement interaction.
- **Depth bar**: Can state the exact list of traffic NOT encrypted under each mode without looking it up.
- **Sources**: Cilium `network/encryption-wireguard`, `network/encryption-ipsec`, the (in)famous GitHub issues debating what's actually encrypted.

### For Scenario 7 (Hubble telemetry)
- **Knowledge area**: Hubble Relay, flow filter config, L7 visibility (proxy required), retention, mTLS between Relay and observers.
- **Depth bar**: Has deployed Hubble Relay multi-tenant and configured RBAC on it.
- **Sources**: Cilium `observability/hubble`, Hubble Relay auth docs.

### For Scenario 8 (Actions runners)
- **Knowledge area**: CNP egress with FQDN + CIDR, GitHub endpoint inventory, runner network requirements.
- **Depth bar**: Has constrained self-hosted runners with network policy in production.
- **Sources**: GitHub docs on runner network requirements, Cilium FQDN guide.

## What I would NOT want this expert to be
- Not a general AppSec / pen-test expert — that's my lane; I'll bring the threat model.
- Not a kernel / eBPF internals expert — separate role; I only need them to know which Cilium knobs are safe and which are sharp.
- Not a generic Kubernetes networking person — `network-specialist` covers L2/L3, BGP, MTU, kube-proxy replacement basics. I want this expert deep in the *policy and identity* surface, not "how does Cilium route packets."
- Not a service mesh / mTLS expert in the Istio/Linkerd sense — Cilium's mTLS story is its own thing; I want them to know Cilium's, not to re-architect us onto Istio.
- Not a compliance/audit-letter writer — I need technical answers, not SOC2-friendly prose.
- Not the SRE who owns the Cilium upgrade runbook — `sre-engineer` owns operational lifecycle; this expert advises on security configuration.
