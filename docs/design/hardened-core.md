# The hardened core of sei-internal-skills

**Status**: Draft
**Created**: 2026-08-27
**Arc**: sei-agentic-mesh
**Constitution**: `.specify/memory/constitution.md` v0.1.0

## 1. Introduction and goals

This repository ships 16 core skills, 17 core agents, 12 experimental skills, and three
Omnigent bundles. The core alone is 14,654 lines of markdown. Every teammate who runs
`make update` installs all of it.

This design cuts that to the set we can polish, integrate, and defend, and it states the
rule that keeps the set from growing back.

The rule is the constitution's Principle IV: **the surface that reaches an engineer
wins**. An agent is reachable by role name. A gate blocks a merge whether or not anyone
invoked it. A skill needs an install and a remembered trigger phrase. Where a convention
can live on a surface that reaches people, it does not belong in one that does not.

### Goals

- **G1** Ship a core small enough that every component has a polished, integrated
  experience and an owner.
- **G2** Move general engineering method from bespoke prose onto public standards the
  model already holds, and record the evidence for each move.
- **G3** Finish the Omnigent integration for `/xreview`, and open the same path for
  `/root-cause`.
- **G4** Give spec-driven development a first-class managed experience, and close the
  loop by reviewing work against the spec that authorized it.

## Non-goals

- **NG1** Rewriting the Sei-local operational skills. `/harbor-dev` is 3,691 lines
  because Sei's harbor cluster needs 3,691 lines. Length is not the defect.
- **NG2** Retiring `experimental/`. Parking is the pressure valve that makes the cut
  possible.
- **NG3** Deleting history. A cut resource keeps its history, in the repository or in
  the archive snapshot.
- **NG4** Changing the sync mechanism. `sync-skills.sh` and `sync-agents.sh` read
  `.claude/` and nothing else. That structural exclusion is what makes the tier boundary
  hold, and this design depends on it.
- **NG5** Forking Spec Kit. We vendor it and add deltas. We do not fork the method.

## 2. Constraints

| Constraint | Consequence for this design |
|---|---|
| Claude Code discovers skills and agents **flat**, under `~/.claude/skills/<name>/` and `~/.claude/agents/<name>.md` | Domains stay metadata. A cut must be a real removal, never a nested folder. |
| Sync never deletes | Retiring a resource does not un-install it. Every cut needs a `prune-retired.sh` entry in the same change. |
| An agent body loads in full on dispatch | Moving a corpus into an agent moves its cost from on-demand to every-dispatch. This is the central trade-off in §7.5. |
| Omnigent registers bundles at server boot from `OMNIGENT_BUILTIN_AGENT_DIRS` | A bundle name is a one-way door. A route resolves `root-cause` and `xreview-scout-codex` by name. |
| Omnigent host-scope discovery scans `~/.claude/skills/` unconditionally | What we ship locally is reachable from a managed session unless a bundle sets `skills: none`. |
| The `sei-agent-driver` stack is unmerged | PRs #339, #340, #341. Any change to `/xreview` must sequence against them. |

## 3. Context and scope

Four surfaces carry a convention to an engineer. Each has a different reach.

| Surface | Reaches an engineer | Cost | Holds |
|---|---|---|---|
| **Contract** — `AGENTS.md`, `CLAUDE.md`, the constitution | Always loaded, no invocation | Every session pays for every line | Anchors, and local rules with no public prior |
| **Agent** — `.claude/agents/<name>.md` | Named by role | Every dispatch pays for the whole body | The persona and the judgement a gate cannot express |
| **Gate** — CI, `vale`, `make verify-*` | Blocks a merge | Runs once per change | The checkable subset |
| **Skill** — `.claude/skills/<name>/` | Install, then a remembered phrase | Loads on trigger only | A procedure with side effects outside the repository |

The Omnigent bundles in `agents/` are a fifth surface with different economics: an
operator bakes them into a server image, and an engineer picks one from a dropdown. A
bundle reaches someone who never installed anything. That is why the flagship workflows
belong there.

## 4. Solution strategy

Three moves, in this order.

1. **Cut the core to what we ship.** §5.
2. **Move general method onto anchors, and keep the evidence.** §6.
3. **Finish the Omnigent path for the two flagships, and close the spec loop.** §8.

Order matters. Anchoring a skill we are about to park wastes the work. Wiring a skill
whose scope is still moving wastes more.

## 5. The hardened core

### 5.1 What ships

**Ten skills at the end of this plan, not eight.** A first draft said eight and no feature
could produce it. The arithmetic: 16 today, minus `/chaos-suite` (Feature 001), minus
`/brevity` and `/pr-quality` (Feature 003), minus `/systems`, `/evm`, and `/idiomatic`
(Feature 002) = **10**.

`/kubernetes` and `/platform` stay, because §7.5 defers them to a four-way de-duplication
pass and **no feature owns that pass.** Feature 007 owns it, and the plan says so rather
than implying a number it cannot reach. The end state is ten, with a named path to eight.


| Component | Kind | Why it stays |
|---|---|---|
| `/xreview` | Skill | The flagship. Real side effects: it commits a review ledger to the DRI's designs repository. Already the Omnigent integration target. |
| `/root-cause` | Skill + bundle | Named as high value. The bundle already exists and a PagerDuty route resolves it. |
| `/harbor-dev` | Skill | Daily self-service. Real side effects: it creates and destroys chains on a live cluster. |
| `/validate-release` | Skill | Real side effects: it reads a live harness and writes a report to Notion. |
| `/author-skill`, `/audit-skill` | Skills | Load-bearing. `/xreview` HALTs when either is absent on a `skill-package` change. See §7.6 — the pin is currently a presence check. |
| `/gov-ops`, `/validator-platform` | Skills | Forced by review. `platform-release-manager` ships and depends on both. §5.3. |
| 16 specialist agents | Agents | The surface that works. See §5.3. |
| `asd-ste100` | Output style | One file. Shipped, opt-in, already installed. |

### 5.2 What cuts

**`/chaos-suite` is cut, not parked.** Its `SKILL.md` invokes eight scripts. None exists.
`scripts/README.md` states it plainly: "These scripts are **scaffold placeholders**." Its
guardrails — a context allowlist, an environment flag, a verbatim confirmation — are
enforced by `context-check.sh`, which also does not exist. The safety model is prose
describing a program nobody wrote. It carries no evals.

**One ground of this cut was false, and the correction resizes the work.** A first draft
said `/chaos-suite` has no inbound citation. It has seven. Two are `description:`
frontmatter anti-triggers in *shipped* skills — `/harbor-dev` and `/gov-ops` both redirect
chaos work to it — and a `description:` is the surface Claude Code reads to choose a skill.
One is executable: `author-skill/scripts/scaffold.sh` names it in a `PROTECTED` array, and
an eval asserts that. The rest sit in `audit-skill`'s protected-list policy, its rule D4
sibling-redirect list, and `author-skill`'s guardrails.

