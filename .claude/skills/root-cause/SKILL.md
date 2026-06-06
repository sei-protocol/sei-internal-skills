---
name: root-cause
model: claude-opus-4-8
description: "Use when an engineer wants to understand a complex problem in the Sei platform stack (sei-k8s-controller, seictl, sei-sidecar, sei-chain, release-test/qa-testing, platform/K8s) with disciplined, data-driven, multi-expert investigation — 'root-cause this', 'why is X breaking', 'this bug keeps coming back', 'investigate the X regression', 'what's actually causing X', 'why did the chain wedge', 'help me understand why X', '/root-cause'. Pulls the right `.claude/agents/` specialists, forces independent hypotheses before evidence, demands retrieved signals (not paraphrased), and refuses to declare a cause without a falsification attempt. Anti-triggers: NOT for live incident commander work — mitigate first, investigate after stabilization; NOT for fixing a known cause (just write the fix); NOT for greenfield design (use /coral or /council); NOT for pre-launch hardening (use /bugbash); NOT for capturing a finished design (use /design); NOT for problems outside the Sei platform stack — out of scope."
---

# Root Cause

Disciplined investigation of complex problems. Multi-expert. Hypothesis-first. Signal-gated. Multi-cause.

This skill exists because **incident investigation collapses under pressure**. Under time, urgency, sunk cost, or a senior voice anchoring on one theory, an agent's natural path is: latch onto the first plausible explanation, narrate what the data "probably shows," propose a restart, declare done. That path is fast. It is also how teams ship the same incident three times.

The skill refuses that path. It enforces a six-step loop where every step is gated on retrieved evidence, every hypothesis carries a falsification criterion, and every finding is multi-cause.

## Guardrails

This skill operates on **active investigation, not live mitigation**. Before any conclusion:

1. **Context check** — the calling repo MUST have a `.claude/agents/` roster (the skill is multi-expert by design). Without a roster, halt and ask the user to point at one or invoke from a repo that has one.
2. **Scope confirmation** — name the system under investigation and the observable effect in the first turn. If the user cannot state the effect ("things feel slow" without a measurable signal), halt and demand a sharper effect statement before generating hypotheses.
3. **Refusal conditions** — this skill will refuse to:
   - **Advance from hypothesis to conclusion without retrieved evidence.** Every signal cited must include the literal command + verbatim output, or be tagged `unverified`. Paraphrased logs are banned.
   - **Declare a cause without a falsification attempt.** Each candidate cause requires a stated observation that would force dropping it. If falsification was never attempted, halt and run it.
   - **Treat a mitigation as a cause.** Restart, rollback, scale-up, kill-pod are restoration actions. They explain the system is *restartable*, not why it broke. "Fixed by rolling back" is not a root-cause finding.
   - **File a single-expert conclusion on a cross-component incident.** If the effect spans ≥2 specialty boundaries (e.g., controller + Sei networking + cloud infra), the investigation must dispatch parallel, **blinded**, hypothesis-first experts. Serial single-expert work on a cross-component incident is a halt.
   - **Accept consensus theater.** If experts return aligned conclusions but their dispatch logs show they saw each other's work before committing, the consensus is invalid. Re-run with proper blinding.
   - **Fire on live incident response.** The skill is for *investigation discipline*, not incident command. If users are actively impacted and no mitigation is in place, redirect: mitigate first, investigate second. The skill belongs after the bleeding has stopped — or in parallel, by a separate person from the on-call.
   - **Fire on problems outside the Sei platform stack.** The in-scope systems are listed in the trigger description. For a problem in another domain, redirect to the right tool rather than forcing a low-confidence cross-domain investigation.

See `references/multi-expert-dispatch.md` for the dispatch contract, `references/falsifiability-checklist.md` for the falsification rules, `references/sei-k8s-signal-ladder.md` for the signal hierarchy, and `references/rationalization-table.md` for the full failure-mode catalog with citations.

## The Five Rules

These are non-negotiable. Every step in the procedure exists to enforce one or more.

