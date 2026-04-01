---
description: "Run a full design cycle for a Tide component: draft, cross-review, resolve"
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, Agent
---

# Full Design Cycle

Run the complete Tide design collaboration protocol for the specified component.

## Input
The user provides a component name or description of what needs to be designed.

## Steps

1. **Read the constitution** at `design/constitution/constitution.md` and the interface registry at `tide/interface-registry.yaml`.

2. **Identify the owner.** Match the component to the component inventory table in the constitution. The owner's specialist agent drafts the LLD.

3. **Round 1 — Draft.** Dispatch the owning specialist agent to draft or update the LLD:
   - The specialist MUST read the interface registry first
   - The LLD follows the template in the constitution (all sections required)
   - Every interface uses exact types, no TBD
   - Every feature traces to a Phase 0-2 business need

4. **Round 2 — Cross-Review.** For each interface boundary the component touches:
   - Identify the other component(s) involved
   - Dispatch the reviewer agent with both specs and the registry entry
   - Collect findings (COMPATIBLE / MISMATCH / MISSING)

5. **Round 3 — Resolution.** For each MISMATCH or MISSING finding:
   - Determine who changes (provider owns the interface)
   - Update the interface registry first
   - Then dispatch the relevant specialist(s) to update their specs

6. **Verify.** Run the `/verify` command to check consistency.

7. **Report.** Summarize what was designed, what mismatches were found and resolved, and any one-way door decisions that need human approval.

## One-Way Door Gate
If the design involves any one-way doors (event signatures, storage layout, CRD spec fields, EIP-712 type hashes), STOP and present these to the user for explicit approval before finalizing.

$ARGUMENTS
