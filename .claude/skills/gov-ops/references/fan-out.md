# Gov-Ops — Vote fan-out

Generate one `GovVote` SeiNodeTask per validator, **per cluster**, from the live validator
list (so every `nodeRef` resolves in its own cluster). Fill `proposalId` only after the
proposal is submitted and the content/id confirm gate (SKILL step 3) passes.

## Template

```yaml
apiVersion: sei.io/v1alpha1
kind: SeiNodeTask
metadata:
  name: govvote-prop<ID>-<seinode>          # e.g. govvote-prop252-validator-0
  namespace: <target-namespace>             # e.g. arctic-1 — never defaulted
  labels:
    op.sei.io/governance: <vote-slug>       # for bulk get / status / prune
spec:
  kind: GovVote
  timeoutSeconds: 600
  target:
    nodeRef:
      name: <seinode>                       # the SeiNode (validator-N-0), NOT the deployment or pod
    requirePhase: Running
    requirePhaseTimeout: 30m                # ride through a transient validator restart
  govVote:
    chainId: <network>                       # e.g. arctic-1
    proposalId: <ID>                         # filled post-submit; assert == resolved submit id
    option: "yes"                            # quote it — bare yes is YAML true
    fees: "<fees>usei"                       # >= gas × min-gas-price, read live per target chain (e.g. arctic-1 0.02usei/gas → 8000usei @ gas 300000; not a constant)
    gas: 300000
    # keyName omitted → resolves node_admin via ResolveOperatorKeyringUID
```

## Generation

- Enumerate validators per cluster from the live list (`kubectl --context <ctx> -n <ns> get
  seinode -l sei.io/role=validator -o name`), not from a static file — keeps `nodeRef` real.
- One manifest set per cluster context; wire into that cluster's Flux path; `flux reconcile
  kustomization flux-system -n flux-system --with-source --context <ctx>` after the merge.
- Fee floor and the SeiNode-name (not pod/deployment) rule are the two most common mistakes —
  both are encoded above. See `sei-protocol/bdchatham-designs designs/seinode-task/seinode-task.md` for the why.
