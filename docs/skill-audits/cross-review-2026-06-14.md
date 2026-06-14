# Skill audit — `/cross-review` (2026-06-14)

Shape: discipline (anti-rubber-stamp). Read-only audit (PLT-534). Refactor via PR-with-council.

## Verdict
Mechanically + semantically clean (static: all 22 pass; third-person description with no Obra CSO smuggle; substantive guardrails; rationalization table + red-flags + Asch/Fagan grounding). Pressure-tested: the skill **held** the domain-only/single-pass/no-dissent scenario via Step 2 (idiomatic-reviewer mandatory for code), Step 3 (assigned dissenter — "without one you get consensus theater"), and the Halt Conditions ("never declare COMPATIBLE to be helpful"). The gaps are in the **slate axes, the resolution loop, and dispatch reachability**.

## Findings

| ID | Sev | Finding | Remediation |
|---|---|---|---|
| **XR-1** | block | **No prose axis in the slate.** Step 2 mandates `idiomatic-reviewer` for code but names no prose reviewer; selection is "boundaries only," so a domain-only slate over a prose-bearing artifact passes — the gap that let verbose comments through. | Step 2: add a `prose-steward` clause symmetric with idiomatic ("artifact carries prose a reader acts on — runbooks, dual-audience docs, dense comments → add prose-steward; rides in a Prose addendum"). State the invariant: "the slate must cover *every axis the artifact has* — boundaries + idiom (if code) + prose (if prose); a domain-only slate over an artifact with code or prose is incomplete." |
| **XR-2** | block | **Resolution is single-pass.** Rule 4 / Step 5 are "fixed or accepted"; the only "re-" verb is an unattributed "re-check." Nothing requires the **raiser** to confirm the fix closes their concern — the orchestrator can self-clear a peer's finding. | Rule 4 + Step 5: a finding addressed by an artifact change closes only when its **raiser re-reviews and confirms** (re-dispatch them), not on orchestrator judgment. Add halt: "a finding marked resolved without the raiser re-reviewing — re-dispatch; self-clearing a peer's finding is the rubber-stamp this skill exists to prevent." |
| **XR-3** | warn | **No doc-divergence category.** The five mismatch categories are all provider-vs-consumer runtime disagreement; "artifact contradicts its authoritative backing doc/spec/interface-registry" has no home. | Add a 6th category to `findings-protocol.md` — "Doc-divergence: contradicts the authoritative backing doc/spec/registry (canonical per CLAUDE.md); always blocking — source of truth wins." Reference from Step 4 + the dispatch brief. |
| **XR-4** | block | **Dispatch assumes shell reachability.** The brief passes "paths or pasted content" but never forbids a `gh`/shell pointer, and ignores Read-only reviewers (prose-steward has no Bash) — a PR-URL pointer is unreachable to them and halts the review (observed in real use). | `reviewer-dispatch.md`: add a Reachability clause — "pass on-disk absolute paths or pasted content, never a gh/git/shell command; reviewers may be Read-only. The orchestrator materializes remote artifacts (PR diffs) to disk before dispatch." Add as an anti-pattern. |

## Layering note (from the pressure pass)
The *iterate-to-converged-consensus across rounds* is a `/workstream`-gate concern; `/cross-review` owns the **rigorous single pass** + (XR-2) the **within-review raiser-confirmed resolution**. So: `/cross-review` gets the slate axes + the resolution-confirmation + doc-divergence + reachability; `/workstream`'s `design-approval` gate *composes* a converged-COMPATIBLE cross-review verdict, and `pr-sign-off` adds the bot-clean + doc/registry-alignment contract.

XR-1 and XR-4 are load-bearing together: adding prose-steward (no Bash) to the slate is what makes the reachability gap bite.
