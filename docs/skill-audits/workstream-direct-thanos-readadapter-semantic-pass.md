---
skill: workstream
shape: procedural (with a discipline spine)
audited_on: 2026-06-14
auditor: bdchatham
audit_skill_version: 1
phase: audit-only
trigger: semantic-conventions pass after the direct-Thanos read-adapter change (commit 839704c, branch kit-readadapter-direct-thanos)
---

# Skill Audit — workstream (semantic conventions pass)

**Shape:** procedural with a discipline spine (checkpoint/guard enforcement)
**Audited:** 2026-06-14 by bdchatham
**Phase:** audit-only (judgment-based semantic pass; static checks already pass 22/22)
**Scope:** the read-adapter change in `references/signal-kit-telemetry.md` (Grafana datasource-proxy → direct Thanos via operator cluster access) and whether it upholds every convention without introducing drift across the kit.

## Summary

- **Block:**   0
- **Warn:**    1
- **Info:**    1
- **Skipped:** 0

The read-adapter change is internally clean in the file it touched: the new direct-Thanos section, the contract-1 parenthetical, and the deferred-future section are mutually consistent and the `grafana/mcp-grafana` mentions that remain are all legitimate (a *negative comparison* explaining why direct-Thanos was chosen, and a *deferred future* path — not stale current-state claims). All headline conventions hold: SKILL.md 162 lines (< 500, B1), description 1014 chars (< 1024, D2) and still accurate to behavior, references one-level-deep (R1), evals present with ≥1 happy + ≥1 halt and all sourced (E1/E2/E3/E4), catalog-listed (C1). **One drift finding:** the `guard-fails-closed-on-partial-read` eval scenario still narrates the guard polling "via the Grafana datasource-proxy" — a read path the change invalidated as the MVP. The eval's underlying mechanism (non-empty `warnings` ⇒ fail closed) is read-path-agnostic and still valid, so this is a stale-narration consistency defect, not a broken eval.

## Block findings

None.

## Warn findings

> Severity: **warn**. Address before broad rollout.

### E (eval validity) / R4 (reference consistency) — eval scenario narrates the superseded Grafana-proxy read path

