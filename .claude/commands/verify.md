---
description: "Verify all component specs and code are consistent with the interface registry"
allowed-tools: Read, Bash, Glob, Grep
---

# Verify Interface Consistency

Check that all design specs and implementation code are consistent with `tide/interface-registry.yaml`.

## Steps

1. **Run the automated checker first:**
   ```bash
   python scripts/verify_registry.py
   ```
   Covers the mechanical checks: env var names, ServiceAccount patterns, function names, file paths. Exit code: `0` = pass, `1` = mismatches (printed to stdout), `2` = registry missing or parse error. Capture the stdout — the failing cases go straight into the report.

2. **Cover what the script doesn't.** For each interface boundary in the registry, verify:

### Event Signatures
- Compute the expected keccak256 topic hash from the canonical signature in the registry
- Check if the Operator's `pkg/constants/events.go` (or equivalent) has the matching hash
- Check if the Solidity contract emits events with the matching signature
- Verify indexed field positions match between contract and Go decoder

### Exit Codes
- Check that the runtime code can produce every exit code listed in the registry
- Check that the Operator handles every exit code (at minimum: success, config error, transient error, OOM, SIGTERM)

### K8s Resources
- Check ServiceAccount names in manifests match what the Operator uses in Job templates
- Check label values match between NetworkPolicy selectors and Job pod labels

3. **Report results** as a table:

   | Interface | Provider | Consumer | Status | Details |
   |-----------|----------|----------|--------|---------|

4. **Summary:** pass/fail of `verify_registry.py`, plus count of COMPATIBLE / MISMATCH / MISSING for the manual checks. List any one-way door interfaces that have changed since last verification.

$ARGUMENTS
