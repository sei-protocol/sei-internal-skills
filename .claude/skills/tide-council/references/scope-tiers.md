# Scope Tier Processes

Each tier defines the exact sequence of steps the coordinator runs. The goal is to match process weight to risk — bigger scope means more components, more interfaces, more chances for mismatches, so more review.

---

## Tier 1: Product (New MVP / Major Subsystem)

**When:** Building something that doesn't exist yet. Multiple new components, new interfaces, new deployment artifacts.

**Duration:** Days to weeks of agent work. Multiple sessions likely.

**Process:**

### Phase 0: Resume Check
If `.tide/workstream.yaml` exists with this effort, resume from the last in_progress phase. Tell the user what's already done and what's next.

### Phase 1: High-Level Design
1. Read the constitution and interface registry
2. Draft a high-level design document covering:
   - Purpose and business need (must trace to Phase 0-2)
   - Component decomposition — which new components are needed
   - Interface map — which components talk to each other, who provides what
   - Deployment model — how it runs (K8s Jobs, Deployments, CronJobs, etc.)
   - Security model — credentials, RBAC, network policies, blast radius
3. Save to `design/high-level/{product-name}.md`
4. Present to user for review before proceeding

### Phase 2: Component Design (repeat per component)
For each component identified in Phase 1, run the Component tier process below. Order matters — design providers before consumers so interfaces are defined when consumers need them.

Recommended ordering:
1. On-chain contracts first (they define event and function signatures)
2. Kubernetes resources second (CRDs, manifests — they define the runtime environment)
3. Application code last (operator controllers, agent runtimes — they consume everything above)

### Phase 3: Cross-Review
After all component LLDs are drafted:
1. For each interface boundary in the interface registry, dispatch the `reviewer` agent
2. Collect all findings into a cross-review document
3. Save to `design/cross-reviews/cross-review-{product-name}.md`
4. Resolve all MISMATCH and MISSING findings:
   - Update the interface registry first
   - Then update the component LLDs to match

### Phase 4: Implementation
Run the Feature tier process for each component, in the same provider-first order.

### Phase 5: Integration Verification
1. Run the `reviewer` across ALL interface boundaries one final time
2. Run all tests (`go test ./...`, `pytest`, `forge test`)
3. Present the full summary to the user

**Checkpointing:** Write `.tide/workstream.yaml` after completing each phase. Product tier will always span sessions — the checkpoint is critical. On completion, archive to `.tide/archive/`.

**Escalation handling:** If a specialist files an escalation during Phase 4 (implementation), the coordinator pauses implementation, assesses the escalation, resolves it (potentially updating the LLD and registry), then resumes. The workstream file records this.

---

## Tier 2: System (Multi-Component Feature)

**When:** A feature that spans multiple existing components. Existing interfaces may need to change or new interfaces may be added.

**Duration:** Hours to a day of agent work.

**Process:**

### Phase 1: Impact Analysis
1. Read the constitution and interface registry
2. Identify which components are affected
3. For each affected component, identify which interfaces are touched
4. Classify each interface change:
   - **New interface** — doesn't exist yet, needs to be added to registry
   - **Modified interface** — exists but needs changes (check for one-way doors!)
   - **Unchanged** — interface exists and isn't affected
5. Present the impact analysis to the user, highlighting any one-way doors

### Phase 2: Interface Registry First
1. Draft all interface changes in the registry
2. For new interfaces: add the full entry (provider, consumer, types, ownership)
3. For modified interfaces: show before/after, flag one-way doors
4. Get user approval for one-way door changes before proceeding

### Phase 3: Design Updates
For each affected component:
1. Dispatch the owning specialist to update the LLD
2. The specialist reads the updated registry and adjusts their spec
3. If the change is significant enough to warrant a new LLD section, add it
4. If it's a small change, update the existing LLD in place

### Phase 4: Cross-Review
Dispatch the `reviewer` for every interface boundary that was touched. Focus the review on the changed interfaces — don't re-review unchanged boundaries unless the user asks.

### Phase 5: Implementation
For each affected component, dispatch the owning specialist to implement the changes. Provider-first ordering.

### Phase 6: Verification
1. Run the `reviewer` across changed interfaces
2. Run tests for all affected components
3. Present summary with interface change log

**Checkpointing:** Write `.tide/workstream.yaml` after Phases 1, 2, and 4. System tier may span sessions if many components are affected. On completion, archive.

**Escalation handling:** Same as Product tier — pause, assess, resolve, resume.

---

## Tier 3: Component (Single-Component Feature)

**When:** A new feature or significant change within a single component. May touch interfaces at the boundary but doesn't require coordinated changes across multiple specialists.

**Duration:** 30 minutes to a few hours.

**Process:**

### Phase 1: Scoping
1. Read the interface registry entries for this component
2. Read the existing LLD if one exists
3. Determine: does this change touch any interface boundary?
   - If yes: check if the interface change is on the provider or consumer side
   - If it's a provider-side change: this might actually be a System tier — re-assess
   - If it's a consumer-side change: the provider's interface is already defined, just implement against it

### Phase 2: Design
1. Dispatch the owning specialist to draft or update the LLD section for this feature
2. The specialist reads the interface registry for all boundaries they participate in
3. LLD follows the constitution's template (all sections, no TBD)

### Phase 3: Interface Check
1. Dispatch the `reviewer` to check the new/updated LLD against the registry
2. If MISMATCH: resolve before implementation
3. If COMPATIBLE: proceed

### Phase 4: Implementation
1. Dispatch the owning specialist to write code
2. Write tests that verify interface contracts, not just internal logic
3. Run existing tests to catch regressions

### Phase 5: Verification
Quick check: does the implementation match the LLD and registry? Run tests. Present results.

---

## Tier 4: Feature (Implementation Only)

**When:** The design is done, the interfaces are defined, just write the code. This is the lightest-weight process — suitable for well-defined tasks where the LLD already exists.

**Duration:** Minutes to an hour.

**Process:**

### Phase 1: Context Loading
1. Read the LLD for this feature
2. Read the interface registry entries for interfaces this code touches
3. Read existing code in the relevant package/module to understand patterns

### Phase 2: Implementation
Dispatch the owning specialist with:
- The specific task description
- The LLD section to implement against
- The interface registry entries to conform to
- The existing code patterns to follow

### Phase 3: Verification
1. Run tests (`go test ./...`, `pytest`, `forge test` as appropriate)
2. Quick interface spot-check: do the types/names in the code match the registry?
3. Present: files modified, tests passed/failed, any concerns

---

## Tier Selection Heuristics

When the coordinator isn't sure which tier applies, these heuristics help:

| Signal | Likely Tier |
|--------|------------|
| "Build a new X from scratch" | Product |
| "We need X to work with Y end-to-end" | System |
| "Add X to the operator" | Component |
| "Implement what's in the LLD" | Feature |
| Multiple components mentioned | System or Product |
| Single component, no existing LLD | Component |
| Single component, LLD exists | Feature |
| "Design" or "architect" in the ask | Product or System |
| "Code" or "implement" in the ask | Feature or Component |
| Interface changes needed | System (if multi-component) or Component (if single) |
| No interface changes | Feature |

When in doubt, ask one question: "Does this change require coordinated updates across multiple components?" If yes → System or Product. If no → Component or Feature.
