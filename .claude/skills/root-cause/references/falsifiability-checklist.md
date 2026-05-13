# Falsifiability Checklist

A hypothesis you cannot cleanly falsify is a hunch. For every candidate cause, write the observation that would force you to drop it — before you go looking for confirming evidence. This is the single most effective guard against confirmation bias.

## The Popper test, applied to incidents

For each hypothesis, answer in writing:

1. **What observable would I expect to see if this hypothesis is TRUE?** (the confirmation signal)
2. **What observable would I expect to see if this hypothesis is FALSE?** (the falsification signal — the load-bearing one)
3. **Are these two observables distinguishable?** If the same data could be interpreted either way, the hypothesis is unfalsifiable as stated. Sharpen it.
4. **Can I obtain the falsification observable right now?** If not, name what blocks you (access, missing instrumentation, time window already passed).

The falsification signal is the rigor check. A hypothesis that "predicts" both "logs will show errors" and "logs will not show errors depending on timing" predicts nothing.

## Patterns

### Good (falsifiable)

> **Hypothesis:** The SeiNodeDeployment controller is reconciling on every status update, creating a hot-loop and starving other reconciles.
>
> **Confirmation:** `kubectl logs <controller>` shows reconcile entries for SeiNodeDeployment `X` at a rate > 1/sec sustained for > 30s, with each entry's elapsed time < 50ms.
>
> **Falsification:** Same logs show reconcile entries at < 1/sec, OR controller logs show no reconciles at all for SeiNodeDeployment X during the incident window.
>
> **Obtainable now?** Yes — controller logs are retained 24h, incident was 2h ago.

### Bad (unfalsifiable)

> **Hypothesis:** Something is wrong with the network.

This predicts nothing specific. Network is too broad — DNS, NodePort routing, NetworkPolicy, peer P2P, JSON-RPC ingress, sidecar proxy, AWS VPC routing, BGP. Sharpen to a specific mechanism (e.g., "the sidecar's upstream connection pool exhausted") before testing.

### Bad (confirmable but not falsifiable)

> **Hypothesis:** A race condition between the reconcile loop and the cache invalidator is causing stale reads.
>
> **Confirmation:** Stale reads observed.
> **Falsification:** ?

The hypothesis predicts an effect ("stale reads") that we already know is happening — that's why we're investigating. Without a specification of *when* the stale reads should appear (which interleaving, under what cache state), no observation can falsify it. Either drop the hypothesis or specify the interleaving testably (e.g., "stale reads should appear only on the second of two rapid-fire requests to the same key within X ms of a cache invalidation").

## Counterfactuals — when they're rigorous, when they're rationalization

Counterfactual reasoning ("would the effect have occurred without X?") is causal-inference-grade rigorous **only when the counterfactual is testable**.

Testable counterfactuals (good):

- **Rollback the deploy.** If the effect disappears, the deploy is implicated. If it persists, the deploy is not the (sole) cause.
- **Replay the traffic at an earlier commit.** Same logic, with a synthetic load.
- **Bisect a git range.** Reduces an "X happened around when Y changed" suspicion to a specific commit.
- **Restart with the suspect config / flag changed.** Single-variable experiment.

Untestable counterfactuals (rationalization):

- "If the on-call had paged sooner, this wouldn't have happened." Hindsight bias. The relevant question is whether the system permitted the on-call to act when needed, not whether they were faster.
- "If we had better monitoring, we'd have caught this." May be true and worth fixing, but it's a recommendation, not a causal finding.
- "If the deploy had been canaried, this wouldn't have happened." Possibly true, but it's a process recommendation, not a mechanism for this incident.

When the counterfactual is untestable, label it explicitly as **speculation** in the output. Don't smuggle hindsight into a causal claim.

## Multi-cause falsification

The skill enforces multi-cause output. Each contributing factor needs its own falsification:

| Factor | Confirmation observable | Falsification observable | Status |
|--------|-------------------------|--------------------------|--------|
| A | … | … | confirmed / falsified / inconclusive |
| B | … | … | … |
| C | … | … | … |

A factor that survives its own falsification attempt is a real contributor. A factor whose falsification was never attempted is a guess on probation — name it as such, do not promote it to "contributing factor."

## When you cannot falsify

Sometimes the observation you need is no longer available — retention expired, the system has restarted, the metric wasn't instrumented. Options:

1. **Reproduce the effect in a controlled environment** (harbor dev cluster, release-test harness) and run the falsification there. Cost: time. Benefit: you can iterate.
2. **Add the instrumentation now, wait for the next occurrence.** Cost: incident might not recur on a useful timeline. Benefit: rigorous when it does.
3. **Label the hypothesis as `unfalsifiable-given-data`.** Honest punt. The investigation surfaces it as a known gap, not as a finding.

Do not pretend you tested when you didn't. The cost of a fabricated falsification is a wrong root cause that the team ships against.
