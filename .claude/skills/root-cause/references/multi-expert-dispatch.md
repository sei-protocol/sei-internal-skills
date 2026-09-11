# Multi-Expert Dispatch Contract

Adding specialists to an investigation helps only when their work is **independent, blinded, and at least one of them has a mandate to dissent**. Loose dispatch produces consensus theater. In that multi-agent failure mode, experts converge sycophantically on whatever the orchestrator surfaced first, fabricating shared evidence to support it.

## Dispatch rules

These are non-negotiable for any cross-component investigation.

### 1. Parallel

Dispatch all specialists in a single message, multiple Agent tool uses. They start simultaneously, with no awareness of each other's existence.

Why: serial dispatch leaks the first specialist's framing into the second's brief. The second specialist's "independent" hypothesis is already anchored.

### 2. Blinded

The brief to specialist B does not contain specialist A's hypotheses, even paraphrased, even as "context." Each specialist sees:

- The effect statement (verbatim from Step 1)
- The signal-ladder data already retrieved (which is observational, not interpretive)
- Their own brief

Nothing else. No "the kubernetes-specialist thinks it might be X."

Why: research on multi-agent LLM systems consistently finds that exposing one agent's output to another collapses independence. Cemri et al. (MAST 2025) document sycophancy among agents as a top failure mode.

### 3. Hypothesis-first

The brief asks for hypotheses *before* asking for analysis. Specific template:

> Given this effect statement: `<verbatim>` and these retrieved signals: `<dump>`, what are the **top three mechanisms** in your domain that could produce this signature, ranked by likelihood? For each, write:
> - The mechanism (how it produces the effect, step by step)
> - The single falsification observation that would force dropping this hypothesis
> - The retrieval command (kubectl, prometheus query, grep target, etc.) that would get that observation
>
> Do not analyze further yet. Submit the hypothesis table and stop.

Why: asking for analysis up front gets you analysis of the first hypothesis the specialist generated. Asking for the table up front gets you a portfolio.

### 4. Assigned dissent

One specialist — typically the one with the least domain proximity to the most likely hypothesis — carries the red-team tag. Their brief adds:

> Your role on this investigation is to argue against the emerging consensus. Generate the strongest counter-hypothesis even if you find it less likely than the others. Identify what evidence the investigation would miss if it converges on the leading hypothesis prematurely.

Why: in clinical-LLM debate studies, *forced* disagreement reduces anchoring on the first plausible explanation. Without a designated dissenter, the multi-agent group converges sycophantically.

### 5. Merge after submission

The orchestrator collects all hypothesis tables, then merges into a single combined table. Specialists see the merged table only after they've submitted their own. Only then is collaborative discussion of the surviving hypotheses appropriate.

### 6. Proposer designs the gating retrieval; the orchestrator runs it

A specialist's brief asks for the *retrieval command* that would get each falsification observation — the proposer **designs** it. But the **orchestrator** runs the command whose output **advances a step transition** (SKILL.md Step 4). The gating evidence is therefore the orchestrator's own harness-captured tool result, not a record relayed back in a sub-agent's message. A relayed `Command`/`Output` block is forgeable prose; output the orchestrator caused is not. Specialists may pre-explore with their own retrievals, but the gate reads the orchestrator's. (This refines the older "the proposer owns the retrieval" rule for gate integrity. SKILL.md Step 4 notes the limit — it does not force the orchestrator to *enter* the gate.)

## Briefing template

For a single specialist:

```
You are <specialist-id> from this repo's .claude/agents/ roster.

Effect under investigation (verbatim, do not paraphrase):
  <effect statement from Step 1>

Signals already retrieved (read-only, observational):
  - <command>: <output excerpt>
  - <command>: <output excerpt>
  ...

Your task:

Generate your top 3 hypotheses for the mechanism producing this effect, ranked by likelihood **within your domain**.

For each hypothesis, return:
  1. Mechanism — step-by-step pathway from cause to effect.
  2. Falsification observation — the single observation that would force dropping this hypothesis.
  3. Retrieval command — the exact kubectl/prometheus/seid/grep invocation that would obtain that observation.

Constraints:
  - You have NOT seen other specialists' hypotheses. Do not speculate about what they might have said.
  - Do not propose fixes yet.
  - If a hypothesis falls outside your domain, note it briefly but spend your three slots on your own domain.
  - Return the table and stop. Further analysis is a separate brief.
```

For the red-team specialist, append:

```
You are assigned red-team for this investigation. After producing your three domain hypotheses,
also produce the **strongest counter-hypothesis** — a mechanism that would explain the effect
but that the rest of the slate is unlikely to propose. Argue against it being missed even if you
think it's lower probability.
```

## Anti-patterns

- **"Quickly summarize what the other experts found so far."** The summary is the contamination. Do not.
- **Sequential dispatch.** A → review → B → review → C. By the time C starts, A's hypothesis is the dominant frame. Always parallel.
- **"What do you think of hypothesis X?"** as a brief. This is a confirmation question, not a hypothesis-generation question. Specialists are sycophantic to direct questions of this form.
- **No designated dissenter.** Without it, you will get four-of-four agreement on a plausible-but-wrong hypothesis and no signal that you are in consensus theater.
- **Treating consensus as evidence.** "All five experts agree" is evidence only if each one committed before seeing the others. Otherwise it is evidence of one well-chosen anchor.
- **Skipping the merge step.** Do not privately pick a "winning" hypothesis from the submissions without surfacing the merged table to the specialists for joint discussion. Otherwise the specialists never get to falsify each other's work.

## When the .claude/agents/ roster is sparse

Some repos have only one or two specialists defined. Options in priority order:

1. **Use the generic `general-purpose` agent for the red-team slot.** Give it the full red-team brief — the generic agent forced to dissent is better than no dissent.
2. **Dispatch the same specialist twice with different framings** — once with the standard brief, once as a red-team. Less ideal (same training data, same priors) but preserves the discipline.
3. **Halt and ask the user.** If the roster cannot cover the affected surface and no acceptable substitute exists, surface the gap as a finding. Do not dispatch a single specialist who'll inevitably anchor.

The skill never accepts single-specialist conclusions on a cross-component incident. If the roster cannot support multi-expert dispatch, output the gap. Say "we do not have the experts to investigate this rigorously; here's the gap" — not a single-expert verdict.
