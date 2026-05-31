# Judge: temporary_migration_notes (LLM-judged)

## Rule

Pin-to-version notes and "until X ships" qualifiers belong in PR descriptions, commit messages, or release notes — NOT in durable documentation that survives the migration.

```
❌ in CLAUDE.md: "Pin to v0.0.16 until sei-protocol/seictl#356 lands."

✅ (delete from durable doc; capture in PR body or release notes)
```

## Scope

- Files: `CLAUDE.md`, `AGENTS.md`, `README.md`, `docs/**`
- Pattern shapes the judge looks for:
  - "Pin to v..."
  - "until X ships" / "until X lands" / "until X is merged"
  - "Temporarily..."
  - "Once X is in, we can remove this"
- v1 does NOT judge migration notes inside code comments, in-repo runbooks under `.runbooks/`, or other transient surfaces.

## Few-shot examples

**Violation 1**: "Pin to seictl v0.0.16 until #356 lands." in CLAUDE.md
**Violation 2**: "Use the temporary workaround until next release." in README.md
**Violation 3**: "Once the controller deploys, remove the manual override." in docs/runbooks/...

**Non-violation 1**: "Set `bpf-map-dynamic-size-ratio: 0.0025` (chart default)." in CLAUDE.md — describes a stable convention, not a migration note.
**Non-violation 2**: "The v2 API replaces v1; v1 was deprecated 2024-08." in README — historical context, not pin-to-old-version.

## Self-consistency

n=3 samples, temp=0.3, require 2/3 agreement.

## Cites

Memory: `feedback_temporary_migration_notes` — "pin-to-old-version hints belong in PRs/release notes, not CLAUDE.md"
