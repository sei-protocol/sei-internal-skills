# tide/

Single source of truth for cross-component interfaces, plus the team runbook for the Tide agent council.

## Files

- **`interface-registry.yaml`** — machine-readable contract for every interface that crosses a component boundary: event signatures, function signatures, EIP-712 type hashes, env vars, exit codes, volume mounts, K8s resources. The registry **is the spec** — when an interface changes, this file is updated first, then the LLDs and code follow.
- **`RUNBOOK.md`** — daily usage of the `/council`, `/coral`, and `/verify` skills. Start here if you're operating the council, not modifying it.

## Why a separate registry

Specs drift. Code drifts. The registry keeps drift visible.

- `scripts/verify_registry.py` checks that runtime env-var references, ServiceAccount patterns, and contract function names match what's in this file.
- CI runs the same check on every PR that touches `pkg/`, `runtimes/`, `contracts/`, `manifests/`, or the registry itself (see `.github/workflows/verify-interfaces.yml`).
- The `/council` and `/coral` skills read the registry before every cross-review pass.
- The `/verify` slash command wraps the script with manual checks (event topic hashes, exit-code handling).

## Ownership rule

Each entry has a `provider` and a `consumer` (or `consumers`). **The provider owns the interface — consumers adapt.** This avoids circular dependencies during interface resolution. The full ownership table is in `AGENTS.md` at the repo root.

## One-way doors

Entries flagged `one_way_door: true` cannot be changed without significant pain after deployment:

- Solidity event signatures (breaks all indexers)
- EIP-712 type hashes (invalidates existing signatures)
- CRD `spec` field names (requires migration)
- Storage slot positions in upgradeable contracts (corrupts state)

Both `/council` and `/coral` require explicit user approval before finalizing any change to a one-way door entry. See `design/constitution/constitution.md` for the full list.

## Updating the registry

1. Edit `interface-registry.yaml` first — provider's definition wins
2. Update the affected LLDs under `design/milestones/`
3. Run `/verify` (or `python scripts/verify_registry.py`)
4. Commit with `feat:` / `refactor:` / `docs:` referencing the component
