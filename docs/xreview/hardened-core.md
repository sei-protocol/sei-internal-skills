# Review ledger — the hardened core

**Target**: `docs/design/hardened-core.md`, `specs/001-reference-integrity/spec.md`,
`specs/002-expert-roster/spec.md`, `.specify/memory/constitution.md`

**Class**: `skill-package`
**Tier**: T3
**Note**: This ledger belongs in the DRI's `<engineer>-designs` repository under
`designs/sei-agentic-mesh/xreview/`. It sits here pending that move.

## Round 1

**State**: OPEN
**OpenFindings**: 9
**Convergence**: split
**Blinded**: yes — no reviewer saw another's brief or output
**Dissenter**: `sre-engineer` (assigned)

### Slate

| Lens | Role | Verdict |
|---|---|---|
| `product-manager` | Scope and composition | RATIFY with conditions |
| `systems-engineer` | Loading architecture, dispatch cost | RATIFY with conditions |
| `sre-engineer` | Operational coverage — **assigned dissenter** | **DISSENT** |
| `prose-steward` | Dual-audience legibility — **pinned, `skill-package`** | **DISSENT** on all five axes |
| `security-specialist` | Safety and capability surface | **DISSENT** |

Two lenses reviewed twice: `product-manager` and `systems-engineer` were re-dispatched
against the revision to check whether it closed their findings.

The three skill-stewards a `skill-package` change pins are `audit-skill`, `author-skill`,
and `prose-steward`. `prose-steward` ran and dissented. **`audit-skill` and `author-skill`
have not run as review rubrics.** That is a pin not yet satisfied, and it is why this
ledger cannot reach a passing state on reviewer coverage alone.

### What the review changed

Corroboration, not anchoring: `product-manager` and `systems-engineer` reached the same
verdict on the fold from different evidence, without seeing each other.

| Finding | Lens | Disposition |
|---|---|---|
| The naive fold relocates 2,450 lines past a block-severity gate with no agent-side equivalent | `systems-engineer` | Closed. §7.4 replaces fold with distillation. |
| Folding `/idiomatic` pre-loads the failure its profile-first gate exists to prevent | `product-manager` | Closed. §7.4. |
| `/chaos-suite` declares 8 scripts, 0 exist; guardrails name a 9th | both | Closed. Cut, not parked. |
| `/chaos-suite` has 7 inbound citations, not 0 — 2 are shipped-skill anti-triggers, 1 is executable | `sre-engineer`, `security-specialist` | Closed. FR-010b. The design's claim was false. |
| Three `/chaos-suite` failure modes live nowhere else | `sre-engineer` | Closed. FR-010a moves them to `/validate-release` before the delete. |
| Parking `/gov-ops` breaks a shipped agent's mainnet refusal | `product-manager` | Closed. FR-013 is now a standing rule, with `pacific-1` verbatim. |
| `vale` does not check the four rules §5.4 claimed | `product-manager`, `systems-engineer` | Closed. Measured with a fixture; the claim was false; the table now carries per-rule verdicts. |
| `vale` can never reach a PR body — the surface all 14 stanzas name | `product-manager` | Closed. `prose-steward`'s scope gains the PR body. |
| **The gate measures the repository; the failure lives on the installed tree** | `sre-engineer` | Closed. `verify-install` added. `output-quality` never syncs, so those citations already dangle on every laptop. |
| 83 dangling citations, not 3. 25 in agent bodies. | `sre-engineer` | Closed. §7.1 carries the breakdown. |
| "001 merges alone" and the `.claude/` freeze contradict | `prose-steward` | Closed. Feature 001 splits into 001a (gates) and 001b (sweep). |
| An 8-skill core no feature can produce | `prose-steward` | Closed. End state is 10; Feature 007 owns the path to 8. |
| `product-engineer` and `product-manager` inverted | `product-manager` | Closed. `product-manager` is dispatched; `product-engineer` is the orphan. |
| **`.claude/skills/` is a headless auto-approving agent's discovery scope** | `security-specialist` | Closed in the design. §7.3. `skills: none` + explicit `tools:` land in Feature 001. |
| The bundle's vendored `root-cause` copy is not byte-for-byte — the 22-line delta is the "no shell" envelope | `security-specialist` | Closed. §7.3. The design's claim was false. |
| §7.4's corpus location is not in the Docker build context | `security-specialist` | Closed. FR-017 names the forced location. |
| Six gates go red; the design named one | `security-specialist` | Closed. §7.1 enumerates all six. |
| No security lens is pinned on a `skill-package` change | `security-specialist` | Closed in the design. §7.3 part 3. |

### Open findings

| # | Finding | Lens | What closes it |
|---|---|---|---|
| 1 | `scripts/managed-settings.json` grants bare `Bash`, `Agent`, `Task` and no verifier reads it | `security-specialist` | Extend `verify-agent-permissions.sh` and the workflow paths filter. |
| 2 | `.claude/settings.local.json` is a documented escape hatch the verifier cannot see, and `SKILL-TEMPLATE.md` teaches it | `security-specialist` | Gitignore it or check it. Name one file in the template. |
| 3 | No audit rule resolves a script a `SKILL.md` declares | `security-specialist` | Add rule S7 to `static-checks.sh` and run it in CI. Broader and cheaper than a bespoke checker. |
| 4 | `sei-droid` registers from a path the in-image gate cannot reach | `security-specialist` | Gate `OMNIGENT_BUILTIN_AGENT_DIRS` so an unparseable path fails the publish. |
| 5 | The install edge can bypass the tier boundary and traverse paths | `security-specialist` | Resolve the backing skill in `.claude/skills/` only; validate the name against a character class; make the edge depth-1. |
| 6 | `/audit-skill` + `/author-skill` are 30% of the ship set with 6 evals, and no feature reduces them | `product-manager` | Name a feature or record the exemption. |
| 7 | `/harbor-dev` is exempted from the density instrument twice without measurement | `product-manager` | Measure it, or mark the exemption unmeasured under Principle V. |
| 8 | The load-rate counter has two opposed consequences and no threshold | `prose-steward` | Decide what rate authorizes the restructure and what rate authorizes the cheaper fix. |
| 9 | `audit-skill` and `author-skill` have not run as review rubrics | pin | Dispatch a reviewer that loads each as its rubric. |

### Rejected findings

None. Every finding above was verified against the tree before acceptance, and the four
that contradicted this design's own claims were confirmed by direct measurement:
the `vale` coverage claim (fixture run), the `/chaos-suite` citation count (grep),
the byte-for-byte bundle claim (`diff`, 138 vs 116 lines), and the dangling-citation
count (83, not 3).
