# Cross-review ledger — /idiomatic Python language pack

Target:       branch framework/idiomatic-python-pack (.claude/skills/idiomatic/references/language-pack-python.md [new], examples-python.md [new], .claude/skills/idiomatic/SKILL.md [wiring], .claude/skills/README.md [stale-path fix]); PR opened from this branch
Class:        skill-package (a pluggable language pack for /idiomatic, conforming to language-pack-TEMPLATE.md)
Tier:         T3 (Component — a self-contained pack + examples + wiring on an established template)
Scope:        Add a Python coding-standards pack to /idiomatic (the next language after Go/Rust/TypeScript/Solidity/Bash), grounded in /research into publicly-established open Python standards. Strategic principle: enshrine the language's OWN published standards (PEPs, official docs, the Google Python Style Guide) + the consolidated community lint registry (Ruff), verified against source — never hand-rolled. Integrated into idiomatic-reviewer (auto-loads by `pyproject.toml` detection) + the standards-champion tooling.
Dissenter:    python-fidelity (the assigned dissenter — verifies every authority/anchor against the PEPs + the official Ruff/mypy registries AND by RUNNING ruff 0.15.17 / mypy 2.1.0 / black 26.5.1; the dominant risk is a wrong/nonexistent lint code or a fabricated "observed" diagnostic shipped to engineers' agents)

## Phase 1 — Design (the pack §1–§7)

### Round 1 — split
| Lens | Verdict | Finding |
|---|---|---|
| python-fidelity (assigned dissenter) | RATIFY | ran the real linters + read the PEPs; all 27 §7 lint codes verified to exist with the claimed names (`ruff rule`), headline ones fired live, mypy codes confirmed under `--strict`, every PEP attribution correct, typing currency accurate (`list[int]`/`X|None`/PEP-695), no fabricated authority. "Held the bar hard and could not break it." |
| author-skill | RATIFY (refine) | [medium] §6 missing the peer's profile-subordination note (Python is more config-dependent — opt-in rule sets); [low] §7 missing the consolidated judgment-only roster; [low] P7 table markdown defect; [low] P2 `@override`/PEP-695 no-anchor note. |
| audit-skill | RATIFY | shape conforms to TEMPLATE + TS peer; URLs well-formed; detection already routes `pyproject.toml`→Python; wiring is the fan-out. |
| prose-steward | REVISE | missing the front "citing a §7 anchor" gate; no §-roadmap; §3 "Explicit/Zen" bullet inverted the do-NOT-flag contract (over-flag risk); §1 P11 lacked its failure-consequence; P7 table defect. |

### Round 2 — RESOLVED, unanimous
All R1 findings applied (front citing-gate + roadmap; §6 profile-subordination note; §7 judgment-only roster; §3 recast to "Must NOT flag" + a "do not flag these" header; P11 consequence; P7 fixed; P2 version precision). All four lenses RATIFY.

## Phase 2 — Implementation (the fan-out: §4 anti-patterns + §5 overlays finalized, examples-python.md, wiring)

### Round 1 — split
| Lens | Verdict | Finding |
|---|---|---|
| python-fidelity (assigned dissenter) | RATIFY | independently re-ran the linters: EVERY "Anchor (observed)" diagnostic in examples-python.md reproduces with matching code + wording; every good snippet passes clean; every §4/§5 code (ASYNC251/230/110, RUF006, PT009/011/018, G004, E721) exists; no overclaim. The anti-fabrication claim holds under independent re-execution. |
| author-skill | REVISE | [high] the **mandatory P10 comment-discipline dimension had no worked example** (the peer TS pack carries one); [medium] the asyncio "good" snippet referenced an undefined `self`; [low] a §3 Zen pair missing; [low] tool-version-string inconsistency; [low] §4 bare-cited `C416` on the loop→comprehension case (where it doesn't fire). |
| audit-skill | RATIFY | all four wiring surfaces (SKILL.md References, README catalog, method.md detection, sync) mutually consistent; the stale `references/languages/<lang>.md` README path corrected; tables uniform; purely additive; no stray files. |
| prose-steward | RATIFY | examples-python.md dual-aligned — per-pair "why" lead + Basis + observed-anchor; divergence pairs carry the explicit do-not-flag signal; §4/§5 read consistently with the Go peer. Low nits (version triplet, EAFP lead, §4 C416 framing) folded in. |

### Round 2 — RESOLVED, unanimous
All author findings applied + **empirically re-verified by running the linters**: added the P10 what-comment/tombstone example pair (good passes clean; `ERA001` confirmed to NOT fire on it, only on commented-out code) and the §3 Zen pair (`@dataclass` good clean); fixed the asyncio good snippet to a module-level `_tasks: set[asyncio.Task[None]]` (both snippets symmetric free `async def`; good passes `ASYNC,RUF006` clean, bad still fires ASYNC251+RUF006); harmonized versions to 0.15.17/2.1.0/26.5.1; corrected the §4 `C416` framing. author + prose round-2 both RATIFY.

## Verdict
RESOLVED — design converged over 2 rounds, implementation over 2 rounds, all lenses RATIFY with zero open findings. The assigned dissenter (python-fidelity) carried the load across both phases by **running ruff/mypy/black against every cited anchor and observed diagnostic** — confirming the pack enshrines the language's own published standards faithfully (real codes, correct PEP attributions, observed-not-asserted diagnostics, good snippets that pass), which is the strategic requirement. Anchored on Ruff as the consolidated community registry (subsumes flake8/pylint/isort/pyupgrade), with line-length treated as project-configurable and the type-checker precondition + opt-in-rule-set caveats explicit. Cursor Bugbot + CI: pending on PR open (Bugbot skip/NEUTRAL satisfies the check half for this doc/pack PR per the recorded review-gate policy).
