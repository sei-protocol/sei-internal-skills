# Rationalization Table (with literature citations)

The skill's rationalization table is the highest-leverage persuasion mechanism — it pre-loads counters to the rationalizations the agent is most likely to generate. Each row below is documented in the published LLM-RCA / multi-agent literature.

## The table

| # | Rationalization | Documented as | Reality |
|---|-----------------|---------------|---------|
| 1 | "The most likely cause is X — let me verify by fixing it." | Premature closure / anchoring. Persistent across CoT reasoning in ~23% of clinical-LLM traces (medRxiv 2025). | A fix is not a test of a hypothesis. Write the falsification criterion **before** applying the fix. |
| 2 | "Based on the symptoms, the logs probably show…" | Plausibility hallucination. Documented in KubeGPT, Arize field analysis ("confident liars"), FSE'24 RCA paper. The same pathology as code-completion hallucination, transplanted to ops. | If you didn't run the command, you don't have evidence. Cite the tool call or tag `unverified`. |
| 3 | "Restarting the pod fixed it, so the root cause was X." | Symptom-as-cause via mitigation theater. Google SRE explicitly distinguishes mitigation (drain/rollback/restart/scale) from root cause. | A restart is evidence the system is *restartable*. Root cause must survive the next deploy. |
| 4 | "You're right, that's probably it — let me investigate that angle." | Sycophancy / authority capitulation. Anthropic's own Petri evals + Sharma et al. (2023). Reduced 70–85% in Opus 4.5 but not eliminated. | When a human supplies a hypothesis, generate **two independent alternatives** before acting on it. |
| 5 | "We're losing money every minute — let me skip ahead to the fix." | Time-pressure compliance. Temporal-awareness paper (arXiv 2601.13206): LLMs translate urgency into skipped verification, not better strategy. StepFly: agents "express intention to proceed but fail to invoke the necessary tools." | Urgency raises the cost of being wrong, which raises the required evidence per action. |
| 6 | "All five experts agree, so this is the cause." | Consensus theater. MAST (Cemri et al., 2025) catalogs sycophancy among agents, role drift, evidence fabrication to support emergent consensus. ICLR'25: multi-agent debate often fails to beat single-agent baselines. | Consensus is evidence only if each expert committed **before** seeing the others. Otherwise it's one well-chosen anchor. |
| 7 | "There's not enough information to determine the root cause." | Paralysis / excessive-caution escape hatch. FSE'24: 66% of ReAct wrong answers were "insufficient information" punts vs. 18–32% for weaker baselines. | After N hypothesis cycles, force a ranked commitment with explicit confidence, not a punt. |
| 8 | "It's probably a race condition." | Domain-flavored guess. The race is not specified — which threads, which shared state, which interleaving. | State the race or drop it. Unspecified race conditions are unfalsifiable. |
| 9 | "We've seen this before — it's the usual culprit." | Pattern-matching as conclusion. Documented in incident-response cognitive-bias literature (cybersecurity-magazine, Allspaw/Woods STELLA report). | Pattern-match is a strong source of hypotheses, weak source of conclusions. Run the falsification observation. |
| 10 | "The dashboards look fine." | Aggregate-blind: pre-aggregated metrics hide tail behavior. Charity Majors: "metrics permanently discard the connective tissue." | Descend from aggregates to raw events (logs, traces, request-level) before declaring no problem. |
| 11 | "Human error — the on-call should have noticed sooner." | Hindsight bias. Cook ("How Complex Systems Fail"), Dekker ("human error is the starting point of investigation, never the conclusion"). | Treat operator action as a symptom and ask why the system permitted/encouraged that action. |
| 12 | "The fix is obvious — let me apply it and move on." | Mitigation collapsing investigation. Indistinguishable from failure mode #3 in effect; different in framing (proactive vs. reactive). | The fix is a separate engagement. Investigation ends at the ranked conclusion + recommended actions, not at applying them. |

## Red-flag phrase list

