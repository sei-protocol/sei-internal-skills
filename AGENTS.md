# sei-internal-skills Agent Roster

This repository hosts specialist agent personas in `.claude/agents/`. They are general-purpose and sync to other repos and user-level via `scripts/sync-agents.sh`. The skills in `.claude/skills/` (notably `/coral`, `/council`, `/xreview`, `/root-cause`) dispatch them.

## Roster

Grouped by **domain** — each agent carries a matching `category:` in its `.claude/agents/<name>.md` frontmatter, and `sync-agents.sh --categories <domain>` syncs a domain. Agents discover **flat** under `~/.claude/agents/`; the domains are metadata, not folders. Sync aliases cross-cut: all agents are `portable` except `sei-network-specialist` (`sei`); `all` is everything.

### platform-infra
| Agent | Scope |
|-------|-------|
| `kubernetes-specialist` | Go + controller-runtime, CRDs, event indexing, Job lifecycle |
| `platform-engineer` | Platform layer — Kustomize/Flux GitOps, EKS Pod Identity, SOPS/KMS secrets, Pod Security, terraform; the sei-k8s-controller deploy manifests. Backed by `/platform`. |
| `network-specialist` | K8s and cloud networking, service mesh |
| `k8s-capacity-management` | Capacity as a discipline: workload right-sizing from observed data, Karpenter NodePool design, DaemonSet overhead, PriorityClass tiers, HPA/VPA/KEDA tuning, scheduling primitives. |
| `sei-network-specialist` | Sei node networking (seid ports, CometBFT P2P, Waterway, Istio quirks). Valuable to any Sei-adjacent work. *(sync alias: `sei`)* |

### observability
| Agent | Scope |
|-------|-------|
| `opentelemetry-expert` | Application-side OpenTelemetry SDK instrumentation. Backend operations → `observability-platform-engineer`. |
| `observability-platform-engineer` | Telemetry backend as a system: Prometheus/Thanos/Loki/Tempo/Alloy/Promtail/Grafana operations, PromQL/LogQL authorship, mixin vendoring, ingester/compactor/store-gateway sizing. |
| `sre-engineer` | Google SRE-flavored: SLOs/SLIs, dashboards, alerts, runbooks (human + agent-callable), post-mortems. Closes the loop by filing `/issue` work when a runbook needs missing tooling. |

### security
| Agent | Scope |
|-------|-------|
| `security-specialist` | Security + adversarial design |

### blockchain
| Agent | Scope |
|-------|-------|
| `solidity-developer` | EVM smart-contract engineering on Sei — Solidity/Foundry, precompiles, gas/parity, address association, upgrade safety, on-chain event indexing. Backed by `/evm`. |

### writing-quality
| Agent | Scope |
|-------|-------|
| `prose-steward` | Dual-audience prose steward — reviews org artifacts (design docs/HLDs, PRDs, 1-pagers) so they read correctly for **both** the human reviewer who scans and the consuming AI agent that ingests linearly and acts on the text. Profile-first (repo `CLAUDE.md` writing conventions outrank its own doctrine), citation-tier honest (Cited findings carry `Basis:`; Stated-opinion is advisory-only, never blocking), suggest-only. Pinned unconditionally by `/xreview` on any `skill-package` change. NOT code idiom (`idiomatic-reviewer`), NOT scope (`product-manager`). |

### code-quality
| Agent | Scope |
|-------|-------|
| `idiomatic-reviewer` | Idiomatic-conformance review, language-pluggable. Digests the repo's agent files + `doc.go` into a local idiom profile that outranks generic idiom, overlays a per-language pack, emits two-altitude (design + surgical) cited findings. Reviews for idiom; does **not** author the system (that's the language specialist, e.g. `kubernetes-specialist`). Backed by the `/idiomatic` skill. |
| `systems-engineer` | Systems software engineer — builds **and** reviews high-performance, reliable, observable, maintainable application code/architectures. Owns "how software behaves on the machine and over time": perf (CPU/mem/I/O/concurrency/latency), failure-modes-by-design (timeouts, back-pressure, idempotency, graceful degradation), observability-by-design, Linux/OS behavior, maintainability. Hooks into the `/idiomatic` standards (idiom ⊂ systems quality) and leans on `idiomatic-reviewer` for the pure idiom pass. Builds/reviews code; does **not** run the platform (→ `sre-engineer` / `platform-engineer` / observability agents). |

