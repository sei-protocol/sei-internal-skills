# seictl CLI surface

Canonical command reference. **`seictl --help` is the source of truth** — when this file disagrees, the CLI wins.

Last verified: 2026-04-28 against sei-protocol/seictl v1 surface (PRs #71/72/73/74/76/77/78/79/81 merged).

## Top-level commands

| Command | Domain | What it does |
|---|---|---|
| `seictl config patch` | local | Patch app.toml/client.toml/config.toml |
| `seictl genesis patch` | local | Patch genesis JSON |
| `seictl patch` | local | Generic TOML/JSON merge-patch |
| `seictl serve` | local | Run the in-pod sidecar HTTP server |
| `seictl await` | local | Wait condition |
| `seictl report` | local | Analyze shadow chain comparison data |
| `seictl context` | cluster | Cluster + identity ground truth |
| `seictl onboard` | cluster | Provision a new engineer's harbor footprint |
| `seictl bench up` | cluster | Render or apply a benchmark workload |
| `seictl bench down` | cluster | Tear down a benchmark by name |
| `seictl bench list` | cluster | Owner-scoped list of running benchmarks |

The skill only invokes commands flagged "cluster" above. The "local" commands aren't in the skill's interface today.

## Cluster-facing commands (skill scope)

Every cluster command accepts `--kubeconfig <path>` (also `$KUBECONFIG`) and `--context <name>` to select the kube target. Omitted, they fall back to the standard kube client-go discovery chain. These aren't repeated per-verb below.

### `seictl context`

```
seictl context [--kubeconfig <path>] [--context <name>]
```

Side-effect-free. AWS reads are best-effort — an expired SSO session yields a result with empty AWS fields rather than failing, so `context` can diagnose the SSO state itself.

```go
type ContextResult struct {
    KubeContext     string    `json:"kubeContext"`
    Cluster         string    `json:"cluster"`
    Server          string    `json:"server"`
    Namespace       string    `json:"namespace"`
    AWSAccount      string    `json:"awsAccount"`
    AWSRegion       string    `json:"awsRegion"`
    AWSPrincipalARN string    `json:"awsPrincipalArn"`
    Engineer        *Engineer `json:"engineer,omitempty"`
}
type Engineer struct{ Alias, Name string }
```

The Claude skill calls this at the start of any session to confirm cluster, identity, and AWS principal.

### `seictl onboard`

```
seictl onboard --alias <alias> [--name <name>]
               [--platform-repo <path>] [--no-pr] [--apply]
```

Default behavior is dry-run; `--apply` performs both side effects:

1. Generates `clusters/harbor/engineers/<alias>/{kustomization,namespace,bench-seiload-sa}.yaml` in the platform repo, branches `seictl/onboard-<alias>`, opens a PR via `gh`.
2. Creates IAM policy + Pod Identity association directly via AWS SDK (`iam:CreatePolicy`, `iam:CreateRole`, `iam:AttachRolePolicy`, `eks:CreatePodIdentityAssociation`). No Terraform.

```go
type OnboardResult struct {
    Alias          string         `json:"alias"`
    IdentityPath   string         `json:"identityPath"`
    GeneratedFiles []string       `json:"generatedFiles"`
    Branch         string         `json:"branch,omitempty"`
    PRURL          string         `json:"prUrl,omitempty"`
    AWSResources   []AWSResource  `json:"awsResources"`
    DryRun         bool           `json:"dryRun"`
}
type AWSResource struct {
    Kind   string `json:"kind"`   // "IAMPolicy" | "IAMRole" | "PodIdentityAssociation"
    ARN    string `json:"arn"`
    Action string `json:"action"` // "create" | "exists" | "would-create"
}
```

Engineer's IAM principal is derived from `aws sts get-caller-identity` and cross-checked against the alias regex. **No `--principal-arn` flag.**

`--no-pr` runs IAM + manifest generation but skips the GitHub PR step — useful when the platform repo is dirty or the engineer wants to inspect generated files first. The IAM resources still get provisioned with `--apply`.

Idempotent: pre-existing IAM resources / Pod Identity association / open PR for the branch are detected and reported as `action: "exists"` with no mutation. A pre-existing Pod Identity association bound to a different role is a hard failure; manual remediation required.

### `seictl bench up`

```
seictl bench up --image <ref> --name <name>
                [--size s|m|l] [--duration <minutes>] [--apply]
```

Required: `--image`, `--name`. Defaults: size `s`, duration `30` (minutes, range 1–240), namespace = `eng-<alias>` from identity.

`--duration` is an **integer in minutes**, not a Go duration string.

Default behavior is dry-run; `--apply` performs server-side apply.

```go
type BenchUpResult struct {
    ChainID      string        `json:"chainId"`         // "bench-<alias>-<name>"
    Name         string        `json:"name"`
    Namespace    string        `json:"namespace"`
    ImageRef     string        `json:"imageRef"`
    ImageDigest  string        `json:"imageDigest"`     // resolved sha256:...
    Size         string        `json:"size"`
    Validators   int           `json:"validators"`
    RPCNodes     int           `json:"rpcNodes"`
    Duration     string        `json:"duration"`        // formatted, e.g. "30m"
    ResultsS3URI string        `json:"resultsS3Uri"`
    DryRun       bool          `json:"dryRun"`
    Manifests    []ManifestRef `json:"manifests"`
    AppliedAt    *time.Time    `json:"appliedAt,omitempty"`
}

type ManifestRef struct {
    Kind, Name, Namespace, Action string
    // action for bench up: "create" | "update" | "unchanged"
    // action for bench down: "deleted" | "not-found" | "still-terminating"
}
```

Sizes: `s` = 4 validators / 1 RPC; `m` = 10 / 2; `l` = 21 / 4.

Chain ID: `bench-<alias>-<name>` (engineer benchmarks); RPC SND is `bench-<alias>-<name>-rpc`. Distinct from autobake's nightly `autobake-<run-id>`.

S3 results path (per platform repo's `harbor-validation-results` schema):
`s3://harbor-validation-results/<namespace>/<job>/<run>/report.log` where `<namespace>` is `eng-<alias>`, `<job>` is the seiload Job name, `<run>` is the bench `--name`.

Action attribution uses `.metadata.generation` (a `Patch` that bumps `generation` is a real spec change → `update`; otherwise `unchanged`). ResourceVersion is unsound here because controllers continuously write status.

### `seictl bench down`

```
seictl bench down --name <name> [--namespace <ns>] [-n <ns>]
```

Label-selected delete with `metav1.DeletePropagationForeground`. No dry-run flag — down is bounded and idempotent (re-run is fine if interrupted). No `--wait`/`--timeout` in v1; the command returns once Delete calls have been issued. Engineer uses `seictl bench list` to observe convergence.

```go
type BenchDownResult struct {
    Name      string        `json:"name"`
    ChainID   string        `json:"chainId"`
    Namespace string        `json:"namespace"`
    Resources []ManifestRef `json:"resources"` // action: "deleted"|"not-found"|"still-terminating"
    DeletedAt *time.Time    `json:"deletedAt,omitempty"`
}
```

If a finalizer keeps a resource around longer than expected: `bench list` will continue showing it. Engineer uses `kubectl` independently if they want to investigate; seictl doesn't redirect to other tools.

### `seictl bench list`

```
seictl bench list [--all-namespaces|-A] [--namespace <ns>|-n <ns>]
```

Owner-scoped via labels (`sei.io/engineer=<alias>` on managed resources; `app.kubernetes.io/part-of=seictl-bench` on the seiload Job). Aggregates by chain ID; validators and RPC SND are reported under one summary, with role-based ready/desired counts.

```go
type BenchListResult struct {
    Items []BenchSummary `json:"items"`
}
type BenchSummary struct {
    ChainID           string `json:"chainId"`
    Name              string `json:"name"`
    Namespace         string `json:"namespace"`
    Owner             string `json:"owner"`
    Phase             string `json:"phase"`
    ValidatorsReady   int    `json:"validatorsReady"`
    ValidatorsDesired int    `json:"validatorsDesired"`
    RPCReady          int    `json:"rpcReady"`
    RPCDesired        int    `json:"rpcDesired"`
    LoadJobPhase      string `json:"loadJobPhase"`
    AgeSeconds        int64  `json:"ageSeconds"`
    ImageDigest       string `json:"imageDigest"`
}
```

## Common patterns

### Output format

JSON envelope on stdout, structured logs on stderr. **No `--format` flag in v1** — JSON is the only emission. If a `text` format ships later, it'll be additive behind a flag, not a default change.

### Output envelope

Every cluster command wraps its result in a Kubernetes-style TypeMeta envelope:

```go
type Envelope struct {
    APIVersion string          `json:"apiVersion"` // always "seictl.sei.io/v1"
    Kind       string          `json:"kind"`       // see Kinds below
    Data       json.RawMessage `json:"data,omitempty"`
    Error      *ErrorBody      `json:"error,omitempty"`
}
type ErrorBody struct {
    Code     int    `json:"code"`
    Category string `json:"category"`
    Message  string `json:"message"`
    Detail   string `json:"detail,omitempty"`
}
```

Kinds: `ContextResult`, `OnboardResult`, `BenchUpResult`, `BenchDownResult`, `BenchListResult`. Breaking changes ship as `seictl.sei.io/v2` alongside v1, not as mutations to v1.

### Exit codes

Eight codes total. The granular cause lives in `error.category` — exit codes are families.

| Code | Family | Meaning |
|---|---|---|
| 0 | | Success |
| 2 | usage | Usage error |
| 3 | not-found | Resource not found |
| 4 | cluster | Cluster unreachable |
| 5 | rbac | Permission denied |
| 10 | bench | Bench failure (specific cause in `error.category`) |
| 20 | onboard | Onboard failure (specific cause in `error.category`) |
| 40 | identity | Identity failure (specific cause in `error.category`) |

Category strings for v1:

- **bench:** `image-policy`, `image-resolution`, `validation`, `namespace-policy`, `apply-failed`, `name-collision`, `finalizer-stuck`, `template-render`
- **onboard:** `alias-invalid`, `platform-repo-missing`, `working-tree-dirty`, `gh-unauthenticated`, `pr-create-failed`, `aws-create-failed`
- **identity:** `malformed`, `missing`, `kubeconfig-parse`, `perms-loose`, `aws-unavailable`

### Read-only by default

`bench up` and `onboard` default to dry-run. Nothing is applied without `--apply`. Safe to expose as MCP tools later without auth gymnastics.

### No ambient state

Commands never `cd`, never modify `~/.kube/config`, never set env vars in the calling shell. Every kubectl invocation is explicit about context and namespace. Discovery cache lives at `~/.kube/cache/discovery/` (managed by client-go's `genericclioptions.ConfigFlags`, shared with kubectl).

## MCP graduation notes

Each subcommand maps to one MCP tool. The MCP server flattens names (`bench_up`, `bench_down`, etc.) — no `cluster_` infix to strip. Tool descriptions come from `seictl <cmd> --help`. JSON schemas above are the v2 MCP tool contracts; stable through MCP graduation.