An engineer who says "run the chaos tests for the release cut" would therefore, after a
naive delete, get routed by two shipping skills to a skill that does not exist — reproducing the
exact defect §7.1 exists to prevent. Feature 001's sweep is sized against seven, one of
them code and one of them a test.

Parking preserves the liability. `make sync-experimental` would hand an engineer a skill
that starts a chaos run against a cluster and halts on a missing script, after the
confirmation prompt already told them the run began. Experimental means still forming, not
declared but unwritten. The real artifact is the runbook at `sei-protocol/platform#169`,
which is not in this repository.

**Three things in it live nowhere else, and they move before the delete.** Review found
each of them absent from `/validate-release` and `/harbor-dev`:

| Knowledge | Where it is now | Why it matters |
|---|---|---|
| **The false-green failure mode** — a scenario reports PASS because the fault never landed — with per-fault evidence: `tc -s qdisc` for packet loss, log-timestamp delta for time skew, tproxy logs for HTTPChaos | `chaos-suite/SKILL.md`, `scripts/README.md`, `references/summary-template.md` | `/validate-release` disclaims this capability in writing: its metrics are "supporting evidence, not an independent second opinion." After a naive delete, nothing in this repository names the highest-consequence outcome on the release surface. |
| **The baseline precondition** — do not layer chaos on an already-degraded cluster | `chaos-suite/SKILL.md` | `/validate-release` records that "there is no pre-chaos baseline phase in the nightly harness." Nothing would state that a verdict taken on a degraded cluster is not a verdict. |
| **Leaked-chaos recovery** — surface the residue, never auto-restart, log the operator response | `chaos-suite/SKILL.md`, `references/guardrails.md` | Harbor is a shared dev cluster that runs the nightly chaos, and `/harbor-dev` works on it daily with no leaked-chaos entry in its troubleshooting reference. |

**Decision.** Move those three into `/validate-release`, and add an **Independent
verification** row to its report template that renders `NOT VERIFIED` by default. Roughly
25 lines. Then delete the other 300. Feature 001 owns the move and the delete as one
change.

**`go-to-market-specialist` is cut.** No skill in either tier references it.

### 5.3 What the review took off the park list

Two candidates failed review, and the reason is the same in both cases: a shipped agent
depends on them.

**`/gov-ops` stays in the core.** `platform-release-manager` ships, and its governance
section reads: "**Always run this through the `/gov-ops` skill**; do not hand-roll the
steps. The skill encodes the hard, fail-closed safety gates this work requires, learned
the hard way on arctic-1." Those gates include an allowlist of context, network, and
namespace triples, and a refusal on any context that co-hosts `pacific-1` mainnet.

Parking it leaves a shipped agent, with mainnet-adjacent scope, instructed to use a skill
the default install does not place on disk. It will hand-roll or halt. Either outcome
converts a documented safety gate into a missing file. 226 lines is not where leanness
lives.

**The rule is durable, not one-iteration.** `/gov-ops` and `/validator-platform` ship for
as long as `platform-release-manager` ships a governance mode. A change that parks or
removes either skill deletes that governance mode in the same change. Never one without
the other. Spec 001 FR-013 carries this as a standing requirement rather than a
this-iteration prohibition.

**`/validator-platform` stays, and moves with `/gov-ops`.** Six of its files depend on
`/gov-ops`, and its sole consumer is `platform-release-manager`. Splitting it across ship,
park, and fold produces a shipped agent whose governance section cites two absent skills
and a relocated corpus. Treat the pair as one unit.

### 5.4 `/brevity` and `/pr-quality` dissolve, on measured coverage

An earlier draft shipped both, because 14 of the 17 core agents hard-path to them and
parking opens 28 dangling citations. That argument holds against *parking*. It does not
hold against *dissolving*, which replaces each citation rather than breaking it.

**A first draft of this section asserted that `vale` already checks four of `/brevity`'s
eight rules. That assertion was false, and two independent reviewers caught it.** The
claim is corrected here from a direct measurement rather than from inspection of rule
names. A section arguing for Principle III must not itself claim a gate that does not fire.

**Measured.** A fixture carrying one violation of each rule, run through the current
`vale` configuration:

| `/brevity` rule | Claimed gate | What the gate actually did | Disposition |
|---|---|---|---|
| 1 — Cut a sentence whose removal changes no reviewer action | BLUF | **Mismapped.** BLUF orders a document. Rule 1 deletes by a reader-action test. Different operation. | Contract line. No gate. |
| 2 — Open on the load-bearing noun, never a wind-up | BLUF | BLUF is an anchor, and Principle II says an unprobed anchor is a contract. | **BLUF**, after a probe verdict. |
| 4 — No "serves to", "aims to", "is responsible for", "allows us to", "exists to", "helps to" | `STE-Passive`, `STE-ApprovedWords` | **Neither fired on any of the six.** They are active voice and ordinary vocabulary. | Needs a new six-entry `substitution` rule. Nobody has written it. |
| 7 — Collapse hedges | `write-good.Weasel` | **Did not fire** on "generally", "typically", "basically", or "essentially" in the fixture. "It should be noted that" flagged only as passive voice, incidentally. | Partial at best. Contract line until a rule exists. |
| 3 — In-code comments at 4 lines or fewer | — | — | `/idiomatic` comment discipline. Number stays stated. |
| 5 — Do not restate a name; delete rather than shorten | — | — | `/idiomatic`. Nearest anchors: **Effective Go**, **Go Code Review Comments**. |
| 6 — Prefer one example over one paragraph | — | — | Contract line. |
| 8 — Treat headers as a budget | — | — | Contract line. |

Honest tally: **one rule maps to an anchor, two move to `/idiomatic`, five become contract
lines, and one of those five needs a `vale` rule that does not exist yet.** Dissolution
still wins, because five contract lines beat 561 markdown lines. It wins for less than the
first draft claimed.

**The gate cannot reach the primary surface, and that is the harder problem.**
`brevity/SKILL.md` line 11 names its two surfaces: PR descriptions and in-code comments.
**A PR body is not a file in the tree.** No `vale` invocation reaches it, ever. Standing
`vale` up in CI does not fix this; the mechanism cannot express the target. The 14 agent
stanzas say "Before `gh pr create`, apply `/brevity` to the staged diff and planned body" —
so the surface that motivates every one of them is the surface no gate can see.

In-code comments come out ahead, though not the way this section first proposed. Rules 3
and 5 were to land in `/idiomatic`. The operator then cut the comment standard out of
`/idiomatic` too, on the ground that its restatements were themselves producing the
verbosity they governed. Both rules land in the contract instead, and they land as
**numbers** — an in-body comment at 4 lines or fewer, a header at 20 or fewer — because
"sparingly" is unfalsifiable and the only numeric bound in the repository lived in
`/brevity`. No gate checks either number; the contract records them as uncheckable. **The PR body
would come out with no owner at all.** `prose-steward`'s declared scope is "a design doc,
HLD, LLD, PRD, or 1-pager" — it excludes a PR body explicitly.

