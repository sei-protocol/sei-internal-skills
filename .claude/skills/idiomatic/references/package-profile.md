# Package idiom profile — mining the repo's own rules

The profile is the higher-priority overlay that makes local convention beat generic idiom. Build it from the repo's agent files and the package's own docs **before** emitting findings.

## What to read

1. **Governing docs** — `CLAUDE.md`, `AGENTS.md`, a constitution file. Repo-wide conventions, prohibitions, mandates.
2. **The target package's docs** — `doc.go`, package README, leading package comment. The structure's lifecycle and invariants live here.
3. **(Fallback only) source** — if the agent files are thin, corroborate from the code itself, and flag that the profile was inferred rather than declared. (Source-mining as a routine step is deferred; see SKILL.md.)

## What to extract

Sort everything you find into these buckets and tag each with an **enforcement status** — `lint` (a linter catches it), `test` (a test guards it), or `convention-only` (nothing mechanical enforces it). Convention-only rules are where this skill adds the most value: a linter will never catch them.

| Bucket | What it becomes | Example |
|---|---|---|
| **Declared conventions** | Style/structure expectations | "imports grouped stdlib / external / module" |
| **Prohibitions** ("Do not…", "Never…") | Correctness-grade anti-patterns | "no `panic` in controller code" |
| **Mandates** ("must…", "every … must…") | Required checks | "every `r.Status().Patch` base must use `MergeFromWithOptimisticLock{}`" |
| **Framework fingerprint** | Which pack overlay is in scope | "controller-runtime v0.23 / kubebuilder" |
| **Stated divergences** | Repo restating/inverting a generic idiom | "three similar lines beat a premature helper" |
| **Stated exceptions** | Carve-outs that *invert* a rule you know | the `SeiNodeTask` `Ready`+`Failed` pair |
| **One-way doors** | Constraints findings must NOT touch | CRD field names, event signatures |

## The precedence rule

Profile beats pack for correctness/divergence; pack fills silence for style; a *new* one-way-door rule is flagged for human, not asserted. See `method.md`.

## Worked example — sei-k8s-controller/CLAUDE.md

These are the load-bearing rules a generic Go reviewer **misses or gets backwards**. Each compiles and lints clean, so only the profile catches it.

| Rule (CLAUDE.md) | Why generic review misses it | Finding action |
|---|---|---|
| **Optimistic-lock status patches** — every `r.Status().Patch` base must be `MergeFromWithOptimisticLock{}`. Plain `MergeFrom`, bare `Status().Update`, SSA-on-status all compile & lint clean but lose stale-write races. | The reviewer sees a valid `Patch` call. The race is a semantic invariant about plan-creation idempotency. | **Correctness.** Flag any status write not built with the optimistic-lock base; cite CLAUDE.md "Status patches". |
| **Single-patch model** — one `DeepCopy` snapshot, mutate in-memory, one flush per reconcile. | No tool counts patches-per-reconcile-path; a second patch is legal Go. | Flag >1 status write per path, or a task writing status instead of mutating in-memory. |
| **Conditions always-present** — `removeCondition` to mean "feature off" is a **bug**; use `setCondition(False, <reason>)`. | A generic reviewer *praises* removing an inapplicable condition as cleanup. The rule **inverts** the generic instinct. | Flag condition removal in steady-state paths; require `False/NotApplicable`. Cite CLAUDE.md "Conditions". |
| **`SeiNodeTask` `Ready`+`Failed` mixed polarity** is the **documented exception** to "don't mix polarities" — the seitask-runner depends on `kubectl wait --for=condition=Ready` and `--for=condition=Failed`. | A reviewer applying the (correct, general) no-mixed-polarity rule recommends collapsing it — **breaking the consumer contract**. | **Do NOT flag.** List under "deliberately not flagging." This is the canonical Rule-2 exception. |
| **`ObservedGeneration` discipline** — every `setCondition` site sets `ObservedGeneration = obj.Generation`. The 4 direct `apimeta.SetStatusCondition` calls in `nodetask/controller.go` are a known divergence to fix on next edit. | A linter can't know a missing field makes staleness undetectable. | Flag direct `SetStatusCondition` lacking `ObservedGeneration`; treat the 4 known sites as expected-on-touch. No hedging. |
| **`Reason` is a stable enum / public API** — no dynamic data in `Reason` (that goes in `Message`); CamelCase. | The reviewer sees a string. | Flag interpolated/`fmt.Sprintf` `Reason` values. `planner.go`'s static reasons + dynamic Message are the positive exemplar. |
| **Atomic plan creation** — plan persisted before any task executes. | Ordering invariant across reconciles; no static tool models it. | Flag building a plan and executing it in the same reconcile without the intervening flush+requeue. |
| **No hand-editing generated files** (`zz_generated.deepcopy.go`, `manifests/`). | A linter lints them like any file. | Suppress findings inside generated files; flag *manual* edits to them. |
| **Anti-DRY stance** — "three similar lines beat a premature helper." | Generic DRY pushes helper extraction at the second repetition. | Do **not** recommend a helper for 2–3 line repeats unless the repetition is a correctness coupling (must-change-together). |

The lesson from pressure-testing: a capable reviewer knows the *general* conditions convention and will still recommend a change that breaks this repo — because it didn't read the exception. The profile is the cure.
