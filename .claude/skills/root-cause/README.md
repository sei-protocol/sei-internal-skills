# Root Cause Analysis

> Telemetry and on-chain signals, deciphered by standard tools, drive a blinded cohort to a ranked root cause.

![Root Cause Analysis architecture diagram](assets/root-cause.png)

Root Cause is a disciplined, multi-expert investigation skill for complex problems in the Sei platform stack. It dispatches a blinded cohort of `.claude/agents/` specialists who commit competing hypotheses before seeing evidence, then gates every advance on signals the orchestrator retrieved itself. Its central guarantee: it declares no cause without a falsification attempt. It ranks no factor unless its gating command ran as a real tool call in the session.

| | |
|---|---|
| **Diagram archetype** | layered-cake (signal) |
| **Visual grammar** | Design 14 · Grammar-version 14.1.0 |
| **Live diagram** | [Open in Lucid](https://lucid.app/lucidchart/f03287f1-e36e-4eba-adec-e09f3e6dc814/edit) |
| **Skill** | [`SKILL.md`](./SKILL.md) |

## What it does

- Runs a six-step loop, with retrieved data gating each step. The steps: establish the effect, dispatch a blinded specialist slate, collect independent hypotheses. Then retrieve gating evidence, build the causal chain, commit to a ranked multi-cause conclusion.
- Forces hypotheses before evidence and assigns a red-team dissenter, so consensus only counts when each expert committed before seeing the others.
- The refusal that matters most: it will not advance from hypothesis to conclusion without a falsification attempt. It treats retrieved-not-extrapolated signals (literal command + verbatim output) as the only admissible evidence. Anything else carries the `unverified` tag.

## Reading the diagram

The layered-cake (signal) archetype stacks knowledge sources from the bottom up. Raw telemetry and on-chain signals sit at the base. The standard retrieval tools (`kubectl`, `seid status`, bounded Prometheus queries) that decipher them sit in the middle, feeding the blinded specialist cohort above. Read upward — each layer composes into the one over it. The arrows carry retrieved evidence, not paraphrase, toward the ranked root cause at the top. The stack is what enforces signals-before-hypotheses: nothing reaches the cohort layer until the layer beneath it has produced real, provenanced data.
