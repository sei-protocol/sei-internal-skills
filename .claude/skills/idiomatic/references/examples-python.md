# Python idiom — worked examples

Loaded **on demand** by the method (step 3) when a worked before/after teaches faster than the rule alone — most useful for the §3 *divergences* and the *judgment-only* dimensions. Each example pairs with a dimension in `language-pack-python.md`; cite the same authority and §7 anchor. **Consult a pair for the pattern and cite it — do not paste the block wholesale into a review.**

These pairs are **original** (authored for this pack, not reproduced from any book/doc — copyright-clean). For lint-anchored items, **"Anchor (observed)"** quotes the diagnostic the *bad* snippet produced when run through `ruff` 0.15.17 / `mypy` 2.1.0 / `black` 26.5.1 — so the cited check is real, correctly named, and fires where claimed; the *good* snippet passes clean. For judgment-only items there is no rule — cite the prose Basis and say no checkable rule exists; never fabricate one.

How to read severity (pack §6): a footgun marked **correctness** is a bug, lead with it; a **judgment-only** item has no machine-checkable anchor; everything else is **style** — bundle it, never lead. Most `ruff` rule-sets here are **opt-in** (not defaults) — confirm the repo selected them (pack §6 profile note) before citing as build-failing.

---

## Divergences — where Python rejects other-language habits (judgment-only; examples matter most)

### §3 · EAFP over LBYL
Pre-checking opens a TOCTOU race — the key can vanish between the check and the use; assume validity and catch instead.

```python
# bad — LBYL: the key can vanish between the check and the use
if key in cache and cache[key] is not None:
    return cache[key]
return load(key)
```
```python
# good — EAFP
try:
    return cache[key]
except KeyError:
    return load(key)
```
Basis: docs glossary (EAFP / LBYL — "can risk introducing a race condition"). Anchor: **none — judgment-only** (no linter says "this should have been try/except"; `SIM105` only tidies an existing `try/except/pass` into `contextlib.suppress`).

### §3 · Attribute over getter/setter
Expose the attribute; reach for `@property` only when access needs behavior. No Java-style accessor pairs.

```python
# bad — accessor pair over a plain value
class Point:
    def __init__(self, x): self._x = x
    def get_x(self): return self._x
    def set_x(self, v): self._x = v
```
```python
# good — public attribute; add @property later only if behavior is needed, callers don't change
class Point:
    def __init__(self, x): self.x = x
```
Basis: data model §3.3; `property` docs. Anchor: **none — judgment-only** (no linter flags a getter/setter pair).

### §3 · Protocol over forced ABC inheritance
Structural ("duck") typing — a class that has the methods matches, without inheriting.

```python
# bad — forces every implementer to subclass
from abc import ABC, abstractmethod
class Reader(ABC):
    @abstractmethod
    def read(self) -> bytes: ...
```
```python
# good — structural; anything with read() matches, no inheritance
from typing import Protocol
class Reader(Protocol):
    def read(self) -> bytes: ...
```
Basis: PEP 544 (Protocols — static duck typing). Anchor: **none — judgment-only** (no linter says "use a Protocol here").

### §3 · Comprehension over a manual accumulate loop
A comprehension for a simple transform/filter reads as one expression.

```python
# bad
result = []
for x in items:
    if x > 0:
        result.append(x * 2)
```
```python
# good
result = [x * 2 for x in items if x > 0]
```
Basis: tutorial §5.1.3 (comprehensions "more concise and readable"). Anchor: **none for the loop→comprehension rewrite — judgment-only**; the *adjacent* `C416` fires only on an *unnecessary* comprehension that's a plain copy (`[i for i in xs]` → `list(xs)`): **observed** `C416 Unnecessary list comprehension (rewrite using \`list()\`)`.

### §3 · Module-level function over a needless class
A stateless operation is a function; don't wrap it in a class to feel object-oriented.

