# SeiNodeTask gov-manifests kit

## 1. What this concern is

**Authoring + GitOps-applying the three gov kinds** — `GovVote`, `GovSoftwareUpgrade`, `GovParamChange` — as **per-node SeiNodeTask manifests**, getting the typed payload encoding right, resolving the signing key, fanning the work across the SeiNode set, and verifying the result on the chain. The generic gov mental model gets the payloads wrong here: it reaches for snake_case, quotes typed numerics (`proposalId`/`upgradeHeight`) or — for a `GovParamChange` `value` — passes a large-integer param as a bare number and double-encodes a struct param value as an escaped string. It also assumes one manifest fans out (there is **no selector**) and that the gates/fee-floor/fan-out template are this skill's (they are **`/gov-ops`'s** — cited, not restated). *Cited:* `sources.md` §controller (CRD payload shapes), §seictl (`gov_*.go` handlers), §gov-ops (the gates + fan-out + fee floor), §sei-skill (the `seid query gov` verify vocabulary).

## 2. The pattern (how this platform does it)

- **Kind ↔ typed payload, camelCase.** The discriminator is **`spec.kind`** (PascalCase enum: `GovVote` / `GovSoftwareUpgrade` / `GovParamChange`), and the matching payload goes under the **sub-key named for the kind** (`spec.govVote:` / `spec.govSoftwareUpgrade:` / `spec.govParamChange:`); `keyName` lives **inside** the payload, not at `spec.`. Fields are **camelCase**, the typed shapes the sidecar handler deserializes (`sidecar/tasks/gov_*.go`). The CEL union rule requires exactly one payload set and the one matching `spec.kind`. *Cited:* §controller (`SeiNodeTaskSpec`, the CEL `kind`↔payload rules), §seictl.
- **Encoding rules (scope: `GovParamChangeEntry.value` ONLY).** The integer-as-JSON-string and double-encode rules apply **only** to a `GovParamChange` change `value` (the raw param value the chain unmarshals to its registered type) — **NOT** to `proposalId`/`upgradeHeight`, which are typed numerics. Within a param `value`: a **large-integer param** is a JSON **string** (`"86400000000000"`) — Sei's request decode routes values through `map[string]any` and bare numbers lose precision above 2^53; a **struct param** is a JSON **object**, not a pre-escaped JSON string (the double-encode trap — the sidecar stringifies the raw bytes exactly once). *Cited:* §controller (`GovParamChangeEntry.value` doc), §seictl (`gov_param_change.go` `paramChange.Value` + the single `string()` conversion).
- **Typed numerics are JSON numbers (unquoted).** `proposalId` is `uint64` and `upgradeHeight` is `int64` → bare JSON **numbers** (`proposalId: 142`, `upgradeHeight: 12750000`). `option` is the lowercase enum (`yes` | `no` | `abstain` | `no_with_veto`). *Cited:* §controller (`GovVotePayload.ProposalID`, `GovSoftwareUpgradePayload.UpgradeHeight`, `GovVotePayload.Option` enum), §seictl (`GovVoteRequest`).
- **Keyring resolution ladder.** `keyName`: **explicit** → **`node_admin`** (if operatorKeyring secret set) → **`validator` gentx key** (unset). `resolveSigningUID` (`internal/task/seinodetask_params.go:231`), CRD comments (`api/v1alpha1/seinodetask_types.go:227-231,286-292,355-362`), `DefaultOperatorKeyName="node_admin"` (`api/v1alpha1/validator_types.go:7`). *Cited:* §controller — **NOT** the stale LLD.
- **Per-node fan-out.** No selector — author **one manifest per SeiNode**. For a vote, fan it across the set per **`/gov-ops`'s `fan-out.md` template** — cite it, don't re-derive; the **fee floor** is `/gov-ops`'s too. *Cited:* profile §1, §gov-ops.
- **Verify on-chain.** Poll the **chain** (proposalId / txHash via the `taskID=` memo), not `status.outputs`. Use `seid query gov` vocabulary (§sei-skill). When a submit lands `Failed`/`Timeout` with no proposalId, `seid q tx <hash>` gives the authoritative `code`/`raw_log` — a committed `code != 0` tx can be masked as a bare `Timeout` (seictl#236). Size the initial deposit against the **live** target-chain `min_deposit` + proposer balance, never a hardcoded/cross-chain number (gate owned by `/gov-ops`). *Cited:* profile §6, §gov-ops.

**Worked: a per-node GovVote** (one of N — fan via `/gov-ops`'s `fan-out.md`):
```yaml
apiVersion: sei.io/v1alpha1                 # group is sei.io (groupversion_info.go +groupName)
kind: SeiNodeTask
metadata:
  name: gov-vote-prop-142-validator-03      # per-node, STABLE; task-ID (UUIDv5) re-joins on re-apply
spec:
  kind: GovVote                             # discriminator is spec.kind (PascalCase enum)
  target:
    nodeRef:
      name: arctic-1-validator-03           # soft name-ref to ONE SeiNode (no selector)
  govVote:                                  # payload under the sub-key matching spec.kind
    chainId: arctic-1                        # required; guardChainID cross-checks env + node /status
    proposalId: 142                          # uint64 → JSON NUMBER (unquoted) — NOT "142"
    option: yes                              # lowercase enum: yes | no | abstain | no_with_veto
    keyName: node_admin                      # INSIDE the payload; ladder: explicit → node_admin → validator gentx key
    fees: 2000usei                           # required; must clear the node's min-gas-price floor — see /gov-ops
    gas: 200000                              # required, uint64
  # GovVote is chain-idempotent (last-write-wins on proposalId/voter) → re-apply safe
```

**Worked: a per-node GovSoftwareUpgrade** (submit kind — submit-once, NEVER delete-recreate):
```yaml
apiVersion: sei.io/v1alpha1
kind: SeiNodeTask
metadata:
  name: gov-swupgrade-v6-1-0-validator-01   # STABLE name → stable task-ID; re-apply re-joins, never re-submits
spec:
  kind: GovSoftwareUpgrade
  target:
    nodeRef:
      name: arctic-1-validator-01
  govSoftwareUpgrade:
    chainId: arctic-1                        # required
    title: "Upgrade to v6.1.0"               # required
    description: "Schedule the v6.1.0 upgrade plan."   # required
    upgradeName: v6.1.0                      # required — field is upgradeName (NOT name)
    upgradeHeight: 12750000                  # int64 → JSON NUMBER (unquoted) — field is upgradeHeight (NOT height)
    upgradeInfo: "https://github.com/sei-protocol/sei-chain/releases/tag/v6.1.0"   # optional — field is upgradeInfo (NOT info)
    initialDeposit: 10000000usei             # required; usei-only, must clear chain min_deposit to enter voting
    keyName: node_admin                      # INSIDE the payload
    fees: 2000usei                           # required; floor is /gov-ops's
    gas: 400000                              # required, uint64
  # submit-once (crash-window re-run of the SAME task adopts via the pre-broadcast
  # marker); a delete-recreate mints a new run → duplicate proposal + deposit.
  # fee floor + gates owned by /gov-ops.
```

**Worked: a per-node GovParamChange** (submit kind — the highest-value example; shows both `value` encodings):
```yaml
apiVersion: sei.io/v1alpha1
kind: SeiNodeTask
metadata:
  name: gov-paramchange-timeout-validator-01   # STABLE name → stable task-ID; re-apply re-joins
spec:
  kind: GovParamChange
  target:
    nodeRef:
      name: arctic-1-validator-01
  govParamChange:
    chainId: arctic-1                          # required
    title: "Adjust consensus + governance params"   # required
    description: "Bump TimeoutCommit and the max deposit period."   # required
    changes:                                   # required, MinItems=1; each is {subspace, key, value}
      - subspace: baseapp
        key: TimeoutParams
        value: {"timeout_commit": "2s", "timeout_propose": "3s"}   # struct param → JSON OBJECT (NOT an escaped string)
      - subspace: gov
        key: maxdepositperiod
        value: "86400000000000"                # large-integer param → JSON STRING (Sei convention; bare number loses precision)
    initialDeposit: 10000000usei               # required; usei-only, >= chain min_deposit
    keyName: node_admin                        # INSIDE the payload
    fees: 8000usei                             # required; floor is /gov-ops's
    gas: 400000                                # required, uint64
  # submit-once and the higher-blast-radius case (no apply-once net; the pre-broadcast
  # marker covers only a same-task crash re-run). never delete-recreate.
```

> **Stable-name → task-ID coupling (the submit-once linchpin).** Each manifest needs a **distinct** `metadata.name` and `nodeRef.name` (one per SeiNode — no selector). The sidecar **task-ID is a UUIDv5 derived from the CR identity**, so a **stable name** is what makes a **re-apply re-join** the existing run instead of minting a new one. Change the name (or delete-recreate) and you mint a fresh task-ID → a duplicate submit + deposit on the non-idempotent kinds. *Cited:* §controller (`status.task.id` deterministic UUIDv5, stable for the CR's lifetime).

## 3. Anti-patterns / failure modes

- **Quoting a typed numeric.** Cue: `proposalId: "142"` / `upgradeHeight: "12750000"` (a string where the type is `uint64`/`int64`). Rewrite: **unquote it** — `proposalId: 142`, `upgradeHeight: 12750000` are JSON numbers (Dimension 1 (Manifest correctness & encoding)). *(The integer-as-string convention is for a `GovParamChangeEntry.value`, NOT for these typed payload fields.)*
- **Large-integer param `value` as a bare number.** Cue: a `GovParamChange` change `value: 86400000000000` (bare number). Rewrite: quote it — `"86400000000000"` — large-integer params are JSON strings (precision loss above 2^53 through the decode path). Scope: a param `value` only (Dimension 1).
- **Double-encoded struct param value.** Cue: a `GovParamChange` change `value` as an escaped JSON string (`"{\"key\":...}"`). Rewrite: the struct value is a **JSON object**, not a pre-escaped string — the sidecar stringifies the raw bytes exactly once, so escaping double-encodes and fails at apply (Dimension 1, profile encoding rule).
- **Discriminator / nesting drift.** Cue: `spec.type` instead of `spec.kind`; payload at `spec.` instead of under `spec.govVote`/`govSoftwareUpgrade`/`govParamChange`; `keyName` at `spec.` instead of inside the payload; `apiVersion: tasks.sei.io/...`. Rewrite: discriminator is `spec.kind`; payload nests under the kind sub-key; `keyName` is in the payload; group is `sei.io/v1alpha1` (Dimension 1).
- **snake_case fields / wrong upgrade field names.** Cue: `proposal_id` / `software_upgrade`; `name`/`height`/`info` for a software upgrade. Rewrite: camelCase, and the upgrade fields are `upgradeName`/`upgradeHeight`/`upgradeInfo` (Dimension 1 (Manifest correctness & encoding)).
- **Assuming `node_admin` signs.** Cue: a gentx-bootstrapped validator with no operatorKeyring secret, manifest assuming `node_admin`. Rewrite: walk the ladder — it falls through to the `validator` gentx key (profile §4).
- **Delete-recreate to retry a submit.** Cue: `delete && apply` on a `GovParamChange`. Rewrite: **refuse** — re-apply re-joins the run; delete-recreate duplicates the proposal + deposit (Dimension 2 (Idempotency & recovery)).
- **Reading `status.outputs` for proposalId.** Cue: polling `status`. Rewrite: read the chain via the `taskID=` memo (Dimension 4 (Result verification)).
- **Restating a `/gov-ops` fact.** Cue: a hard-coded fee floor / a re-written fan-out template / a local mainnet allowlist. Rewrite: cite `/gov-ops` (Dimension 5 (Citation discipline)).

## 4. Review cues

- **Dimension 1 (Manifest correctness & encoding):** `spec.kind`↔payload sub-key match; camelCase; `keyName` inside the payload; `apiVersion: sei.io/v1alpha1`; typed numerics (`proposalId`/`upgradeHeight`) unquoted; `option` lowercase enum; for a `GovParamChangeEntry.value` only — large-integer param a JSON string, struct param a JSON object (not double-encoded); all required payload fields present (chainId/fees/gas, + per-kind); fee floor **cited to `/gov-ops`**. *Basis:* §controller, §seictl, §gov-ops.
- **Dimension 2 (Idempotency & recovery):** GovVote re-apply safe; submit kinds submit-once (same-task crash re-runs adopt via the pre-broadcast marker), no delete-recreate. *Basis:* profile §5, §seictl (the marker, seictl#219).
- **Dimension 3 (Context & mainnet-adjacency safety):** the per-node fan-out targets the intended `(context, network, namespace)`; the allowlist refuse is **`/gov-ops`'s**. *Basis:* profile §8, §gov-ops.
- **Dimension 4 (Result verification):** verified on the chain, not `status.outputs`. *Basis:* profile §6.
- **Dimension 5 (Citation discipline):** keyring ladder cited to §controller (not the stale LLD); gates/fan-out/fee-floor cited to §gov-ops. *Basis:* `sources.md`.

## 5. One-way doors in this concern

- **A `GovSoftwareUpgrade` / `GovParamChange` broadcast** — a real on-chain proposal + deposit; `GovParamChange` is higher-blast-radius. Submit-once; gates + fee floor owned by `/gov-ops`.
- **A delete-recreate on a submit kind** — refuse (duplicate submit + deposit).
- **A fan-out aimed at a mainnet-adjacent context** (arctic-1 co-hosted with pacific-1) — defer to `/gov-ops`'s allowlist refuse; never assert a weaker rule.
