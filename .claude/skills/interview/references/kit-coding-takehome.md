# Coding take-home kit — the mempool challenge

The first kit. Specializes the eight method dimensions for the take-home where the candidate designs and implements a transaction pool (mempool) that collects transactions, orders them by fee/timestamp, and periodically produces a block candidate.

## 1. What this format tests

A take-home is a **work sample** — among the strongest predictors of on-the-job performance (`sources.md` F3) — and it shows real code: structure, testing, documentation, and the candidate's own design choices, built without an interviewer hovering. It is **blind** to: how they think under live pressure, whether they actually understand code they may have generated with AI, and how they reason about productionizing beyond what they had time to build. The **scorecard reads the artifact; the verticals + live discussion cover the blind spots** — productionizing the system is the north star there.

## 2. The prompt being evaluated

The candidate was asked to **design and implement a transaction pool (mempool)**:
- `add`, `remove`, `update` transactions; each tx has at least an ID, fee/priority, sender, recipient, payload.
- Efficient retrieval in priority (or timestamp) order.
- A mechanism that selects transactions to form a block (timer or tx-count trigger).
- Basic checks — gas or tx-count per block.
- A block **header** with the previous block's hash, a timestamp, and minimal metadata.
- An interface or simple CLI to simulate incoming transactions and block production.
- Tests or a demo that inserts mixed-fee transactions, triggers block production, and shows the highest-priority transactions are selected and removed.

Optional bonuses (do **not** dock a submission for skipping these — they are senior-signal and live-discussion fuel): a pre-add validation check; making the mempool faster via parallelization (and how to multithread it); disaster recovery on restart; a dynamic fee market that adjusts to mempool size.

Score the artifact against the deliverables above. The absence of a *bonus* (or any un-asked scope) is a **discussion vertical**, not a low score.

## 3. Dimensions in play

All eight apply. Per `sei-hiring-profile.md`, weight: **maintainability (2), testing (4), documentation (5) are table stakes**; **design/trade-offs (3) and production thinking (7) carry the senior signal**; **AI-collaboration (8) is usually a can't-assess from code alone → a live probe.**

## 4. Behavioral anchors (1–4)

Match the artifact to the observable behavior, not a number. 3 = hire bar, 4 = bar-raising.

| Dimension | 1 — poor | 2 — borderline | 3 — solid (bar) | 4 — bar-raising |
|---|---|---|---|---|
| **1 Correctness & verification** | core deliverables missing/broken (no working add-remove-update, or block selection ignores priority) | core works but edge cases break — empty pool, fee ties, duplicate IDs, remove-missing | all deliverables work incl. gas/tx-count caps; ties + empty + dup handled; highest-priority correctly selected **and removed** | also handles subtle cases (deterministic tie-break, `update` re-prioritizes, block respects gas **and** count) and self-found/fixed bugs visible in commits/tests |
| **2 Code quality & maintainability** | monolithic; tx/pool/block tangled; unclear names; dead code | works but flat structure; naming/abstraction rough; concerns partly mixed | clean separation (tx · pool · block), clear names, right-sized types; reads logically; idiomatic for the language | extensible without over-engineering (priority strategy / block-trigger swappable); reads like production, not a script |
| **3 Design & trade-off articulation** | structure chosen with no rationale; can't say why fee vs timestamp | choices (ordering, eviction, structure) unexplained; no alternatives weighed | justifies the priority structure (heap vs sorted slice vs map) for **this** workload; states gas/tx-count trade-offs; notes add/pop complexity | weighs alternatives against compounding costs (eviction under load, fee dynamics, O(log n) vs O(n)); names what they'd change at scale unprompted |
| **4 Testing discipline** | no tests, or one happy-path assertion | happy path only; corner cases missed; demo "shows it works" but doesn't verify ordering/removal | common + corner cases (empty, ties, caps, remove-missing); demo proves highest-priority **selected and removed** | property/stress coverage (ordering invariant over many txs), tests the gas/count boundary exactly; tests read as specifications |
| **5 Communication & documentation** | no README; assumptions implicit; commits uninformative | minimal README; thin commits; assumptions mostly implicit | README explains design + how to run + stated assumptions; commits narrate; "if I had more time" noted | docs make the trade-offs and productionization gaps explicit — the candidate pre-empts the discussion |
| **6 Handling ambiguity & scope judgment** | misreads prompt / builds the wrong thing; no assumptions | builds literally to the prompt; scope neither cut nor justified | states sensible assumptions (fee-vs-timestamp, what a "block candidate" is); cuts scope deliberately and says so | reframes under-specified parts as design decisions; surfaces the questions a real mempool raises (eviction, replace-by-fee, DoS) and scopes with rationale |
| **7 Production / operational thinking** | no awareness beyond happy path; breaks under any real load/failure | the asked gas/tx caps only; no thought to failure, restart, or scale | rejects invalid tx, enforces bounds; code/README shows awareness of scale/restart limits even if unimplemented | builds or articulates real operability — bounded memory, backpressure/eviction, restart/DR, observability — and frames the in-memory pool as a step toward a production node |
| **8 AI-collaboration quality** | code looks generated with no sign of understanding (inconsistent style, unused scaffolding) | used AI but accepted it uncritically; some generated code doesn't fit / is unused | AI used for well-scoped parts; candidate clearly owns the design; code is coherent (process often noted in README/commits) | evidence of strategic use — decomposition, verification of generated code, overriding the model where wrong; scales themselves with AI |

