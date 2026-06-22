# Secrets (SOPS + per-cell KMS) kit

## 1. What this concern is

Secrets are **delivered into the cluster as SOPS-encrypted files in git, decrypted by Flux at apply time, keyed to a per-cell KMS CMK**. The generic *delivery* mental models — a Secrets Store **CSI `SecretProviderClass`**, External Secrets Operator, or Sealed Secrets — are reasonable *elsewhere* but **none are used here**; reaching for a SecretProviderClass is wrong for this fleet. **Scope the claim carefully:** "no Secrets Manager" is true only of the *delivery path* — a workload may still **read** a cloud secret at runtime via its own role (the `heatseeker` workload does, via `secretsmanager:GetSecretValue` on its Pod-Identity role). Delivery ≠ a workload's runtime SDK fetch. *Cited:* `clusters/<cell>/.sops.yaml`, `clusters/prod/arctic-1/validators/.../*-signing-key.secret.yaml`, `terraform/aws/189176372795/eu-central-1/prod/heatseeker.tf:31-35` (the runtime SM read), `cell-bootstrap.md` Appendix B; `sources.md` §secrets (the generic this overrides).

## 2. The pattern (how this fleet does it)

- **One CMK + alias per cell.** `alias/<cell>`; the cell's secrets all encrypt to that one key. *Cited:* `kms.tf` per cell.
- **The `.sops.yaml` is per-cell, single-rule.** `clusters/<cell>/.sops.yaml` has ONE `creation_rules` entry: `path_regex: .*`, `encrypted_regex: ^(data|stringData|.+=)$`, `kms: <that cell's ARN>`. So it encrypts the `data`/`stringData` of any Secret in the cell tree. *Cited:* `clusters/prod/.sops.yaml`.
- **Flux decrypts.** The cell's root `Kustomization` has `decryption: { provider: sops }`; the kustomize-controller's Pod-Identity role can use the cell KMS key. *Cited:* `clusters/<cell>/flux-system/`.
- **Encrypt from *inside* the cell dir (the footgun).** `sops` resolves `.sops.yaml` by walking up from the CWD — run `sops -e` from within `clusters/<cell>/...` so it picks **that cell's** key. Encrypting from the repo root or another cell picks the wrong rule/key and the cell can't decrypt. *Cited:* `cell-bootstrap.md` Appendix B.
- **What's stored this way:** SOPS-wrapped validator signing keys, pagerduty keys — committed in-tree, encrypted.

## 3. Anti-patterns / failure modes

- **Reaching for CSI / ESO / Sealed Secrets as a *delivery* path.** A `SecretProviderClass`, an `ExternalSecret`, or a `SealedSecret` to get a secret *into* the cluster as a k8s Secret. Cue: any of those kinds/CRDs. Rewrite: a SOPS-encrypted Secret in the cell tree. (Those patterns are valid knowledge — `sources.md` §secrets — just not this fleet's delivery mechanism.) **Not an anti-pattern:** a workload reading a secret from AWS Secrets Manager *at runtime via its own Pod-Identity role* (e.g. `heatseeker`) — that's a legitimate runtime fetch, not a k8s-Secret delivery path; don't flag it as "no Secrets Manager."
- **Plaintext secret material.** A Secret `data`/`stringData`, or a `.env`/token, committed unencrypted; a secret value in a ConfigMap. Cue: unencrypted `data:` in a Secret, secrets in a ConfigMap. Rewrite: SOPS-encrypt it (the `encrypted_regex` covers `data`/`stringData`).
- **Encrypting from the wrong CWD.** Running `sops -e` outside the cell dir → wrong key → the cell's Flux can't decrypt. Cue: a secret that decrypts locally but fails in-cluster. Rewrite: encrypt from inside `clusters/<cell>/`.
- **Cross-cell key reuse.** Encrypting a cell-B secret with cell-A's key. Cue: a `.sops.yaml` ARN that doesn't match the cell. Rewrite: one CMK per cell.

## 4. Review cues

- **Dimension 2 (secrets handling):** SOPS-encrypted (not plaintext, not CSI); `encrypted_regex` covers `data`/`stringData`; right cell key; no secret in a ConfigMap. *Basis:* profile §4, `sources.md` §secrets.
- **Dimension 6 (cloud-identity):** the kustomize-controller's KMS-decrypt access is the cell's scoped Pod-Identity role. *Basis:* profile §3/§4.

## 5. One-way doors in this concern

- **A per-cell KMS / SOPS key boundary change** (rotating/replacing a CMK, changing the `.sops.yaml` rule) re-keys everything in the cell and can lock out decryption — flag for human approval.
- **Committing a previously-plaintext secret** (or rotating a validator signing key) is irreversible in git history and consensus-sensitive — flag it (a leaked-then-rotated key still lives in history).
