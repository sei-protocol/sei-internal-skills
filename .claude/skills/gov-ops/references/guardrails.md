# Gov-Ops — Safety model

Every item here is a **hard, fail-closed gate**. Re-assert the allowlist triple before each side-effecting step.

## Allowlist (positive — not a prod-pattern refuse)

The target chain may live in a `prod-*` cluster, so a `prod`-pattern refuse is wrong. Gate on the exact `(context, network, namespace)` triple, and **refuse any context that co-hosts a non-target chain**.

| kube-context | expected network | namespace | allowed? | why |
|---|---|---|---|---|
| `prod-use2` | `arctic-1` | `arctic-1` | ✅ | single-chain (arctic-1 only) |
| `prod-euw1` | `arctic-1` | `arctic-1` | ✅ | single-chain (arctic-1 only) |
| `prod` (eu-central-1) | `arctic-1` | `arctic-1` | ❌ **REFUSE** | co-hosts `pacific-1` (mainnet) + `atlantic-2`; a namespace typo reaches mainnet |
| any context | mainnet (`pacific-1`) | — | ❌ **REFUSE** | this skill is not for mainnet governance |

Operator extends this table per chain; an entry is required before the skill will run. Pre-flight also asserts `seid status` → `network` == expected and `catching_up == false`, and **pins that RPC endpoint** for every subsequent tx.

## The fail-closed gates

| gate | step | passes when | on fail |
|---|---|---|---|
| allowlist + endpoint pin | every | live triple ∈ allowlist; endpoint pinned | refuse / abort |
| value-shape | submit | `--generate-only` value is JSON of the param type (not a quoted/escaped string) and shape-matches on-chain | block broadcast |
| deposit | submit | `min_deposit` read **live from the target chain** (`seid q gov params`; never hardcoded — gov-settable and per-chain); `initial_deposit >= min_deposit` **and** `proposer_balance >= initial_deposit + fees` | block (under-min → sits in deposit-period; unaffordable → commits but fails DeliverTx `code 5`, no proposal — seictl#236) |
| duplicate-submit | submit | no prior submit of this content already landed | block (avoid 2nd proposal) |
| confirm (broadcast) | submit | operator types `confirm` after the echo | abort |
| content/id confirm | post-submit | proposal-by-resolved-id content matches intent; `status==VOTING_PERIOD` | abort fan-out |
| fee-floor | vote | `fees >= gas × chain-min-gas-price`, the min-gas-price **read live per target chain** (config/gov-settable; the `arctic-1 0.02usei/gas` figure is an example, not a constant) | reject pre-broadcast |
| id-match | vote | fanned `proposalId` == resolved submit id | block fan-out |
| confirm (merge) | vote | operator types `confirm` after the echo | abort |
| active detector | post-broadcast / post-fan-out | no CheckTx code-13; no DeliverTx code-5 (unaffordable deposit); tally moving — resolve via `seid q tx <hash>`, not task status alone (seictl#236) | HALT loudly |
| applied | verify | live `subspace/key` value byte-exact == submitted | HALT / trigger revert |

## Authorization mechanism (fast-path only)

GitOps is the default. The imperative fast-path (`kubectl apply` / `flux suspend` to shared prod) runs **only** with all of:
1. a **named human** authorizing;
2. a **verbatim token** echoing the exact action + context + command;
3. an **append-only audit entry** in `state/audit.log`.

Anything less → refuse, stay GitOps. Never auto-suspend Flux.

## Credential hygiene

Non-validator funded key; keyring sign (never inline mnemonic); `--generate-only` artifacts to gitignored `state/`, shredded post-run; no key material in logs/audit/chat.