- **Rule:** E4 / R4 — evals and references must stay internally consistent with the skill's current behavior; a reference (or eval scenario) that asserts a superseded model contradicts the changed adapter.
- **Source:** semantic.
- **File / line:** `.claude/skills/workstream/evals/evals.json:75` — eval id `guard-fails-closed-on-partial-read`.
- **Evidence:** The scenario reads: *"the agent polls the guard **via the Grafana datasource-proxy**."* The read-adapter change (commit 839704c) replaced the Grafana datasource-proxy with **direct Thanos Query Frontend via the operator's in-cluster kubectl access** as the MVP path, and demoted the Grafana datasource-proxy to the DEFERRED federated path (PLT-527). So the eval narrates a read path the change explicitly invalidated for the MVP the kit now ships.
- **Why it is only `warn`, not `block`:** the eval *tests* the partial-response-safe contract (HTTP 200 + non-empty `warnings` ⇒ inconclusive ⇒ abort). That mechanism is read-path-agnostic — it fires identically on the direct-Thanos API, which is in fact more directly warnings-native than the proxy. The compliance/forbidden signals and `halt_reason` are all still correct. Only the one mechanism phrase is stale.
- **Recommendation:** In the eval scenario string, change "the agent polls the guard via the Grafana datasource-proxy" to "the agent polls the guard via the direct Thanos Query Frontend read path" (or simply "polls the guard's Thanos read adapter"). No change to `expected` (signals/halt_reason already path-agnostic). Optionally update the `source` line if it should track the current adapter; the SRE finding it cites (#2a) is independent of read path, so leaving it is acceptable.

## Info findings

> Severity: **info**. Observations worth surfacing; not a quality bar.

### R4 (clarity nit) — "What this kit does not do" wording now ambiguous against the MVP

- **File / line:** `.claude/skills/workstream/references/signal-kit-telemetry.md:87` — *"No in-cluster direct-to-Thanos / tenancy proxy (deferred)."*
- **Observation:** This line predates the change and meant the deferred *federated scoped-tenancy* proxy. But after the change the MVP read path **is** in-cluster direct-to-Thanos, so the phrase "No in-cluster direct-to-Thanos" now reads as superficially contradicting the new Path-MVP at line 13. It is not an assertion of the old Grafana model (so not drift), only confusable wording.
- **Recommendation (optional):** reword to "No federated scoped-tenancy read proxy (deferred)" to disambiguate from the MVP's operator-dispatched in-cluster tunnel.

## Conventions explicitly re-verified (held after the change)

| Rule | Check | Result |
|---|---|---|
| B1 | SKILL.md ≤ 500 lines | PASS — 162 lines |
| D2 | description < 1024 chars | PASS — 1014 chars |
| D1/D5 | description third-person, "Use when…" | PASS |
| D3/D4 | anti-triggers + sibling redirects present | PASS |
| D6 | description does not summarize workflow into the body's job | PASS — routes triggers; the lifecycle stays in the body |
| R1 | references one level deep | PASS — 3 files directly under `references/` |
| R4 | references extend, do not duplicate, SKILL.md; internally consistent | PASS in `signal-kit-telemetry.md` (the changed file); the one consistency miss is in `evals/`, captured above |
| E1/E2 | evals.json parseable; ≥1 happy + ≥1 halt | PASS — 1 happy, 2 halt, 2 adversarial (5 total) |
| E3 | ≥3 entries | PASS — 5 |
| E4 | each eval sourced | PASS — all 5 trace to RED baseline / SRE cross-review findings |
| B2/B3/B8 | Guardrails + Halt Conditions present and substantive | PASS — 6 refusal conditions; guard fail-closed is Guardrail 6 |
| C1 | listed in `.claude/skills/README.md` | PASS |

### Drift sweep — no other spot assumes the old Grafana-proxy/token model

A full grep of the workstream kit for `grafana|datasource-proxy|bearer|service account|viewer|secrets manager|jwt|token|query-grafana|query_range` returned matches in exactly four places in `signal-kit-telemetry.md` and one in `evals.json`. Classifying each:

- **`signal-kit-telemetry.md:13`** (new Path-MVP) — direct-Thanos; explicitly "No Grafana, no bearer token, no Secrets Manager, no JWT." Correct.
- **`signal-kit-telemetry.md:14`** (warnings-native rationale) — names `grafana/mcp-grafana` only as the *rejected* option ("which drops `warnings`"). Legitimate negative comparison.
- **`signal-kit-telemetry.md:17`** (Future/DEFERRED) — Grafana datasource-proxy + OIDC/JWT/Viewer token framed as the deferred federated path (PLT-527). Legitimate; correctly demoted.
- **`signal-kit-telemetry.md:45`** (contract 1) — parenthetical updated by the change to "the reason the near-term adapter queries Thanos directly rather than via `grafana/mcp-grafana`." Correct.
- **`evals/evals.json:75`** — **the one true drift** (see Warn finding above): asserts the Grafana proxy as the *current* poll path.

No stale assumption was found in SKILL.md, `checkpoint-ledger.md`, or `composition.md`. The guard-primitive prose in SKILL.md references the kit only by name and by the read-path-agnostic fail-closed rule, so it needed no change.

## Pressure scenarios

Not run in this pass — this is the conventions/consistency semantic pass following a localized reference change, not a full re-pressure of the discipline spine. The two guard-focused evals (`guard-fails-closed-on-partial-read`, `guard-inconclusive-on-no-traffic`) already encode the load-bearing P7 scenarios for the telemetry kit and remain valid (modulo the narration fix above).

## Skipped checks

None.

## Verdict

**Conventions upheld; one drift finding (warn).** The change is sound and self-consistent in the file it edited; the only artifact left assuming the superseded Grafana-proxy model is the `guard-fails-closed-on-partial-read` eval's scenario narration. Substance of every eval is intact. Recommend the one-line eval edit before broad rollout.

## References

- Conventions catalog: `.claude/skills/audit-skill/references/conventions-catalog.md`
- Audit methodology: `.claude/skills/audit-skill/SKILL.md`
- Change audited: commit `839704c` on branch `kit-readadapter-direct-thanos`
- Design of record for the kit: `docs/designs/telemetry-signal-kit.md`; deferred federation: PLT-527