### product-management
| Agent | Scope |
|-------|-------|
| `product-engineer` | Product engineering |
| `product-manager` | Product management, scope discipline |
| `go-to-market-specialist` | GTM strategy for novel products — ICP / JTBD / motion / launch. Partner to `product-manager`. |

### release-operations
| Agent | Scope |
|-------|-------|
| `platform-release-manager` | Release management and cut discipline |

One more agent lives in [`experimental/agents/`](./experimental/README.md) and is **not**
installed by default — `sei-interview-expert` (with `/interview`). Opt in with
`make sync-experimental`.

The agent files themselves negotiate cross-agent boundaries (e.g. observability-platform-engineer vs. sre-engineer vs. opentelemetry-expert; k8s-capacity-management vs. platform-engineer). See each `.claude/agents/*.md` for the detailed scope and hand-off rules.

The operating doctrine — engineering principles, output discipline, the workflow skills and when each applies, the xreview discipline, and the key rules — is the `sei-internal-skills-managed` block below. It is maintained once in `scripts/sei-internal-skills-doctrine.md` and distributed to every consuming package; re-inject this repo's copy with `make sync-doctrine-self` after editing the source.

## Install

The fastest path is `make bootstrap` from the repo root, which runs `make sync-agents`, `make sync-skills`, `make sync-output-styles`, and `make update-agent-permissions`. See the README's Setup section for the full flow.

This installs the **core** only. The agents and skills under [`experimental/`](./experimental/README.md) never ride along — opt in by name with `make sync-experimental`.

Agents and skills travel the same way — sei-internal-skills is the canonical home, and the sync scripts push them out to user-scope (`~/.claude/`) and sibling repos.

For sibling-repo or finer-grained installs, call the scripts directly:

```bash
# Mirror portable agents to user-level (any CWD) — same as `make sync-agents`
./scripts/sync-agents.sh --target ~/

# Mirror portable skills to user-level — same as `make sync-skills`
./scripts/sync-skills.sh --target ~/

# Copy portable + sei agents to a sibling repo
./scripts/sync-agents.sh --target ~/work/platform --categories portable,sei

# Copy the sei-team skills (chaos-suite, validate-release, gov-ops, validator-platform, harbor-dev) to user-level
./scripts/sync-skills.sh --target ~/ --categories sei

# Install a single domain (e.g. code-quality → idiomatic, systems)
./scripts/sync-skills.sh --target ~/ --categories code-quality

# Opt into the parked resources (never installed by the commands above)
./scripts/sync-experimental.sh --target ~/

# Preview without copying
./scripts/sync-agents.sh --target ~/ --dry-run
./scripts/sync-skills.sh --target ~/ --dry-run
```

Categories: `portable` (default), `sei`, `all`. Both scripts are non-destructive by default — they refuse to overwrite changed files in the target unless `--force` is passed. The Make targets pass `--force` so subsequent runs pick up sei-internal-skills updates cleanly.

<!-- BEGIN sei-internal-skills-managed (do not edit; managed by sei-internal-skills sync scripts) -->
## Operating with sei-internal-skills resources

This package consumes portable Claude Code skills and specialist agents authored in Sei's sei-internal-skills library and installed under `.claude/`. The skills are invoked as the slash-commands below; the agents are dispatched by those skills. What follows is the opinionated doctrine for operating with them — the *way* to work, not a description of the library.

### Engineering principles

