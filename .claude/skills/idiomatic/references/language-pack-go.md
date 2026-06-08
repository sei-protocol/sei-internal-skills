# Go idiom pack

The worked reference pack, written against `_TEMPLATE.md`. Cite §2 authorities in findings. Go's whole point in this skill: the base model knows most of this — the pack's job is to give findings a **citation** and to encode the **divergences** (§3) where generic software wisdom is *wrong* for Go.

## 1. dimensions[]

| id | dimension | idiomatic rule | cue | authority |
|----|-----------|----------------|-----|-----------|
| D1 | Naming | `MixedCaps` not `under_scores`; short names for short scopes (`i`, `t`, `err`); exported names get a doc comment starting with the name; initialisms keep case (`ID`, `URL`, `RBAC`, `S3`, `HTTP`); avoid stutter (`task.Task…`). | `under_score` idents; exported sym w/o doc comment; `Id`/`Url`/`Http`; `pkg.PkgFoo`. | GCR: Initialisms, MixedCaps |
| D2 | Error handling | Errors are values; wrap with `%w` + context; check with `errors.Is`/`errors.As`; error strings lowercase, no trailing punctuation; don't discard with `_` silently. | `fmt.Errorf` without `%w` when wrapping; `err.Error()` string-matching; `Errorf("Failed.")`. | Effective Go: Errors; GCR: Error Strings |
| D3 | Interface placement | Accept interfaces, return structs; define the interface in the **consumer** package, sized to what it uses; prefer narrow. | Interface defined next to its only implementation; wide interface where 2–3 methods are used. | Go Proverbs; GCR: Interfaces |
| D4 | Zero values | Make the zero value useful; nil slices/maps are valid for read; document sentinel zeros. | Constructor that only sets defaults; nil-map write; undocumented "empty means X". | Effective Go: The zero value |
| D5 | Concurrency | `context.Context` is the first param, never stored in a struct; every goroutine has a bounded lifetime and an observable exit. | `ctx` field on a struct; naked `go` with no join/cancel. | Effective Go: Concurrency; GCR: Contexts |
| D6 | Package boundaries | Package name = its purpose, not its contents; no `util`/`common`/`helpers` grab-bags; imports acyclic and one-way. | `package util`; re-export shim that pays no rent; import cycle. | Effective Go: Package names |
| D7 | Composition | Embedding for behavior reuse; interfaces for polymorphism — not type hierarchies. | Simulated inheritance; deep embedding for data-only reuse. | Effective Go: Embedding |
| D8 | Function shape | Return early; keep the happy path at minimal indentation; named returns only when they clarify or carry a deferred mutation. | `else` after a `return`; deeply nested happy path. | GCR: Indent Error Flow |
| D9 | Mutability & copies | Consistent value/pointer receivers per type; `DeepCopy()` shared cache objects before mutating. | Mixed receiver kinds on one type; mutating a client-cache object without copy. | controller-runtime cache semantics |
| D10 | Comment discipline | Comments are an **uncommon exception** — names and structure carry control flow and intent; the code should read without them. A comment earns its place only when something *above* the code must be explained that names can't convey (a non-obvious *why*, a cross-cutting invariant). Package/structural docs live in `doc.go` (see `datastructure-standard.md`) — the sanctioned exception, not a license for inline prose. **No historical/changelog reasoning in code** ("we used to…", "removed because the alert was noisy") — that belongs in the PR/commit, where an investigator finds it. | `// what` restating the next line; "we used to"/"previously"/commented-out code; a comment a rename would delete; a package owning a non-trivial structure with no `doc.go`. | Effective Go: Commentary; GCR: Comment Sentences |
| D11 | Generics discretion | Type params only when they remove real cross-type duplication; don't parameterize what one interface covers. | `[T any]` used once; generic where an interface is simpler. | Go generics guidance |

## 2. authorities[]

- **Effective Go** — go.dev/doc/effective_go — the canonical idiom reference.
- **GCR / Go Code Review Comments** — go.dev/wiki/CodeReviewComments — the review-time checklist.
- **Google Go Style Guide** — google.github.io/styleguide/go — decisions + best practices.
- **Go Proverbs** — go-proverbs.github.io — Rob Pike's compression of Go philosophy.
- **Uncle Bob / Clean Code** — applies where it agrees with Go (small functions, intention-revealing names, single responsibility); see §3 for where it does NOT.
- **controller-runtime docs** — pkg.go.dev/sigs.k8s.io/controller-runtime — for the framework overlay.

## 3. divergences[] — where Go rejects general software wisdom

This is the load-bearing section. Do **not** flag these.