**Decision: `prose-steward`'s scope gains the PR body.** One line in one agent. No new
skill, no new gate, and the discipline reaches an engineer by role name rather than by a
remembered phrase — which is the outcome this whole design argues for. Without that line,
dissolution deletes the discipline on `/brevity`'s primary surface, and the honest move
would be to keep the skill.

**`/pr-quality` decomposes further.** Its own registry does the work.

| Rule | Kind | Disposition |
|---|---|---|
| `narration_comments` | LLM judge | `idiomatic-reviewer` — the doctrine block already names it the comment champion |
| `temporary_migration_notes` | LLM judge | `idiomatic-reviewer` too. A stated contract rule has no dispatcher; nothing dispatches a document. |
| `authoritative_voice` | LLM judge | `prose-steward` |
| Brevity dispatch | Skill dispatch | Dissolves with `/brevity` |
| `no_cpu_limits` | Script, 39 lines | **The platform repository's CI.** It scans workload YAML. This repository holds 18 YAML files and **zero occurrences of `cpu:`**. |
| `harbor_ecr_convention` | Script, 34 lines | The platform repository's CI. It scans `clusters/harbor/**`, a path this repository does not contain. |

Both scripts had the same defect and a first draft caught only one. Neither targets this
repository. `/pr-quality` was their *distribution mechanism* into repositories that do not
run this library's CI, so dissolving the coordinator dissolves the distribution. **Feature
003 files them against the platform repository with a named owner, or they are lost.**

**The precondition, and it blocks.** `vale` runs nowhere in this repository's CI — no
workflow, no `Makefile` target, no script. Feature 003 stands it up before either skill
dissolves. Until that lands, this design MUST NOT be read as claiming a `vale` gate:
Spec 001 FR-010 forbids exactly that, and §9 marks Q8 accordingly.

### 5.5 What parks

Nothing, in this iteration. The park list did not survive review. Every candidate was
either broken enough to cut or load-bearing enough to ship.

That result is worth recording rather than hiding. The leanness win in this cut comes
from **§7.5**, which moves five knowledge corpora off the slash-command surface, and from
**§5.2**, which removes one broken skill. It does not come from parking.

### 5.6 The agent roster

One cut and two watch items, from a full reverse map of every core skill.

| Agent | Core-skill consumers | Action |
|---|---|---|
| `go-to-market-specialist` | none | **Cut.** No skill references it. |
| `product-manager` | `/xreview`, as scope-cutter on design briefs | **Keep.** A core skill dispatches it. |
| `product-engineer` | none | **Watch — the real orphan.** Its one citation is a step in `pr-quality`'s human "adding a rule" checklist, not a dispatch, and `/pr-quality` dissolves. Same class as `go-to-market-specialist`. |
| The remaining 14 | 2 to 11 each | Keep. |

### 5.7 The density table

Value density, measured. Every shipped, dissolving, distilling, and cut component, with
what it costs a reader and what it returns.

| | Skill | Markdown | Evals | Cited by | Dispatches | Side effect outside the repository |
|---|---|---|---|---|---|---|
| **Ship** | `/gov-ops` | 226 | 9 | 2 | 1 | Submits and votes on chain |
| | `/validator-platform` | 426 | 10 | 1 | 1 | None — corpus for the agent above |
| | `/validate-release` | 664 | 6 | 2 | 1 | Writes a Notion report |
| | `/root-cause` | 672 | 5 | 2 | 7 | None — propose-only. Omnigent bundle. |
| | `/xreview` | 774 | 11 | 7 | 4 | Commits a review ledger. Driver in flight. |
| | `/audit-skill` | 1,205 | 3 | 0 | 0 | Applies diffs, writes a report |
| | `/author-skill` | 1,432 | 3 | 1 | 0 | Scaffolds a directory tree |
| | `/harbor-dev` | 3,487 | 9 | 1 | 0 | Creates and destroys chains |
| **Dissolve** | `/pr-quality` | 322 | 8 | **18** | 0 | Posts a pull-request comment |
| | `/brevity` | 561 | 5 | **16** | 0 | None |
| **Distil** | `/systems` | 316 | 9 | 4 | 5 | None |
| | `/kubernetes` | 377 | 10 | 4 | 5 | None |
| | `/platform` | 390 | 10 | 4 | 5 | None |
| | `/evm` | 573 | 13 | 1 | 5 | None |
| | `/idiomatic` | 2,584 | 22 | 12 | 5 | None |
| **Cut** | `/chaos-suite` | 323 | **0** | 0 | 0 | Eight declared. None exists. |

Four readings follow.

**`/gov-ops` is the densest thing in the repository.** 226 lines, nine evals, two
consumers, and an irreversible on-chain side effect. It was on the park list. That is the
strongest evidence that line count alone is the wrong metric.

**`/brevity` and `/pr-quality` are the most-cited components here — 16 and 18.** No other
component comes close. That number is why parking them was never available: a park breaks
sixteen to eighteen citations at once. It is also why dissolution has to *replace* every
one rather than delete it. The citation count measures reach, and reach is the thing
§5.4 preserves by moving the rules to a surface that loads unasked.

**`/audit-skill` and `/author-skill` are the weakest entries in the ship list.** Together
2,637 markdown lines and six evals — the worst tested-to-size ratio of anything shipping.
`/audit-skill` carries zero citations of its slash form, and it ships only because
`/xreview` pins it by name on a `skill-package` change. §7.6 already found that pin
verifies a filename rather than a rubric. **These two are the next reduction target after
this iteration, and the design does not pretend otherwise.**

**`/idiomatic` earns its size on evidence, not on assertion.** 22 evals, the most of
anything in the repository, and twelve consumers. That is why §7.5 distils it last rather
than first: it is the component where a wrong answer costs the most.

## 6. The anchor strategy

### 6.1 The measurement

A semantic anchor is a public term the model already holds. Naming it retrieves a body of
knowledge that the prompt would otherwise have to restate. The public catalogue lists 196
anchors across 15 categories.

**The catalogue carries the names and nothing else.** It documents no failure modes, no
probe method, and no guidance on when an anchor is the wrong tool. Our three failure modes
— open substitution, silent substitution, invention — and the probe that separates them
are local contract. A reader cannot follow them to the catalogue, so the constitution
states them in full.

