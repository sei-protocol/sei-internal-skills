# Chaos faults on an ephemeral chain (GitOps, Chaos Mesh)

Read this when an engineer wants to inject a fault into their own chain: "partition validator 0", "add 200ms latency", "kill a validator mid-bench", "run the chaos suite against my image". The fault is a Chaos Mesh CR committed next to the chain and the bench in `harbor-engineering-workspace`. Flux applies it; Chaos Mesh injects it; Flux prune removes it. No `kubectl apply`, no agent sleep loop.

The catalog, selector contract, and gates below come from the controller's fault package (`sei-k8s-controller` `harness/faults/*.yaml.tmpl` and `faults.go`; the nightly suite in `test/integration/chaos_test.go` / `chaossuite_test.go` renders from it, `chaos_deferred_test.go` names what it leaves out). `seictl chaos render` renders the same package, so an engineer's experiment and the release gate share one template, not one vocabulary copied twice. Controller #558 / seictl #256 introduced both; before those land the templates live at `test/integration/faults/` and the CLI has no `chaos` tree.

## Platform preconditions

Chaos Mesh on harbor runs with `controllerManager.enableFilterNamespace: true`. It injects only into a namespace that carries:

```yaml
metadata:
  annotations:
    chaos-mesh.org/inject: "enabled"
```