- **Three similar lines beat a premature helper.** DRY/SRP pushes extraction at the second repeat; Go prefers a little duplication over a wrong abstraction. → Don't recommend a helper for 2–3 line repeats unless they're a correctness coupling (must-change-together).
- **Accept interfaces, return structs.** Inverts "program to an interface everywhere." Returning concrete types preserves caller optionality. → Don't recommend an interface return type for its own sake.
- **Errors are values, not exceptions.** No try/catch control flow, no panic-as-control-flow. → Clean Code's "prefer exceptions to return codes" is wrong for Go.
- **Small interfaces, defined late.** Define the interface at the consumer when a second implementation appears — not up-front with the implementation. → Don't recommend extracting an interface before there's a second caller.
- **No getters/setters.** Go drops the `Get` prefix and exposes fields. → Don't flag a public field for lacking accessors; don't recommend `GetFoo()`.
- **A little copying beats a little dependency.** → Don't recommend importing a package to dedupe a tiny helper.

## 4. anti_patterns[]

- **Sentinel-by-string** — cue: comparing `err.Error()` substrings. Rewrite: typed/sentinel error + `errors.Is`/`As`.
- **Context in a struct** — cue: a `ctx context.Context` field. Rewrite: pass `ctx` as the first arg of each method.
- **Grab-bag package** — cue: `package util`/`common`/`helpers`. Rewrite: move each symbol to a purpose-named package.
- **Naked goroutine** — cue: `go f()` with no cancel/join. Rewrite: bound it to a context and make exit observable.
- **`else`-after-`return`** — cue: an `else` block whose `if` already returned. Rewrite: drop the `else`, dedent the happy path.
- **Stutter** — cue: `task.TaskExecution`. Rewrite: `task.Execution`.
- **Changelog comment** — cue: a comment narrating past states ("we used to use X", "removed because the alert was noisy", "previously did Y"). Rewrite: delete it; the history lives in the PR/commit, where an investigating reader finds it — not in the source.
- **What-comment** — cue: a comment restating what the next line does (`// increment the counter` over `i++`). Rewrite: delete it; if the line isn't self-evident, fix the name rather than annotate it.
- **Commented-out code** — cue: blocks of code left commented out. Rewrite: delete; version control already has it.

## 5. framework_overlays[]

### controller-runtime

These add to D1–D11 and carry runtime consequences, so they rank above style.

| id | rule | cue | consequence |
|----|------|-----|-------------|
| CR1 | Reconcile is idempotent & level-triggered — converge from any observed state. | branching on "did X just happen" vs "what is the current state". | edge-trigger assumptions drop work on missed events. |
| CR2 | No `panic`/`log.Fatal`/`os.Exit` below `main` in controller code — return errors, let the queue retry. | any `panic` in reconcile/task code. | crashes the manager instead of retrying. |
| CR3 | Status patches use `client.MergeFromWithOptimisticLock{}`. | `client.MergeFrom(` base for a status write; bare `Status().Update`. | **silent stale-write race** (correctness). |
| CR4 | Single-patch model — one snapshot, mutate in-memory, one flush per reconcile. | >1 status write per path; a task writing status directly. | partial/clobbered status; broken idempotency. |
| CR5 | Cache-aware reads — use `APIReader` to read what this reconcile just wrote via SSA. | re-reading a just-written object through the cached client. | reads stale cache, re-does work. |
| CR6 | Requeue discipline — `RequeueAfter` for polling; errors already back off, don't double-requeue. | tight reconcile loop; returning both an error and a requeue. | hot-loops the queue. |
| CR7 | Condition hygiene — always-present, stable `Reason` enum, `ObservedGeneration` set. | `removeCondition` for "off"; dynamic `Reason`; missing `ObservedGeneration`. | breaks `kubectl describe`, PromQL `absent()`, staleness detection. |
| CR8 | Owner refs / stable SSA `fieldOwner` on child resources. | child created without an owner ref. | orphaned resources on delete; SSA field fights. |

## 6. severity_model

- **correctness** — CR3, CR4 (stale-write/clobber); D9 (cache mutation); any nil-map write.
- **idiom-divergence-with-consequence** — CR1, CR2, CR5, CR6, CR7, CR8; D2 when an unwrapped error loses the chain.
- **style** — D1, D8, D10, the §4 anti-patterns without a runtime consequence, and D11. Bundle these; never lead with them.

Profile note: this pack's CR-overlay and D-dimensions are **subordinate to the repo profile** (`package-profile.md`). The repo's documented exceptions (e.g. the `SeiNodeTask` `Ready`+`Failed` pair) override CR7. Always reconcile against the profile first.
