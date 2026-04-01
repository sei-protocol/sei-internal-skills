---
description: "Spin up an independent workstream with the right specialist team for a small-medium effort"
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, Agent
---

# Independent Workstream

Spin up a focused team of agents to handle a small-medium engineering effort end-to-end.

## Input
The user describes the effort: what needs to be built, changed, or investigated.

## Steps

1. **Scope assessment.** Read the interface registry and constitution. Determine:
   - Which components are touched
   - Which specialists are needed
   - Which interface boundaries are affected
   - Whether any one-way doors are involved

2. **Create a brief.** Write a focused document with:
   - Objective (one sentence)
   - Components affected
   - Interface boundaries touched
   - Done criteria
   - One-way door checklist (if any)

3. **Dispatch specialists.**
   - If components have no interface dependencies between them, dispatch specialists in parallel
   - If there ARE interface dependencies, dispatch the provider first, then the consumer
   - Each specialist gets: the brief, the interface registry, and their relevant LLD

4. **Cross-check.** After specialists complete their work:
   - Run the reviewer agent across all interface boundaries touched
   - Verify no mismatches were introduced

5. **Report.** Present:
   - Files created/modified
   - Interface changes (if any — registry must be updated)
   - Test results
   - Any one-way door decisions requiring human approval

$ARGUMENTS
