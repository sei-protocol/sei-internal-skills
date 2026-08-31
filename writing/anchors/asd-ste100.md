# Anchor: ASD-STE100 (Simplified Technical English)

| Field | Value |
|---|---|
| Registry id | `asd-ste100` |
| Edition | Issue 9, January 2025 |
| Steward | ASD, maintained by STEMG |
| Source | <https://www.asd-ste100.org/> |
| Redistributable | No. See `NOTICE.md`. |
| Vale coverage | Partial |

## What the standard is

ASD-STE100 is a controlled natural language. It has two parts: a set of writing rules
that cover grammar and style, and a controlled dictionary of approved words. Each approved
word has one meaning and one part of speech. The current edition has 53 writing rules and
approximately 900 approved words.

The rules separate two text types, and the limits differ:

- **Procedures**: instructions. Maximum 20 words per sentence. One instruction per sentence.
- **Descriptions**: everything else. Maximum 25 words per sentence. Maximum 6 sentences per paragraph.

## Why it applies to model output

The aerospace industry built STE for a reader who cannot ask a follow-up question. Most
readers of the manuals were not native English speakers, and a misread instruction could
kill someone.

An agent that parses another agent's output is in the same position. It has no
back-channel. It cannot ask "did you mean X or Y?". The constraint that removes ambiguity
for a mechanic removes it for a downstream parser.

## The anchor string

```
Write all English in ASD-STE100 Simplified Technical English: approved words in their
approved part of speech, active voice, simple tenses, one instruction per sentence, no
more than three nouns in a noun cluster.
```

Keep the string short. If a model does not resolve `ASD-STE100`, a longer string will not
fix it. Test recognition instead. See `evals/recognition/`.

## What Vale checks

| Rule | Constraint |
|---|---|
| `STE-SentenceLength-Procedure` | 20 words, procedures |
| `STE-SentenceLength-Description` | 25 words, descriptions |
| `STE-ParagraphLength` | 6 sentences per paragraph |
| `STE-Passive` | be-verb plus past participle |
| `STE-Contractions` | full forms only |
| `STE-NounCluster` | more than three nouns together |
| `STE-Gerund-Instruction` | an instruction that starts with `-ing` |
| `STE-PhrasalVerbs` | a phrasal verb where one verb exists |
| `STE-ApprovedWords` | non-approved word with an approved replacement |

## What Vale cannot check

- One approved meaning per word **in context**. The dictionary approves `close` as a
  verb and not as an adjective. Disambiguation needs word-sense judgement.
- Whether a term is a legitimate Technical Name in this project. Add project terms to
  `styles/config/vocabularies/AgenticWriting/accept.txt`, which makes the decision
  reviewable in Git.
- Whether a procedure step is one action. `Open the file and edit the value` passes the
  word count and breaks the rule.

State this gap out loud when you report on the system. A partial verifier that claims
full coverage is worse than no verifier.

## Known tension

STE is deliberately flat. Do not anchor to it for marketing copy, narrative, or anything
where voice is the product. Clarity is the goal, not brevity: a long answer in short
sentences is correct STE. Never drop a caveat or a scope qualifier to meet a word limit.
Split the sentence instead.

## Prior art

- <https://github.com/danyuchn/asd-ste100-skill> — skill scoped to agent-facing text
- <https://github.com/dandye/ste-writing-style> — skills with an extensible dictionary
- <https://github.com/stuffbucket/vale> — Go linter and MCP server, OpenSTE-derived vocabulary
- <https://github.com/openste/openste> — MIT-licensed wordset
