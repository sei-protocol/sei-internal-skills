# The lifecycle and its composition

`workstream` walks the Coral lifecycle, **invoking** the existing skills at the right seams and inserting checkpoints between them. It does not reimplement any of them, and it does not auto-drive them — the operator still confirms phase transitions; the skill's job is to sequence the composition and enforce the gates.

## The seams (who is invoked, in order)

| Phase | Skill invoked | Checkpoint at the seam |
|---|---|---|
| 1. Declare | — | write + surface the ledger |
| 2. Scope-tier design | `/council` (owns the 4 scope tiers; produces design content) | — |
| 3. Verify — **iterate to convergence** | `/xreview` (blinded, assigned-dissent) — invoked in a loop: dispatch → synthesize ledger → fix → re-invoke (appends a round) → repeat until a passing terminal or `OPEN-BLOCKED` | — (the convergence record is `/xreview`'s review-ledger; a `review-gate` reads it at ship) |
| 4. Capture | `/design` (writes the reviewed design as a durable doc) | — |
| 5. Dual-audience pass, then sign off | `/lingua` (on the captured design **+ acceptance criteria** — fidelity guardrails = anti-drift at authoring) | **`design-approval`** |
| 6. Implement | (the work itself) | (council's one-way-door gate fires here for a one-way-door **change category** — persisted schema/field names, wire/on-disk formats, signed/indexed IDs — gated on category, not deployment target) |
| 7. Capture deferred work | `/issue` — **only if a slice was cut** | — |
| 8. Decorate lineage | `/execution-plan` (delegated; never duplicated) | (execution-plan's first-label confirm is its own gate) |
| 9. Ship | — | **`pr-sign-off`** (human) **or** **`review-gate`** (consensus — reads `/xreview`'s review-ledger fail-closed + confirms declared checks green; operator chooses at declaration). **Plus the pre-merge drift check** when the captured design carries acceptance criteria — criteria-compliance (a Missing criterion blocks; `inconclusive ⇒ surface`) + design-staleness (surfaced, non-blocking); a *facet* of this seam, not a new gate kind (see SKILL.md "The pre-merge drift check") |

## Ordering rules (load-bearing)

- **`/xreview` precedes `/design` capture.** `/design` records what a session decided; it does not review the design content. So verify first (xreview), then capture (design). The `design-approval` checkpoint sits *after* capture — you can't sign off on a design that doesn't exist yet.
- **Verify is a convergence loop, not a single pass.** Step 3 re-invokes `/xreview` (which appends a new review-ledger round) after each fix until the latest round is a passing terminal or `OPEN-BLOCKED`. The workstream sequences the rounds and applies fixes; it **composes** `/xreview` and **reads** its review-ledger — it never reimplements the slate, the dispatch, or the ledger schema (those are PLT-535's). A declared `review-gate` at ship reads that same ledger fail-closed as its done-evidence. See `review-gate.md`.
- **`/lingua` precedes `design-approval`.** The `/lingua` dual-audience pass runs at seam 5 on the captured design *before* the human signs off — **unconditionally** (it is the prose lens on every captured design, criteria or not); when the design carries **acceptance criteria**, those are the highest-stakes part of the pass. Its fidelity guardrails (no invented commitment; an unsettled modal stays typed-undecided, never hardened into a normative `SHALL`) are the anti-drift discipline applied at authoring time. Compose `/lingua`; never reimplement its doctrine.
- **The pre-merge drift check runs at ship, conditionally.** It fires at seam 9 **only when** the captured design carries acceptance criteria, and **only inside a workstream** (never a `/coral` slice or a one-sentence diff). It is a *facet* of the ship seam (not a new gate kind, not a new lifecycle) and reuses the `review-gate`'s target-derivable resolution. Outcomes: a **Missing** criterion **blocks** (like an open finding); an unconfirmable criterion is `inconclusive ⇒ surface` (fail-closed in the never-silently-pass sense — surface it, not abort-the-merge); **design-staleness** only **surfaces** (non-blocking). A design with no criteria simply skips it. See SKILL.md "The pre-merge drift check" (owning definition) and `review-gate.md`.
- **`/issue` is conditional.** Fire it only when there's a genuine deferred slice. Wiring it as a mandatory phase produces reflexive empty-issue prompts.
- **Lineage is delegated.** `workstream` calls `/execution-plan` to stamp the bet label + design link. It never re-implements identity/label/stamp logic — a second implementation mints a second identity. `/execution-plan`'s own first-label-creation confirm is a separate human gate it owns; the ledger does not subsume it.

## Composition boundaries (what workstream is NOT)

- It does **not** own scope tiers — `/council` does. A workstream invokes council and inserts gates around it.
- It is **not** a research engine — see `/research`. A workstream may *checkpoint-gate* a research effort (e.g. an `outcome-alignment` gate after synthesis), but a research effort never *launches* a workstream. (At breadth a research sweep would use a parallel-sweep Workflow — an execution engine, not a lifecycle — but that engine is deferred in `/research`'s MVP, which surfaces the too-wide-sweep limit instead.) This bounds the recursion.
- It does **not** auto-drive — the operator confirms each phase transition; the skill sequences and gates, it doesn't run unattended.

## The `/goal` relationship

`/goal` (the Claude Code harness command) sets the persistent objective and a Stop hook. `workstream` governs how that objective is pursued. The two compose cleanly: `/goal` keeps the agent working toward the objective; `workstream`'s checkpoints decide *where the agent must stop for a human* on the way there. The Stop hook keeps momentum; the checkpoints hold the one-way doors. Neither overrides the other — the hook keeps you working, the ledger keeps you honest.
