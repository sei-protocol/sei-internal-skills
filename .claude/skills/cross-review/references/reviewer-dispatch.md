# Reviewer Dispatch Contract

How to dispatch the independent reviews in Step 3. The goal is *independent* judgment — the engine of effective review (Fagan inspection's individual-preparation phase; a single independent dissenter restores judgment against a conforming majority, per Asch).

## The contract

- **Independent / blinded.** Each specialist reviews the same artifact without seeing any other reviewer's assessment. Do not paste or paraphrase one reviewer's findings into another's brief. If reviewers must run sequentially (tooling limits), still withhold peers' outputs until all have committed.
- **Assigned dissent.** Tag exactly one reviewer as red-team. Their explicit job: argue the artifact is wrong, find the strongest objection, name the boundary most likely to break. Without an assigned dissenter, multi-agent review collapses into agreement (consensus theater / sycophancy).
- **Provider + consumer coverage.** For each boundary, the brief goes to at least the provider-domain specialist and the consumer-domain specialist. The provider defines the interface; the consumer says whether it can actually adapt to it.
- **Evidence-bearing findings only.** The brief demands the specific contract / field / signature / line behind every finding. Reject "looks good."
- **Reachability.** Pass on-disk **absolute paths** or **pasted content** — never a `gh`/`git`/shell command as the pointer to the artifact. Reviewers may be Read-only (e.g. `prose-steward` has no Bash), so a `gh pr diff` pointer is unreachable to them and halts the review. The orchestrator materializes any remote artifact (a PR diff, a fetched doc) to disk *before* dispatch.

## Brief template

Dispatch each reviewer (via the `Agent` tool with the specialist as `subagent_type`) with:

```
You are cross-reviewing a produced artifact. Review it INDEPENDENTLY — do not
assume other reviewers' conclusions.

ARTIFACT: <paths or pasted content — the actual work, read it in full>
BOUNDARIES YOU OWN OR CONSUME: <the interfaces relevant to this specialist>
YOUR ROLE: <standard reviewer | RED-TEAM: argue it's wrong, find the strongest objection>

For each boundary, return findings in this shape — one row per boundary:
| Boundary | Status (COMPATIBLE / MISMATCH / MISSING) | Evidence (the specific contract/field/signature/line) | Why |

Also list anything the artifact ASSUMES but does not state (a MISSING is a
finding, not a nitpick), and anything that CONTRADICTS its authoritative backing
doc/spec/interface-registry (a doc-divergence finding — always blocking, source
of truth wins). Do not return bare approval — every COMPATIBLE must cite what you
checked. If you cannot assess a boundary from the artifact given, say so and name
what you'd need.
```

## Anti-patterns

- **Summarizing peers into the brief.** "The platform-engineer thinks X — do you agree?" destroys independence. Don't.
- **One brief, all reviewers, shared thread.** They'll anchor on whoever answers first.
- **Skipping the dissenter** because "everyone will probably agree." That's the prediction the dissenter exists to test.
- **Briefing "take a look"** instead of the structured, evidence-demanding brief above. Vague briefs produce vague approvals.
- **Manufacturing reviewers** to look thorough. If one specialist genuinely covers the surface, run a single-reviewer pass and label it as such.
- **Passing a shell/`gh`/`git` pointer as the artifact.** A Read-only reviewer can't run it; the review halts. Materialize the diff/doc to disk and pass the path or the pasted content.
