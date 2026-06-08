# Rust idiom pack

Written against `_TEMPLATE.md`. Cite §2 authorities in findings. Like the Go pack, the value here is less "teach Rust" than to give findings a **citation** and encode the **divergences** (§3) where general/other-language habits are *wrong* for Rust. Findings cite the anchor book by chapter plus a publicly-linkable corroborator (a Clippy lint, a Rust API Guideline `C-*` code, or the Rust Book).

Sections: §1 dimensions · §2 authorities · §3 divergences · §4 anti-patterns · §5 framework overlays · §6 severity model. Grounded in *Programming Rust* (Blandy, Orendorff, Tindall, 2nd ed.), the Rust Book, the Rust API Guidelines, and Clippy.

## 1. dimensions[]

| id | dimension | idiomatic rule | cue | authority |
|----|-----------|----------------|-----|-----------|
| R1 | Ownership / borrowing / lifetimes | Borrow when the callee only reads; move/own only when it stores or consumes. Never `.clone()` to silence the borrow checker — restructure scopes/lifetimes. Rely on lifetime elision. Reach for `Cow<'_, T>` when a value is usually borrowed but occasionally owned. | `.clone()` right before a borrow error site; explicit `<'a>` the compiler would elide; `String`/`Vec<T>` param that's only read. | Programming Rust ch. Ownership/References; Clippy `redundant_clone`, `needless_lifetimes` |
| R2 | References & signatures | Accept `&str` over `&String`, `&[T]` over `&Vec<T>`, `impl AsRef<Path>` for path params; own in, borrow out. Pick the receiver deliberately: `&self` reads, `&mut self` mutates, `self` consumes (builders). | `fn f(s: &String)` (rejects literals); `&mut self` on a non-mutating method. | Clippy `ptr_arg`, `borrowed_box`; API Guidelines C-CONV-TRAITS |
| R3 | Error handling | `Result<T,E>` for recoverable failure, `Option<T>` for absence; propagate with `?`, not match-ladders. **No `unwrap`/`expect` in library code** — return `Result` and let the caller decide; when a panic is truly intended use `expect("why")` over bare `unwrap()`. Libraries define an error enum (e.g. `thiserror`) implementing `std::error::Error`; apps may use `anyhow` + `.context()` at the top. | `.unwrap()` under `src/lib.rs`; `panic!` on user/IO/network error; `match res { Ok=>.., Err(e)=>return Err(e) }` ladders; a reusable crate returning `anyhow::Error`. | Programming Rust ch. Error Handling; Rust Book ch.9; Clippy `unwrap_used`/`expect_used`; API Guidelines C-GOOD-ERR, C-FAILURE |
| R4 | Types make illegal states unrepresentable | Newtype over stringly/primitively-typed values; enums with per-variant data over structs-of-`Option`s with implicit rules; `Option<T>` over sentinels (`-1`, `""`); `From`/`Into` (infallible) and `TryFrom`/`TryInto` (fallible) over ad-hoc converters and truncating `as`. | `fn(user_id: String, order_id: String)`; `struct{is_a:bool,is_b:bool,...}`; `return -1` for not-found; `x as u8` that can truncate. | Programming Rust ch. Enums and Patterns; API Guidelines C-NEWTYPE-HIDE, C-CONV-TRAITS; Clippy `cast_lossless` |
| R5 | Traits & generics | Small focused traits; `impl Trait` in arg/return position; generics (static dispatch) by default, `dyn Trait` for heterogeneous collections or to cut monomorphization; derive standard traits eagerly (`Debug` on all public types). Don't over-genericize or put bounds on the struct def. | one trait with many unrelated methods; `Box<dyn Iterator>` where `impl Iterator` fits; a public struct without `#[derive(Debug)]`; `<T: AsRef<str>>` only ever called with `&str`; `struct S<T: Clone>`. | Programming Rust ch. Traits and Generics; API Guidelines C-COMMON-TRAITS, C-DEBUG, C-STRUCT-BOUNDS |
| R6 | Iterators & closures | Iterator adapters (`map`/`filter`/`fold`/`collect`) over `for i in 0..len` index loops; keep the chain lazy — don't `collect()` just to re-iterate; `matches!(x, Pat)` over `match`-returning-bool; closures capture by reference, `move` only to transfer ownership (into a thread/task). | `for i in 0..v.len() { v[i] }`; `let t: Vec<_> = it.collect(); for x in t {}`; `match x { A=>true, _=>false }`; a `move` closure that needlessly clones. | Programming Rust ch. Iterators/Closures; Clippy `needless_collect`, `match_like_matches_macro` |
| R7 | RAII & construction patterns | `Drop` for deterministic cleanup — no manual `close()`/`free()` the caller must remember; return guard types for scoped acquire/release; builder pattern for many-optional-field structs; implement (or derive) `Default` over a zero-init `new()`. | a `Resource` with a `cleanup()` + "remember to call this"; paired `lock()`/`unlock()`; `Thing::new(a,b,None,None,true,None)`; bespoke `new()` that just zero-inits with no `Default`. | Programming Rust ch. References/Utility Traits; Rust Patterns: Builder, Finalisation-in-Drop |
| R8 | Concurrency & async | Let the compiler derive `Send`/`Sync` (no hand-rolled `unsafe impl`); prefer channels (ownership transfer) over `Arc<Mutex<…>>` for pipelines/single-owner; **never hold a `std::sync::Mutex` guard across `.await`**; never block/CPU-spin on the async runtime (use `spawn_blocking`); don't orphan a `tokio::spawn` (await/join or give it a cancellation path); handle `PoisonError` deliberately. | `unsafe impl Send` w/o justification; `Arc<Mutex<Vec<_>>>` used as a work queue; `let g = m.lock().unwrap(); x().await;`; `std::fs`/`thread::sleep`/heavy compute in `async fn`; `tokio::spawn(..)` with the handle dropped; blanket `.lock().unwrap()`. | Programming Rust ch. Concurrency/Async; Rust Book ch.16; Tokio shared-state docs |
| R9 | Unsafe | Minimize `unsafe`; wrap each block in a safe abstraction that upholds the invariants so callers never touch it; every `unsafe` block carries a `// SAFETY:` comment; every `unsafe fn` documents caller obligations in a `# Safety` doc section. | `unsafe` leaking into the public API; an `unsafe { }` with no `// SAFETY:`; an `unsafe fn` with no `# Safety` docs. | Programming Rust ch. Unsafe Code; Clippy `undocumented_unsafe_blocks`; API Guidelines C-FAILURE |
| R10 | Modules & API surface | `pub` only the API; default private (private fields + accessors). Don't leak internal/dependency types through public signatures. Curate `pub use` re-exports into a flat, intentional surface. Names follow RFC 430 (`CamelCase` types, `snake_case` fns, `SCREAMING_SNAKE` consts; iterators `iter`/`iter_mut`/`into_iter`; no `get_` getters). | `pub` on internal helpers/fields; a public fn returning a third-party crate's type; `crate::internal::detail::Thing` call paths; `getFoo()`/`get_foo()` getters. | Programming Rust ch. Crates and Modules; API Guidelines C-CASE, C-GETTER, C-STRUCT-PRIVATE, C-NEWTYPE-HIDE |
| R11 | Comment discipline | Comments are an **uncommon exception** — names and types carry intent. The sanctioned "above the code" docs are `//!` (module/crate) and `///` (item) doc comments, with `# Errors`/`# Panics`/`# Safety` sections on public APIs — these describe the *contract*, not how the code got here. `// SAFETY:` on each `unsafe` block is a *mandated* exception (R9). **No historical/changelog reasoning in code** ("we used to…", "removed because…") — that belongs in the PR/commit. | `// what` restating the next line; "we used to"/"previously"/commented-out code; a comment a rename would delete; a public item with no `///` doc; an `unsafe` block with no `// SAFETY:`. | Programming Rust (Documentation); Rust Book ch.14; API Guidelines C-FAILURE; Clippy `undocumented_unsafe_blocks` |
| R12 | Testing | `#[test]` + `cargo test`; black-box tests in `tests/` exercise the public API (doubles as surface validation); doctests on public items (runnable, stay correct); `#[should_panic(expected="…")]` for intended panics; reach for property testing (`proptest`/`quickcheck`) for large input spaces (parsers, round-trips). | integration behavior tested only by reaching into private internals; doc examples fenced as ` ```text ` to dodge compilation; a panic path with no test; a parser with only a few hand-picked cases. | Programming Rust (tests); Rust Book ch.11/14 |

## 2. authorities[]

- **Programming Rust (Blandy, Orendorff, Tindall, 2nd ed.)** — O'Reilly, 9781492052586 — the anchor real-world Rust reference. Cite by chapter (e.g. "Programming Rust ch. Error Handling").
- **The Rust Book (TRPL)** — doc.rust-lang.org/book — canonical language reference; cite by chapter.
- **Rust API Guidelines** — rust-lang.github.io/api-guidelines — `C-*` checklist codes for public-API design.
- **Clippy** — rust-lang.github.io/rust-clippy — the lint corpus; cite the lint name. Note: `unwrap_used`/`expect_used` are in the *restriction* group (off by default) — enable explicitly for library crates; apply R3's lib rule by judgment regardless.
- **Tokio docs** — tokio.rs / docs.rs/tokio — async runtime idioms (shared state, `spawn_blocking`, `tokio::sync::Mutex`).
- **Rust Design Patterns** — rust-unofficial.github.io/patterns — idioms (newtype, builder, RAII guards, borrowed-args).

## 3. divergences[] — where Rust rejects general / other-language wisdom

Do **not** flag these.

- **No inheritance — compose with traits.** OOP instinct to build a type hierarchy is wrong; Rust uses traits + generics/`dyn` and struct composition. → Don't recommend a base-class-like hierarchy.
- **No null — `Option<T>`.** Don't reintroduce sentinels (`-1`, empty string) the way a C/Java habit might.
- **No exceptions — errors are values (`Result` + `?`).** Don't reach for panic/catch-style control flow.
- **`.clone()` is not the GC escape hatch.** Coming from a GC language, the reflex is to copy freely; in Rust a clone-to-satisfy-the-borrow-checker is a smell, not a fix. → Flag needless clones; don't *suggest* a clone to dodge a lifetime.
- **Private fields + accessors, not public data by default.** Unlike Go (which exposes fields and drops `Get`), Rust idiom (API Guidelines C-STRUCT-PRIVATE) favors private fields with methods — but the *getter is named `foo()` not `get_foo()`* (C-GETTER). → Don't recommend Go-style public fields, and don't recommend a `get_` prefix.
- **Prefer `From`/`TryFrom` over `as`.** `as` casts can silently truncate; don't bless them for numeric/again conversions where a `TryFrom` is right.
- **`unwrap`/`expect` aren't lint-blocked by default.** Their absence from default Clippy doesn't make them fine in libraries — apply R3 by judgment.

## 4. anti_patterns[]

- **`unwrap`/`expect` in library code** — cue: `.unwrap()` under `src/lib.rs` on a recoverable result. Rewrite: return `Result`/`Option` and `?`; if a panic is truly intended, `expect("documented invariant")`.
- **Needless clone** — cue: `.clone()` to dodge a borrow error, or `.clone()` on a `Copy` type. Rewrite: borrow / restructure scope; `.copied()` for `Copy` iterators.
- **`&String` / `&Vec<T>` parameter** — cue: `fn f(s: &String)`. Rewrite: `&str` / `&[T]` (accepts more callers, one fewer indirection).
- **Stringly-typed value** — cue: distinct concepts passed as bare `String`/`u64` (easy to transpose). Rewrite: newtype (`struct UserId(Uuid)`).
- **Catch-all over an owned enum** — cue: `_ => …` on an enum you control. Rewrite: match every variant so a new one is a compile error; `#[non_exhaustive]` on public enums.
- **Collect-then-iterate** — cue: `let v: Vec<_> = it.collect(); for x in v {}`. Rewrite: keep the iterator lazy.
- **Lock guard held across `.await`** — cue: a `std::sync::MutexGuard` live across an `await`. Rewrite: drop the guard before awaiting, scope it to a sync fn, or use `tokio::sync::Mutex`.
- **Blocking on the async runtime** — cue: `std::fs`/`thread::sleep`/heavy compute in `async fn`. Rewrite: `tokio::spawn_blocking` or a dedicated thread.
- **`Arc<Mutex<…>>` as a queue/pipeline** — cue: shared-mutex work queue. Rewrite: an `mpsc` channel (share memory by communicating).
- **Undocumented `unsafe`** — cue: `unsafe { }` with no `// SAFETY:`. Rewrite: add the SAFETY justification, or remove the unsafe.
- **Changelog comment / what-comment / commented-out code** — see R11 (same stance as the Go pack): history lives in the PR, names carry intent, VCS holds deleted code.