```python
# bad — a class holding no state, one method
class Slugifier:
    def slugify(self, s: str) -> str:
        return s.lower().replace(" ", "-")
```
```python
# good
def slugify(s: str) -> str:
    return s.lower().replace(" ", "-")
```
Basis: PEP 20 ("Simple is better than complex"); Python has no "everything in a class" rule. Anchor: **none — judgment-only**.

### §3 · Explicit over clever/implicit
A reader — and a type checker — should *see* the surface, not have it conjured at runtime.

```python
# bad — __getattr__ conjures attributes; nothing can see or check the surface
class Config:
    def __getattr__(self, name):
        return self._data[name]
```
```python
# good — explicit, discoverable, type-checkable
from dataclasses import dataclass
@dataclass
class Config:
    host: str
    port: int
```
Basis: PEP 20 ("Explicit is better than implicit"). Anchor: **none — judgment-only** (no linter flags a `__getattr__` shortcut as "too implicit").

## Lint-anchored footguns (correctness — lead with these)

### §1 P7 · Mutable default argument
The default is created **once** at definition and shared across calls — mutating it leaks state between calls.

```python
# bad
def append(item, bucket=[]):
    bucket.append(item)
    return bucket
```
```python
# good
def append(item, bucket=None):
    bucket = [] if bucket is None else bucket
    bucket.append(item)
    return bucket
```
Basis: PEP 20; flake8-bugbear. Anchor (observed): `ruff --select B` → `B006 Do not use mutable data structures for argument defaults`. (Mutable **class** attribute → `RUF012 Mutable default value for class attribute`.)

### §1 P9 · Bare / swallowing except
A bare `except:` catches `SystemExit`/`KeyboardInterrupt` too and hides every failure.

```python
# bad
try:
    risky()
except:
    pass
```
```python
# good — specific type; if a swallow is intended, say why
import contextlib
with contextlib.suppress(FileNotFoundError):  # absence is fine here
    risky()
```
Basis: PEP 8 (bare except); PEP 20 ("errors should never pass silently"). Anchor (observed): `ruff` → `E722 Do not use bare \`except\``.

### §1 P2 · Legacy typing forms
On modern Python use builtin generics + `|` unions; `typing.List`/`Optional` are deprecated, non-idiomatic, and miss tooling improvements.

```python
# bad
from typing import List, Optional
def first(xs: List[int]) -> Optional[int]: ...
```
```python
# good
def first(xs: list[int]) -> int | None: ...
```
Basis: PEP 585 (builtin generics), PEP 604 (`X | Y`). Anchor (observed): `ruff --select UP` → `UP045 Use \`X | None\` for type annotations`, `UP006 Use \`list\` instead of \`List\` for type annotation`, `UP035 \`typing.List\` is deprecated, use \`list\` instead`.

### §1 P9 / P1 · Singleton & type comparison
Compare singletons by identity; test type with `isinstance`.

```python
# bad
if x == None: ...
if flag == True: ...
if type(x) == int: ...
```
```python
# good
if x is None: ...
if flag: ...
if isinstance(x, int): ...
```
Basis: PEP 8 (Programming Recommendations). Anchor (observed): `E711 Comparison to \`None\` should be \`cond is None\``; `E712 Avoid equality comparisons to \`True\``; `E721 Use \`is\` and \`is not\` for type comparisons, or \`isinstance()\``.

### §1 P5 · `os.path` string-munging → `pathlib`
`pathlib` is the modern, OS-portable, object API.

```python
# bad
import os
cfg = os.path.join(base, "conf", "app.toml")
```
```python
# good
from pathlib import Path
cfg = Path(base) / "conf" / "app.toml"
```
Basis: `pathlib` docs; flake8-use-pathlib. Anchor (observed): `ruff --select PTH` → `PTH118 \`os.path.join()\` should be replaced by \`Path\` with \`/\` operator`. (`PTH` is opt-in.)

### §1 P8 · Wildcard import
`import *` pollutes the namespace and defeats unused-import + undefined-name analysis.

