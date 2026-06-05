# Example: Prompt → Rendered Body

A worked example showing the conversation `/issue` runs and the body it produces. Anchored on the structure of [sei-protocol/sei-k8s-controller#137](https://github.com/sei-protocol/sei-k8s-controller/issues/137).

## User invocation

```
/issue
```

CWD: `~/work/sei-k8s-controller`. The user has been discussing a gap they noticed in PR #136 — that patching `signingKey` onto a Running validator is a silent no-op.

## Skill conversation

> **Skill:** Target repo: `sei-protocol/sei-k8s-controller` (from CWD). Confirm or override?
>
> **User:** confirm.
>
> **Skill:** What's the issue, in one descriptive title? (No `feat:`/`fix:` prefix unless the repo uses that — checked recent issues, it doesn't.)
>
> **User:** Detect spec drift on Running nodes for mid-life SigningKey patch
>
> **Skill:** **Problem** — what's wrong or missing, anchored on observable behavior?
>
> **User:** [pastes the context they had in chat about `buildRunningPlan` only detecting image drift]
>
> **Skill:** **Impact** — who's affected, how badly?
>
> **User:** Primary use case is zero-downtime migration cutover for pacific-1; future use case is validator mode switch.
>
> **Skill:** Suggested experts from `.claude/agents/`: `kubernetes-specialist` (controller plan logic), `platform-engineer` (CRD field surface). Adjust?
>
> **User:** add `sei-network-specialist` — there's a seid restart in the loop.
>
> **Skill:** Got it. Do you have a proposed mechanic, or leave it open for the assignee?
>
> **User:** [sketches the NodeUpdate plan reuse]
>
> **Skill:** Acceptance criteria? I can suggest defaults.
>
> **User:** sure.
>
> **Skill:** [proposes 5 checkboxes; user edits two]
>
> **Skill:** Out of scope?
>
> **User:** mode switch, demoting a validator, rotating secretName.
>
> **Skill:** References? PRs / docs that informed this?
>
> **User:** PRs #135, #136, LLD §11, .tide/validator-migration.md.
>
> **Skill:** Drafted. File as a GitHub issue, a Linear ticket, or print for paste?

## Rendered body

```markdown
## Problem

PR #136 shipped `validator.signingKey.secret.secretName` for the single-shot validator deployment use case — deploy a `SeiNode` with SigningKey set from creation and the controller mounts the Secret on the production StatefulSet pod.

What it explicitly does **not** support: patching `SigningKey` onto an already-Running validator. `buildRunningPlan` (`internal/planner/planner.go:621-628`) only detects image drift today, so a `kubectl patch seinode --patch '{"spec":{"validator":{"signingKey":...}}}'` against a Running node is a silent no-op — the pod doesn't restart, the Secret never mounts, the validator never starts signing. Documented in [LLD §11](...).

## Impact

### Primary use case — zero-downtime migration cutover

The single-shot deployment in PR #136 trades cutover downtime ... [content as in #137]

### Future use case — validator mode switch

A separate, larger ask: convert a Running `fullNode` SeiNode into a `validator` ... [content as in #137]

## Relevant experts

- `kubernetes-specialist` — controller plan logic (`buildRunningPlan`, `buildNodeUpdatePlan`)
- `platform-engineer` — CRD field surface, `SeiNodeStatus`
- `sei-network-specialist` — seid restart semantics inside the rolling-update window

## Proposed approach

Reuse the existing NodeUpdate plan shape ... [content as in #137]

## Acceptance criteria

- [ ] `Status.SigningKeyMountedSecret` field added to `SeiNodeStatus` and stamped by the rollout-watch task on success
- [ ] `buildRunningPlan` detects `spec.validator.signingKey.secret.secretName != status.SigningKeyMountedSecret` and triggers a re-apply plan
- [ ] Re-apply plan includes `validate-signing-key` so Secret defects are caught before the pod rolls
- [ ] Integration test: deploy SeiNode without SigningKey → reaches Running as observer → patch SigningKey in → pod restarts with Secret mounted → SigningKeyReady=True → StatefulSet has the volume + subPath mount
- [ ] LLD §8 updated to re-document the zero-downtime cutover flow as supported

## Out of scope

- **Mode switch (full-node → validator).** Same drift-detection mechanic, larger surface; file as a follow-up once SigningKey drift is shipped and we have a concrete customer ask.
- **Demoting a validator to non-signing.** Currently no field-level immutability blocks unset, but the workflow is unsupported per LLD §11.
- **Rotating `secretName` on a Running validator.** `secretName` is immutable; rotating consensus key requires `MsgEditValidator` and is a coordinated on-chain operation.

## References

- PR #135 — merged LLD
- PR #136 — single-shot deployment shipped
- [LLD §11 — deferred entry with implementation sketch](https://github.com/sei-protocol/sei-k8s-controller/blob/main/docs/design-seinode-validator-signing-key-lld.md#11-what-this-lld-does-not-cover)
- `.tide/validator-migration.md` — internal runbook
```

## End-of-turn summary

> Filed: https://github.com/sei-protocol/sei-k8s-controller/issues/137 — "Detect spec drift on Running nodes for mid-life SigningKey patch"

## Notes on this example

- **No "Background" section.** The standard format folds background into Problem. The reference issue uses Background as a header, but newer filings should use Problem — easier for triagers who scan top-down.
- **No `_TBD — needs design_` placeholder.** This filer had a sketch, so Proposed approach is populated. If they hadn't, the section would be omitted entirely.
- **Three experts is the cap.** This issue has a clean three-way split (controller / CRD / seid) so it's at the limit but not over. A fourth expert would be a signal to hand off to `/council`.
