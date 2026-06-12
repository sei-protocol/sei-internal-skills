# Gov-Ops — Rollback

The skill *executes* only "submit the pre-staged revert proposal," which is re-entry into
SKILL steps 2–6. There is no fast rollback.

## Revert proposal

Staged in pre-flight (SKILL step 1) by snapshotting the **current live value** of each target
`subspace/key` **before** the change applies — byte-exact, so the revert can't re-introduce an
encoding error. The revert is a normal param-change back to those values and **faces a full
voting cycle** (submit → fan votes → tally) under the same `voting_period`. Rollback is not
immediate; set incident expectations accordingly.

## Chain-halted (consensus-param changes)

If a consensus-affecting change (e.g. `baseapp/TimeoutParams`) halts the chain, **governance
cannot recover it** — there is no block production to process the revert. Recovery is
operator-side:

1. Apply a timeout/config override in the validators' node config.
2. Coordinated restart of a quorum of validators.
3. **Never scale a validator's replicas above 1** — the same signing key on two pods is a
   double-sign / tombstone (see the `validator-NN.yaml` `replicas: 1` warning).

## Wrong-content / wrong-id

- Wrong proposalId filled → caught by the content/id confirm gate (step 3) before any vote.
- Voted on the wrong live proposal → votes are not retractable after the window; re-vote
  correctly only while its window is open.
- Wrong content submitted but PASSED → the `applied` assertion (step 6) catches it; trigger
  the revert while the window allows, else HALT loudly and escalate.
