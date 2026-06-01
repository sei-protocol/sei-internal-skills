# Canonical PRFAQ shape

The PRFAQ is **one document, three parts**, ≤ 6 pages total. Page-and-element budget is a forcing function — restricting length develops better thinkers. (Bryar & Carr, *Working Backwards*: "Restricting length is a forcing function that develops better thinkers and communicators.")

## Three-part structure

| Part | Length | Voice | Job |
|---|---|---|---|
| Press release | ≤ 1 page (~350 words) | Customer's POV, future-dated as if launched | The headline customer experience; the litmus test for the product itself |
| External FAQ | 1–2 pages, 3–8 questions | Customer-facing, no jargon | Questions a customer or journalist would actually ask |
| Internal FAQ | 2–4 pages, 12–20 questions | Honest, technical-and-business | Where you figure out how to turn fiction into reality |

The internal FAQ is the bulk of the document. If the external FAQ is longer than the internal, the doc is a brochure — refuse.

> "The PR gives the reader the highlights of the customer experience. The FAQ provides all the salient details of the customer experience as well as a clear-eyed and thorough assessment of how expensive and challenging it will be for the company to build the product or create the service." — Bryar & Carr, *Working Backwards*

## Press release — section-by-section

Eight elements in this order. Synthesized from Bryar/Carr's Coda doc, McAllister's template, Calbucci's PRFAQ 101, and Wille's reproduction.

1. **Heading** — product name in the customer's language. Not a code name, not a marketing tagline. Bryar/Carr: "Name the product in a way the reader (i.e. your target customers) will understand."

2. **Sub-heading** — one sentence, ≤ 20 words, naming the market and the headline benefit. Bryar/Carr: "Describe who the market for the product is and what benefit they will get. Limit this to one sentence." McAllister's formula: `[COMPANY] ANNOUNCES [PRODUCT] TO ENABLE [CUSTOMER SEGMENT] TO [BENEFIT]`.

3. **Summary paragraph** — launch date and city, product overview, headline benefit. Assume the reader stops here. Bryar/Carr: "Assume the reader will not read anything else, so make this paragraph good."

4. **Problem paragraph** — the customer's problem in the customer's voice. Stands alone without naming the product. Concretely felt in two sentences.

5. **Solution paragraph(s)** — how the product solves the problem, in plain language, with named existing alternatives. Bryar/Carr's site: "Today, customers with this problem use x, y, or z products..."

6. **Spokesperson quote** — one internal exec quote, states the *why*. Visionary is allowed; excited is not.

7. **Customer quote** — a hypothetical but named and situated customer in their own voice, describing the benefit. See customer-quote anatomy in `voice-discipline.md`.

8. **Call to action / availability** — the concrete first action: URL, store, signup flow. "Available today at <URL>."

## External FAQ — what belongs

Customer-facing questions a press writer or buyer would actually ask. 3–8 questions, short answers. Reads like a real product-page FAQ.

Canonical topics:
- Pricing model
- Availability and regions
- How it works (high-level, no API names)
- Compatibility / requirements
- Support and SLAs
- How is this different from <named competitor>?
- Where to start

**Hard rule**: external FAQ uses customer language. No API names, no schema fields, no library names, no project code-names, no internal acronyms. If a journalist couldn't understand it, it's wrong.

**Adversarial questions are mandatory.** At least three of:
- "Why is this better than <named competitor>?"
- "What happens when <adverse scenario>?"
- "Why trust your numbers?"
- "Why didn't you build this two years ago?"

Softball-only FAQs are FM22 — refuse.

## Internal FAQ — what belongs

The harder questions. The bulk of the document. Bryar calls it "much longer" than the external FAQ. AWS Executive Insights: "The first half of the FAQ focuses on customer questions … The second half addresses our questions, like, 'Will this product be profitable?'"

Canonical topic list (12–20 questions, opportunistically expanded based on context):

| Topic | Sample question |
|---|---|
| Customer evidence | How do we know customers want this? (named sources, interviews, pilots, signups) |
| TAM and assumptions | What is the addressable market? What 5–10 named assumptions does that depend on? |
| Business model | What is the unit economics? What is the price? What is COGS? |
| Profitability timeline | When does this become profitable? Under what assumptions? |
| Competitive landscape | What are customers using today? Why would they switch? Why wouldn't they? |
| Technical feasibility | What's the engineering risk? What new capabilities does this require? |
| **What could kill this product?** | Mandatory. Existential risks, named concretely. |
| Alternatives considered | What did we consider and reject? Why? (Build / buy / partner / punt.) |
| Success metrics | What metrics measure success at 1 / 3 / 5 years? What are the early signals? |
| **Falsification thresholds** | Mandatory. What metric at what threshold would cause us to stop or pivot? |
| Single-threaded leader | Who owns this? |
| Key assumptions | 5–10 named assumptions with justification or source. |
| Regulatory / legal / privacy | Any non-engineering constraints? |
| Bill of materials | (For hardware) supply chain risk. |
| Dependencies | What does this require from other teams or vendors? |

