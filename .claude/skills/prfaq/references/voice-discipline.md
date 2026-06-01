# Voice discipline

Amazon's PRFAQ voice — short sentences, plain language, concrete nouns, customer-first, declarative. The two failure modes the voice prevents: corporate-speak that hides muddled thinking, and adjective-rich prose that substitutes connotation for evidence.

> "The reason writing a good four-page memo is harder than writing a 20-page PowerPoint is because the narrative structure of a good memo forces better thought and better understanding of what's more important than what, and how things are related." — Amazon at the Forum on Leadership, 2018

## Sentence-level rules

- **Sentence length**: target under 20 words for 70-100% of sentences; ceiling 30 words; median ~15.
- **Voice**: active in ≥90% of sentences.
- **Tense**: PR is present-tense, as if launched.
- **One idea per sentence. One argument per paragraph.**
- **Readability target**: 5th–8th grade (ARI, Gunning Fog).
- **Numbers, names, and verbs replace adjectives.** "Fast" → "200ms." "Cheap" → "$0.023/GB/month." "Many users" → "Six of the top ten US retailers."

## Kill lists (unconditional)

The skill refuses these words and phrases. They are not weak-but-allowed; they are banned. If a sentence needs one, the missing concrete substitute must surface, or the sentence is rewritten or cut.

### Marketing adjectives (banned without data)
`excellent` · `outstanding` · `innovative` · `robust` · `world-class` · `best-in-class` · `cutting-edge` · `next-generation` · `revolutionary` · `comprehensive` · `industry-leading` · `first-of-its-kind` · `state-of-the-art` · `magical`

**Rule:** if an adjective could be replaced with a number, replace it.

### Corporate-speak (banned outright)
`synergy` · `synergies` · `leverage` (verb) · `ecosystem` · `stakeholder` · `deep dive` · `circle back` · `low-hanging fruit` · `best-of-breed` · `paradigm shift` · `disruptive` · `game-changing` · `move the needle` · `boil the ocean` · `holistic`

### Banned adverbs
`very` · `really` · `extremely` · `significantly` · `substantially` · `dramatically`

### Banned qualifications (hedge language)
`I think` · `it seems` · `in our opinion` · `we believe` · `we hope` · `arguably` · `clearly` · `fairly` · `interestingly` · `probably` · `should result in` · `would help` · `might bring` · `nearly all` · `many people believe`

### Weasel words (general)
`a number of` · `large` · `many` · `various` · `significant` · `substantial` · `some` · `a lot` · `most` · `several`

### Banned PR-cliché openings
- "We are excited to announce…" / "Today we're proud to share…" / "Thrilled to introduce…" — strip; just announce.
- "Today marks a milestone…" — open with the dated factual lead.
- "industry-leading" — replace with a number.
- "first-of-its-kind" — name the specific capability that's novel.
- "comprehensive" — list the things.
- "seamlessly" — describe the mechanism.
- "empower" — name the verb the customer actually does.

## The two tests

Every sentence in the PR must pass both:

### "So what?" test
For every sentence: *what does the customer do differently because this is true?* If the sentence describes a feature with no behavioral consequence, cut it or move it to the internal FAQ.

### "Each word earns its place" test
For every word: *what information does this convey?* If the answer is "tone," "filler," or "emphasis," remove it.

## Before / after rewrites

Pattern: strip the adjective; insert a number, a name, or a verb.

| Before (generic) | After (PRFAQ voice) |
|---|---|
| "Our new product is exceptional and provides a robust solution." | "Customers cut average page-load time from 2.1s to 380ms." |
| "We are excited to announce a revolutionary new service." | "Today AWS launches Foo, a managed service for X." |
| "The solution may help customers improve efficiency." | "Customers complete the workflow in 3 steps instead of 11." |
| "Many users have reported significant time savings." | "Pinterest reduced index-build time from 14 hours to 22 minutes." |
| "Our world-class team leveraged cutting-edge AI." | "Foo uses a 7B-parameter model trained on 18 months of support tickets." |
| "It is believed this approach will likely yield substantial benefits." | "This approach cuts cost per request by 60%." |
| "Users seamlessly integrate with their existing workflow." | "Customers connect Foo with one IAM role and one API call." |
| "We deliver a comprehensive, end-to-end solution for enterprise needs." | "Foo handles ingest, transform, and query in a single API." |
| "This is a paradigm-shifting innovation." | "Until today, customers ran two separate jobs. Now they run one." |
| "Our product empowers developers to build better software faster." | "Developers ship the same feature in 1 day instead of 1 week." |
| "The new feature offers various improvements." | "The feature does three things: A, B, and C." |
| "A number of customers have expressed interest." | "Six of the top ten US retailers run Foo in production." |
| "This provides a significant competitive advantage." | "Customers respond to outages 4x faster." |
| "Our platform is intuitive and easy to use." | "A developer launches their first Foo cluster in under 5 minutes." |
| "We leveraged industry-leading expertise to architect a best-of-breed solution." | "Foo runs on the same infrastructure that serves Prime Video." |

## Customer quote — anatomy

The hardest sentence in the PRFAQ. The default LLM (and the default human author) produces marketing voice; the discipline produces customer voice.

