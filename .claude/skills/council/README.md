# Council

> A scope-tiered specialist panel taking a design through phase-gated rounds.

![Council architecture diagram](assets/council.png)

Council is the full-ceremony engineering coordinator: it convenes a panel of specialist agents to design, cross-review, and implement multi-component work whose risk warrants formal process. Its central guarantee is that process weight matches scope — nothing dispatches without an identified tier (Product / System / Component / Feature), and no interface change ships without a dedicated `/xreview` phase whose MISMATCH and MISSING findings are resolved before proceeding.

| | |
|---|---|
| **Diagram archetype** | circular-cohort |
| **Visual grammar** | Design 14 · Grammar-version 14.1.0 |
| **Live diagram** | [Open in Lucid](https://lucid.app/lucidchart/84a42837-3712-4c4f-b8d5-3c4529d39aeb/edit) |
| **Skill** | [`SKILL.md`](./SKILL.md) |

## What it does

- Assesses scope first and sizes the process to it — four tiers from Feature (just implement) up to Product (decompose and design from scratch); coral-sized work is handed back to `/coral`.
- Dispatches specialists from the repo roster, sequentializing provider-before-consumer when they share an interface boundary and parallelizing only when they don't.
- Runs `/xreview` as its own phase against provider, consumer, and the interface source of truth, and halts until every MISMATCH and MISSING is resolved.
- Refuses to cross a one-way door — persisted schema/field names, public API contracts, on-disk or wire formats, signed or indexed identifiers — without explicit user approval, and reads session state fail-loud rather than silently starting fresh.

## Reading the diagram

This is a circular-cohort diagram: the specialist panel sits in a ring around the council coordinator, and the design under work moves through that ring in phase-gated rounds rather than down a single line. Each seat in the ring is one dispatched specialist; the arrows between them carry provider-to-consumer ordering, and the `/xreview` gate sits as the round's checkpoint that the work must clear — COMPATIBLE — before the next round begins. Read it as iteration with a gate, not a one-pass pipeline: the coordinator at the center owns scope-tiering, sequencing, and the one-way-door stop.