## 5. framework_overlays[]

### async / tokio

The Rust analog of a framework overlay — these add to R8 and carry runtime consequences, so they rank above style.

| id | rule | cue | consequence |
|----|------|-----|-------------|
| AX1 | No `std::sync::Mutex` guard held across `.await`. | guard live across an `await`. | the guard isn't `Send`; the task can move threads → **deadlock** (correctness). |
| AX2 | No blocking / CPU-heavy work on the runtime — use `spawn_blocking`. | `std::fs`/`thread::sleep`/tight compute in `async fn`. | starves the executor; stalls unrelated tasks. |
| AX3 | Don't orphan `tokio::spawn` — await/join the handle or give it a cancellation path. | `tokio::spawn(..)` with the `JoinHandle` dropped, no shutdown signal. | task leak; work silently lost on shutdown. |
| AX4 | Use `tokio::sync::Mutex` only when a lock must be held across `.await`; otherwise prefer `std::sync::Mutex` in a sync scope. | `tokio::sync::Mutex` for a lock never held across await (slower), or `std::sync::Mutex` held across await (AX1). | wrong-mutex either wastes perf or risks deadlock. |
| AX5 | Handle `PoisonError` deliberately. | blanket `.lock().unwrap()` everywhere. | a poisoned lock panics the whole pool instead of recovering/propagating. |

## 6. severity_model

- **correctness** — AX1 (lock-across-await deadlock); AX2 (runtime starvation); R9 unsafe invariant violation; R3 `unwrap`/`panic` on external input in a library (panics the caller).
- **idiom-divergence-with-consequence** — R1 needless clone (perf/allocations); R8/AX3 orphaned task (leak); R4 catch-all over an owned enum (silently swallows new variants); R2 `&String`/`&Vec` (rejects callers); AX4/AX5 mutex misuse; R4 truncating `as`.
- **style** — R5 (derive/`impl Trait`/over-generic), R6 (iterator-over-index, `matches!`), R7 (builder/`Default`), R10 naming, R11 comment nits, R1 lifetime elision. Bundle these; never lead with them.

Profile note: this pack is **subordinate to the repo profile** (`package-profile.md`). A repo's documented convention or exception overrides any dimension here — reconcile against the profile first.

## Language → specialist agent

There is no `rust-specialist` agent in the roster today. For deep Rust judgment calls beyond this pack, review on the pack + first principles and flag the gap (per the skill's missing-pack/dispatch guidance); if Rust work becomes common, a `rust-specialist` agent is the un-defer.