Measured on `origin/main` @ `579519d`, counting distinct public standards each package
names, with word-boundary matching. Basis for every line count in this design: markdown in
`SKILL.md` plus `references/`, excluding `evals/` and assets. Ten of the sixteen core
skills are shown — the six omitted (`/brevity`, `/pr-quality`, `/chaos-suite`, `/gov-ops`,
`/validate-release`, `/harbor-dev` sibling rows) name one standard or none, and §5.4
measures `/brevity` and `/pr-quality` directly:

| Skill | Lines | Distinct standards named | Lines per standard |
|---|---|---|---|
| `/systems` | 484 | 12 | 40 |
| `/kubernetes` | 491 | 3 | 164 |
| `/platform` | 504 | 3 | 168 |
| `/xreview` | 1,031 | 6 | 172 |
| `/root-cause` | 744 | 4 | 186 |
| `/evm` | 721 | 1 | 721 |
| `/author-skill` | 1,506 | 2 | 753 |
| `/audit-skill` | 1,279 | 1 | 1,279 |
| `/harbor-dev` | 3,691 | 1 | 3,691 |
| `/idiomatic` | 3,028 | 8 | 379 |

### 6.2 What the measurement means

Density alone is not the signal. The second axis is whether the content **has** a public
prior at all.

| | Public prior exists | No public prior |
|---|---|---|
| **High density** | Anchored correctly. `/systems` is the model: 484 lines, 12 named standards, each one citable. | Citation padding. Not observed. |
| **Low density** | **Restating.** The target. | **Legitimate local content.** `/harbor-dev` earns its 3,691 lines. |

Three findings follow.

**`/systems` is the pattern to copy.** It holds a citable corpus and points at it: Google
SRE, the AWS Builder's Library, OpenTelemetry semantic conventions, TIGER STYLE, Google
AIP, the twelve-factor rules. 484 lines carry twelve standards' worth of method.

**`/author-skill` and `/audit-skill` are the clearest targets.** Together they are 2,637
markdown lines and name two public standards. Skill authoring has real public priors — Diátaxis for
mode, progressive disclosure for structure, Anthropic's own published skill guidance — and
almost none of them appear.

**`/xreview` and `/root-cause` are doing anchored work without naming the anchors.**
`/root-cause` writes "falsification" 60 times and names Popper once; its method also
covers differential diagnosis, fault-tree reasoning, and the blameless postmortem, none
of them named. `/xreview` names Asch four times and red-teaming four times; its boundary
review is Design by Contract and consumer-driven contracts, and its rationalization table
is Janis on groupthink. Naming them shortens the skill and makes each rule arguable
against a source.

**`/harbor-dev` is not a target.** One standard across 3,691 lines is the right ratio for
content with no public prior. Applying a density rule to it would be a category error.

### 6.3 The rule

Principle II governs. An anchor enters the constitution's table only after a probe records
that a model resolves it, with the model identifier and the date. **No anchor in the table
carries a verdict yet.** Until one does, a surprising output means the anchor failed, not
that the model disagreed.

The catalogue is a source of candidate names. It is not evidence, because it records no
verdicts. Importing a name from it without probing is the exact bet Principle II forbids.

### 6.4 The gap this leaves

An anchor is a hint that lives in context. A gate measures the artifact from zero. Prose
quality must not depend on the hint surviving, which is why every anchored rule needs a
`vale` rule or an explicit "uncheckable" record. Principle III makes that mandatory and
Principle V makes the gap visible per anchor.

## 7. Decisions

### 7.1 Decision: reference integrity becomes a fail-closed gate, and lands first

This is the blocking prerequisite. Nothing else merges until it is green.

**The defect already ships.** Three dangling references sit in `main` today, verified:

| Citing file | Cites | Reality |
|---|---|---|
| `.claude/agents/systems-engineer.md` | `.claude/skills/ebpf/` | lives in `experimental/` |
| `.claude/skills/audit-skill/scripts/README.md` | `.claude/skills/coral` | lives in `experimental/` |
| `.claude/skills/author-skill/SKILL.md` | `.claude/skills/terraform-review/` | exists nowhere |

**And a bigger one.** `/code-review` is cited 10 times across 6 core files —
`xreview/SKILL.md`, `idiomatic/SKILL.md`, `systems/SKILL.md`, `.claude/skills/README.md`,
`prose-steward.md`, `idiomatic-reviewer.md` — as the correct hand-off for line-level
correctness. **It exists in neither tier.** The hardened core points at a skill that is
not there, for the one review axis nothing else covers.

**Why no gate catches this.** `verify-catalog` checks that every `category:` maps to a
sync alias. `verify-doctrine-block` checks that this repository's `AGENTS.md` matches the
source file; it never checks that a named skill exists. No workflow resolves a cited skill
path. The failure presents to an engineer as the *agent* misbehaving — "it told me to run
`/pr-quality` and nothing happened" — not as an incomplete install. That erodes confidence
in the skills that do work.

**Decision.** A `verify-references` job resolves every `/skill` citation and every
`.claude/skills/<name>` path in `.claude/agents/**`, `.claude/skills/**`, and
`scripts/sei-internal-skills-doctrine.md` against the tree, and fails closed. **It fails
on `main` today.** That is the point. Feature 001 owns it.

**The gate does not exempt `/code-review`.** A first draft left that open, and the two
readings conflict: a gate cannot both fail closed and tolerate ten known-dangling
citations. Exempting it would weaken a gate to make a check pass, which Principle III names
by name. **Feature 001 rewrites all ten citations** across `xreview/SKILL.md`,
`idiomatic/SKILL.md`, `systems/SKILL.md`, `.claude/skills/README.md`, `prose-steward.md`,
and `idiomatic-reviewer.md`, and records the capability gap once in the doctrine block with
its un-defer trigger: the first correctness defect that reaches `main` through an
`/xreview` that had no lens for it. That is the reference sweep Feature 001 already scopes.

**Six gates go red under this plan, and a first draft named one.** Each has a correct fix
(update the fixture) and a cheap wrong one (delete the assertion). Naming them here is what
keeps the cheap fix off the table at 2am:

| Gate | Assertion | Breaks because |
|---|---|---|
| `verify-references` | new | Intentional. §7.1. |
| `verify-runner-image.yml` | roster `-ge 17` | The roster goes to 16. This is the only detector of accidental roster loss in the runner image. |
| `install.test.sh` | `piped skill brevity` | `/brevity` dissolves. |
| `catalog-coverage.test.sh` | `idiomatic is in portable` | `/idiomatic` leaves `.claude/skills/`. |
| `prune-retired.test.sh` | `test -d .../skills/idiomatic` | Same. This is the only proof the deleting script never eats a core resource. |
| `verify-runner-image.yml` | steward file presence | §7.6 re-keys the pin to rubric files; the gate still checks `SKILL.md`. |

**The sweep is an order of magnitude larger than a first draft said.** Three dangling
references was the count of *path-form* citations. Counting the spec's own definition — a
backticked `/name` or a `.claude/skills/<name>` path — core artifacts hold **83 citations
of resources the core does not hold**, and 25 of them sit inside shipped agent bodies:

