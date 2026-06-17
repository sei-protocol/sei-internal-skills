# sei_omnigent — Sei's Omnigent overlay

The out-of-tree overlay that adopts Omnigent as the meta-harness beneath Tide
while keeping Sei's domain substrate (chain registry, tenancy, TEE memory) in
Sei's hands. Implements **design #11** (`bdchatham-designs/designs/sei-agentic-mesh/`)
— the *hybrid line*: **adopt the runtime glue, own the substrate at the store/identity boundary.**

> Owned in `sei-protocol/Tide`. Designs live in `bdchatham-designs`. Deployment
> runs on the Sei platform Kubernetes cluster. Omnigent is a **pinned dependency**
> (`omnigent==0.1.1`), never a fork.

## The seam (PLT-667)

`omnigent serve` hard-codes its stores. This overlay replaces it with a custom
entrypoint that exposes the store layer as an injection point:

- **`make_stores(cfg) -> Stores`** — the single seam. Phase-1 returns the stock
  SqlAlchemy/Local stores byte-for-byte. Phase-2 swaps `Stores.agent`/`.artifact`
  for chain-backed implementations; Phase-3 swaps `Stores.conversation` for a
  TEE-sealed one — a one-line change here, no Omnigent fork.
- **`build_server(cfg) -> FastAPI`** — clones Omnigent's serve boot sequence
  (DECISION-1=B) but sources every store from `make_stores`, and wires
  `create_app` **by keyword**.
- **`_omnigent_shim`** — the *only* file that imports Omnigent. A pinned-tag bump
  re-verifies here (the DECISION-1 drift-watch surface).

## One-way doors (need human sign-off)

- The `Stores` dataclass field set/names — the seam contract every later phase binds to.
- `create_app` keyword-only wiring (parameter order ≠ store order).
- Auth mode `header` + `account_store=None` (PLT-669) — enabling accounts/OIDC later re-crosses the upstream `app.py:1748` construction.

## Build & deploy

`src/` layout: the package is at `src/sei_omnigent/`; `pyproject.toml` + `tests/`
sit at the project root. Install with `pip install .` (or `pip install -e .` for
dev) — this ships the package + the `sei-omnigent-serve` console script. The CI
`wheel-smoke` job guards that the built wheel is importable.

**Deploy image (PLT-672):** the operator builds `PLACEHOLDER_IMAGE` by
`pip install .`-ing this overlay **into the pinned Omnigent base image** (the
overlay declares `omnigent==0.1.1`, so installing it alongside omnigent gives the
container both). The K8s entrypoint is `python -m sei_omnigent.server.serve_main`
(≡ the `sei-omnigent-serve` script) — a dotted module path, layout-independent.

## Phase-1 slices

| Issue | Adds |
|---|---|
| **PLT-667** *(this)* | the `make_stores` seam + `build_server` |
| PLT-668 | server-default read-only policy (`github_policy` + `deny_mutating_os`) |
| PLT-669 | header-mode auth behind Sei SSO + boot-asserts |
| PLT-670 | `claude-native` harness invariant + roster-discoverable guard |
| PLT-672 | K8s deployment on the platform cluster (overlay manifests) |

## Status

In review (Tide PR #181, cross-review round 2). Not yet wired to CI or the deploy
overlay. `build_server` is import-safe with `omnigent==0.1.1` installed; it
returns a FastAPI app but does not load config or bind/serve — the runnable
`sei-omnigent-serve` console script is added in PLT-672 (deploy).
