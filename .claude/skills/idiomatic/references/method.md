# Method — the two-altitude protocol

The reusable, language-agnostic procedure. The SKILL.md gives the four steps; this file is the detail.

## Step 1 — Build the package idiom profile (hard gate)

Before reading the diff *for findings*, read the repo's governing docs and the target package's own docs and build the profile. Details and the worked sei-k8s-controller example are in `package-profile.md`. The profile is the higher-priority overlay; the language pack fills what the profile is silent on.

Gate: if the profile has not been built, no findings may be emitted. This is Rule 1 of the discipline spine and exists because the dominant review error is confident generic reasoning that ignores what the repo actually mandates.

## Step 2 — Overlay the language pack

Detect the language deterministically, in this order:

1. **Build manifest** — `go.mod` → Go; `package.json` → JS/TS; `Cargo.toml` → Rust; `pyproject.toml`/`setup.py` → Python; `foundry.toml`/`*.sol` → Solidity. Highest signal.
2. **File-extension majority** of the changed/target set — tiebreak and fallback.
3. **Agent-file hint** — a primary language named in `CLAUDE.md`/`AGENTS.md` disambiguates polyglot repos (a Go controller with embedded shell/yaml stays "Go").

Load `references/languages/<lang>.md`. If none exists, review against the profile + first principles and **flag the missing-pack gap** — do not refuse. The profile alone is high-value.

## Step 3 — Two-altitude feedback

Two altitudes, **explicitly separated** so a reader can act on the surgical fixes without first resolving the design discussion.

**Design altitude** — structure, package boundaries, ownership, abstraction level, and idiom-divergence that carries a runtime consequence. "This puts condition-setting in the executor, but the profile says the planner owns conditions" is design-level: it changes a boundary and encodes an invariant.

**Surgical altitude** — a specific line, a specific idiom, with a concrete suggested change. "`file:line`: this status patch base is plain `MergeFrom`; the repo mandates `MergeFromWithOptimisticLock{}`" is surgical.

### Severity model

Rank every finding:

1. **Correctness** — it is wrong or races (e.g. a stale-write race from a non-optimistic-lock status patch). Always wins.
2. **Idiom-divergence with runtime consequence** — idiomatically wrong *and* it breaks something observable (removing a condition breaks `kubectl describe` / PromQL `absent()`; a missing `ObservedGeneration` makes staleness undetectable).
3. **Style** — idiomatically off with no runtime consequence (naming, import grouping, comment form). Lowest. Bundle these; never lead with them.

A language pack's `severity_model` section maps that pack's dimensions onto these three tiers.

## Step 4 — Data-structure documentation check

See `datastructure-standard.md`. Applies when the package owns a non-trivial structure with a lifecycle. Outputs: doc present & conforming? missing sections? invariants documented-but-unguarded? CLAUDE.md "Key Pattern" absent from the owning package's `doc.go` (doc drift)?

## Profile-overlay precedence (the conflict rule)

When the profile and the language pack disagree:

- **Correctness / divergence rules → profile wins.** The repo's documented mandate or exception overrides the generic idiom. This includes the *hard direction*: the profile can establish an **exception** to a rule the pack (and you) correctly know. Check for a documented exception before flagging any textbook anti-pattern.
- **Pure style → pack fills silence.** Where the profile says nothing, apply the pack's style guidance — but style is lowest severity and never the headline.
- **New one-way-door rules → flag for human, don't assert.** If reviewing would have you *introduce* a convention the repo hasn't decided (a field rename that's a wire-format change, a new condition-naming scheme), surface it as a question for human approval, not as a finding. One-way doors are the repo's call, not the reviewer's.

## Citation and anti-hedge discipline (Rule 3)

Every finding carries a basis: an authority from the pack (`authorities[]`) and/or a specific profile rule (CLAUDE.md line, `doc.go` section). Uncited idiom claims are opinions — drop them or cite them. Reject hedged non-findings ("probably fine if X"): resolve the assumption by reading the file, then flag or don't.

## False-positive discipline

On clean idiomatic code: *"reads native — no findings."* Optionally list what you vetted-and-rejected to demonstrate the rigor went into screening candidates, not generating them. Never pad. The cost of a manufactured nit is that the next real finding gets ignored.
