# TypeScript idiom — worked examples

Loaded **on demand** by the method (step 3) when a worked before/after teaches faster than the rule alone — most useful for the §3 *divergences* and the *judgment-only* dimensions. Each example pairs with a dimension in `language-pack-typescript.md`; cite the same authority and §7 anchor. **Consult a pair for the pattern and cite it — do not paste the block wholesale into a review.**

These pairs are **original** (authored for this pack, not reproduced from any book — copyright-clean). For lint-anchored items, **"Anchor (observed)"** means the *bad* snippet was run through `eslint`/`tsc` and produced the quoted diagnostic, and the *good* snippet passed clean — so the cited check is real, correctly named, and fires where claimed. Each anchor line records the rule's **preset** and whether it **requires type info**: **a type-checked rule fails a build only when typed linting (`parserOptions.project`) is configured** — confirmed by running the tools (under plain `recommended` with no `project`, the T4 promise rules do **not** fire; `no-explicit-any` does). For judgment-only items there is no rule — cite the prose Basis and say no checkable rule exists; never fabricate one.

How to read severity: a footgun marked **correctness** is a bug, not a style nit — lead with it. A **judgment-only** item has no machine-checkable anchor. Everything else is **style** — bundle it, never lead with it (pack §6).

---

## Divergences — where TS rejects other-language habits (judgment-only; examples matter most here)

### §3 · `enum` where a union fits — prefer a string-literal union / `as const`
Unions are erasable and structurally checkable; `enum` carries runtime/`const enum` footguns.

```typescript
// bad
enum Status { Active, Paused }
function label(s: Status): string { /* ... */ }
```
```typescript
// good — string-literal union; narrows in a switch, no runtime enum
type Status = 'active' | 'paused';
function label(s: Status): string { /* ... */ }
```
Basis: TS Handbook (Literal Types); Google TS (Enums). Anchor: **none — judgment-only** (no lint flags "this enum should be a union").

### §3 · Default export → named export
Named exports keep import identifiers uniform and rename-safe.

```typescript
// bad
export default class Client {}
```
```typescript
// good
export class Client {}
```
Basis: Google TS (Exports — do not use default exports). Anchor: **none — judgment-only** (the `import/no-default-export` rule lives in the separate `eslint-plugin-import`; whether default exports are allowed is a repo-profile call).

---

## Correctness footguns

### T4 · Floating / misused promise (correctness) — `no-misused-promises` (the `forEach(async)` shape) / `no-floating-promises` (a bare unhandled promise) — recommended-type-checked, **needs type info**
An unhandled promise statement drops rejections and ordering.

```typescript
// bad — the promise floats; refreshAll resolves before fetch completes
export function refreshAll(urls: string[]): void {
  urls.forEach(async (u) => { cache.set(u, await fetch(u)); });
}
```
```typescript
// good — await all of them; errors propagate
export async function refreshAll(urls: string[]): Promise<void> {
  await Promise.all(urls.map(async (u) => { cache.set(u, await fetch(u)); }));
}
```
Basis: typescript-eslint async docs. Anchor (observed): `eslint` → `Promise returned in function argument where a void return was expected` (`@typescript-eslint/no-misused-promises`) for the `forEach(async)` shape; a *bare* unhandled promise statement is `@typescript-eslint/no-floating-promises` (`Promises must be awaited, end with a call to .catch, … or be explicitly marked as ignored with the void operator`). **Both are recommended-type-checked and require `parserOptions.project`** — under plain `recommended` they do not fire (verified).

### T1 · `any` boundary + unsound `as` (correctness) — `no-explicit-any` (recommended)
`any` stops checking and propagates; an `as` on unvalidated data asserts a shape that was never checked.

```typescript
// bad
export function loadConfig(raw: any): Config {
  return JSON.parse(raw) as Config;
}
```
```typescript
// good — unknown in, validate with a type guard, then it's typed by proof not assertion
function isConfig(x: unknown): x is Config {
  return typeof x === 'object' && x !== null && typeof (x as { port?: unknown }).port === 'number';
}
export function loadConfig(raw: string): Config {
  const parsed: unknown = JSON.parse(raw);
  if (!isConfig(parsed)) throw new Error('bad config');
  return parsed;
}
```
Basis: Do's & Don'ts (any); Google TS. Anchor (observed): `eslint` → `Unexpected any. Specify a different type` (`@typescript-eslint/no-explicit-any`, **recommended, no type info** — fires under plain `eslint:recommended` with no `parserOptions.project`). The unsound `as Config` itself is **judgment-only** — no lint flags an *unsound* assertion (`no-unnecessary-type-assertion` catches only *redundant* ones).

### T3 · Non-exhaustive switch (correctness) — `switch-exhaustiveness-check` (off, **needs type info**) + tsc
A switch over a union that misses a case silently returns `undefined`.

```typescript
// bad — no 'archived' case; returns undefined when s === 'archived'
type Status = 'active' | 'paused' | 'archived';
function label(s: Status): string {
  switch (s) {
    case 'active': return 'on';
    case 'paused': return 'off';
  }
}
```
```typescript
// good — never default proves exhaustiveness; a new variant fails to compile
function label(s: Status): string {
  switch (s) {
    case 'active': return 'on';
    case 'paused': return 'off';
    case 'archived': return 'gone';
    default: { const _exhaustive: never = s; return _exhaustive; }
  }
}
```
Basis: TS Handbook (Exhaustiveness checking, The `never` type). Anchors (observed): `eslint` → `Switch is not exhaustive. Cases not matched: "archived"` (`@typescript-eslint/switch-exhaustiveness-check` — **off by default / no preset, requires type info**); and `tsc --strict` → `error TS2366: Function lacks ending return statement and return type does not include 'undefined'`. The `never`-default pattern fails `tsc` with **zero lint config** — the most portable enforcement.

### T10 · `throw` a non-Error (correctness) — `only-throw-error` (recommended-type-checked, **needs type info**)
Throwing a string breaks `instanceof`/stack-trace handling downstream.

```typescript
// bad
throw 'config missing';
```
```typescript
// good
throw new Error('config missing');
```
Basis: Google TS (throw Error not strings). Anchor (observed): `eslint` → `Expected an error object to be thrown` (`@typescript-eslint/only-throw-error`, **recommended-type-checked**, requires type info; this is the current name — it was `no-throw-literal`).

---

## Types & safety

### T1 · Non-null assertion `!` — `no-non-null-assertion` (strict, no type info)
`!` suppresses the check without proving the value is non-null.

```typescript
// bad
function name(u: { name?: string } | null): string { return u!.name!; }
```
```typescript
// good — narrow with a real guard
function name(u: { name?: string } | null): string {
  if (u?.name == null) throw new Error('no name');
  return u.name;
}
```
Basis: Google TS (Non-null assertions). Anchor (observed): `eslint` → `Forbidden non-null assertion` (`@typescript-eslint/no-non-null-assertion`, **strict** preset, no type info).

### T2 · `||` where `??` is meant — `prefer-nullish-coalescing` (stylistic-type-checked)
`||` also replaces `0`/`''`/`false`; `??` replaces only `null`/`undefined`.

```typescript
// bad — port 0 becomes 8080
const port = config.port || 8080;
```
```typescript
// good
const port = config.port ?? 8080;
```
Basis: TS Handbook (Narrowing). Anchor (observed): `eslint` → `Prefer using nullish coalescing operator (??) instead of a logical or (||)` (`@typescript-eslint/prefer-nullish-coalescing`, **stylistic-type-checked**, requires type info).

---

## Modules & comments
