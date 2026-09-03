## Operating with sei-internal-skills resources

This package consumes portable Claude Code skills and specialist agents authored in Sei's sei-internal-skills library and installed under `.claude/`. The skills are invoked as the slash-commands below; the agents are dispatched by those skills. What follows is the opinionated doctrine for operating with them — the *way* to work, not a description of the library.

### Engineering principles

- **Interfaces first** — the primary deliverable of a design is exact signatures, types, errors, and contracts. Implementation guidance is secondary.
- **YAGNI** — only build what traces to a current-phase need. Everything else is explicitly deferred, not silently omitted.
- **Two-way doors only** — prefer reversible decisions. One-way doors (irreversible choices: persisted schema/field names, public API contracts, on-disk or wire formats, anything other systems come to depend on) require explicit human approval before finalizing.
- **Errors are interface** — every error condition is part of the public contract.
- **Provider owns the interface** — when a provider and consumer disagree, the provider's definition is canonical and consumers adapt.

### Output discipline

- **Conventional commits.** `feat:`, `fix:`, `docs:`, `refactor:` — reference the component in scope.
- **Comments & documentation.** Present-state only — never change/history/why-removed
  inline; that belongs in the commit or the pull request. Top-located: package, file or
  type documentation, not the body. **An in-body comment runs to 4 lines or fewer. A
  file or package header runs to 20 or fewer.** Those two numbers are the rule; "sparingly"
  is not checkable and a bound that cannot be violated knowingly is not a bound.
  *No gate checks either number.* They are stated and recorded as uncheckable rather than
  implied to be enforced.
- **PR bodies and in-code prose.** Conclusion first (BLUF). Cut a sentence whose removal
  changes nothing a reviewer would do next. Open on the load-bearing noun, never a wind-up.
  Make every verb do work — no "serves to", "aims to", "is responsible for", "allows us
  to". Collapse hedges. Do not restate a name a signature already carries: delete the
  comment rather than shorten it. Prefer one concrete example over one paragraph. Treat a
  heading as a budget, not a structure tax.

### Using the skills

- **`/xreview`** — the relevant specialists independently review a design, plan, or diff, then synthesize a findings table. The review counterpart to producing the work.
- **`/root-cause`** — disciplined, data-driven, multi-expert investigation of complex problems.
- **`/idiomatic`** and **`/systems`** — review code for language/package idiom, then for systems-level quality on top. Idiom ⊂ systems quality; run them in that order.

Further workflow skills — `/coral`, `/council`, `/bugbash`, `/design`, `/issue`, `/research`, `/workstream` — are **experimental** and ship only on opt-in (`make sync-experimental`). Use them when they are installed; never assume they are.

### xreview discipline

When the relevant specialists review a produced artifact (design, plan, diff, or a set of expert outputs):

- **Blinded and independent** — each reviewer commits its findings before seeing the others'; no reviewer's view is summarized into another's brief.
- **An assigned dissenter** — one reviewer is tasked to argue against the emerging consensus and surface the strongest counter-case.
- **Slate completeness** — the slate covers the domain *and* the idiom axis (`idiomatic-reviewer`) *and*, for doc artifacts, the prose axis (`prose-steward`) — not domain experts alone.
- **Automated review is co-equal** — treat an automated reviewer (e.g. Cursor Bugbot) as a peer input, not noise; an unresolved flag blocks.
- **Confirmed-consensus iteration** — after a fix, re-dispatch the reviewer that raised the finding to confirm closure; merge only on unanimous sign-off with no open concerns. `/xreview` owns the procedure.

### Key rules

- **Provider owns the interface.** Consumers adapt.
- **YAGNI.** Only features tracing to current-phase needs.
- **Errors are interface.** Every error is part of the public contract.
- **One-way-door gate.** Irreversible decisions require explicit human approval before finalizing.
- **Conventional commits.** Reference the component in scope.

### Roles, not roster

Specialists are dispatched by the workflow skills above; for a single-expert consult, use the Agent tool with the agent name as `subagent_type`. The review champions are named contracts: `idiomatic-reviewer` (code idiom, `/idiomatic`) and `prose-steward` (doc-artifact prose). The full roster of available specialists lives in the synced `.claude/agents/` files.
