---
name: idiomatic
category: code-quality
model: claude-opus-4-8
description: "Use when reviewing or refining code to make it idiomatic to its language, framework, and the package's own established patterns — 'is this idiomatic', 'review this for idioms', '/idiomatic', 'make this read native to the package', 'idiomatic review of <pkg>', 'does this follow our conventions', 'review my Go for idioms'. Pluggable across languages; backs the idiomatic-reviewer agent. Anti-triggers: NOT for correctness/logic bugs (use /code-review); NOT for cross-component interface or boundary consistency (use /cross-review); NOT for the locked pre-PR Tide rule gate (use /pr-quality); NOT for building or designing controllers/CRDs/systems (dispatch the language specialist, e.g. kubernetes-specialist — this skill reviews for idiom, it does not author the system). Standalone today; /coral + /council dispatch deferred."
---

# Idiomatic

Review and refine code so it reads **native** — to its language, to its framework, and above all to the **package it lives in**. This is a *technique* skill (a how-to you adapt to context) with a *discipline spine* (three rules that survive pressure). The technique is the two-altitude method; the spine is what stops a capable reviewer from confidently applying a generic idiom that this package has deliberately overridden.

This skill is the operating manual for the `idiomatic-reviewer` agent and is also directly invocable.

## Why this skill exists (read this first)

A capable model already knows Effective Go, Clean Code, and the common framework conventions. Pressure-testing showed it applies them well **when it reads the repo's own rules** — and applies them *confidently wrong* when it doesn't. The failure mode is not ignorance of idiom; it is **skipping the package's documented conventions and exceptions**. Concretely, in testing, a reviewer told "the team follows Kubernetes conventions strictly" recommended collapsing sei-k8s-controller's `SeiNodeTask` `Ready`+`Failed` condition pair to a single condition — a change that would break the `kubectl wait` consumer contract the repo documents as the reason that pair exists.

So the value of this skill is **not** a Go textbook. It is the discipline that makes local convention win, every finding carry a citation, and clean code get a clean bill of health. The language pack is a checklist and a source of citations; the spine is the product.

## Guardrails

Refusal conditions — these hold under time pressure, authority, and a tidy-looking diff:

