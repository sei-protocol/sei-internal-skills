# Intent: benchmark

The skill's headline use case. Engineer says "run a benchmark against image X"; the skill maps it to `seictl bench up`.

Last verified: 2026-04-26 against autobake's k8s_autobake.yml.

## What a benchmark is

A benchmark on harbor produces a **fresh chain** (validators + RPC fleet) running a candidate seid image, with a **seiload** load runner driving traffic for a fixed duration. Results land in S3 keyed by chain ID + image digest.

The pattern is derived from autobake's nightly `k8s_autobake.yml` workflow. Engineer benchmarks reuse the same templates, conventions, and result paths — just with a different chain ID prefix (`bench-` vs `autobake-`) so they're discoverable but distinct.

## Conversation flow

[outline]

### Required (skill always asks if not provided as flags)

- `--image <ref>` — the seid image to test (e.g. `ghcr.io/sei-protocol/sei-chain:pr-1234` or `189176372795.dkr.ecr.us-east-2.amazonaws.com/sei/sei-chain:mock-<sha>`)
- intent description — one sentence, goes into the chain ID slug; required, no default

### Defaulted (skill only asks when defaults would surprise)

- fleet size — `s` (4 validators), `m` (10), `l` (21). Default `s`.
- duration — minutes. Default 30.

### Auto-derived (never asked)

- chain ID — `bench-<alias>-<slug>`
- run ID — same as chain ID (PR-keyed; no separate run_id needed)
- image digest — resolved from tag at invocation time via `aws ecr describe-images`
- seiload profile — `autobake/profiles/autobake_evm_transfer.json` default
- S3 results prefix — derived per autobake convention (see below)

## Fleet shape (autobake-derived)

[outline]

Two SeiNodeDeployments + ConfigMap + Job:

- **SND 1: validators** (chain ID = `bench-<alias>-<slug>`), fresh genesis ceremony, validator role
- **SND 2: RPC nodes** (chain ID = `bench-<alias>-<slug>-rpc`), block-syncs from validators via label-based peer discovery
- **ConfigMap: seiload profile** rendered with chain ID + per-pod RPC endpoints
- **Job: seiload runner** consumes the ConfigMap and writes report.log to S3

Templates that drive the shape (embedded in seictl, derived from):

- `autobake/templates/seinodedeployment.yaml` (SND validator + RPC variants)
- `autobake/templates/seiload-job.yaml`
- `autobake/profiles/autobake_evm_transfer.json` (seiload profile default)

When autobake updates these templates, seictl's embedded copies update on next release.

## S3 results convention

Bucket: `harbor-sei-autobake-results` (shared with nightly autobake; 90-day lifecycle already configured)

Path: `s3://harbor-sei-autobake-results/<chain-id>/<image-sha-12>/<run-id>/report.log`

Engineer benchmark example: `s3://harbor-sei-autobake-results/bench-bdchatham-mempool-ttl/abc123def456/bench-bdchatham-mempool-ttl/report.log`

The `bench-` chain ID prefix partitions engineer results from `autobake-` nightly results. `aws s3 ls s3://harbor-sei-autobake-results/bench-<alias>-` returns just that engineer's runs.

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
- Poll Flux or stream pod logs synchronously (`bench up` ends at "applied" — `seinode diagnose` and `bench list` cover follow-up)
- Re-prompt for identity once captured
