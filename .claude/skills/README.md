# Tide Project Skills

Project-scoped skills for team processes. Each subdirectory is a self-contained skill (SKILL.md + scripts + references + evals).

## First time here?

1. **Inside Tide, no setup needed.** Claude Code auto-discovers everything in this directory.
2. **Never cloned Tide? Get the full toolkit over the wire in one line** (uses your `gh` auth — Tide is internal):
   ```sh
   gh api repos/sei-protocol/Tide/contents/scripts/install.sh -H 'Accept: application/vnd.github.raw' | bash
   ```
   It clones Tide to `~/.tide` (override `TIDE_HOME`), then syncs every portable + Sei skill and agent into `~/.claude`.
3. **Already have the repo?** One command from your checkout:
   ```sh
   make update     # fast-forward this checkout (run from main) + sync ALL skills/agents into ~/.claude + verify
   ```
   (To install only the **portable** set into an external *consumer* repo, run `make bootstrap` instead.)
4. **Re-run either any time** — both are idempotent.

**Edit skills in Tide, never in `~/.claude/skills/`.** Local edits at user-scope get overwritten on next sync. To change a skill, edit it here and PR.

---

**Authoring standard:** read [`SKILL-TEMPLATE.md`](./SKILL-TEMPLATE.md) before creating a new skill.

Claude Code discovers skills as **flat** direct subdirectories of `skills/` — nested folders and custom roots (e.g. `~/.claude/tide/`) are NOT discovered. So domain grouping is **metadata, not directories**. The **single source of truth** is each skill's `category:` SKILL.md frontmatter: the sync scripts *derive* alias membership from it (no hand-maintained per-skill list), and `make verify-catalog` (CI) fails closed if any skill's category maps to no alias. The catalog sections below are descriptive — keep them in step with the skills present, but they are not what the sync reads.

**Domains:** `workflow` · `workstream-bootstrap` · `hardening` · `investigation` · `skill-authoring` · `code-quality` · `performance` · `writing-quality` · `output-quality` (Tide-local) · `security` (Tide-local) · `product-management` · `project-management` · `release-operations` · `engineer-self-service`. The small domain→alias map at the top of `sync-skills.sh` assigns each domain to a sync alias: `portable`, `sei`, or Tide-local (never synced).

## Catalog

### Workflow

Edit these in Tide, never in `~/.claude/skills/` — your edits will be overwritten on next sync. To use them outside Tide, run:

```sh
./scripts/sync-skills.sh
```

- **`coral/`** — Lightweight expert iteration. Knows about the `/issue` handoff (offers to bootstrap deferred slices and end-of-session phase 2 as a tracked issue) and offers `/cross-review` at synthesis when specialist outputs touch a shared boundary.
- **`cross-review/`** — Standalone cross-review action between the orchestrator and the coral/council experts. Dispatches the relevant specialists to **independently** review a produced artifact (design, plan, diff, or set of expert outputs), then synthesizes a COMPATIBLE / MISMATCH / MISSING findings table. Enforces blinded review + an assigned dissenter + evidence-bearing findings to defeat rubber-stamping and consensus theater. The review counterpart to coral's "produce"; `/council` invokes it as its cross-review phase.
- **`council/`** — Full-ceremony multi-component design and scope-tier selection. The heavier sibling of coral; delegates its cross-review phase to `/cross-review`. Teammates will mostly use coral, but council ships alongside so the coral → council handoff works from anywhere.
- **`workstream/`** — Launch and govern a substantial workstream on top of the Coral stack with **declared, enforced human checkpoints**. Composes the lifecycle (`/council` scope-tier → `/cross-review` → `/design` capture → `/issue` → `/execution-plan`) and inserts named human gates (`design-approval`, `pr-sign-off`, custom) the agent must surface and obtain explicit confirmation for before proceeding — gates that survive "keep going" pressure, including a `/goal` Stop hook (the hook governs *stopping*, not *approval*). Layers on the `/goal` mechanism; composes the workflow skills, never edits them. Distinct from `/coral` (single slice) and `/council` (scope-tiered design, which workstream invokes).

### Workstream Bootstrap
Two complementary artifact-capture skills. Coral / council should offer them at handoff moments — `/issue` for **next** work, `/design` for **this** work's design pass. Both pre-fill from session context; user reviews and confirms.

