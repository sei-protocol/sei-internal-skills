# Code safety & quality

Checklist for defensive, resource-bounded construction in code that must stay correct over its lifetime. Our-own-words items (TIGER STYLE and Power-of-Ten ideas are *summarized*, never restated); cite the authority (see `sources.md`).

**Scope note — this is the *systems-safety* slice only.** General code-craft that belongs to other lenses is deliberately out: idiom/naming/control-flow → `/idiomatic`; small-functions / check-every-return / test-size discipline → `/idiomatic` (the language pack's testing + error dimensions); code-review norms (small CLs, approve-to-unblock, review latency) → `/pr-quality` and the cited Google Engineering Practices. This file keeps only the rows that are distinctly about *systems safety under failure and load*.

**Scope note, continued — the one comment class a systems review still flags is the tombstone.** A comment that narrates a *deletion* — naming what a removed rule, permission, field, or block was, or why it was deleted (`// configmaps removed: leader election uses the Lease lock…`) — is in scope here not as comment *idiom* but as a **code-correct-over-time** hazard: stale removal-narration misleads a future operator into reading it as *current* intent and making a wrong call under pressure. Flag it by **citing the idiomatic pack's comment-discipline dimension** (Go `D10` / Rust `R11` — the no-tombstone bar: a deletion gets no tombstone, and there is no "load-bearing context for the deletion" exception); do **not** re-derive or restate the comment-discipline rule here (Rule 3 — cite the idiom lens, don't duplicate it). The rest of comment discipline (what-comments, doc placement, naming) stays entirely `/idiomatic`.

Sections: `## checklist` · `## severity model`. Anchors: TigerBeetle TIGER STYLE, NASA/JPL Power of Ten. Companion: `performance.md` (bounded concurrency), `api-design.md` (validate at the boundary).

## checklist

| id | principle | rule | cue | authority |
|----|-----------|------|-----|-----------|
| SAFE1 | Assert invariants, in pairs | State what must be true and check it; where it matters, confirm the same invariant from two independent paths so a single wrong assumption can't pass silently. Assertions catch the impossible before it corrupts state. | A critical invariant trusted but never checked; a computed result used with no sanity check. | TigerBeetle: TIGER STYLE |
| SAFE2 | Bound every loop and buffer | Give every loop, queue, and buffer an explicit upper limit so a bad input or runaway condition fails fast instead of spinning or growing without end. | A loop with no provable termination bound; an unbounded slice/queue that grows with input. | NASA/JPL: Power of Ten |
| SAFE3 | Fixed memory on the hot path | On hot or long-running paths prefer statically sized / pre-allocated memory over per-operation dynamic allocation — predictable footprint, no fragmentation or alloc-time surprise under load. | Repeated allocate/free in a steady-state loop; growth that depends on untrusted size. | TigerBeetle: TIGER STYLE |
| SAFE4 | Minimize dependencies | Each dependency is attack surface, supply-chain risk, and a version to track — pull one in only when it clearly out-earns the code you'd otherwise write. | A new third-party import to save a few lines; a heavy lib used for one trivial call. | TigerBeetle: TIGER STYLE |
| SAFE5 | Validate input at the trust boundary | Validate data crossing any trust boundary (args, network, files, config) at the point of entry, before use — assume nothing about shape, size, or range. | A request/file/env value used without range/shape checks; a size taken on faith. | TigerBeetle: TIGER STYLE |
| SAFE6 | No unbounded recursion in critical paths | In safety- or availability-critical code, avoid recursion (and indirection) whose depth isn't bounded, so the call graph stays statically traceable and the stack can't be blown by adversarial input. *(Advisory outside genuinely critical/hostile-input code — bounded recursion is fine in ordinary services.)* | Recursion with input-dependent depth on a path exposed to untrusted data. | NASA/JPL: Power of Ten |

## severity model

- **correctness/safety** — SAFE1 (unchecked invariant corrupts state), SAFE2 (unbounded loop/buffer → hang or OOM), SAFE5 (unvalidated input → corruption or injection). Defects, not preferences; lead with them.
- **consequence-under-load** — SAFE3 (alloc on the steady-state path), SAFE6 (unbounded recursion can blow the stack on adversarial input). Bite under volume or hostile input.
- **advisory** — SAFE4 (dependency hygiene). Real and worth enforcing; bundle, don't lead with it over a correctness finding.
