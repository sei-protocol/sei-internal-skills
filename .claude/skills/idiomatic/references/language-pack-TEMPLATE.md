# Language pack contract (TEMPLATE)

A language pack is **data** the method loads. Every pack must provide the six sections below, in this order, so the method stays language-agnostic. **Adding a language = drop one file conforming to this template** at `references/language-pack-<lang>.md` and (optionally) map the language to a specialist agent.

This section schema is a **soft one-way door**: changing it churns every existing pack. Revise deliberately.

Copy the skeleton below and fill it. See `go.md` for a complete worked pack.

---

```markdown
# <Language> idiom pack

> Authorities loaded by this pack are listed in §2; cite them in findings.

## 1. dimensions[]

The named axes of idiom — the checklist. Each: an id, the idiomatic rule, a
detection *cue* (what the reviewer looks for), and an authority ref.

| id | dimension | idiomatic rule | cue | authority |
|----|-----------|----------------|-----|-----------|
| D1 | <e.g. naming> | <rule> | <what to look for> | <§2 ref> |
| …  | | | | |

**Required dimension — comment discipline.** Every pack MUST include a comment-discipline dimension, expressed in the language's idiom. The team standard it encodes: comments are an *uncommon exception* — names and structure carry control flow and intent, so the code reads without them; a comment earns its place only when something *above* the code must be explained that names can't convey. Structural/API docs (the language's package/module doc convention) are the sanctioned exception. **Historical/changelog reasoning never lives in code** ("we used to…", "removed because…") — it belongs in the PR/commit. **A deletion gets no tombstone**: when something is removed, it is removed — no comment naming what was removed or why, and **no "load-bearing context for the deletion" exception** (not even a security-relevant removal); the code states only *current* intent. Cue the reviewer to flag *what*-comments, changelog/history comments, **tombstone/removal-narration comments (and to refute the "but it's load-bearing context" excuse)**, commented-out code, and any comment a rename would delete.

## 2. authorities[]

The citable sources. Findings reference these by name — no naked claims.

- **<Short name>** — <full title / URL> — <what it governs>
- …

## 3. divergences[]

Where THIS language rejects a general software-engineering principle
(Clean Code, classic OOP, generic DRY). This is what makes the pack
opinionated rather than a linter restatement. Each: the general principle,
why the language rejects it, and the rule ("do NOT flag X").

- **<principle>** — general wisdom says X; <lang> says Y because Z.
  → Reviewer must NOT flag <pattern>.
- …

## 4. anti_patterns[]

Named smells. Each: the smell, a detection cue, and the idiomatic rewrite.

- **<name>** — cue: <…>. Rewrite: <…>.
- …

## 5. framework_overlays[]

Sub-packs keyed by framework that ADD or OVERRIDE dimensions, with the
framework's runtime consequences (these usually rank above style).

### <framework>
| id | rule | cue | consequence |
|----|------|-----|-------------|
| F1 | <…> | <…> | <what breaks at runtime> |

## 6. severity_model

How to map this pack's dimensions onto the method's three tiers
(correctness > idiom-divergence-with-runtime-consequence > style).

- correctness: <which dimensions/overlays land here>
- idiom-divergence-with-consequence: <…>
- style: <…>
```

---

## Optional: language → specialist agent map

When a finding needs judgment the static pack can't carry, the method dispatches a specialist agent. Record the mapping here as packs are added:

| language | specialist agent |
|----------|------------------|
| Go | `kubernetes-specialist` (controller-runtime work) |
| Solidity | `solidity-developer` |
| Rust | _(none yet — review on the pack + first principles; add a `rust-specialist` when Rust work is common)_ |
| … | … |
