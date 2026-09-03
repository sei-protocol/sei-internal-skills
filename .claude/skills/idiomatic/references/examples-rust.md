# Rust idiom — worked examples

Loaded **on demand** by the method (step 3) when a worked before/after teaches faster than the rule alone — most useful for the §3 *divergences* (counterintuitive) and the *judgment-only* dimensions (no lint to lean on). Each example pairs with a dimension in `language-pack-rust.md`; cite the same authority and §7 anchor. **Consult a pair for the pattern and cite it — do not paste the block wholesale into a review.**

These pairs are **original** (authored for this pack, not reproduced from any book — copyright-clean). For lint-anchored items, **"Anchor (observed)"** means the *bad* snippet was run through `cargo clippy` and produced the quoted diagnostic **and the *good* snippet passed clean** — so the cited check is real, correctly named, and fires where claimed. The line also records, in bold, whether the lint is **on by default** or **off by default**: **read that bold token, not the group name, to decide whether to cite the lint as build-failing** — the group (`style`/`complexity`/`suspicious`/`restriction`/`pedantic`/`nursery`) explains *why* a lint is on or off but is not the on/off source of truth. So: an *(observed)* anchor's quoted diagnostic is verbatim-real — cite it as-is when it's **on by default**; when it's **off by default**, note the crate must opt in (`#![warn(...)]`). For judgment-only items there is no lint — cite the prose Basis and say no checkable rule exists; never fabricate one.

How to read severity: a footgun marked **correctness** is a bug, not a style nit — lead with it. A **judgment-only** item has no machine-checkable anchor. Everything else is **style** — bundle it, never lead with it (pack §6). The Rust-specific trap: a lint being *named* doesn't mean it *fires* — `restriction`/`pedantic`/`nursery` lints and rustc `missing_docs` are off by default.

---

## Divergences — where Rust rejects other-language habits (judgment-only; examples matter most here)

### R4 · Make illegal states unrepresentable — newtype over stringly-typed
Two `String` parameters of the same type are trivially transposable at the call site; a newtype makes the swap a compile error.

```rust
// bad — caller can swap the two and it still compiles
fn transfer(from: String, to: String, amount: u64) { /* ... */ }
```
```rust
// good — distinct types; transposing from/to won't compile
struct AccountId(String);
fn transfer(from: AccountId, to: AccountId, amount: u64) { /* ... */ }
```
Basis: Programming Rust ch. Enums and Patterns; API Guidelines C-NEWTYPE, C-CUSTOM-TYPE. Anchor: **none — judgment-only** (no lint flags a transposable pair of same-typed args).

### R3 · Errors are values — `Result` + `?`, not a match-ladder
Don't hand-write the propagation a `?` expresses.

```rust
// bad — manual match-ladder
let cfg = match parse(raw) { Ok(c) => c, Err(e) => return Err(e) };
```
```rust
// good — propagate with ?
let cfg = parse(raw)?;
```
Basis: Programming Rust ch. Error Handling; Rust Book ch.9; API Guidelines C-QUESTION-MARK. Anchor: **none — judgment-only** (idiom, not a lint).

### R1 · Don't `.clone()` to silence the borrow checker
A clone added to dodge a borrow error is a smell, not a fix — restructure the scope or borrow.

