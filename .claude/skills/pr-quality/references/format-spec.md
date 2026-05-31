# Finding Format Spec

How the rendered PR comment looks. Matches Tide's broader convention (see `.claude/skills/issue/references/format-spec.md` for the issue-shape sibling).

## Comment shape

```markdown
<!-- tide-pr-quality | sha=<HEAD_SHA> | findings-hash=<sha256> -->
### PR Quality — N finding(s)

- `<file>:<line>` — <one-sentence-fix>.
  Rule: [`<rule_id>`](.claude/memory/<feedback_entry>.md) — <one-sentence-rule-statement>.

- ...

<details><summary>+<M> additional lower-severity findings suppressed</summary>

- `<file>:<line>` — <one-sentence-fix>.
  ...

</details>

---

Suggestive only; humans decide. Opt out via label `skip-pr-quality`.
```

## Rules

1. **Marker is required**. The two-field marker (`sha`, `findings-hash`) MUST be the first line. `post-or-update.sh` parses it to detect prior runs.
2. **Title format**: `### PR Quality — N finding(s)`. N is the post-cap count (max 5).
3. **Finding line shape**: `- \`<file>:<line>\` — <fix>.` Followed on the next line (indented 2 spaces): `Rule: [\`<rule_id>\`](.claude/memory/<feedback_entry>.md) — <statement>.`
4. **Relative repo links** for memory citations. They render as live links in GitHub PR comments.
5. **Suppressed-findings block** is a `<details>` collapsed by default. Include only if `suppressed_count > 0`.
6. **Disclaimer footer** is fixed text: "Suggestive only; humans decide. Opt out via label `skip-pr-quality`."

## Severity rendering

Findings are sorted with `warn` before `nudge`, then mechanical before LLM-judged within tier. There is NO explicit severity badge in the rendered output — the order IS the severity signal. Adding `[WARN]` / `[NUDGE]` prefixes is feature creep; resist.

## What this format does NOT include

- No emoji severity badges (🔴 / 🟡)
- No `[blocker]` / `[nit]` / `[info]` labels
- No reaction-driven dismissal mechanism
- No "ack" or "applied" footers
- No interactive slash-commands ("/show-all", "/dismiss")
- No edit-history of prior runs

All of the above are feature-creep beyond v1. If they become real needs, file a PR against this file.
