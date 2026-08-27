# sei-internal-skills Project Skills

Project-scoped skills for team processes. Each subdirectory is a self-contained skill (SKILL.md + scripts + references + evals).

## First time here?

1. **Inside sei-internal-skills, no setup needed.** Claude Code auto-discovers everything in this directory.
2. **Never cloned sei-internal-skills? Get the full toolkit over the wire in one line** (uses your `gh` auth — sei-internal-skills is internal):
   ```sh
   gh api repos/sei-protocol/sei-internal-skills/contents/scripts/install.sh -H 'Accept: application/vnd.github.raw' | bash
   ```
   It clones sei-internal-skills to `~/.sei-internal-skills` (override `SEI_INTERNAL_SKILLS_HOME`), then syncs every **core** skill and agent (portable + Sei) plus the output styles into `~/.claude`. Nothing under `experimental/` is installed.
3. **Already have the repo?** One command from your checkout:
   ```sh
   make update     # fast-forward this checkout (run from main) + sync the core skills/agents/output-styles into ~/.claude + verify
   ```
   (To install only the **portable** set into an external *consumer* repo, run `make bootstrap` instead.)
4. **Re-run either any time** — both are idempotent.

**Edit skills in sei-internal-skills, never in `~/.claude/skills/`.** Local edits at user-scope get overwritten on next sync. To change a skill, edit it here and PR.

---

**Authoring standard:** read [`SKILL-TEMPLATE.md`](./SKILL-TEMPLATE.md) before creating a new skill.

Claude Code discovers skills as **flat** direct subdirectories of `skills/` — nested folders and custom roots (e.g. `~/.claude/sei-internal-skills/`) are NOT discovered. So domain grouping is **metadata, not directories**. The **single source of truth** is each skill's `category:` SKILL.md frontmatter: the sync scripts *derive* alias membership from it (no hand-maintained per-skill list), and `make verify-catalog` (CI) fails closed if any skill's category maps to no alias. The catalog sections below are descriptive — keep them in step with the skills present, but they are not what the sync reads.

**Domains in the core:** `workflow` · `investigation` · `skill-authoring` · `code-quality` · `platform-infra` · `blockchain` · `output-quality` (sei-internal-skills-local) · `release-operations` · `engineer-self-service`. The small domain→alias map at the top of `sync-skills.sh` assigns each domain to a sync alias: `portable`, `sei`, or sei-internal-skills-local (never synced). The map still carries the domains used only by `experimental/` skills, so parking and promoting a skill needs no map change.

## Catalog

### Workflow

Edit these in sei-internal-skills, never in `~/.claude/skills/` — your edits will be overwritten on next sync. To use them outside sei-internal-skills, run:

```sh
./scripts/sync-skills.sh
```

- **`xreview/`** — Standalone xreview action between the orchestrator and the coral/council experts. Dispatches the relevant specialists to **independently** review a produced artifact (design, plan, diff, or set of expert outputs), then synthesizes a COMPATIBLE / MISMATCH / MISSING findings table. Enforces blinded review + an assigned dissenter + evidence-bearing findings to defeat rubber-stamping and consensus theater. The review counterpart to coral's "produce"; `/council` invokes it as its xreview phase.

### Skill Authoring & Auditing
- **`author-skill/`** — Author a new skill for a specific domain. Drives Intake → deep web research (parallel subagents) → subagent-based RED-GREEN-REFACTOR pressure testing → scaffolds the skill per `SKILL-TEMPLATE.md` — into `experimental/skills/<name>/` by default, or `<repo>/.claude/skills/<name>/` when the tier gate is cleared — with evals derived from the surviving pressure scenarios. Built on Anthropic's skill-authoring best practices and Obra's TDD-for-skills methodology (`references/obra-best-practices.md`, `references/testing-with-subagents.md`, `references/persuasion-principles.md`). Refuses to overwrite canonical workflow skills (coral, council, design, issue, bugbash) or skip the RED baseline.
- **`audit-skill/`** — Audit an existing skill against the team's conventions catalog (`references/conventions-catalog.md`). Two phases: **audit-only** is the default — runs static + semantic + pressure checks and produces a findings report in the DRI's `<engineer>-designs` repo at `designs/<arc>/audits/<skill>-<date>.md` (Design 13; in-repo `docs/skill-audits/` only when no DRI repo and the user confirms), no edits made. **Refactor** is opt-in via `--apply` — per-finding diffs, diff-before-write gate, append-only evals, automatic rollback on verify failure. Canonical skills are auditable freely; refactor requires `--override-protected`. The brown-field sibling of `/author-skill`.

### Code Quality
Language- and framework-idiom conformance review. Pairs with the `idiomatic-reviewer` agent (same domain) — the skill is the machinery, the agent is the standing review lens.