1. **Signals before hypotheses.** Before you generate a hypothesis, you have looked at data. The first move is always `kubectl describe` / `seid status` / a bounded Prometheus query — not an opinion.
2. **Hypotheses before evidence.** Before you go looking for confirming data, write down at least two competing hypotheses. Evidence-gathering with one hypothesis in mind is confirmation bias wearing investigation clothes.
3. **Evidence is retrieved, not extrapolated.** If you cite a log line, you ran the command. If you cite a metric, you ran the query. "The logs probably show…" is fabrication, not evidence. Every signal carries provenance.
4. **Falsification before conclusion.** For every candidate cause, name the observation that would force you to drop it. Then try to obtain that observation. A hypothesis you cannot cleanly falsify is a hunch.
5. **Multi-cause, not root cause.** Complex systems fail through *combinations*. The output is a ranked set of contributing factors with confidence levels, mechanisms, and remaining uncertainty. If you find yourself writing one bullet labeled "the root cause," you have not done the work.

## Procedure

The orchestrator runs the loop. Specialists do their domain work. Both are bound by the Five Rules.

### Step 1 — Establish the effect

State the observable in measurable terms. Required:

- **What is happening** — the literal symptom, observable. Not "things are slow" — "p99 latency on `eth_call` rose from 80ms to 2.1s starting at 14:32 UTC on the four-validator chain in eng-bdchatham."
- **When it started** — wall-clock onset, scoped to the smallest window you can defend.
- **Baseline** — what was true before. If the system has no baseline, you do not know there's an incident; you have a feeling.
- **Blast radius** — which replicas, namespaces, regions, users. Localizes the search.
- **Mitigations already attempted** — record them as data, not as causes.

If any of these can't be stated concretely, halt. Demand the data first.

### Step 2 — Dispatch the expert slate

Read `.claude/agents/` from the calling repo. Pick the smallest set whose combined domains cover the affected surface. Typical Sei-platform mappings:

- sei-k8s-controller behavior → `kubernetes-specialist` + `sei-network-specialist`
- seictl CLI / SeiNode CRD field semantics → `kubernetes-specialist` + `sei-network-specialist`
- sei-sidecar (Waterway proxy, RPC routing) → `sei-network-specialist` + `network-specialist`
- sei-chain (CometBFT P2P, EVM, mempool, state sync) → `sei-network-specialist`
- release-test / qa-testing harness → `kubernetes-specialist` + `sre-engineer`
- platform/K8s manifests (Kustomize, IRSA, secrets) → `platform-engineer` + `network-specialist`

For any cross-cutting effect (latency, resource pressure, cardinality), add `observability-platform-engineer`. For attack-surface or trust-boundary effects, add `security-specialist`.

Dispatch contract (mandatory):

- **Parallel.** Specialists work simultaneously, not in series.
- **Blinded.** Each specialist commits their hypothesis list before seeing peers' outputs. The orchestrator does not summarize one expert's view into another's brief.
- **Assigned dissent.** One specialist is tagged red-team — their job is to argue against the emerging consensus and produce the strongest counter-hypothesis. If no one fills this role, you will get consensus theater.
- **Hypothesis-first brief.** Each specialist is asked: "Given the effect statement, what are the *top three* mechanisms that could produce this signature, ranked by likelihood, with a falsification observation for each?" Not "investigate this."

See `references/multi-expert-dispatch.md` for the full briefing template and anti-patterns.

### Step 3 — Collect independent hypotheses

Each specialist returns: 2–3 hypotheses, each with a proposed mechanism and a falsification observation. The orchestrator merges into a single de-duplicated table:

| # | Hypothesis | Proposed by | Mechanism | Falsification observation |
|---|------------|-------------|-----------|---------------------------|

No hypothesis is acted on until at least two are on the table. Single-hypothesis investigation is a halt.

### Step 4 — Retrieve evidence

For each hypothesis, run the falsification observation. **The specialist who proposed the hypothesis owns the retrieval** — they know the tool, the query, and the expected shape of the answer. Every retrieval produces a record:

```
Hypothesis #N
Command: <literal invocation, copy-pasteable>
Timestamp window: <ISO range>
Output: <verbatim excerpt, ≤30 lines or pointer to full output>
Interpretation: <what this shows, what it doesn't show>
Status: confirms / falsifies / inconclusive
```