**Note on Dimension 8:** the artifact rarely proves AI-collaboration quality on its own. Default to **can't-assess** unless the commits/README show the process, and convert it to a live probe (see §5 vertical) — "walk me through where you used AI and how you decided the output was good enough."

## 5. Vertical seeds (productionization north-star)

The menu. Fire only the seeds that trace to **their** code or a deliberate omission; sharpen each against what they built. Lead with the one that best separates levels for this submission.

- **Ordering at scale** — *Hook:* their priority structure (sort-on-each-call / heap / map). *Ask:* keep it ordered and pop-highest at 100k–1M pending txs — what structure, what add/pop cost? *Strong:* heap / indexed priority queue / skip list, O(log n), handles re-prioritization on `update`. *Weak:* "re-sort the slice each time."
- **Consistent hashing & IDs** — *Hook:* how they hash/ID a tx and compute the block's prev-hash. *Ask:* collision handling? deterministic block hash across nodes? if you shard the mempool across nodes, how route a tx? *Strong:* collision-resistant hash, deterministic serialization, consistent hashing for sharding. *Weak:* ad-hoc/hand-rolled ID, non-deterministic hashing.
- **Concurrency & parallelism** (bonus) — *Hook:* single-threaded add/select; "CPU underutilized." *Ask:* how parallelize, and where's the contention? *Strong:* striped/sharded locks or lock-free structures, separate add vs select paths, per-shard pools. *Weak:* "wrap it in one global mutex" (serializes — the trap; probe whether they see it).
- **Disaster recovery** (bonus) — *Hook:* in-memory pool. *Ask:* node restarts and the mempool is erased — what do you do? *Strong:* WAL/journal, re-gossip from peers, accept bounded transient loss. *Weak:* "save to disk" with no consistency/throughput thought.
- **Dynamic fee market** (bonus) — *Hook:* static fee ordering. *Ask:* make fees adjust to mempool pressure (EIP-1559-style base fee)? *Strong:* base-fee curve on utilization, replace-by-fee, anti-spam. *Weak:* hand-wave.
- **Admission, validation & DoS** (bonus) — *Hook:* their validation check (or its absence). *Ask:* an adversary floods invalid / low-fee / duplicate txs — defenses? *Strong:* stateless + stateful validation, per-sender limits, min-fee floor, dedupe, bounded pool with eviction. *Weak:* "validate the fields."
- **Observability & availability** — *Hook:* no metrics/limits (mirrors the prompt's throttling/observability bonus). *Ask:* how do you know the pool is healthy, and protect node availability under a flood? *Strong:* pool-size / age / reject-rate metrics, admission throttling, eviction-by-age, backpressure to the gossip layer. *Weak:* "add some logs."

## 6. Level notes (L4/5 vs L6 on this task)

- **L4 / L5:** delivers a correct, clean, well-tested mempool to spec; when prompted, can reason through one or two verticals sensibly.
- **L6:** treats "build a mempool" as "build the admission + ordering + availability layer of a node" — **unprompted**, reasons about eviction policy, the fee market as an anti-spam mechanism, sharding/concurrency, DR, and the fact that a public mempool is a DoS surface. The take-home is a starting point they naturally extend toward production.