| Cited | Total | In agent bodies |
|---|---|---|
| `/issue` | 18 | 16 |
| `/coral` | 16 | 2 |
| `/design` | 12 | 2 |
| `/council` | 11 | 2 |
| `/code-review` | 10 | 3 |
| `/workstream` | 9 | 3 |
| `/research` | 4 | 2 |
| `/bugbash` | 3 | 0 |

`sre-engineer` alone carries four instances of "file `/issue` work" — that is its whole
escalation contract, pointing at a parked skill. Feature 001 edits 25 agent citations, not
three. That is a sequencing cost the first draft did not price.

### 7.2 Decision: the gate measures the tree an engineer runs, not the tree an author edits

The assigned dissenter's objection, and it holds.

`verify-references` as first specified resolves citations against **the repository**, where
`.claude/skills/pr-quality/` exists. But `/brevity` and `/pr-quality` declare
`category: output-quality`, and `sync-skills.sh` places that domain in
`SEI_INTERNAL_SKILLS_LOCAL_DOMAINS` — "deliberately NOT synced outward." `make update`
syncs `--categories all`, which resolves to portable plus sei. **Neither skill has ever
reached a machine through the documented daily flow.**

Meanwhile 14 shipped agents carry 23 hard paths to them, and every one of those agents is
in a portable domain, so `make update` installs all 14. The motivating complaint — "it
told me to run `/pr-quality` and nothing happened" — is therefore **reproducible on every laptop
today, and the repository-scoped verifier passes the exact citation that produces it.**
Feature 001 would merge green while the engineer-visible defect stayed untouched.

**Decision.** Split the check.

- `make verify-references` keeps the repository check. It fails `main` today.
- `make verify-install` resolves every citation in `~/.claude/agents/**` and
  `~/.claude/skills/**` against `~/.claude/`, and runs from `sync-all` beside the existing
  prune check. **It fails on 23 citations today.** That is the same argument, applied to
  the tree that matters.
- The sync scripts stamp `version.json` and the source commit into `~/.claude/`, so any
  session can report which roster the machine holds. Nothing does that today.

This also settles §5.4 on firmer ground than the first draft used. Dissolving `/brevity`
and `/pr-quality` does not risk 28 citations, because those citations are already dangling
everywhere except this checkout. **Dissolution repairs them; it does not endanger them.**

### 7.3 Decision: the tier boundary becomes a gate

Today the boundary is a convention plus a README. The README claims 17 core skills; there
are 16. `AGENTS.md` still names `/coral` and `/council` as core dispatchers; both are
experimental. `verify-catalog` catches neither.

**Decision.** A verifier derives the counts and the dispatcher list from the tree and
fails on drift. Feature 001 owns it.

### 7.4 Decision: `.claude/skills/` has two consumers, and the second one auto-approves

This design's model of `.claude/skills/` was single-consumer: a menu an engineer picks
from. **It is also the unconditional discovery scope of a headless agent that approves its
own tool calls.** Security review found the chain, and every link resolves in this tree:

| Link | Evidence |
|---|---|
| The headless bundle auto-approves | `agents/root-cause/config.yaml` — `permission_mode: auto` |
| Its skill filter is `all` | No `skills:` key, and `verify-agent-bundles.yml` asserts `'root-cause': {'filter': 'all'}` |
| This repository populates its host scope | `Dockerfile.runner` copies `.claude` to the overlay, seeded into `$HOME` |
| The admin policy grants bare tools | `scripts/managed-settings.json` allows `Bash`, `Agent`, `Task` with no pattern |
| Its slate is unconstrained | The dispatch brief reads "from this repo's `.claude/agents/` roster" |
| One reachable agent has no tool limit | `.claude/agents/platform-release-manager.md` carries **no `tools:` key** — an unset grant is every tool |
| That agent's manual is a mainnet procedure | Its governance section: "Always run this through the `/gov-ops` skill" |

**The cut makes this worse, and Principle IV is why.** Applying "a skill exists only where a
procedure has side effects outside this repository" removes the inert knowledge corpora and
leaves eight skills of which six mutate something outside the repository. The signal-to-noise
of the headless discovery surface improves — for an attacker as much as for an engineer.
§5.3 is right that `/gov-ops` must ship for the Claude Code surface. One directory serves
both surfaces, and that tension went unexamined.

**Decision, in Feature 001, before any deletion.**

1. Set `skills: none` on `agents/root-cause/config.yaml` and vendor what it needs, matching
   its two siblings. `verify-agent-bundles.yml` already gates non-leakage under `none`.
2. Give `platform-release-manager` an explicit `tools:` list. It is the only unset grant on
   an agent that carries a governance mode.
3. Extend `slate-routing.md` §4a's `security-specialist` trigger surface to `.claude/**`,
   `agents/**`, `scripts/managed-settings.json`, `Dockerfile.runner*`, and
   `.github/workflows/**`. A `skill-package` change pins prose, audit, and author
   unconditionally and does **not** pin security — so the highest-consequence review in this
   repository, of this repository, does not mandate the lens that found this. Five lines.

**A related claim in this design was false.** An earlier draft said the bundles carry a
"byte-for-byte copy so hermeticity does not depend on the host filesystem." Measured: the
vendored `sei-k8s-signal-ladder.md` is 138 lines, the core copy is 116, and the 22-line
delta is the entire MVP deployment envelope — "your **only** signal source is the Grafana
MCP. No shell, no `kubectl`, no `seid`, no `curl`, no node RPC." **The core copy instructs
the agent to run `kubectl describe`.** With the filter at `all`, both are in one session and
which one carries is undetermined. `skills: none` collapses this as a side effect.

### 7.5 Decision: distill the spine, keep the corpus lazy. Do not fold.

Two specialists reviewed the naive fold independently, without seeing each other. Both
rejected it. Their reasoning differs, which makes the agreement corroboration rather than
anchoring.

**The arithmetic was wrong first.** The 5,300-line figure counted `evals/evals.json`,
which never enters a context window, and asset files. The foldable body — `SKILL.md` plus
`references/` — is **4,118 lines**. More importantly, **the fold removes zero lines from the
system.** It relocates them from lazy-load to eager-load.

Counts are `SKILL.md` plus `references/*.md`, measured. Every line count in this design
uses that basis. `/validator-platform` is absent because §5.3 ships it as one unit with
`/gov-ops`.

| Skill | Foldable lines | Agent today | Multiplier |
|---|---|---|---|
| `/idiomatic` | 2,559 | 51 | 50x |
| `/evm` | 549 | 38 | 14x |
| `/platform` | 366 | 39 | 9x |
| `/kubernetes` | 353 | 40 | 9x |
| `/systems` | 291 | 58 | 5x |

