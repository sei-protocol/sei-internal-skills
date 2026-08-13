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
- **Comments & documentation.** Present-state only — never change/history/why-removed inline (that belongs in the PR/commit); sparingly; top-located (package/file/type doc, not the body); comprehensive context in one centralized doc. Champions: `idiomatic-reviewer` owns in-source comments + config annotations (`/idiomatic`); `prose-steward` owns doc artifacts + header-doc prose (`/lingua`). The full rules and the champion-boundary decision procedure live in those two skills.

### Using the skills

- **`/xreview`** — the relevant specialists independently review a design, plan, or diff, then synthesize a findings table. The review counterpart to producing the work.
- **`/root-cause`** — disciplined, data-driven, multi-expert investigation of complex problems.
- **`/idiomatic`** and **`/systems`** — review code for language/package idiom, then for systems-level quality on top. Idiom ⊂ systems quality; run them in that order.
- **`/pr-quality`** — the locked pre-PR rule gate. **`/brevity`** — tighten a PR body.

Further workflow skills — `/coral`, `/council`, `/bugbash`, `/design`, `/issue`, `/research`, `/workstream` — are **experimental** and ship only on opt-in (`make sync-experimental`). Use them when they are installed; never assume they are.

### xreview discipline

When the relevant specialists review a produced artifact (design, plan, diff, or a set of expert outputs):

- **Blinded and independent** — each reviewer commits its findings before seeing the others'; no reviewer's view is summarized into another's brief.
- **An assigned dissenter** — one reviewer is tasked to argue against the emerging consensus and surface the strongest counter-case.
- **Slate completeness** — the slate covers the domain *and* the idiom axis (`idiomatic-reviewer`) — not domain experts alone. For doc artifacts, add the prose axis (`prose-steward`) when it is installed; it is an experimental agent.
- **Automated review is co-equal** — treat an automated reviewer (e.g. Cursor Bugbot) as a peer input, not noise; an unresolved flag blocks.
- **Confirmed-consensus iteration** — after a fix, re-dispatch the reviewer that raised the finding to confirm closure; merge only on unanimous sign-off with no open concerns. `/xreview` owns the procedure.

### Key rules

- **Provider owns the interface.** Consumers adapt.
- **YAGNI.** Only features tracing to current-phase needs.
- **Errors are interface.** Every error is part of the public contract.
- **One-way-door gate.** Irreversible decisions require explicit human approval before finalizing.
- **Conventional commits.** Reference the component in scope.

### Roles, not roster

Specialists are dispatched by the workflow skills above; for a single-expert consult, use the Agent tool with the agent name as `subagent_type`. The review champion is a named contract: `idiomatic-reviewer` (code idiom, `/idiomatic`). The full roster of available specialists lives in the synced `.claude/agents/` files.
