# Cross-review ledger — validator-platform shadow result-export kit

Target:       PR #222 / branch feat/validator-platform-shadow-kit (.claude/skills/validator-platform/** + .claude/agents/platform-release-manager.md + .claude/skills/README.md; research of record: bdchatham-designs designs/shadow-result-export/research/seictl-shadow-result-export-tasks.md)
Class:        skill-package
Tier:         T2
Scope:        Add references/kit-shadow-comparison.md (supported `result-export` comparison flow against an already-shadow-ready node) + reconcile the result-sink generalization and the kit roster across the skill, the agent, and the catalog. config-patch/restart-seid deliberately out of scope (situational break-glass, not the supported flow).

## Round 1
Round:        1
State:        OPEN
OpenFindings: 2
Convergence:  split
Blinded:      no (sequential expert dispatch on the converged state 8206964)
Dissenter:    fidelity-dissenter

### Routing
- Slate: fidelity-dissenter (assigned dissenter — kit technical claims vs the verified research artifact + result-sink correctness/no-overclaim), prose-steward (steward, agent — dual-audience prose + 5-section kit-schema conformance)
- Reviewed at HEAD 8206964 (after the full result-sink generalization + kit-roster sync had rippled across ~7 files; Bugbot independently `completed/success` with zero live findings at this HEAD).

### Per-lens verdicts
| Lens | Verdict | Finding (evidence-bearing) | Resolution |
|---|---|---|---|
| prose-steward | NO-BLOCKERS | 5-section schema conformant; `Dimension N (name)` carried; result-sink sweep complete + identical-in-spirit across all 6 files; bi-audience legibility strong. 1 non-blocking divergence: kit-shadow-comparison.md:45 cited `profile §2` (the `:8443` execution model) for the shadow-readiness anti-pattern — broken provenance pointer. +1 advisory (param-density). | applied @ Round 2 (cite → §1 kit-local precondition + §seictl-shadow) |
| fidelity-dissenter | DISSENT | **BLOCK** (kit-shadow-comparison.md:37): "divergences counter increments at most once per process lifetime (the loop exits on first divergence)" — unverified overclaim (not in the research) + internally inconsistent with the walk-`last+1`→`latest` loop (line 35) and watermark-advances-from-`last+1` re-run safety (line 55). **MINOR** (same line): metric labels `{chain_id,pod_name,divergence_layer}` contradicted the verified research + every sibling file + the kit's own §4 cue, all `{divergence_layer}`. 7 other fidelity claims (params, typed-client gap, layer/watermark/outputs, migrationMode, scope-excludes-config-patch, result-sink consistency, roster) tried-and-could-not-refute → RATIFY. | resolved @ Round 2 |

### Verdict
OPEN — 1 correctness-grade block (the unverified early-exit behavioral claim) + 1 minor (metric-label overclaim) + 1 non-blocking prose cite. Fixed in Round 2 (commit 7ade844).

## Round 2
Round:        2
State:        RESOLVED
OpenFindings: 0
Convergence:  unanimous
Blinded:      no (re-review of the fixed state 7ade844)
Dissenter:    fidelity-dissenter (held — dissent resolved, re-ratified)

### Routing
- Re-dispatched the assigned dissenter (fidelity-dissenter) on HEAD 7ade844 to confirm both findings resolved with no new defect. prose-steward NO-BLOCKERS stands (its non-blocking cite suggestion was applied verbatim in the same commit).

### Per-lens verdicts
| Lens | Verdict | Finding | Resolution |
|---|---|---|---|
| fidelity-dissenter | RATIFY | Finding 1 resolved: early-exit overclaim gone, §2 (walks `last+1`→`latest`) + §5 (re-run advances watermark from `last+1`) now coherent and internally consistent. Finding 2 resolved: `seictl_shadow_divergences_total{divergence_layer}` at both line 37 and the §4 cue; no 3-label form remains; matches research. No new defect (line-45 cite now §1 + §seictl-shadow; only remaining `profile §` is the correct §6 at line 50). | — |
| prose-steward | NO-BLOCKERS | cite divergence applied; verdict stands. | — |

### Checks (declared check set)
| Check | State |
|---|---|
| Cursor Bugbot | completed/success — two consecutive clean re-reviews (8206964, 7ade844), zero comments created after the result-sink/roster sweep; all prior findings re-anchored stale + grep-verified resolved |
| build-smoke | completed/success |
| host-build-smoke | completed/success |
| lint-and-test | completed/success |
| verify | completed/success |
| wheel-smoke | completed/success |

### Verdict
RESOLVED — unanimous (fidelity-dissenter RATIFY + prose-steward NO-BLOCKERS), OpenFindings 0, all declared checks green, mergeable=CLEAN. Merge authorized under the operator's standing "merge on full alignment from the Coral experts and bugbot" for this kit workstream. Not a one-way door (doc/skill-package only; `result-export` itself is reversible per kit §5).
