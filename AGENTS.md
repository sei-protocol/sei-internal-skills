# Tide Agent Roster

This repository hosts specialist agent personas in `.claude/agents/`. They are general-purpose and sync to other repos and user-level via `scripts/sync-agents.sh`. The skills in `.claude/skills/` (notably `/coral`, `/council`, `/cross-review`, `/root-cause`) dispatch them.

## Roster

| Agent | Category | Scope |
|-------|----------|-------|
| `kubernetes-specialist` | Portable | Go + controller-runtime, CRDs, event indexing, Job lifecycle |
| `platform-engineer` | Portable | K8s manifests, Python container runtimes, cloud auth |
| `solidity-developer` | Portable | Solidity / Foundry / OpenZeppelin / ERC standards |
| `network-specialist` | Portable | K8s and cloud networking, service mesh |
| `security-specialist` | Portable | Security + adversarial design |
| `tee-specialist` | Portable | TEE + attestation |
| `product-engineer` | Portable | Product engineering |
| `product-manager` | Portable | Product management, scope discipline |
| `go-to-market-specialist` | Portable | GTM strategy for novel products — ICP / JTBD / motion / launch. Partner to `product-manager`. |
| `platform-release-manager` | Portable | Release management and cut discipline |
| `opentelemetry-expert` | Portable | Application-side OpenTelemetry SDK instrumentation. Backend operations → `observability-platform-engineer`. |
| `observability-platform-engineer` | Portable | Telemetry backend as a system: Prometheus/Thanos/Loki/Tempo/Alloy/Promtail/Grafana operations, PromQL/LogQL authorship, mixin vendoring, ingester/compactor/store-gateway sizing. |
| `k8s-capacity-management` | Portable | Capacity as a discipline: workload right-sizing from observed data, Karpenter NodePool design, DaemonSet overhead, PriorityClass tiers, HPA/VPA/KEDA tuning, scheduling primitives. |
| `sre-engineer` | Portable | Google SRE-flavored: SLOs/SLIs, dashboards, alerts, runbooks (human + agent-callable), post-mortems. Closes the loop by filing `/issue` work when a runbook needs missing tooling. |
| `sei-network-specialist` | Sei-ecosystem | Sei node networking (seid ports, CometBFT P2P, Waterway, Istio quirks). Valuable to any Sei-adjacent work. |

The agent files themselves negotiate cross-agent boundaries (e.g. observability-platform-engineer vs. sre-engineer vs. opentelemetry-expert; k8s-capacity-management vs. platform-engineer). See each `.claude/agents/*.md` for the detailed scope and hand-off rules.

## Working Agreement

Design and review work follows the engineering principles in `CLAUDE.md`:

1. **Two-Way Doors Only** — prefer reversible decisions; one-way doors require explicit approval.
2. **YAGNI** — if it's not required by a current-phase need, exclude it (and say so).
3. **Interfaces First** — exact signatures, types, errors, contracts.
4. **Errors Are Interface** — every error condition is documented.
5. **Provider Owns the Interface** — consumers adapt.

**Output discipline.** Every agent that authors PR descriptions or in-code comments applies `/brevity` (`.claude/skills/brevity/`) before shipping a PR body or WHY-style comment. The skill self-determines when input is at floor — agents do not pre-skip.

**Pre-PR review.** Before invoking `gh pr create`, agents apply `/pr-quality` (`.claude/skills/pr-quality/`) to the staged diff + planned body. Findings surface inline for revision; suggestive only — no merge gating. Post-PR, the user may invoke `/pr-quality <PR>` to post a comment with findings.

## Install

The fastest path is `make bootstrap` from the repo root, which runs `make sync-agents`, `make sync-skills`, and `make update-agent-permissions`. See the README's Setup section for the full flow.

Agents and skills travel the same way — Tide is the canonical home, and the sync scripts push them out to user-scope (`~/.claude/`) and sibling repos.

For sibling-repo or finer-grained installs, call the scripts directly:

```bash
# Mirror portable agents to user-level (any CWD) — same as `make sync-agents`
./scripts/sync-agents.sh --target ~/

# Mirror portable skills to user-level — same as `make sync-skills`
./scripts/sync-skills.sh --target ~/

# Copy portable + sei agents to a sibling repo
./scripts/sync-agents.sh --target ~/work/platform --categories portable,sei

# Copy sei skills (chaos-suite, harbor-dev) to user-level
./scripts/sync-skills.sh --target ~/ --categories sei

# Preview without copying
./scripts/sync-agents.sh --target ~/ --dry-run
./scripts/sync-skills.sh --target ~/ --dry-run
```

Categories: `portable` (default), `sei`, `all`. Both scripts are non-destructive by default — they refuse to overwrite changed files in the target unless `--force` is passed. The Make targets pass `--force` so subsequent runs pick up Tide updates cleanly.

## How to Use These Agents

Agent personas are dispatched by the `/coral`, `/council`, `/cross-review`, and `/root-cause` skills (see `CLAUDE.md`). For direct single-expert consultations, use the Agent tool with the agent name as `subagent_type`.
