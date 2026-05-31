# Brevity Rules — Extended Examples

The 8 rules in SKILL.md, with 2-3 additional before/after pairs per rule. Use this as a reference when a specific surface form isn't covered by the example in the main file.

## Rule 1 — Cut sentences that don't change reviewer action

❌ "This PR introduces a small but important refactor to the validator initialization logic."
✅ (delete entirely — the PR title carries this)

❌ "We've gone through several iterations to land on this approach."
✅ (delete — process narrative, not decision-relevant)

❌ "The previous implementation worked but had some performance characteristics we wanted to improve."
✅ "Replaces O(n²) lookup with hashmap; benchmark in `bench/`."

## Rule 2 — Open on the load-bearing noun

❌ "This PR / This change / This commit ..."
✅ "<Subject of the change> now <verb>s <object>."

❌ "In order to address X, we ..."
✅ "<verb>s X. Fixes #N."

❌ "We are introducing a new feature that ..."
✅ "<feature> now <verb>s ..."

## Rule 3 — In-code comments ≤4 lines

❌
```go
// This function exists to validate the incoming payload before processing.
// It checks for required fields, validates type constraints, and ensures
// referential integrity against the database. The validation is required
// because upstream callers don't always sanitize their inputs.
// We chose this over middleware because the validation rules depend on the
// caller's role, which the middleware layer doesn't have visibility into.
// Note: validation errors should be returned as 400, not 500.
func validate(p Payload, role string) error { ... }
```

✅
```go
// Role-dependent validation; can't be middleware because rules vary by caller role.
func validate(p Payload, role string) error { ... }
```

## Rule 4 — Active verbs, no auxiliary glue

❌ "This function serves to validate the input."
✅ "Validates the input."

❌ "The reconciler aims to converge state."
✅ "Reconciler converges state."

❌ "This config exists to allow operators to override the default."
✅ "Operators override the default via this config."

❌ "We are responsible for managing the lifecycle of these resources."
✅ "We manage these resources' lifecycle." (or even: "We own these resources.")

## Rule 5 — Don't restate names

❌ `// Hash returns a hash of the spec.` on `func (s Spec) Hash() string`
✅ (delete — function signature says this)

❌ `// Slug is the engineer's identifier.` on `Slug string `json:"slug"``
✅ (delete)

❌ `// ChainID is the chain ID.` on `ChainID string`
✅ (delete)

✅ `// ChainID without the chain-prefix (e.g. "pacific-1" not "sei-pacific-1").` on `ChainID string` — earns place because it disambiguates a non-obvious format.

## Rule 6 — One example > one paragraph of explanation

❌ "The function applies the join by matching the pod and namespace labels, then propagating the workload label via group_left semantics."
✅ "`pod_alerts * on (pod, namespace) group_left(workload) kube_pod_info` → series gains `workload`."

❌ "The CRD validation rejects collisions between operator keyring and signing key secret names."
✅ "Setting `operatorKeyring.secret.secretName = signingKey.secret.secretName` → CRD admission fails: `operatorKeyring and signingKey must reference distinct Secrets`."

## Rule 7 — Collapse hedges

Banned words/phrases that always cut or commit:
- "Generally", "typically", "in most cases"
- "It should be noted that", "It is worth mentioning that"
- "Essentially", "basically", "fundamentally"
- "Somewhat", "a bit", "kind of"
- "Quite", "pretty", "rather"

❌ "It should be noted that the timeout is essentially a safety net."
✅ "Timeout is a safety net."

❌ "Generally speaking, this is somewhat of a workaround for the upstream bug."
✅ "Workaround for upstream #N."

## Rule 8 — Sections are a budget, not a structure tax

❌
```
## Summary
One sentence.

## Context
One sentence.

## Files Changed
One file.
```

✅ "One sentence summary. Context: one sentence. Changed file: one file."

Use a section header only when:
1. The section groups ≥2 related items.
2. A reader genuinely needs to skip the rest to find this one.
3. The PR template requires the header (in which case the section is part of the budget, not a bonus).

If you have 3 single-bullet sections, flatten them.
