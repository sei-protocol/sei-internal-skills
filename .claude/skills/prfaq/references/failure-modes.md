# PRFAQ failure-mode catalog

28 failure modes the skill refuses and corrects. Every claim cited.

## How to read

Each failure mode has four parts:
- **Signal** — the observable text pattern in the document.
- **Why it fails** — what the customer or decider experiences.
- **Fix** — the concrete alternative.
- **Source** — where the pattern is documented.

The skill scans a PRFAQ (author or review mode) for each signal and either auto-rewrites (when the fix is mechanical) or refuses (when the failure is structural).

---

## FM1 — Customer-absent press release
**Signal:** PR never names a specific segment or situation. "Users", "customers", "people" appear without modifiers.
**Why it fails:** "If you think your product is for everyone, you are mistaken." Vague customer = unfalsifiable claim of need.
**Fix:** Open with a named, narrow persona in a concrete situation ("Sarah, a Series-A founder reconciling AWS bills quarterly"). One segment; broader audiences later.
**Source:** https://workingbackwards.com/concepts/working-backwards-pr-faq-process/

## FM2 — Solution-before-problem opening
**Signal:** Paragraph 1 describes the product ("Today we're launching X…") rather than the pain. Problem paragraph, if present, buried in paragraph 3+.
**Why it fails:** Canonical order is lead → problem → solution → internal quote → how-it-works → customer quote → call to action. Reversing it means the writer started from product and reverse-engineered need.
**Fix:** Enforce the seven-paragraph order. Problem paragraph precedes solution and stands alone without naming the product.
**Source:** https://www.linkedin.com/pulse/working-backwards-press-release-template-example-ian-mcallister

## FM3 — Buzzword soup
**Signal:** "Leverage", "synergies", "revolutionary", "cutting-edge", "next-generation", "world-class", "seamless", "robust", "transform the way", "unlock value", "frictionless".
**Why it fails:** Buzzwords substitute connotation for specificity. "You cannot hide behind vague statements like 'improve user engagement' or 'leverage synergies.'"
**Fix:** Banned-word list. Each banned word is rewritten as a concrete, measurable, customer-visible behavior change.
**Source:** https://shahmm.medium.com/write-the-press-release-before-you-build-the-product-is-it-a-sound-idea-cfcbc7e8a561

## FM4 — "We are excited to announce" cliché opening
**Signal:** Opens with announcer emotion: "thrilled to announce", "excited to share", "today marks a milestone", "pleased to introduce".
**Why it fails:** The PR is customer-facing; the customer does not care about announcer emotion. The single most reliable tell that the writer skipped the customer-narrative discipline.
**Fix:** Open with a customer outcome or a dated factual lead. Forbid "excited", "thrilled", "proud", "pleased" in the lead.
**Source:** https://workingbackwards.com/resources/working-backwards-pr-faq/

## FM5 — Feature-list-as-PR
**Signal:** PR body is a bulleted list of capabilities (3–7 bullets). Narrative arc replaced by a spec dump.
**Why it fails:** A press release is journalism, not collateral. Bullets let the writer dodge causal reasoning about how features chain to benefit.
**Fix:** Prose only in the PR. Features live in the "how it works" paragraph, expressed as customer-visible behavior — not implementation bullets.
**Source:** https://www.hustlebadger.com/what-do-product-teams-do/amazon-working-backwards-process/

## FM6 — Hedging language
**Signal:** "We believe", "we hope", "should", "may", "could", "soon", "designed to potentially".
**Why it fails:** A press release describes a launched product as fact. Hedging means the team hasn't committed to the outcome it's claiming.
**Fix:** Indicative, present tense, declarative. If a statement can't be made declaratively, lift the uncertainty into the FAQ as an open question.
**Source:** https://shahmm.medium.com/write-the-press-release-before-you-build-the-product-is-it-a-sound-idea-cfcbc7e8a561