The Sei/K8s "first five commands" ladder (see `references/sei-k8s-signal-ladder.md`) is the floor — for any K8s-shaped incident, those signals are pulled before any hypothesis-specific query. They localize the failure and reveal hypotheses you wouldn't have written.

### Step 5 — Build the causal chain

Surviving hypotheses (those not falsified) are assembled into a dependency graph. Required elements:

- **Temporality** — the proposed cause precedes the effect. Verify with timestamps.
- **Mechanism** — *how* the cause produces the effect, step by step. "X correlates with Y" is not a mechanism; "X exhausts the FD table, the next accept() returns EMFILE, the listener crashes, peer count drops, consensus stalls" is.
- **Consistency** — does the signal reproduce across replicas, regions, restarts? If only one instance shows it, name why.
- **Counterfactual** — would the effect have occurred without this cause? Use this only if you have a way to test it (rollback, replay, bisect, controlled rollout). Otherwise mark as speculation.
- **Contributing factors** — what else is on the path. The chain has multiple nodes; name them all.

### Step 6 — Commit to a ranked conclusion

Output shape (this is what the user sees):

```
Effect: <one sentence with measurable signal>

Contributing factors (ranked):
1. <factor> — confidence: high/medium/low — mechanism: <how it contributes>
2. <factor> — confidence: ...
3. <factor> — confidence: ...

Mechanism (causal chain):
<step-by-step from trigger to symptom>

Evidence retrieved:
- [<command>] confirms / falsifies hypothesis #N
- [<command>] confirms / falsifies hypothesis #M

Falsification attempts:
- For hypothesis #N: ran <obs>, result <outcome>, hypothesis status <kept/dropped>

Remaining uncertainty:
- <what we still don't know>
- <what observation would close it>

Recommended next actions:
- <action>, conditional on <signal>
```

If the investigation can't reach Step 6 — too many surviving hypotheses, evidence gaps the team can't close — that is a valid output. Say so explicitly: "investigation paused, three hypotheses surviving, need access to X to falsify further." A clean punt with stated obstacles beats a fabricated conclusion.

## Rationalization Table

Documented LLM failure modes during root-cause investigation. When you notice your own reasoning aligning with the left column, **stop**. The right column is the reframe.

| Excuse | Reality |
|--------|---------|
| "The most likely cause is X — let me verify by fixing it." | A fix is not a test of a hypothesis. Write the falsification criterion **before** applying the fix, or you're confirming, not testing. |
| "Based on the symptoms, the logs probably show…" | If you didn't run the command, you don't have evidence. Plausibility hallucination is the highest-severity failure mode here. Cite the tool call or tag it `unverified`. |
| "Restarting the pod fixed it, so the root cause was X." | A restart is evidence the system is *restartable*. Root cause must survive the next deploy. Mitigation ≠ explanation. |
| "You're right, that's probably it — let me investigate that angle." | When a human supplies a hypothesis, it's both information *and* a poisoned anchor. Generate two independent alternatives before acting on it. |
| "We're losing money every minute — let me skip ahead to the fix." | Urgency raises the cost of being wrong, which raises the required evidence per action. Skipping verification under time pressure is how teams ship the same incident twice. |
| "All five experts agree, so this is the cause." | Consensus is evidence only if each expert committed before seeing the others. If one expert saw the first's view, the others' agreement is sycophancy, not corroboration. |
| "There's not enough information to determine the root cause." | After two hypothesis cycles, force a ranked commitment with explicit confidence, not a punt. Paralysis is also a failure mode. |
| "It's probably a race condition." | A guess wearing a domain coat. State the race: which threads, which shared state, which observable interleaving. Otherwise drop it. |
| "We've seen this before — it's the usual culprit." | Pattern-matching is fast and frequently wrong. Treat as a hypothesis (good), not as a conclusion (bad). Run the falsification observation. |
| "The dashboards look fine." | Pre-aggregated metrics hide the connective tissue you need. Descend to raw events (logs, traces, request-level data) before declaring no problem. |

