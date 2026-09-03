# Package data-structure documentation standard

A concrete standard for documenting a package's **data-structure patterns** — the structures that flow across a package's API and carry a lifecycle, ownership, and invariants. The model is `internal/planner/doc.go` in sei-k8s-controller, which documents the *plan* as a structure that flows planner → status → executor → task, not merely the planner's API. That cross-package data-flow narrative is exactly what `go doc` on a single type cannot express and what a new contributor needs most.

This standard is language-general in spirit; the worked form below is Go (`doc.go`). For other languages, the equivalent is the package/module-level doc comment.

**This is the sanctioned exception to comment discipline (pack dimension D10), not a contradiction of it.** A package-pattern `doc.go` is the case the discipline names explicitly: something *above* the code (a lifecycle, an ownership boundary, a cross-package invariant) that names alone cannot convey. It describes the structure's *contract and current invariants*. Hold it to AGENTS.md **Output discipline** like any other comment.

## When it applies

A package must carry a pattern doc when it owns or threads a **non-trivial data structure with a lifecycle** — a plan, a state machine, a job/task graph, a cursor, a cross-package flow. Pure leaf utilities don't need one.

## Required sections

Each section is required; drop one only with an explicit `N/A — <reason>` line so the omission is visible, not silent.

1. **One-line purpose** — first sentence, starts with `Package <name>`. This is the `go doc` synopsis.
2. **Lifecycle** — the states the structure moves through and the transitions, as an ordered list. Name which package/component drives each step. (planner doc's "Plan Lifecycle" 1–5 is the exemplar.)
3. **Ownership boundaries** — *who mutates what.* The single highest-value section: most cross-package bugs are ownership violations. (planner doc's "Condition Ownership": "the planner owns conditions; the executor only mutates plan/task state.")
4. **Type roles** — each major type in one line: who constructs it, who reads it, who mutates it, and where it lives (in-memory vs persisted in `.status`/store).
5. **Invariants** — assertions that must always hold, phrased so a reviewer or test can check them. e.g. "a plan is persisted before any task executes (atomic creation)"; "`FailedPhase == \"\"` means retry, not terminal." Each should name its guarding test if one exists.
6. **Zero-value & sentinel semantics** — what `nil`/empty/zero mean for each ambiguous field. Go-specific; a reviewer flags code that treats these inconsistently.
7. **Concurrency & staleness** — the write-conflict story (optimistic lock, single-patch, cache vs `APIReader` reads) stated *here*, at the structure's definition, not only in CLAUDE.md.

## Where it lives

`doc.go` in the package that **owns the structure's lifecycle**. If two packages co-own (e.g. planner + task), the owning package's `doc.go` is canonical and the other links to it:

```go
// See internal/planner/doc.go for the plan lifecycle.
```

## How it stays honest (in priority order)

1. **Invariants are testable.** Every invariant should map to a named test or assertion. A documented invariant with no guarding test is a finding: *"documented but unguarded."*
2. **`go doc` renders correctly.** `go doc ./internal/planner` must produce a clean synopsis and readable sections. Broken rendering (a list that doesn't parse) is a finding.
3. **No dangling references.** Exported symbols named in `doc.go` must exist — `gopls`/`go vet` catch dangling refs on rename.
4. **No doc drift vs the agent file.** A "Key Pattern" in `CLAUDE.md` that isn't reflected in the owning package's `doc.go` is a finding — the agent file and the package doc must agree.

## Tooling (existing toolchain — no bespoke build)

- **`go doc ./pkg`** and **`go doc ./pkg.Type`** — primary render; verifies the synopsis (§1) is a valid one-liner.
- **`godoc` / `pkgsite`** — HTML render; confirms headings/lists (Go's doc-comment heading syntax) parse.
- **`golangci-lint`** — enable `ST1000` (require a package doc comment — the cheapest first step), `godot` (comment punctuation), `stylecheck` `ST1020`/`ST1021` (exported-symbol doc form).
- **`gopls`** — hover/rename safety; surfaces undocumented exported symbols.

**Deferred:** a bespoke AST staleness-linter that diffs `doc.go`'s "Type roles" against the package's actual exported types. Un-defer when 3+ packages have adopted this standard and drift is observed in review. Until then, honesty mechanism #4 (manual doc-drift check) and `ST1000` cover it.

## Reusable `doc.go` template skeleton

```go
// Package <name> <one-sentence purpose, starts with the package name>.
//
// # Lifecycle
//
// <Ordered states of the primary data structure and the transitions between
// them. Number the steps. Name which package/component drives each step.>
//
//  1. <Build>:    <who constructs it, from what inputs>
//  2. <Persist>:  <where it is stored — .status, ConfigMap, in-memory>
//  3. <Drive>:    <who advances it; sync vs async semantics>
//  4. <Terminal>: <complete/fail states and who observes them>
//
// # Ownership Boundaries
//
// <Who mutates what. State the division explicitly:
//  "<PkgA> owns <field/condition>; <PkgB> only mutates <other state>.">
//
// # Type Roles
//
// <Type>: <role — constructed by X, read by Y, mutated by Z, lives in
//          .status | memory>.
// <Type>: ...
//
// # Invariants
//
// <Assertions that must always hold, each checkable by a reviewer/test:>
//   - <invariant 1 — and the test that guards it, if any>
//   - <invariant 2>
//
// # Zero-Value & Sentinel Semantics
//
// <What nil/empty/zero mean for each ambiguous field.
//  e.g. "FailedPhase == \"\" means retry; a non-empty value is terminal.">
//
// # Concurrency & Staleness
//
// <The write-conflict story: optimistic lock, single-patch, cache vs
//  APIReader reads. State it here, not only in CLAUDE.md.>
//
// N/A sections: <list any deliberately omitted, with the reason>
package <name>
```

The existing `internal/planner/doc.go` already satisfies sections 1–4 well; under this standard it would add explicit **Invariants** (atomic plan creation; `FailedPhase==""` means retry), **Zero-Value semantics** (nil plan = steady state), and a **Concurrency** section (the optimistic-lock + single-patch story, which lives only in CLAUDE.md).
