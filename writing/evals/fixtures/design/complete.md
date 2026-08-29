# Federated telemetry for the platform fleet

## Background

Each cell keeps its own metrics and no query spans two cells.

## Goals

A single query surface over every cell.

## Non-goals

Long-term retention. That is a separate decision.

## Design

One aggregation layer per region, with a global query front end above it.

## Alternatives

A single central store. Rejected: one region's outage takes every query down.

## Trade-offs

Query latency rises by one network hop. Cell isolation is worth the hop.

## Open questions

Whether the front end needs its own cache.

## References

* `manifests/base/telemetry`
