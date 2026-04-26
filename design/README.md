# design/

Governing documents and per-component low-level designs (LLDs) for Tide.

## Layout

```
design/
├── constitution/                       # The working agreement
│   └── constitution.md                 # Principles, design template, naming conventions
├── high-level/                         # End-to-end system design
│   └── tide-agent-council.md           # Contracts, runtimes, lifecycle, security, costs
├── milestones/                         # Per-component LLDs grouped by delivery milestone
│   ├── m0-contracts/                   # TideCouncil + TideJobHook + deployment suite
│   ├── m1-platform/                    # K8s manifests + Tide Operator
│   ├── m2-review/                      # Agent review runtime
│   ├── m3-execution/                   # Agent execution runtime
│   └── mvp/                            # Event-driven GHA path (parallel track)
└── cross-reviews/                      # Cross-component interface review artifacts
    ├── cross-review-blockchain.md
    ├── cross-review-operator.md
    ├── cross-review-platform.md
    └── cross-review-mvp.md
```

## Reading order

1. **`constitution/constitution.md`** — design principles (two-way doors, YAGNI, interfaces first, errors are interface) and the LLD template every spec follows
2. **`high-level/tide-agent-council.md`** — the system end-to-end: ERC standards, on-chain primitives, off-chain orchestrator, K8s runtime, security model
3. **`milestones/<milestone>/`** — pick the milestone you're working on; each has its own README + LLDs
4. **`cross-reviews/`** — what was caught when adjacent LLDs were diff'd against each other

## How a design moves through this directory

1. **Draft** — owner authors an LLD in `milestones/<milestone>/lld-<component>.md` following the constitution's template
2. **Cross-review** — adjacent owners review the interface boundaries; mismatches go in `cross-reviews/<topic>.md`
3. **Resolve** — `tide/interface-registry.yaml` is updated **first**, then LLDs and (once written) code
4. **Verify** — `/verify` confirms code matches the registry; CI runs the same check on PRs that touch interface-relevant paths

The `/council` skill orchestrates this end-to-end. The `/coral` skill handles single-slice work without the full ceremony. See `../tide/RUNBOOK.md`.

## What is and isn't here

- **Is here:** specs, principles, cross-review artifacts, deferred-features lists
- **Isn't here:**
  - The interface registry — at `../tide/interface-registry.yaml` so tooling can parse it
  - Agent personas — at `../.claude/agents/`
  - Implementation code — lands at `pkg/`, `runtimes/`, `contracts/`, `manifests/` as each milestone reaches done
