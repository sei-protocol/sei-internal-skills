# Platform Dev

> Working knowledge for the Sei platform: the `sei-protocol/platform` repository and the six EKS cells that Flux syncs from it.

platform-dev brings an engineer up to speed on the platform and stays with them while they work on it. It explains the cells and the path of a change from a pull request to a running cell. It also explains Terraform, traffic, secrets, observability and the workloads beside the chains. It reads live state with `kubectl`, `flux` and `/grafana`, so every answer about the fleet comes with the query and the time that produced it. It diagnoses platform faults in a fixed order. It also holds the common procedures, from a Terraform run to a release roll.

The skill is read-only by default. Any write needs an explicit request, and the skill echoes the change before it acts. It never decrypts or prints a secret, never weakens a prune guard, and never applies Terraform without a plan in a merged pull request.

| | |
|---|---|
| **Skill** | [`SKILL.md`](./SKILL.md) |
| **Siblings** | [`giga-dev`](../giga-dev/README.md) for the giga testnet, [`harbor-dev`](../harbor-dev/README.md) for an engineer's own chain on harbor |
| **Data** | [`grafana`](../grafana/SKILL.md) for metrics, logs and alerts |

## What it covers

- **Architecture** ([`references/architecture.md`](references/architecture.md)): cells, the repository map, the change flow and the prune guards. Then Terraform, traffic, secrets, the observability stack and the workloads.
- **Observability** ([`references/observability.md`](references/observability.md)): access through `/grafana`, named queries by question, log selectors, the alert path, dashboards and blind spots.
- **Diagnosis** ([`references/diagnosis.md`](references/diagnosis.md)): failure modes as mechanism, symptom, check and safe response.
- **Procedures** ([`references/procedures.md`](references/procedures.md)): the health sweep and the common changes, each with its writes named, and the one-way doors.
- **Maintenance** ([`references/maintenance.md`](references/maintenance.md)): a check for each drift-prone fact and the platform paths that make the skill stale. It also has a sweep for alert rules that cannot fire.

## Keeping it current

The skill records structure and keeps counts and versions in checks. When a platform pull request changes a path in the [update triggers](references/maintenance.md#2-update-triggers), update the skill in a `fix(platform-dev)` pull request. For a full refresh, ask `/platform-dev refresh the skill`. The skill runs the verify sweep and the rule-metric sweep, then proposes one pull request.

## What it does not cover

- giga-testnet-0 load, lag and saturation: use `/giga-dev`.
- An engineer's own chain on harbor, and engineer onboarding: use `/harbor-dev`.
- sei-k8s-controller code and CRDs: use `/kubernetes`.
- The design of alerts, recording rules and dashboards: use the `sre-engineer` agent.
