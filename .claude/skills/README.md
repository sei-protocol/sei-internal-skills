# Skill catalog

Every directory here that holds a `SKILL.md` is a skill, and `make update` installs it. Edit a skill here, never in `~/.claude/skills/`, because the next sync overwrites a local edit. For the install steps, read the root [`README.md`](../../README.md).

## Skills

- **`harbor-dev/`**: the engineer interface to the harbor EKS dev cluster. A request can be "onboard me", "spin up a chain", "add an RPC fleet", "run a bench" or "tear it down". The skill turns each request into two kinds of action:
  - `seictl network` and `seictl node` calls, on the **SeiNetwork + SeiNode** model;
  - GitOps pull requests against `sei-protocol/harbor-engineering-workspace`. Networking stays outside the controller. The engineer owns the `HTTPRoute`s, plus a load balancer when a use case needs external access.

  A third tree, `seictl workflow`, re-bootstraps or migrates an *existing* node in place, outside the PR and Flux flow. It is the one destructive verb of the skill, and it needs explicit engineer sign-off. The skill needs `seictl` **v0.0.72** or newer. See `harbor-dev/SKILL.md` gate 1.

- **`giga-dev/`**: architecture and operating knowledge for the shared giga testnet, `giga-testnet-0`. It is the Autobahn EVM-only chain across the four prod cells. The skill covers:
  - the cell and node topology, the transaction path, and the serial execute loop that sets the throughput limit;
  - how the load generators and their KEDA scaler hold the chain inside an execute-loop busy band;
  - where every log and metric lives;
  - diagnosis of saturation, stragglers, full-node lag, the block-retention window and scaler faults.

  It is read-only by default. It never suspends a cell-wide Flux Kustomization or wipes a validator. It releases a load pin only when all 40 validators pass the gate. NOT for an engineer's own chain on harbor (`/harbor-dev`).

- **`kubernetes/`**: design and review Kubernetes **operator and controller** code (CRDs, reconcilers, controller-runtime and kubebuilder). It rests on the upstream canon (K8s API conventions, controller-runtime, CRD versioning) and an **always-first Sei-controller profile**. The profile holds the enforced conventions of sei-k8s-controller:
  - plan-driven reconcile, optimistic-lock single-patch status;
  - always-present conditions and reason-as-API, CEL immutability one-way doors, the `kubectl wait` latch.

  It has a method, 5 review dimensions and pluggable kits (`plan-driven-reconciliation`, `sidecar-task-integration`, `crd-design`, `child-resource-lifecycle`; more deferred). Pairs with the `kubernetes-specialist` agent. Distinct from Go idiom review, right-sizing and scheduling (the platform team), deployment manifests (a sei-protocol/platform PR), and node P2P/RPC (`sei-network-specialist`).

## Adding a skill

Follow [Changing the catalog](../../CLAUDE.md#changing-the-catalog) in `CLAUDE.md`.

## Using a skill in another repository

`make update` installs every skill in user scope, `~/.claude/skills/`, so Claude Code finds it in every repository. Copy a skill into a repository only when people who did not install the catalog must get it from that repository:

```sh
./scripts/sync-skills.sh --target <repo> --force
```

[`scripts/README.md`](../../scripts/README.md#sync-skillssh-and-sync-agentssh) says how the script treats a target copy that differs.
