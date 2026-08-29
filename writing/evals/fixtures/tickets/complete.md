# Add a drift guard for the doctrine block

## Problem

Nothing enforces that the distributed block matches its source.

## Impact

The block drifts silently until someone notices at review.

## Relevant experts

* `build-engineer` — the check mode and the CI workflow.

## Proposed approach

Add a read-only check mode that compares the file against the source and exits
non-zero on a difference.

## Acceptance criteria

* The check mode exits non-zero on drift and writes nothing.
* A clean tree exits zero.

## Out of scope

* Auto-fixing in CI.

## References

* `scripts/inject-doctrine.sh`
