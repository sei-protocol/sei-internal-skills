# Federated telemetry for the platform fleet

## Background

Each cell keeps its own metrics and no query spans two cells.

## Goals

A single query surface over every cell.

## Design

One aggregation layer per region, with a global query front end above it.
