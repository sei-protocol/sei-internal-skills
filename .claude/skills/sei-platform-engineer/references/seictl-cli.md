# seictl CLI surface

Canonical command reference. **`seictl --help` is the source of truth** — when this file disagrees, the CLI wins. Regenerate from `seictl --help` periodically.

Last verified: 2026-04-26 against seictl `<unreleased>` (initial draft — commands below are the design contract; not yet implemented).

## Top-level commands

| Command | Domain | What it does |
|---|---|---|
| `seictl config patch` | local | Patch app.toml/client.toml/config.toml |
| `seictl genesis patch` | local | Patch genesis JSON |
| `seictl patch` | local | Generic TOML/JSON merge-patch |
| `seictl serve` | local | Run the in-pod sidecar HTTP server |
| `seictl await` | local | Wait condition |
| `seictl bench` | cluster | Manage benchmark workloads (up/down/list) |
| `seictl onboard` | cluster | Set up your engineer environment |
| `seictl status` | cluster | What you own across the cluster |
| `seictl seinode` | cluster | SeiNode operations (list/diagnose) |
| `seictl controller` | cluster | Controller health |
| `seictl context` | cluster | Cluster + identity ground truth |

The skill only invokes commands flagged "cluster" above. The "local" commands aren't in the skill's interface today.

## Cluster-facing commands (skill scope)

### `seictl bench up`

```
seictl bench up --image <ref> [--slug <slug>] [--size s|m|l] [--duration <minutes>] [--profile <name>] [--apply]
```

[outline: required vs optional flags, defaults, structured output schema, exit codes specific to bench]

### `seictl bench down`

```
seictl bench down --slug <slug>
```

[outline]

### `seictl bench list`

```
seictl bench list [--all-namespaces]
```

[outline: structured JSON shape — list of objects with chain ID, image digest, phase, age]

### `seictl onboard`

```
seictl onboard --alias <alias> [--no-pr] [--update]
```

[outline: --no-pr generates files but doesn't push; --update edits ~/.seictl/engineer.json without regenerating namespace files]

### `seictl status`

```
seictl status [--owner <alias>]
```

[outline: defaults to current engineer's alias from ~/.seictl/engineer.json; lists CRs labeled with that owner]

### `seictl seinode list`

```
seictl seinode list [--all-namespaces] [-n <namespace>]
```

[outline]

### `seictl seinode diagnose`

```
seictl seinode diagnose <name> [-n <namespace>]
```

[outline: phase-aware structured report; shape includes `.phase`, `.conditions`, `.failedTask`, `.recommendation`, `.nextCommand`]

### `seictl controller inspect`

```
seictl controller inspect
```

[outline: controller pod health, leader lease, recent reconcile errors, CRD versions installed]

### `seictl context`

```
seictl context
```

[outline: cluster name, kube-context, AWS account, engineer identity, RBAC verbs available — single fixed-shape JSON object the agent can consult before any other call]

## Common patterns

### Output format

All cluster commands accept `--format=json` (default) and `--format=text`.

### Exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 2 | Usage error |
| 3 | Resource not found |
| 4 | Cluster unreachable |
| 5 | Permission denied |
| 10+ | Domain-specific (per-command — documented in `--help` for each verb) |

### Read-only by default

`bench up` and `bench down` default to **dry-run**. They print the rendered manifests and the planned kubectl actions; nothing is applied without `--apply`. This makes them safe to expose as MCP tools later without auth gymnastics.

### No ambient state

Commands never `cd`, never modify `~/.kube/config`, never set env vars in the calling shell. Every kubectl invocation is explicit about context and namespace.

## MCP graduation notes

Each subcommand maps to one MCP tool. The MCP server flattens names (`bench_up`, `seinode_diagnose`, etc.) — no `cluster_` infix to strip. Tool descriptions come from `seictl <cmd> --help`.
