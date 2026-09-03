# Go idiom — worked examples

Loaded **on demand** by the method (step 3) when a worked before/after teaches faster than the rule alone — most useful for the §3 *divergences* (counterintuitive), the correctness footguns, and the *judgment-only* dimensions (no lint to lean on). Each example pairs with a dimension in `language-pack-go.md`; cite the same authority and §7 anchor. **Consult a pair for the pattern and cite it — do not paste the block wholesale into a review.**

These pairs are **original** (authored for this pack, not reproduced from any book — copyright-clean). For lint-anchored items, **"Anchor (observed)"** means the *bad* snippet was run through the named tool and produced the quoted diagnostic, and the *good* snippet passed clean — so the cited check is real, correctly named, and fires where claimed. So: an *(observed)* anchor's quoted diagnostic is verbatim-real — cite it as-is. For judgment-only items there is no tool — the example exists precisely because no linter catches it; cite the prose Basis and say no checkable rule exists.

How to read severity: a footgun marked **correctness** is a bug, not a style nit — lead with it. A **judgment-only** item has no machine-checkable anchor; flag it on the prose authority and say so (never fabricate an ID — §7). Everything else is **style** — bundle it, never lead with it (pack §6).

---

## Divergences — where Go rejects general software wisdom (judgment-only; examples matter most here)

### D3 · Accept interfaces, return structs; define the interface at the consumer
Returning an interface (and defining it beside its only implementation) hides the concrete type and forecloses caller optionality. Take the interface as a parameter where a second implementation actually exists; return the concrete type.

