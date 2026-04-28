# Coral → Council Handoff Criteria

Coral is for lightweight iteration. Council is for full-ceremony work. When any of these match, stop and ask whether to hand off.

## Component / Interface Count

- ≥3 components affected → council (System tier likely)
- ≥2 interface boundaries touched → council
- Any new interface being *defined* (as opposed to consumed) → council

## One-Way Doors

Immediate handoff, regardless of size:

- Event signatures (topic hashes)
- Storage layout in upgradeable contracts
- CRD spec field names
- EIP-712 type hashes, domain separators
- Anything the repo's CLAUDE.md flags as irreversible

Script: "This touches a one-way door: [what]. Warrants full council process — I'll hand off unless you want to stay in coral with you gating it manually."

## Duration Signals

- User says "this is bigger than I thought" / "let's do this properly" / "we should design this"
- Work clearly won't complete in one session
- Experts start asking for design documents that don't exist

## Interface Changes

Anything requiring an interface-registry update (where a registry exists) → council. Coral doesn't edit the registry.

## What Coral Keeps Handling

- Single-component iteration with defined interfaces
- Debugging, troubleshooting, one-off questions
- Writing code against an existing LLD
- Second opinions from a single specialist
- Explaining existing design decisions
- Quick reviews of small changes

## Handoff Script Examples

- "Now touching 3 components (A, B, C) with interface changes between A→B. That's System tier — hand off to /council?"
- "You just mentioned changing the X event signature. One-way door. Hand off to /council for the gate, or handle it here with you gating manually?"
- "You said 'let's design this properly' — that's a council cue. Hand off?"
