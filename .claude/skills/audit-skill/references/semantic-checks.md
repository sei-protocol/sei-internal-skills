# Semantic Checks

What the semantic-checks subagent verifies and the exact prompts used. These fire in Step 4 of the procedure. Each prompt returns JSONL findings in the same format as static-checks.sh, so they merge cleanly in the findings report.

## When this fires

Step 4. After the static checks have run and the shape is known.

## Why semantic

Some rules are checkable mechanically (line counts, regex matches); others need judgment (does this guardrails stanza feel substantive, or is it boilerplate?). The semantic-checks pass dispatches a subagent that reads the skill and returns findings for the judgment-call rules.

## Subagent dispatch

Use `Agent` with `subagent_type=general-purpose` (or a domain specialist from `.claude/agents/` if one fits the skill under audit). One dispatch per check group; the responses concatenate into `state/run-<ts>/semantic-findings.jsonl`.

## Prompt — Description quality

**Description for the Agent call:** "Semantic audit — description quality"

**Prompt:**

> You are auditing a skill's description for compliance with the team's conventions. Read the YAML frontmatter from this skill:
>
> ```
> <paste SKILL.md frontmatter — usually first 6 lines>
> ```
>
> Return JSONL — one finding per line. For each of these checks, emit `{"id":"<id>","severity":"<sev>","title":"<title>","result":"pass|fail","evidence":"<one-line evidence>","catalog_ref":"<catalog_id>"}`.
>
> Checks:
>
> 1. **D4 (warn):** Does the description include at least one sibling-skill redirect? Look for phrases like "For X, use /other-skill". Adjacent skills in this repo include: coral, council, design, issue, bugbash, author-skill, audit-skill, harbor-dev. If the skill's purpose is adjacent to any of these, a redirect should be present.
>
> 2. **D6 (warn):** Does the description summarize the skill's workflow rather than just routing triggers? This is the Obra CSO trap: if the description describes what the skill *does* (e.g., "runs intake then research then drafts"), Claude reads the description and skips the body. The description should describe *when* to use the skill (triggers, symptoms, anti-triggers, redirects), not *how* the skill operates.
>
>    Look for these workflow-summary markers: verbs like "runs", "produces", "dispatches", "executes", "scaffolds", "creates", "writes" appearing as predicates (the skill's *action*), not as part of an anti-trigger or redirect.
>
> 3. **D7 (info):** Do the trigger phrases use vocabulary a real user would actually type? Or are they paraphrases? Quote the trigger phrases verbatim from the description and flag any that feel like the author's framing rather than the user's framing.
>
> Return only JSONL — no preamble, no commentary.

## Prompt — Body quality (shape-conditional)

**Description for the Agent call:** "Semantic audit — body quality"

**Prompt:**

> You are auditing a skill's body for compliance with the team's conventions. The skill's inferred shape is: `<shape>`.
>
> Read the SKILL.md:
>
> ```
> <paste full SKILL.md>
> ```
>
> Return JSONL — one finding per line.
>
> Checks (shape-conditional):
>
> 1. **B5 (warn, procedural):** For each numbered step in the procedure, does the step have a clear success criterion or named output? If a step says "do X" without naming what it produces or when it's done, flag it.
>
> 2. **B6 (warn, all shapes):** Does the body use one consistent term per concept? Or does it drift between synonyms (e.g., "fetch" and "retrieve" and "pull" for the same operation)? Quote the inconsistencies if any.
>
> 3. **B7 (warn, procedural):** Does the body embed shell commands in prose? Look for inline code blocks with shell syntax (e.g., `kubectl ...`, `gh ...`, `awk ...`). Multi-line shell blocks belong in `scripts/`, not in SKILL.md prose. Inline references like `\`kubectl get pods\`` for context are fine; multi-line `kubectl ... | grep ...` pipelines in prose are not.
>
> 4. **B8 (block, discipline+procedural):** Is the Guardrails stanza substantive? Count the refusal conditions named. A stanza listing ≥3 specific refusal conditions (each with a concrete "this fires when X" clause) is substantive. A stanza that just says "this skill is careful" or "guardrails apply" is a stub. Pass if substantive, fail with the count if not.
>
> 5. **R4 (warn, all shapes):** Do the references duplicate SKILL.md content? Or do they extend it (deep dives, full schemas, expanded examples)? If a reference file is mostly a longer-form rewrite of what SKILL.md already says, flag it.
>
> Return only JSONL.

## Prompt — Persuasion stack (shape-conditional)

**Description for the Agent call:** "Semantic audit — persuasion stack"

**Prompt:**

> You are auditing a skill's persuasion stack for compliance with the team's conventions. The skill's inferred shape is: `<shape>`.
>
> Read the SKILL.md and any `references/*.md`:
>
> ```
> <paste SKILL.md + references>
> ```
>
> The expected persuasion stack per shape (from `references/persuasion-principles.md` in author-skill):
>
> - **discipline** — Authority + Commitment + Social Proof; avoid Liking/Reciprocity.
> - **technique** — Moderate Authority + Unity; avoid heavy authority.
> - **pattern** — Unity + Commitment; avoid heavy authority.
> - **reference** — Clarity only; no persuasion.
> - **procedural** — Light Authority + Commitment; avoid Scarcity (false urgency).
>
> Return JSONL — one finding per line.
>
> Checks:
>
> 1. **P1 (warn, discipline):** Does the skill include a rationalization table mapping excuses to counters? Pass if a markdown table with at least 3 rows exists.
>
> 2. **P2 (warn, discipline):** Does the skill include a red-flags list — phrases that signal "STOP and reset"? Pass if a bulleted list with at least 3 entries exists.
>
> 3. **P3 (warn, discipline):** Does the skill use authority language consistently? Look for "YOU MUST", "Never", "Always", "No exceptions", "Refuse". Pass if used in ≥3 distinct contexts.
>
> 4. **P5 (warn, technique/pattern):** Does the skill use balanced authority + unity? Heavy "YOU MUST" without "we" or "our codebase" is a red flag for these shapes — they need adaptive judgment, not blind compliance.
>
> 5. **P6 (warn, reference):** Does the skill avoid persuasion entirely? Reference skills should be clarity-only; "YOU MUST" language in a reference makes it harder to read, not easier.
>
> Return only JSONL.

## Prompt — Anti-patterns

**Description for the Agent call:** "Semantic audit — anti-patterns"

**Prompt:**

> You are auditing a skill for anti-patterns documented in `references/conventions-catalog.md`.
>
> Read the SKILL.md and all references and scripts:
>
> ```
> <paste full skill contents>
> ```
>
> Return JSONL — one finding per line.
>
> Checks:
>
> 1. **A4 (warn):** Are there multi-language code examples for the same technique? (e.g., same example shown in Python, JavaScript, Go.) Per Obra, "one excellent example beats many mediocre ones." Flag if found.
>
> 2. **A5 (warn):** Are there fill-in-the-blank templates ("Replace <THIS> with your value")? Concrete examples are better. Flag if found.
>
> 3. **A6 (warn):** Are there deeply-nested file references — e.g., "see `references/category/topic/detail.md`"? Refs should be one level deep. Surfaces from prose context (overlaps with R1 which is the filesystem check).
>
> 4. **S3 (warn, procedural):** For each side-effecting script (any script that writes to disk, makes network calls, or modifies state), does it accept `--dry-run`? Read each script and judge. Skip read-only scripts.
>
> 5. **S5 (info, procedural):** Are scripts portable across macOS and Linux? Look for GNU-only constructs: `sed -i ''` with no extension (BSD), `readlink -f` (GNU-only on macOS without coreutils), `date -d` (GNU vs BSD `date -j`). Flag if found.
>
> Return only JSONL.

## Synthesis

The four subagent responses concatenate into `state/run-<ts>/semantic-findings.jsonl`. The findings-report.sh script merges this with `static-findings.jsonl` and `pressure-findings.jsonl` to produce the audit report.

## When the subagent struggles

If a semantic check returns inconclusive findings ("can't tell from this snippet"), don't paper over it. Emit `result: "skip"` with an evidence line explaining why. The findings report surfaces skipped checks separately so the user knows the audit didn't fully cover them.