- **`idiomatic/`** — Review and refine code so it reads native to its language, framework, and the package's own established patterns. Digests the repo's agent files (CLAUDE.md/AGENTS.md) + the package's `doc.go` into a local idiom profile that **outranks** generic textbook idiom, then overlays a pluggable per-language idiom pack (`references/language-pack-<lang>.md`, one file per language — Go, Rust, TypeScript, Solidity, Bash, Python — written against `language-pack-TEMPLATE.md`). Two-altitude output — design-level and surgical line-level, each citing its basis. Discipline spine pressure-tested with subagents: profile-first gate, local-profile-overrides-generic (including documented exceptions), cite-every-finding, and a false-positive guard. Includes a package data-structure documentation standard (`references/datastructure-standard.md`, modeled on sei-k8s-controller's planner→executor→task `doc.go`). Distinct from `/code-review` (correctness), `/xreview` (boundary consistency), and `/pr-quality` (locked rule gate) — durable idiom findings can graduate into the pr-quality registry.
- **`systems/`** — Review/design code & architecture for **systems-level quality**: reliability, observability, performance, safety, API durability. A citable standards corpus grounded in the open canon (Google SRE, AWS Builder's Library, OTel semconv, TigerBeetle TIGER STYLE, NASA Power of Ten, Google AIP, …) split one-level-deep by theme, that the `systems-engineer` agent hooks into (also invocable as `/systems-review`). Findings ranked by **consequence-under-load**, each cited; discipline spine = consequence-ranking · cite-everything/copyright-clean · don't-duplicate-the-idiom-or-ops-lens. Idiom ⊂ systems quality: run `/idiomatic` first, `/systems` on top. Distinct from `/code-review` (correctness), `/xreview` (boundary consistency), and the ops agents (operating the running system).

### Platform Infrastructure
Operator/controller and platform-engineering knowledge, grounded in Sei's actual architecture (sei-k8s-controller, the platform GitOps fleet). Pairs with the `kubernetes-specialist` + `platform-engineer` agents (same domain) — the skill is the machinery, the agent is the standing build/review lens.

- **`kubernetes/`** — Design and review Kubernetes **operator/controller** code (CRDs, reconcilers, controller-runtime/kubebuilder) — grounded in the upstream canon (K8s API conventions, controller-runtime, CRD versioning) and an **always-first Sei-controller profile** distilling sei-k8s-controller's enforced conventions (plan-driven reconcile, optimistic-lock single-patch status, always-present conditions/reason-as-API, CEL immutability one-way-doors, the `kubectl wait` latch). Method + 5 review dimensions + pluggable kits (`plan-driven-reconciliation`, `sidecar-task-integration`, `crd-design`; more deferred). Backs `kubernetes-specialist`. Distinct from `/idiomatic` (Go idiom), `k8s-capacity-management` (right-sizing/scheduling), `/platform` (manifests/GitOps/cloud-auth), `sei-network-specialist` (node P2P/RPC).
- **`platform/`** — Design and review the **platform layer** — Kustomize manifests, Flux GitOps, EKS cloud-auth, secrets, Pod Security, terraform — grounded in the external canon (OpenGitOps, Kustomize, Pod Security Standards, NSA/CISA hardening) and an **always-first Sei-platform profile** distilling the fleet's real conventions: **Flux GitOps**, two-layer Kustomize (`clusters/base`+`manifests/base` via patches/components/replacements, not `postBuild.substitute`), **EKS Pod Identity** default (IRSA = documented old-SDK exception), **SOPS-in-git + per-cell KMS** delivery (not CSI/ESO/Sealed), PSS-`restricted` + CEL VAP, Cilium/VPC-CNI. Method + 6 review dimensions + pluggable kits (`gitops-flux`, `kustomize-composition`, `cloud-auth-pod-identity`, `secrets-sops-kms`, `pod-security-vap`; more deferred). Backs `platform-engineer`. Distinct from `/kubernetes` (controller code), `k8s-capacity-management` (right-sizing), the observability agents (telemetry values/PromQL), `network-specialist` (NP intent/datapath), `sre-engineer` (operating the system).

### Blockchain
EVM smart-contract engineering on Sei. Pairs with the `solidity-developer` agent (same domain) — the skill is the machinery, the agent is the standing build/review lens.

- **`evm/`** — Design and review **EVM smart contracts for Sei** — Solidity/Foundry contracts, precompile integration, gas/parity assumptions, upgrade safety, and on-chain event indexing for agentic consumers — grounded in the external canon (Solidity, OpenZeppelin v5, Foundry, EEA EthTrust v3, EIP-1967/7201) and an **always-first Sei-EVM profile** distilling Sei's real execution-environment facts that override generic L1 habit: **Pectra-no-blobs** on a go-ethereum fork, **instant finality / no pending state**, **governance-mutable gas** (estimate at runtime), **`block.prevrandao` is not randomness**, **IAVL-not-MPT proofs**, the 13 **precompiles** + their `usei`/`wei` decimal traps, the **dual 0x↔bech32 address + association**, and **cross-VM logs bloom-filtered out of EVM logs** (the on-chain-event-receipt trap). Method + 6 review dimensions + pluggable kits (`sei-precompiles`, `evm-parity-gas`, `address-association`, `foundry-tooling`, `upgrade-safety`, `evm-indexing-events`, `randomness-vrf`, `delegated-authority` — ERC-7710/7715 caveat delegation on ERC-4337 for scoped/revocable cross-org agent access; more deferred). Backs `solidity-developer`. Distinct from `/idiomatic` (Solidity idiom/lint), `security-specialist` (deep exploit audit / severity), `sei-network-specialist` (node P2P/RPC).


### Investigation
- **`root-cause/`** — Disciplined, data-driven, multi-expert investigation of complex problems in the Sei platform stack (sei-k8s-controller, seictl, sei-sidecar, sei-chain, release-test/qa-testing, platform/K8s). Forces signals before hypotheses, ≥2 competing hypotheses before evidence, retrieved provenance (not paraphrased), and falsification before conclusion. Dispatches `.claude/agents/` specialists in **parallel + blinded + with assigned dissent** to prevent the consensus-theater / sycophancy failure mode documented in the multi-agent LLM literature. Output is a multi-cause ranked conclusion — never a single root cause. Distinct from `/bugbash` (pre-launch adversarial), `/coral` (collaborative iteration), and live incident command (mitigate first; this skill is for understanding). Problems outside the Sei platform stack are out of scope.

### Authoring Discipline (sei-internal-skills-local — not synced)

These two are project-scoped disciplines applied during authoring inside sei-internal-skills. They are intentionally **not** in any sync category (CLAUDE.md / AGENTS.md reference them as in-repo disciplines):

- **`brevity/`** — Tighten agent-produced PR descriptions and in-code comments before they ship. Self-determines floor; agents don't pre-skip.
- **`pr-quality/`** — Pre-PR review of the staged diff + planned body (verbosity via `/brevity` dispatch + convention rules). Suggestive only; never gates merge.

### Release Operations
- **`chaos-suite/`** — Execute the full chaos test suite (runbook: sei-protocol/platform#169) against a dev or staging Sei cluster and collate results into a release summary. **Status: scaffold** — follows the template; scripts are placeholders pending authoring against the live runbook. Tracking issue: sei-protocol/platform#170.
- **`validate-release/`** — Turn a real nightly chaos run into a **liveness** release report on Notion: raw harbor Prometheus metrics (federated `prometheus-prod` datasource) for the per-scenario story + the harness Job (spec env for the release image, pod-log for the authoritative PASS/FAIL verdict), with panel PNGs embedded. No S3/`report.json` source (that pipeline was removed); leads with `LIVENESS GO`/`NO-GO`, never fabricates a verdict.
- **`gov-ops/`** — Orchestrate a Sei governance proposal lifecycle (submit → confirm → vote → verify) on a target chain, GitOps-native, with **fail-closed safety gates**: a positive `(context, network, namespace)` allowlist that refuses any mainnet-co-hosting context, verbatim `confirm` before each irreversible act, blocking value-shape / deposit / fee-floor / resolved-id gates, and an active code-13 / tally-stall detector. Param-change only; rollback is a referenced runbook. Consumed by the `platform-release-manager` agent; cites `sei-protocol/bdchatham-designs designs/seinode-task/seinode-task.md` for operational facts. NOT for chain spin-up (`/harbor-dev`), release validation (`/validate-release`), or deciding *what* to change.
- **`validator-platform/`** — The **knowledge layer** behind operating K8s-managed Sei validators: how to submit governance proposals and per-node votes via per-node `SeiNodeTask` manifests, grounded at pins in the controller (`@5730aa4`) + seictl (`@79f74a5`). An **always-first Sei-validator profile** (SeiNetwork→SeiNode→SeiNodeTask topology, seictl-sidecar execution at `:8443`, idempotency-per-kind, keyring-resolution ladder, `requirePhase` terminality, structural RPC pin) + 5 review dimensions + pluggable kits (`platform-machinery`, `seinodetask-gov-manifests`, `shadow-comparison`; `gitops-networking` deferred to the M2 `/harbor-dev` refresh). **Cites, never restates** `/gov-ops` (gates/fan-out/fee-floor), `/kubernetes` (controller-author view), and the seinode-task LLD (flagged stale on topology). Backs `platform-release-manager`. NOT orchestration (`/gov-ops`), controller code (`/kubernetes`), or platform GitOps infra (`/platform`).

### Engineer Self-Service
- **`harbor-dev/`** — Engineer-facing interface to the harbor EKS cluster. Translates natural-language intent (spin up an ephemeral chain, attach an RPC fleet, run a bench, onboard me, tear it down) into `seictl network` / `seictl node` invocations (the post-cutover **SeiNetwork + SeiNode** model; `SeiNodeDeployment` removed) and PR-based GitOps deliveries against `sei-protocol/harbor-engineering-workspace`, with networking orchestrated outside the controller (engineer-owned `HTTPRoute`s + a load balancer when a use-case needs external access). A third tree, `seictl workflow`, is a separate imperative path — outside the PR/Flux flow — for re-bootstrapping or migrating an *existing* node in place; it is the skill's one destructive verb and is gated on explicit engineer sign-off. Built on `seictl` v0.0.59+ (`network`/`node`); `workflow` ships in a later release — see `harbor-dev/SKILL.md` gate 1.

### Future Slots
- _(planned)_ Add skills here as the team codifies more processes.

## Not in the core

The catalog above is the **shipped core** — what `make update` installs. Two other
tiers exist:

- **[`experimental/`](../../experimental/README.md)** — parked skills and agents:
  workflow orchestration (`coral`, `council`, `workstream`, `issue`, `design`,
  `research`), deep-dive engineering (`ebpf`, `bugbash`), `interview`, and
  `project-brief`. Nothing there syncs by default; opt in with `make sync-experimental`.
- **The archive** — resources cut entirely in the 2026-08 slim-down (`data-mesh`,
  `prfaq`, `tee`, `diagram`) live with full history in
  [`bdchatham/sei-internal-skills-archive`](https://github.com/bdchatham/sei-internal-skills-archive).

Some descriptions above still name a parked skill in an anti-trigger or a
"see also" — those pointers remain accurate, since the skill still lives in this
repo under `experimental/`.

## Adding a New Skill

1. **Pick the tier first.** A new skill goes in [`experimental/skills/`](../../experimental/README.md) unless you can say why it belongs in the core. The core is what every teammate installs, so each addition costs everyone the effort of filtering past it. `experimental/` costs nobody anything.

   It belongs in the **core** when an engineering team outside its author would reach for it on ordinary work, *and* it is stable enough that changing it is a considered act. Anything else — still forming, narrow audience, exploratory — starts in `experimental/`. Promotion later is one `git mv`.

   Skipping this step is how the catalog reached 33 skills before the 2026-08 slim-down cut it to 17.

2. Read [`SKILL-TEMPLATE.md`](./SKILL-TEMPLATE.md).
3. Draft the guardrails stanza FIRST. If you can't articulate what the skill refuses to do, it isn't ready to author.
4. Scaffold the directory structure from the template, under the tier you picked.
5. Catalog it: a core skill gets an entry in the catalog above, under the appropriate section; an experimental skill gets a row in [`experimental/README.md`](../../experimental/README.md).
6. Make sure `state/` is gitignored. The repo-level `.gitignore` covers both tiers — `.claude/skills/*/state/` and `experimental/skills/*/state/`.
7. Pre-approve the skill's happy-path permissions in `.claude/settings.json` or `.claude/settings.local.json`.

Only a **core** skill needs a `category:` that maps to a sync alias — `make verify-catalog` enforces that, and it only reads `.claude/skills/`. An experimental skill keeps its `category:` frontmatter (so promotion needs no edit), but nothing checks it while it is parked.

## Cross-Repo Skills

A project-scope skill in this repo is only discoverable when Claude Code is running with this repo as CWD. To make a skill discoverable elsewhere, sync it out:

```sh
./scripts/sync-skills.sh                    # daily: portable skills → ~/.claude/skills/
./scripts/sync-skills.sh --categories all                 # also sync the sei-team skills
./scripts/sync-skills.sh --categories code-quality        # just one domain
./scripts/sync-skills.sh --target ~/work/sei-k8s-controller --force  # to another repo
```

If a tracked file in the target differs from sei-internal-skills's version, the skill is reported as a conflict and skipped — re-run with `--force` to overwrite. Target-only files (user customizations, runtime artifacts) are preserved.

Sibling of `scripts/sync-agents.sh` — same shape, same flags. Sync by **domain** (`--categories code-quality`, `--categories workflow`, …) or by **alias**: `portable` (the general-purpose skill set), `sei` (the Sei-team skills: chaos-suite, validate-release, harbor-dev), `all`. `output-quality` (brevity, pr-quality) is sei-internal-skills-local and not synced. Skills under `experimental/` are outside every alias — see [`experimental/README.md`](../../experimental/README.md). Update the domain lists in the script when a skill is added, renamed, or re-categorized.

For procedural skills like `chaos-suite` that operate on remote infrastructure, you can also just run them from sei-internal-skills and pass `--repo` / target paths to direct work elsewhere — no sync needed.
