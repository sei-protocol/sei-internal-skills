# Tide Agent Council

This repository uses an AI agent council for design review and implementation. Three specialized agents collaborate on the Tide system, each with deep domain expertise.

## Council Members

| Agent | Domain | Owns |
|-------|--------|------|
| **blockchain-developer** | Solidity, Foundry, ERC standards, gas optimization, security | `design/milestones/m0-contracts/` |
| **kubernetes-specialist** | Go, controller-runtime, CRDs, EKS, event indexing | `design/milestones/m1-platform/lld-tide-operator.md` |
| **platform-engineer** | Python, container runtimes, Kustomize, RBAC, observability | `design/milestones/m1-platform/lld-k8s-manifests.md`, `design/milestones/m2-review/`, `design/milestones/m3-execution/` |
| **network-specialist** | NetworkPolicies, ingress, DNS, service mesh, cloud networking, runner isolation | `manifests/base/runners/network-policy.yaml`, `manifests/base/network-policies.yaml` |
| **security-specialist** | Threat modeling, adversarial design, crypto protocols, contract auditing, supply chain security | Cross-cutting — reviews all components |
| **tee-specialist** | Nitro Enclaves, SGX/TDX, remote attestation, enclave-to-chain bridges, confidential computing | TEE integration, attestation verification contracts |

## Working Agreement

All design work follows the constitution at `design/constitution/constitution.md`. Key principles:

1. **Two-Way Doors Only** — every decision must be reversible without significant rework
2. **YAGNI** — if it's not required by Phase 0–2 business needs, exclude it
3. **Interfaces First** — exact signatures, types, events, env vars, exit codes
4. **Errors Are Interface** — every error condition is documented
5. **Tests Prove Interfaces** — concrete test specs for every interface

## Cross-Component Interfaces

These interfaces are the most critical contracts between teams. The **provider owns** the interface; consumers adapt.

| Interface | Provider | Consumer(s) |
|-----------|----------|-------------|
| On-chain events (topic hashes, indexed fields) | Solidity contracts | Tide Operator |
| K8s Job env vars, volume mounts, exit codes | Tide Operator | Agent runtimes |
| Completion signaling (termination messages) | Agent runtimes | Tide Operator |
| EIP-712 type hashes, domain separator | TideCouncil contract | Review runtime |
| `/secrets` file layout | K8s SecretProviderClass | Agent runtimes |

## Canonical Env Var Names

Runtime convention wins (runtimes are the consumers):

- `TIDE_KMS_KEY_ARN` — not KEY_ID
- `TIDE_COUNCIL_CONTRACT` — not ADDRESS
- `TIDE_ACP_CONTRACT` — not ADDRESS
- `TIDE_GITHUB_INSTALLATION_ID` — not APP_INSTALLATION_ID

## How to Use These Agents

Agent persona files live in `.claude/agents/`. When working on a Tide component, invoke the relevant agent for domain-specific guidance:

- **Smart contract work** → blockchain-developer
- **Operator / CRD / controller work** → kubernetes-specialist
- **Runtime containers / K8s manifests / infra** → platform-engineer

For cross-cutting design decisions, convene all three agents and follow the cross-review protocol in the constitution.