```rust
// bad — clone purely to end a borrow early
let name = user.name.clone();
process(&user);
println!("{name}");
```
```rust
// good — reorder so the borrow ends naturally; no clone
process(&user);
println!("{}", user.name);
```
Basis: Programming Rust ch. Ownership/References. Anchor: **none — judgment-only** (`redundant_clone` is **nursery — off by default and FP-prone**; don't claim it fires, and never *suggest* a clone to dodge a lifetime).

---

## Correctness / consequence footguns

### R8 · Mutex guard held across `.await` (correctness) — `await_holding_lock`
A `std::sync::MutexGuard` held across an `.await` makes the future `!Send` (won't spawn on a multi-thread runtime) and serializes every task on the lock across the await.

```rust
// bad — guard live across the await point
let g = state.lock().unwrap();
fetch_remote().await;
g.update();
```
```rust
// good — drop the guard before awaiting (or use tokio::sync::Mutex if it must be held)
let snapshot = { let g = state.lock().unwrap(); g.snapshot() };
let data = fetch_remote().await;
state.lock().unwrap().apply(data, snapshot);
```
Basis: pack AX1; Tokio shared-state docs. Anchor (observed): `cargo clippy` (default) → `this MutexGuard is held across an await point` (`clippy::await_holding_lock`, **on by default** — suspicious group). The most important async finding, and it *does* fail a default build.

### R3 · `.unwrap()` in library code (correctness) — `unwrap_used` (off by default)
A library has no business aborting the caller's process on a recoverable error.

```rust
// bad — panics the caller on a missing/garbled file
pub fn load(path: &str) -> Config {
    let raw = std::fs::read_to_string(path).unwrap();
    toml::from_str(&raw).unwrap()
}
```
```rust
// good — return Result, propagate with ?
pub fn load(path: &str) -> Result<Config, ConfigError> {
    let raw = std::fs::read_to_string(path)?;
    Ok(toml::from_str(&raw)?)
}
```
Basis: Programming Rust ch. Error Handling; API Guidelines C-GOOD-ERR. Anchor (observed): `cargo clippy -- -W clippy::unwrap_used` → `used \`unwrap()\` on a \`Result\` value` (`clippy::unwrap_used`). **Off by default — `restriction` group;** it will *not* fire on a default `cargo clippy`, so cite it as R3-by-judgment unless the crate has `#![warn(clippy::unwrap_used)]`.

### R4 · Truncating `as` cast — `cast_possible_truncation` (off by default)
`as` silently truncates; a fallible `TryFrom` surfaces the loss.

```rust
// bad — silently wraps for values > 255
let byte = count as u8;
```
```rust
// good — fallible conversion
let byte = u8::try_from(count)?;
```
Basis: API Guidelines C-CONV-TRAITS. Anchor (observed): `cargo clippy -- -W clippy::cast_possible_truncation` → `casting \`i64\` to \`u8\` may truncate the value`. **Off by default — `pedantic` group;** cite the conversion idiom on prose unless the crate enabled pedantic.

---

## References & signatures

### R2 · `&String` parameter — `ptr_arg`
`&str` accepts string literals and slices too; `&String` forces an allocation-backed value.

```rust
// bad
fn greet(name: &String) -> String { format!("hi {name}") }
```
```rust
// good
fn greet(name: &str) -> String { format!("hi {name}") }
```
Basis: API Guidelines C-CONV-TRAITS. Anchor (observed): `cargo clippy` (default) → `writing \`&String\` instead of \`&str\` involves a new object where a slice will do` (`clippy::ptr_arg`, **on by default** — style).

### R2 · `&Box<T>` parameter — `borrowed_box`
The extra indirection buys nothing; take `&T`.

```rust
// bad
fn sum(b: &Box<i32>) -> i32 { **b }
```
```rust
// good
fn sum(n: &i32) -> i32 { *n }
```
Basis: pack R2. Anchor (observed): `cargo clippy` (default) → `you seem to be trying to use \`&Box<T>\`. Consider using just \`&T\`` (`clippy::borrowed_box`, **on by default** — complexity).

### R1 · Needless explicit lifetimes — `needless_lifetimes`
Let elision carry the common cases.

```rust
// bad
fn first<'a>(s: &'a str) -> &'a str { s }
```
```rust
// good
fn first(s: &str) -> &str { s }
```
Basis: Programming Rust ch. References. Anchor (observed): `cargo clippy` (default) → `the following explicit lifetimes could be elided: 'a` (`clippy::needless_lifetimes`, **on by default** — complexity).

---

## Shape & construction

### R6 · `match`-returning-bool — `match_like_matches_macro`
Use `matches!` for a boolean pattern test.

```rust
// bad
let is_zero = match x { 0 => true, _ => false };
```
```rust
// good
let is_zero = matches!(x, 0);
```
Basis: Programming Rust ch. Iterators. Anchor (observed): `cargo clippy` (default) → `match expression looks like \`matches!\` macro` (`clippy::match_like_matches_macro`, **on by default** — style).

### R6 · `.iter().count()` — `iter_count`
`.len()` is O(1) and says what you mean.

```rust
// bad
let n = v.iter().count();
```
```rust
// good
let n = v.len();
```
Basis: pack R6. Anchor (observed): `cargo clippy` (default) → `called \`.iter().count()\` on a \`Vec\`` (`clippy::iter_count`, **on by default** — complexity).

### R7 · `new()` without `Default` — `new_without_default`
A zero-arg `new()` should be paired with a `Default` impl.

```rust
// bad
pub struct Pool { size: usize }
impl Pool { pub fn new() -> Self { Pool { size: 0 } } }
```
```rust
// good — derive Default; keep new() if you like, but it's now consistent
#[derive(Default)]
pub struct Pool { size: usize }
impl Pool { pub fn new() -> Self { Self::default() } }
```
Basis: API Guidelines C-CTOR. Anchor (observed): `cargo clippy` (default) → `you should consider adding a \`Default\` implementation for \`Pool\`` (`clippy::new_without_default`, **on by default** — style).

---

## Unsafe & docs

### R9 · Unsafe documentation — `missing_safety_doc` (on by default) + `undocumented_unsafe_blocks` (off by default)
Two distinct, complementary lints — and one of them is off by default.

```rust
// bad — pub unsafe fn with no # Safety section; unsafe block with no // SAFETY:
pub unsafe fn from_raw(p: *const u8) -> u8 { *p }
fn read() -> u8 { unsafe { from_raw(ptr()) } }
```
```rust
// good
/// # Safety
/// `p` must be non-null and point to an initialized byte.
pub unsafe fn from_raw(p: *const u8) -> u8 {
    // SAFETY: the caller's contract guarantees p is valid (edition 2024 needs the inner block too).
    unsafe { *p }
}
fn read() -> u8 {
    // SAFETY: ptr() returns a non-null pointer to an initialized byte.
    unsafe { from_raw(ptr()) }
}
```
Basis: Programming Rust ch. Unsafe Code; API Guidelines C-FAILURE. Anchors (observed): `cargo clippy` (default) → `unsafe function's docs are missing a \`# Safety\` section` (`clippy::missing_safety_doc`, **on by default** — style). And `cargo clippy -- -W clippy::undocumented_unsafe_blocks` → flags the `unsafe {}` block (`clippy::undocumented_unsafe_blocks`, **off by default** — restriction; the crate must opt in).