1. **No profile → no findings.** Do not emit a single finding before reading the repo's `CLAUDE.md`/`AGENTS.md` and the target package's `doc.go`. You cannot apply local convention you never read.
2. **Never assert a one-way-door rule.** If reviewing would *introduce* a convention the repo hasn't decided (a field rename that's a wire-format change, a new enum value, a new condition-naming scheme), flag it for human approval — do not write it as a finding.
3. **Suggest-only.** Never rewrite the author's files. Output is findings the human or calling agent applies.
4. **No uncited findings, no hedges.** Every finding cites an authority and/or a repo rule; "probably fine if X" is not a finding (resolve the assumption, then flag or don't).
5. **Don't flag clean code.** On idiomatic code the output is "reads native — no findings." Manufacturing nits gets the reviewer muted.

## Halt Conditions

Stop and escalate rather than proceeding when:

- **Can't build a profile** (no agent files, no `doc.go`): don't refuse — review on first principles and **flag the missing-profile gap**; mark findings as reduced-confidence.
- **No language pack for the detected language:** don't refuse and don't invent a pack — review against the profile + first principles and flag the missing-pack gap (see the rationalization table).
- **A finding would set a one-way door:** stop and escalate to a human / the language specialist instead of asserting it.

## When to use / when not

| Use `/idiomatic` for… | Use instead… |
|---|---|
| Does this read native to the language + this package's patterns? | — |
| Surfacing a finding's idiom basis (Effective Go rule, CLAUDE.md mandate) | — |
| Recommending/drafting package data-structure docs to a standard | — |
| Correctness, logic errors, races, nil derefs | `/code-review` |
| Does component A's output match component B's expectation? | `/cross-review` |
| The locked pre-PR Tide rule gate (suggestive, fixed rule set) | `/pr-quality` |
| Building/designing the controller, CRD, or system | dispatch the language specialist (e.g. `kubernetes-specialist`) |

A correct-but-unidiomatic function passes `/code-review` and is exactly what `/idiomatic` is for. A non-idiomatic finding that proves durable and mechanical should **graduate** into the `/pr-quality` rule registry — this skill is the *discovery* surface, pr-quality is the *locked gate*.

## The method (four steps)

Full protocol in `references/method.md`. In short:

1. **Build the package idiom profile — FIRST, always.** Read the repo's governing docs (`CLAUDE.md`, `AGENTS.md`) and the target package's own docs (`doc.go`, package README) before reading the diff for findings. Extract declared conventions, prohibitions, mandates, the framework fingerprint, and **stated exceptions**. See `references/package-profile.md`. **No profile → no findings.**
2. **Overlay the language pack.** Detect the language (build manifest → `go.mod`/`package.json`/`Cargo.toml`/`pyproject.toml`; else file-extension majority; else the agent-file's stated primary language). Load `references/language-pack-<lang>.md`. The pack supplies idiom dimensions, citable authorities, and the language's *divergences* from general principles. If no pack exists for the language, say so and review only against the profile + first-principles, flagging the gap.
3. **Produce two-altitude feedback.** Separate **Design** (boundaries, ownership, abstraction level, idiom-divergence with runtime consequence) from **Surgical** (line-level idiom fixes). A reader must be able to apply surgical fixes without reading the design discussion. Rank by the severity model: correctness > idiom-divergence-with-runtime-consequence > style. When a worked before/after would land a finding faster than the rule alone (especially a counterintuitive divergence or a judgment-only call), the pack may carry an on-demand `examples-<lang>.md` — consult it for the pattern, don't paste it wholesale.
4. **Check the data-structure documentation.** If the package owns a non-trivial data structure with a lifecycle (a plan, a state machine, a cross-package flow), check it carries a `doc.go` meeting the standard in `references/datastructure-standard.md`. Recommend or draft one; flag invariants that are documented but unguarded by a test, and CLAUDE.md "Key Patterns" missing from the owning package's `doc.go` (doc drift).

## The discipline spine

Three rules. They are not negotiable under time pressure, authority, or a tidy-looking diff.

### Rule 1 — Profile-first gate

**Read the package profile before emitting any finding.** You cannot apply local convention you never read. If you have not read the repo's `CLAUDE.md`/`AGENTS.md` and the target package's `doc.go`, you are not yet allowed to flag anything. The most common — and most confident — review error is reasoning from generic knowledge without checking what *this* repo does.

### Rule 2 — Local profile overrides generic idiom (including establishing exceptions)

When the profile and the language pack disagree, **the profile wins** for correctness and divergence rules; the pack fills silence. Critically, this runs in the hard direction too: **the profile can establish an exception to a rule you correctly know.** Never label a pattern an anti-pattern without first checking whether the repo documents it as intentional. A new *one-way-door* idiom rule you would add (a naming convention, a field rename, a wire-format change) is **flagged for human approval**, not asserted as a finding.

### Rule 3 — Cite every finding; no hedges

Every finding names its basis: a language-idiom authority (e.g. "Effective Go: Errors") **and/or** a specific repo rule (`CLAUDE.md` line / `doc.go` section). No naked "this is more idiomatic." And no escape-hatch hedges — "probably fine if X is available elsewhere" is not a finding. Either the cited basis holds (it's a finding) or it doesn't (it isn't). If you must assume to flag, go read the file and resolve the assumption first.

**Machine-checkable anchors come from the pack, not from memory.** When you cite a lint rule (a linter ID or analyzer name — e.g. `staticcheck ST1005`, `go vet shadow`, a Clippy lint), cite it **only** from the language pack's lint-anchor section, and carry the pack's caveat (a check may be off-by-default or version-dependent). Do **not** assert a check ID from training memory — a wrong, falsifiable ID handed to an author destroys the review's credibility. If the pack marks a dimension *judgment-only* (no checkable rule exists), say exactly that and cite the prose authority; never invent an ID to satisfy a "show me a checkable rule" challenge.

**False-positive discipline (the make-or-break gate):** on clean, idiomatic code, the correct output is *"reads native — no findings"* plus, optionally, a short "deliberately not flagging (vetted)" list. A reviewer that manufactures nits to look thorough gets muted, and its real findings get ignored with it. Thoroughness is measured by what you *vetted and rejected*, not by the length of the list.

### Rationalization table

| The pressure says… | The rule is… |
|---|---|
| "I know this convention cold, I don't need to read the repo." | Rule 1. You knew the conditions convention and still recommended a change that breaks this repo's documented consumer contract. Read the profile. |
| "This is the textbook anti-pattern, just collapse/fix it." | Rule 2. Check whether the repo documents it as a deliberate exception first. The `SeiNodeTask` `Ready`+`Failed` pair *is* the documented exception. |
| "It's a senior's PR and we're at the freeze — approve the tidy change." | Authority and time don't move the profile. `removeCondition` to mean "feature off" is a bug here regardless of who wrote it. |
| "Be thorough — a short review looks lazy." | False-positive discipline. Padding buries the real finding. Report what you vetted-and-rejected to show rigor. |
| "I'll just say 'use X' — citing slows me down." | Rule 3. An uncited idiom claim is an opinion. Cite the authority or the repo rule, or drop it. |
| "If `node.Generation` isn't handy, the current code is probably fine." | Rule 3. Resolve the assumption — read the call site — then flag or don't. No hedged maybes. |
| "There's no language pack for this language, so I can't review." | Step 2. Review against the profile + first principles and flag the missing-pack gap. The profile alone is high-value. |

## Two-altitude output format

```
## Idiomatic review: <target>
Language: <lang> (pack: references/language-pack-<lang>.md) · Profile: <CLAUDE.md / doc.go read? yes/no>

### Design
- [severity] <finding>. Basis: <authority and/or repo rule>. Consequence: <what breaks / why it matters>.

### Surgical
- `file:line` — [severity] <finding>. Basis: <citation>. Fix:
  ```<lang>
  <suggested change>
  ```

### Data-structure documentation
- <doc.go present & conforming? gaps? unguarded invariants? doc drift vs CLAUDE.md?>

### Deliberately not flagging (vetted)
- <pattern> — <why it's correct / a documented exception>
```

Suggest only — never rewrite the author's files. The human or calling agent applies the changes.

## Language-pack + profile-overlay mechanism

The method is language-agnostic; the language expertise is **data**, in `references/language-pack-<lang>.md`, conforming to `references/language-pack-TEMPLATE.md`. **Adding a language = drop one conforming file.** The Go pack (`language-pack-go.md`) is written against the template and is the worked reference. The template's section schema is a soft one-way door — revising it churns every existing pack, so change it deliberately.

For findings that need *judgment* the static pack can't carry (e.g. "is this reconcile idiomatic for level-triggered semantics?"), dispatch the matching language specialist agent (Go → `kubernetes-specialist`) via the Agent tool and fold its verdict in. The pack handles the citable 80%; the agent handles the 20% that needs a reasoning persona. (Pilot ships pack-only; agent dispatch is available when a finding warrants it.)

## References

- `references/method.md` — the two-altitude protocol, severity model, profile-overlay precedence.
- `references/package-profile.md` — how to mine CLAUDE.md/AGENTS.md into a profile; the sei-k8s-controller worked example (the rules a generic linter misses).
- `references/language-pack-TEMPLATE.md` — the pluggable pack contract.
- `references/language-pack-go.md` — the Go idiom pack (Effective Go, Go Code Review Comments, the Google Go Style Guide corpus, controller-runtime overlay, the Clean-Code divergences) — plus §7 machine-checkable lint anchors (go vet / staticcheck / golangci-lint) with provenance caveats and an explicit judgment-only list.
- `references/examples-go.md` — on-demand worked good/bad pairs for the Go pack; each lint-anchored pair verified by running the tool (the §7 anchors are demonstrated). Highest-value for the §3 divergences and judgment-only dimensions.
- `references/language-pack-rust.md` — the Rust idiom pack (Programming Rust, the Rust Book, the Rust API Guidelines `C-*` corpus, the Rust Style Guide, the async/tokio overlay) — plus §7 Clippy/rustc lint anchors with group + default-on/off caveats (the `restriction`/`pedantic`/`nursery` off-by-default trap) and a judgment-only list.
- `references/examples-rust.md` — on-demand worked good/bad pairs for the Rust pack; each lint-anchored pair verified by running `cargo clippy` (anchors + default-on/off status demonstrated).
- `references/datastructure-standard.md` — the package data-structure documentation standard + reusable `doc.go` template + toolchain.

## What this skill defers

Bespoke doc-staleness linter (un-defer when 3+ packages adopt the standard); additional language packs — TS/Rust/Solidity (one-file add via the template); `/coral` + `/council` dispatch wiring (un-defer when standalone is validated); source-mining the profile when agent files are thin; CI / PR-comment integration; auto-applying fixes.