## FM7 — No internal FAQ (FAQ as marketing afterthought)
**Signal:** Only an external/customer FAQ. No internal FAQ on economics, risk, dependencies, "what could go wrong".
**Why it fails:** Without an internal FAQ the doc is a brochure. Working Backwards requires separate Customer FAQs and Stakeholder FAQs.
**Fix:** Mandate both: Customer FAQs (5–10) and Internal FAQs (10–20 covering unit economics, dependencies, risks, kill criteria).
**Source:** https://workingbackwards.com/resources/working-backwards-pr-faq/

## FM8 — Length creep / buried thesis
**Signal:** PR >1 page (≈350 words). Charts in body. Appendices. Three quotes instead of two.
**Why it fails:** "Assuming that more means better… Restricting length is a forcing function that develops better thinkers and communicators."
**Fix:** Hard caps. PR: 1 page, ~350 words. Whole PRFAQ: 6 pages max. Charts only in appendix and only if referenced by an FAQ.
**Source:** https://workingbackwards.com/concepts/working-backwards-pr-faq-process/

## FM9 — Customer quote in marketing voice
**Signal:** "Mary the customer" quote is grammatically polished, names the brand twice, lists three benefits in parallel, ends with a superlative.
**Why it fails:** Real customer quotes are messy and mention one concrete thing. A polished quote means the team wrote it — i.e., they don't know what the customer will actually say.
**Fix:** Quote must (a) name a specific prior pain, (b) mention one new behavior, (c) sound like spoken English. No superlatives, no brand repetition.
**Source:** https://www.linkedin.com/pulse/working-backwards-press-release-template-example-ian-mcallister

## FM10 — Discounted or absent competition
**Signal:** No mention of how customers solve it today. Competitors, if named, are strawmanned. "There is nothing like this on the market" appears.
**Why it fails:** "It is a mistake to discount or not correctly factor in the competition; if the customer problem is big, they already have some method of dealing with it today."
**Fix:** Mandatory "Alternatives today" entry naming at least one real competitor or workaround, honestly stating where it works and where it doesn't.
**Source:** https://commoncog.com/putting-amazons-pr-faq-to-practice/

## FM11 — "Tenets" as platitudes
**Signal:** Tenets read as values — "Be customer obsessed", "Be bold", "Move fast". Non-falsifiable; nobody could violate one by accident.
**Why it fails:** Tenets must be tie-breakers. Platitudes can't resolve arguments because both sides claim them. "Customer Obsession is one of the most overused, abused, and weaponized terms."
**Fix:** Each tenet must be a falsifiable trade-off: "We will [X] even at the cost of [Y]" — e.g., "We refuse to ship without per-customer telemetry, even if it delays launch a sprint."
**Source:** https://fortune.com/2024/07/31/amazon-leadership-principles-questions-future-jeff-bezos-departure-andy-jassy/

## FM12 — PRFAQ-as-spec-sheet (technical detail leak)
**Signal:** "How it works" or external FAQ contains API names, schema fields, infra topology, library choices, latency in ms. Words customers wouldn't use.
**Why it fails:** "The External FAQ is a dialogue with your customer… no corporate jargon. A pitfall is speaking in technical or industry jargon." Lets engineers anchor on implementation before strategy is decided.
**Fix:** External prose passes the "would a journalist write this?" test. Implementation moves to a design doc; high-level architecture allowed in an internal FAQ if load-bearing.
**Source:** https://medium.com/agileinsider/press-releases-for-product-managers-everything-you-need-to-know-942485961e31

## FM13 — Internal acronyms / code-names in customer prose
**Signal:** PR or external FAQ contains acronyms/code-names only employees recognize — "the OPS-2 stack", "Project Atlas", "our T2 cohort".
**Why it fails:** A real press release is read by a reporter who doesn't know your org. Acronyms are the cleanest tell that the writer never inhabited the customer's perspective.
**Fix:** Every acronym in customer-facing sections is defined inline once, is widely-recognized industry (SaaS, API), or is deleted. Project code-names forbidden in the PR and external FAQ.
**Source:** https://workingbackwards.com/resources/working-backwards-pr-faq/