```python
# bad
from os import *
print(getcwd())
```
```python
# good
from os import getcwd
print(getcwd())
```
Basis: PEP 8 (Imports — "wildcard imports should be avoided"). Anchor (observed): `F403 \`from os import *\` used; unable to detect undefined names`; `F405 \`getcwd\` may be undefined, or defined from star imports`.

### §5 asyncio · Blocking the event loop / dangling task (correctness)
Sync calls in an `async def` stall every coroutine; an unreferenced task can be GC'd mid-flight.

```python
# bad
import asyncio, time
async def worker():
    time.sleep(1)                 # blocks the whole loop
    asyncio.create_task(flush())  # result discarded — may be GC'd
```
```python
# good
import asyncio
_tasks: set[asyncio.Task[None]] = set()   # a live reference survives GC
async def worker():
    await asyncio.sleep(1)
    _tasks.add(asyncio.create_task(flush()))  # keep the reference
```
Basis: asyncio overlay (pack §5 A1/A2). Anchor (observed): `ruff --select ASYNC` → `ASYNC251 Async functions should not call \`time.sleep\``; `ruff --select RUF006` → `RUF006 Store a reference to the return value of \`asyncio.create_task\``.

---

## FastAPI (framework overlay — pack §5; enable Ruff's `FAST` set)

### §5 FA1/FA2 · Annotated dependency style — the `B008` inversion
The `= Depends(...)` default-value form loses static type info and trips generic `B008`; the `Annotated` form the docs prefer is clean under both `FAST` and `B008`.

```python
# bad
from fastapi import FastAPI, Depends
app = FastAPI()
def common_params(q: str | None = None) -> dict:
    return {"q": q}
@app.get("/items/")
async def read_items(commons: dict = Depends(common_params)):  # default-value form
    return commons
```
```python
# good
from typing import Annotated
from fastapi import Depends, FastAPI
app = FastAPI()
def common_params(q: str | None = None) -> dict:
    return {"q": q}
CommonsDep = Annotated[dict, Depends(common_params)]  # reusable; type preserved
@app.get("/items/")
async def read_items(commons: CommonsDep) -> dict:
    return commons
```
Basis: FastAPI tutorial/dependencies — "Prefer to use the `Annotated` version if possible." Anchor (observed): `ruff --select FAST002,B008` on the bad → `FAST002 FastAPI dependency without \`Annotated\`` **and** `B008 Do not perform function call \`Depends\` in argument defaults`; the good passes `--select FAST,B008` clean ("All checks passed!"). Note FA2: in a real FastAPI repo, exempt the `B008` false positive via `lint.flake8-bugbear.extend-immutable-calls = ["fastapi.Depends","fastapi.params.Depends", …]` — the `Annotated` form above needs no exemption.

### §5 FA3 · `response_model` only when output differs from the return type
A `response_model=` that duplicates the return annotation is redundant; reserve it for the filtering/security case (return a richer object, declare a narrower output model).

```python
# bad
@app.post("/items/", response_model=Item)   # duplicates the return type
async def create_item(item: Item) -> Item:
    return item
```
```python
# good
@app.post("/items/")                          # return annotation IS the output model
async def create_item(item: Item) -> Item:
    return item
# (keep response_model= ONLY when the output differs — e.g. return UserIn under
#  response_model=UserOut to strip `password` from the response.)
```
Basis: FastAPI tutorial/response-model. Anchor (observed): `ruff --select FAST001` on the bad → `FAST001 FastAPI route with redundant \`response_model\` argument`; the good passes `--select FAST` clean.

### §5 FA4 · Every `{param}` in the route path has a same-named arg
A `{param}` in the path string with no matching function argument is silently unbound — the route can't receive it.

