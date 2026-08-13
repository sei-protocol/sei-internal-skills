# Canonical 1-pager shape (the decision memo)

> **Corpus content — owned by the exemplar corpus (Design 03; harvested under PLT-491).** This file is
> original, our-own-words analysis of what a strong one-page decision memo carries. Doctrine rules cite
> it via `cite: one-pager/shape/<anchor>` (the citing pack is a future build goal — no pack cites these
> anchors yet); the section anchors below are **stable cite targets** — renaming one is a breaking
> change for citing rules.
>
> The shape is synthesized from U.S. federal writing doctrine — **public domain under 17 U.S.C. §105**,
> each source verified first-hand 2026-06-12: AR 25-50 (Army correspondence regulation, which mandates
> bottom-line-up-front at ¶1-38), SECNAV M-5216.5 (Navy correspondence manual, action/decision memo
> formats), AFH 33-337 *The Tongue and Quill* (the BLUF/eSSS organization methods), the Federal Plain
> Language Guidelines, and OMB M-11-15 — plus the cite-only practitioner layer in
> `annotated-exemplars.md`. §105 caveats observed: contractor-produced and state-government works are
> NOT public domain, and third-party material embedded in PD manuals (e.g. quotation epigraphs) stays
> copyrighted — the doctrine is harvested, never the embellishments.
>
> **Genre note (resolves Design 03's open question):** the 1-pager is a **decision memo** — it exists to
> get a named decision made by a named decider — and is *distinct from* the PRFAQ (a working-backwards
> product-discovery artifact owned by `/prfaq`). When a 1-pager argues for building a product, it may
> *summarize* a PRFAQ and link it; it does not replace one.

A 1-pager trades completeness for decision velocity: one page, one decision, one reader who can make it.
Both audiences are served by the same ruthlessness — the human decider gets the ask before the scroll;
an agent reader gets a document whose every section is typed by function (ask, context, options,
recommendation, decision) rather than inferred from prose. The military's version of this genre evolved
under the least forgiving reading conditions there are, which is why its conventions transfer.

---

## bluf-lead

The bottom line up front: the conclusion, recommendation, or ask in the first sentence or two — Army
doctrine makes this an explicit standard (AR 25-50 ¶1-38: main point at the beginning, active voice),
Navy guidance orders most-important-first, and the federal plain-language guidelines open documents with
purpose and bottom line. The practitioner formulation sharpens it: the lead must carry the decision
itself rather than recap the analysis (per Ström-Awn's BLUF essay). **Without it:** the decider reads context they didn't need to
decide, or stops before reaching the ask.

## purpose-and-ask

What this memo wants from whom, by when — typed, not implied. Navy action-memo format leads with the
action required and its due date as the first bullets; the eSSS staffing format opens with a PURPOSE
heading. For the agent reader this is the contract line: the deliverable, the decider, and the deadline
as explicit fields. **Without it:** the memo reads as analysis, and analysis gets filed, not decided.

## background-in-brief

Only the context the decider needs, placed *after* the point — the plain-language guidelines put
background toward the end unless the reader can't parse the main point without it. One short paragraph;
detail goes to attachments or links. **Without it:** either the decider lacks the one fact that makes
the recommendation legible, or — the commoner failure — background swells until the page is a briefing,
not a memo.

## discussion-and-options

The live considerations, compressed: the options weighed, the trade-off that matters, and dissent on the
record — the eSSS format gives DISCUSSION and VIEWS OF OTHERS their own headings, and coordination
doctrine treats recorded disagreement as part of the package, not a defect. The 1-pager analog of an
LLD's rationale-and-alternatives, at one-tenth the length. **Without it:** the recommendation reads as
the only option considered, and the decider re-derives the alternatives in the meeting you wrote the
memo to avoid.

## recommendation

One recommendation, stated as a sentence the decider can sign — the explicit RECOMMENDATION line is
mandated across the Army and Navy decision-memo formats. Multiple recommendations are multiple memos.
**Without it:** the memo asks for a decision while declining to take a position, which transfers the
work back to the decider.

## decision-block

The decision captured *on the artifact*: the military formats type Approved / Disapproved / Other lines
for the decider to mark, with signature or initials. In our context this is the section a decision gets
written into — date, decider, outcome — making the memo self-recording: the artifact and the decision
record are the same document. For the agent reader this is the difference between "a memo that argued"
and "a decision that binds." **Without it:** the decision lives in a meeting, a thread, or memory, and
the memo can't be cited as its record.

## coordination-record

Who has seen it and where they stand — the Navy format requires a COORDINATION line (or an explicit
"NONE"), and staffing doctrine treats a decision memo's credibility as a function of its coordination.
One line per reviewer: name, position, concur/non-concur. **Without it:** the decider can't tell a
socialized recommendation from a unilateral one, and the first objection arrives after approval.

## page-discipline

One page; enclosures carry the rest. Navy action memos are limited to one page unless the issue is
complex; Army doctrine defaults correspondence to a single page with short sentences and short
paragraphs. The constraint is the editor: what survives the cut is what the decider needs. **Without
it:** the memo grows into the document it was supposed to summarize.

## audience-exception

The documented out: for a hostile or skeptical audience, doctrine itself sanctions inverting the order —
*The Tongue and Quill* describes leading indirectly when the reader will resist the conclusion, and the
practitioner literature marks low sender-credibility as BLUF's boundary condition. This is the genre's
own profile-exception: BLUF is the default, not a law, and the exception is *written down*, which is
exactly how the doctrine's repo-profile mechanism expects local overrides to work. **Without it:** the
convention gets applied as dogma precisely where it loses the reader.
