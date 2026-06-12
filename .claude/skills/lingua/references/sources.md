# Sources & licensing

Part of the `/lingua` skill (its `SKILL.md` lands in PLT-479). This file plus `exemplars/` are the
corpus cite contract from **Design 03 — The exemplar corpus**
(https://github.com/sei-protocol/bdchatham-designs/blob/main/designs/sei-agentic-mesh/03-exemplar-corpus.md).
This `sources.md` is THE single license table — there is no separate licenses registry.

## The cite discipline (the contract BG-2 builds against)

A `/lingua` pack rule cites an exemplar as a **prose string**, resolved at author time by grep — the same
pattern `/idiomatic` uses ("Effective Go: Errors") and `/systems` uses (cite-by-short-name). No URI scheme,
no registry (both deferred per Design 03 until the corpus exceeds ~1 vertical and grep stops sufficing).

Cite vocabulary — `cite: <vertical>/<kind>/<target>`:

- `cite: hld/shape/<anchor>` → resolves to `exemplars/hld/canonical-shape.md#<anchor>`.
- `cite: prfaq/source/<Q-id>` → resolves via `exemplars/prfaq.md` (a pointer) into
  `/prfaq/references/primary-sources.md`, by Q-ID (Q1, Q4, …).
- `cite: prfaq/shape/<anchor>` → resolves via `exemplars/prfaq.md` into
  `/prfaq/references/canonical-shape.md#<anchor>`.

**The reserved-quote gate.** Every source carries a license class, and the class is load-bearing:

- **reserved** → cite-and-link only. Never reproduce the source text, and never paraphrase-to-evade
  (a close reword of reserved prose is a reproduction). A citation that would require quoting a reserved
  source is refused at author time — distill the idea in our own words and link instead.
- **openly-licensed** → adapt with attribution.

`canonical-shape.md` files are original analysis, copyright-clean by construction.

**Scope of this table — exemplar/shape sources only.** Doctrine *authority* cites (NN/g, Miller, Sweller,
plainlanguage.gov, the Anthropic context-engineering / tool-writing guidance) are cited **directly by
prose-string + URL** in the doctrine files, exactly the way `/idiomatic` cites "Effective Go" and
`/systems` cites by short name — they are not routed through this corpus.

## The flat table

One row per candidate source named in Design 03's verticals table — **license posture only, no harvested
text**. Vertical status is marked so the "≥1 openly-licensed anchor per net-new vertical" property is
visible even where the exemplar files are deferred (DEFERRED rows name sources; no `exemplars/<vertical>/`
files exist yet).

| Short name | Vertical | URL | Openness |
|---|---|---|---|
| arc42 | HLD (ACTIVE) | https://arc42.org | **CC-BY-SA 4.0** (verified 2026-06-12). Share-alike: adapted text forces CC-BY-SA on the derivative file — so **operating posture is cite-and-link**; adapt only if the adapting file can carry CC-BY-SA |
| C4 model | HLD (ACTIVE) | https://c4model.com | **CC-BY-4.0** (verified 2026-06-12) — adapt w/ attribution |
| Ubl: "Design Docs at Google" | HLD (ACTIVE) | https://www.industrialempathy.com/posts/design-docs-at-google/ | Reserved (cite-and-link only) |
| AWS Well-Architected | HLD (ACTIVE) | https://docs.aws.amazon.com/wellarchitected | Reserved — © Amazon (cite-and-link only) |
| Google SRE design chapters | HLD (ACTIVE) | https://sre.google/books | Reserved — CC-BY-NC-ND (cite-and-link only) |
| PRFAQ sources | PRFAQ (reuse) | /prfaq/references/primary-sources.md | **Owned by `/prfaq`** — NOT duplicated here; that file is the single source of truth for the PRFAQ set (Working Backwards, Amazon shareholder letters, Bryar Coda, Commoncog, …) |
| TIGER STYLE | LLD (DEFERRED) | https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md | **Apache-2.0** (summarize ideas; adapt w/ attribution) — phrasing per `/systems/references/sources.md` |
| Google Engineering Practices | LLD (DEFERRED) | https://google.github.io/eng-practices | **CC-BY-3.0** (adapt w/ attribution) — per `/systems` |
| Google AIP | LLD (DEFERRED) | https://aip.dev | **CC-BY-4.0** (text), Apache-2.0 (samples) — per `/systems` |
| Rust RFCs | LLD (DEFERRED) | https://github.com/rust-lang/rfcs | Apache-2.0 / MIT (openly-licensed) |
| PEPs | LLD (DEFERRED) | https://peps.python.org | CC0 / PSF (openly-licensed) |
| ADR | LLD (DEFERRED) | https://adr.github.io | CC (openly-licensed) |
| Oxide RFD | LLD (DEFERRED) | https://rfd.shared.oxide.computer | Open — confirm (reserved until verified) |
| IETF RFCs | LLD (DEFERRED) | https://www.ietf.org/rfc | Reserved — RFC 5378 / BCP 78 governs reproduction (cite-and-link only) |
| Atlassian PRD templates | PRD (DEFERRED) | https://www.atlassian.com/software/confluence/templates | Published for adoption (openly-licensed) |
| Cagan / SVPG: "How to Write a Good PRD" | PRD (DEFERRED) | https://www.svpg.com | Reserved (cite-and-link only) |
| Lenny's Newsletter | PRD (DEFERRED) | https://www.lennysnewsletter.com | Reserved (cite-and-link only) |
| Working Backwards (Bryar & Carr) | 1-pager (DEFERRED) | https://www.workingbackwards.com | Reserved — trade book (cite-and-link only); facts citable, text not |
| Bezos shareholder letters / PowerPoint ban | 1-pager (DEFERRED) | https://www.aboutamazon.com | Reserved — © Amazon (cite-and-link only) |
| McEnerney: "The Craft of Writing Effectively" | 1-pager (DEFERRED) | https://www.youtube.com/watch?v=vtIzMaLkCaM | Reserved — public talk (cite-and-link only) |
| Shape Up (Basecamp) | 1-pager (DEFERRED) | https://basecamp.com/shapeup | Reserved (cite-and-link only) |

DEFERRED-row license postures are best-effort from Design 03 and re-verified at harvest before any text is
adapted; treat any unverified row as reserved.

Deferred — `lingua://` URI scheme + indexed `registry.md` + resolver: un-defer when the corpus exceeds
~1 vertical of real entries (grep stops sufficing). Per Design 03; not built here.

Full provenance + per-vertical synthesis: Design 03 (link above).
