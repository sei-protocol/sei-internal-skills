# Rationalization Table — Sources

The 10 verbose-output rationalizations from SKILL.md, with source citations for the underlying bias each one exploits.

| Rationalization | Underlying bias | Source |
|---|---|---|
| "The lead said 'be thorough.'" | Authority compliance + reward-model length bias | Saito et al. 2023, *Verbosity Bias in Preference Labeling by LLMs*; Singhal et al. 2024, *A Long Way to Go* |
| "The reviewer will be cold on Monday." | Asymmetric loss perception — missing context feels punishable, redundant context feels free | RLHF training never penalized redundancy (Anthropic Claude Code prompt: "answer concisely with fewer than 4 lines") |
| "This change is complex, so the explanation must match." | False symmetry between problem complexity and explanation complexity | Linus Torvalds kernel commit norms: large patches with short messages |
| "Adding context can't hurt; omitting might." | Same asymmetric loss perception | (same as row 2) |
| "Brevity is for when reviewers have context. They don't here." | Context-dependent exception rationalization | Discipline-skill anti-pattern from Obra's persuasion-principles.md |
| "I'll be scannable — headers and bullets, not wall-of-text." | Structure-tax disguising verbosity | Microsoft Writing Style Guide: "structure is not brevity" |
| "I should restate the request to confirm I understood." | SFT preamble pattern — training examples open with task reformulation | Anthropic prompting best practices: "do not restate the prompt" |
| "I'm not 100% sure — let me hedge." | Verbosity compensation under uncertainty | Zhang et al. 2024, *Verbosity ≠ Veracity* (ACL UncertaintyLP 2025) |
| "A senior engineer would explain the why fully." | Sycophancy — agent over-performs "thorough professional" persona | Sharma et al. 2023, *Towards Understanding Sycophancy in Language Models* |
| "The PR body is the durable record — be complete." | Audience-genre confusion (PR ≠ design doc) | rust-lang/rust PR conventions (28-word reference PR #157179) |
| (bonus, observed in practice) "I'm being clear, not verbose." | Self-evaluation blind spot — LLM-as-judge rates longer outputs as clearer | Dubois et al. 2024, *Length-Controlled AlpacaEval* |

## When to use this table during a brevity pass

1. **Mid-rewrite**: if an excuse forms in your reasoning, scan the left column. If it matches, apply the counter from the right column of SKILL.md and continue.
2. **Post-rewrite**: if the after-version still feels too short, scan the table for which row your "this needs more" is rationalizing.
3. **Cross-review**: if a reviewer pushes back on a compression, ask which row from the table they think doesn't apply. Forces specific disagreement rather than vague "too short."

## Sources (full URLs)

- Saito et al. 2023: https://arxiv.org/pdf/2310.10076
- Singhal et al. 2024: *A Long Way to Go: Investigating Length Correlations in RLHF*
- Sharma et al. 2023: https://arxiv.org/abs/2310.13548
- Zhang et al. 2024 / 2025: https://aclanthology.org/2025.uncertainlp-main.14/
- Dubois et al. 2024: *Length-Controlled AlpacaEval*
- Anthropic prompting best practices: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices
- Microsoft Writing Style Guide: https://learn.microsoft.com/en-us/style-guide/word-choice/use-simple-words-concise-sentences
- GOV.UK Content Design: https://www.gov.uk/guidance/content-design/writing-for-gov-uk
- Google developer documentation style: https://developers.google.com/style/highlights
