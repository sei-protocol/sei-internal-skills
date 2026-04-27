# seictl CLI surface

Canonical command reference. **`seictl --help` is the source of truth** — when this file disagrees, the CLI wins.

Last verified: 2026-04-27 against sei-protocol/seictl#65 (LLD merged; implementation lands in vertical slices behind the LLD's exit-code matrix and JSON schemas).

## Top-level commands

| Command | Domain | What it does |
|---|---|---|
| `seictl config patch` | local | Patch app.toml/client.toml/config.toml |
| `seictl genesis patch` | local | Patch genesis JSON |
| `seictl patch` | local | Generic TOML/JSON merge-patch |
| `seictl serve` | local | Run the in-pod sidecar HTTP server |
| `seictl await` | local | Wait condition |
| `seictl context` | cluster | Cluster + identity ground truth |
| `seictl onboard` | cluster | Set up your engineer environment |
| `seictl bench up` | cluster | Render and apply a benchmark |
| `seictl bench down` | cluster | Tear down a benchmark by name |
| `seictl bench list` | cluster | Owner-scoped list of running benchmarks |

The skill only invokes commands flagged "cluster" above. The "local" commands aren't in the skill's interface today.

## Cluster-facing commands (skill scope)

### `seictl context`

```
seictl context
```

No required flags. Side-effect-free reads only. Returns:

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

1. Generates `clusters/harbor/engineers/<alias>/{kustomization,namespace,bench-seiload-sa}.yaml` in the platform repo, branches `<alias>/onboard-<alias>`, opens a PR via `gh`.
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
type AWSResource struct{ Kind, ARN, Action string }
```

Engineer's IAM principal is derived from `aws sts get-caller-identity` and cross-checked against the alias regex. **No `--principal-arn` flag.**

### `seictl bench up`

```
seictl bench up --image <ref> --name <name>
                [--size s|m|l] [--duration <duration>] [--apply]
```

Required: `--image`, `--name`. Defaults: size `s`, duration `30m`, namespace = `eng-<alias>` from identity.

Default behavior is dry-run; `--apply` performs server-side apply.

```go
type BenchUpResult struct {
    ChainID     string        `json:"chainId"`         // "bench-<alias>-<name>"
    Name        string        `json:"name"`
    Namespace   string        `json:"namespace"`
    ImageRef    string        `json:"imageRef"`
    ImageDigest string        `json:"imageDigest"`     // resolved sha256:...
    Size        string        `json:"size"`
    Validators  int           `json:"validators"`
    RPCNodes    int           `json:"rpcNodes"`
    Duration    string        `json:"duration"`
    ResultsS3URI string       `json:"resultsS3Uri"`
    DryRun      bool          `json:"dryRun"`
    Manifests   []ManifestRef `json:"manifests"`
    AppliedAt   *time.Time    `json:"appliedAt,omitempty"`
}

type ManifestRef struct{ Kind, Name, Namespace, Action string } // action: "create"|"update"|"unchanged" for bench up; "deleted"|"not-found"|"still-terminating" for bench down
```

Sizes: `s` = 4 validators / 1 RPC; `m` = 10 / 2; `l` = 21 / 4.

Chain ID: `bench-<alias>-<name>` (engineer benchmarks); RPC SND is `bench-<alias>-<name>-rpc`. Distinct from autobake's nightly `autobake-<run-id>`.

S3 results: `s3://harbor-sei-autobake-results/<chain-id>/<image-sha-12>/<chain-id>/report.log`.

### `seictl bench down`

```
seictl bench down --name <name> [--namespace <ns>] [--wait] [--timeout 5m]
```

Label-selected delete with `metav1.DeletePropagationForeground`. No dry-run flag — down is bounded and idempotent (re-run is fine if interrupted).

```go
type BenchDownResult struct {
    Name      string        `json:"name"`
    ChainID   string        `json:"chainId"`
    Namespace string        `json:"namespace"`
    Resources []ManifestRef `json:"resources"` // action: "deleted"|"not-found"|"still-terminating"
    DeletedAt *time.Time    `json:"deletedAt,omitempty"`
}
```

On finalizer-stuck timeout: report still-terminating resources; exit non-zero with `category: "finalizer-stuck"`. Engineer uses `kubectl` independently if they want recovery; seictl doesn't redirect to other tools.

### `seictl bench list`

```
seictl bench list [--all-namespaces] [-n <namespace>]
```

Owner-scoped via labels (`app.kubernetes.io/part-of=seictl-bench,sei.io/engineer=<alias>`).

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

All cluster commands accept `--format=json` (default) and `--format=text`. Logging goes to stderr via `seilog`; stdout is reserved for the JSON envelope.

### Output envelope

Every command wraps output in:

```go
type Envelope struct {
    Kind    string          `json:"kind"`              // e.g. "bench.up.result"
    Version string          `json:"version"`           // schema version, "v1"
    Data    json.RawMessage `json:"data,omitempty"`
    Error   *ErrorBody      `json:"error,omitempty"`
}
type ErrorBody struct {
    Code     int    `json:"code"`
    Category string `json:"category"`
    Message  string `json:"message"`
    Detail   string `json:"detail,omitempty"`
}
```

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
- **identity:** `malformed`, `missing`, `kubeconfig-parse`, `perms-loose`

### Read-only by default

`bench up` and `onboard` default to dry-run. Nothing is applied without `--apply`. Safe to expose as MCP tools later without auth gymnastics.

### No ambient state

Commands never `cd`, never modify `~/.kube/config`, never set env vars in the calling shell. Every kubectl invocation is explicit about context and namespace.

## MCP graduation notes

Each subcommand maps to one MCP tool. The MCP server flattens names (`bench_up`, `bench_down`, etc.) — no `cluster_` infix to strip. Tool descriptions come from `seictl <cmd> --help`. JSON schemas above are the v2 MCP tool contracts; stable through MCP graduation.
