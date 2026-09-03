# Anthropic / Obra Best Practices — Distilled for author-skill

Source: <https://github.com/obra/superpowers/blob/main/skills/writing-skills/anthropic-best-practices.md> and the parent `writing-skills/SKILL.md`. Reproduced and condensed for use inside this skill. When this file conflicts with the SKILL-TEMPLATE.md in this repo, the local template wins for procedural skills (it encodes sei-internal-skills's safety conventions) — Obra wins for the description craft, persuasion design, and TDD methodology.

## Core principles

1. **Context window is a public good.** Every line you write in SKILL.md is loaded into every conversation that discovers the skill. Justify the token cost of each section.
2. **Degrees of freedom should match task fragility.** High freedom (prose guidance) for problems with many valid solutions; low freedom (specific scripts) for fragile, error-prone operations.
3. **Test across models.** Verify with Haiku, Sonnet, Opus. Effectiveness depends on the underlying model — what's bulletproof on Opus may collapse on Haiku.

## Naming

- **Gerund (-ing) form for processes:** `creating-skills`, `testing-skills`, `debugging-with-logs`.
- **Active verb-first** when not a gerund: `condition-based-waiting`, `flatten-with-flags`, `root-cause-tracing`.
- **Avoid** vague names like `helper`, `utils`, `common`.

**sei-internal-skills local note:** the existing workflow skills use 1-word imperatives (`coral`, `council`, `design`, `issue`, `bugbash`). When authoring a sei-internal-skills workflow skill, match that style. When authoring a technique/pattern skill at user-scope, prefer Obra's gerund form.

## Description writing — the highest-leverage field

The description is the **only** signal that routes invocation. Get it wrong and the skill never fires; get it right and the skill fires reliably.

Rules:

1. **Third person.** "Processes Excel files", not "I can help you with Excel."
2. **"Use when..." style.** Lead with triggering conditions, not capability claims.
3. **Triggers ONLY — never workflow summary.** This is Obra's hardest-earned lesson: when a description summarizes the workflow, Claude reads the description and skips the skill body. The skill becomes documentation Claude ignores.
4. **Anti-triggers.** Explicit `NOT for X` / `SKIP if Y` clauses prevent over-matching.
5. **Sibling redirects.** "For X, use /other-skill" routes adjacent intents away.
6. **Under 1024 chars total.** Hard limit.
7. **Keyword coverage.** Use the words a user would actually type — error messages, symptoms, tool names, file extensions. Not synonyms.

### Bad → Good

```yaml
# ❌ BAD — first person, vague
description: I can help you write tests when they're flaky

# ❌ BAD — summarizes workflow (Claude will skip the body)
description: Use when executing plans — dispatches subagent per task with code review between tasks

# ✅ GOOD — triggers only
description: Use when tests have race conditions, timing dependencies, or pass/fail inconsistently
```

## Content organization

- **SKILL.md under 500 lines.** Strict ceiling. Push detail to `references/` files one level deep so partial reads still find everything.
- **Progressive disclosure.** SKILL.md is the overview; references/ is the depth. Don't repeat reference content in SKILL.md.
- **TOC for 100+ line reference files.** So partial reads see what's available.
- **One-level-deep refs only.** No `references/sub/deep/file.md` — Claude's partial read may not follow the trail.

## Workflows

- Numbered steps for sequential operations.
- Checklists for items Claude should track and report against.
- "Run validator → fix errors → repeat" loops for style and format compliance.

## Content rules

- **Forward slashes in all paths** (`a/b/c`, never `a\b\c`). Cross-platform.
- **No time-sensitive instructions.** Don't write "as of 2025" or "in the latest version." Use "old patterns" sections for deprecated approaches instead.
- **Consistent terminology.** Pick one word for each concept and use it throughout (always "extract", never alternate with "pull" / "retrieve").
- **One excellent example beats many mediocre ones.** Pick the language closest to the domain and write one complete, runnable, well-commented example.
- **No fill-in-the-blank templates.** Templates are abstract; examples are concrete. Write the concrete one.

## Code in skills

- **Self-documenting code.** The code names its own intent. AGENTS.md "Output discipline" owns what a comment may say. Justify configuration parameters with the reason a specific value was chosen.
- **Pre-made utility scripts.** When reliability matters, provide a script. Clarify whether Claude should *execute* it or *reference* it as a pattern.
- **Verifiable outputs.** For complex tasks, use plan-validate-execute: Claude writes a structured intermediate file, a validator checks it, then Claude executes.
- **Explicit error handling.** Scripts should handle conditions, not defer to Claude. Useful error messages include suggested next steps.

## Evals

Minimum bar: **3 evals per skill**. sei-internal-skills local convention: 1 happy-path + 1 halt-condition is the minimum. 3 is the Obra ideal — happy-path, edge-case, and adversarial.

Build evals **first**:

1. Establish baseline (run the scenarios without the skill).
2. Author the skill.
3. Verify the skill (run the scenarios with the skill).
4. Iterate based on real navigation patterns.

## CSO (Claude Search Optimization)

Future Claude needs to *find* the skill. Optimize for retrieval:

- **Description = when to use, NOT what the skill does.** Same rule, restated.
- **Keyword coverage** — error messages, symptoms, synonyms, tool names. "race condition" AND "flaky" AND "timing".
- **Descriptive naming** — verb-first, active, semantic.
- **Cross-reference skills by name with explicit markers** — `**REQUIRED:** Use superpowers:test-driven-development`. **Never** `@skills/...` — that force-loads and burns context before you need it.

## Token efficiency

For skills loaded into every conversation (getting-started, frequently-loaded):

- **Target <150 words** for getting-started workflows.
- **Target <200 words total** for frequently-loaded skills.
- **Target <500 words** for everything else.

Techniques:

- Move details to `--help` output, not into SKILL.md.
- Cross-reference other skills instead of repeating their content.
- Compress examples — minimal prompt + minimal action.

## Anti-patterns

- ❌ Windows paths.
- ❌ Many competing approaches without a default ("you could use A or B or C..."). Pick a default; offer escape hatches.
- ❌ Vague description.
- ❌ Deeply nested reference files.
- ❌ Assuming packages are pre-installed without a check.
- ❌ Time-sensitive instructions.
- ❌ Multi-language examples for one technique.
- ❌ Generic labels in flowcharts (`step1`, `helper2`).
- ❌ Force-loading other skills with `@` syntax.

## Pre-deployment checklist

Verify before sharing:

- [ ] Description is specific, "Use when..." style, has anti-triggers.
- [ ] SKILL.md under 500 lines.
- [ ] One-level-deep file references.
- [ ] Clear, tested workflows.
- [ ] Explicit error handling in code.
- [ ] Forward slashes in all paths.
- [ ] At least 3 evaluations (or sei-internal-skills's 2-eval minimum).
- [ ] Tested with Haiku, Sonnet, and Opus.
