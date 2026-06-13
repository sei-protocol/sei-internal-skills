# Artifact pack — 1-pager (Amazon-style decision memo)

The third artifact pack. For each section of the 1-pager spine it rules **which audience-model rules
dominate** and cites the corpus shape (`cite: one-pager/shape/<anchor>` →
`exemplars/one-pager/canonical-shape.md#<anchor>`) and the strongest exemplar
(`cite: one-pager/exemplar/<slug>`). Rule names (R1–R5) and basis tiers are owned by
`audience-model.md`; rulings below inherit each rule's tier.

A 1-pager's two audiences pull hardest in different places: the human decider needs the ask before the
scroll and signs at the bottom; the agent reader needs every section typed by function — ask, options,
recommendation, decision, coordination — rather than inferred from prose. This genre is a **decision
memo**, distinct from `/prfaq`'s working-backwards artifact. The pack's job in Translate is to say, per
section, which way to lean — and the answer is almost always "lead harder."

| Section (`cite: one-pager/shape/…`) | Dominant rules | Ruling |
|---|---|---|
| `bluf-lead` | **R2** (peak), R1 | The genre's whole identity: the decision/ask in the first sentence or two, carrying the decision itself — not a recap of the analysis. The human who stops here still got the point; the agent weights it first. Bury the lead and the document stops being a 1-pager. `cite: one-pager/exemplar/ar-25-50-army-correspondence`. |
| `purpose-and-ask` | **R4** (peak), R1 | The contract line, typed: deliverable, decider, deadline as explicit fields — header-level intent typing (INFO/REQUEST/ACTION) an agent can route on. An ask phrased as a soft modal is an Open question, not an ask. `cite: one-pager/exemplar/sehgal-hbr-military-precision`. |
| `background-in-brief` | R2, R5 | Human-leaning, *demoted*: only the context the decider needs to parse the recommendation, placed after the point; color and orientation allowed, but no constraint lives only here. One paragraph — detail goes to enclosures. `cite: one-pager/exemplar/omb-m-11-15`. |
| `discussion-and-options` | R1, R4 | Agent-leaning: options and the trade-off that matters in list form, with recorded dissent (VIEWS OF OTHERS) kept on the page. The LLD's alternatives at one-tenth the length. When options span axes, a compact **decision-matrix table** beats prose (humans scan rows, agents read typed cells) — keep it to the page (page-discipline). `cite: one-pager/exemplar/tongue-and-quill`; `cite: hld/exemplar/seinode-import-volume-shapes` (the matrix form). |
| `recommendation` | **R2** (peak), R4 | One recommendation, stated as a sentence the decider can sign. Multiple recommendations are multiple memos. Decision-first, then rationale by reference. `cite: one-pager/exemplar/secnav-m-5216-5-navy-manual`. |
| `decision-block` | **R4** (peak), R1 | The agent-legible typed-field section: decision, decider, date written *onto the artifact* (Approved/Disapproved/Other). The memo is its own decision record — the difference between "a memo that argued" and "a decision that binds." Never prose; always typed fields. `cite: one-pager/exemplar/secnav-m-5216-5-navy-manual`. |
| `coordination-record` | **R4** (peak), R3 | Typed concurrence: one line per reviewer — name, position, concur/non-concur — or an explicit "NONE." Recorded dissent is part of the package, not a defect; an empty coordination line is itself content. `cite: one-pager/exemplar/secnav-m-5216-5-navy-manual`. |
| `page-discipline` | **R1** (peak), R2 | The editor, not a section: one page, enclosures carry the rest. What survives the cut is what the decider needs. Translate respects the ceiling — restructuring to fit the page is in scope; padding it back out is not. `cite: one-pager/exemplar/ar-25-50-army-correspondence`. |
| `audience-exception` | R2 (inverted), R5 | The genre's own documented profile-exception: for a hostile or low-credibility audience, doctrine sanctions leading *indirectly* — BLUF is the default, not a law. This is where the repo-profile / local-override mechanism applies (SKILL.md gate b): the exception must be *written down* to fire. `cite: one-pager/exemplar/tongue-and-quill`, `cite: one-pager/exemplar/strom-awn-bluf` (the boundary condition). |

## Translate notes specific to 1-pagers

- **Inventory pass priorities** (method step 3): a lead that recaps the analysis instead of carrying the
  decision (the R2 peak failure — `cite: one-pager/shape/bluf-lead`); an ask buried as a soft modal
  rather than a typed field; background that has swelled past one paragraph into a briefing; a
  recommendation that declines to take a position.
- **Typed fields stay typed.** `decision-block` and `coordination-record` are the agent-legible spine of
  this genre — decision/decider/date and name/position/concur. Translating these into flowing prose
  *removes* the type. Restructure within the fields; never dissolve them
  (`cite: one-pager/shape/decision-block`, `cite: one-pager/shape/coordination-record`).
- **The page is the constraint.** `page-discipline` is an editor, so a compression request that drops a
  *constraint or ambiguity marker* to fit the page is Guardrail 3, not editing — surface it. Cut color
  (R5) and detail-to-enclosure first.
- **Composition with /prfaq.** On a 1-pager that argues for building a product, `/prfaq` owns the
  working-backwards thesis: the 1-pager may *summarize and link* a PRFAQ, not replace or restate it.
  Defer to `/prfaq`, don't double-cover — the steward/`/prfaq` boundary (contribute the bi-audience lens, defer what `/prfaq` owns; Sei Agentic Mesh interface I3).

## Deferred (per the design's MVP cut)

PRD pack — one-file-add when a real consumer reviews that vertical. Until then PRDs translate against
`audience-model.md` + first principles with the missing-pack gap flagged.