## FM14 — Capabilities-forward framing
**Signal:** Logic is "we have X, therefore customers want Y." Internal quote mentions company strengths before outcomes. Problem paragraph derivative of the solution.
**Why it fails:** "If your press release is based on your company's current technology… capabilities rather than customer needs, it should be discarded before even writing an FAQ."
**Fix:** Strip every internal-capability reference from the PR. If problem paragraph can't survive without naming company tech, PRFAQ is capability-driven and should be rejected.
**Source:** https://shahmm.medium.com/write-the-press-release-before-you-build-the-product-is-it-a-sound-idea-cfcbc7e8a561

## FM15 — PRFAQ as plan / roadmap
**Signal:** Roadmap, milestone list, sprint plan, OKR table, "phase 1/2/3" rollout. Timeline answered with a Gantt chart.
**Why it fails:** "A strategy is not a plan." Plan-shape invites premature date/scope commitment before the thesis is validated.
**Fix:** No timelines beyond one practical launch date (forcing function, not commitment). Planning lives in a separate doc after approval.
**Source:** https://www.theprfaq.com/articles/7-common-prfaq-mistakes-that-sabotage-great-products

## FM16 — Assumptions presented as fact
**Signal:** Market sizes, conversion rates, WTP, adoption curves stated without hedge or citation. No "What are we assuming?" entry.
**Why it fails:** Pretending certainty about an unbuilt product is the failure the FAQ half is supposed to surface. Reviewers can't tell what to challenge.
**Fix:** Mandatory "Key assumptions" FAQ listing 5–10 named assumptions with justification or source. Numbers in the PR are cited or labeled to-be-validated.
**Source:** https://www.theprfaq.com/articles/7-common-prfaq-mistakes-that-sabotage-great-products

## FM17 — Too-big vision / 5-year hand-wave
**Signal:** Launch date 3–5+ years out or absent. Transformative language ("revolutionize the industry"). Scope unfalsifiable.
**Why it fails:** "Visions too distant or expansive don't guide concrete strategic decisions… they lose credibility as mechanisms for current action."
**Fix:** 3–12 month practical launch (18–24 for genuinely complex). Bigger vision in one FAQ entry, not the PR. No near-term deliverable → premature.
**Source:** https://www.theprfaq.com/articles/7-common-prfaq-mistakes-that-sabotage-great-products

## FM18 — Selling-not-truth-seeking (approval theater)
**Signal:** Doc reads as a pitch deck in prose. Every section sells. No serious risk named. Authors defensive to critique. "Ta-da!" presentation expecting praise.
**Why it fails:** PRFAQ's purpose is to "identify barriers to success and articulate solutions honestly", not to win approval. Shaped for "yes" → reviewers can't see whether thinking is sound.
**Fix:** Internal FAQ must answer "What could go wrong?", "Why might this fail?", "What would cause us to kill the project?" concretely. Applause on first draft is a smell.
**Source:** https://www.theprfaq.com/articles/7-common-prfaq-mistakes-that-sabotage-great-products

## FM19 — Misaligned solution
**Signal:** Problem paragraph names pain A; solution addresses pain B. Or solution covers only a slice but PR implies all. Customer quote praises a benefit the product doesn't deliver.
**Why it fails:** Common in adjacent-market projects: team has a solution and reverse-engineers a problem that almost-but-not-quite matches.
**Fix:** Explicit problem-solution mapping in internal FAQ: list each problem and which part of the solution addresses it, with honest "not addressed" entries.
**Source:** https://www.theprfaq.com/articles/7-common-prfaq-mistakes-that-sabotage-great-products

