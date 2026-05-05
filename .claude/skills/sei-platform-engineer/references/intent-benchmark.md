# Intent: benchmark

The legacy direct-`--apply` benchmark path. Engineer says "run a benchmark against image X" *and* explicitly opts out of the GitOps flow; the procedure maps the intent to `seictl bench up --apply`. Default is GitOps (see `ephemeral-chain-flow.md`); this path is the escape hatch.

Last verified: 2026-04-28 against shipped seictl v1 (`bench up`) and platform repo `harbor-validation-results` schema (PR #72).

## What a benchmark is

A benchmark on harbor produces a **fresh chain** (validators + RPC fleet) running a candidate seid image, with a **seiload** load runner driving traffic for a fixed duration. Results land in S3 keyed by chain ID + image digest.

The pattern is derived from autobake's nightly `k8s_autobake.yml` workflow. Engineer benchmarks reuse the same templates, conventions, and result paths — just with a different chain ID prefix (`bench-` vs `autobake-`) so they're discoverable but distinct.

## Conversation flow

[outline]

### Required (skill always asks if not provided as flags)

- `--image <ref>` — the seid image to test. Must be ECR (locked to `189176372795.dkr.ecr.us-east-2.amazonaws.com/sei/sei-chain:<tag>`).
- `--name <name>` — short identifier for the benchmark, used in chain ID, labels, S3 path. Engineer types it directly; a sensible default is derived from the engineer's stated testing intent.

### Defaulted (skill only asks when defaults would surprise)

- fleet size — `s` (4 validators / 1 RPC), `m` (10 / 2), `l` (21 / 4). Default `s`.
- duration — integer minutes, range 1–240. Default `30`.

### Auto-derived (never asked)

- chain ID — `bench-<alias>-<name>`
- image digest — resolved from tag at invocation time via `aws ecr describe-images`
- seiload profile — `autobake/profiles/autobake_evm_transfer.json` (embedded in seictl)
- S3 results path — `s3://harbor-validation-results/<namespace>/<job>/<run>/report.log` (per platform repo's `harbor-validation-results` schema; see §S3 below)

## Fleet shape (autobake-derived)

[outline]

Two SeiNodeDeployments + ConfigMap + Job:

- **SND 1: validators** (chain ID = `bench-<alias>-<name>`), fresh genesis ceremony, validator role
- **SND 2: RPC nodes** (chain ID = `bench-<alias>-<name>-rpc`), block-syncs from validators via label-based peer discovery
- **ConfigMap: seiload profile** rendered with chain ID + per-pod RPC endpoints
- **Job: seiload runner** consumes the ConfigMap and writes report.log to S3

Templates that drive the shape (embedded in seictl, derived from):

- `autobake/templates/seinodedeployment.yaml` (SND validator + RPC variants)
- `autobake/templates/seiload-job.yaml`
- `autobake/profiles/autobake_evm_transfer.json` (seiload profile default)

When autobake updates these templates, seictl's embedded copies update on next release.

## S3 results convention

Bucket: `harbor-validation-results` (the platform repo's unified validation-results bucket; lifecycle policy managed there)

Path: `s3://harbor-validation-results/<namespace>/<job>/<run>/report.log`

- `<namespace>` — engineer's namespace (`eng-<alias>`)
- `<job>` — the seiload Job name within the bench
- `<run>` — bench `--name`

Engineer benchmark example: `s3://harbor-validation-results/eng-bdchatham/seiload/mempool-ttl/report.log`

Per-engineer results are partitioned at the namespace level. `aws s3 ls s3://harbor-validation-results/eng-<alias>/ --profile sei` returns just that engineer's runs. The IAM policy `seictl onboard` provisions scopes the engineer's PutObject permission to this prefix.

## Pre-flight (in order)

[outline — implemented in seictl, not the skill]

1. `gh auth status` — fail if unauthenticated (only relevant if the run also triggers an onboarding PR)
2. Platform repo locatable (env `SEI_PLATFORM_REPO` or fallback path)
3. `~/.seictl/engineer.json` exists and parses
4. kubectl context = `harbor`
5. ECR image manifest resolves (with autobake's race-guard retry: 3 attempts, 60s sleep — sei-chain CI may be behind)
6. Engineer's namespace (`eng-<alias>`) exists in-cluster (Flux-reconciled from prior PR)
7. Cluster headroom check (warn over 70%, block over 90%)

## PR description shape (when onboarding triggers)

[outline]

If the engineer hasn't onboarded yet, `seictl bench up` halts and routes them to `seictl onboard` first. Onboarding PRs follow `references/pr-conventions.md`.

## Anti-patterns

The skill (and seictl) won't:

- Ask more than 3 questions per `bench up` invocation
- Generate manifests the engineer can't see in `seictl bench up --dry-run`
- Pick "smart" defaults that surprise (e.g. swap profile based on image tag regex)
- Poll Flux or stream pod logs synchronously (`bench up` ends at "applied" — `bench list` covers follow-up; manual `kubectl` covers debug per `references/troubleshooting-seinode.md`)
- Re-prompt for identity once captured