**Finding 1 — the governance hole.** `.claude/skills/audit-skill/scripts/static-checks.sh`
line 135 emits a **block**-severity finding for a `SKILL.md` over 500 lines. That gate is
rule B1, sourced to progressive disclosure. **No equivalent gate exists on any agent.**
`sync-agents.sh --verify` checks only that `category:` maps to a sync alias. The fold
writes a 2,559-line instruction body — five times the ceiling — onto the one surface the
rule cannot see. It does not violate the rule. It relocates the content beyond the rule's
reach, which Principle III forbids: a gate is never weakened to make a check pass.

**Finding 2 — prior inversion.** `/idiomatic` exists for one stated reason, in its own
body: "The failure mode is not ignorance of idiom; it is **skipping the package's
documented conventions and exceptions**." The cure is a sequencing gate. The agent reads
`CLAUDE.md` and `doc.go` first, then loads one language pack. Fold the corpus in and the
reviewer starts every dispatch already holding six language packs and five example
corpora, free and first. The profile-first instruction then competes for attention against
knowledge that arrived before it. **The fold pre-loads the exact failure the skill exists
to prevent, and demotes the cure to a line in a long prompt.**

**Finding 3 — dead weight.** On a Go-only diff, the relevant subset is about 36% of what
would load. The rest is Python, Solidity, Rust, TypeScript, and Bash idiom — each a
competing set of "prefer X over Y" rules for a model that has to pick one.

**Finding 4 — the slate multiplies it.** (Slate figures below are order-of-magnitude
estimates from bytes-per-line, not tokenizer measurements.) `slate-routing.md` §4 wires `idiomatic-reviewer`
on **any code diff**, and §4a pins `systems-engineer` on any change touching concurrency,
resource lifecycle, or back-pressure. A realistic eight-lens T3 slate goes from roughly
13,000 to 135,000 tokens of agent bodies, about 10x. The orchestrator's window is safe —
reviewer outputs return, not bodies — so the damage sits inside the eight parallel
reviewers, and that is where `/xreview` least affords it: a reviewer holding tens of thousands of tokens
of rules before it sees the artifact is the reviewer most likely to produce confident,
well-cited, low-signal output.

**Finding 5 — `/idiomatic` is not 1:1.** Seven consumers outside itself name it: three
agents, `xreview/SKILL.md`, `slate-routing.md`, `audit-skill`'s conventions catalog, and
the doctrine block that syncs into every consuming repository.

**The precedent points the other way.** Commit `e1fc5b4` folded `/language` into
`prose-steward`. The agent went from 112 lines pointing at a 190-line skill to 107 lines
carrying four rules. It kept four of six rules and dropped the packs and the exemplar
corpus. **That fold shrank.** The successful precedent is a distillation, not a move.

**Decision.** Three parts, in order.

1. **Fix the install edge first.** `scripts/install.sh` dispatches `skill` and `agent` as
   disjoint targets by design: "Ask for a skill and you get that skill — not its agent,
   not the skills it references." Therefore `install.sh agent idiomatic-reviewer` lands a 51-line
   agent whose whole operating manual is a dangling pointer. **That is the observed
   adoption failure, and it costs about 20 lines to fix.** An agent's frontmatter names its
   backing skill, and installing the agent installs it. Zero per-dispatch context cost.

2. **Add the missing gate.** An agent body gets a line ceiling and a CI check, before any
   content moves onto that surface.

3. **Distill, do not move.** Each agent absorbs its discipline spine, its profile-first
   gate, and a routing index from concern to reference file. The language packs and example corpora stay on disk and stay lazily read. The slash command
   goes; the corpus moves to a non-triggering location the agent reads.

**Order:** `/systems` first — 291 lines, 5x not 50x, and the lens the driver already
duplicates, so one source of truth pays twice. Then `/evm`, the one true 1:1 case.
`/kubernetes` and `/platform` are not folds at all. Both `platform-engineer` and
`kubernetes-specialist` cite each of them, and `/validator-platform` cites both, so they
need a four-way de-duplication pass first. `/idiomatic` last. It is 62% of the corpus and the
highest-frequency dispatch, so it is where a wrong answer costs most.

### 7.6 Decision: make the steward pin verify a rubric, not a filename

`/xreview` HALTs when `audit-skill` or `author-skill` is "absent from `.claude/skills/`."
A directory holding only a stub `SKILL.md` satisfies that. A dispatched reviewer can load
`audit-skill/SKILL.md` — 154 lines of workflow — never open
`references/conventions-catalog.md`, and the pin reports satisfied. **The pin verifies a
filename, not a rubric.**

**Decision.** `slate-routing.md` §4 names the rubric files rather than the directory, and
a steward's RATIFY or DISSENT verdict cites a catalog rule identifier. A citation gives
falsifiable evidence that the rubric loaded. Presence gives none. About 10 lines of change.

**Feature 004 owns it**, alongside the spec-alignment lens, because both edit
`slate-routing.md` §4 and shipping them together costs one review of that file rather than
two.

It also corrects the record on weight. The pin's real surface is roughly 867 lines of
rubric, not the 2,637 markdown lines the two skills total. The remaining 1,770 lines are
authoring and audit workflow, which stay as skill and unpinned.

### 7.7 Decision: add a spec-alignment reviewer to `/xreview`

`/xreview` has lenses for boundaries, idiom, and prose. It has none for **whether the work
matches the specification that authorized it.**

The gap is real and Spec Kit's own documentation names it. `converge` treats the artifacts
as the sole source of intent and must not modify them, so it can drag correct code toward
a stale spec but cannot detect the staleness. `analyze` runs before implementation and
never reads code. Neither fails closed. Human review is the only backstop today.

**Decision.** A new reviewer lens reads `spec.md`, `tasks.md`, and the Linear ticket, and
reports against the diff on four axes:

| Axis | Finding when it fails |
|---|---|
| **Coverage** | A requirement ID no task and no test names. |
| **Excess** | A change no requirement authorizes. Scope that arrived without a decision. |
| **Staleness** | Code and spec disagree, and the code is right. The spec needs the edit. |
| **Ticket drift** | The Linear ticket's acceptance criteria and the spec's success criteria have diverged. |

It reports in an addendum, like the idiom and prose lenses, because its findings do not
fit the COMPATIBLE / MISMATCH / MISSING boundary schema. Correctness-grade findings gate a
passing verdict. Style-grade findings stay advisory.

This lens is the one that catches groomed work drifting from what the grooming settled.
Feature 004 owns it.

### 7.8 Decision: `specify init` must not write into `.claude/skills/`

`specify init . --integration claude` writes eight `speckit-*` skills into
`.claude/skills/`. In this repository that directory **is** the shipped core, so a scaffold
would ship eight upstream skills to every teammate and break `verify-catalog`.

