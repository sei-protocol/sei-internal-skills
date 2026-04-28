# Standard Issue Format

The canonical body produced by `/issue`. Section order is fixed; empty optional sections are omitted entirely (no placeholder headers).

Anchor example: [sei-protocol/sei-k8s-controller#137](https://github.com/sei-protocol/sei-k8s-controller/issues/137).

## Title

A descriptive sentence. NOT a Conventional-Commits prefix (`feat:` / `fix:`) unless the target repo's recent issues clearly use that style — sample with `gh issue list --repo <target> --limit 10 --state all` before defaulting.

Good: "Detect spec drift on Running nodes for mid-life SigningKey patch"
Bad: "fix: signing key patch is silent no-op"
Bad: "SigningKey bug"

## Body sections (in order)

### `## Problem` — required

What's wrong or what's missing, anchored on **observable behavior**, not the proposed fix. If the user's framing is "do X," push back: what behavior today is wrong, missing, or painful?

One short paragraph or a few sentences. Include file paths and line numbers if the user mentioned them — they help the triager locate the surface area.

### `## Impact` — required

Who's affected, how badly, what breaks if we don't address it. This is the field that drives sizing and prioritization, so it must be specific.

If multiple use cases motivate the issue, use sub-headings (`### Primary use case`, `### Future use case`). Otherwise a single paragraph.

Avoid: "this is bad," "we should do better," generic gestures. Prefer: concrete user, concrete failure mode, concrete cost (downtime, data loss, manual ops, blocked migration).

### `## Relevant experts` — required

A short list of personas — pulled from the target repo's `.claude/agents/` roster — who should look at this. Format:

```markdown
- `kubernetes-specialist` — controller plan logic
- `platform-engineer` — manifest / CRD surface
```

If the repo has no `.claude/agents/` roster, fall back to global Claude personas (e.g. `solidity-developer`, `security-specialist`). See `expert-routing.md` for the discovery rules.

This field is required because it's the routing primitive — without it, issues sit unassigned or land with the wrong reviewer.

### `## Proposed approach` — optional

If the filer already has a sketch, capture it here. Code blocks, file:line references, and step lists are welcome.

If the filer doesn't have a sketch, **omit the section entirely** rather than write `_TBD — needs design_` — an empty section signals false structure. The filer can re-open the issue with `/coral` later if they want a design pass before triage.

### `## Acceptance criteria` — optional

Checkbox list. Each item should be **observable** (a test passes, a field appears, a behavior changes), not a process step ("review the PR"). Suggest reasonable defaults inferred from the impact scope, then let the user edit.

Format:
```markdown
- [ ] `Status.Foo` field added and stamped on success
- [ ] Integration test covers <specific scenario>
- [ ] LLD §X updated to reflect supported flow
```

### `## Out of scope` — optional

Explicit deferrals, with un-defer triggers when known. This is high-value content — it prevents scope creep during implementation and makes the issue much easier to size.

Format:
```markdown
- **Mode switch (full-node → validator).** Same mechanic, larger surface. File as a follow-up once SigningKey drift ships and there's a concrete customer ask.
- **Demoting a validator to non-signing.** No field-level immutability blocks unset, but the workflow is unsupported per LLD §11.
```

### `## References` — optional

PRs, prior issues, design docs, runbooks. Bullet list with descriptive labels (not bare URLs).

```markdown
- PR #135 — merged LLD
- [LLD §11 — deferred entry](https://github.com/.../docs/foo.md#11-deferred)
- `.tide/validator-migration.md` — internal runbook
- Design: `design/milestones/seinode-mid-life-signing-key-drift-lld.md`
```

When this issue gets picked up and a coral/council session produces a design via **`/design`**, the `Design: <path>` line should be added back here so the issue → design lineage is discoverable from either direction. The `/design` skill offers to do this automatically; otherwise add it manually after the design lands.

## Section selection cheat-sheet

| Issue type | Required | Usually included | Often omitted |
|---|---|---|---|
| Bug report | Problem, Impact, Relevant experts | References | Proposed approach, Out of scope |
| Feature ask | Problem, Impact, Relevant experts | Acceptance criteria, Out of scope | Proposed approach (unless filer has a sketch) |
| Deferred follow-up from another piece of work | Problem, Impact, Relevant experts | Proposed approach, References, Out of scope | Acceptance criteria (often inherited from parent) |
| Design ask | Problem, Impact, Relevant experts | Out of scope, References | Acceptance criteria (defer to design pass), Proposed approach |
