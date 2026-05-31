# Judge: authoritative_voice (LLM-judged)

## Rule

Skill content speaks as the expert. Meta-narration ("per skill protocol", "as my instructions say", "as the brevity skill requires") leaks the skill's machinery to the user and weakens the authoritative voice the skill is supposed to embody.

```
❌ in a skill SKILL.md: "As my instructions say, I'll now apply Rule 3..."

✅ "Apply Rule 3."
```

## Scope

- Files matching `.claude/skills/**/*.md`
- Both SKILL.md and references/* files within skill directories

## Few-shot examples

**Violation 1**: "As my instructions say, this section is mandatory."
**Violation 2**: "Per skill protocol, halt if X."
**Violation 3**: "The skill requires me to dispatch via the Agent tool."

**Non-violation 1**: "Halt if X." — direct imperative.
**Non-violation 2**: "Dispatch via the Agent tool." — direct procedural.

## Self-consistency

n=3 samples, temp=0.3, require 2/3 agreement.

## Cites

Memory: `feedback_authoritative_voice` — "skills speak as the expert; never leak 'per skill protocol' / 'as my instructions say' to users"
