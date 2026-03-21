# Milestone 2 — Agent Review Runtime

**Owner:** Platform Engineer
**Phase:** 1
**Dependencies:** M0 (TideCouncil ABI), M1 (Operator creates Jobs, K8s manifests provide namespace/RBAC/secrets)

## Scope

Build the container image that executes a single design review cycle:

1. **Init container** — GitHub App token generation, repo clone, design document hash verification
2. **Main container** — LLM review via Anthropic Claude, structured JSON output, git push, EIP-712 attestation via KMS, on-chain `submitReview()` transaction

## Deliverables

| Spec | Output |
|------|--------|
| `lld-agent-review-runtime.md` | Python container image (init + main), Dockerfile, CI pipeline, unit + integration tests |

## Done Criteria

- Container runs as a K8s Job on testnet
- Produces structured JSON review and pushes to proposals repo
- Signs EIP-712 attestation via AWS KMS
- Submits `submitReview()` transaction to TideCouncil on arctic-1
- Writes `AgentResult` to `/dev/termination-log` (parseable by Operator)
- All exit codes (0, 1, 2, 10-52) exercised in test suite
