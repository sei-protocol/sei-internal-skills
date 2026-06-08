# Go idiom pack

The worked reference pack, written against `_TEMPLATE.md`. Cite §2 authorities in findings. Go's whole point in this skill: the base model knows most of this — the pack's job is to give findings a **citation** and to encode the **divergences** (§3) where generic software wisdom is *wrong* for Go.

Sections: §1 dimensions · §2 authorities · §3 divergences · §4 anti-patterns · §5 framework overlays · §6 severity model. Grounded in Effective Go, Go Code Review Comments, and *Learning Go* (Bodner, 2nd ed.).

## 1. dimensions[]

| id | dimension | idiomatic rule | cue | authority |
|----|-----------|----------------|-----|-----------|
| D1 | Naming | `MixedCaps` not `under_scores`; short names for short scopes (`i`, `t`, `err`); exported names get a doc comment starting with the name; initialisms keep case (`ID`, `URL`, `RBAC`, `S3`, `HTTP`); avoid stutter (`task.Task…`). | `under_score` idents; exported sym w/o doc comment; `Id`/`Url`/`Http`; `pkg.PkgFoo`. | GCR: Initialisms, MixedCaps |
| D2 | Error handling | Errors are values; wrap with `%w` + context; check with `errors.Is`/`errors.As`; error strings lowercase, no trailing punctuation; don't discard with `_` silently. | `fmt.Errorf` without `%w` when wrapping; `err.Error()` string-matching; `Errorf("Failed.")`. | Effective Go: Errors; GCR: Error Strings |
| D3 | Interface placement | Accept interfaces, return structs; define the interface in the **consumer** package, sized to what it uses; prefer narrow. | Interface defined next to its only implementation; wide interface where 2–3 methods are used. | Go Proverbs; GCR: Interfaces |
| D4 | Zero values | Make the zero value useful; nil slices/maps are valid for read; document sentinel zeros. | Constructor that only sets defaults; nil-map write; undocumented "empty means X". | Effective Go: The zero value |
| D5 | Concurrency | Never start a goroutine without knowing how it stops — bounded lifetime, observable exit. `context.Context` is the first param, named `ctx`, never stored in a struct; `defer cancel()`. Unbuffered channels by default — buffer only for a known count or deliberate backpressure. Channels orchestrate, mutexes guard state; keep concurrency an implementation detail (don't expose channels in a public API). | naked `go` with no stop path; `ctx` field on a struct; missing `cancel()`; an arbitrary buffer size "for speed"; `select { …; default: }` busy-waiting; a public func returning a channel the caller must drain. | Effective Go: Concurrency; GCR: Contexts; Learning Go ch.12/14 |
| D6 | Package boundaries | Package name = its purpose, not its contents; no `util`/`common`/`helpers` grab-bags; imports acyclic and one-way. | `package util`; re-export shim that pays no rent; import cycle. | Effective Go: Package names |
| D7 | Composition | Embedding for behavior reuse; interfaces for polymorphism — not type hierarchies. | Simulated inheritance; deep embedding for data-only reuse. | Effective Go: Embedding |
| D8 | Function shape | Return early; keep the happy path at minimal indentation; named returns only when they clarify or carry a deferred mutation. | `else` after a `return`; deeply nested happy path. | GCR: Indent Error Flow |
| D9 | Mutability & copies | Go is call-by-value; pointers signal *mutability* — pass one only to mutate, to satisfy an interface, or for genuinely large data, not to avoid copying a small struct. Consistent value/pointer receivers per type. The `range` value is a copy — index into the slice to mutate. `DeepCopy()` shared cache objects before mutating. | `*SmallStruct` param/return with no mutation; pointer "for performance" on a tiny value; mixed receiver kinds on one type; `for _, v := range s { v.X = … }`; mutating a client-cache object without copy. | Learning Go ch.5/6; controller-runtime cache semantics |
| D10 | Comment discipline | Comments are an **uncommon exception** — names and structure carry control flow and intent; the code should read without them. A comment earns its place only when something *above* the code must be explained that names can't convey (a non-obvious *why*, a cross-cutting invariant). Package/structural docs live in `doc.go` (see `datastructure-standard.md`) — the sanctioned exception, not a license for inline prose. **No historical/changelog reasoning in code** ("we used to…", "removed because the alert was noisy") — that belongs in the PR/commit, where an investigator finds it. | `// what` restating the next line; "we used to"/"previously"/commented-out code; a comment a rename would delete; a package owning a non-trivial structure with no `doc.go`. | Effective Go: Commentary; GCR: Comment Sentences |
| D11 | Generics discretion | Type params when they remove real cross-type duplication or add type-safety an interface can't; prefer an ordinary interface when *behavior* (not the concrete type) varies. Use precise constraints (`comparable`, `cmp.Ordered`, `~int \| ~float64` unions), not `any`. | `[T any]` used once; a generic func whose body only calls interface methods; `[T any]` then a runtime type-switch inside. | Go generics guidance; Learning Go ch.8 |
| D12 | Slices & maps semantics | Slices share a backing array — `append`/re-slice alias; pre-size with `make([]T, 0, n)` when the length is known; hand a sub-slice to appending code via a three-index slice `s[lo:hi:max]`; `copy` for independence, `clear` to zero in place. Prefer a nil slice over `[]T{}` (unless JSON must emit `[]`). Maps: comma-ok for presence, a nil-map write panics, **never depend on iteration order** (sort keys), `map[T]struct{}` as a set. Strings are bytes — `range` for runes, slice on rune boundaries. | sub-slice appended-to expecting independence; `append` in a hot loop on a zero-cap slice; `if m[k] != 0` for presence; ranging a map and asserting order; `s[i]`/`len(s)` treated as characters. | Learning Go ch.3 |
| D13 | Declarations & shadowing | `:=` for inferred init inside funcs; `var x T` when relying on the zero value or at package scope. Watch shadowing — a `:=` in an inner block that re-declares `err` (or a named return) drops the outer assignment. `iota` is for internal, name-referenced enums; don't serialize its numeric values. No mutable package-level globals. | `x := 0` / `s := ""` just for a zero value; `if v, err := f(); …` shadowing an outer `err`; iota constants written to a DB/wire format; exported mutable globals. | Learning Go ch.2/4; `go vet` shadow |
| D14 | Testing | Table-driven tests with `t.Run` subtests; `t.Cleanup` for teardown; `go test -race` for concurrent code; `httptest` + `testdata` golden files + `go-cmp` for readable diffs; benchmarks (`Benchmark*`) and fuzz targets (`FuzzXxx`) where they earn it. Test the public API; don't design around mocks. | copy-pasted near-identical test funcs; concurrent code with no `-race`; `reflect.DeepEqual` with opaque failures; a mock for every collaborator. | Learning Go ch.15; GCR |

## 2. authorities[]

- **Effective Go** — go.dev/doc/effective_go — the canonical idiom reference.
- **GCR / Go Code Review Comments** — go.dev/wiki/CodeReviewComments — the review-time checklist.
- **Google Go Style Guide** — google.github.io/styleguide/go — decisions + best practices.
- **Go Proverbs** — go-proverbs.github.io — Rob Pike's compression of Go philosophy.
- **Learning Go (Bodner, 2nd ed.)** — O'Reilly, 9781098139285 — book-length idiomatic real-world Go; the team's primary idiom reference (declarations, slices/maps, pointers, interfaces, errors, generics, concurrency, testing). Cite by chapter (e.g. "Learning Go ch.3").
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
- **Slice aliasing** — cue: a sub-slice handed off and appended to, expecting independence (or `append` mutating a shared backing array). Rewrite: `copy` to an independent slice, or cap with a three-index slice `s[lo:hi:max]`.
- **Map-order dependence** — cue: ranging a map and relying on output order. Rewrite: collect keys, `sort` them, range the sorted keys.
- **Shadowed `err`** — cue: a `:=` in an inner block re-declaring `err`, so the outer error is never set. Rewrite: declare the vars once and assign with `=`, or check/return inside the block.
- **Goroutine leak** — cue: `go f()` with no termination path; a goroutine blocked forever on a channel nobody closes. Rewrite: bound it to a `context`/done channel with an observable exit.
- **`defer` in a loop** — cue: `defer x.Close()` inside a `for` (fires only at function return). Rewrite: extract the body into a function so the defer fires per iteration, or close explicitly.
- **Typed-nil through interface** — cue: returning a concrete nil pointer as an `error`/interface, so a `!= nil` check is unexpectedly true. Rewrite: return a literal `nil` interface, or check the concrete value before returning.

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

- **correctness** — CR3, CR4 (stale-write/clobber); D9 (cache mutation, shared-slice aliasing); D13 shadowed `err` (a silently dropped error); typed-nil-through-interface; any nil-map write.
- **idiom-divergence-with-consequence** — CR1, CR2, CR5, CR6, CR7, CR8; D2 when an unwrapped error loses the chain; D5 goroutine leaks / busy-wait / buffer misuse that can deadlock; D12 map-order dependence (flaky output).
- **style** — D1, D8, D10, D11, D14, the slice-presizing / nil-slice / `:=`-vs-`var` nits, and the §4 anti-patterns without a runtime consequence. Bundle these; never lead with them.

Profile note: this pack's CR-overlay and D-dimensions are **subordinate to the repo profile** (`package-profile.md`). The repo's documented exceptions (e.g. the `SeiNodeTask` `Ready`+`Failed` pair) override CR7. Always reconcile against the profile first.
