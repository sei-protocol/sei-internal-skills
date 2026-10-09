# Giga Dev

> Architecture and operating knowledge for the shared giga testnet: the Autobahn EVM-only chain that runs as `giga-testnet-0` across the four prod platform cells.

giga-dev explains how the giga testnet works end to end. It covers how its cells, validators, RPC full nodes and lanes fit together, and how a transaction travels from a load generator to execution. It explains why the serial execute loop sets the throughput limit. It shows how the load generators and their KEDA scaler hold the chain below saturation. It shows where every log and metric lives, how to reach it, and how to read it. It also diagnoses the states the chain gets into: saturation, stragglers, full-node lag, the block-retention window and scaler faults.

The skill is read-only by default. Any write to the shared cells needs an explicit request, and the skill echoes the change before it acts. It never suspends a cell-wide Flux Kustomization, and it never wipes a validator. It releases a load pin only when all 40 validators pass the release gate.

| | |
|---|---|
| **Skill** | [`SKILL.md`](./SKILL.md) |
| **Install** | `gh api repos/sei-protocol/sei-internal-skills/contents/scripts/install.sh -H 'Accept: application/vnd.github.raw' \| bash -s -- skill giga-dev` |
| **Sibling** | [`harbor-dev`](../harbor-dev/README.md) for an engineer's own chain on harbor |

## What it covers

- **Architecture** ([`references/architecture.md`](references/architecture.md)): cells, regions and node roles. How Flux and the controller declare the nodes. Autobahn lanes and ordering, EVM-only execution and sender sharding. The transaction path, the execute loop, retention and the performance envelope.
- **Load and scaling** ([`references/load-and-scaling.md`](references/load-and-scaling.md)): sei-load's closed loop and per-pod rate cap. The KEDA triggers and formula, and why the scaler holds 70% to 90% busy. The hard stops, and the pin and release procedure.
- **Observability:** Grafana, Thanos and Loki access, log recipes, a metrics catalog of named queries arranged by question, dashboards, alerts and blind spots ([`references/observability.md`](references/observability.md)).
- **Diagnosis** ([`references/diagnosis.md`](references/diagnosis.md)): an ordered set of questions. Failure modes as mechanism, symptom, check and safe response. Operating rules for shared cells, the fresh-start design and one-way doors.

## What it does not cover

- An engineer's own Autobahn or giga chain on harbor: use `/harbor-dev`.
- sei-k8s-controller code and CRDs: use `/kubernetes`.
- General platform manifests, Flux and Kustomize: use `/platform`.
- arctic-1, pacific-1 and other chains.
- The history of how the testnet reached its current design. The skill teaches the system as it is.
