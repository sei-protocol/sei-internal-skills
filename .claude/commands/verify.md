---
description: "Verify all component specs and code are consistent with the interface registry"
allowed-tools: Read, Bash, Glob, Grep
---

# Verify Interface Consistency

Check that all design specs and implementation code are consistent with the interface registry.

## Steps

1. **Load the registry** from `tide/interface-registry.yaml`.

2. **For each interface boundary in the registry, check:**

### Event Signatures
- Compute the expected keccak256 topic hash from the canonical signature in the registry
- Check if the Operator's `pkg/constants/events.go` (or equivalent) has the matching hash
- Check if the Solidity contract emits events with the matching signature
- Verify indexed field positions match between contract and Go decoder

### Environment Variables
- Check that every REQUIRED env var in the registry is set in the Operator's Job builder code
- Check that every env var the runtimes read (via `os.Getenv` or equivalent) exists in the registry
- Check naming consistency

### Exit Codes
- Check that the runtime code can produce every exit code listed in the registry
- Check that the Operator handles every exit code (at minimum: success, config error, transient error, OOM, SIGTERM)

### Function Signatures
- Check that runtime code calls contract functions with the exact signatures in the registry
- Check parameter count and types

### K8s Resources
- Check ServiceAccount names in manifests match what the Operator uses in Job templates
- Check label values match between NetworkPolicy selectors and Job pod labels

3. **Report results** as a table:

   | Interface | Provider | Consumer | Status | Details |
   |-----------|----------|----------|--------|---------|

4. **Summary:** Count of COMPATIBLE, MISMATCH, MISSING. List any one-way door interfaces that have changed since last verification.

$ARGUMENTS