- **Interfaces first** — the primary deliverable of a design is exact signatures, types, errors, and contracts. Implementation guidance is secondary.
- **YAGNI** — only build what traces to a current-phase need. Everything else is explicitly deferred, not silently omitted.
- **Two-way doors only** — prefer reversible decisions. One-way doors (irreversible choices: persisted schema/field names, public API contracts, on-disk or wire formats, anything other systems come to depend on) require explicit human approval before finalizing.
- **Errors are interface** — every error condition is part of the public contract.
- **Provider owns the interface** — when a provider and consumer disagree, the provider's definition is canonical and consumers adapt.

### Output discipline

- **Conventional commits.** `feat:`, `fix:`, `docs:`, `refactor:` — reference the component in scope.
- **Comments & documentation.** Present-state only — never change/history/why-removed inline (that belongs in the PR/commit); sparingly; top-located (package/file/type doc, not the body); comprehensive context in one centralized doc. Champions: `idiomatic-reviewer` owns in-source comments + config annotations (`/idiomatic`); `prose-steward` owns doc artifacts + header-doc prose. The full rules live in `/idiomatic`; the boundary table between the two is in its `references/comment-discipline.md`.

### Using the skills

- **`/xreview`** — the relevant specialists independently review a design, plan, or diff, then synthesize a findings table. The review counterpart to producing the work.
- **`/root-cause`** — disciplined, data-driven, multi-expert investigation of complex problems.
- **`/idiomatic`** and **`/systems`** — review code for language/package idiom, then for systems-level quality on top. Idiom ⊂ systems quality; run them in that order.
- **`/pr-quality`** — the locked pre-PR rule gate. **`/brevity`** — tighten a PR body.

Further workflow skills — `/coral`, `/council`, `/bugbash`, `/design`, `/issue`, `/research`, `/workstream` — are **experimental** and ship only on opt-in (`make sync-experimental`). Use them when they are installed; never assume they are.

### xreview discipline

When the relevant specialists review a produced artifact (design, plan, diff, or a set of expert outputs):

- **Blinded and independent** — each reviewer commits its findings before seeing the others'; no reviewer's view is summarized into another's brief.
- **An assigned dissenter** — one reviewer is tasked to argue against the emerging consensus and surface the strongest counter-case.
- **Slate completeness** — the slate covers the domain *and* the idiom axis (`idiomatic-reviewer`) *and*, for doc artifacts, the prose axis (`prose-steward`) — not domain experts alone.
- **Automated review is co-equal** — treat an automated reviewer (e.g. Cursor Bugbot) as a peer input, not noise; an unresolved flag blocks.
- **Confirmed-consensus iteration** — after a fix, re-dispatch the reviewer that raised the finding to confirm closure; merge only on unanimous sign-off with no open concerns. `/xreview` owns the procedure.

### Key rules

- **Provider owns the interface.** Consumers adapt.
- **YAGNI.** Only features tracing to current-phase needs.
- **Errors are interface.** Every error is part of the public contract.
- **One-way-door gate.** Irreversible decisions require explicit human approval before finalizing.
- **Conventional commits.** Reference the component in scope.

### Roles, not roster

Specialists are dispatched by the workflow skills above; for a single-expert consult, use the Agent tool with the agent name as `subagent_type`. The review champions are named contracts: `idiomatic-reviewer` (code idiom, `/idiomatic`) and `prose-steward` (doc-artifact prose). The full roster of available specialists lives in the synced `.claude/agents/` files.
<!-- END sei-internal-skills-managed -->

## Writing

Prose in this repository follows the contract in
[`writing/CONTRACT.md`](writing/CONTRACT.md). It names public standards rather
than restating them, so a finding leads to a clause somebody else published.

[`writing/CONTEXT.md`](writing/CONTEXT.md) is the short form, generated from the
registry. Load that one into an agent. Read the long one yourself.

Vale checks the checkable part. Run `./writing/scripts/lint.sh` before you push,
or read `writing/README.md` for the rest.

The contract sits in its own file rather than in a section here. A sync script
writes this one from the doctrine block, and two generators writing one file is a
collision nobody wins.
