# SeiNodeTask gov-manifests kit

## 1. What this concern is

**Authoring + GitOps-applying the three gov kinds** — `GovVote`, `GovSoftwareUpgrade`, `GovParamChange` — as **per-node SeiNodeTask manifests**, getting the typed payload encoding right, resolving the signing key, fanning the work across the SeiNode set, and verifying the result on the chain. The generic gov mental model gets the payloads wrong here: it reaches for snake_case, encodes a string-typed integer as a JSON number, and double-encodes a struct param value as an escaped string. It also assumes one manifest fans out (there is **no selector**) and that the gates/fee-floor/fan-out template are this skill's (they are **`/gov-ops`'s** — cited, not restated). *Cited:* `sources.md` §controller (CRD payload shapes), §seictl (`gov_*.go` handlers), §gov-ops (the gates + fan-out + fee floor), §sei-skill (the `seid query gov` verify vocabulary).

## 2. The pattern (how this platform does it)

- **Kind ↔ typed payload, camelCase.** Each kind has a typed payload the sidecar handler deserializes (`sidecar/tasks/gov_*.go`); fields are **camelCase**. *Cited:* §seictl, §controller.
- **Encoding rules.** A chain parameter the SDK expects as a **JSON string** is `"5"`, not `5` (integer-as-JSON-string). A **struct param value** is a **JSON object**, not an escaped JSON string (the double-encode trap). *Cited:* §controller (CRD payload), §seictl (handler deserialization).
- **Keyring resolution ladder.** `keyName`: **explicit** → **`node_admin`** (if operatorKeyring secret set) → **`validator` gentx key** (unset). `resolveSigningUID` (`internal/task/seinodetask_params.go:231`), CRD comments (`api/v1alpha1/seinodetask_types.go:227-231,286-292,355-362`), `DefaultOperatorKeyName="node_admin"` (`api/v1alpha1/validator_types.go:7`). *Cited:* §controller — **NOT** the stale LLD.
- **Per-node fan-out.** No selector — author **one manifest per SeiNode**. For a vote, fan it across the set per **`/gov-ops`'s `fan-out.md` template** — cite it, don't re-derive; the **fee floor** is `/gov-ops`'s too. *Cited:* profile §1, §gov-ops.
- **Verify on-chain.** Poll the **chain** (proposalId / txHash via the `taskID=` memo), not `status.outputs`. Use `seid query gov` vocabulary (§sei-skill). *Cited:* profile §6.

**Worked: a per-node GovVote** (one of N — fan via `/gov-ops`'s `fan-out.md`):
```yaml
apiVersion: tasks.sei.io/v1alpha1
kind: SeiNodeTask
metadata:
  name: gov-vote-prop-142-validator-03      # per-node; task-ID (UUIDv5) re-joins on re-apply
spec:
  target:
    nodeRef:
      name: arctic-1-validator-03           # soft name-ref to ONE SeiNode (no selector)
  type: GovVote
  govVote:                                  # camelCase typed payload
    proposalId: "142"                       # integer-as-JSON-STRING — not 142
    option: VOTE_OPTION_YES
  keyName: node_admin                       # ladder: explicit → node_admin → validator gentx key
  # GovVote is chain-idempotent (last-write-wins) → re-apply safe
```

**Worked: a per-node GovSoftwareUpgrade** (submit kind — submit-once, NEVER delete-recreate):
```yaml
apiVersion: tasks.sei.io/v1alpha1
kind: SeiNodeTask
metadata:
  name: gov-swupgrade-v6-1-0-validator-01
spec:
  target:
    nodeRef:
      name: arctic-1-validator-01
  type: GovSoftwareUpgrade
  govSoftwareUpgrade:
    title: "Upgrade to v6.1.0"
    name: v6.1.0
    height: "12750000"                      # integer-as-JSON-STRING
    info: "https://github.com/sei-protocol/sei-chain/releases/tag/v6.1.0"
  keyName: node_admin
  # NOT idempotent (seictl#174 open). submit-once; a delete-recreate mints a new run
  # → duplicate proposal + deposit. fee floor + gates owned by /gov-ops.
```

## 3. Anti-patterns / failure modes

- **Integer-as-JSON-number.** Cue: `proposalId: 142` / `height: 12750000` (bare number). Rewrite: quote it — `"142"` (Dimension 1 (Manifest correctness & encoding)).
- **Double-encoded param value.** Cue: a `GovParamChange` `value` as an escaped JSON string (`"{\"key\":...}"`). Rewrite: the struct value is a **JSON object**, not a string (Dimension 1 (Manifest correctness & encoding), profile encoding rule).
- **snake_case fields.** Cue: `proposal_id` / `software_upgrade`. Rewrite: camelCase (Dimension 1 (Manifest correctness & encoding)).
- **Assuming `node_admin` signs.** Cue: a gentx-bootstrapped validator with no operatorKeyring secret, manifest assuming `node_admin`. Rewrite: walk the ladder — it falls through to the `validator` gentx key (profile §4).
- **Delete-recreate to retry a submit.** Cue: `delete && apply` on a `GovParamChange`. Rewrite: **refuse** — re-apply re-joins the run; delete-recreate duplicates the proposal + deposit (Dimension 2 (Idempotency & recovery)).
- **Reading `status.outputs` for proposalId.** Cue: polling `status`. Rewrite: read the chain via the `taskID=` memo (Dimension 4 (Result verification)).
- **Restating a `/gov-ops` fact.** Cue: a hard-coded fee floor / a re-written fan-out template / a local mainnet allowlist. Rewrite: cite `/gov-ops` (Dimension 5 (Citation discipline)).

## 4. Review cues

- **Dimension 1 (Manifest correctness & encoding):** kind↔payload match; camelCase; integer-as-JSON-string; struct param value a JSON object (not double-encoded); fee floor **cited to `/gov-ops`**. *Basis:* §controller, §seictl, §gov-ops.
- **Dimension 2 (Idempotency & recovery):** GovVote re-apply safe; submit kinds submit-once, no delete-recreate. *Basis:* profile §5, §seictl (`seictl#174`).
- **Dimension 3 (Context & mainnet-adjacency safety):** the per-node fan-out targets the intended `(context, network, namespace)`; the allowlist refuse is **`/gov-ops`'s**. *Basis:* profile §8, §gov-ops.
- **Dimension 4 (Result verification):** verified on the chain, not `status.outputs`. *Basis:* profile §6.
- **Dimension 5 (Citation discipline):** keyring ladder cited to §controller (not the stale LLD); gates/fan-out/fee-floor cited to §gov-ops. *Basis:* `sources.md`.

## 5. One-way doors in this concern

- **A `GovSoftwareUpgrade` / `GovParamChange` broadcast** — a real on-chain proposal + deposit; `GovParamChange` is higher-blast-radius. Submit-once; gates + fee floor owned by `/gov-ops`.
- **A delete-recreate on a submit kind** — refuse (duplicate submit + deposit).
- **A fan-out aimed at a mainnet-adjacent context** (arctic-1 co-hosted with pacific-1) — defer to `/gov-ops`'s allowlist refuse; never assert a weaker rule.
