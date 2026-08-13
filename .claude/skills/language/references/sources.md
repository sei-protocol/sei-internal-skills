# Sources & licensing

Part of the `/language` skill (its `SKILL.md` lands in PLT-479). This file plus `exemplars/` are the
corpus cite contract from **Design 03 — The exemplar corpus**
(https://github.com/sei-protocol/bdchatham-designs/blob/main/designs/sei-agentic-mesh/03-exemplar-corpus.md).
This `sources.md` is THE single license table — there is no separate licenses registry.

## The cite discipline (the contract BG-2 builds against)

A `/language` pack rule cites an exemplar as a **prose string**, resolved at author time by grep — the same
pattern `/idiomatic` uses ("Effective Go: Errors") and `/systems` uses (cite-by-short-name). No URI scheme,
no registry (both deferred per Design 03 until the corpus exceeds ~1 vertical and grep stops sufficing).

Cite vocabulary — `cite: <vertical>/<kind>/<target>`:

- `cite: hld/shape/<anchor>` → resolves to `exemplars/hld/canonical-shape.md#<anchor>`.
- `cite: lld/shape/<anchor>` → resolves to `exemplars/lld/canonical-shape.md#<anchor>` (PLT-490).
- `cite: hld/exemplar/<slug>` / `cite: lld/exemplar/<slug>` / `cite: one-pager/exemplar/<slug>` →
  resolve to `exemplars/<vertical>/annotated-exemplars.md#<slug>` — annotated pointers at public,
  process-vetted documents (positive exemplars only).
- `cite: one-pager/shape/<anchor>` → resolves to `exemplars/one-pager/canonical-shape.md#<anchor>`
  (PLT-491).
- `cite: prfaq/source/<Q-id>` → resolves via `exemplars/prfaq.md` (a pointer) into
  `/prfaq/references/primary-sources.md`, by Q-ID (Q1, Q4, …).
- `cite: prfaq/shape/<anchor>` → resolves via `exemplars/prfaq.md` into
  `/prfaq/references/canonical-shape.md#<anchor>`.

**The reserved-quote gate.** Every source carries a license class, and the class is load-bearing:

- **reserved** → cite-and-link only. Never reproduce the source text, and never paraphrase-to-evade
  (a close reword of reserved prose is a reproduction). A citation that would require quoting a reserved
  source is refused at author time — distill the idea in our own words and link instead.
- **openly-licensed** → adapt with attribution.
- **org-owned** → an internal Sei doc the org owns (these repos). Adapt with attribution; distill into
  our-own-words `canonical-shape`/annotation anchors, never reproduce source text wholesale (Design 03
  entry path). The first engineering-genre exemplars, harvested PLT-495. **Precedent (this is the class's
  first use):** quoted fragments *illustrate*, they don't *substitute* for the distillation — keep any
  verbatim span short and attributed; if a sentence can be paraphrased without losing the point,
  paraphrase it.

`canonical-shape.md` files are original analysis, copyright-clean by construction.

**Scope of this table — exemplar/shape sources only.** Doctrine *authority* cites (NN/g, Miller, Sweller,
plainlanguage.gov, the Anthropic context-engineering / tool-writing guidance) are cited **directly by
prose-string + URL** in the doctrine files, exactly the way `/idiomatic` cites "Effective Go" and
`/systems` cites by short name — they are not routed through this corpus.

## The flat table

One row per candidate source named in Design 03's verticals table — **license posture only, no harvested
text**. Vertical status is marked so the "≥1 openly-licensed anchor per net-new vertical" property is
visible even where the exemplar files are deferred (DEFERRED/candidate rows name sources whose
exemplar content doesn't exist yet; ACTIVE verticals have `exemplars/<vertical>/` files).

| Short name | Vertical | URL | Openness |
|---|---|---|---|
| arc42 | HLD (ACTIVE) | https://arc42.org | **CC-BY-SA 4.0** (verified 2026-06-12). Share-alike: adapted text forces CC-BY-SA on the derivative file — so **operating posture is cite-and-link**; adapt only if the adapting file can carry CC-BY-SA |
| C4 model | HLD (ACTIVE) | https://c4model.com | **CC-BY-4.0** (verified 2026-06-12) — adapt w/ attribution |
| Ubl: "Design Docs at Google" | HLD (ACTIVE) | https://www.industrialempathy.com/posts/design-docs-at-google/ | Reserved (cite-and-link only) |
| AWS Well-Architected | HLD (ACTIVE) | https://docs.aws.amazon.com/wellarchitected | Reserved — © Amazon (cite-and-link only) |
| Google SRE design chapters | HLD (ACTIVE) | https://sre.google/books | Reserved — CC-BY-NC-ND (cite-and-link only) |
| PRFAQ sources | PRFAQ (reuse) | /prfaq/references/primary-sources.md | **Owned by `/prfaq`** — NOT duplicated here; that file is the single source of truth for the PRFAQ set (Working Backwards, Amazon shareholder letters, Bryar Coda, Commoncog, …) |
| TIGER STYLE | LLD (candidate) | https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md | **Apache-2.0** (summarize ideas; adapt w/ attribution) — phrasing per `/systems/references/sources.md` |
| Google Engineering Practices | LLD (candidate) | https://google.github.io/eng-practices | **CC-BY-3.0** (adapt w/ attribution) — per `/systems` |
| Google AIP | LLD (candidate) | https://aip.dev | **CC-BY-4.0** (text), Apache-2.0 (samples) — per `/systems` |
| Rust RFCs | LLD (ACTIVE) | https://github.com/rust-lang/rfcs | **Apache-2.0 OR MIT** (verified 2026-06-12: LICENSE-APACHE + LICENSE-MIT at repo root; per-RFC — repo-wide relicensing in progress per the README, confirm the specific RFC before adapting its text) — adapt w/ attribution |
| Go proposals / design docs | LLD (ACTIVE) | https://github.com/golang/proposal | **BSD-3-Clause** (verified 2026-06-12: repo LICENSE) — adapt w/ attribution + notice |
| Ethereum EIPs | LLD (ACTIVE) | https://eips.ethereum.org | **CC0 1.0** (verified 2026-06-12: repo LICENSE.md + per-EIP mandated waiver) — public-domain dedication, freest class |
| Linux kernel docs | LLD (ACTIVE) | https://docs.kernel.org | **GPL-2.0 (default; verified 2026-06-12 per COPYING + per-file SPDX)** — **cite-and-link ONLY, never adapt** (copyleft); small `(GPL-2.0+ OR CC-BY-4.0)` dual-tagged exception set exists (admin-guide reporting/regressions docs) but the design exemplars (RCU/, memory-barriers.txt, process/) are GPL-only |
| sei-config config-manager DESIGN | LLD (ACTIVE) | sei-protocol/bdchatham-designs:designs/config-manager/DESIGN.md | **org-owned** (verified 2026-06-13; PLT-495) — adapt w/ attribution; rollout/sequencing exemplar. Relocated from sei-config docs/ (PLT-497). |
| seictl validation-substrate | LLD (ACTIVE) | sei-protocol/bdchatham-designs:designs/validation-substrate/validation-substrate.md | **org-owned** (verified 2026-06-13; PLT-495) — adapt w/ attribution; v1-ship-cut/un-defer-trigger exemplar. Relocated from seictl docs/ (PLT-497). |
| sei-k8s-controller import-volume (shapes) | HLD (ACTIVE) | sei-protocol/bdchatham-designs:designs/seinode-import-volume/design-seinode-import-volume.md | **org-owned** (verified 2026-06-13; PLT-495) — adapt w/ attribution; decision-matrix exemplar. Relocated from sei-k8s-controller docs/ (PLT-497). |
| PEPs | LLD (candidate) | https://peps.python.org | CC0 / PSF (openly-licensed) |
| ADR | LLD (candidate) | https://adr.github.io | CC (openly-licensed) |
| Oxide RFD | LLD (candidate) | https://rfd.shared.oxide.computer | Open — confirm (reserved until verified) |
| IETF RFCs | LLD (candidate) | https://www.ietf.org/rfc | Reserved — RFC 5378 / BCP 78 governs reproduction (cite-and-link only) |
| Atlassian PRD templates | PRD (DEFERRED) | https://www.atlassian.com/software/confluence/templates | Published for adoption (openly-licensed) |
| Cagan / SVPG: "How to Write a Good PRD" | PRD (DEFERRED) | https://www.svpg.com | Reserved (cite-and-link only) |
| Lenny's Newsletter | PRD (DEFERRED) | https://www.lennysnewsletter.com | Reserved (cite-and-link only) |
| Working Backwards (Bryar & Carr) | 1-pager (candidate) | https://www.workingbackwards.com | Reserved — trade book (cite-and-link only); facts citable, text not |
| Bezos shareholder letters / PowerPoint ban | 1-pager (candidate) | https://www.aboutamazon.com | Reserved — © Amazon (cite-and-link only) |
| McEnerney: "The Craft of Writing Effectively" | 1-pager (candidate) | https://www.youtube.com/watch?v=vtIzMaLkCaM | Reserved — public talk (cite-and-link only) |
| Shape Up (Basecamp) | 1-pager (candidate) | https://basecamp.com/shapeup | Reserved (cite-and-link only) |
| AR 25-50 (Army correspondence) | 1-pager (ACTIVE) | https://armypubs.army.mil/epubs/DR_pubs/DR_a/ARN42124-AR_25-50-007-WEB-13.pdf | **Public domain — 17 U.S.C. §105** (verified 2026-06-12) — adapt freely; doctrine only, not embedded third-party material |
| SECNAV M-5216.5 (Navy correspondence manual) | 1-pager (ACTIVE) | https://www.secnav.navy.mil/doni/ (verified copy: https://www.navyband.navy.mil/documents/secnav-m52165-ch1.pdf) | **Public domain — §105** (verified 2026-06-12 via the deep-linked PDF) — adapt freely |
| AFH 33-337 "The Tongue and Quill" | 1-pager (ACTIVE) | https://www.e-publishing.af.mil/ (verified copy: https://www.govinfo.gov/content/pkg/GOVPUB-D301-PURL-gpo67301/pdf/GOVPUB-D301-PURL-gpo67301.pdf) | **Public domain — §105** (verified 2026-06-12 via the deep-linked GPO copy) — adapt freely **except embedded literary epigraphs (third-party copyright)** |
| Federal Plain Language Guidelines + OMB M-11-15 | 1-pager (ACTIVE) | https://digital.gov/guides/plain-language/ · https://obamawhitehouse.archives.gov/sites/default/files/omb/memoranda/2011/m11-15.pdf | **Public domain — §105** (verified 2026-06-12; original 2011 guidelines via GSA archive — plainlanguage.gov redirects to digital.gov) — adapt freely |
| BVP published investment memos | 1-pager (ACTIVE) | https://www.bvp.com/memos | Reserved — freely published, © Bessemer (cite-and-link only) |
| Coinbase mission memo (Armstrong) | 1-pager (ACTIVE) | https://www.coinbase.com/blog/coinbase-is-a-mission-focused-company | Reserved — author-published (cite-and-link only) |
| BLUF practitioner essays (Ström-Awn; Sehgal/HBR; *Smart Brevity*) | 1-pager (ACTIVE) | https://mattstromawn.com/writing/bluf/ · https://hbr.org/2016/11/how-to-write-email-with-military-precision · https://www.hachettebookgroup.com/titles/jim-vandehei/smart-brevity/9781523516971/ | Reserved (cite-and-link only — no quoted fragments; *Smart Brevity* is a trade book) |

DEFERRED/candidate-row license postures are best-effort from Design 03 and re-verified at harvest before
any text is adapted; treat any unverified row as reserved. ACTIVE rows dated "verified 2026-06-12" were
checked first-hand against the repo license files (PLT-490 harvest) or, for the 1-pager rows, against
the official PD documents (PLT-491 harvest; §105 caveats: contractor and state-government works are NOT
PD, and embedded third-party material in PD manuals stays copyrighted). Vertical status: HLD, LLD, and
one-pager are ACTIVE — each has `exemplars/<vertical>/` canonical-shape + annotated-exemplars files;
`candidate` rows are named sources not yet exemplified.

**Registry un-defer trigger: MET as of PLT-490** — HLD and LLD are both ACTIVE with real entries, so
Design 03's condition ("the corpus exceeds ~1 vertical") now holds. The `language://` URI scheme + indexed
`registry.md` + resolver are **not built here** (the identifier scheme is a soft one-way door requiring a
deliberate human decision); they are the flagged next corpus build goal, no longer an indefinite deferral.
Until then, grep over prose-string cites remains the resolution mechanism.

Full provenance + per-vertical synthesis: Design 03 (link above).
