# Milestone 3 — Agent Execution Runtime

**Owner:** Platform Engineer
**Phase:** 2
**Dependencies:** M0 (ERC-8183 ABI), M1 (Operator creates Jobs), M2 (shared completion signaling protocol)

## Scope

Build the container image that executes a funded coding task:

1. **Init container** — GitHub App token generation, upstream repo clone, workspace branch creation, task spec loading
2. **Main container** — Coding agent framework (OpenHands default), iterative edit-test-fix cycle, git push, PR creation, deliverable hash submission via `submit()` on ERC-8183

## Deliverables

| Spec | Output |
|------|--------|
| `lld-agent-execution-runtime.md` | Python container image (init + main), Dockerfile, CI pipeline, unit + integration tests |

## Done Criteria

- Container runs as a K8s Job on testnet
- Clones upstream, creates workspace branch, runs coding agent
- Pushes code and opens PR to deliverables repo
- Submits `submit(jobId, deliverableHash)` on-chain
- Writes `AgentResult` to `/dev/termination-log`
- SIGTERM handler gracefully pushes partial work
- OpenHands and custom framework modes both functional