```python
# bad
@app.get("/things/{thing_id}")
async def read_thing(query: str) -> dict:   # no `thing_id` arg
    return {"query": query}
```
```python
# good
@app.get("/things/{thing_id}")
async def read_thing(thing_id: int, query: str) -> dict:
    return {"thing_id": thing_id, "query": query}
```
Basis: FastAPI tutorial/path-params (signature-name matching). Anchor (observed): `ruff --select FAST003` on the bad → `FAST003 Parameter \`thing_id\` appears in route path, but not in \`read_thing\` signature`; the good passes clean. (The separate `/users/me`-before-`/users/{id}` ordering footgun is judgment-only — no `FAST` rule covers it.)

### §5 FA5 · Don't block the event loop in an `async def` route (correctness)
A sync call in an `async def` route stalls every concurrent request. Use a plain `def` route (FastAPI runs it in a threadpool) or an async client + `await`.

```python
# bad
import time, requests
@app.get("/data/")
async def get_data() -> dict:
    time.sleep(1)                              # blocks the loop
    return requests.get("https://x/api").json()  # sync HTTP in async route
```
```python
# good — plain def: FastAPI runs it in a threadpool, the loop stays free
import requests
@app.get("/data/")
def get_data() -> dict:
    return requests.get("https://x/api").json()
```
Basis: FastAPI `/async/` — "If you just don't know, use normal `def`." Anchor (observed): `ruff --select ASYNC` on the bad → `ASYNC251 Async functions should not call \`time.sleep\`` **and** `ASYNC210 Async functions should not call blocking HTTP methods`; the good is a plain `def` (no async context → the `ASYNC` rules don't apply).

---

## Type quality — requires a configured type checker (pack §5 TC1)

### §1 P2 · Public boundary annotated; types must match
Annotate the exported surface; a type checker then catches mismatches a linter can't.

```python
# bad — untyped boundary + a real mismatch
def add(a, b):
    return a + b
total = add("x", 3)
```
```python
# good
def add(a: int, b: int) -> int:
    return a + b
total = add(1, 3)
```
Basis: PEP 484/526; `typing` docs. Anchor (observed): `mypy --strict` → `error: Function is missing a type annotation [no-untyped-def]`; on a forced mismatch `error: Argument 1 to "add" has incompatible type "str"; expected "int" [arg-type]` and `error: Incompatible return value type (got "str", expected "int") [return-value]`. (`[no-untyped-def]` fires only under `--strict`/`--disallow-untyped-defs`; no type checker → judgment-only.)

### §1 P1 / P3 · Naming & docstrings on the public surface
snake_case functions, CapWords classes; public functions carry a PEP 257 docstring.

```python
# bad
def getItem(): return 1          # camelCase, no docstring, no return type
```
```python
# good
def get_item() -> int:
    """Return the current item."""
    return 1
```
Basis: PEP 8 (Naming), PEP 257 (Docstrings). Anchor (observed): `ruff --select N,ANN,D` → `N802 Function name \`getItem\` should be lowercase`; `ANN201 Missing return type annotation for public function`; `D103 Missing docstring in public function`. (`N`/`ANN`/`D` are opt-in; `D` needs a chosen convention.)

### §1 P9 · f-string in a logging call
An f-string formats **eagerly**, even when the log level is disabled — wasted work and it defeats log aggregation by message template.

```python
# bad
logging.info(f"processed {n} items")
```
```python
# good
logging.info("processed %s items", n)
```
Basis: flake8-logging-format. Anchor (observed): `ruff --select G` → `G004 Logging statement uses f-string`. (`G` is opt-in.)

---

## Formatting — defer to the formatter (pack §1 P11)

### §1 P11 · Let `black` own whitespace & quotes
Don't hand-format; run the formatter. Line length is the project's configured value (black/Ruff default 88), not a hand-litigated 79.

```python
# bad
x = {'a':1,'b':2}
y=[1,2,3]
```
```python
# good — produced by `black` (double quotes, spacing)
x = {"a": 1, "b": 2}
y = [1, 2, 3]
```
Basis: black *code style* (deterministic, line-length 88). Anchor (observed): `black --check` reformats the bad block to the good one (`'`→`"`, `:`/`=` spacing); `black` is the authority, not a lint code.