## FM20 — PRFAQ written too late
**Signal:** First drafted as launch nears, mid-build, or as post-hoc justification. Tense slips between "will" and "does".
**Why it fails:** "The worst time to write a PRFAQ is just before launch." Late-PRFAQ becomes retrospective theater.
**Fix:** PRFAQ is gating — no significant engineering commitment until reviewed and approved. Mid-build requests indicate misalignment, not a missing artifact. A **labeled retrospective PRFAQ** (header clearly marks it as authored post-build, written in present/past tense, with internal FAQ doing the load-bearing work) is a legitimate alternative; a PRFAQ pretending to predate the build is the failure mode.
**Source:** https://www.theprfaq.com/articles/7-common-prfaq-mistakes-that-sabotage-great-products

## FM21 — Polished-prose perfectionism
**Signal:** Parallelism, anaphora, thesaurus vocabulary. Multiple edit rounds on word choice but no new evidence.
**Why it fails:** "What they need is to develop clarity of thought." Polish hides muddled thinking and makes the doc brittle to challenge.
**Fix:** First draft ugly. Edit for clarity, not style. If a sentence can't reduce to subject-verb-object without losing meaning, it probably hides a missing fact.
**Source:** https://www.theprfaq.com/articles/prfaq-starting-guide

## FM22 — Softball external FAQ
**Signal:** FAQs read like "How easy is it to get started?" — invitations to praise. No "Why wouldn't I…" or "What if X doesn't work?".
**Why it fails:** The FAQ exists to surface objections. A FAQ without hard ones is marketing copy, not a decision artifact.
**Fix:** ≥3 adversarial questions — "Why is this better than X?", "What happens when [adverse scenario]?", "Why trust your numbers?" — answered with specifics.
**Source:** https://workingbackwards.com/resources/working-backwards-pr-faq/

## FM23 — No falsification entry
**Signal:** Internal FAQ names no conditions to stop or pivot. No success metric paired with a failure threshold.
**Why it fails:** Unfalsifiable PRFAQ — every result reads as validation. The deepest theater failure mode.
**Fix:** Mandatory "What metrics would cause us to stop or pivot?" Each top-line claim in the PR gets a paired falsification threshold.
**Source:** https://workingbackwards.com/concepts/working-backwards-pr-faq-process/

## FM24 — Formatting cheats (bullets, charts, color)
**Signal:** Bullet/numbered lists, color highlights, screenshots, diagrams, callout boxes, sized headings compensating for thin prose.
**Why it fails:** Narrative is the forcing function. "Don't use images or diagrams. Use only black text… Avoid bullet points. Write in narrative form." Bullets let writers dodge transitions and trade-offs.
**Fix:** Plain prose. Line numbers for review. Charts only in appendix and only if referenced by an FAQ. Headings short and unstyled.
**Source:** https://www.theprfaq.com/prfaq-101

## FM25 — PRFAQ substituted for PRD / roadmap / OKR
**Signal:** PRFAQ is the only document. Eng uses it as spec; marketing as launch plan; leadership as OKR commitment. Doc grows as it absorbs roles.
**Why it fails:** PRFAQ answers "should we do this?". As PRD it underspecifies; as roadmap it overcommits. Conflation hides which decisions are made vs. open.
**Fix:** PRFAQ scoped to vision + strategy + customer thesis. PRD, plan, marketing, OKRs live in their own documents written after PRFAQ approval.
**Source:** https://www.theprfaq.com/prfaq-101

## FM26 — Weaponized PRFAQ
**Signal:** Exec assigns a PRFAQ for an initiative they've already decided to reject. Or reverse: rubber-stamped despite gaps because reviewer "likes the team." Reviews dominated by stylistic critique. "Stand out from other proposals" framing.
**Why it fails:** "Excessive focus on documentation leads to its weaponization… spinning off a couple of people to spend weeks writing a document they will reject later." Also "as weapons to put those who aren't favorites in their place."
**Fix:** Time-box first reviewable draft (≤ 2 weeks). Decision-maker states upfront what would change their mind. Feedback addresses content, not prose polish.
**Source:** https://www.theprfaq.com/articles/amazon-writing-culture ; https://fortune.com/2024/07/31/amazon-leadership-principles-questions-future-jeff-bezos-departure-andy-jassy/

