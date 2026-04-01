---
description: "Generate implementation code from a low-level design spec, consistent with the interface registry"
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, Agent
---

# Implement from Spec

Generate production code from a low-level design document, ensuring all interfaces match the registry.

## Input
The user provides a path to an LLD or a description of what to implement.

## Steps

1. **Read the spec.** Load the LLD from `design/milestones/`.

2. **Read the interface registry** at `tide/interface-registry.yaml`. Identify every interface this component provides or consumes.

3. **Identify the owner.** Dispatch the appropriate specialist agent to write the implementation:
   - Go code (Operator) -> kubernetes-specialist
   - Python code (runtimes) -> platform-engineer
   - Solidity code (contracts) -> blockchain-developer

4. **Implementation requirements for the specialist:**
   - All interface types, signatures, and names MUST match the registry exactly
   - All error cases from the LLD must be handled
   - All exit codes must match the registry
   - Write tests that verify the interface contracts (not just internal logic)
   - Follow the code conventions in CLAUDE.md

5. **Post-implementation check:**
   - Run existing tests (`go test ./...`, `pytest`, `forge test` as appropriate)
   - Dispatch the reviewer agent to verify the implementation matches the registry
   - Flag any deviations

6. **Output:** Present the files created/modified and any test results.

$ARGUMENTS
