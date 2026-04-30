# Findings Log Format

The findings log lives at `docs/bugbash/<target>.md`. It is a single growing markdown file — items append in numbered order, never reorder, never drop. This makes diffs reviewable and lets `/issue` hand-offs cite stable item numbers.

## Top-level shape

```markdown
# Bugbash: <target name>

**Target path:** `<path/to/component>`
**Started:** YYYY-MM-DD
**Last updated:** YYYY-MM-DD
**Experts:** kubernetes-specialist, security-specialist, sei-network-specialist, platform-engineer, product-manager

## Summary

| Severity | Count | Launch-blocking |
|----------|-------|-----------------|
| Critical | 2     | yes             |
| High     | 4     | yes             |
| Medium   | 7     | tracked         |
| Low      | 3     | tracked         |

## Findings

[Item entries, one per finding, in append order — see per-item shape below.]

## Launch Verdict

[Populated at convergence — one block per expert with their ship-it / conditional / don't-ship call.]
```

The Summary table and Launch Verdict section are written by the orchestrator at convergence; while the run is in progress, only the Findings section grows.

## Per-item shape

Every finding follows this exact structure. Section order is fixed; section headers are exact.

```markdown
## Item N: <short imperative title>

### Overview

#### Experts involved

- **Finder:** <agent-name>
- **Challenger:** <agent-name> — verdict: <confirm | downgrade from X | refute (note: refuted findings are NOT logged here)>
- **Severity:** Critical | High | Medium | Low

### Scenario

<2–5 sentences. Describe the conditions under which the bug manifests — what input, what state, what concurrency, what cluster posture. Concrete enough to reproduce.>

### Impact / Risk / Priority

<2–4 sentences. What goes wrong when this fires? Who notices? Is the failure silent or loud? Does it corrupt state or just degrade? Anchor the severity choice in the rubric — see references/severity-rubric.md.>

### Issue

<The body — observed behavior, expected behavior, where in the code (file:line). What's missing or wrong. Cite the interface contract or registry entry if the finding is an interface violation.>

**Fix sketch:**

<1–3 paragraphs or a short bulleted plan. Not a patch — a sketch. The fix happens outside this skill.>

**Test coverage:**

<What test would have caught this. Unit / integration / e2e level. Specific assertions or scenarios. If there's an existing test that should have caught it but didn't, name it and explain why it missed.>

**Metric:** <only present when operationally critical>

<Skip this subsection entirely for most findings. Add it only when the failure mode is one the team needs ongoing visibility into in production — e.g., a silent data-corruption path, a retry budget that could exhaust, a queue depth that could runaway. State the metric name, type (counter / gauge / histogram), and the alert threshold that would page someone. If a finding's failure is loud (panic, error log, failed reconcile), no metric is needed — existing observability catches it.>
```

## Rules of the format

- **No reordering.** Item N stays Item N forever. If a finding is later refuted in a follow-up bugbash, append a strikethrough note; don't delete or renumber.
- **No silent edits.** When updating a finding (severity changed at challenger pass, fix sketch refined), the orchestrator may rewrite the item, but the item number is permanent.
- **Exact headers.** `Overview`, `Experts involved`, `Scenario`, `Impact / Risk / Priority`, `Issue`. Tooling and `/issue` will key off these.
- **Severity is not optional.** Every item has one. See the rubric.
- **Metrics are judicious.** The default is no metric. Add one only when the failure is silent and operationally consequential. If three items in a target want a metric, at least two of them probably don't actually need one — challenge the metric in the next pass.
- **Fix sketch is not a patch.** Bugbash is read-only. The sketch describes the shape of the fix, not the lines of code. Whoever picks up the issue does the actual implementation.
- **Cite file:line in the Issue body.** Concrete pointers turn the artifact into a working document for the eventual fixer.

## Refuted findings

Refuted candidates do NOT appear in the findings log. They are recorded only in `.bugbash/<target>.yaml` under `pass-N.refuted:` with the challenger's reasoning, so a future run can avoid re-surfacing the same false positive. The findings log stays clean — only confirmed findings.

## Launch Verdict shape

Populated at convergence (procedure step 5). One block per expert in the slate:

```markdown
## Launch Verdict

### kubernetes-specialist — ship-it

<one-sentence rationale>

### security-specialist — conditional

Ship-it if the following are closed: Item 3, Item 7, Item 12.

### product-manager — ship-it

<rationale>

### platform-engineer — don't-ship

<explanation of the structural issue not captured in any single finding>
```

The skill is done when every verdict is ship-it, OR every verdict is ship-it / conditional AND every named finding across conditionals is Critical or High. See SKILL.md step 5.