## FM27 — Missing energy ("flat on paper")
**Signal:** PR reads as adequate but flat. Customer quote technically correct but emotionally inert. Team isn't enthused after reading their own draft.
**Why it fails:** "If you can't make it exciting in a press release, it won't be exciting in real life." Flat means the customer thesis isn't strong.
**Fix:** Gut-check is a hard gate. Don't paper over flatness with adjectives — rework the thesis. If unfixable, kill the project.
**Source:** https://www.optimizeforoutcomes.com/the-prfaq/

## FM28 — Solving the wrong problem (Fire-Phone pattern)
**Signal:** PR describes a novel capability no customer asked for. Internal quote emphasizes innovation. Problem paragraph thin because team backfilled it after the solution.
**Why it fails:** Fire Phone's Dynamic Perspective 3D was a feature Bezos demanded but "had the small problem that nobody seems to want their phone displays to be 3D." Internal alignment substituted for customer evidence.
**Fix:** Mandatory "Who asked for this?" FAQ answered with concrete pre-PRFAQ research (interview count, transcripts, signups, WTP data). No evidence trail → premature.
**Source:** https://www.productlessons.xyz/article/why-amazon-fire-phone-failed-case-study

---

## LLM-specific failure-mode shortlist

The patterns an LLM produces by default. The skill assumes guilty until disproven.

1. **FM4 — "We are excited to announce" cliché.** The default LLM press-release lead.
2. **FM3 — Buzzword soup.** "Leverage", "seamless", "revolutionary", "transform" without specificity.
3. **FM9 — Marketing-voice customer quote.** Polished, brand-mentioning, parallel-structure (training data is marketing-edited).
4. **FM5 + FM24 — Feature-list-as-PR + bullets.** Bulleted capability dumps.
5. **FM1 — Customer-absent PR.** Without a specified persona, LLMs write "for users", never a named persona in a concrete situation.
6. **FM22 — Softball external FAQ.** Uniformly reassuring; never adversarial.
7. **FM21 — Polished-prose perfectionism.** Smooth, literary, thesaurus-rich prose hides muddled thinking.
8. **Generic-context fluff.** "Without the context… ChatGPT would give me the average list of viral features." LLMs regress to the training-set mean.

**Source:** https://www.theprfaq.com/articles/vibe-coding-marketing-plans-prfaq-provides-ai-context

---

## PRFAQ theater diagnostic — 5 questions

Distinguishes ritualistic adoption from genuine practice. Theater PRFAQs dodge ≥3. **Fewer than 4 of 5 answerable with a pointer to specific text → refuse to bless.**

1. **"What would falsify the customer thesis?"** Genuine: named metric with a kill threshold ("kill if NPS < X at month 6"). Theater: only success metrics. (FM23)
2. **"What customer-evidence trail predates this document?"** Genuine: interviews, surveys, paid pilots cited. Theater: "we believe customers will…" or unsourced TAM. (FM16, FM28)
3. **"How does the customer solve this today, and what wouldn't they switch from?"** Genuine: named competitor with honest strengths. Theater: "nothing like this" / "current solutions are clunky." (FM10)
4. **"What's in the internal FAQ a marketing brochure couldn't say?"** Genuine: explicit assumptions, risks, unit economics, kill criteria. Theater: a technical version of the external FAQ. (FM7, FM18, FM23)
5. **"If the reviewer said 'I want to do this anyway,' what would change?"** Genuine: real conditions exist. Theater: nothing changes; doc is post-hoc rationalization. (FM20, FM26)

**Source:** https://www.theprfaq.com/articles/amazon-writing-culture ; https://fortune.com/2024/07/31/amazon-leadership-principles-questions-future-jeff-bezos-departure-andy-jassy/