The `sei-spec` Omnigent bundle runs exactly that command against any repository it opens.
Every repository with a governed `.claude/skills/` has the same collision.

**Decision.** Scaffold `.specify/` only. Feature 005 owns the general fix.

## 8. The Omnigent path

Two flagships, at different stages.

**`/xreview`** is mid-integration. The `sei-agent-driver` Go module drives one Omnigent
session per unit of work, adopting the session across dispatches so the agent's memory of
the tree survives. `xreview-scout-codex` supplies an independent reading on a different
model, and the driver refuses a scout that shares the review's own agent — a reading the
synthesis produced is not a second opinion. Three PRs are open: #339 session driver,
#340 xreview workload, #341 the CLI.

**`/root-cause`** has a bundle and a PagerDuty route. Its MVP envelope is Grafana MCP
only: no shell, no cluster API, no node RPC. The skill's signal ladder reaches past that
envelope, and the bundle prompt tells the agent to punt cleanly rather than substitute a
localizing metric for a decisive one. Widening the envelope is Feature 006.

**Spec-driven development** has the `sei-spec` bundle. It vendors Spec Kit 0.15.0, sets
`skills: none` so no rival methodology leaks in, and owns git because Spec Kit performs no
git operations at all. Its recorded limits are the work of Feature 005: no constitution
in the bundle, `converge` cannot detect a stale spec, and nothing decides whether a task
deserves the ceremony.

## 9. Quality requirements

Each requirement names the command that checks it, or the word `judgement`.

| # | Requirement | Verifier |
|---|---|---|
| Q1 | Every cited skill path and every `/skill` citation resolves. | `make verify-references` |
| Q2 | Every core resource meets the Principle VI conditions: evals present, citations resolve, an owner named, a doctrine-block entry. | `make verify-catalog` |
| Q3 | Every documented count matches the tree. | `make verify-catalog` |
| Q4 | Every core skill carries evals. | `make verify-catalog` |
| Q5 | No agent body exceeds its line ceiling. | `make verify-agent-size` |
| Q6 | Every anchor named in a core artifact resolves to the constitution's table. | a registry check in CI |
| Q7 | Every anchor in the table carries a probe verdict for the current default model. | the probe suite |
| Q8 | Every governed artifact passes the prose gate at error level. | `vale` — **not running in CI today.** Feature 003 stands it up. Until then this row records a gap, not a check. |
| Q9 | A cut resource has a `prune-retired.sh` entry in the same change. | `make verify-prune` |
| Q10 | A restructured agent scores no worse than its skill-backed predecessor. | the eval A/B in §10 |

**Q10 has no instrument today.** The six knowledge skills carry 1,105 lines of
`evals/evals.json` in a documented schema whose central field is `skill_loaded`, which is
an A/B harness. **No agent has an evals slot, and CI runs no evals at all.** One reference to
`evals.json` exists across `scripts/`, the `Makefile`, and `.github/`, and it asserts only
that the installer copied the file. Feature 002 moves the evals with the corpus. Until it does, the
restructure is unfalsifiable, and Principle III says to say so rather than imply a check.

## 10. Measurement

The restructure rests on a premise nobody here has measured: that an engineer does not
reach a skill through a remembered trigger phrase. Nothing in this repository records a
skill invocation. Two instruments, both cheap, both before the restructure.

**The load-rate counter.** One line per agent: report, in the output header, which
reference files it loaded. Count, across real `/xreview` runs, how often
`idiomatic-reviewer` loaded a language pack at all. A high rate means the pointer works
and the restructure solves nothing. A low rate justifies it — and adding the `Skill` tool
to the agent's frontmatter is then the cheaper fix for the same number.

**The three-way eval.** Run each skill's existing scenarios three ways: agent with pointer
and skill present; agent with pointer and skill absent; restructured agent. The second arm
measures the dangling-install failure directly. **If it scores level with the first, the
premise is wrong and §7.5 should not proceed.** Cap the sample; do not sweep.

## 11. Risks

Ranked by consequence, not by how easy each is to see.

| # | Risk | Consequence |
|---|---|---|
| R1 | The restructure lands with no instrument. | It deletes 988 lines of the only A/B harness in the repository and moves content to a surface with no slot for a replacement. "Did it work" becomes unanswerable by construction. §9 Q10. |
| R2 | Reference rot spreads to every consuming repository. | The doctrine block syncs into each consumer's `AGENTS.md` and names `/idiomatic`, `/systems`, `/pr-quality`, and `/brevity`. `verify-doctrine-block` checks self-consistency only, so a dangling name propagates silently. §7.1. |
| R3 | Deleting a core skill changes the Omnigent root-cause agent. | `agents/root-cause/config.yaml` carries **no `skills:` key**, unlike its two siblings, so host-scope discovery of `~/.claude/skills/` is live and the runner image seeds that directory. A root-cause session loses `/kubernetes`, `/platform`, and `/systems` as loadable corpora exactly when it is fanning out to those specialists. |
| R4 | A fourth bundle is unauditable from here. | `ecr-server.yml` registers `/opt/agents/sei-droid` alongside the three tracked bundles. That bundle is not in this repository, so nobody can read its `skills:` filter from here. Read it off the running server before deleting anything. |
| R5 | The restructure lands under three open pull requests. | #339, #340, #341 all carry `.claude/` identical to `main`. `verify-runner-image` and `ecr-runner` trigger on `.claude/**`, so every slice re-runs its image gates against a roster it was not written against. |
| R6 | The headless path keeps its own copy of the standards. | `sei-agent-driver/internal/xreview/prompt.go` line 255 hardcodes the systems lens as Go string literals — a goroutine with no exit path, a lock held across a blocking call, a retry of something not idempotent. That is `slate-routing.md` §4a re-typed into a language no review tooling reads. A third source of truth, already in flight. |
| R7 | **Realised, and accepted.** `/idiomatic/references/comment-discipline.md` held the only copy of the boundary table between `idiomatic-reviewer` and `prose-steward`, and the file is now deleted. | The boundary went with it. The contract states the standard; it does not state which lens owns which half. If the two reviewers begin to overlap or to each assume the other covers comments, that is the cost, and restoring a boundary statement in the contract is the fix. |
| R8 | Anchoring without probing. | A named anchor the model resolves to a neighbouring concept degrades silently. Only the open-substitution case announces itself. |
| R9 | A cut resource stays installed. | Sync never deletes. Everyone who synced before the cut keeps `/chaos-suite` until they run `make prune-retired-apply`. |

## Open questions

1. **Does the premise hold?** §10 tests it. If an agent already loads its pack reliably,
   §7.5 is solving a problem that is not there, and the install-edge fix in §7.5 part 1 is
   the whole change.
2. **What is `sei-droid`'s `skills:` filter?** Unreadable from this repository. R4.
3. **What is the failing threshold for a probe?** Published practice uses 80% and 50%
   bands. Adopting them without measuring our own anchors borrows a number.
