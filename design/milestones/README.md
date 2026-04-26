# Milestones

Each milestone delivers a runnable subsystem. LLDs (low-level designs) follow the template in `../constitution/constitution.md`.

| Milestone | Component(s) | Owner | Depends on | LLDs |
|-----------|--------------|-------|------------|------|
| **[M0 — Smart Contracts](m0-contracts/)** | TideCouncil, TideJobHook, deployment suite | Blockchain Developer | — | `lld-tide-council.md`, `lld-tide-job-hook.md`, `lld-contract-deployment.md` |
| **[M1 — Platform & Operator](m1-platform/)** | K8s manifests, Tide Operator | K8s Specialist + Platform Engineer | M0 (ABIs + addresses) | `lld-tide-operator.md`, `lld-k8s-manifests.md` |
| **[M2 — Review Runtime](m2-review/)** | Agent review container | Platform Engineer | M0, M1 | `lld-agent-review-runtime.md` |
| **[M3 — Execution Runtime](m3-execution/)** | Agent execution container | Platform Engineer | M0, M1, M2 (shared protocols) | `lld-agent-execution-runtime.md` |
| **[MVP](mvp/)** | Event-driven GHA path on self-hosted runners (parallel track) | Brandon (sole operator) | M0 contracts deployed | `setup-runbook.md`, `chain-indexer-and-agent-containers.md`, `tekton-pipeline.md`, `contract-deployment-and-wallets.md`, `github-app-setup.md` |

## How to read an LLD

Every LLD answers the same questions in the same order, per the constitution's template:

1. **Purpose** — which Phase 0–2 business need does this serve?
2. **Dependencies** — what it consumes (with exact interface references) and what it explicitly does NOT consume
3. **Interface Specification** — every function, event, type, env var, error condition (no "TBD")
4. **State Model** — what state lives where, how it transitions (Mermaid `stateDiagram-v2`)
5. **Internal Design** — enough for a mid-level engineer to implement
6. **Error Handling** — every error case and how it's surfaced
7. **Test Specification** — concrete test cases with setup, action, expected result
8. **Deployment** — testnet/mainnet differences
9. **Deferred** — features explicitly excluded with one-line rationale tracing to YAGNI

## Cross-cutting interfaces

The contracts that span milestones — event signatures, env vars, exit codes, EIP-712 types, K8s resources — live in `tide/interface-registry.yaml`, not in any individual LLD. **When an LLD and the registry disagree, the registry wins.**

Cross-component review artifacts live in `../cross-reviews/`:

- `cross-review-blockchain.md` — TideCouncil + TideJobHook against the Operator's indexer
- `cross-review-operator.md` — Operator against the runtimes
- `cross-review-platform.md` — K8s manifests against the Operator
- `cross-review-mvp.md` — MVP path against M0–M3

## MVP vs M1–M3

The `mvp/` track is a parallel **event-driven path** that uses Claude Code headless on self-hosted GitHub Actions runners. It shares the M0 contracts but bypasses the K8s Operator + Python runtimes — useful for fast iteration toward a first end-to-end on-chain `ProposalApproved` event.

M1–M3 remain the path to a production-grade runtime (K8s Operator + per-agent ServiceAccounts + KMS-backed signing + granular exit codes).