Pair the table with phrase-level halt signals — surface patterns that indicate one of the rationalizations is firing. Apply them to the agent's own output (and to specialists' returned briefs).

### State-of-system claims without provenance

When applied to *system state* (not to predictions about a future test):

- "probably"
- "likely"
- "I'd expect"
- "should be"
- "would show"
- "typically"
- "appears to"
- "seems like"

These predict, they don't observe. Demand the tool call.

### Passive voice on data sources

- "the logs indicate"
- "metrics suggest"
- "it appears that"
- "the data shows" (without a citation)

Passive voice hides who looked. If no one looked, no one knows.

### Pre-action minimization

- "let me just"
- "quickly try"
- "simply restart"
- "go ahead and rollback"
- "real quick"

These minimize the action so the agent doesn't have to defend skipping verification.

### Authority capitulation

- "you're right"
- "good point"
- "that makes sense"

In response to a hypothesis (rather than to retrieved evidence). The pattern is fine in response to data; it's a smell in response to a claim.

### Time-pressure laundering

- "given the urgency"
- "to save time"
- "skipping ahead"
- "we don't have time to"

Explicit verification-skipping wearing a justification. The justification doesn't change the math: skipped verification raises the probability of being wrong.

### Consensus laundering

- "the team agrees"
- "all experts converge"
- "we all see it"
- "clearly the cause"

Sycophantic alignment masquerading as evidence. Only valid if each opinion was committed independently.

### Closure smells

- "fixed by …" as the *last* line of an RCA — mitigation masquerading as explanation.
- "more investigation needed" as a terminal state — paralysis punt.
- "the root cause is X" with one bullet and no contributing factors — single-cause reduction of a multi-cause failure.

## Citations

- Roy et al., *Exploring LLM-based Agents for Root Cause Analysis* (FSE'24). https://arxiv.org/html/2403.04123v1
- Cemri et al., *Why Do Multi-Agent LLM Systems Fail?* (MAST, 2025). https://arxiv.org/pdf/2503.13657
- Arize, *Why AI Agents Break: A Field Analysis of Production Failures*. https://arize.com/blog/common-ai-agent-failures/
- Anthropic, *Claude Opus 4.5 System Card* (sycophancy evals). https://www.anthropic.com/claude-opus-4-5-system-card
- Sharma et al., *Towards Understanding Sycophancy in Language Models*. https://arxiv.org/pdf/2310.13548
- medRxiv 2025, *LLM Reasoning Does Not Protect Against Clinical Cognitive Biases*. https://www.medrxiv.org/content/10.1101/2025.06.22.25330078v1.full
- npj Digital Medicine 2025, *Cognitive bias in clinical large language models*. https://www.nature.com/articles/s41746-025-01790-0
- *Real-Time Deadlines Reveal Temporal Awareness Failures in LLM Strategic Dialogues*. https://arxiv.org/html/2601.13206v1
- *Agentic Troubleshooting Guide Automation (StepFly)*. https://arxiv.org/html/2510.10074
- Google SRE Book, *Effective Troubleshooting*. https://sre.google/sre-book/effective-troubleshooting/
- Google SRE Book, *Monitoring Distributed Systems*. https://sre.google/sre-book/monitoring-distributed-systems/
- Google SRE Workbook, *Incident Response*. https://sre.google/workbook/incident-response/
- Cook, *How Complex Systems Fail*. https://how.complexsystems.fail/
- Dekker, *The Field Guide to Understanding Human Error*. https://sidneydekker.com/books
- Charity Majors, *Observability is a Many-Splendored Definition*. https://charity.wtf/2020/03/03/observability-is-a-many-splendored-thing/
- Cindy Sridharan, *Monitoring and Observability*. https://copyconstruct.medium.com/monitoring-and-observability-8417d1952e1c
- Brendan Gregg, *The USE Method*. https://www.brendangregg.com/usemethod.html
- KubeGPT (illustration of fabricated kubectl output). https://kubegpt.org/