```go
// bad — producer-side interface, returned from the constructor
package store

type Store interface{ Get(id string) (Item, error) }
type pg struct{ /* ... */ }
func New() Store { return &pg{} }   // caller can't reach *pg's other methods
```
```go
// good — return the concrete type; let a consumer define the narrow interface it needs
package store

type PG struct{ /* ... */ }
func New() *PG { return &PG{} }

// in the consuming package, sized to what it uses:
// type getter interface{ Get(id string) (Item, error) }
```
Basis: Go Proverbs; GCR: Interfaces; GGSG: Decisions — Interfaces. Anchor: **none — judgment-only** (§7 marks D3 so; `ireturn` is a weak proxy that doesn't verify consumer-side placement). Do not offer a linter "to enforce it."

### Cross-cutting · Observability field on a domain type (§3 divergence + §4 anti-pattern)
A field whose *only* reader is a metric label / log key couples the domain type to the metric taxonomy — the type churns whenever the taxonomy does. Map the sentinel → its string at the metric boundary instead. This also fixes a latent correctness bug: a `map[error]string` is keyed by `==` identity and **misses a `%w`-wrapped error**, silently emitting an empty label.

```go
// bad — the error carries its own metric label, and lookup is by map identity
type ValidationError struct{ Code string } // Code is the metric label
var severityByErr = map[error]string{ErrAppHash: "page"}
func record(err error) { counterInc(severityByErr[err]) } // misses fmt.Errorf("%w", ErrAppHash)
```
```go
// good — classify at the boundary with errors.Is (sees through %w); domain type carries no label
func severity(err error) string {
    switch {
    case errors.Is(err, ErrAppHash):
        return "page"
    default:
        return "ticket"
    }
}
```
Basis: §3 divergence (presentation belongs to its own layer) + §4 anti-pattern. Anchor: **none** — two findings, neither machine-checkable: the layering call is *judgment-only*; the `map[error]` identity miss (it can't see through `%w`, so a wrapped error gets an empty label) is a **correctness** defect — lead with it.

### §3 · A little copying beats a premature helper / a new dependency
DRY/SRP pushes extraction at the second repeat; Go prefers a little duplication over the wrong abstraction (and over a dependency pulled in to save a few lines).

```go
// bad — a one-call indirection and a dependency to dedupe two trivial lines
import "github.com/acme/strutil"
func key(s string) string { return strutil.TrimLower(s) }
```
```go
// good — inline the two lines where they're used; no indirection, no dep
key := strings.ToLower(strings.TrimSpace(s))
```
Basis: §3 divergences (a-little-copying; premature-helper). Anchor: **none — judgment-only**. Don't recommend a helper for 2–3 line repeats unless they're a must-change-together correctness coupling.

---

## Correctness footguns

### D13 · Shadowed `err` (correctness) — `shadow`
A `:=` in an inner block re-declares `err`; if the outer `err` is read afterward, the inner assignment is lost.

```go
// bad — inner err shadows the outer; the final `return ..., err` reads the stale outer value
data, err := load()
if err != nil { return 0, err }
if more, err := fetch(); err == nil { data += more } // shadows err
return data, err
```
```go
// good — reuse the outer err with =, or check inside the block
data, err := load()
if err != nil { return 0, err }
more, err := fetch()
if err != nil { return 0, err }
return data + more, nil
```
Basis: Learning Go ch.4; GGSG: Best Practices — Shadowing. Anchor (observed): `go vet -vettool=$(which shadow)` → `declaration of "err" shadows declaration at line 7`. **Caveat:** `shadow` is **off by default** in `go vet` — it fails a build only if the repo enabled it.

### D12 · Slice aliasing via `make([]T, n)` then `append` (correctness) — `makezero`
`make([]T, n)` creates `n` zero elements; `append` then adds *after* them. The classic intent was capacity, not length.

```go
// bad — n leading zeros, then the real data appended after them
s := make([]int, n)
s = append(s, vals...) // [0,0,…,0, vals…]
```
```go
// good — length 0, capacity n
s := make([]int, 0, n)
s = append(s, vals...)
```
Basis: Learning Go ch.3. Anchor (observed): `golangci-lint` (`makezero`) → `append to slice "s" with non-zero initialized length`.

### D12 · Write to a nil map (correctness) — `SA5000`
A nil map reads fine but panics on write.

```go
// bad
var m map[string]int
m["x"] = 1 // panic: assignment to entry in nil map
```
```go
// good
m := make(map[string]int)
m["x"] = 1
```
Basis: Learning Go ch.3. Anchor (observed): `staticcheck` → `SA5000: assignment to nil map`.

### D9 · Copying a value that holds a lock (correctness) — `copylocks`
A struct embedding a `sync.Mutex` must travel by pointer; a value copy duplicates the lock and breaks mutual exclusion.

```go
// bad — g is copied; its mutex no longer guards the original
type guarded struct { mu sync.Mutex; n int }
func use(g guarded) int { return g.n }
```
```go
// good — pass a pointer
func use(g *guarded) int { return g.n }
```
Basis: GGSG: Decisions — Copying / Receiver type. Anchor (observed): `go vet` (default) → `use passes lock by value: <pkg>.guarded contains sync.Mutex` (golangci-lint's govet prints the same with a `copylocks:` prefix). **Name is `copylocks` (plural)** — `go vet -copylock` is rejected.

---

## Error handling

### D2 · Wrap with `%w`, not `%v`; `%w` goes last — `errorlint`
`%v` flattens the cause to a string, so callers can't `errors.Is`/`errors.As` it. Use `%w`, placed last — unless you're wrapping a sentinel to categorize, where the sentinel leads.

```go
// bad — severs the chain
return fmt.Errorf("normalize record: %v", err)
```
```go
// good — preserves it
return fmt.Errorf("normalize record: %w", err)
// sentinel-categorizing form: fmt.Errorf("%w: invalid header: %v", ErrParse, err)
```
Basis: Go blog 1.13 errors; GGSG: Best Practices — Placement of %w. Anchor (observed): `golangci-lint` (`errorlint`) → `non-wrapping format verb for fmt.Errorf. Use %w to format errors`.

### D2 · Error-string form, and no `"failed to"` filler — `ST1005` (form only)
Error strings are lowercase, no trailing punctuation. A `"failed to"`/`"error:"` prefix is content-free — a non-nil error already signals failure.

```go
// bad
errors.New("Failed to parse config.")
```
```go
// good
errors.New("parse config")
```
Basis: GGSG: Decisions — Error strings; Best Practices — Adding information to errors. Anchor (observed): `staticcheck` → `ST1005: error strings should not be capitalized` **and** `… should not end with punctuation`. **The `"failed to"` filler is judgment-only** — `ST1005` catches capitalization/punctuation, not the redundant prefix; cite the GGSG heading and say no checkable rule covers it.

### D2 · In-band error sentinel — judgment-only
Don't signal failure with a magic value; return a second result.

```go
// bad — -1 means "not found"
func indexOf(s []string, x string) int { /* … */ return -1 }
```
```go
// good
func indexOf(s []string, x string) (int, bool) { /* … */ return 0, false }
```
Basis: GGSG: Decisions — In-band errors. Anchor: **none — judgment-only**.

---

## Shape & structure

### D8 · `else` after a returning `if` — `revive: indent-error-flow`
Keep the happy path at minimal indentation (line of sight).

```go
// bad
if x < 0 { return -x } else { return x }
```
```go
// good
if x < 0 { return -x }
return x
```
Basis: GCR: Indent Error Flow; GGSG: Decisions — Indent error flow. Anchor (observed): `golangci-lint` (`revive`) → `indent-error-flow: if block ends with a return statement, so drop this else and outdent its block`. **Caveat:** revive is not default-on and its rule names are version-dependent.

### D8 · Named results only to enable a naked return — `nakedret`
Name results only to disambiguate same-typed returns or for a deferred-closure assignment — not just so the final `return` can be naked; and naked returns only in short functions.

```go
// bad — names exist only for the naked return; reader must scroll to learn what `return` yields
func parse(x int) (result int, err error) {
    if x < 0 { err = ErrParse; return }
    result = x * 2
    return
}
```
```go
// good — explicit returns
func parse(x int) (int, error) {
    if x < 0 { return 0, ErrParse }
    return x * 2, nil
}
```
Basis: GGSG: Decisions — Named result parameters. Anchor (observed): `golangci-lint` (`nakedret`) → `naked return in func "parse" with N lines of code` (past the configured threshold).

### D5 · `context.Context` stored in a struct — `containedctx`
`ctx` is a per-call parameter, not state. Pass it as the first arg of each method.

```go
// bad
type worker struct { ctx context.Context; id int }
```
```go
// good
type worker struct { id int }
func (w *worker) run(ctx context.Context) error { /* … */ return nil }
```
Basis: GGSG: Decisions — Contexts. Anchor (observed): `golangci-lint` (`containedctx`) → `found a struct that contains a context.Context field`.

### D5 · Discarded `cancel` from `context.WithCancel` — `lostcancel`
Not calling `cancel` leaks the context (and its timer/goroutine).

```go
// bad — cancel discarded; ctx used locally
ctx, _ := context.WithCancel(parent)
work(ctx)
```
```go
// good — same shape, cancel deferred
ctx, cancel := context.WithCancel(parent)
defer cancel()
work(ctx)
```
Basis: GCR: Contexts. Anchor (observed): `go vet` (default) → `the cancel function returned by context.WithCancel should be called, not discarded, to avoid a context leak`. (If the function instead *returns* a cancelable context, don't `defer cancel()` inside — return `(ctx, cancel)` and let the caller defer it, or you hand back an already-canceled context.)

### Anti-pattern · Ignored error return — `errcheck`
Don't drop an error on the floor.

```go
// bad
doThing() // returns error
```
```go
// good
if err := doThing(); err != nil { return err }
```
Basis: GCR: Handle errors. Anchor (observed): `golangci-lint` (`errcheck`) → `Error return value is not checked`.

### D15 · Import grouping — `gci`
Imports form blank-line-separated groups (stdlib, then project/third-party), each sorted.

```go
// bad
import (
    "fmt"
    "github.com/some/dep"
    "context"
)
```
```go
// good
import (
    "context"
    "fmt"

    "github.com/some/dep"
)
```
Basis: GGSG: Decisions — Import grouping; Best Practices — Import ordering. Anchor (observed): `gci` is a **formatter** in golangci-lint v2 — checked with `golangci-lint fmt --diff` (it regroups stdlib above third-party with a blank line and sorts within each group). `gofmt` alone does **not** enforce the inter-group blank line.

---