4. **Which file is the contract?** `CLAUDE.md` loads every session. The `AGENTS.md`
   doctrine block syncs to consuming repositories. The Spec Kit phases read the
   constitution. One should generate the others. Which one is the source?
5. **Does `/code-review` get authored, or does the gap get recorded?** Four core skills
   hand off to it and it does not exist. Authoring it grows the core the moment we cut it.
   Recording the gap in the doctrine block is the cheaper honest move. §7.1.
6. **Do `product-engineer` and `product-manager` survive?** Both now have a single core
   consumer.
7. **Does the spec-alignment lens need Linear at review time?** A headless driver run may
   not carry the Linear MCP. A spec-and-tasks-only mode may be the shippable subset.

## Alternatives

**Fold the corpus into the agent body.** This was the plan. Two independent reviews
rejected it, and §7.5 records why: it relocates 2,559 lines past a block-severity gate
that has no agent-side equivalent, it pre-loads the exact failure `/idiomatic` exists to
prevent, and it multiplies an eight-lens slate by roughly ten. It removes zero lines from
the system.

**Vendor a corpus per agent, as the Omnigent bundles do.** `agents/root-cause/skills/`
carries a byte-for-byte copy so hermeticity does not depend on the host filesystem. The
Claude Code analogue is `.claude/agents/<name>/` as a directory. Rejected for now:
`sync-agents.sh` globs `*.md` and copies a single file, and `install.sh`,
`prune-retired.sh`, `Dockerfile.runner`, and `verify-catalog.yml` all assume the flat
form. **Changing the agent layout is a one-way door** and needs explicit human approval.

**Keep all 16 core skills and cut only the orphan.** Rejected. It changes nothing a
teammate filters past, and it leaves `/chaos-suite` — eight declared scripts, none
written — installed and dispatchable.

**Delete the knowledge skills and rely on the model's own priors.** Rejected. Principle II
forbids it. Without a probe verdict, deleting stated text bets that the anchor holds, and
silent substitution gives no signal when it does not.

**Fork Spec Kit to fix the `.claude/skills/` collision.** Rejected as NG5. The collision
has a smaller fix: scaffold `.specify/` only.

## Trade-offs

| We accept | To get |
|---|---|
| Eight shipped skills, not six. `/gov-ops` and `/validator-platform` came back. | A core whose every component is reachable and whose dependencies resolve. A smaller number that broke a safety gate is not leaner, it is wrong. |
| Dissolving two skills costs a CI gate we do not have yet. | Four rules become two anchors that improve as models train on more text about them, and the 14 agent stanzas that hold them point at a contract that needs no install. |
| Reference integrity lands first and fails `main` on day one. | A gate that catches the class of bug that has already shipped three times, before a cut multiplies it. |
| The restructure needs an instrument built before it starts. | An answer to "did it work" that is not a matter of opinion. |
| The restructure waits behind three open pull requests. | A review contract that is not moving under its own tests. |
| Anchoring costs a probe suite before it saves a line. | Rules that stay reliable as models improve, and findings a reviewer can argue against a source. |
| `/chaos-suite` goes rather than parks, losing a written procedure. | No engineer receives a safety model that describes a program nobody wrote. |

## The feature set

| # | Feature | Delivers | Depends on |
|---|---|---|---|
| 001 | Reference integrity and the core boundary | `verify-references` fail-closed. The catalog verifier. The `/chaos-suite` cut, the orphan-agent cut, and the reference sweep. | — |
| 002 | Expert roster and the agent surface | The install dependency edge, the agent size gate, the evals slot, the load-rate counter, then the distillation. | 001 |
| 003 | Anchor discipline | `vale` in CI **first**, then the registry, the probe suite, the density measurement. Then `/brevity` and `/pr-quality` dissolve and the 14 agent stanzas holding their 28 citations become contract references. | 001 |
| 004 | Spec-alignment reviewer | The fourth `/xreview` lens: coverage, excess, staleness, ticket drift. | 003 |
| 005 | Omnigent spec-driven experience | A constitution in the bundle, the Linear path, the scaffold fix. | 001 |
| 006 | root-cause on Omnigent | The trigger route and the signal envelope past the Grafana MCP MVP. | 002 |
| 007 | `/kubernetes` + `/platform` de-duplication | The four-way de-dup §7.5 defers, then the distillation. This is the path from ten skills to eight. | 002 |

**Sequencing, and the contradiction a first draft carried.** "001 merges alone and green
first" and "nothing touches `.claude/` while #339, #340, and #341 stay open" cannot both
hold: Feature 001 removes `/chaos-suite` from `.claude/skills/`, removes an agent, and
edits 25 citations inside `.claude/agents/**`.

**Resolution: Feature 001 splits at the `.claude/` boundary.**

| Slice | Touches | When |
|---|---|---|
| **001a — the gates** | `scripts/`, `.github/` only | Immediately. `verify-references` and `verify-install` land red and stay red. A failing gate on `main` is the forcing function, and it costs the driver stack no rebase. |
| **001b — the sweep** | `.claude/agents/**`, `.claude/skills/**` | After #339, #340, #341 merge. The 25 citation edits, the `/chaos-suite` knowledge rescue and delete, the orphan-agent cut. |

001a is what makes 001b urgent rather than optional. 002 stops after its instrument until
§10 reports.

## References

- `.specify/memory/constitution.md` — the contract this design obeys.
- `.claude/skills/audit-skill/scripts/static-checks.sh` line 135 — rule B1, the 500-line
  block gate with no agent-side equivalent.
- `.claude/skills/xreview/references/slate-routing.md` — the tier floor in §3, the steward
  pin in §4, the mandatory concern-lenses in §4a. Feature 004 extends it.
- `.claude/skills/idiomatic/SKILL.md` line 12 — why the skill exists, and the
  profile-first gate the fold would demote.
- `.claude/agents/platform-release-manager.md` — the `/gov-ops` safety dependency.
- `.claude/skills/chaos-suite/scripts/README.md` — "scaffold placeholders".
- `agents/root-cause/config.yaml` — no `skills:` key. `agents/sei-spec/README.md` — what
  `skills: none` does and does not do.
- `.github/workflows/ecr-server.yml` line 154 — the fourth, untracked bundle.
- `scripts/sei-internal-skills-doctrine.md` lines 16, 22, 23, 25, 47 — the names that
  syncs into every consuming repository.
- `origin/slice/3-cli:sei-agent-driver/internal/xreview/prompt.go` line 255 — the third
  copy of the systems lens.
- Semantic anchor catalogue: <https://llm-coding.github.io/Semantic-Anchors/>
- `bdchatham/agentic-writing` `specs/001-anchored-agentic-tooling/spec.md` — the public V2
  thesis this design applies internally.