The engineer base overlay (`clusters/harbor/engineers/base/`, sei-protocol/platform#1678) stamps the annotation on every `eng-*` namespace and grants the tenant Flux Role the `chaos-mesh.org` verbs. Until that PR is live in the cell, a Chaos CR in an `eng-*` namespace is admitted, and then nothing happens: no `AllInjected`, no events on the targets. Confirm before rendering:

```sh
kubectl get ns eng-<alias> -o jsonpath='{.metadata.annotations.chaos-mesh\.org/inject}'   # → enabled
# Chaos Mesh CRDs installed in the cell (your own access, not Flux's): `yes`, or `no` with a
# "server doesn't have a resource type" warning → Chaos Mesh is not installed; a bare `no` is only your principal
kubectl auth can-i create networkchaos.chaos-mesh.org -n eng-<alias> --context=harbor
```

Flux applies the CR as the cell's ServiceAccount `<alias>` (the base names it `tenant`; the overlay replaces that with the alias), bound to the Role `<alias>` in the same namespace. Its `chaos-mesh.org` grant has no direct probe from an engineer principal: `kubectl auth can-i --as=system:serviceaccount:eng-<alias>:<alias>` needs the `impersonate` verb, which the `admin` ClusterRole the cell binds engineers to does not carry. The grant surfaces at apply time instead: a missing verb turns the `exp-<RUN_ID>` Kustomization `Ready=False` with `networkchaos.chaos-mesh.org ... is forbidden: User "system:serviceaccount:eng-<alias>:<alias>"` in the message. Read that as a platform ask (PLT-1253, platform#1678 not yet live in the cell), the same as an empty annotation; neither is something to patch by hand.

## Selector contract (admission-enforced)

A `ValidatingAdmissionPolicy` (`clusters/harbor/admission/tenant-chaos-policy.yaml`, `failurePolicy: Fail`) gates every Chaos Mesh `v1alpha1` resource in `eng-*` namespaces. A CR that breaks the contract is rejected at admission, and Flux reports the Kustomization `Ready=False` with the policy message. The contract:

- Every selector reachable from the spec (`spec.selector`, `spec.target.selector`, each Schedule/Workflow template) sets `namespaces: ["<request namespace>"]`, nonempty, every entry equal to the CR's own namespace.
- If a selector sets `pods:`, every map key equals the CR's namespace. Chaos Mesh short-circuits to that map when present, so a foreign key would be a cross-tenant fault.
- `Schedule.spec.workflow` is rejected. `Workflow.spec.templates[].schedule` is rejected. Nest kinds, not schedulers.
- Workflow template `templateType` is one of `NetworkChaos`, `PodChaos`, `StressChaos`, `TimeChaos`, `IOChaos`, `Suspend`, `Serial`, `Parallel`, `Task`, `StatusCheck`. Schedule `type` is one of the five Chaos kinds.
- Status-only updates and finalizer removal pass; only CREATE and spec-changing UPDATE are gated.

Cross-namespace faults are out of scope for this skill (the anti-trigger "NOT for cross-tenant work" holds).

## Stable selectors and correlation labels

Target pods by the controller's frozen selector keys, never by pod name:

| Target | Selector |
|---|---|
| Any validator of chain `<chain-id>` | `sei.io/nodedeployment In [<chain-id>]` |
| Validator `k` | add `sei.io/node In [<chain-id>-k]` |
| Every validator except `k` | add `sei.io/node NotIn [<chain-id>-k]` |
| A follower `<chain-id>-rpc-<k>` | `sei.io/node In [<chain-id>-rpc-<k>]` |

`sei.io/nodedeployment` is the owning SeiNetwork name, stamped on every validator child; the harness partition template combines it with `sei.io/node` for the victim. Followers carry no `sei.io/nodedeployment`, so a `nodedeployment`-only selector never faults the observer.

Label every fault CR with the experiment's run token:

```yaml
metadata:
  labels:
    sei.io/harness-run: "<RUN_ID>"
```

Prometheus and `kubectl get <kind> -l sei.io/harness-run=<RUN_ID>` correlate the fault window with the bench and the chain. Use the same `<RUN_ID>` the bench Job carries (`sei.io/bench-name`).

## Fault catalog (active, ten)

Placeholders: `<NS>` = `eng-<alias>`, `<CHAIN>` = the SeiNetwork name, `<RUN>` = run token, `<DUR>` = duration (harness default `3m`). Every entry below is the harness template with those substitutions. Eight faults are `mode: one` — Chaos Mesh picks one validator at random; `network-partition` pins validator `<CHAIN>-0` as the isolated side. Either way one validator is faulted and the chain holds because the committee is 4 and `f=1`. The one exception is `network-latency`: it is **mesh-wide** — every validator link, with no `sei.io/node` narrowing on either side (`mode: all` alone does not mark it; the partition's target side is `mode: all` too, narrowed by `NotIn`) — survivable because the slowdown is symmetric, not because quorum is preserved — `seictl chaos list` prints `mesh-wide` in its scope column for it and `one-validator` for the other nine.

Render, do not copy:

```bash
seictl chaos list                       # name, kind, duration|one-shot, scope, summary
seictl chaos list --output json         # same catalog, machine-readable: [{name, kind, oneShot, meshWide, summary}]
seictl chaos render network-partition --chain-id <CHAIN> --run-id <RUN> -n <NS> --duration <DUR> > chaos-network-partition.yaml
seictl chaos render pod-failure       --chain-id <CHAIN> --run-id <RUN> -n <NS>                  > chaos-pod-failure.yaml
```

`render` touches no cluster; it prints YAML to stdout. It refuses `--duration` on a one-shot fault and refuses its absence on a duration-bearing one, and it rejects a `--duration` that does not parse as a Go duration or is not positive (`2`, `0s`, `-1m`). `--namespace` is required. `--chain-id` and `--run-id` must be DNS-1123 labels (lowercase alphanumerics and `-`, ≤63 characters) because they land in resource names and label values; the generated name must itself fit 63 characters — `<fault>-<run-id>` for most faults, `byzantine-corrupt-<run-id>` for `byzantine`, so size the token against the longest name you will render, so a long run token fails at render, not at admission. Every refusal is a `metav1.Status` `BadRequest` on stderr, so parse `.reason`, do not grep. Pin the seictl version that rendered the file in the experiment's PR description; that is the template provenance.

| Name | Kind / action | Shape | Duration |
|---|---|---|---|
| `network-partition` | `NetworkChaos` / `partition` | validator 0 isolated from validators 1–3, `direction: both` | yes |
| `network-latency` | `NetworkChaos` / `delay` | `200ms` ± `100ms` jitter, `correlation: "50"`, all validator links | yes |
| `packet-loss` | `NetworkChaos` / `loss` | `15%` correlated loss between one validator and the rest | yes |
| `bandwidth-limit` | `NetworkChaos` / `bandwidth` | one validator throttled to `50mbps` (`limit: 20000`, `buffer: 10000`) | yes |
| `byzantine` | `NetworkChaos` / `corrupt` | `10%` packet corruption from one validator, `correlation: "25"` | yes |
| `cpu-stress` | `StressChaos` | one validator, `cpu: {workers: 4, load: 80}` | yes |
| `memory-stress` | `StressChaos` | one validator, `memory: {workers: 2, size: "16GB"}` — cgroup-bounded by the container memory limit; size it to the pool's `--memory` | yes |
| `time-skew` | `TimeChaos` | one validator, `timeOffset: "-30s"`, `clockIds: [CLOCK_REALTIME]`, `containerNames: [seid]` | yes |
| `pod-failure` | `PodChaos` / `pod-kill` | one validator pod deleted, `gracePeriod: 0`; the StatefulSet recreates it | **one-shot** |
| `container-kill` | `PodChaos` / `container-kill` | `seid` container of one validator killed, `gracePeriod: 0`; kubelet restarts in place | **one-shot** |

Canonical shape, the partition (the one fault that names a specific victim):

```yaml
apiVersion: chaos-mesh.org/v1alpha1
kind: NetworkChaos
metadata:
  name: network-partition-<RUN>
  namespace: <NS>
  labels:
    sei.io/harness-run: "<RUN>"
spec:
  action: partition
  mode: all
  selector:
    namespaces: ["<NS>"]
    expressionSelectors:
      - {key: sei.io/nodedeployment, operator: In, values: ["<CHAIN>"]}
      - {key: sei.io/node, operator: In, values: ["<CHAIN>-0"]}
  direction: both
  target:
    mode: all
    selector:
      namespaces: ["<NS>"]
      expressionSelectors:
        - {key: sei.io/nodedeployment, operator: In, values: ["<CHAIN>"]}
        - {key: sei.io/node, operator: NotIn, values: ["<CHAIN>-0"]}
  duration: "<DUR>"
```

Canonical one-shot, the pod kill:

```yaml
apiVersion: chaos-mesh.org/v1alpha1
kind: PodChaos
metadata:
  name: pod-failure-<RUN>
  namespace: <NS>
  labels:
    sei.io/harness-run: "<RUN>"
spec:
  action: pod-kill
  mode: one
  gracePeriod: 0
  selector:
    namespaces: ["<NS>"]
    expressionSelectors:
      - {key: sei.io/nodedeployment, operator: In, values: ["<CHAIN>"]}
```

The two shapes above are what `seictl chaos render` emits for those names; render the other eight the same way. On a seictl without the `chaos` tree, copy `sei-k8s-controller/harness/faults/<name>.yaml.tmpl` (`test/integration/faults/` on a controller before #558), replacing `{{.RunID}}`, `{{.Namespace}}`, `{{.ChainID}}`, `{{.Duration}}`. Do not hand-tune a fault's numbers for a first run; the harness values are the ones the release gate has passed under.

## Deferred faults (do not render)

- **`dns-chaos`** — a rediscovery fault. Live MConnections never re-resolve, so an in-flight chain shows nothing under the fault; the assert has to be recovery-focused with peer-FQDN patterns. Not in the suite until that assert exists.
- **`disk-io-latency` (`IOChaos`)** — Chaos Mesh's toda injector reopens each target fd by pathname while the process is ptrace-paused. Both SeiDB engines (pebbledb compaction, memIAVL snapshot pruning) unlink files a live reader still holds, so the reopen is a structural `ENOENT`, not a race. It corrupts the store rather than delaying it. Refuse, and explain why, until Chaos Mesh ships an injector that does not reopen-by-path or a cgroup `io.max` / dm-delay replacement is validated.

## The `f=1` rule

Every catalog fault targets **one** validator of a **four**-validator committee (or, for latency, degrades all links symmetrically). Tendermint needs more than 2/3 of voting power; with 4 equal validators the chain tolerates exactly one faulty node. Two validators faulted at once, or any single victim on a 3-validator chain of equal power (losing one leaves exactly 2/3, which is not more than 2/3), halts the chain — that is a different experiment (liveness loss), not a degradation measurement. Keep the template's selector (`mode: one`, or the partition's `<CHAIN>-0` pin) on a 4-validator pool unless the engineer explicitly asks for a halt.

## Lifecycle and gates

The harness sequence, which a GitOps experiment reproduces by hand or in a Workflow:

```text
provision(4 validators + 1 unfaulted RPC follower)
→ SeiNetwork Ready, followers Running, every validator placement Scheduled (command below)
→ Flux applies the Chaos CR
→ gate AllInjected=True                    (fault reached its targets)
→ assert the follower's height advances by ≥3 while the fault is active,
  and the SeiNetwork `Producing` condition stays `HeightAdvancing` (`HeightStalled` is the halt;
  an empty-blocks-off Autobahn chain needs load running for this gate — without it the height
  sits at 0 and the condition reads `Idle`, which is neither the halt nor a pass)
→ gate AllRecovered=True                   (duration-bearing faults only)
→ require every validator pod Ready
→ verify the follower is caught up
```

Gate commands:

```sh
# Placement (precondition): one row per validator, every row Scheduled; a Pending row on a Dedicated pool is a capacity ask, not a chain to fault.
# .status.nodes[*].{placement,workerNode} exist from controller 7163c98 (spec 006); an older controller prints no rows. Compare the row count
# to .status.replicas — fewer rows than replicas is a failed gate, not a pass. Fallback on any controller: kubectl get pods -o wide, NODE column nonempty.
kubectl get seinetwork <chain-id> -n eng-<alias> \
  -o jsonpath='{.status.replicas}{"\n"}{range .status.nodes[*]}{.name}{"\t"}{.placement}{"\t"}{.workerNode}{"\n"}{end}'

# Injected / recovered (conditions live on the Chaos CR)
kubectl get <kind> <name> -n eng-<alias> \
  -o jsonpath='{range .status.conditions[*]}{.type}={.status}{"\n"}{end}'
# → AllInjected=True … AllRecovered=False while active; AllRecovered=True after duration

# On the Workflow path the child chaos objects carry controller-chosen names: list them by
# -l chaos-mesh.org/workflow=exp-<RUN> and read the same conditions on each.
# How many pods the fault actually hit (mode: one → exactly 1)
kubectl get <kind> <name> -n eng-<alias> -o jsonpath='{.status.experiment.containerRecords[*].id}'

# Follower height under fault (the observer is never a target)
# <FOLLOWER_URL> is the full per-pod evmJsonRpc URL recipe #1 returns; -d alone sends form-encoded and the handler returns 415
curl -s -H 'Content-Type: application/json' <FOLLOWER_URL> -d '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}'

# Validators back after recovery
kubectl get pod -n eng-<alias> -l sei.io/nodedeployment=<chain-id> \
  -o custom-columns='POD:.metadata.name,READY:.status.containerStatuses[*].ready,RESTARTS:.status.containerStatuses[*].restartCount'
```

`AllInjected` that never turns `True` within about two minutes means the selector matched nothing (wrong `<CHAIN>` or a follower-only label), or the namespace lacks the inject annotation. `kubectl describe <kind> <name>` shows the resolved targets or the empty selection.

**One-shot faults have no `AllRecovered`.** `pod-kill` and `container-kill` carry no `duration`; recovery is the killed pod rejoining. Read recovery from the pod table plus the follower staying live. A `Workflow` step that waits on `AllRecovered` for a one-shot never completes.

**Duration-bearing faults must self-expire.** Omit `duration` and the fault persists until the CR is deleted, and the recovery gate hangs. Always set `<DUR>`.

**Land one-shot kills only on a producing chain.** A `pod-kill` that fires during genesis assembly kills a validator before it has produced a block and the ceremony fails. Merge the kill after `Ready`, or sequence it behind a `Suspend` in a Workflow — a timer, not a readiness gate; see *Sequencing with a Workflow (one PR)* below. A `StatusCheck` template (HTTP probe against the follower's `eth_blockNumber`, admitted by the policy) is the only in-Workflow readiness gate.

## Experiment directory convention

One experiment is one directory, one run token, one PR:

```text
engineers/<alias>/exp-<RUN_ID>/
├── kustomization.yaml            # lists every file below; commonLabels: sei.io/harness-run: "<RUN_ID>"
├── seinetwork-<chain-id>.yaml    # 4 validators, --node-isolation Dedicated, configValues as needed
├── seinode-<chain-id>-rpc-0.yaml # the unfaulted observer / bench target
├── seiload-configmap.yaml        # profile (see sei-load-bench.md)
├── seiload-job.yaml              # bench Job, sei.io/bench-name: <RUN_ID>
└── chaos-<fault>.yaml            # one or more Chaos CRs, or a single Workflow
```

Append `exp-<RUN_ID>` to `engineers/<alias>/kustomization.yaml`'s `resources:`. Flux applies the whole directory together, so **a Chaos CR in the same commit as the SeiNetwork starts injecting as soon as pods exist** — before genesis completes. Two ways to order it:

1. **Two PRs.** Chain + observer first; merge, wait for `Ready` and the placement check; then bench + chaos in a second PR to the same directory.
2. **One PR, a `Workflow` whose first child is a `StatusCheck`.** The Workflow is the only declarative sequencer the admission policy admits; it replaces agent sleeps. A `Suspend` alone is a timer: a cold image pull outruns it into genesis. Lead with the `StatusCheck` below and keep a `Suspend` after it only for bench ramp.

Teardown is `git rm -r engineers/<alias>/exp-<RUN_ID>/` plus the `kustomization.yaml` entry; Flux prunes the directory's objects with no ordering guarantee: if the validator pods go before the Chaos CR, the chaos finalizer has no target to recover and can hold the Kustomization in `Terminating`. Tear down in two merges when a fault is live — first remove the Chaos CRs (on the Workflow path, the Workflow object) and wait for both `kubectl get networkchaos,podchaos,stresschaos,timechaos -n eng-<alias> -l sei.io/harness-run=<RUN_ID>` and `... -l chaos-mesh.org/workflow=exp-<RUN_ID>` to return nothing (Workflow children carry the second label, not the first), then remove the rest. Never `kubectl delete` a Chaos CR that Flux owns — Flux re-applies it on the next reconcile and the fault re-injects. Emergency stop for a fault that is halting the chain: `kubectl annotate <kind> <name> -n eng-<alias> experiment.chaos-mesh.org/pause=true` recovers the targets immediately and survives re-apply (the annotation is not in the manifest, so Flux leaves it); on the Workflow path the child's name is controller-chosen, so resolve it first with `kubectl get networkchaos,podchaos,stresschaos,timechaos -n eng-<alias> -l chaos-mesh.org/workflow=exp-<RUN_ID>` and annotate that object. Follow with the removal PR.

### Sequencing with a Workflow (one PR)

```yaml
apiVersion: chaos-mesh.org/v1alpha1
kind: Workflow
metadata:
  name: exp-<RUN>
  namespace: <NS>
  labels:
    sei.io/harness-run: "<RUN>"
spec:
  entry: timeline
  templates:
    - name: timeline
      templateType: Serial
      deadline: 40m
      children: [rpc-up, warmup, partition, settle, kill]
    - name: rpc-up                # readiness gate: the observer answers JSON-RPC, so genesis completed; it does NOT prove every validator pod is scheduled
      templateType: StatusCheck
      deadline: 20m
      statusCheck:
        mode: Synchronous         # ends on the first success; the Serial parent then advances
        type: HTTP
        intervalSeconds: 10
        timeoutSeconds: 5
        failureThreshold: 120      # 120 × 10s = the deadline; the Workflow fails instead of faulting a chain that never came up
        successThreshold: 1
        http:
          url: <FOLLOWER_URL>     # the per-pod evmJsonRpc URL recipe #1 returns; in-cluster DNS, not a port-forward
          method: POST
          headers:
            Content-Type: [application/json]
          body: '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}'
          criteria:
            statusCode: "200"
    - name: warmup
      templateType: Suspend
      deadline: 5m               # bench ramp only; readiness is the StatusCheck above
    - name: partition
      templateType: NetworkChaos
      deadline: 3m
      networkChaos:
        action: partition
        mode: all
        selector:
          namespaces: ["<NS>"]
          expressionSelectors:
            - {key: sei.io/nodedeployment, operator: In, values: ["<CHAIN>"]}
            - {key: sei.io/node, operator: In, values: ["<CHAIN>-0"]}
        direction: both
        target:
          mode: all
          selector:
            namespaces: ["<NS>"]
            expressionSelectors:
              - {key: sei.io/nodedeployment, operator: In, values: ["<CHAIN>"]}
              - {key: sei.io/node, operator: NotIn, values: ["<CHAIN>-0"]}
    - name: settle
      templateType: Suspend
      deadline: 5m
    - name: kill
      templateType: PodChaos
      deadline: 1m
      podChaos:
        action: pod-kill
        mode: one
        gracePeriod: 0
        selector:
          namespaces: ["<NS>"]
          expressionSelectors:
            - {key: sei.io/nodedeployment, operator: In, values: ["<CHAIN>"]}
```

`StatusCheck` criteria match the HTTP status only, so `200` means the RPC answers, not that height advances; on an empty-blocks-off chain the height stays `0x0` until load and this gate still passes, which is the intended "genesis completed" check (the controller reports the same state as `Producing=False/Idle`, `seinetwork-crd.md`). It proves the observer, not the validators: a Dedicated pool with one pod stuck `Pending` on capacity still serves RPC from the others, and a fault would then hit a 3-of-4 quorum (see the f=1 rule). Nothing in the manifest gates on placement: Flux applies the Workflow at merge and it advances on its own, so in the one-PR path the placement precondition (`.status.nodes[*].placement` all `Scheduled`, the `kubectl get seinetwork` recipe in *Lifecycle and gates*) is a manual check the operator must land before the first fault fires — that is the moment the observer first answers plus the `warmup` `Suspend` (5m above), bounded above by the `StatusCheck` `deadline` (20m); that window is the whole budget, and missing it means the fault fires against whatever scheduled. **Default to the two-PR path whenever the pool is `Dedicated`** (capacity is unknowable before the SeiNetwork is applied); use the one-PR path only for a `Shared` pool, or a Dedicated pool whose placement you have already confirmed on a live run. A Workflow template's `deadline` is the fault duration for a duration-bearing kind. Every nested selector still needs `namespaces: ["<NS>"]` — the admission policy walks the templates. Read Workflow progress with `kubectl get workflow exp-<RUN> -n <NS> -o jsonpath='{.status.conditions}'` and the child `WorkflowNode` objects (`kubectl get workflownode -n <NS> -l chaos-mesh.org/workflow=exp-<RUN>`).

The `warmup` `Suspend` is bench ramp only, never the readiness gate: the `StatusCheck` ahead of it absorbs a cold image pull or a Dedicated pool waiting on capacity, so size `warmup` from the load profile's ramp, not from time-to-`Ready`. The two-PR path remains the choice when the engineer wants to inspect the chain by hand before the first fault.

## Interpreting a run

- **Liveness only.** These faults and gates prove the chain kept producing and recovered. They do not prove transaction correctness; the bench's `report.log` shows throughput and latency under the fault, nothing about state.
- **Bench numbers under a fault are a degradation measurement, not a baseline.** Report them against a same-image, no-fault run with the same footprint and isolation. A comparison of two images under a fault needs both sides to carry an identical Chaos CR with the same `<DUR>`. `network-partition` pins the victim (`<CHAIN>-0`) so that holds by construction; a `mode: one` fault picks at random per run, so record each side's victim from `status.experiment.containerRecords` and say so in the comparison — or, when the ordinal matters, add `sei.io/node In [<CHAIN>-0]` to the pod selector of the rendered CR in **both runs** (one selector; never mirror it onto a `target`, which would make source and target the same pod and inject nothing) and note the hand-edit next to the seictl version pin in the PR description.
- **The follower is the witness.** It is never a target; if it stalls, the chain stalled. A follower with `sei.io/nodedeployment` in its labels would be a mis-rendered node, not a follower.
- **Read the Chaos CR's events before the chain's.** `kubectl describe <kind> <name>` shows selector resolution, injection per target, and recovery; a fault that never injected produces a clean run that means nothing.

## What is not here yet

- `seictl chaos render` / `bench render` are on seictl main (#256, templates from controller #558); the newest tag at the time of writing, `v0.0.72`, predates them. Probe `seictl chaos --help` before the first use in a session; an older binary fails at parse. Without the verbs, build from source (`preflight.md`) or substitute into `harness/faults/` by hand as described under the catalog.
- Fault templates are not published as a versioned artefact; the `harness/faults` package on `sei-k8s-controller` main is the source of truth and seictl pins one commit of it in `go.mod`. Record the seictl version (or the controller commit you copied from) in the experiment's PR description.
- `seictl mcp` (seictl #257) exposes these verbs as `chaos_list` / `chaos_render` / `bench_render` MCP tools; prefer them when the host has them (`references/seictl-cli.md`, *`seictl mcp`*). Like the render verbs, the newest tag at the time of writing predates them.
- A Dedicated pool with a validator `Pending` on capacity (the placement gate command under *Lifecycle and gates*; `kubectl get pods -o wide` with an empty `NODE` column on an older controller) is not a chain to run chaos on; the fault lands on three validators and `f=1` no longer holds.