**"What could kill this?" and "Falsification thresholds" are non-negotiable.** A PRFAQ without these is unfalsifiable theater. (See FM18 + FM23 in `failure-modes.md`.)

## Length and pacing

| Element | Length | Source |
|---|---|---|
| PR | ≤ 1 page, ~350 words | Bryar/Carr (book + site); McAllister pushes harder ("½ page") |
| External FAQ | 1–2 pages, 3–8 questions | Calbucci (explicit), Bryar/Carr (implicit categories) |
| Internal FAQ | 2–4 pages, 12–20 questions | Calbucci (12–18); Bryar/Carr public template (~20+) |
| Whole doc | ≤ 6 pages | Bryar/Carr (book), Calbucci (hard rule) |
| First-draft latency | A few hours, not a few days | Bryar/Carr |
| Polish-to-ship latency | Days to weeks, multiple iterations | Bryar/Carr; Dave Limp ("normally takes many iterations") |

> "We know that people read complex information at the rough average of three minutes per page, which in turn defines the functional length of a written narrative as about six pages for a 60-minute meeting." — Bryar & Carr, *Working Backwards*

## PR-first or FAQ-first?

**PR-first** (Bryar/Carr canonical): customer thesis is settled; write the PR first as a forcing function. If you can't write a one-page launch story, the thesis isn't done. Use when the team has strong customer evidence and is sequencing the writing.

**FAQ-first** (Calbucci debug mode): customer thesis is uncertain; write the customer/problem/market/value FAQs first to expose unstable assumptions, then write the PR last as a synthesis. Use when the team is trying to figure out *whether* to build, or when prior PR drafts kept changing.

**When in doubt: FAQ-first.** PR-first under uncertain inputs produces capability-driven framing (FM14).

## Reading ceremony

The PRFAQ is the meeting input. Not slides. Not a pitch. Amazon's reading mechanics:

- Document is **distributed at the start of the meeting**, not pre-read. Prevents "I skimmed it"; equalizes context.
- ~20 minutes silent reading for a 6-page memo (Julian Dunn codifies 10–12 for shorter docs, 20–30 for full PRFAQ).
- Attendees mark up the doc, write margin notes.
- ~30–40 minutes discussion. Author answers; room pressure-tests.
- Discussion is **truth-seeking, not selling** — "improving vs. deciding."
- Common exec question: "So what?"

> "I like a crisp document and a messy meeting." — Amazon leadership, recounted in BusinessToday

> "As soon as we agree on that document, the decision is made. That project is green-lighted. The next step is to find a single threaded leader to run that project." — Dave Limp, Amazon Chronicles

> "It is very rare that I would see a working backwards document like that for any new product…that we would approve the first time we saw it. Normally it takes many iterations." — Dave Limp

> "The fact that most PR/FAQs don't get approved is a feature, not a bug." — Bryar & Carr, *Working Backwards*

## MLP — the launch shape

> "We break our releases into 'minimum lovable products.' Each iteration addresses a significant customer need that they're excited to buy and use." — AWS Executive Insights

The PR is written about the *lovable* launch, not the minimum-shippable one. If you can't write a compelling PR for the first release, the first release isn't lovable yet.

MVP-style "what could we cut" thinking lives in the internal FAQ — but the cut version must still earn a press release a customer would care about. An MVP can be "technically launchable but no one would buy it"; an MLP is "the smallest version customers are excited to buy and use." **The PR is the test.**

## Sources

- workingbackwards.com (Bryar & Carr): https://workingbackwards.com/resources/working-backwards-pr-faq/ and https://workingbackwards.com/concepts/working-backwards-pr-faq-process/
- Bryar's Coda doc: https://coda.io/@colin-bryar/working-backwards-how-write-an-amazon-pr-faq
- Bryar & Carr, *Working Backwards* (St. Martin's, 2021)
- McAllister template: https://www.linkedin.com/pulse/working-backwards-press-release-template-example-ian-mcallister
- Calbucci, PRFAQ 101: https://www.theprfaq.com/prfaq-101
- AWS Executive Insights: https://aws.amazon.com/executive-insights/content/product-management-at-amazon/
- Dave Limp on Amazon Chronicles: https://amazonchronicles.substack.com/p/working-backwards-dave-limp-on-amazons
- Bill Carr, LinkedIn Oct 2024: https://www.linkedin.com/posts/bill-carr_in-the-mid-2000s-amazon-developed-the-working-activity-7394107910051516416-no5e
- Julian Dunn (review-meeting clock): https://www.juliandunn.net/2022/09/09/demystifying-the-pr-faq/
- Amazon 2017 shareholder letter: https://www.aboutamazon.com/news/company-news/2017-letter-to-shareholders