### Bad quote (auto-reject)

> "We're thrilled to partner with AWS. Their world-class platform has been a game-changer, empowering us to deliver innovative experiences to our customers."

Unusable: five weasel/marketing words; no situation; no number; no problem; could be about any vendor.

### Good quote (passes)

> "Before [Foo], reindexing our 240B-pin catalog took 14 hours. We held releases for it. Now it takes 22 minutes, so we ship reindexes during normal business hours."

It works because:
1. **Specific situation:** reindexing the catalog.
2. **Numbers that matter:** 14h → 22m, 240B pins.
3. **Problem-relevant detail:** "we held releases for it" — the reader feels the pain.
4. **Behavioral consequence:** "ship during normal business hours" — what they now *do differently*.
5. **The customer's voice, not the company's** — no "thrilled," no "partner."

### Five-question customer-quote test

Every customer quote must pass all five:

1. Can you name the specific job or workflow?
2. Is there a number in the quote?
3. Does it describe what they *did before*?
4. Does it name a behavior they changed?
5. Could you swap the company name for a competitor and have the quote still make sense? **If yes, rewrite.**

### Spokesperson vs. customer

- **Spokesperson** (internal exec): states *the why*. Allowed to be visionary; not allowed to be excited.
- **Customer**: states *what changed*. Allowed to be flat. **Flat is more credible than enthusiastic.**

### When no real customer signal exists

Insert `[REAL QUOTE NEEDED — paraphrase from <evidence source: interview / pilot transcript / Slack message>]` in the quote slot and mark the PRFAQ NOT READY. Do not fabricate a quote. This placeholder is allowed **only when the other three required inputs (segment, pain-with-evidence, named alternative) are concrete and only the quote is missing.**

## Voice fingerprint — the editorial checklist

15 markers. A genuine PRFAQ passes most or all.

1. A **specific customer is named** (named company, named industry, or named role) — not "users."
2. A **number appears in the first paragraph** — latency, price, size, count, percentage.
3. **No banned words.** Run the kill lists before submitting.
4. **No sentence longer than 30 words.** Median ~15.
5. **Active voice in ≥90% of sentences.**
6. The **customer's "before" is described concretely** — pain felt in 2 sentences.
7. The **customer's "after" is a behavior change**, not a feeling. "Ships during business hours" beats "feels confident."
8. The **customer quote names a specific job, includes a number, and is not swappable to a competitor.**
9. The **spokesperson quote states *why*, not what.** Visionary is fine; excited is not.
10. **No "we are excited to announce."** Just announce.
11. **No bullet-pointed feature list in the press release body.** Features become prose with verbs.
12. **Plain-language analogies for novel concepts** — e.g., "Storage for the Internet."
13. **Reads as if launch day already happened.** No "we plan to" / "in coming months."
14. **FAQ answers hard questions**, not friendly ones — "What does this cost the customer?" "What's the worst-case failure?" "Why didn't we build this two years ago?"
15. **One idea per paragraph. Headings are descriptive, not clever.**

## Tonal touchstones

What good PRFAQ voice draws from:

- **Strunk & White**, *The Elements of Style* — a touchstone for Amazon writing. "Omit needless words." "Use the active voice." "Use definite, specific, concrete language."
- **George Orwell**, "Politics and the English Language" — six rules including no stale metaphor, short word over long, cut every cuttable word.
- **Steve Jobs** — concrete customer outcome over spec ("1,000 songs in your pocket"). What Amazon does NOT take: theatrical superlatives, presenter-as-protagonist.
- **Hemingway** — short sentences, verbs that do work, few subordinate clauses.

## Sources

- Amazon 2017 shareholder letter: https://www.aboutamazon.com/news/company-news/2017-letter-to-shareholders
- CNBC, 6-page memos: https://www.cnbc.com/2018/04/23/what-jeff-bezos-learned-from-requiring-6-page-memos-at-amazon.html
- 4 Cs of Writing at Amazon (Peter Tilsen): https://www.linkedin.com/pulse/4-cs-writing-amazon-peter-tilsen-8fnnc
- Write Like an Amazonian: https://medium.com/@apappascs/write-like-an-amazonian-14-tips-for-clear-and-persuasive-communication-e2a11afc7362
- Ian Nowland on 6-page and 2-page docs: https://inowland.medium.com/using-6-page-and-2-page-documents-to-make-organizational-decisions-3216badde909
- Francis Shanahan, one-pager like an Amazonian: https://francisshanahan.substack.com/p/how-to-write-a-one-pager-like-an
- theprfaq.com — Amazon writing culture: https://www.theprfaq.com/articles/amazon-writing-culture
- BusinessToday, Amazon meeting philosophy: https://www.businesstoday.in/technology/news/story/i-like-a-crisp-document-and-a-messy-meeting-inside-jeff-bezoss-meeting-philosophy-409580-2023-12-15
- Amazon Press Center, Kindle 2007: https://press.aboutamazon.com/2007/11/introducing-amazon-kindle
- Amazon Press Center, AWS S3 2006: https://press.aboutamazon.com/2006/3/amazon-web-services-launches