See `references/rationalization-table.md` for the full table with literature citations.

## Red Flags — STOP and Reset

Phrases that appear in your own reasoning when one of the rationalizations above is firing. Treat each as a halt signal:

- **"probably"**, **"likely"**, **"I'd expect"**, **"should be"**, **"would show"**, **"typically"** — when applied to *system state*, not to predictions about a future test.
- **Passive voice on data sources**: "the logs indicate", "metrics suggest", "it appears that" — without a citation.
- **"Let me just"**, **"quickly try"**, **"simply restart"**, **"go ahead and rollback"** — pre-action minimization.
- **"You're right"**, **"good point"**, **"that makes sense"** — in response to a hypothesis (not in response to retrieved evidence).
- **"Given the urgency"**, **"to save time"**, **"skipping ahead"** — explicit verification-skipping.
- **"The team agrees"**, **"all experts converge"**, **"clearly the cause"** — consensus laundering.
- **"Fixed by"** — as the *last* line of an RCA. Mitigation masquerading as explanation.
- **"More investigation needed"** — as a terminal state. Paralysis punt.

**All of these mean: Stop. Show the tool call. Generate an alternative. Or write the falsification.**

## Halt Conditions

Stop and report to the user if:

- The effect cannot be stated in measurable terms after two attempts to sharpen it.
- The calling repo has no `.claude/agents/` roster and the user can't point at one.
- After Step 3, only one hypothesis survives the merge — single-hypothesis investigation is consensus-of-one, equally invalid.
- Specialists' dispatch logs show they saw each other's outputs before committing — consensus theater. Re-run with proper blinding.
- A specialist returns a hypothesis with no falsification observation — re-dispatch with that requirement.
- Three retrieval attempts produce no verifiable evidence — the system isn't observable enough to investigate. Surface the observability gap as the finding; do not fabricate.
- The investigation runs longer than the user's stated bound without converging — report surviving hypotheses, evidence gaps, and what would unblock progress.

**Never auto-remediate without surfacing.** If the investigation reveals an obvious fix, propose it as a follow-up — do not apply it under cover of investigation.

## What This Skill Does Not Do

- **Live incident command.** Mitigate first, investigate after. The on-call's job is to restore service; this skill's job is to explain what happened, ideally not on the critical path.
- **Postmortem doc capture.** Out of scope by design (deferred). The skill produces a conversational summary; converting it into a `docs/postmortems/` artifact is a future companion skill.
- **Single-expert deep-dive.** If the problem is genuinely contained to one specialist's domain and the user knows it, just `/coral` that specialist directly.
- **Fix-it work.** Once the contributing factors are identified, fixing them is its own engagement. The skill ends at the ranked conclusion + recommended actions.
- **Cover domains outside the Sei platform stack.** Out of scope per the trigger description. A problem in another domain gets a redirect to the right tool, not a forced cross-domain investigation.

## Output (end-of-session summary)

One paragraph in chat: the effect, the top contributing factor(s) with confidence, the mechanism in one sentence, the strongest evidence, and the remaining uncertainty. Then a list of recommended next actions, each conditioned on the signal that justifies it.

Example:

> Effect: `eth_call` p99 latency rose from 80ms to 2.1s on the four-validator chain at 14:32 UTC. Contributing factors: (high confidence) sei-chain mempool re-check on every block after a SeiDB compaction stall — mechanism is compaction blocked the EVM state read path, mempool re-check fell behind, JSON-RPC backed up on state reads; (medium) the affected nodes had `--db-backend=goleveldb` from a stale ConfigMap. Evidence: `seid status` showed `latest_block_height` advancing but `catching_up: false` paired with a 30-second AppHash log gap; `kubectl describe pod` showed no resource pressure; SeiDB compaction logs showed the stall. Remaining uncertainty: whether the compaction was triggered by the snapshot interval or by a peer-replay storm — would need state-sync metrics from 14:25–14:32 to disambiguate. Recommended: (a) pin db-backend in the SeiNodeDeployment template, (b) add a compaction-duration alert, (c) defer the state-sync question to a focused follow-up.