- **`issue/`** — Synthesize the current session into a standard-format issue that bootstraps the next pickup, filed to **GitHub or Linear** (asked at the create step). Required body sections: Problem, Impact, Relevant experts. Fires when a deferred slice surfaces, the user cuts scope, or the session closes with an obvious phase 2.
- **`design/`** — Capture the current session's design as a markdown doc under `docs/designs/` (or repo-specific path; Tide → `design/milestones/` or `design/high-level/`). ADR-flavored body with mermaid diagrams encouraged. Threads bidirectional lineage to the source issue (frontmatter `Issue: #n` forward; offers to update issue's References reverse). Fires when the deliverable IS a design (LLD, architecture sketch, system-tier decision).
### Product Management
Product-decision discipline before engineering scoping. Pairs with the `go-to-market-specialist` agent (same domain).

- **`prfaq/`** — Author or review a PRFAQ (Amazon working-backwards Press Release + FAQ) before greenlighting a product/feature/initiative. Forces customer-thesis discipline (named customer, named pain, named alternatives, falsification thresholds). Refuses theater: buzzword soup, customer-absent prose, FAQ-as-marketing, polished perfectionism over thinking. Three modes: Author / Review / Verdict. Companion to `/design` (capture this design) and `/issue` (capture next workstream).

### Skill Authoring & Auditing
- **`author-skill/`** — Author a new skill for a specific domain. Drives Intake → deep web research (parallel subagents) → subagent-based RED-GREEN-REFACTOR pressure testing → scaffolds the skill into `<repo>/.claude/skills/<name>/` per `SKILL-TEMPLATE.md`, with evals derived from the surviving pressure scenarios. Built on Anthropic's skill-authoring best practices and Obra's TDD-for-skills methodology (`references/obra-best-practices.md`, `references/testing-with-subagents.md`, `references/persuasion-principles.md`). Refuses to overwrite canonical workflow skills (coral, council, design, issue, bugbash) or skip the RED baseline.
- **`audit-skill/`** — Audit an existing skill against the team's conventions catalog (`references/conventions-catalog.md`). Two phases: **audit-only** is the default — runs static + semantic + pressure checks and produces a findings report at `docs/skill-audits/<skill>-<date>.md`, no edits made. **Refactor** is opt-in via `--apply` — per-finding diffs, diff-before-write gate, append-only evals, automatic rollback on verify failure. Canonical skills are auditable freely; refactor requires `--override-protected`. The brown-field sibling of `/author-skill`.

### Code Quality
Language- and framework-idiom conformance review. Pairs with the `idiomatic-reviewer` agent (same domain) — the skill is the machinery, the agent is the standing review lens.

