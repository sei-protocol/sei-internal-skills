---
description: "Cross-review a design doc or code against the interface registry and consuming/providing specs"
allowed-tools: Read, Bash, Glob, Grep, Agent
---

# Cross-Review

Review a specific design document or code file for interface consistency with all components it touches.

## Input
The user provides a path to a design document, spec file, or code directory.

## Steps

1. **Read the interface registry** at `tide/interface-registry.yaml`.

2. **Identify boundaries.** Determine which interface boundaries this component participates in (as provider or consumer). Use the cross-component interfaces section of the registry.

3. **For each boundary**, dispatch the reviewer agent with:
   - The file being reviewed
   - The counterpart spec or code
   - The relevant registry entries

4. **Collect findings** and present them in a summary table:

   | # | Interface | Status | Details | Owner to Fix | Severity |
   |---|-----------|--------|---------|-------------|----------|

5. **For CRITICAL/HIGH findings**, propose specific fixes and identify which files need to change.

6. **For one-way door interfaces** (event signatures, storage layout, CRD spec fields, EIP-712 types), flag explicitly even if COMPATIBLE — these deserve extra scrutiny.

$ARGUMENTS
