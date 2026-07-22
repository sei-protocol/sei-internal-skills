# EVM indexing & events kit

## 1. What this concern is

Agentic systems on Sei **react to on-chain events** — so reliably *receiving* events is a first-class concern, and **Sei's EVM diverges from L1 in ways that silently break a naive indexer**. The generic habit this kit corrects: building an Ethereum-style `eth_getLogs`/topic-filter indexer that assumes (a) all state changes come from EVM txns with logs, (b) a pending mempool, (c) reorgs needing confirmation depth, and (d) every tx produces a receipt. On Sei, **cross-VM activity is bloom-filtered out of EVM logs**, there is **no pending state**, finality is **instant** (no reorgs), and some txns produce **no receipt** — each is a way an agent misses or mis-reads an event. *Cited:* `x/evm/AGENTS.md`; sei-docs `evm/evm-parity/finality.mdx`, `evm/tracing/`, `evm/indexer-providers/`; `sources.md` §sei.

## 2. The pattern (how Sei does it)

- **Index confirmed blocks; finality is instant.** ~400 ms blocks, `tx.wait(1)` final, no reorgs — **no confirmation-depth / reorg-rollback machinery needed** (a real simplification). Follow `latest`; do **not** track `pending`. *Cited:* sei-docs `evm/evm-parity/finality.mdx`; profile §1.
- **Receive via WS subscription or polling.** `eth_subscribe(logs, {address, topics})` over the Sei WS endpoint (`wss://evm-ws.sei-apis.com` / testnet), or poll `eth_getLogs` per block at ~400 ms cadence. WS pending-tx subscriptions are **unreliable** (no pending state) — subscribe to `logs`/`newHeads`, not pending. *Cited:* sei-docs `evm/evm-parity/finality.mdx`, `evm/evm-parity/evm-compatibility.mdx`.
- **Account for cross-VM activity (the load-bearing trap).** Non-EVM txns (native/bank sends) mutate EVM-visible balances **with no EVM tx or log**. Cross-VM calls emit **synthetic receipts/logs**, but a **separate EVM-only bloom filter EXCLUDES CW-originated logs** — so a standard topic-filtered consumer can **miss** cross-VM events. If completeness matters, reconcile against state (or a Sei-aware indexer), don't trust an EVM-log-only view. *Cited:* `x/evm/AGENTS.md`; profile §8.
- **Handle Sei failure-receipt semantics.** A nonce-mismatch tx produces **no receipt**; other ante failures produce a **status-0 receipt** (nonce incremented). "No receipt" ≠ "never happened". *Cited:* `x/evm/AGENTS.md`; profile §7.
- **Deep reconstruction via tracing.** `debug_traceTransaction`/`debug_traceBlockByNumber`/`debug_traceCall`/`debug_traceStateAccess`, `callTracer` + custom JS tracers — for state-access reconstruction beyond what logs carry. *Cited:* sei-docs `evm/tracing/`.
- **Indexer options (self-host vs managed):** The Graph (subgraphs), Goldsky (subgraphs + Mirror streaming), Envio (HyperIndex/HyperSync), Moralis, GoldRush/Covalent, Sim by Dune. Pick by latency/control needs; a Sei-aware indexer handles the cross-VM caveat better than a hand-rolled log scanner. *Cited:* sei-docs `evm/indexer-providers/`.
- **Design events for consumption (contract-author side).** Emit well-structured events with `indexed` topics for the fields agents filter on; an event signature is a **one-way door** (topic hash is permanent once indexers depend on it). *Cited:* `sources.md` §solidity; profile (one-way doors).

## 3. Anti-patterns / failure modes

- **EVM-log-only completeness assumption.** Cue: an indexer/agent that derives balances or "all activity" purely from `eth_getLogs`/topic filters. Rewrite: account for cross-VM state changes (no EVM log) and bloom-excluded CW-origin logs — reconcile against state or use a Sei-aware indexer. **[the dominant missed-event bug]** *Cited:* profile §8.
- **Pending-mempool logic.** Cue: subscribing to pending txns, pending-nonce arithmetic, "speed up when seen in mempool". Rewrite: no pending state on Sei — act on confirmed `latest`. *Cited:* profile §1.
- **Confirmation-depth / reorg handling.** Cue: "wait 12 blocks", reorg-rollback code. Rewrite: instant finality — `tx.wait(1)` is final; delete the depth logic. *Cited:* profile §1.
- **Assuming every tx has a receipt.** Cue: an indexer that errors/stalls when `eth_getTransactionReceipt` returns null. Rewrite: handle no-receipt (nonce-mismatch) and status-0 (ante failure) cases. *Cited:* profile §7.
- **Under-indexed events (contract side).** Cue: events with no `indexed` topics for the filtered field, or state changes with no event at all. Rewrite: emit `indexed` topics for agent-filterable fields; emit an event per state transition agents must observe. *Cited:* `sources.md` §solidity.

## 4. Review cues

- **Dimension 6 (testing & verification adequacy):** the indexing path is tested against Sei semantics — cross-VM state change (no EVM log), a status-0/no-receipt tx, instant finality (no reorg path); event shape asserted in Foundry tests. *Basis:* profile §1, §7, §8; `sources.md` §foundry.
- **Dimension 1 (security & exploitability):** an agent that acts on events does not trust an incomplete EVM-log-only view for security-critical decisions (cross-VM changes are invisible to it); confirmed-only, never pending. *Basis:* profile §8, §1.
- **Dimension 4 (external-call & value handling):** events emitted per state transition agents consume; `indexed` topics present for filtered fields. *Basis:* `sources.md` §solidity.

## 5. One-way doors in this concern

- **Event signatures** (name + arg types → topic hash) — permanent once indexers/agents subscribe to them; changing one silently breaks every consumer. Flag for human approval; version via a new event, don't mutate.
- **The set of `indexed` topics** on a depended-on event — changing indexing breaks existing topic filters; treat as part of the event's one-way-door contract.
