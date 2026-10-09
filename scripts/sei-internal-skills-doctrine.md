## Operating with sei-internal-skills resources

This package uses Claude Code skills and specialist agents from Sei's sei-internal-skills repository, installed under `.claude/`. The doctrine below is the way to work with them.

### Engineering principles

- **Interfaces first** — the primary deliverable of a design is exact signatures, types, errors, and contracts. Implementation guidance is secondary.
- **YAGNI** — only build what traces to a current-phase need. Everything else is explicitly deferred, not silently omitted.
- **Two-way doors only** — prefer reversible decisions. One-way doors need explicit human approval before finalizing. Those are the irreversible choices: persisted schema/field names, public API contracts, on-disk or wire formats, anything other systems come to depend on.
- **Errors are interface** — every error condition is part of the public contract.
- **Provider owns the interface** — when a provider and consumer disagree, the provider's definition is canonical and consumers adapt.

### Output discipline

- **Conventional commits.** `feat:`, `fix:`, `docs:`, `refactor:` — reference the component in scope.
- **Comments & documentation.** Present-state only — never change/history/why-removed
  inline; that belongs in the commit or the pull request. Top-located: package, file or
  type documentation, not the body. **An in-body comment runs to 4 lines or fewer, and a
  file or package header runs to 20 or fewer.** Those two numbers are the rule; "sparingly"
  is not checkable and a bound nobody can knowingly violate is not a bound.
  *No gate checks either number*, so this document states and records them as uncheckable
  rather than implying enforcement.
- **PR bodies and in-code prose.** Conclusion first (BLUF); cut a sentence whose removal
  changes nothing a reviewer would do next. Open on the load-bearing noun, never a wind-up.
  Make every verb do work — no "serves to", "aims to", "is responsible for", "allows us
  to" — and collapse hedges. Do not restate a name a signature already carries: delete the
  comment rather than shorten it. Prefer one concrete example over one paragraph, and treat
  a heading as a budget, not a structure tax.

### Using the skills

- **`/harbor-dev`**: run your own ephemeral Sei chain in `eng-<alias>` on the harbor dev cluster. Spin it up, bench it, inspect it, tear it down. Start with "onboard me".
- **`/giga-dev`**: operate and diagnose the shared giga testnet, `giga-testnet-0`, on the prod cells. Read-only by default.
- **`/kubernetes`**: design or review sei-k8s-controller code (CRDs, reconcilers, status and conditions).

For a single-expert consult, use the Agent tool with the agent name as `subagent_type`. The agents live in `.claude/agents/`.
