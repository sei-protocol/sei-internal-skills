# Finding Format Spec

How the rendered post-PR comment looks. v1 posts a fresh comment per invocation (no anchored marker, no dedupe — those are deferred per `rule-registry.md`).

## Comment shape

```markdown
### PR Quality — N finding(s)

- `<file>:<line>` — <one-sentence-fix>.
  Rule: [`<rule_id>`](.claude/memory/<feedback_entry>.md) — <one-sentence-rule-statement>.

- ...

---

Suggestive only; humans decide.
```

## Rules

1. **Title format**: `### PR Quality — N finding(s)`. N is the total finding count (uncapped in v1).
2. **Finding line shape**: `- \`<file>:<line>\` — <fix>.` Followed on the next line (indented 2 spaces): `Rule: [\`<rule_id>\`](.claude/memory/<feedback_entry>.md) — <statement>.`
3. **Relative repo links** for memory citations. They render as live links in GitHub PR comments.
4. **Disclaimer footer** is fixed text: "Suggestive only; humans decide."

## Severity rendering

Findings are sorted `warn` before `nudge`, then mechanical before LLM-judged within tier. There is NO explicit severity badge — the order IS the signal. Adding `[WARN]` / `[NUDGE]` prefixes is feature creep; resist.

## What this format does NOT include

- No anchored marker / hash dedupe (v1 posts fresh)
- No 5-finding cap or suppressed-block disclosure (v1 uncapped)
- No emoji severity badges (🔴 / 🟡)
- No `[blocker]` / `[nit]` / `[info]` labels
- No reaction-driven dismissal mechanism
- No interactive slash-commands
- No opt-out label reference (local invocation; user just doesn't invoke)

All of the above are feature-creep beyond v1. Un-defer triggers documented in `rule-registry.md` deferred-mechanisms table.