- **`idiomatic/`** — Review and refine code so it reads native to its language, framework, and the package's own established patterns. Digests the repo's agent files (CLAUDE.md/AGENTS.md) + the package's `doc.go` into a local idiom profile that **outranks** generic textbook idiom, then overlays a pluggable per-language idiom pack (`references/language-pack-<lang>.md`, one file per language — Go, Rust, TypeScript, Solidity, Bash, Python — written against `language-pack-TEMPLATE.md`). Two-altitude output — design-level and surgical line-level, each citing its basis. Discipline spine pressure-tested with subagents: profile-first gate, local-profile-overrides-generic (including documented exceptions), cite-every-finding, and a false-positive guard. Includes a package data-structure documentation standard (`references/datastructure-standard.md`, modeled on sei-k8s-controller's planner→executor→task `doc.go`). Distinct from `/code-review` (correctness), `/cross-review` (boundary consistency), and `/pr-quality` (locked rule gate) — durable idiom findings can graduate into the pr-quality registry.
- **`systems/`** — Review/design code & architecture for **systems-level quality**: reliability, observability, performance, safety, API durability. A citable standards corpus grounded in the open canon (Google SRE, AWS Builder's Library, OTel semconv, TigerBeetle TIGER STYLE, NASA Power of Ten, Google AIP, …) split one-level-deep by theme, that the `systems-engineer` agent hooks into (also invocable as `/systems-review`). Findings ranked by **consequence-under-load**, each cited; discipline spine = consequence-ranking · cite-everything/copyright-clean · don't-duplicate-the-idiom-or-ops-lens. Idiom ⊂ systems quality: run `/idiomatic` first, `/systems` on top. Distinct from `/code-review` (correctness), `/cross-review` (boundary consistency), and the ops agents (operating the running system).
- **`ebpf/`** — Design and run **kernel-level observability and performance benchmarks** with eBPF/bpftrace — off-CPU/blocked time, lock/futex contention, syscall + block-I/O latency *distributions*, run-queue latency, TCP-level health: what pod/Prometheus telemetry structurally can't show. Technique-with-spine (mirrors `/idiomatic`/`/systems`), composed by the `systems-engineer` agent; pluggable packs (MVP: `pack-perf-methodology` — USE method, on/off-CPU, the tool→signal→concern map, the verifier-forced-idiom `divergences[]`, the §verify-anchor table). Discipline spine pressure-tested: measure-don't-assume (retrieved signal, not a guess) · overhead-bound-before-attach · open-loop-for-tail (coordinated-omission guard) · eBPF-complements-not-replaces-pprof · no-privileged-deploy-without-the-one-way-door-gate. Distinct from `opentelemetry-expert` (in-process SDK), `observability-platform-engineer` (telemetry backend/queries), `sre-engineer` (SLO/alert/runbook), `/systems` (app/architecture where eBPF is just the diagnostic), `network-specialist` (NetworkPolicy intent vs. the Cilium datapath). Capability design: PLT-649.

### Writing Quality
Dual-audience (human ↔ AI agent) prose conformance. Pairs with the `prose-steward` agent (PLT-480, same domain) — the skill is the doctrine + machinery, the agent is the standing review lens.

- **`lingua/`** — Translate an org artifact (design doc/HLD, PRD, 1-pager) so it reads correctly for **both** audiences: the human who scans (NN/g F-pattern, working-memory limits) and the AI agent that ingests linearly and acts on what it reads. Mirrors `/idiomatic`'s mechanism — a cited `references/audience-model.md` (rules R1–R6 with basis tiers: **Cited** may surface as findings; **Stated-opinion** is advisory-only, never blocking, each with a falsification line) + pluggable artifact packs (`hld`, `lld`, `1pager`; PRD deferred) grounded in a license-verified exemplar corpus (Rust/Go/Ethereum/Linux design docs; US-gov public-domain BLUF doctrine) + a repo-profile overlay (`CLAUDE.md` "Writing conventions") that **outranks** the generic doctrine and can establish exceptions. Discipline spine pressure-tested with subagents: two-part gate (no doctrine → no output; profile read first), fidelity (never invent commitments), typed ambiguity (soft modals become Open questions, never promoted to requirements), advisory-only (no agent-parsed format — council-gated one-way door). Grounded by the exemplar corpus under `references/exemplars/` (cite contract from PLT-478). One mode (Translate); Compose/Review deferred — review arrives via `prose-steward`. Distinct from `/idiomatic` (code idiom), `/prfaq` (the PRFAQ vertical), `/systems` (runtime quality), `/brevity` (PR-body tightening).

### Security
TEE attestation design + verification review. Pairs with the `tee-specialist` agent (same domain) — the skill is the machinery (method + pluggable platform kits), the agent is the standing SME dispatched into workstreams, designs, and research.

- **`tee/`** — Design and review **Trusted Execution Environment** integrations — attestation flows, on-chain verification of enclave identity, attestation-conditioned key release — grounded in vendor specs + the Sei deployment profile, not paraphrased generality. Mirrors `/idiomatic`'s mechanism: a vendor-agnostic `references/method.md` (RATS roles, the Sei on-chain cost ranking, the cross-cutting verifier-policy dimensions VP1–VP16, the severity model) + an always-first `tee-profile.md` Sei overlay + pluggable per-platform **kits** (`kit-aws-nitro`, `kit-intel-sgx-tdx`, `kit-amd-sev-snp`, `kit-nvidia-cc`, `kit-tpm-rats`, `kit-sei-onchain`; each conforms to `kit-TEMPLATE.md` and **cites** the `design/research/tee/*` ground truth rather than paraphrasing it — adding a platform is one conforming file). Discipline spine pressure-tested with subagents: claim+trust-model gate · profile/kit-override-generic-vendor-knowledge (incl. the validator-as-host hard direction) · cite-every-vendor-claim / never-fabricate-from-memory · one-way-doors-flagged · trust-model-honesty. Distinct from `security-specialist` (general non-TEE threat modeling) and the consuming-system specialists (`solidity-developer`, `kubernetes-specialist`) that build what the attestation gates. **Tide-local** (its `design/research/tee/*` corpus lives here) — not synced until the corpus is portable. Refactor: PLT-677.

### Hardening
- **`bugbash/`** — Long-running, read-only adversarial review of an existing system by the council of experts. Loops discovery + challenger passes against a named target (`/bugbash SeiNode controller`) until the experts converge on a launch verdict. Output is a structured findings log at `docs/bugbash/<target>.md` with per-item Scenario / Impact / Issue / Fix sketch / Test coverage. Inspired by the [RALPHY loop](https://github.com/snarktank/ralph), reframed for hardening before launch. Distinct from `/security-review` (single-pass, security-only) and `/coral` (collaborative iteration, not adversarial).

### Investigation
- **`root-cause/`** — Disciplined, data-driven, multi-expert investigation of complex problems in the Sei platform stack (sei-k8s-controller, seictl, sei-sidecar, sei-chain, release-test/qa-testing, platform/K8s). Forces signals before hypotheses, ≥2 competing hypotheses before evidence, retrieved provenance (not paraphrased), and falsification before conclusion. Dispatches `.claude/agents/` specialists in **parallel + blinded + with assigned dissent** to prevent the consensus-theater / sycophancy failure mode documented in the multi-agent LLM literature. Output is a multi-cause ranked conclusion — never a single root cause. Distinct from `/bugbash` (pre-launch adversarial), `/coral` (collaborative iteration), and live incident command (mitigate first; this skill is for understanding). Problems outside the Sei platform stack are out of scope.
- **`research/`** — First-class research: scope a question (the decision it informs + falsifiable claims), run a **multi-modal sweep** (angles blind to each other), **adversarially verify** every finding (reusing `/cross-review`'s assigned-dissent primitive — no finding ships unverified), run one completeness pass, and capture a durable **findings artifact** at `design/research/<slug>.md` threaded to issues/bets via `/execution-plan`. Discipline spine = no-finding-ships-unverified · refuse-a-vague-question · discover-don't-decide. Distinct from `/root-cause` (Sei-stack incident investigation), `/design` (decisions, not findings), and `/coral` (iteration). Generalizes `author-skill`'s research recipe; a research effort may be checkpoint-gated by `/workstream` but never launches one.

### Authoring Discipline (Tide-local — not synced)

These two are project-scoped disciplines applied during authoring inside Tide. They are intentionally **not** in any sync category (CLAUDE.md / AGENTS.md reference them as in-repo disciplines):

- **`brevity/`** — Tighten agent-produced PR descriptions and in-code comments before they ship. Self-determines floor; agents don't pre-skip.
- **`pr-quality/`** — Pre-PR review of the staged diff + planned body (verbosity via `/brevity` dispatch + convention rules). Suggestive only; never gates merge.

### Release Operations
- **`chaos-suite/`** — Execute the full chaos test suite (runbook: sei-protocol/platform#169) against a dev or staging Sei cluster and collate results into a release summary. **Status: scaffold** — follows the template; scripts are placeholders pending authoring against the live runbook. Tracking issue: sei-protocol/platform#170.
- **`validate-release/`** — Collect a completed chaos-suite run's results from S3 + Thanos/Grafana, derive per-scenario metrics and panel PNGs, and push a structured release-validation report to Notion. Companion to `/chaos-suite` (run) → `/validate-release` (report).
- **`gov-ops/`** — Orchestrate a Sei governance proposal lifecycle (submit → confirm → vote → verify) on a target chain, GitOps-native, with **fail-closed safety gates**: a positive `(context, network, namespace)` allowlist that refuses any mainnet-co-hosting context, verbatim `confirm` before each irreversible act, blocking value-shape / deposit / fee-floor / resolved-id gates, and an active code-13 / tally-stall detector. Param-change only; rollback is a referenced runbook. Consumed by the `platform-release-manager` agent; cites `sei-protocol/bdchatham-designs designs/seinode-task/seinode-task.md` for operational facts. NOT for chain spin-up (`/harbor-dev`), release validation (`/validate-release`), or deciding *what* to change.

### Project Management (Impact Hub)
- **`execution-plan/`** — The mechanism substrate of the agentic-PM loop: decorate and read the `bet↔design↔issue↔PR` graph (stamp a Linear issue to its bet's `impact:<slug>` label + design link, the `betGraph` read contract, `reconcile`/backfill lineage) so `/issue`, `/design`, `/impact-weekly`, `/impact-portfolio`, and the `technical-program-manager` agent share **one** identity/label/cache implementation. Identity = bet Notion page-ID; the plan is a *derived set* (no plan object); decoration on the team's own issues is automatic, with confirm-gates only on first label-create + any Notion write; no lifecycle/exec writes; drift is human-resolved. Procedural, guardrails-first; backs the `technical-program-manager` agent. Sei/Impact-Hub-scoped.
- **`impact-weekly/`** — Roll up an engineer's Linear week (+ linked PRs) into the matching Impact Hub bet as a substantiated, executive-summary Weekly-log entry, draft→confirm→write. The producer in the work loop in `docs/designs/impact-hub-pm-skill-suite.md`; failure modes (mis-tracking, bloat, unsubstantiated claims) are engineered as refusals.
- **`impact-portfolio/`** — The weekly cross-project executive report: one human-confirmed Notion page per week under the Impact Hub's Weekly Reports (exec summary + per-project sections with owner, Overall Confidence, ≤3 substantiated bullets). Reads the week's per-bet Weekly-log toggles (+ a Linear `impact:<slug>` activity scan); read-only on bets, writes only its own report page. The reader-facing synthesis tail; design at `docs/designs/impact-portfolio-weekly-report.md`. (`impact-eoq`, the per-engineer quarter rollup, is the remaining deferred phase-2 sibling.)

### Engineer Self-Service
- **`harbor-dev/`** — Engineer-facing interface to the harbor EKS cluster. Translates natural-language intent (spin up an ephemeral chain, attach an RPC fleet, run a bench, onboard me, tear it down) into `seictl nd` invocations and PR-based GitOps deliveries against `sei-protocol/harbor-engineering-workspace`. Built on `seictl` v0.0.43+.

### Future Slots
- _(planned)_ Add skills here as the team codifies more processes.

## Adding a New Skill

1. Read [`SKILL-TEMPLATE.md`](./SKILL-TEMPLATE.md).
2. Draft the guardrails stanza FIRST. If you can't articulate what the skill refuses to do, it isn't ready to author.
3. Scaffold the directory structure from the template.
4. Add an entry to the catalog above under the appropriate section.
5. Make sure `state/` is gitignored (the repo-level `.gitignore` already covers `.claude/skills/*/state/`).
6. Pre-approve the skill's happy-path permissions in `.claude/settings.json` or `.claude/settings.local.json`.

## Cross-Repo Skills

A project-scope skill in this repo is only discoverable when Claude Code is running with this repo as CWD. To make a skill discoverable elsewhere, sync it out:

```sh
./scripts/sync-skills.sh                    # daily: portable skills → ~/.claude/skills/
./scripts/sync-skills.sh --categories all                 # also sync the sei-team skills
./scripts/sync-skills.sh --categories project-management  # just one domain
./scripts/sync-skills.sh --target ~/work/sei-k8s-controller --force  # to another repo
```

If a tracked file in the target differs from Tide's version, the skill is reported as a conflict and skipped — re-run with `--force` to overwrite. Target-only files (user customizations, runtime artifacts) are preserved.

Sibling of `scripts/sync-agents.sh` — same shape, same flags. Sync by **domain** (`--categories project-management`, `--categories workflow`, …) or by **alias**: `portable` (the general-purpose skill set, incl. code-quality + writing-quality), `sei` (the 5 Sei-team skills: impact-weekly, impact-portfolio, chaos-suite, validate-release, harbor-dev), `all`. `output-quality` (brevity, pr-quality) is Tide-local and not synced. Update the domain lists in the script when a skill is added, renamed, or re-categorized.

For procedural skills like `chaos-suite` that operate on remote infrastructure, you can also just run them from Tide and pass `--repo` / target paths to direct work elsewhere — no sync needed.
