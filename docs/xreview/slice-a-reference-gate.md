# Review ledger — Slice A, the reference gate

Target:       `scripts/verify-references.sh`, `scripts/tests/verify-references.test.sh`,
`.github/workflows/verify-references.yml`, two `Makefile` targets, a `scripts/README.md` row

Class:        shared-stack
Tier:         T3
**Note**: belongs in the DRI's designs repository under
`designs/sei-agentic-mesh/xreview/`. It sits here pending that move.

## Round 1

Round:        1

State:        OPEN
OpenFindings: 4
Convergence:  split
Blinded:      yes
Dissenter:    platform-engineer
Lenses:       3

### Routing

Steward `idiomatic-reviewer` wired by file-type-present (code diff). §4a pinned
`systems-engineer` (the change walks the filesystem and manages process lifecycle).
`prose-steward`, `audit-skill` and `author-skill` are **not** pinned this round: the Round 1
diff touches `scripts/` and `.github/` only, so the class is `shared-stack`, not
`skill-package`. They become mandatory in Round 2, when the sweep touches `.claude/`.

| Lens | Role | Verdict |
|---|---|---|
| `idiomatic-reviewer` | Package idiom, comment discipline | **DISSENT** |
| `systems-engineer` | §4a, filesystem and process lifecycle | **DISSENT** |
| `platform-engineer` | Assigned dissenter | **DISSENT** |

Three of three dissented. That is the review working.

### Findings closed

| Finding | Lens | Evidence | Closed by |
|---|---|---|---|
| **The gate did not catch the defect its own header claimed.** Repo scope reported zero findings on `/brevity` and `/pr-quality`; the `--installed` scope that would catch them exits 2 on a CI runner. | dissenter | Verified: `grep -c 'cites /(brevity\|pr-quality)'` returned 0 | New `UNSHIPPED` class, reading `SEI_INTERNAL_SKILLS_LOCAL_DOMAINS` from `sync-skills.sh` rather than restating it. **41 findings across 29 agent files.** |
| **PARKED-as-error errors on the correct authoring pattern.** The doctrine block names seven parked skills in one sentence that states the tier and the install command. The gate called that seven errors. | dissenter | `sei-internal-skills-doctrine.md:25`, 7 findings on one line | PARKED demoted to a warning that never affects exit status. 74 of 95 findings stop gating. |
| **The same `set -e` + `pipefail` bug the file documents, one line from the fix.** A tree with no `experimental/skills` died silently: exit 1, zero bytes. That is every consuming repository. | `systems-engineer` | Reproduced on a fixture; now a regression test | `|| true` guards the whole pipeline in `list_dirs()` |
| **One unreadable file discarded every finding already computed.** No `-type f`, so a dangling symlink aborts the run. | `systems-engineer` | Reproduced | `-type f` on both `find` calls |
| **The escape hatch could not clear a line carrying two names.** Ten lines in this corpus carry two or three. The gate printed a remediation that provably could not succeed. | both, independently | `product-engineer.md:88` carries three | Marker accepts a list; one marker clears every name on the line |
| **`MISSING-SCRIPT` had no escape hatch**, and its repo-root fallback is dead in installed scope | `systems-engineer` | 16 unclearable findings | Marker consulted; the check now runs in repository scope only |
| **`--installed` could never reach zero** — 12 installed skills come from elsewhere | `systems-engineer` | 23 of 180 measured | Own and foreign counted separately; the mode never gates |
| **Silent-green.** The scan builds an ERE interval regex from a string. If awk stops matching, the gate prints success having checked nothing. | `systems-engineer` | Not verifiable on the runner from here | A canary plants a citation and refuses to report if the scanner misses it. Verified by breaking the regex. |
| **`push: branches: [main]`** — no sibling verifier carries it, and it guarantees a permanently red default branch | `systems-engineer` | Confirmed against `verify-catalog`, `verify-experimental` | Removed |
| **No regression suite**, unlike every sibling gate | both, independently | `scripts/tests/` holds six | `verify-references.test.sh`, 10 cases, wired as a second workflow step |
| **`--help` printed `!/usr/bin/env bash`; `--target` with no value exited 1 against a contract saying 2** | `idiomatic-reviewer` | Both reproduced | `usage()` hoisted with the sibling filter; explicit guard exits 2 |
| **History-in-source**, twice — comments narrating bugs already fixed | `idiomatic-reviewer` | `AGENTS.md:118`, comment-discipline rules 1 and 4 | Removed. Present-state rationale kept. |
| **Header misstated its own class count**, and `--quiet` was undocumented | `idiomatic-reviewer` | Header said two classes; the code emits five | Header rewritten; every flag documented |
| **Missing `scripts/README.md` row** | `idiomatic-reviewer` | Every sibling has one | Added |

### Findings open

| # | Finding | Lens | What closes it |
|---|---|---|---|
| 1 | **The baseline decision.** The gate lands at 62 errors. 87 citation findings collapse to 13 distinct missing resources across 26 files — a 6.7:1 inflation that makes the debt look larger than it is. "Fail closed at 62" and "ratchet from 62" are different products, and this is a human call. | `systems-engineer` | An operator decision. The plan of record is gate-and-sweep in one pull request, which makes a baseline unnecessary — but only if the sweep actually lands with it. |
| 2 | **The stop list is a global, unowned blind spot.** An entry silences a name everywhere, silently. A future skill named `status` or `auth` would be permanently invisible. | both, independently | Scope entries to files, or require the exemption at the citation site. The suite records the collision as `KNOWN` rather than hiding it. |
| 3 | **The scan is fence-blind** and misses a citation split across lines. One of 87 findings sits in a fence today. | `systems-engineer` | Un-defer the first time anyone documents an example command in a fence. |
| 4 | **The marker syntax is a durable commitment.** It will land in the doctrine block and render into every consuming repository's `AGENTS.md`. | dissenter | Settle the syntax before the sweep, not during it. |

