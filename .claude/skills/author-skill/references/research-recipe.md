# Research Recipe

The skill being authored is grounded in current, verifiable material — not just training data. This file is the prompt template for the research phase (Step 3 in the procedure).

## When this fires

Step 3. Before any drafting. After Intake (Step 1) and Shape decision (Step 2).

## Why research

Training data has a knowledge cutoff. Domains evolve — best practices, library APIs, terminology, common failure modes shift. A skill grounded only in training data ages poorly and ships with outdated rationalizations.

Research also surfaces *expert idioms* — the specific words and patterns experts in the domain use. Those words become the description's keywords and the skill body's terminology. A skill that uses outsider terminology gets bypassed by experts who don't recognize the framing.

## Parallel research streams

Dispatch 2–4 `general-purpose` subagents via `Agent`, one per stream. Each runs in parallel and returns a concise summary. The orchestrator synthesizes the results.

### Stream 1 — Best practices and conventions

**Prompt template:**

> Research best practices for `<domain>` with focus on `<focus>`. The audience is `<expertise needed>` who would invoke a skill for this. Look for:
>
> - Established conventions and idioms
> - Authoritative sources (official docs, well-known experts, framework maintainers)
> - Patterns that experts apply consistently
> - Recent evolutions or changes in the last 12-24 months
>
> Return: a bullet list of 5-10 concrete practices, each with a source URL. Quote any pattern names experts use verbatim — those become keywords.

### Stream 2 — Common failure modes

**Prompt template:**

> Research common failure modes, antipatterns, and gotchas for `<domain>` with focus on `<focus>`. Look for:
>
> - Specific failure modes documented in postmortems, GitHub issues, or expert writeups
> - Common rationalizations that lead to those failures ("I'll just skip X this once")
> - Symptoms that signal the failure mode is happening
> - Recommended interventions
>
> Return: a table of failure modes with columns: Failure | Rationalization | Symptom | Intervention. Quote rationalizations verbatim when possible — those become the skill's rationalization-table seed.

### Stream 3 — Authoritative sources

**Prompt template:**

> Locate the canonical references for `<domain>`. Look for:
>
> - Official documentation
> - The "definitive" book or blog post if one exists
> - Active community resources (StackOverflow tag, Reddit subreddit, Discord) for currency
> - Tooling that codifies the practice (linters, validators, frameworks)
>
> Return: a list of URLs with one-line annotations. Mark which sources are official vs. community vs. third-party.

### Stream 4 — Terminology and expert idioms (optional, when domain is unfamiliar)

**Prompt template:**

> Build a glossary of expert terminology for `<domain>` with focus on `<focus>`. For each term:
>
> - The term itself (verbatim — experts use specific words)
> - A one-sentence definition
> - Whether it's industry-standard or community-specific
>
> Aim for 10-20 terms. Prioritize terms that appear in the best-practices stream's output — those are the load-bearing vocabulary.

## Synthesis

After the parallel streams return, the orchestrator synthesizes into `state/run-<ts>/research-notes.md`:

```markdown
# Research Notes — <skill name>

## Domain
<one paragraph from Intake>

## Best practices (Stream 1)
- Practice 1 — source URL
- Practice 2 — source URL
...

## Failure modes (Stream 2)
| Failure | Rationalization | Symptom | Intervention |
|---------|-----------------|---------|--------------|
| ...     | ...             | ...     | ...          |

## Authoritative sources (Stream 3)
- Official: <URL>
- Definitive: <URL>
- Community: <URL>
- Tooling: <URL>

## Terminology (Stream 4)
- Term 1: definition
- Term 2: definition
...

## Synthesized findings
<2-3 paragraphs the orchestrator writes after reading all four streams. The synthesized findings are what informs the skill body.>
```

## User review

After synthesis, show the user the research notes and ask:

> "Anything missing from your own context that should be in here before we draft the skill?"

Capture additions. The user often has internal context (postmortems, internal docs, team conventions) the public web doesn't have.

## When research returns insufficient material

If the parallel streams come back thin (no authoritative sources, no documented failure modes, no clear conventions):

- The domain may be new or proprietary. Surface this and ask the user to provide source material directly.
- The domain may be too narrow. Surface and ask if the skill should target a broader slice.
- The domain may not need a skill (no recurring patterns, no rationalization-prone constraints). Surface — per Obra, "Don't create [skills] for one-off solutions."

**Do not paper over thin research by leaning on training data.** A skill grounded in vague training data is worse than no skill — it ships with outdated material and false confidence.

## Caching research notes

Research notes go to `state/run-<ts>/research-notes.md` and are *not* committed to the final skill directory. The skill body cites authoritative sources by URL where appropriate; the raw research dump stays in state.

This keeps the SKILL.md token budget tight and avoids shipping research that ages.

## Time budget

For most domains, the research phase should complete in 5-10 minutes of parallel subagent execution. If a stream takes longer, halt that stream and surface what was found so far. Research is bounded — the skill won't be a literature review.
