# Cell-local ingest for the metrics fleet

## 1. Introduction and goals

Every cell writes metrics locally and no query spans two cells.

## 4. Solution strategy

One aggregation layer per region, with a global query front end above it.

## 2. Architecture constraints

The fleet runs on managed Kubernetes and the query surface must stay read-only.

## Non-goals

Long-term retention. That is a separate decision.

## Alternatives

A single global write path. Rejected: one region's outage would stop every write.

## Trade-offs

A per-region layer costs more to run than one global layer. It buys blast-radius
containment, and an outage then costs one region rather than the fleet.

## Open questions

Who owns the regional layer once it runs in three regions?