### Rejected findings

None. Every finding above was reproduced before it was accepted.

One claim was **checked and did not hold as I first read it**: my own `grep` for `brevity|pr-quality` returned 2, which looked like it refuted the dissenter's headline objection. Both matches were file *paths* containing `pr-quality`, not citations of it. The dissenter was right; my check was sloppy. Recorded because the near-miss is the finding.

## Round 2

Round:        2

State:        OPEN
OpenFindings: 3
Convergence:  unanimous
Blinded:      yes
Dissenter:    security-specialist
Lenses:       5

### Routing

Class is `skill-package` now that the diff touches `.claude/` and `agents/`, so
`audit-skill` + `author-skill` + `prose-steward` are pinned unconditionally. §4a — the row
this change adds — pinned `security-specialist` on the agent-instruction surface. The rule
was dogfooded on the change that introduced it.

| Lens | Kind | Verdict |
|---|---|---|
| `prose-steward` | agent-steward, pinned | **DISSENT** |
| `audit-skill` | skill-steward, loaded as rubric | **DISSENT** |
| `author-skill` | skill-steward, loaded as rubric | **DISSENT** |
| `security-specialist` | §4a, assigned dissenter | **DISSENT** |
| `seidroid` | the repository's own agentic reviewer, on the PR | **CHANGES_REQUESTED** |

Five for five. Four ran blinded and in parallel; `seidroid` ran independently on the pull
request. Three of the five reached the headline finding separately, which makes it
corroboration rather than one opinion.

### The headline finding

**The gate reported clean on the defect its own header names.** It matched a backticked
`/name` only, and every `description:` in this repository writes the bare form. So
`idiomatic/SKILL.md:5` carried "(use /pr-quality)" — a retired skill, on the surface that
routes invocation — while `verify-references` exited 0.

This is Round 1's finding recurring in a new form: a fail-closed gate that fails open, whose
green run then gets cited as evidence. The fix adds a bare-name pass with a boundary on each
side, skips a token carrying a digit, strips trailing punctuation, and stops scanning a
marker line as content. It also widens the scan to `README.md`, `AGENTS.md` and `CLAUDE.md`
— the two documents a new engineer reads first, and the reason a stale skill count survived
a sweep.

### Findings closed

| Finding | Lens | Closed by |
|---|---|---|
| Gate blind to bare slash-names on the routing surface | prose, audit, author, seidroid | Third scan pass; scan set widened to the root documents |
| `platform-release-manager`'s exhaustive `tools:` list had no `Skill`, severing its path to `/gov-ops` — the skill this change keeps in the core *because* that agent depends on it | security | `Skill` added. `Agent` and `Task` stay off. |
| `prose-steward` never gained the PR body, which the design calls the condition for an honest dissolution | prose, author | Scope extended |
| The marker harvested every name in a 400-character window, so a rationale could hide citations or trip a false stale | security | Names precede the em-dash; rationale exempts nothing |
| The design claimed `vale` runs nowhere in CI, twice, in a file this change adds | prose | Corrected. It runs on `.claude/**` at error level. |
| The suite counted a known gap as a pass, so its headline said 13 passed when twelve ran a check | author | Third bucket; headline reports the gap |
| Markers duplicated, orphaned, or sat between table rows where an HTML comment terminates the table | audit, author | De-duplicated; table rows reworded instead |
| A phantom pack dimension `D10` cited after its row was removed | audit, author | Citation dropped |
| `/chaos-suite` residue FR-010b required, including an eval asserting a list a correct agent could no longer name truthfully | audit, author, seidroid | All seven sites closed |
| A roster line describing `prose-steward` with a framing the agent says it dropped | prose | Corrected |
| The replacement stanza ran 36 words in 14 agents | seidroid | Split to three sentences |

### Findings open, and accepted with the reason recorded

| # | Finding | Lens | Why it stands |
|---|---|---|---|
| 1 | The `idiomatic-reviewer` / `prose-steward` boundary table lived only in the deleted file and went with it | prose | The contract states the standard but not which lens owns which half. Recorded in the design's R7 at its real size. If the two lenses begin to overlap, restoring a boundary line to the contract is the fix. |
| 2 | `skills: none` narrows the headless chain but does not close it — the agent roster is not governed by that filter, and 14 of 16 agents grant unrestricted `Bash` | security | Pre-existing, and the change is a net improvement. Closing it needs an `agents:`-side control the harness may not expose. |
| 3 | §4a's new row pins the artifacts and not the machinery that writes them, and an absent concern-lens degrades rather than halts | security | The path expansion is a follow-up. The un-defer condition is the second time an instruction-surface change lands without the lens. |

### Rejected findings

One, and the rejection is recorded because the reviewers split on it. `audit-skill` called the
`chaos-suite` protected-list entry a stale assertion that blocks authoring a real chaos skill;
`security-specialist` called it harmless over-protection that fails safe. The audit reading
won on evidence: an eval asserted the agent surfaces a list containing `chaos-suite`, and a
correct agent reading the live tree cannot say that truthfully. The entry is removed.
